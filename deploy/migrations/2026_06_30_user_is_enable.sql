-- user: 增加 is_enable（启停标志）
ALTER TABLE `user`
  ADD COLUMN `is_enable` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '1 启用 / 0 停用'
    AFTER `last_login_ip`,
  ADD INDEX `idx_is_enable` (`is_enable`) USING BTREE;
