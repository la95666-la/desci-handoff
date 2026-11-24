-- JoinCare IDO Sprint 1A-P3-A · durable deep-reorg detection evidence.
-- This migration deliberately does not auto-compensate financial facts. It
-- freezes a detected post-commit canonicality break and keeps all reconciliation
-- paths fail-closed until a separately audited compensation workflow resolves it.

SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_event_reorg_incident;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_event_reorg_incident()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('chain_event_sync_state','chain_event_record','chain_event_failure','fund_ledger','ido_purchase_entitlement'))<>5 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event reorg evidence prerequisites are missing';
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('chain_event_block_checkpoint','chain_event_reorg_incident','chain_event_reorg_event')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='pre-existing chain event reorg evidence tables require manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_event_reorg_incident();
DROP PROCEDURE jc_preflight_chain_event_reorg_incident;

CREATE TABLE chain_event_block_checkpoint (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  chain_id BIGINT UNSIGNED NOT NULL,
  block_number BIGINT UNSIGNED NOT NULL,
  block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_checkpoint_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  checkpoint_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_checkpoint_block (chain_id,block_number),
  UNIQUE KEY uk_chain_event_checkpoint_hash (checkpoint_hash),
  UNIQUE KEY uk_chain_event_checkpoint_parent (previous_checkpoint_hash),
  CONSTRAINT chk_chain_event_checkpoint_identity CHECK (
    chain_id>0 AND block_number>0 AND block_hash REGEXP '^0x[0-9a-f]{64}$'
  ) ENFORCED,
  CONSTRAINT chk_chain_event_checkpoint_digests CHECK (
    previous_checkpoint_hash REGEXP '^[0-9a-f]{64}$' AND checkpoint_hash REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_incident (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  incident_key VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  cursor_block BIGINT UNSIGNED NOT NULL,
  expected_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  observed_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  cursor_checkpoint_id BIGINT UNSIGNED NOT NULL,
  v2_start_block BIGINT UNSIGNED NOT NULL,
  status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  active_slot TINYINT UNSIGNED NULL,
  detection_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  detected_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  resolved_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_incident_key (incident_key),
  UNIQUE KEY uk_chain_event_reorg_active (active_slot),
  KEY idx_chain_event_reorg_status_time (status,detected_at,id),
  KEY idx_chain_event_reorg_checkpoint (cursor_checkpoint_id),
  CONSTRAINT fk_chain_event_reorg_checkpoint FOREIGN KEY (cursor_checkpoint_id)
    REFERENCES chain_event_block_checkpoint(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_identity CHECK (
    chain_id>0 AND cursor_block>0 AND v2_start_block>0
    AND contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND contract_address<>'0x0000000000000000000000000000000000000000'
    AND contract_version='v2_stablecoin'
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_hashes CHECK (
    incident_key REGEXP '^[0-9a-f]{64}$'
    AND expected_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND observed_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND expected_block_hash<>observed_block_hash
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_state CHECK (
    status='detected' AND active_slot=1 AND detection_code='CURSOR_BLOCK_HASH_MISMATCH' AND resolved_at IS NULL
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_event (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  incident_id BIGINT UNSIGNED NOT NULL,
  sequence_no BIGINT UNSIGNED NOT NULL,
  event_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  message VARCHAR(500) NOT NULL,
  previous_event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_event_sequence (incident_id,sequence_no),
  UNIQUE KEY uk_chain_event_reorg_event_hash (event_hash),
  KEY idx_chain_event_reorg_event_incident (incident_id),
  CONSTRAINT fk_chain_event_reorg_event_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_event_fields CHECK (
    incident_id>0 AND sequence_no>0 AND event_type='DETECTED'
    AND CHAR_LENGTH(TRIM(message)) BETWEEN 3 AND 500
    AND previous_event_hash REGEXP '^[0-9a-f]{64}$' AND event_hash REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DROP TRIGGER IF EXISTS trg_chain_event_checkpoint_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_checkpoint_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_checkpoint_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_incident_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_incident_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_incident_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_event_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_event_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_event_no_delete;
DELIMITER $$
CREATE TRIGGER trg_chain_event_checkpoint_validate_insert BEFORE INSERT ON chain_event_block_checkpoint FOR EACH ROW
BEGIN
  DECLARE prior_hash VARCHAR(64);
  DECLARE prior_block BIGINT UNSIGNED;
  DECLARE prior_chain BIGINT UNSIGNED;
  SET prior_hash=COALESCE((SELECT checkpoint_hash FROM chain_event_block_checkpoint ORDER BY id DESC LIMIT 1),REPEAT('0',64));
  SET prior_block=COALESCE((SELECT block_number FROM chain_event_block_checkpoint ORDER BY id DESC LIMIT 1),0);
  SET prior_chain=COALESCE((SELECT chain_id FROM chain_event_block_checkpoint ORDER BY id DESC LIMIT 1),NEW.chain_id);
  IF NEW.previous_checkpoint_hash<>prior_hash THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event checkpoint chain is not contiguous';
  END IF;
  IF NEW.chain_id<>prior_chain OR NEW.block_number<=prior_block THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event checkpoint cursor is not strictly monotonic';
  END IF;
  IF NEW.checkpoint_hash<>LOWER(SHA2(CONCAT(
    'joincare.chain-event.checkpoint.v1',CHAR(10),NEW.chain_id,CHAR(10),NEW.block_number,CHAR(10),
    NEW.block_hash,CHAR(10),NEW.previous_checkpoint_hash
  ),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event checkpoint digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_checkpoint_no_update BEFORE UPDATE ON chain_event_block_checkpoint FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event checkpoint rows are append-only'; END$$
CREATE TRIGGER trg_chain_event_checkpoint_no_delete BEFORE DELETE ON chain_event_block_checkpoint FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event checkpoint rows are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_incident_validate_insert BEFORE INSERT ON chain_event_reorg_incident FOR EACH ROW
BEGIN
  IF NOT EXISTS (SELECT 1 FROM chain_event_block_checkpoint c WHERE c.id=NEW.cursor_checkpoint_id
      AND c.chain_id=NEW.chain_id AND c.block_number=NEW.cursor_block AND c.block_hash=NEW.expected_block_hash) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg incident cursor checkpoint identity mismatch';
  END IF;
  IF NEW.incident_key<>LOWER(SHA2(CONCAT(
    'joincare.chain-event.reorg-incident.v1',CHAR(10),NEW.chain_id,CHAR(10),NEW.contract_address,CHAR(10),
    NEW.cursor_block,CHAR(10),NEW.expected_block_hash,CHAR(10),NEW.observed_block_hash
  ),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg incident digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_incident_no_update BEFORE UPDATE ON chain_event_reorg_incident FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg incident rows are append-only until compensation migration'; END$$
CREATE TRIGGER trg_chain_event_reorg_incident_no_delete BEFORE DELETE ON chain_event_reorg_incident FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg incident rows are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_event_validate_insert BEFORE INSERT ON chain_event_reorg_event FOR EACH ROW
BEGIN
  IF NEW.sequence_no<>1 OR NEW.previous_event_hash<>REPEAT('0',64)
     OR EXISTS (SELECT 1 FROM chain_event_reorg_event WHERE incident_id=NEW.incident_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg incident event chain is not canonical';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_incident i WHERE i.id=NEW.incident_id AND i.status='detected' AND i.active_slot=1) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg incident event requires active incident';
  END IF;
  IF NEW.event_hash<>LOWER(SHA2(CONCAT(
    'joincare.chain-event.reorg-event.v1',CHAR(10),NEW.incident_id,CHAR(10),NEW.sequence_no,CHAR(10),
    NEW.event_type,CHAR(10),NEW.message,CHAR(10),NEW.previous_event_hash
  ),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg incident event digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_event_no_update BEFORE UPDATE ON chain_event_reorg_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg incident event rows are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_event_no_delete BEFORE DELETE ON chain_event_reorg_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg incident event rows are append-only'; END$$
DELIMITER ;

INSERT INTO chain_event_block_checkpoint (
  chain_id,block_number,block_hash,previous_checkpoint_hash,checkpoint_hash,created_at
)
SELECT p.chain_id,s.cursor_block,LOWER(s.cursor_block_hash),REPEAT('0',64),LOWER(SHA2(CONCAT(
  'joincare.chain-event.checkpoint.v1',CHAR(10),p.chain_id,CHAR(10),s.cursor_block,CHAR(10),
  LOWER(s.cursor_block_hash),CHAR(10),REPEAT('0',64)
),256)),CURRENT_TIMESTAMP(6)
FROM chain_event_sync_state s
JOIN (SELECT chain_id FROM projects WHERE contract_version='v2_stablecoin' GROUP BY chain_id) p ON 1=1
WHERE s.id=1 AND s.initialized=1 AND s.v2_registered=1 AND s.cursor_block>0
  AND LOWER(s.cursor_block_hash) REGEXP '^0x[0-9a-f]{64}$'
  AND (SELECT COUNT(DISTINCT chain_id) FROM projects WHERE contract_version='v2_stablecoin')=1;

DROP PROCEDURE IF EXISTS jc_assert_chain_event_reorg_incident;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_reorg_incident()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('chain_event_block_checkpoint','chain_event_reorg_incident','chain_event_reorg_event') AND TABLE_TYPE='BASE TABLE')<>3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event reorg evidence tables are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM (
      SELECT TABLE_NAME,COUNT(*) column_count FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('chain_event_block_checkpoint','chain_event_reorg_incident','chain_event_reorg_event') GROUP BY TABLE_NAME
    ) c WHERE (TABLE_NAME='chain_event_block_checkpoint' AND column_count=7)
      OR (TABLE_NAME='chain_event_reorg_incident' AND column_count=17)
      OR (TABLE_NAME='chain_event_reorg_event' AND column_count=8))<>3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event reorg evidence columns are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM (
      SELECT TABLE_NAME,INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('chain_event_block_checkpoint','chain_event_reorg_incident','chain_event_reorg_event') GROUP BY TABLE_NAME,INDEX_NAME
    ) i WHERE INDEX_NAME IN (
      'PRIMARY','uk_chain_event_checkpoint_block','uk_chain_event_checkpoint_hash','uk_chain_event_checkpoint_parent',
      'uk_chain_event_reorg_incident_key','uk_chain_event_reorg_active','idx_chain_event_reorg_status_time','idx_chain_event_reorg_checkpoint',
      'uk_chain_event_reorg_event_sequence','uk_chain_event_reorg_event_hash','idx_chain_event_reorg_event_incident'
    ))<>13 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event reorg evidence indexes are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
      AND TRIGGER_NAME IN ('trg_chain_event_checkpoint_validate_insert','trg_chain_event_checkpoint_no_update','trg_chain_event_checkpoint_no_delete',
      'trg_chain_event_reorg_incident_validate_insert','trg_chain_event_reorg_incident_no_update','trg_chain_event_reorg_incident_no_delete',
      'trg_chain_event_reorg_event_validate_insert','trg_chain_event_reorg_event_no_update','trg_chain_event_reorg_event_no_delete'))<>9 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event reorg evidence triggers are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_NAME IN ('fk_chain_event_reorg_checkpoint','fk_chain_event_reorg_event_incident') AND DELETE_RULE='RESTRICT')<>2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event reorg evidence foreign keys are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES' AND CONSTRAINT_NAME IN (
      'chk_chain_event_checkpoint_identity','chk_chain_event_checkpoint_digests','chk_chain_event_reorg_identity',
      'chk_chain_event_reorg_hashes','chk_chain_event_reorg_state','chk_chain_event_reorg_event_fields'))<>6 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event reorg evidence checks are incomplete';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_reorg_incident();
DROP PROCEDURE jc_assert_chain_event_reorg_incident;
