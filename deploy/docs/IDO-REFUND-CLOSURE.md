# IDO 失败分支退款闭环 — 功能补齐设计

> 状态：待实施
> 关联：`LOCAL-IDO-MANUAL-TEST-GUIDE.md`（第 11 节失败分支）、`JoinCareIDOv2.sol`
> 目标：补齐「募资结束但未达软顶」后的结算与用户退款，使失败/取消分支形成完整闭环。

## 1. 背景

IDO V2 合约已实现终态与资金退回逻辑，但前端/后端缺少衔接入口。当前若到达募资结束时间（`endTime`）且未达软顶，项目会卡在 `Active` 状态，用户无法通过界面退款，项目方也无法在正确时机取回项目币。

## 2. 现状盘点

| 层 | 能力 | 状态 |
|---|---|---|
| 合约 | `endProject`（结算：Active → Successful/Failed） | ✅ 已有（任何人可调，仅要求 `block.timestamp >= endTime`） |
| 合约 | `refund`（用户退认购款） | ✅ 已有（`msg.sender` 自助，要求 Failed/Cancelled） |
| 合约 | `batchRefund`（owner 批量退） | ✅ 已有（仅 owner） |
| 合约 | `withdrawProjectTokens`（项目方取回项目币） | ✅ 已有 |
| 合约 | `withdrawRaisedFunds`（项目方提取募资款，仅 Successful） | ✅ 已有 |
| 前端 | 项目方「取回项目币」按钮（Dashboard） | ✅ 已有 |
| 前端 | 用户「退款」按钮 | ❌ 缺失 |
| 前端 | 项目状态统一派生（`deriveProjectStage`，live/upcoming/ended_*） | ✅ 已实现 |
| 后端 | `endProject` 结算触发 | ❌ 缺失（仅有 `refundRecord` 查询） |
| 后端 | 用户退款执行接口 | ❌ 缺失 |

## 3. 待补齐功能清单

1. **结算触发**（`endProject` / finalize）：让 `Active` 项目在 `endTime` 之后落成 `Failed` 或 `Successful`。
2. **用户退款入口**（`refund`）：失败/取消后，用户在界面上一键退认购款。
3. **状态展示联动**：退款/取回后，界面状态与按钮联动更新（部分已由 `deriveProjectStage` 覆盖）。

## 4. 详细设计

### 4.1 结算触发（endProject）

**目标**：`endTime` 过后自动（或手动）触发 `endProject`，把项目从 `Active` 结算为 `Successful`（`totalRaised >= softCap`）或 `Failed`（`totalRaised < softCap`）。

**触发方式（建议双通道）**：

- **主通道：后端 job 自动结算**
  - 链事件 job（`joincare-job`）在扫描区块时，检测 `status = Active && block.timestamp >= endTime` 的项目，调用合约 `endProject`（用 relayer/owner 私钥签名）。
  - 幂等：合约 `endProject` 已要求 `status == Active`，重复调用会 revert，天然幂等。
  - 结算结果由链事件回同步（`ProjectEnded` 事件 → 更新数据库 `status`）。

- **兜底通道：Admin 手动结算**
  - Admin 项目列表对「Active 且已过 endTime」的项目提供「结束项目」按钮。
  - 后端 Admin 接口：`POST /admin/v1/projects/:id/end`（校验 `m2_project:execute`，调用 `endProject`）。

**验收**：
- `endTime` 过后，job 自动结算，数据库 `status` 由 `Active` → `Successful`/`Failed`。
- 结算后产生 `ProjectEnded` 链事件，`fund_ledger`/`chain_event_record` 正确落库。
- 重复触发不产生重复状态变更。

### 4.2 用户退款入口（refund）

**目标**：`Failed`/`Cancelled` 项目，用户可自助退回认购款（USDT）。

**调用方式（建议前端直连签名）**：

- 前端在失败/取消态的面板中显示「退款」按钮。
- 用户点击 → 前端 `contractService.refund(project)` → 用户钱包签名调合约 `refund(projectId)`（`msg.sender` 即用户，天然退给本人）。
- 前端已有 `contractService.refund` 方法，仅需补 UI 与状态联动。

**前端 UI 改动（`LaunchpadActiveFundingPanel.vue`）**：

- `ended_failed` / `failed` / `cancelled` 态时，若 `userPurchaseInfo.paymentAmount > 0 && !refunded`，显示「退款」按钮。
- 点击后：`isRefunding` 状态 → 调 `refund` → 成功后刷新 `getFundingState`（refunded 置位、余额恢复）→ 弹成功/失败 modal。
- 已退款后显示「已退款」占位，按钮隐藏。

**后端接口（可选，用于审计/记录）**：

- 退款是纯链上动作，可不经后端；但建议保留 `refundRecord` 查询（已有）用于核对。
- 若产品要求后端统一记账，可加 `POST /project/v1/:projectID/refund`（后端 relayer 调 `batchRefund([msg.sender])`），但会增加签名/权限复杂度，**首期建议前端直连**。

**验收**：
- 失败项目，用户点「退款」→ 钱包签名 → USDT 原路退回，余额恢复。
- 第二次点「退款」被合约 `AlreadyRefunded` 拒绝，前端提示已退款。
- 成功项目不显示退款按钮。
- 退款后 `refunded` 状态在页面与 `fundingState` 接口中一致。

### 4.3 状态展示联动（已基本完成）

- 已实现 `deriveProjectStage`：`Active + 时间过 + raised<softCap` → `ended_failed`，面板显示「未达软顶」。
- 补齐后需额外处理：
  - `ended_failed` 时，面板在「未达软顶」基础上**增加退款按钮**。
  - 数据库 `status` 被结算为 `Failed` 后，`deriveProjectStage` 返回 `failed`，面板展示一致（「已失败 + 退款按钮」）。
  - 项目方 Dashboard 的「取回项目币」按钮在 `Failed` 态继续可用（现有逻辑已覆盖）。

## 5. 完整状态流转

```
                    ┌──────────── 募资结束 endTime ────────────┐
                    │                                          │
              [Active] -- endProject 结算 --> 判定             │
                    │                                          │
        raised >= softCap                 raised < softCap      │
                    │                          │               │
              [Successful]                 [Failed]             │
                    │                          │               │
    项目方: withdrawRaisedFunds      用户: refund (退USDT)      │
    项目方: withdrawProjectTokens    项目方: withdrawProjectTokens│
    用户: claim 项目币               (取回全部项目币)            │
```

**取消分支**：

```
[Pending/Active] -- cancelProject(owner) --> [Cancelled]
      │
      ├─ 用户: refund（退 USDT）
      └─ 项目方: withdrawProjectTokens（取回项目币）
```

## 6. 验收标准

- [ ] `endTime` 后项目能自动/手动结算为 `Successful` 或 `Failed`（数据库与链上一致）。
- [ ] 失败项目用户可一键退款，USDT 原路退回，重复退款被拒。
- [ ] 取消项目同样可退款。
- [ ] 成功项目不出现退款入口，用户正常 claim 项目币。
- [ ] 项目方失败后可取回全部项目币，成功后取回未售出项目币。
- [ ] 所有状态在列表页 / 详情页 / 面板 / Admin 展示一致（`deriveProjectStage` 唯一事实源）。
- [ ] `fund_ledger` / `chain_event_record` 无重复入账，对账（reconciliation）保持平衡。
- [ ] 退款/结算不改变 Hardhat 链上已固化的事实，链事件 job 无 dead-letter。

## 7. 风险与边界

- **结算触发不能依赖用户**：`endProject` 合约虽无权限限制，但产品上必须由 job/Admin 保证及时触发，否则项目长期卡 `Active`。
- **退款必须用户本人签名**：退款走 `msg.sender`，前端直连签名是正确路径；若改为后端 relayer，需改用 `batchRefund` 并增加权限校验。
- **时间基准**：结算/展示以**链上 `block.timestamp`** 为准，数据库 `end_time` 仅用于展示，二者需由链事件同步保持一致。
- **不做假成功**：未达软顶绝不能因 UI 隐藏退款入口而掩盖 `Failed` 状态。

## 8. 关联改动点

| 位置 | 改动 |
|---|---|
| 后端 `joincare-job`（链事件） | 检测 `Active && 时间过`，自动调 `endProject` |
| 后端 Admin（controller + route） | `POST /admin/v1/projects/:id/end` 手动结算 |
| 前端 `LaunchpadActiveFundingPanel.vue` | 失败/取消态增加「退款」按钮与状态联动 |
| 前端 `contract.js` | 复用现有 `refund` 方法（无需改动或仅补错误处理） |
| 后端（可选） | `POST /project/v1/:projectID/refund`（若要求后端记账） |
