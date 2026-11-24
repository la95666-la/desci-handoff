const { ethers, upgrades } = require("hardhat");
const fs = require("fs");
const path = require("path");
require("dotenv").config();

/**
 * Deploy the JoinCare task center check-in contract with a UUPS proxy.
 * initialize() takes no arguments — deployer becomes owner.
 */
async function main() {
    console.log("========================================");
    console.log("Deploy JoinCareCheckIn");
    console.log("========================================\n");

    const [deployer] = await ethers.getSigners();
    const balance = await ethers.provider.getBalance(deployer.address);
    const network = await ethers.provider.getNetwork();

    console.log("Network:", network.name);
    console.log("Chain ID:", network.chainId.toString());
    console.log("Deployer:", deployer.address);
    console.log("Balance:", ethers.formatEther(balance), "BNB\n");

    const JoinCareCheckIn = await ethers.getContractFactory("JoinCareCheckIn");
    const checkIn = await upgrades.deployProxy(
        JoinCareCheckIn,
        [],
        {
            kind: "uups",
            initializer: "initialize",
        }
    );

    await checkIn.waitForDeployment();
    const checkInAddress = await checkIn.getAddress();
    const implementationAddress = await upgrades.erc1967.getImplementationAddress(checkInAddress);
    const deploymentTx = checkIn.deploymentTransaction();
    const receipt = await deploymentTx.wait();
    const implementationCode = await ethers.provider.getCode(implementationAddress);
    if (implementationCode === "0x") throw new Error("implementation has no runtime code");
    const owner = await checkIn.owner();

    console.log("JoinCareCheckIn proxy deployed:", checkInAddress);
    console.log("Implementation:", implementationAddress);
    console.log("Owner:", owner);

    const deployment = {
        schema: "joincare.checkin.deployment.v1",
        network: network.name,
        chainId: network.chainId.toString(),
        deployer: deployer.address,
        owner,
        proxyAddress: checkInAddress,
        implementationAddress,
        implementationCodeHash: ethers.keccak256(implementationCode),
        deploymentTransactionHash: deploymentTx.hash,
        deploymentBlockNumber: receipt.blockNumber,
        deploymentBlockHash: receipt.blockHash,
        deployedAt: new Date().toISOString(),
    };
    const manifestPath = process.env.CHECKIN_DEPLOYMENT_MANIFEST;
    if (!manifestPath) throw new Error("CHECKIN_DEPLOYMENT_MANIFEST is required");
    fs.mkdirSync(path.dirname(manifestPath), { recursive: true });
    fs.writeFileSync(manifestPath, JSON.stringify(deployment, null, 2) + "\n", { flag: "wx", mode: 0o600 });
    console.log("\nDeployment info:");
    console.log(JSON.stringify(deployment, null, 2));
}

main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error("Deploy failed:", error);
        process.exit(1);
    });
