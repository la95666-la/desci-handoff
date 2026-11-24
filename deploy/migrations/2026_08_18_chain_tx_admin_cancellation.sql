-- JoinCare IDO Sprint 1A-P2 · durable administrator cancellation evidence.
-- This migration extends 2026_08_17; do not run it independently.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_tx_admin_cancellation;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_tx_admin_cancellation()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_tx_submission' AND TABLE_TYPE = 'BASE TABLE'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain_tx_submission prerequisite table is missing';
  END IF;
  SET @cancel_columns = (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_tx_submission'
      AND COLUMN_NAME IN ('cancelled_by_admin_id','cancel_reason','cancelled_at')
  );
  IF @cancel_columns NOT IN (0, 3) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'partial chain transaction cancellation schema requires manual recovery';
  END IF;
  IF @cancel_columns = 3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'pre-existing chain transaction cancellation schema requires manual verification';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_tx_submission'
      AND CONSTRAINT_NAME IN ('chk_chain_tx_state','chk_chain_tx_active_state','chk_chain_tx_signed_shape')
      AND CONSTRAINT_TYPE = 'CHECK' AND ENFORCED = 'YES'
  ) <> 3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = '2026_08_17 chain transaction checks are missing';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_tx_admin_cancellation();
DROP PROCEDURE jc_preflight_chain_tx_admin_cancellation;

ALTER TABLE `chain_tx_submission`
  DROP CHECK `chk_chain_tx_state`,
  DROP CHECK `chk_chain_tx_active_state`,
  DROP CHECK `chk_chain_tx_signed_shape`,
  ADD COLUMN `cancelled_by_admin_id` BIGINT UNSIGNED NULL AFTER `finalized_at`,
  ADD COLUMN `cancel_reason` VARCHAR(500) NOT NULL DEFAULT '' AFTER `cancelled_by_admin_id`,
  ADD COLUMN `cancelled_at` DATETIME(6) NULL AFTER `cancel_reason`,
  ADD CONSTRAINT `chk_chain_tx_state` CHECK (`state` IN ('intent','ready','signed','submitted','broadcast_unknown','mined_success','mined_revert','final_success','final_revert','applied','cancelled','admin_cancelled','ambiguous')) ENFORCED,
  ADD CONSTRAINT `chk_chain_tx_active_state` CHECK (
    (`state` IN ('applied','cancelled','admin_cancelled','final_revert') AND `active_slot` IS NULL)
    OR (`state` NOT IN ('applied','cancelled','admin_cancelled','final_revert') AND `active_slot` = 1)
  ) ENFORCED,
  ADD CONSTRAINT `chk_chain_tx_signed_shape` CHECK (
    (`state` IN ('intent','ready','cancelled','admin_cancelled') AND `nonce` IS NULL AND `gas_limit` IS NULL AND `raw_tx` IS NULL AND `tx_hash` IS NULL)
    OR (`state` NOT IN ('intent','ready','cancelled','admin_cancelled') AND `nonce` IS NOT NULL AND `gas_limit` IS NOT NULL AND OCTET_LENGTH(`raw_tx`) > 0 AND `tx_hash` IS NOT NULL)
  ) ENFORCED,
  ADD CONSTRAINT `chk_chain_tx_admin_cancellation` CHECK (
    (`state` = 'admin_cancelled' AND `cancelled_by_admin_id` IS NOT NULL AND `cancelled_by_admin_id` > 0 AND CHAR_LENGTH(TRIM(`cancel_reason`)) BETWEEN 3 AND 500 AND `cancelled_at` IS NOT NULL)
    OR (`state` <> 'admin_cancelled' AND `cancelled_by_admin_id` IS NULL AND `cancel_reason` = '' AND `cancelled_at` IS NULL)
  ) ENFORCED;

DROP PROCEDURE IF EXISTS jc_assert_chain_tx_admin_cancellation;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_tx_admin_cancellation()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_tx_submission' AND (
      (COLUMN_NAME = 'cancelled_by_admin_id' AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'YES')
      OR (COLUMN_NAME = 'cancel_reason' AND COLUMN_TYPE = 'varchar(500)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
      OR (COLUMN_NAME = 'cancelled_at' AND COLUMN_TYPE = 'datetime(6)' AND IS_NULLABLE = 'YES')
    )
  ) <> 3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain transaction cancellation columns are incompatible';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_tx_submission'
      AND CONSTRAINT_NAME IN ('chk_chain_tx_state','chk_chain_tx_active_state','chk_chain_tx_signed_shape','chk_chain_tx_admin_cancellation')
      AND CONSTRAINT_TYPE = 'CHECK' AND ENFORCED = 'YES'
  ) <> 4 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain transaction cancellation checks are incomplete';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.CHECK_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME = 'chk_chain_tx_admin_cancellation'
      AND CHECK_CLAUSE LIKE '%admin_cancelled%cancelled_by_admin_id%cancel_reason%cancelled_at%'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain transaction cancellation check clause is incompatible';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_tx_admin_cancellation();
DROP PROCEDURE jc_assert_chain_tx_admin_cancellation;
