// Off-chain official-page policy only; never used to reject chain event accounting.
export function meetsMinPurchase(amountRaw, project) {
  const configured = project?.minPurchaseAmountRaw
  if (configured === '' || configured == null) return true
  if (!/^[1-9]\d*$/.test(String(configured))) return false
  return amountRaw >= BigInt(configured)
}
