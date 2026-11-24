'use strict';

// Offline construction only. This module has no provider, signer, private key,
// Safe integration, gas submission, or broadcasting capability.
const { ethers, artifacts } = require('hardhat');
const { canonicalJson, digest } = require('../fairLaunchArchitecture');
const { parseJsonNoDuplicateKeys } = require('./fairLaunchUnsignedDeploymentBundle');

const INPUT_SCHEMA = 'fair-launch-mainnet-payment-token-governance-input-v1';
const BUNDLE_SCHEMA = 'fair-launch-mainnet-payment-token-governance-bundle-v1';
const BUNDLE_DOMAIN = 'joincare.fair-launch.mainnet-payment-token-governance.v1';
const POLICY_SCHEMA = 'fair-launch-payment-token-policy-evidence-v1';
const POLICY_DOMAIN = 'joincare.fair-launch.payment-token-policy-evidence.v1';
const SALT_DOMAIN = 'joincare.fair-launch.mainnet-payment-token-governance-salt.v1';
const STATUS = 'UNSIGNED_OFFLINE_REVIEW_REQUIRED';
const CHAIN_ID = 56;
const GENESIS_HASH = '0x0d21840abff46b96c84b2ac9e10e4f5cdaeb5693cb665db62a2f3b02d2d57b5b';
const PROXY = '0xaEC57de2b134e3539edbB70a552483031a14BD84';
const TIMELOCK = '0x0eE7f9f79B44446eFE8741037D5d46419266083D';
const GOVERNANCE_SAFE = '0x79c527B8Af5b1E27d5052f8dCe04D20733e72350';
const MIN_DELAY = 172800n;
const ZERO = ethers.ZeroHash;
const TOKENS = Object.freeze({
  USDT: Object.freeze({
    address: '0x55d398326f99059fF775485246999027B3197955',
    decimals: 18,
    runtimeCodeHash: '0x97a48aa4c129657440dafdacd4c836389734d28cc4a0ca7403e68da660a74a59',
    runtimeKind: 'direct',
  }),
  USDC: Object.freeze({
    address: '0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d',
    decimals: 18,
    runtimeCodeHash: '0x56069ff2fd59ae482f7984aa894b6d3bc84cb910354dcda9cd6c828d61526914',
    runtimeKind: 'eip1967_proxy',
    implementationAddress: '0xba5fe23f8a3a24bed3236f05f2fcf35fd0bf0b5c',
    implementationRuntimeCodeHash: '0xd2d7cf7504e1cb572af28476d45907fc30bee1297917bed153e6fbba125821a3',
  }),
});

function exactKeys(value, keys, label) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${label} must be an object`);
  const actual = Object.keys(value);
  if (actual.length !== keys.length || actual.some((key) => !keys.includes(key))) throw new Error(`${label} schema invalid`);
}
function address(value, label) {
  if (typeof value !== 'string' || !ethers.isAddress(value) || value === ethers.ZeroAddress) throw new Error(`${label} invalid`);
  return ethers.getAddress(value);
}
function bytes32(value, label, allowZero = false) {
  if (!/^0x[0-9a-f]{64}$/.test(value || '') || (!allowZero && value === ZERO)) throw new Error(`${label} invalid`);
  return value;
}
function uintString(value, label) {
  if (!/^(0|[1-9]\d*)$/.test(value || '')) throw new Error(`${label} invalid`);
  return BigInt(value);
}
function iso(value, label) {
  if (typeof value !== 'string' || !Number.isFinite(Date.parse(value)) || new Date(value).toISOString() !== value) throw new Error(`${label} invalid`);
  return value;
}
function assertExact(value, expected, label) {
  if (String(value).toLowerCase() !== String(expected).toLowerCase()) throw new Error(`${label} mismatch`);
}
function validateToken(input, expected, symbol) {
  exactKeys(input, ['symbol', 'address', 'onchainSymbol', 'decimals', 'runtimeCodeHash', 'runtimeCodeBytes', 'runtimeKind', 'owner', 'implementation'], `${symbol} token`);
  if (input.symbol !== symbol || input.onchainSymbol !== symbol) throw new Error(`${symbol} symbol mismatch`);
  assertExact(address(input.address, `${symbol} address`), expected.address, `${symbol} address`);
  if (input.decimals !== expected.decimals) throw new Error(`${symbol} decimals mismatch`);
  assertExact(bytes32(input.runtimeCodeHash, `${symbol} runtime code hash`), expected.runtimeCodeHash, `${symbol} runtime code hash`);
  if (!Number.isSafeInteger(input.runtimeCodeBytes) || input.runtimeCodeBytes < 1) throw new Error(`${symbol} runtime byte length invalid`);
  if (input.runtimeKind !== expected.runtimeKind) throw new Error(`${symbol} runtime kind mismatch`);
  address(input.owner, `${symbol} owner`);
  if (expected.runtimeKind === 'direct') {
    if (input.implementation !== null) throw new Error(`${symbol} implementation must be null`);
  } else {
    exactKeys(input.implementation, ['address', 'runtimeCodeHash', 'runtimeCodeBytes'], `${symbol} implementation`);
    assertExact(address(input.implementation.address, `${symbol} implementation address`), expected.implementationAddress, `${symbol} implementation address`);
    assertExact(bytes32(input.implementation.runtimeCodeHash, `${symbol} implementation runtime code hash`), expected.implementationRuntimeCodeHash, `${symbol} implementation runtime code hash`);
    if (!Number.isSafeInteger(input.implementation.runtimeCodeBytes) || input.implementation.runtimeCodeBytes < 1) throw new Error(`${symbol} implementation runtime byte length invalid`);
  }
  return {
    symbol,
    address: ethers.getAddress(input.address),
    onchainSymbol: input.onchainSymbol,
    decimals: input.decimals,
    runtimeCodeHash: input.runtimeCodeHash,
    runtimeCodeBytes: input.runtimeCodeBytes,
    runtimeKind: input.runtimeKind,
    owner: ethers.getAddress(input.owner),
    implementation: input.implementation === null ? null : {
      address: ethers.getAddress(input.implementation.address),
      runtimeCodeHash: input.implementation.runtimeCodeHash,
      runtimeCodeBytes: input.implementation.runtimeCodeBytes,
    },
  };
}
function validateInput(input) {
  exactKeys(input, ['schemaVersion', 'environment', 'chainId', 'genesisHash', 'changeTicket', 'observation', 'fairLaunchProxy', 'governanceTimelock', 'governanceSafe', 'minDelaySeconds', 'policyVersion', 'riskAcceptance', 'tokens'], 'governance input');
  if (input.schemaVersion !== INPUT_SCHEMA || input.environment !== 'bsc-mainnet' || input.chainId !== CHAIN_ID) throw new Error('network identity invalid');
  assertExact(bytes32(input.genesisHash, 'genesis hash'), GENESIS_HASH, 'genesis hash');
  if (!/^[A-Za-z0-9][A-Za-z0-9._/-]{2,127}$/.test(input.changeTicket || '')) throw new Error('change ticket invalid');
  exactKeys(input.observation, ['blockNumber', 'blockHash', 'observedAt'], 'observation');
  const blockNumber = uintString(input.observation.blockNumber, 'observation block number');
  if (blockNumber < 1n) throw new Error('observation block number invalid');
  bytes32(input.observation.blockHash, 'observation block hash');
  iso(input.observation.observedAt, 'observation time');
  assertExact(address(input.fairLaunchProxy, 'Fair Launch Proxy'), PROXY, 'Fair Launch Proxy');
  assertExact(address(input.governanceTimelock, 'Governance Timelock'), TIMELOCK, 'Governance Timelock');
  assertExact(address(input.governanceSafe, 'Governance Safe'), GOVERNANCE_SAFE, 'Governance Safe');
  if (uintString(input.minDelaySeconds, 'minimum delay') !== MIN_DELAY) throw new Error('minimum delay mismatch');
  if (input.policyVersion !== 1) throw new Error('policy version must be 1');
  exactKeys(input.riskAcceptance, ['scope', 'maximumRealFunds', 'centralizedIssuerControlsAccepted', 'proxyImplementationMonitoringRequired', 'identityDriftEmergencyBlockRequired', 'exactBalanceDeltaRequired', 'limitations'], 'risk acceptance');
  if (input.riskAcceptance.scope !== 'test-server-minimum-real-funds-only' || input.riskAcceptance.maximumRealFunds !== 'per-project-governance-approved-cap') throw new Error('risk scope invalid');
  for (const key of ['centralizedIssuerControlsAccepted', 'proxyImplementationMonitoringRequired', 'identityDriftEmergencyBlockRequired', 'exactBalanceDeltaRequired']) if (input.riskAcceptance[key] !== true) throw new Error(`${key} must be true`);
  if (!Array.isArray(input.riskAcceptance.limitations) || input.riskAcceptance.limitations.length < 3 || input.riskAcceptance.limitations.some((value) => typeof value !== 'string' || value.length < 8)) throw new Error('risk limitations invalid');
  if (!Array.isArray(input.tokens) || input.tokens.length !== 2) throw new Error('exactly USDT and USDC are required');
  const bySymbol = Object.fromEntries(input.tokens.map((token) => [token.symbol, token]));
  if (Object.keys(bySymbol).length !== 2 || !bySymbol.USDT || !bySymbol.USDC) throw new Error('token set invalid');
  return { ...input, observation: { ...input.observation, blockNumber: String(blockNumber) }, tokens: ['USDT', 'USDC'].map((symbol) => validateToken(bySymbol[symbol], TOKENS[symbol], symbol)) };
}
function policyEvidence(input, token) {
  return {
    schemaVersion: POLICY_SCHEMA,
    domain: POLICY_DOMAIN,
    environment: input.environment,
    chainId: input.chainId,
    genesisHash: input.genesisHash,
    fairLaunchProxy: ethers.getAddress(input.fairLaunchProxy),
    policyVersion: input.policyVersion,
    observation: input.observation,
    token,
    enforcement: {
      onchainRuntimeCodeHashPinned: true,
      onchainDecimalsPinned: true,
      projectSnapshotImmutable: true,
      exactBalanceDeltaRequired: true,
      governanceEmergencyBlockOnIdentityDrift: true,
      offchainImplementationIdentityMonitoringRequired: token.runtimeKind === 'eip1967_proxy',
    },
    riskAcceptance: input.riskAcceptance,
  };
}
async function buildPaymentTokenGovernanceBundle(rawInput, sourceCommit) {
  const input = validateInput(rawInput);
  if (!/^[0-9a-f]{40}$/.test(sourceCommit || '')) throw new Error('clean contract source commit required');
  const admin = new ethers.Interface((await artifacts.readArtifact('JoinCareFairLaunchV1AdminFacet')).abi);
  const timelock = new ethers.Interface((await artifacts.readArtifact('JoinCareFairLaunchTimelock')).abi);
  const policies = input.tokens.map((token) => {
    const evidence = policyEvidence(input, token);
    return { symbol: token.symbol, token: token.address, policyVersion: input.policyVersion, evidence, policyDigest: digest(evidence) };
  });
  const calls = [];
  for (const policy of policies) {
    calls.push({
      order: calls.length + 1,
      symbol: policy.symbol,
      action: 'registerPaymentTokenPolicyRevision',
      target: ethers.getAddress(input.fairLaunchProxy),
      valueRaw: '0',
      data: admin.encodeFunctionData('registerPaymentTokenPolicyRevision', [policy.token, [true, input.tokens.find((token) => token.symbol === policy.symbol).decimals, input.policyVersion, input.tokens.find((token) => token.symbol === policy.symbol).runtimeCodeHash, policy.policyDigest]]),
      expectedEvent: 'FairLaunchPaymentTokenPolicyRevisionRegistered',
    });
    calls.push({
      order: calls.length + 1,
      symbol: policy.symbol,
      action: 'setPaymentTokenAdmission',
      target: ethers.getAddress(input.fairLaunchProxy),
      valueRaw: '0',
      data: admin.encodeFunctionData('setPaymentTokenAdmission', [policy.token, input.policyVersion, true]),
      expectedEvent: 'FairLaunchPaymentTokenAdmissionUpdated',
    });
  }
  for (const call of calls) call.dataKeccak256 = ethers.keccak256(call.data);
  const targets = calls.map((call) => call.target);
  const values = calls.map(() => 0n);
  const payloads = calls.map((call) => call.data);
  const saltIntent = {
    domain: SALT_DOMAIN,
    environment: input.environment,
    chainId: input.chainId,
    fairLaunchProxy: ethers.getAddress(input.fairLaunchProxy),
    governanceTimelock: ethers.getAddress(input.governanceTimelock),
    policyVersion: input.policyVersion,
    policies: policies.map(({ symbol, token, policyDigest }) => ({ symbol, token, policyDigest })),
    callDataKeccak256: calls.map((call) => call.dataKeccak256),
  };
  const salt = digest(saltIntent);
  const operationId = ethers.keccak256(ethers.AbiCoder.defaultAbiCoder().encode(['address[]', 'uint256[]', 'bytes[]', 'bytes32', 'bytes32'], [targets, values, payloads, ZERO, salt]));
  const scheduleCalldata = timelock.encodeFunctionData('scheduleBatch', [targets, values, payloads, ZERO, salt, MIN_DELAY]);
  const executeCalldata = timelock.encodeFunctionData('executeBatch', [targets, values, payloads, ZERO, salt]);
  const payload = {
    schemaVersion: BUNDLE_SCHEMA,
    domain: BUNDLE_DOMAIN,
    status: STATUS,
    broadcastPermitted: false,
    environment: input.environment,
    chainId: input.chainId,
    genesisHash: input.genesisHash,
    changeTicket: input.changeTicket,
    sourceCommit,
    observation: input.observation,
    authorities: { governanceSafe: ethers.getAddress(input.governanceSafe), governanceTimelock: ethers.getAddress(input.governanceTimelock), fairLaunchProxy: ethers.getAddress(input.fairLaunchProxy) },
    minimumDelaySeconds: String(MIN_DELAY),
    predecessor: ZERO,
    saltIntent,
    salt,
    operationId,
    policies,
    calls,
    scheduleTransaction: { target: ethers.getAddress(input.governanceTimelock), valueRaw: '0', data: scheduleCalldata, dataKeccak256: ethers.keccak256(scheduleCalldata) },
    executeTransaction: { target: ethers.getAddress(input.governanceTimelock), valueRaw: '0', data: executeCalldata, dataKeccak256: ethers.keccak256(executeCalldata), earliestExecution: 'schedule-block-timestamp-plus-172800-seconds' },
    requiredPreState: {
      timelockOwnerOfFairLaunchProxy: true,
      governanceSafeProposer: true,
      governanceSafeExecutor: true,
      governanceSafeCanceller: true,
      policyRevision1Absent: ['USDT', 'USDC'],
      admissionDisabled: ['USDT', 'USDC'],
    },
    requiredPostState: {
      policyVersion: 1,
      admissionEnabledForNewProjects: ['USDT', 'USDC'],
      emergencyBlocked: false,
      emittedEvents: calls.map((call) => call.expectedEvent),
    },
    gasBudget: {
      source: 'offline conservative review cap; wallet must estimate immediately before submission',
      scheduleBatchGasLimit: '1500000',
      executeBatchGasLimit: '2500000',
      totalMaximumGasUnits: '4000000',
      perTransactionValueWei: '0',
    },
  };
  return { ...payload, bundleDigest: digest(payload) };
}

module.exports = { BUNDLE_DOMAIN, BUNDLE_SCHEMA, CHAIN_ID, GENESIS_HASH, GOVERNANCE_SAFE, INPUT_SCHEMA, MIN_DELAY, POLICY_DOMAIN, POLICY_SCHEMA, PROXY, SALT_DOMAIN, STATUS, TIMELOCK, TOKENS, buildPaymentTokenGovernanceBundle, parseJsonNoDuplicateKeys, validateInput };
