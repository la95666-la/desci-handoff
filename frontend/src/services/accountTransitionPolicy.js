function connectorIdentity(connector) {
  return connector?.uid || connector?.id || connector?.name || null
}

export function shouldAuthenticateAccountTransition(account, previous) {
  if (!account?.isConnected || !account?.address) return false
  if (!previous?.isConnected) return true

  const nextAddress = String(account.address).toLowerCase()
  const previousAddress = previous?.address ? String(previous.address).toLowerCase() : null
  if (nextAddress !== previousAddress) return true

  return connectorIdentity(account.connector) !== connectorIdentity(previous?.connector)
}
