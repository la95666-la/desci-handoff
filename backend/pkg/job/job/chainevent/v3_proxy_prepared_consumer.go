package chainevent

import (
	"context"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
)

type v3ConsumerCursor struct {
	CursorBlock, Revision uint64
	CursorHash, State     string
}

func readV3ConsumerCursor(db *gorm.DB, b idov3whitelist.ExpectedProjectBinding, lock bool) (v3ConsumerCursor, error) {
	var c v3ConsumerCursor
	q := `SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version=?`
	if lock {
		q += " FOR UPDATE"
	}
	r := db.Raw(q, b.ChainID, v3Lower(b.ProxyAddress), b.ProjectID, v3StreamVersion).Scan(&c)
	if r.Error != nil || r.RowsAffected != 1 || c.State != "running" || c.CursorBlock == 0 || c.Revision == 0 || c.Revision == math.MaxUint64 {
		return c, errV3Stream
	}
	return c, nil
}

type v3ConsumerUnit struct {
	headers  []*types.Header
	observed idov3whitelist.ProjectIdentityObservation
	logs     []types.Log
	policy   []*commonchain.V3PolicyEvent
	empty    bool
}

// Explicit RPC-only preparation -> ONE writer transaction. No tentative SQL
// writes or replay-until-next-RPC loop. All durable publication checks remain.
func consumeV3Stream(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, s v3StreamScope, b idov3whitelist.ExpectedProjectBinding, rt idov3whitelist.RuntimeIdentity, limit uint64) (resultErr error) {
	shared := ctx != nil && ctx.Value(v3SharedBatchKey{}) == true
	maximum := v3ConsumerBlockLimit(ctx)
	if ctx == nil || db == nil || rpc == nil || !s.valid() || b.ChainID != s.Chain || b.ProxyAddress != s.Proxy || rt != s.Runtime || limit == 0 || limit > maximum {
		return errV3Stream
	}
	defer func() { resultErr = finishV3ProjectRescan(ctx, db, s, b.ProjectID, resultErr) }()
	tracked := &v3TrackedRPC{v3ScannerRPC: rpc}
	rpc = tracked
	defer func() { resultErr = tracked.finish(ctx, "consumer_prepare_or_publish", resultErr) }()
	v3TracePhase(ctx, "stream_snapshot")
	before, e := v3StreamCursorRead(db.WithContext(ctx), s, false)
	if e != nil || before.Mode != "stream" || before.State != "running" {
		return errV3Stream
	}
	if e = recheckV3ScannerAuditGrant(ctx, db, b, before); e != nil {
		return e
	}
	v3TracePhase(ctx, "project_cursor")
	original, e := readV3ConsumerCursor(db.WithContext(ctx), b, false)
	if e != nil || original.CursorBlock < before.AnchorBlock || original.CursorBlock > before.CursorBlock || original.CursorBlock == math.MaxUint64 {
		return errV3Stream
	}
	end := before.CursorBlock
	if end-original.CursorBlock > limit {
		end = original.CursorBlock + limit
	}
	if scheduled, _ := ctx.Value(v3ScheduledConsumerBudget{}).(bool); scheduled {
		if err := requireV3ConsumerWindowBudget(ctx, end > original.CursorBlock); err != nil {
			return err
		}
	}
	v3TraceWindow(ctx, original.CursorBlock, original.Revision, before.Epoch, original.CursorBlock+1, end)
	v3TracePhase(ctx, "coverage_read")
	var coverage map[uint64]v3StreamBlock
	if shared {
		coverage, e = readV3SharedCoverage(ctx, db, s, before, original.CursorBlock, end)
	} else {
		coverage, e = readV3ConsumerCoverage(ctx, db, s, before.Epoch, original.CursorBlock, end)
	}
	if e != nil || v3LowerHashHeader(coverage[original.CursorBlock].header()) != original.CursorHash {
		return errV3Stream
	}
	// The public coverage is not a replacement for THIS project's durable
	// checkpoint, including idle ticks which do not enter a journal writer.
	var checkpoints []struct {
		Hash string `gorm:"column:block_hash"`
	}
	v3TracePhase(ctx, "project_checkpoint")
	read := db.WithContext(ctx).Raw(`SELECT block_hash FROM ido_v3_block_checkpoint WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version=? AND block_number=? AND status='canonical' LIMIT 2`, b.ChainID, v3Lower(b.ProxyAddress), b.ProjectID, v3StreamVersion, original.CursorBlock).Scan(&checkpoints)
	if read.Error != nil {
		return read.Error
	}
	if len(checkpoints) != 1 || checkpoints[0].Hash != original.CursorHash {
		return v3Conflict("project_checkpoint")
	}
	// No side-effecting legacy halt here. The caller serializes revocation
	// stream->project after receiving an integrity error.
	v3TracePhase(ctx, "cursor_anchor")
	if e = v3StreamAnchor(ctx, rpc, s, original.CursorBlock, common.HexToHash(original.CursorHash)); e != nil {
		return e
	}
	v3TracePhase(ctx, "public_anchor")
	if e = v3StreamAnchor(ctx, rpc, s, before.CursorBlock, common.HexToHash(before.CursorHash)); e != nil {
		return e
	}
	if shared && end == original.CursorBlock {
		// No new evidence: do not lock/update business rows or refresh the
		// durable consumer success timestamp merely because a poll occurred.
		v3TraceConsumerProgress(ctx, original.CursorBlock, original.CursorBlock, before.CursorBlock)
		logV3ConsumerProgress(ctx, b.ProjectID)
		if err := verifyV3ProjectClosure(ctx, db, rpc, s, b, rt, original, before); err != nil {
			return &v3ScanDeferred{stage: "closure_check_unavailable", cause: err}
		}
		if err := publishV3ProjectFreshness(ctx, db, s, b, original, before); err != nil {
			return &v3ScanDeferred{stage: "freshness_publication_unavailable", cause: err}
		}
		return nil
	}
	replay := v3CoverageRPC{v3ScannerRPC: rpc, blocks: coverage, proxy: s.Proxy}
	var units []v3ConsumerUnit
	var lastUnit time.Duration
	preparedEnd := original.CursorBlock
	for height := original.CursorBlock + 1; height <= end; {
		if len(units) > 0 && yieldV3RangeBetweenWindows(ctx, lastUnit) {
			break
		}
		unitStarted := time.Now()
		last := height
		// Shared mode routes by project ID: only own/global events require
		// heavy replay. The reference path keeps complete-proxy empty windows.
		// Unknown/malformed topics never become quiet evidence.
		quiet := len(coverage[height].Logs) == 0
		if shared {
			affects, err := v3BlockAffectsProject(coverage[height], b.ProjectID)
			if err != nil {
				return err
			}
			quiet = !affects
		}
		if quiet {
			for last < end && last-height+1 < maximum {
				nextQuiet := len(coverage[last+1].Logs) == 0
				if shared {
					affects, err := v3BlockAffectsProject(coverage[last+1], b.ProjectID)
					if err != nil {
						return err
					}
					nextQuiet = !affects
				}
				if !nextQuiet {
					break
				}
				last++
			}
			v3TracePhase(ctx, "prepare_empty")
			v3TraceBlock(ctx, height, 0, 0, 0)
			var u v3ConsumerUnit
			if shared {
				u, e = prepareV3ProjectQuiet(ctx, replay, s, b, rt, height, last)
			} else {
				u, e = prepareV3ConsumerEmpty(ctx, replay, s, b, rt, height, last)
			}
			if e != nil {
				return e
			}
			units = append(units, u)
		} else {
			v3TracePhase(ctx, "prepare_event")
			v3TraceBlock(ctx, height, len(coverage[height].Logs), 0, v3PurchaseCount(coverage[height].Logs, b))
			u, e := prepareV3ConsumerBlock(ctx, replay, s, b, rt, height)
			if e != nil {
				return e
			}
			units = append(units, u)
		}
		preparedEnd = last
		lastUnit = time.Since(unitStarted)
		height = last + 1
	}
	targetHash := coverage[preparedEnd].header().Hash()
	v3TracePhase(ctx, "precommit_anchor")
	if e = v3StreamAnchor(ctx, rpc, s, preparedEnd, targetHash); e != nil {
		return e
	}
	if e = v3StreamAnchor(ctx, rpc, s, before.CursorBlock, common.HexToHash(before.CursorHash)); e != nil {
		return e
	}
	var published v3ConsumerCursor
	// Do not knowingly commit a new tip with no time left for postcommit
	// anchors/publication. Old evidence stays unchanged on this scheduling
	// yield. Timeout after commit still fails closed and is retried normally.
	if scheduled, _ := ctx.Value(v3ScheduledConsumerBudget{}).(bool); scheduled {
		if err := requireV3PublicationBudget(ctx); err != nil {
			return err
		}
	}
	dbFailures := &v3DBFailures{Interface: db.Logger}
	e = v3StreamWriterTx(ctx, db.Session(&gorm.Session{Logger: dbFailures}), func(tx *gorm.DB) error {
		v3TracePhase(ctx, "commit_stream_lock")
		current, e := v3StreamCursorRead(tx, s, true)
		if e != nil {
			return errV3Stream
		}
		if !v3ConsumerStreamProgress(before, current) {
			return v3StreamSnapshotChanged(before, current)
		}
		v3TracePhase(ctx, "commit_project_cas")
		now, e := readV3ConsumerCursor(tx, b, true)
		if e != nil {
			return errV3Stream
		}
		if now != original {
			if now.CursorBlock > original.CursorBlock && now.Revision > original.Revision && now.Revision-original.Revision == now.CursorBlock-original.CursorBlock {
				return &v3ScanDeferred{"project_cas_changed", errV3Stream}
			}
			return v3Conflict("project_cursor_rewrite")
		}
		if e = recheckV3ScannerAuditGrant(ctx, tx, b, current); e != nil {
			return e
		}
		if e = checkV3TrackingFence(ctx, tx, b.ProjectID); e != nil {
			return e
		}
		// Epoch-scoped evidence is immutable; recheck its pinned boundaries under
		// current stream lock, not the whole payload for each RPC/SQL probe.
		v3TracePhase(ctx, "commit_coverage")
		for _, anchor := range []struct {
			n    uint64
			hash common.Hash
		}{{original.CursorBlock, common.HexToHash(original.CursorHash)}, {preparedEnd, targetHash}, {before.CursorBlock, common.HexToHash(before.CursorHash)}} {
			blocks, e := readV3Coverage(tx, s, anchor.n, anchor.n)
			if e != nil || blocks[anchor.n].header().Hash() != anchor.hash {
				return errV3Stream
			}
		}
		for _, u := range units {
			first, last := u.headers[0], u.headers[len(u.headers)-1]
			v3TraceBlock(ctx, last.Number.Uint64(), 0, len(u.logs), v3PurchaseCount(u.logs, b))
			if u.empty {
				v3TracePhase(ctx, "commit_empty")
				if _, e := (v3Journal{db: tx}).applyVerifiedEmptyRangeBounded(ctx, b, u.headers, first.ParentHash, u.observed, maximum); e != nil {
					return e
				}
			} else {
				v3TracePhase(ctx, "commit_event")
				if e := (v3Journal{db: tx, preparedPolicy: u.policy}).applyChecked(ctx, b, last.Number.Uint64(), first.ParentHash, last.Hash(), u.logs, u.observed); e != nil {
					return e
				}
			}
		}
		v3TracePhase(ctx, "commit_cursor_verify")
		published, e = readV3ConsumerCursor(tx, b, false)
		if e != nil || published.CursorBlock != preparedEnd || published.CursorHash != strings.ToLower(targetHash.Hex()) || published.Revision < original.Revision || published.Revision-original.Revision != preparedEnd-original.CursorBlock {
			return errV3Stream
		}
		v3TracePhase(ctx, "commit_consumer_metadata")
		r := tx.Exec(`INSERT INTO ido_v3_proxy_consumer(chain_id,proxy_address,contract_version,project_id,cursor_block,cursor_hash,revision,last_success_at) VALUES(?,?,?,?,?,?,?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE cursor_block=VALUES(cursor_block),cursor_hash=VALUES(cursor_hash),revision=VALUES(revision),last_success_at=VALUES(last_success_at)`, append(s.args(), b.ProjectID, published.CursorBlock, published.CursorHash, published.Revision)...)
		if r.Error == nil && ctx.Err() != nil {
			return ctx.Err()
		}
		if r.Error == nil {
			v3TracePhase(ctx, "commit_finalize")
		}
		return r.Error
	})
	e = dbFailures.finish(e)
	if e != nil {
		return e
	}
	v3TracePhase(ctx, "postcommit_anchor")
	final := &v3TrackedRPC{v3ScannerRPC: rpc}
	check := v3StreamAnchor(ctx, final, s, published.CursorBlock, targetHash)
	if check == nil {
		check = v3StreamAnchor(ctx, final, s, before.CursorBlock, common.HexToHash(before.CursorHash))
	}
	if check == nil {
		if err := verifyV3ProjectClosure(ctx, db, rpc, s, b, rt, published, before); err != nil {
			return &v3ScanDeferred{stage: "closure_check_unavailable", cause: err}
		}
		if err := publishV3ProjectFreshness(ctx, db, s, b, published, before); err != nil {
			return &v3ScanDeferred{stage: "freshness_publication_unavailable", cause: err}
		}
		v3TraceConsumerProgress(ctx, original.CursorBlock, published.CursorBlock, before.CursorBlock)
		logV3ConsumerProgress(ctx, b.ProjectID)
		return nil
	}
	if e := final.finish(ctx, "postcommit_anchor", check); v3Deferred(e) {
		return e
	}
	logV3ScanFailure(ctx, b.ProjectID, check, "persistent_revoke")
	cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if e := v3StreamWriterTx(cleanup, db, func(tx *gorm.DB) error {
		if _, e := v3StreamCursorRead(tx, s, true); e != nil {
			return e
		}
		return (v3Journal{db: tx}).haltRangeUnknown(cleanup, b, published.CursorBlock, published.Revision)
	}); e != nil {
		logV3ScanFailure(ctx, b.ProjectID, e, "revoke_failed")
		return errV3StreamConflict
	}
	logV3ScanFailure(ctx, b.ProjectID, check, "revoked")
	return errV3Stream
}

func v3PurchaseCount(logs []types.Log, b idov3whitelist.ExpectedProjectBinding) int {
	n := 0
	for _, event := range logs {
		if decoded, e := commonchain.DecodeV3ReplayLog(event, b); e == nil && decoded.Purchase != nil {
			n++
		}
	}
	return n
}

func v3ConsumerStreamProgress(before, current v3StreamCursor) bool {
	if before.Mode != "stream" || before.State != "running" || current.Mode != before.Mode || current.State != "running" || current.Epoch != before.Epoch || current.AnchorBlock != before.AnchorBlock || current.CursorBlock < before.CursorBlock || current.Revision < before.Revision {
		return false
	}
	if current.CursorBlock == before.CursorBlock {
		return current.CursorHash == before.CursorHash && current.Revision == before.Revision
	}
	return current.Revision > before.Revision && current.Revision-before.Revision <= current.CursorBlock-before.CursorBlock
}

func prepareV3ConsumerEmpty(ctx context.Context, rpc v3CoverageRPC, s v3StreamScope, b idov3whitelist.ExpectedProjectBinding, rt idov3whitelist.RuntimeIdentity, start, end uint64) (v3ConsumerUnit, error) {
	var zero v3ConsumerUnit
	if start == 0 || end < start || end-start >= 128 {
		return zero, errV3Stream
	}
	if e := v3StreamAnchor(ctx, rpc.v3ScannerRPC, s, end, rpc.blocks[end].header().Hash()); e != nil {
		return zero, e
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(b.ChainID), ProjectID: new(big.Int).SetUint64(b.ProjectID), ProxyAddress: b.ProxyAddress}
	first, e := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, rt, start, rpc.blocks[start].header().Hash())
	if e != nil || idov3whitelist.MatchProjectIdentityObservation(b, first) != nil {
		return zero, errV3Stream
	}
	last, e := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, rt, end, rpc.blocks[end].header().Hash())
	if e != nil || idov3whitelist.MatchProjectIdentityObservation(b, last) != nil {
		return zero, errV3Stream
	}
	first.BlockNumber, first.BlockHash = 0, common.Hash{}
	last.BlockNumber, last.BlockHash = 0, common.Hash{}
	if first != last {
		return zero, errV3Stream
	}
	p, e := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, rt, start, rpc.blocks[start].header().Hash())
	if e != nil {
		return zero, e
	}
	q, e := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, rt, end, rpc.blocks[end].header().Hash())
	if e != nil || p.Policy != q.Policy {
		return zero, errV3Stream
	}
	u := v3ConsumerUnit{observed: last, empty: true}
	for n := start; n <= end; n++ {
		if len(rpc.blocks[n].Logs) != 0 {
			return zero, errV3Stream
		}
		u.headers = append(u.headers, types.CopyHeader(rpc.blocks[n].header()))
	}
	if e := v3StreamAnchor(ctx, rpc.v3ScannerRPC, s, end, rpc.blocks[end].header().Hash()); e != nil {
		return zero, e
	}
	return u, nil
}

func prepareV3ConsumerBlock(ctx context.Context, rpc v3CoverageRPC, s v3StreamScope, b idov3whitelist.ExpectedProjectBinding, rt idov3whitelist.RuntimeIdentity, height uint64) (v3ConsumerUnit, error) {
	var zero v3ConsumerUnit
	var full []types.Log
	v3TracePhase(ctx, "event_scan_receipts")
	parent, hash, logs, e := v3ScanBlock(ctx, rpc, b, rt, height, s.Confirmations, &full)
	if e != nil {
		return zero, e
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(b.ChainID), ProjectID: new(big.Int).SetUint64(b.ProjectID), ProxyAddress: b.ProxyAddress}
	v3TraceBlock(ctx, height, len(full), len(logs), v3PurchaseCount(logs, b))
	v3TracePhase(ctx, "event_project_recheck")
	observed, e := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, rt, height, hash)
	if e != nil || idov3whitelist.MatchProjectIdentityObservation(b, observed) != nil {
		return zero, errV3Stream
	}
	v3TracePhase(ctx, "event_filter_recheck")
	if e := v3RecheckCompleteFilter(ctx, rpc, b, height, full); e != nil {
		return zero, e
	}
	v3TracePhase(ctx, "event_receipt_recheck")
	for _, l := range logs {
		event, e := commonchain.DecodeV3ReplayLog(l, b)
		if e != nil {
			return zero, e
		}
		receipt, e := rpc.TransactionReceipt(ctx, l.TxHash)
		if e != nil {
			return zero, e
		}
		switch {
		case event.Created != nil:
			e = commonchain.MatchV3CreatedReceipt(event.Created, b, receipt)
		case event.Purchase != nil:
			e = commonchain.MatchV3PurchaseReceipt(event.Purchase, commonchain.V3PurchaseBinding{ChainID: b.ChainID, ProxyAddress: b.ProxyAddress, ProjectID: b.ProjectID, PaymentToken: b.PaymentToken, ProjectToken: b.ProjectToken}, receipt)
		case event.Lifecycle != nil:
			e = commonchain.MatchV3LifecycleReceipt(event.Lifecycle, b, receipt)
		default:
			return zero, errV3Stream
		}
		if e != nil {
			return zero, e
		}
	}
	var policy []*commonchain.V3PolicyEvent
	if v3PolicyJournalEnabled() {
		v3TracePhase(ctx, "event_policy_prepare")
		policy, e = prepareV3PolicyFacts(ctx, rpc, b, rt, height, hash)
		if e != nil {
			return zero, e
		}
	}
	if rpc.blocks[height].header().ParentHash != parent {
		return zero, errV3Stream
	}
	v3TracePhase(ctx, "event_final_anchor")
	if e := v3StreamAnchor(ctx, rpc.v3ScannerRPC, s, height, hash); e != nil {
		return zero, e
	}
	return v3ConsumerUnit{headers: []*types.Header{types.CopyHeader(rpc.blocks[height].header())}, observed: observed, logs: v3CopyLogs(logs), policy: policy}, nil
}
