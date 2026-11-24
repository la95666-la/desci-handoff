-- Add an independent public-list discovery flag for Launchpad projects.
-- is_listed=0 hides a project from Home and Launchpad list APIs only. Direct
-- project detail URLs and all lifecycle, funding, claim and refund behavior
-- remain unchanged. Existing projects default to visible for compatibility.

SET @column_exists := (
  SELECT COUNT(*)
  FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'projects'
    AND COLUMN_NAME = 'is_listed'
);
SET @sql := IF(
  @column_exists = 0,
  'ALTER TABLE projects ADD COLUMN is_listed TINYINT(1) NOT NULL DEFAULT 1 AFTER launch_at',
  'SELECT ''projects.is_listed already exists'' AS msg'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @constraint_exists := (
  SELECT COUNT(*)
  FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE()
    AND TABLE_NAME = 'projects'
    AND CONSTRAINT_NAME = 'chk_projects_is_listed'
    AND CONSTRAINT_TYPE = 'CHECK'
);
SET @sql := IF(
  @constraint_exists = 0,
  'ALTER TABLE projects ADD CONSTRAINT chk_projects_is_listed CHECK (is_listed IN (0,1))',
  'SELECT ''chk_projects_is_listed already exists'' AS msg'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @index_exists := (
  SELECT COUNT(*)
  FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'projects'
    AND INDEX_NAME = 'idx_projects_public_list'
);
SET @sql := IF(
  @index_exists = 0,
  'ALTER TABLE projects ADD INDEX idx_projects_public_list (is_listed, status, id)',
  'SELECT ''idx_projects_public_list already exists'' AS msg'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
