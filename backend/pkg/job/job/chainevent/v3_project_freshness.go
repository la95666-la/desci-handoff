package chainevent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/log"
)

type v3FreshnessSession struct {
	token string
	ready bool
}
type v3FreshnessKey struct{}

// Private process-owned token. Restart invalidates old project evidence before
// new publication. Missing schema fails closed ONLY when explicitly enabled.
func (j *ApprovedV3ScannerJob) prepareProjectFreshness(ctx context.Context) (context.Context, error) {
	if !idov3whitelist.ProjectFreshnessEnabled() {
		j.freshness = v3FreshnessSession{}
		return ctx, nil
	}
	if j.freshness.token == "" {
		var raw [32]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return ctx, err
		}
		j.freshness.token = hex.EncodeToString(raw[:])
	}
	if !j.freshness.ready {
		if err := v3ProjectFreshnessSchema(ctx, j.Database); err != nil {
			return ctx, err
		}
		scope := productionV3StreamScope()
		err := v3StreamWriterTx(ctx, j.Database, func(tx *gorm.DB) error {
			s, err := v3StreamCursorRead(tx, scope, true)
			if err != nil {
				return err
			}
			head := s.ObservedHead
			if head < s.CursorBlock {
				head = s.CursorBlock
			}
			return tx.Exec(`INSERT INTO ido_v3_source_freshness(chain_id,proxy_address,contract_version,epoch,process_token,state,target_block,target_hash,observed_head,verified_at) VALUES(?,?,?,?,?,'checking',?,?,?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE epoch=VALUES(epoch),process_token=VALUES(process_token),state='checking',verified_at=UTC_TIMESTAMP(6)`, append(scope.args(), s.Epoch, j.freshness.token, s.CursorBlock, s.CursorHash, head)...).Error
		})
		if err != nil {
			return ctx, err
		}
		j.freshness.ready = true
	}
	return context.WithValue(ctx, v3FreshnessKey{}, j.freshness.token), nil
}

func v3ProjectFreshnessSchema(ctx context.Context, db *gorm.DB) error {
	var tables, triggers, constraints int64
	if db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN ('ido_v3_source_freshness','ido_v3_project_freshness')`).Scan(&tables).Error != nil || tables != 2 {
		return errors.New("V3 project freshness schema unavailable")
	}
	if db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN ('jc_v3_fresh_source_insert_1029','jc_v3_fresh_source_update_1029','jc_v3_fresh_project_insert_1029','jc_v3_fresh_project_update_1029')`).Scan(&triggers).Error != nil || triggers != 4 {
		return errors.New("V3 project freshness writer fences unavailable")
	}
	if db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND ((TABLE_NAME IN ('ido_v3_source_freshness','ido_v3_project_freshness') AND CONSTRAINT_NAME='PRIMARY' AND CONSTRAINT_TYPE='PRIMARY KEY') OR (CONSTRAINT_NAME IN ('fk_v3_fresh_source','fk_v3_fresh_project') AND CONSTRAINT_TYPE='FOREIGN KEY') OR (CONSTRAINT_NAME IN ('chk_v3_fresh_source','chk_v3_fresh_project') AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES'))`).Scan(&constraints).Error != nil || constraints != 6 {
		return errors.New("V3 project freshness constraints unavailable")
	}
	return nil
}

// Refresh source health from an independent confirmed RPC anchor each round.
// On source failure mark checking immediately, without freshening project rows.
func markV3FreshSource(ctx context.Context, db *gorm.DB, scope v3StreamScope, target v3SharedConfirmed, head uint64, healthy bool) error {
	token, ok := ctx.Value(v3FreshnessKey{}).(string)
	if !ok {
		return nil
	}
	return v3StreamWriterTx(ctx, db, func(tx *gorm.DB) error {
		s, err := v3StreamCursorRead(tx, scope, true)
		if err != nil {
			return err
		}
		if healthy && (s.CursorBlock < target.height || s.CursorBlock-target.height > idov3whitelist.SourceFreshnessMaxConfirmedLag) {
			healthy = false
		}
		state := "checking"
		if healthy && s.State == "running" && s.Mode == "stream" {
			state = "healthy"
		}
		r := tx.Exec(`UPDATE ido_v3_source_freshness SET epoch=?,state=?,target_block=?,target_hash=?,observed_head=?,verified_at=UTC_TIMESTAMP(6) WHERE chain_id=? AND proxy_address=? AND contract_version=? AND process_token=?`, append([]any{s.Epoch, state, target.height, strings.ToLower(target.hash.Hex()), head}, append(scope.args(), token)...)...)
		if r.Error != nil || r.RowsAffected != 1 {
			return errors.New("V3 project freshness process changed")
		}
		return nil
	})
}

func invalidateV3ProjectFreshness(ctx context.Context, db *gorm.DB, scope v3StreamScope, id uint64) error {
	token, ok := ctx.Value(v3FreshnessKey{}).(string)
	if !ok {
		return nil
	}
	return db.WithContext(ctx).Exec(`DELETE FROM ido_v3_project_freshness WHERE chain_id=? AND proxy_address=? AND contract_version=? AND project_id=? AND process_token=?`, append(scope.args(), id, token)...).Error
}

// Normal rescans and inconclusive/transient failures leave the last verified
// row untouched: no deletion, success refresh or TTL extension. Definite
// identity/evidence conflicts invalidate immediately, even with a cancelled
// project context. Exact cursor/revision/epoch/process checks at readers still
// close any postcommit publication gap without needing an eager DELETE.
func finishV3ProjectRescan(ctx context.Context, db *gorm.DB, scope v3StreamScope, id uint64, result error) error {
	if result == nil || v3Deferred(result) {
		return result
	}
	cleanup, cancel := freshnessCleanupContext(ctx)
	defer cancel()
	if err := invalidateV3ProjectFreshness(cleanup, db, scope, id); err != nil {
		// Retain the original failure class so integrity revocation cannot be
		// converted into a harmless retry by an invalidation I/O failure.
		return fmt.Errorf("%w: project freshness invalidation unavailable", result)
	}
	return result
}

// Called only after verified postcommit anchors. The financial transaction has
// already committed; failure here removes availability, never rolls back or
// fabricates financial success. Publication locks and rechecks exact live state.
func publishV3ProjectFreshness(ctx context.Context, db *gorm.DB, scope v3StreamScope, b idov3whitelist.ExpectedProjectBinding, published v3ConsumerCursor, before v3StreamCursor) error {
	token, ok := ctx.Value(v3FreshnessKey{}).(string)
	if !ok {
		return nil
	}
	grant, ok := ctx.Value(v3AuditGrantKey{}).(v3AuditEntry)
	if !ok {
		return errors.New("V3 project freshness requires scanner audit")
	}
	target := v3SharedConfirmed{height: before.CursorBlock, hash: common.HexToHash(before.CursorHash)}
	if target.height == 0 || target.hash == (common.Hash{}) {
		return errors.New("V3 project freshness target unavailable")
	}
	if published.CursorBlock < target.height {
		return nil
	} // backfill progress != fresh
	err := v3StreamWriterTx(ctx, db, func(tx *gorm.DB) error {
		s, err := v3StreamCursorRead(tx, scope, true)
		if err != nil {
			return err
		}
		if !v3ConsumerStreamProgress(before, s) {
			return &v3ScanDeferred{stage: "freshness_stream_changed"}
		}
		cursor, err := readV3ConsumerCursor(tx, b, true)
		if err != nil || cursor != published {
			return &v3ScanDeferred{stage: "freshness_cursor_changed"}
		}
		if err := validateV3ScannerAuditGrant(ctx, grant, b, s, func() (v3AuditSnapshot, error) { return readV3AuditSnapshot(ctx, tx, b.ProjectID) }); err != nil {
			return err
		}
		if _, err := readV3PublishedProjection(ctx, tx, b); err != nil {
			return err
		}
		candidate, digest, err := idov3whitelist.ReadProjectFreshnessIdentity(ctx, tx, b.ProjectID)
		if err != nil || candidate.CursorBlock != published.CursorBlock || candidate.CursorRevision != published.Revision || candidate.CursorHash != common.HexToHash(published.CursorHash) {
			return errors.New("V3 freshness identity changed")
		}
		coverage, err := readV3Coverage(tx, scope, target.height, target.height)
		if err != nil || coverage[target.height].header().Hash() != target.hash {
			return errors.New("V3 freshness target changed")
		}
		var owners []struct {
			Token string `gorm:"column:process_token"`
			Epoch uint64
			State string
		}
		r := tx.Raw(`SELECT process_token,epoch,state FROM ido_v3_source_freshness WHERE chain_id=? AND proxy_address=? AND contract_version=? FOR UPDATE`, scope.args()...).Scan(&owners)
		if r.Error != nil || len(owners) != 1 || owners[0].Token != token || owners[0].Epoch != s.Epoch || owners[0].State != "healthy" {
			return errors.New("V3 freshness source unavailable")
		}
		return tx.Exec(`INSERT INTO ido_v3_project_freshness(chain_id,proxy_address,contract_version,project_id,epoch,process_token,cursor_block,cursor_hash,cursor_revision,target_block,target_hash,evidence_digest,verified_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE epoch=VALUES(epoch),process_token=VALUES(process_token),cursor_block=VALUES(cursor_block),cursor_hash=VALUES(cursor_hash),cursor_revision=VALUES(cursor_revision),target_block=VALUES(target_block),target_hash=VALUES(target_hash),evidence_digest=VALUES(evidence_digest),verified_at=UTC_TIMESTAMP(6)`, append(scope.args(), b.ProjectID, s.Epoch, token, published.CursorBlock, published.CursorHash, published.Revision, target.height, strings.ToLower(target.hash.Hex()), digest)...).Error
	})
	if err == nil {
		if logger := log.GetLogger(); logger != nil {
			logger.WithFields(map[string]any{"project_id": fmt.Sprintf("%d", b.ProjectID), "cursor": published.CursorBlock, "target": target.height, "max_age_seconds": int64(idov3whitelist.ProjectFreshnessMaxAge / time.Second), "max_lag_blocks": idov3whitelist.ProjectFreshnessMaxLag}).Info("V3 project freshness verified")
		}
	}
	return err
}

func freshnessCleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}
