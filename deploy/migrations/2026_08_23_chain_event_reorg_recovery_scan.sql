-- JoinCare IDO Sprint 1A-P3-B1 · durable replacement-branch recovery scan.
-- Completion proves only that a fixed safe range was scanned and staged. It
-- does not resolve the incident or mutate canonical financial projections.

SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_event_reorg_recovery;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_event_reorg_recovery()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('chain_event_sync_state','chain_event_block_checkpoint','chain_event_reorg_incident','chain_event_reorg_event'))<>4 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event reorg recovery prerequisites are missing';
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_recovery_job','chain_event_reorg_recovery_event',
      'chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch','chain_event_reorg_recovery_manifest',
      'chain_event_reorg_recovery_definition')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='pre-existing reorg recovery tables require manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_event_reorg_recovery();
DROP PROCEDURE jc_preflight_chain_event_reorg_recovery;

CREATE TABLE chain_event_reorg_recovery_run (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_uuid CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  attempt_no INT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  legacy_contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  algorithm_version VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  boundary_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  boundary_checkpoint_id BIGINT UNSIGNED NULL,
  boundary_block BIGINT UNSIGNED NOT NULL,
  boundary_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  target_safe_block BIGINT UNSIGNED NOT NULL,
  target_safe_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  confirmation_depth BIGINT UNSIGNED NOT NULL,
  expected_batch_count INT UNSIGNED NOT NULL,
  expected_scanned_blocks BIGINT UNSIGNED NOT NULL,
  requested_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_recovery_uuid (run_uuid),
  UNIQUE KEY uk_chain_event_reorg_recovery_attempt (incident_id,attempt_no),
  KEY idx_chain_event_reorg_recovery_incident (incident_id,id),
  KEY idx_chain_event_reorg_recovery_checkpoint (boundary_checkpoint_id),
  CONSTRAINT fk_chain_event_reorg_recovery_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_recovery_checkpoint FOREIGN KEY (boundary_checkpoint_id)
    REFERENCES chain_event_block_checkpoint(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_recovery_run_identity CHECK (
    incident_id>0 AND attempt_no>0 AND chain_id>0
    AND run_uuid REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
    AND contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND contract_address<>'0x0000000000000000000000000000000000000000'
    AND legacy_contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND legacy_contract_address<>'0x0000000000000000000000000000000000000000'
    AND legacy_contract_address<>contract_address
    AND contract_version='v2_stablecoin'
    AND algorithm_version='joincare-reorg-recovery-scan-v1'
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_recovery_boundary CHECK (
    boundary_kind='last_canonical_checkpoint' AND boundary_checkpoint_id IS NOT NULL AND boundary_block>0
      AND boundary_block_hash REGEXP '^0x[0-9a-f]{64}$'
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_recovery_target CHECK (
    target_safe_block>boundary_block AND target_safe_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND expected_scanned_blocks=target_safe_block-boundary_block
    AND expected_batch_count=CEIL(expected_scanned_blocks/100)
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_recovery_job (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  state VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  active_slot TINYINT UNSIGNED NULL,
  next_block BIGINT UNSIGNED NOT NULL,
  next_batch_ordinal INT UNSIGNED NOT NULL,
  next_log_ordinal BIGINT UNSIGNED NOT NULL,
  observed_log_count BIGINT UNSIGNED NOT NULL,
  batch_evidence_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  log_evidence_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  lease_token VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  lease_expires_at DATETIME(6) NULL,
  fence_epoch BIGINT UNSIGNED NOT NULL DEFAULT 0,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  last_error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  last_error_message VARCHAR(500) NOT NULL DEFAULT '',
  completed_at DATETIME(6) NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_recovery_active (incident_id,active_slot),
  KEY idx_chain_event_reorg_recovery_job_state (state,updated_at,run_id),
  CONSTRAINT fk_chain_event_reorg_recovery_job_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_recovery_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_recovery_job_state CHECK (
    state IN ('scanning','completed','invalidated')
    AND ((state='scanning' AND active_slot=1 AND completed_at IS NULL)
      OR (state<>'scanning' AND active_slot IS NULL AND completed_at IS NOT NULL))
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_recovery_job_digests CHECK (
    batch_evidence_digest REGEXP '^[0-9a-f]{64}$' AND log_evidence_digest REGEXP '^[0-9a-f]{64}$'
    AND next_batch_ordinal>0 AND next_log_ordinal>0 AND fence_epoch>=0
    AND ((lease_token='' AND lease_expires_at IS NULL) OR (lease_token REGEXP '^[0-9a-f]{64}$' AND lease_expires_at IS NOT NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_recovery_event (
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
  UNIQUE KEY uk_chain_event_reorg_recovery_event_sequence (run_id,sequence_no),
  UNIQUE KEY uk_chain_event_reorg_recovery_event_hash (event_hash),
  KEY idx_chain_event_reorg_recovery_event_type (run_id,event_type,created_at),
  CONSTRAINT fk_chain_event_reorg_recovery_event_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_recovery_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_recovery_event_fields CHECK (
    sequence_no>0 AND event_type IN ('CREATED','BATCH_COMMITTED','COMPLETED','INVALIDATED')
    AND stage IN ('boundary','scan','finalize') AND CHAR_LENGTH(TRIM(message)) BETWEEN 3 AND 500
    AND previous_event_hash REGEXP '^[0-9a-f]{64}$' AND event_hash REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_recovery_log (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  block_number BIGINT UNSIGNED NOT NULL,
  block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tx_index INT UNSIGNED NOT NULL,
  log_index INT UNSIGNED NOT NULL,
  contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  raw_log MEDIUMBLOB NOT NULL,
  raw_log_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  observed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_recovery_log_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_recovery_log_identity (run_id,block_number,tx_hash,log_index),
  UNIQUE KEY uk_chain_event_reorg_recovery_log_digest (run_id,row_digest),
  KEY idx_chain_event_reorg_recovery_log_block (run_id,block_number,tx_index,log_index),
  CONSTRAINT fk_chain_event_reorg_recovery_log_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_recovery_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_recovery_log_identity CHECK (
    ordinal>0 AND block_number>0 AND block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND tx_hash REGEXP '^0x[0-9a-f]{64}$' AND contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND contract_version IN ('v1_legacy','v2_stablecoin') AND OCTET_LENGTH(raw_log) BETWEEN 2 AND 1048576
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_recovery_log_digests CHECK (
    raw_log_digest REGEXP '^[0-9a-f]{64}$' AND previous_row_digest REGEXP '^[0-9a-f]{64}$'
    AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_recovery_batch (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal INT UNSIGNED NOT NULL,
  from_block BIGINT UNSIGNED NOT NULL,
  to_block BIGINT UNSIGNED NOT NULL,
  from_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  to_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  first_log_ordinal BIGINT UNSIGNED NOT NULL,
  log_count BIGINT UNSIGNED NOT NULL,
  batch_log_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_batch_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  batch_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  observed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_recovery_batch_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_recovery_batch_range (run_id,from_block,to_block),
  UNIQUE KEY uk_chain_event_reorg_recovery_batch_digest (run_id,batch_digest),
  CONSTRAINT fk_chain_event_reorg_recovery_batch_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_recovery_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_recovery_batch_identity CHECK (
    ordinal>0 AND from_block>0 AND to_block>=from_block AND to_block-from_block<100
    AND from_block_hash REGEXP '^0x[0-9a-f]{64}$' AND to_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND first_log_ordinal>0
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_recovery_batch_digests CHECK (
    batch_log_digest REGEXP '^[0-9a-f]{64}$' AND previous_batch_digest REGEXP '^[0-9a-f]{64}$'
    AND batch_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_recovery_manifest (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  batch_count INT UNSIGNED NOT NULL,
  log_count BIGINT UNSIGNED NOT NULL,
  batch_evidence_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  log_evidence_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  completed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_recovery_manifest_digest (manifest_digest),
  KEY idx_chain_event_reorg_recovery_manifest_incident (incident_id,run_id),
  CONSTRAINT fk_chain_event_reorg_recovery_manifest_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_recovery_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_recovery_manifest_fields CHECK (
    incident_id>0 AND batch_count>0 AND batch_evidence_digest REGEXP '^[0-9a-f]{64}$'
    AND log_evidence_digest REGEXP '^[0-9a-f]{64}$' AND manifest_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_recovery_definition (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  object_type VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  object_name VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  definition_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_recovery_definition_object (object_type,object_name),
  UNIQUE KEY uk_chain_event_reorg_recovery_definition_digest (row_digest),
  CONSTRAINT chk_chain_event_reorg_recovery_definition_identity CHECK (
    object_type IN ('CHECK','TRIGGER') AND CHAR_LENGTH(object_name) BETWEEN 3 AND 128
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_recovery_definition_digests CHECK (
    definition_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_run_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_run_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_run_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_job_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_job_validate_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_job_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_event_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_event_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_event_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_log_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_log_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_log_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_batch_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_batch_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_batch_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_manifest_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_manifest_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_manifest_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_definition_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_definition_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_definition_no_delete;
DELIMITER $$
CREATE TRIGGER trg_chain_event_reorg_recovery_run_validate_insert BEFORE INSERT ON chain_event_reorg_recovery_run FOR EACH ROW
BEGIN
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_incident i WHERE i.id=NEW.incident_id AND i.status='detected'
      AND i.active_slot=1 AND i.chain_id=NEW.chain_id AND i.contract_address=NEW.contract_address) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery run requires matching active incident';
  END IF;
  IF NEW.attempt_no<>COALESCE((SELECT MAX(r.attempt_no)+1 FROM chain_event_reorg_recovery_run r WHERE r.incident_id=NEW.incident_id),1) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery attempt is not contiguous';
  END IF;
  IF NEW.boundary_checkpoint_id IS NOT NULL AND NOT EXISTS (
      SELECT 1 FROM chain_event_block_checkpoint c WHERE c.id=NEW.boundary_checkpoint_id
      AND c.chain_id=NEW.chain_id AND c.block_number=NEW.boundary_block AND c.block_hash=NEW.boundary_block_hash) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery boundary checkpoint identity mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_run_no_update BEFORE UPDATE ON chain_event_reorg_recovery_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery runs are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_run_no_delete BEFORE DELETE ON chain_event_reorg_recovery_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery runs are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_recovery_job_validate_insert BEFORE INSERT ON chain_event_reorg_recovery_job FOR EACH ROW
BEGIN
  IF NEW.state<>'scanning' OR NEW.active_slot<>1 OR NEW.next_batch_ordinal<>1 OR NEW.next_log_ordinal<>1
     OR NEW.observed_log_count<>0 OR NEW.batch_evidence_digest<>REPEAT('0',64) OR NEW.log_evidence_digest<>REPEAT('0',64)
     OR NEW.lease_token<>'' OR NEW.lease_expires_at IS NOT NULL OR NEW.fence_epoch<>0
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_run r WHERE r.id=NEW.run_id AND r.incident_id=NEW.incident_id
       AND NEW.next_block=r.boundary_block+1) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery job is not canonical initial state';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_job_validate_update BEFORE UPDATE ON chain_event_reorg_recovery_job FOR EACH ROW
BEGIN
  IF NEW.run_id<>OLD.run_id OR NEW.incident_id<>OLD.incident_id OR NEW.next_block<OLD.next_block
     OR NEW.next_batch_ordinal<OLD.next_batch_ordinal OR NEW.next_log_ordinal<OLD.next_log_ordinal
     OR NEW.observed_log_count<OLD.observed_log_count OR NEW.attempt_count<OLD.attempt_count OR NEW.fence_epoch<OLD.fence_epoch
     OR (OLD.state<>'scanning' AND NEW.state<>OLD.state) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery job transition is not monotonic';
  END IF;
  IF NEW.state='scanning' AND (
      NOT ((NEW.fence_epoch=OLD.fence_epoch AND (NEW.lease_token=OLD.lease_token
          OR (NEW.lease_token='' AND NEW.lease_expires_at IS NULL)))
        OR (NEW.fence_epoch=OLD.fence_epoch+1 AND (OLD.lease_expires_at IS NULL OR OLD.lease_expires_at<=CURRENT_TIMESTAMP(6))
          AND NEW.lease_token REGEXP '^[0-9a-f]{64}$' AND NEW.lease_expires_at>CURRENT_TIMESTAMP(6)))
      OR NEW.attempt_count>OLD.attempt_count+1
      OR (NEW.next_block>OLD.next_block AND (NEW.next_block<>LEAST(OLD.next_block+100,(SELECT target_safe_block+1 FROM chain_event_reorg_recovery_run WHERE id=NEW.run_id))
        OR NEW.next_batch_ordinal<>OLD.next_batch_ordinal+1
        OR NEW.next_log_ordinal<>OLD.next_log_ordinal+(NEW.observed_log_count-OLD.observed_log_count)
        OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_batch b WHERE b.run_id=NEW.run_id
          AND b.ordinal=OLD.next_batch_ordinal AND b.from_block=OLD.next_block AND b.to_block=NEW.next_block-1
          AND b.first_log_ordinal=OLD.next_log_ordinal AND b.log_count=NEW.observed_log_count-OLD.observed_log_count
          AND b.batch_digest=NEW.batch_evidence_digest
          AND ((b.log_count=0 AND NEW.log_evidence_digest=OLD.log_evidence_digest)
            OR (b.log_count>0 AND b.batch_log_digest=NEW.log_evidence_digest)))))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery scanning transition bypassed lease or batch checkpoint';
  END IF;
  IF NEW.state='completed' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_manifest m WHERE m.run_id=NEW.run_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='completed reorg recovery job requires manifest';
  END IF;
  IF NEW.state<>'scanning' AND (
      OLD.state<>'scanning' OR OLD.lease_token NOT REGEXP '^[0-9a-f]{64}$' OR OLD.lease_expires_at<=CURRENT_TIMESTAMP(6)
      OR NEW.active_slot IS NOT NULL OR NEW.lease_token<>'' OR NEW.lease_expires_at IS NOT NULL
      OR NEW.fence_epoch<>OLD.fence_epoch OR NEW.next_block<>OLD.next_block
      OR NEW.next_batch_ordinal<>OLD.next_batch_ordinal OR NEW.next_log_ordinal<>OLD.next_log_ordinal
      OR NEW.observed_log_count<>OLD.observed_log_count OR NEW.batch_evidence_digest<>OLD.batch_evidence_digest
      OR NEW.log_evidence_digest<>OLD.log_evidence_digest OR NEW.completed_at IS NULL
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_event e WHERE e.run_id=NEW.run_id
        AND e.sequence_no=(SELECT MAX(x.sequence_no) FROM chain_event_reorg_recovery_event x WHERE x.run_id=NEW.run_id)
        AND e.event_type=CASE WHEN NEW.state='completed' THEN 'COMPLETED' ELSE 'INVALIDATED' END)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery terminal transition lacks leased matching event';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_job_no_delete BEFORE DELETE ON chain_event_reorg_recovery_job FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery jobs cannot be deleted'; END$$

CREATE TRIGGER trg_chain_event_reorg_recovery_event_validate_insert BEFORE INSERT ON chain_event_reorg_recovery_event FOR EACH ROW
BEGIN
  DECLARE prior_hash VARCHAR(64);
  DECLARE prior_sequence INT UNSIGNED;
  SET prior_hash=COALESCE((SELECT event_hash FROM chain_event_reorg_recovery_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),REPEAT('0',64));
  SET prior_sequence=COALESCE((SELECT sequence_no FROM chain_event_reorg_recovery_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),0);
  IF NEW.sequence_no<>prior_sequence+1 OR NEW.previous_event_hash<>prior_hash THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery event chain is not contiguous';
  END IF;
  IF (NEW.sequence_no=1 AND (NEW.event_type<>'CREATED' OR NEW.stage<>'boundary' OR NEW.error_code<>''))
     OR (NEW.sequence_no>1 AND NEW.event_type='CREATED')
     OR (NEW.event_type='BATCH_COMMITTED' AND (NEW.stage<>'scan' OR NEW.error_code<>''))
     OR (NEW.event_type='COMPLETED' AND (NEW.stage<>'finalize' OR NEW.error_code<>''
       OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_manifest m WHERE m.run_id=NEW.run_id)))
     OR (NEW.event_type='INVALIDATED' AND CHAR_LENGTH(TRIM(NEW.error_code))<3) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery event does not match persistent state';
  END IF;
  IF (NEW.event_type='CREATED' AND NOT EXISTS (
      SELECT 1 FROM chain_event_reorg_recovery_job j WHERE j.run_id=NEW.run_id AND j.state='scanning'
      AND j.active_slot=1 AND j.next_batch_ordinal=1 AND j.next_log_ordinal=1
      AND j.observed_log_count=0 AND j.lease_token='' AND j.lease_expires_at IS NULL))
     OR (NEW.event_type<>'CREATED' AND NOT EXISTS (
      SELECT 1 FROM chain_event_reorg_recovery_job j WHERE j.run_id=NEW.run_id AND j.state='scanning'
      AND j.active_slot=1 AND j.lease_token REGEXP '^[0-9a-f]{64}$'
      AND j.lease_expires_at>CURRENT_TIMESTAMP(6))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery event requires matching active leased job';
  END IF;
  IF NEW.event_hash<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-recovery-event.v1',CHAR(10),NEW.run_id,CHAR(10),
      NEW.sequence_no,CHAR(10),NEW.event_type,CHAR(10),NEW.stage,CHAR(10),NEW.error_code,CHAR(10),NEW.message,CHAR(10),NEW.previous_event_hash),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery event digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_event_no_update BEFORE UPDATE ON chain_event_reorg_recovery_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery events are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_event_no_delete BEFORE DELETE ON chain_event_reorg_recovery_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery events are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_recovery_log_validate_insert BEFORE INSERT ON chain_event_reorg_recovery_log FOR EACH ROW
BEGIN
  DECLARE prior_digest VARCHAR(64);
  DECLARE prior_ordinal BIGINT UNSIGNED;
  DECLARE pending_from BIGINT UNSIGNED;
  DECLARE pending_target BIGINT UNSIGNED;
  DECLARE expected_v1 VARCHAR(42);
  DECLARE expected_v2 VARCHAR(42);
  SET prior_digest=COALESCE((SELECT row_digest FROM chain_event_reorg_recovery_log WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET prior_ordinal=COALESCE((SELECT ordinal FROM chain_event_reorg_recovery_log WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),0);
  SELECT j.next_block,r.target_safe_block,r.legacy_contract_address,r.contract_address
    INTO pending_from,pending_target,expected_v1,expected_v2
    FROM chain_event_reorg_recovery_job j JOIN chain_event_reorg_recovery_run r ON r.id=j.run_id
    WHERE j.run_id=NEW.run_id AND j.state='scanning' AND j.active_slot=1
      AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6);
  IF pending_from IS NULL OR NEW.block_number<pending_from OR NEW.block_number>LEAST(pending_from+99,pending_target)
     OR NOT ((NEW.contract_version='v1_legacy' AND NEW.contract_address=expected_v1)
       OR (NEW.contract_version='v2_stablecoin' AND NEW.contract_address=expected_v2)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery log is outside active leased scan range';
  END IF;
  IF NEW.ordinal<>prior_ordinal+1 OR NEW.previous_row_digest<>prior_digest
     OR NEW.raw_log_digest<>LOWER(SHA2(NEW.raw_log,256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery log chain or payload digest mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-recovery-log.v1',CHAR(10),NEW.run_id,CHAR(10),NEW.ordinal,CHAR(10),
      NEW.block_number,CHAR(10),NEW.block_hash,CHAR(10),NEW.tx_hash,CHAR(10),NEW.tx_index,CHAR(10),NEW.log_index,CHAR(10),
      NEW.contract_address,CHAR(10),NEW.contract_version,CHAR(10),NEW.raw_log_digest,CHAR(10),NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery log row digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_log_no_update BEFORE UPDATE ON chain_event_reorg_recovery_log FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery logs are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_log_no_delete BEFORE DELETE ON chain_event_reorg_recovery_log FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery logs are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_recovery_batch_validate_insert BEFORE INSERT ON chain_event_reorg_recovery_batch FOR EACH ROW
BEGIN
  DECLARE prior_digest VARCHAR(64);
  DECLARE prior_ordinal INT UNSIGNED;
  DECLARE prior_to BIGINT UNSIGNED;
  DECLARE expected_first_log BIGINT UNSIGNED;
  DECLARE run_boundary BIGINT UNSIGNED;
  DECLARE run_target BIGINT UNSIGNED;
  DECLARE exact_logs BIGINT UNSIGNED;
  DECLARE materialized_logs BIGINT UNSIGNED;
  SET prior_digest=COALESCE((SELECT batch_digest FROM chain_event_reorg_recovery_batch WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET prior_ordinal=COALESCE((SELECT ordinal FROM chain_event_reorg_recovery_batch WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),0);
  SELECT boundary_block,target_safe_block INTO run_boundary,run_target FROM chain_event_reorg_recovery_run WHERE id=NEW.run_id;
  SET prior_to=COALESCE((SELECT to_block FROM chain_event_reorg_recovery_batch WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),run_boundary);
  SET expected_first_log=COALESCE((SELECT first_log_ordinal+log_count FROM chain_event_reorg_recovery_batch WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),1);
  IF NEW.ordinal<>prior_ordinal+1 OR NEW.from_block<>prior_to+1 OR NEW.to_block<>LEAST(NEW.from_block+99,run_target)
     OR NEW.first_log_ordinal<>expected_first_log OR NEW.previous_batch_digest<>prior_digest
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_job j WHERE j.run_id=NEW.run_id AND j.state='scanning'
       AND j.active_slot=1 AND j.next_block=NEW.from_block AND j.next_batch_ordinal=NEW.ordinal AND j.next_log_ordinal=NEW.first_log_ordinal
       AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery batch chain or range is not contiguous';
  END IF;
  IF NEW.log_count=0 AND NEW.batch_log_digest<>REPEAT('0',64) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='empty recovery batch must use zero log digest';
  END IF;
  SET exact_logs=(SELECT COUNT(*) FROM chain_event_reorg_recovery_log l WHERE l.run_id=NEW.run_id AND NEW.log_count>0
    AND l.ordinal BETWEEN NEW.first_log_ordinal AND NEW.first_log_ordinal+NEW.log_count-1
    AND l.block_number BETWEEN NEW.from_block AND NEW.to_block);
  SET materialized_logs=(SELECT COUNT(*) FROM chain_event_reorg_recovery_log l WHERE l.run_id=NEW.run_id AND l.ordinal>=NEW.first_log_ordinal);
  IF materialized_logs<>NEW.log_count OR (NEW.log_count=0 AND exact_logs<>0) OR (NEW.log_count>0 AND (exact_logs<>NEW.log_count
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_log l WHERE l.run_id=NEW.run_id
        AND l.ordinal=NEW.first_log_ordinal+NEW.log_count-1 AND l.row_digest=NEW.batch_log_digest))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery batch log evidence is incomplete';
  END IF;
  IF NEW.batch_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-recovery-batch.v1',CHAR(10),NEW.run_id,CHAR(10),NEW.ordinal,CHAR(10),
      NEW.from_block,CHAR(10),NEW.to_block,CHAR(10),NEW.from_block_hash,CHAR(10),NEW.to_block_hash,CHAR(10),NEW.first_log_ordinal,CHAR(10),
      NEW.log_count,CHAR(10),NEW.batch_log_digest,CHAR(10),NEW.previous_batch_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery batch digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_batch_no_update BEFORE UPDATE ON chain_event_reorg_recovery_batch FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery batches are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_batch_no_delete BEFORE DELETE ON chain_event_reorg_recovery_batch FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery batches are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_recovery_manifest_validate_insert BEFORE INSERT ON chain_event_reorg_recovery_manifest FOR EACH ROW
BEGIN
  DECLARE actual_batches INT UNSIGNED;
  DECLARE actual_logs BIGINT UNSIGNED;
  DECLARE last_batch VARCHAR(64);
  DECLARE last_log VARCHAR(64);
  DECLARE run_target BIGINT UNSIGNED;
  DECLARE final_to BIGINT UNSIGNED;
  SELECT expected_batch_count,target_safe_block INTO actual_batches,run_target FROM chain_event_reorg_recovery_run WHERE id=NEW.run_id AND incident_id=NEW.incident_id;
  IF actual_batches IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery manifest run identity mismatch'; END IF;
  IF NEW.batch_count<>actual_batches OR (SELECT COUNT(*) FROM chain_event_reorg_recovery_batch WHERE run_id=NEW.run_id)<>actual_batches THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery manifest batch set is incomplete';
  END IF;
  SELECT COALESCE(SUM(log_count),0),MAX(to_block) INTO actual_logs,final_to FROM chain_event_reorg_recovery_batch WHERE run_id=NEW.run_id;
  IF NEW.log_count<>actual_logs OR (SELECT COUNT(*) FROM chain_event_reorg_recovery_log WHERE run_id=NEW.run_id)<>actual_logs OR final_to<>run_target THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery manifest range or log set is incomplete';
  END IF;
  SET last_batch=COALESCE((SELECT batch_digest FROM chain_event_reorg_recovery_batch WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET last_log=COALESCE((SELECT row_digest FROM chain_event_reorg_recovery_log WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  IF NEW.batch_evidence_digest<>last_batch OR NEW.log_evidence_digest<>last_log THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery manifest evidence roots mismatch';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_job j WHERE j.run_id=NEW.run_id AND j.incident_id=NEW.incident_id
      AND j.state='scanning' AND j.active_slot=1 AND j.lease_token REGEXP '^[0-9a-f]{64}$'
      AND j.lease_expires_at>CURRENT_TIMESTAMP(6) AND j.next_block=run_target+1
      AND j.next_batch_ordinal=NEW.batch_count+1 AND j.observed_log_count=NEW.log_count
      AND j.batch_evidence_digest=NEW.batch_evidence_digest AND j.log_evidence_digest=NEW.log_evidence_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery manifest requires active complete leased job';
  END IF;
  IF NEW.manifest_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-recovery-manifest.v1',CHAR(10),NEW.run_id,CHAR(10),NEW.incident_id,CHAR(10),
      NEW.batch_count,CHAR(10),NEW.log_count,CHAR(10),NEW.batch_evidence_digest,CHAR(10),NEW.log_evidence_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery manifest digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_manifest_no_update BEFORE UPDATE ON chain_event_reorg_recovery_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery manifests are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_manifest_no_delete BEFORE DELETE ON chain_event_reorg_recovery_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery manifests are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_definition_validate_insert BEFORE INSERT ON chain_event_reorg_recovery_definition FOR EACH ROW
BEGIN
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-recovery-definition.v1',CHAR(10),
      NEW.object_type,CHAR(10),NEW.object_name,CHAR(10),NEW.definition_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery definition attestation digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_definition_no_update BEFORE UPDATE ON chain_event_reorg_recovery_definition FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery definition attestations are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_definition_no_delete BEFORE DELETE ON chain_event_reorg_recovery_definition FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery definition attestations are append-only'; END$$
DELIMITER ;

INSERT INTO chain_event_reorg_recovery_definition (object_type,object_name,definition_digest,row_digest,created_at)
SELECT object_type,object_name,definition_digest,LOWER(SHA2(CONCAT(
  'joincare.chain-event.reorg-recovery-definition.v1',CHAR(10),object_type,CHAR(10),object_name,CHAR(10),definition_digest
),256)),CURRENT_TIMESTAMP(6)
FROM (
  SELECT 'CHECK' object_type,tc.CONSTRAINT_NAME object_name,LOWER(SHA2(CONCAT(
    'joincare.mysql-check-definition.v1',CHAR(10),tc.TABLE_SCHEMA,CHAR(10),tc.TABLE_NAME,CHAR(10),
    tc.ENFORCED,CHAR(10),cc.CHECK_CLAUSE
  ),256)) definition_digest
  FROM information_schema.TABLE_CONSTRAINTS tc
  JOIN information_schema.CHECK_CONSTRAINTS cc ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
  WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK'
    AND tc.TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_recovery_job',
      'chain_event_reorg_recovery_event','chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch',
      'chain_event_reorg_recovery_manifest','chain_event_reorg_recovery_definition')
  UNION ALL
  SELECT 'TRIGGER',TRIGGER_NAME,LOWER(SHA2(CONCAT(
    'joincare.mysql-trigger-definition.v1',CHAR(10),EVENT_OBJECT_SCHEMA,CHAR(10),EVENT_OBJECT_TABLE,CHAR(10),
    ACTION_TIMING,CHAR(10),EVENT_MANIPULATION,CHAR(10),ACTION_ORIENTATION,CHAR(10),ACTION_STATEMENT,CHAR(10),
    SQL_MODE,CHAR(10),DEFINER,CHAR(10),CHARACTER_SET_CLIENT,CHAR(10),COLLATION_CONNECTION,CHAR(10),DATABASE_COLLATION
  ),256))
  FROM information_schema.TRIGGERS
  WHERE TRIGGER_SCHEMA=DATABASE() AND EVENT_OBJECT_TABLE IN ('chain_event_reorg_recovery_run',
    'chain_event_reorg_recovery_job','chain_event_reorg_recovery_event','chain_event_reorg_recovery_log',
    'chain_event_reorg_recovery_batch','chain_event_reorg_recovery_manifest','chain_event_reorg_recovery_definition')
) definitions
ORDER BY object_type,object_name;

DROP PROCEDURE IF EXISTS jc_assert_chain_event_reorg_recovery;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_reorg_recovery()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'
      AND TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_recovery_job','chain_event_reorg_recovery_event',
      'chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch','chain_event_reorg_recovery_manifest',
      'chain_event_reorg_recovery_definition'))<>7 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery tables are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND (
      (TRIGGER_NAME='trg_chain_event_reorg_recovery_run_validate_insert' AND ACTION_STATEMENT LIKE '%matching active incident%' AND ACTION_STATEMENT LIKE '%attempt is not contiguous%' AND ACTION_STATEMENT LIKE '%boundary checkpoint identity%')
      OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_job_validate_insert' AND ACTION_STATEMENT LIKE '%canonical initial state%')
      OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_job_validate_update' AND ACTION_STATEMENT LIKE '%scanning transition bypassed lease or batch checkpoint%' AND ACTION_STATEMENT LIKE '%terminal transition lacks leased matching event%' AND ACTION_STATEMENT LIKE '%lease_expires_at%' AND ACTION_STATEMENT LIKE '%batch_digest%' AND ACTION_STATEMENT LIKE '%requires manifest%')
      OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_event_validate_insert' AND ACTION_STATEMENT LIKE '%event chain%' AND ACTION_STATEMENT LIKE '%matching active leased job%'
        AND ACTION_STATEMENT LIKE '%NEW.event_type=''CREATED''%' AND ACTION_STATEMENT LIKE '%next_batch_ordinal=1%'
        AND ACTION_STATEMENT LIKE '%lease_token=''''%' AND ACTION_STATEMENT LIKE '%NEW.event_type<>''CREATED''%'
        AND ACTION_STATEMENT LIKE '%active_slot=1%' AND ACTION_STATEMENT LIKE '%lease_token REGEXP%'
        AND ACTION_STATEMENT LIKE '%lease_expires_at>CURRENT_TIMESTAMP(6)%' AND ACTION_STATEMENT LIKE '%SHA2%')
      OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_log_validate_insert' AND ACTION_STATEMENT LIKE '%active leased scan range%' AND ACTION_STATEMENT LIKE '%raw_log_digest%' AND ACTION_STATEMENT LIKE '%row digest%' AND ACTION_STATEMENT LIKE '%SHA2%')
      OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_batch_validate_insert' AND ACTION_STATEMENT LIKE '%range is not contiguous%' AND ACTION_STATEMENT LIKE '%first_log_ordinal%' AND ACTION_STATEMENT LIKE '%exact_logs%' AND ACTION_STATEMENT LIKE '%batch log evidence%' AND ACTION_STATEMENT LIKE '%SHA2%')
      OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_manifest_validate_insert' AND ACTION_STATEMENT LIKE '%batch set is incomplete%' AND ACTION_STATEMENT LIKE '%range or log set is incomplete%' AND ACTION_STATEMENT LIKE '%evidence roots%' AND ACTION_STATEMENT LIKE '%active complete leased job%' AND ACTION_STATEMENT LIKE '%SHA2%')
      OR (TRIGGER_NAME IN ('trg_chain_event_reorg_recovery_run_no_update','trg_chain_event_reorg_recovery_event_no_update','trg_chain_event_reorg_recovery_log_no_update','trg_chain_event_reorg_recovery_batch_no_update','trg_chain_event_reorg_recovery_manifest_no_update') AND EVENT_MANIPULATION='UPDATE' AND ACTION_STATEMENT LIKE '%append-only%')
      OR (TRIGGER_NAME IN ('trg_chain_event_reorg_recovery_run_no_delete','trg_chain_event_reorg_recovery_job_no_delete','trg_chain_event_reorg_recovery_event_no_delete','trg_chain_event_reorg_recovery_log_no_delete','trg_chain_event_reorg_recovery_batch_no_delete','trg_chain_event_reorg_recovery_manifest_no_delete') AND EVENT_MANIPULATION='DELETE' AND ACTION_STATEMENT LIKE '%SIGNAL%')
      OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_definition_validate_insert' AND ACTION_STATEMENT LIKE '%definition attestation digest mismatch%' AND ACTION_STATEMENT LIKE '%SHA2%')
      OR (TRIGGER_NAME IN ('trg_chain_event_reorg_recovery_definition_no_update','trg_chain_event_reorg_recovery_definition_no_delete') AND ACTION_STATEMENT LIKE '%definition attestations are append-only%' AND ACTION_STATEMENT LIKE '%SIGNAL%')
    ))<>21 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery triggers are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_NAME IN ('fk_chain_event_reorg_recovery_incident','fk_chain_event_reorg_recovery_checkpoint',
      'fk_chain_event_reorg_recovery_job_run','fk_chain_event_reorg_recovery_event_run','fk_chain_event_reorg_recovery_log_run',
      'fk_chain_event_reorg_recovery_batch_run','fk_chain_event_reorg_recovery_manifest_run') AND DELETE_RULE='RESTRICT')<>7 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery foreign keys are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS tc
      JOIN information_schema.CHECK_CONSTRAINTS cc ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
      WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK' AND tc.ENFORCED='YES' AND (
      (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_run_identity' AND cc.CHECK_CLAUSE LIKE '%run_uuid%regexp%' AND cc.CHECK_CLAUSE LIKE '%legacy_contract_address%' AND cc.CHECK_CLAUSE LIKE '%joincare-reorg-recovery-scan-v1%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_boundary' AND cc.CHECK_CLAUSE LIKE '%last_canonical_checkpoint%' AND cc.CHECK_CLAUSE LIKE '%boundary_checkpoint_id%is not null%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_target' AND cc.CHECK_CLAUSE LIKE '%expected_scanned_blocks%' AND cc.CHECK_CLAUSE LIKE '%expected_batch_count%' AND cc.CHECK_CLAUSE LIKE '%100%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_job_state' AND cc.CHECK_CLAUSE LIKE '%scanning%' AND cc.CHECK_CLAUSE LIKE '%completed%' AND cc.CHECK_CLAUSE LIKE '%invalidated%' AND cc.CHECK_CLAUSE NOT LIKE '%failed%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_job_digests' AND cc.CHECK_CLAUSE LIKE '%lease_token%regexp%' AND cc.CHECK_CLAUSE LIKE '%lease_expires_at%' AND cc.CHECK_CLAUSE LIKE '%fence_epoch%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_event_fields' AND cc.CHECK_CLAUSE LIKE '%BATCH_COMMITTED%' AND cc.CHECK_CLAUSE LIKE '%INVALIDATED%' AND cc.CHECK_CLAUSE LIKE '%previous_event_hash%regexp%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_log_identity' AND cc.CHECK_CLAUSE LIKE '%v1_legacy%' AND cc.CHECK_CLAUSE LIKE '%v2_stablecoin%' AND cc.CHECK_CLAUSE LIKE '%raw_log%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_log_digests' AND cc.CHECK_CLAUSE LIKE '%raw_log_digest%regexp%' AND cc.CHECK_CLAUSE LIKE '%previous_row_digest%regexp%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_batch_identity' AND cc.CHECK_CLAUSE LIKE '%to_block%from_block%' AND cc.CHECK_CLAUSE LIKE '%100%' AND cc.CHECK_CLAUSE LIKE '%first_log_ordinal%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_batch_digests' AND cc.CHECK_CLAUSE LIKE '%batch_log_digest%regexp%' AND cc.CHECK_CLAUSE LIKE '%previous_batch_digest%regexp%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_manifest_fields' AND cc.CHECK_CLAUSE LIKE '%regexp_like%batch_evidence_digest%' AND cc.CHECK_CLAUSE LIKE '%regexp_like%log_evidence_digest%' AND cc.CHECK_CLAUSE LIKE '%regexp_like%manifest_digest%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_definition_identity' AND cc.CHECK_CLAUSE LIKE '%CHECK%' AND cc.CHECK_CLAUSE LIKE '%TRIGGER%' AND cc.CHECK_CLAUSE LIKE '%object_name%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_definition_digests' AND cc.CHECK_CLAUSE LIKE '%regexp_like%definition_digest%' AND cc.CHECK_CLAUSE LIKE '%regexp_like%row_digest%')
    ))<>13 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery checks are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM chain_event_reorg_recovery_definition)<>34
    OR (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS tc
        WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK'
          AND tc.TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_recovery_job',
            'chain_event_reorg_recovery_event','chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch',
            'chain_event_reorg_recovery_manifest','chain_event_reorg_recovery_definition'))
       +(SELECT COUNT(*) FROM information_schema.TRIGGERS
        WHERE TRIGGER_SCHEMA=DATABASE() AND EVENT_OBJECT_TABLE IN ('chain_event_reorg_recovery_run',
          'chain_event_reorg_recovery_job','chain_event_reorg_recovery_event','chain_event_reorg_recovery_log',
          'chain_event_reorg_recovery_batch','chain_event_reorg_recovery_manifest',
          'chain_event_reorg_recovery_definition'))<>34
    OR EXISTS (
      SELECT 1 FROM chain_event_reorg_recovery_definition d
      LEFT JOIN (
        SELECT 'CHECK' object_type,tc.CONSTRAINT_NAME object_name,LOWER(SHA2(CONCAT(
          'joincare.mysql-check-definition.v1',CHAR(10),tc.TABLE_SCHEMA,CHAR(10),tc.TABLE_NAME,CHAR(10),
          tc.ENFORCED,CHAR(10),cc.CHECK_CLAUSE
        ),256)) definition_digest
        FROM information_schema.TABLE_CONSTRAINTS tc
        JOIN information_schema.CHECK_CONSTRAINTS cc ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
        WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK'
          AND tc.TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_recovery_job',
            'chain_event_reorg_recovery_event','chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch',
            'chain_event_reorg_recovery_manifest','chain_event_reorg_recovery_definition')
        UNION ALL
        SELECT 'TRIGGER',TRIGGER_NAME,LOWER(SHA2(CONCAT(
          'joincare.mysql-trigger-definition.v1',CHAR(10),EVENT_OBJECT_SCHEMA,CHAR(10),EVENT_OBJECT_TABLE,CHAR(10),
          ACTION_TIMING,CHAR(10),EVENT_MANIPULATION,CHAR(10),ACTION_ORIENTATION,CHAR(10),ACTION_STATEMENT,CHAR(10),
          SQL_MODE,CHAR(10),DEFINER,CHAR(10),CHARACTER_SET_CLIENT,CHAR(10),COLLATION_CONNECTION,CHAR(10),DATABASE_COLLATION
        ),256))
        FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND EVENT_OBJECT_TABLE IN (
          'chain_event_reorg_recovery_run','chain_event_reorg_recovery_job','chain_event_reorg_recovery_event',
          'chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch','chain_event_reorg_recovery_manifest',
          'chain_event_reorg_recovery_definition')
      ) current_definition ON current_definition.object_type=d.object_type AND current_definition.object_name=d.object_name
      WHERE current_definition.object_name IS NULL OR current_definition.definition_digest<>d.definition_digest
        OR d.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-recovery-definition.v1',CHAR(10),
          d.object_type,CHAR(10),d.object_name,CHAR(10),d.definition_digest),256))
    ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery exact definition attestations are incomplete or drifted';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_reorg_recovery();
DROP PROCEDURE jc_assert_chain_event_reorg_recovery;
