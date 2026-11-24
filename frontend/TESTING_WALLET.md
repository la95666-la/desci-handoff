# 钱包连接测试指南 / Wallet Connection Testing Guide

## 🧪 测试前准备

### 1. 安装 MetaMask
如果还没有安装 MetaMask：
1. 访问 https://metamask.io/download/
2. 选择您的浏览器（Chrome/Firefox/Edge）
3. 点击 "Install MetaMask"
4. 按照提示完成安装

### 2. 创建或导入钱包
- 如果是新用户：创建新钱包并保存助记词
- 如果已有钱包：导入现有钱包

### 3. 添加 BSC 网络（如果还没有）
1. 打开 MetaMask
2. 点击顶部的网络下拉菜单
3. 点击 "Add Network"
4. 选择 "Add a network manually"
5. 输入以下信息：
   - **Network Name**: BNB Smart Chain
   - **RPC URL**: https://bsc-dataseed1.binance.org
   - **Chain ID**: 56
   - **Currency Symbol**: BNB
   - **Block Explorer**: https://bscscan.com
6. 点击 "Save"

---

## ✅ 测试场景

### 场景 1: 首次连接钱包

**步骤：**
1. 打开应用 http://localhost:5175/
2. 点击导航栏的 **"Connect Wallet"** 按钮
3. MetaMask 弹出窗口应该出现
4. 选择要连接的账户
5. 点击 **"Next"** → **"Connect"**

**预期结果：**
- ✅ 按钮文字变为钱包地址缩写（如 `0x1234...5678`）
- ✅ 按钮颜色变为绿色（primary color）
- ✅ 控制台没有错误

**桌面端额外检查：**
- ✅ 点击地址按钮，下拉菜单出现
- ✅ 菜单显示完整地址、BNB 余额、JCC 余额
- ✅ 菜单包含 "Copy Address"、"View on BscScan"、"Disconnect" 按钮

**移动端额外检查：**
- ✅ 打开移动菜单
- ✅ 钱包信息卡片显示完整信息
- ✅ 所有操作按钮可见

---

### 场景 2: 错误网络检测

**步骤：**
1. 在 MetaMask 中切换到以太坊主网（Ethereum Mainnet）
2. 刷新页面或重新连接钱包

**预期结果：**
- ✅ 按钮变为红色
- ✅ 按钮文字显示 **"Switch to BSC"**
- ✅ 点击按钮后 MetaMask 提示切换网络
- ✅ 确认后自动切换到 BSC 网络
- ✅ 按钮恢复正常状态

---

### 场景 3: 自动重连

**步骤：**
1. 连接钱包（参考场景 1）
2. 刷新浏览器页面（F5 或 Cmd+R）

**预期结果：**
- ✅ 页面加载后自动显示已连接状态
- ✅ 钱包地址正确显示
- ✅ 余额信息正确显示
- ✅ 无需重新授权

---

### 场景 4: 账户切换

**步骤：**
1. 连接钱包
2. 打开 MetaMask
3. 切换到另一个账户

**预期结果：**
- ✅ 应用自动检测到账户变化
- ✅ 显示的地址更新为新账户
- ✅ 余额更新为新账户的余额
- ✅ 无需手动刷新

---

### 场景 5: 断开连接

**桌面端步骤：**
1. 点击钱包地址按钮
2. 在下拉菜单中点击 **"Disconnect"**（红色按钮）

**移动端步骤：**
1. 打开移动菜单
2. 在钱包信息卡片中点击 **"Disconnect"**

**预期结果：**
- ✅ 按钮恢复为 **"Connect Wallet"**
- ✅ 按钮颜色恢复为橙色（accent color）
- ✅ 下拉菜单/信息卡片消失
- ✅ localStorage 中的连接状态被清除
- ✅ 刷新页面后不会自动重连

---

### 场景 6: 复制地址

**步骤：**
1. 连接钱包
2. 打开钱包菜单
3. 点击 **"Copy Address"**

**预期结果：**
- ✅ 弹出提示 "Copy Address ✓"
- ✅ 地址已复制到剪贴板
- ✅ 可以粘贴到其他地方验证

---

### 场景 7: 在 BscScan 查看

**步骤：**
1. 连接钱包
2. 打开钱包菜单
3. 点击 **"View on BscScan"**

**预期结果：**
- ✅ 新标签页打开
- ✅ 跳转到 BscScan 地址页面
- ✅ URL 包含正确的钱包地址
- ✅ 如果是主网，URL 是 `https://bscscan.com/address/...`
- ✅ 如果是测试网，URL 是 `https://testnet.bscscan.com/address/...`

---

### 场景 8: 网络切换（手动）

**步骤：**
1. 连接钱包（BSC 网络）
2. 打开 MetaMask
3. 手动切换到其他网络（如 Polygon）

**预期结果：**
- ✅ 页面自动刷新
- ✅ 刷新后检测到错误网络
- ✅ 显示 "Switch to BSC" 按钮

---

### 场景 9: 没有安装 MetaMask

**步骤：**
1. 在没有安装 MetaMask 的浏览器中打开应用
2. 点击 **"Connect Wallet"**

**预期结果：**
- ✅ 弹出错误提示
- ✅ 自动打开 MetaMask 下载页面
- ✅ 应用不会崩溃

---

### 场景 10: 拒绝连接

**步骤：**
1. 点击 **"Connect Wallet"**
2. 在 MetaMask 弹窗中点击 **"Cancel"** 或 **"Reject"**

**预期结果：**
- ✅ 按钮保持 "Connect Wallet" 状态
- ✅ 显示错误提示（可选）
- ✅ 应用不会崩溃
- ✅ 可以再次尝试连接

---

## 🎨 UI/UX 测试

### 桌面端（> 768px）
- ✅ 钱包按钮在导航栏右侧
- ✅ 下拉菜单位置正确（右对齐）
- ✅ 下拉菜单宽度合适（72 = 288px）
- ✅ Hover 效果流畅
- ✅ 点击外部区域关闭菜单

### 移动端（< 768px）
- ✅ 钱包按钮在移动菜单中
- ✅ 信息卡片布局清晰
- ✅ 按钮大小适合触摸
- ✅ 文字大小可读
- ✅ 滚动流畅

### 深色/浅色主题
- ✅ 浅色主题下所有元素可见
- ✅ 深色主题下所有元素可见
- ✅ 对比度足够
- ✅ 切换主题时钱包状态保持

### 中英文切换
- ✅ 所有文本正确翻译
- ✅ 按钮文字长度合适
- ✅ 布局不会因文字长度变化而破坏

---

## 🐛 已知问题和限制

### 当前限制
1. **JCC 余额显示为 0**
   - 原因：合约地址未配置
   - 解决：在 `src/services/web3.js` 中设置正确的 JCC 代币地址

2. **仅支持 MetaMask**
   - 其他钱包（WalletConnect、OKX）尚未集成
   - 计划在后续版本中添加

3. **网络切换会刷新页面**
   - 这是 MetaMask 推荐的做法
   - 确保状态一致性

---

## 📊 测试检查清单

复制以下清单用于测试：

```
[ ] 场景 1: 首次连接钱包
[ ] 场景 2: 错误网络检测
[ ] 场景 3: 自动重连
[ ] 场景 4: 账户切换
[ ] 场景 5: 断开连接
[ ] 场景 6: 复制地址
[ ] 场景 7: 在 BscScan 查看
[ ] 场景 8: 网络切换（手动）
[ ] 场景 9: 没有安装 MetaMask
[ ] 场景 10: 拒绝连接

UI/UX:
[ ] 桌面端布局
[ ] 移动端布局
[ ] 浅色主题
[ ] 深色主题
[ ] 中文界面
[ ] 英文界面
```

---

## 🔍 调试技巧

### 查看控制台日志
打开浏览器开发者工具（F12），查看：
- 连接成功/失败的日志
- 账户变化的日志
- 网络变化的日志
- 错误信息

### 查看 localStorage
在控制台输入：
```javascript
localStorage.getItem('walletConnected')
localStorage.getItem('walletAddress')
```

### 清除连接状态
在控制台输入：
```javascript
localStorage.removeItem('walletConnected')
localStorage.removeItem('walletAddress')
location.reload()
```

---

## ✅ 测试完成标准

所有场景测试通过 ✓
- 桌面端 UI 正常 ✓
- 移动端 UI 正常 ✓
- 深色/浅色主题正常 ✓
- 中英文切换正常 ✓
- 无控制台错误 ✓
- 无 UI 布局问题 ✓

