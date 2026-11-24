# Job 日志可靠输出与 V3 消费暂停诊断

## 事件背景与边界

2026-10-27，项目 `43368658176704510` 在用户报告使用项目方钱包投资
0.05 USDT 后出现 `V3ReorgHalted`。数据库最后成功游标为 `130106501`，
成功 revision 3388，暂停后 revision 3389；journal 只有创建、充值、激活，
没有已提交的 purchase。此证据不能证明投资失败，也不能证明暂停由某一具体
校验触发：本轮未获取投资原 hash，旧日志缺失，可能的事务回滚不会留下 purchase。

本改动仅改善 Job 日志，不恢复生产项目、不广播交易、不改变资金/暂停/重试门禁。
投资原 hash 和成功回执仍是核验投资结果所需的独立证据。

## 旧日志为何不可见

- 现场 Job 配置没有 `logger.level`。旧 `levels[level]` 未找到 key 时取
  logrus 零值 `PanicLevel`，INFO/WARN/ERROR 都可能被过滤。
- 应用输出只有滚动文件，没有容器 stderr；writer 初始化错误被忽略，写入
  失败也没有可用的独立兜底。不能根据文件不存在反推业务没有发生异常。

## Job 专用初始化

`cmd/job` 改用 `common/log.InitJob`，Core/Notice 仍使用原 Init，不顺手更改：

- 空/无效级别默认 INFO；显式合法级别仍受尊重。默认保留期 7 天。
- JSON 输出带 RFC3339Nano 时区时间；固定 prefix `job`，目录为
  `<logger.dir 或 RunDir>/logs/job-YYYYMMDD.log`。
- 容器 stderr 与文件双写。文件初始化/写入失败不阻止 stderr；运行期
  `log_file_write_failed` 固定通知限一次，不包含私密路径或 OS 错误正文。
- `logging_ready` 启动通知直接写 stderr，即使显式高日志级别也可核对初始化。
- 常见 RPC URL、Bearer 和凭据键值在 Job formatter 中脱敏。新 V3 诊断仅
  输出结构化字段白名单，不依赖 formatter 来保护 raw SQL/钱包/金额/calldata。
  这不是对旧代码所有潜在敏感日志的完整审计。

## V3 失败记录

每次项目调度创建独立 trace，安全字段包括：

- project_id（字符串）、phase、cursor_block/revision、epoch、window_from/to；
- 正在处理的 block_number、proxy/selected log 数和 purchase_count；
- 有限 RPC 方法名、错误类别（deadline/canceled/network/rpc_http/rpc_response/
  database/evidence_conflict/unclassified）、MySQL 错误编号；
- context_status、elapsed_ms 和 decision。

关键决策在执行前/后记录：defer、persistent_revoke、revoked、revoke_failed；
producer 对应 revalidate / persistent_stream_halt / stream_halted / halt_failed。
失败时必须保留发生错误的 phase，不在撤销回调中改写为“清理成功”。

事件路径细分：准备/回执/project recheck/policy/final anchor，进入事务后细分
journal insert、历史 replay、创建一致性、projection match、financial apply、
business apply、outbox apply、cursor update，再到 COMMIT 和 postcommit anchor。
投资事件只记录计数，不记录买方、支付金额、proof、完整交易或 RPC 参数。
trace 跟随 context，单项目隔离并加 mutex，支持并发 header 回调。

日志示意（字段值仅示例，不代表现场根因）：

```json
{"msg":"V3 scan failure decision","project_id":"43368658176704510","phase":"journal_projection_match","block_number":130106502,"purchase_count":1,"decision":"persistent_revoke","error_kind":"unclassified"}
```

若这条记录出现，就能把“投资后页面暂停”缩小到某个具体阶段，而不是猜测 RPC。
通用 helper 内部仍可能只返回 redacted error；此次不把错误正文写入日志，
后续可继续为具体不变量增加有限枚举 check。

## 验证与运维验收

- 空/错误配置下 WARN 同时可见于文件与 stderr；文件路径不可用、运行期
  writer 失败/短写仍能持续输出；有限失败通知不会刷屏或泄露路径。
- 病例字段/脱敏/项目隔离/并发 trace 的 unit/race 测试。
- 原 Job、journal 经济/幂等/回滚、MySQL 约束测试保持不变，通过说明观测不改变业务结果。
- 发布后应在 `docker logs jc-job` 中看到 logging_ready；模拟或自然失败应能
  获取项目、阶段、窗口与决策。核对容器日志轮转/磁盘预算需独立配置检查。
- 本次不补写历史日志，不能恢复已经丢失的 22:37 暂停原因。未部署前生产不会
  自动获得这些日志。打包和发布分别授权。
