import { deriveProjectStage } from './projectStatus.js'

// Presentation only. Investor writes simulate the exact contract call; owner
// writes retain their dedicated complete preflight. Labels authorize neither.
export function v3FundingStage(project, state, nowMs = Date.now()) {
  if (!state) return deriveProjectStage(project, nowMs)
  switch (state.status) {
    case 0: return 'pending'
    case 1: {
      const end = String(state.project?.endTime ?? '')
      const start = String(state.project?.startTime ?? '')
      const now = BigInt(Math.floor(nowMs / 1000))
      if (!/^\d+$/.test(start) || !/^\d+$/.test(end) || start === '0' || end === '0') return 'ended'
      if (now >= BigInt(end)) return 'ended'
      return state.saleOpen === true && now >= BigInt(start) ? 'live' : 'upcoming'
    }
    case 2: return 'successful'
    case 3: return 'failed'
    case 4: return 'canceled'
    default: return 'ended'
  }
}

export function v3WalletActions(state, position, claimableRaw = 0n) {
  const contributed = BigInt(position?.paymentAmountRaw ?? 0)
  return {
    claim: state?.status === 2 && BigInt(claimableRaw) > 0n,
    refund: (state?.status === 3 || state?.status === 4) && contributed > 0n && position?.refunded === false,
  }
}
