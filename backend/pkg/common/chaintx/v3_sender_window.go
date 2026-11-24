package chaintx

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
)

// Scheduling/diagnostics ONLY: this snapshot does not authorize signatures or
// exemptions. RequireV3SenderReady and the exact SQL view remain authoritative.
type V3SenderMaintenanceWindow struct {
	EvidenceExpiresAt *time.Time `json:"evidence_expires_at,omitempty"`
	RemainingSeconds  *int64     `json:"evidence_remaining_seconds,omitempty"`
	ActiveSubmission  *uint64    `json:"-"`
	RetiredCount      int        `json:"retired_count"`
}

func ReadV3SenderMaintenanceWindow(ctx context.Context, database *gorm.DB, chain uint64, sender common.Address) (V3SenderMaintenanceWindow, error) {
	var out V3SenderMaintenanceWindow
	if ctx == nil || ctx.Err() != nil || database == nil || chain == 0 || sender == (common.Address{}) {
		return out, errors.New("sender scheduling snapshot unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	started := time.Now()
	err := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var fences []struct{ ActiveSubmissionID *uint64 }
		if err := tx.Raw(`SELECT active_submission_id FROM chain_relayer_nonce WHERE chain_id=? AND BINARY LOWER(sender_address)=BINARY ?`, chain, strings.ToLower(sender.Hex())).Scan(&fences).Error; err != nil || len(fences) != 1 {
			return errors.New("sender scheduling fence unavailable")
		}
		out.ActiveSubmission = fences[0].ActiveSubmissionID
		var rows []struct {
			State *string
			// Parse duration in SQL UTC; never interpret a bare UTC DATETIME
			// through Core's Asia/Shanghai driver Location as an absolute time.
			Remaining *int64 `gorm:"column:remaining"`
		}
		if err := tx.Raw(`SELECT e.state,TIMESTAMPDIFF(SECOND,UTC_TIMESTAMP(6),e.expires_at) AS remaining
 FROM chain_tx_submission s LEFT JOIN ido_v3_retired_nonce_evidence e ON e.id=(
 SELECT x.id FROM ido_v3_retired_nonce_evidence x WHERE x.submission_id=s.id ORDER BY x.id DESC LIMIT 1)
 WHERE s.chain_id=? AND BINARY LOWER(s.sender_address)=BINARY ? AND s.state='v3_reorg_suspended'
 ORDER BY s.id LIMIT 33`, chain, strings.ToLower(sender.Hex())).Scan(&rows).Error; err != nil || len(rows) > 32 {
			return errors.New("sender scheduling evidence unavailable")
		}
		out.RetiredCount = len(rows)
		for _, row := range rows {
			if row.State == nil || *row.State != "valid" || row.Remaining == nil {
				out.RemainingSeconds = nil
				return nil // unknown/revoked is not freshness or authority
			}
			if out.RemainingSeconds == nil || *row.Remaining < *out.RemainingSeconds {
				n := *row.Remaining
				out.RemainingSeconds = &n
			}
		}
		return nil
	})
	if err == nil && out.RemainingSeconds != nil {
		// Anchor at read start, not completion: query latency must not inflate
		// the available runway. This remains only a scheduling hint.
		t := started.UTC().Add(time.Duration(*out.RemainingSeconds) * time.Second)
		out.EvidenceExpiresAt = &t
	}
	return out, err
}
