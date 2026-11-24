-- JoinCare Fair Launch v3 governance and policy projections.
-- ADDITIVE LOCAL-ONLY candidate. Production Apply remains disabled.
SET NAMES utf8mb4;
DROP PROCEDURE IF EXISTS jc_fair_launch_local_e2e_0910;
DELIMITER $$
CREATE PROCEDURE jc_fair_launch_local_e2e_0910()
BEGIN
  IF COALESCE(@JC_FAIR_LAUNCH_LOCAL_E2E_APPROVED,0)<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch local E2E migration requires local-only execution gate'; END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_project_state_v3') THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='09_09 Fair Launch financial projection migration missing'; END IF;
END$$
DELIMITER ;
CALL jc_fair_launch_local_e2e_0910();
DROP PROCEDURE jc_fair_launch_local_e2e_0910;

CREATE TABLE IF NOT EXISTS fair_launch_governance_current_v3 (
  source_id BIGINT UNSIGNED NOT NULL,
  run_row_id BIGINT UNSIGNED NOT NULL,
  latest_source_revision_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  contract_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  operator_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  pause_guardian_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  unpause_timelock_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  treasury_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform_fee_receiver CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform_fee_bps SMALLINT UNSIGNED NOT NULL,
  contributions_paused TINYINT UNSIGNED NOT NULL,
  payouts_paused TINYINT UNSIGNED NOT NULL,
  claims_pause_until BIGINT UNSIGNED NOT NULL,
  state_generation BIGINT UNSIGNED NOT NULL,
  last_event_identity_id BIGINT UNSIGNED NULL,
  last_placement_id BIGINT UNSIGNED NULL,
  last_source_revision_id BIGINT UNSIGNED NULL,
  last_effect_manifest_version INT UNSIGNED NULL,
  last_application_attempt_id BIGINT UNSIGNED NULL,
  source_application_lineage_id BIGINT UNSIGNED NULL,
  state_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY(source_id),
  CONSTRAINT fk_fl_gov_current_run_chain FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_local_e2e_run(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_gov_current_source_chain FOREIGN KEY(source_id,chain_id) REFERENCES chain_contract_source(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_gov_current_revision FOREIGN KEY(latest_source_revision_id,source_id) REFERENCES chain_contract_source_revision(id,source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_gov_current_event FOREIGN KEY(last_event_identity_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_gov_current_placement FOREIGN KEY(last_placement_id,last_event_identity_id) REFERENCES chain_event_placement_v2(id,event_identity_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_gov_current_application FOREIGN KEY(last_application_attempt_id,last_event_identity_id,last_placement_id) REFERENCES chain_event_application_attempt_v2(id,event_identity_id,placement_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_gov_current CHECK(
    chain_id=31337 AND contract_address REGEXP '^0x[0-9a-f]{40}$' AND operator_address REGEXP '^0x[0-9a-f]{40}$' AND
    pause_guardian_address REGEXP '^0x[0-9a-f]{40}$' AND unpause_timelock_address REGEXP '^0x[0-9a-f]{40}$' AND
    treasury_address REGEXP '^0x[0-9a-f]{40}$' AND platform_fee_receiver REGEXP '^0x[0-9a-f]{40}$' AND
    platform_fee_bps<=1000 AND contributions_paused IN(0,1) AND payouts_paused IN(0,1) AND state_generation>0 AND state_digest REGEXP '^[0-9a-f]{64}$' AND
    ((last_event_identity_id IS NULL AND last_placement_id IS NULL AND last_source_revision_id IS NULL AND last_effect_manifest_version IS NULL AND last_application_attempt_id IS NULL AND source_application_lineage_id IS NULL) OR
     (last_event_identity_id IS NOT NULL AND last_placement_id IS NOT NULL AND last_source_revision_id IS NOT NULL AND last_effect_manifest_version>0 AND last_application_attempt_id IS NOT NULL AND source_application_lineage_id IS NOT NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_governance_history_v3 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  state_generation BIGINT UNSIGNED NOT NULL,
  before_payload_jcs LONGTEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  before_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  after_payload_jcs LONGTEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  after_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_event_identity_id BIGINT UNSIGNED NOT NULL,
  source_placement_id BIGINT UNSIGNED NOT NULL,
  source_application_attempt_id BIGINT UNSIGNED NOT NULL,
  effect_manifest_version INT UNSIGNED NOT NULL,
  source_application_lineage_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_gov_history_generation(source_id,state_generation),
  CONSTRAINT fk_fl_gov_history_run_chain FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_local_e2e_run(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_gov_history_source_chain FOREIGN KEY(source_id,chain_id) REFERENCES chain_contract_source(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_gov_history_revision FOREIGN KEY(source_revision_id,source_id) REFERENCES chain_contract_source_revision(id,source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_gov_history_event FOREIGN KEY(source_event_identity_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_gov_history_placement FOREIGN KEY(source_placement_id,source_event_identity_id,source_revision_id) REFERENCES chain_event_placement_v2(id,event_identity_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_gov_history_application FOREIGN KEY(source_application_attempt_id,source_event_identity_id,source_placement_id) REFERENCES chain_event_application_attempt_v2(id,event_identity_id,placement_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_gov_history CHECK(chain_id=31337 AND state_generation>0 AND effect_manifest_version>0 AND JSON_VALID(before_payload_jcs) AND JSON_VALID(after_payload_jcs) AND before_digest REGEXP '^[0-9a-f]{64}$' AND after_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_policy_current_v3 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  latest_source_revision_id BIGINT UNSIGNED NOT NULL,
  policy_kind VARCHAR(40) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  subject_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  policy_version INT UNSIGNED NOT NULL,
  enabled TINYINT UNSIGNED NOT NULL,
  emergency_blocked TINYINT UNSIGNED NOT NULL,
  token_decimals TINYINT UNSIGNED NULL,
  runtime_code_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  policy_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  attestation_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  state_generation BIGINT UNSIGNED NOT NULL,
  last_event_identity_id BIGINT UNSIGNED NOT NULL,
  last_placement_id BIGINT UNSIGNED NOT NULL,
  last_source_revision_id BIGINT UNSIGNED NOT NULL,
  last_effect_manifest_version INT UNSIGNED NOT NULL,
  last_application_attempt_id BIGINT UNSIGNED NOT NULL,
  source_application_lineage_id BIGINT UNSIGNED NOT NULL,
  state_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_policy_current(source_id,policy_kind,subject_address),
  CONSTRAINT fk_fl_policy_current_run_chain FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_local_e2e_run(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_policy_current_source_chain FOREIGN KEY(source_id,chain_id) REFERENCES chain_contract_source(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_policy_current_revision FOREIGN KEY(latest_source_revision_id,source_id) REFERENCES chain_contract_source_revision(id,source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_policy_current_event FOREIGN KEY(last_event_identity_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_policy_current_placement FOREIGN KEY(last_placement_id,last_event_identity_id) REFERENCES chain_event_placement_v2(id,event_identity_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_policy_current_application FOREIGN KEY(last_application_attempt_id,last_event_identity_id,last_placement_id) REFERENCES chain_event_application_attempt_v2(id,event_identity_id,placement_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_policy_current CHECK(
    chain_id=31337 AND policy_kind IN('payment_token_policy','payment_token_admission','payment_token_emergency','project_token_policy','beneficiary_safe','release_policy') AND
    subject_address REGEXP '^0x[0-9a-f]{40}$' AND policy_version>0 AND enabled IN(0,1) AND emergency_blocked IN(0,1) AND
    (token_decimals IS NULL OR token_decimals BETWEEN 1 AND 18) AND (runtime_code_hash IS NULL OR runtime_code_hash REGEXP '^[0-9a-f]{64}$') AND
    policy_digest REGEXP '^[0-9a-f]{64}$' AND (attestation_digest IS NULL OR attestation_digest REGEXP '^[0-9a-f]{64}$') AND state_generation>0 AND last_effect_manifest_version>0 AND state_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_policy_history_v3 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  policy_current_id BIGINT UNSIGNED NOT NULL,
  run_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  state_generation BIGINT UNSIGNED NOT NULL,
  before_payload_jcs LONGTEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  before_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  after_payload_jcs LONGTEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  after_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_event_identity_id BIGINT UNSIGNED NOT NULL,
  source_placement_id BIGINT UNSIGNED NOT NULL,
  source_application_attempt_id BIGINT UNSIGNED NOT NULL,
  effect_manifest_version INT UNSIGNED NOT NULL,
  source_application_lineage_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_policy_history_generation(policy_current_id,state_generation),
  CONSTRAINT fk_fl_policy_history_current FOREIGN KEY(policy_current_id) REFERENCES fair_launch_policy_current_v3(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_policy_history_run_chain FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_local_e2e_run(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_policy_history_source_chain FOREIGN KEY(source_id,chain_id) REFERENCES chain_contract_source(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_policy_history_revision FOREIGN KEY(source_revision_id,source_id) REFERENCES chain_contract_source_revision(id,source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_policy_history_placement FOREIGN KEY(source_placement_id,source_event_identity_id,source_revision_id) REFERENCES chain_event_placement_v2(id,event_identity_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_policy_history_application FOREIGN KEY(source_application_attempt_id,source_event_identity_id,source_placement_id) REFERENCES chain_event_application_attempt_v2(id,event_identity_id,placement_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_policy_history CHECK(chain_id=31337 AND state_generation>0 AND effect_manifest_version>0 AND JSON_VALID(before_payload_jcs) AND JSON_VALID(after_payload_jcs) AND before_digest REGEXP '^[0-9a-f]{64}$' AND after_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
