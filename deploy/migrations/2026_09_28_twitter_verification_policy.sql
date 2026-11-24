-- Admin-controlled X task verification policy.
-- Safe default: 0 = supported X tasks must use the existing read-only X API.
-- Explicit 1 = skip only the external X API and use the existing transactional
-- task completion path. Task/version/UID uniqueness remains authoritative.
-- This migration starts no jobs and grants no runtime or chain authority.

SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_require_twitter_verification_setting_0928;
DELIMITER $$
CREATE PROCEDURE jc_require_twitter_verification_setting_0928()
BEGIN
  IF (
    SELECT COUNT(*)
    FROM settings
    WHERE `key` = 'twitter.verification_disabled'
  ) > 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='duplicate twitter verification settings must be resolved before 0928';
  END IF;
  IF EXISTS (
    SELECT 1
    FROM settings
    WHERE `key` = 'twitter.verification_disabled'
      AND TRIM(LOWER(`value`)) NOT IN ('0','1','false','true')
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid twitter verification setting must be resolved before 0928';
  END IF;
END$$
DELIMITER ;
CALL jc_require_twitter_verification_setting_0928();
DROP PROCEDURE jc_require_twitter_verification_setting_0928;

INSERT INTO settings (`key`, `value`, `type`)
VALUES ('twitter.verification_disabled', '0', 'int')
ON DUPLICATE KEY UPDATE `key` = VALUES(`key`);
