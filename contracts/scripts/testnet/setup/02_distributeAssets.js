/**
 * 分发资产 (BNB 和 JCC)
 * 从 deployer 账户分发测试资产到各个测试账户
 */

const { ethers } = require("hardhat");
const fs = require("fs");
const path = require("path");

async function main() {
    console.log("\n" + "═".repeat(80));
    console.log("                    分发测试资产");
    console.log("═".repeat(80) + "\n");

    // 加载配置
    const accountsConfig = require("../config/accounts.json");
    const contractsConfig = require("../config/contracts.json");
    const testParams = require("../config/testParams.json");

    // 获取 deployer 账户
    const [deployer] = await ethers.getSigners();
    console.log(`📍 Deployer 地址: ${deployer.address}\n`);

    // 验证 deployer 地址
    if (deployer.address.toLowerCase() !== contractsConfig.deployer.address.toLowerCase()) {
        throw new Error(
            `Deployer 地址不匹配!\n` +
            `  配置: ${contractsConfig.deployer.address}\n` +
            `  实际: ${deployer.address}`
        );
    }

    // 检查 deployer 余额
    const deployerBNB = await ethers.provider.getBalance(deployer.address);
    console.log(`💰 Deployer BNB 余额: ${ethers.formatEther(deployerBNB)}`);

    const jccToken = await ethers.getContractAt(
        "JCC",
        contractsConfig.contracts.jccToken
    );
    const deployerJCC = await jccToken.balanceOf(deployer.address);
    console.log(`💰 Deployer JCC 余额: ${ethers.formatEther(deployerJCC)}\n`);

    // 计算所需资产
    const bnbDistribution = testParams.bnbDistribution;
    const jccDistribution = testParams.jccDistribution;

    const totalBNBNeeded = ethers.parseEther(
        (parseFloat(bnbDistribution.projectOwner) +
        parseFloat(bnbDistribution.feeReceiver) +
        parseFloat(bnbDistribution.users) * 5 +
        0.0015).toString() // 加上转账 gas 费用
    );

    const totalJCCNeeded = ethers.parseEther(
        (parseFloat(jccDistribution.perUser) * 5).toString()
    );

    console.log("📊 所需资产:");
    console.log(`   BNB: ${ethers.formatEther(totalBNBNeeded)}`);
    console.log(`   JCC: ${ethers.formatEther(totalJCCNeeded)}\n`);

    // 验证余额充足
    if (deployerBNB < totalBNBNeeded) {
        throw new Error(
            `BNB 余额不足!\n` +
            `  需要: ${ethers.formatEther(totalBNBNeeded)}\n` +
            `  拥有: ${ethers.formatEther(deployerBNB)}`
        );
    }

    if (deployerJCC < totalJCCNeeded) {
        throw new Error(
            `JCC 余额不足!\n` +
            `  需要: ${ethers.formatEther(totalJCCNeeded)}\n` +
            `  拥有: ${ethers.formatEther(deployerJCC)}`
        );
    }

    console.log("✅ 余额验证通过\n");

    // 准备分发列表
    const distributions = [
        {
            name: "projectOwner",
            address: accountsConfig.accounts.projectOwner.address,
            bnb: bnbDistribution.projectOwner,
            jcc: "0"
        },
        {
            name: "feeReceiver",
            address: accountsConfig.accounts.feeReceiver.address,
            bnb: bnbDistribution.feeReceiver,
            jcc: "0"
        },
        {
            name: "user1",
            address: accountsConfig.accounts.user1.address,
            bnb: bnbDistribution.users,
            jcc: jccDistribution.perUser
        },
        {
            name: "user2",
            address: accountsConfig.accounts.user2.address,
            bnb: bnbDistribution.users,
            jcc: jccDistribution.perUser
        },
        {
            name: "user3",
            address: accountsConfig.accounts.user3.address,
            bnb: bnbDistribution.users,
            jcc: jccDistribution.perUser
        },
        {
            name: "user4",
            address: accountsConfig.accounts.user4.address,
            bnb: bnbDistribution.users,
            jcc: jccDistribution.perUser
        },
        {
            name: "user5",
            address: accountsConfig.accounts.user5.address,
            bnb: bnbDistribution.users,
            jcc: jccDistribution.perUser
        }
    ];

    console.log("🚀 开始分发资产...\n");

    const results = [];

    // 分发 BNB
    console.log("📤 分发 BNB:");
    for (const dist of distributions) {
        const bnbAmount = ethers.parseEther(dist.bnb);

        if (bnbAmount > 0n) {
            console.log(`   → ${dist.name}: ${dist.bnb} BNB`);

            const tx = await deployer.sendTransaction({
                to: dist.address,
                value: bnbAmount
            });

            const receipt = await tx.wait(2);
            console.log(`     ✅ 交易确认: ${receipt.hash}`);

            results.push({
                type: "BNB",
                to: dist.name,
                address: dist.address,
                amount: dist.bnb,
                hash: receipt.hash
            });
        }
    }

    console.log("\n📤 分发 JCC:");
    for (const dist of distributions) {
        const jccAmount = ethers.parseEther(dist.jcc);

        if (jccAmount > 0n) {
            console.log(`   → ${dist.name}: ${dist.jcc} JCC`);

            const tx = await jccToken.transfer(dist.address, jccAmount);
            const receipt = await tx.wait(2);
            console.log(`     ✅ 交易确认: ${receipt.hash}`);

            results.push({
                type: "JCC",
                to: dist.name,
                address: dist.address,
                amount: dist.jcc,
                hash: receipt.hash
            });
        }
    }

    console.log("\n" + "─".repeat(80));
    console.log("\n✅ 资产分发完成!\n");

    // 验证分发结果
    console.log("🔍 验证账户余额:\n");

    for (const dist of distributions) {
        const bnbBalance = await ethers.provider.getBalance(dist.address);
        const jccBalance = await jccToken.balanceOf(dist.address);

        console.log(`${dist.name}:`);
        console.log(`   地址: ${dist.address}`);
        console.log(`   BNB: ${ethers.formatEther(bnbBalance)}`);
        console.log(`   JCC: ${ethers.formatEther(jccBalance)}`);

        // 验证余额是否正确
        const expectedBNB = ethers.parseEther(dist.bnb);
        const expectedJCC = ethers.parseEther(dist.jcc);

        if (bnbBalance < expectedBNB) {
            console.log(`   ⚠️  BNB 余额不足`);
        }

        if (jccBalance < expectedJCC) {
            console.log(`   ⚠️  JCC 余额不足`);
        }

        console.log("");
    }

    // 检查 deployer 剩余余额
    const finalBNB = await ethers.provider.getBalance(deployer.address);
    const finalJCC = await jccToken.balanceOf(deployer.address);

    console.log("─".repeat(80));
    console.log("\n💰 Deployer 剩余余额:");
    console.log(`   BNB: ${ethers.formatEther(finalBNB)}`);
    console.log(`   JCC: ${ethers.formatEther(finalJCC)}\n`);

    // 保存分发记录
    const recordPath = path.join(__dirname, "../reports/distribution.json");
    const record = {
        timestamp: new Date().toISOString(),
        deployer: deployer.address,
        distributions: results,
        finalBalances: {
            deployer: {
                bnb: ethers.formatEther(finalBNB),
                jcc: ethers.formatEther(finalJCC)
            }
        }
    };

    const dir = path.dirname(recordPath);
    if (!fs.existsSync(dir)) {
        fs.mkdirSync(dir, { recursive: true });
    }

    fs.writeFileSync(recordPath, JSON.stringify(record, null, 2));
    console.log(`💾 分发记录已保存: ${recordPath}\n`);

    console.log("═".repeat(80) + "\n");
}

main()
    .then(() => process.exit(0))
    .catch(error => {
        console.error("\n❌ 分发失败:", error.message);
        console.error(error);
        process.exit(1);
    });
