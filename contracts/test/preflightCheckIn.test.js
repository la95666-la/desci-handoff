const { expect } = require("chai");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { ethers } = require("hardhat");
const { upgrades } = require("hardhat");
const { IMPLEMENTATION_SLOT, expectedFromEnvironment, implementationFromSlot, validateObserved, preflight } = require("../scripts/preflightCheckIn");
const { writeManifestExclusive } = require("../scripts/upgradeCheckIn");

describe("CheckIn deployment preflight", function () {
    let expected;
    let provider;

    beforeEach(async function () {
        const CheckIn = await ethers.getContractFactory("JoinCareCheckIn");
        const proxy = await upgrades.deployProxy(CheckIn, [], { kind: "uups", initializer: "initialize" });
        await proxy.waitForDeployment();
        const proxyAddress = await proxy.getAddress();
        const implementationAddress = await upgrades.erc1967.getImplementationAddress(proxyAddress);
        const implementationCode = await ethers.provider.getCode(implementationAddress);
        const network = await ethers.provider.getNetwork();
        expected = {
            chainId: network.chainId.toString(),
            proxyAddress,
            implementationAddress,
            implementationCodeHash: ethers.keccak256(implementationCode),
            owner: await proxy.owner(),
        };
        provider = ethers.provider;
    });

    it("verifies chain, ERC1967 implementation, runtime hash, and owner", async function () {
        expect(await preflight(provider, expected)).to.deep.equal(expected);
        const slot = await provider.getStorage(expected.proxyAddress, IMPLEMENTATION_SLOT);
        expect(implementationFromSlot(slot)).to.equal(expected.implementationAddress);
    });

    for (const [field, replacement] of [
        ["chainId", "97"],
        ["proxyAddress", "0x0000000000000000000000000000000000000001"],
        ["implementationAddress", "0x0000000000000000000000000000000000000001"],
        ["implementationCodeHash", "0x" + "11".repeat(32)],
        ["owner", "0x0000000000000000000000000000000000000001"],
    ]) {
        it("rejects wrong " + field, async function () {
            const wrong = { ...expected, [field]: replacement };
            if (field === "proxyAddress") {
                await expect(preflight(provider, wrong)).to.be.rejectedWith("proxy has no runtime code");
            } else {
                await expect(preflight(provider, wrong)).to.be.rejectedWith(field + " mismatch");
            }
        });
    }

    it("writes upgrade manifests exclusively", function () {
        const directory = fs.mkdtempSync(path.join(os.tmpdir(), "checkin-upgrade-"));
        const manifest = path.join(directory, "manifest.json");
        const value = { schema: "joincare.checkin.upgrade.v1", ...expected };
        writeManifestExclusive(manifest, value);
        expect(JSON.parse(fs.readFileSync(manifest, "utf8"))).to.deep.equal(value);
        expect(() => writeManifestExclusive(manifest, value)).to.throw();
        fs.rmSync(directory, { recursive: true });
    });

    it("rejects an unknown manifest schema", function () {
        const directory = fs.mkdtempSync(path.join(os.tmpdir(), "checkin-manifest-"));
        const manifest = path.join(directory, "manifest.json");
        fs.writeFileSync(manifest, JSON.stringify({ schema: "unknown" }));
        expect(() => expectedFromEnvironment({ CHECKIN_PREFLIGHT_MANIFEST: manifest })).to.throw("unsupported CheckIn manifest schema");
        fs.rmSync(directory, { recursive: true });
    });

    it("rejects manifest without transaction and block evidence", function () {
        const directory = fs.mkdtempSync(path.join(os.tmpdir(), "checkin-evidence-"));
        const manifest = path.join(directory, "manifest.json");
        fs.writeFileSync(manifest, JSON.stringify({ schema: "joincare.checkin.deployment.v1", ...expected }));
        expect(() => expectedFromEnvironment({ CHECKIN_PREFLIGHT_MANIFEST: manifest })).to.throw("manifest transaction hash is invalid");
        fs.rmSync(directory, { recursive: true });
    });

    it("rejects malformed ERC1967 storage values", function () {
        expect(() => implementationFromSlot("0x1234")).to.throw("invalid ERC1967 slot value");
    });

    it("compares addresses case-insensitively but every field exactly", function () {
        const observed = { ...expected, owner: expected.owner.toLowerCase() };
        expect(validateObserved(expected, observed)).to.equal(observed);
    });
});
