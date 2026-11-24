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
