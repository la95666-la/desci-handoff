# JoinCare 生产合约权限移交操作说明

> 文档状态：生产交割操作规范；本文档本身不构成任何链上写操作授权。  
> 适用网络：BNB Smart Chain 主网（`chainId=56`），其他网络必须重新生成并审核全部地址、交易和证据。  
> 权限接收方：JoinCare 系统委托方/产品所有方，以下简称“产品所有方”。  
> 适用组件：`JoinCareIDOv2`、`JoinCareFairLaunchV1`、Fair Launch Governance/Unpause Timelock、Safe、Operator、Treasury、Platform Fee Receiver、`JC` Token、`JoinCareCheckIn`。  
> 核心目标：交割完成后，产品所有方拥有全部平台级最终控制权；开发方、部署方、测试钱包和临时 Relayer 不保留任何生产权限。

---

## 1. 文档目的

本说明用于指导 JoinCare 正式生产部署或既有部署迁移时，将全部平台级链上权限安全移交给产品所有方，并形成可独立复核的链上证据。

移交完成后的目标状态：

```text
产品所有方
  ├─ 控制 Governance Safe
  ├─ 控制 Governance/Upgrade Timelock
  ├─ 控制 Emergency Safe
  ├─ 控制 Unpause Timelock
  ├─ 控制 Treasury Safe
  ├─ 控制 Platform Fee Safe
  ├─ 控制生产 Operator/KMS/MPC Signer
  ├─ 控制 JC Token 和 CheckIn 治理
  └─ 控制生产发布、域名、云账号和密钥管理系统

开发方/部署方
  ├─ 不是任何生产 Safe Owner
  ├─ 没有 Timelock proposer/canceller/executor/admin 角色
  ├─ 不是任何生产合约 owner 或 pendingOwner
  ├─ 不持有生产 Operator/Relayer/Executor 密钥
  ├─ 不是 Treasury、平台费或 recovery 受益人
  └─ 仅在产品所有方另行书面授权时参与维护
```

本说明不改变募资项目资金归属：

- JoinCare 平台费进入产品所有方的 Platform Fee Safe；
- 募资项目净收益进入对应发行项目的 Project Proceeds Safe；
- 用户退款和 Token Claim 仍按合约固定受益人执行；
- 平台所有权不代表可以绕过项目快照或储备会计提取项目资金。

---

## 2. 强制安全原则

### 2.1 不得将全部权限集中到单个 EOA

产品所有方可以拥有全部最终控制权，但必须采用分域治理：

| 权限域 | 推荐承载方式 |
|---|---|
| 非紧急治理、升级、Token policy | Governance Safe → Governance Timelock |
| 紧急暂停 | 独立 Emergency Safe |
| 恢复业务 | 独立 Unpause Timelock |
| 日常生命周期交易 | 独立 Operator KMS/MPC/Signer |
| Treasury | Treasury Safe |
| 平台费 | Platform Fee Safe |
| 募资项目净收益 | 对应项目 Project Proceeds Safe |

禁止将 Governance、Emergency、Operator、Treasury 和平台费全部放入同一个普通钱包。

### 2.2 不读取或传递钱包秘密

操作过程中不得在文档、聊天、Git、日志、截图或发布包中出现：

- 私钥、助记词、密码；
- 原始签名、raw transaction；
- RPC API key；
- KMS/MPC 恢复材料；
- Safe Owner 的秘密凭据。

所有钱包签名由产品所有方在钱包或 Safe 官方界面逐笔确认。

### 2.3 写操作逐阶段授权

以下每一类动作必须有单独授权，前一动作的授权不得推导为后一动作已获授权：

1. 创建或调整 Safe；
2. Timelock `schedule`；
3. Timelock `execute`；
4. `transferOwnership`；
5. `acceptOwnership`；
6. Operator、Treasury、Fee Receiver 或 Guardian 变更；
7. UUPS 升级；
8. 生产服务启用。

### 2.4 不得使用生产 Operator/Relayer 私钥手工治理

Operator 仅承担已批准的日常生命周期操作。不得使用 Operator 或 Relayer 私钥执行 Owner、Safe、Timelock、升级、Token policy 或权限移交交易，以免破坏 nonce 对账和签名域隔离。

### 2.5 链上操作不可撤销

所谓“回滚”只能是再次执行一组新的链上治理交易，不是数据库回滚。必须坚持：

```text
先授予新权限 → 验证新权限可用 → 再撤销旧权限 → 最终负向验证
```

不得先撤销旧控制方再尝试建立新控制方。

---

## 3. 合约和权限模型

### 3.1 JoinCareIDOv2

`JoinCareIDOv2` 是 UUPS Proxy，未使用独立 `ProxyAdmin`。Proxy 上的 `owner()` 同时控制：

- UUPS `upgradeToAndCall`；
- `pause()` / `unpause()`；
- `setOperator()`；
- `setPlatformFees()`；
- `configurePaymentToken()`；
- `configureProjectToken()`；
- Owner 参与的项目创建、取消和批量退款；
- `recoverExcessTokens()`。

该合约使用 `Ownable2StepUpgradeable`：

```text
当前 Owner 调用 transferOwnership(newOwner)
        ↓
pendingOwner = newOwner，旧 Owner 仍然有效
        ↓
newOwner 调用 acceptOwnership()
        ↓
owner = newOwner，pendingOwner = 0
```

`renounceOwnership()` 被禁用，不得以放弃 Owner 代替移交。

### 3.2 JoinCareFairLaunchV1

Fair Launch 是 UUPS Proxy，未使用独立 `ProxyAdmin`。推荐治理拓扑：

```text
产品所有方 Governance Safe
          │ proposer/canceller/executor
          ▼
48 小时 Governance/Upgrade Timelock
          │ owner / upgrade authority
          ▼
JoinCareFairLaunchV1 Proxy
```

Fair Launch 具有多个独立权限域：

| 权限 | 合约字段/角色 | 主要能力 |
|---|---|---|
| 治理与升级 | `owner()` | UUPS 升级、角色/费率/policy/recovery 治理 |
| 生命周期执行 | `operator()` | create/activate/release/cancel |
| 紧急暂停 | `pauseGuardian()` | 暂停 contributions/payouts/claims |
| 恢复业务 | `unpauseTimelock()` | 恢复三个暂停域 |
| Recovery 接收 | `treasury()` | 接收合法 excess token recovery |
| 平台费 | `platformFeeConfig()` | 平台费率及接收 Safe |
| 项目收益 | 项目快照 `projectProceedsReceiver` | 对应项目募集净收益 |

Fair Launch 同样使用 `Ownable2StepUpgradeable`，且 `renounceOwnership()` 永久回滚。

### 3.3 JC Token

`JC` 使用 `Ownable2Step`。Owner 可调用 `mint()`，因此生产 Owner 必须是产品所有方控制的 Token Governance Safe 或 Timelock，不得保留开发方 mint 权限。

### 3.4 JoinCareCheckIn

`JoinCareCheckIn` 使用单步 `OwnableUpgradeable`。Owner 控制：

- UUPS 升级；
- Pause/Unpause；
- ERC20 rescue。

`transferOwnership(newOwner)` 成功后立即生效，没有 `acceptOwnership()` 第二步。执行前必须对目标地址进行双人复核。

### 3.5 Facet、Library 和 Implementation

Fair Launch 的 Core、Release、Funds、Admin、Views Facet 及 Logic/Runtime Library 没有需要分别移交的 Owner。实际权限和状态位于 Proxy。

Implementation 构造时已禁用 initializer。移交重点是：

- Proxy `owner/pendingOwner`；
- ERC-1967 implementation slot；
- Timelock 角色；
- Safe Owner/threshold；
- Operator、Guardian、Treasury、Fee Receiver。

---

## 4. 推荐生产方案

### 4.1 路径 A：使用最终生产权限全新部署（推荐）

适用于正式生产尚未部署，或现有部署仅用于测试的场景。

推荐顺序：

1. 产品所有方确认签名人及职责；
2. 创建生产 Governance、Emergency、Treasury、Platform Fee Safe；
3. 验证全部 Safe 地址、Owner、threshold、module、guard 和 runtime；
4. 部署 Governance Timelock 和 Unpause Timelock；
5. 将产品所有方 Safe 配置为对应 Timelock proposer/canceller/executor；
6. 部署 Implementation；
7. 部署 Proxy 并原子 initialize，直接写入最终生产权限；
8. 配置 Token policy、Beneficiary Safe 和 Release policy；
9. 验证 deployer 没有任何生产权限；
10. 完成 20-block finality 和双 RPC 只读复核；
11. 签署权限交割验收单后再启用 Backend/Job/Operator。

优点：

- 没有测试 Owner、测试项目和测试治理历史；
- 无需在有资产的合约上迁移关键角色；
- 初始化时直接绑定最终权限；
- 最容易审计、验收和披露。

### 4.2 路径 B：迁移既有部署

仅在产品所有方明确决定复用既有 Proxy，且完成代码、状态、资产、历史项目和治理风险复核后使用。

必须逐域迁移，不能只修改 `owner()` 后即宣称完成交割。

---

## 5. 移交前资料与地址确认

### 5.1 产品所有方必须提供的最终地址

不得只登记“管理员地址”而不说明用途。每一个地址都必须明确：地址类型、实际控制方、允许能力、禁止能力、是否接收资产、是否可变，以及移交后的责任人。

#### 5.1.1 地址类型说明

| 地址类型 | 含义 | 是否持有业务状态 | 是否应接收业务资产 | 移交关注点 |
|---|---|---:|---:|---|
| EOA | 由单个私钥、KMS 或 MPC 控制的链上账户 | 否 | 通常否 | 私钥控制方、nonce、gas、是否与其他签名域复用 |
| Safe | 多签合约钱包 | Safe 自身保存 Owner/threshold/nonce | 按角色决定 | Owner 集、threshold、module、guard、fallback handler |
| Timelock | 延时治理合约 | 保存角色、delay 和 operation | 通常不接收业务资产 | proposer/canceller/executor/admin、delay、operation 状态 |
| UUPS Proxy | 用户和系统实际交互的稳定合约地址 | 是 | 是，按合约储备规则托管 | Owner、Implementation slot、业务状态和储备 |
| Implementation | Proxy delegatecall 的逻辑代码 | 否 | 否 | runtime code hash、initializer 已禁用、升级来源 |
| Facet | Fair Launch Dispatcher 的不可变委托目标 | 否 | 否 | 功能边界、runtime hash；没有独立 Owner |
| Library | 被 Implementation/Facet 链接的逻辑或运行时库 | 否 | 否 | runtime hash；没有独立 Owner |
| ERC20 Token | 支付币或项目币合约 | Token 自身保存余额 | 是 | Token 身份、decimals、runtime/policy；不等同 JoinCare 管理权 |
| Zero Address | `0x0000000000000000000000000000000000000000` | 否 | 否 | 表示无 pending owner、无外部 admin 或开放角色时须结合具体字段解释 |

#### 5.1.2 生产地址用途登记表

建立并由双方签字确认以下表格。`最终生产地址` 不得留空后直接进入部署或迁移阶段。

| 地址名称 | 最终生产地址 | 要求类型/控制方式 | 具体用途 | 明确不能做什么 | 是否接收资金 | 变更方式 | 已独立复核 |
|---|---|---|---|---|---:|---|---|
| IDO V2 Proxy | `<address>` | UUPS Proxy | IDO 用户、项目方、Admin 和 Operator 的实际交互入口；保存 IDO 项目、购买、reserve、Owner、Operator 和 fee 配置 | 不能用 Implementation 地址代替；不能向非目标链同地址转账 | 是，按 IDO reserve 规则托管支付币和项目币 | 稳定地址；逻辑通过 Owner 治理的 UUPS 升级变更 | `[ ]` |
| IDO V2 Implementation | `<address>` | 已验证合约代码 | IDO V2 Proxy 当前执行的逻辑代码，由 ERC-1967 slot 指向 | 不保存 Proxy 业务状态；不接收项目款；不单独转 Owner；用户不应直接调用 | 否 | Proxy Owner 执行 `upgradeToAndCall` 后更新 | `[ ]` |
| Fair Launch Proxy | `<address>` | UUPS ERC1967 Proxy | Fair Launch 用户、项目方、Admin 和 Operator 的唯一业务入口；保存项目、position、policy、角色和 reserve | 不能与 Dispatcher Implementation、Facet 或 Library 混淆 | 是，按 Fair Launch reserve 规则托管支付币和项目币 | 稳定地址；Owner 由两步流程迁移，Implementation 由治理升级 | `[ ]` |
| Fair Launch Dispatcher Implementation | `<address>` | 已验证 Dispatcher 代码 | Proxy 当前 Implementation；根据 selector 委托到 Core/Release/Funds/Admin/Views Facet | 不保存 Proxy 状态；不接收资金；不直接提供给 C 端作为 spender | 否 | Governance Timelock 执行 UUPS 升级后替换 | `[ ]` |
| Fair Launch Facet/Library Set | `<addresses-and-digests>` | 五个 Facet + Logic/Runtime Library | 构成当前 Fair Launch 完整逻辑依赖集合；用于 ABI、selector、code hash 和 source revision 验证 | 没有独立 Owner；不能作为 Proxy、Treasury、Token、spender 或收款地址 | 否 | 随新的 Dispatcher/完整升级集合重新部署和冻结 | `[ ]` |
| Governance Safe | `<address>` | 产品所有方多签，推荐 2/3 或 3/5 | 提出、取消并按配置执行 Governance Timelock operation；批准升级、角色、policy、费率和 recovery 治理 | 不直接作为 Fair Launch Proxy Owner；不能绕过 Timelock delay；不应用作 Operator | 不接收项目款；外层交易 gas 通常由执行钱包承担 | Safe self-call 调整 Owner/threshold | `[ ]` |
| Emergency Safe | `<address>` | 产品所有方独立多签 | 作为 Fair Launch `pauseGuardian`，紧急暂停 contributions、payouts、claims；可控制 Unpause Timelock 的提案流程 | 不能升级、改费率、改 Token policy、提取项目款或代替 Governance Safe | 不接收项目款；外层交易 gas 通常由执行钱包承担 | Safe self-call；Fair Launch Guardian 地址由 Governance Timelock 修改 | `[ ]` |
| Treasury Safe | `<address>` | 产品所有方财资多签 | 作为 Fair Launch `treasury`，接收经治理批准、扣除全部 reserve 后的 excess-token recovery | 不能直接提取用户储备、项目收益或平台费；不是 Proxy Owner | 是，仅接收合法 recovery 资产 | Fair Launch `setTreasury`，需 Governance Timelock | `[ ]` |
| Platform Fee Safe | `<address>` | 产品所有方财务多签 | 接收 IDO/Fair Launch 平台手续费；Fair Launch 中还需 beneficiary attestation | 不能接收项目方净收益；不能升级或修改费率；不能提取非快照项目的费用 | 是，平台费 | IDO `setPlatformFees`；Fair Launch `setPlatformFee`，均只影响相应规则下的新项目/快照 | `[ ]` |
| Governance Timelock | `<address>` | self-admin Timelock，推荐 48h delay | 作为 Fair Launch `owner()` 和唯一 UUPS Upgrade Authority；执行治理配置、Token policy、角色、费率、invalidation 和 recovery | 不能跳过 delay；不应持有开发方 EOA admin；不作为日常 Operator | 否；Timelock 合约不支付交易 gas，外部 executor 支付 | Timelock 自身角色治理；Proxy Owner 采用两步迁移 | `[ ]` |
| Unpause Timelock | `<address>` | 独立 self-admin Timelock | 作为 Fair Launch `unpauseTimelock()`，仅恢复 contributions、payouts、claims | 不能暂停、升级、改配置、提取资金或创建项目 | 否 | Timelock 角色治理；Fair Launch `setUnpauseTimelock` | `[ ]` |
| IDO V2 Governance Safe | `<address>` | 产品所有方多签；可与 Governance Safe 相同或按批准方案独立 | 作为 IDO V2 `owner()`；控制升级、暂停、Operator、Token 准入、平台费、批量退款和 excess recovery | 不应直接作为自动化 Operator；不得绕过历史项目快照 | 否，除必要 gas | IDO V2 `transferOwnership` + `acceptOwnership` | `[ ]` |
| IDO V2 Operator | `<address>` | 独立 KMS/MPC/Signer EOA | 执行 IDO V2 项目创建等受限生命周期动作 | 不能升级、暂停、配置 Token、修改费率、batch refund 或 recovery；不是资金受益人 | 否，仅持有受控 gas | IDO V2 Owner 调用 `setOperator` | `[ ]` |
| Fair Launch Operator | `<address>` | 独立 KMS/MPC/Signer EOA | 执行 create、activate、release、cancel 等 durable lifecycle action | 不能升级、暂停/恢复、改 policy/费率/角色、提取资金；不能用于手工治理 | 否，仅持有受控 gas | Governance Timelock 调用 `setOperator` | `[ ]` |
| JC Token Governance | `<address>` | 产品所有方 Token Safe 或 Timelock | 作为 JC Token `owner()`，控制 `mint()` | 不能由开发方保留；不应与自动化 Operator 私钥复用 | JC Token 是否持有由治理决定；不应接收 IDO 项目款 | JC `transferOwnership` + `acceptOwnership` | `[ ]` |
| JC Token Contract | `<address>` | ERC20 合约 | JoinCare 平台 Token 的余额、转账、总供应及 mint 入口 | 不是 Governance Safe、Operator 或 IDO/Fair Launch Proxy；Token Owner 地址与 Token 合约地址不能混淆 | 是，作为 ERC20 资产本身 | 合约地址通常固定；Owner 采用两步迁移 | `[ ]` |
| CheckIn Governance | `<address>` | 产品所有方 Safe 或 Timelock | 作为 CheckIn `owner()`，控制升级、pause/unpause 和 ERC20 rescue | 不能提取其他合约储备；不能与 CheckIn Proxy 地址混淆 | 只可接收明确 rescue 的误转资产 | CheckIn 单步 `transferOwnership` | `[ ]` |
| CheckIn Proxy | `<address>` | UUPS Proxy | 用户实际调用的签到业务入口；保存签到业务状态和 Owner | 不能与 Implementation 或 Governance 地址混淆 | 正常业务不应托管项目资金 | 稳定地址；通过 Owner 治理升级 | `[ ]` |
| CheckIn Implementation | `<address>` | 已验证合约代码 | CheckIn Proxy 当前逻辑代码 | 不保存 Proxy 业务状态；不单独接收资产或转 Owner | 否 | CheckIn Owner 执行 UUPS 升级后更新 | `[ ]` |
| Project Proceeds Safe | `<address-per-project>` | 每个募资发行项目审核通过的多签 | 接收对应项目成功后的募集净收益；Fair Launch 中由创建时快照固定 | 不是 JoinCare 平台费地址；不能治理平台或提取其他项目资金 | 是，对应项目净收益 | 新项目创建前准入并写入快照；既有项目不能普通修改 | `[ ]` |
| Payment Token | `<token-addresses>` | 经治理准入的 ERC20 | 用户出资、退款、项目收益和平台费的计价/结算资产 | Token 合约不属于 JoinCare；不能把 Token 地址当作 receiver、Proxy 或 spender | 是，作为结算资产 | 通过 policy revision/admission 治理准入，不改变外部 Token 所有权 | `[ ]` |
| Project Token | `<address-per-project>` | 经治理准入的 ERC20 | 项目方存入、用户成功后 Claim、失败/取消后退回项目方的发行资产 | 不是支付币、平台费地址或治理地址；不得使用未准入或 runtime 漂移 Token | 是，作为发行资产 | 新项目创建前完成 policy；项目创建后地址快照固定 | `[ ]` |
| Production Deployer | `<address>` | 一次性部署 EOA/受控部署器 | 发送 CREATE、部署 Proxy/Implementation/Timelock，支付部署 gas | 部署完成后不能保留 Owner、Timelock 角色、Operator、Safe Owner 或资金受益权 | 否，仅部署 gas | 不迁移为业务角色；部署后做零残留验证 | `[ ]` |
| Permissionless Timelock Executor（如启用） | `zero address role` 或 `<address>` | 由治理方案明确 | 在 operation ready 后代为调用 execute；不能改变 operation payload | 不能 schedule、cancel 或修改 operation；开放 executor 不等于开放治理 | 否，仅支付执行 gas | Timelock 自身 `grantRole/revokeRole` | `[ ]` |

不得以 ENS、昵称、钱包标签或截图代替完整校验和地址。不得将 `Proxy`、`Implementation`、`Safe`、`Timelock` 和 `Operator` 地址互相替代。

#### 5.1.3 当前 Fair Launch 测试部署地址及用途

以下地址是 BSC 主网小额测试治理和已部署 Fair Launch 合约的当前状态，用于识别迁移来源和制作 preflight。它们是**测试参考地址，不自动构成生产准入**。

##### A. 核心合约和代码组件

| 地址 | 名称/类型 | 当前用途 | 是否持有状态/资产 | 权限移交处理 |
|---|---|---|---:|---|
| `0xaEC57de2b134e3539edbB70a552483031a14BD84` | Fair Launch ERC1967 Proxy | C 端、Admin、Operator 和用户实际调用的稳定入口；保存全部项目、position、reserve、角色和 policy 状态 | 是 | 生产若复用此部署，所有角色迁移都围绕该 Proxy；不得把 Implementation 地址提供给用户调用 |
| `0x45BE772bF52624c3cA5938832571687c5C59384C` | Dispatcher Implementation | Proxy 当前 ERC-1967 implementation；按 selector 委托到五个 Facet，并承载 UUPS 升级入口 | 否 | 没有独立 Owner；由 Proxy Owner 通过 UUPS 升级替换；禁止向该地址存款或直接初始化 |
| `0x25c6808be02fEfe43D7574f8b34E731F844d7141` | Core Facet | 处理 create、deposit、activate、contribute、end、cancel 相关 selector | 否 | 不单独移交；核对 runtime hash和 Dispatcher immutable 绑定 |
| `0xF77af7c40788b9c6ecB070229b186bF6c95d099a` | Release Facet | 处理 successful release、客观 runtime fail 和治理 invalidation 相关 selector | 否 | 不单独移交；不能作为普通调用入口或资金接收地址 |
| `0x0902E0131DAC3C8Eb566997476f04db227AA5c2b` | Funds Facet | 处理 refund、claim、project proceeds、platform fee 和 failed-token withdrawal selector | 否 | 不单独移交；实际资金始终由 Proxy 地址托管和转出 |
| `0xC7f62cccBFF48bF8920AfF57797d702eb6291D8E` | Admin Facet | 处理 pause/unpause、角色、Token policy、Beneficiary Safe、fee 和 recovery selector | 否 | 不单独移交；权限检查读取 Proxy storage 中的 Owner/Guardian/Timelock |
| `0x9D280BF09326546cf634AD14ee9723Cf503e918B` | Views Facet | 提供 project、position、角色、policy、reserve 等只读 selector | 否 | 不单独移交；只读系统仍应通过 Proxy 调用 |
| `0x7a78F769D0CE93615c9a1a78B7Df9cc9376fbd79` | Logic Library | 执行纯计算和参数验证，不读取/写入业务 storage | 否 | 不单独移交；升级时新组件必须重新核对链接地址和 code hash |
| `0x3e08DB515592aCAe733119861999C179aC50ab6E` | Runtime Library | 执行 Token code hash、decimals、balance、solvency 等 runtime 检查 | 否 | 不单独移交；不能作为 Treasury、Token 或用户交互地址 |

##### B. 当前测试治理、执行和资金地址

| 地址 | 当前角色 | 用途说明 | 明确不能做什么 | 生产处置 |
|---|---|---|---|---|
| `0x79c527B8Af5b1E27d5052f8dCe04D20733e72350` | Governance Test Safe | Governance Timelock 的 proposer/canceller/executor；负责提出并批准延时治理 operation | 不是 Fair Launch 直接 Owner；不能跳过 Timelock；不是 Treasury/平台费/项目收益地址 | 使用产品所有方生产 Governance Safe 替换，或完整轮换全部 Safe Owners 后重新准入 |
| `0x0eE7f9f79B44446eFE8741037D5d46419266083D` | Governance Timelock；Fair Launch `owner()` | 48 小时延时治理；当前唯一 Upgrade Authority；执行 Operator、Guardian、Treasury、fee、Token policy、invalidation 和 recovery 治理 | 不能由单个 EOA立即操作；不负责日常项目生命周期；不接收项目款 | 若复用，可替换其控制 Safe；若更换 Timelock，必须执行 Proxy 两步 ownership 迁移 |
| `0x070EBAE10689CB90EEC1377d2F23b3825Ab8c0C8` | Emergency Test Safe；`pauseGuardian()` | 可立即暂停 contributions、payouts、claims；也是当前 Unpause Timelock 的控制 Safe | 不能升级、改费率、改 Token policy、unpause 或提取项目资金 | 换为产品所有方独立 Emergency Safe，不能与单个 Operator EOA 合并 |
| `0xD8950F857a3485501E6cDcBE12Deae23911F35c0` | Unpause Timelock；`unpauseTimelock()` | 1 小时延时后恢复 contributions、payouts、claims | 不能 pause、升级、改角色/费率/policy、创建项目或提取资金 | 替换控制 Safe，或部署新的产品所有方 Unpause Timelock 后由 Governance Timelock更新地址 |
| `0x67D704A3C4c52f590C02050ee1e18652A6D784f1` | Treasury Test Safe；`treasury()` | 接收 `recoverExcessTokens` 转出的、超过全部 reserve 的合法 excess Token | 不能提取用户 reserve、项目收益或平台费；没有升级权 | 换为产品所有方 Treasury Safe，并重新核对是否存在可 recovery 余额 |
| `0xdBD2A66D827AeaFca9b65a6DC79501263730cbE1` | Platform Fee Test Safe | 当前全局平台费 receiver，费率为 250 bps；只有项目快照指向它时才能提取该项目平台费 | 不能提取项目净收益、用户退款储备或任意 Token；不能自行修改费率 | 换为产品所有方 Platform Fee Safe；变更只影响按新配置创建的项目快照 |
| `0x4aBA60fe416f5500E58eb07B5b487dbdF3A34dD9` | Project Proceeds Test Safe | 测试项目的募集净收益 receiver；由项目创建快照固定 | 不是平台 Treasury 或 Platform Fee Safe；不能提取其他项目收益 | 仅作为测试项目 receiver；正式项目应使用对应发行项目审核通过的独立 Safe |
| `0x1cB633B1B9D8ABa8DD26acB36cA5780373048b9d` | Fair Launch Test Operator EOA | 执行 create、activate、release、cancel 等受控生命周期动作并支付 gas | 不能升级、暂停/恢复、改角色/费率/policy、提取资金；不得用于手工治理 | 换为产品所有方生产 KMS/MPC/Signer，并保持 nonce 和 secret 管理独立 |
| `0xc7b4bf323C94CEb2E6e9bb7fbf89D06140941e5f` | 历史 IDO deployer；Fair Launch 测试 deployer；当前测试 Safe Owner A | 曾发送 Timelock和九组件 CREATE；仅因明确测试例外成为测试 Safe Owner A | 当前不是 Fair Launch Owner、Operator、Guardian、Treasury 或 Fee Receiver；不得保留生产 Safe Owner 或任何生产权限 | 正式生产必须从所有 Safe Owner、Timelock 角色、部署和运行权限中撤销 |

##### C. 当前测试 Safe Owner 地址

当前五个测试 Safe 共用以下 `2/3` Owner 集。这三个地址控制测试 Safe，而不是直接控制 Fair Launch Proxy；它们必须通过 Safe 达到门限，再经相应 Timelock或 Safe transaction执行。

| 地址 | 当前用途 | 生产要求 |
|---|---|---|
| `0xc7b4bf323C94CEb2E6e9bb7fbf89D06140941e5f` | Test Safe Owner A；历史 IDO deployer 测试例外 | 必须撤销，不得成为生产 Safe Owner |
| `0x35D3F22ab317569EA15728e59bC0EEeff521b687` | Test Safe Owner B | 除非产品所有方正式书面指定并重新完成身份/职责核验，否则不得自动沿用到生产 |
| `0x0C47E90BA6225c53a17650272f7bF4dC3dE805B3` | Test Safe Owner C | 除非产品所有方正式书面指定并重新完成身份/职责核验，否则不得自动沿用到生产 |

Safe Owner 地址的用途是对 Safe proposal 签名，不是直接接收平台费、项目款或调用 Proxy 的共享私钥。不得把任一 Safe Owner 地址同时用作 Operator/Relayer。

##### D. Safe 官方公共依赖地址

以下地址是当前测试 Safe 创建和运行所使用的 Safe v1.4.1 公共基础设施。它们是第三方公共依赖，不是 JoinCare 的 Owner、Operator、Treasury 或 Fee Receiver。

| 地址 | 组件 | 用途说明 | 禁止误用 |
|---|---|---|---|
| `0x4e1DCf7AD4e460CfD30791CCC4F9c8a4f820ec67` | SafeProxyFactory | 部署 Safe Proxy；根据 singleton、initializer 和 salt 创建 Safe 地址 | 不能作为 JoinCare Safe、Owner、Timelock、receiver 或业务 Proxy |
| `0x41675C099F32341bf84BFc5382aF534df5C7461a` | Safe Singleton | 当前测试 Safe 创建时使用的官方 Safe v1.4.1 singleton 依赖 | 不是产品所有方 Safe 地址；不能向其转入平台费或项目款 |
| `0x29fcB43b46531BcA003ddC8FCB67FFE91900C762` | SafeL2 Singleton | 当前 BSC 测试 Safe 最终使用的 `1.4.1+L2` singleton/mastercopy | 不是 JoinCare Governance Safe；不能被填写为 Safe Owner 或 receiver |
| `0xBD89A1CE4DDe368FFAB0eC35506eEcE0b1fFdc54` | SafeToL2Setup | Safe 创建时切换到 SafeL2 singleton 的 setup helper | 只在已审核的 Safe 初始化流程中使用；不是日常治理目标 |
| `0xfd0732Dc9E303f09fCEf3a7388Ad10A83459Ec99` | CompatibilityFallbackHandler | Safe 官方 fallback handler，提供签名兼容等处理 | 不是 Safe Owner、module、guard 或资金 receiver |

生产 Safe 必须重新核验其 singleton、factory、fallback handler 和 runtime code hash；不得仅因地址与测试记录一致就跳过供应链核验。

##### E. 支付 Token 地址

| 地址 | Token | 用途说明 | JoinCare 是否拥有其管理权 |
|---|---|---|---|
| `0x55d398326f99059fF775485246999027B3197955` | BSC USDT | Fair Launch/IDO 可准入的支付资产；主网 decimals 为 18 | 否。JoinCare 只能治理是否准入和是否 emergency block，不能控制发行方冻结、黑名单或升级能力 |
| `0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d` | BSC USDC | Fair Launch/IDO 可准入的支付资产；主网 decimals 为 18，且为外部 Proxy Token | 否。JoinCare 不能控制 USDC Implementation；必须监控 implementation/runtime 漂移并按 policy fail closed |

Token 地址不是 JoinCare Treasury、Operator 或合约 Owner 地址。Token `approve` 的 spender 必须是 IDO/Fair Launch Proxy，而不是 Implementation、Facet、Safe 或 Operator。

#### 5.1.4 历史 IDO V2 地址及用途

以下是历史 BSC 主网 IDO V2 的只读链上状态，用于说明迁移来源。其 EOA 治理结构不适合作为新的 Fair Launch 生产治理模板。

| 地址 | 当前/历史用途 | 权限或资金能力 | 生产处置 |
|---|---|---|---|
| `0x962256B9f09Dfd6f2C20891FeDb5163Dd03183C1` | 历史 IDO V2 UUPS Proxy | 用户和系统实际交互入口；保存项目、购买、reserve、owner/operator/fee 配置 | 若继续作为生产 IDO，必须完成 Owner、Operator、Fee Receiver迁移及历史项目快照审计；不得当作 Fair Launch Proxy |
| `0xff83bff37F7A51AD9307BD17553965a12251A431` | 历史 IDO V2 Implementation | Proxy 当前逻辑代码；不保存 Proxy 业务状态 | 不接收资金、不单独转 Owner；通过 Proxy Owner 的 UUPS 升级更换 |
| `0x7EabAe98f4CCF1b5377134257329ecD87BB53231` | 历史 IDO V2 `owner()` EOA | 当前控制升级、pause、Operator、Token 准入、费率、batch refund 和 recovery | 正式交割时迁移至产品所有方 Governance Safe；接受完成后该 EOA必须无 Owner 权限 |
| `0xfd712061ee08E070A930eCD45E935Ae5d7f5445C` | 历史 IDO V2 `operator()` EOA | 执行受限项目生命周期操作 | 替换为产品所有方独立 IDO生产 Signer；不得自动成为 Safe Owner或治理 Owner |
| `0x4aB97A8Fc84909B31a086aAb1E9AB28d528Bb820` | 历史 IDO V2 `feeReceiver()` EOA | 接收按项目创建快照计算的平台费 | 新项目应改为产品所有方 Platform Fee Safe；历史项目仍可能继续向该快照地址付款，迁移前必须盘点 |

历史 IDO 当前 `pendingOwner` 为零地址，平台费率为 `250 bps / 2.5%`。修改全局 Fee Receiver 或 Owner 不会修改历史项目已经保存的 `feeReceiver/projectOwner/successFeeRate`。

### 5.2 Safe 检查项

每个生产 Safe 至少验证：

```text
chainId
Safe 地址
singleton/mastercopy 版本
runtime code hash
Owner 完整集合
threshold
nonce
modules
guard
module guard
fallback handler
余额及持有 Token
```

默认要求：

- 不安装未审核 module；
- 不安装未审核 guard；
- Owner 不包含开发方、部署方、测试钱包或共享账户；
- threshold 不得为单人可控；
- Safe Owner 与 Operator/Relayer 地址不复用。

### 5.3 Timelock 检查项

每个 Timelock 至少验证：

```text
地址和 runtime code hash
minDelay
DEFAULT_ADMIN_ROLE
PROPOSER_ROLE
CANCELLER_ROLE
EXECUTOR_ROLE
是否向 zero address 开放 executor
是否残留 deployer 或测试 Safe 角色
当前 pending/ready operation
```

如果采用 self-admin Timelock：

- Timelock 自身持有 `DEFAULT_ADMIN_ROLE`；
- 外部 EOA、开发方和 deployer 不得持有 admin；
- `grantRole`、`revokeRole` 和 `updateDelay` 必须由 Timelock 对自身执行。

### 5.4 Proxy 与 Implementation 检查项

对每个 UUPS Proxy 验证：

```text
chainId 和 genesis hash
Proxy 地址、runtime code hash
ERC-1967 implementation slot
Implementation 地址、runtime code hash
owner、pendingOwner
operator/guardian/treasury/fee receiver
暂停状态
reserved balance 与实际余额
初始化不能重复执行
Implementation 初始化必须失败
非 Owner 升级必须失败
```

---

## 6. Safe 控制权移交

### 6.1 推荐方式：创建新的生产 Safe

若现有 Safe 包含任何开发方或测试 Owner，优先创建新的生产 Safe，而不是原地轮换。

流程：

1. 产品所有方在官方 Safe 界面创建生产 Safe；
2. 所有 Owner 独立确认地址、网络和 threshold；
3. 以一笔最小无资产测试交易验证 Safe 可正常达到门限并执行；
4. 再将新 Safe 接入 Timelock 或目标合约；
5. 验证新权限后撤销旧 Safe 的角色。

### 6.2 必须原地轮换 Safe Owner 时

Safe Owner 调整必须由 Safe 对自身发起交易。推荐顺序：

1. 先添加产品所有方新 Owner；
2. 根据目标治理规则调整 threshold；
3. 验证新 Owner 集能够独立达到门限；
4. 再逐个移除开发方/测试 Owner；
5. 最后重新验证完整 Owner 集和 threshold。

不得执行：

- 先移除旧 Owner，导致剩余签名人无法达到 threshold；
- 将 threshold 临时降为 `1` 后长期保留；
- 在同一批操作中加入未审核 module/guard；
- 仅依据 Safe UI 显示宣称完成，必须读取链上状态和事件。

### 6.3 Safe 完成证据

每个 Safe 保存：

- Safe transaction hash；
- 外层链上 transaction hash；
- `ExecutionSuccess` 事件；
- 执行前后 Owner 列表和 threshold；
- nonce 变化；
- module/guard/fallback handler 状态；
- receipt block/hash；
- exact 20-block successor parent-chain finality；
- 第二独立 RPC 的一致结果。

---

## 7. Timelock 角色移交

### 7.1 复用现有 Timelock，仅替换控制 Safe

对 self-admin OpenZeppelin `TimelockController`，角色变更需要由旧 proposer 提案，Timelock 到期后对自身执行。

推荐两阶段迁移：

#### 阶段 1：授予新 Safe 权限

Timelock operation 的目标为 Timelock 自身，调用：

```text
grantRole(PROPOSER_ROLE, newGovernanceSafe)
grantRole(CANCELLER_ROLE, newGovernanceSafe)
grantRole(EXECUTOR_ROLE, newGovernanceSafe)  // 仅在 executor 为关闭模式时
```

等待 delay 并执行后，验证新 Safe 的三个角色均为 `true`。

#### 阶段 2：撤销旧 Safe 权限

由新 Safe 提案，Timelock 对自身执行：

```text
revokeRole(PROPOSER_ROLE, oldGovernanceSafe)
revokeRole(CANCELLER_ROLE, oldGovernanceSafe)
revokeRole(EXECUTOR_ROLE, oldGovernanceSafe)
```

执行后验证旧 Safe、开发方和 deployer 均无角色。

两阶段之间存在受控的短期权限重叠，必须记录开始时间、结束时间和 operation ID。禁止在新 Safe 尚不能独立提案和执行时撤销旧 Safe。

### 7.2 更换整个 Governance Timelock

若 Fair Launch 要从旧 Timelock 迁移到新的产品所有方 Timelock：

1. 完成新 Timelock 部署和角色验证；
2. 旧 Timelock schedule：Fair Launch Proxy `transferOwnership(newTimelock)`；
3. 等待旧 Timelock delay；
4. 用户单独授权后 execute；
5. 验证 `owner=oldTimelock`、`pendingOwner=newTimelock`；
6. 新 Timelock schedule：Fair Launch Proxy `acceptOwnership()`；
7. 等待新 Timelock delay；
8. 用户单独授权后 execute；
9. 验证 `owner=newTimelock`、`pendingOwner=0`；
10. 验证旧 Timelock不能再调用 Owner-only 函数或升级 Proxy。

注意：第 5 步至第 8 步期间，旧 Timelock仍是 Owner。不得把 `pendingOwner` 已设置误报为所有权已完成。

### 7.3 Unpause Timelock 移交

两种方式：

- 保留原 Unpause Timelock，仅按 7.1 替换 proposer/canceller/executor Safe；或
- 部署新的 Unpause Timelock，再由 Governance Timelock调用 Fair Launch `setUnpauseTimelock(newTimelock)`。

更新后分别验证：

- 新 Timelock可以执行 `unpause*`；
- 旧 Timelock调用 `unpause*` 必须失败；
- Pause Guardian 与 Unpause Timelock 地址保持不同。

---

## 8. IDO V2 权限移交步骤

### 8.1 只读 preflight

记录：

```text
Proxy/Implementation 地址及 code hash
owner()
pendingOwner()
operator()
feeReceiver()
successFeeRate()
paused()
支付 Token 和项目 Token 准入状态
所有未终结项目及其储备
```

确认新 Governance Safe、Operator 和 Platform Fee Safe 均为最终生产地址。

### 8.2 先迁移可变角色

当前 Owner 逐项执行并验证：

```solidity
setOperator(PRODUCTION_IDO_OPERATOR)
setPlatformFees(APPROVED_FEE_BPS, PRODUCTION_PLATFORM_FEE_SAFE)
```

注意：`setPlatformFees` 只影响后续新建项目；既有项目使用创建时快照的 `feeReceiver` 和 `successFeeRate`。

### 8.3 发起 Owner 两步移交

当前 Owner 调用：

```solidity
transferOwnership(PRODUCTION_GOVERNANCE_SAFE)
```

交易 finalized 后必须读取：

```text
owner()        = 旧 Owner
pendingOwner() = PRODUCTION_GOVERNANCE_SAFE
```

### 8.4 产品所有方接受 Owner

由生产 Governance Safe 对 IDO V2 Proxy 调用：

```solidity
acceptOwnership()
```

完成后验证：

```text
owner()        = PRODUCTION_GOVERNANCE_SAFE
pendingOwner() = 0x0000000000000000000000000000000000000000
```

### 8.5 IDO V2 负向验证

使用只读模拟或 `eth_call` 验证：

- 旧 Owner 调用 `setOperator` 失败；
- 旧 Owner 调用 `pause` 失败；
- 旧 Owner 调用 `configurePaymentToken` 失败；
- 旧 Owner 调用 `upgradeToAndCall` 失败；
- 任意未授权 EOA 调用 Owner-only 函数失败；
- 新 Governance Safe 的批准交易可调用 Owner-only 函数。

### 8.6 关于现有 transferOwnership.js

当前通用脚本 `JoinCare_SmartContract-main/scripts/transferOwnership.js` 中的 IDO 分支只调用 `transferOwnership()`，且日志文字源于旧 `JoinCare_v1` 单步模型。

`JoinCareIDOv2` 实际为 `Ownable2StepUpgradeable`，因此：

- 不得将脚本输出“已转移 owner”当作移交完成；
- 必须额外由新 Safe 执行 `acceptOwnership()`；
- 必须验证 `pendingOwner()` 清零；
- 正式操作前应使用专门、checksum-bound 的 IDO V2 移交 bundle，而不是直接复用该通用脚本。

---

## 9. Fair Launch 权限移交步骤

### 9.1 推荐迁移顺序

```text
1. 产品所有方生产 Safe 完成并验收
2. Governance/Unpause Timelock 完成并验收
3. Operator、Guardian、Treasury、Fee Receiver 完成准备
4. 迁移/替换 Timelock 控制 Safe
5. 更新 Fair Launch 非 Owner 角色
6. 必要时迁移 Fair Launch Owner 到新 Governance Timelock
7. 撤销全部旧角色
8. 完整负向验证
```

### 9.2 Governance Timelock 更新非 Owner 角色

由产品所有方 Governance Safe 发起 Timelock operation，目标为 Fair Launch Proxy，按批准范围调用：

```solidity
setOperator(PRODUCTION_FAIR_LAUNCH_OPERATOR)
setPauseGuardian(PRODUCTION_EMERGENCY_SAFE)
setUnpauseTimelock(PRODUCTION_UNPAUSE_TIMELOCK)
setTreasury(PRODUCTION_TREASURY_SAFE)
setPlatformFee(APPROVED_FEE_BPS, PRODUCTION_PLATFORM_FEE_SAFE)
```

可以使用原子 batch，但必须逐项冻结：

- target；
- value；
- calldata；
- calldata hash；
- predecessor；
- salt；
- operation ID；
- expected call count 和 index；
- earliest execute time。

`scheduleBatch` 与 `executeBatch` 是两个独立授权边界。

### 9.3 Fair Launch Owner 迁移

如果新生产部署在 initializer 中已直接使用最终 Governance Timelock，无需再次迁移。

如果复用旧 Proxy 并替换 Timelock，严格按照 7.2 的两步流程执行：

```text
旧 Timelock → transferOwnership(newTimelock)
新 Timelock → acceptOwnership()
```

完成后：

```text
owner()        = PRODUCTION_GOVERNANCE_TIMELOCK
pendingOwner() = zero address
```

### 9.4 Beneficiary Safe 与项目资金

产品所有方 Platform Fee Safe 需要通过：

```solidity
configureBeneficiarySafe(account, true, attestationDigest, attestationVersion)
```

完成审核和准入。

每个募资发行项目的 Project Proceeds Safe 也必须独立准入。不得把所有发行项目收益地址统一替换为产品所有方 Treasury。

### 9.5 Fair Launch 负向验证

至少验证：

- deployer 不能 `setOperator`；
- 旧 Timelock不能执行 Owner-only 函数；
- 旧 Operator 不能 create/activate/release/cancel；
- 旧 Guardian 不能 pause；
- 旧 Unpause Timelock不能 unpause；
- 任意 EOA 不能升级 Proxy；
- `renounceOwnership()` 永久失败；
- Implementation 重复初始化失败；
- Proxy 重复初始化失败；
- Treasury、平台费和项目收益提款只能进入对应固定 Safe。

---

## 10. JC Token 权限移交

### 10.1 Preflight

记录：

```text
Token 地址和 runtime code hash
owner()
pendingOwner()
totalSupply()
产品所有方 Token Governance Safe
```

### 10.2 两步移交

当前 Owner：

```solidity
transferOwnership(PRODUCTION_TOKEN_GOVERNANCE_SAFE)
```

产品所有方 Token Governance Safe：

```solidity
acceptOwnership()
```

最终验证：

- `owner()` 为产品所有方 Safe；
- `pendingOwner()` 为零地址；
- 旧 Owner 调用 `mint()` 失败；
- 新 Owner 仅通过批准的 Safe 流程执行 mint；
- 开发方不保留 Token mint 权限。

---

## 11. JoinCareCheckIn 权限移交

`JoinCareCheckIn` 是单步 Owner 转移。当前 Owner 调用：

```solidity
transferOwnership(PRODUCTION_CHECKIN_GOVERNANCE)
```

该交易成功后立即生效。操作前必须确认：

- 目标地址非零；
- 目标地址是已验证的产品所有方 Safe/Timelock；
- 地址、chainId 和 target Proxy 已由两人独立核对；
- 产品所有方已证明能够控制目标地址。

完成后验证：

- `owner()` 为产品所有方治理地址；
- 旧 Owner 无法 pause/unpause；
- 旧 Owner 无法 upgrade；
- 旧 Owner 无法 rescue ERC20；
- 新治理地址可通过正式流程执行上述操作。

---

## 12. 已创建项目的快照限制

全局 Owner、Fee Receiver 或治理权限移交，不会自动重写已经创建项目的快照。

### 12.1 IDO V2 已有项目固定字段

至少包括：

- `projectOwner`；
- `feeReceiver`；
- `successFeeRate`；
- project/payment Token；
- cap、时间和 vesting 参数。

### 12.2 Fair Launch 已有项目固定字段

至少包括：

- `projectOwner`；
- `projectProceedsReceiver`；
- `platformFeeReceiver`；
- `platformFeeBps`；
- project/payment Token policy digest/version；
- release policy digest/version；
- Token、cap、时间和 vesting 参数。

因此在移交前必须枚举所有非终结项目并确认：

- 项目受益人地址是否仍可控制；
- 平台费地址是否符合合同；
- 是否存在测试项目或测试资产；
- 是否需要先终结、取消或重新创建项目。

链上项目已创建后，不得通过手工改数据库或伪造事件修改这些快照。

---

## 13. 每笔交易的执行门禁

每一笔权限移交交易执行前必须重新完成：

### 13.1 链身份

```text
chainId = 56
genesis hash
最新 finalized block
目标 Proxy/Timelock/Safe 地址
runtime code hash
ERC-1967 implementation slot
```

### 13.2 业务 pre-state

```text
owner/pendingOwner
Safe owners/threshold/nonce
Timelock roles/minDelay/operation state
operator/guardian/unpause/treasury/fee receiver
paused 状态
项目和储备状态
```

### 13.3 交易事实

```text
from / Safe
target
value = 0（除非另有书面批准）
selector
完整参数
calldata hash
Safe transaction hash 和 nonce
Timelock operation ID、salt、predecessor
gas limit 和最大费用预算
预期事件和预期 post-state
```

任一事实变化都必须废弃旧确认并重新 preflight。

---

## 14. Receipt、事件与 Finality 验证

每笔成功交易至少保存：

```text
transaction hash
status = 1
from / to
nonce
block number / block hash / timestamp
gas used / effective gas price / fee
input calldata hash
关键事件及全部参数
执行前后状态差异
```

BSC 主网 finality 按 exact `20_blocks` 验证：

1. 从 receipt block 开始读取第 1 至第 20 个 successor；
2. 每个 successor 的 `parentHash` 必须等于前一个 block hash；
3. 两轮读取结果必须一致；
4. 使用第二个独立 RPC provider 重复验证；
5. 两个 provider 对 receipt block hash 和第 20 个 successor hash 必须一致。

未经 finality 验证的权限变更不得标记为完成。

---

## 15. 开发方零残留权限验收

交割结束后，对所有已知开发方、部署方、测试和旧 Relayer 地址逐个检查。

### 15.1 必须为零的权限

```text
所有生产 Safe Owner 身份
Timelock DEFAULT_ADMIN_ROLE
Timelock PROPOSER_ROLE
Timelock CANCELLER_ROLE
Timelock EXECUTOR_ROLE
IDO/Fair Launch/JC/CheckIn owner 或 pendingOwner
IDO/Fair Launch Operator
Pause Guardian / Unpause authority
Treasury / Platform Fee Receiver
KMS/MPC/secret manager 访问权
生产服务器和发布控制面写权限
```

### 15.2 负向模拟

对每个旧地址使用 `eth_call` 或隔离模拟验证至少一个对应敏感 selector 必须回滚，并保存 revert 结果。

### 15.3 不能只检查合约

完整交割还必须同步移交或撤销：

- 域名/DNS；
- 云账号和服务器；
- Git 仓库和 CI/CD；
- 容器 Registry；
- RPC 服务账号；
- KMS/MPC/Web3Signer；
- Safe 官方账户和通知渠道；
- 监控、告警和应急联系人；
- 发布签名根和 recovery portal 发布权。

链上 Owner 已转移但开发方仍控制 Operator 密钥、发布服务器或 KMS，不构成完整交割。

---

## 16. 故障与恢复边界

### 16.1 `transferOwnership` 已发起但未接受

状态：

```text
owner = 旧 Owner
pendingOwner = 新地址
```

处理：

- 暂停其他治理变更；
- 重新核验 pendingOwner；
- 如果地址正确，由新 Owner 执行 accept；
- 如果地址错误，由当前旧 Owner 重新调用 `transferOwnership(correctAddress)` 覆盖 pendingOwner；
- 不得由无关地址尝试 accept。

### 16.2 新 Owner 已接受但发现配置错误

所有权已不可原地回滚。必须由新 Owner 发起新的两步移交返回正确地址，并重新走完整 finality 和验收流程。

### 16.3 Timelock 新角色已授予但旧角色尚未撤销

这是受控迁移窗口，不是最终状态。必须：

- 冻结非移交治理；
- 验证新 Safe 可独立控制；
- 尽快执行已批准的旧角色撤销 operation；
- 记录重叠窗口和全部 operation ID。

### 16.4 旧角色已撤销但新 Safe 无法操作

这是严重治理锁死事故。禁止通过开发方私钥或未批准升级绕过。应按产品所有方预先批准的灾难恢复方案处理；若没有链上恢复路径，只能评估新部署和业务迁移。

### 16.5 Safe Owner/threshold 错误

如果 Safe 仍可达到当前 threshold，立即发起修正 Safe self-call；如果已无法达到 threshold，不能由外部管理员恢复，必须按 Safe 官方恢复机制或重新部署治理体系评估。

---

## 17. 禁止事项

严禁：

- 将所有生产权限移交给单个 EOA；
- 将开发方地址保留为“备用 Owner”；
- 将部署钱包保留为 Timelock admin；
- 通过 `renounceOwnership()` 代替移交；
- 在未验证新权限前撤销旧权限；
- 将 `pendingOwner` 设置成功误报为移交完成；
- 将 Safe 达到签名门限误报为链上执行完成；
- 将 Timelock scheduled/ready 误报为 executed；
- 用 Operator/Relayer 私钥手工执行治理交易；
- 在数据库中伪造链上 Owner、Safe 或 Timelock 状态；
- 手工修改 ERC-1967 slot 或合约 storage；
- 使用测试 RPC、错误 chainId 或未经核验的合约地址；
- 将测试 Safe、测试 Owner、测试 Treasury 或测试平台费地址直接视为生产准入；
- 未经单独授权广播任何不可撤销的主网交易。

---

## 18. 最终验收清单

### 18.1 IDO V2

- [ ] Proxy/Implementation 地址和 code hash 正确；
- [ ] `owner` 为产品所有方 Governance Safe；
- [ ] `pendingOwner` 为零地址；
- [ ] Operator 为产品所有方生产 Signer；
- [ ] Fee Receiver 为产品所有方 Platform Fee Safe；
- [ ] 费率与商业合同一致；
- [ ] 旧 Owner/Operator 的敏感调用均失败；
- [ ] 非授权 UUPS 升级失败。

### 18.2 Fair Launch

- [ ] Proxy `owner` 为产品所有方 Governance Timelock；
- [ ] `pendingOwner` 为零地址；
- [ ] Governance Safe Owner/threshold 正确；
- [ ] Governance Timelock 角色和 delay 正确；
- [ ] Emergency Safe Owner/threshold 正确；
- [ ] Unpause Timelock 角色和 delay 正确；
- [ ] Operator 为产品所有方生产 Signer；
- [ ] Treasury 为产品所有方 Treasury Safe；
- [ ] Platform Fee Receiver 为产品所有方 Platform Fee Safe；
- [ ] Platform Fee BPS 与合同一致；
- [ ] Beneficiary Safe attestation 正确；
- [ ] deployer、开发方和测试地址没有权限；
- [ ] `renounceOwnership` 失败；
- [ ] 非授权升级和配置变更失败。

### 18.3 JC Token / CheckIn

- [ ] JC Token Owner 为产品所有方 Token Governance；
- [ ] JC Token pendingOwner 为零地址；
- [ ] 旧 Owner 不能 mint；
- [ ] CheckIn Owner 为产品所有方治理地址；
- [ ] 旧 Owner 不能 pause、upgrade 或 rescue。

### 18.4 治理和基础设施

- [ ] 所有 Safe 均无未审核 module/guard；
- [ ] 所有 Timelock 均无开发方/部署方角色；
- [ ] Operator、Safe Owner、Guardian、Executor 签名域分离；
- [ ] KMS/MPC/secret manager 权限已移交；
- [ ] CI/CD、服务器、域名和 Registry 权限已移交；
- [ ] 每笔交易 receipt/event/finality 证据齐全；
- [ ] 权限清单和链上状态一致；
- [ ] 产品所有方已完成独立恢复演练。

---

## 19. 交割证据目录建议

建议为每次生产交割建立只读证据目录：

```text
authority-handover-<date>/
  00-scope-and-approval/
  01-address-register/
  02-safe-preflight/
  03-timelock-preflight/
  04-proxy-implementation-preflight/
  05-unsigned-calldata-bundles/
  06-safe-proposals/
  07-timelock-schedule-receipts/
  08-timelock-execute-receipts/
  09-ownership-transfer-receipts/
  10-ownership-accept-receipts/
  11-role-update-receipts/
  12-finality-evidence/
  13-negative-authority-tests/
  14-final-state-snapshot/
  15-signed-acceptance/
```

证据文件必须记录 SHA-256，并保持 append-only。不得保存钱包秘密、原始签名或 raw transaction。

---

## 20. 交割签署模板

```text
网络：BNB Smart Chain mainnet
Chain ID：56
交割日期：
交割范围：IDO V2 / Fair Launch / JC Token / CheckIn / Safe / Timelock / Operator

产品所有方确认：
1. 已独立核对全部生产地址和门限；
2. 已证明能够控制 Governance/Emergency/Treasury/Fee Safe；
3. 已接管 KMS/MPC/Operator 和发布控制面；
4. 已收到全部 receipt、事件、finality 和负向验证证据；
5. 已知链上交易不可撤销，恢复需要新的治理交易；
6. 已知历史项目的 receiver/fee/policy 快照不会随 Owner 自动修改；
7. 已确认开发方不保留生产权限。

产品所有方授权代表：
签署日期：

开发/交付方确认：
1. 不保留生产 Owner、Safe Owner、Timelock 角色和 Operator 密钥；
2. 不保留 Treasury、平台费或 recovery 受益权；
3. 已交付权限和证据清单；
4. 未保存产品所有方钱包秘密。

开发/交付方授权代表：
签署日期：
```

---

## 21. 权威源码与相关文档

操作前必须以目标发布版本的源码和链上状态为准：

```text
JoinCare_SmartContract-main/contracts/JoinCareIDOv2.sol
JoinCare_SmartContract-main/contracts/JoinCareFairLaunchV1.sol
JoinCare_SmartContract-main/contracts/fairlaunch/JoinCareFairLaunchV1Shared.sol
JoinCare_SmartContract-main/contracts/fairlaunch/JoinCareFairLaunchV1AdminFacet.sol
JoinCare_SmartContract-main/contracts/governance/JoinCareFairLaunchTimelock.sol
JoinCare_SmartContract-main/contracts/JC.sol
JoinCare_SmartContract-main/contracts/JoinCareCheckIn.sol
```

相关设计和部署记录：

```text
docs/IDO-FAIR-LAUNCH-DEVELOPMENT-DESIGN.md
docs/IDO-FAIR-LAUNCH-DEVELOPMENT-PLAN-SECURITY-AUDIT.md
docs/FAIR-LAUNCH-V1-DEPLOYMENT-AND-RECOVERY-RUNBOOK.md
docs/FAIR-LAUNCH-BSC-MAINNET-GOVERNANCE-TEST-RECORD.md
docs/IDO-BSC-MAINNET-DEPLOYMENT-GUIDE.md
docs/IDO-BSC-MAINNET-DEPLOYMENT-RECORD.md
```

如本文与目标生产 commit 的源码不一致，以源码、编译产物、链上状态和最新经批准的治理决议为准，并在执行前修订本文。
