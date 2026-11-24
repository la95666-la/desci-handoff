# IDO V3 管理权限迁移（Safe）操作手册与 ABI

> 用途：交接操作步骤、ABI 与验收规范。本文和附件不构成生产写入授权；未填写并复核新签名人/新 Safe、未完成应用兼容性准备时，不得发起交易。本次编写未执行任何链上迁移。

> **源码修订说明（待发布，不代表线上已生效）：**`fix/v3-dynamic-owner` 已将生产 approved-mode 的固定 Owner 地址比较改为在所核验区块读取 `owner()`；Admin 同步移除旧地址比较。仍保留独立 Owner Safe code hash、既有 Token 提案 2/3 门槛和 Governor/Operator/Proxy/Implementation 约束。第 7 节关于固定地址的说明是**旧部署的迁移风险**；仅在 Backend、独立 Job、Admin 支持版本分别发布并验收后才解除。新 Safe 字节码不同或 legacy signed-manifest 模式仍需独立处理，不承诺任何 Safe 版本都可无配置移交。

## 1. 先选对迁移类型

| 需求 | 应选择的方案 | 是否改变 V3 的 Owner/Governor 地址 |
|---|---|---|
| 换管理团队、交接硬件钱包，仍使用现有 Safe | **方案 A：更换 Safe 签名人**，通常优先 | 不改变 |
| V3 Owner 必须改为另一个已部署 Safe | **方案 B：两步 Owner 移交** | 只改变 Owner |
| 更换白名单治理团队 | 对 Governor Safe 单独执行方案 A | 不改变 Governor |
| 白名单 Governor 必须换为另一个 Safe 地址 | **当前 V3 无直接接口**，需单独升级设计/审计 | 非本手册常规操作 |
| 仅更换平台业务签名地址 | 附录 Operator 迁移 | 只改变 Operator，不是 Safe 管理团队交接 |

**Owner Safe 与 Governor Safe 是两个独立账户。更换其中一个的签名人，不会自动更换另一个。**

## 2. 当前部署与快照

以下是 **2026-11-03 10:02:35 UTC+8，区块 131349488** 的只读快照，不是实施时可直接复用的 nonce。

| 项目 | 地址 / 数值 |
|---|---|
| 网络 | BSC 主网，chainId `56`；Safe 网络标识 `bnb` |
| V3 Proxy | `0x0D4040875742975269c55E16C8884d3Ee9510B8E` |
| Implementation | `0x887fCC6E54Fc8229919166Ad90B16C19B4992560` |
| 当前 Owner Safe | `0x79c527B8Af5b1E27d5052f8dCe04D20733e72350` |
| 当前 Governor Safe | `0xc46659e67543AAAeC553F7f3FC8F004B7Dd286EF` |
| Operator | `0xfd712061ee08E070A930eCD45E935Ae5d7f5445C` |
| pendingOwner | 零地址，无在途 Owner 移交 |
| Owner Safe | VERSION `1.4.1`，threshold `2`，nonce 快照 `6` |
| Governor Safe | VERSION `1.5.0`，threshold `2`，nonce 快照 `9` |
| 两个 Safe 的 modules | 此次分页读取均为空，next 为 sentinel `0x...01` |

两个 Safe 当前签名人为相同的三人，`getOwners()` 顺序如下：

1. `0xc7b4bf323C94CEb2E6e9bb7fbf89D06140941e5f`
2. `0x35D3F22ab317569EA15728e59bC0EEeff521b687`
3. `0x0C47E90BA6225c53a17650272f7bF4dC3dE805B3`

两个地址在逻辑上分权，但现有签名成员相同，不代表密钥保管人员独立。Safe VERSION、modules 列表为空也不能替代 singleton、Guard、Fallback Handler 和密钥控制权审计。

源码依据：`JoinCareIDOv3.sol`、OpenZeppelin 5.1.0 `Ownable2StepUpgradeable.sol`；合约仓库提交 `3ca73317b5cbefba65402d235aaa3222f536d1e0`。Safe 接口参考见文末。治理前须重新查询，升级实现后本文需要重新审阅。

## 3. 所有方案的执行前门禁

1. 填写交接单：选择 A/B、原 Safe、新成员/新 Safe、目标 threshold、操作人和复核人、维护窗口。
2. 使用硬件钱包屏幕/独立渠道复核新地址，证明新成员实际可签名；不得仅信任聊天粘贴地址或地址簿名称。新 Safe 必须已部署在 BSC，不是其他链同地址或尚未部署的预测地址。
3. 在两个独立 RPC 固定区块读取 V3 `owner/pendingOwner/operator/whitelistGovernor` 和 EIP-1967 implementation slot，确认目标身份。
4. 对涉及的每个 Safe 读取 `getOwners/getThreshold/nonce/VERSION`、模块完整分页、Guard/Fallback Handler、singleton 等；与 UI 的队列对照，冻结无关提案，记录尚未执行的 nonce、SafeTxHash、calldata 和签名。
5. 备份应用配置、部署版本与治理状态。数据库/outbox 仅作核验，不手工改成功状态。方案 B 必须先完成第 7 节的兼容准备。
6. 明确交易内层 `to/value/data/operation`，逐字节独立解码。本文普通内层调用均为 **CALL（0）、value=0 BNB**。若 Safe UI 用 MultiSend/委托执行包装批次，必须另外核对外层 MultiSend 地址、operation 与每个内层调用，不把外层 DelegateCall 误当本文的直接 CALL。
7. 对实际 SafeTx 做模拟，记录 nonce、SafeTxHash、预估 gas、签名门槛与支付 gas 的执行钱包。Safe 自身转账 value 为 0 不等于无需 BNB gas。
8. 当前签名门槛不足时，不得降低门槛或拿 relayer 私钥绕过。不要开启与交接无关的 module、Guard 或恢复插件。

## 4. 方案 A：保留 Safe 地址，更换签名人（推荐常规交接）

### A1. 使用 Safe 官方管理界面

1. 打开 `https://app.safe.global/`，选择 BNB Smart Chain；添加/进入准确的 Owner Safe 地址，不根据最近使用列表猜账户。
2. 用**当前签名人钱包**连接。在 Settings → Setup / Signers / Owners（菜单名随版本变化）检查现有三名成员和 `2 of 3`。
3. 选择 Replace owner / 替换签名人，指定一名旧成员与已独立核验的新成员。若界面没有 Replace，使用 A2，不随意先删除到不足门槛。
4. 预览确认：**调用目标是这个 Safe 自己，不是 V3 Proxy，也不是 Implementation**；实际方法为 `swapOwner(prevOwner,oldOwner,newOwner)`，value=0。
5. 默认一次只替换一人，保留两名已验证可用成员。保持原 `2/3`，不要顺手降为 `1/3`。每笔需要满足**执行当时**的成员和门槛，通常先收集当前三人中至少两人的批准。
6. 复核 SafeTxHash/nonce 后执行，等待至少 15 次确认并复核 canonical receipt。
7. 独立读取 `getOwners/getThreshold`：旧成员已移除、新成员出现、threshold 未变；核对 `RemovedOwner/AddedOwner` 和 Safe 执行结果。
8. 让新成员参与下一笔合法迁移交易的签名，确认其控制权后再换下一人。成员每次变化后重新读取链表和 nonce，重新建提案；不要批量预签后续交易。
9. 如果还要换 Governor 的管理团队，切换至 Governor Safe，重新执行步骤 2–8；这需要 Governor Safe 自己的批准。
10. 验收 V3 `owner()` 和 `whitelistGovernor()` 地址均保持原值，Operator/费率/收款人/Implementation 不变；检查 Backend/Job/freshness/Proof 和权限校验没有异常。

### A2. Safe Transaction Builder 的精确参数

Safe Apps → Transaction Builder → 输入**正在管理的 Safe 自己的地址** → 加载附件 `safe-owner-management.abi.json` → 选 `swapOwner`。

| 字段 | 值 |
|---|---|
| Safe 发起账户 | 正在交接的 Owner Safe 或 Governor Safe |
| 内层目标 to | 同一个 Safe 地址（self-call） |
| 内层 operation / value | CALL `0` / `0` BNB |
| 方法 | `swapOwner(address prevOwner,address oldOwner,address newOwner)` |
| prevOwner | 当前链表里指向 oldOwner 的前驱；不是签名钱包、不是阈值 |
| oldOwner | 本次被替换的签名人 |
| newOwner | 尚未在该 Safe 中的新签名人；不能为零地址、sentinel 或 Safe 自己 |

`getOwners()` 返回链表遍历顺序。若 oldOwner 在第 1 位，prevOwner 使用 sentinel：

`0x0000000000000000000000000000000000000001`

否则 prevOwner 使用当前列表中 oldOwner 的前一位。**下面只展示快照的对应关系，每笔执行后都会变化，不能照抄旧表。**

| oldOwner（快照） | prevOwner（快照） |
|---|---|
| `0xc7b4bf323C94CEb2E6e9bb7fbf89D06140941e5f` | `0x0000000000000000000000000000000000000001` |
| `0x35D3F22ab317569EA15728e59bC0EEeff521b687` | `0xc7b4bf323C94CEb2E6e9bb7fbf89D06140941e5f` |
| `0x0C47E90BA6225c53a17650272f7bF4dC3dE805B3` | `0x35D3F22ab317569EA15728e59bC0EEeff521b687` |

`swapOwner` 不改变 ownerCount 和 threshold。若必须调整数量/门槛：

- `addOwnerWithThreshold(newOwner, threshold)`：增加成员并设置有效门槛；先确认新成员已控制密钥。
- `removeOwner(prevOwner, owner, threshold)`：删除并设置有效门槛；不得把最后可用签名能力移除。
- `changeThreshold(threshold)`：只改门槛，要求 `1 <= threshold <= ownerCount`，但业务要求不应退化为单签。
- 三者同样必须 Safe self-call，EOA 直接向 Safe 发送这些方法会被拒绝。优先官方 UI，若批处理必须逐步模拟链表变化。

### A3. 失败、撤销与历史提案

- 未执行提案不改变成员。UI 删除提案或“Reject”不一定使其他人手中的签名失效；有风险时由当前门槛执行同 nonce 的安全取消/替换交易，并核对链上 nonce 已消耗。
- **成员变更不会自动清空 Safe nonce 或撤销全部历史提案。**后续 nonce 的旧签名若签名人仍是 owner 且满足当前门槛，仍可能被使用。必须审查/处理所有遗留提案。
- 替换成功后需回退时，由**新状态下**足够的成员批准反向 `swapOwner`；原成员不能单方面回滚。若新团队丢失足够签名能力，不能保证可恢复。

## 5. 方案 B：把 V3 Owner 地址转交给新 Safe

> **严格门禁：当前应用固定信任原 Owner Safe 地址。未通过第 7 节的配套适配与演练，不得执行 B2/B3。**链上 `acceptOwnership` 成功不等于平台迁移完成。本文没有指定新 Safe，因此不提供可误点执行的成品交易批次。

### B1. 准备并验收新 Safe

1. 在 BSC 主网通过官方 Safe 创建并部署新 Safe，地址记为 `<NEW_OWNER_SAFE>`。成员、threshold 经双人复核，确认每名成员能签名；建议延续至少 `2/3`。
2. 查 `eth_getCode` 非空、singleton/版本可信，核对模块/Guard/Handler、成员、threshold 和 nonce；已验证的新 Safe 必须能通过标准执行交易调用外部合约。
3. 记录新/旧 Safe 地址、应用适配版本、暂停/恢复方案、全部遗留 nonce。不要用普通 EOA 作为新 Owner。
4. 如维护窗口需要暂停，单独审批 `pause()`；暂停认购不会停止 Job 读链。只有确认交接方案要求时才暂停，不默认暂停生产项目。

### B2. 原 Owner Safe 发起移交

在**原 Owner Safe**中打开 Transaction Builder：

| 字段 | 必须填写 |
|---|---|
| 发起 Safe | `0x79c527B8Af5b1E27d5052f8dCe04D20733e72350`，以执行前 `owner()` 为准 |
| 目标 to | **V3 Proxy** `0x0D4040875742975269c55E16C8884d3Ee9510B8E` |
| ABI 文件 | `v3-governance-migration.abi.json` |
| 方法 / 参数 | `transferOwnership(newOwner)` / `<NEW_OWNER_SAFE>` |
| 内层 operation / value | CALL `0` / `0` BNB |

1. 使用原 Safe 的当前 nonce 建提案，当前有效门槛批准，模拟并执行。
2. receipt 的 V3 地址必须发出 `OwnershipTransferStarted(oldSafe,newSafe)`；Safe 必须是实际执行成功，不仅外层 receipt status=1。
3. 至少 15 次确认后读取：`owner()==OLD_SAFE`、`pendingOwner()==NEW_SAFE`。
4. 此时**原 Safe 仍拥有 Owner 权限**，新 Safe 尚未获得。不要删除原 Safe 成员、销毁旧密钥或宣称迁移完成。

### B3. 新 Safe 接受移交

在**新 Owner Safe**中打开 Transaction Builder：

| 字段 | 必须填写 |
|---|---|
| 发起 Safe | `<NEW_OWNER_SAFE>` |
| 目标 to | 同一个 **V3 Proxy** |
| 方法 | `acceptOwnership()`，无参数 |
| 内层 operation / value | CALL `0` / `0` BNB |
| calldata | `0x79ba5097`（同时用附件 ABI 独立编码核对） |

1. 使用**新 Safe 自己的 nonce**和门槛收集签名，复核 SafeTxHash，执行并等待确认。
2. 不能由新 Safe 的某个成员在 BscScan 直接连接个人钱包调用接受；V3 看到的 `msg.sender` 必须是新 Safe。
3. 核对 V3 `OwnershipTransferred(oldSafe,newSafe)`，以及 `owner()==NEW_SAFE`、`pendingOwner()==0x0`。
4. `operator()`、`whitelistGovernor()`、Implementation、各项目参数/收款快照不应变化。Owner 移交不会自动移交白名单 Governor 或更改 Fee Receiver。
5. 完成应用配套切换，验收第 8 节；需要恢复暂停时，只能由当前新 Owner Safe 执行单独批准的 `unpause()`。

**B2 与 B3 是两个不同 Safe 发起的链上步骤，不要在原 Safe 的一个普通批次里放 `transferOwnership + acceptOwnership`：第二步的 msg.sender 错误。**

### B4. 中断处理与回退

- B2 执行前：不执行提案即可保持原状态，但遗留签名仍要按 A3 处理。
- B2 成功、B3 未执行：原 Owner 可再调用 `transferOwnership(correctSafe)` 替换 pendingOwner；不要使用另一笔相同 nonce 广播覆盖已确认交易。
- 原 Owner 如需把 pending 指向自己，当前实现允许向自己这个**有代码的 Safe**调用 `transferOwnership(oldSafe)`，然后由该 Safe 调 `acceptOwnership()` 清空 pending；必须独立模拟，分步验收。
- **不得照抄 OpenZeppelin 通用教程用 `transferOwnership(0x0)` 取消。**当前 V3 override 要求 `newOwner.code.length>0`，零地址会 revert。`renounceOwnership()` 也被禁止。
- B3 成功后：原 Safe 无权单方面撤销；需新 Safe 再按同样两步方式移交回旧 Safe。若新 Safe 无法执行，不能承诺有后门恢复。

## 6. Governor Safe 的迁移边界

当前 V3 仅初始化时设置 `whitelistGovernor`，没有 `setWhitelistGovernor`。因此：

1. 常规团队交接：保留 Governor Safe 地址，执行方案 A，更换其中签名成员与门槛。
2. Owner 转到新 Safe 后，Governor 仍是旧 Governor Safe，需单独交接签名成员。
3. 必须更换 Governor 合约地址时，另行设计 UUPS 升级（存储兼容、权限、初始化/迁移、回退、应用信任更新及审计），经新授权后实施。本手册不提供“直接改存储”或通用升级 calldata。

## 7. Backend / Job 配套兼容门禁（方案 B 必做）

当前 Backend/Job 代码中 `pkg/common/idov3whitelist/approved_set_runtime.go` 固定检查：

- Owner `0x79c527b8af5b1e27d5052f8dce04d20733e72350`；
- Governor `0xc46659e67543aaaec553f7f3fc8f004b7dd286ef`；
- 对相应 Safe code hash 和 Proxy/Implementation 做独立信任检查。

`approved_set_record.go` 等也有 Governor 身份绑定。只改 YAML 中 Safe code hash，**不能替代硬编码地址变更**。

链上 Owner 地址改变后，若应用仍只信任旧地址，创建、治理审查、定期身份核验/投影或 freshness 等链路可能拒绝运行。不能宣称“只转 Owner，不影响平台”。

实施方案 B 前应：

1. 在隔离分支识别所有旧地址、owner/governor、pendingOwner、code hash、历史创建准入、Proof/策略/事件处理和缓存依赖。
2. 设计精确迁移信任策略：明确旧/新地址在相应历史区块和当前区块的合法范围、确认事件、切换条件和恢复路径；不是把身份校验删除或无限期允许任意地址。
3. 在隔离演练环境模拟 `OwnershipTransferStarted → OwnershipTransferred`，验证 Job 历史审计与实时扫块均可用，项目/源 freshness 持续更新，不误判 reorg。
4. 生成独立受控 Backend/Job 版本，明确发布顺序、维护窗口和回滚；未获部署授权不发布。新 Safe 字节码即使与旧版相同，也不代表地址校验会通过。
5. 迁移期间保留原 outbox/hash/nonce 记录，冻结无关签名操作；不通过手工改 DB 或 relayer 私钥绕过新权限。

方案 A 不改变合约角色地址，通常兼容性风险较低，但仍须检查应用是否另外绑定 Safe 成员/threshold，不能将“通常”当作无须验收。

## 8. 最终验收与回执

| 验收项 | 方案 A | 方案 B |
|---|---|---|
| Safe 成员和 threshold | 与批准的交接单一致 | 新 Safe 符合批准要求 |
| V3 owner / pendingOwner | 原 Owner / 无意外 pending | 新 Safe / 零地址 |
| Governor | 原地址，成员按单独方案交接 | 默认不变，成员另行核验 |
| Operator / Fee Receiver / 项目快照 / Implementation | 无未授权变化 | 无未授权变化 |
| Safe 执行结果 | receipt、ExecutionSuccess、成员事件/状态匹配 | 两步分别成功、Owner 事件和状态匹配 |
| Job / 项目游标 | 两次采样持续推进 | 两次采样持续推进，无新 halted |
| Projection / freshness / API | 对齐当前 epoch/revision、未失效 | 对齐当前 epoch/revision、未失效 |
| 原签名提案与密钥 | 处理遗留 nonce，不留可执行危险提案 | 原 Safe 退役前处理历史提案并验收 |

**仅外层交易 `status=1` 不足以证明 Safe 内层成功。**Safe 可出现外层成功但 `ExecutionFailure`，需同时核对 Safe 事件、V3 事件和读取后的最终状态。外层 `from` 可能是 gas 执行者，V3 实际授权调用者应是对应 Safe。

交接回执必须记录：chainId、Proxy、实施前后固定区块/hash、成员/threshold、Safe nonce、SafeTxHash、外层交易 hash、to/value/data/operation、模拟结果、签名人、确认数、事件、应用版本、两次游标采样和残余风险。不要记录私钥、助记词、RPC 凭据、JWT 或完整 signed raw transaction。

## 9. ABI 与离线编码附件

同目录附件：`ido-v3-safe-migration/`。

| 文件 | 用途 |
|---|---|
| `v3-governance-migration.abi.json` | V3 Proxy 的 Owner 两步迁移、角色 getter、Operator 修改及事件 |
| `safe-owner-management.abi.json` | Safe self-call 成员/门槛修改、查询与事件 |
| `V3-full.abi.json` | 从当前业务编译产物导出的完整 ABI，便于独立复核 |
| `migration-snapshot.json` | 本文当前快照；不是待签提案 |
| `encode-migration.cjs` | 纯离线生成/解码 calldata，不连接 RPC、不签名、不发送 |
| `selectors.json` | 方法 selector，供审核核对 |

### 9.1 核心接口（便于阅读，不替代 JSON ABI 文件）

```solidity
// 调用目标：V3 Proxy
function owner() external view returns (address);
function pendingOwner() external view returns (address);
function transferOwnership(address newOwner) external;
function acceptOwnership() external;
function operator() external view returns (address);
function setOperator(address newOperator) external;
function whitelistGovernor() external view returns (address);
event OwnershipTransferStarted(address indexed previousOwner, address indexed newOwner);
event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

// 调用目标：当前正在管理的 Safe 自己
function getOwners() external view returns (address[] memory);
function getThreshold() external view returns (uint256);
function nonce() external view returns (uint256);
function isOwner(address owner) external view returns (bool);
function swapOwner(address prevOwner,address oldOwner,address newOwner) external;
function addOwnerWithThreshold(address owner,uint256 threshold) external;
function removeOwner(address prevOwner,address owner,uint256 threshold) external;
function changeThreshold(uint256 threshold) external;
event AddedOwner(address owner);
event RemovedOwner(address owner);
event ChangedThreshold(uint256 threshold);
```

### 9.2 离线工具使用

工具需 Node.js 和 ethers v6。可把整个附件目录复制到已安装 ethers 的合约仓库根目录，再在该目录的 PowerShell 运行以下命令。尖括号为占位符，不是有效地址；在填写并复核前不要提交。

```powershell
# 只生成 calldata，不会联网或发送交易
node .\ido-v3-safe-migration\encode-migration.cjs v3 transferOwnership <NEW_OWNER_SAFE>
node .\ido-v3-safe-migration\encode-migration.cjs v3 acceptOwnership
node .\ido-v3-safe-migration\encode-migration.cjs safe swapOwner <PREV_OWNER> <OLD_SIGNER> <NEW_SIGNER>
node .\ido-v3-safe-migration\encode-migration.cjs safe changeThreshold 2

# 独立解码已有 calldata
node .\ido-v3-safe-migration\encode-migration.cjs decode v3 <CALLDATA_HEX>
node .\ido-v3-safe-migration\encode-migration.cjs decode safe <CALLDATA_HEX>
```

工具输出不含 nonce、gas 和签名，也不构成已验证的交易。执行前须把输出 data 与正确的发起 Safe、目标 to、value=0、operation=CALL 配套核对并模拟。不能向 Implementation 发这些业务管理调用。

## 10. 附录：仅迁移 Operator

如目标只是更换后端业务执行者，由**当前 Owner Safe**向 **V3 Proxy**调用 `setOperator(newOperator)`，内层 CALL/value=0；核对 `OperatorUpdated` 和 `operator()`。

此操作不会移交 Safe 管理权、Governor 或 Fee Receiver。当前应用有 Operator 身份及创建审批绑定，必须先设计配置/审批兼容；已有签名交易继续按原 hash/nonce 对账，不另签、不手工释放 nonce fence。不要把需要受控 EOA 签名的自动业务通道随意换成需要多签的 Safe 后仍期待后台自动执行。

## 11. 官方参考与相关文档

- Safe 管理成员：<https://help.safe.global/articles/9629761095-managing-safe-owners-and-signatures>
- Safe Transaction Builder：<https://help.safe.global/articles/4180673514-transaction-builder>
- Safe swapOwner：<https://docs.safe.global/reference-smart-account/owners/swapOwner>
- Safe addOwnerWithThreshold：<https://docs.safe.global/reference-smart-account/owners/addOwnerWithThreshold>
- Safe removeOwner：<https://docs.safe.global/reference-smart-account/owners/removeOwner>
- Safe changeThreshold：<https://docs.safe.global/reference-smart-account/owners/changeThreshold>
- 本地权限总览：`IDO-V3-ADDRESS-PERMISSIONS.md`
- 已验证 V3 Proxy：<https://bscscan.com/address/0x0D4040875742975269c55E16C8884d3Ee9510B8E#code>
- 已验证 Implementation：<https://bscscan.com/address/0x887fCC6E54Fc8229919166Ad90B16C19B4992560#code>

操作人员遇到目标地址、ABI、权限、nonce、模拟、回执或应用兼容任一不一致时，停止并记录具体差异，不根据本文旧快照继续写入。
