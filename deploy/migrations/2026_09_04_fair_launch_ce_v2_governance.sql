-- JoinCare Fair Launch CE-V2, governance and shadow-ledger schema.
-- ADDITIVE ONLY. Executable only through the explicit approved Fair Launch migration gate.
SET NAMES utf8mb4;
SET @jc_fl_gate = COALESCE(@JC_FAIR_LAUNCH_MIGRATION_APPROVED,0);
DROP PROCEDURE IF EXISTS jc_require_fair_launch_gate_0904;
DELIMITER $$
CREATE PROCEDURE jc_require_fair_launch_gate_0904()
BEGIN
  IF @jc_fl_gate<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch CE-V2 migration requires approved execution gate'; END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='funding_mode') THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='09_03 Fair Launch project migration missing'; END IF;
END$$
DELIMITER ;
CALL jc_require_fair_launch_gate_0904();
DROP PROCEDURE jc_require_fair_launch_gate_0904;

CREATE TABLE IF NOT EXISTS chain_contract_source (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  chain_id BIGINT UNSIGNED NOT NULL,
  contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  funding_mode VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  registered_at DATETIME(6) NOT NULL,
  retired_at DATETIME(6) NULL,
  start_block BIGINT UNSIGNED NOT NULL,
  enabled TINYINT UNSIGNED NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_source_address (chain_id,contract_address),
  CONSTRAINT chk_source_address CHECK (chain_id>0 AND contract_address REGEXP '^0x[0-9a-f]{40}$' AND enabled IN (0,1)) ENFORCED
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS chain_contract_source_revision (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  source_id BIGINT UNSIGNED NOT NULL,
  revision_no INT UNSIGNED NOT NULL,
  activation_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  proxy_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  abi_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  confirmation_policy VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  governance_tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  governance_operation_id VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  implementation_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  implementation_code_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  dependency_manifest_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  abi_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  from_block_number BIGINT UNSIGNED NOT NULL,
  from_transaction_index INT UNSIGNED NOT NULL,
  from_tx_log_ordinal_inclusive INT UNSIGNED NOT NULL,
  to_block_number BIGINT UNSIGNED NULL,
  to_transaction_index INT UNSIGNED NULL,
  to_tx_log_ordinal_exclusive INT UNSIGNED NULL,
  activation_event_identity_id BIGINT UNSIGNED NULL,
  predecessor_revision_id BIGINT UNSIGNED NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_source_revision_no (source_id,revision_no),
  UNIQUE KEY uk_source_revision_identity_pair (id,source_id),
  UNIQUE KEY uk_source_revision_boundary (source_id,from_block_number,from_transaction_index,from_tx_log_ordinal_inclusive),
  CONSTRAINT fk_source_revision_source FOREIGN KEY (source_id) REFERENCES chain_contract_source(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_source_revision_predecessor_same_source FOREIGN KEY (predecessor_revision_id,source_id) REFERENCES chain_contract_source_revision(id,source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_source_revision CHECK (revision_no>0 AND implementation_address REGEXP '^0x[0-9a-f]{40}$' AND implementation_code_hash REGEXP '^[0-9a-f]{64}$' AND dependency_manifest_digest REGEXP '^[0-9a-f]{64}$' AND ((to_block_number IS NULL AND to_transaction_index IS NULL AND to_tx_log_ordinal_exclusive IS NULL) OR (to_block_number IS NOT NULL AND to_transaction_index IS NOT NULL AND to_tx_log_ordinal_exclusive IS NOT NULL))) ENFORCED
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS chain_ingestion_cursor (
  chain_id BIGINT UNSIGNED NOT NULL,
  archived_through_block BIGINT UNSIGNED NOT NULL,
  archived_through_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  last_tx_index INT UNSIGNED NOT NULL,
  last_log_index INT UNSIGNED NOT NULL,
  lease_token CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  fence_epoch BIGINT UNSIGNED NOT NULL,
  status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  last_error VARCHAR(256) NULL,
  updated_at DATETIME(6) NOT NULL,
  cursor_generation BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (chain_id),
  CONSTRAINT chk_chain_cursor CHECK (chain_id>0 AND archived_through_block_hash REGEXP '^0x[0-9a-f]{64}$' AND cursor_generation>0) ENFORCED
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS source_materialization_cursor (
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  materialized_through_block BIGINT UNSIGNED NOT NULL,
  materialized_through_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  materialized_through_tx_index INT UNSIGNED NOT NULL,
  materialized_through_tx_log_ordinal INT UNSIGNED NOT NULL,
  last_event_identity_id BIGINT UNSIGNED NULL,
  lease_token CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  fence_epoch BIGINT UNSIGNED NOT NULL,
  status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  last_error VARCHAR(256) NULL,
  updated_at DATETIME(6) NOT NULL,
  cursor_generation BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (source_id),
  CONSTRAINT fk_materialization_cursor_source FOREIGN KEY (source_id) REFERENCES chain_contract_source(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_materialization_cursor_revision_same_source FOREIGN KEY (source_revision_id,source_id) REFERENCES chain_contract_source_revision(id,source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_materialization_cursor CHECK (cursor_generation>0) ENFORCED
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS source_ingestion_coverage (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  source_id BIGINT UNSIGNED NOT NULL,
  from_block BIGINT UNSIGNED NOT NULL,
  through_block BIGINT UNSIGNED NOT NULL,
  coverage_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  finalized TINYINT UNSIGNED NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_source_coverage_range (source_id,from_block,through_block),
  CONSTRAINT fk_source_coverage_source FOREIGN KEY (source_id) REFERENCES chain_contract_source(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_source_coverage CHECK (from_block<=through_block AND coverage_digest REGEXP '^[0-9a-f]{64}$' AND finalized IN (0,1)) ENFORCED
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS chain_event_identity_v2 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  chain_id BIGINT UNSIGNED NOT NULL,
  tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tx_log_ordinal INT UNSIGNED NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_ce_identity (chain_id,tx_hash,tx_log_ordinal),
  CONSTRAINT chk_ce_identity CHECK (chain_id>0 AND tx_hash REGEXP '^0x[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS chain_event_placement_v2 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  event_identity_id BIGINT UNSIGNED NOT NULL,
  generation INT UNSIGNED NOT NULL,
  block_number BIGINT UNSIGNED NOT NULL,
  block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  transaction_index INT UNSIGNED NOT NULL,
  rpc_log_index INT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  canonical_slot TINYINT UNSIGNED NULL,
  finalized_at DATETIME(6) NULL,
  noncanonical_at DATETIME(6) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_ce_placement_generation (event_identity_id,generation),
  UNIQUE KEY uk_ce_placement_identity_pair (id,event_identity_id),
  UNIQUE KEY uk_ce_placement_identity_revision (id,event_identity_id,source_revision_id),
  UNIQUE KEY uk_ce_placement_canonical (event_identity_id,canonical_slot),
  CONSTRAINT fk_ce_placement_identity FOREIGN KEY (event_identity_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_ce_placement_source_revision FOREIGN KEY (source_revision_id) REFERENCES chain_contract_source_revision(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_ce_placement CHECK (generation>0 AND block_hash REGEXP '^0x[0-9a-f]{64}$' AND (canonical_slot IS NULL OR canonical_slot=1)) ENFORCED
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS chain_event_placement_observation_v2 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  placement_id BIGINT UNSIGNED NOT NULL,
  event_identity_id BIGINT UNSIGNED NOT NULL,
  provider_identity VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  emitting_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  topics_data_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  raw_log_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  observed_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_ce_observation_provider (placement_id,provider_identity,raw_log_digest),
  UNIQUE KEY uk_ce_observation_identity_pair (id,event_identity_id),
  CONSTRAINT fk_ce_observation_placement_identity FOREIGN KEY (placement_id,event_identity_id) REFERENCES chain_event_placement_v2(id,event_identity_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_ce_observation CHECK (emitting_address REGEXP '^0x[0-9a-f]{40}$' AND topics_data_digest REGEXP '^[0-9a-f]{64}$' AND raw_log_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS chain_event_payload_conflict_v2 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  event_identity_id BIGINT UNSIGNED NOT NULL,
  first_observation_id BIGINT UNSIGNED NOT NULL,
  conflicting_observation_id BIGINT UNSIGNED NOT NULL,
  state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  evidence_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  detected_at DATETIME(6) NOT NULL,
  resolved_at DATETIME(6) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_ce_payload_conflict (event_identity_id,first_observation_id,conflicting_observation_id),
  CONSTRAINT fk_ce_conflict_identity FOREIGN KEY (event_identity_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_ce_conflict_first_observation FOREIGN KEY (first_observation_id,event_identity_id) REFERENCES chain_event_placement_observation_v2(id,event_identity_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_ce_conflict_second_observation FOREIGN KEY (conflicting_observation_id,event_identity_id) REFERENCES chain_event_placement_observation_v2(id,event_identity_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_ce_payload_conflict CHECK (state IN ('open','resolved') AND evidence_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS chain_event_application_attempt_v2 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  event_identity_id BIGINT UNSIGNED NOT NULL,
  placement_id BIGINT UNSIGNED NOT NULL,
  effect_version INT UNSIGNED NOT NULL,
  expected_effect TINYINT UNSIGNED NOT NULL,
  correlation_sender_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NULL,
  correlation_calldata_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  correlation_nonce BIGINT UNSIGNED NULL,
  attempt_no INT UNSIGNED NOT NULL,
  predecessor_application_id BIGINT UNSIGNED NULL,
  requested_by_replay_audit_id BIGINT UNSIGNED NULL,
  disposition VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  effect_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  applied_at DATETIME(6) NULL,
  reversed_at DATETIME(6) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_ce_application_attempt (event_identity_id,placement_id,effect_version,attempt_no),
  UNIQUE KEY uk_ce_application_identity_pair (id,event_identity_id),
  UNIQUE KEY uk_ce_application_identity_placement (id,event_identity_id,placement_id),
  CONSTRAINT fk_ce_application_identity FOREIGN KEY (event_identity_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_ce_application_placement_identity FOREIGN KEY (placement_id,event_identity_id) REFERENCES chain_event_placement_v2(id,event_identity_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_ce_application_predecessor_same_identity FOREIGN KEY (predecessor_application_id,event_identity_id) REFERENCES chain_event_application_attempt_v2(id,event_identity_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_ce_application CHECK (effect_version>0 AND expected_effect IN (0,1) AND attempt_no>0 AND ((correlation_sender_address IS NULL AND correlation_calldata_digest IS NULL AND correlation_nonce IS NULL) OR (correlation_sender_address REGEXP '^0x[0-9a-f]{40}$' AND correlation_calldata_digest REGEXP '^[0-9a-f]{64}$' AND correlation_nonce IS NOT NULL)) AND disposition IN ('exact','conditional','blocked','no_op','reversed') AND effect_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS chain_event_identity_quarantine_v2 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  legacy_table VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  legacy_id BIGINT UNSIGNED NOT NULL,
  raw_identity_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  failure_reason VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tool_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  adjudication_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_ce_quarantine_legacy (legacy_table,legacy_id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS fund_ledger_v2_shadow (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  source_application_attempt_id BIGINT UNSIGNED NOT NULL,
  entry_ordinal INT UNSIGNED NOT NULL,
  effect_manifest_version INT UNSIGNED NOT NULL,
  tx_log_ordinal INT UNSIGNED NOT NULL,
  rpc_log_index INT UNSIGNED NOT NULL,
  token_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  amount_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  counterparty_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  reversal_of_ledger_id BIGINT UNSIGNED NULL,
  compensation_generation INT UNSIGNED NULL,
  row_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_fund_ledger_v2_entry (source_application_attempt_id,entry_ordinal),
  UNIQUE KEY uk_fund_ledger_v2_reversal (reversal_of_ledger_id,compensation_generation),
  CONSTRAINT fk_fund_ledger_v2_application FOREIGN KEY (source_application_attempt_id) REFERENCES chain_event_application_attempt_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fund_ledger_v2_reversal FOREIGN KEY (reversal_of_ledger_id) REFERENCES fund_ledger_v2_shadow(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fund_ledger_v2 CHECK (effect_manifest_version>0 AND amount_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(amount_raw)<78 OR amount_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND row_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS ce_v2_cutover_control (
  chain_id BIGINT UNSIGNED NOT NULL,
  phase VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_generation BIGINT UNSIGNED NOT NULL,
  reader_generation BIGINT UNSIGNED NOT NULL,
  cutover_block_number BIGINT UNSIGNED NULL,
  cutover_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL,
  validation_query_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  validation_result_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  quarantine_count BIGINT UNSIGNED NOT NULL,
  unresolved_failure_count BIGINT UNSIGNED NOT NULL,
  source_coverage_gap_count BIGINT UNSIGNED NOT NULL,
  approved_manifest_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  PRIMARY KEY (chain_id),
  CONSTRAINT chk_ce_cutover CHECK (phase IN ('m1_expand','m2_backfill','m3_shadow_write','m4_shadow_read','m5_fenced','m6_v2_read','m7_v2_only','m8_retired') AND NOT(phase IN ('m5_fenced','m6_v2_read','m7_v2_only','m8_retired') AND (quarantine_count<>0 OR unresolved_failure_count<>0 OR source_coverage_gap_count<>0 OR cutover_block_hash IS NULL OR validation_result_digest IS NULL OR approved_manifest_digest IS NULL))) ENFORCED
) ENGINE=InnoDB;
-- M1-only nullable additions; legacy final_event_record_id remains authoritative through M5.
DROP PROCEDURE IF EXISTS jc_fl_exec_if_missing;
DELIMITER $$
CREATE PROCEDURE jc_fl_exec_if_missing(IN p_kind VARCHAR(16), IN p_table VARCHAR(64), IN p_name VARCHAR(64), IN p_sql LONGTEXT)
BEGIN
  IF (p_kind='column' AND NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=p_table AND COLUMN_NAME=p_name))
     OR (p_kind='constraint' AND NOT EXISTS (SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME=p_table AND CONSTRAINT_NAME=p_name)) THEN
    SET @jc_fl_ddl=p_sql; PREPARE jc_fl_stmt FROM @jc_fl_ddl; EXECUTE jc_fl_stmt; DEALLOCATE PREPARE jc_fl_stmt;
  END IF;
END$$
DELIMITER ;
CALL jc_fl_exec_if_missing('column','chain_event_record','chain_id_v2','ALTER TABLE chain_event_record ADD COLUMN chain_id_v2 BIGINT UNSIGNED NULL;');
CALL jc_fl_exec_if_missing('column','chain_event_record','tx_log_ordinal_v2','ALTER TABLE chain_event_record ADD COLUMN tx_log_ordinal_v2 INT UNSIGNED NULL;');
CALL jc_fl_exec_if_missing('column','chain_event_record','event_identity_id_v2','ALTER TABLE chain_event_record ADD COLUMN event_identity_id_v2 BIGINT UNSIGNED NULL;');
CALL jc_fl_exec_if_missing('column','chain_event_record','placement_id_v2','ALTER TABLE chain_event_record ADD COLUMN placement_id_v2 BIGINT UNSIGNED NULL;');
CALL jc_fl_exec_if_missing('column','chain_event_record','application_attempt_id_v2','ALTER TABLE chain_event_record ADD COLUMN application_attempt_id_v2 BIGINT UNSIGNED NULL;');
CALL jc_fl_exec_if_missing('column','chain_event_failure','event_identity_id_v2','ALTER TABLE chain_event_failure ADD COLUMN event_identity_id_v2 BIGINT UNSIGNED NULL;');
CALL jc_fl_exec_if_missing('column','chain_event_failure','placement_id_v2','ALTER TABLE chain_event_failure ADD COLUMN placement_id_v2 BIGINT UNSIGNED NULL;');
CALL jc_fl_exec_if_missing('column','chain_event_failure','source_revision_id_v2','ALTER TABLE chain_event_failure ADD COLUMN source_revision_id_v2 BIGINT UNSIGNED NULL;');
CALL jc_fl_exec_if_missing('column','chain_event_failure','failure_generation_v2','ALTER TABLE chain_event_failure ADD COLUMN failure_generation_v2 INT UNSIGNED NULL;');
CALL jc_fl_exec_if_missing('column','chain_event_replay_audit','requested_placement_generation_v2','ALTER TABLE chain_event_replay_audit ADD COLUMN requested_placement_generation_v2 INT UNSIGNED NULL;');
CALL jc_fl_exec_if_missing('column','chain_event_replay_audit','effect_version_v2','ALTER TABLE chain_event_replay_audit ADD COLUMN effect_version_v2 INT UNSIGNED NULL;');
CALL jc_fl_exec_if_missing('column','chain_tx_submission','final_event_application_id_v2','ALTER TABLE chain_tx_submission ADD COLUMN final_event_application_id_v2 BIGINT UNSIGNED NULL;');
CALL jc_fl_exec_if_missing('column','chain_tx_submission','expected_final_effect_version_v2','ALTER TABLE chain_tx_submission ADD COLUMN expected_final_effect_version_v2 INT UNSIGNED NULL;');
CREATE TABLE IF NOT EXISTS governance_approval (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  approval_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  initiator_id BIGINT UNSIGNED NOT NULL,
  approver_id BIGINT UNSIGNED NOT NULL,
  expected_selector CHAR(10) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  canonical_payload_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_governance_approval_digest (approval_digest),
  CONSTRAINT chk_governance_approval CHECK (initiator_id<>approver_id AND approval_digest REGEXP '^[0-9a-f]{64}$' AND expected_selector REGEXP '^0x[0-9a-f]{8}$' AND canonical_payload_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS governance_safe_nonce_slot (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  chain_id BIGINT UNSIGNED NOT NULL,
  safe_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  safe_nonce BIGINT UNSIGNED NOT NULL,
  owner_set_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  threshold_snapshot INT UNSIGNED NOT NULL,
  state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  canonical_safe_tx_id BIGINT UNSIGNED NULL,
  conflict_detected_at DATETIME(6) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_safe_nonce_slot (chain_id,safe_address,safe_nonce),
  CONSTRAINT chk_safe_nonce_slot CHECK (threshold_snapshot>0 AND state IN ('open','conflicted','consumed','reconciling')) ENFORCED
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS governance_safe_tx (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  nonce_slot_id BIGINT UNSIGNED NOT NULL,
  approval_id BIGINT UNSIGNED NOT NULL,
  safe_tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  canonical_payload_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  state VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  canonical_winner_slot TINYINT UNSIGNED NULL,
  external_execution_tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL,
  executed_safe_tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL,
  execution_application_id BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_safe_tx_hash (nonce_slot_id,safe_tx_hash),
  UNIQUE KEY uk_safe_tx_slot_pair (id,nonce_slot_id),
  UNIQUE KEY uk_safe_winner (nonce_slot_id,canonical_winner_slot),
  CONSTRAINT fk_safe_tx_slot FOREIGN KEY (nonce_slot_id) REFERENCES governance_safe_nonce_slot(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_safe_tx_approval FOREIGN KEY (approval_id) REFERENCES governance_approval(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_safe_tx_execution_application FOREIGN KEY (execution_application_id) REFERENCES chain_event_application_attempt_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_safe_tx CHECK (state IN ('draft','proposed','collecting_signatures','threshold_reached','submitted','broadcast_unknown','executed','applied','rejected','expired','superseded','execution_failed','reorged','reconciling') AND (canonical_winner_slot IS NULL OR canonical_winner_slot=1)) ENFORCED
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS governance_safe_signature (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  safe_tx_id BIGINT UNSIGNED NOT NULL,
  owner_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  signature MEDIUMBLOB NOT NULL,
  signature_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  observed_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_safe_owner_signature (safe_tx_id,owner_address),
  CONSTRAINT fk_safe_signature_tx FOREIGN KEY (safe_tx_id) REFERENCES governance_safe_tx(id) ON UPDATE RESTRICT ON DELETE RESTRICT
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS governance_timelock_operation (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  chain_id BIGINT UNSIGNED NOT NULL,
  timelock_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  safe_tx_id BIGINT UNSIGNED NOT NULL,
  operation_id VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  operation_kind VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  predecessor VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  salt VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  expected_call_count INT UNSIGNED NOT NULL,
  delay_seconds BIGINT UNSIGNED NOT NULL,
  state VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  schedule_block_timestamp BIGINT UNSIGNED NULL,
  ready_at BIGINT UNSIGNED NULL,
  schedule_application_id BIGINT UNSIGNED NULL,
  execution_application_id BIGINT UNSIGNED NULL,
  target_effect_application_id BIGINT UNSIGNED NULL,
  target_expected_effect_version INT UNSIGNED NOT NULL,
  canonical_operation_payload_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  recomputed_operation_id VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  recomputed_operation_payload_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  is_operation_done TINYINT UNSIGNED NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_timelock_operation (chain_id,timelock_address,operation_id),
  CONSTRAINT fk_timelock_safe_tx FOREIGN KEY (safe_tx_id) REFERENCES governance_safe_tx(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_timelock_schedule_application FOREIGN KEY (schedule_application_id) REFERENCES chain_event_application_attempt_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_timelock_execution_application FOREIGN KEY (execution_application_id) REFERENCES chain_event_application_attempt_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_timelock_target_application FOREIGN KEY (target_effect_application_id) REFERENCES chain_event_application_attempt_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_timelock_operation CHECK (expected_call_count>0 AND operation_kind IN ('single','batch') AND state IN ('planned','safe_approved','scheduling','scheduled','ready','executing','executed','applied','cancelling','cancelled','blocked','reorged','reconciling') AND ((schedule_block_timestamp IS NULL AND ready_at IS NULL) OR ready_at=schedule_block_timestamp+delay_seconds)) ENFORCED
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS governance_timelock_call (
  operation_row_id BIGINT UNSIGNED NOT NULL,
  call_index INT UNSIGNED NOT NULL,
  target VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  value_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  calldata_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  canonical_call_payload_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  expected_schedule_effect_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  expected_execute_effect_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  scheduled_application_id BIGINT UNSIGNED NULL,
  executed_application_id BIGINT UNSIGNED NULL,
  PRIMARY KEY (operation_row_id,call_index),
  CONSTRAINT fk_timelock_call FOREIGN KEY (operation_row_id) REFERENCES governance_timelock_operation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_timelock_call_scheduled_application FOREIGN KEY (scheduled_application_id) REFERENCES chain_event_application_attempt_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_timelock_call_executed_application FOREIGN KEY (executed_application_id) REFERENCES chain_event_application_attempt_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_timelock_call_value_uint256 CHECK (value_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(value_raw)<78 OR value_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935')) ENFORCED
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS governance_executor_submission (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  timelock_operation_row_id BIGINT UNSIGNED NOT NULL,
  attempt_no INT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  executor_eoa VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  action VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  state VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  nonce BIGINT UNSIGNED NULL,
  raw_tx_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL,
  active_slot TINYINT UNSIGNED NULL,
  execution_application_id BIGINT UNSIGNED NULL,
  external_execution_application_id BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_executor_attempt (timelock_operation_row_id,attempt_no),
  UNIQUE KEY uk_executor_submission_sender_pair (id,chain_id,executor_eoa),
  UNIQUE KEY uk_executor_sender_nonce (chain_id,executor_eoa,nonce),
  UNIQUE KEY uk_executor_active (chain_id,executor_eoa,active_slot),
  CONSTRAINT fk_executor_operation FOREIGN KEY (timelock_operation_row_id) REFERENCES governance_timelock_operation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_executor_execution_application FOREIGN KEY (execution_application_id) REFERENCES chain_event_application_attempt_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_executor_external_application FOREIGN KEY (external_execution_application_id) REFERENCES chain_event_application_attempt_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_executor_submission CHECK (action IN ('execute','executeBatch') AND state IN ('ready','signing_reserved','signed','submitted','broadcast_unknown','mined_success','final_success','executed_event_applied','released','superseded_external','final_revert','ambiguous','reorged','reconciling') AND (active_slot IS NULL OR active_slot=1)) ENFORCED
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS governance_executor_nonce (
  chain_id BIGINT UNSIGNED NOT NULL,
  executor_eoa VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  next_nonce BIGINT UNSIGNED NOT NULL,
  active_submission_id BIGINT UNSIGNED NULL,
  PRIMARY KEY (chain_id,executor_eoa),
  UNIQUE KEY uk_executor_active_submission (active_submission_id),
  CONSTRAINT fk_executor_nonce_active_same_sender FOREIGN KEY (active_submission_id,chain_id,executor_eoa) REFERENCES governance_executor_submission(id,chain_id,executor_eoa) ON UPDATE RESTRICT ON DELETE RESTRICT
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS ce_v2_equivalence_validation (
  chain_id BIGINT UNSIGNED NOT NULL,
  validation_generation BIGINT UNSIGNED NOT NULL,
  invariant_name VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  validation_query_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  expected_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  actual_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  mismatch_count BIGINT UNSIGNED NOT NULL,
  evidence_root_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  result_row_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  checked_through_block BIGINT UNSIGNED NOT NULL,
  checked_through_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY (chain_id,validation_generation,invariant_name),
  CONSTRAINT chk_ce_equivalence_validation CHECK (invariant_name IN ('v1_to_v2_exactly_one','v2_to_v1_exactly_one','failure_applied_disjoint','final_reference_identity_effect','aggregate_digests','source_coverage','ordinal_reconstruction')) ENFORCED
) ENGINE=InnoDB;
-- Static validation views. Production implementation may use equivalent transactional queries/procedures.

CALL jc_fl_exec_if_missing('constraint','source_materialization_cursor','fk_materialization_cursor_last_identity','ALTER TABLE source_materialization_cursor ADD CONSTRAINT fk_materialization_cursor_last_identity FOREIGN KEY (last_event_identity_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT;');
CALL jc_fl_exec_if_missing('constraint','chain_contract_source_revision','fk_source_revision_activation_identity','ALTER TABLE chain_contract_source_revision ADD CONSTRAINT fk_source_revision_activation_identity FOREIGN KEY (activation_event_identity_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT;');
CALL jc_fl_exec_if_missing('constraint','chain_event_record','fk_chain_event_record_identity_v2','ALTER TABLE chain_event_record ADD CONSTRAINT fk_chain_event_record_identity_v2 FOREIGN KEY (event_identity_id_v2) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT;');
CALL jc_fl_exec_if_missing('constraint','chain_event_record','fk_chain_event_record_placement_same_identity_v2','ALTER TABLE chain_event_record ADD CONSTRAINT fk_chain_event_record_placement_same_identity_v2 FOREIGN KEY (placement_id_v2,event_identity_id_v2) REFERENCES chain_event_placement_v2(id,event_identity_id) ON UPDATE RESTRICT ON DELETE RESTRICT;');
CALL jc_fl_exec_if_missing('constraint','chain_event_record','fk_chain_event_record_application_same_lineage_v2','ALTER TABLE chain_event_record ADD CONSTRAINT fk_chain_event_record_application_same_lineage_v2 FOREIGN KEY (application_attempt_id_v2,event_identity_id_v2,placement_id_v2) REFERENCES chain_event_application_attempt_v2(id,event_identity_id,placement_id) ON UPDATE RESTRICT ON DELETE RESTRICT;');
CALL jc_fl_exec_if_missing('constraint','chain_event_failure','fk_chain_event_failure_identity_v2','ALTER TABLE chain_event_failure ADD CONSTRAINT fk_chain_event_failure_identity_v2 FOREIGN KEY (event_identity_id_v2) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT;');
CALL jc_fl_exec_if_missing('constraint','chain_event_failure','fk_chain_event_failure_same_lineage_v2','ALTER TABLE chain_event_failure ADD CONSTRAINT fk_chain_event_failure_same_lineage_v2 FOREIGN KEY (placement_id_v2,event_identity_id_v2,source_revision_id_v2) REFERENCES chain_event_placement_v2(id,event_identity_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT;');
CALL jc_fl_exec_if_missing('constraint','chain_event_application_attempt_v2','fk_ce_application_replay_audit','ALTER TABLE chain_event_application_attempt_v2 ADD CONSTRAINT fk_ce_application_replay_audit FOREIGN KEY (requested_by_replay_audit_id) REFERENCES chain_event_replay_audit(id) ON UPDATE RESTRICT ON DELETE RESTRICT;');
CALL jc_fl_exec_if_missing('constraint','chain_tx_submission','fk_chain_tx_final_application_v2','ALTER TABLE chain_tx_submission ADD CONSTRAINT fk_chain_tx_final_application_v2 FOREIGN KEY (final_event_application_id_v2) REFERENCES chain_event_application_attempt_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT;');
CALL jc_fl_exec_if_missing('constraint','governance_safe_nonce_slot','fk_safe_nonce_canonical_tx_same_slot','ALTER TABLE governance_safe_nonce_slot ADD CONSTRAINT fk_safe_nonce_canonical_tx_same_slot FOREIGN KEY (canonical_safe_tx_id,id) REFERENCES governance_safe_tx(id,nonce_slot_id) ON UPDATE RESTRICT ON DELETE RESTRICT;');
DROP PROCEDURE jc_fl_exec_if_missing;

CREATE OR REPLACE VIEW ce_v2_m5_gate_v AS SELECT c.chain_id FROM ce_v2_cutover_control c JOIN ce_v2_equivalence_validation v ON v.chain_id=c.chain_id AND v.validation_generation=c.writer_generation AND v.validation_query_version=c.validation_query_version AND v.checked_through_block=c.cutover_block_number AND v.checked_through_block_hash=c.cutover_block_hash WHERE c.phase='m5_fenced' AND c.quarantine_count=0 AND c.unresolved_failure_count=0 AND c.source_coverage_gap_count=0 AND c.cutover_block_hash IS NOT NULL AND c.validation_query_version IS NOT NULL AND c.validation_result_digest IS NOT NULL AND c.approved_manifest_digest IS NOT NULL AND v.result_row_digest=SHA2(CONCAT_WS(':',v.invariant_name,v.validation_query_version,v.expected_digest,v.actual_digest,v.mismatch_count,v.evidence_root_digest,v.checked_through_block,v.checked_through_block_hash),256) GROUP BY c.chain_id,c.validation_result_digest HAVING COUNT(DISTINCT v.invariant_name)=7 AND SUM(v.mismatch_count)=0 AND SUM(v.expected_digest<>v.actual_digest)=0 AND c.validation_result_digest=SHA2(CONCAT_WS('|',MAX(CASE WHEN v.invariant_name='v1_to_v2_exactly_one' THEN v.result_row_digest END),MAX(CASE WHEN v.invariant_name='v2_to_v1_exactly_one' THEN v.result_row_digest END),MAX(CASE WHEN v.invariant_name='failure_applied_disjoint' THEN v.result_row_digest END),MAX(CASE WHEN v.invariant_name='final_reference_identity_effect' THEN v.result_row_digest END),MAX(CASE WHEN v.invariant_name='aggregate_digests' THEN v.result_row_digest END),MAX(CASE WHEN v.invariant_name='source_coverage' THEN v.result_row_digest END),MAX(CASE WHEN v.invariant_name='ordinal_reconstruction' THEN v.result_row_digest END)),256);
CREATE OR REPLACE VIEW ce_v2_m6_final_event_gate_v AS SELECT s.id FROM chain_tx_submission s JOIN chain_event_application_attempt_v2 a ON a.id=s.final_event_application_id_v2 AND a.effect_version=s.expected_final_effect_version_v2 JOIN chain_event_identity_v2 i ON i.id=a.event_identity_id AND i.chain_id=s.chain_id AND i.tx_hash=s.tx_hash JOIN chain_event_placement_v2 p ON p.id=a.placement_id AND p.event_identity_id=a.event_identity_id LEFT JOIN chain_event_payload_conflict_v2 c ON c.event_identity_id=a.event_identity_id AND c.state='open' WHERE p.canonical_slot=1 AND p.finalized_at IS NOT NULL AND p.noncanonical_at IS NULL AND a.expected_effect=1 AND a.disposition IN ('exact','conditional') AND a.applied_at IS NOT NULL AND a.reversed_at IS NULL AND a.correlation_sender_address=LOWER(s.sender_address) AND a.correlation_calldata_digest=SHA2(s.call_data,256) AND a.correlation_nonce=s.nonce AND c.id IS NULL;
CREATE OR REPLACE VIEW fund_ledger_v2_provenance_gap_v AS SELECT l.id FROM fund_ledger_v2_shadow l JOIN chain_event_application_attempt_v2 a ON a.id=l.source_application_attempt_id JOIN chain_event_placement_v2 p ON p.id=a.placement_id AND p.event_identity_id=a.event_identity_id JOIN chain_event_identity_v2 i ON i.id=a.event_identity_id WHERE l.tx_log_ordinal<>i.tx_log_ordinal OR l.rpc_log_index<>p.rpc_log_index;
CREATE OR REPLACE VIEW governance_timelock_call_evidence_gap_v AS SELECT o.id FROM governance_timelock_operation o LEFT JOIN governance_timelock_call c ON c.operation_row_id=o.id LEFT JOIN chain_event_application_attempt_v2 scheduled ON scheduled.id=c.scheduled_application_id LEFT JOIN chain_event_application_attempt_v2 executed ON executed.id=c.executed_application_id LEFT JOIN chain_event_placement_v2 sp ON sp.id=scheduled.placement_id AND sp.event_identity_id=scheduled.event_identity_id LEFT JOIN chain_event_placement_v2 ep ON ep.id=executed.placement_id AND ep.event_identity_id=executed.event_identity_id GROUP BY o.id,o.expected_call_count HAVING COUNT(c.call_index)<>o.expected_call_count OR MIN(c.call_index)<>0 OR MAX(c.call_index)<>o.expected_call_count-1 OR COUNT(DISTINCT c.call_index)<>o.expected_call_count OR SUM(scheduled.id IS NULL OR executed.id IS NULL OR sp.canonical_slot<>1 OR sp.finalized_at IS NULL OR sp.noncanonical_at IS NOT NULL OR ep.canonical_slot<>1 OR ep.finalized_at IS NULL OR ep.noncanonical_at IS NOT NULL OR c.canonical_call_payload_digest<>SHA2(CONCAT_WS(':',c.call_index,c.target,c.value_raw,c.calldata_digest),256) OR scheduled.effect_digest<>c.expected_schedule_effect_digest OR executed.effect_digest<>c.expected_execute_effect_digest)>0;
CREATE OR REPLACE VIEW governance_convergence_gate_v AS SELECT o.id FROM governance_timelock_operation o JOIN governance_safe_tx s ON s.id=o.safe_tx_id AND s.canonical_payload_digest=o.canonical_operation_payload_digest JOIN governance_safe_nonce_slot slot ON slot.id=s.nonce_slot_id AND slot.canonical_safe_tx_id=s.id JOIN governance_approval approval ON approval.id=s.approval_id AND approval.canonical_payload_digest=s.canonical_payload_digest JOIN chain_event_application_attempt_v2 safe_execution ON safe_execution.id=s.execution_application_id JOIN chain_event_identity_v2 safe_identity ON safe_identity.id=safe_execution.event_identity_id AND safe_identity.chain_id=slot.chain_id AND safe_identity.tx_hash=s.external_execution_tx_hash JOIN chain_event_placement_v2 safe_place ON safe_place.id=safe_execution.placement_id AND safe_place.event_identity_id=safe_execution.event_identity_id JOIN chain_event_application_attempt_v2 timelock_execution ON timelock_execution.id=o.execution_application_id JOIN chain_event_placement_v2 timelock_place ON timelock_place.id=timelock_execution.placement_id AND timelock_place.event_identity_id=timelock_execution.event_identity_id JOIN chain_event_application_attempt_v2 target ON target.id=o.target_effect_application_id AND target.effect_version=o.target_expected_effect_version JOIN chain_event_placement_v2 target_place ON target_place.id=target.placement_id AND target_place.event_identity_id=target.event_identity_id LEFT JOIN chain_event_payload_conflict_v2 safe_conflict ON safe_conflict.event_identity_id=safe_execution.event_identity_id AND safe_conflict.state='open' LEFT JOIN chain_event_payload_conflict_v2 timelock_conflict ON timelock_conflict.event_identity_id=timelock_execution.event_identity_id AND timelock_conflict.state='open' LEFT JOIN chain_event_payload_conflict_v2 target_conflict ON target_conflict.event_identity_id=target.event_identity_id AND target_conflict.state='open' LEFT JOIN governance_timelock_call_evidence_gap_v gap ON gap.id=o.id JOIN governance_executor_submission executor ON executor.timelock_operation_row_id=o.id AND (executor.execution_application_id=o.execution_application_id OR executor.external_execution_application_id=o.execution_application_id) WHERE s.canonical_winner_slot=1 AND s.executed_safe_tx_hash=s.safe_tx_hash AND o.recomputed_operation_id=o.operation_id AND o.recomputed_operation_payload_digest=o.canonical_operation_payload_digest AND o.is_operation_done=1 AND safe_place.canonical_slot=1 AND safe_place.finalized_at IS NOT NULL AND safe_place.noncanonical_at IS NULL AND timelock_place.canonical_slot=1 AND timelock_place.finalized_at IS NOT NULL AND timelock_place.noncanonical_at IS NULL AND target_place.canonical_slot=1 AND target_place.finalized_at IS NOT NULL AND target_place.noncanonical_at IS NULL AND safe_execution.applied_at IS NOT NULL AND safe_execution.reversed_at IS NULL AND timelock_execution.applied_at IS NOT NULL AND timelock_execution.reversed_at IS NULL AND target.expected_effect=1 AND target.applied_at IS NOT NULL AND target.reversed_at IS NULL AND safe_conflict.id IS NULL AND timelock_conflict.id IS NULL AND target_conflict.id IS NULL AND gap.id IS NULL;
-- CROSS_ROW_RULES_REQUIRED:
-- payload conflict blocks non-blocked applications; application/reversal history is append-only;
-- M5 requires quarantine_count=0 and V1/V2 equivalence; M6 reader switch is atomic;
-- Safe hash and Timelock operation ID are recomputed from exact deployed versions;
-- all Timelock call indexes 0..N-1 and target event application are required;
-- executor signing_reserved follows Gateway journal recovery and never shares Operator nonce.
