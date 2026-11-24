// 部署 JoinCareCheckIn 到本地 hardhat node（UUPS proxy）。
// 目的：测试环境端到端联调；deployer 用 hardhat #0。
//   deployer nonce 0 → implementation @ 0x5FbDB2315678afecb367f032d93F642f64180aa3
//   deployer nonce 1 → proxy         @ 0xe7f1725E7734CE288F8367e1Bb143E90bb3F0512
// 后端 config 里 contractAddress 必须写 proxy 地址。

const { ethers, upgrades } = require("hardhat");

async function main() {
  const [deployer] = await ethers.getSigners();

  console.log("[deploy] deployer =", deployer.address);
  console.log("[deploy] balance  =", ethers.formatEther(await ethers.provider.getBalance(deployer.address)), "ETH");
  console.log("[deploy] nonce    =", await ethers.provider.getTransactionCount(deployer.address));

  const CheckIn = await ethers.getContractFactory("JoinCareCheckIn");
  const proxy = await upgrades.deployProxy(CheckIn, [], {
    kind: "uups",
    initializer: "initialize",
  });
  await proxy.waitForDeployment();

  const proxyAddr = await proxy.getAddress();
  const implAddr  = await upgrades.erc1967.getImplementationAddress(proxyAddr);

  console.log("[deploy] JoinCareCheckIn proxy          =", proxyAddr);
  console.log("[deploy] JoinCareCheckIn implementation =", implAddr);
  console.log("[deploy] === write this to backend config.yaml chain.task.contractAddress ===");
  console.log("[deploy]", proxyAddr);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
