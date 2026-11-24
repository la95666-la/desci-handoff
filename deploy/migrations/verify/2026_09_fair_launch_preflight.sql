-- Fail-closed production-like baseline preflight. Any non-zero blocker SIGNALs.
DROP PROCEDURE IF EXISTS jc_fl_assert_preflight;
DELIMITER $$
CREATE PROCEDURE jc_fl_assert_preflight()
BEGIN
  DECLARE v_count BIGINT DEFAULT 0;
  IF NOT EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects')
     OR NOT EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_event_record')
     OR NOT EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_event_failure')
     OR NOT EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_event_replay_audit')
     OR NOT EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_tx_submission') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='production-like baseline tables missing';
  END IF;
  SELECT COUNT(*) INTO v_count FROM projects WHERE contract_version='fair_launch_v1' AND onchain_locked<>0;
  IF v_count<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='preflight locked Fair Launch blocker'; END IF;
  IF EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='funding_mode') THEN
    SELECT COUNT(*) INTO v_count FROM projects WHERE funding_mode IS NOT NULL AND funding_mode NOT IN ('capped_presale','fair_launch');
    IF v_count<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='preflight unknown funding_mode blocker'; END IF;
    SELECT COUNT(*) INTO v_count FROM projects WHERE (funding_mode='fair_launch' AND contract_version NOT IN ('unassigned','fair_launch_v1')) OR (funding_mode='capped_presale' AND contract_version='fair_launch_v1');
    IF v_count<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='preflight discriminator mismatch blocker'; END IF;
  END IF;
  SELECT COUNT(*) INTO v_count FROM chain_event_failure WHERE status IN ('retrying','dead_letter');
  IF v_count<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='preflight active chain failure blocker'; END IF;
  SELECT COUNT(*) INTO v_count FROM chain_tx_submission WHERE active_slot=1;
  IF v_count<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='preflight active outbox slot blocker'; END IF;
END$$
DELIMITER ;
CALL jc_fl_assert_preflight();
DROP PROCEDURE jc_fl_assert_preflight;
SELECT 'fair-launch blocker-zero preflight PASS' AS result;
