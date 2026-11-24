/**
 * 测试模块 4: 项目激活
 * 测试项目激活功能
 */

const { ethers } = require("hardhat");
const ContractHelper = require("../utils/contractHelper");
const VerificationHelper = require("../utils/verificationHelper");
const TimeHelper = require("../utils/timeHelper");

async function runActivationTests() {
    console.log("\n" + "═".repeat(80));
    console.log("                    测试模块 4: 项目激活");
    console.log("═".repeat(80) + "\n");

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

    const contractHelper = new ContractHelper(ethers.provider);
    await contractHelper.initContracts(deployer);

    const verifier = new VerificationHelper(contractHelper);
    const timeHelper = new TimeHelper();

    const { platform } = contractHelper.contracts;

    console.log(`📊 项目信息:`);
    console.log(`   项目 ID: ${projectId}`);
    console.log(`   开始时间: ${timeHelper.formatTimestamp(projectInfo.schedule.projectStart)}\n`);

    // 测试 1: 等待项目开始时间
    console.log("📝 测试 1: 等待项目开始时间\n");

    try {
        const now = timeHelper.now();
        const startTime = projectInfo.schedule.projectStart;

        if (now < startTime) {
            console.log(`   当前时间: ${timeHelper.formatTimestamp(now)}`);
            console.log(`   开始时间: ${timeHelper.formatTimestamp(startTime)}`);
            console.log(`   需要等待: ${timeHelper.formatDuration(startTime - now)}\n`);

            await timeHelper.waitUntil(startTime);
        } else {
            console.log(`   ✅ 已到达开始时间\n`);
        }

        verifier.recordTest(
            "项目激活",
            "等待项目开始时间",
            true,
            "等待完成"
        );
    } catch (error) {
        verifier.recordTest(
            "项目激活",
            "等待项目开始时间",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 2: 项目自动激活
    console.log("📝 测试 2: 手动激活项目\n");

    try {
        // 项目需要手动激活
        const tx = await platform.connect(deployer).activateProject(projectId);
        const receipt = await contractHelper.waitForTx(tx);

        // 检查项目状态
        const project = await contractHelper.getProject(projectId);
		console.log(`   激活后状态: ${["Pending", "Active", "Ended", "Successful", "Failed", "Cancelled"][project.status]}`);

        // 项目应该变为 Active (1)
        const passed = project.status === 1; // Active

        verifier.recordTest(
            "项目激活",
            "项目手动激活",
            passed,
            passed ? "项目已激活" : `状态: ${project.status}`
        );
    } catch (error) {
        verifier.recordTest(
            "项目激活",
            "项目手动激活",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 3: 验证项目可以接受认购
    console.log("📝 测试 3: 验证项目可以接受认购\n");

    try {
        const project = await contractHelper.getProject(projectId);

        // 验证项目参数
        console.log(`   硬顶: ${ethers.formatEther(project.hardCap)} JCC`);
        console.log(`   软顶: ${ethers.formatEther(project.softCap)} JCC`);
        console.log(`   已募集: ${ethers.formatEther(project.totalRaised)} JCC`);
        console.log(`   单用户上限: ${ethers.formatEther(project.maxPerUser)} JCC`);

        const passed =
            project.status === 1 && // Active
            BigInt(project.totalRaised) === 0n && // 还没有募集
            BigInt(project.hardCap) > 0n; // 硬顶大于 0

        verifier.recordTest(
            "项目激活",
            "项目可以接受认购",
            passed,
            passed ? "项目状态正常" : "项目状态异常"
        );
    } catch (error) {
        verifier.recordTest(
            "项目激活",
            "项目可以接受认购",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 打印测试报告
    const report = verifier.printReport();

    return {
        passed: report.summary.passed === report.summary.total,
        report
    };
}

module.exports = runActivationTests;

// 如果直接运行此文件
if (require.main === module) {
    runActivationTests()
        .then(result => {
            process.exit(result.passed ? 0 : 1);
        })
        .catch(error => {
            console.error("\n❌ 测试执行失败:", error.message);
            console.error(error);
            process.exit(1);
        });
}
