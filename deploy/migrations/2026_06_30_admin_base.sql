-- ===================================================================
-- JoinCare admin platform — base schema (Phase 1)
--   1) admin_users      管理员账号
--   2) admin_roles      角色（6 预设：super_admin/operator/approver/compliance/support/auditor）
--   3) admin_role_permissions  角色 × 模块 × 动作（粗矩阵；Phase 1 仅作 RBAC 数据基座，UI 编辑器留 Phase 2）
--   4) 初始化 6 个预设角色（公开快照不预置管理员账号）
-- 对应 spec §0.3 / §5.1（M1 身份与基础设施）
-- ===================================================================
SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;


-- -------------------------------------------------------------------
-- 1. admin_users
-- -------------------------------------------------------------------
DROP TABLE IF EXISTS `admin_users`;
CREATE TABLE `admin_users` (
  `id`              BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '管理员 id',
  `username`        VARCHAR(64)  NOT NULL COMMENT '登录用户名',
  `password_hash`   VARCHAR(120) NOT NULL COMMENT 'bcrypt 散列',
  `email`           VARCHAR(120) NOT NULL DEFAULT '' COMMENT '邮箱（找回密码 / 通知）',
  `display_name`    VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '显示名',
  `role_code`       VARCHAR(32)  NOT NULL COMMENT '角色 code（关联 admin_roles.code）',
  `is_enable`       TINYINT(1)   NOT NULL DEFAULT 1 COMMENT '1 启用 / 0 停用',
  `last_login_at`   DATETIME     NULL DEFAULT NULL COMMENT '上次登录时间',
  `last_login_ip`   VARCHAR(45)  NOT NULL DEFAULT '' COMMENT '上次登录 IP（支持 IPv6）',
  `created_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `uk_username` (`username`) USING BTREE,
  INDEX `idx_role_code` (`role_code`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '后台管理员账号';


-- -------------------------------------------------------------------
-- 2. admin_roles
-- -------------------------------------------------------------------
DROP TABLE IF EXISTS `admin_roles`;
CREATE TABLE `admin_roles` (
  `id`           BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
  `code`         VARCHAR(32)  NOT NULL COMMENT '角色 code（业务唯一）',
  `name`         VARCHAR(64)  NOT NULL COMMENT '展示名',
  `description`  VARCHAR(255) NOT NULL DEFAULT '' COMMENT '职责描述',
  `is_built_in`  TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '1 预设角色（不可删） / 0 自定义',
  `sort`         INT          NOT NULL DEFAULT 0 COMMENT '排序',
  `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `uk_code` (`code`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '后台角色定义';


-- -------------------------------------------------------------------
-- 3. admin_role_permissions
--   粗矩阵：role × module × action × scope
--   action: read / write / approve / execute  (Phase 2 再扩 4-eye)
--   scope:  all / self / none                 (Phase 1 默认 all)
-- -------------------------------------------------------------------
DROP TABLE IF EXISTS `admin_role_permissions`;
CREATE TABLE `admin_role_permissions` (
  `id`         BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
  `role_code`  VARCHAR(32) NOT NULL COMMENT '角色 code',
  `module`     VARCHAR(32) NOT NULL COMMENT '模块 code: m1_identity/m2_project/m5_task/m7_user/m8_i18n/news',
  `action`     VARCHAR(16) NOT NULL COMMENT 'read/write/approve/execute',
  `scope`      VARCHAR(16) NOT NULL DEFAULT 'all' COMMENT 'all/self/none',
  `created_at` DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `uk_role_module_action` (`role_code`, `module`, `action`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '角色权限粗矩阵';


-- -------------------------------------------------------------------
-- 4. 初始化数据
-- -------------------------------------------------------------------

-- 4.1 六个预设角色（与 spec §2.1 对齐）
INSERT INTO `admin_roles` (`code`, `name`, `description`, `is_built_in`, `sort`) VALUES
  ('super_admin', '超级管理员',  '系统配置、角色管理、紧急 pause；最高权限',           1, 10),
  ('operator',    '运营',        '日常项目运营、通知 / 文案推送、链上动作发起',         1, 20),
  ('approver',    '审批员',      '高风险动作 4-eye 复核；仅审批不能发起',              1, 30),
  ('compliance',  '合规',        'KYC / 制裁名单 / 用户档案审核',                       1, 40),
  ('support',     '客服',        '用户工单、积分调整（限额）、定向通知',                1, 50),
  ('auditor',     '审计',        '全模块只读 + 日志导出',                                1, 60);

-- 4.2 Phase 1 角色权限种子（仅覆盖本期 5 个模块 + news；Phase 2 起接 4-eye 后再补 approve/execute）
-- super_admin：全模块全动作
INSERT INTO `admin_role_permissions` (`role_code`, `module`, `action`, `scope`) VALUES
  ('super_admin','m1_identity','read','all'),  ('super_admin','m1_identity','write','all'),
  ('super_admin','m2_project','read','all'),   ('super_admin','m2_project','write','all'),
  ('super_admin','m5_task','read','all'),      ('super_admin','m5_task','write','all'),
  ('super_admin','m7_user','read','all'),      ('super_admin','m7_user','write','all'),
  ('super_admin','m8_i18n','read','all'),      ('super_admin','m8_i18n','write','all'),
  ('super_admin','news','read','all'),         ('super_admin','news','write','all');

-- operator：除身份管理外都可写
INSERT INTO `admin_role_permissions` (`role_code`, `module`, `action`, `scope`) VALUES
  ('operator','m1_identity','read','self'),
  ('operator','m2_project','read','all'),  ('operator','m2_project','write','all'),
  ('operator','m5_task','read','all'),     ('operator','m5_task','write','all'),
  ('operator','m7_user','read','all'),
  ('operator','m8_i18n','read','all'),     ('operator','m8_i18n','write','all'),
  ('operator','news','read','all'),        ('operator','news','write','all');

-- auditor：全模块只读
INSERT INTO `admin_role_permissions` (`role_code`, `module`, `action`, `scope`) VALUES
  ('auditor','m1_identity','read','all'),
  ('auditor','m2_project','read','all'),
  ('auditor','m5_task','read','all'),
  ('auditor','m7_user','read','all'),
  ('auditor','m8_i18n','read','all'),
  ('auditor','news','read','all');

-- approver / compliance / support：Phase 1 暂只给只读（4-eye / KYC / 工单等高级动作 Phase 2+ 再细化）
INSERT INTO `admin_role_permissions` (`role_code`, `module`, `action`, `scope`) VALUES
  ('approver','m2_project','read','all'), ('approver','m5_task','read','all'),
  ('approver','m7_user','read','all'),    ('approver','m8_i18n','read','all'),
  ('approver','news','read','all'),
  ('compliance','m7_user','read','all'),  ('compliance','m2_project','read','all'),
  ('support','m7_user','read','all'),     ('support','m5_task','read','all');


-- 4.3 公开快照不预置管理员；仅为本地隔离测试另行配置账号。


SET FOREIGN_KEY_CHECKS = 1;
