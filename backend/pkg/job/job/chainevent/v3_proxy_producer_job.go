package chainevent

import (
	"context"
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/common/projecttracking"
	"time"
)

// Producer has its own bounded lane; a slow project's original-create audit
// must not reduce public backfill to a single 12s slice per 50s consumer tick.
// DB CAS/stream revision remains the cross-process writer fence.
func (j *ApprovedV3ScannerJob) runProxyProducer() {
	if !j.producerMu.TryLock() {
		return
	}
	defer j.producerMu.Unlock()
	parent := j.Context
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	ctx = newV3ScanTrace(ctx, 0)
	if projecttracking.Enabled() {
		if err := projecttracking.SchemaReady(ctx, j.Database); err != nil {
			if logger := log.GetLogger(); logger != nil {
				logger.Warn("V3 tracking schema unavailable")
			}
			return
		}
	}
	observed := false
	defer func() {
		if !observed && idov3whitelist.ProjectFreshnessEnabled() {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = v3StreamWriterTx(cleanup, j.Database, func(tx *gorm.DB) error {
				return tx.Exec(`UPDATE ido_v3_source_freshness SET state='checking' WHERE chain_id=? AND proxy_address=? AND contract_version=?`, productionV3StreamScope().args()...).Error
			})
		}
	}()
	v3TracePhase(ctx, "producer_rpc_setup")
	rpc, e := chainrpc.DialScanner(ctx, j.RPCURL)
	if e != nil {
		return
	}
	defer rpc.Close()
	s := productionV3StreamScope()
	c, e := v3StreamCursorRead(j.Database.WithContext(ctx), s, false)
	if e != nil {
		var count int64
		if j.Stream.Mode != "shadow" || j.Database.WithContext(ctx).Raw(`SELECT COUNT(*) FROM ido_v3_proxy_stream WHERE chain_id=? AND proxy_address=? AND contract_version=?`, s.args()...).Scan(&count).Error != nil || count != 0 {
			return
		}
		e = bootstrapV3Stream(ctx, j.Database, rpc, s, "shadow", j.Stream.Anchor, j.Stream.AnchorHash)
		if e != nil {
			return
		}
		c, e = v3StreamCursorRead(j.Database.WithContext(ctx), s, false)
	}
	if e != nil || c.State != "running" || c.Mode != j.Stream.Mode {
		return
	}
	v3TraceWindow(ctx, c.CursorBlock, c.Revision, c.Epoch, c.CursorBlock+1, c.CursorBlock)
	if c.Epoch == 1 {
		anchor, e := readV3Coverage(j.Database.WithContext(ctx), s, j.Stream.Anchor, j.Stream.Anchor)
		if e != nil || anchor[j.Stream.Anchor].header().Hash() != j.Stream.AnchorHash {
			return
		}
	}
	v3TracePhase(ctx, "producer_collect_or_commit")
	n, e := produceV3StreamBudget(ctx, j.Database, rpc, s, &j.producerBudget)
	if e != nil {
		decision := "producer_revalidate"
		if v3Deferred(e) {
			decision = "defer"
		}
		logV3ScanFailure(ctx, 0, e, decision)
		if v3Deferred(e) {
			// A failed producer observation cannot leave a newly advertised full
			// scan success. This never advances success time or rewrites history.
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			invalidateErr := j.Database.WithContext(cleanup).Exec(`UPDATE ido_v3_approved_scan_health SET state='checking',generation=generation+1 WHERE chain_id=? AND proxy_address=? AND contract_version=?`, s.args()...).Error
			cancel()
			if logger := log.GetLogger(); logger != nil {
				logger.WithFields(map[string]any{"blocks": n, "failure_class": "deferred", "health_invalidation_failed": invalidateErr != nil}).Warn("V3 producer observation incomplete; retained durable prefix")
			}
			return
		}
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		// Benign budget exhaustion keeps immutable committed progress but must
		// re-establish the live tip independently, never infer "healthy".
		current, readErr := v3StreamCursorRead(j.Database.WithContext(cleanup), s, false)
		temporary := !errors.Is(e, errV3StreamConflict) && (errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(e, errV3UnitDeadline))
		if !(temporary && readErr == nil && v3StreamAnchor(cleanup, rpc, s, current.CursorBlock, common.HexToHash(current.CursorHash)) == nil) {
			if c.Mode == "stream" {
				logV3ScanFailure(ctx, 0, e, "persistent_stream_halt")
				if haltErr := haltV3Stream(cleanup, j.Database, s); haltErr != nil {
					logV3ScanFailure(ctx, 0, haltErr, "stream_halt_failed")
				} else {
					logV3ScanFailure(ctx, 0, e, "stream_halted")
				}
			} else {
				_ = j.Database.WithContext(cleanup).Exec(`UPDATE ido_v3_proxy_stream SET state='halted',revision=revision+1 WHERE chain_id=? AND proxy_address=? AND contract_version=? AND state='running'`, s.args()...).Error
			}
		}
	}
	observed = e == nil
	if logger := log.GetLogger(); logger != nil {
		logger.WithFields(map[string]any{"blocks": n, "batch": j.producerBudget.size(), "failed": e != nil}).Info("V3 public producer progress")
	}
}
