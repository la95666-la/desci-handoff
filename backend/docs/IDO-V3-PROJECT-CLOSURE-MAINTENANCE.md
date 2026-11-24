# V3 项目结清与低频维护

本变更只新增平台元数据，不增加合约交易，不改变 `projects.status`、历史账本、钱包权益或 outbox。

## 三个独立维度

- 链上结果仍为 Pending / Active / Successful / Failed / Canceled。
- 平台 `finished` 仅表示全部支付币和项目币义务经确认区块核验完成。
- `mode` 为 normal / maintenance / archived。归档不删除任何历史，也不自动改变公开列表可见性。

## Finished

只在项目追上公共覆盖边界时评估。先用 canonical projection 做金额候选判断；候选闭合后才进行完整 canonical replay、逐钱包义务、固定块 getProject / runtime / policy、退出 flags 和独立区块锚点核验。金额全部用 raw 整数，不用浮点数或共享合约余额推断。

- Successful：项目方实收 = raised − floor(raised × 项目创建快照费率 / 10000)；平台费全额退出；用户 claimed = sold；claimed + 项目方退回项目币 = deposited。
- Failed / Canceled：全部用户 refunded = raised；项目方退回项目币 = deposited。
- 退出 flags 与 canonical 提现事件都必须存在，零金额也不以“数值相等”替代已执行证明。
- 未决交易/active nonce slot、active reorg incident 不允许 Finished。
- 证据含 epoch、区块/hash、cursor revision、SHA256 摘要、finished_at。源恢复、cursor 不匹配或新 wake 时不得展示为可归档。

## 调度

手动低频维护仅允许已结束、已完成当前同步且无未决交易的项目。不是暂停合约或禁止用户操作。

- 未到期项目从候选集合中实际过滤，不因空闲容量被反复扫描。
- 成功追上覆盖边界后，使用数据库 UTC 计算下一次时间：完成时间 + 12 小时。
- 尚未追上或失败的任务不推迟 12 小时；原有预算和有限退避继续工作，绝不跳 cursor。
- 公共 producer 持续运行。项目事件与 coverage 同事务持久化 wake_block；控制事件唤醒该 scope 的非 normal 项目。重启不丢唤醒。
- Active / Pending 或出现未决交易，即使元数据处于维护模式，也不能被低频过滤。
- 原项目证书 TTL 300 秒、660 blocks；公共源 TTL 90 秒、120/135 blocks 不变。门禁失败时可排队提前同步，但当前请求仍拒绝，需同步后重试。
- 旧全项目成功标记不会把 maintenance 跳过误认为完成整轮；使用本功能必须保持项目独立 freshness 开关开启。

## Admin

读取：`GET /v1/projects/:id/tracking`。

修改：`POST /v1/projects/:id/tracking`，`{action, reason, revision}`。action：maintenance、normal、sync、archive、unarchive。需要 project execute 权限，事务中重新检查管理员权限、V3 身份、状态、cursor、revision 和未决交易。原因 3–500 字符。审计 UPDATE / DELETE 被数据库 trigger 拒绝。

归档要求 Finished 证据当前、无 wake、公共生产最近 90 秒内成功，证据距公共 tip 至多 120 blocks。归档后仍消费公共事件异常；冲突/恢复会撤回结清结论并重新跟踪。

## 分阶段发布（未执行）

1. 备份并独立审核 `2026_10_30_ido_v3_project_closure_tracking.sql`，显式设置 migration approval；不得让应用启动自动建表。
2. 部署兼容的新 Backend、Job、Admin，开关默认关闭，无历史 backfill。
3. 确认 `idoV3ProjectFreshness.enabled=true`，迁移及 schema fences 完整，再独立授权启用 Job 的 `idoV3ProjectTracking.enabled=true`；完成初始核验后启用 Backend 同名开关。
4. 观察 Finished、wake、due、交易队列和 Active 时效。先选择单个终态项目进入 maintenance。
5. 回退先关闭 Backend / Job tracking 开关，再回退二进制；保留 additive 表和审计。关闭后恢复原有全部项目调度，不删除证据/审计，不逆转资金事实。

本实现仅支持 V3；V2 不套用这些结清规则。未执行生产迁移、配置写入、打包或部署。

## 验证

`GOEXPERIMENT=jsonv2 go test ./pkg/common/projecttracking ./pkg/common/idov3whitelist ./pkg/job/... ./pkg/core/admin/... ./pkg/core/service/project`

`scripts/test-project-tracking-mysql.sh` 只创建 network-none 临时 MySQL 8.4；迁移重复安装、约束、审计防改、真实事件重放与 projection parity、成功/失败分支结清、due 排除、事件唤醒及事务回滚在本地验证。fixture 路径支持 `JC_LOCAL_SCHEMA_FIXTURE_ROOT`、`JC_LOCAL_FRESHNESS_SQL`、`JC_LOCAL_TRACKING_SQL`，不接受生产 DSN。
