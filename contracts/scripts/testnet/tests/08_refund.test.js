/**
 * 测试模块 8: 退款测试
 * 测试失败项目的退款功能
 */

const { ethers } = require("hardhat");
const ContractHelper = require("../utils/contractHelper");
const VerificationHelper = require("../utils/verificationHelper");
const TimeHelper = require("../utils/timeHelper");
const GasTracker = require("../utils/gasTracker");

async function runRefundTests() {
    console.log("\n" + "═".repeat(80));
    console.log("                    测试模块 8: 退款测试");
    console.log("═".repeat(80) + "\n");

    const accountsConfig = require("../config/accounts.json");
    const testParams = require("../config/testParams.json");

    const [deployer] = await ethers.getSigners();
    const projectOwnerPrivateKey = accountsConfig.accounts.projectOwner.privateKey;
    const projectOwner = new ethers.Wallet(projectOwnerPrivateKey, ethers.provider);

    const contractHelper = new ContractHelper(ethers.provider);
    await contractHelper.initContracts(deployer);

    const verifier = new VerificationHelper(contractHelper);
    const timeHelper = new TimeHelper();
    const gasTracker = new GasTracker();

    const { platform, jccToken, projectToken } = contractHelper.contracts;

    console.log("📝 创建一个失败的测试项目用于退款测试\n");

    // 创建一个新项目用于退款测试
    const failedProjectId = BigInt(Date.now()) * 1000000n + BigInt(Math.floor(Math.random() * 1000000));
    const now = timeHelper.now();

    const projectParams = {
        projectId: failedProjectId,
        projectToken: await projectToken.getAddress(),
        projectOwner: projectOwner.address,
        tokenPrice: ethers.parseEther("1"),
        hardCap: ethers.parseEther("10000"),
        softCap: ethers.parseEther("5000"), // 软顶 5000
        maxPerUser: ethers.parseEther("1000"),
        startTime: now + 60, // 1分钟后开始
        endTime: now + 300, // 5分钟后结束
        vestingTimes: [now + 400, now + 500, now + 600],
        vestingPercents: [3333, 3333, 3334]
    };

    // 测试 1: 创建测试项目
    console.log("📝 测试 1: 创建失败测试项目\n");

    try {
        const tx = await platform.connect(deployer).createProject(
            projectParams.projectId,
            projectParams.projectToken,
            projectParams.projectOwner,
            projectParams.tokenPrice,
            projectParams.hardCap,
            projectParams.softCap,
            projectParams.maxPerUser,
            projectParams.startTime,
            projectParams.endTime,
            projectParams.vestingTimes,
            projectParams.vestingPercents,
            projectParams.vestingTimes[projectParams.vestingTimes.length - 1] - projectParams.vestingTimes[0]
        );

        await contractHelper.waitForTx(tx);

        console.log(`   项目 ID: ${failedProjectId}`);
        console.log(`   软顶: ${ethers.formatEther(projectParams.softCap)} JCC`);

        verifier.recordTest(
            "退款测试",
            "创建失败测试项目",
            true,
            "项目创建成功"
        );
    } catch (error) {
        verifier.recordTest(
            "退款测试",
            "创建失败测试项目",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 2: 项目方充值代币
    console.log("📝 测试 2: 项目方充值代币\n");

    try {
        const requiredTokens = (projectParams.hardCap * ethers.parseEther("1")) / projectParams.tokenPrice;
        const platformAddress = await platform.getAddress();

        // 授权
        await projectToken.connect(projectOwner).approve(platformAddress, requiredTokens);

        // 充值
        const tx = await platform.connect(projectOwner).depositProjectTokens(
            failedProjectId,
            requiredTokens
        );

        await contractHelper.waitForTx(tx);

        verifier.recordTest(
            "退款测试",
            "项目方充值代币",
            true,
            `充值: ${ethers.formatEther(requiredTokens)} PT`
        );
    } catch (error) {
        verifier.recordTest(
            "退款测试",
            "项目方充值代币",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 等待项目开始
    console.log("⏳ 等待项目开始...\n");
    await timeHelper.waitUntil(projectParams.startTime);

    // 测试 3: 用户认购（但不达到软顶）
    console.log("📝 测试 3: 用户认购（不达到软顶）\n");

    try {
        const user1PrivateKey = accountsConfig.accounts.user1.privateKey;
        const user1 = new ethers.Wallet(user1PrivateKey, ethers.provider);

        const purchaseAmount = ethers.parseEther("1000"); // 只认购 1000，不达到 5000 软顶
        const platformAddress = await platform.getAddress();

        // 授权
        await jccToken.connect(user1).approve(platformAddress, purchaseAmount);

        // 认购
        const tx = await platform.connect(user1).purchase(
            failedProjectId,
            purchaseAmount
        );

        await contractHelper.waitForTx(tx);

        console.log(`   认购金额: ${ethers.formatEther(purchaseAmount)} JCC`);
        console.log(`   软顶: ${ethers.formatEther(projectParams.softCap)} JCC`);
        console.log(`   未达到软顶，项目将失败`);

        verifier.recordTest(
            "退款测试",
            "用户认购（不达到软顶）",
            true,
            "认购成功"
        );
    } catch (error) {
        verifier.recordTest(
            "退款测试",
            "用户认购（不达到软顶）",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 等待项目结束
    console.log("⏳ 等待项目结束...\n");
    await timeHelper.waitUntil(projectParams.endTime);

    // 测试 4: 结束项目（失败）
    console.log("📝 测试 4: 结束项目（失败）\n");

    try {
        const tx = await platform.connect(projectOwner).endProject(failedProjectId);
        await contractHelper.waitForTx(tx);

        const project = await contractHelper.getProject(failedProjectId);
		const isFailed = project.status === 4; // Failed

		console.log(`   项目状态: ${["Pending", "Active", "Ended", "Successful", "Failed", "Cancelled"][project.status]}`);
        console.log(`   已募集: ${ethers.formatEther(project.totalRaised)} JCC`);
        console.log(`   软顶: ${ethers.formatEther(project.softCap)} JCC`);

        verifier.recordTest(
            "退款测试",
            "项目标记为失败",
            isFailed,
            isFailed ? "项目失败" : "状态不正确"
        );
    } catch (error) {
        verifier.recordTest(
            "退款测试",
            "项目标记为失败",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 5: 用户申请退款
    console.log("📝 测试 5: 用户申请退款\n");

    try {
        const user1PrivateKey = accountsConfig.accounts.user1.privateKey;
        const user1 = new ethers.Wallet(user1PrivateKey, ethers.provider);

        const balanceBefore = await jccToken.balanceOf(user1.address);
        console.log(`   退款前余额: ${ethers.formatEther(balanceBefore)} JCC`);

        const tx = await platform.connect(user1).refund(failedProjectId);
        const receipt = await contractHelper.waitForTx(tx);
        gasTracker.recordTransaction("用户退款", receipt, "退款测试");

        const balanceAfter = await jccToken.balanceOf(user1.address);
        console.log(`   退款后余额: ${ethers.formatEther(balanceAfter)} JCC`);

        const refunded = balanceAfter - balanceBefore;
        const expectedRefund = ethers.parseEther("1000");

        console.log(`   退款金额: ${ethers.formatEther(refunded)} JCC`);

        const passed = refunded === expectedRefund;

        verifier.recordTest(
            "退款测试",
            "用户申请退款成功",
            passed,
            passed ? `退款: ${ethers.formatEther(refunded)} JCC` : `期望: ${ethers.formatEther(expectedRefund)}, 实际: ${ethers.formatEther(refunded)}`
        );

        // 验证退款记录
        const purchase = await contractHelper.getUserPurchase(failedProjectId, user1.address);
        const isRefunded = purchase.refunded;

        verifier.recordTest(
            "退款测试",
            "退款记录已标记",
            isRefunded,
            isRefunded ? "已标记为已退款" : "未标记"
        );

    } catch (error) {
        verifier.recordTest(
            "退款测试",
            "用户申请退款成功",
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

module.exports = runRefundTests;

// 如果直接运行此文件
if (require.main === module) {
    runRefundTests()
        .then(result => {
            process.exit(result.passed ? 0 : 1);
        })
        .catch(error => {
            console.error("\n❌ 测试执行失败:", error.message);
            console.error(error);
            process.exit(1);
        });
}
