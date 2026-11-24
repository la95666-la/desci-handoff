import test from 'node:test'
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import { frontendEnvFromChainManifest } from '../scripts/fair-launch-local-e2e-env.js'

const address = (digit) => `0x${digit.repeat(40)}`
const hash = (digit) => `0x${digit.repeat(64)}`
const contractKeys = ['proxy', 'dispatcher', 'coreFacet', 'releaseFacet', 'fundsFacet', 'adminFacet', 'viewsFacet', 'logicLibrary', 'runtimeLibrary']

function canonical(value) {
  if (Array.isArray(value)) return value.map(canonical)
  if (value && typeof value === 'object') return Object.fromEntries(Object.keys(value).sort().map((key) => [key, canonical(value[key])]))
  return value
}

function signedManifest(payloadOverrides = {}, envelopeMutator = null) {
  const contracts = Object.fromEntries(contractKeys.map((key, index) => [key, {
    address: address((index + 1).toString(16)),
    runtimeCodeHash: hash(((index + 1) % 15 + 1).toString(16)),
  }]))
  const payload = {
    schemaVersion: 'fair-launch-local-e2e-deployment-v1',
    authority: 'TEST_ONLY_NOT_PRODUCTION_ADMITTED',
    runId: 'fl-local-env-0001',
    createdAt: '2026-09-10T11:00:00Z',
    expiresAt: '2026-09-10T12:00:00Z',
    network: { chainId: '31337', rpcUrl: 'http://127.0.0.1:8545' },
    contracts,
    assets: {
      paymentToken: { address: address('a'), runtimeCodeHash: hash('a') },
      projectTokens: [
        { key: 'failed', address: address('b'), runtimeCodeHash: hash('b') },
        { key: 'successful', address: address('c'), runtimeCodeHash: hash('c') },
      ],
    },
    safes: {
      proceeds: { address: address('d'), runtimeCodeHash: hash('d') },
      platformFee: { address: address('e'), runtimeCodeHash: hash('e') },
    },
    projects: [{ key: 'successful', projectId: '1002', projectToken: address('c'), paymentToken: address('a') }],
    deployment: { observationBlockNumber: '23', observationBlockHash: hash('f') },
    ...payloadOverrides,
  }
  const { privateKey, publicKey } = crypto.generateKeyPairSync('ed25519')
  const publicJwk = publicKey.export({ format: 'jwk' })
  const keyId = `local-e2e-${crypto.createHash('sha256').update(publicKey.export({ type: 'spki', format: 'der' })).digest('hex').slice(0, 24)}`
  const canonicalPayload = JSON.stringify(canonical(payload))
  const envelope = {
    payload,
    signature: {
      algorithm: 'ed25519',
      keyId,
      payloadSha256: crypto.createHash('sha256').update(canonicalPayload).digest('hex'),
      value: crypto.sign(null, Buffer.from(canonicalPayload), privateKey).toString('base64'),
    },
    testTrustRoot: {
      authority: 'TEST_ONLY_NOT_PRODUCTION_ADMITTED',
      keyId,
      publicKeyJwk: publicJwk,
    },
  }
  envelopeMutator?.(envelope)
  return envelope
}

const apiAuthority = (overrides = {}) => ({
  authority: 'TEST_ONLY_NOT_PRODUCTION_ADMITTED',
  runId: 'fl-local-env-0001',
  baseUrl: 'http://127.0.0.1:19001',
  frontendOrigin: 'http://127.0.0.1:4173',
  adminOrigin: 'http://127.0.0.1:5666',
  token: 'f'.repeat(64),
  expiresAt: '2026-09-10T11:30:00Z',
  status: 'LOCAL_ONLY_HTTP_ADMITTED',
  ...overrides,
})
const generate = (envelope = signedManifest(), origin = 'http://127.0.0.1:4173', authority = apiAuthority({ frontendOrigin: origin })) =>
  frontendEnvFromChainManifest(envelope, 'successful', origin, authority)

test('generator verifies Contract-format Ed25519 envelope and binds signed identity evidence', () => {
  const envelope = signedManifest()
  const output = generate(envelope)
  assert.match(output, /VITE_FAIR_LAUNCH_TEST_MODE=local_test/)
  assert.match(output, /VITE_HARDHAT_RPC=http:\/\/127\.0\.0\.1:8545/)
  assert.match(output, /VITE_FAIR_LAUNCH_LOCAL_FRONTEND_ORIGIN=http:\/\/127\.0\.0\.1:4173/)
  assert.match(output, new RegExp(`VITE_FAIR_LAUNCH_LOCAL_MANIFEST_PAYLOAD_SHA256=${envelope.signature.payloadSha256}`))
  assert.match(output, new RegExp(`VITE_FAIR_LAUNCH_LOCAL_MANIFEST_KEY_ID=${envelope.signature.keyId}`))
  assert.match(output, /VITE_FAIR_LAUNCH_LOCAL_DEPLOYMENT_IDENTITIES_DIGEST=0x[0-9a-f]{64}/)
  assert.match(output, /"identity":"contract\.proxy"/)
  assert.match(output, /"identity":"contract\.coreFacet"/)
  assert.match(output, /"identity":"safe\.platformFee"/)
  assert.match(output, /VITE_FAIR_LAUNCH_LOCAL_OBSERVATION_BLOCK_NUMBER=23/)
  assert.match(output, /VITE_FAIR_LAUNCH_LOCAL_API_URL=http:\/\/127\.0\.0\.1:19001/)
  assert.match(output, /VITE_FAIR_LAUNCH_LOCAL_API_TOKEN=[0-9a-f]{64}/)
  assert.doesNotMatch(output, /private.?key|mnemonic|signature.value/i)
})

test('generator rejects missing, tampered and unknown Contract-format signatures', () => {
  assert.throws(() => generate(signedManifest({}, (envelope) => { delete envelope.signature })), /signature is missing/)
  assert.throws(() => generate(signedManifest({}, (envelope) => { envelope.payload.runId = 'tampered-run-0001' })), /signed payload digest mismatch/)
  assert.throws(() => generate(signedManifest({}, (envelope) => { envelope.signature.value = Buffer.alloc(64).toString('base64') })), /signature verification failed/)
  assert.throws(() => generate(signedManifest({}, (envelope) => { envelope.testTrustRoot.keyId = 'local-e2e-000000000000000000000000' })), /signature key is unknown/)
  assert.throws(() => generate(signedManifest({}, (envelope) => { envelope.signature.keyId = 'local-e2e-000000000000000000000000' })), /signature key is unknown/)
  assert.throws(() => generate(signedManifest({}, (envelope) => { delete envelope.testTrustRoot })), /trust root is missing/)
})

test('generator rejects RPC/browser aliases, implicit or wrong ports, and invalid manifest identities', () => {
  assert.throws(() => generate(signedManifest({ network: { chainId: '31337', rpcUrl: 'http://localhost:8545' } })), /run RPC URL/)
  assert.throws(() => generate(signedManifest({ network: { chainId: '31337', rpcUrl: 'http://127.0.0.1' } })), /run RPC URL/)
  for (const origin of ['http://localhost:4173', 'http://[::1]:4173', 'http://127.0.0.1', 'http://127.0.0.1:4173/path']) {
    assert.throws(() => generate(signedManifest(), origin), /frontend origin/)
  }
  assert.throws(() => generate(signedManifest(), 'http://127.0.0.1:8545'), /distinct explicit ports/)
  assert.throws(() => generate(signedManifest({ authority: 'production' })), /authority/)
  assert.throws(() => generate(signedManifest({ expiresAt: '2026-09-12T12:00:00Z' })), /validity window/)
  assert.throws(() => generate(signedManifest({ deployment: { observationBlockNumber: '0', observationBlockHash: hash('f') } })), /observationBlockNumber/)
})

test('generator requires hashes for every Contract-manifest security-relevant deployment', () => {
  const missingFacet = signedManifest()
  delete missingFacet.payload.contracts.coreFacet.runtimeCodeHash
  assert.throws(() => generate(missingFacet), /signed payload digest mismatch/)

  const unsignedMissingFacet = signedManifest({
    contracts: {
      proxy: { address: address('1'), runtimeCodeHash: hash('1') },
      dispatcher: { address: address('2'), runtimeCodeHash: hash('2') },
    },
  })
  assert.throws(() => generate(unsignedMissingFacet), /contract\.coreFacet address/)

  const missingSafeHash = signedManifest({
    safes: {
      proceeds: { address: address('d'), runtimeCodeHash: hash('d') },
      platformFee: { address: address('e') },
    },
  })
  assert.throws(() => generate(missingSafeHash), /safe\.platformFee runtime code hash/)
})

test('generator rejects missing, stale, aliased and cross-run Backend HTTP authority', () => {
  assert.throws(() => frontendEnvFromChainManifest(signedManifest(), 'successful', 'http://127.0.0.1:4173', null), /authority is missing/)
  assert.throws(() => generate(signedManifest(), 'http://127.0.0.1:4173', apiAuthority({ baseUrl: 'http://localhost:19001' })), /Backend HTTP URL/)
  assert.throws(() => generate(signedManifest(), 'http://127.0.0.1:4173', apiAuthority({ runId: 'other-local-run' })), /binding/)
  assert.throws(() => generate(signedManifest(), 'http://127.0.0.1:4173', apiAuthority({ expiresAt: '2026-09-10T12:30:00Z' })), /interval/)
})
