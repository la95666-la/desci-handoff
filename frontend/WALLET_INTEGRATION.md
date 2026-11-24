# 钱包集成说明 / Wallet Integration Guide

## 🎉 已实现功能 / Implemented Features

### ✅ 真实的 Web3 钱包连接
- MetaMask 钱包连接
- 自动重连（刷新页面后保持连接状态）
- 账户切换检测
- 网络切换检测
- BNB 余额查询
- JCC 代币余额查询（需配置合约地址）

### ✅ 网络管理
- 自动检测当前网络
- 一键切换到 BSC 主网
- 支持 BSC 测试网
- 错误网络提示

### ✅ 用户界面
- 连接/断开钱包按钮
- 钱包地址缩写显示
- 余额显示（BNB + JCC）
- 下拉菜单（桌面端）
- 完整信息卡片（移动端）
- 复制地址功能
- 在 BscScan 查看地址
- 连接状态指示器

---

## 📦 技术栈

- **ethers.js v6** - Web3 库
- **Pinia** - 状态管理
- **Vue 3 Composition API** - 响应式框架

---

## 🔧 配置 JCC 代币合约

如果您已经部署了 JCC 代币合约，请更新以下文件：

### `src/services/web3.js`

```javascript
// 将这行：
const JCC_TOKEN_ADDRESS = '0x0000000000000000000000000000000000000000' // Placeholder

// 替换为您的实际合约地址：
const JCC_TOKEN_ADDRESS = '0xYourActualJCCTokenAddress'
```

---

## 🚀 使用方法

### 1. 安装 MetaMask

用户需要先安装 MetaMask 浏览器扩展：
- Chrome: https://metamask.io/download/
- Firefox: https://metamask.io/download/
- Edge: https://metamask.io/download/

### 2. 连接钱包

1. 点击导航栏的 **"Connect Wallet"** 按钮
2. MetaMask 会弹出授权窗口
3. 选择要连接的账户
4. 点击 **"Next"** 和 **"Connect"**
5. 连接成功后会显示钱包地址缩写

### 3. 切换网络

如果您连接的不是 BSC 网络：
1. 按钮会显示 **"Switch to BSC"**（红色）
2. 点击按钮
3. MetaMask 会提示切换网络
4. 如果您的 MetaMask 没有 BSC 网络，会自动添加

### 4. 查看余额

**桌面端：**
- 点击钱包地址按钮
- 下拉菜单会显示：
  - 完整地址
  - BNB 余额
  - JCC 余额
  - 操作按钮

**移动端：**
- 打开移动菜单
- 钱包信息卡片会显示所有信息

### 5. 断开连接

1. 点击钱包地址打开菜单
2. 点击 **"Disconnect"** 按钮（红色）
3. 钱包状态会被清除

---

## 🔐 安全特性

### 自动重连
- 刷新页面后自动重连上次连接的钱包
- 使用 localStorage 保存连接状态
- 验证账户是否仍然授权

### 账户切换检测
- 监听 MetaMask 账户切换事件
- 自动更新显示的地址和余额
- 如果用户断开所有账户，自动清除连接状态

### 网络切换检测
- 监听 MetaMask 网络切换事件
- 自动刷新页面以确保状态一致
- 防止在错误网络上进行交易

---

## 🛠️ 开发者指南

### Wallet Store API

```javascript
import { useWalletStore } from '@/stores/wallet'

const walletStore = useWalletStore()

// 连接钱包
await walletStore.connectWallet()

// 断开钱包
walletStore.disconnect()

// 切换到 BSC 主网
await walletStore.switchNetwork(false)

// 切换到 BSC 测试网
await walletStore.switchNetwork(true)

// 刷新余额
await walletStore.refreshBalances()

// 访问状态
console.log(walletStore.address)        // 钱包地址
console.log(walletStore.chainId)        // 链 ID
console.log(walletStore.isConnected)    // 是否连接
console.log(walletStore.balance)        // BNB 余额
console.log(walletStore.jccBalance)     // JCC 余额
console.log(walletStore.shortAddress)   // 地址缩写
console.log(walletStore.isCorrectNetwork) // 是否在正确网络
```

### Web3 Service API

```javascript
import { web3Service } from '@/services/web3'

// 检查 MetaMask 是否安装
const isInstalled = web3Service.isMetaMaskInstalled()

// 连接钱包
const walletData = await web3Service.connectWallet()

// 切换到 BSC
await web3Service.switchToBSC(false) // false = mainnet, true = testnet

// 获取当前账户
const account = await web3Service.getCurrentAccount()

// 监听账户变化
web3Service.onAccountsChanged((accounts) => {
  console.log('Accounts changed:', accounts)
})

// 监听链变化
web3Service.onChainChanged((chainId) => {
  console.log('Chain changed:', chainId)
})
```

---

## 🌐 支持的网络

### BSC Mainnet (主网)
- Chain ID: 56 (0x38)
- RPC: https://bsc-dataseed1.binance.org
- Explorer: https://bscscan.com

### BSC Testnet (测试网)
- Chain ID: 97 (0x61)
- RPC: https://data-seed-prebsc-1-s1.binance.org:8545
- Explorer: https://testnet.bscscan.com

---

## ❓ 常见问题

### Q: 为什么 JCC 余额显示为 0？
A: 需要在 `src/services/web3.js` 中配置正确的 JCC 代币合约地址。

### Q: 如何添加其他代币余额查询？
A: 在 `web3Service.connectWallet()` 方法中添加类似 JCC 的查询逻辑。

### Q: 如何支持其他钱包（如 WalletConnect）？
A: 需要集成 WalletConnect 库，并在 `web3Service` 中添加相应的连接方法。

### Q: 刷新页面后为什么会自动连接？
A: 这是设计的功能。如果不需要，可以在 `App.vue` 中移除 `walletStore.autoConnect()` 调用。

### Q: 如何禁用自动重连？
A: 在 `src/App.vue` 中注释掉这行：
```javascript
// await walletStore.autoConnect()
```

---

## 🔄 下一步计划

- [ ] 集成 WalletConnect（支持移动端钱包）
- [ ] 集成 OKX Wallet
- [ ] 添加钱包切换功能（支持多个钱包）
- [ ] 添加交易历史查询
- [ ] 添加 Gas 费用估算
- [ ] 添加代币授权管理

---

## 📝 更新日志

### 2026-02-07
- ✅ 实现 MetaMask 钱包连接
- ✅ 实现自动重连功能
- ✅ 实现网络切换功能
- ✅ 实现余额查询功能
- ✅ 实现账户/网络变化监听
- ✅ 完善桌面端和移动端 UI

