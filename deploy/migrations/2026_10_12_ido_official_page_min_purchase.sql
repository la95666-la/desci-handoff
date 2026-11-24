-- Chain-independent, per-purchase minimum for the official V2 purchase UI only.
-- NULL disables the policy. Does not affect contract calls or event accounting.
SET @column_exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'projects'
    AND COLUMN_NAME = 'min_purchase_amount_raw'
);
SET @sql := IF(@column_exists = 0,
  'ALTER TABLE projects ADD COLUMN min_purchase_amount_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER max_per_user_raw',
  'SELECT ''projects.min_purchase_amount_raw already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
