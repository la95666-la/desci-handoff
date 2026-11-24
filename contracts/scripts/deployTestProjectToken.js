const { ethers } = require("hardhat");
require("dotenv").config();

/**
 * 部署 JoinCare IDO 测试项目代币。
 *
 * 可选环境变量：
 * - TEST_PROJECT_TOKEN_NAME            默认 JoinCare Test Project Token
 * - TEST_PROJECT_TOKEN_SYMBOL          默认 JCPT
 * - TEST_PROJECT_TOKEN_INITIAL_SUPPLY  默认 1000000（token 单位）
 * - TEST_PROJECT_TOKEN_MAX_SUPPLY      默认 10000000（token 单位）
 * - TEST_PROJECT_TOKEN_OWNER           默认部署账户
 * - TEST_TOKEN_ALLOW_MAINNET           设为 I_UNDERSTAND_REAL_CHAIN 才允许 BSC 主网(56)
 *
 * 本地示例：
 *   npm run deploy:test-project-token -- --network localhost
 *
 * BSC 测试网示例（使用 hardhat.config.js 的 jcTarget）：
 *   JC_DEPLOY_RPC=<rpc> JC_DEPLOY_CHAIN_ID=97 JC_DEPLOY_PRIVATE_KEY=<key> \
 *   npm run deploy:test-project-token -- --network jcTarget
 *
 * BSC 主网（受控小规模 IDO 测试；必须显式口令）：
 *   JC_DEPLOY_RPC=<rpc> JC_DEPLOY_CHAIN_ID=56 JC_DEPLOY_PRIVATE_KEY=<key> \
 *   TEST_TOKEN_ALLOW_MAINNET=I_UNDERSTAND_REAL_CHAIN \
 *   npm run deploy:test-project-token -- --network jcTarget
 */
async function main() {
    const [deployer] = await ethers.getSigners();
    const network = await ethers.provider.getNetwork();
    const chainId = Number(network.chainId);

    // 只允许已批准的测试链，防止测试代币误发到任何生产 EVM 网络。
    // BSC 主网(56) 需要显式口令 TEST_TOKEN_ALLOW_MAINNET=I_UNDERSTAND_REAL_CHAIN
    // （受控小规模 IDO 真链测试，见 deploy 仓 docs/IDO-BSC-MAINNET-DEPLOYMENT-GUIDE.md）。
    const allowedTestChainIds = new Set([97, 1337, 31337]);
    const mainnetPassphraseGranted =
        chainId === 56 && String(process.env.TEST_TOKEN_ALLOW_MAINNET || "").trim() === "I_UNDERSTAND_REAL_CHAIN";
    if (!allowedTestChainIds.has(chainId) && !mainnetPassphraseGranted) {
        throw new Error(
            `TestProjectToken 仅允许部署到 BSC Testnet/本地链（主网需 TEST_TOKEN_ALLOW_MAINNET=I_UNDERSTAND_REAL_CHAIN），当前 chainId=${chainId}`
        );
    }
    if (mainnetPassphraseGranted) {
        console.log("⚠ 主网部署已授权（TEST_TOKEN_ALLOW_MAINNET）：chainId=56，将发送真实交易");
    }

    const name = process.env.TEST_PROJECT_TOKEN_NAME || "JoinCare Test Project Token";
    const symbol = process.env.TEST_PROJECT_TOKEN_SYMBOL || "JCPT";
    const initialSupplyText = process.env.TEST_PROJECT_TOKEN_INITIAL_SUPPLY || "1000000";
    const maxSupplyText = process.env.TEST_PROJECT_TOKEN_MAX_SUPPLY || "10000000";
    const owner = process.env.TEST_PROJECT_TOKEN_OWNER || deployer.address;

    if (!ethers.isAddress(owner) || owner === ethers.ZeroAddress) {
        throw new Error("TEST_PROJECT_TOKEN_OWNER 必须是有效的非零地址");
    }

    const initialSupply = ethers.parseUnits(initialSupplyText, 18);
    const maxSupply = ethers.parseUnits(maxSupplyText, 18);

    if (initialSupply > maxSupply) {
        throw new Error("TEST_PROJECT_TOKEN_INITIAL_SUPPLY 不能超过 TEST_PROJECT_TOKEN_MAX_SUPPLY");
    }

    console.log("========================================");
    console.log("部署 JoinCare 测试项目代币");
    console.log("========================================");
    console.log("network:", network.name);
    console.log("chainId:", chainId);
    console.log("deployer:", deployer.address);
    console.log("project owner:", owner);
    console.log("name:", name);
    console.log("symbol:", symbol);
    console.log("initial supply:", initialSupplyText);
    console.log("max supply:", maxSupplyText);

    const TestProjectToken = await ethers.getContractFactory("TestProjectToken");
    const token = await TestProjectToken.deploy(
        name,
        symbol,
        initialSupply,
        maxSupply,
        owner
    );
    await token.waitForDeployment();

    const address = await token.getAddress();
    const deployment = {
        network: network.name,
        chainId: network.chainId.toString(),
        address,
        deployer: deployer.address,
        owner: await token.owner(),
        name: await token.name(),
        symbol: await token.symbol(),
        decimals: Number(await token.decimals()),
        totalSupply: ethers.formatUnits(await token.totalSupply(), 18),
        maxSupply: ethers.formatUnits(await token.maxSupply(), 18),
        deployedAt: new Date().toISOString(),
    };

    console.log("\n部署成功：");
    console.log(JSON.stringify(deployment, null, 2));
    console.log("\n下一步：将 address 作为 IDO createProject 的 projectToken 地址。");
}

main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error("部署失败:", error.message);
        process.exit(1);
    });
