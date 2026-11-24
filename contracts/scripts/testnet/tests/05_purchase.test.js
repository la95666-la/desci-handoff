/**
 * 测试模块 5: 用户认购
 * 测试用户认购功能
 */

const { ethers } = require("hardhat");
const ContractHelper = require("../utils/contractHelper");
const VerificationHelper = require("../utils/verificationHelper");
const TimeHelper = require("../utils/timeHelper");
const GasTracker = require("../utils/gasTracker");

async function runPurchaseTests() {
    console.log("\n" + "═".repeat(80));
    console.log("                    测试模块 5: 用户认购");
    console.log("═".repeat(80) + "\n");

    const accountsConfig = require("../config/accounts.json");
    const testParams = require("../config/testParams.json");
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

    const { platform, jccToken } = contractHelper.contracts;

    console.log(`📊 认购信息:`);
    console.log(`   项目 ID: ${projectId}`);
    console.log(`   单用户上限: ${testParams.projectParams.maxPerUser} JCC\n`);

    // 测试 1: 用户 1 授权 JCC
    console.log("📝 测试 1: 用户 1 授权 JCC\n");

    try {
        const user1 = users[0];
        const approveAmount = ethers.parseEther("1000");
        const platformAddress = await platform.getAddress();

        console.log(`   ${user1.name} 授权 ${ethers.formatEther(approveAmount)} JCC`);

        const tx = await jccToken.connect(user1.wallet).approve(
            platformAddress,
            approveAmount
        );

        const receipt = await contractHelper.waitForTx(tx);
        gasTracker.recordTransaction(`${user1.name} 授权`, receipt, "用户认购");

        // 验证授权额度
        const allowance = await jccToken.allowance(user1.wallet.address, platformAddress);
        const passed = allowance >= approveAmount;

        verifier.recordTest(
            "用户认购",
            "用户授权 JCC 成功",
            passed,
            passed ? `授权额度: ${ethers.formatEther(allowance)} JCC` : "授权额度不足"
        );
    } catch (error) {
        verifier.recordTest(
            "用户认购",
            "用户授权 JCC 成功",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 等待项目开始
    console.log("⏳ 等待项目开始...\n");
    const now = timeHelper.now();
    const projectStart = projectInfo.schedule.projectStart;

    if (now < projectStart) {
        await timeHelper.waitUntil(projectStart);
    } else {
        console.log("   ✅ 项目已开始\n");
    }

    console.log("");

    // 测试 2: 用户 1 认购代币
    console.log("📝 测试 2: 用户 1 认购代币\n");

    try {
        const user1 = users[0];
        const purchaseAmount = ethers.parseEther("500"); // 认购 500 JCC

        const balanceBefore = await jccToken.balanceOf(user1.wallet.address);
        console.log(`   认购前余额: ${ethers.formatEther(balanceBefore)} JCC`);
        console.log(`   认购金额: ${ethers.formatEther(purchaseAmount)} JCC`);

        const tx = await platform.connect(user1.wallet).purchase(
            projectId,
            purchaseAmount
        );

        const receipt = await contractHelper.waitForTx(tx);
        gasTracker.recordTransaction(`${user1.name} 认购`, receipt, "用户认购");

        const balanceAfter = await jccToken.balanceOf(user1.wallet.address);
        console.log(`   认购后余额: ${ethers.formatEther(balanceAfter)} JCC`);

        // 验证余额变化
        const spent = balanceBefore - balanceAfter;
        const passed = spent === purchaseAmount;

        verifier.recordTest(
            "用户认购",
            "用户认购代币成功",
            passed,
            passed ? `支付: ${ethers.formatEther(spent)} JCC` : "支付金额不正确"
        );

        // 验证认购记录
        await verifier.verifyUserPurchase(
            projectId,
            user1.wallet.address,
            500,
            500,
            "用户 1 认购记录正确"
        );

    } catch (error) {
        verifier.recordTest(
            "用户认购",
            "用户认购代币成功",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 3: 多个用户认购
    console.log("📝 测试 3: 多个用户认购\n");

    try {
        let allPassed = true;

        // 用户 2-5 各认购 1000 JCC
        for (let i = 1; i < 5; i++) {
            const user = users[i];
            const purchaseAmount = ethers.parseEther("1000");
            const platformAddress = await platform.getAddress();

            console.log(`   ${user.name} 认购 ${ethers.formatEther(purchaseAmount)} JCC`);

            // 授权
            const approveTx = await jccToken.connect(user.wallet).approve(
                platformAddress,
                purchaseAmount
            );
            await approveTx.wait(2);

            // 认购
            const purchaseTx = await platform.connect(user.wallet).purchase(
                projectId,
                purchaseAmount
            );
            const receipt = await purchaseTx.wait(2);
            gasTracker.recordTransaction(`${user.name} 认购`, receipt, "用户认购");

            console.log(`     ✅ 认购成功`);
        }

        // 验证总募集金额
        const project = await contractHelper.getProject(projectId);
        const expectedRaised = ethers.parseEther("4500"); // 500 + 1000*4
        const passed = project.totalRaised === expectedRaised.toString();

        verifier.recordTest(
            "用户认购",
            "多个用户认购成功",
            passed,
            passed
                ? `总募集: ${ethers.formatEther(project.totalRaised)} JCC`
                : `期望: ${ethers.formatEther(expectedRaised)}, 实际: ${ethers.formatEther(project.totalRaised)}`
        );

    } catch (error) {
        verifier.recordTest(
            "用户认购",
            "多个用户认购成功",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 4: 验证 Purchase 事件
    console.log("📝 测试 4: 验证 Purchase 事件\n");

    try {
        const filter = platform.filters.Purchase(projectId);
        const events = await platform.queryFilter(filter);

        const passed = events.length >= 5; // 至少 5 个用户认购

        if (passed) {
            console.log(`   共触发 ${events.length} 个 Purchase 事件`);
            for (let i = 0; i < Math.min(3, events.length); i++) {
                const event = events[i];
                console.log(`     ${i + 1}. 用户: ${event.args.user.slice(0, 8)}..., 金额: ${ethers.formatEther(event.args.jccAmount)} JCC`);
            }
        }

        verifier.recordTest(
            "用户认购",
            "Purchase 事件触发",
            passed,
            passed ? `触发 ${events.length} 次` : "事件数量不足"
        );
    } catch (error) {
        verifier.recordTest(
            "用户认购",
            "Purchase 事件触发",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 5: 拒绝超过单用户上限的认购
    console.log("📝 测试 5: 拒绝超过单用户上限的认购\n");

    try {
        const user1 = users[0];
        const excessAmount = ethers.parseEther("600"); // 已认购 500，再认购 600 会超过 1000 上限

        console.log(`   ${user1.name} 尝试再认购 ${ethers.formatEther(excessAmount)} JCC`);
        console.log(`   (已认购 500 JCC，总计 1100 JCC，超过 1000 JCC 上限)`);

        const tx = platform.connect(user1.wallet).purchase(
            projectId,
            excessAmount
        );

        await tx;

        verifier.recordTest(
            "用户认购",
            "拒绝超过单用户上限",
            false,
            "应该拒绝但成功了"
        );
    } catch (error) {
        const passed = error.message.includes("revert") || error.message.includes("exceed");
        verifier.recordTest(
            "用户认购",
            "拒绝超过单用户上限",
            passed,
            passed ? "正确拒绝" : `意外错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 6: 验证项目达到软顶
    console.log("📝 测试 6: 验证项目达到软顶\n");

    try {
        const project = await contractHelper.getProject(projectId);
        const softCap = ethers.parseEther(testParams.projectParams.softCap);

        console.log(`   软顶: ${ethers.formatEther(softCap)} JCC`);
        console.log(`   已募集: ${ethers.formatEther(project.totalRaised)} JCC`);

        const passed = BigInt(project.totalRaised) >= softCap;

        verifier.recordTest(
            "用户认购",
            "项目达到软顶",
            passed,
            passed ? "已达到软顶" : "未达到软顶"
        );
    } catch (error) {
        verifier.recordTest(
            "用户认购",
            "项目达到软顶",
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

module.exports = runPurchaseTests;

// 如果直接运行此文件
if (require.main === module) {
    runPurchaseTests()
        .then(result => {
            process.exit(result.passed ? 0 : 1);
        })
        .catch(error => {
            console.error("\n❌ 测试执行失败:", error.message);
            console.error(error);
            process.exit(1);
        });
}
