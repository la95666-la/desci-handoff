// A missing project identity blocks writes, but is not evidence of a wallet
// network mismatch. The banner and its expected-chain label share one result.
export function dashboardNetworkState(items, connectedChainId, fallbackChainId) {
  const chains = (items || [])
    .filter((item) => !['Setup', 'Set'].includes(item?.status))
    .map((item) => Number(item?.chainId))
    .filter((id) => Number.isSafeInteger(id) && id > 0)
  const connected = connectedChainId == null ? null : Number(connectedChainId)
  const mismatch = connected == null ? null : chains.find((id) => id !== connected)
  return {
    wrongNetwork: mismatch != null,
    expectedChainId: mismatch ?? chains[0] ?? Number(fallbackChainId),
  }
}
