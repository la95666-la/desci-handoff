-- JoinCare Fair Launch BSC mainnet archive recovery provider admission.
-- ADDITIVE ONLY. Creates no provider admission, recovery, lease, cursor or job.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_require_fair_launch_mainnet_recovery_provider_0917;
DELIMITER $$
CREATE PROCEDURE jc_require_fair_launch_mainnet_recovery_provider_0917()
BEGIN
  IF COALESCE(@JC_FAIR_LAUNCH_MAINNET_RECOVERY_PROVIDER_APPROVED,0)<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet recovery provider migration requires approved execution gate';
  END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_mainnet_archive_recovery_receipt_v1') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet recovery provider migration requires 0916';
  END IF;
END$$
DELIMITER ;
CALL jc_require_fair_launch_mainnet_recovery_provider_0917();
DROP PROCEDURE jc_require_fair_launch_mainnet_recovery_provider_0917;

CREATE TABLE IF NOT EXISTS fair_launch_mainnet_recovery_provider_v1 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  activation_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  provider_identity VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  rpc_identity_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  provider_role VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  admission_manifest_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  approval_evidence_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  valid_from DATETIME(6) NOT NULL,
  valid_until DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_mainnet_recovery_provider_identity(activation_row_id,provider_identity),
  UNIQUE KEY uk_fl_mainnet_recovery_provider_digest(activation_row_id,rpc_identity_digest),
  CONSTRAINT fk_fl_mainnet_recovery_provider_activation FOREIGN KEY(activation_row_id,chain_id,source_id,source_revision_id) REFERENCES fair_launch_mainnet_activation_v1(authority_row_id,chain_id,source_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mainnet_recovery_provider CHECK(
    chain_id=56 AND provider_identity REGEXP '^[a-z0-9][a-z0-9._-]{0,63}$' AND rpc_identity_digest REGEXP '^[0-9a-f]{64}$' AND
    provider_role IN('primary','independent') AND admission_manifest_digest REGEXP '^[0-9a-f]{64}$' AND approval_evidence_digest REGEXP '^[0-9a-f]{64}$' AND
    valid_from<valid_until
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

DROP TRIGGER IF EXISTS trg_fl_mainnet_recovery_provider_validate_insert;
DROP TRIGGER IF EXISTS trg_fl_mainnet_recovery_provider_no_update;
DROP TRIGGER IF EXISTS trg_fl_mainnet_recovery_provider_no_delete;
DELIMITER $$
CREATE TRIGGER trg_fl_mainnet_recovery_provider_validate_insert BEFORE INSERT ON fair_launch_mainnet_recovery_provider_v1 FOR EACH ROW BEGIN
  DECLARE v_primary_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin;
  IF NEW.rpc_identity_digest<>LOWER(SHA2(CONCAT('joincare:fair-launch:mainnet-rpc:v1',CHAR(0),NEW.provider_identity),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet recovery provider identity digest mismatch';
  END IF;
  SELECT rpc_identity_digest INTO v_primary_digest FROM fair_launch_mainnet_activation_v1
    WHERE authority_row_id=NEW.activation_row_id AND chain_id=NEW.chain_id AND source_id=NEW.source_id AND source_revision_id=NEW.source_revision_id AND
      valid_from<=NEW.created_at AND valid_until>NEW.created_at;
  IF v_primary_digest IS NULL OR
     (NEW.provider_role='primary' AND NEW.rpc_identity_digest<>v_primary_digest) OR
     (NEW.provider_role='independent' AND NEW.rpc_identity_digest=v_primary_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet recovery provider role mismatch';
  END IF;
END$$
CREATE TRIGGER trg_fl_mainnet_recovery_provider_no_update BEFORE UPDATE ON fair_launch_mainnet_recovery_provider_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_recovery_provider_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_recovery_provider_no_delete BEFORE DELETE ON fair_launch_mainnet_recovery_provider_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_recovery_provider_v1 cannot be deleted'; END$$
DELIMITER ;

DROP TRIGGER IF EXISTS trg_fl_mainnet_recovery_receipt_provider_gate;
DELIMITER $$
CREATE TRIGGER trg_fl_mainnet_recovery_receipt_provider_gate AFTER INSERT ON fair_launch_mainnet_archive_recovery_receipt_v1 FOR EACH ROW BEGIN
  DECLARE v_replacement_approval CHAR(64) CHARACTER SET ascii COLLATE ascii_bin;
  DECLARE v_primary_count BIGINT UNSIGNED DEFAULT 0;
  DECLARE v_independent_count BIGINT UNSIGNED DEFAULT 0;
  IF NEW.reason_code IN('canonical_conflict','rpc_inconsistent') THEN
    SELECT approval_evidence_digest INTO v_replacement_approval FROM fair_launch_mainnet_activation_lease_v1
      WHERE id=NEW.replacement_lease_id AND activation_row_id=NEW.activation_row_id AND chain_id=NEW.chain_id;
    SELECT COUNT(*) INTO v_primary_count FROM fair_launch_mainnet_recovery_provider_v1
      WHERE activation_row_id=NEW.activation_row_id AND chain_id=NEW.chain_id AND source_id=NEW.source_id AND source_revision_id=NEW.source_revision_id AND
        provider_role='primary' AND valid_from<=NEW.created_at AND valid_until>NEW.created_at;
    SELECT COUNT(*) INTO v_independent_count FROM fair_launch_mainnet_recovery_provider_v1
      WHERE activation_row_id=NEW.activation_row_id AND chain_id=NEW.chain_id AND source_id=NEW.source_id AND source_revision_id=NEW.source_revision_id AND
        provider_role='independent' AND valid_from<=NEW.created_at AND valid_until>NEW.created_at;
    IF v_replacement_approval IS NULL OR NEW.approval_evidence_digest<>v_replacement_approval OR v_primary_count<>1 OR v_independent_count<1 OR NEW.verification_count<2 OR NEW.verification_count>v_primary_count+v_independent_count THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet recovery provider evidence unavailable';
    END IF;
  END IF;
END$$
DELIMITER ;
