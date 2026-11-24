-- JoinCare Fair Launch v3 project and wallet financial projections.
-- ADDITIVE LOCAL-ONLY candidate. Not in production Apply/release paths.
SET NAMES utf8mb4;
SET @jc_fl_local_gate=COALESCE(@JC_FAIR_LAUNCH_LOCAL_E2E_APPROVED,0);
DROP PROCEDURE IF EXISTS jc_fair_launch_local_e2e_0909;
DELIMITER $$
CREATE PROCEDURE jc_fair_launch_local_e2e_0909()
BEGIN
  IF @jc_fl_local_gate<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch local E2E migration requires local-only execution gate'; END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_project_binding_v3') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='09_08 Fair Launch local binding migration missing';
  END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_event_application_attempt_v2' AND INDEX_NAME='uk_ce_application_v3_provenance') THEN
    ALTER TABLE chain_event_application_attempt_v2 ADD UNIQUE KEY uk_ce_application_v3_provenance(id,event_identity_id,placement_id,effect_version);
  END IF;
END$$
DELIMITER ;
CALL jc_fair_launch_local_e2e_0909();
DROP PROCEDURE jc_fair_launch_local_e2e_0909;

CREATE TABLE IF NOT EXISTS fair_launch_project_state_v3 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  project_binding_id BIGINT UNSIGNED NOT NULL,
  run_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  contract_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  project_token_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  payment_token_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  terminal_reason VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  sale_token_amount_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  total_raised_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  total_tokens_deposited_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  total_tokens_claimed_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  participant_count_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  final_total_raised_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NULL,
  final_price_numerator_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NULL,
  final_price_denominator_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NULL,
  platform_fee_reserved_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_proceeds_paid_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform_fee_paid_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  settlement_block_number BIGINT UNSIGNED NULL,
  settlement_block_hash CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL,
  settlement_anchor_block_number BIGINT UNSIGNED NULL,
  settlement_anchor_block_hash CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL,
  settlement_tx_hash CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL,
  settlement_tx_log_ordinal INT UNSIGNED NULL,
  release_manifest_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  release_policy_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  release_policy_version INT UNSIGNED NULL,
  project_proceeds_withdrawn TINYINT UNSIGNED NOT NULL,
  platform_fee_withdrawn TINYINT UNSIGNED NOT NULL,
  failed_project_tokens_withdrawn TINYINT UNSIGNED NOT NULL,
  tombstoned TINYINT UNSIGNED NOT NULL,
  state_generation BIGINT UNSIGNED NOT NULL,
  last_event_identity_id BIGINT UNSIGNED NULL,
  last_placement_id BIGINT UNSIGNED NULL,
  last_source_revision_id BIGINT UNSIGNED NULL,
  last_effect_manifest_version INT UNSIGNED NULL,
  last_application_attempt_id BIGINT UNSIGNED NULL,
  source_application_lineage_id BIGINT UNSIGNED NULL,
  state_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_project_state_binding(project_binding_id),
  UNIQUE KEY uk_fl_project_state_identity(chain_id,contract_address,contract_version,project_id),
  CONSTRAINT fk_fl_project_state_binding FOREIGN KEY(project_binding_id,chain_id,source_id,contract_address,contract_version,project_id) REFERENCES fair_launch_project_binding_v3(id,chain_id,source_id,contract_address,contract_version,project_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_project_state_run_chain FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_local_e2e_run(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_project_state_event FOREIGN KEY(last_event_identity_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_project_state_placement FOREIGN KEY(last_placement_id,last_event_identity_id) REFERENCES chain_event_placement_v2(id,event_identity_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_project_state_application FOREIGN KEY(last_application_attempt_id,last_event_identity_id,last_placement_id) REFERENCES chain_event_application_attempt_v2(id,event_identity_id,placement_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_project_state CHECK(
    chain_id=31337 AND contract_version='fair_launch_v1' AND project_id>0 AND
    contract_address REGEXP '^0x[0-9a-f]{40}$' AND project_token_address REGEXP '^0x[0-9a-f]{40}$' AND payment_token_address REGEXP '^0x[0-9a-f]{40}$' AND
    status IN ('Pending','Active','SuccessfulPendingRelease','Successful','Failed','Cancelled') AND
    terminal_reason IN ('none','soft_cap_met','soft_cap_not_met','cancelled','release_invalidated','payment_token_invalidated') AND
    sale_token_amount_raw REGEXP '^[1-9][0-9]{0,77}$' AND (CHAR_LENGTH(sale_token_amount_raw)<78 OR sale_token_amount_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
    total_raised_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(total_raised_raw)<78 OR total_raised_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
    total_tokens_deposited_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(total_tokens_deposited_raw)<78 OR total_tokens_deposited_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
    total_tokens_claimed_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(total_tokens_claimed_raw)<78 OR total_tokens_claimed_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
    participant_count_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(participant_count_raw)<78 OR participant_count_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
    (final_total_raised_raw IS NULL OR (final_total_raised_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(final_total_raised_raw)<78 OR final_total_raised_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935'))) AND
    (final_price_numerator_raw IS NULL OR (final_price_numerator_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(final_price_numerator_raw)<78 OR final_price_numerator_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935'))) AND
    (final_price_denominator_raw IS NULL OR (final_price_denominator_raw REGEXP '^[1-9][0-9]{0,77}$' AND (CHAR_LENGTH(final_price_denominator_raw)<78 OR final_price_denominator_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935'))) AND
    platform_fee_reserved_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(platform_fee_reserved_raw)<78 OR platform_fee_reserved_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
    project_proceeds_paid_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(project_proceeds_paid_raw)<78 OR project_proceeds_paid_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
    platform_fee_paid_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(platform_fee_paid_raw)<78 OR platform_fee_paid_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
    project_proceeds_withdrawn IN(0,1) AND platform_fee_withdrawn IN(0,1) AND failed_project_tokens_withdrawn IN(0,1) AND tombstoned IN(0,1) AND
    state_generation>0 AND state_digest REGEXP '^[0-9a-f]{64}$' AND
    ((last_event_identity_id IS NULL AND last_placement_id IS NULL AND last_source_revision_id IS NULL AND last_effect_manifest_version IS NULL AND last_application_attempt_id IS NULL AND source_application_lineage_id IS NULL) OR
     (last_event_identity_id IS NOT NULL AND last_placement_id IS NOT NULL AND last_source_revision_id IS NOT NULL AND last_effect_manifest_version>0 AND last_application_attempt_id IS NOT NULL AND source_application_lineage_id IS NOT NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_position_v3 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  project_binding_id BIGINT UNSIGNED NOT NULL,
  run_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  contract_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  wallet_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  payment_token_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  payment_token_decimals TINYINT UNSIGNED NOT NULL,
  contribution_amount_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contribution_amount_display VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_token_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_token_decimals TINYINT UNSIGNED NOT NULL,
  claimed_tokens_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  claimed_tokens_display VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  refunded TINYINT UNSIGNED NOT NULL,
  first_contribution_event_id BIGINT UNSIGNED NULL,
  last_contribution_event_id BIGINT UNSIGNED NULL,
  last_claim_event_id BIGINT UNSIGNED NULL,
  last_refund_event_id BIGINT UNSIGNED NULL,
  last_event_identity_id BIGINT UNSIGNED NULL,
  last_placement_id BIGINT UNSIGNED NULL,
  last_source_revision_id BIGINT UNSIGNED NULL,
  last_effect_manifest_version INT UNSIGNED NULL,
  last_application_attempt_id BIGINT UNSIGNED NULL,
  source_application_lineage_id BIGINT UNSIGNED NULL,
  state_generation BIGINT UNSIGNED NOT NULL,
  state_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tombstoned TINYINT UNSIGNED NOT NULL,
  status VARCHAR(20) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_position_v3_identity(chain_id,contract_address,contract_version,project_id,wallet_address),
  KEY idx_fl_position_v3_wallet(wallet_address,project_id),
  CONSTRAINT fk_fl_position_v3_binding FOREIGN KEY(project_binding_id,chain_id,source_id,contract_address,contract_version,project_id) REFERENCES fair_launch_project_binding_v3(id,chain_id,source_id,contract_address,contract_version,project_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_position_v3_run_chain FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_local_e2e_run(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_position_v3_first FOREIGN KEY(first_contribution_event_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_position_v3_last_contribution FOREIGN KEY(last_contribution_event_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_position_v3_last_claim FOREIGN KEY(last_claim_event_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_position_v3_last_refund FOREIGN KEY(last_refund_event_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_position_v3 CHECK(
    chain_id=31337 AND contract_version='fair_launch_v1' AND project_id>0 AND
    contract_address REGEXP '^0x[0-9a-f]{40}$' AND wallet_address REGEXP '^0x[0-9a-f]{40}$' AND payment_token_address REGEXP '^0x[0-9a-f]{40}$' AND project_token_address REGEXP '^0x[0-9a-f]{40}$' AND
    payment_token_decimals BETWEEN 1 AND 18 AND project_token_decimals BETWEEN 1 AND 18 AND
    contribution_amount_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(contribution_amount_raw)<78 OR contribution_amount_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
    claimed_tokens_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(claimed_tokens_raw)<78 OR claimed_tokens_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
    refunded IN(0,1) AND tombstoned IN(0,1) AND status IN('canonical','reversed','quarantined') AND state_generation>0 AND state_digest REGEXP '^[0-9a-f]{64}$' AND
    ((last_event_identity_id IS NULL AND last_placement_id IS NULL AND last_source_revision_id IS NULL AND last_effect_manifest_version IS NULL AND last_application_attempt_id IS NULL AND source_application_lineage_id IS NULL) OR
     (last_event_identity_id IS NOT NULL AND last_placement_id IS NOT NULL AND last_source_revision_id IS NOT NULL AND last_effect_manifest_version>0 AND last_application_attempt_id IS NOT NULL AND source_application_lineage_id IS NOT NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
