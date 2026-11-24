// Package projecttracking owns off-chain scheduling/closure metadata, never
// contract lifecycle, financial balances, signing or transaction authority.
package projecttracking

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/spf13/viper"
	"gorm.io/gorm"
)

const MaintenanceInterval = 12 * time.Hour

func Enabled() bool { return viper.GetBool("idoV3ProjectTracking.enabled") }

type Row struct {
	ProjectID        string     `gorm:"column:project_id" json:"project_id"`
	Mode             string     `json:"mode"`
	Finished         bool       `json:"finished"`
	FinishedAt       *time.Time `json:"finished_at"`
	EvidenceEpoch    uint64     `json:"evidence_epoch"`
	EvidenceBlock    uint64     `json:"evidence_block"`
	EvidenceHash     string     `json:"evidence_hash"`
	EvidenceRevision uint64     `json:"evidence_revision"`
	EvidenceDigest   string     `json:"-"`
	PendingReason    string     `json:"pending_reason"`
	NextScanAt       *time.Time `json:"next_scan_at"`
	LastScanAt       *time.Time `json:"last_scan_at"`
	WakeBlock        uint64     `json:"wake_block"`
	Revision         uint64     `json:"revision"`
	Due              bool       `gorm:"column:due" json:"due"`
	EvidenceCurrent  bool       `gorm:"column:evidence_current" json:"evidence_current"`
	Urgent           bool       `gorm:"column:urgent" json:"-"`
	Unresolved       bool       `gorm:"column:unresolved" json:"-"`
	RecoveryRequired bool       `gorm:"column:recovery_required" json:"-"`
}

// Stored tracking DATETIME values are UTC wall-clock, irrespective of the
// application's parseTime loc=Local. Emit UTC explicitly, not a false +08:00.
func (r Row) MarshalJSON() ([]byte, error) {
	type alias Row
	stamp := func(t *time.Time) *string {
		if t == nil {
			return nil
		}
		s := t.Format("2006-01-02T15:04:05.999999") + "Z"
		return &s
	}
	return json.Marshal(struct {
		alias
		FinishedAt *string `json:"finished_at"`
		NextScanAt *string `json:"next_scan_at"`
		LastScanAt *string `json:"last_scan_at"`
	}{alias(r), stamp(r.FinishedAt), stamp(r.NextScanAt), stamp(r.LastScanAt)})
}

// SQL UTC determines scheduling. Never interpret a UTC bare DATETIME as the
// application's local wall clock to decide whether twelve hours have elapsed.
func Read(ctx context.Context, db *gorm.DB, id uint64, lock bool) (Row, error) {
	var rows []Row
	q := `SELECT t.*, (t.next_scan_at IS NULL OR t.next_scan_at<=UTC_TIMESTAMP(6)) AS due,
 (s.chain_id IS NULL OR s.state<>'running' OR s.mode<>'stream' OR c.project_id IS NULL OR c.state<>'running' OR (t.evidence_epoch>0 AND t.evidence_epoch<>s.epoch)
 OR EXISTS(SELECT 1 FROM chain_event_reorg_incident r WHERE r.chain_id=t.chain_id AND r.active_slot=1 AND r.status='detected')) AS recovery_required,
 (s.state='running' AND s.mode='stream' AND t.evidence_epoch=s.epoch AND c.state='running' AND t.evidence_block=c.cursor_block AND t.evidence_revision=c.revision AND BINARY t.evidence_hash=BINARY c.cursor_hash AND NOT EXISTS(SELECT 1 FROM chain_event_reorg_incident r WHERE r.chain_id=t.chain_id AND r.active_slot=1 AND r.status='detected')) AS evidence_current,
 (EXISTS(SELECT 1 FROM chain_tx_submission sub WHERE sub.project_id=t.project_id AND sub.chain_id=t.chain_id AND (sub.active_slot=1 OR sub.state IN ('signed','submitted','broadcast_unknown','mined_success','mined_revert','final_success','ambiguous')))
 OR EXISTS(SELECT 1 FROM chain_relayer_nonce n JOIN chain_tx_submission sub ON sub.id=n.active_submission_id WHERE sub.project_id=t.project_id AND n.chain_id=t.chain_id)
 OR EXISTS(SELECT 1 FROM chain_event_failure f WHERE LOWER(f.contract_address)=t.proxy_address AND f.contract_version=t.contract_version AND f.status='dead_letter')
 OR EXISTS(SELECT 1 FROM projects p WHERE p.project_id=t.project_id AND p.status IN ('Pending','Active'))) AS urgent
 ,(EXISTS(SELECT 1 FROM chain_tx_submission sub WHERE sub.project_id=t.project_id AND sub.chain_id=t.chain_id AND (sub.active_slot=1 OR sub.state IN ('signed','submitted','broadcast_unknown','mined_success','mined_revert','final_success','ambiguous')))
 OR EXISTS(SELECT 1 FROM chain_relayer_nonce n JOIN chain_tx_submission sub ON sub.id=n.active_submission_id WHERE sub.project_id=t.project_id AND n.chain_id=t.chain_id)
 OR EXISTS(SELECT 1 FROM chain_event_failure f WHERE LOWER(f.contract_address)=t.proxy_address AND f.contract_version=t.contract_version AND f.status='dead_letter')) AS unresolved
 FROM ido_v3_project_tracking t LEFT JOIN ido_v3_proxy_stream s ON s.chain_id=t.chain_id AND s.proxy_address=t.proxy_address AND s.contract_version=t.contract_version
 LEFT JOIN ido_v3_scan_cursor c ON c.project_id=t.project_id AND c.chain_id=t.chain_id AND c.proxy_address=t.proxy_address AND c.contract_version=t.contract_version WHERE t.project_id=?`
	if lock {
		q += " FOR UPDATE"
	}
	r := db.WithContext(ctx).Raw(q, id).Scan(&rows)
	if r.Error != nil {
		return Row{}, errors.New("V3 tracking state unavailable")
	}
	if len(rows) == 0 {
		return Row{Mode: "normal", Due: true, PendingReason: "not_verified"}, nil
	}
	if len(rows) != 1 {
		return Row{}, errors.New("V3 tracking state ambiguous")
	}
	return rows[0], nil
}

func SchemaReady(ctx context.Context, db *gorm.DB) error {
	var n int64
	if db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN ('ido_v3_project_tracking','ido_v3_tracking_audit')`).Scan(&n).Error != nil || n != 2 {
		return errors.New("V3 tracking migration required")
	}
	if db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN ('jc_v3_tracking_audit_update_1030','jc_v3_tracking_audit_delete_1030')`).Scan(&n).Error != nil || n != 2 {
		return errors.New("V3 tracking audit fences unavailable")
	}
	if db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND ((TABLE_NAME IN ('ido_v3_project_tracking','ido_v3_tracking_audit') AND CONSTRAINT_NAME='PRIMARY') OR CONSTRAINT_NAME IN ('fk_v3_tracking_binding','fk_v3_tracking_audit') OR (CONSTRAINT_NAME='chk_v3_tracking' AND ENFORCED='YES'))`).Scan(&n).Error != nil || n != 5 {
		return errors.New("V3 tracking constraints unavailable")
	}
	return nil
}

func Audit(tx *gorm.DB, id uint64, action string, admin *uint64, reason, before, after string, revision uint64) error {
	return tx.Exec(`INSERT INTO ido_v3_tracking_audit(project_id,action,admin_id,reason,before_mode,after_mode,tracking_revision) VALUES(?,?,?,?,?,?,?)`, id, action, admin, reason, before, after, revision).Error
}

// Caller holds stream->project locks. No metadata alteration unlocks a project,
// changes projects.status or grants a transaction/financial entitlement.
func Ensure(tx *gorm.DB, id uint64) error {
	return tx.Exec(`INSERT INTO ido_v3_project_tracking(project_id,chain_id,proxy_address,contract_version) SELECT b.project_id,b.chain_id,b.proxy_address,b.contract_version FROM ido_v3_project_binding b WHERE b.project_id=? ON DUPLICATE KEY UPDATE project_id=VALUES(project_id)`, id).Error
}

// Includes orphaned active nonce fences and source dead letters: a terminal
// application status alone cannot authorize closure/retirement.
func HasUnresolvedWork(tx *gorm.DB, id, chain uint64, proxy string) (bool, error) {
	var n int64
	err := tx.Raw(`SELECT
 (SELECT COUNT(*) FROM chain_tx_submission WHERE project_id=? AND chain_id=? AND (active_slot=1 OR state IN ('signed','submitted','broadcast_unknown','mined_success','mined_revert','final_success','ambiguous')))
 +(SELECT COUNT(*) FROM chain_relayer_nonce n JOIN chain_tx_submission t ON t.id=n.active_submission_id WHERE t.project_id=? AND n.chain_id=?)
 +(SELECT COUNT(*) FROM chain_event_failure WHERE LOWER(contract_address)=? AND contract_version='ido_v3_stablecoin' AND status='dead_letter')`, id, chain, id, chain, proxy).Scan(&n).Error
	if err != nil {
		return true, errors.New("V3 unresolved work unavailable")
	}
	return n > 0, nil
}
