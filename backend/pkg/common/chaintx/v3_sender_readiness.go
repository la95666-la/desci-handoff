package chaintx

import (
	"context"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/v3diagnostic"
)

const SenderPreflightReserve = 180 * time.Second
const SenderSigningReserve = 30 * time.Second

// Refresh proactively while requests still have 180s runway. A 30s scheduler
// cadence plus <=90s audit must not deliberately wait for request rejection.
const SenderMaintenanceReserve = 270 * time.Second

// Read-only, DB-only readiness, before any intent or expensive create audit.
// A second current locking check runs inside signing. This is not a nonce
// allocation, exemption refresh, or permission to ignore an unknown attempt.
func RequireV3SenderReady(ctx context.Context, db *gorm.DB, chain uint64, sender common.Address) error {
	if ctx == nil || ctx.Err() != nil || db == nil || chain == 0 || sender == (common.Address{}) {
		return v3diagnostic.New("SENDER_EVIDENCE_UNAVAILABLE", "sender_readiness", nil)
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Same global linearization point as signing/broadcast/maintenance.
		// A fast preflight must not invert suspended-row vs nonce lock order.
		if chainreorg.LockNoActiveIncidentTx(tx, chain, "0x0d4040875742975269c55e16c8884d3ee9510b8e") != nil {
			return v3diagnostic.New("SENDER_EVIDENCE_UNAVAILABLE", "sender_readiness", nil)
		}
		// This is advisory DB readiness; signing and first broadcast independently
		// verify confirmed/latest/pending nonce. Never let another project's
		// financial recovery TTL decide whether this project can operate.
		if err := senderIsolationSchemaReady(tx); err != nil {
			return v3diagnostic.New("SENDER_EVIDENCE_UNAVAILABLE", "sender_readiness", err)
		}
		var fences []entity.ChainRelayerNonce
		if tx.Raw(`SELECT * FROM chain_relayer_nonce WHERE chain_id=? AND BINARY LOWER(sender_address)=BINARY ? FOR UPDATE`, chain, strings.ToLower(sender.Hex())).Scan(&fences).Error != nil || len(fences) > 1 {
			return v3diagnostic.New("SENDER_EVIDENCE_UNAVAILABLE", "sender_readiness", nil)
		}
		if len(fences) == 1 && fences[0].ActiveSubmissionID != nil {
			return v3diagnostic.New("NONCE_UNRESOLVED", "nonce_check", nil)
		}
		if len(fences) != 1 {
			return v3diagnostic.New("NONCE_UNRESOLVED", "nonce_check", nil)
		}
		return requireConsumedSuspendedTx(tx, chain, sender, fences[0].NextNonce)
	})
}

func requireV3SenderEvidenceTx(ctx context.Context, tx *gorm.DB, chain uint64, sender common.Address, reserve time.Duration) error {
	if ctx == nil || ctx.Err() != nil || tx == nil || chain == 0 || sender == (common.Address{}) {
		return v3diagnostic.New("SENDER_EVIDENCE_UNAVAILABLE", "sender_readiness", nil)
	}
	var rows []struct{ ID uint64 }
	if tx.Raw(`SELECT id FROM chain_tx_submission WHERE chain_id=? AND BINARY LOWER(sender_address)=BINARY ? AND state='v3_reorg_suspended' ORDER BY id FOR UPDATE`, chain, strings.ToLower(sender.Hex())).Scan(&rows).Error != nil {
		return v3diagnostic.New("SENDER_EVIDENCE_UNAVAILABLE", "sender_readiness", nil)
	}
	if len(rows) == 0 {
		return nil
	}
	if !viper.GetBool("idoV3RetiredNonce.enabled") {
		return v3diagnostic.New("SENDER_SUSPENDED", "sender_readiness", nil)
	}
	if retiredNonceSchemaReady(tx) != nil {
		return v3diagnostic.New("SENDER_EVIDENCE_UNAVAILABLE", "sender_readiness", nil)
	}
	for _, row := range rows {
		// Current read, not a repeatable-read snapshot established by an earlier
		// project/approval lookup. Lock latest evidence to serialize revocation.
		var latest []struct {
			ID    uint64
			State string
		}
		if tx.Raw(`SELECT id,state FROM ido_v3_retired_nonce_evidence WHERE submission_id=? ORDER BY id DESC LIMIT 1 FOR UPDATE`, row.ID).Scan(&latest).Error != nil {
			return v3diagnostic.New("SENDER_EVIDENCE_UNAVAILABLE", "sender_readiness", nil)
		}
		if len(latest) != 1 || latest[0].State != "valid" {
			return v3diagnostic.New("SENDER_RETIREMENT_REVIEW_REQUIRED", "sender_readiness", nil)
		}
		var valid []struct{ ID uint64 }
		// Same canonical predicate as the nonce trigger, with additional remaining
		// runway. Never accept a valid-looking row independently of the SQL view.
		if tx.Raw(`SELECT e.id FROM ido_v3_retired_nonce_evidence e JOIN ido_v3_retired_nonce_exemption x ON x.id=e.submission_id WHERE e.id=? AND e.expires_at>TIMESTAMPADD(SECOND,?,UTC_TIMESTAMP(6)) FOR SHARE`, latest[0].ID, int64(reserve/time.Second)).Scan(&valid).Error != nil {
			return v3diagnostic.New("SENDER_EVIDENCE_UNAVAILABLE", "sender_readiness", nil)
		}
		if len(valid) != 1 {
			return v3diagnostic.New("SENDER_EVIDENCE_PENDING", "sender_readiness", nil)
		}
	}
	return nil
}
