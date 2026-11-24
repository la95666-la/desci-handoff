'use strict';
const fs = require('fs');
const path = require('path');
const assert = require('assert');
const crypto = require('crypto');
const { keccak256, toUtf8Bytes, zeroPadValue } = require('ethers');

const root = path.resolve(__dirname, '../..');
const dir = path.resolve(process.argv[2] || path.join(root, '..', '..', '..', '..', 'joincare-deploy', 'evidence', 'fair-launch', 'contract-baseline'));
const goldenDir = path.join(__dirname, 'golden');
const readAt = (base, name) => JSON.parse(fs.readFileSync(path.join(base, name), 'utf8'));
const read = name => readAt(dir, name);
const provenance = read('provenance.json');
const functions = read('function-selectors.json');
const events = read('event-topics.json');
const errors = read('custom-error-selectors.json');
const layout = read('storage-layout.json');

assert.equal(provenance.status, 'AUDIT_ONLY_NON_PRODUCTION_SPECIMEN');
assert.equal(provenance.compiler, '0.8.22+commit.4fc1097e');
assert.deepEqual(provenance.optimizer, { enabled: true, runs: 200 });
assert.equal(provenance.viaIR, true);
assert.equal(provenance.evmVersion, 'paris');
assert.deepEqual(provenance.openzeppelin, { contracts: '5.1.0', contractsUpgradeable: '5.1.0' });
assert.equal(provenance.validationLayoutVersion, '1.2');
assert.match(provenance.validationContractVersionWithoutMetadata, /^[0-9a-f]{64}$/);
assert.match(provenance.buildInfoSha256, /^[0-9a-f]{64}$/);
assert.match(provenance.validationsSha256, /^[0-9a-f]{64}$/);
assert.equal(provenance.counts.ownedEvents, 32);
assert.equal(provenance.counts.ownedErrors, 56);
assert(!errors.some(x => x.signature === 'MechanicalArtifactOnly()'));
assert(!functions.some(x => x.signature === 'MAX_FEE_RATE()'));

const expectedMutability = {
  'calculateFinalAllocation(uint256,address)': 'view',
  'calculateClaimable(uint256,address)': 'view',
  'projectTokenRuntimeMatches(uint256)': 'view',
  'paymentTokenRuntimeMatches(uint256)': 'view'
};
for (const [signature, stateMutability] of Object.entries(expectedMutability)) {
  const item = functions.find(x => x.signature === signature);
  assert(item, signature);
  assert.equal(item.stateMutability, stateMutability, signature);
}
for (const sig of [
  'releaseSuccessfulProject(uint256,uint256,uint256,bytes32,bytes32,uint32,bytes32)',
  'registerPaymentTokenPolicyRevision(address,(bool,uint8,uint32,bytes32,bytes32))',
  'invalidatePendingRelease(uint256,uint8,bytes32,bytes32)'
]) assert(functions.some(x => x.signature === sig), sig);
for (const sig of [
  'Initialized(uint64)',
  'OwnershipTransferStarted(address,address)',
  'OwnershipTransferred(address,address)',
  'Upgraded(address)'
]) assert(events.some(x => x.signature === sig && !x.owned), sig);

// These files are static reviewer-owned golden snapshots. The generator never
// writes them, so additions, removals, mutability changes, tuple drift and
// storage changes all fail as exhaustive structural diffs.
for (const [actual, golden] of [
  ['abi.json', 'abi.golden.json'],
  ['function-selectors.json', 'function-selectors.golden.json'],
  ['event-topics.json', 'event-topics.golden.json'],
  ['custom-error-selectors.json', 'custom-error-selectors.golden.json'],
  ['storage-layout.json', 'storage-layout.golden.json']
]) assert.deepStrictEqual(read(actual), readAt(goldenDir, golden), `${actual}: golden drift`);

const rootLabels = [
  'operator', 'pauseGuardian', 'unpauseTimelock', 'treasury', 'platformFeeReceiver',
  'platformFeeBps', 'contributionsPaused', 'payoutsPaused', 'claimsPauseUntil',
  'paymentTokenAdmissions', 'paymentTokenPolicyRevisions', 'projectTokenPolicies',
  'reservedBalances', 'approvedBeneficiarySafes', 'beneficiarySafeAttestations',
  'beneficiarySafeAttestationVersions', 'releasePolicies', 'releasePolicyVersions',
  'projects', 'positions', 'hasParticipated', 'vestingTimes', 'vestingBps', '__gap'
];
assert.deepEqual(layout.members.map(x => x.label), rootLabels);
assert.equal(layout.members.at(-1).type.label, 'uint256[32]');

function findType(node, suffix) {
  if (node.label && node.label.endsWith(suffix)) return node;
  if (node.members) for (const member of node.members) {
    if (typeof member !== 'string') { const found = findType(member.type, suffix); if (found) return found; }
  }
  for (const child of [node.key, node.value, node.base]) { const found = child && findType(child, suffix); if (found) return found; }
  return null;
}
const revisions = layout.members.find(x => x.label === 'paymentTokenPolicyRevisions').type;
assert.equal(revisions.key.label, 'address');
assert.equal(revisions.value.key.label, 'uint32');
assert(revisions.value.value.label.endsWith('.PaymentTokenPolicyRevision'));
const positions = layout.members.find(x => x.label === 'positions').type;
assert.equal(positions.key.label, 'uint256');
assert.equal(positions.value.key.label, 'address');
assert(positions.value.value.label.endsWith('.UserPosition'));

const projectType = findType(layout.members.find(x => x.label === 'projects').type, '.Project');
assert(projectType);
const settlement = [
  'settlementBlockNumber', 'settlementAnchorBlockNumber', 'settlementAnchorBlockHash',
  'settlementTxHash', 'settlementTxLogOrdinal', 'releaseManifestDigest'
];
const labels = projectType.members.map(x => x.label);
assert.deepEqual(labels.slice(labels.indexOf(settlement[0]), labels.indexOf(settlement[0]) + settlement.length), settlement);

const namespaceHashMinusOne = BigInt(keccak256(toUtf8Bytes('joincare.storage.JoinCareFairLaunchV1'))) - 1n;
const encoded = zeroPadValue(`0x${namespaceHashMinusOne.toString(16)}`, 32);
const slot = `0x${(BigInt(keccak256(encoded)) & ~0xffn).toString(16).padStart(64, '0')}`;
assert.equal(slot, layout.namespaceSlot);

for (const line of fs.readFileSync(path.join(dir, 'SHA256SUMS'), 'utf8').trim().split('\n')) {
  const [want, name] = line.split(/  /);
  const got = crypto.createHash('sha256').update(fs.readFileSync(path.join(dir, name))).digest('hex');
  assert.equal(got, want, name);
}
console.log('Fair Launch contract baseline: PASS');
