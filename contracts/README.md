# JoinCare IDO Platform - Smart Contracts

JoinCare IDO平台智能合约 - 专注于生物医药行业的去中心化代币发行平台

## 📋 项目概述

JoinCare是一个基于BNB Chain的IDO（Initial DEX Offering）平台，专为生物医药行业项目提供代币发行服务。平台采用UUPS可升级模式，支持单合约管理多个项目，具备完善的安全机制和完全灵活的代币释放方案。

### 核心特性

- ✅ **UUPS可升级模式** - 支持无限次升级，无需重新部署代理
- ✅ **单合约多项目** - 一个合约管理所有IDO项目
- ✅ **完全灵活的释放机制** - 支持任意复杂的释放时间表（数组方式）
- ✅ **去中心化项目结束** - 任何人都可以在时间到期后结束项目
- ✅ **平台费用** - 成功项目收取2%抽成
- ✅ **双重激活机制** - 手动激活 + 自动激活
- ✅ **完善的安全防护** - 7大安全机制保障资金安全

## 🔒 安全机制

1. **ReentrancyGuard** - 防止重入攻击
2. **Pausable** - 紧急暂停功能
3. **Ownable** - 权限控制
4. **SafeERC20** - 安全的ERC20交互
5. **Fee-on-transfer防御** - balanceBefore/balanceAfter模式
6. **18 decimals强制验证** - 防止精度错误
7. **释放参数验证** - 防止配置错误和除零错误

## 📦 项目结构

```
JoinCare_SmartContract/
├── contracts/
│   ├── JoinCare_v1.sol              # 主合约（UUPS可升级）
│   ├── interfaces/
│   │   └── IJoinCare.sol            # 接口定义
│   └── mocks/
│       ├── MockERC20.sol            # 测试用ERC20代币
│       ├── MockJCC.sol              # 测试用JCC代币
│       └── MockFeeOnTransferToken.sol # 测试用fee-on-transfer代币
├── scripts/
│   ├── deploy.js                    # UUPS代理部署脚本
│   └── upgrade.js                   # 升级脚本
├── test/
│   └── JoinCare.test.js             # 完整测试套件（41个测试）
├── hardhat.config.js                # Hardhat配置
├── package.json                     # 项目依赖
├── .env.example                     # 环境变量示例
└── README.md                        # 项目文档
```

## 🚀 快速开始

### 环境要求

- Node.js >= 16.0.0
- npm >= 8.0.0

### 安装依赖

```bash
npm install
```

### 配置环境变量

复制 `.env.example` 为 `.env` 并填写配置：

```bash
cp .env.example .env
```

编辑 `.env` 文件：

```env
# 部署账户私钥（不含0x前缀）
PRIVATE_KEY=your_private_key_here

# RPC URLs
BSC_TESTNET_RPC=https://data-seed-prebsc-1-s1.binance.org:8545
BSC_MAINNET_RPC=https://bsc-dataseed.binance.org

# BscScan API Key（用于合约验证）
BSCSCAN_API_KEY=your_bscscan_api_key_here

# 合约部署参数
JCC_TOKEN_ADDRESS=0x0000000000000000000000000000000000000000
FEE_RECEIVER_ADDRESS=0x0000000000000000000000000000000000000000
INITIAL_FEE_RATE=200
```

## 🔨 编译和测试

### 编译合约

```bash
npm run compile
```

### 运行测试

```bash
npm test
```

**当前测试状态**: ✅ 41/41 测试通过 (100%)

### 测试覆盖率

```bash
npm run coverage
```

### Gas报告

```bash
REPORT_GAS=true npm test
```

## 📤 部署

### 部署到BSC测试网

```bash
npm run deploy:testnet
```

### 部署到BSC主网

```bash
npm run deploy:mainnet
```

### 升级合约

```bash
# 设置PROXY_ADDRESS环境变量
export PROXY_ADDRESS=0x...

# 升级到新版本
npm run upgrade:testnet
# 或
npm run upgrade:mainnet
```

## 📖 合约使用说明

### 项目生命周期

1. **创建项目** - 平台管理员调用 `createProject()`
2. **充值代币** - 项目方调用 `depositProjectTokens()`
3. **激活项目** - 管理员调用 `activateProject()` 或首次认购时自动激活
4. **用户认购** - 用户调用 `purchase()`
5. **项目结束** - 任何人在时间到期后调用 `endProject()`（去中心化）
6. **资金分配** - 项目方提取资金和代币
7. **代币领取** - 用户在释放时间到达后调用 `claim()` 领取代币

### 关键函数

#### 管理员函数

- `createProject()` - 创建新项目
- `activateProject()` - 手动激活项目
- `endProject()` - 结束项目（owner可随时调用，其他人需等到endTime后）
- `setPlatformFees()` - 设置平台费用
- `pause()` / `unpause()` - 紧急暂停/恢复

#### 项目方函数

- `depositProjectTokens()` - 充值项目代币
- `withdrawRaisedFunds()` - 提取募集资金（成功时）
- `withdrawProjectTokens()` - 提取项目代币（成功/失败时）

#### 用户函数

- `purchase()` - 认购项目代币
- `claim()` - 领取代币（释放时间到达后）
- `refund()` - 申请退款（失败时）

#### 查询函数

- `getProject()` - 查询项目信息
- `getUserPurchase()` - 查询用户认购信息
- `calculateClaimable()` - 计算可领取数量
- `getParticipantCount()` - 查询参与者数量

## 🎯 灵活释放机制

### 新特性说明

平台支持**完全灵活的代币释放时间表**，使用两个数组定义：

- `vestingTimes[]` - 释放时间点数组（Unix时间戳）
- `vestingPercents[]` - 对应的释放百分比数组（基点，总和必须=10000）

### 释放机制示例

#### 示例1：不均匀释放
```javascript
// TGE释放20%，1个月后20%，3个月后20%，5个月后10%，9个月后30%
vestingTimes: [tge, tge+1m, tge+3m, tge+5m, tge+9m]
vestingPercents: [2000, 2000, 2000, 1000, 3000]  // 总和=10000 (100%)
```

#### 示例2：一次性释放
```javascript
// TGE时100%释放
vestingTimes: [tge]
vestingPercents: [10000]
```

#### 示例3：递增释放
```javascript
// 逐步递增的释放比例
vestingTimes: [tge, tge+1m, tge+2m, tge+3m]
vestingPercents: [1000, 2000, 3000, 4000]  // 10%, 20%, 30%, 40%
```

#### 示例4：标准线性释放
```javascript
// 每月释放20%，共5个月
vestingTimes: [tge, tge+1m, tge+2m, tge+3m, tge+4m]
vestingPercents: [2000, 2000, 2000, 2000, 2000]
```

### 释放参数约束

1. **数组不能为空** - 至少需要一个释放点
2. **数组长度一致** - `vestingTimes.length` 必须等于 `vestingPercents.length`
3. **数组长度限制** - 最多10个释放点（平衡灵活性和Gas成本）
4. **时间严格递增** - 每个时间点必须大于前一个
5. **百分比总和=100%** - 所有百分比之和必须等于10000（基点）
6. **百分比必须>0** - 每个释放点的百分比必须大于0
7. **第一个时间晚于endTime** - 第一个释放时间必须在认购结束之后

## ⚠️ 重要注意事项

### 项目代币要求

- **必须是18 decimals** - 不支持6/8 decimals代币（如USDT/USDC/WBTC）
- 原因：简化计算逻辑，避免精度转换错误

### 项目方充值要求

虽然合约不强制验证充值数量，但项目方必须充值足够的代币以确保项目成功：
- 如果充值不足，用户认购到一定程度后将无法继续购买
- 这会导致项目无法达到软顶，最终失败
- 项目失败后用户可以全额退款，项目方无法获得任何收益
- **因此项目方有强烈的经济动机充值足够的代币**

### 去中心化项目结束

- **任何人**都可以在 `endTime` 之后调用 `endProject()` 结束项目
- **owner** 可以在任何时候调用（用于紧急情况，如发现项目方欺诈）
- 这种设计提升了平台的去中心化程度，减少了对平台方的依赖

### 时间约束

- 第一个释放时间必须严格晚于认购结束时间
- 建议间隔1-3天，避免竞态条件

### 安全建议

1. **部署前** - 在测试网充分测试
2. **部署后** - 在BscScan上验证合约
3. **升级时** - 验证存储布局兼容性
4. **紧急情况** - 使用 `pause()` 暂停合约
5. **主网前** - 建议进行专业安全审计

## 📊 测试覆盖

### 测试模块

测试套件包含以下模块（共41个测试）：

- ✅ 部署和初始化测试 (3个)
- ✅ 项目创建测试 (6个) - 参数验证、权限检查
- ✅ 项目方充值测试 (4个) - fee-on-transfer防御
- ✅ 项目激活测试 (3个) - 手动/自动激活
- ✅ 项目结束测试 (5个) - 去中心化结束机制
- ✅ 用户认购测试 (6个) - 硬顶、单用户上限、自动结束
- ✅ 代币领取测试 (6个) - 灵活释放机制
- ✅ 退款测试 (5个) - 单个/批量退款
- ✅ 资金提取测试 (3个) - 平台费用、双花防护

### 测试覆盖率

当前覆盖率：
- 语句覆盖率：87.43%
- 分支覆盖率：58.00%
- 函数覆盖率：70.00%
- 行覆盖率：85.64%

目标覆盖率：
- 语句覆盖率：≥85% ✅
- 分支覆盖率：≥80% ⚠️
- 函数覆盖率：≥90% ⚠️
- 行覆盖率：≥85% ✅

## 📈 项目统计

### 代码量统计

```
总文件数: 15个
总代码量: ~3,500行
├── 合约代码: ~1,252行
├── 测试代码: ~1,033行
├── 脚本代码: ~200行
├── 配置文件: ~100行
└── 文档: ~900行
```

### 交付物清单

#### 智能合约 (5个文件)
- ✅ JoinCare_v1.sol (913行) - 主合约
- ✅ IJoinCare.sol (200行) - 接口定义
- ✅ MockERC20.sol (40行) - 测试用ERC20
- ✅ MockJCC.sol (15行) - 测试用JCC代币
- ✅ MockFeeOnTransferToken.sol (84行) - 测试用fee-on-transfer代币

#### 测试文件 (1个文件)
- ✅ JoinCare.test.js (1033行) - 41个测试，100%通过

#### 部署脚本 (2个文件)
- ✅ deploy.js - UUPS代理部署
- ✅ upgrade.js - 合约升级

#### 配置文件 (4个文件)
- ✅ package.json - 项目依赖
- ✅ hardhat.config.js - Hardhat配置（含BSC网络）
- ✅ .env.example - 环境变量模板
- ✅ .gitignore - Git忽略规则

## 🔗 合约地址

### BSC Testnet

- 代理地址：`待部署`
- 实现地址：`待部署`

### BSC Mainnet

- 代理地址：`待部署`
- 实现地址：`待部署`

## 🛠️ 技术栈

- **Solidity**: ^0.8.22
- **OpenZeppelin Contracts**: ^5.0.0
- **Hardhat**: ^2.19.0
- **Ethers.js**: ^6.9.0
- **Chai**: ^4.3.10

## 📝 更新日志

### v1.1.0 (2026-02-12)
- ✅ 重构释放机制：从固定间隔改为完全灵活的数组方式
- ✅ 去中心化项目结束：任何人都可以在endTime后结束项目
- ✅ 更新所有测试用例以适配新的释放机制
- ✅ 测试数量从36个增加到41个

### v1.0.0 (2026-02-10)
- ✅ 初始版本发布
- ✅ 实现UUPS可升级模式
- ✅ 实现单合约多项目架构
- ✅ 实现7大安全机制
- ✅ 完成36个测试用例

## 📝 许可证

MIT License

## 🤝 贡献

欢迎提交Issue和Pull Request

## 📧 联系方式

- 项目网站：待定
- 技术支持：待定

---

**⚠️ 免责声明**

本智能合约仅供学习和研究使用。使用前请进行充分的安全审计。开发团队不对使用本合约造成的任何损失负责。

---

**开发完成日期**: 2026-02-12
**项目状态**: ✅ 核心开发完成，待测试网部署
**建议**: 在主网部署前进行专业安全审计
