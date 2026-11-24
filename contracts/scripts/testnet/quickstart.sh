#!/bin/bash

# BSC 测试网测试 - 快速启动脚本
# 此脚本将引导您完成整个测试流程

set -euo pipefail  # 遇到错误立即退出

echo "═══════════════════════════════════════════════════════════════════════════════"
echo "                    BSC 测试网测试 - 快速启动向导"
echo "═══════════════════════════════════════════════════════════════════════════════"
echo ""

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 检查网络参数
NETWORK="${NETWORK:-jcTarget}"
if [ "$NETWORK" = "jcTarget" ]; then
    : "${JC_DEPLOY_RPC:?使用 jcTarget 时必须设置 JC_DEPLOY_RPC}"
    : "${JC_DEPLOY_CHAIN_ID:?使用 jcTarget 时必须设置 JC_DEPLOY_CHAIN_ID}"
    : "${JC_DEPLOY_PRIVATE_KEY:?使用 jcTarget 时必须设置 JC_DEPLOY_PRIVATE_KEY}"
fi

echo "📋 测试流程概览:"
echo ""
echo "   1. 生成测试账户 (7 个)"
echo "   2. 分发 BNB 和 JCC"
echo "   3. 部署项目代币"
echo "   4. 验证设置"
echo "   5. 运行完整测试 (约 30 分钟)"
echo ""
echo "───────────────────────────────────────────────────────────────────────────────"
echo ""

# 步骤 1: 生成测试账户
echo "步骤 1/5: 生成测试账户"
echo ""
read -p "是否生成测试账户? (y/n) " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "🔐 正在生成测试账户..."
    npx hardhat run scripts/testnet/setup/01_generateAccounts.js --network $NETWORK
    echo ""
    echo -e "${GREEN}✅ 测试账户生成完成${NC}"
    echo ""
else
    echo "⏭️  跳过生成测试账户"
    echo ""
fi

# 步骤 2: 分发资产
echo "───────────────────────────────────────────────────────────────────────────────"
echo ""
echo "步骤 2/5: 分发 BNB 和 JCC"
echo ""
echo -e "${YELLOW}⚠️  注意: 此步骤需要 deployer 账户有足够的 BNB 和 JCC${NC}"
echo "   需要: 0.83 BNB 和 1,000,000 JCC"
echo ""
read -p "是否分发资产? (y/n) " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "💰 正在分发资产..."
    npx hardhat run scripts/testnet/setup/02_distributeAssets.js --network $NETWORK
    echo ""
    echo -e "${GREEN}✅ 资产分发完成${NC}"
    echo ""
else
    echo "⏭️  跳过资产分发"
    echo ""
fi

# 步骤 3: 部署项目代币
echo "───────────────────────────────────────────────────────────────────────────────"
echo ""
echo "步骤 3/5: 部署项目代币"
echo ""
read -p "是否部署项目代币? (y/n) " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "🚀 正在部署项目代币..."
    npx hardhat run scripts/testnet/setup/03_deployProjectToken.js --network $NETWORK
    echo ""
    echo -e "${GREEN}✅ 项目代币部署完成${NC}"
    echo ""
else
    echo "⏭️  跳过项目代币部署"
    echo ""
fi

# 步骤 4: 验证设置
echo "───────────────────────────────────────────────────────────────────────────────"
echo ""
echo "步骤 4/5: 验证设置"
echo ""
read -p "是否验证设置? (y/n) " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "🔍 正在验证设置..."
    npx hardhat run scripts/testnet/setup/04_verifySetup.js --network $NETWORK
    echo ""
    echo -e "${GREEN}✅ 设置验证完成${NC}"
    echo ""
else
    echo "⏭️  跳过设置验证"
    echo ""
fi

# 步骤 5: 运行完整测试
echo "───────────────────────────────────────────────────────────────────────────────"
echo ""
echo "步骤 5/5: 运行完整测试"
echo ""
echo -e "${YELLOW}⚠️  注意: 当前参数下完整测试约耗时 30 分钟${NC}"
echo "   包含时间依赖的测试（vesting 等待）"
echo ""
read -p "是否运行完整测试? (y/n) " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "🧪 正在运行完整测试..."
    echo ""
    npx hardhat run scripts/testnet/runFullTest.js --network "$NETWORK"
    echo ""
    echo -e "${GREEN}✅ 测试完成${NC}"
    echo ""
    echo "📊 测试报告已保存到 scripts/testnet/reports/"
    echo ""
else
    echo "⏭️  跳过完整测试"
    echo ""
fi

echo "═══════════════════════════════════════════════════════════════════════════════"
echo ""
echo "🎉 测试流程完成!"
echo ""
echo "📁 查看测试报告:"
echo "   • scripts/testnet/reports/fullTestReport.md"
echo "   • scripts/testnet/reports/gasReport.json"
echo ""
echo "🔗 在 BscScan 上查看交易:"
echo "   https://testnet.bscscan.com"
echo ""
echo "═══════════════════════════════════════════════════════════════════════════════"
