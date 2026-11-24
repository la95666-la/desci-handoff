/**
 * 测试模块 6: 项目结束
 * 测试项目结束功能
 */

const { ethers } = require("hardhat");
const ContractHelper = require("../utils/contractHelper");
const VerificationHelper = require("../utils/verificationHelper");
const TimeHelper = require("../utils/timeHelper");
const GasTracker = require("../utils/gasTracker");

async function runProjectEndingTests() {
    console.log("\n" + "═".repeat(80));
    console.log("                    测试模块 6: 项目结束");
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
    const timeHelper = new TimeHelper();
    const gasTracker = new GasTracker();

    const { platform } = contractHelper.contracts;

    console.log(`📊 项目信息:`);
    console.log(`   项目 ID: ${projectId}`);
    console.log(`   结束时间: ${timeHelper.formatTimestamp(projectInfo.schedule.projectEnd)}\n`);

    // 测试 1: 等待项目结束时间
    console.log("📝 测试 1: 等待项目结束时间\n");

    try {
        const now = timeHelper.now();
        const endTime = projectInfo.schedule.projectEnd;

        if (now < endTime) {
            console.log(`   当前时间: ${timeHelper.formatTimestamp(now)}`);
            console.log(`   结束时间: ${timeHelper.formatTimestamp(endTime)}`);
            console.log(`   需要等待: ${timeHelper.formatDuration(endTime - now)}\n`);

            await timeHelper.waitUntil(endTime);
        } else {
            console.log(`   ✅ 已到达结束时间\n`);
        }

        verifier.recordTest(
            "项目结束",
            "等待项目结束时间",
            true,
            "等待完成"
        );
    } catch (error) {
        verifier.recordTest(
            "项目结束",
            "等待项目结束时间",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 2: 项目方结束项目
    console.log("📝 测试 2: 项目方结束项目\n");

    try {
        // 检查当前状态
        let project = await contractHelper.getProject(projectId);
        console.log(`   结束前状态: ${["Pending", "Active", "Ended", "Successful", "Failed", "Cancelled"][project.status]}`);
        console.log(`   已募集: ${ethers.formatEther(project.totalRaised)} JCC`);

        const tx = await platform.connect(projectOwner).endProject(projectId);
        const receipt = await contractHelper.waitForTx(tx);
        gasTracker.recordTransaction("结束项目", receipt, "项目结束");

        // 检查结束后状态
        project = await contractHelper.getProject(projectId);
        console.log(`   结束后状态: ${["Pending", "Active", "Ended", "Successful", "Failed", "Cancelled"][project.status]}`);

		// 项目应该变为 Successful (3) 或 Failed (4)
		const passed = project.status === 3 || project.status === 4;

        verifier.recordTest(
            "项目结束",
            "项目方结束项目成功",
            passed,
			passed ? `最终状态: ${["Pending", "Active", "Ended", "Successful", "Failed", "Cancelled"][project.status]}` : "状态异常"
        );
    } catch (error) {
        verifier.recordTest(
            "项目结束",
            "项目方结束项目成功",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 3: 验证项目达到软顶（成功）
    console.log("📝 测试 3: 验证项目达到软顶（成功）\n");

    try {
        const project = await contractHelper.getProject(projectId);
        const softCap = BigInt(project.softCap);
        const totalRaised = BigInt(project.totalRaised);

        console.log(`   软顶: ${ethers.formatEther(softCap)} JCC`);
        console.log(`   已募集: ${ethers.formatEther(totalRaised)} JCC`);

        const reachedSoftCap = totalRaised >= softCap;
		const isSuccessful = project.status === 3;

        const passed = reachedSoftCap && isSuccessful;

        verifier.recordTest(
            "项目结束",
            "项目达到软顶（成功）",
            passed,
            passed ? "项目成功" : `达到软顶: ${reachedSoftCap}, 状态: ${project.status}`
        );
    } catch (error) {
        verifier.recordTest(
            "项目结束",
            "项目达到软顶（成功）",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 4: 验证 ProjectEnded 事件
    console.log("📝 测试 4: 验证 ProjectEnded 事件\n");

    try {
        const filter = platform.filters.ProjectEnded(projectId);
        const events = await platform.queryFilter(filter);

        const passed = events.length > 0;

        if (passed) {
            const event = events[0];
            console.log(`   事件参数:`);
            console.log(`     项目 ID: ${event.args.projectId}`);
			console.log(`     最终状态: ${["Pending", "Active", "Ended", "Successful", "Failed", "Cancelled"][event.args.status]}`);
            console.log(`     总募集: ${ethers.formatEther(event.args.totalRaised)} JCC`);
        }

        verifier.recordTest(
            "项目结束",
            "ProjectEnded 事件触发",
            passed,
            passed ? "" : "事件未触发"
        );
    } catch (error) {
        verifier.recordTest(
            "项目结束",
            "ProjectEnded 事件触发",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 5: 结束后无法再认购
    console.log("📝 测试 5: 结束后无法再认购\n");

    try {
        const user1PrivateKey = accountsConfig.accounts.user1.privateKey;
        const user1 = new ethers.Wallet(user1PrivateKey, ethers.provider);

        const tx = platform.connect(user1).purchase(
            projectId,
            ethers.parseEther("100")
        );

        await tx;

        verifier.recordTest(
            "项目结束",
            "结束后无法再认购",
            false,
            "应该拒绝但成功了"
        );
    } catch (error) {
        const passed = error.message.includes("revert") || error.message.includes("ended");
        verifier.recordTest(
            "项目结束",
            "结束后无法再认购",
            passed,
            passed ? "正确拒绝" : `意外错误: ${error.message}`
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

module.exports = runProjectEndingTests;

// 如果直接运行此文件
if (require.main === module) {
    runProjectEndingTests()
        .then(result => {
            process.exit(result.passed ? 0 : 1);
        })
        .catch(error => {
            console.error("\n❌ 测试执行失败:", error.message);
            console.error(error);
            process.exit(1);
        });
}
