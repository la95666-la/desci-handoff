const { expect } = require('chai');
const { ethers, upgrades } = require('hardhat');
const { time } = require('@nomicfoundation/hardhat-network-helpers');
const { buildTree, leaf } = require('../scripts/lib/idoV3Whitelist');

describe('JoinCareIDOv3 independent whitelist prototype', function () {
  it('keeps the pinned project reader ABI shape and selectors stable', async () => {
    const artifact = await ethers.getContractFactory('JoinCareIDOv3');
    const method = artifact.interface.getFunction('getProject(uint256)');
    expect(method.selector).to.equal(ethers.id('getProject(uint256)').slice(0, 10));
    expect(method.outputs).to.have.length(1);
    const components = method.outputs[0].components;
    expect(components.map(({ name, type }) => `${name}:${type}`)).to.deep.equal([
      'projectId:uint256', 'projectToken:address', 'paymentToken:address', 'projectOwner:address',
      'feeReceiver:address', 'projectTokenDecimals:uint8', 'paymentTokenDecimals:uint8',
      'successFeeRate:uint16', 'status:uint8', 'tokenPrice:uint256', 'hardCap:uint256',
      'softCap:uint256', 'maxPerUser:uint256', 'startTime:uint256', 'endTime:uint256',
      'totalRaised:uint256', 'totalTokensSold:uint256', 'totalTokensDeposited:uint256',
      'fundsWithdrawn:bool', 'platformFeeWithdrawn:bool', 'projectTokensWithdrawn:bool',
      'vestingTimes:uint256[]', 'vestingPercents:uint256[]',
    ]);
    expect(artifact.interface.getFunction('owner()').selector).to.equal(ethers.id('owner()').slice(0, 10));
    expect(artifact.interface.getFunction('whitelistGovernor()').selector)
      .to.equal(ethers.id('whitelistGovernor()').slice(0, 10));
  });

  async function fixture() {
    const [deployer, signerA, signerB, signerC, operator, buyer, outsider, projectOwner, feeReceiver] = await ethers.getSigners();
    const Governor = await ethers.getContractFactory('IDOv3TwoOfThreeGovernorMock');
    const governor = await Governor.deploy([signerA.address, signerB.address, signerC.address]);
    const Token = await ethers.getContractFactory('MockUSDT');
    const payment = await Token.deploy(ethers.parseUnits('1000000', 6), ethers.parseUnits('10000000', 6), deployer.address);
    const Sale = await ethers.getContractFactory('TestProjectToken');
    const sale = await Sale.deploy('V3 Token', 'V3T', ethers.parseEther('1000000'), ethers.parseEther('10000000'), projectOwner.address);
    const V2 = await ethers.getContractFactory('JoinCareIDOv2');
    const v2 = await upgrades.deployProxy(V2, [deployer.address, operator.address, feeReceiver.address, 200], { kind: 'uups' });
    const V3 = await ethers.getContractFactory('JoinCareIDOv3');
    const v3 = await upgrades.deployProxy(V3, [governor.target, operator.address, feeReceiver.address, 200, governor.target], { kind: 'uups' });
    await v3.waitForDeployment();
    // The governor is not the day-to-day project operator; token configuration
    // calls are also governed on this prototype proxy.
    async function governed(data, target = v3.target) {
      const id = (await governor.nextProposal()) + 1n;
      await governor.connect(signerA).propose(target, data);
      await expect(governor.connect(signerA).execute(id)).to.be.revertedWith('threshold not met');
      await governor.connect(signerB).approve(id);
      await governor.connect(signerC).execute(id);
    }
    await governed(v3.interface.encodeFunctionData('configurePaymentToken', [payment.target, true]));
    await governed(v3.interface.encodeFunctionData('configureProjectToken', [sale.target, true]));
    await payment.transfer(buyer.address, ethers.parseUnits('1000', 6));
    await payment.transfer(outsider.address, ethers.parseUnits('1000', 6));
    async function create(id) {
      const start = (await time.latest()) + 1000;
      const end = start + 1000;
      await v3.connect(operator).createProject(id, sale.target, payment.target, projectOwner.address,
        feeReceiver.address, 200, ethers.parseUnits('2', 6), ethers.parseUnits('1000', 6),
        ethers.parseUnits('500', 6), ethers.parseUnits('600', 6), start, end, [end + 100], [10000]);
      await sale.connect(projectOwner).approve(v3.target, ethers.parseEther('1000'));
      await v3.connect(projectOwner).depositProjectTokens(id, ethers.parseEther('1000'));
      return { start, end };
    }
    async function policy(id, version, enabled, root) {
      return governed(v3.interface.encodeFunctionData('setWhitelistPolicy', [id, version, enabled, root]));
    }
    return { v2, v3, governor, signerA, signerB, operator, buyer, outsider, projectOwner, feeReceiver, payment, sale, create, policy, governed };
  }

  it('requires contract-based owner and policy governor and leaves V2 a distinct proxy', async () => {
    const f = await fixture();
    expect(f.v2.target).not.to.equal(f.v3.target);
    expect(await f.v3.contractVersion()).to.equal(ethers.id('joincare.ido_v3_stablecoin.v1'));
    expect(await f.v3.owner()).to.equal(f.governor.target);
    expect(await f.v2.owner()).not.to.equal(f.v3.owner());
    await expect(f.governor.connect(f.signerA).propose(f.v3.target,
      f.v3.interface.encodeFunctionData('transferOwnership', [f.operator.address]))).not.to.be.reverted;
    await f.governor.connect(f.signerB).approve(await f.governor.nextProposal());
    await expect(f.governor.connect(f.signerB).execute(await f.governor.nextProposal()))
      .to.be.revertedWithCustomError(f.v3, 'GovernanceContractRequired');
    const V3 = await ethers.getContractFactory('JoinCareIDOv3');
    await expect(upgrades.deployProxy(V3,
      [f.operator.address, f.operator.address, f.operator.address, 100, f.governor.target],
      { kind: 'uups' })).to.be.revertedWithCustomError(f.v3, 'GovernanceContractRequired');
  });

  it('prevents non-governors and the day-to-day operator from changing the root', async () => {
    const f = await fixture();
    await f.create(1);
    const root = await f.v3.whitelistLeaf(1, f.buyer.address);
    await expect(f.v3.connect(f.operator).setWhitelistPolicy(1, 0, true, root))
      .to.be.revertedWithCustomError(f.v3, 'UnauthorizedWhitelistGovernor');
    await expect(f.v3.connect(f.signerA).setWhitelistPolicy(1, 0, true, root))
      .to.be.revertedWithCustomError(f.v3, 'UnauthorizedWhitelistGovernor');
    await f.policy(1, 0, true, root);
    const current = await f.v3.getWhitelistPolicy(1);
    expect(current.enabled).to.equal(true);
    expect(current.root).to.equal(root);
    expect(current.version).to.equal(1n);
    await expect(f.policy(1, 0, true, root))
      .to.be.revertedWithCustomError(f.v3, 'WhitelistVersionChanged');
    await expect(f.policy(1, 1, true, ethers.ZeroHash))
      .to.be.revertedWithCustomError(f.v3, 'InvalidWhitelistPolicy');
  });

  it('freezes a closed Pending project mode at activation and disallows late first enrollment', async () => {
    const f = await fixture();
    const { start } = await f.create(24);
    const root = await f.v3.whitelistLeaf(24, f.buyer.address);
    await f.policy(24, 0, true, root);
    await f.policy(24, 1, false, ethers.ZeroHash);
    expect((await f.v3.getWhitelistPolicy(24)).version).to.equal(2n);
    await f.v3.connect(f.operator).activateProject(24);
    await expect(f.policy(24, 2, true, root)).to.be.revertedWithCustomError(f.v3, 'WhitelistModeFrozen');
    await time.increaseTo(start);
    const amount = ethers.parseUnits('12', 6);
    await f.payment.connect(f.buyer).approve(f.v3.target, amount);
    await expect(f.v3.connect(f.buyer).purchaseWithProof(24, amount, 2, []))
      .to.be.revertedWithCustomError(f.v3, 'WhitelistNotEnabled');
    await expect(f.v3.connect(f.buyer).purchase(24, amount)).to.emit(f.v3, 'Purchased');
  });

  it('emits a reconstructable versioned policy event only after governed execution', async () => {
    const f = await fixture();
    await f.create(14);
    const root = await f.v3.whitelistLeaf(14, f.buyer.address);
    await f.policy(14, 0, true, root);
    const filter = f.v3.filters.WhitelistPolicyUpdated(14n, 1n);
    const events = await f.v3.queryFilter(filter);
    expect(events.length).to.equal(1);
    const event = events[0];
    expect(event.address).to.equal(f.v3.target);
    expect(event.topics.length).to.equal(3);
    expect(event.topics[0]).to.equal(ethers.id('WhitelistPolicyUpdated(uint256,uint64,bool,bytes32)'));
    expect(event.topics[1]).to.equal(ethers.zeroPadValue(ethers.toBeHex(14n), 32));
    expect(event.topics[2]).to.equal(ethers.zeroPadValue(ethers.toBeHex(1n), 32));
    expect(event.data).to.equal(ethers.AbiCoder.defaultAbiCoder().encode(['bool', 'bytes32'], [true, root]));
  });

  it('closes every proof bypass and permits an approved member only at the current version', async () => {
    const f = await fixture();
    const { start } = await f.create(2);
    const root = await f.v3.whitelistLeaf(2, f.buyer.address);
    await f.policy(2, 0, true, root);
    await f.v3.connect(f.operator).activateProject(2);
    await time.increaseTo(start);
    const amount = ethers.parseUnits('100', 6);
    await f.payment.connect(f.buyer).approve(f.v3.target, amount);
    await f.payment.connect(f.outsider).approve(f.v3.target, amount);
    await expect(f.v3.connect(f.buyer).purchase(2, amount))
      .to.be.revertedWithCustomError(f.v3, 'WhitelistProofRequired');
    await expect(f.v3.connect(f.outsider).purchaseWithProof(2, amount, 1, []))
      .to.be.revertedWithCustomError(f.v3, 'WhitelistProofInvalid');
    await expect(f.v3.connect(f.buyer).purchaseWithProof(2, amount, 0, []))
      .to.be.revertedWithCustomError(f.v3, 'WhitelistVersionChanged');
    await expect(f.v3.connect(f.buyer).purchaseWithProof(2, amount, 1, []))
      .to.emit(f.v3, 'Purchased');
    expect((await f.v3.getUserPurchase(2, f.buyer.address)).paymentAmount).to.equal(amount);
  });

  it('freezes enable/disable at Active but permits governed membership replacement', async () => {
    const f = await fixture();
    const { start } = await f.create(3);
    const buyerRoot = await f.v3.whitelistLeaf(3, f.buyer.address);
    await f.policy(3, 0, true, buyerRoot);
    await f.v3.connect(f.operator).activateProject(3);
    await time.increaseTo(start);
    const amount = ethers.parseUnits('25', 6);
    await f.payment.connect(f.buyer).approve(f.v3.target, amount);
    await f.payment.connect(f.outsider).approve(f.v3.target, amount);
    await f.v3.connect(f.buyer).purchaseWithProof(3, amount, 1, []);
    await expect(f.policy(3, 1, false, ethers.ZeroHash))
      .to.be.revertedWithCustomError(f.v3, 'WhitelistModeFrozen');
    const outsiderRoot = await f.v3.whitelistLeaf(3, f.outsider.address);
    await f.policy(3, 1, true, outsiderRoot);
    expect((await f.v3.getWhitelistPolicy(3)).version).to.equal(2n);
    await expect(f.v3.connect(f.buyer).purchaseWithProof(3, amount, 1, []))
      .to.be.revertedWithCustomError(f.v3, 'WhitelistVersionChanged');
    await expect(f.v3.connect(f.buyer).purchaseWithProof(3, amount, 2, []))
      .to.be.revertedWithCustomError(f.v3, 'WhitelistProofInvalid');
    await expect(f.v3.connect(f.outsider).purchaseWithProof(3, amount, 2, []))
      .to.emit(f.v3, 'Purchased');
    expect((await f.v3.getUserPurchase(3, f.buyer.address)).paymentAmount).to.equal(amount);
    await f.create(4);
    await f.v3.connect(f.operator).activateProject(4);
    await expect(f.policy(4, 0, true, await f.v3.whitelistLeaf(4, f.buyer.address)))
      .to.be.revertedWithCustomError(f.v3, 'WhitelistModeFrozen');
  });

  it('permits Pending disable, public purchase, and preserves independent V2 identity', async () => {
    const f = await fixture();
    const { start } = await f.create(5);
    await f.policy(5, 0, true, await f.v3.whitelistLeaf(5, f.buyer.address));
    await f.policy(5, 1, false, ethers.ZeroHash);
    await f.v3.connect(f.operator).activateProject(5);
    await time.increaseTo(start);
    const amount = ethers.parseUnits('10', 6);
    await f.payment.connect(f.outsider).approve(f.v3.target, amount);
    await expect(f.v3.connect(f.outsider).purchase(5, amount)).to.emit(f.v3, 'Purchased');
    await expect(f.v3.connect(f.outsider).purchaseWithProof(5, amount, 2, []))
      .to.be.revertedWithCustomError(f.v3, 'WhitelistNotEnabled');
    expect((await f.v2.getProject(5)).projectId).to.equal(0n);
  });

  it('keeps an already purchased wallet refundable after removal and freezes terminal policy', async () => {
    const f = await fixture();
    const { start, end } = await f.create(7);
    await f.policy(7, 0, true, await f.v3.whitelistLeaf(7, f.buyer.address));
    await f.v3.connect(f.operator).activateProject(7);
    await time.increaseTo(start);
    const amount = ethers.parseUnits('30', 6);
    await f.payment.connect(f.buyer).approve(f.v3.target, amount);
    await f.v3.connect(f.buyer).purchaseWithProof(7, amount, 1, []);
    await f.policy(7, 1, true, await f.v3.whitelistLeaf(7, f.outsider.address));
    await time.increaseTo(end);
    await f.v3.endProject(7);
    await expect(f.policy(7, 2, true, await f.v3.whitelistLeaf(7, f.buyer.address)))
      .to.be.revertedWithCustomError(f.v3, 'InvalidProjectStatus');
    await expect(f.v3.connect(f.buyer).refund(7)).to.emit(f.v3, 'Refunded');
    expect((await f.v3.getUserPurchase(7, f.buyer.address)).refunded).to.equal(true);
  });

  it('retains purchased positions after removal across success, vesting claims and independent fee exits', async () => {
    const f = await fixture();
    const { start, end } = await f.create(8);
    const amount = ethers.parseUnits('500', 6); // reaches the soft cap, but not hard cap
    await f.policy(8, 0, true, await f.v3.whitelistLeaf(8, f.buyer.address));
    await f.v3.connect(f.operator).activateProject(8);
    await time.increaseTo(start);
    await f.payment.connect(f.buyer).approve(f.v3.target, amount);
    await f.v3.connect(f.buyer).purchaseWithProof(8, amount, 1, []);
    await f.policy(8, 1, true, await f.v3.whitelistLeaf(8, f.outsider.address));
    await time.increaseTo(end);
    await f.v3.endProject(8);
    expect((await f.v3.getProject(8)).status).to.equal(2n); // Successful
    await expect(f.v3.connect(f.buyer).refund(8)).to.be.revertedWithCustomError(f.v3, 'InvalidProjectStatus');
    const ownerBefore = await f.payment.balanceOf(f.projectOwner.address);
    const feeBefore = await f.payment.balanceOf(f.feeReceiver.address);
    await f.v3.connect(f.projectOwner).withdrawRaisedFunds(8);
    await f.v3.withdrawPlatformFee(8);
    expect((await f.payment.balanceOf(f.projectOwner.address)) - ownerBefore).to.equal(ethers.parseUnits('490', 6));
    expect((await f.payment.balanceOf(f.feeReceiver.address)) - feeBefore).to.equal(ethers.parseUnits('10', 6));
    await time.increaseTo(end + 100);
    const saleBefore = await f.sale.balanceOf(f.buyer.address);
    await f.v3.connect(f.buyer).claim(8);
    expect((await f.sale.balanceOf(f.buyer.address)) - saleBefore).to.equal(ethers.parseEther('250'));
    expect(await f.v3.reservedBalances(f.payment.target)).to.equal(0n);
    await expect(f.policy(8, 2, true, await f.v3.whitelistLeaf(8, f.buyer.address)))
      .to.be.revertedWithCustomError(f.v3, 'InvalidProjectStatus');
  });

  it('cannot use paused policy updates as a way to revoke an Active whitelist', async () => {
    const f = await fixture();
    await f.create(9);
    await f.policy(9, 0, true, await f.v3.whitelistLeaf(9, f.buyer.address));
    await f.v3.connect(f.operator).activateProject(9);
    await f.governed(f.v3.interface.encodeFunctionData('pause'));
    await expect(f.policy(9, 1, false, ethers.ZeroHash))
      .to.be.revertedWithCustomError(f.v3, 'WhitelistModeFrozen');
    await f.policy(9, 1, true, await f.v3.whitelistLeaf(9, f.outsider.address));
    expect((await f.v3.getWhitelistPolicy(9)).version).to.equal(2n);
    await f.governed(f.v3.interface.encodeFunctionData('unpause'));
    await expect(f.v3.connect(f.buyer).purchaseWithProof(9, 1, 2, []))
      .to.be.revertedWithCustomError(f.v3, 'WhitelistProofInvalid');
  });

  it('rejects project and proxy replay of an otherwise valid member proof', async () => {
    const f = await fixture();
    const a = await f.create(10);
    const b = await f.create(11);
    const chainId = (await ethers.provider.getNetwork()).chainId;
    const aScope = { chainId, proxyAddress: f.v3.target, projectId: 10n };
    const tree = buildTree(aScope, [f.buyer.address, f.outsider.address]);
    await f.policy(10, 0, true, tree.root);
    // Even a governed mistaken reuse of another project's root cannot admit
    // its members at project 11: whitelistLeaf always binds the actual ID.
    await f.policy(11, 0, true, tree.root);
    await f.v3.connect(f.operator).activateProject(10);
    await f.v3.connect(f.operator).activateProject(11);
    await time.increaseTo(Math.max(a.start, b.start));
    const amount = ethers.parseUnits('10', 6);
    await f.payment.connect(f.buyer).approve(f.v3.target, amount * 2n);
    const proof = tree.proofFor(f.buyer.address);
    await expect(f.v3.connect(f.buyer).purchaseWithProof(10, amount, 1, proof))
      .to.emit(f.v3, 'Purchased');
    await expect(f.v3.connect(f.buyer).purchaseWithProof(11, amount, 1, proof))
      .to.be.revertedWithCustomError(f.v3, 'WhitelistProofInvalid');

    const V3 = await ethers.getContractFactory('JoinCareIDOv3');
    const otherProxy = await upgrades.deployProxy(V3,
      [f.governor.target, f.operator.address, f.feeReceiver.address, 200, f.governor.target], { kind: 'uups' });
    await otherProxy.waitForDeployment();
    expect(leaf({ ...aScope, proxyAddress: otherProxy.target }, f.buyer.address))
      .not.to.equal(await f.v3.whitelistLeaf(10, f.buyer.address));
    expect(leaf({ ...aScope, chainId: chainId + 1n }, f.buyer.address))
      .not.to.equal(await f.v3.whitelistLeaf(10, f.buyer.address));
  });

  it('isolates shared payment reserves when a whitelist project is cancelled and another project fails', async () => {
    const f = await fixture();
    const first = await f.create(12);
    const second = await f.create(13);
    await f.policy(12, 0, true, await f.v3.whitelistLeaf(12, f.buyer.address));
    await f.v3.connect(f.operator).activateProject(12);
    await f.v3.connect(f.operator).activateProject(13);
    await time.increaseTo(Math.max(first.start, second.start));
    const a = ethers.parseUnits('100', 6);
    const b = ethers.parseUnits('80', 6);
    await f.payment.connect(f.buyer).approve(f.v3.target, a);
    await f.payment.connect(f.outsider).approve(f.v3.target, b);
    await f.v3.connect(f.buyer).purchaseWithProof(12, a, 1, []);
    await f.v3.connect(f.outsider).purchase(13, b);
    expect(await f.v3.reservedBalances(f.payment.target)).to.equal(a + b);
    await f.governed(f.v3.interface.encodeFunctionData('cancelProject', [12]));
    await expect(f.v3.connect(f.buyer).purchaseWithProof(12, a, 1, []))
      .to.be.revertedWithCustomError(f.v3, 'InvalidProjectStatus');
    await f.v3.connect(f.buyer).refund(12);
    expect(await f.v3.reservedBalances(f.payment.target)).to.equal(b);
    expect(await f.payment.balanceOf(f.v3.target)).to.equal(b);
    await time.increaseTo(Math.max(first.end, second.end));
    await f.v3.endProject(13);
    expect((await f.v3.getProject(13)).status).to.equal(3n); // Failed
    await f.v3.connect(f.outsider).refund(13);
    expect(await f.v3.reservedBalances(f.payment.target)).to.equal(0n);
    expect(await f.payment.balanceOf(f.v3.target)).to.equal(0n);
  });

  it('rejects fee-on-transfer deposits and purchases without changing whitelist accounting', async () => {
    const f = await fixture();
    const Token = await ethers.getContractFactory('MockFeeOnTransferToken');
    const feeToken = await Token.deploy(1000000);
    await f.governed(f.v3.interface.encodeFunctionData('configurePaymentToken', [feeToken.target, true]));
    const start = (await time.latest()) + 1000;
    const end = start + 1000;
    await f.v3.connect(f.operator).createProject(15, f.sale.target, feeToken.target, f.projectOwner.address,
      f.feeReceiver.address, 200, ethers.parseEther('2'), ethers.parseEther('1000'),
      ethers.parseEther('500'), ethers.parseEther('600'), start, end, [end + 100], [10000]);
    await f.sale.connect(f.projectOwner).approve(f.v3.target, ethers.parseEther('1000'));
    await f.v3.connect(f.projectOwner).depositProjectTokens(15, ethers.parseEther('1000'));
    await f.policy(15, 0, true, await f.v3.whitelistLeaf(15, f.buyer.address));
    await f.v3.connect(f.operator).activateProject(15);
    await feeToken.mint(f.buyer.address, ethers.parseEther('100'));
    await time.increaseTo(start);
    const amount = ethers.parseEther('20');
    await feeToken.connect(f.buyer).approve(f.v3.target, amount);
    await expect(f.v3.connect(f.buyer).purchaseWithProof(15, amount, 1, []))
      .to.be.revertedWithCustomError(f.v3, 'UnsupportedTokenBehavior');
    expect((await f.v3.getProject(15)).totalRaised).to.equal(0n);
    expect((await f.v3.getUserPurchase(15, f.buyer.address)).paymentAmount).to.equal(0n);
    expect(await f.v3.reservedBalances(feeToken.target)).to.equal(0n);

    const taxedSale = await Token.deploy(1000000);
    await f.governed(f.v3.interface.encodeFunctionData('configureProjectToken', [taxedSale.target, true]));
    const nextStart = (await time.latest()) + 1000;
    const nextEnd = nextStart + 1000;
    await f.v3.connect(f.operator).createProject(16, taxedSale.target, f.payment.target, f.projectOwner.address,
      f.feeReceiver.address, 200, ethers.parseUnits('2', 6), ethers.parseUnits('1000', 6),
      ethers.parseUnits('500', 6), ethers.parseUnits('600', 6), nextStart, nextEnd, [nextEnd + 100], [10000]);
    await taxedSale.mint(f.projectOwner.address, ethers.parseEther('100'));
    await taxedSale.connect(f.projectOwner).approve(f.v3.target, ethers.parseEther('100'));
    await expect(f.v3.connect(f.projectOwner).depositProjectTokens(16, ethers.parseEther('100')))
      .to.be.revertedWithCustomError(f.v3, 'UnsupportedTokenBehavior');
    expect((await f.v3.getProject(16)).totalTokensDeposited).to.equal(0n);
  });

  it('does not drain pooled refunds when a listed payment asset takes an outgoing fee', async () => {
    const f = await fixture();
    const Token = await ethers.getContractFactory('MockOutgoingFeeToken');
    const payment = await Token.deploy('V3 outgoing fee USD', 'V3O', 6);
    await f.governed(f.v3.interface.encodeFunctionData('configurePaymentToken', [payment.target, true]));
    const start = (await time.latest()) + 1000;
    const end = start + 1000;
    for (const id of [17, 18]) {
      await f.v3.connect(f.operator).createProject(id, f.sale.target, payment.target, f.projectOwner.address,
        f.feeReceiver.address, 200, ethers.parseUnits('2', 6), ethers.parseUnits('1000', 6),
        ethers.parseUnits('500', 6), ethers.parseUnits('600', 6), start, end, [end + 100], [10000]);
      await f.sale.connect(f.projectOwner).approve(f.v3.target, ethers.parseEther('1000'));
      await f.v3.connect(f.projectOwner).depositProjectTokens(id, ethers.parseEther('1000'));
    }
    await f.policy(17, 0, true, await f.v3.whitelistLeaf(17, f.buyer.address));
    await f.v3.connect(f.operator).activateProject(17);
    await f.v3.connect(f.operator).activateProject(18);
    const amount = ethers.parseUnits('100', 6);
    await payment.mint(f.buyer.address, amount);
    await payment.mint(f.outsider.address, amount);
    await time.increaseTo(start);
    await payment.connect(f.buyer).approve(f.v3.target, amount);
    await payment.connect(f.outsider).approve(f.v3.target, amount);
    await f.v3.connect(f.buyer).purchaseWithProof(17, amount, 1, []);
    await f.v3.connect(f.outsider).purchase(18, amount);
    await time.increaseTo(end);
    await f.v3.endProject(17);
    await f.v3.endProject(18);
    await expect(f.v3.connect(f.buyer).refund(17))
      .to.be.revertedWithCustomError(f.v3, 'UnsupportedOutgoingTokenBehavior');
    expect(await f.v3.reservedBalances(payment.target)).to.equal(amount * 2n);
    expect(await payment.balanceOf(f.v3.target)).to.equal(amount * 2n);
    expect((await f.v3.getUserPurchase(17, f.buyer.address)).refunded).to.equal(false);
    expect((await f.v3.getProject(18)).totalRaised).to.equal(amount);
  });

  it('does not spend another project reserve after a negative payment-token rebase', async () => {
    const f = await fixture();
    const Token = await ethers.getContractFactory('MockNegativeRebaseToken');
    const payment = await Token.deploy();
    await f.governed(f.v3.interface.encodeFunctionData('configurePaymentToken', [payment.target, true]));
    const start = (await time.latest()) + 1000;
    const end = start + 1000;
    for (const id of [19, 20]) {
      await f.v3.connect(f.operator).createProject(id, f.sale.target, payment.target, f.projectOwner.address,
        f.feeReceiver.address, 200, ethers.parseEther('2'), ethers.parseEther('1000'),
        ethers.parseEther('500'), ethers.parseEther('600'), start, end, [end + 100], [10000]);
      await f.sale.connect(f.projectOwner).approve(f.v3.target, ethers.parseEther('1000'));
      await f.v3.connect(f.projectOwner).depositProjectTokens(id, ethers.parseEther('1000'));
    }
    await f.policy(19, 0, true, await f.v3.whitelistLeaf(19, f.buyer.address));
    await f.v3.connect(f.operator).activateProject(19);
    await f.v3.connect(f.operator).activateProject(20);
    const amount = ethers.parseEther('100');
    await payment.transfer(f.buyer.address, amount);
    await payment.transfer(f.outsider.address, amount);
    await time.increaseTo(start);
    await payment.connect(f.buyer).approve(f.v3.target, amount);
    await payment.connect(f.outsider).approve(f.v3.target, amount);
    await f.v3.connect(f.buyer).purchaseWithProof(19, amount, 1, []);
    await f.v3.connect(f.outsider).purchase(20, amount);
    await time.increaseTo(end);
    await f.v3.endProject(19);
    await f.v3.endProject(20);
    await payment.slash(f.v3.target, ethers.parseEther('50'));
    await expect(f.v3.connect(f.buyer).refund(19))
      .to.be.revertedWithCustomError(f.v3, 'TokenReserveInsolvent')
      .withArgs(payment.target, ethers.parseEther('150'), ethers.parseEther('200'));
    expect((await f.v3.getUserPurchase(19, f.buyer.address)).refunded).to.equal(false);
    expect(await f.v3.reservedBalances(payment.target)).to.equal(ethers.parseEther('200'));
  });

  it('keeps creation-time fee recipient and rate after governed global fee changes', async () => {
    const f = await fixture();
    const { start, end } = await f.create(21);
    await f.policy(21, 0, true, await f.v3.whitelistLeaf(21, f.buyer.address));
    const nextReceiver = f.outsider.address;
    await f.governed(f.v3.interface.encodeFunctionData('setPlatformFees', [500, nextReceiver]));
    await f.v3.connect(f.operator).activateProject(21);
    await time.increaseTo(start);
    const amount = ethers.parseUnits('500', 6);
    await f.payment.connect(f.buyer).approve(f.v3.target, amount);
    await f.v3.connect(f.buyer).purchaseWithProof(21, amount, 1, []);
    await time.increaseTo(end);
    await f.v3.endProject(21);
    const initialFee = await f.payment.balanceOf(f.feeReceiver.address);
    const nextFee = await f.payment.balanceOf(nextReceiver);
    await f.v3.connect(f.projectOwner).withdrawRaisedFunds(21);
    await f.v3.withdrawPlatformFee(21);
    expect((await f.v3.getProject(21)).successFeeRate).to.equal(200n);
    expect((await f.payment.balanceOf(f.feeReceiver.address)) - initialFee).to.equal(ethers.parseUnits('10', 6));
    expect((await f.payment.balanceOf(nextReceiver)) - nextFee).to.equal(0n);
    await expect(f.v3.withdrawPlatformFee(21)).to.be.revertedWithCustomError(f.v3, 'AlreadyWithdrawn');
  });

  it('requires contract owner authority to upgrade and retains policy and positions in a compatible UUPS implementation', async () => {
    const f = await fixture();
    const { start } = await f.create(22);
    const root = await f.v3.whitelistLeaf(22, f.buyer.address);
    await f.policy(22, 0, true, root);
    await f.v3.connect(f.operator).activateProject(22);
    await time.increaseTo(start);
    const amount = ethers.parseUnits('25', 6);
    await f.payment.connect(f.buyer).approve(f.v3.target, amount);
    await f.v3.connect(f.buyer).purchaseWithProof(22, amount, 1, []);
    const Next = await ethers.getContractFactory('JoinCareIDOv3UpgradeMock');
    const nextImplementation = await Next.deploy();
    await expect(f.v3.connect(f.operator).upgradeToAndCall(nextImplementation.target, '0x'))
      .to.be.revertedWithCustomError(f.v3, 'OwnableUnauthorizedAccount');
    await expect(f.v3.connect(f.signerA).upgradeToAndCall(nextImplementation.target, '0x'))
      .to.be.revertedWithCustomError(f.v3, 'OwnableUnauthorizedAccount');
    await f.governed(f.v3.interface.encodeFunctionData('upgradeToAndCall', [nextImplementation.target, '0x']));
    const upgraded = Next.attach(f.v3.target);
    expect(await upgraded.implementationVersion()).to.equal(2n);
    expect(await upgraded.contractVersion()).to.equal(ethers.id('joincare.ido_v3_stablecoin.v1'));
    expect(await upgraded.owner()).to.equal(f.governor.target);
    expect((await upgraded.getWhitelistPolicy(22)).root).to.equal(root);
    expect((await upgraded.getWhitelistPolicy(22)).version).to.equal(1n);
    expect((await upgraded.getUserPurchase(22, f.buyer.address)).paymentAmount).to.equal(amount);
    expect((await upgraded.getProject(22)).totalRaised).to.equal(amount);
  });

  it('binds proof leaves to chain, proxy, project and buyer; accepts a sorted two-leaf proof', async () => {
    const f = await fixture();
    const { start } = await f.create(6);
    const chainId = (await ethers.provider.getNetwork()).chainId;
    const scope = { chainId, proxyAddress: f.v3.target, projectId: 6n };
    const tree = buildTree(scope, [f.outsider.address, f.buyer.address]);
    const a = leaf(scope, f.buyer.address);
    const b = leaf(scope, f.outsider.address);
    expect(a).to.equal(await f.v3.whitelistLeaf(6, f.buyer.address));
    await f.policy(6, 0, true, tree.root);
    await f.v3.connect(f.operator).activateProject(6);
    await time.increaseTo(start);
    const amount = ethers.parseUnits('20', 6);
    await f.payment.connect(f.buyer).approve(f.v3.target, amount);
    await f.payment.connect(f.outsider).approve(f.v3.target, amount);
    await expect(f.v3.connect(f.buyer).purchaseWithProof(6, amount, 1, tree.proofFor(f.buyer.address))).to.emit(f.v3, 'Purchased');
    await expect(f.v3.connect(f.outsider).purchaseWithProof(6, amount, 1, tree.proofFor(f.outsider.address))).to.emit(f.v3, 'Purchased');
    const schema = await f.v3.WHITELIST_LEAF_SCHEMA();
    const coder = ethers.AbiCoder.defaultAbiCoder();
    expect(a).to.equal(ethers.keccak256(coder.encode(['bytes32', 'uint256', 'address', 'uint256', 'address'],
      [schema, chainId, f.v3.target, 6n, f.buyer.address])));
    expect(a).not.to.equal(ethers.keccak256(coder.encode(['bytes32', 'uint256', 'address', 'uint256', 'address'],
      [schema, chainId, f.v2.target, 6n, f.buyer.address])));
    expect(a).not.to.equal(await f.v3.whitelistLeaf(5, f.buyer.address));
    expect(a).not.to.equal(await f.v3.whitelistLeaf(6, f.outsider.address));
  });

  it('verifies three-leaf odd promotion against the deployed contract, not just local hashing', async () => {
    const f = await fixture();
    const { start } = await f.create(8);
    const chainId = (await ethers.provider.getNetwork()).chainId;
    const scope = { chainId, proxyAddress: f.v3.target, projectId: 8n };
    const third = f.projectOwner.address;
    const tree = buildTree(scope, [f.outsider.address, third, f.buyer.address]);
    expect(tree.leaves.length).to.equal(3);
    await f.policy(8, 0, true, tree.root);
    await f.v3.connect(f.operator).activateProject(8);
    await time.increaseTo(start);
    const amount = ethers.parseUnits('12', 6);
    for (const actor of [f.buyer, f.outsider]) {
      await f.payment.connect(actor).approve(f.v3.target, amount);
      await expect(f.v3.connect(actor).purchaseWithProof(8, amount, 1, tree.proofFor(actor.address)))
        .to.emit(f.v3, 'Purchased');
    }
    await f.payment.transfer(third, amount);
    await f.payment.connect(f.projectOwner).approve(f.v3.target, amount);
    await expect(f.v3.connect(f.projectOwner).purchaseWithProof(8, amount, 1, tree.proofFor(third)))
      .to.emit(f.v3, 'Purchased');
  });
});
