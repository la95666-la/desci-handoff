/**
 * 测试模块 3: 代币充值
 * 测试项目方充值代币到合约
 */

const { ethers } = require("hardhat");
const ContractHelper = require("../utils/contractHelper");
const VerificationHelper = require("../utils/verificationHelper");
const GasTracker = require("../utils/gasTracker");

async function runDepositTokensTests() {
    console.log("\n" + "═".repeat(80));
    console.log("                    测试模块 3: 代币充值");
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
    const projectOwnerPrivateKey = accountsConfig.accounts.projectOwner.privateKey;
    const projectOwner = new ethers.Wallet(projectOwnerPrivateKey, ethers.provider);

    const contractHelper = new ContractHelper(ethers.provider);
    await contractHelper.initContracts(deployer);

    const verifier = new VerificationHelper(contractHelper);
    const gasTracker = new GasTracker();

    const { platform, projectToken } = contractHelper.contracts;

    // 计算需要充值的代币数量
    const hardCap = ethers.parseEther(testParams.projectParams.hardCap);
    const tokenPrice = ethers.parseEther(testParams.projectParams.tokenPrice);
    // 正确计算：hardCap / tokenPrice，但保持 18 位精度
    const requiredTokens = (hardCap * ethers.parseEther("1")) / tokenPrice;

    console.log(`📊 充值信息:`);
    console.log(`   项目 ID: ${projectId}`);
    console.log(`   硬顶: ${testParams.projectParams.hardCap} JCC`);
    console.log(`   代币价格: ${testParams.projectParams.tokenPrice} JCC`);
    console.log(`   需要充值: ${ethers.formatEther(requiredTokens)} PT\n`);

    // 测试 1: 项目方授权代币
    console.log("📝 测试 1: 项目方授权代币\n");

    try {
        const platformAddress = await platform.getAddress();

        console.log(`   授权 ${ethers.formatEther(requiredTokens)} PT 给平台合约`);

        const tx = await projectToken.connect(projectOwner).approve(
            platformAddress,
            requiredTokens
        );

        const receipt = await contractHelper.waitForTx(tx);
        gasTracker.recordTransaction("授权代币", receipt, "代币充值");

        // 验证授权额度
        const allowance = await projectToken.allowance(projectOwner.address, platformAddress);
        const passed = allowance >= requiredTokens;

        verifier.recordTest(
            "代币充值",
            "项目方授权代币成功",
            passed,
            passed ? `授权额度: ${ethers.formatEther(allowance)} PT` : "授权额度不足"
        );
    } catch (error) {
        verifier.recordTest(
            "代币充值",
            "项目方授权代币成功",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 2: 项目方充值代币
    console.log("📝 测试 2: 项目方充值代币\n");

    try {
        const balanceBefore = await projectToken.balanceOf(projectOwner.address);
        console.log(`   充值前余额: ${ethers.formatEther(balanceBefore)} PT`);

        const tx = await platform.connect(projectOwner).depositProjectTokens(
            projectId,
            requiredTokens
        );

        const receipt = await contractHelper.waitForTx(tx);
        gasTracker.recordTransaction("充值代币", receipt, "代币充值");

        const balanceAfter = await projectToken.balanceOf(projectOwner.address);
        console.log(`   充值后余额: ${ethers.formatEther(balanceAfter)} PT`);

        // 验证余额变化
        const transferred = balanceBefore - balanceAfter;
        const passed = transferred === requiredTokens;

        verifier.recordTest(
            "代币充值",
            "项目方充值代币成功",
            passed,
            passed ? `充值: ${ethers.formatEther(transferred)} PT` : "充值金额不正确"
        );

        // 验证合约余额
        const platformAddress = await platform.getAddress();
        const contractBalance = await projectToken.balanceOf(platformAddress);
        console.log(`   合约余额: ${ethers.formatEther(contractBalance)} PT`);

    } catch (error) {
        verifier.recordTest(
            "代币充值",
            "项目方充值代币成功",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 3: 验证 ProjectTokensDeposited 事件
    console.log("📝 测试 3: 验证 ProjectTokensDeposited 事件\n");

    try {
        const filter = platform.filters.ProjectTokensDeposited(projectId);
        const events = await platform.queryFilter(filter);

        const passed = events.length > 0;

        if (passed) {
            const event = events[0];
            console.log(`   事件参数:`);
            console.log(`     项目 ID: ${event.args.projectId}`);
            console.log(`     充值数量: ${ethers.formatEther(event.args.amount)} PT`);
        }

        verifier.recordTest(
            "代币充值",
            "ProjectTokensDeposited 事件触发",
            passed,
            passed ? "" : "事件未触发"
        );
    } catch (error) {
        verifier.recordTest(
            "代币充值",
            "ProjectTokensDeposited 事件触发",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 4: 非项目方无法充值
    console.log("📝 测试 4: 非项目方无法充值\n");

    try {
        // 使用 deployer 账户尝试充值（不是项目方）
        const tx = platform.connect(deployer).depositProjectTokens(
            projectId,
            ethers.parseEther("100")
        );

        await tx;

        verifier.recordTest(
            "代币充值",
            "非项目方无法充值",
            false,
            "应该拒绝但成功了"
        );
    } catch (error) {
        const passed = error.message.includes("revert") || error.message.includes("owner");
        verifier.recordTest(
            "代币充值",
            "非项目方无法充值",
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

module.exports = runDepositTokensTests;

// 如果直接运行此文件
if (require.main === module) {
    runDepositTokensTests()
        .then(result => {
            process.exit(result.passed ? 0 : 1);
        })
        .catch(error => {
            console.error("\n❌ 测试执行失败:", error.message);
            console.error(error);
            process.exit(1);
        });
}
