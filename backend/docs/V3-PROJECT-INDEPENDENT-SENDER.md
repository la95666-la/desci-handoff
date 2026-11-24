# V3 项目操作与历史项目状态解耦

## 修复范围

其他项目的 `V3ReorgHalted` / archived / `v3_reorg_suspended` 财务恢复状态，不再单独决定当前项目能否创建、激活、结算或提取平台费。原交易和历史证据原样保留，不伪造 applied、不删除记录、不修改旧 nonce。

## 保留的实际安全约束

- 发送者共享 nonce 仍全局串行：active_submission_id 非空、nonce 漂移、pending 与 latest 不一致，均拒绝。
- suspended 记录如 nonce 未知或尚未被确认消耗，仍阻断；这些是发送者真实签名风险，不是项目状态限制。
- 签名前，两个独立 RPC 在共同的 15-confirmed 区块/hash 核对 consumed nonce，并核对 latest/pending；旧签名 nonce 必须低于 confirmed consumed nonce。
- 原始首次广播继续检查两个 RPC 的 nonce、交易/hash 不存在、精确 calldata/value/gas 模拟和余额，持久化 broadcast-unknown 后最多发送一次，不自动重发。
- 本项目 running cursor、原创建交易、审批、权限、生命周期和结算模拟保持不变。共享真实链级 reorg incident 仍 fail closed。

## 必需配套迁移

`pkg/common/chaintx/v3_sender_isolation.sql` 是独立候选迁移，不能混入普通应用发布自动执行。

它仅替换两条 chain_relayer_nonce 触发器，检查新 active slot 指向准确签名记录、next_nonce 正确递增；未知/同或更高 nonce 的 suspended 记录和其他 active signed 记录继续阻断。旧 nonce 的记录不再依赖 retired evidence TTL 或其他项目 status。

安装需 `SET @JC_V3_SENDER_ISOLATION_APPROVED=1`；应用预检检查触发器包含 `V3 nonce isolation v1` 标记。未安装时拒绝放行，不假装修复生效。

生产迁移要先验证 DB UUID/权限、备份两条原触发器的 SHOW CREATE TRIGGER 和相关 nonce/outbox 行、确认无签名/广播并发并暂停写入口。MySQL DDL 非事务性：替换期间不得让旧版本写入；验收两条触发器存在及定义一致后才启用兼容 Backend。失败以备份的原始触发器恢复，不手工释放槽位。此补丁没有执行任何生产 DDL、广播、结算或状态更新。

## 验收

1. 46 条旧签名 nonce 已确认消耗、本项目正常且无 active slot：允许进入本项目预检，不要求其他项目先恢复或退役。
2. 任一历史签名 nonce 未知/未消耗：拒绝，不删除记录。
3. 两 RPC nonce/hash 不一致、真正 active slot、pending/DB 漂移、项目 halted：拒绝。
4. 结算仍由受控 Admin 入口创建新 intent，原始 hash/nonce 对账；本修复本身不执行结算。
