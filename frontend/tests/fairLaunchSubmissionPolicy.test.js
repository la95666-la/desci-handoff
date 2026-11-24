import test from 'node:test'
import assert from 'node:assert/strict'
import { keccak256, sha256 } from 'ethers'
import {
  evaluateFairLaunchLocalAdmissionEvidence,
  evaluateFairLaunchSubmissionPolicy,
  isExactFairLaunchLocalFrontendOrigin,
  isExactFairLaunchLocalRpcUrl,
  normalizeFairLaunchLocalManifest,
  verifyFairLaunchLocalDeployment,
} from '../src/services/fairLaunchSubmissionPolicy.js'

const code = '0x6001600055'
const codeHash = keccak256(code)
const address = (digit) => `0x${digit.repeat(40)}`
const addresses = {
  proxy: address('1'),
  implementation: address('2'),
  payment: address('a'),
  project: address('b'),
}
const identityNames = [
  'contract.proxy', 'contract.dispatcher', 'contract.coreFacet', 'contract.releaseFacet',
  'contract.fundsFacet', 'contract.adminFacet', 'contract.viewsFacet',
  'contract.logicLibrary', 'contract.runtimeLibrary', 'asset.paymentToken',
  'asset.projectToken.successful', 'safe.proceeds', 'safe.platformFee',
]
const identityAddresses = ['1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd']
const deploymentIdentities = identityNames.map((identity, index) => ({
  identity,
  address: address(identityAddresses[index]),
  runtimeCodeHash: codeHash,
})).sort((a, b) => a.identity.localeCompare(b.identity))
const deploymentIdentitiesJson = JSON.stringify(deploymentIdentities.map(({ identity, address: value, runtimeCodeHash }) => ({ address: value, identity, runtimeCodeHash })))
const deploymentIdentitiesDigest = sha256(new TextEncoder().encode(deploymentIdentitiesJson))
const blockHash = `0x${'e'.repeat(64)}`
const nowMs = Date.parse('2026-09-10T12:00:00Z')
const build = {
  dev: true,
  prod: false,
  environment: 'local',
  mode: 'local_test',
  chainId: '31337',
  optIn: 'true',
  rpcUrl: 'http://127.0.0.1:8545',
  manifestRpcUrl: 'http://127.0.0.1:8545',
  manifestFrontendOrigin: 'http://127.0.0.1:4173',
  manifestChainId: '31337',
  manifestProjectId: '7',
  manifestProxyAddress: addresses.proxy,
  manifestImplementationAddress: addresses.implementation,
  manifestImplementationCodeHash: codeHash,
  manifestPaymentTokenAddress: addresses.payment,
  manifestProjectTokenAddress: addresses.project,
  manifestDeploymentIdentities: deploymentIdentitiesJson,
  manifestDeploymentIdentitiesDigest: deploymentIdentitiesDigest,
  manifestPayloadSha256: 'a'.repeat(64),
  manifestSigningKeyId: 'local-e2e-0123456789abcdef01234567',
  manifestObservationBlockNumber: '9',
  manifestObservationBlockHash: blockHash,
  manifestRunId: 'run-20260910-local-001',
  manifestCreatedAt: '2026-09-10T11:00:00Z',
  manifestExpiresAt: '2026-09-10T13:00:00Z',
	apiUrl: 'http://127.0.0.1:19001',
	apiRunId: 'run-20260910-local-001',
	apiToken: 'f'.repeat(64),
	apiExpiresAt: '2026-09-10T12:30:00Z',
}
const project = {
  id: 7,
  fundingMode: 'fair_launch',
  contractVersion: 'fair_launch_v1',
  chainId: 31337,
  contractAddress: addresses.proxy,
  paymentTokenAddress: addresses.payment,
  tokenAddress: addresses.project,
  runtimeIdentity: {
    chainId: 31337,
    proxyAddress: addresses.proxy,
    implementationAddress: addresses.implementation,
    implementationCodeHash: codeHash,
    observedBlockNumber: '9',
    observedBlockHash: blockHash,
    runtimeVerified: true,
  },
}
const origin = { origin: 'http://127.0.0.1:4173' }
const inputs = (overrides = {}) => ({
  build: { ...build, ...(overrides.build || {}) },
  origin: overrides.origin === undefined ? origin : overrides.origin,
  project: { ...project, ...(overrides.project || {}) },
  nowMs: overrides.nowMs ?? nowMs,
})
const evidence = (overrides = {}) => evaluateFairLaunchLocalAdmissionEvidence(inputs(overrides))
const decide = (overrides = {}) => evaluateFairLaunchSubmissionPolicy(inputs(overrides))

function unavailable(result) {
  assert.deepEqual(result, { enabled: false, reason: 'durable_wallet_submission_unavailable' })
}

test('fresh local evidence plus run-bound Backend HTTP authority enables only the dedicated local submission lane', () => {
  const result = evidence()
  assert.equal(result.enabled, true)
  assert.equal(result.mode, 'local_hardhat_ephemeral')
  assert.equal(result.manifest.frontendOrigin, 'http://127.0.0.1:4173')
  assert.equal(result.manifest.deploymentIdentities.length, 13)
  assert.equal(result.manifest.payloadSha256, 'a'.repeat(64))
  assert.equal(result.manifest.signingKeyId, 'local-e2e-0123456789abcdef01234567')
	  const policy = decide()
	  assert.equal(policy.enabled, true)
	  assert.equal(policy.apiUrl, 'http://127.0.0.1:19001')
	  assert.equal(policy.apiRunId, 'run-20260910-local-001')
  assert.equal(isExactFairLaunchLocalRpcUrl('http://127.0.0.1:8545'), true)
  assert.equal(isExactFairLaunchLocalFrontendOrigin('http://127.0.0.1:4173'), true)
})

test('local evidence rejects browser/RPC aliases, missing explicit ports and exact-port mismatches', () => {
  for (const rpc of ['http://localhost:8545', 'http://[::1]:8545', 'http://127.0.0.1', 'http://127.0.0.1:8545/rpc']) {
    assert.equal(isExactFairLaunchLocalRpcUrl(rpc), false)
    unavailable(evidence({ build: { rpcUrl: rpc } }))
  }
  for (const value of ['http://localhost:4173', 'http://[::1]:4173', 'http://127.0.0.1', 'http://127.0.0.1:4173/path']) {
    assert.equal(isExactFairLaunchLocalFrontendOrigin(value), false)
    unavailable(evidence({ origin: { origin: value } }))
  }
  unavailable(evidence({ origin: { origin: 'http://127.0.0.1:4174' } }))
  unavailable(evidence({ origin: { origin: build.rpcUrl } }))
  unavailable(evidence({ origin: null }))
})

test('local evidence rejects production, stale or incomplete signed/build identity evidence', () => {
  for (const patch of [
    { dev: false }, { prod: true }, { environment: 'prod' }, { mode: '' }, { chainId: 56 }, { optIn: 'false' },
    { manifestRpcUrl: 'http://127.0.0.1:9545' }, { manifestChainId: 56 },
    { manifestPayloadSha256: '' }, { manifestSigningKeyId: '' },
    { manifestDeploymentIdentities: '' }, { manifestDeploymentIdentitiesDigest: `0x${'f'.repeat(64)}` },
    { manifestProxyAddress: '' }, { manifestObservationBlockNumber: '' },
  ]) unavailable(evidence({ build: patch }))
  unavailable(evidence({ nowMs: Date.parse('2026-09-10T13:00:00Z') }))
  unavailable(evidence({ nowMs: Date.parse('2026-09-10T10:00:00Z') }))
  for (const patch of [
    { fundingMode: 'capped_presale' }, { contractVersion: 'v2_stablecoin' }, { chainId: 56 },
    { contractAddress: addresses.implementation }, { paymentTokenAddress: addresses.project },
    { tokenAddress: addresses.payment }, { runtimeIdentity: { ...project.runtimeIdentity, runtimeVerified: false } },
    { runtimeIdentity: { ...project.runtimeIdentity, observedBlockNumber: '8' } },
  ]) unavailable(evidence({ project: patch }))
})

test('wallet submission authority rejects missing, stale, aliased and cross-run Backend capability', () => {
	for (const patch of [
		{ apiUrl: '' }, { apiUrl: 'http://localhost:19001' }, { apiUrl: build.rpcUrl },
		{ apiUrl: build.manifestFrontendOrigin }, { apiRunId: 'other-run-20260910' },
		{ apiToken: '' }, { apiToken: 'A'.repeat(64) }, { apiExpiresAt: '2026-09-10T11:59:59Z' },
		{ apiExpiresAt: '2026-09-10T13:30:00Z' },
	]) unavailable(decide({ build: patch }))
})

test('manifest normalizer has no permissive defaults', () => {
  assert.equal(normalizeFairLaunchLocalManifest({}), null)
  assert.equal(normalizeFairLaunchLocalManifest(null), null)
})

test('live deployment verification checks expected runtime hash for every represented identity and observed block', async () => {
  const policy = evidence()
  const provider = {
    getCode: async () => code,
    getBlock: async (number) => number === 9 ? { hash: blockHash } : null,
  }
  assert.equal(await verifyFairLaunchLocalDeployment(provider, policy, project, build.rpcUrl), true)
  assert.equal(await verifyFairLaunchLocalDeployment({ ...provider, getCode: async () => '0x' }, policy, project, build.rpcUrl), false)
  assert.equal(await verifyFairLaunchLocalDeployment({
    ...provider,
    getCode: async (candidate) => candidate === address('7') ? '0x6002' : code,
  }, policy, project, build.rpcUrl), false)
  assert.equal(await verifyFairLaunchLocalDeployment(provider, policy, project, 'http://localhost:8545'), false)
})
