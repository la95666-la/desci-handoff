package schema

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

const securityControlPlaneMigration = "2026_08_30_security_control_plane_and_event_fence.sql"
const securityControlPlaneChecksum = "2401bf1d558e125406e59b4bb76e3ad4acb724ba60de8867b979114e25c9b1e7"

const securityControlPlaneColumnsQuery = `SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND (
(TABLE_NAME='admin_users' AND COLUMN_NAME='session_version' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='1') OR
(TABLE_NAME='admin_users' AND COLUMN_NAME='password_changed_at' AND COLUMN_TYPE='datetime' AND IS_NULLABLE='YES') OR
(TABLE_NAME='admin_users' AND COLUMN_NAME='locked_until' AND COLUMN_TYPE='datetime' AND IS_NULLABLE='YES') OR
(TABLE_NAME='admin_users' AND COLUMN_NAME='failed_login_count' AND COLUMN_TYPE='int' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='0') OR
(TABLE_NAME='admin_users' AND COLUMN_NAME='deleted_at' AND COLUMN_TYPE='datetime' AND IS_NULLABLE='YES') OR
(TABLE_NAME='user' AND COLUMN_NAME='is_enable' AND COLUMN_TYPE IN ('tinyint','tinyint(1)') AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='1') OR
(TABLE_NAME='user' AND COLUMN_NAME='session_version' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='1') OR
(TABLE_NAME='user_wallet' AND COLUMN_NAME='verified_at' AND COLUMN_TYPE='datetime(6)' AND IS_NULLABLE='YES') OR
(TABLE_NAME='user_wallet' AND COLUMN_NAME='verification_method' AND COLUMN_TYPE='varchar(32)' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='') OR
(TABLE_NAME='user_wallet' AND COLUMN_NAME='verified_evm_identity' AND COLUMN_TYPE='varchar(42)' AND EXTRA LIKE '%STORED GENERATED%'))`

func RequireSecurityControlPlane(database *gorm.DB) error {
	if database == nil {
		return errors.New("security control-plane schema check requires a database")
	}
	database = primarySchemaDatabase(database)
	var ledger int64
	if err := database.Raw(`SELECT COUNT(*) FROM schema_migrations WHERE status='applied' AND filename=? AND checksum=?
AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE status='applying')`, securityControlPlaneMigration, securityControlPlaneChecksum).Scan(&ledger).Error; err != nil {
		return fmt.Errorf("check security control-plane migration ledger: %w", err)
	}
	if ledger != 1 {
		return fmt.Errorf("security control-plane migration ledger incomplete: got=%d want=1; apply 2026_08_30 before starting core/job", ledger)
	}
	var columns int64
	if err := database.Raw(securityControlPlaneColumnsQuery).Scan(&columns).Error; err != nil {
		return fmt.Errorf("check security control-plane columns: %w", err)
	}
	if columns != 10 {
		return fmt.Errorf("security control-plane columns incomplete: got=%d want=10", columns)
	}
	var guards int64
	if err := database.Raw(`SELECT
(SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet' AND INDEX_NAME='uk_user_wallet_verified_evm' AND NON_UNIQUE=0)+
(SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet' AND CONSTRAINT_NAME='chk_user_wallet_verification' AND ENFORCED='YES')+
(SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN ('trg_chain_event_failure_canonical_insert','trg_chain_event_failure_canonical_update') AND ACTION_STATEMENT LIKE '%canonical chain event cannot%')+
(SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN ('trg_user_wallet_verified_owner_insert','trg_user_wallet_verified_owner_update','trg_user_primary_identity_require_update') AND ACTION_STATEMENT LIKE '%owner claim%')+
(SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME='trg_user_primary_identity_claim_insert' AND ACTION_STATEMENT LIKE '%INSERT INTO user_wallet_identity%')`).Scan(&guards).Error; err != nil {
		return fmt.Errorf("check security control-plane guards: %w", err)
	}
	if guards != 8 {
		return fmt.Errorf("security control-plane guards incomplete: got=%d want=8", guards)
	}
	var identities int64
	if err := database.Raw(`SELECT
(SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet_identity')+
(SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet_identity' AND CONSTRAINT_NAME='chk_user_wallet_identity' AND ENFORCED='YES')+
(SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN ('trg_user_wallet_identity_no_update','trg_user_wallet_identity_no_delete') AND ACTION_STATEMENT LIKE '%non-recyclable%')`).Scan(&identities).Error; err != nil {
		return fmt.Errorf("check wallet identity registry: %w", err)
	}
	if identities != 8 {
		return fmt.Errorf("wallet identity registry incomplete: got=%d want=8", identities)
	}
	return nil
}
