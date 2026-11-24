-- JoinCare IDO Sprint 1A-P3-B2-B0 · fail-closed reversibility assessment.
-- This migration creates evidence only. It cannot switch an effective
-- generation, mutate current facts, move cursor/checkpoints/outbox, or resolve
-- an active reorg incident. Every completed v1 assessment is BLOCKED by the
-- explicit prerequisites required by later application-plan work.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_event_reorg_assessment;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_event_reorg_assessment()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
      'chain_event_sync_state','chain_event_reorg_incident',
      'chain_event_reorg_materialization_run','chain_event_reorg_materialization_job',
      'chain_event_reorg_materialization_effect','chain_event_reorg_materialization_manifest'))<>6 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment prerequisites are missing';
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
      'chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
      'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker',
      'chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='pre-existing reorg assessment tables require manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_event_reorg_assessment();
DROP PROCEDURE jc_preflight_chain_event_reorg_assessment;

CREATE TABLE chain_event_reorg_assessment_run (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_uuid CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  materialization_run_id BIGINT UNSIGNED NOT NULL,
  materialization_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  algorithm_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  boundary_block BIGINT UNSIGNED NOT NULL,
  boundary_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  target_safe_block BIGINT UNSIGNED NOT NULL,
  target_safe_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  expected_effect_count BIGINT UNSIGNED NOT NULL,
  materialization_effect_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_assessment_uuid (run_uuid),
  UNIQUE KEY uk_chain_event_reorg_assessment_materialization (materialization_run_id),
  UNIQUE KEY uk_chain_event_reorg_assessment_incident (incident_id),
  CONSTRAINT fk_chain_event_reorg_assessment_run_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_assessment_run_materialization FOREIGN KEY (materialization_run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_assessment_run_identity CHECK (
    incident_id>0 AND materialization_run_id>0 AND chain_id>0
    AND run_uuid REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
    AND algorithm_version='joincare-reorg-reversibility-assessment-v1'
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_assessment_run_range CHECK (
    boundary_block>0 AND target_safe_block>=boundary_block
    AND boundary_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND target_safe_block_hash REGEXP '^0x[0-9a-f]{64}$'
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_assessment_run_source CHECK (
    expected_effect_count<=99993
    AND materialization_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND materialization_effect_root REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_assessment_job (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  state VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  active_slot TINYINT UNSIGNED NULL,
  next_effect_ordinal BIGINT UNSIGNED NOT NULL,
  disposition_count BIGINT UNSIGNED NOT NULL,
  exact_count BIGINT UNSIGNED NOT NULL,
  conditional_count BIGINT UNSIGNED NOT NULL,
  blocked_count BIGINT UNSIGNED NOT NULL,
  noop_count BIGINT UNSIGNED NOT NULL,
  blocker_count BIGINT UNSIGNED NOT NULL,
  disposition_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  blocker_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  lease_token VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  lease_expires_at DATETIME(6) NULL,
  fence_epoch BIGINT UNSIGNED NOT NULL DEFAULT 0,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  last_error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  last_error_message VARCHAR(500) NOT NULL DEFAULT '',
  completed_at DATETIME(6) NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_assessment_active (incident_id,active_slot),
  KEY idx_chain_event_reorg_assessment_job_state (state,updated_at,run_id),
  CONSTRAINT fk_chain_event_reorg_assessment_job_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_assessment_job_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_assessment_job_state CHECK (
    state IN ('preparing','completed','invalidated')
    AND ((state='preparing' AND active_slot IS NOT NULL AND active_slot=1 AND completed_at IS NULL)
      OR (state<>'preparing' AND active_slot IS NULL AND completed_at IS NOT NULL))
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_assessment_job_checkpoint CHECK (
    next_effect_ordinal>0 AND disposition_count<=100000 AND blocker_count<=100000
    AND exact_count+conditional_count+blocked_count+noop_count=disposition_count
    AND disposition_digest REGEXP '^[0-9a-f]{64}$' AND blocker_digest REGEXP '^[0-9a-f]{64}$'
    AND ((lease_token='' AND lease_expires_at IS NULL)
      OR (lease_token REGEXP '^[0-9a-f]{64}$' AND lease_expires_at IS NOT NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_assessment_disposition (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  materialization_effect_id BIGINT UNSIGNED NOT NULL,
  source_effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  branch VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_type VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  replayability VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  action_kind VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  blocker_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  plan_payload MEDIUMBLOB NOT NULL,
  plan_payload_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_assessment_disposition_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_assessment_disposition_effect (run_id,materialization_effect_id),
  UNIQUE KEY uk_chain_event_reorg_assessment_disposition_digest (run_id,row_digest),
  UNIQUE KEY uk_chain_event_reorg_assessment_disposition_run_id (run_id,id),
  KEY idx_chain_event_reorg_assessment_disposition_project (run_id,project_id,ordinal),
  CONSTRAINT fk_chain_event_reorg_assessment_disposition_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_assessment_disposition_effect FOREIGN KEY (materialization_effect_id)
    REFERENCES chain_event_reorg_materialization_effect(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_assessment_disposition_identity CHECK (
    ordinal>0 AND materialization_effect_id>0 AND project_id<=9223372036854775807
    AND source_effect_digest REGEXP '^[0-9a-f]{64}$'
    AND branch IN ('old_committed','replacement')
    AND contract_version IN ('v1_legacy','v2_stablecoin')
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_assessment_disposition_semantics CHECK (
    replayability IN ('exact','conditional','blocked','noop')
    AND ((replayability IN ('conditional','blocked') AND CHAR_LENGTH(blocker_code)>0)
      OR (replayability IN ('exact','noop') AND blocker_code=''))
    AND action_kind REGEXP '^(apply|reverse)_[a-z0-9_]{3,56}$'
    AND OCTET_LENGTH(plan_payload) BETWEEN 2 AND 1048576
    AND plan_payload_digest REGEXP '^[0-9a-f]{64}$'
    AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_assessment_blocker (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  disposition_id BIGINT UNSIGNED NULL,
  scope VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  blocker_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  safe_detail VARCHAR(500) NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_assessment_blocker_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_assessment_blocker_disposition (run_id,disposition_id),
  UNIQUE KEY uk_chain_event_reorg_assessment_blocker_digest (run_id,row_digest),
  CONSTRAINT fk_chain_event_reorg_assessment_blocker_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_assessment_blocker_disposition FOREIGN KEY (run_id,disposition_id)
    REFERENCES chain_event_reorg_assessment_disposition(run_id,id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_assessment_blocker_fields CHECK (
    ordinal>0 AND scope IN ('run','effect')
    AND ((scope='run' AND disposition_id IS NULL) OR (scope='effect' AND disposition_id IS NOT NULL))
    AND CHAR_LENGTH(blocker_code) BETWEEN 3 AND 64
    AND CHAR_LENGTH(TRIM(safe_detail)) BETWEEN 10 AND 500
    AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_assessment_event (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  sequence_no INT UNSIGNED NOT NULL,
  event_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  message VARCHAR(500) NOT NULL,
  previous_event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_assessment_event_sequence (run_id,sequence_no),
  UNIQUE KEY uk_chain_event_reorg_assessment_event_hash (event_hash),
  CONSTRAINT fk_chain_event_reorg_assessment_event_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_assessment_event_fields CHECK (
    sequence_no>0 AND event_type IN ('CREATED','COMPLETED','INVALIDATED')
    AND ((event_type='CREATED' AND sequence_no=1 AND error_code='')
      OR (event_type='COMPLETED' AND sequence_no>1 AND error_code='')
      OR (event_type='INVALIDATED' AND sequence_no>1 AND CHAR_LENGTH(error_code)>0))
    AND CHAR_LENGTH(TRIM(message)) BETWEEN 3 AND 500
    AND previous_event_hash REGEXP '^[0-9a-f]{64}$' AND event_hash REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_assessment_manifest (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  materialization_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  outcome VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  disposition_count BIGINT UNSIGNED NOT NULL,
  exact_count BIGINT UNSIGNED NOT NULL,
  conditional_count BIGINT UNSIGNED NOT NULL,
  blocked_count BIGINT UNSIGNED NOT NULL,
  noop_count BIGINT UNSIGNED NOT NULL,
  blocker_count BIGINT UNSIGNED NOT NULL,
  disposition_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  blocker_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  completed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_assessment_manifest_digest (manifest_digest),
  CONSTRAINT fk_chain_event_reorg_assessment_manifest_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_assessment_manifest_fields CHECK (
    incident_id>0 AND outcome='BLOCKED' AND blocker_count>=7
    AND exact_count+conditional_count+blocked_count+noop_count=disposition_count
    AND materialization_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND disposition_digest REGEXP '^[0-9a-f]{64}$' AND blocker_digest REGEXP '^[0-9a-f]{64}$'
    AND manifest_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DELIMITER $$
CREATE TRIGGER trg_chain_event_reorg_assessment_run_validate_insert BEFORE INSERT ON chain_event_reorg_assessment_run FOR EACH ROW
BEGIN
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment run id must be database assigned'; END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_incident i WHERE i.id=NEW.incident_id
      AND i.status='detected' AND i.active_slot=1 AND i.chain_id=NEW.chain_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment run requires matching active incident';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_run r
      JOIN chain_event_reorg_materialization_job j ON j.run_id=r.id AND j.state='completed'
      JOIN chain_event_reorg_materialization_manifest m ON m.run_id=r.id AND m.incident_id=r.incident_id
      WHERE r.id=NEW.materialization_run_id AND r.incident_id=NEW.incident_id AND r.chain_id=NEW.chain_id
      AND r.boundary_block=NEW.boundary_block AND r.boundary_block_hash=NEW.boundary_block_hash
      AND r.target_safe_block=NEW.target_safe_block AND r.target_safe_block_hash=NEW.target_safe_block_hash
      AND m.manifest_digest=NEW.materialization_manifest_digest
      AND m.old_effect_count+m.replacement_effect_count=NEW.expected_effect_count
      AND (SELECT COUNT(*) FROM chain_event_reorg_materialization_effect e WHERE e.run_id=r.id)=NEW.expected_effect_count
      AND (SELECT COALESCE(SUM(OCTET_LENGTH(e.semantic_payload)),0) FROM chain_event_reorg_materialization_effect e
        WHERE e.run_id=r.id)<=67108864
      AND NEW.materialization_effect_root=LOWER(SHA2(CONCAT('joincare.chain-event.reorg-assessment-source.v1',CHAR(10),
        m.run_id,CHAR(10),m.incident_id,CHAR(10),m.old_effect_count,CHAR(10),m.replacement_effect_count,CHAR(10),
        m.ignored_log_count,CHAR(10),m.old_effect_digest,CHAR(10),m.replacement_effect_digest,CHAR(10),m.manifest_digest),256))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment materialization source identity mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_run_no_update BEFORE UPDATE ON chain_event_reorg_assessment_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment runs are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_run_no_delete BEFORE DELETE ON chain_event_reorg_assessment_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment runs are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_assessment_job_validate_insert BEFORE INSERT ON chain_event_reorg_assessment_job FOR EACH ROW
BEGIN
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_run r WHERE r.id=NEW.run_id AND r.incident_id=NEW.incident_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment job incident does not match parent run';
  END IF;
  IF NEW.state<>'preparing' OR NEW.active_slot IS NULL OR NEW.active_slot<>1 OR NEW.next_effect_ordinal<>1
     OR NEW.disposition_count<>0 OR NEW.exact_count<>0 OR NEW.conditional_count<>0 OR NEW.blocked_count<>0
     OR NEW.noop_count<>0 OR NEW.blocker_count<>0 OR NEW.disposition_digest<>REPEAT('0',64)
     OR NEW.blocker_digest<>REPEAT('0',64) OR NEW.lease_token<>'' OR NEW.lease_expires_at IS NOT NULL
     OR NEW.fence_epoch<>0 OR NEW.attempt_count<>0 OR NEW.last_error_code<>'' OR NEW.last_error_message<>'' THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment job initial checkpoint is not canonical';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_job_validate_update BEFORE UPDATE ON chain_event_reorg_assessment_job FOR EACH ROW
BEGIN
  IF NEW.run_id<>OLD.run_id OR NEW.incident_id<>OLD.incident_id OR NEW.fence_epoch<OLD.fence_epoch
     OR NEW.next_effect_ordinal<OLD.next_effect_ordinal OR NEW.disposition_count<OLD.disposition_count
     OR NEW.exact_count<OLD.exact_count OR NEW.conditional_count<OLD.conditional_count
     OR NEW.blocked_count<OLD.blocked_count OR NEW.noop_count<OLD.noop_count OR NEW.blocker_count<OLD.blocker_count
     OR (OLD.state<>'preparing' AND (NEW.state<>OLD.state OR NOT (NEW.active_slot<=>OLD.active_slot))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment job transition is not monotonic';
  END IF;
  IF NEW.state='preparing' AND EXISTS (SELECT 1 FROM chain_event_reorg_assessment_event e
      WHERE e.run_id=NEW.run_id AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment terminal event forbids further preparation';
  END IF;
  IF NEW.state='preparing' AND (NEW.active_slot IS NULL OR NEW.active_slot<>1 OR NOT (
      (NEW.fence_epoch=OLD.fence_epoch AND OLD.lease_token REGEXP '^[0-9a-f]{64}$'
        AND OLD.lease_expires_at>CURRENT_TIMESTAMP(6) AND (NEW.lease_token=OLD.lease_token OR NEW.lease_token=''))
      OR (NEW.fence_epoch=OLD.fence_epoch+1 AND (OLD.lease_token='' OR OLD.lease_expires_at<=CURRENT_TIMESTAMP(6))
        AND NEW.lease_token REGEXP '^[0-9a-f]{64}$' AND NEW.lease_expires_at>CURRENT_TIMESTAMP(6)
        AND NEW.next_effect_ordinal=OLD.next_effect_ordinal AND NEW.disposition_count=OLD.disposition_count
        AND NEW.exact_count=OLD.exact_count AND NEW.conditional_count=OLD.conditional_count
        AND NEW.blocked_count=OLD.blocked_count AND NEW.noop_count=OLD.noop_count AND NEW.blocker_count=OLD.blocker_count
        AND NEW.disposition_digest=OLD.disposition_digest AND NEW.blocker_digest=OLD.blocker_digest
        AND NEW.attempt_count=OLD.attempt_count+1))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment lease or checkpoint transition rejected';
  END IF;
  IF (SELECT COUNT(*) FROM chain_event_reorg_assessment_disposition d WHERE d.run_id=NEW.run_id)<>NEW.disposition_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_assessment_disposition d WHERE d.run_id=NEW.run_id AND d.replayability='exact')<>NEW.exact_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_assessment_disposition d WHERE d.run_id=NEW.run_id AND d.replayability='conditional')<>NEW.conditional_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_assessment_disposition d WHERE d.run_id=NEW.run_id AND d.replayability='blocked')<>NEW.blocked_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_assessment_disposition d WHERE d.run_id=NEW.run_id AND d.replayability='noop')<>NEW.noop_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_assessment_blocker b WHERE b.run_id=NEW.run_id)<>NEW.blocker_count
     OR COALESCE((SELECT d.row_digest FROM chain_event_reorg_assessment_disposition d
       WHERE d.run_id=NEW.run_id ORDER BY d.ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.disposition_digest
     OR COALESCE((SELECT b.row_digest FROM chain_event_reorg_assessment_blocker b
       WHERE b.run_id=NEW.run_id ORDER BY b.ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.blocker_digest THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment job checkpoint does not match evidence rows';
  END IF;
  IF NEW.state IN ('completed','invalidated') AND NOT (OLD.state='preparing'
      AND OLD.lease_token REGEXP '^[0-9a-f]{64}$' AND OLD.lease_expires_at>CURRENT_TIMESTAMP(6)
      AND NEW.active_slot IS NULL AND NEW.lease_token='' AND NEW.lease_expires_at IS NULL
      AND NEW.next_effect_ordinal=OLD.next_effect_ordinal AND NEW.disposition_count=OLD.disposition_count
      AND NEW.exact_count=OLD.exact_count AND NEW.conditional_count=OLD.conditional_count
      AND NEW.blocked_count=OLD.blocked_count AND NEW.noop_count=OLD.noop_count AND NEW.blocker_count=OLD.blocker_count
      AND NEW.disposition_digest=OLD.disposition_digest AND NEW.blocker_digest=OLD.blocker_digest
      AND NEW.fence_epoch=OLD.fence_epoch) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment terminal transition requires active DB lease';
  END IF;
  IF NEW.state='completed' AND (NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_manifest m
      WHERE m.run_id=NEW.run_id AND m.incident_id=NEW.incident_id AND m.outcome='BLOCKED'
      AND m.disposition_count=NEW.disposition_count AND m.exact_count=NEW.exact_count
      AND m.conditional_count=NEW.conditional_count AND m.blocked_count=NEW.blocked_count
      AND m.noop_count=NEW.noop_count AND m.blocker_count=NEW.blocker_count
      AND m.disposition_digest=NEW.disposition_digest AND m.blocker_digest=NEW.blocker_digest)
      OR (SELECT COUNT(*) FROM chain_event_reorg_assessment_event e WHERE e.run_id=NEW.run_id
        AND e.event_type IN ('COMPLETED','INVALIDATED'))<>1
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_event e WHERE e.run_id=NEW.run_id
        AND e.event_type='COMPLETED' AND e.sequence_no=(SELECT MAX(sequence_no) FROM chain_event_reorg_assessment_event WHERE run_id=NEW.run_id))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment completion requires blocked manifest and terminal event';
  END IF;
  IF NEW.state='invalidated' AND (CHAR_LENGTH(NEW.last_error_code)=0
      OR (SELECT COUNT(*) FROM chain_event_reorg_assessment_event e WHERE e.run_id=NEW.run_id
        AND e.event_type IN ('COMPLETED','INVALIDATED'))<>1
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_event e WHERE e.run_id=NEW.run_id
        AND e.event_type='INVALIDATED' AND e.error_code=NEW.last_error_code
        AND e.sequence_no=(SELECT MAX(sequence_no) FROM chain_event_reorg_assessment_event WHERE run_id=NEW.run_id))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment invalidation requires matching terminal event';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_job_no_delete BEFORE DELETE ON chain_event_reorg_assessment_job FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment jobs cannot be deleted'; END$$

CREATE TRIGGER trg_chain_event_reorg_assessment_disposition_validate_insert BEFORE INSERT ON chain_event_reorg_assessment_disposition FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64);
  DECLARE expected_ordinal BIGINT UNSIGNED;
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition id must be database assigned'; END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.active_slot=1
      AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition requires active leased checkpoint';
  END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_assessment_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment terminal event forbids dispositions';
  END IF;
  SET expected_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_assessment_disposition WHERE run_id=NEW.run_id),1);
  SET expected_previous=COALESCE((SELECT row_digest FROM chain_event_reorg_assessment_disposition
      WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),(SELECT disposition_digest FROM chain_event_reorg_assessment_job WHERE run_id=NEW.run_id));
  IF NEW.ordinal<>expected_ordinal OR NEW.previous_row_digest<>expected_previous THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition chain is not contiguous';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_run r
      JOIN chain_event_reorg_materialization_effect e ON e.run_id=r.materialization_run_id
      WHERE r.id=NEW.run_id AND e.id=NEW.materialization_effect_id AND e.ordinal=NEW.ordinal
      AND e.effect_digest=NEW.source_effect_digest AND e.branch=NEW.branch
      AND e.contract_version=NEW.contract_version AND e.event_type=NEW.event_type AND e.project_id=NEW.project_id
      AND ((e.classification='ignored_abi_log' AND NEW.replayability='noop' AND NEW.action_kind='apply_approved_abi_noop'
          AND NEW.branch='replacement' AND NEW.event_type='ignored' AND NEW.project_id=0)
        OR (e.classification='semantic_event' AND NEW.replayability<>'noop' AND NEW.event_type<>'ignored'))
      AND ((NEW.branch='old_committed' AND LEFT(NEW.action_kind,8)='reverse_')
        OR (NEW.branch='replacement' AND LEFT(NEW.action_kind,6)='apply_'))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition source effect mismatch';
  END IF;
  IF JSON_VALID(NEW.plan_payload)<>1
     OR JSON_CONTAINS_PATH(NEW.plan_payload,'all','$.schema_version','$.source_effect_digest','$.branch',
       '$.contract_version','$.event_type','$.project_id','$.replayability','$.action_kind')<>1
     OR JSON_TYPE(JSON_EXTRACT(NEW.plan_payload,'$.schema_version'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(NEW.plan_payload,'$.source_effect_digest'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(NEW.plan_payload,'$.branch'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(NEW.plan_payload,'$.contract_version'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(NEW.plan_payload,'$.event_type'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(NEW.plan_payload,'$.project_id'))<>'INTEGER'
     OR JSON_TYPE(JSON_EXTRACT(NEW.plan_payload,'$.replayability'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(NEW.plan_payload,'$.action_kind'))<>'STRING'
     OR JSON_UNQUOTE(JSON_EXTRACT(NEW.plan_payload,'$.schema_version'))<>'reorg-assessment-plan-v1'
     OR JSON_UNQUOTE(JSON_EXTRACT(NEW.plan_payload,'$.source_effect_digest'))<>NEW.source_effect_digest
     OR JSON_UNQUOTE(JSON_EXTRACT(NEW.plan_payload,'$.branch'))<>NEW.branch
     OR JSON_UNQUOTE(JSON_EXTRACT(NEW.plan_payload,'$.contract_version'))<>NEW.contract_version
     OR JSON_UNQUOTE(JSON_EXTRACT(NEW.plan_payload,'$.event_type'))<>NEW.event_type
     OR CAST(JSON_EXTRACT(NEW.plan_payload,'$.project_id') AS UNSIGNED)<>NEW.project_id
     OR JSON_UNQUOTE(JSON_EXTRACT(NEW.plan_payload,'$.replayability'))<>NEW.replayability
     OR JSON_UNQUOTE(JSON_EXTRACT(NEW.plan_payload,'$.action_kind'))<>NEW.action_kind
     OR ((NEW.blocker_code='' AND (JSON_LENGTH(NEW.plan_payload)<>8 OR JSON_CONTAINS_PATH(NEW.plan_payload,'one','$.blocker_code')=1))
       OR (NEW.blocker_code<>'' AND (JSON_LENGTH(NEW.plan_payload)<>9
         OR JSON_CONTAINS_PATH(NEW.plan_payload,'all','$.blocker_code')<>1
         OR JSON_TYPE(JSON_EXTRACT(NEW.plan_payload,'$.blocker_code'))<>'STRING'
         OR JSON_UNQUOTE(JSON_EXTRACT(NEW.plan_payload,'$.blocker_code'))<>NEW.blocker_code))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition plan payload identity mismatch';
  END IF;
  IF NEW.plan_payload_digest<>LOWER(SHA2(NEW.plan_payload,256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition payload digest mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-assessment-disposition.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.ordinal,CHAR(10),NEW.materialization_effect_id,CHAR(10),NEW.source_effect_digest,CHAR(10),
      NEW.branch,CHAR(10),NEW.contract_version,CHAR(10),NEW.event_type,CHAR(10),NEW.project_id,CHAR(10),
      NEW.replayability,CHAR(10),NEW.action_kind,CHAR(10),NEW.blocker_code,CHAR(10),NEW.plan_payload_digest,CHAR(10),
      NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition row digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_disposition_no_update BEFORE UPDATE ON chain_event_reorg_assessment_disposition FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment dispositions are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_disposition_no_delete BEFORE DELETE ON chain_event_reorg_assessment_disposition FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment dispositions are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_assessment_blocker_validate_insert BEFORE INSERT ON chain_event_reorg_assessment_blocker FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64);
  DECLARE expected_ordinal BIGINT UNSIGNED;
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment blocker id must be database assigned'; END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.active_slot=1 AND j.lease_token REGEXP '^[0-9a-f]{64}$'
      AND j.lease_expires_at>CURRENT_TIMESTAMP(6)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment blocker requires active DB lease';
  END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_assessment_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment terminal event forbids blockers';
  END IF;
  SET expected_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_assessment_blocker WHERE run_id=NEW.run_id),1);
  SET expected_previous=COALESCE((SELECT row_digest FROM chain_event_reorg_assessment_blocker
      WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),(SELECT blocker_digest FROM chain_event_reorg_assessment_job WHERE run_id=NEW.run_id));
  IF NEW.ordinal<>expected_ordinal OR NEW.previous_row_digest<>expected_previous THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment blocker chain is not contiguous';
  END IF;
  IF NEW.scope='effect' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_disposition d
      WHERE d.id=NEW.disposition_id AND d.run_id=NEW.run_id AND d.blocker_code=NEW.blocker_code
      AND d.replayability IN ('conditional','blocked')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment effect blocker does not match disposition';
  END IF;
  IF NEW.scope='run' AND NOT (NEW.ordinal BETWEEN 1 AND 7 AND NEW.blocker_code=CASE NEW.ordinal
      WHEN 1 THEN 'BOUNDARY_STATE_SNAPSHOT_REQUIRED'
      WHEN 2 THEN 'V1_SOURCE_ORIGIN_UNATTESTED'
      WHEN 3 THEN 'GLOBAL_CONFIG_BASELINE_REQUIRED'
      WHEN 4 THEN 'OUTBOX_GENERATION_BINDING_REQUIRED'
      WHEN 5 THEN 'GENERATION_CHECKPOINT_SCHEMA_REQUIRED'
      WHEN 6 THEN 'DUAL_CONTROL_AUTHORIZATION_REQUIRED'
      WHEN 7 THEN 'INDEPENDENT_CHAIN_PREFLIGHT_REQUIRED' END) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment mandatory run blocker is not approved';
  END IF;
  IF NEW.scope='effect' AND NEW.ordinal<=7 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment effect blocker cannot replace mandatory run blockers';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-assessment-blocker.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.ordinal,CHAR(10),COALESCE(NEW.disposition_id,0),CHAR(10),NEW.scope,CHAR(10),
      NEW.blocker_code,CHAR(10),NEW.safe_detail,CHAR(10),NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment blocker row digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_blocker_no_update BEFORE UPDATE ON chain_event_reorg_assessment_blocker FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment blockers are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_blocker_no_delete BEFORE DELETE ON chain_event_reorg_assessment_blocker FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment blockers are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_assessment_event_validate_insert BEFORE INSERT ON chain_event_reorg_assessment_event FOR EACH ROW
BEGIN
  DECLARE prior_hash VARCHAR(64);
  DECLARE prior_sequence INT UNSIGNED;
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment event id must be database assigned'; END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_assessment_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment terminal event is final';
  END IF;
  SET prior_hash=COALESCE((SELECT event_hash FROM chain_event_reorg_assessment_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),REPEAT('0',64));
  SET prior_sequence=COALESCE((SELECT sequence_no FROM chain_event_reorg_assessment_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),0);
  IF NEW.sequence_no<>prior_sequence+1 OR NEW.previous_event_hash<>prior_hash THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment event chain is not contiguous';
  END IF;
  IF NEW.event_type='CREATED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.active_slot=1 AND j.fence_epoch=0 AND j.lease_token='') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment CREATED event requires canonical initial job';
  END IF;
  IF NEW.event_type<>'CREATED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.active_slot=1 AND j.lease_token REGEXP '^[0-9a-f]{64}$'
      AND j.lease_expires_at>CURRENT_TIMESTAMP(6)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment event requires active DB lease';
  END IF;
  IF NEW.event_type='COMPLETED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_manifest m
      WHERE m.run_id=NEW.run_id AND m.outcome='BLOCKED') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment completion event requires blocked manifest';
  END IF;
  IF NEW.event_type='INVALIDATED' AND EXISTS (SELECT 1 FROM chain_event_reorg_assessment_manifest m WHERE m.run_id=NEW.run_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment manifest forbids invalidation';
  END IF;
  IF NEW.event_hash<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-assessment-event.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.sequence_no,CHAR(10),NEW.event_type,CHAR(10),NEW.error_code,CHAR(10),
      NEW.message,CHAR(10),NEW.previous_event_hash),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment event digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_event_no_update BEFORE UPDATE ON chain_event_reorg_assessment_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment events are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_event_no_delete BEFORE DELETE ON chain_event_reorg_assessment_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment events are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_assessment_manifest_validate_insert BEFORE INSERT ON chain_event_reorg_assessment_manifest FOR EACH ROW
BEGIN
  IF EXISTS (SELECT 1 FROM chain_event_reorg_assessment_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment terminal event forbids manifest insertion';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_job j
      JOIN chain_event_reorg_assessment_run r ON r.id=j.run_id AND r.incident_id=j.incident_id
      WHERE j.run_id=NEW.run_id AND j.incident_id=NEW.incident_id AND j.state='preparing' AND j.active_slot=1
      AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
      AND r.materialization_manifest_digest=NEW.materialization_manifest_digest
      AND r.expected_effect_count=NEW.disposition_count
      AND j.disposition_count=NEW.disposition_count AND j.exact_count=NEW.exact_count
      AND j.conditional_count=NEW.conditional_count AND j.blocked_count=NEW.blocked_count
      AND j.noop_count=NEW.noop_count AND j.blocker_count=NEW.blocker_count
      AND j.disposition_digest=NEW.disposition_digest AND j.blocker_digest=NEW.blocker_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment manifest requires complete leased checkpoint';
  END IF;
  IF (SELECT COUNT(*) FROM chain_event_reorg_assessment_disposition WHERE run_id=NEW.run_id)<>NEW.disposition_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_assessment_blocker WHERE run_id=NEW.run_id)<>NEW.blocker_count
     OR COALESCE((SELECT row_digest FROM chain_event_reorg_assessment_disposition WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.disposition_digest
     OR COALESCE((SELECT row_digest FROM chain_event_reorg_assessment_blocker WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.blocker_digest THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment manifest evidence sets are incomplete';
  END IF;
  IF NEW.outcome<>'BLOCKED' OR NEW.blocker_count<7 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment v1 cannot authorize a generation switch';
  END IF;
  IF NEW.manifest_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-assessment-manifest.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.incident_id,CHAR(10),NEW.materialization_manifest_digest,CHAR(10),NEW.outcome,CHAR(10),
      NEW.disposition_count,CHAR(10),NEW.exact_count,CHAR(10),NEW.conditional_count,CHAR(10),NEW.blocked_count,CHAR(10),
      NEW.noop_count,CHAR(10),NEW.blocker_count,CHAR(10),NEW.disposition_digest,CHAR(10),NEW.blocker_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment manifest digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_manifest_no_update BEFORE UPDATE ON chain_event_reorg_assessment_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment manifests are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_manifest_no_delete BEFORE DELETE ON chain_event_reorg_assessment_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment manifests are append-only'; END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS jc_assert_chain_event_reorg_assessment;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_reorg_assessment()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'
      AND TABLE_NAME IN ('chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
      'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker',
      'chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest'))<>6 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment tables are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
      AND TRIGGER_NAME REGEXP '^trg_chain_event_reorg_assessment_')<>18 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment triggers are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES'
      AND CONSTRAINT_NAME REGEXP '^chk_chain_event_reorg_assessment_')<>10 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment checks are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_NAME REGEXP '^fk_chain_event_reorg_assessment_' AND DELETE_RULE='RESTRICT')<>10 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment foreign keys are incomplete';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_reorg_assessment();
DROP PROCEDURE jc_assert_chain_event_reorg_assessment;

-- Deployment runner must replace this candidate checksum with the exact file
-- SHA256 before execution and record status=applied only after postflight.
