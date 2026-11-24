// Test-env only. 覆盖原 hardhat.config.js，让 node 监听 0.0.0.0 才能被 backend 容器访问。
require("@nomicfoundation/hardhat-toolbox");
require("@openzeppelin/hardhat-upgrades");

const FAIR_LAUNCH_PRODUCTION_SETTINGS = {
  optimizer: { enabled: true, runs: 200 },
  viaIR: true,
};
const fairLaunchOverride = () => ({ version: "0.8.22", settings: FAIR_LAUNCH_PRODUCTION_SETTINGS });

module.exports = {
  solidity: {
    compilers: [
      { version: "0.8.30", settings: { optimizer: { enabled: true, runs: 200 }, viaIR: true } },
      // Exact-pragma 0.8.22 sources default to the same frozen baseline.
      { version: "0.8.22", settings: FAIR_LAUNCH_PRODUCTION_SETTINGS },
    ],
    // Explicit production overrides prevent future compiler-list changes from moving a facet.
    overrides: Object.fromEntries([
      "contracts/JoinCareFairLaunchV1.sol",
      "contracts/fairlaunch/JoinCareFairLaunchV1Shared.sol",
      "contracts/fairlaunch/JoinCareFairLaunchV1CoreFacet.sol",
      "contracts/fairlaunch/JoinCareFairLaunchV1ReleaseFacet.sol",
      "contracts/fairlaunch/JoinCareFairLaunchV1FundsFacet.sol",
      "contracts/fairlaunch/JoinCareFairLaunchV1AdminFacet.sol",
      "contracts/fairlaunch/JoinCareFairLaunchV1ViewsFacet.sol",
      "contracts/libraries/JoinCareFairLaunchV1Logic.sol",
      "contracts/libraries/JoinCareFairLaunchV1Runtime.sol",
      "contracts/governance/JoinCareFairLaunchTimelock.sol",
    ].map(source => [source, fairLaunchOverride()])),
  },
  networks: {
    hardhat: {
      chainId: 31337,
      // 允许通过 docker 内网访问
      hostname: "0.0.0.0",
      // 保证 relayer/deployer 账户余额充足
      accounts: {
        count: 20,
        accountsBalance: "10000000000000000000000", // 10000 ETH
      },
    },
    // 部署脚本 hardhat run --network localhost 时用
    localhost: {
      url: "http://127.0.0.1:8545",
      chainId: 31337,
    },
    // 生产链部署目标 · 由 ./deploy.sh contracts deploy 动态注入
    // 使用: JC_DEPLOY_RPC=... JC_DEPLOY_CHAIN_ID=... JC_DEPLOY_PRIVATE_KEY=... hardhat run --network jcTarget
    jcTarget: {
      url: process.env.JC_DEPLOY_RPC || "http://127.0.0.1:8545",
      chainId: process.env.JC_DEPLOY_CHAIN_ID
        ? parseInt(process.env.JC_DEPLOY_CHAIN_ID, 10)
        : 31337,
      accounts: process.env.JC_DEPLOY_PRIVATE_KEY
        ? [process.env.JC_DEPLOY_PRIVATE_KEY.startsWith("0x")
            ? process.env.JC_DEPLOY_PRIVATE_KEY
            : "0x" + process.env.JC_DEPLOY_PRIVATE_KEY]
        : [],
    },
  },
  mocha: { timeout: 60000 },
  // 只编译签到相关合约：设 JC_CHECKIN_ONLY=1 时走 ./contracts-checkin
  // build.sh 编译时会 export JC_CHECKIN_ONLY=1，避免拉入 JCC/Launchpad
  paths: {
    sources: process.env.JC_CHECKIN_ONLY
      ? "./contracts-checkin"
      : process.env.JC_FAIR_LAUNCH_MODEL_ONLY
        ? "./contracts-model"
        : "./contracts",
  },
};
