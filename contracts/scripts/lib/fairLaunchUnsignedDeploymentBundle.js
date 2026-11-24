'use strict';

// Offline construction only. This module never reads a key, creates a signer,
// calls a provider, estimates gas, signs, or broadcasts a transaction.
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { ethers, artifacts } = require('hardhat');
const {
  PRODUCTION_COMPILER,
  canonicalJson,
  currentSourceIdentity,
  digest,
  sortedWireAbi,
  verifyActualRouting,
  verifyAggregateAbi,
  verifyProductionCompilerSettings,
} = require('../fairLaunchArchitecture');

const ROOT = path.resolve(__dirname, '../..');
const EMPTY_CODE_HASH = ethers.keccak256('0x');
const INPUT_SCHEMA = 'fair-launch-mainnet-deployment-input-v1';
const BUNDLE_SCHEMA = 'fair-launch-mainnet-unsigned-deployment-bundle-v1';
const BUNDLE_DOMAIN = 'joincare.fair-launch.mainnet-unsigned-deployment-bundle.v1';
const STATUS = 'UNSIGNED_OFFLINE_REVIEW_REQUIRED';
const UINT64_MAX = (1n << 64n) - 1n;
const ROLE_RULES = Object.freeze({
  governanceOwner: { kinds: ['safe', 'timelock'], codeType: 'contract' },
  operator: { kinds: ['durable-signer'], codeType: ['contract', 'eoa'] },
  pauseGuardian: { kinds: ['guardian'], codeType: ['contract', 'eoa'] },
  unpauseTimelock: { kinds: ['timelock'], codeType: 'contract' },
  treasury: { kinds: ['safe'], codeType: 'contract' },
  platformFeeReceiver: { kinds: ['safe'], codeType: 'contract' },
  projectProceedsReceiver: { kinds: ['safe'], codeType: 'contract' },
});
const DEPLOYMENTS = Object.freeze([
  ['logic', 'JoinCareFairLaunchV1Logic'],
  ['runtime', 'JoinCareFairLaunchV1Runtime'],
  ['core', 'JoinCareFairLaunchV1CoreFacet'],
  ['release', 'JoinCareFairLaunchV1ReleaseFacet'],
  ['funds', 'JoinCareFairLaunchV1FundsFacet'],
  ['admin', 'JoinCareFairLaunchV1AdminFacet'],
  ['views', 'JoinCareFairLaunchV1ViewsFacet'],
  ['dispatcher', 'JoinCareFairLaunchV1'],
  ['proxy', 'FairLaunchERC1967Proxy'],
]);
const SOURCE_FILES = Object.freeze([
  'contracts/JoinCareFairLaunchV1.sol',
  'contracts/fairlaunch/JoinCareFairLaunchV1Shared.sol',
  'contracts/fairlaunch/JoinCareFairLaunchV1CoreFacet.sol',
  'contracts/fairlaunch/JoinCareFairLaunchV1ReleaseFacet.sol',
  'contracts/fairlaunch/JoinCareFairLaunchV1FundsFacet.sol',
  'contracts/fairlaunch/JoinCareFairLaunchV1AdminFacet.sol',
  'contracts/fairlaunch/JoinCareFairLaunchV1ViewsFacet.sol',
  'contracts/libraries/JoinCareFairLaunchV1Logic.sol',
  'contracts/libraries/JoinCareFairLaunchV1Runtime.sol',
  'contracts/mocks/FairLaunchERC1967Proxy.sol',
]);

function exactKeys(value, keys, label) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${label} must be an object`);
  const actual = Object.keys(value);
  if (actual.length !== keys.length || actual.some((key) => !keys.includes(key))) throw new Error(`${label} schema invalid`);
}
function parseJsonNoDuplicateKeys(text, label = 'deployment input') {
  let index = 0;
  const whitespace = () => { while (/\s/.test(text[index] || '')) index += 1; };
  const string = () => {
    const start = index;
    if (text[index++] !== '"') throw new Error(`${label} invalid JSON`);
    let escaped = false;
    while (index < text.length) {
      const character = text[index++];
      if (escaped) { escaped = false; continue; }
      if (character === '\\') { escaped = true; continue; }
      if (character === '"') return JSON.parse(text.slice(start, index));
    }
    throw new Error(`${label} invalid JSON`);
  };
  const value = () => {
    whitespace();
    const character = text[index];
    if (character === '"') return string();
    if (character === '{') return object();
    if (character === '[') return array();
    const match = text.slice(index).match(/^(true|false|null|-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?)/);
    if (!match) throw new Error(`${label} invalid JSON`);
    index += match[0].length;
    return JSON.parse(match[0]);
  };
  const object = () => {
    index += 1;
    const result = {};
    const seen = new Set();
    whitespace();
    if (text[index] === '}') { index += 1; return result; }
    while (true) {
      whitespace();
      const key = string();
      if (seen.has(key)) throw new Error(`${label} duplicate field: ${key}`);
      if (['__proto__', 'prototype', 'constructor'].includes(key)) throw new Error(`${label} unsafe field: ${key}`);
      seen.add(key);
      whitespace();
      if (text[index++] !== ':') throw new Error(`${label} invalid JSON`);
      Object.defineProperty(result, key, { value: value(), enumerable: true, writable: true, configurable: true });
      whitespace();
      if (text[index] === '}') { index += 1; return result; }
      if (text[index++] !== ',') throw new Error(`${label} invalid JSON`);
    }
  };
  const array = () => {
    index += 1;
    const result = [];
    whitespace();
    if (text[index] === ']') { index += 1; return result; }
    while (true) {
      result.push(value());
      whitespace();
      if (text[index] === ']') { index += 1; return result; }
      if (text[index++] !== ',') throw new Error(`${label} invalid JSON`);
    }
  };
  const result = value();
  whitespace();
  if (index !== text.length) throw new Error(`${label} trailing data`);
  return result;
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
function role(value, name) {
  exactKeys(value, ['address', 'authorityKind', 'codeType', 'runtimeCodeHash', 'attestationDigest', 'attestationVersion'], `role ${name}`);
  const rule = ROLE_RULES[name];
  if (!rule.kinds.includes(value.authorityKind)) throw new Error(`role ${name} authority kind invalid`);
  const allowedCodeTypes = Array.isArray(rule.codeType) ? rule.codeType : [rule.codeType];
  if (!allowedCodeTypes.includes(value.codeType)) throw new Error(`role ${name} code type invalid`);
  const runtimeCodeHash = bytes32(value.runtimeCodeHash, `role ${name} runtime code hash`);
  if (value.codeType === 'eoa' && runtimeCodeHash !== EMPTY_CODE_HASH) throw new Error(`role ${name} EOA code hash invalid`);
  if (value.codeType === 'contract' && runtimeCodeHash === EMPTY_CODE_HASH) throw new Error(`role ${name} contract code hash invalid`);
  return {
    address: address(value.address, `role ${name} address`),
    authorityKind: value.authorityKind,
    codeType: value.codeType,
    runtimeCodeHash,
    attestationDigest: bytes32(value.attestationDigest, `role ${name} attestation digest`),
    attestationVersion: positiveInteger(value.attestationVersion, `role ${name} attestation version`, 0xffffffff),
  };
}
function validateInput(input) {
  exactKeys(input, ['schemaVersion', 'environment', 'chainId', 'changeTicket', 'observation', 'deployer', 'roles', 'platformFee'], 'deployment input');
  if (input.schemaVersion !== INPUT_SCHEMA || input.environment !== 'bsc-mainnet' || input.chainId !== 56) throw new Error('deployment input network identity invalid');
  if (!/^[A-Za-z0-9][A-Za-z0-9._/-]{2,127}$/.test(input.changeTicket || '')) throw new Error('change ticket invalid');
  exactKeys(input.observation, ['blockNumber', 'blockHash', 'observedAt'], 'chain observation');
  const blockNumber = uintString(input.observation.blockNumber, 'observation block number', UINT64_MAX);
  if (blockNumber < 1n) throw new Error('observation block number invalid');
  bytes32(input.observation.blockHash, 'observation block hash');
  if (typeof input.observation.observedAt !== 'string' || !Number.isFinite(Date.parse(input.observation.observedAt)) || new Date(input.observation.observedAt).toISOString() !== input.observation.observedAt) throw new Error('observation timestamp invalid');
  exactKeys(input.deployer, ['address', 'codeType', 'runtimeCodeHash', 'startingNonce', 'latestNonce', 'pendingNonce', 'balanceWei', 'attestationDigest'], 'deployer');
  if (input.deployer.codeType !== 'eoa' || input.deployer.runtimeCodeHash !== EMPTY_CODE_HASH) throw new Error('deployer must be an observed EOA');
  const startingNonce = uintString(input.deployer.startingNonce, 'deployer starting nonce', UINT64_MAX);
  if (uintString(input.deployer.latestNonce, 'deployer latest nonce', UINT64_MAX) !== startingNonce || uintString(input.deployer.pendingNonce, 'deployer pending nonce', UINT64_MAX) !== startingNonce) throw new Error('deployer nonce observation mismatch');
  if (uintString(input.deployer.balanceWei, 'deployer balance') < 1n) throw new Error('deployer balance unavailable');
  const deployer = { ...input.deployer, address: address(input.deployer.address, 'deployer address'), attestationDigest: bytes32(input.deployer.attestationDigest, 'deployer attestation digest') };
  exactKeys(input.roles, Object.keys(ROLE_RULES), 'roles');
  const roles = Object.fromEntries(Object.keys(ROLE_RULES).map((name) => [name, role(input.roles[name], name)]));
  const governanceDomain = ['governanceOwner', 'operator', 'pauseGuardian', 'unpauseTimelock'].map((name) => roles[name].address.toLowerCase());
  if (new Set(governanceDomain).size !== governanceDomain.length) throw new Error('governance role addresses must be distinct');
  if (roles.platformFeeReceiver.address.toLowerCase() === roles.projectProceedsReceiver.address.toLowerCase()) throw new Error('platform fee and project proceeds receivers must be distinct');
  exactKeys(input.platformFee, ['bps', 'decisionDigest', 'decisionVersion'], 'platform fee');
  if (!Number.isSafeInteger(input.platformFee.bps) || input.platformFee.bps < 0 || input.platformFee.bps > 1000) throw new Error('platform fee bps unresolved or invalid');
  const platformFee = { bps: input.platformFee.bps, decisionDigest: bytes32(input.platformFee.decisionDigest, 'platform fee decision digest'), decisionVersion: positiveInteger(input.platformFee.decisionVersion, 'platform fee decision version', 0xffffffff) };
  return {
    ...input,
    observation: { ...input.observation },
    deployer,
    roles,
    platformFee,
    startingNonce,
  };
}
function linkBytecode(bytecode, references, libraries, label) {
  let body = bytecode.slice(2);
  for (const [source, names] of Object.entries(references || {})) {
    for (const [name, rows] of Object.entries(names)) {
      const linked = libraries[name];
      if (!linked) throw new Error(`${label} missing library ${source}:${name}`);
      const replacement = linked.toLowerCase().slice(2);
      for (const row of rows) {
        if (row.length !== 20) throw new Error(`${label} library reference length invalid`);
        body = `${body.slice(0, row.start * 2)}${replacement}${body.slice((row.start + row.length) * 2)}`;
      }
    }
  }
  if (/[^0-9a-f]/i.test(body)) throw new Error(`${label} unresolved bytecode reference`);
  return `0x${body}`;
}
async function artifact(name) {
  const value = await artifacts.readArtifact(name);
  if (value.contractName !== name || value.bytecode === '0x' || value.deployedBytecode === '0x') throw new Error(`${name} artifact invalid`);
  return value;
}
async function creationData(value, linkedBytecode, args) {
  const factory = new ethers.ContractFactory(value.abi, linkedBytecode);
  return (await factory.getDeployTransaction(...args)).data;
}
function replaceRuntimeReferences(bytecode, references, replacements, label) {
  let body = bytecode.slice(2);
  for (const [id, rows] of Object.entries(references)) {
    const replacement = replacements[id];
    if (!replacement) throw new Error(`${label} immutable ${id} replacement unavailable`);
    const word = ethers.zeroPadValue(replacement, 32).slice(2).toLowerCase();
    for (const row of rows) {
      if (row.length !== 32) throw new Error(`${label} immutable reference length invalid`);
      body = `${body.slice(0, row.start * 2)}${word}${body.slice((row.start + row.length) * 2)}`;
    }
  }
  return `0x${body}`;
}
function walkAst(value, visit) {
  if (!value || typeof value !== 'object') return;
  visit(value);
  for (const child of Object.values(value)) {
    if (Array.isArray(child)) child.forEach((entry) => walkAst(entry, visit));
    else walkAst(child, visit);
  }
}
async function dispatcherRuntime(artifactValue, addresses) {
  const build = await artifacts.getBuildInfo(`${artifactValue.sourceName}:${artifactValue.contractName}`);
  if (!build) throw new Error('dispatcher build info unavailable');
  const output = build.output.contracts[artifactValue.sourceName][artifactValue.contractName];
  const references = output.evm.deployedBytecode.immutableReferences;
  const ids = {};
  walkAst(build.output.sources[artifactValue.sourceName].ast, (node) => {
    if (node.nodeType === 'VariableDeclaration' && node.mutability === 'immutable') ids[String(node.id)] = node.name;
  });
  const replacements = {};
  let inheritedSelfCount = 0;
  for (const id of Object.keys(references)) {
    if (ids[id]) {
      const key = ids[id].replace(/Facet$/, '');
      if (!addresses[key]) throw new Error(`dispatcher immutable ${ids[id]} address unavailable`);
      replacements[id] = addresses[key];
    } else {
      replacements[id] = addresses.dispatcher;
      inheritedSelfCount += 1;
    }
  }
  if (inheritedSelfCount !== 1 || Object.keys(ids).length !== 5 || Object.keys(references).length !== 6) throw new Error('dispatcher immutable reference shape invalid');
  return replaceRuntimeReferences(artifactValue.deployedBytecode, references, replacements, 'dispatcher');
}
function sourceIdentity() {
  const sourceFiles = SOURCE_FILES.map((file) => ({ path: file, sha256: crypto.createHash('sha256').update(fs.readFileSync(path.join(ROOT, file))).digest('hex') }));
  return { sourceFiles, deploymentSourceTreeDigest: digest(sourceFiles) };
}
async function buildUnsignedDeploymentBundle(rawInput) {
  const input = validateInput(rawInput);
  const compiler = await verifyProductionCompilerSettings();
  if (canonicalJson(compiler) !== canonicalJson(PRODUCTION_COMPILER)) throw new Error('production compiler identity mismatch');
  const approvalSource = currentSourceIdentity();
  const sources = sourceIdentity();
  const artifactRows = Object.fromEntries(await Promise.all(DEPLOYMENTS.map(async ([key, name]) => [key, await artifact(name)])));
  const addresses = {};
  for (let index = 0; index < DEPLOYMENTS.length; index += 1) addresses[DEPLOYMENTS[index][0]] = ethers.getCreateAddress({ from: input.deployer.address, nonce: input.startingNonce + BigInt(index) });
  const libraries = { JoinCareFairLaunchV1Logic: addresses.logic, JoinCareFairLaunchV1Runtime: addresses.runtime };
  const initData = new ethers.Interface(artifactRows.dispatcher.abi).encodeFunctionData('initialize', [
    input.roles.governanceOwner.address,
    input.roles.operator.address,
    input.roles.pauseGuardian.address,
    input.roles.unpauseTimelock.address,
    input.roles.treasury.address,
    input.roles.platformFeeReceiver.address,
    input.platformFee.bps,
  ]);
  const transactions = [];
  const runtimeCode = {};
  for (let index = 0; index < DEPLOYMENTS.length; index += 1) {
    const [key, name] = DEPLOYMENTS[index];
    const value = artifactRows[key];
    const linkedCreation = linkBytecode(value.bytecode, value.linkReferences, libraries, name);
    const linkedRuntime = linkBytecode(value.deployedBytecode, value.deployedLinkReferences, libraries, name);
    let args = [];
    if (key === 'dispatcher') args = [addresses.core, addresses.release, addresses.funds, addresses.admin, addresses.views];
    if (key === 'proxy') args = [addresses.dispatcher, initData];
    const data = await creationData(value, linkedCreation, args);
    transactions.push({ order: index + 1, nonce: String(input.startingNonce + BigInt(index)), kind: 'contract_creation', contractKey: key, contractName: name, predictedAddress: addresses[key], constructorArguments: args, valueRaw: '0', data, dataKeccak256: ethers.keccak256(data) });
    runtimeCode[key] = linkedRuntime;
  }
  runtimeCode.dispatcher = await dispatcherRuntime(artifactRows.dispatcher, addresses);
  const componentOrder = ['dispatcher', 'core', 'release', 'funds', 'admin', 'views', 'logic', 'runtime'];
  const components = componentOrder.map((key) => ({ key, name: artifactRows[key].contractName, address: addresses[key], runtimeCodeHash: ethers.keccak256(runtimeCode[key]) }));
  const abi = await verifyAggregateAbi();
  const selectorMap = await verifyActualRouting();
  const storageLayout = require('../audit/golden/storage-layout.golden.json');
  const fullSetIdentity = { schemaVersion: 'fair-launch-full-set-v1', compiler, components, aggregateAbiDigest: digest(sortedWireAbi(abi)), selectorMapDigest: digest(selectorMap), storageLayoutDigest: digest(storageLayout) };
  const fullSetManifest = { ...fullSetIdentity, fullSetDigest: digest(fullSetIdentity) };
  const payload = {
    schemaVersion: BUNDLE_SCHEMA,
    domain: BUNDLE_DOMAIN,
    status: STATUS,
    broadcastPermitted: false,
    environment: 'bsc-mainnet',
    chainId: 56,
    changeTicket: input.changeTicket,
    observation: input.observation,
    deployer: { ...input.deployer, startingNonce: String(input.startingNonce) },
    roles: input.roles,
    upgradeAuthority: input.roles.governanceOwner,
    platformFee: input.platformFee,
    source: { contractCommit: approvalSource.sourceCommit, approvalSourceTreeDigest: approvalSource.sourceTreeDigest, ...sources },
    compiler,
    initialization: { implementationAddress: addresses.dispatcher, proxyAddress: addresses.proxy, calldata: initData, calldataKeccak256: ethers.keccak256(initData) },
    erc1967ImplementationSlot: '0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc',
    transactionCount: transactions.length,
    finalNonceExclusive: String(input.startingNonce + BigInt(transactions.length)),
    transactions,
    fullSetManifest,
    proxy: { contractName: artifactRows.proxy.contractName, address: addresses.proxy, implementationAddress: addresses.dispatcher, runtimeCodeHash: ethers.keccak256(runtimeCode.proxy) },
  };
  return { ...payload, bundleDigest: digest(payload) };
}

module.exports = { BUNDLE_DOMAIN, BUNDLE_SCHEMA, EMPTY_CODE_HASH, INPUT_SCHEMA, STATUS, buildUnsignedDeploymentBundle, parseJsonNoDuplicateKeys, validateInput };
