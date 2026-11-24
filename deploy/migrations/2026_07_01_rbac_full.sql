-- ============================================================
-- Batch 1: 完整 RBAC 数据模型（N:M 角色 + 审计日志 + 4 眼工单 + 软删除 + 登录安全）
-- 参考 design §3.8.1 - §3.8.3
-- ============================================================

-- ------------------------------------------------------------
-- 1. admin_user_roles: 管理员 ↔ 角色 N:M
--    设计原文 "用户可拥有多个角色"，替代 admin_users.role_code 单值方案
--    role_code 字段保留为"主角色"（兼容老登录 UI 显示），实际权限校验走 join 表
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `admin_user_roles` (
  `id`         BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
  `admin_id`   BIGINT(20) UNSIGNED NOT NULL,
  `role_code`  VARCHAR(32) NOT NULL,
  `granted_by` BIGINT(20) UNSIGNED NULL COMMENT '授予人 admin_users.id（初始 seed 为 NULL）',
  `granted_at` DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `uk_admin_role` (`admin_id`, `role_code`) USING BTREE,
  INDEX `idx_role` (`role_code`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = 'admin 用户-角色 N:M';

-- 迁移旧数据：把 admin_users.role_code 的现有单值同步到 admin_user_roles
INSERT IGNORE INTO `admin_user_roles` (`admin_id`, `role_code`, `granted_at`)
  SELECT `id`, `role_code`, `created_at` FROM `admin_users` WHERE `role_code` <> '';

-- ------------------------------------------------------------
-- 2. admin_audit_log: 操作审计日志（仅追加 + 哈希链）
--    prev_hash + payload → sha256 = hash；核对时可从任意点重算链条一致性
--    设计原文 §3.8.2 "操作日志被篡改 → 日志单独 DB 实例 + 仅追加 + 哈希链式校验"
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `admin_audit_log` (
  `id`         BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
  `admin_id`   BIGINT(20) UNSIGNED NOT NULL,
  `username`   VARCHAR(64) NOT NULL DEFAULT '',
  `role_codes` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '逗号分隔当时的角色集',
  `ip`         VARCHAR(45)  NOT NULL DEFAULT '',
  `user_agent` VARCHAR(500) NOT NULL DEFAULT '',
  `method`     VARCHAR(10)  NOT NULL COMMENT 'GET/POST/PUT/DELETE',
  `path`       VARCHAR(255) NOT NULL,
  `route`      VARCHAR(255) NOT NULL DEFAULT '' COMMENT '路由模板，如 /admin/v1/users/:id/disable',
  `target_id`  VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '路径中主对象 ID',
  `req_body`   MEDIUMTEXT   NULL COMMENT 'JSON；敏感字段应 mask 后写',
  `resp_status` INT(11)     NOT NULL DEFAULT 0,
  `resp_hash`  CHAR(64)     NOT NULL DEFAULT '' COMMENT 'sha256 hex 返回体（防篡改校验，非明文避免占空间）',
  `prev_hash`  CHAR(64)     NOT NULL DEFAULT '' COMMENT '上一条 hash（链式）',
  `hash`       CHAR(64)     NOT NULL COMMENT 'sha256(prev_hash + admin_id + method + path + req_body + resp_status + resp_hash + created_at)',
  `duration_ms` INT(11)     NOT NULL DEFAULT 0,
  `note`       VARCHAR(500) NOT NULL DEFAULT '' COMMENT '业务上下文注释',
  `created_at` DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_admin_created` (`admin_id`, `created_at`) USING BTREE,
  INDEX `idx_target`   (`target_id`, `created_at`) USING BTREE,
  INDEX `idx_created`  (`created_at`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '管理员操作审计（仅追加）';

-- ------------------------------------------------------------
-- 3. admin_approval: 4 眼审批工单
--    高风险操作提交 → status=pending → 二审 approve/reject → executed
--    payload 存原始入参 JSON，approve 时后端**重放**执行（不信前端）
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `admin_approval` (
  `id`             BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
  `operation`      VARCHAR(64)  NOT NULL COMMENT '操作 code，如 user.disable / user.adjust_point / admin.create',
  `target_kind`    VARCHAR(32)  NOT NULL DEFAULT '' COMMENT 'user/admin/news/announcement/…',
  `target_id`      VARCHAR(64)  NOT NULL DEFAULT '',
  `payload`        MEDIUMTEXT   NOT NULL COMMENT '原始入参 JSON（含所有必要字段用于重放）',
  `reason`         VARCHAR(500) NOT NULL DEFAULT '' COMMENT '发起人给的执行理由',
  `initiator_id`   BIGINT(20) UNSIGNED NOT NULL,
  `initiator_name` VARCHAR(64)  NOT NULL DEFAULT '',
  `approver_id`    BIGINT(20) UNSIGNED NULL,
  `approver_name`  VARCHAR(64)  NOT NULL DEFAULT '',
  `status`         VARCHAR(16)  NOT NULL DEFAULT 'pending' COMMENT 'pending/approved/rejected/expired/executed/failed',
  `reject_reason`  VARCHAR(500) NOT NULL DEFAULT '',
  `result`         TEXT         NULL COMMENT '执行结果/错误（JSON）',
  `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `reviewed_at`    DATETIME     NULL,
  `executed_at`    DATETIME     NULL,
  `expires_at`     DATETIME     NULL COMMENT '过期时间；到期未审批自动 expired',
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_status_created` (`status`, `created_at`) USING BTREE,
  INDEX `idx_initiator`      (`initiator_id`) USING BTREE,
  INDEX `idx_target`         (`target_kind`, `target_id`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '4 眼审批工单';

-- ------------------------------------------------------------
-- 4. 关键实体加 deleted_at（软删除 + 30 天可恢复）— 幂等版本
--    MySQL 8.0.29+ 支持 IF NOT EXISTS，用 information_schema 兼容更老版本
-- ------------------------------------------------------------
DELIMITER $$
DROP PROCEDURE IF EXISTS jc_add_col_if_missing$$
CREATE PROCEDURE jc_add_col_if_missing(IN tbl VARCHAR(64), IN col VARCHAR(64), IN spec TEXT)
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = tbl AND COLUMN_NAME = col
  ) THEN
    SET @sql = CONCAT('ALTER TABLE `', tbl, '` ADD COLUMN `', col, '` ', spec);
    PREPARE stmt FROM @sql;
    EXECUTE stmt;
    DEALLOCATE PREPARE stmt;
  END IF;
END$$
DROP PROCEDURE IF EXISTS jc_add_idx_if_missing$$
CREATE PROCEDURE jc_add_idx_if_missing(IN tbl VARCHAR(64), IN idx VARCHAR(64), IN col VARCHAR(64))
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = tbl AND INDEX_NAME = idx
  ) THEN
    SET @sql = CONCAT('ALTER TABLE `', tbl, '` ADD INDEX `', idx, '` (`', col, '`)');
    PREPARE stmt FROM @sql;
    EXECUTE stmt;
    DEALLOCATE PREPARE stmt;
  END IF;
END$$
DELIMITER ;

CALL jc_add_col_if_missing('news',         'deleted_at', 'DATETIME NULL DEFAULT NULL COMMENT ''软删除时间''');
CALL jc_add_col_if_missing('task',         'deleted_at', 'DATETIME NULL DEFAULT NULL');
CALL jc_add_col_if_missing('home_banner',  'deleted_at', 'DATETIME NULL DEFAULT NULL');
CALL jc_add_col_if_missing('announcement', 'deleted_at', 'DATETIME NULL DEFAULT NULL');
CALL jc_add_col_if_missing('admin_users',  'deleted_at', 'DATETIME NULL DEFAULT NULL');
CALL jc_add_idx_if_missing('news',         'idx_deleted_at', 'deleted_at');
CALL jc_add_idx_if_missing('task',         'idx_deleted_at', 'deleted_at');
CALL jc_add_idx_if_missing('home_banner',  'idx_deleted_at', 'deleted_at');
CALL jc_add_idx_if_missing('announcement', 'idx_deleted_at', 'deleted_at');
CALL jc_add_idx_if_missing('admin_users',  'idx_deleted_at', 'deleted_at');

-- ------------------------------------------------------------
-- 5. admin_users 加登录安全字段（幂等）
-- ------------------------------------------------------------
CALL jc_add_col_if_missing('admin_users', 'password_changed_at', 'DATETIME NULL DEFAULT NULL COMMENT ''上次改密时间；用于 90 天轮换提示''');
CALL jc_add_col_if_missing('admin_users', 'locked_until',        'DATETIME NULL DEFAULT NULL COMMENT ''锁定到期时间（5 次失败锁 30min）''');
CALL jc_add_col_if_missing('admin_users', 'failed_login_count',  'INT(11) NOT NULL DEFAULT 0 COMMENT ''连续失败次数（登录成功清零）''');

DROP PROCEDURE IF EXISTS jc_add_col_if_missing;
DROP PROCEDURE IF EXISTS jc_add_idx_if_missing;

-- ------------------------------------------------------------
-- 6. 补 seed：给 6 角色补全 M4 内容管理（news / banner / announcement）+ M7 用户管理 权限
--    module code 复用现有 (m1_identity/m2_project/m5_task/m7_user/m8_i18n/news)
--    Phase 3 新增 `content` 作为 news+banner+announcement 的合并模块
-- ------------------------------------------------------------
-- content 模块：内容运营都能读写；super_admin/operator 有写权；其他只读
INSERT IGNORE INTO `admin_role_permissions` (`role_code`, `module`, `action`, `scope`) VALUES
  ('super_admin','content','read','all'),  ('super_admin','content','write','all'),
  ('super_admin','m7_user','write','all'), ('super_admin','m7_user','approve','all'),
  ('super_admin','m8_i18n','write','all'),
  ('super_admin','audit','read','all'),
  ('super_admin','rbac','write','all'),    -- 权限管理本身
  ('super_admin','rbac','read','all'),
  ('operator','content','read','all'),     ('operator','content','write','all'),
  ('operator','m8_i18n','read','all'),     ('operator','m8_i18n','write','all'),
  ('operator','m7_user','read','all'),     -- 只读用户
  ('approver','content','read','all'),
  ('approver','audit','read','all'),
  ('auditor','content','read','all'),
  ('auditor','audit','read','all'),        -- 审计员看审计日志
  ('auditor','m7_user','read','all'),
  ('auditor','m2_project','read','all'),
  ('compliance','content','read','all'),
  ('compliance','audit','read','all'),
  ('compliance','m7_user','read','all'),   ('compliance','m7_user','approve','all'),
  ('support','content','read','all'),
  ('support','m7_user','read','all');
