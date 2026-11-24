-- ============================================================
-- Sprint A: Launchpad 项目详情页 Phase 1（展示 & 订阅）数据层
--   - project 表扩 12 字段（category / cover / launch_at / *_html / *_json）
--   - 新增 4 个用户行为表：subscriber / watch / whitelist / update
--   - 新增 upload_file 表（通用文件上传元数据）
-- ============================================================
SET NAMES utf8mb4;

-- ---------------- project 扩字段（一次性 ALTER，非幂等）----------------
-- 生产上单次 apply；测试环境每次 down/up 都是全新 DB
-- （下面的 CREATE TABLE 都用 IF NOT EXISTS 保证子表幂等）
ALTER TABLE `projects`
  ADD COLUMN `category`         VARCHAR(40)  NOT NULL DEFAULT '' COMMENT '项目分类（antibody / neuro / compound / data / rare-disease 等）',
  ADD COLUMN `cover_url`        VARCHAR(500) NOT NULL DEFAULT '' COMMENT '首屏主视觉 cover 图 URL',
  ADD COLUMN `cover_video_url`  VARCHAR(500) NOT NULL DEFAULT '' COMMENT 'Pitch video URL（可选）',
  ADD COLUMN `launch_at`        DATETIME     NULL COMMENT '计划开售时间（Pre-launch 阶段倒计时用）',
  ADD COLUMN `story_html`       MEDIUMTEXT   COMMENT 'Story 板块富文本 HTML（TipTap sanitized）',
  ADD COLUMN `research_html`    MEDIUMTEXT   COMMENT 'Research & Data 板块富文本 HTML',
  ADD COLUMN `risks_html`       MEDIUMTEXT   COMMENT 'Risks & Disclosures 板块富文本 HTML',
  ADD COLUMN `team_json`        JSON         NULL COMMENT '结构化 team [{name, role, avatar, bio, links}, ...]',
  ADD COLUMN `tokenomics_json`  JSON         NULL COMMENT '结构化 tokenomics {allocations: [], vesting: {tge_pct, cliff, linear_months}}',
  ADD COLUMN `roadmap_json`     JSON         NULL COMMENT '结构化 roadmap [{quarter, title, desc, status: done|active|planned}, ...]',
  ADD COLUMN `documents_json`   JSON         NULL COMMENT '结构化 documents [{title, url, kind}, ...]',
  ADD COLUMN `faq_json`         JSON         NULL COMMENT '结构化 FAQ [{q, a}, ...]',
  ADD COLUMN `onchain_json`     JSON         NULL COMMENT '合约地址预告 [{label, address, chain}, ...]';

-- ---------------- project_update：项目方发帖时间线 ----------------
CREATE TABLE IF NOT EXISTS `project_updates` (
  `id`               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id`       INT             NOT NULL COMMENT '关联 project.project_id',
  `author_admin_id`  BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '发帖 admin id',
  `title`            VARCHAR(200)    NOT NULL DEFAULT '',
  `content_html`     MEDIUMTEXT      COMMENT 'TipTap 富文本 HTML（sanitized）',
  `is_published`     TINYINT(1)      NOT NULL DEFAULT 0 COMMENT '0 草稿 / 1 已发布',
  `published_at`     DATETIME        NULL COMMENT '发布时间（发布触发订阅推送）',
  `created_at`       DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`       DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  INDEX `idx_project_pub` (`project_id`, `is_published`, `published_at`)
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目方发帖 Updates 时间线';

-- ---------------- project_subscriber：Notify me 订阅（发 Update 时推送）----------------
CREATE TABLE IF NOT EXISTS `project_subscribers` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id` INT             NOT NULL,
  `uid`        INT             NOT NULL,
  `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_uid` (`project_id`, `uid`)
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目订阅（站内信推送目标）';

-- ---------------- project_watch：Watchlist ----------------
CREATE TABLE IF NOT EXISTS `project_watches` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id` INT             NOT NULL,
  `uid`        INT             NOT NULL,
  `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_uid` (`project_id`, `uid`)
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户关注（Watchlist）';

-- ---------------- project_whitelist：Warmup 阶段的白名单登记 ----------------
CREATE TABLE IF NOT EXISTS `project_whitelists` (
  `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id`     INT             NOT NULL,
  `uid`            INT             NOT NULL,
  `wallet_address` VARCHAR(42)     NOT NULL DEFAULT '',
  `note`           VARCHAR(200)    NOT NULL DEFAULT '' COMMENT '用户可选备注',
  `approved`       TINYINT(1)      NOT NULL DEFAULT 0 COMMENT '0 待审 / 1 通过（Phase 1 默认全通过，Phase 2 admin 可拒）',
  `created_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_uid` (`project_id`, `uid`)
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目白名单（Warmup 阶段登记）';

-- ---------------- upload_file：通用上传文件元数据（供 admin 上传 pdf / img 用）----------------
CREATE TABLE IF NOT EXISTS `upload_files` (
  `id`                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `uploader_admin_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `original_name`     VARCHAR(255)    NOT NULL DEFAULT '',
  `storage_path`      VARCHAR(500)    NOT NULL DEFAULT '' COMMENT 'LocalFs: /uploads/xxx.pdf; MinIO: bucket 内 key',
  `public_url`        VARCHAR(500)    NOT NULL DEFAULT '' COMMENT '前端可访问的完整 URL（含 base）',
  `mime`              VARCHAR(100)    NOT NULL DEFAULT '',
  `size_bytes`        BIGINT          NOT NULL DEFAULT 0,
  `created_at`        DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  INDEX `idx_admin` (`uploader_admin_id`)
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文件上传元数据（LocalFs Phase 1，MinIO Phase 2 切换）';
