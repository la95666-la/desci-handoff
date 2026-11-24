/** A mined claim consumes the unlocked tranche even if the portfolio index lags. */
export function claimInProgressFor(itemKey, pendingKey) {
  return pendingKey != null && itemKey === pendingKey
}

export function applyVerifiedClaimPosition(item, availableRaw, claimedRaw, formattedAmount) {
  const available = BigInt(availableRaw)
  const claimed = BigInt(claimedRaw)
  if (available < 0n || claimed < 0n) throw new Error('Invalid claim position')
  return {
    ...item,
    claimableRaw: available.toString(),
    claimableAmount: Number(formattedAmount),
    claimedRaw: claimed.toString(),
    claimCheckUnavailable: false,
  }
}

export function claimActionState(item, confirmedClaimTime = 0, now = Date.now()) {
  if (item?.claimCheckUnavailable) return { claimed: false, disabled: true, unavailable: true }
  const next = Number(item?.nextUnlockTime) || 0
  const exactCappedPresale = item?.contractVersion === 'v2_stablecoin' || item?.contractVersion === 'ido_v3_stablecoin'
  const locked = exactCappedPresale && confirmedClaimTime > 0 && (!next || next > now)
  // Capped-presale raw amounts are authoritative when present: a rounded display of 0
  // must not disable a positive dust claim, nor may a stale display enable 0 raw.
  let available = Number(item?.claimableAmount) > 0
  if (exactCappedPresale && item.claimableRaw != null) {
    try { available = BigInt(item.claimableRaw) > 0n } catch { available = false }
  }
  // "Claimed" describes a successfully consumed tranche, not a never-claimed
  // position whose first unlock is still pending.
  let previouslyClaimed = Number(item?.totalClaimed) > 0
  if (exactCappedPresale && item.claimedRaw != null) {
    try { previouslyClaimed = BigInt(item.claimedRaw) > 0n } catch { previouslyClaimed = false }
  }
  return { claimed: locked || (!available && previouslyClaimed), disabled: locked || !available }
}
