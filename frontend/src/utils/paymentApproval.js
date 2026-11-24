// Both capped-presale generations compare exact payment-token units, not
// rounded displays. A missing/invalid allowance can never authorize purchase.
export function paymentApprovalState(amountRaw, allowanceRaw) {
  try {
    const amount = BigInt(amountRaw)
    if (amount <= 0n) return { valid: false, needsApproval: false, approved: false }
    if (allowanceRaw == null) return { valid: true, needsApproval: true, approved: false }
    const allowance = BigInt(allowanceRaw)
    return { valid: true, needsApproval: allowance < amount, approved: allowance >= amount }
  } catch { return { valid: false, needsApproval: false, approved: false } }
}

// A successful receipt may lead a lagging RPC, but must never mask revocation
// or consumption at/after that receipt's block. No cross-wallet persistence.
export function reconcilePaymentAllowance(observedRaw, observedBlock, confirmation) {
  const observed = BigInt(observedRaw)
  if (confirmation && Number.isSafeInteger(confirmation.blockNumber) &&
      Number.isSafeInteger(observedBlock) && observedBlock < confirmation.blockNumber) {
    return { raw: BigInt(confirmation.raw), confirmation }
  }
  return { raw: observed, confirmation: null }
}
