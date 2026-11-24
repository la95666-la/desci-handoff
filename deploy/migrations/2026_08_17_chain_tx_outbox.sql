-- JoinCare IDO Sprint 1A-P2 · durable relayer transaction outbox and nonce fence.
-- Signed raw transactions are public bearer payloads; they never contain private keys.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_tx_outbox;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_tx_outbox()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'transaction_record' AND TABLE_TYPE = 'BASE TABLE'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'transaction_record prerequisite table is missing';
  END IF;

  IF EXISTS (
    SELECT 1 FROM transaction_record
    WHERE type IN ('v2_create','v2_activate','project_end') AND TRIM(tx_hash) = ''
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'blank lifecycle transaction_record rows require manual chain reconciliation before outbox migration';
  END IF;

  IF EXISTS (
    SELECT 1 FROM transaction_record tr
    WHERE tr.type IN ('v2_create','v2_activate')
      AND TRIM(tr.tx_hash) <> ''
      AND NOT EXISTS (
        SELECT 1 FROM chain_event_record ce
        WHERE LOWER(ce.tx_hash) = LOWER(tr.tx_hash)
          AND ce.event = CASE tr.type WHEN 'v2_create' THEN 'ProjectCreated' ELSE 'ProjectActivated' END
      )
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'unconsumed legacy V2 lifecycle hashes require manual reconciliation before outbox migration';
  END IF;

  SET @outbox_tables = (
    SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME IN ('chain_tx_submission','chain_relayer_nonce')
  );
  IF @outbox_tables NOT IN (0, 2) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'partial chain transaction outbox schema requires manual recovery';
  END IF;
  IF @outbox_tables = 2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'pre-existing chain transaction outbox requires manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_tx_outbox();
DROP PROCEDURE jc_preflight_chain_tx_outbox;

CREATE TABLE `chain_tx_submission` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `action` VARCHAR(32) NOT NULL,
  `attempt_no` INT UNSIGNED NOT NULL,
  `chain_id` BIGINT UNSIGNED NOT NULL,
  `sender_address` VARCHAR(42) NOT NULL,
  `contract_address` VARCHAR(42) NOT NULL,
  `call_data` MEDIUMBLOB NOT NULL,
  `value_raw` VARCHAR(78) NOT NULL DEFAULT '0',
  `state` VARCHAR(32) NOT NULL,
  `nonce` BIGINT UNSIGNED NULL,
  `gas_limit` BIGINT UNSIGNED NULL,
  `gas_price_raw` VARCHAR(78) NOT NULL DEFAULT '0',
  `raw_tx` MEDIUMBLOB NULL,
  `tx_hash` VARCHAR(66) NULL,
  `active_slot` TINYINT UNSIGNED NULL,
  `broadcast_count` INT UNSIGNED NOT NULL DEFAULT 0,
  `last_broadcast_at` DATETIME(6) NULL,
  `last_error` TEXT NOT NULL,
  `receipt_status` BIGINT UNSIGNED NULL,
  `receipt_block_number` BIGINT UNSIGNED NULL,
  `receipt_block_hash` VARCHAR(66) NOT NULL DEFAULT '',
  `receipt_gas_used` BIGINT UNSIGNED NULL,
  `final_event_record_id` BIGINT UNSIGNED NULL,
  `finalized_at` DATETIME(6) NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_chain_tx_business_attempt` (`chain_id`,`contract_address`,`project_id`,`action`,`attempt_no`),
  UNIQUE KEY `uk_chain_tx_active_business` (`chain_id`,`contract_address`,`project_id`,`action`,`active_slot`),
  UNIQUE KEY `uk_chain_tx_sender_nonce` (`chain_id`,`sender_address`,`nonce`),
  UNIQUE KEY `uk_chain_tx_hash` (`tx_hash`),
  KEY `idx_chain_tx_recovery` (`state`,`updated_at`,`id`),
  CONSTRAINT `chk_chain_tx_project` CHECK (`project_id` > 0) ENFORCED,
  CONSTRAINT `chk_chain_tx_attempt` CHECK (`attempt_no` > 0) ENFORCED,
  CONSTRAINT `chk_chain_tx_chain` CHECK (`chain_id` > 0) ENFORCED,
  CONSTRAINT `chk_chain_tx_sender` CHECK (`sender_address` REGEXP '^0x[0-9A-Fa-f]{40}$' AND `sender_address` <> '0x0000000000000000000000000000000000000000') ENFORCED,
  CONSTRAINT `chk_chain_tx_contract` CHECK (`contract_address` REGEXP '^0x[0-9A-Fa-f]{40}$' AND `contract_address` <> '0x0000000000000000000000000000000000000000') ENFORCED,
  CONSTRAINT `chk_chain_tx_value` CHECK (`value_raw` = '0') ENFORCED,
  CONSTRAINT `chk_chain_tx_calldata` CHECK (OCTET_LENGTH(`call_data`) > 0) ENFORCED,
  CONSTRAINT `chk_chain_tx_action` CHECK (`action` IN ('v2_create','v2_activate','project_end')) ENFORCED,
  CONSTRAINT `chk_chain_tx_state` CHECK (`state` IN ('intent','ready','signed','submitted','broadcast_unknown','mined_success','mined_revert','final_success','final_revert','applied','cancelled','ambiguous')) ENFORCED,
  CONSTRAINT `chk_chain_tx_active` CHECK (`active_slot` IS NULL OR `active_slot` = 1) ENFORCED,
  CONSTRAINT `chk_chain_tx_active_state` CHECK (
    (`state` IN ('applied','cancelled','final_revert') AND `active_slot` IS NULL)
    OR (`state` NOT IN ('applied','cancelled','final_revert') AND `active_slot` = 1)
  ) ENFORCED,
  CONSTRAINT `chk_chain_tx_hash_format` CHECK (`tx_hash` IS NULL OR `tx_hash` REGEXP '^0x[0-9A-Fa-f]{64}$') ENFORCED,
  CONSTRAINT `chk_chain_tx_receipt_hash` CHECK (`receipt_block_hash` = '' OR `receipt_block_hash` REGEXP '^0x[0-9A-Fa-f]{64}$') ENFORCED,
  CONSTRAINT `chk_chain_tx_receipt_status` CHECK (`receipt_status` IS NULL OR `receipt_status` IN (0,1)) ENFORCED,
  CONSTRAINT `chk_chain_tx_gas_price` CHECK (`gas_price_raw` REGEXP '^(0|[1-9][0-9]{0,64})$') ENFORCED,
  CONSTRAINT `chk_chain_tx_signed_shape` CHECK (
    (`state` IN ('intent','ready','cancelled') AND `nonce` IS NULL AND `gas_limit` IS NULL AND `raw_tx` IS NULL AND `tx_hash` IS NULL)
    OR (`state` NOT IN ('intent','ready','cancelled') AND `nonce` IS NOT NULL AND `gas_limit` IS NOT NULL AND OCTET_LENGTH(`raw_tx`) > 0 AND `tx_hash` IS NOT NULL)
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `chain_relayer_nonce` (
  `chain_id` BIGINT UNSIGNED NOT NULL,
  `sender_address` VARCHAR(42) NOT NULL,
  `next_nonce` BIGINT UNSIGNED NOT NULL,
  `active_submission_id` BIGINT UNSIGNED NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`chain_id`,`sender_address`),
  UNIQUE KEY `uk_chain_relayer_active_submission` (`active_submission_id`),
  CONSTRAINT `chk_chain_relayer_chain` CHECK (`chain_id` > 0) ENFORCED,
  CONSTRAINT `chk_chain_relayer_sender` CHECK (`sender_address` REGEXP '^0x[0-9A-Fa-f]{40}$' AND `sender_address` <> '0x0000000000000000000000000000000000000000') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DROP PROCEDURE IF EXISTS jc_assert_chain_tx_outbox_schema;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_tx_outbox_schema()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_tx_submission'
      AND COLUMN_NAME IN ('id','project_id','action','attempt_no','chain_id','sender_address','contract_address','call_data','value_raw','state','nonce','gas_limit','gas_price_raw','raw_tx','tx_hash','active_slot','broadcast_count','last_broadcast_at','last_error','receipt_status','receipt_block_number','receipt_block_hash','receipt_gas_used','final_event_record_id','finalized_at','created_at','updated_at')
  ) <> 27 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain_tx_submission schema is incomplete';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_relayer_nonce'
      AND COLUMN_NAME IN ('chain_id','sender_address','next_nonce','active_submission_id','created_at','updated_at')
  ) <> 6 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain_relayer_nonce schema is incomplete';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND (
      (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME = 'id' AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO' AND EXTRA LIKE '%auto_increment%')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME IN ('project_id','chain_id') AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME = 'action' AND COLUMN_TYPE = 'varchar(32)' AND IS_NULLABLE = 'NO')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME = 'attempt_no' AND COLUMN_TYPE = 'int unsigned' AND IS_NULLABLE = 'NO')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME IN ('sender_address','contract_address') AND COLUMN_TYPE = 'varchar(42)' AND IS_NULLABLE = 'NO')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME IN ('call_data','raw_tx') AND DATA_TYPE = 'mediumblob')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME IN ('value_raw','gas_price_raw') AND COLUMN_TYPE = 'varchar(78)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT IN ('0'))
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME = 'state' AND COLUMN_TYPE = 'varchar(32)' AND IS_NULLABLE = 'NO')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME IN ('nonce','gas_limit','receipt_status','receipt_block_number','receipt_gas_used','final_event_record_id') AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'YES')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME = 'tx_hash' AND COLUMN_TYPE = 'varchar(66)' AND IS_NULLABLE = 'YES')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME = 'active_slot' AND COLUMN_TYPE = 'tinyint unsigned' AND IS_NULLABLE = 'YES')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME = 'broadcast_count' AND COLUMN_TYPE = 'int unsigned' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME = 'last_error' AND DATA_TYPE = 'text' AND IS_NULLABLE = 'NO')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME = 'receipt_block_hash' AND COLUMN_TYPE = 'varchar(66)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
      OR (TABLE_NAME = 'chain_relayer_nonce' AND COLUMN_NAME IN ('chain_id','next_nonce') AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO')
      OR (TABLE_NAME = 'chain_relayer_nonce' AND COLUMN_NAME = 'sender_address' AND COLUMN_TYPE = 'varchar(42)' AND IS_NULLABLE = 'NO')
      OR (TABLE_NAME = 'chain_relayer_nonce' AND COLUMN_NAME = 'active_submission_id' AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'YES')
    )
  ) <> 27 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain transaction outbox critical column definitions are incompatible';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_tx_submission'
      AND CONSTRAINT_TYPE = 'CHECK' AND ENFORCED = 'YES'
      AND CONSTRAINT_NAME IN ('chk_chain_tx_project','chk_chain_tx_attempt','chk_chain_tx_chain','chk_chain_tx_sender','chk_chain_tx_contract','chk_chain_tx_value','chk_chain_tx_calldata','chk_chain_tx_action','chk_chain_tx_state','chk_chain_tx_active','chk_chain_tx_active_state','chk_chain_tx_hash_format','chk_chain_tx_receipt_hash','chk_chain_tx_receipt_status','chk_chain_tx_gas_price','chk_chain_tx_signed_shape')
  ) <> 16 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain_tx_submission enforced checks are incomplete';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM (
      SELECT INDEX_NAME, GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') AS columns_in_order, MIN(NON_UNIQUE) AS non_unique
      FROM information_schema.STATISTICS
      WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_tx_submission'
      GROUP BY INDEX_NAME
    ) indexes_for_submission
    WHERE (INDEX_NAME = 'uk_chain_tx_business_attempt' AND columns_in_order = 'chain_id,contract_address,project_id,action,attempt_no' AND non_unique = 0)
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain transaction business attempt fence is missing';
  END IF;
  IF (
    SELECT COUNT(*) FROM (
      SELECT INDEX_NAME, GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') AS columns_in_order, MIN(NON_UNIQUE) AS non_unique
      FROM information_schema.STATISTICS
      WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_tx_submission'
      GROUP BY INDEX_NAME
    ) indexes_for_submission
    WHERE non_unique = 0 AND (
      (INDEX_NAME = 'uk_chain_tx_active_business' AND columns_in_order = 'chain_id,contract_address,project_id,action,active_slot')
      OR (INDEX_NAME = 'uk_chain_tx_sender_nonce' AND columns_in_order = 'chain_id,sender_address,nonce')
      OR (INDEX_NAME = 'uk_chain_tx_hash' AND columns_in_order = 'tx_hash')
    )
  ) <> 3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain transaction unique fences are incomplete';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_relayer_nonce'
      AND INDEX_NAME = 'uk_chain_relayer_active_submission' AND COLUMN_NAME = 'active_submission_id' AND NON_UNIQUE = 0
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain relayer active submission fence is missing';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_tx_outbox_schema();
DROP PROCEDURE jc_assert_chain_tx_outbox_schema;

-- Rollout requires exclusive nonce ownership for the configured relayer key.
-- Do not insert chain_relayer_nonce manually until latest and pending RPC nonce
-- plus all historical pending transactions have been audited.
