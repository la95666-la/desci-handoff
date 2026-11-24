# BNB Chain 钱包生成脚本

## 📋 概述

这个脚本用于生成 BNB Chain 兼容的以太坊钱包地址，包括公钥、私钥和助记词。

## 🚀 使用方法

### 1. 生成单个钱包

```bash
npm run generate:wallet
# 或
node scripts/generateWallet.js
```

### 2. 生成多个钱包

```bash
# 生成 5 个钱包
npm run generate:wallet 5
# 或
node scripts/generateWallet.js 5
```

### 3. 生成钱包并保存到文件

```bash
# 生成 3 个钱包并保存到 wallets.json
npm run generate:wallet 3 save
# 或
node scripts/generateWallet.js 3 save
```

## 📊 输出信息

每个生成的钱包包含以下信息：

```
📍 公钥地址 (Address)
   - 用于接收资金的地址
   - 可以公开分享

🔑 私钥 (Private Key)
   - 用于签署交易
   - ⚠️ 绝对不能分享或暴露

📝 助记词 (Mnemonic)
   - 12 个单词的恢复短语
   - 可以恢复钱包
   - ⚠️ 需要妥善保管

🛣️ 派生路径 (Derivation Path)
   - 钱包的派生路径信息
```

## 💾 保存到文件

使用 `save` 参数时，钱包信息会保存到 `scripts/wallets.json`：

```json
{
  "generatedAt": "2026-02-12T16:17:48.368Z",
  "count": 2,
  "wallets": [
    {
      "index": 1,
      "address": "<钱包地址>",
      "privateKey": "<私钥，勿分享>",
      "mnemonic": "<助记词，勿分享>"
    }
  ]
}
```

## ⚠️ 安全提示

### 🔒 私钥安全

1. **绝不分享私钥**
   - 不要在任何地方暴露私钥
   - 不要通过邮件、聊天或社交媒体分享
   - 不要上传到版本控制系统

2. **妥善保管**
   - 将私钥保存在安全的地方
   - 考虑使用硬件钱包
   - 定期备份助记词

3. **文件安全**
   - `wallets.json` 已添加到 `.gitignore`
   - 不要将其提交到 Git
   - 定期删除不需要的钱包文件

### 🛡️ 最佳实践

1. **测试网络**
   - 在 BSC 测试网上测试钱包
   - 不要在主网上使用测试钱包

2. **多签钱包**
   - 对于大额资金，考虑使用多签钱包
   - 例如：Gnosis Safe

3. **冷存储**
   - 对于长期存储，使用冷钱包
   - 例如：Ledger、Trezor

## 🔗 BNB Chain 网络信息

### BSC 测试网

- **网络名称**: BSC Testnet
- **RPC URL**: https://data-seed-prebsc-1-s1.binance.org:8545
- **Chain ID**: 97
- **符号**: tBNB
- **浏览器**: https://testnet.bscscan.com

### BSC 主网

- **网络名称**: Binance Smart Chain
- **RPC URL**: https://bsc-dataseed.binance.org
- **Chain ID**: 56
- **符号**: BNB
- **浏览器**: https://bscscan.com

## 📝 示例

### 生成单个钱包

```bash
$ npm run generate:wallet

🔐 正在生成 1 个 BNB Chain 钱包...

╔════════════════════════════════════════════════════════════════╗
║                    钱包 #1
╚════════════════════════════════════════════════════════════════╝

📍 公钥地址 (Address):
   0xE2278679979979cd93Eb27384A8fA8D6cfE8aEaa

🔑 私钥 (Private Key):
   0x10da3e16c75c52ac904bdf53a5d6d8aa64cc709fc656d904989f76526388a49a

📝 助记词 (Mnemonic):
   prosper humor anxiety naive desk sphere there intact vault hurry run clean
```

### 生成多个钱包并保存

```bash
$ npm run generate:wallet 3 save

🔐 正在生成 3 个 BNB Chain 钱包...

[钱包信息输出...]

✅ 钱包信息已保存到: scripts/wallets.json
⚠️  请妥善保管此文件，不要上传到版本控制系统！
```

## 🔧 技术细节

### 依赖

- **ethers.js**: 用于生成以太坊兼容的钱包
- **Node.js**: 运行环境

### 钱包生成方式

1. 使用 `ethers.Wallet.createRandom()` 生成随机钱包
2. 每个钱包都是独立的，使用不同的随机数生成
3. 钱包遵循 BIP-39 标准（助记词）
4. 钱包遵循 BIP-44 标准（派生路径）

### 兼容性

- ✅ BNB Chain (BSC)
- ✅ 以太坊 (Ethereum)
- ✅ 所有 EVM 兼容链

## ❓ 常见问题

### Q: 生成的钱包可以在 MetaMask 中使用吗？

**A**: 是的，可以。你可以使用私钥或助记词导入到 MetaMask：
1. 打开 MetaMask
2. 点击"导入账户"
3. 选择"私钥"或"助记词"
4. 粘贴相应的信息

### Q: 钱包地址是否可以重复？

**A**: 理论上可能，但概率极低（约 1/2^160）。每次生成都会创建一个新的随机钱包。

### Q: 如何恢复钱包？

**A**: 使用助记词可以恢复钱包：
1. 在任何支持 BIP-39 的钱包中导入助记词
2. 钱包会自动恢复所有派生账户

### Q: 生成的钱包在哪个网络上有效？

**A**: 生成的钱包在所有 EVM 兼容链上都有效，包括：
- BNB Chain (BSC)
- 以太坊
- Polygon
- Arbitrum
- 等等

## 📞 支持

如有问题，请参考：
- [ethers.js 文档](https://docs.ethers.org/)
- [BNB Chain 文档](https://docs.bnbchain.org/)
- [MetaMask 帮助](https://support.metamask.io/)
