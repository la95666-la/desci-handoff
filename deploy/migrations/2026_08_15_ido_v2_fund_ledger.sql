-- JoinCare IDO Sprint 1A-P2 · V2 project identity and append-only raw-unit fund ledger.
-- This migration does not assign stablecoin addresses to legacy JCC rows and does not start V2 event consumption.

SET NAMES utf8mb4;

-- Fail before changing business tables when the pre-existing schema is ambiguous.
DROP PROCEDURE IF EXISTS jc_preflight_ido_v2_schema;
DELIMITER $$
CREATE PROCEDURE jc_preflight_ido_v2_schema()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'projects' AND TABLE_TYPE = 'BASE TABLE'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'projects table is missing';
  END IF;

  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'projects'
      AND (
        (COLUMN_NAME = 'payment_token' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH >= 20 AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'token_address' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH >= 42 AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'token_decimals' AND DATA_TYPE IN ('tinyint','smallint','mediumint','int','bigint') AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'status' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH >= 20 AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME IN ('token_price','hard_cap','soft_cap','max_per_user','total_raised','total_deposited')
          AND DATA_TYPE = 'decimal' AND NUMERIC_SCALE <= 18
          AND NUMERIC_PRECISION - NUMERIC_SCALE <= 47 AND IS_NULLABLE = 'NO')
      )
  ) <> 10 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'projects base IDO column definitions are incompatible';
  END IF;

  SET @existing_v2_column_count = (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'projects'
      AND COLUMN_NAME IN (
        'payment_token_address','payment_token_decimals','chain_id','contract_address',
        'contract_version','onchain_locked','token_price_raw','hard_cap_raw','soft_cap_raw',
        'max_per_user_raw','total_raised_raw','total_deposited_raw'
      )
  );
  IF @existing_v2_column_count NOT IN (0, 12) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'partial projects IDO V2 schema requires manual recovery';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'fund_ledger'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'pre-existing fund_ledger requires manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_ido_v2_schema();
DROP PROCEDURE jc_preflight_ido_v2_schema;

-- Refuse ambiguous historical data before any auto-committing DDL is attempted.
DROP PROCEDURE IF EXISTS jc_assert_known_payment_token_values;
DELIMITER $$
CREATE PROCEDURE jc_assert_known_payment_token_values()
BEGIN
  IF EXISTS (
    SELECT 1 FROM `projects`
    WHERE UPPER(TRIM(`payment_token`)) NOT IN ('', 'JCC', 'JC', 'USDT', 'USDC')
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'unknown historical payment_token values require manual classification';
  END IF;
  IF EXISTS (
    SELECT 1 FROM `projects`
    WHERE `status` NOT IN ('Setup', 'Set', 'pre_launch', 'warmup')
      AND UPPER(TRIM(`payment_token`)) = ''
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'non-draft rows without a settlement asset require manual classification';
  END IF;

  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'projects'
      AND COLUMN_NAME IN (
        'payment_token_address','payment_token_decimals','chain_id','contract_address',
        'contract_version','onchain_locked','token_price_raw','hard_cap_raw','soft_cap_raw',
        'max_per_user_raw','total_raised_raw','total_deposited_raw'
      )
  ) = 12 THEN
    SET @ambiguous_v2_count = 0;
    SET @check_v2_sql =
      'SELECT COUNT(*) INTO @ambiguous_v2_count FROM `projects` WHERE UPPER(TRIM(`payment_token`)) IN (''USDT'',''USDC'') AND `contract_version` <> ''v2_stablecoin''';
    PREPARE check_v2_stmt FROM @check_v2_sql;
    EXECUTE check_v2_stmt;
    DEALLOCATE PREPARE check_v2_stmt;
    IF @ambiguous_v2_count > 0 THEN
      SIGNAL SQLSTATE '45000'
        SET MESSAGE_TEXT = 'historical USDT/USDC rows require explicit V2 identity classification';
    END IF;
  ELSEIF EXISTS (
    SELECT 1 FROM `projects`
    WHERE UPPER(TRIM(`payment_token`)) IN ('USDT', 'USDC')
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'historical USDT/USDC rows require explicit V2 identity classification';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_known_payment_token_values();
DROP PROCEDURE jc_assert_known_payment_token_values;

DROP PROCEDURE IF EXISTS jc_add_projects_column;
DELIMITER $$
CREATE PROCEDURE jc_add_projects_column(
  IN p_column_name VARCHAR(64),
  IN p_column_definition TEXT,
  IN p_expected_column_type VARCHAR(64),
  IN p_expected_default VARCHAR(64)
)
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'projects' AND COLUMN_NAME = p_column_name
  ) THEN
    IF NOT EXISTS (
      SELECT 1 FROM information_schema.COLUMNS
      WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'projects'
        AND COLUMN_NAME = p_column_name
        AND COLUMN_TYPE = p_expected_column_type
        AND IS_NULLABLE = 'NO'
        AND COLUMN_DEFAULT = p_expected_default
    ) THEN
      SIGNAL SQLSTATE '45000'
        SET MESSAGE_TEXT = 'existing projects IDO V2 column definition is incompatible';
    END IF;
  ELSE
    SET @add_column_sql = CONCAT(
      'ALTER TABLE `projects` ADD COLUMN `', REPLACE(p_column_name, '`', '``'), '` ', p_column_definition
    );
    PREPARE stmt FROM @add_column_sql;
    EXECUTE stmt;
    DEALLOCATE PREPARE stmt;
  END IF;
END$$
DELIMITER ;

CALL jc_add_projects_column('payment_token_address', 'VARCHAR(42) NOT NULL DEFAULT '''' AFTER `payment_token`', 'varchar(42)', '');
CALL jc_add_projects_column('payment_token_decimals', 'TINYINT UNSIGNED NOT NULL DEFAULT 0 AFTER `payment_token_address`', 'tinyint unsigned', '0');
CALL jc_add_projects_column('chain_id', 'BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER `payment_token_decimals`', 'bigint unsigned', '0');
CALL jc_add_projects_column('contract_address', 'VARCHAR(42) NOT NULL DEFAULT '''' AFTER `chain_id`', 'varchar(42)', '');
CALL jc_add_projects_column('contract_version', 'VARCHAR(32) NOT NULL DEFAULT ''unassigned'' AFTER `contract_address`', 'varchar(32)', 'unassigned');
CALL jc_add_projects_column('onchain_locked', 'TINYINT(1) NOT NULL DEFAULT 0 AFTER `contract_version`', 'tinyint(1)', '0');
CALL jc_add_projects_column('token_price_raw', 'VARCHAR(78) NOT NULL DEFAULT ''0'' AFTER `token_price`', 'varchar(78)', '0');
CALL jc_add_projects_column('hard_cap_raw', 'VARCHAR(78) NOT NULL DEFAULT ''0'' AFTER `hard_cap`', 'varchar(78)', '0');
CALL jc_add_projects_column('soft_cap_raw', 'VARCHAR(78) NOT NULL DEFAULT ''0'' AFTER `soft_cap`', 'varchar(78)', '0');
CALL jc_add_projects_column('max_per_user_raw', 'VARCHAR(78) NOT NULL DEFAULT ''0'' AFTER `max_per_user`', 'varchar(78)', '0');
CALL jc_add_projects_column('total_raised_raw', 'VARCHAR(78) NOT NULL DEFAULT ''0'' AFTER `total_raised`', 'varchar(78)', '0');
CALL jc_add_projects_column('total_deposited_raw', 'VARCHAR(78) NOT NULL DEFAULT ''0'' AFTER `total_deposited`', 'varchar(78)', '0');
DROP PROCEDURE jc_add_projects_column;

DROP PROCEDURE IF EXISTS jc_assert_unambiguous_v2_history;
DELIMITER $$
CREATE PROCEDURE jc_assert_unambiguous_v2_history()
BEGIN
  IF EXISTS (
    SELECT 1 FROM `projects`
    WHERE UPPER(TRIM(`payment_token`)) IN ('USDT', 'USDC')
      AND (
        `contract_version` IS NULL OR `contract_version` <> 'v2_stablecoin'
        OR `chain_id` IS NULL OR `chain_id` = 0
        OR `payment_token_decimals` IS NULL OR `payment_token_decimals` NOT BETWEEN 1 AND 18
        OR `token_decimals` IS NULL OR `token_decimals` NOT BETWEEN 1 AND 18
        OR `payment_token_address` IS NULL OR `payment_token_address` NOT REGEXP '^0x[0-9A-Fa-f]{40}$'
        OR `payment_token_address` = '0x0000000000000000000000000000000000000000'
        OR `contract_address` IS NULL OR `contract_address` NOT REGEXP '^0x[0-9A-Fa-f]{40}$'
        OR `contract_address` = '0x0000000000000000000000000000000000000000'
        OR `token_address` IS NULL OR `token_address` NOT REGEXP '^0x[0-9A-Fa-f]{40}$'
        OR `token_address` = '0x0000000000000000000000000000000000000000'
      )
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'historical USDT/USDC rows require explicit V2 identity classification';
  END IF;

  IF EXISTS (
    SELECT 1 FROM `projects`
    WHERE UPPER(TRIM(`payment_token`)) IN ('USDT', 'USDC')
      AND (
        `token_price_raw` IS NULL OR `token_price_raw` NOT REGEXP '^(0|[1-9][0-9]{0,64})$'
        OR `hard_cap_raw` IS NULL OR `hard_cap_raw` NOT REGEXP '^(0|[1-9][0-9]{0,64})$'
        OR `soft_cap_raw` IS NULL OR `soft_cap_raw` NOT REGEXP '^(0|[1-9][0-9]{0,64})$'
        OR `max_per_user_raw` IS NULL OR `max_per_user_raw` NOT REGEXP '^(0|[1-9][0-9]{0,64})$'
        OR `total_raised_raw` IS NULL OR `total_raised_raw` NOT REGEXP '^(0|[1-9][0-9]{0,64})$'
        OR `total_deposited_raw` IS NULL OR `total_deposited_raw` NOT REGEXP '^(0|[1-9][0-9]{0,64})$'
      )
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'historical V2 raw amounts are missing or non-canonical';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM (
      SELECT p.*,
        CASE `payment_token_decimals`
          WHEN 1 THEN 10 WHEN 2 THEN 100 WHEN 3 THEN 1000 WHEN 4 THEN 10000
          WHEN 5 THEN 100000 WHEN 6 THEN 1000000 WHEN 7 THEN 10000000
          WHEN 8 THEN 100000000 WHEN 9 THEN 1000000000 WHEN 10 THEN 10000000000
          WHEN 11 THEN 100000000000 WHEN 12 THEN 1000000000000
          WHEN 13 THEN 10000000000000 WHEN 14 THEN 100000000000000
          WHEN 15 THEN 1000000000000000 WHEN 16 THEN 10000000000000000
          WHEN 17 THEN 100000000000000000 ELSE 1000000000000000000
        END AS payment_scale,
        CASE `token_decimals`
          WHEN 1 THEN 10 WHEN 2 THEN 100 WHEN 3 THEN 1000 WHEN 4 THEN 10000
          WHEN 5 THEN 100000 WHEN 6 THEN 1000000 WHEN 7 THEN 10000000
          WHEN 8 THEN 100000000 WHEN 9 THEN 1000000000 WHEN 10 THEN 10000000000
          WHEN 11 THEN 100000000000 WHEN 12 THEN 1000000000000
          WHEN 13 THEN 10000000000000 WHEN 14 THEN 100000000000000
          WHEN 15 THEN 1000000000000000 WHEN 16 THEN 10000000000000000
          WHEN 17 THEN 100000000000000000 ELSE 1000000000000000000
        END AS project_scale
      FROM `projects` p
      WHERE UPPER(TRIM(`payment_token`)) IN ('USDT', 'USDC')
    ) v2
    WHERE
      `status` IS NULL
      OR (`status` NOT IN ('Setup', 'Set', 'pre_launch', 'warmup') AND (`onchain_locked` IS NULL OR `onchain_locked` <> 1))
      OR `token_price` * payment_scale <> FLOOR(`token_price` * payment_scale)
      OR `hard_cap` * payment_scale <> FLOOR(`hard_cap` * payment_scale)
      OR `soft_cap` * payment_scale <> FLOOR(`soft_cap` * payment_scale)
      OR `max_per_user` * payment_scale <> FLOOR(`max_per_user` * payment_scale)
      OR `total_raised` * payment_scale <> FLOOR(`total_raised` * payment_scale)
      OR `total_deposited` * project_scale <> FLOOR(`total_deposited` * project_scale)
      OR CAST(`token_price_raw` AS DECIMAL(65,0)) <> `token_price` * payment_scale
      OR CAST(`hard_cap_raw` AS DECIMAL(65,0)) <> `hard_cap` * payment_scale
      OR CAST(`soft_cap_raw` AS DECIMAL(65,0)) <> `soft_cap` * payment_scale
      OR CAST(`max_per_user_raw` AS DECIMAL(65,0)) <> `max_per_user` * payment_scale
      OR CAST(`total_raised_raw` AS DECIMAL(65,0)) <> `total_raised` * payment_scale
      OR CAST(`total_deposited_raw` AS DECIMAL(65,0)) <> `total_deposited` * project_scale
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'historical V2 identity lock or raw/display amounts are inconsistent';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_unambiguous_v2_history();
DROP PROCEDURE jc_assert_unambiguous_v2_history;

-- Remove the unsafe JCC default. Existing values remain for classification below.
ALTER TABLE `projects`
  MODIFY COLUMN `payment_token` VARCHAR(20) NOT NULL DEFAULT '' COMMENT '受信支付币 symbol；V2 仅 USDT/USDC',
  MODIFY COLUMN `token_price` DECIMAL(65,18) NOT NULL DEFAULT 0 COMMENT '展示值；raw 字段为链上事实',
  MODIFY COLUMN `hard_cap` DECIMAL(65,18) NOT NULL DEFAULT 0 COMMENT '展示值；raw 字段为链上事实',
  MODIFY COLUMN `soft_cap` DECIMAL(65,18) NOT NULL DEFAULT 0 COMMENT '展示值；raw 字段为链上事实',
  MODIFY COLUMN `max_per_user` DECIMAL(65,18) NOT NULL DEFAULT 0 COMMENT '展示值；raw 字段为链上事实',
  MODIFY COLUMN `total_raised` DECIMAL(65,18) NOT NULL DEFAULT 0 COMMENT '展示聚合；raw 字段为链上事实',
  MODIFY COLUMN `total_deposited` DECIMAL(65,18) NOT NULL DEFAULT 0 COMMENT '展示聚合；raw 字段为链上事实';

-- Content/setup rows have never been valid on-chain IDO facts. Force an explicit stablecoin choice later.
UPDATE `projects`
SET `payment_token` = '',
    `payment_token_address` = '',
    `payment_token_decimals` = 0,
    `chain_id` = 0,
    `contract_address` = '',
    `contract_version` = 'unassigned',
    `onchain_locked` = 0
WHERE `status` IN ('Setup', 'Set', 'pre_launch', 'warmup')
  AND UPPER(TRIM(`payment_token`)) IN ('', 'JCC', 'JC');

-- Non-draft historical rows are marked legacy without inventing chain/contract/token addresses.
UPDATE `projects`
SET `payment_token` = 'JCC',
    `payment_token_address` = '',
    `payment_token_decimals` = 18,
    `chain_id` = 0,
    `contract_address` = '',
    `contract_version` = 'v1_legacy_unverified',
    `onchain_locked` = 1
WHERE `status` NOT IN ('Setup', 'Set', 'pre_launch', 'warmup')
  AND UPPER(TRIM(`payment_token`)) IN ('JCC', 'JC')
  AND `contract_version` = 'unassigned';

CREATE TABLE IF NOT EXISTS `fund_ledger` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `chain_id` BIGINT UNSIGNED NOT NULL,
  `contract_address` VARCHAR(42) NOT NULL,
  `contract_version` VARCHAR(32) NOT NULL,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `wallet_address` VARCHAR(42) NOT NULL DEFAULT '',
  `counterparty_address` VARCHAR(42) NOT NULL DEFAULT '',
  `entry_type` VARCHAR(32) NOT NULL,
  `asset_role` VARCHAR(16) NOT NULL,
  `token_symbol` VARCHAR(20) NOT NULL DEFAULT '',
  `token_address` VARCHAR(42) NOT NULL,
  `token_decimals` TINYINT UNSIGNED NOT NULL,
  `amount_raw` VARCHAR(78) NOT NULL COMMENT 'canonical base-10 uint256 string',
  `amount_display` VARCHAR(160) NOT NULL COMMENT 'deterministic display string; never a float',
  `tx_hash` VARCHAR(66) NOT NULL,
  `log_index` INT UNSIGNED NOT NULL,
  `block_number` BIGINT UNSIGNED NOT NULL,
  `block_hash` VARCHAR(66) NOT NULL,
  `status` VARCHAR(20) NOT NULL DEFAULT 'canonical',
  `event_data` JSON NOT NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_fund_ledger_event` (`chain_id`, `tx_hash`, `log_index`),
  KEY `idx_fund_ledger_project` (`chain_id`, `contract_address`, `project_id`),
  KEY `idx_fund_ledger_wallet` (`wallet_address`),
  KEY `idx_fund_ledger_token` (`token_address`),
  KEY `idx_fund_ledger_block` (`block_number`),
  KEY `idx_fund_ledger_status` (`status`),
  CONSTRAINT `chk_fund_ledger_token_decimals` CHECK (`token_decimals` BETWEEN 1 AND 18),
  CONSTRAINT `chk_fund_ledger_positive_identity` CHECK (`chain_id` > 0 AND `project_id` > 0 AND `block_number` > 0),
  CONSTRAINT `chk_fund_ledger_contract_address` CHECK (
    `contract_address` REGEXP '^0x[0-9A-Fa-f]{40}$'
    AND `contract_address` <> '0x0000000000000000000000000000000000000000'
  ),
  CONSTRAINT `chk_fund_ledger_token_address` CHECK (
    `token_address` REGEXP '^0x[0-9A-Fa-f]{40}$'
    AND `token_address` <> '0x0000000000000000000000000000000000000000'
  ),
  CONSTRAINT `chk_fund_ledger_tx_hash` CHECK (`tx_hash` REGEXP '^0x[0-9A-Fa-f]{64}$'),
  CONSTRAINT `chk_fund_ledger_block_hash` CHECK (`block_hash` REGEXP '^0x[0-9A-Fa-f]{64}$'),
  CONSTRAINT `chk_fund_ledger_amount_raw` CHECK (
    `amount_raw` REGEXP '^(0|[1-9][0-9]{0,77})$'
    AND (
      CHAR_LENGTH(`amount_raw`) < 78
      OR `amount_raw` <= '115792089237316195423570985008687907853269984665640564039457584007913129639935'
    )
  ),
  CONSTRAINT `chk_fund_ledger_amount_display` CHECK (
    BINARY `amount_display` = BINARY CASE
      WHEN `amount_raw` = '0' THEN '0'
      WHEN CHAR_LENGTH(`amount_raw`) <= `token_decimals` THEN
        CONCAT('0.', TRIM(TRAILING '0' FROM LPAD(`amount_raw`, `token_decimals`, '0')))
      WHEN TRIM(TRAILING '0' FROM RIGHT(`amount_raw`, `token_decimals`)) = '' THEN
        LEFT(`amount_raw`, CHAR_LENGTH(`amount_raw`) - `token_decimals`)
      ELSE CONCAT(
        LEFT(`amount_raw`, CHAR_LENGTH(`amount_raw`) - `token_decimals`),
        '.',
        TRIM(TRAILING '0' FROM RIGHT(`amount_raw`, `token_decimals`))
      )
    END
  ),
  CONSTRAINT `chk_fund_ledger_contract_version` CHECK (`contract_version` = 'v2_stablecoin'),
  CONSTRAINT `chk_fund_ledger_entry_type` CHECK (`entry_type` IN (
    'project_token_deposited','purchase_payment','payment_refunded','project_token_claimed',
    'project_proceeds_paid','platform_fee_paid','project_tokens_withdrawn'
  )),
  CONSTRAINT `chk_fund_ledger_asset_role` CHECK (`asset_role` IN ('payment', 'project')),
  CONSTRAINT `chk_fund_ledger_entry_asset_role` CHECK (
    (`entry_type` IN ('project_token_deposited','project_token_claimed','project_tokens_withdrawn') AND `asset_role` = 'project')
    OR
    (`entry_type` IN ('purchase_payment','payment_refunded','project_proceeds_paid','platform_fee_paid') AND `asset_role` = 'payment')
  ),
  CONSTRAINT `chk_fund_ledger_status` CHECK (`status` IN ('canonical', 'reversed', 'reconciliation'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DROP PROCEDURE IF EXISTS jc_assert_ido_v2_ledger_schema;
DELIMITER $$
CREATE PROCEDURE jc_assert_ido_v2_ledger_schema()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'projects'
      AND COLUMN_NAME IN (
        'payment_token_address','payment_token_decimals','chain_id','contract_address',
        'contract_version','onchain_locked','token_price_raw','hard_cap_raw','soft_cap_raw',
        'max_per_user_raw','total_raised_raw','total_deposited_raw'
      )
  ) <> 12 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'projects IDO V2 identity columns are incomplete';
  END IF;

  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'projects'
      AND (
        (COLUMN_NAME IN ('payment_token_address','contract_address') AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 42 AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
        OR (COLUMN_NAME = 'payment_token' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 20 AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
        OR (COLUMN_NAME IN ('payment_token_decimals') AND DATA_TYPE = 'tinyint' AND COLUMN_TYPE = 'tinyint unsigned' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0')
        OR (COLUMN_NAME = 'chain_id' AND DATA_TYPE = 'bigint' AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0')
        OR (COLUMN_NAME = 'contract_version' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 32 AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = 'unassigned')
        OR (COLUMN_NAME = 'onchain_locked' AND DATA_TYPE = 'tinyint' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0')
        OR (COLUMN_NAME IN ('token_price_raw','hard_cap_raw','soft_cap_raw','max_per_user_raw','total_raised_raw','total_deposited_raw') AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 78 AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0')
        OR (COLUMN_NAME IN ('token_price','hard_cap','soft_cap','max_per_user','total_raised','total_deposited') AND DATA_TYPE = 'decimal' AND NUMERIC_PRECISION = 65 AND NUMERIC_SCALE = 18 AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0.000000000000000000')
      )
  ) <> 19 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'projects IDO V2 column definitions are incompatible';
  END IF;

  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'fund_ledger'
      AND COLUMN_NAME IN (
        'id','chain_id','contract_address','contract_version','project_id','wallet_address',
        'counterparty_address','entry_type','asset_role','token_symbol','token_address',
        'token_decimals','amount_raw','amount_display','tx_hash','log_index','block_number',
        'block_hash','status','event_data','created_at','updated_at'
      )
  ) <> 22 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'fund_ledger schema is incomplete';
  END IF;

  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'fund_ledger'
      AND (
        (COLUMN_NAME = 'id' AND DATA_TYPE = 'bigint' AND COLUMN_TYPE = 'bigint unsigned' AND EXTRA LIKE '%auto_increment%' AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME IN ('chain_id','project_id','block_number') AND DATA_TYPE = 'bigint' AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME IN ('contract_address','token_address') AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 42 AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT IS NULL)
        OR (COLUMN_NAME IN ('wallet_address','counterparty_address') AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 42 AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
        OR (COLUMN_NAME IN ('contract_version','entry_type') AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 32 AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'asset_role' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 16 AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'token_symbol' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 20 AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
        OR (COLUMN_NAME = 'status' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 20 AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = 'canonical')
        OR (COLUMN_NAME = 'token_decimals' AND DATA_TYPE = 'tinyint' AND COLUMN_TYPE = 'tinyint unsigned' AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'amount_raw' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 78 AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'amount_display' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 160 AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME IN ('tx_hash','block_hash') AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 66 AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'log_index' AND DATA_TYPE = 'int' AND COLUMN_TYPE = 'int unsigned' AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'event_data' AND DATA_TYPE = 'json' AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'created_at' AND DATA_TYPE = 'datetime' AND DATETIME_PRECISION = 6 AND IS_NULLABLE = 'NO' AND LOWER(COLUMN_DEFAULT) = 'current_timestamp(6)')
        OR (COLUMN_NAME = 'updated_at' AND DATA_TYPE = 'datetime' AND DATETIME_PRECISION = 6 AND IS_NULLABLE = 'NO' AND LOWER(COLUMN_DEFAULT) = 'current_timestamp(6)' AND LOWER(EXTRA) LIKE '%on update current_timestamp(6)%')
      )
  ) <> 22 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'fund_ledger column definitions are incompatible';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM (
      SELECT INDEX_NAME,
             GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') AS columns_in_order,
             MIN(NON_UNIQUE) AS non_unique
      FROM information_schema.STATISTICS
      WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'fund_ledger'
      GROUP BY INDEX_NAME
    ) ledger_indexes
    WHERE columns_in_order = 'chain_id,tx_hash,log_index' AND non_unique = 0
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'fund_ledger event identity is missing';
  END IF;

  IF (
    SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'fund_ledger'
      AND CONSTRAINT_TYPE = 'CHECK'
      AND ENFORCED = 'YES'
      AND CONSTRAINT_NAME IN (
        'chk_fund_ledger_token_decimals','chk_fund_ledger_positive_identity',
        'chk_fund_ledger_contract_address','chk_fund_ledger_token_address',
        'chk_fund_ledger_tx_hash','chk_fund_ledger_block_hash','chk_fund_ledger_amount_raw',
        'chk_fund_ledger_amount_display',
        'chk_fund_ledger_contract_version','chk_fund_ledger_entry_type',
        'chk_fund_ledger_asset_role','chk_fund_ledger_entry_asset_role',
        'chk_fund_ledger_status'
      )
  ) <> 13 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'fund_ledger check constraints are incomplete';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.CHECK_CONSTRAINTS cc
    JOIN information_schema.TABLE_CONSTRAINTS tc
      ON tc.CONSTRAINT_SCHEMA = cc.CONSTRAINT_SCHEMA AND tc.CONSTRAINT_NAME = cc.CONSTRAINT_NAME
    WHERE cc.CONSTRAINT_SCHEMA = DATABASE() AND tc.TABLE_NAME = 'fund_ledger'
      AND cc.CONSTRAINT_NAME = 'chk_fund_ledger_amount_raw'
      AND cc.CHECK_CLAUSE LIKE '%115792089237316195423570985008687907853269984665640564039457584007913129639935%'
      AND cc.CHECK_CLAUSE LIKE '%amount_raw%'
  ) OR NOT EXISTS (
    SELECT 1 FROM information_schema.CHECK_CONSTRAINTS cc
    JOIN information_schema.TABLE_CONSTRAINTS tc
      ON tc.CONSTRAINT_SCHEMA = cc.CONSTRAINT_SCHEMA AND tc.CONSTRAINT_NAME = cc.CONSTRAINT_NAME
    WHERE cc.CONSTRAINT_SCHEMA = DATABASE() AND tc.TABLE_NAME = 'fund_ledger'
      AND cc.CONSTRAINT_NAME = 'chk_fund_ledger_amount_display'
      AND cc.CHECK_CLAUSE LIKE '%amount_display%'
      AND cc.CHECK_CLAUSE LIKE '%amount_raw%'
      AND cc.CHECK_CLAUSE LIKE '%token_decimals%'
  ) OR NOT EXISTS (
    SELECT 1 FROM information_schema.CHECK_CONSTRAINTS cc
    JOIN information_schema.TABLE_CONSTRAINTS tc
      ON tc.CONSTRAINT_SCHEMA = cc.CONSTRAINT_SCHEMA AND tc.CONSTRAINT_NAME = cc.CONSTRAINT_NAME
    WHERE cc.CONSTRAINT_SCHEMA = DATABASE() AND tc.TABLE_NAME = 'fund_ledger'
      AND cc.CONSTRAINT_NAME = 'chk_fund_ledger_entry_type'
      AND cc.CHECK_CLAUSE LIKE '%project_token_deposited%'
      AND cc.CHECK_CLAUSE LIKE '%purchase_payment%'
      AND cc.CHECK_CLAUSE LIKE '%payment_refunded%'
      AND cc.CHECK_CLAUSE LIKE '%project_token_claimed%'
      AND cc.CHECK_CLAUSE LIKE '%project_proceeds_paid%'
      AND cc.CHECK_CLAUSE LIKE '%platform_fee_paid%'
      AND cc.CHECK_CLAUSE LIKE '%project_tokens_withdrawn%'
  ) OR NOT EXISTS (
    SELECT 1 FROM information_schema.CHECK_CONSTRAINTS cc
    JOIN information_schema.TABLE_CONSTRAINTS tc
      ON tc.CONSTRAINT_SCHEMA = cc.CONSTRAINT_SCHEMA AND tc.CONSTRAINT_NAME = cc.CONSTRAINT_NAME
    WHERE cc.CONSTRAINT_SCHEMA = DATABASE() AND tc.TABLE_NAME = 'fund_ledger'
      AND cc.CONSTRAINT_NAME = 'chk_fund_ledger_entry_asset_role'
      AND cc.CHECK_CLAUSE LIKE '%entry_type%'
      AND cc.CHECK_CLAUSE LIKE '%asset_role%'
      AND cc.CHECK_CLAUSE LIKE '%payment%'
      AND cc.CHECK_CLAUSE LIKE '%project%'
  ) OR NOT EXISTS (
    SELECT 1 FROM information_schema.CHECK_CONSTRAINTS cc
    JOIN information_schema.TABLE_CONSTRAINTS tc
      ON tc.CONSTRAINT_SCHEMA = cc.CONSTRAINT_SCHEMA AND tc.CONSTRAINT_NAME = cc.CONSTRAINT_NAME
    WHERE cc.CONSTRAINT_SCHEMA = DATABASE() AND tc.TABLE_NAME = 'fund_ledger'
      AND cc.CONSTRAINT_NAME = 'chk_fund_ledger_contract_version'
      AND cc.CHECK_CLAUSE LIKE '%v2_stablecoin%'
  ) OR NOT EXISTS (
    SELECT 1 FROM information_schema.CHECK_CONSTRAINTS cc
    JOIN information_schema.TABLE_CONSTRAINTS tc
      ON tc.CONSTRAINT_SCHEMA = cc.CONSTRAINT_SCHEMA AND tc.CONSTRAINT_NAME = cc.CONSTRAINT_NAME
    WHERE cc.CONSTRAINT_SCHEMA = DATABASE() AND tc.TABLE_NAME = 'fund_ledger'
      AND cc.CONSTRAINT_NAME = 'chk_fund_ledger_status'
      AND cc.CHECK_CLAUSE LIKE '%canonical%'
      AND cc.CHECK_CLAUSE LIKE '%reversed%'
      AND cc.CHECK_CLAUSE LIKE '%reconciliation%'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'fund_ledger check constraint definitions are incompatible';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_ido_v2_ledger_schema();
DROP PROCEDURE jc_assert_ido_v2_ledger_schema;

-- Verification:
--   SHOW CREATE TABLE fund_ledger;
--   SELECT status,payment_token,contract_version,onchain_locked,COUNT(*) FROM projects GROUP BY 1,2,3,4;
