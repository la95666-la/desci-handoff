/**
 * 测试模块 1: 部署验证
 * 验证已部署的合约配置是否正确
 */

const { ethers } = require("hardhat");
const ContractHelper = require("../utils/contractHelper");
const VerificationHelper = require("../utils/verificationHelper");

async function runDeploymentTests() {
    console.log("\n" + "═".repeat(80));
    console.log("                    测试模块 1: 部署验证");
    console.log("═".repeat(80) + "\n");

    const contractsConfig = require("../config/contracts.json");
    const [deployer] = await ethers.getSigners();

    const contractHelper = new ContractHelper(ethers.provider);
    await contractHelper.initContracts(deployer);

    const verifier = new VerificationHelper(contractHelper);

    const { jccToken, platform, projectToken } = contractHelper.contracts;

    // 测试 1: 验证 JCC 代币配置
    console.log("📝 测试 1: 验证 JCC 代币配置\n");

    try {
        const jccName = await jccToken.name();
        const jccSymbol = await jccToken.symbol();
        const jccDecimals = await jccToken.decimals();
        const jccTotalSupply = await jccToken.totalSupply();

        console.log(`   名称: ${jccName}`);
        console.log(`   符号: ${jccSymbol}`);
        console.log(`   精度: ${jccDecimals}`);
        console.log(`   总供应量: ${ethers.formatEther(jccTotalSupply)}`);

        const passed =
            jccName === "JoinCare Coin" &&
            jccSymbol === "JCC" &&
            Number(jccDecimals) === 18;

        verifier.recordTest(
            "部署验证",
            "JCC 代币配置正确",
            passed,
            passed ? "" : "配置不匹配"
        );
    } catch (error) {
        verifier.recordTest(
            "部署验证",
            "JCC 代币配置正确",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 2: 验证 JoinCare 平台配置
    console.log("📝 测试 2: 验证 JoinCare 平台配置\n");

    try {
        const platformJCC = await platform.jccToken();
        const platformFeesStruct = await platform.platformFees();
        const platformFeeRate = platformFeesStruct.successFeeRate;
        const platformFeeReceiver = platformFeesStruct.feeReceiver;
        const platformOwner = await platform.owner();

        console.log(`   JCC 地址: ${platformJCC}`);
        console.log(`   费率: ${platformFeeRate} (${Number(platformFeeRate) / 100}%)`);
        console.log(`   费用接收者: ${platformFeeReceiver}`);
        console.log(`   Owner: ${platformOwner}`);

        const jccAddress = await jccToken.getAddress();
        const passed =
            platformJCC.toLowerCase() === jccAddress.toLowerCase() &&
            Number(platformFeeRate) === 200 &&
            platformOwner.toLowerCase() === deployer.address.toLowerCase();

        verifier.recordTest(
            "部署验证",
            "JoinCare 平台配置正确",
            passed,
            passed ? "" : "配置不匹配"
        );
    } catch (error) {
        verifier.recordTest(
            "部署验证",
            "JoinCare 平台配置正确",
            false,
            `错误: ${error.message}`
        );
    }

    console.log("");

    // 测试 3: 验证项目代币部署
    console.log("📝 测试 3: 验证项目代币部署\n");

    try {
        if (!projectToken) {
            throw new Error("项目代币未部署");
        }

        const ptName = await projectToken.name();
        const ptSymbol = await projectToken.symbol();
        const ptDecimals = await projectToken.decimals();
        const ptTotalSupply = await projectToken.totalSupply();

        console.log(`   名称: ${ptName}`);
        console.log(`   符号: ${ptSymbol}`);
        console.log(`   精度: ${ptDecimals}`);
        console.log(`   总供应量: ${ethers.formatEther(ptTotalSupply)}`);

        const passed = Number(ptDecimals) === 18;

        verifier.recordTest(
            "部署验证",
            "项目代币部署正确",
            passed,
            passed ? "" : "配置不匹配"
        );
    } catch (error) {
        verifier.recordTest(
            "部署验证",
            "项目代币部署正确",
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

module.exports = runDeploymentTests;

// 如果直接运行此文件
if (require.main === module) {
    runDeploymentTests()
        .then(result => {
            process.exit(result.passed ? 0 : 1);
        })
        .catch(error => {
            console.error("\n❌ 测试执行失败:", error.message);
            console.error(error);
            process.exit(1);
        });
}
