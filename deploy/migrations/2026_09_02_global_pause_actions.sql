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
