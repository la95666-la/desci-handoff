-- JoinCare Fair Launch v3 canonical typed financial ledgers and immutable enforcement.
-- ADDITIVE LOCAL-ONLY candidate. Production Apply/release remain disabled.
SET NAMES utf8mb4;
DROP PROCEDURE IF EXISTS jc_fair_launch_local_e2e_0912;
DELIMITER $$
CREATE PROCEDURE jc_fair_launch_local_e2e_0912()
BEGIN
  IF COALESCE(@JC_FAIR_LAUNCH_LOCAL_E2E_APPROVED,0)<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch local E2E migration requires local-only execution gate'; END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_application_lineage_v3') THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='09_11 Fair Launch effect lineage migration missing'; END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_contract_source' AND INDEX_NAME='uk_fl_source_id_chain_contract_v3') THEN
    ALTER TABLE chain_contract_source ADD UNIQUE KEY uk_fl_source_id_chain_contract_v3(id,chain_id,contract_address);
  END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_project_state_v3' AND CONSTRAINT_NAME='fk_fl_project_state_lineage') THEN
    ALTER TABLE fair_launch_project_state_v3 ADD CONSTRAINT fk_fl_project_state_lineage FOREIGN KEY(source_application_lineage_id,run_row_id,chain_id,source_id,last_event_identity_id,last_placement_id,last_source_revision_id,last_effect_manifest_version) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_id,event_identity_id,placement_id,source_revision_id,effect_manifest_version) ON UPDATE RESTRICT ON DELETE RESTRICT;
  END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_position_v3' AND CONSTRAINT_NAME='fk_fl_position_v3_lineage') THEN
    ALTER TABLE fair_launch_position_v3 ADD CONSTRAINT fk_fl_position_v3_lineage FOREIGN KEY(source_application_lineage_id,run_row_id,chain_id,source_id,last_event_identity_id,last_placement_id,last_source_revision_id,last_effect_manifest_version) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_id,event_identity_id,placement_id,source_revision_id,effect_manifest_version) ON UPDATE RESTRICT ON DELETE RESTRICT;
  END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_governance_current_v3' AND CONSTRAINT_NAME='fk_fl_gov_current_lineage') THEN
    ALTER TABLE fair_launch_governance_current_v3 ADD CONSTRAINT fk_fl_gov_current_lineage FOREIGN KEY(source_application_lineage_id,run_row_id,chain_id,source_id,last_event_identity_id,last_placement_id,last_source_revision_id,last_effect_manifest_version) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_id,event_identity_id,placement_id,source_revision_id,effect_manifest_version) ON UPDATE RESTRICT ON DELETE RESTRICT;
  END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_governance_history_v3' AND CONSTRAINT_NAME='fk_fl_gov_history_lineage') THEN
    ALTER TABLE fair_launch_governance_history_v3 ADD CONSTRAINT fk_fl_gov_history_lineage FOREIGN KEY(source_application_lineage_id,run_row_id,chain_id,source_id,source_event_identity_id,source_placement_id,source_revision_id,effect_manifest_version) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_id,event_identity_id,placement_id,source_revision_id,effect_manifest_version) ON UPDATE RESTRICT ON DELETE RESTRICT;
  END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_policy_current_v3' AND CONSTRAINT_NAME='fk_fl_policy_current_lineage') THEN
    ALTER TABLE fair_launch_policy_current_v3 ADD CONSTRAINT fk_fl_policy_current_lineage FOREIGN KEY(source_application_lineage_id,run_row_id,chain_id,source_id,last_event_identity_id,last_placement_id,last_source_revision_id,last_effect_manifest_version) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_id,event_identity_id,placement_id,source_revision_id,effect_manifest_version) ON UPDATE RESTRICT ON DELETE RESTRICT;
  END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_policy_history_v3' AND CONSTRAINT_NAME='fk_fl_policy_history_lineage') THEN
    ALTER TABLE fair_launch_policy_history_v3 ADD CONSTRAINT fk_fl_policy_history_lineage FOREIGN KEY(source_application_lineage_id,run_row_id,chain_id,source_id,source_event_identity_id,source_placement_id,source_revision_id,effect_manifest_version) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_id,event_identity_id,placement_id,source_revision_id,effect_manifest_version) ON UPDATE RESTRICT ON DELETE RESTRICT;
  END IF;
END$$
DELIMITER ;
CALL jc_fair_launch_local_e2e_0912();
DROP PROCEDURE jc_fair_launch_local_e2e_0912;

-- Trigger replacement is admitted only on a fresh disposable schema. This is a
-- mechanical maintenance boundary: reapply after any v3 financial/effect row
-- exists is rejected before the first DROP TRIGGER.
DROP PROCEDURE IF EXISTS jc_fl_0912_require_empty_maintenance;
DELIMITER $$
CREATE PROCEDURE jc_fl_0912_require_empty_maintenance()
BEGIN
  DECLARE v_lock INT DEFAULT 0; DECLARE v_rows BIGINT UNSIGNED DEFAULT 0;
  SELECT GET_LOCK(CONCAT('jc_fl_0912:',DATABASE()),10) INTO v_lock;
  IF v_lock<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch 0912 exclusive maintenance lock unavailable'; END IF;
  SELECT (SELECT COUNT(*) FROM fair_launch_application_lineage_v3)+(SELECT COUNT(*) FROM fair_launch_effect_journal_v3)+(SELECT COUNT(*) FROM fair_launch_lineage_reversal_v3)+(SELECT COUNT(*) FROM fair_launch_effect_dependency_v3)+(SELECT COUNT(*) FROM fair_launch_effect_attestation_v3)+(SELECT COUNT(*) FROM fair_launch_cursor_checkpoint_v3) INTO v_rows;
  IF v_rows<>0 THEN DO RELEASE_LOCK(CONCAT('jc_fl_0912:',DATABASE())); SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch 0912 reapply requires fresh disposable schema'; END IF;
END$$
DELIMITER ;
CALL jc_fl_0912_require_empty_maintenance();
DROP PROCEDURE jc_fl_0912_require_empty_maintenance;

CREATE TABLE IF NOT EXISTS fair_launch_ledger_entry_v3 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  project_binding_id BIGINT UNSIGNED NOT NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  contract_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  wallet_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  counterparty_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  entry_type VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  asset_role VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  flow_direction VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  token_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  token_decimals TINYINT UNSIGNED NOT NULL,
  amount_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  amount_display VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_event_identity_id BIGINT UNSIGNED NOT NULL,
  source_placement_id BIGINT UNSIGNED NOT NULL,
  source_application_lineage_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  entry_ordinal INT UNSIGNED NOT NULL,
  effect_manifest_version INT UNSIGNED NOT NULL,
  reversal_of_ledger_id BIGINT UNSIGNED NULL,
  compensation_generation INT UNSIGNED NULL,
  row_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_ledger_forward(source_application_lineage_id,entry_ordinal),
  UNIQUE KEY uk_fl_ledger_compensation(reversal_of_ledger_id),
  KEY idx_fl_ledger_project(chain_id,contract_address,contract_version,project_id,created_at,id),
  KEY idx_fl_ledger_wallet(wallet_address,project_id,created_at,id),
  CONSTRAINT fk_fl_ledger_binding FOREIGN KEY(project_binding_id,chain_id,source_id,contract_address,contract_version,project_id) REFERENCES fair_launch_project_binding_v3(id,chain_id,source_id,contract_address,contract_version,project_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_ledger_lineage FOREIGN KEY(source_application_lineage_id,run_row_id,chain_id,source_id,source_event_identity_id,source_placement_id,source_revision_id,effect_manifest_version) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_id,event_identity_id,placement_id,source_revision_id,effect_manifest_version) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_ledger_reversal FOREIGN KEY(reversal_of_ledger_id) REFERENCES fair_launch_ledger_entry_v3(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_ledger_v3 CHECK(
    chain_id=31337 AND project_id>0 AND contract_version='fair_launch_v1' AND
    contract_address REGEXP '^0x[0-9a-f]{40}$' AND wallet_address REGEXP '^0x[0-9a-f]{40}$' AND counterparty_address REGEXP '^0x[0-9a-f]{40}$' AND token_address REGEXP '^0x[0-9a-f]{40}$' AND
    entry_type IN('project_token_deposited','payment_contributed','project_token_claimed','payment_refunded','project_proceeds_paid','platform_fee_paid','project_tokens_withdrawn','project_token_deposited_reversal','payment_contributed_reversal','project_token_claimed_reversal','payment_refunded_reversal','project_proceeds_paid_reversal','platform_fee_paid_reversal','project_tokens_withdrawn_reversal') AND
    asset_role IN('payment','project') AND flow_direction IN('inflow','outflow','compensation') AND token_decimals BETWEEN 1 AND 18 AND
    amount_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(amount_raw)<78 OR amount_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND row_digest REGEXP '^[0-9a-f]{64}$' AND entry_ordinal>0 AND effect_manifest_version>0 AND
    ((reversal_of_ledger_id IS NULL AND compensation_generation IS NULL AND flow_direction<>'compensation' AND entry_type NOT LIKE '%_reversal') OR
     (reversal_of_ledger_id IS NOT NULL AND compensation_generation>0 AND flow_direction='compensation' AND entry_type LIKE '%_reversal'))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_source_token_binding_v3 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  contract_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  token_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  token_decimals TINYINT UNSIGNED NOT NULL,
  metadata_version INT UNSIGNED NOT NULL,
  admission_generation BIGINT UNSIGNED NOT NULL,
  runtime_code_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  metadata_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  admitted_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_source_token_binding_natural(run_row_id,chain_id,source_id,contract_address,token_address,admission_generation),
  UNIQUE KEY uk_fl_source_token_binding_identity(id,run_row_id,chain_id,source_id,contract_address,token_address,token_decimals),
  CONSTRAINT fk_fl_source_token_binding_run FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_local_e2e_run(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_source_token_binding_source FOREIGN KEY(source_id,chain_id,contract_address) REFERENCES chain_contract_source(id,chain_id,contract_address) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_source_token_binding CHECK(chain_id=31337 AND contract_address REGEXP '^0x[0-9a-f]{40}$' AND token_address REGEXP '^0x[0-9a-f]{40}$' AND token_decimals BETWEEN 1 AND 18 AND metadata_version>0 AND admission_generation>0 AND runtime_code_hash REGEXP '^[0-9a-f]{64}$' AND metadata_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_governance_ledger_entry_v3 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  contract_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  token_binding_id BIGINT UNSIGNED NOT NULL,
  entry_type VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  asset_role VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  flow_direction VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  token_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  receiver_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  token_decimals TINYINT UNSIGNED NOT NULL,
  amount_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  amount_display VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_application_lineage_id BIGINT UNSIGNED NOT NULL,
  source_event_identity_id BIGINT UNSIGNED NOT NULL,
  source_placement_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  entry_ordinal INT UNSIGNED NOT NULL,
  effect_manifest_version INT UNSIGNED NOT NULL,
  reversal_of_ledger_id BIGINT UNSIGNED NULL,
  compensation_generation INT UNSIGNED NULL,
  row_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_gov_ledger_forward(source_application_lineage_id,entry_ordinal),
  UNIQUE KEY uk_fl_gov_ledger_compensation(reversal_of_ledger_id),
  CONSTRAINT fk_fl_gov_ledger_source FOREIGN KEY(source_id,chain_id) REFERENCES chain_contract_source(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_gov_ledger_token_binding FOREIGN KEY(token_binding_id,run_row_id,chain_id,source_id,contract_address,token_address,token_decimals) REFERENCES fair_launch_source_token_binding_v3(id,run_row_id,chain_id,source_id,contract_address,token_address,token_decimals) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_gov_ledger_run_chain FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_local_e2e_run(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_gov_ledger_lineage FOREIGN KEY(source_application_lineage_id,run_row_id,chain_id,source_id,source_event_identity_id,source_placement_id,source_revision_id,effect_manifest_version) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_id,event_identity_id,placement_id,source_revision_id,effect_manifest_version) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_gov_ledger_reversal FOREIGN KEY(reversal_of_ledger_id) REFERENCES fair_launch_governance_ledger_entry_v3(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_gov_ledger_v3 CHECK(chain_id=31337 AND contract_address REGEXP '^0x[0-9a-f]{40}$' AND token_address REGEXP '^0x[0-9a-f]{40}$' AND receiver_address REGEXP '^0x[0-9a-f]{40}$' AND entry_type IN('excess_tokens_recovered','excess_tokens_recovered_reversal') AND asset_role='governance_token' AND flow_direction IN('outflow','compensation') AND token_decimals BETWEEN 1 AND 18 AND amount_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(amount_raw)<78 OR amount_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND row_digest REGEXP '^[0-9a-f]{64}$' AND entry_ordinal>0 AND effect_manifest_version>0 AND ((reversal_of_ledger_id IS NULL AND compensation_generation IS NULL AND entry_type='excess_tokens_recovered' AND flow_direction='outflow') OR (reversal_of_ledger_id IS NOT NULL AND compensation_generation>0 AND entry_type='excess_tokens_recovered_reversal' AND flow_direction='compensation'))) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

DROP TRIGGER IF EXISTS trg_fl_source_token_binding_v3_no_update;
DROP TRIGGER IF EXISTS trg_fl_source_token_binding_v3_no_delete;
DROP TRIGGER IF EXISTS trg_fl_lineage_v3_before_insert;
DROP TRIGGER IF EXISTS trg_fl_lineage_v3_after_insert;
DROP TRIGGER IF EXISTS trg_fl_lineage_v3_no_update;
DROP TRIGGER IF EXISTS trg_fl_lineage_v3_no_delete;
DROP TRIGGER IF EXISTS trg_fl_reversal_guard_v3_validate_insert;
DROP TRIGGER IF EXISTS trg_fl_reversal_guard_v3_validate_update;
DROP TRIGGER IF EXISTS trg_fl_reversal_guard_v3_no_delete;
DROP TRIGGER IF EXISTS trg_fl_journal_v3_set_guard;
DROP TRIGGER IF EXISTS trg_fl_journal_v3_no_update;
DROP TRIGGER IF EXISTS trg_fl_journal_v3_no_delete;
DROP TRIGGER IF EXISTS trg_fl_lineage_reversal_v3_validate;
DROP TRIGGER IF EXISTS trg_fl_lineage_reversal_v3_after_insert;
DROP TRIGGER IF EXISTS trg_fl_lineage_reversal_v3_no_update;
DROP TRIGGER IF EXISTS trg_fl_lineage_reversal_v3_no_delete;
DROP TRIGGER IF EXISTS trg_fl_journal_reversal_v3_validate;
DROP TRIGGER IF EXISTS trg_fl_journal_reversal_v3_no_update;
DROP TRIGGER IF EXISTS trg_fl_journal_reversal_v3_no_delete;
DROP TRIGGER IF EXISTS trg_fl_dependency_v3_set_guard;
DROP TRIGGER IF EXISTS trg_fl_dependency_v3_no_update;
DROP TRIGGER IF EXISTS trg_fl_dependency_v3_no_delete;
DROP TRIGGER IF EXISTS trg_fl_attestation_v3_set_guard;
DROP TRIGGER IF EXISTS trg_fl_attestation_v3_no_update;
DROP TRIGGER IF EXISTS trg_fl_attestation_v3_no_delete;
DROP TRIGGER IF EXISTS trg_fl_cursor_checkpoint_v3_set_guard;
DROP TRIGGER IF EXISTS trg_fl_cursor_checkpoint_v3_no_update;
DROP TRIGGER IF EXISTS trg_fl_cursor_checkpoint_v3_no_delete;
DROP TRIGGER IF EXISTS trg_fl_gov_history_v3_set_guard;
DROP TRIGGER IF EXISTS trg_fl_gov_history_v3_no_update;
DROP TRIGGER IF EXISTS trg_fl_gov_history_v3_no_delete;
DROP TRIGGER IF EXISTS trg_fl_policy_history_v3_set_guard;
DROP TRIGGER IF EXISTS trg_fl_policy_history_v3_no_update;
DROP TRIGGER IF EXISTS trg_fl_policy_history_v3_no_delete;
DROP TRIGGER IF EXISTS trg_fl_reversal_evidence_item_v3_validate;
DROP TRIGGER IF EXISTS trg_fl_reversal_evidence_item_v3_no_update;
DROP TRIGGER IF EXISTS trg_fl_reversal_evidence_item_v3_no_delete;
DROP TRIGGER IF EXISTS trg_fl_reversal_evidence_manifest_v3_validate;
DROP TRIGGER IF EXISTS trg_fl_reversal_evidence_manifest_v3_no_update;
DROP TRIGGER IF EXISTS trg_fl_reversal_evidence_manifest_v3_no_delete;
DROP TRIGGER IF EXISTS trg_fl_reversal_closure_v3_validate;
DROP TRIGGER IF EXISTS trg_fl_reversal_closure_v3_after_insert;
DROP TRIGGER IF EXISTS trg_fl_reversal_closure_v3_no_update;
DROP TRIGGER IF EXISTS trg_fl_reversal_closure_v3_no_delete;
DROP TRIGGER IF EXISTS trg_fl_ledger_v3_validate_compensation;
DROP TRIGGER IF EXISTS trg_fl_ledger_v3_no_update;
DROP TRIGGER IF EXISTS trg_fl_ledger_v3_no_delete;
DROP TRIGGER IF EXISTS trg_fl_gov_ledger_v3_validate_compensation;
DROP TRIGGER IF EXISTS trg_fl_gov_ledger_v3_no_update;
DROP TRIGGER IF EXISTS trg_fl_gov_ledger_v3_no_delete;
DROP TRIGGER IF EXISTS trg_fl_observation_v3_no_update;
DROP TRIGGER IF EXISTS trg_fl_observation_v3_no_delete;
DROP TRIGGER IF EXISTS trg_fl_local_run_v3_immutable;
DROP TRIGGER IF EXISTS trg_fl_local_run_v3_no_delete;
DELIMITER $$
CREATE TRIGGER trg_fl_lineage_v3_before_insert BEFORE INSERT ON fair_launch_application_lineage_v3 FOR EACH ROW BEGIN
  DECLARE v_state VARCHAR(16); DECLARE v_reverse BIGINT UNSIGNED;
  IF NEW.reverses_lineage_id IS NOT NULL THEN
    SELECT state,reversal_lineage_id INTO v_state,v_reverse FROM fair_launch_reversal_serialization_guard_v3 WHERE reversed_lineage_id=NEW.reverses_lineage_id FOR UPDATE;
    IF v_state='closed' OR (v_reverse IS NOT NULL AND v_reverse<>NEW.id) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch reversal guard rejects lineage'; END IF;
  END IF;
END$$
CREATE TRIGGER trg_fl_lineage_v3_after_insert AFTER INSERT ON fair_launch_application_lineage_v3 FOR EACH ROW BEGIN
  IF NEW.disposition='forward' THEN
    INSERT INTO fair_launch_reversal_serialization_guard_v3(reversed_lineage_id,reversal_lineage_id,run_row_id,chain_id,source_binding_id,source_id,source_revision_id,state,created_at,bound_at,closed_at) VALUES(NEW.id,NULL,NEW.run_row_id,NEW.chain_id,NEW.source_binding_id,NEW.source_id,NEW.source_revision_id,'open',NEW.created_at,NULL,NULL);
  END IF;
END$$
CREATE TRIGGER trg_fl_lineage_v3_no_update BEFORE UPDATE ON fair_launch_application_lineage_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_application_lineage_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_lineage_v3_no_delete BEFORE DELETE ON fair_launch_application_lineage_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_application_lineage_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_reversal_guard_v3_validate_insert BEFORE INSERT ON fair_launch_reversal_serialization_guard_v3 FOR EACH ROW BEGIN IF NEW.state<>'open' OR NEW.reversal_lineage_id IS NOT NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reversal guard must initialize open'; END IF; END$$
CREATE TRIGGER trg_fl_reversal_guard_v3_validate_update BEFORE UPDATE ON fair_launch_reversal_serialization_guard_v3 FOR EACH ROW BEGIN
  IF NOT (OLD.reversed_lineage_id<=>NEW.reversed_lineage_id AND OLD.run_row_id<=>NEW.run_row_id AND OLD.chain_id<=>NEW.chain_id AND OLD.source_binding_id<=>NEW.source_binding_id AND OLD.source_id<=>NEW.source_id AND OLD.source_revision_id<=>NEW.source_revision_id AND OLD.created_at<=>NEW.created_at) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch reversal guard identity is immutable'; END IF;
  IF OLD.state='open' AND NEW.state='bound' THEN
    IF OLD.reversal_lineage_id IS NOT NULL OR NEW.reversal_lineage_id IS NULL OR NEW.bound_at IS NULL OR NEW.closed_at IS NOT NULL OR NOT EXISTS(SELECT 1 FROM fair_launch_lineage_reversal_v3 x WHERE x.reversed_lineage_id=NEW.reversed_lineage_id AND x.reversal_lineage_id=NEW.reversal_lineage_id AND x.created_at=NEW.bound_at) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid Fair Launch reversal guard transition'; END IF;
  ELSEIF OLD.state='bound' AND NEW.state='closed' THEN
    IF OLD.reversal_lineage_id<>NEW.reversal_lineage_id OR OLD.bound_at<>NEW.bound_at OR NEW.closed_at IS NULL OR NOT EXISTS(SELECT 1 FROM fair_launch_reversal_closure_v3 x WHERE x.reversed_lineage_id=NEW.reversed_lineage_id AND x.reversal_lineage_id=NEW.reversal_lineage_id AND x.created_at=NEW.closed_at) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid Fair Launch reversal guard transition'; END IF;
  ELSE SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid Fair Launch reversal guard transition';
  END IF;
END$$
CREATE TRIGGER trg_fl_reversal_guard_v3_no_delete BEFORE DELETE ON fair_launch_reversal_serialization_guard_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_reversal_serialization_guard_v3 cannot be deleted'; END$$
CREATE TRIGGER trg_fl_journal_v3_set_guard BEFORE INSERT ON fair_launch_effect_journal_v3 FOR EACH ROW BEGIN DECLARE v_forward BIGINT UNSIGNED DEFAULT NULL; DECLARE v_state VARCHAR(16) DEFAULT NULL; SELECT CASE WHEN disposition='forward' THEN id WHEN disposition='reversal' THEN reverses_lineage_id ELSE NULL END INTO v_forward FROM fair_launch_application_lineage_v3 WHERE id=NEW.source_application_lineage_id; IF v_forward IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch lineage has no forward guard'; END IF; SELECT state INTO v_state FROM fair_launch_reversal_serialization_guard_v3 WHERE reversed_lineage_id=v_forward FOR UPDATE; IF v_state IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch lineage has no forward guard'; END IF; IF NEW.source_application_lineage_id<>v_forward THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch ordinary journal requires forward lineage'; END IF; IF v_state='closed' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch reversal closure is final'; END IF; END$$
CREATE TRIGGER trg_fl_journal_v3_no_update BEFORE UPDATE ON fair_launch_effect_journal_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_effect_journal_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_journal_v3_no_delete BEFORE DELETE ON fair_launch_effect_journal_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_effect_journal_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_lineage_reversal_v3_validate BEFORE INSERT ON fair_launch_lineage_reversal_v3 FOR EACH ROW BEGIN
  DECLARE v_state VARCHAR(16); DECLARE v_reverse BIGINT UNSIGNED; SELECT state,reversal_lineage_id INTO v_state,v_reverse FROM fair_launch_reversal_serialization_guard_v3 WHERE reversed_lineage_id=NEW.reversed_lineage_id FOR UPDATE;
  IF v_state='closed' OR (v_reverse IS NOT NULL AND v_reverse<>NEW.reversal_lineage_id) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch reversal closure is final'; END IF;
  IF NOT EXISTS(SELECT 1 FROM fair_launch_application_lineage_v3 f JOIN fair_launch_application_lineage_v3 r ON r.id=NEW.reversal_lineage_id WHERE f.id=NEW.reversed_lineage_id AND f.disposition='forward' AND f.state='applied' AND r.disposition='reversal' AND r.state='applied' AND r.reverses_lineage_id=f.id AND r.run_row_id=f.run_row_id AND r.chain_id=f.chain_id AND r.source_binding_id=f.source_binding_id AND r.source_id=f.source_id AND r.event_identity_id=f.event_identity_id AND r.source_revision_id=f.source_revision_id AND r.effect_manifest_version=f.effect_manifest_version) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid one-time Fair Launch reversal lineage link'; END IF;
END$$
CREATE TRIGGER trg_fl_lineage_reversal_v3_after_insert AFTER INSERT ON fair_launch_lineage_reversal_v3 FOR EACH ROW BEGIN UPDATE fair_launch_reversal_serialization_guard_v3 SET reversal_lineage_id=NEW.reversal_lineage_id,state='bound',bound_at=NEW.created_at WHERE reversed_lineage_id=NEW.reversed_lineage_id AND state='open'; END$$
CREATE TRIGGER trg_fl_lineage_reversal_v3_no_update BEFORE UPDATE ON fair_launch_lineage_reversal_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_lineage_reversal_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_lineage_reversal_v3_no_delete BEFORE DELETE ON fair_launch_lineage_reversal_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_lineage_reversal_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_journal_reversal_v3_validate BEFORE INSERT ON fair_launch_journal_reversal_v3 FOR EACH ROW BEGIN DECLARE v_state VARCHAR(16); DECLARE v_reverse BIGINT UNSIGNED; SELECT state,reversal_lineage_id INTO v_state,v_reverse FROM fair_launch_reversal_serialization_guard_v3 WHERE reversed_lineage_id=NEW.reversed_lineage_id FOR UPDATE; IF v_state='closed' OR v_reverse<>NEW.reversal_lineage_id THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch reversal closure is final'; END IF; IF NOT EXISTS(SELECT 1 FROM fair_launch_effect_journal_v3 WHERE id=NEW.source_journal_id AND source_application_lineage_id=NEW.reversed_lineage_id) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid Fair Launch journal reversal provenance'; END IF; END$$
CREATE TRIGGER trg_fl_journal_reversal_v3_no_update BEFORE UPDATE ON fair_launch_journal_reversal_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_journal_reversal_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_journal_reversal_v3_no_delete BEFORE DELETE ON fair_launch_journal_reversal_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_journal_reversal_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_dependency_v3_set_guard BEFORE INSERT ON fair_launch_effect_dependency_v3 FOR EACH ROW BEGIN DECLARE v_forward BIGINT UNSIGNED DEFAULT NULL; DECLARE v_state VARCHAR(16) DEFAULT NULL; SELECT CASE WHEN disposition='forward' THEN id WHEN disposition='reversal' THEN reverses_lineage_id ELSE NULL END INTO v_forward FROM fair_launch_application_lineage_v3 WHERE id=NEW.dependent_lineage_id; IF v_forward IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch lineage has no forward guard'; END IF; SELECT state INTO v_state FROM fair_launch_reversal_serialization_guard_v3 WHERE reversed_lineage_id=v_forward FOR UPDATE; IF v_state IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch lineage has no forward guard'; END IF; IF NEW.dependent_lineage_id<>v_forward THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch ordinary dependency requires forward lineage'; END IF; IF v_state='closed' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch reversal closure is final'; END IF; END$$
CREATE TRIGGER trg_fl_dependency_v3_no_update BEFORE UPDATE ON fair_launch_effect_dependency_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_effect_dependency_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_dependency_v3_no_delete BEFORE DELETE ON fair_launch_effect_dependency_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_effect_dependency_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_attestation_v3_set_guard BEFORE INSERT ON fair_launch_effect_attestation_v3 FOR EACH ROW BEGIN DECLARE v_forward BIGINT UNSIGNED DEFAULT NULL; DECLARE v_state VARCHAR(16) DEFAULT NULL; SELECT CASE WHEN disposition='forward' THEN id WHEN disposition='reversal' THEN reverses_lineage_id ELSE NULL END INTO v_forward FROM fair_launch_application_lineage_v3 WHERE id=NEW.source_application_lineage_id; IF v_forward IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch lineage has no forward guard'; END IF; SELECT state INTO v_state FROM fair_launch_reversal_serialization_guard_v3 WHERE reversed_lineage_id=v_forward FOR UPDATE; IF v_state IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch lineage has no forward guard'; END IF; IF v_state='closed' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch reversal closure is final'; END IF; END$$
CREATE TRIGGER trg_fl_attestation_v3_no_update BEFORE UPDATE ON fair_launch_effect_attestation_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_effect_attestation_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_attestation_v3_no_delete BEFORE DELETE ON fair_launch_effect_attestation_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_effect_attestation_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_cursor_checkpoint_v3_set_guard BEFORE INSERT ON fair_launch_cursor_checkpoint_v3 FOR EACH ROW BEGIN DECLARE v_forward BIGINT UNSIGNED DEFAULT NULL; DECLARE v_state VARCHAR(16) DEFAULT NULL; SELECT reverses_lineage_id INTO v_forward FROM fair_launch_application_lineage_v3 WHERE id=NEW.reversal_application_lineage_id AND disposition='reversal'; IF v_forward IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch cursor checkpoint requires reversal lineage'; END IF; SELECT state INTO v_state FROM fair_launch_reversal_serialization_guard_v3 WHERE reversed_lineage_id=v_forward FOR UPDATE; IF v_state IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch lineage has no forward guard'; END IF; IF v_state='closed' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch reversal closure is final'; END IF; END$$
CREATE TRIGGER trg_fl_cursor_checkpoint_v3_no_update BEFORE UPDATE ON fair_launch_cursor_checkpoint_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_cursor_checkpoint_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_cursor_checkpoint_v3_no_delete BEFORE DELETE ON fair_launch_cursor_checkpoint_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_cursor_checkpoint_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_gov_history_v3_set_guard BEFORE INSERT ON fair_launch_governance_history_v3 FOR EACH ROW BEGIN DECLARE v_forward BIGINT UNSIGNED DEFAULT NULL; DECLARE v_state VARCHAR(16) DEFAULT NULL; SELECT CASE WHEN disposition='forward' THEN id WHEN disposition='reversal' THEN reverses_lineage_id ELSE NULL END INTO v_forward FROM fair_launch_application_lineage_v3 WHERE id=NEW.source_application_lineage_id; IF v_forward IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch lineage has no forward guard'; END IF; SELECT state INTO v_state FROM fair_launch_reversal_serialization_guard_v3 WHERE reversed_lineage_id=v_forward FOR UPDATE; IF v_state IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch lineage has no forward guard'; END IF; IF v_state='closed' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch reversal closure is final'; END IF; END$$
CREATE TRIGGER trg_fl_gov_history_v3_no_update BEFORE UPDATE ON fair_launch_governance_history_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_governance_history_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_gov_history_v3_no_delete BEFORE DELETE ON fair_launch_governance_history_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_governance_history_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_policy_history_v3_set_guard BEFORE INSERT ON fair_launch_policy_history_v3 FOR EACH ROW BEGIN DECLARE v_forward BIGINT UNSIGNED DEFAULT NULL; DECLARE v_state VARCHAR(16) DEFAULT NULL; SELECT CASE WHEN disposition='forward' THEN id WHEN disposition='reversal' THEN reverses_lineage_id ELSE NULL END INTO v_forward FROM fair_launch_application_lineage_v3 WHERE id=NEW.source_application_lineage_id; IF v_forward IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch lineage has no forward guard'; END IF; SELECT state INTO v_state FROM fair_launch_reversal_serialization_guard_v3 WHERE reversed_lineage_id=v_forward FOR UPDATE; IF v_state IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch lineage has no forward guard'; END IF; IF v_state='closed' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch reversal closure is final'; END IF; END$$
CREATE TRIGGER trg_fl_policy_history_v3_no_update BEFORE UPDATE ON fair_launch_policy_history_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_policy_history_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_policy_history_v3_no_delete BEFORE DELETE ON fair_launch_policy_history_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_policy_history_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_reversal_evidence_item_v3_validate BEFORE INSERT ON fair_launch_reversal_evidence_item_v3 FOR EACH ROW BEGIN
  DECLARE v_state VARCHAR(16) DEFAULT NULL; DECLARE v_reverse BIGINT UNSIGNED DEFAULT NULL; DECLARE v_previous CHAR(64); DECLARE v_expected CHAR(64); SELECT state,reversal_lineage_id INTO v_state,v_reverse FROM fair_launch_reversal_serialization_guard_v3 WHERE reversed_lineage_id=NEW.reversed_lineage_id FOR UPDATE;
  IF v_state IS NULL OR v_state='closed' OR v_reverse<>NEW.reversal_lineage_id THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch reversal closure is final'; END IF;
  IF NEW.evidence_ordinal=1 THEN SET v_previous=LOWER(SHA2(CONCAT('JC_FL_REVERSAL_EVIDENCE_V1',LPAD(HEX(NEW.reversed_lineage_id),16,'0'),LPAD(HEX(NEW.reversal_lineage_id),16,'0')),256)); ELSE SELECT item_chain_digest INTO v_previous FROM fair_launch_reversal_evidence_item_v3 WHERE reversed_lineage_id=NEW.reversed_lineage_id AND evidence_ordinal=NEW.evidence_ordinal-1; END IF;
  IF v_previous IS NULL OR BINARY NEW.previous_chain_digest<>BINARY v_previous THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid Fair Launch evidence digest chain'; END IF;
  SET v_expected=LOWER(SHA2(CONCAT('JC_FL_REVERSAL_EVIDENCE_ITEM_V1',NEW.previous_chain_digest,LPAD(HEX(NEW.evidence_ordinal),16,'0'),LPAD(HEX(OCTET_LENGTH(NEW.evidence_kind)),8,'0'),HEX(NEW.evidence_kind),LPAD(HEX(NEW.evidence_id),16,'0'),NEW.evidence_digest),256)); IF BINARY NEW.item_chain_digest<>BINARY v_expected THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid Fair Launch evidence digest chain'; END IF;
  IF (NEW.evidence_kind='source_journal' AND NOT EXISTS(SELECT 1 FROM fair_launch_effect_journal_v3 WHERE id=NEW.evidence_id AND source_application_lineage_id=NEW.reversed_lineage_id AND after_digest=NEW.evidence_digest)) OR (NEW.evidence_kind='dependency' AND NOT EXISTS(SELECT 1 FROM fair_launch_effect_dependency_v3 WHERE id=NEW.evidence_id AND dependent_lineage_id=NEW.reversed_lineage_id AND dependency_digest=NEW.evidence_digest)) OR (NEW.evidence_kind='attestation' AND NOT EXISTS(SELECT 1 FROM fair_launch_effect_attestation_v3 WHERE id=NEW.evidence_id AND source_application_lineage_id IN(NEW.reversed_lineage_id,NEW.reversal_lineage_id) AND attestation_digest=NEW.evidence_digest)) OR (NEW.evidence_kind='cursor_checkpoint' AND NOT EXISTS(SELECT 1 FROM fair_launch_cursor_checkpoint_v3 WHERE id=NEW.evidence_id AND reversal_application_lineage_id=NEW.reversal_lineage_id AND checkpoint_digest=NEW.evidence_digest)) OR (NEW.evidence_kind='governance_history' AND NOT EXISTS(SELECT 1 FROM fair_launch_governance_history_v3 WHERE id=NEW.evidence_id AND source_application_lineage_id IN(NEW.reversed_lineage_id,NEW.reversal_lineage_id) AND after_digest=NEW.evidence_digest)) OR (NEW.evidence_kind='policy_history' AND NOT EXISTS(SELECT 1 FROM fair_launch_policy_history_v3 WHERE id=NEW.evidence_id AND source_application_lineage_id IN(NEW.reversed_lineage_id,NEW.reversal_lineage_id) AND after_digest=NEW.evidence_digest)) OR (NEW.evidence_kind='source_ledger' AND NOT EXISTS(SELECT 1 FROM fair_launch_ledger_entry_v3 WHERE id=NEW.evidence_id AND source_application_lineage_id=NEW.reversed_lineage_id AND reversal_of_ledger_id IS NULL AND row_digest=NEW.evidence_digest)) OR (NEW.evidence_kind='source_governance_ledger' AND NOT EXISTS(SELECT 1 FROM fair_launch_governance_ledger_entry_v3 WHERE id=NEW.evidence_id AND source_application_lineage_id=NEW.reversed_lineage_id AND reversal_of_ledger_id IS NULL AND row_digest=NEW.evidence_digest)) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid Fair Launch reversal evidence item'; END IF;
END$$
CREATE TRIGGER trg_fl_reversal_evidence_item_v3_no_update BEFORE UPDATE ON fair_launch_reversal_evidence_item_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_reversal_evidence_item_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_reversal_evidence_item_v3_no_delete BEFORE DELETE ON fair_launch_reversal_evidence_item_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_reversal_evidence_item_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_reversal_evidence_manifest_v3_validate BEFORE INSERT ON fair_launch_reversal_evidence_manifest_v3 FOR EACH ROW BEGIN
  DECLARE v_state VARCHAR(16) DEFAULT NULL; DECLARE v_reverse BIGINT UNSIGNED DEFAULT NULL; DECLARE v_expected CHAR(64); SELECT state,reversal_lineage_id INTO v_state,v_reverse FROM fair_launch_reversal_serialization_guard_v3 WHERE reversed_lineage_id=NEW.reversed_lineage_id FOR UPDATE;
  IF v_state IS NULL OR v_state='closed' OR v_reverse<>NEW.reversal_lineage_id THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch reversal closure is final'; END IF;
  IF NEW.source_journal_count<>(SELECT COUNT(*) FROM fair_launch_reversal_evidence_item_v3 WHERE reversed_lineage_id=NEW.reversed_lineage_id AND evidence_kind='source_journal') OR NEW.dependency_count<>(SELECT COUNT(*) FROM fair_launch_reversal_evidence_item_v3 WHERE reversed_lineage_id=NEW.reversed_lineage_id AND evidence_kind='dependency') OR NEW.attestation_count<>(SELECT COUNT(*) FROM fair_launch_reversal_evidence_item_v3 WHERE reversed_lineage_id=NEW.reversed_lineage_id AND evidence_kind='attestation') OR NEW.cursor_checkpoint_count<>(SELECT COUNT(*) FROM fair_launch_reversal_evidence_item_v3 WHERE reversed_lineage_id=NEW.reversed_lineage_id AND evidence_kind='cursor_checkpoint') OR NEW.governance_history_count<>(SELECT COUNT(*) FROM fair_launch_reversal_evidence_item_v3 WHERE reversed_lineage_id=NEW.reversed_lineage_id AND evidence_kind='governance_history') OR NEW.policy_history_count<>(SELECT COUNT(*) FROM fair_launch_reversal_evidence_item_v3 WHERE reversed_lineage_id=NEW.reversed_lineage_id AND evidence_kind='policy_history') OR NEW.source_ledger_count<>(SELECT COUNT(*) FROM fair_launch_reversal_evidence_item_v3 WHERE reversed_lineage_id=NEW.reversed_lineage_id AND evidence_kind='source_ledger') OR NEW.source_governance_ledger_count<>(SELECT COUNT(*) FROM fair_launch_reversal_evidence_item_v3 WHERE reversed_lineage_id=NEW.reversed_lineage_id AND evidence_kind='source_governance_ledger') OR NEW.evidence_item_count<>(SELECT COUNT(*) FROM fair_launch_reversal_evidence_item_v3 WHERE reversed_lineage_id=NEW.reversed_lineage_id) OR BINARY NEW.final_item_chain_digest<>BINARY IF(NEW.evidence_item_count=0,LOWER(SHA2(CONCAT('JC_FL_REVERSAL_EVIDENCE_V1',LPAD(HEX(NEW.reversed_lineage_id),16,'0'),LPAD(HEX(NEW.reversal_lineage_id),16,'0')),256)),(SELECT item_chain_digest FROM fair_launch_reversal_evidence_item_v3 WHERE reversed_lineage_id=NEW.reversed_lineage_id ORDER BY evidence_ordinal DESC LIMIT 1)) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='incomplete Fair Launch reversal evidence manifest'; END IF;
  SET v_expected=LOWER(SHA2(CONCAT('JC_FL_REVERSAL_MANIFEST_V1',LPAD(HEX(NEW.reversed_lineage_id),16,'0'),LPAD(HEX(NEW.reversal_lineage_id),16,'0'),LPAD(HEX(NEW.source_journal_count),16,'0'),LPAD(HEX(NEW.dependency_count),16,'0'),LPAD(HEX(NEW.attestation_count),16,'0'),LPAD(HEX(NEW.cursor_checkpoint_count),16,'0'),LPAD(HEX(NEW.governance_history_count),16,'0'),LPAD(HEX(NEW.policy_history_count),16,'0'),LPAD(HEX(NEW.source_ledger_count),16,'0'),LPAD(HEX(NEW.source_governance_ledger_count),16,'0'),LPAD(HEX(NEW.evidence_item_count),16,'0'),NEW.final_item_chain_digest),256)); IF BINARY NEW.manifest_digest<>BINARY v_expected THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid Fair Launch evidence manifest digest'; END IF;
END$$
CREATE TRIGGER trg_fl_reversal_evidence_manifest_v3_no_update BEFORE UPDATE ON fair_launch_reversal_evidence_manifest_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_reversal_evidence_manifest_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_reversal_evidence_manifest_v3_no_delete BEFORE DELETE ON fair_launch_reversal_evidence_manifest_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_reversal_evidence_manifest_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_source_token_binding_v3_no_update BEFORE UPDATE ON fair_launch_source_token_binding_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_source_token_binding_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_source_token_binding_v3_no_delete BEFORE DELETE ON fair_launch_source_token_binding_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_source_token_binding_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_ledger_v3_validate_compensation BEFORE INSERT ON fair_launch_ledger_entry_v3 FOR EACH ROW BEGIN
  DECLARE v_forward BIGINT UNSIGNED DEFAULT NULL; DECLARE v_state VARCHAR(16) DEFAULT NULL; DECLARE v_disposition VARCHAR(20); DECLARE v_project_token CHAR(42); DECLARE v_project_decimals TINYINT UNSIGNED; DECLARE v_payment_token CHAR(42); DECLARE v_payment_decimals TINYINT UNSIGNED; DECLARE v_expected_role VARCHAR(16); DECLARE v_expected_direction VARCHAR(16); DECLARE v_expected_token CHAR(42); DECLARE v_expected_decimals TINYINT UNSIGNED; DECLARE v_fraction VARCHAR(160); DECLARE v_display VARCHAR(160);
  SELECT project_token_address,project_token_decimals,payment_token_address,payment_token_decimals INTO v_project_token,v_project_decimals,v_payment_token,v_payment_decimals FROM fair_launch_project_binding_v3 WHERE id=NEW.project_binding_id;
  IF v_project_token IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch ledger project binding missing'; END IF;
  IF NEW.entry_type IN('project_token_deposited','project_token_claimed','project_tokens_withdrawn','project_token_deposited_reversal','project_token_claimed_reversal','project_tokens_withdrawn_reversal') THEN SET v_expected_role='project',v_expected_token=v_project_token,v_expected_decimals=v_project_decimals; ELSE SET v_expected_role='payment',v_expected_token=v_payment_token,v_expected_decimals=v_payment_decimals; END IF;
  SET v_expected_direction=CASE NEW.entry_type WHEN 'project_token_deposited' THEN 'inflow' WHEN 'payment_contributed' THEN 'inflow' WHEN 'project_token_claimed' THEN 'outflow' WHEN 'payment_refunded' THEN 'outflow' WHEN 'project_proceeds_paid' THEN 'outflow' WHEN 'platform_fee_paid' THEN 'outflow' WHEN 'project_tokens_withdrawn' THEN 'outflow' ELSE 'compensation' END;
  IF NEW.asset_role<>v_expected_role OR NEW.flow_direction<>v_expected_direction OR BINARY NEW.token_address<>BINARY v_expected_token OR NEW.token_decimals<>v_expected_decimals THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid Fair Launch typed financial semantics'; END IF;
  IF NEW.amount_raw='0' THEN SET v_display='0'; ELSEIF CHAR_LENGTH(NEW.amount_raw)<=NEW.token_decimals THEN SET v_fraction=TRIM(TRAILING '0' FROM CONCAT(REPEAT('0',NEW.token_decimals-CHAR_LENGTH(NEW.amount_raw)),NEW.amount_raw)); SET v_display=IF(v_fraction='','0',CONCAT('0.',v_fraction)); ELSE SET v_fraction=TRIM(TRAILING '0' FROM RIGHT(NEW.amount_raw,NEW.token_decimals)); SET v_display=IF(v_fraction='',LEFT(NEW.amount_raw,CHAR_LENGTH(NEW.amount_raw)-NEW.token_decimals),CONCAT(LEFT(NEW.amount_raw,CHAR_LENGTH(NEW.amount_raw)-NEW.token_decimals),'.',v_fraction)); END IF;
  IF BINARY NEW.amount_display<>BINARY v_display THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid Fair Launch canonical amount display'; END IF;
  SELECT disposition,CASE WHEN disposition='forward' THEN id WHEN disposition='reversal' THEN reverses_lineage_id ELSE NULL END INTO v_disposition,v_forward FROM fair_launch_application_lineage_v3 WHERE id=NEW.source_application_lineage_id;
  IF v_forward IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch lineage has no forward guard'; END IF; SELECT state INTO v_state FROM fair_launch_reversal_serialization_guard_v3 WHERE reversed_lineage_id=v_forward FOR UPDATE; IF v_state IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch lineage has no forward guard'; END IF; IF v_state='closed' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch reversal closure is final'; END IF;
  IF NEW.reversal_of_ledger_id IS NULL AND v_disposition<>'forward' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch ordinary ledger requires forward lineage'; END IF;
  IF NEW.reversal_of_ledger_id IS NOT NULL AND (v_disposition<>'reversal' OR NOT EXISTS(SELECT 1 FROM fair_launch_ledger_entry_v3 f JOIN fair_launch_lineage_reversal_v3 p ON p.reversed_lineage_id=f.source_application_lineage_id JOIN fair_launch_application_lineage_v3 r ON r.id=p.reversal_lineage_id WHERE f.id=NEW.reversal_of_ledger_id AND p.reversal_lineage_id=NEW.source_application_lineage_id AND NEW.run_row_id=f.run_row_id AND NEW.chain_id=f.chain_id AND NEW.source_id=f.source_id AND NEW.project_binding_id=f.project_binding_id AND NEW.project_id=f.project_id AND BINARY NEW.contract_address=BINARY f.contract_address AND BINARY NEW.contract_version=BINARY f.contract_version AND BINARY NEW.wallet_address=BINARY f.wallet_address AND BINARY NEW.counterparty_address=BINARY f.counterparty_address AND BINARY NEW.token_address=BINARY f.token_address AND NEW.token_decimals=f.token_decimals AND BINARY NEW.amount_raw=BINARY f.amount_raw AND BINARY NEW.amount_display=BINARY f.amount_display AND NEW.asset_role=f.asset_role AND NEW.entry_ordinal=f.entry_ordinal AND NEW.entry_type=CONCAT(f.entry_type,'_reversal') AND NEW.flow_direction='compensation' AND NEW.compensation_generation=r.placement_generation)) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid Fair Launch exact compensation semantics'; END IF;
END$$
CREATE TRIGGER trg_fl_ledger_v3_no_update BEFORE UPDATE ON fair_launch_ledger_entry_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_ledger_entry_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_ledger_v3_no_delete BEFORE DELETE ON fair_launch_ledger_entry_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_ledger_entry_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_gov_ledger_v3_validate_compensation BEFORE INSERT ON fair_launch_governance_ledger_entry_v3 FOR EACH ROW BEGIN
  DECLARE v_forward BIGINT UNSIGNED DEFAULT NULL; DECLARE v_state VARCHAR(16) DEFAULT NULL; DECLARE v_disposition VARCHAR(20); DECLARE v_fraction VARCHAR(160); DECLARE v_display VARCHAR(160);
  IF NEW.asset_role<>'governance_token' OR NEW.flow_direction<>IF(NEW.reversal_of_ledger_id IS NULL,'outflow','compensation') THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid Fair Launch typed governance semantics'; END IF;
  IF NEW.amount_raw='0' THEN SET v_display='0'; ELSEIF CHAR_LENGTH(NEW.amount_raw)<=NEW.token_decimals THEN SET v_fraction=TRIM(TRAILING '0' FROM CONCAT(REPEAT('0',NEW.token_decimals-CHAR_LENGTH(NEW.amount_raw)),NEW.amount_raw)); SET v_display=IF(v_fraction='','0',CONCAT('0.',v_fraction)); ELSE SET v_fraction=TRIM(TRAILING '0' FROM RIGHT(NEW.amount_raw,NEW.token_decimals)); SET v_display=IF(v_fraction='',LEFT(NEW.amount_raw,CHAR_LENGTH(NEW.amount_raw)-NEW.token_decimals),CONCAT(LEFT(NEW.amount_raw,CHAR_LENGTH(NEW.amount_raw)-NEW.token_decimals),'.',v_fraction)); END IF; IF BINARY NEW.amount_display<>BINARY v_display THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid Fair Launch canonical amount display'; END IF;
  SELECT disposition,CASE WHEN disposition='forward' THEN id WHEN disposition='reversal' THEN reverses_lineage_id ELSE NULL END INTO v_disposition,v_forward FROM fair_launch_application_lineage_v3 WHERE id=NEW.source_application_lineage_id; IF v_forward IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch lineage has no forward guard'; END IF; SELECT state INTO v_state FROM fair_launch_reversal_serialization_guard_v3 WHERE reversed_lineage_id=v_forward FOR UPDATE; IF v_state IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch lineage has no forward guard'; END IF; IF v_state='closed' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch reversal closure is final'; END IF;
  IF NEW.reversal_of_ledger_id IS NULL AND v_disposition<>'forward' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch ordinary governance ledger requires forward lineage'; END IF;
  IF NEW.reversal_of_ledger_id IS NOT NULL AND (v_disposition<>'reversal' OR NOT EXISTS(SELECT 1 FROM fair_launch_governance_ledger_entry_v3 f JOIN fair_launch_lineage_reversal_v3 p ON p.reversed_lineage_id=f.source_application_lineage_id JOIN fair_launch_application_lineage_v3 r ON r.id=p.reversal_lineage_id WHERE f.id=NEW.reversal_of_ledger_id AND p.reversal_lineage_id=NEW.source_application_lineage_id AND NEW.run_row_id=f.run_row_id AND NEW.chain_id=f.chain_id AND NEW.source_id=f.source_id AND BINARY NEW.contract_address=BINARY f.contract_address AND NEW.token_binding_id=f.token_binding_id AND BINARY NEW.token_address=BINARY f.token_address AND BINARY NEW.receiver_address=BINARY f.receiver_address AND NEW.token_decimals=f.token_decimals AND BINARY NEW.amount_raw=BINARY f.amount_raw AND BINARY NEW.amount_display=BINARY f.amount_display AND NEW.asset_role=f.asset_role AND NEW.entry_ordinal=f.entry_ordinal AND f.entry_type='excess_tokens_recovered' AND NEW.entry_type='excess_tokens_recovered_reversal' AND NEW.compensation_generation=r.placement_generation)) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid Fair Launch exact governance compensation semantics'; END IF;
END$$
CREATE TRIGGER trg_fl_gov_ledger_v3_no_update BEFORE UPDATE ON fair_launch_governance_ledger_entry_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_governance_ledger_entry_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_gov_ledger_v3_no_delete BEFORE DELETE ON fair_launch_governance_ledger_entry_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_governance_ledger_entry_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_reversal_closure_v3_validate BEFORE INSERT ON fair_launch_reversal_closure_v3 FOR EACH ROW BEGIN
  DECLARE v_state VARCHAR(16) DEFAULT NULL; DECLARE v_reverse BIGINT UNSIGNED DEFAULT NULL; DECLARE v_expected CHAR(64); SELECT state,reversal_lineage_id INTO v_state,v_reverse FROM fair_launch_reversal_serialization_guard_v3 WHERE reversed_lineage_id=NEW.reversed_lineage_id FOR UPDATE;
  IF v_state<>'bound' OR v_reverse<>NEW.reversal_lineage_id OR NOT EXISTS(SELECT 1 FROM fair_launch_reversal_evidence_manifest_v3 m WHERE m.reversed_lineage_id=NEW.reversed_lineage_id AND m.reversal_lineage_id=NEW.reversal_lineage_id AND m.manifest_digest=NEW.evidence_manifest_digest AND m.source_journal_count=NEW.source_journal_count AND m.dependency_count=NEW.dependency_count AND m.attestation_count=NEW.attestation_count AND m.cursor_checkpoint_count=NEW.cursor_checkpoint_count AND m.governance_history_count=NEW.governance_history_count AND m.policy_history_count=NEW.policy_history_count AND m.source_ledger_count=NEW.source_ledger_count AND m.source_governance_ledger_count=NEW.source_governance_ledger_count) OR NEW.source_journal_count<>(SELECT COUNT(*) FROM fair_launch_effect_journal_v3 WHERE source_application_lineage_id=NEW.reversed_lineage_id) OR NEW.linked_journal_count<>(SELECT COUNT(*) FROM fair_launch_journal_reversal_v3 WHERE reversed_lineage_id=NEW.reversed_lineage_id AND reversal_lineage_id=NEW.reversal_lineage_id) OR NEW.dependency_count<>(SELECT COUNT(*) FROM fair_launch_effect_dependency_v3 WHERE dependent_lineage_id=NEW.reversed_lineage_id) OR NEW.attestation_count<>(SELECT COUNT(*) FROM fair_launch_effect_attestation_v3 WHERE source_application_lineage_id IN(NEW.reversed_lineage_id,NEW.reversal_lineage_id)) OR NEW.cursor_checkpoint_count<>(SELECT COUNT(*) FROM fair_launch_cursor_checkpoint_v3 WHERE reversal_application_lineage_id=NEW.reversal_lineage_id) OR NEW.governance_history_count<>(SELECT COUNT(*) FROM fair_launch_governance_history_v3 WHERE source_application_lineage_id IN(NEW.reversed_lineage_id,NEW.reversal_lineage_id)) OR NEW.policy_history_count<>(SELECT COUNT(*) FROM fair_launch_policy_history_v3 WHERE source_application_lineage_id IN(NEW.reversed_lineage_id,NEW.reversal_lineage_id)) OR NEW.source_ledger_count<>(SELECT COUNT(*) FROM fair_launch_ledger_entry_v3 WHERE source_application_lineage_id=NEW.reversed_lineage_id AND reversal_of_ledger_id IS NULL) OR NEW.compensation_ledger_count<>(SELECT COUNT(DISTINCT reversal_of_ledger_id) FROM fair_launch_ledger_entry_v3 WHERE source_application_lineage_id=NEW.reversal_lineage_id AND reversal_of_ledger_id IS NOT NULL) OR EXISTS(SELECT 1 FROM fair_launch_ledger_entry_v3 f WHERE f.source_application_lineage_id=NEW.reversed_lineage_id AND f.reversal_of_ledger_id IS NULL AND NOT EXISTS(SELECT 1 FROM fair_launch_ledger_entry_v3 c WHERE c.reversal_of_ledger_id=f.id AND c.source_application_lineage_id=NEW.reversal_lineage_id)) OR NEW.source_governance_ledger_count<>(SELECT COUNT(*) FROM fair_launch_governance_ledger_entry_v3 WHERE source_application_lineage_id=NEW.reversed_lineage_id AND reversal_of_ledger_id IS NULL) OR NEW.compensation_governance_ledger_count<>(SELECT COUNT(DISTINCT reversal_of_ledger_id) FROM fair_launch_governance_ledger_entry_v3 WHERE source_application_lineage_id=NEW.reversal_lineage_id AND reversal_of_ledger_id IS NOT NULL) OR EXISTS(SELECT 1 FROM fair_launch_governance_ledger_entry_v3 f WHERE f.source_application_lineage_id=NEW.reversed_lineage_id AND f.reversal_of_ledger_id IS NULL AND NOT EXISTS(SELECT 1 FROM fair_launch_governance_ledger_entry_v3 c WHERE c.reversal_of_ledger_id=f.id AND c.source_application_lineage_id=NEW.reversal_lineage_id)) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='incomplete Fair Launch reversal closure'; END IF;
  SET v_expected=LOWER(SHA2(CONCAT('JC_FL_REVERSAL_CLOSURE_V1',LPAD(HEX(NEW.reversed_lineage_id),16,'0'),LPAD(HEX(NEW.reversal_lineage_id),16,'0'),NEW.evidence_manifest_digest,LPAD(HEX(NEW.source_journal_count),16,'0'),LPAD(HEX(NEW.linked_journal_count),16,'0'),LPAD(HEX(NEW.dependency_count),16,'0'),LPAD(HEX(NEW.attestation_count),16,'0'),LPAD(HEX(NEW.cursor_checkpoint_count),16,'0'),LPAD(HEX(NEW.governance_history_count),16,'0'),LPAD(HEX(NEW.policy_history_count),16,'0'),LPAD(HEX(NEW.source_ledger_count),16,'0'),LPAD(HEX(NEW.compensation_ledger_count),16,'0'),LPAD(HEX(NEW.source_governance_ledger_count),16,'0'),LPAD(HEX(NEW.compensation_governance_ledger_count),16,'0'),DATE_FORMAT(NEW.created_at,'%Y-%m-%dT%H:%i:%s.%fZ')),256)); IF BINARY NEW.closure_digest<>BINARY v_expected THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid Fair Launch reversal closure digest'; END IF;
END$$
CREATE TRIGGER trg_fl_reversal_closure_v3_after_insert AFTER INSERT ON fair_launch_reversal_closure_v3 FOR EACH ROW BEGIN UPDATE fair_launch_reversal_serialization_guard_v3 SET state='closed',closed_at=NEW.created_at WHERE reversed_lineage_id=NEW.reversed_lineage_id AND reversal_lineage_id=NEW.reversal_lineage_id AND state='bound'; END$$
CREATE TRIGGER trg_fl_reversal_closure_v3_no_update BEFORE UPDATE ON fair_launch_reversal_closure_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_reversal_closure_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_reversal_closure_v3_no_delete BEFORE DELETE ON fair_launch_reversal_closure_v3 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_reversal_closure_v3 is append-only'; END$$
CREATE TRIGGER trg_fl_observation_v3_no_update BEFORE UPDATE ON fair_launch_local_e2e_observation FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_local_e2e_observation is append-only'; END$$
CREATE TRIGGER trg_fl_observation_v3_no_delete BEFORE DELETE ON fair_launch_local_e2e_observation FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_local_e2e_observation is append-only'; END$$
CREATE TRIGGER trg_fl_local_run_v3_immutable BEFORE UPDATE ON fair_launch_local_e2e_run FOR EACH ROW BEGIN
  IF NOT (OLD.run_id<=>NEW.run_id AND OLD.environment<=>NEW.environment AND OLD.exact_rpc_url<=>NEW.exact_rpc_url AND OLD.redis_endpoint<=>NEW.redis_endpoint AND OLD.chain_id<=>NEW.chain_id AND OLD.database_name<=>NEW.database_name AND OLD.mysql_server_uuid<=>NEW.mysql_server_uuid AND OLD.mysql_version<=>NEW.mysql_version AND OLD.container_id<=>NEW.container_id AND OLD.owner_label_digest<=>NEW.owner_label_digest AND OLD.schema_generation<=>NEW.schema_generation AND OLD.baseline_digest<=>NEW.baseline_digest AND OLD.migration_manifest_digest<=>NEW.migration_manifest_digest AND OLD.schema_digest<=>NEW.schema_digest AND OLD.grant_digest<=>NEW.grant_digest AND OLD.source_registry_digest<=>NEW.source_registry_digest AND OLD.hardhat_genesis_hash<=>NEW.hardhat_genesis_hash AND OLD.chain_manifest_digest<=>NEW.chain_manifest_digest AND OLD.deployment_manifest_digest<=>NEW.deployment_manifest_digest AND OLD.runtime_admission_digest<=>NEW.runtime_admission_digest AND OLD.created_at<=>NEW.created_at AND OLD.expires_at<=>NEW.expires_at) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch local run fingerprint is immutable'; END IF;
  IF NOT ((OLD.state='created' AND NEW.state='verified' AND NEW.closed_at IS NULL) OR (OLD.state IN('created','verified') AND NEW.state='expired' AND NEW.closed_at IS NULL) OR (OLD.state IN('created','verified','expired') AND NEW.state='closed' AND OLD.closed_at IS NULL AND NEW.closed_at IS NOT NULL)) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid Fair Launch local run state transition'; END IF;
END$$
CREATE TRIGGER trg_fl_local_run_v3_no_delete BEFORE DELETE ON fair_launch_local_e2e_run FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_local_e2e_run cannot be deleted'; END$$
DELIMITER ;
DO RELEASE_LOCK(CONCAT('jc_fl_0912:',DATABASE()));
