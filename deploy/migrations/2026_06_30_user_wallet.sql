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
