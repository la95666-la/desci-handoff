// Only the purchase owned by this mounted panel is normal in-flight work.
// Restored/unknown submissions still require original-transaction recovery.
export function purchaseFeedbackKey(project, wallet) {
  return JSON.stringify([String(project?.id ?? ''), String(project?.contractAddress ?? '').toLowerCase(),
    Number(project?.chainId), String(wallet || '').toLowerCase()])
}

export function isLocalPurchasePending(entry, activeKey, currentKey) {
  return !!activeKey && activeKey === currentKey &&
    ['purchase(uint256,uint256)', 'purchaseWithProof(uint256,uint256,uint64,bytes32[])'].includes(entry?.action)
}
