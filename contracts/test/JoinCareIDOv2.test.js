const { expect } = require("chai");
const { ethers, upgrades } = require("hardhat");
const { time } = require("@nomicfoundation/hardhat-network-helpers");

describe("JoinCareIDOv2 stablecoin settlement", function () {
    async function deployFixture() {
        const [owner, projectOwner, user1, user2, feeReceiver, nextFeeReceiver, safe] =
            await ethers.getSigners();

        const MockUSDT = await ethers.getContractFactory("MockUSDT");
        const usdt = await MockUSDT.deploy(
            ethers.parseUnits("1000000", 6),
            ethers.parseUnits("10000000", 6),
            owner.address
        );
        const MockUSDC = await ethers.getContractFactory("MockUSDC");
        const usdc = await MockUSDC.deploy(
            ethers.parseUnits("1000000", 18),
            ethers.parseUnits("10000000", 18),
            owner.address
        );

        const TestProjectToken = await ethers.getContractFactory("TestProjectToken");
        const projectToken = await TestProjectToken.deploy(
            "V2 Project Token",
            "V2PT",
            ethers.parseEther("1000000"),
            ethers.parseEther("10000000"),
            projectOwner.address
        );

        const JoinCareIDOv2 = await ethers.getContractFactory("JoinCareIDOv2");
        const platform = await upgrades.deployProxy(
            JoinCareIDOv2,
            [owner.address, owner.address, feeReceiver.address, 200],
            { kind: "uups", initializer: "initialize" }
        );

        await platform.configurePaymentToken(await usdt.getAddress(), true);
        await platform.configurePaymentToken(await usdc.getAddress(), true);
        await platform.configureProjectToken(await projectToken.getAddress(), true);
        await usdt.transfer(user1.address, ethers.parseUnits("10000", 6));
        await usdt.transfer(user2.address, ethers.parseUnits("10000", 6));
        await usdc.transfer(user1.address, ethers.parseUnits("10000", 18));
        await usdc.transfer(user2.address, ethers.parseUnits("10000", 18));

        return {
            platform,
            usdt,
            usdc,
            projectToken,
            owner,
            projectOwner,
            user1,
            user2,
            feeReceiver,
            nextFeeReceiver,
            safe,
        };
    }

    async function createProject(fixture, options = {}) {
        const {
            platform,
            usdt,
            projectToken,
            projectOwner,
            feeReceiver,
        } = fixture;
        const paymentToken = options.paymentToken || usdt;
        const paymentDecimals = Number(await paymentToken.decimals());
        const projectId = options.projectId || 1n;
        const latest = await time.latest();
        const startTime = options.startTime || latest + 100;
        const endTime = options.endTime || startTime + 1000;
        const tokenPrice = options.tokenPrice || ethers.parseUnits("2", paymentDecimals);
        const hardCap = options.hardCap || ethers.parseUnits("1000", paymentDecimals);
        const softCap = options.softCap || ethers.parseUnits("500", paymentDecimals);
        const maxPerUser = options.maxPerUser || ethers.parseUnits("600", paymentDecimals);
        const vestingTimes = [endTime + 100, endTime + 200];
        const vestingPercents = [2500, 7500];

        await platform.createProject(
            projectId,
            await projectToken.getAddress(),
            await paymentToken.getAddress(),
            projectOwner.address,
            feeReceiver.address,
            200,
            tokenPrice,
            hardCap,
            softCap,
            maxPerUser,
            startTime,
            endTime,
            vestingTimes,
            vestingPercents
        );

        const depositAmount = options.depositAmount || ethers.parseEther("1000");
        await projectToken.connect(projectOwner).approve(await platform.getAddress(), depositAmount);
        await platform.connect(projectOwner).depositProjectTokens(projectId, depositAmount);
        await platform.activateProject(projectId);

        return {
            projectId,
            paymentToken,
            paymentDecimals,
            startTime,
            endTime,
            tokenPrice,
            hardCap,
            softCap,
            maxPerUser,
            vestingTimes,
            depositAmount,
        };
    }

    async function approveAndPurchase(platform, paymentToken, user, projectId, amount) {
        await paymentToken.connect(user).approve(await platform.getAddress(), amount);
        return platform.connect(user).purchase(projectId, amount);
    }

    it("deploys only as an initialized fresh proxy and uses two-step ownership", async function () {
        const fixture = await deployFixture();
        const { platform, owner, safe, feeReceiver } = fixture;

        expect(await platform.owner()).to.equal(owner.address);
        expect(await platform.feeReceiver()).to.equal(feeReceiver.address);
        expect(await platform.successFeeRate()).to.equal(200);

        await platform.transferOwnership(safe.address);
        expect(await platform.owner()).to.equal(owner.address);
        expect(await platform.pendingOwner()).to.equal(safe.address);
        await platform.connect(safe).acceptOwnership();
        expect(await platform.owner()).to.equal(safe.address);

        const Implementation = await ethers.getContractFactory("JoinCareIDOv2");
        const implementation = await Implementation.deploy();
        await expect(implementation.initialize(owner.address, owner.address, feeReceiver.address, 200))
            .to.be.revertedWithCustomError(implementation, "InvalidInitialization");
    });

    it("governs payment tokens by contract address and rejects unknown or EOA addresses", async function () {
        const fixture = await deployFixture();
        const { platform, owner, projectOwner, projectToken } = fixture;
        const latest = await time.latest();

        await expect(platform.configurePaymentToken(ethers.ZeroAddress, true))
            .to.be.revertedWithCustomError(platform, "InvalidAddress");
        await expect(platform.configurePaymentToken(owner.address, true))
            .to.be.revertedWithCustomError(platform, "TokenHasNoCode")
            .withArgs(owner.address);
        await expect(platform.configurePaymentToken(owner.address, false))
            .to.be.revertedWithCustomError(platform, "PaymentTokenNotConfigured")
            .withArgs(owner.address);

        await expect(platform.createProject(
            99,
            await projectToken.getAddress(),
            owner.address,
            projectOwner.address,
            fixture.feeReceiver.address,
            200,
            1,
            100,
            50,
            10,
            latest + 100,
            latest + 200,
            [latest + 300],
            [10000]
        )).to.be.revertedWithCustomError(platform, "PaymentTokenNotAllowed").withArgs(owner.address);
    });

    it("rejects whitelist decimals drift after a token is registered", async function () {
        const fixture = await deployFixture();
        const { platform } = fixture;
        const MutableToken = await ethers.getContractFactory("MockMutableDecimalsToken");
        const token = await MutableToken.deploy(6);
        const tokenAddress = await token.getAddress();

        await platform.configurePaymentToken(tokenAddress, true);
        await platform.configureProjectToken(tokenAddress, true);
        await token.setDecimals(18);

        await expect(platform.configurePaymentToken(tokenAddress, true))
            .to.be.revertedWithCustomError(platform, "TokenDecimalsChanged")
            .withArgs(tokenAddress, 6, 18);
        await expect(platform.configureProjectToken(tokenAddress, true))
            .to.be.revertedWithCustomError(platform, "TokenDecimalsChanged")
            .withArgs(tokenAddress, 6, 18);
    });

    it("locks payment token, decimals, and fee snapshot when the project is created", async function () {
        const fixture = await deployFixture();
        const { platform, usdt, feeReceiver, nextFeeReceiver } = fixture;
        await createProject(fixture);

        await platform.setPlatformFees(500, nextFeeReceiver.address);
        await platform.configurePaymentToken(await usdt.getAddress(), false);

        const project = await platform.getProject(1);
        expect(project.paymentToken).to.equal(await usdt.getAddress());
        expect(project.paymentTokenDecimals).to.equal(6);
        expect(project.projectTokenDecimals).to.equal(18);
        expect(project.successFeeRate).to.equal(200);
        expect(project.feeReceiver).to.equal(feeReceiver.address);

        await time.increaseTo(project.startTime);
        await usdt.connect(fixture.user1).approve(await platform.getAddress(), ethers.parseUnits("100", 6));
        await expect(platform.connect(fixture.user1).purchase(1, ethers.parseUnits("100", 6)))
            .to.be.revertedWithCustomError(platform, "PaymentTokenNotAllowed")
            .withArgs(await usdt.getAddress());

        const latest = await time.latest();
        await expect(platform.createProject(
            2,
            project.projectToken,
            await usdt.getAddress(),
            fixture.projectOwner.address,
            feeReceiver.address,
            200,
            ethers.parseUnits("2", 6),
            ethers.parseUnits("1000", 6),
            ethers.parseUnits("500", 6),
            ethers.parseUnits("600", 6),
            latest + 100,
            latest + 200,
            [latest + 300],
            [10000]
        )).to.be.revertedWithCustomError(platform, "FeeSnapshotChanged")
            .withArgs(feeReceiver.address, nextFeeReceiver.address, 200, 500);

        await expect(platform.createProject(
            2,
            project.projectToken,
            await usdt.getAddress(),
            fixture.projectOwner.address,
            nextFeeReceiver.address,
            500,
            ethers.parseUnits("2", 6),
            ethers.parseUnits("1000", 6),
            ethers.parseUnits("500", 6),
            ethers.parseUnits("600", 6),
            latest + 100,
            latest + 200,
            [latest + 300],
            [10000]
        )).to.be.revertedWithCustomError(platform, "PaymentTokenNotAllowed");
    });

    it("stops activation and new purchases when project-token governance revokes the token", async function () {
        const fixture = await deployFixture();
        const { platform, projectToken, usdt, user1 } = fixture;
        const project = await createProject(fixture);
        await platform.configureProjectToken(await projectToken.getAddress(), false);
        await time.increaseTo(project.startTime);
        await usdt.connect(user1).approve(await platform.getAddress(), ethers.parseUnits("100", 6));
        await expect(platform.connect(user1).purchase(1, ethers.parseUnits("100", 6)))
            .to.be.revertedWithCustomError(platform, "ProjectTokenNotAllowed")
            .withArgs(await projectToken.getAddress());

        const latest = await time.latest();
        await platform.configureProjectToken(await projectToken.getAddress(), true);
        await platform.createProject(
            2,
            await projectToken.getAddress(),
            await usdt.getAddress(),
            fixture.projectOwner.address,
            fixture.feeReceiver.address,
            200,
            ethers.parseUnits("2", 6),
            ethers.parseUnits("1000", 6),
            ethers.parseUnits("500", 6),
            ethers.parseUnits("600", 6),
            latest + 100,
            latest + 200,
            [latest + 300],
            [10000]
        );
        const deposit = ethers.parseEther("10");
        await projectToken.connect(fixture.projectOwner).approve(await platform.getAddress(), deposit);
        await platform.connect(fixture.projectOwner).depositProjectTokens(2, deposit);
        await platform.configureProjectToken(await projectToken.getAddress(), false);
        await expect(platform.activateProject(2))
            .to.be.revertedWithCustomError(platform, "ProjectTokenNotAllowed")
            .withArgs(await projectToken.getAddress());
    });

    it("uses raw payment units and produces equal project-token amounts for 6 and 18 decimals", async function () {
        const fixture = await deployFixture();
        const { platform, usdt, usdc, user1, user2 } = fixture;
        const usdtProject = await createProject(fixture, { projectId: 1n, paymentToken: usdt });
        const usdcProject = await createProject(fixture, { projectId: 2n, paymentToken: usdc });
        await time.increaseTo(usdtProject.startTime);

        await approveAndPurchase(platform, usdt, user1, 1, ethers.parseUnits("100", 6));
        await approveAndPurchase(platform, usdc, user2, 2, ethers.parseUnits("100", 18));

        const purchase1 = await platform.getUserPurchase(1, user1.address);
        const purchase2 = await platform.getUserPurchase(2, user2.address);
        expect(purchase1.paymentAmount).to.equal(ethers.parseUnits("100", 6));
        expect(purchase2.paymentAmount).to.equal(ethers.parseUnits("100", 18));
        expect(purchase1.tokensBought).to.equal(ethers.parseEther("50"));
        expect(purchase2.tokensBought).to.equal(ethers.parseEther("50"));
        expect((await platform.getProject(1)).totalRaised).to.equal(ethers.parseUnits("100", 6));
        expect((await platform.getProject(2)).totalRaised).to.equal(ethers.parseUnits("100", 18));
    });

    it("isolates user limits and payment balances across USDT and USDC projects", async function () {
        const fixture = await deployFixture();
        const { platform, usdt, usdc, user1 } = fixture;
        const first = await createProject(fixture, { projectId: 1n, paymentToken: usdt });
        await createProject(fixture, { projectId: 2n, paymentToken: usdc });
        await time.increaseTo(first.startTime);

        const usdtAmount = ethers.parseUnits("600", 6);
        const usdcAmount = ethers.parseUnits("600", 18);
        await approveAndPurchase(platform, usdt, user1, 1, usdtAmount);
        await approveAndPurchase(platform, usdc, user1, 2, usdcAmount);

        expect(await usdt.balanceOf(await platform.getAddress())).to.equal(usdtAmount);
        expect(await usdc.balanceOf(await platform.getAddress())).to.equal(usdcAmount);
        await expect(approveAndPurchase(platform, usdt, user1, 1, 1n))
            .to.be.revertedWithCustomError(platform, "ExceedsMaxPerUser");
    });

    it("emits reconstructable purchase data and finalizes exactly at the hard cap", async function () {
        const fixture = await deployFixture();
        const { platform, usdt, user1, user2 } = fixture;
        const project = await createProject(fixture);
        await time.increaseTo(project.startTime);

        const firstAmount = ethers.parseUnits("600", 6);
        await usdt.connect(user1).approve(await platform.getAddress(), firstAmount);
        await expect(platform.connect(user1).purchase(1, firstAmount))
            .to.emit(platform, "Purchased")
            .withArgs(
                1,
                user1.address,
                await usdt.getAddress(),
                firstAmount,
                await fixture.projectToken.getAddress(),
                ethers.parseEther("300")
            );

        const finalAmount = ethers.parseUnits("400", 6);
        await usdt.connect(user2).approve(await platform.getAddress(), finalAmount);
        await expect(platform.connect(user2).purchase(1, finalAmount))
            .to.emit(platform, "ProjectEnded")
            .withArgs(1, 2, await usdt.getAddress(), project.hardCap);

        const stored = await platform.getProject(1);
        expect(stored.status).to.equal(2); // Successful
        expect(stored.totalRaised).to.equal(project.hardCap);
        await expect(platform.connect(user2).purchase(1, 1n))
            .to.be.revertedWithCustomError(platform, "InvalidProjectStatus");
    });

    it("settles a successful project in its payment token using the creation-time fee snapshot", async function () {
        const fixture = await deployFixture();
        const { platform, usdt, user1, projectOwner, feeReceiver, nextFeeReceiver } = fixture;
        const project = await createProject(fixture);
        await platform.setPlatformFees(500, nextFeeReceiver.address);
        await time.increaseTo(project.startTime);

        const amount = ethers.parseUnits("500", 6);
        await approveAndPurchase(platform, usdt, user1, 1, amount);
        await time.increaseTo(project.endTime);
        await platform.endProject(1);

        const ownerBefore = await usdt.balanceOf(projectOwner.address);
        const feeBefore = await usdt.balanceOf(feeReceiver.address);
        await platform.connect(projectOwner).withdrawRaisedFunds(1);

        expect(await usdt.balanceOf(projectOwner.address) - ownerBefore).to.equal(ethers.parseUnits("490", 6));
        expect(await usdt.balanceOf(feeReceiver.address) - feeBefore).to.equal(0);
        expect(await usdt.balanceOf(await platform.getAddress())).to.equal(ethers.parseUnits("10", 6));

        await platform.withdrawPlatformFee(1);
        expect(await usdt.balanceOf(feeReceiver.address) - feeBefore).to.equal(ethers.parseUnits("10", 6));
        expect(await usdt.balanceOf(nextFeeReceiver.address)).to.equal(0);
        await expect(platform.connect(projectOwner).withdrawRaisedFunds(1))
            .to.be.revertedWithCustomError(platform, "AlreadyWithdrawn");
        await expect(platform.withdrawPlatformFee(1))
            .to.be.revertedWithCustomError(platform, "AlreadyWithdrawn");
    });

    it("keeps sold project tokens reserved and releases them through vesting", async function () {
        const fixture = await deployFixture();
        const { platform, usdt, user1, user2, projectOwner, projectToken } = fixture;
        const project = await createProject(fixture);
        await time.increaseTo(project.startTime);

        await approveAndPurchase(platform, usdt, user1, 1, ethers.parseUnits("500", 6));
        await time.increaseTo(project.endTime);
        await platform.endProject(1);
        await platform.connect(projectOwner).withdrawProjectTokens(1);

        expect(await projectToken.balanceOf(await platform.getAddress())).to.equal(ethers.parseEther("250"));
        await time.increaseTo(project.vestingTimes[0]);
        expect(await platform.calculateClaimable(1, user1.address)).to.equal(ethers.parseEther("62.5"));
        await platform.connect(user1).claim(1);
        expect(await projectToken.balanceOf(user1.address)).to.equal(ethers.parseEther("62.5"));

        await time.increaseTo(project.vestingTimes[1]);
        expect(await platform.calculateClaimable(1, user1.address)).to.equal(ethers.parseEther("187.5"));
        await platform.connect(user1).claim(1);
        expect(await projectToken.balanceOf(user1.address)).to.equal(ethers.parseEther("250"));
        expect(await projectToken.balanceOf(await platform.getAddress())).to.equal(0);
        await expect(platform.connect(user1).claim(1))
            .to.be.revertedWithCustomError(platform, "NoTokensToClaim");
    });

    it("refunds a failed project in the original token exactly once", async function () {
        const fixture = await deployFixture();
        const { platform, usdc, user1 } = fixture;
        const project = await createProject(fixture, { paymentToken: usdc });
        await time.increaseTo(project.startTime);
        const amount = ethers.parseUnits("100", 18);
        await approveAndPurchase(platform, usdc, user1, 1, amount);
        await time.increaseTo(project.endTime);
        await platform.endProject(1);

        const before = await usdc.balanceOf(user1.address);
        await platform.connect(user1).refund(1);
        expect(await usdc.balanceOf(user1.address) - before).to.equal(amount);
        await expect(platform.connect(user1).refund(1))
            .to.be.revertedWithCustomError(platform, "AlreadyRefunded");
    });

    it("batch-refunds only eligible users and leaves no payment-token deficit", async function () {
        const fixture = await deployFixture();
        const { platform, usdt, user1, user2 } = fixture;
        const project = await createProject(fixture);
        await time.increaseTo(project.startTime);
        const amount1 = ethers.parseUnits("100", 6);
        const amount2 = ethers.parseUnits("200", 6);
        await approveAndPurchase(platform, usdt, user1, 1, amount1);
        await approveAndPurchase(platform, usdt, user2, 1, amount2);
        await time.increaseTo(project.endTime);
        await platform.endProject(1);

        const before1 = await usdt.balanceOf(user1.address);
        const before2 = await usdt.balanceOf(user2.address);
        await platform.batchRefund(1, [user1.address, user2.address, user1.address]);
        expect(await usdt.balanceOf(user1.address) - before1).to.equal(amount1);
        expect(await usdt.balanceOf(user2.address) - before2).to.equal(amount2);
        expect(await usdt.balanceOf(await platform.getAddress())).to.equal(0);
        expect((await platform.getUserPurchase(1, user1.address)).refunded).to.equal(true);
        expect((await platform.getUserPurchase(1, user2.address)).refunded).to.equal(true);
    });

    it("cancellation blocks purchases, enables same-token refunds, and returns all project tokens", async function () {
        const fixture = await deployFixture();
        const { platform, usdt, user1, projectOwner, projectToken } = fixture;
        const project = await createProject(fixture);
        await time.increaseTo(project.startTime);
        const payment = ethers.parseUnits("100", 6);
        await approveAndPurchase(platform, usdt, user1, 1, payment);
        await platform.cancelProject(1);

        await expect(platform.connect(user1).purchase(1, 1n))
            .to.be.revertedWithCustomError(platform, "InvalidProjectStatus");
        await platform.connect(user1).refund(1);

        const before = await projectToken.balanceOf(projectOwner.address);
        await platform.connect(projectOwner).withdrawProjectTokens(1);
        expect(await projectToken.balanceOf(projectOwner.address) - before).to.equal(project.depositAmount);
        expect(await usdt.balanceOf(await platform.getAddress())).to.equal(0);
    });

    it("rejects fee-on-transfer payment tokens instead of creating an accounting deficit", async function () {
        const fixture = await deployFixture();
        const { platform, projectOwner, projectToken, user1 } = fixture;
        const FeeToken = await ethers.getContractFactory("MockFeeOnTransferToken");
        const feeToken = await FeeToken.deploy(1000000);
        await platform.configurePaymentToken(await feeToken.getAddress(), true);
        await feeToken.transfer(user1.address, ethers.parseEther("1000"));
        const project = await createProject(fixture, {
            paymentToken: feeToken,
            tokenPrice: ethers.parseEther("2"),
            hardCap: ethers.parseEther("1000"),
            softCap: ethers.parseEther("500"),
            maxPerUser: ethers.parseEther("600"),
        });
        await time.increaseTo(project.startTime);
        const amount = ethers.parseEther("100");
        await feeToken.connect(user1).approve(await platform.getAddress(), amount);

        await expect(platform.connect(user1).purchase(1, amount))
            .to.be.revertedWithCustomError(platform, "UnsupportedTokenBehavior");
        expect((await platform.getProject(1)).totalRaised).to.equal(0);
        expect((await platform.getUserPurchase(1, user1.address)).paymentAmount).to.equal(0);
        expect(await projectToken.balanceOf(await platform.getAddress())).to.equal(project.depositAmount);
        expect(projectOwner.address).not.to.equal(ethers.ZeroAddress);
    });

    it("rejects fee-on-transfer project-token deposits", async function () {
        const fixture = await deployFixture();
        const { platform, usdt, projectOwner } = fixture;
        const FeeToken = await ethers.getContractFactory("MockFeeOnTransferToken");
        const projectToken = await FeeToken.deploy(1000000);
        await platform.configureProjectToken(await projectToken.getAddress(), true);
        await projectToken.transfer(projectOwner.address, ethers.parseEther("1000"));
        const latest = await time.latest();
        await platform.createProject(
            11,
            await projectToken.getAddress(),
            await usdt.getAddress(),
            projectOwner.address,
            fixture.feeReceiver.address,
            200,
            ethers.parseUnits("2", 6),
            ethers.parseUnits("1000", 6),
            ethers.parseUnits("500", 6),
            ethers.parseUnits("600", 6),
            latest + 100,
            latest + 200,
            [latest + 300],
            [10000]
        );
        const deposit = ethers.parseEther("100");
        await projectToken.connect(projectOwner).approve(await platform.getAddress(), deposit);

        await expect(platform.connect(projectOwner).depositProjectTokens(11, deposit))
            .to.be.revertedWithCustomError(platform, "UnsupportedTokenBehavior");
        expect((await platform.getProject(11)).totalTokensDeposited).to.equal(0);
    });

    it("rejects an outgoing-fee token without consuming another project's pooled reserve", async function () {
        const fixture = await deployFixture();
        const { platform, projectToken, projectOwner, user1, user2 } = fixture;
        const OutgoingFeeToken = await ethers.getContractFactory("MockOutgoingFeeToken");
        const paymentToken = await OutgoingFeeToken.deploy("Outgoing Fee USD", "OFUSD", 6);
        await platform.configurePaymentToken(await paymentToken.getAddress(), true);

        await paymentToken.mint(user1.address, ethers.parseUnits("1000", 6));
        await paymentToken.mint(user2.address, ethers.parseUnits("1000", 6));
        const first = await createProject(fixture, { projectId: 21n, paymentToken });
        const second = await createProject(fixture, { projectId: 22n, paymentToken });
        await time.increaseTo(first.startTime);
        const amount = ethers.parseUnits("100", 6);
        await approveAndPurchase(platform, paymentToken, user1, 21, amount);
        await approveAndPurchase(platform, paymentToken, user2, 22, amount);
        await time.increaseTo(Math.max(first.endTime, second.endTime));
        await platform.endProject(21);
        await platform.endProject(22);

        const platformAddress = await platform.getAddress();
        const balanceBefore = await paymentToken.balanceOf(platformAddress);
        const reserveBefore = await platform.reservedBalances(await paymentToken.getAddress());
        await expect(platform.connect(user1).refund(21))
            .to.be.revertedWithCustomError(platform, "UnsupportedOutgoingTokenBehavior");
        expect(await paymentToken.balanceOf(platformAddress)).to.equal(balanceBefore);
        expect(await platform.reservedBalances(await paymentToken.getAddress())).to.equal(reserveBefore);
        expect((await platform.getUserPurchase(21, user1.address)).refunded).to.equal(false);
        expect((await platform.getProject(22)).totalRaised).to.equal(amount);
        expect(await projectToken.balanceOf(platformAddress)).to.be.greaterThan(0);
        expect(projectOwner.address).not.to.equal(ethers.ZeroAddress);
    });

    it("fails closed after a negative rebase instead of spending another project's reserve", async function () {
        const fixture = await deployFixture();
        const { platform, user1, user2 } = fixture;
        const NegativeRebaseToken = await ethers.getContractFactory("MockNegativeRebaseToken");
        const paymentToken = await NegativeRebaseToken.deploy();
        await platform.configurePaymentToken(await paymentToken.getAddress(), true);
        await paymentToken.transfer(user1.address, ethers.parseEther("1000"));
        await paymentToken.transfer(user2.address, ethers.parseEther("1000"));

        const first = await createProject(fixture, {
            projectId: 31n,
            paymentToken,
            tokenPrice: ethers.parseEther("2"),
            hardCap: ethers.parseEther("1000"),
            softCap: ethers.parseEther("500"),
            maxPerUser: ethers.parseEther("600"),
        });
        const second = await createProject(fixture, {
            projectId: 32n,
            paymentToken,
            tokenPrice: ethers.parseEther("2"),
            hardCap: ethers.parseEther("1000"),
            softCap: ethers.parseEther("500"),
            maxPerUser: ethers.parseEther("600"),
        });
        await time.increaseTo(first.startTime);
        const amount = ethers.parseEther("100");
        await approveAndPurchase(platform, paymentToken, user1, 31, amount);
        await approveAndPurchase(platform, paymentToken, user2, 32, amount);
        await time.increaseTo(Math.max(first.endTime, second.endTime));
        await platform.endProject(31);
        await platform.endProject(32);

        const platformAddress = await platform.getAddress();
        await paymentToken.slash(platformAddress, ethers.parseEther("50"));
        expect(await paymentToken.balanceOf(platformAddress)).to.equal(ethers.parseEther("150"));
        expect(await platform.reservedBalances(await paymentToken.getAddress())).to.equal(ethers.parseEther("200"));
        await expect(platform.connect(user1).refund(31))
            .to.be.revertedWithCustomError(platform, "TokenReserveInsolvent")
            .withArgs(await paymentToken.getAddress(), ethers.parseEther("150"), ethers.parseEther("200"));
        expect((await platform.getUserPurchase(31, user1.address)).refunded).to.equal(false);
        expect(await paymentToken.balanceOf(platformAddress)).to.equal(ethers.parseEther("150"));
    });

    it("restricts governance, lifecycle overrides, and upgrades to the owner", async function () {
        const fixture = await deployFixture();
        const { platform, usdt, user1, nextFeeReceiver } = fixture;
        await createProject(fixture);

        await expect(platform.connect(user1).configurePaymentToken(await usdt.getAddress(), false))
            .to.be.revertedWithCustomError(platform, "OwnableUnauthorizedAccount")
            .withArgs(user1.address);
        await expect(platform.connect(user1).setPlatformFees(100, nextFeeReceiver.address))
            .to.be.revertedWithCustomError(platform, "OwnableUnauthorizedAccount")
            .withArgs(user1.address);
        await expect(platform.connect(user1).pause())
            .to.be.revertedWithCustomError(platform, "OwnableUnauthorizedAccount")
            .withArgs(user1.address);
        await expect(platform.connect(user1).cancelProject(1))
            .to.be.revertedWithCustomError(platform, "OwnableUnauthorizedAccount")
            .withArgs(user1.address);
        await expect(platform.renounceOwnership())
            .to.be.revertedWithCustomError(platform, "OwnershipRenunciationDisabled");
    });

    it("keeps a dedicated lifecycle operator after owner migrates to a Safe", async function () {
        const fixture = await deployFixture();
        const { platform, owner, safe, projectOwner, projectToken, usdt, feeReceiver } = fixture;
        await platform.transferOwnership(safe.address);
        await platform.connect(safe).acceptOwnership();
        expect(await platform.operator()).to.equal(owner.address);

        const latest = await time.latest();
        await expect(platform.connect(owner).createProject(
            88, await projectToken.getAddress(), await usdt.getAddress(), projectOwner.address,
            feeReceiver.address, 200, ethers.parseUnits("2", 6), ethers.parseUnits("1000", 6),
            ethers.parseUnits("500", 6), ethers.parseUnits("600", 6), latest + 100, latest + 200,
            [latest + 300], [10000]
        )).to.emit(platform, "ProjectCreated");
        await expect(platform.connect(projectOwner).activateProject(88))
            .to.be.revertedWithCustomError(platform, "UnauthorizedOperator");
        await expect(platform.connect(safe).setOperator(projectOwner.address))
            .to.emit(platform, "OperatorUpdated").withArgs(owner.address, projectOwner.address);
    });

    it("preserves project and purchase storage across a compatible UUPS upgrade", async function () {
        const fixture = await deployFixture();
        const { platform, usdt, user1, owner } = fixture;
        const project = await createProject(fixture);
        await time.increaseTo(project.startTime);
        const amount = ethers.parseUnits("100", 6);
        await approveAndPurchase(platform, usdt, user1, 1, amount);

        const proxyAddress = await platform.getAddress();
        const Upgrade = await ethers.getContractFactory("JoinCareIDOv2UpgradeMock");
        const upgraded = await upgrades.upgradeProxy(proxyAddress, Upgrade, { kind: "uups" });

        expect(await upgraded.getAddress()).to.equal(proxyAddress);
        expect(await upgraded.implementationVersion()).to.equal(2);
        expect(await upgraded.owner()).to.equal(owner.address);
        expect(await upgraded.operator()).to.equal(owner.address);
        const storedProject = await upgraded.getProject(1);
        const storedPurchase = await upgraded.getUserPurchase(1, user1.address);
        expect(storedProject.paymentToken).to.equal(await usdt.getAddress());
        expect(storedProject.totalRaised).to.equal(amount);
        expect(storedPurchase.paymentAmount).to.equal(amount);
        expect(storedPurchase.tokensBought).to.equal(ethers.parseEther("50"));
    });

    it("pause blocks new inflows but cannot freeze terminal exits or a pending owner cancellation", async function () {
        const fixture = await deployFixture();
        const { platform, usdt, user1, user2, projectOwner, projectToken } = fixture;
        const project = await createProject(fixture);
        await time.increaseTo(project.startTime);
        const amount = ethers.parseUnits("100", 6);
        await approveAndPurchase(platform, usdt, user1, 1, amount);
        await time.increaseTo(project.endTime);
        await platform.pause();

        await expect(platform.connect(user1).purchase(1, 1n))
            .to.be.revertedWithCustomError(platform, "EnforcedPause");
        await platform.connect(user2).endProject(1);
        await platform.connect(user1).refund(1);
        await platform.connect(projectOwner).withdrawProjectTokens(1);
        expect(await usdt.balanceOf(await platform.getAddress())).to.equal(0);

        const latest = await time.latest();
        await platform.unpause();
        await platform.createProject(
            2,
            await projectToken.getAddress(),
            await usdt.getAddress(),
            projectOwner.address,
            fixture.feeReceiver.address,
            200,
            ethers.parseUnits("2", 6),
            ethers.parseUnits("1000", 6),
            ethers.parseUnits("500", 6),
            ethers.parseUnits("600", 6),
            latest + 100,
            latest + 200,
            [latest + 300],
            [10000]
        );
        const pendingDeposit = ethers.parseEther("10");
        await projectToken.connect(projectOwner).approve(await platform.getAddress(), pendingDeposit);
        await platform.connect(projectOwner).depositProjectTokens(2, pendingDeposit);
        await platform.pause();
        await platform.connect(projectOwner).cancelProject(2);
        const ownerBefore = await projectToken.balanceOf(projectOwner.address);
        await platform.connect(projectOwner).withdrawProjectTokens(2);
        expect(await projectToken.balanceOf(projectOwner.address) - ownerBefore).to.equal(pendingDeposit);
    });


    it("fails closed when token decimals drift during the project lifecycle", async function () {
        const fixture = await deployFixture();
        const { platform, projectToken, projectOwner, feeReceiver, user1 } = fixture;
        const MutableToken = await ethers.getContractFactory("MockMutableDecimalsToken");
        const paymentToken = await MutableToken.deploy(6);
        const paymentAddress = await paymentToken.getAddress();
        await platform.configurePaymentToken(paymentAddress, true);

        const latest = await time.latest();
        const createArgs = [
            70, await projectToken.getAddress(), paymentAddress, projectOwner.address,
            feeReceiver.address, 200, ethers.parseUnits("2", 6), ethers.parseUnits("1000", 6),
            ethers.parseUnits("500", 6), ethers.parseUnits("600", 6), latest + 100, latest + 200,
            [latest + 300], [10000],
        ];
        await paymentToken.setDecimals(18);
        await expect(platform.createProject(...createArgs))
            .to.be.revertedWithCustomError(platform, "TokenDecimalsChanged")
            .withArgs(paymentAddress, 6, 18);

        await paymentToken.setDecimals(6);
        await platform.createProject(...createArgs);
        const deposit = ethers.parseEther("250");
        await projectToken.connect(projectOwner).approve(await platform.getAddress(), deposit);
        await platform.connect(projectOwner).depositProjectTokens(70, deposit);
        await platform.activateProject(70);
        await paymentToken.transfer(user1.address, ethers.parseUnits("100", 6));
        await time.increaseTo(latest + 100);
        await paymentToken.connect(user1).approve(await platform.getAddress(), ethers.parseUnits("100", 6));
        await paymentToken.setDecimals(18);
        await expect(platform.connect(user1).purchase(70, ethers.parseUnits("100", 6)))
            .to.be.revertedWithCustomError(platform, "TokenDecimalsChanged")
            .withArgs(paymentAddress, 6, 18);
    });

    it("requires enough project inventory to cover the soft cap before activation", async function () {
        const fixture = await deployFixture();
        const { platform, usdt, projectToken, projectOwner, feeReceiver } = fixture;
        const latest = await time.latest();
        await platform.createProject(
            71, await projectToken.getAddress(), await usdt.getAddress(), projectOwner.address,
            feeReceiver.address, 200, ethers.parseUnits("2", 6), ethers.parseUnits("1000", 6),
            ethers.parseUnits("500", 6), ethers.parseUnits("600", 6), latest + 100, latest + 200,
            [latest + 300], [10000]
        );
        const below = ethers.parseEther("249");
        await projectToken.connect(projectOwner).approve(await platform.getAddress(), ethers.parseEther("250"));
        await platform.connect(projectOwner).depositProjectTokens(71, below);
        await expect(platform.activateProject(71)).to.be.revertedWithCustomError(platform, "InsufficientProjectTokens");
        await platform.connect(projectOwner).depositProjectTokens(71, ethers.parseEther("1"));
        await expect(platform.activateProject(71)).to.emit(platform, "ProjectActivated").withArgs(71);
    });

    it("ends purchases strictly before endTime while permitting finalization at endTime", async function () {
        const fixture = await deployFixture();
        const { platform, usdt, user1 } = fixture;
        const project = await createProject(fixture);
        await time.increaseTo(project.endTime);
        await usdt.connect(user1).approve(await platform.getAddress(), ethers.parseUnits("1", 6));
        await expect(platform.connect(user1).purchase(1, ethers.parseUnits("1", 6)))
            .to.be.revertedWithCustomError(platform, "ProjectEndedAlready");
        await expect(platform.endProject(1)).to.emit(platform, "ProjectEnded");
    });

    it("recovers only token balances above the recorded reserve", async function () {
        const fixture = await deployFixture();
        const { platform, projectToken, projectOwner, safe, user1 } = fixture;
        await createProject(fixture);
        const tokenAddress = await projectToken.getAddress();
        const excess = ethers.parseEther("10");
        await projectToken.connect(projectOwner).transfer(await platform.getAddress(), excess);
        await expect(platform.connect(user1).recoverExcessTokens(tokenAddress, safe.address, 1))
            .to.be.revertedWithCustomError(platform, "OwnableUnauthorizedAccount");
        await expect(platform.recoverExcessTokens(tokenAddress, safe.address, excess + 1n))
            .to.be.revertedWithCustomError(platform, "ExcessiveRecoveryRequested")
            .withArgs(tokenAddress, excess);
        await expect(platform.recoverExcessTokens(tokenAddress, safe.address, excess))
            .to.emit(platform, "ExcessTokensRecovered").withArgs(tokenAddress, safe.address, excess);
        expect(await projectToken.balanceOf(safe.address)).to.equal(excess);
        expect(await platform.reservedBalances(tokenAddress)).to.equal(ethers.parseEther("1000"));
    });

    it("pause cannot freeze successful-project proceeds, fees, or claims", async function () {
        const fixture = await deployFixture();
        const { platform, usdt, user1, projectOwner, feeReceiver, projectToken } = fixture;
        const project = await createProject(fixture);
        await time.increaseTo(project.startTime);
        await approveAndPurchase(platform, usdt, user1, 1, ethers.parseUnits("500", 6));
        await time.increaseTo(project.endTime);
        await platform.endProject(1);
        await time.increaseTo(project.vestingTimes[0]);
        await platform.pause();

        await platform.connect(projectOwner).withdrawRaisedFunds(1);
        await platform.withdrawPlatformFee(1);
        await platform.connect(user1).claim(1);

        expect(await usdt.balanceOf(projectOwner.address)).to.equal(ethers.parseUnits("490", 6));
        expect(await usdt.balanceOf(feeReceiver.address)).to.equal(ethers.parseUnits("10", 6));
        expect(await projectToken.balanceOf(user1.address)).to.equal(ethers.parseEther("62.5"));
    });
});
