-- JoinCare Fair Launch BSC mainnet financial materialization authority.
-- ADDITIVE ONLY. Creates no profile/cursor/receipt, starts no job and does not
-- enable wallet submission or the historical global materialization switch.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_require_fair_launch_mainnet_materialization_0918;
DELIMITER $$
CREATE PROCEDURE jc_require_fair_launch_mainnet_materialization_0918()
BEGIN
  IF COALESCE(@JC_FAIR_LAUNCH_MAINNET_MATERIALIZATION_APPROVED,0)<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet materialization migration requires approved execution gate';
  END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_mainnet_recovery_provider_v1') OR
     NOT EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_mainnet_archive_batch_event_v1') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet materialization migration requires 0915-0917';
  END IF;
  IF EXISTS(SELECT 1 FROM fair_launch_mainnet_archive_batch_event_v1 LIMIT 1) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet materialization migration requires an empty archive event set';
  END IF;
END$$
DELIMITER ;
CALL jc_require_fair_launch_mainnet_materialization_0918();
DROP PROCEDURE jc_require_fair_launch_mainnet_materialization_0918;

-- Durable raw ABI payload needed to reconstruct and independently decode an
-- archived event after a process restart. The observation digests remain the
-- CE-V2 authority; the insert trigger recomputes both digests from these bytes.
CREATE TABLE IF NOT EXISTS fair_launch_mainnet_archive_event_payload_v1 (
  archive_event_id BIGINT UNSIGNED NOT NULL,
  activation_row_id BIGINT UNSIGNED NOT NULL,
  event_identity_id BIGINT UNSIGNED NOT NULL,
  placement_id BIGINT UNSIGNED NOT NULL,
  observation_id BIGINT UNSIGNED NOT NULL,
  topic_count TINYINT UNSIGNED NOT NULL,
  topic0 CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  topic1 CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL,
  topic2 CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL,
  topic3 CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL,
  data_hex LONGTEXT CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  payload_row_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(archive_event_id),
  UNIQUE KEY uk_fl_mainnet_archive_payload_evidence(activation_row_id,event_identity_id,placement_id,observation_id),
  CONSTRAINT fk_fl_mainnet_archive_payload_event FOREIGN KEY(archive_event_id) REFERENCES fair_launch_mainnet_archive_batch_event_v1(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mainnet_archive_payload CHECK(
    topic_count BETWEEN 1 AND 4 AND topic0 REGEXP '^0x[0-9a-f]{64}$' AND
    (topic1 IS NULL OR topic1 REGEXP '^0x[0-9a-f]{64}$') AND
    (topic2 IS NULL OR topic2 REGEXP '^0x[0-9a-f]{64}$') AND
    (topic3 IS NULL OR topic3 REGEXP '^0x[0-9a-f]{64}$') AND
    ((topic_count=1 AND topic1 IS NULL AND topic2 IS NULL AND topic3 IS NULL) OR
     (topic_count=2 AND topic1 IS NOT NULL AND topic2 IS NULL AND topic3 IS NULL) OR
     (topic_count=3 AND topic1 IS NOT NULL AND topic2 IS NOT NULL AND topic3 IS NULL) OR
     (topic_count=4 AND topic1 IS NOT NULL AND topic2 IS NOT NULL AND topic3 IS NOT NULL)) AND
    data_hex REGEXP '^0x([0-9a-f]{2})*$' AND CHAR_LENGTH(data_hex)<=131074 AND
    payload_row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Row existence is the mainnet-only runtime admission. It is immutable and
-- independent from the historical LOCAL-only compile-time switch.
CREATE TABLE IF NOT EXISTS fair_launch_mainnet_materialization_profile_v1 (
  activation_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  source_binding_id BIGINT UNSIGNED NOT NULL,
  bootstrap_receipt_id BIGINT UNSIGNED NOT NULL,
  bootstrap_archive_event_id BIGINT UNSIGNED NOT NULL,
  initial_event_identity_id BIGINT UNSIGNED NOT NULL,
  effect_manifest_version INT UNSIGNED NOT NULL,
  effect_manifest_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  initial_block_number BIGINT UNSIGNED NOT NULL,
  initial_block_hash CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  initial_transaction_index INT UNSIGNED NOT NULL,
  initial_tx_log_ordinal INT UNSIGNED NOT NULL,
  profile_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  approval_evidence_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  valid_from DATETIME(6) NOT NULL,
  valid_until DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(activation_row_id),
  UNIQUE KEY uk_fl_mainnet_materialization_profile_source(activation_row_id,chain_id,source_id,source_revision_id,source_binding_id),
  UNIQUE KEY uk_fl_mainnet_materialization_profile_bootstrap(bootstrap_receipt_id,bootstrap_archive_event_id,initial_event_identity_id),
  CONSTRAINT fk_fl_mainnet_materialization_profile_activation FOREIGN KEY(activation_row_id,chain_id,source_id,source_revision_id) REFERENCES fair_launch_mainnet_activation_v1(authority_row_id,chain_id,source_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_materialization_profile_binding FOREIGN KEY(source_binding_id,activation_row_id,chain_id,source_id,source_revision_id) REFERENCES fair_launch_source_revision_binding_v3(id,run_row_id,chain_id,source_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_materialization_profile_bootstrap FOREIGN KEY(bootstrap_receipt_id) REFERENCES fair_launch_mainnet_project_bootstrap_receipt_v1(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_materialization_profile_archive_event FOREIGN KEY(bootstrap_archive_event_id) REFERENCES fair_launch_mainnet_archive_batch_event_v1(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_materialization_profile_initial_event FOREIGN KEY(initial_event_identity_id,chain_id) REFERENCES chain_event_identity_v2(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mainnet_materialization_profile CHECK(
    chain_id=56 AND effect_manifest_version=1 AND
    effect_manifest_digest REGEXP '^[0-9a-f]{64}$' AND initial_block_hash REGEXP '^0x[0-9a-f]{64}$' AND
    profile_digest REGEXP '^[0-9a-f]{64}$' AND approval_evidence_digest REGEXP '^[0-9a-f]{64}$' AND
    created_at<=valid_from AND valid_from<valid_until
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_mainnet_materialization_cursor_v1 (
  activation_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  source_binding_id BIGINT UNSIGNED NOT NULL,
  materialized_through_block BIGINT UNSIGNED NOT NULL,
  materialized_through_block_hash CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  materialized_through_tx_index INT UNSIGNED NOT NULL,
  materialized_through_tx_log_ordinal INT UNSIGNED NOT NULL,
  last_event_identity_id BIGINT UNSIGNED NULL,
  last_archive_event_id BIGINT UNSIGNED NULL,
  last_materialization_receipt_id BIGINT UNSIGNED NULL,
  lease_generation BIGINT UNSIGNED NOT NULL,
  fence_epoch BIGINT UNSIGNED NOT NULL,
  lease_token_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  cursor_generation BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY(activation_row_id),
  UNIQUE KEY uk_fl_mainnet_materialization_cursor_source(activation_row_id,chain_id,source_id,source_revision_id,source_binding_id),
  CONSTRAINT fk_fl_mainnet_materialization_cursor_profile FOREIGN KEY(activation_row_id,chain_id,source_id,source_revision_id,source_binding_id) REFERENCES fair_launch_mainnet_materialization_profile_v1(activation_row_id,chain_id,source_id,source_revision_id,source_binding_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_materialization_cursor_event FOREIGN KEY(last_event_identity_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_materialization_cursor_archive_event FOREIGN KEY(last_archive_event_id) REFERENCES fair_launch_mainnet_archive_batch_event_v1(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mainnet_materialization_cursor CHECK(
    chain_id=56 AND materialized_through_block_hash REGEXP '^0x[0-9a-f]{64}$' AND
    lease_generation>0 AND fence_epoch>0 AND lease_token_hash REGEXP '^[0-9a-f]{64}$' AND
    status IN('active','blocked','revoked') AND cursor_generation>0 AND
    ((last_event_identity_id IS NOT NULL AND last_archive_event_id IS NOT NULL AND last_materialization_receipt_id IS NULL) OR
     (last_event_identity_id IS NOT NULL AND last_archive_event_id IS NOT NULL AND last_materialization_receipt_id IS NOT NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_mainnet_materialization_receipt_v1 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  activation_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  source_binding_id BIGINT UNSIGNED NOT NULL,
  archive_batch_receipt_id BIGINT UNSIGNED NOT NULL,
  archive_event_id BIGINT UNSIGNED NOT NULL,
  event_identity_id BIGINT UNSIGNED NOT NULL,
  placement_id BIGINT UNSIGNED NOT NULL,
  observation_id BIGINT UNSIGNED NOT NULL,
  application_attempt_id BIGINT UNSIGNED NOT NULL,
  source_application_lineage_id BIGINT UNSIGNED NOT NULL,
  attestation_id BIGINT UNSIGNED NOT NULL,
  effect_manifest_version INT UNSIGNED NOT NULL,
  payload_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  effect_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  ledger_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  mutation_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  attestation_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  lease_generation BIGINT UNSIGNED NOT NULL,
  fence_epoch BIGINT UNSIGNED NOT NULL,
  previous_cursor_generation BIGINT UNSIGNED NOT NULL,
  previous_receipt_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  receipt_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_mainnet_materialization_archive_event(archive_event_id),
  UNIQUE KEY uk_fl_mainnet_materialization_application(application_attempt_id),
  UNIQUE KEY uk_fl_mainnet_materialization_lineage(source_application_lineage_id),
  UNIQUE KEY uk_fl_mainnet_materialization_receipt(activation_row_id,receipt_digest),
  UNIQUE KEY uk_fl_mainnet_materialization_receipt_identity(id,activation_row_id),
  CONSTRAINT fk_fl_mainnet_materialization_receipt_profile FOREIGN KEY(activation_row_id,chain_id,source_id,source_revision_id,source_binding_id) REFERENCES fair_launch_mainnet_materialization_profile_v1(activation_row_id,chain_id,source_id,source_revision_id,source_binding_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_materialization_receipt_batch FOREIGN KEY(archive_batch_receipt_id,activation_row_id) REFERENCES fair_launch_mainnet_archive_batch_receipt_v1(id,activation_row_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_materialization_receipt_archive_event FOREIGN KEY(archive_event_id) REFERENCES fair_launch_mainnet_archive_batch_event_v1(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_materialization_receipt_event FOREIGN KEY(event_identity_id,chain_id) REFERENCES chain_event_identity_v2(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_materialization_receipt_placement FOREIGN KEY(placement_id,event_identity_id,source_revision_id) REFERENCES chain_event_placement_v2(id,event_identity_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_materialization_receipt_observation FOREIGN KEY(observation_id,event_identity_id,placement_id) REFERENCES chain_event_placement_observation_v2(id,event_identity_id,placement_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_materialization_receipt_application FOREIGN KEY(application_attempt_id,event_identity_id,placement_id,effect_manifest_version) REFERENCES chain_event_application_attempt_v2(id,event_identity_id,placement_id,effect_version) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_materialization_receipt_lineage FOREIGN KEY(source_application_lineage_id,activation_row_id,chain_id,source_binding_id,source_id,event_identity_id,placement_id,source_revision_id,effect_manifest_version) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_binding_id,source_id,event_identity_id,placement_id,source_revision_id,effect_manifest_version) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_materialization_receipt_attestation FOREIGN KEY(attestation_id) REFERENCES fair_launch_effect_attestation_v3(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mainnet_materialization_receipt CHECK(
    chain_id=56 AND effect_manifest_version=1 AND lease_generation>0 AND fence_epoch>0 AND previous_cursor_generation>0 AND
    payload_digest REGEXP '^[0-9a-f]{64}$' AND effect_digest REGEXP '^[0-9a-f]{64}$' AND ledger_digest REGEXP '^[0-9a-f]{64}$' AND
    mutation_digest REGEXP '^[0-9a-f]{64}$' AND attestation_digest REGEXP '^[0-9a-f]{64}$' AND
    (previous_receipt_digest IS NULL OR previous_receipt_digest REGEXP '^[0-9a-f]{64}$') AND receipt_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE fair_launch_mainnet_materialization_cursor_v1
  ADD CONSTRAINT fk_fl_mainnet_materialization_cursor_last_receipt FOREIGN KEY(last_materialization_receipt_id,activation_row_id) REFERENCES fair_launch_mainnet_materialization_receipt_v1(id,activation_row_id) ON UPDATE RESTRICT ON DELETE RESTRICT;

DROP TRIGGER IF EXISTS trg_fl_mainnet_archive_payload_validate_insert;
DROP TRIGGER IF EXISTS trg_fl_mainnet_archive_payload_no_update;
DROP TRIGGER IF EXISTS trg_fl_mainnet_archive_payload_no_delete;
DROP TRIGGER IF EXISTS trg_fl_mainnet_archive_cursor_payload_gate;
DROP TRIGGER IF EXISTS trg_fl_mainnet_materialization_profile_validate_insert;
DROP TRIGGER IF EXISTS trg_fl_mainnet_materialization_profile_no_update;
DROP TRIGGER IF EXISTS trg_fl_mainnet_materialization_profile_no_delete;
DROP TRIGGER IF EXISTS trg_fl_mainnet_materialization_cursor_validate_insert;
DROP TRIGGER IF EXISTS trg_fl_mainnet_materialization_cursor_validate_update;
DROP TRIGGER IF EXISTS trg_fl_mainnet_materialization_cursor_no_delete;
DROP TRIGGER IF EXISTS trg_fl_mainnet_materialization_receipt_validate_insert;
DROP TRIGGER IF EXISTS trg_fl_mainnet_materialization_receipt_no_update;
DROP TRIGGER IF EXISTS trg_fl_mainnet_materialization_receipt_no_delete;
DELIMITER $$
CREATE TRIGGER trg_fl_mainnet_archive_payload_validate_insert BEFORE INSERT ON fair_launch_mainnet_archive_event_payload_v1 FOR EACH ROW BEGIN
  DECLARE v_topics_hex LONGTEXT;
  DECLARE v_topics_digest CHAR(64);
  DECLARE v_raw_digest CHAR(64);
  DECLARE v_emitting_address CHAR(42);
  IF NOT EXISTS(
    SELECT 1 FROM fair_launch_mainnet_archive_batch_event_v1 e
    JOIN fair_launch_mainnet_archive_batch_receipt_v1 b ON b.id=e.batch_receipt_id AND b.activation_row_id=e.activation_row_id
    JOIN chain_event_placement_observation_v2 o ON o.id=e.observation_id AND o.event_identity_id=e.event_identity_id AND o.placement_id=e.placement_id
    WHERE e.id=NEW.archive_event_id AND e.activation_row_id=NEW.activation_row_id AND e.event_identity_id=NEW.event_identity_id AND
      e.placement_id=NEW.placement_id AND e.observation_id=NEW.observation_id AND b.chain_id=56
  ) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive payload evidence mismatch'; END IF;
  SELECT o.emitting_address INTO v_emitting_address FROM chain_event_placement_observation_v2 o WHERE o.id=NEW.observation_id;
  SET v_topics_hex=CONCAT(SUBSTRING(NEW.topic0,3),IF(NEW.topic_count>=2,SUBSTRING(NEW.topic1,3),''),IF(NEW.topic_count>=3,SUBSTRING(NEW.topic2,3),''),IF(NEW.topic_count>=4,SUBSTRING(NEW.topic3,3),''),SUBSTRING(NEW.data_hex,3));
  SET v_topics_digest=LOWER(SHA2(UNHEX(v_topics_hex),256));
  SET v_raw_digest=LOWER(SHA2(UNHEX(CONCAT(SUBSTRING(v_emitting_address,3),v_topics_hex)),256));
  IF NOT EXISTS(SELECT 1 FROM chain_event_placement_observation_v2 o WHERE o.id=NEW.observation_id AND o.topics_data_digest=v_topics_digest AND o.raw_log_digest=v_raw_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive payload digest mismatch';
  END IF;
  IF NEW.payload_row_digest<>LOWER(SHA2(CONCAT('joincare:fair-launch:mainnet-archive-payload:v1',CHAR(0),NEW.archive_event_id,CHAR(0),NEW.activation_row_id,CHAR(0),NEW.event_identity_id,CHAR(0),NEW.placement_id,CHAR(0),NEW.observation_id,CHAR(0),NEW.topic_count,CHAR(0),NEW.topic0,CHAR(0),COALESCE(NEW.topic1,''),CHAR(0),COALESCE(NEW.topic2,''),CHAR(0),COALESCE(NEW.topic3,''),CHAR(0),NEW.data_hex),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive payload row digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_fl_mainnet_archive_payload_no_update BEFORE UPDATE ON fair_launch_mainnet_archive_event_payload_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_archive_event_payload_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_archive_payload_no_delete BEFORE DELETE ON fair_launch_mainnet_archive_event_payload_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_archive_event_payload_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_archive_cursor_payload_gate BEFORE UPDATE ON fair_launch_mainnet_archive_cursor_v1 FOR EACH ROW BEGIN
  IF OLD.status='active' AND NEW.status='active' AND NEW.archived_through_block>OLD.archived_through_block AND
     NOT EXISTS(
       SELECT 1 FROM fair_launch_mainnet_archive_batch_receipt_v1 b
       WHERE b.id=NEW.last_batch_receipt_id AND b.activation_row_id=NEW.activation_row_id AND
         (SELECT COUNT(*) FROM fair_launch_mainnet_archive_batch_event_v1 e WHERE e.batch_receipt_id=b.id)=
         (SELECT COUNT(*) FROM fair_launch_mainnet_archive_batch_event_v1 e JOIN fair_launch_mainnet_archive_event_payload_v1 p ON p.archive_event_id=e.id WHERE e.batch_receipt_id=b.id)
     ) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive cursor payload set mismatch'; END IF;
END$$
CREATE TRIGGER trg_fl_mainnet_materialization_profile_validate_insert BEFORE INSERT ON fair_launch_mainnet_materialization_profile_v1 FOR EACH ROW BEGIN
  IF NOT EXISTS(
    SELECT 1 FROM fair_launch_mainnet_activation_v1 a
    JOIN fair_launch_source_revision_binding_v3 s ON s.id=NEW.source_binding_id AND s.run_row_id=a.authority_row_id AND s.chain_id=a.chain_id AND s.source_id=a.source_id AND s.source_revision_id=a.source_revision_id
    JOIN fair_launch_mainnet_project_bootstrap_receipt_v1 bootstrap ON bootstrap.id=NEW.bootstrap_receipt_id AND bootstrap.activation_row_id=a.authority_row_id AND bootstrap.chain_id=a.chain_id AND bootstrap.source_id=a.source_id AND bootstrap.source_revision_id=a.source_revision_id
    JOIN fair_launch_mainnet_archive_batch_event_v1 archive_event ON archive_event.id=NEW.bootstrap_archive_event_id AND archive_event.activation_row_id=a.authority_row_id AND archive_event.event_identity_id=bootstrap.event_identity_id AND archive_event.placement_id=bootstrap.placement_id AND archive_event.observation_id=bootstrap.observation_id
    JOIN fair_launch_mainnet_archive_event_payload_v1 raw_payload ON raw_payload.archive_event_id=archive_event.id
    JOIN chain_event_identity_v2 identity_row ON identity_row.id=bootstrap.event_identity_id AND identity_row.chain_id=a.chain_id
    JOIN chain_event_placement_v2 placement ON placement.id=bootstrap.placement_id AND placement.event_identity_id=identity_row.id AND placement.source_revision_id=a.source_revision_id
    JOIN fair_launch_mainnet_activation_lease_v1 l ON l.activation_row_id=a.authority_row_id AND l.chain_id=a.chain_id
    WHERE a.authority_row_id=NEW.activation_row_id AND a.chain_id=NEW.chain_id AND a.source_id=NEW.source_id AND a.source_revision_id=NEW.source_revision_id AND
      archive_event.event_name='FairLaunchProjectCreated' AND archive_event.payload_digest=bootstrap.payload_digest AND NEW.initial_event_identity_id=identity_row.id AND
      NEW.initial_block_number=placement.block_number AND NEW.initial_block_hash=placement.block_hash AND NEW.initial_transaction_index=placement.transaction_index AND NEW.initial_tx_log_ordinal=identity_row.tx_log_ordinal AND
      placement.canonical_slot=1 AND placement.finalized_at IS NOT NULL AND placement.noncanonical_at IS NULL AND l.status='active' AND l.active_slot=1 AND l.valid_from<=NEW.created_at AND l.valid_until>NEW.created_at AND
      (SELECT h.state FROM fair_launch_mainnet_activation_history_v1 h WHERE h.activation_row_id=a.authority_row_id ORDER BY h.sequence_no DESC LIMIT 1) IN('source_activated','wallet_enabled') AND
      EXISTS(SELECT 1 FROM fair_launch_mainnet_component_binding_v1 c WHERE c.activation_row_id=a.authority_row_id AND c.chain_id=a.chain_id AND c.component='job')
  ) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet materialization profile authority mismatch'; END IF;
END$$
CREATE TRIGGER trg_fl_mainnet_materialization_profile_no_update BEFORE UPDATE ON fair_launch_mainnet_materialization_profile_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_materialization_profile_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_materialization_profile_no_delete BEFORE DELETE ON fair_launch_mainnet_materialization_profile_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_materialization_profile_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_materialization_cursor_validate_insert BEFORE INSERT ON fair_launch_mainnet_materialization_cursor_v1 FOR EACH ROW BEGIN
  IF NEW.status<>'active' OR NEW.cursor_generation<>1 OR NEW.last_event_identity_id IS NULL OR NEW.last_archive_event_id IS NULL OR NEW.last_materialization_receipt_id IS NOT NULL OR
     NOT EXISTS(
       SELECT 1 FROM fair_launch_mainnet_materialization_profile_v1 p
       JOIN fair_launch_mainnet_activation_lease_v1 l ON l.activation_row_id=p.activation_row_id AND l.chain_id=p.chain_id
       JOIN source_materialization_cursor c ON c.source_id=p.source_id AND c.source_revision_id=p.source_revision_id
       WHERE p.activation_row_id=NEW.activation_row_id AND p.chain_id=NEW.chain_id AND p.source_id=NEW.source_id AND p.source_revision_id=NEW.source_revision_id AND p.source_binding_id=NEW.source_binding_id AND
         p.initial_block_number=NEW.materialized_through_block AND p.initial_block_hash=NEW.materialized_through_block_hash AND p.initial_transaction_index=NEW.materialized_through_tx_index AND p.initial_tx_log_ordinal=NEW.materialized_through_tx_log_ordinal AND p.initial_event_identity_id=NEW.last_event_identity_id AND p.bootstrap_archive_event_id=NEW.last_archive_event_id AND p.valid_from<=NEW.created_at AND p.valid_until>NEW.created_at AND
         l.status='active' AND l.active_slot=1 AND l.lease_generation=NEW.lease_generation AND l.fence_epoch=NEW.fence_epoch AND l.lease_token_hash=NEW.lease_token_hash AND
         c.materialized_through_block=NEW.materialized_through_block AND c.materialized_through_block_hash=NEW.materialized_through_block_hash AND c.materialized_through_tx_index=NEW.materialized_through_tx_index AND c.materialized_through_tx_log_ordinal=NEW.materialized_through_tx_log_ordinal AND
         c.last_event_identity_id=NEW.last_event_identity_id AND c.lease_token=NEW.lease_token_hash AND c.fence_epoch=NEW.fence_epoch AND c.status='active' AND c.last_error=''
     ) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet materialization cursor admission mismatch'; END IF;
END$$
CREATE TRIGGER trg_fl_mainnet_materialization_receipt_validate_insert BEFORE INSERT ON fair_launch_mainnet_materialization_receipt_v1 FOR EACH ROW BEGIN
  IF NOT EXISTS(
    SELECT 1 FROM fair_launch_mainnet_materialization_cursor_v1 c
    JOIN fair_launch_mainnet_materialization_profile_v1 profile ON profile.activation_row_id=c.activation_row_id
    JOIN fair_launch_mainnet_activation_lease_v1 lease_row ON lease_row.activation_row_id=c.activation_row_id AND lease_row.chain_id=c.chain_id
    JOIN fair_launch_mainnet_archive_cursor_v1 archive_cursor ON archive_cursor.activation_row_id=c.activation_row_id AND archive_cursor.chain_id=c.chain_id AND archive_cursor.source_id=c.source_id AND archive_cursor.source_revision_id=c.source_revision_id
    JOIN fair_launch_mainnet_archive_batch_event_v1 e ON e.id=NEW.archive_event_id AND e.activation_row_id=c.activation_row_id
    JOIN fair_launch_mainnet_archive_batch_receipt_v1 b ON b.id=e.batch_receipt_id AND b.activation_row_id=e.activation_row_id
    JOIN fair_launch_mainnet_archive_event_payload_v1 raw_payload ON raw_payload.archive_event_id=e.id
    JOIN chain_event_identity_v2 i ON i.id=e.event_identity_id AND i.chain_id=c.chain_id
    JOIN chain_event_placement_v2 placement ON placement.id=e.placement_id AND placement.event_identity_id=i.id AND placement.source_revision_id=c.source_revision_id
    JOIN chain_event_placement_observation_v2 observation_row ON observation_row.id=e.observation_id AND observation_row.event_identity_id=i.id AND observation_row.placement_id=placement.id
    JOIN source_materialization_cursor source_cursor ON source_cursor.source_id=c.source_id AND source_cursor.source_revision_id=c.source_revision_id
    JOIN chain_event_application_attempt_v2 application_row ON application_row.id=NEW.application_attempt_id AND application_row.event_identity_id=i.id AND application_row.placement_id=placement.id AND application_row.effect_version=NEW.effect_manifest_version
    JOIN fair_launch_application_lineage_v3 lineage ON lineage.id=NEW.source_application_lineage_id AND lineage.ce_application_attempt_id=application_row.id
    JOIN fair_launch_effect_attestation_v3 attestation ON attestation.id=NEW.attestation_id AND attestation.source_application_lineage_id=lineage.id
    WHERE c.activation_row_id=NEW.activation_row_id AND c.chain_id=NEW.chain_id AND c.source_id=NEW.source_id AND c.source_revision_id=NEW.source_revision_id AND c.source_binding_id=NEW.source_binding_id AND
      c.status='active' AND c.cursor_generation=NEW.previous_cursor_generation AND c.lease_generation=NEW.lease_generation AND c.fence_epoch=NEW.fence_epoch AND
      profile.effect_manifest_version=NEW.effect_manifest_version AND profile.valid_from<=NEW.created_at AND profile.valid_until>NEW.created_at AND
      lease_row.status='active' AND lease_row.active_slot=1 AND lease_row.lease_generation=NEW.lease_generation AND lease_row.fence_epoch=NEW.fence_epoch AND lease_row.lease_token_hash=c.lease_token_hash AND
      archive_cursor.status='active' AND archive_cursor.lease_generation=NEW.lease_generation AND archive_cursor.fence_epoch=NEW.fence_epoch AND archive_cursor.lease_token_hash=c.lease_token_hash AND archive_cursor.archived_through_block>=placement.block_number AND
      e.batch_receipt_id=NEW.archive_batch_receipt_id AND e.event_identity_id=NEW.event_identity_id AND e.placement_id=NEW.placement_id AND e.observation_id=NEW.observation_id AND
      e.event_name IN('FairLaunchTokensDeposited','FairLaunchActivated','FairLaunchContribution','FairLaunchEnded','FairLaunchSuccessfulReleased','FairLaunchCancelled','FairLaunchRefunded','FairLaunchClaimed','FairLaunchProjectProceedsWithdrawn','FairLaunchPlatformFeeWithdrawn','FairLaunchFailedProjectTokensWithdrawn') AND e.payload_digest=NEW.payload_digest AND
      placement.canonical_slot=1 AND placement.finalized_at IS NOT NULL AND placement.noncanonical_at IS NULL AND
      (placement.block_number>c.materialized_through_block OR (placement.block_number=c.materialized_through_block AND (placement.transaction_index>c.materialized_through_tx_index OR (placement.transaction_index=c.materialized_through_tx_index AND i.tx_log_ordinal>c.materialized_through_tx_log_ordinal)))) AND
      source_cursor.materialized_through_block=placement.block_number AND source_cursor.materialized_through_block_hash=placement.block_hash AND source_cursor.materialized_through_tx_index=placement.transaction_index AND source_cursor.materialized_through_tx_log_ordinal=i.tx_log_ordinal AND source_cursor.last_event_identity_id=i.id AND source_cursor.lease_token=c.lease_token_hash AND source_cursor.fence_epoch=c.fence_epoch AND source_cursor.status='active' AND source_cursor.last_error='' AND
      application_row.expected_effect=1 AND application_row.disposition='exact' AND application_row.applied_at IS NOT NULL AND application_row.reversed_at IS NULL AND
      lineage.run_row_id=c.activation_row_id AND lineage.chain_id=c.chain_id AND lineage.source_binding_id=c.source_binding_id AND lineage.source_id=c.source_id AND lineage.event_identity_id=i.id AND lineage.placement_id=placement.id AND lineage.source_revision_id=c.source_revision_id AND lineage.effect_manifest_version=NEW.effect_manifest_version AND lineage.disposition='forward' AND lineage.state='applied' AND lineage.payload_digest=NEW.payload_digest AND lineage.effect_digest=NEW.effect_digest AND lineage.ledger_digest=NEW.ledger_digest AND lineage.mutation_digest=NEW.mutation_digest AND
      attestation.payload_digest=NEW.payload_digest AND attestation.effect_digest=NEW.effect_digest AND attestation.ledger_digest=NEW.ledger_digest AND attestation.mutation_digest=NEW.mutation_digest AND attestation.attestation_digest=NEW.attestation_digest AND
      ((c.last_materialization_receipt_id IS NULL AND NEW.previous_receipt_digest IS NULL) OR (c.last_materialization_receipt_id IS NOT NULL AND NEW.previous_receipt_digest=(SELECT previous_receipt.receipt_digest FROM fair_launch_mainnet_materialization_receipt_v1 previous_receipt WHERE previous_receipt.id=c.last_materialization_receipt_id)))
  ) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet materialization receipt evidence mismatch'; END IF;
  IF EXISTS(
    SELECT 1 FROM fair_launch_mainnet_archive_batch_event_v1 earlier
    JOIN chain_event_identity_v2 earlier_identity ON earlier_identity.id=earlier.event_identity_id
    JOIN chain_event_placement_v2 earlier_placement ON earlier_placement.id=earlier.placement_id AND earlier_placement.event_identity_id=earlier_identity.id
    JOIN fair_launch_mainnet_materialization_cursor_v1 c ON c.activation_row_id=NEW.activation_row_id
    JOIN chain_event_placement_v2 current_placement ON current_placement.id=NEW.placement_id
    JOIN chain_event_identity_v2 current_identity ON current_identity.id=NEW.event_identity_id
    WHERE earlier.activation_row_id=NEW.activation_row_id AND earlier.event_name IN('FairLaunchTokensDeposited','FairLaunchActivated','FairLaunchContribution','FairLaunchEnded','FairLaunchSuccessfulReleased','FairLaunchCancelled','FairLaunchRefunded','FairLaunchClaimed','FairLaunchProjectProceedsWithdrawn','FairLaunchPlatformFeeWithdrawn','FairLaunchFailedProjectTokensWithdrawn') AND
      earlier_placement.canonical_slot=1 AND earlier_placement.finalized_at IS NOT NULL AND earlier_placement.noncanonical_at IS NULL AND
      (earlier_placement.block_number>c.materialized_through_block OR (earlier_placement.block_number=c.materialized_through_block AND (earlier_placement.transaction_index>c.materialized_through_tx_index OR (earlier_placement.transaction_index=c.materialized_through_tx_index AND earlier_identity.tx_log_ordinal>c.materialized_through_tx_log_ordinal)))) AND
      (earlier_placement.block_number<current_placement.block_number OR (earlier_placement.block_number=current_placement.block_number AND (earlier_placement.transaction_index<current_placement.transaction_index OR (earlier_placement.transaction_index=current_placement.transaction_index AND earlier_identity.tx_log_ordinal<current_identity.tx_log_ordinal))))
  ) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet materialization receipt predecessor mismatch'; END IF;
END$$
CREATE TRIGGER trg_fl_mainnet_materialization_cursor_validate_update BEFORE UPDATE ON fair_launch_mainnet_materialization_cursor_v1 FOR EACH ROW BEGIN
  IF NOT(OLD.activation_row_id<=>NEW.activation_row_id AND OLD.chain_id<=>NEW.chain_id AND OLD.source_id<=>NEW.source_id AND OLD.source_revision_id<=>NEW.source_revision_id AND OLD.source_binding_id<=>NEW.source_binding_id AND OLD.lease_generation<=>NEW.lease_generation AND OLD.fence_epoch<=>NEW.fence_epoch AND OLD.lease_token_hash<=>NEW.lease_token_hash AND OLD.created_at<=>NEW.created_at) OR
     OLD.status<>'active' OR NEW.status<>'active' OR NEW.cursor_generation<>OLD.cursor_generation+1 OR NEW.last_materialization_receipt_id IS NULL OR NEW.last_materialization_receipt_id<=>OLD.last_materialization_receipt_id OR
     NOT EXISTS(
       SELECT 1 FROM fair_launch_mainnet_materialization_receipt_v1 r
       JOIN fair_launch_mainnet_archive_batch_event_v1 e ON e.id=r.archive_event_id
       JOIN chain_event_identity_v2 i ON i.id=r.event_identity_id
       JOIN chain_event_placement_v2 p ON p.id=r.placement_id AND p.event_identity_id=i.id
       WHERE r.id=NEW.last_materialization_receipt_id AND r.activation_row_id=NEW.activation_row_id AND r.previous_cursor_generation=OLD.cursor_generation AND r.lease_generation=NEW.lease_generation AND r.fence_epoch=NEW.fence_epoch AND
         r.archive_event_id=NEW.last_archive_event_id AND r.event_identity_id=NEW.last_event_identity_id AND
         p.block_number=NEW.materialized_through_block AND p.block_hash=NEW.materialized_through_block_hash AND p.transaction_index=NEW.materialized_through_tx_index AND i.tx_log_ordinal=NEW.materialized_through_tx_log_ordinal AND
         ((OLD.last_materialization_receipt_id IS NULL AND r.previous_receipt_digest IS NULL) OR (OLD.last_materialization_receipt_id IS NOT NULL AND r.previous_receipt_digest=(SELECT previous_receipt.receipt_digest FROM fair_launch_mainnet_materialization_receipt_v1 previous_receipt WHERE previous_receipt.id=OLD.last_materialization_receipt_id)))
     ) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet materialization cursor receipt mismatch'; END IF;
END$$
CREATE TRIGGER trg_fl_mainnet_materialization_cursor_no_delete BEFORE DELETE ON fair_launch_mainnet_materialization_cursor_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_materialization_cursor_v1 cannot be deleted'; END$$
CREATE TRIGGER trg_fl_mainnet_materialization_receipt_no_update BEFORE UPDATE ON fair_launch_mainnet_materialization_receipt_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_materialization_receipt_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_materialization_receipt_no_delete BEFORE DELETE ON fair_launch_mainnet_materialization_receipt_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_materialization_receipt_v1 is append-only'; END$$
DELIMITER ;
