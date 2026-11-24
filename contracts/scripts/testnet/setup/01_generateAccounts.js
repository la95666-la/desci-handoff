/**
 * 生成测试账户
 * 使用现有的 generateWallet.js 生成 7 个测试账户
 */

const { ethers } = require("ethers");
const fs = require("fs");
const path = require("path");

/**
 * 生成单个钱包
 */
function generateWallet(role, index = 0) {
    const wallet = ethers.Wallet.createRandom();

    return {
        role,
        index,
        address: wallet.address,
        privateKey: wallet.privateKey,
        mnemonic: wallet.mnemonic.phrase
    };
}

/**
 * 主函数
 */
async function main() {
    console.log("\n" + "═".repeat(80));
    console.log("                    生成测试账户");
    console.log("═".repeat(80) + "\n");

    // 定义需要的账户角色
    const roles = [
        { role: "projectOwner", description: "项目方" },
        { role: "feeReceiver", description: "平台费用接收者" },
        { role: "user1", description: "测试用户 1" },
        { role: "user2", description: "测试用户 2" },
        { role: "user3", description: "测试用户 3" },
        { role: "user4", description: "测试用户 4" },
        { role: "user5", description: "测试用户 5" }
    ];

    console.log(`🔐 正在生成 ${roles.length} 个测试账户...\n`);

    const accounts = {};

    // 生成账户
    for (let i = 0; i < roles.length; i++) {
        const { role, description } = roles[i];
        const wallet = generateWallet(role, i + 1);

        accounts[role] = {
            role,
            description,
            address: wallet.address,
            privateKey: wallet.privateKey,
            mnemonic: wallet.mnemonic
        };

        console.log(`✅ ${i + 1}. ${description} (${role})`);
        console.log(`   地址: ${wallet.address}`);
        console.log(`   私钥: ${wallet.privateKey.slice(0, 20)}...`);
        console.log("");
    }

    // 添加部署者账户信息（从配置文件读取）
    const contractsConfig = require("../config/contracts.json");
    accounts.deployer = {
        role: "deployer",
        description: "合约部署者和资产分发者",
        address: contractsConfig.deployer.address,
        privateKey: "*** 请从环境变量或安全存储中获取 ***",
        note: "此账户已存在，拥有 1,000,000 JCC 和 0.83 BNB"
    };

    // 保存到配置文件
    const configPath = path.join(__dirname, "../config/accounts.json");
    const config = {
        generatedAt: new Date().toISOString(),
        network: "bsc-testnet",
        chainId: 97,
        accounts
    };

    fs.writeFileSync(configPath, JSON.stringify(config, null, 2));

    console.log("─".repeat(80));
    console.log(`\n✅ 成功生成 ${roles.length} 个测试账户`);
    console.log(`💾 配置已保存: ${configPath}`);

    console.log("\n⚠️  安全提示:");
    console.log("   • 请妥善保管私钥，不要分享给任何人");
    console.log("   • 不要将 accounts.json 上传到版本控制系统");
    console.log("   • 这些账户仅用于测试网测试");

    console.log("\n📋 下一步:");
    console.log("   1. 运行 02_distributeAssets.js 分发 BNB 和 JCC");
    console.log("   2. 或手动从水龙头获取 BNB: https://testnet.bnbchain.org/faucet-smart");

    console.log("\n" + "═".repeat(80) + "\n");

    // 打印账户摘要
    console.log("📊 账户摘要:\n");
    console.log("角色".padEnd(20) + "地址");
    console.log("─".repeat(80));
    for (const [key, account] of Object.entries(accounts)) {
        if (key !== "deployer") {
            console.log(`${account.description.padEnd(20)} ${account.address}`);
        }
    }
    console.log("─".repeat(80) + "\n");
}

// 运行主函数
main()
    .then(() => process.exit(0))
    .catch(error => {
        console.error("\n❌ 生成账户失败:", error.message);
        process.exit(1);
    });
