import { keccak256, sha256 } from 'ethers'

const ADDRESS_PATTERN = /^0x[0-9a-fA-F]{40}$/
const HASH_PATTERN = /^0x[0-9a-fA-F]{64}$/
const PROJECT_ID_PATTERN = /^[1-9]\d*$/
const RAW_PATTERN = /^(0|[1-9]\d*)$/
const RUN_ID_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._-]{7,127}$/
const KEY_ID_PATTERN = /^local-e2e-[0-9a-f]{24}$/
const SHA256_PATTERN = /^[0-9a-f]{64}$/
const DEPLOYMENT_IDENTITY_PATTERN = /^(contract\.(proxy|dispatcher|coreFacet|releaseFacet|fundsFacet|adminFacet|viewsFacet|logicLibrary|runtimeLibrary)|asset\.paymentToken|asset\.projectToken\.[A-Za-z0-9][A-Za-z0-9_-]{1,31}|safe\.(proceeds|platformFee))$/
const REQUIRED_DEPLOYMENT_IDENTITIES = Object.freeze([
  'contract.proxy', 'contract.dispatcher', 'contract.coreFacet', 'contract.releaseFacet',
  'contract.fundsFacet', 'contract.adminFacet', 'contract.viewsFacet',
  'contract.logicLibrary', 'contract.runtimeLibrary', 'asset.paymentToken',
  'safe.proceeds', 'safe.platformFee',
])
const ZERO_ADDRESS = /^0x0{40}$/i
const ZERO_HASH = /^0x0{64}$/i
const MANIFEST_MAX_LIFETIME_MS = 24 * 60 * 60 * 1000
const MANIFEST_CLOCK_SKEW_MS = 60 * 1000
const HTTP_TOKEN_PATTERN = /^[0-9a-f]{64}$/

export const FAIR_LAUNCH_LOCAL_TEST_MODE = 'local_hardhat_ephemeral'
export const FAIR_LAUNCH_LOCAL_TEST_WATERMARK = 'LOCAL HARDHAT TEST ONLY — NO REAL FUNDS'
export const FAIR_LAUNCH_SUBMISSION_UNAVAILABLE_REASON = 'durable_wallet_submission_unavailable'

function disabled() {
  return { enabled: false, reason: FAIR_LAUNCH_SUBMISSION_UNAVAILABLE_REASON }
}

function normalizeHostname(hostname) {
  const value = String(hostname || '').trim().toLowerCase()
  return value.startsWith('[') && value.endsWith(']') ? value.slice(1, -1) : value
}

export function isExactLoopbackHostname(hostname) {
  return ['localhost', '127.0.0.1', '::1'].includes(normalizeHostname(hostname))
}

export function isLoopbackHttpUrl(value, { allowPort = true } = {}) {
  try {
    const url = new URL(String(value || ''))
    if (url.protocol !== 'http:' || !isExactLoopbackHostname(url.hostname)) return false
    if (url.username || url.password || url.search || url.hash) return false
    if (!allowPort && url.port) return false
    return true
  } catch {
    return false
  }
}

// RPC identity is stricter than browser-origin identity. Once admitted, the
// LOCAL-ONLY lane accepts no localhost/IPv6 alias and requires an explicit,
// run-specific loopback port.
function exactFairLaunchLoopbackUrl(value) {
  try {
    const url = new URL(String(value || ''))
    const port = Number(url.port)
    if (url.protocol !== 'http:' || normalizeHostname(url.hostname) !== '127.0.0.1' ||
        !/^\d+$/.test(url.port) || port < 1 || port > 65535 || url.pathname !== '/' ||
        url.username || url.password || url.search || url.hash) return ''
    return url.origin
  } catch {
    return ''
  }
}

export function isExactFairLaunchLocalRpcUrl(value) {
  return exactFairLaunchLoopbackUrl(value) !== ''
}

export function isExactFairLaunchLocalFrontendOrigin(value) {
  return exactFairLaunchLoopbackUrl(value) !== ''
}

function normalizeAddress(value) {
  const address = String(value || '').trim()
  return ADDRESS_PATTERN.test(address) && !ZERO_ADDRESS.test(address) ? address.toLowerCase() : ''
}

function normalizeHash(value) {
  const hash = String(value || '').trim()
  return HASH_PATTERN.test(hash) && !ZERO_HASH.test(hash) ? hash.toLowerCase() : ''
}

function canonical(value) {
  if (Array.isArray(value)) return value.map(canonical)
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.keys(value).sort().map((key) => [key, canonical(value[key])]))
  }
  return value
}

function normalizedDeploymentIdentities(value, expectedDigest) {
  let rows
  try {
    rows = typeof value === 'string' ? JSON.parse(value) : value
  } catch {
    return null
  }
  if (!Array.isArray(rows) || rows.length === 0) return null
  const normalized = rows.map((row) => ({
    identity: String(row?.identity || '').trim(),
    address: normalizeAddress(row?.address),
    runtimeCodeHash: normalizeHash(row?.runtimeCodeHash),
  })).sort((a, b) => a.identity.localeCompare(b.identity))
  if (normalized.some((row) => !DEPLOYMENT_IDENTITY_PATTERN.test(row.identity) || !row.address || !row.runtimeCodeHash)) return null
  if (new Set(normalized.map((row) => row.identity)).size !== normalized.length) return null
  if (new Set(normalized.map((row) => row.address)).size !== normalized.length) return null
  if (REQUIRED_DEPLOYMENT_IDENTITIES.some((identity) => !normalized.some((row) => row.identity === identity))) return null
  if (!normalized.some((row) => row.identity.startsWith('asset.projectToken.'))) return null
  const digest = normalizeHash(expectedDigest)
  if (!digest || sha256(new TextEncoder().encode(JSON.stringify(canonical(normalized)))).toLowerCase() !== digest) return null
  return normalized
}

export function normalizeFairLaunchLocalManifest(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const chainId = Number(value.chainId)
  const projectId = String(value.projectId || '').trim()
  const proxyAddress = normalizeAddress(value.proxyAddress)
  const implementationAddress = normalizeAddress(value.implementationAddress)
  const paymentTokenAddress = normalizeAddress(value.paymentTokenAddress)
  const projectTokenAddress = normalizeAddress(value.projectTokenAddress)
  const implementationCodeHash = normalizeHash(value.implementationCodeHash)
  const frontendOrigin = exactFairLaunchLoopbackUrl(value.frontendOrigin)
  const payloadSha256 = String(value.payloadSha256 || '').trim()
  const signingKeyId = String(value.signingKeyId || '').trim()
  const deploymentIdentitiesDigest = normalizeHash(value.deploymentIdentitiesDigest)
  const deploymentIdentities = normalizedDeploymentIdentities(value.deploymentIdentities, deploymentIdentitiesDigest)
  const observationBlockNumber = String(value.observationBlockNumber || '').trim()
  const observationBlockHash = normalizeHash(value.observationBlockHash)
  const runId = String(value.runId || '').trim()
  const createdAt = String(value.createdAt || '').trim()
  const expiresAt = String(value.expiresAt || '').trim()
  const createdAtMs = Date.parse(createdAt)
  const expiresAtMs = Date.parse(expiresAt)
  if (chainId !== 31337 || !PROJECT_ID_PATTERN.test(projectId) || !RUN_ID_PATTERN.test(runId)) return null
  if (!RAW_PATTERN.test(observationBlockNumber) || observationBlockNumber === '0') return null
  if (!Number.isFinite(createdAtMs) || !Number.isFinite(expiresAtMs) || expiresAtMs <= createdAtMs) return null
  if (expiresAtMs - createdAtMs > MANIFEST_MAX_LIFETIME_MS) return null
  if (!proxyAddress || !implementationAddress || !paymentTokenAddress || !projectTokenAddress) return null
  if (!implementationCodeHash || !observationBlockHash || !frontendOrigin || !deploymentIdentities) return null
  if (!SHA256_PATTERN.test(payloadSha256) || !KEY_ID_PATTERN.test(signingKeyId)) return null
  if (new Set([proxyAddress, implementationAddress, paymentTokenAddress, projectTokenAddress]).size !== 4) return null
  const identityByName = Object.fromEntries(deploymentIdentities.map((row) => [row.identity, row]))
  if (identityByName['contract.proxy']?.address !== proxyAddress ||
      identityByName['contract.dispatcher']?.address !== implementationAddress ||
      identityByName['contract.dispatcher']?.runtimeCodeHash !== implementationCodeHash ||
      identityByName['asset.paymentToken']?.address !== paymentTokenAddress ||
      !deploymentIdentities.some((row) => row.identity.startsWith('asset.projectToken.') && row.address === projectTokenAddress)) return null
  return {
    chainId,
    projectId,
    proxyAddress,
    implementationAddress,
    implementationCodeHash,
    paymentTokenAddress,
    projectTokenAddress,
    frontendOrigin,
    payloadSha256,
    signingKeyId,
    deploymentIdentities,
    deploymentIdentitiesDigest,
    observationBlockNumber,
    observationBlockHash,
    runId,
    createdAt,
    expiresAt,
  }
}

function manifestFromBuild(build) {
  return normalizeFairLaunchLocalManifest({
    chainId: build?.manifestChainId,
    projectId: build?.manifestProjectId,
    proxyAddress: build?.manifestProxyAddress,
    implementationAddress: build?.manifestImplementationAddress,
    implementationCodeHash: build?.manifestImplementationCodeHash,
    paymentTokenAddress: build?.manifestPaymentTokenAddress,
    projectTokenAddress: build?.manifestProjectTokenAddress,
    frontendOrigin: build?.manifestFrontendOrigin,
    payloadSha256: build?.manifestPayloadSha256,
    signingKeyId: build?.manifestSigningKeyId,
    deploymentIdentities: build?.manifestDeploymentIdentities,
    deploymentIdentitiesDigest: build?.manifestDeploymentIdentitiesDigest,
    observationBlockNumber: build?.manifestObservationBlockNumber,
    observationBlockHash: build?.manifestObservationBlockHash,
    runId: build?.manifestRunId,
    createdAt: build?.manifestCreatedAt,
    expiresAt: build?.manifestExpiresAt,
  })
}

function localBuildAdmission(build, origin, nowMs = Date.now()) {
  if (build?.dev !== true || build?.prod === true || String(build?.mode || '') !== 'local_test') return disabled()
  if (String(build?.environment || '') !== 'local') return disabled()
  if (Number(build?.chainId) !== 31337 || String(build?.optIn || '') !== 'true') return disabled()
  const rpcUrl = exactFairLaunchLoopbackUrl(build?.rpcUrl)
  const browserOrigin = exactFairLaunchLoopbackUrl(origin?.origin)
  if (!rpcUrl || !browserOrigin) return disabled()
  if (String(build?.manifestRpcUrl || '') !== rpcUrl) return disabled()
  const manifest = manifestFromBuild(build)
  if (!manifest || manifest.frontendOrigin !== browserOrigin || manifest.frontendOrigin === rpcUrl) return disabled()
  const now = Number(nowMs)
  const createdAtMs = Date.parse(manifest.createdAt)
  const expiresAtMs = Date.parse(manifest.expiresAt)
  if (!Number.isFinite(now) || createdAtMs > now + MANIFEST_CLOCK_SKEW_MS || now >= expiresAtMs) return disabled()
  return { enabled: true, mode: FAIR_LAUNCH_LOCAL_TEST_MODE, manifest, rpcUrl }
}

function field(value, camel, snake) {
  return value?.[camel] ?? value?.[snake]
}

function projectMatchesManifest(project, manifest) {
  const fundingMode = String(field(project, 'fundingMode', 'funding_mode') || '').trim().toLowerCase()
  const contractVersion = String(field(project, 'contractVersion', 'contract_version') || '').trim().toLowerCase()
  if (fundingMode !== 'fair_launch' || contractVersion !== 'fair_launch_v1') return false
  const runtime = project?.runtimeIdentity || project?.runtime_identity || project?.fairLaunchRuntime?.identity
  if (!runtime || runtime.runtimeVerified !== true) return false
  if (String(project?.id ?? project?.projectId ?? project?.projectID ?? '') !== manifest.projectId) return false
  if (Number(field(project, 'chainId', 'chain_id')) !== 31337 || Number(runtime.chainId) !== 31337) return false
  const addresses = [
    [field(project, 'contractAddress', 'contract_address'), manifest.proxyAddress],
    [runtime.proxyAddress, manifest.proxyAddress],
    [runtime.implementationAddress, manifest.implementationAddress],
    [field(project, 'paymentTokenAddress', 'payment_token_address'), manifest.paymentTokenAddress],
    [field(project, 'tokenAddress', 'token_address'), manifest.projectTokenAddress],
  ]
  if (addresses.some(([actual, expected]) => normalizeAddress(actual) !== expected)) return false
  if (normalizeHash(runtime.implementationCodeHash) !== manifest.implementationCodeHash) return false
  if (String(runtime.observedBlockNumber || '') !== manifest.observationBlockNumber) return false
  if (normalizeHash(runtime.observedBlockHash) !== manifest.observationBlockHash) return false
  return true
}

/**
 * Pure LOCAL-ONLY identity evidence check. This deliberately does not authorize
 * a transaction: it only proves that the browser build, chain manifest and
 * Backend read DTO describe the same fresh Hardhat run.
 */
export function evaluateFairLaunchLocalAdmissionEvidence({ build, origin, project, nowMs = Date.now() } = {}) {
  try {
    const admission = localBuildAdmission(build, origin, nowMs)
    if (!admission.enabled || !projectMatchesManifest(project, admission.manifest)) return disabled()
    return admission
  } catch {
    return disabled()
  }
}

/**
 * Browser writes remain mechanically closed until Backend exposes the reviewed
 * run-bound local authority/materializer contract. Chain identity evidence
 * alone must never be upgraded into financial submission authority.
 */
export function evaluateFairLaunchSubmissionPolicy(inputs = {}) {
	  const admission = evaluateFairLaunchLocalAdmissionEvidence(inputs)
	  if (!admission.enabled) return disabled()
	  const apiUrl = exactFairLaunchLoopbackUrl(inputs?.build?.apiUrl)
	  const apiRunId = String(inputs?.build?.apiRunId || '').trim()
	  const apiToken = String(inputs?.build?.apiToken || '').trim()
	  const apiExpiresAt = String(inputs?.build?.apiExpiresAt || '').trim()
	  const apiExpiresAtMs = Date.parse(apiExpiresAt)
	  const now = Number(inputs?.nowMs ?? Date.now())
	  if (!apiUrl || apiUrl === admission.rpcUrl || apiUrl === admission.manifest.frontendOrigin) return disabled()
	  if (apiRunId !== admission.manifest.runId || !HTTP_TOKEN_PATTERN.test(apiToken)) return disabled()
	  if (!Number.isFinite(apiExpiresAtMs) || !Number.isFinite(now) || now >= apiExpiresAtMs || apiExpiresAtMs > Date.parse(admission.manifest.expiresAt)) return disabled()
	  return { ...admission, apiUrl, apiRunId, apiToken, apiExpiresAt }
}

function buildInputs(env) {
  return {
    dev: env?.DEV === true,
    prod: env?.PROD === true,
    environment: env?.VITE_ENVIRONMENT,
    mode: env?.VITE_FAIR_LAUNCH_TEST_MODE,
    chainId: env?.VITE_CHECKIN_CHAIN_ID,
    optIn: env?.VITE_FAIR_LAUNCH_LOCAL_WALLET_SUBMISSION,
    rpcUrl: env?.VITE_HARDHAT_RPC,
    manifestChainId: env?.VITE_FAIR_LAUNCH_LOCAL_CHAIN_ID,
    manifestProjectId: env?.VITE_FAIR_LAUNCH_LOCAL_PROJECT_ID,
    manifestProxyAddress: env?.VITE_FAIR_LAUNCH_LOCAL_PROXY_ADDRESS,
    manifestImplementationAddress: env?.VITE_FAIR_LAUNCH_LOCAL_IMPLEMENTATION_ADDRESS,
    manifestImplementationCodeHash: env?.VITE_FAIR_LAUNCH_LOCAL_IMPLEMENTATION_CODE_HASH,
    manifestPaymentTokenAddress: env?.VITE_FAIR_LAUNCH_LOCAL_PAYMENT_TOKEN_ADDRESS,
    manifestProjectTokenAddress: env?.VITE_FAIR_LAUNCH_LOCAL_PROJECT_TOKEN_ADDRESS,
    manifestFrontendOrigin: env?.VITE_FAIR_LAUNCH_LOCAL_FRONTEND_ORIGIN,
    manifestPayloadSha256: env?.VITE_FAIR_LAUNCH_LOCAL_MANIFEST_PAYLOAD_SHA256,
    manifestSigningKeyId: env?.VITE_FAIR_LAUNCH_LOCAL_MANIFEST_KEY_ID,
    manifestDeploymentIdentities: env?.VITE_FAIR_LAUNCH_LOCAL_DEPLOYMENT_IDENTITIES_JSON,
    manifestDeploymentIdentitiesDigest: env?.VITE_FAIR_LAUNCH_LOCAL_DEPLOYMENT_IDENTITIES_DIGEST,
    manifestObservationBlockNumber: env?.VITE_FAIR_LAUNCH_LOCAL_OBSERVATION_BLOCK_NUMBER,
    manifestObservationBlockHash: env?.VITE_FAIR_LAUNCH_LOCAL_OBSERVATION_BLOCK_HASH,
    manifestRunId: env?.VITE_FAIR_LAUNCH_LOCAL_RUN_ID,
    manifestRpcUrl: env?.VITE_FAIR_LAUNCH_LOCAL_RPC_URL,
    manifestCreatedAt: env?.VITE_FAIR_LAUNCH_LOCAL_CREATED_AT,
    manifestExpiresAt: env?.VITE_FAIR_LAUNCH_LOCAL_EXPIRES_AT,
	  apiUrl: env?.VITE_FAIR_LAUNCH_LOCAL_API_URL,
	  apiRunId: env?.VITE_FAIR_LAUNCH_LOCAL_API_RUN_ID,
	  apiToken: env?.VITE_FAIR_LAUNCH_LOCAL_API_TOKEN,
	  apiExpiresAt: env?.VITE_FAIR_LAUNCH_LOCAL_API_EXPIRES_AT,
  }
}

function browserOrigin(location) {
  return location ? { origin: location.origin } : null
}

export function currentFairLaunchSubmissionPolicy(project) {
  if (import.meta.env?.DEV !== true) return disabled()
  const env = import.meta.env || {}
  const location = typeof window === 'undefined' ? null : window.location
  return evaluateFairLaunchSubmissionPolicy({ build: buildInputs(env), origin: browserOrigin(location), project })
}

function isNonEmptyCode(value) {
  return typeof value === 'string' && /^0x[0-9a-fA-F]+$/.test(value) && value !== '0x' && !/^0x0+$/.test(value)
}

export async function verifyFairLaunchLocalDeployment(provider, policy, project, activeRpcUrl = '') {
  if (!policy?.enabled || !provider?.getCode || !provider?.getBlock) return false
  if (!isExactFairLaunchLocalRpcUrl(activeRpcUrl) || String(activeRpcUrl) !== String(policy.rpcUrl || '')) return false
  const manifest = policy.manifest
  const runtime = project?.runtimeIdentity || project?.runtime_identity || project?.fairLaunchRuntime?.identity
  try {
    const observedBlockNumber = Number(runtime?.observedBlockNumber)
    if (!Number.isSafeInteger(observedBlockNumber) || observedBlockNumber < 1 || String(observedBlockNumber) !== String(runtime?.observedBlockNumber || '')) return false
    const [codes, observedBlock] = await Promise.all([
      Promise.all(manifest.deploymentIdentities.map((identity) => provider.getCode(identity.address))),
      provider.getBlock(observedBlockNumber),
    ])
    if (codes.length !== manifest.deploymentIdentities.length || codes.some((code) => !isNonEmptyCode(code))) return false
    for (let index = 0; index < codes.length; index += 1) {
      if (keccak256(codes[index]).toLowerCase() !== manifest.deploymentIdentities[index].runtimeCodeHash) return false
    }
    return normalizeHash(observedBlock?.hash) === manifest.observationBlockHash
  } catch {
    return false
  }
}

/** Watermark admission intentionally omits project identity, but retains every build/origin/manifest factor. */
export function currentFairLaunchLocalTestWatermark() {
  try {
    if (import.meta.env?.DEV !== true) return ''
    const env = import.meta.env || {}
    const location = typeof window === 'undefined' ? null : window.location
    const admission = localBuildAdmission(buildInputs(env), browserOrigin(location), Date.now())
    return admission.enabled ? `${FAIR_LAUNCH_LOCAL_TEST_WATERMARK} • RUN ${admission.manifest.runId}` : ''
  } catch {
    return ''
  }
}
