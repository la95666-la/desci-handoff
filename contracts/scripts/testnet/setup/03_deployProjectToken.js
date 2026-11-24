/**
 * 部署项目代币
 * 部署一个测试用的 ERC20 代币作为项目代币
 */

const { ethers } = require("hardhat");
const fs = require("fs");
const path = require("path");

async function main() {
    console.log("\n" + "═".repeat(80));
    console.log("                    部署项目代币");
    console.log("═".repeat(80) + "\n");

    // 加载配置
    const accountsConfig = require("../config/accounts.json");
    const contractsConfig = require("../config/contracts.json");

    // 获取 projectOwner 账户
    const projectOwnerPrivateKey = accountsConfig.accounts.projectOwner.privateKey;
    const provider = ethers.provider;
    const projectOwner = new ethers.Wallet(projectOwnerPrivateKey, provider);

    console.log(`📍 项目方地址: ${projectOwner.address}\n`);

    // 检查余额
    const balance = await provider.getBalance(projectOwner.address);
    console.log(`💰 BNB 余额: ${ethers.formatEther(balance)}`);

    if (balance < ethers.parseEther("0.01")) {
        throw new Error("BNB 余额不足，请先运行 02_distributeAssets.js");
    }

    console.log("✅ 余额充足\n");

    // 部署项目代币
    console.log("🚀 部署项目代币...\n");

    const TestProjectToken = await ethers.getContractFactory("TestProjectToken", projectOwner);

    const tokenName = process.env.TEST_PROJECT_TOKEN_NAME || "Test Project Token";
    const tokenSymbol = process.env.TEST_PROJECT_TOKEN_SYMBOL || "TPT";
    const initialSupply = ethers.parseUnits(
        process.env.TEST_PROJECT_TOKEN_INITIAL_SUPPLY || "1000000",
        18
    );
    const configuredMaxSupply = ethers.parseUnits(
        process.env.TEST_PROJECT_TOKEN_MAX_SUPPLY || "10000000",
        18
    );

    const projectToken = await TestProjectToken.deploy(
        tokenName,
        tokenSymbol,
        initialSupply,
        configuredMaxSupply,
        projectOwner.address
    );

    await projectToken.waitForDeployment();
    const projectTokenAddress = await projectToken.getAddress();

    console.log(`✅ 项目代币已部署: ${projectTokenAddress}\n`);

    // 验证部署
    console.log("🔍 验证代币信息:");
    const name = await projectToken.name();
    const symbol = await projectToken.symbol();
    const decimals = await projectToken.decimals();
    const totalSupply = await projectToken.totalSupply();
    const maxSupply = await projectToken.maxSupply();
    const ownerBalance = await projectToken.balanceOf(projectOwner.address);

    console.log(`   名称: ${name}`);
    console.log(`   符号: ${symbol}`);
    console.log(`   精度: ${decimals}`);
    console.log(`   总供应量: ${ethers.formatEther(totalSupply)}`);
    console.log(`   供应量硬顶: ${ethers.formatEther(maxSupply)}`);
    console.log(`   项目方余额: ${ethers.formatEther(ownerBalance)}\n`);

    // 更新配置文件
    contractsConfig.contracts.projectToken = projectTokenAddress;

    const configPath = path.join(__dirname, "../config/contracts.json");
    fs.writeFileSync(configPath, JSON.stringify(contractsConfig, null, 2));

    console.log(`💾 配置已更新: ${configPath}\n`);

    // 保存部署记录
    const recordPath = path.join(__dirname, "../reports/projectToken.json");
    const record = {
        timestamp: new Date().toISOString(),
        address: projectTokenAddress,
        deployer: projectOwner.address,
        name,
        symbol,
        decimals: Number(decimals),
        totalSupply: ethers.formatEther(totalSupply),
        maxSupply: ethers.formatEther(maxSupply),
        network: "bsc-testnet",
        explorerUrl: `https://testnet.bscscan.com/token/${projectTokenAddress}`
    };

    const dir = path.dirname(recordPath);
    if (!fs.existsSync(dir)) {
        fs.mkdirSync(dir, { recursive: true });
    }

    fs.writeFileSync(recordPath, JSON.stringify(record, null, 2));
    console.log(`💾 部署记录已保存: ${recordPath}\n`);

    console.log("─".repeat(80));
    console.log("\n✅ 项目代币部署完成!\n");

    console.log("📋 下一步:");
    console.log("   运行 04_verifySetup.js 验证所有设置\n");

    console.log("🔗 区块链浏览器:");
    console.log(`   ${record.explorerUrl}\n`);

    console.log("═".repeat(80) + "\n");
}

main()
    .then(() => process.exit(0))
    .catch(error => {
        console.error("\n❌ 部署失败:", error.message);
        console.error(error);
        process.exit(1);
    });
