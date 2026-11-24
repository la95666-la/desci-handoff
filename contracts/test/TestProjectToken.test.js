const { expect } = require("chai");
const { ethers, upgrades } = require("hardhat");

describe("TestProjectToken", function () {
    let token;
    let owner;
    let alice;
    let safe;

    const INITIAL_SUPPLY = ethers.parseEther("1000000");
    const MAX_SUPPLY = ethers.parseEther("10000000");

    beforeEach(async function () {
        [owner, alice, safe] = await ethers.getSigners();
        const TestProjectToken = await ethers.getContractFactory("TestProjectToken");
        token = await TestProjectToken.deploy(
            "JoinCare Test Project Token",
            "JCPT",
            INITIAL_SUPPLY,
            MAX_SUPPLY,
            owner.address
        );
        await token.waitForDeployment();
    });

    it("使用 18 decimals，并把初始供应量铸给项目方", async function () {
        expect(await token.name()).to.equal("JoinCare Test Project Token");
        expect(await token.symbol()).to.equal("JCPT");
        expect(await token.decimals()).to.equal(18);
        expect(await token.owner()).to.equal(owner.address);
        expect(await token.totalSupply()).to.equal(INITIAL_SUPPLY);
        expect(await token.balanceOf(owner.address)).to.equal(INITIAL_SUPPLY);
        expect(await token.maxSupply()).to.equal(MAX_SUPPLY);
    });

    it("拒绝为零的供应量硬顶", async function () {
        const TestProjectToken = await ethers.getContractFactory("TestProjectToken");
        await expect(
            TestProjectToken.deploy("Bad Token", "BAD", 0, 0, owner.address)
        ).to.be.revertedWithCustomError(TestProjectToken, "InvalidMaxSupply");
    });

    it("拒绝超过硬顶的初始供应量", async function () {
        const TestProjectToken = await ethers.getContractFactory("TestProjectToken");
        await expect(
            TestProjectToken.deploy("Bad Token", "BAD", MAX_SUPPLY + 1n, MAX_SUPPLY, owner.address)
        )
            .to.be.revertedWithCustomError(TestProjectToken, "InitialSupplyExceedsMaxSupply")
            .withArgs(MAX_SUPPLY + 1n, MAX_SUPPLY);
    });

    it("owner 可在硬顶内增发", async function () {
        const amount = ethers.parseEther("500000");
        await token.mint(alice.address, amount);
        expect(await token.balanceOf(alice.address)).to.equal(amount);
        expect(await token.totalSupply()).to.equal(INITIAL_SUPPLY + amount);
    });

    it("非 owner 不能增发", async function () {
        await expect(token.connect(alice).mint(alice.address, 1n))
            .to.be.revertedWithCustomError(token, "OwnableUnauthorizedAccount")
            .withArgs(alice.address);
    });

    it("增发不能突破硬顶", async function () {
        const available = MAX_SUPPLY - INITIAL_SUPPLY;
        await expect(token.mint(alice.address, available + 1n))
            .to.be.revertedWithCustomError(token, "MintExceedsMaxSupply")
            .withArgs(available + 1n, available);
    });

    it("持有人可以销毁自己的代币", async function () {
        const amount = ethers.parseEther("100");
        await token.burn(amount);
        expect(await token.totalSupply()).to.equal(INITIAL_SUPPLY - amount);
        expect(await token.balanceOf(owner.address)).to.equal(INITIAL_SUPPLY - amount);
    });

    it("经 allowance 授权后可以 burnFrom", async function () {
        const amount = ethers.parseEther("100");
        await token.approve(alice.address, amount);
        await token.connect(alice).burnFrom(owner.address, amount);
        expect(await token.balanceOf(owner.address)).to.equal(INITIAL_SUPPLY - amount);
    });

    it("owner 必须通过两步流程迁移", async function () {
        await token.transferOwnership(safe.address);
        expect(await token.owner()).to.equal(owner.address);
        expect(await token.pendingOwner()).to.equal(safe.address);

        await token.connect(safe).acceptOwnership();
        expect(await token.owner()).to.equal(safe.address);

        await expect(token.mint(alice.address, 1n))
            .to.be.revertedWithCustomError(token, "OwnableUnauthorizedAccount")
            .withArgs(owner.address);
        await token.connect(safe).mint(alice.address, 1n);
        expect(await token.balanceOf(alice.address)).to.equal(1n);
    });
    it("可用于 IDO 项目创建和项目代币充值", async function () {
        const [, projectOwner, feeReceiver] = await ethers.getSigners();

        const MockJCC = await ethers.getContractFactory("MockJCC");
        const jc = await MockJCC.deploy(ethers.parseEther("1000000"));

        const JoinCare = await ethers.getContractFactory("JoinCare_v1");
        const platform = await upgrades.deployProxy(
            JoinCare,
            [await jc.getAddress(), feeReceiver.address, 200],
            { kind: "uups", initializer: "initialize" }
        );

        const projectTokenAmount = ethers.parseEther("100000");
        await token.transfer(projectOwner.address, projectTokenAmount);

        const latestBlock = await ethers.provider.getBlock("latest");
        const startTime = latestBlock.timestamp + 60;
        const endTime = startTime + 3600;
        const vestingTimes = [endTime + 3600, endTime + 7200];
        const vestingPercents = [5000, 5000];

        await platform.createProject(
            1,
            await token.getAddress(),
            projectOwner.address,
            ethers.parseEther("1"),
            ethers.parseEther("100000"),
            ethers.parseEther("50000"),
            ethers.parseEther("1000"),
            startTime,
            endTime,
            vestingTimes,
            vestingPercents,
            3600
        );

        await token.connect(projectOwner).approve(await platform.getAddress(), projectTokenAmount);
        await platform.connect(projectOwner).depositProjectTokens(1, projectTokenAmount);

        const project = await platform.projects(1);
        expect(project.projectToken).to.equal(await token.getAddress());
        expect(project.totalTokensDeposited).to.equal(projectTokenAmount);
        expect(await token.balanceOf(await platform.getAddress())).to.equal(projectTokenAmount);
    });
});
