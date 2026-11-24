-- ============================================================
-- P4: 用户备注（admin 内部）+ 用户登录日志
-- ============================================================


CREATE TABLE IF NOT EXISTS `user_note` (
  `id`             BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
  `uid`            BIGINT(20)   NOT NULL COMMENT 'user.id',
  `admin_id`       BIGINT(20) UNSIGNED NOT NULL COMMENT '写备注的 admin',
  `admin_username` VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '冗余，方便列表直接显示',
  `note`           VARCHAR(1000) NOT NULL DEFAULT '' COMMENT '备注文本',
  `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_uid_created` (`uid`, `created_at`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '用户备注（admin 内部标注）';


CREATE TABLE IF NOT EXISTS `user_login_log` (
  `id`           BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
  `uid`          BIGINT(20)   NOT NULL,
  `ip`           VARCHAR(45)  NOT NULL DEFAULT '',
  `user_agent`   VARCHAR(500) NOT NULL DEFAULT '',
  `login_type`   VARCHAR(20)  NOT NULL DEFAULT 'wallet' COMMENT 'wallet / auto / register',
  `logged_in_at` DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_uid_time` (`uid`, `logged_in_at`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '用户登录日志';
