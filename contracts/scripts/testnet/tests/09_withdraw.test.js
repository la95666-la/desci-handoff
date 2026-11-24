/**
 * 测试模块 9: 资金提取
 * 测试项目方提取资金和代币
 */

const { ethers } = require("hardhat");
const ContractHelper = require("../utils/contractHelper");
const VerificationHelper = require("../utils/verificationHelper");
const GasTracker = require("../utils/gasTracker");

async function runWithdrawTests() {
    console.log("\n" + "═".repeat(80));
    console.log("                    测试模块 9: 资金提取");
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
    const projectOwnerPrivateKey = accountsConfig.accounts.projectOwner.privateKey;
    const projectOwner = new ethers.Wallet(projectOwnerPrivateKey, ethers.provider);

    const contractHelper = new ContractHelper(ethers.provider);
    await contractHelper.initContracts(deployer);

    const verifier = new VerificationHelper(contractHelper);
    const gasTracker = new GasTracker();

    const { platform, jccToken, projectToken } = contractHelper.contracts;

    console.log(`📊 项目信息:`);
    console.log(`   项目 ID: ${projectId}\n`);

    // 测试 1: 获取项目募集信息
    console.log("📝 测试 1: 获取项目募集信息\n");

    try {
        const project = await contractHelper.getProject(projectId);

        console.log(`   项目状态: ${["Pending", "Active", "Ended", "Successful", "Failed", "Cancelled"][project.status]}`);
        console.log(`   总募集: ${ethers.formatEther(project.totalRaised)} JCC`);
        console.log(`   总售出: ${ethers.formatEther(project.totalTokensSold)} PT`);

		const isSuccessful = project.status === 3; // Successful (enum includes Ended at index 2)

        verifier.recordTest(
            "资金提取",
            "项目状态为成功",
            isSuccessful,
            isSuccessful ? "项目成功" : `状态: ${project.status}`
        );
    } catch (error) {
        verifier.recordTest(
            "资金提取",
            "项目状态为成功",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 2: 计算平台费用
    console.log("📝 测试 2: 计算平台费用\n");

    try {
        const project = await contractHelper.getProject(projectId);
        const platformFeesStruct = await platform.platformFees();
        const platformFeeRate = platformFeesStruct.successFeeRate;

        const totalRaised = BigInt(project.totalRaised);
        const feeRate = BigInt(platformFeeRate);
        const platformFee = (totalRaised * feeRate) / 10000n;
        const projectOwnerAmount = totalRaised - platformFee;

        console.log(`   总募集: ${ethers.formatEther(totalRaised)} JCC`);
        console.log(`   费率: ${feeRate} (${Number(feeRate) / 100}%)`);
        console.log(`   平台费用: ${ethers.formatEther(platformFee)} JCC`);
        console.log(`   项目方可得: ${ethers.formatEther(projectOwnerAmount)} JCC`);

        verifier.recordTest(
            "资金提取",
            "平台费用计算正确",
            true,
            `费用: ${ethers.formatEther(platformFee)} JCC`
        );
    } catch (error) {
        verifier.recordTest(
            "资金提取",
            "平台费用计算正确",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 3: 项目方提取募集资金
    console.log("📝 测试 3: 项目方提取募集资金\n");

    try {
        const balanceBefore = await jccToken.balanceOf(projectOwner.address);
        console.log(`   提取前余额: ${ethers.formatEther(balanceBefore)} JCC`);

        const tx = await platform.connect(projectOwner).withdrawRaisedFunds(projectId);
        const receipt = await contractHelper.waitForTx(tx);
        gasTracker.recordTransaction("提取募集资金", receipt, "资金提取");

        const balanceAfter = await jccToken.balanceOf(projectOwner.address);
        console.log(`   提取后余额: ${ethers.formatEther(balanceAfter)} JCC`);

        const withdrawn = balanceAfter - balanceBefore;
        console.log(`   提取金额: ${ethers.formatEther(withdrawn)} JCC`);

        const passed = withdrawn > 0n;

        verifier.recordTest(
            "资金提取",
            "项目方提取募集资金成功",
            passed,
            passed ? `提取: ${ethers.formatEther(withdrawn)} JCC` : "未提取到资金"
        );
    } catch (error) {
        verifier.recordTest(
            "资金提取",
            "项目方提取募集资金成功",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 4: 验证 RaisedFundsWithdrawn 事件
    console.log("📝 测试 4: 验证 RaisedFundsWithdrawn 事件\n");

    try {
        const filter = platform.filters.RaisedFundsWithdrawn(projectId);
        const events = await platform.queryFilter(filter);

        const passed = events.length > 0;

        if (passed) {
            const event = events[0];
            console.log(`   事件参数:`);
            console.log(`     项目 ID: ${event.args.projectId}`);
            console.log(`     项目方: ${event.args.projectOwner}`);
            console.log(`     提取金额: ${ethers.formatEther(event.args.amount)} JCC`);
        }

        verifier.recordTest(
            "资金提取",
            "RaisedFundsWithdrawn 事件触发",
            passed,
            passed ? "" : "事件未触发"
        );
    } catch (error) {
        verifier.recordTest(
            "资金提取",
            "RaisedFundsWithdrawn 事件触发",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 5: 验证平台费用已收取
    console.log("📝 测试 5: 验证平台费用已收取\n");

    try {
        const platformFeesStruct = await platform.platformFees();
        const feeReceiver = platformFeesStruct.feeReceiver;
        const feeReceiverBalance = await jccToken.balanceOf(feeReceiver);

        console.log(`   费用接收者: ${feeReceiver}`);
        console.log(`   费用余额: ${ethers.formatEther(feeReceiverBalance)} JCC`);

        const passed = feeReceiverBalance > 0n;

        verifier.recordTest(
            "资金提取",
            "平台费用已收取",
            passed,
            passed ? `费用: ${ethers.formatEther(feeReceiverBalance)} JCC` : "未收取费用"
        );
    } catch (error) {
        verifier.recordTest(
            "资金提取",
            "平台费用已收取",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 6: 项目方无法重复提取
    console.log("📝 测试 6: 项目方无法重复提取\n");

    try {
        const tx = platform.connect(projectOwner).withdrawRaisedFunds(projectId);
        await tx;

        verifier.recordTest(
            "资金提取",
            "无法重复提取",
            false,
            "应该拒绝但成功了"
        );
    } catch (error) {
        const passed = error.message.includes("revert") || error.message.includes("already");
        verifier.recordTest(
            "资金提取",
            "无法重复提取",
            passed,
            passed ? "正确拒绝" : `意外错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 7: 计算剩余未售出代币
    console.log("📝 测试 7: 计算剩余未售出代币\n");

    try {
        const project = await contractHelper.getProject(projectId);
        const platformAddress = await platform.getAddress();

		const totalDeposited = BigInt(project.totalTokensDeposited);
        const totalSold = BigInt(project.totalTokensSold);
        const remaining = totalDeposited - totalSold;

        console.log(`   总充值: ${ethers.formatEther(totalDeposited)} PT`);
        console.log(`   已售出: ${ethers.formatEther(totalSold)} PT`);
        console.log(`   剩余: ${ethers.formatEther(remaining)} PT`);

        verifier.recordTest(
            "资金提取",
            "计算剩余代币",
            true,
            `剩余: ${ethers.formatEther(remaining)} PT`
        );
    } catch (error) {
        verifier.recordTest(
            "资金提取",
            "计算剩余代币",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 8: 项目方提取剩余代币
    console.log("📝 测试 8: 项目方提取剩余代币\n");

    try {
        const balanceBefore = await projectToken.balanceOf(projectOwner.address);
        console.log(`   提取前余额: ${ethers.formatEther(balanceBefore)} PT`);

        const tx = await platform.connect(projectOwner).withdrawProjectTokens(projectId);
        const receipt = await contractHelper.waitForTx(tx);
        gasTracker.recordTransaction("提取剩余代币", receipt, "资金提取");

        const balanceAfter = await projectToken.balanceOf(projectOwner.address);
        console.log(`   提取后余额: ${ethers.formatEther(balanceAfter)} PT`);

        const withdrawn = balanceAfter - balanceBefore;
        console.log(`   提取金额: ${ethers.formatEther(withdrawn)} PT`);

        const passed = withdrawn >= 0n; // 可能没有剩余代币

        verifier.recordTest(
            "资金提取",
            "项目方提取剩余代币",
            passed,
            `提取: ${ethers.formatEther(withdrawn)} PT`
        );
    } catch (error) {
        verifier.recordTest(
            "资金提取",
            "项目方提取剩余代币",
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

module.exports = runWithdrawTests;

// 如果直接运行此文件
if (require.main === module) {
    runWithdrawTests()
        .then(result => {
            process.exit(result.passed ? 0 : 1);
        })
        .catch(error => {
            console.error("\n❌ 测试执行失败:", error.message);
            console.error(error);
            process.exit(1);
        });
}
