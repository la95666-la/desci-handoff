import { formatUnits } from 'ethers'

// Legacy projects.total_raised is not the V3 financial projection. Until a
// project-chain display read succeeds, do not present its default 0 as fact.
export function initialOwnerStats(item) {
  if (item.contractVersion === 'ido_v3_stablecoin') {
    return { totalRaised: null, tokensSold: null, ownerStatsLoaded: false, ownerStatsError: false }
  }
  return { totalRaised: String(item.totalRaised ?? '0'), tokensSold: String(item.soldToken ?? '0'),
    ownerStatsLoaded: true, ownerStatsError: false }
}

export function applyOwnerStats(project, raw) {
  const raised = BigInt(raw.totalRaised), sold = BigInt(raw.totalTokensSold)
  if (raised < 0n || sold < 0n) throw Error('Invalid owner stats')
  Object.assign(project, { totalRaised: formatUnits(raised, project.paymentTokenDecimals),
    tokensSold: formatUnits(sold, project.tokenDecimals), ownerStatsLoaded: true, ownerStatsError: false })
}

export function ownerStatText(project, key, format) {
  return project.contractVersion === 'ido_v3_stablecoin' && !project.ownerStatsLoaded ? '—' : format(project[key])
}
