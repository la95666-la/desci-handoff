'use strict';

const { expect } = require('chai');
const { ethers } = require('hardhat');

describe('FairLaunchSafeExecutor TEST ONLY', function () {
  it('requires two distinct owner approvals and calls as the Safe contract', async function () {
    const [ownerA, ownerB, ownerC, caller] = await ethers.getSigners();
    const Safe = await ethers.getContractFactory('FairLaunchSafeExecutor');
    const Token = await ethers.getContractFactory('FairLaunchTokenMock');
    const safe = await Safe.deploy([ownerA.address, ownerB.address, ownerC.address], 2);
    const token = await Token.deploy('T', 'T', 18);
    await Promise.all([safe.waitForDeployment(), token.waitForDeployment()]);
    const data = token.interface.encodeFunctionData('mint', [await safe.getAddress(), 7n]);
    const salt = ethers.id('safe-test');
    await safe.connect(ownerA).approveTransaction(await token.getAddress(), 0, data, salt);
    await expect(safe.connect(caller).executeTransaction(await token.getAddress(), 0, data, salt))
      .to.be.revertedWithCustomError(safe, 'InsufficientApprovals');
    await expect(safe.connect(ownerA).approveTransaction(await token.getAddress(), 0, data, salt))
      .to.be.revertedWithCustomError(safe, 'AlreadyApproved');
    await safe.connect(ownerB).approveTransaction(await token.getAddress(), 0, data, salt);
    await safe.connect(caller).executeTransaction(await token.getAddress(), 0, data, salt);
    expect(await token.balanceOf(await safe.getAddress())).to.equal(7n);
    await expect(safe.connect(caller).executeTransaction(await token.getAddress(), 0, data, salt))
      .to.be.revertedWithCustomError(safe, 'AlreadyExecuted');
  });

  it('rejects non-owners, duplicate owners, and any threshold other than 2-of-3', async function () {
    const [a, b, c, outsider] = await ethers.getSigners();
    const Safe = await ethers.getContractFactory('FairLaunchSafeExecutor');
    await expect(Safe.deploy([a.address, b.address, c.address], 1)).to.be.revertedWithCustomError(Safe, 'InvalidThreshold');
    await expect(Safe.deploy([a.address, a.address, c.address], 2)).to.be.revertedWithCustomError(Safe, 'DuplicateOwner');
    const safe = await Safe.deploy([a.address, b.address, c.address], 2);
    await safe.waitForDeployment();
    await expect(safe.connect(outsider).approveTransaction(a.address, 0, '0x', ethers.ZeroHash))
      .to.be.revertedWithCustomError(safe, 'NotOwner');
  });
});
