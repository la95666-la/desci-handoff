import crypto from 'node:crypto'
import { readFile, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'

const ADDRESS = /^0x[0-9a-fA-F]{40}$/
const HASH = /^0x[0-9a-fA-F]{64}$/
const SHA256 = /^[0-9a-f]{64}$/
const RUN_ID = /^[A-Za-z0-9][A-Za-z0-9._-]{7,127}$/
const KEY_ID = /^local-e2e-[0-9a-f]{24}$/
const MAX_LIFETIME_MS = 24 * 60 * 60 * 1000
const CONTRACT_IDENTITIES = Object.freeze([
  'proxy', 'dispatcher', 'coreFacet', 'releaseFacet', 'fundsFacet',
  'adminFacet', 'viewsFacet', 'logicLibrary', 'runtimeLibrary',
])

function fail(message) {
  throw new Error(`Fair Launch local E2E env: ${message}`)
}

function requireString(value, name, pattern = null) {
  const text = String(value ?? '').trim()
  if (!text || (pattern && !pattern.test(text))) fail(`${name} is invalid`)
  return text
}

function requireAddress(value, name) {
  const address = requireString(value, name, ADDRESS).toLowerCase()
  if (/^0x0{40}$/.test(address)) fail(`${name} is zero`)
  return address
}

function requireHash(value, name) {
  const hash = requireString(value, name, HASH).toLowerCase()
  if (/^0x0{64}$/.test(hash)) fail(`${name} is zero`)
  return hash
}

// This intentionally matches scripts/local/fairLaunchLocalE2E.js in the
// Contract repository. Changing canonicalization here would make otherwise
// valid Contract-generated signatures incompatible.
function canonical(value) {
  if (Array.isArray(value)) return value.map(canonical)
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.keys(value).sort().map((key) => [key, canonical(value[key])]))
  }
  return value
}

function canonicalJson(value) {
  return JSON.stringify(canonical(value))
}

function sha256(value) {
  return crypto.createHash('sha256').update(value).digest('hex')
}

function exactLoopbackUrl(value, name) {
  let url
  try {
    url = new URL(requireString(value, name))
  } catch {
    fail(`${name} is invalid`)
  }
  if (url.protocol !== 'http:' || url.hostname !== '127.0.0.1' || !/^\d+$/.test(url.port) ||
      Number(url.port) < 1 || Number(url.port) > 65535 || url.pathname !== '/' ||
      url.username || url.password || url.search || url.hash) {
    fail(`${name} must be an exact http://127.0.0.1:<explicit-port> URL`)
  }
  return url.origin
}

function verifyTestOnlyEnvelope(envelope) {
  if (!envelope || typeof envelope !== 'object' || Array.isArray(envelope)) fail('manifest envelope is missing')
  const payload = envelope.payload
  const signature = envelope.signature
  const trustRoot = envelope.testTrustRoot
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) fail('manifest payload is missing')
  if (!signature || typeof signature !== 'object' || Array.isArray(signature)) fail('manifest signature is missing')
  if (!trustRoot || typeof trustRoot !== 'object' || Array.isArray(trustRoot)) fail('approved local trust root is missing')
  if (trustRoot.authority !== 'TEST_ONLY_NOT_PRODUCTION_ADMITTED') fail('local trust root authority is invalid')
  if (signature.algorithm !== 'ed25519') fail('manifest signature algorithm is invalid')

  const keyId = requireString(signature.keyId, 'signature keyId', KEY_ID)
  if (requireString(trustRoot.keyId, 'trust root keyId', KEY_ID) !== keyId) fail('manifest signature key is unknown')
  const publicJwk = trustRoot.publicKeyJwk
  if (!publicJwk || typeof publicJwk !== 'object' || Array.isArray(publicJwk) ||
      publicJwk.kty !== 'OKP' || publicJwk.crv !== 'Ed25519' || typeof publicJwk.x !== 'string' || !publicJwk.x || 'd' in publicJwk) {
    fail('local trust root public key is invalid')
  }

  let publicKey
  try {
    publicKey = crypto.createPublicKey({ key: publicJwk, format: 'jwk' })
  } catch {
    fail('local trust root public key is invalid')
  }
  const derivedKeyId = `local-e2e-${sha256(publicKey.export({ type: 'spki', format: 'der' })).slice(0, 24)}`
  if (derivedKeyId !== keyId) fail('manifest signature key identity is invalid')

  const payloadCanonical = canonicalJson(payload)
  const payloadSha256 = sha256(payloadCanonical)
  if (requireString(signature.payloadSha256, 'signed payload digest', SHA256) !== payloadSha256) fail('signed payload digest mismatch')
  const signatureValue = requireString(signature.value, 'signature value')
  let verified = false
  try {
    verified = crypto.verify(null, Buffer.from(payloadCanonical), publicKey, Buffer.from(signatureValue, 'base64'))
  } catch {
    verified = false
  }
  if (!verified) fail('manifest signature verification failed')
  return { payload, payloadSha256, keyId }
}

function deploymentIdentity(identity, value) {
  return {
    identity,
    address: requireAddress(value?.address, `${identity} address`),
    runtimeCodeHash: requireHash(value?.runtimeCodeHash, `${identity} runtime code hash`),
  }
}

function deploymentIdentities(payload) {
  const rows = CONTRACT_IDENTITIES.map((key) => deploymentIdentity(`contract.${key}`, payload.contracts?.[key]))
  rows.push(deploymentIdentity('asset.paymentToken', payload.assets?.paymentToken))
  const projectTokens = payload.assets?.projectTokens
  if (!Array.isArray(projectTokens) || projectTokens.length === 0) fail('project token deployments are missing')
  for (const token of projectTokens) {
    const key = requireString(token?.key, 'project token key', /^[A-Za-z0-9][A-Za-z0-9_-]{1,31}$/)
    rows.push(deploymentIdentity(`asset.projectToken.${key}`, token))
  }
  rows.push(deploymentIdentity('safe.proceeds', payload.safes?.proceeds))
  rows.push(deploymentIdentity('safe.platformFee', payload.safes?.platformFee))
  if (new Set(rows.map((row) => row.identity)).size !== rows.length) fail('deployment identity is duplicated')
  if (new Set(rows.map((row) => row.address)).size !== rows.length) fail('security-relevant deployment address is duplicated')
  return rows.sort((a, b) => a.identity.localeCompare(b.identity))
}

function localApiAuthority(value, runId, frontendOrigin, manifestExpiresAt) {
	if (!value) fail('Backend HTTP authority is missing')
	const allowed = ['authority', 'runId', 'baseUrl', 'frontendOrigin', 'adminOrigin', 'token', 'expiresAt', 'status'].sort()
	if (Object.keys(value).sort().join('|') !== allowed.join('|')) fail('Backend HTTP authority fields are invalid')
	if (value.authority !== 'TEST_ONLY_NOT_PRODUCTION_ADMITTED' || value.status !== 'LOCAL_ONLY_HTTP_ADMITTED') fail('Backend HTTP authority is not admitted')
	if (value.runId !== runId || value.frontendOrigin !== frontendOrigin || !/^[0-9a-f]{64}$/.test(String(value.token || ''))) fail('Backend HTTP authority binding is invalid')
	const baseUrl = exactLoopbackUrl(value.baseUrl, 'Backend HTTP URL')
	const adminOrigin = exactLoopbackUrl(value.adminOrigin, 'Admin origin')
	const expiresAt = requireString(value.expiresAt, 'Backend HTTP expiry')
	if (baseUrl === frontendOrigin || baseUrl === adminOrigin || adminOrigin === frontendOrigin || !Number.isFinite(Date.parse(expiresAt)) || Date.parse(expiresAt) > Date.parse(manifestExpiresAt)) fail('Backend HTTP authority interval/origin is invalid')
	return { baseUrl, adminOrigin, token: value.token, expiresAt }
}

export function frontendEnvFromChainManifest(envelope, projectKey = 'successful', frontendOrigin = '', apiAuthorityValue = null) {
  const { payload, payloadSha256, keyId } = verifyTestOnlyEnvelope(envelope)
  if (payload.schemaVersion !== 'fair-launch-local-e2e-deployment-v1') fail('schemaVersion is not supported')
  if (payload.authority !== 'TEST_ONLY_NOT_PRODUCTION_ADMITTED') fail('authority is not TEST ONLY')

  const runId = requireString(payload.runId, 'runId', RUN_ID)
  const createdAt = requireString(payload.createdAt, 'createdAt')
  const expiresAt = requireString(payload.expiresAt, 'expiresAt')
  const createdAtMs = Date.parse(createdAt)
  const expiresAtMs = Date.parse(expiresAt)
  if (!Number.isFinite(createdAtMs) || !Number.isFinite(expiresAtMs) || expiresAtMs <= createdAtMs || expiresAtMs - createdAtMs > MAX_LIFETIME_MS) {
    fail('manifest validity window is invalid')
  }

  const network = payload.network || {}
  if (String(network.chainId) !== '31337') fail('network must be Hardhat chain 31337')
  const rpcUrl = exactLoopbackUrl(network.rpcUrl, 'run RPC URL')
  const exactFrontendOrigin = exactLoopbackUrl(frontendOrigin, 'frontend origin')
  if (rpcUrl === exactFrontendOrigin) fail('frontend origin and RPC URL must use distinct explicit ports')
	const apiAuthority = localApiAuthority(apiAuthorityValue, runId, exactFrontendOrigin, expiresAt)
	if (apiAuthority.baseUrl === rpcUrl) fail('Backend HTTP URL and RPC URL must use distinct explicit ports')

  const projects = Array.isArray(payload.projects) ? payload.projects : []
  const project = projects.find((item) => item?.key === projectKey)
  if (!project) fail(`project ${projectKey} is missing`)
  const projectId = requireString(project.projectId, 'projectId', /^[1-9]\d*$/)
  const observationBlockNumber = requireString(payload.deployment?.observation?.blockNumber ?? payload.deployment?.observationBlockNumber, 'observationBlockNumber', /^[1-9]\d*$/)
  const identities = deploymentIdentities(payload)
  const projectToken = identities.find((row) => row.address === requireAddress(project.projectToken, 'project token address'))
  if (!projectToken || !projectToken.identity.startsWith('asset.projectToken.')) fail('selected project token deployment is not represented')
  if (requireAddress(project.paymentToken, 'project payment token address') !== identities.find((row) => row.identity === 'asset.paymentToken')?.address) {
    fail('selected project payment token deployment is not represented')
  }
  const identityJson = canonicalJson(identities)
  const identityDigest = `0x${sha256(identityJson)}`
  const byIdentity = Object.fromEntries(identities.map((row) => [row.identity, row]))

  const values = {
    VITE_ENVIRONMENT: 'local',
    VITE_FAIR_LAUNCH_TEST_MODE: 'local_test',
    VITE_FAIR_LAUNCH_LOCAL_WALLET_SUBMISSION: 'true',
    VITE_CHECKIN_CHAIN_ID: '31337',
    VITE_HARDHAT_RPC: rpcUrl,
    VITE_FAIR_LAUNCH_LOCAL_RPC_URL: rpcUrl,
    VITE_FAIR_LAUNCH_LOCAL_FRONTEND_ORIGIN: exactFrontendOrigin,
    VITE_FAIR_LAUNCH_LOCAL_CHAIN_ID: '31337',
    VITE_FAIR_LAUNCH_LOCAL_PROJECT_ID: projectId,
    VITE_FAIR_LAUNCH_LOCAL_PROXY_ADDRESS: byIdentity['contract.proxy'].address,
    VITE_FAIR_LAUNCH_LOCAL_IMPLEMENTATION_ADDRESS: byIdentity['contract.dispatcher'].address,
    VITE_FAIR_LAUNCH_LOCAL_IMPLEMENTATION_CODE_HASH: byIdentity['contract.dispatcher'].runtimeCodeHash,
    VITE_FAIR_LAUNCH_LOCAL_PAYMENT_TOKEN_ADDRESS: byIdentity['asset.paymentToken'].address,
    VITE_FAIR_LAUNCH_LOCAL_PROJECT_TOKEN_ADDRESS: projectToken.address,
    VITE_FAIR_LAUNCH_LOCAL_DEPLOYMENT_IDENTITIES_JSON: identityJson,
    VITE_FAIR_LAUNCH_LOCAL_DEPLOYMENT_IDENTITIES_DIGEST: identityDigest,
    VITE_FAIR_LAUNCH_LOCAL_OBSERVATION_BLOCK_NUMBER: observationBlockNumber,
    VITE_FAIR_LAUNCH_LOCAL_OBSERVATION_BLOCK_HASH: requireHash(payload.deployment?.observation?.blockHash ?? payload.deployment?.observationBlockHash, 'observation block hash'),
    VITE_FAIR_LAUNCH_LOCAL_RUN_ID: runId,
    VITE_FAIR_LAUNCH_LOCAL_MANIFEST_PAYLOAD_SHA256: payloadSha256,
    VITE_FAIR_LAUNCH_LOCAL_MANIFEST_KEY_ID: keyId,
    VITE_FAIR_LAUNCH_LOCAL_CREATED_AT: createdAt,
    VITE_FAIR_LAUNCH_LOCAL_EXPIRES_AT: expiresAt,
	VITE_FAIR_LAUNCH_LOCAL_API_URL: apiAuthority.baseUrl,
	VITE_FAIR_LAUNCH_LOCAL_API_RUN_ID: runId,
	VITE_FAIR_LAUNCH_LOCAL_API_TOKEN: apiAuthority.token,
	VITE_FAIR_LAUNCH_LOCAL_API_EXPIRES_AT: apiAuthority.expiresAt,
  }
	  return '# GENERATED FROM VERIFIED TEST-ONLY CHAIN AND BACKEND HTTP AUTHORITIES.\n' +
    Object.entries(values).map(([key, value]) => `${key}=${value}`).join('\n') + '\n'
}

async function main() {
	  const [manifestPath, apiAuthorityPath, outputPath, frontendOrigin, projectKey = 'successful'] = process.argv.slice(2)
	  if (!manifestPath || !apiAuthorityPath || !outputPath || !frontendOrigin) {
	    fail('usage: node scripts/fair-launch-local-e2e-env.js <chain-manifest.json> <backend-http-authority.json> <output.env> <http://127.0.0.1:frontend-port> [project-key]')
  }
  const envelope = JSON.parse(await readFile(resolve(manifestPath), 'utf8'))
	  const apiAuthority = JSON.parse(await readFile(resolve(apiAuthorityPath), 'utf8'))
	  const content = frontendEnvFromChainManifest(envelope, projectKey, frontendOrigin, apiAuthority)
  await writeFile(resolve(outputPath), content, { encoding: 'utf8', mode: 0o600, flag: 'wx' })
  console.log(`Wrote TEST ONLY frontend env: ${resolve(outputPath)}`)
}

if (process.argv[1] && import.meta.url === new URL(`file:///${process.argv[1].replaceAll('\\', '/')}`).href) {
  main().catch((error) => {
    console.error(error?.message || error)
    process.exitCode = 1
  })
}
