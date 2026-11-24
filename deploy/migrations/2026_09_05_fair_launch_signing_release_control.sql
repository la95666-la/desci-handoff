-- JoinCare Fair Launch signing generations, Gateway journal, Source Registry and watermark persistence.
-- ADDITIVE ONLY. No key material is stored. Requires explicit approved migration gate.
SET NAMES utf8mb4;
DROP PROCEDURE IF EXISTS jc_fair_launch_control_0905;
DELIMITER $$
CREATE PROCEDURE jc_fair_launch_control_0905()
BEGIN
  IF COALESCE(@JC_FAIR_LAUNCH_MIGRATION_APPROVED,0)<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch control migration requires approved execution gate';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_contract_source') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='09_04 CE-V2 migration missing';
  END IF;
END$$
DELIMITER ;
CALL jc_fair_launch_control_0905();
DROP PROCEDURE jc_fair_launch_control_0905;

CREATE TABLE IF NOT EXISTS fair_launch_signing_generation (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  submission_id BIGINT UNSIGNED NOT NULL,
  generation INT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  expected_signer_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  nonce BIGINT UNSIGNED NOT NULL,
  signing_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  unsigned_tx_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  raw_tx LONGBLOB NULL,
  raw_tx_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL,
  state VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  predecessor_generation_id BIGINT UNSIGNED NULL,
  committed_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_signing_generation (submission_id,generation),
  UNIQUE KEY uk_fl_signing_identity (id,submission_id,generation,chain_id,expected_signer_address,nonce,signing_digest),
  UNIQUE KEY uk_fl_signing_digest (submission_id,signing_digest),
  UNIQUE KEY uk_fl_raw_tx_digest (raw_tx_digest),
  UNIQUE KEY uk_fl_tx_hash (tx_hash),
  CONSTRAINT fk_fl_signing_submission FOREIGN KEY (submission_id) REFERENCES chain_tx_submission(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_signing_predecessor FOREIGN KEY (predecessor_generation_id) REFERENCES fair_launch_signing_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_signing_generation CHECK (generation>0 AND expected_signer_address REGEXP '^0x[0-9a-f]{40}$' AND signing_digest REGEXP '^[0-9a-f]{64}$' AND state IN ('reserved','gateway_requested','raw_committed','returned','superseded','recovery','rejected')) ENFORCED
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS fair_launch_gateway_signing_journal (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  submission_id BIGINT UNSIGNED NOT NULL,
  signing_generation_id BIGINT UNSIGNED NOT NULL,
  signing_generation INT UNSIGNED NOT NULL,
  journal_generation INT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  expected_signer_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  nonce BIGINT UNSIGNED NOT NULL,
  signing_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  request_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  raw_tx LONGBLOB NULL,
  raw_tx_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL,
  state VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  last_error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_gateway_submission_generation (submission_id,journal_generation),
  UNIQUE KEY uk_fl_gateway_nonce_generation (chain_id,expected_signer_address,nonce,journal_generation),
  UNIQUE KEY uk_fl_gateway_signing_generation (signing_generation_id,journal_generation),
  CONSTRAINT fk_fl_gateway_generation_identity FOREIGN KEY (signing_generation_id,submission_id,signing_generation,chain_id,expected_signer_address,nonce,signing_digest) REFERENCES fair_launch_signing_generation(id,submission_id,generation,chain_id,expected_signer_address,nonce,signing_digest) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_gateway_journal CHECK (signing_generation>0 AND journal_generation>0 AND expected_signer_address REGEXP '^0x[0-9a-f]{40}$' AND signing_digest REGEXP '^[0-9a-f]{64}$' AND request_digest REGEXP '^[0-9a-f]{64}$' AND state IN ('requested','signer_pending','raw_committed','returned','recovery','rejected')) ENFORCED
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS fair_launch_source_registry_binding_quarantine (
  snapshot_id BIGINT UNSIGNED NOT NULL,
  reason VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  quarantined_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(snapshot_id),
  CONSTRAINT chk_fl_registry_binding_quarantine CHECK(CHAR_LENGTH(reason)>0) ENFORCED
) ENGINE=InnoDB;
DROP PROCEDURE IF EXISTS jc_fl_inventory_registry_binding;
DELIMITER $$
CREATE PROCEDURE jc_fl_inventory_registry_binding()
BEGIN
  DECLARE v_missing BIGINT DEFAULT 0;
  IF EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_source_registry_snapshot') THEN
    SELECT 15-COUNT(*) INTO v_missing FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_source_registry_snapshot' AND COLUMN_NAME IN ('canonical_payload','chain_id','proxy_address','implementation_address','implementation_code_hash','abi_digest','source_id','source_revision_id','source_revision_no','runtime_generation','policy_generation','source_registry_generation','config_generation','payload_digest','revision');
    IF v_missing<>0 THEN
      SET @jc_fl_registry_quarantine_sql='INSERT IGNORE INTO fair_launch_source_registry_binding_quarantine(snapshot_id,reason) SELECT id,''legacy snapshot lacks exact source/runtime binding'' FROM fair_launch_source_registry_snapshot'; PREPARE s FROM @jc_fl_registry_quarantine_sql; EXECUTE s; DEALLOCATE PREPARE s;
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='legacy Source Registry snapshot schema quarantined; exact binding repair required';
    END IF;
  END IF;
END$$
DELIMITER ;
CALL jc_fl_inventory_registry_binding();
DROP PROCEDURE jc_fl_inventory_registry_binding;

CREATE TABLE IF NOT EXISTS fair_launch_source_registry_snapshot (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  environment VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  revision BIGINT UNSIGNED NOT NULL,
  canonical_payload LONGTEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  payload_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  envelope_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  proxy_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  implementation_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  implementation_code_hash CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  abi_digest CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  source_revision_no INT UNSIGNED NOT NULL,
  runtime_generation BIGINT UNSIGNED NOT NULL,
  policy_generation BIGINT UNSIGNED NOT NULL,
  source_registry_generation BIGINT UNSIGNED NOT NULL,
  config_generation BIGINT UNSIGNED NOT NULL,
  key_set_version BIGINT UNSIGNED NOT NULL,
  valid_from DATETIME(6) NOT NULL,
  valid_until DATETIME(6) NULL,
  accepted_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_source_registry_revision (environment,revision),
  UNIQUE KEY uk_fl_registry_exact(environment,chain_id,proxy_address,implementation_address,implementation_code_hash,abi_digest,source_id,source_revision_id,source_revision_no,revision),
  CONSTRAINT fk_fl_registry_source FOREIGN KEY(source_id) REFERENCES chain_contract_source(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_registry_revision_source FOREIGN KEY(source_revision_id,source_id) REFERENCES chain_contract_source_revision(id,source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_source_registry CHECK (revision>0 AND chain_id>0 AND source_id>0 AND source_revision_id>0 AND source_revision_no>0 AND runtime_generation>0 AND policy_generation>0 AND source_registry_generation>0 AND config_generation>0 AND key_set_version>0 AND JSON_VALID(canonical_payload) AND proxy_address REGEXP '^0x[0-9a-f]{40}$' AND implementation_address REGEXP '^0x[0-9a-f]{40}$' AND implementation_code_hash REGEXP '^0x[0-9a-f]{64}$' AND abi_digest REGEXP '^0x[0-9a-f]{64}$' AND payload_digest REGEXP '^[0-9a-f]{64}$' AND envelope_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB;
-- A legacy/partially applied snapshot table cannot be silently admitted. It is inventoried in the quarantine
-- table and execution stops before exact NOT NULL/FK/index admission; an operator-reviewed repair is required.
-- Source Registry trigger/procedure bodies are installed only by the schema-admin
-- privilege provisioning phase from scripts/fair-launch/source-registry-security-objects.sql.
-- Ordinary migrations intentionally do not create incidental-definer security objects.

-- payload_digest is lowercase SHA-256 of RFC 8785 JCS canonical_payload. MySQL deliberately does not fake JCS;
-- Backend parses and recanonicalizes read-back LONGTEXT. A real MySQL 8 pooled-connection reuse test must prove
-- handler cleanup, rollback behavior, direct-insert denial and no residual session evidence before admission.

CREATE TABLE IF NOT EXISTS fair_launch_feature_watermark (
  environment VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  generation BIGINT UNSIGNED NOT NULL,
  capability_manifest_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_registry_revision BIGINT UNSIGNED NOT NULL,
  source_registry_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  deployment_attestation_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  dependency_manifest_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  key_set_version BIGINT UNSIGNED NOT NULL,
  minimum_backend_job_generation BIGINT UNSIGNED NOT NULL,
  minimum_frontend_generation BIGINT UNSIGNED NOT NULL,
  minimum_admin_generation BIGINT UNSIGNED NOT NULL,
  schema_generation BIGINT UNSIGNED NOT NULL,
  event_model_generation BIGINT UNSIGNED NOT NULL,
  accepted_at DATETIME(6) NOT NULL,
  PRIMARY KEY(environment),
  UNIQUE KEY uk_fl_watermark_generation (environment,generation),
  CONSTRAINT chk_fl_watermark CHECK (generation>0 AND source_registry_revision>0 AND key_set_version>0 AND capability_manifest_digest REGEXP '^[0-9a-f]{64}$' AND source_registry_digest REGEXP '^[0-9a-f]{64}$' AND deployment_attestation_digest REGEXP '^[0-9a-f]{64}$' AND dependency_manifest_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS fair_launch_restore_recovery_attestation (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  environment VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  watermark_generation BIGINT UNSIGNED NOT NULL,
  payload_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  envelope_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  reconciliation_state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  initiator_id BIGINT UNSIGNED NOT NULL,
  approver_id BIGINT UNSIGNED NOT NULL,
  executor_id BIGINT UNSIGNED NOT NULL,
  accepted_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_restore_payload (environment,payload_digest),
  CONSTRAINT chk_fl_restore_attestation CHECK (reconciliation_state IN ('pending','balanced','rejected') AND initiator_id<>approver_id AND initiator_id<>executor_id AND approver_id<>executor_id AND payload_digest REGEXP '^[0-9a-f]{64}$' AND envelope_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB;
