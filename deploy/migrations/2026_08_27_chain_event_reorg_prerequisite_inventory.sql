-- JoinCare IDO Sprint 1A-P3-B2-B1-A · fail-closed prerequisite inventory.
-- V1 records only the seven unresolved prerequisite domains. It cannot accept
-- trusted artifacts, build a candidate generation, mutate current state, bind
-- outbox transactions, move checkpoints/cursor, or resolve an incident.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_event_reorg_prerequisite;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_event_reorg_prerequisite()
BEGIN
  IF (SELECT COUNT(*) FROM schema_migrations WHERE status='applied' AND (
      (filename='2026_08_25_chain_event_reorg_reversibility_assessment.sql'
        AND checksum='31764df81b1f74ef8a00a8bd785b81f9835dd9132a3e02361c217c8ca98ab68e')
      OR (filename='2026_08_25_zz_chain_event_reorg_assessment_fk_restrict.sql'
        AND checksum='ae2387d475ca19edf3292592e051b7d00bc4ed2e167b3be7d788bb431c799575')
      OR (filename='2026_08_26_chain_event_reorg_assessment_plan_json.sql'
        AND checksum='32f952eb708663ed381a407ccea827a53f4c8e7436cc48efde0be8d4201b467b')
    ))<>3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite assessment migrations are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_TYPE='BASE TABLE' AND TABLE_NAME IN ('chain_event_sync_state','chain_event_reorg_incident',
      'chain_event_reorg_assessment_run','chain_event_reorg_assessment_job','chain_event_reorg_assessment_manifest'))<>5 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite source tables are incomplete';
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
      'chain_event_reorg_prerequisite_run','chain_event_reorg_prerequisite_job',
      'chain_event_reorg_prerequisite_artifact','chain_event_reorg_prerequisite_requirement',
      'chain_event_reorg_prerequisite_event','chain_event_reorg_prerequisite_manifest')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='pre-existing reorg prerequisite tables require manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_event_reorg_prerequisite();
DROP PROCEDURE jc_preflight_chain_event_reorg_prerequisite;

CREATE TABLE chain_event_reorg_prerequisite_run (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_uuid CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  assessment_run_id BIGINT UNSIGNED NOT NULL,
  assessment_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  algorithm_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  boundary_block BIGINT UNSIGNED NOT NULL,
  boundary_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  expected_requirement_count BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_uuid (run_uuid),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_assessment (assessment_run_id),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_incident (incident_id),
  CONSTRAINT fk_chain_event_reorg_prerequisite_run_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_prerequisite_run_assessment FOREIGN KEY (assessment_run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_prerequisite_run_identity CHECK (
    incident_id>0 AND assessment_run_id>0 AND chain_id>0 AND boundary_block>0
    AND run_uuid REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
    AND algorithm_version='joincare-reorg-prerequisite-inventory-v1'
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_prerequisite_run_source CHECK (
    assessment_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND boundary_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND expected_requirement_count=7
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_prerequisite_job (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  state VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  active_slot TINYINT UNSIGNED NULL,
  next_requirement_ordinal BIGINT UNSIGNED NOT NULL,
  requirement_count BIGINT UNSIGNED NOT NULL,
  unmet_count BIGINT UNSIGNED NOT NULL,
  untrusted_count BIGINT UNSIGNED NOT NULL,
  deferred_count BIGINT UNSIGNED NOT NULL,
  artifact_count BIGINT UNSIGNED NOT NULL,
  requirement_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  artifact_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  lease_token VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  lease_expires_at DATETIME(6) NULL,
  fence_epoch BIGINT UNSIGNED NOT NULL DEFAULT 0,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  last_error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  last_error_message VARCHAR(500) NOT NULL DEFAULT '',
  completed_at DATETIME(6) NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_active (incident_id,active_slot),
  KEY idx_chain_event_reorg_prerequisite_job_state (state,updated_at,run_id),
  CONSTRAINT fk_chain_event_reorg_prerequisite_job_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_prerequisite_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_prerequisite_job_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_prerequisite_job_state CHECK (
    state IN ('preparing','completed','invalidated')
    AND ((state='preparing' AND active_slot IS NOT NULL AND active_slot=1 AND completed_at IS NULL)
      OR (state<>'preparing' AND active_slot IS NULL AND completed_at IS NOT NULL))
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_prerequisite_job_checkpoint CHECK (
    next_requirement_ordinal BETWEEN 1 AND 8 AND requirement_count<=7
    AND unmet_count+untrusted_count+deferred_count=requirement_count
    AND artifact_count=0 AND artifact_digest=REPEAT('0',64)
    AND requirement_digest REGEXP '^[0-9a-f]{64}$'
    AND ((lease_token='' AND lease_expires_at IS NULL)
      OR (lease_token REGEXP '^[0-9a-f]{64}$' AND lease_expires_at IS NOT NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_prerequisite_artifact (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  requirement_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  artifact_kind VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  schema_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_payload MEDIUMBLOB NOT NULL,
  source_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  safe_summary VARCHAR(500) NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_artifact_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_artifact_digest (run_id,row_digest),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_artifact_run_id (run_id,id),
  CONSTRAINT fk_chain_event_reorg_prerequisite_artifact_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_prerequisite_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_prerequisite_artifact_fields CHECK (
    ordinal>0 AND OCTET_LENGTH(source_payload) BETWEEN 1 AND 1048576
    AND requirement_code REGEXP '^[A-Z0-9_]{3,64}$'
    AND artifact_kind REGEXP '^[a-z0-9_]{3,64}$'
    AND schema_version REGEXP '^[a-z0-9_.-]{3,64}$'
    AND source_kind IN ('database_snapshot','chain_attestation','external_attestation')
    AND source_digest REGEXP '^[0-9a-f]{64}$'
    AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
    AND CHAR_LENGTH(TRIM(safe_summary)) BETWEEN 10 AND 500
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_prerequisite_requirement (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  requirement_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  evidence_status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  artifact_id BIGINT UNSIGNED NULL,
  safe_detail VARCHAR(500) NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_requirement_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_requirement_code (run_id,requirement_code),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_requirement_digest (run_id,row_digest),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_requirement_artifact (run_id,artifact_id),
  CONSTRAINT fk_chain_event_reorg_prerequisite_requirement_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_prerequisite_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_prerequisite_requirement_artifact FOREIGN KEY (run_id,artifact_id)
    REFERENCES chain_event_reorg_prerequisite_artifact(run_id,id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_prerequisite_requirement_fields CHECK (
    ordinal BETWEEN 1 AND 7 AND evidence_status IN ('unmet','deferred') AND artifact_id IS NULL
    AND requirement_code REGEXP '^[A-Z0-9_]{3,64}$'
    AND CHAR_LENGTH(TRIM(safe_detail)) BETWEEN 10 AND 500
    AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_prerequisite_event (
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
  UNIQUE KEY uk_chain_event_reorg_prerequisite_event_sequence (run_id,sequence_no),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_event_hash (event_hash),
  CONSTRAINT fk_chain_event_reorg_prerequisite_event_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_prerequisite_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_prerequisite_event_fields CHECK (
    sequence_no>0 AND event_type IN ('CREATED','COMPLETED','INVALIDATED')
    AND ((event_type='INVALIDATED' AND CHAR_LENGTH(error_code)>0) OR (event_type<>'INVALIDATED' AND error_code=''))
    AND CHAR_LENGTH(TRIM(message)) BETWEEN 10 AND 500
    AND previous_event_hash REGEXP '^[0-9a-f]{64}$' AND event_hash REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_prerequisite_manifest (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  assessment_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  outcome VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  candidate_ready TINYINT UNSIGNED NOT NULL,
  requirement_count BIGINT UNSIGNED NOT NULL,
  satisfied_count BIGINT UNSIGNED NOT NULL,
  unmet_count BIGINT UNSIGNED NOT NULL,
  untrusted_count BIGINT UNSIGNED NOT NULL,
  deferred_count BIGINT UNSIGNED NOT NULL,
  artifact_count BIGINT UNSIGNED NOT NULL,
  requirement_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  artifact_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  completed_at DATETIME(6) NOT NULL,
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_manifest_digest (manifest_digest),
  CONSTRAINT fk_chain_event_reorg_prerequisite_manifest_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_prerequisite_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_prerequisite_manifest_fields CHECK (
    outcome='BLOCKED' AND candidate_ready=0 AND requirement_count=7 AND satisfied_count=0
    AND unmet_count=5 AND untrusted_count=0 AND deferred_count=2 AND artifact_count=0
    AND assessment_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND requirement_digest REGEXP '^[0-9a-f]{64}$'
    AND artifact_digest=REPEAT('0',64) AND event_digest REGEXP '^[0-9a-f]{64}$'
    AND manifest_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DELIMITER $$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_run_validate_insert BEFORE INSERT ON chain_event_reorg_prerequisite_run FOR EACH ROW
BEGIN
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite run id must be database assigned'; END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_incident i
      JOIN chain_event_reorg_assessment_run a ON a.incident_id=i.id
      JOIN chain_event_reorg_assessment_job j ON j.run_id=a.id AND j.incident_id=i.id
      JOIN chain_event_reorg_assessment_manifest m ON m.run_id=a.id AND m.incident_id=i.id
      WHERE i.id=NEW.incident_id AND i.active_slot=1 AND i.status='detected'
      AND a.id=NEW.assessment_run_id AND a.chain_id=NEW.chain_id
      AND a.boundary_block=NEW.boundary_block AND a.boundary_block_hash=NEW.boundary_block_hash
      AND j.state='completed' AND m.outcome='BLOCKED'
      AND m.manifest_digest=NEW.assessment_manifest_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite run source assessment identity mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_run_no_update BEFORE UPDATE ON chain_event_reorg_prerequisite_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite runs are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_run_no_delete BEFORE DELETE ON chain_event_reorg_prerequisite_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite runs are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_prerequisite_job_validate_insert BEFORE INSERT ON chain_event_reorg_prerequisite_job FOR EACH ROW
BEGIN
  IF NEW.state<>'preparing' OR NEW.active_slot IS NULL OR NEW.active_slot<>1 OR NEW.next_requirement_ordinal<>1
     OR NEW.requirement_count<>0 OR NEW.unmet_count<>0 OR NEW.untrusted_count<>0 OR NEW.deferred_count<>0
     OR NEW.artifact_count<>0 OR NEW.requirement_digest<>REPEAT('0',64) OR NEW.artifact_digest<>REPEAT('0',64)
     OR NEW.lease_token<>'' OR NEW.lease_expires_at IS NOT NULL OR NEW.fence_epoch<>0 OR NEW.attempt_count<>0
     OR NEW.last_error_code<>'' OR NEW.last_error_message<>'' OR NEW.completed_at IS NOT NULL
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_run r WHERE r.id=NEW.run_id AND r.incident_id=NEW.incident_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite job requires canonical initial state';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_job_validate_update BEFORE UPDATE ON chain_event_reorg_prerequisite_job FOR EACH ROW
BEGIN
  IF NEW.run_id<>OLD.run_id OR NEW.incident_id<>OLD.incident_id OR NEW.fence_epoch<OLD.fence_epoch
     OR NEW.attempt_count<OLD.attempt_count OR NEW.requirement_count<OLD.requirement_count
     OR NEW.next_requirement_ordinal<OLD.next_requirement_ordinal OR NEW.artifact_count<>0
     OR NEW.artifact_digest<>REPEAT('0',64) OR OLD.state<>'preparing' THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite job transition is not allowed';
  END IF;
  IF NEW.state='preparing' AND EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_event e
      WHERE e.run_id=NEW.run_id AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite terminal event forbids preparing job updates';
  END IF;
  IF NEW.state IN ('completed','invalidated') AND (
      OLD.active_slot IS NULL OR OLD.active_slot<>1 OR OLD.lease_token NOT REGEXP '^[0-9a-f]{64}$'
      OR OLD.lease_expires_at IS NULL OR OLD.lease_expires_at<=CURRENT_TIMESTAMP(6)
      OR NEW.active_slot IS NOT NULL OR NEW.lease_token<>'' OR NEW.lease_expires_at IS NOT NULL
      OR NEW.fence_epoch<>OLD.fence_epoch OR NEW.attempt_count<>OLD.attempt_count
      OR NEW.next_requirement_ordinal<>OLD.next_requirement_ordinal
      OR NEW.requirement_count<>OLD.requirement_count OR NEW.unmet_count<>OLD.unmet_count
      OR NEW.untrusted_count<>OLD.untrusted_count OR NEW.deferred_count<>OLD.deferred_count
      OR NEW.requirement_digest<>OLD.requirement_digest OR NEW.artifact_count<>OLD.artifact_count
      OR NEW.artifact_digest<>OLD.artifact_digest OR NEW.completed_at IS NULL) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite terminal transition requires the active fenced lease';
  END IF;
  IF NEW.state='completed' AND (
      NEW.last_error_code<>'' OR NEW.last_error_message<>'' OR NEW.requirement_count<>7
      OR NEW.unmet_count<>5 OR NEW.untrusted_count<>0 OR NEW.deferred_count<>2
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_event e
          WHERE e.run_id=NEW.run_id AND e.sequence_no=2 AND e.event_type='COMPLETED')
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_manifest m
          WHERE m.run_id=NEW.run_id AND m.outcome='BLOCKED' AND m.candidate_ready=0)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite completion requires terminal evidence and blocked manifest';
  END IF;
  IF NEW.state='invalidated' AND (
      CHAR_LENGTH(NEW.last_error_code)=0 OR CHAR_LENGTH(NEW.last_error_message)=0
      OR EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_manifest m WHERE m.run_id=NEW.run_id)
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_event e
          WHERE e.run_id=NEW.run_id AND e.sequence_no=2 AND e.event_type='INVALIDATED'
          AND e.error_code=NEW.last_error_code AND e.message=NEW.last_error_message)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite invalidation requires matching terminal evidence';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_job_no_delete BEFORE DELETE ON chain_event_reorg_prerequisite_job FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite jobs cannot be deleted'; END$$

CREATE TRIGGER trg_chain_event_reorg_prerequisite_artifact_validate_insert BEFORE INSERT ON chain_event_reorg_prerequisite_artifact FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='B1-A v1 forbids prerequisite artifacts'; END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_artifact_no_update BEFORE UPDATE ON chain_event_reorg_prerequisite_artifact FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite artifacts are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_artifact_no_delete BEFORE DELETE ON chain_event_reorg_prerequisite_artifact FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite artifacts are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_prerequisite_requirement_validate_insert BEFORE INSERT ON chain_event_reorg_prerequisite_requirement FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64);
  DECLARE expected_ordinal BIGINT UNSIGNED;
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite requirement id must be database assigned'; END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.active_slot=1 AND j.lease_token REGEXP '^[0-9a-f]{64}$'
      AND j.lease_expires_at>CURRENT_TIMESTAMP(6)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite requirement requires active DB lease';
  END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite terminal event forbids requirements';
  END IF;
  SET expected_previous=COALESCE((SELECT row_digest FROM chain_event_reorg_prerequisite_requirement
      WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),
      (SELECT requirement_digest FROM chain_event_reorg_prerequisite_job WHERE run_id=NEW.run_id));
  SET expected_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_prerequisite_requirement WHERE run_id=NEW.run_id),1);
  IF NEW.ordinal<>expected_ordinal OR NEW.previous_row_digest<>expected_previous OR NEW.artifact_id IS NOT NULL
     OR NEW.requirement_code<>CASE NEW.ordinal
       WHEN 1 THEN 'BOUNDARY_STATE_SNAPSHOT_REQUIRED'
       WHEN 2 THEN 'V1_SOURCE_ORIGIN_UNATTESTED'
       WHEN 3 THEN 'GLOBAL_CONFIG_BASELINE_REQUIRED'
       WHEN 4 THEN 'OUTBOX_GENERATION_BINDING_REQUIRED'
       WHEN 5 THEN 'GENERATION_CHECKPOINT_SCHEMA_REQUIRED'
       WHEN 6 THEN 'DUAL_CONTROL_AUTHORIZATION_REQUIRED'
       WHEN 7 THEN 'INDEPENDENT_CHAIN_PREFLIGHT_REQUIRED' END
     OR NEW.evidence_status<>CASE WHEN NEW.ordinal BETWEEN 1 AND 5 THEN 'unmet' ELSE 'deferred' END THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite requirement matrix is not approved';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-prerequisite-requirement.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.ordinal,CHAR(10),NEW.requirement_code,CHAR(10),NEW.evidence_status,CHAR(10),
      0,CHAR(10),NEW.safe_detail,CHAR(10),NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite requirement digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_requirement_no_update BEFORE UPDATE ON chain_event_reorg_prerequisite_requirement FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite requirements are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_requirement_no_delete BEFORE DELETE ON chain_event_reorg_prerequisite_requirement FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite requirements are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_prerequisite_event_validate_insert BEFORE INSERT ON chain_event_reorg_prerequisite_event FOR EACH ROW
BEGIN
  DECLARE prior_hash VARCHAR(64);
  DECLARE prior_sequence INT UNSIGNED;
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite event id must be database assigned'; END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite terminal event is final';
  END IF;
  SET prior_hash=COALESCE((SELECT event_hash FROM chain_event_reorg_prerequisite_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),REPEAT('0',64));
  SET prior_sequence=COALESCE((SELECT sequence_no FROM chain_event_reorg_prerequisite_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),0);
  IF NEW.sequence_no<>prior_sequence+1 OR NEW.previous_event_hash<>prior_hash
     OR (NEW.event_type='CREATED' AND (NEW.sequence_no<>1 OR NEW.previous_event_hash<>REPEAT('0',64)))
     OR (NEW.event_type IN ('COMPLETED','INVALIDATED') AND NEW.sequence_no<>2) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite event chain is not contiguous';
  END IF;
  IF NEW.event_type='CREATED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.fence_epoch=0 AND j.lease_token='') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite CREATED event requires canonical initial job';
  END IF;
  IF NEW.event_type<>'CREATED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite event requires active DB lease';
  END IF;
  IF NEW.event_type='COMPLETED' AND (
      EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_manifest m WHERE m.run_id=NEW.run_id)
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_job j WHERE j.run_id=NEW.run_id
          AND j.requirement_count=7 AND j.unmet_count=5 AND j.untrusted_count=0 AND j.deferred_count=2
          AND j.artifact_count=0 AND j.next_requirement_ordinal=8)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite completion event requires complete leased checkpoint';
  END IF;
  IF NEW.event_type='INVALIDATED' AND EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_manifest m WHERE m.run_id=NEW.run_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite manifest forbids invalidation';
  END IF;
  IF NEW.event_hash<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-prerequisite-event.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.sequence_no,CHAR(10),NEW.event_type,CHAR(10),NEW.error_code,CHAR(10),
      NEW.message,CHAR(10),NEW.previous_event_hash),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite event digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_event_no_update BEFORE UPDATE ON chain_event_reorg_prerequisite_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite events are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_event_no_delete BEFORE DELETE ON chain_event_reorg_prerequisite_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite events are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_prerequisite_manifest_validate_insert BEFORE INSERT ON chain_event_reorg_prerequisite_manifest FOR EACH ROW
BEGIN
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_job j
      JOIN chain_event_reorg_prerequisite_run r ON r.id=j.run_id AND r.incident_id=j.incident_id
      WHERE j.run_id=NEW.run_id AND j.incident_id=NEW.incident_id AND j.state='preparing' AND j.active_slot=1
      AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
      AND r.assessment_manifest_digest=NEW.assessment_manifest_digest
      AND j.requirement_count=NEW.requirement_count AND j.unmet_count=NEW.unmet_count
      AND j.untrusted_count=NEW.untrusted_count AND j.deferred_count=NEW.deferred_count
      AND j.artifact_count=NEW.artifact_count AND j.requirement_digest=NEW.requirement_digest
      AND j.artifact_digest=NEW.artifact_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite manifest requires complete leased checkpoint';
  END IF;
  IF (SELECT COUNT(*) FROM chain_event_reorg_prerequisite_requirement WHERE run_id=NEW.run_id)<>7
     OR (SELECT COUNT(*) FROM chain_event_reorg_prerequisite_requirement WHERE run_id=NEW.run_id AND evidence_status='unmet')<>5
     OR (SELECT COUNT(*) FROM chain_event_reorg_prerequisite_requirement WHERE run_id=NEW.run_id AND evidence_status='deferred')<>2
     OR EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_artifact WHERE run_id=NEW.run_id)
     OR (SELECT COUNT(*) FROM chain_event_reorg_prerequisite_event WHERE run_id=NEW.run_id
         AND sequence_no=2 AND event_type='COMPLETED' AND event_hash=NEW.event_digest)<>1
     OR (SELECT COUNT(*) FROM chain_event_reorg_prerequisite_event WHERE run_id=NEW.run_id)<>2
     OR COALESCE((SELECT row_digest FROM chain_event_reorg_prerequisite_requirement WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.requirement_digest THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite manifest evidence sets are incomplete';
  END IF;
  IF NEW.outcome<>'BLOCKED' OR NEW.candidate_ready<>0 OR NEW.satisfied_count<>0 OR NEW.artifact_count<>0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite v1 cannot make a candidate generation ready';
  END IF;
  IF NEW.manifest_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-prerequisite-manifest.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.incident_id,CHAR(10),NEW.assessment_manifest_digest,CHAR(10),NEW.outcome,CHAR(10),
      'false',CHAR(10),NEW.requirement_count,CHAR(10),NEW.satisfied_count,CHAR(10),NEW.unmet_count,CHAR(10),
      NEW.untrusted_count,CHAR(10),NEW.deferred_count,CHAR(10),NEW.artifact_count,CHAR(10),
      NEW.requirement_digest,CHAR(10),NEW.artifact_digest,CHAR(10),NEW.event_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite manifest digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_manifest_no_update BEFORE UPDATE ON chain_event_reorg_prerequisite_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite manifests are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_manifest_no_delete BEFORE DELETE ON chain_event_reorg_prerequisite_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite manifests are append-only'; END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS jc_assert_chain_event_reorg_prerequisite;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_reorg_prerequisite()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'
      AND TABLE_NAME IN ('chain_event_reorg_prerequisite_run','chain_event_reorg_prerequisite_job',
      'chain_event_reorg_prerequisite_artifact','chain_event_reorg_prerequisite_requirement',
      'chain_event_reorg_prerequisite_event','chain_event_reorg_prerequisite_manifest'))<>6 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite tables are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
      AND EVENT_OBJECT_TABLE IN ('chain_event_reorg_prerequisite_run','chain_event_reorg_prerequisite_job',
      'chain_event_reorg_prerequisite_artifact','chain_event_reorg_prerequisite_requirement',
      'chain_event_reorg_prerequisite_event','chain_event_reorg_prerequisite_manifest'))<>18 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite triggers are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES'
      AND TABLE_NAME IN ('chain_event_reorg_prerequisite_run','chain_event_reorg_prerequisite_job',
      'chain_event_reorg_prerequisite_artifact','chain_event_reorg_prerequisite_requirement',
      'chain_event_reorg_prerequisite_event','chain_event_reorg_prerequisite_manifest'))<>8 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite checks are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_NAME REGEXP '^fk_chain_event_reorg_prerequisite_'
      AND UPDATE_RULE='RESTRICT' AND DELETE_RULE='RESTRICT')<>9 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite foreign keys are incomplete';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_reorg_prerequisite();
DROP PROCEDURE jc_assert_chain_event_reorg_prerequisite;

-- Deployment runner records this file checksum and status=applied only after
-- every postflight assertion succeeds.
