'use strict';

const { expect } = require('chai');
const { ethers } = require('hardhat');
const { buildPaymentTokenGovernanceBundle, GENESIS_HASH, GOVERNANCE_SAFE, PROXY, TIMELOCK, TOKENS } = require('../scripts/lib/fairLaunchPaymentTokenGovernanceBundle');

function input() {
  return {
    schemaVersion: 'fair-launch-mainnet-payment-token-governance-input-v1', environment: 'bsc-mainnet', chainId: 56, genesisHash: GENESIS_HASH, changeTicket: 'JC-FL-BSC-PAYMENT-TOKENS-20260916',
    observation: { blockNumber: '122255504', blockHash: '0xd839c26e493a6a08ddebe305972139fa6e27e870b25416405e330cb1c091524d', observedAt: '2026-09-16T16:42:54.755Z' },
    fairLaunchProxy: PROXY, governanceTimelock: TIMELOCK, governanceSafe: GOVERNANCE_SAFE, minDelaySeconds: '172800', policyVersion: 1,
    riskAcceptance: { scope: 'test-server-minimum-real-funds-only', maximumRealFunds: 'per-project-governance-approved-cap', centralizedIssuerControlsAccepted: true, proxyImplementationMonitoringRequired: true, identityDriftEmergencyBlockRequired: true, exactBalanceDeltaRequired: true, limitations: ['Issuer administrative controls are outside protocol control.', 'Proxy implementation changes require offchain monitoring.', 'Blacklist or transfer restrictions can delay refunds.'] },
    tokens: [
      { symbol: 'USDT', address: TOKENS.USDT.address, onchainSymbol: 'USDT', decimals: 18, runtimeCodeHash: TOKENS.USDT.runtimeCodeHash, runtimeCodeBytes: 4413, runtimeKind: 'direct', owner: '0xF68a4b64162906efF0fF6aE34E2bB1Cd42FEf62d', implementation: null },
      { symbol: 'USDC', address: TOKENS.USDC.address, onchainSymbol: 'USDC', decimals: 18, runtimeCodeHash: TOKENS.USDC.runtimeCodeHash, runtimeCodeBytes: 1596, runtimeKind: 'eip1967_proxy', owner: '0xF68a4b64162906efF0fF6aE34E2bB1Cd42FEf62d', implementation: { address: TOKENS.USDC.implementationAddress, runtimeCodeHash: TOKENS.USDC.implementationRuntimeCodeHash, runtimeCodeBytes: 5066 } },
    ],
  };
}

describe('Fair Launch payment-token governance bundle', function () {
  it('builds one deterministic four-call timelock batch', async function () {
    const bundle = await buildPaymentTokenGovernanceBundle(input(), 'fe29c78c6c281cfeaa6507e8659e1d0c4dd45e41');
    expect(bundle.broadcastPermitted).to.equal(false);
    expect(bundle.calls.map((call) => call.action)).to.deep.equal(['registerPaymentTokenPolicyRevision', 'setPaymentTokenAdmission', 'registerPaymentTokenPolicyRevision', 'setPaymentTokenAdmission']);
    expect(bundle.policies).to.have.length(2);
    expect(new Set(bundle.policies.map((policy) => policy.policyDigest)).size).to.equal(2);
    expect(bundle.gasBudget.totalMaximumGasUnits).to.equal('4000000');
    const timelock = new ethers.Interface((await ethers.getContractFactory('JoinCareFairLaunchTimelock')).interface.fragments);
    const decoded = timelock.decodeFunctionData('scheduleBatch', bundle.scheduleTransaction.data);
    expect(decoded[0]).to.have.length(4);
    expect(decoded[3]).to.equal(ethers.ZeroHash);
    expect(decoded[4]).to.equal(bundle.salt);
    expect(decoded[5]).to.equal(172800n);
    const calculated = ethers.keccak256(ethers.AbiCoder.defaultAbiCoder().encode(['address[]', 'uint256[]', 'bytes[]', 'bytes32', 'bytes32'], [decoded[0], decoded[1], decoded[2], decoded[3], decoded[4]]));
    expect(calculated).to.equal(bundle.operationId);
  });

  it('rejects runtime and risk-policy drift', async function () {
    const changed = input();
    changed.tokens[1].runtimeCodeHash = ethers.ZeroHash;
    await expect(buildPaymentTokenGovernanceBundle(changed, 'fe29c78c6c281cfeaa6507e8659e1d0c4dd45e41')).to.be.rejectedWith('runtime code hash');
    const weakened = input();
    weakened.riskAcceptance.identityDriftEmergencyBlockRequired = false;
    await expect(buildPaymentTokenGovernanceBundle(weakened, 'fe29c78c6c281cfeaa6507e8659e1d0c4dd45e41')).to.be.rejectedWith('identityDriftEmergencyBlockRequired');
  });
});
