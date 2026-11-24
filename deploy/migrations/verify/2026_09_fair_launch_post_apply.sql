-- Fail-closed post-apply verification. Every assertion SIGNALs on mismatch.
DROP PROCEDURE IF EXISTS jc_fl_assert_post_apply;
DELIMITER $$
CREATE PROCEDURE jc_fl_assert_post_apply()
BEGIN
  DECLARE v_count BIGINT DEFAULT 0;

  IF COALESCE(@JC_FL_SCHEMA_SHAPE_MISMATCHES,-1)<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reviewed exact normalized schema shape mismatch'; END IF;
  IF COALESCE(@JC_FL_SCHEMA_MANIFEST_DIGEST,'') NOT REGEXP '^[0-9a-f]{64}$' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='schema manifest digest missing'; END IF;

  SELECT COUNT(*) INTO v_count FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects'
    AND COLUMN_NAME IN ('funding_mode','sale_token_amount_raw','min_contribution_raw','max_contribution_raw','final_total_raised_raw','final_sale_token_amount_raw','settlement_block_number','settlement_block_hash','settlement_parent_block_number','settlement_parent_block_hash','release_attestation_digest');
  IF v_count<>11 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='post-apply project column count must equal 11'; END IF;
  SELECT COUNT(*) INTO v_count FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='funding_mode' AND IS_NULLABLE='NO';
  IF v_count<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='funding_mode must be NOT NULL'; END IF;

  SELECT COUNT(*) INTO v_count FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN ('chain_contract_source','chain_contract_source_revision','chain_event_identity_v2','chain_event_placement_v2','chain_event_placement_observation_v2','chain_event_application_attempt_v2','fund_ledger_v2_shadow','ce_v2_cutover_control');
  IF v_count<>8 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='post-apply CE-V2 table count must equal 8'; END IF;
  SELECT COUNT(*) INTO v_count FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN ('fair_launch_signing_generation','fair_launch_gateway_signing_journal','fair_launch_source_registry_snapshot','fair_launch_feature_watermark','fair_launch_restore_recovery_attestation','fair_launch_action_request','fair_launch_action_transition');
  IF v_count<>7 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='post-apply control/action table count must equal 7'; END IF;

  SELECT COUNT(*) INTO v_count FROM projects WHERE funding_mode IS NULL;
  IF v_count<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='post-apply NULL funding_mode blocker'; END IF;
  SELECT COUNT(*) INTO v_count FROM fair_launch_project_inventory WHERE inventory_state='quarantined';
  IF v_count<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='post-apply project inventory quarantine blocker'; END IF;
  SELECT COUNT(*) INTO v_count FROM chain_event_identity_quarantine_v2;
  IF v_count<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='post-apply identity quarantine blocker'; END IF;
  SELECT COUNT(*) INTO v_count FROM chain_event_payload_conflict_v2 WHERE resolved_at IS NULL;
  IF v_count<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='post-apply payload conflict blocker'; END IF;
  SELECT COUNT(*) INTO v_count FROM fund_ledger_v2_provenance_gap_v;
  IF v_count<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='post-apply shadow provenance blocker'; END IF;
  SELECT COUNT(*) INTO v_count FROM ce_v2_cutover_control c WHERE c.phase='m5_fenced' AND NOT EXISTS (SELECT 1 FROM ce_v2_m5_gate_v g WHERE g.chain_id=c.chain_id);
  IF v_count<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='post-apply M5 blocker'; END IF;
  SELECT COUNT(*) INTO v_count FROM chain_tx_submission s WHERE s.final_event_application_id_v2 IS NOT NULL AND NOT EXISTS (SELECT 1 FROM ce_v2_m6_final_event_gate_v g WHERE g.id=s.id);
  IF v_count<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='post-apply M6 final-event blocker'; END IF;
END$$
DELIMITER ;
CALL jc_fl_assert_post_apply();
DROP PROCEDURE jc_fl_assert_post_apply;
SELECT 'fair-launch post-apply exact assertions PASS' AS result;
