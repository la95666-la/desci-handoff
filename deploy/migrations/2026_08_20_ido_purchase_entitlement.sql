-- JoinCare IDO Sprint 1A-P2 · append-only V2 purchase project-token entitlement facts.
-- This migration extends 2026_08_15 and 2026_08_16; do not run it independently.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_ido_purchase_entitlement;
DELIMITER $$
CREATE PROCEDURE jc_preflight_ido_purchase_entitlement()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME IN ('projects','fund_ledger','chain_event_record')
      AND TABLE_TYPE = 'BASE TABLE'
  ) <> 3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'purchase entitlement prerequisite tables are missing';
  END IF;
  IF EXISTS (
    SELECT 1 FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ido_purchase_entitlement'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'pre-existing purchase entitlement table requires manual verification';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND (
      (TABLE_NAME = 'fund_ledger' AND COLUMN_NAME IN (
        'id','chain_id','contract_address','contract_version','project_id','wallet_address','counterparty_address',
        'entry_type','asset_role','token_symbol','token_address','token_decimals','amount_raw',
        'amount_display','tx_hash','log_index','block_number','block_hash','status','event_data'
      ))
      OR (TABLE_NAME = 'projects' AND COLUMN_NAME IN (
        'project_id','chain_id','contract_address','contract_version','payment_token',
        'payment_token_address','payment_token_decimals','token_symbol','token_address','token_decimals',
        'total_raised_raw','participant_count'
      ))
      OR (TABLE_NAME = 'chain_event_record' AND COLUMN_NAME IN ('event','contract_version'))
    )
  ) <> 34 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'purchase entitlement prerequisite columns are incomplete';
  END IF;
  IF EXISTS (
    SELECT 1 FROM fund_ledger
    WHERE contract_version = 'v2_stablecoin' AND entry_type = 'purchase_payment'
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'historical V2 purchases require canonical entitlement replay before migration';
  END IF;
  IF EXISTS (
    SELECT 1 FROM chain_event_record
    WHERE contract_version = 'v2_stablecoin' AND event = 'Purchased'
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'historical V2 Purchased records require canonical entitlement replay before migration';
  END IF;
  IF EXISTS (
    SELECT 1 FROM projects
    WHERE contract_version = 'v2_stablecoin'
      AND (total_raised_raw <> '0' OR participant_count <> 0)
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'historical V2 purchase-derived project state requires canonical entitlement replay before migration';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_ido_purchase_entitlement();
DROP PROCEDURE jc_preflight_ido_purchase_entitlement;

CREATE TABLE `ido_purchase_entitlement` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `fund_ledger_id` BIGINT UNSIGNED NOT NULL,
  `chain_id` BIGINT UNSIGNED NOT NULL,
  `contract_address` VARCHAR(42) NOT NULL,
  `contract_version` VARCHAR(32) NOT NULL,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `wallet_address` VARCHAR(42) NOT NULL,
  `payment_token_symbol` VARCHAR(20) NOT NULL,
  `payment_token_address` VARCHAR(42) NOT NULL,
  `payment_token_decimals` TINYINT UNSIGNED NOT NULL,
  `payment_amount_raw` VARCHAR(78) NOT NULL,
  `payment_amount_display` VARCHAR(160) NOT NULL,
  `project_token_symbol` VARCHAR(20) NOT NULL,
  `project_token_address` VARCHAR(42) NOT NULL,
  `project_token_decimals` TINYINT UNSIGNED NOT NULL,
  `project_token_amount_raw` VARCHAR(78) NOT NULL,
  `project_token_amount_display` VARCHAR(160) NOT NULL,
  `tx_hash` VARCHAR(66) NOT NULL,
  `log_index` INT UNSIGNED NOT NULL,
  `block_number` BIGINT UNSIGNED NOT NULL,
  `block_hash` VARCHAR(66) NOT NULL,
  `status` VARCHAR(20) NOT NULL DEFAULT 'canonical',
  `event_data` JSON NOT NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_ido_purchase_entitlement_ledger` (`fund_ledger_id`),
  UNIQUE KEY `uk_ido_purchase_entitlement_event` (`chain_id`,`tx_hash`,`log_index`),
  KEY `idx_ido_purchase_entitlement_project` (`chain_id`,`contract_address`,`project_id`),
  KEY `idx_ido_purchase_entitlement_wallet` (`chain_id`,`contract_address`,`project_id`,`wallet_address`),
  KEY `idx_ido_purchase_entitlement_block` (`chain_id`,`block_number`),
  CONSTRAINT `fk_ido_purchase_entitlement_ledger`
    FOREIGN KEY (`fund_ledger_id`) REFERENCES `fund_ledger` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `chk_ido_purchase_entitlement_positive_identity` CHECK (
    `fund_ledger_id` > 0 AND `chain_id` > 0 AND `project_id` > 0 AND `block_number` > 0
  ) ENFORCED,
  CONSTRAINT `chk_ido_purchase_entitlement_addresses` CHECK (
    `contract_address` REGEXP '^0x[0-9A-Fa-f]{40}$'
    AND `contract_address` <> '0x0000000000000000000000000000000000000000'
    AND `wallet_address` REGEXP '^0x[0-9A-Fa-f]{40}$'
    AND `wallet_address` <> '0x0000000000000000000000000000000000000000'
    AND `payment_token_address` REGEXP '^0x[0-9A-Fa-f]{40}$'
    AND `payment_token_address` <> '0x0000000000000000000000000000000000000000'
    AND `project_token_address` REGEXP '^0x[0-9A-Fa-f]{40}$'
    AND `project_token_address` <> '0x0000000000000000000000000000000000000000'
    AND LOWER(`payment_token_address`) <> LOWER(`project_token_address`)
  ) ENFORCED,
  CONSTRAINT `chk_ido_purchase_entitlement_tokens` CHECK (
    `payment_token_symbol` IN ('USDT','USDC')
    AND CHAR_LENGTH(TRIM(`project_token_symbol`)) > 0
    AND `payment_token_decimals` BETWEEN 1 AND 18
    AND `project_token_decimals` BETWEEN 1 AND 18
  ) ENFORCED,
  CONSTRAINT `chk_ido_purchase_entitlement_payment_raw` CHECK (
    `payment_amount_raw` REGEXP '^[1-9][0-9]{0,77}$'
    AND (
      CHAR_LENGTH(`payment_amount_raw`) < 78
      OR `payment_amount_raw` <= '115792089237316195423570985008687907853269984665640564039457584007913129639935'
    )
  ) ENFORCED,
  CONSTRAINT `chk_ido_purchase_entitlement_project_raw` CHECK (
    `project_token_amount_raw` REGEXP '^[1-9][0-9]{0,77}$'
    AND (
      CHAR_LENGTH(`project_token_amount_raw`) < 78
      OR `project_token_amount_raw` <= '115792089237316195423570985008687907853269984665640564039457584007913129639935'
    )
  ) ENFORCED,
  CONSTRAINT `chk_ido_purchase_entitlement_payment_display` CHECK (
    BINARY `payment_amount_display` = BINARY CASE
      WHEN CHAR_LENGTH(`payment_amount_raw`) <= `payment_token_decimals` THEN
        CONCAT('0.', TRIM(TRAILING '0' FROM LPAD(`payment_amount_raw`, `payment_token_decimals`, '0')))
      WHEN TRIM(TRAILING '0' FROM RIGHT(`payment_amount_raw`, `payment_token_decimals`)) = '' THEN
        LEFT(`payment_amount_raw`, CHAR_LENGTH(`payment_amount_raw`) - `payment_token_decimals`)
      ELSE CONCAT(
        LEFT(`payment_amount_raw`, CHAR_LENGTH(`payment_amount_raw`) - `payment_token_decimals`), '.',
        TRIM(TRAILING '0' FROM RIGHT(`payment_amount_raw`, `payment_token_decimals`))
      )
    END
  ) ENFORCED,
  CONSTRAINT `chk_ido_purchase_entitlement_project_display` CHECK (
    BINARY `project_token_amount_display` = BINARY CASE
      WHEN CHAR_LENGTH(`project_token_amount_raw`) <= `project_token_decimals` THEN
        CONCAT('0.', TRIM(TRAILING '0' FROM LPAD(`project_token_amount_raw`, `project_token_decimals`, '0')))
      WHEN TRIM(TRAILING '0' FROM RIGHT(`project_token_amount_raw`, `project_token_decimals`)) = '' THEN
        LEFT(`project_token_amount_raw`, CHAR_LENGTH(`project_token_amount_raw`) - `project_token_decimals`)
      ELSE CONCAT(
        LEFT(`project_token_amount_raw`, CHAR_LENGTH(`project_token_amount_raw`) - `project_token_decimals`), '.',
        TRIM(TRAILING '0' FROM RIGHT(`project_token_amount_raw`, `project_token_decimals`))
      )
    END
  ) ENFORCED,
  CONSTRAINT `chk_ido_purchase_entitlement_hashes` CHECK (
    `tx_hash` REGEXP '^0x[0-9A-Fa-f]{64}$' AND `block_hash` REGEXP '^0x[0-9A-Fa-f]{64}$'
  ) ENFORCED,
  CONSTRAINT `chk_ido_purchase_entitlement_version` CHECK (`contract_version` = 'v2_stablecoin') ENFORCED,
  CONSTRAINT `chk_ido_purchase_entitlement_status` CHECK (`status` = 'canonical') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DROP TRIGGER IF EXISTS `trg_ido_purchase_entitlement_no_update`;
DROP TRIGGER IF EXISTS `trg_ido_purchase_entitlement_no_delete`;
DROP TRIGGER IF EXISTS `trg_ido_purchase_entitlement_validate_insert`;
DELIMITER $$
CREATE TRIGGER `trg_ido_purchase_entitlement_validate_insert`
BEFORE INSERT ON `ido_purchase_entitlement`
FOR EACH ROW
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM `fund_ledger` f
    WHERE f.id = NEW.fund_ledger_id
      AND f.entry_type = 'purchase_payment'
      AND f.asset_role = 'payment'
      AND f.status = 'canonical'
      AND f.chain_id = NEW.chain_id
      AND LOWER(f.contract_address) = LOWER(NEW.contract_address)
      AND f.contract_version = NEW.contract_version
      AND f.project_id = NEW.project_id
      AND LOWER(f.wallet_address) = LOWER(NEW.wallet_address)
      AND LOWER(f.counterparty_address) = LOWER(NEW.contract_address)
      AND f.token_symbol = NEW.payment_token_symbol
      AND LOWER(f.token_address) = LOWER(NEW.payment_token_address)
      AND f.token_decimals = NEW.payment_token_decimals
      AND BINARY f.amount_raw = BINARY NEW.payment_amount_raw
      AND BINARY f.amount_display = BINARY NEW.payment_amount_display
      AND LOWER(f.tx_hash) = LOWER(NEW.tx_hash)
      AND f.log_index = NEW.log_index
      AND f.block_number = NEW.block_number
      AND LOWER(f.block_hash) = LOWER(NEW.block_hash)
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'purchase entitlement parent ledger identity mismatch';
  END IF;
END$$
CREATE TRIGGER `trg_ido_purchase_entitlement_no_update`
BEFORE UPDATE ON `ido_purchase_entitlement`
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'IDO purchase entitlement rows are append-only';
END$$
CREATE TRIGGER `trg_ido_purchase_entitlement_no_delete`
BEFORE DELETE ON `ido_purchase_entitlement`
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'IDO purchase entitlement rows are append-only';
END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS jc_assert_ido_purchase_entitlement;
DELIMITER $$
CREATE PROCEDURE jc_assert_ido_purchase_entitlement()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ido_purchase_entitlement' AND (
      (COLUMN_NAME IN ('id','fund_ledger_id','chain_id','project_id','block_number') AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME IN ('contract_address','wallet_address','payment_token_address','project_token_address') AND COLUMN_TYPE = 'varchar(42)' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME = 'contract_version' AND COLUMN_TYPE = 'varchar(32)' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME IN ('payment_token_symbol','project_token_symbol') AND COLUMN_TYPE = 'varchar(20)' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME IN ('payment_token_decimals','project_token_decimals') AND COLUMN_TYPE = 'tinyint unsigned' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME IN ('payment_amount_raw','project_token_amount_raw') AND COLUMN_TYPE = 'varchar(78)' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME IN ('payment_amount_display','project_token_amount_display') AND COLUMN_TYPE = 'varchar(160)' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME IN ('tx_hash','block_hash') AND COLUMN_TYPE = 'varchar(66)' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME = 'log_index' AND COLUMN_TYPE = 'int unsigned' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME = 'status' AND COLUMN_TYPE = 'varchar(20)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = 'canonical')
      OR (COLUMN_NAME = 'event_data' AND DATA_TYPE = 'json' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME = 'created_at' AND COLUMN_TYPE = 'datetime(6)' AND IS_NULLABLE = 'NO')
    )
  ) <> 24 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'purchase entitlement columns are incompatible';
  END IF;
  IF (
    SELECT COUNT(*) FROM (
      SELECT INDEX_NAME, GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') AS columns_in_order, MIN(NON_UNIQUE) AS non_unique
      FROM information_schema.STATISTICS
      WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ido_purchase_entitlement'
      GROUP BY INDEX_NAME
    ) entitlement_indexes
    WHERE non_unique = 0 AND columns_in_order IN ('fund_ledger_id','chain_id,tx_hash,log_index')
  ) <> 2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'purchase entitlement unique identities are incomplete';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'ido_purchase_entitlement'
      AND CONSTRAINT_TYPE = 'CHECK' AND ENFORCED = 'YES'
      AND CONSTRAINT_NAME IN (
        'chk_ido_purchase_entitlement_positive_identity','chk_ido_purchase_entitlement_addresses',
        'chk_ido_purchase_entitlement_tokens','chk_ido_purchase_entitlement_payment_raw',
        'chk_ido_purchase_entitlement_project_raw','chk_ido_purchase_entitlement_payment_display',
        'chk_ido_purchase_entitlement_project_display','chk_ido_purchase_entitlement_hashes',
        'chk_ido_purchase_entitlement_version','chk_ido_purchase_entitlement_status'
      )
  ) <> 10 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'purchase entitlement checks are incomplete';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.REFERENTIAL_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'ido_purchase_entitlement'
      AND CONSTRAINT_NAME = 'fk_ido_purchase_entitlement_ledger'
      AND REFERENCED_TABLE_NAME = 'fund_ledger' AND DELETE_RULE = 'RESTRICT'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'purchase entitlement ledger foreign key is missing';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.TRIGGERS
    WHERE TRIGGER_SCHEMA = DATABASE() AND EVENT_OBJECT_TABLE = 'ido_purchase_entitlement'
      AND TRIGGER_NAME IN ('trg_ido_purchase_entitlement_validate_insert','trg_ido_purchase_entitlement_no_update','trg_ido_purchase_entitlement_no_delete')
  ) <> 3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'purchase entitlement validation/append-only triggers are incomplete';
  END IF;
  IF EXISTS (
    SELECT 1 FROM fund_ledger
    WHERE contract_version = 'v2_stablecoin' AND entry_type = 'purchase_payment'
    LIMIT 1
  ) OR EXISTS (
    SELECT 1 FROM chain_event_record
    WHERE contract_version = 'v2_stablecoin' AND event = 'Purchased'
    LIMIT 1
  ) OR EXISTS (
    SELECT 1 FROM projects
    WHERE contract_version = 'v2_stablecoin'
      AND (total_raised_raw <> '0' OR participant_count <> 0)
    LIMIT 1
  ) OR EXISTS (
    SELECT 1 FROM ido_purchase_entitlement LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'purchase entitlement migration history invariant failed';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_ido_purchase_entitlement();
DROP PROCEDURE jc_assert_ido_purchase_entitlement;

-- Verification:
--   SHOW CREATE TABLE ido_purchase_entitlement;
--   SELECT COUNT(*) FROM fund_ledger WHERE entry_type='purchase_payment';
--   SELECT COUNT(*) FROM ido_purchase_entitlement;
