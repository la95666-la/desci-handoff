-- Daily per-inviter admission cap.
-- Registrations above the cap remain valid but do not create invite_record rows,
-- so they cannot later generate invite reward points.
-- This migration starts no jobs and grants no runtime or chain authority.

SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_require_settings_keys_unique_0920;
DELIMITER $$
CREATE PROCEDURE jc_require_settings_keys_unique_0920()
BEGIN
  IF EXISTS (
    SELECT 1
    FROM settings
    GROUP BY `key`
    HAVING COUNT(*) > 1
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='duplicate settings keys must be resolved before 0920';
  END IF;
END$$
DELIMITER ;
CALL jc_require_settings_keys_unique_0920();
DROP PROCEDURE jc_require_settings_keys_unique_0920;

SET @exists := (
  SELECT COUNT(*)
  FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'settings'
    AND INDEX_NAME = 'uk_settings_key'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE settings ADD UNIQUE INDEX uk_settings_key (`key`)',
  'SELECT ''uk_settings_key already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

INSERT INTO settings (`key`, `value`, `type`)
VALUES ('invite.daily_cap', '10', 'int')
ON DUPLICATE KEY UPDATE `key` = VALUES(`key`);

SET @exists := (
  SELECT COUNT(*)
  FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'invite_record'
    AND INDEX_NAME = 'idx_inviter_created_at'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE invite_record ADD INDEX idx_inviter_created_at (inviter_uid, created_at)',
  'SELECT ''idx_inviter_created_at already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
