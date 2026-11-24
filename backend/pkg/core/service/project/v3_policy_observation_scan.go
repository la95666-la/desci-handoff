package project

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

// Reject known recovery conflicts; this does not certify policy or finances.
// Policy callers separately authenticate current root/version, complete member
// snapshots, approved reviews and exact Safe receipts. Never wait on unrelated
// project financial consumption or consumer->certificate publication here.
func requireV3PolicyEvidenceStore(ctx context.Context, database *gorm.DB, id uint64) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	invalid := errors.New("V3 policy evidence store unavailable")
	if ctx == nil || database == nil || id == 0 {
		return invalid
	}
	var rows []struct{ ProjectID uint64 }
	r := database.WithContext(ctx).Raw(`SELECT c.project_id FROM ido_v3_scan_cursor c
 WHERE c.chain_id=56 AND BINARY c.proxy_address=BINARY ? AND c.project_id=?
 AND BINARY c.contract_version=BINARY 'ido_v3_stablecoin' AND c.state='running'
 AND NOT EXISTS(SELECT 1 FROM chain_event_reorg_incident r WHERE r.chain_id=c.chain_id AND r.active_slot=1 AND r.status='detected') LIMIT 2`, strings.ToLower(v3TokenProxy), id).Scan(&rows)
	if r.Error != nil || len(rows) != 1 || rows[0].ProjectID != id {
		return invalid
	}
	return nil
}
