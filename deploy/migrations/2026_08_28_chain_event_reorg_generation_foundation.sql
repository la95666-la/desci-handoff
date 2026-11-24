-- JoinCare IDO Sprint 1A-P3-B2-B1-B0 · fail-closed candidate generation foundation.
-- This migration freezes a complete runtime outbox/nonce snapshot and the five
-- B1-B requirements. It cannot admit trusted attestations, create candidate
-- financial facts, write a generation checkpoint, switch effective state, or
-- resolve the incident.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_event_reorg_generation;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_event_reorg_generation()
BEGIN
  IF (SELECT COUNT(*) FROM schema_migrations WHERE status='applied' AND filename='2026_08_27_chain_event_reorg_prerequisite_inventory.sql'
      AND checksum='fa41cc6655bc2be808ca3b45e4ff42590d0126f6ed8dedc37141d553a8704e0a')<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation prerequisite migration is incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE' AND TABLE_NAME IN (
      'chain_event_sync_state','chain_event_reorg_incident','chain_event_reorg_materialization_run',
      'chain_event_reorg_materialization_manifest','chain_event_reorg_assessment_run','chain_event_reorg_assessment_manifest',
      'chain_event_reorg_prerequisite_run','chain_event_reorg_prerequisite_job','chain_event_reorg_prerequisite_manifest',
      'chain_tx_submission','chain_relayer_nonce'))<>11 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation source tables are incomplete';
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
      'chain_event_reorg_generation','chain_event_reorg_generation_job','chain_event_reorg_generation_sender','chain_event_reorg_generation_submission',
      'chain_event_reorg_generation_nonce','chain_event_reorg_generation_requirement','chain_event_reorg_generation_event',
      'chain_event_reorg_generation_manifest','chain_event_reorg_candidate_project','chain_event_reorg_candidate_ledger',
      'chain_event_reorg_candidate_v2_entitlement','chain_event_reorg_candidate_v1_fact',
      'chain_event_reorg_candidate_wallet_aggregate','chain_event_reorg_candidate_delta',
      'chain_event_reorg_generation_checkpoint')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='pre-existing reorg generation tables require manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_event_reorg_generation();
DROP PROCEDURE jc_preflight_chain_event_reorg_generation;

-- The snapshot predicate is chain+sender across every state/contract. The
-- existing sender+nonce unique key cannot range-lock unsigned nonce=NULL rows.
ALTER TABLE chain_tx_submission
  ADD KEY idx_chain_tx_generation_sender (chain_id,sender_address,id);

CREATE TABLE chain_event_reorg_generation (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  generation_uuid CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  attempt_no INT UNSIGNED NOT NULL,
  generation_kind VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  prerequisite_run_id BIGINT UNSIGNED NOT NULL,
  prerequisite_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  assessment_run_id BIGINT UNSIGNED NOT NULL,
  assessment_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  materialization_run_id BIGINT UNSIGNED NOT NULL,
  materialization_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  boundary_block BIGINT UNSIGNED NOT NULL,
  boundary_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  target_safe_block BIGINT UNSIGNED NOT NULL,
  target_safe_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  legacy_contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  v2_contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  old_effect_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  replacement_effect_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  algorithm_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  schema_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  descriptor_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_generation_uuid (generation_uuid),
  UNIQUE KEY uk_chain_event_reorg_generation_attempt (incident_id,attempt_no),
  UNIQUE KEY uk_chain_event_reorg_generation_descriptor (descriptor_digest),
  KEY idx_chain_event_reorg_generation_prerequisite (prerequisite_run_id),
  CONSTRAINT fk_chain_event_reorg_generation_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_generation_prerequisite FOREIGN KEY (prerequisite_run_id)
    REFERENCES chain_event_reorg_prerequisite_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_generation_assessment FOREIGN KEY (assessment_run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_generation_materialization FOREIGN KEY (materialization_run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_generation_identity CHECK (
    incident_id>0 AND attempt_no>0 AND prerequisite_run_id>0 AND assessment_run_id>0 AND materialization_run_id>0
    AND chain_id>0 AND boundary_block>0 AND target_safe_block>=boundary_block
    AND generation_kind='candidate'
    AND generation_uuid REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
    AND algorithm_version='joincare-reorg-generation-foundation-v1' AND schema_version='reorg-generation-v1'
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_generation_sources CHECK (
    prerequisite_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND assessment_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND materialization_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND old_effect_root REGEXP '^[0-9a-f]{64}$' AND replacement_effect_root REGEXP '^[0-9a-f]{64}$'
    AND descriptor_digest REGEXP '^[0-9a-f]{64}$'
    AND boundary_block_hash REGEXP '^0x[0-9a-f]{64}$' AND target_safe_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND legacy_contract_address REGEXP '^0x[0-9a-f]{40}$' AND legacy_contract_address<>CONCAT('0x',REPEAT('0',40))
    AND v2_contract_address REGEXP '^0x[0-9a-f]{40}$' AND v2_contract_address<>CONCAT('0x',REPEAT('0',40))
    AND legacy_contract_address<>v2_contract_address
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_generation_job (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  state VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  stage VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  active_slot TINYINT UNSIGNED NULL,
  sender_count BIGINT UNSIGNED NOT NULL,
  submission_count BIGINT UNSIGNED NOT NULL,
  submission_payload_bytes BIGINT UNSIGNED NOT NULL,
  nonce_count BIGINT UNSIGNED NOT NULL,
  requirement_count BIGINT UNSIGNED NOT NULL,
  unmet_count BIGINT UNSIGNED NOT NULL,
  verified_count BIGINT UNSIGNED NOT NULL,
  sender_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  submission_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  nonce_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  requirement_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  lease_token VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  lease_expires_at DATETIME(6) NULL,
  fence_epoch BIGINT UNSIGNED NOT NULL DEFAULT 0,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  last_error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  last_error_message VARCHAR(500) NOT NULL DEFAULT '',
  completed_at DATETIME(6) NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_generation_active (incident_id,active_slot),
  KEY idx_chain_event_reorg_generation_job_state (state,updated_at,run_id),
  CONSTRAINT fk_chain_event_reorg_generation_job_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_generation_job_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_generation_job_state CHECK (
    state IN ('preparing','completed','invalidated') AND stage IN ('snapshot','requirements','finalize','done')
    AND ((state='preparing' AND active_slot=1 AND active_slot IS NOT NULL AND completed_at IS NULL)
      OR (state<>'preparing' AND active_slot IS NULL AND completed_at IS NOT NULL))
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_generation_job_checkpoint CHECK (
    requirement_count<=5 AND unmet_count+verified_count=requirement_count
    AND sender_root REGEXP '^[0-9a-f]{64}$' AND submission_root REGEXP '^[0-9a-f]{64}$'
    AND nonce_root REGEXP '^[0-9a-f]{64}$' AND requirement_root REGEXP '^[0-9a-f]{64}$'
    AND ((lease_token='' AND lease_expires_at IS NULL)
      OR (lease_token REGEXP '^[0-9a-f]{64}$' AND lease_expires_at IS NOT NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_generation_sender (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  sender_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_contract_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL,
  writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(id),
  UNIQUE KEY uk_chain_event_reorg_generation_sender_ordinal(run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_generation_sender_identity(run_id,chain_id,sender_address),
  UNIQUE KEY uk_chain_event_reorg_generation_sender_digest(run_id,row_digest),
  CONSTRAINT fk_chain_event_reorg_generation_sender_run FOREIGN KEY(run_id)
    REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_generation_sender_fields CHECK(
    ordinal>0 AND chain_id>0 AND sender_address REGEXP '^0x[0-9a-f]{40}$'
    AND source_contract_root REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0
    AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$'
    AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_generation_submission (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  source_submission_id BIGINT UNSIGNED NOT NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  action VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_attempt_no INT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  sender_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  call_data MEDIUMBLOB NOT NULL,
  call_data_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  value_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_state VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  nonce BIGINT UNSIGNED NULL,
  gas_limit BIGINT UNSIGNED NULL,
  gas_price_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  raw_tx MEDIUMBLOB NULL,
  raw_tx_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL,
  source_active_slot TINYINT UNSIGNED NULL,
  broadcast_count INT UNSIGNED NOT NULL,
  last_broadcast_at DATETIME(6) NULL,
  last_error_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  receipt_status BIGINT UNSIGNED NULL,
  receipt_block_number BIGINT UNSIGNED NULL,
  receipt_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  receipt_gas_used BIGINT UNSIGNED NULL,
  final_event_record_id BIGINT UNSIGNED NULL,
  finalized_at DATETIME(6) NULL,
  cancelled_by_admin_id BIGINT UNSIGNED NULL,
  cancel_reason_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  cancelled_at DATETIME(6) NULL,
  generation_disposition VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_snapshot_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL,
  writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_created_at DATETIME(6) NOT NULL,
  source_updated_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_generation_submission_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_generation_submission_source (run_id,source_submission_id),
  UNIQUE KEY uk_chain_event_reorg_generation_submission_digest (run_id,row_digest),
  CONSTRAINT fk_chain_event_reorg_generation_submission_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_generation_submission_source FOREIGN KEY (source_submission_id)
    REFERENCES chain_tx_submission(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_generation_submission_fields CHECK (
    ordinal>0 AND source_submission_id>0 AND project_id>0 AND source_attempt_no>0 AND chain_id>0
    AND OCTET_LENGTH(call_data) BETWEEN 1 AND 1048576 AND (raw_tx IS NULL OR OCTET_LENGTH(raw_tx)<=1048576)
    AND call_data_digest REGEXP '^[0-9a-f]{64}$' AND raw_tx_digest REGEXP '^[0-9a-f]{64}$'
    AND last_error_digest REGEXP '^[0-9a-f]{64}$' AND cancel_reason_digest REGEXP '^[0-9a-f]{64}$'
    AND source_snapshot_digest REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0
    AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$'
    AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
    AND sender_address REGEXP '^0x[0-9a-f]{40}$' AND contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND generation_disposition='unresolved'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_generation_nonce (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  sender_address VARCHAR(42) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  next_nonce BIGINT UNSIGNED NOT NULL,
  active_submission_id BIGINT UNSIGNED NULL,
  source_snapshot_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL,
  writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_created_at DATETIME(6) NOT NULL,
  source_updated_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_generation_nonce_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_generation_nonce_source (run_id,chain_id,sender_address),
  UNIQUE KEY uk_chain_event_reorg_generation_nonce_digest (run_id,row_digest),
  CONSTRAINT fk_chain_event_reorg_generation_nonce_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_generation_nonce_source FOREIGN KEY (chain_id,sender_address)
    REFERENCES chain_relayer_nonce(chain_id,sender_address) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_generation_nonce_active FOREIGN KEY (run_id,active_submission_id)
    REFERENCES chain_event_reorg_generation_submission(run_id,source_submission_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_generation_nonce_fields CHECK (
    ordinal>0 AND chain_id>0 AND CAST(sender_address AS BINARY)=CAST(LOWER(sender_address) AS BINARY)
    AND sender_address REGEXP '^0x[0-9a-f]{40}$'
    AND source_snapshot_digest REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0
    AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$'
    AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_generation_requirement (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  requirement_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  evidence_status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  safe_detail VARCHAR(500) NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL,
  writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_generation_requirement_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_generation_requirement_code (run_id,requirement_code),
  UNIQUE KEY uk_chain_event_reorg_generation_requirement_digest (run_id,row_digest),
  CONSTRAINT fk_chain_event_reorg_generation_requirement_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_generation_requirement_fields CHECK (
    ordinal BETWEEN 1 AND 5 AND evidence_status IN ('unmet','verified')
    AND requirement_code REGEXP '^[A-Z0-9_]{3,64}$' AND CHAR_LENGTH(TRIM(safe_detail)) BETWEEN 10 AND 500
    AND writer_fence_epoch>0 AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$'
    AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_generation_event (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  sequence_no INT UNSIGNED NOT NULL,
  event_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  stage VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  message VARCHAR(500) NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL,
  writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_generation_event_sequence (run_id,sequence_no),
  UNIQUE KEY uk_chain_event_reorg_generation_event_hash (event_hash),
  CONSTRAINT fk_chain_event_reorg_generation_event_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_generation_event_fields CHECK (
    sequence_no>0 AND event_type IN ('CREATED','COMPLETED','INVALIDATED') AND stage IN ('create','finalize','invalidate')
    AND ((event_type='INVALIDATED' AND CHAR_LENGTH(error_code)>0) OR (event_type<>'INVALIDATED' AND error_code=''))
    AND CHAR_LENGTH(TRIM(message)) BETWEEN 10 AND 500
    AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$'
    AND previous_event_hash REGEXP '^[0-9a-f]{64}$' AND event_hash REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_generation_manifest (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  descriptor_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  prerequisite_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  assessment_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  materialization_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  outcome VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  candidate_ready TINYINT UNSIGNED NOT NULL,
  requirement_count BIGINT UNSIGNED NOT NULL,
  unmet_count BIGINT UNSIGNED NOT NULL,
  verified_count BIGINT UNSIGNED NOT NULL,
  sender_count BIGINT UNSIGNED NOT NULL,
  submission_count BIGINT UNSIGNED NOT NULL,
  submission_payload_bytes BIGINT UNSIGNED NOT NULL,
  nonce_count BIGINT UNSIGNED NOT NULL,
  candidate_project_count BIGINT UNSIGNED NOT NULL,
  candidate_ledger_count BIGINT UNSIGNED NOT NULL,
  candidate_entitlement_count BIGINT UNSIGNED NOT NULL,
  candidate_v1_fact_count BIGINT UNSIGNED NOT NULL,
  candidate_wallet_count BIGINT UNSIGNED NOT NULL,
  signed_delta_count BIGINT UNSIGNED NOT NULL,
  checkpoint_count BIGINT UNSIGNED NOT NULL,
  sender_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  submission_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  nonce_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  requirement_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  candidate_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  delta_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  checkpoint_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_count BIGINT UNSIGNED NOT NULL,
  event_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL,
  writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  completed_at DATETIME(6) NOT NULL,
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_generation_manifest_digest (manifest_digest),
  CONSTRAINT fk_chain_event_reorg_generation_manifest_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_generation_manifest_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_generation_manifest_fields CHECK (
    outcome='BLOCKED' AND candidate_ready=0 AND requirement_count=5 AND unmet_count=4 AND verified_count=1
    AND candidate_project_count=0 AND candidate_ledger_count=0 AND candidate_entitlement_count=0
    AND candidate_v1_fact_count=0 AND candidate_wallet_count=0 AND signed_delta_count=0 AND checkpoint_count=0
    AND candidate_root=REPEAT('0',64) AND delta_root=REPEAT('0',64) AND checkpoint_root=REPEAT('0',64)
    AND event_count=2 AND writer_fence_epoch>0
    AND descriptor_digest REGEXP '^[0-9a-f]{64}$' AND prerequisite_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND assessment_manifest_digest REGEXP '^[0-9a-f]{64}$' AND materialization_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND sender_root REGEXP '^[0-9a-f]{64}$' AND submission_root REGEXP '^[0-9a-f]{64}$'
    AND nonce_root REGEXP '^[0-9a-f]{64}$' AND requirement_root REGEXP '^[0-9a-f]{64}$'
    AND event_root REGEXP '^[0-9a-f]{64}$' AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$'
    AND manifest_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Successor candidate interfaces. B1-B0 installs exact domain identities but
-- physically refuses all rows until a separate trusted-evidence migration is approved.
CREATE TABLE chain_event_reorg_candidate_project (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, run_id BIGINT UNSIGNED NOT NULL, ordinal BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL, contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, project_id_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  state_payload MEDIUMBLOB NOT NULL, state_payload_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL, writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), PRIMARY KEY(id),
  UNIQUE KEY uk_chain_event_reorg_candidate_project_ordinal(run_id,ordinal), UNIQUE KEY uk_chain_event_reorg_candidate_project_identity(run_id,chain_id,contract_address,contract_version,project_id_raw),
  CONSTRAINT fk_chain_event_reorg_candidate_project_run FOREIGN KEY(run_id) REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_candidate_project_fields CHECK(ordinal>0 AND project_id_raw REGEXP '^[1-9][0-9]{0,77}$' AND OCTET_LENGTH(state_payload) BETWEEN 2 AND 1048576 AND state_payload_digest REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0 AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$' AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_candidate_ledger (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, run_id BIGINT UNSIGNED NOT NULL, ordinal BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL, contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, log_index INT UNSIGNED NOT NULL,
  presence VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, fact_payload MEDIUMBLOB NOT NULL,
  fact_payload_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, source_effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL, writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), PRIMARY KEY(id),
  UNIQUE KEY uk_chain_event_reorg_candidate_ledger_ordinal(run_id,ordinal), UNIQUE KEY uk_chain_event_reorg_candidate_ledger_identity(run_id,chain_id,contract_address,tx_hash,log_index),
  CONSTRAINT fk_chain_event_reorg_candidate_ledger_run FOREIGN KEY(run_id) REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_candidate_ledger_fields CHECK(ordinal>0 AND presence IN('present','absent') AND OCTET_LENGTH(fact_payload) BETWEEN 2 AND 1048576 AND fact_payload_digest REGEXP '^[0-9a-f]{64}$' AND source_effect_digest REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0 AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$' AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_candidate_v2_entitlement (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, run_id BIGINT UNSIGNED NOT NULL, ordinal BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL, contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, log_index INT UNSIGNED NOT NULL,
  presence VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, fact_payload MEDIUMBLOB NOT NULL,
  fact_payload_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, source_effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL, writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), PRIMARY KEY(id),
  UNIQUE KEY uk_chain_event_reorg_candidate_entitlement_ordinal(run_id,ordinal), UNIQUE KEY uk_chain_event_reorg_candidate_entitlement_identity(run_id,chain_id,contract_address,tx_hash,log_index),
  CONSTRAINT fk_chain_event_reorg_candidate_entitlement_run FOREIGN KEY(run_id) REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_candidate_entitlement_fields CHECK(ordinal>0 AND presence IN('present','absent') AND OCTET_LENGTH(fact_payload) BETWEEN 2 AND 1048576 AND fact_payload_digest REGEXP '^[0-9a-f]{64}$' AND source_effect_digest REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0 AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$' AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_candidate_v1_fact (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, run_id BIGINT UNSIGNED NOT NULL, ordinal BIGINT UNSIGNED NOT NULL,
  fact_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, stable_identity_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  fact_payload MEDIUMBLOB NOT NULL, fact_payload_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL, writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), PRIMARY KEY(id),
  UNIQUE KEY uk_chain_event_reorg_candidate_v1_ordinal(run_id,ordinal), UNIQUE KEY uk_chain_event_reorg_candidate_v1_identity(run_id,stable_identity_digest),
  CONSTRAINT fk_chain_event_reorg_candidate_v1_run FOREIGN KEY(run_id) REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_candidate_v1_fields CHECK(ordinal>0 AND fact_kind IN('entitlement','claim','refund','lifecycle') AND stable_identity_digest REGEXP '^[0-9a-f]{64}$' AND OCTET_LENGTH(fact_payload) BETWEEN 2 AND 1048576 AND fact_payload_digest REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0 AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$' AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_candidate_wallet_aggregate (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, run_id BIGINT UNSIGNED NOT NULL, ordinal BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL, contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, wallet_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  token_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, token_decimals TINYINT UNSIGNED NOT NULL,
  aggregate_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, amount_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, writer_fence_epoch BIGINT UNSIGNED NOT NULL,
  writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), PRIMARY KEY(id),
  UNIQUE KEY uk_chain_event_reorg_candidate_wallet_ordinal(run_id,ordinal), UNIQUE KEY uk_chain_event_reorg_candidate_wallet_identity(run_id,chain_id,contract_address,project_id_raw,wallet_address,token_address,aggregate_kind),
  CONSTRAINT fk_chain_event_reorg_candidate_wallet_run FOREIGN KEY(run_id) REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_candidate_wallet_fields CHECK(ordinal>0 AND project_id_raw REGEXP '^[1-9][0-9]{0,77}$' AND token_decimals BETWEEN 1 AND 18 AND amount_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND source_root REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0 AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$' AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_candidate_delta (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, run_id BIGINT UNSIGNED NOT NULL, ordinal BIGINT UNSIGNED NOT NULL,
  domain VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, stable_identity_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  asset_role VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, token_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  token_decimals TINYINT UNSIGNED NOT NULL, boundary_raw MEDIUMTEXT CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  old_branch_delta MEDIUMTEXT CHARACTER SET ascii COLLATE ascii_bin NOT NULL, replacement_branch_delta MEDIUMTEXT CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  candidate_raw MEDIUMTEXT CHARACTER SET ascii COLLATE ascii_bin NOT NULL, switch_delta MEDIUMTEXT CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, writer_fence_epoch BIGINT UNSIGNED NOT NULL,
  writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), PRIMARY KEY(id),
  UNIQUE KEY uk_chain_event_reorg_candidate_delta_ordinal(run_id,ordinal), UNIQUE KEY uk_chain_event_reorg_candidate_delta_identity(run_id,domain,stable_identity_digest,asset_role,token_address),
  CONSTRAINT fk_chain_event_reorg_candidate_delta_run FOREIGN KEY(run_id) REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_candidate_delta_fields CHECK(ordinal>0 AND asset_role IN('payment','project','global') AND token_decimals BETWEEN 1 AND 18 AND boundary_raw REGEXP '^(0|-?[1-9][0-9]*)$' AND old_branch_delta REGEXP '^(0|-?[1-9][0-9]*)$' AND replacement_branch_delta REGEXP '^(0|-?[1-9][0-9]*)$' AND candidate_raw REGEXP '^(0|-?[1-9][0-9]*)$' AND switch_delta REGEXP '^(0|-?[1-9][0-9]*)$' AND source_root REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0 AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$' AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_generation_checkpoint (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, run_id BIGINT UNSIGNED NOT NULL, chain_id BIGINT UNSIGNED NOT NULL,
  block_number BIGINT UNSIGNED NOT NULL, block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  registered_source_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, evidence_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, ledger_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  entitlement_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, delta_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  submission_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, nonce_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_checkpoint_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, checkpoint_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL, writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), PRIMARY KEY(id), UNIQUE KEY uk_chain_event_reorg_generation_checkpoint_run(run_id),
  UNIQUE KEY uk_chain_event_reorg_generation_checkpoint_digest(checkpoint_digest),
  CONSTRAINT fk_chain_event_reorg_generation_checkpoint_run FOREIGN KEY(run_id) REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_generation_checkpoint_fields CHECK(chain_id>0 AND block_number>0 AND block_hash REGEXP '^0x[0-9a-f]{64}$' AND registered_source_root REGEXP '^[0-9a-f]{64}$' AND evidence_root REGEXP '^[0-9a-f]{64}$' AND project_root REGEXP '^[0-9a-f]{64}$' AND ledger_root REGEXP '^[0-9a-f]{64}$' AND entitlement_root REGEXP '^[0-9a-f]{64}$' AND delta_root REGEXP '^[0-9a-f]{64}$' AND submission_root REGEXP '^[0-9a-f]{64}$' AND nonce_root REGEXP '^[0-9a-f]{64}$' AND previous_checkpoint_digest REGEXP '^[0-9a-f]{64}$' AND checkpoint_digest REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0 AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DELIMITER $$
CREATE TRIGGER trg_chain_event_reorg_generation_validate_insert BEFORE INSERT ON chain_event_reorg_generation FOR EACH ROW
BEGIN
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation id must be database assigned'; END IF;
  IF NEW.attempt_no<>COALESCE((SELECT MAX(g.attempt_no)+1 FROM chain_event_reorg_generation g WHERE g.incident_id=NEW.incident_id),1) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation attempt sequence is not contiguous';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_incident i
      JOIN chain_event_reorg_prerequisite_run p ON p.incident_id=i.id
      JOIN chain_event_reorg_prerequisite_job pj ON pj.run_id=p.id AND pj.state='completed'
      JOIN chain_event_reorg_prerequisite_manifest pm ON pm.run_id=p.id AND pm.incident_id=i.id
      JOIN chain_event_reorg_assessment_run a ON a.id=p.assessment_run_id AND a.incident_id=i.id
      JOIN chain_event_reorg_assessment_manifest am ON am.run_id=a.id AND am.incident_id=i.id
      JOIN chain_event_reorg_materialization_run m ON m.id=a.materialization_run_id AND m.incident_id=i.id
      JOIN chain_event_reorg_materialization_manifest mm ON mm.run_id=m.id AND mm.incident_id=i.id
      WHERE i.id=NEW.incident_id AND i.active_slot=1 AND i.status='detected'
      AND p.id=NEW.prerequisite_run_id AND pm.manifest_digest=NEW.prerequisite_manifest_digest
      AND a.id=NEW.assessment_run_id AND am.manifest_digest=NEW.assessment_manifest_digest
      AND m.id=NEW.materialization_run_id AND mm.manifest_digest=NEW.materialization_manifest_digest
      AND pm.outcome='BLOCKED' AND pm.candidate_ready=0 AND NEW.chain_id=a.chain_id
      AND NEW.boundary_block=a.boundary_block AND NEW.boundary_block_hash=a.boundary_block_hash
      AND NEW.target_safe_block=a.target_safe_block AND NEW.target_safe_block_hash=a.target_safe_block_hash
      AND NEW.legacy_contract_address=m.legacy_contract_address AND NEW.v2_contract_address=m.contract_address
      AND NEW.old_effect_root=mm.old_effect_digest AND NEW.replacement_effect_root=mm.replacement_effect_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation descriptor source identity mismatch';
  END IF;
  IF NEW.descriptor_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-descriptor.v1',CHAR(10),
      NEW.generation_uuid,CHAR(10),NEW.incident_id,CHAR(10),NEW.attempt_no,CHAR(10),NEW.generation_kind,CHAR(10),
      NEW.prerequisite_run_id,CHAR(10),NEW.prerequisite_manifest_digest,CHAR(10),NEW.assessment_run_id,CHAR(10),
      NEW.assessment_manifest_digest,CHAR(10),NEW.materialization_run_id,CHAR(10),NEW.materialization_manifest_digest,CHAR(10),
      NEW.chain_id,CHAR(10),NEW.boundary_block,CHAR(10),NEW.boundary_block_hash,CHAR(10),NEW.target_safe_block,CHAR(10),
      NEW.target_safe_block_hash,CHAR(10),NEW.legacy_contract_address,CHAR(10),NEW.v2_contract_address,CHAR(10),
      NEW.old_effect_root,CHAR(10),NEW.replacement_effect_root,CHAR(10),NEW.algorithm_version,CHAR(10),NEW.schema_version,CHAR(10),
      DATE_FORMAT(NEW.created_at,'%Y-%m-%dT%H:%i:%s.%fZ')),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation descriptor digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_generation_no_update BEFORE UPDATE ON chain_event_reorg_generation FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation descriptors are immutable'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_no_delete BEFORE DELETE ON chain_event_reorg_generation FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation descriptors are immutable'; END$$

CREATE TRIGGER trg_chain_event_reorg_generation_job_validate_insert BEFORE INSERT ON chain_event_reorg_generation_job FOR EACH ROW
BEGIN
  IF NEW.state<>'preparing' OR NEW.stage<>'snapshot' OR NEW.active_slot IS NULL OR NEW.active_slot<>1
     OR NEW.sender_count<>0 OR NEW.submission_count<>0 OR NEW.submission_payload_bytes<>0 OR NEW.nonce_count<>0
     OR NEW.requirement_count<>0 OR NEW.unmet_count<>0 OR NEW.verified_count<>0
     OR NEW.sender_root<>REPEAT('0',64) OR NEW.submission_root<>REPEAT('0',64) OR NEW.nonce_root<>REPEAT('0',64)
     OR NEW.requirement_root<>REPEAT('0',64) OR NEW.lease_token<>'' OR NEW.lease_expires_at IS NOT NULL
     OR NEW.fence_epoch<>0 OR NEW.attempt_count<>0 OR NEW.last_error_code<>'' OR NEW.last_error_message<>''
     OR NEW.completed_at IS NOT NULL OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation g WHERE g.id=NEW.run_id AND g.incident_id=NEW.incident_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation job requires canonical initial state';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_generation_job_validate_update BEFORE UPDATE ON chain_event_reorg_generation_job FOR EACH ROW
BEGIN
  IF NEW.run_id<>OLD.run_id OR NEW.incident_id<>OLD.incident_id OR OLD.state<>'preparing'
     OR NEW.fence_epoch<OLD.fence_epoch OR NEW.attempt_count<OLD.attempt_count
     OR NEW.sender_count<OLD.sender_count OR NEW.submission_count<OLD.submission_count OR NEW.nonce_count<OLD.nonce_count
     OR NEW.requirement_count<OLD.requirement_count THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation job transition is not allowed';
  END IF;
  IF NEW.state='preparing' AND EXISTS (SELECT 1 FROM chain_event_reorg_generation_event e WHERE e.run_id=NEW.run_id AND e.event_type IN('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation terminal event forbids preparing updates';
  END IF;
  IF NEW.state='preparing' AND NEW.lease_token REGEXP '^[0-9a-f]{64}$' AND (
      NOT (OLD.lease_token='' OR OLD.lease_expires_at IS NULL OR OLD.lease_expires_at<=CURRENT_TIMESTAMP(6))
      OR NEW.lease_expires_at IS NULL OR NEW.lease_expires_at<=CURRENT_TIMESTAMP(6)
      OR NEW.lease_token=OLD.lease_token
      OR NEW.fence_epoch<>OLD.fence_epoch+1 OR NEW.attempt_count<>OLD.attempt_count+1
      OR NEW.stage<>OLD.stage OR NEW.sender_count<>OLD.sender_count OR NEW.submission_count<>OLD.submission_count
      OR NEW.submission_payload_bytes<>OLD.submission_payload_bytes OR NEW.nonce_count<>OLD.nonce_count
      OR NEW.requirement_count<>OLD.requirement_count OR NEW.unmet_count<>OLD.unmet_count OR NEW.verified_count<>OLD.verified_count
      OR NEW.sender_root<>OLD.sender_root OR NEW.submission_root<>OLD.submission_root OR NEW.nonce_root<>OLD.nonce_root
      OR NEW.requirement_root<>OLD.requirement_root OR NEW.last_error_code<>OLD.last_error_code
      OR NEW.last_error_message<>OLD.last_error_message OR NEW.completed_at IS NOT NULL) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation lease claim is not canonical';
  END IF;
  IF NEW.state='preparing' AND NEW.lease_token='' AND (
      OLD.lease_token NOT REGEXP '^[0-9a-f]{64}$' OR OLD.lease_expires_at IS NULL OR OLD.lease_expires_at<=CURRENT_TIMESTAMP(6)
      OR NEW.lease_expires_at IS NOT NULL OR NEW.fence_epoch<>OLD.fence_epoch OR NEW.attempt_count<>OLD.attempt_count
      OR NEW.completed_at IS NOT NULL OR NOT (
        (NEW.stage=OLD.stage AND NEW.sender_count=OLD.sender_count AND NEW.submission_count=OLD.submission_count
          AND NEW.submission_payload_bytes=OLD.submission_payload_bytes AND NEW.nonce_count=OLD.nonce_count
          AND NEW.requirement_count=OLD.requirement_count AND NEW.unmet_count=OLD.unmet_count
          AND NEW.verified_count=OLD.verified_count AND NEW.sender_root=OLD.sender_root
          AND NEW.submission_root=OLD.submission_root AND NEW.nonce_root=OLD.nonce_root
          AND NEW.requirement_root=OLD.requirement_root AND CHAR_LENGTH(NEW.last_error_code)>0
          AND CHAR_LENGTH(NEW.last_error_message)>0)
        OR (OLD.stage='snapshot' AND NEW.stage='requirements' AND NEW.requirement_count=0
          AND NEW.unmet_count=0 AND NEW.verified_count=0 AND NEW.requirement_root=REPEAT('0',64)
          AND NEW.sender_count=(SELECT COUNT(*) FROM chain_event_reorg_generation_sender s WHERE s.run_id=NEW.run_id)
          AND NEW.submission_count=(SELECT COUNT(*) FROM chain_event_reorg_generation_submission s WHERE s.run_id=NEW.run_id)
          AND NEW.submission_payload_bytes=(SELECT COALESCE(SUM(OCTET_LENGTH(s.call_data)+COALESCE(OCTET_LENGTH(s.raw_tx),0)),0) FROM chain_event_reorg_generation_submission s WHERE s.run_id=NEW.run_id)
          AND NEW.nonce_count=(SELECT COUNT(*) FROM chain_event_reorg_generation_nonce n WHERE n.run_id=NEW.run_id)
          AND NEW.sender_root=COALESCE((SELECT row_digest FROM chain_event_reorg_generation_sender s WHERE s.run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))
          AND NEW.submission_root=COALESCE((SELECT row_digest FROM chain_event_reorg_generation_submission s WHERE s.run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))
          AND NEW.nonce_root=COALESCE((SELECT row_digest FROM chain_event_reorg_generation_nonce n WHERE n.run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))
          AND NEW.last_error_code='' AND NEW.last_error_message='')
        OR (OLD.stage='requirements' AND NEW.stage='finalize' AND NEW.sender_count=OLD.sender_count
          AND NEW.submission_count=OLD.submission_count AND NEW.submission_payload_bytes=OLD.submission_payload_bytes
          AND NEW.nonce_count=OLD.nonce_count AND NEW.sender_root=OLD.sender_root
          AND NEW.submission_root=OLD.submission_root AND NEW.nonce_root=OLD.nonce_root
          AND NEW.requirement_count=5 AND NEW.unmet_count=4 AND NEW.verified_count=1
          AND NEW.requirement_root=COALESCE((SELECT row_digest FROM chain_event_reorg_generation_requirement r WHERE r.run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))
          AND NEW.last_error_code='' AND NEW.last_error_message='')
      )) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation leased checkpoint is not canonical';
  END IF;
  IF NEW.state='preparing' AND NEW.lease_token='' AND OLD.lease_token='' THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation preparing updates require a lease transition';
  END IF;
  IF NEW.state IN('completed','invalidated') AND (OLD.active_slot IS NULL OR OLD.active_slot<>1
     OR OLD.lease_token NOT REGEXP '^[0-9a-f]{64}$' OR OLD.lease_expires_at IS NULL OR OLD.lease_expires_at<=CURRENT_TIMESTAMP(6)
     OR NEW.active_slot IS NOT NULL OR NEW.lease_token<>'' OR NEW.lease_expires_at IS NOT NULL
     OR NEW.fence_epoch<>OLD.fence_epoch OR NEW.attempt_count<>OLD.attempt_count OR NEW.completed_at IS NULL
     OR NEW.sender_count<>OLD.sender_count OR NEW.submission_count<>OLD.submission_count
     OR NEW.submission_payload_bytes<>OLD.submission_payload_bytes OR NEW.nonce_count<>OLD.nonce_count
     OR NEW.requirement_count<>OLD.requirement_count OR NEW.unmet_count<>OLD.unmet_count
     OR NEW.verified_count<>OLD.verified_count OR NEW.sender_root<>OLD.sender_root
     OR NEW.submission_root<>OLD.submission_root OR NEW.nonce_root<>OLD.nonce_root
     OR NEW.requirement_root<>OLD.requirement_root) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation terminal transition requires active fenced lease';
  END IF;
  IF NEW.state='completed' AND (NEW.stage<>'done' OR NEW.last_error_code<>'' OR NEW.last_error_message<>''
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_event e WHERE e.run_id=NEW.run_id AND e.sequence_no=2 AND e.event_type='COMPLETED')
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_manifest m WHERE m.run_id=NEW.run_id AND m.outcome='BLOCKED' AND m.candidate_ready=0)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation completion requires terminal blocked evidence';
  END IF;
  IF NEW.state='invalidated' AND (CHAR_LENGTH(NEW.last_error_code)=0 OR CHAR_LENGTH(NEW.last_error_message)=0
     OR EXISTS (SELECT 1 FROM chain_event_reorg_generation_manifest m WHERE m.run_id=NEW.run_id)
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_event e WHERE e.run_id=NEW.run_id AND e.sequence_no=2
         AND e.event_type='INVALIDATED' AND e.error_code=NEW.last_error_code AND e.message=NEW.last_error_message)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation invalidation requires matching terminal evidence';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_generation_job_no_delete BEFORE DELETE ON chain_event_reorg_generation_job FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation jobs cannot be deleted'; END$$

CREATE TRIGGER trg_chain_event_reorg_generation_submission_validate_insert BEFORE INSERT ON chain_event_reorg_generation_submission FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64); DECLARE expected_ordinal BIGINT UNSIGNED;
  SET expected_previous=COALESCE((SELECT row_digest FROM chain_event_reorg_generation_submission WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET expected_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_generation_submission WHERE run_id=NEW.run_id),1);
  IF NEW.id<>0 OR NEW.ordinal<>expected_ordinal OR NEW.previous_row_digest<>expected_previous
     OR NEW.call_data_digest<>LOWER(SHA2(NEW.call_data,256))
     OR NEW.raw_tx_digest<>CASE WHEN NEW.raw_tx IS NULL THEN REPEAT('0',64) ELSE LOWER(SHA2(NEW.raw_tx,256)) END
     OR NEW.last_error_digest<>(SELECT LOWER(SHA2(last_error,256)) FROM chain_tx_submission WHERE id=NEW.source_submission_id)
     OR NEW.cancel_reason_digest<>(SELECT LOWER(SHA2(cancel_reason,256)) FROM chain_tx_submission WHERE id=NEW.source_submission_id)
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_job j WHERE j.run_id=NEW.run_id AND j.state='preparing'
       AND j.stage='snapshot' AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
       AND j.fence_epoch=NEW.writer_fence_epoch AND LOWER(SHA2(j.lease_token,256))=NEW.writer_lease_token_digest)
     OR NOT EXISTS (SELECT 1 FROM chain_tx_submission s WHERE s.id=NEW.source_submission_id AND s.project_id=NEW.project_id
       AND s.action=NEW.action AND s.attempt_no=NEW.source_attempt_no AND s.chain_id=NEW.chain_id
       AND LOWER(s.sender_address)=NEW.sender_address AND LOWER(s.contract_address)=NEW.contract_address
       AND s.call_data=NEW.call_data AND s.value_raw=NEW.value_raw AND s.state=NEW.source_state
       AND s.nonce<=>NEW.nonce AND s.gas_limit<=>NEW.gas_limit AND s.gas_price_raw=NEW.gas_price_raw
       AND s.raw_tx<=>NEW.raw_tx AND LOWER(COALESCE(s.tx_hash,''))=COALESCE(NEW.tx_hash,'')
       AND s.active_slot<=>NEW.source_active_slot AND s.broadcast_count=NEW.broadcast_count
       AND s.last_broadcast_at<=>NEW.last_broadcast_at
       AND s.receipt_status<=>NEW.receipt_status AND s.receipt_block_number<=>NEW.receipt_block_number
       AND LOWER(s.receipt_block_hash)=NEW.receipt_block_hash AND s.receipt_gas_used<=>NEW.receipt_gas_used
       AND s.final_event_record_id<=>NEW.final_event_record_id AND s.finalized_at<=>NEW.finalized_at
       AND s.cancelled_by_admin_id<=>NEW.cancelled_by_admin_id AND s.cancelled_at<=>NEW.cancelled_at
       AND s.created_at=NEW.source_created_at AND s.updated_at=NEW.source_updated_at) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation submission snapshot identity mismatch';
  END IF;
  IF NEW.source_snapshot_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-submission-source.v1',CHAR(10),
      NEW.source_submission_id,CHAR(10),NEW.project_id,CHAR(10),NEW.action,CHAR(10),NEW.source_attempt_no,CHAR(10),
      NEW.chain_id,CHAR(10),NEW.sender_address,CHAR(10),NEW.contract_address,CHAR(10),NEW.call_data_digest,CHAR(10),
      NEW.value_raw,CHAR(10),NEW.source_state,CHAR(10),COALESCE(NEW.nonce,'null'),CHAR(10),COALESCE(NEW.gas_limit,'null'),CHAR(10),
      NEW.gas_price_raw,CHAR(10),NEW.raw_tx_digest,CHAR(10),COALESCE(NEW.tx_hash,'null'),CHAR(10),
      COALESCE(NEW.source_active_slot,'null'),CHAR(10),NEW.broadcast_count,CHAR(10),COALESCE(DATE_FORMAT(NEW.last_broadcast_at,'%Y-%m-%dT%H:%i:%s.%fZ'),'null'),CHAR(10),
      NEW.last_error_digest,CHAR(10),COALESCE(NEW.receipt_status,'null'),CHAR(10),COALESCE(NEW.receipt_block_number,'null'),CHAR(10),
      NEW.receipt_block_hash,CHAR(10),COALESCE(NEW.receipt_gas_used,'null'),CHAR(10),COALESCE(NEW.final_event_record_id,'null'),CHAR(10),
      COALESCE(DATE_FORMAT(NEW.finalized_at,'%Y-%m-%dT%H:%i:%s.%fZ'),'null'),CHAR(10),COALESCE(NEW.cancelled_by_admin_id,'null'),CHAR(10),
      NEW.cancel_reason_digest,CHAR(10),COALESCE(DATE_FORMAT(NEW.cancelled_at,'%Y-%m-%dT%H:%i:%s.%fZ'),'null'),CHAR(10),
      DATE_FORMAT(NEW.source_created_at,'%Y-%m-%dT%H:%i:%s.%fZ'),CHAR(10),DATE_FORMAT(NEW.source_updated_at,'%Y-%m-%dT%H:%i:%s.%fZ')),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation submission source digest mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-submission.v1',CHAR(10),NEW.run_id,CHAR(10),
      NEW.ordinal,CHAR(10),NEW.source_submission_id,CHAR(10),NEW.source_snapshot_digest,CHAR(10),NEW.generation_disposition,
      CHAR(10),NEW.writer_fence_epoch,CHAR(10),NEW.writer_lease_token_digest,CHAR(10),NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation submission row digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_generation_submission_no_update BEFORE UPDATE ON chain_event_reorg_generation_submission FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation submission snapshots are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_submission_no_delete BEFORE DELETE ON chain_event_reorg_generation_submission FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation submission snapshots are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_generation_nonce_validate_insert BEFORE INSERT ON chain_event_reorg_generation_nonce FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64); DECLARE expected_ordinal BIGINT UNSIGNED;
  SET expected_previous=COALESCE((SELECT row_digest FROM chain_event_reorg_generation_nonce WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET expected_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_generation_nonce WHERE run_id=NEW.run_id),1);
  IF NEW.id<>0 OR NEW.ordinal<>expected_ordinal OR NEW.previous_row_digest<>expected_previous
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_job j WHERE j.run_id=NEW.run_id AND j.state='preparing'
       AND j.stage='snapshot' AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
       AND j.fence_epoch=NEW.writer_fence_epoch AND LOWER(SHA2(j.lease_token,256))=NEW.writer_lease_token_digest)
     OR NOT EXISTS (SELECT 1 FROM chain_relayer_nonce n WHERE n.chain_id=NEW.chain_id AND LOWER(n.sender_address)=NEW.sender_address
       AND n.next_nonce=NEW.next_nonce AND n.active_submission_id<=>NEW.active_submission_id
       AND n.created_at=NEW.source_created_at AND n.updated_at=NEW.source_updated_at) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation nonce snapshot identity mismatch';
  END IF;
  IF NEW.source_snapshot_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-nonce-source.v1',CHAR(10),
      NEW.chain_id,CHAR(10),NEW.sender_address,CHAR(10),NEW.next_nonce,CHAR(10),COALESCE(NEW.active_submission_id,'null'),CHAR(10),
      DATE_FORMAT(NEW.source_created_at,'%Y-%m-%dT%H:%i:%s.%fZ'),CHAR(10),DATE_FORMAT(NEW.source_updated_at,'%Y-%m-%dT%H:%i:%s.%fZ')),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation nonce source digest mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-nonce.v1',CHAR(10),NEW.run_id,CHAR(10),
      NEW.ordinal,CHAR(10),NEW.chain_id,CHAR(10),NEW.sender_address,CHAR(10),NEW.next_nonce,CHAR(10),
      COALESCE(NEW.active_submission_id,0),CHAR(10),NEW.source_snapshot_digest,CHAR(10),NEW.writer_fence_epoch,CHAR(10),
      NEW.writer_lease_token_digest,CHAR(10),NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation nonce row digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_generation_nonce_no_update BEFORE UPDATE ON chain_event_reorg_generation_nonce FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation nonce snapshots are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_nonce_no_delete BEFORE DELETE ON chain_event_reorg_generation_nonce FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation nonce snapshots are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_generation_requirement_validate_insert BEFORE INSERT ON chain_event_reorg_generation_requirement FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64); DECLARE expected_ordinal BIGINT UNSIGNED;
  SET expected_previous=COALESCE((SELECT row_digest FROM chain_event_reorg_generation_requirement WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET expected_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_generation_requirement WHERE run_id=NEW.run_id),1);
  IF NEW.id<>0 OR NEW.ordinal<>expected_ordinal OR NEW.previous_row_digest<>expected_previous
     OR NEW.requirement_code<>CASE NEW.ordinal WHEN 1 THEN 'BOUNDARY_STATE_SNAPSHOT_REQUIRED' WHEN 2 THEN 'V1_SOURCE_ORIGIN_UNATTESTED'
       WHEN 3 THEN 'GLOBAL_CONFIG_BASELINE_REQUIRED' WHEN 4 THEN 'OUTBOX_GENERATION_BINDING_REQUIRED'
       WHEN 5 THEN 'GENERATION_CHECKPOINT_SCHEMA_REQUIRED' END
     OR NEW.evidence_status<>CASE WHEN NEW.ordinal=5 THEN 'verified' ELSE 'unmet' END
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_job j WHERE j.run_id=NEW.run_id AND j.state='preparing'
       AND j.stage='requirements' AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
       AND j.fence_epoch=NEW.writer_fence_epoch AND LOWER(SHA2(j.lease_token,256))=NEW.writer_lease_token_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation requirement matrix or fence mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-requirement.v1',CHAR(10),NEW.run_id,CHAR(10),
      NEW.ordinal,CHAR(10),NEW.requirement_code,CHAR(10),NEW.evidence_status,CHAR(10),NEW.safe_detail,CHAR(10),
      NEW.writer_fence_epoch,CHAR(10),NEW.writer_lease_token_digest,CHAR(10),NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation requirement row digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_generation_requirement_no_update BEFORE UPDATE ON chain_event_reorg_generation_requirement FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation requirements are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_requirement_no_delete BEFORE DELETE ON chain_event_reorg_generation_requirement FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation requirements are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_generation_event_validate_insert BEFORE INSERT ON chain_event_reorg_generation_event FOR EACH ROW
BEGIN
  DECLARE prior_hash VARCHAR(64); DECLARE prior_sequence INT UNSIGNED;
  SET prior_hash=COALESCE((SELECT event_hash FROM chain_event_reorg_generation_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),REPEAT('0',64));
  SET prior_sequence=COALESCE((SELECT sequence_no FROM chain_event_reorg_generation_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),0);
  IF NEW.id<>0 OR NEW.sequence_no<>prior_sequence+1 OR NEW.previous_event_hash<>prior_hash
     OR (NEW.event_type='CREATED' AND (NEW.sequence_no<>1 OR NEW.stage<>'create' OR NEW.writer_fence_epoch<>0 OR NEW.writer_lease_token_digest<>REPEAT('0',64)))
     OR (NEW.event_type IN('COMPLETED','INVALIDATED') AND (NEW.sequence_no<>2 OR NEW.writer_fence_epoch=0))
     OR (NEW.event_type<>'CREATED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_job j WHERE j.run_id=NEW.run_id
       AND j.state='preparing' AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
       AND j.fence_epoch=NEW.writer_fence_epoch AND LOWER(SHA2(j.lease_token,256))=NEW.writer_lease_token_digest))
     OR (NEW.event_type='CREATED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_job j WHERE j.run_id=NEW.run_id AND j.fence_epoch=0 AND j.lease_token='')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation event sequence or fence mismatch';
  END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_generation_event e WHERE e.run_id=NEW.run_id AND e.event_type IN('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation terminal event is final';
  END IF;
  IF NEW.event_type='COMPLETED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_job j WHERE j.run_id=NEW.run_id
      AND j.stage='finalize' AND j.requirement_count=5 AND j.unmet_count=4 AND j.verified_count=1) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation completion event requires complete blocked checkpoint';
  END IF;
  IF NEW.event_hash<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-event.v1',CHAR(10),NEW.run_id,CHAR(10),
      NEW.sequence_no,CHAR(10),NEW.event_type,CHAR(10),NEW.stage,CHAR(10),NEW.error_code,CHAR(10),NEW.message,CHAR(10),
      NEW.writer_fence_epoch,CHAR(10),NEW.writer_lease_token_digest,CHAR(10),NEW.previous_event_hash),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation event digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_generation_event_no_update BEFORE UPDATE ON chain_event_reorg_generation_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation events are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_event_no_delete BEFORE DELETE ON chain_event_reorg_generation_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation events are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_generation_manifest_validate_insert BEFORE INSERT ON chain_event_reorg_generation_manifest FOR EACH ROW
BEGIN
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation g JOIN chain_event_reorg_generation_job j ON j.run_id=g.id AND j.incident_id=g.incident_id
      WHERE g.id=NEW.run_id AND g.incident_id=NEW.incident_id AND g.descriptor_digest=NEW.descriptor_digest
      AND g.prerequisite_manifest_digest=NEW.prerequisite_manifest_digest AND g.assessment_manifest_digest=NEW.assessment_manifest_digest
      AND g.materialization_manifest_digest=NEW.materialization_manifest_digest AND j.state='preparing' AND j.stage='finalize'
      AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
      AND j.fence_epoch=NEW.writer_fence_epoch AND LOWER(SHA2(j.lease_token,256))=NEW.writer_lease_token_digest
      AND j.sender_count=NEW.sender_count AND j.submission_count=NEW.submission_count
      AND j.submission_payload_bytes=NEW.submission_payload_bytes AND j.nonce_count=NEW.nonce_count
      AND j.requirement_count=NEW.requirement_count AND j.unmet_count=NEW.unmet_count AND j.verified_count=NEW.verified_count
      AND j.sender_root=NEW.sender_root AND j.submission_root=NEW.submission_root AND j.nonce_root=NEW.nonce_root
      AND j.requirement_root=NEW.requirement_root) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation manifest requires complete leased checkpoint';
  END IF;
  IF (SELECT COUNT(*) FROM chain_event_reorg_generation_submission WHERE run_id=NEW.run_id)<>NEW.submission_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_generation_sender WHERE run_id=NEW.run_id)<>NEW.sender_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_generation_nonce WHERE run_id=NEW.run_id)<>NEW.nonce_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_generation_requirement WHERE run_id=NEW.run_id)<>5
     OR (SELECT COUNT(*) FROM chain_event_reorg_generation_requirement WHERE run_id=NEW.run_id AND evidence_status='unmet')<>4
     OR (SELECT COUNT(*) FROM chain_event_reorg_generation_requirement WHERE run_id=NEW.run_id AND evidence_status='verified')<>1
     OR (SELECT COUNT(*) FROM chain_event_reorg_generation_event WHERE run_id=NEW.run_id)<>2
     OR (SELECT COUNT(*) FROM chain_event_reorg_generation_event WHERE run_id=NEW.run_id AND sequence_no=2 AND event_type='COMPLETED' AND event_hash=NEW.event_root)<>1
     OR COALESCE((SELECT row_digest FROM chain_event_reorg_generation_sender WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.sender_root
     OR COALESCE((SELECT row_digest FROM chain_event_reorg_generation_submission WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.submission_root
     OR COALESCE((SELECT row_digest FROM chain_event_reorg_generation_nonce WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.nonce_root
     OR COALESCE((SELECT row_digest FROM chain_event_reorg_generation_requirement WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.requirement_root
     OR EXISTS (SELECT 1 FROM chain_event_reorg_candidate_project WHERE run_id=NEW.run_id)
     OR EXISTS (SELECT 1 FROM chain_event_reorg_candidate_ledger WHERE run_id=NEW.run_id)
     OR EXISTS (SELECT 1 FROM chain_event_reorg_candidate_v2_entitlement WHERE run_id=NEW.run_id)
     OR EXISTS (SELECT 1 FROM chain_event_reorg_candidate_v1_fact WHERE run_id=NEW.run_id)
     OR EXISTS (SELECT 1 FROM chain_event_reorg_candidate_wallet_aggregate WHERE run_id=NEW.run_id)
     OR EXISTS (SELECT 1 FROM chain_event_reorg_candidate_delta WHERE run_id=NEW.run_id)
     OR EXISTS (SELECT 1 FROM chain_event_reorg_generation_checkpoint WHERE run_id=NEW.run_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation manifest evidence sets are incomplete';
  END IF;
  IF NEW.manifest_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-manifest.v1',CHAR(10),NEW.run_id,CHAR(10),
      NEW.incident_id,CHAR(10),NEW.descriptor_digest,CHAR(10),NEW.prerequisite_manifest_digest,CHAR(10),NEW.assessment_manifest_digest,
      CHAR(10),NEW.materialization_manifest_digest,CHAR(10),NEW.outcome,CHAR(10),'false',CHAR(10),NEW.requirement_count,CHAR(10),
      NEW.unmet_count,CHAR(10),NEW.verified_count,CHAR(10),NEW.sender_count,CHAR(10),NEW.submission_count,CHAR(10),
      NEW.submission_payload_bytes,CHAR(10),NEW.nonce_count,CHAR(10),NEW.candidate_project_count,CHAR(10),
      NEW.candidate_ledger_count,CHAR(10),NEW.candidate_entitlement_count,CHAR(10),NEW.candidate_v1_fact_count,CHAR(10),
      NEW.candidate_wallet_count,CHAR(10),NEW.signed_delta_count,CHAR(10),NEW.checkpoint_count,CHAR(10),
      NEW.sender_root,CHAR(10),NEW.submission_root,CHAR(10),NEW.nonce_root,CHAR(10),NEW.requirement_root,CHAR(10),
      NEW.candidate_root,CHAR(10),NEW.delta_root,CHAR(10),NEW.checkpoint_root,CHAR(10),
      NEW.event_count,CHAR(10),NEW.event_root,CHAR(10),
      NEW.writer_fence_epoch,CHAR(10),NEW.writer_lease_token_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation manifest digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_generation_manifest_no_update BEFORE UPDATE ON chain_event_reorg_generation_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation manifests are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_manifest_no_delete BEFORE DELETE ON chain_event_reorg_generation_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation manifests are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_generation_sender_validate_insert BEFORE INSERT ON chain_event_reorg_generation_sender FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64); DECLARE expected_ordinal BIGINT UNSIGNED;
  SET expected_previous=COALESCE((SELECT row_digest FROM chain_event_reorg_generation_sender WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET expected_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_generation_sender WHERE run_id=NEW.run_id),1);
  IF NEW.id<>0 OR NEW.ordinal<>expected_ordinal OR NEW.previous_row_digest<>expected_previous
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_job j WHERE j.run_id=NEW.run_id AND j.state='preparing'
       AND j.stage='snapshot' AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
       AND j.fence_epoch=NEW.writer_fence_epoch AND LOWER(SHA2(j.lease_token,256))=NEW.writer_lease_token_digest)
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation g JOIN chain_tx_submission s ON s.chain_id=g.chain_id
       AND LOWER(s.sender_address)=NEW.sender_address AND LOWER(s.contract_address) IN(g.legacy_contract_address,g.v2_contract_address)
       WHERE g.id=NEW.run_id AND g.chain_id=NEW.chain_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation sender snapshot identity or fence mismatch';
  END IF;
  IF NEW.source_contract_root<>(SELECT LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-sender-source.v1',CHAR(10),
      g.chain_id,CHAR(10),NEW.sender_address,CHAR(10),
      IF(EXISTS(SELECT 1 FROM chain_tx_submission s WHERE s.chain_id=g.chain_id AND LOWER(s.sender_address)=NEW.sender_address AND LOWER(s.contract_address)=g.legacy_contract_address),'true','false'),CHAR(10),
      IF(EXISTS(SELECT 1 FROM chain_tx_submission s WHERE s.chain_id=g.chain_id AND LOWER(s.sender_address)=NEW.sender_address AND LOWER(s.contract_address)=g.v2_contract_address),'true','false')),256))
      FROM chain_event_reorg_generation g WHERE g.id=NEW.run_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation sender source root mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-sender.v1',CHAR(10),NEW.run_id,CHAR(10),
      NEW.ordinal,CHAR(10),NEW.chain_id,CHAR(10),NEW.sender_address,CHAR(10),NEW.source_contract_root,CHAR(10),
      NEW.writer_fence_epoch,CHAR(10),NEW.writer_lease_token_digest,CHAR(10),NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation sender row digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_generation_sender_no_update BEFORE UPDATE ON chain_event_reorg_generation_sender FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation sender snapshots are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_sender_no_delete BEFORE DELETE ON chain_event_reorg_generation_sender FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation sender snapshots are append-only'; END$$

-- Future interfaces are immutable and completely closed in B1-B0.
CREATE TRIGGER trg_chain_event_reorg_candidate_project_validate_insert BEFORE INSERT ON chain_event_reorg_candidate_project FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='B1-B0 forbids candidate project rows'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_project_no_update BEFORE UPDATE ON chain_event_reorg_candidate_project FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate project rows are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_project_no_delete BEFORE DELETE ON chain_event_reorg_candidate_project FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate project rows are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_ledger_validate_insert BEFORE INSERT ON chain_event_reorg_candidate_ledger FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='B1-B0 forbids candidate ledger rows'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_ledger_no_update BEFORE UPDATE ON chain_event_reorg_candidate_ledger FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate ledger rows are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_ledger_no_delete BEFORE DELETE ON chain_event_reorg_candidate_ledger FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate ledger rows are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_entitlement_validate_insert BEFORE INSERT ON chain_event_reorg_candidate_v2_entitlement FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='B1-B0 forbids candidate entitlement rows'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_entitlement_no_update BEFORE UPDATE ON chain_event_reorg_candidate_v2_entitlement FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate entitlement rows are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_entitlement_no_delete BEFORE DELETE ON chain_event_reorg_candidate_v2_entitlement FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate entitlement rows are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_v1_validate_insert BEFORE INSERT ON chain_event_reorg_candidate_v1_fact FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='B1-B0 forbids candidate V1 facts'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_v1_no_update BEFORE UPDATE ON chain_event_reorg_candidate_v1_fact FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate V1 facts are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_v1_no_delete BEFORE DELETE ON chain_event_reorg_candidate_v1_fact FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate V1 facts are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_wallet_validate_insert BEFORE INSERT ON chain_event_reorg_candidate_wallet_aggregate FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='B1-B0 forbids candidate wallet aggregates'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_wallet_no_update BEFORE UPDATE ON chain_event_reorg_candidate_wallet_aggregate FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate wallet aggregates are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_wallet_no_delete BEFORE DELETE ON chain_event_reorg_candidate_wallet_aggregate FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate wallet aggregates are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_delta_validate_insert BEFORE INSERT ON chain_event_reorg_candidate_delta FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='B1-B0 forbids candidate signed deltas'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_delta_no_update BEFORE UPDATE ON chain_event_reorg_candidate_delta FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate deltas are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_delta_no_delete BEFORE DELETE ON chain_event_reorg_candidate_delta FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate deltas are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_checkpoint_validate_insert BEFORE INSERT ON chain_event_reorg_generation_checkpoint FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='B1-B0 forbids generation checkpoints'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_checkpoint_no_update BEFORE UPDATE ON chain_event_reorg_generation_checkpoint FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation checkpoints are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_checkpoint_no_delete BEFORE DELETE ON chain_event_reorg_generation_checkpoint FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation checkpoints are append-only'; END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS jc_assert_chain_event_reorg_generation;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_reorg_generation()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE' AND TABLE_NAME IN (
      'chain_event_reorg_generation','chain_event_reorg_generation_job','chain_event_reorg_generation_sender','chain_event_reorg_generation_submission',
      'chain_event_reorg_generation_nonce','chain_event_reorg_generation_requirement','chain_event_reorg_generation_event',
      'chain_event_reorg_generation_manifest','chain_event_reorg_candidate_project','chain_event_reorg_candidate_ledger',
      'chain_event_reorg_candidate_v2_entitlement','chain_event_reorg_candidate_v1_fact',
      'chain_event_reorg_candidate_wallet_aggregate','chain_event_reorg_candidate_delta','chain_event_reorg_generation_checkpoint'))<>15 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation foundation tables are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND EVENT_OBJECT_TABLE IN (
      'chain_event_reorg_generation','chain_event_reorg_generation_job','chain_event_reorg_generation_sender','chain_event_reorg_generation_submission',
      'chain_event_reorg_generation_nonce','chain_event_reorg_generation_requirement','chain_event_reorg_generation_event',
      'chain_event_reorg_generation_manifest','chain_event_reorg_candidate_project','chain_event_reorg_candidate_ledger',
      'chain_event_reorg_candidate_v2_entitlement','chain_event_reorg_candidate_v1_fact',
      'chain_event_reorg_candidate_wallet_aggregate','chain_event_reorg_candidate_delta','chain_event_reorg_generation_checkpoint'))<>45 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation foundation triggers are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES'
      AND TABLE_NAME IN ('chain_event_reorg_generation','chain_event_reorg_generation_job','chain_event_reorg_generation_sender','chain_event_reorg_generation_submission',
      'chain_event_reorg_generation_nonce','chain_event_reorg_generation_requirement','chain_event_reorg_generation_event',
      'chain_event_reorg_generation_manifest','chain_event_reorg_candidate_project','chain_event_reorg_candidate_ledger',
      'chain_event_reorg_candidate_v2_entitlement','chain_event_reorg_candidate_v1_fact',
      'chain_event_reorg_candidate_wallet_aggregate','chain_event_reorg_candidate_delta','chain_event_reorg_generation_checkpoint'))<>17 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation foundation checks are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_NAME REGEXP '^fk_chain_event_reorg_(generation|candidate)_' AND UPDATE_RULE='RESTRICT' AND DELETE_RULE='RESTRICT')<>23 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation foundation foreign keys are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM (
      SELECT INDEX_NAME,GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') columns_in_order
      FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_tx_submission'
      GROUP BY INDEX_NAME
    ) source_indexes WHERE INDEX_NAME='idx_chain_tx_generation_sender' AND columns_in_order='chain_id,sender_address,id')<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation source range indexes are incomplete';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_reorg_generation();
DROP PROCEDURE jc_assert_chain_event_reorg_generation;

-- Deployment runner records checksum/status only after every postflight succeeds.
