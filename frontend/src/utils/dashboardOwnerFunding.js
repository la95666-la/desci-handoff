// Preserve backend identity evidence verbatim; never infer a lock from status.
export function dashboardOwnerIdentity(item) {
  return {
    onchainLocked: item.onchainLocked,
    feeReceiver: item.feeReceiver,
    ownerAddress: item.ownerAddress,
  }
}

export function ownerFundingReady(project) {
  if (project.contractVersion !== 'ido_v3_stablecoin') return true
  return ownerFundingVisible(project) && !project.v3Failure && !project.v3Pending?.length
}

// Read-only data stays visible while the original submission is reconciled;
// visibility is never permission to send another transaction.
export function ownerFundingVisible(project) {
  if (project.contractVersion !== 'ido_v3_stablecoin') return true
  return project.v3FundingLoaded === true && !project.deposit?.statusLoading &&
    (!project.v3Failure || project.v3Failure === 'IDO_V3_SUBMISSION_UNRESOLVED')
}

// Display-only substate of Pending, not activation or permission to send.
// Never use shortfall.zero: its initial placeholder is already true.
export function ownerDepositComplete(project) {
  if (project.status !== 'Pending' || project.deposit?.statusLoading) return false
  if (project.contractVersion === 'ido_v3_stablecoin') {
    if (!ownerFundingVisible(project)) return false
  } else if (project.contractVersion === 'v2_stablecoin') {
    if (project.fundingSnapshotLoaded !== true) return false
  } else return false
  try {
    const required = project.hardCapRequired
    return required?.valid === true && BigInt(required.raw) > 0n &&
      BigInt(project.depositedRaw) >= BigInt(required.raw)
  } catch { return false }
}
