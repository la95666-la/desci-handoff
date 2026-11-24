import { decimalRatioPercent, resolveProjectFundingRoute } from './fairLaunch.js'

// Display evidence only; never a wallet-action permission or chain fallback.
export function publicFundingProgress(project) {
  const route = resolveProjectFundingRoute(project)
  if (!route.ok || route.kind !== 'ido_v3') return null
  if (project.fundingDataStatus === 'syncing' || project.totalRaised === null ||
      project.totalRaised === undefined || project.totalRaised === '') return { ready: false, percent: null }
  try {
    if (!/^(0|[1-9][0-9]*)(\.[0-9]+)?$/.test(String(project.totalRaised)) ||
        !/^(0|[1-9][0-9]*)(\.[0-9]+)?$/.test(String(project.hardCap)) ||
        !/[1-9]/.test(String(project.hardCap))) return { ready: false, percent: null }
    const percent = decimalRatioPercent(String(project.totalRaised), String(project.hardCap), 2)
    return { ready: true, percent }
  } catch { return { ready: false, percent: null } }
}
