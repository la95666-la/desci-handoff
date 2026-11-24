const hre = require("hardhat");
const { ethers } = hre;
require("dotenv").config();

function parsePositiveUnits(name, value, decimals) {
    if (!/^\d+(\.\d+)?$/.test(value)) {
        throw new Error(`${name} 必须是正数十进制字符串`);
    }
    const amount = ethers.parseUnits(value, decimals);
    if (amount === 0n) throw new Error(`${name} 必须大于 0`);
    return amount;
}

async function deployToken(contractName, decimals, owner, initialText, capText) {
    const initialSupply = parsePositiveUnits(`${contractName}_INITIAL_SUPPLY`, initialText, decimals);
    const maxSupply = parsePositiveUnits(`${contractName}_MAX_SUPPLY`, capText, decimals);
    if (initialSupply > maxSupply) {
        throw new Error(`${contractName} 初始供应量不能超过供应硬顶`);
    }

    const Token = await ethers.getContractFactory(contractName);
    const token = await Token.deploy(initialSupply, maxSupply, owner);
    await token.waitForDeployment();

    return {
        address: await token.getAddress(),
        owner: await token.owner(),
        name: await token.name(),
        symbol: await token.symbol(),
        decimals: Number(await token.decimals()),
        totalSupply: ethers.formatUnits(await token.totalSupply(), decimals),
        maxSupply: ethers.formatUnits(await token.maxSupply(), decimals),
    };
}

async function main() {
    if (hre.network.name === "jcTarget") {
        for (const name of ["JC_DEPLOY_RPC", "JC_DEPLOY_CHAIN_ID", "JC_DEPLOY_PRIVATE_KEY"]) {
            if (!String(process.env[name] || "").trim()) {
                throw new Error(`使用 jcTarget 时缺少 ${name}`);
            }
        }
    }

    const [deployer] = await ethers.getSigners();
    const network = await ethers.provider.getNetwork();
    const chainId = Number(network.chainId);
    if (process.env.JC_DEPLOY_CHAIN_ID && network.chainId !== BigInt(process.env.JC_DEPLOY_CHAIN_ID)) {
        throw new Error(`chainId 不匹配: RPC=${network.chainId} 配置=${process.env.JC_DEPLOY_CHAIN_ID}`);
    }
    if (!new Set([97, 1337, 31337]).has(chainId)) {
        throw new Error(`MockUSDT/MockUSDC 仅允许部署到 BSC Testnet/本地链，当前 chainId=${chainId}`);
    }

    const owner = process.env.MOCK_PAYMENT_TOKEN_OWNER || deployer.address;
    if (!ethers.isAddress(owner) || owner === ethers.ZeroAddress) {
        throw new Error("MOCK_PAYMENT_TOKEN_OWNER 必须是有效非零地址");
    }

    const initialText = process.env.MOCK_PAYMENT_INITIAL_SUPPLY || "1000000";
    const capText = process.env.MOCK_PAYMENT_MAX_SUPPLY || "10000000";
    // Keep a single deployer nonce stream deterministic on remote RPCs.
    const usdt = await deployToken("MockUSDT", 6, owner, initialText, capText);
    const usdc = await deployToken("MockUSDC", 18, owner, initialText, capText);

    console.log(JSON.stringify({
        network: network.name,
        chainId: network.chainId.toString(),
        deployer: deployer.address,
        MockUSDT: usdt,
        MockUSDC: usdc,
        deployedAt: new Date().toISOString(),
    }, null, 2));
}

main().then(() => process.exit(0)).catch((error) => {
    console.error("部署失败:", error.message);
    process.exit(1);
});
