const { ethers, upgrades } = require("hardhat");
require("dotenv").config();

/**
 * 升级JoinCare IDO平台（UUPS代理模式）
 *
 * 升级流程：
 * 1. 验证存储布局兼容性
 * 2. 部署新的实现合约
 * 3. 调用代理的upgradeTo函数
 * 4. 验证升级成功
 */
async function main() {
	for (const name of ["JC_DEPLOY_RPC", "JC_DEPLOY_CHAIN_ID", "JC_DEPLOY_PRIVATE_KEY"]) {
		if (!String(process.env[name] || "").trim()) {
			throw new Error(`缺少 ${name}；拒绝使用 jcTarget localhost 兜底`);
		}
	}
    console.log("========================================");
    console.log("开始升级 JoinCare IDO 平台");
    console.log("========================================\n");

    // 1. 获取升级账户
    const [upgrader] = await ethers.getSigners();
    console.log("升级账户:", upgrader.address);

    const balance = await ethers.provider.getBalance(upgrader.address);
    console.log("账户余额:", ethers.formatEther(balance), "BNB\n");

    // 2. 获取现有代理地址
    const PROXY_ADDRESS = process.env.PROXY_ADDRESS;

	if (!ethers.isAddress(PROXY_ADDRESS) || PROXY_ADDRESS === ethers.ZeroAddress) {
        throw new Error("请在.env文件中设置有效的PROXY_ADDRESS");
    }

    console.log("现有代理地址:", PROXY_ADDRESS);

	const network = await ethers.provider.getNetwork();
	if (process.env.JC_DEPLOY_CHAIN_ID && network.chainId !== BigInt(process.env.JC_DEPLOY_CHAIN_ID)) {
		throw new Error(`chainId 不匹配: RPC=${network.chainId} 配置=${process.env.JC_DEPLOY_CHAIN_ID}`);
	}
	if ((await ethers.provider.getCode(PROXY_ADDRESS)) === "0x") {
		throw new Error("PROXY_ADDRESS 上没有合约代码");
	}

    // 3. 获取当前实现地址
    const currentImplementation = await upgrades.erc1967.getImplementationAddress(PROXY_ADDRESS);
    console.log("当前实现地址:", currentImplementation);

    // 4. 验证当前合约状态
    const proxy = await ethers.getContractAt("JoinCare_v1", PROXY_ADDRESS);
    const owner = await proxy.owner();
    console.log("合约所有者:", owner);

    if (owner.toLowerCase() !== upgrader.address.toLowerCase()) {
        throw new Error("只有合约所有者可以升级合约");
    }

    // 5. 部署新的实现合约
    console.log("\n正在部署新的实现合约...");

    // 注意：这里假设新版本合约名为 JoinCare_v2
    // 如果是升级到v1的新版本，请使用 "JoinCare_v1"
    const NewJoinCare = await ethers.getContractFactory("JoinCare_v1");

    console.log("正在验证存储布局兼容性...");

    const upgraded = await upgrades.upgradeProxy(PROXY_ADDRESS, NewJoinCare, {
        kind: "uups"
    });

    await upgraded.waitForDeployment();

    console.log("✅ 升级成功!");

    // 6. 获取新的实现地址
    const newImplementation = await upgrades.erc1967.getImplementationAddress(PROXY_ADDRESS);
    console.log("新实现地址:", newImplementation);
	if (newImplementation.toLowerCase() === currentImplementation.toLowerCase()) {
		throw new Error("实现地址未变化；拒绝把无实际升级当成成功");
	}

    // 7. 验证升级
    console.log("\n正在验证升级...");

    const jccToken = await upgraded.jccToken();
    const platformFees = await upgraded.platformFees();
	const ownerAfter = await upgraded.owner();

    console.log("✅ 升级验证成功!");
    console.log("- JCC代币:", jccToken);
    console.log("- 费用接收地址:", platformFees.feeReceiver);
    console.log("- 费率:", platformFees.successFeeRate.toString(), "基点");
	console.log("- 合约所有者:", ownerAfter);
	if (ownerAfter.toLowerCase() !== owner.toLowerCase()) {
		throw new Error(`升级后 owner 发生变化: ${owner} -> ${ownerAfter}`);
	}
	if (jccToken === ethers.ZeroAddress || platformFees.feeReceiver === ethers.ZeroAddress) {
		throw new Error("升级后关键地址 getter 返回零地址");
	}

    // 8. 保存升级信息
    const upgradeInfo = {
		network: network.name,
		chainId: network.chainId.toString(),
        upgrader: upgrader.address,
        proxyAddress: PROXY_ADDRESS,
        oldImplementation: currentImplementation,
        newImplementation: newImplementation,
        upgradedAt: new Date().toISOString()
    };

    console.log("\n========================================");
    console.log("升级信息:");
    console.log(JSON.stringify(upgradeInfo, null, 2));
    console.log("========================================");

    console.log("\n⚠️  重要提醒:");
    console.log("1. 代理地址保持不变:", PROXY_ADDRESS);
    console.log("2. 实现地址已更新:", newImplementation);
    console.log("3. 请在BscScan上验证新的实现合约");
    console.log("4. 建议将升级信息保存到文件中\n");

    return {
        proxy: PROXY_ADDRESS,
        oldImplementation: currentImplementation,
        newImplementation: newImplementation
    };
}

// 执行升级
main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error("升级失败:", error);
        process.exit(1);
    });
