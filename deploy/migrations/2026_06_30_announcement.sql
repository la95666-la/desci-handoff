-- ============================================================
-- announcement: 系统公告（弹窗/横幅）
--   type='popup'   弹窗（首次进入弹一次，localStorage 'jc_announcement_dismissed' 记 id）
--   type='banner'  顶部横幅（持续显示直到 end_at 或 is_enable=0）
--   priority       同 type 内取 priority 最大的一条作 "current"
-- ============================================================


CREATE TABLE IF NOT EXISTS `announcement` (
  `id`         BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
  `title`      VARCHAR(200)  NOT NULL DEFAULT '',
  `content`    TEXT          NOT NULL COMMENT '富文本（前端不直接 v-html，后端入库前 bluemonday 清洗）',
  `type`       VARCHAR(16)   NOT NULL DEFAULT 'popup' COMMENT 'popup / banner',
  `language`   VARCHAR(8)    NOT NULL DEFAULT 'en' COMMENT 'en / tw / zh',
  `priority`   INT(11)       NOT NULL DEFAULT 0 COMMENT '同 type+language 内取最大优先级',
  `start_at`   DATETIME      NULL DEFAULT NULL,
  `end_at`     DATETIME      NULL DEFAULT NULL,
  `is_enable`  TINYINT(1)    NOT NULL DEFAULT 1,
  `created_at` DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_type_lang_enable_prio` (`type`, `language`, `is_enable`, `priority`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '系统公告';
