const hre = require("hardhat");
const { ethers, upgrades } = hre;

function check(condition, message) {
    if (!condition) throw new Error(message);
}

async function waitUntil(provider, timestamp, chainId) {
    if (chainId === 31337n) {
        await provider.send("evm_setNextBlockTimestamp", [timestamp]);
        await provider.send("evm_mine", []);
        return;
    }
    for (;;) {
        const latest = await provider.getBlock("latest");
        if (latest.timestamp >= timestamp) return;
        await new Promise((resolve) => setTimeout(resolve, Math.min((timestamp - latest.timestamp) * 1000, 5000)));
    }
}

async function main() {
    const provider = new ethers.JsonRpcProvider(String(process.env.JC_DEPLOY_RPC || "http://127.0.0.1:8545"));
    const network = await provider.getNetwork();
    check([97n, 31337n].includes(network.chainId), `V2 lifecycle仅允许BSC Testnet/Hardhat，当前${network.chainId}`);
    let owner, projectOwner, user, feeReceiver;
    if (network.chainId === 31337n) {
        [owner, projectOwner, user, feeReceiver] = await Promise.all([0, 1, 2, 3].map((index) => provider.getSigner(index)));
    } else {
        const keys = ["JC_DEPLOY_PRIVATE_KEY", "IDO_V2_E2E_PROJECT_OWNER_KEY", "IDO_V2_E2E_USER_KEY", "IDO_V2_E2E_FEE_RECEIVER_KEY"];
        const values = keys.map((name) => String(process.env[name] || "").trim());
        if (values.some((value) => !value)) throw new Error(`BSC Testnet E2E需要4个独立账户私钥: ${keys.join(",")}`);
        [owner, projectOwner, user, feeReceiver] = values.map((value) => new ethers.Wallet(value.startsWith("0x") ? value : `0x${value}`, provider));
        const distinct = new Set([owner.address, projectOwner.address, user.address, feeReceiver.address].map((value) => value.toLowerCase()));
        check(distinct.size === 4, "BSC Testnet E2E四个角色地址必须互不相同");
    }

    const USDT = await ethers.getContractFactory("MockUSDT", owner);
    const usdt = await USDT.deploy(ethers.parseUnits("1000000", 6), ethers.parseUnits("10000000", 6), owner.address);
    const USDC = await ethers.getContractFactory("MockUSDC", owner);
    const usdc = await USDC.deploy(ethers.parseUnits("1000000", 18), ethers.parseUnits("10000000", 18), owner.address);
    const Token = await ethers.getContractFactory("TestProjectToken", owner);
    const token = await Token.deploy("V2 E2E Project", "V2E2E", ethers.parseUnits("1000000", 18), ethers.parseUnits("10000000", 18), projectOwner.address);
    const Platform = await ethers.getContractFactory("JoinCareIDOv2", owner);
    const platform = await upgrades.deployProxy(Platform, [owner.address, owner.address, feeReceiver.address, 200], { kind: "uups", initializer: "initialize" });
    await Promise.all([usdt.waitForDeployment(), usdc.waitForDeployment(), token.waitForDeployment(), platform.waitForDeployment()]);
    await (await platform.configurePaymentToken(await usdt.getAddress(), true)).wait();
    await (await platform.configurePaymentToken(await usdc.getAddress(), true)).wait();
    await (await platform.configureProjectToken(await token.getAddress(), true)).wait();

    await (await usdt.transfer(user.address, ethers.parseUnits("1000", 6))).wait();
    await (await usdc.transfer(user.address, ethers.parseUnits("1000", 18))).wait();
    const latest = (await provider.getBlock("latest")).timestamp;
    const startDelay = Number(process.env.IDO_V2_E2E_START_DELAY || (network.chainId === 31337n ? 30 : 120));
    const duration = Number(process.env.IDO_V2_E2E_DURATION || (network.chainId === 31337n ? 60 : 180));
    if (!Number.isInteger(startDelay) || !Number.isInteger(duration) || startDelay < 30 || duration < 60) throw new Error("E2E时间参数无效");
    const projectSpecs = [
        { id: 1n, payment: usdt, decimals: 6, soft: "100", buy: "120" },
        { id: 2n, payment: usdc, decimals: 18, soft: "500", buy: "50" },
    ];
    for (const [index, spec] of projectSpecs.entries()) {
        const start = latest + startDelay;
        const end = start + duration + index;
        await (await platform.createProject(
            spec.id, await token.getAddress(), await spec.payment.getAddress(), projectOwner.address, feeReceiver.address, 200,
            ethers.parseUnits("2", spec.decimals), ethers.parseUnits("1000", spec.decimals), ethers.parseUnits(spec.soft, spec.decimals), ethers.parseUnits("600", spec.decimals),
            start, end, [end + 30], [10000]
        )).wait();
        await (await token.connect(projectOwner).approve(await platform.getAddress(), ethers.parseUnits("1000", 18))).wait();
        await (await platform.connect(projectOwner).depositProjectTokens(spec.id, ethers.parseUnits("1000", 18))).wait();
        await (await platform.activateProject(spec.id)).wait();
    }
    await waitUntil(provider, latest + startDelay, network.chainId);
    for (const spec of projectSpecs) {
        const amount = ethers.parseUnits(spec.buy, spec.decimals);
        await (await spec.payment.connect(user).approve(await platform.getAddress(), amount)).wait();
        await (await platform.connect(user).purchase(spec.id, amount)).wait();
        const purchase = await platform.getUserPurchase(spec.id, user.address);
        check(purchase.paymentAmount === amount, `project ${spec.id} payment amount mismatch`);
    }
    const proxy = await platform.getAddress();
    check(await platform.reservedBalances(await usdt.getAddress()) === await usdt.balanceOf(proxy), "USDT reserve mismatch");
    check(await platform.reservedBalances(await usdc.getAddress()) === await usdc.balanceOf(proxy), "USDC reserve mismatch");

    await waitUntil(provider, latest + startDelay + duration + 2, network.chainId);
    await (await platform.endProject(1)).wait();
    await (await platform.endProject(2)).wait();
    check(Number((await platform.getProject(1)).status) === 2, "USDT project should be Successful");
    check(Number((await platform.getProject(2)).status) === 3, "USDC project should be Failed");

    const refundBefore = await usdc.balanceOf(user.address);
    await (await platform.connect(user).refund(2)).wait();
    check((await usdc.balanceOf(user.address)) - refundBefore === ethers.parseUnits("50", 18), "USDC refund mismatch");
    await (await platform.connect(projectOwner).withdrawRaisedFunds(1)).wait();
    await (await platform.withdrawPlatformFee(1)).wait();
    await (await platform.connect(projectOwner).withdrawProjectTokens(1)).wait();
    await (await platform.connect(projectOwner).withdrawProjectTokens(2)).wait();
    check(await platform.reservedBalances(await usdt.getAddress()) === 0n, "USDT reserve should be zero after settlement");
    check(await platform.reservedBalances(await usdc.getAddress()) === 0n, "USDC reserve should be zero after refund");

    console.log(JSON.stringify({ chainId: network.chainId.toString(), proxy, status: "PASS", projects: ["USDT-success", "USDC-failed-refunded"] }, null, 2));
}

main().then(() => process.exit(0)).catch((error) => {
    console.error(error);
    process.exit(1);
});
