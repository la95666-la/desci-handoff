package idov3whitelist

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"joincare_server/pkg/common/entity"
)

// A project's certificate is usable for five minutes from verified_at. Normal
// rescans never slide that deadline; only verified atomic publication does.
const ProjectFreshnessMaxAge = 5 * time.Minute
const SourceFreshnessMaxAge = 90 * time.Second

// About five minutes at the observed 2.2 blocks/second. This is a block
// allowance, NOT an elapsed-time guarantee; the independent 300s TTL applies.
const ProjectFreshnessMaxLag uint64 = 660

// Public production must remain close to chain head. Never derive these
// independent source bounds from the more tolerant per-project allowance.
const SourceFreshnessMaxConfirmedLag uint64 = 120
const SourceFreshnessMaxHeadLag uint64 = 135

func ProjectFreshnessEnabled() bool { return viper.GetBool("idoV3ProjectFreshness.enabled") }

// Exact approval/original creation identity, not a cache or proof grant. Cursor
// is excluded from the identity digest and separately matched byte-for-byte.
func ReadProjectFreshnessIdentity(ctx context.Context, db *gorm.DB, id uint64) (ApprovedLockedCandidate, string, error) {
	invalid := errors.New("V3 project freshness identity unavailable")
	if ctx == nil || ctx.Err() != nil || db == nil || id == 0 {
		return ApprovedLockedCandidate{}, "", invalid
	}
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	c, err := ReadApprovedLockedCandidate(ctx, db, id)
	if err != nil {
		return c, "", invalid
	}
	var rows []entity.ChainTxSubmission
	read := db.WithContext(ctx).Raw(`SELECT * FROM chain_tx_submission WHERE id=? AND project_id=? AND chain_id=? AND BINARY action=BINARY 'ido_v3_create' AND BINARY state=BINARY 'applied' AND active_slot IS NULL AND BINARY contract_address=BINARY ? LIMIT 2`, c.SubmissionID, id, c.Binding.ChainID, c.Record.Identity.Proxy).Scan(&rows)
	if read.Error != nil || len(rows) != 1 {
		return c, "", invalid
	}
	raw, err := json.Marshal(struct {
		Record                    ApprovedSetRecord
		Binding                   ExpectedProjectBinding
		Runtime                   RuntimeIdentity
		Submission, CreationBlock uint64
		CreationHash, CreationTx  common.Hash
		CreationLog               uint
		CreationPayload           string
		Row                       entity.ChainTxSubmission
		RawTx, CallData           []byte
		SignedAt                  *time.Time
	}{c.Record, c.Binding, c.Runtime, c.SubmissionID, c.CreationBlock, c.CreationHash, c.CreationTx, c.CreationLog, c.CreationPayload, rows[0], rows[0].RawTx, rows[0].CallData, rows[0].V3SignedAt})
	if err != nil {
		return c, "", invalid
	}
	digest := sha256.Sum256(raw)
	return c, hex.EncodeToString(digest[:]), nil
}

// Gate for ONE project. Other project scan failures/global round state are
// deliberately absent. Independent caller-side live admission/policy/signing
// checks remain mandatory. SQL UTC controls all freshness timestamps.
func RequireFreshApprovedV3Project(ctx context.Context, db *gorm.DB, id uint64) (resultErr error) {
	defer func() {
		// A gated operation never borrows twelve-hour freshness. Persist a
		// catch-up request and still reject this attempt until normal evidence
		// becomes available. No signing, cursor jump or timestamp renewal.
		if resultErr != nil && ctx != nil && ctx.Err() == nil && db != nil && id > 0 && viper.GetBool("idoV3ProjectTracking.enabled") {
			_ = db.WithContext(ctx).Exec(`UPDATE ido_v3_project_tracking SET wake_block=GREATEST(wake_block,evidence_block,1),next_scan_at=UTC_TIMESTAMP(6),updated_at=UTC_TIMESTAMP(6) WHERE project_id=? AND mode IN ('maintenance','archived')`, id).Error
		}
	}()
	invalid := errors.New("V3 project scanner freshness unavailable")
	if ctx == nil || ctx.Err() != nil || db == nil || id == 0 {
		return invalid
	}
	c, digest, err := ReadProjectFreshnessIdentity(ctx, db, id)
	if err != nil {
		return invalid
	}
	return requireProjectFreshnessSnapshot(ctx, db, id, c, digest)
}

func requireProjectFreshnessSnapshot(ctx context.Context, db *gorm.DB, id uint64, c ApprovedLockedCandidate, digest string) error {
	invalid := errors.New("V3 project scanner freshness unavailable")
	var rows []struct{ ProjectID uint64 }
	read := db.WithContext(ctx).Raw(`SELECT f.project_id FROM ido_v3_project_freshness f
 JOIN ido_v3_source_freshness h ON h.chain_id=f.chain_id AND h.proxy_address=f.proxy_address AND h.contract_version=f.contract_version AND h.epoch=f.epoch AND h.process_token=f.process_token
 JOIN ido_v3_proxy_stream s ON s.chain_id=h.chain_id AND s.proxy_address=h.proxy_address AND s.contract_version=h.contract_version AND s.epoch=h.epoch
 JOIN ido_v3_scan_cursor c ON c.chain_id=f.chain_id AND c.proxy_address=f.proxy_address AND c.contract_version=f.contract_version AND c.project_id=f.project_id AND c.cursor_block=f.cursor_block AND c.cursor_hash=f.cursor_hash AND c.revision=f.cursor_revision
 JOIN ido_v3_project_projection p ON p.chain_id=c.chain_id AND p.proxy_address=c.proxy_address AND p.contract_version=c.contract_version AND p.project_id=c.project_id AND p.cursor_block=c.cursor_block AND p.cursor_hash=c.cursor_hash AND p.revision=c.revision
 JOIN ido_v3_block_checkpoint ck ON ck.chain_id=c.chain_id AND ck.proxy_address=c.proxy_address AND ck.contract_version=c.contract_version AND ck.project_id=c.project_id AND ck.block_number=c.cursor_block AND ck.block_hash=c.cursor_hash AND ck.status='canonical'
 JOIN ido_v3_proxy_coverage cov ON cov.chain_id=s.chain_id AND cov.proxy_address=s.proxy_address AND cov.contract_version=s.contract_version AND cov.epoch=s.epoch AND cov.block_number=f.target_block AND cov.block_hash=f.target_hash
 WHERE f.chain_id=56 AND BINARY f.proxy_address=BINARY ? AND BINARY f.contract_version=BINARY 'ido_v3_stablecoin' AND f.project_id=?
 AND f.evidence_digest=? AND f.cursor_block=? AND BINARY f.cursor_hash=BINARY ? AND f.cursor_revision=?
 AND h.state='healthy' AND s.mode='stream' AND s.state='running' AND c.state='running'
 AND f.verified_at<=UTC_TIMESTAMP(6) AND f.verified_at>=TIMESTAMPADD(SECOND,?,UTC_TIMESTAMP(6))
 AND h.verified_at<=UTC_TIMESTAMP(6) AND h.verified_at>=TIMESTAMPADD(SECOND,?,UTC_TIMESTAMP(6))
 AND s.last_success_at<=UTC_TIMESTAMP(6) AND s.last_success_at>=TIMESTAMPADD(SECOND,?,UTC_TIMESTAMP(6))
 AND f.cursor_block>=f.target_block AND s.cursor_block>=f.cursor_block AND s.cursor_block-f.cursor_block<=?
 AND h.target_block<=s.cursor_block AND s.observed_head>=s.cursor_block AND s.observed_head-s.cursor_block<=?
 AND h.observed_head>=h.target_block AND h.observed_head-h.target_block>=14
 AND NOT EXISTS(SELECT 1 FROM chain_event_reorg_incident r WHERE r.chain_id=f.chain_id AND LOWER(r.contract_address)=f.proxy_address AND r.active_slot=1 AND r.status='detected')
 LIMIT 2`, approvedScanProxy, id, digest, c.CursorBlock, strings.ToLower(c.CursorHash.Hex()), c.CursorRevision,
		-int64(ProjectFreshnessMaxAge/time.Second), -int64(SourceFreshnessMaxAge/time.Second), -int64(SourceFreshnessMaxAge/time.Second),
		ProjectFreshnessMaxLag, SourceFreshnessMaxHeadLag).Scan(&rows)
	if read.Error != nil || len(rows) != 1 || rows[0].ProjectID != id {
		return invalid
	}
	return nil
}

// Compatibility is explicit: disabled means original global behavior; enabled
// means project evidence ONLY, with no fallback on missing schema/evidence.
func RequireApprovedV3ProjectScan(ctx context.Context, db *gorm.DB, id uint64, review bool) error {
	if ProjectFreshnessEnabled() {
		return RequireFreshApprovedV3Project(ctx, db, id)
	}
	if review {
		return RequireRecentApprovedV3ReviewScan(ctx, db)
	}
	return RequireFreshApprovedV3Scan(ctx, db)
}
