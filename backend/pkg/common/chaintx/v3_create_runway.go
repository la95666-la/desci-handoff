package chaintx

import (
	"context"
	"joincare_server/pkg/common/v3diagnostic"
	"time"
)

// Initial admission covers the complete 150s create request plus 30s margin.
// Signing needs a fresh 90s runway for post-sign checks and first broadcast.
// Neither reserve grants permission to broadcast, nor changes the approved
// deadline, receipt admission or the separate sender evidence reserves.
const V3CreateRequestReserve = 180 * time.Second
const V3CreateSigningReserve = 90 * time.Second

const V3CreatePostSignBudget = 60 * time.Second

func requireV3CreateSigningBudget(ctx context.Context, now time.Time) error {
	if ctx == nil || ctx.Err() != nil {
		return v3diagnostic.New("CHECK_TIMEOUT", "prepare_signature", nil)
	}
	if deadline, ok := ctx.Deadline(); !ok || !now.Add(V3CreatePostSignBudget).Before(deadline) {
		return v3diagnostic.New("CHECK_TIMEOUT", "prepare_signature", nil)
	}
	return nil
}

func RequireV3CreateRunway(now time.Time, deadline int64, reserve time.Duration, stage string) error {
	if deadline <= 1 || !now.Before(time.Unix(deadline, 0)) {
		return v3diagnostic.New("CREATE_DEADLINE_EXPIRED", stage, nil)
	}
	if reserve < 0 || !now.Add(reserve).Before(time.Unix(deadline, 0)) {
		return v3diagnostic.New("CREATE_RUNWAY_INSUFFICIENT", stage, nil)
	}
	return nil
}

// Shared by production signing and the disposable MySQL test. DB UTC, not
// DSN timezone or an earlier API read, is authoritative for persistence.
const v3PersistSignedSQL = `UPDATE chain_tx_submission SET state='signed',nonce=?,gas_limit=?,gas_price_raw=?,
	raw_tx=?,tx_hash=?,v3_signed_at=UTC_TIMESTAMP(6),updated_at=? WHERE id=? AND state='ready'
	AND active_slot=1 AND nonce IS NULL AND raw_tx IS NULL AND tx_hash IS NULL
	AND UTC_TIMESTAMP(6) < ?`
