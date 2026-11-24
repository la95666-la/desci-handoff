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
