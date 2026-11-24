-- JoinCare IDO Sprint 1A-P0
-- 1) 修复 chain_event_record 错误的单列唯一索引；事件身份应为 (block_number, tx_hash, log_index)
-- 2) 链上 create/activate 仅授予 super_admin execute；项目申请审批授予 approver approve
--
-- 执行前备份：
--   mysqldump --single-transaction "$JC_MYSQL_DB" chain_event_record admin_role_permissions \
--     > backup_2026_08_13_ido_event_reliability.sql

SET NAMES utf8mb4;

-- 若已有重复事件，拒绝自动建唯一索引；先人工核对业务聚合，禁止直接删事实记录。
DROP PROCEDURE IF EXISTS jc_assert_chain_event_identity_unique;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_identity_unique()
BEGIN
  IF EXISTS (
    SELECT 1
    FROM chain_event_record
    GROUP BY block_number, tx_hash, log_index
    HAVING COUNT(*) > 1
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'duplicate chain_event_record identities found; reconcile before migration';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_identity_unique();
DROP PROCEDURE jc_assert_chain_event_identity_unique;

-- 删除 block_number/log_index 上既存的单列唯一索引，索引名按 information_schema 动态识别。
DROP PROCEDURE IF EXISTS jc_drop_chain_event_single_unique_indexes;
DELIMITER $$
CREATE PROCEDURE jc_drop_chain_event_single_unique_indexes()
BEGIN
  DECLARE done INT DEFAULT 0;
  DECLARE idx_name VARCHAR(64);
  DECLARE idx_cursor CURSOR FOR
    SELECT s.INDEX_NAME
    FROM information_schema.STATISTICS s
    WHERE s.TABLE_SCHEMA = DATABASE()
      AND s.TABLE_NAME = 'chain_event_record'
      AND s.NON_UNIQUE = 0
      AND s.INDEX_NAME <> 'PRIMARY'
    GROUP BY s.INDEX_NAME
    HAVING COUNT(*) = 1
       AND MAX(s.COLUMN_NAME) IN ('block_number', 'log_index');
  DECLARE CONTINUE HANDLER FOR NOT FOUND SET done = 1;

  OPEN idx_cursor;
  drop_loop: LOOP
    FETCH idx_cursor INTO idx_name;
    IF done = 1 THEN
      LEAVE drop_loop;
    END IF;
    SET @drop_sql = CONCAT('ALTER TABLE `chain_event_record` DROP INDEX `', REPLACE(idx_name, '`', '``'), '`');
    PREPARE stmt FROM @drop_sql;
    EXECUTE stmt;
    DEALLOCATE PREPARE stmt;
  END LOOP;
  CLOSE idx_cursor;
END$$
DELIMITER ;
CALL jc_drop_chain_event_single_unique_indexes();
DROP PROCEDURE jc_drop_chain_event_single_unique_indexes;

-- 幂等创建正确的事件复合唯一索引。
SET @has_identity_index = (
  SELECT COUNT(*)
  FROM (
    SELECT INDEX_NAME,
           GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') AS columns_in_order,
           MIN(NON_UNIQUE) AS non_unique
    FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_record'
    GROUP BY INDEX_NAME
  ) indexes_for_table
  WHERE columns_in_order = 'block_number,tx_hash,log_index' AND non_unique = 0
);
SET @create_identity_sql = IF(
  @has_identity_index = 0,
  'ALTER TABLE `chain_event_record` ADD UNIQUE INDEX `uk_chain_event_identity` (`block_number`, `tx_hash`, `log_index`)',
  'SELECT 1'
);
PREPARE stmt FROM @create_identity_sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

INSERT IGNORE INTO admin_role_permissions (role_code, module, action, scope)
VALUES
  ('super_admin', 'm2_project', 'execute', 'all'),
  ('approver', 'm2_project', 'approve', 'all');

-- 验证：
--   SHOW INDEX FROM chain_event_record;
--   SELECT role_code,module,action,scope FROM admin_role_permissions
--     WHERE module='m2_project' AND action='execute';
