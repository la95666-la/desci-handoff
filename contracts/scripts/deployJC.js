const { ethers } = require("hardhat");
require("dotenv").config();

/**
 * 部署 JC 平台代币（A3 口径：原 JCC 统一改名 JC；合约含 10 亿 mint 硬顶）
 *
 * 部署流程：
 * 1. 部署 JC ERC20 代币（owner = 部署账户）
 * 2. 验证部署成功（名称/符号/总量/硬顶）
 * 3. 输出代币地址与后续多签迁移指引
 *
 * 环境变量：
 * - JC_INITIAL_SUPPLY  初始供应量（token 单位，默认 1000000）
 */
async function main() {
    console.log("========================================");
    console.log("开始部署 JC 代币");
    console.log("========================================\n");

    const [deployer] = await ethers.getSigners();
    console.log("部署账户:", deployer.address);

    const balance = await ethers.provider.getBalance(deployer.address);
    console.log("账户余额:", ethers.formatEther(balance), "BNB\n");

    console.log("正在部署 JC 代币...");

    const JC = await ethers.getContractFactory("JC");
    const initialSupply = Number(process.env.JC_INITIAL_SUPPLY || 1000000);

    const jc = await JC.deploy(initialSupply);
    await jc.waitForDeployment();

    const jcAddress = await jc.getAddress();
    console.log("✅ JC 代币部署成功!");
    console.log("代币地址:", jcAddress);

    console.log("\n正在验证部署...");

    const name = await jc.name();
    const symbol = await jc.symbol();
    const decimals = await jc.decimals();
    const totalSupply = await jc.totalSupply();
    const maxSupply = await jc.MAX_SUPPLY();
    const ownerBalance = await jc.balanceOf(deployer.address);

    console.log("✅ 部署验证成功!");
    console.log("- 代币名称:", name);
    console.log("- 代币符号:", symbol);
    console.log("- 小数位数:", decimals);
    console.log("- 当前总供应:", ethers.formatUnits(totalSupply, decimals), symbol);
    console.log("- 总量硬顶:", ethers.formatUnits(maxSupply, decimals), symbol);
    console.log("- 部署者余额:", ethers.formatUnits(ownerBalance, decimals), symbol);

    const deploymentInfo = {
        network: (await ethers.provider.getNetwork()).name,
        chainId: (await ethers.provider.getNetwork()).chainId.toString(),
        deployer: deployer.address,
        jcAddress: jcAddress,
        name: name,
        symbol: symbol,
        decimals: decimals.toString(),
        totalSupply: totalSupply.toString(),
        maxSupply: maxSupply.toString(),
        deployedAt: new Date().toISOString()
    };

    console.log("\n========================================");
    console.log("部署信息:");
    console.log(JSON.stringify(deploymentInfo, null, 2));
    console.log("========================================");

    console.log("\n⚠️  重要提醒:");
    console.log("1. 请保存 JC 代币地址:", jcAddress);
    console.log("2. 将此地址写入 .env 的 JC_TOKEN_ADDRESS（JoinCare IDO 合约部署需要）");
    console.log("3. 生产环境必须将 owner 迁移到多签（Gnosis Safe）：");
    console.log("   MULTISIG_ADDRESS=<safe> npx hardhat run scripts/transferOwnership.js --network <net>\n");

    return { jcAddress };
}

main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error("部署失败:", error);
        process.exit(1);
    });
