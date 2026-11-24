const { ethers, upgrades } = require("hardhat");
const fs = require("fs");
const path = require("path");
require("dotenv").config();

/**
 * Upgrade the standalone JoinCare task center check-in contract.
 *
 * Environment:
 * - CHECKIN_PROXY_ADDRESS: required UUPS proxy address.
 */
function writeManifestExclusive(manifestPath, manifest) {
    if (!manifestPath) throw new Error("CHECKIN_UPGRADE_MANIFEST is required");
    fs.mkdirSync(path.dirname(manifestPath), { recursive: true });
    fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + "\n", { flag: "wx", mode: 0o600 });
}

async function main() {
    console.log("========================================");
    console.log("Upgrade JoinCareCheckIn");
    console.log("========================================\n");

    const [upgrader] = await ethers.getSigners();
    const balance = await ethers.provider.getBalance(upgrader.address);
    const network = await ethers.provider.getNetwork();
    const proxyAddress = process.env.CHECKIN_PROXY_ADDRESS;

    if (!proxyAddress || !ethers.isAddress(proxyAddress)) {
        throw new Error("Please set a valid CHECKIN_PROXY_ADDRESS in .env");
    }

    console.log("Network:", network.name);
    console.log("Chain ID:", network.chainId.toString());
    console.log("Upgrader:", upgrader.address);
    console.log("Balance:", ethers.formatEther(balance), "BNB");
    console.log("Proxy:", proxyAddress);

    const currentImplementation = await upgrades.erc1967.getImplementationAddress(proxyAddress);
    console.log("Current implementation:", currentImplementation);

    const proxy = await ethers.getContractAt("JoinCareCheckIn", proxyAddress);
    const owner = await proxy.owner();
    console.log("Owner:", owner);

    if (owner.toLowerCase() !== upgrader.address.toLowerCase()) {
        throw new Error("Only the proxy owner can upgrade JoinCareCheckIn");
    }

    const JoinCareCheckIn = await ethers.getContractFactory("JoinCareCheckIn");
    const upgraded = await upgrades.upgradeProxy(proxyAddress, JoinCareCheckIn, {
        kind: "uups",
    });

    await upgraded.waitForDeployment();

    const newImplementation = await upgrades.erc1967.getImplementationAddress(proxyAddress);
    console.log("\nUpgrade successful");
    console.log("Proxy unchanged:", proxyAddress);
    console.log("New implementation:", newImplementation);

    const transaction = upgraded.deploymentTransaction();
    if (!transaction) throw new Error("upgrade transaction evidence is unavailable");
    const receipt = await transaction.wait();
    if (!receipt?.blockHash) throw new Error("upgrade receipt evidence is incomplete");
    const implementationCode = await ethers.provider.getCode(newImplementation);
    if (implementationCode === "0x") throw new Error("new implementation has no runtime code");
    const manifest = {
        schema: "joincare.checkin.upgrade.v1",
        network: network.name,
        chainId: network.chainId.toString(),
        upgrader: upgrader.address,
        owner,
        proxyAddress,
        oldImplementationAddress: currentImplementation,
        implementationAddress: newImplementation,
        implementationCodeHash: ethers.keccak256(implementationCode),
        upgradeTransactionHash: transaction.hash,
        upgradeBlockNumber: receipt.blockNumber,
        upgradeBlockHash: receipt.blockHash,
        upgradedAt: new Date().toISOString(),
    };
    writeManifestExclusive(process.env.CHECKIN_UPGRADE_MANIFEST, manifest);
    console.log("\nUpgrade info:");
    console.log(JSON.stringify(manifest, null, 2));
}

if (require.main === module) {
    main()
        .then(() => process.exit(0))
        .catch((error) => {
            console.error("Upgrade failed:", error);
            process.exit(1);
        });
}

module.exports = { main, writeManifestExclusive };
