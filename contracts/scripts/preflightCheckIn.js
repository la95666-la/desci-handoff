const fs = require("fs");
const { ethers } = require("ethers");

const IMPLEMENTATION_SLOT = "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc";
const OWNER_ABI = ["function owner() view returns (address)"];

function required(value, name) {
    if (value === undefined || value === null || String(value) === "") {
        throw new Error(name + " is required");
    }
    return String(value);
}

function normalizeAddress(value, name) {
    const address = required(value, name);
    if (!ethers.isAddress(address) || address === ethers.ZeroAddress) {
        throw new Error(name + " is not a non-zero address");
    }
    return ethers.getAddress(address);
}

function expectedFromEnvironment(env = process.env) {
    let manifest = {};
    if (env.CHECKIN_PREFLIGHT_MANIFEST) {
        manifest = JSON.parse(fs.readFileSync(env.CHECKIN_PREFLIGHT_MANIFEST, "utf8"));
        const allowed = new Set(["joincare.checkin.deployment.v1", "joincare.checkin.upgrade.v1"]);
        if (!allowed.has(manifest.schema)) throw new Error("unsupported CheckIn manifest schema");
        const transactionHash = manifest.deploymentTransactionHash ?? manifest.upgradeTransactionHash;
        const blockNumber = manifest.deploymentBlockNumber ?? manifest.upgradeBlockNumber;
        const blockHash = manifest.deploymentBlockHash ?? manifest.upgradeBlockHash;
        if (!/^0x[0-9a-fA-F]{64}$/.test(String(transactionHash))) throw new Error("manifest transaction hash is invalid");
        if (!Number.isSafeInteger(Number(blockNumber)) || Number(blockNumber) < 0) throw new Error("manifest block number is invalid");
        if (!/^0x[0-9a-fA-F]{64}$/.test(String(blockHash))) throw new Error("manifest block hash is invalid");
    }
    return {
        chainId: required(manifest.chainId ?? env.CHECKIN_EXPECTED_CHAIN_ID, "chainId"),
        proxyAddress: normalizeAddress(manifest.proxyAddress ?? env.CHECKIN_PROXY_ADDRESS, "proxyAddress"),
        implementationAddress: normalizeAddress(manifest.implementationAddress ?? env.CHECKIN_EXPECTED_IMPLEMENTATION, "implementationAddress"),
        implementationCodeHash: required(manifest.implementationCodeHash ?? env.CHECKIN_EXPECTED_IMPLEMENTATION_CODE_HASH, "implementationCodeHash").toLowerCase(),
        owner: normalizeAddress(manifest.owner ?? env.CHECKIN_EXPECTED_OWNER, "owner"),
    };
}

function implementationFromSlot(value) {
    if (!/^0x[0-9a-fA-F]{64}$/.test(value)) throw new Error("invalid ERC1967 slot value");
    return normalizeAddress("0x" + value.slice(-40), "ERC1967 implementation");
}

function validateObserved(expected, observed) {
    const comparisons = [
        ["chainId", String(observed.chainId), String(expected.chainId)],
        ["proxyAddress", observed.proxyAddress.toLowerCase(), expected.proxyAddress.toLowerCase()],
        ["implementationAddress", observed.implementationAddress.toLowerCase(), expected.implementationAddress.toLowerCase()],
        ["implementationCodeHash", observed.implementationCodeHash.toLowerCase(), expected.implementationCodeHash.toLowerCase()],
        ["owner", observed.owner.toLowerCase(), expected.owner.toLowerCase()],
    ];
    for (const [name, actual, wanted] of comparisons) {
        if (actual !== wanted) throw new Error(name + " mismatch: expected " + wanted + ", got " + actual);
    }
    return observed;
}

async function inspect(provider, expected) {
    const network = await provider.getNetwork();
    const proxyCode = await provider.getCode(expected.proxyAddress);
    if (proxyCode === "0x") throw new Error("proxy has no runtime code");
    const slot = await provider.getStorage(expected.proxyAddress, IMPLEMENTATION_SLOT);
    const implementationAddress = implementationFromSlot(slot);
    const implementationCode = await provider.getCode(implementationAddress);
    if (implementationCode === "0x") throw new Error("implementation has no runtime code");
    const contract = new ethers.Contract(expected.proxyAddress, OWNER_ABI, provider);
    return {
        chainId: network.chainId.toString(),
        proxyAddress: expected.proxyAddress,
        implementationAddress,
        implementationCodeHash: ethers.keccak256(implementationCode),
        owner: await contract.owner(),
    };
}

async function preflight(provider, expected) {
    return validateObserved(expected, await inspect(provider, expected));
}

async function main() {
    const rpc = required(process.env.JC_DEPLOY_RPC, "JC_DEPLOY_RPC");
    const expected = expectedFromEnvironment();
    const observed = await preflight(new ethers.JsonRpcProvider(rpc), expected);
    console.log(JSON.stringify({ status: "verified", ...observed }));
}

if (require.main === module) {
    main().catch((error) => {
        console.error("CheckIn preflight failed:", error.message);
        process.exit(1);
    });
}

module.exports = { IMPLEMENTATION_SLOT, expectedFromEnvironment, implementationFromSlot, validateObserved, inspect, preflight };
