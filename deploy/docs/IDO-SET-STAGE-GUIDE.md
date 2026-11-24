# JoinCare IDO V2 项目 Set 阶段操作指南

> **当前版本安全边界（优先于下文历史流程）：Set 数据库写入不可用。**
> `scripts/ido_prepare_set.py` 只允许精确顺序 `PARAMS.json --dry-run` 做离线参数校验和 SQL 审阅证据；不接受 `--`、选项换序、重复选项或其他旧执行参数。
> 任何非 dry-run 调用都会在读取参数文件、RPC、Docker、凭据或数据库配置前返回
> `fair_launch_transformation_unavailable`。当前没有获准的 MySQL 执行命令、密码传输或
> production Set 操作路径。下文生命周期和字段约束保留为权威历史设计，不是可执行说明。
>
> 关联脚本：`joincare-deploy/scripts/ido_prepare_set.py`
> 参数模板：`joincare-deploy/scripts/ido-project-params.example.json`
> 文档性质：离线审阅及历史约束说明

## 1. Set 阶段在生命周期中的位置

```
pre_launch ──(Set脚本)──> Set ──(Admin创建上链)──> Pending ──(充值+Admin激活)──> Active
   资料期              参数就位              链上已建,锁定        募资中
                                                                        │
                                                    结算/取消 ──> Successful / Failed / Canceled
```

**一句话定义**：把「链上创建所需的全部参数」在数据库就位，使后端能通过 `createProject` 前的一致性校验。Set 是纯数据准备，不发任何链上交易。

**最后反悔点**：Set 之后的「创建上链」会把参数固化到合约且永不可改（`onchain_locked=1` 后数据库也锁死）。锁定后想改时间窗/价格/代币，只能取消项目重建。所以所有核对必须在 Set 阶段完成。

## 2. 为什么需要脚本而不是 Admin 表单

1. **参数是链上交易的数字孪生**——`createProject` 将这些值固化进合约，写错一个 raw 值就是链上事故。后端创建时会做完整校验（raw/display 一致、caps 不变式、时间窗、vesting 合计 100%），脚本把这些校验**前置**到准备阶段，提前暴露错误。
2. **受控原则**——Admin 内容编辑接口（`PUT /projects/:id/content`）只开放 name/cover/story 等营销字段，**故意不包含**链上创建参数。当前脚本只生成 dry-run 审阅证据；数据库变更必须等待未来获准的独立执行器。
3. **raw 值不能人算**——`0.1 USDC` = `100000000000000000`（18 位小数），手工换算必错。脚本用十进制字符串移位精确计算，无浮点误差，并保证 display 与 raw 严格成对。

## 3. 脚本写入的数据与约束

### 3.1 链身份

| 字段 | 约束 |
|---|---|
| `contract_version` | 固定 `v2_stablecoin` |
| `chain_id` | 必须与 `secret.yaml` 的 `chain.project.chainID` 一致 |
| `contract_address` | 必须与 `secret.yaml` 的 `chain.project.v2.contractAddress` 一致（IDO V2 proxy） |

### 3.2 代币身份

| 字段 | 约束 |
|---|---|
| `token_address` / `token_decimals` | 项目币合约地址；decimals 1~18；须已被平台 `configureProjectToken(token, true)` |
| `payment_token` / `payment_token_address` / `payment_token_decimals` | 支付币须已在 `secret.yaml` 的 `v2.paymentTokens` 白名单（如 USDT 6 位、USDC 18 位） |
| `owner_address` | 项目方收款地址（成功后提取募资款、失败后取回项目币的接收人） |
| 项目币 ≠ 支付币 | 同地址直接拒绝 |

### 3.3 经济参数（display 与 raw 成对写入）

| 字段 | 约束 |
|---|---|
| `token_price` | > 0；精度不得超过支付币 decimals |
| `hard_cap` / `soft_cap` | > 0；soft ≤ hard |
| `max_per_user` | > 0；≤ hard |
| `fee_receiver` / `success_fee_rate` | 费用快照；创建时合约比对，不匹配拒绝 |

### 3.4 时间与解锁

| 字段 | 约束 |
|---|---|
| `start_time` | **必须在未来**（创建时后端再校验一次；已过期会报 `project sale times are no longer valid for create`） |
| `end_time` | > start |
| `project_tge` 表 | 1~10 期；百分比合计 = 100；每期时间 ≥ end_time；首期 = end + `tge_after_end_min`，之后按 `tge_interval_min` 递推 |

### 3.5 历史执行器必须满足的硬性安全守卫（当前未实现/未准入）

- 仅当 `onchain_locked = 0` 且 `status IN (pre_launch / Set / warmup / Setup)` 时才允许更新，否则必须回滚——**已上链锁定的项目永远改不了**；
- 必须全程单事务，任何一步失败整体回滚；
- 未来执行器必须证明幂等。当前脚本只渲染审阅证据，不执行这些数据库守卫。

## 4. 参数文件说明

照抄 `ido-project-params.example.json`，逐项填写：

```jsonc
{
  "project_id": 2,                    // 业务项目 ID（projects.project_id）
  "chain_id": 31337,                  // 与 secret.yaml chain.project.chainID 一致
  "ido_contract_address": "0x...",    // 与 secret.yaml chain.project.v2.contractAddress 一致
  "fee_receiver": "0x...",            // 与 secret.yaml chain.project.v2.feeReceiver 一致
  "success_fee_rate": 250,            // 与 secret.yaml chain.project.v2.successFeeRate 一致

  "token_address": "0x...",           // 项目币合约
  "token_decimals": 18,
  "payment_token": "USDC",            // 必须在 v2.paymentTokens 白名单内
  "payment_token_address": "0x...",
  "payment_token_decimals": 18,
  "owner_address": "0x...",           // 项目方收款地址

  "token_price": "0.1",               // 金额一律用字符串，避免 JSON 浮点误差
  "hard_cap": "1000",
  "soft_cap": "500",
  "max_per_user": "400",

  "start_time": "+20m",               // 相对偏移（+m/+h/+d/+s）或绝对 "2026-09-01T20:00:00Z"(UTC)
  "end_time": "+3h",
  "tge_after_end_min": 10,            // 首期解锁距 end 多少分钟
  "tge_interval_min": 0,              // 多期间隔；0 = 单期
  "tge": [ { "percent": 100 } ]       // 一次性 100% 解锁；多期示例见 §7
}
```

## 5. 当前版本唯一允许的操作：离线 dry-run

```bash
cd /mnt/d/la/desci/joincare-deploy/scripts
python3 ido_prepare_set.py my-params.json --dry-run
```

该命令只读取参数文件、执行纯本地换算与约束校验，并打印标有
`OFFLINE EVIDENCE ONLY; DO NOT EXECUTE` 的 SQL 审阅材料。不得把输出送入数据库。
非 dry-run、RPC取时、Docker探测、MySQL执行和凭据读取在当前版本均不存在。

历史上的“执行后核对”只能在未来经过独立准入的数据库 transformation executor
成功执行后重新启用；当前不能据此声称项目已进入 Set 或可创建上链项目。

## 6. 正式启用前的完整 checklist

按顺序逐项确认，缺一不可：

1. [ ] 项目币合约已部署并人工验证：decimals、totalSupply ≥ hard_cap/price、owner 持仓充足
2. [ ] 平台已在合约上 `configureProjectToken(token, true)`（relayer = 合约 owner）
3. [ ] 支付币已列入 `secret.yaml` 的 `chain.project.v2.paymentTokens`（地址 + decimals 与链上实测一致）
4. [ ] `chain_id` / `ido_contract_address` / `fee_receiver` / `success_fee_rate` 四项与 `secret.yaml` 逐字一致
5. [ ] `owner_address` 已与项目方书面确认（成功提款、失败退币的唯一接收地址）
6. [ ] 金额参数（价格/软顶/硬顶/单人上限）已经业务方签字确认
7. [ ] 时间窗已确认（含时区换算；**DB 存无时区裸值，后端按容器时区 UTC+8 解释**，脚本靠 `db_tz_offset_hours=8` 对齐，务必与后端容器 TZ 一致）
8. [ ] TGE 解锁计划（期数/比例/时间）已与 vesting 条款一致
9. [ ] `--dry-run` 输出已由第二人复核
10. [ ] 保持数据库不变；等待独立准入的 transformation authority（当前不可用）

## 7. 常见示例

### 一次性解锁（默认模板）

```json
"tge_after_end_min": 10,
"tge_interval_min": 0,
"tge": [ { "percent": 100 } ]
```

### 线性 4 期解锁（TGE 25% + 之后每 30 天 25%）

```json
"tge_after_end_min": 60,
"tge_interval_min": 43200,
"tge": [
  { "percent": 25 },
  { "percent": 25 },
  { "percent": 25 },
  { "percent": 25 }
]
```

## 8. 错误对照表

| 现象 | 原因 | 处理 |
|---|---|---|
| `start_time 不在未来` / `project sale times are no longer valid for create` | ① 时间窗真已过；② **时区错位**：DB 存的是无时区裸值，后端按容器时区（Asia/Shanghai, UTC+8）解释，而写入时用了 UTC 语义 → 差 8 小时被拒 | 在参数文件中修正时间/`db_tz_offset_hours`并重新做离线 dry-run；当前不得写库。已上链只能取消重建 |
| `精度超过 N 位小数` | 金额小数位超过支付币 decimals | 调整金额精度 |
| `GUARD_FAILED: 项目不存在/已锁定/状态不允许` | 项目已上链（`onchain_locked=1`）或处于终态 | 该项目不可再改参数；如需变更走取消/重建 |
| `soft_cap > hard_cap` / `max_per_user > hard_cap` | caps 不变式 | 修正参数 |
| `TGE 百分比合计 != 100` | 解锁比例配置错误 | 修正 tge 数组 |
| 创建上链时报 `fee snapshot` 不匹配 | `fee_receiver`/`success_fee_rate` 与合约当前值不一致 | 修正离线参数并等待获准 transformation authority；当前不得写库 |
| 创建上链时报 payment token mismatch | 支付币不在可信配置或地址/decimals 不一致 | 修正离线参数并等待获准 transformation authority；当前不得写库 |

## 9. 与后续阶段的衔接

Set 完成只是第一步，完整链路：

| 步骤 | 操作人 | 动作 | 状态变化 |
|---|---|---|---|
| 1 | 未来获准的独立 transformation executor | 当前不存在；本脚本只做 dry-run | pre_launch → **Set** |
| 2 | Admin（m2_project:execute） | 创建链上项目（durable） | Set → **Pending**（`onchain_locked=1`，参数永久锁定） |
| 3 | 项目方 | 项目币 `approve` + `depositProjectTokens`（≥ hard_cap/price） | 充值入合约托管 |
| 4 | Admin | 激活上链（要求已锁 + 储备充足） | Pending → **Active** |
| 5 | 用户 | start 后认购（`purchase`） | 募资 |
| 6 | Admin | 结束结算（`endProject`，合约按 softCap 判定） | Active → **Successful / Failed** |
| 7 | 用户 / 项目方 | 成功：claim / withdrawRaisedFunds；失败：refund / withdrawProjectTokens | 终态资金交割 |

> 注意：当前版本只能离线复核参数，不能推进步骤 1。未来步骤 2 之后一切以链上为准。
