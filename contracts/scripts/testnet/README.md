# BSC 测试网测试脚本

本目录包含 JoinCare IDO 平台在 BSC 测试网上的完整测试套件。

## 📁 目录结构

```
scripts/testnet/
├── config/
│   ├── accounts.json         # 测试账户配置（含私钥）
│   ├── contracts.json        # 已部署合约地址
│   └── testParams.json       # 测试参数（时间、金额等）
├── setup/
│   ├── 01_generateAccounts.js
│   ├── 02_distributeAssets.js
│   ├── 03_deployProjectToken.js
│   └── 04_verifySetup.js
├── tests/                    # 单模块测试文件（可独立运行）
│   ├── 01_deployment.test.js
│   ├── 02_projectCreation.test.js
│   ├── 03_depositTokens.test.js
│   ├── 04_activation.test.js
│   ├── 05_purchase.test.js
│   ├── 06_projectEnding.test.js
│   ├── 07_claim.test.js
│   ├── 08_refund.test.js
│   └── 09_withdraw.test.js
├── utils/
│   ├── testHelper.js         # 公共工具：logTest / expectRevert / preflight / waitUntil
│   ├── contractHelper.js
│   ├── timeHelper.js
│   ├── verificationHelper.js
│   └── gasTracker.js
├── reports/
│   ├── TEST_REPORT.md               # runCompleteTest 的 Markdown 报告
│   ├── FULL_TEST_REPORT.md          # runFullTest 的 Markdown 报告（最新）
│   ├── fullTestReport.json          # runCompleteTest 的 JSON 报告
│   └── fullTestReport_advanced.json # runFullTest 的 JSON 报告（最新）
├── checkBalances.js          # 余额检查工具
├── supplementJCC.js          # JCC 补充工具
├── runCompleteTest.js        # 🟢 基础完整测试（13测试，首选快速验证）
└── runFullTest.js            # 🔵 全面测试（45测试，主网前验证）
```

---

## 🚀 快速开始

### 部署测试项目代币

独立部署脚本使用 `TestProjectToken`（固定 18 decimals、供应量硬顶、仅 owner 可增发），只允许 BSC Testnet（97）和本地测试链（1337/31337）。

```bash
# 本地 Hardhat 节点
npm run deploy:test-project-token -- --network localhost

# BSC 测试网（hardhat.config.js 的 jcTarget）
JC_DEPLOY_RPC=<rpc> JC_DEPLOY_CHAIN_ID=97 JC_DEPLOY_PRIVATE_KEY=<key> npm run deploy:test-project-token -- --network jcTarget
```

可通过以下环境变量覆盖默认值：`TEST_PROJECT_TOKEN_NAME`、`TEST_PROJECT_TOKEN_SYMBOL`、`TEST_PROJECT_TOKEN_INITIAL_SUPPLY`、`TEST_PROJECT_TOKEN_MAX_SUPPLY`、`TEST_PROJECT_TOKEN_OWNER`。默认名称/符号为 `JoinCare Test Project Token / JCPT`，初始供应量 1,000,000，硬顶 10,000,000。

完整测试网账户流程中的 `setup/03_deployProjectToken.js` 也已切换到该合约，部署后会自动更新 `config/contracts.json`。

### 检查账户余额

```bash
JC_DEPLOY_RPC=<rpc> JC_DEPLOY_CHAIN_ID=97 JC_DEPLOY_PRIVATE_KEY=<key> npx hardhat run scripts/testnet/checkBalances.js --network jcTarget
```

### 运行基础完整测试（推荐首次）

```bash
JC_DEPLOY_RPC=<rpc> JC_DEPLOY_CHAIN_ID=97 JC_DEPLOY_PRIVATE_KEY=<key> npm run test:e2e:quick
```

- **测试数量**：13 个
- **耗时**：约 20 分钟
- **覆盖**：Happy Path 完整流程

### 运行全面测试（推荐主网前验证）

```bash
JC_DEPLOY_RPC=<rpc> JC_DEPLOY_CHAIN_ID=97 JC_DEPLOY_PRIVATE_KEY=<key> npm run test:e2e:full
```

- **测试数量**：45 个
- **耗时**：约 30 分钟
- **覆盖**：Happy Path + 参数验证 + 权限控制 + 失败项目退款 + 双花防护

---

## 📊 测试脚本对比

| 特性 | `runCompleteTest.js` | `runFullTest.js` |
|------|---------------------|------------------|
| 测试数量 | 13 | **45** |
| 耗时 | ~20 分钟 | ~30 分钟 |
| 参数验证（revert）| ❌ | ✅ 9 项 |
| 权限控制（revert）| ❌ | ✅ 7 项 |
| 失败项目退款流程 | ❌ | ✅ |
| 重复操作防护 | ❌ | ✅ |
| Vesting 完整3轮 | ✅ | ✅ |
| 资金提取含手续费 | ✅ | ✅ |
| 双花防护测试 | ❌ | ✅ |

---

## 🧪 测试覆盖详情（runFullTest.js，共 45 项）

### Phase 1：参数验证 & 权限控制（无需等待，9项）

| # | 测试项 | 预期结果 |
|---|--------|---------| 
| 1 | 过期的 startTime | revert |
| 2 | endTime ≤ startTime | revert |
| 3 | 空的 vesting 数组 | revert |
| 4 | vesting 数组长度不匹配 | revert |
| 5 | vesting 总比例 ≠ 100% | revert |
| 6 | vesting 时间不递增 | revert |
| 7 | 第一个 vesting 早于 endTime | revert |
| 8 | softCap > hardCap | revert |
| 9 | 非 owner 创建项目 | revert |

### Phase 2：项目创建 & 激活（12项）

| # | 测试项 | 预期结果 |
|---|--------|---------|
| 10 | 创建成功项目 | ✅ |
| 11 | 创建失败项目 | ✅ |
| 12 | 重复 projectId | revert |
| 13 | 非 projectOwner 充值 | revert |
| 14 | 未充值时激活 | revert |
| 15 | 成功项目充值金额正确 | ✅ |
| 16 | 非 owner 激活 | revert |
| 17 | 失败项目充值 | ✅ |
| 18 | 手动激活成功项目 | ✅ |
| 19 | Active 状态再次充值 | revert |
| 20 | 手动激活失败项目 | ✅ |
| 21 | 非 owner 在 endTime 前结束 | revert |

### Phase 3：认购阶段（等待 startTime，4项）

| # | 测试项 | 预期结果 |
|---|--------|---------|
| 22 | 5用户各购 1000 JCC，达到软顶 | ✅ |
| 23 | 超过单用户限额 | revert |
| 24 | 失败项目少量认购 | ✅ |
| 25 | Non-failed 项目不能退款 | revert |

### Phase 4：失败项目退款（等待 p2End，7项）

| # | 测试项 | 预期结果 |
|---|--------|---------|
| 26 | 结束失败项目，状态为 Failed | ✅ |
| 27 | user1 退款成功 | ✅ |
| 28 | 重复退款 | revert |
| 29 | 无认购记录不能退款 | revert |
| 30 | 失败项目提取全部代币 | ✅ |
| 31 | 失败项目不能提取募集资金 | revert |
| 32 | 重复提取失败项目代币 | revert |

### Phase 5：成功项目 Claim（等待 p1End + vestingTimes，7项）

| # | 测试项 | 预期结果 |
|---|--------|---------|
| 33 | 结束成功项目，状态为 Successful | ✅ |
| 34 | Vesting 开始前不能 Claim | revert |
| 35 | 第一次 Vesting（50%）| ✅ |
| 36 | 重复 Claim（无新释放）| revert |
| 37 | 无认购记录不能 Claim | revert |
| 38 | 第二次 Vesting（30%）| ✅ |
| 39 | 第三次 Vesting（20%）| ✅ |

### Phase 6：资金提取（6项）

| # | 测试项 | 预期结果 |
|---|--------|---------|
| 40 | 非 projectOwner 不能提取募集资金 | revert |
| 41 | 提取募集资金（扣2%手续费）| ✅ |
| 42 | 重复提取募集资金 | revert |
| 43 | 非 projectOwner 不能提取项目代币 | revert |
| 44 | 提取未售出代币 | ✅ |
| 45 | 重复提取项目代币 | revert |

---

## ⚙️ 时间配置（config/testParams.json）

```json
{
  "timeConfig": {
    "option": "Full",
    "projectStartDelay": 600,
    "projectDuration": 300,
    "vestingDelays": [180, 480, 780],
    "vestingPercents": [5000, 3000, 2000]
  }
}
```

### 时间说明

| 关键时刻 | 计算方式 | 说明 |
|----------|---------|------|
| `p1Start` / `p2Start` | `cur + 600s` | 两个项目同一时刻开始 |
| `p2End` | `p2Start + 180s` | 失败项目仅 3 分钟，先于 p1End 结束 |
| `p1End` | `p1Start + 300s` | 成功项目 5 分钟 |
| `Vesting[0]` | `p1End + 180s` | 第一次释放 50% |
| `Vesting[1]` | `p1End + 480s` | 第二次释放 30% |
| `Vesting[2]` | `p1End + 780s` | 第三次释放 20% |

> ⚠️ `projectStartDelay` 必须 ≥ 600s，以确保 Phase 1（约 2 分钟）+ Phase 2（约 7 分钟）全部前置操作能在 startTime 到来前完成。

---

## 🔧 工具说明（utils/testHelper.js）

### `expectRevert(fn, expectedMsg)`

测试预期会 revert 的交易，兼容两种错误格式：

1. **传统 require 字符串**：`"Project not failed"` 等
2. **OZ v5 Custom Error**：`OwnableUnauthorizedAccount`、`EnforcedPause` 等（链上返回 ABI 编码 hex，自动解码匹配）

### `preflight(params)`

执行前，自动检查并补充各账户的 JCC / PT 余额，确保测试不因余额不足失败。

### `waitUntil(targetTimestamp, label)`

等待至指定的 Unix 时间戳，含 +5s 缓冲。

---

## 💰 账户结构

| 角色 | 用途 |
|------|------|
| deployer | 合约 owner，创建/激活/结束项目，mint JCC/PT |
| projectOwner | 充值代币、提取募集资金和未售出代币 |
| user1~5 | 参与认购、Claim 代币、退款 |

---

## ✅ 最新测试结果

### runCompleteTest.js

**测试日期**: 2026-02-21
**通过率**: 13/13 (100%) ✅

### runFullTest.js

**测试日期**: 2026-02-22
**通过率**: **45/45 (100%) ✅**
**总耗时**: 30 分 33 秒

---

**最后更新**: 2026-02-22
