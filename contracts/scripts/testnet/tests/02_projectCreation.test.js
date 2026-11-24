/**
 * 测试模块 2: 项目创建
 * 测试项目创建功能和参数验证
 */

const { ethers } = require("hardhat");
const ContractHelper = require("../utils/contractHelper");
const VerificationHelper = require("../utils/verificationHelper");
const TimeHelper = require("../utils/timeHelper");

async function runProjectCreationTests() {
    console.log("\n" + "═".repeat(80));
    console.log("                    测试模块 2: 项目创建");
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

    const { platform, projectToken } = contractHelper.contracts;

    // 生成项目 ID
    const projectId = BigInt(Date.now()) * 1000000n + BigInt(Math.floor(Math.random() * 1000000));

    // 创建时间表
    const schedule = timeHelper.createSchedule({
        projectStartDelay: testParams.timeConfig.projectStartDelay,
        projectDuration: testParams.timeConfig.projectDuration,
        vestingDelays: testParams.timeConfig.vestingDelays,
        vestingPercents: testParams.timeConfig.vestingPercents
    });

    timeHelper.printSchedule(schedule);

    const projectTokenAddress = await projectToken.getAddress();

    // 测试 1: 创建标准项目
    console.log("📝 测试 1: 创建标准项目\n");

    try {
        const projectParams = {
            projectId: projectId,
            projectToken: projectTokenAddress,
            projectOwner: projectOwner.address,
            tokenPrice: ethers.parseEther(testParams.projectParams.tokenPrice),
            hardCap: ethers.parseEther(testParams.projectParams.hardCap),
            softCap: ethers.parseEther(testParams.projectParams.softCap),
            maxPerUser: ethers.parseEther(testParams.projectParams.maxPerUser),
            startTime: schedule.projectStart,
            endTime: schedule.projectEnd,
            vestingTimes: schedule.vestingTimes,
            vestingPercents: schedule.vestingPercents
        };

        console.log("   项目参数:");
        console.log(`     项目 ID: ${projectId}`);
        console.log(`     代币价格: ${testParams.projectParams.tokenPrice} JCC`);
        console.log(`     硬顶: ${testParams.projectParams.hardCap} JCC`);
        console.log(`     软顶: ${testParams.projectParams.softCap} JCC`);
        console.log(`     单用户上限: ${testParams.projectParams.maxPerUser} JCC`);

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
            schedule.vestingTimes[schedule.vestingTimes.length - 1] - schedule.projectEnd  // vestingDuration
        );

        const receipt = await contractHelper.waitForTx(tx);

        verifier.recordTest(
            "项目创建",
            "创建标准项目成功",
            receipt.status === 1,
            receipt.status === 1 ? `交易: ${receipt.hash}` : "交易失败"
        );

        // 验证项目状态
        await verifier.verifyProjectState(projectId, 0, "项目状态为 Pending");

    } catch (error) {
        verifier.recordTest(
            "项目创建",
            "创建标准项目成功",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 2: 验证 ProjectCreated 事件
    console.log("📝 测试 2: 验证 ProjectCreated 事件\n");

    try {
        const filter = platform.filters.ProjectCreated(projectId);
        const events = await platform.queryFilter(filter);

        const passed = events.length > 0;

        if (passed) {
            const event = events[0];
            console.log(`   事件参数:`);
            console.log(`     项目 ID: ${event.args.projectId}`);
            console.log(`     项目方: ${event.args.projectOwner}`);
            console.log(`     项目代币: ${event.args.projectToken}`);
        }

        verifier.recordTest(
            "项目创建",
            "ProjectCreated 事件触发",
            passed,
            passed ? "" : "事件未触发"
        );
    } catch (error) {
        verifier.recordTest(
            "项目创建",
            "ProjectCreated 事件触发",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 3: 拒绝无效的时间参数
    console.log("📝 测试 3: 拒绝无效的时间参数\n");

    try {
        const invalidProjectId = projectId + 1n;
        const now = timeHelper.now();

        // 尝试创建开始时间晚于结束时间的项目
        const tx = platform.connect(deployer).createProject(
            invalidProjectId,
            projectTokenAddress,
            projectOwner.address,
            ethers.parseEther("1"),
            ethers.parseEther("10000"),
            ethers.parseEther("5000"),
            ethers.parseEther("1000"),
            now + 3600,
            now + 1800, // 结束时间早于开始时间
            schedule.vestingTimes,
            schedule.vestingPercents,
            schedule.vestingTimes[schedule.vestingTimes.length - 1] - schedule.projectEnd
        );

        await tx;

        // 如果没有抛出错误，测试失败
        verifier.recordTest(
            "项目创建",
            "拒绝无效时间参数",
            false,
            "应该拒绝但成功了"
        );
    } catch (error) {
        // 预期会抛出错误
        const passed = error.message.includes("revert") || error.message.includes("invalid");
        verifier.recordTest(
            "项目创建",
            "拒绝无效时间参数",
            passed,
            passed ? "正确拒绝" : `意外错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 4: 拒绝无效的 vesting 参数
    console.log("📝 测试 4: 拒绝无效的 vesting 参数\n");

    try {
        const invalidProjectId = projectId + 2n;

        // 尝试创建 vesting 百分比总和不等于 10000 的项目
        const tx = platform.connect(deployer).createProject(
            invalidProjectId,
            projectTokenAddress,
            projectOwner.address,
            ethers.parseEther("1"),
            ethers.parseEther("10000"),
            ethers.parseEther("5000"),
            ethers.parseEther("1000"),
            schedule.projectStart,
            schedule.projectEnd,
            schedule.vestingTimes,
            [3000, 3000, 3000], // 总和为 9000，不等于 10000
            schedule.vestingTimes[schedule.vestingTimes.length - 1] - schedule.projectEnd
        );

        await tx;

        verifier.recordTest(
            "项目创建",
            "拒绝无效 vesting 参数",
            false,
            "应该拒绝但成功了"
        );
    } catch (error) {
        const passed = error.message.includes("revert") || error.message.includes("invalid");
        verifier.recordTest(
            "项目创建",
            "拒绝无效 vesting 参数",
            passed,
            passed ? "正确拒绝" : `意外错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 5: 拒绝重复的项目 ID
    console.log("📝 测试 5: 拒绝重复的项目 ID\n");

    try {
        // 尝试使用相同的项目 ID 创建项目
        const tx = platform.connect(deployer).createProject(
            projectId, // 使用已存在的项目 ID
            projectTokenAddress,
            projectOwner.address,
            ethers.parseEther("1"),
            ethers.parseEther("10000"),
            ethers.parseEther("5000"),
            ethers.parseEther("1000"),
            schedule.projectStart,
            schedule.projectEnd,
            schedule.vestingTimes,
            schedule.vestingPercents,
            schedule.vestingTimes[schedule.vestingTimes.length - 1] - schedule.projectEnd
        );

        await tx;

        verifier.recordTest(
            "项目创建",
            "拒绝重复项目 ID",
            false,
            "应该拒绝但成功了"
        );
    } catch (error) {
        const passed = error.message.includes("revert") || error.message.includes("exists");
        verifier.recordTest(
            "项目创建",
            "拒绝重复项目 ID",
            passed,
            passed ? "正确拒绝" : `意外错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 6: 非 owner 无法创建项目
    console.log("📝 测试 6: 非 owner 无法创建项目\n");

    try {
        const invalidProjectId = projectId + 3n;

        // 使用 projectOwner 账户尝试创建项目（只有 deployer 是 owner）
        const tx = platform.connect(projectOwner).createProject(
            invalidProjectId,
            projectTokenAddress,
            projectOwner.address,
            ethers.parseEther("1"),
            ethers.parseEther("10000"),
            ethers.parseEther("5000"),
            ethers.parseEther("1000"),
            schedule.projectStart,
            schedule.projectEnd,
            schedule.vestingTimes,
            schedule.vestingPercents,
            schedule.vestingTimes[schedule.vestingTimes.length - 1] - schedule.projectEnd
        );

        await tx;

        verifier.recordTest(
            "项目创建",
            "非 owner 无法创建项目",
            false,
            "应该拒绝但成功了"
        );
    } catch (error) {
        // 检查是否是权限错误（包括自定义错误 0x118cdaa7 或 Ownable 相关错误）
        const passed = error.message.includes("Ownable") ||
                      error.message.includes("owner") ||
                      error.message.includes("0x118cdaa7") ||
                      error.message.includes("revert");
        verifier.recordTest(
            "项目创建",
            "非 owner 无法创建项目",
            passed,
            passed ? "正确拒绝" : `意外错误: ${error.message}`
        );
    }

    console.log("");

    // 保存项目 ID 供后续测试使用
    const fs = require("fs");
    const path = require("path");
    const projectInfoPath = path.join(__dirname, "../reports/projectInfo.json");

    const projectInfo = {
        projectId: projectId.toString(),
        schedule,
        createdAt: new Date().toISOString()
    };

    const dir = path.dirname(projectInfoPath);
    if (!fs.existsSync(dir)) {
        fs.mkdirSync(dir, { recursive: true });
    }

    fs.writeFileSync(projectInfoPath, JSON.stringify(projectInfo, null, 2));
    console.log(`💾 项目信息已保存: ${projectInfoPath}\n`);

    // 打印测试报告
    const report = verifier.printReport();

    return {
        passed: report.summary.passed === report.summary.total,
        report,
        projectId: projectId.toString(),
        schedule
    };
}

module.exports = runProjectCreationTests;

// 如果直接运行此文件
if (require.main === module) {
    runProjectCreationTests()
        .then(result => {
            process.exit(result.passed ? 0 : 1);
        })
        .catch(error => {
            console.error("\n❌ 测试执行失败:", error.message);
            console.error(error);
            process.exit(1);
        });
}
