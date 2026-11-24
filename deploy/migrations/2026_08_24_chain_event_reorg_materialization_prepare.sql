-- JoinCare IDO Sprint 1A-P3-B2-A · append-only semantic effect preparation.
-- This migration does not change current project, ledger, entitlement, outbox,
-- cursor, checkpoint, or incident state. A later audited generation switch
-- consumes only a completed manifest produced by this evidence layer.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_event_reorg_materialization;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_event_reorg_materialization()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
      'chain_event_sync_state','chain_event_record','chain_event_reorg_incident',
      'chain_event_reorg_recovery_run','chain_event_reorg_recovery_job',
      'chain_event_reorg_recovery_log','chain_event_reorg_recovery_manifest'))<>7 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization prerequisites are missing';
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
      'chain_event_reorg_materialization_run','chain_event_reorg_materialization_old_source',
      'chain_event_reorg_materialization_job',
      'chain_event_reorg_materialization_effect','chain_event_reorg_materialization_branch',
      'chain_event_reorg_materialization_event','chain_event_reorg_materialization_manifest')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='pre-existing reorg materialization tables require manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_event_reorg_materialization();
DROP PROCEDURE jc_preflight_chain_event_reorg_materialization;

CREATE TABLE chain_event_reorg_materialization_run (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_uuid CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  recovery_run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  algorithm_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  recovery_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  legacy_contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  boundary_block BIGINT UNSIGNED NOT NULL,
  boundary_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  old_cursor_block BIGINT UNSIGNED NOT NULL,
  old_cursor_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  target_safe_block BIGINT UNSIGNED NOT NULL,
  target_safe_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  expected_old_record_count BIGINT UNSIGNED NOT NULL,
  old_record_source_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  expected_replacement_log_count BIGINT UNSIGNED NOT NULL,
  replacement_log_source_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_materialization_uuid (run_uuid),
  UNIQUE KEY uk_chain_event_reorg_materialization_recovery (recovery_run_id),
  UNIQUE KEY uk_chain_event_reorg_materialization_incident (incident_id),
  CONSTRAINT fk_chain_event_reorg_materialization_recovery FOREIGN KEY (recovery_run_id)
    REFERENCES chain_event_reorg_recovery_run(id) ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_materialization_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_materialization_run_identity CHECK (
    recovery_run_id>0 AND incident_id>0 AND chain_id>0
    AND run_uuid REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
    AND algorithm_version='joincare-reorg-materialization-prepare-v1'
    AND contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND legacy_contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND contract_address<>legacy_contract_address
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_materialization_run_range CHECK (
    boundary_block>0 AND old_cursor_block>boundary_block AND target_safe_block>=old_cursor_block
    AND boundary_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND old_cursor_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND target_safe_block_hash REGEXP '^0x[0-9a-f]{64}$'
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_materialization_run_sources CHECK (
    recovery_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND old_record_source_digest REGEXP '^[0-9a-f]{64}$'
    AND replacement_log_source_digest REGEXP '^[0-9a-f]{64}$'
    AND expected_old_record_count<=50000 AND expected_replacement_log_count<=50000
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_materialization_old_source (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  chain_event_record_id BIGINT UNSIGNED NOT NULL,
  event VARCHAR(50) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  block_number BIGINT UNSIGNED NOT NULL,
  block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  log_index INT UNSIGNED NOT NULL,
  contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_data MEDIUMBLOB NOT NULL,
  event_data_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_materialization_old_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_materialization_old_record (run_id,chain_event_record_id),
  UNIQUE KEY uk_chain_event_reorg_materialization_old_digest (run_id,row_digest),
  KEY idx_chain_event_reorg_materialization_old_block (run_id,block_number,chain_event_record_id),
  CONSTRAINT fk_chain_event_reorg_materialization_old_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_materialization_old_record FOREIGN KEY (chain_event_record_id)
    REFERENCES chain_event_record(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_materialization_old_source CHECK (
    ordinal>0 AND chain_event_record_id>0 AND block_number>0
    AND block_hash REGEXP '^0x[0-9a-f]{64}$' AND tx_hash REGEXP '^0x[0-9a-f]{64}$'
    AND contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND contract_version IN ('v1_legacy','v2_stablecoin')
    AND OCTET_LENGTH(event_data) BETWEEN 2 AND 1048576
    AND event_data_digest REGEXP '^[0-9a-f]{64}$'
    AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_materialization_job (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  state VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  active_slot TINYINT UNSIGNED NULL,
  next_branch VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  next_old_block BIGINT UNSIGNED NOT NULL,
  next_old_record_id BIGINT UNSIGNED NOT NULL,
  next_replacement_ordinal BIGINT UNSIGNED NOT NULL,
  next_effect_ordinal BIGINT UNSIGNED NOT NULL,
  next_branch_ordinal BIGINT UNSIGNED NOT NULL,
  old_effect_count BIGINT UNSIGNED NOT NULL,
  replacement_effect_count BIGINT UNSIGNED NOT NULL,
  ignored_log_count BIGINT UNSIGNED NOT NULL,
  old_effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  replacement_effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  lease_token VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  lease_expires_at DATETIME(6) NULL,
  fence_epoch BIGINT UNSIGNED NOT NULL DEFAULT 0,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  last_error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  last_error_message VARCHAR(500) NOT NULL DEFAULT '',
  completed_at DATETIME(6) NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_materialization_active (incident_id,active_slot),
  KEY idx_chain_event_reorg_materialization_job_state (state,updated_at,run_id),
  CONSTRAINT fk_chain_event_reorg_materialization_job_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_materialization_job_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_materialization_job_state CHECK (
    state IN ('preparing','completed','invalidated')
    AND ((state='preparing' AND active_slot=1 AND completed_at IS NULL)
      OR (state<>'preparing' AND active_slot IS NULL AND completed_at IS NOT NULL))
    AND next_branch IN ('old_committed','replacement','done')
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_materialization_job_checkpoint CHECK (
    next_old_block>0 AND next_replacement_ordinal>0 AND next_effect_ordinal>0 AND next_branch_ordinal>0
    AND ignored_log_count<=replacement_effect_count AND fence_epoch>=0
    AND old_effect_digest REGEXP '^[0-9a-f]{64}$'
    AND replacement_effect_digest REGEXP '^[0-9a-f]{64}$'
    AND ((lease_token='' AND lease_expires_at IS NULL)
      OR (lease_token REGEXP '^[0-9a-f]{64}$' AND lease_expires_at IS NOT NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_materialization_effect (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  branch VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  branch_ordinal BIGINT UNSIGNED NOT NULL,
  source_table VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  block_number BIGINT UNSIGNED NOT NULL,
  block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  log_index INT UNSIGNED NOT NULL,
  contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_type VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  classification VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  semantic_payload MEDIUMBLOB NOT NULL,
  semantic_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_materialization_effect_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_materialization_effect_branch (run_id,branch,branch_ordinal),
  UNIQUE KEY uk_chain_event_reorg_materialization_effect_source (run_id,branch,source_table,source_id),
  UNIQUE KEY uk_chain_event_reorg_materialization_effect_digest (run_id,effect_digest),
  KEY idx_chain_event_reorg_materialization_effect_project (run_id,branch,project_id,ordinal),
  CONSTRAINT fk_chain_event_reorg_materialization_effect_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_materialization_effect_identity CHECK (
    ordinal>0 AND branch_ordinal>0 AND source_id>0 AND block_number>0
    AND branch IN ('old_committed','replacement')
    AND ((branch='old_committed' AND source_table='chain_event_reorg_materialization_old_source')
      OR (branch='replacement' AND source_table='chain_event_reorg_recovery_log'))
    AND block_hash REGEXP '^0x[0-9a-f]{64}$' AND tx_hash REGEXP '^0x[0-9a-f]{64}$'
    AND contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND contract_version IN ('v1_legacy','v2_stablecoin')
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_materialization_effect_semantics CHECK (
    classification IN ('semantic_event','ignored_abi_log')
    AND ((classification='semantic_event' AND event_type<>'ignored'
        AND (project_id>0
          OR (project_id=0 AND contract_version='v1_legacy' AND event_type='PlatformFeesUpdated')
          OR (project_id=0 AND contract_version='v2_stablecoin'
            AND event_type IN ('PaymentTokenConfigured','ProjectTokenConfigured','PlatformFeesUpdated'))))
      OR (classification='ignored_abi_log' AND branch='replacement' AND project_id=0 AND event_type='ignored'))
    AND OCTET_LENGTH(semantic_payload) BETWEEN 2 AND 1048576
    AND semantic_digest REGEXP '^[0-9a-f]{64}$'
    AND previous_effect_digest REGEXP '^[0-9a-f]{64}$' AND effect_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_materialization_branch (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  branch VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  effect_count BIGINT UNSIGNED NOT NULL,
  ignored_count BIGINT UNSIGNED NOT NULL,
  effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  summary_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  completed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_materialization_branch (run_id,branch),
  UNIQUE KEY uk_chain_event_reorg_materialization_branch_digest (summary_digest),
  CONSTRAINT fk_chain_event_reorg_materialization_branch_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_materialization_branch_fields CHECK (
    branch IN ('old_committed','replacement') AND ignored_count<=effect_count
    AND effect_digest REGEXP '^[0-9a-f]{64}$' AND summary_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_materialization_event (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  sequence_no INT UNSIGNED NOT NULL,
  event_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  stage VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  message VARCHAR(500) NOT NULL,
  previous_event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_materialization_event_sequence (run_id,sequence_no),
  UNIQUE KEY uk_chain_event_reorg_materialization_event_hash (event_hash),
  CONSTRAINT fk_chain_event_reorg_materialization_event_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_materialization_event_fields CHECK (
    sequence_no>0 AND event_type IN ('CREATED','BRANCH_COMPLETED','COMPLETED','INVALIDATED')
    AND stage IN ('old_branch','old_committed','replacement','finalize','prepare')
    AND ((event_type='CREATED' AND sequence_no=1 AND stage='old_branch' AND error_code='')
      OR (event_type='BRANCH_COMPLETED' AND sequence_no>1 AND stage IN ('old_committed','replacement') AND error_code='')
      OR (event_type='COMPLETED' AND sequence_no>1 AND stage='finalize' AND error_code='')
      OR (event_type='INVALIDATED' AND sequence_no>1 AND stage='prepare' AND CHAR_LENGTH(error_code)>0))
    AND CHAR_LENGTH(TRIM(message)) BETWEEN 3 AND 500
    AND previous_event_hash REGEXP '^[0-9a-f]{64}$' AND event_hash REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_materialization_manifest (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  recovery_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  old_effect_count BIGINT UNSIGNED NOT NULL,
  replacement_effect_count BIGINT UNSIGNED NOT NULL,
  ignored_log_count BIGINT UNSIGNED NOT NULL,
  old_effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  replacement_effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  completed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_materialization_manifest_digest (manifest_digest),
  CONSTRAINT fk_chain_event_reorg_materialization_manifest_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_materialization_manifest_fields CHECK (
    incident_id>0 AND ignored_log_count<=replacement_effect_count
    AND recovery_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND old_effect_digest REGEXP '^[0-9a-f]{64}$'
    AND replacement_effect_digest REGEXP '^[0-9a-f]{64}$'
    AND manifest_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_run_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_run_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_run_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_old_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_old_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_old_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_job_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_job_validate_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_job_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_effect_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_effect_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_effect_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_branch_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_branch_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_branch_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_event_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_event_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_event_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_manifest_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_manifest_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_manifest_no_delete;
DELIMITER $$
CREATE TRIGGER trg_chain_event_reorg_materialization_run_validate_insert BEFORE INSERT ON chain_event_reorg_materialization_run FOR EACH ROW
BEGIN
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization run id must be database assigned'; END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_incident i WHERE i.id=NEW.incident_id AND i.status='detected' AND i.active_slot=1
      AND i.chain_id=NEW.chain_id AND i.cursor_block=NEW.old_cursor_block AND i.expected_block_hash=NEW.old_cursor_block_hash) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization run requires matching active incident';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_run r
      JOIN chain_event_reorg_recovery_job j ON j.run_id=r.id AND j.state='completed'
      JOIN chain_event_reorg_recovery_manifest m ON m.run_id=r.id
      WHERE r.id=NEW.recovery_run_id AND r.incident_id=NEW.incident_id AND r.chain_id=NEW.chain_id
      AND r.contract_address=NEW.contract_address AND r.legacy_contract_address=NEW.legacy_contract_address
      AND r.boundary_block=NEW.boundary_block AND r.boundary_block_hash=NEW.boundary_block_hash
      AND r.target_safe_block=NEW.target_safe_block AND r.target_safe_block_hash=NEW.target_safe_block_hash
      AND m.manifest_digest=NEW.recovery_manifest_digest AND m.log_count=NEW.expected_replacement_log_count
      AND m.log_evidence_digest=NEW.replacement_log_source_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization run recovery manifest identity mismatch';
  END IF;
  IF NEW.expected_old_record_count<>(SELECT COUNT(*) FROM chain_event_record e WHERE e.block_number>NEW.boundary_block
      AND e.block_number<=NEW.old_cursor_block AND ((e.contract_version='v1_legacy'
        AND LOWER(e.contract_address)=NEW.legacy_contract_address) OR (e.contract_version='v2_stablecoin'
        AND LOWER(e.contract_address)=NEW.contract_address))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization old source count mismatch';
  END IF;
  IF (SELECT COALESCE(SUM(OCTET_LENGTH(e.data)),0) FROM chain_event_record e WHERE e.block_number>NEW.boundary_block
      AND e.block_number<=NEW.old_cursor_block AND ((e.contract_version='v1_legacy'
        AND LOWER(e.contract_address)=NEW.legacy_contract_address) OR (e.contract_version='v2_stablecoin'
        AND LOWER(e.contract_address)=NEW.contract_address)))>12582912
     OR (SELECT COALESCE(SUM(OCTET_LENGTH(l.raw_log)),0) FROM chain_event_reorg_recovery_log l
        WHERE l.run_id=NEW.recovery_run_id)>12582912 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization source payload limit exceeded';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_run_no_update BEFORE UPDATE ON chain_event_reorg_materialization_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization runs are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_run_no_delete BEFORE DELETE ON chain_event_reorg_materialization_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization runs are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_materialization_old_validate_insert BEFORE INSERT ON chain_event_reorg_materialization_old_source FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64);
  DECLARE expected_ordinal BIGINT UNSIGNED;
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='old source id must be database assigned'; END IF;
  SET expected_previous=COALESCE((SELECT row_digest FROM chain_event_reorg_materialization_old_source
    WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET expected_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_materialization_old_source
    WHERE run_id=NEW.run_id),1);
  IF NEW.ordinal<>expected_ordinal OR NEW.previous_row_digest<>expected_previous THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='old source snapshot chain is not contiguous';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_run mr
      JOIN chain_event_reorg_incident i ON i.id=mr.incident_id AND i.status='detected' AND i.active_slot=1
      JOIN chain_event_record r ON r.id=NEW.chain_event_record_id
      WHERE mr.id=NEW.run_id AND r.block_number>mr.boundary_block AND r.block_number<=mr.old_cursor_block
      AND ((r.contract_version='v1_legacy' AND LOWER(r.contract_address)=mr.legacy_contract_address)
        OR (r.contract_version='v2_stablecoin' AND LOWER(r.contract_address)=mr.contract_address))
      AND r.event=NEW.event AND r.block_number=NEW.block_number AND LOWER(r.block_hash)=NEW.block_hash
      AND LOWER(r.tx_hash)=NEW.tx_hash AND r.log_index=NEW.log_index
      AND LOWER(r.contract_address)=NEW.contract_address AND r.contract_version=NEW.contract_version
      AND SHA2(r.data,256)=NEW.event_data_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='old source snapshot parent identity mismatch';
  END IF;
  IF NEW.event_data_digest<>LOWER(SHA2(NEW.event_data,256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='old source snapshot payload digest mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-old-record-source.v1',CHAR(10),
      NEW.ordinal,CHAR(10),NEW.chain_event_record_id,CHAR(10),NEW.event,CHAR(10),NEW.block_number,CHAR(10),
      NEW.block_hash,CHAR(10),NEW.tx_hash,CHAR(10),NEW.log_index,CHAR(10),NEW.contract_address,CHAR(10),
      NEW.contract_version,CHAR(10),NEW.event_data_digest,CHAR(10),NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='old source snapshot row digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_old_no_update BEFORE UPDATE ON chain_event_reorg_materialization_old_source FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization old source is append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_old_no_delete BEFORE DELETE ON chain_event_reorg_materialization_old_source FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization old source is append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_materialization_job_validate_insert BEFORE INSERT ON chain_event_reorg_materialization_job FOR EACH ROW
BEGIN
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_run r WHERE r.id=NEW.run_id AND r.incident_id=NEW.incident_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization job incident does not match parent run';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_run r WHERE r.id=NEW.run_id
      AND r.expected_old_record_count=(SELECT COUNT(*) FROM chain_event_reorg_materialization_old_source s WHERE s.run_id=NEW.run_id)
      AND r.old_record_source_digest=COALESCE((SELECT s.row_digest FROM chain_event_reorg_materialization_old_source s
        WHERE s.run_id=NEW.run_id ORDER BY s.ordinal DESC LIMIT 1),REPEAT('0',64))
      AND (SELECT COALESCE(SUM(OCTET_LENGTH(s.event_data)),0) FROM chain_event_reorg_materialization_old_source s
        WHERE s.run_id=NEW.run_id)<=12582912) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization old source snapshot is incomplete';
  END IF;
  IF NEW.state<>'preparing' OR NEW.active_slot<>1 OR NEW.next_branch<>'old_committed'
     OR NEW.next_old_record_id<>0 OR NEW.next_replacement_ordinal<>1 OR NEW.next_effect_ordinal<>1
     OR NEW.next_branch_ordinal<>1 OR NEW.old_effect_count<>0 OR NEW.replacement_effect_count<>0
     OR NEW.ignored_log_count<>0 OR NEW.old_effect_digest<>REPEAT('0',64)
     OR NEW.replacement_effect_digest<>REPEAT('0',64) OR NEW.lease_token<>'' OR NEW.lease_expires_at IS NOT NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization job initial checkpoint is not canonical';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_job_validate_update BEFORE UPDATE ON chain_event_reorg_materialization_job FOR EACH ROW
BEGIN
  IF NEW.state='preparing' AND EXISTS (SELECT 1 FROM chain_event_reorg_materialization_event e
      WHERE e.run_id=NEW.run_id AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization terminal event forbids further job preparation';
  END IF;
  IF NEW.run_id<>OLD.run_id OR NEW.incident_id<>OLD.incident_id OR NEW.fence_epoch<OLD.fence_epoch
     OR NEW.next_effect_ordinal<OLD.next_effect_ordinal OR NEW.old_effect_count<OLD.old_effect_count
     OR NEW.replacement_effect_count<OLD.replacement_effect_count OR NEW.ignored_log_count<OLD.ignored_log_count
     OR (OLD.state<>'preparing' AND (NEW.state<>OLD.state OR NEW.active_slot<>OLD.active_slot)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization job transition is not monotonic';
  END IF;
  IF NEW.state='preparing' AND NOT (
      (NEW.fence_epoch=OLD.fence_epoch AND OLD.lease_token REGEXP '^[0-9a-f]{64}$'
        AND OLD.lease_expires_at>CURRENT_TIMESTAMP(6) AND (NEW.lease_token=OLD.lease_token OR NEW.lease_token=''))
      OR (NEW.fence_epoch=OLD.fence_epoch+1 AND (OLD.lease_token='' OR OLD.lease_expires_at<=CURRENT_TIMESTAMP(6))
        AND NEW.lease_token REGEXP '^[0-9a-f]{64}$' AND NEW.lease_expires_at>CURRENT_TIMESTAMP(6)
        AND NEW.state=OLD.state AND NEW.active_slot=OLD.active_slot AND NEW.next_branch=OLD.next_branch
        AND NEW.next_old_block=OLD.next_old_block AND NEW.next_old_record_id=OLD.next_old_record_id
        AND NEW.next_replacement_ordinal=OLD.next_replacement_ordinal
        AND NEW.next_effect_ordinal=OLD.next_effect_ordinal AND NEW.next_branch_ordinal=OLD.next_branch_ordinal
        AND NEW.old_effect_count=OLD.old_effect_count AND NEW.replacement_effect_count=OLD.replacement_effect_count
        AND NEW.ignored_log_count=OLD.ignored_log_count AND NEW.old_effect_digest=OLD.old_effect_digest
        AND NEW.replacement_effect_digest=OLD.replacement_effect_digest AND NEW.attempt_count=OLD.attempt_count+1)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization lease or checkpoint transition rejected';
  END IF;
  IF NEW.state='preparing' AND NOT (
      NEW.next_branch=OLD.next_branch
      OR (OLD.next_branch='old_committed' AND NEW.next_branch='replacement'
        AND EXISTS (SELECT 1 FROM chain_event_reorg_materialization_branch b
          WHERE b.run_id=NEW.run_id AND b.branch='old_committed')
        AND (SELECT COUNT(*) FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
          AND e.event_type='BRANCH_COMPLETED' AND e.stage='old_committed')=1)
      OR (OLD.next_branch='replacement' AND NEW.next_branch='done'
        AND (SELECT COUNT(*) FROM chain_event_reorg_materialization_branch b WHERE b.run_id=NEW.run_id)=2
        AND (SELECT COUNT(*) FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
          AND e.event_type='BRANCH_COMPLETED' AND e.stage='replacement')=1)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization branch transition is not contiguous';
  END IF;
  IF NEW.state IN ('completed','invalidated') AND NOT (OLD.state='preparing' AND OLD.lease_token REGEXP '^[0-9a-f]{64}$'
      AND OLD.lease_expires_at>CURRENT_TIMESTAMP(6) AND NEW.active_slot IS NULL AND NEW.lease_token='' AND NEW.lease_expires_at IS NULL
      AND NEW.next_branch=OLD.next_branch AND NEW.next_old_block=OLD.next_old_block
      AND NEW.next_old_record_id=OLD.next_old_record_id AND NEW.next_replacement_ordinal=OLD.next_replacement_ordinal
      AND NEW.next_effect_ordinal=OLD.next_effect_ordinal AND NEW.next_branch_ordinal=OLD.next_branch_ordinal
      AND NEW.old_effect_count=OLD.old_effect_count AND NEW.replacement_effect_count=OLD.replacement_effect_count
      AND NEW.ignored_log_count=OLD.ignored_log_count AND NEW.old_effect_digest=OLD.old_effect_digest
      AND NEW.replacement_effect_digest=OLD.replacement_effect_digest AND NEW.fence_epoch=OLD.fence_epoch) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization terminal transition requires active DB lease';
  END IF;
  IF NEW.state='completed' AND (NEW.next_branch<>'done'
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_manifest m WHERE m.run_id=NEW.run_id
        AND m.incident_id=NEW.incident_id AND m.old_effect_count=NEW.old_effect_count
        AND m.replacement_effect_count=NEW.replacement_effect_count AND m.ignored_log_count=NEW.ignored_log_count
        AND m.old_effect_digest=NEW.old_effect_digest AND m.replacement_effect_digest=NEW.replacement_effect_digest)
      OR (SELECT COUNT(*) FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
        AND e.event_type IN ('COMPLETED','INVALIDATED'))<>1
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
        AND e.event_type='COMPLETED' AND e.sequence_no=(SELECT MAX(sequence_no) FROM chain_event_reorg_materialization_event WHERE run_id=NEW.run_id))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization completed transition requires matching manifest and terminal event';
  END IF;
  IF NEW.state='invalidated' AND (CHAR_LENGTH(NEW.last_error_code)=0
      OR (SELECT COUNT(*) FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
        AND e.event_type IN ('COMPLETED','INVALIDATED'))<>1
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
        AND e.event_type='INVALIDATED' AND e.error_code=NEW.last_error_code
        AND e.sequence_no=(SELECT MAX(sequence_no) FROM chain_event_reorg_materialization_event WHERE run_id=NEW.run_id))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization invalidated transition requires matching terminal event';
  END IF;
  IF NEW.state='preparing' AND NEW.next_branch='done'
      AND (SELECT COUNT(*) FROM chain_event_reorg_materialization_branch WHERE run_id=NEW.run_id)<>2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization done checkpoint requires both branch summaries';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_job_no_delete BEFORE DELETE ON chain_event_reorg_materialization_job FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization jobs cannot be deleted'; END$$

CREATE TRIGGER trg_chain_event_reorg_materialization_effect_validate_insert BEFORE INSERT ON chain_event_reorg_materialization_effect FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64);
  DECLARE expected_global BIGINT UNSIGNED;
  DECLARE expected_branch BIGINT UNSIGNED;
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization effect id must be database assigned'; END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization terminal event forbids further effects';
  END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_materialization_branch b
      WHERE b.run_id=NEW.run_id AND b.branch=NEW.branch) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization branch summary forbids further effects';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_job WHERE run_id=NEW.run_id
      AND state='preparing' AND active_slot=1 AND next_branch=NEW.branch
      AND lease_token REGEXP '^[0-9a-f]{64}$' AND lease_expires_at>CURRENT_TIMESTAMP(6)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization effect requires active leased branch';
  END IF;
  SET expected_global=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_materialization_effect WHERE run_id=NEW.run_id),
    (SELECT next_effect_ordinal FROM chain_event_reorg_materialization_job WHERE run_id=NEW.run_id));
  SET expected_branch=COALESCE((SELECT MAX(branch_ordinal)+1 FROM chain_event_reorg_materialization_effect WHERE run_id=NEW.run_id AND branch=NEW.branch),
    (SELECT next_branch_ordinal FROM chain_event_reorg_materialization_job WHERE run_id=NEW.run_id));
  SET expected_previous=COALESCE((SELECT effect_digest FROM chain_event_reorg_materialization_effect
      WHERE run_id=NEW.run_id AND branch=NEW.branch ORDER BY branch_ordinal DESC LIMIT 1),
    (SELECT CASE NEW.branch WHEN 'old_committed' THEN old_effect_digest ELSE replacement_effect_digest END
      FROM chain_event_reorg_materialization_job WHERE run_id=NEW.run_id));
  IF expected_global IS NULL OR NEW.ordinal<>expected_global OR NEW.branch_ordinal<>expected_branch
     OR NEW.previous_effect_digest<>expected_previous THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization effect chain is not contiguous or leased';
  END IF;
  IF NEW.branch='old_committed' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_old_source r
      WHERE r.id=NEW.source_id AND r.run_id=NEW.run_id AND r.block_number=NEW.block_number
      AND r.block_hash=NEW.block_hash AND r.tx_hash=NEW.tx_hash AND r.log_index=NEW.log_index
      AND r.contract_address=NEW.contract_address AND r.contract_version=NEW.contract_version
      AND r.event=NEW.event_type AND r.event_data_digest=NEW.semantic_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='old materialization effect source identity mismatch';
  END IF;
  IF NEW.branch='replacement' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_log l
      JOIN chain_event_reorg_materialization_run r ON r.id=NEW.run_id AND r.recovery_run_id=l.run_id
      WHERE l.id=NEW.source_id AND l.block_number=NEW.block_number AND l.block_hash=NEW.block_hash
      AND l.tx_hash=NEW.tx_hash AND l.log_index=NEW.log_index AND l.contract_address=NEW.contract_address
      AND l.contract_version=NEW.contract_version) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='replacement materialization effect source identity mismatch';
  END IF;
  IF NEW.semantic_digest<>LOWER(SHA2(NEW.semantic_payload,256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization semantic payload digest mismatch';
  END IF;
  IF NEW.effect_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-materialization-effect.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.ordinal,CHAR(10),NEW.branch,CHAR(10),NEW.branch_ordinal,CHAR(10),
      NEW.source_table,CHAR(10),NEW.source_id,CHAR(10),NEW.block_number,CHAR(10),NEW.block_hash,CHAR(10),
      NEW.tx_hash,CHAR(10),NEW.log_index,CHAR(10),NEW.contract_address,CHAR(10),NEW.contract_version,CHAR(10),
      NEW.event_type,CHAR(10),NEW.project_id,CHAR(10),NEW.classification,CHAR(10),NEW.semantic_digest,CHAR(10),
      NEW.previous_effect_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization effect digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_effect_no_update BEFORE UPDATE ON chain_event_reorg_materialization_effect FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization effects are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_effect_no_delete BEFORE DELETE ON chain_event_reorg_materialization_effect FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization effects are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_materialization_branch_validate_insert BEFORE INSERT ON chain_event_reorg_materialization_branch FOR EACH ROW
BEGIN
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization branch id must be database assigned'; END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization terminal event forbids further branch summaries';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.active_slot=1 AND j.next_branch=NEW.branch
      AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
      AND ((NEW.branch='old_committed' AND j.old_effect_count=NEW.effect_count AND j.old_effect_digest=NEW.effect_digest)
        OR (NEW.branch='replacement' AND j.replacement_effect_count=NEW.effect_count
          AND j.replacement_effect_digest=NEW.effect_digest AND j.ignored_log_count=NEW.ignored_count))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization branch summary requires complete leased checkpoint';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_run r WHERE r.id=NEW.run_id
      AND ((NEW.branch='old_committed' AND r.expected_old_record_count=NEW.effect_count AND NEW.ignored_count=0)
        OR (NEW.branch='replacement' AND r.expected_replacement_log_count=NEW.effect_count))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization branch count does not match frozen source';
  END IF;
  IF (SELECT COUNT(*) FROM chain_event_reorg_materialization_effect e WHERE e.run_id=NEW.run_id AND e.branch=NEW.branch)<>NEW.effect_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_materialization_effect e WHERE e.run_id=NEW.run_id
       AND e.branch=NEW.branch AND e.classification='ignored_abi_log')<>NEW.ignored_count
     OR COALESCE((SELECT e.effect_digest FROM chain_event_reorg_materialization_effect e
       WHERE e.run_id=NEW.run_id AND e.branch=NEW.branch ORDER BY e.branch_ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.effect_digest THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization branch effect set is incomplete';
  END IF;
  IF NEW.summary_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-materialization-branch.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.branch,CHAR(10),NEW.effect_count,CHAR(10),NEW.ignored_count,CHAR(10),NEW.effect_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization branch summary digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_branch_no_update BEFORE UPDATE ON chain_event_reorg_materialization_branch FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization branch summaries are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_branch_no_delete BEFORE DELETE ON chain_event_reorg_materialization_branch FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization branch summaries are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_materialization_event_validate_insert BEFORE INSERT ON chain_event_reorg_materialization_event FOR EACH ROW
BEGIN
  DECLARE prior_hash VARCHAR(64);
  DECLARE prior_sequence INT UNSIGNED;
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization event id must be database assigned'; END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization terminal event is final';
  END IF;
  SET prior_hash=COALESCE((SELECT event_hash FROM chain_event_reorg_materialization_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),REPEAT('0',64));
  SET prior_sequence=COALESCE((SELECT sequence_no FROM chain_event_reorg_materialization_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),0);
  IF NEW.sequence_no<>prior_sequence+1 OR NEW.previous_event_hash<>prior_hash THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization event chain is not contiguous';
  END IF;
  IF NEW.event_type='CREATED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.active_slot=1 AND j.fence_epoch=0 AND j.lease_token='') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization CREATED event requires canonical unleased initial job';
  END IF;
  IF NEW.event_type<>'CREATED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.active_slot=1 AND j.lease_token REGEXP '^[0-9a-f]{64}$'
      AND j.lease_expires_at>CURRENT_TIMESTAMP(6)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization event requires active DB lease';
  END IF;
  IF NEW.event_type='BRANCH_COMPLETED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_job j
      JOIN chain_event_reorg_materialization_branch b ON b.run_id=j.run_id AND b.branch=NEW.stage
      WHERE j.run_id=NEW.run_id AND j.next_branch=NEW.stage AND j.state='preparing' AND j.active_slot=1) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization branch event requires matching summary and checkpoint';
  END IF;
  IF NEW.event_type='BRANCH_COMPLETED' AND EXISTS (SELECT 1 FROM chain_event_reorg_materialization_event e
      WHERE e.run_id=NEW.run_id AND e.event_type='BRANCH_COMPLETED' AND e.stage=NEW.stage) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization branch event is unique per branch';
  END IF;
  IF NEW.event_type='COMPLETED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_job j
      JOIN chain_event_reorg_materialization_manifest m ON m.run_id=j.run_id AND m.incident_id=j.incident_id
      WHERE j.run_id=NEW.run_id AND j.next_branch='done' AND j.state='preparing' AND j.active_slot=1) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization completed event requires matching manifest';
  END IF;
  IF NEW.event_type='INVALIDATED' AND EXISTS (SELECT 1 FROM chain_event_reorg_materialization_manifest m
      WHERE m.run_id=NEW.run_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization manifest forbids invalidation';
  END IF;
  IF NEW.event_hash<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-materialization-event.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.sequence_no,CHAR(10),NEW.event_type,CHAR(10),NEW.stage,CHAR(10),
      NEW.error_code,CHAR(10),NEW.message,CHAR(10),NEW.previous_event_hash),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization event digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_event_no_update BEFORE UPDATE ON chain_event_reorg_materialization_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization events are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_event_no_delete BEFORE DELETE ON chain_event_reorg_materialization_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization events are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_materialization_manifest_validate_insert BEFORE INSERT ON chain_event_reorg_materialization_manifest FOR EACH ROW
BEGIN
  IF EXISTS (SELECT 1 FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization terminal event forbids manifest insertion';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_job j
      JOIN chain_event_reorg_materialization_run r ON r.id=j.run_id
      WHERE j.run_id=NEW.run_id AND j.incident_id=NEW.incident_id AND j.state='preparing' AND j.active_slot=1
      AND r.incident_id=NEW.incident_id
      AND j.next_branch='done' AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
      AND r.recovery_manifest_digest=NEW.recovery_manifest_digest
      AND j.old_effect_count=NEW.old_effect_count AND j.replacement_effect_count=NEW.replacement_effect_count
      AND j.ignored_log_count=NEW.ignored_log_count AND j.old_effect_digest=NEW.old_effect_digest
      AND j.replacement_effect_digest=NEW.replacement_effect_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization manifest requires complete leased job';
  END IF;
  IF (SELECT COUNT(*) FROM chain_event_reorg_materialization_branch WHERE run_id=NEW.run_id)<>2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization manifest branch summaries are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
      AND e.event_type='BRANCH_COMPLETED' AND e.stage IN ('old_committed','replacement'))<>2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization manifest branch audit events are incomplete';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_branch b WHERE b.run_id=NEW.run_id
      AND b.branch='old_committed' AND b.effect_count=NEW.old_effect_count AND b.ignored_count=0
      AND b.effect_digest=NEW.old_effect_digest)
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_branch b WHERE b.run_id=NEW.run_id
      AND b.branch='replacement' AND b.effect_count=NEW.replacement_effect_count
      AND b.ignored_count=NEW.ignored_log_count AND b.effect_digest=NEW.replacement_effect_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization manifest branch roots mismatch';
  END IF;
  IF NEW.manifest_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-materialization-manifest.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.incident_id,CHAR(10),NEW.recovery_manifest_digest,CHAR(10),
      NEW.old_effect_count,CHAR(10),NEW.replacement_effect_count,CHAR(10),NEW.ignored_log_count,CHAR(10),
      NEW.old_effect_digest,CHAR(10),NEW.replacement_effect_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization manifest digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_manifest_no_update BEFORE UPDATE ON chain_event_reorg_materialization_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization manifests are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_manifest_no_delete BEFORE DELETE ON chain_event_reorg_materialization_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization manifests are append-only'; END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS jc_assert_chain_event_reorg_materialization;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_reorg_materialization()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'
      AND TABLE_NAME IN ('chain_event_reorg_materialization_run','chain_event_reorg_materialization_old_source',
      'chain_event_reorg_materialization_job',
      'chain_event_reorg_materialization_effect','chain_event_reorg_materialization_branch',
      'chain_event_reorg_materialization_event','chain_event_reorg_materialization_manifest'))<>7 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization tables are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
      AND TRIGGER_NAME REGEXP '^trg_chain_event_reorg_materialization_')<>21 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization triggers are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES'
      AND CONSTRAINT_NAME REGEXP '^chk_chain_event_reorg_materialization_')<>11 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization checks are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_NAME REGEXP '^fk_chain_event_reorg_materialization_' AND DELETE_RULE='RESTRICT')<>10 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization foreign keys are incomplete';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_reorg_materialization();
DROP PROCEDURE jc_assert_chain_event_reorg_materialization;

-- Deployment runner must replace this candidate checksum with the exact file
-- SHA256 before execution and record status=applied only after postflight.
