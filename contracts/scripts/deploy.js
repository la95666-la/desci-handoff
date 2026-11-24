const { ethers, upgrades } = require("hardhat");
require("dotenv").config();

/**
 * 部署JoinCare IDO平台（UUPS代理模式）
 *
 * 部署流程：
 * 1. 部署实现合约（JoinCare_v1）
 * 2. 部署UUPS代理并初始化
 * 3. 验证部署成功
 * 4. 输出关键地址
 */
async function main() {
	for (const name of ["JC_DEPLOY_RPC", "JC_DEPLOY_CHAIN_ID", "JC_DEPLOY_PRIVATE_KEY"]) {
		if (!String(process.env[name] || "").trim()) {
			throw new Error(`缺少 ${name}；拒绝使用 jcTarget localhost 兜底`);
		}
	}
    console.log("========================================");
    console.log("开始部署 JoinCare IDO 平台");
    console.log("========================================\n");

    // 1. 获取部署账户
    const [deployer] = await ethers.getSigners();
    console.log("部署账户:", deployer.address);
	const network = await ethers.provider.getNetwork();
	if (process.env.JC_DEPLOY_CHAIN_ID && network.chainId !== BigInt(process.env.JC_DEPLOY_CHAIN_ID)) {
		throw new Error(`chainId 不匹配: RPC=${network.chainId} 配置=${process.env.JC_DEPLOY_CHAIN_ID}`);
	}

    const balance = await ethers.provider.getBalance(deployer.address);
    console.log("账户余额:", ethers.formatEther(balance), "BNB\n");

    // 2. 加载环境变量
    const JCC_TOKEN_ADDRESS = process.env.JCC_TOKEN_ADDRESS;
    const FEE_RECEIVER_ADDRESS = process.env.FEE_RECEIVER_ADDRESS || deployer.address;
    const INITIAL_FEE_RATE = process.env.INITIAL_FEE_RATE || "200"; // 默认2%

    console.log("初始化参数:");
    console.log("- JCC代币地址:", JCC_TOKEN_ADDRESS);
    console.log("- 费用接收地址:", FEE_RECEIVER_ADDRESS);
    console.log("- 初始费率:", INITIAL_FEE_RATE, "基点 (", parseInt(INITIAL_FEE_RATE) / 100, "%)\n");

    // 3. 验证参数
	if (!ethers.isAddress(JCC_TOKEN_ADDRESS) || JCC_TOKEN_ADDRESS === ethers.ZeroAddress) {
        throw new Error("请在.env文件中设置有效的JCC_TOKEN_ADDRESS");
    }
	if ((await ethers.provider.getCode(JCC_TOKEN_ADDRESS)) === "0x") {
		throw new Error("JCC_TOKEN_ADDRESS 上没有合约代码");
	}
	if (!ethers.isAddress(FEE_RECEIVER_ADDRESS) || FEE_RECEIVER_ADDRESS === ethers.ZeroAddress) {
		throw new Error("FEE_RECEIVER_ADDRESS 必须是有效非零地址");
	}

    // 4. 部署UUPS代理
    console.log("正在部署 JoinCare_v1 合约（UUPS代理模式）...");

    const JoinCare = await ethers.getContractFactory("JoinCare_v1");

    const proxy = await upgrades.deployProxy(
        JoinCare,
        [JCC_TOKEN_ADDRESS, FEE_RECEIVER_ADDRESS, INITIAL_FEE_RATE],
        {
            kind: "uups",
            initializer: "initialize"
        }
    );

    await proxy.waitForDeployment();
    const proxyAddress = await proxy.getAddress();

    console.log("✅ 代理合约部署成功!");
    console.log("代理地址:", proxyAddress);

    // 5. 获取实现合约地址
    const implementationAddress = await upgrades.erc1967.getImplementationAddress(proxyAddress);
    console.log("实现合约地址:", implementationAddress);

    // 6. 验证部署
    console.log("\n正在验证部署...");

    const jccToken = await proxy.jccToken();
    const platformFees = await proxy.platformFees();
    const owner = await proxy.owner();

    console.log("✅ 部署验证成功!");
    console.log("- JCC代币:", jccToken);
    console.log("- 费用接收地址:", platformFees.feeReceiver);
    console.log("- 费率:", platformFees.successFeeRate.toString(), "基点");
    console.log("- 合约所有者:", owner);

    // 7. 保存部署信息
    const deploymentInfo = {
		network: network.name,
		chainId: network.chainId.toString(),
        deployer: deployer.address,
        proxyAddress: proxyAddress,
        implementationAddress: implementationAddress,
        jccToken: jccToken,
        feeReceiver: platformFees.feeReceiver,
        feeRate: platformFees.successFeeRate.toString(),
        deployedAt: new Date().toISOString()
    };

    console.log("\n========================================");
    console.log("部署信息:");
    console.log(JSON.stringify(deploymentInfo, null, 2));
    console.log("========================================");

    console.log("\n⚠️  重要提醒:");
    console.log("1. 请保存代理地址:", proxyAddress);
    console.log("2. 请在BscScan上验证合约");
    console.log("3. 升级时使用代理地址，而非实现地址");
    console.log("4. 建议将部署信息保存到文件中\n");

    return {
        proxy: proxyAddress,
        implementation: implementationAddress
    };
}

// 执行部署
main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error("部署失败:", error);
        process.exit(1);
    });
