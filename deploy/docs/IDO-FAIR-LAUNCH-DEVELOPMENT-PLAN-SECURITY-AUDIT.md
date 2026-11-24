# JoinCare Fair Launch 开发计划多通道安全审计

> 审计对象：`docs/IDO-FAIR-LAUNCH-DEVELOPMENT-DESIGN.md`
> 审计性质：开发前架构、经济模型、资金安全和可实施性审计
> 审计方式：四个独立只读通道交叉审阅，主会话去重和裁决
> 审计日期：2026-09-06
> 当前结论：**候选技术 Freeze = FROZEN / CONDITIONAL GO（§18.4 六项机械条件已通过）；总体生产 = NO-GO**

> 历史说明：§4-§9 保留 2026-09-06 第一轮审计原始发现，不能被解读为每项仍然开放。Owner 随后冻结 Gate A-D、E1-E7，并在第三轮后确认 E8-A。第二/第三轮历史报告见 §14-§17，范围受限 freeze re-check和当前裁决见 §18；权威设计始终以 `IDO-FAIR-LAUNCH-DEVELOPMENT-DESIGN.md §0` 及其规范性附录为准。

---

## 1. 结论摘要

Fair Launch 的总体技术方向可行，以下基础决策应保留：

- 不修改现有 `JoinCareIDOv2`；
- 新建独立 `JoinCareFairLaunchV1` UUPS Proxy；
- `funding_mode=fair_launch` 与 `contract_version=fair_launch_v1` 分离；
- 固定 `saleTokenAmount`，到期后按贡献比例统一分配；
- 达到 Soft Cap 后继续保持 `Active`；
- `endProject` 为 O(1)，禁止遍历参与者；
- 认购时只形成支付贡献事实，不形成固定项目币 entitlement；
- Fair Launch 与 IDO V2 共用 Operator 时，nonce fence 按 `chainId + senderAddress` 共享；
- 第一阶段不实现 Pancake 自动流动性。

但当前计划仍存在会导致下列后果的阻断问题：

```text
项目方关联钱包自募资并稀释真实参与者
项目 Token 变更行为后用户无法 Claim，而项目方已提走稳定币
取消与到期结算竞争，终态由交易排序决定
Claim 截止后没收用户已取得权益
失败退款长期无人执行导致本金永久锁定
UUPS 单点治理可改变或转移全部托管资产
最终价格整数表示归零
reorg、事件身份、outbox fence 和多合约监听无法安全落地
未知合约版本回落到旧 ABI
前端 Number 精度损失和过期状态驱动金融操作
Admin Preview 与实际提交参数不一致
```

因此：

## **禁止在当前文档基础上冻结 Solidity ABI、存储布局或开始生产资金代码。**

允许继续的工作仅限：

- 无资金数学原型；
- mock Token 和测试夹具；
- Solidity 属性测试模型；
- DB/API 判别联合类型草图；
- 通用 runtime registry 原型；
- 不绑定最终 ABI 的 UI 原型；
- 风险和产品决策评审。

---

## 2. 审计范围

四个独立通道分别审阅：

1. **智能合约与经济模型**
   - 数学、舍入、Token 储备、Claim、退款、提款、Pause、UUPS、恶意 ERC-20；
2. **Backend 与可靠性**
   - DB、durable outbox、nonce fence、事件、reorg、账本、reconciliation、部署回滚；
3. **C 端和 Admin**
   - 合约路由、精度、状态权威、钱包交易、Preview、四眼审批和财务 UX；
4. **对抗性跨系统审计**
   - 自募资、鲸鱼、Sybil、锁资、治理、法律公平性和运营逃生路径。

审计同时参考了当前：

```text
JoinCareIDOv2.sol
Backend durable outbox
V2 chain event handler
reorg materialization/recovery
fund ledger
Frontend projectContext/contract/projectStatus
LaunchpadActiveFundingPanel
Admin onchain_params
```

本次未修改业务代码，未运行链上测试。

---

## 3. 风险分级

| 等级 | 定义 | 开发处理 |
|---|---|---|
| P0 / BLOCKER | 可能导致用户资产损失、永久锁资、终态被治理操纵，或会改变 ABI/存储的未决规则 | 编码前必须解决 |
| P1 / HIGH | 生产实现必然涉及，当前计划不足以安全实现或验收 | 主网代码冻结前必须解决 |
| P2 / MEDIUM | 不一定立即损失资产，但会造成审计、可用性、运营或用户理解风险 | 合并前解决或明确接受 |
| Residual | 技术无法完全消除，只能限制、披露和运营管理 | Owner 书面接受 |

---

## 4. P0 阻断项

### P0-1 项目方关联钱包自募资和末期稀释

#### 问题

Fair Launch 成功条件只有：

```text
finalTotalRaised >= softCap
```

项目方或关联方可以在结束前通过多个钱包投入大额稳定币：

1. 使项目达到 Soft Cap；
2. 制造虚假融资额和钱包参与数；
3. 稀释真实参与者最终分配；
4. 项目成功后由项目方收回大部分稳定币；
5. 关联钱包同时按比例取回项目 Token；
6. 实际主要成本为平台费和 Gas。

禁止 `projectOwner` 本地址参与或设置钱包上限不能阻止该问题，因为可以使用关联钱包和 Sybil 地址。

#### 审计裁决

“无 Hard Cap”不是该问题的唯一原因，但会放大最大稀释程度。增加一个高上限可以限制最坏稀释，却不能证明融资来自独立参与者。

#### 必须决策

必须明确选择：

```text
A. 公开 permissionless，接受无法技术识别关联方
B. KYC/身份 allowlist
C. 项目方保证金/惩罚机制
D. 募集款部分进入里程碑托管或独立流动性储备
E. 延迟提款 + 挑战/披露期
```

如选择 A：

- “Fair”只能表示统一清算价格；
- 不得宣传为身份公平、反鲸鱼或防自募资；
- 钱包数不得称作独立投资者人数；
- Admin 必须审查 Soft Cap 合理性、资金来源和关联交易风险；
- 用户风险披露必须明确末期大额贡献和关联地址风险。

#### 验收条件

- 形成书面经济安全说明；
- 对项目方控制多个钱包建模；
- 如果采用链上限制，属性测试证明最大损害边界；
- 不能把“禁止 projectOwner 地址”作为充分防护。

---

### P0-2 恶意或可变项目 Token 可阻止 Claim

#### 问题

精确充值和 decimals 校验只能证明 Token 在当时可以转账，不能防止充值后：

- Proxy 升级；
- 开启转账税；
- 暂停；
- 拉黑 Fair Launch 合约或用户；
- 改变 `transfer/balanceOf` 语义；
- rebase；
- 管理员增发并砸盘。

可能出现：

```text
用户稳定币已进入合约
→ 项目成功
→ 项目方提走稳定币
→ Token 行为被修改
→ 所有或部分用户永久无法 Claim
```

#### 必须修改

冻结项目 Token 准入策略。建议 V1 只接受：

```text
平台审计的标准 Token
不可升级或升级权受可信多签 + timelock 约束
无可变税费
无任意用户黑名单
无可暂停用户 Claim 的单方权限
无 rebasing
```

至少记录并展示：

```text
runtime code hash
是否 Proxy
implementation
Proxy Admin
mint 权限
pause 权限
blacklist 权限
tax 权限
```

同时需要：

- 成功后项目方提款延迟/挑战期；
- `payoutsPaused` 可阻止项目方和平台提款；
- Token 异常时 Claim 与提款的事件响应流程；
- 明确 generic ERC-20 无法保证未来可转移性。

#### 验收条件

使用可升级、可暂停、可拉黑、可变税 Token 测试：

- 不合规 Token 在准入时被拒绝；或
- Token 进入异常状态后，项目方募集款和平台费保持不可提取；
- 风险状态可在 C 端/Admin 清晰显示；
- 不得发生用户不能领币而项目方已不可逆提走款项。

---

### P0-3 Active 取消与 permissionless 到期结算竞争

#### 问题

当前计划同时允许：

```text
Active → Canceled
Active + now >= endTime → endProject()
```

若到期后仍允许取消，则 `cancelProject` 和 `endProject` 的排序决定最终状态。平台可以在已经确定成功后抢先取消，外部调用者也可能抢在紧急取消前完成结算。

#### 必须冻结为

```text
Pending:
  项目方或平台按审批规则取消

Active 且 block.timestamp < endTime:
  仅平台紧急治理可取消

Active 且 block.timestamp >= endTime:
  cancel 必须 revert
  只能 permissionless endProject
```

Active 取消还必须：

- 多签/四眼审批；
- 事件记录 `reasonCode` 和证据哈希；
- 明确支付币全部退回用户；
- 平台费为 0；
- 项目 Token 可由项目方取回；
- 取消不得与普通 pause 混同。

#### 验收条件

在 `endTime-1`、`endTime`、`endTime+1` 和同区块排序测试中：

- 到期后 cancel 永远失败；
- 任何地址都能执行确定性结算；
- 交易排序不能改变到期后的成功/失败结果。

---

### P0-4 Claim 截止后没收用户权益

#### 问题

当前建议：

```text
最后一期 Vesting 后 365 天
→ Claim 永久关闭
→ saleTokenAmount - totalTokensClaimed 全部给项目方
```

该余额混合了：

- 数学舍入尘埃；
- 用户已经通过成功募资取得、但尚未领取的权益。

在 O(1) 结算且没有遍历/快照所有用户分配的模型中，合约不能简单区分两者。

#### 风险

用户可能因主站、RPC、Token pause/blacklist、司法、继承或钱包恢复问题错过期限。直接把余额交给项目方具有重大公平和法律风险。

#### 建议 V1 选择

最安全的 V1 方案是：

```text
用户 Claim 永不过期
不 sweep saleTokenAmount 内的余额
只允许提取 totalTokensDeposited - saleTokenAmount 的明确超额充值
接受极少量舍入尘埃永久留在合约
```

如果业务必须设置期限：

- 未领取用户权益转入长期 Claim Escrow/Treasury，不得直接给项目方；
- Deadline 只能延长，不能缩短；
- 必须取得法务意见；
- 必须提供独立恢复页面和充分通知；
- 尘埃与用户权益必须有可证明的分离方案。

#### 验收条件

- Owner 对“永不过期”或“Escrow”书面选择；
- 不允许开发人员默认把未领权益给项目方；
- Claim/sweep 时间边界无重叠；
- 如果存在 Deadline，法务和资产处置规则必须先完成。

---

### P0-5 失败/取消退款可能永久锁资

#### 问题

仅提供用户主动 `refund()` 会使遗失钱包、前端下线或长期不操作用户的本金永久留在 reserve。

退款设置过期并转给项目方又会构成没收用户本金风险。

#### 必须修改

建议：

```solidity
refund(projectId)
refundFor(projectId, user)
```

其中 `refundFor`：

- 任何地址可触发；
- 资金只能发送到链上记录的用户地址；
- 不允许调用者指定收款地址；
- 用户本金退款原则上不过期；
- Batch 只作为链下循环优化，不作为唯一恢复路径；
- Batch 有最大长度，单用户失败不能永久阻断其他用户的独立退款。

需要交付无 Backend 依赖的退款恢复页面和公开 ABI。

稳定币发行方拉黑用户或合约时可能无法退款，该风险只能披露和运营标记，不能伪装成普通 RPC 重试。

#### 验收条件

- 任意第三方可替用户触发退款，但无法改变收款人；
- 退款无普通业务过期；
- 用户单独退款始终可用；
- 平台/主站停止后仍可直接链上恢复；
- `TOKEN_FROZEN/TRANSFER_BLOCKED` 有独立状态和 runbook。

---

### P0-6 UUPS 治理仍是托管级单点

#### 问题

`Ownable2Step` 只降低误转 Owner 风险，不能防止 Owner 被盗或主动部署恶意实现。Owner 可以通过升级改变资金规则或转走全部资产。

#### 必须修改

- Upgrade Owner 必须是多签，不得是单 EOA；
- 非紧急升级必须有 timelock；
- 公开待升级 implementation 和 code hash；
- Pause Guardian 与 Upgrade Authority 分离；
- 禁用 `renounceOwnership`；
- implementation constructor `_disableInitializers()`；
- Proxy 部署和 `initialize` 原子完成；
- `_authorizeUpgrade` 明确绑定升级多签/timelock；
- Active/Successful 在途项目升级需要单独策略；
- 页面公开 Proxy、Implementation、Owner、多签门限和 timelock；
- 用户风险披露必须承认升级治理可改变合约行为。

#### 验收条件

- implementation 无法直接初始化；
- Proxy 不能重复初始化；
- 非授权地址不能升级；
- upgrade storage layout 测试覆盖多项目、共享 reserve、贡献、Claim 和退款；
- 升级前后所有状态 bit-for-bit 保持；
- storage gap 大小和 V1 字段顺序在编码前冻结。

---

### P0-7 最终价格可能归零

#### 问题

```text
finalTokenPriceRaw
= floor(finalTotalRaisedRaw × 10^projectTokenDecimals / saleTokenAmountRaw)
```

极端参数下成功项目的 `finalTokenPriceRaw` 可以为 0。这会破坏 API、审计和未来流动性定价。

#### 必须修改

用户 entitlement 继续使用直接比例公式，不允许使用已舍入价格反推。

价格权威事实建议保存为：

```text
finalTotalRaisedRaw
saleTokenAmountRaw
paymentTokenDecimals
projectTokenDecimals
```

展示价格使用：

- 明确高精度 `priceX18/priceScale`；或
- numerator/denominator 有理数。

同时创建时校验最小成功融资额在选定价格精度下不会归零。

#### 验收条件

- 混合 6/18、18/6、8/18 decimals；
- 极低价格；
- 不可整除金额；
- `Successful ⇒ displayed final price representable and nonzero`；
- allocation 与价格显示舍入相互独立。

---

### P0-8 当前 reorg/replay 基础设施不足以安全接入 Fair Launch

#### 问题

设计只列出需要纳入 reorg 的表和模块，但未定义：

- 每个 Fair Launch 事件的 materialization effect；
- inverse effect；
- 依赖 DAG；
- common-ancestor 快照；
- outbox/nonce generation binding；
- exact/conditional/blocked/no-op 分类；
- incident close-out。

现有 Backend 的 reorg prerequisite inventory 仍明确存在未完成前置条件。直接增加 Fair Launch handler 会使 contribution、end、claim、refund 的反转和依赖无法安全处理。

#### 必须修改

在设计中增加完整 Fair Launch reorg 规范：

```text
contribution → position/project aggregates
contribution → refund eligibility
successful end → final allocation/claim
claim/refund/withdrawal → ledger/reserve projections
```

并定义：

- 旧分支事件保留为 `reverted`；
- 替代分支重新 materialize；
- participantCount 只反转首次贡献增量；
- 后续 Claim/Refund 的依赖如何阻断或补偿；
- cursor 只有在状态和 nonce/outbox 证明一致后才能推进。

#### 验收条件

自动化测试完成：

```text
贡献 → 结算 → Claim
→ 替换贡献所在分支
→ 检测并冻结
→ materialize 两分支
→ 正确分类依赖
→ replay/blocked
→ 无重复 ledger/position/participant
→ nonce/outbox 一致
→ reconciliation BALANCED
→ 无手工 SQL 关闭 incident
```

---

## 5. P1 高风险项：合约和资金状态机

### P1-1 Pause 必须拆分权限域

单一全局 Pause 同时存在两种冲突目标：

- 阻止资金进一步流出；
- 不冻结用户 Claim/Refund。

建议拆分：

```text
contributionsPaused
payoutsPaused
claimsPaused（仅确认 Claim 路径有漏洞时）
```

默认语义：

| 操作 | contributionsPaused | payoutsPaused | claimsPaused |
|---|---:|---:|---:|
| contribute | 禁止 | 不影响 | 不影响 |
| endProject | 允许 | 允许 | 允许 |
| refund | 允许 | 允许 | 允许 |
| claim | 允许 | 允许 | 禁止 |
| owner proceeds | 不影响 | 禁止 | 不影响 |
| platform fee | 不影响 | 禁止 | 不影响 |
| excess/sweep | 不影响 | 禁止 | 不影响 |
| cancel | 按独立治理规则 | 禁止普通执行 | 不影响 |

必须另行冻结 Pause 是否停止销售时钟。推荐 V1 不改变链上时钟；若暂停持续过久，平台只能在 `endTime` 前按紧急多签取消。到期后必须确定性结算。

---

### P1-2 项目 Token 取回状态机不完整

必须定义三个独立动作和事件：

1. Failed/Cancelled 全额项目 Token 取回；
2. Successful 超额充值取回：`totalDeposited - saleTokenAmount`；
3. 未领取权益处理：按 P0-4 决策，V1 建议不提供项目方 sweep。

每个动作都要独立 flag、事件和 reserve delta，禁止使用含义模糊的通用 `withdrawProjectTokens()`。

---

### P1-3 `recoverExcessTokens` 未定义且可能破坏聚合 reserve

恢复函数必须：

- `onlyOwner` + `nonReentrant`；
- `amount <= balance - reservedBalances[token]`；
- reserve 不变；
- 核对 sender decrease 和 recipient increase；
- 转账后再次验证 `balanceAfter >= reserve`；
- 共享 Token 多项目测试。

不能直接复制当前 V2 只调用 `safeTransfer` 的恢复路径。

---

### P1-4 Contribution 必须复核 Token 运行时身份

每次贡献前检查：

- payment/project Token 仍在当前项目允许状态；
- 两个 Token 地址仍有 code；
- runtime decimals 与快照一致；
- 关键 Token runtime identity 未违反准入策略。

Token delist 规则必须区分：

- 禁止新项目/新贡献；
- 已失败项目仍允许退款；
- 已成功项目 Claim 按独立 incident policy 处理。

---

### P1-5 Vesting 校验不完整

必须要求：

```text
1 <= length <= 10
times.length == percents.length
times[i] > times[i-1]
percents[i] > 0
sum(percents) == 10,000
claim policy 明确
```

是否允许首期时间等于 `endTime` 需要冻结。若允许，仍必须等链上 `Successful` 后才可 Claim。

---

### P1-6 零分配风险

在无 Hard Cap 且向下取整模型中，非零贡献最终可能获得 0 个项目 Token raw unit。静态 `minContribution` 无法保证，因为后续总募集额可以持续增加分母。

必须选择：

- 引入总募集上限，结合 Token 精度保证最小分配；或
- 明确接受并在签名前披露“最终分配可能向下取整为 0”；或
- 限制项目参数使该风险在业务范围内不可发生。

不能在结算后简单退回零分配用户，因为移除其贡献会改变所有人的分母并破坏 O(1) 模型。

---

### P1-7 Active 贡献是否可撤回必须明确

当前模型实际上是：

```text
Active 阶段贡献不可撤回
仅 Failed/Cancelled 可退款
```

建议 V1 保持不可撤回，但签名前必须明确：

- 最终价格未确定；
- 后续大额贡献会稀释最终分配；
- 没有最低最终可得 Token 保护；
- Approve 不等于贡献成功；
- 到期前用户不能主动退出。

如果产品要求可撤回，需要重新设计 Soft Cap 操纵和挤兑风险，不能局部增加 withdrawContribution。

---

### P1-8 Manual Liquidity 风险披露

第一阶段项目方成功后可能取得全部净募集款，而平台不保证建池或锁仓。必须在列表、详情、认购确认和 Admin Preview 明确：

```text
No automatic liquidity
No LP lock enforced by JoinCare
Project owner controls withdrawn proceeds
Token may have no secondary market
```

不得使用“自动上市”“保证流动性”等文案。

---

## 6. P1 高风险项：Backend、Job 和数据

### P1-9 outbox signer fence 释放规则表述错误

原计划写“`active_slot` 在 final/applied 后释放”，其中 `final_success` 不应释放 signer fence。

正确矩阵：

| 结果 | Submission | 释放 signer |
|---|---|---|
| 未签名管理取消 | cancelled | 是 |
| confirmed revert | final_revert | 是 |
| confirmed success、事件未应用 | final_success | **否** |
| canonical event applied | applied | 是，原子执行 |
| broadcast unknown / ambiguous | unknown/ambiguous | 否 |

`fair_launch_pause/unpause` 也必须有 `Paused/Unpaused` 事件推进 `applied`，否则可能永久占用 V2 与 Fair Launch 共用的 signer。

---

### P1-10 多合约监听仍停留在 V1/V2 硬编码模型

现有代码会把“非 V2 地址”导向 V1，Fair Launch 日志可能被错误解码并阻断全局 cursor。

需要通用持久化 source registry：

```text
chain_id
contract_address
contract_version
enabled
start_block
proxy_kind
implementation_address
implementation_code_hash
abi_version
confirmation_policy
registered_at
retired_at
```

并明确选择：

- chain-wide cursor；或
- per-source cursor。

所有 decoder、runtime verification、failure、reorg 和 replay 都从 registry 选路，禁止 `if V2 else V1`。

---

### P1-11 写入前缺少 runtime identity 原子边界

仅在配置读取或事件消费时验证 Implementation 不够。必须在模拟、nonce 保留和签名前：

1. 固定读取块；
2. 核对 Chain ID；
3. 读取 EIP-1967 implementation slot；
4. 核对 implementation code hash；
5. 核对 ABI/runtime epoch；
6. 执行 state preflight/`eth_call`；
7. 之后才允许 nonce reservation 和签名。

Runtime 不一致必须在签名前失败，不能消耗 nonce。

---

### P1-12 permissionless 终态和 durable submission 的竞态不完整

外部地址可能在平台 action 处于以下任一状态时先完成 `endProject`：

```text
intent
ready
signed
submitted
broadcast_unknown
mined_success
final_success
ambiguous
```

unsigned intent 可以取消；已签名/已广播 action 不能简单删除，必须继续由 nonce recovery 确认最终结果，并识别 `AlreadyEnded` 为被外部终态取代后的预期结果。

每种状态都必须测试并最终达到：

- 一个 canonical terminal effect；
- 无重复 attempt；
- 无 dead letter；
- signer fence 释放；
- ledger/project 状态正确。

平台费提款必须明确：

- permissionless trigger + 无 submission 事件合法；或
- Owner/Operator 限制 + 强制 durable submission。

不能合约 permissionless、Backend 却要求 submission。

---

### P1-13 链事件身份不兼容 re-inclusion 和多链（第一轮历史提案；第三轮已纠正）

事件身份应统一为：

```text
chain_id + tx_hash + tx_log_ordinal
```

RPC `logIndex`、`block_number/block_hash` 是 canonical placement，不是事件唯一身份。

需要迁移：

- `chain_event_record`；
- `chain_event_failure`；
- replay/materialization 关联表；
- 所有 lookup 和 resolution path。

必须测试同一 tx/log 在 reorg 后不同 block 重新纳入时不产生重复 ledger 和投影。

---

### P1-14 `DECIMAL(65,18)` 不能覆盖 uint256 领域

`DECIMAL(65,18)` 只有 47 位整数容量，而有效 uint256 经 6/18 decimals 显示后可能远超该范围。

建议：

- raw string 是唯一权威；
- display 使用 canonical decimal string；
- 不用 DB decimal 做 reconciliation；
- 或在合约创建前强制统一业务上限，证明所有列均可容纳。

`fair_launch_position` 的 display 字段也必须明确 SQL 类型。

---

### P1-15 Ledger entry type 超长且资金流不完整

部分拟议类型超过当前 `entry_type varchar(32)`：

```text
fair_launch_project_proceeds_withdrawn
fair_launch_platform_fee_withdrawn
fair_launch_excess_project_tokens_withdrawn
fair_launch_unclaimed_project_tokens_swept
```

建议使用稳定、版本无关的语义类型，并用独立 `contract_version/funding_mode` 分类：

```text
project_token_deposited
payment_contributed
payment_refunded
project_token_claimed
project_proceeds_paid
platform_fee_paid
project_tokens_withdrawn
```

还必须补齐 Failed/Cancelled 项目 Token 全额取回的事件和 ledger 事实。

---

### P1-16 缺少 contract-token 聚合 reconciliation

现有计划只有项目级和钱包级检查。共享 Token 时还需要：

```text
chain_id + contract_address + token_address + evidence_block
```

```text
expectedAggregateReserveRaw
= sum(expectedProjectReserveRaw)

chainReservedBalanceRaw == expectedAggregateReserveRaw
erc20BalanceRaw >= chainReservedBalanceRaw
```

否则单项目看似 BALANCED，跨项目 reserve 归属仍可能错误。

---

### P1-17 自动结算时间和 chain preflight 未冻结

不能用容器本地 `time.Now()` 判断是否应该签名。必须：

- DB session TZ 启动时验证；
- 持久化精确链 epoch；
- Job 先读链上 `status/endTime/block.timestamp`；
- 未到期是 no-sign/no-nonce；
- runtime identity 校验通过后才 reserve nonce。

测试需让 Backend、Job、MySQL 使用不同 TZ，并证明到期前不会创建交易。

---

### P1-18 回滚策略不能回到不支持 Fair Launch 的旧版本

第一个 Fair Launch 项目创建上链后，回滚不能使用当前 pre-Fair Backend/Job。

必须维护 Fair-capable maintenance image，回滚含义只能是：

```text
关闭新建入口
继续事件消费、自动结算、Claim/Refund 投影、outbox recovery 和 reconciliation
```

若数据库存在非终态 Fair Launch 项目，不支持 Fair 的 Job 必须启动失败，而不是静默运行。

---

## 7. P1 高风险项：C 端与 Admin

### P1-19 未知 contract version 可能回落 Legacy ABI

当前前端 `!isV2Project()` 会进入 legacy 路径。必须改为穷举 allowlist：

```text
legacy_jcc
v2_stablecoin
fair_launch_v1
otherwise → fail closed
```

不得为声明了非 legacy 版本的项目补 legacy 默认地址。

Fair Launch 合约还必须有明确 exists 语义：

- `projectExists` mapping；并且
- `getProject` 对不存在 ID revert，或返回显式 `exists`。

全零 tuple 绝不能映射为 Pending。

---

### P1-20 禁止 JavaScript `Number` 参与财务计算

当前前端大量使用 `Number()` 计算 raised、cap、quota、price 和 token amount。Fair Launch 必须制定强制规则：

```text
API raw = decimal string
内部计算 = bigint
用户输入 → normalize string → parseUnits
显示 = formatUnits / decimal string formatter
```

禁止：

```text
Number(raw)
浮点除法计算价格/分配
对财务 Number 使用 toLocaleString
type=number 接受科学计数法
```

预估必须与 Solidity `mulDiv floor` 完全一致。

---

### P1-21 金融动作缺少 freshness/finality 约定

API 需返回：

```text
chainBlockNumber
chainBlockHash
confirmations
observedAt
stateSource
isFinalized
reconciliationState
```

每次签名前重新链读：

- 项目存在和完整身份；
- status；
- balance；
- allowance；
- user position；
- contribution limit；
- state simulation/estimateGas。

旧区块异步响应不得覆盖新区块状态。API/RPC 都不可用时，不能用缓存展示数据开放金融按钮。

---

### P1-22 所有钱包写入必须共用交易状态机

不仅 `contribute/claim/refund`，以下也必须统一：

- payment/project Token approve；
- deposit；
- owner proceeds；
- excess withdrawal；
- fee withdrawal；
- 任何未来 sweep。

状态机：

```text
preflight
→ awaiting_wallet
→ broadcast_unknown | hash_received
→ mined_success | mined_revert
→ confirmations_pending
→ confirmed
→ projection_pending
→ applied
```

重要修正：`estimateGas` 超时并不能证明估算方法不支持，不能自动 fallback 发送。只有明确 `-32601/Method not found/unsupported method` 才允许 fallback。

Pending/uncertain 交易必须跨页面刷新持久化，禁止诱导用户重复发送。

---

### P1-23 Admin Preview 未绑定实际提交 payload

当前 Admin 可在 Preview 后继续修改表单，提交时重新从当前表单构造 payload，可能出现：

```text
人工确认 A
实际提交 B
```

Preview 必须返回：

```text
previewId
canonicalPayload
payloadDigest
schemaVersion
expiresAt
trustedRuntimeSnapshot
```

Submit 只能提交 `previewId + payloadDigest + idempotencyKey`，服务端执行精确的 canonical payload。任何字段、模式、项目或受信配置变化都必须使 Preview 失效。

审批记录、calldata 和 ProjectCreated 事件建议共同绑定 canonical `configHash`。

---

### P1-24 unavailable/not-applicable/pending 不能都用 0

当前 API 示例把下列情况混为 0：

- 无 Hard Cap；
- 尚未结算最终价格；
- 当前无募集，估算价格不存在；
- maxContribution 无限制；
- claimDeadline 不存在；
- 数学真实结果为 0；
- 数据缺失。

必须使用显式类型或至少 `null + state`：

```json
{
  "finalTokenPrice": {
    "state": "pending_settlement",
    "raw": null,
    "display": null
  },
  "hardCap": {
    "state": "not_applicable",
    "raw": null,
    "display": null
  }
}
```

Fair Launch 不能显示 `Hard Cap: 0`，也不能把 `claimDeadline=0` 显示为 1970。

---

### P1-25 Approval UX 必须默认精确金额

- 服务层不得默认 `max`；
- 显示完整 Token 地址、spender Proxy 地址、Chain ID 和 exact amount；
- 输入变化后重新核对 allowance；
- 支持需要 zero-first approve 的 Token，但两笔交易必须分别确认；
- 第一笔状态不明时禁止自动发第二笔；
- 项目结束后显示 residual allowance/revoke 指引。

---

### P1-26 Claim/Refund/提款签名前信息不足

签名前必须显示：

**Claim**

```text
final allocation
vested
claimed
claimable
deadline/policy
destination
```

**Refund**

```text
full contribution amount
payment Token address
destination wallet
authoritative terminal state
```

**Owner proceeds**

```text
finalTotalRaised
fee rate
fee amount
owner net
receiver
one-time effect
```

**Project Token withdrawal** 必须区分失败取回、超额取回和用户权益处理。

---

### P1-27 四眼审批范围不足

除 Set 外，下列动作至少需要双人控制、digest 绑定和执行前复核：

- Active cancellation；
- pause/unpause；
- platform fee withdrawal（若由 Admin 发起）；
- Token allowlist/受信 runtime/Operator/Fee 配置；
- UUPS upgrade；
- 任何用户未领取权益处置。

审批需绑定：

```text
chain + contract + version + project + action + args + expected state + digest
```

并记录 initiator、approver、executor、reason、MFA/step-up 和 tx。

---

### P1-28 模式切换会残留隐藏字段

Admin 请求必须是真正的 discriminated union。模式切换时：

- 清除/隔离不兼容字段；
- Preview 失效；
- 构造请求时只允许当前模式字段；
- Backend 对 forbidden fields 即使为 0 也拒绝；
- 两浏览器标签竞争由服务端 `onchain_locked` 和版本检查阻止。

---

## 8. P2 中风险和工程完善项

1. `batchRefund` 最大数组长度和失败隔离；
2. `minContribution >= 0` 对 uint256 无意义，应明确 0 的业务语义；
3. 平台费提款 caller 语义必须与 durable event policy 一致；
4. Claim 边界必须明确为半开区间，禁止 Claim 和资产处置同一时间均可执行；
5. Permissionless `endProject` 无调用奖励，需要 Job SLA 和直接 UI；
6. 可考虑 Fair Launch 使用独立 Operator，降低与 V2 共用 nonce 的爆炸半径；
7. 关键地址丢失/制裁恢复规则需要确认；
8. 小额 Claim/Refund Gas 高于资产价值时的代付策略；
9. Financial UI 达到 WCAG 2.2 AA；
10. 测试增加跨标签、页面刷新、交易 replacement、DST、长数字和高对比度；
11. 项目 Token 权限必须在 C 端公开展示；
12. 暂停、升级和 Token 异常必须有公开公告事件和状态页。

---

## 9. 仍然存在且无法完全消除的残余风险

即使修复上述全部问题，仍无法完全消除：

- 公开链 Sybil 和关联方参与；
- 末期鲸鱼稀释与区块排序；
- USDT/USDC 发行方冻结；
- 项目 Token 市场价值归零；
- 项目方不建立流动性；
- UUPS 多签和 timelock 的残余中心化；
- 公链拥堵、MEV 和 RPC 故障；
- 用户私钥遗失；
- 司法和制裁导致的资产不可转移。

这些风险必须在产品条款和签名前确认中披露，不能标记为“技术已解决”。

---

## 10. Owner 决策记录

Owner 已完成 Gate A-D 全部产品与架构选择。权威详细版本已回写：

```text
docs/IDO-FAIR-LAUNCH-DEVELOPMENT-DESIGN.md §0
```

### Gate A 已冻结（第一轮历史记录；E5/E8 和当前 Gate 见 §14-§17）

- Public permissionless participation；
- Fair 仅指统一最终清算价格；
- 无 Hard Cap；
- 项目方关联钱包风险只做披露、Admin 尽调和异常监控，不采用 KYC、保证金或挑战期；
- Active Contribution 不可撤回；
- 接受极端情况下最终分配向下取整为 0 raw unit；
- Claim 在 V1 中无协议截止且无 sweep；
- Failed/Cancelled 本金退款在 V1 中无协议截止，并提供 permissionless `refundFor`；
- 到期后 cancel 永远 revert，`endProject` permissionless；
- V1 不做自动流动性，Claim 不依赖流动性；
- Successful 后项目方可立即提款；
- 选择开发前取得正式法务书面意见。

### Gate B 已冻结

- 项目 Token 使用严格准入：不可升级、无可变税、无 rebase、无 pause/blacklist、无保留增发权；
- 禁止超额充值，激活要求精确等于 `saleTokenAmount`；
- 三域 Pause，Pause 不延长销售时间，Refund/End 永远可用；
- 永久 Claim，增加 permissionless `claimFor`，不提供用户未领取权益 sweep；
- UUPS 使用 2-of-3 Upgrade Safe + 48h timelock；独立 2-of-3 Pause Guardian；Unpause 24h timelock；
- 禁止紧急升级绕过 timelock；
- 使用独立 Fair Launch Operator；取消只能通过 Operator durable outbox；
- receiver、fee 和 Token 在创建时快照，提款只能到快照 beneficiary。

### Gate C 已冻结

- BSC 主网首发；链级 ingestion + Source 级 materialization cursor；
- Source Registry 是选路权威；第一轮曾采用 `chain_id + tx_hash + log_index`，第三轮已纠正为 tx-local ordinal，见 §17；
- 展示 5 confirmations、资金事实 finalized 20 confirmations；
- Primary/Secondary RPC，生产禁止跳过 runtime verification；
- 签名架构为 `JoinCare Sign Policy Gateway + Consensys Web3Signer + AWS KMS secp256k1 非导出密钥`；
- Web3Signer 只签名，durable outbox 继续唯一管理 nonce、广播、receipt、recovery 和 applied；
- final_success 不释放 signer fence；
- 三层 reconciliation、source-isolated failure、Fair-capable maintenance rollback 和受控 Operator 轮换。

### Gate D 已冻结

- 未知 contract version 和不存在项目 fail closed；
- 金融签名前直接链读，API 提供 provenance/finality；
- 全链路 decimal string + bigint；
- Estimated/Final 严格区分，Fair Launch 不显示 Hard Cap；
- 持久化钱包交易状态机和严格 Gas fallback；
- Approve 默认精确金额，每次 Contribution 签名前确认全部风险；
- permissionless Finalize、独立 Recovery Portal、Claim/Refund 不依赖 JWT；
- Admin Preview digest 有效期 15 分钟，Approval 执行窗 2 小时；
- 扩展四眼审批，高危动作三权分离；
- 关键风险在详情页和每次签名前以繁中、简中、英文展示。

### 历史快照：第一轮结束时尚未完成的证据（已被 §16-§17 取代）

- [ ] Gate A 法务书面意见；
- [x] 第二轮设计审计（已完成，结论 NO-GO）；
- [ ] Sign Policy Gateway + Web3Signer + AWS KMS POC 和恢复演练；
- [ ] 实现、测试、reorg/nonce 演练和主网前独立合约审计。

---

## 11. 允许开发重新开始的入口条件

### Gate A：产品和法律

- 产品选择和风险披露规则已冻结；
- **仍阻断：开发前法务书面意见尚未取得。**

### Gate B：合约安全规范（第一轮历史入口，当前状态见 §16-§17）

- Token、Pause、取消、Claim/Refund、UUPS、价格和零分配策略均已完成产品级冻结；
- 第二轮和第三轮均已完成且为 NO-GO；当前需整改后 freeze re-check；
- 仍需实现后合约单测、fuzz、invariant、storage layout 和独立安全审计。

### Gate C：Backend 可靠性

- source/runtime registry、outbox release、permissionless 竞态、event identity、reorg、聚合 reconciliation 和 maintenance rollback 的目标规则已冻结；
- 仍需完成详细 schema/状态转换设计、自动化测试和灾难恢复演练；
- 仍需完成 Sign Policy Gateway + Web3Signer + AWS KMS POC。

### Gate D：C 端与 Admin

- contract routing、project existence、BigInt、provenance、钱包状态机、Preview digest、四眼审批和 typed semantics 的产品规则已冻结；
- 仍需实现、测试、可访问性验收和 Recovery Portal 演练。

本段为第一轮历史结论，已被 §16-§17 取代。当前 Gate A-D 和 E1-E8 决策层已完成，第二轮和第三轮审计也已完成但均为 `NO-GO`；法务、签名 POC和技术整改后的 freeze re-check 仍阻断。

---

## 12. 建议的修订顺序

```text
1. Owner 决策矩阵
2. 经济与治理安全规范
3. Token 准入与提款保护
4. 合约状态机、Pause、Claim/Refund 权益
5. 价格精度与参数范围
6. UUPS storage/initializer/governance
7. Backend source registry 与事件身份
8. outbox terminal convergence 与 reorg
9. ledger/reconciliation/schema
10. C 端 BigInt、路由、交易状态机
11. Admin Preview digest 与扩展四眼审批
12. 更新测试矩阵
13. 再次设计审计
14. 冻结 ABI/storage
15. 开始生产实现
```

---

## 13. 最终审计意见

### 当前状态：**NO-GO**

不建议废弃 Fair Launch 方案。其双合约架构、比例分配公式和 O(1) 结算是正确基础。

真正的问题是：当前计划尚未把经济信任、Token 可变性、用户权益到期、取消竞争、治理中心化和现有 Backend/reorg 约束转化为可执行规范。

在这些问题未解决前直接编码，最危险的结果不是普通功能 Bug，而是：

```text
用户稳定币已被项目方提走，但项目 Token 无法领取；
到期项目最终状态由平台 cancel 与公开 end 的交易排序决定；
用户未领取权益或未退款本金被永久锁定或错误处置；
canonical 链事件已发生，但 outbox、ledger、cursor 或 signer fence 无法收敛；
前端因错误 ABI、浮点精度或过期状态请求用户签署错误交易。
```

第二轮四通道审计已完成且结论为 NO-GO。Owner 已完成 E1-E8 选择；第三轮也已完成并仍为 NO-GO，当前必须完成第三轮整改、Gate A 法务、签名 POC和后续 freeze re-check，方可冻结 `JoinCareFairLaunchV1` 的 ABI、事件和存储布局。

---

## 14. 第二轮四通道审计补充报告

### 14.1 执行记录与结论

第二轮采用四个独立只读通道：合约/治理、Backend/signer/reorg、Frontend/Admin/UX 和跨系统一致性。Backend 通道首次因 provider stream 断开失败，已按同协议重试成功；四通道最终全部有效完成。

```text
结论：NO-GO

已确认实质解决或接受：
  永久 Claim/Refund、到期禁止 cancel、三域 Pause、精确充值、
  O(1) 比例分配、有理数价格、显式项目存在、BigInt、
  Source Registry 方向、三层 reconciliation 和独立 Fair Operator。

第二轮 ABI/设计阻断：
  链下 finality/BALANCED 无法约束直接提款；
  runtime mismatch 可能造成无终态锁资；
  Safe/Timelock/Guardian 拓扑未冻结；
  Token admission 未形成项目快照 attestation；
  ABI/events/errors/storage 不完整；
  projectProceedsReceiver 未贯穿 Set/Preview/digest；
  beneficiary 恢复、永久权益限定语义未冻结；
  nonce/signing journal、Source revision/cursor、event identity migration、
  permissionless-end/reorg convergence、Recovery Portal 和金融 UX 不完整。
```

### 14.2 Owner E1-E7 决策

Owner 全部采用推荐方案：

| 决策 | 冻结结果 | 权威设计 |
|---|---|---|
| E1 | `SuccessfulPendingRelease` + 20 confirmations/runtime/BALANCED attested Operator Release | §0.7, §7.7 |
| E2 | Release 前 runtime mismatch → Failed(TokenRuntimeInvalid)+永久退款+零平台费；正式 Successful 后不回滚 | §0.7, §7.7, §7.19 |
| E3 | 2-of-3 Governance Safe → 48h Governance/Upgrade Timelock；独立 24h Unpause Timelock；独立 2-of-3 Pause Guardian | §0.7, §7.17, §7.20 |
| E4 | Token policy/runtime/attestation 治理登记并按项目链上快照 | §0.7, §7.20-§7.22 |
| E5 | “永久”限定为 V1 无截止、无 sweep，受 Token/钱包/链/Pause/升级约束 | §0.7, §7.9 |
| E6 | 用户地址不迁移；项目方和平台 receiver 为不可变、已审核 2-of-3 Safe | §0.7, §7.11, §7.20 |
| E7 | Claim Pause 单次最多 7 天，自动到期，24h Timelock 可提前解除 | §0.7, §7.13, §7.20 |

### 14.3 Nonce 裁决

Backend reviewer 建议将 `next_nonce` 定义为“下一个未 finalized 消耗的 nonce”，并在 final revert/applied 时推进。主会话对现有 `pkg/common/chaintx/outbox.go` 与 migration 的代码事实检查表明，当前实现采用另一套自洽且更保守的语义：

```text
next_nonce = 下一个尚未被本平台签名承诺的 nonce

Gateway exact raw_tx crash-safe durable
→ Job 原子保存 raw_tx/tx_hash/signed active slot
→ next_nonce = nonce + 1

final_success 保持 fence
applied/final_revert 只释放 fence，不再次推进 nonce
```

该语义防止已签但广播不明的 nonce 被新业务交易复用，也正确处理 mined revert 已消耗 nonce。第二轮 reviewer 关于“finalization 才推进”的具体建议不采纳；其 P0 本质要求——**精确、唯一、crash-safe、可测试的 nonce 生命周期**——通过权威设计 §10.2-§10.4 满足。

## 15. 第一轮发现状态矩阵

| 第一轮发现族 | 当前状态 | 处理 |
|---|---|---|
| 关联钱包/Sybil/鲸鱼/无 Hard Cap | ACCEPTED RESIDUAL | 限定 Fair 含义、尽调/监控、不宣称预防、每次签名前披露 |
| 恶意/可变 Token | RESOLVED BY DESIGN，待审计/实现 | Token attestation、项目快照、runtime mismatch Failed 路径、Release gate |
| cancel/end 竞态 | RESOLVED BY DESIGN | 到期后 cancel revert，permissionless end exact tx convergence |
| Claim/Refund deadline/sweep | RESOLVED BY DECISION | V1 无 deadline/sweep，claimFor/refundFor，限定永久含义 |
| UUPS 单点和 Pause | RESOLVED BY DESIGN，待部署验证 | 两 Timelock + 两 Safe 拓扑，无紧急升级绕过，Claim pause 自动到期 |
| 整数最终价格/0 display | RESOLVED | numerator/denominator 是权威；展示精度不作为 Solidity 准入条件 |
| reorg/source/event identity | 第三轮再次发现并已整改，待 freeze re-check | tx-local ordinal、Source revision/cursor、identity/placement、staged migration、append-only reversal |
| outbox/signer | PARTIAL → 规范已补齐，待 POC | 原子 nonce 语义、Gateway journal、边界 fault injection |
| Frontend BigInt/路由/交易 | PARTIAL → 规范已补齐，待实现 | typed API、fresh preflight、persistent intent、strict Gas、action confirmation |
| Admin Preview/四眼 | PARTIAL → 规范已补齐，待实现 | receiver/attestation digest、三人两两不同、step-up MFA、全治理 Preview |
| Recovery/rollback | PARTIAL → 规范已补齐，待演练 | signed manifest Portal、feature watermark、capability startup gate、write-disabled restore |

## 16. 当前正式 Gate 与后续审计入口

```text
Gate A 产品决策       FROZEN
Gate A 法务签署       PENDING
Gate B 安全决策       FROZEN；E1-E8 已冻结；freeze re-check见 §18
Gate C 可靠性决策     FROZEN；等待 signer POC 和故障/灾备演练
Gate D 财务 UX 决策   FROZEN；等待实现、WCAG 和 Recovery Portal 演练
第二轮审计            COMPLETE / NO-GO
第三轮审计            COMPLETE / NO-GO
候选技术 Freeze       CONDITIONAL GO，待 §18.4 机械产物
总体生产              NO-GO
```

以下为第三轮入口清单的历史记录；本轮已执行并得到 §17 的 NO-GO，未通过项转入 freeze re-check：

1. §7.20-§7.23 exact ABI、事件、errors、storage namespace/gap、transfer reserve；
2. Release gate、runtime mismatch 和治理拓扑无旁路；
3. §8.5 Source revision/cursor、event identity migration 和 placement/reorg；
4. §10.2-§10.4 nonce 与 signing journal crash protocol；
5. permissionless end 在全部 submission 状态下收敛；
6. typed API、project receiver、exact approve、严格 Gas和 action-specific confirmation；
7. signed Recovery Portal manifest、三语言 disclosure parity 和 WCAG 2.2 AA；
8. maintenance watermark/image capability/DB restore。

历史入口已由 §18 执行。当前仍须先完成 §18.4 机械产物；Gate A 法务和 Sign Policy Gateway POC依然独立阻断生产资金实现与主网部署。

---

## 17. 第三轮四通道审计结论

### 17.1 正式结论

```text
第三轮审计：COMPLETE
技术设计 freeze：NO-GO
总体生产状态：NO-GO
```

四通道一致确认 E1-E7 已显著改善架构，但审计时仍存在基础可实施性缺口。以下为交叉去重后的权威清单；其后 Owner 已冻结 E8-A，见 §17.6。

### 17.2 P0 阻断

1. **远程签名同 nonce 双签窗口**：Gateway commit 在 Job sender fence 之前时，另一个 submission 可使用相同 nonce。整改为 Job 先 durable `signing_reserved + active_submission_id`，Gateway 再签；Gateway 同时唯一约束 `(chain_id,signer,nonce)`。`signing_reserved` 取消前必须权威确认 journal 无 raw tx。
2. **事件 identity 的 `logIndex` 不稳定**：RPC `logIndex` 是块内全局序号，re-inclusion 后可能变化。稳定 identity 改为 `(chain_id,tx_hash,tx_log_ordinal)`；RPC logIndex、block、tx index 和 Source revision 属于 placement generation。
3. **支付 Token runtime mismatch 终态未冻结（审计时 P0，现已按 E8-A 关闭设计决策）**：项目 Token 的 E2 路径不能自动套用于稳定币，因为转为 Failed 会改变 payment reserve 的受益人。Owner 已在第三轮后选择推荐方案 A：Pending 禁止激活，Active 禁止新增贡献但不自动改变既有归属，到期可正常 end 至 Pending Release，支付 Token mismatch 阻止 Release；仅 48h Governance Timelock 可裁决 Failed/Refund，或等待项目 exact snapshot 恢复并重新对账后 Release；Successful 永不回滚，Claim 不受影响，支付币转出 fail closed。权威规则见设计 §0.8、§7.19。

### 17.3 已直接整改的 P1

- Release 不受 `payoutsPaused` 阻止，避免绕过 E7 形成无限 Claim 暂停；提款仍受 Payout Pause；
- Release attestation 明确 Operator/Gateway 信任边界、20-block 公式、settlement blockhash、canonical manifest和 policy digest；不再声称 Solidity 自算 DB BALANCED；这是 E1 的工程化边界澄清，不新增独立 Owner 信任主体；
- 生命周期 DB/API 增加 `chain_status/terminal_reason/settlement/release` identity；派生 ended-active 与链上 pending-release 分离；
- 事件/action/caller/fence 表扩展至 deposit、invalidation、proceeds、failed Token、pause/unpause、config/policy/beneficiary/release policy和 inherited governance events；
- ABI 增加 namespaced global/config/reserve getters；删除未定义 `claimsAvailableAt`；
- beneficiary Safe attestation digest/version 进入 create calldata、Project storage 和 Created event；
- Release ABI 采用 settlement block/hash、tx hash、tx-local ordinal、manifest digest；
- Replacement 使用 append-only signing generation，不覆盖历史 raw tx/hash；
- Source revision 使用 block/tx/intra-tx 有序边界，revision 绑定 placement/application；
- 新 Source historical backfill 使用 coverage cursor，不倒退 live chain cursor；
- Recovery Portal 固定 RFC 8785 JCS + Portal-pinned 2-of-3 Ed25519 trust root、generation/expiry/anti-rollback/rotation；
- Gas fallback 使用签名 `wallet-gas-policy.v1`，按 chain/version/runtime/selector 键控；
- 项目 Token approve/deposit 使用 exact target、zero-first 和专属确认；
- Position API 改为 typed financial discriminated response和 provenance；
- Feature watermark 由链上事实 + 签名 Release Control Plane 支撑，不依赖可回滚 DB；
- 部署顺序改为先 Timelock，后原子初始化 Proxy；
- E5 文案规范化为“V1 无协议截止且无 sweep”，不作绝对永久保证。

### 17.4 仍需进一步工程规范/短复审

1. **已补齐，待 freeze re-check**：Event Identity CE-V1 → CE-V2 逐表 cutover matrix，包含 writer/reader generation、receipt backfill provenance、quarantine、验证 query、最后回滚边界和 FK/index/trigger 退役顺序；
2. **已补齐，待 freeze re-check**：Safe proposal、Timelock operation、automated executor EOA 的独立 durable schema、状态转换、permissionless execution、nonce conflict和 reorg收敛；
3. **已补齐，待 freeze re-check/工具实现**：RFC 8785 + 2-of-3 Ed25519 signed image capability manifest、Release Control Plane/host/chain 三源单调 watermark、key rotation和启动前门禁；
4. **已补齐，待 freeze re-check**：Payment Token E8-A 已同步 ABI terminal reason、event、DB/API、reserve重分类和测试；必须验证无 permissionless payment-token fail 旁路；
5. 下一步执行范围受限的短周期 freeze re-check，而不是将原第三轮结果改写为通过。

### 17.5 独立证据 Gate

以下不是本轮设计缺陷，但继续独立阻止生产：

- Gate A 正式法务书面意见；
- Sign Policy Gateway + Web3Signer + AWS KMS POC 和 fault injection；
- 实现、合约 fuzz/invariant/storage validation；
- reorg、restore、governance、Recovery Portal 演练；
- 主网前独立合约/系统安全审计。

当前不得冻结生产 ABI/event/storage，不得开始真实资金实现或部署主网 Proxy。

### 17.6 第三轮后 Owner 决策 E8

Owner 已选择推荐方案 **E8-A**，支付稳定币 runtime/policy mismatch 的权威规则冻结为：

```text
Pending:
  mismatch / emergency blocked → 禁止 activate，可按 Pending 取消

Active && now < endTime:
  禁止新 Contribution
  不自动改变既有支付币归属
  可走既有 Operator 紧急取消

Expired Active:
  payment Token mismatch 不允许 permissionless 自动 Failed
  endProject 仍按 Soft Cap + project Token 客观状态结算
  达 Soft Cap且 project Token正常 → SuccessfulPendingRelease

SuccessfulPendingRelease:
  payment Token mismatch / emergency blocked → 阻止 Release并保持状态
  仅 48h Governance Timelock 可：
    1. invalidate → Failed(PaymentTokenGovernanceInvalidation) / Refund / fee=0
    2. 等待 exact 项目快照恢复，重新 finalized reconciliation 后 Release

Successful:
  永不回滚 Failed
  project Token Claim 不受 payment Token mismatch 影响
  proceeds / fee transfer fail closed

Failed / Cancelled:
  Refund 入口不因 policy mismatch 被关闭
  Token 实际转账失败则原子 revert，reserve/ledger/position 不变，可恢复后重试
```

禁止用新全局 payment Token policy revision 覆盖旧项目快照；禁止 Operator、Admin、项目方或 permissionless caller自行决定支付币资金归属。该决策已同步至设计 §0.8、§7.19、候选 ABI/error/event、DB/API 和测试矩阵。

## 18. 范围受限 Freeze Re-check 结论

### 18.1 执行状态和裁决边界

```text
Workflow                    cb9bf083-40df-4294-bfcc-ab627845852d
Mission                     95e1d833-1a9c-4a6e-b992-57ade75b39ae
Contract + E8-A lane        COMPLETE
CE-V2 + Governance lane     COMPLETE
Release + Watermark lane    COMPLETE
Review-time freeze verdict  NO-GO
Post-correction spec status FROZEN / CONDITIONAL GO for candidate technical freeze
Mechanical §18.4 gate       PASS (items 1-6)
Overall production status   NO-GO
```

本轮只复查第三轮后的工程规范，不重开 Gate A-D 或 E1-E8。三个 reviewer 在审阅时一致认为候选技术 freeze 仍有确定性规范缺口，因此原始 review verdict 是 `NO-GO`。这些缺口均属于实现规则/数据模型闭合问题，不需要新的 Owner 产品决策；主会话已按最小范围回写设计文档。下列六项机械验证现已全部通过，候选技术设计进入 **FROZEN / CONDITIONAL GO**；该结论不得被表述为生产、法务、签名基础设施或主网 readiness。

### 18.2 已确认正确且未重开的冻结规则

- E8-A 生命周期、唯一 48h Timelock资金归属裁决、Successful 不回滚和 Refund reserve语义保持不变；
- Release/Pause/固定 beneficiary Safe、三权分离和 Safe/Timelock/executor nonce domain保持不变；
- event identity继续使用 `(chain_id,tx_hash,tx_log_ordinal)`；
- CE-V1→CE-V2 的 M0-M8 staged cutover和 M6 后禁止逐请求 fallback保持不变；
- signed capability manifest、host/Control Plane/chain monotonic watermark方向保持不变。

### 18.3 本轮发现和规范整改

| 领域 | Review-time 阻断 | 当前规范整改 |
|---|---|---|
| Payment Token policy | 每个 token 只有一个 mutable config，无法恢复旧项目 exact policy snapshot | 改为 append-only `(token,policyVersion)` revision registry + 当前 admission pointer + 独立 emergency block；旧 revision不可覆盖/删除 |
| Release evidence | 依赖未来执行时 `blockhash(settlementBlockNumber)`，存在 256-block窗口 | `endProject` 永久保存 parent-block anchor number/hash；Release绑定 storage和 Operator canonical receipt/parent attestation |
| E8 selector isolation | 通用 invalidation selector仍可能编码 payment-token evidence | 增加非支付币 `GovernanceInvalidationClass`；Gateway/Admin/Timelock拒绝 payment evidence走通用 selector |
| Event identity | identity row含 address/payload digest，混入 placement-dependent observation | identity只保留三元组；每个 placement generation追加 observation；payload conflict建 incident并 fail closed |
| Application history | 单行 application/upsert不支持 replay/reversal lineage | append-only application attempt，含 effect version、attempt number和 predecessor/replay lineage |
| CE-V2 ledger | legacy unique constraint与多 entry ordinal冲突 | M1建立独立 `fund_ledger_v2_shadow`；M3-M5写 shadow，M6原子切换；entry ordinal来自冻结 effect manifest |
| Source revision | 同交易升级边界含糊 | 半开 range `(block,tx,first_tx_log_ordinal)`；`Upgraded`用固定 inherited ABI解码，新 revision从其后第一个 Proxy log生效 |
| Outbox cutover | final event旧/新 FK和 fence释放条件不够精确 | M0-M5双写并验证 identity/effect；M6+只认 canonical/finalized expected V2 application attempt |
| Safe workflow | 唯一 nonce定义无法保存外部冲突，且平台广播边界含糊 | 独立 Safe nonce slot保存全部候选并冻结 conflict；平台不代签/广播 Safe exec，仅观察外部 execution tx |
| Timelock batch | schedule/execute聚合和 ready time不完整 | 保存 expected call count/index payload；canonical block timestamp计算 ready；全部 index + on-chain state后推进 |
| Release bootstrap | host state丢失后链 evidence发现循环依赖 DB registry | 增加独立签名 environment bootstrap，固定 chain/proxy/factory、discovery block、Control Plane和 registry signer roots |
| Artifact hashing | manifest可能在自身 artifact内，形成 hash自引用 | capability manifest改为 Release record detached sidecar/envelope；先验签，再以其 hash验证 artifact |
| Watermark join | `max`对 schema/event model/capability集合语义含糊 | 定义 typed monotonic join：集合 union、有序 enum/schema max、identity exact-match/conflict fail closed |
| Key/host lifecycle | rotation、revocation freshness和新主机 enrollment不完整 | 定义 key-set version范围、2-of-3 rotation/revocation、24h freshness、新主机enrollment和host-state-loss re-enrollment |
| Startup/rollback | Agent和应用门禁职责、rollback顺序不够可执行 | 分离 Agent pre-apply与 application pre-listener；旧 target eligibility在启动旧容器前验证 |
| Restore | restore成功与产品 watermark generation混用 | 新增独立 restore recovery attestation；重复restore不提高产品最低 generation |

### 18.4 CONDITIONAL GO 条件（已完成）

以下条件已全部满足，**候选技术设计**正式标记为 `FROZEN / CONDITIONAL GO`：

1. 从同一 Solidity compile artifact生成 ABI selector、event topic、custom error selector和storage-layout基线；
2. 运行 storage-layout diff，确认新增 versioned policy registry与 settlement anchor字段的 exact order/slot；
3. 生成 CE-V2 DDL/约束草案并验证：identity/placement observation/application attempt、payload conflict、shadow ledger、M5/M6 fence和 final-event FK；
4. 生成 Safe/Timelock/executor schema和状态机测试向量，包括 same-nonce conflict、batch缺 index、external execution和 reorg；
5. 生成 RFC 8785 manifest/bootstrap/key-set/watermark/recovery-attestation JSON schemas及 golden vectors，验证 detached artifact hash无自引用；
6. 再执行一次仅检查上述机械产物的短周期 freeze verification，不重开 Owner决策。 **PASS**。最终绑定验证记录：`D:/pi-tmp/fair-launch-current-final-verification.log`；`MECHANICAL_RC=0`、`DIFF_CHECK_RC=0`、`FINAL_VERIFICATION_RC=0`，并记录 CE schema/vector/verifier/test 输入 SHA-256。

独立关闭复核结论：Contract items 1-2、CE-V2/Governance items 3-4、Release item 5及最终 item 6全部 `PASS`。机械 specimen仍不是生产资金实现；后续已从冻结设计生成显式 deployment-gated 的可执行 migration，但未应用、未连接任何数据库。本节结论只冻结候选技术 ABI/storage/CE/release规范，不声明部署或运行 readiness。

### 18.5 仍独立阻断部署/生产的 Gate

> Owner 2026-09-07 决定：以下条目是 migration apply、部署、runtime enable、签名/广播和真实资金操作的门禁，不是代码开发门禁。生产候选代码、gated migration、离线部署脚手架、CI和 Recovery Portal可以先完成；本节通过前总体部署/生产仍为 **NO-GO**。

即使 §18.4 全部完成，以下仍使总体状态保持 **NO-GO**：

- Gate A 法务书面意见；
- Sign Policy Gateway + Web3Signer + AWS KMS POC、threat model、crash/duplicate-sign/recovery/key-rotation演练；
- capability/watermark/enrollment/restore机制的真实实现和灾备演练；
- CE-V2 migration代码已生成；仍缺 quarantine为零证据、disposable/生产 cutover演练和正式 apply批准；
- 合约实现后的 unit/fuzz/invariant/storage/upgrade测试及主网前独立安全审计；
- WCAG 2.2 AA、Recovery Portal和完整财务 UX验收。

因此本轮正式表述为：**Review-time NO-GO；整改和独立机械关闭复核后 candidate technical freeze = FROZEN / CONDITIONAL GO；overall production = NO-GO。**
