# JoinCare IDO 手工测试后修订记录

> 用途：记录手工测试过程中确认的问题及修订状态。2026-08-18已完成IDO-FU-001至IDO-FU-006修订、本地部署及自动/接口回归；现恢复人工测试，先执行六项浏览器冒烟，再从project_id=3的Buyer购买流程继续。
>
> 当前链上锚点：project_id=3 已由 durable v2_activate intent id=2 自动安全重试并激活，状态 Active；激活交易 0xa88d7ec0d2cfdd1a9d9662e71efb028360a267e6a5e21d0f241fe988bc9dd277，ProjectActivated位于block 22；SMRT deposited/reserved=10,000。不得重复激活或重建Hardhat。

## 2026-08-18修订部署与自动回归结果

- 本地数据库已备份至 `.local-test/backups/remediation-20260818_185320`，随后应用 `2026_08_31_restore_archived_project.sql`；迁移账本为41/41 applied、0 applying。
- Backend全量Go测试通过；Frontend 39/39单测与production build通过；Admin 24/24单测、typecheck与production build通过。
- Backend/Job/Frontend/Admin新制品已部署到本机测试环境；四个应用容器健康。Hardhat、MySQL、Redis未重启或重建。
- Hardhat容器ID、启动时间、restart count和block 22保持不变；project_id=3仍为Active，create/activate intent与交易哈希不变；chain-events lag=0、dead_letters=0。
- IDO-FU-001接口回归：project_id=2返回restore_allowed=false、restore_blocker=PROJECT_RESTORE_STATE_UNKNOWN；POST restore返回HTTP 409同名稳定错误码，项目状态保持archived。
- IDO-FU-002/004自动回归：本地31337登录策略不再自动切到97；chainChanged清理旧session，ensureSigner按chain/account/provider原子重建；每次写操作按project.chainId fail-closed。
- IDO-FU-003/006构建回归：Dashboard已复用JcFloatingNavbar、HomeFooter、PixelGrid及jc-shell；支持英文/繁中、亮/暗、移动/桌面布局，并显示真实balance、allowance、deposited、soft/hard required与shortfall，Approve exact和Deposit分步执行。
- IDO-FU-005自动回归：稳定码统一为PROJECT_TOKEN_RESERVE_INSUFFICIENT；Admin按decimals把raw值显示为可读金额并屏蔽durable、estimateGas、VM Exception等内部错误。
- 尚待人工浏览器冒烟：MetaMask 31337登录与切链、Dashboard语言/主题/375/768/1440布局、Approve/Deposit按钮状态，以及Admin中文交互。完成后可将六项改为“已关闭”。

## 缺陷状态说明

- 待修订：问题已确认，尚未实现修复；
- 修订中：正在修改代码；
- 待回归：已修改，尚未完成手工回归；
- 已关闭：修复和回归均通过。

---

## IDO-FU-001 项目禁用后无法恢复

| 字段 | 内容 |
|---|---|
| 发现日期 | 2026-08-18 |
| 发现阶段 | 本地 IDO V2 手工测试，项目 fixture 准入阶段 |
| 模块 | Admin 项目管理 / Backend Admin API |
| 严重程度 | Medium |
| 优先级 | P1（本轮测试后修订） |
| 当前状态 | 待回归（修订已部署，自动/接口回归通过） |
| 发现人 | 手工测试 |
| 受影响项目 | 未绑定链上的项目草稿；本次复现为 project_id=2 |

### 现象

管理员在项目列表点击“禁用”后，项目状态被修改为 archived，并从公开 Launchpad 列表中移除。Admin 页面没有“启用”“恢复”或“取消归档”按钮，Backend 也没有对应 restore/unarchive API，因此无法通过正常产品流程恢复。

### 复现步骤

1. 登录 Admin；
2. 进入项目列表；
3. 选择 contract_version=unassigned 且 onchain_locked=0 的项目；
4. 点击“禁用”并确认；
5. 刷新项目列表；
6. 项目状态变为 archived；
7. 检查操作区。

### 实际结果

- “禁用”按钮变为不可用；
- 没有“启用/恢复”按钮；
- 公开 Launchpad 不再返回该项目；
- 只能通过直接修改数据库恢复，不符合受控、可审计的管理流程。

### 预期结果

对允许恢复的未上链项目，Admin 应提供明确的“启用/恢复”操作；Backend 应通过带权限、事务、状态校验和审计记录的接口恢复项目。

### 已确认根因

Backend 只有：

    POST /admin/v1/projects/:id/disable

该接口直接把 status 更新为 archived。当前不存在 enable、restore 或 unarchive API。Admin 项目列表也只实现了 disableProject，没有恢复操作。

### 安全边界

- contract_version=v2_stablecoin 或 onchain_locked=1 的项目不得通过内容管理恢复/禁用；其状态必须由链上生命周期和事件同步决定；
- 只允许恢复未绑定链、没有不可变资金/交易事实的项目；
- 必须校验 chain_tx_submission、transaction_record、fund_ledger、ido_purchase_entitlement 等不可变事实；
- 必须要求 m2_project:write 权限并写入 Admin 审计日志；
- 不允许通过通用内容编辑接口任意修改链上项目状态。

### 建议修订

1. 新增受控接口：

       POST /admin/v1/projects/:id/restore

2. 保存项目禁用前状态，例如增加 archived_from_status，或写入专用、不可覆盖的状态变更审计记录；
3. 恢复时回到禁用前状态，而不是无条件设为 pre_launch；
4. Admin 对 archived 且满足恢复条件的项目显示“启用/恢复”按钮；
5. 确认弹窗展示项目 ID、原状态和目标状态；
6. 并发请求保持幂等，重复恢复不得产生状态漂移；
7. API 返回稳定错误码，区分无权限、已恢复、链上锁定和存在不可变事实。

### 修复验收条件

- pre_launch → archived → pre_launch 成功；
- warmup → archived → warmup 成功；
- candidate → archived → candidate 成功（若该状态允许禁用）；
- 重复 restore 幂等或返回明确冲突；
- 无权限管理员返回 403；
- V2/onchain_locked 项目无法 disable 或 restore；
- 存在交易/资金事实的项目无法恢复；
- Admin 审计记录包含操作者、项目 ID、原状态、目标状态、时间和结果；
- 恢复后公共 Launchpad 按目标状态正确展示；
- 不产生链上交易、资金账本或 chain event；
- Backend、Job 和 /health/chain-events 保持健康。

### 本轮测试现场状态

记录时数据库状态：

| project_id | status | contract_version | onchain_locked | 说明 |
|---:|---|---|---:|---|
| 1 | pre_launch | unassigned | 0 | 已从生产 staging 恢复原始 draft |
| 2 | archived | unassigned | 0 | 保持缺陷复现场景，暂不直接改库恢复 |
| 3 | Set | v2_stablecoin | 0 | 本轮 IDO 链上创建测试对象 |

### 关联源码

- Backend：pkg/core/admin/controller/project_content.go 的 DisableProject；
- Backend route：pkg/core/admin/route.go 的 /projects/:id/disable；
- Admin：apps/web-antd/src/views/joincare/projects/list.vue；
- Admin API：apps/web-antd/src/api/joincare/project_content.ts。

### 临时处理

本轮继续测试 project_id=3。project_id=2 保持 archived 作为复现证据，不直接修改数据库。修复后再执行禁用/恢复回归。

---

## IDO-FU-002 本地链登录被强制切换到 BSC Testnet

| 字段 | 内容 |
|---|---|
| 发现日期 | 2026-08-18 |
| 发现阶段 | Project Owner 登录 Dashboard，准备项目币充值 |
| 模块 | Frontend 钱包认证与网络选择 |
| 严重程度 | High |
| 优先级 | P0（本轮测试后优先修订） |
| 当前状态 | 待回归（修订已部署，自动/接口回归通过） |
| 受影响环境 | 本地 Hardhat 31337；其他非 56/97 的受控测试网络同样可能受影响 |

### 现象与复现

1. 钱包连接 JoinCare Local Hardhat，Chain ID=31337；
2. Frontend连接钱包阶段网络保持正确；
3. 执行 JoinCare 登录/注册签名；
4. 钱包被自动请求并切换到 BSC Testnet，Chain ID=97。

### 实际结果

认证流程改变用户当前网络，导致本地 IDO Dashboard、项目币 approve/deposit 和后续购买无法直接在项目配置的 31337 网络执行，并产生用户误在公共测试网签名或发送交易的风险。

### 已确认根因

wallet store 的 signAndSubmitAuth 将网络策略硬编码为：生产构建仅允许 56；其他构建允许 56/97，并以 97 为目标网络。逻辑没有使用后端下发的 chainId、当前项目 chainId 或本地构建允许的 31337。

### 临时安全规则

- 在 BSC Testnet 上不得签署项目币 approve、deposit、purchase 或其他资金交易；
- 发现自动切链后立即停止交易流程；
- 只有在确认登录 token 已建立、手工切回 31337 且链上写入 guard再次校验项目 chainId时，才允许继续；
- 如果写入层不能 fail-closed校验 31337，本轮 Dashboard资金测试记为 BLOCKED。

### 建议修订

1. 登录签名不应无条件改变网络；若登录消息不依赖链，允许在当前受信网络签名；
2. 如必须绑定网络，应使用后端下发并经构建允许列表校验的 chainId；
3. 本地构建显式允许 31337，生产构建仍严格只允许 56；
4. IDO链上写操作必须以项目 chain_id 为准，并在交易前展示目标网络、合约和 token；
5. 连接、登录和每次写操作分别执行 fail-closed 网络校验，不能依赖之前的网络状态；
6. 切链失败或用户拒绝时不得进入“已连接但链错误”的半状态。

### 修复验收条件

- 本地 31337 登录不切到 97；
- 生产构建仍拒绝 97/31337，只接受 56；
- BSC Testnet构建只按明确配置接受 97；
- Project Owner approve/deposit前目标链必须等于项目 chain_id；
- Buyer purchase/claim/refund同样校验项目 chain_id；
- 错误链时不发送交易，并给出目标 Chain ID；
- 切换账户、刷新页面和重新登录后策略一致；
- WalletConnect、MetaMask等连接方式行为一致。

### 关联源码

- Frontend：src/stores/wallet.js 的 signAndSubmitAuth；
- Frontend：src/config/web3modal.js 的网络配置；
- Frontend：src/services/contract.js 的 _ensureProjectSigner。

---

## IDO-FU-003 Dashboard 项目充值缺少余额与 Required Deposit 信息

| 字段 | 内容 |
|---|---|
| 发现日期 | 2026-08-18 |
| 发现阶段 | Project Owner 为 Pending 项目准备 SMRT 充值 |
| 模块 | Frontend Dashboard / My Projects |
| 严重程度 | Medium |
| 优先级 | P1（本轮测试后修订） |
| 当前状态 | 待回归（修订已部署，自动/接口回归通过） |

### 现象

Dashboard 能识别 project_id=3，状态为 Pending，并提供 Deposit 输入和按钮，但没有显示项目方当前 SMRT 余额，也没有显示覆盖 Soft Cap/Hard Cap 所需的 Required Deposit。页面没有独立 Approve 按钮；Deposit 实际执行“最大额度授权 + 指定数量充值”两笔交易。

### 风险

- 项目方无法在提交前判断余额是否充足；
- 无法直观看出应至少存入多少项目币才能激活、多少项目币才能覆盖 Hard Cap；
- 一键按钮没有充分说明将连续请求两笔交易以及第一笔是无限授权；
- 容易因误填数量导致激活失败或后续购买 reserve 不足。

### 建议修订

1. 显示项目币钱包余额、当前 allowance、已充值量；
2. 显示覆盖 Soft Cap 和 Hard Cap 的项目币数量；
3. 显示还需补充的差额；
4. 将 Approve 与 Deposit 分步展示，或在一键流程前明确说明两笔交易；
5. 明确提示 Max Approval，并允许选择精确授权；
6. 每笔交易分别展示 pending/success/failure 和 tx hash；
7. 第一笔成功、第二笔失败时支持安全重试，不重复要求授权。

### 修复验收条件

- 正确显示 SMRT 余额 20,000；
- Soft Cap Required=5,000 SMRT，Hard Cap Required=10,000 SMRT；
- 输入 4,000 时提示低于 Soft Cap但允许负向测试或需明确确认；
- 授权对象显示本地 IDO V2 Proxy；
- 授权 token必须是 SMRT，不得是 mUSDT；
- 两笔交易状态与链上余额、allowance、deposit一致；
- 错误网络时在任何钱包交易弹窗前 fail-closed。

---

## IDO-FU-004 手工切回项目链后 Dashboard Deposit 仍报 Network error

| 字段 | 内容 |
|---|---|
| 发现日期 | 2026-08-18 |
| 发现阶段 | TC-FUND-001，Project Owner输入4,000 SMRT后点击Deposit |
| 模块 | Frontend Web3 provider/signer生命周期与错误展示 |
| 严重程度 | High |
| 优先级 | P0（本轮测试后优先修订） |
| 当前状态 | 待回归（修订已部署，自动/接口回归通过） |

### 复现步骤

1. 在31337连接并登录；
2. 登录流程因IDO-FU-002切到97；
3. 手工切回31337；
4. 登录状态保持，Dashboard识别Pending项目3；
5. 输入4,000并点击Deposit。

### 实际结果

钱包弹窗出现前页面显示“Deposit Failed / Network error. Please check your connection.”；没有Approve或Deposit交易。链上确认区块未增加、SMRT余额20,000、allowance=0、deposited=0。

### 技术原因范围

RPC与CORS均正常，项目chain_id=31337正确。Frontend Web3Service缓存provider/signer；ensureSigner在signer非空时直接返回，不核对当前EIP-1193 chainId，也不在chainChanged后原子重建provider/signer。Ethers网络变化异常被friendlyError统一映射为笼统Network error，真实错误和expected/connected chain被隐藏。

### 建议修订

1. chainChanged时清除并重建BrowserProvider和signer；
2. ensureSigner每次写操作都从EIP-1193读取eth_chainId和eth_accounts，不因signer非空跳过校验；
3. provider重建后再执行项目chain_id fail-closed检查；
4. 对NETWORK_ERROR显示expected/connected Chain ID，不映射成普通连接错误；
5. 网络变化期间禁用写按钮，避免并发使用旧signer；
6. Approve与Deposit分步持久化状态，支持第一笔成功、第二笔失败后的恢复。

### 验收条件

- 97切回31337后无需重登录即可正常弹出SMRT Approve；
- 钱包弹窗前校验项目chain_id=31337；
- provider/signer对应当前账户和当前链；
- 真实RPC断线与chain mismatch显示不同错误；
- 本次失败场景不产生区块、allowance或deposit变化；
- MetaMask、OKX与WalletConnect均回归。

### 本轮处理

Dashboard项目币充值UI用例记为BLOCKED。为继续验证后端和合约生命周期，由本地环境管理员模拟同一Project Owner地址完成4,000 SMRT approve+deposit；该动作只推进fixture，不计入Frontend UI通过。

---

## IDO-FU-005 Admin激活操作未中文化且直接暴露技术错误

| 字段 | 内容 |
|---|---|
| 发现日期 | 2026-08-18 |
| 发现阶段 | TC-FUND-001，4,000 SMRT低于激活最低5,000时尝试激活 |
| 模块 | Admin项目列表 / Backend V2 activation错误映射 |
| 严重程度 | Medium |
| 优先级 | P1（本轮测试后修订） |
| 当前状态 | 待回归（修订已部署，自动/接口回归通过） |

### 现象

Admin项目列表中的激活按钮和确认弹窗使用英文；激活失败toast直接显示：

    V2 activation durable dispatch incomplete; tx_hash=: estimate gas failed: Error: VM Exception while processing transaction: reverted with custom error 'InsufficientProjectTokens()'

### 预期

中文后台应使用一致中文文案；业务可预检错误应在提交前给出可操作提示，不应向管理员暴露RPC、gas estimation、durable dispatch和Solidity custom error原文。

建议中文：

- 按钮：激活链上项目；
- 弹窗标题：确认激活链上项目；
- 弹窗说明：激活后项目将按链上时间开放认购，请确认项目代币已足额存入；
- 余额不足：项目代币不足，当前已存入4,000 SMRT，激活至少需要5,000 SMRT，尚缺1,000 SMRT；
- 成功：链上激活请求已提交；
- 失败：项目激活失败，请根据提示处理后重试。

### 建议修订

1. Admin按钮、弹窗、toast全部接入中文i18n；
2. Backend在建立或dispatch intent前执行只读激活预检或返回稳定业务错误码；
3. 将InsufficientProjectTokens映射为PROJECT_TOKEN_RESERVE_INSUFFICIENT；
4. 返回current_deposit_raw、required_deposit_raw、shortfall_raw、token_symbol和decimals；
5. Admin按decimals显示人类可读值；
6. RPC/estimateGas内部错误只写受控日志，不直接回显；
7. 无tx hash时不显示空的tx_hash=；
8. 保留ready intent重试时必须幂等，不新增重复intent。

### 本次负向测试结果

- v2_activate intent id=2，state=ready，attempt_no=1；
- tx_hash为空，未广播；
- receipt不存在；
- ProjectActivated事件为0；
- 项目状态保持Pending；
- totalTokensDeposited/reserved保持4,000 SMRT；
- /health/chain-events保持HTTP 200。

### 验收条件

- 中文环境无英文操作文案；
- 4,000/5,000场景显示准确短缺1,000 SMRT；
- 不暴露VM Exception、estimate gas、custom error或durable内部状态；
- 失败不广播、不改变状态、不重复建立intent；
- 补足后可复用或安全重试并成功激活；
- 英文环境仍显示完整、自然的英文业务提示。

---

## IDO-FU-006 Dashboard与全站视觉和交互规范不一致

| 字段 | 内容 |
|---|---|
| 发现日期 | 2026-08-18 |
| 发现阶段 | Project Owner Dashboard项目充值测试 |
| 模块 | Frontend Dashboard |
| 严重程度 | High |
| 优先级 | P0（继续人工测试前完成） |
| 当前状态 | 待回归（修订已部署，自动/接口回归通过） |

### 现象

Dashboard页面的布局、导航、色彩、组件、信息层级和交互反馈与站内其他页面明显不一致；英文/繁体中文切换、亮色/暗色切换等全局能力未按/tasks页面的实现和体验统一。项目方资金操作区域还缺少余额、Required Deposit、allowance和两步交易状态。

### 目标

以 http://127.0.0.1:13000/tasks 的页面风格和全局交互为基准重新设计Dashboard，而非局部换色。统一：

- 全局浮动导航和页脚；
- 页面画布、内容宽度、留白、卡片圆角、边框和阴影；
- 英文/繁体中文切换；
- 亮色/暗色切换；
- 桌面端和移动端响应式布局；
- 加载、空状态、错误、成功和交易pending反馈；
- 钱包地址、网络、代币、金额和项目状态的可读展示；
- 项目方与普通用户功能的信息架构。

### 功能要求

1. Dashboard所有用户可见文案进入统一i18n，不混用简体、繁体和英文硬编码；
2. 使用全局主题状态，dark/light下文字、背景、边框和图表均可读；
3. 项目方卡片显示项目币余额、allowance、已存入、Soft Cap Required、Hard Cap Required和差额；
4. Approve与Deposit分步、逐笔展示tx状态和hash；
5. claim/refund/withdraw等资金操作采用一致确认和错误组件；
6. 网络不匹配时显示expected/connected Chain ID并禁用资金按钮；
7. 保留现有业务数据源和安全校验，不以静态mock替代；
8. /tasks、/launchpad和Dashboard之间导航及主题/语言状态保持一致。

### 验收条件

- Dashboard与/tasks在导航、画布、主题和语言体验上统一；
- EN/繁中切换后全页面无遗漏硬编码；
- dark/light切换即时生效且刷新后行为与全站一致；
- 375px、768px、1440px关键视口无横向溢出；
- Project Owner可清晰完成Approve、Deposit并查看Required Deposit；
- Buyer可清晰查看购买、claim和refund状态；
- provider/chain变化后按钮状态和错误提示准确；
- 现有Backend/Job/合约安全门禁和资金精度不回归。
