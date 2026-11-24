-- Executed only by the schema administrator after reviewed migrations.
-- Fixed locked definer and grants are provisioned by provision-source-registry-privileges.sh.
DROP TRIGGER IF EXISTS trg_fl_registry_payload_verify;
DROP TRIGGER IF EXISTS trg_fl_registry_payload_no_update;
DROP PROCEDURE IF EXISTS jc_insert_fair_launch_source_registry_snapshot;
DELIMITER $$
CREATE DEFINER='jc_fl_registry_definer'@'localhost' TRIGGER trg_fl_registry_payload_verify BEFORE INSERT ON fair_launch_source_registry_snapshot FOR EACH ROW
BEGIN
  IF COALESCE(@JC_FAIR_LAUNCH_VERIFIED_REGISTRY_PAYLOAD_DIGEST,'')<>NEW.payload_digest THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='application JCS payload verification evidence missing'; END IF;
  IF JSON_LENGTH(NEW.canonical_payload)<>14 OR NOT (JSON_UNQUOTE(JSON_EXTRACT(NEW.canonical_payload,'$.environment'))<=>NEW.environment) OR NOT (JSON_UNQUOTE(JSON_EXTRACT(NEW.canonical_payload,'$.revision'))<=>CAST(NEW.revision AS CHAR)) OR NOT (JSON_UNQUOTE(JSON_EXTRACT(NEW.canonical_payload,'$.chain_id'))<=>CAST(NEW.chain_id AS CHAR)) OR NOT (JSON_UNQUOTE(JSON_EXTRACT(NEW.canonical_payload,'$.proxy_address'))<=>NEW.proxy_address) OR NOT (JSON_UNQUOTE(JSON_EXTRACT(NEW.canonical_payload,'$.implementation_address'))<=>NEW.implementation_address) OR NOT (JSON_UNQUOTE(JSON_EXTRACT(NEW.canonical_payload,'$.implementation_code_hash'))<=>NEW.implementation_code_hash) OR NOT (JSON_UNQUOTE(JSON_EXTRACT(NEW.canonical_payload,'$.abi_digest'))<=>NEW.abi_digest) OR NOT (JSON_UNQUOTE(JSON_EXTRACT(NEW.canonical_payload,'$.source_id'))<=>CAST(NEW.source_id AS CHAR)) OR NOT (JSON_UNQUOTE(JSON_EXTRACT(NEW.canonical_payload,'$.source_revision_id'))<=>CAST(NEW.source_revision_id AS CHAR)) OR NOT (JSON_UNQUOTE(JSON_EXTRACT(NEW.canonical_payload,'$.source_revision_no'))<=>CAST(NEW.source_revision_no AS CHAR)) OR NOT (JSON_UNQUOTE(JSON_EXTRACT(NEW.canonical_payload,'$.runtime_generation'))<=>CAST(NEW.runtime_generation AS CHAR)) OR NOT (JSON_UNQUOTE(JSON_EXTRACT(NEW.canonical_payload,'$.policy_generation'))<=>CAST(NEW.policy_generation AS CHAR)) OR NOT (JSON_UNQUOTE(JSON_EXTRACT(NEW.canonical_payload,'$.source_registry_generation'))<=>CAST(NEW.source_registry_generation AS CHAR)) OR NOT (JSON_UNQUOTE(JSON_EXTRACT(NEW.canonical_payload,'$.config_generation'))<=>CAST(NEW.config_generation AS CHAR)) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='canonical payload relational binding mismatch'; END IF;
  IF NOT (LOWER(SUBSTRING(NEW.implementation_code_hash,3))<=>(SELECT r.implementation_code_hash FROM chain_contract_source_revision r WHERE r.id=NEW.source_revision_id AND r.source_id=NEW.source_id AND r.revision_no=NEW.source_revision_no)) OR NOT (LOWER(SUBSTRING(NEW.abi_digest,3))<=>(SELECT r.abi_digest FROM chain_contract_source_revision r WHERE r.id=NEW.source_revision_id AND r.source_id=NEW.source_id AND r.revision_no=NEW.source_revision_no)) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prefixed snapshot digest does not match unprefixed source revision'; END IF;
END$$
CREATE DEFINER='jc_fl_registry_definer'@'localhost' TRIGGER trg_fl_registry_payload_no_update BEFORE UPDATE ON fair_launch_source_registry_snapshot FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='accepted Source Registry snapshot is append-only'; END$$
CREATE DEFINER='jc_fl_registry_definer'@'localhost' PROCEDURE jc_insert_fair_launch_source_registry_snapshot(
  IN p_environment VARCHAR(32), IN p_revision BIGINT UNSIGNED,
  IN p_canonical_payload LONGTEXT, IN p_payload_digest CHAR(64), IN p_verified_jcs_digest CHAR(64),
  IN p_envelope_digest CHAR(64), IN p_chain_id BIGINT UNSIGNED,
  IN p_proxy_address CHAR(42), IN p_implementation_address CHAR(42),
  IN p_implementation_code_hash CHAR(66), IN p_abi_digest CHAR(66),
  IN p_source_id BIGINT UNSIGNED, IN p_source_revision_id BIGINT UNSIGNED,
  IN p_source_revision_no INT UNSIGNED, IN p_runtime_generation BIGINT UNSIGNED,
  IN p_policy_generation BIGINT UNSIGNED, IN p_source_registry_generation BIGINT UNSIGNED,
  IN p_config_generation BIGINT UNSIGNED, IN p_key_set_version BIGINT UNSIGNED,
  IN p_valid_from DATETIME(6), IN p_valid_until DATETIME(6), IN p_accepted_at DATETIME(6)
)
SQL SECURITY DEFINER
BEGIN
  DECLARE EXIT HANDLER FOR SQLEXCEPTION
  BEGIN
    SET @JC_FAIR_LAUNCH_VERIFIED_REGISTRY_PAYLOAD_DIGEST = NULL;
    RESIGNAL;
  END;
  IF p_verified_jcs_digest<>p_payload_digest OR p_payload_digest NOT REGEXP '^[0-9a-f]{64}$' THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='verified JCS digest mismatch';
  END IF;
  SET @JC_FAIR_LAUNCH_VERIFIED_REGISTRY_PAYLOAD_DIGEST = p_verified_jcs_digest;
  INSERT INTO fair_launch_source_registry_snapshot(
    environment,revision,canonical_payload,payload_digest,envelope_digest,chain_id,
    proxy_address,implementation_address,implementation_code_hash,abi_digest,
    source_id,source_revision_id,source_revision_no,runtime_generation,policy_generation,
    source_registry_generation,config_generation,key_set_version,valid_from,valid_until,accepted_at
  ) VALUES (
    p_environment,p_revision,p_canonical_payload,p_payload_digest,p_envelope_digest,p_chain_id,
    p_proxy_address,p_implementation_address,p_implementation_code_hash,p_abi_digest,
    p_source_id,p_source_revision_id,p_source_revision_no,p_runtime_generation,p_policy_generation,
    p_source_registry_generation,p_config_generation,p_key_set_version,p_valid_from,p_valid_until,p_accepted_at
  );
  SET @JC_FAIR_LAUNCH_VERIFIED_REGISTRY_PAYLOAD_DIGEST = NULL;
END$$
DELIMITER ;
