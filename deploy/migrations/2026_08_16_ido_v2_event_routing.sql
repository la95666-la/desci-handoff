-- JoinCare IDO Sprint 1A-P2 · persist V1/V2 event source routing and V2 scan registration.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_v2_event_routing;
DELIMITER $$
CREATE PROCEDURE jc_preflight_v2_event_routing()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME IN ('projects','chain_event_record','chain_event_failure','chain_event_sync_state','fund_ledger')
  ) <> 5 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'V2 event routing prerequisite tables are missing';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND (
      (TABLE_NAME = 'chain_event_record' AND COLUMN_NAME IN ('contract_address','contract_version'))
      OR (TABLE_NAME = 'chain_event_failure' AND COLUMN_NAME IN ('contract_address','contract_version'))
      OR (TABLE_NAME = 'chain_event_sync_state' AND COLUMN_NAME IN ('v2_registered','v2_contract_address','v2_start_block','v2_implementation_address','v2_implementation_code_hash'))
      OR (TABLE_NAME = 'projects' AND COLUMN_NAME IN ('fee_receiver','success_fee_rate','platform_fee_withdrawn'))
    )
  ) <> 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'partial or pre-existing V2 event routing schema requires manual recovery';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_v2_event_routing();
DROP PROCEDURE jc_preflight_v2_event_routing;

DROP PROCEDURE IF EXISTS jc_add_event_source_column;
DELIMITER $$
CREATE PROCEDURE jc_add_event_source_column(IN p_table VARCHAR(64), IN p_name VARCHAR(64), IN p_definition TEXT)
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = p_table AND COLUMN_NAME = p_name
  ) THEN
    SET @sql = CONCAT('ALTER TABLE `', REPLACE(p_table, '`', '``'), '` ADD COLUMN `', REPLACE(p_name, '`', '``'), '` ', p_definition);
    PREPARE stmt FROM @sql;
    EXECUTE stmt;
    DEALLOCATE PREPARE stmt;
  END IF;
END$$
DELIMITER ;

CALL jc_add_event_source_column('chain_event_record', 'contract_address', 'VARCHAR(42) NOT NULL DEFAULT '''' AFTER `log_index`');
CALL jc_add_event_source_column('chain_event_record', 'contract_version', 'VARCHAR(32) NOT NULL DEFAULT ''v1_legacy_unverified'' AFTER `contract_address`');
CALL jc_add_event_source_column('chain_event_failure', 'contract_address', 'VARCHAR(42) NOT NULL DEFAULT '''' AFTER `log_index`');
CALL jc_add_event_source_column('chain_event_failure', 'contract_version', 'VARCHAR(32) NOT NULL DEFAULT ''v1_legacy_unverified'' AFTER `contract_address`');
CALL jc_add_event_source_column('chain_event_sync_state', 'v2_registered', 'TINYINT(1) NOT NULL DEFAULT 0 AFTER `lease_token`');
CALL jc_add_event_source_column('chain_event_sync_state', 'v2_contract_address', 'VARCHAR(42) NOT NULL DEFAULT '''' AFTER `v2_registered`');
CALL jc_add_event_source_column('chain_event_sync_state', 'v2_start_block', 'BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER `v2_contract_address`');
CALL jc_add_event_source_column('chain_event_sync_state', 'v2_implementation_address', 'VARCHAR(42) NOT NULL DEFAULT '''' AFTER `v2_start_block`');
CALL jc_add_event_source_column('chain_event_sync_state', 'v2_implementation_code_hash', 'VARCHAR(66) NOT NULL DEFAULT '''' AFTER `v2_implementation_address`');
CALL jc_add_event_source_column('projects', 'fee_receiver', 'VARCHAR(42) NOT NULL DEFAULT '''' AFTER `onchain_locked`');
CALL jc_add_event_source_column('projects', 'success_fee_rate', 'SMALLINT UNSIGNED NOT NULL DEFAULT 0 AFTER `fee_receiver`');
CALL jc_add_event_source_column('projects', 'platform_fee_withdrawn', 'TINYINT(1) NOT NULL DEFAULT 0 AFTER `fund_withdrawn`');
DROP PROCEDURE jc_add_event_source_column;

-- Existing rows receive v1_legacy_unverified from the ADD COLUMN default because
-- their historical contract address was not persisted and must not be invented.

DROP PROCEDURE IF EXISTS jc_assert_v2_event_routing_schema;
DELIMITER $$
CREATE PROCEDURE jc_assert_v2_event_routing_schema()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND (
      (TABLE_NAME = 'chain_event_record' AND COLUMN_NAME IN ('contract_address','contract_version'))
      OR (TABLE_NAME = 'chain_event_failure' AND COLUMN_NAME IN ('contract_address','contract_version'))
      OR (TABLE_NAME = 'chain_event_sync_state' AND COLUMN_NAME IN ('v2_registered','v2_contract_address','v2_start_block','v2_implementation_address','v2_implementation_code_hash'))
      OR (TABLE_NAME = 'projects' AND COLUMN_NAME IN ('fee_receiver','success_fee_rate','platform_fee_withdrawn'))
    )
  ) <> 12 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'V2 event routing columns are incomplete';
  END IF;

  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND (
      (TABLE_NAME IN ('chain_event_record','chain_event_failure') AND COLUMN_NAME = 'contract_address' AND COLUMN_TYPE = 'varchar(42)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
      OR (TABLE_NAME IN ('chain_event_record','chain_event_failure') AND COLUMN_NAME = 'contract_version' AND COLUMN_TYPE = 'varchar(32)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = 'v1_legacy_unverified')
      OR (TABLE_NAME = 'chain_event_sync_state' AND COLUMN_NAME = 'v2_registered' AND COLUMN_TYPE = 'tinyint(1)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0')
      OR (TABLE_NAME = 'chain_event_sync_state' AND COLUMN_NAME = 'v2_contract_address' AND COLUMN_TYPE = 'varchar(42)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
      OR (TABLE_NAME = 'chain_event_sync_state' AND COLUMN_NAME = 'v2_start_block' AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0')
      OR (TABLE_NAME = 'chain_event_sync_state' AND COLUMN_NAME = 'v2_implementation_address' AND COLUMN_TYPE = 'varchar(42)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
      OR (TABLE_NAME = 'chain_event_sync_state' AND COLUMN_NAME = 'v2_implementation_code_hash' AND COLUMN_TYPE = 'varchar(66)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
      OR (TABLE_NAME = 'projects' AND COLUMN_NAME = 'fee_receiver' AND COLUMN_TYPE = 'varchar(42)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
      OR (TABLE_NAME = 'projects' AND COLUMN_NAME = 'success_fee_rate' AND COLUMN_TYPE = 'smallint unsigned' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0')
      OR (TABLE_NAME = 'projects' AND COLUMN_NAME = 'platform_fee_withdrawn' AND COLUMN_TYPE = 'tinyint(1)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0')
    )
  ) <> 12 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'V2 event routing column definitions are incompatible';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_v2_event_routing_schema();
DROP PROCEDURE jc_assert_v2_event_routing_schema;

-- Verification:
--   SELECT id,cursor_block,v2_registered,v2_contract_address,v2_start_block,v2_implementation_address,v2_implementation_code_hash FROM chain_event_sync_state;
