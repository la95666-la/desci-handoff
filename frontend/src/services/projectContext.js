import {
  CONTRACT_VERSION_IDO_V3_STABLECOIN,
  CONTRACT_VERSION_FAIR_LAUNCH_V1,
  CONTRACT_VERSION_V2_STABLECOIN,
  resolveProjectFundingRoute,
  trustedFairLaunchRuntime,
} from '../utils/fairLaunch.js'

export const CONTRACT_VERSION_V2 = CONTRACT_VERSION_V2_STABLECOIN
export { CONTRACT_VERSION_FAIR_LAUNCH_V1 }

export function isV2Project(project) {
  return project?.contractVersion === CONTRACT_VERSION_V2
}

export function normalizeProjectContext(project, legacyDefaults) {
  if (!project || !(project.id ?? project.projectId ?? project.projectID)) throw new Error('Project identity is missing')
  const route = resolveProjectFundingRoute(project)
  if (!route.ok) throw new Error(`Unsupported project funding identity: ${route.reason}`)

  if (route.kind === 'ido_v3') {
    // Do not supply V2 globals or a legacy ABI for an incomplete V3 identity.
    // Dedicated V3 actions enforce pinned local routing and simulate exact calls.
    for (const key of ['contractAddress', 'paymentTokenAddress', 'tokenAddress']) {
      if (!/^0x[0-9a-fA-F]{40}$/.test(String(project[key] || '')) ||
          /^0x0{40}$/i.test(String(project[key]))) throw new Error(`V3 project ${key} is invalid`)
    }
    const chainId = Number(project.chainId)
    const paymentTokenDecimals = Number(project.paymentTokenDecimals)
    const tokenDecimals = Number(project.tokenDecimals)
    if (!Number.isSafeInteger(chainId) || chainId <= 0) throw new Error('V3 project chainId is invalid')
    if (!Number.isInteger(paymentTokenDecimals) || paymentTokenDecimals < 1 || paymentTokenDecimals > 18) throw new Error('V3 payment token decimals are invalid')
    if (!Number.isInteger(tokenDecimals) || tokenDecimals < 1 || tokenDecimals > 18) throw new Error('V3 project token decimals are invalid')
    const rawId = project.id ?? project.projectId ?? project.projectID
    if (typeof rawId !== 'string' && typeof rawId !== 'bigint' && !(typeof rawId === 'number' && Number.isSafeInteger(rawId))) throw new Error('V3 project ID is invalid')
    const id = String(rawId)
    if (!/^[1-9][0-9]*$/.test(id)) throw new Error('V3 project ID is invalid')
    return { ...project, id, contractVersion: CONTRACT_VERSION_IDO_V3_STABLECOIN, fundingMode: route.fundingMode, chainId, paymentTokenDecimals, tokenDecimals, readOnlyFunding: false }
  }

  // Only known historical capped-presale versions may use the pre-existing
  // legacy defaults. Fair Launch must never inherit a production contract/token address.
  if (route.kind === 'capped_presale' && !isV2Project(project)) {
    return {
      ...project,
      id: project.id ?? project.projectId ?? project.projectID,
      fundingMode: route.fundingMode,
      chainId: Number(project.chainId || legacyDefaults.chainId),
      contractAddress: project.contractAddress || legacyDefaults.contractAddress,
      paymentTokenAddress: project.paymentTokenAddress || legacyDefaults.paymentTokenAddress,
      paymentTokenDecimals: Number(project.paymentTokenDecimals || 18),
      tokenDecimals: Number(project.tokenDecimals || 18),
    }
  }

  if (route.kind === 'fair_launch') {
    const runtime = trustedFairLaunchRuntime(project)
    const paymentTokenAddress = project.paymentTokenAddress || project.payment_token_address
    const tokenAddress = project.tokenAddress || project.token_address
    for (const [key, value] of Object.entries({ paymentTokenAddress, tokenAddress })) {
      if (!/^0x[0-9a-fA-F]{40}$/.test(String(value || ''))) throw new Error(`Fair Launch project ${key} is invalid`)
    }
    const paymentTokenDecimals = Number(project.paymentTokenDecimals ?? project.payment_token_decimals)
    const tokenDecimals = Number(project.tokenDecimals ?? project.token_decimals)
    if (!Number.isInteger(paymentTokenDecimals) || paymentTokenDecimals < 1 || paymentTokenDecimals > 36) throw new Error('Fair Launch payment token decimals are invalid')
    if (!Number.isInteger(tokenDecimals) || tokenDecimals < 1 || tokenDecimals > 36) throw new Error('Fair Launch project token decimals are invalid')
    return {
      ...project,
      id: project.id ?? project.projectId ?? project.projectID,
      fundingMode: route.fundingMode,
      contractAddress: runtime.enabled ? runtime.address : String(project.contractAddress || project.contract_address || ''),
      chainId: runtime.enabled ? runtime.chainId : Number(project.chainId || project.chain_id || 0),
      paymentTokenAddress,
      tokenAddress,
      paymentTokenDecimals,
      tokenDecimals,
      fairLaunchRuntime: runtime,
      readOnlyFunding: !runtime.enabled,
    }
  }

  for (const key of ['contractAddress', 'paymentTokenAddress', 'tokenAddress']) {
    if (!/^0x[0-9a-fA-F]{40}$/.test(String(project[key] || ''))) {
      throw new Error(`V2 project ${key} is invalid`)
    }
  }
  const chainId = Number(project.chainId)
  const paymentTokenDecimals = Number(project.paymentTokenDecimals)
  const tokenDecimals = Number(project.tokenDecimals)
  if (!Number.isInteger(chainId) || chainId <= 0) throw new Error('V2 project chainId is invalid')
  if (!Number.isInteger(paymentTokenDecimals) || paymentTokenDecimals < 1 || paymentTokenDecimals > 18) throw new Error('V2 payment token decimals are invalid')
  if (!Number.isInteger(tokenDecimals) || tokenDecimals < 1 || tokenDecimals > 18) throw new Error('V2 project token decimals are invalid')
  return { ...project, id: project.id ?? project.projectId ?? project.projectID, fundingMode: route.fundingMode, chainId, paymentTokenDecimals, tokenDecimals }
}
