-- JoinCare Fair Launch LOCAL-ONLY E2E run, source and project binding.
-- ADDITIVE ONLY. Frozen migrations 0903-0907 are not modified.
-- This migration is intentionally absent from every production Apply/release path.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_fair_launch_local_e2e_0908;
DELIMITER $$
CREATE PROCEDURE jc_fair_launch_local_e2e_0908()
BEGIN
  IF COALESCE(@JC_FAIR_LAUNCH_LOCAL_E2E_APPROVED,0)<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch local E2E migration requires local-only execution gate';
  END IF;
  IF @JC_FAIR_LAUNCH_LOCAL_E2E_RUN_ID IS NULL OR @JC_FAIR_LAUNCH_LOCAL_E2E_RUN_ID NOT REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$' THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch local E2E migration requires canonical random run id';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_contract_source_revision') OR
     NOT EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_effect_application') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch local E2E migration requires complete 0903-0907 schema';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND INDEX_NAME='uk_projects_row_project_v3') THEN
    ALTER TABLE projects ADD UNIQUE KEY uk_projects_row_project_v3(id,project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_contract_source' AND INDEX_NAME='uk_fl_source_chain_v3') THEN
    ALTER TABLE chain_contract_source ADD UNIQUE KEY uk_fl_source_chain_v3(id,chain_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_event_identity_v2' AND INDEX_NAME='uk_fl_event_identity_chain_v3') THEN
    ALTER TABLE chain_event_identity_v2 ADD UNIQUE KEY uk_fl_event_identity_chain_v3(id,chain_id);
  END IF;
END$$
DELIMITER ;
CALL jc_fair_launch_local_e2e_0908();
DROP PROCEDURE jc_fair_launch_local_e2e_0908;

CREATE TABLE IF NOT EXISTS fair_launch_local_e2e_run (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  environment VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  exact_rpc_url VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  redis_endpoint VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  database_name VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  mysql_server_uuid CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  mysql_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  container_id CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  owner_label_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  schema_generation BIGINT UNSIGNED NOT NULL,
  baseline_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  migration_manifest_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  schema_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  grant_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_registry_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  hardhat_genesis_hash CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  chain_manifest_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  deployment_manifest_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  runtime_admission_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  closed_at DATETIME(6) NULL,
  state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_local_run_id(run_id),
  UNIQUE KEY uk_fl_local_run_db(container_id,database_name),
  UNIQUE KEY uk_fl_local_run_chain(id,chain_id),
  CONSTRAINT chk_fl_local_run CHECK(
    environment='local-e2e' AND chain_id=31337 AND schema_generation>=12 AND
    run_id REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$' AND
    exact_rpc_url REGEXP '^http://127[.]0[.]0[.]1:[1-9][0-9]{3,4}$' AND
    redis_endpoint REGEXP '^127[.]0[.]0[.]1:[1-9][0-9]{3,4}$' AND
    mysql_server_uuid REGEXP '^[0-9a-f-]{36}$' AND mysql_version REGEXP '^8[.]4[.]' AND
    container_id REGEXP '^[0-9a-f]{64}$' AND owner_label_digest REGEXP '^[0-9a-f]{64}$' AND
    baseline_digest REGEXP '^[0-9a-f]{64}$' AND migration_manifest_digest REGEXP '^[0-9a-f]{64}$' AND
    schema_digest REGEXP '^[0-9a-f]{64}$' AND grant_digest REGEXP '^[0-9a-f]{64}$' AND
    source_registry_digest REGEXP '^[0-9a-f]{64}$' AND hardhat_genesis_hash REGEXP '^0x[0-9a-f]{64}$' AND
    chain_manifest_digest REGEXP '^[0-9a-f]{64}$' AND deployment_manifest_digest REGEXP '^[0-9a-f]{64}$' AND
    runtime_admission_digest REGEXP '^[0-9a-f]{64}$' AND created_at<expires_at AND
    state IN ('created','verified','expired','closed') AND
    ((state='closed' AND closed_at IS NOT NULL) OR (state<>'closed' AND closed_at IS NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_local_e2e_observation (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_row_id BIGINT UNSIGNED NOT NULL,
  observation_kind VARCHAR(40) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  observed_value LONGTEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  value_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  collector_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  observed_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_local_observation(run_row_id,observation_kind,value_digest),
  CONSTRAINT fk_fl_local_observation_run FOREIGN KEY(run_row_id) REFERENCES fair_launch_local_e2e_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_local_observation CHECK(
    value_digest REGEXP '^[0-9a-f]{64}$' AND JSON_VALID(observed_value) AND
    observation_kind IN ('mysql_server','mysql_information_schema','mysql_grants','mysql_container','redis_instance','hardhat_genesis','hardhat_chain_id','chain_manifest','deployment_manifest','runtime_admission','source_registry')
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_source_revision_binding_v3 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  from_block_number BIGINT UNSIGNED NOT NULL,
  from_transaction_index INT UNSIGNED NOT NULL,
  from_tx_log_ordinal_inclusive INT UNSIGNED NOT NULL,
  to_block_number BIGINT UNSIGNED NULL,
  to_transaction_index INT UNSIGNED NULL,
  to_tx_log_ordinal_exclusive INT UNSIGNED NULL,
  source_binding_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_source_binding_run(run_row_id,source_id,source_revision_id),
  UNIQUE KEY uk_fl_source_binding_identity(id,source_id,source_revision_id),
  UNIQUE KEY uk_fl_source_binding_provenance(id,run_row_id,chain_id,source_id,source_revision_id),
  CONSTRAINT fk_fl_source_binding_run_chain FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_local_e2e_run(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_source_binding_source_chain FOREIGN KEY(source_id,chain_id) REFERENCES chain_contract_source(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_source_binding_revision FOREIGN KEY(source_revision_id,source_id) REFERENCES chain_contract_source_revision(id,source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_source_binding CHECK(
    chain_id=31337 AND source_binding_digest REGEXP '^[0-9a-f]{64}$' AND
    ((to_block_number IS NULL AND to_transaction_index IS NULL AND to_tx_log_ordinal_exclusive IS NULL) OR
     (to_block_number IS NOT NULL AND to_transaction_index IS NOT NULL AND to_tx_log_ordinal_exclusive IS NOT NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_project_binding_v3 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_row_id BIGINT UNSIGNED NOT NULL,
  project_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  contract_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  project_id_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_binding_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  project_token_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_token_decimals TINYINT UNSIGNED NOT NULL,
  payment_token_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  payment_token_decimals TINYINT UNSIGNED NOT NULL,
  binding_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_project_binding_identity(chain_id,contract_address,contract_version,project_id),
  UNIQUE KEY uk_fl_project_binding_row(run_row_id,project_row_id),
  UNIQUE KEY uk_fl_project_binding_lineage(id,chain_id,contract_address,contract_version,project_id),
  UNIQUE KEY uk_fl_project_binding_provenance(id,chain_id,source_id,contract_address,contract_version,project_id),
  CONSTRAINT fk_fl_project_binding_run_chain FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_local_e2e_run(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_project_binding_project FOREIGN KEY(project_row_id,project_id) REFERENCES projects(id,project_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_project_binding_source_binding FOREIGN KEY(source_binding_id,run_row_id,chain_id,source_id,source_revision_id) REFERENCES fair_launch_source_revision_binding_v3(id,run_row_id,chain_id,source_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_project_binding_source_chain FOREIGN KEY(source_id,chain_id) REFERENCES chain_contract_source(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_project_binding_revision FOREIGN KEY(source_revision_id,source_id) REFERENCES chain_contract_source_revision(id,source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_project_binding CHECK(
    chain_id=31337 AND contract_version='fair_launch_v1' AND project_id>0 AND project_id_raw=CAST(project_id AS CHAR) AND
    project_id_raw REGEXP '^[1-9][0-9]{0,77}$' AND (CHAR_LENGTH(project_id_raw)<78 OR project_id_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
    contract_address REGEXP '^0x[0-9a-f]{40}$' AND project_token_address REGEXP '^0x[0-9a-f]{40}$' AND payment_token_address REGEXP '^0x[0-9a-f]{40}$' AND
    project_token_decimals BETWEEN 1 AND 18 AND payment_token_decimals BETWEEN 1 AND 18 AND
    contract_address<>'0x0000000000000000000000000000000000000000' AND project_token_address<>'0x0000000000000000000000000000000000000000' AND
    payment_token_address<>'0x0000000000000000000000000000000000000000' AND project_token_address<>payment_token_address AND binding_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
