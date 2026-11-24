-- ===================================================================
-- copywriting_config 双语聚合视图所需的 schema 调整 (2026-06-30)
--   1) value 列加宽到 TEXT（部分文案 > 255 字）
--   2) 加 UNIQUE INDEX (key, language) 防止 dup
-- ===================================================================
SET NAMES utf8mb4;

ALTER TABLE `copywriting_config`
  MODIFY COLUMN `value` TEXT NOT NULL COMMENT '文案内容（可长文）';

-- 加 UNIQUE INDEX，若已存在则忽略
SET @idx_exists := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE table_schema = DATABASE()
    AND table_name = 'copywriting_config'
    AND index_name = 'uk_key_lang');
SET @stmt := IF(@idx_exists = 0,
  'ALTER TABLE `copywriting_config` ADD UNIQUE INDEX `uk_key_lang` (`key`, `language`)',
  'SELECT "uk_key_lang already exists" AS note');
PREPARE s FROM @stmt; EXECUTE s; DEALLOCATE PREPARE s;
