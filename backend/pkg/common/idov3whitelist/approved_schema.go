package idov3whitelist

import (
	"errors"

	"gorm.io/gorm"
)

// ApprovedV3SchemaReady checks the immutable evidence table and its fences.
// It is a deployment preflight, not migration execution or admission of any
// individual project. Missing/partially installed schema fails closed.
func ApprovedV3SchemaReady(database *gorm.DB) error {
	invalid := errors.New("V3 approved Set schema unavailable")
	if database == nil {
		return invalid
	}
	var tables int64
	if database.Raw(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
		AND TABLE_NAME IN ('admin_approval','projects','project_tge','ido_v3_runtime',
		'ido_v3_project_binding','ido_v3_set_create_commitment','ido_v3_set_approval_identity',
		'chain_tx_submission','chain_relayer_nonce','ido_v3_scan_cursor',
		'ido_v3_event_journal','ido_v3_block_checkpoint','ido_v3_project_projection',
		'ido_v3_policy_review','ido_v3_policy_review_member','ido_v3_policy_review_decision',
		'ido_v3_policy_review_execution','ido_v3_policy_event_journal',
		'ido_v3_financial_effect','fair_launch_mfa_assertion',
		'ido_v3_approved_scan_health')`).Scan(&tables).Error != nil || tables != 21 {
		return invalid
	}
	var triggers int64
	if database.Raw(`SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
		AND TRIGGER_NAME IN ('jc_v3_approved_set_insert_1024','jc_v3_approved_set_update_1024',
		'jc_v3_approved_set_delete_1024','jc_v3_approved_set_approval_update_1024',
		'jc_v3_approved_set_approval_delete_1024','jc_v3_approved_set_terms_update_1024',
		'jc_v3_approved_tge_insert_1024','jc_v3_approved_tge_update_1024',
		'jc_v3_approved_tge_delete_1024')`).Scan(&triggers).Error != nil || triggers != 9 {
		return invalid
	}
	// A table with the right name and triggers but weaker keys is not the
	// reviewed evidence store. In particular approval_id must be unique and
	// the project/binding/approval foreign keys cannot be omitted. Check the
	// enforced CHECK as well; MySQL can retain a NOT ENFORCED declaration.
	var constraints int64
	if database.Raw(`SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
		WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_SCHEMA=DATABASE()
		AND TABLE_NAME='ido_v3_set_approval_identity' AND (
			(CONSTRAINT_NAME='PRIMARY' AND CONSTRAINT_TYPE='PRIMARY KEY') OR
			(CONSTRAINT_NAME='uk_v3_approved_set_approval' AND CONSTRAINT_TYPE='UNIQUE') OR
			(CONSTRAINT_NAME IN ('fk_v3_approved_set_binding','fk_v3_approved_set_commitment',
				'fk_v3_approved_set_approval') AND CONSTRAINT_TYPE='FOREIGN KEY') OR
			(CONSTRAINT_NAME='chk_v3_approved_set_identity' AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES'))`).Scan(&constraints).Error != nil || constraints != 6 {
		return invalid
	}
	var scanColumns int64
	if database.Raw(`SELECT COUNT(*) FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='ido_v3_approved_scan_health'
		AND COLUMN_NAME IN ('started_at','last_success_at','generation','binding_count','state')`).Scan(&scanColumns).Error != nil || scanColumns != 5 {
		return invalid
	}
	var scanConstraints int64
	if database.Raw(`SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
		WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_SCHEMA=DATABASE()
		AND TABLE_NAME='ido_v3_approved_scan_health' AND (
			(CONSTRAINT_NAME='PRIMARY' AND CONSTRAINT_TYPE='PRIMARY KEY') OR
			(CONSTRAINT_NAME='fk_v3_approved_scan_runtime' AND CONSTRAINT_TYPE='FOREIGN KEY') OR
			(CONSTRAINT_NAME='chk_v3_approved_scan_health' AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES'))`).Scan(&scanConstraints).Error != nil || scanConstraints != 3 {
		return invalid
	}
	// Switching scanner sources while a legacy signed-Set binding lacks an
	// approved identity would either strand it or halt its published proof.
	// Require an explicit, reviewed migration for every existing binding.
	var legacy int64
	if database.Raw(`SELECT COUNT(*) FROM ido_v3_project_binding b
		LEFT JOIN ido_v3_set_approval_identity i ON i.project_id=b.project_id
		 AND i.chain_id=b.chain_id AND BINARY i.proxy_address=BINARY b.proxy_address
		 AND BINARY i.contract_version=BINARY b.contract_version
		WHERE b.chain_id=56 AND BINARY b.proxy_address=BINARY ?
		 AND BINARY b.contract_version=BINARY 'ido_v3_stablecoin' AND i.project_id IS NULL`,
		"0x0d4040875742975269c55e16c8884d3ee9510b8e").Scan(&legacy).Error != nil || legacy != 0 {
		return invalid
	}
	return nil
}
