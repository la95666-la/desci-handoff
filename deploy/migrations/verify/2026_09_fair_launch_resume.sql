-- Safe resume verifier for an interrupted Fair Launch migration ledger row.
-- Exact shape is supplied from the independently reviewed schema manifest before this file.
DROP PROCEDURE IF EXISTS jc_fl_assert_resume;
DELIMITER $$
CREATE PROCEDURE jc_fl_assert_resume()
BEGIN
  DECLARE v_count BIGINT DEFAULT 0;
  IF @JC_FAIR_LAUNCH_RESUME_FILENAME NOT IN (
    '2026_09_03_fair_launch_project_model.sql','2026_09_04_fair_launch_ce_v2_governance.sql',
    '2026_09_05_fair_launch_signing_release_control.sql','2026_09_06_fair_launch_action_execution.sql') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='resume filename is not an approved Fair Launch migration';
  END IF;
  SELECT COUNT(*) INTO v_count FROM schema_migrations WHERE filename=@JC_FAIR_LAUNCH_RESUME_FILENAME AND checksum=@JC_FAIR_LAUNCH_RESUME_CHECKSUM AND status='applying';
  IF v_count<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='resume ledger identity/checksum mismatch'; END IF;
  IF COALESCE(@JC_FL_RESUME_PHASE_DIGEST,'') NOT REGEXP '^[0-9a-f]{64}$' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='independently admitted resume phase digest missing'; END IF;
  IF COALESCE(@JC_FL_RESUME_SHAPE_MISMATCHES,-1)<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='interrupted Fair Launch schema exact-shape mismatch'; END IF;
  IF COALESCE(@JC_FL_RESUME_NONEMPTY_PARTIALS,-1)<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='non-empty partial migration object requires manual quarantine'; END IF;
END$$
DELIMITER ;
CALL jc_fl_assert_resume();
DROP PROCEDURE jc_fl_assert_resume;
SELECT 'fair-launch exact-shape safe resume precondition PASS' AS result;
