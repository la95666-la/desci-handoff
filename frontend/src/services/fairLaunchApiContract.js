import {
  CONTRACT_VERSION_FAIR_LAUNCH_V1,
  FUNDING_MODE_FAIR_LAUNCH,
} from '../utils/fairLaunch.js'

const ADDRESS_PATTERN = /^0x[0-9a-fA-F]{40}$/
const HASH_PATTERN = /^0x[0-9a-fA-F]{64}$/
const RAW_PATTERN = /^(0|[1-9]\d*)$/

function requireObject(value, name) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(`Fair Launch ${name} is missing`)
  return value
}

function requireString(value, name) {
  const text = String(value ?? '').trim()
  if (!text) throw new Error(`Fair Launch ${name} is missing`)
  return text
}

function normalizeFinancialState(value, name) {
  const state = requireObject(value, `projection.${name}`)
  const raw = state.raw == null ? null : String(state.raw)
  if (raw !== null && !RAW_PATTERN.test(raw)) throw new Error(`Fair Launch projection.${name}.raw is invalid`)
  return {
    state: requireString(state.state, `projection.${name}.state`),
    raw,
    display: state.display == null ? null : String(state.display),
  }
}

export function normalizeFairLaunchRuntimeIdentity(value) {
  const runtime = requireObject(value, 'runtime identity')
  const chainId = Number(runtime.chainId)
  const sourceRegistryRevision = Number(runtime.sourceRegistryRevision)
  const observedBlockNumber = String(runtime.observedBlockNumber ?? '')
  const normalized = {
    chainId,
    proxyAddress: requireString(runtime.proxyAddress, 'runtime.proxyAddress'),
    implementationAddress: requireString(runtime.implementationAddress, 'runtime.implementationAddress'),
    implementationCodeHash: requireString(runtime.implementationCodeHash, 'runtime.implementationCodeHash'),
    sourceRegistryRevision,
    sourceRegistryDigest: requireString(runtime.sourceRegistryDigest, 'runtime.sourceRegistryDigest'),
    observedBlockNumber,
    observedBlockHash: requireString(runtime.observedBlockHash, 'runtime.observedBlockHash'),
    observedAt: requireString(runtime.observedAt, 'runtime.observedAt'),
    runtimeVerified: runtime.runtimeVerified === true,
  }
  if (!Number.isSafeInteger(chainId) || chainId <= 0) throw new Error('Fair Launch runtime.chainId is invalid')
  if (!Number.isSafeInteger(sourceRegistryRevision) || sourceRegistryRevision <= 0) throw new Error('Fair Launch runtime.sourceRegistryRevision is invalid')
  if (!RAW_PATTERN.test(observedBlockNumber) || observedBlockNumber === '0') throw new Error('Fair Launch runtime.observedBlockNumber is invalid')
  for (const key of ['proxyAddress', 'implementationAddress']) {
    if (!ADDRESS_PATTERN.test(normalized[key])) throw new Error(`Fair Launch runtime.${key} is invalid`)
  }
  if (!HASH_PATTERN.test(normalized.implementationCodeHash)) throw new Error('Fair Launch runtime.implementationCodeHash is invalid')
  for (const key of ['sourceRegistryDigest', 'observedBlockHash']) {
    const digest = normalized[key].startsWith('0x') ? normalized[key] : `0x${normalized[key]}`
    if (!HASH_PATTERN.test(digest)) throw new Error(`Fair Launch runtime.${key} is invalid`)
    normalized[key] = digest
  }
  if (!normalized.runtimeVerified) throw new Error('Fair Launch runtime is not verified')
  if (!Number.isFinite(Date.parse(normalized.observedAt))) throw new Error('Fair Launch runtime.observedAt is invalid')
  return normalized
}

export function canBootstrapFairLaunchPosition(project, walletState, options = {}) {
  const fundingMode = String(project?.fundingMode ?? project?.funding_mode ?? '').trim().toLowerCase()
  const contractVersion = String(project?.contractVersion ?? project?.contract_version ?? '').trim().toLowerCase()
  const wallet = String(walletState?.address ?? '').trim()
  const authenticatedRead = walletState?.isLoggedIn === true
  const admittedLocalRead = options?.localAuthorityRead === true
  return fundingMode === FUNDING_MODE_FAIR_LAUNCH &&
    contractVersion === CONTRACT_VERSION_FAIR_LAUNCH_V1 &&
    walletState?.isConnected === true &&
    (authenticatedRead || admittedLocalRead) &&
    ADDRESS_PATTERN.test(wallet)
}

function normalizeAsset(value, name) {
  const asset = requireObject(value, `assets.${name}`)
  const address = requireString(asset.address, `assets.${name}.address`)
  const decimals = Number(asset.decimals)
  if (!ADDRESS_PATTERN.test(address)) throw new Error(`Fair Launch assets.${name}.address is invalid`)
  if (!Number.isInteger(decimals) || decimals < 1 || decimals > 36) throw new Error(`Fair Launch assets.${name}.decimals is invalid`)
  return { address, decimals }
}

function normalizeActionDescriptors(value) {
  const actions = requireObject(value, 'actions')
  const normalized = {}
  for (const [name, descriptorValue] of Object.entries(actions)) {
    const descriptor = requireObject(descriptorValue, `actions.${name}`)
    if (typeof descriptor.available !== 'boolean') throw new Error(`Fair Launch actions.${name}.available is invalid`)
    normalized[name] = {
      ...descriptor,
      available: descriptor.available,
      reason: descriptor.reason == null ? '' : String(descriptor.reason),
    }
  }
  return normalized
}

export function normalizeFairLaunchPositionDto(value, expectedWallet = '') {
  const dto = requireObject(value, 'position response')
  if (dto.fundingMode !== FUNDING_MODE_FAIR_LAUNCH || dto.contractVersion !== CONTRACT_VERSION_FAIR_LAUNCH_V1) {
    throw new Error('Fair Launch response funding identity mismatch')
  }
  const projectId = requireString(dto.projectId, 'projectId')
  if (!RAW_PATTERN.test(projectId) || projectId === '0') throw new Error('Fair Launch response projectId is invalid')
  const wallet = requireString(dto.wallet, 'wallet').toLowerCase()
  if (!ADDRESS_PATTERN.test(wallet)) throw new Error('Fair Launch response wallet is invalid')
  if (expectedWallet && wallet !== String(expectedWallet).toLowerCase()) throw new Error('Fair Launch response wallet mismatch')
  const projection = requireObject(dto.projection, 'projection')
  const assets = requireObject(dto.assets, 'assets')
  const source = requireObject(dto.source, 'source')
  const runtime = normalizeFairLaunchRuntimeIdentity(dto.runtime)
  const stateSource = requireString(source.stateSource, 'source.stateSource')
  if (!['canonical_chain_event', 'canonical_chain_read', 'trusted_runtime_registry'].includes(stateSource)) {
    throw new Error('Fair Launch response state source is untrusted')
  }
  const finality = requireString(source.finality, 'source.finality').toLowerCase()
  if (finality !== 'finalized') throw new Error('Fair Launch response finality is not finalized')
  const reconciliationState = requireString(source.reconciliationState, 'source.reconciliationState').toLowerCase()
  if (!['balanced', 'unavailable', 'pending', 'unbalanced'].includes(reconciliationState)) {
    throw new Error('Fair Launch response reconciliation state is invalid')
  }
  return {
    ...dto,
    projectId,
    wallet,
    fundingMode: FUNDING_MODE_FAIR_LAUNCH,
    contractVersion: CONTRACT_VERSION_FAIR_LAUNCH_V1,
    chainStatus: requireString(dto.chainStatus, 'chainStatus'),
    assets: {
      paymentToken: normalizeAsset(assets.paymentToken, 'paymentToken'),
      projectToken: normalizeAsset(assets.projectToken, 'projectToken'),
    },
    projection: {
      ...projection,
      contribution: normalizeFinancialState(projection.contribution, 'contribution'),
      claimed: normalizeFinancialState(projection.claimed, 'claimed'),
      claimable: normalizeFinancialState(projection.claimable, 'claimable'),
      finalAllocation: normalizeFinancialState(projection.finalAllocation, 'finalAllocation'),
      refund: normalizeFinancialState(projection.refund, 'refund'),
    },
    actions: normalizeActionDescriptors(dto.actions),
    runtime,
    source: {
      ...source,
      stateSource,
      finality,
      reconciliationState,
    },
  }
}

export function normalizeFairLaunchPortfolioDto(value, expectedWallet = '') {
  const dto = requireObject(value, 'portfolio response')
  return {
    positions: Array.isArray(dto.positions) ? dto.positions.map((item) => normalizeFairLaunchPositionDto(item, expectedWallet)) : [],
    claims: Array.isArray(dto.claims) ? dto.claims.map((item) => normalizeFairLaunchHistoryDto(item, expectedWallet)) : [],
    refunds: Array.isArray(dto.refunds) ? dto.refunds.map((item) => normalizeFairLaunchHistoryDto(item, expectedWallet)) : [],
  }
}

function normalizeFairLaunchHistoryDto(value, expectedWallet) {
  const dto = requireObject(value, 'history item')
  if (dto.fundingMode !== FUNDING_MODE_FAIR_LAUNCH || dto.contractVersion !== CONTRACT_VERSION_FAIR_LAUNCH_V1) {
    throw new Error('Fair Launch history funding identity mismatch')
  }
  if (expectedWallet && String(dto.wallet || '').toLowerCase() !== String(expectedWallet).toLowerCase()) {
    throw new Error('Fair Launch history wallet mismatch')
  }
  return dto
}

export function fairLaunchPositionWalletContext(position, expectedWallet = '') {
  const dto = normalizeFairLaunchPositionDto(position, expectedWallet || position.wallet)
  return {
    ...dto,
    ...dto.projection,
    chainId: dto.runtime.chainId,
    contractAddress: dto.runtime.proxyAddress,
    paymentTokenAddress: dto.assets.paymentToken.address,
    paymentTokenDecimals: dto.assets.paymentToken.decimals,
    tokenAddress: dto.assets.projectToken.address,
    tokenDecimals: dto.assets.projectToken.decimals,
    runtimeIdentity: dto.runtime,
    stateSource: dto.source.stateSource,
    finality: dto.source.finality,
    reconciliationState: dto.source.reconciliationState,
  }
}

export function mergeFairLaunchPositionIdentity(project, position, expectedWallet = '') {
  const dto = fairLaunchPositionWalletContext(position, expectedWallet || position.wallet)
  const expectedProjectId = String(project?.id ?? project?.projectId ?? project?.projectID ?? '')
  if (!expectedProjectId || dto.projectId !== expectedProjectId) throw new Error('Fair Launch response project identity mismatch')
  return {
    ...project,
    fundingMode: dto.fundingMode,
    contractVersion: dto.contractVersion,
    chainId: dto.chainId,
    contractAddress: dto.contractAddress,
    paymentTokenAddress: dto.paymentTokenAddress,
    paymentTokenDecimals: dto.paymentTokenDecimals,
    tokenAddress: dto.tokenAddress,
    tokenDecimals: dto.tokenDecimals,
    runtimeIdentity: dto.runtimeIdentity,
    stateSource: dto.stateSource,
    finality: dto.finality,
    reconciliationState: dto.reconciliationState,
    actions: dto.actions,
    chainStatus: dto.chainStatus,
  }
}
