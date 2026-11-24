const { expect } = require("chai");
const { ethers } = require("hardhat");

describe("Mock payment tokens", function () {
    let owner;
    let user;
    let safe;

    beforeEach(async function () {
        [owner, user, safe] = await ethers.getSigners();
    });

    async function deploy(name, decimals, initial = "1000000", cap = "10000000") {
        const Token = await ethers.getContractFactory(name);
        const token = await Token.deploy(
            ethers.parseUnits(initial, decimals),
            ethers.parseUnits(cap, decimals),
            owner.address
        );
        await token.waitForDeployment();
        return token;
    }

    for (const tokenCase of [
        { contract: "MockUSDT", name: "JoinCare Mock USDT", symbol: "mUSDT", decimals: 6 },
        { contract: "MockUSDC", name: "JoinCare Mock USDC", symbol: "mUSDC", decimals: 18 },
    ]) {
        describe(`${tokenCase.contract} (${tokenCase.decimals} decimals)`, function () {
            it("uses the configured identity, precision, owner, and supply cap", async function () {
                const token = await deploy(tokenCase.contract, tokenCase.decimals);
                const initial = ethers.parseUnits("1000000", tokenCase.decimals);
                const cap = ethers.parseUnits("10000000", tokenCase.decimals);

                expect(await token.name()).to.equal(tokenCase.name);
                expect(await token.symbol()).to.equal(tokenCase.symbol);
                expect(await token.decimals()).to.equal(tokenCase.decimals);
                expect(await token.owner()).to.equal(owner.address);
                expect(await token.balanceOf(owner.address)).to.equal(initial);
                expect(await token.totalSupply()).to.equal(initial);
                expect(await token.maxSupply()).to.equal(cap);
            });

            it("allows only the owner to mint within the immutable cap", async function () {
                const token = await deploy(tokenCase.contract, tokenCase.decimals);
                const amount = ethers.parseUnits("500", tokenCase.decimals);

                await token.mint(user.address, amount);
                expect(await token.balanceOf(user.address)).to.equal(amount);

                await expect(token.connect(user).mint(user.address, 1n))
                    .to.be.revertedWithCustomError(token, "OwnableUnauthorizedAccount")
                    .withArgs(user.address);

                const available = await token.maxSupply() - await token.totalSupply();
                await expect(token.mint(user.address, available + 1n))
                    .to.be.revertedWithCustomError(token, "MintExceedsMaxSupply")
                    .withArgs(available + 1n, available);
            });

            it("uses two-step ownership transfer for a future Safe", async function () {
                const token = await deploy(tokenCase.contract, tokenCase.decimals);
                await token.transferOwnership(safe.address);

                expect(await token.owner()).to.equal(owner.address);
                expect(await token.pendingOwner()).to.equal(safe.address);

                await token.connect(safe).acceptOwnership();
                expect(await token.owner()).to.equal(safe.address);
                await token.connect(safe).mint(user.address, 1n);
            });

            it("rejects a zero supply cap and initial supply above the cap", async function () {
                const Token = await ethers.getContractFactory(tokenCase.contract);
                await expect(Token.deploy(0, 0, owner.address))
                    .to.be.revertedWithCustomError(Token, "InvalidMaxSupply");

                const cap = ethers.parseUnits("1", tokenCase.decimals);
                await expect(Token.deploy(cap + 1n, cap, owner.address))
                    .to.be.revertedWithCustomError(Token, "InitialSupplyExceedsMaxSupply")
                    .withArgs(cap + 1n, cap);
            });
        });
    }

    it("represents one display unit with different raw amounts", async function () {
        const usdt = await deploy("MockUSDT", 6);
        const usdc = await deploy("MockUSDC", 18);

        await usdt.mint(user.address, 1_000_000n);
        await usdc.mint(user.address, 1_000_000_000_000_000_000n);

        expect(ethers.formatUnits(await usdt.balanceOf(user.address), 6)).to.equal("1.0");
        expect(ethers.formatUnits(await usdc.balanceOf(user.address), 18)).to.equal("1.0");
    });
});
