const cappedVersions = ['v2_stablecoin', 'ido_v3_stablecoin']
export const isCappedPortfolio = (item) => cappedVersions.includes(item?.contractVersion)
export const positiveRaw = (raw) => {
  try { return BigInt(raw ?? 0) > 0n } catch { return false }
}

// A failed provider is unavailable, not an empty portfolio. Its neighbours
// still load and the caller must render the explicit per-source warning.
export async function loadPortfolioSource(read, source, errors) {
  try { return await read() }
  catch {
    errors.push(source)
    return { positions: [], purchases: [], claims: [], refunds: [] }
  }
}

export function cappedPositionContext(item) {
  return {
    projectId: String(item.projectID), fundingMode: 'capped_presale',
    contractVersion: item.contractVersion, contractAddress: item.contractAddress, chainId: item.chainId,
    onchainLocked: item.onchainLocked, feeReceiver: item.feeReceiver, ownerAddress: item.ownerAddress,
    paymentToken: item.paymentToken, paymentTokenAddress: item.paymentTokenAddress,
    paymentTokenDecimals: item.paymentTokenDecimals, tokenAddress: item.projectTokenAddress,
    tokenDecimals: item.projectTokenDecimals,
  }
}

// Older portfolio DTOs lack locked/receiver evidence. Fetch exact fresh detail
// rather than inventing it from a purchase. ContractService revalidates it.
export async function verifiedPortfolioContext(item, readDetail, { deferFreshIdentity = false } = {}) {
  if (item.contractVersion !== 'ido_v3_stablecoin') return { ...item, id: item.projectId }
  // Complete locked DTOs can proceed directly to ContractService, which always
  // verifies fresh detail plus pinned chain identity before sending. This is
  // input context, not reusable transaction authority. Older DTOs still hydrate.
  if (deferFreshIdentity && item.onchainLocked === true && item.feeReceiver) {
    return { ...item, id: String(item.projectId) }
  }
  const detail = await readDetail(item.projectId, { fresh: true })
  for (const key of ['contractVersion', 'fundingMode', 'chainId', 'contractAddress', 'tokenAddress',
    'paymentTokenAddress', 'tokenDecimals', 'paymentTokenDecimals']) {
    const expected = key === 'fundingMode' ? 'capped_presale' : item[key]
    if (expected == null || String(detail[key]).toLowerCase() !== String(expected).toLowerCase()) {
      throw Object.assign(new Error('IDO_V3_IDENTITY_UNAVAILABLE'), { code: 'IDO_V3_IDENTITY_UNAVAILABLE' })
    }
  }
  if (detail.onchainLocked !== true) throw Object.assign(new Error('IDO_V3_IDENTITY_UNAVAILABLE'), { code: 'IDO_V3_IDENTITY_UNAVAILABLE' })
  return { ...item, ...detail, id: String(item.projectId), projectId: String(item.projectId) }
}

export const newestPortfolioRecords = (items) => items.sort((a, b) => Number(b.time) - Number(a.time))
