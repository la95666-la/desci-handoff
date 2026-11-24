'use strict';
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const { id } = require('ethers');

const root = path.resolve(__dirname, '../..');
const buildInfoDir = path.join(root, 'artifacts-audit', 'build-info');
const targetSource = 'contracts-audit/JoinCareFairLaunchV1ArtifactBaseline.sol';
const targetContract = 'JoinCareFairLaunchV1ArtifactBaseline';
const outDir = path.resolve(process.argv[2] || path.join(root, '..', '..', '..', '..', 'joincare-deploy', 'evidence', 'fair-launch', 'contract-baseline'));
const namespace = 'erc7201:joincare.storage.JoinCareFairLaunchV1';
const namespaceSlot = '0x7ebb4c309dc14c823b32233ea7cfb602ee106e1bc7e105581a751cd0bcb46700';

function sha256(data) { return crypto.createHash('sha256').update(data).digest('hex'); }
function stable(value) {
  if (Array.isArray(value)) return value.map(stable);
  if (value && typeof value === 'object') return Object.fromEntries(Object.keys(value).sort().map(k => [k, stable(value[k])]));
  return value;
}
function writeJson(name, value) { fs.writeFileSync(path.join(outDir, name), JSON.stringify(stable(value), null, 2) + '\n'); }
function canonicalType(input) {
  if (input.type.startsWith('tuple')) {
    const suffix = input.type.slice('tuple'.length);
    return `(${input.components.map(canonicalType).join(',')})${suffix}`;
  }
  return input.type;
}
function signature(item) { return `${item.name}(${(item.inputs || []).map(canonicalType).join(',')})`; }
function splitTopLevelPair(value) {
  let depth = 0;
  for (let i = 0; i < value.length; i += 1) {
    if (value[i] === '(') depth += 1;
    else if (value[i] === ')') depth -= 1;
    else if (value[i] === ',' && depth === 0) return [value.slice(0, i), value.slice(i + 1)];
  }
  throw new Error(`cannot split type pair: ${value}`);
}
function normalizeType(typeId, types, seen = new Set()) {
  const t = types[typeId] || { label: typeId, numberOfBytes: null };
  const result = { label: t.label, numberOfBytes: t.numberOfBytes };
  if (seen.has(typeId)) return result;
  const nextSeen = new Set(seen); nextSeen.add(typeId);
  if (t.members) result.members = t.members.map(m => typeof m === 'string' ? m : ({ label: m.label, slot: m.slot, offset: m.offset, type: normalizeType(m.type, types, nextSeen) }));
  if (typeId.startsWith('t_mapping(') && typeId.endsWith(')')) {
    const [key, value] = splitTopLevelPair(typeId.slice(10, -1));
    result.key = normalizeType(key, types, nextSeen);
    result.value = normalizeType(value, types, nextSeen);
  }
  const array = /^t_array\((.+)\)(?:dyn|[0-9]+)_storage$/.exec(typeId);
  if (array) result.base = normalizeType(array[1], types, nextSeen);
  return result;
}

const buildFiles = fs.readdirSync(buildInfoDir).filter(f => f.endsWith('.json'));
if (buildFiles.length !== 1) throw new Error(`expected exactly one audit build-info, got ${buildFiles.length}`);
const buildPath = path.join(buildInfoDir, buildFiles[0]);
const buildBytes = fs.readFileSync(buildPath);
const build = JSON.parse(buildBytes);
if (build.solcVersion !== '0.8.22') throw new Error(`unexpected solc ${build.solcVersion}`);
const output = build.output.contracts?.[targetSource]?.[targetContract];
if (!output) throw new Error('target contract missing from build-info');
const pkg = JSON.parse(fs.readFileSync(path.join(root, 'package.json')));
for (const dep of ['@openzeppelin/contracts', '@openzeppelin/contracts-upgradeable']) {
  if (pkg.devDependencies[dep] !== '5.1.0') throw new Error(`${dep} must be exactly 5.1.0`);
}
const validations = JSON.parse(fs.readFileSync(path.join(root, 'cache-audit', 'validations.json')));
let validation;
for (const entry of validations.log) {
  const key = `${targetSource}:${targetContract}`;
  if (entry[key]) validation = entry[key];
}
if (!validation) throw new Error('OpenZeppelin validation layout missing');
const layout = validation.layout;
const nsMembers = layout.namespaces?.[namespace];
if (!nsMembers) throw new Error(`namespace ${namespace} missing`);
const fairTypeEntry = Object.entries(layout.types).find(([, value]) => value.label.endsWith('.FairLaunchStorage'));
if (!fairTypeEntry) throw new Error('FairLaunchStorage type missing');
const [, fairType] = fairTypeEntry;
const semanticLayout = {
  namespace,
  namespaceSlot,
  layoutVersion: layout.layoutVersion,
  members: fairType.members.map(m => ({ label: m.label, slot: m.slot, offset: m.offset, type: normalizeType(m.type, layout.types) })),
  inheritedNamespaces: Object.keys(layout.namespaces).filter(x => x !== namespace).sort()
};

const abi = output.abi;
const functions = abi.filter(x => x.type === 'function').map(x => ({ signature: signature(x), selector: id(signature(x)).slice(0, 10), stateMutability: x.stateMutability })).sort((a,b) => a.signature.localeCompare(b.signature));
const events = abi.filter(x => x.type === 'event').map(x => ({ signature: signature(x), topic0: id(signature(x)), anonymous: x.anonymous, owned: x.name.startsWith('FairLaunch') })).sort((a,b) => a.signature.localeCompare(b.signature));
const errors = abi.filter(x => x.type === 'error').map(x => ({ signature: signature(x), selector: id(signature(x)).slice(0, 10), owned: !['AddressEmptyCode','ERC1967InvalidImplementation','ERC1967NonPayable','FailedCall','InvalidInitialization','NotInitializing','OwnableInvalidOwner','OwnableUnauthorizedAccount','ReentrancyGuardReentrantCall','UUPSUnauthorizedCallContext','UUPSUnsupportedProxiableUUID'].includes(x.name) })).sort((a,b) => a.signature.localeCompare(b.signature));

fs.mkdirSync(outDir, { recursive: true });
writeJson('abi.json', abi);
writeJson('function-selectors.json', functions);
writeJson('event-topics.json', events);
writeJson('custom-error-selectors.json', errors);
writeJson('storage-layout.json', semanticLayout);
const provenance = {
  status: 'AUDIT_ONLY_NON_PRODUCTION_SPECIMEN',
  source: targetSource,
  contract: targetContract,
  compiler: build.solcLongVersion,
  optimizer: build.input.settings.optimizer,
  viaIR: build.input.settings.viaIR,
  evmVersion: build.input.settings.evmVersion || null,
  openzeppelin: { contracts: '5.1.0', contractsUpgradeable: '5.1.0' },
  namespace,
  namespaceSlot,
  buildInfoSha256: sha256(buildBytes),
  validationsSha256: sha256(fs.readFileSync(path.join(root, 'cache-audit', 'validations.json'))),
  validationLayoutVersion: layout.layoutVersion,
  validationContractVersionWithoutMetadata: validation.version.withoutMetadata,
  compilerInputSha256: sha256(Buffer.from(JSON.stringify(stable(build.input)))),
  packageLockSha256: sha256(fs.readFileSync(path.join(root, 'package-lock.json'))),
  auditConfigSha256: sha256(fs.readFileSync(path.join(root, 'hardhat.audit.config.js'))),
  sourceSha256: sha256(fs.readFileSync(path.join(root, targetSource))),
  deployedBytecodeSha256Denylist: sha256(Buffer.from(output.evm.deployedBytecode.object, 'hex')),
  counts: { functions: functions.length, events: events.length, errors: errors.length, ownedEvents: events.filter(x => x.owned).length, ownedErrors: errors.filter(x => x.owned).length }
};
writeJson('provenance.json', provenance);
const hashes = fs.readdirSync(outDir).filter(x => x.endsWith('.json')).sort().map(name => `${sha256(fs.readFileSync(path.join(outDir, name)))}  ${name}`);
fs.writeFileSync(path.join(outDir, 'SHA256SUMS'), hashes.join('\n') + '\n');
console.log(`generated ${outDir}`);
