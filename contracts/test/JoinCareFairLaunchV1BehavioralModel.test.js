const { expect } = require("chai");
const { ethers } = require("hardhat");
const { time } = require("@nomicfoundation/hardhat-network-helpers");

const describeModel = process.env.JC_FAIR_LAUNCH_MODEL_ONLY === "1" ? describe : describe.skip;

describeModel("JoinCareFairLaunchV1BehavioralModel (NON-PRODUCTION)", function () {
  const PROJECT = 1n;
  const SALE = ethers.parseEther("1000");
  const SOFT_CAP = ethers.parseUnits("100", 6);
  const MIN = ethers.parseUnits("10", 6);
  const MAX = ethers.parseUnits("300", 6);

  async function deployToken(name, symbol, decimals = 18) {
    const Token = await ethers.getContractFactory("ModelExactERC20");
    return Token.deploy(name, symbol, decimals);
  }

  async function deployAdversarialToken(name, symbol, decimals = 18) {
    const Token = await ethers.getContractFactory("ModelAdversarialERC20");
    return Token.deploy(name, symbol, decimals);
  }

  async function deployFixture(options = {}) {
    const [operator, projectOwner, user1, user2, outsider] = await ethers.getSigners();
    const projectToken = options.projectToken || await deployToken("Project", "PRJ", 18);
    const paymentToken = options.paymentToken || await deployToken("USD", "USD", 6);
    const Model = await ethers.getContractFactory("JoinCareFairLaunchV1BehavioralModel");
    const model = await Model.deploy(operator.address);
    const now = await time.latest();
    const startTime = now + 100;
    const endTime = startTime + 1_000;
    const params = {
      projectId: options.projectId || PROJECT,
      projectToken: await projectToken.getAddress(),
      paymentToken: await paymentToken.getAddress(),
      projectOwner: projectOwner.address,
      saleTokenAmount: options.saleTokenAmount || SALE,
      softCap: options.softCap || SOFT_CAP,
      minContribution: options.minContribution || MIN,
      maxContribution: options.maxContribution === undefined ? MAX : options.maxContribution,
      startTime,
      endTime,
    };
    const vestingTimes = [endTime, endTime + 100];
    const vestingBps = [2500, 7500];
    await model.createProject(params, vestingTimes, vestingBps);
    return { model, projectToken, paymentToken, operator, projectOwner, user1, user2, outsider, params, vestingTimes };
  }

  async function depositAndActivate(f) {
    await f.projectToken.mint(f.projectOwner.address, f.params.saleTokenAmount);
    await f.projectToken.connect(f.projectOwner).approve(await f.model.getAddress(), f.params.saleTokenAmount);
    await f.model.connect(f.projectOwner).depositProjectTokens(PROJECT, f.params.saleTokenAmount);
    await f.model.activateProject(PROJECT);
  }

  async function contribute(f, user, amount) {
    await f.paymentToken.mint(user.address, amount);
    await f.paymentToken.connect(user).approve(await f.model.getAddress(), amount);
    return f.model.connect(user).contribute(PROJECT, amount);
  }

  it("is explicitly labeled non-production and preserves frozen status ordering", async function () {
    const f = await deployFixture();
    expect(await f.model.modelDisclaimer()).to.equal("NON_PRODUCTION_BEHAVIORAL_MODEL_DO_NOT_DEPLOY");
    expect((await f.model.getProject(PROJECT)).status).to.equal(0n); // Pending
    await depositAndActivate(f);
    expect((await f.model.getProject(PROJECT)).status).to.equal(1n); // Active
  });

  it("requires exactly saleTokenAmount before activation and conserves aggregate token reserve", async function () {
    const f = await deployFixture();
    await f.projectToken.mint(f.projectOwner.address, SALE + 1n);
    await f.projectToken.connect(f.projectOwner).approve(await f.model.getAddress(), SALE + 1n);
    await f.model.connect(f.projectOwner).depositProjectTokens(PROJECT, SALE / 2n);
    await expect(f.model.activateProject(PROJECT))
      .to.be.revertedWithCustomError(f.model, "InsufficientProjectTokens")
      .withArgs(SALE, SALE / 2n);
    await expect(f.model.connect(f.projectOwner).depositProjectTokens(PROJECT, SALE / 2n + 1n))
      .to.be.revertedWithCustomError(f.model, "ExceedsSaleTokenAmount");
    await f.model.connect(f.projectOwner).depositProjectTokens(PROJECT, SALE / 2n);
    expect(await f.model.reservedBalance(await f.projectToken.getAddress())).to.equal(SALE);
    expect(await f.projectToken.balanceOf(await f.model.getAddress())).to.equal(SALE);
    await f.model.activateProject(PROJECT);
  });

  it("has no hard cap, enforces per-transaction minimum and cumulative maximum", async function () {
    const f = await deployFixture({ maxContribution: MAX });
    await depositAndActivate(f);
    await time.increaseTo(f.params.startTime);
    await expect(contribute(f, f.user1, MIN - 1n))
      .to.be.revertedWithCustomError(f.model, "BelowMinimumContribution")
      .withArgs(MIN, MIN - 1n);
    await contribute(f, f.user1, ethers.parseUnits("200", 6));
    await contribute(f, f.user1, ethers.parseUnits("100", 6));
    await expect(contribute(f, f.user1, MIN))
      .to.be.revertedWithCustomError(f.model, "ExceedsMaxContribution");

    const unlimited = await deployFixture({ maxContribution: 0n, softCap: SOFT_CAP });
    await unlimited.projectToken.mint(unlimited.projectOwner.address, SALE);
    await unlimited.projectToken.connect(unlimited.projectOwner).approve(await unlimited.model.getAddress(), SALE);
    await unlimited.model.connect(unlimited.projectOwner).depositProjectTokens(PROJECT, SALE);
    await unlimited.model.activateProject(PROJECT);
    await time.increaseTo(unlimited.params.startTime);
    const aboveAnyCap = ethers.parseUnits("1000000", 6);
    await unlimited.paymentToken.mint(unlimited.user1.address, aboveAnyCap);
    await unlimited.paymentToken.connect(unlimited.user1).approve(await unlimited.model.getAddress(), aboveAnyCap);
    await unlimited.model.connect(unlimited.user1).contribute(PROJECT, aboveAnyCap);
    expect((await unlimited.model.getProject(PROJECT)).totalRaised).to.equal(aboveAnyCap);
  });

  it("aggregates project-token reserves across projects sharing one token", async function () {
    const f = await deployFixture();
    const second = { ...f.params, projectId: 2n, startTime: f.params.startTime + 1, endTime: f.params.endTime + 1 };
    await f.model.createProject(second, [second.endTime, second.endTime + 100], [5000, 5000]);
    await f.projectToken.mint(f.projectOwner.address, SALE * 2n);
    await f.projectToken.connect(f.projectOwner).approve(await f.model.getAddress(), SALE * 2n);
    await f.model.connect(f.projectOwner).depositProjectTokens(PROJECT, SALE);
    await f.model.connect(f.projectOwner).depositProjectTokens(2n, SALE);
    expect(await f.model.reservedBalance(await f.projectToken.getAddress())).to.equal(SALE * 2n);
    expect(await f.projectToken.balanceOf(await f.model.getAddress())).to.equal(SALE * 2n);
  });

  it("ends in O(1), freezes rational price, and uses Math.mulDiv allocation", async function () {
    const f = await deployFixture({ maxContribution: 0n });
    await depositAndActivate(f);
    await time.increaseTo(f.params.startTime);
    const a = ethers.parseUnits("123", 6);
    const b = ethers.parseUnits("377", 6);
    await contribute(f, f.user1, a);
    await contribute(f, f.user2, b);
    await time.increaseTo(f.params.endTime);
    const tx = await f.model.connect(f.outsider).endProject(PROJECT);
    const receipt = await tx.wait();
    expect(receipt.gasUsed).to.be.lessThan(150_000n);
    const project = await f.model.getProject(PROJECT);
    expect(project.status).to.equal(2n); // SuccessfulPendingRelease
    expect(project.finalTotalRaised).to.equal(a + b);
    expect(project.finalPriceNumerator).to.equal(a + b);
    expect(project.finalPriceDenominator).to.equal(SALE);
    expect(await f.model.calculateFinalAllocation(PROJECT, f.user1.address))
      .to.equal((a * SALE) / (a + b));
    expect((await f.model.getPosition(PROJECT, f.user1.address)).contributionAmount).to.equal(a);
  });

  it("keeps endProject gas independent of materially different participant counts", async function () {
    const signers = await ethers.getSigners();

    async function endGas(participantCount) {
      const f = await deployFixture({ maxContribution: 0n, softCap: 1n, minContribution: 1n });
      await depositAndActivate(f);
      await time.increaseTo(f.params.startTime);
      for (let i = 0; i < participantCount; ++i) {
        const user = signers[4 + i];
        await contribute(f, user, 1n);
      }
      await time.increaseTo(f.params.endTime);
      return (await (await f.model.connect(f.outsider).endProject(PROJECT)).wait()).gasUsed;
    }

    const oneParticipantGas = await endGas(1);
    const twelveParticipantGas = await endGas(12);
    expect(twelveParticipantGas).to.equal(oneParticipantGas);
    expect(twelveParticipantGas).to.be.lessThan(150_000n);
  });

  it("uses full-precision Math.mulDiv when the multiplication product exceeds uint256", async function () {
    const hugeSale = 1n << 200n;
    const paymentEach = 1n << 100n;
    const f = await deployFixture({
      saleTokenAmount: hugeSale,
      softCap: 1n,
      minContribution: 1n,
      maxContribution: 0n,
    });
    await depositAndActivate(f);
    await time.increaseTo(f.params.startTime);
    await contribute(f, f.user1, paymentEach);
    await contribute(f, f.user2, paymentEach);
    await time.increaseTo(f.params.endTime);
    await f.model.endProject(PROJECT);
    expect(await f.model.calculateFinalAllocation(PROJECT, f.user1.address)).to.equal(1n << 199n);
  });

  it("keeps floor-rounding dust reserved and independent of claim order", async function () {
    const f = await deployFixture({ saleTokenAmount: 1000n, softCap: 1n, minContribution: 1n, maxContribution: 0n });
    await depositAndActivate(f);
    await time.increaseTo(f.params.startTime);
    await contribute(f, f.user1, 1n);
    await contribute(f, f.user2, 2n);
    await time.increaseTo(f.params.endTime);
    await f.model.endProject(PROJECT);
    await f.model.acknowledgeSuccessfulReleaseForModel(PROJECT);
    await time.increaseTo(f.vestingTimes[1]);
    expect(await f.model.calculateFinalAllocation(PROJECT, f.user1.address)).to.equal(333n);
    expect(await f.model.calculateFinalAllocation(PROJECT, f.user2.address)).to.equal(666n);
    await f.model.connect(f.user2).claim(PROJECT);
    await f.model.connect(f.user1).claim(PROJECT);
    expect(await f.model.reservedBalance(await f.projectToken.getAddress())).to.equal(1n);
    expect(await f.projectToken.balanceOf(await f.model.getAddress())).to.equal(1n);
  });

  it("applies cumulative vesting math and conserves successful project-token reserve", async function () {
    const f = await deployFixture({ maxContribution: 0n });
    await depositAndActivate(f);
    await time.increaseTo(f.params.startTime);
    await contribute(f, f.user1, ethers.parseUnits("100", 6));
    await contribute(f, f.user2, ethers.parseUnits("300", 6));
    await time.increaseTo(f.params.endTime);
    await f.model.endProject(PROJECT);
    expect(await f.model.calculateClaimable(PROJECT, f.user1.address)).to.equal(0n);
    await f.model.acknowledgeSuccessfulReleaseForModel(PROJECT);
    const allocation = SALE / 4n;
    expect(await f.model.calculateFinalAllocation(PROJECT, f.user1.address)).to.equal(allocation);
    expect(await f.model.calculateClaimable(PROJECT, f.user1.address)).to.equal(allocation / 4n);
    await f.model.connect(f.outsider).claimFor(PROJECT, f.user1.address);
    expect(await f.projectToken.balanceOf(f.user1.address)).to.equal(allocation / 4n);
    expect(await f.model.reservedBalance(await f.projectToken.getAddress())).to.equal(SALE - allocation / 4n);
    await time.increaseTo(f.vestingTimes[1]);
    expect(await f.model.calculateClaimable(PROJECT, f.user1.address)).to.equal(allocation - allocation / 4n);
    await f.model.connect(f.user1).claim(PROJECT);
    expect(await f.projectToken.balanceOf(f.user1.address)).to.equal(allocation);
    expect(await f.model.reservedBalance(await f.projectToken.getAddress())).to.equal(SALE - allocation);
  });

  it("supports failed and cancelled refunds without changing obligations on duplicate attempts", async function () {
    const failed = await deployFixture({ softCap: ethers.parseUnits("1000", 6), maxContribution: 0n });
    await depositAndActivate(failed);
    await time.increaseTo(failed.params.startTime);
    const amount = ethers.parseUnits("50", 6);
    await contribute(failed, failed.user1, amount);
    await time.increaseTo(failed.params.endTime);
    await failed.model.endProject(PROJECT);
    expect((await failed.model.getProject(PROJECT)).status).to.equal(4n);
    await failed.model.connect(failed.outsider).refundFor(PROJECT, failed.user1.address);
    expect(await failed.paymentToken.balanceOf(failed.user1.address)).to.equal(amount);
    expect(await failed.model.reservedBalance(await failed.paymentToken.getAddress())).to.equal(0n);
    await expect(failed.model.connect(failed.user1).refund(PROJECT))
      .to.be.revertedWithCustomError(failed.model, "AlreadyRefunded");

    const cancelled = await deployFixture({ maxContribution: 0n });
    await depositAndActivate(cancelled);
    await time.increaseTo(cancelled.params.startTime);
    await contribute(cancelled, cancelled.user1, amount);
    await cancelled.model.cancelProject(PROJECT);
    expect((await cancelled.model.getProject(PROJECT)).status).to.equal(5n);
    await cancelled.model.connect(cancelled.user1).refund(PROJECT);
    await cancelled.model.connect(cancelled.projectOwner).withdrawFailedProjectTokens(PROJECT);
    expect(await cancelled.model.reservedBalance(await cancelled.projectToken.getAddress())).to.equal(0n);
  });

  it("rejects already-insolvent project-token deposits before transferFrom", async function () {
    const projectToken = await deployAdversarialToken("Project", "PRJ", 18);
    const f = await deployFixture({ projectToken });
    const firstDeposit = SALE / 2n;
    await projectToken.mint(f.projectOwner.address, SALE);
    await projectToken.connect(f.projectOwner).approve(await f.model.getAddress(), SALE);
    await f.model.connect(f.projectOwner).depositProjectTokens(PROJECT, firstDeposit);
    await projectToken.forceBurn(await f.model.getAddress(), 1n);
    await projectToken.resetTransferCounters();
    await projectToken.setTransferTraps(false, true);

    await expect(f.model.connect(f.projectOwner).depositProjectTokens(PROJECT, SALE - firstDeposit))
      .to.be.revertedWithCustomError(f.model, "TokenReserveInsolvent")
      .withArgs(await projectToken.getAddress(), firstDeposit - 1n, firstDeposit);
    expect(await projectToken.transferFromCalls()).to.equal(0n);
    expect((await f.model.getProject(PROJECT)).totalTokensDeposited).to.equal(firstDeposit);
    expect(await f.model.reservedBalance(await projectToken.getAddress())).to.equal(firstDeposit);
  });

  it("rejects already-insolvent contributions before transferFrom", async function () {
    const paymentToken = await deployAdversarialToken("USD", "USD", 6);
    const f = await deployFixture({ paymentToken, maxContribution: 0n });
    await depositAndActivate(f);
    await time.increaseTo(f.params.startTime);
    await contribute(f, f.user1, MIN);
    await paymentToken.forceBurn(await f.model.getAddress(), 1n);
    await paymentToken.mint(f.user2.address, MIN);
    await paymentToken.connect(f.user2).approve(await f.model.getAddress(), MIN);
    await paymentToken.resetTransferCounters();
    await paymentToken.setTransferTraps(false, true);

    await expect(f.model.connect(f.user2).contribute(PROJECT, MIN))
      .to.be.revertedWithCustomError(f.model, "TokenReserveInsolvent")
      .withArgs(await paymentToken.getAddress(), MIN - 1n, MIN);
    expect(await paymentToken.transferFromCalls()).to.equal(0n);
    expect((await f.model.getProject(PROJECT)).totalRaised).to.equal(MIN);
    expect((await f.model.getPosition(PROJECT, f.user2.address)).contributionAmount).to.equal(0n);
  });

  it("rejects already-insolvent refund, claim, and failed-token withdrawal before transfer", async function () {
    const payment = await deployAdversarialToken("USD", "USD", 6);
    const failed = await deployFixture({ paymentToken: payment, maxContribution: 0n, softCap: MIN * 2n });
    await depositAndActivate(failed);
    await time.increaseTo(failed.params.startTime);
    await contribute(failed, failed.user1, MIN);
    await time.increaseTo(failed.params.endTime);
    await failed.model.endProject(PROJECT);
    await payment.forceBurn(await failed.model.getAddress(), 1n);
    await payment.resetTransferCounters();
    await payment.setTransferTraps(true, false);
    await expect(failed.model.connect(failed.user1).refund(PROJECT))
      .to.be.revertedWithCustomError(failed.model, "TokenReserveInsolvent");
    expect(await payment.transferCalls()).to.equal(0n);
    expect((await failed.model.getPosition(PROJECT, failed.user1.address)).refunded).to.equal(false);

    const claimToken = await deployAdversarialToken("Project", "PRJ", 18);
    const successful = await deployFixture({ projectToken: claimToken, maxContribution: 0n, softCap: 1n, minContribution: 1n });
    await depositAndActivate(successful);
    await time.increaseTo(successful.params.startTime);
    await contribute(successful, successful.user1, 1n);
    await time.increaseTo(successful.params.endTime);
    await successful.model.endProject(PROJECT);
    await successful.model.acknowledgeSuccessfulReleaseForModel(PROJECT);
    await time.increaseTo(successful.vestingTimes[1]);
    await claimToken.forceBurn(await successful.model.getAddress(), 1n);
    await claimToken.resetTransferCounters();
    await claimToken.setTransferTraps(true, false);
    await expect(successful.model.connect(successful.user1).claim(PROJECT))
      .to.be.revertedWithCustomError(successful.model, "TokenReserveInsolvent");
    expect(await claimToken.transferCalls()).to.equal(0n);
    expect((await successful.model.getPosition(PROJECT, successful.user1.address)).claimedTokens).to.equal(0n);

    const withdrawalToken = await deployAdversarialToken("Project", "PRJ", 18);
    const cancelled = await deployFixture({ projectToken: withdrawalToken, maxContribution: 0n });
    await depositAndActivate(cancelled);
    await cancelled.model.cancelProject(PROJECT);
    await withdrawalToken.forceBurn(await cancelled.model.getAddress(), 1n);
    await withdrawalToken.resetTransferCounters();
    await withdrawalToken.setTransferTraps(true, false);
    await expect(cancelled.model.connect(cancelled.projectOwner).withdrawFailedProjectTokens(PROJECT))
      .to.be.revertedWithCustomError(cancelled.model, "TokenReserveInsolvent");
    expect(await withdrawalToken.transferCalls()).to.equal(0n);
    expect((await cancelled.model.getProject(PROJECT)).failedProjectTokensWithdrawn).to.equal(false);
  });

  it("rejects fee-on-transfer inputs and rolls back accounting", async function () {
    const FeeToken = await ethers.getContractFactory("ModelFeeOnTransferERC20");
    const feeProjectToken = await FeeToken.deploy();
    const normalPayment = await deployToken("USD", "USD", 6);
    const f = await deployFixture({ projectToken: feeProjectToken, paymentToken: normalPayment });
    await feeProjectToken.mint(f.projectOwner.address, SALE);
    await feeProjectToken.connect(f.projectOwner).approve(await f.model.getAddress(), SALE);
    await expect(f.model.connect(f.projectOwner).depositProjectTokens(PROJECT, SALE))
      .to.be.revertedWithCustomError(f.model, "UnsupportedTokenBehavior");
    expect(await f.model.reservedBalance(await feeProjectToken.getAddress())).to.equal(0n);
    expect((await f.model.getProject(PROJECT)).totalTokensDeposited).to.equal(0n);

    const normalProject = await deployToken("Project", "PRJ", 18);
    const feePayment = await FeeToken.deploy();
    const p = await deployFixture({ projectToken: normalProject, paymentToken: feePayment, maxContribution: 0n });
    await depositAndActivate(p);
    await time.increaseTo(p.params.startTime);
    await feePayment.mint(p.user1.address, ethers.parseEther("100"));
    await feePayment.connect(p.user1).approve(await p.model.getAddress(), ethers.parseEther("100"));
    await expect(p.model.connect(p.user1).contribute(PROJECT, ethers.parseEther("100")))
      .to.be.revertedWithCustomError(p.model, "UnsupportedTokenBehavior");
    expect((await p.model.getProject(PROJECT)).totalRaised).to.equal(0n);
  });

  it("rejects taxed outgoing refunds and atomically preserves refund flag and reserve", async function () {
    const Outgoing = await ethers.getContractFactory("ModelOutgoingFeeERC20");
    const payment = await Outgoing.deploy();
    const f = await deployFixture({ paymentToken: payment, maxContribution: 0n, softCap: ethers.parseEther("1000") });
    await depositAndActivate(f);
    await time.increaseTo(f.params.startTime);
    const amount = ethers.parseEther("100");
    await payment.mint(f.user1.address, amount);
    await payment.connect(f.user1).approve(await f.model.getAddress(), amount);
    await f.model.connect(f.user1).contribute(PROJECT, amount);
    await time.increaseTo(f.params.endTime);
    await f.model.endProject(PROJECT);
    await expect(f.model.connect(f.user1).refund(PROJECT))
      .to.be.revertedWithCustomError(f.model, "UnsupportedOutgoingTokenBehavior");
    expect((await f.model.getPosition(PROJECT, f.user1.address)).refunded).to.equal(false);
    expect(await f.model.reservedBalance(await payment.getAddress())).to.equal(amount);
    expect(await payment.balanceOf(await f.model.getAddress())).to.equal(amount);
  });

  it("rejects taxed outgoing claims and atomically preserves claimed totals and reserve", async function () {
    const Outgoing = await ethers.getContractFactory("ModelOutgoingFeeERC20");
    const projectToken = await Outgoing.deploy();
    const f = await deployFixture({ projectToken, saleTokenAmount: SALE, softCap: 1n, minContribution: 1n, maxContribution: 0n });
    await depositAndActivate(f);
    await time.increaseTo(f.params.startTime);
    await contribute(f, f.user1, 3n);
    await time.increaseTo(f.params.endTime);
    await f.model.endProject(PROJECT);
    await f.model.acknowledgeSuccessfulReleaseForModel(PROJECT);
    await time.increaseTo(f.vestingTimes[1]);
    await expect(f.model.connect(f.user1).claim(PROJECT))
      .to.be.revertedWithCustomError(f.model, "UnsupportedOutgoingTokenBehavior");
    expect((await f.model.getPosition(PROJECT, f.user1.address)).claimedTokens).to.equal(0n);
    expect((await f.model.getProject(PROJECT)).totalTokensClaimed).to.equal(0n);
    expect(await f.model.reservedBalance(await projectToken.getAddress())).to.equal(SALE);
    expect(await projectToken.balanceOf(await f.model.getAddress())).to.equal(SALE);
  });

  it("fails closed on early end, after-end cancel, contribution in terminal state, and claim before release", async function () {
    const f = await deployFixture({ maxContribution: 0n });
    await depositAndActivate(f);
    await expect(f.model.endProject(PROJECT)).to.be.revertedWithCustomError(f.model, "ProjectNotEnded");
    await time.increaseTo(f.params.startTime);
    await contribute(f, f.user1, SOFT_CAP);
    await time.increaseTo(f.params.endTime);
    await expect(f.model.cancelProject(PROJECT)).to.be.revertedWithCustomError(f.model, "CancellationWindowClosed");
    await f.model.endProject(PROJECT);
    await expect(f.model.connect(f.user1).contribute(PROJECT, MIN))
      .to.be.revertedWithCustomError(f.model, "InvalidProjectStatus");
    await expect(f.model.connect(f.user1).claim(PROJECT))
      .to.be.revertedWithCustomError(f.model, "InvalidProjectStatus");
    await expect(f.model.connect(f.user1).refund(PROJECT))
      .to.be.revertedWithCustomError(f.model, "InvalidProjectStatus");
  });
});
