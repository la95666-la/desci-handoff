# JoinCare IDO V3 白名单开发计划（独立新合约）

状态：开发中，待产品／安全评审；**不授权生产数据库写入、合约部署、上传或发布**。跨仓库工作项写在本分支，具体实现状态以各仓库为准。

> 新的只读负例增量：Backend `9a750ae` 对 `getProject` 的两段动态 vesting 数组先验证字节长度再索引，并枚举所有合法 tuple 的严格前缀，确保畸形截断 RPC 返回正常拒绝而非 panic；`8abdba4` 将同块项目累计募资／累计售出值与成功购买日志的单笔金额比较，防止单笔超过同块读回总额，允许同块其他购买令总额更高。七个相关 Go 包测试和 vet 通过。累计值只是瞬时历史块观察，不是财务账本或持续 reorg 保证；仍不得开放 V3 扫描、proof 或认购。

> 进度（M1 局部原型，未完成门禁）：智能合约仓库已从 `feature/fair-launch-development` 新建本地 `feature/ido-v3-whitelist`，新增独立 `JoinCareIDOv3` 和仅用于本地验证的 2-of-3 Governor Mock。原型以合约身份为 owner／策略 governor，独立 Proxy、按项目 root/version、Active 模式冻结、两种购买入口和事件均已编写并通过目标测试；本地 Merkle 构建器、跨语言黄金向量及奇数叶链上 proof 测试见合约仓库 `docs/IDO-V3-WHITELIST-VECTOR-SPEC.md`。生产 Safe 地址／阈值、编译审计、全量生命周期、Core/Job/DB/Admin/Frontend 仍未完成。**Mock 不能部署为生产治理合约，V3 不可上线。**

> 进度（M2 的安全基础组件，**未接入正式业务**）：Backend 仓库本地分支 `feature/ido-v3-whitelist` 新增 `pkg/common/idov3whitelist`，逐字节复验 Solidity／JS 共用的 1、2、3、5 地址向量（空 proof、奇数叶晋升），并加入独立 `ido_v3_stablecoin` 项目身份断言；没有迁移、API、Core/Job 路由或白名单资格服务。Backend 原有 `pkg/notice` 工作区改动保持原样。

> 进度（只读快照基础，**尚未接入链上资格接口**）：Backend `pkg/common/idov3whitelist/policy_reader.go` 提供无签名／广播能力的 V3 固定块策略读取，要求调用方给出预期 chainId、Proxy、项目 ID、固定块哈希、预先批准的 Proxy 与 implementation code hash 和 implementation 地址；读回 EIP-1967 implementation 槽、`contractVersion()` 和严格 ABI 的 `getWhitelistPolicy`，读后复验块哈希，错误脱敏。调用方**仍须独立证明确认深度／重组策略、四眼审批的完整成员快照及版本、钱包绑定**；仅固定块读回与 root 相等不足以向用户发 proof。不能用这里的读取结果作为 V3 上线准入。

> 进度（M0 成员快照候选，**并非审批**）：合约仓库 `test/fixtures/ido-v3-snapshot-v1.json` 与 Backend `pkg/common/idov3whitelist/testdata/ido-v3-snapshot-v1.json` 使用相同黄金向量，按完整排序叶集合、root、目标版本与 chainId／Proxy／项目身份生成不可歧义的候选摘要。该摘要尚未绑定 Admin 独立审批身份、oldVersion、链上收据／finality 或不可变存储；它不可单独触发策略写入或向钱包发放 proof。

> 进度（本地收据匹配基础，**不是上线准入**）：Backend `pkg/common/idov3whitelist/receipt.go` 可对**由上层独立取得**的成功交易收据，核对 txHash、blockHash／高度、V3 Proxy 的项目级 `WhitelistPolicyUpdated` 事件、目标版本与 root，并与同块固定读回的策略及成员快照候选摘要比对；合约目标测试锁定事件 ABI。该纯函数不能证明收据真伪、Admin 四眼授权、完整名单的持久化、链上最终性或重组恢复；上层链事件／outbox／审批闭环完成前仍不得向钱包发放 proof。

> 进度（只读确认高度候选，**非最终性证书**）：Backend `pkg/common/idov3whitelist/evidence.go` 将受信 RPC 返回的交易收据、同块策略／事件、确认深度、确认块当前策略及区块哈希读回复核组合为瞬时证据。调用方仍须从审批记录提供准确 txHash、身份配置及四眼快照，设定经审查的链特定确认阈值，并对后续 reorg／版本变化重新核对；不能把一次读取直接标为 `effective` 或发放 proof。无生产 RPC、DB 或 Job 接线。

> 进度（M2／M4 局部增量，**非资格上线**）：Backend `2c2f32f` 将精确 V3 身份只用于独立的只读项目投影，同时拒绝经旧 create／activate 路径写入；`e487f91` 新增对 Pending／Active 版本和模式冻结的离线变更预览（不是审批或签名）。Web `534df68` 识别精确 V3 身份用于只读展示，但不调用 V2 合约／授权／认购／领取／退款，未有治理和 proof 服务前显示不可认购。Deploy 的**独立干净 worktree** `feature/ido-v3-whitelist` 自 `780d31b` 增加受控迁移门禁及 V3 独立身份／候选快照／完整成员／执行观察数据表；后续触发器、限制和测试见下段。原 Deploy 仓库其他未提交内容未触碰。仍需完整不可变成员批准、真实治理执行、Core／Job／outbox／reorg、用户 proof、安全审查与 M5 验收；不得据此开启 V3 认购。

> 后续安全增量（仍非 M2–M5 完成）：Deploy `316e847` 的独立 worktree 在基础表添加 V2 Proxy 不可复用、精确项目绑定及身份只读触发器，`state` 限定为 `unverified`／`candidate`／`awaiting_governance`，不可伪称 `effective`；本地隔离 MySQL 测试通过，**未对生产数据库执行迁移**。Backend `6d66dfe` 只验证调用方提供的完整快照与双人 ID／链观察的一致性，无法证明 Admin 身份或收据真实性；`8b20a59`、`4f1348e` 隔离 V3 与旧 `project_whitelists` 自登记；`ac4577d` 禁止新的 V3 outbox action 越过未完成的 materializer；`434dd5f` 只提供独立策略日志解码。Web `c24f0db`、`778420e` 的列表／详情不再把报名数字或 Live 文案表达为 V3 认购资格。Admin 独立分支 `feature/ido-v3-whitelist`／`4f712c6e` 仅显示精确 V3 身份并封锁 V2 操作及财务选项。合约 `f6fdc05` 增加 Pending 关闭后 Active 冻结回归。**治理选型、真实四眼执行、不可变审批状态机、V3 生命周期多源扫描／资金账本／reorg、资格 API 与钱包认购仍未实现；不得开放 V3。**

> 历史失败记录（后续重跑结果见下文）：V3＋V2 目标 Hardhat 套件曾为 48 passing；旧 `FairLaunchSafeExecutor TEST ONLY` 在专属配置单独运行 2 passing。当时默认 root 测试为 185 passing、17 pending、1 failing（旧 Architecture authority 断言），local-e2e 全套为 22 passing、6 failing。随后同一分支重跑为默认 186 passing／17 pending、local-e2e 28 passing／0 failing；**不能推断历史失败根因已经修复，也不能宣称治理、财务或资格闭环已完成**。Web 全套 233 passing（修正 Windows 测试路径大小写导致的 Vite HTML proxy fixture 错误，`a009bd3`）；Admin 项目谓词 19 passing 且本项目 vue-tsc 无报错。

> 持续开发提交：Deploy `4ed340b`／`63bb286`／`80ffb07` 将基础候选快照、成员及执行观察限制为只追加，要求规范小写链身份和成员编码；经 WSL 中 `--network none` 的 MySQL 8.4.10 一次性容器验证正反例，**这不是完整生产 schema、事务并发或迁移许可**。Backend `5f17a7f` 增加候选策略事件与执行收据的单日志匹配，排斥重复、失败和孤块，但它仍不证明审批、最终性或赋予 Job materialization／proof 服务能力。两个提交组均不改变 V2 运行时，不上线 V3。

> 最新增量：Deploy `38cd1f6` 以单独门禁迁移允许项目表精确的 `capped_presale`／`ido_v3_stablecoin` 组合，隔离 MySQL 仅验证了部分基线形状；Backend `9479b79` 对调用方提供的候选成员行做完整、连续、规范编码及 commitment 一致性校验，**未认证数据来源**。Backend `e76125f` 封锁 V3 在通用 Admin 内容管理中的 lifecycle status、归档和恢复（即使尚未 onchain_locked）；定向 Go 测试与 vet 通过，MySQL 集成用例因未配置一次性测试库而跳过。前述均非四眼审批、治理最终性、财务 materialization 或 proof 许可。

> 后续核验：Backend `469a936` 拒绝 V3 经旧 minimum-purchase 接口以空值清除 V2 策略，`1c9fffb` 将通用生命周期错误改为 chain-managed 文案；Admin controller 包本地 Go 测试和 vet 通过（无测试 MySQL DSN）。合约默认配置显式运行 `npx hardhat test test/*.test.js` 此次 **186 passing／17 pending／0 failing**，旧 Architecture 单文件 14 passing、与 V3 文件组合 33 passing；原记录的 185／17／1 失败未在此次复现，不能据此推断先前失败根因已经修复。专属配置 `npm run test:fair-launch:local-e2e -- --no-compile` 此次 **28 passing／0 failing**（依赖已有编译工件），旧 22／6 失败亦未复现；两种本地套件的通过不等同治理、独立审计或 M5 验收。

> Backend `8a93b3f` 进一步禁止通用 Admin 内容编辑接口添加、删除或变更 `onchain_json.idoV3` 与 `onchain_json.ido_v3` 命名空间（保留原 Fair Launch 保护及营销字段编辑）；该 JSON 本身仍不是 V3 权威审批／资格来源。controller 包测试／vet 通过，既有无关 notice 工作区改动未触碰。

> Backend `2e7a12f` 在现有审批服务的提交／复核入口预留并拒绝 `ido_v3.`、`ido_v3_`、`v3.`、`v3_` 动作，阻止未来仅注册 handler 就误将候选证据当四眼审批流程；测试／vet 通过，未创建 Admin V3 可用工作流。后续完成可信状态机、权限／MFA、快照与链上治理审查后才可按明确条件移除该门禁。

> Backend `c18b3f6` 新增 `pkg/core/service/idov3candidate/AuditCandidate`：在 MySQL REPEATABLE READ 只读事务中按完整 V3 项目／binding／未验证 runtime 身份查询候选及全部成员，复算规范 commitment；对数据库错误返回脱敏失败，仅返回摘要而非树或 proof。sqlmock 正反例及 Go vet 通过，尚未运行一次性**完整** MySQL schema 集成／并发测试；基础表仍无可信审批状态变更，结果**不能**授权交易或资格发放。

> Backend `a0688c4` 在上述只读审计中增加从持久行读取 `expected_version` 并要求无溢出的 `target_version=expected_version+1`，负例覆盖非连续版本；仍未校验链上当前版本，也不授予审批或 proof 权限。

> Backend `e2fc87d` 将精确 V3 身份纳入 V2 create／activate／end／cancel／平台费 withdraw 的负例测试，证明未进入旧 relayer 路径；仅补充隔离证据，不代表 V3 对应动作已有实现。

> Backend `c4028ca` 覆盖候选 DB 读取错误脱敏测试；`6cb1419` 进一步要求旧 minimum-purchase 接口在**清空或设置**时均先验证持久化 `capped_presale`／`v2_stablecoin` 精确身份，杜绝空值绕过及错配模式。这仍不构成 V3 最低额策略，V3 项目继续拒绝旧接口。

> 后续 Backend 增量 `3db28c2`／`7376b39`／`fb9ce0e`／`90c39d2`／`d07a567`／`f988233`：公开读模型读取并核验持久化 V3 模式／版本，不把 Warmup 自登记数展示为 V3 成员；增加与 V2 **同签名 topic** 的 V3 Purchased 专属解码、精确项目／代币／成功收据匹配及瞬时规范块／确认高度读回；Job 可校验**禁用态**独立 V3 shadow 配置但不加入 V1/V2 扫描地址；共享 outbox／gateway 负例证明 V3 仍在签名、广播和收据前被门禁拒绝。相关 Go 包的定向测试／vet 通过，未连接生产链／库。以上不是可信 runtime admission、完整 schema／并发演练、持久财务 materialization、持续 reorg 或 proof 权限，M2–M5 仍未完成。

> 候选审计隔离演练（仍非生产资格验收）：Backend `53c314a`／`7546717`／`db8ad6c` 新增可选的 MySQL 8.4 测试，使用只读账号与 REPEATABLE READ 事务验证 `AuditCandidate`：缺失成员拒绝、查询中另一连接提交新成员也不能跨越快照、下一事务才可看到完整成员、错误 chain／project／Proxy／snapshot 拒绝；追加一份 SQL 约束允许、但成员摘要与完整成员不匹配的候选，证明审计会复算并拒绝，旧候选不受影响。Deploy `1becba8`／`6cfa95b`／`ebafc72`／`cefa58d`／`af4a622`／`8e98826`／`15d594b` 在 WSL `--network none` 一次性容器内，校验并装载 pre-0903 基线及完整原有表／迁移账本，顺序回放 0903–1015 现存迁移（0908–0912 本地门禁、0913–0918 主网命名 DDL 的**空表**本地门禁、1014／1015 V3），并将审计账号的 SELECT 仅授予五张必需表；测试确认不能 UPDATE 或读取 Warmup 自登记／Admin 审批／执行表，最后读回 runtime 仍未验证、两份候选仍未批准且无执行记录。另在迁移前插入 V2 身份行，确认 V3 不能复用其 Proxy，迁移和审计后 V2 身份及原始募资值仍不变。密码不出容器，未连接生产库。**这是一次性全表形状／合成 V2 行／空库并发和权限演练，不是生产数据分布、现网 migration ledger、真实 Admin 四眼／治理、V3 生命周期／财务／reorg 或 proof 验收；M2/M3 尚未完成。**

> 治理身份只读观察增量：Backend `a96fe1a`／`116a877` 在 `pkg/common/idov3whitelist/ReadGovernanceAt` 增加固定区块的 `owner()`／`whitelistGovernor()`、双方合约代码哈希、V3 Proxy／implementation／版本及区块哈希一致性检查，错误对外脱敏；返回的治理观察不包含 Policy、Tree 或 Proof，避免误作资格来源。通过模拟 RPC 正反例与定向 Go 测试／vet；不签名、不广播、不接收本地 Governor Mock 为可信生产治理，也不认证签名人、阈值、四眼审批或发放 proof。治理审查与独立最终性／持续 reorg 仍待完成。

> 项目身份只读观察增量：Backend `4a634cf`／`ff94c65`／`cceed43` 的 `ReadProjectIdentityAt` 在严格 V3 runtime／固定区块读取后，对 `getProject(uint256)` 的项目 ID、支付／销售代币、状态、规范 ABI、募资上限／已募集与已出售约束、时间窗及动态 vesting 时间／比例做负例验证，并二次读回区块哈希；观察携带链 ID／Proxy，纯内存 `MatchProjectIdentityObservation` 可与调用方独立提供的完整项目／代币绑定逐字段比较，不返回认购或 proof。**匹配器本身不能认证该绑定来自可信数据库或 runtime 审批**，也没有注册 V3 scanner、写财务账本或持续处理 reorg；不能凭该读回开放认购。

> 购买证据只读交叉核验：Backend `115d6c4` 的 `MatchV3PurchaseProjectObservation` 要求成功 V3 收据中的唯一购买日志与项目读回处于同一 block number／hash，chain／Proxy／projectId／支付及销售代币逐字段一致，允许本笔购买到达硬顶后读回 `Successful`，拒绝 Pending／失败／取消等矛盾状态。调用方仍必须独立认证持久化绑定及 runtime、核验规范链与持续 reorg；此匹配器**不注册扫描器，不写账本或资格**。

> 购买证据组合读回增量：Backend `dd0c280` 增加只读 `ReconcileV3PurchaseProjectEvidence`，先做既有 V3 收据／规范区块及确认高度瞬时核验，再在收据区块通过独立 V3 runtime 校验读取项目 tuple、比较调用方提供的完整绑定，并在全部历史读回后复核收据块与确认头。模拟 RPC 正反例通过；结果仍可能随后 reorg，不证明调用方绑定来源可信，不注册 Job scanner 或写任何购买／资格事实。

> ABI 独立正例：Backend `24d4dd6` 除手写 tuple 负例外，增加使用 go-ethereum ABI 编码器、按 V3 `getProject` 完整 23 字段及两个动态数组签名生成的多期 vesting 项目结果，并由 `ReadProjectIdentityAt` 解码通过；七个相关 Go 包定向测试／vet 通过。此例只是编码兼容性，尚非链上真实项目或端到端扫描验收。

> 新一轮兼容性与状态负例：Backend `b559aec` 令 V3 项目固定区块读回拒绝 Pending 带募集额、Successful 未达软顶、Failed 已达软顶；也验证四类合法状态，七个相关 Go 包测试／vet 通过。合约 `61241cb` 在现有 Hardhat V3 测试中锁定 `getProject(uint256)` 返回 tuple 23 字段次序／类型及 `owner()`／`whitelistGovernor()` selector，单文件 `--no-compile` 20 passing。测试依赖当前编译工件；这些检查不能替代正式 ABI 冻结、跨仓库部署工件审计或生产验收。

> 瞬时收据防竞态：Backend `7ff2941` 在组合购买／项目历史读回之后重新拉取同一 txHash 收据并匹配唯一 V3 购买日志，再复查收据块与确认头；第二次收据失败、状态变化或 RPC 原始错误均拒绝且脱敏。七个相关 Go 包定向测试／vet 通过。它不替代持续 reorg 监控、持久扫描或财务 materialization。

> 跨仓库本地 ABI 门禁：Backend `3f249e5` 在显式提供 `JOINCARE_IDO_V3_ARTIFACT` 时读取编译出的 `JoinCareIDOv3.json`，核对 `getProject` 完整 tuple／`getWhitelistPolicy`／owner／governor／version／`purchaseWithProof` 签名与可变性；模拟 tuple 顺序、proof 版本类型、治理 getter 漂移均 fail-closed。普通 Backend 测试未提供 artifact 时跳过此可选检查；它验证本地编译工件而非经审计生产部署字节码。

## 0. 基线、范围与成功标准

- **切换前关闭 V2 业务入口，上线后 V3 是唯一新 IDO 业务路径**。产品方确认目前 V2 仅有少量内测项目、没有实际 IDO 业务；V2 内测项目如有链上资产，由项目方自行通过原 V2 合约处理，不把遗留核对、清算、迁移或后台收尾能力列为 V3 升级／上线前置条件。V2 Proxy 不升级，既有项目／资金不迁移，历史数据不被 V3 重解释，也不追溯启用白名单。
- **新建独立 `JoinCareIDOv3` implementation + 新 Proxy 地址**；项目在未创建上链前选择 V3，并永久绑定 `contract_version=ido_v3_stablecoin`、chainId、Proxy 和经验证的实现身份。`onchain_locked=1` 后不得修改版本或目标合约；已有 Setup／Set 草稿是否转选 V3 须经过单独校验，已创建的 Pending／Active V2 项目一律不能转选。
- V3 仍是 capped-presale USDT／USDC，保留 V2 的财务语义与独立的结算、退款、领取；白名单只限制认购资格，不能改变既有持仓、已募集资金或追索权益。合同复用 V2 逻辑要做独立安全审计与回归，而不是假定复制即可上线。
- 验收目标：直调 V3 合约也不能绕过已启用白名单；Active 后启用模式不可从关变开、也不可从开变关；原已启用项目可在四眼治理后增删成员、只影响后续交易；V2 全部历史行为不受影响。

### 状态机（以链上状态为准）

| V3 项目链上状态 | 白名单关闭 | 白名单开启 |
| --- | --- | --- |
| 尚未创建 | 后台配置草案，不产生链上资格 | 后台配置初始成员草案，需四眼审批才可正式应用 |
| Pending | 可经四眼审批开启／维持关闭 | 可经四眼审批更新成员或关闭；变更必须链上生效后才可激活 |
| Active（含尚未到认购开始时间） | 禁止开启或重新开启 | 禁止关闭；允许经四眼审批增删成员并链上生效 |
| Successful／Failed／Cancelled | 只读归档 | 只读归档；既有购买仍可依规则领取／退款 |

**冻结点是链上 `activateProject` 成功导致的 Active，而不是 `startTime` 或仅有 Admin 按钮点击。** 所有白名单写入（包括首次开启、Pending 关闭、运行时增删）均需四眼；纯预览不需要。Active 成员变更使用交易执行时最新生效版本；历史购买永不因移除地址被撤销。若产品希望创建时冻结模式或禁止 Pending 关闭，需要变更状态机和测试，不允许仅改前端提示。

## 1. 已核对的实施障碍与设计门禁

1. V2 的 `purchase(uint256,uint256)` 没有白名单检查；Warmup `project_whitelists` 为用户自行加入、默认 `approved=1` 的互动表，不是认购准入名单。V3 须建独立项目策略、成员与版本数据，**不得自动导入报名用户**。
2. Backend `pkg/core/service/project/payment_token.go` 信任 `chain.project.v2.*` 的**单一**地址／实现／支付币配置；`v2_relayer.go` 多处使用 `ChainProjectV2`；`onchain_params.go` 将 Set 固定为 `v2_stablecoin`。不得把 V3 地址塞进 V2 配置字段。
3. Job `pkg/job/chain/chain.go` 当前校验 V2 实现并只将一个 V2 地址放入 V2 扫描源；事件、身份校验、reorg/snapshot、组合报表多处硬编码 V2 版本。新项目不能仅靠新增 ABI 和前端路由使用 V3。
4. Frontend `src/utils/fairLaunch.js` 的 capped-presale 路由、`src/services/projectContext.js`、`src/services/contract.js` 的 ABI 和 `purchase()` 编码目前只认识 V2；未知版本必须继续 fail-closed，不得落回 legacy/V2 合约。
5. Admin 已有双人审批、权限再检查与审计，但只覆盖后台流程。若 V3 owner／operator 单签能调用策略函数，**链上四眼可被绕过**；独立 V3 的优势是可在首次部署前把治理设计好。新部署不自动解决密钥治理问题。

### 合约治理选择（M0 冻结）

**优先建议 V3 将策略管理交给专用多签治理执行器（例如 2-of-N Safe），而将日常 create／activate relayer 与策略管理权分离。** Admin 发起与不同 Admin 审批负责业务授权；多签的两位独立签名人和真实执行负责链上授权；二者身份和流程必须明确映射，不把两名 Admin 登录等同于两份链上签名。若希望由 relayer 广播，可改为合约原生 EIP-712 双签许可：固定两个独立签名者／角色、权限更替机制、chainId、V3 Proxy、projectId、动作、oldVersion、newRoot、newEnabled、nonce、deadline，合约验证签名并消费 nonce。两种方案须择一并做威胁模型；**不得增加 owner/operator 单签的备用白名单写入口**。若采用 UUPS，Proxy upgrade 权限也必须受独立治理控制；否则不能声称链上策略无法被单方改写。Owner／Operator／Safe 具体身份、密钥保管、轮换、升级与紧急暂停模型确认前，不得签署部署准入。

## 2. 分期交付与验收

### M0 — 决策／基线／架构评审（先于业务编码）

> M0 审查包已形成：[`IDO-V3-M0-ARCHITECTURE-DECISION.md`](./IDO-V3-M0-ARCHITECTURE-DECISION.md) 给出双 Safe 治理设计候选、权限/状态转移、风险表、序列、测试矩阵、无密钥身份快照模板和具名签核条件。**尚无产品／安全／财务／运维批准，M0 不得标完成；其中 Safe 阈值、签名人保管、上链身份及链特定确认深度仍为待核对值。**此包不解除 V3 审批、outbox、扫描、proof 和钱包入口门禁。

- 锁定 `contract_version=ido_v3_stablecoin`、独立 V3 Proxy 与新项目入口；切换前关闭 V2 新业务入口，不以 V2 内测项目清单作为 V3 开发或发布门禁。确定初始白名单在「创建时一并设置」还是「创建后 Pending 设置」：推荐 Pending 独立交易，激活必须有链上确认，避免修改 V2 create 接口与 Set 审批语义。
- 确定治理方案：多签执行还是 EIP-712 双签经 relayer；是否允许 Active 删除成员、批量上限、频率限制、生效窗口、Pending 能否关闭；当前计划默认允许 Active 增删、Pending 可关闭、没有额外时间锁，但均需审批。
- 固定 Merkle 规范：叶节点域绑定 chainId、V3 Proxy、projectId、规范化钱包地址、schemaVersion；哈希配对／排序、奇数叶、单叶、空名单与 duplicate 规则，Solidity/Go/JS 共用黄金向量。proof 不增加每钱包额度；额度仍受 `maxPerUser` 控制。
- 明确白名单治理费用承担、确认深度、回执／reorg 容忍度、签名密钥或 Safe 托管；完成财务和安全审查。交付：设计文档、状态转移表、序列图、测试矩阵与可复现的身份快照（不含密钥）。

### M1 — V3 新合约／本地隔离验证（智能合约仓库）

- 从**固定的 V2 逻辑基线**构建新 `JoinCareIDOv3`，只将功能部署在新的 Proxy。保留独立的支付资产允许列表、费率／接收方快照、硬／软顶、TGE、claim/refund 和资金守恒规则；为 V3 建明确版本标识／验证接口，给运行时身份核验使用。
- 实现项目白名单 `enabled + root + monotonically increasing version`、只读策略接口、版本事件与错误码。建议公开项目保留 `purchase(uint256,uint256)`；白名单项目增加 `purchaseWithProof(projectId,amount,proof)`，旧入口对已启用项目必须拒绝；proof 入口对公开项目按明确约定拒绝或等价执行，绝不形成不同限额逻辑。`msg.sender` 为认证地址；claim/refund 不再次检查白名单。
- 合约在 Pending 允许经治理变更模式／root；Active 只允许“开启且保持开启”的 root 变化；禁止 Active 首开／关闭及终态更新。治理输入绑定旧版本并消费防重放 nonce，拒绝空 root、无效成员证明和状态竞态。`activateProject` 在链上复验策略模式／root 自洽，不能仅依赖 Backend 预检。
- 测试：多项目／跨合约／跨链 proof 与签名重放、旧购买入口绕过、非法／过期 proof、根更新前后夹块交易、模式非法转移、余额／手续费／资金守恒、V2 既有合约回归。提交可复现编译、ABI、storage layout、构建摘要、安全审计问题清单。**此里程碑不部署主网。**

### M2 — 项目身份与多合约基础设施（Backend／Job／数据库／deploy）

> Backend `e6be7bf`／`c123e93`：扩展 **仅诊断、仍为 unverified** 的精确项目/runtime 只读联合审计，将 `governor_address`／`start_block` 纳入结果及严格校验；新增全字段独立预期值对比，拒绝错链、错 Proxy／implementation／governor、起始块或代码哈希漂移，且只返回错误、不产生可信绑定或扫描凭证。七个相关 Go 包定向测试／vet 及隔离 `--network none` MySQL 8.4 顺序迁移和候选审计演练通过。它不认证预期值来源、签核、Safe 权限或链上部署，**不解除 V3 Job／outbox／approval／proof 门禁，M2 代码与测试仍未完成**。Hero 包已部署且正常仅是用户反馈，不作为 V3 发布授权或独立部署核验。

> Backend `8982d10` 修复 `v3_recovery.go` 在生产源码中误依赖 `_test.go` 内 `v3LowerHash` 的编译问题，改为本文件标准库 hash 规范化；`GOEXPERIMENT=jsonv2 go test`／`go vet` 覆盖 `./pkg/common/... ./pkg/core/service/... ./pkg/job/...` 全部通过。此修复不注册恢复协调器，不代表 M2 整体完成。

> Backend `6e3e7eb`／`f2aa4f9` 增加 V3 项目方钱包专属 `depositProjectTokens(uint256,uint256)` ABI 打包及 Pending／精确项目绑定／项目方角色的只读规划器，并要求提供的 caller 地址与固定块观察中的项目方一致；通用 relayer lifecycle 方法明确不接受该调用，金额与 projectId 检查正数和 uint256 边界。显式提供的 V3 编译 artifact ABI 校验通过，三组 Backend 包的广域 Go test/vet 通过。该规划器不认证 caller 的钱包控制权、不处理 ERC20 allowance／钱包签名／广播，也不解除旧 outbox 或 Job 门禁；**M2 仍未完成**。

> Backend `1ee74e5` 将相同的 caller 地址核验覆盖 Pending 取消、项目款和项目币提现的 V3 只读规划器；仅凭 `V3ProjectOwner` 角色标签及其它地址不能预览项目方动作。`GOEXPERIMENT=jsonv2 go test ./...` 与 `go vet ./...` 全仓通过（包含原有 notice 工作区改动，未纳入提交）。这些校验仍不证明钱包控制权，也不替代执行时重读和模拟；M2 全栈代码／测试仍未完成，M0／M1 签核与生产准入独立保持未通过。

> Backend `479b672` 要求 V3 创建与生命周期只读预览的 Operator／Owner Safe 角色不仅有标签，还与单独传入的非零、非 Proxy、互不复用的预期 caller 地址相符；空预期或仅换角色标签不被接受。外部预期值仍需经过正式来源审核及链上固定块 owner/operator 身份复核，签名权／Safe 门限和执行时状态并未在此认证；全仓 Go 单测通过，但不构成任何 V3 出站授权或 M2 完成。

> Backend `e418efe` 增加只读 `ReadLifecycleCallersAt`：沿用独立预期治理身份及固定块 V3 Proxy／implementation／版本和 owner／governor 合约代码验证，再核验 `operator()` Getter 的完整 ABI 字及同块后读锚点。测试覆盖错 operator、角色重合、畸形返回、RPC 错误脱敏及读中换块；全仓 Go test/vet、cmd build 通过。此观察**不认证预期值来源、Safe 配置、签名保管或生产 runtime，不注册 Job／outbox**；M2 全部代码与测试未完成，M0／M1 签核与生产准入仍待审。

> Backend `4760436` 在上述固定块治理观察中拒绝 owner 与 governor 同地址；创建／生命周期预览对 Operator／Owner Safe 均要求两者同时有独立预期地址且互不重合，单个地址不能同时承担两种角色。目标 Go 包单测／vet 通过。此约束仍不证明两个真实 Safe 的签名人独立性、阈值或权限配置；M0 决议未签、M2 未完成。

> Backend `4018e1e` 对**未注册** V3 journal 的 `resumeAtAncestor` 增加预期 cursor revision，在 `FOR UPDATE` 锁下复核及条件 `UPDATE` 再次匹配，拒绝只读预检之后并发 revision 漂移；SQL mock 含 stale revision 负例，网络隔离 MySQL 8.4 候选审计／创建／rollback 演练与 Backend 全仓 Go test/vet/cmd build 通过。测试通过不把 `v3CheckSurvivingPrefix` 只读观察升格为恢复许可，仍缺真正受认证的祖先选择、锁内链上重验与事后持续 canonical 复核；M2 整体未完成。

> Backend `f477af3` 对同一**未注册** journal 的 `rollback` 增加预期 revision：孤儿 journal／checkpoint 与投影撤销之前在 `FOR UPDATE` 锁内拒绝漂移，cursor 条件写再带 revision；SQL mock 有旧 revision 不写入负例，隔离 MySQL 8.4 演练及 Backend 全仓 Go test/vet/cmd build 通过。这仅是数据库并发 fence，不认证外部 ancestor／runtime，不可据此把原型用于自动恢复、钱包余额或发布；M2 仍未完成。

> Backend `22abd58` 对只读、**unverified** V3 项目联合审计增加独立预期项目身份全字段比较：chainId／projectId／Proxy／项目币／支付币变更一律拒绝，返回错误而不构造可信 `ExpectedProjectBinding`。目标包 test/vet 通过；此比较只改善诊断，不能以候选行与自报预期相等为由注册扫描、构建 proof 或建立财务权益。M2 信任准入与出站闭环仍未完成。

> Backend `b86169b` 进一步要求 V3 原型 `rollback` 必须从已 `reorg_halted` 且 revision 匹配的 cursor 执行；运行中的 cursor 即便收到匹配旧 checkpoint 也不能直接孤儿写入。隔离 MySQL 8.4 合成演练先证实运行态 rollback 拒绝，再明确 `haltMismatch` 撤销投影，最后在停机态执行孤儿回退；Backend 全仓 Go test/vet/cmd build 通过。`haltMismatch` 只在已证实规范哈希不匹配时使用；此演练未实现可信 ancestor 选择、锁内链上复核、正式运行或生产恢复，M2 未完成。

> Backend `3c76224` 在未注册范围扫描最终确认数 head 读取之后，再次核验已持久化 cursor 的规范哈希；隔离 MySQL 8.4 合成对抗用例在首次最终 canonical 读与末次 head 之间替换区块，验证本轮报错、`reorg_halted` 以及内部投影撤销。全仓 Go test/vet/cmd build 通过。有限重复读不保证后续不会 reorg，仍缺持续监视、经认证来源与完整恢复协调；M2 未完成。

> Backend `1402cc6` 同样在未注册创建块 bootstrap 的最后确认头读取后，重新核对父块及创建块哈希；合成 RPC 在该 head 读取时替换创建块，验证拒绝且尚未触及数据库。网络隔离 MySQL 8.4 迁移／创建演练及全仓 Go test/vet/cmd build 通过。重复锚点读取不是永久确认、已批准来源或正式创建入库入口；M2 仍未完成。

> Backend `cb645f9` 对未注册范围扫描的部分成功分支增加已提交 cursor 的失败后规范性复核：第二块读取失败时，若第一块在失败期间被替换，原子停机撤销第一块投影，而不只返回第二块错误。隔离 MySQL 8.4 合成对抗及全仓 Go test/vet/cmd build 通过；RPC 故障仍仅为未知状态，不猜测重组祖先或授权自动恢复，M2 未完成。

> Backend `94b3208` 在未注册范围扫描最终确认头降低或不可用时，也独立复核当前 cursor 的规范哈希；合成 MySQL 8.4 用例验证：若 head 同时降低且已发布区块被替换，停机撤销投影；单独低 head 不构成 reorg 证据。全仓 Go test/vet/cmd build 通过，仍非持续监视、可信准入或 M2 完成。

> Backend `fb894c7` 为未注册创建块 bootstrap 增加**事务提交后**的持久 cursor／checkpoint／链上规范哈希复核：合成 MySQL 8.4 在创建投影刚提交后替换区块，证实 bootstrap 报错、停机并撤销投影；网络隔离 runner、全仓 Go test/vet/cmd build 通过。RPC 不可用仍为未知，不把提交后短时复核视为永久最终性，也不注册生产 Job 或向 Core 公开钱包权益；M2 未完成。

> Backend `566dc00` 修复该 bootstrap 在**提交后 RPC 未知**时仅报错但可能留下可读投影的失效路径：精确的首个 cursor 高度／哈希及 revision=1 锁内 fence 后，原子撤销内部投影并停机，不孤儿化 journal、不推断祖先；若有并发 cursor 推进则拒绝错误地撤销。SQL mock 与网络隔离 MySQL 8.4 覆盖 RPC 未知、已证实 reorg 和并发漂移；全仓 Go test/vet/cmd build 通过。未注册且不构成可信生产恢复；M2 未完成。

> Backend `f2b0b49` 对未注册范围扫描部分提交后的**规范 RPC 未知**补齐相同的停机撤销：仅在本批次已提交高度／revision 正好匹配锁内 cursor 时原子删除内部投影并停机，不孤儿化、也不撤销其他 worker 后续进度。SQL mock 覆盖 revision／高度漂移，网络隔离 MySQL 8.4 在第 98 块提交、第 99 块失败且 98 块头读取故障时验证撤销；全仓 Go test/vet/cmd build 通过。这是失效关闭，不是可信来源、最终性或自动恢复；M2 未完成。

> Backend `feb0be6` 将上述失败关闭延伸到未注册范围扫描**全部块提交后**的首轮／末轮 canonical 复核，以及最终 head 不足确认同时 canonical 未知的分支；已被另一并发检查停机时不会错误报撤销失败。网络隔离 MySQL 8.4 合成回归及全仓 Go test/vet/cmd build 通过；仍缺来源认证、锁内链上许可及持续注册的监控，M2 未完成。

> Backend `e996299` 修正最终 head 单独失去所要求确认深度的分支：即使规范哈希仍匹配，刚提交的内部投影也不能保持可读；在高度／revision 锁内 fence 下停机撤销，保留 canonical journal 并等待正式恢复。网络隔离 MySQL 8.4 及全仓 Go test/vet/cmd build 通过。head 降低本身不判定重组，V3 仍未注册或完成 M2。

> Backend `4083bf8` 在范围扫描**开始前**已有内部投影而 head 落后 cursor／低于所需确认深度时，也按精确高度／revision fence 停机撤销；隔离 MySQL 8.4 验证低 head 与单纯浅确认均不会错误孤儿化 journal、却不再对内提供钱包投影。全仓 Go test/vet/cmd build 通过；这不构成实际链上重组证明或正式恢复，M2 仍未完成。

> Backend `b410866` 对已有内部投影、但本轮范围扫描读取 head 直接失败的分支也停机撤销；隔离 MySQL 8.4 验证 RPC outage 不会被当成已证明 reorg 或成功空闲，canonical journal 保留且钱包投影不再可读。全仓 Go test/vet/cmd build 通过；持续服务与恢复协调仍缺失，M2 未完成。

> Backend `7e5ad05` 对下一轮扫描**最初**的 cursor canonical 预检失败（包括规范区块 RPC 不可用）也调用相同的精确高度／revision 锁内停机撤销，已由另一检查停机时幂等返回；隔离 MySQL 8.4 及全仓 Go test/vet/cmd build 通过。未注册 Job 还无法持续执行这些检查，可信来源、恢复及财务 applied 均未闭环，M2 未完成。

> Backend `557cfa9` 在未注册创建块 bootstrap **提交后**再次检查确认头深度；规范哈希仍匹配、但 head 短暂变浅时也停机撤销内部投影，保留创建 journal；隔离 MySQL 8.4 合成降低 head 及全仓 Go test/vet/cmd build 通过。低 head 不是重组祖先选择或生产恢复依据，M2 仍未完成。

> Backend `a736693` 补充隔离 MySQL 8.4 回归：已提交创建投影后下一轮初始 cursor header RPC 不可用，精确 fence 撤销投影、保留 canonical 创建 journal；测试恢复其他场景时**删除并重新 bootstrap 测试夹具**，不伪造 halted cursor 回到 running。全仓 Go test/vet/cmd build 通过；这不是生产恢复入口，M2 仍未完成。

> Backend `6af0236` 对已发布 V3 投影、但下一轮扫描无法重读 chainId（或读到错链）的分支执行同一高度／revision fence 停机撤销；隔离 MySQL 8.4 用链身份 RPC 失败验证 canonical 创建 journal 保留、内部投影禁读，重新 bootstrap 仅限测试夹具。全仓 Go test/vet/cmd build 通过；无正式来源和持续 Job，M2 仍未完成。

> Backend `f394b52` 修复范围扫描首个新区块失败（本批 `scanned=0`）、随后的已发布 cursor 规范哈希 RPC 不可读时漏撤销**上一批**投影；现按旧高度／revision 精确停机。隔离 MySQL 8.4 验证 cursor 停机与内部投影禁读；全仓 Go test/vet/cmd build 通过。未知 RPC 不等于重组证据，M2 未完成。

> Backend `7cc7776` 对未注册创建 bootstrap 的提交后成功 head 核验再增加父块 hash 独立读取；父块 RPC 在提交后不可用时精确停机撤销创建投影，不能仅凭仍匹配的创建块头保留发布。隔离 MySQL 8.4 测试及全仓 Go test/vet/cmd build 通过。这些时点复核仍不是持续监控、已认证运行时或正式恢复，M2 未完成。

> Backend `91670e4` 新增 V3 项目方钱包充值的**固定区块只读**余额与 `allowance(owner,V3Proxy)` 预检，严格匹配预期 owner／项目币／chainId／区块 hash／ABI 返回，并在读取后再次检查锚点；定向单测覆盖错链、余额不足、错误 spender、畸形 RPC、换块及错误脱敏，全仓 Go test/vet/cmd build 通过。它仅返回 calldata，不签名、不授权、不创建共享 outbox、也不执行 ERC20 approve；执行时需钱包重检与签名，M2 仍未完成。

> Backend `f4f2596` 要求该钱包充值预检另行提供经独立审核的项目币运行时 code hash；空值或链上字节码与预期不符在读 allowance 前失败关闭，不能把不受信 RPC 自报的 code 作为可信身份。定向单测及全仓 Go test/vet/cmd build 通过。哈希来源尚未准入，仍非可用钱包执行链路或 M2 完成。

> Backend `de1d88b` 分别测试充值余额不足与 allowance 不足；`945fc41` 为 V3 创建、生命周期和项目方充值增加只读 transport 规划，严格区分 Operator relayer、Owner Safe、项目方／公共钱包，禁止以 Safe 提案伪装 relayer outbox 或以角色标签冒充项目方。定向及全仓 Go test/vet/cmd build 通过；规划结果不构造持久 action／Safe 提案、不证明身份来源或签名保管，M2 仍未完成。

> Backend `fcc657d` 测试公共结算／平台费触发与项目方提现都只能使用钱包 transport；`75637a0` 在 V3 充值余额／allowance 读取后再比对固定块项目币 code hash，拒绝读中代码身份变化。定向和全仓 Go test/vet/cmd build 通过；不会对独立受审 token hash 来源、签名、广播或持续最终性作出推断，M2 仍未完成。

> Backend `68b0bd2` 让项目方 V3 充值只读预检在余额／allowance／code hash 核验后，使用**真实 owner From、独立 Proxy To、相同固定块 calldata** 做一次 `eth_call` 模拟，拒绝回退、非空 ABI 回执和读中换块；定向与全仓 Go test/vet/cmd build 通过。这仍是瞬时预检，钱包签名前须重新模拟并自行授权／广播；不解除出站门禁，M2 未完成。

> Backend `ca8344b` 对未注册 V3 journal 的连续 apply，在 `FOR UPDATE` 取得 cursor 后禁止高度／revision 溢出，并将相同 revision 加入最后 cursor 条件更新以拒绝锁内漂移；SQL mock、隔离 MySQL 8.4 及全仓 Go test/vet/cmd build 通过。这仅是原子并发 fence，不认证 caller-supplied runtime、Job 来源或恢复祖先，M2 仍未完成。

> Backend `accf05a` 修复无新增确认块的 idle tick：若末次 canonical header 复核 RPC 不可用，也对先前 cursor／revision 精确停机撤销内部投影，保留 journal 而不推断重组。隔离 MySQL 8.4 回归覆盖读两次后失败及投影禁读；全仓 Go test/vet/cmd build 通过。正式来源准入、持续监视及认证恢复仍未实现，M2 未完成。

> Backend `ffe6367` 增加 journal apply 的 SQL mock 负例：高度或 revision 溢出在任何写入前拒绝；条件 cursor 更新因 revision fence 未命中时，checkpoint／投影写入一并回滚。全仓 Go test/vet/cmd build 通过；与上述隔离 MySQL 验证只覆盖局部原子性，未构成 M2 可信来源或正式扫描闭环。

> Backend `cc74cce`／`8e85ea4`／`ab54b3b` 补齐未注册 V3 journal 停机与恢复边界的 revision fence：已停机幂等仅承认紧接原 revision 的停机，`haltMismatch` 锁内读取和条件写均绑定同一 revision，rollback/resume 与停机拒绝 revision 溢出；追加旧 bootstrap halt、范围 halt 溢出、mismatch halt 溢出及条件更新失败回滚投影删除的负例。SQL mock、隔离 MySQL 8.4、全仓 Go test/vet/cmd build 通过（最后新增负例已通过目标包测试）；仍不认证祖先／runtime，也不接入正式 Job 或 outbox，M2 未完成。

> Backend `0473e17` 增加未注册 journal rollback／resume 的最大 revision 负例：必须在读取锁内 cursor 后、查询 checkpoint 或写孤儿／投影前拒绝溢出；目标包测试通过。这只验证拒绝写入的安全边界，不能将只读恢复预检当作认证祖先授权，M2 未完成。

> Backend `ee8ea5c` 统一 V3 只读策略证据、未注册 scanner／bootstrap／recovery／range 的确认深度计算：以 `head-block >= minimum-1` 避免 `uint64` 最大块高 `+1` 回绕，并覆盖零输入、浅确认、最大值边界。全仓 Go test/vet/cmd build 通过；算术安全不证明规范链最终性、来源准入或 M2 完成。

> 生命周期事件只读基础：Backend `6f32dea` 增加专属 `DecodeV3LifecycleLog`（充值、激活、结算、取消、退款、领取及三种提现）和 `MatchV3LifecycleBinding`，严格核验显式 chainId／Proxy、事件 ABI、项目 ID、代币以及必要的金额/状态；不走 V2 解码路由，不注册 Job、持久账本或改变 outbox 门禁。七个相关 Go 包定向测试/vet 通过；共享事件 topic 绝不单独构成 V3 来源认证。剩余 M2 的真正可信身份准入、完整扫描/账本/reorg 与出站恢复仍未完成。

> Backend `c8f4957` 对 V3 生命周期事件添加指定 txHash 的唯一成功收据日志匹配，拒绝失败状态、重复 logIndex、异源/替换内容；`4f30036` 在只读 RPC 上组合 chainId、确认深度、收据、规范块和确认头的重复锚点读回，RPC 错误脱敏。七个 Go 包定向测试/vet 通过。它只是**一次调用的瞬时证据**，不认证 binding 来源、不注册扫描、不写账本，也不覆盖后续 reorg；M2 仍未完成。

> Backend `197650b` 补充 V3 独立 `ProjectCreated` 固定 4 topics／11 字 ABI 的创建快照解码及精确链／Proxy／项目／双代币绑定比较，校验费率、decimals、募资区间、地址和全长编码；共享 V2 topic 仍不得作为版本路由。七个相关 Go 包测试/vet 通过。事件只能为后续受控项目绑定提供证据输入，不能自己创建项目、审批、注册扫描或写财务状态。

> Backend `030dfcb` 为 V3 创建快照增加唯一成功收据日志匹配，拒绝失败收据、重复 logIndex、异源和字段替换；七个相关 Go 包定向测试与 vet 通过。此匹配仍不认证受信项目/治理来源、不具备 canonical/reorg 持续复核或项目入库权限；M2 的 durable scanner、ledger 和 outbox 仍未闭环。

> Backend `f3e9acc` 增加**离线纯内存** `RebuildV3Replay`，按同块 logIndex 顺序重放创建、充值、激活、购买、结算／取消、退款／领取及提现，验证项目代币绑定、硬顶／充值准备金、软顶状态、手续费和资金分配，拒绝同块不同 hash、重复索引和非法状态；单测覆盖成功、失败、取消与移除尾部事件后的重建结果。该模型无受信来源、持久化、Job scanner、账本唯一键、真实重组 recovery 或资金写入，**不能视为 M2 财务闭环**。

> 后续 Backend `6c379df` 加入逐钱包购买／退款／领取、价格、单钱包上限及收款方校验；`2214ad0` 将明确给定 V3 Proxy 的 raw log 解成 replay 输入，拒绝共享 topic 的 V2 地址，不把 topic 当准入。Deploy `5a9ab81`／`95a6cf5` 增加**单独、门禁、未应用生产**的 V3 cursor／不可删 journal／空块 checkpoint／项目与钱包投影候选表，隔离 MySQL 8.4 `--network none` 顺序迁移演练通过。Backend `7c94f82` 提供**未注册、无外部调用者**的事务型 journal prototype：按项目批次重建投影，失败回滚，重组时 orphan 并撤销投影、将 cursor 置 `reorg_halted`；八个相关 Go 包测试和 vet 通过。缺可信 runtime admission、完整区块与成功收据认证、Job 调度、多项目扫描、持久恢复／重放、独立 Core action／outbox 和 applied 财务接线，**M2 仍未完成，不开放生产 V3**。

> Backend `4181992`／`7157dd5`／`22d6693` 为事务 journal 增加上一块哈希 fence、唯一 cursor／checkpoint 校验和持久投影写入测试；`76c7a76` 可在**外部已认证祖先块**的前提下，从停机状态按存活前缀重建再顺序扫描，仍无自动可信恢复；`822a81f` 增加未注册、仅只读的固定区块 V3 事件与成功收据扫描雏形，尚无生产来源准入。Backend `a6e5ba0`＋Deploy `2501732` 在同一个网络隔离 MySQL 8.4 全表形状演练中验证无效事件事务回滚、空块 checkpoint、重组 orphan/halt、恢复后替换空块及投影撤销；这仍**不是**创建／认购／财务入账闭环或 V3 扫描启用许可。

> Backend `862d3c3`／`7b00f29` 为未注册 scanner 增加已知项目 topic 白名单、日志排序及按持久 cursor 比对规范区块 hash；`b316444` 限定 replay 累计数为 uint256。`0ea77a9` 扩展同一网络隔离 MySQL 演练，使用合成 ABI 事件验证创建→充值→激活→认购→成功→钱包领取的独立投影、重组撤销孤儿领取和祖先重建钱包持仓。**合成日志不是可信扫描结果**；仍无授权的 runtime 准入和 Core／outbox／applied 接线，M2 未完成。

> Backend `5ccf3a9` 使未注册 scanner 在提交前重读过滤器返回的完整日志集，拒绝瞬时变化；`0f924ab`／`8a56458` 增加与 V2 分离的 V3 创建、激活、取消、结算与三种提现 ABI calldata packer，使用明确提供的已编译合约 artifact 验证签名／参数形状。它们不签名、不广播、不绕过 V3 旧门禁；尚缺可信身份与按版本 Core 入口、受控 durable action、Safe／relayer 分流、applied 推进及 scanner 运行闭环。

> Backend `349549c` 在未注册 scanner 的收据第二遍复核后再验证确认高度和规范区块锚点；`eb7ba03` 将 cursor 确认发生规范哈希不匹配时的同一事务停机与已发布项目／钱包投影撤销加入隔离 journal，RPC 失败不能作为链重组证据。`7d9232f` 允许已停机 cursor 按已核验的先祖 checkpoint 回退孤儿事件，测试演练通过；`649329c` 在隔离 MySQL 证明合成领取投影先撤销再回滚恢复。需要**已认证链上先祖与逐笔收据**才能实用，不能把本地合成恢复当作真实运行中的自动重组处理；V3 scanner／outbox／审批仍关闭，M2 未完成。

> Backend `111be12` 增加离线完整区块重放的状态／raised／sold 与同块固定 `getProject` 只读观察的一致性比较；`4f31d47` 在隔离 journal 内增加可选 `applyChecked` 路径，财务投影与独立给定的项目观察不符时整笔事务回滚，合成 MySQL 演练确认 cursor／checkpoint 不推进。传入的 observation 不会在此认证，正式扫描调度仍未接线，不能据此发表钱包余额或宣布 M2 完成。

> Backend `5448a60` 将**未注册**的固定区块扫描与事务型 `applyChecked` 组合，收据验证后重读 runtime／项目并复核区块及确认高度；`ba1aeef` 将隔离 MySQL 合成生命周期各区块改为 checked journal，并在金额不符时验证整事务不发布。缺真实独立身份来源、Safe 签核、运行调度和连续 reorg/applied 对账，不能据此开启 V3 认购或证明 M2 完成。

> Backend `252db2e` 要求重建时每个 canonical journal 事件区块都有同项目、同高度且哈希相同的 canonical checkpoint，拒绝越过 cursor 的事件及不一致 checkpoint；网络隔离 MySQL、定向测试和 vet 通过。该约束不认证 RPC，也不能填补 Job 注册、可信身份、专属 outbox 与 `applied` 状态的缺口。

> Backend `3dd0fae` 在 journal 重建时检查持久 `event_kind` 与事件 payload 精确一致，拒绝不匹配的领取／认购事实；`b345f23` 使只读 scanner cursor 检查要求同一 canonical checkpoint；`c5f96ad` 又在 journal 的事务锁内要求**上一块** cursor/hash 有匹配 checkpoint，缺失时任何空块或事件都不得推进。SQL mock、网络隔离 MySQL 与 vet 通过；仍未取得可信 runtime 准入或启用扫描。

> Backend `a753882` 使已发布投影遇到成功读库但缺失／错配的 cursor checkpoint 时立即按精确 cursor fence 原子停机并撤销，DB 读取故障仍只报告未知而不推断 reorg；定向测试／vet 通过。这里的数据库不一致也不自动判定链上重组或许可重新扫描，须经受信来源及完整事件恢复核验。

> Backend `1094d8d` 用无签名能力的固定区块 RPC fake 对**未注册** scanner 做成功空块读取与两次 FilterLogs 变化拒绝回归；`69a5306` 验证已停机 cursor 在 checked scan 中不得继续走 RPC 项目读取。八个 Go 包目标测试／vet 通过。这不是可信 RPC 准入、正式运行调度、带购买和成功收据的全路径集成测试，也不解除生产门禁。

> Backend `fd0d51d` 使用独立 ABI pack 的 `ProjectCreated` 事件和成功收据，对未注册固定区块 scanner 加入完整读取正例、第二次收据变化及同 topic 异地址拒绝负例；定向测试／vet 通过。假 RPC 的项目／runtime 不是受批准身份；还需购买和其他生命周期跨项目边界、持续重组与真实运行对账。

> Backend `04fafef` 的同 Proxy 多项目合成购买测试要求固定块项目读取／成功购买收据绑定项目 7，过滤同 topic 的项目 8 日志，并拒绝失败收据；定向测试／vet 通过。测试夹具不是可信审批或身份准入，不会在 V2 地址上注册 scanner，也未触发持久 applied 财务。

> Backend `821825a` 在网络隔离 MySQL 上验证：创建之前的空块即使 runtime／getProject 假读取成功，也不允许 checked scanner 发布无创建事实的投影或推进 cursor；离线 journal 的合成空块 checkpoint 只供内部测试。正式扫描需独立认证创建块／startBlock，不能“先扫描空块再默认升格为 V3 已创建项目”。

> Backend `5f31c1f`／`e3705b5` 对独立 ABI 编码的 23 字段 `getProject` 读出增加创建时 owner、feeReceiver、精度、费率、价格、软硬顶、个人上限及时间窗比对；`44946be` 将精确 `ProjectCreated` 事件与同块项目观察的经济参数比对纳入未注册 scanner，价格字段漂移拒绝。仍不能从事件推定受批准 runtime／项目来源；不授权入库、余额、认购或生产发布。

> Backend `569dbab`／`0f78515` 在 checked journal 每个后续区块重读唯一 canonical `ProjectCreated` 事实及 checkpoint，并比较当前固定块 `getProject` 的不可变经济参数；`c855428` 用网络隔离 MySQL 验证后来价格字段漂移会让整块 journal／cursor／checkpoint 回滚。持久行的创建来源仍须受信身份和成功收据认证，之后每次 reorg 必须持续核验；没有运行中的 Job／applied／outbox 闭环。

> Backend `3057a2c` 增加**仅 Job 包内部**投影一致性读取，要求运行中的 V3 cursor 与同 revision／高度／哈希的投影及 canonical checkpoint 三方一致；隔离 MySQL 合成钱包验证停机后读不到。它不能作为 Core 钱包 API 或链上余额，也未认证 runtime 来源；仍不能将该投影向 C 端开放。

> Backend `0593e26` 新增**未注册**创建块 bootstrap 原型：只接受显式项目／runtime／创建块，读全块和成功收据、唯一 ProjectCreated、创建经济参数及前块父哈希；重复核对区块锚点与确认深度，再用单个数据库事务初始化上一块 checkpoint/cursor 并走 checked journal。合成 RPC 对缺失／歧义创建、父块不匹配、价格漂移和事务失败提供拒绝回归。显式输入依然**没有可信审批/持久来源认证**，没有生产启用或稳定 reorg 调度；这不是可用的创建入库入口。

> Backend `34bf80f`／Deploy `08f9426` 将该 bootstrap 加入 `--network none` 一次性 MySQL 8.4 演练：合成创建后验证前块及创建块 checkpoint、唯一 journal 事件、revision=1 投影和重复 bootstrap 拒绝；仍未认证项目、runtime 或真实链，不视为 M2 完成或生产迁移。

> Backend `d3c0c11` 同一隔离 MySQL fixture 在合成创建后通过未注册 checked scanner 处理后续空块（cursor revision=2），再模拟其规范哈希不符，证明停机撤销内部可读投影。这只测试连续两块与明确 reorg 信号，不具备受信来源、真实长程扫描、恢复重放或对外钱包权益。

> Backend `ff6138e` 在 bootstrap 最后一次固定块项目读取后重新检查创建交易的成功收据及唯一事件，拒绝期间提供者将第三次收据改为失败；合成单测及隔离 MySQL 回归通过。短期复核不等于持续 finality。

> Backend `e781a8c` 在未注册 checked scanner 的第二次项目/runtime 读取之后，对待入 journal 的每条创建、购买及生命周期日志再次做指定 tx 成功收据匹配；网络隔离 MySQL 构造第三次收据转失败，验证 cursor 不推进。它仍不证明读链提供者可信，也不能替代重组后撤销与逐笔恢复。

> Backend `27dcd91` 增加**未注册、仅只读**的有限区块存活前缀预检：对已明确给定的创建块至先祖范围逐块比对项目 checkpoint／链头连续性、完整过滤日志与成功收据、持久 payload/kind、创建经济参数及后续固定块不可变字段，结束前再次读回数据库行和首尾锚点；隔离 MySQL 对创建块成功、换收据和缺中间块拒绝。它不选择重组先祖、不执行 rollback/resume，也不能将一次核验结果升格为可信来源或生产恢复授权。

> Backend `63e2283` 对每个存活前缀区块重放截至该块的全部 V3 事件，并核对对应固定块 `getProject` 的状态、募资和售出量；隔离 MySQL 合成创建及定向 vet 回归通过。尚未把该只读判断绑定到加锁 rollback/resume 的事务准入，仍非自动可信恢复。

> Backend `03f396b` 在该只读前缀预检每块项目读取后，再次逐条核对成功收据与日志；隔离 MySQL 证明第三次创建收据变为失败时拒绝前缀。仍须持续锚点复核、受信身份与写入时 fence，不能用作自动恢复许可证。

> Backend `3ba1f84` 的隔离 MySQL 测试故意篡改持久创建 `event_kind` 和 canonical checkpoint 哈希，分别验证只读存活前缀拒绝，恢复夹具后原测试继续。测试不说明真实库可任意修复，也不允许手动编辑生产 journal。

> Backend `5662e5a` 在隔离 MySQL 合成两高度只读 RPC 上，对创建块及下一空块的完整存活前缀正向复核通过；单高度假 RPC 缺中间头仍拒绝。此为可重复夹具，不是主网跨块提供者或持久恢复准入。

> Backend `aac669e` 修补纯内存财务重放的激活约束，按 V3 合约 `Math.mulDiv(softCap, 10^projectTokenDecimals, tokenPrice)` 的向下取整语义要求项目代币充值量足够；缺额拒绝，恰好达到软顶的前缀允许，后续购买仍受库存上限约束。Go 定向测试／vet 与隔离 MySQL 合成 journal 通过，不替代链上真实业务校验或可信财务对账。

> M2 局部增量：Backend `ca8d5b0` 的 `AuditInertProjectIdentity` 以 REPEATABLE READ 只读事务核对锁定的 V3 项目、精确 `chainId+Proxy+projectId+contractVersion` binding、未验证 runtime 与非零代币／实现／代码哈希，失败及数据库错误脱敏；sqlmock 正反例、七个相关 Go 包测试与 vet 通过。这是**未验证身份行的离线诊断**，不是可信 runtime 准入、真实链上治理认证、专属 Job 扫描、财务 materialization 或 V3 上线许可。M0/M1 仍未签核，旧门禁不解除。

> 隔离 MySQL 8.4 验证：Backend `86a42b2` 扩展一次性 `--network none` 全表形状合成数据演练，将锁定的 V3 项目代币与支付币地址置为精确规范值，验证审计账号只读核对项目／binding／未验证 runtime，错误 chain／Proxy 拒绝；原有候选完整成员、只读权限与 V2 合成行不变测试通过。此演练没有现网项目分布、生产 ledger、链上 Safe 身份或审批/扫描证明。

> 防止不可信类型升级：Backend `ddd8b1d` 将上述未验证数据库审计返回限定为 `InertProjectIdentityAudit`，不再直接返回读链函数所需的 `ExpectedProjectBinding`／`RuntimeIdentity`；后续可信 runtime 来源与准入必须独立建立，不能把 unverified 行自动提升。定向测试／vet 通过，扫描和 outbox 门禁保持关闭。

> 后续 M2 局部代码：Backend `0a1c3d4` 在 scanner／bootstrap／只读前缀预检的后续项目读取后重检**完整** Proxy 日志，`7114ef8` 在前缀预检结束逐块复核全部区块头（含空块）；`4f5a079` 深拷贝首次过滤日志及嵌套字段，防止复用 RPC 缓冲区使两次比较同时变化。`0b7ccd8`／`ab849ca` 增加 V3 创建及生命周期的显式项目绑定、角色／状态 ABI 规划器，**不认证输入来源，不调用 V2 outbox，不签名／广播**。八个 Backend 相关 Go 包的 `GOEXPERIMENT=jsonv2 go test` 与 `go vet` 通过。scanner、bootstrap、恢复和规划器仍未接入可信来源、Core、正式 Job 或财务 `applied`；**M2 代码与单元测试整体未完成**，M0／M1 签核和生产准入另行报告。

> Backend `3ec9772`／`23a7912`：创建块在项目读取后逐条复核所有目标项目事件的成功收据（同交易创建与充值均覆盖），变化后不得启动事务；`6575fba` 将无 caller-role 限制的结算与平台费触发条件和合约一致化。`dde7d80`／`67c558b`：checked scanner 提交前复核父区块哈希，明确发生父块替换时由加锁 cursor fence 停机并撤销已发布投影，RPC 错误不能假定重组。八个 Go 包 test/vet 及网络隔离 MySQL 8.4.10 合成演练通过；**仍未注册可信准入／持续 Job／财务 applied／出站链路，M2 整体未完成**，没有生产操作。

> Backend `5af3224`／`6a34269`／`c3639fb`／`d0aa6d6`：只读恢复前缀结束时再次逐块核对完整 Proxy 日志、成功收据、固定块项目观察、持久 journal/checkpoint 和连续区块头；隔离 MySQL／SQL mock 对晚期过滤器、收据、经济参数及 DB 行变化均拒绝。`2172d37` 增加**未注册**、至多 512 块的连续确认块扫描驱动器，从既存运行中 cursor 调用 checked scanner，停机 cursor 拒绝、失败不跳块。测试／vet 通过并不等于已认证来源、锁内恢复许可或正式运行，**M2 仍未完成**。

> Backend `f11f581` 在隔离 MySQL 8.4.10 将该未注册驱动器用于从已 bootstrap 创建块连续扫描下一空块，验证 checkpoint／revision 和确认边界；`fc1a00e` 在无新确认块的 idle 返回前再核对持久 cursor，合成规范块于 head 读取时替换则停机撤销投影。`658ff94`／`5a1e3ff` 拒绝仅有未知键或标量根键的 V3 Job shadow 来源，避免误认为未配置。八个相关 Go 包测试／vet 与网络隔离 MySQL 演练通过；**V3 Job 仍未注册，来源未认证，财务 applied／outbox／可信恢复未闭环，M2 未完成**。

> Backend `bf25b92`：若只读 RPC 报告的最新块低于已持久 cursor，高度信息为未知／不一致，有限扫描返回错误而非成功空闲，亦不凭落后头推断重组或撤销；隔离 MySQL 回归确认 cursor 仍运行，八包测试／vet 通过。该防护不替代可信数据源或生产恢复流程。

> Backend `6950cd7`／`887c5c8`：有限驱动器在读取 head 的前后核对持久 cursor 的高度／哈希／revision／running 状态，拒绝并发更新后的空闲返回或陈旧扫描起点。`c8cf1ba` 在最后一块 checked journal 提交后再次检查规范 cursor；隔离 MySQL 模拟末尾替换，证明已提交块数可报告但必须返回错误、加锁停机并撤销内部投影。八个相关 Go 包测试／vet 与网络隔离演练通过。**这些瞬时复核不构成可信来源、链上最终性或正式恢复；M2 仍未完成。**

> Backend `cdc6630`／`988b20b`：有限扫描完成后再读 head，若最后已提交块失去所需确认深度则返回错误；空闲时已持久 cursor 若仅剩不足确认数也不能被报告为健康空闲。隔离 MySQL 对两种深度下降做拒绝回归，单独落后 head 仍作为未知，不凭它停机或假定重组；八包测试／vet 通过。已提交投影仍须正式 Job 持续监控，**不能据此称 M2 完成或发布**。

> Backend `f49ba4f`：隔离 MySQL 对两块的未注册范围扫描验证部分提交语义：第 94 块空块提交、第 95 块不支持 topic 拒绝时仅计 1 块、cursor 停在 94、无 95 checkpoint；修正合成 RPC 后下次从 95 精确继续。八包测试／vet 与网络隔离演练通过，**不等于可信链上重放、正式调度或自动恢复**。

> Backend `537fd02`：已提交的最后区块经规范检查后，在末尾 RPC head 读前后用 cursor 高度／哈希／revision／running 状态及本批次已推进块数核对，拒绝并发改写后报告成功；隔离 MySQL 模拟 96 块提交后仅 revision 变化，驱动器返回已提交数加错误、不把 revision 漂移误判为重组。八包 Go 测试／vet 与网络隔离演练通过。仍为未注册且不能作为可信恢复许可，**M2 未完成**。

- 数据库新增 V3 身份、策略／成员快照／版本／批准记录／链上生效证据等迁移；不抹除原 V2 行、`project_whitelists` 行或财务历史。若共享链事件／账本表，所有写入与唯一键须以 `(chainId, contractAddress, projectId, contractVersion)` 绑定，历史仅以其已存身份读取，绝不以全局最新配置重解释旧记录。审计现有仅按 `project_id` 聚合的路径及跨版本碰撞，明确全局 ID 唯一性的约束。
- 引入独立可信 `chain.project.v3.*` 配置（chainId、Proxy、implementation 地址／code hash、startBlock、治理身份、支付币、费率快照）；在**现有后端主流程上扩展** Set／创建／激活／取消／结算／提现，V3 上线后不维持 V2/V3 两套日常操作入口。切换前关闭 V2 新业务写入；不为 V2 内测遗留开发升级专属后台收尾流程。Set 仍走现有预览与四眼，不让前端任意提交合约地址。新 V3 项目创建上链后永久锁定版本和 Proxy。使用独立 Action 名与 ABI，修改 outbox DB action CHECK、确认处理器、事件白名单和链交易页面。
- Job 在现有框架中接入 V3 来源、事件解码／materialization、applied 推进、rollback/replay、钱包权益与对账；V2 不作为 V3 上线后的并行业务扫描要求，也不以 V2 内测遗留核对作为 V3 Job 注册前置条件。不能简单把 V2 address 换成 V3 后重解释历史；切换时需保证已持久化交易的 nonce/outbox 安全约束，不重发或伪造旧交易状态。
- relayer／nonce：不同 Proxy 可共用同一链上 signer，但必须使用同一 sender 的全局 nonce fence 和 durable outbox 串行；若改用独立 signer，需单独身份、权限和 nonce cursor，不复用 V2 signer 槽。已持久签名只按原 txHash/nonce 对账，不另签或手工交易；`status=0` 先终结为 `final_revert`，再通过受控入口允许新 attempt。Safe 模式则另建 Safe 提案、签名、执行回执链路，不能假装它已进入 relayer outbox。
- V3 激活前校验链上模式／root／版本与审批已生效的快照一致；proof 服务能返回对应版本。成员更改在链上 finality/reorg 考量后才标记 effective，审批 `executed` 不等于链上 `applied`。DB／Job 中的旧版本应留存以支持历史区块和重组。

### M3 — 四眼审批、Admin 与资格服务（Backend／Admin）

- 后台只读预览输出候选地址规范化集合、diff、root、旧版本、新版本、链上项目状态及审批理由；地址去重，限单次规模，导入记录不得自动使用旧 Warmup `approved=1`。审批 payload 固定候选摘要及期望链上基线；审批时锁项目／版本并再次检查双方独立身份、实时权限、过期、状态和 root，过期计划或竞争更新 fail-closed。
- 根据 M0 治理方案，批准后进入 `approved_pending_execution`，Safe 或 EIP-712 收集真正的两份链上授权；链上执行及独立读回版本／root 后才 `effective`。Pending 关闭允许，Active 关闭和首开即使经过两人审批也拒绝；失败、超时、reorg 显示未生效并允许按 nonce/outbox 流程恢复。
- Admin 日常项目页以 V3 的版本、Proxy、当前模式／root 摘要／生效版本、候选差异、审批和链上收据为准；不为 V2 内测项目维持并行操作界面或新增收尾入口，历史记录不获得 V3 白名单动作。V3 新项目在 Admin Set／create／activate 的门禁清晰；不改变原有 Set 四眼。
- C 端资格／proof 接口只面向目标项目和已绑定钱包，返回准确的 V3 chainId/Proxy/root/version 与本钱包 proof；不泄露名单全集，更新后清掉旧版缓存。任何不一致时只允许安全失败，不可依赖旧报名表认定资格。

### M4 — C 端版本路由与用户流程（本分支）

- `fairLaunch.js`、`projectContext.js`、配置 ABI、列表／详情／面板按 `funding_mode + contract_version + chainId + contractAddress` 精确识别 V3；切换后 V2 不再作为新业务认购路径，不以旧内测项目为由要求保留并行钱包流程。未知版本或不受信 V3 地址只读／fail-closed，绝不落回 legacy/V2 地址。
- 已启用 V3 项目连接钱包后读取资格／proof 与链上当前模式、root、version：不合格禁用认购，合格执行 `purchaseWithProof`；公开 V3 项目无需 proof。root 变化时刷新或拒绝旧 proof，交易发送前校验绑定的链、Proxy、projectId、钱包及版本；一次签名／广播，后续回执和刷新只读。官方固定可信 RPC 优先，不重新引入钱包 BlockRazor 作为关键读取门禁。
- 为拒绝名单、过期 root、proof 获取失败等使用固定的本地化诊断；不暴露完整 calldata、名单全集、钱包 Provider 或任意 RPC 错误文本。Warmup 报名文案明确“申请／登记，不代表获得认购资格”。V3 钱包 approve、claim、refund 需完整验收；V2 旧认购入口关闭，内测资产如有遗留由项目方自行经原链上合约处理。

### M5 — 集成测试／发布准备（另行授权后才实施生产）

- 网络隔离的 Hardhat + 一次性完整 MySQL 验证 V3 创建→设置策略→资金充值→激活→资格购买→成员增删→结算→领取／退款，同时验证 V2 新业务入口关闭、V3 不误路由到 V2 Proxy 且不重解释旧数据；无需逐一核对 V2 内测项目的未结权益。覆盖直接合约调用、旧页面拒绝新业务、审批权限撤销、重放、签名人串通／绕过、gas、receipt `status=0/1`、RPC Pending、reorg、恢复、Job 停启和 V3 账本逐笔对账。
- 发布次序建议：受控关闭 V2 新业务入口 → 准备经审计的 V3 工件及批准的独立身份 → 默认禁用 V3 的 Backend/Job 变更与受控迁移 → V3 读链与对账影子校验 → 经单独生产授权部署 V3 新 Proxy → 核对 Proxy implementation/runtime code、owner/operator/Safe、chainId、startBlock、费率／代币策略 → Admin/C 端切换到 V3 唯一新业务入口。每一步有监测／停机／回退方案；不把 V2 内测资金提取或历史项目收尾作为 V3 发布门禁。
- **回退边界：** 尚无 V3 项目上链可关闭新项目入口并回退应用；已有 V3 交易或资金后不能换地址、删除账本或“回滚”链状态，必须继续为该 Proxy 提供事件扫描、提款／退款／领取和恢复能力。升级、生产 DB 写入、前端候选包、上传与部署分别审批，不因计划批准而自动执行。

## 3. 执行顺序、仓库与完成条件

| 里程碑 | 主仓库 | 前置门禁 | 可审查交付 |
| --- | --- | --- | --- |
| M0 | 跨仓库设计 | 产品确认状态机与治理强度 | 固定接口／身份／威胁模型／黄金向量 |
| M1 | `JoinCare_SmartContract-main` | M0 | 独立 V3 工件、ABI、单测、安全审查 |
| M2 | `joincare_server-master`、`joincare-deploy` | M1 ABI／事件冻结 | 迁移、版本路由、Core/Job/outbox/reorg 本地闭环 |
| M3 | `joincare_server-master`、`joincare-admin` | M2 生命周期与权限 | 双人审批、链上授权／生效状态、资格接口与 Admin 流程 |
| M4 | `joincare-web` (`feature/ido-whitelist`) | M1+M3 契约稳定 | V2 新业务入口关闭、V3 认购与 UI/单测／构建 |
| M5 | 各仓库 | M1–M4 | 全链路验收、独立发布决策与操作清单 |

允许并行开发测试桩，但不允许先发布只有前端准入、没有合约强制的版本。每仓库独立目标测试、完整相关测试和固定提交打包；保留现有无关工作区改动。任何实际合约部署、生产配置、DB 迁移或组件上线均需另行明确授权。

## 4. 待确认决策（未决则对应里程碑不能进入上线）

1. 是否确认 Active 为模式冻结点、Pending 可经审批关闭？默认如状态表。
2. Active 成员是否允许**增和删**，且立即影响后续区块；是否需要时间锁／速率或批量上限？默认允许增删，批量上限在 M0 定稿。
3. 链上四眼采用专用多签执行还是合约原生 EIP-712 双签＋relayer？如何确保 V3 UUPS upgrade 权限同样受控？此项是安全设计门禁。
4. 白名单是否只判定钱包地址，不附加个人额度／轮次／邀请码？本计划只做地址资格。
5. 已在 Setup/Set 但尚未上链的旧草稿是否允许主动重新选择 V3？默认只开放**新建且未完成 Set 的项目**，不改已上链 V2。
6. Warmup 自助报名是否仅作导入候选，需要独立人工审查？默认不自动导入，也不公开名单全集。
