const fs = require("fs");
const path = require("path");
const hre = require("hardhat");
const { ethers, upgrades } = hre;

function required(name) {
    const value = String(process.env[name] || "").trim();
    if (!value) throw new Error(`缺少 ${name}`);
    return value;
}

function address(name) {
    const value = required(name);
    if (!ethers.isAddress(value) || value === ethers.ZeroAddress) {
        throw new Error(`${name} 必须是有效非零地址`);
    }
    return ethers.getAddress(value);
}

function uint16(name) {
    const value = required(name);
    if (!/^\d+$/.test(value) || Number(value) > 1000) {
        throw new Error(`${name} 必须是0到1000的basis points整数`);
    }
    return Number(value);
}

async function assertToken(tokenAddress, expectedDecimals, label) {
    if ((await ethers.provider.getCode(tokenAddress)) === "0x") {
        throw new Error(`${label} 地址没有合约代码`);
    }
    const token = new ethers.Contract(tokenAddress, ["function decimals() view returns (uint8)"], ethers.provider);
    const actual = Number(await token.decimals());
    if (actual !== expectedDecimals) {
        throw new Error(`${label} decimals不匹配: expected=${expectedDecimals} actual=${actual}`);
    }
}

async function main() {
    required("JC_DEPLOY_RPC");
    const configuredChainID = BigInt(required("JC_DEPLOY_CHAIN_ID"));
    required("JC_DEPLOY_PRIVATE_KEY");
    const feeReceiver = address("IDO_V2_FEE_RECEIVER");
    const feeRate = uint16("IDO_V2_SUCCESS_FEE_RATE");
    const usdt = address("IDO_V2_USDT_ADDRESS");
    const usdc = address("IDO_V2_USDC_ADDRESS");
    const usdtDecimals = Number(required("IDO_V2_USDT_DECIMALS"));
    const usdcDecimals = Number(required("IDO_V2_USDC_DECIMALS"));
    if (usdt === usdc) throw new Error("USDT与USDC地址不能相同");
    if (![usdtDecimals, usdcDecimals].every((value) => Number.isInteger(value) && value >= 1 && value <= 18)) {
        throw new Error("支付币decimals必须是1到18的整数");
    }

    const [deployer] = await ethers.getSigners();
    const initialOwnerRaw = String(process.env.IDO_V2_INITIAL_OWNER || "").trim();
    const initialOwner = initialOwnerRaw ? address("IDO_V2_INITIAL_OWNER") : deployer.address;
    const network = await ethers.provider.getNetwork();
    if (network.chainId !== configuredChainID) {
        throw new Error(`chainId不匹配: RPC=${network.chainId} configured=${configuredChainID}`);
    }
    await assertToken(usdt, usdtDecimals, "USDT");
    await assertToken(usdc, usdcDecimals, "USDC");

    const Factory = await ethers.getContractFactory("JoinCareIDOv2");
    const operator = address("IDO_V2_OPERATOR");
    const proxy = await upgrades.deployProxy(Factory, [initialOwner, operator, feeReceiver, feeRate], {
        kind: "uups",
        initializer: "initialize",
    });
    const deploymentTx = proxy.deploymentTransaction();
    await proxy.waitForDeployment();
    const proxyAddress = await proxy.getAddress();
    const receipt = await deploymentTx.wait();
    const implementationAddress = await upgrades.erc1967.getImplementationAddress(proxyAddress);
    const implementationCode = await ethers.provider.getCode(implementationAddress);
    const implementationCodeHash = ethers.keccak256(implementationCode);

    if ((await proxy.owner()).toLowerCase() !== initialOwner.toLowerCase()) throw new Error("owner初始化不匹配");
    if ((await proxy.operator()).toLowerCase() !== operator.toLowerCase()) throw new Error("operator初始化不匹配");
    if ((await proxy.feeReceiver()).toLowerCase() !== feeReceiver.toLowerCase()) throw new Error("feeReceiver初始化不匹配");
    if (Number(await proxy.successFeeRate()) !== feeRate) throw new Error("successFeeRate初始化不匹配");

    // Token governance requires the configured owner. For deployment automation,
    // initialOwner must be the deployer; ownership can move through Ownable2Step afterwards.
    if (initialOwner.toLowerCase() !== deployer.address.toLowerCase()) {
        throw new Error("IDO_V2_INITIAL_OWNER必须先使用deployer，以完成支付币治理配置；之后再走两步Safe迁移");
    }
    await (await proxy.configurePaymentToken(usdt, true)).wait();
    await (await proxy.configurePaymentToken(usdc, true)).wait();
    for (const [label, token, decimals] of [["USDT", usdt, usdtDecimals], ["USDC", usdc, usdcDecimals]]) {
        const config = await proxy.paymentTokenConfigs(token);
        if (!config.allowed || Number(config.decimals) !== decimals) throw new Error(`${label}治理配置验证失败`);
    }

    const block = await ethers.provider.getBlock(receipt.blockNumber);
    const manifest = {
        schema: "joincare.ido-v2.deployment.v1",
        network: hre.network.name,
        chainId: network.chainId.toString(),
        deployer: deployer.address,
        owner: initialOwner,
        operator,
        feeReceiver,
        successFeeRate: feeRate,
        proxyAddress,
        implementationAddress,
        implementationCodeHash,
        deploymentTxHash: receipt.hash,
        deploymentBlockNumber: receipt.blockNumber,
        deploymentBlockHash: receipt.blockHash,
        deploymentBlockParentHash: block.parentHash,
        paymentTokens: {
            USDT: { address: usdt, decimals: usdtDecimals, allowed: true },
            USDC: { address: usdc, decimals: usdcDecimals, allowed: true },
        },
        deployedAt: new Date().toISOString(),
    };
    const output = process.env.IDO_V2_DEPLOYMENT_MANIFEST;
    if (output) {
        const destination = path.resolve(output);
        fs.mkdirSync(path.dirname(destination), { recursive: true });
        fs.writeFileSync(destination, `${JSON.stringify(manifest, null, 2)}\n`, { flag: "wx" });
    }
    console.log(`IDO_V2_DEPLOYMENT_MANIFEST_JSON=${JSON.stringify(manifest)}`);
}

main().then(() => process.exit(0)).catch((error) => {
    console.error(error.message || error);
    process.exit(1);
});
