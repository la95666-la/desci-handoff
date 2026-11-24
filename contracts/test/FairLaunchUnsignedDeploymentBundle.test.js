'use strict';

const { expect } = require('chai');
const { ethers } = require('hardhat');
const { aggregateInterface, deployFacetSet, manifest } = require('../scripts/fairLaunchArchitecture');
const { EMPTY_CODE_HASH, INPUT_SCHEMA, STATUS, buildUnsignedDeploymentBundle, parseJsonNoDuplicateKeys } = require('../scripts/lib/fairLaunchUnsignedDeploymentBundle');

const HASH = (label) => ethers.id(label).toLowerCase();
function contractRole(address, authorityKind, label) {
  return { address, authorityKind, codeType: 'contract', runtimeCodeHash: HASH(`${label}-runtime`), attestationDigest: HASH(`${label}-attestation`), attestationVersion: 1 };
}
function eoaRole(address, authorityKind, label) {
  return { address, authorityKind, codeType: 'eoa', runtimeCodeHash: EMPTY_CODE_HASH, attestationDigest: HASH(`${label}-attestation`), attestationVersion: 1 };
}
async function fixture(startingNonce) {
  const signers = await ethers.getSigners();
  return {
    schemaVersion: INPUT_SCHEMA,
    environment: 'bsc-mainnet',
    chainId: 56,
    changeTicket: 'JC-FL-MAINNET-TEST-1',
    observation: { blockNumber: '123456789', blockHash: HASH('observation-block'), observedAt: '2026-09-15T00:00:00.000Z' },
    deployer: { address: signers[0].address, codeType: 'eoa', runtimeCodeHash: EMPTY_CODE_HASH, startingNonce: String(startingNonce), latestNonce: String(startingNonce), pendingNonce: String(startingNonce), balanceWei: '1000000000000000000', attestationDigest: HASH('deployer-attestation') },
    roles: {
      governanceOwner: contractRole(signers[1].address, 'timelock', 'governance'),
      operator: eoaRole(signers[2].address, 'durable-signer', 'operator'),
      pauseGuardian: eoaRole(signers[3].address, 'guardian', 'guardian'),
      unpauseTimelock: contractRole(signers[4].address, 'timelock', 'unpause'),
      treasury: contractRole(signers[5].address, 'safe', 'treasury'),
      platformFeeReceiver: contractRole(signers[6].address, 'safe', 'fee'),
      projectProceedsReceiver: contractRole(signers[7].address, 'safe', 'proceeds'),
    },
    platformFee: { bps: 250, decisionDigest: HASH('fee-decision'), decisionVersion: 1 },
  };
}
describe('Fair Launch mainnet unsigned deployment bundle', function () {
  it('predicts the exact deployment set, initialization and runtime identities without signing', async function () {
    const [deployer] = await ethers.getSigners();
    const startingNonce = await deployer.getNonce();
    const input = await fixture(startingNonce);
    const bundle = await buildUnsignedDeploymentBundle(input);
    expect(bundle.status).to.equal(STATUS);
    expect(bundle.broadcastPermitted).to.equal(false);
    expect(bundle.chainId).to.equal(56);
    expect(bundle.transactions).to.have.length(9);
    expect(bundle.transactions.map((row) => row.nonce)).to.deep.equal(Array.from({ length: 9 }, (_, index) => String(startingNonce + index)));
    const set = await deployFacetSet();
    const iface = await aggregateInterface();
    const Proxy = await ethers.getContractFactory('FairLaunchERC1967Proxy');
    const proxy = await Proxy.deploy(await set.dispatcher.getAddress(), iface.encodeFunctionData('initialize', [
      input.roles.governanceOwner.address,
      input.roles.operator.address,
      input.roles.pauseGuardian.address,
      input.roles.unpauseTimelock.address,
      input.roles.treasury.address,
      input.roles.platformFeeReceiver.address,
      input.platformFee.bps,
    ]));
    await proxy.waitForDeployment();
    const actual = await manifest(set);
    expect(bundle.fullSetManifest).to.deep.equal(actual);
    expect(bundle.proxy.address).to.equal(await proxy.getAddress());
    expect(bundle.transactions.at(-1).data).to.equal(proxy.deploymentTransaction().data);
    expect(bundle.proxy.runtimeCodeHash).to.equal(ethers.keccak256(await ethers.provider.getCode(await proxy.getAddress())));
    const contract = new ethers.Contract(await proxy.getAddress(), iface, deployer);
    expect(await contract.owner()).to.equal(input.roles.governanceOwner.address);
    expect(await contract.operator()).to.equal(input.roles.operator.address);
    expect(await contract.pauseGuardian()).to.equal(input.roles.pauseGuardian.address);
    expect(await contract.unpauseTimelock()).to.equal(input.roles.unpauseTimelock.address);
    expect(await contract.treasury()).to.equal(input.roles.treasury.address);
    expect(await contract.platformFeeConfig()).to.deep.equal([250n, input.roles.platformFeeReceiver.address]);
  });
  it('fails closed for unresolved fee, non-contract governance, nonce drift, network drift and role reuse', async function () {
    const [deployer] = await ethers.getSigners();
    const base = await fixture(await deployer.getNonce());
    const cases = [
      [(value) => { value.platformFee.bps = null; }, 'platform fee bps unresolved or invalid'],
      [(value) => { value.roles.governanceOwner = eoaRole(value.roles.governanceOwner.address, 'timelock', 'bad-owner'); }, 'role governanceOwner code type invalid'],
      [(value) => { value.deployer.pendingNonce = String(BigInt(value.deployer.pendingNonce) + 1n); }, 'deployer nonce observation mismatch'],
      [(value) => { value.chainId = 97; }, 'deployment input network identity invalid'],
      [(value) => { value.roles.operator.address = value.roles.governanceOwner.address; }, 'governance role addresses must be distinct'],
      [(value) => { value.roles.projectProceedsReceiver.address = value.roles.platformFeeReceiver.address; }, 'platform fee and project proceeds receivers must be distinct'],
    ];
    for (const [mutate, message] of cases) {
      const changed = structuredClone(base);
      mutate(changed);
      await expect(buildUnsignedDeploymentBundle(changed)).to.be.rejectedWith(message);
    }
  });
  it('rejects duplicate, unsafe and trailing JSON before authority validation', function () {
    expect(() => parseJsonNoDuplicateKeys('{"chainId":56,"chainId":97}')).to.throw('duplicate field: chainId');
    expect(() => parseJsonNoDuplicateKeys('{"__proto__":{}}')).to.throw('unsafe field: __proto__');
    expect(() => parseJsonNoDuplicateKeys('{} true')).to.throw('trailing data');
  });
});
