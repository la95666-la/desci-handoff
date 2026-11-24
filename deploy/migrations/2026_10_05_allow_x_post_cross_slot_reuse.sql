-- Allow operators to assign an earlier slot-1 X post to slot 2 after slot 1
-- rotates. The task_round(task_id,target_ref) unique index and task_record
-- (task_id,task_version,uid) unique index remain in place for per-task reward
-- safety; task(platform,task_key,target_slot) remains unique.
-- Run this BEFORE rolling out the corresponding Backend task validation change.
-- Record SHOW INDEX FROM task / task_target_history before applying for rollback.

SET @exists := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task'
    AND INDEX_NAME = 'uk_platform_key_target'
);
SET @sql := IF(@exists > 0,
  'ALTER TABLE task DROP INDEX uk_platform_key_target',
  'SELECT ''uk_platform_key_target already absent'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Replace the cross-slot historical uniqueness with a non-unique lookup index.
-- The existing per-task/version uniqueness continues to protect round identity.
SET @exists := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task_target_history'
    AND INDEX_NAME = 'idx_task_target_history_lookup'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE task_target_history ADD INDEX idx_task_target_history_lookup (platform, task_key, target_ref)',
  'SELECT ''idx_task_target_history_lookup already present'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task_target_history'
    AND INDEX_NAME = 'uk_task_target_history'
);
SET @sql := IF(@exists > 0,
  'ALTER TABLE task_target_history DROP INDEX uk_task_target_history',
  'SELECT ''uk_task_target_history already absent'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
