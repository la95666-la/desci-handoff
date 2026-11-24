-- ============================================================
-- P6: TG 每日活跃统计
--   1 条消息即"活跃 1 次"；同一 (chat, tg_user, day) 只算 1 个活跃用户
--   INSERT ... ON DUPLICATE KEY UPDATE 累加 message_count
-- ============================================================


CREATE TABLE IF NOT EXISTS `tg_daily_activity` (
  `id`            BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
  `chat_id`       BIGINT(20)   NOT NULL COMMENT 'TG chat_id（supergroup 是 -100xxx）',
  `chat_title`    VARCHAR(200) NOT NULL DEFAULT '' COMMENT '群名（冗余，方便 admin 直接展示）',
  `tg_user_id`    BIGINT(20)   NOT NULL,
  `tg_username`   VARCHAR(100) NOT NULL DEFAULT '',
  `uid`           BIGINT(20)   NULL DEFAULT NULL COMMENT 'joincare user.id（bind 过的才有）',
  `day`           DATE         NOT NULL,
  `message_count` INT(11)      NOT NULL DEFAULT 1 COMMENT '当日累计发言数（INSERT ON DUP UPDATE +1）',
  `first_msg_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `last_msg_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `uk_chat_user_day` (`chat_id`, `tg_user_id`, `day`) USING BTREE,
  INDEX `idx_day` (`day`) USING BTREE,
  INDEX `idx_uid_day` (`uid`, `day`) USING BTREE,
  INDEX `idx_chat_day` (`chat_id`, `day`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = 'TG 每日活跃统计（1 行 = 1 用户当天在某群活跃）';
