export const FUNDING_MODE_CAPPED_PRESALE = 'capped_presale'
export const FUNDING_MODE_FAIR_LAUNCH = 'fair_launch'
export const CONTRACT_VERSION_V2_STABLECOIN = 'v2_stablecoin'
export const CONTRACT_VERSION_IDO_V3_STABLECOIN = 'ido_v3_stablecoin'
export const CONTRACT_VERSION_FAIR_LAUNCH_V1 = 'fair_launch_v1'

const LEGACY_CAPPED_VERSIONS = new Set(['legacy_jcc', 'v1_legacy', 'v1_legacy_unverified'])
const DECIMAL_PATTERN = /^(0|[1-9]\d*)(?:\.(\d+))?$/
const RAW_PATTERN = /^(0|[1-9]\d*)$/

function projectField(project, camel, snake) {
  return project?.[camel] ?? project?.[snake]
}

export function resolveProjectFundingRoute(project) {
  const suppliedMode = String(projectField(project, 'fundingMode', 'funding_mode') ?? '').trim().toLowerCase()
  const contractVersion = String(projectField(project, 'contractVersion', 'contract_version') ?? '').trim().toLowerCase()

  // Existing capped-presale API records predate funding_mode. Preserve only their
  // known version mapping; unknown versions never inherit the legacy route.
  const fundingMode = suppliedMode || (
    contractVersion === CONTRACT_VERSION_V2_STABLECOIN || LEGACY_CAPPED_VERSIONS.has(contractVersion)
      ? FUNDING_MODE_CAPPED_PRESALE
      : ''
  )

  if (fundingMode === FUNDING_MODE_FAIR_LAUNCH && contractVersion === CONTRACT_VERSION_FAIR_LAUNCH_V1) {
    return { ok: true, kind: 'fair_launch', fundingMode, contractVersion, readOnly: false }
  }
  // V3 uses its own pinned proxy, ABI and wallet/proof path. It never inherits
  // V2/legacy transactions; this routing flag is not purchase authority.
  if (fundingMode === FUNDING_MODE_CAPPED_PRESALE && contractVersion === CONTRACT_VERSION_IDO_V3_STABLECOIN) {
    return { ok: true, kind: 'ido_v3', fundingMode, contractVersion, readOnly: false }
  }
  if (
    fundingMode === FUNDING_MODE_CAPPED_PRESALE &&
    (contractVersion === CONTRACT_VERSION_V2_STABLECOIN || LEGACY_CAPPED_VERSIONS.has(contractVersion))
  ) {
    return { ok: true, kind: 'capped_presale', fundingMode, contractVersion, readOnly: false }
  }

  const reason = !fundingMode || !contractVersion
    ? 'missing_funding_identity'
    : 'funding_identity_mismatch'
  return { ok: false, kind: 'unsupported', fundingMode, contractVersion, readOnly: true, reason }
}

export function decimalToScaledInteger(value, scale = 18) {
  if (!Number.isInteger(scale) || scale < 0 || scale > 36) throw new Error('Invalid decimal scale')
  const text = String(value ?? '').trim()
  const match = DECIMAL_PATTERN.exec(text)
  if (!match) throw new Error('Invalid decimal string')
  const fraction = match[2] || ''
  if (fraction.length > scale) throw new Error('Decimal precision exceeds scale')
  return BigInt(text.split('.')[0]) * (10n ** BigInt(scale)) + BigInt((fraction + '0'.repeat(scale)).slice(0, scale) || '0')
}

export function compareDecimalStrings(left, right) {
  const leftText = String(left ?? '').trim()
  const rightText = String(right ?? '').trim()
  const leftFraction = DECIMAL_PATTERN.exec(leftText)?.[2]?.length || 0
  const rightFraction = DECIMAL_PATTERN.exec(rightText)?.[2]?.length || 0
  const scale = Math.max(leftFraction, rightFraction)
  const a = decimalToScaledInteger(leftText, scale)
  const b = decimalToScaledInteger(rightText, scale)
  return a === b ? 0 : a > b ? 1 : -1
}

export function compareUnsignedIntegerStrings(left, right) {
  const leftText = String(left ?? '').trim()
  const rightText = String(right ?? '').trim()
  if (!RAW_PATTERN.test(leftText) || !RAW_PATTERN.test(rightText)) throw new Error('Invalid unsigned integer string')
  const a = BigInt(leftText)
  const b = BigInt(rightText)
  return a === b ? 0 : a > b ? 1 : -1
}

export function hasReachedSoftCap(project) {
  const totalRaisedRaw = projectField(project, 'totalRaisedRaw', 'total_raised_raw')
  const softCapRaw = projectField(project, 'softCapRaw', 'soft_cap_raw')
  if (totalRaisedRaw !== undefined && totalRaisedRaw !== null && softCapRaw !== undefined && softCapRaw !== null) {
    try {
      return compareUnsignedIntegerStrings(totalRaisedRaw, softCapRaw) >= 0 && compareUnsignedIntegerStrings(softCapRaw, '0') > 0
    } catch { return false }
  }

  const totalRaised = project?.totalRaised ?? project?.raised
  const softCap = projectField(project, 'softCap', 'soft_cap')
  try {
    return compareDecimalStrings(totalRaised, softCap) >= 0 && compareDecimalStrings(softCap, '0') > 0
  } catch { return false }
}

export function hasUnixTimePassed(unixSeconds, nowMs = Date.now()) {
  const secondsText = String(unixSeconds ?? '').trim()
  if (!RAW_PATTERN.test(secondsText) || secondsText === '0') return false
  const now = typeof nowMs === 'bigint'
    ? nowMs
    : BigInt(String(Math.trunc(Number(nowMs))))
  return now >= BigInt(secondsText) * 1000n
}

export function decimalRatioPercent(numerator, denominator, fractionDigits = 1) {
  if (!Number.isInteger(fractionDigits) || fractionDigits < 0 || fractionDigits > 6) {
    throw new Error('Invalid percentage precision')
  }
  const numeratorText = String(numerator ?? '').trim()
  const denominatorText = String(denominator ?? '').trim()
  const numeratorFraction = DECIMAL_PATTERN.exec(numeratorText)?.[2]?.length || 0
  const denominatorFraction = DECIMAL_PATTERN.exec(denominatorText)?.[2]?.length || 0
  const inputScale = Math.max(numeratorFraction, denominatorFraction)
  const raised = decimalToScaledInteger(numeratorText || '0', inputScale)
  const target = decimalToScaledInteger(denominatorText || '0', inputScale)
  if (target === 0n) return '0' + (fractionDigits ? `.${'0'.repeat(fractionDigits)}` : '')
  const precision = 10n ** BigInt(fractionDigits)
  const scaledPercent = raised * 100n * precision / target
  const whole = scaledPercent / precision
  if (!fractionDigits) return whole.toString()
  return `${whole}.${(scaledPercent % precision).toString().padStart(fractionDigits, '0')}`
}

export function clampPercentForBar(percent) {
  const text = String(percent ?? '').trim()
  if (!DECIMAL_PATTERN.test(text)) return '0'
  if (compareDecimalStrings(text, '100') >= 0) return '100'
  return text
}

export function formatDecimalString(value, maxFractionDigits = 6) {
  if (!Number.isInteger(maxFractionDigits) || maxFractionDigits < 0 || maxFractionDigits > 36) {
    throw new Error('Invalid display precision')
  }
  const text = String(value ?? '').trim()
  const match = DECIMAL_PATTERN.exec(text)
  if (!match) return '—'
  const [whole, fraction = ''] = text.split('.')
  const grouped = whole.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  const shownFraction = fraction.slice(0, maxFractionDigits).replace(/0+$/, '')
  return shownFraction ? `${grouped}.${shownFraction}` : grouped
}

export function formatRawUnits(rawValue, decimals, maxFractionDigits = 6) {
  const raw = String(rawValue ?? '').trim()
  if (!RAW_PATTERN.test(raw)) return '—'
  if (!Number.isInteger(decimals) || decimals < 0 || decimals > 36) return '—'
  const padded = raw.padStart(decimals + 1, '0')
  const split = padded.length - decimals
  const whole = decimals ? padded.slice(0, split) : padded
  const fraction = decimals ? padded.slice(split) : ''
  return formatDecimalString(fraction ? `${whole}.${fraction}` : whole, maxFractionDigits)
}

// Fair Launch public DTOs use these exact camelCase fields. Missing values are
// unavailable, not financial zero; snake-case or unrelated legacy fields must
// not silently substitute for the canonical read model.
export function formatCanonicalSaleTokenAmount(project, maxFractionDigits = 6) {
  const display = project?.saleTokenAmount
  if (display !== undefined && display !== null && String(display).trim() !== '') {
    return formatDecimalString(display, maxFractionDigits)
  }
  const decimals = Number(project?.tokenDecimals)
  return formatRawUnits(project?.saleTokenAmountRaw, decimals, maxFractionDigits)
}

export function calculateProportionalAllocation(contributionRaw, saleTokenAmountRaw, totalRaisedRaw) {
  const contribution = BigInt(String(contributionRaw))
  const saleAmount = BigInt(String(saleTokenAmountRaw))
  const totalRaised = BigInt(String(totalRaisedRaw))
  if (contribution < 0n || saleAmount <= 0n || totalRaised <= 0n) return 0n
  return contribution * saleAmount / totalRaised
}

export function normalizeFinancialState(value, allowed = ['available']) {
  const state = String(value?.state || '').trim().toLowerCase()
  if (!allowed.includes(state)) return { state: state || 'unavailable', available: false, raw: null, display: null }
  const raw = value?.raw == null ? null : String(value.raw)
  if (raw == null || !RAW_PATTERN.test(raw)) return { state: 'invalid', available: false, raw: null, display: null }
  return { state, available: true, raw, display: value?.display == null ? null : String(value.display) }
}

export function trustedFairLaunchRuntime(project) {
  const route = resolveProjectFundingRoute(project)
  if (!route.ok || route.kind !== 'fair_launch') return { enabled: false, reason: route.reason || 'not_fair_launch' }
  const identity = project?.runtimeIdentity || project?.runtime_identity
  if (!identity || typeof identity !== 'object') return { enabled: false, reason: 'backend_runtime_identity_missing' }
  const address = String(identity.proxyAddress || '')
  const chainId = Number(identity.chainId)
  const source = String(project?.stateSource || project?.source?.stateSource || '').toLowerCase()
  if (!/^0x[0-9a-fA-F]{40}$/.test(address)) return { enabled: false, reason: 'invalid_contract_address' }
  if (!Number.isInteger(chainId) || chainId <= 0) return { enabled: false, reason: 'invalid_chain_id' }
  if (identity.runtimeVerified !== true) return { enabled: false, reason: 'contract_runtime_unverified' }
  if (!/^0x[0-9a-fA-F]{40}$/.test(String(identity.implementationAddress || ''))) return { enabled: false, reason: 'implementation_address_missing' }
  for (const value of [identity.implementationCodeHash, identity.sourceRegistryDigest, identity.observedBlockHash]) {
    if (!/^0x[0-9a-fA-F]{64}$/.test(String(value || ''))) return { enabled: false, reason: 'runtime_evidence_incomplete' }
  }
  if (!Number.isSafeInteger(Number(identity.sourceRegistryRevision)) || Number(identity.sourceRegistryRevision) <= 0) return { enabled: false, reason: 'source_revision_invalid' }
  if (!RAW_PATTERN.test(String(identity.observedBlockNumber || '')) || String(identity.observedBlockNumber) === '0') return { enabled: false, reason: 'runtime_observation_missing' }
  if (!Number.isFinite(Date.parse(String(identity.observedAt || '')))) return { enabled: false, reason: 'runtime_observation_missing' }
  if (!['canonical_chain_event', 'canonical_chain_read', 'trusted_runtime_registry'].includes(source)) return { enabled: false, reason: 'untrusted_state_source' }
  if (projectField(project, 'contractAddress', 'contract_address') && String(projectField(project, 'contractAddress', 'contract_address')).toLowerCase() !== address.toLowerCase()) return { enabled: false, reason: 'runtime_proxy_mismatch' }
  if (projectField(project, 'chainId', 'chain_id') && Number(projectField(project, 'chainId', 'chain_id')) !== chainId) return { enabled: false, reason: 'runtime_chain_mismatch' }
  return { enabled: true, address, chainId, identity }
}

export function fairLaunchActionAvailable(project, action) {
  const runtime = trustedFairLaunchRuntime(project)
  if (!runtime.enabled) return runtime
  const actions = project?.actions || project?.fairLaunchActions || project?.fair_launch_actions
  const declaration = actions?.[action]
  if (!declaration || declaration.available !== true) return { enabled: false, reason: declaration?.reason || 'backend_action_unavailable' }
  const finality = String(project?.finality || project?.source?.finality || '').toLowerCase()
  if (['claim', 'refund', 'withdrawProjectProceeds', 'withdrawPlatformFee', 'withdrawFailedProjectTokens'].includes(action) && finality !== 'finalized') {
    return { enabled: false, reason: 'state_not_finalized' }
  }
  if (['claim', 'withdrawProjectProceeds', 'withdrawPlatformFee'].includes(action) && String(project?.reconciliationState || project?.source?.reconciliationState || '').toUpperCase() !== 'BALANCED') {
    return { enabled: false, reason: 'reconciliation_not_balanced' }
  }
  return { enabled: true, address: runtime.address, chainId: runtime.chainId }
}

export function fairLaunchSettlementState(project, nowMs = Date.now()) {
  const status = String(project?.status ?? '').trim().replace(/([a-z0-9])([A-Z])/g, '$1_$2').toLowerCase()
  if (status === 'successful_pending_release') return 'pending_release'
  if (status === 'successful') return 'released_success'
  if (status === 'failed' || status === 'canceled' || status === 'cancelled' || status === 'refund') return status === 'cancelled' ? 'canceled' : status

  const ended = hasUnixTimePassed(project?.endTime ?? project?.end_time, nowMs)
  const softCapReached = hasReachedSoftCap(project)

  if (status === 'active' && ended) return softCapReached ? 'ended_success_pending_settlement' : 'ended_failed_pending_settlement'
  if (status === 'active' && softCapReached) return 'soft_cap_reached_live'
  return status || 'unknown'
}
