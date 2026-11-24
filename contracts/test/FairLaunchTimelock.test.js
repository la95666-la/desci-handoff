'use strict';

const { expect } = require('chai');
const { ethers } = require('hardhat');
const { EMPTY_CODE_HASH } = require('../scripts/lib/fairLaunchUnsignedDeploymentBundle');
const { INPUT_SCHEMA, SAFE_FALLBACK_HANDLER, SAFE_L2_SINGLETON, SAFE_RUNTIME_CODE_HASH, STATUS, buildUnsignedTimelockBundle, parseJsonNoDuplicateKeys } = require('../scripts/lib/fairLaunchUnsignedTimelockBundle');

const hash = (label) => ethers.id(label).toLowerCase();
function safe(address, owners, label) {
  return { address, runtimeCodeHash: SAFE_RUNTIME_CODE_HASH, owners, threshold: 2, nonce: '0', singleton: SAFE_L2_SINGLETON, fallbackHandler: SAFE_FALLBACK_HANDLER, modules: [], guard: ethers.ZeroAddress, moduleGuard: ethers.ZeroAddress, creationTxHash: hash(`${label}-creation`), creationBlock: label === 'governance' ? '122150762' : '122151949' };
}
async function fixture(startingNonce) {
  const signers = await ethers.getSigners();
  return {
    schemaVersion: INPUT_SCHEMA,
    environment: 'bsc-mainnet',
    chainId: 56,
    changeTicket: 'JC-FL-TIMELOCK-TEST-1',
    observation: { blockNumber: '122158375', blockHash: hash('timelock-observation'), observedAt: '2026-09-16T04:20:00.000Z' },
    deployer: { address: signers[0].address, codeType: 'eoa', runtimeCodeHash: EMPTY_CODE_HASH, startingNonce: String(startingNonce), latestNonce: String(startingNonce), pendingNonce: String(startingNonce), balanceWei: '1000000000000000000', attestationDigest: hash('deployer') },
    safes: { governance: safe(signers[1].address, [signers[3].address, signers[4].address, signers[5].address], 'governance'), emergency: safe(signers[2].address, [signers[3].address, signers[4].address, signers[5].address], 'emergency') },
  };
}
describe('JoinCare Fair Launch Timelocks', function () {
  it('pins the independently observed official BSC Safe proxy runtime identity', function () {
    const observedProxyRuntime = '0x608060405273ffffffffffffffffffffffffffffffffffffffff600054167fa619486e0000000000000000000000000000000000000000000000000000000060003514156050578060005260206000f35b3660008037600080366000845af43d6000803e60008114156070573d6000fd5b3d6000f3fea264697066735822122003d1488ee65e08fa41e58e888a9865554c535f2c77126a82cb4c0f917f31441364736f6c63430007060033';
    expect(ethers.keccak256(observedProxyRuntime)).to.equal(SAFE_RUNTIME_CODE_HASH);
  });
  it('constructs two exact unsigned CREATE transactions and enforces self-admin roles', async function () {
    const [deployer] = await ethers.getSigners();
    const startingNonce = await deployer.getNonce();
    const input = await fixture(startingNonce);
    const bundle = await buildUnsignedTimelockBundle(input);
    expect(bundle.status).to.equal(STATUS);
    expect(bundle.broadcastPermitted).to.equal(false);
    expect(bundle.transactions.map((row) => row.minDelaySeconds)).to.deep.equal(['172800', '3600']);
    expect(bundle.transactions.map((row) => row.nonce)).to.deep.equal([String(startingNonce), String(startingNonce + 1)]);
    const Factory = await ethers.getContractFactory('JoinCareFairLaunchTimelock');
    const governance = await Factory.deploy(172800, [input.safes.governance.address], [input.safes.governance.address]);
    await governance.waitForDeployment();
    const unpause = await Factory.deploy(3600, [input.safes.emergency.address], [input.safes.emergency.address]);
    await unpause.waitForDeployment();
    const deployed = [governance, unpause];
    for (let index = 0; index < deployed.length; index += 1) {
      const contract = deployed[index], transaction = bundle.transactions[index];
      expect(await contract.getAddress()).to.equal(transaction.predictedAddress);
      expect(contract.deploymentTransaction().data).to.equal(transaction.data);
      expect(ethers.keccak256(await ethers.provider.getCode(await contract.getAddress()))).to.equal(transaction.expectedRuntimeCodeHash);
      const controller = index === 0 ? input.safes.governance.address : input.safes.emergency.address;
      expect(await contract.getMinDelay()).to.equal(index === 0 ? 172800n : 3600n);
      expect(await contract.hasRole(bundle.roleIds.defaultAdmin, await contract.getAddress())).to.equal(true);
      expect(await contract.hasRole(bundle.roleIds.defaultAdmin, deployer.address)).to.equal(false);
      expect(await contract.hasRole(bundle.roleIds.proposer, controller)).to.equal(true);
      expect(await contract.hasRole(bundle.roleIds.canceller, controller)).to.equal(true);
      expect(await contract.hasRole(bundle.roleIds.executor, controller)).to.equal(true);
      expect(await contract.hasRole(bundle.roleIds.executor, ethers.ZeroAddress)).to.equal(false);
    }
  });
  it('fails closed on network, nonce, Safe-domain, threshold and code-identity drift', async function () {
    const [deployer] = await ethers.getSigners();
    const base = await fixture(await deployer.getNonce());
    const cases = [
      [(value) => { value.chainId = 97; }, 'timelock network identity invalid'],
      [(value) => { value.deployer.pendingNonce = String(BigInt(value.deployer.pendingNonce) + 1n); }, 'deployer nonce observation mismatch'],
      [(value) => { value.safes.emergency.address = value.safes.governance.address; }, 'governance and emergency Safe domains must be distinct'],
      [(value) => { value.safes.governance.threshold = 1; }, 'governance Safe threshold must be 2'],
      [(value) => { value.safes.emergency.runtimeCodeHash = EMPTY_CODE_HASH; }, 'emergency Safe runtime code identity invalid'],
      [(value) => { value.safes.emergency.singleton = value.safes.emergency.address; }, 'emergency Safe singleton identity invalid'],
      [(value) => { value.safes.governance.modules = [value.safes.emergency.address]; }, 'governance Safe modules invalid'],
      [(value) => { value.safes.emergency.owners[2] = value.safes.governance.address; }, 'governance and emergency Safe owner sets must match'],
      [(value) => { value.observation.blockNumber = value.safes.emergency.creationBlock; }, 'emergency Safe lacks 20-block finality'],
    ];
    for (const [mutate, message] of cases) {
      const changed = structuredClone(base); mutate(changed);
      await expect(buildUnsignedTimelockBundle(changed)).to.be.rejectedWith(message);
    }
  });
  it('rejects duplicate and unsafe JSON before authority validation', function () {
    expect(() => parseJsonNoDuplicateKeys('{"chainId":56,"chainId":97}', 'timelock input')).to.throw('duplicate field: chainId');
    expect(() => parseJsonNoDuplicateKeys('{"constructor":{}}', 'timelock input')).to.throw('unsafe field: constructor');
  });
});
