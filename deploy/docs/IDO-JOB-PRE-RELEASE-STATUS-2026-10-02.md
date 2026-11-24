# IDO Job / Fair Launch 发布前验收状态（2026-10-02）

**结论：NO-GO；此文件不是生产变更授权。** 本仓库当前 `docker-compose.yml` 只有 `services: {}`，`jc-release` 对 Job 保持 `job_runtime_admission_unavailable`，不能把离线构建或测试当作生产 Job 上线。不得绕过现有门禁、手动向生产 Compose 塞入 Job、让 Job 使用 Core operator 签名者，或因归档 scanner 可用就启动 Fair Launch 财务物化。

**目标边界（用户已确认）**：保留此前 Backend、Frontend、Admin 的受控发布能力，并增加 IDO Job 发布；这不等于授权 Fair Launch 财务物化或立即启动生产 Job。2026-10-02 只读预检发现：当前 checkout 的 `jc-release-server` API/Storage 拒绝所有新上传及非日志命令，Agent 还单独拒绝 Job，策略注册表的 Job 为 unavailable；因此仅更新服务端二进制不可能完成目标，直接从此 checkout 构建还会撤销已有 Core 服务发布入口。旧版 Job 自举实现复制 Backend 环境变量／挂载，不满足独立 signer 隔离，不能恢复。生产应用主机未见 Job 配置、Compose 服务或运行容器。当前静态发布策略测试也返回非零，且其“所有策略不可用”断言与保留三种服务能力的目标冲突。须先由策略维护者在正反向回归下统一 Server、Agent、策略、发布包、迁移与运行准入，再制作可回滚的升级包；现有生产控制机二进制未更换。

继续推进：在 `tools/jc-release/internal/strategies/ido_admission.go` 增加**只读、非授权**的 IDO Job 静态布局检查及正反向测试（缺配置、Fair Launch 角色、V1 模式、启用 scanner、Core 入口、Backend 整体配置挂载、缺 Job readiness 均拒绝）。这是未来发布准入的一个必要但远不充分的条件，尚未接到 Agent 发布路径，也不会在生产调用；它不验证私钥、链身份、迁移状态、RPC 历史、健康及回滚。`tools/jc-release` 全量 `go test ./... -count=1` 通过（`D:/pi-tmp/jc-release-layout-20261002.log`）。生产服务未升级，Job 发布仍未开放。

复核 `cmd/job` 实际读取 `job/common/secret/config.yaml` 后，修正这项静态检查：它现在要求独立 `job-runtime/config` 三文件（不再接受先前错误的单个 `config/job`），只读检查 Job signer 字段存在且 Core relayer 字段不进入 Job secret；Compose 必须单一只读挂载到 `/app/config`，精确 Job 入口，不继承 Backend 的 `env_file` 或 `environment`，并指向 `/health/job-ready`。这项检查会在进程内短暂读取私钥字段以判断是否为空，**不打印或持久化密钥值，不证明 signer 地址不同，也不表示允许发布**。Windows 和 WSL Linux 测试均通过；Go 全量回归见 `D:/pi-tmp/jc-release-layout-final.log`。生产 `docker compose config --services` 只有 redis/redis-init/backend/tgbot/admin/frontend；Job 配置仍不存在。下一阶段仍必须解决 Server/Agent 旧能力与静态门禁的策略冲突及生产运行准入。

追加 Viper 合并顺序的反向测试：`common`/`secret` 对环境、角色、V1 和 scanner 标记的覆盖被拒绝，任何文件含 Core relayer 字段也拒绝；避免只核对 `job/config.yaml` 却在后加载配置中打开 Fair Launch。候选 `jc-release` Go 全量测试通过（`D:/pi-tmp/jc-release-layout-override.log`）；静态激活门禁仍为 **FAIL**（`D:/pi-tmp/jc-release-static-post-layout.log`，先命中已有安全文档），不得称为发布准入成功。

又补充 Viper 键名大小写归一化的拒绝性回归，防止在后加载配置里写 `APPLICATION.jobmode` 或 `CHAIN.PROJECT.PRIVATE` 绕开静态检查。Go 全量和 vet 分别通过（`D:/pi-tmp/jc-release-layout-case.log`、`D:/pi-tmp/jc-release-layout-case-vet.log`）；仍没有将此检查接入实际发布路径。**静态门禁失败、生产 Job 所需迁移／配置／发布通道缺失，均维持 NO-GO。**

检查器进一步拒绝 Job Compose 中在其他路径挂载 Backend 的整个配置树，除了单独的只读 Job 配置只允许 `job-logs` 命名卷；并要求 Linux 宿主 Job 配置目录非组/其他用户可写。Go 全量回归 `D:/pi-tmp/jc-release-layout-permissions.log` 与 WSL Linux 定向测试通过。**这些加强没有使静态检查变成生产授权**，也不应为了消除之前整套静态门禁失败而跳过测试。

补上大小写重复键的无序 map 回归，保证 `application` 与 `APPLICATION.jobmode` 同时存在时不依赖 Go map 遍历顺序；全量 Go 回归 `D:/pi-tmp/jc-release-layout-duplicate-case.log` 通过。该源码只定义必要条件，服务器与 Agent 仍拒绝 Job 命令。

按“保留三种既有能力并开放 IDO Job”的整条发布路径重新检查时，尝试的独立 Job 容器策略原型在代码审查中发现**无法安全交付首次部署和自动回滚**，并且无法单靠 Docker save 的 `manifest.json` 证明已签署的制品来源及部署资格；该原型已在同一工作轮删除，未注册到 Agent、未构建升级包、更未上服务器。当前不应通过补一个 image tag 或跳过 `Backup` 来开放入口。要构成可发布候选，必须有：按组件／环境绑定的受控制品准入和镜像摘要、生产迁移／签名者与链身份核验、首部署失败时明确的安全停止及后续版本的可验证回滚、Server/Agent 同步准入、静态策略的正反向回归。这些必要证据目前不齐；局部布局测试通过不等于完成用户授权的发布目标。

在不改变生产行为的前提下增加 `tools/jc-release/internal/activation/artifact.go` 的**离线、未接线**双签名制品声明验证器及反向回归：域／环境／组件／generation／过期时间／SHA-256／image ID 精确匹配；固定至少三把受信公钥中两把不同 Ed25519 签名，拒绝额外 JSON 字段和非规范字节。测试生成临时密钥；没有默认的生产公钥、持久单调状态或 Agent 调用，因此它不会打开 API、不能代替生产制品准入，且不保证回滚 target 资格。Go 全量／vet 通过（`D:/pi-tmp/jc-release-artifact-unwired.log`、`D:/pi-tmp/jc-release-artifact-unwired-vet.log`）。Server、Agent、静态策略及生产迁移阻断均保持原状，生产服务没有升级。

签名验证还要求**整个 envelope** 精确规范序列化，拒绝重复 `declaration` 键以及不同 JSON 解析器可能歧义的空白／键顺序；全量 Go 测试与 vet 通过（`D:/pi-tmp/jc-release-artifact-canonical.log`、`D:/pi-tmp/jc-release-artifact-canonical-vet.log`）。仍无上线授权和可执行 Job 策略。

用户进一步明确：**是否执行发布应由授权操作人员决定，不能用“六个组件一律没有发布策略”的业务冻结代替人的决策。** 静态脚本原先要求 `&unavailableStrategy` 恰好 6 个，还禁止策略目录出现 `exec.Command`/Docker；这与已存在的 Backend 镜像及 Frontend/Admin 静态策略相互矛盾。当时已将这两项不成立的笼统断言改为显式组件矩阵（三项已有策略；其余三项当时没有可执行策略），同时保留各自正反向单测及原有的其他安全测试。该轮**只修改了测试对当时事实的断言**，没有改 Server/Agent API 或部署控制；`jc-release` Go 全量回归 `D:/pi-tmp/jc-release-policy-matrix-go.log` 通过，静态整套当时 FAIL（`D:/pi-tmp/jc-release-human-policy-static.log` 先命中历史安全文档误报，后续规则未运行）。上述是 10-02 时间点记录；此后 Job 本地候选策略已经开发，但不代表生产已安装或获准启动。

## 已有隔离证据（不等同于发布批准）

| 范围 | 已验证 | 不可外推为 |
|---|---|---|
| 固定基线＋0903–0907＋候选 1002 迁移的一次性 MySQL 8.4 | 旧 outbox INSERT 缺列失败；受控会话审批后的迁移重复执行、checksum ledger、当前 INSERT 以及 Core／Job 启动时全部 schema gates 通过。 | 生产数据库已经迁移，或 0908–0918 的完整 Fair Launch 生命周期已验收。 |
| IDO V2 outbox | Core/Job 共享结束槽互斥，Job 独立签名者 nonce fence，同一交易重播；**完整迁移库＋隔离 Redis＋真实本地 Hardhat** 的 ProjectEndJob 实际每分钟 cron 广播、receipt、scanner 实际 cron 消费 `ProjectEnded`、outbox `applied` 及独立 Go 进程接班重扫不重复通过。 | 整个 `StartAll` 所有任务的生产类编排、真实重组恢复或生产链运行。 |
| Scanner | 完整库和一次性 Redis Cluster 验证锁、失败持久化、死信、告警及后继进程重新取得租约；生产同款 cron scanner 分别触发过注入链头故障及真实本地 Hardhat 成功链上事件处理，游标持久化。 | 真实重组后的事务性补偿、整套 Job 周期运行或生产监控。 |
| 停机 | IDO scanner 的立即追块和 cron 共享停机 context；隔离 Redis＋MySQL 验证链头读取中止后故障落库、锁释放可接班。 | 其他 Job 子任务均支持取消，或完整部署在失联 RPC 上能于停机超时前优雅结束。 |
| TESTCAT #110 | 完整迁移库真实 CHECK 复现旧审批 INSERT 的 MySQL 3819；修复后的合成申请 INSERT 成功，五个 Fair Launch-only raw 列仍为 NULL。 | 修复的 Backend 已发布或用户已重审申请。 |

在当前候选代码上运行的后端 `GOEXPERIMENT=jsonv2 go test ./... -count=1` 和 `go vet ./...` 均通过。首次全量 Go 测试曾因 Windows 上两个 Notice 测试的 `t.TempDir()` 日志文件仍被轮转 writer 持有而失败；将这两处测试 logger 的输出改成 `io.Discard` 后重跑通过。该测试修正不改变生产日志配置。

可复查的隔离测试入口：`scripts/tests/ido_outbox_schema_disposable_test.sh`（其可选 Go/Redis/cron 测试均需一次性本地依赖）；实际完整库脚本不会连接外部 RPC。Core 与 Job 的 schema gate 回归分别位于 `pkg/core/service/project/application_approval_full_schema_test.go` 与 `pkg/job/job/chainevent/ido_v2_automatic_end_full_schema_test.go`（后端源码仓库）。候选迁移 `migrations/2026_10_02_chain_tx_submission_signing_metadata.sql` 的 SHA-256 为 `0a0d4a20a36065175aadf17111a7730b473615628d3e88c9eadcd3e9fc9d8bb2`；修改任一字节后必须重新审核迁移与 Core／Job 的 checksum 绑定，不可仅改常量放行。

## 发布前尚未满足的验收条件

1. **IDO V2 剩余全链路**：已在隔离完整库＋Redis＋真实本地 Hardhat 跑通实际到期 cron、独立 Job signer 的 `endProject` 广播／receipt、真实 scanner cron 消费和游标推进、独立 Go 进程成功接班不重复；接下来须覆盖整个 `StartAll` 其他任务，以及真实重组/故障后的事务性恢复与停机边界。生产确认深度、DB 容器时区和 RPC 历史能力仍须独立验证。不能把本地成功当成生产可用。
2. **Fair Launch 财务与治理**：审查全部 CE-V2/V3 效果与 digest、原子重组反转、生产 authority wiring、0908–0918 迁移及真实 MySQL/RPC 生命周期；独立证明 scanner 的归档结果可以安全物化和反转。在此之前保留 `FairLaunchMaterializationEnabled=false` 和 `fairLaunchCEV2PersistenceEnabled=false`，Fair Launch Job 角色继续拒绝启动。
3. **生产发布设施及审查**：当前 Compose/`jc-release` 没有可用 Job 服务/策略。必须设计并审核分离的 IDO 与 Fair Launch 进程及签名者、配置来源、数据库最小权限、健康/停止/回滚、链身份、RPC 历史可用性、确认深度和观测告警，再走独立受控发布准入；不能通过删除 `job_runtime_admission_unavailable` 代替证据。
4. **迁移及 Core 部署顺序**：候选 1002 迁移必须通过外部备份/回滚证据、checksum-bound manifest 和受控迁移审查；其后才能发布带 `RequireChainTxSigningMetadata` 的 Core/Job。迁移未经生产确认时发布新 Core 会 fail-closed。旧的 TESTCAT Backend 候选包不包含后续 schema gate 代码，不能充当当前整套 Core/Job 发布包。
5. **独立审批问题**：先受控发布并验证 TESTCAT #110 的 Backend 3819 修复，之后由用户自行重试审批；不更改生产 CHECK、不代替用户操作申请或资金。`testido` 与 TESTCAT 是否同一项目尚未确认。
6. **现有静态策略测试**：`scripts/tests/fair_launch_activation_static_test.sh` 当前返回非零，文档扫描把两个既有已跟踪文档的拒绝/不可用说明命中禁词模式；完整失败输出保存在本地测试日志，不能因为部分子测试显示 PASS 就声称整套静态门禁通过。复查发现 `PROHIBITED_DOC` 及 MySQL 操作指令模式均可跨同一 Markdown 长段落从文件名或安全说明开始贪婪匹配后文，不能把该命中直接解释为真实可用操作。但测试继续运行也识别出真实问题：历史 Daqi 隔离演练脚本曾在 MySQL 命令参数中传递密码；现已改成一次性受保护的 MySQL 配置文件和容器密钥文件，尚未在有合格隔离备份的环境执行端到端回归。另有已有的拒绝性 grant fixture、只读发布验证器也会被后续通用扫描命中。策略测试未改动、仍不通过；必须逐项复核和加入真假违规回归，不得简单关停检测。

2026-10-02 再次从当前候选编译 Linux/amd64 scanner 测试二进制后，隔离完整库＋Redis＋实际 cron **故障注入**测试通过（`D:/pi-tmp/ido-prerelease-final-cron.log`，包括 Core/Job schema gates）；一次性测试容器清理完毕。它仍未覆盖真实 Hardhat 的完整库成功扫描。静态策略失败的复查输出见 `D:/pi-tmp/fl-activation-static-4.log`；此轮未修改策略脚本及其受保护的运行时准入门禁。

后续候选修正：V2 项目自动结束现在从 RPC 已确认区块读取时间，避免 Hardhat 快进链时漏调度或宿主时钟偏快时提前广播；安全块为零或 RPC 出错时拒绝调度。`TestConfirmedV2EndTimeUsesSafeBlockNotHostClock` 与 `GOEXPERIMENT=jsonv2 go test ./... -count=1`、`go vet ./...` 已通过（`D:/pi-tmp/ido-project-end-clock.log`、`D:/pi-tmp/ido-prerelease-all-current.log`、`D:/pi-tmp/ido-prerelease-vet-current.log`）。下述实际定时器回归进一步验证该候选修正；尚不能取消其他 IDO 发布条件。

随后在一次性网络隔离容器中完成这条实际定时器回归：`scripts/tests/ido_outbox_schema_disposable_test.sh` 设置 `JC_IDO_FULL_SCHEMA_GO_TEST=1`、`JC_IDO_FULL_SCHEMA_REDIS_TEST=1`、`JC_IDO_FULL_SCHEMA_HARDHAT_TEST=1` 并使用重新编译的 Linux/amd64 Go 测试二进制，运行 `TestIDOV2FullSchemaHardhatEndAndScanner`。证据 `D:/pi-tmp/ido-hardhat-fullschema-run5.log` 显示通过（17.99 秒），包含实际 ProjectEnd cron、真实 Hardhat receipt、V2 `ProjectEnded`、游标和后继重扫；启动/迁移/outbox 的旧测试也通过。Hardhat 使用独立临时命名空间和只读源码挂载，私钥 fixture 仅在临时容器内文件及 0700 临时目录中传递，测试后容器和目录已清理。**这里只做隔离测试，生产迁移与发布均未发生。**

再扩展成功扫描使用实际 `GetChainEventJob.Add` 注册和 `cron.Start` 调度，并以第二个独立 Go 测试进程重新取得 Redis 锁／DB 租约，核对游标及结束事件幂等。`D:/pi-tmp/ido-hardhat-fullschema-run7.log` 两个阶段分别 PASS（86.06 秒／0.05 秒）。首次扩展试验曾被此前**故障路径测试留下的 scanner 状态**阻塞；现在仅在受控一次性 schema 的首阶段重置该 singleton，后继阶段不重置，以验证真正的跨进程持久化。此测试仍不涵盖生产链或 Fair Launch 财务物化。

随后四个可选门禁 **Go＋Redis＋故障 cron＋Hardhat 成功 cron** 与 Core 全 schema 审批回归在同一一次性脚本中联合 PASS：`D:/pi-tmp/ido-prerelease-combined-final2.log`。两个每分钟 cron 各自等待独立时钟边界，测试超时改为 155 秒；100 秒的首次组合运行仅因测试总超时结束，不代表业务成功路径失败。已确认测试容器与宿主 fixture 均无残留。Go 全量测试／vet 仍通过（`D:/pi-tmp/ido-all-after-successor.log`、`D:/pi-tmp/ido-vet-after-successor.log`）。

Daqi 一次性演练脚本的数据库密码传输加了非执行型静态回归 `scripts/tests/daqi_rehearsal_secret_transport_test.sh`（`bash -n`、配置／密钥文件、现有容器拒绝、CID 清理和命令行明文拦截均 PASS）。**它不加载备份、不代表隔离备份演练通过，也不改变 Fair Launch 整套静态门禁仍失败的结论。**

即使所有代码项完成，生产链身份、RPC、密钥分离、数据库权限、备份与业务操作仍需指定责任人审核和授权；未通过时保留 NO-GO。
