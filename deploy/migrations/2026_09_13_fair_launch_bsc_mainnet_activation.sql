-- JoinCare Fair Launch controlled test-server / BSC mainnet activation root.
-- ADDITIVE REBASE ONLY. Migrations 0908-0912 and their LOCAL-ONLY evidence
-- remain immutable; this migration introduces a common authority parent so the
-- V3 provenance graph can admit either the historical local-e2e root or one
-- exact, bounded chainId=56 activation root.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_fair_launch_mainnet_0913_preflight;
DELIMITER $$
CREATE PROCEDURE jc_fair_launch_mainnet_0913_preflight()
BEGIN
  IF COALESCE(@JC_FAIR_LAUNCH_MAINNET_ACTIVATION_APPROVED,0)<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch BSC mainnet activation migration requires explicit execution gate';
  END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_local_e2e_run') OR
     NOT EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_governance_ledger_entry_v3') OR
     NOT EXISTS(SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_project_state_v3' AND CONSTRAINT_NAME='chk_fl_project_state' AND CONSTRAINT_TYPE='CHECK') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch BSC mainnet activation migration requires complete 0908-0912 schema';
  END IF;
END$$
DELIMITER ;
CALL jc_fair_launch_mainnet_0913_preflight();
DROP PROCEDURE jc_fair_launch_mainnet_0913_preflight;

CREATE TABLE IF NOT EXISTS fair_launch_authority_v1 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  authority_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  authority_identity VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  environment VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_authority_identity(authority_kind,authority_identity),
  UNIQUE KEY uk_fl_authority_chain(id,chain_id),
  CONSTRAINT chk_fl_authority_v1 CHECK(
    (authority_kind='local_e2e' AND environment='local-e2e' AND chain_id=31337) OR
    (authority_kind='test_server_bsc_mainnet' AND environment='test-server' AND chain_id=56)
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO fair_launch_authority_v1(id,authority_kind,authority_identity,environment,chain_id,created_at)
SELECT id,'local_e2e',run_id,environment,chain_id,created_at
FROM fair_launch_local_e2e_run
ON DUPLICATE KEY UPDATE id=VALUES(id);

ALTER TABLE fair_launch_local_e2e_run
  ADD CONSTRAINT fk_fl_local_run_authority FOREIGN KEY(id,chain_id) REFERENCES fair_launch_authority_v1(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT;

CREATE TABLE IF NOT EXISTS fair_launch_mainnet_activation_v1 (
  authority_row_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  activation_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  environment VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  rpc_genesis_hash CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  rpc_identity_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  proxy_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  proxy_runtime_code_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  implementation_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  implementation_code_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  abi_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  source_start_block BIGINT UNSIGNED NOT NULL,
  source_start_block_hash CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_registry_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  payment_token_allowlist_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_allowlist_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  release_manifest_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  migration_manifest_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  activation_manifest_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  signing_key_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  signature_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  governance_evidence_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  valid_from DATETIME(6) NOT NULL,
  valid_until DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(authority_row_id),
  UNIQUE KEY uk_fl_mainnet_activation_id(activation_id),
  UNIQUE KEY uk_fl_mainnet_activation_chain(authority_row_id,chain_id),
  UNIQUE KEY uk_fl_mainnet_activation_source(authority_row_id,chain_id,source_id,source_revision_id),
  CONSTRAINT fk_fl_mainnet_activation_authority FOREIGN KEY(authority_row_id,chain_id) REFERENCES fair_launch_authority_v1(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_activation_source FOREIGN KEY(source_id,chain_id) REFERENCES chain_contract_source(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_activation_revision FOREIGN KEY(source_revision_id,source_id) REFERENCES chain_contract_source_revision(id,source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mainnet_activation CHECK(
    environment='test-server' AND chain_id=56 AND
    activation_id REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$' AND
    rpc_genesis_hash REGEXP '^0x[0-9a-f]{64}$' AND source_start_block_hash REGEXP '^0x[0-9a-f]{64}$' AND
    proxy_address REGEXP '^0x[0-9a-f]{40}$' AND implementation_address REGEXP '^0x[0-9a-f]{40}$' AND
    proxy_address<>'0x0000000000000000000000000000000000000000' AND implementation_address<>'0x0000000000000000000000000000000000000000' AND
    proxy_runtime_code_hash REGEXP '^[0-9a-f]{64}$' AND implementation_code_hash REGEXP '^[0-9a-f]{64}$' AND abi_digest REGEXP '^[0-9a-f]{64}$' AND
    rpc_identity_digest REGEXP '^[0-9a-f]{64}$' AND source_registry_digest REGEXP '^[0-9a-f]{64}$' AND payment_token_allowlist_digest REGEXP '^[0-9a-f]{64}$' AND
    project_allowlist_digest REGEXP '^[0-9a-f]{64}$' AND release_manifest_digest REGEXP '^[0-9a-f]{64}$' AND migration_manifest_digest REGEXP '^[0-9a-f]{64}$' AND
    activation_manifest_digest REGEXP '^[0-9a-f]{64}$' AND signature_digest REGEXP '^[0-9a-f]{64}$' AND governance_evidence_digest REGEXP '^[0-9a-f]{64}$' AND
    source_start_block>0 AND CHAR_LENGTH(signing_key_id)>0 AND created_at<=valid_from AND valid_from<valid_until
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_mainnet_activation_lease_v1 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  activation_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  lease_generation BIGINT UNSIGNED NOT NULL,
  fence_epoch BIGINT UNSIGNED NOT NULL,
  lease_token_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  component_set_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  approval_evidence_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  valid_from DATETIME(6) NOT NULL,
  valid_until DATETIME(6) NOT NULL,
  status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  active_slot TINYINT UNSIGNED NULL,
  created_at DATETIME(6) NOT NULL,
  revoked_at DATETIME(6) NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_mainnet_lease_generation(activation_row_id,lease_generation),
  UNIQUE KEY uk_fl_mainnet_lease_active(activation_row_id,active_slot),
  CONSTRAINT fk_fl_mainnet_lease_activation FOREIGN KEY(activation_row_id,chain_id) REFERENCES fair_launch_mainnet_activation_v1(authority_row_id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mainnet_lease CHECK(
    chain_id=56 AND lease_generation>0 AND fence_epoch>0 AND
    lease_token_hash REGEXP '^[0-9a-f]{64}$' AND component_set_digest REGEXP '^[0-9a-f]{64}$' AND approval_evidence_digest REGEXP '^[0-9a-f]{64}$' AND
    valid_from<valid_until AND status IN('active','revoked','expired') AND (active_slot IS NULL OR active_slot=1) AND
    ((status='active' AND active_slot=1 AND revoked_at IS NULL) OR (status IN('revoked','expired') AND active_slot IS NULL AND revoked_at IS NOT NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_mainnet_activation_history_v1 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  activation_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  sequence_no BIGINT UNSIGNED NOT NULL,
  previous_state VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NULL,
  state VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_entry_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  entry_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  evidence_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_mainnet_activation_history_sequence(activation_row_id,sequence_no),
  UNIQUE KEY uk_fl_mainnet_activation_history_digest(activation_row_id,entry_digest),
  CONSTRAINT fk_fl_mainnet_activation_history_activation FOREIGN KEY(activation_row_id,chain_id) REFERENCES fair_launch_mainnet_activation_v1(authority_row_id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mainnet_activation_history CHECK(
    chain_id=56 AND sequence_no>0 AND state IN('prepared','admitted','source_activated','wallet_enabled','revoked','expired') AND
    entry_digest REGEXP '^[0-9a-f]{64}$' AND evidence_digest REGEXP '^[0-9a-f]{64}$' AND
    ((sequence_no=1 AND previous_state IS NULL AND previous_entry_digest IS NULL AND state='prepared') OR
     (sequence_no>1 AND previous_state IN('prepared','admitted','source_activated','wallet_enabled') AND previous_entry_digest REGEXP '^[0-9a-f]{64}$'))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_mainnet_component_binding_v1 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  activation_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  component VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  git_commit CHAR(40) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  artifact_sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  artifact_size_bytes BIGINT UNSIGNED NOT NULL,
  build_manifest_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  sbom_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_mainnet_component(activation_row_id,component),
  CONSTRAINT fk_fl_mainnet_component_activation FOREIGN KEY(activation_row_id,chain_id) REFERENCES fair_launch_mainnet_activation_v1(authority_row_id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mainnet_component CHECK(
    chain_id=56 AND component IN('contract','deploy','backend','job','frontend','admin','release_tool','migration_bundle') AND
    git_commit REGEXP '^[0-9a-f]{40}$' AND artifact_sha256 REGEXP '^[0-9a-f]{64}$' AND artifact_size_bytes>0 AND
    build_manifest_digest REGEXP '^[0-9a-f]{64}$' AND sbom_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_mainnet_payment_token_v1 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  activation_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  token_symbol VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  token_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  token_decimals TINYINT UNSIGNED NOT NULL,
  runtime_code_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  policy_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  policy_version INT UNSIGNED NOT NULL,
  max_activation_value_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  binding_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_mainnet_payment_symbol(activation_row_id,token_symbol),
  UNIQUE KEY uk_fl_mainnet_payment_address(activation_row_id,token_address),
  UNIQUE KEY uk_fl_mainnet_payment_identity(activation_row_id,chain_id,token_address,token_decimals),
  CONSTRAINT fk_fl_mainnet_payment_activation FOREIGN KEY(activation_row_id,chain_id) REFERENCES fair_launch_mainnet_activation_v1(authority_row_id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mainnet_payment CHECK(
    chain_id=56 AND token_symbol IN('USDT','USDC') AND token_address REGEXP '^0x[0-9a-f]{40}$' AND token_address<>'0x0000000000000000000000000000000000000000' AND
    token_decimals=18 AND runtime_code_hash REGEXP '^[0-9a-f]{64}$' AND policy_digest REGEXP '^[0-9a-f]{64}$' AND policy_version>0 AND
    max_activation_value_raw REGEXP '^[1-9][0-9]{0,77}$' AND binding_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_mainnet_project_admission_v1 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  activation_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  project_row_id BIGINT UNSIGNED NOT NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  payment_token_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  payment_token_decimals TINYINT UNSIGNED NOT NULL,
  project_token_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_token_decimals TINYINT UNSIGNED NOT NULL,
  project_token_runtime_code_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  max_total_contribution_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  max_wallet_contribution_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  admission_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_mainnet_project_id(activation_row_id,project_id),
  UNIQUE KEY uk_fl_mainnet_project_row(activation_row_id,project_row_id),
  UNIQUE KEY uk_fl_mainnet_project_identity(activation_row_id,chain_id,project_id,payment_token_address,project_token_address),
  CONSTRAINT fk_fl_mainnet_project_activation FOREIGN KEY(activation_row_id,chain_id) REFERENCES fair_launch_mainnet_activation_v1(authority_row_id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_project_payment FOREIGN KEY(activation_row_id,chain_id,payment_token_address,payment_token_decimals) REFERENCES fair_launch_mainnet_payment_token_v1(activation_row_id,chain_id,token_address,token_decimals) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_project_row FOREIGN KEY(project_row_id,project_id) REFERENCES projects(id,project_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mainnet_project CHECK(
    chain_id=56 AND project_id>0 AND payment_token_decimals=18 AND project_token_decimals BETWEEN 1 AND 18 AND
    payment_token_address REGEXP '^0x[0-9a-f]{40}$' AND project_token_address REGEXP '^0x[0-9a-f]{40}$' AND payment_token_address<>project_token_address AND
    project_token_runtime_code_hash REGEXP '^[0-9a-f]{64}$' AND max_total_contribution_raw REGEXP '^[1-9][0-9]{0,77}$' AND
    max_wallet_contribution_raw REGEXP '^[1-9][0-9]{0,77}$' AND admission_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_mainnet_watermark_v1 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  activation_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  watermark_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id BIGINT UNSIGNED NULL,
  event_identity_id BIGINT UNSIGNED NULL,
  evidence_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_mainnet_watermark(activation_row_id,watermark_kind,project_id),
  CONSTRAINT fk_fl_mainnet_watermark_activation FOREIGN KEY(activation_row_id,chain_id) REFERENCES fair_launch_mainnet_activation_v1(authority_row_id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_watermark_event FOREIGN KEY(event_identity_id,chain_id) REFERENCES chain_event_identity_v2(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mainnet_watermark CHECK(
    chain_id=56 AND watermark_kind IN('source_activated','first_project_created','wallet_submission_enabled') AND
    evidence_digest REGEXP '^[0-9a-f]{64}$' AND
    ((watermark_kind='source_activated' AND project_id IS NULL) OR (watermark_kind<>'source_activated' AND project_id IS NOT NULL AND project_id>0))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Existing V3 rows keep their IDs and values. Only their authority parent and
-- chain predicate are generalized; every chainId=56 insert is additionally
-- guarded by the exact activation/project bindings below.
ALTER TABLE fair_launch_source_revision_binding_v3 DROP FOREIGN KEY fk_fl_source_binding_run_chain;
ALTER TABLE fair_launch_project_binding_v3 DROP FOREIGN KEY fk_fl_project_binding_run_chain;
ALTER TABLE fair_launch_project_state_v3 DROP FOREIGN KEY fk_fl_project_state_run_chain;
ALTER TABLE fair_launch_position_v3 DROP FOREIGN KEY fk_fl_position_v3_run_chain;
ALTER TABLE fair_launch_governance_current_v3 DROP FOREIGN KEY fk_fl_gov_current_run_chain;
ALTER TABLE fair_launch_governance_history_v3 DROP FOREIGN KEY fk_fl_gov_history_run_chain;
ALTER TABLE fair_launch_policy_current_v3 DROP FOREIGN KEY fk_fl_policy_current_run_chain;
ALTER TABLE fair_launch_policy_history_v3 DROP FOREIGN KEY fk_fl_policy_history_run_chain;
ALTER TABLE fair_launch_cursor_checkpoint_v3 DROP FOREIGN KEY fk_fl_cursor_checkpoint_run_chain;
ALTER TABLE fair_launch_source_token_binding_v3 DROP FOREIGN KEY fk_fl_source_token_binding_run;
ALTER TABLE fair_launch_governance_ledger_entry_v3 DROP FOREIGN KEY fk_fl_gov_ledger_run_chain;

ALTER TABLE fair_launch_source_revision_binding_v3 ADD CONSTRAINT fk_fl_source_binding_authority FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_authority_v1(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE fair_launch_project_binding_v3 ADD CONSTRAINT fk_fl_project_binding_authority FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_authority_v1(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE fair_launch_project_state_v3 ADD CONSTRAINT fk_fl_project_state_authority FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_authority_v1(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE fair_launch_position_v3 ADD CONSTRAINT fk_fl_position_v3_authority FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_authority_v1(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE fair_launch_governance_current_v3 ADD CONSTRAINT fk_fl_gov_current_authority FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_authority_v1(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE fair_launch_governance_history_v3 ADD CONSTRAINT fk_fl_gov_history_authority FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_authority_v1(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE fair_launch_policy_current_v3 ADD CONSTRAINT fk_fl_policy_current_authority FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_authority_v1(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE fair_launch_policy_history_v3 ADD CONSTRAINT fk_fl_policy_history_authority FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_authority_v1(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE fair_launch_cursor_checkpoint_v3 ADD CONSTRAINT fk_fl_cursor_checkpoint_authority FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_authority_v1(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE fair_launch_source_token_binding_v3 ADD CONSTRAINT fk_fl_source_token_binding_authority FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_authority_v1(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE fair_launch_governance_ledger_entry_v3 ADD CONSTRAINT fk_fl_gov_ledger_authority FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_authority_v1(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT;

DROP PROCEDURE IF EXISTS jc_fl_0913_allow_mainnet_check;
DELIMITER $$
CREATE PROCEDURE jc_fl_0913_allow_mainnet_check(IN p_table VARCHAR(64), IN p_constraint VARCHAR(64))
BEGIN
  DECLARE v_clause LONGTEXT;
  DECLARE v_next LONGTEXT;
  SELECT CHECK_CLAUSE INTO v_clause FROM information_schema.CHECK_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_NAME=p_constraint;
  IF v_clause IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch V3 check constraint missing during activation rebase'; END IF;
  SET v_next=REPLACE(v_clause,'`chain_id` = 31337','`chain_id` in (31337,56)');
  SET v_next=REPLACE(v_next,'chain_id = 31337','chain_id in (31337,56)');
  SET v_next=REPLACE(v_next,'chain_id=31337','chain_id in (31337,56)');
  IF v_next=v_clause THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch V3 check constraint is not the frozen local predicate'; END IF;
  SET v_next=REPLACE(v_next,CONCAT(CHAR(92),CHAR(39)),CHAR(39));
  SET @drop_check=CONCAT('ALTER TABLE `',p_table,'` DROP CHECK `',p_constraint,'`');
  PREPARE stmt FROM @drop_check; EXECUTE stmt; DEALLOCATE PREPARE stmt;
  SET @add_check=CONCAT('ALTER TABLE `',p_table,'` ADD CONSTRAINT `',p_constraint,'` CHECK (',v_next,') ENFORCED');
  PREPARE stmt FROM @add_check; EXECUTE stmt; DEALLOCATE PREPARE stmt;
END$$
DELIMITER ;
CALL jc_fl_0913_allow_mainnet_check('fair_launch_source_revision_binding_v3','chk_fl_source_binding');
CALL jc_fl_0913_allow_mainnet_check('fair_launch_project_binding_v3','chk_fl_project_binding');
CALL jc_fl_0913_allow_mainnet_check('fair_launch_project_state_v3','chk_fl_project_state');
CALL jc_fl_0913_allow_mainnet_check('fair_launch_position_v3','chk_fl_position_v3');
CALL jc_fl_0913_allow_mainnet_check('fair_launch_governance_current_v3','chk_fl_gov_current');
CALL jc_fl_0913_allow_mainnet_check('fair_launch_governance_history_v3','chk_fl_gov_history');
CALL jc_fl_0913_allow_mainnet_check('fair_launch_policy_current_v3','chk_fl_policy_current');
CALL jc_fl_0913_allow_mainnet_check('fair_launch_policy_history_v3','chk_fl_policy_history');
CALL jc_fl_0913_allow_mainnet_check('fair_launch_application_lineage_v3','chk_fl_lineage');
CALL jc_fl_0913_allow_mainnet_check('fair_launch_reversal_serialization_guard_v3','chk_fl_reversal_guard');
CALL jc_fl_0913_allow_mainnet_check('fair_launch_effect_guard_v3','chk_fl_effect_guard');
CALL jc_fl_0913_allow_mainnet_check('fair_launch_effect_journal_v3','chk_fl_journal');
CALL jc_fl_0913_allow_mainnet_check('fair_launch_effect_dependency_v3','chk_fl_dependency');
CALL jc_fl_0913_allow_mainnet_check('fair_launch_effect_attestation_v3','chk_fl_attestation');
CALL jc_fl_0913_allow_mainnet_check('fair_launch_cursor_checkpoint_v3','chk_fl_cursor_checkpoint');
CALL jc_fl_0913_allow_mainnet_check('fair_launch_ledger_entry_v3','chk_fl_ledger_v3');
CALL jc_fl_0913_allow_mainnet_check('fair_launch_source_token_binding_v3','chk_fl_source_token_binding');
CALL jc_fl_0913_allow_mainnet_check('fair_launch_governance_ledger_entry_v3','chk_fl_gov_ledger_v3');
DROP PROCEDURE jc_fl_0913_allow_mainnet_check;

DROP TRIGGER IF EXISTS trg_fl_local_run_v1_authority_insert;
DROP TRIGGER IF EXISTS trg_fl_mainnet_activation_v1_authority_insert;
DROP TRIGGER IF EXISTS trg_fl_authority_v1_no_update;
DROP TRIGGER IF EXISTS trg_fl_authority_v1_no_delete;
DROP TRIGGER IF EXISTS trg_fl_mainnet_activation_v1_no_update;
DROP TRIGGER IF EXISTS trg_fl_mainnet_activation_v1_no_delete;
DROP TRIGGER IF EXISTS trg_fl_mainnet_lease_v1_validate_update;
DROP TRIGGER IF EXISTS trg_fl_mainnet_lease_v1_no_delete;
DROP TRIGGER IF EXISTS trg_fl_mainnet_activation_history_v1_validate_insert;
DROP TRIGGER IF EXISTS trg_fl_mainnet_activation_history_v1_no_update;
DROP TRIGGER IF EXISTS trg_fl_mainnet_activation_history_v1_no_delete;
DROP TRIGGER IF EXISTS trg_fl_mainnet_component_v1_no_update;
DROP TRIGGER IF EXISTS trg_fl_mainnet_component_v1_no_delete;
DROP TRIGGER IF EXISTS trg_fl_mainnet_payment_v1_no_update;
DROP TRIGGER IF EXISTS trg_fl_mainnet_payment_v1_no_delete;
DROP TRIGGER IF EXISTS trg_fl_mainnet_project_v1_no_update;
DROP TRIGGER IF EXISTS trg_fl_mainnet_project_v1_no_delete;
DROP TRIGGER IF EXISTS trg_fl_mainnet_watermark_v1_no_update;
DROP TRIGGER IF EXISTS trg_fl_mainnet_watermark_v1_no_delete;
DROP TRIGGER IF EXISTS trg_fl_source_binding_v3_authority_insert;
DROP TRIGGER IF EXISTS trg_fl_project_binding_v3_authority_insert;
DROP TRIGGER IF EXISTS trg_fl_source_token_binding_v3_authority_insert;
DELIMITER $$
CREATE TRIGGER trg_fl_local_run_v1_authority_insert BEFORE INSERT ON fair_launch_local_e2e_run FOR EACH ROW BEGIN
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch local authority ID is parent allocated'; END IF;
  INSERT INTO fair_launch_authority_v1(authority_kind,authority_identity,environment,chain_id,created_at) VALUES('local_e2e',NEW.run_id,'local-e2e',31337,NEW.created_at);
  SET NEW.id=LAST_INSERT_ID();
END$$
CREATE TRIGGER trg_fl_mainnet_activation_v1_authority_insert BEFORE INSERT ON fair_launch_mainnet_activation_v1 FOR EACH ROW BEGIN
  IF NEW.authority_row_id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet authority ID is parent allocated'; END IF;
  IF NOT EXISTS(SELECT 1 FROM chain_contract_source s JOIN chain_contract_source_revision r ON r.id=NEW.source_revision_id AND r.source_id=s.id WHERE s.id=NEW.source_id AND s.chain_id=NEW.chain_id AND s.contract_address=NEW.proxy_address AND s.contract_version='fair_launch_v1' AND s.funding_mode='fair_launch' AND s.start_block=NEW.source_start_block AND s.enabled=1 AND s.retired_at IS NULL AND r.activation_block_hash=NEW.source_start_block_hash AND r.proxy_kind='uups' AND r.abi_digest=NEW.abi_digest AND r.implementation_address=NEW.implementation_address AND r.implementation_code_hash=NEW.implementation_code_hash AND r.from_block_number=NEW.source_start_block AND r.to_block_number IS NULL) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet activation source identity mismatch'; END IF;
  INSERT INTO fair_launch_authority_v1(authority_kind,authority_identity,environment,chain_id,created_at) VALUES('test_server_bsc_mainnet',NEW.activation_id,'test-server',56,NEW.created_at);
  SET NEW.authority_row_id=LAST_INSERT_ID();
END$$
CREATE TRIGGER trg_fl_authority_v1_no_update BEFORE UPDATE ON fair_launch_authority_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_authority_v1 is immutable'; END$$
CREATE TRIGGER trg_fl_authority_v1_no_delete BEFORE DELETE ON fair_launch_authority_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_authority_v1 cannot be deleted'; END$$
CREATE TRIGGER trg_fl_mainnet_activation_v1_no_update BEFORE UPDATE ON fair_launch_mainnet_activation_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_activation_v1 is immutable'; END$$
CREATE TRIGGER trg_fl_mainnet_activation_v1_no_delete BEFORE DELETE ON fair_launch_mainnet_activation_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_activation_v1 cannot be deleted'; END$$
CREATE TRIGGER trg_fl_mainnet_lease_v1_validate_update BEFORE UPDATE ON fair_launch_mainnet_activation_lease_v1 FOR EACH ROW BEGIN
  IF NOT(OLD.id<=>NEW.id AND OLD.activation_row_id<=>NEW.activation_row_id AND OLD.chain_id<=>NEW.chain_id AND OLD.lease_generation<=>NEW.lease_generation AND OLD.fence_epoch<=>NEW.fence_epoch AND OLD.lease_token_hash<=>NEW.lease_token_hash AND OLD.component_set_digest<=>NEW.component_set_digest AND OLD.approval_evidence_digest<=>NEW.approval_evidence_digest AND OLD.valid_from<=>NEW.valid_from AND OLD.valid_until<=>NEW.valid_until AND OLD.created_at<=>NEW.created_at) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet lease identity is immutable'; END IF;
  IF OLD.status<>'active' OR NEW.status NOT IN('revoked','expired') OR NEW.active_slot IS NOT NULL OR NEW.revoked_at IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid Fair Launch mainnet lease terminal transition'; END IF;
END$$
CREATE TRIGGER trg_fl_mainnet_lease_v1_no_delete BEFORE DELETE ON fair_launch_mainnet_activation_lease_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_activation_lease_v1 cannot be deleted'; END$$
CREATE TRIGGER trg_fl_mainnet_activation_history_v1_validate_insert BEFORE INSERT ON fair_launch_mainnet_activation_history_v1 FOR EACH ROW BEGIN
  IF NEW.sequence_no>1 AND NOT EXISTS(SELECT 1 FROM fair_launch_mainnet_activation_history_v1 h WHERE h.activation_row_id=NEW.activation_row_id AND h.sequence_no=NEW.sequence_no-1 AND h.state=NEW.previous_state AND h.entry_digest=NEW.previous_entry_digest) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet activation history predecessor mismatch'; END IF;
  IF NEW.sequence_no>1 AND NOT((NEW.previous_state='prepared' AND NEW.state='admitted') OR (NEW.previous_state='admitted' AND NEW.state='source_activated') OR (NEW.previous_state='source_activated' AND NEW.state IN('wallet_enabled','revoked','expired')) OR (NEW.previous_state='wallet_enabled' AND NEW.state IN('revoked','expired'))) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid Fair Launch mainnet activation transition'; END IF;
END$$
CREATE TRIGGER trg_fl_mainnet_activation_history_v1_no_update BEFORE UPDATE ON fair_launch_mainnet_activation_history_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_activation_history_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_activation_history_v1_no_delete BEFORE DELETE ON fair_launch_mainnet_activation_history_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_activation_history_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_component_v1_no_update BEFORE UPDATE ON fair_launch_mainnet_component_binding_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_component_binding_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_component_v1_no_delete BEFORE DELETE ON fair_launch_mainnet_component_binding_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_component_binding_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_payment_v1_no_update BEFORE UPDATE ON fair_launch_mainnet_payment_token_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_payment_token_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_payment_v1_no_delete BEFORE DELETE ON fair_launch_mainnet_payment_token_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_payment_token_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_project_v1_no_update BEFORE UPDATE ON fair_launch_mainnet_project_admission_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_project_admission_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_project_v1_no_delete BEFORE DELETE ON fair_launch_mainnet_project_admission_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_project_admission_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_watermark_v1_no_update BEFORE UPDATE ON fair_launch_mainnet_watermark_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_watermark_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_watermark_v1_no_delete BEFORE DELETE ON fair_launch_mainnet_watermark_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_watermark_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_source_binding_v3_authority_insert BEFORE INSERT ON fair_launch_source_revision_binding_v3 FOR EACH ROW BEGIN
  DECLARE v_kind VARCHAR(32);
  SELECT authority_kind INTO v_kind FROM fair_launch_authority_v1 WHERE id=NEW.run_row_id AND chain_id=NEW.chain_id;
  IF (NEW.chain_id=31337 AND v_kind<>'local_e2e') OR (NEW.chain_id=56 AND (v_kind<>'test_server_bsc_mainnet' OR NOT EXISTS(SELECT 1 FROM fair_launch_mainnet_activation_v1 a WHERE a.authority_row_id=NEW.run_row_id AND a.chain_id=NEW.chain_id AND a.source_id=NEW.source_id AND a.source_revision_id=NEW.source_revision_id AND a.source_start_block=NEW.from_block_number))) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch source binding authority mismatch'; END IF;
END$$
CREATE TRIGGER trg_fl_project_binding_v3_authority_insert BEFORE INSERT ON fair_launch_project_binding_v3 FOR EACH ROW BEGIN
  IF NEW.chain_id=56 AND NOT EXISTS(SELECT 1 FROM fair_launch_mainnet_activation_v1 a JOIN fair_launch_mainnet_project_admission_v1 p ON p.activation_row_id=a.authority_row_id AND p.chain_id=a.chain_id WHERE a.authority_row_id=NEW.run_row_id AND a.source_id=NEW.source_id AND a.source_revision_id=NEW.source_revision_id AND a.proxy_address=NEW.contract_address AND p.project_row_id=NEW.project_row_id AND p.project_id=NEW.project_id AND p.payment_token_address=NEW.payment_token_address AND p.payment_token_decimals=NEW.payment_token_decimals AND p.project_token_address=NEW.project_token_address AND p.project_token_decimals=NEW.project_token_decimals) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet project binding is not admitted'; END IF;
END$$
CREATE TRIGGER trg_fl_source_token_binding_v3_authority_insert BEFORE INSERT ON fair_launch_source_token_binding_v3 FOR EACH ROW BEGIN
  IF NEW.chain_id=56 AND NOT EXISTS(SELECT 1 FROM fair_launch_mainnet_payment_token_v1 p WHERE p.activation_row_id=NEW.run_row_id AND p.chain_id=NEW.chain_id AND p.token_address=NEW.token_address AND p.token_decimals=NEW.token_decimals AND p.runtime_code_hash=NEW.runtime_code_hash UNION ALL SELECT 1 FROM fair_launch_mainnet_project_admission_v1 p WHERE p.activation_row_id=NEW.run_row_id AND p.chain_id=NEW.chain_id AND p.project_token_address=NEW.token_address AND p.project_token_decimals=NEW.token_decimals AND p.project_token_runtime_code_hash=NEW.runtime_code_hash) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet source token is not admitted'; END IF;
END$$
DELIMITER ;
