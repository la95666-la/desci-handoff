# JoinCare IDO V3 地址权限与操作范围

## 1. 适用范围与证据

本文区分 **合约直接授权地址、Safe 签名人、项目方、投资人和平台服务**，用于权限交接与操作审阅，不是自动执行脚本或生产写入授权。

| 项目 | 核验值 |
|---|---|
| 网络 | BNB Smart Chain 主网，chainId **56** |
| V3 Proxy（业务调用目标） | `0x0D4040875742975269c55E16C8884d3Ee9510B8E` |
| 当前 Implementation | `0x887fCC6E54Fc8229919166Ad90B16C19B4992560` |
| 地址快照区块 | **131178189** |
| 快照时间 | **2026-11-02 12:37:31 UTC+8**（04:37:31 UTC） |
| 语义依据 | `joincare_code/joincare_code/_extracted/JoinCare_SmartContract-main/contracts/JoinCareIDOv3.sol` |
| 源码仓库提交 | `3ca73317b5cbefba65402d235aaa3222f536d1e0` |

地址、费率、Safe 签名成员与门槛为上述时间点的链上读取结果。前一快照（区块 131177551）的角色、Safe 成员及门槛已由两个独立 RPC 交叉核验，本次再次读取确认。函数权限依据源码解释；这不是对源码与部署字节码的新增完整审计证明。实施操作前必须重新核对链上配置。合约升级后应重新审阅本文全部语义。

## 2. 当前全局地址

| 角色 | 当前地址 | 性质与边界 |
|---|---|---|
| Owner | `0x79c527B8Af5b1E27d5052f8dCe04D20733e72350` | Owner Safe，最高治理角色，可授权 UUPS 升级 |
| Whitelist Governor | `0xc46659e67543AAAeC553F7f3FC8F004B7Dd286EF` | 独立的白名单治理 Safe |
| Operator | `0xfd712061ee08E070A930eCD45E935Ae5d7f5445C` | EOA，负责受限业务执行；不是治理 Safe，也不是投资人资金中转地址 |
| 全局 Fee Receiver | `0xa94bb1563311B86fc80cb3c063514de8Cc5a87a5` | EOA；收款身份本身不赋予治理权限 |
| Pending Owner | `0x0000000000000000000000000000000000000000` | 当前无待接受的 Owner 移交 |

当前全局 `successFeeRate=1000 bps`，即 **10%**。这不是所有既有项目的费率：每个项目使用创建时固化的 `successFeeRate` 和 `feeReceiver` 快照。

### 2.1 两个 Safe 的签名成员

两个 Safe 当前均为 **2 / 3**，成员相同：

1. `0xc7b4bf323C94CEb2E6e9bb7fbf89D06140941e5f`
2. `0x35D3F22ab317569EA15728e59bC0EEeff521b687`
3. `0x0C47E90BA6225c53a17650272f7bF4dC3dE805B3`

| Safe | VERSION | 已启用 Modules |
|---|---|---|
| Owner Safe | 1.4.1 | 无 |
| Whitelist Governor Safe | 1.5.0 | 无 |

**权限由 Safe 地址持有，不是由签名人单独持有。** 单个成员直接调用 V3 的 Owner/Governor 专属函数，不会因其 Safe 成员身份而获准。需通过相应 Safe 满足门槛后执行。

两个 Safe 虽为不同地址且权限分离，但使用同一组签名人，**不能描述为人员或密钥保管独立**。`modules=[]` 只说明本次模块列表为空，不等于已完成 Guard、Fallback Handler、部署可信性和密钥保管的全面审计。

## 3. Owner Safe：治理权限

| 操作 | 函数 | 条件与影响范围 |
|---|---|---|
| 升级实现 | `upgradeToAndCall(...)`（UUPS 继承入口） | `_authorizeUpgrade` 限制为 Owner，且 Owner 必须是合约地址；可改变整个 Proxy 的执行逻辑，最高风险 |
| 发起 Owner 移交 | `transferOwnership(newOwner)` | 仅 Owner；新地址必须有合约代码，且不能是 Proxy 本身 |
| 接受 Owner 移交 | `acceptOwnership()`（继承入口） | 仅当前 `pendingOwner`；两步移交未接受前，原 Owner 继续持有权限 |
| 放弃 Owner | `renounceOwnership()` | **禁止**，始终 revert |
| 全局暂停/恢复 | `pause()` / `unpause()` | 作用于创建、充值、激活、认购；不冻结退款、领取和既定提取入口 |
| 设置全局费率/收款方 | `setPlatformFees(rate, receiver)` | 影响以后创建的项目；不覆盖既有项目快照，费率不得超过合约上限 |
| 更换业务执行者 | `setOperator(newOperator)` | 新地址非零且不能为 Proxy；需同步应用身份和在途交易核验 |
| 支付币准入/禁用 | `configurePaymentToken(token, allowed)` | 启用时读取 decimals，已登记精度不得漂移；禁用会阻止依赖该币的新认购 |
| 项目币准入/禁用 | `configureProjectToken(token, allowed)` | 可影响创建、充值、激活、认购；退出函数没有同样的准入开关限制 |
| 创建/激活 | `createProject(...)` / `activateProject(id)` | Owner 也可执行 Operator 的业务入口，但不能绕过合约参数和状态限制 |
| 取消项目 | `cancelProject(id)` | Pending 或 Active；Active 阶段仅 Owner 可取消；终态不可取消 |
| 批量退款 | `batchRefund(id, users)` | 项目 Failed/Cancelled；按用户既有认购退款，直接转给原用户，不转给调用人 |
| 回收多余币 | `recoverExcessTokens(token, recipient, amount)` | 只能回收实际余额超过 `reservedBalances[token]` 的部分；不能直接侵占被预留的用户/项目负债 |

**重要边界：**上述限制描述当前实现。Owner 的升级权限意味着它可通过新实现改变规则，因此不能把“当前实现无任意提款函数”解释为“治理方在任何升级下都无法影响资金”。

## 4. Whitelist Governor Safe：白名单权限

唯一专属业务写入口：

```solidity
setWhitelistPolicy(projectId, expectedVersion, enabled, root)
```

- 调用者必须精确等于 `whitelistGovernor`，Owner、Operator、Admin JWT 本身不拥有该权限。
- Pending：可按规则启用、关闭或替换策略。
- Active：**仅原本已启用白名单的项目，可保持启用并替换 Root**；不能启用原本公开的项目，也不能关闭原本启用的白名单。
- Successful / Failed / Cancelled：不可再修改。
- `expectedVersion` 必须匹配当前版本，每次成功修改版本加一；enabled 与零/非零 Root 必须匹配，禁止无变化写入。
- Proof 叶子绑定 chainId、Proxy、projectId、buyer，不能跨链/跨项目复用。

当前实现 **没有 `setWhitelistGovernor`**。Governor 在初始化时设置；普通 Owner 调用不能直接替换。更换签名人可在 Governor Safe 内按其规则执行，不改变 Governor Safe 地址；改变 V3 所记录的 Governor 地址需要另行审阅实现升级方案，不能伪装成普通配置修改。

## 5. Operator：日常业务执行

当前实现仅在以下两处调用 `_requireOperatorOrOwner()`：

| 函数 | Operator 可操作范围 |
|---|---|
| `createProject(...)` | 创建尚不存在的项目，明确指定项目方、两类 Token、caps、价格、时间窗及 vesting；费率/收款快照必须匹配当前全局值，代币需已准入，参数须通过验证 |
| `activateProject(id)` | Pending → Active；必须满足项目币准入/精度、白名单配置一致及储备条件 |

**充值阈值需分清两层：**当前合约激活检查的是 **Soft Cap 所需项目币**；官方页面 Owner 充值策略按 **Hard Cap** 备足。这不是相同限制。认购时合约仍逐笔检查实际可用项目币，储备不足会 revert。

Operator 不能因其身份升级、暂停、修改费率/收款人、改 Token 准入、改白名单或提取他人净募集款。也不能在 Active 项目上取消项目。

Operator 并非低风险“只读 Job 地址”：创建时能指定资金相关项目身份和条款，必须遵循应用的 Set 审批、固定参数和 durable outbox 流程。不能把合约允许的直接调用范围等同于平台业务授权范围。

### 5.1 平台如何使用它

Admin 操作 → 后端权限/Set/链上身份校验 → 持久化交易 intent/outbox → 受控签名与广播 → 原始 receipt 与事件对账 → Job 发布投影。

- 私钥不应放在 Admin 浏览器，Admin 登录态不是链上角色。
- Job 索引器读取区块、receipt、日志，不因“负责同步”就取得 Owner 权限。
- 应用也可使用执行地址触发到期结算/平台费提取，但这两项合约入口不是 Operator 专属，详见第 8 节。
- 绝不用 relayer/Operator 私钥手工交易绕过 outbox 与 nonce fence；已有签名交易应按原 hash/nonce 对账，不能因超时自动另签。

## 6. 项目方 `project.projectOwner`

每个项目各自固化，没有单一全局项目方地址，需通过 `getProject(projectId)` 查询。

| 函数 | 条件 | 资金去向 |
|---|---|---|
| `depositProjectTokens(id, amount)` | 仅该项目方，Pending，未全局暂停，项目币已准入且精度匹配；先授权 Proxy，到账必须精确 | 项目方 → Proxy 的项目币储备 |
| `cancelProject(id)` | 项目方仅可取消 Pending；Active 需 Owner Safe | 只改生命周期，不自动转款；后续按退款/提币处理 |
| `withdrawRaisedFunds(id)` | 仅该项目方，Successful，未提取 | 净募集款 → **固定项目方**；不能传入替代收款人 |
| `withdrawProjectTokens(id)` | 仅该项目方，Successful/Failed/Cancelled，未提取 | Successful 退未售项目币；Failed/Cancelled 退全部充值项目币，均给固定项目方 |

净募集款 = `totalRaised - floor(totalRaised × 项目费率 / 10000)`。项目方提净款不以平台费已提取为前置条件；项目币提取与净款提取也是分别记账。

当前实现无已创建项目的 Owner/Token/价格/caps/时间/vesting 任意编辑入口；不能通过改数据库伪造链上变更。

## 7. 投资人/用户钱包

| 函数 | 条件与限制 | 资金去向 |
|---|---|---|
| `purchase(id, amount)` | 白名单关闭、Active、未暂停、在开始/结束时间窗内、两类 Token 准入及精度正常、余额/Allowance 足够、单钱包累计不超 `maxPerUser`、不超 hardCap、项目币储备充足 | 用户支付币 → Proxy；项目币权益记录给 `msg.sender` |
| `purchaseWithProof(id, amount, version, proof)` | 白名单启用，版本/Root/当前钱包 Proof 匹配；其余认购规则同上 | 同上 |
| `claim(id)` | Successful，按绝对 vesting 时间累计解锁，扣除已领取量，存在可领金额 | 项目币 → 调用用户自己 |
| `refund(id)` | Failed/Cancelled，有未退认购；单次聚合退还该用户全部应退款 | 支付币 → 调用用户自己 |

用户需自行准备 BSC 主网 gas（BNB）。代币 `approve` 调用目标是 **ERC-20 Token 合约**，spender 为 **V3 Proxy**，不是 Safe、Operator 或 Implementation。

官方页面最低购买金额是**单笔页面策略**，不是合约额外最低购买限制；合约对 `maxPerUser` 的限制是**累计钱包额度**。展示三位小数不改变 raw/链上计算精度。

## 8. 无专属角色限制，但资金接收方固定的入口

| 函数 | 任何地址可触发的条件 | 不能做到的事 |
|---|---|---|
| `endProject(id)` | Active 且链上时间达到 endTime；按募集额是否达到 softCap，落成 Successful/Failed | 不能提前结算，不能任意选择成功/失败，调用人不获得资金 |
| `withdrawPlatformFee(id)` | Successful、平台费未提取；金额由项目固化快照计算 | 只能付给该项目的 `feeReceiver`，不能付给触发者或当前新配置地址 |

认购达到 Hard Cap 的最后一笔还会在合约内自动 finalize。外部调用 `endProject` 不是所有成功结算的唯一触发方式。

“任何地址可触发”不代表“任何地址可取走资金”。Operator/Owner/用户都可能作为触发者，但收款规则不随触发者变化。

## 9. 暂停、收款与治理风险速查

### 9.1 `pause()` 的精确影响

- 阻止：`createProject`、`depositProjectTokens`、`activateProject`、`purchase`、`purchaseWithProof`。
- 不因该暂停标记而阻止：`endProject`、`cancelProject`、`refund`、`batchRefund`、`claim`、`withdrawRaisedFunds`、`withdrawPlatformFee`、`withdrawProjectTokens`。
- Owner 治理配置、Governor 策略和超额 Token 回收也不带 `whenNotPaused`；各自仍受权限和参数检查。
- 不带 `whenNotPaused` 不等于必定成功：Token 本身暂停、黑名单、精度/转账行为异常或储备不足等依然可能导致退出失败。

### 9.2 Fee Receiver 不是管理者

全局 `feeReceiver()` 影响新项目；既有项目实际支付使用 `getProject(id).feeReceiver`。签名前必须明确**项目快照的实际收款人和费率**，不能只查看全局配置。

`withdrawPlatformFee` 与 `withdrawRaisedFunds` 分别有一次性标志，不会因触发者变化重复支付。`recoverExcessTokens` 可指定接收人，但受全 Token 聚合储备余额约束，不能把正常 escrow 解释为“多余币”。

### 9.3 Safe 的边界

- Safe 签名人/门槛调整属于 Safe 自身管理，不通过 V3 的 `setOperator`。
- 当前 Owner 接口只检查目标有代码，**不会自行证明新 Owner 是正确 Safe 或满足 2/3**；这属于实施前的治理核验责任。
- Owner Safe 可更换 Operator，但不能据此修改 Governor Safe 的签名成员。
- 当前实现将白名单治理独立给 Governor；但 Owner 升级实现的最终风险仍需纳入信任模型。

## 10. 只读查询与操作前清单

任何人可读取 `owner()`、`pendingOwner()`、`operator()`、`whitelistGovernor()`、`feeReceiver()`、`successFeeRate()`、`paused()`、`getProject(id)`、`getWhitelistPolicy(id)`、`getUserPurchase(id,user)`、`calculateClaimable(id,user)`、`getProjectParticipants(id)` 及公开 Token 配置/储备 getter。只读权限不表示具备交易权限。

每次重要操作前核对：

1. chainId **56**、目标 **Proxy**、当前 Implementation 与方法 ABI；不能向 Implementation 直接发送业务调用。
2. Safe 地址/签名成员/threshold、执行目标、calldata、value、nonce；代币金额与原始精度不得用浮点数作为权威。
3. 项目实际生命周期、不可变参数、Token、收款快照、费率、时间窗和足额储备。
4. 既有 outbox hash/nonce、同项目在途尝试、模拟结果与 gas；未知广播结果不得自动重发。
5. 权限移交采用两步接受，部署/参数变更/地址移交分别授权；单次 RPC 或恢复授权不覆盖治理写入。
6. 上链后以独立 RPC 原始 receipt、canonical 区块及已确认事件验收；Admin 成功提示、容器健康、数据库手工状态均不是单独的链上成功证明。

本文未执行任何权限、资金或配置变更，也不包含私钥、RPC 凭据、登录令牌或签名交易。
