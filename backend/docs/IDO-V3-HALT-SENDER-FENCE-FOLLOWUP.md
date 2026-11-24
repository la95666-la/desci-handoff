# 待解决：V3 间歇性暂停、锁竞争与跨项目 sender 阻塞

- 状态：**OPEN / 暂缓处理**。2026-10-27 用户要求先恢复项目，较大修改后续统一处理。
- 范围：V3 公共流／项目消费者的暂停与撤销路径，以及共享 Operator 的 nonce 签名守卫。
- 本记录不是生产执行授权，也不代表根因已全部查明。恢复成功不等于永久修复。

## 现象与影响

1. 公共流或项目消费者间歇性进入 `halted` / `reorg_halted`，项目显示
   `V3ReorgHalted`；未知源暂停本身不能证明真实链重组。
2. 项目投影撤销时，原已 applied 的创建／激活／结算记录转为
   `v3_reorg_suspended`，保留原签名、hash、nonce 和事件关联。
3. 当前 Go 签名器与数据库 nonce 守卫按共享 sender 检查 suspended 记录。
   即使无 active slot，另一个项目的创建也可能被阻止。
4. TESTCAT `43359242266083329` 创建 #58–#61 均 unsigned cancelled：
   nonce/hash 为空，未上链。排障时确认同 sender 的非退役项目 suspended
   记录是必须先解决的硬阻塞；内部阶段错误未完整保存，不能排除其他检查失败。

## 已保存的证据

- 生产 MySQL `LATEST DETECTED DEADLOCK`（2026-10-27 03:33:49 UTC）：
  - 一方持公共流行 X 锁，等待项目游标行 X 锁：
    `haltV3Stream` 的全项目 `SELECT ... FOR UPDATE`。
  - 另一方持项目游标行 X 锁，在将其更新为 `reorg_halted` 时等待公共流行 S 锁。
    该更新涉及查询公共流的游标触发器，存在反向锁顺序。
  - MySQL 回滚其中一方；嵌套事务还出现 `SAVEPOINT ... does not exist`。
- 证据文件：`D:/pi-tmp/jc-v161-deadlock.log`，并复制到私密恢复备份目录。
- 尚未闭环：每次最初触发撤销／全局 halt 的准确阶段、RPC 错误／预算耗尽／
  取消与死锁之间的因果顺序。不能把所有暂停都归因于同一个死锁。
- 停止 Job 可能取消正在运行的扫描并触发保护撤销；维护窗口也要考虑该副作用。

## 临时恢复结果（历史快照，后续必须重新查询）

线上 Job v161：source `8698101f8c0478d164af7b7c90400c45537e997b`，
release `83e12468-0066-44a2-98d1-c6e865372f08`，generation 21。

- `43342706038013953`：核验原 #55–#57、审计恢复并补扫 5812 块。
- `43264392090877953`：核验原 #52–#54、审计恢复并补扫 2155 块。
- 最后读回两个项目 `Failed / running`，#52–#57 均 applied、active slot NULL。
- nonce 原为 48；恢复期间独立应用创建 #62（项目 `43359438123302913`）使用
  nonce 48 且 applied，故 next_nonce 正常推进到 49。不是恢复脚本改 nonce，
  也不是助手发起的新创建。
- Job 同一镜像／容器恢复 healthy、readiness=true、pending=null；没有修改全局
  守卫、退役名单或配置，没有重签／重发／直接 SQL 伪造 applied。

私密回执与目标快照（不把原始配置／签名材料复制进仓库）：

- `/home/la956/.joincare-production-backups/20261027T055338Z-project-43342706038013953-recovery/`
  - `RECEIPT.json` SHA256 `63a38adcb218d60f1b80132137f14f46139caa1353513de78aa521e7aa1c6376`
- `/home/la956/.joincare-production-backups/20261027T060938Z-project-43264392090877953-recovery/`
  - `RECEIPT.json` SHA256 `107322e0ca6c38434dde86208eafd43017abb9bb4b4544f8a0ea5f17cabed757`
- 应用机对应证据：`/var/backups/joincare/20261027T055400Z-project6-recovery/`、
  `/var/backups/joincare/20261027T061100Z-project-old-recovery/`。

## 后续处理清单

- [ ] 补充脱敏阶段诊断：准入、RPC、准备、提交、最终复核、撤销；保留原始失败，
      区分已完成预算让出、真实超时、canonical 冲突、死锁和维护取消。
- [ ] 用真实双连接 MySQL 测试复现全局 halt／单项目撤销／消费／恢复间锁竞争，
      统一公共流与项目／业务行的锁顺序，处理 SQL 整事务被回滚后的嵌套事务语义。
- [ ] 区分“项目投影不可信”和“sender nonce 不明确”：评估仅在原签名、原 hash、
      canonical 回执、确认深度、nonce 已消耗以及链上／DB nonce 一致均独立证明后，
      允许无关项目签名的受控证据机制。Go 和 SQL 必须使用一致判定。
- [ ] 退役豁免不是非退役项目的通用放行；不得通过归档项目、删 suspended、清 nonce、
      放宽 canonical／确认深度或伪造 healthy 绕过设计。
- [ ] 核对全局扫描健康与 Job readiness：container healthy 不证明 V3 所有项目可用，
      业务暂停／未消费回执会使发布与回滚健康检查失败，需明确发布前预检。
- [ ] 回归：真实重组、未知原交易／nonce、receipt 替换、CAS、崩溃、取消、幂等、
      deadlock victim、精确 revision 撤销；真实不明确状态仍须拒绝新签名。
- [ ] 发布后持续验证公共流和每个项目 lag 收敛、不反复暂停；证明一个项目投影暂停
      不会在已证明 nonce 安全时无期限阻止另一个项目。源码／打包／发布分别授权。

## 重点代码

- `pkg/job/job/chainevent/v3_proxy_consumer.go`：`haltV3Stream`。
- `v3_proxy_prepared_consumer.go`：准备／发布／postcommit 撤销。
- `v3_proxy_runtime.go`、`v3_proxy_producer_job.go`：调用者错误／暂停处理。
- `v3_journal.go`：`haltRangeUnknown`、恢复事务。
- `v3_proxy_stream_schema.sql`：stream cursor triggers。
- `pkg/common/chaintx/v3_retired_nonce.go`：`requireV3SuspendedSenderTx`。
- `pkg/common/chaintx/v3_retired_nonce.sql`：nonce triggers 与退役证据 view。

相关：`IDO-V3-CONSUMER-PREPARED.md`、`IDO-V3-STREAM-THROUGHPUT.md`。
