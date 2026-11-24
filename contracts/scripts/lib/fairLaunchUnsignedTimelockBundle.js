'use strict';

// Offline construction only. No provider, signer, key, gas estimation, signing,
// or broadcasting is available from this module.
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { ethers, artifacts } = require('hardhat');
const { canonicalJson, digest } = require('../fairLaunchArchitecture');
const { EMPTY_CODE_HASH, parseJsonNoDuplicateKeys } = require('./fairLaunchUnsignedDeploymentBundle');

const ROOT = path.resolve(__dirname, '../..');
const INPUT_SCHEMA = 'fair-launch-mainnet-timelock-input-v1';
const BUNDLE_SCHEMA = 'fair-launch-mainnet-unsigned-timelock-bundle-v1';
const BUNDLE_DOMAIN = 'joincare.fair-launch.mainnet-unsigned-timelock-bundle.v1';
const STATUS = 'UNSIGNED_OFFLINE_REVIEW_REQUIRED';
const CONTRACT_NAME = 'JoinCareFairLaunchTimelock';
const SOURCE_FILE = 'contracts/governance/JoinCareFairLaunchTimelock.sol';
const GOVERNANCE_DELAY = 172800n;
const UNPAUSE_DELAY = 3600n;
const UINT64_MAX = (1n << 64n) - 1n;
const SAFE_L2_SINGLETON = '0x29fcB43b46531BcA003ddC8FCB67FFE91900C762';
const SAFE_FALLBACK_HANDLER = '0xfd0732Dc9E303f09fCEf3a7388Ad10A83459Ec99';
const SAFE_RUNTIME_CODE_HASH = '0xd7d408ebcd99b2b70be43e20253d6d92a8ea8fab29bd3be7f55b10032331fb4c';
const SAFE_EVIDENCE_DOMAIN = 'joincare.fair-launch.mainnet-safe-evidence.v1';

function exactKeys(value, keys, label) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${label} must be an object`);
  const actual = Object.keys(value);
  if (actual.length !== keys.length || actual.some((key) => !keys.includes(key))) throw new Error(`${label} schema invalid`);
}
function address(value, label) {
  if (typeof value !== 'string' || !ethers.isAddress(value) || value === ethers.ZeroAddress) throw new Error(`${label} invalid`);
  return ethers.getAddress(value);
}
function bytes32(value, label) {
  if (!/^0x[0-9a-f]{64}$/.test(value || '') || value === ethers.ZeroHash) throw new Error(`${label} invalid`);
  return value;
}
function uintString(value, label, maximum = (1n << 256n) - 1n) {
  if (!/^(0|[1-9]\d*)$/.test(value || '')) throw new Error(`${label} invalid`);
  const parsed = BigInt(value);
  if (parsed > maximum) throw new Error(`${label} invalid`);
  return parsed;
}
function positiveInteger(value, label, maximum = Number.MAX_SAFE_INTEGER) {
  if (!Number.isSafeInteger(value) || value < 1 || value > maximum) throw new Error(`${label} invalid`);
  return value;
}
function denseArray(value, length, label) {
  if (!Array.isArray(value) || value.length !== length || Object.keys(value).length !== value.length) throw new Error(`${label} invalid`);
  return value;
}
function safeAuthority(value, label) {
  exactKeys(value, ['address', 'runtimeCodeHash', 'owners', 'threshold', 'nonce', 'singleton', 'fallbackHandler', 'modules', 'guard', 'moduleGuard', 'creationTxHash', 'creationBlock'], label);
  if (value.runtimeCodeHash !== SAFE_RUNTIME_CODE_HASH) throw new Error(`${label} runtime code identity invalid`);
  if (value.threshold !== 2) throw new Error(`${label} threshold must be 2`);
  const owners = denseArray(value.owners, 3, `${label} owners`).map((owner, index) => address(owner, `${label} owner ${index}`));
  if (new Set(owners.map((owner) => owner.toLowerCase())).size !== 3) throw new Error(`${label} owner domains must be distinct`);
  if (value.nonce !== '0') throw new Error(`${label} must be unused at admission`);
  if (address(value.singleton, `${label} singleton`) !== SAFE_L2_SINGLETON) throw new Error(`${label} singleton identity invalid`);
  if (address(value.fallbackHandler, `${label} fallback handler`) !== SAFE_FALLBACK_HANDLER) throw new Error(`${label} fallback handler identity invalid`);
  denseArray(value.modules, 0, `${label} modules`);
  if (value.guard !== ethers.ZeroAddress || value.moduleGuard !== ethers.ZeroAddress) throw new Error(`${label} guard admission invalid`);
  const creationBlock = uintString(value.creationBlock, `${label} creation block`, UINT64_MAX);
  if (creationBlock < 1n) throw new Error(`${label} creation block invalid`);
  const evidence = {
    address: address(value.address, `${label} address`),
    runtimeCodeHash: value.runtimeCodeHash,
    owners,
    threshold: value.threshold,
    nonce: value.nonce,
    singleton: SAFE_L2_SINGLETON,
    fallbackHandler: SAFE_FALLBACK_HANDLER,
    modules: [],
    guard: value.guard,
    moduleGuard: value.moduleGuard,
    creationTxHash: bytes32(value.creationTxHash, `${label} creation tx hash`),
    creationBlock: String(creationBlock),
  };
  return { ...evidence, evidenceDomain: SAFE_EVIDENCE_DOMAIN, evidenceDigest: digest({ domain: SAFE_EVIDENCE_DOMAIN, ...evidence }) };
}
function validateInput(input) {
  exactKeys(input, ['schemaVersion', 'environment', 'chainId', 'changeTicket', 'observation', 'deployer', 'safes'], 'timelock input');
  if (input.schemaVersion !== INPUT_SCHEMA || input.environment !== 'bsc-mainnet' || input.chainId !== 56) throw new Error('timelock network identity invalid');
  if (!/^[A-Za-z0-9][A-Za-z0-9._/-]{2,127}$/.test(input.changeTicket || '')) throw new Error('change ticket invalid');
  exactKeys(input.observation, ['blockNumber', 'blockHash', 'observedAt'], 'chain observation');
  if (uintString(input.observation.blockNumber, 'observation block number', UINT64_MAX) < 1n) throw new Error('observation block number invalid');
  bytes32(input.observation.blockHash, 'observation block hash');
  if (typeof input.observation.observedAt !== 'string' || !Number.isFinite(Date.parse(input.observation.observedAt)) || new Date(input.observation.observedAt).toISOString() !== input.observation.observedAt) throw new Error('observation timestamp invalid');
  exactKeys(input.deployer, ['address', 'codeType', 'runtimeCodeHash', 'startingNonce', 'latestNonce', 'pendingNonce', 'balanceWei', 'attestationDigest'], 'deployer');
  if (input.deployer.codeType !== 'eoa' || input.deployer.runtimeCodeHash !== EMPTY_CODE_HASH) throw new Error('deployer must be an observed EOA');
  const startingNonce = uintString(input.deployer.startingNonce, 'deployer starting nonce', UINT64_MAX);
  if (uintString(input.deployer.latestNonce, 'deployer latest nonce', UINT64_MAX) !== startingNonce || uintString(input.deployer.pendingNonce, 'deployer pending nonce', UINT64_MAX) !== startingNonce) throw new Error('deployer nonce observation mismatch');
  if (uintString(input.deployer.balanceWei, 'deployer balance') < 1n) throw new Error('deployer balance unavailable');
  exactKeys(input.safes, ['governance', 'emergency'], 'timelock safes');
  const safes = { governance: safeAuthority(input.safes.governance, 'governance Safe'), emergency: safeAuthority(input.safes.emergency, 'emergency Safe') };
  if (safes.governance.address.toLowerCase() === safes.emergency.address.toLowerCase()) throw new Error('governance and emergency Safe domains must be distinct');
  if (canonicalJson(safes.governance.owners) !== canonicalJson(safes.emergency.owners)) throw new Error('governance and emergency Safe owner sets must match');
  const observationBlock = uintString(input.observation.blockNumber, 'observation block number', UINT64_MAX);
  for (const [name, safe] of Object.entries(safes)) {
    if (observationBlock < BigInt(safe.creationBlock) || observationBlock - BigInt(safe.creationBlock) + 1n < 20n) throw new Error(`${name} Safe lacks 20-block finality`);
  }
  return {
    ...input,
    deployer: { ...input.deployer, address: address(input.deployer.address, 'deployer address'), attestationDigest: bytes32(input.deployer.attestationDigest, 'deployer attestation digest') },
    safes,
    startingNonce,
  };
}
function sourceIdentity(build) {
  const files = Object.entries(build.input.sources).map(([file, value]) => ({ path: file, sha256: crypto.createHash('sha256').update(value.content).digest('hex') })).sort((a, b) => a.path.localeCompare(b.path));
  const packageLockSha256 = crypto.createHash('sha256').update(fs.readFileSync(path.join(ROOT, 'package-lock.json'))).digest('hex');
  return { files, packageLockSha256, sourceTreeDigest: digest({ files, packageLockSha256 }) };
}
async function buildUnsignedTimelockBundle(rawInput) {
  const input = validateInput(rawInput);
  const artifact = await artifacts.readArtifact(CONTRACT_NAME);
  const build = await artifacts.getBuildInfo(`${artifact.sourceName}:${artifact.contractName}`);
  if (!build) throw new Error('timelock build info unavailable');
  const compiler = {
    solcVersion: build.solcVersion,
    solcLongVersion: build.solcLongVersion,
    viaIR: build.input.settings.viaIR === true,
    optimizer: { enabled: build.input.settings.optimizer?.enabled === true, runs: build.input.settings.optimizer?.runs },
  };
  const expectedCompiler = { solcVersion: '0.8.22', solcLongVersion: '0.8.22+commit.4fc1097e', viaIR: true, optimizer: { enabled: true, runs: 200 } };
  if (canonicalJson(compiler) !== canonicalJson(expectedCompiler)) throw new Error('timelock compiler identity mismatch');
  const rows = [
    { key: 'governance', delay: GOVERNANCE_DELAY, safe: input.safes.governance },
    { key: 'unpause', delay: UNPAUSE_DELAY, safe: input.safes.emergency },
  ];
  const factory = new ethers.ContractFactory(artifact.abi, artifact.bytecode);
  const transactions = [];
  for (let index = 0; index < rows.length; index += 1) {
    const row = rows[index];
    const nonce = input.startingNonce + BigInt(index);
    const predictedAddress = ethers.getCreateAddress({ from: input.deployer.address, nonce });
    const constructorArguments = [row.delay, [row.safe.address], [row.safe.address]];
    const data = (await factory.getDeployTransaction(...constructorArguments)).data;
    transactions.push({
      order: index + 1,
      nonce: String(nonce),
      kind: 'contract_creation',
      authority: row.key,
      contractName: CONTRACT_NAME,
      predictedAddress,
      minDelaySeconds: String(row.delay),
      proposer: row.safe.address,
      canceller: row.safe.address,
      executor: row.safe.address,
      admin: ethers.ZeroAddress,
      constructorArguments: [String(row.delay), [row.safe.address], [row.safe.address]],
      valueRaw: '0',
      data,
      dataKeccak256: ethers.keccak256(data),
      expectedRuntimeCodeHash: ethers.keccak256(artifact.deployedBytecode),
    });
  }
  const roleIds = {
    defaultAdmin: ethers.ZeroHash,
    proposer: ethers.id('PROPOSER_ROLE'),
    executor: ethers.id('EXECUTOR_ROLE'),
    canceller: ethers.id('CANCELLER_ROLE'),
  };
  const payload = {
    schemaVersion: BUNDLE_SCHEMA,
    domain: BUNDLE_DOMAIN,
    status: STATUS,
    broadcastPermitted: false,
    environment: input.environment,
    chainId: input.chainId,
    changeTicket: input.changeTicket,
    observation: input.observation,
    deployer: { ...input.deployer, startingNonce: String(input.startingNonce) },
    safes: input.safes,
    source: sourceIdentity(build),
    compiler,
    openZeppelinContractsVersion: require('../../node_modules/@openzeppelin/contracts/package.json').version,
    contractIdentity: {
      contractName: CONTRACT_NAME,
      abiDigest: digest(artifact.abi),
      creationBytecodeHash: ethers.keccak256(artifact.bytecode),
      runtimeCodeHash: ethers.keccak256(artifact.deployedBytecode),
    },
    transactionCount: transactions.length,
    finalNonceExclusive: String(input.startingNonce + BigInt(transactions.length)),
    roleIds,
    transactions,
  };
  return { ...payload, bundleDigest: digest(payload) };
}

module.exports = { BUNDLE_DOMAIN, BUNDLE_SCHEMA, INPUT_SCHEMA, SAFE_EVIDENCE_DOMAIN, SAFE_FALLBACK_HANDLER, SAFE_L2_SINGLETON, SAFE_RUNTIME_CODE_HASH, STATUS, buildUnsignedTimelockBundle, parseJsonNoDuplicateKeys, validateInput };
