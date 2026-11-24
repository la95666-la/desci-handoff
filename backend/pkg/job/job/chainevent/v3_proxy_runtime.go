package chainevent

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/common/projecttracking"
)

type V3ProxyStreamConfig struct {
	Mode             string
	Anchor           uint64
	AnchorHash       common.Hash
	ExternalProducer bool // runtime-owned separate scheduler, not a YAML authority flag
}

// Explicit config only; default-off must not query draft tables on old installs.
func ConfiguredV3ProxyStream(db *gorm.DB) (V3ProxyStreamConfig, error) {
	c := V3ProxyStreamConfig{Mode: viper.GetString("idoV3ProxyStream.mode"), Anchor: viper.GetUint64("idoV3ProxyStream.anchorBlock"), AnchorHash: common.HexToHash(viper.GetString("idoV3ProxyStream.anchorHash"))}
	if c.Mode == "" || c.Mode == "off" {
		c.Mode = "off"
		return c, nil
	}
	c.ExternalProducer = true
	if (c.Mode != "shadow" && c.Mode != "stream") || c.Anchor == 0 || c.AnchorHash == (common.Hash{}) || db == nil {
		return c, errV3Stream
	}
	var tables int64
	if db.Raw(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN ('ido_v3_proxy_stream','ido_v3_proxy_coverage','ido_v3_proxy_consumer')`).Scan(&tables).Error != nil || tables != 3 {
		return c, errV3Stream
	}
	var triggers int64
	if db.Raw(`SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN ('jc_v3_coverage_update','jc_v3_coverage_delete','jc_v3_stream_cursor_insert','jc_v3_stream_cursor_update')`).Scan(&triggers).Error != nil || triggers != 4 {
		return c, errV3Stream
	}
	var constraints int64
	if db.Raw(`SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_NAME IN ('chk_v3_stream','chk_v3_coverage','fk_v3_coverage_stream','fk_v3_consumer_binding')`).Scan(&constraints).Error != nil || constraints != 4 {
		return c, errV3Stream
	}
	var epochColumns int64
	if db.Raw(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN ('ido_v3_proxy_stream','ido_v3_proxy_coverage') AND COLUMN_NAME='epoch' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO'`).Scan(&epochColumns).Error != nil || epochColumns != 2 {
		return c, errV3Stream
	}
	return c, nil
}

func productionV3StreamScope() v3StreamScope {
	return v3StreamScope{Chain: 56, Proxy: common.HexToAddress("0x0d4040875742975269c55e16c8884d3ee9510b8e"), Runtime: idov3whitelist.RuntimeIdentity{Implementation: common.HexToAddress("0x887fcc6e54fc8229919166ad90b16c19b4992560"), ProxyCodeHash: common.HexToHash("0x5974ff3c9404cc36ed5c7d8a3eab148940366bbf46740d064255c13a842f3cb8"), CodeHash: common.HexToHash("0x015279399127ac168667c7fc5238ec04a16923c7561d54b4d823d77e5f55c590")}, Confirmations: 15}
}

func runV3ProxyStream(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, c V3ProxyStreamConfig, owner, governor common.Hash, after *uint64) (approvedV3ScanResult, error) {
	return runV3ProxyStreamWithBudget(ctx, db, rpc, c, owner, governor, after, &v3ProducerBudget{})
}

func runV3ProxyStreamWithBudget(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, c V3ProxyStreamConfig, owner, governor common.Hash, after *uint64, budget *v3ProducerBudget) (result approvedV3ScanResult, resultErr error) {
	s := productionV3StreamScope()
	sourcePublished := false
	defer func() {
		if !sourcePublished {
			cleanup, cancel := freshnessCleanupContext(ctx)
			defer cancel()
			_ = markV3FreshSource(cleanup, db, s, v3SharedConfirmed{height: 1}, 1, false)
		}
	}()
	stream, e := v3StreamCursorRead(db.WithContext(ctx), s, false)
	if e != nil {
		if c.ExternalProducer {
			return approvedV3ScanResult{}, errV3Stream
		}
		// Only shadow can bootstrap; never switch an existing row or silently
		// treat a DB failure as permission to initialize a live writer.
		var count int64
		if c.Mode != "shadow" || db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM ido_v3_proxy_stream WHERE chain_id=? AND proxy_address=? AND contract_version=?`, s.args()...).Scan(&count).Error != nil || count != 0 {
			return approvedV3ScanResult{}, errV3Stream
		}
		if e = bootstrapV3Stream(ctx, db, rpc, s, "shadow", c.Anchor, c.AnchorHash); e != nil {
			return approvedV3ScanResult{}, e
		}
		stream, e = v3StreamCursorRead(db.WithContext(ctx), s, false)
	}
	if e != nil || stream.Mode != c.Mode || (stream.Epoch == 1 && stream.AnchorBlock != c.Anchor) || (stream.Epoch > 1 && stream.AnchorBlock < c.Anchor) {
		return approvedV3ScanResult{}, errV3Stream
	}
	if stream.Epoch == 1 {
		anchor, e := readV3Coverage(db.WithContext(ctx), s, c.Anchor, c.Anchor)
		if e != nil || anchor[c.Anchor].header().Hash() != c.AnchorHash {
			return approvedV3ScanResult{}, errV3Stream
		}
	}
	if c.ExternalProducer {
		if stream.State != "running" || v3StreamAnchor(ctx, rpc, s, stream.CursorBlock, common.HexToHash(stream.CursorHash)) != nil {
			return approvedV3ScanResult{}, errV3Stream
		}
	}
	producerCtx, cancelProducer := context.WithTimeout(ctx, 12*time.Second)
	if !c.ExternalProducer {
		_, e = produceV3StreamBudget(producerCtx, db, rpc, s, budget)
	}
	cancelProducer()
	if e != nil {
		// Parent slice expiry is expected scheduling, not a proven reorg.
		// Establish the durable tip independently before allowing consumers.
		if !errors.Is(e, errV3StreamConflict) && (errors.Is(producerCtx.Err(), context.DeadlineExceeded) || errors.Is(e, errV3UnitDeadline)) && ctx.Err() == nil {
			current, readErr := v3StreamCursorRead(db.WithContext(ctx), s, false)
			if readErr == nil && current.State == "running" && v3StreamAnchor(ctx, rpc, s, current.CursorBlock, common.HexToHash(current.CursorHash)) == nil {
				// Fresh tip only authorizes consumers to inspect existing evidence;
				// no publication of full-tick health on a producer failure.
				if c.Mode == "shadow" {
					return scanApprovedV3Fair(ctx, db, rpc, owner, governor, after)
				}
				return approvedV3ScanResult{}, e
			}
		}
	}
	if e != nil {
		if v3Deferred(e) {
			return approvedV3ScanResult{}, e
		}
		// Unknown source is a stop, not an inferred ancestor. Shadow failures
		// halt only evidence production, not the active legacy project writers.
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if c.Mode == "stream" {
			if haltErr := haltV3Stream(cleanup, db, s); haltErr != nil {
				return approvedV3ScanResult{}, fmt.Errorf("%w: projection revocation failed", errV3Stream)
			}
		} else {
			_ = db.WithContext(cleanup).Exec(`UPDATE ido_v3_proxy_stream SET state='halted',revision=revision+1 WHERE chain_id=? AND proxy_address=? AND contract_version=? AND state='running'`, s.args()...).Error
			// Shadow is observational: do not stop/revoke production's legacy
			// workload because an optional evidence producer failed.
			return scanApprovedV3Fair(ctx, db, rpc, owner, governor, after)
		}
		return approvedV3ScanResult{}, e
	}
	if c.Mode == "stream" && e == nil {
		// A recovered/quota-limited producer may be far behind. Consumers that
		// have not yet been covered are deferred, not mislabeled corrupt and
		// revoked merely because backfill has not reached their existing tip.
		current, readErr := v3StreamCursorRead(db.WithContext(ctx), s, false)
		var ahead int64
		if readErr != nil || db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM ido_v3_scan_cursor c WHERE chain_id=? AND proxy_address=? AND contract_version=? AND cursor_block>? AND EXISTS(SELECT 1 FROM projects p WHERE p.project_id=c.project_id AND BINARY p.status<>BINARY 'archived')`, append(s.args(), current.CursorBlock)...).Scan(&ahead).Error != nil {
			return approvedV3ScanResult{}, errV3Stream
		}
		if ahead > 0 {
			return approvedV3ScanResult{}, fmt.Errorf("%w: producer backfill pending", errV3Stream)
		}
	}
	if c.Mode == "shadow" {
		if e = shadowV3Consumer(ctx, db, s); e != nil {
			_ = db.WithContext(ctx).Exec(`UPDATE ido_v3_proxy_stream SET state='halted',revision=revision+1 WHERE chain_id=? AND proxy_address=? AND contract_version=? AND state='running'`, s.args()...).Error
		}
		return scanApprovedV3Fair(ctx, db, rpc, owner, governor, after)
	}
	var ids []uint64
	var cursor uint64
	for {
		page, e := idov3whitelist.ListApprovedV3ProjectIDs(ctx, db, cursor, 128)
		if e != nil {
			return approvedV3ScanResult{}, e
		}
		ids = append(ids, page...)
		if len(page) < 128 {
			break
		}
		cursor = page[len(page)-1]
	}
	// Shared financial/effect indexes and nested recovery transactions can
	// deadlock across projects. Public production remains independently concurrent.
	ctx = newV3CoverageReads(ctx, db)
	var candidateErr error
	if projecttracking.Enabled() && !idov3whitelist.ProjectFreshnessEnabled() {
		return approvedV3ScanResult{}, errors.New("V3 tracking requires project freshness")
	}
	allCount := len(ids)
	ids, candidateErr = selectV3TrackingCandidates(ctx, db, ids)
	if candidateErr != nil {
		return approvedV3ScanResult{}, candidateErr
	}
	defer func() {
		if len(ids) < allCount {
			result.Total = allCount
			result.Skipped += allCount - len(ids)
			if resultErr == nil {
				resultErr = errors.New("V3 scheduled maintenance not full enumeration")
			}
		}
	}()
	ctx = context.WithValue(ctx, v3SharedBatchKey{}, true)
	// One confirmed public deployment height per bounded round.
	head, headErr := rpc.BlockNumber(ctx)
	if headErr != nil || head < 14 {
		return approvedV3ScanResult{}, errV3Stream
	}
	confirmed := head - 14
	h, headErr := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(confirmed))
	if headErr != nil || h == nil || h.Number == nil || !h.Number.IsUint64() || h.Number.Uint64() != confirmed {
		return approvedV3ScanResult{}, errV3Stream
	}
	ctx = context.WithValue(ctx, v3SharedConfirmedKey{}, v3SharedConfirmed{height: confirmed, hash: h.Hash()})
	if _, enabled := ctx.Value(v3FreshnessKey{}).(string); enabled {
		if err := v3StreamRuntime(ctx, rpc, s, h); err != nil {
			return approvedV3ScanResult{}, err
		}
	}
	freshTarget := v3SharedConfirmed{height: confirmed, hash: h.Hash()}
	if _, enabled := ctx.Value(v3FreshnessKey{}).(string); enabled {
		// Producer normally trails confirmed head by a few blocks. Certify
		// its actual independently verified committed tip, not an uncovered
		// future height which no consumer can yet reach.
		current, e := v3StreamCursorRead(db.WithContext(ctx), s, false)
		if e != nil {
			return approvedV3ScanResult{}, e
		}
		if current.CursorBlock < confirmed && confirmed-current.CursorBlock > idov3whitelist.SourceFreshnessMaxConfirmedLag {
			return approvedV3ScanResult{}, &v3ScanDeferred{stage: "source_freshness_lag"}
		}
		if e := v3StreamAnchor(ctx, rpc, s, current.CursorBlock, common.HexToHash(current.CursorHash)); e != nil {
			return approvedV3ScanResult{}, e
		}
		freshTarget = v3SharedConfirmed{height: current.CursorBlock, hash: common.HexToHash(current.CursorHash)}
	}
	if err := markV3FreshSource(ctx, db, s, freshTarget, head, true); err != nil {
		return approvedV3ScanResult{}, err
	}
	sourcePublished = true
	publicReads := newV3DeploymentReads(rpc)
	rpc = publicReads
	defer func() {
		if logger := log.GetLogger(); logger != nil {
			hits, reads := publicReads.stats()
			logger.WithFields(map[string]any{"deployment_shared_hits": hits, "deployment_source_reads": reads, "max_consumer_blocks": v3SharedConsumerBlocks, "confirmed_block": confirmed}).Info("V3 shared event batch reads")
		}
	}()
	if turn, ok := ctx.Value(v3AuditContextKey{}).(*v3AuditTurn); ok && turn.cache != nil {
		turn.cache.retain(ids)
	}
	scan := func(projectCtx context.Context, id uint64) (resultErr error) {
		if projecttracking.Enabled() {
			row, err := projecttracking.Read(projectCtx, db, id, false)
			if err != nil {
				return err
			}
			projectCtx = context.WithValue(projectCtx, trackingFenceKey{}, trackingFence{revision: row.Revision, mode: row.Mode})
		}
		// Keep previous evidence during ordinary checking; revoke only after
		// an actual integrity/identity failure (including before consumption).
		defer func() { resultErr = finishV3ProjectRescan(projectCtx, db, s, id, resultErr) }()
		attempt := func() error {
			v3TracePhase(projectCtx, "stream_project_entry")
			// Keep current Set/original-create/runtime admission entirely independent
			// of stream coverage. Bridge/recovery runs through the existing path.
			// Running-project admission is read-only and must not hold the public
			// producer row while doing historical RPC. Bootstrap/recovery still use
			// their controlled transaction because they can mutate project cursors.
			work := func(database *gorm.DB) error {
				current, e := v3StreamCursorRead(database.WithContext(projectCtx), s, false)
				if e != nil || current.State != "running" || current.Mode != "stream" {
					return errV3Stream
				}
				if e := requireV3RecoveryCoverage(projectCtx, database, s, id); e != nil {
					return e
				}
				return scanApprovedV3ProjectWithRange(projectCtx, database, rpc, id, owner, governor, func(c context.Context, db *gorm.DB, r v3ScannerRPC, a idov3whitelist.AdmittedProject) error {
					return consumeV3StreamWindows(c, db, r, s, a.Binding, a.Runtime)
				})
			}
			var cursor struct{ State string }
			v3TracePhase(projectCtx, "cursor_state")
			read := db.WithContext(projectCtx).Raw(`SELECT state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version=?`, s.Chain, v3Lower(s.Proxy), id, v3StreamVersion).Scan(&cursor)
			if read.Error != nil {
				return errV3Stream
			}
			if read.RowsAffected == 1 && cursor.State == "running" {
				return work(db)
			}
			// Stream recovery requires full public coverage and explicit reviewed
			// original-hash recovery, not repeated legacy <=512-block attempts under
			// the public row lock. Keep halted projects fenced until operator recovery.
			if read.RowsAffected == 1 {
				return errV3Stream
			}
			return v3StreamWriterTx(projectCtx, db, func(tx *gorm.DB) error {
				if _, e := v3StreamCursorRead(tx.WithContext(projectCtx), s, true); e != nil {
					return e
				}
				return work(tx)
			})
		}
		if retries, ok := ctx.Value(v3RetryKey{}).(*v3RetryState); ok {
			return retries.attempt(projectCtx, id, attempt)
		}
		return attempt()
	}
	revoke := func(id uint64) error {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return v3StreamWriterTx(cleanup, db, func(tx *gorm.DB) error {
			if _, e := v3StreamCursorRead(tx, s, true); e != nil {
				return e
			}
			return haltApprovedV3Projection(cleanup, tx, id)
		})
	}
	if schedule, ok := ctx.Value(v3PriorityKey{}).(*v3PrioritySchedule); ok {
		blocked := func(uint64) bool { return false }
		if retries, ok := ctx.Value(v3RetryKey{}).(*v3RetryState); ok {
			blocked = retries.blocked
		}
		return runV3PriorityRound(ctx, ids, schedule, time.Now, func() ([]v3PriorityHint, error) {
			hints, err := loadV3PriorityHints(ctx, db, ids, time.Now())
			if err != nil {
				return nil, err
			}
			if turn, ok := ctx.Value(v3AuditContextKey{}).(*v3AuditTurn); ok && turn.cache != nil {
				for i := range hints {
					if hints[i].Locked {
						hints[i].FullAudit = turn.cache.needsFull(hints[i].ID, time.Now())
					}
				}
			}
			return schedule.eventHints(ctx, db, s, hints)
		}, blocked, scan, revoke)
	}
	return fairApprovedV3ScanWithStartBudget(ctx, ids, after, approvedV3ProjectBudget, approvedV3StreamAdmissionFloor, 1, scan, revoke)
}

// Shadow->stream is explicit and transactional. Call only after separate
// rollout approval, with exact expected revision and no running old process.
// No new public HTTP route or automatic startup switch is provided.
func switchV3StreamMode(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, s v3StreamScope, revision uint64, target string) error {
	if target != "shadow" && target != "stream" {
		return errV3Stream
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		c, e := v3StreamCursorRead(tx, s, true)
		if e != nil || c.State != "running" || c.Revision != revision || c.Mode == target {
			return errV3Stream
		}
		if e = v3StreamAnchor(ctx, rpc, s, c.CursorBlock, common.HexToHash(c.CursorHash)); e != nil {
			return e
		}
		if c.Mode == "shadow" && target == "stream" {
			if e = shadowV3Consumer(ctx, tx, s); e != nil {
				return e
			}
		}
		// All project tips must already be represented by matching continuous
		// public coverage. Never advance a consumer cursor during mode switch.
		var rows []struct {
			ProjectID         uint64
			CursorBlock       uint64
			CursorHash, State string
		}
		if tx.Raw(`SELECT project_id,cursor_block,cursor_hash,state FROM ido_v3_scan_cursor c WHERE chain_id=? AND proxy_address=? AND contract_version=? AND EXISTS(SELECT 1 FROM projects p WHERE p.project_id=c.project_id AND BINARY p.status<>BINARY 'archived') FOR UPDATE`, s.args()...).Scan(&rows).Error != nil {
			return errV3Stream
		}
		for _, p := range rows {
			if p.State != "running" || p.CursorBlock < c.AnchorBlock || p.CursorBlock > c.CursorBlock {
				return errV3Stream
			}
			coverage, e := readV3Coverage(tx, s, p.CursorBlock, p.CursorBlock)
			if e != nil || strings.ToLower(coverage[p.CursorBlock].header().Hash().Hex()) != p.CursorHash {
				return errV3Stream
			}
			if e = v3StreamAnchor(ctx, rpc, s, p.CursorBlock, common.HexToHash(p.CursorHash)); e != nil {
				return e
			}
			if target == "stream" {
				if _, e = auditV3ProjectParityInput(ctx, tx, rpc, s, p.ProjectID, p.CursorBlock, common.HexToHash(p.CursorHash)); e != nil {
					return e
				}
			}
		}
		r := tx.Exec(`UPDATE ido_v3_proxy_stream SET mode=?,revision=revision+1 WHERE chain_id=? AND proxy_address=? AND contract_version=? AND revision=? AND state='running'`, append([]any{target}, append(s.args(), revision)...)...)
		if r.Error != nil || r.RowsAffected != 1 {
			return errV3Stream
		}
		return nil
	})
}

// Separate progress report; never a substitute for existing proof/fee gates.
func V3ProxyStreamProgress(ctx context.Context, db *gorm.DB) (map[string]any, error) {
	if ctx == nil || db == nil {
		return nil, errV3Stream
	}
	s := productionV3StreamScope()
	c, e := v3StreamCursorRead(db.WithContext(ctx), s, false)
	if e != nil {
		return nil, e
	}
	var consumers []struct {
		ProjectID   uint64
		CursorBlock uint64
		State       string
		ProducerGap uint64
	}
	e = db.WithContext(ctx).Raw(`SELECT p.project_id,p.cursor_block,p.state,GREATEST(CAST(? AS SIGNED)-CAST(p.cursor_block AS SIGNED),0) AS producer_gap FROM ido_v3_scan_cursor p WHERE p.chain_id=? AND p.proxy_address=? AND p.contract_version=? AND EXISTS(SELECT 1 FROM projects project WHERE project.project_id=p.project_id AND BINARY project.status<>BINARY 'archived') ORDER BY p.project_id`, append([]any{c.CursorBlock}, s.args()...)...).Scan(&consumers).Error
	if e != nil {
		return nil, e
	}
	confirmed := uint64(0)
	if c.ObservedHead >= s.Confirmations {
		confirmed = c.ObservedHead - s.Confirmations + 1
	}
	producerLag := uint64(0)
	if confirmed > c.CursorBlock {
		producerLag = confirmed - c.CursorBlock
	}
	var pending struct {
		Count  uint64
		Oldest string
	}
	if db.WithContext(ctx).Raw(`SELECT COUNT(*) AS count,COALESCE(CAST(MIN(updated_at) AS CHAR),'') AS oldest FROM chain_tx_submission WHERE chain_id=? AND contract_address=? AND state='final_success'`, s.Chain, v3Lower(s.Proxy)).Scan(&pending).Error != nil {
		return nil, errV3Stream
	}
	return map[string]any{"producer": c, "confirmed_head": confirmed, "producer_lag": producerLag, "consumers": consumers, "pending_final_success": pending}, nil
}
