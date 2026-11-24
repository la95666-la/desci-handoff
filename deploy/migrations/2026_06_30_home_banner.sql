-- ============================================================
-- home_banner: 首页 Banner（含 3D 动效选择）
--   每条 banner 携带语种 + 排期 + 当前激活的 3D 动效类型
--   C 端取 (language + is_enable=1 + 排期内) 中 sort 最高的一条作为当前 hero
-- ============================================================


CREATE TABLE IF NOT EXISTS `home_banner` (
  `id`             BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `title`          VARCHAR(200)  NOT NULL DEFAULT '' COMMENT '主标题',
  `subtitle`       VARCHAR(500)  NOT NULL DEFAULT '' COMMENT '副标题',
  `cta_label`      VARCHAR(64)   NOT NULL DEFAULT '' COMMENT 'CTA 按钮文字',
  `cta_link`       VARCHAR(500)  NOT NULL DEFAULT '' COMMENT 'CTA 按钮跳转链接',
  `image_url`      VARCHAR(500)  NOT NULL DEFAULT '' COMMENT '背景图（可选，留空则纯 3D 背景）',
  `hero_animation` VARCHAR(32)   NOT NULL DEFAULT 'dandelion' COMMENT '3D 动效类型：dandelion / dna / none',
  `language`       VARCHAR(8)    NOT NULL DEFAULT 'en' COMMENT 'en / tw / zh',
  `start_at`       DATETIME      NULL DEFAULT NULL COMMENT '生效起（NULL = 立即）',
  `end_at`         DATETIME      NULL DEFAULT NULL COMMENT '生效止（NULL = 长期）',
  `sort`           INT(11)       NOT NULL DEFAULT 0 COMMENT '同语种内排序，越大越靠前',
  `is_enable`      TINYINT(1)    NOT NULL DEFAULT 1 COMMENT '1 启用 / 0 停用',
  `created_at`     DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`     DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_lang_enable_sort` (`language`, `is_enable`, `sort`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '首页 Banner';

-- ----------------------------
-- 默认种子：每语种各一条 dandelion banner
-- ----------------------------
INSERT IGNORE INTO `home_banner` (`title`, `subtitle`, `cta_label`, `cta_link`, `hero_animation`, `language`, `sort`, `is_enable`) VALUES
  ('Own the Future of Medicine', 'A biomedical innovation protocol owned and driven by a global community.', 'Explore Launchpad', '/launchpad', 'dandelion', 'en', 100, 1),
  ('擁有醫療的未來',                 '由全球社群共同擁有並驅動的生物醫學創新協議。',                                            '探索 Launchpad',     '/launchpad', 'dandelion', 'tw', 100, 1),
  ('拥有医疗的未来',                 '由全球社区共同拥有并驱动的生物医学创新协议。',                                            '探索 Launchpad',     '/launchpad', 'dandelion', 'zh', 100, 1);
