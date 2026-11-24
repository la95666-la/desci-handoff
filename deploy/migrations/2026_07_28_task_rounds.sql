-- X task rounds: changing a post target starts a new independently claimable
-- round while preserving previous completion and point-flow history.

SET @exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task' AND COLUMN_NAME = 'version'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE task ADD COLUMN version INT UNSIGNED NOT NULL DEFAULT 1 AFTER is_enable',
  'SELECT ''task.version already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task' AND COLUMN_NAME = 'target_ref'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE task ADD COLUMN target_ref VARCHAR(200) NOT NULL DEFAULT '''' AFTER version',
  'SELECT ''task.target_ref already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @round_started_was_missing := (
  SELECT IF(COUNT(*) = 0, 1, 0) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task' AND COLUMN_NAME = 'round_started_at'
);
SET @sql := IF(@round_started_was_missing = 1,
  'ALTER TABLE task ADD COLUMN round_started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER target_ref',
  'SELECT ''task.round_started_at already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql := IF(@round_started_was_missing = 1,
  'UPDATE task SET round_started_at = created_at',
  'SELECT ''task.round_started_at retained'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task_record' AND COLUMN_NAME = 'task_version'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE task_record ADD COLUMN task_version INT UNSIGNED NOT NULL DEFAULT 1 AFTER platform',
  'SELECT ''task_record.task_version already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task_record' AND COLUMN_NAME = 'target_ref'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE task_record ADD COLUMN target_ref VARCHAR(200) NOT NULL DEFAULT '''' AFTER verify_status',
  'SELECT ''task_record.target_ref already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task_record' AND COLUMN_NAME = 'target_url'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE task_record ADD COLUMN target_url VARCHAR(500) NOT NULL DEFAULT '''' AFTER target_ref',
  'SELECT ''task_record.target_url already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Replace every historical one-task-per-user index variant with the
-- round-aware key. task_id is globally unique, so platform is audit data and
-- does not need to participate in uniqueness.
SET @exists := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task_record' AND INDEX_NAME = 'uk_task_uid'
);
SET @sql := IF(@exists > 0,
  'ALTER TABLE task_record DROP INDEX uk_task_uid',
  'SELECT ''uk_task_uid not present'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task_record' AND INDEX_NAME = 'uk_platform_task_uid'
);
SET @sql := IF(@exists > 0,
  'ALTER TABLE task_record DROP INDEX uk_platform_task_uid',
  'SELECT ''uk_platform_task_uid not present'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task_record' AND INDEX_NAME = 'uk_platform_task_version_uid'
);
SET @sql := IF(@exists > 0,
  'ALTER TABLE task_record DROP INDEX uk_platform_task_version_uid',
  'SELECT ''uk_platform_task_version_uid not present'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task_record' AND INDEX_NAME = 'uk_task_version_uid'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE task_record ADD UNIQUE INDEX uk_task_version_uid (task_id, task_version, uid)',
  'SELECT ''uk_task_version_uid already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `task_round` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `task_id` BIGINT UNSIGNED NOT NULL,
  `version` INT UNSIGNED NOT NULL DEFAULT 1,
  `target_ref` VARCHAR(200) NOT NULL,
  `target_url` VARCHAR(500) NOT NULL DEFAULT '',
  `amount_point` INT NOT NULL DEFAULT 0,
  `started_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE INDEX `uk_task_round` (`task_id`, `version`),
  UNIQUE INDEX `uk_task_target` (`task_id`, `target_ref`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Post-bound X task round history';

-- Normalize the currently configured post for existing tasks. Host changes,
-- query strings and trailing path fragments do not change the extracted ID.
UPDATE task
SET target_ref = SUBSTRING_INDEX(
  SUBSTRING_INDEX(SUBSTRING_INDEX(url, '/status/', -1), '?', 1), '/', 1
)
WHERE platform = 'twitter'
  AND task_key IN ('like', 'retweet', 'reply', 'bookmark')
  AND target_ref = ''
  AND url LIKE '%/status/%';

INSERT INTO task_round
  (task_id, version, target_ref, target_url, amount_point, started_at, created_at)
SELECT id, version, target_ref, url, amount_point, round_started_at, round_started_at
FROM task
WHERE platform = 'twitter'
  AND task_key IN ('like', 'retweet', 'reply', 'bookmark')
  AND target_ref <> ''
ON DUPLICATE KEY UPDATE task_id = VALUES(task_id);

-- All pre-migration records belong to round 1. Snapshot the target that was
-- current when this feature was introduced; future writes store snapshots in
-- application code.
UPDATE task_record r
JOIN task t ON t.id = r.task_id
SET r.task_version = 1,
    r.target_ref = IF(r.target_ref = '', t.target_ref, r.target_ref),
    r.target_url = IF(r.target_url = '', t.url, r.target_url)
WHERE r.task_version = 1;
