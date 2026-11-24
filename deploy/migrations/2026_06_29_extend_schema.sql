-- ===================================================================
-- JoinCare schema 扩展 (2026-06-29)
--   1) user: 新增 Telegram 字段
--   2) task: 扩展支持 Telegram；改造为单步多平台结构
--   3) invite: 新增 N 级邀请链路
--   4) 新增 news 表（多语言新闻）
-- ===================================================================
SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;


-- -------------------------------------------------------------------
-- 1. user 表：新增 Telegram 字段
-- -------------------------------------------------------------------
ALTER TABLE `user`
  -- telegram_id NULLable：MySQL UNIQUE 索引允许多条 NULL，避免未绑定 TG 的用户撞键
  ADD COLUMN `telegram_id`           VARCHAR(50)  NULL DEFAULT NULL COMMENT 'Telegram 用户 ID (numeric)' AFTER `twitter_avatar`,
  ADD COLUMN `telegram_username`     VARCHAR(100) NOT NULL DEFAULT '' COMMENT 'Telegram 用户名 (@xxx)'     AFTER `telegram_id`,
  ADD COLUMN `telegram_first_name`   VARCHAR(100) NOT NULL DEFAULT '' COMMENT 'Telegram first name'       AFTER `telegram_username`,
  ADD COLUMN `telegram_last_name`    VARCHAR(100) NOT NULL DEFAULT '' COMMENT 'Telegram last name'        AFTER `telegram_first_name`,
  ADD COLUMN `telegram_avatar`       VARCHAR(500) NOT NULL DEFAULT '' COMMENT 'Telegram 头像 URL'         AFTER `telegram_last_name`,
  ADD COLUMN `last_login_ip`         VARCHAR(45)  NOT NULL DEFAULT '' COMMENT '上次登录 IP（支持 IPv6）'    AFTER `last_login_at`,
  ADD UNIQUE INDEX `uk_telegram_id` (`telegram_id`);


-- -------------------------------------------------------------------
-- 2. task：扩展为通用社交任务（支持 X / Telegram）
--    旧表是「一行三步」(follow_url + like_url + retweet_url)，
--    新表是「一行一步」+ platform 字段，更通用
--
--    旧 task 表先备份为 task_legacy_v1，再 DROP 重建
-- -------------------------------------------------------------------
RENAME TABLE `task` TO `task_legacy_v1`;

CREATE TABLE `task`  (
  `id`          INT(11) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '任务id',
  `platform`    VARCHAR(20)  CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'twitter' COMMENT '平台：twitter / telegram',
  `task_key`    VARCHAR(50)  CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '任务键：follow / like / bookmark / reply / join / chat …',
  `title`       VARCHAR(200) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '默认显示标题（多语见 copywriting_config）',
  `description` VARCHAR(500) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '默认显示描述',
  `url`         VARCHAR(500) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '目标 URL（推文 / 群组链接 / 帖子）',
  `cta_label`   VARCHAR(50)  CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '按钮文案（Open X / Open Telegram …）',
  `verify_kind` VARCHAR(50)  CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'manual' COMMENT '校验方式：manual / twitter_api / telegram_bot',
  `verify_meta` TEXT         CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NULL COMMENT '校验参数 JSON (tweet_id / chat_id / bot_token_ref 等)',
  `amount_point` INT(11) NOT NULL DEFAULT 0 COMMENT '奖励积分',
  `sort`        INT(11) NOT NULL DEFAULT 0 COMMENT '排序，越小越靠前',
  `is_enable`   TINYINT(3) NOT NULL DEFAULT 1 COMMENT '是否可用 0/1',
  `created_at`  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `uk_platform_key` (`platform`, `task_key`) USING BTREE,
  INDEX `idx_platform_enable` (`platform`, `is_enable`, `sort`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '社交任务表（X / Telegram 通用）' ROW_FORMAT = Dynamic;

-- 预置当前前端 TaskHall.vue 用到的 6 条任务
INSERT INTO `task` (`platform`, `task_key`, `title`, `description`, `url`, `cta_label`, `verify_kind`, `amount_point`, `sort`) VALUES
('twitter',  'follow',   'Follow @JoinCare on X',          'Stay updated with our latest research drops.',          'https://x.com/joincare',                                  'Open X',        'twitter_api', 25, 10),
('twitter',  'like',     'Like our flagship post',         'Boost a peer-reviewed research thread.',                'https://x.com/joincare/status/1800000000000000001',       'Open post',     'twitter_api', 25, 20),
('twitter',  'bookmark', 'Bookmark research drop',         'Save the on-chain IP launch announcement.',             'https://x.com/joincare/status/1800000000000000002',       'Open post',     'twitter_api', 25, 30),
('twitter',  'reply',    'Reply with your research wish',  'Tell us which area JoinCare should fund next.',         'https://x.com/joincare/status/1800000000000000003',       'Open thread',   'twitter_api', 25, 40),
('telegram', 'join',     'Join JoinCare Telegram',         'The official builder + researcher channel.',            'https://t.me/joincare',                                   'Open Telegram', 'telegram_bot', 50, 10),
('telegram', 'chat',     'Drop a hello in the community',  'Introduce yourself — share your interest area.',        'https://t.me/joincare_chat',                              'Open chat',     'telegram_bot', 50, 20);

-- 旧 task_record 字段对齐：增加 platform 列，重建唯一索引
ALTER TABLE `task_record`
  ADD COLUMN `platform` VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'twitter' COMMENT '平台' AFTER `task_id`,
  DROP INDEX `uk_task_uid`,
  ADD UNIQUE INDEX `uk_platform_task_uid` (`platform`, `task_id`, `uid`) USING BTREE;


-- -------------------------------------------------------------------
-- 3. 邀请关系：N 级链路（闭包表）
--    invite_record 保留「直接邀请」(L1) 行为；新增 invite_lineage
--    存储完整祖先链路：descendant 用户 → 多个 ancestor + level
--    例：A 邀 B、B 邀 C → 给 C 写两行：
--      (C, B, level=1) + (C, A, level=2)
-- -------------------------------------------------------------------
CREATE TABLE `invite_lineage`  (
  `id`             BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `descendant_uid` BIGINT(20) UNSIGNED NOT NULL COMMENT '被邀请用户ID',
  `ancestor_uid`   BIGINT(20) UNSIGNED NOT NULL COMMENT '祖先邀请人ID',
  `level`          INT(11) NOT NULL COMMENT '层级：1=直接邀请, 2=二级, …',
  `reward_point`   INT(11) NOT NULL DEFAULT 0 COMMENT '本层级应得积分',
  `is_settled`     TINYINT(3) NOT NULL DEFAULT 0 COMMENT '是否已结算（0未/1已）',
  `created_at`     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `uk_descendant_ancestor` (`descendant_uid`, `ancestor_uid`) USING BTREE,
  INDEX `idx_ancestor_level` (`ancestor_uid`, `level`) USING BTREE,
  INDEX `idx_descendant_level` (`descendant_uid`, `level`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '多级邀请关系闭包表' ROW_FORMAT = Dynamic;


-- -------------------------------------------------------------------
-- 4. 新闻表 + 多语言翻译
--    主表 news 存放：封面图、外链、状态、排序、发布时间等元数据
--    子表 news_translation 存每种语言的 title / summary / author 等
--    与现有 copywriting_config (key + language) 风格一致
-- -------------------------------------------------------------------
CREATE TABLE `news`  (
  `id`            BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `slug`          VARCHAR(200) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'URL slug，唯一标识',
  `category`      VARCHAR(50)  CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '分类：press / announcement / blog …',
  `cover_url`     VARCHAR(500) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '封面图 URL',
  `source_url`    VARCHAR(500) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部原文链接（press 必填）',
  `source_name`   VARCHAR(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '原始媒体名称：Barchart / EU Reporter …',
  `is_featured`   TINYINT(3) NOT NULL DEFAULT 0 COMMENT '是否在首页 Latest Media 推荐位 0/1',
  `sort`          INT(11) NOT NULL DEFAULT 0 COMMENT '排序，越小越靠前',
  `status`        TINYINT(4) NOT NULL DEFAULT 0 COMMENT '状态 0=草稿 / 1=已发布 / 2=已下架',
  `publish_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '发布时间',
  `created_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `uk_slug` (`slug`) USING BTREE,
  INDEX `idx_status_featured_sort` (`status`, `is_featured`, `sort`) USING BTREE,
  INDEX `idx_publish_at` (`publish_at`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '新闻 / 媒体报道主表' ROW_FORMAT = Dynamic;

CREATE TABLE `news_translation`  (
  `id`         BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `news_id`    BIGINT(20) UNSIGNED NOT NULL COMMENT '新闻 ID',
  `language`   VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '语言：en / tw / zh-cn',
  `title`      VARCHAR(300) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '标题',
  `summary`    VARCHAR(800) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '摘要',
  `content`    LONGTEXT     CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NULL COMMENT '正文 (Markdown / HTML)',
  `author`     VARCHAR(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '署名作者',
  `author_role` VARCHAR(150) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '署名职位描述',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `uk_news_lang` (`news_id`, `language`) USING BTREE,
  INDEX `idx_news_id` (`news_id`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '新闻多语言翻译表' ROW_FORMAT = Dynamic;

-- 预置首页 Latest Media 三条
INSERT INTO `news` (`slug`, `category`, `cover_url`, `source_url`, `source_name`, `is_featured`, `sort`, `status`, `publish_at`) VALUES
('joincare-token2049-singapore',   'press', '/figma/v2/news/joincare-token2049.jpg', 'https://www.barchart.com/story/news/35330430/joincare-debuts-at-token2049-singapore-empowering-biomedical-innovation-with-web3-and-reshaping-the-research-financing-landscape', 'Barchart',    1, 10, 1, '2026-06-29 00:00:00'),
('innoblock-2024-global-web3',     'press', '/figma/v2/news/innoblock-2024.webp',    'https://www.coingabbar.com/en/crypto-press-release/innoblock-2024-elevating-web3-innovation-globally',                                                                                'CoinGabbar',  1, 20, 1, '2026-06-29 00:00:00'),
('p2x1-membrane-protein',          'press', '/figma/v2/news/jiuge-p2x1.jpg',         'https://www.eureporter.co/health/2026/01/28/hangzhou-jiuge-biotech-advances-membrane-protein-production-with-p2x1-antibody-p007/',                                                  'EU Reporter', 1, 30, 1, '2026-01-28 00:00:00');

INSERT INTO `news_translation` (`news_id`, `language`, `title`, `summary`, `author`, `author_role`) VALUES
(1, 'en',    'JoinCare Debuts at TOKEN2049 Singapore — Empowering Biomedical Innovation with Web3', '',                 'Barchart',   'TOKEN2049 Coverage'),
(1, 'tw', 'JoinCare 亮相 TOKEN2049 新加坡 — 以 Web3 賦能生物醫學創新',                                  '',                 'Barchart',   'TOKEN2049 報導'),
(2, 'en',    'InnoBlock 2024: Elevating Web3 Innovation Globally',                                      '',                 'CoinGabbar', 'Web3 Innovation'),
(2, 'tw', 'InnoBlock 2024：將 Web3 創新推向全球',                                                     '',                 'CoinGabbar', 'Web3 創新'),
(3, 'en',    'Advancing Membrane Protein Production with P2X1 Antibody',                                '',                 'EU Reporter','Health · Biotech'),
(3, 'tw', '以 P2X1 抗體推進膜蛋白生產',                                                              '',                 'EU Reporter','健康・生技');


SET FOREIGN_KEY_CHECKS = 1;
