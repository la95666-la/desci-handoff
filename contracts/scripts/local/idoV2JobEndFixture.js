// Disposable localhost-only fixture for the Go Job outbox + real Hardhat RPC test.
// Never point this at an external chain or a funded signer.
const fs = require("fs");
const { ethers, upgrades, network } = require("hardhat");

async function main() {
  if (network.name !== "jcTarget" || process.env.JC_DEPLOY_RPC !== "http://127.0.0.1:18546" || (await ethers.provider.getNetwork()).chainId !== 31337n) {
    throw new Error("IDO Job end fixture requires localhost Hardhat chain 31337");
  }
  const output = process.env.JC_IDO_END_FIXTURE;
  if (!output) throw new Error("JC_IDO_END_FIXTURE path required");
  const projectId = process.env.JC_IDO_FULL_SCHEMA_HARDHAT_FIXTURE ? 19 : 9;
  const [owner, projectOwner, feeReceiver] = [0, 1, 4].map(index => ethers.HDNodeWallet.fromPhrase(
    "test test test test test test test test test test test junk", undefined,
    `m/44'/60'/0'/0/${index}`
  ).connect(ethers.provider));
  const jobKey = ethers.HDNodeWallet.fromPhrase(
    "test test test test test test test test test test test junk", undefined,
    "m/44'/60'/0'/0/8"
  );
  const usdt = await (await ethers.getContractFactory("MockUSDT", owner)).deploy(
    ethers.parseUnits("1000000", 6), ethers.parseUnits("10000000", 6), owner.address
  );
  const usdc = await (await ethers.getContractFactory("MockUSDC", owner)).deploy(
    ethers.parseUnits("1000000", 18), ethers.parseUnits("10000000", 18), owner.address
  );
  const token = await (await ethers.getContractFactory("TestProjectToken", owner)).deploy(
    "Disposable Job Token", "DJT", ethers.parseEther("1000000"), ethers.parseEther("10000000"), projectOwner.address
  );
  const platform = await upgrades.deployProxy(
    await ethers.getContractFactory("JoinCareIDOv2", owner),
    [owner.address, owner.address, feeReceiver.address, 200],
    { kind: "uups", initializer: "initialize" }
  );
  await platform.waitForDeployment();
  await (await platform.configurePaymentToken(await usdt.getAddress(), true)).wait();
  await (await platform.configurePaymentToken(await usdc.getAddress(), true)).wait();
  await (await platform.configureProjectToken(await token.getAddress(), true)).wait();
  const latest = (await ethers.provider.getBlock("latest")).timestamp;
  const startTime = latest + 60;
  const endTime = startTime + 60;
  await (await platform.createProject(
    projectId, await token.getAddress(), await usdt.getAddress(), projectOwner.address,
    feeReceiver.address, 200, ethers.parseUnits("2", 6), ethers.parseUnits("1000", 6),
    ethers.parseUnits("500", 6), ethers.parseUnits("600", 6), startTime, endTime,
    [endTime + 100, endTime + 200], [2500, 7500]
  )).wait();
  await (await token.connect(projectOwner).approve(await platform.getAddress(), ethers.parseEther("1000"))).wait();
  await (await platform.connect(projectOwner).depositProjectTokens(projectId, ethers.parseEther("1000"))).wait();
  await (await platform.activateProject(projectId)).wait();
  await ethers.provider.send("evm_setNextBlockTimestamp", [endTime + 1]);
  await ethers.provider.send("evm_mine", []);
  const implementation = await upgrades.erc1967.getImplementationAddress(await platform.getAddress());
  const implementationCode = await ethers.provider.getCode(implementation);
  const startBlock = (await ethers.provider.getBlockNumber()) + 1;
  fs.writeFileSync(output, JSON.stringify({
    chainId: 31337, contract: await platform.getAddress(), projectId,
    endTime, paymentToken: await usdt.getAddress(), usdc: await usdc.getAddress(),
    owner: owner.address, implementation, implementationCodeHash: ethers.keccak256(implementationCode),
    startBlock, jobKey: jobKey.privateKey
  }), { flag: "wx", mode: 0o600 });
  console.log("disposable IDO V2 Hardhat fixture deployed; manifest written privately");
}

main().catch(err => { console.error(err); process.exitCode = 1; });
