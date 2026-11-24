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
