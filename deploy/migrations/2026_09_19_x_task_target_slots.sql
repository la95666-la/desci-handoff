-- X post task target slots.
-- Existing tasks remain slot 1. Like/reply/retweet may additionally use slot 2,
-- with independent points, enablement, version rounds and completion records.
-- This migration only changes task configuration shape; it starts no jobs and
-- grants no runtime, chain, scanner or materializer authority.

SET NAMES utf8mb4;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task' AND COLUMN_NAME = 'target_slot'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE task ADD COLUMN target_slot TINYINT UNSIGNED NOT NULL DEFAULT 1 AFTER task_key',
  'SELECT ''task.target_slot already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

DROP PROCEDURE IF EXISTS jc_require_x_task_history_0919;
DELIMITER $$
CREATE PROCEDURE jc_require_x_task_history_0919()
BEGIN
  IF EXISTS(
    SELECT 1
    FROM task_round r
    JOIN task t ON t.id = r.task_id
    WHERE t.platform = 'twitter'
      AND t.task_key IN ('like','reply','retweet')
    GROUP BY t.platform, t.task_key, r.target_ref
    HAVING COUNT(*) > 1
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='duplicate historical X action targets must be resolved before 0919';
  END IF;
END$$
DELIMITER ;
CALL jc_require_x_task_history_0919();
DROP PROCEDURE jc_require_x_task_history_0919;

-- Cross-slot history is globally unique per interaction type. A post that was
-- already rewarded in slot 1 cannot later be recycled into slot 2 (or vice
-- versa), even after the original slot advances to a new round.
CREATE TABLE IF NOT EXISTS `task_target_history` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `platform` VARCHAR(20) NOT NULL,
  `task_key` VARCHAR(50) NOT NULL,
  `target_ref` VARCHAR(200) NOT NULL,
  `task_id` BIGINT UNSIGNED NOT NULL,
  `task_version` INT UNSIGNED NOT NULL,
  `target_slot` TINYINT UNSIGNED NOT NULL,
  `target_url` VARCHAR(500) NOT NULL DEFAULT '',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE INDEX `uk_task_target_history` (`platform`, `task_key`, `target_ref`),
  UNIQUE INDEX `uk_task_target_history_round` (`task_id`, `task_version`),
  CONSTRAINT `chk_task_target_history_slot` CHECK (`target_slot` IN (1,2)) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Cross-slot X post reward target history';

INSERT INTO task_target_history
  (platform, task_key, target_ref, task_id, task_version, target_slot, target_url, created_at)
SELECT t.platform, t.task_key, r.target_ref, r.task_id, r.version,
       COALESCE(NULLIF(t.target_slot,0),1), r.target_url, r.created_at
FROM task_round r
JOIN task t ON t.id = r.task_id
WHERE t.platform = 'twitter'
  AND t.task_key IN ('like','reply','retweet')
ON DUPLICATE KEY UPDATE task_id = VALUES(task_id);

UPDATE task SET target_slot = 1 WHERE target_slot IS NULL OR target_slot = 0;

-- Existing data must already satisfy the new identity rule. Abort rather than
-- silently choosing a winner if historical duplicates are present.
DROP PROCEDURE IF EXISTS jc_require_x_task_slots_0919;
DELIMITER $$
CREATE PROCEDURE jc_require_x_task_slots_0919()
BEGIN
  IF EXISTS(
    SELECT 1 FROM task
    GROUP BY platform, task_key, target_slot
    HAVING COUNT(*) > 1
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='duplicate task platform/key/slot rows must be resolved before 0919';
  END IF;
  IF EXISTS(
    SELECT 1 FROM task
    GROUP BY platform, task_key, target_ref
    HAVING COUNT(*) > 1
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='duplicate task platform/key/target rows must be resolved before 0919';
  END IF;
END$$
DELIMITER ;
CALL jc_require_x_task_slots_0919();
DROP PROCEDURE jc_require_x_task_slots_0919;

-- Replace the historical one-row-per-platform/key uniqueness rule with a
-- one-row-per-slot rule. Index names are checked dynamically for safe replay.
SET @old_unique_index := (
  SELECT INDEX_NAME
  FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'task'
    AND NON_UNIQUE = 0
  GROUP BY INDEX_NAME
  HAVING GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX) = 'platform,task_key'
  LIMIT 1
);
SET @sql := IF(@old_unique_index IS NULL,
  'SELECT ''legacy task platform/key unique index not present'' AS msg',
  CONCAT('ALTER TABLE task DROP INDEX `', REPLACE(@old_unique_index, '`', '``'), '`'));
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task' AND INDEX_NAME = 'uk_platform_key_slot'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE task ADD UNIQUE INDEX uk_platform_key_slot (platform, task_key, target_slot)',
  'SELECT ''uk_platform_key_slot already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- A post cannot occupy both slots for the same action. This also preserves
-- the historical one-row identity for non-post tasks whose target_ref is ''.
SET @exists := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task' AND INDEX_NAME = 'uk_platform_key_target'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE task ADD UNIQUE INDEX uk_platform_key_target (platform, task_key, target_ref)',
  'SELECT ''uk_platform_key_target already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Database-level fail-closed enforcement. Only X post actions support slot 2;
-- every other task type remains a single slot task.
DROP TRIGGER IF EXISTS bi_task_target_slot_0919;
DELIMITER $$
CREATE TRIGGER bi_task_target_slot_0919
BEFORE INSERT ON task
FOR EACH ROW
BEGIN
  IF NEW.target_slot NOT IN (1, 2) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='task target_slot must be 1 or 2';
  END IF;
  IF NEW.target_slot = 2 AND NOT (
    LOWER(NEW.platform)='twitter' AND LOWER(NEW.task_key) IN ('like','reply','retweet')
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='task target_slot 2 is limited to X post tasks';
  END IF;
END$$
DELIMITER ;

DROP TRIGGER IF EXISTS bu_task_target_slot_0919;
DELIMITER $$
CREATE TRIGGER bu_task_target_slot_0919
BEFORE UPDATE ON task
FOR EACH ROW
BEGIN
  IF NEW.target_slot NOT IN (1, 2) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='task target_slot must be 1 or 2';
  END IF;
  IF NEW.target_slot = 2 AND NOT (
    LOWER(NEW.platform)='twitter' AND LOWER(NEW.task_key) IN ('like','reply','retweet')
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='task target_slot 2 is limited to X post tasks';
  END IF;
END$$
DELIMITER ;
