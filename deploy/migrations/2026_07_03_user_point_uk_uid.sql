-- ============================================================
-- Fix: user_point 表 uid 列缺 UNIQUE 索引，导致 points.ChangePoints 里
-- clause.OnConflict{Columns: [uid]} 的 UPSERT 从未触发 → 每次 INSERT
-- 累积多条行（uid=19 曾出现 3 行：50/60/50）。
--
-- 幂等：DELETE 只在有重复时命中；CREATE UNIQUE INDEX 若已存在会失败，
-- 用 SET @sql = ... 分支写法避免误报。
-- ============================================================

-- 1) 清理每个 uid 只保留最大 point、id 最小的那一行
DELETE up1 FROM user_point up1
  INNER JOIN user_point up2
  ON up1.uid = up2.uid
  AND (up1.point < up2.point OR (up1.point = up2.point AND up1.id > up2.id));

-- 2) 加 UNIQUE 索引（存在则跳过）
SET @exists := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'user_point'
    AND INDEX_NAME = 'uk_uid'
);
SET @sql := IF(@exists = 0, 'ALTER TABLE user_point ADD UNIQUE INDEX uk_uid (uid)', 'SELECT ''uk_uid already exists'' AS msg');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
