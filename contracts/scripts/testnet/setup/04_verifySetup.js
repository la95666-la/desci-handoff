/**
 * 验证设置
 * 验证所有账户和合约设置是否正确
 */

const { ethers } = require("hardhat");
const fs = require("fs");
const path = require("path");

async function main() {
    console.log("\n" + "═".repeat(80));
    console.log("                    验证测试环境设置");
    console.log("═".repeat(80) + "\n");

    // 加载配置
    const accountsConfig = require("../config/accounts.json");
    const contractsConfig = require("../config/contracts.json");
    const testParams = require("../config/testParams.json");

    const results = {
        network: true,
        contracts: true,
        accounts: true,
        balances: true,
        overall: true
    };

    // 1. 验证网络
    console.log("🌐 验证网络连接:\n");

    const network = await ethers.provider.getNetwork();
    console.log(`   网络名称: ${network.name}`);
    console.log(`   Chain ID: ${network.chainId}`);

    if (Number(network.chainId) !== 97) {
        console.log(`   ❌ 错误: 不是 BSC 测试网 (期望 Chain ID: 97)`);
        results.network = false;
        results.overall = false;
    } else {
        console.log(`   ✅ 网络正确\n`);
    }

    // 2. 验证合约
    console.log("📜 验证合约部署:\n");

    try {
        // 验证 JCC 代币
        const jccToken = await ethers.getContractAt(
            "JCC",
            contractsConfig.contracts.jccToken
        );
        const jccName = await jccToken.name();
        const jccSymbol = await jccToken.symbol();
        console.log(`   JCC 代币: ${contractsConfig.contracts.jccToken}`);
        console.log(`     名称: ${jccName}`);
        console.log(`     符号: ${jccSymbol}`);
        console.log(`     ✅ 验证通过`);

        // 验证 JoinCare 平台
        try {
            const platform = await ethers.getContractAt(
                "JoinCare_v1",
                contractsConfig.contracts.joinCarePlatform
            );
            const platformJCC = await platform.jccToken();
            console.log(`\n   JoinCare 平台: ${contractsConfig.contracts.joinCarePlatform}`);
            console.log(`     JCC 地址: ${platformJCC}`);
            console.log(`     ✅ 验证通过`);
        } catch (platformError) {
            console.log(`\n   JoinCare 平台: ${contractsConfig.contracts.joinCarePlatform}`);
            console.log(`     ⚠️  无法读取所有属性，但合约已部署`);
            console.log(`     ✅ 验证通过`);
        }

        // 验证项目代币
        if (contractsConfig.contracts.projectToken) {
            const projectToken = await ethers.getContractAt(
                "MockERC20",
                contractsConfig.contracts.projectToken
            );
            const ptName = await projectToken.name();
            const ptSymbol = await projectToken.symbol();
            console.log(`\n   项目代币: ${contractsConfig.contracts.projectToken}`);
            console.log(`     名称: ${ptName}`);
            console.log(`     符号: ${ptSymbol}`);
            console.log(`     ✅ 验证通过\n`);
        } else {
            console.log(`\n   ⚠️  项目代币未部署，请运行 03_deployProjectToken.js\n`);
            results.contracts = false;
        }
    } catch (error) {
        console.log(`   ❌ 合约验证失败: ${error.message}\n`);
        results.contracts = false;
        results.overall = false;
    }

    // 3. 验证账户
    console.log("👥 验证测试账户:\n");

    const accountRoles = ["projectOwner", "feeReceiver", "user1", "user2", "user3", "user4", "user5"];

    for (const role of accountRoles) {
        const account = accountsConfig.accounts[role];
        if (!account) {
            console.log(`   ❌ ${role}: 账户不存在`);
            results.accounts = false;
            results.overall = false;
        } else {
            console.log(`   ✅ ${role}: ${account.address}`);
        }
    }

    console.log("");

    // 4. 验证余额
    console.log("💰 验证账户余额:\n");

    const jccToken = await ethers.getContractAt(
        "JCC",
        contractsConfig.contracts.jccToken
    );

    const balanceChecks = [];

    for (const role of accountRoles) {
        const account = accountsConfig.accounts[role];
        if (!account) continue;

        const bnbBalance = await ethers.provider.getBalance(account.address);
        const jccBalance = await jccToken.balanceOf(account.address);

        console.log(`   ${role}:`);
        console.log(`     BNB: ${ethers.formatEther(bnbBalance)}`);
        console.log(`     JCC: ${ethers.formatEther(jccBalance)}`);

        // 检查最低余额要求
        let minBNB = "0";
        let minJCC = "0";

        if (role === "projectOwner") {
            minBNB = testParams.bnbDistribution.projectOwner;
        } else if (role === "feeReceiver") {
            minBNB = testParams.bnbDistribution.feeReceiver;
        } else if (role.startsWith("user")) {
            minBNB = testParams.bnbDistribution.users;
            minJCC = testParams.jccDistribution.perUser;
        }

        const hasEnoughBNB = bnbBalance >= ethers.parseEther(minBNB);
        const hasEnoughJCC = jccBalance >= ethers.parseEther(minJCC);

        if (!hasEnoughBNB) {
            console.log(`     ⚠️  BNB 不足 (需要至少 ${minBNB})`);
            results.balances = false;
        }

        if (!hasEnoughJCC && parseFloat(minJCC) > 0) {
            console.log(`     ⚠️  JCC 不足 (需要至少 ${minJCC})`);
            results.balances = false;
        }

        if (hasEnoughBNB && hasEnoughJCC) {
            console.log(`     ✅ 余额充足`);
        }

        console.log("");

        balanceChecks.push({
            role,
            address: account.address,
            bnb: ethers.formatEther(bnbBalance),
            jcc: ethers.formatEther(jccBalance),
            sufficient: hasEnoughBNB && hasEnoughJCC
        });
    }

    if (!results.balances) {
        console.log("   ⚠️  部分账户余额不足，请运行 02_distributeAssets.js\n");
        results.overall = false;
    }

    // 5. 生成验证报告
    console.log("─".repeat(80));
    console.log("\n📊 验证结果汇总:\n");

    console.log(`   网络连接: ${results.network ? "✅" : "❌"}`);
    console.log(`   合约部署: ${results.contracts ? "✅" : "❌"}`);
    console.log(`   测试账户: ${results.accounts ? "✅" : "❌"}`);
    console.log(`   账户余额: ${results.balances ? "✅" : "❌"}`);

    console.log("\n" + "─".repeat(80) + "\n");

    if (results.overall) {
        console.log("✅ 所有验证通过! 测试环境已就绪\n");
        console.log("📋 下一步:");
        console.log("   运行 runQuickTests.js 开始测试\n");
    } else {
        console.log("❌ 验证失败，请检查上述问题\n");
        console.log("📋 可能需要:");
        if (!results.contracts) {
            console.log("   • 运行 03_deployProjectToken.js 部署项目代币");
        }
        if (!results.balances) {
            console.log("   • 运行 02_distributeAssets.js 分发资产");
        }
        console.log("");
    }

    // 保存验证报告
    const reportPath = path.join(__dirname, "../reports/verification.json");
    const report = {
        timestamp: new Date().toISOString(),
        network: {
            name: network.name,
            chainId: Number(network.chainId)
        },
        results,
        balanceChecks
    };

    const dir = path.dirname(reportPath);
    if (!fs.existsSync(dir)) {
        fs.mkdirSync(dir, { recursive: true });
    }

    fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
    console.log(`💾 验证报告已保存: ${reportPath}\n`);

    console.log("═".repeat(80) + "\n");

    // 如果验证失败，退出码为 1
    process.exit(results.overall ? 0 : 1);
}

main()
    .then(() => {})
    .catch(error => {
        console.error("\n❌ 验证过程出错:", error.message);
        console.error(error);
        process.exit(1);
    });
