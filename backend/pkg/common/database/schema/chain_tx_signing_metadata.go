package schema

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// The shared outbox model includes these fields in GORM INSERTs even for the
// legacy IDO V2 signer path. Without the migration, Core and Job fail before
// any endProject transaction can be reserved on the pinned production schema.
const chainTxSigningMetadataMigration = "2026_10_02_chain_tx_submission_signing_metadata.sql"
const chainTxSigningMetadataChecksum = "0a0d4a20a36065175aadf17111a7730b473615628d3e88c9eadcd3e9fc9d8bb2"

func RequireChainTxSigningMetadata(database *gorm.DB) error {
	if database == nil {
		return errors.New("chain tx signing metadata schema check requires a database")
	}
	database = primarySchemaDatabase(database)
	var ledger int64
	if err := database.Raw(`SELECT COUNT(*) FROM schema_migrations WHERE status='applied' AND filename=? AND checksum=?
AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE status='applying')`, chainTxSigningMetadataMigration, chainTxSigningMetadataChecksum).Scan(&ledger).Error; err != nil {
		return fmt.Errorf("check chain tx signing metadata migration ledger: %w", err)
	}
	if ledger != 1 {
		return fmt.Errorf("chain tx signing metadata migration ledger incomplete: got=%d want=1; apply %s through controlled migration before starting Core/Job", ledger, chainTxSigningMetadataMigration)
	}
	var columns int64
	if err := database.Raw(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_tx_submission' AND (
(COLUMN_NAME IN ('unsigned_tx_digest','signing_digest') AND COLUMN_TYPE='varchar(66)' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='' AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin') OR
(COLUMN_NAME='replacement_generation' AND COLUMN_TYPE='int unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='0') OR
(COLUMN_NAME='signing_reserved_at' AND COLUMN_TYPE='datetime(6)' AND IS_NULLABLE='YES') OR
(COLUMN_NAME='signing_recovery_reason' AND DATA_TYPE='text' AND IS_NULLABLE='NO' AND (COLUMN_DEFAULT='' OR REPLACE(COLUMN_DEFAULT,CHAR(92),'')=CONCAT('_utf8mb4',CHAR(39),CHAR(39))))
)`).Scan(&columns).Error; err != nil {
		return fmt.Errorf("check chain tx signing metadata columns: %w", err)
	}
	if columns != 5 {
		return fmt.Errorf("chain tx signing metadata columns incomplete: got=%d want=5", columns)
	}
	return nil
}
