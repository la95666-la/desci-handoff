# JoinCare 本地 IDO V2 手工测试指南

> 适用对象：测试工程师、产品验收人员、测试环境管理员  
> 适用环境：JoinCare WSL 本机隔离测试环境  
> 文档性质：手工测试参考流程，不是生产部署或链上运维手册
>
> **修订记录（2026-08-19 晚）**：按实际测试进度与本轮新增能力修订——项目 3 已走完「失败+退款」全链路（非原计划的取消分支）；新增 Admin 结算/取消/全局暂停入口与 C 端退款入口；账本页改下拉选项目；钱包详情新增实时退款状态；详见各节内【2026-08-19 修订】标注。

## 1. 测试目标

本手册用于验证 JoinCare IDO V2 从项目资料、链上创建、项目币托管、激活、用户认购到成功/失败/取消分支的业务表现，并核对 Frontend、Admin、Backend、Job、MySQL、Redis 与 Hardhat 链之间的一致性。

测试重点：

- 数据库项目与链上 projectId 正确映射；
- 6 位和 18 位支付币的金额换算正确；
- 项目币 reserve 不跨项目、不透支；
- 项目生命周期和角色权限正确；
- 成功项目可领取并提取，失败/取消项目可退款；
- 链事件同步无 dead-letter、无滞后；
- 页面显示值、数据库 raw 值和合约状态一致；
- 错误操作 fail-closed，不通过手工改库绕过。

## 2. 当前测试环境

### 2.1 访问地址

| 模块 | 地址 |
|---|---|
| Frontend | http://127.0.0.1:13000 |
| Launchpad | http://127.0.0.1:13000/launchpad |
| Admin | http://127.0.0.1:13001 |
| Backend health | http://127.0.0.1:13004/health |
| Job health | http://127.0.0.1:13007/health |
| 链事件深度健康 | http://127.0.0.1:13007/health/chain-events |

Admin 测试登录：公开快照不附带预置管理员账号或密码。请先在隔离的本地测试环境使用自己的管理员账号和独立凭据，再登录验证。

### 2.2 本地链

| 项目 | 值 |
|---|---|
| 网络 | Hardhat 本地链 |
| Chain ID | 31337 |
| IDO V2 Proxy | 0x5FC8d32690cc91D4c39d9d3abcBD16989F875707 |
| IDO V2 Implementation | 0xDc64a140Aa3E981100a9becA4E685f962f0cF6C9 |
| Mock USDT | 0x9fE46736679d2D9a65F0992F2272dE9f3c7fa6e0，6 decimals |
| Mock USDC | 0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9，18 decimals |

Hardhat 是内存链。禁止删除或重新创建 jc-local-hardhat 容器，否则合约和交易全部消失。使用项目提供的启停脚本，不要直接执行 docker compose down。

### 2.3 当前数据库项目

生产快照包含 3 个项目，业务 project_id 为 1、2、3。

【2026-08-19 修订】当前实际状态：

- **project 3**：已上链（V2 + USDT 6 decimals）、已激活、已完成认购与失败结算（400 < soft 500 → Failed）、Buyer 已全额退款；可作为「失败+退款」分支的现成参照；
- **project 1 / 2**：仍为 pre_launch（contract_version = unassigned，未上链），可继续按 §6.1 策略准备成功/失败分支；
- 合约 owner 已转移为 relayer（Hardhat account 1，0x7099...79C8，两步式 transferOwnership+acceptOwnership），owner-only 操作（pause/unpause/cancel Active 项目/平台费提取）由 relayer 私钥执行。

新项目上链前，测试工程师不得通过 SQL 把 unassigned 改成 v2。必须先由环境管理员完成“项目 fixture 准备”。

## 3. 角色与职责

| 角色 | 建议账户 | 允许操作 |
|---|---|---|
| 平台 Owner（合约层） | **已转移至 Hardhat account 1（relayer）** | 治理、配置项目代币、取消 Active 项目、全局暂停/恢复、平台费用提取 |
| Operator / Job signer | Hardhat account 1 | 创建、激活、结束结算、取消项目和受控平台操作 |
| Fee Receiver | Hardhat account 2 | 接收平台成功费 |
| Project Owner 1 | Hardhat account 3 | 项目 1 充值项目币、提取项目收益/剩余币 |
| Project Owner 2 | Hardhat account 4 | 项目 2 充值项目币、提取剩余币 |
| Project Owner 3 | Hardhat account 5 | 项目 3 充值项目币、提取剩余币 |
| Buyer A | Hardhat account 6 | USDT 认购、领取/退款 |
| Buyer B | Hardhat account 7 | USDT 认购、边界测试 |
| Buyer C | Hardhat account 8 | USDC 认购、领取/退款 |
| 未授权用户 | Hardhat account 9 | 权限负向测试 |

测试私钥由测试环境管理员通过安全渠道提供或导入测试钱包。本手册不得记录私钥、助记词或生产钱包凭据。

## 4. 执行渠道约定

每条用例标注执行渠道：

- UI：测试工程师通过 Frontend/Admin/钱包完成；
- 环境管理员：部署 token、分发测试币、推进 Hardhat 时间等环境动作；
- 观察项：测试工程师通过页面、Admin 或健康接口核验；
- 产品缺口：当前 UI 未暴露的能力，记录缺口，不要求测试工程师自行运行合约脚本。

禁止：

- 手工 UPDATE projects.onchain_locked；
- 手工伪造 chain_event_record、fund_ledger 或 entitlement；
- 删除 dead-letter 以制造健康假象；
- 使用生产 RPC、生产私钥或生产稳定币；
- 将本地地址或交易误记为生产证据。

## 5. 测试前环境准入

### TC-ENV-001 服务健康

步骤：

1. 在 joincare-deploy 目录执行：

       .local-test/jc-local.sh status
       .local-test/jc-local.sh smoke

2. 浏览器访问 Backend、Job 和链事件健康地址。

预期：

- MySQL、Redis、Hardhat、Backend、Job 均 healthy；
- Frontend/Admin 可打开；
- /health/chain-events 返回 HTTP 200；
- status=true；
- dead_letters=0；
- blocked_chain_transactions=0；
- lag_blocks=0；
- last_failure_at=null。

失败处理：任一项失败即停止 IDO 测试，保存健康响应和容器日志，通知环境管理员。

### TC-ENV-002 钱包网络

步骤：

1. 将测试钱包连接到环境管理员提供的 Hardhat RPC；
2. 检查钱包 Chain ID；
3. 检查当前账户与用例角色相符。

预期：

- Chain ID 为 31337；
- 页面未提示错误网络；
- 钱包地址与角色表一致；
- 钱包有足够本地 ETH 支付 gas。

注意：Hardhat RPC 当前未映射宿主机端口。如果浏览器钱包无法访问 RPC，这是环境接入问题，不是 IDO 业务失败。由环境管理员提供浏览器可达但仍隔离的 RPC 接入方式后再测。

### TC-ENV-003 项目 fixture 准入

在 Admin 项目列表核对 project_id 1、2、3。

正式执行生命周期测试前，应满足：

| 字段 | 预期 |
|---|---|
| contract_version | v2_stablecoin 或当前约定的 V2 值 |
| chain_id | 31337 |
| contract_address | 本地 IDO V2 Proxy |
| token_address | 每个项目独立的本地项目币地址 |
| payment_token_address | 本地 Mock USDT 或 Mock USDC |
| onchain_locked | 1 |
| status | Pending 或 Active，取决于准备阶段 |

若仍为 unassigned/onchain_locked=0，停止测试并执行第 6 节的环境准备，不得手改数据库。

## 6. 环境管理员前置准备

本节由测试环境管理员执行，测试工程师只验收结果。

### 6.1 三项目测试策略

| project_id | 测试分支 | 支付币 | 目标结果 |
|---:|---|---|---|
| 1 | 成功募资 | Mock USDT，6 decimals | Successful、claim、收益/费用提取 |
| 2 | 低于软顶 | Mock USDC，18 decimals | Failed、refund |
| 3 | 募资中取消 | Mock USDT，6 decimals | Cancelled、refund |

【2026-08-19 修订】实际执行中 project 3 被用于「成功上链→认购→未达软顶结算→退款」链路验证（已终态 Failed，用户已退款 400 USDT）。剩余可测：

- 用 project 3 补测「取消分支」已不可行（终态不可逆）；取消分支需重开项目或复用 project 1/2 重新排期；
- 建议后续：project 1 走成功分支、project 2 走 USDC 失败分支、另选/新项目走取消分支。

### 6.2 部署项目代币

每个项目部署独立 TestProjectToken：

- 18 decimals；
- owner 分别为 Project Owner 1/2/3；
- 初始供应量足够覆盖 hard cap 对应项目币；
- maxSupply 有明确上限；
- 三个项目不得共用同一项目币。

平台 Owner 对三个地址分别执行 configureProjectToken(token, true)。

验收证据：

- token 地址；
- deployment tx hash；
- owner；
- decimals；
- totalSupply/maxSupply；
- configureProjectToken tx hash。

### 6.3 推荐短周期参数

参数由环境管理员按测试窗口生成，不应使用生产项目原始日期。

建议：

- startTime = 准备完成后的未来 5～10 分钟；
- endTime = startTime 后 30～60 分钟；
- vestingTime = endTime 后 5～10 分钟；
- vestingPercent = 10000，一次性解锁；
- successFeeRate = 当前合约值 250；
- expectedFeeReceiver = 当前合约 feeReceiver；
- Project 1/3 使用 USDT；Project 2 使用 USDC。

为了便于手测，建议数值：

| 参数 | USDT 项目示例 | USDC 项目示例 |
|---|---:|---:|
| token price | 0.1 USDT | 0.1 USDC |
| hard cap | 1,000 | 1,000 |
| soft cap | 500 | 500 |
| max per user | 400 | 400 |
| 项目币需求（覆盖 hard cap） | 10,000 tokens | 10,000 tokens |

实际 raw 值由系统按 decimals 换算，不允许测试工程师手工猜 raw 值写库。

### 6.4 创建、充值、激活

1. Admin 使用“创建链上项目”；
2. 等待交易完成和 Job 同步；
3. 项目方 approve 项目币给 IDO proxy；
4. 项目方 depositProjectTokens，数量覆盖 hard cap；
5. Admin 使用“激活项目”；
6. 等待状态变为 Active。

如果当前 UI 没有项目币 approve/deposit 按钮，由环境管理员通过受控 fixture 工具完成，并把它记录为“环境准备”，不得冒充 UI 测试通过。

### 6.5 分发测试支付币

- Buyer A/B：分发 Mock USDT；
- Buyer C：分发 Mock USDC；
- 每个 buyer 的余额大于 maxPerUser；
- 保存 mint tx hash 和发放前后余额。

## 7. 项目资料与链上映射测试

### TC-MAP-001 项目 1 创建链上项目

前置：项目 1 为 unassigned，项目币已部署并允许，时间参数有效。

步骤：

1. Admin 登录；
2. 进入项目列表；
3. 找到 project_id=1；
4. 点击“创建链上项目”；
5. 阅读确认框，确认 projectId、合约、token、payment token、费用快照；
6. 确认执行；
7. 等待页面成功提示；
8. 刷新列表和详情。

预期：

- 只创建一次；
- contract_version 变为 V2；
- chain_id=31337；
- contract_address 等于本地 proxy；
- project token/payment token 地址正确；
- onchain_locked=1；
- 状态变为 Pending；
- chain_event_record 有 ProjectCreated；
- Job 深度健康仍为 HTTP 200。

证据：操作前后截图、API响应、交易哈希、区块号、Admin 审计记录。

### TC-MAP-002 重复创建

步骤：对已创建的同一 project_id 再点击创建。

预期：

- UI 按钮隐藏/禁用，或 Backend 拒绝；
- 不产生第二个 ProjectCreated；
- 不覆盖 token、owner、fee、cap、time；
- 项目仍保持唯一链上身份。

### TC-MAP-003 非授权管理员

使用没有 m2_project:execute 权限的管理员访问项目列表。

预期：

- 不显示执行按钮或请求返回 403；
- 不发送链上交易；
- Admin 审计中可识别拒绝事件。

## 8. 项目币充值与激活测试

### TC-FUND-001 充值不足软顶

由 Project Owner 1 先充值低于 soft cap 所需的项目币，再尝试激活。

预期：

- 激活失败；
- 状态仍 Pending；
- 错误明确指向 reserve 不足；
- 不产生 ProjectActivated；
- 已充值余额仍归属于项目 1。

### TC-FUND-002 补足并激活

步骤：

1. 追加充值，使 reserve 至少覆盖 hard cap；
2. Admin 点击激活；
3. 刷新页面。

预期：

- 激活成功；
- 状态 Active；
- ProjectActivated 被 Job 同步；
- deposit 总量与合约余额/ledger一致；
- 项目 2、3 reserve 不变化。

### TC-FUND-003 非项目方充值

用 Buyer 或其他 Project Owner 调用充值入口。

预期：交易被拒绝，不改变 reserve。

### TC-FUND-004 异常代币

仅在环境管理员提供异常 ERC-20 fixture 时执行：fee-on-transfer、decimals 漂移或未允许 token。

预期：配置、充值、创建或激活 fail-closed；不得出现“交易成功但账本少到账”。

## 9. 认购通用测试

### TC-BUY-001 钱包未连接

打开 /launchpad/1，不连接钱包。

预期：显示 Connect Wallet；不能直接购买。

### TC-BUY-002 错误网络

连接非 31337 网络。

预期：提示切换网络；不发送 purchase。

### TC-BUY-003 支付币授权

步骤：

1. Buyer A 连接钱包；
2. 输入有效金额；
3. 点击“授权支付币”；
4. 钱包确认；
5. 等待确认并刷新 allowance。

预期：

- 授权对象为本地 IDO proxy；
- token 为项目配置的 payment token；
- 页面显示 Approval Successful；
- 支付币余额不因 approve 减少。

安全观察：当前 UI 可能使用 max approval。记录实际 allowance；若产品要求最小授权，应单独提缺陷。

### TC-BUY-004 正常 USDT 购买

使用 Buyer A 对项目 1 购买 100 USDT。

预期：

- purchase 成功；
- Buyer USDT 减少 100；
- 合约收到 100；
- 项目 totalRaised 增加 100；
- entitlement 按 0.1 价格增加 1,000 项目币；
- 页面显示值与 6-decimal raw 换算一致；
- participant_count 首次购买加 1；
- Purchased 事件、fund ledger、entitlement 均存在。

### TC-BUY-005 同一用户再次购买

Buyer A 再购买 50 USDT。

预期：

- totalRaised 累加为 150；
- Buyer entitlement 累加；
- participant_count 不重复加；
- 每用户累计额度正确。

### TC-BUY-006 USDC 18-decimal 购买

Buyer C 对项目 2 购买 100 USDC。

预期：显示金额仍为 100，raw 为 100 × 10^18；不能按 6 decimals 处理。

### TC-BUY-007 零、负数和非数字

依次输入：空、0、-1、字母、超长小数、科学计数形式。

预期：

- UI阻止提交或 Backend/合约拒绝；
- 不发送无意义交易；
- 无 Purchased 事件；
- 页面无 NaN/Infinity。

### TC-BUY-008 超过钱包余额

输入大于 payment token balance 的金额。

预期：显示余额不足，purchase 不可执行或交易失败且状态不变。

### TC-BUY-009 超过 maxPerUser

累计购买超过 maxPerUser。

预期：交易被拒绝；之前成功记录不回滚、不重复。

### TC-BUY-010 超过 hard cap

将项目推近 hard cap，再尝试使总额超过 hard cap。

预期：整笔拒绝，不允许部分成交，totalRaised 不超过 hard cap。

### TC-BUY-011 时间边界

分别在 startTime 前、startTime、endTime 前一秒、endTime执行购买。

预期：

| 时间 | 结果 |
|---|---|
| startTime 前 | 拒绝 |
| startTime | 允许 |
| endTime 前一秒 | 允许 |
| endTime | 拒绝 |

推进 Hardhat 时间由环境管理员执行，并保存推进前后区块时间。

### TC-BUY-012 多项目资金隔离

在项目 1 和项目 3 分别购买相同 payment token。

预期：

- totalRaised、reserve、participant、entitlement 按 projectId 隔离；
- 项目 1 的购买不能消耗项目 3 项目币；
- fund ledger 的 project_id 正确。

## 10. 项目 1：成功分支

### TC-SUCCESS-001 达到软顶并结束

1. 多个 Buyer 购买，使 totalRaised >= softCap；
2. 环境管理员推进到 endTime；
3. 执行结束/finalize入口，或等待 Job 处理约定流程；
4. 刷新项目。

预期：状态 Successful；成功费按 250 basis points计算；claimable entitlement 固定；不能再购买或退款。

### TC-SUCCESS-002 vesting 前领取

在 vestingTime 前尝试 claim。

预期：不可领取或可领取数量为 0；不改变 claimed amount。

### TC-SUCCESS-003 vesting 后领取

推进到 vestingTime 后，Buyer A 领取。

预期：

- Buyer 项目币余额增加；
- claimed amount 增加且不超过 entitlement；
- claim 事件和 ledger一致；
- 重复点击不能重复领取同一份额度。

如果当前 Frontend 没有 claim UI，记录产品缺口，由环境管理员使用受控测试入口验证合约，不将其计为 UI PASS。

### TC-SUCCESS-004 项目方提取收益

Project Owner 1 提取 raised funds。

预期：只能项目方提取净收益；金额 = totalRaised - platform fee；重复提取被拒绝。

### TC-SUCCESS-005 平台提取费用

平台 Fee Receiver/允许角色提取成功费。

预期：收款地址、费率和金额与项目创建时快照一致；治理后修改当前 fee 不应改写既有项目快照。

### TC-SUCCESS-006 提取剩余项目币

项目方提取未售项目币。

预期：不能动用仍需覆盖未领取 entitlement 的 reserve；提取后用户 claim仍能全额完成。

## 11. 项目 2：失败分支

【2026-08-19 修订】本分支已在 project 3（USDT）完整验证通过，可直接参照其证据；USDC 18-decimal 版本待 project 2 补测。

### TC-FAIL-001 低于软顶结束

1. 项目 2 只购买少量 USDC；
2. 保证 totalRaised < softCap；
3. 推进到 endTime 并结束。

结束方式（新增 UI 入口）：Admin → 项目列表 → 对「Active 且已过 end_time」的项目点击红色「**结束结算**」按钮（对应 `POST /admin/v1/projects/:id/end`，durable 提交合约 `endProject`）；合约按 totalRaised 与 softCap 自动判定 Successful/Failed。

预期：状态 Failed；项目方不能提取募资款；Buyer 可退款；不能 claim 项目币。

### TC-FAIL-002 用户退款

【2026-08-19 修订】退款入口已上线，两条路径任选：

- **C 端详情页**：项目面板在失败/取消态且用户有未退认购款时显示「**退款**」按钮（前端直连签名调合约 `refund`，msg.sender 即退款对象）；
- **Dashboard**：「可退款项目」区块（按项目聚合）点「退款」；购买历史不再逐笔显示退款按钮（退款是按用户+项目聚合一次性退全款，逐笔按钮会导致 AlreadyRefunded 误报，已修复）。

注意：合约 refund 按（用户, 项目）聚合一次性退全部 paymentAmount（例：4 笔购买 30+50+200+120=400，一次退 400）；第二次 refund 被合约 `AlreadyRefunded` 拒绝。

预期：

- USDC 精确退回（USDT 同理）；
- refunded amount 正确；
- 第二次 refund 被拒绝；
- total fund ledger 守恒；
- 18-decimal 显示与 raw 一致；
- Dashboard 购买历史显示「项目状态」与「退款状态」两列（可退款/已退款），刷新后与链上一致。

### TC-FAIL-003 项目方取回项目币

入口：项目方在 Dashboard「My Projects」点「取回项目币」（已有功能，无需新增）。

预期：失败后项目方可取回全部充值项目币；用户无 claimable reserve。

## 12. 项目 3：取消分支与中途管控

【2026-08-19 修订】取消与暂停均已产品化为 Admin 入口；project 3 已终态，取消分支需另择项目。

### TC-CANCEL-001 Active 后取消

1. 激活目标项目；
2. Buyer 完成一笔购买；
3. Admin → 项目列表 → 对「Pending/Active + 已锁定 + 无在途提交」的项目点击红色「**取消项目**」按钮（对应 `POST /admin/v1/projects/:id/cancel`）；
4. 阅读确认弹窗（不可逆警告：用户可退款、项目方可取回项目币、认购终止）后确认。

预期：状态 Cancelled；停止新购买；Buyer 可退款（C 端退款按钮对 cancelled 态同样生效）；不能 claim；项目方可按规则取回项目币。

### TC-CANCEL-002 非授权取消

由无 m2_project:execute 权限的管理员、Operator 以外的账户尝试取消。

预期：非授权角色失败（403 或按钮不可见）；状态不变；不产生取消事件；Admin 审计可识别。

### TC-CANCEL-003 暂停不能冻结退出

【2026-08-19 修订】暂停已产品化：Admin 项目列表右上角「**全局暂停认购**」/「**恢复认购**」按钮（`POST /admin/v1/ido/pause|unpause`，需 owner=relayer，均弹确认框）。

1. 点击「全局暂停认购」并确认；
2. 尝试新购买；
3. 对已终态项目执行退款/领取/提款。

预期：暂停期间新购买/激活/项目币充值被合约 `EnforcedPausable` 拒绝；refund/claim/withdraw 不受影响；恢复认购后新购买立即可用；链上 `paused()` 与按钮状态一致。

注：pause 是**合约全局**的（影响所有项目），确认弹窗有明确提示。

### TC-LIFECYCLE-004 结算触发（新增）

前置：项目 Active 且已过 endTime。

步骤：Admin 点击「结束结算」→ 确认 → 等待链同步。

预期：raised≥softCap → Successful；raised<softCap → Failed；重复点击被拒（合约要求 Active）；chain_tx_submission 推进到 applied 且活跃槽释放（否则后续审计任务会被 unapplied 记录阻塞）；ProjectEnded 链事件落库。

## 13. Admin 与账本核验

每完成一笔交易，在 Admin 核对：

- 项目状态和累计金额；
- fund ledger；
- raw/display amount；
- payment token symbol/address/decimals；
- project token amount；
- entitlement；
- tx hash、block number、log index；
- contract version = V2；
- project_id 正确；
- 同一链事件没有重复入账。

【2026-08-19 修订】入口变化：

- 三个资金账本页（用户认购及权益及逐笔交易账本 / 对账检查 / 审计）已从手输 project_id 改为**可搜索下拉选择**（`#id · 项目名 · 状态`）；
- 「逐笔交易账本」页点钱包可看详情：新增**实时退款状态**（预期退款=项目终态且有认购；实际退款=账本 payment_refunded，含格式化金额）与**项目状态**；审计快照区金额已按 decimals 格式化（raw 降级为小字注记），并标注观测时间——快照是创建审计任务那一刻的链上固定区块观测，晚于最近链上事件时需重跑审计；
- 余额/授权/贡献由后端按登录钱包直查（`GET /project/v1/:id/fundingState`），不再依赖浏览器钱包 provider。

### TC-LEDGER-001 幂等性

重启 Job 或重复扫描同一区块。

预期：不重复增加 raised、participant、entitlement或 ledger；事件唯一键保持有效。

### TC-LEDGER-002 on-chain reconciliation

使用 Admin 的 on-chain reconciliation 功能。

预期：数据库 expected 与链上 actual 匹配；manifest/evidence 完整；无 silent repair。若不匹配，任务应明确失败并保留证据。

### TC-LEDGER-003 深度健康

每个关键阶段访问 /health/chain-events。

预期始终 HTTP 200、dead_letters=0、blocked_chain_transactions=0、lag_blocks在允许范围内。

## 14. 页面与内容测试

### TC-UI-001 项目列表

检查三个项目的：封面、名称、标签、时间、进度、状态和支付币。

注意：生产 SQL 备份只有图片路径，没有实际图片文件。当前本机 uploads为空时，图片 404 属于“文件资产未恢复”，不能误判为项目数据库丢失。

### TC-UI-002 项目详情

检查中英文内容、story、research、risks、team、tokenomics、roadmap、documents、FAQ及 on-chain信息。

### TC-UI-003 金额格式

检查：

- USDT 不多显示错误的 12 位零；
- USDC 不被截断为 6 decimals；
- 项目币显示遵循自身 decimals；
- 极小金额、最大额度和舍入边界；
- raw 数值不得使用 JavaScript Number 导致精度丢失；
- 【新增】Admin 钱包详情审计快照的预期/实际支付、Token、Claim 均显示除以 decimals 后的可读金额（raw 仅小字注记）。

### TC-UI-005（新增）Dashboard 退款相关展示

检查：

- 购买历史分「项目状态」「退款状态」两列：项目状态显示 Failed/Successful 等终态，退款状态显示 可退款（附退款按钮，按项目聚合）/ 已退款 / —；
- 「可退款项目」区块仅列出聚合可退余额 > 0 的项目，退款后消失；
- 退款成功后该项目所有购买记录状态联动为已退款，不再出现可点击的失效退款按钮；
- 列表/详情/面板/Admin 各端状态一致（统一由 deriveProjectStage 派生）。

### TC-UI-006（新增）Pre-Launch 倒计时

- 设置入口：Admin → 项目列表 → 编辑内容 → 「计划开售时间 launch_at」；
- launch_at 仅驱动 Pre-Launch 页面倒计时展示，随时可改、可清空，不影响链上 startTime（真实开售时间合约创建后不可改）；
- 倒计时归零后阶段推进取决于项目状态（pre_launch→warmup→active）而非倒计时本身。

### TC-UI-004 刷新和交易等待

交易 pending期间刷新页面、切换项目或断开钱包。

预期：不会显示虚假成功；确认后能从链/API恢复最终状态；重复点击有防抖或幂等保护。

## 15. 安全与权限测试

至少覆盖：

- 普通管理员不能创建/激活链上项目；
- Buyer 不能激活、取消或提取项目方资金；
- Project Owner A 不能操作 Project B；
- Operator 不能绕过 Owner-only治理；
- 非允许 payment token不能购买；
- project token不能等于 payment token；
- fee receiver/rate快照不匹配时创建失败；
- token decimals改变时 fail-closed；
- 重复 projectId创建失败；
- 已锁定链上字段不能通过 Admin内容编辑覆盖；
- 无效/过期 token返回 401/403，不泄露敏感信息；
- 错误日志不包含私钥、JWT、数据库密码。

## 16. 不执行或暂缓的测试

除非负责人另行授权，不执行：

- BSC Testnet/主网交易；
- 生产 Safe 操作；
- 生产数据库写入；
- 生产钱包导入；
- 生产 USDT/USDC转账；
- 镜像或 Git远端 push；
- Lark/COS/Twitter/TG真实调用；
- 删除安全 ledger、trigger或 attestation；
- 手工清理 dead-letter后声称测试通过。

Notice WebSocket和 TG Bot不属于本轮 IDO核心手工验收范围。

## 17. 缺陷记录规范

每个缺陷至少记录：

| 字段 | 内容 |
|---|---|
| 缺陷标题 | 模块 + 动作 + 实际错误 |
| 环境 | 本机 WSL、Chain ID 31337 |
| project_id | 1/2/3 |
| 角色/钱包 | 角色名 + 地址，禁止私钥 |
| 前置状态 | Pending/Active/Successful/Failed/Cancelled |
| 操作步骤 | 可重复的编号步骤 |
| 预期结果 | 明确状态和金额 |
| 实际结果 | 页面/API/链/DB表现 |
| tx hash | 如发生交易则必填 |
| block/log index | 链事件问题必填 |
| API请求 | 路径、状态码、脱敏响应 |
| 健康状态 | /health/chain-events响应 |
| 截图/录像 | 操作前后和钱包确认 |
| 是否可复现 | 次数和比例 |

严禁粘贴私钥、助记词、完整 JWT、数据库密码或生产敏感URL签名。

## 18. 单用例执行记录模板

    用例编号：
    测试人：
    执行时间：
    环境版本：Backend 9bff2bf / Frontend 7ee4015 / Contracts 144a85a
    Chain ID：31337
    project_id：
    钱包角色：
    前置状态：

    步骤：
    1.
    2.
    3.

    预期：
    实际：
    结果：PASS / FAIL / BLOCKED / NOT RUN

    tx hash：
    block number：
    log index：
    API状态码：
    chain health：
    截图/附件：
    缺陷编号：
    备注：

## 19. 测试批次汇总模板

| 用例组 | 总数 | PASS | FAIL | BLOCKED | NOT RUN |
|---|---:|---:|---:|---:|---:|
| 环境准入 | 3 | | | | |
| 项目映射 | 3 | | | | |
| 充值激活 | 4 | | | | |
| 认购 | 12 | | | | |
| 成功分支 | 6 | | | | |
| 失败分支（含结算触发） | 4 | | | | |
| 取消分支与中途管控 | 4 | | | | |
| 账本同步 | 3 | | | | |
| UI内容 | 6 | | | | |
| 安全权限 | 按清单 | | | | |

发布判断建议：

- 任一资金守恒、越权、重复入账、错误 decimals、跨项目 reserve、无法退款/领取问题：阻断发布；
- 任一 schema gate、dead-letter、reconciliation mismatch：阻断发布；
- 图片资产未恢复、非核心样式或文案问题：按环境阻塞或普通缺陷分类；
- 【2026-08-19 修订】claim / refund / withdraw 的 UI 入口均已上线（C 端退款按钮、Dashboard 聚合退款、项目方取回项目币、claim 领取）；如再出现缺失，仍按产品能力缺口处理，不得用合约脚本测试结果替代 UI验收。

## 20. 环境重置

查看和冒烟：

    cd /mnt/d/la/desci/joincare-deploy
    .local-test/jc-local.sh status
    .local-test/jc-local.sh smoke

停止/恢复：

    .local-test/jc-local.sh down
    .local-test/jc-local.sh up

不要执行 rebuild-db，除非确认允许删除当前基于生产快照的本机测试数据。rebuild-db会恢复成 joincare.sql + 40 migrations的干净 seed库，不会重新导入生产快照。

若 Hardhat链丢失，数据库中的链上映射会悬空。必须由环境管理员从头重建基础合约、三个项目 fixture和数据库映射；测试工程师不得自行修正地址。
