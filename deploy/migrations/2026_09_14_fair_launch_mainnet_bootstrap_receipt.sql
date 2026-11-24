-- JoinCare Fair Launch BSC mainnet ProjectCreated bootstrap receipt.
-- ADDITIVE ONLY. Does not create an activation, lease, source, watermark or
-- wallet capability. It only permits the exact bootstrap provenance shape.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_require_fair_launch_mainnet_bootstrap_0914;
DELIMITER $$
CREATE PROCEDURE jc_require_fair_launch_mainnet_bootstrap_0914()
BEGIN
  IF COALESCE(@JC_FAIR_LAUNCH_MAINNET_BOOTSTRAP_APPROVED,0)<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet bootstrap receipt migration requires approved execution gate';
  END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_mainnet_activation_v1') OR
     NOT EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_project_state_v3') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet bootstrap receipt migration requires 0913';
  END IF;
END$$
DELIMITER ;
CALL jc_require_fair_launch_mainnet_bootstrap_0914();
DROP PROCEDURE jc_require_fair_launch_mainnet_bootstrap_0914;

CREATE TABLE IF NOT EXISTS fair_launch_mainnet_project_bootstrap_receipt_v1 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  activation_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  project_binding_id BIGINT UNSIGNED NOT NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  event_identity_id BIGINT UNSIGNED NOT NULL,
  placement_id BIGINT UNSIGNED NOT NULL,
  observation_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  placement_generation INT UNSIGNED NOT NULL,
  effect_manifest_version INT UNSIGNED NOT NULL,
  payload_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  binding_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  initial_state_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  receipt_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_mainnet_bootstrap_project(activation_row_id,project_id),
  UNIQUE KEY uk_fl_mainnet_bootstrap_binding(project_binding_id),
  UNIQUE KEY uk_fl_mainnet_bootstrap_event(event_identity_id),
  UNIQUE KEY uk_fl_mainnet_bootstrap_receipt(receipt_digest),
  CONSTRAINT fk_fl_mainnet_bootstrap_activation FOREIGN KEY(activation_row_id,chain_id) REFERENCES fair_launch_mainnet_activation_v1(authority_row_id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_bootstrap_binding FOREIGN KEY(project_binding_id) REFERENCES fair_launch_project_binding_v3(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_bootstrap_event FOREIGN KEY(event_identity_id,chain_id) REFERENCES chain_event_identity_v2(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_bootstrap_placement FOREIGN KEY(placement_id,event_identity_id,source_revision_id) REFERENCES chain_event_placement_v2(id,event_identity_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_bootstrap_observation FOREIGN KEY(observation_id,event_identity_id) REFERENCES chain_event_placement_observation_v2(id,event_identity_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_bootstrap_revision FOREIGN KEY(source_revision_id,source_id) REFERENCES chain_contract_source_revision(id,source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mainnet_bootstrap_receipt CHECK(
    chain_id=56 AND project_id>0 AND placement_generation>0 AND effect_manifest_version>0 AND
    payload_digest REGEXP '^[0-9a-f]{64}$' AND binding_digest REGEXP '^[0-9a-f]{64}$' AND
    initial_state_digest REGEXP '^[0-9a-f]{64}$' AND receipt_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE fair_launch_project_state_v3
  ADD COLUMN bootstrap_receipt_id BIGINT UNSIGNED NULL AFTER source_application_lineage_id,
  ADD UNIQUE KEY uk_fl_project_state_bootstrap_receipt(bootstrap_receipt_id),
  ADD CONSTRAINT fk_fl_project_state_bootstrap_receipt FOREIGN KEY(bootstrap_receipt_id) REFERENCES fair_launch_mainnet_project_bootstrap_receipt_v1(id) ON UPDATE RESTRICT ON DELETE RESTRICT;

ALTER TABLE fair_launch_project_state_v3 DROP CHECK chk_fl_project_state;
ALTER TABLE fair_launch_project_state_v3 ADD CONSTRAINT chk_fl_project_state CHECK(
  chain_id IN(31337,56) AND contract_version='fair_launch_v1' AND project_id>0 AND
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
  (
    (chain_id=31337 AND bootstrap_receipt_id IS NULL AND last_event_identity_id IS NULL AND last_placement_id IS NULL AND last_source_revision_id IS NULL AND last_effect_manifest_version IS NULL AND last_application_attempt_id IS NULL AND source_application_lineage_id IS NULL) OR
    (chain_id=31337 AND bootstrap_receipt_id IS NULL AND last_event_identity_id IS NOT NULL AND last_placement_id IS NOT NULL AND last_source_revision_id IS NOT NULL AND last_effect_manifest_version>0 AND last_application_attempt_id IS NOT NULL AND source_application_lineage_id IS NOT NULL) OR
    (chain_id=56 AND bootstrap_receipt_id IS NOT NULL AND last_event_identity_id IS NOT NULL AND last_placement_id IS NOT NULL AND last_source_revision_id IS NOT NULL AND last_effect_manifest_version>0 AND ((state_generation=1 AND last_application_attempt_id IS NULL AND source_application_lineage_id IS NULL) OR (last_application_attempt_id IS NOT NULL AND source_application_lineage_id IS NOT NULL)))
  )
) ENFORCED;

DROP TRIGGER IF EXISTS trg_fl_mainnet_bootstrap_receipt_no_update;
DROP TRIGGER IF EXISTS trg_fl_mainnet_bootstrap_receipt_no_delete;
DROP TRIGGER IF EXISTS trg_fl_mainnet_bootstrap_receipt_validate_insert;
DROP TRIGGER IF EXISTS trg_fl_mainnet_project_state_bootstrap_insert;
DROP TRIGGER IF EXISTS trg_fl_mainnet_project_state_bootstrap_update;
DELIMITER $$
CREATE TRIGGER trg_fl_mainnet_bootstrap_receipt_validate_insert BEFORE INSERT ON fair_launch_mainnet_project_bootstrap_receipt_v1 FOR EACH ROW BEGIN
  IF NOT EXISTS(
    SELECT 1
    FROM fair_launch_mainnet_activation_v1 a
    JOIN fair_launch_project_binding_v3 b ON b.id=NEW.project_binding_id
    JOIN chain_event_identity_v2 i ON i.id=NEW.event_identity_id
    JOIN chain_event_placement_v2 p ON p.id=NEW.placement_id AND p.event_identity_id=i.id
    JOIN chain_event_placement_observation_v2 o ON o.id=NEW.observation_id AND o.event_identity_id=i.id AND o.placement_id=p.id
    WHERE a.authority_row_id=NEW.activation_row_id AND a.chain_id=NEW.chain_id AND
      b.run_row_id=a.authority_row_id AND b.chain_id=a.chain_id AND b.project_id=NEW.project_id AND
      b.source_id=NEW.source_id AND b.source_revision_id=NEW.source_revision_id AND b.binding_digest=NEW.binding_digest AND
      i.chain_id=NEW.chain_id AND
      p.source_revision_id=NEW.source_revision_id AND p.generation=NEW.placement_generation AND
      p.canonical_slot=1 AND p.finalized_at IS NOT NULL AND p.noncanonical_at IS NULL
  ) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet project bootstrap receipt evidence mismatch'; END IF;
END$$
CREATE TRIGGER trg_fl_mainnet_bootstrap_receipt_no_update BEFORE UPDATE ON fair_launch_mainnet_project_bootstrap_receipt_v1 FOR EACH ROW BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_project_bootstrap_receipt_v1 is append-only';
END$$
CREATE TRIGGER trg_fl_mainnet_bootstrap_receipt_no_delete BEFORE DELETE ON fair_launch_mainnet_project_bootstrap_receipt_v1 FOR EACH ROW BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_project_bootstrap_receipt_v1 is append-only';
END$$
CREATE TRIGGER trg_fl_mainnet_project_state_bootstrap_insert BEFORE INSERT ON fair_launch_project_state_v3 FOR EACH ROW BEGIN
  IF NEW.chain_id=56 AND NOT EXISTS(
    SELECT 1 FROM fair_launch_mainnet_project_bootstrap_receipt_v1 r
    WHERE r.id=NEW.bootstrap_receipt_id AND r.activation_row_id=NEW.run_row_id AND r.chain_id=NEW.chain_id AND
      r.project_binding_id=NEW.project_binding_id AND r.project_id=NEW.project_id AND r.event_identity_id=NEW.last_event_identity_id AND
      r.placement_id=NEW.last_placement_id AND r.source_id=NEW.source_id AND r.source_revision_id=NEW.last_source_revision_id AND
      r.effect_manifest_version=NEW.last_effect_manifest_version AND r.initial_state_digest=NEW.state_digest
  ) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet project bootstrap receipt mismatch'; END IF;
END$$
CREATE TRIGGER trg_fl_mainnet_project_state_bootstrap_update BEFORE UPDATE ON fair_launch_project_state_v3 FOR EACH ROW BEGIN
  IF OLD.bootstrap_receipt_id<>NEW.bootstrap_receipt_id OR (OLD.bootstrap_receipt_id IS NULL)<>(NEW.bootstrap_receipt_id IS NULL) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch project bootstrap receipt identity is immutable';
  END IF;
  IF NEW.chain_id=56 AND NOT EXISTS(
    SELECT 1 FROM fair_launch_mainnet_project_bootstrap_receipt_v1 r
    WHERE r.id=NEW.bootstrap_receipt_id AND r.activation_row_id=NEW.run_row_id AND r.chain_id=NEW.chain_id AND
      r.project_binding_id=NEW.project_binding_id AND r.project_id=NEW.project_id AND r.source_id=NEW.source_id
  ) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet project bootstrap receipt authority mismatch'; END IF;
END$$
DELIMITER ;
