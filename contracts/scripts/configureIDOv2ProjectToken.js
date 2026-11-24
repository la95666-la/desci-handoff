const fs = require("fs");
const path = require("path");
const hre = require("hardhat");
const { ethers } = hre;

function required(name) {
    const value = String(process.env[name] || "").trim();
    if (!value) throw new Error(`缺少 ${name}`);
    return value;
}

function address(name) {
    const value = required(name);
    if (!ethers.isAddress(value) || value === ethers.ZeroAddress) throw new Error(`${name}必须是有效非零地址`);
    return ethers.getAddress(value);
}

async function main() {
    required("JC_DEPLOY_RPC");
    const configuredChainID = BigInt(required("JC_DEPLOY_CHAIN_ID"));
    required("JC_DEPLOY_PRIVATE_KEY");
    const proxyAddress = address("IDO_V2_PROXY_ADDRESS");
    const tokenAddress = address("IDO_V2_PROJECT_TOKEN_ADDRESS");
    if (proxyAddress === tokenAddress) throw new Error("项目币不能与IDO V2 proxy相同");

    const [signer] = await ethers.getSigners();
    const network = await ethers.provider.getNetwork();
    if (network.chainId !== configuredChainID) throw new Error(`chainId不匹配: RPC=${network.chainId} configured=${configuredChainID}`);
    if (network.chainId !== 97n && process.env.IDO_V2_ALLOW_MAINNET_GOVERNANCE !== "I_UNDERSTAND_SAFE_IS_REQUIRED") {
        throw new Error("该脚本默认仅允许BSC Testnet；主网项目币治理必须走Safe，除非显式设置批准口令");
    }
    if ((await ethers.provider.getCode(proxyAddress)) === "0x") throw new Error("IDO V2 proxy没有合约代码");
    if ((await ethers.provider.getCode(tokenAddress)) === "0x") throw new Error("项目币没有合约代码");

    const proxy = await ethers.getContractAt("JoinCareIDOv2", proxyAddress);
    const owner = await proxy.owner();
    if (owner.toLowerCase() !== signer.address.toLowerCase()) throw new Error(`signer不是当前owner: owner=${owner} signer=${signer.address}`);
    const token = new ethers.Contract(tokenAddress, ["function decimals() view returns (uint8)"], ethers.provider);
    const decimals = Number(await token.decimals());
    if (!Number.isInteger(decimals) || decimals < 1 || decimals > 18) throw new Error(`项目币decimals不受支持: ${decimals}`);

    const before = await proxy.projectTokenConfigs(tokenAddress);
    let receipt = null;
    if (!before.allowed || Number(before.decimals) !== decimals) {
        const tx = await proxy.configureProjectToken(tokenAddress, true);
        receipt = await tx.wait();
    }
    const after = await proxy.projectTokenConfigs(tokenAddress);
    if (!after.allowed || Number(after.decimals) !== decimals) throw new Error("项目币治理配置验证失败");

    const manifest = {
        schema: "joincare.ido-v2.project-token-governance.v1",
        chainId: network.chainId.toString(),
        proxyAddress,
        owner,
        projectTokenAddress: tokenAddress,
        projectTokenDecimals: decimals,
        allowed: true,
        transactionHash: receipt?.hash || null,
        blockNumber: receipt?.blockNumber || null,
        blockHash: receipt?.blockHash || null,
        idempotent: receipt === null,
        configuredAt: new Date().toISOString(),
    };
    const output = required("IDO_V2_PROJECT_TOKEN_MANIFEST");
    const destination = path.resolve(output);
    fs.mkdirSync(path.dirname(destination), { recursive: true });
    fs.writeFileSync(destination, `${JSON.stringify(manifest, null, 2)}\n`, { flag: "wx" });
    console.log(`IDO_V2_PROJECT_TOKEN_MANIFEST_JSON=${JSON.stringify(manifest)}`);
}

main().then(() => process.exit(0)).catch((error) => {
    console.error(error.message || error);
    process.exit(1);
});
