/**
 * 测试模块 7: 代币领取
 * 测试用户领取代币功能（时间依赖）
 */

const { ethers } = require("hardhat");
const ContractHelper = require("../utils/contractHelper");
const VerificationHelper = require("../utils/verificationHelper");
const TimeHelper = require("../utils/timeHelper");
const GasTracker = require("../utils/gasTracker");

async function runClaimTests() {
    console.log("\n" + "═".repeat(80));
    console.log("                    测试模块 7: 代币领取");
    console.log("═".repeat(80) + "\n");

    const accountsConfig = require("../config/accounts.json");
    const fs = require("fs");
    const path = require("path");

    // 加载项目信息
    const projectInfoPath = path.join(__dirname, "../reports/projectInfo.json");
    if (!fs.existsSync(projectInfoPath)) {
        throw new Error("项目信息文件不存在，请先运行 02_projectCreation.test.js");
    }
    const projectInfo = JSON.parse(fs.readFileSync(projectInfoPath, "utf8"));
    const projectId = BigInt(projectInfo.projectId);

    const [deployer] = await ethers.getSigners();

    // 创建用户钱包
    const users = [];
    for (let i = 1; i <= 5; i++) {
        const userKey = `user${i}`;
        const privateKey = accountsConfig.accounts[userKey].privateKey;
        const wallet = new ethers.Wallet(privateKey, ethers.provider);
        users.push({ name: userKey, wallet });
    }

    const contractHelper = new ContractHelper(ethers.provider);
    await contractHelper.initContracts(deployer);

    const verifier = new VerificationHelper(contractHelper);
    const timeHelper = new TimeHelper();
    const gasTracker = new GasTracker();

    const { platform, projectToken } = contractHelper.contracts;

    console.log(`📊 项目信息:`);
    console.log(`   项目 ID: ${projectId}`);
    console.log(`\n📅 Vesting 时间表:`);
    for (let i = 0; i < projectInfo.schedule.vestingTimes.length; i++) {
        const vestingTime = projectInfo.schedule.vestingTimes[i];
        const percent = projectInfo.schedule.vestingPercents[i] / 100;
        console.log(`   阶段 ${i + 1}: ${timeHelper.formatTimestamp(vestingTime)} (${percent}%)`);
    }
    console.log("");

    // 测试 1: 等待第一次 vesting 时间
    console.log("📝 测试 1: 等待第一次 vesting 时间\n");

    try {
        const now = timeHelper.now();
        const firstVestingTime = projectInfo.schedule.vestingTimes[0];

        if (now < firstVestingTime) {
            console.log(`   当前时间: ${timeHelper.formatTimestamp(now)}`);
            console.log(`   第一次 vesting: ${timeHelper.formatTimestamp(firstVestingTime)}`);
            console.log(`   需要等待: ${timeHelper.formatDuration(firstVestingTime - now)}\n`);

            await timeHelper.waitUntil(firstVestingTime);
        } else {
            console.log(`   ✅ 已到达第一次 vesting 时间\n`);
        }

        verifier.recordTest(
            "代币领取",
            "等待第一次 vesting 时间",
            true,
            "等待完成"
        );
    } catch (error) {
        verifier.recordTest(
            "代币领取",
            "等待第一次 vesting 时间",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 2: 用户 1 领取第一次代币
    console.log("📝 测试 2: 用户 1 领取第一次代币\n");

    try {
        const user1 = users[0];

        // 获取认购信息
        const purchase = await contractHelper.getUserPurchase(projectId, user1.wallet.address);
		console.log(`   总认购: ${ethers.formatEther(purchase.totalTokensBought)} PT`);
		console.log(`   已领取: ${ethers.formatEther(purchase.claimedTokens)} PT`);

		// 计算第一次可领取数量（按本次项目真实 vesting 配置）。
		const expectedClaim = BigInt(purchase.totalTokensBought) * BigInt(projectInfo.schedule.vestingPercents[0]) / 10000n;
        console.log(`   预期领取: ${ethers.formatEther(expectedClaim)} PT`);

        const balanceBefore = await projectToken.balanceOf(user1.wallet.address);

        const tx = await platform.connect(user1.wallet).claim(projectId);
        const receipt = await contractHelper.waitForTx(tx);
        gasTracker.recordTransaction(`${user1.name} 第一次领取`, receipt, "代币领取");

        const balanceAfter = await projectToken.balanceOf(user1.wallet.address);
        const claimed = balanceAfter - balanceBefore;

        console.log(`   实际领取: ${ethers.formatEther(claimed)} PT`);

        // 允许 1 wei 的误差
        const diff = claimed > expectedClaim ? claimed - expectedClaim : expectedClaim - claimed;
        const passed = diff <= 1n;

        verifier.recordTest(
            "代币领取",
            "用户领取第一次代币成功",
            passed,
            passed ? `领取: ${ethers.formatEther(claimed)} PT` : `期望: ${ethers.formatEther(expectedClaim)}, 实际: ${ethers.formatEther(claimed)}`
        );
    } catch (error) {
        verifier.recordTest(
            "代币领取",
            "用户领取第一次代币成功",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

	// 测试 3: 验证 Claimed 事件
	console.log("📝 测试 3: 验证 Claimed 事件\n");

    try {
        const user1 = users[0];
		const filter = platform.filters.Claimed(projectId, user1.wallet.address);
        const events = await platform.queryFilter(filter);

        const passed = events.length > 0;

        if (passed) {
            const event = events[events.length - 1]; // 最新的事件
            console.log(`   事件参数:`);
            console.log(`     项目 ID: ${event.args.projectId}`);
            console.log(`     用户: ${event.args.user}`);
            console.log(`     领取数量: ${ethers.formatEther(event.args.amount)} PT`);
        }

        verifier.recordTest(
            "代币领取",
			"Claimed 事件触发",
            passed,
            passed ? "" : "事件未触发"
        );
    } catch (error) {
        verifier.recordTest(
            "代币领取",
			"Claimed 事件触发",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 4: 等待第二次 vesting 时间并领取
    console.log("📝 测试 4: 等待第二次 vesting 时间并领取\n");

    try {
        const now = timeHelper.now();
        const secondVestingTime = projectInfo.schedule.vestingTimes[1];

        if (now < secondVestingTime) {
            console.log(`   等待第二次 vesting: ${timeHelper.formatTimestamp(secondVestingTime)}`);
            console.log(`   需要等待: ${timeHelper.formatDuration(secondVestingTime - now)}\n`);

            await timeHelper.waitUntil(secondVestingTime);
        }

        const user1 = users[0];
        const balanceBefore = await projectToken.balanceOf(user1.wallet.address);

        const tx = await platform.connect(user1.wallet).claim(projectId);
        const receipt = await contractHelper.waitForTx(tx);
        gasTracker.recordTransaction(`${user1.name} 第二次领取`, receipt, "代币领取");

        const balanceAfter = await projectToken.balanceOf(user1.wallet.address);
        const claimed = balanceAfter - balanceBefore;

        console.log(`   第二次领取: ${ethers.formatEther(claimed)} PT`);

        const passed = claimed > 0n;

        verifier.recordTest(
            "代币领取",
            "第二次领取成功",
            passed,
            passed ? `领取: ${ethers.formatEther(claimed)} PT` : "未领取到代币"
        );
    } catch (error) {
        verifier.recordTest(
            "代币领取",
            "第二次领取成功",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 5: 等待第三次 vesting 时间并领取
    console.log("📝 测试 5: 等待第三次 vesting 时间并领取\n");

    try {
        const now = timeHelper.now();
        const thirdVestingTime = projectInfo.schedule.vestingTimes[2];

        if (now < thirdVestingTime) {
            console.log(`   等待第三次 vesting: ${timeHelper.formatTimestamp(thirdVestingTime)}`);
            console.log(`   需要等待: ${timeHelper.formatDuration(thirdVestingTime - now)}\n`);

            await timeHelper.waitUntil(thirdVestingTime);
        }

        const user1 = users[0];
        const balanceBefore = await projectToken.balanceOf(user1.wallet.address);

        const tx = await platform.connect(user1.wallet).claim(projectId);
        const receipt = await contractHelper.waitForTx(tx);
        gasTracker.recordTransaction(`${user1.name} 第三次领取`, receipt, "代币领取");

        const balanceAfter = await projectToken.balanceOf(user1.wallet.address);
        const claimed = balanceAfter - balanceBefore;

        console.log(`   第三次领取: ${ethers.formatEther(claimed)} PT`);

        const passed = claimed > 0n;

        verifier.recordTest(
            "代币领取",
            "第三次领取成功",
            passed,
            passed ? `领取: ${ethers.formatEther(claimed)} PT` : "未领取到代币"
        );
    } catch (error) {
        verifier.recordTest(
            "代币领取",
            "第三次领取成功",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 6: 验证用户已领取全部代币
    console.log("📝 测试 6: 验证用户已领取全部代币\n");

    try {
        const user1 = users[0];
        const purchase = await contractHelper.getUserPurchase(projectId, user1.wallet.address);

		console.log(`   总认购: ${ethers.formatEther(purchase.totalTokensBought)} PT`);
		console.log(`   已领取: ${ethers.formatEther(purchase.claimedTokens)} PT`);

        // 允许 2 wei 的误差（因为有 3 次领取，每次可能有 1 wei 误差）
		const diff = BigInt(purchase.totalTokensBought) > BigInt(purchase.claimedTokens)
			? BigInt(purchase.totalTokensBought) - BigInt(purchase.claimedTokens)
			: BigInt(purchase.claimedTokens) - BigInt(purchase.totalTokensBought);

        const passed = diff <= 2n;

        verifier.recordTest(
            "代币领取",
            "用户已领取全部代币",
            passed,
            passed ? "领取完成" : `差额: ${diff.toString()} wei`
        );
    } catch (error) {
        verifier.recordTest(
            "代币领取",
            "用户已领取全部代币",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 打印 Gas 报告
    gasTracker.printReport();

    // 打印测试报告
    const report = verifier.printReport();

    return {
        passed: report.summary.passed === report.summary.total,
        report,
        gasTracker
    };
}

module.exports = runClaimTests;

// 如果直接运行此文件
if (require.main === module) {
    runClaimTests()
        .then(result => {
            process.exit(result.passed ? 0 : 1);
        })
        .catch(error => {
            console.error("\n❌ 测试执行失败:", error.message);
            console.error(error);
            process.exit(1);
        });
}
