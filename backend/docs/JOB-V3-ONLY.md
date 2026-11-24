# Job V3-only 运行模式

配置 `application.jobMode: ido-v3`，保留 `chain.project.v1.enabled: false`、完整 V3 独立源/approved/freshness 配置及 `idoV3ProxyStream.mode: stream`。

## 实际作用

- 不初始化 V1/V2 RPC client，不读取 V2 历史区块，不要求 V2 jobPrivate 来恢复旧交易。
- 不注册 V1/V2 event scanner、reorg worker、transaction recovery、自动结算/TGE 或 V2 对账 worker。
- 仅运行 approved V3 scanner、独立 public producer、原始回执及既有 sender-maintenance lanes。
- `/health/job-ready` 读取 V3 公共流当前状态、epoch、SQL UTC 时间年龄、确认链头 lag、源 freshness 及当前 V3 阻塞交易/真实 reorg incident；不会因为 V2 旧游标落后而失败。
- 历史 halted V3 项目仍保持原状态，通过 `/health/ido-v3-stream` 看到，不会自动恢复。健康接口不授权交易。
- `ido` 原模式保持兼容，启用它仍会运行 V2；不是单独调整一个 V2 enabled 参数。
- `chain.project.v3.enabled:false` 是既有身份配置字段，实际 V3 扫描由 `idoV3Approved.enabled:true` 驱动，不要把该身份字段改 true。

## 发布边界

这是 Job 源码/配置兼容变更，不执行数据库迁移，不解除旧 Job journal。Backend RPC、旧 V2 数据和合约状态均不修改。

旧 Job 不认识 `ido-v3`，不能先切生产配置后期待旧镜像正常启动/回滚。发布时必须协调配置与支持版本，并为失败回滚恢复旧配置。

当前若存在失败 `job_pending`，禁止直接再发 upgrade 或清空 pending；先保存精确失败命令/release/generation/容器/readiness，设计明确的受控恢复和配置兼容顺序。旧镜像 V2 readiness 已坏时，不能谎报其健康来通过 journal 清理门禁。

本模式只解决 V2 退役和对应 health 依赖；不能修复真正的 V3 RPC/游标、suspended sender 或 nonce 冲突。Backend sender-isolation 仍需要单独授权的数据库触发器迁移与应用发布。
