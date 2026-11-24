# JoinCare IDO Fair Launch 模式详细开发设计

> 文档状态：Gate A-D / E1-E8 已冻结；候选技术 Freeze 为 FROZEN / CONDITIONAL GO；Owner 已确认法务、签名 POC、审计与恢复演练是部署门禁，不阻断代码开发；部署/生产仍为 NO-GO
> 适用范围：智能合约、Backend、Job、链事件、资金账本、C 端、Admin、部署与验收
> 目标网络：首期仅 BNB Smart Chain 主网 `chainId=56`，数据模型保留多链复合身份
> 现有模式：`capped_presale` / `v2_stablecoin`
> 新增模式：`fair_launch` / `fair_launch_v1`

---

## 0. Gate A-D 冻结决策与实施边界

本节记录 Owner 在多通道安全审计后确认的最终决策。本节优先于本文其他章节；如后文存在与本节不一致的旧描述，以本节为准，并必须在编码前删除旧描述。安全审计详见：

```text
docs/IDO-FAIR-LAUNCH-DEVELOPMENT-PLAN-SECURITY-AUDIT.md
```

### 0.1 当前 Gate 状态

```text
Gate A 产品决策：FROZEN
Gate A 法务签署：PENDING，部署/启用真实资金前必须完成
Gate B 合约安全决策：FROZEN，E1-E8 已冻结；§18.4 机械复核已通过
Gate C Backend/Job 决策：FROZEN，等待实现、POC 和恢复演练
Gate D Frontend/Admin 决策：FROZEN，等待实现和金融 UX 验收
候选技术 Freeze：FROZEN / CONDITIONAL GO

当前总体结论：NO-GO
```

§18.4 机械复核已经通过并冻结候选技术基线。Owner 决定 Gate A 法务、签名 POC、独立审计和恢复演练属于部署门禁，不再阻断代码开发；§18.5 完成前仍禁止迁移应用、部署、启用写链或真实资金。因此：

- 候选 ABI、事件、storage layout及机械 evidence可作为受限 Phase-1 开发基线，但不得宣称为生产部署基线；
- 禁止部署任何 Fair Launch Proxy、implementation或生产治理组件；
- 允许完成生产候选代码、可执行但显式 gated 的 migration、配置模板、离线部署/升级脚手架、CI、Recovery Portal和运维工具；
- 所有运行时功能必须 disabled-by-default，部署/迁移/签名/广播必须由独立 attestation、change ticket、checksum和 §18.5 门禁显式放行。

### 0.2 Gate A：产品、经济与法律决策

已冻结：

1. Fair Launch 为完全公开、permissionless 钱包参与，不做 KYC/身份 allowlist；
2. `Fair` 仅表示所有参与者使用相同最终清算价格，不表示反 Sybil、反鲸鱼、身份公平或防关联方；
3. 无 Hard Cap；
4. 接受无法技术识别项目方关联钱包的残余风险，仅通过强制披露、Admin 尽调、异常贡献监控和用户风险提示管理；
5. Active 阶段用户贡献不可撤回，只有 Failed/Cancelled 可退款；
6. 接受极端情况下极小贡献经向下取整后最终分配为 0 raw unit，并要求签名前明确披露；
7. 用户 Claim 无 V1 协议截止且无 sweep；
8. Failed/Cancelled 用户本金退款无 V1 协议截止；
9. 提供 permissionless `refundFor(projectId, user)`，资金只能退给链上记录的 user；
10. Pending 可按审批规则取消；Active 且 `now < endTime` 仅平台紧急治理可取消；`now >= endTime` 后取消必须 revert，只允许 permissionless `endProject`；
11. V1 不做 Pancake 自动流动性，Claim 不依赖人工流动性完成；
12. 项目 Successful 后允许项目方立即提款，不设挑战期；
13. 部署或启用真实资金前必须取得法务书面意见，至少覆盖 Fair 定义、不可撤回、关联方自募资、永久 Claim/Refund、Manual Liquidity 和立即提款。

Owner 明确接受但不能被描述为“已消除”的高残余风险：

```text
关联钱包自募资和虚假融资额
末期鲸鱼持续稀释
项目成功后项目方立即取得净募集款
无自动流动性和无 LP 锁仓保证
稳定币发行方冻结
极小贡献最终分配为 0 raw unit
```

### 0.3 Gate B：合约与资金安全决策

已冻结：

- 项目 Token 使用严格准入：V1 不接受 Proxy、可变税、rebase、pause、blacklist、可变转账规则或保留增发权的 Token；
- 保存并公示 Token runtime code hash、decimals、supply、权限和验证区块，在创建、充值、激活、贡献、结算、提款和 Claim 复核；
- 项目 Token 可分多次充值，但 `totalDeposited <= saleTokenAmount`，只有精确相等才可激活；
- 支付 Token 仅限平台全局受信稳定币 allowlist，禁止 fee-on-transfer、rebase 和原生 BNB；
- `minContribution` 是每笔最低贡献且必须大于 0；`maxContribution` 是单钱包累计上限，0 表示不限；
- Vesting 为 1~10 期、时间严格递增、单期比例大于 0、合计 10,000 bps，首期可等于 `endTime`；
- Claim 无 V1 协议截止，使用累计 entitlement 减已领取量，不提供未领取用户权益 sweep；
- 提供 permissionless `claimFor(projectId, user)`，Token 只能发送给 user；
- Pause 拆为 `contributionsPaused`、`payoutsPaused`、`claimsPauseUntil`；Claim Pause 单次最长 7 天；普通 Pause 永不阻止 `endProject/refund/refundFor`；
- Pause 不停止或延长销售时钟；
- Failed/Cancelled 项目方可取回全部项目 Token；Successful 的 `saleTokenAmount` 在 V1 中不得 sweep并持续记账保障未 Claim 权益，但可执行性受 §0.7 E5 限定；
- 禁止正常业务超额充值，因此 Successful 不需要超额项目币取回路径；
- 意外转入的非 reserve Token 只能由治理多签恢复到固定 Treasury，并证明 `balanceAfter >= reservedBalances[token]`；
- 2-of-3 Governance Safe 是 proposer/canceller，48h TimelockController 是 UUPS owner 和唯一 Upgrade Authority；
- Pause Guardian 为独立 2-of-3 Safe；Unpause 使用 24 小时 timelock；
- 不允许紧急升级绕过 timelock；禁止 EOA Upgrade Owner 和 `renounceOwnership`；
- implementation constructor 调用 `_disableInitializers()`，Proxy 部署与 initialize 原子完成；
- Fair Launch 使用独立 Operator；取消只能由 Operator 通过 durable outbox 执行；
- 创建时快照 `projectOwner`、`projectProceedsReceiver`、`platformFeeBps`、`platformFeeReceiver` 和 payment Token；
- 项目方收益和平台费只能发送到快照 beneficiary；
- `endProject`、`claimFor`、`refundFor` permissionless；项目方收益和平台费提款只允许对应 beneficiary 发起。

### 0.4 Gate C：Backend、Job、链事件与签名架构

已冻结：

- 首期生产只支持 BSC 主网 `chainId=56`；
- 使用链级 canonical ingestion cursor + Source 级 materialization cursor；单 Source 故障不得阻塞其他合约；
- 持久化 Contract Source Registry 是 ABI/runtime 选路权威，未知地址或版本 fail closed；
- 链事件唯一身份为 `chain_id + tx_hash + tx_log_ordinal`；RPC block-global `logIndex` 与 block number/hash 只表示 canonical placement；
- 普通展示 5 confirmations，资金事实 finalized 20 confirmations；
- 生产使用不同供应商的 Primary/Secondary RPC，至少一个支持历史 `eth_getStorageAt`，禁止 `skipRuntimeVerify=true`；
- Fair Launch Operator 使用独立远程签名架构：`JoinCare Sign Policy Gateway + Consensys Web3Signer + AWS KMS secp256k1 非导出密钥`；
- Web3Signer 只签名，不管理 nonce、不广播；现有 durable outbox 是唯一 nonce、广播、receipt、recovery 和 applied 状态机；
- 策略网关独立验证 approval digest、chain、受信 Proxy、selector/calldata、nonce slot、runtime identity、Gas 和费用上限；
- nonce fence 始终为 `chain_id + sender_address`；`final_success` 不释放 active slot，canonical event applied 后才原子释放；
- permissionless 外部 `endProject` 抢先后，未签名 intent 标记 superseded；已签名/广播 submission 继续完成 nonce/receipt 收敛；
- Job 每 60 秒扫描，到期后 5 分钟未提交 Warning、15 分钟 Critical；链上 `block.timestamp/status` 是签名前权威；
- finalized canonical event 才能 materialize 最终 fund ledger；ledger append-only，reorg 使用 reversal；
- uint256 raw 使用无损十进制字符串，不以 `DECIMAL(65,18)` 作为交易或对账权威；
- reconciliation 覆盖项目、用户和 contract-token 聚合三层；提款和恢复前必须 `BALANCED`；
- 首个 Fair Launch 上链后禁止回滚到不支持 Fair 的 Backend/Job，只能使用 Fair-capable maintenance image；
- Operator 轮换前必须清空所有旧 signer 未决 nonce，禁止在 `broadcast_unknown` 时切换。

### 0.5 Gate D：C 端、Admin、钱包和财务 UX

已冻结：

- 合约版本穷举 `legacy_jcc/v2_stablecoin/fair_launch_v1`，未知版本不回落 Legacy ABI；
- 合约和客户端均显式验证项目存在；全零 tuple 不得解释为 Pending；
- 普通展示使用 finalized Backend projection，任何金融签名前直接链读、验证身份并 staticCall/estimateGas；
- API 返回 block number/hash、confirmations、observedAt、state source、finality 和 reconciliation state；
- 财务 raw 为十进制字符串，内部计算使用 bigint，禁止 JavaScript `Number/parseFloat`；
- 募资期间价格和分配只能显示 Estimated，并说明后续贡献会稀释；Fair Launch 页面完全不显示 Hard Cap 字段；
- 所有钱包写入使用可持久化交易状态机，非法 tx hash、receipt timeout 和 broadcast unknown 均 fail closed；
- Gas 正常估算加 25% buffer；业务 revert 不 fallback；只有明确 unsupported method、同 calldata staticCall 成功且 selector 有审核上限时才可 fallback；
- Approve 默认精确金额；每次 Contribution 签名前都显示完整链、合约、Token、金额和风险确认；
- 到期但未结算时向所有用户显示 permissionless Finalize 入口；
- 普通页面为自己 Claim/Refund，独立恢复工具提供 `claimFor/refundFor`；Claim/Refund 不依赖 JWT；
- API 用显式 `available/not_applicable/pending_settlement/unlimited/unavailable/invalid`，禁止用 0 混淆语义；
- Admin funding mode 为严格判别联合类型，Fair Launch 请求禁止 tokenPrice/hardCap 字段；
- Preview 有效期 15 分钟并绑定 canonical payload digest；审批通过后 2 小时内执行；配置/runtime 变化立即失效；
- Set、创建、取消、三类 Pause/Unpause、Operator、Token allowlist、runtime、费率、UUPS 和 recovery 均纳入四眼审批；
- 高危动作采用 initiator、approver、executor 三权分离；
- 主网上线前交付不依赖 Backend/JWT 的静态 Recovery Portal；
- 详情页和每次签名前提供繁中、简中、英文关键风险披露。
- E8-A 支付 Token 事故时，Active 页面明确显示“已暂停新增贡献、既有贡献归属未改变”；Pending Release 页面显示“Release 已阻断，等待 48h 治理裁决或 exact snapshot 恢复”，不得误显示 Failed/可退款；只有链上治理失效事件后才开放 Refund。

### 0.6 允许进入下一阶段的条件

> Owner 2026-09-07 边界修订：本节历史清单现解释为**部署/启用门禁**，不再解释为编码门禁。代码、测试、可执行但 gated 的 migration及离线运维脚手架可以完成；任何 migration apply、网络调用、签名、部署、runtime enable或真实资金操作仍须满足全部清单和 §18.5。

```text
1. Gate A 法务书面意见完成；
2. 本文所有与 §0 冲突的旧内容全部修订；
3. Sign Policy Gateway + Web3Signer + AWS KMS 完成无生产私钥的 POC；
4. 第二轮只读设计审计已完成，当前结论为 NO-GO；
5. 第二轮和第三轮只读审计均已完成，结论均为 NO-GO；
6. E8 已冻结；完成第三轮残留整改，执行短周期 freeze re-check并至少取得 CONDITIONAL GO；
7. 才允许冻结 ABI、事件和 storage layout并进入生产实现。
```

### 0.7 第二轮审计补充决策 E1-E7

Owner 已冻结：

1. **链上 Release Gate**：达到 Soft Cap 的项目在 `endProject` 后先进入 `SuccessfulPendingRelease`。至少经过 20 个区块确认、Token runtime 正常且 reconciliation attestation 完成后，由独立 Operator 通过 durable `fair_launch_release` 推进到 `Successful`；只有此后才开放 Claim、项目方收益和平台费提款。
2. **项目 Token runtime 异常终态**：
   - Pending/Active 到期前异常：停止贡献并允许紧急取消；
   - 到期且达到 Soft Cap但 runtime 客观不匹配：`Failed + TokenRuntimeInvalid`，用户永久退款，平台费为 0；
   - `SuccessfulPendingRelease` 阶段异常：客观 mismatch 可 permissionless fail；链上无法客观识别的风险由 48h Governance Timelock 失效处理；
   - 已正式 `Successful` 后不回滚为 Failed，只进入安全事故、Pause 和修复流程。
3. **治理拓扑**：
   - 2-of-3 Governance Safe 是 48h Governance/Upgrade Timelock 的 proposer/canceller；
   - 48h Timelock 是 UUPS owner 和 upgrade executor，并执行 Operator、Token policy、fee/runtime 和 recovery 治理；
   - 独立 24h Unpause Timelock 只执行三类 Unpause；
   - 独立 2-of-3 Pause Guardian 可立即执行三类 Pause，但不能 Unpause、升级、改参数或转移资产。
4. **Token Admission Attestation**：治理登记 `runtimeCodeHash/policyDigest/policyVersion/decimals`；项目创建时快照。合约验证可客观验证的 code hash、decimals、enabled/emergency 和余额差；不可通用链上识别的 mint/pause/blacklist/tax/rebase 由不可变证据包 attestation 覆盖。
5. **永久权益限定含义**：V1 无协议级 Claim/Refund 截止日期且无 sweep；可执行性仍受 Token 可转移性、钱包控制、链运行、临时 Pause 和公开升级治理约束，不宣传为任何情况下绝对保证。
6. **固定 Safe beneficiary**：用户 Claim/Refund 地址永不迁移；`projectProceedsReceiver` 和 `platformFeeReceiver` 必须是审核通过的固定 2-of-3 Safe，项目记录中不可修改。只有对应 Safe 可发起提款，资金只能返回自身地址；不允许 projectOwner、Admin 或 Operator 代提。
7. **Claim Pause 自动到期**：每次 `claimsPauseUntil` 最长为调用时起 7 天；到期自动恢复。继续暂停必须产生新的公开治理事件；24h Unpause Timelock 可提前解除。Refund 和 End 永不受影响。

### 0.8 第三轮补充决策 E8：支付 Token runtime/policy mismatch

第三轮四通道审计已完成，结论仍为 `NO-GO`。Owner 已确认 E8 采用推荐方案 A，冻结如下：

1. **Pending**：支付 Token runtime、decimals、项目快照 policy 或全局 emergency 状态不合格时禁止激活；已有项目可按 Pending 取消规则退出。
2. **Active 且 `block.timestamp < endTime`**：禁止新 Contribution；既有贡献不自动改变归属，项目仍可走现有 Operator 紧急取消路径。
3. **到期 Active**：`endProject` 仍按 Soft Cap 和项目 Token 客观状态确定结果。支付 Token mismatch 本身不得 permissionless 把项目改为 Failed；达到 Soft Cap且项目 Token 正常时进入 `SuccessfulPendingRelease`。
4. **SuccessfulPendingRelease**：支付 Token runtime/policy mismatch 或 emergency block 必须阻止 Release，但状态保持不变。只有 48h Governance Timelock 可基于 `reasonCode + evidenceHash` 选择：
   - 支付 Token 治理失效 → `Failed + PaymentTokenGovernanceInvalidation`，用户获得退款权、平台费为 0；或
   - 等待支付 Token 恢复为该项目创建时的精确 runtime/policy 快照，重新完成 finalized reconciliation 后继续 Release。
5. **不可改写项目快照**：治理不得为了放行旧项目而修改其 payment Token code hash、policy digest/version 或 decimals；全局批准新 policy revision 只影响新项目。
6. **Successful**：支付 Token 后续 mismatch 不得回滚项目为 Failed，也不影响项目 Token Claim。项目收益和平台费提款在 payment Token 不合格时 fail closed；不得改 receiver、绕过精确转账或重分类为用户退款。
7. **Refund 安全出口**：Failed/Cancelled 的 Refund 不因普通 Pause、policy mismatch 或 emergency flag 被协议主动关闭，但每次仍要求支付 Token 实际精确转账成功；若 Token 冻结、revert 或余额 delta 异常，整笔退款 revert，用户义务和 reserve 保持不变并可在 Token 恢复后重试。

E8 不改变 E1-E7，不允许 Operator、Admin 或项目方自行裁决支付币资金归属。第三轮整改仍需 freeze re-check，且 Gate A 法务和签名 POC 继续独立阻止部署和真实资金启用，但不阻止代码完成。

## 1. 背景与目标

JoinCare 当前 IDO V2 仅支持固定价格的软顶/硬顶预售：

```text
固定 tokenPrice
+ softCap
+ hardCap
+ 达到 hardCap 立即结束
```

目标是在不影响现有 IDO V2 项目和资金安全的前提下，新增类似 PinkSale Fair Launch 的模式：

```text
固定本轮销售代币总量 saleTokenAmount
+ 必填最低融资额 softCap
+ 不设置 hardCap
+ 募资期间价格随 totalRaised 浮动
+ 达到 softCap 后仍继续认购至 endTime
+ 募资结束后统一确定最终价格和用户分配
```

核心业务结果：

1. 所有参与者最终使用相同价格；
2. 早参与和晚参与不会获得不同单价；
3. 达到软顶表示“成功条件已满足”，不表示项目已经结算；
4. 项目只有在 `endTime` 后完成链上结算，才进入 `Successful`；
5. 未达到软顶则进入 `Failed`，参与者可以原额退款；
6. 不修改现有 `JoinCareIDOv2` 的 ABI、存储布局和历史项目行为。

---

## 2. 范围与非目标

### 2.1 第一阶段必须交付

- 独立 Fair Launch UUPS 合约和 Proxy；
- 稳定币支付；
- 项目币和支付币 allowlist；
- 固定 `saleTokenAmount`；
- 必填 `softCap`；
- 可选单次最低贡献和单钱包累计最高贡献；
- 无 Hard Cap，认购持续到 `endTime`；
- 结束成功/失败结算；
- 成功后的统一比例分配和 Vesting Claim；
- 失败/取消后的退款；
- 项目方募集款提款；
- 平台成功费提款；
- 项目币多项目储备隔离与守恒；
- durable outbox、自动到期结算、链事件、reorg、账本和 reconciliation；
- C 端和 Admin 完整支持；
- BSC 主网前全链路验收。

### 2.2 第一阶段不实现

- PancakeSwap 自动建池；
- 自动添加流动性；
- LP Token/NFT 自动锁仓；
- 动态增发项目币；
- Active 阶段修改销售代币总量；
- Active 阶段修改 Soft Cap、时间窗或贡献限制；
- 跨链聚合募资；
- 结算时遍历全部参与者；
- 在认购时承诺最终项目币数量。

### 2.3 后续扩展预留

后续版本可在不修改现有 IDO V2 的前提下新增：

```text
JoinCareLiquidityManager
├─ PancakeV2LiquidityAdapter
├─ PancakeV3LiquidityAdapter
└─ JoinCareLiquidityLocker
```

Fair Launch V1 需要预留流动性模式和存储扩展位，但首期只开放 `Manual`。

---

## 3. 双合约总体架构

```text
JoinCare Launchpad
│
├─ Capped Presale
│  ├─ contract: JoinCareIDOv2 Proxy
│  ├─ funding_mode: capped_presale
│  ├─ contract_version: v2_stablecoin
│  ├─ 固定价格
│  ├─ Soft Cap
│  └─ Hard Cap
│
└─ Fair Launch
   ├─ contract: JoinCareFairLaunchV1 Proxy
   ├─ funding_mode: fair_launch
   ├─ contract_version: fair_launch_v1
   ├─ 固定销售代币总量
   ├─ Soft Cap
   ├─ 无 Hard Cap
   └─ 结束后统一定价
```

### 3.1 架构原则

1. `JoinCareIDOv2` 保持不变；
2. Fair Launch 使用独立 Proxy、ABI、事件和存储；
3. 两个合约可以使用相同治理体系和 Platform Fee Receiver，但 Fair Launch V1 使用独立 Operator 地址；
4. 两个合约独立暂停，避免一个模式故障影响另一个模式；
5. Backend 依据 `contract_version` 选择合约客户端和业务公式；
6. C 端依据项目链身份选择 ABI 和写入方法；
7. Admin 对用户展示“募资模式”，不要求管理员手工选择合约地址；
8. 合约地址、Chain ID、费率和 Token allowlist 必须来自受信服务端配置；
9. 每个 Operator 的 durable nonce 域仍然是：

```text
chain_id + sender_address
```

不能按合约地址分别维护 nonce。同一 sender 即使未来管理多个合约，也必须共享同一 nonce 域；Fair Launch 使用独立 sender 后与 V2 自然隔离。

---

## 4. 术语和业务标识

| 名称 | 建议值 | 说明 |
|---|---|---|
| 募资模式 | `capped_presale` | 现有固定价格软顶/硬顶模式 |
| 募资模式 | `fair_launch` | 新增浮动价格模式 |
| 合约版本 | `v2_stablecoin` | 现有 `JoinCareIDOv2` |
| 合约版本 | `fair_launch_v1` | 新 `JoinCareFairLaunchV1` |
| 销售代币总量 | `sale_token_amount` | 成功后供所有参与者按比例分配的项目币总量 |
| 最低融资额 | `soft_cap` | 到期时判断成功或失败的支付币阈值 |
| 当前预估价格 | `estimated_token_price` | 以当前募集额计算，仅用于展示 |
| 最终价格 | `final_token_price` | 到期成功结算后冻结的统一价格 |
| 用户贡献 | `contribution` | 用户实际支付并托管在合约中的支付币 |
| 最终分配 | `final_allocation` | 成功后按用户贡献占比计算的项目币总量 |

业务项目 ID `projects.project_id` 保持全局唯一。项目链身份必须使用完整四元组：

```text
chain_id + contract_address + contract_version + project_id
```

禁止只用 `project_id` 关联链事件或资金事实。

---

## 5. 数学模型

以下计算全部使用 Token raw unit 和整数运算，禁止使用浮点数。

### 5.1 最终价格

成功结算后：

最终价格权威保存为有理数，而不是可能归零的单一整数：

```text
finalPriceNumeratorRaw   = finalTotalRaisedRaw
finalPriceDenominatorRaw = saleTokenAmountRaw
```

展示层结合 payment/project Token decimals 使用明确 scale 的高精度格式化。一整颗项目币对应多少支付币仅是展示结果，不得用舍入后的价格反推用户 entitlement。

示例：

```text
saleTokenAmount = 1,000,000 FOS2
finalTotalRaised = 25,000 USDT

final price = 25,000 / 1,000,000
            = 0.025 USDT/FOS2
```

### 5.2 用户最终分配

```text
finalAllocationRaw
= floor(userContributionRaw × saleTokenAmountRaw / finalTotalRaisedRaw)
```

示例：

```text
用户贡献 100 USDT
最终募集 25,000 USDT
销售总量 1,000,000 FOS2

用户最终分配
= 100 × 1,000,000 / 25,000
= 4,000 FOS2
```

### 5.3 募资期间当前预估价格

当 `totalRaisedRaw > 0`：

```text
estimatedPriceNumeratorRaw   = totalRaisedRaw
estimatedPriceDenominatorRaw = saleTokenAmountRaw
```

这是有理数展示输入，不得作为链上 entitlement 或最终财务事实。前端只使用 bigint/decimal string 进行格式化，不得使用 JavaScript `Number`。

### 5.4 用户输入后的预估分配

用户已有贡献时，前端预估必须使用累计贡献和加入本次输入后的项目总额：

```text
estimatedAllocationAfterInput
= floor(
    (existingUserContributionRaw + inputRaw)
    × saleTokenAmountRaw
    / (currentTotalRaisedRaw + inputRaw)
  )
```

页面必须标注：最终分配会随后续总募集额变化。

### 5.5 Soft Cap 完成率

```text
softCapProgressBps
= floor(totalRaisedRaw × 10,000 / softCapRaw)
```

显示可以超过 100%，例如 250%。进度条视觉宽度可封顶 100%，文字保留真实比例。

### 5.6 舍入规则

- 所有用户分配使用向下取整；
- `sum(userFinalAllocation) <= saleTokenAmount`；
- 不允许把全部舍入余量奖励给最后一个 Claim 用户，避免 Claim 顺序影响收益；
- 未被分配的舍入尘埃和用户尚未领取权益均留在合约 reserve；V1 不设置 Claim Deadline，也不提供 sweep。

---

## 6. 生命周期与状态语义

### 6.1 主生命周期

```text
PreLaunch / Setup
       │ Set 参数审批
       ▼
      Set
       │ durable fair_launch_create
       ▼
    Pending
       │ 项目方充值 saleTokenAmount
       │ durable fair_launch_activate
       ▼
     Active
       ├─ totalRaised < softCap
       │    状态：募资中 / 软顶未达成
       │
       ├─ totalRaised >= softCap
       │    链上仍为 Active
       │    派生标记：softCapReached=true
       │    状态：成功条件已满足 / 持续募资中
       │
       └─ endTime 到达
            │ durable fair_launch_end
            ├─ totalRaised >= softCap 且项目 Token runtime 正常
            │    → SuccessfulPendingRelease
            │    → 20 confirmations + reconciliation attestation
            │    → durable fair_launch_release
            │    → Successful
            ├─ totalRaised >= softCap 但项目 Token runtime 不匹配
            │    → Failed(TokenRuntimeInvalid)
            └─ totalRaised < softCap  → Failed

Pending / Active ──受控取消──> Cancelled
```

### 6.2 达到 Soft Cap 不得进入 Successful

达到 Soft Cap 后、在 `endTime` 前仍必须：

- 保持链上 `Active`；
- 继续接受贡献；
- 不允许 Claim；
- 不允许项目方提款；
- 不允许平台提费；
- 不冻结最终价格；
- 不生成最终项目币 entitlement。

只有 `endTime` 后先完成确定性链上结算，再经过链上 Release Gate，才进入 `Successful`。`SuccessfulPendingRelease` 已冻结最终价格和 entitlement，但尚不可 Claim 或提款。

### 6.3 到期但尚未链上结算

如果页面时间已结束，但链上仍为 `Active`：

```text
totalRaised >= softCap → ended_success_pending_settlement
totalRaised < softCap  → ended_failed_pending_settlement
```

这两个值仅是“链上仍 Active，但时间已结束”的派生展示阶段，不能开放 Claim、退款或提款。页面统一提示：

```text
等待链上结算 / Awaiting on-chain settlement
```

一旦链上 `endProject` 成功，权威 `chainStatus` 即为 `successful_pending_release` 或 `failed`，不得继续归入 `ended_*_pending_settlement`。`successful_pending_release` 是真实链上状态，不是时间派生状态；页面应显示“结算完成，等待安全释放”。

退款只对链上权威 `Failed` 或 `Cancelled` 开放。Wire/API/DB 统一使用 `cancelled`；仅展示层可按语言显示 “Canceled/Cancelled/已取消”。

---

## 7. 智能合约设计

### 7.1 新合约

建议新增：

```text
contracts/JoinCareFairLaunchV1.sol
```

采用：

- Solidity `^0.8.22`；
- OpenZeppelin UUPS；
- `Ownable2StepUpgradeable`；
- 独立三域 Pause 状态和受限治理函数；
- `ReentrancyGuardUpgradeable`；
- `SafeERC20`；
- `Math.mulDiv`；
- 独立 Proxy，不复用 IDO V2 Proxy。

### 7.2 状态结构

概念结构（仅解释字段含义，不作为 storage freeze；类型、顺序和完整字段以 §7.22 为唯一候选基线）：

```solidity
enum ProjectStatus {
    Pending,
    Active,
    SuccessfulPendingRelease,
    Successful,
    Failed,
    Cancelled
}

enum TerminalReason {
    None,
    SoftCapNotReached,
    CancelledByGovernance,
    TokenRuntimeInvalid,
    PaymentTokenGovernanceInvalidation
}

enum GovernanceInvalidationClass {
    ProjectTokenNonObjectiveRisk,
    ProtocolAccountingRisk,
    LegalOrComplianceHold
}

enum LiquidityMode {
    Manual,
    ReservedAuto
}

struct Project {
    uint256 projectId;
    address projectToken;
    address paymentToken;
    address projectOwner;
    address projectProceedsReceiver;
    bytes32 projectProceedsReceiverAttestationDigest;
    uint32 projectProceedsReceiverAttestationVersion;
    address platformFeeReceiver;
    bytes32 platformFeeReceiverAttestationDigest;
    uint32 platformFeeReceiverAttestationVersion;

    uint8 projectTokenDecimals;
    uint8 paymentTokenDecimals;
    uint16 platformFeeBps;
    ProjectStatus status;
    TerminalReason terminalReason;

    uint256 saleTokenAmount;
    uint256 softCap;
    uint256 minContribution;
    uint256 maxContribution;
    uint256 startTime;
    uint256 endTime;
    uint256 totalRaised;
    uint256 totalTokensDeposited;
    uint256 totalTokensClaimed;
    uint256 participantCount;

    uint256 finalTotalRaised;
    uint256 finalPriceNumerator;
    uint256 finalPriceDenominator;

    bytes32 projectTokenCodeHash;
    bytes32 projectTokenPolicyDigest;
    uint32 projectTokenPolicyVersion;

    uint256 settlementBlockNumber;
    uint256 settlementAnchorBlockNumber;
    bytes32 settlementAnchorBlockHash;
    bytes32 settlementTxHash;
    uint32 settlementTxLogOrdinal;
    bytes32 releaseManifestDigest;
    bytes32 releasePolicyDigest;
    uint32 releasePolicyVersion;

    bool fundsWithdrawn;
    bool platformFeeWithdrawn;
    bool failedProjectTokensWithdrawn;

    uint256[] vestingTimes;
    uint256[] vestingPercents;
}

struct UserPosition {
    uint256 contributionAmount;
    uint256 claimedTokens;
    bool refunded;
}
```

流动性预留建议放在独立 mapping，而不是以后改变 `Project` 既有字段顺序：

```solidity
struct LiquidityConfig {
    LiquidityMode mode;
    uint16 paymentBps;
    uint16 listingPriceMultiplierBps;
    uint64 lockDuration;
    address manager;
    bool provisioned;
}

mapping(uint256 => LiquidityConfig) private _liquidityConfigs;
```

第一阶段强制 `mode == Manual`，并保留 storage gap。

### 7.3 全局状态

```solidity
uint256 public constant BASIS_POINTS = 10_000;
uint256 public constant MAX_FEE_RATE = 1_000;
uint8 public constant MAX_SUPPORTED_DECIMALS = 18;

address public platformFeeReceiver;
uint16 public platformFeeBps;
address public operator;

mapping(address => PaymentTokenAdmission) public paymentTokenAdmissions;
mapping(address => mapping(uint32 => PaymentTokenPolicyRevision)) public paymentTokenPolicyRevisions;
mapping(address => ProjectTokenPolicy) public projectTokenPolicies;
mapping(address => uint256) public reservedBalances;
mapping(uint256 => Project) private _projects;
mapping(uint256 => mapping(address => UserPosition)) private _positions;
mapping(uint256 => mapping(address => bool)) public hasParticipated;
mapping(uint256 => bool) public projectExists;

bool public contributionsPaused;
bool public payoutsPaused;
uint64 public claimsPauseUntil;
```

不建议在链上保存可无限增长的参与者数组；参与者列表由事件索引和 Backend 分页提供。合约只保存 `participantCount`。

### 7.4 创建项目

建议接口：

```solidity
function createProject(
    uint256 projectId,
    address projectToken,
    address paymentToken,
    address projectOwner,
    address projectProceedsReceiver,
    bytes32 projectProceedsReceiverAttestationDigest,
    uint32 projectProceedsReceiverAttestationVersion,
    address expectedPlatformFeeReceiver,
    bytes32 expectedPlatformFeeReceiverAttestationDigest,
    uint32 expectedPlatformFeeReceiverAttestationVersion,
    uint16 expectedPlatformFeeBps,
    uint256 saleTokenAmount,
    uint256 softCap,
    uint256 minContribution,
    uint256 maxContribution,
    uint256 startTime,
    uint256 endTime,
    bytes32 expectedProjectTokenPolicyDigest,
    uint32 expectedProjectTokenPolicyVersion,
    bytes32 expectedPaymentTokenPolicyDigest,
    uint32 expectedPaymentTokenPolicyVersion,
    bytes32 expectedReleasePolicyDigest,
    uint256[] calldata vestingTimes,
    uint256[] calldata vestingPercents
) external;
```

本节展开参数仅用于说明；生产 selector 以 §7.20 的 `createProject(CreateProjectParams,uint64[],uint16[])` 为唯一候选 ABI，禁止同时实现两个 create overload。

创建校验：

- 仅独立 Fair Launch Operator 可创建，必须来自已批准的 durable outbox；治理 Owner/Safe 不得绕过 outbox 直接创建；
- `projectId > 0`；
- 项目不存在；
- Token、Owner、Project Proceeds Receiver、Platform Fee Receiver 地址非零；
- 两个 receiver 均在创建时通过 `approvedBeneficiarySafes`，其 attestation digest/version 与 calldata 期望值一致并快照进 Project/Event；
- 项目币与支付币地址不同；
- 两种 Token 均已 allowlist；
- 链上 runtime decimals 与配置一致；
- `saleTokenAmount > 0`；
- `softCap > 0`；
- `minContribution > 0`，表示每笔 Contribution 的最低金额；
- `maxContribution == 0` 表示不限；
- 若 max 非零，则 `maxContribution >= minContribution`；
- `startTime > block.timestamp`；
- `endTime > startTime`；
- Vesting 1~10 期，时间严格递增，每期比例大于 0，比例合计 10,000 bps；
- 第一 Vesting 时间可等于 `endTime`，但只有链上 Successful 后才可 Claim；
- 不设置 Claim Deadline，用户 Claim 无 V1 协议截止；
- Platform Fee Receiver 和 Success Fee Rate 必须与当前治理配置快照相等；
- 项目 Token 必须满足 §0.3 严格准入：不可升级、无税、无 rebase、无 pause/blacklist、无保留增发权；
- `softCap > 0` 与 `saleTokenAmount > 0` 已保证权威有理数价格非零；展示格式不作为 Solidity 创建约束。Backend/Frontend 使用至少 36 位小数工作精度、向下舍入，并在过小值时使用科学计数或“< 最小显示单位”，不得显示为真实 0。

### 7.5 项目币充值和激活

充值：

```solidity
function depositProjectTokens(uint256 projectId, uint256 amount)
    external nonReentrant;
```

约束：

- 仅 `Pending`；
- 仅链上记录的 `projectOwner`；
- Token 仍满足严格准入和 runtime identity；
- decimals、runtime code hash 和权限快照未漂移；
- `amount > 0`；
- `totalTokensDeposited + amount <= saleTokenAmount`；
- 必须精确到账，拒绝 fee-on-transfer/rebase 异常；
- 增加 `reservedBalances[projectToken]`。

激活：

```solidity
function activateProject(uint256 projectId) external;
```

仅独立 Fair Launch Operator 可激活，必须来自 durable outbox。项目方只负责 `depositProjectTokens`，不能自行激活。

第一阶段要求：

```text
totalTokensDeposited == saleTokenAmount
```

激活后 `saleTokenAmount` 不得修改，也不允许 Active 阶段把新增充值纳入销售分母。

### 7.6 用户贡献

建议接口：

```solidity
function contribute(uint256 projectId, uint256 paymentAmount)
    external nonReentrant;
```

校验：

- 项目为 `Active`；
- `contributionsPaused == false`；
- `startTime <= now < endTime`；
- `paymentAmount > 0`；
- 若 `minContribution > 0`，单次贡献不得低于该值；
- 若 `maxContribution > 0`，用户累计贡献不得超过该值；
- 不检查 Hard Cap；
- 不因达到 Soft Cap 自动结束；
- 支付币必须精确到账；
- 支付币和项目币 runtime identity 必须符合项目快照；
- 只累计支付金额，不计算或写入最终项目币数量。

核心逻辑：

```solidity
position.contributionAmount += paymentAmount;
project.totalRaised += paymentAmount;

if (!hasParticipated[projectId][msg.sender]) {
    hasParticipated[projectId][msg.sender] = true;
    project.participantCount += 1;
}
```

### 7.7 到期结算

```solidity
function endProject(uint256 projectId) external;
```

建议允许任何地址在到期后调用，因为结算结果完全由链上事实决定，不授予调用者自由裁量权。平台 Job 仍作为主要调用方并走 durable outbox。

结算必须是 O(1)，禁止遍历参与者：

```solidity
project.finalTotalRaised = project.totalRaised;

if (project.totalRaised >= project.softCap) {
    if (!_projectTokenRuntimeMatches(project)) {
        project.status = ProjectStatus.Failed;
        project.terminalReason = TerminalReason.TokenRuntimeInvalid;
    } else {
        project.status = ProjectStatus.SuccessfulPendingRelease;
        project.finalPriceNumerator = project.totalRaised;
        project.finalPriceDenominator = project.saleTokenAmount;
    }
} else {
    project.status = ProjectStatus.Failed;
    project.terminalReason = TerminalReason.SoftCapNotReached;
}
```

达到 Soft Cap 并不直接开放资产：

```solidity
function releaseSuccessfulProject(
    uint256 projectId,
    uint256 settlementBlockNumber,
    uint256 settlementAnchorBlockNumber,
    bytes32 settlementAnchorBlockHash,
    bytes32 settlementTxHash,
    uint32 settlementTxLogOrdinal,
    bytes32 releaseManifestDigest
) external;
```

仅独立 Fair Launch Operator 可调用，必须通过 durable outbox和 Sign Policy Gateway。校验：

- 项目为 `SuccessfulPendingRelease`；
- 参数 `settlementBlockNumber` 必须严格等于项目在 `endProject` 内保存的 `settlementBlockNumber`；
- `block.number >= settlementBlockNumber + FINALIZED_CONFIRMATIONS`，其中 V1 `FINALIZED_CONFIRMATIONS = 20`；确认数定义固定为 `currentBlock - settlementBlockNumber`，因此 settlement block 后第 20 个区块才满足；Backend/UI 必须使用同一公式，不能按包含 settlement block 的 `+1` 口径；
- `endProject` 在交易执行时保存 `settlementBlockNumber = block.number`，并保存永久可验证锚点 `settlementAnchorBlockNumber = block.number - 1` 与 `settlementAnchorBlockHash = blockhash(block.number - 1)`；后者在结算交易执行时由 EVM 客观取得且写入 storage，不依赖未来 256-block 窗口；
- Release 参数中的 settlement/anchor block number/hash 必须与项目 storage 精确相等。Backend manifest 还必须证明 canonical `FairLaunchEnded` receipt 的 block number 等于 `settlementBlockNumber`，其 parent hash 等于 `settlementAnchorBlockHash`；Solidity验证已保存锚点和 20-block 深度，Operator/Gateway 对 receipt/parent linkage 作 attestation；
- `settlementTxHash/settlementTxLogOrdinal` 是 canonical `FairLaunchEnded` 事件身份，由 Operator attestation 提交并写入链上审计承诺；Solidity 不能从当前调用直接读取历史 receipt/log，因此不虚假宣称自行证明它们；
- 项目 Token runtime 仍与项目快照一致且未被 emergency blocked；
- 支付 Token runtime、decimals、policy digest/version 仍与项目创建快照精确一致，且当前全局配置未 emergency blocked；不一致时 Release 必须 revert并等待 E8-A 恢复或 48h Timelock invalidation；
- 项目创建时快照的 `releasePolicyDigest/releasePolicyVersion` 必须匹配登记项且仍处于 Governance Timelock 启用状态；新 policy revision 只影响新项目，紧急 disable 可阻止引用旧 digest 的未 Release 项目继续释放；
- `releaseManifestDigest != 0`，并写入项目作为不可变 Operator attestation。该 digest 必须绑定 canonical JSON/CBOR release manifest：release policy digest、attester/operator 地址、chain/proxy/version/project、Ended event identity、settlement block number、保存的 parent anchor block number/hash、Source revision、runtime/policy snapshots、20-confirmation观察块、项目/钱包/contract-token reconciliation 结果和 `BALANCED` 结论；
- 一次性从 `SuccessfulPendingRelease` 进入 `Successful`；
- 不改变 finalTotalRaised、分配、Token 或 beneficiary。

安全边界必须准确表述：合约客观验证状态、20-block 深度、结算时永久保存的 parent-block anchor、runtime 和一次性转换；`BALANCED` 的计算真实性由 E1 已冻结的独立 Operator 身份作 attestation，并由 Sign Policy Gateway 约束，而不是 Solidity 自行重算 Backend ledger。这里没有新增第三个 attester 或新 Owner 决策。Gateway 只允许 release selector，要求 manifest digest 与已批准 durable intent、双 RPC canonical evidence 和 DB reconciliation snapshot 完全一致。直接 beneficiary 无法绕过链上 Release 状态，但 Operator/签名策略被攻破仍是已披露治理风险。

在 Pending Release 客观检测到 runtime mismatch 时：

```solidity
function failPendingReleaseForProjectTokenRuntimeMismatch(uint256 projectId) external;
```

该函数只处理**项目 Token** 客观 mismatch。任何地址均可调用，但合约必须自行验证 mismatch；结果为 `Failed + TokenRuntimeInvalid`，用户在 V1 中获得无协议截止退款权、平台费为 0。支付 Token mismatch 不得调用该 permissionless 路径；按 E8-A 保持 `SuccessfulPendingRelease`，只有 48h Governance Timelock 可调用 `invalidatePendingReleaseForPaymentToken` 裁决为 `Failed + PaymentTokenGovernanceInvalidation`。链上无法客观判断的其他风险由 48h Governance Timelock 调用通用 invalidation 方法，并绑定 reason/evidence hash。

### 7.8 Claim 和 Vesting

最终用户分配不在结算时批量写入，而是在 view/Claim 时计算：

```solidity
function calculateFinalAllocation(uint256 projectId, address user)
    public view returns (uint256);

function calculateClaimable(uint256 projectId, address user)
    public view returns (uint256);

function claim(uint256 projectId) external nonReentrant;
function claimFor(uint256 projectId, address user) external nonReentrant;
```

```text
finalAllocation
= floor(contribution × saleTokenAmount / finalTotalRaised)

vestedAllocation
= floor(finalAllocation × unlockedBps / 10,000)

claimable
= max(vestedAllocation - claimedTokens, 0)
```

Claim 条件：

- 链上状态为 `Successful`；
- `block.timestamp >= claimsPauseUntil`；
- `claimable > 0`；
- 每次 Claim 增加用户和项目的 `claimedTokens`；
- 项目币从 reserve 中精确转出；
- `claimFor` 只能把 Token 发送给参数 `user`，调用者不能改变接收人；
- Claim 无 V1 协议截止，且不提供 sweep；限定含义见 §0.7 E5。

### 7.9 无 V1 Claim 截止和舍入尘埃

用户成功项目权益在 V1 中没有协议截止。V1 不提供 `sweepUnclaimedProjectTokens`，项目方、平台和治理均不得取回 `saleTokenAmount - totalTokensClaimed`。这不是对 Token、钱包、链或未来治理在任何情况下永久可执行的绝对保证，限定含义见 §0.7 E5。

该余额可能同时包含用户尚未领取权益和向下取整尘埃；在 O(1) 模型下不得把它们混合后转给项目方。极少量不可分配尘埃允许永久留在合约 reserve。未来如需 Claim Deadline，必须通过新版本和独立法务、安全审计设计长期 Claim Escrow。

### 7.10 退款

```solidity
function refund(uint256 projectId) external nonReentrant;
function refundFor(uint256 projectId, address user) external nonReentrant;
```

仅链上状态为 `Failed` 或 `Cancelled` 时可退款：

```text
refundAmount = contributionAmount
```

禁止依据前端时间派生状态提前退款。

退款在 V1 中无协议截止且无 sweep。`refundFor` 可由任何地址调用，但支付币只能发送到链上记录的 `user`；不得允许调用者指定其他收款地址。Batch 只能是链下循环或有界辅助工具，不得成为用户唯一退款路径。可执行性限定见 §0.7 E5。

### 7.11 募集款和平台费提款

成功后：

```text
platformFee = floor(finalTotalRaised × platformFeeBps / 10,000)
projectOwnerProceeds = finalTotalRaised - platformFee
```

接口：

```solidity
function withdrawProjectProceeds(uint256 projectId) external nonReentrant;
function withdrawPlatformFee(uint256 projectId) external nonReentrant;
```

- `payoutsPaused == false`；
- 项目方收益只允许快照 `projectProceedsReceiver` Safe 发起，并只能转给该 Safe；
- 平台费只允许快照 `platformFeeReceiver` 发起，并只能转给该地址；
- 两项分别记录一次性提款标记；
- 只有链上 `Successful` 才可提款；20 confirmations、runtime 和 reconciliation 条件已经由前置 `releaseSuccessfulProject` 写入链上，不依赖 UI 自律；
- 后续自动流动性启用后，项目方收益需要扣除流动性资金预留。

### 7.12 项目币取回

失败/取消：项目方可以取回全部已充值项目币。

成功：

- V1 禁止充值超过 `saleTokenAmount`，因此不存在正常业务超额取回；
- `saleTokenAmount` 必须永久留在合约保障尚未 Claim 的用户；
- 不允许 sweep 用户未领取权益。

### 7.13 Pause 语义

必须拆分三个权限域：

```text
contributionsPaused：阻止新 Contribution
payoutsPaused：阻止项目方收益、平台费和资产 recovery
claimsPauseUntil：仅在确认 Claim 路径存在漏洞时临时阻止 Claim/claimFor，单次最长 7 天并自动过期
```

普通 Pause 永远不得阻止：

- `endProject`；
- `refund`；
- `refundFor`。

Pause 不停止、不顺延 `startTime/endTime`。如果暂停持续过久，只能在 `endTime` 前按 Active 紧急取消规则处理；`now >= endTime` 后 cancel 必须 revert并只能确定性结算。

### 7.14 Token 储备守恒

继续使用按 Token 聚合的：

```solidity
mapping(address => uint256) public reservedBalances;
```

每次流入增加 reserve，每次精确转出减少 reserve。任何时刻必须满足：

```text
IERC20(token).balanceOf(contract) >= reservedBalances[token]
```

同一个项目币可以服务多个 Fair Launch 项目，因此不能用全局 Token reserve 等于单项目储备作为审计条件。

### 7.15 事件设计

建议使用独立事件名，避免 Backend 将 Fair Launch 贡献误解为固定价格购买：

```solidity
event FairLaunchProjectCreated(...);
event FairLaunchTokensDeposited(...);
event FairLaunchActivated(...);

event FairLaunchContribution(
    uint256 indexed projectId,
    address indexed user,
    address indexed paymentToken,
    uint256 paymentAmount,
    uint256 userContribution,
    uint256 totalRaised
);

event FairLaunchEnded(
    uint256 indexed projectId,
    ProjectStatus status,
    TerminalReason terminalReason,
    address indexed paymentToken,
    uint256 finalTotalRaised,
    uint256 finalPriceNumerator,
    uint256 finalPriceDenominator,
    uint256 settlementBlockNumber,
    uint256 settlementAnchorBlockNumber,
    bytes32 settlementAnchorBlockHash
);

event FairLaunchCancelled(...);
event FairLaunchRefunded(...);
event FairLaunchClaimed(...);
event FairLaunchProjectProceedsWithdrawn(...);
event FairLaunchPlatformFeeWithdrawn(...);
event FairLaunchFailedProjectTokensWithdrawn(...);
event FairLaunchContributionsPaused(...);
event FairLaunchContributionsUnpaused(...);
event FairLaunchPayoutsPaused(...);
event FairLaunchPayoutsUnpaused(...);
event FairLaunchClaimsPaused(...);
event FairLaunchClaimsUnpaused(...);
event FairLaunchExcessTokensRecovered(...);
```

`FairLaunchContribution` 不得包含“最终项目币数量”。如果需要页面展示预估值，由 C 端或只读 API 计算，并明确标为估算。

### 7.16 合约错误

至少包含：

```text
ProjectAlreadyExists
ProjectDoesNotExist
InvalidProjectStatus
InvalidProjectParameters
ProjectNotStarted
ProjectEndedAlready
BelowMinimumContribution
ExceedsMaxContribution
UnauthorizedProjectOwner
InsufficientProjectTokens
NoContribution
AlreadyRefunded
NoTokensToClaim
AlreadyWithdrawn
UnsupportedTokenBehavior
TokenReserveInsolvent
ContributionsPaused
PayoutsPaused
ClaimsPaused
TokenRuntimeMismatch
CancellationWindowClosed
InvalidBeneficiary
```

### 7.17 UUPS 与治理

- 48h Governance/Upgrade Timelock 为 UUPS owner 和唯一 Upgrade Authority；2-of-3 Governance Safe 仅为 proposer/canceller，禁止单 EOA 和 `renounceOwnership`；
- 非紧急升级强制 48 小时 timelock，不允许紧急升级绕过；
- Pause Guardian 使用独立 2-of-3 Safe，只能执行受限 Pause，不能升级、改参数或转移资金；
- Unpause 由治理 Safe 执行并等待 24 小时 timelock；
- Operator 只负责通过 durable outbox 执行受控生命周期动作；
- Fair Launch Operator 与 IDO V2 Operator 分离；
- implementation constructor 必须 `_disableInitializers()`；
- Proxy 部署和 initialize 原子完成，禁止重复初始化；
- 升级前必须执行 storage layout 校验，不得调整既有字段顺序、类型或删除字段；
- V1 冻结 storage gap 大小；
- Safe/KMS/Signer 凭据不得进入 Backend、Job、Git、聊天、服务器明文配置或发布包；
- 合约部署、升级、治理均需留存 tx、receipt、implementation code hash、timelock 和多签验证记录。

### 7.18 取消状态机

```text
Pending:
  项目方或平台可在 Admin 发起取消申请；
  链上只能由独立 Operator 经 durable outbox 执行。

Active && block.timestamp < endTime:
  仅平台紧急治理可发起；
  必须四眼审批、reasonCode 和 evidenceHash。

Active && block.timestamp >= endTime:
  cancelProject 必须 revert CancellationWindowClosed；
  任何地址均可调用 endProject。
```

取消后全部参与者支付币在 V1 中无协议截止可退款、平台费为 0、项目方可取回全部项目 Token。到期后的交易排序不得改变成功或失败结果。

### 7.19 Token runtime 与资产恢复

项目 Token 必须在创建、充值、激活、贡献、结算、Release、项目方提款和 Claim 时复核 runtime identity。项目 Token 发生不一致时：

```text
Pending：禁止创建/激活，可取消并返还项目 Token
Active 且未到期：禁止新贡献，允许 Operator 紧急取消
到期且达到 Soft Cap：endProject → Failed(TokenRuntimeInvalid)
SuccessfulPendingRelease：客观 mismatch permissionless fail；主观风险走 48h governance invalidation
Successful：不回滚为 Failed；暂停受影响路径并进入事故/升级修复
所有 Failed/Cancelled：Refund/refundFor 在 V1 中无协议截止，平台费为 0
记录 TOKEN_RUNTIME_MISMATCH、reasonCode 和 evidenceHash
```

支付 Token 使用 E8-A 的独立矩阵，不能复用上述 permissionless 项目 Token fail：

```text
Pending：mismatch/emergency blocked → 禁止 activate，可按 Pending 取消
Active 且未到期：禁止新 Contribution；既有资金归属不自动改变，可走 Operator 紧急取消
到期 Active：endProject 不因支付 Token mismatch 自动 Failed；Soft Cap 达成且项目 Token 正常 → SuccessfulPendingRelease
SuccessfulPendingRelease：mismatch/emergency blocked → 阻止 Release并保持原状态
  48h Governance Timelock 可：
    A. invalidatePendingReleaseForPaymentToken → Failed(PaymentTokenGovernanceInvalidation)
    B. 不执行 invalidation，等待精确项目快照恢复并重新 finalized reconciliation 后 Release
Successful：永不回滚 Failed；Claim 不受支付 Token mismatch 影响；proceeds/fee transfer fail closed
Failed/Cancelled：Refund/refundFor 始终保留协议入口，但实际 transfer 必须精确成功；失败时 reserve/ledger/position 原子不变
```

治理“等待恢复”只接受项目创建时快照的 exact `runtimeCodeHash/policyDigest/policyVersion/decimals` 再次匹配；不得用新全局 policy 覆盖旧项目快照。`invalidatePendingReleaseForPaymentToken` 仅限 48h Governance Timelock、仅限 `SuccessfulPendingRelease`、仅在 payment Token mismatch/emergency blocked 时执行，并必须记录预期/观察 runtime、reasonCode 和 evidenceHash。

意外直接转入、且不属于任何 reserve 的 Token 可由治理多签恢复到固定 Treasury：

```text
recoverable = token.balanceOf(contract) - reservedBalances[token]
```

恢复必须 `nonReentrant`，接收人不可由调用时任意指定；转账后必须再次证明 `balanceAfter >= reservedBalances[token]`。不得恢复任何用户或项目 reserve。

### 7.20 规范性 Solidity ABI 候选基线

本节是第三轮设计审计使用并经本轮整改、E8-A 更新的**完整候选基线**，优先于 §7.1-§7.19 中的“建议接口”或伪代码。它不是生产 freeze：Gate A 法务、整改后的 freeze re-check 至少 `CONDITIONAL GO` 和签名 POC 完成前仍禁止部署或实现真实资金合约。

编译基线：Solidity `0.8.22`、OpenZeppelin Contracts/Contracts Upgradeable `5.1.0`、UUPS/ERC-1967。实现继承顺序固定为：

```solidity
contract JoinCareFairLaunchV1 is
    Initializable,
    UUPSUpgradeable,
    Ownable2StepUpgradeable,
    ReentrancyGuardUpgradeable
```

`owner()` 必须是 48h Governance/Upgrade `TimelockController`；`pauseGuardian` 和 `unpauseTimelock` 使用独立地址检查，不使用可混淆的通用 role fallback。

规范枚举和值：

```solidity
enum ProjectStatus {
    Pending,                   // 0
    Active,                    // 1
    SuccessfulPendingRelease, // 2
    Successful,                // 3
    Failed,                    // 4
    Cancelled                  // 5
}

enum TerminalReason {
    None,                    // 0
    SoftCapNotReached,       // 1
    CancelledByOperator,     // 2
    TokenRuntimeInvalid,     // 3
    GovernanceInvalidation,             // 4，非 Token 客观风险治理失效
    PaymentTokenGovernanceInvalidation  // 5，E8-A 48h Timelock 裁决
}

enum GovernanceInvalidationClass {
    ProjectTokenNonObjectiveRisk, // 0
    ProtocolAccountingRisk,       // 1
    LegalOrComplianceHold         // 2
}

struct PaymentTokenPolicyRevision {
    bool exists;
    uint8 decimals;
    uint32 policyVersion;
    bytes32 runtimeCodeHash;
    bytes32 policyDigest;
}

struct PaymentTokenAdmission {
    uint32 currentPolicyVersion;
    bool enabledForNewProjects;
    bool emergencyBlocked;
}

struct ProjectTokenPolicy {
    bool enabledForNewProjects;
    bool emergencyBlocked;
    uint8 decimals;
    uint32 policyVersion;
    bytes32 runtimeCodeHash;
    bytes32 policyDigest;
}

struct CreateProjectParams {
    uint256 projectId;
    address projectToken;
    address paymentToken;
    address projectOwner;
    address projectProceedsReceiver;
    bytes32 projectProceedsReceiverAttestationDigest;
    uint32 projectProceedsReceiverAttestationVersion;
    address expectedPlatformFeeReceiver;
    bytes32 expectedPlatformFeeReceiverAttestationDigest;
    uint32 expectedPlatformFeeReceiverAttestationVersion;
    uint16 expectedPlatformFeeBps;
    uint256 saleTokenAmount;
    uint256 softCap;
    uint256 minContribution;
    uint256 maxContribution;
    uint64 startTime;
    uint64 endTime;
    bytes32 expectedProjectTokenPolicyDigest;
    uint32 expectedProjectTokenPolicyVersion;
    bytes32 expectedPaymentTokenPolicyDigest;
    uint32 expectedPaymentTokenPolicyVersion;
    bytes32 expectedReleasePolicyDigest;
    uint32 expectedReleasePolicyVersion;
}
```

全文件统一使用 `platformFeeBps`，不再使用 `successFeeRate` 作为 Fair Launch wire/storage 名称。

初始化和所有 V1 自有 external/public selector：

```solidity
function initialize(
    address governanceTimelock,
    address initialOperator,
    address initialPauseGuardian,
    address initialUnpauseTimelock,
    address initialTreasury,
    address initialPlatformFeeReceiver,
    uint16 initialPlatformFeeBps
) external initializer;

function createProject(
    CreateProjectParams calldata params,
    uint64[] calldata vestingTimes,
    uint16[] calldata vestingBps
) external;
function depositProjectTokens(uint256 projectId, uint256 amount) external;
function activateProject(uint256 projectId) external;
function contribute(uint256 projectId, uint256 paymentAmount) external;
function endProject(uint256 projectId) external;
function releaseSuccessfulProject(
    uint256 projectId,
    uint256 settlementBlockNumber,
    uint256 settlementAnchorBlockNumber,
    bytes32 settlementAnchorBlockHash,
    bytes32 settlementTxHash,
    uint32 settlementTxLogOrdinal,
    bytes32 releaseManifestDigest
) external;
function failPendingReleaseForProjectTokenRuntimeMismatch(uint256 projectId) external;
function invalidatePendingRelease(
    uint256 projectId,
    GovernanceInvalidationClass invalidationClass,
    bytes32 reasonCode,
    bytes32 evidenceHash
) external;
function invalidatePendingReleaseForPaymentToken(
    uint256 projectId,
    bytes32 reasonCode,
    bytes32 evidenceHash
) external;
function cancelProject(uint256 projectId, bytes32 reasonCode, bytes32 evidenceHash) external;

function refund(uint256 projectId) external;
function refundFor(uint256 projectId, address user) external;
function claim(uint256 projectId) external;
function claimFor(uint256 projectId, address user) external;
function withdrawProjectProceeds(uint256 projectId) external;
function withdrawPlatformFee(uint256 projectId) external;
function withdrawFailedProjectTokens(uint256 projectId) external;

function pauseContributions(bytes32 reasonCode, bytes32 evidenceHash) external;
function unpauseContributions(bytes32 operationId) external;
function pausePayouts(bytes32 reasonCode, bytes32 evidenceHash) external;
function unpausePayouts(bytes32 operationId) external;
function pauseClaims(uint64 requestedUntil, bytes32 reasonCode, bytes32 evidenceHash) external;
function unpauseClaims(bytes32 operationId) external;

function setOperator(address newOperator) external;
function setPauseGuardian(address newPauseGuardian) external;
function setUnpauseTimelock(address newUnpauseTimelock) external;
function setTreasury(address newTreasury) external;
function setPlatformFee(uint16 newPlatformFeeBps, address newPlatformFeeReceiver) external;
function registerPaymentTokenPolicyRevision(address token, PaymentTokenPolicyRevision calldata revision) external;
function setPaymentTokenAdmission(address token, uint32 currentPolicyVersion, bool enabledForNewProjects) external;
function setPaymentTokenEmergencyBlocked(address token, bool emergencyBlocked) external;
function configureProjectToken(address token, ProjectTokenPolicy calldata policy) external;
function configureBeneficiarySafe(address account, bool approved, bytes32 attestationDigest, uint32 attestationVersion) external;
function configureReleasePolicy(bytes32 policyDigest, uint32 policyVersion, bool enabled) external;
function recoverExcessTokens(address token, uint256 amount) external;

function projectExists(uint256 projectId) external view returns (bool);
function getProject(uint256 projectId) external view returns (Project memory);
function getPosition(uint256 projectId, address user) external view returns (UserPosition memory);
function getVestingSchedule(uint256 projectId)
    external view returns (uint64[] memory times, uint16[] memory bps);
function calculateFinalAllocation(uint256 projectId, address user) public view returns (uint256);
function calculateClaimable(uint256 projectId, address user) public view returns (uint256);
function isClaimsPaused() public view returns (bool);
function projectTokenRuntimeMatches(uint256 projectId) external view returns (bool);
function paymentTokenRuntimeMatches(uint256 projectId) external view returns (bool);
function contractVersion() external pure returns (bytes32); // keccak256("fair_launch_v1")
function operator() external view returns (address);
function pauseGuardian() external view returns (address);
function unpauseTimelock() external view returns (address);
function treasury() external view returns (address);
function platformFeeConfig() external view returns (uint16 feeBps, address receiver);
function paymentTokenAdmission(address token) external view returns (PaymentTokenAdmission memory);
function paymentTokenPolicyRevision(address token, uint32 policyVersion) external view returns (PaymentTokenPolicyRevision memory);
function projectTokenPolicy(address token) external view returns (ProjectTokenPolicy memory);
function reservedBalance(address token) external view returns (uint256);
function beneficiarySafeConfig(address account) external view returns (bool approved, bytes32 attestationDigest, uint32 attestationVersion);
function releasePolicyConfig(bytes32 policyDigest) external view returns (bool enabled, uint32 policyVersion);
```

继承 ABI 同时包含 OZ V5 的 `owner/pendingOwner/transferOwnership/acceptOwnership/upgradeToAndCall/proxiableUUID/UPGRADE_INTERFACE_VERSION`。`renounceOwnership()` 必须 override 并始终 revert；`_authorizeUpgrade` 必须 `onlyOwner`，即只有 48h Timelock 能执行升级。Canonical event manifest 分为 `v1OwnedEvents` 和 inherited OZ events；后者至少包含 `Initialized(uint64)`、`OwnershipTransferred(address,address)`、`OwnershipTransferStarted(address,address)`、`Upgraded(address)`，CI 两部分均从同一编译产物生成并分别校验。

Payment Token policy revision 是 append-only：`registerPaymentTokenPolicyRevision` 只能写入从未存在的 `(token,policyVersion)`，不得更新或删除；`setPaymentTokenAdmission` 只移动“新项目当前采用版本”的指针并要求 revision 已存在；`setPaymentTokenEmergencyBlocked` 是独立当前紧急开关。项目始终按创建时 `paymentTokenPolicyVersion` 查询不可变 revision，再叠加当前 emergency block。这样新版本只影响新项目，而旧项目仍可验证和恢复其 exact snapshot。

权限和暂停矩阵：

| 函数族 | Caller | 普通 Pause 行为 |
|---|---|---|
| create/activate/release/cancel | `operator` 精确地址 | create/activate 受 contributions pause；release/cancel 不受 payouts pause，Release 只开放 Claim 状态，收益提款仍由 payouts pause 独立阻止 |
| deposit | `projectOwner` | contributions pause 时阻止 |
| contribute | 任意用户 | contributions pause 时阻止 |
| end/project-token objective fail | permissionless | 三类 Pause 均不阻止；payment Token mismatch 不允许 permissionless fail |
| payment-token governance invalidation | 48h Governance Timelock | 三类 Pause 均不阻止；仅 `SuccessfulPendingRelease` 可执行 |
| governance invalidation/config/upgrade/recovery | 48h Timelock owner | recovery 受 payouts pause，其余按 selector 治理 |
| pause* | `pauseGuardian` Safe | 可立即调用 |
| unpause* | `unpauseTimelock` | 仅 24h Timelock 可调用 |
| claim/claimFor | 任意地址 | `block.timestamp < claimsPauseUntil` 时阻止 |
| refund/refundFor | 任意地址 | 永不被普通 Pause 阻止 |
| proceeds/fee | 对应快照 receiver Safe | payouts pause 时阻止 |
| failed project Token | `projectOwner`，只转回 projectOwner | payouts pause 时阻止 |

View 语义冻结：项目不存在一律 revert `ProjectDoesNotExist`，不得返回全零 tuple；非 `Successful` 时 `calculateFinalAllocation/calculateClaimable` 返回 0；无贡献或真实向下取整为 0 也返回 0，但 API 必须用项目状态和 contribution 字段区分原因；Claim pause 不改变只读 claimable 数值，只通过 `isClaimsPaused()` 暴露可执行性。

`GovernanceInvalidationClass` 枚举必须明确排除 payment Token runtime/policy/emergency 类；这些证据只能走 `invalidatePendingReleaseForPaymentToken`。Gateway、Admin Preview和 Timelock policy 必须拒绝把 payment-token evidence 编码进通用 invalidation selector，确保 E8 专属事件和证据 schema不可旁路。

### 7.21 完整事件和错误候选基线

```solidity
event FairLaunchProjectCreated(
    uint256 indexed projectId,
    address indexed projectToken,
    address indexed paymentToken,
    address projectOwner,
    address projectProceedsReceiver,
    bytes32 projectProceedsReceiverAttestationDigest,
    uint32 projectProceedsReceiverAttestationVersion,
    address platformFeeReceiver,
    bytes32 platformFeeReceiverAttestationDigest,
    uint32 platformFeeReceiverAttestationVersion,
    uint16 platformFeeBps,
    uint256 saleTokenAmount,
    uint256 softCap,
    uint256 minContribution,
    uint256 maxContribution,
    uint64 startTime,
    uint64 endTime,
    bytes32 projectTokenPolicyDigest,
    uint32 projectTokenPolicyVersion,
    bytes32 paymentTokenPolicyDigest,
    uint32 paymentTokenPolicyVersion,
    bytes32 releasePolicyDigest,
    uint32 releasePolicyVersion
);
event FairLaunchTokensDeposited(uint256 indexed projectId, address indexed projectToken, address indexed from, uint256 amount, uint256 totalDeposited);
event FairLaunchActivated(uint256 indexed projectId);
event FairLaunchContribution(uint256 indexed projectId, address indexed user, address indexed paymentToken, uint256 paymentAmount, uint256 userContribution, uint256 totalRaised);
event FairLaunchEnded(uint256 indexed projectId, ProjectStatus status, TerminalReason terminalReason, address indexed paymentToken, uint256 finalTotalRaised, uint256 finalPriceNumerator, uint256 finalPriceDenominator, uint256 settlementBlockNumber, uint256 settlementAnchorBlockNumber, bytes32 settlementAnchorBlockHash);
event FairLaunchSuccessfulReleased(uint256 indexed projectId, uint256 indexed settlementBlockNumber, address indexed attester, uint256 settlementAnchorBlockNumber, bytes32 settlementAnchorBlockHash, bytes32 settlementTxHash, uint32 settlementTxLogOrdinal, bytes32 releaseManifestDigest, bytes32 releasePolicyDigest, uint32 releasePolicyVersion);
event FairLaunchPendingReleaseInvalidated(uint256 indexed projectId, TerminalReason terminalReason, GovernanceInvalidationClass invalidationClass, bytes32 reasonCode, bytes32 evidenceHash, address indexed caller);
event FairLaunchPaymentTokenInvalidated(uint256 indexed projectId, address indexed paymentToken, address indexed timelock, bytes32 expectedRuntimeCodeHash, bytes32 observedRuntimeCodeHash, bytes32 expectedPolicyDigest, uint32 expectedPolicyVersion, bytes32 observedPolicyDigest, uint32 observedPolicyVersion, bool emergencyBlocked, bytes32 reasonCode, bytes32 evidenceHash);
event FairLaunchCancelled(uint256 indexed projectId, ProjectStatus previousStatus, bytes32 reasonCode, bytes32 evidenceHash, address indexed operator);
event FairLaunchRefunded(uint256 indexed projectId, address indexed user, address indexed paymentToken, uint256 amount, address caller);
event FairLaunchClaimed(uint256 indexed projectId, address indexed user, address indexed projectToken, uint256 amount, uint256 totalUserClaimed, address caller);
event FairLaunchProjectProceedsWithdrawn(uint256 indexed projectId, address indexed paymentToken, address indexed projectProceedsReceiver, uint256 amount, uint256 platformFeeReserved);
event FairLaunchPlatformFeeWithdrawn(uint256 indexed projectId, address indexed paymentToken, address indexed platformFeeReceiver, uint256 amount);
event FairLaunchFailedProjectTokensWithdrawn(uint256 indexed projectId, address indexed projectToken, address indexed projectOwner, uint256 amount);
event FairLaunchContributionsPaused(address indexed guardian, bytes32 reasonCode, bytes32 evidenceHash);
event FairLaunchContributionsUnpaused(address indexed timelock, bytes32 operationId);
event FairLaunchPayoutsPaused(address indexed guardian, bytes32 reasonCode, bytes32 evidenceHash);
event FairLaunchPayoutsUnpaused(address indexed timelock, bytes32 operationId);
event FairLaunchClaimsPaused(address indexed guardian, uint64 pauseUntil, bytes32 reasonCode, bytes32 evidenceHash);
event FairLaunchClaimsUnpaused(address indexed timelock, bytes32 operationId);
event FairLaunchOperatorUpdated(address indexed previousOperator, address indexed newOperator);
event FairLaunchPauseGuardianUpdated(address indexed previousGuardian, address indexed newGuardian);
event FairLaunchUnpauseTimelockUpdated(address indexed previousTimelock, address indexed newTimelock);
event FairLaunchTreasuryUpdated(address indexed previousTreasury, address indexed newTreasury);
event FairLaunchPlatformFeeUpdated(uint16 previousFeeBps, uint16 newFeeBps, address indexed previousReceiver, address indexed newReceiver);
event FairLaunchPaymentTokenPolicyRevisionRegistered(address indexed token, uint32 indexed policyVersion, uint8 decimals, bytes32 runtimeCodeHash, bytes32 policyDigest);
event FairLaunchPaymentTokenAdmissionUpdated(address indexed token, uint32 indexed currentPolicyVersion, bool enabledForNewProjects);
event FairLaunchPaymentTokenEmergencyBlockUpdated(address indexed token, bool emergencyBlocked);
event FairLaunchProjectTokenConfigured(address indexed token, bool enabledForNewProjects, bool emergencyBlocked, uint8 decimals, uint32 policyVersion, bytes32 runtimeCodeHash, bytes32 policyDigest);
event FairLaunchBeneficiarySafeConfigured(address indexed account, bool approved, bytes32 attestationDigest, uint32 attestationVersion);
event FairLaunchReleasePolicyConfigured(bytes32 indexed policyDigest, uint32 policyVersion, bool enabled);
event FairLaunchExcessTokensRecovered(address indexed token, address indexed treasury, uint256 amount);
```

V1 自有 custom errors 必须完整定义，禁止同一条件有多种字符串错误：

```solidity
error InvalidAddress();
error InvalidAmount();
error InvalidFeeBps(uint16 actual, uint16 maximum);
error InvalidTokenDecimals(uint8 decimals);
error InvalidProjectParameters();
error InvalidVestingSchedule();
error ProjectAlreadyExists(uint256 projectId);
error ProjectDoesNotExist(uint256 projectId);
error InvalidProjectStatus(ProjectStatus expected, ProjectStatus actual);
error UnauthorizedOperator(address caller);
error UnauthorizedPauseGuardian(address caller);
error UnauthorizedUnpauseTimelock(address caller);
error UnauthorizedProjectOwner(address caller);
error UnauthorizedBeneficiary(address caller, address expected);
error BeneficiarySafeNotApproved(address account);
error BeneficiarySafeAttestationMismatch(address account, bytes32 expectedDigest, uint32 expectedVersion, bytes32 actualDigest, uint32 actualVersion);
error TokenNotConfigured(address token);
error TokenNotAllowed(address token);
error TokenEmergencyBlocked(address token);
error TokenHasNoCode(address token);
error TokenRuntimeMismatch(address token, bytes32 expected, bytes32 actual);
error TokenDecimalsChanged(address token, uint8 expected, uint8 actual);
error TokenPolicySnapshotChanged(address token, bytes32 expectedDigest, uint32 expectedVersion, bytes32 actualDigest, uint32 actualVersion);
error PaymentTokenPolicyRevisionAlreadyExists(address token, uint32 policyVersion);
error PaymentTokenPolicyRevisionDoesNotExist(address token, uint32 policyVersion);
error ProjectNotStarted(uint64 startTime);
error ProjectNotEnded(uint64 endTime);
error ProjectEndedAlready(uint64 endTime);
error BelowMinimumContribution(uint256 minimum, uint256 actual);
error ExceedsMaxContribution(uint256 maximum, uint256 resultingTotal);
error ExceedsSaleTokenAmount(uint256 saleTokenAmount, uint256 resultingDeposit);
error InsufficientProjectTokens(uint256 required, uint256 deposited);
error UnsupportedTokenBehavior(address token, uint256 requested, uint256 observed);
error UnsupportedOutgoingTokenBehavior(address token, uint256 requested, uint256 senderDecrease, uint256 recipientIncrease);
error TokenReserveInsolvent(address token, uint256 balance, uint256 reserved);
error NoContribution(address user);
error AlreadyRefunded(address user);
error NoTokensToClaim(address user);
error AlreadyWithdrawn();
error ContributionsArePaused();
error PayoutsArePaused();
error ClaimsArePaused(uint64 pauseUntil);
error InvalidClaimsPauseUntil(uint64 requestedUntil, uint64 maximumUntil);
error CancellationWindowClosed(uint64 endTime);
error InvalidReleaseEvidence();
error SettlementAnchorMismatch(uint256 expectedBlockNumber, bytes32 expectedBlockHash, uint256 actualBlockNumber, bytes32 actualBlockHash);
error ReleaseConfirmationDepthNotReached(uint256 requiredBlock, uint256 currentBlock);
error InvalidReconciliationDigest();
error ReleasePolicyNotEnabled(bytes32 policyDigest);
error ReleasePolicySnapshotMismatch(bytes32 expectedDigest, uint32 expectedVersion, uint32 actualVersion);
error ProjectTokenRuntimeStillMatches();
error PaymentTokenNotInvalidatable();
error PaymentTokenGovernanceInvalidationRequired();
error InvalidGovernanceInvalidationClass(GovernanceInvalidationClass invalidationClass);
error ExcessiveRecoveryRequested(address token, uint256 recoverable, uint256 requested);
error OwnershipRenunciationDisabled();
```

Backend、Recovery Portal、Gateway allowlist 和 indexer 必须由同一编译产物生成 function selector/event topic manifest；禁止手抄常量。CI 比较 ABI JSON、selector manifest、event topic manifest 和本节 canonical signature，任一漂移即失败。

### 7.22 V1 namespaced storage 候选基线

使用 ERC-7201 风格 namespaced storage，避免与 OZ V5 namespaced parent storage 碰撞。命名空间固定为 `joincare.storage.JoinCareFairLaunchV1`，位置固定为：

```text
0x7ebb4c309dc14c823b32233ea7cfb602ee106e1bc7e105581a751cd0bcb46700
```

`Project` 字段和根 storage 的顺序均不可重排、改类型或删除：

```solidity
struct Project {
    uint256 projectId;
    address projectToken;
    address paymentToken;
    address projectOwner;
    address projectProceedsReceiver;
    bytes32 projectProceedsReceiverAttestationDigest;
    uint32 projectProceedsReceiverAttestationVersion;
    address platformFeeReceiver;
    bytes32 platformFeeReceiverAttestationDigest;
    uint32 platformFeeReceiverAttestationVersion;
    uint8 projectTokenDecimals;
    uint8 paymentTokenDecimals;
    uint16 platformFeeBps;
    ProjectStatus status;
    TerminalReason terminalReason;
    uint32 projectTokenPolicyVersion;
    uint32 paymentTokenPolicyVersion;
    uint64 startTime;
    uint64 endTime;
    uint256 saleTokenAmount;
    uint256 softCap;
    uint256 minContribution;
    uint256 maxContribution;
    uint256 totalRaised;
    uint256 totalTokensDeposited;
    uint256 totalTokensClaimed;
    uint256 participantCount;
    uint256 finalTotalRaised;
    uint256 finalPriceNumerator;
    uint256 finalPriceDenominator;
    uint256 settlementBlockNumber;
    uint256 settlementAnchorBlockNumber;
    bytes32 settlementAnchorBlockHash;
    bytes32 settlementTxHash;
    uint32 settlementTxLogOrdinal;
    bytes32 releaseManifestDigest;
    bytes32 projectTokenCodeHash;
    bytes32 projectTokenPolicyDigest;
    bytes32 paymentTokenCodeHash;
    bytes32 paymentTokenPolicyDigest;
    bytes32 releasePolicyDigest;
    uint32 releasePolicyVersion;
    bool projectProceedsWithdrawn;
    bool platformFeeWithdrawn;
    bool failedProjectTokensWithdrawn;
}

struct UserPosition {
    uint256 contributionAmount;
    uint256 claimedTokens;
    bool refunded;
}

/// @custom:storage-location erc7201:joincare.storage.JoinCareFairLaunchV1
struct FairLaunchStorage {
    address operator;
    address pauseGuardian;
    address unpauseTimelock;
    address treasury;
    address platformFeeReceiver;
    uint16 platformFeeBps;
    bool contributionsPaused;
    bool payoutsPaused;
    uint64 claimsPauseUntil;
    mapping(address => PaymentTokenAdmission) paymentTokenAdmissions;
    mapping(address => mapping(uint32 => PaymentTokenPolicyRevision)) paymentTokenPolicyRevisions;
    mapping(address => ProjectTokenPolicy) projectTokenPolicies;
    mapping(address => uint256) reservedBalances;
    mapping(address => bool) approvedBeneficiarySafes;
    mapping(address => bytes32) beneficiarySafeAttestations;
    mapping(address => uint32) beneficiarySafeAttestationVersions;
    mapping(bytes32 => bool) releasePolicies;
    mapping(bytes32 => uint32) releasePolicyVersions;
    mapping(uint256 => Project) projects;
    mapping(uint256 => mapping(address => UserPosition)) positions;
    mapping(uint256 => mapping(address => bool)) hasParticipated;
    mapping(uint256 => uint64[]) vestingTimes;
    mapping(uint256 => uint16[]) vestingBps;
    uint256[32] __gap;
}
```

V1 的 `__gap` 明确为 32 个 slot。升级只能在现有字段之后、`__gap` 之前追加字段并等量缩减 gap；mapping/struct 既有字段顺序不可改变。CI 必须保存 `storageLayout` JSON并运行 OpenZeppelin `validateUpgrade`；任何 namespace、slot 或 layout 变化都需要新审计。

Initializer 顺序固定：参数全部非零和 fee 上限校验 → `__Ownable_init(governanceTimelock)` → `__Ownable2Step_init()` → `__ReentrancyGuard_init()` → 写入自有 namespace。implementation constructor 调用 `_disableInitializers()`。Proxy deploy 与 initializer 必须在一个部署操作中完成。

### 7.23 精确转账与 reserve 转换表

所有入口先读取合约余额，执行 SafeERC20 transfer，再要求 `contractAfter - contractBefore == amount`；之后增加 obligation reserve。所有出口先按 Checks-Effects-Interactions 减少对应 obligation 和聚合 reserve，执行 transfer，再核对 `contractBefore - contractAfter == amount` 及 recipient 增量；任一检查失败时整笔交易 revert，状态和 reserve 原子回滚。

| 操作 | 项目义务变化 | `reservedBalances[token]` |
|---|---:|---:|
| deposit project Token | deposited `+amount` | `+amount` |
| contribute payment Token | contribution `+amount` | `+amount` |
| refund/refundFor | refundable contribution `-amount` | `-amount` |
| claim/claimFor | unclaimed successful allocation `-amount` | `-amount` |
| project proceeds | unpaid net proceeds `-amount` | `-amount` |
| platform fee | unpaid fee `-amount` | `-amount` |
| failed/cancelled project Token return | deposited obligation `-amount` | `-amount` |
| excess recovery | 无业务义务变化 | 不变 |

`FairLaunchPaymentTokenInvalidated` 只将 payment obligation 从 pending successful settlement 重分类为 refundable contribution，金额和聚合 `reservedBalances[paymentToken]` 均不变；后续实际 `refund/refundFor` 才减少 reserve。治理裁决本身不是资金转账。

每次入口前后、出口前后均断言 `balance >= reservedBalances`。如果调用开始时已不满足，立即 revert `TokenReserveInsolvent`。Recovery 只能转到固定 Treasury，`amount <= balance-reserve`，不得接受任意 recipient。Token 返回 false、无返回但行为异常、malformed `balanceOf`、fee-on-transfer、recipient 未精确到账或 rebase 导致 delta 不精确均 fail closed。

---

## 8. 数据库设计

### 8.1 `projects` 表新增字段

建议迁移新增：

```sql
funding_mode varchar(32) NOT NULL DEFAULT 'capped_presale'
sale_token_amount decimal(65,18) NOT NULL DEFAULT 0
sale_token_amount_raw varchar(78) NOT NULL DEFAULT '0'
min_contribution decimal(65,18) NOT NULL DEFAULT 0
min_contribution_raw varchar(78) NOT NULL DEFAULT '0'
final_token_price_display varchar(160) NULL
final_price_numerator_raw varchar(78) NOT NULL DEFAULT '0'
final_price_denominator_raw varchar(78) NOT NULL DEFAULT '0'
final_total_raised decimal(65,18) NOT NULL DEFAULT 0
final_total_raised_raw varchar(78) NOT NULL DEFAULT '0'
total_claimed decimal(65,18) NOT NULL DEFAULT 0
total_claimed_raw varchar(78) NOT NULL DEFAULT '0'
liquidity_mode varchar(32) NOT NULL DEFAULT 'manual'
chain_status varchar(48) NOT NULL DEFAULT 'not_created'
terminal_reason varchar(48) NOT NULL DEFAULT 'none'
project_token_runtime_state varchar(32) NOT NULL DEFAULT 'unavailable'
payment_token_runtime_state varchar(32) NOT NULL DEFAULT 'unavailable'
payment_token_runtime_observed_code_hash varchar(66) NULL
payment_token_runtime_observed_policy_digest varchar(66) NULL
payment_token_runtime_observed_policy_version int unsigned NULL
payment_token_runtime_observed_block_number bigint unsigned NULL
payment_token_runtime_observed_block_hash varchar(66) NULL
payment_token_invalidation_reason_code varchar(66) NULL
payment_token_invalidation_evidence_hash varchar(66) NULL
payment_token_invalidation_operation_id varchar(66) NULL
payment_token_invalidated_at datetime(6) NULL
settlement_block_number bigint unsigned NULL
settlement_anchor_block_number bigint unsigned NULL
settlement_anchor_block_hash varchar(66) NULL
settlement_tx_hash varchar(66) NULL
settlement_tx_log_ordinal int unsigned NULL
release_policy_digest varchar(66) NULL
release_manifest_digest varchar(66) NULL
release_tx_hash varchar(66) NULL
release_tx_log_ordinal int unsigned NULL
released_at datetime(6) NULL
project_proceeds_receiver varchar(42) NOT NULL DEFAULT ''
project_proceeds_receiver_attestation_digest varchar(66) NOT NULL DEFAULT ''
project_proceeds_receiver_attestation_version int unsigned NOT NULL DEFAULT 0
platform_fee_receiver_attestation_digest varchar(66) NOT NULL DEFAULT ''
platform_fee_receiver_attestation_version int unsigned NOT NULL DEFAULT 0
```

兼容规则：

- 历史项目回填 `funding_mode='capped_presale'`；
- 若历史 schema 的 `token_price/hard_cap` 为 NOT NULL，Fair Launch 行允许在 DB 内写 0 作为 legacy storage sentinel；API、审批、业务判断和 UI 必须始终映射为 `not_applicable`，不得把 sentinel 当成真实财务值；
- raw uint256 十进制字符串是交易和 reconciliation 唯一权威；`DECIMAL(65,18)` 若保留只能作为可重建展示缓存；
- 最终价格权威保存 numerator/denominator，展示层使用明确 scale 的高精度字符串；
- Claim 无 V1 协议截止，不保存 `claim_deadline`；
- `onchain_locked=1` 后上述链上经济参数不可修改。

`chain_status` wire 值固定为 `not_created/pending/active/successful_pending_release/successful/failed/cancelled`，与营销/派生 `status` 分离；`terminal_reason` 固定为 `none/soft_cap_not_reached/cancelled_by_operator/token_runtime_invalid/governance_invalidation/payment_token_governance_invalidation`。另保存 `project_token_runtime_state/payment_token_runtime_state`，至少支持 `match/mismatch/emergency_blocked/unavailable`。Ended、Invalidated 和 Released materialization 必须在同一事务更新上述 identity/reason/release 字段；不得只写泛化 `projects.status`。

E8-A 不新增伪链上状态：支付 Token mismatch 时项目仍是 `Active` 或 `SuccessfulPendingRelease`，API 通过 `payment_token_runtime_state` 和 `actions.release/contribute.reason` 表达阻断。只有 canonical `FairLaunchPaymentTokenInvalidated` 应用后，`chain_status=failed` 且 `terminal_reason=payment_token_governance_invalidation`。

### 8.2 Fair Launch 用户仓位投影表

新增：

```text
fair_launch_position
```

建议字段：

```text
id
chain_id
contract_address
contract_version
project_id
wallet_address
payment_token_address
payment_token_decimals
contribution_amount_raw
contribution_amount_display
project_token_address
project_token_decimals
claimed_tokens_raw
claimed_tokens_display
refunded
first_contribution_event_id
last_contribution_event_id
last_claim_event_id
last_refund_event_id
status
created_at
updated_at
```

唯一键：

```text
chain_id + contract_address + contract_version + project_id + wallet_address
```

该表是事件投影，不是链上原始事实。原始资金事实继续写入 `fund_ledger`。

### 8.2.1 Operator signing reservation schema

现有 `chain_tx_submission` 增加明确状态/字段：

```text
state includes signing_reserved/signing_recovery
unsigned_tx_digest / signing_digest
replacement_generation
signing_reserved_at / signing_recovery_reason
```

`chain_relayer_nonce.active_submission_id` 必须 FK 到相同 `(chain_id,sender_address)` 的 submission；部分唯一索引保证每个 sender 只有一个 `active_slot=1`。Gateway journal 的 `(chain_id,expected_signer_address,nonce)` 唯一约束是第二道独立防线。DB/Gateway 两边状态不一致时只允许 recovery reconcile，不允许新签名。

### 8.3 不复用固定价格 entitlement

Fair Launch 贡献时不得创建现有 `ido_purchase_entitlement`，因为贡献发生时最终项目币分配尚未确定。

建议：

- `fund_ledger` 保存不可变资金事件；
- `fair_launch_position` 保存用户累计贡献/已领/退款投影；
- 成功后 API 按链上最终快照动态计算 `finalAllocation`；
- Claim 事件产生真实的项目币转出账本；
- 如后续审计需要，可新增按 reconciliation 证据块生成的 allocation snapshot 表，但不能伪装成认购时事实。

### 8.4 账本 entry type

新增或明确复用以下类型：

```text
payment_contributed
payment_refunded
project_token_claimed
project_proceeds_accrued
project_proceeds_paid
platform_fee_accrued
platform_fee_paid
project_tokens_withdrawn
```

所有金额必须同时保存：

```text
amount_raw
amount_display
token_address
token_decimals
asset_role
counterparty_address
tx_hash
tx_log_ordinal
rpc_log_index
block_number
block_hash
contract_version
```

Ledger 类型保持版本无关语义，并使用独立 `funding_mode/contract_version` 分类，避免超过现有 `entry_type varchar(32)`。只有 finalized canonical event 可以生成最终 ledger；receipt、API 请求或 submission success 均不是资金账本。Reorg 通过 reversal 记录处理，不物理删除历史。

### 8.5 Contract Source Registry 与事件身份

新增持久化、阈值签名的 off-chain Source Registry。DB 是其物化副本，签名 revision envelope和 bootstrap-pinned signer key set才是授权来源；Source identity 与不可变 revision 分表，不能在原记录上覆盖 ABI/runtime：

```text
chain_contract_source:
  id
  chain_id
  contract_address
  contract_version
  funding_mode
  enabled
  registered_at
  retired_at

chain_contract_source_revision:
  id
  source_id
  revision_no
  effective_from_block
  effective_from_tx_index
  effective_from_intra_tx_boundary
  effective_to_block nullable
  effective_to_tx_index nullable
  effective_to_intra_tx_boundary nullable
  activation_block_hash
  proxy_kind
  implementation_address
  implementation_code_hash
  abi_version
  abi_digest
  confirmation_policy
  governance_tx_hash
  governance_operation_id
  created_at
```

同一 Source revision ordered range 不得重叠。边界类型固定为 `(block_number, transaction_index, first_tx_log_ordinal_inclusive)`，range 为半开区间 `[from,to)`。`Upgraded` 事件始终用固定 inherited ERC-1967/UUPS Proxy ABI 解码，不依赖业务 revision；新 implementation revision 从同一交易中 `Upgraded` 之后第一个由 Proxy 发出的 tx-local log ordinal开始，若该交易之后无 Proxy log，则从下一交易 ordinal 0 开始。Revision 保存 activation event identity；reorg 时回退到 canonical common ancestor并重新计算 activation boundary。`source_revision_id` 属于 placement/decoding/application generation，不属于稳定 event identity；禁止根据当前 implementation 解码历史日志。

Cursor 必须持久化为两个层次：

```text
chain_ingestion_cursor:
  chain_id / block_number / block_hash / last_tx_index / last_log_index
  lease_token / fence_epoch / status / last_error / updated_at

source_materialization_cursor:
  source_id / source_revision_id
  block_number / block_hash / last_event_id
  lease_token / fence_epoch / status / last_error / updated_at
```

事件 materialization effects、ledger/projection 和 Source cursor 推进必须在同一 DB 事务中完成。一个 Source blocked 不阻止其他健康 Source 的 materialization。

新 Source 的 `start_block` 早于 live chain cursor 时，使用独立 `source_ingestion_coverage` 区间记录 historical backfill；不得倒退 live chain cursor。Chain cursor 仅在区块 header/transactions/receipts 及所有已知 Source 原始日志 durable archive 后推进。Source 只有 `[start,live_cursor]` 无缺口且 materialization cursor追平时才标 `caught_up`。

RPC `logIndex` 是块内全局序号，同一交易在另一块 re-inclusion 时可能变化，不能作为稳定 identity。事件唯一身份统一为：

```text
chain_id + tx_hash + tx_log_ordinal
```

`tx_log_ordinal` 按 transaction receipt 的有序 logs 在该交易内计算，0-based；RPC block-global `log_index` 仅属于 placement。Registry 记录不能物理删除，未知版本或地址 fail closed，不得回落 Legacy ABI。

稳定事件 identity 与 canonical placement 必须分离：

```text
chain_event_identity:
  id / chain_id / tx_hash / tx_log_ordinal

chain_event_placement:
  id / event_id / generation / block_number / block_hash
  transaction_index / rpc_log_index / source_revision_id / canonical_observation_id
  canonical / observed_at / noncanonical_at / reorg_incident_id

chain_event_placement_observation:
  id / event_id / placement_generation
  emitting_address / topics_data_digest / raw_log_digest
  observed_at / rpc_provider_identity

chain_event_application_attempt:
  id / event_id / placement_generation / effect_version / attempt_no
  predecessor_application_id nullable / requested_by_replay_audit_id nullable
  disposition(exact|conditional|blocked|no_op|reversed)
  effect_digest / applied_at / reversed_at
```

同一 `(chain_id,tx_hash,tx_log_ordinal)` 在不同块重新 inclusion 时仍是同一 identity，但追加新的 placement generation和 observation；若 emitting address/topics/data digest 改变，创建 append-only `chain_event_payload_conflict_incident` 并禁止 application，不能修改 identity或丢弃新 observation。Application attempt 唯一 `(event_id,placement_generation,effect_version,attempt_no)`；replay/reversal 均新增 attempt，不覆盖旧行。深度 reorg 后不得回退 EOA `next_nonce` 或重开旧 signer slot；通过 append-only reversal 修正 projection/ledger，并独立 reconcile transaction canonicality 与 account nonce。

现有 schema 的事件身份迁移必须分阶段：新增 nullable `chain_id/tx_log_ordinal` → 用 receipt 有序 logs 和可信 Source 回填 → 地址/hash 小写及 binary collation 规范化 → 检测新 identity 下重复 → dual write/read → 建新唯一索引 → 改 NOT NULL → 删除旧索引。无法证明 chain/ordinal 的历史行必须中止迁移并人工裁决，禁止默认归类到 BSC 56。`chain_event_record`、failure、replay、materialization、reorg association、canonical-failure trigger 和所有 tx-hash 唯一键必须一起迁移。

### 8.5.1 Event Identity V2 逐表 cutover matrix

版本名冻结：

```text
CE-V1        = 旧 block_number + tx_hash + rpc log_index 模型
CE-V2-shadow = 同事务写 V1 和 identity/placement/application，但读仍来自 V1
CE-V2-read   = V2 权威读；V1 仅保留回滚观察
CE-V2-only   = V2 约束生效；禁止旧 writer/image 启动
```

受影响表和字段迁移：

| 现有对象 | Expand / backfill | CE-V2 权威关系 | Cutover 前验证 |
|---|---|---|---|
| `chain_event_record` | 增加 nullable `chain_id/tx_log_ordinal/event_identity_id/placement_id/application_id`；保留旧主键 | 一条 legacy row 通过 bridge 映射到 identity + canonical placement + application generation | 每行 chain、receipt、ordinal、address/topics/data digest 可证明；无一对多冲突 |
| `chain_event_failure` | 增加 `event_identity_id/placement_id/source_revision_id/failure_generation` | failure 绑定具体 identity/placement；未知 placement 可先绑定 raw observation quarantine | unresolved failure 与 canonical application 不得同时存在 |
| `chain_event_replay_audit` | 保留 `failure_id`，增加 `requested_placement_generation/effect_version` | replay 只能产生新的 application attempt，不改 identity | 每个 replay 可追溯 failure 和 Admin reason |
| `chain_tx_submission` | 增加 `final_event_application_id`；旧 `final_event_record_id` 暂保留 | applied fence 只认 canonical V2 application | 两个字段必须解析到同一 identity/effect；不一致阻断 cutover |
| `fund_ledger` | M1 创建独立 `fund_ledger_v2_shadow`，字段包含 `source_application_attempt_id/entry_ordinal/reversal_of_ledger_id/tx_log_ordinal/rpc_log_index` | M3-M5 V2 effects 只写 shadow；M6 原子切 V2 ledger reader/writer；旧表保持 V1 单行唯一键 | amount/token/counterparty 与 application effect digest 一致；shadow 可全量重建 |
| `ido_purchase_entitlement` 与 Fair position | 增加 source application/reversal generation | projection 只由 canonical application和 reversal 更新 | 聚合值可由 ledger/application 全量重建且 digest 相同 |
| `chain_event_reorg_incident` / generation / candidate 表 | 增加 identity/placement/application generation refs | reorg 改 placement canonicality并追加 reversal，不修改 identity | 所有旧 incident event 可定位 common ancestor和受影响 placement |
| `chain_event_sync_state` | 只读兼容；新增 chain ingestion cursor和 Source materialization cursor | CE-V2 cursor 为权威，旧 cursor 冻结为审计快照 | 新 cursor 不落后且 block hash 与旧安全点一致 |
| canonical-failure triggers | 先 dual predicate | 最终只按 identity + canonical placement + application state判断 | V1/V2 predicate 结果零差异 |

切换阶段和回滚边界：

| 阶段 | Writer | Reader | 必须完成的动作 | 回滚边界 |
|---|---|---|---|---|
| M0 Inventory | CE-V1 | CE-V1 | 冻结表/FK/index/trigger 清单、旧 image digest、行数和 Merkle/digest 根；清空 active reorg maintenance | 可完整回滚 |
| M1 Expand | CE-V1 | CE-V1 | 只新增 nullable 列、新表、bridge、索引，并创建 `fund_ledger_v2_shadow`；不得删除旧约束 | 可回滚应用；schema 保留 |
| M2 Backfill | CE-V1 | CE-V1 | 用可信 receipt ordered logs 回填 ordinal；保存 RPC、block/hash、Source revision和 backfill tool digest | 可回滚应用；不得删除回填证据 |
| M3 Shadow write | CE-V2-shadow | CE-V1 | 每个新 log 在同一事务写 V1 + V2；写 `dual_write_digest` | 只能回到声明 CE-V2-shadow capability 的 image |
| M4 Shadow read | CE-V2-shadow | CE-V1 + V2 compare | V2 只影子 materialize；逐事件、项目、钱包、ledger、cursor 比较 | 同 M3 |
| M5 Cutover fence | 停止业务 materialization | 停止金融读切换 | 固定 finalized cutover block/hash；无 unresolved failure、active reorg、未知 ordinal；快照全部 outbox final-event refs | **最后可回到 CE-V1 reader 的点** |
| M6 Authoritative read | CE-V2-shadow | CE-V2-read | 原子切 reader pointer/capability generation；重放 cutover 后日志 | 不得启动 CE-V1-only image；问题走 forward fix或 Fair-capable maintenance |
| M7 Constraints | CE-V2-only | CE-V2-read | 新字段 NOT NULL、新 FK/unique/check、V2 canonical-failure trigger；禁止旧 writer lease | 不可回退旧 writer |
| M8 Retire legacy | CE-V2-only | CE-V2-only | 至少两个发布周期和一次 restore/reorg 演练后才删除旧 unique/index/columns | 仅新备份恢复 + forward migration |

M2 无法证明的数据写入 `chain_event_identity_quarantine`，字段至少包含 legacy table/id、raw identity、候选 chain/source/receipt、失败原因、tool version和人工裁决 digest。Quarantine 非空时禁止 M5。

M4/M5 必须运行以下等价验证族，而不是只比较行数：

```text
V1 legacy row → exactly one V2 identity/placement/application
V2 canonical application → exactly one expected V1 canonical row
unresolved failure ∩ canonical applied = empty
final_event_record_id ↔ final_event_application_id identity/effect equal
ledger/project/wallet/entitlement aggregate digests equal
cutover block/header/receipt/source coverage has no gap
tx_log_ordinal reconstructed from receipt and topics_data_digest matches
```

Reader precedence 固定：M0-M4 只服务 V1；M6 起只服务 V2，禁止逐请求 fallback。M6 后发现 V2 缺行时必须 block 对应 Source并修复，不得静默回读 V1。所有 migration/cutover manifest 绑定 schema migration checksum、writer/reader image digest、cutover block/hash、验证 query version、结果 digest和四眼批准。

`entry_ordinal` 来自按 `contract_version + event_signature + effect_manifest_version` 冻结的 effect manifest，0-based，禁止使用 DB insert order。V2 ledger 每行必须 FK `source_application_attempt_id`；reversal 行通过 `reversal_of_ledger_id` 指向原行。同一原 ledger 在同一 canonical compensation generation 最多一个有效 reversal；再次 reorg 使用新的 application/compensation attempt，不覆盖旧 reversal。

---

## 9. Backend 服务端修改

### 9.1 常量与领域模型

新增：

```go
const ContractVersionFairLaunchV1 = "fair_launch_v1"
const FundingModeCappedPresale = "capped_presale"
const FundingModeFairLaunch = "fair_launch"
```

所有只接受 `v2_stablecoin` 的判断需要分类处理：

- 真正只属于固定价格公式的代码继续限定 V2；
- 通用链上项目功能改为支持受信 contract version 集合；
- 禁止简单把所有 `== v2_stablecoin` 改成 `!= unassigned`。

### 9.2 受信配置

建议配置：

```yaml
chain:
  project:
    chainID: 56
    url: "..."
    confirmedConfirmations: 5
    finalizedConfirmations: 20

    v2:
      contractAddress: "0x..."
      implementationAddress: "0x..."
      implementationCodeHash: "0x..."
      startBlock: 0

    fairLaunchV1:
      enabled: false
      contractAddress: "0x..."
      implementationAddress: "0x..."
      implementationCodeHash: "0x..."
      startBlock: 0
      platformFeeReceiver: "0x..."
      platformFeeBps: 250
      paymentTokens:
        USDT:
          address: "0x..."
          decimals: 18
```

上线前 `enabled=false`；合约部署、Backend、Job、Admin 和 C 端全部验证后再启用。

生产必须配置不同供应商的 Primary/Secondary RPC，至少一个支持历史 `eth_getStorageAt`。RPC identity 不一致时 fail closed；生产禁止 `skipRuntimeVerify=true`。

### 9.3 合约客户端注册表

不要继续扩张单个全局 chain client。建议建立：

```go
type ProjectContractRuntime struct {
    Version string
    Address common.Address
    ABI abi.ABI
    StartBlock uint64
    Implementation common.Address
    CodeHash common.Hash
}

type ProjectContractRegistry interface {
    Runtime(version string) (ProjectContractRuntime, error)
    Client(version string) (*chain.Client, error)
}
```

所有读写均按项目的 `contract_version` 和受信配置选路，禁止接受客户端传入任意合约地址。

### 9.4 Set 参数接口

现有接口可保持：

```text
GET  /admin/v1/projects/:id/onchain-params
POST /admin/v1/projects/:id/onchain-params/preview
POST /admin/v1/projects/:id/onchain-params
```

请求改为判别联合类型：

```json
{
  "funding_mode": "fair_launch",
  "payment_token": "USDT",
  "token_address": "0x...",
  "token_symbol": "FOS2",
  "owner_address": "0x...",
  "project_proceeds_receiver": "0xVerifiedProjectSafe...",
  "sale_token_amount": "1000000",
  "soft_cap": "10000",
  "min_contribution": "10",
  "max_contribution": "1000",
  "start_time": "2026-10-01T04:00:00Z",
  "end_time": "2026-10-08T04:00:00Z",
  "tge_after_end_min": 10,
  "tge_interval_min": 0,
  "tge": [{ "percent": "100" }]
}
```

Fair Launch Preview 返回：

```text
funding_mode
contract_version
trusted chain/contract/fee snapshot
project/payment token identity and decimals
sale_token_amount display/raw
soft_cap display/raw
min/max contribution display/raw
soft-cap implied minimum successful price
start/end UTC
vesting schedule
required project token deposit
project owner and verified project proceeds receiver Safe
platform fee receiver Safe
project/payment Token policy digest and policy version
```

Fair Launch 不得要求或返回可编辑的：

```text
token_price
hard_cap
```

### 9.5 Set 写入与四眼审批

审批 payload 必须完整保存 `funding_mode` 和全部 Fair Launch 参数。审批执行时重新校验：

- 项目仍未锁定；
- 状态仍允许 Set；
- Token 运行时身份未变化；
- 受信合约地址和费率快照未变化；
- 时间仍在未来；
- display/raw 仍一致；
- TGE/Vesting 仍合法，且不存在 Claim Deadline 字段。

通过后写：

```text
status = Set
funding_mode = fair_launch
contract_version = fair_launch_v1
contract_address = trusted fairLaunchV1 proxy
legacy token_price storage sentinel = 0
legacy hard_cap storage sentinel = 0
```

上述 sentinel 只为兼容历史 NOT NULL 列，不得进入 canonical payload 或作为 API 财务语义。

### 9.6 公共 API

项目列表/详情增加：

```json
{
  "fundingMode": "fair_launch",
  "contractVersion": "fair_launch_v1",
  "chainStatus": "successful_pending_release",
  "projectProceedsReceiver": "0xVerifiedProjectSafe...",
  "platformFeeReceiver": "0xVerifiedPlatformSafe...",
  "saleTokenAmount": "1000000",
  "saleTokenAmountRaw": "1000000000000000000000000",
  "softCap": "10000",
  "softCapRaw": "10000000000000000000000",
  "minContribution": "10",
  "minContributionRaw": "10000000000000000000",
  "maxContribution": "1000",
  "maxContributionRaw": "1000000000000000000000",
  "hardCap": { "state": "not_applicable", "raw": null, "display": null },
  "softCapReached": true,
  "softCapProgressBps": "15000",
  "estimatedTokenPrice": { "state": "available", "display": "0.015", "numeratorRaw": "15000", "denominatorRaw": "1000000" },
  "finalTokenPrice": { "state": "pending_settlement", "display": null, "numeratorRaw": null, "denominatorRaw": null },
  "terminalReason": { "state": "available", "code": "none" },
  "successfulRelease": {
    "state": "pending",
    "settlementBlockNumber": "123400",
    "settlementAnchorBlockNumber": "123399",
    "settlementAnchorBlockHash": "0x...",
    "settlementTxHash": "0x...",
    "settlementTxLogOrdinal": "0",
    "releaseManifestDigest": null,
    "requiredConfirmations": 20,
    "currentConfirmations": 19
  },
  "projectTokenRuntime": { "state": "match", "policyDigest": "0x...", "policyVersion": 3, "codeHash": "0x...", "emergencyBlocked": false },
  "paymentTokenRuntime": { "state": "match", "policyDigest": "0x...", "policyVersion": 7, "codeHash": "0x...", "emergencyBlocked": false },
  "paymentTokenIncident": { "state": "none", "releaseBlocked": false, "contributionBlocked": false, "governanceDecision": null, "reasonCode": null, "evidenceHash": null, "operationId": null },
  "releasePolicy": { "state": "enabled", "digest": "0x...", "version": 2 },
  "projectProceedsReceiverAttestation": { "state": "approved_snapshot", "digest": "0x...", "version": 4 },
  "platformFeeReceiverAttestation": { "state": "approved_snapshot", "digest": "0x...", "version": 4 },
  "claimDeadline": { "state": "not_applicable", "value": null },
  "chainBlockNumber": "123456",
  "chainBlockHash": "0x...",
  "confirmations": 20,
  "observedAt": "2026-09-06T00:00:00Z",
  "stateSource": "canonical_chain_event",
  "finality": "finalized",
  "reconciliationState": "BALANCED"
}
```

禁止使用 0 同时表达“不适用、尚未结算、不限、不可用和真实零值”。允许状态至少包括 `available/not_applicable/pending_settlement/unlimited/unavailable/invalid`。

### 9.7 用户仓位 API

建议扩展 V2 portfolio 或新增：

```text
GET /client/project/v1/fair-launch/:projectId/position
```

返回：

```json
{
  "projectId": "8",
  "wallet": "0x...",
  "chainStatus": "successful_pending_release",
  "terminalReason": "none",
  "contribution": { "state": "available", "raw": "100000000000000000000", "display": "100" },
  "estimatedAllocation": { "state": "not_applicable", "raw": null, "display": null },
  "finalAllocation": { "state": "available", "raw": "3200000000000000000000", "display": "3200" },
  "claimed": { "state": "available", "raw": "0", "display": "0" },
  "claimable": { "state": "pending_release", "raw": null, "display": null },
  "refund": { "state": "not_applicable", "raw": null, "display": null },
  "paymentTokenRuntime": { "state": "match", "releaseBlocked": false, "contributionBlocked": false },
  "actions": {
    "claim": { "available": false, "reason": "successful_release_pending" },
    "refund": { "available": false, "reason": "project_not_failed_or_cancelled" },
    "contribute": { "available": false, "reason": "project_not_active" },
    "release": { "available": false, "reason": "operator_only" }
  },
  "source": {
    "blockNumber": "123456",
    "blockHash": "0x...",
    "confirmations": 20,
    "finality": "finalized",
    "reconciliationState": "BALANCED",
    "observedAt": "2026-09-06T00:00:00Z"
  }
}
```

所有财务值为判别对象，`state` 至少支持 `available/not_applicable/pending_settlement/pending_release/unavailable/invalid`，raw 仅在 available 时为十进制字符串。E8-A 下 `paymentTokenRuntime.state=mismatch/emergency_blocked` 必须使 Active 的 `contribute` 或 Pending Release 的 `release` 显式不可用，但不能把 API `chainStatus` 伪造为 Failed。成功终态的 `finalAllocation` 必须使用链上 `finalTotalRaised`，不能使用可能漂移的前端 totalRaised。未知 state、provenance 过期或跨字段 block/hash 不一致时，动作必须 disabled并等待 fresh chain preflight。

---

## 10. Durable Outbox 与生命周期动作

### 10.1 Operator durable action

```text
fair_launch_create
fair_launch_activate
fair_launch_end
fair_launch_cancel
fair_launch_release
```

`fair_launch_end` 由 Job 自动创建，但允许外部钱包 permissionless 抢先结束。`fair_launch_release` 只能由 Operator 在结算事件至少 20 confirmations、runtime 正常且 reconciliation 为 `BALANCED` 后创建。上述动作使用独立 Fair Launch Operator 和 `chain_relayer_nonce`。

Governance Safe/timelock 动作采用另一套受控 workflow 和 operation ID：

```text
fair_launch_pause_contributions
fair_launch_unpause_contributions
fair_launch_pause_payouts
fair_launch_unpause_payouts
fair_launch_pause_claims
fair_launch_unpause_claims
fair_launch_operator_update
fair_launch_pause_guardian_update
fair_launch_unpause_timelock_update
fair_launch_treasury_update
fair_launch_platform_fee_update
fair_launch_token_policy_update
fair_launch_beneficiary_safe_update
fair_launch_release_policy_update
fair_launch_pending_release_invalidate
fair_launch_payment_token_invalidate
fair_launch_upgrade
fair_launch_recover_excess
```

治理 workflow 必须持久化 canonical payload、四眼/三权审批、Safe transaction hash、timelock operation ID、链上 tx 和事件，但不得使用 Fair Launch Operator 签名，也不得占用 `chain_relayer_nonce`。

项目方和用户钱包动作不走平台代签：

```text
depositProjectTokens
contribute
claim
claimFor
refund
refundFor
withdrawProjectProceeds
withdrawPlatformFee
```

### 10.2 nonce fence

Fair Launch V1 使用独立 Operator，但所有 signer 的 nonce fence 规则保持一致：

```text
chain_relayer_nonce 主键仍为 chain_id + sender_address
```

同一 sender 的所有 Operator action 共享一个 `active_submission_id`。不得增加 contract address 到 nonce 主键。Safe 多签使用自身 Safe nonce 和 timelock operation ID，不得混入 EOA signer nonce fence。

治理自动执行还有独立的 Gas payer nonce 域：

```text
Operator action:
  (chain_id, operator_eoa, operator_nonce)
Safe proposal:
  (chain_id, safe_address, safe_nonce, safe_tx_hash)
Timelock operation:
  (chain_id, timelock_address, operation_id, predecessor, salt)
Automated executor transaction:
  (chain_id, executor_eoa, executor_nonce, tx_hash)
```

平台自动 executor 使用自己的 `(chain_id, executor_eoa)` durable fence；permissionless executor 由外部 EOA 自付 Gas。两者均不得占用 Operator fence。治理事件应用必须关联 approval digest、Safe transaction、Timelock operation 和实际 execution transaction。

Governance durable records 至少拆为：

```text
governance_safe_transaction:
  nonce_slot_id / chain_id / safe / safe_nonce / safe_tx_hash / canonical_payload_digest
  threshold / owner_set_digest / proposal_state / external_execution_tx_hash

governance_safe_nonce_slot:
  chain_id / safe / safe_nonce / state(open|conflicted|consumed|reconciling)
  winning_safe_tx_id nullable

governance_timelock_operation:
  chain_id / timelock / operation_id / predecessor / salt
  target / value / calldata_digest / delay / scheduled_at / ready_at
  state(scheduled|ready|executing|executed|cancelled|reorged)

governance_executor_submission:
  chain_id / executor_eoa / nonce / operation_id / raw_tx generations / receipt state
```

唯一键分别按上述 nonce/operation 域建立；Safe executed 不等于 Timelock executed，Timelock scheduled/ready 也不等于链上配置已 materialize。三层状态只能由各自 canonical event/receipt推进并通过显式 FK 关联，不复用 Operator `chain_relayer_nonce`。

### 10.2.1 Governance durable 状态机

#### Safe proposal

```text
draft
→ proposed                 // canonical Safe tx payload和 safeNonce 冻结
→ collecting_signatures   // 追加 owner signatures，不覆盖
→ threshold_reached
→ submitted               // Safe execTransaction 广播或外部执行等待
→ executed                // canonical Safe ExecutionSuccess
→ applied                 // 对应 Timelock schedule/config事件已 materialize

任意 pre-execution → rejected / expired / superseded
submitted → broadcast_unknown / execution_failed / reorged
```

约束：

- `safe_tx_hash = EIP-712 Safe domain + canonical transaction payload + safe_nonce`，数据库必须重算；
- nonce slot 唯一 `(chain_id,safe_address,safe_nonce)`；transaction 唯一 `(nonce_slot_id,safe_tx_hash)`。一个 slot 可保存多个竞争候选，但最多一个 canonical winner；发现第二个不同 hash 时 slot=`conflicted`，冻结自动提交并保留全部 payload/signature/发现来源；
- owner signature 唯一 `(safe_tx_id,owner_address)`，验证 owner set digest和 threshold snapshot；
- `threshold_reached` 不等于链上 executed；`ExecutionFailure` 不自动允许复用 Safe nonce，必须查询 Safe 当前 nonce；
- 平台永不代签或广播 `Safe.execTransaction`；它由 Safe owner 钱包或外部 executor 广播。`submitted/broadcast_unknown` 只由观察到的外部 execution tx identity推进，不占用平台 EOA nonce。外部地址执行已达 threshold 的 Safe tx时，必须匹配 exact safe tx hash；
- Safe reorg 只回退 execution placement/application，不向后修改已观察的 Safe nonce；进入 reconcile，直到 canonical nonce/事件收敛。

#### Timelock operation

`operation_id` 必须按目标 Timelock 实际 `hashOperation/hashOperationBatch` 重算，绑定 targets、values、calldatas、predecessor和 salt。记录 `expected_call_count` 及每个 index 的 target/value/calldata digest：

```text
planned
→ safe_approved
→ scheduling
→ scheduled     // canonical CallScheduled，保存 ready_at
→ ready         // 链上 timestamp >= ready_at 且 predecessor done
→ executing
→ executed      // canonical CallExecuted / isOperationDone
→ applied       // 目标合约事件和配置投影已 materialize

scheduled/ready → cancelling → cancelled
任意 canonical placement变化 → reorged → reconciling → 回到 canonical state
```

约束：唯一 `(chain_id,timelock_address,operation_id)`；相同 operation ID 的 canonical payload 必须字节完全相同。Batch 只有 `[0,expected_call_count)` 所有 `CallScheduled` index 均 canonical、payload 匹配且链上 `isOperationPending/Ready` 一致时才进入 scheduled/ready；`ready_at = canonical schedule block.timestamp + on-chain delay`，禁止使用 ingestion 时间。只有全部 `CallExecuted` index 匹配且 `isOperationDone(operationId)=true` 才进入 executed；缺 index或 payload不一致保持 blocked。重新 schedule 已 cancelled operation 必须使用新 salt/operation ID，不覆盖旧记录。Permissionless executor 抢先执行时，平台 executor intent 标 superseded-with-external-execution，但仍应用目标事件。

#### Automated executor EOA

自动 executor 采用与 Operator 相同的通用 sender fence算法，但使用独立表/role namespace和独立地址：

```text
ready → signing_reserved → signed → submitted/broadcast_unknown
→ mined_success → final_success → executed_event_applied
→ released
```

`(chain_id,executor_eoa)` 只有一个 active submission；`next_nonce` 同样在 exact raw transaction durable 时推进，reorg 不后退。调用 calldata 只能是 `TimelockController.execute/executeBatch` 且 operation ID 必须处于 ready。Operator、executor、Guardian Safe owner、beneficiary Safe owner 地址域必须配置检查为互不复用；部署检查发现相同地址即 fail closed。

#### 三层收敛和异常

```text
Admin approval digest
→ Safe tx hash / Safe nonce
→ Timelock operation ID
→ executor tx hash（可为空，若 permissionless external executor）
→ target contract event identity
→ materialized configuration/effect version
```

任何层出现 payload mismatch、nonce conflict、operation cancelled、unexpected external execution或 reorg，都必须保留 append-only incident和历史 hash。只有 target contract canonical event应用后，治理业务动作才是 `applied`；Safe success或 Timelock done 但目标事件缺失时保持 blocked，不伪造完成。

### 10.3 幂等与重试

- 同一项目、动作和 attempt 使用 durable intent；
- `broadcast_unknown` 必须先核对 receipt、事件和 pending nonce；
- receipt timeout 不等于失败；
- 只有明确 final revert 才允许新 attempt；
- applied 由对应链事件推进；
- `final_success` 但事件未应用时不得释放 `active_slot`；
- 只有 unsigned cancel、finalized revert 或 canonical event `applied` 才能释放；
- `applied/final_revert` 只释放 active fence，不得再次增加 `next_nonce`；
- 不得使用 Operator 私钥手工补交易。

`chain_relayer_nonce.next_nonce` 的唯一语义冻结为：**下一个尚未被本平台签名承诺的 nonce**。它不是“最近 finalized nonce”，不得依据单一 RPC pending nonce 向后移动。

```text
unsigned intent:
  不占用 nonce，不推进 next_nonce

进入 signing 前的 DB 事务：
  锁定 (chain_id,sender) 行
  要求 active_submission_id IS NULL
  submission.nonce = next_nonce
  冻结 unsigned_tx_digest/signing_digest
  submission.state = signing_reserved
  submission.active_slot = 1
  chain_relayer_nonce.active_submission_id = submission.id
  next_nonce 暂不推进

只有 signing_reserved durable 后才调用 Gateway。

Gateway 已 crash-safe 持久化 exact raw_tx 后，Job 第二个单事务提交：
  submission.raw_tx/raw_tx_digest/tx_hash/state=signed/active_slot=1
  chain_relayer_nonce.next_nonce=submission.nonce+1

signed/submitted/broadcast_unknown/ambiguous/final_success:
  保持 active_submission_id；next_nonce 不变

unsigned supersession：
  仅尚未进入 signing_reserved 时可普通取消

signing_reserved supersession/timeout：
  必须先查询 Gateway journal
  只有 Gateway 权威证明该 submission/digest 无 committed raw_tx 时才可释放 fence
  journal 状态未知时保持 fence并进入 signing_recovery

canonical final revert：
  释放 active_submission_id；不推进 next_nonce

canonical event applied after final success：
  M0-M5 双写 final_event_record_id + final_event_application_id，要求 identity/effect 相等
  M6+ 只在 final_event_application_id 指向 canonical + finalized + expected effect_version 的 application attempt时释放 active_submission_id
  final_event_record_id 仅保留历史审计；不推进 next_nonce
```

一旦 `signing_reserved` 写入，sender 已被唯一 submission 围栏；一旦 exact raw transaction durable，nonce 已被平台不可逆承诺，即使尚未观察到 receipt，也不能安全签出另一业务交易复用该 nonce。Gateway 除 `submission_id` 唯一外，还必须对 `(chain_id,expected_signer_address,nonce)` 建唯一约束。DB 必须以 FK/constraint 证明 active submission 属于同一 chain/sender；禁止两个 active sender slot。

### 10.4 远程签名边界

生产签名架构：

```text
JoinCare Job / durable outbox
→ JoinCare Sign Policy Gateway
→ Consensys Web3Signer
→ AWS KMS secp256k1 非导出密钥
```

- 策略网关校验 `submissionId/intentDigest/chainId/from/to/value/selector/calldata/nonce/gas/runtime identity`；
- Web3Signer 只签名和返回 raw signed transaction，不管理 nonce、不广播；
- durable outbox 是唯一广播、receipt、replacement、recovery 和 applied 状态机；
- 服务间使用私网、mTLS 和最小权限；禁止任意 `eth_sign/personal_sign/typed-data`；
- 私钥不得进入 Backend、Job、环境变量、YAML、数据库、Git 或发布包。

Sign Policy Gateway 必须实现独立、持久化、幂等的 signing journal。Journal 以 `submission_id` 唯一，同时唯一约束 `(chain_id,expected_signer_address,nonce)`，并保存：

```text
submission_id / signing_digest / intent_digest / approval_digest
runtime_snapshot_digest
chain_id / from / to / nonce / value / gas_limit / fee fields / calldata
signer_key_id / expected_signer_address
raw_tx / raw_tx_digest / tx_hash
state / attempt_count / created_at / committed_at
```

`signing_digest` 覆盖完整 unsigned transaction、chain、sender、nonce、Gas/fee、canonical calldata、approval 和 runtime snapshot。相同 submission + 相同 digest 的重试只能返回**完全相同**的 journaled raw transaction；相同 submission + 不同 digest 必须 fail closed。

Replacement 使用 append-only signing generation，不覆盖原 raw transaction：

```text
unique(submission_id, replacement_generation)
unique(chain_id, signer, nonce, replacement_generation)
```

generation 0 为首次签名；后续 generation 必须保持 chain/from/nonce/to/value/canonical calldata 完全相同，只允许经 policy 上限约束的 fee fields 和必要 gas limit 变化。每代保存独立 signing digest/raw tx/hash，所有旧 hash 永久保留用于 receipt/reorg 查询；一个 submission 只能有一个 canonical winning receipt。Receipt status 0 本身不含 revert reason；“外部 end 抢先导致 expected revert”必须通过同块或可信 historical `eth_call`、链上项目终态和 exact calldata 证据分类，证据不足时记录 generic final_revert，不伪造 AlreadyEnded reason。

Gateway 收到 Web3Signer 结果后必须：解码 raw tx → 核对所有冻结字段 → 恢复 signer 地址并匹配 KMS public key → 校验和规范化 signature → 计算 tx hash/raw digest → 在 Gateway journal 提交 `raw_tx + tx_hash` → 然后才可返回 Job。Gateway/Web3Signer 均不得持有 broadcast RPC 权限。

| 故障点 | 恢复动作 |
|---|---|
| Job signing reservation 前 | 无 sender fence，不得调用 Gateway |
| reservation commit 后、signer 调用前 | fence 已占用；同 submission/digest 可安全重试 |
| signer timeout/返回未知 | 先查询 Gateway journal；不得直接生成新 digest或换业务交易 |
| signer 返回后、Gateway commit 前 | Gateway 以同 submission/digest 恢复；未确认 journal 前不得返回 |
| Gateway commit 后、Job signed commit 前 | fence 仍绑定原 submission；Job 读取完全相同 raw tx，再原子写 outbox/推进 next_nonce |
| Job commit 后、broadcast 前 | 重启后广播已保存 exact raw tx |
| broadcast response 丢失 | 只重播同一 raw tx或查询 hash/receipt/account nonce，不重新签名 |

所有 raw transaction 读取、重播和人工恢复必须最小权限并写审计日志。POC 必须对每个边界注入 crash/timeout，证明始终只有一个 exact raw tx/hash，或在任何 raw tx durable 前安全失败。

### 10.5 创建和激活

通用 Admin endpoint 根据项目 `contract_version` 打包不同 calldata。响应继续返回：

```text
project_id
submission_id
status
submitted
tx_hash
```

### 10.6 自动到期结算

必须改造现有 `pkg/job/job/chainevent/projectEnd.go`：

- 查询所有已到期且 DB 为 Active 的受信合约版本；
- 按 `contract_version` 获取 runtime/client；
- 为 Fair Launch 创建 `fair_launch_end`；
- 为 IDO V2 创建对应 V2 end action；
- 保留 Redis token lock 和续租；
- 按共享 nonce fence 串行保留/派发；
- 合约已被外部 permissionless 调用结束时，Job 通过链快照/事件收敛，不重复广播。

扫描间隔为 60 秒；到期后 5 分钟仍未提交为 Warning，15 分钟为 Critical。Job 必须依据链上 `block.timestamp/status/endTime`，未到期不得 reserve nonce。

这一项是 Fair Launch 上线阻断项，不能依赖管理员人工结束。

---

## 11. 链事件与 Job 修改

### 11.1 多合约监听

Job 使用持久化、阈值签名的 off-chain Source Registry 注册：

```text
IDO V1（如仍启用）
IDO V2
Fair Launch V1
```

链级 canonical ingestion cursor 负责统一抓取区块；每个 Source 有独立 materialization cursor和：

- `contract_version`；
- `start_block`；
- implementation identity；
- ABI decoder；
- runtime verification；
- reorg incident scope。

某一 Source 解码或 materialization 失败只阻塞该 Source；其他健康 Source 继续处理。未知 Source 日志仅归档，绝不能使用 Legacy ABI 猜测解码。

### 11.2 Contribution 事件处理

`FairLaunchContribution` 事务内：

1. 验证项目 chain/contract/version 身份；
2. 插入 immutable `fund_ledger` 支付贡献事实；
3. upsert `fair_launch_position.contribution_amount_raw`；
4. 首次参与时推进 `participant_count`；
5. 更新项目 `total_raised_raw/display`；
6. 不写固定价格 entitlement；
7. 不写最终项目币数量；
8. 保持 chain + tx hash + tx-local log ordinal 幂等。

### 11.3 Ended 事件处理

`FairLaunchEnded`：

- 验证状态枚举；
- 更新 DB `SuccessfulPendingRelease/Failed`；
- 冻结 `final_total_raised_raw/display`；
- 成功时冻结最终价格 numerator/denominator 和高精度 display；
- 推进 durable `fair_launch_end` 到 applied；
- permissionless 外部结束没有 submission 时仍必须正常应用事件；
- 不遍历并写入所有用户 allocation。

`FairLaunchSuccessfulReleased` 才将 DB 推进为 `Successful`，写入 settlement identity、release manifest/policy digest/version、Release tx identity和 attester，并开放 Claim/提款投影。该事件必须匹配 Operator `fair_launch_release` 的 exact tx hash、sender、nonce 和 calldata。Manifest 中的 reconciliation result 是 Operator attestation；DB 另保存实际 reconciliation snapshot ID/digest，不得把 event 本身描述为 Solidity 证明 BALANCED。E8-A 要求 Release materialization 同时确认项目快照 payment Token runtime/policy 为 match；否则事件与本地预期冲突，Source 必须 blocked并升级事故，不能静默开放提款。

`FairLaunchPaymentTokenInvalidated` 只有匹配 48h Timelock operation、项目仍为 `SuccessfulPendingRelease` 且事件 evidence 完整时，才推进 `Failed(PaymentTokenGovernanceInvalidation)`；同时生成用户 payment reserve 的 refundable 分类和零平台费投影，但不逐钱包制造虚假转账账本。它不属于 Operator outbox，不得释放或推进 Operator EOA nonce fence。

### 11.4 Claim/Refund/提款事件

分别：

- 写 immutable fund ledger；
- 更新 `fair_launch_position` claimed/refunded；
- 更新项目 total claimed/提款 flag；
- 严格核对 Token 地址、decimals、counterparty 和 raw 金额；
- 推进对应 durable action（如属于平台动作）。

### 11.5 Durable action 与事件映射

| 事件 | Action | 是否允许无 submission |
|---|---|---|
| FairLaunchProjectCreated | `fair_launch_create` | 否，治理异常需告警 |
| FairLaunchTokensDeposited | projectOwner 钱包 | 是 |
| FairLaunchActivated | `fair_launch_activate` | 否，治理异常需告警 |
| FairLaunchEnded | `fair_launch_end` | 是，permissionless 到期结算 |
| FairLaunchSuccessfulReleased | `fair_launch_release` | 否，必须匹配 Operator durable submission |
| FairLaunchPendingReleaseInvalidated | `fair_launch_pending_release_invalidate`，48h Governance Timelock workflow | 否，必须匹配治理 operation 和非 payment-token `GovernanceInvalidationClass` |
| FairLaunchPaymentTokenInvalidated | `fair_launch_payment_token_invalidate`，48h Governance Timelock workflow | 否，必须匹配治理 operation；不得释放 Operator EOA fence |
| FairLaunchCancelled | `fair_launch_cancel` | 否，必须匹配 Operator durable submission |
| FairLaunchProjectProceedsWithdrawn | projectProceedsReceiver Safe 钱包 | 是 |
| FairLaunchPlatformFeeWithdrawn | beneficiary 钱包 | 是 |
| FairLaunchFailedProjectTokensWithdrawn | projectOwner 钱包 | 是 |
| FairLaunchContribution | 用户钱包 | 是 |
| FairLaunchClaimed | 用户钱包 | 是 |
| FairLaunchRefunded | 用户/permissionless refundFor | 是 |
| FairLaunchContributionsPaused | Pause Guardian Safe execution | 否，必须匹配 Guardian Safe tx 和实际 execution tx |
| FairLaunchContributionsUnpaused | 24h Unpause Timelock workflow | 否，必须匹配 Safe/timelock operation |
| FairLaunchPayoutsPaused | Pause Guardian Safe execution | 否，必须匹配 Guardian Safe tx 和实际 execution tx |
| FairLaunchPayoutsUnpaused | 24h Unpause Timelock workflow | 否，必须匹配 Safe/timelock operation |
| FairLaunchClaimsPaused | Pause Guardian Safe execution | 否，必须匹配 Guardian Safe tx 和实际 execution tx |
| FairLaunchClaimsUnpaused | 24h Unpause Timelock workflow | 否，必须匹配 Safe/timelock operation |
| FairLaunchOperatorUpdated | 48h Governance Timelock workflow | 否 |
| FairLaunchPauseGuardianUpdated | 48h Governance Timelock workflow | 否 |
| FairLaunchUnpauseTimelockUpdated | 48h Governance Timelock workflow | 否 |
| FairLaunchTreasuryUpdated | 48h Governance Timelock workflow | 否 |
| FairLaunchPlatformFeeUpdated | 48h Governance Timelock workflow | 否 |
| FairLaunchPaymentTokenPolicyRevisionRegistered | 48h Governance Timelock workflow | 否 |
| FairLaunchPaymentTokenAdmissionUpdated | 48h Governance Timelock workflow | 否 |
| FairLaunchPaymentTokenEmergencyBlockUpdated | 48h Governance Timelock workflow | 否，必须匹配治理 operation；Pause Guardian 不得修改 Token policy |
| FairLaunchProjectTokenConfigured | 48h Governance Timelock workflow | 否 |
| FairLaunchBeneficiarySafeConfigured | 48h Governance Timelock workflow | 否 |
| FairLaunchReleasePolicyConfigured | 48h Governance Timelock workflow | 否 |
| Initialized | deployment manifest / Proxy constructor initialization | 否，必须匹配已批准部署 operation |
| OwnershipTransferStarted | 48h Governance Timelock ownership workflow | 否，必须匹配 Safe/timelock operation |
| OwnershipTransferred | 48h Governance Timelock ownership workflow | 否，必须匹配 accept/initialization operation |
| Upgraded | Governance Safe workflow | 否，必须匹配 Safe/timelock operation |
| FairLaunchExcessTokensRecovered | Governance Safe workflow | 否，必须匹配已批准 recovery operation |

### 11.6 Reorg 和 replay

必须将 `fair_launch_v1` 纳入：

- cursor/checkpoint；
- reorg incident；
- recovery scan；
- materialization effect；
- assessment disposition；
- conditional replay；
- dead letter 和 Admin replay。

Contribution reorg 回滚需要同时处理：

- fund ledger canonical/reverted；
- position contribution 聚合；
- participant count；
- project total raised；
- 后续 Claim/Refund 的依赖关系。

不得把现有 V2 的 `payment → fixed tokens` 公式套到 Fair Launch。

Fair Launch 必须为每个事件定义 materialization effect、inverse effect、依赖 DAG、common-ancestor 快照和 `exact/conditional/blocked/no-op` 分类。旧分支保留为 reverted，替代分支重新 materialize；深度 reorg 时自动冻结新贡献、项目方提款、平台提费和新的 privileged outbox，但不得阻止链上 `endProject/refund/refundFor`。

Permissionless `endProject` 与 Operator submission 的竞态按 tx identity 收敛：

- 只有 `event.chain_id + event.tx_hash == submission.chain_id + submission.tx_hash`，且解码的 sender/calldata/nonce 一致时，事件才可关联 submission。M0-M5 双写 `final_event_record_id/final_event_application_id`；M6+ 只有 canonical + finalized + expected effect version 的 `final_event_application_id` 可最终释放 fence，旧 record ID 不再参与状态转换；
- 外部 `FairLaunchEnded` 正常应用项目状态，但只能把尚未进入 signing 的平台 intent 标为 `superseded_external`；
- 已 signed/submitted/broadcast_unknown 的 Operator transaction 必须继续按原 nonce 恢复，或使用同 nonce replacement；外部事件绝不能释放它的 fence；
- 若该交易最终以 `AlreadyEnded/InvalidProjectStatus` revert，作为 expected final revert 记录，链上 nonce 已消耗，随后只释放 fence，不再次推进 `next_nonce`；
- 一个项目终态 effect 只应用一次，重复/替代事件按 `no_op` 或 incident 处理。

### 11.7 Materialization effect、inverse 和依赖 DAG

每个 canonical placement generation 使用下表；“inverse”只产生 append-only reversal/compensation，不删除原 event 或 ledger：

| 事件 | Forward effect | Inverse effect | 主要前置/后继依赖 |
|---|---|---|---|
| ProjectCreated | 建项目 identity/config/policy snapshot，锁 onchain identity | 项目 projection 标 noncanonical；有后继时先 blocked | 根 → Deposit/Activate |
| TokensDeposited | deposited/reserve 增加，ledger 记录项目 Token custody | deposited/reserve projection 减少，追加 reversal | Created → Activate/withdraw |
| Activated | Pending → Active | Active → Pending | exact deposit → Contribution/End/Cancel |
| Contribution | payment ledger、position、totalRaised、participantCount | ledger reversal，减 position/total；首次参与消失才减 wallet count | Active → End/Refund |
| Ended(Failed) | Active → Failed，冻结 total/reason | Failed → Active，清终态快照 | Contributions → Refund/Token return |
| Ended(PendingRelease) | Active → SuccessfulPendingRelease，冻结 final totals/price | 恢复 Active并清 final/release 字段 | Contributions → Release/Invalidation |
| SuccessfulReleased | PendingRelease → Successful，写 evidence/digest | 恢复 PendingRelease；Claim/payout后继先逆序 reversal | Ended → Claim/proceeds/fee |
| PendingReleaseInvalidated | PendingRelease → Failed(reason) | 恢复 PendingRelease | Ended → Refund/Token return |
| PaymentTokenInvalidated | PendingRelease → Failed(PaymentTokenGovernanceInvalidation)，保存 48h Timelock evidence | 恢复 PendingRelease并清治理裁决投影 | Ended + payment mismatch → Refund/Token return |
| Cancelled | Pending/Active → Cancelled | 恢复 previousStatus | Created/Contributions → Refund/Token return |
| Refunded | payment reserve/position refundable 减少，refunded=true | append payment reversal，refunded=false | Failed/Cancelled + Contribution |
| Claimed | project Token reserve 减少，claimed 增加 | append project Token reversal，claimed 减少 | Released + vesting + Contribution |
| ProjectProceedsWithdrawn | net payment reserve 减少，flag=true | append reversal，flag=false | Released |
| PlatformFeeWithdrawn | fee reserve 减少，flag=true | append reversal，flag=false | Released |
| FailedProjectTokensWithdrawn | project Token reserve 减少，flag=true | append reversal，flag=false | Failed/Cancelled |
| Pause/Unpause | 更新对应控制投影和治理审计链 | 恢复上一控制状态 | Safe/Timelock execution identity |
| Config/Policy/Operator | 追加治理版本和 current projection | current pointer 回到前一版本，历史不删 | Safe/Timelock；影响后续 Preview/signing |
| Upgraded | 激活新 Source revision | revision range 回退并重选 prior revision | Safe/48h Timelock → subsequent logs |
| ExcessRecovered | 追加 non-reserve Treasury 资金事实 | append reversal | governance + aggregate solvency |

全局依赖顺序：

```text
SourceRevision/Created
→ Deposit → Activate → Contribution*
→ Ended
   ├─ Failed/Cancelled → Refund* / FailedTokenWithdrawal
   └─ SuccessfulPendingRelease → Release
       → Claim* / ProjectProceeds / PlatformFee
```

Reorg inverse 必须按 DAG 逆序执行。下游 canonical placement 尚存在时，上游 inverse 标 `blocked_dependency` 并自动扩大 common-ancestor recovery range；同一 generation 已应用/已逆转分别为 `no_op`。同 identity 新 placement generation重新 inclusion 时执行 `conditional`：若当前 projection 已由等价事实满足则 no-op，否则 exact apply。每次恢复保存 common ancestor block/hash、受影响 Source revisions、event/application generations、ledger reversal IDs、outbox/nonce assessment和最终 reconciliation digest。

---

## 12. Reconciliation 设计

### 12.1 项目级检查

至少包括：

```text
chain_project_id
chain_project_identity
chain_project_decimals_fee
chain_project_status
chain_sale_token_amount
chain_soft_cap
chain_min_contribution
chain_max_contribution
chain_start_time
chain_end_time
chain_total_raised
chain_total_tokens_deposited
chain_total_tokens_claimed
chain_final_total_raised
chain_final_price_numerator_denominator
chain_settlement_event_identity
chain_release_manifest_policy_identity
chain_project_token_runtime_policy_snapshot
chain_payment_token_runtime_policy_snapshot
chain_beneficiary_safe_attestation_snapshots
chain_payment_token_incident_and_governance_evidence
chain_withdrawal_flags
chain_vesting_schedule
chain_participant_count
```

### 12.2 钱包级检查

```text
chain_user_contribution
db_user_contribution
chain_final_allocation
formula_final_allocation
chain_claimed
db_claimed_ledger
chain_refunded
db_refund_ledger
chain_claimable
formula_claimable
```

### 12.3 守恒公式

成功项目：

```text
payment token:
finalTotalRaised
= projectOwnerProceeds
 + platformFee
 + paymentReserveRemaining
```

提款全部完成后：

```text
paymentReserveRemaining = 0
```

项目币在 Successful 后任意时刻：

```text
saleTokenAmount
= totalTokensClaimed
 + projectTokenReserveRemaining
```

用户 Claim 在 V1 中无协议截止，因此不能要求 Successful 项目的项目币 reserve 最终归零，也不存在用户未领取余额 sweep；可执行性限定见 §0.7 E5。

失败/取消项目：

```text
totalContributions
= totalRefunded
 + paymentReserveRemaining
```

全部退款后 payment reserve 为 0；项目方取回代币后该项目项目币 reserve 为 0。

E8-A 治理失效项目适用同一失败守恒，但 `totalContributions` 在 canonical `FairLaunchPaymentTokenInvalidated` 后才从 pending-release payment obligation 重分类为 refundable obligation；重分类不改变 `reservedBalances[paymentToken]`，平台费 obligation 必须为 0。事件前不得提前显示或记账为 Refund。

### 12.4 最终价格一致性

```text
chain.finalPriceNumeratorRaw   = chain.finalTotalRaisedRaw
chain.finalPriceDenominatorRaw = chain.saleTokenAmountRaw
```

展示价格使用明确 scale 的高精度格式化；用户 entitlement 始终直接使用 `Math.mulDiv(contribution, saleTokenAmount, finalTotalRaised)`，不得通过舍入后的价格反推。任何 raw 不一致必须标记 `UNBALANCED`。

### 12.5 Contract-Token 聚合检查

除项目级和钱包级外，必须按：

```text
chain_id + contract_address + token_address + evidence_block
```

验证：

```text
expectedAggregateReserveRaw = sum(expectedProjectReserveRaw)
chainReservedBalanceRaw == expectedAggregateReserveRaw
erc20BalanceRaw >= chainReservedBalanceRaw
```

每个 finalized 资金事件后执行；Active 项目每 5 分钟、终态项目每小时执行；提款、提费、recovery、发布、升级和 Operator 轮换前后强制执行。任何资金释放前相关项目和 Token 聚合必须为 `BALANCED`。

Release manifest 的 `BALANCED` 生成必须绑定同一 finalized evidence block/hash：项目级、所有有贡献的钱包级、contract-token aggregate 三组检查均通过，且 Source/materialization cursor 已覆盖 settlement event 和 evidence block。Manifest canonical payload 保存每组 snapshot ID/digest、row count、query/schema version、primary/secondary RPC identity、observed block/hash 和异常计数；任何 skipped/unknown/stale 项均不能汇总为 BALANCED。

---

## 13. C 端修改

### 13.1 项目上下文与合约路由

修改：

```text
src/services/projectContext.js
src/services/contract.js
src/config/contracts.js
```

新增 Fair Launch ABI 和方法路由：

```javascript
switch (project.contractVersion) {
  case 'legacy_jcc':
  case 'v2_stablecoin':
  case 'fair_launch_v1':
    // explicit adapter
    break
  default:
    throw new Error('UNSUPPORTED_CONTRACT_VERSION')
}
```

必须使用 Backend 返回并经过受信链策略验证的：

```text
chainId
contractAddress
contractVersion
paymentTokenAddress
paymentTokenDecimals
projectTokenAddress
projectTokenDecimals
```

禁止根据 Token symbol 推断合约或 decimals。

Fair Launch 必须显式验证 `projectExists`、返回 projectId、projectToken 和 projectOwner；不存在或未知版本时只允许显示营销内容和链身份，全部金融按钮 fail closed。

### 13.2 Launchpad 列表

Fair Launch 卡片：

- 增加 `Fair Launch` 模式标签；
- 不显示 Hard Cap；
- 显示已募集金额；
- 显示 Soft Cap 和完成率；
- 达软顶后显示“成功条件已满足”；
- 进度条按 Soft Cap 计算，可显示超过 100% 的文字；
- 倒计时仍按 `upcoming → startTime`、`live → endTime`。

示例：

```text
Fair Launch
已募集 15,000 USDT
Soft Cap 10,000 USDT · 150%
已达到软顶 · 持续募资中
```

### 13.3 Launchpad 详情

显示：

```text
模式：Fair Launch
销售代币总量
当前募集金额
Soft Cap
Soft Cap 完成率
当前预估价格
最终价格（结算后显示）
单次最低贡献
单钱包最高贡献（0 则显示不限）
开始/结束时间
Vesting
```

隐藏：

```text
Hard Cap
固定 Token Price
Raised / Hard Cap
```

### 13.4 认购组件

流程仍为：

```text
Approve payment token → Contribute
```

Approve 默认只授权本次精确金额，显示完整 Token、spender Proxy、Chain ID 和金额；Approve 成功不得称为 Contribution 成功。项目结束后提示 residual allowance/revoke。

“精确授权”表示 `approve(spender,value)` 的目标总 allowance，不是增加差额。默认 target 等于当前确认的 Contribution amount；allowance 相等时无需授权，低于时签 target，高于时提示 residual allowance并提供降额/Revoke。输入、账户、Chain、Proxy、Token、projectId 或 runtime 变化都会使旧 allowance 读取和确认失效。

对 zero-first Token：先独立确认 `approve(proxy,0)`；只有该交易确认且 fresh allowance 为 0 后，用户再次独立确认 `approve(proxy,target)`，绝不自动发第二笔。第一笔 unknown/replaced/reverted 时停止。确认页显示 current allowance、signed value、resulting total allowance、spender、Token 和 Chain；任何路径禁止默认 unlimited/max approval。

输入区显示：

```text
本次贡献
用户累计贡献
当前预估可获得项目币
最终数量将在募资结束并完成链上结算后确定
```

校验：

- 安全上下文；
- 钱包链匹配；
- payment allowance；
- 余额；
- minContribution；
- 累计 maxContribution；
- start/end 时间；
- 权威链上 Active 状态。

所有钱包写入使用可持久化状态机：

```text
preflight → awaiting_wallet → broadcast_unknown/hash_received
→ mined_success/mined_revert → confirmations_pending → confirmed
→ projection_pending → applied
```

`approve/contribute/deposit/claim/claimFor/refund/refundFor/提款/提费/revoke` 均适用。页面刷新后不能丢失 uncertain/pending 状态。

持久化 transaction intent 至少包含 `chainId/from/to/contractVersion/projectId/selector/calldataDigest/value/nonce?/walletRequestId/txHash/replacementOf/replacedBy/createdAt/updatedAt/lastCheckedBlockNumber/lastCheckedBlockHash/state/projectionState`。只有完整 identity 相同才能恢复或阻塞动作；logout、JWT 过期、刷新、切账户或切链不得删除 `broadcast_unknown`。

Gas 策略：

- live `estimateGas()`；
- 25% buffer；
- 只有明确 `-32601/Method not found/unsupported method`，且同 sender、同 calldata、同受信 runtime/block 的 `eth_call` 成功时，才允许 selector-specific reviewed fallback；timeout、普通 RPC 错误、空错误和业务 revert不得 fallback；
- 业务 revert 必须 fail-closed；
- 严格验证 `0x + 64 hex` 交易哈希；
- 广播状态不明时禁止自动第二次发送；
- receipt status 0 必须报执行失败并保留合法 tx hash。

Fallback limits 来自随 ABI 编译产物签名发布的 `wallet-gas-policy.v1` manifest，键为 `chainId + contractVersion + runtimeCodeHash + selector`，值包含 exact fallback gasLimit 和 maximum fee ceiling。Unknown selector、runtime/code hash不匹配、manifest 过期或签名错误均禁止 fallback。首个 V1 候选 limit 只能在合约 worst-case 测试后写入 manifest，不能在设计文档中猜测通用 300,000。钱包确认页显示 fallback 原因、gasLimit 和最大费用。

每次 Contribution 签名前必须展示项目/链/Proxy/Token/本次和累计金额、预计价格/分配、不可撤回、后续稀释、可能 0 raw unit、无自动流动性/LP 锁仓，并要求风险勾选。财务计算只使用 decimal string + bigint，禁止 `Number/parseFloat`。

所有金融动作使用 action-specific confirmation，禁止通用无标签按钮直接唤起钱包。共同字段包括 chain、完整 Proxy/Token 地址、selector/action、raw-derived display amount、source block/hash、destination、lifecycle/finality/reconciliation、一次性/永久效果、Pause/runtime warning。Claim 显示 final/vested/claimed/claimable 和固定 user destination；Refund 显示全额 payment、终态 reason和固定 destination；两类提款显示 final raised、fee bps、fee/net、固定 receiver Safe和一次性效果；Failed Token withdrawal 显示终态原因、数量和固定 projectOwner；`claimFor/refundFor` 分开显示 caller 与 target user。任一事实变化要求 fresh preflight 和重新确认。

### 13.5 用户投资组合

募资中：

```text
我的贡献：100 USDT
当前预估分配：约 4,000 FOS2
最终分配：待结算
```

成功结算后：

```text
最终分配：3,200 FOS2
已领取：800 FOS2
可领取：400 FOS2
下一期解锁时间：...
```

失败/取消后：显示可退款金额，只有权威终态开放退款。

到期但链上仍 Active 时，所有用户可见 “Finalize on-chain settlement” permissionless 入口；链上结算确认前 Claim/Refund 均不可用。Claim/Refund 只需钱包连接，不依赖平台 JWT。

主网上线前交付独立静态 Recovery Portal，支持 `endProject/claim/claimFor/refund/refundFor`，不依赖 Backend、MySQL、Redis 或 JWT。Portal 内置签名发布的 versioned deployment manifest，固定 chain 56、Proxy、ABI version/hash、expected implementation/runtime code hash、start block和 explorer references；校验 projectExists 和项目 identity，拒绝任意地址注入、未知版本、错误链和 runtime mismatch。

Manifest 规范冻结为 RFC 8785 JCS canonical JSON，domain separation 为 `joincare.recovery-portal.deployment-manifest.v1`，使用 Portal 构建时内置的 2-of-3 Ed25519 公钥阈值验证；Manifest 内携带的 key 永不作为信任根。字段必须包含 `generation/issuedAt/expiresAt/keySetVersion/previousManifestDigest`。Portal 仅接受 generation 单调增加、未过期、链式 previous digest匹配且 key set 未撤销的 manifest；回滚或同 generation 不同 digest fail closed。Key rotation 由旧 key set 2-of-3 签署新 key set并设置生效 generation，紧急撤销通过同时发布的新 Portal binary 和公开 revocation record。Manifest 离线生成、双人复核、三签中的至少两签，并随静态包发布，公开 digest、签名和更新记录。

### 13.6 项目方 Dashboard

Pending 阶段改为模式感知：

```text
Fair Launch 销售总量
项目方余额
Allowance
已充值
尚需充值
```

充值目标：

```text
saleTokenAmount
```

项目 Token approval 完全复用 §13.4 的 exact target 和 zero-first 两步规则：spender 必须是 Fair Launch Proxy，禁止 unlimited。Deposit 专属确认显示 chain、project Token/Proxy 完整地址、current allowance、signed target allowance、当前已充值、本次充值、充值后总量、剩余量和 `saleTokenAmount`；任一值变化必须重新 preflight。

不显示 Soft/Hard Cap 对应项目币需求。

Active 阶段显示：

```text
当前募集
Soft Cap
Soft Cap 完成率
当前预估价格
参与钱包数
剩余时间
```

Successful 阶段显示最终价格、最终募集、已领取总量和提款动作。只有 API 状态为 `finalized + BALANCED` 且签名前链读一致时开放提款。

### 13.7 多语言

风险披露使用稳定 semantic disclosure ID，并为繁中、简中和英文提供审核后的完整正文。所有 locale 必须包含同一 ID 集合；缺失、空值、截断或 fallback 均阻止签名。至少覆盖：关联钱包/自募资且钱包数不等于独立投资者、末期鲸鱼稀释、Active 不可撤回、0 raw allocation、Release 后快速提款、无自动流动性/LP lock/二级市场保证、稳定币发行方冻结、UUPS/多签治理、Token 不保证永久可转移和固定地址丢失风险。

基础标签至少增加繁中、简中和英文：

```text
Fair Launch
浮動價格
銷售代幣總量
當前預估價格
最終價格
最終價格將在募資結束後確定
已達到軟頂
成功條件已滿足，認購持續至結束時間
等待鏈上結算
最低參與金額
單錢包最高參與金額
無上限
```

所有金融页面、签名弹窗和 Recovery Portal 必须达到 WCAG 2.2 AA：light/dark 文字与非文字对比、键盘完整操作和可见 focus、非仅颜色状态、modal focus trap/restore、screen-reader 金额/地址/链/交易状态播报、200% zoom/reflow、移动端 target、reduced motion、字段错误关联及无需 tooltip 即可读取完整地址。测试包含 axe 加人工 keyboard、screen reader、high contrast、light/dark、200% zoom和移动端验收。

---

## 14. Admin 管理端修改

### 14.1 模式选择

链上参数页增加必选：

```text
募资模式
├─ 固定价格预售（Capped Presale）
└─ 公平发行（Fair Launch）
```

模式一旦项目创建上链并 `onchain_locked=1`，不可修改。

Funding Mode 使用严格判别联合类型。Fair Launch 请求即使携带 `tokenPrice=0` 或 `hardCap=0` 也必须拒绝；切换模式清除不兼容字段并使 Preview/Approval 失效。

### 14.2 条件表单

固定价格预售继续显示：

```text
Token Price
Soft Cap
Hard Cap
Max Per User
```

Fair Launch 显示：

```text
Sale Token Amount
Soft Cap
Minimum Contribution
Maximum Contribution（0 = 不限制）
Start Time
End Time
Vesting Schedule
Project Proceeds Receiver Safe
```

Fair Launch 隐藏且不得提交：

```text
Token Price
Hard Cap
```

Ant Design Vue 组件必须显式 import 并使用 PascalCase。

### 14.3 Preview

Preview 弹窗必须醒目标注：

```text
模式：Fair Launch
目标合约：JoinCareFairLaunchV1 Proxy
无 Hard Cap
价格在结束后统一确定
达到 Soft Cap 不会提前结束
项目上链后所有经济参数不可修改
```

要求多次确认并进入四眼审批；发起人不能自批。

Preview 必须返回 `previewId/canonicalPayload/payloadDigest/schemaVersion/runtimeSnapshot/runtimeBlockNumber/expiresAt`，有效期 15 分钟。Submit 只接受 `previewId + payloadDigest + idempotencyKey`；审批通过后 2 小时内执行。表单、模式、runtime、allowlist、费率、receiver、Operator、项目版本或权限变化均立即失效。

### 14.4 项目列表与动作

列表增加：

- Funding Mode；
- Contract Version；
- Sale Token Amount；
- Soft Cap Reached；
- Estimated/Final Price；
- Settlement Pending；
- Finality/Reconciliation；

动作统一为：

```text
创建上链
激活
结束
成功释放
取消
对账
```

平台提费不是 Admin execute 动作，只能由固定 `platformFeeReceiver` Safe 钱包调用；项目方收益同理只由固定 `projectProceedsReceiver` Safe 发起。

服务端按 `contract_version` 路由。Admin 不允许管理员手工填写目标合约地址。

### 14.5 权限

沿用：

```text
m2_project:read
m2_project:write
m2_project:approve
m2_project:execute
```

- 参数提交需要 write；
- 四眼审批需要 approve；
- 创建/激活/结束/成功释放/取消需要 execute；
- Approver 账号不自动获得 execute；
- Super Admin 也必须遵守发起人禁止自批。

### 14.6 链交易页

支持显示新 action：

```text
fair_launch_create
fair_launch_activate
fair_launch_end
fair_launch_release
fair_launch_cancel
```

显示 contract version/address，避免同一 project ID 的不同合约动作混淆。

### 14.7 链事件和资金审计页

增加 Fair Launch 事件过滤、仓位投影、贡献账本和公式说明。认购期不得展示伪造的固定项目币 entitlement，应显示：

```text
Contribution confirmed
Final allocation pending settlement
```

### 14.8 暂停入口

IDO V2 和 Fair Launch 分开显示：

```text
IDO V2: Running / Paused
Fair Launch V1: Running / Paused
```

暂停请求继续走受控 durable action 和审计日志。

Admin 分别显示 Contribution/Payout/Claim Pause。Set、创建、Pending/Active 取消、全部 Pause/Unpause、Operator、Token allowlist、受信 runtime、费率/receiver、UUPS 和 recovery 均需四眼审批；UUPS、Operator、recovery、受信 implementation 和全局 payout/claim pause 使用 initiator、approver、executor 三权分离。执行页必须展示完整 calldata、解码参数、digest、runtime 和 Preview 差异。

所有三权动作强制 `initiator_id`、`approver_id`、`executor_id` 两两不同，以不可变用户 identity 判断而非角色名称或 session。禁止共享账号、Super Admin bypass；审批和执行必须 step-up MFA。Set 之外的治理 Preview 同样绑定 chain、contract、version、selector、全部参数、expected state、runtime snapshot、Safe threshold、digest、expiry 和 idempotency key；角色、Safe threshold、runtime、fee、receiver、Operator 或项目状态变化都会撤销批准。

---

## 15. Set 脚本和参数模板

建议不要复制两套完全独立的金额/时间实现。可将公共 raw 换算、时区、TGE 和数据库事务守卫抽成共享模块，并提供两个入口：

```text
scripts/ido_prepare_set.py
scripts/fair_launch_prepare_set.py
scripts/fair-launch-project-params.example.json
```

Fair Launch 参数示例：

```json
{
  "project_id": 8,
  "funding_mode": "fair_launch",
  "chain_id": 56,
  "fair_launch_contract_address": "0x...",
  "project_proceeds_receiver": "0xVerifiedProjectSafe...",
  "platform_fee_receiver": "0xVerifiedPlatformSafe...",
  "platform_fee_bps": 250,

  "token_address": "0x...",
  "token_decimals": 18,
  "payment_token": "USDT",
  "payment_token_address": "0x...",
  "payment_token_decimals": 18,
  "owner_address": "0x...",

  "sale_token_amount": "1000000",
  "soft_cap": "10000",
  "min_contribution": "10",
  "max_contribution": "1000",

  "start_time": "+20m",
  "end_time": "+7d",
  "tge_after_end_min": 10,
  "tge_interval_min": 0,
  "tge": [{ "percent": 100 }],

  "db_tz_offset_hours": 8
}
```

脚本必须：

- 支持 `--dry-run`；
- display/raw 精确换算；
- 不接受科学计数法；
- 校验受信配置；
- 校验链上 Token decimals/code；
- 校验时间窗和容器 TZ；
- 单事务；
- `onchain_locked=0` 守卫；
- 只允许 Setup/Set/PreLaunch/Warmup；
- 写入 `contract_version=fair_launch_v1`；
- 将 `token_price/hard_cap` 清零；
- 输出 Soft Cap 对应的隐含最低成功价格，仅供预览。

---

## 16. 安全要求

### 16.1 必须满足

- 合约结算 O(1)，禁止遍历参与者；
- 使用 `Math.mulDiv`，禁止先乘后除溢出风险；
- 所有 raw 值使用 uint256 语义；
- Token 必须精确转入和转出；
- 拒绝 fee-on-transfer、rebasing 或异常 ERC20 行为；
- Token decimals 创建和运行时双重验证；
- 项目币/支付币 allowlist；
- ReentrancyGuard；
- reserve solvency；
- 经济参数上链后不可修改；
- 认购时不写最终分配；
- 达软顶不提前提款；
- 到期未结算不提前 Claim/Refund；
- Claim 无 V1 协议截止，不提供用户未领取权益 sweep；
- Upgrade Authority 使用 2-of-3 Safe + 48h timelock，独立 Pause Guardian；
- Fair Launch 使用独立 Operator，nonce 域仍为 `chainId + sender`；
- 前端广播不确定时 fail-closed；
- 所有 durable action 必须有事件推进 applied。

### 16.2 重点威胁模型

| 威胁 | 防护 |
|---|---|
| 项目方少充值或超额充值 | 累计不得超过且激活要求精确等于 `saleTokenAmount` |
| 达软顶后项目方提前提款 | 只有链上 Successful 可提款 |
| 结算 Gas DoS | `endProject` 不遍历参与者 |
| 用户未领取权益被没收 | V1 无 Claim 截止且不提供 sweep，尘埃允许留在 reserve；可执行性受 E5 限定 |
| 重复 Claim | 累计 `claimedTokens` |
| 重复退款 | `refunded` flag |
| 多合约 Operator nonce 冲突 | `chainId + sender` 全局 nonce fence |
| 伪造目标合约 | 服务端受信 registry，不接受客户端地址 |
| 前端把预估当最终事实 | API/UI 明确 estimated/final 字段分离 |
| 到期前端误判终态 | 金融动作仅信链上权威状态 |
| reorg 重复/丢失账本 | chainId + tx hash + tx-local log ordinal 身份及 source-isolated recovery |
| 全局 pause 冻结退款 | 三域 Pause；refund/refundFor/endProject 永远可用 |
| 项目方关联钱包自募资 | 无法技术消除；强制披露、Admin 尽调和异常贡献监控 |
| 项目 Token 后门 | V1 严格准入并在关键动作复核 runtime identity |
| 支付 Token 升级/冻结导致资金归属争议 | E8-A：先阻止 activate/contribute/release；仅 48h Timelock 可在 Pending Release 裁决 Failed，Successful 不回滚，转账失败原子保留 reserve |
| 到期取消/结算竞态 | now >= endTime 后 cancel 永远 revert |

---

## 17. 测试计划

### 17.1 合约单元测试

至少覆盖：

1. 初始化和重复初始化；
2. Owner 两步转移；
3. Operator 权限；
4. Token allowlist 和 decimals；
5. 创建参数边界；
6. Fee snapshot 漂移拒绝；
7. 项目重复创建；
8. 项目方精确充值；
9. 非项目方充值拒绝；
10. 充值不足无法激活；
11. 未开始/已结束贡献拒绝；
12. minContribution；
13. 累计 maxContribution；
14. 无 Hard Cap，多笔大额贡献仍可持续；
15. 达 Soft Cap 后仍可贡献；
16. 到期前不能结束；
17. 到期低于 Soft Cap → Failed；
18. 到期达到 Soft Cap → SuccessfulPendingRelease；
19. 未满 20 confirmations、任一项目快照 Token runtime/policy mismatch 或 reconciliation 非 BALANCED 时不能 Release；
20. 合法 Operator Release 后 → Successful；
21. Pending Release 项目 Token runtime mismatch → permissionless Failed(TokenRuntimeInvalid) 且可退款；
22. Pending/Active 支付 Token mismatch 分别阻止 activate/contribute，但不自动改变既有支付币归属；
23. 到期 Active 的支付 Token mismatch 不阻止 permissionless end，达到 Soft Cap且项目 Token正常时进入 SuccessfulPendingRelease；
24. Pending Release 支付 Token mismatch/emergency block 阻止 Release，permissionless fail 必须拒绝；
25. 仅 48h Governance Timelock 可执行 payment-token invalidation → Failed(PaymentTokenGovernanceInvalidation)，平台费 0；
26. 支付 Token 恢复到项目 exact snapshot并重新 finalized reconciliation 后可继续 Release；新全局 policy revision 不得覆盖旧项目快照；
27. Successful 后支付 Token mismatch 不回滚 Failed、不阻止项目 Token Claim，proceeds/fee transfer fail closed；
28. Failed/Cancelled 的支付 Token transfer 失败时 Refund 原子 revert，reserve/position/ledger 不变，恢复后可重试；
29. 最终价格公式；
30. 多用户最终分配公式；
31. 多次贡献累计；
32. 多期 Vesting Claim；
33. 重复 Claim；
34. V1 无截止 Claim 和 claimFor；
35. 舍入尘埃持续 reserve；
36. Failed/Cancelled refund；
37. 重复 refund；
38. 项目方提款；
39. 平台提费；
40. 超额充值拒绝；
41. 用户未领取项目币不可 sweep；
42. 多项目共享同一 Token reserve；
43. fee-on-transfer Token 拒绝；
44. reentrancy Token；
45. pause 语义和 Claim Pause 7 天自动到期；
46. recoverExcess 不得侵占 reserve；
47. UUPS upgrade authorization 和两个 Timelock 拓扑；
48. storage layout validation；
49. cancel/endTime 同区块竞争；
50. 三类 Pause 互不越权；
51. runtime identity 变化后贡献和提款 fail closed；
52. recoverExcess 不得侵占多项目聚合 reserve；
53. Upgrade Safe/timelock/guardian 权限边界。

### 17.2 属性/模糊测试

关键 invariant：

```text
sum(claimedByUsers) <= saleTokenAmount
totalTokensClaimed <= saleTokenAmount
contract token balance >= reservedBalances[token]
final allocation does not depend on contribution order
same contribution ratio receives same final token ratio
endProject gas does not grow with participant count
totalRaised never decreases except through terminal payouts represented by reserve
V1 claim rights have no protocol deadline and cannot be swept
refundFor/claimFor can never redirect beneficiary
cancelProject always reverts at or after endTime
successful displayed price is representable and nonzero
```

### 17.3 Backend/Job 测试

- Set polymorphic request；
- raw/display 精度；
- contract registry routing；
- durable action identity；
- sender-scoped nonce fence；
- Contribution event idempotency；
- Position aggregate；
- Ended 状态和最终价格；
- permissionless Ended 无 submission；
- Claim/Refund/提款账本；
- reorg rollback/replay；
- dead letter；
- 自动到期结算；
- reconciliation BALANCED/UNBALANCED；
- 6/18 decimals；
- runtime implementation/code hash verification。
- source-isolated cursor；
- `chain_id + tx_hash + tx_log_ordinal` re-inclusion，RPC logIndex 变化；
- final_success 不释放 signer fence；
- external end 抢先时所有 submission 状态收敛；
- contract-token 聚合 reserve；
- Sign Policy Gateway/Web3Signer/KMS POC 与故障恢复；
- Backend/Job/MySQL 不同 TZ 时不提前签名；
- Fair-capable maintenance image 启动门禁。
- table-driven nonce：unsigned supersession、signed crash、final success、final revert、replacement、restart，证明不复用、不跳号且单 sender 只有一个 active slot；
- Gateway/Web3Signer/KMS/Job 每个边界 fault injection，同 submission 的 calldata/nonce/gas/runtime/approval/chain 篡改全部 fail closed；
- Safe nonce、Timelock operation ID、executor EOA nonce 与 Operator nonce 相互独立；
- Source revision upgrade/reorg、旧新 ABI 选路和 cursor 回退；
- event identity chain_id staged migration，unknown chain/duplicate 数据阻断；
- external end 在 intent/ready/signing/signed/submitted/broadcast_unknown/mined/final_success 各状态竞争；
- applied 后深度 reorg：append-only reversal、re-inclusion 单次应用且 next_nonce 永不后退；
- active/unresolved-signature/reorg 场景回滚 maintenance image 和 DB restore 演练。

### 17.4 C 端测试

- 两种 funding mode 路由；
- Fair Launch 不显示 Hard Cap；
- Soft Cap 进度超过 100%；
- estimated/final 文案；
- 输入后预估公式；
- 达软顶继续认购；
- 到期未结算等待状态；
- 仅权威 Failed/Cancelled 退款；
- Successful Claim；
- 动态 Gas；
- 非法 tx hash；
- receipt revert；
- 暗色/移动端；
- BigInt 极值和 6/18 decimals；
- 跨页面刷新和 replacement/uncertain 交易；
- 未知版本、全零项目、过期 API 响应；
- permissionless finalize 和 Recovery Portal；
- 繁中、简中和英文风险披露。
- exact approve target、zero-first 两步独立确认和 allowance/input 变化；
- 每个 selector 的 unsupported-method fallback 必须先 staticCall 并使用审核 Gas 上限；
- Claim/Refund/两类提款/Failed Token withdrawal/claimFor/refundFor 专属确认页；
- transaction identity 跨 tab、刷新、logout、切钱包、切链、replacement 和 local-storage corruption；
- Recovery Portal 在 Backend/MySQL/Redis/JWT 下线时，校验签名 manifest、错误链、错误 Proxy、runtime mismatch 和不存在项目；
- disclosure ID 三语言 parity，缺失翻译阻止签名；
- axe + keyboard + screen reader + 200% zoom + high contrast + light/dark + mobile WCAG 2.2 AA。

### 17.5 Admin 测试

- 模式切换条件字段；
- Preview；
- 四眼审批；
- 发起人自批拒绝；
- execute 权限；
- onchain locked 后不可修改；
- contract version 对应动作；
- 交易/事件/账本筛选；
- 独立 pause 状态；
- reconciliation 页面。
- Preview digest 和 15 分钟失效；
- Approval 2 小时执行窗；
- 模式切换 forbidden fields；
- 高危动作三权分离；
- calldata/runtime 差异阻断执行。

### 17.6 E2E 场景

#### 场景 A：成功且超过 Soft Cap

```text
Soft Cap = 10 USDT
Sale Tokens = 1,000 FOS2
最终募集 = 25 USDT
```

验证：

- 第 10 USDT 后仍可继续贡献；
- endTime 前保持 Active；
- endTime 后先进入 SuccessfulPendingRelease；
- 至少 20 confirmations、runtime 正常且 reconciliation BALANCED 后 durable Release 为 Successful；
- 最终价格 0.025 USDT/FOS2；
- 多用户按比例 Claim；
- 项目方和平台提款；
- reconciliation BALANCED。

#### 场景 B：失败退款

```text
Soft Cap = 10 USDT
最终募集 = 7 USDT
```

验证：Failed、所有用户原额退款、项目方取回项目币、平台费为 0。

#### 场景 C：取消

Pending 和 Active 分别测试取消权限、退款和项目币取回。

#### 场景 D：舍入和永久 Claim

使用不能整除的贡献比例，确认 Claim 总和不超过销售总量、极小贡献可能为 0 raw unit、未领取用户权益不可 sweep、尘埃保持在 reserve。

#### 场景 E：共享 Token

两个 Fair Launch 项目使用同一项目币，验证聚合 reserve 与单项目资金事实均正确。

---

## 18. 部署与发布顺序

### 18.1 合约部署前

- 完成需求冻结；
- 合约测试、fuzz、invariant；
- storage layout；
- 独立安全审计；
- 多签/Owner/Operator/Fee Receiver 确认；
- Gate A 法务书面意见；
- Sign Policy Gateway + Web3Signer + AWS KMS POC 和灾难恢复演练；
- 第三轮 NO-GO 整改完成后的短周期 freeze re-check 达到至少 CONDITIONAL GO；
- BSC RPC 和历史查询能力确认。

### 18.2 合约部署

1. 部署并配置 48h Governance/Upgrade Timelock 和 24h Unpause Timelock；
2. 设置 2-of-3 Governance Safe 为两个 Timelock 的 proposer/canceller，executor 为 permissionless；
3. 部署 Implementation；
4. 部署 UUPS Proxy 并原子 initialize，`governanceTimelock` 参数直接使用已部署的 48h Timelock；
5. 验证 Proxy/Implementation；
6. 配置 Payment Token 和测试 Project Token；
7. 配置独立 2-of-3 Pause Guardian、KMS Fair Launch Operator和两个固定 beneficiary Safe attestation；
8. 通过 Safe/Timelock 验证治理调用后，撤销所有 deployer/temporary admin/proposer/canceller 角色；
9. 记录 start block、implementation code hash、ABI digest 和 Source revision；
10. 核对 Owner/Timelock/Guardian/Operator/Receiver/Fee/Pause 状态；
11. 验证 Sign Policy Gateway/Web3Signer mTLS、selector allowlist 和 signer-only 边界；
12. 禁止使用 Job/Operator 私钥做手工治理交易。

### 18.3 应用发布

建议顺序：

```text
DB migration
→ Backend
→ Job
→ Admin
→ Frontend
→ fairLaunchV1.enabled=true
```

上线采用 expand schema 和 disabled Source Registry；合约及应用验证完成后才启用 Source 和创建入口。一旦有 Fair Launch 项目上链，只能回滚到 Fair-capable maintenance image。

首次启用 Fair Launch Source 或创建首个项目时，写入 `fair_launch_feature_watermark`。Watermark 不能只存在于可回滚数据库；生产主机还必须有独立 enrollment/bootstrap trust root、主机单调状态和 Release Control Plane 签名状态。缺少任一必需根时是 unknown/fail closed，不得解释为 generation 0 或“从未启用”。

#### Environment bootstrap record

每个生产环境在 Agent 安装/主机 enrollment 时写入只读、独立于应用 DB 和 release artifact 的签名 bootstrap record。它由 Agent binary 内置 bootstrap root key set 验证，至少包含：

```text
domain = joincare.release.environment-bootstrap.v1
environmentId / chainId
fairLaunchProxyAddress 或 approvedDeploymentFactoryAddress
minimumDiscoveryBlock
releaseControlPlaneId / pinned HTTPS URL / TLS SPKI digest
watermarkRootKeySetDigest
sourceRegistryAuthorityKind = offchain-signed-registry
sourceRegistrySignerKeySetDigest
hostStateVolumeId / enrollmentGeneration
issuedAt / expiresAt nullable / previousBootstrapDigest
```

Source Registry 是 DB 中的 off-chain signed registry，不得描述为链上 Registry。链证据只从 bootstrap 固定的 Proxy/factory 和 minimum discovery block发现 code、deployment、`FairLaunchProjectCreated` 与升级事件。新主机必须执行 2-of-3 enrollment ceremony并取得当前 bootstrap、key-set、watermark和 Source Registry snapshot；已 enrollment 主机丢失 host monotonic volume 时必须停止 normal startup并重新 enrollment，不能把缺失视为零。Control Plane 和 host state同时不可用时，可用 bootstrap 固定链身份做 discovery；bootstrap 本身不可用或签名无效时拒绝正常启动。

#### Detached signed image capability envelope

Capability manifest 使用 RFC 8785 JCS，domain 为 `joincare.release.image-capability.v1`。它是 Release record 的**独立 sidecar/envelope**，不放入其所绑定的 tarball，避免 artifact hash 自引用：

```text
Release record
├─ artifact URL
├─ artifact SHA-256
├─ capability manifest canonical bytes
├─ capability manifest digest
├─ keySetVersion
└─ detached signatures[>=2]
```

Backend 与 Job 采用一个共享 `backend-job` image manifest generation和同一 image ID；Agent deployment state分别记录 Backend/Job 当前是否已部署该 generation，以支持 `backend → job` 的分步发布。Manifest 最小 schema：

```json
{
  "domain": "joincare.release.image-capability.v1",
  "environmentId": "joincare-mainnet",
  "generation": 12,
  "issuedAt": "2026-09-06T00:00:00Z",
  "expiresAt": "2026-12-06T00:00:00Z",
  "keySetVersion": 3,
  "component": "backend-job",
  "artifactSha256": "64 lowercase hex",
  "containerImageId": "sha256:...",
  "sourceCommit": "40 lowercase hex",
  "schemaMin": "2026_09_XX_expand",
  "schemaMax": "2026_09_XX_v2_constraints",
  "eventModel": "CE-V2-only",
  "capabilities": [
    "fair-launch.source-registry",
    "fair-launch.ingestion",
    "fair-launch.materialization",
    "fair-launch.outbox-recovery",
    "fair-launch.external-end-convergence",
    "fair-launch.claim-refund-projection",
    "fair-launch.reconciliation",
    "fair-launch.governance-workflows",
    "fair-launch.e8-payment-token"
  ],
  "maintenanceModes": ["write-disabled-replay", "signer-recovery"],
  "previousManifestDigest": "64 lowercase hex"
}
```

验证顺序固定：

```text
1. 拉取 detached manifest envelope
2. 以 bootstrap-pinned key-set chain 验证 threshold、generation、expiry和 signatures
3. 从已验证 manifest 取得 expected artifact SHA
4. 下载 artifact并核对 jc-release Release.SHA256 == manifest artifactSha256 == computed SHA256
5. docker load 后核对实际 image ID
6. Agent 原子持久化 accepted deployment attestation
7. 仅在 rollback eligibility/startup gate通过后创建或启动容器
```

#### Key-set rotation and revocation

签名 key-set record 固定为：

```text
domain / keySetVersion / threshold
keys[{keyId,algorithm=Ed25519,publicKey}]
validFromGeneration / validThroughGeneration
supersedesKeySetVersion / previousKeySetDigest
revokedAtGeneration nullable / revocationReason
issuedAt / signaturesByPreviousKeySet
```

旧 key set 不得签署超过 `validThroughGeneration` 的 manifest/watermark；新 key set 只有到 `validFromGeneration` 才有效。Emergency revocation record由当前未撤销 key set 2-of-3 签署并保存到 Control Plane和host monotonic cache。正常模式要求 revocation state不老于 24 小时；Control Plane 超过 24 小时不可达时只能继续当前已运行 image，禁止 upgrade/rollback到新 target；若当前 key或manifest已知 revoked则立即 maintenance fail closed。Host monotonic volume丢失必须重新 enrollment，不能依赖旧缓存恢复 rollback protection。

#### Restore-safe deployment watermark

签名 deployment watermark domain 为 `joincare.release.fair-launch-watermark.v1`：

```text
generation / environmentId / chainId / proxyAddress / implementationCodeHash
sourceRegistryRevision / firstEnabledBlock / firstProjectCreatedBlock nullable
minimumEventModel / requiredCapabilities[] / minimumSchemaVersion
minimumBackendJobManifestGeneration
previousWatermarkDigest / issuedAt / keySetVersion
```

Backend/Job 共用一个 `minimumBackendJobManifestGeneration`；各组件部署状态可不同，但都必须最终达到同一最低 generation。Release Control Plane 只能提高最低要求，不能删除 capability或降低 schema/event model。

三类可信来源使用 typed monotonic join，不使用含糊的标量 `max`：

```text
requiredCapabilities = union(all trusted requirements)
minimumBackendJobManifestGeneration = numeric max
minimumSchemaVersion = max by signed ordered schema-version registry
minimumEventModel = max by CE-V1 < CE-V2-shadow < CE-V2-read < CE-V2-only
proxy/factory/chain/environment identity = all sources exact equal；不一致即 incident
implementation/source revision = 按 canonical chain placement和签名 registry规则收敛；冲突 fail closed
```

Agent 在独立持久卷保存最后接受的 watermark/key-set/revocation/deployment-attestation digest和 generation。链证据依据 environment bootstrap 固定地址发现；DB Source Registry只作为签名 off-chain证据参与 typed join。任何来源证明 Fair Launch 曾启用，就不得因 DB restore或控制面短时不可用而降级。

#### Agent pre-apply gate 与 application pre-listener gate

Agent pre-apply gate负责：

```text
detached manifest/key-set/revocation signatures
artifact SHA / image ID
host watermark typed join
upgrade及rollback target eligibility
原子写 deployment attestation
```

Application pre-listener gate负责：

```text
读取只读挂载的 deployment-attestation digest
核对 running image ID / build commit / component role
读取 host watermark snapshot
双 RPC 核对 chain/proxy/deployment evidence
只读核对 DB schema、CE writer/reader generation、Source Registry signature
计算 effective typed requirements
决定 normal 或明确 maintenance mode
```

Backend/Job 进程不要求重新计算原始 tarball SHA；它信任 Agent 签名/持久化并只读挂载的 deployment attestation，同时核对自身 image/build identity。

启动顺序固定：

```text
1. Agent完成 target eligibility，容器尚未启动
2. Agent验证并原子持久化 deployment attestation
3. 进程启动但不监听 HTTP、不申请 worker lease、不连接 signer/broadcaster
4. application pre-listener gate完成所有 host/chain/DB检查
5. 满足要求才开放 listener/worker/signing/broadcast
```

自动 rollback 在恢复旧 image或启动旧容器**之前**执行相同 target eligibility。旧 target不满足 effective watermark时不得启动，只进入 maintenance并保留当前/失败容器证据；禁止“先回滚启动，再健康检查”。现有 jc-release 仅有 `Release.SHA256` 和事后 health check，生产实现必须扩展 Release metadata/Agent state记录 `capability_manifest_digest/key_set_version/watermark_generation/image_id/schema_range/deployment_attestation_digest`，且 backup failure 对资金组件必须阻止升级。

#### DB restore recovery attestation

DB 落后时只允许 `write-disabled-replay`：允许 canonical ingestion archive、Source backfill、reorg detection、receipt/nonce查询和 reconciliation dry-run；禁止 Admin mutation、项目创建/激活/取消/Release、Gateway签名、广播、财务 ledger finalization和对外“可执行”金融状态。

恢复完成不提高 deployment watermark；另生成签名 `joincare.restore-recovery-attestation.v1`：

```text
restoredSnapshotId / snapshotCreatedAt
bootstrapDigest / effectiveWatermarkDigest / deploymentAttestationDigest
replayFromCheckpoint / replayThroughBlock / replayThroughBlockHash
CE identity/placement/application roots
outbox/Gateway journal/EOA nonce reconciliation roots
Safe/Timelock/executor reconciliation roots
financial project/wallet/contract-token reconciliation roots
result = BALANCED / approverIds / completedAt
```

只有 recovery attestation 为 BALANCED、initiator/approver/executor 三权分离且与当前 effective deployment watermark一致时才恢复写流量。仅当产品最低 software/schema/capability 要求真正提升时才发布新 deployment watermark generation；重复 restore演练不伪造更高产品 generation。

Backend 与 Job 共用镜像时仍按：

```text
backend → job
```

升级 Job 后必须核对 `/proc/1/cmdline` 或 `docker top` 为 `/app/joincare-job`。

### 18.4 灰度启用

- 先在本地 Hardhat 全链路；
- 再测试链；
- 再 BSC 主网最小金额项目；
- 只允许白名单 Admin 创建首批 Fair Launch；
- 首个项目结束、Claim、提款、提费和 reconciliation 全部 BALANCED 后再开放常规创建。

### 18.5 回滚原则

- 已创建链上项目不可迁回 IDO V2；
- 已锁定项目不能切换 funding mode；
- 应用层可以关闭新建入口，但必须继续服务已有 Fair Launch 的贡献、结算、Claim 和退款；
- 合约紧急问题走 pause/upgrade，不得删除 DB 链上事实；
- 事件和账本修复走 reorg/replay/reconciliation，不得手工伪造交易记录。
- 回滚只关闭新创建和 privileged write；canonical ingestion、Source cursor、outbox recovery、settlement monitoring、reorg和 reconciliation 必须继续运行；
- 数据库 restore 后必须 write-disabled 启动，从可信 canonical checkpoint replay，完成 signer/nonce/reconciliation 后才恢复流量。

---

## 19. 验收标准

### 19.1 功能验收

- Admin 可选择 Fair Launch 并完成 Preview + 四眼审批；
- 创建上链后项目进入 Pending 且 `onchain_locked=1`；
- 项目方充值完整销售总量后可激活；
- 达 Soft Cap 后贡献继续成功；
- 无任何 Hard Cap 自动结束逻辑；
- endTime 后自动 durable 结算；
- 成功候选项目通过 20 confirmations/runtime/reconciliation 的链上 Release Gate；
- 成功项目所有用户按统一最终价格分配；
- 失败/取消项目可原额退款；
- 项目方和平台提款正确；
- V1 无截止 Claim/claimFor 和无截止 Refund/refundFor 正确，并按 E5 限定披露；
- C 端、Dashboard、Admin 正确展示模式和状态。

### 19.2 数据验收

每个关键动作必须同时核对：

```text
transaction
receipt status
event
chain snapshot
chain_tx_submission
active_slot
chain_relayer_nonce
projects
fair_launch_position
fund_ledger
reconciliation
public API
Admin API
C 端页面
```

### 19.3 安全验收

- 无私钥进入 Git、聊天、服务器配置或发布包；
- Operator nonce 无漂移；
- 无 unresolved durable failure；
- 无 dead letter；
- reserve solvent；
- 合约代码和配置身份匹配；
- 所有财务 reconciliation 为 `BALANCED`；
- 生产 Frontend 不含 Hardhat RPC、31337 或占位 WalletConnect ID。
- Primary/Secondary RPC identity 一致且生产 runtime verify 未跳过；
- Sign Policy Gateway 不能签任意地址、任意 selector、错误 digest 或错误 nonce；
- Recovery Portal 可在 Backend/JWT 下线时完成 end/claim/refund；
- 所有关键风险有繁中、简中、英文版本并在每次 Contribution 签名前确认。

---

## 20. Gate 决策结果与开放项

原 12 项产品问题及审计新增问题均已由 Owner 冻结，权威结果见 §0。关键结果包括：

```text
minContribution = 单次最低贡献，必须 > 0
maxContribution = 单钱包累计上限，0 表示不限
Pending 可申请取消；Active 到期前仅平台紧急治理取消
now >= endTime 后 cancel 永远 revert
endProject permissionless
Claim/Refund 在 V1 中无协议截止且无 sweep，受 E5 限定
未领取用户权益不可 sweep
提供 claimFor/refundFor，固定发送到原 user
V1 不做自动流动性，Claim 不依赖流动性
SuccessfulPendingRelease 完成客观 Release 检查后立即进入 Successful 并可提款
项目 Token 不允许超额充值
Fair Launch 使用独立 Operator 和远程 KMS 签名
```

产品选择已经完成；仍未完成的是工程规范复审和 Gate 证据：

1. Gate A 法务书面签署；
2. 第三轮整改后的短周期 freeze re-check 至少 CONDITIONAL GO；
3. Sign Policy Gateway + Web3Signer + AWS KMS POC；
4. 实现、测试、reorg/nonce/灾备演练和主网前独立合约审计。

在上述证据完成前不得将本文状态改为 GO。

---

## 21. 建议开发拆分

### Epic A：合约

- `JoinCareFairLaunchV1.sol`；
- 部署/升级脚本；
- ABI；
- 单测/fuzz/invariant；
- storage layout；
- 部署文档。

### Epic B：Backend 基础模型

- migration；
- entity/VO；
- trusted runtime registry；
- Set/Preview/审批；
- 公共 API。

### Epic C：Durable 与链事件

- Fair Launch client；
- outbox actions；
- shared nonce fence；
- event decoder/handler；
- auto end；
- reorg/replay/dead letter。

### Epic D：账本与对账

- fund ledger entry types；
- fair launch position；
- project/wallet reconciliation；
- Admin 审计页面。

### Epic E：Admin

- funding mode 表单；
- Preview/审批；
- lifecycle actions；
- pause；
- transaction/event/reconciliation 展示。

### Epic F：C 端

- ABI 和 contract routing；
- 列表/详情；
- contribute；
- estimated/final allocation；
- Claim/Refund；
- 项目方 Dashboard；
- 多语言、暗色和移动端。

### Epic G：发布与主网验收

- runtime 配置；
- CI/CD packages；
- 测试服务器；
- BSC 主网最小金额项目；
- 成功/失败/取消三分支；
- 守恒和上线实录。

---

## 22. 最终设计结论

JoinCare Fair Launch 第一版采用独立合约：

```text
JoinCareIDOv2
└─ capped_presale：固定价格 + Soft Cap + Hard Cap

JoinCareFairLaunchV1
└─ fair_launch：固定销售代币总量 + Soft Cap + 无 Hard Cap + 结束后统一定价
```

最关键的不变量：

```text
达到 Soft Cap ≠ 已结算 Successful
贡献发生时 ≠ 最终项目币 entitlement 已确定
页面时间结束 ≠ 链上终态已成立
无 Hard Cap ≠ 无代币储备约束
nonce 域由 chainId + sender 决定，不能按 contract address 拆分
```

补充冻结不变量：

```text
Fair = 统一清算价格 ≠ 反 Sybil/反鲸鱼
Successful ≠ 项目 Token 将来一定可转移
用户未 Claim ≠ 项目方可取回其权益
receipt success ≠ ledger applied
final_success ≠ signer fence 可释放
未知 contract version ≠ Legacy
API 显示状态 ≠ 钱包可直接签名
```

链上 `FairLaunchEnded(SuccessfulPendingRelease, ...)` 只冻结最终募集和价格；只有 `FairLaunchSuccessfulReleased(...)` 确认后，Claim、项目方提款和平台提费才正式开放。

当前 Gate A-D 和补充 E1-E8 均已冻结。第二轮和第三轮四通道设计审计均已完成，第三轮确定性发现和 E8-A 已回写；§18.4 短周期 freeze re-check 已通过，候选技术设计为 **FROZEN / CONDITIONAL GO**。Gate A 法务签署、签名 POC、真实实现、迁移与恢复演练及 §18.5 生产门禁仍未完成，因此总体继续保持 **NO-GO**。
