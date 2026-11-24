-- JoinCare IDO Sprint 1A-P2 · durable operational on-chain reconciliation evidence.
-- This is database-local evidence (digest chained), not an external KMS/WORM attestation.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_ido_reconciliation_evidence;
DELIMITER $$
CREATE PROCEDURE jc_preflight_ido_reconciliation_evidence()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
      'projects','fund_ledger','ido_purchase_entitlement','chain_event_sync_state','chain_event_failure','chain_tx_submission'
    ) AND TABLE_TYPE='BASE TABLE'
  ) <> 6 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation prerequisite tables are missing';
  END IF;
  IF EXISTS (
    SELECT 1 FROM information_schema.TABLES
    WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
      'ido_reconciliation_run','ido_reconciliation_job','ido_reconciliation_event',
      'ido_reconciliation_wallet','ido_reconciliation_check','ido_reconciliation_manifest'
    )
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='pre-existing IDO reconciliation evidence tables require manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_ido_reconciliation_evidence();
DROP PROCEDURE jc_preflight_ido_reconciliation_evidence;

CREATE TABLE `ido_reconciliation_run` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `run_uuid` CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `chain_id` BIGINT UNSIGNED NOT NULL,
  `contract_address` VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `requested_by_admin_id` BIGINT UNSIGNED NOT NULL,
  `requested_role_codes` VARCHAR(500) NOT NULL,
  `reason` VARCHAR(500) NOT NULL,
  `idempotency_key` VARCHAR(100) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `algorithm_version` VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `canonicalization_version` VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `evidence_block_number` BIGINT UNSIGNED NOT NULL,
  `evidence_block_hash` VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `implementation_address` VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `implementation_code_hash` VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `confirmation_depth` BIGINT UNSIGNED NOT NULL,
  `max_healthy_lag_blocks` BIGINT UNSIGNED NOT NULL,
  `expected_wallet_count` INT UNSIGNED NOT NULL,
  `expected_check_count` INT UNSIGNED NOT NULL,
  `expected_wallet_identity_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `expected_check_identity_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `source_payload` MEDIUMBLOB NOT NULL,
  `source_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `requested_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_ido_reconciliation_run_uuid` (`run_uuid`),
  UNIQUE KEY `uk_ido_reconciliation_request_idempotency` (`requested_by_admin_id`,`idempotency_key`),
  KEY `idx_ido_reconciliation_project_time` (`project_id`,`requested_at`,`id`),
  CONSTRAINT `chk_ido_reconciliation_run_identity` CHECK (`project_id`>0 AND `chain_id`>0 AND `requested_by_admin_id`>0 AND `evidence_block_number`>0) ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_run_addresses` CHECK (
    `contract_address` REGEXP '^0x[0-9a-f]{40}$' AND `contract_address`<>'0x0000000000000000000000000000000000000000'
    AND `implementation_address` REGEXP '^0x[0-9a-f]{40}$' AND `implementation_address`<>'0x0000000000000000000000000000000000000000'
  ) ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_run_hashes` CHECK (
    `evidence_block_hash` REGEXP '^0x[0-9a-f]{64}$' AND `implementation_code_hash` REGEXP '^0x[0-9a-f]{64}$'
    AND `source_digest` REGEXP '^[0-9a-f]{64}$'
    AND `expected_wallet_identity_digest` REGEXP '^[0-9a-f]{64}$'
    AND `expected_check_identity_digest` REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_run_request` CHECK (
    `run_uuid` REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
    AND CHAR_LENGTH(TRIM(`reason`)) BETWEEN 3 AND 500
    AND CHAR_LENGTH(`idempotency_key`) BETWEEN 16 AND 100
    AND CHAR_LENGTH(`requested_role_codes`) BETWEEN 1 AND 500
  ) ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_run_versions` CHECK (
    `algorithm_version`='joincare-ido-reconciliation-v1'
    AND `canonicalization_version`='canonical-json-v1'
    AND `max_healthy_lag_blocks`>0
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `ido_reconciliation_job` (
  `run_id` BIGINT UNSIGNED NOT NULL,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `state` VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'queued',
  `outcome` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NULL,
  `active_slot` TINYINT UNSIGNED NULL DEFAULT 1,
  `attempt_count` INT UNSIGNED NOT NULL DEFAULT 0,
  `consecutive_error_count` INT UNSIGNED NOT NULL DEFAULT 0,
  `next_wallet_ordinal` INT UNSIGNED NOT NULL DEFAULT 0,
  `next_attempt_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `lease_token` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  `lease_expires_at` DATETIME(6) NULL,
  `last_error_code` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  `last_error_message` VARCHAR(500) NOT NULL DEFAULT '',
  `cancel_requested_by_admin_id` BIGINT UNSIGNED NULL,
  `cancel_reason` VARCHAR(500) NULL,
  `cancel_requested_at` DATETIME(6) NULL,
  `started_at` DATETIME(6) NULL,
  `completed_at` DATETIME(6) NULL,
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`run_id`),
  UNIQUE KEY `uk_ido_reconciliation_active_project` (`project_id`,`active_slot`),
  KEY `idx_ido_reconciliation_job_claim` (`state`,`next_attempt_at`,`lease_expires_at`,`run_id`),
  KEY `idx_ido_reconciliation_job_lease` (`lease_expires_at`,`state`),
  CONSTRAINT `fk_ido_reconciliation_job_run` FOREIGN KEY (`run_id`) REFERENCES `ido_reconciliation_run` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `chk_ido_reconciliation_job_state` CHECK (`state` IN ('queued','running','retrying','cancel_requested','finalizing','completed','failed','invalidated','cancelled')) ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_job_outcome` CHECK (
    (`state`='completed' AND `outcome` IN ('BALANCED','UNBALANCED') AND `active_slot` IS NULL AND `completed_at` IS NOT NULL)
    OR (`state`<>'completed' AND `outcome` IS NULL)
  ) ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_job_active` CHECK (
    (`state` IN ('queued','running','retrying','cancel_requested','finalizing') AND `active_slot`=1 AND `completed_at` IS NULL)
    OR (`state` IN ('completed','failed','invalidated','cancelled') AND `active_slot` IS NULL AND `completed_at` IS NOT NULL)
  ) ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_job_lease_fields` CHECK (
    (`state` IN ('running','cancel_requested','finalizing') AND `lease_token` REGEXP '^[0-9a-f]{64}$' AND `lease_expires_at` IS NOT NULL)
    OR (`state` NOT IN ('running','cancel_requested','finalizing') AND `lease_token`='' AND `lease_expires_at` IS NULL)
  ) ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_job_cancel` CHECK (
    (`cancel_requested_by_admin_id` IS NULL AND `cancel_reason` IS NULL AND `cancel_requested_at` IS NULL)
    OR (`cancel_requested_by_admin_id`>0 AND CHAR_LENGTH(TRIM(`cancel_reason`)) BETWEEN 3 AND 500 AND `cancel_requested_at` IS NOT NULL)
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `ido_reconciliation_event` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `run_id` BIGINT UNSIGNED NOT NULL,
  `sequence_no` INT UNSIGNED NOT NULL,
  `event_type` VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `stage` VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `error_code` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  `message` VARCHAR(500) NOT NULL DEFAULT '',
  `actor_admin_id` BIGINT UNSIGNED NULL,
  `lease_token_hash` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  `previous_event_hash` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `event_hash` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_ido_reconciliation_event_sequence` (`run_id`,`sequence_no`),
  UNIQUE KEY `uk_ido_reconciliation_event_hash` (`event_hash`),
  KEY `idx_ido_reconciliation_event_type` (`run_id`,`event_type`,`created_at`),
  CONSTRAINT `fk_ido_reconciliation_event_run` FOREIGN KEY (`run_id`) REFERENCES `ido_reconciliation_run` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `chk_ido_reconciliation_event_fields` CHECK (
    `sequence_no`>0 AND `event_type` IN ('QUEUED','STARTED','PAGE_COMPLETED','RETRY_SCHEDULED','CANCEL_REQUESTED','CANCELLED','FAILED','INVALIDATED','FINALIZED')
    AND CHAR_LENGTH(`stage`) BETWEEN 1 AND 32
    AND `previous_event_hash` REGEXP '^[0-9a-f]{64}$' AND `event_hash` REGEXP '^[0-9a-f]{64}$'
    AND (`lease_token_hash`='' OR `lease_token_hash` REGEXP '^[0-9a-f]{64}$')
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `ido_reconciliation_wallet` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `run_id` BIGINT UNSIGNED NOT NULL,
  `ordinal` INT UNSIGNED NOT NULL,
  `wallet_address` VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `expected_payment_raw` VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `actual_payment_raw` VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `expected_tokens_raw` VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `actual_tokens_raw` VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `expected_claimed_raw` VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `actual_claimed_raw` VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `expected_refunded` TINYINT(1) NOT NULL,
  `actual_refunded` TINYINT(1) NOT NULL,
  `previous_row_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `identity_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `row_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `observed_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_ido_reconciliation_wallet_ordinal` (`run_id`,`ordinal`),
  UNIQUE KEY `uk_ido_reconciliation_wallet_address` (`run_id`,`wallet_address`),
  KEY `idx_ido_reconciliation_wallet_lookup` (`wallet_address`,`run_id`),
  CONSTRAINT `fk_ido_reconciliation_wallet_run` FOREIGN KEY (`run_id`) REFERENCES `ido_reconciliation_run` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `chk_ido_reconciliation_wallet_identity` CHECK (`wallet_address` REGEXP '^0x[0-9a-f]{40}$' AND `wallet_address`<>'0x0000000000000000000000000000000000000000') ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_wallet_raw` CHECK (
    `expected_payment_raw` REGEXP '^(0|[1-9][0-9]{0,77})$' AND `actual_payment_raw` REGEXP '^(0|[1-9][0-9]{0,77})$'
    AND `expected_tokens_raw` REGEXP '^(0|[1-9][0-9]{0,77})$' AND `actual_tokens_raw` REGEXP '^(0|[1-9][0-9]{0,77})$'
    AND `expected_claimed_raw` REGEXP '^(0|[1-9][0-9]{0,77})$' AND `actual_claimed_raw` REGEXP '^(0|[1-9][0-9]{0,77})$'
    AND (CHAR_LENGTH(`expected_payment_raw`)<78 OR `expected_payment_raw`<='115792089237316195423570985008687907853269984665640564039457584007913129639935')
    AND (CHAR_LENGTH(`actual_payment_raw`)<78 OR `actual_payment_raw`<='115792089237316195423570985008687907853269984665640564039457584007913129639935')
    AND (CHAR_LENGTH(`expected_tokens_raw`)<78 OR `expected_tokens_raw`<='115792089237316195423570985008687907853269984665640564039457584007913129639935')
    AND (CHAR_LENGTH(`actual_tokens_raw`)<78 OR `actual_tokens_raw`<='115792089237316195423570985008687907853269984665640564039457584007913129639935')
    AND (CHAR_LENGTH(`expected_claimed_raw`)<78 OR `expected_claimed_raw`<='115792089237316195423570985008687907853269984665640564039457584007913129639935')
    AND (CHAR_LENGTH(`actual_claimed_raw`)<78 OR `actual_claimed_raw`<='115792089237316195423570985008687907853269984665640564039457584007913129639935')
    AND `expected_refunded` IN (0,1) AND `actual_refunded` IN (0,1)
    AND `previous_row_digest` REGEXP '^[0-9a-f]{64}$' AND `identity_digest` REGEXP '^[0-9a-f]{64}$'
    AND `row_digest` REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `ido_reconciliation_manifest` (
  `run_id` BIGINT UNSIGNED NOT NULL,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `outcome` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `wallet_count` INT UNSIGNED NOT NULL,
  `check_count` INT UNSIGNED NOT NULL,
  `failed_check_count` INT UNSIGNED NOT NULL,
  `payload` MEDIUMBLOB NOT NULL,
  `payload_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `wallet_evidence_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `check_evidence_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `previous_manifest_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `manifest_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `completed_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`run_id`),
  UNIQUE KEY `uk_ido_reconciliation_manifest_digest` (`manifest_digest`),
  UNIQUE KEY `uk_ido_reconciliation_manifest_parent` (`project_id`,`previous_manifest_digest`),
  KEY `idx_ido_reconciliation_manifest_project` (`project_id`,`run_id`),
  CONSTRAINT `fk_ido_reconciliation_manifest_run` FOREIGN KEY (`run_id`) REFERENCES `ido_reconciliation_run` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `chk_ido_reconciliation_manifest_outcome` CHECK (
    (`outcome`='BALANCED' AND `failed_check_count`=0 AND `check_count`>0)
    OR (`outcome`='UNBALANCED' AND `failed_check_count`>0 AND `failed_check_count`<=`check_count`)
  ) ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_manifest_hashes` CHECK (
    `payload_digest` REGEXP '^[0-9a-f]{64}$' AND `wallet_evidence_digest` REGEXP '^[0-9a-f]{64}$'
    AND `check_evidence_digest` REGEXP '^[0-9a-f]{64}$' AND `previous_manifest_digest` REGEXP '^[0-9a-f]{64}$'
    AND `manifest_digest` REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `ido_reconciliation_check` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `run_id` BIGINT UNSIGNED NOT NULL,
  `ordinal` INT UNSIGNED NOT NULL,
  `scope` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `code` VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `ok` TINYINT(1) NOT NULL,
  `expected_exact` VARCHAR(500) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `actual_exact` VARCHAR(500) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `message` VARCHAR(500) NOT NULL,
  `previous_row_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `identity_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `row_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_ido_reconciliation_check_ordinal` (`run_id`,`ordinal`),
  UNIQUE KEY `uk_ido_reconciliation_check_code` (`run_id`,`scope`,`code`),
  CONSTRAINT `fk_ido_reconciliation_check_run` FOREIGN KEY (`run_id`) REFERENCES `ido_reconciliation_run` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `chk_ido_reconciliation_check_fields` CHECK (
    `scope` IN ('database','onchain') AND CHAR_LENGTH(`code`) BETWEEN 1 AND 160 AND `ok` IN (0,1)
    AND `previous_row_digest` REGEXP '^[0-9a-f]{64}$' AND `identity_digest` REGEXP '^[0-9a-f]{64}$'
    AND `row_digest` REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DROP TRIGGER IF EXISTS trg_ido_reconciliation_run_no_update;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_run_no_delete;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_run_validate_insert;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_event_validate_insert;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_event_no_update;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_event_no_delete;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_wallet_validate_insert;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_wallet_no_update;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_wallet_no_delete;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_manifest_validate_insert;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_manifest_no_update;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_manifest_no_delete;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_job_validate_update;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_job_no_delete;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_job_validate_insert;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_check_validate_insert;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_check_no_update;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_check_no_delete;
DELIMITER $$
CREATE TRIGGER trg_ido_reconciliation_run_validate_insert BEFORE INSERT ON ido_reconciliation_run FOR EACH ROW
BEGIN
  IF JSON_VALID(CAST(NEW.source_payload AS CHAR))=0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation source payload is not valid JSON';
  END IF;
  IF NEW.source_digest<>LOWER(SHA2(NEW.source_payload,256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation source digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_ido_reconciliation_run_no_update BEFORE UPDATE ON ido_reconciliation_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation run rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_run_no_delete BEFORE DELETE ON ido_reconciliation_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation run rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_event_no_update BEFORE UPDATE ON ido_reconciliation_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation event rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_event_no_delete BEFORE DELETE ON ido_reconciliation_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation event rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_event_validate_insert BEFORE INSERT ON ido_reconciliation_event FOR EACH ROW
BEGIN
  DECLARE prior_sequence INT UNSIGNED DEFAULT 0;
  DECLARE prior_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  DECLARE current_state VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin;
  IF EXISTS (SELECT 1 FROM ido_reconciliation_event WHERE run_id=NEW.run_id AND event_type IN ('CANCELLED','FAILED','INVALIDATED','FINALIZED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='terminal IDO reconciliation event already exists';
  END IF;
  SELECT state INTO current_state FROM ido_reconciliation_job WHERE run_id=NEW.run_id;
  IF current_state IS NULL OR NOT ((NEW.event_type='QUEUED' AND current_state='queued')
       OR (NEW.event_type IN ('STARTED','PAGE_COMPLETED') AND current_state='running')
       OR (NEW.event_type='RETRY_SCHEDULED' AND current_state='retrying')
       OR (NEW.event_type='CANCEL_REQUESTED' AND current_state='cancel_requested')
       OR (NEW.event_type='CANCELLED' AND current_state='cancelled')
       OR (NEW.event_type='FAILED' AND current_state='failed')
       OR (NEW.event_type='INVALIDATED' AND current_state='invalidated')
       OR (NEW.event_type='FINALIZED' AND current_state='completed')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation event does not match job state';
  END IF;
  SET prior_sequence=COALESCE((SELECT sequence_no FROM ido_reconciliation_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),0);
  SET prior_hash=COALESCE((SELECT event_hash FROM ido_reconciliation_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),REPEAT('0',64));
  IF NEW.sequence_no<>prior_sequence+1 OR NEW.previous_event_hash<>prior_hash THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation event chain is not contiguous';
  END IF;
  IF NEW.event_hash<>LOWER(SHA2(CONCAT(
    'joincare.ido.reconciliation.event.v1',CHAR(10),NEW.run_id,CHAR(10),NEW.sequence_no,CHAR(10),
    NEW.event_type,CHAR(10),NEW.stage,CHAR(10),NEW.error_code,CHAR(10),NEW.message,CHAR(10),
    COALESCE(NEW.actor_admin_id,0),CHAR(10),NEW.lease_token_hash,CHAR(10),NEW.previous_event_hash
  ),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation event digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_ido_reconciliation_wallet_validate_insert BEFORE INSERT ON ido_reconciliation_wallet FOR EACH ROW
BEGIN
  DECLARE prior_ordinal INT UNSIGNED DEFAULT 0;
  DECLARE prior_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  DECLARE prior_identity_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  IF NOT EXISTS (SELECT 1 FROM ido_reconciliation_job j WHERE j.run_id=NEW.run_id AND j.state IN ('running','finalizing') AND j.lease_expires_at>CURRENT_TIMESTAMP(6))
     OR EXISTS (SELECT 1 FROM ido_reconciliation_manifest m WHERE m.run_id=NEW.run_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='wallet evidence requires an active unfinalized reconciliation job';
  END IF;
  SET prior_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM ido_reconciliation_wallet WHERE run_id=NEW.run_id),0);
  SET prior_hash=COALESCE((SELECT row_digest FROM ido_reconciliation_wallet WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET prior_identity_hash=COALESCE((SELECT identity_digest FROM ido_reconciliation_wallet WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  IF NEW.ordinal<>prior_ordinal OR NEW.previous_row_digest<>prior_hash THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation wallet evidence chain is not contiguous';
  END IF;
  IF NEW.identity_digest<>LOWER(SHA2(CONCAT(
    'joincare.ido.reconciliation.wallet-identity.v1',CHAR(10),NEW.ordinal,CHAR(10),NEW.wallet_address,CHAR(10),
    NEW.expected_payment_raw,CHAR(10),NEW.expected_tokens_raw,CHAR(10),NEW.expected_claimed_raw,CHAR(10),
    NEW.expected_refunded,CHAR(10),prior_identity_hash
  ),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation wallet identity digest mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT(
    'joincare.ido.reconciliation.wallet.v1',CHAR(10),NEW.run_id,CHAR(10),NEW.ordinal,CHAR(10),
    NEW.wallet_address,CHAR(10),NEW.expected_payment_raw,CHAR(10),NEW.actual_payment_raw,CHAR(10),
    NEW.expected_tokens_raw,CHAR(10),NEW.actual_tokens_raw,CHAR(10),NEW.expected_claimed_raw,CHAR(10),
    NEW.actual_claimed_raw,CHAR(10),NEW.expected_refunded,CHAR(10),NEW.actual_refunded,CHAR(10),NEW.previous_row_digest
  ),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation wallet digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_ido_reconciliation_wallet_no_update BEFORE UPDATE ON ido_reconciliation_wallet FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation wallet rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_wallet_no_delete BEFORE DELETE ON ido_reconciliation_wallet FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation wallet rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_check_validate_insert BEFORE INSERT ON ido_reconciliation_check FOR EACH ROW
BEGIN
  DECLARE prior_ordinal INT UNSIGNED DEFAULT 0;
  DECLARE prior_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  DECLARE prior_identity_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  IF NOT EXISTS (SELECT 1 FROM ido_reconciliation_job j WHERE j.run_id=NEW.run_id AND j.state='finalizing' AND j.lease_expires_at>CURRENT_TIMESTAMP(6))
     OR EXISTS (SELECT 1 FROM ido_reconciliation_manifest m WHERE m.run_id=NEW.run_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='check evidence requires a finalizing unfinalized reconciliation job';
  END IF;
  SET prior_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM ido_reconciliation_check WHERE run_id=NEW.run_id),0);
  SET prior_hash=COALESCE((SELECT row_digest FROM ido_reconciliation_check WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET prior_identity_hash=COALESCE((SELECT identity_digest FROM ido_reconciliation_check WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  IF NEW.ordinal<>prior_ordinal OR NEW.previous_row_digest<>prior_hash THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation check evidence chain is not contiguous';
  END IF;
  IF NEW.identity_digest<>LOWER(SHA2(CONCAT(
    'joincare.ido.reconciliation.check-identity.v1',CHAR(10),NEW.ordinal,CHAR(10),NEW.scope,CHAR(10),NEW.code,CHAR(10),prior_identity_hash
  ),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation check identity digest mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT(
    'joincare.ido.reconciliation.check.v1',CHAR(10),NEW.run_id,CHAR(10),NEW.ordinal,CHAR(10),
    NEW.scope,CHAR(10),NEW.code,CHAR(10),NEW.ok,CHAR(10),NEW.expected_exact,CHAR(10),NEW.actual_exact,CHAR(10),NEW.message,CHAR(10),NEW.previous_row_digest
  ),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation check digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_ido_reconciliation_check_no_update BEFORE UPDATE ON ido_reconciliation_check FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation check rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_check_no_delete BEFORE DELETE ON ido_reconciliation_check FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation check rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_manifest_validate_insert BEFORE INSERT ON ido_reconciliation_manifest FOR EACH ROW
BEGIN
  DECLARE expected_wallets INT UNSIGNED;
  DECLARE actual_wallets INT UNSIGNED;
  DECLARE actual_checks INT UNSIGNED;
  DECLARE actual_failed_checks INT UNSIGNED;
  DECLARE wallet_failed_checks INT UNSIGNED;
  DECLARE last_wallet_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  DECLARE last_check_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  DECLARE last_wallet_identity_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  DECLARE last_check_identity_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  DECLARE expected_checks INT UNSIGNED;
  DECLARE expected_wallet_identity_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin;
  DECLARE expected_check_identity_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin;
  DECLARE prior_manifest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  SELECT r.expected_wallet_count,r.expected_check_count,r.expected_wallet_identity_digest,r.expected_check_identity_digest
    INTO expected_wallets,expected_checks,expected_wallet_identity_digest,expected_check_identity_digest FROM ido_reconciliation_run r
    JOIN ido_reconciliation_job j ON j.run_id=r.id
    WHERE r.id=NEW.run_id AND r.project_id=NEW.project_id AND j.state='finalizing' AND j.lease_expires_at>CURRENT_TIMESTAMP(6) FOR UPDATE;
  IF expected_wallets IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='manifest requires finalizing reconciliation job'; END IF;
  SELECT COUNT(*) INTO actual_wallets FROM ido_reconciliation_wallet WHERE run_id=NEW.run_id;
  IF actual_wallets<>expected_wallets OR NEW.wallet_count<>expected_wallets THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='manifest wallet evidence is incomplete';
  END IF;
  SELECT COALESCE(SUM(
    (expected_payment_raw<>actual_payment_raw)+(expected_tokens_raw<>actual_tokens_raw)+
    (expected_claimed_raw<>actual_claimed_raw)+(expected_refunded<>actual_refunded)
  ),0) INTO wallet_failed_checks FROM ido_reconciliation_wallet WHERE run_id=NEW.run_id;
  SET last_wallet_digest=COALESCE((SELECT row_digest FROM ido_reconciliation_wallet WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET last_wallet_identity_digest=COALESCE((SELECT identity_digest FROM ido_reconciliation_wallet WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  IF NEW.wallet_evidence_digest<>last_wallet_digest THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='manifest wallet evidence digest does not match';
  END IF;
  IF last_wallet_identity_digest<>expected_wallet_identity_digest THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='manifest wallet identity set does not match frozen source';
  END IF;
  SELECT COUNT(*),COALESCE(SUM(CASE WHEN ok=0 THEN 1 ELSE 0 END),0) INTO actual_checks,actual_failed_checks
    FROM ido_reconciliation_check WHERE run_id=NEW.run_id;
  IF actual_checks+(actual_wallets*4)<>NEW.check_count OR NEW.check_count<>expected_checks
     OR actual_failed_checks+wallet_failed_checks<>NEW.failed_check_count THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='manifest check evidence counts do not match';
  END IF;
  SET last_check_digest=COALESCE((SELECT row_digest FROM ido_reconciliation_check WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET last_check_identity_digest=COALESCE((SELECT identity_digest FROM ido_reconciliation_check WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  IF NEW.check_evidence_digest<>last_check_digest THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='manifest check evidence digest does not match';
  END IF;
  IF last_check_identity_digest<>expected_check_identity_digest THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='manifest check identity set does not match frozen source';
  END IF;
  IF NOT ((NEW.outcome='BALANCED' AND NEW.failed_check_count=0 AND NEW.check_count>0)
       OR (NEW.outcome='UNBALANCED' AND NEW.failed_check_count>0 AND NEW.failed_check_count<=NEW.check_count)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='manifest outcome does not match check counts';
  END IF;
  IF NEW.payload_digest<>LOWER(SHA2(NEW.payload,256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation manifest payload digest mismatch';
  END IF;
  IF JSON_VALID(CAST(NEW.payload AS CHAR))=0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation manifest payload is not valid JSON';
  END IF;
  SET prior_manifest=COALESCE((SELECT manifest_digest FROM ido_reconciliation_manifest
    WHERE project_id=NEW.project_id ORDER BY run_id DESC LIMIT 1),REPEAT('0',64));
  IF NEW.previous_manifest_digest<>prior_manifest THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation manifest chain mismatch';
  END IF;
  IF NEW.manifest_digest<>LOWER(SHA2(CONCAT(
    'joincare.ido.reconciliation.manifest.v1',CHAR(10),NEW.run_id,CHAR(10),NEW.project_id,CHAR(10),
    NEW.outcome,CHAR(10),NEW.wallet_count,CHAR(10),NEW.check_count,CHAR(10),NEW.failed_check_count,CHAR(10),
    NEW.payload_digest,CHAR(10),NEW.wallet_evidence_digest,CHAR(10),NEW.check_evidence_digest,CHAR(10),NEW.previous_manifest_digest
  ),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation manifest digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_ido_reconciliation_manifest_no_update BEFORE UPDATE ON ido_reconciliation_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation manifest rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_manifest_no_delete BEFORE DELETE ON ido_reconciliation_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation manifest rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_job_validate_update BEFORE UPDATE ON ido_reconciliation_job FOR EACH ROW
BEGIN
  IF NEW.run_id<>OLD.run_id OR NEW.project_id<>OLD.project_id OR NEW.attempt_count<OLD.attempt_count OR NEW.next_wallet_ordinal<OLD.next_wallet_ordinal THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation job identity or monotonic progress changed';
  END IF;
  IF OLD.state IN ('completed','failed','invalidated','cancelled') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='terminal IDO reconciliation jobs are immutable';
  END IF;
  IF NOT (
    (OLD.state='queued' AND NEW.state IN ('running','cancelled'))
    OR (OLD.state='running' AND NEW.state IN ('running','retrying','cancel_requested','finalizing','failed','invalidated'))
    OR (OLD.state='retrying' AND NEW.state IN ('running','cancelled'))
    OR (OLD.state='cancel_requested' AND NEW.state IN ('cancel_requested','cancelled'))
    OR (OLD.state='finalizing' AND NEW.state IN ('retrying','completed','failed','invalidated'))
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid IDO reconciliation job state transition';
  END IF;
  IF NEW.state='completed' AND NOT EXISTS (
    SELECT 1 FROM ido_reconciliation_manifest m WHERE m.run_id=NEW.run_id AND m.outcome=NEW.outcome
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='completed reconciliation job requires matching manifest';
  END IF;
END$$
CREATE TRIGGER trg_ido_reconciliation_job_validate_insert BEFORE INSERT ON ido_reconciliation_job FOR EACH ROW
BEGIN
  IF NEW.state<>'queued' OR NEW.outcome IS NOT NULL OR NEW.active_slot<>1 OR NEW.attempt_count<>0
     OR NEW.consecutive_error_count<>0 OR NEW.next_wallet_ordinal<>0 OR NEW.lease_token<>''
     OR NEW.lease_expires_at IS NOT NULL OR NEW.cancel_requested_by_admin_id IS NOT NULL
     OR NEW.cancel_reason IS NOT NULL OR NEW.cancel_requested_at IS NOT NULL
     OR NEW.last_error_code<>'' OR NEW.last_error_message<>''
     OR NEW.started_at IS NOT NULL OR NEW.completed_at IS NOT NULL
     OR NOT EXISTS (SELECT 1 FROM ido_reconciliation_run r WHERE r.id=NEW.run_id AND r.project_id=NEW.project_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation job must start in canonical queued state';
  END IF;
END$$
CREATE TRIGGER trg_ido_reconciliation_job_no_delete BEFORE DELETE ON ido_reconciliation_job FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation job rows cannot be deleted'; END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS jc_assert_ido_reconciliation_evidence;
DELIMITER $$
CREATE PROCEDURE jc_assert_ido_reconciliation_evidence()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
    'ido_reconciliation_run','ido_reconciliation_job','ido_reconciliation_event',
    'ido_reconciliation_wallet','ido_reconciliation_check','ido_reconciliation_manifest'
  ) AND TABLE_TYPE='BASE TABLE')<>6 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation evidence tables are incomplete'; END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN (
    'trg_ido_reconciliation_run_validate_insert',
    'trg_ido_reconciliation_run_no_update','trg_ido_reconciliation_run_no_delete',
    'trg_ido_reconciliation_event_validate_insert',
    'trg_ido_reconciliation_event_no_update','trg_ido_reconciliation_event_no_delete',
    'trg_ido_reconciliation_wallet_validate_insert','trg_ido_reconciliation_wallet_no_update','trg_ido_reconciliation_wallet_no_delete',
    'trg_ido_reconciliation_check_validate_insert','trg_ido_reconciliation_check_no_update','trg_ido_reconciliation_check_no_delete',
    'trg_ido_reconciliation_manifest_validate_insert','trg_ido_reconciliation_manifest_no_update','trg_ido_reconciliation_manifest_no_delete',
    'trg_ido_reconciliation_job_validate_insert','trg_ido_reconciliation_job_validate_update','trg_ido_reconciliation_job_no_delete'
  ))<>18 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation evidence triggers are incomplete'; END IF;
  IF (SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('ido_reconciliation_job','ido_reconciliation_event','ido_reconciliation_wallet','ido_reconciliation_check','ido_reconciliation_manifest')
      AND DELETE_RULE='RESTRICT')<>5 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation evidence foreign keys are incomplete'; END IF;
  IF (SELECT COUNT(DISTINCT CONCAT(TABLE_NAME,':',INDEX_NAME)) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('ido_reconciliation_run','ido_reconciliation_job','ido_reconciliation_event',
        'ido_reconciliation_wallet','ido_reconciliation_check','ido_reconciliation_manifest')
      AND INDEX_NAME IN ('PRIMARY','uk_ido_reconciliation_run_uuid','uk_ido_reconciliation_request_idempotency','idx_ido_reconciliation_project_time',
      'uk_ido_reconciliation_active_project','idx_ido_reconciliation_job_claim','idx_ido_reconciliation_job_lease',
      'uk_ido_reconciliation_event_sequence','uk_ido_reconciliation_event_hash','idx_ido_reconciliation_event_type',
      'uk_ido_reconciliation_wallet_ordinal','uk_ido_reconciliation_wallet_address','idx_ido_reconciliation_wallet_lookup',
      'uk_ido_reconciliation_check_ordinal','uk_ido_reconciliation_check_code','uk_ido_reconciliation_manifest_digest',
      'uk_ido_reconciliation_manifest_parent','idx_ido_reconciliation_manifest_project'))<>23 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation evidence indexes are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES'
      AND CONSTRAINT_NAME LIKE 'chk_ido_reconciliation_%')<>16 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation evidence checks are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND (
      (TABLE_NAME='ido_reconciliation_run' AND COLUMN_NAME IN ('expected_check_count','expected_wallet_identity_digest','expected_check_identity_digest'))
      OR (TABLE_NAME='ido_reconciliation_wallet' AND COLUMN_NAME IN ('previous_row_digest','identity_digest'))
      OR (TABLE_NAME='ido_reconciliation_check' AND COLUMN_NAME IN ('previous_row_digest','identity_digest'))
      OR (TABLE_NAME='ido_reconciliation_manifest' AND COLUMN_NAME IN ('wallet_evidence_digest','check_evidence_digest'))
  ))<>9 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation evidence root columns are incomplete'; END IF;
END$$
DELIMITER ;
CALL jc_assert_ido_reconciliation_evidence();
DROP PROCEDURE jc_assert_ido_reconciliation_evidence;

-- Manual postflight evidence:
-- SHOW CREATE TABLE ido_reconciliation_run;
-- SHOW CREATE TABLE ido_reconciliation_job;
-- SHOW CREATE TABLE ido_reconciliation_manifest;
-- SELECT TRIGGER_NAME,ACTION_STATEMENT FROM information_schema.TRIGGERS
-- WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME LIKE 'trg_ido_reconciliation_%';
