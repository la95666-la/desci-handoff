-- joincare Fair Launch pinned production-like baseline v1
-- source: joincare.sql + ordered migrations through 2026_09_02
SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ----------------------------
-- Table structure for apply_project
-- ----------------------------
DROP TABLE IF EXISTS `apply_project`;
CREATE TABLE `apply_project`  (
  `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `name` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '姓名',
  `city` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '所在城市/国家',
  `email` varchar(200) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '邮箱',
  `telegram` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'Telegram',
  `wechat` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '微信',
  `major` varchar(200) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '专业/研究方向',
  `identity` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '当前身份（医生/科研人员/医学生/创业者/独立开发者/其他）',
  `organization` varchar(200) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '所属机构/学校/公司（选填）',
  `project_name` varchar(200) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '项目名称',
  `project_desc` varchar(500) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '用一句话介绍你的项目',
  `project_category` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '项目所属方向（生物医药/长寿抗衰/抗肿瘤/传染病防治/AI+医疗健康/医疗设备/其他）',
  `current_step` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '目前做到哪一步了（只有想法/已有方案或研究设计/已有实验数据/已有原型或Demo/正在推进落地/其他）',
  `possessions` varchar(200) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '目前已经具备哪些内容（实验数据/专利或申请/论文成果/产品原型/合作资源/社区基础/暂时还没有）',
  `apply_reason` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '为什么想申请 JoinCare IDO',
  `support_wanted` varchar(200) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '希望获得什么支持（资金支持/项目曝光/AI工具支持/Web3 发行支持/顾问或资源对接/社区建设/品牌包装）',
  `project_link` varchar(500) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '项目介绍/PDF链接',
  `additional_info` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '其他想补充的内容',
  `status` tinyint(4) NOT NULL DEFAULT 0 COMMENT '状态（0待审核/1已通过/2已拒绝）',
  `wallet_address` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '钱包地址',
  `record_id` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '多维表格对应ID',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `idx_id`(`record_id`) USING BTREE,
  INDEX `idx_created_at`(`created_at`) USING BTREE,
  INDEX `idx_address`(`wallet_address`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 14 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = 'IDO 申请表' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for chain_event_record
-- ----------------------------
DROP TABLE IF EXISTS `chain_event_record`;
CREATE TABLE `chain_event_record`  (
  `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '记录唯一ID',
  `event` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '事件类型',
  `block_number` bigint(20) UNSIGNED NOT NULL DEFAULT 0 COMMENT '链上数字',
  `log_index` bigint(20) NOT NULL,
  `tx_hash` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '事件hash值',
  `data` text CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '事件参数',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `idx_num`(`block_number`, `tx_hash`, `log_index`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 202 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci COMMENT = '链上事件记录' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for copywriting_config
-- ----------------------------
DROP TABLE IF EXISTS `copywriting_config`;
CREATE TABLE `copywriting_config`  (
  `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '配置唯一ID',
  `key` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '',
  `value` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '文案内容',
  `language` varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT 'zh-cn' COMMENT '语言代码',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 57 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci COMMENT = '文案配置表' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for invite_record
-- ----------------------------
DROP TABLE IF EXISTS `invite_record`;
CREATE TABLE `invite_record`  (
  `id` int(11) NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `uid` int(11) NOT NULL DEFAULT 0 COMMENT '用户ID',
  `inviter_uid` int(11) NOT NULL DEFAULT 0 COMMENT '邀请人用户ID',
  `receive_amount` tinyint(3) NOT NULL DEFAULT 0 COMMENT '是否领取奖励',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_uid`(`uid`) USING BTREE,
  INDEX `idx_inviter_uid`(`inviter_uid`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 3 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '邀请记录表' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for invite_user_code
-- ----------------------------
DROP TABLE IF EXISTS `invite_user_code`;
CREATE TABLE `invite_user_code`  (
  `id` int(11) NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `uid` int(11) NOT NULL COMMENT '用户ID',
  `code` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '邀请码',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `idx_code`(`code`) USING BTREE,
  INDEX `idx_uid`(`uid`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 16 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '用户邀请码表' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for notification_read_time
-- ----------------------------
DROP TABLE IF EXISTS `notification_read_time`;
CREATE TABLE `notification_read_time`  (
  `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '记录唯一ID',
  `uid` bigint(20) UNSIGNED NOT NULL COMMENT '用户ID',
  `last_read_at` timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '最后已读时间',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `uk_user_id`(`uid`) USING BTREE,
  INDEX `idx_last_read_at`(`last_read_at`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 15 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci COMMENT = '用户通知已读时间表' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for notifications
-- ----------------------------
DROP TABLE IF EXISTS `notifications`;
CREATE TABLE `notifications`  (
  `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '通知ID',
  `uid` bigint(20) UNSIGNED NOT NULL COMMENT '接收用户ID',
  `type` varchar(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '通知类型',
  `title` varchar(200) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '通知标题',
  `content` varchar(512) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '通知内容',
  `extra` text CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '内容额外变量',
  `related_id` bigint(20) UNSIGNED NULL DEFAULT NULL COMMENT '关联对象ID(任务/提现记录等)',
  `created_at` timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间',
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_type`(`type`) USING BTREE,
  INDEX `idx_created_at`(`uid`, `created_at`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 288 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci COMMENT = '通知表' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for project_files
-- ----------------------------
DROP TABLE IF EXISTS `project_files`;
CREATE TABLE `project_files`  (
  `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `project_id` bigint(20) UNSIGNED NOT NULL COMMENT '关联的项目ID',
  `type` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '文件类型',
  `path` varchar(500) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '文件存储路径',
  `size` int(10) UNSIGNED NOT NULL DEFAULT 0 COMMENT '文件大小(字节)',
  `mime_type` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'MIME类型',
  `width` int(10) UNSIGNED NOT NULL DEFAULT 0 COMMENT '图片宽度(像素)',
  `height` int(10) UNSIGNED NOT NULL DEFAULT 0 COMMENT '图片高度(像素)',
  `uploaded_by` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '上传者标识',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '上传时间',
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_project_id`(`project_id`) USING BTREE,
  INDEX `idx_file_type`(`type`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 71 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '项目文件表' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for project_tge
-- ----------------------------
DROP TABLE IF EXISTS `project_tge`;
CREATE TABLE `project_tge`  (
  `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `project_id` bigint(20) UNSIGNED NOT NULL COMMENT '链上项目ID(合约中的projectId)',
  `index` int(11) NOT NULL,
  `tge_percent` decimal(5, 2) NOT NULL DEFAULT 0.00 COMMENT 'TGE累计释放比例(%)',
  `vesting_percent` decimal(5, 2) NOT NULL COMMENT '本次释放的比例',
  `vesting_interval` int(11) UNSIGNED NOT NULL COMMENT '间隔时间',
  `vesting_time` datetime NOT NULL COMMENT '释放时间',
  `vesting_duration` int(11) NOT NULL COMMENT '释放次数',
  `is_notification` tinyint(3) NOT NULL DEFAULT 0 COMMENT '是否发送了通知',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `uk_id_index`(`project_id`, `index`) USING BTREE,
  INDEX `idx_time`(`vesting_time`, `is_notification`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 109 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '项目释放时间表' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for projects
-- ----------------------------
DROP TABLE IF EXISTS `projects`;
CREATE TABLE `projects`  (
  `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `project_id` bigint(20) UNSIGNED NOT NULL COMMENT '链上项目ID(合约中的projectId)',
  `owner_address` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '项目主钱包地址',
  `name_en` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '项目英文名称',
  `name_cn` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '项目中文名称',
  `slug` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'URL友好的项目标识',
  `tagline` varchar(500) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '一句话简介',
  `description` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '项目详细介绍(支持Markdown/富文本)',
  `tags` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '项目标签/赛道(JSON数组)',
  `team_info` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '团队介绍',
  `roadmap` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '路线图',
  `partners` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '合作伙伴信息',
  `token_address` varchar(42) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '项目代币合约地址',
  `token_symbol` varchar(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '代币符号',
  `token_decimals` tinyint(3) UNSIGNED NOT NULL DEFAULT 18 COMMENT '代币精度',
  `payment_token` varchar(42) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'JCC' COMMENT '认购币种(默认JCC)',
  `token_price` decimal(36, 18) NOT NULL DEFAULT 0.000000000000000000 COMMENT '代币价格(单位:JCC)',
  `hard_cap` decimal(36, 18) NOT NULL DEFAULT 0.000000000000000000 COMMENT '硬顶(募资上限)',
  `soft_cap` decimal(36, 18) NOT NULL DEFAULT 0.000000000000000000 COMMENT '软顶(募资下限)',
  `max_per_user` decimal(36, 18) NOT NULL DEFAULT 0.000000000000000000 COMMENT '单用户认购上限',
  `start_time` datetime NOT NULL DEFAULT '1970-01-01 00:00:00' COMMENT '认购开始时间',
  `end_time` datetime NOT NULL DEFAULT '1970-01-01 00:00:00' COMMENT '认购结束时间',
  `tge_time` datetime NOT NULL DEFAULT '1970-01-01 00:00:00' COMMENT 'TGE时间(代币生成事件)',
  `total_deposited` decimal(36, 18) NOT NULL DEFAULT 0.000000000000000000 COMMENT '项目方充值金额',
  `total_raised` decimal(36, 18) NOT NULL DEFAULT 0.000000000000000000 COMMENT '已募集金额',
  `participant_count` int(10) UNSIGNED NOT NULL DEFAULT 0 COMMENT '参与人数',
  `token_withdrawn` tinyint(3) NOT NULL DEFAULT 0 COMMENT '充值代币是否已提取',
  `fund_withdrawn` tinyint(3) NOT NULL DEFAULT 0 COMMENT '募集资金是否已提取',
  `status` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '项目管理状态',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `uk_project_id`(`project_id`) USING BTREE,
  INDEX `idx_start_time`(`start_time`) USING BTREE,
  INDEX `idx_end_time`(`end_time`) USING BTREE,
  INDEX `idx_token_address`(`token_address`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 49 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '项目主表' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for settings
-- ----------------------------
DROP TABLE IF EXISTS `settings`;
CREATE TABLE `settings`  (
  `key` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '',
  `value` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '',
  `type` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT ''
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for task
-- ----------------------------
DROP TABLE IF EXISTS `task`;
CREATE TABLE `task`  (
  `id` int(11) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '任务id',
  `follow_url` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '关注url',
  `like_url` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '点赞url',
  `retweet_url` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '转发url',
  `is_enable` tinyint(3) NOT NULL DEFAULT 0 COMMENT '是否可用',
  `amount_point` int(11) NOT NULL DEFAULT 0 COMMENT '奖励积分',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 9 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '任务表' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for task_record
-- ----------------------------
DROP TABLE IF EXISTS `task_record`;
CREATE TABLE `task_record`  (
  `id` int(11) UNSIGNED NOT NULL AUTO_INCREMENT,
  `task_id` int(11) NOT NULL DEFAULT 0 COMMENT '任务ID',
  `uid` int(11) NOT NULL DEFAULT 0 COMMENT '用户ID',
  `amount_point` int(11) NOT NULL COMMENT '奖励积分',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `uk_task_uid`(`task_id`, `uid`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 14 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '任务完成记录表' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for transaction_record
-- ----------------------------
DROP TABLE IF EXISTS `transaction_record`;
CREATE TABLE `transaction_record`  (
  `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `project_id` bigint(20) UNSIGNED NOT NULL COMMENT '链上项目ID(合约中的projectId)',
  `type` varchar(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '交易类型',
  `tx_hash` varchar(66) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '交易hash',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 1 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '交易记录表' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for user
-- ----------------------------
DROP TABLE IF EXISTS `user`;
CREATE TABLE `user`  (
  `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '用户id',
  `twitter_id` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'Twitter用户ID',
  `twitter_handle` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'Twitter用户名(@xxx)',
  `twitter_display_name` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'Twitter昵称',
  `twitter_avatar` varchar(500) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'Twitter头像URL',
  `wallet_address` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '钱包地址',
  `type` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '用户类型',
  `last_login_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '上次登录时间',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `uk_address`(`wallet_address`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 18 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '用户表' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for user_check_in_record
-- ----------------------------
DROP TABLE IF EXISTS `user_check_in_record`;
CREATE TABLE `user_check_in_record`  (
  `id` int(11) NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `uid` int(11) NOT NULL DEFAULT 0 COMMENT '用户ID',
  `wallet_address` varchar(42) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '钱包地址',
  `check_in_date` varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '签到日期（格式：YYYY-MM-DD）',
  `check_in_day` bigint(20) NOT NULL DEFAULT 0 COMMENT '签到的第几天',
  `check_in_at` datetime NOT NULL COMMENT '签到时间',
  `point` int(11) NOT NULL DEFAULT 0 COMMENT '获得积分',
  `total_check_in_days` int(11) NOT NULL DEFAULT 0 COMMENT '累计签到天数',
  `continuous_check_in_days` int(11) NOT NULL DEFAULT 0 COMMENT '连续签到天数',
  `nonce` varchar(66) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '随机数',
  `deadline` bigint(20) NOT NULL DEFAULT 0 COMMENT '截止时间（时间戳）',
  `signature` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '签名',
  `tx_hash` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '交易哈希',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `idx_uid_date`(`uid`, `check_in_date`) USING BTREE,
  INDEX `idx_wallet_address`(`wallet_address`) USING BTREE,
  INDEX `idx_check_in_date`(`check_in_date`) USING BTREE,
  INDEX `idx_check_in_day`(`check_in_day`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 16 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '用户签到记录表' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for user_claim_record
-- ----------------------------
DROP TABLE IF EXISTS `user_claim_record`;
CREATE TABLE `user_claim_record`  (
  `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `uid` bigint(20) UNSIGNED NOT NULL COMMENT '用户ID',
  `project_id` bigint(20) UNSIGNED NOT NULL COMMENT '链上项目ID(合约中的projectId)',
  `claim_token` decimal(36, 18) NOT NULL COMMENT '领取代币数量',
  `block_number` int(11) NOT NULL COMMENT '链上num',
  `tx_hash` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '交易hash',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 11 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '用户领取代币表' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for user_point
-- ----------------------------
DROP TABLE IF EXISTS `user_point`;
CREATE TABLE `user_point`  (
  `id` int(11) NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `uid` int(11) NOT NULL COMMENT '用户ID',
  `point` int(11) NOT NULL DEFAULT 0 COMMENT '积分数量',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_uid`(`uid`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 22 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '用户积分表' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for user_point_flow
-- ----------------------------
DROP TABLE IF EXISTS `user_point_flow`;
CREATE TABLE `user_point_flow`  (
  `id` int(11) NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `uid` int(11) NOT NULL COMMENT '用户ID',
  `change_point` int(11) NOT NULL DEFAULT 0 COMMENT '变更积分数（正负表示增减）',
  `remain` int(11) NOT NULL DEFAULT 0 COMMENT '剩余积分数',
  `type` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '流水类型',
  `desc` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '描述信息',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_uid`(`uid`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 19 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '用户积分流水表' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for user_project
-- ----------------------------
DROP TABLE IF EXISTS `user_project`;
CREATE TABLE `user_project`  (
  `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `uid` bigint(20) UNSIGNED NOT NULL COMMENT '用户ID',
  `project_id` bigint(20) UNSIGNED NOT NULL COMMENT '链上项目ID(合约中的projectId)',
  `purchase_token` decimal(36, 18) NOT NULL COMMENT '认购代币数量',
  `next_claim_time` datetime NOT NULL COMMENT '下次领取时间',
  `claim_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '上次领取时间',
  `claim_token` decimal(36, 18) NOT NULL COMMENT '累计领取代币',
  `is_refund` int(11) NOT NULL DEFAULT 0 COMMENT '是否已退款',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `idx_uid_project`(`uid`, `project_id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 17 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '用户认购项目表' ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for user_refund_record
-- ----------------------------
DROP TABLE IF EXISTS `user_refund_record`;
CREATE TABLE `user_refund_record`  (
  `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `uid` bigint(20) UNSIGNED NOT NULL COMMENT '用户ID',
  `project_id` bigint(20) UNSIGNED NOT NULL COMMENT '链上项目ID(合约中的projectId)',
  `refund_token` decimal(36, 18) NOT NULL COMMENT '领取代币数量',
  `block_number` int(11) NOT NULL COMMENT '链上num',
  `tx_hash` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '交易hash',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 4 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '用户退款代币表' ROW_FORMAT = Dynamic;

SET FOREIGN_KEY_CHECKS = 1;

-- BEGIN 2026_06_29_extend_schema.sql
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

-- END 2026_06_29_extend_schema.sql

-- BEGIN 2026_06_30_admin_base.sql
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

-- END 2026_06_30_admin_base.sql

-- BEGIN 2026_06_30_announcement.sql
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

-- END 2026_06_30_announcement.sql

-- BEGIN 2026_06_30_checkin_rules.sql
-- ===================================================================
-- 签到阶梯奖励规则 (2026-06-30)
--   按 continuousCheckInDays 取「最高匹配档」(day_threshold <= streak ORDER BY day_threshold DESC LIMIT 1)
--   bonus_point = 当日实际得分（不叠加）
-- ===================================================================
SET NAMES utf8mb4;

DROP TABLE IF EXISTS `checkin_rule`;
CREATE TABLE `checkin_rule` (
  `id`            BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
  `day_threshold` INT(11) NOT NULL COMMENT '达到该连续天数时生效',
  `bonus_point`   INT(11) NOT NULL COMMENT '该档当日实际得分（含基础分，非叠加）',
  `is_enable`     TINYINT(3) NOT NULL DEFAULT 1 COMMENT '0 停用 / 1 启用',
  `sort`          INT(11) NOT NULL DEFAULT 0,
  `created_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE INDEX `uk_day_threshold` (`day_threshold`)
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='签到阶梯奖励';

-- 默认阶梯
INSERT INTO `checkin_rule` (`day_threshold`, `bonus_point`, `sort`) VALUES
  (1,  10,  10),
  (3,  15,  20),
  (7,  30,  30),
  (14, 50,  40),
  (30, 100, 50);

-- END 2026_06_30_checkin_rules.sql

-- BEGIN 2026_06_30_copywriting_bilingual.sql
-- ===================================================================
-- copywriting_config 双语聚合视图所需的 schema 调整 (2026-06-30)
--   1) value 列加宽到 TEXT（部分文案 > 255 字）
--   2) 加 UNIQUE INDEX (key, language) 防止 dup
-- ===================================================================
SET NAMES utf8mb4;

ALTER TABLE `copywriting_config`
  MODIFY COLUMN `value` TEXT NOT NULL COMMENT '文案内容（可长文）';

-- 加 UNIQUE INDEX，若已存在则忽略
SET @idx_exists := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE table_schema = DATABASE()
    AND table_name = 'copywriting_config'
    AND index_name = 'uk_key_lang');
SET @stmt := IF(@idx_exists = 0,
  'ALTER TABLE `copywriting_config` ADD UNIQUE INDEX `uk_key_lang` (`key`, `language`)',
  'SELECT "uk_key_lang already exists" AS note');
PREPARE s FROM @stmt; EXECUTE s; DEALLOCATE PREPARE s;

-- END 2026_06_30_copywriting_bilingual.sql

-- BEGIN 2026_06_30_home_banner.sql
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

-- END 2026_06_30_home_banner.sql

-- BEGIN 2026_06_30_task_record_verify.sql
-- ============================================================
-- task_record: 增加 verify_status 字段，记录 Twitter / TG 校验结果
--   pending       : 提交但尚未校验（异步任务，Phase 1 不使用）
--   manual_pass   : 非 twitter_api（无需校验，直接发分）
--   twitter_pass  : Twitter API 校验通过
--   twitter_fail  : Twitter API 返回 false（已退分，记录留底）
--   twitter_err   : Twitter API 调用异常（429 / 401 等；Phase 1 视为通过）
-- ============================================================

ALTER TABLE `task_record`
  ADD COLUMN `verify_status` VARCHAR(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci
    NOT NULL DEFAULT 'manual_pass' COMMENT '校验状态：pending/manual_pass/twitter_pass/twitter_fail/twitter_err'
    AFTER `amount_point`,
  ADD INDEX `idx_verify_status` (`verify_status`) USING BTREE;

-- END 2026_06_30_task_record_verify.sql

-- BEGIN 2026_06_30_user_is_enable.sql
-- user: 增加 is_enable（启停标志）
ALTER TABLE `user`
  ADD COLUMN `is_enable` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '1 启用 / 0 停用'
    AFTER `last_login_ip`,
  ADD INDEX `idx_is_enable` (`is_enable`) USING BTREE;

-- END 2026_06_30_user_is_enable.sql

-- BEGIN 2026_06_30_user_wallet.sql
-- ============================================================
-- user_wallet: 用户多钱包绑定（Phase 1 基础占位）
--   user.wallet_address 仍是注册时的"主钱包"，user_wallet 记录所有绑定钱包
--   chain: 'evm'（含 BSC/ETH）/ 'sol' / 'sui' …Phase 1 仅 evm
-- ============================================================


CREATE TABLE IF NOT EXISTS `user_wallet` (
  `id`             BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
  `uid`            INT(11)      NOT NULL COMMENT 'user.id',
  `wallet_address` VARCHAR(64)  NOT NULL,
  `chain`          VARCHAR(16)  NOT NULL DEFAULT 'evm' COMMENT 'evm / sol / sui ...',
  `is_primary`     TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '1 = 主钱包；同 uid 仅一条 = 1',
  `bound_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `uk_uid_wallet_chain` (`uid`, `wallet_address`, `chain`) USING BTREE,
  INDEX `idx_wallet` (`wallet_address`) USING BTREE,
  INDEX `idx_uid_primary` (`uid`, `is_primary`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT = '用户多钱包绑定';

-- END 2026_06_30_user_wallet.sql

-- BEGIN 2026_07_01_rbac_full.sql
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

-- END 2026_07_01_rbac_full.sql

-- BEGIN 2026_07_01_tg_daily_activity.sql
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

-- END 2026_07_01_tg_daily_activity.sql

-- BEGIN 2026_07_01_user_note_and_login_log.sql
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

-- END 2026_07_01_user_note_and_login_log.sql

-- BEGIN 2026_07_03_user_point_uk_uid.sql
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

-- END 2026_07_03_user_point_uk_uid.sql

-- BEGIN 2026_07_04_launchpad_content.sql
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

-- END 2026_07_04_launchpad_content.sql

-- BEGIN 2026_07_04_project_bilingual.sql
-- Sprint 07-04 · project 双语字段 (English 主 + 繁體中文 副)
-- tagline / story_html / research_html / risks_html 4 个字段做双语并列
-- JSON 字段 (team/tokenomics/roadmap/documents/faq/onchain) 内嵌 _zh 后缀属性，无需加 DB 列
--
-- 幂等：用 information_schema 检查列是否存在（MySQL 8 不支持 ADD COLUMN IF NOT EXISTS）
SET NAMES utf8mb4;

-- helper macro replaced with inline prepared statements
DELIMITER $$
DROP PROCEDURE IF EXISTS jc_add_col_if_missing $$
CREATE PROCEDURE jc_add_col_if_missing(
    IN col_name VARCHAR(64),
    IN col_def  VARCHAR(255)
)
BEGIN
  IF NOT EXISTS (
      SELECT 1 FROM information_schema.columns
      WHERE table_schema = DATABASE() AND table_name = 'projects' AND column_name = col_name
  ) THEN
      SET @sql := CONCAT('ALTER TABLE projects ADD COLUMN ', col_name, ' ', col_def);
      PREPARE stmt FROM @sql;
      EXECUTE stmt;
      DEALLOCATE PREPARE stmt;
  END IF;
END $$
DELIMITER ;

CALL jc_add_col_if_missing('tagline_zh',       "VARCHAR(500) NOT NULL DEFAULT '' AFTER tagline");
CALL jc_add_col_if_missing('story_html_zh',    "MEDIUMTEXT NULL AFTER story_html");
CALL jc_add_col_if_missing('research_html_zh', "MEDIUMTEXT NULL AFTER research_html");
CALL jc_add_col_if_missing('risks_html_zh',    "MEDIUMTEXT NULL AFTER risks_html");

DROP PROCEDURE jc_add_col_if_missing;

-- END 2026_07_04_project_bilingual.sql

-- BEGIN 2026_07_04_projects_zh.sql
-- 补齐 project 2 (P2X1) + 3 (Smarto Neuro) 的封面 + 繁体中文内容
-- 前端 pickField/pick 会根据 language 自动 pick _zh 字段，空则 fallback 到 english
SET NAMES utf8mb4;

-- Project 2 · 繁体中文内容 + cover
UPDATE projects SET
  cover_url        = '/figma/v2/project-card-image-3.png',
  tagline_zh       = '首個功能性 P2X1 離子通道單株抗體 — 一個平台，三大適應症（膿毒症、腫瘤、抗衰老）。',
  story_html_zh    = '<h2>背景 — 為何離子通道單抗是最難的一類生物藥</h2>
<p>G 蛋白偶聯受體 (GPCR) 與離子通道合計約占 FDA 批准藥物靶點的一半 (分別 34% 與 13%)，但至今只有兩款 GPCR 單株抗體 (<em>erenumab</em>、<em>mogamulizumab</em>) 進入臨床。抗體在選擇性、生體可用率、半衰期與效應功能上均優於小分子；但保持天然構象的抗原製備長期是產業瓶頸。</p>
<p>離子通道又比 GPCR 更難。GPCR 是七次跨膜的單體膜蛋白；離子通道則是<strong>多亞基、多次跨膜的功能複合體，且在開放與關閉之間切換不同構象</strong>。輝瑞生物藥部門公開研發 GPCR / 離子通道抗體藥，但進展緩慢。</p>

<h2>我們的方法 — Nanodisc 保留天然構象抗原</h2>
<p>P2X1 屬於 ATP 門控 (P2X) 家族：由三個同源亞基構成，每亞基兩次跨膜。范春雷教授團隊擴增 hP2X1-mFc.copGFP/293 細胞，製備<strong>保有天然四級結構的 hP2X1 Nanodisc</strong>，並經 Protein A/G 純化。這種天然構象抗原是整個平台的關鍵突破。</p>

<h2>從免疫到人源化 hP007A</h2>
<ul>
  <li>以 hP2X1 Nanodisc 免疫 7 隻 Balb/c 小鼠；其中 1 隻血清效價達 1:204,800（OD ≥ 1）。</li>
  <li>融合細胞 + ELISA 篩選獲得 16 個陽性克隆；前 8 個（P001–P008）進行後續驗證。</li>
  <li><strong>P007 與 P008</strong> 能將 LPS 抑制的中性粒細胞趨化能力從 CFS = 6 恢復至 CFS ≈ 16（健康對照水平），p &lt; 0.0001。</li>
  <li>19 種 CDR 移植至人源 IgG1 組合，篩得親和力最佳的 <strong>hP007A</strong>：Kd 從 0.389 μM（小鼠）降至 <strong>3.97 nM</strong>（人源化）— 提升 100 倍。</li>
</ul>

<h2>三大適應症 — 同一平台</h2>
<p>P2X1 在血小板、中性粒細胞、巨噬細胞及多種腫瘤細胞上表現。作為 ATP 門控鈣離子通道，它參與 LPS 誘導的全身炎症、細胞凋亡、突觸傳遞與血小板活化 — 一款抗體可撬動三個獨立適應症：</p>
<ol>
  <li><strong>膿毒症 / 嚴重內毒素血症</strong> — hP007A 提升 LPS 誘導膿毒症模型小鼠的生存率。</li>
  <li><strong>實體腫瘤</strong> — 0.92 mg/kg P007 單藥於 NOD-SCID + PBMC 的 H460（人肺癌）模型中實現<em>完全</em>抑制腫瘤生長，療效超越 Durvalumab（臨床 PD-L1 抗體），且瘤組織內部呈現空腔。</li>
  <li><strong>抗衰老 / 長壽（XD001）</strong> — 配套研發項目，影片記錄小鼠表型延壽至相當於人類約 132 歲。</li>
</ol>',
  research_html_zh = '<h2>關鍵臨床前數據</h2>
<h3>親和力（SPR，hP2X1 Nanodisc 芯片）</h3>
<table><thead><tr><th>抗體</th><th>Kd</th><th>備註</th></tr></thead>
<tbody>
<tr><td>P007（小鼠）</td><td>0.389 μM</td><td>原始克隆</td></tr>
<tr><td><strong>hP007A</strong>（人源化）</td><td><strong>3.97 nM</strong></td><td>CDR 移植至人源 IgG1</td></tr>
</tbody></table>

<h3>中性粒細胞趨化能力恢復</h3>
<p>LPS 抑制後的中性粒細胞（CFS = 6，****p &lt; 0.0001 vs 正常對照）。P007 與 P008 將 CFS 恢復至 16 與 15.8，與健康對照組無顯著差異（p &gt; 0.05）。</p>

<h3>患者體內 P2X1 表現</h3>
<ul>
  <li>膿毒症 / 感染患者（n = 53）：中性粒細胞表面 P2X1 顯著高於健康對照組；趨化評分與 P2X1 水平呈負相關。</li>
  <li>腫瘤患者：中性粒細胞 P2X1 表現上升；r = −0.83，****p &lt; 0.0001 與趨化能力呈負相關。</li>
</ul>

<h3>腫瘤抑制（體內，NOD-SCID + 人 PBMC）</h3>
<ul>
  <li>H460 肺癌模型：0.92 mg/kg P007 → <strong>完全</strong>抑制生長；優於 Durvalumab 陽性對照。</li>
  <li>A375 人黑色素瘤模型：3.68 mg/kg P007 劑量依賴性體積縮小，組織學觀察到空腔形成。</li>
</ul>

<h3>膿毒症生存率提升</h3>
<p>LPS 誘導致死性內毒素血症小鼠：hP007A（1 號）相對於 4 個 CDR2 定點突變克隆（2–5 號）呈現顯著的生存曲線分離，驗證功能表位。</p>

<h3>配套流式檢測抗體 FCM001</h3>
<p>P2X1 流式檢測抗體（專利 CN118652341A）在特異性與靈敏度均優於市售試劑。已完成技術轉讓：與<strong>中翰盛泰生物</strong>簽訂 200 萬元人民幣轉讓合約。</p>',
  risks_html_zh    = '<h3>研發階段風險</h3>
<ul>
  <li><strong>CMC 放大。</strong> hP007A 抗原製備需 Nanodisc 配方 — CDMO 資質認證處於關鍵路徑。時程：1.5–2 年，預算約 1,000 萬元人民幣。</li>
  <li><strong>雙軌監管申報。</strong> 項目將同時申報 NMPA 與 FDA IND；抗血栓、抗腫瘤、抗衰老可能拆分為獨立 IND。</li>
  <li><strong>首類離子通道單抗。</strong> 監管方對此類藥物的先例有限，預期需比常規 GPCR 抗體提供額外的安全性研究。</li>
  <li><strong>對融資進度依賴。</strong> 目前實驗室階段 5 年花費 1,000 萬元人民幣；CMC + 臨床前 + I 期臨床再需 ~2,100 萬元人民幣，受融資節點制約。延遲會層層影響。</li>
  <li><strong>Token / Phase 2 揭露。</strong> 本次 Phase 1 僅提供訂閱與內容展示；不涉及鏈上發售、購買合約與釋放時程。Phase 2（合約 + 交易）將另行公告，並附獨立的風險揭露。</li>
</ul>'
WHERE project_id = 2;

-- Project 2 · JSON 内 _zh 字段
UPDATE projects SET team_json = JSON_SET(team_json, '$[0].name_zh', "范春雷 教授", '$[0].role_zh', "項目負責人", '$[0].bio_zh', "浙江中醫藥大學生命科學學院。hP2X1 Nanodisc 抗原平台與 8 個 P2X1 功能性單株抗體克隆的主要發明人。三項已公開專利發明人（CN118638232A、CN118717967A、CN118652341A）與一項 PCT（國際檢索報告正面）。") WHERE project_id = 2;
UPDATE projects SET team_json = JSON_SET(team_json, '$[1].name_zh', "孫炳偉 教授", '$[1].role_zh', "臨床負責人", '$[1].bio_zh', "蘇州市立醫院。發現內毒素抑制中性粒細胞趨化功能與中性粒細胞膜上 P2X1 過表達相關 — 這是 P2X1 抗體項目的原始生物學洞見。") WHERE project_id = 2;
UPDATE projects SET team_json = JSON_SET(team_json, '$[2].name_zh', "JoinCare 基金會", '$[2].role_zh', "孵化夥伴", '$[2].bio_zh', "主導孵化，構建 CMC-CDMO 合作與雙軌（NMPA / FDA）IND 申報。") WHERE project_id = 2;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[0].quarter_zh', "2020–2025", '$[0].title_zh', "抗原、單抗、人源化階段完成", '$[0].desc_zh', "hP2X1 Nanodisc 抗原；8 個小鼠克隆（P001–P008）；P007 人源化為 hP007A（Kd 3.97 nM）。5 年預算 1,000 萬元人民幣。專利 CN118638232A、CN118717967A、CN118652341A + PCT。") WHERE project_id = 2;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[1].quarter_zh', "2026 Q3–Q4", '$[1].title_zh', "上線前訂閱與內容展示", '$[1].desc_zh', "此頁面。Notify-me + 收藏 + 官方 Updates。無鏈上發售。") WHERE project_id = 2;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[2].quarter_zh', "2026 Q4 – 2028 Q2", '$[2].title_zh', "CMC（委託 CDMO）", '$[2].desc_zh', "細胞株構建 → 規模化生產 → 穩定性研究。預算約 1,000 萬元人民幣。受種子輪關賬制約。") WHERE project_id = 2;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[3].quarter_zh', "2027 Q2 – Q4", '$[3].title_zh', "抗血栓適應症臨床前藥效 + 安全性", '$[3].desc_zh', "與 CMC 中期並行。預算約 600 萬元人民幣，委託 CDMO。") WHERE project_id = 2;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[4].quarter_zh', "2028 Q3", '$[4].title_zh', "IND 申報（NMPA + FDA）— 抗血栓", '$[4].desc_zh', "進入 I 期臨床。License-out 預期回收資金。") WHERE project_id = 2;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[5].quarter_zh', "2027 – 2028", '$[5].title_zh', "抗腫瘤與抗衰老適應症線", '$[5].desc_zh', "獨立的臨床前 + IND 申報。抗衰老 XD001 為配套項目。") WHERE project_id = 2;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[0].title_zh', "專利 · 功能性 P2X1 膜蛋白單抗製備與應用", '$[0].kind_zh', "專利 · CN118638232A") WHERE project_id = 2;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[1].title_zh', "專利 · 抗血小板凝集功能性 P2X1 單抗應用", '$[1].kind_zh', "專利 · CN118717967A") WHERE project_id = 2;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[2].title_zh', "專利 · P2X1 流式檢測單株抗體", '$[2].kind_zh', "專利 · CN118652341A") WHERE project_id = 2;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[3].title_zh', "PCT 國際檢索報告 — 新穎性、創造性、產業實用性均確認", '$[3].kind_zh', "PCT 報告") WHERE project_id = 2;
UPDATE projects SET faq_json = JSON_SET(faq_json, '$[0].q_zh', "為什麼 P2X1 抗體比 GPCR 抗體更難?", '$[0].a_zh', "GPCR 是七次跨膜單體膜蛋白。P2X1 是同源三聚體，每亞基兩次跨膜，且開放與關閉呈不同構象。以天然四級結構製備抗原是產業瓶頸 — 而 hP2X1 Nanodisc 平台正是解決此問題。") WHERE project_id = 2;
UPDATE projects SET faq_json = JSON_SET(faq_json, '$[1].q_zh', "Phase 1 只是訂閱頁，我實際加入的是什麼?", '$[1].a_zh', "Phase 1 = Notify-me 訂閱 + 官方 Updates + 收藏 + （Warmup 階段）白名單登記。無鏈上發售。Phase 2（合約、購買、釋放時程）將另行公告，並附獨立的風險揭露。") WHERE project_id = 2;
UPDATE projects SET faq_json = JSON_SET(faq_json, '$[2].q_zh', "如何從一款抗體衍生出三個適應症?", '$[2].a_zh', "P2X1 在血小板（血栓）、中性粒細胞（膿毒症、炎症）與多種腫瘤細胞（腫瘤）上表現。同一克隆 — 不同的作用機制。每個適應症獨立進行臨床前與 IND 申報。") WHERE project_id = 2;
UPDATE projects SET faq_json = JSON_SET(faq_json, '$[3].q_zh', "預期融資時程?", '$[3].a_zh', "目前種子資本覆蓋 CMC（1,000 萬元人民幣，約 2 年）與主要適應症臨床前（600 萬元人民幣，約 6 個月）。NMPA + FDA IND 申報目標為 2028 Q3。後續抗腫瘤 / 抗衰老 IND 靠 license-out 回收循環資金。") WHERE project_id = 2;
UPDATE projects SET onchain_json = JSON_SET(onchain_json, '$[0].label_zh', "P2X1 代幣（TGE 後）") WHERE project_id = 2;
UPDATE projects SET onchain_json = JSON_SET(onchain_json, '$[1].label_zh', "釋放合約") WHERE project_id = 2;
UPDATE projects SET onchain_json = JSON_SET(onchain_json, '$[2].label_zh', "公開發售合約") WHERE project_id = 2;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[0].label_zh', "公開發售（Phase 2）") WHERE project_id = 2;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[1].label_zh', "研發 + CMC 儲備") WHERE project_id = 2;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[2].label_zh', "團隊 + 顧問") WHERE project_id = 2;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[3].label_zh', "生態 / 社群") WHERE project_id = 2;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[4].label_zh', "基金會金庫") WHERE project_id = 2;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[5].label_zh', "流動性") WHERE project_id = 2;


-- Project 3 · 繁体中文内容 + cover
UPDATE projects SET
  cover_url        = '/figma/v2/project-card-image-4.png',
  tagline_zh       = '60 年的 GABA 抑制神經受體 / gephyrin 理論體系，落地為三條神經健康產品線 — FDA 保健品、40Hz 二類醫療器械、D-P 新藥管線。',
  story_html_zh    = '<h2>理論 — 兩代 GABA 抑制神經受體科學家</h2>
<p><strong>1969 · 理查・奧爾森教授（UCLA）</strong> — 被譽為「GABA 受體之父」。發現大腦由 GABA 抑制與興奮兩個系統平衡運作；健康即平衡。抑制系統失效時，興奮系統釋放神經毒素，導致神經元受損並中斷傳導。</p>
<p><strong>2004 · 梁京教授</strong> — 提出導向 / 支撐蛋白（gephyrin）假說。當大腦 gephyrin 降至基線 50% 以下，GABA-A 受體功能崩潰。此假說目前被視為阿茲海默症的<em>第四大病理假說</em>。</p>
<p><strong>2020 · 線粒體層面延伸</strong> — 腦細胞線粒體受損 → ATP 下降 → gephyrin 下降 → GABA-A 信號流失 → 神經退化。</p>
<p><strong>2021 · 神經免疫缺損假說</strong> — 過度活化的小膠質細胞（壓力、社交隔離、睡眠不足所致）引發神經炎症。</p>

<h2>化合物 — 從 6,000 種植物黃酮到 D-P</h2>
<p>自 2003 年，梁京實驗室篩選了 6,000 多種天然植物黃酮。<strong>D-黃酮組合</strong>（天然小分子黃酮混合）作為 GABA-A 正向調節劑脫穎而出：可恢復 gephyrin、提升線粒體 ATP、快速穿越血腦屏障、無依賴性。2018 年以 FDA 註冊的 Smarto-One 保健品問世。</p>
<p>2020 年：對天然骨架進行藥物化學優化，得到<strong>單體化合物 D-P</strong> — 穩定性與功效提升 50 倍，並實現<strong>15 分鐘內快速穿越血腦屏障</strong>。D-P 為新藥管線的基底化合物（D-P1 針對阿茲海默症中早期 + BPSD 症狀；D-P2 針對抑鬱 / 情感障礙）。</p>

<h2>三條產品線，同一套理論</h2>
<ol>
  <li><strong>保健品（FDA 註冊）— Smarto-One / Carefreeing™ 系列。</strong>已於美中兩地跨境電商上線。288 人志願者研究顯示能改善睡眠 / 壓力 / 情緒。目標客群為 25–40 歲都市女性；三年營收計劃 1.2 億 / 3.5 億 / 6 億人民幣（2026–2028）。</li>
  <li><strong>二類醫療器械 — Smarto Light™ 40Hz。</strong>非侵入式伽瑪波多感官（聲 + 光 + 振動）刺激。基於 MIT（蔡立慧院士）40Hz 伽瑪研究。二類醫療器械註冊申請中。與北京和輝光電（ISO 9001，申請 ISO 13485）合作生產。</li>
  <li><strong>新藥 — D-P1 / D-P2。</strong>D-P1：阿茲海默症中早期治療 + BPSD 症狀改善。D-P2：情感障礙。兩者實驗室階段資料均已完成，等待 I 期臨床。配套 IVD：4 個檢測試劑盒（AD7C-NTP、Aβ1-42、P-Tau-181、Aβ42）。</li>
</ol>',
  research_html_zh = '<h2>證據基礎</h2>

<h3>D-黃酮 — 人體與動物實驗證據</h3>
<ul>
  <li>增強 GABA-A 受體張力電流與相位電流（Shen 等，<em>J. Neuroscience</em> 2012）。</li>
  <li>降低轉基因 AD 小鼠 Aβ 斑塊負荷（Liang 等，<em>J. Neurochem</em> 2014）。</li>
  <li>改善 AD 模型囓齒動物的焦慮、記憶、運動功能。透過小膠質細胞正常化逆轉壓力誘導的神經炎症（Liang 等，<em>J. Neuroinflammation</em> 2022）。</li>
  <li>288 人人體觀察研究 — 減少睡眠障礙、壓力與情緒問題（PLOS ONE 2022，Smarto-One）。</li>
  <li>USC IRB 核准的人體試驗；2018 年至今累計 20,000+ 真實消費者研究。</li>
</ul>

<h3>D-P — 第二代優化單體</h3>
<ul>
  <li>相較 D-黃酮天然骨架，穩定性與功效提升 50 倍。</li>
  <li>15 分鐘內穿越血腦屏障（解決小分子生體可用率瓶頸）。</li>
  <li>8 小時全身清除，無殘留、無依賴、無戒斷風險。</li>
  <li>實驗室階段數據 + 可重現性 + 穩定製備方法均已完成。等待 I 期臨床。</li>
</ul>

<h3>40Hz 伽瑪醫療器械 — MIT 臨床證據</h3>
<ul>
  <li>Aβ40 清除率提升 28%（Nature — 類淋巴機制）。</li>
  <li>Tau 磷酸化降低 37%（Tau P301S 模型）。</li>
  <li>MMSE 認知衰退速度減緩 83%（n = 76 雙盲）。</li>
  <li>ADCS-ADL 評分下降速率減緩 84%（p &lt; 0.001）。</li>
  <li>默認模式網路連接度提升 41%；6 週後突觸蛋白標記物提升 68%。</li>
  <li>相較口服多奈哌齊（6 個月）：MMSE 改善 +3.2 vs +1.5；副作用發生率 &lt; 1% vs 28%；日均成本 ¥80 vs ¥220；依從性 98% vs 63%。</li>
</ul>

<h3>市場 — 保健品管線</h3>
<ul>
  <li>中國神經健康保健品市場：2025 年預估 1,800 億元人民幣，年複合增長率 15%+（Frost &amp; Sullivan）。</li>
  <li>中國情感障礙：1.32 億患者，人均年支出 1.7 萬元人民幣，市場空間 324.7 億元人民幣。</li>
  <li>中國睡眠障礙：3 億患者，人均年支出 932 元人民幣，市場空間 4,000 億元人民幣。</li>
</ul>',
  risks_html_zh    = '<h3>三條監管路徑 — 三種風險特徵</h3>
<ul>
  <li><strong>保健品（Smarto-One）</strong> — 已獲美國 FDA 膳食補充劑註冊 + 中國食品質量檢驗認證。監管風險最低；產品已產生現金流。風險在於競爭（中國市場多數同類產品缺乏臨床驗證 — 市場教育成本）。</li>
  <li><strong>二類醫療器械（Smarto Light™ 40Hz）</strong> — 兩份二類申請進行中。標準醫療器械時程（18–24 個月）；生產夥伴（北京和輝光電）正申請 ISO 13485。IP：MIT 40Hz 方法（授權）+ 專有三合一（聲 / 光 / 振動）方案 + 原創 40Hz 音樂版權。</li>
  <li><strong>新藥（D-P1 / D-P2）</strong> — 完整的 FDA / NMPA 新藥路徑。IND 前研究 3–6 個月；前置研究 6 個月；臨床前 18 個月；IND 審查 3–6 個月；分期臨床 1 年；NDA 6–12 個月。並非全新化合物（天然 D-黃酮母骨架已具備消費級安全紀錄），但整體時程仍為 4–5 年。</li>
</ul>

<h3>融資架構</h3>
<ul>
  <li>種子輪：目標人民幣 4,500 萬元，釋出 20–25% 股權。關賬期限公告為 2026 年 1 月 30 日。</li>
  <li>資金用途：藥品研發 + 產品研發 + 跨境電商品牌 + 線上市場投流 + 渠道建設 + 生產。</li>
  <li>完整財務模型與其他戰略文件需在簽署 NDA 後提供。</li>
  <li><strong>本次 Phase 1 僅提供訂閱與內容展示</strong> — 無鏈上發售、無購買合約、無釋放時程上線。Phase 2（合約 + 交易）將另行公告，並附獨立的風險揭露。</li>
</ul>'
WHERE project_id = 3;

-- Project 3 · JSON 内 _zh 字段
UPDATE projects SET team_json = JSON_SET(team_json, '$[0].name_zh', "梁京 教授", '$[0].role_zh', "科學負責人", '$[0].bio_zh', "美國南加州大學藥學院教授。英國皇家醫學院院士（2019 迄今）。倫敦「新科學家」評選（2012）。提出「導向 / 支撐蛋白 — gephyrin」假說（2004），後被公認為阿茲海默症第四大病理假說。發表 82 篇同行評議論文；2012 年 J. Neuroscience 論文入選 NIH「影響世界的 1000 篇」。2008 年 NIH 大腦挑戰基金首席科學家。《中國藥理學》期刊主編。多項技術發明專利。") WHERE project_id = 3;
UPDATE projects SET team_json = JSON_SET(team_json, '$[1].name_zh', "理查・奧爾森 教授（追念）", '$[1].role_zh', "奠基理論", '$[1].bio_zh', "UCLA 醫學院終身傑出教授。被譽為「GABA 受體之父」。獲諾貝爾委員會頒發神經科學終身成就獎。NIH 職業發展大獎；Javits 神經科學研究獎；美國 HHS 心血管成就獎。發表 266 篇 SCI 論文；擔任《藥物學》《神經科學》《癲癇病》《酒精研究》編委。") WHERE project_id = 3;
UPDATE projects SET team_json = JSON_SET(team_json, '$[2].name_zh', "王蓉 教授", '$[2].role_zh', "中國實驗室負責人", '$[2].bio_zh', "首都醫科大學宣武醫院中心實驗室主任。國家老年疾病臨床醫學研究中心 AD 外周標誌物檢測與早期干預聯盟主任委員。北京神經內科學會基礎與轉化醫學專業委員會主任委員。神經絲蛋白 AD7c-NTP 尿液檢測發明人。發表 32 篇第一作者、92 篇通訊作者論文，其中 SCI 37 篇。") WHERE project_id = 3;
UPDATE projects SET team_json = JSON_SET(team_json, '$[3].name_zh', "于順 教授", '$[3].role_zh', "中國實驗室 · 帕金森病", '$[3].bio_zh', "首都醫科大學宣武醫院國家老年疾病臨床研究中心。北京市老年病醫療研究中心分子診斷實驗室主任。首都醫科大學帕金森病研究所副所長。") WHERE project_id = 3;
UPDATE projects SET team_json = JSON_SET(team_json, '$[4].name_zh', "劉魚 教授", '$[4].role_zh', "中國實驗室 · 輸血醫學", '$[4].bio_zh', "北京協和醫學院教授。約翰霍普金斯大學醫學院訪問學者。中國醫學科學院輸血研究所輸血醫學實驗中心主任。") WHERE project_id = 3;
UPDATE projects SET team_json = JSON_SET(team_json, '$[5].name_zh', "趙凡", '$[5].role_zh', "行銷長 · 商務負責人", '$[5].bio_zh', "前抖音市場部 CMO / 服飾快消行業總經理。操盤業務規模 5,000 億元人民幣，年預算 70 億元人民幣，管理團隊 250 人。負責 B 端生態、C 端品牌建設與跨境電商渠道。") WHERE project_id = 3;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[0].quarter_zh', "2003–2018", '$[0].title_zh', "D-黃酮發現 → Smarto-One 上市", '$[0].desc_zh', "篩選 6,000+ 種黃酮 → D-黃酮組合 → FDA 保健品註冊 → 中美渠道 + 20,000 人真實研究。") WHERE project_id = 3;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[1].quarter_zh', "2020", '$[1].title_zh', "D-P 單體化合物完成", '$[1].desc_zh', "結構優化單體，穩定性與功效提升 50 倍，實現 15 分鐘穿越血腦屏障。實驗室階段 + 可重現性 + 製備方法均已完成。") WHERE project_id = 3;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[2].quarter_zh', "2026 Q3–Q4", '$[2].title_zh', "上線前訂閱與內容展示", '$[2].desc_zh', "此頁面。Notify-me + 收藏 + 官方 Updates。無鏈上發售。") WHERE project_id = 3;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[3].quarter_zh', "2026 Q4", '$[3].title_zh', "種子輪關賬（人民幣 4,500 萬元，釋出 20–25% 股權）", '$[3].desc_zh', "期限 2026-01-30。資金用途對應藥品研發 + 醫療器械建設 + 渠道拓展。") WHERE project_id = 3;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[4].quarter_zh', "2027", '$[4].title_zh', "D-P1 IND（阿茲海默症中早期 + BPSD）", '$[4].desc_zh', "18 個月臨床前 → IND 申報。並行：跨境電商 3.5 億元人民幣營收計劃（渠道矩陣 + 明星直播帶貨）。") WHERE project_id = 3;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[5].quarter_zh', "2027", '$[5].title_zh', "Smarto Light™ 40Hz — 二類醫療器械上市", '$[5].desc_zh', "兩份二類申請預期核准。居家眼罩 + 醫院睡眠艙 SKU。ISO 13485 定案。") WHERE project_id = 3;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[6].quarter_zh', "2028", '$[6].title_zh', "D-P2 IND（情感障礙 / 抑鬱）", '$[6].desc_zh', "CMC 中期後 6 個月臨床前。IVD 4 檢測面板配套上市，用於 AD 早期篩查。") WHERE project_id = 3;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[7].quarter_zh', "2028", '$[7].title_zh', "商務第三年 · 6 億元人民幣營收計劃", '$[7].desc_zh', "抖音店播矩陣 + 天貓 / 京東 + 山姆 + 100 家線下零售。毛利率目標 82%。投資者退出：IPO / 併購（對標湯臣倍健收購 Life-Space PS 5–8 倍）。") WHERE project_id = 3;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[0].title_zh', "美國 FDA · 膳食補充劑註冊（Carefreeing™ / Smarto-One）", '$[0].kind_zh', "監管") WHERE project_id = 3;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[1].title_zh', "中國食品質量檢驗認證", '$[1].kind_zh', "監管") WHERE project_id = 3;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[2].title_zh', "普尼測試 · 急性經口毒性認證", '$[2].kind_zh', "安全") WHERE project_id = 3;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[3].title_zh', "D-黃酮 — 18 篇代表性同行評議論文", '$[3].kind_zh', "論文") WHERE project_id = 3;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[4].title_zh', "MIT 40Hz 伽瑪 — Nature 類淋巴機制 + NCT-03556280 II 期參考", '$[4].kind_zh', "論文") WHERE project_id = 3;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[5].title_zh', "Smarto-One PLOS ONE 2022 — 288 人壓力 / 焦慮 / 睡眠研究", '$[5].kind_zh', "論文") WHERE project_id = 3;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[6].title_zh', "湯臣倍健收購 Life-Space（PS 5–8 倍退出參考）", '$[6].kind_zh', "退出對標") WHERE project_id = 3;
UPDATE projects SET faq_json = JSON_SET(faq_json, '$[0].q_zh', "D-黃酮（Smarto-One）與 D-P 新藥有什麼區別？", '$[0].a_zh', "D-黃酮是天然植物黃酮混合物，自 2018 年起以 FDA 註冊的膳食補充劑形式銷售。D-P 是同一骨架經藥物化學優化的單體化合物 — 穩定性與功效提升 50 倍，15 分鐘穿越血腦屏障，現進入完整的新藥路徑（阿茲海默症 D-P1、抑鬱 D-P2）。") WHERE project_id = 3;
UPDATE projects SET faq_json = JSON_SET(faq_json, '$[1].q_zh', "為何同時發展三條產品線？", '$[1].a_zh', "同一套理論（GABA-A / gephyrin 修復），三條監管路徑。保健品已產生現金流，資助研發。二類醫療器械 18–24 個月回本。新藥為長線高價值資產。跨產品線交叉補貼。") WHERE project_id = 3;
UPDATE projects SET faq_json = JSON_SET(faq_json, '$[2].q_zh', "Phase 1 只是訂閱頁，我實際加入的是什麼？", '$[2].a_zh', "Phase 1 = Notify-me 訂閱 + 官方 Updates + 收藏 + （Warmup 階段）白名單登記。無鏈上發售。Phase 2（合約 + 購買 + 釋放時程）將另行公告，並附獨立的風險揭露。") WHERE project_id = 3;
UPDATE projects SET faq_json = JSON_SET(faq_json, '$[3].q_zh', "種子輪何時關賬？", '$[3].a_zh', "種子輪目標人民幣 4,500 萬元，釋出 20–25% 股權。公告期限：2026-01-30。詳細財務模型與其他戰略文件需在簽署 NDA 後提供。") WHERE project_id = 3;
UPDATE projects SET onchain_json = JSON_SET(onchain_json, '$[0].label_zh', "SMRT 代幣（TGE 後）") WHERE project_id = 3;
UPDATE projects SET onchain_json = JSON_SET(onchain_json, '$[1].label_zh', "釋放合約") WHERE project_id = 3;
UPDATE projects SET onchain_json = JSON_SET(onchain_json, '$[2].label_zh', "公開發售合約") WHERE project_id = 3;
UPDATE projects SET onchain_json = JSON_SET(onchain_json, '$[3].label_zh', "基金會金庫多簽") WHERE project_id = 3;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[0].label_zh', "公開發售（Phase 2）") WHERE project_id = 3;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[1].label_zh', "研發 — D-P 新藥") WHERE project_id = 3;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[2].label_zh', "醫療器械 + IVD 建設") WHERE project_id = 3;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[3].label_zh', "團隊 + 顧問") WHERE project_id = 3;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[4].label_zh', "跨境電商") WHERE project_id = 3;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[5].label_zh', "基金會金庫") WHERE project_id = 3;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[6].label_zh', "流動性") WHERE project_id = 3;


-- END 2026_07_04_projects_zh.sql

-- BEGIN 2026_07_04_seed_projects.sql
-- Seed two Phase-1 projects (P2X1 antibody + Smarto Neuro)
-- Idempotent: ON DUPLICATE KEY UPDATE on projects.project_id (unique)
-- Generated by migrations/_gen_2026_07_04_seed_projects.py
SET NAMES utf8mb4;
INSERT INTO projects (
  `project_id`,`owner_address`,`name_en`,`name_cn`,`tagline`,`description`,`tags`,`team_info`,`roadmap`,`partners`,`token_address`,`token_symbol`,`token_decimals`,`payment_token`,`token_price`,`hard_cap`,`soft_cap`,`max_per_user`,`start_time`,`end_time`,`tge_time`,`status`,`category`,`cover_url`,`cover_video_url`,`launch_at`,`story_html`,`research_html`,`risks_html`,`team_json`,`tokenomics_json`,`roadmap_json`,`documents_json`,`faq_json`,`onchain_json`
) VALUES (
  2,
  '0x0000000000000000000000000000000000000000',
  'Functional P2X1 Antibody Development',
  '新型功能性 P2X1 单抗药物研发',
  'First-in-class functional monoclonal antibody targeting the P2X1 ion channel — one platform, three indications (sepsis, oncology, longevity).',
  'The P2X1 antibody platform delivers the first functional monoclonal antibody targeting an ATP-gated calcium ion channel. Ion-channel mAbs are a globally difficult class of biologics — P2X1 is a homotrimeric two-pass transmembrane channel whose open/closed conformations pose an antigen-preparation problem no small-molecule can solve. Prof. Chunlei Fan''s team at Zhejiang Chinese Medical University''s School of Life Sciences has produced hP2X1 Nanodiscs preserving native conformation, immunized Balb/c mice, and screened 8 positive clones. Clone P007 restores LPS-suppressed neutrophil chemotaxis to baseline, humanized hP007A achieves 100× affinity gain (Kd = 3.97 nM), and pre-clinical animal work shows tumor-growth suppression outperforming the PD-L1 antibody Durvalumab, plus a sepsis-survival rescue in mice.',
  '["Antibody", "P2X1", "Ion Channel", "Sepsis", "Oncology", "Anti-aging", "DeSci"]',
  '',
  '',
  'Zhejiang Chinese Medical University · Suzhou Municipal Hospital · Zhonghansentai Biotech (200万流式抗体转让合作)',
  '0x0000000000000000000000000000000000000000',
  'P2X1',
  18,
  'JCC',
  0.10,
  0,
  0,
  0,
  '2026-09-30 12:00:00',
  '2026-10-30 12:00:00',
  '2026-11-15 12:00:00',
  'pre_launch',
  'Biotech · Antibody',
  '',
  '',
  '2026-09-30 12:00:00',
  '<h2>Background — why ion-channel mAbs are the hardest class of biologics</h2>
<p>G-protein-coupled receptors (GPCRs) and ion channels together account for nearly half of all FDA-approved drug targets (34% and 13% respectively), yet only two GPCR monoclonal antibodies (<em>erenumab</em>, <em>mogamulizumab</em>) have reached the clinic. Antibodies outperform small molecules on selectivity, bioavailability, half-life and effector function — but antigen preparation with native conformation remains the industry blocker.</p>
<p>Ion channels are strictly harder than GPCRs. GPCRs are single-subunit seven-pass membrane proteins; ion channels are <strong>multi-subunit, multi-pass functional assemblies that switch between open and closed conformations</strong>. Pfizer''s biologics group is publicly working on GPCR / ion-channel antibody drugs — progress is slow.</p>

<h2>Our approach — native-conformation antigen via Nanodiscs</h2>
<p>P2X1 belongs to the ATP-gated (P2X) family: a homotrimer with each subunit crossing the membrane twice. Prof. Chunlei Fan''s team amplified hP2X1-mFc.copGFP/293 cells, prepared <strong>hP2X1 Nanodiscs</strong> preserving native quaternary structure, and purified via Protein A/G. This native-conformation antigen unlocks the entire program.</p>

<h2>From immunization to humanized hP007A</h2>
<ul>
  <li>Immunized 7 Balb/c mice with hP2X1 Nanodiscs; one mouse showed serum titer ≥ 1:204,800 (OD ≥ 1).</li>
  <li>Fusion + ELISA screen yielded 16 positive clones; the top 8 (P001–P008) were characterized.</li>
  <li><strong>P007 and P008</strong> restore LPS-suppressed neutrophil chemotaxis from CFS = 6 back to CFS ≈ 16 (normal baseline), p &lt; 0.0001.</li>
  <li>CDR grafting onto human IgG1 across 19 combinations produced <strong>hP007A</strong> with best affinity: Kd shifted from 0.389 μM (mouse) to <strong>3.97 nM</strong> (humanized) — a 100× gain.</li>
</ul>

<h2>Three indication tracks — one platform</h2>
<p>P2X1 is expressed on platelets, neutrophils, macrophages, smooth muscle and multiple tumor lines. As an ATP-gated Ca²⁺ channel it participates in LPS-induced systemic inflammation, apoptosis, synaptic transmission, and platelet activation — giving one antibody three orthogonal indications:</p>
<ol>
  <li><strong>Sepsis / severe endotoxemia</strong> — hP007A rescues mouse survival in LPS-induced sepsis models.</li>
  <li><strong>Solid-tumor oncology</strong> — 0.92 mg/kg P007 monotherapy achieved <em>complete</em> tumor-growth suppression in NOD-SCID mice grafted with H460 (human lung cancer) + PBMCs, outperforming Durvalumab (clinical PD-L1 mAb), with cavity formation observed in tumor tissue.</li>
  <li><strong>Anti-aging / longevity (XD001)</strong> — companion program, video-documented mouse phenotype extension to ~132 human-year equivalent.</li>
</ol>',
  '<h2>Key pre-clinical data</h2>
<h3>Affinity (SPR, hP2X1 Nanodiscs chip)</h3>
<table><thead><tr><th>Antibody</th><th>Kd</th><th>Note</th></tr></thead>
<tbody>
<tr><td>P007 (mouse)</td><td>0.389 μM</td><td>original clone</td></tr>
<tr><td><strong>hP007A</strong> (humanized)</td><td><strong>3.97 nM</strong></td><td>CDR-grafted onto human IgG1</td></tr>
</tbody></table>

<h3>Neutrophil chemotaxis rescue</h3>
<p>LPS-suppressed neutrophils (CFS = 6, ****p &lt; 0.0001 vs normal). P007 and P008 restore CFS to 16 and 15.8 respectively — indistinguishable from healthy controls (p &gt; 0.05).</p>

<h3>P2X1 expression in patients</h3>
<ul>
  <li>Sepsis / infection patients (n = 53): neutrophil-surface P2X1 significantly higher than healthy controls; chemotaxis scores negatively correlated with P2X1 level.</li>
  <li>Cancer patients: neutrophil P2X1 elevated; r = −0.83, ****p &lt; 0.0001 with chemotaxis capacity.</li>
</ul>

<h3>Tumor suppression (in-vivo, NOD-SCID + human PBMC)</h3>
<ul>
  <li>H460 lung-cancer model: 0.92 mg/kg P007 → <strong>complete</strong> growth suppression; superior to Durvalumab positive control.</li>
  <li>A375 human-melanoma model: 3.68 mg/kg P007 dose-dependent volume reduction, cavity formation histology.</li>
</ul>

<h3>Sepsis survival rescue</h3>
<p>LPS-induced lethal endotoxemia mice: hP007A (clone 1) shows clear survival curve separation vs 4 CDR2-point-mutant clones (2–5), validating functional epitope.</p>

<h3>Companion assay — flow-cytometry antibody FCM001</h3>
<p>P2X1 flow-detection antibody (patent CN118652341A) outperforms commercial reagents in specificity and sensitivity. Already licensed: <strong>¥2,000,000 transfer contract</strong> with Zhonghansentai Biotech.</p>',
  '<h3>Development-stage risks</h3>
<ul>
  <li><strong>CMC scale-up.</strong> hP007A antigen prep requires Nanodisc formulation — CDMO qualification is on the critical path. Timeline: 1.5–2 years, budget ~¥10M.</li>
  <li><strong>Regulatory dual-track.</strong> Program will pursue both NMPA and FDA IND simultaneously; anti-thrombosis vs anti-tumor vs anti-aging indications may split into separate INDs.</li>
  <li><strong>First-in-class ion-channel mAb.</strong> Regulators have limited precedent; expect additional safety-margin studies vs a well-trodden GPCR mAb path.</li>
  <li><strong>Program-financing dependency.</strong> The current lab-completed stage cost ¥10M over 5 years; CMC + preclinical + Phase-I is another ~¥21M gated by fundraising. Delays cascade.</li>
  <li><strong>Token / Phase-2 disclosure.</strong> Phase-1 of this listing is subscription and content only; no on-chain sale, no purchase contract, no vesting is live. Phase-2 (contracts + trading) will be re-announced with independent risk disclosures.</li>
</ul>',
  '[{"name": "Prof. Chunlei Fan (范春雷)", "role": "PROGRAM LEAD", "bio": "Zhejiang Chinese Medical University · School of Life Sciences. Principal inventor of the hP2X1 Nanodisc antigen platform and 8 P2X1 functional mAb clones. Named inventor on 3 published patents (CN118638232A, CN118717967A, CN118652341A) and one PCT with positive international-search report.", "avatar_url": ""}, {"name": "Prof. Bingwei Sun (孙炳伟)", "role": "CLINICAL LEAD", "bio": "Suzhou Municipal Hospital. Discovered that endotoxin-suppressed neutrophil chemotaxis correlates with P2X1 overexpression on the neutrophil membrane — the original biological insight motivating the P2X1 antibody program.", "avatar_url": ""}, {"name": "JoinCare Foundation", "role": "INCUBATION PARTNER", "bio": "Lead incubator, structuring CMC-CDMO partnerships and dual-track (NMPA / FDA) IND filings.", "avatar_url": ""}]',
  '{"tge_percent": 5, "cliff_months": 12, "linear_months": 24, "allocations": [{"label": "Public sale (Phase 2)", "percent": 20, "color": "#74f5a1"}, {"label": "R&D + CMC reserve", "percent": 30, "color": "#3884ff"}, {"label": "Team + advisors", "percent": 15, "color": "#fa8c16"}, {"label": "Ecosystem / community", "percent": 15, "color": "#a55dff"}, {"label": "Foundation treasury", "percent": 10, "color": "#212121"}, {"label": "Liquidity", "percent": 10, "color": "#1aa64a"}]}',
  '[{"quarter": "2020–2025", "title": "Antigen · mAb · humanization complete", "desc": "hP2X1 Nanodisc antigen; 8 mouse clones (P001–P008); P007 humanized to hP007A (Kd 3.97 nM). 5-year budget ¥10M. Patents CN118638232A, CN118717967A, CN118652341A + PCT.", "status": "done"}, {"quarter": "2026 Q3–Q4", "title": "Pre-launch subscription + content live", "desc": "This page. Notify-me + watchlist + updates. No on-chain sale yet.", "status": "active"}, {"quarter": "2026 Q4 – 2028 Q2", "title": "CMC (CDMO)", "desc": "Cell-line construction → scaled production → stability studies. Budget ~¥10M via CDMO. Gated by seed round close.", "status": "todo"}, {"quarter": "2027 Q2 – Q4", "title": "Preclinical efficacy + safety (thrombosis indication)", "desc": "Parallel to CMC mid-stage. Budget ~¥6M via CDMO.", "status": "todo"}, {"quarter": "2028 Q3", "title": "IND (NMPA + FDA) — thrombosis", "desc": "Enter Phase I. License-out expected to recycle capital.", "status": "todo"}, {"quarter": "2027 – 2028", "title": "Oncology + anti-aging indication tracks", "desc": "Independent preclinical + IND filings. Anti-aging XD001 companion program.", "status": "todo"}]',
  '[{"title": "Patent · Functional P2X1 mAb — preparation & application", "kind": "PATENT · CN118638232A", "url": "https://cprs.patentstar.com.cn/Search/Detail?ANE=CN118638232A"}, {"title": "Patent · Anti-platelet-aggregation P2X1 mAb application", "kind": "PATENT · CN118717967A", "url": "https://cprs.patentstar.com.cn/Search/Detail?ANE=CN118717967A"}, {"title": "Patent · P2X1 flow-cytometry monoclonal detection antibody", "kind": "PATENT · CN118652341A", "url": "https://cprs.patentstar.com.cn/Search/Detail?ANE=CN118652341A"}, {"title": "PCT international search report — novelty / inventiveness / industrial applicability confirmed", "kind": "PCT REPORT", "url": ""}]',
  '[{"q": "Why is a P2X1 antibody drug harder than a GPCR antibody drug?", "a": "GPCRs are seven-pass single-subunit membrane proteins. P2X1 is a homotrimer, each subunit crossing the membrane twice, with distinct open/closed conformations. Producing an antigen in native quaternary structure is the industry blocker — and it is what the hP2X1 Nanodisc platform solves."}, {"q": "Isn''t Phase-1 just a subscription page — what am I actually joining?", "a": "Phase 1 = notify-me subscription + team-published Updates + watchlist + (in warmup) whitelist. Nothing on-chain sells here. Phase 2 (contracts, purchase, vesting) will be a separately-announced event with its own risk disclosures."}, {"q": "How does the platform go from one antibody to three indications?", "a": "P2X1 is expressed on platelets (thrombosis), neutrophils (sepsis, inflammation) and multiple tumor lines (oncology). One clone — different mechanism-of-action. Independent preclinical + IND per indication."}, {"q": "What''s the expected fundraising timeline?", "a": "The current seed capital covers CMC (¥10M, ~2 years) and preclinical for the lead indication (¥6M, ~6 months). NMPA + FDA IND filings target 2028 Q3. Subsequent oncology / anti-aging IND filings are self-financed via the license-out recycle."}]',
  '[{"label": "P2X1 token (post-TGE)", "address": "", "chain": "BSC"}, {"label": "Vesting contract", "address": "", "chain": "BSC"}, {"label": "Public-sale contract", "address": "", "chain": "BSC"}]'
) ON DUPLICATE KEY UPDATE
  `owner_address`=VALUES(`owner_address`),
  `name_en`=VALUES(`name_en`),
  `name_cn`=VALUES(`name_cn`),
  `tagline`=VALUES(`tagline`),
  `description`=VALUES(`description`),
  `tags`=VALUES(`tags`),
  `team_info`=VALUES(`team_info`),
  `roadmap`=VALUES(`roadmap`),
  `partners`=VALUES(`partners`),
  `token_address`=VALUES(`token_address`),
  `token_symbol`=VALUES(`token_symbol`),
  `token_decimals`=VALUES(`token_decimals`),
  `payment_token`=VALUES(`payment_token`),
  `token_price`=VALUES(`token_price`),
  `hard_cap`=VALUES(`hard_cap`),
  `soft_cap`=VALUES(`soft_cap`),
  `max_per_user`=VALUES(`max_per_user`),
  `start_time`=VALUES(`start_time`),
  `end_time`=VALUES(`end_time`),
  `tge_time`=VALUES(`tge_time`),
  `status`=VALUES(`status`),
  `category`=VALUES(`category`),
  `cover_url`=VALUES(`cover_url`),
  `cover_video_url`=VALUES(`cover_video_url`),
  `launch_at`=VALUES(`launch_at`),
  `story_html`=VALUES(`story_html`),
  `research_html`=VALUES(`research_html`),
  `risks_html`=VALUES(`risks_html`),
  `team_json`=VALUES(`team_json`),
  `tokenomics_json`=VALUES(`tokenomics_json`),
  `roadmap_json`=VALUES(`roadmap_json`),
  `documents_json`=VALUES(`documents_json`),
  `faq_json`=VALUES(`faq_json`),
  `onchain_json`=VALUES(`onchain_json`);

INSERT INTO projects (
  `project_id`,`owner_address`,`name_en`,`name_cn`,`tagline`,`description`,`tags`,`team_info`,`roadmap`,`partners`,`token_address`,`token_symbol`,`token_decimals`,`payment_token`,`token_price`,`hard_cap`,`soft_cap`,`max_per_user`,`start_time`,`end_time`,`tge_time`,`status`,`category`,`cover_url`,`cover_video_url`,`launch_at`,`story_html`,`research_html`,`risks_html`,`team_json`,`tokenomics_json`,`roadmap_json`,`documents_json`,`faq_json`,`onchain_json`
) VALUES (
  3,
  '0x0000000000000000000000000000000000000000',
  'Smarto Neuro — Prof. Jing Liang Lab Technology Transfer',
  'Smarto Neuro · 梁京院士团队专利成果转化',
  '60 years of GABA / gephyrin neuroscience turned into three neuro-health product lines — FDA-cleared supplement, 40Hz Class-II device, and D-P new-drug pipeline.',
  'Smarto Neuro is the technology-transfer vehicle for two generations of GABA-A / inhibitory-neuroreceptor science: Prof. Richard W. Olsen (UCLA — the ''father of the GABA receptor''), and Prof. Jing Liang (USC School of Pharmacy · Fellow of the Royal Society of Medicine). Prof. Liang extended Olsen''s 1969 inhibitory-neuroreceptor theory into a mitochondrial-scale mechanism (2020) and a neuro-immune-deficiency hypothesis (2021). From 6,000+ natural plant flavonoids, the lab isolated the D-Flavonoid group — a positive modulator of gephyrin (the guiding/support protein of GABA-A receptors) — which shipped in 2018 as the FDA-registered Smarto-One supplement (over 20,000 user studies). A 2020 medicinal-chemistry sweep produced D-P, a novel single-molecule with 50× safety/efficacy gain and 15-minute blood-brain-barrier penetration. In parallel, a Class-II medical device based on MIT''s 40Hz gamma stimulation targets clinical Alzheimer''s intervention.',
  '["Neuroscience", "Alzheimer''s", "GABA-A", "Gephyrin", "Class-II Device", "Nutraceutical", "DeSci"]',
  '',
  '',
  'USC School of Pharmacy · Xuanwu Hospital (Capital Medical University) · Peking Union Medical College · MIT (40Hz IP) · Beijing Hehui Optoelectronics · ISO 9001 / applying ISO 13485',
  '0x0000000000000000000000000000000000000000',
  'SMRT',
  18,
  'JCC',
  0.20,
  0,
  0,
  0,
  '2026-10-15 12:00:00',
  '2026-11-15 12:00:00',
  '2026-12-01 12:00:00',
  'pre_launch',
  'Neuroscience · Deep Science',
  '',
  '',
  '2026-10-15 12:00:00',
  '<h2>The theory — two generations of GABA inhibitory-neuroreceptor science</h2>
<p><strong>1969 · Prof. Richard W. Olsen (UCLA)</strong> — the ''father of the GABA receptor''. Discovered that the brain is balanced by two systems (GABA inhibition and excitation); health = equilibrium. When inhibition fails, excitation releases neurotoxins → neuronal damage → transmission failure.</p>
<p><strong>2004 · Prof. Jing Liang</strong> — the guiding/support-protein (gephyrin) hypothesis. When brain gephyrin drops below 50% of baseline, GABA-A receptor function collapses. This is now recognized as the <em>fourth pathological hypothesis</em> of Alzheimer''s disease.</p>
<p><strong>2020 · Mitochondrial extension</strong> — damaged brain-cell mitochondria → ATP drop → gephyrin drop → GABA-A signal loss → neurodegeneration.</p>
<p><strong>2021 · Neuro-immune-deficiency hypothesis</strong> — over-activated microglia (stress, isolation, sleep loss) trigger neuro-inflammation.</p>

<h2>The compound — from 6,000 plant flavonoids to D-P</h2>
<p>From 2003 the Liang lab screened 6,000+ natural plant flavonoids. The <strong>D-Flavonoid group</strong> (mixture of natural small-molecule flavonoids) emerged as a positive GABA-A modulator: restores gephyrin, boosts mitochondrial ATP, crosses the blood-brain barrier, no dependency. Shipped as the FDA-registered Smarto-One supplement in 2018.</p>
<p>2020: medicinal-chemistry optimization on the natural scaffold produced <strong>D-P</strong> — a single molecule with 50× gains in stability and efficacy, and <strong>15-minute BBB penetration</strong>. D-P is the base compound for the drug pipeline (D-P1 for Alzheimer''s mid-early stage + BPSD; D-P2 for depression / mood disorders).</p>

<h2>Three product lines, one theory</h2>
<ol>
  <li><strong>Nutraceutical (FDA-registered) — Smarto-One / Carefreeing™ series.</strong> Live in USA + China cross-border e-commerce. 288-person volunteer study shows improvement in sleep / stress / mood. Target market: 25–40 y/o urban women; 3-year revenue plan ¥1.2B / ¥3.5B / ¥6B (2026–2028).</li>
  <li><strong>Class-II medical device — Smarto Light™ 40Hz.</strong> Non-invasive gamma-band multisensory (sound + light + vibration) stimulation. Based on MIT (Prof. Li-Huei Tsai) 40Hz gamma research. Two Class-II filings pending. Manufactured with Beijing Hehui Optoelectronics (ISO 9001, applying ISO 13485).</li>
  <li><strong>New drug — D-P1 / D-P2.</strong> D-P1: Alzheimer''s mid-early treatment + BPSD symptom relief. D-P2: mood disorders. Both have completed lab-stage data; awaiting Phase-I. IVD companion: 4 test kits (AD7C-NTP, Aβ1-42, P-Tau-181, Aβ42).</li>
</ol>',
  '<h2>Key evidence base</h2>

<h3>D-Flavonoid — human & animal evidence</h3>
<ul>
  <li>Enhances GABA-A receptor tonic and phasic current (Shen et al., <em>J. Neuroscience</em> 2012).</li>
  <li>Reduces Aβ plaque burden in transgenic AD mice (Liang et al., <em>J. Neurochem</em> 2014).</li>
  <li>Rescues anxiety, memory, motor function in AD-model rodents. Reverses stress-induced neuroinflammation via microglia normalization (Liang et al., <em>J. Neuroinflammation</em> 2022).</li>
  <li>288-person human observational study — reduces sleep disturbance, stress and mood problems (PLOS ONE 2022, Smarto-One).</li>
  <li>USC IRB clearance for human testing; over 20,000 real-world consumer studies (2018–now).</li>
</ul>

<h3>D-P — 2nd-generation optimized single molecule</h3>
<ul>
  <li>50× stability & efficacy gain vs D-Flavonoid natural scaffold.</li>
  <li>15-minute blood-brain-barrier penetration (small-molecule bioavailability solved).</li>
  <li>8-hour full-body clearance, no residue, no dependency, no withdrawal-syndrome risk.</li>
  <li>Lab-stage data + repeatability + stable manufacturing method — complete. Awaiting Phase-I.</li>
</ul>

<h3>40Hz gamma device — MIT clinical evidence</h3>
<ul>
  <li>Aβ40 clearance +28% (Nature — glymphatic mechanism).</li>
  <li>Tau phosphorylation −37% (Tau P301S model).</li>
  <li>MMSE cognitive decline slowed 83% (n = 76 double-blind).</li>
  <li>ADCS-ADL score decline slowed 84% (p &lt; 0.001).</li>
  <li>Default-mode-network connectivity +41%; synaptic-protein marker +68% at 6 weeks.</li>
  <li>vs oral donepezil (6 mo): MMSE +3.2 vs +1.5; side-effect &lt; 1% vs 28%; ¥80/day vs ¥220/day; adherence 98% vs 63%.</li>
</ul>

<h3>Market — supplement pipeline</h3>
<ul>
  <li>China neuro-health nutraceutical market: 2025 est. ¥180B, CAGR 15%+ (Frost & Sullivan).</li>
  <li>Mood disorders in CN: 132M patients, ¥17k/yr avg spend, ¥32.5B TAM.</li>
  <li>Sleep disorders in CN: 300M patients, ¥932/yr avg spend, ¥400B TAM.</li>
</ul>',
  '<h3>Three regulatory tracks — three risk profiles</h3>
<ul>
  <li><strong>Nutraceutical (Smarto-One)</strong> — already FDA-registered dietary supplement in USA + China food-quality certified. Lowest regulatory risk; revenue-generating. Risk is competitive (unverified competitor claims dominate the CN market — market-education cost).</li>
  <li><strong>Class-II medical device (Smarto Light™ 40Hz)</strong> — two Class-II filings in progress. Standard medical-device timeline (18–24 months); manufacturing partner (Beijing Hehui Optoelectronics) applying for ISO 13485. IP: MIT 40Hz methods (licensed) + proprietary tri-modal (sound / light / vibration) protocol + original 40Hz music compositions.</li>
  <li><strong>New drug (D-P1 / D-P2)</strong> — full FDA / NMPA drug pathway. 3–6 months IND prep; 6 months prior-research; 18 months preclinical; 3–6 months IND review; 1 year phased trials; 6–12 months NDA. Not first-in-class (natural D-Flavonoid parent scaffold has consumer safety track record), but full timeline is 4–5 years.</li>
</ul>

<h3>Financing structure</h3>
<ul>
  <li>Seed round: ¥45M RMB target, 20–25% equity release. Close deadline announced 2026-01-30.</li>
  <li>Use of funds: drug R&D + product R&D + cross-border-EC brand + online market flow + channel-build + production.</li>
  <li>Full financial model + additional strategic docs available only after NDA execution.</li>
  <li><strong>Phase-1 of this listing is subscription and content only</strong> — no on-chain sale, no purchase contract, no vesting is live. Phase 2 (contracts + trading) will be separately announced with independent risk disclosures.</li>
</ul>',
  '[{"name": "Prof. Jing Liang (梁京)", "role": "SCIENTIFIC LEAD", "bio": "USC School of Pharmacy, professor. Fellow of the Royal Society of Medicine (2019–). Named ''New Scientist'' by London (2012). Proposed the ''guiding / support protein — gephyrin'' hypothesis (2004), later recognized as the fourth pathological hypothesis of Alzheimer''s. 82 peer-reviewed papers; 2012 J. Neuroscience paper selected NIH ''Top 1000 papers influencing the world''. Chief investigator NIH Brain Challenge (2008). Editor-in-Chief, Chinese Pharmacology journal. Multiple technology invention patents.", "avatar_url": ""}, {"name": "Prof. Richard W. Olsen (奥森・理查德, in memoriam)", "role": "FOUNDING THEORY", "bio": "UCLA School of Medicine, lifetime distinguished professor. ''Father of the GABA receptor''. Nobel Committee lifetime-achievement award (Neuroscience). NIH career development award; Javits Neuroscience Prize; US HHS cardiovascular achievement award. 266 SCI papers; editor of Pharmacology, Neuroscience, Epilepsia, Alcoholism.", "avatar_url": ""}, {"name": "Prof. Rong Wang (王蓉)", "role": "CN LAB LEAD", "bio": "Director, Central Laboratory, Xuanwu Hospital (Capital Medical University). Chair, AD Peripheral-Biomarker & Early-Intervention Alliance (National Old-Age Disease Clinical Research Center). Chair, Basic & Translational Medicine Committee, Beijing Neurology Association. Neurofilament AD7c-NTP urine detection inventor. 32 first-author + 92 corresponding-author papers, 37 SCI.", "avatar_url": ""}, {"name": "Prof. Shun Yu (于顺)", "role": "CN LAB · PARKINSON''S", "bio": "Xuanwu Hospital, National Aging-Disease Clinical Research Center. Director, Molecular Diagnostics Lab, Beijing Aging Medicine Research Center. Deputy director, Parkinson''s Research Institute (Capital Medical University Brain Research Institute).", "avatar_url": ""}, {"name": "Prof. Yu Liu (刘鱼)", "role": "CN LAB · TRANSFUSION MEDICINE", "bio": "Peking Union Medical College, professor. Johns Hopkins visiting scholar. Director, Transfusion Medicine Experimental Center, Chinese Academy of Medical Sciences.", "avatar_url": ""}, {"name": "Fan Zhao (赵凡)", "role": "CMO — COMMERCE LEAD", "bio": "Former CMO / Fast-Moving Consumer Head, ByteDance Douyin Marketing. Operated ¥500B GMV, ¥7B annual budget, 250-person team. Leading B-side ecosystem + C-side brand build + cross-border e-commerce channels.", "avatar_url": ""}]',
  '{"tge_percent": 5, "cliff_months": 6, "linear_months": 18, "allocations": [{"label": "Public sale (Phase 2)", "percent": 22, "color": "#74f5a1"}, {"label": "R&D — D-P new drug", "percent": 22, "color": "#3884ff"}, {"label": "Device + IVD build-out", "percent": 12, "color": "#a55dff"}, {"label": "Team + advisors", "percent": 15, "color": "#fa8c16"}, {"label": "Cross-border commerce", "percent": 10, "color": "#f5222d"}, {"label": "Foundation treasury", "percent": 10, "color": "#212121"}, {"label": "Liquidity", "percent": 9, "color": "#1aa64a"}]}',
  '[{"quarter": "2003–2018", "title": "D-Flavonoid discovery → Smarto-One shipped", "desc": "6,000+ flavonoid screen → D-Flavonoid group → FDA supplement registration → China + USA channels + 20,000-user studies.", "status": "done"}, {"quarter": "2020", "title": "D-P single molecule complete", "desc": "Structure-optimized single-compound with 50× stability / efficacy gain and 15-min BBB penetration. Lab-stage + repeatability + manufacturing method complete.", "status": "done"}, {"quarter": "2026 Q3–Q4", "title": "Pre-launch subscription + content live", "desc": "This page. Notify-me + watchlist + updates. No on-chain sale yet.", "status": "active"}, {"quarter": "2026 Q4", "title": "Seed round close (¥45M target, 20–25% eq.)", "desc": "Deadline 2026-01-30. Use-of-funds gates drug R&D + device build-out + channel expansion.", "status": "todo"}, {"quarter": "2027", "title": "D-P1 IND (Alzheimer''s mid-early + BPSD)", "desc": "18 months preclinical → IND submission. Parallel: cross-border e-commerce ¥3.5B revenue plan (channel matrix + celebrity live-commerce).", "status": "todo"}, {"quarter": "2027", "title": "Smarto Light™ 40Hz — Class-II approval", "desc": "Two Class-II filings expected. Home-eyemask + hospital sleep-pod SKUs. ISO 13485 finalization.", "status": "todo"}, {"quarter": "2028", "title": "D-P2 IND (mood disorders / depression)", "desc": "6 months preclinical after CMC mid-stage. IVD 4-panel kit companion launches for AD early-screening.", "status": "todo"}, {"quarter": "2028", "title": "Commerce Y3 · ¥6B revenue plan", "desc": "Douyin store matrix + Tmall / JD + Sam''s Club + 100 offline retail. Gross margin 82% target. Investor exit via IPO / M&A (comps: Chengcyan-Life Space acquisition PS 5–8×).", "status": "todo"}]',
  '[{"title": "US FDA · dietary supplement registration (Carefreeing™ / Smarto-One)", "kind": "REGULATORY", "url": ""}, {"title": "China food-quality inspection certificate", "kind": "REGULATORY", "url": ""}, {"title": "PONY test · oral acute-toxicity certification", "kind": "SAFETY", "url": ""}, {"title": "D-Flavonoid — 18 selected peer-reviewed papers (J. Neurochem, PNAS, J. Neuroscience, PLOS ONE, Nature 2019)", "kind": "PAPERS", "url": ""}, {"title": "MIT 40Hz gamma — Nature glymphatic + NCT-03556280 Phase-II reference", "kind": "PAPERS", "url": "https://www.nature.com/articles/s41586-019-1330-0"}, {"title": "Smarto-One PLOS ONE 2022 — 288-user stress / anxiety / sleep study", "kind": "PAPERS", "url": ""}, {"title": "Chengcyan-Life Space acquisition (PS 5–8× exit comp reference)", "kind": "EXIT COMP", "url": ""}]',
  '[{"q": "What is the difference between D-Flavonoid (Smarto-One) and D-P new drug?", "a": "D-Flavonoid is a natural mixture of plant flavonoids, shipping since 2018 as an FDA-registered dietary supplement. D-P is a single medicinal-chemistry-optimized molecule derived from the same scaffold — 50× stability / efficacy gain, 15-min BBB penetration, and is now entering the full drug pathway (Alzheimer''s D-P1, depression D-P2)."}, {"q": "Why three product lines simultaneously?", "a": "One theory (GABA-A / gephyrin restoration), three regulatory tracks. The nutraceutical is already-cashflow, funding R&D. The Class-II device pays back in 18–24 months. The new drug is the long-horizon high-value asset. Cross-subsidy across product lines."}, {"q": "Isn''t Phase-1 just a subscription page — what am I actually joining?", "a": "Phase 1 = notify-me subscription + team-published Updates + watchlist + (in warmup) whitelist. Nothing on-chain sells here. Phase 2 (contracts + purchase + vesting) will be a separately-announced event with its own risk disclosures."}, {"q": "When does the seed round close?", "a": "Seed round is ¥45M RMB, 20–25% equity release. Announced deadline: 2026-01-30. Detailed financial-model + additional strategic docs available only after NDA execution."}]',
  '[{"label": "SMRT token (post-TGE)", "address": "", "chain": "BSC"}, {"label": "Vesting contract", "address": "", "chain": "BSC"}, {"label": "Public-sale contract", "address": "", "chain": "BSC"}, {"label": "Foundation treasury multi-sig", "address": "", "chain": "BSC"}]'
) ON DUPLICATE KEY UPDATE
  `owner_address`=VALUES(`owner_address`),
  `name_en`=VALUES(`name_en`),
  `name_cn`=VALUES(`name_cn`),
  `tagline`=VALUES(`tagline`),
  `description`=VALUES(`description`),
  `tags`=VALUES(`tags`),
  `team_info`=VALUES(`team_info`),
  `roadmap`=VALUES(`roadmap`),
  `partners`=VALUES(`partners`),
  `token_address`=VALUES(`token_address`),
  `token_symbol`=VALUES(`token_symbol`),
  `token_decimals`=VALUES(`token_decimals`),
  `payment_token`=VALUES(`payment_token`),
  `token_price`=VALUES(`token_price`),
  `hard_cap`=VALUES(`hard_cap`),
  `soft_cap`=VALUES(`soft_cap`),
  `max_per_user`=VALUES(`max_per_user`),
  `start_time`=VALUES(`start_time`),
  `end_time`=VALUES(`end_time`),
  `tge_time`=VALUES(`tge_time`),
  `status`=VALUES(`status`),
  `category`=VALUES(`category`),
  `cover_url`=VALUES(`cover_url`),
  `cover_video_url`=VALUES(`cover_video_url`),
  `launch_at`=VALUES(`launch_at`),
  `story_html`=VALUES(`story_html`),
  `research_html`=VALUES(`research_html`),
  `risks_html`=VALUES(`risks_html`),
  `team_json`=VALUES(`team_json`),
  `tokenomics_json`=VALUES(`tokenomics_json`),
  `roadmap_json`=VALUES(`roadmap_json`),
  `documents_json`=VALUES(`documents_json`),
  `faq_json`=VALUES(`faq_json`),
  `onchain_json`=VALUES(`onchain_json`);


-- END 2026_07_04_seed_projects.sql

-- BEGIN 2026_07_05_user_fk_cascade.sql
-- Sprint · 修根因：删除 user 时联动清关联表
-- 事件：手动 DELETE FROM user 时残留了 invite_user_code / user_point 等表的孤儿数据，
-- 导致新用户误用旧邀请码时 invite_record.inviter_uid 指向已删 uid，邀请奖励发到孤儿账号。
--
-- 修复策略：因 user.id (bigint unsigned) 与 uid (int) 列类型不兼容无法加 FK CASCADE，
-- 改在 backend admin service.DeleteUser 里用 SQL Transaction 联动清理（见 pkg/core/admin/service/user）。
-- 本迁移只做 **孤儿数据清理**，把历史遗留干净。

SET NAMES utf8mb4;

DELETE FROM user_point           WHERE uid NOT IN (SELECT id FROM user);
DELETE FROM user_point_flow      WHERE uid NOT IN (SELECT id FROM user);
DELETE FROM user_check_in_record WHERE uid NOT IN (SELECT id FROM user);
DELETE FROM task_record          WHERE uid NOT IN (SELECT id FROM user);
DELETE FROM invite_user_code     WHERE uid NOT IN (SELECT id FROM user);
DELETE FROM invite_record        WHERE uid NOT IN (SELECT id FROM user);
UPDATE invite_record SET inviter_uid = 0 WHERE inviter_uid > 0 AND inviter_uid NOT IN (SELECT id FROM user);

-- END 2026_07_05_user_fk_cascade.sql

-- BEGIN 2026_07_23_important_news_alert.sql
-- ============================================================
-- Important news spotlight settings
--   News remains the source of truth for title/summary/cover/content.
--   Alert settings only control when and where the spotlight appears.
-- ============================================================
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_add_news_alert_col;
DELIMITER $$
CREATE PROCEDURE jc_add_news_alert_col(
  IN p_column VARCHAR(64),
  IN p_definition TEXT
)
BEGIN
  IF NOT EXISTS (
    SELECT 1
      FROM information_schema.columns
     WHERE table_schema = DATABASE()
       AND table_name = 'news'
       AND column_name = p_column
  ) THEN
    SET @ddl = CONCAT('ALTER TABLE `news` ADD COLUMN `', p_column, '` ', p_definition);
    PREPARE stmt FROM @ddl;
    EXECUTE stmt;
    DEALLOCATE PREPARE stmt;
  END IF;
END$$
DELIMITER ;

CALL jc_add_news_alert_col('is_important', 'TINYINT(1) NOT NULL DEFAULT 0 COMMENT ''show as important-news spotlight''');
CALL jc_add_news_alert_col('alert_priority', 'INT NOT NULL DEFAULT 0 COMMENT ''higher value wins when alerts overlap''');
CALL jc_add_news_alert_col('alert_start_at', 'DATETIME NULL DEFAULT NULL COMMENT ''spotlight schedule start''');
CALL jc_add_news_alert_col('alert_end_at', 'DATETIME NULL DEFAULT NULL COMMENT ''spotlight schedule end''');
CALL jc_add_news_alert_col('alert_frequency', 'VARCHAR(16) NOT NULL DEFAULT ''once'' COMMENT ''once / daily / session''');
CALL jc_add_news_alert_col('alert_pages', 'VARCHAR(255) NOT NULL DEFAULT ''home,ecosystem,launchpad'' COMMENT ''comma-separated route groups''');
CALL jc_add_news_alert_col('alert_delay_seconds', 'INT NOT NULL DEFAULT 3 COMMENT ''delay after page is ready, 0-15''');
CALL jc_add_news_alert_col('alert_revision', 'INT NOT NULL DEFAULT 1 COMMENT ''increment to show the alert again''');

DROP PROCEDURE jc_add_news_alert_col;

DROP PROCEDURE IF EXISTS jc_add_news_alert_idx;
DELIMITER $$
CREATE PROCEDURE jc_add_news_alert_idx()
BEGIN
  IF NOT EXISTS (
    SELECT 1
      FROM information_schema.statistics
     WHERE table_schema = DATABASE()
       AND table_name = 'news'
       AND index_name = 'idx_news_alert_active'
  ) THEN
    ALTER TABLE `news`
      ADD INDEX `idx_news_alert_active`
      (`is_important`, `status`, `alert_priority`, `publish_at`);
  END IF;
END$$
DELIMITER ;

CALL jc_add_news_alert_idx();
DROP PROCEDURE jc_add_news_alert_idx;

-- END 2026_07_23_important_news_alert.sql

-- BEGIN 2026_07_28_task_rounds.sql
-- X task rounds: changing a post target starts a new independently claimable
-- round while preserving previous completion and point-flow history.

SET @exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task' AND COLUMN_NAME = 'version'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE task ADD COLUMN version INT UNSIGNED NOT NULL DEFAULT 1 AFTER is_enable',
  'SELECT ''task.version already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task' AND COLUMN_NAME = 'target_ref'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE task ADD COLUMN target_ref VARCHAR(200) NOT NULL DEFAULT '''' AFTER version',
  'SELECT ''task.target_ref already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @round_started_was_missing := (
  SELECT IF(COUNT(*) = 0, 1, 0) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task' AND COLUMN_NAME = 'round_started_at'
);
SET @sql := IF(@round_started_was_missing = 1,
  'ALTER TABLE task ADD COLUMN round_started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER target_ref',
  'SELECT ''task.round_started_at already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql := IF(@round_started_was_missing = 1,
  'UPDATE task SET round_started_at = created_at',
  'SELECT ''task.round_started_at retained'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task_record' AND COLUMN_NAME = 'task_version'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE task_record ADD COLUMN task_version INT UNSIGNED NOT NULL DEFAULT 1 AFTER platform',
  'SELECT ''task_record.task_version already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task_record' AND COLUMN_NAME = 'target_ref'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE task_record ADD COLUMN target_ref VARCHAR(200) NOT NULL DEFAULT '''' AFTER verify_status',
  'SELECT ''task_record.target_ref already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task_record' AND COLUMN_NAME = 'target_url'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE task_record ADD COLUMN target_url VARCHAR(500) NOT NULL DEFAULT '''' AFTER target_ref',
  'SELECT ''task_record.target_url already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Replace every historical one-task-per-user index variant with the
-- round-aware key. task_id is globally unique, so platform is audit data and
-- does not need to participate in uniqueness.
SET @exists := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task_record' AND INDEX_NAME = 'uk_task_uid'
);
SET @sql := IF(@exists > 0,
  'ALTER TABLE task_record DROP INDEX uk_task_uid',
  'SELECT ''uk_task_uid not present'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task_record' AND INDEX_NAME = 'uk_platform_task_uid'
);
SET @sql := IF(@exists > 0,
  'ALTER TABLE task_record DROP INDEX uk_platform_task_uid',
  'SELECT ''uk_platform_task_uid not present'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task_record' AND INDEX_NAME = 'uk_platform_task_version_uid'
);
SET @sql := IF(@exists > 0,
  'ALTER TABLE task_record DROP INDEX uk_platform_task_version_uid',
  'SELECT ''uk_platform_task_version_uid not present'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'task_record' AND INDEX_NAME = 'uk_task_version_uid'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE task_record ADD UNIQUE INDEX uk_task_version_uid (task_id, task_version, uid)',
  'SELECT ''uk_task_version_uid already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `task_round` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `task_id` BIGINT UNSIGNED NOT NULL,
  `version` INT UNSIGNED NOT NULL DEFAULT 1,
  `target_ref` VARCHAR(200) NOT NULL,
  `target_url` VARCHAR(500) NOT NULL DEFAULT '',
  `amount_point` INT NOT NULL DEFAULT 0,
  `started_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE INDEX `uk_task_round` (`task_id`, `version`),
  UNIQUE INDEX `uk_task_target` (`task_id`, `target_ref`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Post-bound X task round history';

-- Normalize the currently configured post for existing tasks. Host changes,
-- query strings and trailing path fragments do not change the extracted ID.
UPDATE task
SET target_ref = SUBSTRING_INDEX(
  SUBSTRING_INDEX(SUBSTRING_INDEX(url, '/status/', -1), '?', 1), '/', 1
)
WHERE platform = 'twitter'
  AND task_key IN ('like', 'retweet', 'reply', 'bookmark')
  AND target_ref = ''
  AND url LIKE '%/status/%';

INSERT INTO task_round
  (task_id, version, target_ref, target_url, amount_point, started_at, created_at)
SELECT id, version, target_ref, url, amount_point, round_started_at, round_started_at
FROM task
WHERE platform = 'twitter'
  AND task_key IN ('like', 'retweet', 'reply', 'bookmark')
  AND target_ref <> ''
ON DUPLICATE KEY UPDATE task_id = VALUES(task_id);

-- All pre-migration records belong to round 1. Snapshot the target that was
-- current when this feature was introduced; future writes store snapshots in
-- application code.
UPDATE task_record r
JOIN task t ON t.id = r.task_id
SET r.task_version = 1,
    r.target_ref = IF(r.target_ref = '', t.target_ref, r.target_ref),
    r.target_url = IF(r.target_url = '', t.url, r.target_url)
WHERE r.task_version = 1;

-- END 2026_07_28_task_rounds.sql

-- BEGIN 2026_08_13_ido_event_reliability.sql
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

-- END 2026_08_13_ido_event_reliability.sql

-- BEGIN 2026_08_14_chain_event_recovery.sql
-- JoinCare IDO Sprint 1A-P0 · event recovery and observability
-- Adds durable retry/dead-letter/replay records, sync health checkpoints and block hashes.

SET NAMES utf8mb4;

SET @has_block_hash = (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_record' AND COLUMN_NAME = 'block_hash'
);
SET @add_block_hash_sql = IF(
  @has_block_hash = 0,
  'ALTER TABLE chain_event_record ADD COLUMN block_hash VARCHAR(66) NOT NULL DEFAULT '''' AFTER block_number',
  'SELECT 1'
);
PREPARE stmt FROM @add_block_hash_sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `chain_event_failure` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `event` VARCHAR(50) NOT NULL DEFAULT '',
  `block_number` BIGINT UNSIGNED NOT NULL,
  `block_hash` VARCHAR(66) NOT NULL DEFAULT '',
  `tx_hash` VARCHAR(100) NOT NULL,
  `log_index` INT UNSIGNED NOT NULL,
  `raw_log` LONGTEXT NOT NULL,
  `error_message` TEXT NOT NULL,
  `status` VARCHAR(20) NOT NULL,
  `retry_count` INT UNSIGNED NOT NULL DEFAULT 0,
  `consecutive_count` INT UNSIGNED NOT NULL DEFAULT 0,
  `replay_count` INT UNSIGNED NOT NULL DEFAULT 0,
  `first_failed_at` DATETIME(6) NOT NULL,
  `last_failed_at` DATETIME(6) NOT NULL,
  `replay_requested_at` DATETIME(6) NULL,
  `resolved_at` DATETIME(6) NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_chain_event_failure_identity` (`block_number`, `tx_hash`, `log_index`),
  KEY `idx_chain_event_failure_status` (`status`),
  KEY `idx_chain_event_failure_last_failed` (`last_failed_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `chain_event_sync_state` (
  `id` BIGINT UNSIGNED NOT NULL,
  `initialized` TINYINT(1) NOT NULL DEFAULT 0,
  `cursor_block` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `cursor_block_hash` VARCHAR(66) NOT NULL DEFAULT '',
  `lease_token` VARCHAR(64) NOT NULL DEFAULT '',
  `latest_block` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `safe_block` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `lag_blocks` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `last_started_at` DATETIME(6) NULL,
  `last_success_at` DATETIME(6) NULL,
  `last_failure_at` DATETIME(6) NULL,
  `last_error` TEXT NOT NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO chain_event_sync_state (
  id, cursor_block, cursor_block_hash, latest_block, safe_block, lag_blocks,
  last_error, created_at, updated_at
) VALUES (1, 0, '', 0, 0, 0, '', CURRENT_TIMESTAMP(6), CURRENT_TIMESTAMP(6))
ON DUPLICATE KEY UPDATE id = VALUES(id);

DROP PROCEDURE IF EXISTS jc_assert_chain_event_recovery_schema;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_recovery_schema()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_failure'
      AND COLUMN_NAME IN (
        'id','event','block_number','block_hash','tx_hash','log_index','raw_log','error_message',
        'status','retry_count','consecutive_count','replay_count','first_failed_at','last_failed_at',
        'replay_requested_at','resolved_at','created_at','updated_at'
      )
  ) <> 18 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'chain_event_failure schema is incomplete';
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM (
      SELECT INDEX_NAME,
             GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') AS columns_in_order,
             MIN(NON_UNIQUE) AS non_unique
      FROM information_schema.STATISTICS
      WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_failure'
      GROUP BY INDEX_NAME
    ) indexes_for_failure
    WHERE columns_in_order = 'block_number,tx_hash,log_index' AND non_unique = 0
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'chain_event_failure unique identity is missing';
  END IF;

  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_sync_state'
      AND COLUMN_NAME IN (
        'id','initialized','cursor_block','cursor_block_hash','lease_token','latest_block','safe_block',
        'lag_blocks','last_started_at','last_success_at','last_failure_at','last_error','created_at','updated_at'
      )
  ) <> 14 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'chain_event_sync_state schema is incomplete';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_recovery_schema();
DROP PROCEDURE jc_assert_chain_event_recovery_schema;

-- Verification:
--   SHOW CREATE TABLE chain_event_failure;
--   SHOW CREATE TABLE chain_event_sync_state;
--   SHOW COLUMNS FROM chain_event_record LIKE 'block_hash';

-- END 2026_08_14_chain_event_recovery.sql

-- BEGIN 2026_08_15_ido_v2_fund_ledger.sql
-- JoinCare IDO Sprint 1A-P2 · V2 project identity and append-only raw-unit fund ledger.
-- This migration does not assign stablecoin addresses to legacy JCC rows and does not start V2 event consumption.

SET NAMES utf8mb4;

-- Fail before changing business tables when the pre-existing schema is ambiguous.
DROP PROCEDURE IF EXISTS jc_preflight_ido_v2_schema;
DELIMITER $$
CREATE PROCEDURE jc_preflight_ido_v2_schema()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'projects' AND TABLE_TYPE = 'BASE TABLE'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'projects table is missing';
  END IF;

  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'projects'
      AND (
        (COLUMN_NAME = 'payment_token' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH >= 20 AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'token_address' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH >= 42 AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'token_decimals' AND DATA_TYPE IN ('tinyint','smallint','mediumint','int','bigint') AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'status' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH >= 20 AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME IN ('token_price','hard_cap','soft_cap','max_per_user','total_raised','total_deposited')
          AND DATA_TYPE = 'decimal' AND NUMERIC_SCALE <= 18
          AND NUMERIC_PRECISION - NUMERIC_SCALE <= 47 AND IS_NULLABLE = 'NO')
      )
  ) <> 10 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'projects base IDO column definitions are incompatible';
  END IF;

  SET @existing_v2_column_count = (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'projects'
      AND COLUMN_NAME IN (
        'payment_token_address','payment_token_decimals','chain_id','contract_address',
        'contract_version','onchain_locked','token_price_raw','hard_cap_raw','soft_cap_raw',
        'max_per_user_raw','total_raised_raw','total_deposited_raw'
      )
  );
  IF @existing_v2_column_count NOT IN (0, 12) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'partial projects IDO V2 schema requires manual recovery';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'fund_ledger'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'pre-existing fund_ledger requires manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_ido_v2_schema();
DROP PROCEDURE jc_preflight_ido_v2_schema;

-- Refuse ambiguous historical data before any auto-committing DDL is attempted.
DROP PROCEDURE IF EXISTS jc_assert_known_payment_token_values;
DELIMITER $$
CREATE PROCEDURE jc_assert_known_payment_token_values()
BEGIN
  IF EXISTS (
    SELECT 1 FROM `projects`
    WHERE UPPER(TRIM(`payment_token`)) NOT IN ('', 'JCC', 'JC', 'USDT', 'USDC')
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'unknown historical payment_token values require manual classification';
  END IF;
  IF EXISTS (
    SELECT 1 FROM `projects`
    WHERE `status` NOT IN ('Setup', 'Set', 'pre_launch', 'warmup')
      AND UPPER(TRIM(`payment_token`)) = ''
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'non-draft rows without a settlement asset require manual classification';
  END IF;

  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'projects'
      AND COLUMN_NAME IN (
        'payment_token_address','payment_token_decimals','chain_id','contract_address',
        'contract_version','onchain_locked','token_price_raw','hard_cap_raw','soft_cap_raw',
        'max_per_user_raw','total_raised_raw','total_deposited_raw'
      )
  ) = 12 THEN
    SET @ambiguous_v2_count = 0;
    SET @check_v2_sql =
      'SELECT COUNT(*) INTO @ambiguous_v2_count FROM `projects` WHERE UPPER(TRIM(`payment_token`)) IN (''USDT'',''USDC'') AND `contract_version` <> ''v2_stablecoin''';
    PREPARE check_v2_stmt FROM @check_v2_sql;
    EXECUTE check_v2_stmt;
    DEALLOCATE PREPARE check_v2_stmt;
    IF @ambiguous_v2_count > 0 THEN
      SIGNAL SQLSTATE '45000'
        SET MESSAGE_TEXT = 'historical USDT/USDC rows require explicit V2 identity classification';
    END IF;
  ELSEIF EXISTS (
    SELECT 1 FROM `projects`
    WHERE UPPER(TRIM(`payment_token`)) IN ('USDT', 'USDC')
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'historical USDT/USDC rows require explicit V2 identity classification';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_known_payment_token_values();
DROP PROCEDURE jc_assert_known_payment_token_values;

DROP PROCEDURE IF EXISTS jc_add_projects_column;
DELIMITER $$
CREATE PROCEDURE jc_add_projects_column(
  IN p_column_name VARCHAR(64),
  IN p_column_definition TEXT,
  IN p_expected_column_type VARCHAR(64),
  IN p_expected_default VARCHAR(64)
)
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'projects' AND COLUMN_NAME = p_column_name
  ) THEN
    IF NOT EXISTS (
      SELECT 1 FROM information_schema.COLUMNS
      WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'projects'
        AND COLUMN_NAME = p_column_name
        AND COLUMN_TYPE = p_expected_column_type
        AND IS_NULLABLE = 'NO'
        AND COLUMN_DEFAULT = p_expected_default
    ) THEN
      SIGNAL SQLSTATE '45000'
        SET MESSAGE_TEXT = 'existing projects IDO V2 column definition is incompatible';
    END IF;
  ELSE
    SET @add_column_sql = CONCAT(
      'ALTER TABLE `projects` ADD COLUMN `', REPLACE(p_column_name, '`', '``'), '` ', p_column_definition
    );
    PREPARE stmt FROM @add_column_sql;
    EXECUTE stmt;
    DEALLOCATE PREPARE stmt;
  END IF;
END$$
DELIMITER ;

CALL jc_add_projects_column('payment_token_address', 'VARCHAR(42) NOT NULL DEFAULT '''' AFTER `payment_token`', 'varchar(42)', '');
CALL jc_add_projects_column('payment_token_decimals', 'TINYINT UNSIGNED NOT NULL DEFAULT 0 AFTER `payment_token_address`', 'tinyint unsigned', '0');
CALL jc_add_projects_column('chain_id', 'BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER `payment_token_decimals`', 'bigint unsigned', '0');
CALL jc_add_projects_column('contract_address', 'VARCHAR(42) NOT NULL DEFAULT '''' AFTER `chain_id`', 'varchar(42)', '');
CALL jc_add_projects_column('contract_version', 'VARCHAR(32) NOT NULL DEFAULT ''unassigned'' AFTER `contract_address`', 'varchar(32)', 'unassigned');
CALL jc_add_projects_column('onchain_locked', 'TINYINT(1) NOT NULL DEFAULT 0 AFTER `contract_version`', 'tinyint(1)', '0');
CALL jc_add_projects_column('token_price_raw', 'VARCHAR(78) NOT NULL DEFAULT ''0'' AFTER `token_price`', 'varchar(78)', '0');
CALL jc_add_projects_column('hard_cap_raw', 'VARCHAR(78) NOT NULL DEFAULT ''0'' AFTER `hard_cap`', 'varchar(78)', '0');
CALL jc_add_projects_column('soft_cap_raw', 'VARCHAR(78) NOT NULL DEFAULT ''0'' AFTER `soft_cap`', 'varchar(78)', '0');
CALL jc_add_projects_column('max_per_user_raw', 'VARCHAR(78) NOT NULL DEFAULT ''0'' AFTER `max_per_user`', 'varchar(78)', '0');
CALL jc_add_projects_column('total_raised_raw', 'VARCHAR(78) NOT NULL DEFAULT ''0'' AFTER `total_raised`', 'varchar(78)', '0');
CALL jc_add_projects_column('total_deposited_raw', 'VARCHAR(78) NOT NULL DEFAULT ''0'' AFTER `total_deposited`', 'varchar(78)', '0');
DROP PROCEDURE jc_add_projects_column;

DROP PROCEDURE IF EXISTS jc_assert_unambiguous_v2_history;
DELIMITER $$
CREATE PROCEDURE jc_assert_unambiguous_v2_history()
BEGIN
  IF EXISTS (
    SELECT 1 FROM `projects`
    WHERE UPPER(TRIM(`payment_token`)) IN ('USDT', 'USDC')
      AND (
        `contract_version` IS NULL OR `contract_version` <> 'v2_stablecoin'
        OR `chain_id` IS NULL OR `chain_id` = 0
        OR `payment_token_decimals` IS NULL OR `payment_token_decimals` NOT BETWEEN 1 AND 18
        OR `token_decimals` IS NULL OR `token_decimals` NOT BETWEEN 1 AND 18
        OR `payment_token_address` IS NULL OR `payment_token_address` NOT REGEXP '^0x[0-9A-Fa-f]{40}$'
        OR `payment_token_address` = '0x0000000000000000000000000000000000000000'
        OR `contract_address` IS NULL OR `contract_address` NOT REGEXP '^0x[0-9A-Fa-f]{40}$'
        OR `contract_address` = '0x0000000000000000000000000000000000000000'
        OR `token_address` IS NULL OR `token_address` NOT REGEXP '^0x[0-9A-Fa-f]{40}$'
        OR `token_address` = '0x0000000000000000000000000000000000000000'
      )
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'historical USDT/USDC rows require explicit V2 identity classification';
  END IF;

  IF EXISTS (
    SELECT 1 FROM `projects`
    WHERE UPPER(TRIM(`payment_token`)) IN ('USDT', 'USDC')
      AND (
        `token_price_raw` IS NULL OR `token_price_raw` NOT REGEXP '^(0|[1-9][0-9]{0,64})$'
        OR `hard_cap_raw` IS NULL OR `hard_cap_raw` NOT REGEXP '^(0|[1-9][0-9]{0,64})$'
        OR `soft_cap_raw` IS NULL OR `soft_cap_raw` NOT REGEXP '^(0|[1-9][0-9]{0,64})$'
        OR `max_per_user_raw` IS NULL OR `max_per_user_raw` NOT REGEXP '^(0|[1-9][0-9]{0,64})$'
        OR `total_raised_raw` IS NULL OR `total_raised_raw` NOT REGEXP '^(0|[1-9][0-9]{0,64})$'
        OR `total_deposited_raw` IS NULL OR `total_deposited_raw` NOT REGEXP '^(0|[1-9][0-9]{0,64})$'
      )
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'historical V2 raw amounts are missing or non-canonical';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM (
      SELECT p.*,
        CASE `payment_token_decimals`
          WHEN 1 THEN 10 WHEN 2 THEN 100 WHEN 3 THEN 1000 WHEN 4 THEN 10000
          WHEN 5 THEN 100000 WHEN 6 THEN 1000000 WHEN 7 THEN 10000000
          WHEN 8 THEN 100000000 WHEN 9 THEN 1000000000 WHEN 10 THEN 10000000000
          WHEN 11 THEN 100000000000 WHEN 12 THEN 1000000000000
          WHEN 13 THEN 10000000000000 WHEN 14 THEN 100000000000000
          WHEN 15 THEN 1000000000000000 WHEN 16 THEN 10000000000000000
          WHEN 17 THEN 100000000000000000 ELSE 1000000000000000000
        END AS payment_scale,
        CASE `token_decimals`
          WHEN 1 THEN 10 WHEN 2 THEN 100 WHEN 3 THEN 1000 WHEN 4 THEN 10000
          WHEN 5 THEN 100000 WHEN 6 THEN 1000000 WHEN 7 THEN 10000000
          WHEN 8 THEN 100000000 WHEN 9 THEN 1000000000 WHEN 10 THEN 10000000000
          WHEN 11 THEN 100000000000 WHEN 12 THEN 1000000000000
          WHEN 13 THEN 10000000000000 WHEN 14 THEN 100000000000000
          WHEN 15 THEN 1000000000000000 WHEN 16 THEN 10000000000000000
          WHEN 17 THEN 100000000000000000 ELSE 1000000000000000000
        END AS project_scale
      FROM `projects` p
      WHERE UPPER(TRIM(`payment_token`)) IN ('USDT', 'USDC')
    ) v2
    WHERE
      `status` IS NULL
      OR (`status` NOT IN ('Setup', 'Set', 'pre_launch', 'warmup') AND (`onchain_locked` IS NULL OR `onchain_locked` <> 1))
      OR `token_price` * payment_scale <> FLOOR(`token_price` * payment_scale)
      OR `hard_cap` * payment_scale <> FLOOR(`hard_cap` * payment_scale)
      OR `soft_cap` * payment_scale <> FLOOR(`soft_cap` * payment_scale)
      OR `max_per_user` * payment_scale <> FLOOR(`max_per_user` * payment_scale)
      OR `total_raised` * payment_scale <> FLOOR(`total_raised` * payment_scale)
      OR `total_deposited` * project_scale <> FLOOR(`total_deposited` * project_scale)
      OR CAST(`token_price_raw` AS DECIMAL(65,0)) <> `token_price` * payment_scale
      OR CAST(`hard_cap_raw` AS DECIMAL(65,0)) <> `hard_cap` * payment_scale
      OR CAST(`soft_cap_raw` AS DECIMAL(65,0)) <> `soft_cap` * payment_scale
      OR CAST(`max_per_user_raw` AS DECIMAL(65,0)) <> `max_per_user` * payment_scale
      OR CAST(`total_raised_raw` AS DECIMAL(65,0)) <> `total_raised` * payment_scale
      OR CAST(`total_deposited_raw` AS DECIMAL(65,0)) <> `total_deposited` * project_scale
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'historical V2 identity lock or raw/display amounts are inconsistent';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_unambiguous_v2_history();
DROP PROCEDURE jc_assert_unambiguous_v2_history;

-- Remove the unsafe JCC default. Existing values remain for classification below.
ALTER TABLE `projects`
  MODIFY COLUMN `payment_token` VARCHAR(20) NOT NULL DEFAULT '' COMMENT '受信支付币 symbol；V2 仅 USDT/USDC',
  MODIFY COLUMN `token_price` DECIMAL(65,18) NOT NULL DEFAULT 0 COMMENT '展示值；raw 字段为链上事实',
  MODIFY COLUMN `hard_cap` DECIMAL(65,18) NOT NULL DEFAULT 0 COMMENT '展示值；raw 字段为链上事实',
  MODIFY COLUMN `soft_cap` DECIMAL(65,18) NOT NULL DEFAULT 0 COMMENT '展示值；raw 字段为链上事实',
  MODIFY COLUMN `max_per_user` DECIMAL(65,18) NOT NULL DEFAULT 0 COMMENT '展示值；raw 字段为链上事实',
  MODIFY COLUMN `total_raised` DECIMAL(65,18) NOT NULL DEFAULT 0 COMMENT '展示聚合；raw 字段为链上事实',
  MODIFY COLUMN `total_deposited` DECIMAL(65,18) NOT NULL DEFAULT 0 COMMENT '展示聚合；raw 字段为链上事实';

-- Content/setup rows have never been valid on-chain IDO facts. Force an explicit stablecoin choice later.
UPDATE `projects`
SET `payment_token` = '',
    `payment_token_address` = '',
    `payment_token_decimals` = 0,
    `chain_id` = 0,
    `contract_address` = '',
    `contract_version` = 'unassigned',
    `onchain_locked` = 0
WHERE `status` IN ('Setup', 'Set', 'pre_launch', 'warmup')
  AND UPPER(TRIM(`payment_token`)) IN ('', 'JCC', 'JC');

-- Non-draft historical rows are marked legacy without inventing chain/contract/token addresses.
UPDATE `projects`
SET `payment_token` = 'JCC',
    `payment_token_address` = '',
    `payment_token_decimals` = 18,
    `chain_id` = 0,
    `contract_address` = '',
    `contract_version` = 'v1_legacy_unverified',
    `onchain_locked` = 1
WHERE `status` NOT IN ('Setup', 'Set', 'pre_launch', 'warmup')
  AND UPPER(TRIM(`payment_token`)) IN ('JCC', 'JC')
  AND `contract_version` = 'unassigned';

CREATE TABLE IF NOT EXISTS `fund_ledger` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `chain_id` BIGINT UNSIGNED NOT NULL,
  `contract_address` VARCHAR(42) NOT NULL,
  `contract_version` VARCHAR(32) NOT NULL,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `wallet_address` VARCHAR(42) NOT NULL DEFAULT '',
  `counterparty_address` VARCHAR(42) NOT NULL DEFAULT '',
  `entry_type` VARCHAR(32) NOT NULL,
  `asset_role` VARCHAR(16) NOT NULL,
  `token_symbol` VARCHAR(20) NOT NULL DEFAULT '',
  `token_address` VARCHAR(42) NOT NULL,
  `token_decimals` TINYINT UNSIGNED NOT NULL,
  `amount_raw` VARCHAR(78) NOT NULL COMMENT 'canonical base-10 uint256 string',
  `amount_display` VARCHAR(160) NOT NULL COMMENT 'deterministic display string; never a float',
  `tx_hash` VARCHAR(66) NOT NULL,
  `log_index` INT UNSIGNED NOT NULL,
  `block_number` BIGINT UNSIGNED NOT NULL,
  `block_hash` VARCHAR(66) NOT NULL,
  `status` VARCHAR(20) NOT NULL DEFAULT 'canonical',
  `event_data` JSON NOT NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_fund_ledger_event` (`chain_id`, `tx_hash`, `log_index`),
  KEY `idx_fund_ledger_project` (`chain_id`, `contract_address`, `project_id`),
  KEY `idx_fund_ledger_wallet` (`wallet_address`),
  KEY `idx_fund_ledger_token` (`token_address`),
  KEY `idx_fund_ledger_block` (`block_number`),
  KEY `idx_fund_ledger_status` (`status`),
  CONSTRAINT `chk_fund_ledger_token_decimals` CHECK (`token_decimals` BETWEEN 1 AND 18),
  CONSTRAINT `chk_fund_ledger_positive_identity` CHECK (`chain_id` > 0 AND `project_id` > 0 AND `block_number` > 0),
  CONSTRAINT `chk_fund_ledger_contract_address` CHECK (
    `contract_address` REGEXP '^0x[0-9A-Fa-f]{40}$'
    AND `contract_address` <> '0x0000000000000000000000000000000000000000'
  ),
  CONSTRAINT `chk_fund_ledger_token_address` CHECK (
    `token_address` REGEXP '^0x[0-9A-Fa-f]{40}$'
    AND `token_address` <> '0x0000000000000000000000000000000000000000'
  ),
  CONSTRAINT `chk_fund_ledger_tx_hash` CHECK (`tx_hash` REGEXP '^0x[0-9A-Fa-f]{64}$'),
  CONSTRAINT `chk_fund_ledger_block_hash` CHECK (`block_hash` REGEXP '^0x[0-9A-Fa-f]{64}$'),
  CONSTRAINT `chk_fund_ledger_amount_raw` CHECK (
    `amount_raw` REGEXP '^(0|[1-9][0-9]{0,77})$'
    AND (
      CHAR_LENGTH(`amount_raw`) < 78
      OR `amount_raw` <= '115792089237316195423570985008687907853269984665640564039457584007913129639935'
    )
  ),
  CONSTRAINT `chk_fund_ledger_amount_display` CHECK (
    BINARY `amount_display` = BINARY CASE
      WHEN `amount_raw` = '0' THEN '0'
      WHEN CHAR_LENGTH(`amount_raw`) <= `token_decimals` THEN
        CONCAT('0.', TRIM(TRAILING '0' FROM LPAD(`amount_raw`, `token_decimals`, '0')))
      WHEN TRIM(TRAILING '0' FROM RIGHT(`amount_raw`, `token_decimals`)) = '' THEN
        LEFT(`amount_raw`, CHAR_LENGTH(`amount_raw`) - `token_decimals`)
      ELSE CONCAT(
        LEFT(`amount_raw`, CHAR_LENGTH(`amount_raw`) - `token_decimals`),
        '.',
        TRIM(TRAILING '0' FROM RIGHT(`amount_raw`, `token_decimals`))
      )
    END
  ),
  CONSTRAINT `chk_fund_ledger_contract_version` CHECK (`contract_version` = 'v2_stablecoin'),
  CONSTRAINT `chk_fund_ledger_entry_type` CHECK (`entry_type` IN (
    'project_token_deposited','purchase_payment','payment_refunded','project_token_claimed',
    'project_proceeds_paid','platform_fee_paid','project_tokens_withdrawn'
  )),
  CONSTRAINT `chk_fund_ledger_asset_role` CHECK (`asset_role` IN ('payment', 'project')),
  CONSTRAINT `chk_fund_ledger_entry_asset_role` CHECK (
    (`entry_type` IN ('project_token_deposited','project_token_claimed','project_tokens_withdrawn') AND `asset_role` = 'project')
    OR
    (`entry_type` IN ('purchase_payment','payment_refunded','project_proceeds_paid','platform_fee_paid') AND `asset_role` = 'payment')
  ),
  CONSTRAINT `chk_fund_ledger_status` CHECK (`status` IN ('canonical', 'reversed', 'reconciliation'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DROP PROCEDURE IF EXISTS jc_assert_ido_v2_ledger_schema;
DELIMITER $$
CREATE PROCEDURE jc_assert_ido_v2_ledger_schema()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'projects'
      AND COLUMN_NAME IN (
        'payment_token_address','payment_token_decimals','chain_id','contract_address',
        'contract_version','onchain_locked','token_price_raw','hard_cap_raw','soft_cap_raw',
        'max_per_user_raw','total_raised_raw','total_deposited_raw'
      )
  ) <> 12 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'projects IDO V2 identity columns are incomplete';
  END IF;

  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'projects'
      AND (
        (COLUMN_NAME IN ('payment_token_address','contract_address') AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 42 AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
        OR (COLUMN_NAME = 'payment_token' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 20 AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
        OR (COLUMN_NAME IN ('payment_token_decimals') AND DATA_TYPE = 'tinyint' AND COLUMN_TYPE = 'tinyint unsigned' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0')
        OR (COLUMN_NAME = 'chain_id' AND DATA_TYPE = 'bigint' AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0')
        OR (COLUMN_NAME = 'contract_version' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 32 AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = 'unassigned')
        OR (COLUMN_NAME = 'onchain_locked' AND DATA_TYPE = 'tinyint' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0')
        OR (COLUMN_NAME IN ('token_price_raw','hard_cap_raw','soft_cap_raw','max_per_user_raw','total_raised_raw','total_deposited_raw') AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 78 AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0')
        OR (COLUMN_NAME IN ('token_price','hard_cap','soft_cap','max_per_user','total_raised','total_deposited') AND DATA_TYPE = 'decimal' AND NUMERIC_PRECISION = 65 AND NUMERIC_SCALE = 18 AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0.000000000000000000')
      )
  ) <> 19 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'projects IDO V2 column definitions are incompatible';
  END IF;

  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'fund_ledger'
      AND COLUMN_NAME IN (
        'id','chain_id','contract_address','contract_version','project_id','wallet_address',
        'counterparty_address','entry_type','asset_role','token_symbol','token_address',
        'token_decimals','amount_raw','amount_display','tx_hash','log_index','block_number',
        'block_hash','status','event_data','created_at','updated_at'
      )
  ) <> 22 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'fund_ledger schema is incomplete';
  END IF;

  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'fund_ledger'
      AND (
        (COLUMN_NAME = 'id' AND DATA_TYPE = 'bigint' AND COLUMN_TYPE = 'bigint unsigned' AND EXTRA LIKE '%auto_increment%' AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME IN ('chain_id','project_id','block_number') AND DATA_TYPE = 'bigint' AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME IN ('contract_address','token_address') AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 42 AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT IS NULL)
        OR (COLUMN_NAME IN ('wallet_address','counterparty_address') AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 42 AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
        OR (COLUMN_NAME IN ('contract_version','entry_type') AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 32 AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'asset_role' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 16 AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'token_symbol' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 20 AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
        OR (COLUMN_NAME = 'status' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 20 AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = 'canonical')
        OR (COLUMN_NAME = 'token_decimals' AND DATA_TYPE = 'tinyint' AND COLUMN_TYPE = 'tinyint unsigned' AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'amount_raw' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 78 AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'amount_display' AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 160 AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME IN ('tx_hash','block_hash') AND DATA_TYPE = 'varchar' AND CHARACTER_MAXIMUM_LENGTH = 66 AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'log_index' AND DATA_TYPE = 'int' AND COLUMN_TYPE = 'int unsigned' AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'event_data' AND DATA_TYPE = 'json' AND IS_NULLABLE = 'NO')
        OR (COLUMN_NAME = 'created_at' AND DATA_TYPE = 'datetime' AND DATETIME_PRECISION = 6 AND IS_NULLABLE = 'NO' AND LOWER(COLUMN_DEFAULT) = 'current_timestamp(6)')
        OR (COLUMN_NAME = 'updated_at' AND DATA_TYPE = 'datetime' AND DATETIME_PRECISION = 6 AND IS_NULLABLE = 'NO' AND LOWER(COLUMN_DEFAULT) = 'current_timestamp(6)' AND LOWER(EXTRA) LIKE '%on update current_timestamp(6)%')
      )
  ) <> 22 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'fund_ledger column definitions are incompatible';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM (
      SELECT INDEX_NAME,
             GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') AS columns_in_order,
             MIN(NON_UNIQUE) AS non_unique
      FROM information_schema.STATISTICS
      WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'fund_ledger'
      GROUP BY INDEX_NAME
    ) ledger_indexes
    WHERE columns_in_order = 'chain_id,tx_hash,log_index' AND non_unique = 0
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'fund_ledger event identity is missing';
  END IF;

  IF (
    SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'fund_ledger'
      AND CONSTRAINT_TYPE = 'CHECK'
      AND ENFORCED = 'YES'
      AND CONSTRAINT_NAME IN (
        'chk_fund_ledger_token_decimals','chk_fund_ledger_positive_identity',
        'chk_fund_ledger_contract_address','chk_fund_ledger_token_address',
        'chk_fund_ledger_tx_hash','chk_fund_ledger_block_hash','chk_fund_ledger_amount_raw',
        'chk_fund_ledger_amount_display',
        'chk_fund_ledger_contract_version','chk_fund_ledger_entry_type',
        'chk_fund_ledger_asset_role','chk_fund_ledger_entry_asset_role',
        'chk_fund_ledger_status'
      )
  ) <> 13 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'fund_ledger check constraints are incomplete';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.CHECK_CONSTRAINTS cc
    JOIN information_schema.TABLE_CONSTRAINTS tc
      ON tc.CONSTRAINT_SCHEMA = cc.CONSTRAINT_SCHEMA AND tc.CONSTRAINT_NAME = cc.CONSTRAINT_NAME
    WHERE cc.CONSTRAINT_SCHEMA = DATABASE() AND tc.TABLE_NAME = 'fund_ledger'
      AND cc.CONSTRAINT_NAME = 'chk_fund_ledger_amount_raw'
      AND cc.CHECK_CLAUSE LIKE '%115792089237316195423570985008687907853269984665640564039457584007913129639935%'
      AND cc.CHECK_CLAUSE LIKE '%amount_raw%'
  ) OR NOT EXISTS (
    SELECT 1 FROM information_schema.CHECK_CONSTRAINTS cc
    JOIN information_schema.TABLE_CONSTRAINTS tc
      ON tc.CONSTRAINT_SCHEMA = cc.CONSTRAINT_SCHEMA AND tc.CONSTRAINT_NAME = cc.CONSTRAINT_NAME
    WHERE cc.CONSTRAINT_SCHEMA = DATABASE() AND tc.TABLE_NAME = 'fund_ledger'
      AND cc.CONSTRAINT_NAME = 'chk_fund_ledger_amount_display'
      AND cc.CHECK_CLAUSE LIKE '%amount_display%'
      AND cc.CHECK_CLAUSE LIKE '%amount_raw%'
      AND cc.CHECK_CLAUSE LIKE '%token_decimals%'
  ) OR NOT EXISTS (
    SELECT 1 FROM information_schema.CHECK_CONSTRAINTS cc
    JOIN information_schema.TABLE_CONSTRAINTS tc
      ON tc.CONSTRAINT_SCHEMA = cc.CONSTRAINT_SCHEMA AND tc.CONSTRAINT_NAME = cc.CONSTRAINT_NAME
    WHERE cc.CONSTRAINT_SCHEMA = DATABASE() AND tc.TABLE_NAME = 'fund_ledger'
      AND cc.CONSTRAINT_NAME = 'chk_fund_ledger_entry_type'
      AND cc.CHECK_CLAUSE LIKE '%project_token_deposited%'
      AND cc.CHECK_CLAUSE LIKE '%purchase_payment%'
      AND cc.CHECK_CLAUSE LIKE '%payment_refunded%'
      AND cc.CHECK_CLAUSE LIKE '%project_token_claimed%'
      AND cc.CHECK_CLAUSE LIKE '%project_proceeds_paid%'
      AND cc.CHECK_CLAUSE LIKE '%platform_fee_paid%'
      AND cc.CHECK_CLAUSE LIKE '%project_tokens_withdrawn%'
  ) OR NOT EXISTS (
    SELECT 1 FROM information_schema.CHECK_CONSTRAINTS cc
    JOIN information_schema.TABLE_CONSTRAINTS tc
      ON tc.CONSTRAINT_SCHEMA = cc.CONSTRAINT_SCHEMA AND tc.CONSTRAINT_NAME = cc.CONSTRAINT_NAME
    WHERE cc.CONSTRAINT_SCHEMA = DATABASE() AND tc.TABLE_NAME = 'fund_ledger'
      AND cc.CONSTRAINT_NAME = 'chk_fund_ledger_entry_asset_role'
      AND cc.CHECK_CLAUSE LIKE '%entry_type%'
      AND cc.CHECK_CLAUSE LIKE '%asset_role%'
      AND cc.CHECK_CLAUSE LIKE '%payment%'
      AND cc.CHECK_CLAUSE LIKE '%project%'
  ) OR NOT EXISTS (
    SELECT 1 FROM information_schema.CHECK_CONSTRAINTS cc
    JOIN information_schema.TABLE_CONSTRAINTS tc
      ON tc.CONSTRAINT_SCHEMA = cc.CONSTRAINT_SCHEMA AND tc.CONSTRAINT_NAME = cc.CONSTRAINT_NAME
    WHERE cc.CONSTRAINT_SCHEMA = DATABASE() AND tc.TABLE_NAME = 'fund_ledger'
      AND cc.CONSTRAINT_NAME = 'chk_fund_ledger_contract_version'
      AND cc.CHECK_CLAUSE LIKE '%v2_stablecoin%'
  ) OR NOT EXISTS (
    SELECT 1 FROM information_schema.CHECK_CONSTRAINTS cc
    JOIN information_schema.TABLE_CONSTRAINTS tc
      ON tc.CONSTRAINT_SCHEMA = cc.CONSTRAINT_SCHEMA AND tc.CONSTRAINT_NAME = cc.CONSTRAINT_NAME
    WHERE cc.CONSTRAINT_SCHEMA = DATABASE() AND tc.TABLE_NAME = 'fund_ledger'
      AND cc.CONSTRAINT_NAME = 'chk_fund_ledger_status'
      AND cc.CHECK_CLAUSE LIKE '%canonical%'
      AND cc.CHECK_CLAUSE LIKE '%reversed%'
      AND cc.CHECK_CLAUSE LIKE '%reconciliation%'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'fund_ledger check constraint definitions are incompatible';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_ido_v2_ledger_schema();
DROP PROCEDURE jc_assert_ido_v2_ledger_schema;

-- Verification:
--   SHOW CREATE TABLE fund_ledger;
--   SELECT status,payment_token,contract_version,onchain_locked,COUNT(*) FROM projects GROUP BY 1,2,3,4;

-- END 2026_08_15_ido_v2_fund_ledger.sql

-- BEGIN 2026_08_16_ido_v2_event_routing.sql
-- JoinCare IDO Sprint 1A-P2 · persist V1/V2 event source routing and V2 scan registration.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_v2_event_routing;
DELIMITER $$
CREATE PROCEDURE jc_preflight_v2_event_routing()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME IN ('projects','chain_event_record','chain_event_failure','chain_event_sync_state','fund_ledger')
  ) <> 5 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'V2 event routing prerequisite tables are missing';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND (
      (TABLE_NAME = 'chain_event_record' AND COLUMN_NAME IN ('contract_address','contract_version'))
      OR (TABLE_NAME = 'chain_event_failure' AND COLUMN_NAME IN ('contract_address','contract_version'))
      OR (TABLE_NAME = 'chain_event_sync_state' AND COLUMN_NAME IN ('v2_registered','v2_contract_address','v2_start_block','v2_implementation_address','v2_implementation_code_hash'))
      OR (TABLE_NAME = 'projects' AND COLUMN_NAME IN ('fee_receiver','success_fee_rate','platform_fee_withdrawn'))
    )
  ) <> 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'partial or pre-existing V2 event routing schema requires manual recovery';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_v2_event_routing();
DROP PROCEDURE jc_preflight_v2_event_routing;

DROP PROCEDURE IF EXISTS jc_add_event_source_column;
DELIMITER $$
CREATE PROCEDURE jc_add_event_source_column(IN p_table VARCHAR(64), IN p_name VARCHAR(64), IN p_definition TEXT)
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = p_table AND COLUMN_NAME = p_name
  ) THEN
    SET @sql = CONCAT('ALTER TABLE `', REPLACE(p_table, '`', '``'), '` ADD COLUMN `', REPLACE(p_name, '`', '``'), '` ', p_definition);
    PREPARE stmt FROM @sql;
    EXECUTE stmt;
    DEALLOCATE PREPARE stmt;
  END IF;
END$$
DELIMITER ;

CALL jc_add_event_source_column('chain_event_record', 'contract_address', 'VARCHAR(42) NOT NULL DEFAULT '''' AFTER `log_index`');
CALL jc_add_event_source_column('chain_event_record', 'contract_version', 'VARCHAR(32) NOT NULL DEFAULT ''v1_legacy_unverified'' AFTER `contract_address`');
CALL jc_add_event_source_column('chain_event_failure', 'contract_address', 'VARCHAR(42) NOT NULL DEFAULT '''' AFTER `log_index`');
CALL jc_add_event_source_column('chain_event_failure', 'contract_version', 'VARCHAR(32) NOT NULL DEFAULT ''v1_legacy_unverified'' AFTER `contract_address`');
CALL jc_add_event_source_column('chain_event_sync_state', 'v2_registered', 'TINYINT(1) NOT NULL DEFAULT 0 AFTER `lease_token`');
CALL jc_add_event_source_column('chain_event_sync_state', 'v2_contract_address', 'VARCHAR(42) NOT NULL DEFAULT '''' AFTER `v2_registered`');
CALL jc_add_event_source_column('chain_event_sync_state', 'v2_start_block', 'BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER `v2_contract_address`');
CALL jc_add_event_source_column('chain_event_sync_state', 'v2_implementation_address', 'VARCHAR(42) NOT NULL DEFAULT '''' AFTER `v2_start_block`');
CALL jc_add_event_source_column('chain_event_sync_state', 'v2_implementation_code_hash', 'VARCHAR(66) NOT NULL DEFAULT '''' AFTER `v2_implementation_address`');
CALL jc_add_event_source_column('projects', 'fee_receiver', 'VARCHAR(42) NOT NULL DEFAULT '''' AFTER `onchain_locked`');
CALL jc_add_event_source_column('projects', 'success_fee_rate', 'SMALLINT UNSIGNED NOT NULL DEFAULT 0 AFTER `fee_receiver`');
CALL jc_add_event_source_column('projects', 'platform_fee_withdrawn', 'TINYINT(1) NOT NULL DEFAULT 0 AFTER `fund_withdrawn`');
DROP PROCEDURE jc_add_event_source_column;

-- Existing rows receive v1_legacy_unverified from the ADD COLUMN default because
-- their historical contract address was not persisted and must not be invented.

DROP PROCEDURE IF EXISTS jc_assert_v2_event_routing_schema;
DELIMITER $$
CREATE PROCEDURE jc_assert_v2_event_routing_schema()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND (
      (TABLE_NAME = 'chain_event_record' AND COLUMN_NAME IN ('contract_address','contract_version'))
      OR (TABLE_NAME = 'chain_event_failure' AND COLUMN_NAME IN ('contract_address','contract_version'))
      OR (TABLE_NAME = 'chain_event_sync_state' AND COLUMN_NAME IN ('v2_registered','v2_contract_address','v2_start_block','v2_implementation_address','v2_implementation_code_hash'))
      OR (TABLE_NAME = 'projects' AND COLUMN_NAME IN ('fee_receiver','success_fee_rate','platform_fee_withdrawn'))
    )
  ) <> 12 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'V2 event routing columns are incomplete';
  END IF;

  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND (
      (TABLE_NAME IN ('chain_event_record','chain_event_failure') AND COLUMN_NAME = 'contract_address' AND COLUMN_TYPE = 'varchar(42)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
      OR (TABLE_NAME IN ('chain_event_record','chain_event_failure') AND COLUMN_NAME = 'contract_version' AND COLUMN_TYPE = 'varchar(32)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = 'v1_legacy_unverified')
      OR (TABLE_NAME = 'chain_event_sync_state' AND COLUMN_NAME = 'v2_registered' AND COLUMN_TYPE = 'tinyint(1)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0')
      OR (TABLE_NAME = 'chain_event_sync_state' AND COLUMN_NAME = 'v2_contract_address' AND COLUMN_TYPE = 'varchar(42)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
      OR (TABLE_NAME = 'chain_event_sync_state' AND COLUMN_NAME = 'v2_start_block' AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0')
      OR (TABLE_NAME = 'chain_event_sync_state' AND COLUMN_NAME = 'v2_implementation_address' AND COLUMN_TYPE = 'varchar(42)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
      OR (TABLE_NAME = 'chain_event_sync_state' AND COLUMN_NAME = 'v2_implementation_code_hash' AND COLUMN_TYPE = 'varchar(66)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
      OR (TABLE_NAME = 'projects' AND COLUMN_NAME = 'fee_receiver' AND COLUMN_TYPE = 'varchar(42)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
      OR (TABLE_NAME = 'projects' AND COLUMN_NAME = 'success_fee_rate' AND COLUMN_TYPE = 'smallint unsigned' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0')
      OR (TABLE_NAME = 'projects' AND COLUMN_NAME = 'platform_fee_withdrawn' AND COLUMN_TYPE = 'tinyint(1)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0')
    )
  ) <> 12 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'V2 event routing column definitions are incompatible';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_v2_event_routing_schema();
DROP PROCEDURE jc_assert_v2_event_routing_schema;

-- Verification:
--   SELECT id,cursor_block,v2_registered,v2_contract_address,v2_start_block,v2_implementation_address,v2_implementation_code_hash FROM chain_event_sync_state;

-- END 2026_08_16_ido_v2_event_routing.sql

-- BEGIN 2026_08_16_zz_chain_event_collation_compat.sql
-- JoinCare IDO migration-path compatibility.
-- The original chain_event_record table used utf8mb4_general_ci while
-- transaction_record uses utf8mb4_unicode_ci. MySQL 8 rejects the equality
-- predicates in the 2026_08_17 outbox preflight when these collations differ.
-- Normalize only the two compared legacy columns before that migration.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_event_collation_compat;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_event_collation_compat()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'
      AND TABLE_NAME IN ('chain_event_record','transaction_record')
  )<>2 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='chain event collation prerequisite tables are missing';
  END IF;

  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA=DATABASE() AND (
      (TABLE_NAME='chain_event_record' AND COLUMN_NAME='event'
        AND COLUMN_TYPE='varchar(50)' AND IS_NULLABLE='NO'
        AND CHARACTER_SET_NAME='utf8mb4'
        AND COLLATION_NAME IN ('utf8mb4_general_ci','utf8mb4_unicode_ci'))
      OR (TABLE_NAME='chain_event_record' AND COLUMN_NAME='tx_hash'
        AND COLUMN_TYPE='varchar(100)' AND IS_NULLABLE='NO'
        AND CHARACTER_SET_NAME='utf8mb4'
        AND COLLATION_NAME IN ('utf8mb4_general_ci','utf8mb4_unicode_ci'))
      OR (TABLE_NAME='transaction_record' AND COLUMN_NAME='type'
        AND COLUMN_TYPE='varchar(20)' AND IS_NULLABLE='NO'
        AND CHARACTER_SET_NAME='utf8mb4' AND COLLATION_NAME='utf8mb4_unicode_ci')
      OR (TABLE_NAME='transaction_record' AND COLUMN_NAME='tx_hash'
        AND COLUMN_TYPE='varchar(66)' AND IS_NULLABLE='NO'
        AND CHARACTER_SET_NAME='utf8mb4' AND COLLATION_NAME='utf8mb4_unicode_ci')
    )
  )<>4 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='chain event comparison column definitions are incompatible';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM (
      SELECT INDEX_NAME,
        GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') columns_in_order,
        MIN(NON_UNIQUE) non_unique
      FROM information_schema.STATISTICS
      WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_event_record'
      GROUP BY INDEX_NAME
    ) indexes_for_event
    WHERE columns_in_order='block_number,tx_hash,log_index' AND non_unique=0
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='chain event identity index is missing before collation normalization';
  END IF;

  IF EXISTS (
    SELECT 1 FROM chain_event_record
    GROUP BY block_number,tx_hash COLLATE utf8mb4_unicode_ci,log_index
    HAVING COUNT(*)>1
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='chain event identities collide under target collation';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_event_collation_compat();
DROP PROCEDURE jc_preflight_chain_event_collation_compat;

ALTER TABLE chain_event_record
  MODIFY COLUMN event VARCHAR(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  MODIFY COLUMN tx_hash VARCHAR(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL;

DROP PROCEDURE IF EXISTS jc_assert_chain_event_collation_compat;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_collation_compat()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA=DATABASE() AND (
      (TABLE_NAME='chain_event_record' AND COLUMN_NAME='event'
        AND COLUMN_TYPE='varchar(50)' AND IS_NULLABLE='NO'
        AND CHARACTER_SET_NAME='utf8mb4' AND COLLATION_NAME='utf8mb4_unicode_ci')
      OR (TABLE_NAME='chain_event_record' AND COLUMN_NAME='tx_hash'
        AND COLUMN_TYPE='varchar(100)' AND IS_NULLABLE='NO'
        AND CHARACTER_SET_NAME='utf8mb4' AND COLLATION_NAME='utf8mb4_unicode_ci')
      OR (TABLE_NAME='transaction_record' AND COLUMN_NAME='type'
        AND COLUMN_TYPE='varchar(20)' AND IS_NULLABLE='NO'
        AND CHARACTER_SET_NAME='utf8mb4' AND COLLATION_NAME='utf8mb4_unicode_ci')
      OR (TABLE_NAME='transaction_record' AND COLUMN_NAME='tx_hash'
        AND COLUMN_TYPE='varchar(66)' AND IS_NULLABLE='NO'
        AND CHARACTER_SET_NAME='utf8mb4' AND COLLATION_NAME='utf8mb4_unicode_ci')
    )
  )<>4 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='chain event comparison collations were not normalized';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM (
      SELECT INDEX_NAME,
        GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') columns_in_order,
        MIN(NON_UNIQUE) non_unique
      FROM information_schema.STATISTICS
      WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_event_record'
      GROUP BY INDEX_NAME
    ) indexes_for_event
    WHERE columns_in_order='block_number,tx_hash,log_index' AND non_unique=0
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='chain event identity index changed during collation normalization';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_collation_compat();
DROP PROCEDURE jc_assert_chain_event_collation_compat;

-- END 2026_08_16_zz_chain_event_collation_compat.sql

-- BEGIN 2026_08_17_chain_tx_outbox.sql
-- JoinCare IDO Sprint 1A-P2 · durable relayer transaction outbox and nonce fence.
-- Signed raw transactions are public bearer payloads; they never contain private keys.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_tx_outbox;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_tx_outbox()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'transaction_record' AND TABLE_TYPE = 'BASE TABLE'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'transaction_record prerequisite table is missing';
  END IF;

  IF EXISTS (
    SELECT 1 FROM transaction_record
    WHERE type IN ('v2_create','v2_activate','project_end') AND TRIM(tx_hash) = ''
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'blank lifecycle transaction_record rows require manual chain reconciliation before outbox migration';
  END IF;

  IF EXISTS (
    SELECT 1 FROM transaction_record tr
    WHERE tr.type IN ('v2_create','v2_activate')
      AND TRIM(tr.tx_hash) <> ''
      AND NOT EXISTS (
        SELECT 1 FROM chain_event_record ce
        WHERE LOWER(ce.tx_hash) = LOWER(tr.tx_hash)
          AND ce.event = CASE tr.type WHEN 'v2_create' THEN 'ProjectCreated' ELSE 'ProjectActivated' END
      )
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'unconsumed legacy V2 lifecycle hashes require manual reconciliation before outbox migration';
  END IF;

  SET @outbox_tables = (
    SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME IN ('chain_tx_submission','chain_relayer_nonce')
  );
  IF @outbox_tables NOT IN (0, 2) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'partial chain transaction outbox schema requires manual recovery';
  END IF;
  IF @outbox_tables = 2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'pre-existing chain transaction outbox requires manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_tx_outbox();
DROP PROCEDURE jc_preflight_chain_tx_outbox;

CREATE TABLE `chain_tx_submission` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `action` VARCHAR(32) NOT NULL,
  `attempt_no` INT UNSIGNED NOT NULL,
  `chain_id` BIGINT UNSIGNED NOT NULL,
  `sender_address` VARCHAR(42) NOT NULL,
  `contract_address` VARCHAR(42) NOT NULL,
  `call_data` MEDIUMBLOB NOT NULL,
  `value_raw` VARCHAR(78) NOT NULL DEFAULT '0',
  `state` VARCHAR(32) NOT NULL,
  `nonce` BIGINT UNSIGNED NULL,
  `gas_limit` BIGINT UNSIGNED NULL,
  `gas_price_raw` VARCHAR(78) NOT NULL DEFAULT '0',
  `raw_tx` MEDIUMBLOB NULL,
  `tx_hash` VARCHAR(66) NULL,
  `active_slot` TINYINT UNSIGNED NULL,
  `broadcast_count` INT UNSIGNED NOT NULL DEFAULT 0,
  `last_broadcast_at` DATETIME(6) NULL,
  `last_error` TEXT NOT NULL,
  `receipt_status` BIGINT UNSIGNED NULL,
  `receipt_block_number` BIGINT UNSIGNED NULL,
  `receipt_block_hash` VARCHAR(66) NOT NULL DEFAULT '',
  `receipt_gas_used` BIGINT UNSIGNED NULL,
  `final_event_record_id` BIGINT UNSIGNED NULL,
  `finalized_at` DATETIME(6) NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_chain_tx_business_attempt` (`chain_id`,`contract_address`,`project_id`,`action`,`attempt_no`),
  UNIQUE KEY `uk_chain_tx_active_business` (`chain_id`,`contract_address`,`project_id`,`action`,`active_slot`),
  UNIQUE KEY `uk_chain_tx_sender_nonce` (`chain_id`,`sender_address`,`nonce`),
  UNIQUE KEY `uk_chain_tx_hash` (`tx_hash`),
  KEY `idx_chain_tx_recovery` (`state`,`updated_at`,`id`),
  CONSTRAINT `chk_chain_tx_project` CHECK (`project_id` > 0) ENFORCED,
  CONSTRAINT `chk_chain_tx_attempt` CHECK (`attempt_no` > 0) ENFORCED,
  CONSTRAINT `chk_chain_tx_chain` CHECK (`chain_id` > 0) ENFORCED,
  CONSTRAINT `chk_chain_tx_sender` CHECK (`sender_address` REGEXP '^0x[0-9A-Fa-f]{40}$' AND `sender_address` <> '0x0000000000000000000000000000000000000000') ENFORCED,
  CONSTRAINT `chk_chain_tx_contract` CHECK (`contract_address` REGEXP '^0x[0-9A-Fa-f]{40}$' AND `contract_address` <> '0x0000000000000000000000000000000000000000') ENFORCED,
  CONSTRAINT `chk_chain_tx_value` CHECK (`value_raw` = '0') ENFORCED,
  CONSTRAINT `chk_chain_tx_calldata` CHECK (OCTET_LENGTH(`call_data`) > 0) ENFORCED,
  CONSTRAINT `chk_chain_tx_action` CHECK (`action` IN ('v2_create','v2_activate','project_end')) ENFORCED,
  CONSTRAINT `chk_chain_tx_state` CHECK (`state` IN ('intent','ready','signed','submitted','broadcast_unknown','mined_success','mined_revert','final_success','final_revert','applied','cancelled','ambiguous')) ENFORCED,
  CONSTRAINT `chk_chain_tx_active` CHECK (`active_slot` IS NULL OR `active_slot` = 1) ENFORCED,
  CONSTRAINT `chk_chain_tx_active_state` CHECK (
    (`state` IN ('applied','cancelled','final_revert') AND `active_slot` IS NULL)
    OR (`state` NOT IN ('applied','cancelled','final_revert') AND `active_slot` = 1)
  ) ENFORCED,
  CONSTRAINT `chk_chain_tx_hash_format` CHECK (`tx_hash` IS NULL OR `tx_hash` REGEXP '^0x[0-9A-Fa-f]{64}$') ENFORCED,
  CONSTRAINT `chk_chain_tx_receipt_hash` CHECK (`receipt_block_hash` = '' OR `receipt_block_hash` REGEXP '^0x[0-9A-Fa-f]{64}$') ENFORCED,
  CONSTRAINT `chk_chain_tx_receipt_status` CHECK (`receipt_status` IS NULL OR `receipt_status` IN (0,1)) ENFORCED,
  CONSTRAINT `chk_chain_tx_gas_price` CHECK (`gas_price_raw` REGEXP '^(0|[1-9][0-9]{0,64})$') ENFORCED,
  CONSTRAINT `chk_chain_tx_signed_shape` CHECK (
    (`state` IN ('intent','ready','cancelled') AND `nonce` IS NULL AND `gas_limit` IS NULL AND `raw_tx` IS NULL AND `tx_hash` IS NULL)
    OR (`state` NOT IN ('intent','ready','cancelled') AND `nonce` IS NOT NULL AND `gas_limit` IS NOT NULL AND OCTET_LENGTH(`raw_tx`) > 0 AND `tx_hash` IS NOT NULL)
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `chain_relayer_nonce` (
  `chain_id` BIGINT UNSIGNED NOT NULL,
  `sender_address` VARCHAR(42) NOT NULL,
  `next_nonce` BIGINT UNSIGNED NOT NULL,
  `active_submission_id` BIGINT UNSIGNED NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`chain_id`,`sender_address`),
  UNIQUE KEY `uk_chain_relayer_active_submission` (`active_submission_id`),
  CONSTRAINT `chk_chain_relayer_chain` CHECK (`chain_id` > 0) ENFORCED,
  CONSTRAINT `chk_chain_relayer_sender` CHECK (`sender_address` REGEXP '^0x[0-9A-Fa-f]{40}$' AND `sender_address` <> '0x0000000000000000000000000000000000000000') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DROP PROCEDURE IF EXISTS jc_assert_chain_tx_outbox_schema;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_tx_outbox_schema()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_tx_submission'
      AND COLUMN_NAME IN ('id','project_id','action','attempt_no','chain_id','sender_address','contract_address','call_data','value_raw','state','nonce','gas_limit','gas_price_raw','raw_tx','tx_hash','active_slot','broadcast_count','last_broadcast_at','last_error','receipt_status','receipt_block_number','receipt_block_hash','receipt_gas_used','final_event_record_id','finalized_at','created_at','updated_at')
  ) <> 27 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain_tx_submission schema is incomplete';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_relayer_nonce'
      AND COLUMN_NAME IN ('chain_id','sender_address','next_nonce','active_submission_id','created_at','updated_at')
  ) <> 6 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain_relayer_nonce schema is incomplete';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND (
      (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME = 'id' AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO' AND EXTRA LIKE '%auto_increment%')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME IN ('project_id','chain_id') AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME = 'action' AND COLUMN_TYPE = 'varchar(32)' AND IS_NULLABLE = 'NO')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME = 'attempt_no' AND COLUMN_TYPE = 'int unsigned' AND IS_NULLABLE = 'NO')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME IN ('sender_address','contract_address') AND COLUMN_TYPE = 'varchar(42)' AND IS_NULLABLE = 'NO')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME IN ('call_data','raw_tx') AND DATA_TYPE = 'mediumblob')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME IN ('value_raw','gas_price_raw') AND COLUMN_TYPE = 'varchar(78)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT IN ('0'))
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME = 'state' AND COLUMN_TYPE = 'varchar(32)' AND IS_NULLABLE = 'NO')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME IN ('nonce','gas_limit','receipt_status','receipt_block_number','receipt_gas_used','final_event_record_id') AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'YES')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME = 'tx_hash' AND COLUMN_TYPE = 'varchar(66)' AND IS_NULLABLE = 'YES')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME = 'active_slot' AND COLUMN_TYPE = 'tinyint unsigned' AND IS_NULLABLE = 'YES')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME = 'broadcast_count' AND COLUMN_TYPE = 'int unsigned' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '0')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME = 'last_error' AND DATA_TYPE = 'text' AND IS_NULLABLE = 'NO')
      OR (TABLE_NAME = 'chain_tx_submission' AND COLUMN_NAME = 'receipt_block_hash' AND COLUMN_TYPE = 'varchar(66)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
      OR (TABLE_NAME = 'chain_relayer_nonce' AND COLUMN_NAME IN ('chain_id','next_nonce') AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO')
      OR (TABLE_NAME = 'chain_relayer_nonce' AND COLUMN_NAME = 'sender_address' AND COLUMN_TYPE = 'varchar(42)' AND IS_NULLABLE = 'NO')
      OR (TABLE_NAME = 'chain_relayer_nonce' AND COLUMN_NAME = 'active_submission_id' AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'YES')
    )
  ) <> 27 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain transaction outbox critical column definitions are incompatible';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_tx_submission'
      AND CONSTRAINT_TYPE = 'CHECK' AND ENFORCED = 'YES'
      AND CONSTRAINT_NAME IN ('chk_chain_tx_project','chk_chain_tx_attempt','chk_chain_tx_chain','chk_chain_tx_sender','chk_chain_tx_contract','chk_chain_tx_value','chk_chain_tx_calldata','chk_chain_tx_action','chk_chain_tx_state','chk_chain_tx_active','chk_chain_tx_active_state','chk_chain_tx_hash_format','chk_chain_tx_receipt_hash','chk_chain_tx_receipt_status','chk_chain_tx_gas_price','chk_chain_tx_signed_shape')
  ) <> 16 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain_tx_submission enforced checks are incomplete';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM (
      SELECT INDEX_NAME, GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') AS columns_in_order, MIN(NON_UNIQUE) AS non_unique
      FROM information_schema.STATISTICS
      WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_tx_submission'
      GROUP BY INDEX_NAME
    ) indexes_for_submission
    WHERE (INDEX_NAME = 'uk_chain_tx_business_attempt' AND columns_in_order = 'chain_id,contract_address,project_id,action,attempt_no' AND non_unique = 0)
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain transaction business attempt fence is missing';
  END IF;
  IF (
    SELECT COUNT(*) FROM (
      SELECT INDEX_NAME, GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') AS columns_in_order, MIN(NON_UNIQUE) AS non_unique
      FROM information_schema.STATISTICS
      WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_tx_submission'
      GROUP BY INDEX_NAME
    ) indexes_for_submission
    WHERE non_unique = 0 AND (
      (INDEX_NAME = 'uk_chain_tx_active_business' AND columns_in_order = 'chain_id,contract_address,project_id,action,active_slot')
      OR (INDEX_NAME = 'uk_chain_tx_sender_nonce' AND columns_in_order = 'chain_id,sender_address,nonce')
      OR (INDEX_NAME = 'uk_chain_tx_hash' AND columns_in_order = 'tx_hash')
    )
  ) <> 3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain transaction unique fences are incomplete';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_relayer_nonce'
      AND INDEX_NAME = 'uk_chain_relayer_active_submission' AND COLUMN_NAME = 'active_submission_id' AND NON_UNIQUE = 0
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain relayer active submission fence is missing';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_tx_outbox_schema();
DROP PROCEDURE jc_assert_chain_tx_outbox_schema;

-- Rollout requires exclusive nonce ownership for the configured relayer key.
-- Do not insert chain_relayer_nonce manually until latest and pending RPC nonce
-- plus all historical pending transactions have been audited.

-- END 2026_08_17_chain_tx_outbox.sql

-- BEGIN 2026_08_18_chain_tx_admin_cancellation.sql
-- JoinCare IDO Sprint 1A-P2 · durable administrator cancellation evidence.
-- This migration extends 2026_08_17; do not run it independently.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_tx_admin_cancellation;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_tx_admin_cancellation()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_tx_submission' AND TABLE_TYPE = 'BASE TABLE'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain_tx_submission prerequisite table is missing';
  END IF;
  SET @cancel_columns = (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_tx_submission'
      AND COLUMN_NAME IN ('cancelled_by_admin_id','cancel_reason','cancelled_at')
  );
  IF @cancel_columns NOT IN (0, 3) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'partial chain transaction cancellation schema requires manual recovery';
  END IF;
  IF @cancel_columns = 3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'pre-existing chain transaction cancellation schema requires manual verification';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_tx_submission'
      AND CONSTRAINT_NAME IN ('chk_chain_tx_state','chk_chain_tx_active_state','chk_chain_tx_signed_shape')
      AND CONSTRAINT_TYPE = 'CHECK' AND ENFORCED = 'YES'
  ) <> 3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = '2026_08_17 chain transaction checks are missing';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_tx_admin_cancellation();
DROP PROCEDURE jc_preflight_chain_tx_admin_cancellation;

ALTER TABLE `chain_tx_submission`
  DROP CHECK `chk_chain_tx_state`,
  DROP CHECK `chk_chain_tx_active_state`,
  DROP CHECK `chk_chain_tx_signed_shape`,
  ADD COLUMN `cancelled_by_admin_id` BIGINT UNSIGNED NULL AFTER `finalized_at`,
  ADD COLUMN `cancel_reason` VARCHAR(500) NOT NULL DEFAULT '' AFTER `cancelled_by_admin_id`,
  ADD COLUMN `cancelled_at` DATETIME(6) NULL AFTER `cancel_reason`,
  ADD CONSTRAINT `chk_chain_tx_state` CHECK (`state` IN ('intent','ready','signed','submitted','broadcast_unknown','mined_success','mined_revert','final_success','final_revert','applied','cancelled','admin_cancelled','ambiguous')) ENFORCED,
  ADD CONSTRAINT `chk_chain_tx_active_state` CHECK (
    (`state` IN ('applied','cancelled','admin_cancelled','final_revert') AND `active_slot` IS NULL)
    OR (`state` NOT IN ('applied','cancelled','admin_cancelled','final_revert') AND `active_slot` = 1)
  ) ENFORCED,
  ADD CONSTRAINT `chk_chain_tx_signed_shape` CHECK (
    (`state` IN ('intent','ready','cancelled','admin_cancelled') AND `nonce` IS NULL AND `gas_limit` IS NULL AND `raw_tx` IS NULL AND `tx_hash` IS NULL)
    OR (`state` NOT IN ('intent','ready','cancelled','admin_cancelled') AND `nonce` IS NOT NULL AND `gas_limit` IS NOT NULL AND OCTET_LENGTH(`raw_tx`) > 0 AND `tx_hash` IS NOT NULL)
  ) ENFORCED,
  ADD CONSTRAINT `chk_chain_tx_admin_cancellation` CHECK (
    (`state` = 'admin_cancelled' AND `cancelled_by_admin_id` IS NOT NULL AND `cancelled_by_admin_id` > 0 AND CHAR_LENGTH(TRIM(`cancel_reason`)) BETWEEN 3 AND 500 AND `cancelled_at` IS NOT NULL)
    OR (`state` <> 'admin_cancelled' AND `cancelled_by_admin_id` IS NULL AND `cancel_reason` = '' AND `cancelled_at` IS NULL)
  ) ENFORCED;

DROP PROCEDURE IF EXISTS jc_assert_chain_tx_admin_cancellation;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_tx_admin_cancellation()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_tx_submission' AND (
      (COLUMN_NAME = 'cancelled_by_admin_id' AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'YES')
      OR (COLUMN_NAME = 'cancel_reason' AND COLUMN_TYPE = 'varchar(500)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = '')
      OR (COLUMN_NAME = 'cancelled_at' AND COLUMN_TYPE = 'datetime(6)' AND IS_NULLABLE = 'YES')
    )
  ) <> 3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain transaction cancellation columns are incompatible';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_tx_submission'
      AND CONSTRAINT_NAME IN ('chk_chain_tx_state','chk_chain_tx_active_state','chk_chain_tx_signed_shape','chk_chain_tx_admin_cancellation')
      AND CONSTRAINT_TYPE = 'CHECK' AND ENFORCED = 'YES'
  ) <> 4 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain transaction cancellation checks are incomplete';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.CHECK_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME = 'chk_chain_tx_admin_cancellation'
      AND CHECK_CLAUSE LIKE '%admin_cancelled%cancelled_by_admin_id%cancel_reason%cancelled_at%'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain transaction cancellation check clause is incompatible';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_tx_admin_cancellation();
DROP PROCEDURE jc_assert_chain_tx_admin_cancellation;

-- END 2026_08_18_chain_tx_admin_cancellation.sql

-- BEGIN 2026_08_19_chain_event_replay_audit.sql
-- JoinCare IDO Sprint 1A-P2 · append-only administrator replay evidence.
-- This migration extends 2026_08_14 and 2026_08_16; do not run it independently.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_event_replay_audit;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_event_replay_audit()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_failure' AND TABLE_TYPE = 'BASE TABLE'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain_event_failure prerequisite table is missing';
  END IF;
  IF EXISTS (
    SELECT 1 FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_replay_audit'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'pre-existing chain event replay audit table requires manual verification';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_failure'
      AND COLUMN_NAME IN ('id','event','status','replay_count','replay_requested_at')
  ) <> 5 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain event recovery prerequisite columns are missing';
  END IF;
  IF EXISTS (
    SELECT 1 FROM chain_event_failure
    WHERE replay_count <> 0 OR replay_requested_at IS NOT NULL
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'historical replay records require manual evidence reconciliation before migration';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_event_replay_audit();
DROP PROCEDURE jc_preflight_chain_event_replay_audit;

CREATE TABLE `chain_event_replay_audit` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `failure_id` BIGINT UNSIGNED NOT NULL,
  `replay_no` INT UNSIGNED NOT NULL,
  `admin_id` BIGINT UNSIGNED NOT NULL,
  `reason` VARCHAR(500) NOT NULL,
  `requested_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_chain_event_replay_no` (`failure_id`,`replay_no`),
  KEY `idx_chain_event_replay_admin` (`admin_id`,`requested_at`),
  CONSTRAINT `fk_chain_event_replay_failure` FOREIGN KEY (`failure_id`) REFERENCES `chain_event_failure` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `chk_chain_event_replay_number` CHECK (`replay_no` > 0) ENFORCED,
  CONSTRAINT `chk_chain_event_replay_admin` CHECK (`admin_id` > 0) ENFORCED,
  CONSTRAINT `chk_chain_event_replay_reason` CHECK (CHAR_LENGTH(TRIM(`reason`)) BETWEEN 3 AND 500) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DROP TRIGGER IF EXISTS `trg_chain_event_replay_audit_no_update`;
DROP TRIGGER IF EXISTS `trg_chain_event_replay_audit_no_delete`;
DELIMITER $$
CREATE TRIGGER `trg_chain_event_replay_audit_no_update`
BEFORE UPDATE ON `chain_event_replay_audit`
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain event replay audit rows are append-only';
END$$
CREATE TRIGGER `trg_chain_event_replay_audit_no_delete`
BEFORE DELETE ON `chain_event_replay_audit`
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain event replay audit rows are append-only';
END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS jc_assert_chain_event_replay_audit;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_replay_audit()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_replay_audit' AND (
      (COLUMN_NAME = 'id' AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME = 'failure_id' AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME = 'replay_no' AND COLUMN_TYPE = 'int unsigned' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME = 'admin_id' AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME = 'reason' AND COLUMN_TYPE = 'varchar(500)' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME = 'requested_at' AND COLUMN_TYPE = 'datetime(6)' AND IS_NULLABLE = 'NO')
    )
  ) <> 6 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain event replay audit columns are incompatible';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM (
      SELECT INDEX_NAME, GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') AS columns_in_order, MIN(NON_UNIQUE) AS non_unique
      FROM information_schema.STATISTICS
      WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_replay_audit'
      GROUP BY INDEX_NAME
    ) replay_indexes
    WHERE columns_in_order = 'failure_id,replay_no' AND non_unique = 0
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain event replay audit unique sequence is missing';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_replay_audit'
      AND CONSTRAINT_NAME IN ('chk_chain_event_replay_number','chk_chain_event_replay_admin','chk_chain_event_replay_reason')
      AND CONSTRAINT_TYPE = 'CHECK' AND ENFORCED = 'YES'
  ) <> 3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain event replay audit checks are incomplete';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.REFERENTIAL_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_replay_audit'
      AND CONSTRAINT_NAME = 'fk_chain_event_replay_failure' AND REFERENCED_TABLE_NAME = 'chain_event_failure'
      AND DELETE_RULE = 'RESTRICT'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain event replay audit foreign key is missing';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.TRIGGERS
    WHERE TRIGGER_SCHEMA = DATABASE() AND EVENT_OBJECT_TABLE = 'chain_event_replay_audit'
      AND TRIGGER_NAME IN ('trg_chain_event_replay_audit_no_update','trg_chain_event_replay_audit_no_delete')
  ) <> 2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain event replay audit append-only triggers are incomplete';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_replay_audit();
DROP PROCEDURE jc_assert_chain_event_replay_audit;

-- END 2026_08_19_chain_event_replay_audit.sql

-- BEGIN 2026_08_20_ido_purchase_entitlement.sql
-- JoinCare IDO Sprint 1A-P2 · append-only V2 purchase project-token entitlement facts.
-- This migration extends 2026_08_15 and 2026_08_16; do not run it independently.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_ido_purchase_entitlement;
DELIMITER $$
CREATE PROCEDURE jc_preflight_ido_purchase_entitlement()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME IN ('projects','fund_ledger','chain_event_record')
      AND TABLE_TYPE = 'BASE TABLE'
  ) <> 3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'purchase entitlement prerequisite tables are missing';
  END IF;
  IF EXISTS (
    SELECT 1 FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ido_purchase_entitlement'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'pre-existing purchase entitlement table requires manual verification';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND (
      (TABLE_NAME = 'fund_ledger' AND COLUMN_NAME IN (
        'id','chain_id','contract_address','contract_version','project_id','wallet_address','counterparty_address',
        'entry_type','asset_role','token_symbol','token_address','token_decimals','amount_raw',
        'amount_display','tx_hash','log_index','block_number','block_hash','status','event_data'
      ))
      OR (TABLE_NAME = 'projects' AND COLUMN_NAME IN (
        'project_id','chain_id','contract_address','contract_version','payment_token',
        'payment_token_address','payment_token_decimals','token_symbol','token_address','token_decimals',
        'total_raised_raw','participant_count'
      ))
      OR (TABLE_NAME = 'chain_event_record' AND COLUMN_NAME IN ('event','contract_version'))
    )
  ) <> 34 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'purchase entitlement prerequisite columns are incomplete';
  END IF;
  IF EXISTS (
    SELECT 1 FROM fund_ledger
    WHERE contract_version = 'v2_stablecoin' AND entry_type = 'purchase_payment'
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'historical V2 purchases require canonical entitlement replay before migration';
  END IF;
  IF EXISTS (
    SELECT 1 FROM chain_event_record
    WHERE contract_version = 'v2_stablecoin' AND event = 'Purchased'
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'historical V2 Purchased records require canonical entitlement replay before migration';
  END IF;
  IF EXISTS (
    SELECT 1 FROM projects
    WHERE contract_version = 'v2_stablecoin'
      AND (total_raised_raw <> '0' OR participant_count <> 0)
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'historical V2 purchase-derived project state requires canonical entitlement replay before migration';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_ido_purchase_entitlement();
DROP PROCEDURE jc_preflight_ido_purchase_entitlement;

CREATE TABLE `ido_purchase_entitlement` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `fund_ledger_id` BIGINT UNSIGNED NOT NULL,
  `chain_id` BIGINT UNSIGNED NOT NULL,
  `contract_address` VARCHAR(42) NOT NULL,
  `contract_version` VARCHAR(32) NOT NULL,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `wallet_address` VARCHAR(42) NOT NULL,
  `payment_token_symbol` VARCHAR(20) NOT NULL,
  `payment_token_address` VARCHAR(42) NOT NULL,
  `payment_token_decimals` TINYINT UNSIGNED NOT NULL,
  `payment_amount_raw` VARCHAR(78) NOT NULL,
  `payment_amount_display` VARCHAR(160) NOT NULL,
  `project_token_symbol` VARCHAR(20) NOT NULL,
  `project_token_address` VARCHAR(42) NOT NULL,
  `project_token_decimals` TINYINT UNSIGNED NOT NULL,
  `project_token_amount_raw` VARCHAR(78) NOT NULL,
  `project_token_amount_display` VARCHAR(160) NOT NULL,
  `tx_hash` VARCHAR(66) NOT NULL,
  `log_index` INT UNSIGNED NOT NULL,
  `block_number` BIGINT UNSIGNED NOT NULL,
  `block_hash` VARCHAR(66) NOT NULL,
  `status` VARCHAR(20) NOT NULL DEFAULT 'canonical',
  `event_data` JSON NOT NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_ido_purchase_entitlement_ledger` (`fund_ledger_id`),
  UNIQUE KEY `uk_ido_purchase_entitlement_event` (`chain_id`,`tx_hash`,`log_index`),
  KEY `idx_ido_purchase_entitlement_project` (`chain_id`,`contract_address`,`project_id`),
  KEY `idx_ido_purchase_entitlement_wallet` (`chain_id`,`contract_address`,`project_id`,`wallet_address`),
  KEY `idx_ido_purchase_entitlement_block` (`chain_id`,`block_number`),
  CONSTRAINT `fk_ido_purchase_entitlement_ledger`
    FOREIGN KEY (`fund_ledger_id`) REFERENCES `fund_ledger` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `chk_ido_purchase_entitlement_positive_identity` CHECK (
    `fund_ledger_id` > 0 AND `chain_id` > 0 AND `project_id` > 0 AND `block_number` > 0
  ) ENFORCED,
  CONSTRAINT `chk_ido_purchase_entitlement_addresses` CHECK (
    `contract_address` REGEXP '^0x[0-9A-Fa-f]{40}$'
    AND `contract_address` <> '0x0000000000000000000000000000000000000000'
    AND `wallet_address` REGEXP '^0x[0-9A-Fa-f]{40}$'
    AND `wallet_address` <> '0x0000000000000000000000000000000000000000'
    AND `payment_token_address` REGEXP '^0x[0-9A-Fa-f]{40}$'
    AND `payment_token_address` <> '0x0000000000000000000000000000000000000000'
    AND `project_token_address` REGEXP '^0x[0-9A-Fa-f]{40}$'
    AND `project_token_address` <> '0x0000000000000000000000000000000000000000'
    AND LOWER(`payment_token_address`) <> LOWER(`project_token_address`)
  ) ENFORCED,
  CONSTRAINT `chk_ido_purchase_entitlement_tokens` CHECK (
    `payment_token_symbol` IN ('USDT','USDC')
    AND CHAR_LENGTH(TRIM(`project_token_symbol`)) > 0
    AND `payment_token_decimals` BETWEEN 1 AND 18
    AND `project_token_decimals` BETWEEN 1 AND 18
  ) ENFORCED,
  CONSTRAINT `chk_ido_purchase_entitlement_payment_raw` CHECK (
    `payment_amount_raw` REGEXP '^[1-9][0-9]{0,77}$'
    AND (
      CHAR_LENGTH(`payment_amount_raw`) < 78
      OR `payment_amount_raw` <= '115792089237316195423570985008687907853269984665640564039457584007913129639935'
    )
  ) ENFORCED,
  CONSTRAINT `chk_ido_purchase_entitlement_project_raw` CHECK (
    `project_token_amount_raw` REGEXP '^[1-9][0-9]{0,77}$'
    AND (
      CHAR_LENGTH(`project_token_amount_raw`) < 78
      OR `project_token_amount_raw` <= '115792089237316195423570985008687907853269984665640564039457584007913129639935'
    )
  ) ENFORCED,
  CONSTRAINT `chk_ido_purchase_entitlement_payment_display` CHECK (
    BINARY `payment_amount_display` = BINARY CASE
      WHEN CHAR_LENGTH(`payment_amount_raw`) <= `payment_token_decimals` THEN
        CONCAT('0.', TRIM(TRAILING '0' FROM LPAD(`payment_amount_raw`, `payment_token_decimals`, '0')))
      WHEN TRIM(TRAILING '0' FROM RIGHT(`payment_amount_raw`, `payment_token_decimals`)) = '' THEN
        LEFT(`payment_amount_raw`, CHAR_LENGTH(`payment_amount_raw`) - `payment_token_decimals`)
      ELSE CONCAT(
        LEFT(`payment_amount_raw`, CHAR_LENGTH(`payment_amount_raw`) - `payment_token_decimals`), '.',
        TRIM(TRAILING '0' FROM RIGHT(`payment_amount_raw`, `payment_token_decimals`))
      )
    END
  ) ENFORCED,
  CONSTRAINT `chk_ido_purchase_entitlement_project_display` CHECK (
    BINARY `project_token_amount_display` = BINARY CASE
      WHEN CHAR_LENGTH(`project_token_amount_raw`) <= `project_token_decimals` THEN
        CONCAT('0.', TRIM(TRAILING '0' FROM LPAD(`project_token_amount_raw`, `project_token_decimals`, '0')))
      WHEN TRIM(TRAILING '0' FROM RIGHT(`project_token_amount_raw`, `project_token_decimals`)) = '' THEN
        LEFT(`project_token_amount_raw`, CHAR_LENGTH(`project_token_amount_raw`) - `project_token_decimals`)
      ELSE CONCAT(
        LEFT(`project_token_amount_raw`, CHAR_LENGTH(`project_token_amount_raw`) - `project_token_decimals`), '.',
        TRIM(TRAILING '0' FROM RIGHT(`project_token_amount_raw`, `project_token_decimals`))
      )
    END
  ) ENFORCED,
  CONSTRAINT `chk_ido_purchase_entitlement_hashes` CHECK (
    `tx_hash` REGEXP '^0x[0-9A-Fa-f]{64}$' AND `block_hash` REGEXP '^0x[0-9A-Fa-f]{64}$'
  ) ENFORCED,
  CONSTRAINT `chk_ido_purchase_entitlement_version` CHECK (`contract_version` = 'v2_stablecoin') ENFORCED,
  CONSTRAINT `chk_ido_purchase_entitlement_status` CHECK (`status` = 'canonical') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DROP TRIGGER IF EXISTS `trg_ido_purchase_entitlement_no_update`;
DROP TRIGGER IF EXISTS `trg_ido_purchase_entitlement_no_delete`;
DROP TRIGGER IF EXISTS `trg_ido_purchase_entitlement_validate_insert`;
DELIMITER $$
CREATE TRIGGER `trg_ido_purchase_entitlement_validate_insert`
BEFORE INSERT ON `ido_purchase_entitlement`
FOR EACH ROW
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM `fund_ledger` f
    WHERE f.id = NEW.fund_ledger_id
      AND f.entry_type = 'purchase_payment'
      AND f.asset_role = 'payment'
      AND f.status = 'canonical'
      AND f.chain_id = NEW.chain_id
      AND LOWER(f.contract_address) = LOWER(NEW.contract_address)
      AND f.contract_version = NEW.contract_version
      AND f.project_id = NEW.project_id
      AND LOWER(f.wallet_address) = LOWER(NEW.wallet_address)
      AND LOWER(f.counterparty_address) = LOWER(NEW.contract_address)
      AND f.token_symbol = NEW.payment_token_symbol
      AND LOWER(f.token_address) = LOWER(NEW.payment_token_address)
      AND f.token_decimals = NEW.payment_token_decimals
      AND BINARY f.amount_raw = BINARY NEW.payment_amount_raw
      AND BINARY f.amount_display = BINARY NEW.payment_amount_display
      AND LOWER(f.tx_hash) = LOWER(NEW.tx_hash)
      AND f.log_index = NEW.log_index
      AND f.block_number = NEW.block_number
      AND LOWER(f.block_hash) = LOWER(NEW.block_hash)
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'purchase entitlement parent ledger identity mismatch';
  END IF;
END$$
CREATE TRIGGER `trg_ido_purchase_entitlement_no_update`
BEFORE UPDATE ON `ido_purchase_entitlement`
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'IDO purchase entitlement rows are append-only';
END$$
CREATE TRIGGER `trg_ido_purchase_entitlement_no_delete`
BEFORE DELETE ON `ido_purchase_entitlement`
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'IDO purchase entitlement rows are append-only';
END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS jc_assert_ido_purchase_entitlement;
DELIMITER $$
CREATE PROCEDURE jc_assert_ido_purchase_entitlement()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ido_purchase_entitlement' AND (
      (COLUMN_NAME IN ('id','fund_ledger_id','chain_id','project_id','block_number') AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME IN ('contract_address','wallet_address','payment_token_address','project_token_address') AND COLUMN_TYPE = 'varchar(42)' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME = 'contract_version' AND COLUMN_TYPE = 'varchar(32)' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME IN ('payment_token_symbol','project_token_symbol') AND COLUMN_TYPE = 'varchar(20)' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME IN ('payment_token_decimals','project_token_decimals') AND COLUMN_TYPE = 'tinyint unsigned' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME IN ('payment_amount_raw','project_token_amount_raw') AND COLUMN_TYPE = 'varchar(78)' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME IN ('payment_amount_display','project_token_amount_display') AND COLUMN_TYPE = 'varchar(160)' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME IN ('tx_hash','block_hash') AND COLUMN_TYPE = 'varchar(66)' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME = 'log_index' AND COLUMN_TYPE = 'int unsigned' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME = 'status' AND COLUMN_TYPE = 'varchar(20)' AND IS_NULLABLE = 'NO' AND COLUMN_DEFAULT = 'canonical')
      OR (COLUMN_NAME = 'event_data' AND DATA_TYPE = 'json' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME = 'created_at' AND COLUMN_TYPE = 'datetime(6)' AND IS_NULLABLE = 'NO')
    )
  ) <> 24 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'purchase entitlement columns are incompatible';
  END IF;
  IF (
    SELECT COUNT(*) FROM (
      SELECT INDEX_NAME, GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') AS columns_in_order, MIN(NON_UNIQUE) AS non_unique
      FROM information_schema.STATISTICS
      WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ido_purchase_entitlement'
      GROUP BY INDEX_NAME
    ) entitlement_indexes
    WHERE non_unique = 0 AND columns_in_order IN ('fund_ledger_id','chain_id,tx_hash,log_index')
  ) <> 2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'purchase entitlement unique identities are incomplete';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'ido_purchase_entitlement'
      AND CONSTRAINT_TYPE = 'CHECK' AND ENFORCED = 'YES'
      AND CONSTRAINT_NAME IN (
        'chk_ido_purchase_entitlement_positive_identity','chk_ido_purchase_entitlement_addresses',
        'chk_ido_purchase_entitlement_tokens','chk_ido_purchase_entitlement_payment_raw',
        'chk_ido_purchase_entitlement_project_raw','chk_ido_purchase_entitlement_payment_display',
        'chk_ido_purchase_entitlement_project_display','chk_ido_purchase_entitlement_hashes',
        'chk_ido_purchase_entitlement_version','chk_ido_purchase_entitlement_status'
      )
  ) <> 10 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'purchase entitlement checks are incomplete';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.REFERENTIAL_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'ido_purchase_entitlement'
      AND CONSTRAINT_NAME = 'fk_ido_purchase_entitlement_ledger'
      AND REFERENCED_TABLE_NAME = 'fund_ledger' AND DELETE_RULE = 'RESTRICT'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'purchase entitlement ledger foreign key is missing';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.TRIGGERS
    WHERE TRIGGER_SCHEMA = DATABASE() AND EVENT_OBJECT_TABLE = 'ido_purchase_entitlement'
      AND TRIGGER_NAME IN ('trg_ido_purchase_entitlement_validate_insert','trg_ido_purchase_entitlement_no_update','trg_ido_purchase_entitlement_no_delete')
  ) <> 3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'purchase entitlement validation/append-only triggers are incomplete';
  END IF;
  IF EXISTS (
    SELECT 1 FROM fund_ledger
    WHERE contract_version = 'v2_stablecoin' AND entry_type = 'purchase_payment'
    LIMIT 1
  ) OR EXISTS (
    SELECT 1 FROM chain_event_record
    WHERE contract_version = 'v2_stablecoin' AND event = 'Purchased'
    LIMIT 1
  ) OR EXISTS (
    SELECT 1 FROM projects
    WHERE contract_version = 'v2_stablecoin'
      AND (total_raised_raw <> '0' OR participant_count <> 0)
    LIMIT 1
  ) OR EXISTS (
    SELECT 1 FROM ido_purchase_entitlement LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'purchase entitlement migration history invariant failed';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_ido_purchase_entitlement();
DROP PROCEDURE jc_assert_ido_purchase_entitlement;

-- Verification:
--   SHOW CREATE TABLE ido_purchase_entitlement;
--   SELECT COUNT(*) FROM fund_ledger WHERE entry_type='purchase_payment';
--   SELECT COUNT(*) FROM ido_purchase_entitlement;

-- END 2026_08_20_ido_purchase_entitlement.sql

-- BEGIN 2026_08_21_ido_reconciliation_evidence.sql
-- JoinCare IDO Sprint 1A-P2 · durable operational on-chain reconciliation evidence.
-- This is database-local evidence (digest chained), not an external KMS/WORM attestation.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_ido_reconciliation_evidence;
DELIMITER $$
CREATE PROCEDURE jc_preflight_ido_reconciliation_evidence()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
      'projects','fund_ledger','ido_purchase_entitlement','chain_event_sync_state','chain_event_failure','chain_tx_submission'
    ) AND TABLE_TYPE='BASE TABLE'
  ) <> 6 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation prerequisite tables are missing';
  END IF;
  IF EXISTS (
    SELECT 1 FROM information_schema.TABLES
    WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
      'ido_reconciliation_run','ido_reconciliation_job','ido_reconciliation_event',
      'ido_reconciliation_wallet','ido_reconciliation_check','ido_reconciliation_manifest'
    )
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='pre-existing IDO reconciliation evidence tables require manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_ido_reconciliation_evidence();
DROP PROCEDURE jc_preflight_ido_reconciliation_evidence;

CREATE TABLE `ido_reconciliation_run` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `run_uuid` CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `chain_id` BIGINT UNSIGNED NOT NULL,
  `contract_address` VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `requested_by_admin_id` BIGINT UNSIGNED NOT NULL,
  `requested_role_codes` VARCHAR(500) NOT NULL,
  `reason` VARCHAR(500) NOT NULL,
  `idempotency_key` VARCHAR(100) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `algorithm_version` VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `canonicalization_version` VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `evidence_block_number` BIGINT UNSIGNED NOT NULL,
  `evidence_block_hash` VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `implementation_address` VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `implementation_code_hash` VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `confirmation_depth` BIGINT UNSIGNED NOT NULL,
  `max_healthy_lag_blocks` BIGINT UNSIGNED NOT NULL,
  `expected_wallet_count` INT UNSIGNED NOT NULL,
  `expected_check_count` INT UNSIGNED NOT NULL,
  `expected_wallet_identity_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `expected_check_identity_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `source_payload` MEDIUMBLOB NOT NULL,
  `source_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `requested_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_ido_reconciliation_run_uuid` (`run_uuid`),
  UNIQUE KEY `uk_ido_reconciliation_request_idempotency` (`requested_by_admin_id`,`idempotency_key`),
  KEY `idx_ido_reconciliation_project_time` (`project_id`,`requested_at`,`id`),
  CONSTRAINT `chk_ido_reconciliation_run_identity` CHECK (`project_id`>0 AND `chain_id`>0 AND `requested_by_admin_id`>0 AND `evidence_block_number`>0) ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_run_addresses` CHECK (
    `contract_address` REGEXP '^0x[0-9a-f]{40}$' AND `contract_address`<>'0x0000000000000000000000000000000000000000'
    AND `implementation_address` REGEXP '^0x[0-9a-f]{40}$' AND `implementation_address`<>'0x0000000000000000000000000000000000000000'
  ) ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_run_hashes` CHECK (
    `evidence_block_hash` REGEXP '^0x[0-9a-f]{64}$' AND `implementation_code_hash` REGEXP '^0x[0-9a-f]{64}$'
    AND `source_digest` REGEXP '^[0-9a-f]{64}$'
    AND `expected_wallet_identity_digest` REGEXP '^[0-9a-f]{64}$'
    AND `expected_check_identity_digest` REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_run_request` CHECK (
    `run_uuid` REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
    AND CHAR_LENGTH(TRIM(`reason`)) BETWEEN 3 AND 500
    AND CHAR_LENGTH(`idempotency_key`) BETWEEN 16 AND 100
    AND CHAR_LENGTH(`requested_role_codes`) BETWEEN 1 AND 500
  ) ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_run_versions` CHECK (
    `algorithm_version`='joincare-ido-reconciliation-v1'
    AND `canonicalization_version`='canonical-json-v1'
    AND `max_healthy_lag_blocks`>0
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `ido_reconciliation_job` (
  `run_id` BIGINT UNSIGNED NOT NULL,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `state` VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'queued',
  `outcome` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NULL,
  `active_slot` TINYINT UNSIGNED NULL DEFAULT 1,
  `attempt_count` INT UNSIGNED NOT NULL DEFAULT 0,
  `consecutive_error_count` INT UNSIGNED NOT NULL DEFAULT 0,
  `next_wallet_ordinal` INT UNSIGNED NOT NULL DEFAULT 0,
  `next_attempt_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `lease_token` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  `lease_expires_at` DATETIME(6) NULL,
  `last_error_code` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  `last_error_message` VARCHAR(500) NOT NULL DEFAULT '',
  `cancel_requested_by_admin_id` BIGINT UNSIGNED NULL,
  `cancel_reason` VARCHAR(500) NULL,
  `cancel_requested_at` DATETIME(6) NULL,
  `started_at` DATETIME(6) NULL,
  `completed_at` DATETIME(6) NULL,
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`run_id`),
  UNIQUE KEY `uk_ido_reconciliation_active_project` (`project_id`,`active_slot`),
  KEY `idx_ido_reconciliation_job_claim` (`state`,`next_attempt_at`,`lease_expires_at`,`run_id`),
  KEY `idx_ido_reconciliation_job_lease` (`lease_expires_at`,`state`),
  CONSTRAINT `fk_ido_reconciliation_job_run` FOREIGN KEY (`run_id`) REFERENCES `ido_reconciliation_run` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `chk_ido_reconciliation_job_state` CHECK (`state` IN ('queued','running','retrying','cancel_requested','finalizing','completed','failed','invalidated','cancelled')) ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_job_outcome` CHECK (
    (`state`='completed' AND `outcome` IN ('BALANCED','UNBALANCED') AND `active_slot` IS NULL AND `completed_at` IS NOT NULL)
    OR (`state`<>'completed' AND `outcome` IS NULL)
  ) ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_job_active` CHECK (
    (`state` IN ('queued','running','retrying','cancel_requested','finalizing') AND `active_slot`=1 AND `completed_at` IS NULL)
    OR (`state` IN ('completed','failed','invalidated','cancelled') AND `active_slot` IS NULL AND `completed_at` IS NOT NULL)
  ) ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_job_lease_fields` CHECK (
    (`state` IN ('running','cancel_requested','finalizing') AND `lease_token` REGEXP '^[0-9a-f]{64}$' AND `lease_expires_at` IS NOT NULL)
    OR (`state` NOT IN ('running','cancel_requested','finalizing') AND `lease_token`='' AND `lease_expires_at` IS NULL)
  ) ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_job_cancel` CHECK (
    (`cancel_requested_by_admin_id` IS NULL AND `cancel_reason` IS NULL AND `cancel_requested_at` IS NULL)
    OR (`cancel_requested_by_admin_id`>0 AND CHAR_LENGTH(TRIM(`cancel_reason`)) BETWEEN 3 AND 500 AND `cancel_requested_at` IS NOT NULL)
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `ido_reconciliation_event` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `run_id` BIGINT UNSIGNED NOT NULL,
  `sequence_no` INT UNSIGNED NOT NULL,
  `event_type` VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `stage` VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `error_code` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  `message` VARCHAR(500) NOT NULL DEFAULT '',
  `actor_admin_id` BIGINT UNSIGNED NULL,
  `lease_token_hash` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  `previous_event_hash` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `event_hash` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_ido_reconciliation_event_sequence` (`run_id`,`sequence_no`),
  UNIQUE KEY `uk_ido_reconciliation_event_hash` (`event_hash`),
  KEY `idx_ido_reconciliation_event_type` (`run_id`,`event_type`,`created_at`),
  CONSTRAINT `fk_ido_reconciliation_event_run` FOREIGN KEY (`run_id`) REFERENCES `ido_reconciliation_run` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `chk_ido_reconciliation_event_fields` CHECK (
    `sequence_no`>0 AND `event_type` IN ('QUEUED','STARTED','PAGE_COMPLETED','RETRY_SCHEDULED','CANCEL_REQUESTED','CANCELLED','FAILED','INVALIDATED','FINALIZED')
    AND CHAR_LENGTH(`stage`) BETWEEN 1 AND 32
    AND `previous_event_hash` REGEXP '^[0-9a-f]{64}$' AND `event_hash` REGEXP '^[0-9a-f]{64}$'
    AND (`lease_token_hash`='' OR `lease_token_hash` REGEXP '^[0-9a-f]{64}$')
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `ido_reconciliation_wallet` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `run_id` BIGINT UNSIGNED NOT NULL,
  `ordinal` INT UNSIGNED NOT NULL,
  `wallet_address` VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `expected_payment_raw` VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `actual_payment_raw` VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `expected_tokens_raw` VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `actual_tokens_raw` VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `expected_claimed_raw` VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `actual_claimed_raw` VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `expected_refunded` TINYINT(1) NOT NULL,
  `actual_refunded` TINYINT(1) NOT NULL,
  `previous_row_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `identity_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `row_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `observed_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_ido_reconciliation_wallet_ordinal` (`run_id`,`ordinal`),
  UNIQUE KEY `uk_ido_reconciliation_wallet_address` (`run_id`,`wallet_address`),
  KEY `idx_ido_reconciliation_wallet_lookup` (`wallet_address`,`run_id`),
  CONSTRAINT `fk_ido_reconciliation_wallet_run` FOREIGN KEY (`run_id`) REFERENCES `ido_reconciliation_run` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `chk_ido_reconciliation_wallet_identity` CHECK (`wallet_address` REGEXP '^0x[0-9a-f]{40}$' AND `wallet_address`<>'0x0000000000000000000000000000000000000000') ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_wallet_raw` CHECK (
    `expected_payment_raw` REGEXP '^(0|[1-9][0-9]{0,77})$' AND `actual_payment_raw` REGEXP '^(0|[1-9][0-9]{0,77})$'
    AND `expected_tokens_raw` REGEXP '^(0|[1-9][0-9]{0,77})$' AND `actual_tokens_raw` REGEXP '^(0|[1-9][0-9]{0,77})$'
    AND `expected_claimed_raw` REGEXP '^(0|[1-9][0-9]{0,77})$' AND `actual_claimed_raw` REGEXP '^(0|[1-9][0-9]{0,77})$'
    AND (CHAR_LENGTH(`expected_payment_raw`)<78 OR `expected_payment_raw`<='115792089237316195423570985008687907853269984665640564039457584007913129639935')
    AND (CHAR_LENGTH(`actual_payment_raw`)<78 OR `actual_payment_raw`<='115792089237316195423570985008687907853269984665640564039457584007913129639935')
    AND (CHAR_LENGTH(`expected_tokens_raw`)<78 OR `expected_tokens_raw`<='115792089237316195423570985008687907853269984665640564039457584007913129639935')
    AND (CHAR_LENGTH(`actual_tokens_raw`)<78 OR `actual_tokens_raw`<='115792089237316195423570985008687907853269984665640564039457584007913129639935')
    AND (CHAR_LENGTH(`expected_claimed_raw`)<78 OR `expected_claimed_raw`<='115792089237316195423570985008687907853269984665640564039457584007913129639935')
    AND (CHAR_LENGTH(`actual_claimed_raw`)<78 OR `actual_claimed_raw`<='115792089237316195423570985008687907853269984665640564039457584007913129639935')
    AND `expected_refunded` IN (0,1) AND `actual_refunded` IN (0,1)
    AND `previous_row_digest` REGEXP '^[0-9a-f]{64}$' AND `identity_digest` REGEXP '^[0-9a-f]{64}$'
    AND `row_digest` REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `ido_reconciliation_manifest` (
  `run_id` BIGINT UNSIGNED NOT NULL,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `outcome` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `wallet_count` INT UNSIGNED NOT NULL,
  `check_count` INT UNSIGNED NOT NULL,
  `failed_check_count` INT UNSIGNED NOT NULL,
  `payload` MEDIUMBLOB NOT NULL,
  `payload_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `wallet_evidence_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `check_evidence_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `previous_manifest_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `manifest_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `completed_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`run_id`),
  UNIQUE KEY `uk_ido_reconciliation_manifest_digest` (`manifest_digest`),
  UNIQUE KEY `uk_ido_reconciliation_manifest_parent` (`project_id`,`previous_manifest_digest`),
  KEY `idx_ido_reconciliation_manifest_project` (`project_id`,`run_id`),
  CONSTRAINT `fk_ido_reconciliation_manifest_run` FOREIGN KEY (`run_id`) REFERENCES `ido_reconciliation_run` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `chk_ido_reconciliation_manifest_outcome` CHECK (
    (`outcome`='BALANCED' AND `failed_check_count`=0 AND `check_count`>0)
    OR (`outcome`='UNBALANCED' AND `failed_check_count`>0 AND `failed_check_count`<=`check_count`)
  ) ENFORCED,
  CONSTRAINT `chk_ido_reconciliation_manifest_hashes` CHECK (
    `payload_digest` REGEXP '^[0-9a-f]{64}$' AND `wallet_evidence_digest` REGEXP '^[0-9a-f]{64}$'
    AND `check_evidence_digest` REGEXP '^[0-9a-f]{64}$' AND `previous_manifest_digest` REGEXP '^[0-9a-f]{64}$'
    AND `manifest_digest` REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `ido_reconciliation_check` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `run_id` BIGINT UNSIGNED NOT NULL,
  `ordinal` INT UNSIGNED NOT NULL,
  `scope` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `code` VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `ok` TINYINT(1) NOT NULL,
  `expected_exact` VARCHAR(500) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `actual_exact` VARCHAR(500) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `message` VARCHAR(500) NOT NULL,
  `previous_row_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `identity_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `row_digest` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_ido_reconciliation_check_ordinal` (`run_id`,`ordinal`),
  UNIQUE KEY `uk_ido_reconciliation_check_code` (`run_id`,`scope`,`code`),
  CONSTRAINT `fk_ido_reconciliation_check_run` FOREIGN KEY (`run_id`) REFERENCES `ido_reconciliation_run` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `chk_ido_reconciliation_check_fields` CHECK (
    `scope` IN ('database','onchain') AND CHAR_LENGTH(`code`) BETWEEN 1 AND 160 AND `ok` IN (0,1)
    AND `previous_row_digest` REGEXP '^[0-9a-f]{64}$' AND `identity_digest` REGEXP '^[0-9a-f]{64}$'
    AND `row_digest` REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DROP TRIGGER IF EXISTS trg_ido_reconciliation_run_no_update;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_run_no_delete;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_run_validate_insert;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_event_validate_insert;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_event_no_update;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_event_no_delete;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_wallet_validate_insert;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_wallet_no_update;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_wallet_no_delete;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_manifest_validate_insert;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_manifest_no_update;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_manifest_no_delete;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_job_validate_update;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_job_no_delete;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_job_validate_insert;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_check_validate_insert;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_check_no_update;
DROP TRIGGER IF EXISTS trg_ido_reconciliation_check_no_delete;
DELIMITER $$
CREATE TRIGGER trg_ido_reconciliation_run_validate_insert BEFORE INSERT ON ido_reconciliation_run FOR EACH ROW
BEGIN
  IF JSON_VALID(CAST(NEW.source_payload AS CHAR))=0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation source payload is not valid JSON';
  END IF;
  IF NEW.source_digest<>LOWER(SHA2(NEW.source_payload,256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation source digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_ido_reconciliation_run_no_update BEFORE UPDATE ON ido_reconciliation_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation run rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_run_no_delete BEFORE DELETE ON ido_reconciliation_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation run rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_event_no_update BEFORE UPDATE ON ido_reconciliation_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation event rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_event_no_delete BEFORE DELETE ON ido_reconciliation_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation event rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_event_validate_insert BEFORE INSERT ON ido_reconciliation_event FOR EACH ROW
BEGIN
  DECLARE prior_sequence INT UNSIGNED DEFAULT 0;
  DECLARE prior_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  DECLARE current_state VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin;
  IF EXISTS (SELECT 1 FROM ido_reconciliation_event WHERE run_id=NEW.run_id AND event_type IN ('CANCELLED','FAILED','INVALIDATED','FINALIZED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='terminal IDO reconciliation event already exists';
  END IF;
  SELECT state INTO current_state FROM ido_reconciliation_job WHERE run_id=NEW.run_id;
  IF current_state IS NULL OR NOT ((NEW.event_type='QUEUED' AND current_state='queued')
       OR (NEW.event_type IN ('STARTED','PAGE_COMPLETED') AND current_state='running')
       OR (NEW.event_type='RETRY_SCHEDULED' AND current_state='retrying')
       OR (NEW.event_type='CANCEL_REQUESTED' AND current_state='cancel_requested')
       OR (NEW.event_type='CANCELLED' AND current_state='cancelled')
       OR (NEW.event_type='FAILED' AND current_state='failed')
       OR (NEW.event_type='INVALIDATED' AND current_state='invalidated')
       OR (NEW.event_type='FINALIZED' AND current_state='completed')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation event does not match job state';
  END IF;
  SET prior_sequence=COALESCE((SELECT sequence_no FROM ido_reconciliation_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),0);
  SET prior_hash=COALESCE((SELECT event_hash FROM ido_reconciliation_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),REPEAT('0',64));
  IF NEW.sequence_no<>prior_sequence+1 OR NEW.previous_event_hash<>prior_hash THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation event chain is not contiguous';
  END IF;
  IF NEW.event_hash<>LOWER(SHA2(CONCAT(
    'joincare.ido.reconciliation.event.v1',CHAR(10),NEW.run_id,CHAR(10),NEW.sequence_no,CHAR(10),
    NEW.event_type,CHAR(10),NEW.stage,CHAR(10),NEW.error_code,CHAR(10),NEW.message,CHAR(10),
    COALESCE(NEW.actor_admin_id,0),CHAR(10),NEW.lease_token_hash,CHAR(10),NEW.previous_event_hash
  ),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation event digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_ido_reconciliation_wallet_validate_insert BEFORE INSERT ON ido_reconciliation_wallet FOR EACH ROW
BEGIN
  DECLARE prior_ordinal INT UNSIGNED DEFAULT 0;
  DECLARE prior_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  DECLARE prior_identity_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  IF NOT EXISTS (SELECT 1 FROM ido_reconciliation_job j WHERE j.run_id=NEW.run_id AND j.state IN ('running','finalizing') AND j.lease_expires_at>CURRENT_TIMESTAMP(6))
     OR EXISTS (SELECT 1 FROM ido_reconciliation_manifest m WHERE m.run_id=NEW.run_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='wallet evidence requires an active unfinalized reconciliation job';
  END IF;
  SET prior_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM ido_reconciliation_wallet WHERE run_id=NEW.run_id),0);
  SET prior_hash=COALESCE((SELECT row_digest FROM ido_reconciliation_wallet WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET prior_identity_hash=COALESCE((SELECT identity_digest FROM ido_reconciliation_wallet WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  IF NEW.ordinal<>prior_ordinal OR NEW.previous_row_digest<>prior_hash THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation wallet evidence chain is not contiguous';
  END IF;
  IF NEW.identity_digest<>LOWER(SHA2(CONCAT(
    'joincare.ido.reconciliation.wallet-identity.v1',CHAR(10),NEW.ordinal,CHAR(10),NEW.wallet_address,CHAR(10),
    NEW.expected_payment_raw,CHAR(10),NEW.expected_tokens_raw,CHAR(10),NEW.expected_claimed_raw,CHAR(10),
    NEW.expected_refunded,CHAR(10),prior_identity_hash
  ),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation wallet identity digest mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT(
    'joincare.ido.reconciliation.wallet.v1',CHAR(10),NEW.run_id,CHAR(10),NEW.ordinal,CHAR(10),
    NEW.wallet_address,CHAR(10),NEW.expected_payment_raw,CHAR(10),NEW.actual_payment_raw,CHAR(10),
    NEW.expected_tokens_raw,CHAR(10),NEW.actual_tokens_raw,CHAR(10),NEW.expected_claimed_raw,CHAR(10),
    NEW.actual_claimed_raw,CHAR(10),NEW.expected_refunded,CHAR(10),NEW.actual_refunded,CHAR(10),NEW.previous_row_digest
  ),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation wallet digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_ido_reconciliation_wallet_no_update BEFORE UPDATE ON ido_reconciliation_wallet FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation wallet rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_wallet_no_delete BEFORE DELETE ON ido_reconciliation_wallet FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation wallet rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_check_validate_insert BEFORE INSERT ON ido_reconciliation_check FOR EACH ROW
BEGIN
  DECLARE prior_ordinal INT UNSIGNED DEFAULT 0;
  DECLARE prior_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  DECLARE prior_identity_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  IF NOT EXISTS (SELECT 1 FROM ido_reconciliation_job j WHERE j.run_id=NEW.run_id AND j.state='finalizing' AND j.lease_expires_at>CURRENT_TIMESTAMP(6))
     OR EXISTS (SELECT 1 FROM ido_reconciliation_manifest m WHERE m.run_id=NEW.run_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='check evidence requires a finalizing unfinalized reconciliation job';
  END IF;
  SET prior_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM ido_reconciliation_check WHERE run_id=NEW.run_id),0);
  SET prior_hash=COALESCE((SELECT row_digest FROM ido_reconciliation_check WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET prior_identity_hash=COALESCE((SELECT identity_digest FROM ido_reconciliation_check WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  IF NEW.ordinal<>prior_ordinal OR NEW.previous_row_digest<>prior_hash THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation check evidence chain is not contiguous';
  END IF;
  IF NEW.identity_digest<>LOWER(SHA2(CONCAT(
    'joincare.ido.reconciliation.check-identity.v1',CHAR(10),NEW.ordinal,CHAR(10),NEW.scope,CHAR(10),NEW.code,CHAR(10),prior_identity_hash
  ),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation check identity digest mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT(
    'joincare.ido.reconciliation.check.v1',CHAR(10),NEW.run_id,CHAR(10),NEW.ordinal,CHAR(10),
    NEW.scope,CHAR(10),NEW.code,CHAR(10),NEW.ok,CHAR(10),NEW.expected_exact,CHAR(10),NEW.actual_exact,CHAR(10),NEW.message,CHAR(10),NEW.previous_row_digest
  ),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation check digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_ido_reconciliation_check_no_update BEFORE UPDATE ON ido_reconciliation_check FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation check rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_check_no_delete BEFORE DELETE ON ido_reconciliation_check FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation check rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_manifest_validate_insert BEFORE INSERT ON ido_reconciliation_manifest FOR EACH ROW
BEGIN
  DECLARE expected_wallets INT UNSIGNED;
  DECLARE actual_wallets INT UNSIGNED;
  DECLARE actual_checks INT UNSIGNED;
  DECLARE actual_failed_checks INT UNSIGNED;
  DECLARE wallet_failed_checks INT UNSIGNED;
  DECLARE last_wallet_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  DECLARE last_check_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  DECLARE last_wallet_identity_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  DECLARE last_check_identity_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  DECLARE expected_checks INT UNSIGNED;
  DECLARE expected_wallet_identity_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin;
  DECLARE expected_check_identity_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin;
  DECLARE prior_manifest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT REPEAT('0',64);
  SELECT r.expected_wallet_count,r.expected_check_count,r.expected_wallet_identity_digest,r.expected_check_identity_digest
    INTO expected_wallets,expected_checks,expected_wallet_identity_digest,expected_check_identity_digest FROM ido_reconciliation_run r
    JOIN ido_reconciliation_job j ON j.run_id=r.id
    WHERE r.id=NEW.run_id AND r.project_id=NEW.project_id AND j.state='finalizing' AND j.lease_expires_at>CURRENT_TIMESTAMP(6) FOR UPDATE;
  IF expected_wallets IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='manifest requires finalizing reconciliation job'; END IF;
  SELECT COUNT(*) INTO actual_wallets FROM ido_reconciliation_wallet WHERE run_id=NEW.run_id;
  IF actual_wallets<>expected_wallets OR NEW.wallet_count<>expected_wallets THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='manifest wallet evidence is incomplete';
  END IF;
  SELECT COALESCE(SUM(
    (expected_payment_raw<>actual_payment_raw)+(expected_tokens_raw<>actual_tokens_raw)+
    (expected_claimed_raw<>actual_claimed_raw)+(expected_refunded<>actual_refunded)
  ),0) INTO wallet_failed_checks FROM ido_reconciliation_wallet WHERE run_id=NEW.run_id;
  SET last_wallet_digest=COALESCE((SELECT row_digest FROM ido_reconciliation_wallet WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET last_wallet_identity_digest=COALESCE((SELECT identity_digest FROM ido_reconciliation_wallet WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  IF NEW.wallet_evidence_digest<>last_wallet_digest THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='manifest wallet evidence digest does not match';
  END IF;
  IF last_wallet_identity_digest<>expected_wallet_identity_digest THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='manifest wallet identity set does not match frozen source';
  END IF;
  SELECT COUNT(*),COALESCE(SUM(CASE WHEN ok=0 THEN 1 ELSE 0 END),0) INTO actual_checks,actual_failed_checks
    FROM ido_reconciliation_check WHERE run_id=NEW.run_id;
  IF actual_checks+(actual_wallets*4)<>NEW.check_count OR NEW.check_count<>expected_checks
     OR actual_failed_checks+wallet_failed_checks<>NEW.failed_check_count THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='manifest check evidence counts do not match';
  END IF;
  SET last_check_digest=COALESCE((SELECT row_digest FROM ido_reconciliation_check WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET last_check_identity_digest=COALESCE((SELECT identity_digest FROM ido_reconciliation_check WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  IF NEW.check_evidence_digest<>last_check_digest THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='manifest check evidence digest does not match';
  END IF;
  IF last_check_identity_digest<>expected_check_identity_digest THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='manifest check identity set does not match frozen source';
  END IF;
  IF NOT ((NEW.outcome='BALANCED' AND NEW.failed_check_count=0 AND NEW.check_count>0)
       OR (NEW.outcome='UNBALANCED' AND NEW.failed_check_count>0 AND NEW.failed_check_count<=NEW.check_count)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='manifest outcome does not match check counts';
  END IF;
  IF NEW.payload_digest<>LOWER(SHA2(NEW.payload,256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation manifest payload digest mismatch';
  END IF;
  IF JSON_VALID(CAST(NEW.payload AS CHAR))=0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation manifest payload is not valid JSON';
  END IF;
  SET prior_manifest=COALESCE((SELECT manifest_digest FROM ido_reconciliation_manifest
    WHERE project_id=NEW.project_id ORDER BY run_id DESC LIMIT 1),REPEAT('0',64));
  IF NEW.previous_manifest_digest<>prior_manifest THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation manifest chain mismatch';
  END IF;
  IF NEW.manifest_digest<>LOWER(SHA2(CONCAT(
    'joincare.ido.reconciliation.manifest.v1',CHAR(10),NEW.run_id,CHAR(10),NEW.project_id,CHAR(10),
    NEW.outcome,CHAR(10),NEW.wallet_count,CHAR(10),NEW.check_count,CHAR(10),NEW.failed_check_count,CHAR(10),
    NEW.payload_digest,CHAR(10),NEW.wallet_evidence_digest,CHAR(10),NEW.check_evidence_digest,CHAR(10),NEW.previous_manifest_digest
  ),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation manifest digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_ido_reconciliation_manifest_no_update BEFORE UPDATE ON ido_reconciliation_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation manifest rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_manifest_no_delete BEFORE DELETE ON ido_reconciliation_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation manifest rows are append-only'; END$$
CREATE TRIGGER trg_ido_reconciliation_job_validate_update BEFORE UPDATE ON ido_reconciliation_job FOR EACH ROW
BEGIN
  IF NEW.run_id<>OLD.run_id OR NEW.project_id<>OLD.project_id OR NEW.attempt_count<OLD.attempt_count OR NEW.next_wallet_ordinal<OLD.next_wallet_ordinal THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation job identity or monotonic progress changed';
  END IF;
  IF OLD.state IN ('completed','failed','invalidated','cancelled') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='terminal IDO reconciliation jobs are immutable';
  END IF;
  IF NOT (
    (OLD.state='queued' AND NEW.state IN ('running','cancelled'))
    OR (OLD.state='running' AND NEW.state IN ('running','retrying','cancel_requested','finalizing','failed','invalidated'))
    OR (OLD.state='retrying' AND NEW.state IN ('running','cancelled'))
    OR (OLD.state='cancel_requested' AND NEW.state IN ('cancel_requested','cancelled'))
    OR (OLD.state='finalizing' AND NEW.state IN ('retrying','completed','failed','invalidated'))
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='invalid IDO reconciliation job state transition';
  END IF;
  IF NEW.state='completed' AND NOT EXISTS (
    SELECT 1 FROM ido_reconciliation_manifest m WHERE m.run_id=NEW.run_id AND m.outcome=NEW.outcome
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='completed reconciliation job requires matching manifest';
  END IF;
END$$
CREATE TRIGGER trg_ido_reconciliation_job_validate_insert BEFORE INSERT ON ido_reconciliation_job FOR EACH ROW
BEGIN
  IF NEW.state<>'queued' OR NEW.outcome IS NOT NULL OR NEW.active_slot<>1 OR NEW.attempt_count<>0
     OR NEW.consecutive_error_count<>0 OR NEW.next_wallet_ordinal<>0 OR NEW.lease_token<>''
     OR NEW.lease_expires_at IS NOT NULL OR NEW.cancel_requested_by_admin_id IS NOT NULL
     OR NEW.cancel_reason IS NOT NULL OR NEW.cancel_requested_at IS NOT NULL
     OR NEW.last_error_code<>'' OR NEW.last_error_message<>''
     OR NEW.started_at IS NOT NULL OR NEW.completed_at IS NOT NULL
     OR NOT EXISTS (SELECT 1 FROM ido_reconciliation_run r WHERE r.id=NEW.run_id AND r.project_id=NEW.project_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation job must start in canonical queued state';
  END IF;
END$$
CREATE TRIGGER trg_ido_reconciliation_job_no_delete BEFORE DELETE ON ido_reconciliation_job FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation job rows cannot be deleted'; END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS jc_assert_ido_reconciliation_evidence;
DELIMITER $$
CREATE PROCEDURE jc_assert_ido_reconciliation_evidence()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
    'ido_reconciliation_run','ido_reconciliation_job','ido_reconciliation_event',
    'ido_reconciliation_wallet','ido_reconciliation_check','ido_reconciliation_manifest'
  ) AND TABLE_TYPE='BASE TABLE')<>6 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation evidence tables are incomplete'; END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN (
    'trg_ido_reconciliation_run_validate_insert',
    'trg_ido_reconciliation_run_no_update','trg_ido_reconciliation_run_no_delete',
    'trg_ido_reconciliation_event_validate_insert',
    'trg_ido_reconciliation_event_no_update','trg_ido_reconciliation_event_no_delete',
    'trg_ido_reconciliation_wallet_validate_insert','trg_ido_reconciliation_wallet_no_update','trg_ido_reconciliation_wallet_no_delete',
    'trg_ido_reconciliation_check_validate_insert','trg_ido_reconciliation_check_no_update','trg_ido_reconciliation_check_no_delete',
    'trg_ido_reconciliation_manifest_validate_insert','trg_ido_reconciliation_manifest_no_update','trg_ido_reconciliation_manifest_no_delete',
    'trg_ido_reconciliation_job_validate_insert','trg_ido_reconciliation_job_validate_update','trg_ido_reconciliation_job_no_delete'
  ))<>18 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation evidence triggers are incomplete'; END IF;
  IF (SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('ido_reconciliation_job','ido_reconciliation_event','ido_reconciliation_wallet','ido_reconciliation_check','ido_reconciliation_manifest')
      AND DELETE_RULE='RESTRICT')<>5 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation evidence foreign keys are incomplete'; END IF;
  IF (SELECT COUNT(DISTINCT CONCAT(TABLE_NAME,':',INDEX_NAME)) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('ido_reconciliation_run','ido_reconciliation_job','ido_reconciliation_event',
        'ido_reconciliation_wallet','ido_reconciliation_check','ido_reconciliation_manifest')
      AND INDEX_NAME IN ('PRIMARY','uk_ido_reconciliation_run_uuid','uk_ido_reconciliation_request_idempotency','idx_ido_reconciliation_project_time',
      'uk_ido_reconciliation_active_project','idx_ido_reconciliation_job_claim','idx_ido_reconciliation_job_lease',
      'uk_ido_reconciliation_event_sequence','uk_ido_reconciliation_event_hash','idx_ido_reconciliation_event_type',
      'uk_ido_reconciliation_wallet_ordinal','uk_ido_reconciliation_wallet_address','idx_ido_reconciliation_wallet_lookup',
      'uk_ido_reconciliation_check_ordinal','uk_ido_reconciliation_check_code','uk_ido_reconciliation_manifest_digest',
      'uk_ido_reconciliation_manifest_parent','idx_ido_reconciliation_manifest_project'))<>23 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation evidence indexes are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES'
      AND CONSTRAINT_NAME LIKE 'chk_ido_reconciliation_%')<>16 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation evidence checks are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND (
      (TABLE_NAME='ido_reconciliation_run' AND COLUMN_NAME IN ('expected_check_count','expected_wallet_identity_digest','expected_check_identity_digest'))
      OR (TABLE_NAME='ido_reconciliation_wallet' AND COLUMN_NAME IN ('previous_row_digest','identity_digest'))
      OR (TABLE_NAME='ido_reconciliation_check' AND COLUMN_NAME IN ('previous_row_digest','identity_digest'))
      OR (TABLE_NAME='ido_reconciliation_manifest' AND COLUMN_NAME IN ('wallet_evidence_digest','check_evidence_digest'))
  ))<>9 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IDO reconciliation evidence root columns are incomplete'; END IF;
END$$
DELIMITER ;
CALL jc_assert_ido_reconciliation_evidence();
DROP PROCEDURE jc_assert_ido_reconciliation_evidence;

-- Manual postflight evidence:
-- SHOW CREATE TABLE ido_reconciliation_run;
-- SHOW CREATE TABLE ido_reconciliation_job;
-- SHOW CREATE TABLE ido_reconciliation_manifest;
-- SELECT TRIGGER_NAME,ACTION_STATEMENT FROM information_schema.TRIGGERS
-- WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME LIKE 'trg_ido_reconciliation_%';

-- END 2026_08_21_ido_reconciliation_evidence.sql

-- BEGIN 2026_08_22_chain_event_reorg_incident.sql
-- JoinCare IDO Sprint 1A-P3-A · durable deep-reorg detection evidence.
-- This migration deliberately does not auto-compensate financial facts. It
-- freezes a detected post-commit canonicality break and keeps all reconciliation
-- paths fail-closed until a separately audited compensation workflow resolves it.

SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_event_reorg_incident;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_event_reorg_incident()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('chain_event_sync_state','chain_event_record','chain_event_failure','fund_ledger','ido_purchase_entitlement'))<>5 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event reorg evidence prerequisites are missing';
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('chain_event_block_checkpoint','chain_event_reorg_incident','chain_event_reorg_event')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='pre-existing chain event reorg evidence tables require manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_event_reorg_incident();
DROP PROCEDURE jc_preflight_chain_event_reorg_incident;

CREATE TABLE chain_event_block_checkpoint (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  chain_id BIGINT UNSIGNED NOT NULL,
  block_number BIGINT UNSIGNED NOT NULL,
  block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_checkpoint_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  checkpoint_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_checkpoint_block (chain_id,block_number),
  UNIQUE KEY uk_chain_event_checkpoint_hash (checkpoint_hash),
  UNIQUE KEY uk_chain_event_checkpoint_parent (previous_checkpoint_hash),
  CONSTRAINT chk_chain_event_checkpoint_identity CHECK (
    chain_id>0 AND block_number>0 AND block_hash REGEXP '^0x[0-9a-f]{64}$'
  ) ENFORCED,
  CONSTRAINT chk_chain_event_checkpoint_digests CHECK (
    previous_checkpoint_hash REGEXP '^[0-9a-f]{64}$' AND checkpoint_hash REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_incident (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  incident_key VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  cursor_block BIGINT UNSIGNED NOT NULL,
  expected_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  observed_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  cursor_checkpoint_id BIGINT UNSIGNED NOT NULL,
  v2_start_block BIGINT UNSIGNED NOT NULL,
  status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  active_slot TINYINT UNSIGNED NULL,
  detection_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  detected_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  resolved_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_incident_key (incident_key),
  UNIQUE KEY uk_chain_event_reorg_active (active_slot),
  KEY idx_chain_event_reorg_status_time (status,detected_at,id),
  KEY idx_chain_event_reorg_checkpoint (cursor_checkpoint_id),
  CONSTRAINT fk_chain_event_reorg_checkpoint FOREIGN KEY (cursor_checkpoint_id)
    REFERENCES chain_event_block_checkpoint(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_identity CHECK (
    chain_id>0 AND cursor_block>0 AND v2_start_block>0
    AND contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND contract_address<>'0x0000000000000000000000000000000000000000'
    AND contract_version='v2_stablecoin'
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_hashes CHECK (
    incident_key REGEXP '^[0-9a-f]{64}$'
    AND expected_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND observed_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND expected_block_hash<>observed_block_hash
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_state CHECK (
    status='detected' AND active_slot=1 AND detection_code='CURSOR_BLOCK_HASH_MISMATCH' AND resolved_at IS NULL
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_event (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  incident_id BIGINT UNSIGNED NOT NULL,
  sequence_no BIGINT UNSIGNED NOT NULL,
  event_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  message VARCHAR(500) NOT NULL,
  previous_event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_event_sequence (incident_id,sequence_no),
  UNIQUE KEY uk_chain_event_reorg_event_hash (event_hash),
  KEY idx_chain_event_reorg_event_incident (incident_id),
  CONSTRAINT fk_chain_event_reorg_event_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_event_fields CHECK (
    incident_id>0 AND sequence_no>0 AND event_type='DETECTED'
    AND CHAR_LENGTH(TRIM(message)) BETWEEN 3 AND 500
    AND previous_event_hash REGEXP '^[0-9a-f]{64}$' AND event_hash REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DROP TRIGGER IF EXISTS trg_chain_event_checkpoint_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_checkpoint_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_checkpoint_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_incident_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_incident_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_incident_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_event_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_event_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_event_no_delete;
DELIMITER $$
CREATE TRIGGER trg_chain_event_checkpoint_validate_insert BEFORE INSERT ON chain_event_block_checkpoint FOR EACH ROW
BEGIN
  DECLARE prior_hash VARCHAR(64);
  DECLARE prior_block BIGINT UNSIGNED;
  DECLARE prior_chain BIGINT UNSIGNED;
  SET prior_hash=COALESCE((SELECT checkpoint_hash FROM chain_event_block_checkpoint ORDER BY id DESC LIMIT 1),REPEAT('0',64));
  SET prior_block=COALESCE((SELECT block_number FROM chain_event_block_checkpoint ORDER BY id DESC LIMIT 1),0);
  SET prior_chain=COALESCE((SELECT chain_id FROM chain_event_block_checkpoint ORDER BY id DESC LIMIT 1),NEW.chain_id);
  IF NEW.previous_checkpoint_hash<>prior_hash THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event checkpoint chain is not contiguous';
  END IF;
  IF NEW.chain_id<>prior_chain OR NEW.block_number<=prior_block THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event checkpoint cursor is not strictly monotonic';
  END IF;
  IF NEW.checkpoint_hash<>LOWER(SHA2(CONCAT(
    'joincare.chain-event.checkpoint.v1',CHAR(10),NEW.chain_id,CHAR(10),NEW.block_number,CHAR(10),
    NEW.block_hash,CHAR(10),NEW.previous_checkpoint_hash
  ),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event checkpoint digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_checkpoint_no_update BEFORE UPDATE ON chain_event_block_checkpoint FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event checkpoint rows are append-only'; END$$
CREATE TRIGGER trg_chain_event_checkpoint_no_delete BEFORE DELETE ON chain_event_block_checkpoint FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event checkpoint rows are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_incident_validate_insert BEFORE INSERT ON chain_event_reorg_incident FOR EACH ROW
BEGIN
  IF NOT EXISTS (SELECT 1 FROM chain_event_block_checkpoint c WHERE c.id=NEW.cursor_checkpoint_id
      AND c.chain_id=NEW.chain_id AND c.block_number=NEW.cursor_block AND c.block_hash=NEW.expected_block_hash) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg incident cursor checkpoint identity mismatch';
  END IF;
  IF NEW.incident_key<>LOWER(SHA2(CONCAT(
    'joincare.chain-event.reorg-incident.v1',CHAR(10),NEW.chain_id,CHAR(10),NEW.contract_address,CHAR(10),
    NEW.cursor_block,CHAR(10),NEW.expected_block_hash,CHAR(10),NEW.observed_block_hash
  ),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg incident digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_incident_no_update BEFORE UPDATE ON chain_event_reorg_incident FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg incident rows are append-only until compensation migration'; END$$
CREATE TRIGGER trg_chain_event_reorg_incident_no_delete BEFORE DELETE ON chain_event_reorg_incident FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg incident rows are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_event_validate_insert BEFORE INSERT ON chain_event_reorg_event FOR EACH ROW
BEGIN
  IF NEW.sequence_no<>1 OR NEW.previous_event_hash<>REPEAT('0',64)
     OR EXISTS (SELECT 1 FROM chain_event_reorg_event WHERE incident_id=NEW.incident_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg incident event chain is not canonical';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_incident i WHERE i.id=NEW.incident_id AND i.status='detected' AND i.active_slot=1) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg incident event requires active incident';
  END IF;
  IF NEW.event_hash<>LOWER(SHA2(CONCAT(
    'joincare.chain-event.reorg-event.v1',CHAR(10),NEW.incident_id,CHAR(10),NEW.sequence_no,CHAR(10),
    NEW.event_type,CHAR(10),NEW.message,CHAR(10),NEW.previous_event_hash
  ),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg incident event digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_event_no_update BEFORE UPDATE ON chain_event_reorg_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg incident event rows are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_event_no_delete BEFORE DELETE ON chain_event_reorg_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg incident event rows are append-only'; END$$
DELIMITER ;

INSERT INTO chain_event_block_checkpoint (
  chain_id,block_number,block_hash,previous_checkpoint_hash,checkpoint_hash,created_at
)
SELECT p.chain_id,s.cursor_block,LOWER(s.cursor_block_hash),REPEAT('0',64),LOWER(SHA2(CONCAT(
  'joincare.chain-event.checkpoint.v1',CHAR(10),p.chain_id,CHAR(10),s.cursor_block,CHAR(10),
  LOWER(s.cursor_block_hash),CHAR(10),REPEAT('0',64)
),256)),CURRENT_TIMESTAMP(6)
FROM chain_event_sync_state s
JOIN (SELECT chain_id FROM projects WHERE contract_version='v2_stablecoin' GROUP BY chain_id) p ON 1=1
WHERE s.id=1 AND s.initialized=1 AND s.v2_registered=1 AND s.cursor_block>0
  AND LOWER(s.cursor_block_hash) REGEXP '^0x[0-9a-f]{64}$'
  AND (SELECT COUNT(DISTINCT chain_id) FROM projects WHERE contract_version='v2_stablecoin')=1;

DROP PROCEDURE IF EXISTS jc_assert_chain_event_reorg_incident;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_reorg_incident()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('chain_event_block_checkpoint','chain_event_reorg_incident','chain_event_reorg_event') AND TABLE_TYPE='BASE TABLE')<>3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event reorg evidence tables are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM (
      SELECT TABLE_NAME,COUNT(*) column_count FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('chain_event_block_checkpoint','chain_event_reorg_incident','chain_event_reorg_event') GROUP BY TABLE_NAME
    ) c WHERE (TABLE_NAME='chain_event_block_checkpoint' AND column_count=7)
      OR (TABLE_NAME='chain_event_reorg_incident' AND column_count=17)
      OR (TABLE_NAME='chain_event_reorg_event' AND column_count=8))<>3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event reorg evidence columns are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM (
      SELECT TABLE_NAME,INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('chain_event_block_checkpoint','chain_event_reorg_incident','chain_event_reorg_event') GROUP BY TABLE_NAME,INDEX_NAME
    ) i WHERE INDEX_NAME IN (
      'PRIMARY','uk_chain_event_checkpoint_block','uk_chain_event_checkpoint_hash','uk_chain_event_checkpoint_parent',
      'uk_chain_event_reorg_incident_key','uk_chain_event_reorg_active','idx_chain_event_reorg_status_time','idx_chain_event_reorg_checkpoint',
      'uk_chain_event_reorg_event_sequence','uk_chain_event_reorg_event_hash','idx_chain_event_reorg_event_incident'
    ))<>13 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event reorg evidence indexes are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
      AND TRIGGER_NAME IN ('trg_chain_event_checkpoint_validate_insert','trg_chain_event_checkpoint_no_update','trg_chain_event_checkpoint_no_delete',
      'trg_chain_event_reorg_incident_validate_insert','trg_chain_event_reorg_incident_no_update','trg_chain_event_reorg_incident_no_delete',
      'trg_chain_event_reorg_event_validate_insert','trg_chain_event_reorg_event_no_update','trg_chain_event_reorg_event_no_delete'))<>9 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event reorg evidence triggers are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_NAME IN ('fk_chain_event_reorg_checkpoint','fk_chain_event_reorg_event_incident') AND DELETE_RULE='RESTRICT')<>2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event reorg evidence foreign keys are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES' AND CONSTRAINT_NAME IN (
      'chk_chain_event_checkpoint_identity','chk_chain_event_checkpoint_digests','chk_chain_event_reorg_identity',
      'chk_chain_event_reorg_hashes','chk_chain_event_reorg_state','chk_chain_event_reorg_event_fields'))<>6 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event reorg evidence checks are incomplete';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_reorg_incident();
DROP PROCEDURE jc_assert_chain_event_reorg_incident;

-- END 2026_08_22_chain_event_reorg_incident.sql

-- BEGIN 2026_08_23_chain_event_reorg_recovery_scan.sql
-- JoinCare IDO Sprint 1A-P3-B1 · durable replacement-branch recovery scan.
-- Completion proves only that a fixed safe range was scanned and staged. It
-- does not resolve the incident or mutate canonical financial projections.

SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_event_reorg_recovery;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_event_reorg_recovery()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('chain_event_sync_state','chain_event_block_checkpoint','chain_event_reorg_incident','chain_event_reorg_event'))<>4 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain event reorg recovery prerequisites are missing';
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_recovery_job','chain_event_reorg_recovery_event',
      'chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch','chain_event_reorg_recovery_manifest',
      'chain_event_reorg_recovery_definition')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='pre-existing reorg recovery tables require manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_event_reorg_recovery();
DROP PROCEDURE jc_preflight_chain_event_reorg_recovery;

CREATE TABLE chain_event_reorg_recovery_run (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_uuid CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  attempt_no INT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  legacy_contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  algorithm_version VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  boundary_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  boundary_checkpoint_id BIGINT UNSIGNED NULL,
  boundary_block BIGINT UNSIGNED NOT NULL,
  boundary_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  target_safe_block BIGINT UNSIGNED NOT NULL,
  target_safe_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  confirmation_depth BIGINT UNSIGNED NOT NULL,
  expected_batch_count INT UNSIGNED NOT NULL,
  expected_scanned_blocks BIGINT UNSIGNED NOT NULL,
  requested_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_recovery_uuid (run_uuid),
  UNIQUE KEY uk_chain_event_reorg_recovery_attempt (incident_id,attempt_no),
  KEY idx_chain_event_reorg_recovery_incident (incident_id,id),
  KEY idx_chain_event_reorg_recovery_checkpoint (boundary_checkpoint_id),
  CONSTRAINT fk_chain_event_reorg_recovery_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_recovery_checkpoint FOREIGN KEY (boundary_checkpoint_id)
    REFERENCES chain_event_block_checkpoint(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_recovery_run_identity CHECK (
    incident_id>0 AND attempt_no>0 AND chain_id>0
    AND run_uuid REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
    AND contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND contract_address<>'0x0000000000000000000000000000000000000000'
    AND legacy_contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND legacy_contract_address<>'0x0000000000000000000000000000000000000000'
    AND legacy_contract_address<>contract_address
    AND contract_version='v2_stablecoin'
    AND algorithm_version='joincare-reorg-recovery-scan-v1'
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_recovery_boundary CHECK (
    boundary_kind='last_canonical_checkpoint' AND boundary_checkpoint_id IS NOT NULL AND boundary_block>0
      AND boundary_block_hash REGEXP '^0x[0-9a-f]{64}$'
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_recovery_target CHECK (
    target_safe_block>boundary_block AND target_safe_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND expected_scanned_blocks=target_safe_block-boundary_block
    AND expected_batch_count=CEIL(expected_scanned_blocks/100)
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_recovery_job (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  state VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  active_slot TINYINT UNSIGNED NULL,
  next_block BIGINT UNSIGNED NOT NULL,
  next_batch_ordinal INT UNSIGNED NOT NULL,
  next_log_ordinal BIGINT UNSIGNED NOT NULL,
  observed_log_count BIGINT UNSIGNED NOT NULL,
  batch_evidence_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  log_evidence_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  lease_token VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  lease_expires_at DATETIME(6) NULL,
  fence_epoch BIGINT UNSIGNED NOT NULL DEFAULT 0,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  last_error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  last_error_message VARCHAR(500) NOT NULL DEFAULT '',
  completed_at DATETIME(6) NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_recovery_active (incident_id,active_slot),
  KEY idx_chain_event_reorg_recovery_job_state (state,updated_at,run_id),
  CONSTRAINT fk_chain_event_reorg_recovery_job_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_recovery_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_recovery_job_state CHECK (
    state IN ('scanning','completed','invalidated')
    AND ((state='scanning' AND active_slot=1 AND completed_at IS NULL)
      OR (state<>'scanning' AND active_slot IS NULL AND completed_at IS NOT NULL))
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_recovery_job_digests CHECK (
    batch_evidence_digest REGEXP '^[0-9a-f]{64}$' AND log_evidence_digest REGEXP '^[0-9a-f]{64}$'
    AND next_batch_ordinal>0 AND next_log_ordinal>0 AND fence_epoch>=0
    AND ((lease_token='' AND lease_expires_at IS NULL) OR (lease_token REGEXP '^[0-9a-f]{64}$' AND lease_expires_at IS NOT NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_recovery_event (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  sequence_no INT UNSIGNED NOT NULL,
  event_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  stage VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  message VARCHAR(500) NOT NULL,
  previous_event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_recovery_event_sequence (run_id,sequence_no),
  UNIQUE KEY uk_chain_event_reorg_recovery_event_hash (event_hash),
  KEY idx_chain_event_reorg_recovery_event_type (run_id,event_type,created_at),
  CONSTRAINT fk_chain_event_reorg_recovery_event_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_recovery_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_recovery_event_fields CHECK (
    sequence_no>0 AND event_type IN ('CREATED','BATCH_COMMITTED','COMPLETED','INVALIDATED')
    AND stage IN ('boundary','scan','finalize') AND CHAR_LENGTH(TRIM(message)) BETWEEN 3 AND 500
    AND previous_event_hash REGEXP '^[0-9a-f]{64}$' AND event_hash REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_recovery_log (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  block_number BIGINT UNSIGNED NOT NULL,
  block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tx_index INT UNSIGNED NOT NULL,
  log_index INT UNSIGNED NOT NULL,
  contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  raw_log MEDIUMBLOB NOT NULL,
  raw_log_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  observed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_recovery_log_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_recovery_log_identity (run_id,block_number,tx_hash,log_index),
  UNIQUE KEY uk_chain_event_reorg_recovery_log_digest (run_id,row_digest),
  KEY idx_chain_event_reorg_recovery_log_block (run_id,block_number,tx_index,log_index),
  CONSTRAINT fk_chain_event_reorg_recovery_log_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_recovery_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_recovery_log_identity CHECK (
    ordinal>0 AND block_number>0 AND block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND tx_hash REGEXP '^0x[0-9a-f]{64}$' AND contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND contract_version IN ('v1_legacy','v2_stablecoin') AND OCTET_LENGTH(raw_log) BETWEEN 2 AND 1048576
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_recovery_log_digests CHECK (
    raw_log_digest REGEXP '^[0-9a-f]{64}$' AND previous_row_digest REGEXP '^[0-9a-f]{64}$'
    AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_recovery_batch (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal INT UNSIGNED NOT NULL,
  from_block BIGINT UNSIGNED NOT NULL,
  to_block BIGINT UNSIGNED NOT NULL,
  from_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  to_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  first_log_ordinal BIGINT UNSIGNED NOT NULL,
  log_count BIGINT UNSIGNED NOT NULL,
  batch_log_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_batch_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  batch_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  observed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_recovery_batch_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_recovery_batch_range (run_id,from_block,to_block),
  UNIQUE KEY uk_chain_event_reorg_recovery_batch_digest (run_id,batch_digest),
  CONSTRAINT fk_chain_event_reorg_recovery_batch_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_recovery_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_recovery_batch_identity CHECK (
    ordinal>0 AND from_block>0 AND to_block>=from_block AND to_block-from_block<100
    AND from_block_hash REGEXP '^0x[0-9a-f]{64}$' AND to_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND first_log_ordinal>0
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_recovery_batch_digests CHECK (
    batch_log_digest REGEXP '^[0-9a-f]{64}$' AND previous_batch_digest REGEXP '^[0-9a-f]{64}$'
    AND batch_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_recovery_manifest (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  batch_count INT UNSIGNED NOT NULL,
  log_count BIGINT UNSIGNED NOT NULL,
  batch_evidence_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  log_evidence_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  completed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_recovery_manifest_digest (manifest_digest),
  KEY idx_chain_event_reorg_recovery_manifest_incident (incident_id,run_id),
  CONSTRAINT fk_chain_event_reorg_recovery_manifest_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_recovery_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_recovery_manifest_fields CHECK (
    incident_id>0 AND batch_count>0 AND batch_evidence_digest REGEXP '^[0-9a-f]{64}$'
    AND log_evidence_digest REGEXP '^[0-9a-f]{64}$' AND manifest_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_recovery_definition (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  object_type VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  object_name VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  definition_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_recovery_definition_object (object_type,object_name),
  UNIQUE KEY uk_chain_event_reorg_recovery_definition_digest (row_digest),
  CONSTRAINT chk_chain_event_reorg_recovery_definition_identity CHECK (
    object_type IN ('CHECK','TRIGGER') AND CHAR_LENGTH(object_name) BETWEEN 3 AND 128
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_recovery_definition_digests CHECK (
    definition_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_run_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_run_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_run_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_job_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_job_validate_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_job_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_event_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_event_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_event_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_log_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_log_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_log_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_batch_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_batch_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_batch_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_manifest_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_manifest_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_manifest_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_definition_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_definition_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_recovery_definition_no_delete;
DELIMITER $$
CREATE TRIGGER trg_chain_event_reorg_recovery_run_validate_insert BEFORE INSERT ON chain_event_reorg_recovery_run FOR EACH ROW
BEGIN
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_incident i WHERE i.id=NEW.incident_id AND i.status='detected'
      AND i.active_slot=1 AND i.chain_id=NEW.chain_id AND i.contract_address=NEW.contract_address) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery run requires matching active incident';
  END IF;
  IF NEW.attempt_no<>COALESCE((SELECT MAX(r.attempt_no)+1 FROM chain_event_reorg_recovery_run r WHERE r.incident_id=NEW.incident_id),1) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery attempt is not contiguous';
  END IF;
  IF NEW.boundary_checkpoint_id IS NOT NULL AND NOT EXISTS (
      SELECT 1 FROM chain_event_block_checkpoint c WHERE c.id=NEW.boundary_checkpoint_id
      AND c.chain_id=NEW.chain_id AND c.block_number=NEW.boundary_block AND c.block_hash=NEW.boundary_block_hash) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery boundary checkpoint identity mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_run_no_update BEFORE UPDATE ON chain_event_reorg_recovery_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery runs are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_run_no_delete BEFORE DELETE ON chain_event_reorg_recovery_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery runs are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_recovery_job_validate_insert BEFORE INSERT ON chain_event_reorg_recovery_job FOR EACH ROW
BEGIN
  IF NEW.state<>'scanning' OR NEW.active_slot<>1 OR NEW.next_batch_ordinal<>1 OR NEW.next_log_ordinal<>1
     OR NEW.observed_log_count<>0 OR NEW.batch_evidence_digest<>REPEAT('0',64) OR NEW.log_evidence_digest<>REPEAT('0',64)
     OR NEW.lease_token<>'' OR NEW.lease_expires_at IS NOT NULL OR NEW.fence_epoch<>0
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_run r WHERE r.id=NEW.run_id AND r.incident_id=NEW.incident_id
       AND NEW.next_block=r.boundary_block+1) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery job is not canonical initial state';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_job_validate_update BEFORE UPDATE ON chain_event_reorg_recovery_job FOR EACH ROW
BEGIN
  IF NEW.run_id<>OLD.run_id OR NEW.incident_id<>OLD.incident_id OR NEW.next_block<OLD.next_block
     OR NEW.next_batch_ordinal<OLD.next_batch_ordinal OR NEW.next_log_ordinal<OLD.next_log_ordinal
     OR NEW.observed_log_count<OLD.observed_log_count OR NEW.attempt_count<OLD.attempt_count OR NEW.fence_epoch<OLD.fence_epoch
     OR (OLD.state<>'scanning' AND NEW.state<>OLD.state) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery job transition is not monotonic';
  END IF;
  IF NEW.state='scanning' AND (
      NOT ((NEW.fence_epoch=OLD.fence_epoch AND (NEW.lease_token=OLD.lease_token
          OR (NEW.lease_token='' AND NEW.lease_expires_at IS NULL)))
        OR (NEW.fence_epoch=OLD.fence_epoch+1 AND (OLD.lease_expires_at IS NULL OR OLD.lease_expires_at<=CURRENT_TIMESTAMP(6))
          AND NEW.lease_token REGEXP '^[0-9a-f]{64}$' AND NEW.lease_expires_at>CURRENT_TIMESTAMP(6)))
      OR NEW.attempt_count>OLD.attempt_count+1
      OR (NEW.next_block>OLD.next_block AND (NEW.next_block<>LEAST(OLD.next_block+100,(SELECT target_safe_block+1 FROM chain_event_reorg_recovery_run WHERE id=NEW.run_id))
        OR NEW.next_batch_ordinal<>OLD.next_batch_ordinal+1
        OR NEW.next_log_ordinal<>OLD.next_log_ordinal+(NEW.observed_log_count-OLD.observed_log_count)
        OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_batch b WHERE b.run_id=NEW.run_id
          AND b.ordinal=OLD.next_batch_ordinal AND b.from_block=OLD.next_block AND b.to_block=NEW.next_block-1
          AND b.first_log_ordinal=OLD.next_log_ordinal AND b.log_count=NEW.observed_log_count-OLD.observed_log_count
          AND b.batch_digest=NEW.batch_evidence_digest
          AND ((b.log_count=0 AND NEW.log_evidence_digest=OLD.log_evidence_digest)
            OR (b.log_count>0 AND b.batch_log_digest=NEW.log_evidence_digest)))))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery scanning transition bypassed lease or batch checkpoint';
  END IF;
  IF NEW.state='completed' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_manifest m WHERE m.run_id=NEW.run_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='completed reorg recovery job requires manifest';
  END IF;
  IF NEW.state<>'scanning' AND (
      OLD.state<>'scanning' OR OLD.lease_token NOT REGEXP '^[0-9a-f]{64}$' OR OLD.lease_expires_at<=CURRENT_TIMESTAMP(6)
      OR NEW.active_slot IS NOT NULL OR NEW.lease_token<>'' OR NEW.lease_expires_at IS NOT NULL
      OR NEW.fence_epoch<>OLD.fence_epoch OR NEW.next_block<>OLD.next_block
      OR NEW.next_batch_ordinal<>OLD.next_batch_ordinal OR NEW.next_log_ordinal<>OLD.next_log_ordinal
      OR NEW.observed_log_count<>OLD.observed_log_count OR NEW.batch_evidence_digest<>OLD.batch_evidence_digest
      OR NEW.log_evidence_digest<>OLD.log_evidence_digest OR NEW.completed_at IS NULL
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_event e WHERE e.run_id=NEW.run_id
        AND e.sequence_no=(SELECT MAX(x.sequence_no) FROM chain_event_reorg_recovery_event x WHERE x.run_id=NEW.run_id)
        AND e.event_type=CASE WHEN NEW.state='completed' THEN 'COMPLETED' ELSE 'INVALIDATED' END)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery terminal transition lacks leased matching event';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_job_no_delete BEFORE DELETE ON chain_event_reorg_recovery_job FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery jobs cannot be deleted'; END$$

CREATE TRIGGER trg_chain_event_reorg_recovery_event_validate_insert BEFORE INSERT ON chain_event_reorg_recovery_event FOR EACH ROW
BEGIN
  DECLARE prior_hash VARCHAR(64);
  DECLARE prior_sequence INT UNSIGNED;
  SET prior_hash=COALESCE((SELECT event_hash FROM chain_event_reorg_recovery_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),REPEAT('0',64));
  SET prior_sequence=COALESCE((SELECT sequence_no FROM chain_event_reorg_recovery_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),0);
  IF NEW.sequence_no<>prior_sequence+1 OR NEW.previous_event_hash<>prior_hash THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery event chain is not contiguous';
  END IF;
  IF (NEW.sequence_no=1 AND (NEW.event_type<>'CREATED' OR NEW.stage<>'boundary' OR NEW.error_code<>''))
     OR (NEW.sequence_no>1 AND NEW.event_type='CREATED')
     OR (NEW.event_type='BATCH_COMMITTED' AND (NEW.stage<>'scan' OR NEW.error_code<>''))
     OR (NEW.event_type='COMPLETED' AND (NEW.stage<>'finalize' OR NEW.error_code<>''
       OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_manifest m WHERE m.run_id=NEW.run_id)))
     OR (NEW.event_type='INVALIDATED' AND CHAR_LENGTH(TRIM(NEW.error_code))<3) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery event does not match persistent state';
  END IF;
  IF (NEW.event_type='CREATED' AND NOT EXISTS (
      SELECT 1 FROM chain_event_reorg_recovery_job j WHERE j.run_id=NEW.run_id AND j.state='scanning'
      AND j.active_slot=1 AND j.next_batch_ordinal=1 AND j.next_log_ordinal=1
      AND j.observed_log_count=0 AND j.lease_token='' AND j.lease_expires_at IS NULL))
     OR (NEW.event_type<>'CREATED' AND NOT EXISTS (
      SELECT 1 FROM chain_event_reorg_recovery_job j WHERE j.run_id=NEW.run_id AND j.state='scanning'
      AND j.active_slot=1 AND j.lease_token REGEXP '^[0-9a-f]{64}$'
      AND j.lease_expires_at>CURRENT_TIMESTAMP(6))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery event requires matching active leased job';
  END IF;
  IF NEW.event_hash<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-recovery-event.v1',CHAR(10),NEW.run_id,CHAR(10),
      NEW.sequence_no,CHAR(10),NEW.event_type,CHAR(10),NEW.stage,CHAR(10),NEW.error_code,CHAR(10),NEW.message,CHAR(10),NEW.previous_event_hash),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery event digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_event_no_update BEFORE UPDATE ON chain_event_reorg_recovery_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery events are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_event_no_delete BEFORE DELETE ON chain_event_reorg_recovery_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery events are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_recovery_log_validate_insert BEFORE INSERT ON chain_event_reorg_recovery_log FOR EACH ROW
BEGIN
  DECLARE prior_digest VARCHAR(64);
  DECLARE prior_ordinal BIGINT UNSIGNED;
  DECLARE pending_from BIGINT UNSIGNED;
  DECLARE pending_target BIGINT UNSIGNED;
  DECLARE expected_v1 VARCHAR(42);
  DECLARE expected_v2 VARCHAR(42);
  SET prior_digest=COALESCE((SELECT row_digest FROM chain_event_reorg_recovery_log WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET prior_ordinal=COALESCE((SELECT ordinal FROM chain_event_reorg_recovery_log WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),0);
  SELECT j.next_block,r.target_safe_block,r.legacy_contract_address,r.contract_address
    INTO pending_from,pending_target,expected_v1,expected_v2
    FROM chain_event_reorg_recovery_job j JOIN chain_event_reorg_recovery_run r ON r.id=j.run_id
    WHERE j.run_id=NEW.run_id AND j.state='scanning' AND j.active_slot=1
      AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6);
  IF pending_from IS NULL OR NEW.block_number<pending_from OR NEW.block_number>LEAST(pending_from+99,pending_target)
     OR NOT ((NEW.contract_version='v1_legacy' AND NEW.contract_address=expected_v1)
       OR (NEW.contract_version='v2_stablecoin' AND NEW.contract_address=expected_v2)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery log is outside active leased scan range';
  END IF;
  IF NEW.ordinal<>prior_ordinal+1 OR NEW.previous_row_digest<>prior_digest
     OR NEW.raw_log_digest<>LOWER(SHA2(NEW.raw_log,256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery log chain or payload digest mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-recovery-log.v1',CHAR(10),NEW.run_id,CHAR(10),NEW.ordinal,CHAR(10),
      NEW.block_number,CHAR(10),NEW.block_hash,CHAR(10),NEW.tx_hash,CHAR(10),NEW.tx_index,CHAR(10),NEW.log_index,CHAR(10),
      NEW.contract_address,CHAR(10),NEW.contract_version,CHAR(10),NEW.raw_log_digest,CHAR(10),NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery log row digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_log_no_update BEFORE UPDATE ON chain_event_reorg_recovery_log FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery logs are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_log_no_delete BEFORE DELETE ON chain_event_reorg_recovery_log FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery logs are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_recovery_batch_validate_insert BEFORE INSERT ON chain_event_reorg_recovery_batch FOR EACH ROW
BEGIN
  DECLARE prior_digest VARCHAR(64);
  DECLARE prior_ordinal INT UNSIGNED;
  DECLARE prior_to BIGINT UNSIGNED;
  DECLARE expected_first_log BIGINT UNSIGNED;
  DECLARE run_boundary BIGINT UNSIGNED;
  DECLARE run_target BIGINT UNSIGNED;
  DECLARE exact_logs BIGINT UNSIGNED;
  DECLARE materialized_logs BIGINT UNSIGNED;
  SET prior_digest=COALESCE((SELECT batch_digest FROM chain_event_reorg_recovery_batch WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET prior_ordinal=COALESCE((SELECT ordinal FROM chain_event_reorg_recovery_batch WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),0);
  SELECT boundary_block,target_safe_block INTO run_boundary,run_target FROM chain_event_reorg_recovery_run WHERE id=NEW.run_id;
  SET prior_to=COALESCE((SELECT to_block FROM chain_event_reorg_recovery_batch WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),run_boundary);
  SET expected_first_log=COALESCE((SELECT first_log_ordinal+log_count FROM chain_event_reorg_recovery_batch WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),1);
  IF NEW.ordinal<>prior_ordinal+1 OR NEW.from_block<>prior_to+1 OR NEW.to_block<>LEAST(NEW.from_block+99,run_target)
     OR NEW.first_log_ordinal<>expected_first_log OR NEW.previous_batch_digest<>prior_digest
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_job j WHERE j.run_id=NEW.run_id AND j.state='scanning'
       AND j.active_slot=1 AND j.next_block=NEW.from_block AND j.next_batch_ordinal=NEW.ordinal AND j.next_log_ordinal=NEW.first_log_ordinal
       AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery batch chain or range is not contiguous';
  END IF;
  IF NEW.log_count=0 AND NEW.batch_log_digest<>REPEAT('0',64) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='empty recovery batch must use zero log digest';
  END IF;
  SET exact_logs=(SELECT COUNT(*) FROM chain_event_reorg_recovery_log l WHERE l.run_id=NEW.run_id AND NEW.log_count>0
    AND l.ordinal BETWEEN NEW.first_log_ordinal AND NEW.first_log_ordinal+NEW.log_count-1
    AND l.block_number BETWEEN NEW.from_block AND NEW.to_block);
  SET materialized_logs=(SELECT COUNT(*) FROM chain_event_reorg_recovery_log l WHERE l.run_id=NEW.run_id AND l.ordinal>=NEW.first_log_ordinal);
  IF materialized_logs<>NEW.log_count OR (NEW.log_count=0 AND exact_logs<>0) OR (NEW.log_count>0 AND (exact_logs<>NEW.log_count
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_log l WHERE l.run_id=NEW.run_id
        AND l.ordinal=NEW.first_log_ordinal+NEW.log_count-1 AND l.row_digest=NEW.batch_log_digest))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery batch log evidence is incomplete';
  END IF;
  IF NEW.batch_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-recovery-batch.v1',CHAR(10),NEW.run_id,CHAR(10),NEW.ordinal,CHAR(10),
      NEW.from_block,CHAR(10),NEW.to_block,CHAR(10),NEW.from_block_hash,CHAR(10),NEW.to_block_hash,CHAR(10),NEW.first_log_ordinal,CHAR(10),
      NEW.log_count,CHAR(10),NEW.batch_log_digest,CHAR(10),NEW.previous_batch_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery batch digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_batch_no_update BEFORE UPDATE ON chain_event_reorg_recovery_batch FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery batches are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_batch_no_delete BEFORE DELETE ON chain_event_reorg_recovery_batch FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery batches are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_recovery_manifest_validate_insert BEFORE INSERT ON chain_event_reorg_recovery_manifest FOR EACH ROW
BEGIN
  DECLARE actual_batches INT UNSIGNED;
  DECLARE actual_logs BIGINT UNSIGNED;
  DECLARE last_batch VARCHAR(64);
  DECLARE last_log VARCHAR(64);
  DECLARE run_target BIGINT UNSIGNED;
  DECLARE final_to BIGINT UNSIGNED;
  SELECT expected_batch_count,target_safe_block INTO actual_batches,run_target FROM chain_event_reorg_recovery_run WHERE id=NEW.run_id AND incident_id=NEW.incident_id;
  IF actual_batches IS NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery manifest run identity mismatch'; END IF;
  IF NEW.batch_count<>actual_batches OR (SELECT COUNT(*) FROM chain_event_reorg_recovery_batch WHERE run_id=NEW.run_id)<>actual_batches THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery manifest batch set is incomplete';
  END IF;
  SELECT COALESCE(SUM(log_count),0),MAX(to_block) INTO actual_logs,final_to FROM chain_event_reorg_recovery_batch WHERE run_id=NEW.run_id;
  IF NEW.log_count<>actual_logs OR (SELECT COUNT(*) FROM chain_event_reorg_recovery_log WHERE run_id=NEW.run_id)<>actual_logs OR final_to<>run_target THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery manifest range or log set is incomplete';
  END IF;
  SET last_batch=COALESCE((SELECT batch_digest FROM chain_event_reorg_recovery_batch WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET last_log=COALESCE((SELECT row_digest FROM chain_event_reorg_recovery_log WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  IF NEW.batch_evidence_digest<>last_batch OR NEW.log_evidence_digest<>last_log THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery manifest evidence roots mismatch';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_job j WHERE j.run_id=NEW.run_id AND j.incident_id=NEW.incident_id
      AND j.state='scanning' AND j.active_slot=1 AND j.lease_token REGEXP '^[0-9a-f]{64}$'
      AND j.lease_expires_at>CURRENT_TIMESTAMP(6) AND j.next_block=run_target+1
      AND j.next_batch_ordinal=NEW.batch_count+1 AND j.observed_log_count=NEW.log_count
      AND j.batch_evidence_digest=NEW.batch_evidence_digest AND j.log_evidence_digest=NEW.log_evidence_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery manifest requires active complete leased job';
  END IF;
  IF NEW.manifest_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-recovery-manifest.v1',CHAR(10),NEW.run_id,CHAR(10),NEW.incident_id,CHAR(10),
      NEW.batch_count,CHAR(10),NEW.log_count,CHAR(10),NEW.batch_evidence_digest,CHAR(10),NEW.log_evidence_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery manifest digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_manifest_no_update BEFORE UPDATE ON chain_event_reorg_recovery_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery manifests are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_manifest_no_delete BEFORE DELETE ON chain_event_reorg_recovery_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery manifests are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_definition_validate_insert BEFORE INSERT ON chain_event_reorg_recovery_definition FOR EACH ROW
BEGIN
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-recovery-definition.v1',CHAR(10),
      NEW.object_type,CHAR(10),NEW.object_name,CHAR(10),NEW.definition_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery definition attestation digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_definition_no_update BEFORE UPDATE ON chain_event_reorg_recovery_definition FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery definition attestations are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_recovery_definition_no_delete BEFORE DELETE ON chain_event_reorg_recovery_definition FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery definition attestations are append-only'; END$$
DELIMITER ;

INSERT INTO chain_event_reorg_recovery_definition (object_type,object_name,definition_digest,row_digest,created_at)
SELECT object_type,object_name,definition_digest,LOWER(SHA2(CONCAT(
  'joincare.chain-event.reorg-recovery-definition.v1',CHAR(10),object_type,CHAR(10),object_name,CHAR(10),definition_digest
),256)),CURRENT_TIMESTAMP(6)
FROM (
  SELECT 'CHECK' object_type,tc.CONSTRAINT_NAME object_name,LOWER(SHA2(CONCAT(
    'joincare.mysql-check-definition.v1',CHAR(10),tc.TABLE_SCHEMA,CHAR(10),tc.TABLE_NAME,CHAR(10),
    tc.ENFORCED,CHAR(10),cc.CHECK_CLAUSE
  ),256)) definition_digest
  FROM information_schema.TABLE_CONSTRAINTS tc
  JOIN information_schema.CHECK_CONSTRAINTS cc ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
  WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK'
    AND tc.TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_recovery_job',
      'chain_event_reorg_recovery_event','chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch',
      'chain_event_reorg_recovery_manifest','chain_event_reorg_recovery_definition')
  UNION ALL
  SELECT 'TRIGGER',TRIGGER_NAME,LOWER(SHA2(CONCAT(
    'joincare.mysql-trigger-definition.v1',CHAR(10),EVENT_OBJECT_SCHEMA,CHAR(10),EVENT_OBJECT_TABLE,CHAR(10),
    ACTION_TIMING,CHAR(10),EVENT_MANIPULATION,CHAR(10),ACTION_ORIENTATION,CHAR(10),ACTION_STATEMENT,CHAR(10),
    SQL_MODE,CHAR(10),DEFINER,CHAR(10),CHARACTER_SET_CLIENT,CHAR(10),COLLATION_CONNECTION,CHAR(10),DATABASE_COLLATION
  ),256))
  FROM information_schema.TRIGGERS
  WHERE TRIGGER_SCHEMA=DATABASE() AND EVENT_OBJECT_TABLE IN ('chain_event_reorg_recovery_run',
    'chain_event_reorg_recovery_job','chain_event_reorg_recovery_event','chain_event_reorg_recovery_log',
    'chain_event_reorg_recovery_batch','chain_event_reorg_recovery_manifest','chain_event_reorg_recovery_definition')
) definitions
ORDER BY object_type,object_name;

DROP PROCEDURE IF EXISTS jc_assert_chain_event_reorg_recovery;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_reorg_recovery()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'
      AND TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_recovery_job','chain_event_reorg_recovery_event',
      'chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch','chain_event_reorg_recovery_manifest',
      'chain_event_reorg_recovery_definition'))<>7 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery tables are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND (
      (TRIGGER_NAME='trg_chain_event_reorg_recovery_run_validate_insert' AND ACTION_STATEMENT LIKE '%matching active incident%' AND ACTION_STATEMENT LIKE '%attempt is not contiguous%' AND ACTION_STATEMENT LIKE '%boundary checkpoint identity%')
      OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_job_validate_insert' AND ACTION_STATEMENT LIKE '%canonical initial state%')
      OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_job_validate_update' AND ACTION_STATEMENT LIKE '%scanning transition bypassed lease or batch checkpoint%' AND ACTION_STATEMENT LIKE '%terminal transition lacks leased matching event%' AND ACTION_STATEMENT LIKE '%lease_expires_at%' AND ACTION_STATEMENT LIKE '%batch_digest%' AND ACTION_STATEMENT LIKE '%requires manifest%')
      OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_event_validate_insert' AND ACTION_STATEMENT LIKE '%event chain%' AND ACTION_STATEMENT LIKE '%matching active leased job%'
        AND ACTION_STATEMENT LIKE '%NEW.event_type=''CREATED''%' AND ACTION_STATEMENT LIKE '%next_batch_ordinal=1%'
        AND ACTION_STATEMENT LIKE '%lease_token=''''%' AND ACTION_STATEMENT LIKE '%NEW.event_type<>''CREATED''%'
        AND ACTION_STATEMENT LIKE '%active_slot=1%' AND ACTION_STATEMENT LIKE '%lease_token REGEXP%'
        AND ACTION_STATEMENT LIKE '%lease_expires_at>CURRENT_TIMESTAMP(6)%' AND ACTION_STATEMENT LIKE '%SHA2%')
      OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_log_validate_insert' AND ACTION_STATEMENT LIKE '%active leased scan range%' AND ACTION_STATEMENT LIKE '%raw_log_digest%' AND ACTION_STATEMENT LIKE '%row digest%' AND ACTION_STATEMENT LIKE '%SHA2%')
      OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_batch_validate_insert' AND ACTION_STATEMENT LIKE '%range is not contiguous%' AND ACTION_STATEMENT LIKE '%first_log_ordinal%' AND ACTION_STATEMENT LIKE '%exact_logs%' AND ACTION_STATEMENT LIKE '%batch log evidence%' AND ACTION_STATEMENT LIKE '%SHA2%')
      OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_manifest_validate_insert' AND ACTION_STATEMENT LIKE '%batch set is incomplete%' AND ACTION_STATEMENT LIKE '%range or log set is incomplete%' AND ACTION_STATEMENT LIKE '%evidence roots%' AND ACTION_STATEMENT LIKE '%active complete leased job%' AND ACTION_STATEMENT LIKE '%SHA2%')
      OR (TRIGGER_NAME IN ('trg_chain_event_reorg_recovery_run_no_update','trg_chain_event_reorg_recovery_event_no_update','trg_chain_event_reorg_recovery_log_no_update','trg_chain_event_reorg_recovery_batch_no_update','trg_chain_event_reorg_recovery_manifest_no_update') AND EVENT_MANIPULATION='UPDATE' AND ACTION_STATEMENT LIKE '%append-only%')
      OR (TRIGGER_NAME IN ('trg_chain_event_reorg_recovery_run_no_delete','trg_chain_event_reorg_recovery_job_no_delete','trg_chain_event_reorg_recovery_event_no_delete','trg_chain_event_reorg_recovery_log_no_delete','trg_chain_event_reorg_recovery_batch_no_delete','trg_chain_event_reorg_recovery_manifest_no_delete') AND EVENT_MANIPULATION='DELETE' AND ACTION_STATEMENT LIKE '%SIGNAL%')
      OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_definition_validate_insert' AND ACTION_STATEMENT LIKE '%definition attestation digest mismatch%' AND ACTION_STATEMENT LIKE '%SHA2%')
      OR (TRIGGER_NAME IN ('trg_chain_event_reorg_recovery_definition_no_update','trg_chain_event_reorg_recovery_definition_no_delete') AND ACTION_STATEMENT LIKE '%definition attestations are append-only%' AND ACTION_STATEMENT LIKE '%SIGNAL%')
    ))<>21 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery triggers are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_NAME IN ('fk_chain_event_reorg_recovery_incident','fk_chain_event_reorg_recovery_checkpoint',
      'fk_chain_event_reorg_recovery_job_run','fk_chain_event_reorg_recovery_event_run','fk_chain_event_reorg_recovery_log_run',
      'fk_chain_event_reorg_recovery_batch_run','fk_chain_event_reorg_recovery_manifest_run') AND DELETE_RULE='RESTRICT')<>7 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery foreign keys are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS tc
      JOIN information_schema.CHECK_CONSTRAINTS cc ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
      WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK' AND tc.ENFORCED='YES' AND (
      (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_run_identity' AND cc.CHECK_CLAUSE LIKE '%run_uuid%regexp%' AND cc.CHECK_CLAUSE LIKE '%legacy_contract_address%' AND cc.CHECK_CLAUSE LIKE '%joincare-reorg-recovery-scan-v1%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_boundary' AND cc.CHECK_CLAUSE LIKE '%last_canonical_checkpoint%' AND cc.CHECK_CLAUSE LIKE '%boundary_checkpoint_id%is not null%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_target' AND cc.CHECK_CLAUSE LIKE '%expected_scanned_blocks%' AND cc.CHECK_CLAUSE LIKE '%expected_batch_count%' AND cc.CHECK_CLAUSE LIKE '%100%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_job_state' AND cc.CHECK_CLAUSE LIKE '%scanning%' AND cc.CHECK_CLAUSE LIKE '%completed%' AND cc.CHECK_CLAUSE LIKE '%invalidated%' AND cc.CHECK_CLAUSE NOT LIKE '%failed%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_job_digests' AND cc.CHECK_CLAUSE LIKE '%lease_token%regexp%' AND cc.CHECK_CLAUSE LIKE '%lease_expires_at%' AND cc.CHECK_CLAUSE LIKE '%fence_epoch%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_event_fields' AND cc.CHECK_CLAUSE LIKE '%BATCH_COMMITTED%' AND cc.CHECK_CLAUSE LIKE '%INVALIDATED%' AND cc.CHECK_CLAUSE LIKE '%previous_event_hash%regexp%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_log_identity' AND cc.CHECK_CLAUSE LIKE '%v1_legacy%' AND cc.CHECK_CLAUSE LIKE '%v2_stablecoin%' AND cc.CHECK_CLAUSE LIKE '%raw_log%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_log_digests' AND cc.CHECK_CLAUSE LIKE '%raw_log_digest%regexp%' AND cc.CHECK_CLAUSE LIKE '%previous_row_digest%regexp%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_batch_identity' AND cc.CHECK_CLAUSE LIKE '%to_block%from_block%' AND cc.CHECK_CLAUSE LIKE '%100%' AND cc.CHECK_CLAUSE LIKE '%first_log_ordinal%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_batch_digests' AND cc.CHECK_CLAUSE LIKE '%batch_log_digest%regexp%' AND cc.CHECK_CLAUSE LIKE '%previous_batch_digest%regexp%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_manifest_fields' AND cc.CHECK_CLAUSE LIKE '%regexp_like%batch_evidence_digest%' AND cc.CHECK_CLAUSE LIKE '%regexp_like%log_evidence_digest%' AND cc.CHECK_CLAUSE LIKE '%regexp_like%manifest_digest%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_definition_identity' AND cc.CHECK_CLAUSE LIKE '%CHECK%' AND cc.CHECK_CLAUSE LIKE '%TRIGGER%' AND cc.CHECK_CLAUSE LIKE '%object_name%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_definition_digests' AND cc.CHECK_CLAUSE LIKE '%regexp_like%definition_digest%' AND cc.CHECK_CLAUSE LIKE '%regexp_like%row_digest%')
    ))<>13 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery checks are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM chain_event_reorg_recovery_definition)<>34
    OR (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS tc
        WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK'
          AND tc.TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_recovery_job',
            'chain_event_reorg_recovery_event','chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch',
            'chain_event_reorg_recovery_manifest','chain_event_reorg_recovery_definition'))
       +(SELECT COUNT(*) FROM information_schema.TRIGGERS
        WHERE TRIGGER_SCHEMA=DATABASE() AND EVENT_OBJECT_TABLE IN ('chain_event_reorg_recovery_run',
          'chain_event_reorg_recovery_job','chain_event_reorg_recovery_event','chain_event_reorg_recovery_log',
          'chain_event_reorg_recovery_batch','chain_event_reorg_recovery_manifest',
          'chain_event_reorg_recovery_definition'))<>34
    OR EXISTS (
      SELECT 1 FROM chain_event_reorg_recovery_definition d
      LEFT JOIN (
        SELECT 'CHECK' object_type,tc.CONSTRAINT_NAME object_name,LOWER(SHA2(CONCAT(
          'joincare.mysql-check-definition.v1',CHAR(10),tc.TABLE_SCHEMA,CHAR(10),tc.TABLE_NAME,CHAR(10),
          tc.ENFORCED,CHAR(10),cc.CHECK_CLAUSE
        ),256)) definition_digest
        FROM information_schema.TABLE_CONSTRAINTS tc
        JOIN information_schema.CHECK_CONSTRAINTS cc ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
        WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK'
          AND tc.TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_recovery_job',
            'chain_event_reorg_recovery_event','chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch',
            'chain_event_reorg_recovery_manifest','chain_event_reorg_recovery_definition')
        UNION ALL
        SELECT 'TRIGGER',TRIGGER_NAME,LOWER(SHA2(CONCAT(
          'joincare.mysql-trigger-definition.v1',CHAR(10),EVENT_OBJECT_SCHEMA,CHAR(10),EVENT_OBJECT_TABLE,CHAR(10),
          ACTION_TIMING,CHAR(10),EVENT_MANIPULATION,CHAR(10),ACTION_ORIENTATION,CHAR(10),ACTION_STATEMENT,CHAR(10),
          SQL_MODE,CHAR(10),DEFINER,CHAR(10),CHARACTER_SET_CLIENT,CHAR(10),COLLATION_CONNECTION,CHAR(10),DATABASE_COLLATION
        ),256))
        FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND EVENT_OBJECT_TABLE IN (
          'chain_event_reorg_recovery_run','chain_event_reorg_recovery_job','chain_event_reorg_recovery_event',
          'chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch','chain_event_reorg_recovery_manifest',
          'chain_event_reorg_recovery_definition')
      ) current_definition ON current_definition.object_type=d.object_type AND current_definition.object_name=d.object_name
      WHERE current_definition.object_name IS NULL OR current_definition.definition_digest<>d.definition_digest
        OR d.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-recovery-definition.v1',CHAR(10),
          d.object_type,CHAR(10),d.object_name,CHAR(10),d.definition_digest),256))
    ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg recovery exact definition attestations are incomplete or drifted';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_reorg_recovery();
DROP PROCEDURE jc_assert_chain_event_reorg_recovery;

-- END 2026_08_23_chain_event_reorg_recovery_scan.sql

-- BEGIN 2026_08_24_chain_event_reorg_materialization_prepare.sql
-- JoinCare IDO Sprint 1A-P3-B2-A · append-only semantic effect preparation.
-- This migration does not change current project, ledger, entitlement, outbox,
-- cursor, checkpoint, or incident state. A later audited generation switch
-- consumes only a completed manifest produced by this evidence layer.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_event_reorg_materialization;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_event_reorg_materialization()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
      'chain_event_sync_state','chain_event_record','chain_event_reorg_incident',
      'chain_event_reorg_recovery_run','chain_event_reorg_recovery_job',
      'chain_event_reorg_recovery_log','chain_event_reorg_recovery_manifest'))<>7 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization prerequisites are missing';
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
      'chain_event_reorg_materialization_run','chain_event_reorg_materialization_old_source',
      'chain_event_reorg_materialization_job',
      'chain_event_reorg_materialization_effect','chain_event_reorg_materialization_branch',
      'chain_event_reorg_materialization_event','chain_event_reorg_materialization_manifest')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='pre-existing reorg materialization tables require manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_event_reorg_materialization();
DROP PROCEDURE jc_preflight_chain_event_reorg_materialization;

CREATE TABLE chain_event_reorg_materialization_run (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_uuid CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  recovery_run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  algorithm_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  recovery_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  legacy_contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  boundary_block BIGINT UNSIGNED NOT NULL,
  boundary_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  old_cursor_block BIGINT UNSIGNED NOT NULL,
  old_cursor_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  target_safe_block BIGINT UNSIGNED NOT NULL,
  target_safe_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  expected_old_record_count BIGINT UNSIGNED NOT NULL,
  old_record_source_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  expected_replacement_log_count BIGINT UNSIGNED NOT NULL,
  replacement_log_source_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_materialization_uuid (run_uuid),
  UNIQUE KEY uk_chain_event_reorg_materialization_recovery (recovery_run_id),
  UNIQUE KEY uk_chain_event_reorg_materialization_incident (incident_id),
  CONSTRAINT fk_chain_event_reorg_materialization_recovery FOREIGN KEY (recovery_run_id)
    REFERENCES chain_event_reorg_recovery_run(id) ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_materialization_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_materialization_run_identity CHECK (
    recovery_run_id>0 AND incident_id>0 AND chain_id>0
    AND run_uuid REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
    AND algorithm_version='joincare-reorg-materialization-prepare-v1'
    AND contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND legacy_contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND contract_address<>legacy_contract_address
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_materialization_run_range CHECK (
    boundary_block>0 AND old_cursor_block>boundary_block AND target_safe_block>=old_cursor_block
    AND boundary_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND old_cursor_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND target_safe_block_hash REGEXP '^0x[0-9a-f]{64}$'
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_materialization_run_sources CHECK (
    recovery_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND old_record_source_digest REGEXP '^[0-9a-f]{64}$'
    AND replacement_log_source_digest REGEXP '^[0-9a-f]{64}$'
    AND expected_old_record_count<=50000 AND expected_replacement_log_count<=50000
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_materialization_old_source (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  chain_event_record_id BIGINT UNSIGNED NOT NULL,
  event VARCHAR(50) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  block_number BIGINT UNSIGNED NOT NULL,
  block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  log_index INT UNSIGNED NOT NULL,
  contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_data MEDIUMBLOB NOT NULL,
  event_data_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_materialization_old_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_materialization_old_record (run_id,chain_event_record_id),
  UNIQUE KEY uk_chain_event_reorg_materialization_old_digest (run_id,row_digest),
  KEY idx_chain_event_reorg_materialization_old_block (run_id,block_number,chain_event_record_id),
  CONSTRAINT fk_chain_event_reorg_materialization_old_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_materialization_old_record FOREIGN KEY (chain_event_record_id)
    REFERENCES chain_event_record(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_materialization_old_source CHECK (
    ordinal>0 AND chain_event_record_id>0 AND block_number>0
    AND block_hash REGEXP '^0x[0-9a-f]{64}$' AND tx_hash REGEXP '^0x[0-9a-f]{64}$'
    AND contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND contract_version IN ('v1_legacy','v2_stablecoin')
    AND OCTET_LENGTH(event_data) BETWEEN 2 AND 1048576
    AND event_data_digest REGEXP '^[0-9a-f]{64}$'
    AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_materialization_job (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  state VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  active_slot TINYINT UNSIGNED NULL,
  next_branch VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  next_old_block BIGINT UNSIGNED NOT NULL,
  next_old_record_id BIGINT UNSIGNED NOT NULL,
  next_replacement_ordinal BIGINT UNSIGNED NOT NULL,
  next_effect_ordinal BIGINT UNSIGNED NOT NULL,
  next_branch_ordinal BIGINT UNSIGNED NOT NULL,
  old_effect_count BIGINT UNSIGNED NOT NULL,
  replacement_effect_count BIGINT UNSIGNED NOT NULL,
  ignored_log_count BIGINT UNSIGNED NOT NULL,
  old_effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  replacement_effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  lease_token VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  lease_expires_at DATETIME(6) NULL,
  fence_epoch BIGINT UNSIGNED NOT NULL DEFAULT 0,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  last_error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  last_error_message VARCHAR(500) NOT NULL DEFAULT '',
  completed_at DATETIME(6) NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_materialization_active (incident_id,active_slot),
  KEY idx_chain_event_reorg_materialization_job_state (state,updated_at,run_id),
  CONSTRAINT fk_chain_event_reorg_materialization_job_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_materialization_job_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_materialization_job_state CHECK (
    state IN ('preparing','completed','invalidated')
    AND ((state='preparing' AND active_slot=1 AND completed_at IS NULL)
      OR (state<>'preparing' AND active_slot IS NULL AND completed_at IS NOT NULL))
    AND next_branch IN ('old_committed','replacement','done')
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_materialization_job_checkpoint CHECK (
    next_old_block>0 AND next_replacement_ordinal>0 AND next_effect_ordinal>0 AND next_branch_ordinal>0
    AND ignored_log_count<=replacement_effect_count AND fence_epoch>=0
    AND old_effect_digest REGEXP '^[0-9a-f]{64}$'
    AND replacement_effect_digest REGEXP '^[0-9a-f]{64}$'
    AND ((lease_token='' AND lease_expires_at IS NULL)
      OR (lease_token REGEXP '^[0-9a-f]{64}$' AND lease_expires_at IS NOT NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_materialization_effect (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  branch VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  branch_ordinal BIGINT UNSIGNED NOT NULL,
  source_table VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  block_number BIGINT UNSIGNED NOT NULL,
  block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  log_index INT UNSIGNED NOT NULL,
  contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_type VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  classification VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  semantic_payload MEDIUMBLOB NOT NULL,
  semantic_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_materialization_effect_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_materialization_effect_branch (run_id,branch,branch_ordinal),
  UNIQUE KEY uk_chain_event_reorg_materialization_effect_source (run_id,branch,source_table,source_id),
  UNIQUE KEY uk_chain_event_reorg_materialization_effect_digest (run_id,effect_digest),
  KEY idx_chain_event_reorg_materialization_effect_project (run_id,branch,project_id,ordinal),
  CONSTRAINT fk_chain_event_reorg_materialization_effect_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_materialization_effect_identity CHECK (
    ordinal>0 AND branch_ordinal>0 AND source_id>0 AND block_number>0
    AND branch IN ('old_committed','replacement')
    AND ((branch='old_committed' AND source_table='chain_event_reorg_materialization_old_source')
      OR (branch='replacement' AND source_table='chain_event_reorg_recovery_log'))
    AND block_hash REGEXP '^0x[0-9a-f]{64}$' AND tx_hash REGEXP '^0x[0-9a-f]{64}$'
    AND contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND contract_version IN ('v1_legacy','v2_stablecoin')
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_materialization_effect_semantics CHECK (
    classification IN ('semantic_event','ignored_abi_log')
    AND ((classification='semantic_event' AND event_type<>'ignored'
        AND (project_id>0
          OR (project_id=0 AND contract_version='v1_legacy' AND event_type='PlatformFeesUpdated')
          OR (project_id=0 AND contract_version='v2_stablecoin'
            AND event_type IN ('PaymentTokenConfigured','ProjectTokenConfigured','PlatformFeesUpdated'))))
      OR (classification='ignored_abi_log' AND branch='replacement' AND project_id=0 AND event_type='ignored'))
    AND OCTET_LENGTH(semantic_payload) BETWEEN 2 AND 1048576
    AND semantic_digest REGEXP '^[0-9a-f]{64}$'
    AND previous_effect_digest REGEXP '^[0-9a-f]{64}$' AND effect_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_materialization_branch (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  branch VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  effect_count BIGINT UNSIGNED NOT NULL,
  ignored_count BIGINT UNSIGNED NOT NULL,
  effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  summary_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  completed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_materialization_branch (run_id,branch),
  UNIQUE KEY uk_chain_event_reorg_materialization_branch_digest (summary_digest),
  CONSTRAINT fk_chain_event_reorg_materialization_branch_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_materialization_branch_fields CHECK (
    branch IN ('old_committed','replacement') AND ignored_count<=effect_count
    AND effect_digest REGEXP '^[0-9a-f]{64}$' AND summary_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_materialization_event (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  sequence_no INT UNSIGNED NOT NULL,
  event_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  stage VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  message VARCHAR(500) NOT NULL,
  previous_event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_materialization_event_sequence (run_id,sequence_no),
  UNIQUE KEY uk_chain_event_reorg_materialization_event_hash (event_hash),
  CONSTRAINT fk_chain_event_reorg_materialization_event_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_materialization_event_fields CHECK (
    sequence_no>0 AND event_type IN ('CREATED','BRANCH_COMPLETED','COMPLETED','INVALIDATED')
    AND stage IN ('old_branch','old_committed','replacement','finalize','prepare')
    AND ((event_type='CREATED' AND sequence_no=1 AND stage='old_branch' AND error_code='')
      OR (event_type='BRANCH_COMPLETED' AND sequence_no>1 AND stage IN ('old_committed','replacement') AND error_code='')
      OR (event_type='COMPLETED' AND sequence_no>1 AND stage='finalize' AND error_code='')
      OR (event_type='INVALIDATED' AND sequence_no>1 AND stage='prepare' AND CHAR_LENGTH(error_code)>0))
    AND CHAR_LENGTH(TRIM(message)) BETWEEN 3 AND 500
    AND previous_event_hash REGEXP '^[0-9a-f]{64}$' AND event_hash REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_materialization_manifest (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  recovery_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  old_effect_count BIGINT UNSIGNED NOT NULL,
  replacement_effect_count BIGINT UNSIGNED NOT NULL,
  ignored_log_count BIGINT UNSIGNED NOT NULL,
  old_effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  replacement_effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  completed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_materialization_manifest_digest (manifest_digest),
  CONSTRAINT fk_chain_event_reorg_materialization_manifest_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_materialization_manifest_fields CHECK (
    incident_id>0 AND ignored_log_count<=replacement_effect_count
    AND recovery_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND old_effect_digest REGEXP '^[0-9a-f]{64}$'
    AND replacement_effect_digest REGEXP '^[0-9a-f]{64}$'
    AND manifest_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_run_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_run_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_run_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_old_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_old_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_old_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_job_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_job_validate_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_job_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_effect_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_effect_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_effect_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_branch_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_branch_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_branch_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_event_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_event_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_event_no_delete;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_manifest_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_manifest_no_update;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_manifest_no_delete;
DELIMITER $$
CREATE TRIGGER trg_chain_event_reorg_materialization_run_validate_insert BEFORE INSERT ON chain_event_reorg_materialization_run FOR EACH ROW
BEGIN
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization run id must be database assigned'; END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_incident i WHERE i.id=NEW.incident_id AND i.status='detected' AND i.active_slot=1
      AND i.chain_id=NEW.chain_id AND i.cursor_block=NEW.old_cursor_block AND i.expected_block_hash=NEW.old_cursor_block_hash) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization run requires matching active incident';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_run r
      JOIN chain_event_reorg_recovery_job j ON j.run_id=r.id AND j.state='completed'
      JOIN chain_event_reorg_recovery_manifest m ON m.run_id=r.id
      WHERE r.id=NEW.recovery_run_id AND r.incident_id=NEW.incident_id AND r.chain_id=NEW.chain_id
      AND r.contract_address=NEW.contract_address AND r.legacy_contract_address=NEW.legacy_contract_address
      AND r.boundary_block=NEW.boundary_block AND r.boundary_block_hash=NEW.boundary_block_hash
      AND r.target_safe_block=NEW.target_safe_block AND r.target_safe_block_hash=NEW.target_safe_block_hash
      AND m.manifest_digest=NEW.recovery_manifest_digest AND m.log_count=NEW.expected_replacement_log_count
      AND m.log_evidence_digest=NEW.replacement_log_source_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization run recovery manifest identity mismatch';
  END IF;
  IF NEW.expected_old_record_count<>(SELECT COUNT(*) FROM chain_event_record e WHERE e.block_number>NEW.boundary_block
      AND e.block_number<=NEW.old_cursor_block AND ((e.contract_version='v1_legacy'
        AND LOWER(e.contract_address)=NEW.legacy_contract_address) OR (e.contract_version='v2_stablecoin'
        AND LOWER(e.contract_address)=NEW.contract_address))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization old source count mismatch';
  END IF;
  IF (SELECT COALESCE(SUM(OCTET_LENGTH(e.data)),0) FROM chain_event_record e WHERE e.block_number>NEW.boundary_block
      AND e.block_number<=NEW.old_cursor_block AND ((e.contract_version='v1_legacy'
        AND LOWER(e.contract_address)=NEW.legacy_contract_address) OR (e.contract_version='v2_stablecoin'
        AND LOWER(e.contract_address)=NEW.contract_address)))>12582912
     OR (SELECT COALESCE(SUM(OCTET_LENGTH(l.raw_log)),0) FROM chain_event_reorg_recovery_log l
        WHERE l.run_id=NEW.recovery_run_id)>12582912 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization source payload limit exceeded';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_run_no_update BEFORE UPDATE ON chain_event_reorg_materialization_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization runs are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_run_no_delete BEFORE DELETE ON chain_event_reorg_materialization_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization runs are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_materialization_old_validate_insert BEFORE INSERT ON chain_event_reorg_materialization_old_source FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64);
  DECLARE expected_ordinal BIGINT UNSIGNED;
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='old source id must be database assigned'; END IF;
  SET expected_previous=COALESCE((SELECT row_digest FROM chain_event_reorg_materialization_old_source
    WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET expected_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_materialization_old_source
    WHERE run_id=NEW.run_id),1);
  IF NEW.ordinal<>expected_ordinal OR NEW.previous_row_digest<>expected_previous THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='old source snapshot chain is not contiguous';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_run mr
      JOIN chain_event_reorg_incident i ON i.id=mr.incident_id AND i.status='detected' AND i.active_slot=1
      JOIN chain_event_record r ON r.id=NEW.chain_event_record_id
      WHERE mr.id=NEW.run_id AND r.block_number>mr.boundary_block AND r.block_number<=mr.old_cursor_block
      AND ((r.contract_version='v1_legacy' AND LOWER(r.contract_address)=mr.legacy_contract_address)
        OR (r.contract_version='v2_stablecoin' AND LOWER(r.contract_address)=mr.contract_address))
      AND r.event=NEW.event AND r.block_number=NEW.block_number AND LOWER(r.block_hash)=NEW.block_hash
      AND LOWER(r.tx_hash)=NEW.tx_hash AND r.log_index=NEW.log_index
      AND LOWER(r.contract_address)=NEW.contract_address AND r.contract_version=NEW.contract_version
      AND SHA2(r.data,256)=NEW.event_data_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='old source snapshot parent identity mismatch';
  END IF;
  IF NEW.event_data_digest<>LOWER(SHA2(NEW.event_data,256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='old source snapshot payload digest mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-old-record-source.v1',CHAR(10),
      NEW.ordinal,CHAR(10),NEW.chain_event_record_id,CHAR(10),NEW.event,CHAR(10),NEW.block_number,CHAR(10),
      NEW.block_hash,CHAR(10),NEW.tx_hash,CHAR(10),NEW.log_index,CHAR(10),NEW.contract_address,CHAR(10),
      NEW.contract_version,CHAR(10),NEW.event_data_digest,CHAR(10),NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='old source snapshot row digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_old_no_update BEFORE UPDATE ON chain_event_reorg_materialization_old_source FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization old source is append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_old_no_delete BEFORE DELETE ON chain_event_reorg_materialization_old_source FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization old source is append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_materialization_job_validate_insert BEFORE INSERT ON chain_event_reorg_materialization_job FOR EACH ROW
BEGIN
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_run r WHERE r.id=NEW.run_id AND r.incident_id=NEW.incident_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization job incident does not match parent run';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_run r WHERE r.id=NEW.run_id
      AND r.expected_old_record_count=(SELECT COUNT(*) FROM chain_event_reorg_materialization_old_source s WHERE s.run_id=NEW.run_id)
      AND r.old_record_source_digest=COALESCE((SELECT s.row_digest FROM chain_event_reorg_materialization_old_source s
        WHERE s.run_id=NEW.run_id ORDER BY s.ordinal DESC LIMIT 1),REPEAT('0',64))
      AND (SELECT COALESCE(SUM(OCTET_LENGTH(s.event_data)),0) FROM chain_event_reorg_materialization_old_source s
        WHERE s.run_id=NEW.run_id)<=12582912) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization old source snapshot is incomplete';
  END IF;
  IF NEW.state<>'preparing' OR NEW.active_slot<>1 OR NEW.next_branch<>'old_committed'
     OR NEW.next_old_record_id<>0 OR NEW.next_replacement_ordinal<>1 OR NEW.next_effect_ordinal<>1
     OR NEW.next_branch_ordinal<>1 OR NEW.old_effect_count<>0 OR NEW.replacement_effect_count<>0
     OR NEW.ignored_log_count<>0 OR NEW.old_effect_digest<>REPEAT('0',64)
     OR NEW.replacement_effect_digest<>REPEAT('0',64) OR NEW.lease_token<>'' OR NEW.lease_expires_at IS NOT NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization job initial checkpoint is not canonical';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_job_validate_update BEFORE UPDATE ON chain_event_reorg_materialization_job FOR EACH ROW
BEGIN
  IF NEW.state='preparing' AND EXISTS (SELECT 1 FROM chain_event_reorg_materialization_event e
      WHERE e.run_id=NEW.run_id AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization terminal event forbids further job preparation';
  END IF;
  IF NEW.run_id<>OLD.run_id OR NEW.incident_id<>OLD.incident_id OR NEW.fence_epoch<OLD.fence_epoch
     OR NEW.next_effect_ordinal<OLD.next_effect_ordinal OR NEW.old_effect_count<OLD.old_effect_count
     OR NEW.replacement_effect_count<OLD.replacement_effect_count OR NEW.ignored_log_count<OLD.ignored_log_count
     OR (OLD.state<>'preparing' AND (NEW.state<>OLD.state OR NEW.active_slot<>OLD.active_slot)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization job transition is not monotonic';
  END IF;
  IF NEW.state='preparing' AND NOT (
      (NEW.fence_epoch=OLD.fence_epoch AND OLD.lease_token REGEXP '^[0-9a-f]{64}$'
        AND OLD.lease_expires_at>CURRENT_TIMESTAMP(6) AND (NEW.lease_token=OLD.lease_token OR NEW.lease_token=''))
      OR (NEW.fence_epoch=OLD.fence_epoch+1 AND (OLD.lease_token='' OR OLD.lease_expires_at<=CURRENT_TIMESTAMP(6))
        AND NEW.lease_token REGEXP '^[0-9a-f]{64}$' AND NEW.lease_expires_at>CURRENT_TIMESTAMP(6)
        AND NEW.state=OLD.state AND NEW.active_slot=OLD.active_slot AND NEW.next_branch=OLD.next_branch
        AND NEW.next_old_block=OLD.next_old_block AND NEW.next_old_record_id=OLD.next_old_record_id
        AND NEW.next_replacement_ordinal=OLD.next_replacement_ordinal
        AND NEW.next_effect_ordinal=OLD.next_effect_ordinal AND NEW.next_branch_ordinal=OLD.next_branch_ordinal
        AND NEW.old_effect_count=OLD.old_effect_count AND NEW.replacement_effect_count=OLD.replacement_effect_count
        AND NEW.ignored_log_count=OLD.ignored_log_count AND NEW.old_effect_digest=OLD.old_effect_digest
        AND NEW.replacement_effect_digest=OLD.replacement_effect_digest AND NEW.attempt_count=OLD.attempt_count+1)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization lease or checkpoint transition rejected';
  END IF;
  IF NEW.state='preparing' AND NOT (
      NEW.next_branch=OLD.next_branch
      OR (OLD.next_branch='old_committed' AND NEW.next_branch='replacement'
        AND EXISTS (SELECT 1 FROM chain_event_reorg_materialization_branch b
          WHERE b.run_id=NEW.run_id AND b.branch='old_committed')
        AND (SELECT COUNT(*) FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
          AND e.event_type='BRANCH_COMPLETED' AND e.stage='old_committed')=1)
      OR (OLD.next_branch='replacement' AND NEW.next_branch='done'
        AND (SELECT COUNT(*) FROM chain_event_reorg_materialization_branch b WHERE b.run_id=NEW.run_id)=2
        AND (SELECT COUNT(*) FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
          AND e.event_type='BRANCH_COMPLETED' AND e.stage='replacement')=1)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization branch transition is not contiguous';
  END IF;
  IF NEW.state IN ('completed','invalidated') AND NOT (OLD.state='preparing' AND OLD.lease_token REGEXP '^[0-9a-f]{64}$'
      AND OLD.lease_expires_at>CURRENT_TIMESTAMP(6) AND NEW.active_slot IS NULL AND NEW.lease_token='' AND NEW.lease_expires_at IS NULL
      AND NEW.next_branch=OLD.next_branch AND NEW.next_old_block=OLD.next_old_block
      AND NEW.next_old_record_id=OLD.next_old_record_id AND NEW.next_replacement_ordinal=OLD.next_replacement_ordinal
      AND NEW.next_effect_ordinal=OLD.next_effect_ordinal AND NEW.next_branch_ordinal=OLD.next_branch_ordinal
      AND NEW.old_effect_count=OLD.old_effect_count AND NEW.replacement_effect_count=OLD.replacement_effect_count
      AND NEW.ignored_log_count=OLD.ignored_log_count AND NEW.old_effect_digest=OLD.old_effect_digest
      AND NEW.replacement_effect_digest=OLD.replacement_effect_digest AND NEW.fence_epoch=OLD.fence_epoch) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization terminal transition requires active DB lease';
  END IF;
  IF NEW.state='completed' AND (NEW.next_branch<>'done'
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_manifest m WHERE m.run_id=NEW.run_id
        AND m.incident_id=NEW.incident_id AND m.old_effect_count=NEW.old_effect_count
        AND m.replacement_effect_count=NEW.replacement_effect_count AND m.ignored_log_count=NEW.ignored_log_count
        AND m.old_effect_digest=NEW.old_effect_digest AND m.replacement_effect_digest=NEW.replacement_effect_digest)
      OR (SELECT COUNT(*) FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
        AND e.event_type IN ('COMPLETED','INVALIDATED'))<>1
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
        AND e.event_type='COMPLETED' AND e.sequence_no=(SELECT MAX(sequence_no) FROM chain_event_reorg_materialization_event WHERE run_id=NEW.run_id))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization completed transition requires matching manifest and terminal event';
  END IF;
  IF NEW.state='invalidated' AND (CHAR_LENGTH(NEW.last_error_code)=0
      OR (SELECT COUNT(*) FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
        AND e.event_type IN ('COMPLETED','INVALIDATED'))<>1
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
        AND e.event_type='INVALIDATED' AND e.error_code=NEW.last_error_code
        AND e.sequence_no=(SELECT MAX(sequence_no) FROM chain_event_reorg_materialization_event WHERE run_id=NEW.run_id))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization invalidated transition requires matching terminal event';
  END IF;
  IF NEW.state='preparing' AND NEW.next_branch='done'
      AND (SELECT COUNT(*) FROM chain_event_reorg_materialization_branch WHERE run_id=NEW.run_id)<>2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization done checkpoint requires both branch summaries';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_job_no_delete BEFORE DELETE ON chain_event_reorg_materialization_job FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization jobs cannot be deleted'; END$$

CREATE TRIGGER trg_chain_event_reorg_materialization_effect_validate_insert BEFORE INSERT ON chain_event_reorg_materialization_effect FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64);
  DECLARE expected_global BIGINT UNSIGNED;
  DECLARE expected_branch BIGINT UNSIGNED;
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization effect id must be database assigned'; END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization terminal event forbids further effects';
  END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_materialization_branch b
      WHERE b.run_id=NEW.run_id AND b.branch=NEW.branch) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization branch summary forbids further effects';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_job WHERE run_id=NEW.run_id
      AND state='preparing' AND active_slot=1 AND next_branch=NEW.branch
      AND lease_token REGEXP '^[0-9a-f]{64}$' AND lease_expires_at>CURRENT_TIMESTAMP(6)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization effect requires active leased branch';
  END IF;
  SET expected_global=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_materialization_effect WHERE run_id=NEW.run_id),
    (SELECT next_effect_ordinal FROM chain_event_reorg_materialization_job WHERE run_id=NEW.run_id));
  SET expected_branch=COALESCE((SELECT MAX(branch_ordinal)+1 FROM chain_event_reorg_materialization_effect WHERE run_id=NEW.run_id AND branch=NEW.branch),
    (SELECT next_branch_ordinal FROM chain_event_reorg_materialization_job WHERE run_id=NEW.run_id));
  SET expected_previous=COALESCE((SELECT effect_digest FROM chain_event_reorg_materialization_effect
      WHERE run_id=NEW.run_id AND branch=NEW.branch ORDER BY branch_ordinal DESC LIMIT 1),
    (SELECT CASE NEW.branch WHEN 'old_committed' THEN old_effect_digest ELSE replacement_effect_digest END
      FROM chain_event_reorg_materialization_job WHERE run_id=NEW.run_id));
  IF expected_global IS NULL OR NEW.ordinal<>expected_global OR NEW.branch_ordinal<>expected_branch
     OR NEW.previous_effect_digest<>expected_previous THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization effect chain is not contiguous or leased';
  END IF;
  IF NEW.branch='old_committed' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_old_source r
      WHERE r.id=NEW.source_id AND r.run_id=NEW.run_id AND r.block_number=NEW.block_number
      AND r.block_hash=NEW.block_hash AND r.tx_hash=NEW.tx_hash AND r.log_index=NEW.log_index
      AND r.contract_address=NEW.contract_address AND r.contract_version=NEW.contract_version
      AND r.event=NEW.event_type AND r.event_data_digest=NEW.semantic_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='old materialization effect source identity mismatch';
  END IF;
  IF NEW.branch='replacement' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_log l
      JOIN chain_event_reorg_materialization_run r ON r.id=NEW.run_id AND r.recovery_run_id=l.run_id
      WHERE l.id=NEW.source_id AND l.block_number=NEW.block_number AND l.block_hash=NEW.block_hash
      AND l.tx_hash=NEW.tx_hash AND l.log_index=NEW.log_index AND l.contract_address=NEW.contract_address
      AND l.contract_version=NEW.contract_version) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='replacement materialization effect source identity mismatch';
  END IF;
  IF NEW.semantic_digest<>LOWER(SHA2(NEW.semantic_payload,256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization semantic payload digest mismatch';
  END IF;
  IF NEW.effect_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-materialization-effect.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.ordinal,CHAR(10),NEW.branch,CHAR(10),NEW.branch_ordinal,CHAR(10),
      NEW.source_table,CHAR(10),NEW.source_id,CHAR(10),NEW.block_number,CHAR(10),NEW.block_hash,CHAR(10),
      NEW.tx_hash,CHAR(10),NEW.log_index,CHAR(10),NEW.contract_address,CHAR(10),NEW.contract_version,CHAR(10),
      NEW.event_type,CHAR(10),NEW.project_id,CHAR(10),NEW.classification,CHAR(10),NEW.semantic_digest,CHAR(10),
      NEW.previous_effect_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization effect digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_effect_no_update BEFORE UPDATE ON chain_event_reorg_materialization_effect FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization effects are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_effect_no_delete BEFORE DELETE ON chain_event_reorg_materialization_effect FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization effects are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_materialization_branch_validate_insert BEFORE INSERT ON chain_event_reorg_materialization_branch FOR EACH ROW
BEGIN
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization branch id must be database assigned'; END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization terminal event forbids further branch summaries';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.active_slot=1 AND j.next_branch=NEW.branch
      AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
      AND ((NEW.branch='old_committed' AND j.old_effect_count=NEW.effect_count AND j.old_effect_digest=NEW.effect_digest)
        OR (NEW.branch='replacement' AND j.replacement_effect_count=NEW.effect_count
          AND j.replacement_effect_digest=NEW.effect_digest AND j.ignored_log_count=NEW.ignored_count))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization branch summary requires complete leased checkpoint';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_run r WHERE r.id=NEW.run_id
      AND ((NEW.branch='old_committed' AND r.expected_old_record_count=NEW.effect_count AND NEW.ignored_count=0)
        OR (NEW.branch='replacement' AND r.expected_replacement_log_count=NEW.effect_count))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization branch count does not match frozen source';
  END IF;
  IF (SELECT COUNT(*) FROM chain_event_reorg_materialization_effect e WHERE e.run_id=NEW.run_id AND e.branch=NEW.branch)<>NEW.effect_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_materialization_effect e WHERE e.run_id=NEW.run_id
       AND e.branch=NEW.branch AND e.classification='ignored_abi_log')<>NEW.ignored_count
     OR COALESCE((SELECT e.effect_digest FROM chain_event_reorg_materialization_effect e
       WHERE e.run_id=NEW.run_id AND e.branch=NEW.branch ORDER BY e.branch_ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.effect_digest THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization branch effect set is incomplete';
  END IF;
  IF NEW.summary_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-materialization-branch.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.branch,CHAR(10),NEW.effect_count,CHAR(10),NEW.ignored_count,CHAR(10),NEW.effect_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization branch summary digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_branch_no_update BEFORE UPDATE ON chain_event_reorg_materialization_branch FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization branch summaries are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_branch_no_delete BEFORE DELETE ON chain_event_reorg_materialization_branch FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization branch summaries are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_materialization_event_validate_insert BEFORE INSERT ON chain_event_reorg_materialization_event FOR EACH ROW
BEGIN
  DECLARE prior_hash VARCHAR(64);
  DECLARE prior_sequence INT UNSIGNED;
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization event id must be database assigned'; END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization terminal event is final';
  END IF;
  SET prior_hash=COALESCE((SELECT event_hash FROM chain_event_reorg_materialization_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),REPEAT('0',64));
  SET prior_sequence=COALESCE((SELECT sequence_no FROM chain_event_reorg_materialization_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),0);
  IF NEW.sequence_no<>prior_sequence+1 OR NEW.previous_event_hash<>prior_hash THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization event chain is not contiguous';
  END IF;
  IF NEW.event_type='CREATED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.active_slot=1 AND j.fence_epoch=0 AND j.lease_token='') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization CREATED event requires canonical unleased initial job';
  END IF;
  IF NEW.event_type<>'CREATED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.active_slot=1 AND j.lease_token REGEXP '^[0-9a-f]{64}$'
      AND j.lease_expires_at>CURRENT_TIMESTAMP(6)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization event requires active DB lease';
  END IF;
  IF NEW.event_type='BRANCH_COMPLETED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_job j
      JOIN chain_event_reorg_materialization_branch b ON b.run_id=j.run_id AND b.branch=NEW.stage
      WHERE j.run_id=NEW.run_id AND j.next_branch=NEW.stage AND j.state='preparing' AND j.active_slot=1) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization branch event requires matching summary and checkpoint';
  END IF;
  IF NEW.event_type='BRANCH_COMPLETED' AND EXISTS (SELECT 1 FROM chain_event_reorg_materialization_event e
      WHERE e.run_id=NEW.run_id AND e.event_type='BRANCH_COMPLETED' AND e.stage=NEW.stage) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization branch event is unique per branch';
  END IF;
  IF NEW.event_type='COMPLETED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_job j
      JOIN chain_event_reorg_materialization_manifest m ON m.run_id=j.run_id AND m.incident_id=j.incident_id
      WHERE j.run_id=NEW.run_id AND j.next_branch='done' AND j.state='preparing' AND j.active_slot=1) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization completed event requires matching manifest';
  END IF;
  IF NEW.event_type='INVALIDATED' AND EXISTS (SELECT 1 FROM chain_event_reorg_materialization_manifest m
      WHERE m.run_id=NEW.run_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization manifest forbids invalidation';
  END IF;
  IF NEW.event_hash<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-materialization-event.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.sequence_no,CHAR(10),NEW.event_type,CHAR(10),NEW.stage,CHAR(10),
      NEW.error_code,CHAR(10),NEW.message,CHAR(10),NEW.previous_event_hash),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization event digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_event_no_update BEFORE UPDATE ON chain_event_reorg_materialization_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization events are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_event_no_delete BEFORE DELETE ON chain_event_reorg_materialization_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization events are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_materialization_manifest_validate_insert BEFORE INSERT ON chain_event_reorg_materialization_manifest FOR EACH ROW
BEGIN
  IF EXISTS (SELECT 1 FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization terminal event forbids manifest insertion';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_job j
      JOIN chain_event_reorg_materialization_run r ON r.id=j.run_id
      WHERE j.run_id=NEW.run_id AND j.incident_id=NEW.incident_id AND j.state='preparing' AND j.active_slot=1
      AND r.incident_id=NEW.incident_id
      AND j.next_branch='done' AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
      AND r.recovery_manifest_digest=NEW.recovery_manifest_digest
      AND j.old_effect_count=NEW.old_effect_count AND j.replacement_effect_count=NEW.replacement_effect_count
      AND j.ignored_log_count=NEW.ignored_log_count AND j.old_effect_digest=NEW.old_effect_digest
      AND j.replacement_effect_digest=NEW.replacement_effect_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization manifest requires complete leased job';
  END IF;
  IF (SELECT COUNT(*) FROM chain_event_reorg_materialization_branch WHERE run_id=NEW.run_id)<>2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization manifest branch summaries are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM chain_event_reorg_materialization_event e WHERE e.run_id=NEW.run_id
      AND e.event_type='BRANCH_COMPLETED' AND e.stage IN ('old_committed','replacement'))<>2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization manifest branch audit events are incomplete';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_branch b WHERE b.run_id=NEW.run_id
      AND b.branch='old_committed' AND b.effect_count=NEW.old_effect_count AND b.ignored_count=0
      AND b.effect_digest=NEW.old_effect_digest)
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_branch b WHERE b.run_id=NEW.run_id
      AND b.branch='replacement' AND b.effect_count=NEW.replacement_effect_count
      AND b.ignored_count=NEW.ignored_log_count AND b.effect_digest=NEW.replacement_effect_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization manifest branch roots mismatch';
  END IF;
  IF NEW.manifest_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-materialization-manifest.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.incident_id,CHAR(10),NEW.recovery_manifest_digest,CHAR(10),
      NEW.old_effect_count,CHAR(10),NEW.replacement_effect_count,CHAR(10),NEW.ignored_log_count,CHAR(10),
      NEW.old_effect_digest,CHAR(10),NEW.replacement_effect_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization manifest digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_manifest_no_update BEFORE UPDATE ON chain_event_reorg_materialization_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization manifests are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_materialization_manifest_no_delete BEFORE DELETE ON chain_event_reorg_materialization_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization manifests are append-only'; END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS jc_assert_chain_event_reorg_materialization;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_reorg_materialization()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'
      AND TABLE_NAME IN ('chain_event_reorg_materialization_run','chain_event_reorg_materialization_old_source',
      'chain_event_reorg_materialization_job',
      'chain_event_reorg_materialization_effect','chain_event_reorg_materialization_branch',
      'chain_event_reorg_materialization_event','chain_event_reorg_materialization_manifest'))<>7 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization tables are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
      AND TRIGGER_NAME REGEXP '^trg_chain_event_reorg_materialization_')<>21 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization triggers are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES'
      AND CONSTRAINT_NAME REGEXP '^chk_chain_event_reorg_materialization_')<>11 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization checks are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_NAME REGEXP '^fk_chain_event_reorg_materialization_' AND DELETE_RULE='RESTRICT')<>10 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg materialization foreign keys are incomplete';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_reorg_materialization();
DROP PROCEDURE jc_assert_chain_event_reorg_materialization;

-- Deployment runner must replace this candidate checksum with the exact file
-- SHA256 before execution and record status=applied only after postflight.

-- END 2026_08_24_chain_event_reorg_materialization_prepare.sql

-- BEGIN 2026_08_25_chain_event_reorg_reversibility_assessment.sql
-- JoinCare IDO Sprint 1A-P3-B2-B0 · fail-closed reversibility assessment.
-- This migration creates evidence only. It cannot switch an effective
-- generation, mutate current facts, move cursor/checkpoints/outbox, or resolve
-- an active reorg incident. Every completed v1 assessment is BLOCKED by the
-- explicit prerequisites required by later application-plan work.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_event_reorg_assessment;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_event_reorg_assessment()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
      'chain_event_sync_state','chain_event_reorg_incident',
      'chain_event_reorg_materialization_run','chain_event_reorg_materialization_job',
      'chain_event_reorg_materialization_effect','chain_event_reorg_materialization_manifest'))<>6 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment prerequisites are missing';
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
      'chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
      'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker',
      'chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='pre-existing reorg assessment tables require manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_event_reorg_assessment();
DROP PROCEDURE jc_preflight_chain_event_reorg_assessment;

CREATE TABLE chain_event_reorg_assessment_run (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_uuid CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  materialization_run_id BIGINT UNSIGNED NOT NULL,
  materialization_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  algorithm_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  boundary_block BIGINT UNSIGNED NOT NULL,
  boundary_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  target_safe_block BIGINT UNSIGNED NOT NULL,
  target_safe_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  expected_effect_count BIGINT UNSIGNED NOT NULL,
  materialization_effect_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_assessment_uuid (run_uuid),
  UNIQUE KEY uk_chain_event_reorg_assessment_materialization (materialization_run_id),
  UNIQUE KEY uk_chain_event_reorg_assessment_incident (incident_id),
  CONSTRAINT fk_chain_event_reorg_assessment_run_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_assessment_run_materialization FOREIGN KEY (materialization_run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_assessment_run_identity CHECK (
    incident_id>0 AND materialization_run_id>0 AND chain_id>0
    AND run_uuid REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
    AND algorithm_version='joincare-reorg-reversibility-assessment-v1'
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_assessment_run_range CHECK (
    boundary_block>0 AND target_safe_block>=boundary_block
    AND boundary_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND target_safe_block_hash REGEXP '^0x[0-9a-f]{64}$'
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_assessment_run_source CHECK (
    expected_effect_count<=99993
    AND materialization_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND materialization_effect_root REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_assessment_job (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  state VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  active_slot TINYINT UNSIGNED NULL,
  next_effect_ordinal BIGINT UNSIGNED NOT NULL,
  disposition_count BIGINT UNSIGNED NOT NULL,
  exact_count BIGINT UNSIGNED NOT NULL,
  conditional_count BIGINT UNSIGNED NOT NULL,
  blocked_count BIGINT UNSIGNED NOT NULL,
  noop_count BIGINT UNSIGNED NOT NULL,
  blocker_count BIGINT UNSIGNED NOT NULL,
  disposition_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  blocker_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  lease_token VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  lease_expires_at DATETIME(6) NULL,
  fence_epoch BIGINT UNSIGNED NOT NULL DEFAULT 0,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  last_error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  last_error_message VARCHAR(500) NOT NULL DEFAULT '',
  completed_at DATETIME(6) NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_assessment_active (incident_id,active_slot),
  KEY idx_chain_event_reorg_assessment_job_state (state,updated_at,run_id),
  CONSTRAINT fk_chain_event_reorg_assessment_job_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_assessment_job_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_assessment_job_state CHECK (
    state IN ('preparing','completed','invalidated')
    AND ((state='preparing' AND active_slot IS NOT NULL AND active_slot=1 AND completed_at IS NULL)
      OR (state<>'preparing' AND active_slot IS NULL AND completed_at IS NOT NULL))
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_assessment_job_checkpoint CHECK (
    next_effect_ordinal>0 AND disposition_count<=100000 AND blocker_count<=100000
    AND exact_count+conditional_count+blocked_count+noop_count=disposition_count
    AND disposition_digest REGEXP '^[0-9a-f]{64}$' AND blocker_digest REGEXP '^[0-9a-f]{64}$'
    AND ((lease_token='' AND lease_expires_at IS NULL)
      OR (lease_token REGEXP '^[0-9a-f]{64}$' AND lease_expires_at IS NOT NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_assessment_disposition (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  materialization_effect_id BIGINT UNSIGNED NOT NULL,
  source_effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  branch VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_type VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  replayability VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  action_kind VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  blocker_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  plan_payload MEDIUMBLOB NOT NULL,
  plan_payload_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_assessment_disposition_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_assessment_disposition_effect (run_id,materialization_effect_id),
  UNIQUE KEY uk_chain_event_reorg_assessment_disposition_digest (run_id,row_digest),
  UNIQUE KEY uk_chain_event_reorg_assessment_disposition_run_id (run_id,id),
  KEY idx_chain_event_reorg_assessment_disposition_project (run_id,project_id,ordinal),
  CONSTRAINT fk_chain_event_reorg_assessment_disposition_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_assessment_disposition_effect FOREIGN KEY (materialization_effect_id)
    REFERENCES chain_event_reorg_materialization_effect(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_assessment_disposition_identity CHECK (
    ordinal>0 AND materialization_effect_id>0 AND project_id<=9223372036854775807
    AND source_effect_digest REGEXP '^[0-9a-f]{64}$'
    AND branch IN ('old_committed','replacement')
    AND contract_version IN ('v1_legacy','v2_stablecoin')
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_assessment_disposition_semantics CHECK (
    replayability IN ('exact','conditional','blocked','noop')
    AND ((replayability IN ('conditional','blocked') AND CHAR_LENGTH(blocker_code)>0)
      OR (replayability IN ('exact','noop') AND blocker_code=''))
    AND action_kind REGEXP '^(apply|reverse)_[a-z0-9_]{3,56}$'
    AND OCTET_LENGTH(plan_payload) BETWEEN 2 AND 1048576
    AND plan_payload_digest REGEXP '^[0-9a-f]{64}$'
    AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_assessment_blocker (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  disposition_id BIGINT UNSIGNED NULL,
  scope VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  blocker_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  safe_detail VARCHAR(500) NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_assessment_blocker_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_assessment_blocker_disposition (run_id,disposition_id),
  UNIQUE KEY uk_chain_event_reorg_assessment_blocker_digest (run_id,row_digest),
  CONSTRAINT fk_chain_event_reorg_assessment_blocker_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_assessment_blocker_disposition FOREIGN KEY (run_id,disposition_id)
    REFERENCES chain_event_reorg_assessment_disposition(run_id,id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_assessment_blocker_fields CHECK (
    ordinal>0 AND scope IN ('run','effect')
    AND ((scope='run' AND disposition_id IS NULL) OR (scope='effect' AND disposition_id IS NOT NULL))
    AND CHAR_LENGTH(blocker_code) BETWEEN 3 AND 64
    AND CHAR_LENGTH(TRIM(safe_detail)) BETWEEN 10 AND 500
    AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_assessment_event (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  sequence_no INT UNSIGNED NOT NULL,
  event_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  message VARCHAR(500) NOT NULL,
  previous_event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_assessment_event_sequence (run_id,sequence_no),
  UNIQUE KEY uk_chain_event_reorg_assessment_event_hash (event_hash),
  CONSTRAINT fk_chain_event_reorg_assessment_event_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_assessment_event_fields CHECK (
    sequence_no>0 AND event_type IN ('CREATED','COMPLETED','INVALIDATED')
    AND ((event_type='CREATED' AND sequence_no=1 AND error_code='')
      OR (event_type='COMPLETED' AND sequence_no>1 AND error_code='')
      OR (event_type='INVALIDATED' AND sequence_no>1 AND CHAR_LENGTH(error_code)>0))
    AND CHAR_LENGTH(TRIM(message)) BETWEEN 3 AND 500
    AND previous_event_hash REGEXP '^[0-9a-f]{64}$' AND event_hash REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_assessment_manifest (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  materialization_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  outcome VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  disposition_count BIGINT UNSIGNED NOT NULL,
  exact_count BIGINT UNSIGNED NOT NULL,
  conditional_count BIGINT UNSIGNED NOT NULL,
  blocked_count BIGINT UNSIGNED NOT NULL,
  noop_count BIGINT UNSIGNED NOT NULL,
  blocker_count BIGINT UNSIGNED NOT NULL,
  disposition_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  blocker_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  completed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_assessment_manifest_digest (manifest_digest),
  CONSTRAINT fk_chain_event_reorg_assessment_manifest_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_assessment_manifest_fields CHECK (
    incident_id>0 AND outcome='BLOCKED' AND blocker_count>=7
    AND exact_count+conditional_count+blocked_count+noop_count=disposition_count
    AND materialization_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND disposition_digest REGEXP '^[0-9a-f]{64}$' AND blocker_digest REGEXP '^[0-9a-f]{64}$'
    AND manifest_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DELIMITER $$
CREATE TRIGGER trg_chain_event_reorg_assessment_run_validate_insert BEFORE INSERT ON chain_event_reorg_assessment_run FOR EACH ROW
BEGIN
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment run id must be database assigned'; END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_incident i WHERE i.id=NEW.incident_id
      AND i.status='detected' AND i.active_slot=1 AND i.chain_id=NEW.chain_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment run requires matching active incident';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_materialization_run r
      JOIN chain_event_reorg_materialization_job j ON j.run_id=r.id AND j.state='completed'
      JOIN chain_event_reorg_materialization_manifest m ON m.run_id=r.id AND m.incident_id=r.incident_id
      WHERE r.id=NEW.materialization_run_id AND r.incident_id=NEW.incident_id AND r.chain_id=NEW.chain_id
      AND r.boundary_block=NEW.boundary_block AND r.boundary_block_hash=NEW.boundary_block_hash
      AND r.target_safe_block=NEW.target_safe_block AND r.target_safe_block_hash=NEW.target_safe_block_hash
      AND m.manifest_digest=NEW.materialization_manifest_digest
      AND m.old_effect_count+m.replacement_effect_count=NEW.expected_effect_count
      AND (SELECT COUNT(*) FROM chain_event_reorg_materialization_effect e WHERE e.run_id=r.id)=NEW.expected_effect_count
      AND (SELECT COALESCE(SUM(OCTET_LENGTH(e.semantic_payload)),0) FROM chain_event_reorg_materialization_effect e
        WHERE e.run_id=r.id)<=67108864
      AND NEW.materialization_effect_root=LOWER(SHA2(CONCAT('joincare.chain-event.reorg-assessment-source.v1',CHAR(10),
        m.run_id,CHAR(10),m.incident_id,CHAR(10),m.old_effect_count,CHAR(10),m.replacement_effect_count,CHAR(10),
        m.ignored_log_count,CHAR(10),m.old_effect_digest,CHAR(10),m.replacement_effect_digest,CHAR(10),m.manifest_digest),256))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment materialization source identity mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_run_no_update BEFORE UPDATE ON chain_event_reorg_assessment_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment runs are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_run_no_delete BEFORE DELETE ON chain_event_reorg_assessment_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment runs are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_assessment_job_validate_insert BEFORE INSERT ON chain_event_reorg_assessment_job FOR EACH ROW
BEGIN
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_run r WHERE r.id=NEW.run_id AND r.incident_id=NEW.incident_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment job incident does not match parent run';
  END IF;
  IF NEW.state<>'preparing' OR NEW.active_slot IS NULL OR NEW.active_slot<>1 OR NEW.next_effect_ordinal<>1
     OR NEW.disposition_count<>0 OR NEW.exact_count<>0 OR NEW.conditional_count<>0 OR NEW.blocked_count<>0
     OR NEW.noop_count<>0 OR NEW.blocker_count<>0 OR NEW.disposition_digest<>REPEAT('0',64)
     OR NEW.blocker_digest<>REPEAT('0',64) OR NEW.lease_token<>'' OR NEW.lease_expires_at IS NOT NULL
     OR NEW.fence_epoch<>0 OR NEW.attempt_count<>0 OR NEW.last_error_code<>'' OR NEW.last_error_message<>'' THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment job initial checkpoint is not canonical';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_job_validate_update BEFORE UPDATE ON chain_event_reorg_assessment_job FOR EACH ROW
BEGIN
  IF NEW.run_id<>OLD.run_id OR NEW.incident_id<>OLD.incident_id OR NEW.fence_epoch<OLD.fence_epoch
     OR NEW.next_effect_ordinal<OLD.next_effect_ordinal OR NEW.disposition_count<OLD.disposition_count
     OR NEW.exact_count<OLD.exact_count OR NEW.conditional_count<OLD.conditional_count
     OR NEW.blocked_count<OLD.blocked_count OR NEW.noop_count<OLD.noop_count OR NEW.blocker_count<OLD.blocker_count
     OR (OLD.state<>'preparing' AND (NEW.state<>OLD.state OR NOT (NEW.active_slot<=>OLD.active_slot))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment job transition is not monotonic';
  END IF;
  IF NEW.state='preparing' AND EXISTS (SELECT 1 FROM chain_event_reorg_assessment_event e
      WHERE e.run_id=NEW.run_id AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment terminal event forbids further preparation';
  END IF;
  IF NEW.state='preparing' AND (NEW.active_slot IS NULL OR NEW.active_slot<>1 OR NOT (
      (NEW.fence_epoch=OLD.fence_epoch AND OLD.lease_token REGEXP '^[0-9a-f]{64}$'
        AND OLD.lease_expires_at>CURRENT_TIMESTAMP(6) AND (NEW.lease_token=OLD.lease_token OR NEW.lease_token=''))
      OR (NEW.fence_epoch=OLD.fence_epoch+1 AND (OLD.lease_token='' OR OLD.lease_expires_at<=CURRENT_TIMESTAMP(6))
        AND NEW.lease_token REGEXP '^[0-9a-f]{64}$' AND NEW.lease_expires_at>CURRENT_TIMESTAMP(6)
        AND NEW.next_effect_ordinal=OLD.next_effect_ordinal AND NEW.disposition_count=OLD.disposition_count
        AND NEW.exact_count=OLD.exact_count AND NEW.conditional_count=OLD.conditional_count
        AND NEW.blocked_count=OLD.blocked_count AND NEW.noop_count=OLD.noop_count AND NEW.blocker_count=OLD.blocker_count
        AND NEW.disposition_digest=OLD.disposition_digest AND NEW.blocker_digest=OLD.blocker_digest
        AND NEW.attempt_count=OLD.attempt_count+1))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment lease or checkpoint transition rejected';
  END IF;
  IF (SELECT COUNT(*) FROM chain_event_reorg_assessment_disposition d WHERE d.run_id=NEW.run_id)<>NEW.disposition_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_assessment_disposition d WHERE d.run_id=NEW.run_id AND d.replayability='exact')<>NEW.exact_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_assessment_disposition d WHERE d.run_id=NEW.run_id AND d.replayability='conditional')<>NEW.conditional_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_assessment_disposition d WHERE d.run_id=NEW.run_id AND d.replayability='blocked')<>NEW.blocked_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_assessment_disposition d WHERE d.run_id=NEW.run_id AND d.replayability='noop')<>NEW.noop_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_assessment_blocker b WHERE b.run_id=NEW.run_id)<>NEW.blocker_count
     OR COALESCE((SELECT d.row_digest FROM chain_event_reorg_assessment_disposition d
       WHERE d.run_id=NEW.run_id ORDER BY d.ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.disposition_digest
     OR COALESCE((SELECT b.row_digest FROM chain_event_reorg_assessment_blocker b
       WHERE b.run_id=NEW.run_id ORDER BY b.ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.blocker_digest THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment job checkpoint does not match evidence rows';
  END IF;
  IF NEW.state IN ('completed','invalidated') AND NOT (OLD.state='preparing'
      AND OLD.lease_token REGEXP '^[0-9a-f]{64}$' AND OLD.lease_expires_at>CURRENT_TIMESTAMP(6)
      AND NEW.active_slot IS NULL AND NEW.lease_token='' AND NEW.lease_expires_at IS NULL
      AND NEW.next_effect_ordinal=OLD.next_effect_ordinal AND NEW.disposition_count=OLD.disposition_count
      AND NEW.exact_count=OLD.exact_count AND NEW.conditional_count=OLD.conditional_count
      AND NEW.blocked_count=OLD.blocked_count AND NEW.noop_count=OLD.noop_count AND NEW.blocker_count=OLD.blocker_count
      AND NEW.disposition_digest=OLD.disposition_digest AND NEW.blocker_digest=OLD.blocker_digest
      AND NEW.fence_epoch=OLD.fence_epoch) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment terminal transition requires active DB lease';
  END IF;
  IF NEW.state='completed' AND (NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_manifest m
      WHERE m.run_id=NEW.run_id AND m.incident_id=NEW.incident_id AND m.outcome='BLOCKED'
      AND m.disposition_count=NEW.disposition_count AND m.exact_count=NEW.exact_count
      AND m.conditional_count=NEW.conditional_count AND m.blocked_count=NEW.blocked_count
      AND m.noop_count=NEW.noop_count AND m.blocker_count=NEW.blocker_count
      AND m.disposition_digest=NEW.disposition_digest AND m.blocker_digest=NEW.blocker_digest)
      OR (SELECT COUNT(*) FROM chain_event_reorg_assessment_event e WHERE e.run_id=NEW.run_id
        AND e.event_type IN ('COMPLETED','INVALIDATED'))<>1
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_event e WHERE e.run_id=NEW.run_id
        AND e.event_type='COMPLETED' AND e.sequence_no=(SELECT MAX(sequence_no) FROM chain_event_reorg_assessment_event WHERE run_id=NEW.run_id))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment completion requires blocked manifest and terminal event';
  END IF;
  IF NEW.state='invalidated' AND (CHAR_LENGTH(NEW.last_error_code)=0
      OR (SELECT COUNT(*) FROM chain_event_reorg_assessment_event e WHERE e.run_id=NEW.run_id
        AND e.event_type IN ('COMPLETED','INVALIDATED'))<>1
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_event e WHERE e.run_id=NEW.run_id
        AND e.event_type='INVALIDATED' AND e.error_code=NEW.last_error_code
        AND e.sequence_no=(SELECT MAX(sequence_no) FROM chain_event_reorg_assessment_event WHERE run_id=NEW.run_id))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment invalidation requires matching terminal event';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_job_no_delete BEFORE DELETE ON chain_event_reorg_assessment_job FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment jobs cannot be deleted'; END$$

CREATE TRIGGER trg_chain_event_reorg_assessment_disposition_validate_insert BEFORE INSERT ON chain_event_reorg_assessment_disposition FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64);
  DECLARE expected_ordinal BIGINT UNSIGNED;
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition id must be database assigned'; END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.active_slot=1
      AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition requires active leased checkpoint';
  END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_assessment_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment terminal event forbids dispositions';
  END IF;
  SET expected_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_assessment_disposition WHERE run_id=NEW.run_id),1);
  SET expected_previous=COALESCE((SELECT row_digest FROM chain_event_reorg_assessment_disposition
      WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),(SELECT disposition_digest FROM chain_event_reorg_assessment_job WHERE run_id=NEW.run_id));
  IF NEW.ordinal<>expected_ordinal OR NEW.previous_row_digest<>expected_previous THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition chain is not contiguous';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_run r
      JOIN chain_event_reorg_materialization_effect e ON e.run_id=r.materialization_run_id
      WHERE r.id=NEW.run_id AND e.id=NEW.materialization_effect_id AND e.ordinal=NEW.ordinal
      AND e.effect_digest=NEW.source_effect_digest AND e.branch=NEW.branch
      AND e.contract_version=NEW.contract_version AND e.event_type=NEW.event_type AND e.project_id=NEW.project_id
      AND ((e.classification='ignored_abi_log' AND NEW.replayability='noop' AND NEW.action_kind='apply_approved_abi_noop'
          AND NEW.branch='replacement' AND NEW.event_type='ignored' AND NEW.project_id=0)
        OR (e.classification='semantic_event' AND NEW.replayability<>'noop' AND NEW.event_type<>'ignored'))
      AND ((NEW.branch='old_committed' AND LEFT(NEW.action_kind,8)='reverse_')
        OR (NEW.branch='replacement' AND LEFT(NEW.action_kind,6)='apply_'))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition source effect mismatch';
  END IF;
  IF JSON_VALID(NEW.plan_payload)<>1
     OR JSON_CONTAINS_PATH(NEW.plan_payload,'all','$.schema_version','$.source_effect_digest','$.branch',
       '$.contract_version','$.event_type','$.project_id','$.replayability','$.action_kind')<>1
     OR JSON_TYPE(JSON_EXTRACT(NEW.plan_payload,'$.schema_version'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(NEW.plan_payload,'$.source_effect_digest'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(NEW.plan_payload,'$.branch'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(NEW.plan_payload,'$.contract_version'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(NEW.plan_payload,'$.event_type'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(NEW.plan_payload,'$.project_id'))<>'INTEGER'
     OR JSON_TYPE(JSON_EXTRACT(NEW.plan_payload,'$.replayability'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(NEW.plan_payload,'$.action_kind'))<>'STRING'
     OR JSON_UNQUOTE(JSON_EXTRACT(NEW.plan_payload,'$.schema_version'))<>'reorg-assessment-plan-v1'
     OR JSON_UNQUOTE(JSON_EXTRACT(NEW.plan_payload,'$.source_effect_digest'))<>NEW.source_effect_digest
     OR JSON_UNQUOTE(JSON_EXTRACT(NEW.plan_payload,'$.branch'))<>NEW.branch
     OR JSON_UNQUOTE(JSON_EXTRACT(NEW.plan_payload,'$.contract_version'))<>NEW.contract_version
     OR JSON_UNQUOTE(JSON_EXTRACT(NEW.plan_payload,'$.event_type'))<>NEW.event_type
     OR CAST(JSON_EXTRACT(NEW.plan_payload,'$.project_id') AS UNSIGNED)<>NEW.project_id
     OR JSON_UNQUOTE(JSON_EXTRACT(NEW.plan_payload,'$.replayability'))<>NEW.replayability
     OR JSON_UNQUOTE(JSON_EXTRACT(NEW.plan_payload,'$.action_kind'))<>NEW.action_kind
     OR ((NEW.blocker_code='' AND (JSON_LENGTH(NEW.plan_payload)<>8 OR JSON_CONTAINS_PATH(NEW.plan_payload,'one','$.blocker_code')=1))
       OR (NEW.blocker_code<>'' AND (JSON_LENGTH(NEW.plan_payload)<>9
         OR JSON_CONTAINS_PATH(NEW.plan_payload,'all','$.blocker_code')<>1
         OR JSON_TYPE(JSON_EXTRACT(NEW.plan_payload,'$.blocker_code'))<>'STRING'
         OR JSON_UNQUOTE(JSON_EXTRACT(NEW.plan_payload,'$.blocker_code'))<>NEW.blocker_code))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition plan payload identity mismatch';
  END IF;
  IF NEW.plan_payload_digest<>LOWER(SHA2(NEW.plan_payload,256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition payload digest mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-assessment-disposition.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.ordinal,CHAR(10),NEW.materialization_effect_id,CHAR(10),NEW.source_effect_digest,CHAR(10),
      NEW.branch,CHAR(10),NEW.contract_version,CHAR(10),NEW.event_type,CHAR(10),NEW.project_id,CHAR(10),
      NEW.replayability,CHAR(10),NEW.action_kind,CHAR(10),NEW.blocker_code,CHAR(10),NEW.plan_payload_digest,CHAR(10),
      NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition row digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_disposition_no_update BEFORE UPDATE ON chain_event_reorg_assessment_disposition FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment dispositions are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_disposition_no_delete BEFORE DELETE ON chain_event_reorg_assessment_disposition FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment dispositions are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_assessment_blocker_validate_insert BEFORE INSERT ON chain_event_reorg_assessment_blocker FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64);
  DECLARE expected_ordinal BIGINT UNSIGNED;
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment blocker id must be database assigned'; END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.active_slot=1 AND j.lease_token REGEXP '^[0-9a-f]{64}$'
      AND j.lease_expires_at>CURRENT_TIMESTAMP(6)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment blocker requires active DB lease';
  END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_assessment_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment terminal event forbids blockers';
  END IF;
  SET expected_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_assessment_blocker WHERE run_id=NEW.run_id),1);
  SET expected_previous=COALESCE((SELECT row_digest FROM chain_event_reorg_assessment_blocker
      WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),(SELECT blocker_digest FROM chain_event_reorg_assessment_job WHERE run_id=NEW.run_id));
  IF NEW.ordinal<>expected_ordinal OR NEW.previous_row_digest<>expected_previous THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment blocker chain is not contiguous';
  END IF;
  IF NEW.scope='effect' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_disposition d
      WHERE d.id=NEW.disposition_id AND d.run_id=NEW.run_id AND d.blocker_code=NEW.blocker_code
      AND d.replayability IN ('conditional','blocked')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment effect blocker does not match disposition';
  END IF;
  IF NEW.scope='run' AND NOT (NEW.ordinal BETWEEN 1 AND 7 AND NEW.blocker_code=CASE NEW.ordinal
      WHEN 1 THEN 'BOUNDARY_STATE_SNAPSHOT_REQUIRED'
      WHEN 2 THEN 'V1_SOURCE_ORIGIN_UNATTESTED'
      WHEN 3 THEN 'GLOBAL_CONFIG_BASELINE_REQUIRED'
      WHEN 4 THEN 'OUTBOX_GENERATION_BINDING_REQUIRED'
      WHEN 5 THEN 'GENERATION_CHECKPOINT_SCHEMA_REQUIRED'
      WHEN 6 THEN 'DUAL_CONTROL_AUTHORIZATION_REQUIRED'
      WHEN 7 THEN 'INDEPENDENT_CHAIN_PREFLIGHT_REQUIRED' END) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment mandatory run blocker is not approved';
  END IF;
  IF NEW.scope='effect' AND NEW.ordinal<=7 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment effect blocker cannot replace mandatory run blockers';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-assessment-blocker.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.ordinal,CHAR(10),COALESCE(NEW.disposition_id,0),CHAR(10),NEW.scope,CHAR(10),
      NEW.blocker_code,CHAR(10),NEW.safe_detail,CHAR(10),NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment blocker row digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_blocker_no_update BEFORE UPDATE ON chain_event_reorg_assessment_blocker FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment blockers are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_blocker_no_delete BEFORE DELETE ON chain_event_reorg_assessment_blocker FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment blockers are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_assessment_event_validate_insert BEFORE INSERT ON chain_event_reorg_assessment_event FOR EACH ROW
BEGIN
  DECLARE prior_hash VARCHAR(64);
  DECLARE prior_sequence INT UNSIGNED;
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment event id must be database assigned'; END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_assessment_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment terminal event is final';
  END IF;
  SET prior_hash=COALESCE((SELECT event_hash FROM chain_event_reorg_assessment_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),REPEAT('0',64));
  SET prior_sequence=COALESCE((SELECT sequence_no FROM chain_event_reorg_assessment_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),0);
  IF NEW.sequence_no<>prior_sequence+1 OR NEW.previous_event_hash<>prior_hash THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment event chain is not contiguous';
  END IF;
  IF NEW.event_type='CREATED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.active_slot=1 AND j.fence_epoch=0 AND j.lease_token='') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment CREATED event requires canonical initial job';
  END IF;
  IF NEW.event_type<>'CREATED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.active_slot=1 AND j.lease_token REGEXP '^[0-9a-f]{64}$'
      AND j.lease_expires_at>CURRENT_TIMESTAMP(6)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment event requires active DB lease';
  END IF;
  IF NEW.event_type='COMPLETED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_manifest m
      WHERE m.run_id=NEW.run_id AND m.outcome='BLOCKED') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment completion event requires blocked manifest';
  END IF;
  IF NEW.event_type='INVALIDATED' AND EXISTS (SELECT 1 FROM chain_event_reorg_assessment_manifest m WHERE m.run_id=NEW.run_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment manifest forbids invalidation';
  END IF;
  IF NEW.event_hash<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-assessment-event.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.sequence_no,CHAR(10),NEW.event_type,CHAR(10),NEW.error_code,CHAR(10),
      NEW.message,CHAR(10),NEW.previous_event_hash),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment event digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_event_no_update BEFORE UPDATE ON chain_event_reorg_assessment_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment events are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_event_no_delete BEFORE DELETE ON chain_event_reorg_assessment_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment events are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_assessment_manifest_validate_insert BEFORE INSERT ON chain_event_reorg_assessment_manifest FOR EACH ROW
BEGIN
  IF EXISTS (SELECT 1 FROM chain_event_reorg_assessment_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment terminal event forbids manifest insertion';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_job j
      JOIN chain_event_reorg_assessment_run r ON r.id=j.run_id AND r.incident_id=j.incident_id
      WHERE j.run_id=NEW.run_id AND j.incident_id=NEW.incident_id AND j.state='preparing' AND j.active_slot=1
      AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
      AND r.materialization_manifest_digest=NEW.materialization_manifest_digest
      AND r.expected_effect_count=NEW.disposition_count
      AND j.disposition_count=NEW.disposition_count AND j.exact_count=NEW.exact_count
      AND j.conditional_count=NEW.conditional_count AND j.blocked_count=NEW.blocked_count
      AND j.noop_count=NEW.noop_count AND j.blocker_count=NEW.blocker_count
      AND j.disposition_digest=NEW.disposition_digest AND j.blocker_digest=NEW.blocker_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment manifest requires complete leased checkpoint';
  END IF;
  IF (SELECT COUNT(*) FROM chain_event_reorg_assessment_disposition WHERE run_id=NEW.run_id)<>NEW.disposition_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_assessment_blocker WHERE run_id=NEW.run_id)<>NEW.blocker_count
     OR COALESCE((SELECT row_digest FROM chain_event_reorg_assessment_disposition WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.disposition_digest
     OR COALESCE((SELECT row_digest FROM chain_event_reorg_assessment_blocker WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.blocker_digest THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment manifest evidence sets are incomplete';
  END IF;
  IF NEW.outcome<>'BLOCKED' OR NEW.blocker_count<7 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment v1 cannot authorize a generation switch';
  END IF;
  IF NEW.manifest_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-assessment-manifest.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.incident_id,CHAR(10),NEW.materialization_manifest_digest,CHAR(10),NEW.outcome,CHAR(10),
      NEW.disposition_count,CHAR(10),NEW.exact_count,CHAR(10),NEW.conditional_count,CHAR(10),NEW.blocked_count,CHAR(10),
      NEW.noop_count,CHAR(10),NEW.blocker_count,CHAR(10),NEW.disposition_digest,CHAR(10),NEW.blocker_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment manifest digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_manifest_no_update BEFORE UPDATE ON chain_event_reorg_assessment_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment manifests are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_assessment_manifest_no_delete BEFORE DELETE ON chain_event_reorg_assessment_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment manifests are append-only'; END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS jc_assert_chain_event_reorg_assessment;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_reorg_assessment()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'
      AND TABLE_NAME IN ('chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
      'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker',
      'chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest'))<>6 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment tables are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
      AND TRIGGER_NAME REGEXP '^trg_chain_event_reorg_assessment_')<>18 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment triggers are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES'
      AND CONSTRAINT_NAME REGEXP '^chk_chain_event_reorg_assessment_')<>10 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment checks are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_NAME REGEXP '^fk_chain_event_reorg_assessment_' AND DELETE_RULE='RESTRICT')<>10 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg assessment foreign keys are incomplete';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_reorg_assessment();
DROP PROCEDURE jc_assert_chain_event_reorg_assessment;

-- Deployment runner must replace this candidate checksum with the exact file
-- SHA256 before execution and record status=applied only after postflight.

-- END 2026_08_25_chain_event_reorg_reversibility_assessment.sql

-- BEGIN 2026_08_25_zz_chain_event_reorg_assessment_fk_restrict.sql
-- Preserve the frozen 2026_08_25 assessment migration while making its
-- intended update/delete restrictions explicit in MySQL information_schema.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_reorg_assessment_fk_restrict;
DELIMITER $$
CREATE PROCEDURE jc_preflight_reorg_assessment_fk_restrict()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_TYPE='FOREIGN KEY'
      AND TABLE_NAME IN ('chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
        'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker',
        'chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest')
  )<>10 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment foreign key count does not match frozen migration baseline';
  END IF;

  IF (
    SELECT COUNT(*) FROM (
      SELECT rc.TABLE_NAME,rc.CONSTRAINT_NAME,rc.REFERENCED_TABLE_NAME,
        rc.UPDATE_RULE,rc.DELETE_RULE,
        GROUP_CONCAT(k.COLUMN_NAME ORDER BY k.ORDINAL_POSITION SEPARATOR ',') child_columns,
        GROUP_CONCAT(k.REFERENCED_COLUMN_NAME ORDER BY k.ORDINAL_POSITION SEPARATOR ',') parent_columns
      FROM information_schema.REFERENTIAL_CONSTRAINTS rc
      JOIN information_schema.KEY_COLUMN_USAGE k
        ON k.CONSTRAINT_SCHEMA=rc.CONSTRAINT_SCHEMA
        AND k.CONSTRAINT_NAME=rc.CONSTRAINT_NAME
        AND k.TABLE_NAME=rc.TABLE_NAME
      WHERE rc.CONSTRAINT_SCHEMA=DATABASE()
        AND rc.UNIQUE_CONSTRAINT_SCHEMA=DATABASE()
        AND k.REFERENCED_TABLE_SCHEMA=DATABASE()
      GROUP BY rc.TABLE_NAME,rc.CONSTRAINT_NAME,rc.REFERENCED_TABLE_NAME,
        rc.UPDATE_RULE,rc.DELETE_RULE
    ) f
    WHERE CONCAT(TABLE_NAME,'|',CONSTRAINT_NAME,'|',child_columns,'|',
      REFERENCED_TABLE_NAME,'|',parent_columns,'|',UPDATE_RULE,'|',DELETE_RULE) IN (
      'chain_event_reorg_assessment_run|fk_chain_event_reorg_assessment_run_incident|incident_id|chain_event_reorg_incident|id|NO ACTION|RESTRICT',
      'chain_event_reorg_assessment_run|fk_chain_event_reorg_assessment_run_materialization|materialization_run_id|chain_event_reorg_materialization_run|id|NO ACTION|RESTRICT',
      'chain_event_reorg_assessment_job|fk_chain_event_reorg_assessment_job_run|run_id|chain_event_reorg_assessment_run|id|NO ACTION|RESTRICT',
      'chain_event_reorg_assessment_job|fk_chain_event_reorg_assessment_job_incident|incident_id|chain_event_reorg_incident|id|NO ACTION|RESTRICT',
      'chain_event_reorg_assessment_disposition|fk_chain_event_reorg_assessment_disposition_run|run_id|chain_event_reorg_assessment_run|id|NO ACTION|RESTRICT',
      'chain_event_reorg_assessment_disposition|fk_chain_event_reorg_assessment_disposition_effect|materialization_effect_id|chain_event_reorg_materialization_effect|id|NO ACTION|RESTRICT',
      'chain_event_reorg_assessment_blocker|fk_chain_event_reorg_assessment_blocker_run|run_id|chain_event_reorg_assessment_run|id|NO ACTION|RESTRICT',
      'chain_event_reorg_assessment_blocker|fk_chain_event_reorg_assessment_blocker_disposition|run_id,disposition_id|chain_event_reorg_assessment_disposition|run_id,id|NO ACTION|RESTRICT',
      'chain_event_reorg_assessment_event|fk_chain_event_reorg_assessment_event_run|run_id|chain_event_reorg_assessment_run|id|NO ACTION|RESTRICT',
      'chain_event_reorg_assessment_manifest|fk_chain_event_reorg_assessment_manifest_run|run_id|chain_event_reorg_assessment_run|id|NO ACTION|RESTRICT'
    )
  )<>10 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment foreign keys do not match frozen migration baseline';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_reorg_assessment_fk_restrict();
DROP PROCEDURE jc_preflight_reorg_assessment_fk_restrict;

ALTER TABLE chain_event_reorg_assessment_run
  ADD CONSTRAINT fk_chain_event_reorg_assessment_run_incident_shadow FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  ADD CONSTRAINT fk_chain_event_reorg_assessment_run_materialization_shadow FOREIGN KEY (materialization_run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_run
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_run_incident,
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_run_materialization;
ALTER TABLE chain_event_reorg_assessment_run
  ADD CONSTRAINT fk_chain_event_reorg_assessment_run_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  ADD CONSTRAINT fk_chain_event_reorg_assessment_run_materialization FOREIGN KEY (materialization_run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_run
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_run_incident_shadow,
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_run_materialization_shadow;

ALTER TABLE chain_event_reorg_assessment_job
  ADD CONSTRAINT fk_chain_event_reorg_assessment_job_run_shadow FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  ADD CONSTRAINT fk_chain_event_reorg_assessment_job_incident_shadow FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_job
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_job_run,
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_job_incident;
ALTER TABLE chain_event_reorg_assessment_job
  ADD CONSTRAINT fk_chain_event_reorg_assessment_job_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  ADD CONSTRAINT fk_chain_event_reorg_assessment_job_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_job
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_job_run_shadow,
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_job_incident_shadow;

ALTER TABLE chain_event_reorg_assessment_disposition
  ADD CONSTRAINT fk_chain_event_reorg_assessment_disposition_run_shadow FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  ADD CONSTRAINT fk_chain_event_reorg_assessment_disposition_effect_shadow FOREIGN KEY (materialization_effect_id)
    REFERENCES chain_event_reorg_materialization_effect(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_disposition
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_disposition_run,
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_disposition_effect;
ALTER TABLE chain_event_reorg_assessment_disposition
  ADD CONSTRAINT fk_chain_event_reorg_assessment_disposition_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  ADD CONSTRAINT fk_chain_event_reorg_assessment_disposition_effect FOREIGN KEY (materialization_effect_id)
    REFERENCES chain_event_reorg_materialization_effect(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_disposition
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_disposition_run_shadow,
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_disposition_effect_shadow;

ALTER TABLE chain_event_reorg_assessment_blocker
  ADD CONSTRAINT fk_chain_event_reorg_assessment_blocker_run_shadow FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  ADD CONSTRAINT fk_chain_event_reorg_assessment_blocker_disposition_shadow FOREIGN KEY (run_id,disposition_id)
    REFERENCES chain_event_reorg_assessment_disposition(run_id,id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_blocker
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_blocker_run,
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_blocker_disposition;
ALTER TABLE chain_event_reorg_assessment_blocker
  ADD CONSTRAINT fk_chain_event_reorg_assessment_blocker_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  ADD CONSTRAINT fk_chain_event_reorg_assessment_blocker_disposition FOREIGN KEY (run_id,disposition_id)
    REFERENCES chain_event_reorg_assessment_disposition(run_id,id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_blocker
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_blocker_run_shadow,
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_blocker_disposition_shadow;

ALTER TABLE chain_event_reorg_assessment_event
  ADD CONSTRAINT fk_chain_event_reorg_assessment_event_run_shadow FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_event
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_event_run;
ALTER TABLE chain_event_reorg_assessment_event
  ADD CONSTRAINT fk_chain_event_reorg_assessment_event_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_event
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_event_run_shadow;

ALTER TABLE chain_event_reorg_assessment_manifest
  ADD CONSTRAINT fk_chain_event_reorg_assessment_manifest_run_shadow FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_manifest
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_manifest_run;
ALTER TABLE chain_event_reorg_assessment_manifest
  ADD CONSTRAINT fk_chain_event_reorg_assessment_manifest_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_manifest
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_manifest_run_shadow;

DROP PROCEDURE IF EXISTS jc_assert_reorg_assessment_fk_restrict;
DELIMITER $$
CREATE PROCEDURE jc_assert_reorg_assessment_fk_restrict()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_TYPE='FOREIGN KEY'
      AND TABLE_NAME IN ('chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
        'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker',
        'chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest')
  )<>10 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment foreign key count changed during RESTRICT normalization';
  END IF;

  IF (
    SELECT COUNT(*) FROM (
      SELECT rc.TABLE_NAME,rc.CONSTRAINT_NAME,rc.REFERENCED_TABLE_NAME,
        rc.UPDATE_RULE,rc.DELETE_RULE,
        GROUP_CONCAT(k.COLUMN_NAME ORDER BY k.ORDINAL_POSITION SEPARATOR ',') child_columns,
        GROUP_CONCAT(k.REFERENCED_COLUMN_NAME ORDER BY k.ORDINAL_POSITION SEPARATOR ',') parent_columns
      FROM information_schema.REFERENTIAL_CONSTRAINTS rc
      JOIN information_schema.KEY_COLUMN_USAGE k
        ON k.CONSTRAINT_SCHEMA=rc.CONSTRAINT_SCHEMA
        AND k.CONSTRAINT_NAME=rc.CONSTRAINT_NAME
        AND k.TABLE_NAME=rc.TABLE_NAME
      WHERE rc.CONSTRAINT_SCHEMA=DATABASE()
        AND rc.UNIQUE_CONSTRAINT_SCHEMA=DATABASE()
        AND k.REFERENCED_TABLE_SCHEMA=DATABASE()
      GROUP BY rc.TABLE_NAME,rc.CONSTRAINT_NAME,rc.REFERENCED_TABLE_NAME,
        rc.UPDATE_RULE,rc.DELETE_RULE
    ) f
    WHERE CONCAT(TABLE_NAME,'|',CONSTRAINT_NAME,'|',child_columns,'|',
      REFERENCED_TABLE_NAME,'|',parent_columns,'|',UPDATE_RULE,'|',DELETE_RULE) IN (
      'chain_event_reorg_assessment_run|fk_chain_event_reorg_assessment_run_incident|incident_id|chain_event_reorg_incident|id|RESTRICT|RESTRICT',
      'chain_event_reorg_assessment_run|fk_chain_event_reorg_assessment_run_materialization|materialization_run_id|chain_event_reorg_materialization_run|id|RESTRICT|RESTRICT',
      'chain_event_reorg_assessment_job|fk_chain_event_reorg_assessment_job_run|run_id|chain_event_reorg_assessment_run|id|RESTRICT|RESTRICT',
      'chain_event_reorg_assessment_job|fk_chain_event_reorg_assessment_job_incident|incident_id|chain_event_reorg_incident|id|RESTRICT|RESTRICT',
      'chain_event_reorg_assessment_disposition|fk_chain_event_reorg_assessment_disposition_run|run_id|chain_event_reorg_assessment_run|id|RESTRICT|RESTRICT',
      'chain_event_reorg_assessment_disposition|fk_chain_event_reorg_assessment_disposition_effect|materialization_effect_id|chain_event_reorg_materialization_effect|id|RESTRICT|RESTRICT',
      'chain_event_reorg_assessment_blocker|fk_chain_event_reorg_assessment_blocker_run|run_id|chain_event_reorg_assessment_run|id|RESTRICT|RESTRICT',
      'chain_event_reorg_assessment_blocker|fk_chain_event_reorg_assessment_blocker_disposition|run_id,disposition_id|chain_event_reorg_assessment_disposition|run_id,id|RESTRICT|RESTRICT',
      'chain_event_reorg_assessment_event|fk_chain_event_reorg_assessment_event_run|run_id|chain_event_reorg_assessment_run|id|RESTRICT|RESTRICT',
      'chain_event_reorg_assessment_manifest|fk_chain_event_reorg_assessment_manifest_run|run_id|chain_event_reorg_assessment_run|id|RESTRICT|RESTRICT'
    )
  )<>10 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment foreign keys were not normalized to RESTRICT';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_reorg_assessment_fk_restrict();
DROP PROCEDURE jc_assert_reorg_assessment_fk_restrict;

-- END 2026_08_25_zz_chain_event_reorg_assessment_fk_restrict.sql

-- BEGIN 2026_08_26_chain_event_reorg_assessment_plan_json.sql
-- MySQL JSON functions reject MEDIUMBLOB input as binary text. Keep the
-- immutable raw plan bytes and SHA-256 binding, but validate their explicit
-- utf8mb4 representation in the assessment disposition trigger.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_reorg_assessment_plan_json;
DELIMITER $$
CREATE PROCEDURE jc_preflight_reorg_assessment_plan_json()
BEGIN
  IF (SELECT COUNT(*) FROM schema_migrations WHERE status='applied' AND (
      (filename='2026_08_25_chain_event_reorg_reversibility_assessment.sql'
        AND checksum='31764df81b1f74ef8a00a8bd785b81f9835dd9132a3e02361c217c8ca98ab68e')
      OR (filename='2026_08_25_zz_chain_event_reorg_assessment_fk_restrict.sql'
        AND checksum='ae2387d475ca19edf3292592e051b7d00bc4ed2e167b3be7d788bb431c799575')
    ))<>2 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment plan JSON prerequisite migrations are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS
      WHERE TRIGGER_SCHEMA=DATABASE()
        AND TRIGGER_NAME='trg_chain_event_reorg_assessment_disposition_validate_insert'
        AND EVENT_OBJECT_TABLE='chain_event_reorg_assessment_disposition'
        AND ACTION_TIMING='BEFORE' AND EVENT_MANIPULATION='INSERT'
        AND ACTION_ORIENTATION='ROW'
        AND LOWER(SHA2(ACTION_STATEMENT,256))='d2c83257179e123d87e8a98bf4a583b2811d9ceb9bafd5de9e46110e908f2e8d'
        AND ACTION_STATEMENT LIKE '%JSON_VALID(NEW.plan_payload)%'
        AND ACTION_STATEMENT NOT LIKE '%CONVERT(NEW.plan_payload USING utf8mb4)%'
        AND ACTION_STATEMENT LIKE '%SHA2(NEW.plan_payload,256)%')<>1 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment plan JSON trigger does not match approved binary-input baseline';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_reorg_assessment_plan_json();
DROP PROCEDURE jc_preflight_reorg_assessment_plan_json;

DROP TRIGGER IF EXISTS trg_chain_event_reorg_assessment_disposition_migration_guard;
DELIMITER $$
CREATE TRIGGER trg_chain_event_reorg_assessment_disposition_migration_guard
BEFORE INSERT ON chain_event_reorg_assessment_disposition FOR EACH ROW
PRECEDES trg_chain_event_reorg_assessment_disposition_validate_insert
BEGIN
  SIGNAL SQLSTATE '45000'
    SET MESSAGE_TEXT='assessment disposition writes are blocked during schema migration';
END$$
DELIMITER ;

DROP TRIGGER trg_chain_event_reorg_assessment_disposition_validate_insert;
DELIMITER $$
CREATE TRIGGER trg_chain_event_reorg_assessment_disposition_validate_insert
BEFORE INSERT ON chain_event_reorg_assessment_disposition FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64);
  DECLARE expected_ordinal BIGINT UNSIGNED;
  DECLARE plan_json LONGTEXT CHARACTER SET utf8mb4;
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition id must be database assigned'; END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.active_slot=1
      AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition requires active leased checkpoint';
  END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_assessment_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment terminal event forbids dispositions';
  END IF;
  SET expected_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_assessment_disposition WHERE run_id=NEW.run_id),1);
  SET expected_previous=COALESCE((SELECT row_digest FROM chain_event_reorg_assessment_disposition
      WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),(SELECT disposition_digest FROM chain_event_reorg_assessment_job WHERE run_id=NEW.run_id));
  IF NEW.ordinal<>expected_ordinal OR NEW.previous_row_digest<>expected_previous THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition chain is not contiguous';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_run r
      JOIN chain_event_reorg_materialization_effect e ON e.run_id=r.materialization_run_id
      WHERE r.id=NEW.run_id AND e.id=NEW.materialization_effect_id AND e.ordinal=NEW.ordinal
      AND e.effect_digest=NEW.source_effect_digest AND e.branch=NEW.branch
      AND e.contract_version=NEW.contract_version AND e.event_type=NEW.event_type AND e.project_id=NEW.project_id
      AND ((e.classification='ignored_abi_log' AND NEW.replayability='noop' AND NEW.action_kind='apply_approved_abi_noop'
          AND NEW.branch='replacement' AND NEW.event_type='ignored' AND NEW.project_id=0)
        OR (e.classification='semantic_event' AND NEW.replayability<>'noop' AND NEW.event_type<>'ignored'))
      AND ((NEW.branch='old_committed' AND LEFT(NEW.action_kind,8)='reverse_')
        OR (NEW.branch='replacement' AND LEFT(NEW.action_kind,6)='apply_'))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition source effect mismatch';
  END IF;
  SET plan_json=CONVERT(NEW.plan_payload USING utf8mb4);
  IF plan_json IS NULL
     OR CONVERT(plan_json USING binary) IS NULL
     OR CONVERT(plan_json USING binary)<>NEW.plan_payload
     OR JSON_VALID(plan_json) IS NULL OR JSON_VALID(plan_json)<>1
     OR JSON_CONTAINS_PATH(plan_json,'all','$.schema_version','$.source_effect_digest','$.branch',
       '$.contract_version','$.event_type','$.project_id','$.replayability','$.action_kind')<>1
     OR JSON_TYPE(JSON_EXTRACT(plan_json,'$.schema_version'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(plan_json,'$.source_effect_digest'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(plan_json,'$.branch'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(plan_json,'$.contract_version'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(plan_json,'$.event_type'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(plan_json,'$.project_id'))<>'INTEGER'
     OR JSON_TYPE(JSON_EXTRACT(plan_json,'$.replayability'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(plan_json,'$.action_kind'))<>'STRING'
     OR JSON_UNQUOTE(JSON_EXTRACT(plan_json,'$.schema_version'))<>'reorg-assessment-plan-v1'
     OR JSON_UNQUOTE(JSON_EXTRACT(plan_json,'$.source_effect_digest'))<>NEW.source_effect_digest
     OR JSON_UNQUOTE(JSON_EXTRACT(plan_json,'$.branch'))<>NEW.branch
     OR JSON_UNQUOTE(JSON_EXTRACT(plan_json,'$.contract_version'))<>NEW.contract_version
     OR JSON_UNQUOTE(JSON_EXTRACT(plan_json,'$.event_type'))<>NEW.event_type
     OR CAST(JSON_EXTRACT(plan_json,'$.project_id') AS UNSIGNED)<>NEW.project_id
     OR JSON_UNQUOTE(JSON_EXTRACT(plan_json,'$.replayability'))<>NEW.replayability
     OR JSON_UNQUOTE(JSON_EXTRACT(plan_json,'$.action_kind'))<>NEW.action_kind
     OR ((NEW.blocker_code='' AND (JSON_LENGTH(plan_json)<>8 OR JSON_CONTAINS_PATH(plan_json,'one','$.blocker_code')=1))
       OR (NEW.blocker_code<>'' AND (JSON_LENGTH(plan_json)<>9
         OR JSON_CONTAINS_PATH(plan_json,'all','$.blocker_code')<>1
         OR JSON_TYPE(JSON_EXTRACT(plan_json,'$.blocker_code'))<>'STRING'
         OR JSON_UNQUOTE(JSON_EXTRACT(plan_json,'$.blocker_code'))<>NEW.blocker_code))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition plan payload identity mismatch';
  END IF;
  IF NEW.plan_payload_digest<>LOWER(SHA2(NEW.plan_payload,256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition payload digest mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-assessment-disposition.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.ordinal,CHAR(10),NEW.materialization_effect_id,CHAR(10),NEW.source_effect_digest,CHAR(10),
      NEW.branch,CHAR(10),NEW.contract_version,CHAR(10),NEW.event_type,CHAR(10),NEW.project_id,CHAR(10),
      NEW.replayability,CHAR(10),NEW.action_kind,CHAR(10),NEW.blocker_code,CHAR(10),NEW.plan_payload_digest,CHAR(10),
      NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition row digest mismatch';
  END IF;
END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS jc_assert_reorg_assessment_plan_json;
DELIMITER $$
CREATE PROCEDURE jc_assert_reorg_assessment_plan_json()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS
      WHERE TRIGGER_SCHEMA=DATABASE()
        AND TRIGGER_NAME='trg_chain_event_reorg_assessment_disposition_validate_insert'
        AND EVENT_OBJECT_TABLE='chain_event_reorg_assessment_disposition'
        AND ACTION_TIMING='BEFORE' AND EVENT_MANIPULATION='INSERT'
        AND ACTION_ORIENTATION='ROW'
        AND LOWER(SHA2(ACTION_STATEMENT,256))='18468a554548fda14e35185e1b49a9739b4a8957b62b96c1c952f906da413564'
        AND ACTION_STATEMENT LIKE '%DECLARE plan_json LONGTEXT CHARACTER SET utf8mb4%'
        AND ACTION_STATEMENT LIKE '%CONVERT(NEW.plan_payload USING utf8mb4)%'
        AND ACTION_STATEMENT LIKE '%CONVERT(plan_json USING binary)<>NEW.plan_payload%'
        AND ACTION_STATEMENT LIKE '%JSON_VALID(plan_json) IS NULL%'
        AND ACTION_STATEMENT LIKE '%JSON_VALID(plan_json)%'
        AND ACTION_STATEMENT LIKE '%JSON_CONTAINS_PATH(plan_json,''all''%'
        AND ACTION_STATEMENT LIKE '%JSON_LENGTH(plan_json)%'
        AND ACTION_STATEMENT LIKE '%SHA2(NEW.plan_payload,256)%'
        AND ACTION_STATEMENT LIKE '%assessment disposition row digest mismatch%')<>1 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment plan JSON trigger normalization is incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS
      WHERE TRIGGER_SCHEMA=DATABASE()
        AND TRIGGER_NAME='trg_chain_event_reorg_assessment_disposition_migration_guard'
        AND EVENT_OBJECT_TABLE='chain_event_reorg_assessment_disposition'
        AND ACTION_TIMING='BEFORE' AND EVENT_MANIPULATION='INSERT'
        AND ACTION_ORIENTATION='ROW'
        AND ACTION_STATEMENT LIKE '%assessment disposition writes are blocked during schema migration%')<>1 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment plan JSON migration guard is unavailable';
  END IF;
  IF NOT EXISTS (SELECT 1
      FROM information_schema.TRIGGERS guard_trigger
      JOIN information_schema.TRIGGERS validator_trigger
        ON validator_trigger.TRIGGER_SCHEMA=guard_trigger.TRIGGER_SCHEMA
       AND validator_trigger.EVENT_OBJECT_TABLE=guard_trigger.EVENT_OBJECT_TABLE
      WHERE guard_trigger.TRIGGER_SCHEMA=DATABASE()
        AND guard_trigger.TRIGGER_NAME='trg_chain_event_reorg_assessment_disposition_migration_guard'
        AND validator_trigger.TRIGGER_NAME='trg_chain_event_reorg_assessment_disposition_validate_insert'
        AND guard_trigger.ACTION_ORDER<validator_trigger.ACTION_ORDER) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment plan JSON migration guard order is unsafe';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS
      WHERE TRIGGER_SCHEMA=DATABASE()
        AND EVENT_OBJECT_TABLE IN ('chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
          'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker',
          'chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest'))<>19 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment trigger set changed during plan JSON normalization';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_reorg_assessment_plan_json();
DROP PROCEDURE jc_assert_reorg_assessment_plan_json;

DROP TRIGGER trg_chain_event_reorg_assessment_disposition_migration_guard;

DROP PROCEDURE IF EXISTS jc_assert_reorg_assessment_plan_json_final;
DELIMITER $$
CREATE PROCEDURE jc_assert_reorg_assessment_plan_json_final()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS
      WHERE TRIGGER_SCHEMA=DATABASE()
        AND EVENT_OBJECT_TABLE IN ('chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
          'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker',
          'chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest'))<>18
     OR EXISTS (SELECT 1 FROM information_schema.TRIGGERS
      WHERE TRIGGER_SCHEMA=DATABASE()
        AND TRIGGER_NAME='trg_chain_event_reorg_assessment_disposition_migration_guard') THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment plan JSON migration guard cleanup is incomplete';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_reorg_assessment_plan_json_final();
DROP PROCEDURE jc_assert_reorg_assessment_plan_json_final;

-- END 2026_08_26_chain_event_reorg_assessment_plan_json.sql

-- BEGIN 2026_08_27_chain_event_reorg_prerequisite_inventory.sql
-- JoinCare IDO Sprint 1A-P3-B2-B1-A · fail-closed prerequisite inventory.
-- V1 records only the seven unresolved prerequisite domains. It cannot accept
-- trusted artifacts, build a candidate generation, mutate current state, bind
-- outbox transactions, move checkpoints/cursor, or resolve an incident.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_event_reorg_prerequisite;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_event_reorg_prerequisite()
BEGIN
  IF (SELECT COUNT(*) FROM schema_migrations WHERE status='applied' AND (
      (filename='2026_08_25_chain_event_reorg_reversibility_assessment.sql'
        AND checksum='31764df81b1f74ef8a00a8bd785b81f9835dd9132a3e02361c217c8ca98ab68e')
      OR (filename='2026_08_25_zz_chain_event_reorg_assessment_fk_restrict.sql'
        AND checksum='ae2387d475ca19edf3292592e051b7d00bc4ed2e167b3be7d788bb431c799575')
      OR (filename='2026_08_26_chain_event_reorg_assessment_plan_json.sql'
        AND checksum='32f952eb708663ed381a407ccea827a53f4c8e7436cc48efde0be8d4201b467b')
    ))<>3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite assessment migrations are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_TYPE='BASE TABLE' AND TABLE_NAME IN ('chain_event_sync_state','chain_event_reorg_incident',
      'chain_event_reorg_assessment_run','chain_event_reorg_assessment_job','chain_event_reorg_assessment_manifest'))<>5 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite source tables are incomplete';
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
      'chain_event_reorg_prerequisite_run','chain_event_reorg_prerequisite_job',
      'chain_event_reorg_prerequisite_artifact','chain_event_reorg_prerequisite_requirement',
      'chain_event_reorg_prerequisite_event','chain_event_reorg_prerequisite_manifest')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='pre-existing reorg prerequisite tables require manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_event_reorg_prerequisite();
DROP PROCEDURE jc_preflight_chain_event_reorg_prerequisite;

CREATE TABLE chain_event_reorg_prerequisite_run (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_uuid CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  assessment_run_id BIGINT UNSIGNED NOT NULL,
  assessment_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  algorithm_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  boundary_block BIGINT UNSIGNED NOT NULL,
  boundary_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  expected_requirement_count BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_uuid (run_uuid),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_assessment (assessment_run_id),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_incident (incident_id),
  CONSTRAINT fk_chain_event_reorg_prerequisite_run_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_prerequisite_run_assessment FOREIGN KEY (assessment_run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_prerequisite_run_identity CHECK (
    incident_id>0 AND assessment_run_id>0 AND chain_id>0 AND boundary_block>0
    AND run_uuid REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
    AND algorithm_version='joincare-reorg-prerequisite-inventory-v1'
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_prerequisite_run_source CHECK (
    assessment_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND boundary_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND expected_requirement_count=7
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_prerequisite_job (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  state VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  active_slot TINYINT UNSIGNED NULL,
  next_requirement_ordinal BIGINT UNSIGNED NOT NULL,
  requirement_count BIGINT UNSIGNED NOT NULL,
  unmet_count BIGINT UNSIGNED NOT NULL,
  untrusted_count BIGINT UNSIGNED NOT NULL,
  deferred_count BIGINT UNSIGNED NOT NULL,
  artifact_count BIGINT UNSIGNED NOT NULL,
  requirement_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  artifact_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  lease_token VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  lease_expires_at DATETIME(6) NULL,
  fence_epoch BIGINT UNSIGNED NOT NULL DEFAULT 0,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  last_error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  last_error_message VARCHAR(500) NOT NULL DEFAULT '',
  completed_at DATETIME(6) NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_active (incident_id,active_slot),
  KEY idx_chain_event_reorg_prerequisite_job_state (state,updated_at,run_id),
  CONSTRAINT fk_chain_event_reorg_prerequisite_job_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_prerequisite_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_prerequisite_job_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_prerequisite_job_state CHECK (
    state IN ('preparing','completed','invalidated')
    AND ((state='preparing' AND active_slot IS NOT NULL AND active_slot=1 AND completed_at IS NULL)
      OR (state<>'preparing' AND active_slot IS NULL AND completed_at IS NOT NULL))
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_prerequisite_job_checkpoint CHECK (
    next_requirement_ordinal BETWEEN 1 AND 8 AND requirement_count<=7
    AND unmet_count+untrusted_count+deferred_count=requirement_count
    AND artifact_count=0 AND artifact_digest=REPEAT('0',64)
    AND requirement_digest REGEXP '^[0-9a-f]{64}$'
    AND ((lease_token='' AND lease_expires_at IS NULL)
      OR (lease_token REGEXP '^[0-9a-f]{64}$' AND lease_expires_at IS NOT NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_prerequisite_artifact (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  requirement_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  artifact_kind VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  schema_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_payload MEDIUMBLOB NOT NULL,
  source_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  safe_summary VARCHAR(500) NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_artifact_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_artifact_digest (run_id,row_digest),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_artifact_run_id (run_id,id),
  CONSTRAINT fk_chain_event_reorg_prerequisite_artifact_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_prerequisite_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_prerequisite_artifact_fields CHECK (
    ordinal>0 AND OCTET_LENGTH(source_payload) BETWEEN 1 AND 1048576
    AND requirement_code REGEXP '^[A-Z0-9_]{3,64}$'
    AND artifact_kind REGEXP '^[a-z0-9_]{3,64}$'
    AND schema_version REGEXP '^[a-z0-9_.-]{3,64}$'
    AND source_kind IN ('database_snapshot','chain_attestation','external_attestation')
    AND source_digest REGEXP '^[0-9a-f]{64}$'
    AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
    AND CHAR_LENGTH(TRIM(safe_summary)) BETWEEN 10 AND 500
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_prerequisite_requirement (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  requirement_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  evidence_status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  artifact_id BIGINT UNSIGNED NULL,
  safe_detail VARCHAR(500) NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_requirement_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_requirement_code (run_id,requirement_code),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_requirement_digest (run_id,row_digest),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_requirement_artifact (run_id,artifact_id),
  CONSTRAINT fk_chain_event_reorg_prerequisite_requirement_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_prerequisite_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_prerequisite_requirement_artifact FOREIGN KEY (run_id,artifact_id)
    REFERENCES chain_event_reorg_prerequisite_artifact(run_id,id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_prerequisite_requirement_fields CHECK (
    ordinal BETWEEN 1 AND 7 AND evidence_status IN ('unmet','deferred') AND artifact_id IS NULL
    AND requirement_code REGEXP '^[A-Z0-9_]{3,64}$'
    AND CHAR_LENGTH(TRIM(safe_detail)) BETWEEN 10 AND 500
    AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_prerequisite_event (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  sequence_no INT UNSIGNED NOT NULL,
  event_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  message VARCHAR(500) NOT NULL,
  previous_event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_event_sequence (run_id,sequence_no),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_event_hash (event_hash),
  CONSTRAINT fk_chain_event_reorg_prerequisite_event_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_prerequisite_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_prerequisite_event_fields CHECK (
    sequence_no>0 AND event_type IN ('CREATED','COMPLETED','INVALIDATED')
    AND ((event_type='INVALIDATED' AND CHAR_LENGTH(error_code)>0) OR (event_type<>'INVALIDATED' AND error_code=''))
    AND CHAR_LENGTH(TRIM(message)) BETWEEN 10 AND 500
    AND previous_event_hash REGEXP '^[0-9a-f]{64}$' AND event_hash REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_prerequisite_manifest (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  assessment_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  outcome VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  candidate_ready TINYINT UNSIGNED NOT NULL,
  requirement_count BIGINT UNSIGNED NOT NULL,
  satisfied_count BIGINT UNSIGNED NOT NULL,
  unmet_count BIGINT UNSIGNED NOT NULL,
  untrusted_count BIGINT UNSIGNED NOT NULL,
  deferred_count BIGINT UNSIGNED NOT NULL,
  artifact_count BIGINT UNSIGNED NOT NULL,
  requirement_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  artifact_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  completed_at DATETIME(6) NOT NULL,
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_prerequisite_manifest_digest (manifest_digest),
  CONSTRAINT fk_chain_event_reorg_prerequisite_manifest_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_prerequisite_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_prerequisite_manifest_fields CHECK (
    outcome='BLOCKED' AND candidate_ready=0 AND requirement_count=7 AND satisfied_count=0
    AND unmet_count=5 AND untrusted_count=0 AND deferred_count=2 AND artifact_count=0
    AND assessment_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND requirement_digest REGEXP '^[0-9a-f]{64}$'
    AND artifact_digest=REPEAT('0',64) AND event_digest REGEXP '^[0-9a-f]{64}$'
    AND manifest_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DELIMITER $$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_run_validate_insert BEFORE INSERT ON chain_event_reorg_prerequisite_run FOR EACH ROW
BEGIN
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite run id must be database assigned'; END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_incident i
      JOIN chain_event_reorg_assessment_run a ON a.incident_id=i.id
      JOIN chain_event_reorg_assessment_job j ON j.run_id=a.id AND j.incident_id=i.id
      JOIN chain_event_reorg_assessment_manifest m ON m.run_id=a.id AND m.incident_id=i.id
      WHERE i.id=NEW.incident_id AND i.active_slot=1 AND i.status='detected'
      AND a.id=NEW.assessment_run_id AND a.chain_id=NEW.chain_id
      AND a.boundary_block=NEW.boundary_block AND a.boundary_block_hash=NEW.boundary_block_hash
      AND j.state='completed' AND m.outcome='BLOCKED'
      AND m.manifest_digest=NEW.assessment_manifest_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite run source assessment identity mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_run_no_update BEFORE UPDATE ON chain_event_reorg_prerequisite_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite runs are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_run_no_delete BEFORE DELETE ON chain_event_reorg_prerequisite_run FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite runs are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_prerequisite_job_validate_insert BEFORE INSERT ON chain_event_reorg_prerequisite_job FOR EACH ROW
BEGIN
  IF NEW.state<>'preparing' OR NEW.active_slot IS NULL OR NEW.active_slot<>1 OR NEW.next_requirement_ordinal<>1
     OR NEW.requirement_count<>0 OR NEW.unmet_count<>0 OR NEW.untrusted_count<>0 OR NEW.deferred_count<>0
     OR NEW.artifact_count<>0 OR NEW.requirement_digest<>REPEAT('0',64) OR NEW.artifact_digest<>REPEAT('0',64)
     OR NEW.lease_token<>'' OR NEW.lease_expires_at IS NOT NULL OR NEW.fence_epoch<>0 OR NEW.attempt_count<>0
     OR NEW.last_error_code<>'' OR NEW.last_error_message<>'' OR NEW.completed_at IS NOT NULL
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_run r WHERE r.id=NEW.run_id AND r.incident_id=NEW.incident_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite job requires canonical initial state';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_job_validate_update BEFORE UPDATE ON chain_event_reorg_prerequisite_job FOR EACH ROW
BEGIN
  IF NEW.run_id<>OLD.run_id OR NEW.incident_id<>OLD.incident_id OR NEW.fence_epoch<OLD.fence_epoch
     OR NEW.attempt_count<OLD.attempt_count OR NEW.requirement_count<OLD.requirement_count
     OR NEW.next_requirement_ordinal<OLD.next_requirement_ordinal OR NEW.artifact_count<>0
     OR NEW.artifact_digest<>REPEAT('0',64) OR OLD.state<>'preparing' THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite job transition is not allowed';
  END IF;
  IF NEW.state='preparing' AND EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_event e
      WHERE e.run_id=NEW.run_id AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite terminal event forbids preparing job updates';
  END IF;
  IF NEW.state IN ('completed','invalidated') AND (
      OLD.active_slot IS NULL OR OLD.active_slot<>1 OR OLD.lease_token NOT REGEXP '^[0-9a-f]{64}$'
      OR OLD.lease_expires_at IS NULL OR OLD.lease_expires_at<=CURRENT_TIMESTAMP(6)
      OR NEW.active_slot IS NOT NULL OR NEW.lease_token<>'' OR NEW.lease_expires_at IS NOT NULL
      OR NEW.fence_epoch<>OLD.fence_epoch OR NEW.attempt_count<>OLD.attempt_count
      OR NEW.next_requirement_ordinal<>OLD.next_requirement_ordinal
      OR NEW.requirement_count<>OLD.requirement_count OR NEW.unmet_count<>OLD.unmet_count
      OR NEW.untrusted_count<>OLD.untrusted_count OR NEW.deferred_count<>OLD.deferred_count
      OR NEW.requirement_digest<>OLD.requirement_digest OR NEW.artifact_count<>OLD.artifact_count
      OR NEW.artifact_digest<>OLD.artifact_digest OR NEW.completed_at IS NULL) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite terminal transition requires the active fenced lease';
  END IF;
  IF NEW.state='completed' AND (
      NEW.last_error_code<>'' OR NEW.last_error_message<>'' OR NEW.requirement_count<>7
      OR NEW.unmet_count<>5 OR NEW.untrusted_count<>0 OR NEW.deferred_count<>2
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_event e
          WHERE e.run_id=NEW.run_id AND e.sequence_no=2 AND e.event_type='COMPLETED')
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_manifest m
          WHERE m.run_id=NEW.run_id AND m.outcome='BLOCKED' AND m.candidate_ready=0)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite completion requires terminal evidence and blocked manifest';
  END IF;
  IF NEW.state='invalidated' AND (
      CHAR_LENGTH(NEW.last_error_code)=0 OR CHAR_LENGTH(NEW.last_error_message)=0
      OR EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_manifest m WHERE m.run_id=NEW.run_id)
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_event e
          WHERE e.run_id=NEW.run_id AND e.sequence_no=2 AND e.event_type='INVALIDATED'
          AND e.error_code=NEW.last_error_code AND e.message=NEW.last_error_message)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite invalidation requires matching terminal evidence';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_job_no_delete BEFORE DELETE ON chain_event_reorg_prerequisite_job FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite jobs cannot be deleted'; END$$

CREATE TRIGGER trg_chain_event_reorg_prerequisite_artifact_validate_insert BEFORE INSERT ON chain_event_reorg_prerequisite_artifact FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='B1-A v1 forbids prerequisite artifacts'; END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_artifact_no_update BEFORE UPDATE ON chain_event_reorg_prerequisite_artifact FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite artifacts are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_artifact_no_delete BEFORE DELETE ON chain_event_reorg_prerequisite_artifact FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite artifacts are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_prerequisite_requirement_validate_insert BEFORE INSERT ON chain_event_reorg_prerequisite_requirement FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64);
  DECLARE expected_ordinal BIGINT UNSIGNED;
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite requirement id must be database assigned'; END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.active_slot=1 AND j.lease_token REGEXP '^[0-9a-f]{64}$'
      AND j.lease_expires_at>CURRENT_TIMESTAMP(6)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite requirement requires active DB lease';
  END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite terminal event forbids requirements';
  END IF;
  SET expected_previous=COALESCE((SELECT row_digest FROM chain_event_reorg_prerequisite_requirement
      WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),
      (SELECT requirement_digest FROM chain_event_reorg_prerequisite_job WHERE run_id=NEW.run_id));
  SET expected_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_prerequisite_requirement WHERE run_id=NEW.run_id),1);
  IF NEW.ordinal<>expected_ordinal OR NEW.previous_row_digest<>expected_previous OR NEW.artifact_id IS NOT NULL
     OR NEW.requirement_code<>CASE NEW.ordinal
       WHEN 1 THEN 'BOUNDARY_STATE_SNAPSHOT_REQUIRED'
       WHEN 2 THEN 'V1_SOURCE_ORIGIN_UNATTESTED'
       WHEN 3 THEN 'GLOBAL_CONFIG_BASELINE_REQUIRED'
       WHEN 4 THEN 'OUTBOX_GENERATION_BINDING_REQUIRED'
       WHEN 5 THEN 'GENERATION_CHECKPOINT_SCHEMA_REQUIRED'
       WHEN 6 THEN 'DUAL_CONTROL_AUTHORIZATION_REQUIRED'
       WHEN 7 THEN 'INDEPENDENT_CHAIN_PREFLIGHT_REQUIRED' END
     OR NEW.evidence_status<>CASE WHEN NEW.ordinal BETWEEN 1 AND 5 THEN 'unmet' ELSE 'deferred' END THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite requirement matrix is not approved';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-prerequisite-requirement.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.ordinal,CHAR(10),NEW.requirement_code,CHAR(10),NEW.evidence_status,CHAR(10),
      0,CHAR(10),NEW.safe_detail,CHAR(10),NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite requirement digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_requirement_no_update BEFORE UPDATE ON chain_event_reorg_prerequisite_requirement FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite requirements are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_requirement_no_delete BEFORE DELETE ON chain_event_reorg_prerequisite_requirement FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite requirements are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_prerequisite_event_validate_insert BEFORE INSERT ON chain_event_reorg_prerequisite_event FOR EACH ROW
BEGIN
  DECLARE prior_hash VARCHAR(64);
  DECLARE prior_sequence INT UNSIGNED;
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite event id must be database assigned'; END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite terminal event is final';
  END IF;
  SET prior_hash=COALESCE((SELECT event_hash FROM chain_event_reorg_prerequisite_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),REPEAT('0',64));
  SET prior_sequence=COALESCE((SELECT sequence_no FROM chain_event_reorg_prerequisite_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),0);
  IF NEW.sequence_no<>prior_sequence+1 OR NEW.previous_event_hash<>prior_hash
     OR (NEW.event_type='CREATED' AND (NEW.sequence_no<>1 OR NEW.previous_event_hash<>REPEAT('0',64)))
     OR (NEW.event_type IN ('COMPLETED','INVALIDATED') AND NEW.sequence_no<>2) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite event chain is not contiguous';
  END IF;
  IF NEW.event_type='CREATED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.fence_epoch=0 AND j.lease_token='') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite CREATED event requires canonical initial job';
  END IF;
  IF NEW.event_type<>'CREATED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite event requires active DB lease';
  END IF;
  IF NEW.event_type='COMPLETED' AND (
      EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_manifest m WHERE m.run_id=NEW.run_id)
      OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_job j WHERE j.run_id=NEW.run_id
          AND j.requirement_count=7 AND j.unmet_count=5 AND j.untrusted_count=0 AND j.deferred_count=2
          AND j.artifact_count=0 AND j.next_requirement_ordinal=8)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite completion event requires complete leased checkpoint';
  END IF;
  IF NEW.event_type='INVALIDATED' AND EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_manifest m WHERE m.run_id=NEW.run_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite manifest forbids invalidation';
  END IF;
  IF NEW.event_hash<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-prerequisite-event.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.sequence_no,CHAR(10),NEW.event_type,CHAR(10),NEW.error_code,CHAR(10),
      NEW.message,CHAR(10),NEW.previous_event_hash),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite event digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_event_no_update BEFORE UPDATE ON chain_event_reorg_prerequisite_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite events are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_event_no_delete BEFORE DELETE ON chain_event_reorg_prerequisite_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite events are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_prerequisite_manifest_validate_insert BEFORE INSERT ON chain_event_reorg_prerequisite_manifest FOR EACH ROW
BEGIN
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_job j
      JOIN chain_event_reorg_prerequisite_run r ON r.id=j.run_id AND r.incident_id=j.incident_id
      WHERE j.run_id=NEW.run_id AND j.incident_id=NEW.incident_id AND j.state='preparing' AND j.active_slot=1
      AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
      AND r.assessment_manifest_digest=NEW.assessment_manifest_digest
      AND j.requirement_count=NEW.requirement_count AND j.unmet_count=NEW.unmet_count
      AND j.untrusted_count=NEW.untrusted_count AND j.deferred_count=NEW.deferred_count
      AND j.artifact_count=NEW.artifact_count AND j.requirement_digest=NEW.requirement_digest
      AND j.artifact_digest=NEW.artifact_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite manifest requires complete leased checkpoint';
  END IF;
  IF (SELECT COUNT(*) FROM chain_event_reorg_prerequisite_requirement WHERE run_id=NEW.run_id)<>7
     OR (SELECT COUNT(*) FROM chain_event_reorg_prerequisite_requirement WHERE run_id=NEW.run_id AND evidence_status='unmet')<>5
     OR (SELECT COUNT(*) FROM chain_event_reorg_prerequisite_requirement WHERE run_id=NEW.run_id AND evidence_status='deferred')<>2
     OR EXISTS (SELECT 1 FROM chain_event_reorg_prerequisite_artifact WHERE run_id=NEW.run_id)
     OR (SELECT COUNT(*) FROM chain_event_reorg_prerequisite_event WHERE run_id=NEW.run_id
         AND sequence_no=2 AND event_type='COMPLETED' AND event_hash=NEW.event_digest)<>1
     OR (SELECT COUNT(*) FROM chain_event_reorg_prerequisite_event WHERE run_id=NEW.run_id)<>2
     OR COALESCE((SELECT row_digest FROM chain_event_reorg_prerequisite_requirement WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.requirement_digest THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite manifest evidence sets are incomplete';
  END IF;
  IF NEW.outcome<>'BLOCKED' OR NEW.candidate_ready<>0 OR NEW.satisfied_count<>0 OR NEW.artifact_count<>0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite v1 cannot make a candidate generation ready';
  END IF;
  IF NEW.manifest_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-prerequisite-manifest.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.incident_id,CHAR(10),NEW.assessment_manifest_digest,CHAR(10),NEW.outcome,CHAR(10),
      'false',CHAR(10),NEW.requirement_count,CHAR(10),NEW.satisfied_count,CHAR(10),NEW.unmet_count,CHAR(10),
      NEW.untrusted_count,CHAR(10),NEW.deferred_count,CHAR(10),NEW.artifact_count,CHAR(10),
      NEW.requirement_digest,CHAR(10),NEW.artifact_digest,CHAR(10),NEW.event_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='prerequisite manifest digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_manifest_no_update BEFORE UPDATE ON chain_event_reorg_prerequisite_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite manifests are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_prerequisite_manifest_no_delete BEFORE DELETE ON chain_event_reorg_prerequisite_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite manifests are append-only'; END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS jc_assert_chain_event_reorg_prerequisite;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_reorg_prerequisite()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'
      AND TABLE_NAME IN ('chain_event_reorg_prerequisite_run','chain_event_reorg_prerequisite_job',
      'chain_event_reorg_prerequisite_artifact','chain_event_reorg_prerequisite_requirement',
      'chain_event_reorg_prerequisite_event','chain_event_reorg_prerequisite_manifest'))<>6 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite tables are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
      AND EVENT_OBJECT_TABLE IN ('chain_event_reorg_prerequisite_run','chain_event_reorg_prerequisite_job',
      'chain_event_reorg_prerequisite_artifact','chain_event_reorg_prerequisite_requirement',
      'chain_event_reorg_prerequisite_event','chain_event_reorg_prerequisite_manifest'))<>18 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite triggers are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES'
      AND TABLE_NAME IN ('chain_event_reorg_prerequisite_run','chain_event_reorg_prerequisite_job',
      'chain_event_reorg_prerequisite_artifact','chain_event_reorg_prerequisite_requirement',
      'chain_event_reorg_prerequisite_event','chain_event_reorg_prerequisite_manifest'))<>8 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite checks are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_NAME REGEXP '^fk_chain_event_reorg_prerequisite_'
      AND UPDATE_RULE='RESTRICT' AND DELETE_RULE='RESTRICT')<>9 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg prerequisite foreign keys are incomplete';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_reorg_prerequisite();
DROP PROCEDURE jc_assert_chain_event_reorg_prerequisite;

-- Deployment runner records this file checksum and status=applied only after
-- every postflight assertion succeeds.

-- END 2026_08_27_chain_event_reorg_prerequisite_inventory.sql

-- BEGIN 2026_08_28_chain_event_reorg_generation_foundation.sql
-- JoinCare IDO Sprint 1A-P3-B2-B1-B0 · fail-closed candidate generation foundation.
-- This migration freezes a complete runtime outbox/nonce snapshot and the five
-- B1-B requirements. It cannot admit trusted attestations, create candidate
-- financial facts, write a generation checkpoint, switch effective state, or
-- resolve the incident.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_event_reorg_generation;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_event_reorg_generation()
BEGIN
  IF (SELECT COUNT(*) FROM schema_migrations WHERE status='applied' AND filename='2026_08_27_chain_event_reorg_prerequisite_inventory.sql'
      AND checksum='fa41cc6655bc2be808ca3b45e4ff42590d0126f6ed8dedc37141d553a8704e0a')<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation prerequisite migration is incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE' AND TABLE_NAME IN (
      'chain_event_sync_state','chain_event_reorg_incident','chain_event_reorg_materialization_run',
      'chain_event_reorg_materialization_manifest','chain_event_reorg_assessment_run','chain_event_reorg_assessment_manifest',
      'chain_event_reorg_prerequisite_run','chain_event_reorg_prerequisite_job','chain_event_reorg_prerequisite_manifest',
      'chain_tx_submission','chain_relayer_nonce'))<>11 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation source tables are incomplete';
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
      'chain_event_reorg_generation','chain_event_reorg_generation_job','chain_event_reorg_generation_sender','chain_event_reorg_generation_submission',
      'chain_event_reorg_generation_nonce','chain_event_reorg_generation_requirement','chain_event_reorg_generation_event',
      'chain_event_reorg_generation_manifest','chain_event_reorg_candidate_project','chain_event_reorg_candidate_ledger',
      'chain_event_reorg_candidate_v2_entitlement','chain_event_reorg_candidate_v1_fact',
      'chain_event_reorg_candidate_wallet_aggregate','chain_event_reorg_candidate_delta',
      'chain_event_reorg_generation_checkpoint')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='pre-existing reorg generation tables require manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_event_reorg_generation();
DROP PROCEDURE jc_preflight_chain_event_reorg_generation;

-- The snapshot predicate is chain+sender across every state/contract. The
-- existing sender+nonce unique key cannot range-lock unsigned nonce=NULL rows.
ALTER TABLE chain_tx_submission
  ADD KEY idx_chain_tx_generation_sender (chain_id,sender_address,id);

CREATE TABLE chain_event_reorg_generation (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  generation_uuid CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  attempt_no INT UNSIGNED NOT NULL,
  generation_kind VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  prerequisite_run_id BIGINT UNSIGNED NOT NULL,
  prerequisite_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  assessment_run_id BIGINT UNSIGNED NOT NULL,
  assessment_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  materialization_run_id BIGINT UNSIGNED NOT NULL,
  materialization_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  boundary_block BIGINT UNSIGNED NOT NULL,
  boundary_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  target_safe_block BIGINT UNSIGNED NOT NULL,
  target_safe_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  legacy_contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  v2_contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  old_effect_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  replacement_effect_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  algorithm_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  schema_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  descriptor_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_generation_uuid (generation_uuid),
  UNIQUE KEY uk_chain_event_reorg_generation_attempt (incident_id,attempt_no),
  UNIQUE KEY uk_chain_event_reorg_generation_descriptor (descriptor_digest),
  KEY idx_chain_event_reorg_generation_prerequisite (prerequisite_run_id),
  CONSTRAINT fk_chain_event_reorg_generation_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_generation_prerequisite FOREIGN KEY (prerequisite_run_id)
    REFERENCES chain_event_reorg_prerequisite_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_generation_assessment FOREIGN KEY (assessment_run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_generation_materialization FOREIGN KEY (materialization_run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_generation_identity CHECK (
    incident_id>0 AND attempt_no>0 AND prerequisite_run_id>0 AND assessment_run_id>0 AND materialization_run_id>0
    AND chain_id>0 AND boundary_block>0 AND target_safe_block>=boundary_block
    AND generation_kind='candidate'
    AND generation_uuid REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
    AND algorithm_version='joincare-reorg-generation-foundation-v1' AND schema_version='reorg-generation-v1'
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_generation_sources CHECK (
    prerequisite_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND assessment_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND materialization_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND old_effect_root REGEXP '^[0-9a-f]{64}$' AND replacement_effect_root REGEXP '^[0-9a-f]{64}$'
    AND descriptor_digest REGEXP '^[0-9a-f]{64}$'
    AND boundary_block_hash REGEXP '^0x[0-9a-f]{64}$' AND target_safe_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND legacy_contract_address REGEXP '^0x[0-9a-f]{40}$' AND legacy_contract_address<>CONCAT('0x',REPEAT('0',40))
    AND v2_contract_address REGEXP '^0x[0-9a-f]{40}$' AND v2_contract_address<>CONCAT('0x',REPEAT('0',40))
    AND legacy_contract_address<>v2_contract_address
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_generation_job (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  state VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  stage VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  active_slot TINYINT UNSIGNED NULL,
  sender_count BIGINT UNSIGNED NOT NULL,
  submission_count BIGINT UNSIGNED NOT NULL,
  submission_payload_bytes BIGINT UNSIGNED NOT NULL,
  nonce_count BIGINT UNSIGNED NOT NULL,
  requirement_count BIGINT UNSIGNED NOT NULL,
  unmet_count BIGINT UNSIGNED NOT NULL,
  verified_count BIGINT UNSIGNED NOT NULL,
  sender_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  submission_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  nonce_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  requirement_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  lease_token VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  lease_expires_at DATETIME(6) NULL,
  fence_epoch BIGINT UNSIGNED NOT NULL DEFAULT 0,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  last_error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  last_error_message VARCHAR(500) NOT NULL DEFAULT '',
  completed_at DATETIME(6) NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_generation_active (incident_id,active_slot),
  KEY idx_chain_event_reorg_generation_job_state (state,updated_at,run_id),
  CONSTRAINT fk_chain_event_reorg_generation_job_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_generation_job_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_generation_job_state CHECK (
    state IN ('preparing','completed','invalidated') AND stage IN ('snapshot','requirements','finalize','done')
    AND ((state='preparing' AND active_slot=1 AND active_slot IS NOT NULL AND completed_at IS NULL)
      OR (state<>'preparing' AND active_slot IS NULL AND completed_at IS NOT NULL))
  ) ENFORCED,
  CONSTRAINT chk_chain_event_reorg_generation_job_checkpoint CHECK (
    requirement_count<=5 AND unmet_count+verified_count=requirement_count
    AND sender_root REGEXP '^[0-9a-f]{64}$' AND submission_root REGEXP '^[0-9a-f]{64}$'
    AND nonce_root REGEXP '^[0-9a-f]{64}$' AND requirement_root REGEXP '^[0-9a-f]{64}$'
    AND ((lease_token='' AND lease_expires_at IS NULL)
      OR (lease_token REGEXP '^[0-9a-f]{64}$' AND lease_expires_at IS NOT NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_generation_sender (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  sender_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_contract_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL,
  writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(id),
  UNIQUE KEY uk_chain_event_reorg_generation_sender_ordinal(run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_generation_sender_identity(run_id,chain_id,sender_address),
  UNIQUE KEY uk_chain_event_reorg_generation_sender_digest(run_id,row_digest),
  CONSTRAINT fk_chain_event_reorg_generation_sender_run FOREIGN KEY(run_id)
    REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_generation_sender_fields CHECK(
    ordinal>0 AND chain_id>0 AND sender_address REGEXP '^0x[0-9a-f]{40}$'
    AND source_contract_root REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0
    AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$'
    AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_generation_submission (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  source_submission_id BIGINT UNSIGNED NOT NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  action VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_attempt_no INT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  sender_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  call_data MEDIUMBLOB NOT NULL,
  call_data_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  value_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_state VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  nonce BIGINT UNSIGNED NULL,
  gas_limit BIGINT UNSIGNED NULL,
  gas_price_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  raw_tx MEDIUMBLOB NULL,
  raw_tx_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL,
  source_active_slot TINYINT UNSIGNED NULL,
  broadcast_count INT UNSIGNED NOT NULL,
  last_broadcast_at DATETIME(6) NULL,
  last_error_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  receipt_status BIGINT UNSIGNED NULL,
  receipt_block_number BIGINT UNSIGNED NULL,
  receipt_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  receipt_gas_used BIGINT UNSIGNED NULL,
  final_event_record_id BIGINT UNSIGNED NULL,
  finalized_at DATETIME(6) NULL,
  cancelled_by_admin_id BIGINT UNSIGNED NULL,
  cancel_reason_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  cancelled_at DATETIME(6) NULL,
  generation_disposition VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_snapshot_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL,
  writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_created_at DATETIME(6) NOT NULL,
  source_updated_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_generation_submission_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_generation_submission_source (run_id,source_submission_id),
  UNIQUE KEY uk_chain_event_reorg_generation_submission_digest (run_id,row_digest),
  CONSTRAINT fk_chain_event_reorg_generation_submission_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_generation_submission_source FOREIGN KEY (source_submission_id)
    REFERENCES chain_tx_submission(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_generation_submission_fields CHECK (
    ordinal>0 AND source_submission_id>0 AND project_id>0 AND source_attempt_no>0 AND chain_id>0
    AND OCTET_LENGTH(call_data) BETWEEN 1 AND 1048576 AND (raw_tx IS NULL OR OCTET_LENGTH(raw_tx)<=1048576)
    AND call_data_digest REGEXP '^[0-9a-f]{64}$' AND raw_tx_digest REGEXP '^[0-9a-f]{64}$'
    AND last_error_digest REGEXP '^[0-9a-f]{64}$' AND cancel_reason_digest REGEXP '^[0-9a-f]{64}$'
    AND source_snapshot_digest REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0
    AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$'
    AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
    AND sender_address REGEXP '^0x[0-9a-f]{40}$' AND contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND generation_disposition='unresolved'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_generation_nonce (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  sender_address VARCHAR(42) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  next_nonce BIGINT UNSIGNED NOT NULL,
  active_submission_id BIGINT UNSIGNED NULL,
  source_snapshot_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL,
  writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_created_at DATETIME(6) NOT NULL,
  source_updated_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_generation_nonce_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_generation_nonce_source (run_id,chain_id,sender_address),
  UNIQUE KEY uk_chain_event_reorg_generation_nonce_digest (run_id,row_digest),
  CONSTRAINT fk_chain_event_reorg_generation_nonce_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_generation_nonce_source FOREIGN KEY (chain_id,sender_address)
    REFERENCES chain_relayer_nonce(chain_id,sender_address) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_generation_nonce_active FOREIGN KEY (run_id,active_submission_id)
    REFERENCES chain_event_reorg_generation_submission(run_id,source_submission_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_generation_nonce_fields CHECK (
    ordinal>0 AND chain_id>0 AND CAST(sender_address AS BINARY)=CAST(LOWER(sender_address) AS BINARY)
    AND sender_address REGEXP '^0x[0-9a-f]{40}$'
    AND source_snapshot_digest REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0
    AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$'
    AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_generation_requirement (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  ordinal BIGINT UNSIGNED NOT NULL,
  requirement_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  evidence_status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  safe_detail VARCHAR(500) NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL,
  writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_generation_requirement_ordinal (run_id,ordinal),
  UNIQUE KEY uk_chain_event_reorg_generation_requirement_code (run_id,requirement_code),
  UNIQUE KEY uk_chain_event_reorg_generation_requirement_digest (run_id,row_digest),
  CONSTRAINT fk_chain_event_reorg_generation_requirement_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_generation_requirement_fields CHECK (
    ordinal BETWEEN 1 AND 5 AND evidence_status IN ('unmet','verified')
    AND requirement_code REGEXP '^[A-Z0-9_]{3,64}$' AND CHAR_LENGTH(TRIM(safe_detail)) BETWEEN 10 AND 500
    AND writer_fence_epoch>0 AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$'
    AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_generation_event (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  sequence_no INT UNSIGNED NOT NULL,
  event_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  stage VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  message VARCHAR(500) NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL,
  writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_chain_event_reorg_generation_event_sequence (run_id,sequence_no),
  UNIQUE KEY uk_chain_event_reorg_generation_event_hash (event_hash),
  CONSTRAINT fk_chain_event_reorg_generation_event_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_generation_event_fields CHECK (
    sequence_no>0 AND event_type IN ('CREATED','COMPLETED','INVALIDATED') AND stage IN ('create','finalize','invalidate')
    AND ((event_type='INVALIDATED' AND CHAR_LENGTH(error_code)>0) OR (event_type<>'INVALIDATED' AND error_code=''))
    AND CHAR_LENGTH(TRIM(message)) BETWEEN 10 AND 500
    AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$'
    AND previous_event_hash REGEXP '^[0-9a-f]{64}$' AND event_hash REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_generation_manifest (
  run_id BIGINT UNSIGNED NOT NULL,
  incident_id BIGINT UNSIGNED NOT NULL,
  descriptor_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  prerequisite_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  assessment_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  materialization_manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  outcome VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  candidate_ready TINYINT UNSIGNED NOT NULL,
  requirement_count BIGINT UNSIGNED NOT NULL,
  unmet_count BIGINT UNSIGNED NOT NULL,
  verified_count BIGINT UNSIGNED NOT NULL,
  sender_count BIGINT UNSIGNED NOT NULL,
  submission_count BIGINT UNSIGNED NOT NULL,
  submission_payload_bytes BIGINT UNSIGNED NOT NULL,
  nonce_count BIGINT UNSIGNED NOT NULL,
  candidate_project_count BIGINT UNSIGNED NOT NULL,
  candidate_ledger_count BIGINT UNSIGNED NOT NULL,
  candidate_entitlement_count BIGINT UNSIGNED NOT NULL,
  candidate_v1_fact_count BIGINT UNSIGNED NOT NULL,
  candidate_wallet_count BIGINT UNSIGNED NOT NULL,
  signed_delta_count BIGINT UNSIGNED NOT NULL,
  checkpoint_count BIGINT UNSIGNED NOT NULL,
  sender_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  submission_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  nonce_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  requirement_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  candidate_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  delta_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  checkpoint_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_count BIGINT UNSIGNED NOT NULL,
  event_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL,
  writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  manifest_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  completed_at DATETIME(6) NOT NULL,
  PRIMARY KEY (run_id),
  UNIQUE KEY uk_chain_event_reorg_generation_manifest_digest (manifest_digest),
  CONSTRAINT fk_chain_event_reorg_generation_manifest_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_chain_event_reorg_generation_manifest_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_generation_manifest_fields CHECK (
    outcome='BLOCKED' AND candidate_ready=0 AND requirement_count=5 AND unmet_count=4 AND verified_count=1
    AND candidate_project_count=0 AND candidate_ledger_count=0 AND candidate_entitlement_count=0
    AND candidate_v1_fact_count=0 AND candidate_wallet_count=0 AND signed_delta_count=0 AND checkpoint_count=0
    AND candidate_root=REPEAT('0',64) AND delta_root=REPEAT('0',64) AND checkpoint_root=REPEAT('0',64)
    AND event_count=2 AND writer_fence_epoch>0
    AND descriptor_digest REGEXP '^[0-9a-f]{64}$' AND prerequisite_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND assessment_manifest_digest REGEXP '^[0-9a-f]{64}$' AND materialization_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND sender_root REGEXP '^[0-9a-f]{64}$' AND submission_root REGEXP '^[0-9a-f]{64}$'
    AND nonce_root REGEXP '^[0-9a-f]{64}$' AND requirement_root REGEXP '^[0-9a-f]{64}$'
    AND event_root REGEXP '^[0-9a-f]{64}$' AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$'
    AND manifest_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Successor candidate interfaces. B1-B0 installs exact domain identities but
-- physically refuses all rows until a separate trusted-evidence migration is approved.
CREATE TABLE chain_event_reorg_candidate_project (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, run_id BIGINT UNSIGNED NOT NULL, ordinal BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL, contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, project_id_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  state_payload MEDIUMBLOB NOT NULL, state_payload_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL, writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), PRIMARY KEY(id),
  UNIQUE KEY uk_chain_event_reorg_candidate_project_ordinal(run_id,ordinal), UNIQUE KEY uk_chain_event_reorg_candidate_project_identity(run_id,chain_id,contract_address,contract_version,project_id_raw),
  CONSTRAINT fk_chain_event_reorg_candidate_project_run FOREIGN KEY(run_id) REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_candidate_project_fields CHECK(ordinal>0 AND project_id_raw REGEXP '^[1-9][0-9]{0,77}$' AND OCTET_LENGTH(state_payload) BETWEEN 2 AND 1048576 AND state_payload_digest REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0 AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$' AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_candidate_ledger (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, run_id BIGINT UNSIGNED NOT NULL, ordinal BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL, contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, log_index INT UNSIGNED NOT NULL,
  presence VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, fact_payload MEDIUMBLOB NOT NULL,
  fact_payload_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, source_effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL, writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), PRIMARY KEY(id),
  UNIQUE KEY uk_chain_event_reorg_candidate_ledger_ordinal(run_id,ordinal), UNIQUE KEY uk_chain_event_reorg_candidate_ledger_identity(run_id,chain_id,contract_address,tx_hash,log_index),
  CONSTRAINT fk_chain_event_reorg_candidate_ledger_run FOREIGN KEY(run_id) REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_candidate_ledger_fields CHECK(ordinal>0 AND presence IN('present','absent') AND OCTET_LENGTH(fact_payload) BETWEEN 2 AND 1048576 AND fact_payload_digest REGEXP '^[0-9a-f]{64}$' AND source_effect_digest REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0 AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$' AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_candidate_v2_entitlement (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, run_id BIGINT UNSIGNED NOT NULL, ordinal BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL, contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, log_index INT UNSIGNED NOT NULL,
  presence VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, fact_payload MEDIUMBLOB NOT NULL,
  fact_payload_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, source_effect_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL, writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), PRIMARY KEY(id),
  UNIQUE KEY uk_chain_event_reorg_candidate_entitlement_ordinal(run_id,ordinal), UNIQUE KEY uk_chain_event_reorg_candidate_entitlement_identity(run_id,chain_id,contract_address,tx_hash,log_index),
  CONSTRAINT fk_chain_event_reorg_candidate_entitlement_run FOREIGN KEY(run_id) REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_candidate_entitlement_fields CHECK(ordinal>0 AND presence IN('present','absent') AND OCTET_LENGTH(fact_payload) BETWEEN 2 AND 1048576 AND fact_payload_digest REGEXP '^[0-9a-f]{64}$' AND source_effect_digest REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0 AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$' AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_candidate_v1_fact (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, run_id BIGINT UNSIGNED NOT NULL, ordinal BIGINT UNSIGNED NOT NULL,
  fact_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, stable_identity_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  fact_payload MEDIUMBLOB NOT NULL, fact_payload_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL, writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), PRIMARY KEY(id),
  UNIQUE KEY uk_chain_event_reorg_candidate_v1_ordinal(run_id,ordinal), UNIQUE KEY uk_chain_event_reorg_candidate_v1_identity(run_id,stable_identity_digest),
  CONSTRAINT fk_chain_event_reorg_candidate_v1_run FOREIGN KEY(run_id) REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_candidate_v1_fields CHECK(ordinal>0 AND fact_kind IN('entitlement','claim','refund','lifecycle') AND stable_identity_digest REGEXP '^[0-9a-f]{64}$' AND OCTET_LENGTH(fact_payload) BETWEEN 2 AND 1048576 AND fact_payload_digest REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0 AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$' AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_candidate_wallet_aggregate (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, run_id BIGINT UNSIGNED NOT NULL, ordinal BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL, contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, wallet_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  token_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, token_decimals TINYINT UNSIGNED NOT NULL,
  aggregate_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, amount_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, writer_fence_epoch BIGINT UNSIGNED NOT NULL,
  writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), PRIMARY KEY(id),
  UNIQUE KEY uk_chain_event_reorg_candidate_wallet_ordinal(run_id,ordinal), UNIQUE KEY uk_chain_event_reorg_candidate_wallet_identity(run_id,chain_id,contract_address,project_id_raw,wallet_address,token_address,aggregate_kind),
  CONSTRAINT fk_chain_event_reorg_candidate_wallet_run FOREIGN KEY(run_id) REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_candidate_wallet_fields CHECK(ordinal>0 AND project_id_raw REGEXP '^[1-9][0-9]{0,77}$' AND token_decimals BETWEEN 1 AND 18 AND amount_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND source_root REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0 AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$' AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_candidate_delta (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, run_id BIGINT UNSIGNED NOT NULL, ordinal BIGINT UNSIGNED NOT NULL,
  domain VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, stable_identity_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  asset_role VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, token_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  token_decimals TINYINT UNSIGNED NOT NULL, boundary_raw MEDIUMTEXT CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  old_branch_delta MEDIUMTEXT CHARACTER SET ascii COLLATE ascii_bin NOT NULL, replacement_branch_delta MEDIUMTEXT CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  candidate_raw MEDIUMTEXT CHARACTER SET ascii COLLATE ascii_bin NOT NULL, switch_delta MEDIUMTEXT CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, writer_fence_epoch BIGINT UNSIGNED NOT NULL,
  writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, previous_row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), PRIMARY KEY(id),
  UNIQUE KEY uk_chain_event_reorg_candidate_delta_ordinal(run_id,ordinal), UNIQUE KEY uk_chain_event_reorg_candidate_delta_identity(run_id,domain,stable_identity_digest,asset_role,token_address),
  CONSTRAINT fk_chain_event_reorg_candidate_delta_run FOREIGN KEY(run_id) REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_candidate_delta_fields CHECK(ordinal>0 AND asset_role IN('payment','project','global') AND token_decimals BETWEEN 1 AND 18 AND boundary_raw REGEXP '^(0|-?[1-9][0-9]*)$' AND old_branch_delta REGEXP '^(0|-?[1-9][0-9]*)$' AND replacement_branch_delta REGEXP '^(0|-?[1-9][0-9]*)$' AND candidate_raw REGEXP '^(0|-?[1-9][0-9]*)$' AND switch_delta REGEXP '^(0|-?[1-9][0-9]*)$' AND source_root REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0 AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$' AND previous_row_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE chain_event_reorg_generation_checkpoint (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, run_id BIGINT UNSIGNED NOT NULL, chain_id BIGINT UNSIGNED NOT NULL,
  block_number BIGINT UNSIGNED NOT NULL, block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  registered_source_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, evidence_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, ledger_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  entitlement_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, delta_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  submission_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, nonce_root VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_checkpoint_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, checkpoint_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  writer_fence_epoch BIGINT UNSIGNED NOT NULL, writer_lease_token_digest VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), PRIMARY KEY(id), UNIQUE KEY uk_chain_event_reorg_generation_checkpoint_run(run_id),
  UNIQUE KEY uk_chain_event_reorg_generation_checkpoint_digest(checkpoint_digest),
  CONSTRAINT fk_chain_event_reorg_generation_checkpoint_run FOREIGN KEY(run_id) REFERENCES chain_event_reorg_generation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_chain_event_reorg_generation_checkpoint_fields CHECK(chain_id>0 AND block_number>0 AND block_hash REGEXP '^0x[0-9a-f]{64}$' AND registered_source_root REGEXP '^[0-9a-f]{64}$' AND evidence_root REGEXP '^[0-9a-f]{64}$' AND project_root REGEXP '^[0-9a-f]{64}$' AND ledger_root REGEXP '^[0-9a-f]{64}$' AND entitlement_root REGEXP '^[0-9a-f]{64}$' AND delta_root REGEXP '^[0-9a-f]{64}$' AND submission_root REGEXP '^[0-9a-f]{64}$' AND nonce_root REGEXP '^[0-9a-f]{64}$' AND previous_checkpoint_digest REGEXP '^[0-9a-f]{64}$' AND checkpoint_digest REGEXP '^[0-9a-f]{64}$' AND writer_fence_epoch>0 AND writer_lease_token_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DELIMITER $$
CREATE TRIGGER trg_chain_event_reorg_generation_validate_insert BEFORE INSERT ON chain_event_reorg_generation FOR EACH ROW
BEGIN
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation id must be database assigned'; END IF;
  IF NEW.attempt_no<>COALESCE((SELECT MAX(g.attempt_no)+1 FROM chain_event_reorg_generation g WHERE g.incident_id=NEW.incident_id),1) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation attempt sequence is not contiguous';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_incident i
      JOIN chain_event_reorg_prerequisite_run p ON p.incident_id=i.id
      JOIN chain_event_reorg_prerequisite_job pj ON pj.run_id=p.id AND pj.state='completed'
      JOIN chain_event_reorg_prerequisite_manifest pm ON pm.run_id=p.id AND pm.incident_id=i.id
      JOIN chain_event_reorg_assessment_run a ON a.id=p.assessment_run_id AND a.incident_id=i.id
      JOIN chain_event_reorg_assessment_manifest am ON am.run_id=a.id AND am.incident_id=i.id
      JOIN chain_event_reorg_materialization_run m ON m.id=a.materialization_run_id AND m.incident_id=i.id
      JOIN chain_event_reorg_materialization_manifest mm ON mm.run_id=m.id AND mm.incident_id=i.id
      WHERE i.id=NEW.incident_id AND i.active_slot=1 AND i.status='detected'
      AND p.id=NEW.prerequisite_run_id AND pm.manifest_digest=NEW.prerequisite_manifest_digest
      AND a.id=NEW.assessment_run_id AND am.manifest_digest=NEW.assessment_manifest_digest
      AND m.id=NEW.materialization_run_id AND mm.manifest_digest=NEW.materialization_manifest_digest
      AND pm.outcome='BLOCKED' AND pm.candidate_ready=0 AND NEW.chain_id=a.chain_id
      AND NEW.boundary_block=a.boundary_block AND NEW.boundary_block_hash=a.boundary_block_hash
      AND NEW.target_safe_block=a.target_safe_block AND NEW.target_safe_block_hash=a.target_safe_block_hash
      AND NEW.legacy_contract_address=m.legacy_contract_address AND NEW.v2_contract_address=m.contract_address
      AND NEW.old_effect_root=mm.old_effect_digest AND NEW.replacement_effect_root=mm.replacement_effect_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation descriptor source identity mismatch';
  END IF;
  IF NEW.descriptor_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-descriptor.v1',CHAR(10),
      NEW.generation_uuid,CHAR(10),NEW.incident_id,CHAR(10),NEW.attempt_no,CHAR(10),NEW.generation_kind,CHAR(10),
      NEW.prerequisite_run_id,CHAR(10),NEW.prerequisite_manifest_digest,CHAR(10),NEW.assessment_run_id,CHAR(10),
      NEW.assessment_manifest_digest,CHAR(10),NEW.materialization_run_id,CHAR(10),NEW.materialization_manifest_digest,CHAR(10),
      NEW.chain_id,CHAR(10),NEW.boundary_block,CHAR(10),NEW.boundary_block_hash,CHAR(10),NEW.target_safe_block,CHAR(10),
      NEW.target_safe_block_hash,CHAR(10),NEW.legacy_contract_address,CHAR(10),NEW.v2_contract_address,CHAR(10),
      NEW.old_effect_root,CHAR(10),NEW.replacement_effect_root,CHAR(10),NEW.algorithm_version,CHAR(10),NEW.schema_version,CHAR(10),
      DATE_FORMAT(NEW.created_at,'%Y-%m-%dT%H:%i:%s.%fZ')),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation descriptor digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_generation_no_update BEFORE UPDATE ON chain_event_reorg_generation FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation descriptors are immutable'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_no_delete BEFORE DELETE ON chain_event_reorg_generation FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation descriptors are immutable'; END$$

CREATE TRIGGER trg_chain_event_reorg_generation_job_validate_insert BEFORE INSERT ON chain_event_reorg_generation_job FOR EACH ROW
BEGIN
  IF NEW.state<>'preparing' OR NEW.stage<>'snapshot' OR NEW.active_slot IS NULL OR NEW.active_slot<>1
     OR NEW.sender_count<>0 OR NEW.submission_count<>0 OR NEW.submission_payload_bytes<>0 OR NEW.nonce_count<>0
     OR NEW.requirement_count<>0 OR NEW.unmet_count<>0 OR NEW.verified_count<>0
     OR NEW.sender_root<>REPEAT('0',64) OR NEW.submission_root<>REPEAT('0',64) OR NEW.nonce_root<>REPEAT('0',64)
     OR NEW.requirement_root<>REPEAT('0',64) OR NEW.lease_token<>'' OR NEW.lease_expires_at IS NOT NULL
     OR NEW.fence_epoch<>0 OR NEW.attempt_count<>0 OR NEW.last_error_code<>'' OR NEW.last_error_message<>''
     OR NEW.completed_at IS NOT NULL OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation g WHERE g.id=NEW.run_id AND g.incident_id=NEW.incident_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation job requires canonical initial state';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_generation_job_validate_update BEFORE UPDATE ON chain_event_reorg_generation_job FOR EACH ROW
BEGIN
  IF NEW.run_id<>OLD.run_id OR NEW.incident_id<>OLD.incident_id OR OLD.state<>'preparing'
     OR NEW.fence_epoch<OLD.fence_epoch OR NEW.attempt_count<OLD.attempt_count
     OR NEW.sender_count<OLD.sender_count OR NEW.submission_count<OLD.submission_count OR NEW.nonce_count<OLD.nonce_count
     OR NEW.requirement_count<OLD.requirement_count THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation job transition is not allowed';
  END IF;
  IF NEW.state='preparing' AND EXISTS (SELECT 1 FROM chain_event_reorg_generation_event e WHERE e.run_id=NEW.run_id AND e.event_type IN('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation terminal event forbids preparing updates';
  END IF;
  IF NEW.state='preparing' AND NEW.lease_token REGEXP '^[0-9a-f]{64}$' AND (
      NOT (OLD.lease_token='' OR OLD.lease_expires_at IS NULL OR OLD.lease_expires_at<=CURRENT_TIMESTAMP(6))
      OR NEW.lease_expires_at IS NULL OR NEW.lease_expires_at<=CURRENT_TIMESTAMP(6)
      OR NEW.lease_token=OLD.lease_token
      OR NEW.fence_epoch<>OLD.fence_epoch+1 OR NEW.attempt_count<>OLD.attempt_count+1
      OR NEW.stage<>OLD.stage OR NEW.sender_count<>OLD.sender_count OR NEW.submission_count<>OLD.submission_count
      OR NEW.submission_payload_bytes<>OLD.submission_payload_bytes OR NEW.nonce_count<>OLD.nonce_count
      OR NEW.requirement_count<>OLD.requirement_count OR NEW.unmet_count<>OLD.unmet_count OR NEW.verified_count<>OLD.verified_count
      OR NEW.sender_root<>OLD.sender_root OR NEW.submission_root<>OLD.submission_root OR NEW.nonce_root<>OLD.nonce_root
      OR NEW.requirement_root<>OLD.requirement_root OR NEW.last_error_code<>OLD.last_error_code
      OR NEW.last_error_message<>OLD.last_error_message OR NEW.completed_at IS NOT NULL) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation lease claim is not canonical';
  END IF;
  IF NEW.state='preparing' AND NEW.lease_token='' AND (
      OLD.lease_token NOT REGEXP '^[0-9a-f]{64}$' OR OLD.lease_expires_at IS NULL OR OLD.lease_expires_at<=CURRENT_TIMESTAMP(6)
      OR NEW.lease_expires_at IS NOT NULL OR NEW.fence_epoch<>OLD.fence_epoch OR NEW.attempt_count<>OLD.attempt_count
      OR NEW.completed_at IS NOT NULL OR NOT (
        (NEW.stage=OLD.stage AND NEW.sender_count=OLD.sender_count AND NEW.submission_count=OLD.submission_count
          AND NEW.submission_payload_bytes=OLD.submission_payload_bytes AND NEW.nonce_count=OLD.nonce_count
          AND NEW.requirement_count=OLD.requirement_count AND NEW.unmet_count=OLD.unmet_count
          AND NEW.verified_count=OLD.verified_count AND NEW.sender_root=OLD.sender_root
          AND NEW.submission_root=OLD.submission_root AND NEW.nonce_root=OLD.nonce_root
          AND NEW.requirement_root=OLD.requirement_root AND CHAR_LENGTH(NEW.last_error_code)>0
          AND CHAR_LENGTH(NEW.last_error_message)>0)
        OR (OLD.stage='snapshot' AND NEW.stage='requirements' AND NEW.requirement_count=0
          AND NEW.unmet_count=0 AND NEW.verified_count=0 AND NEW.requirement_root=REPEAT('0',64)
          AND NEW.sender_count=(SELECT COUNT(*) FROM chain_event_reorg_generation_sender s WHERE s.run_id=NEW.run_id)
          AND NEW.submission_count=(SELECT COUNT(*) FROM chain_event_reorg_generation_submission s WHERE s.run_id=NEW.run_id)
          AND NEW.submission_payload_bytes=(SELECT COALESCE(SUM(OCTET_LENGTH(s.call_data)+COALESCE(OCTET_LENGTH(s.raw_tx),0)),0) FROM chain_event_reorg_generation_submission s WHERE s.run_id=NEW.run_id)
          AND NEW.nonce_count=(SELECT COUNT(*) FROM chain_event_reorg_generation_nonce n WHERE n.run_id=NEW.run_id)
          AND NEW.sender_root=COALESCE((SELECT row_digest FROM chain_event_reorg_generation_sender s WHERE s.run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))
          AND NEW.submission_root=COALESCE((SELECT row_digest FROM chain_event_reorg_generation_submission s WHERE s.run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))
          AND NEW.nonce_root=COALESCE((SELECT row_digest FROM chain_event_reorg_generation_nonce n WHERE n.run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))
          AND NEW.last_error_code='' AND NEW.last_error_message='')
        OR (OLD.stage='requirements' AND NEW.stage='finalize' AND NEW.sender_count=OLD.sender_count
          AND NEW.submission_count=OLD.submission_count AND NEW.submission_payload_bytes=OLD.submission_payload_bytes
          AND NEW.nonce_count=OLD.nonce_count AND NEW.sender_root=OLD.sender_root
          AND NEW.submission_root=OLD.submission_root AND NEW.nonce_root=OLD.nonce_root
          AND NEW.requirement_count=5 AND NEW.unmet_count=4 AND NEW.verified_count=1
          AND NEW.requirement_root=COALESCE((SELECT row_digest FROM chain_event_reorg_generation_requirement r WHERE r.run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))
          AND NEW.last_error_code='' AND NEW.last_error_message='')
      )) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation leased checkpoint is not canonical';
  END IF;
  IF NEW.state='preparing' AND NEW.lease_token='' AND OLD.lease_token='' THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation preparing updates require a lease transition';
  END IF;
  IF NEW.state IN('completed','invalidated') AND (OLD.active_slot IS NULL OR OLD.active_slot<>1
     OR OLD.lease_token NOT REGEXP '^[0-9a-f]{64}$' OR OLD.lease_expires_at IS NULL OR OLD.lease_expires_at<=CURRENT_TIMESTAMP(6)
     OR NEW.active_slot IS NOT NULL OR NEW.lease_token<>'' OR NEW.lease_expires_at IS NOT NULL
     OR NEW.fence_epoch<>OLD.fence_epoch OR NEW.attempt_count<>OLD.attempt_count OR NEW.completed_at IS NULL
     OR NEW.sender_count<>OLD.sender_count OR NEW.submission_count<>OLD.submission_count
     OR NEW.submission_payload_bytes<>OLD.submission_payload_bytes OR NEW.nonce_count<>OLD.nonce_count
     OR NEW.requirement_count<>OLD.requirement_count OR NEW.unmet_count<>OLD.unmet_count
     OR NEW.verified_count<>OLD.verified_count OR NEW.sender_root<>OLD.sender_root
     OR NEW.submission_root<>OLD.submission_root OR NEW.nonce_root<>OLD.nonce_root
     OR NEW.requirement_root<>OLD.requirement_root) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation terminal transition requires active fenced lease';
  END IF;
  IF NEW.state='completed' AND (NEW.stage<>'done' OR NEW.last_error_code<>'' OR NEW.last_error_message<>''
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_event e WHERE e.run_id=NEW.run_id AND e.sequence_no=2 AND e.event_type='COMPLETED')
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_manifest m WHERE m.run_id=NEW.run_id AND m.outcome='BLOCKED' AND m.candidate_ready=0)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation completion requires terminal blocked evidence';
  END IF;
  IF NEW.state='invalidated' AND (CHAR_LENGTH(NEW.last_error_code)=0 OR CHAR_LENGTH(NEW.last_error_message)=0
     OR EXISTS (SELECT 1 FROM chain_event_reorg_generation_manifest m WHERE m.run_id=NEW.run_id)
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_event e WHERE e.run_id=NEW.run_id AND e.sequence_no=2
         AND e.event_type='INVALIDATED' AND e.error_code=NEW.last_error_code AND e.message=NEW.last_error_message)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation invalidation requires matching terminal evidence';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_generation_job_no_delete BEFORE DELETE ON chain_event_reorg_generation_job FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation jobs cannot be deleted'; END$$

CREATE TRIGGER trg_chain_event_reorg_generation_submission_validate_insert BEFORE INSERT ON chain_event_reorg_generation_submission FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64); DECLARE expected_ordinal BIGINT UNSIGNED;
  SET expected_previous=COALESCE((SELECT row_digest FROM chain_event_reorg_generation_submission WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET expected_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_generation_submission WHERE run_id=NEW.run_id),1);
  IF NEW.id<>0 OR NEW.ordinal<>expected_ordinal OR NEW.previous_row_digest<>expected_previous
     OR NEW.call_data_digest<>LOWER(SHA2(NEW.call_data,256))
     OR NEW.raw_tx_digest<>CASE WHEN NEW.raw_tx IS NULL THEN REPEAT('0',64) ELSE LOWER(SHA2(NEW.raw_tx,256)) END
     OR NEW.last_error_digest<>(SELECT LOWER(SHA2(last_error,256)) FROM chain_tx_submission WHERE id=NEW.source_submission_id)
     OR NEW.cancel_reason_digest<>(SELECT LOWER(SHA2(cancel_reason,256)) FROM chain_tx_submission WHERE id=NEW.source_submission_id)
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_job j WHERE j.run_id=NEW.run_id AND j.state='preparing'
       AND j.stage='snapshot' AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
       AND j.fence_epoch=NEW.writer_fence_epoch AND LOWER(SHA2(j.lease_token,256))=NEW.writer_lease_token_digest)
     OR NOT EXISTS (SELECT 1 FROM chain_tx_submission s WHERE s.id=NEW.source_submission_id AND s.project_id=NEW.project_id
       AND s.action=NEW.action AND s.attempt_no=NEW.source_attempt_no AND s.chain_id=NEW.chain_id
       AND LOWER(s.sender_address)=NEW.sender_address AND LOWER(s.contract_address)=NEW.contract_address
       AND s.call_data=NEW.call_data AND s.value_raw=NEW.value_raw AND s.state=NEW.source_state
       AND s.nonce<=>NEW.nonce AND s.gas_limit<=>NEW.gas_limit AND s.gas_price_raw=NEW.gas_price_raw
       AND s.raw_tx<=>NEW.raw_tx AND LOWER(COALESCE(s.tx_hash,''))=COALESCE(NEW.tx_hash,'')
       AND s.active_slot<=>NEW.source_active_slot AND s.broadcast_count=NEW.broadcast_count
       AND s.last_broadcast_at<=>NEW.last_broadcast_at
       AND s.receipt_status<=>NEW.receipt_status AND s.receipt_block_number<=>NEW.receipt_block_number
       AND LOWER(s.receipt_block_hash)=NEW.receipt_block_hash AND s.receipt_gas_used<=>NEW.receipt_gas_used
       AND s.final_event_record_id<=>NEW.final_event_record_id AND s.finalized_at<=>NEW.finalized_at
       AND s.cancelled_by_admin_id<=>NEW.cancelled_by_admin_id AND s.cancelled_at<=>NEW.cancelled_at
       AND s.created_at=NEW.source_created_at AND s.updated_at=NEW.source_updated_at) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation submission snapshot identity mismatch';
  END IF;
  IF NEW.source_snapshot_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-submission-source.v1',CHAR(10),
      NEW.source_submission_id,CHAR(10),NEW.project_id,CHAR(10),NEW.action,CHAR(10),NEW.source_attempt_no,CHAR(10),
      NEW.chain_id,CHAR(10),NEW.sender_address,CHAR(10),NEW.contract_address,CHAR(10),NEW.call_data_digest,CHAR(10),
      NEW.value_raw,CHAR(10),NEW.source_state,CHAR(10),COALESCE(NEW.nonce,'null'),CHAR(10),COALESCE(NEW.gas_limit,'null'),CHAR(10),
      NEW.gas_price_raw,CHAR(10),NEW.raw_tx_digest,CHAR(10),COALESCE(NEW.tx_hash,'null'),CHAR(10),
      COALESCE(NEW.source_active_slot,'null'),CHAR(10),NEW.broadcast_count,CHAR(10),COALESCE(DATE_FORMAT(NEW.last_broadcast_at,'%Y-%m-%dT%H:%i:%s.%fZ'),'null'),CHAR(10),
      NEW.last_error_digest,CHAR(10),COALESCE(NEW.receipt_status,'null'),CHAR(10),COALESCE(NEW.receipt_block_number,'null'),CHAR(10),
      NEW.receipt_block_hash,CHAR(10),COALESCE(NEW.receipt_gas_used,'null'),CHAR(10),COALESCE(NEW.final_event_record_id,'null'),CHAR(10),
      COALESCE(DATE_FORMAT(NEW.finalized_at,'%Y-%m-%dT%H:%i:%s.%fZ'),'null'),CHAR(10),COALESCE(NEW.cancelled_by_admin_id,'null'),CHAR(10),
      NEW.cancel_reason_digest,CHAR(10),COALESCE(DATE_FORMAT(NEW.cancelled_at,'%Y-%m-%dT%H:%i:%s.%fZ'),'null'),CHAR(10),
      DATE_FORMAT(NEW.source_created_at,'%Y-%m-%dT%H:%i:%s.%fZ'),CHAR(10),DATE_FORMAT(NEW.source_updated_at,'%Y-%m-%dT%H:%i:%s.%fZ')),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation submission source digest mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-submission.v1',CHAR(10),NEW.run_id,CHAR(10),
      NEW.ordinal,CHAR(10),NEW.source_submission_id,CHAR(10),NEW.source_snapshot_digest,CHAR(10),NEW.generation_disposition,
      CHAR(10),NEW.writer_fence_epoch,CHAR(10),NEW.writer_lease_token_digest,CHAR(10),NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation submission row digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_generation_submission_no_update BEFORE UPDATE ON chain_event_reorg_generation_submission FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation submission snapshots are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_submission_no_delete BEFORE DELETE ON chain_event_reorg_generation_submission FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation submission snapshots are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_generation_nonce_validate_insert BEFORE INSERT ON chain_event_reorg_generation_nonce FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64); DECLARE expected_ordinal BIGINT UNSIGNED;
  SET expected_previous=COALESCE((SELECT row_digest FROM chain_event_reorg_generation_nonce WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET expected_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_generation_nonce WHERE run_id=NEW.run_id),1);
  IF NEW.id<>0 OR NEW.ordinal<>expected_ordinal OR NEW.previous_row_digest<>expected_previous
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_job j WHERE j.run_id=NEW.run_id AND j.state='preparing'
       AND j.stage='snapshot' AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
       AND j.fence_epoch=NEW.writer_fence_epoch AND LOWER(SHA2(j.lease_token,256))=NEW.writer_lease_token_digest)
     OR NOT EXISTS (SELECT 1 FROM chain_relayer_nonce n WHERE n.chain_id=NEW.chain_id AND LOWER(n.sender_address)=NEW.sender_address
       AND n.next_nonce=NEW.next_nonce AND n.active_submission_id<=>NEW.active_submission_id
       AND n.created_at=NEW.source_created_at AND n.updated_at=NEW.source_updated_at) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation nonce snapshot identity mismatch';
  END IF;
  IF NEW.source_snapshot_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-nonce-source.v1',CHAR(10),
      NEW.chain_id,CHAR(10),NEW.sender_address,CHAR(10),NEW.next_nonce,CHAR(10),COALESCE(NEW.active_submission_id,'null'),CHAR(10),
      DATE_FORMAT(NEW.source_created_at,'%Y-%m-%dT%H:%i:%s.%fZ'),CHAR(10),DATE_FORMAT(NEW.source_updated_at,'%Y-%m-%dT%H:%i:%s.%fZ')),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation nonce source digest mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-nonce.v1',CHAR(10),NEW.run_id,CHAR(10),
      NEW.ordinal,CHAR(10),NEW.chain_id,CHAR(10),NEW.sender_address,CHAR(10),NEW.next_nonce,CHAR(10),
      COALESCE(NEW.active_submission_id,0),CHAR(10),NEW.source_snapshot_digest,CHAR(10),NEW.writer_fence_epoch,CHAR(10),
      NEW.writer_lease_token_digest,CHAR(10),NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation nonce row digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_generation_nonce_no_update BEFORE UPDATE ON chain_event_reorg_generation_nonce FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation nonce snapshots are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_nonce_no_delete BEFORE DELETE ON chain_event_reorg_generation_nonce FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation nonce snapshots are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_generation_requirement_validate_insert BEFORE INSERT ON chain_event_reorg_generation_requirement FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64); DECLARE expected_ordinal BIGINT UNSIGNED;
  SET expected_previous=COALESCE((SELECT row_digest FROM chain_event_reorg_generation_requirement WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET expected_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_generation_requirement WHERE run_id=NEW.run_id),1);
  IF NEW.id<>0 OR NEW.ordinal<>expected_ordinal OR NEW.previous_row_digest<>expected_previous
     OR NEW.requirement_code<>CASE NEW.ordinal WHEN 1 THEN 'BOUNDARY_STATE_SNAPSHOT_REQUIRED' WHEN 2 THEN 'V1_SOURCE_ORIGIN_UNATTESTED'
       WHEN 3 THEN 'GLOBAL_CONFIG_BASELINE_REQUIRED' WHEN 4 THEN 'OUTBOX_GENERATION_BINDING_REQUIRED'
       WHEN 5 THEN 'GENERATION_CHECKPOINT_SCHEMA_REQUIRED' END
     OR NEW.evidence_status<>CASE WHEN NEW.ordinal=5 THEN 'verified' ELSE 'unmet' END
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_job j WHERE j.run_id=NEW.run_id AND j.state='preparing'
       AND j.stage='requirements' AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
       AND j.fence_epoch=NEW.writer_fence_epoch AND LOWER(SHA2(j.lease_token,256))=NEW.writer_lease_token_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation requirement matrix or fence mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-requirement.v1',CHAR(10),NEW.run_id,CHAR(10),
      NEW.ordinal,CHAR(10),NEW.requirement_code,CHAR(10),NEW.evidence_status,CHAR(10),NEW.safe_detail,CHAR(10),
      NEW.writer_fence_epoch,CHAR(10),NEW.writer_lease_token_digest,CHAR(10),NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation requirement row digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_generation_requirement_no_update BEFORE UPDATE ON chain_event_reorg_generation_requirement FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation requirements are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_requirement_no_delete BEFORE DELETE ON chain_event_reorg_generation_requirement FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation requirements are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_generation_event_validate_insert BEFORE INSERT ON chain_event_reorg_generation_event FOR EACH ROW
BEGIN
  DECLARE prior_hash VARCHAR(64); DECLARE prior_sequence INT UNSIGNED;
  SET prior_hash=COALESCE((SELECT event_hash FROM chain_event_reorg_generation_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),REPEAT('0',64));
  SET prior_sequence=COALESCE((SELECT sequence_no FROM chain_event_reorg_generation_event WHERE run_id=NEW.run_id ORDER BY sequence_no DESC LIMIT 1),0);
  IF NEW.id<>0 OR NEW.sequence_no<>prior_sequence+1 OR NEW.previous_event_hash<>prior_hash
     OR (NEW.event_type='CREATED' AND (NEW.sequence_no<>1 OR NEW.stage<>'create' OR NEW.writer_fence_epoch<>0 OR NEW.writer_lease_token_digest<>REPEAT('0',64)))
     OR (NEW.event_type IN('COMPLETED','INVALIDATED') AND (NEW.sequence_no<>2 OR NEW.writer_fence_epoch=0))
     OR (NEW.event_type<>'CREATED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_job j WHERE j.run_id=NEW.run_id
       AND j.state='preparing' AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
       AND j.fence_epoch=NEW.writer_fence_epoch AND LOWER(SHA2(j.lease_token,256))=NEW.writer_lease_token_digest))
     OR (NEW.event_type='CREATED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_job j WHERE j.run_id=NEW.run_id AND j.fence_epoch=0 AND j.lease_token='')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation event sequence or fence mismatch';
  END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_generation_event e WHERE e.run_id=NEW.run_id AND e.event_type IN('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation terminal event is final';
  END IF;
  IF NEW.event_type='COMPLETED' AND NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_job j WHERE j.run_id=NEW.run_id
      AND j.stage='finalize' AND j.requirement_count=5 AND j.unmet_count=4 AND j.verified_count=1) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation completion event requires complete blocked checkpoint';
  END IF;
  IF NEW.event_hash<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-event.v1',CHAR(10),NEW.run_id,CHAR(10),
      NEW.sequence_no,CHAR(10),NEW.event_type,CHAR(10),NEW.stage,CHAR(10),NEW.error_code,CHAR(10),NEW.message,CHAR(10),
      NEW.writer_fence_epoch,CHAR(10),NEW.writer_lease_token_digest,CHAR(10),NEW.previous_event_hash),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation event digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_generation_event_no_update BEFORE UPDATE ON chain_event_reorg_generation_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation events are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_event_no_delete BEFORE DELETE ON chain_event_reorg_generation_event FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation events are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_generation_manifest_validate_insert BEFORE INSERT ON chain_event_reorg_generation_manifest FOR EACH ROW
BEGIN
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation g JOIN chain_event_reorg_generation_job j ON j.run_id=g.id AND j.incident_id=g.incident_id
      WHERE g.id=NEW.run_id AND g.incident_id=NEW.incident_id AND g.descriptor_digest=NEW.descriptor_digest
      AND g.prerequisite_manifest_digest=NEW.prerequisite_manifest_digest AND g.assessment_manifest_digest=NEW.assessment_manifest_digest
      AND g.materialization_manifest_digest=NEW.materialization_manifest_digest AND j.state='preparing' AND j.stage='finalize'
      AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
      AND j.fence_epoch=NEW.writer_fence_epoch AND LOWER(SHA2(j.lease_token,256))=NEW.writer_lease_token_digest
      AND j.sender_count=NEW.sender_count AND j.submission_count=NEW.submission_count
      AND j.submission_payload_bytes=NEW.submission_payload_bytes AND j.nonce_count=NEW.nonce_count
      AND j.requirement_count=NEW.requirement_count AND j.unmet_count=NEW.unmet_count AND j.verified_count=NEW.verified_count
      AND j.sender_root=NEW.sender_root AND j.submission_root=NEW.submission_root AND j.nonce_root=NEW.nonce_root
      AND j.requirement_root=NEW.requirement_root) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation manifest requires complete leased checkpoint';
  END IF;
  IF (SELECT COUNT(*) FROM chain_event_reorg_generation_submission WHERE run_id=NEW.run_id)<>NEW.submission_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_generation_sender WHERE run_id=NEW.run_id)<>NEW.sender_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_generation_nonce WHERE run_id=NEW.run_id)<>NEW.nonce_count
     OR (SELECT COUNT(*) FROM chain_event_reorg_generation_requirement WHERE run_id=NEW.run_id)<>5
     OR (SELECT COUNT(*) FROM chain_event_reorg_generation_requirement WHERE run_id=NEW.run_id AND evidence_status='unmet')<>4
     OR (SELECT COUNT(*) FROM chain_event_reorg_generation_requirement WHERE run_id=NEW.run_id AND evidence_status='verified')<>1
     OR (SELECT COUNT(*) FROM chain_event_reorg_generation_event WHERE run_id=NEW.run_id)<>2
     OR (SELECT COUNT(*) FROM chain_event_reorg_generation_event WHERE run_id=NEW.run_id AND sequence_no=2 AND event_type='COMPLETED' AND event_hash=NEW.event_root)<>1
     OR COALESCE((SELECT row_digest FROM chain_event_reorg_generation_sender WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.sender_root
     OR COALESCE((SELECT row_digest FROM chain_event_reorg_generation_submission WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.submission_root
     OR COALESCE((SELECT row_digest FROM chain_event_reorg_generation_nonce WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.nonce_root
     OR COALESCE((SELECT row_digest FROM chain_event_reorg_generation_requirement WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64))<>NEW.requirement_root
     OR EXISTS (SELECT 1 FROM chain_event_reorg_candidate_project WHERE run_id=NEW.run_id)
     OR EXISTS (SELECT 1 FROM chain_event_reorg_candidate_ledger WHERE run_id=NEW.run_id)
     OR EXISTS (SELECT 1 FROM chain_event_reorg_candidate_v2_entitlement WHERE run_id=NEW.run_id)
     OR EXISTS (SELECT 1 FROM chain_event_reorg_candidate_v1_fact WHERE run_id=NEW.run_id)
     OR EXISTS (SELECT 1 FROM chain_event_reorg_candidate_wallet_aggregate WHERE run_id=NEW.run_id)
     OR EXISTS (SELECT 1 FROM chain_event_reorg_candidate_delta WHERE run_id=NEW.run_id)
     OR EXISTS (SELECT 1 FROM chain_event_reorg_generation_checkpoint WHERE run_id=NEW.run_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation manifest evidence sets are incomplete';
  END IF;
  IF NEW.manifest_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-manifest.v1',CHAR(10),NEW.run_id,CHAR(10),
      NEW.incident_id,CHAR(10),NEW.descriptor_digest,CHAR(10),NEW.prerequisite_manifest_digest,CHAR(10),NEW.assessment_manifest_digest,
      CHAR(10),NEW.materialization_manifest_digest,CHAR(10),NEW.outcome,CHAR(10),'false',CHAR(10),NEW.requirement_count,CHAR(10),
      NEW.unmet_count,CHAR(10),NEW.verified_count,CHAR(10),NEW.sender_count,CHAR(10),NEW.submission_count,CHAR(10),
      NEW.submission_payload_bytes,CHAR(10),NEW.nonce_count,CHAR(10),NEW.candidate_project_count,CHAR(10),
      NEW.candidate_ledger_count,CHAR(10),NEW.candidate_entitlement_count,CHAR(10),NEW.candidate_v1_fact_count,CHAR(10),
      NEW.candidate_wallet_count,CHAR(10),NEW.signed_delta_count,CHAR(10),NEW.checkpoint_count,CHAR(10),
      NEW.sender_root,CHAR(10),NEW.submission_root,CHAR(10),NEW.nonce_root,CHAR(10),NEW.requirement_root,CHAR(10),
      NEW.candidate_root,CHAR(10),NEW.delta_root,CHAR(10),NEW.checkpoint_root,CHAR(10),
      NEW.event_count,CHAR(10),NEW.event_root,CHAR(10),
      NEW.writer_fence_epoch,CHAR(10),NEW.writer_lease_token_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation manifest digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_generation_manifest_no_update BEFORE UPDATE ON chain_event_reorg_generation_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation manifests are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_manifest_no_delete BEFORE DELETE ON chain_event_reorg_generation_manifest FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation manifests are append-only'; END$$

CREATE TRIGGER trg_chain_event_reorg_generation_sender_validate_insert BEFORE INSERT ON chain_event_reorg_generation_sender FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64); DECLARE expected_ordinal BIGINT UNSIGNED;
  SET expected_previous=COALESCE((SELECT row_digest FROM chain_event_reorg_generation_sender WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),REPEAT('0',64));
  SET expected_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_generation_sender WHERE run_id=NEW.run_id),1);
  IF NEW.id<>0 OR NEW.ordinal<>expected_ordinal OR NEW.previous_row_digest<>expected_previous
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation_job j WHERE j.run_id=NEW.run_id AND j.state='preparing'
       AND j.stage='snapshot' AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)
       AND j.fence_epoch=NEW.writer_fence_epoch AND LOWER(SHA2(j.lease_token,256))=NEW.writer_lease_token_digest)
     OR NOT EXISTS (SELECT 1 FROM chain_event_reorg_generation g JOIN chain_tx_submission s ON s.chain_id=g.chain_id
       AND LOWER(s.sender_address)=NEW.sender_address AND LOWER(s.contract_address) IN(g.legacy_contract_address,g.v2_contract_address)
       WHERE g.id=NEW.run_id AND g.chain_id=NEW.chain_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation sender snapshot identity or fence mismatch';
  END IF;
  IF NEW.source_contract_root<>(SELECT LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-sender-source.v1',CHAR(10),
      g.chain_id,CHAR(10),NEW.sender_address,CHAR(10),
      IF(EXISTS(SELECT 1 FROM chain_tx_submission s WHERE s.chain_id=g.chain_id AND LOWER(s.sender_address)=NEW.sender_address AND LOWER(s.contract_address)=g.legacy_contract_address),'true','false'),CHAR(10),
      IF(EXISTS(SELECT 1 FROM chain_tx_submission s WHERE s.chain_id=g.chain_id AND LOWER(s.sender_address)=NEW.sender_address AND LOWER(s.contract_address)=g.v2_contract_address),'true','false')),256))
      FROM chain_event_reorg_generation g WHERE g.id=NEW.run_id) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation sender source root mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-sender.v1',CHAR(10),NEW.run_id,CHAR(10),
      NEW.ordinal,CHAR(10),NEW.chain_id,CHAR(10),NEW.sender_address,CHAR(10),NEW.source_contract_root,CHAR(10),
      NEW.writer_fence_epoch,CHAR(10),NEW.writer_lease_token_digest,CHAR(10),NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation sender row digest mismatch';
  END IF;
END$$
CREATE TRIGGER trg_chain_event_reorg_generation_sender_no_update BEFORE UPDATE ON chain_event_reorg_generation_sender FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation sender snapshots are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_sender_no_delete BEFORE DELETE ON chain_event_reorg_generation_sender FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation sender snapshots are append-only'; END$$

-- Future interfaces are immutable and completely closed in B1-B0.
CREATE TRIGGER trg_chain_event_reorg_candidate_project_validate_insert BEFORE INSERT ON chain_event_reorg_candidate_project FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='B1-B0 forbids candidate project rows'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_project_no_update BEFORE UPDATE ON chain_event_reorg_candidate_project FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate project rows are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_project_no_delete BEFORE DELETE ON chain_event_reorg_candidate_project FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate project rows are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_ledger_validate_insert BEFORE INSERT ON chain_event_reorg_candidate_ledger FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='B1-B0 forbids candidate ledger rows'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_ledger_no_update BEFORE UPDATE ON chain_event_reorg_candidate_ledger FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate ledger rows are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_ledger_no_delete BEFORE DELETE ON chain_event_reorg_candidate_ledger FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate ledger rows are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_entitlement_validate_insert BEFORE INSERT ON chain_event_reorg_candidate_v2_entitlement FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='B1-B0 forbids candidate entitlement rows'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_entitlement_no_update BEFORE UPDATE ON chain_event_reorg_candidate_v2_entitlement FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate entitlement rows are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_entitlement_no_delete BEFORE DELETE ON chain_event_reorg_candidate_v2_entitlement FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate entitlement rows are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_v1_validate_insert BEFORE INSERT ON chain_event_reorg_candidate_v1_fact FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='B1-B0 forbids candidate V1 facts'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_v1_no_update BEFORE UPDATE ON chain_event_reorg_candidate_v1_fact FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate V1 facts are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_v1_no_delete BEFORE DELETE ON chain_event_reorg_candidate_v1_fact FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate V1 facts are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_wallet_validate_insert BEFORE INSERT ON chain_event_reorg_candidate_wallet_aggregate FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='B1-B0 forbids candidate wallet aggregates'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_wallet_no_update BEFORE UPDATE ON chain_event_reorg_candidate_wallet_aggregate FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate wallet aggregates are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_wallet_no_delete BEFORE DELETE ON chain_event_reorg_candidate_wallet_aggregate FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate wallet aggregates are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_delta_validate_insert BEFORE INSERT ON chain_event_reorg_candidate_delta FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='B1-B0 forbids candidate signed deltas'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_delta_no_update BEFORE UPDATE ON chain_event_reorg_candidate_delta FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate deltas are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_candidate_delta_no_delete BEFORE DELETE ON chain_event_reorg_candidate_delta FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='candidate deltas are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_checkpoint_validate_insert BEFORE INSERT ON chain_event_reorg_generation_checkpoint FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='B1-B0 forbids generation checkpoints'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_checkpoint_no_update BEFORE UPDATE ON chain_event_reorg_generation_checkpoint FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation checkpoints are append-only'; END$$
CREATE TRIGGER trg_chain_event_reorg_generation_checkpoint_no_delete BEFORE DELETE ON chain_event_reorg_generation_checkpoint FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation checkpoints are append-only'; END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS jc_assert_chain_event_reorg_generation;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_reorg_generation()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE' AND TABLE_NAME IN (
      'chain_event_reorg_generation','chain_event_reorg_generation_job','chain_event_reorg_generation_sender','chain_event_reorg_generation_submission',
      'chain_event_reorg_generation_nonce','chain_event_reorg_generation_requirement','chain_event_reorg_generation_event',
      'chain_event_reorg_generation_manifest','chain_event_reorg_candidate_project','chain_event_reorg_candidate_ledger',
      'chain_event_reorg_candidate_v2_entitlement','chain_event_reorg_candidate_v1_fact',
      'chain_event_reorg_candidate_wallet_aggregate','chain_event_reorg_candidate_delta','chain_event_reorg_generation_checkpoint'))<>15 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation foundation tables are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND EVENT_OBJECT_TABLE IN (
      'chain_event_reorg_generation','chain_event_reorg_generation_job','chain_event_reorg_generation_sender','chain_event_reorg_generation_submission',
      'chain_event_reorg_generation_nonce','chain_event_reorg_generation_requirement','chain_event_reorg_generation_event',
      'chain_event_reorg_generation_manifest','chain_event_reorg_candidate_project','chain_event_reorg_candidate_ledger',
      'chain_event_reorg_candidate_v2_entitlement','chain_event_reorg_candidate_v1_fact',
      'chain_event_reorg_candidate_wallet_aggregate','chain_event_reorg_candidate_delta','chain_event_reorg_generation_checkpoint'))<>45 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation foundation triggers are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES'
      AND TABLE_NAME IN ('chain_event_reorg_generation','chain_event_reorg_generation_job','chain_event_reorg_generation_sender','chain_event_reorg_generation_submission',
      'chain_event_reorg_generation_nonce','chain_event_reorg_generation_requirement','chain_event_reorg_generation_event',
      'chain_event_reorg_generation_manifest','chain_event_reorg_candidate_project','chain_event_reorg_candidate_ledger',
      'chain_event_reorg_candidate_v2_entitlement','chain_event_reorg_candidate_v1_fact',
      'chain_event_reorg_candidate_wallet_aggregate','chain_event_reorg_candidate_delta','chain_event_reorg_generation_checkpoint'))<>17 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation foundation checks are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
      AND CONSTRAINT_NAME REGEXP '^fk_chain_event_reorg_(generation|candidate)_' AND UPDATE_RULE='RESTRICT' AND DELETE_RULE='RESTRICT')<>23 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation foundation foreign keys are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM (
      SELECT INDEX_NAME,GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') columns_in_order
      FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_tx_submission'
      GROUP BY INDEX_NAME
    ) source_indexes WHERE INDEX_NAME='idx_chain_tx_generation_sender' AND columns_in_order='chain_id,sender_address,id')<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='reorg generation source range indexes are incomplete';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_reorg_generation();
DROP PROCEDURE jc_assert_chain_event_reorg_generation;

-- Deployment runner records checksum/status only after every postflight succeeds.

-- END 2026_08_28_chain_event_reorg_generation_foundation.sql

-- BEGIN 2026_08_29_chain_event_v2_only_source_policy.sql
-- JoinCare IDO V2-only registered-source policy.
-- This additive migration allows a chain environment to attest that no legacy
-- V1/JCC event source was ever registered. It does not satisfy historical V1
-- origin requirements, build a candidate, switch projections, or resolve an
-- incident.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_event_v2_only_source_policy;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_event_v2_only_source_policy()
BEGIN
  IF (SELECT COUNT(*) FROM schema_migrations WHERE status='applied'
      AND filename='2026_08_28_chain_event_reorg_generation_foundation.sql'
      AND checksum='467eebfacca807f72271b0414624c787e48fd4f0e0cebb834fcfaba2a4b04b15')<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V2-only source policy requires the frozen 08_28 foundation';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'
      AND TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_materialization_run','chain_event_reorg_generation'))<>3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V2-only source policy run tables are incomplete';
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_materialization_run','chain_event_reorg_generation')
      AND COLUMN_NAME='legacy_source_registered') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='pre-existing V2-only source policy columns require manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_event_v2_only_source_policy();
DROP PROCEDURE jc_preflight_chain_event_v2_only_source_policy;

ALTER TABLE chain_event_reorg_recovery_run
  ADD COLUMN legacy_source_registered TINYINT UNSIGNED NOT NULL DEFAULT 1 AFTER legacy_contract_address,
  DROP CHECK chk_chain_event_reorg_recovery_run_identity,
  ADD CONSTRAINT chk_chain_event_reorg_recovery_run_identity CHECK (
    incident_id>0 AND attempt_no>0 AND chain_id>0
    AND run_uuid REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
    AND contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND contract_address<>'0x0000000000000000000000000000000000000000'
    AND legacy_source_registered IN (0,1)
    AND ((legacy_source_registered=1
      AND legacy_contract_address REGEXP '^0x[0-9a-f]{40}$'
      AND legacy_contract_address<>'0x0000000000000000000000000000000000000000'
      AND legacy_contract_address<>contract_address)
      OR (legacy_source_registered=0 AND legacy_contract_address=''))
    AND contract_version='v2_stablecoin'
    AND algorithm_version='joincare-reorg-recovery-scan-v1'
  ) ENFORCED;

ALTER TABLE chain_event_reorg_materialization_run
  ADD COLUMN legacy_source_registered TINYINT UNSIGNED NOT NULL DEFAULT 1 AFTER legacy_contract_address,
  DROP CHECK chk_chain_event_reorg_materialization_run_identity,
  ADD CONSTRAINT chk_chain_event_reorg_materialization_run_identity CHECK (
    recovery_run_id>0 AND incident_id>0 AND chain_id>0
    AND run_uuid REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
    AND algorithm_version='joincare-reorg-materialization-prepare-v1'
    AND contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND contract_address<>'0x0000000000000000000000000000000000000000'
    AND legacy_source_registered IN (0,1)
    AND ((legacy_source_registered=1
      AND legacy_contract_address REGEXP '^0x[0-9a-f]{40}$'
      AND legacy_contract_address<>'0x0000000000000000000000000000000000000000'
      AND contract_address<>legacy_contract_address)
      OR (legacy_source_registered=0 AND legacy_contract_address=''))
  ) ENFORCED;

ALTER TABLE chain_event_reorg_generation
  ADD COLUMN legacy_source_registered TINYINT UNSIGNED NOT NULL DEFAULT 1 AFTER legacy_contract_address,
  DROP CHECK chk_chain_event_reorg_generation_sources,
  ADD CONSTRAINT chk_chain_event_reorg_generation_sources CHECK (
    prerequisite_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND assessment_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND materialization_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND old_effect_root REGEXP '^[0-9a-f]{64}$' AND replacement_effect_root REGEXP '^[0-9a-f]{64}$'
    AND descriptor_digest REGEXP '^[0-9a-f]{64}$'
    AND boundary_block_hash REGEXP '^0x[0-9a-f]{64}$' AND target_safe_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND legacy_source_registered IN (0,1)
    AND ((legacy_source_registered=1
      AND legacy_contract_address REGEXP '^0x[0-9a-f]{40}$'
      AND legacy_contract_address<>CONCAT('0x',REPEAT('0',40))
      AND legacy_contract_address<>v2_contract_address)
      OR (legacy_source_registered=0 AND legacy_contract_address=''))
    AND v2_contract_address REGEXP '^0x[0-9a-f]{40}$' AND v2_contract_address<>CONCAT('0x',REPEAT('0',40))
  ) ENFORCED;

DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_run_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_generation_validate_insert;
DELIMITER $$
CREATE TRIGGER trg_chain_event_reorg_materialization_run_validate_insert BEFORE INSERT ON chain_event_reorg_materialization_run FOR EACH ROW
BEGIN
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization run id must be database assigned'; END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_incident i WHERE i.id=NEW.incident_id AND i.status='detected' AND i.active_slot=1
      AND i.chain_id=NEW.chain_id AND i.cursor_block=NEW.old_cursor_block AND i.expected_block_hash=NEW.old_cursor_block_hash) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization run requires matching active incident';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_run r
      JOIN chain_event_reorg_recovery_job j ON j.run_id=r.id AND j.state='completed'
      JOIN chain_event_reorg_recovery_manifest m ON m.run_id=r.id
      WHERE r.id=NEW.recovery_run_id AND r.incident_id=NEW.incident_id AND r.chain_id=NEW.chain_id
      AND r.contract_address=NEW.contract_address AND r.legacy_contract_address=NEW.legacy_contract_address
      AND r.legacy_source_registered=NEW.legacy_source_registered
      AND r.boundary_block=NEW.boundary_block AND r.boundary_block_hash=NEW.boundary_block_hash
      AND r.target_safe_block=NEW.target_safe_block AND r.target_safe_block_hash=NEW.target_safe_block_hash
      AND m.manifest_digest=NEW.recovery_manifest_digest AND m.log_count=NEW.expected_replacement_log_count
      AND m.log_evidence_digest=NEW.replacement_log_source_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization run recovery manifest identity mismatch';
  END IF;
  IF NEW.expected_old_record_count<>(SELECT COUNT(*) FROM chain_event_record e WHERE e.block_number>NEW.boundary_block
      AND e.block_number<=NEW.old_cursor_block AND ((NEW.legacy_source_registered=1 AND e.contract_version='v1_legacy'
        AND LOWER(e.contract_address)=NEW.legacy_contract_address) OR (e.contract_version='v2_stablecoin'
        AND LOWER(e.contract_address)=NEW.contract_address))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization old source count mismatch';
  END IF;
  IF (SELECT COALESCE(SUM(OCTET_LENGTH(e.data)),0) FROM chain_event_record e WHERE e.block_number>NEW.boundary_block
      AND e.block_number<=NEW.old_cursor_block AND ((NEW.legacy_source_registered=1 AND e.contract_version='v1_legacy'
        AND LOWER(e.contract_address)=NEW.legacy_contract_address) OR (e.contract_version='v2_stablecoin'
        AND LOWER(e.contract_address)=NEW.contract_address)))>12582912
     OR (SELECT COALESCE(SUM(OCTET_LENGTH(l.raw_log)),0) FROM chain_event_reorg_recovery_log l
        WHERE l.run_id=NEW.recovery_run_id)>12582912 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization source payload limit exceeded';
  END IF;
END$$

CREATE TRIGGER trg_chain_event_reorg_generation_validate_insert BEFORE INSERT ON chain_event_reorg_generation FOR EACH ROW
BEGIN
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation id must be database assigned'; END IF;
  IF NEW.attempt_no<>COALESCE((SELECT MAX(g.attempt_no)+1 FROM chain_event_reorg_generation g WHERE g.incident_id=NEW.incident_id),1) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation attempt sequence is not contiguous';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_incident i
      JOIN chain_event_reorg_prerequisite_run p ON p.incident_id=i.id
      JOIN chain_event_reorg_prerequisite_job pj ON pj.run_id=p.id AND pj.state='completed'
      JOIN chain_event_reorg_prerequisite_manifest pm ON pm.run_id=p.id AND pm.incident_id=i.id
      JOIN chain_event_reorg_assessment_run a ON a.id=p.assessment_run_id AND a.incident_id=i.id
      JOIN chain_event_reorg_assessment_manifest am ON am.run_id=a.id AND am.incident_id=i.id
      JOIN chain_event_reorg_materialization_run m ON m.id=a.materialization_run_id AND m.incident_id=i.id
      JOIN chain_event_reorg_materialization_manifest mm ON mm.run_id=m.id AND mm.incident_id=i.id
      WHERE i.id=NEW.incident_id AND i.active_slot=1 AND i.status='detected'
      AND p.id=NEW.prerequisite_run_id AND pm.manifest_digest=NEW.prerequisite_manifest_digest
      AND a.id=NEW.assessment_run_id AND am.manifest_digest=NEW.assessment_manifest_digest
      AND m.id=NEW.materialization_run_id AND mm.manifest_digest=NEW.materialization_manifest_digest
      AND pm.outcome='BLOCKED' AND pm.candidate_ready=0 AND NEW.chain_id=a.chain_id
      AND NEW.boundary_block=a.boundary_block AND NEW.boundary_block_hash=a.boundary_block_hash
      AND NEW.target_safe_block=a.target_safe_block AND NEW.target_safe_block_hash=a.target_safe_block_hash
      AND NEW.legacy_source_registered=m.legacy_source_registered
      AND NEW.legacy_contract_address=m.legacy_contract_address AND NEW.v2_contract_address=m.contract_address
      AND NEW.old_effect_root=mm.old_effect_digest AND NEW.replacement_effect_root=mm.replacement_effect_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation descriptor source identity mismatch';
  END IF;
  IF NEW.descriptor_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-descriptor.v1',CHAR(10),
      NEW.generation_uuid,CHAR(10),NEW.incident_id,CHAR(10),NEW.attempt_no,CHAR(10),NEW.generation_kind,CHAR(10),
      NEW.prerequisite_run_id,CHAR(10),NEW.prerequisite_manifest_digest,CHAR(10),NEW.assessment_run_id,CHAR(10),
      NEW.assessment_manifest_digest,CHAR(10),NEW.materialization_run_id,CHAR(10),NEW.materialization_manifest_digest,CHAR(10),
      NEW.chain_id,CHAR(10),NEW.boundary_block,CHAR(10),NEW.boundary_block_hash,CHAR(10),NEW.target_safe_block,CHAR(10),
      NEW.target_safe_block_hash,CHAR(10),IF(NEW.legacy_source_registered=1,'true','false'),CHAR(10),NEW.legacy_contract_address,CHAR(10),NEW.v2_contract_address,CHAR(10),
      NEW.old_effect_root,CHAR(10),NEW.replacement_effect_root,CHAR(10),NEW.algorithm_version,CHAR(10),NEW.schema_version,CHAR(10),
      DATE_FORMAT(NEW.created_at,'%Y-%m-%dT%H:%i:%s.%fZ')),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation descriptor digest mismatch';
  END IF;
END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS jc_assert_chain_event_v2_only_source_policy;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_v2_only_source_policy()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_materialization_run','chain_event_reorg_generation')
      AND COLUMN_NAME='legacy_source_registered' AND COLUMN_TYPE='tinyint unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='1')<>3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V2-only source policy columns are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS tc
      JOIN information_schema.CHECK_CONSTRAINTS cc ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
      WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK' AND tc.ENFORCED='YES' AND (
      (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_run_identity' AND REPLACE(cc.CHECK_CLAUSE,' ','') LIKE '%legacy_source_registered%in(0,1)%' AND cc.CHECK_CLAUSE LIKE '%legacy_contract_address%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_materialization_run_identity' AND REPLACE(cc.CHECK_CLAUSE,' ','') LIKE '%legacy_source_registered%in(0,1)%' AND cc.CHECK_CLAUSE LIKE '%legacy_contract_address%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_generation_sources' AND REPLACE(cc.CHECK_CLAUSE,' ','') LIKE '%legacy_source_registered%in(0,1)%' AND cc.CHECK_CLAUSE LIKE '%legacy_contract_address%')))<>3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V2-only source policy checks are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND (
      (TRIGGER_NAME='trg_chain_event_reorg_materialization_run_validate_insert' AND ACTION_STATEMENT LIKE '%legacy_source_registered%')
      OR (TRIGGER_NAME='trg_chain_event_reorg_generation_validate_insert' AND ACTION_STATEMENT LIKE '%legacy_source_registered%' AND ACTION_STATEMENT LIKE '%descriptor digest mismatch%')))<>2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V2-only source policy triggers are incomplete';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_v2_only_source_policy();
DROP PROCEDURE jc_assert_chain_event_v2_only_source_policy;

-- Deployment runner records checksum/status only after every postflight succeeds.

-- END 2026_08_29_chain_event_v2_only_source_policy.sql

-- BEGIN 2026_08_30_security_control_plane_and_event_fence.sql
-- JoinCare first-batch security blockers: live session revocation, verified
-- secondary wallets, least-privilege bot RBAC, and canonical event failure fence.
-- Additive only. Existing JWTs intentionally become invalid after application rollout.

DELIMITER $$
DROP PROCEDURE IF EXISTS jc_security_0830$$
CREATE PROCEDURE jc_security_0830()
BEGIN
  IF (SELECT COUNT(*) FROM schema_migrations WHERE status='applied' AND filename='2026_08_29_chain_event_v2_only_source_policy.sql')<>1
     OR EXISTS (SELECT 1 FROM schema_migrations WHERE status='applying' AND filename<>'2026_08_30_security_control_plane_and_event_fence.sql') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='08_30 security migration prerequisites are incomplete';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='admin_users' AND COLUMN_NAME='session_version') THEN
    ALTER TABLE admin_users ADD COLUMN session_version BIGINT UNSIGNED NOT NULL DEFAULT 1 AFTER is_enable;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='admin_users' AND COLUMN_NAME='password_changed_at') THEN
    ALTER TABLE admin_users ADD COLUMN password_changed_at DATETIME NULL AFTER password_hash;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='admin_users' AND COLUMN_NAME='locked_until') THEN
    ALTER TABLE admin_users ADD COLUMN locked_until DATETIME NULL AFTER password_changed_at;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='admin_users' AND COLUMN_NAME='failed_login_count') THEN
    ALTER TABLE admin_users ADD COLUMN failed_login_count INT NOT NULL DEFAULT 0 AFTER locked_until;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='admin_users' AND COLUMN_NAME='deleted_at') THEN
    ALTER TABLE admin_users ADD COLUMN deleted_at DATETIME NULL AFTER updated_at, ADD KEY idx_deleted_at(deleted_at);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user' AND COLUMN_NAME='is_enable') THEN
    ALTER TABLE `user` ADD COLUMN is_enable TINYINT NOT NULL DEFAULT 1 AFTER last_login_ip;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user' AND COLUMN_NAME='session_version') THEN
    ALTER TABLE `user` ADD COLUMN session_version BIGINT UNSIGNED NOT NULL DEFAULT 1 AFTER is_enable;
  END IF;
  CREATE TABLE IF NOT EXISTS user_wallet (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    uid INT NOT NULL,
    wallet_address VARCHAR(64) NOT NULL,
    chain VARCHAR(16) NOT NULL DEFAULT 'evm',
    is_primary TINYINT NOT NULL DEFAULT 0,
    bound_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY(id),
    UNIQUE KEY uk_uid_wallet_chain(uid,wallet_address,chain),
    KEY idx_wallet(wallet_address),
    KEY idx_uid_primary(uid,is_primary)
  ) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='verified user wallet bindings';
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet' AND COLUMN_NAME='verified_at') THEN
    ALTER TABLE user_wallet ADD COLUMN verified_at DATETIME(6) NULL AFTER is_primary;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet' AND COLUMN_NAME='verification_method') THEN
    ALTER TABLE user_wallet ADD COLUMN verification_method VARCHAR(32) NOT NULL DEFAULT '' AFTER verified_at;
  END IF;

  CREATE TABLE IF NOT EXISTS admin_user_roles (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    admin_id BIGINT UNSIGNED NOT NULL,
    role_code VARCHAR(32) NOT NULL,
    granted_by BIGINT UNSIGNED NULL,
    granted_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY(id),
    UNIQUE KEY uk_admin_role(admin_id,role_code),
    KEY idx_role(role_code)
  ) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='admin user role membership';
  INSERT IGNORE INTO admin_user_roles(admin_id,role_code,granted_at)
    SELECT id,role_code,created_at FROM admin_users WHERE role_code<>'';

  CREATE TABLE IF NOT EXISTS admin_approval (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    operation VARCHAR(64) NOT NULL,
    target_kind VARCHAR(32) NOT NULL DEFAULT '',
    target_id VARCHAR(64) NOT NULL DEFAULT '',
    payload MEDIUMTEXT NOT NULL,
    reason VARCHAR(500) NOT NULL DEFAULT '',
    initiator_id BIGINT UNSIGNED NOT NULL,
    initiator_name VARCHAR(64) NOT NULL DEFAULT '',
    approver_id BIGINT UNSIGNED NULL,
    approver_name VARCHAR(64) NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    reject_reason VARCHAR(500) NOT NULL DEFAULT '',
    result TEXT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    reviewed_at DATETIME NULL,
    executed_at DATETIME NULL,
    expires_at DATETIME NULL,
    PRIMARY KEY(id),
    KEY idx_status_created(status,created_at),
    KEY idx_initiator(initiator_id),
    KEY idx_target(target_kind,target_id)
  ) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='four-eye approval work orders';

  CREATE TABLE IF NOT EXISTS admin_audit_log (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    admin_id BIGINT UNSIGNED NOT NULL,
    username VARCHAR(64) NOT NULL DEFAULT '',
    role_codes VARCHAR(255) NOT NULL DEFAULT '',
    ip VARCHAR(45) NOT NULL DEFAULT '',
    user_agent VARCHAR(500) NOT NULL DEFAULT '',
    method VARCHAR(10) NOT NULL,
    path VARCHAR(255) NOT NULL,
    route VARCHAR(255) NOT NULL DEFAULT '',
    target_id VARCHAR(64) NOT NULL DEFAULT '',
    req_body MEDIUMTEXT NULL,
    resp_status INT NOT NULL DEFAULT 0,
    resp_hash CHAR(64) NOT NULL DEFAULT '',
    prev_hash CHAR(64) NOT NULL DEFAULT '',
    hash CHAR(64) NOT NULL,
    duration_ms INT NOT NULL DEFAULT 0,
    note VARCHAR(500) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY(id),
    KEY idx_admin_created(admin_id,created_at),
    KEY idx_target(target_id,created_at),
    KEY idx_created(created_at)
  ) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='append-only admin operation audit';

  UPDATE admin_approval
  SET status='expired', reviewed_at=COALESCE(reviewed_at,CURRENT_TIMESTAMP)
  WHERE status='pending';
  UPDATE admin_approval
  SET payload='{"redacted":"legacy admin credential payload removed by 08_30"}'
  WHERE operation='admin.create';

  -- Only an existing row equal to the registration wallet is trusted without a
  -- new signature. All other historical Phase-1 rows remain visibly unverified.
  UPDATE user_wallet w
  JOIN `user` u ON u.id=w.uid AND LOWER(u.wallet_address)=LOWER(w.wallet_address)
  SET w.verified_at=COALESCE(w.verified_at,w.bound_at),
      w.verification_method=IF(w.verification_method='', 'primary_registration', w.verification_method),
      w.is_primary=1
  WHERE w.chain='evm' AND w.wallet_address REGEXP '^0x[0-9A-Fa-f]{40}$';
  UPDATE user_wallet w
  LEFT JOIN `user` u ON u.id=w.uid AND LOWER(u.wallet_address)=LOWER(w.wallet_address)
  SET w.is_primary=0
  WHERE w.is_primary<>0 AND (w.verified_at IS NULL OR u.id IS NULL);

  IF EXISTS (
    SELECT 1 FROM user_wallet
    WHERE verified_at IS NOT NULL AND chain='evm'
    GROUP BY LOWER(wallet_address) HAVING COUNT(DISTINCT uid)>1 LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='verified EVM wallet belongs to multiple users; reconcile before migration';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet' AND COLUMN_NAME='verified_evm_identity') THEN
    ALTER TABLE user_wallet ADD COLUMN verified_evm_identity VARCHAR(42)
      CHARACTER SET ascii COLLATE ascii_bin
      GENERATED ALWAYS AS (CASE WHEN verified_at IS NOT NULL AND chain='evm' THEN LOWER(wallet_address) ELSE NULL END) STORED;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet' AND INDEX_NAME='uk_user_wallet_verified_evm') THEN
    ALTER TABLE user_wallet ADD UNIQUE KEY uk_user_wallet_verified_evm (verified_evm_identity);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet' AND CONSTRAINT_NAME='chk_user_wallet_verification') THEN
    ALTER TABLE user_wallet ADD CONSTRAINT chk_user_wallet_verification CHECK ((
      (verified_at IS NULL AND verification_method='') OR
      (verified_at IS NOT NULL AND verification_method IN ('primary_registration','personal_sign') AND chain='evm'
       AND wallet_address REGEXP '^0x[0-9A-Fa-f]{40}$'
       AND LOWER(wallet_address)<>'0x0000000000000000000000000000000000000000')
    ) AND (is_primary=0 OR verified_at IS NOT NULL)) ENFORCED;
  END IF;

  CREATE TABLE IF NOT EXISTS user_wallet_identity (
    wallet_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    uid INT NOT NULL,
    chain VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    claim_method VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    claimed_at DATETIME(6) NOT NULL,
    PRIMARY KEY(wallet_address),
    KEY idx_user_wallet_identity_uid(uid),
    CONSTRAINT chk_user_wallet_identity CHECK (
      wallet_address REGEXP '^0x[0-9a-f]{40}$' AND
      wallet_address<>'0x0000000000000000000000000000000000000000' AND
      uid>0 AND chain='evm' AND claim_method IN ('primary_registration','personal_sign')
    ) ENFORCED
  ) ENGINE=InnoDB COMMENT='non-recyclable verified wallet ownership registry';

  IF EXISTS (
    SELECT 1 FROM `user` u JOIN user_wallet_identity i ON i.wallet_address=LOWER(u.wallet_address)
    WHERE i.uid<>u.id LIMIT 1
  ) OR EXISTS (
    SELECT 1 FROM user_wallet w JOIN user_wallet_identity i ON i.wallet_address=LOWER(w.wallet_address)
    WHERE w.verified_at IS NOT NULL AND w.chain='evm' AND i.uid<>w.uid LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='existing wallet identity ownership conflicts with authoritative users';
  END IF;
  INSERT IGNORE INTO user_wallet_identity(wallet_address,uid,chain,claim_method,claimed_at)
    SELECT LOWER(wallet_address),id,'evm','primary_registration',created_at FROM `user`
    WHERE wallet_address REGEXP '^0x[0-9A-Fa-f]{40}$' AND LOWER(wallet_address)<>'0x0000000000000000000000000000000000000000';
  INSERT IGNORE INTO user_wallet_identity(wallet_address,uid,chain,claim_method,claimed_at)
    SELECT LOWER(wallet_address),uid,'evm',verification_method,verified_at FROM user_wallet
    WHERE verified_at IS NOT NULL AND chain='evm';
  IF EXISTS (
    SELECT 1 FROM `user` u LEFT JOIN user_wallet_identity i ON i.wallet_address=LOWER(u.wallet_address) AND i.uid=u.id
    WHERE u.wallet_address REGEXP '^0x[0-9A-Fa-f]{40}$' AND i.wallet_address IS NULL LIMIT 1
  ) OR EXISTS (
    SELECT 1 FROM user_wallet w LEFT JOIN user_wallet_identity i ON i.wallet_address=LOWER(w.wallet_address) AND i.uid=w.uid
    WHERE w.verified_at IS NOT NULL AND w.chain='evm' AND i.wallet_address IS NULL LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='wallet identity backfill is incomplete';
  END IF;

  INSERT IGNORE INTO admin_roles(code,name,description,is_built_in,sort)
  VALUES('tgbot_service','TG Bot Service','Telegram bot least-privilege service identity',1,70);
  DELETE FROM admin_role_permissions WHERE role_code='tgbot_service';
  INSERT IGNORE INTO admin_role_permissions(role_code,module,action,scope) VALUES
    ('tgbot_service','service','read','all'),
    ('tgbot_service','service','execute','all'),
    ('super_admin','approval','read','all'),
    ('operator','approval','read','all'),
    ('approver','approval','read','all'),
    ('compliance','approval','read','all'),
    ('support','approval','read','all'),
    ('auditor','approval','read','all'),
    ('approver','m7_user','approve','all'),
    ('approver','rbac','approve','all'),
    ('approver','news','approve','all'),
    ('super_admin','rbac','approve','all'),
    ('super_admin','news','approve','all');

  -- Existing bot login is narrowed automatically. Incrementing session_version
  -- ensures any former super-admin token cannot survive the role transition.
  UPDATE admin_users a
  SET a.role_code='tgbot_service', a.session_version=a.session_version+1
  WHERE a.username='tgbot_service' AND (
    a.role_code<>'tgbot_service' OR EXISTS (
      SELECT 1 FROM admin_user_roles r WHERE r.admin_id=a.id AND r.role_code<>'tgbot_service'
    )
  );
  DELETE r FROM admin_user_roles r JOIN admin_users a ON a.id=r.admin_id WHERE a.username='tgbot_service';
  INSERT IGNORE INTO admin_user_roles(admin_id,role_code,granted_at)
  SELECT id,'tgbot_service',CURRENT_TIMESTAMP FROM admin_users WHERE username='tgbot_service';
END$$
CALL jc_security_0830()$$
DROP PROCEDURE jc_security_0830$$

DROP TRIGGER IF EXISTS trg_user_wallet_verified_owner_insert$$
CREATE TRIGGER trg_user_wallet_verified_owner_insert
BEFORE INSERT ON user_wallet FOR EACH ROW
BEGIN
  IF NEW.verified_at IS NOT NULL AND NEW.chain='evm' AND NOT EXISTS (
    SELECT 1 FROM user_wallet_identity i WHERE i.wallet_address=LOWER(NEW.wallet_address) AND i.uid=NEW.uid AND i.chain='evm'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='verified EVM wallet lacks an immutable owner claim';
  END IF;
END$$

DROP TRIGGER IF EXISTS trg_user_wallet_verified_owner_update$$
CREATE TRIGGER trg_user_wallet_verified_owner_update
BEFORE UPDATE ON user_wallet FOR EACH ROW
BEGIN
  IF NEW.verified_at IS NOT NULL AND NEW.chain='evm' AND NOT EXISTS (
    SELECT 1 FROM user_wallet_identity i WHERE i.wallet_address=LOWER(NEW.wallet_address) AND i.uid=NEW.uid AND i.chain='evm'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='verified EVM wallet lacks an immutable owner claim';
  END IF;
END$$

DROP TRIGGER IF EXISTS trg_user_primary_identity_claim_insert$$
CREATE TRIGGER trg_user_primary_identity_claim_insert
AFTER INSERT ON `user` FOR EACH ROW
BEGIN
  IF NEW.wallet_address REGEXP '^0x[0-9A-Fa-f]{40}$' THEN
    INSERT INTO user_wallet_identity(wallet_address,uid,chain,claim_method,claimed_at)
    VALUES(LOWER(NEW.wallet_address),NEW.id,'evm','primary_registration',CURRENT_TIMESTAMP(6));
  END IF;
END$$

DROP TRIGGER IF EXISTS trg_user_primary_identity_require_update$$
CREATE TRIGGER trg_user_primary_identity_require_update
BEFORE UPDATE ON `user` FOR EACH ROW
BEGIN
  IF LOWER(NEW.wallet_address)<>LOWER(OLD.wallet_address) AND NOT EXISTS (
    SELECT 1 FROM user_wallet_identity i WHERE i.wallet_address=LOWER(NEW.wallet_address) AND i.uid=NEW.id AND i.chain='evm'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='primary wallet lacks an immutable owner claim';
  END IF;
END$$

DROP TRIGGER IF EXISTS trg_user_wallet_identity_no_update$$
CREATE TRIGGER trg_user_wallet_identity_no_update
BEFORE UPDATE ON user_wallet_identity FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='verified wallet identities are non-recyclable';
END$$

DROP TRIGGER IF EXISTS trg_user_wallet_identity_no_delete$$
CREATE TRIGGER trg_user_wallet_identity_no_delete
BEFORE DELETE ON user_wallet_identity FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='verified wallet identities are non-recyclable';
END$$

DROP TRIGGER IF EXISTS trg_chain_event_failure_canonical_insert$$
CREATE TRIGGER trg_chain_event_failure_canonical_insert
BEFORE INSERT ON chain_event_failure FOR EACH ROW
BEGIN
  IF NEW.status<>'resolved' AND EXISTS (
    SELECT 1 FROM chain_event_record r
    WHERE r.block_number=NEW.block_number AND r.tx_hash=NEW.tx_hash AND r.log_index=NEW.log_index
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='canonical chain event cannot have an unresolved failure';
  END IF;
END$$

DROP TRIGGER IF EXISTS trg_chain_event_failure_canonical_update$$
CREATE TRIGGER trg_chain_event_failure_canonical_update
BEFORE UPDATE ON chain_event_failure FOR EACH ROW
BEGIN
  IF NEW.status<>'resolved' AND EXISTS (
    SELECT 1 FROM chain_event_record r
    WHERE r.block_number=NEW.block_number AND r.tx_hash=NEW.tx_hash AND r.log_index=NEW.log_index
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='canonical chain event cannot reopen an unresolved failure';
  END IF;
END$$

DROP PROCEDURE IF EXISTS jc_security_0830_postflight$$
CREATE PROCEDURE jc_security_0830_postflight()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND (
      (TABLE_NAME='admin_users' AND COLUMN_NAME='session_version' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='1') OR
      (TABLE_NAME='admin_users' AND COLUMN_NAME='password_changed_at' AND COLUMN_TYPE='datetime' AND IS_NULLABLE='YES') OR
      (TABLE_NAME='admin_users' AND COLUMN_NAME='locked_until' AND COLUMN_TYPE='datetime' AND IS_NULLABLE='YES') OR
      (TABLE_NAME='admin_users' AND COLUMN_NAME='failed_login_count' AND COLUMN_TYPE='int' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='0') OR
      (TABLE_NAME='admin_users' AND COLUMN_NAME='deleted_at' AND COLUMN_TYPE='datetime' AND IS_NULLABLE='YES') OR
      (TABLE_NAME='user' AND COLUMN_NAME='is_enable' AND COLUMN_TYPE IN ('tinyint','tinyint(1)') AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='1') OR
      (TABLE_NAME='user' AND COLUMN_NAME='session_version' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='1') OR
      (TABLE_NAME='user_wallet' AND COLUMN_NAME='verified_at' AND COLUMN_TYPE='datetime(6)' AND IS_NULLABLE='YES') OR
      (TABLE_NAME='user_wallet' AND COLUMN_NAME='verification_method' AND COLUMN_TYPE='varchar(32)' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='') OR
      (TABLE_NAME='user_wallet' AND COLUMN_NAME='verified_evm_identity' AND COLUMN_TYPE='varchar(42)' AND EXTRA LIKE '%STORED GENERATED%')
    ))<>10 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='08_30 security columns are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet' AND INDEX_NAME='uk_user_wallet_verified_evm' AND NON_UNIQUE=0)<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='verified EVM wallet uniqueness is unavailable';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet_identity')<>5
     OR (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet_identity' AND CONSTRAINT_NAME='chk_user_wallet_identity' AND ENFORCED='YES')<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='non-recyclable wallet identity registry is unavailable';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet' AND CONSTRAINT_NAME='chk_user_wallet_verification' AND ENFORCED='YES')<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='verified wallet CHECK is unavailable';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
      AND TRIGGER_NAME IN ('trg_chain_event_failure_canonical_insert','trg_chain_event_failure_canonical_update')
      AND ACTION_STATEMENT LIKE '%canonical chain event cannot%')<>2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='canonical event failure triggers are unavailable';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
      AND TRIGGER_NAME IN ('trg_user_wallet_identity_no_update','trg_user_wallet_identity_no_delete')
      AND ACTION_STATEMENT LIKE '%non-recyclable%')<>2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='wallet identity append-only guards are unavailable';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
      AND TRIGGER_NAME IN ('trg_user_wallet_verified_owner_insert','trg_user_wallet_verified_owner_update','trg_user_primary_identity_claim_insert','trg_user_primary_identity_require_update')
      AND ACTION_STATEMENT LIKE '%owner claim%')<>3
     OR (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME='trg_user_primary_identity_claim_insert' AND ACTION_STATEMENT LIKE '%INSERT INTO user_wallet_identity%')<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='global verified wallet ownership triggers are unavailable';
  END IF;
  IF (SELECT COUNT(*) FROM admin_role_permissions WHERE role_code='tgbot_service')<>2
     OR (SELECT COUNT(*) FROM admin_role_permissions WHERE role_code='tgbot_service' AND module='service' AND action IN ('read','execute') AND scope='all')<>2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='TG bot least-privilege permissions are unavailable';
  END IF;
END$$
CALL jc_security_0830_postflight()$$
DROP PROCEDURE jc_security_0830_postflight$$
DELIMITER ;

-- END 2026_08_30_security_control_plane_and_event_fence.sql

-- BEGIN 2026_08_31_restore_archived_project.sql
-- 2026_08_31_restore_archived_project.sql
-- Admin project restore: POST /admin/v1/projects/:id/restore
--
-- POST /admin/v1/projects/:id/disable archives a project by setting
-- status='archived'. Before this migration the pre-archive lifecycle status
-- was lost, so a later restore could only guess a target (e.g. pre_launch),
-- which the restore endpoint must never do unconditionally.
--
-- This column records the pre-archive status at disable time so restore can
-- revive the project to its exact prior lifecycle state. It is intentionally
-- NOT a boolean "was_pre_launch" flag because drafts can also be
-- Setup / Set / warmup and the recorded value is the restore target.
ALTER TABLE `projects`
  ADD COLUMN `pre_archive_status` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci
    NOT NULL DEFAULT '' COMMENT 'status before the project was archived (restore target)' AFTER `status`;

-- END 2026_08_31_restore_archived_project.sql

-- BEGIN 2026_09_01_platform_fee_withdraw_action.sql
-- 平台费用提取 durable action：扩展 chain_tx_submission.action 白名单（若约束存在）
-- 幂等：约束不存在的库跳过（如 joincare_local_test 从未创建过该约束）
SET @s = (
  SELECT IF(
    EXISTS(
      SELECT 1 FROM information_schema.TABLE_CONSTRAINTS
      WHERE CONSTRAINT_SCHEMA = DATABASE()
        AND TABLE_NAME = 'chain_tx_submission'
        AND CONSTRAINT_NAME = 'chk_chain_tx_action'
        AND CONSTRAINT_TYPE = 'CHECK'
    ),
    'ALTER TABLE chain_tx_submission DROP CHECK chk_chain_tx_action',
    'SELECT 1'
  )
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

ALTER TABLE chain_tx_submission
  ADD CONSTRAINT chk_chain_tx_action CHECK (
    `action` IN ('v2_create','v2_activate','project_end','project_cancel','platform_fee_withdraw')
  );

-- END 2026_09_01_platform_fee_withdraw_action.sql

-- BEGIN 2026_09_02_global_pause_actions.sql
-- 全局暂停/恢复 durable action（安全审计 HIGH-1）：
-- 1) chk_chain_tx_project 放开 project_id=0，但仅限 global_pause / global_unpause 白名单 action
-- 2) chk_chain_tx_action 白名单加入 global_pause / global_unpause
-- 幂等：约束不存在的库（如 GORM AutoMigrate 建的本地测试库）跳过对应步骤
SET NAMES utf8mb4;

-- 1) 放开 project_id=0（组合校验替代原 project_id > 0）
SET @drop_project = (
  SELECT IF(
    EXISTS(
      SELECT 1 FROM information_schema.TABLE_CONSTRAINTS
      WHERE CONSTRAINT_SCHEMA = DATABASE()
        AND TABLE_NAME = 'chain_tx_submission'
        AND CONSTRAINT_NAME = 'chk_chain_tx_project'
        AND CONSTRAINT_TYPE = 'CHECK'
    ),
    'ALTER TABLE chain_tx_submission DROP CHECK chk_chain_tx_project',
    'SELECT 1'
  )
);
PREPARE st FROM @drop_project; EXECUTE st; DEALLOCATE PREPARE st;

SET @add_project = (
  SELECT IF(
    NOT EXISTS(
      SELECT 1 FROM information_schema.TABLE_CONSTRAINTS
      WHERE CONSTRAINT_SCHEMA = DATABASE()
        AND TABLE_NAME = 'chain_tx_submission'
        AND CONSTRAINT_NAME = 'chk_chain_tx_project'
        AND CONSTRAINT_TYPE = 'CHECK'
    ),
    'ALTER TABLE chain_tx_submission ADD CONSTRAINT chk_chain_tx_project CHECK (
       (`project_id` > 0)
       OR (`project_id` = 0 AND `action` IN (''global_pause'',''global_unpause''))
     ) ENFORCED',
    'SELECT 1'
  )
);
PREPARE st FROM @add_project; EXECUTE st; DEALLOCATE PREPARE st;

-- 2) action 白名单扩展
SET @drop_action = (
  SELECT IF(
    EXISTS(
      SELECT 1 FROM information_schema.TABLE_CONSTRAINTS
      WHERE CONSTRAINT_SCHEMA = DATABASE()
        AND TABLE_NAME = 'chain_tx_submission'
        AND CONSTRAINT_NAME = 'chk_chain_tx_action'
        AND CONSTRAINT_TYPE = 'CHECK'
    ),
    'ALTER TABLE chain_tx_submission DROP CHECK chk_chain_tx_action',
    'SELECT 1'
  )
);
PREPARE st FROM @drop_action; EXECUTE st; DEALLOCATE PREPARE st;

SET @add_action = (
  SELECT IF(
    NOT EXISTS(
      SELECT 1 FROM information_schema.TABLE_CONSTRAINTS
      WHERE CONSTRAINT_SCHEMA = DATABASE()
        AND TABLE_NAME = 'chain_tx_submission'
        AND CONSTRAINT_NAME = 'chk_chain_tx_action'
        AND CONSTRAINT_TYPE = 'CHECK'
    ),
    'ALTER TABLE chain_tx_submission ADD CONSTRAINT chk_chain_tx_action CHECK (
       `action` IN (''v2_create'',''v2_activate'',''project_end'',''project_cancel'',''platform_fee_withdraw'',''global_pause'',''global_unpause'')
     ) ENFORCED',
    'SELECT 1'
  )
);
PREPARE st FROM @add_action; EXECUTE st; DEALLOCATE PREPARE st;

-- END 2026_09_02_global_pause_actions.sql
