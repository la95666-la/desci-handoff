# IDO Job 第二阶段：生产前提核验记录（2026-10-03 UTC）

> **2026-10-04 Job 配置已准备、未发布：** 用户授权补全前述公开配置。应用机以 `Go-http-client/1.1` 的只读 BSC RPC 请求核对 chain ID 56、BSC USDT `0x55d398326f99059fF775485246999027B3197955` 和 USDC `0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d` 非空代码和 `decimals=18`，V2 proxy 的 `paymentTokenConfigs(address)` 均返回 `allowed=1,decimals=18`；起点及最新附近 501 区块 `eth_getLogs` 成功。这是时间点抽样，不代替长期 RPC 保障。先将原 `agent.yaml`、Job 公共 YAML、正式 Compose 备份到应用机 root-only `/var/backups/jc-release/job-config-20261004T053626Z`，再原子写入：Job `paymentTokens.USDT/USDC` 的地址和 18 decimals、Agent `job_environment: prod`、正式 Compose `job` 服务和 `job-logs` 卷（依据此前候选的只读独立挂载及 `/health/job-ready` 探针，固定镜像标签 `joincare/job:ido-097e51e-6ba80c3`，实际 Agent Apply 会使用已核验的不可变镜像 ID override）。未读/输出/改写 Job 私钥、DB 或 Redis secret。仅重启 `jc-agent` 使环境生效，运行哈希仍 `a5fc78a76aee64335e4ff606b91111bc8c52870d460b9c957ba1fab1232f553c`；正式 Compose 现有 Job 定义但**未 `up`**，容器仍为 0，Backend healthy。包尚未上传 `jc-release`、没有下发发布命令；未完成 Core signer 控制/nonce/outbox、历史活动空档及 scanner 首次运行核验，**仍不可声称 Job 正常运行**。

> **2026-10-04 最新状态（本段覆盖下文旧时间点结论）：** 用户要求按 CI/CD 组件包 → 人工上传 `jc-release` → 应用机 `jc-agent` 的原有链路开始。发现 Docker 当前 `save` 包使用 OCI `blobs/sha256/` 条目，修复 Server 镜像 ID 解析并添加回归（`6ba80c31fb0cada95f8d9367ba6fa0d4342c2322`）；从该提交的 `git archive` 构建并校验 `jc-release-20261004T045055Z-6ba80c31fb0c`，Go 全量测试、vet、WSL race 均通过。逐机重新核对已获用户确认的 SSH 指纹、verify-only、dry-run、备份后，应用机只升级 Agent：运行/磁盘哈希 `a5fc78a76aee64335e4ff606b91111bc8c52870d460b9c957ba1fab1232f553c`，备份 `/var/backups/jc-release/20261004T045139Z-6ba80c31fb0c`；控制机只升级 Server：运行/磁盘哈希 `84ce46e76b35bf96f4ee6933c9b17c144d4c11d275ac88f99a9ee7cd23ae0e86`，备份 `/var/backups/jc-release/20261004T045226Z-6ba80c31fb0c`。二者 active，Backend healthy，控制机历史 Agent 未变。应用机 Agent 仍未配置 `job_environment`（需要 `prod`），正式 Compose Job 服务和容器均为 0。

> 同轮按打包技能从 Backend **固定提交** `097e51ead601196b04e7089e4761e1c1c5fca229` 的 `git archive` 运行 Job/schema 测试并构建 Linux amd64 Job ELF（SHA-256 `23f29805ae77f424274bba177c90224956d50edf82714c22c3aea3659ddae136`）；由部署提交的专用 `deploy/Dockerfile.job.runtime` 本地 Docker build 出 Job-only 镜像 `sha256:b235e599b065205ac6390cafa432bcdc923e021f4fe0eef3245a00417cb8a0d5`，gzip docker-save 包在 `dist/joincare-job-ido-097e51e-6ba80c3.tar.gz`，大小 69,147,936 bytes，SHA-256 `236d1a112637ec4c6cf1b52a4f8a9f0c62ca71d8f34c3d1b8411cd052e616c2c`；本机实包解析测试通过。**GitHub Actions 任务仍 `if: false`，这是按技能本机构建，不是 CI 运行回执。包未上传，未下发 Job 发布。** 惰性 Compose 仍是 `joincare/job:review-pending`；Job 独立配置三树只读键检查发现 V2 `paymentTokens.USDT`/`USDC` **均缺失**，而 Backend Job 源码 `pkg/job/chain/chain.go:verifyLatestV2Governance` 会要求这两币地址/decimals 并核对链上治理，首次启动会失败；应在受控只读链核验后补入可信配置，不能猜测。还需审查正式 Compose、历史活动空档、Core signer/nonce/outbox、扫描注册与运行回滚。**继续 NO-GO；未启动 Job。**

> **2026-10-03 控制机 Release Server 升级完成（最新）：** 用户对上一条列明的控制机 ED25519 指纹和 Server-only 升级回复确认。重查控制机 `154.26.137.111` 公钥指纹 `SHA256:L6leP77U5TrKW04QP1jtYTA7pBEq2/DPJ5peRu81ZCQ`，将提交 `4428f44046a4d23f4752295768d9a6850997a40f` 的包送入该机 root-only staging；该机 `--verify-only`、`--component server --dry-run` 通过后仅替换并重启 `jc-release-server`。运行与磁盘 SHA-256 均为 `1da102c032df79c7d9569ee2128ffb883f88b2d7c539941dd6afee70902a27fe`，服务 active/stable，`NRestarts=0`；旧 Server 哈希 `d99b4a444d5bb36739ee1fba5a2f153a229432ba8dc68ee1f39edaa909e8e7e5` 已备份于该机 `/var/backups/jc-release/20261003T180334Z-4428f44046a4`，原 YAML/state 未修改。控制机历史 Agent 哈希仍为 `b012c65e8baf94a931ff8bff6ff68205222674e038723ecd43e5d851c9dcb3f0`；应用机新 Agent 仍 active、Backend healthy、正式 Compose 无 Job 且容器为 0。**只有控制面二进制已升级**；生产 Job trust keys、签署材料、不可变镜像与正式 Compose/运行证据依旧缺失，不表示 Job 已获准启动。

> **2026-10-03 jc-release 部分升级（最新）：** 用户明确授权升级 `jc-release`。从部署提交 `4428f44046a4d23f4752295768d9a6850997a40f` 的 `git archive` 构建 Linux amd64 Server/Agent，完整 Go 测试、vet、Linux root/race 与 Fair Launch 静态门禁通过；校验绑定包 `jc-release-20261003T175320Z-4428f44046a4`。应用机 `18.143.209.192` 已独立确认指纹，包校验和 Agent dry-run 通过后，仅升级该机 `jc-agent`：运行及磁盘 SHA-256 均为 `51664187ab58d130886eeb03e2bf4b60d7201496614ee4380dea3884ff2ad803`，服务 active/stable，原二进制与配置摘要备份于该机 `/var/backups/jc-release/20261003T175454Z-4428f44046a4`；Backend healthy，正式 Compose 无 Job、Job 容器为 0。**控制机 Release Server 尚未升级**：其 SSH ED25519 指纹 `SHA256:L6leP77U5TrKW04QP1jtYTA7pBEq2/DPJ5peRu81ZCQ` 目前仅来自本地 known_hosts 与远端自述，未经过独立渠道确认；不得因应用机指纹已确认而推及控制机。控制机历史 Agent 亦未升级且根目录为 `/opt/joincare-test`。已安装应用机 Agent 但尚未配置生产 Job trust/readiness keys；这只是部分控制面升级，不开放 Job 发布或启动。控制机指纹经独立确认后才能继续该机 Server 的包校验、dry-run、备份与升级；仍须审查 Job 正式镜像、签名声明、独立回执及 Compose/健康与回滚。

> **2026-10-03 非 RPC 准备进度（最新；NO-GO）：** 为优先准备其他部分，已在应用机内从 Backend secret **仅复制** Redis `host` 列表及认证字段至 Job 独立 root:root `0600` secret；与先前获授权的同账号 MySQL 七字段、Job signer 共存，无 Core 私钥，也未传输密码。Job 配置已填入公开 BSC 只读观察的 V2 proxy、`operator()`、ERC1967 implementation 与 runtime Keccak 哈希，均待最终 RPC/批准 manifest 复核；`startBlock` 仍为空，未猜测。生产 `chain_event_sync_state` 第 1 行只读检查为 `initialized=0`、`v2_registered=0`、cursor/start/safe=0，不能把未注册状态冒充已就绪。应用机 `/opt/joincare-prod/job-runtime/job-compose.candidate.yaml` 是 root `0600` 的**惰性候选**（非 Docker 默认识别文件），含只读独立 Job 配置挂载、独立日志卷、指定健康探针和审阅占位镜像；`docker compose -f docker-compose.yml -f job-runtime/job-compose.candidate.yaml config --format json` 显示只增加 `job` 服务和 `job-logs` 卷，现有服务及网络解析未变。**实际** `docker compose config --services` 仍无 Job，容器为 0；候选镜像标记不是可安装制品，不得对候选执行 `up`。尚须经证实的 V2 部署/扫描起始区块、合格历史 RPC、scanner 源注册/游标与失败恢复、独立证据签署人/信任公钥、正式不可变镜像、升级/回滚审查和分别授权的升级与首次启动。用户希望先完成其他准备，并不构成将失败 RPC 标为 `verified` 的授权。

> **2026-10-03 复测与 DB 配置更新（最新）：** 用户明确选择 Job 使用 Backend **同一 MySQL 账号**。在已核对应用机上，仅于本机将 Backend secret 中 `db.mysql` 的七个必要字段复制到独立 Job `secret/config.yaml`，原有 Job signer 原样保留、无 Core 私钥；没有传输或输出密码，文件仍 root:root `0600`。回读确认值与 Backend 对应字段一致，Backend healthy，Job 服务／容器仍为 0；Job Redis 尚未配置，且共用 MySQL 账号并不等于最小权限。对已写入的临时公共 RPC `https://bsc-dataseed.binance.org/` 从应用机复测：chain ID 56；以观察时最新块约 125519487 为基准，深度 20、5000、100000 的单区块 `eth_getLogs` **均**返回 `-32005`；深度 5000、100000 的 `eth_getStorageAt` 返回 `-32000`，只有深度 20 的存储读取成功。它不满足需要历史日志和合约身份复验的运行条件，**拒绝签发 `chain-rpc.receipt` 的 `verified` 结果或双签 readiness**。必须替换并验收 RPC；不得把用户的“后面更换”当作当前合格证据。V2 起始区块和其余发布身份绑定仍未齐备；NO-GO 不变。

> **2026-10-03 临时 RPC 配置更新：** 按用户限定授权，仅把此前试测的 `https://bsc-dataseed.binance.org/` 写入应用机 `/opt/joincare-prod/job-runtime/config/job/config.yaml` 的 `chain.project.url`；用 root:root `0600` 文件做原子替换并回读校验，其他行、Backend 配置、服务均未变。该公共 RPC 对近期 `eth_getLogs` 曾返回 `-32005 limit exceeded`，**仅为临时占位，不可签署 chain-rpc verified 回执或用来放行 Job**；换成合格 RPC 前须再验历史块、日志、重组窗口和链身份。只读链上查询当前 Job signer `0x4434c744708656783D42ea6463A1fe52Dc8F78a2` 的 latest/pending nonce 都是 0；文档中的 operator 地址为 12/12。对生产 MySQL UUID `78dbed8b-77a9-11f1-ac1c-06eab0296b63` 的只读事务查询，两个地址在 `chain_relayer_nonce` 及 `chain_tx_submission` 均无记录；这不证明实际 Core signer 的 nonce 归属，也不预留 Job lane。Job 独立配置尚无 DB/Redis 凭据，不能直接复制 Backend 完整 secret（其账号范围、权限及 Redis 拓扑需单独审查）。Job Compose 服务/容器仍为 0，Backend healthy。未安装 Agent/Backend 候选、未签回执、未启 Job。

> **2026-10-03 后续状态（下文保留当时只读盘点，不再代表当前状态）：** 应用机 `18.143.209.192` 的 ED25519 指纹已由用户经独立渠道确认。生产 MySQL UUID `78dbed8b-77a9-11f1-ac1c-06eab0296b63` 的当前备份已完成隔离恢复检查；经分别授权，仅执行 10-02 signing-metadata 迁移，五列和 `applied` ledger 已只读核对。备份及执行记录位于操作方私有目录 `/home/la956/.joincare-production-backups/20261003T130727Z-ido-job-preflight`，不应复制进本仓库。`4fb0c3d` 的 Agent 候选不再要求生产主机提供 Core 私钥：从独立 Job 私钥推导地址，公开 Core operator 仍须由签名者及链上证据独立核验。用户更换 Job 私钥后，只读计算出的**现用** Job 公开地址为 `0x4434c744708656783D42ea6463A1fe52Dc8F78a2`，旧地址 `0xE4EFa2D82b34c09172567dE2BD79FDf0C9BfAb7C` 不可复用。应用机只读 BSC 查询得到 chain ID 56、文档所列 proxy `0x962256B9f09Dfd6f2C20891FeDb5163Dd03183C1` 的非空代码、ERC1967 implementation `0xff83bff37f7a51ad9307bd17553965a12251a431`、当前 `operator()` `0xfd712061ee08e070a930ecd45e935ae5d7f5445c`；implementation 代码 Keccak-256 为 `0x4852dfab98106ea376d1521ac16843c265a166f2f8153d36e3a646d68d12c256`。这些是公共 RPC 的时间点观察，不是已批准的运行配置或历史数据能力证明；该公共 RPC 的近期 `eth_getLogs` 返回 `-32005 limit exceeded`。生产 Job 的 RPC、V2 起始区块、合约和 operator 字段尚未写入独立配置，Job Compose 服务／容器和 Agent 受信密钥仍不存在；Backend healthy。**仍为 NO-GO**：需生产 RPC 历史能力、起始区块和 Core 控制权/nonce/outbox、DB/Redis 与 Compose 权限、真实回执审查，并分别授权 Server/Agent/Backend 升级与 Job 首次启动。

**结果：BLOCKED / production NO-GO。** 本记录是只读证据盘点，不是发布、迁移、资金操作或审批授权。第一阶段 `jc-release` 本地候选的通过记录不能代替生产证据；不要将测试公钥、测试 signer、测试 Compose 或一次性数据库用于生产。

## 已核对（本地，只能证明候选状态）

| 前提 | 本地核对结果 | 尚需独立证据 |
|---|---|---|
| 候选迁移 | `migrations/2026_10_02_chain_tx_submission_signing_metadata.sql` 的 SHA-256 为 `0a0d4a20a36065175aadf17111a7730b473615628d3e88c9eadcd3e9fc9d8bb2`，与后端 `pkg/common/database/schema/chain_tx_signing_metadata.go` 的启动门禁常量一致。 | 生产 MySQL 8 的 server UUID、迁移 ledger、五列实测形状；受控迁移的外部 manifest、备份回执、隔离恢复演练、审批和执行顺序。不能因 checksum 相符而直接执行 SQL。 |
| 应用布局 | 当前仓库 `docker-compose.yml` 是 `services: {}`；无 `job-runtime/config/{job,common,secret}/config.yaml`。 | 应用机实际 Compose 解析结果、Job 专用只读挂载/权限、最小 DB 权限和运行时健康；不能从 Backend 复制配置或凭据。 |
| 发布包 | `tools/jc-release/scripts/upgrade-jc-release.sh` 仍明确只接受 `production-core-services-job-runtime-disabled`。 | 重新审查并制作匹配实际发布边界、可回滚且经授权的 Server/Agent 包；现有旧包不能冒充 Job 候选。 |
| signer | Agent 本地实现从独立 Job 私钥推导地址，公开 Core operator 地址须匹配受信声明及 Job 配置且与 Job 地址不同；未配置生产公钥。 | 授权人员须独立核验 Core signer 控制权、链上 `operator()`、Job signer 保管与 nonce/outbox 槽；仅公开地址相符不能证明 Core 控制权。不要输出或复制 Core 私钥、使用 relayer 私钥做手工交易。 |
| 链与 RPC | 本地隔离 Hardhat、完整测试库的 cron/scanner 证据不能证明 BSC 主网可用。 | 应用机的链 ID、合约/代理地址和代码身份、确认深度、RPC 历史区块/日志查询、失败和重组恢复、DB/容器时区；不得从配置文本推定 RPC 实际能力。 |
| Fair Launch | 后端 `FairLaunchMaterializationEnabled=false`；`fairLaunchCEV2PersistenceEnabled` 继承该常量。 | 继续禁止财务物化和 Fair Launch Job 角色；归档 scanner 的测试通过不解除此禁令。 |

## 本轮生产只读连接结果

- 控制机 `154.26.137.111`：使用已知 SSH 主机密钥、BatchMode 和短连接超时尝试 `root` 身份读取服务/配置**元信息**，公钥认证被拒绝；未获得生产服务状态、备份或数据库证据，未修改远端。
- 应用机 `18.143.209.192`：`ubuntu` 身份尝试时本机缺少经核准的 ED25519 主机密钥，StrictHostKeyChecking 拒绝连接；**没有**使用 `accept-new`、关闭校验或自动信任新指纹，未修改远端。
- 以上结果是访问/主机身份阻断，**不是**“生产配置缺失”的新实测结论；此前应用机无 Job 服务的结论仍仅是先前只读预检的时间点事实。

## 后续只读验收顺序（由具备授权身份者执行并回传脱敏回执）

1. 先通过独立可信渠道确认应用机主机密钥指纹和访问身份；控制机重新建立受限只读访问。记录时间、执行人和主机身份，不在日志中输出配置正文或任何密钥。
2. 验证 Server/Agent 运行二进制哈希、systemd 单元、配置/状态文件权限与 `job_pending` 是否存在；核对发布包来源及可恢复的原二进制。不要仅依据本仓库源码推断已安装版本。
3. 验证数据库备份**可恢复**且其回执与 server UUID、迁移 manifest、目标 schema 及批准单绑定；只读核对 ledger 和列形状。备份/恢复证据不足则停止，不能运行迁移。
4. 核对独立 signer 地址、Job 配置权限与有效 Compose 解析结果；通过不泄露私钥的方式核对链 ID、合约、确认深度、历史 RPC、时区、Job/DB 最小权限和故障告警。
5. 形成缺口清单与签字材料。只有前提齐备，才另行评估受控 Server/Agent 升级、迁移与最终 Job 启动；每个动作分别取得人工授权。TESTCAT #110 的 3819 修复仍单独发布验收，不能代用户审批。

**目前没有生产准入、迁移执行、发布包升级或 Job 启动的证据；第二阶段未通过。**

## 第二阶段本地开发交付（不改变上述生产结论）

Agent 现接入独立的 readiness 双签及六份本机证据回执 SHA-256 校验，绑定 Agent/SSH 主机公钥指纹、发布 ID/制品哈希/镜像 ID/代数和 1002 迁移校验值；缺失/不一致则在下载前拒绝。它**不**验证回执正文真实性、数据库和链的实时状态，也不签发审批。安全边界与缺失的生产证据逐项见 `SECOND-PHASE-REVIEW.md`。构建脚本 `scripts/build-job-review-candidate.sh` 只产出 Server/Agent 离线审阅 ELF 和 checksum，刻意不含可执行升级器、Compose、签名或生产密钥；不能用旧 `production-core-services-job-runtime-disabled` 脚本作 Job 升级。此处只是本地开发交付，仍为 production NO-GO。

本地追加跨进程 Job 锁、变更前二次准入以及六份 receipt 的规范结构与身份/24 小时观察时间检查；Windows 与 WSL 非 root Go 测试/vet 已通过，WSL `sudo` 在 **`TMPDIR=/root` 的隔离 fixture** 上用仅测试用 SSH 公钥完成 Linux-root 准入正反向测试。生产二进制固定读取 `/etc/ssh/ssh_host_ed25519_key.pub`，没有对生产机器运行此测试，也未验证实际主机指纹、当前数据库或主网 RPC。测试不能代替生产审批。

候选审计追加：Linux 对声明及回执在打开时拒绝最终路径符号链接，并核对打开后的 inode 为 root 所有、普通文件、权限不高于 0600，限制读取字节数；WSL 非 root、WSL root 和 Windows Go 回归通过。此前离线审阅 ELF **早于**该改动，不能作为当前源码二进制。路径祖先竞态、root 原地改写、签署人断言与实际 signer/合约 `operator()`/RPC 的真实性仍无法由这些文件校验自动证明；正式候选和独立生产核验仍阻断。

在 WSL 中执行完整 `scripts/tests/fair_launch_local_e2e_schema_static_test.sh`，09-03 至 09-18 的脚本静态检查及 Node specification 均通过（仅本地静态验证，非生产 schema 实测）。

本地回滚审查进一步要求已存在 `jc-job` 的 Docker Compose service/working_dir 标签同时绑定当前部署根目录；Apply、健康检查及回滚也核对该身份。异项目或历史无标签容器不被直接接管。WSL 的 `docker compose config --format json` 在**隔离测试 YAML** 上确认覆盖镜像、只读挂载和 attempt 标签生效，未访问 daemon；**未核验应用机实际 Compose**，也未执行真实 Docker 回滚。

补足本机 Docker daemon 集成回归：`JC_JOB_DOCKER_INTEGRATION=1 go test -tags docker_integration ./internal/strategies -run '^TestJobLocalDockerFirstDeployAndFailedUpgradeRollback$' -count=1 -v` 在 WSL 通过；使用隔离 nginx 健康/失败镜像，执行首次健康部署、旧镜像备份、失败升级后恢复、失败首次部署后按 attempt 标签清理，并确认 `jc-job` 测试容器及测试镜像清除。若本机已有 `jc-job`，测试直接跳过而不接管。此处是**真实本地 Docker 的 mock Job 回滚**，不是生产应用机、真实 IDO Job 或生产数据回滚；前面的历史静态检查描述保留其当时含义。

取消中的命令不再直接把已取消的 context 传给回滚：单独最多 3 分钟的恢复 context 尝试回滚并验证；失败继续保留 `job_pending`。新增取消后成功恢复与不可验证恢复阻断的测试。该逻辑不解决 Agent 进程崩溃后的自动恢复，仍须人工核查。

离线审阅构建脚本现于编译前后对 `tools/jc-release` 已跟踪与未忽略未跟踪文件清单做哈希比对，包内保留 `SOURCE-SHA256SUMS` 与 `SHA256SUMS` 供独立复核。清单不覆盖被忽略文件、Go 工具链来源、仓库其他路径或后端/合约等其他仓库；工作树仍脏，**不是可安装包、正式来源证明或生产授权**。旧审阅 ELF 已过时。完整差异、外部依赖及人工来源审查尚未完成。

再收紧签名身份：从 Job/Core 本地明文 secp256k1 私钥计算以太坊地址，要求与签署声明的 Job/Core 地址及 Job V2 `coreOperatorAddress` 一致；签署的 chain ID、V2 合约地址也须匹配本地 Job 配置；加密或不可推导私钥继续拒绝，反向测试覆盖签署地址错配、配置中途漂移及大小写重复字段。Job 角色校验还要求 chain ID、RPC URL、V2 合约地址及 operator 字段只出现在规范 Job 配置路径，不允许后续 common/secret 重写。Go Windows/WSL 非 root/WSL root、WSL race 测试已通过。**不证明实际容器挂载的密钥或链上 `operator()`**，也未探测生产 RPC/DB；先前离线审阅 ELF 再次过时，仍须重新构建且仅供审阅。

## 续查：既有 WSL SSH 配置的只读预检（同日 UTC；仍为 NO-GO）

上面的 Windows Git Bash SSH 尝试**仍是当时真实失败**；随后发现本机 WSL 有另一套既存 `known_hosts` 和 SSH 身份，`StrictHostKeyChecking=yes`、`BatchMode=yes` 对两台主机连接成功。该命中仅证明匹配 WSL 中既有记录，**尚未取得独立渠道的主机指纹复核**，不能把它写成最终身份授权。本次只做 `stat`、哈希、systemd、Docker inspect/config 和状态字段存在性查询；未传输配置正文、私钥、令牌，未执行升级、迁移或启动容器。以下两台主机的根目录和职责**不同**，不能互相代替。

| 主机 | 脱敏只读结果 | 判定 |
|---|---|---|
| 控制机 `154.26.137.111` (`root`, `vmi2870755`) | `jc-release-server`、`jc-agent` 均 active，运行与磁盘二进制哈希分别为 `d99b4a444d5bb36739ee1fba5a2f153a229432ba8dc68ee1f39edaa909e8e7e5` 和 `b012c65e8baf94a931ff8bff6ff68205222674e038723ecd43e5d851c9dcb3f0`；Agent `root=/opt/joincare-test`，配置无 `job_environment`/`job_trusted_keys`，状态无 `job_pending`/`job_generation` 字段但有历史 `job` deployed 项。其 Compose 列出 `job`，`jc-job` 容器已 exited（2026-09-16 创建），无 Compose service 标签；`/opt/joincare-test/job-runtime/config` 三文件缺失，Compose 文件元信息 mode 777、UNKNOWN 所有者。 | 历史 Job 记录**不是**新候选的独立 IDO Job 资格或回滚目标；旧状态不可自动清理。需查明旧 Job 身份/残留及控制机目录权限与配置来源。 |
| 应用机 `18.143.209.192` (`ubuntu`, `ip-172-26-10-135`) | `jc-agent` active、二进制 SHA-256 `8670aa3ade2c2bc9a51ae108d803e498391102910494618e7bfc004593d4b677`；Agent `root=/opt/joincare-prod`，配置无两项 Job trust 字段，状态无 Job pending/generation/deployed 项。生产 Compose 解析服务为 `redis`, `redis-init`, `backend`, `admin`, `frontend`, `tgbot`；无 Job 服务/容器，`job-runtime/config` 缺失；`jc-backend` 为 running/healthy，镜像 ID `sha256:e08179f5d572a6dd9139a89a1b7eb03f556a00e72d921d235e2d4cd9ef1e955a`。 | 生产应用机当前不满足 Job 静态布局或受信准入；不能用控制机历史 Job 代替。 |
| 应用机备份目录 | `/var/backups/joincare` 有 2026-07-08 的约 275 MB 压缩包和 `.sha256`；本轮未见同目录 restore/receipt 文件；备份文件 mode 644。 | 仅证明旧归档存在，不证明当前 DB 的合格备份、恢复演练、回滚可行性或数据机权限；其可读权限需安全审查。**没有**打开备份正文或取出 DB 凭据。 |

控制机与应用机 Agent 二进制哈希不同；不能假定均是第一阶段本地候选，亦不能只升级其中一个后声称 Server/Agent 协同通过。本轮未连接/查询生产 MySQL，未验证迁移 ledger、五列形状、DB TZ、最小权限、signer 地址、链 ID/合约或历史 RPC。继续之前须由操作方确认可信主机指纹及哪台机器承担发布 Agent，提供当前 DB 的备份与**隔离恢复**回执、受限数据库只读查询授权，并审查旧 Job 状态；任何生产写操作仍需单独授权。

## 再次只读核验：身份待复核，备份/发布目标未放行

- WSL 已缓存的 ED25519 SSH 指纹为：控制机 `SHA256:L6leP77U5TrKW04QP1jtYTA7pBEq2/DPJ5peRu81ZCQ`，应用机 `SHA256:W1U/noXg5EAbUL4N1k0IT/57udTCXcnvlk5Oxi+FD5E`。这是**本地 known_hosts 的值**，不是独立可信渠道验证的主机真实指纹；必须请基础设施负责人用另一渠道确认后再用作生产身份依据。
- 应用机 Agent 的运行进程 `/proc/<MainPID>/exe` 哈希与磁盘 `jc-agent` 哈希一致（`8670aa3ade2c2bc9a51ae108d803e498391102910494618e7bfc004593d4b677`）；控制机的两个服务也各自运行与磁盘相同的二进制。但这些哈希**不能证明**运行版本包含第一阶段未提交的本地候选代码；应用机根路径仍为 `/opt/joincare-prod`，控制机根路径仍为 `/opt/joincare-test`。
- 应用机已知备份目录仅有 2026-07-08 的归档与校验文件，该归档权限为 `0644 root:root`；已知目录内未发现恢复/回执文件，常用路径范围内未发现迁移 manifest/restore receipt。**搜索范围有限**，不能断言全系统绝无回执，也不能把旧归档判为当前备份。已核对元信息和归档 SHA-256，未读取归档或访问数据库。生产 MySQL 的 server UUID、迁移 ledger 和列形状仍**未测**。
- 控制机旧 `jc-job` 目前 `exited`、没有 Compose service 标签，状态文件中仍有历史 Job deployed 项和指向旧备份的路径；这些旧备份是 JSON 容器元数据，**不是**候选 Job 的镜像 `docker save` 回滚制品。未删除容器、状态或备份。应用机没有 Job deployed 项。

**下一道人工/证据门禁**：① 负责人从独立渠道确认上述指纹与目标主机；② 明确唯一应用机 Agent 和控制机职责；③ 提供当前数据库、server UUID 绑定且隔离恢复通过的备份回执及最小权限只读数据库查询授权；④ 审查控制机历史 Job 残留和将来的包来源/回滚；⑤ 再核验 signer、链/RPC、时区和 Job 专用配置。任一缺口未闭合，不制作或执行生产迁移/Job 启动决定。

## 同轮后续：离线权限边界与备份校验（不改变生产）

- 重新核对当前后端源码 `pkg/job/chain/chain.go`：IDO V2 Job 已要求显式 `chain.project.v2.coreOperatorAddress` 与链上 `operator()` 一致、**不得等于 Job signer**；合约 `JoinCareIDOv2.sol:endProject` 不要求 operator。先前“单一 operator 与独立 Job 签名者不可并存”的观察适用于旧实现，**不应套用当前候选**。但这只修正了代码层冲突，不证明生产链上的 operator、独立 signer 或 nonce lane 已核对；仍须实际核验。普通 IDO `starter.Chain()` 同时注册其他 reorg、恢复、TGE 和对账任务，绝不可把进程等同于只读 scanner。
- 应用机旧压缩备份的 `.sha256` sidecar 用 `sha256sum -c --status` 只读核对通过；它是 2026-07-08 归档，**不是**当前数据库的恢复演练或可用于 1002 迁移的回执。未读取备份内容或数据库。
- 本地 Job Compose 布局检查原先用带少数字段的 struct 解码，其余字段被 YAML 解码器忽略，可能放过 `privileged`、`network_mode`、`cap_add`、`build`、额外 labels 及 YAML 合并继承。已在 `internal/strategies/ido_admission.go` 增加解析后的 Job 字段白名单，并在测试中拒绝上述注入；`go test ./... -count=1`、`go vet ./...` 与 Fair Launch 静态套件均通过（静态日志 `D:/pi-tmp/jc-release-static-second-phase.log`）。这是**本地候选**加固，不会自动更改生产已装 Agent，且不能替代生产实际 `docker compose config` 验证。
- 随后收紧 Job 健康探针：原先只查命令字符串包含 `/health/job-ready`，可被 `CMD-SHELL` 或附加命令伪装；现在要求精确 `CMD curl --fail http://127.0.0.1:13007/health/job-ready`，拒绝未知 healthcheck 字段（包括 `disable`）和非正 `retries`。增加反向测试，完整 Go 测试／vet 及 Fair Launch 静态测试通过（`D:/pi-tmp/jc-release-static-second-phase-health.log`）。此处仍是本地候选，**未**证明应用机有该健康探针，也未开放生产 Job。
