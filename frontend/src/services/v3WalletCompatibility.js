import { Interface, getAddress, toQuantity } from 'ethers'

const approve = new Interface(['function approve(address spender,uint256 value)'])

// Standard EIP-1193 payload only. The ERC-20 amount belongs in calldata, never
// in native value. Re-encode approvals to reject trailing bytes/zero amounts.
export function v3WalletPayload(to, data, from, gas, chainId) {
  if (!/^0x(?:[0-9a-fA-F]{2})+$/.test(data) || BigInt(gas) <= 0n || !Number.isSafeInteger(chainId) || chainId <= 0) throw Error('Invalid wallet request')
  if (data.slice(0, 10).toLowerCase() === approve.getFunction('approve').selector) {
    const decoded = approve.decodeFunctionData('approve', data)
    if (decoded.value <= 0n || approve.encodeFunctionData('approve', [decoded.spender, decoded.value]).toLowerCase() !== data.toLowerCase()) throw Error('Invalid approval amount')
  }
  return { from: getAddress(from), to: getAddress(to), data, value: '0x0', gas: toQuantity(gas), chainId: toQuantity(chainId) }
}

// Only explicit, structured rejection/unsupported responses prove no broadcast.
// Never infer this from arbitrary message text, timeout or transport errors.
export function v3WalletErrorKind(error) {
  const queue = [error], seen = new Set(), codes = []
  while (queue.length && seen.size < 12) {
    const value = queue.shift()
    if (!value || typeof value !== 'object' || seen.has(value)) continue
    seen.add(value)
    if ([value.hash, value.transactionHash, value.receipt?.hash].some(hash => /^0x[0-9a-fA-F]{64}$/.test(String(hash || '')))) return 'unknown'
    if (value.code != null) codes.push(value.code)
    queue.push(value.error, value.data, value.originalError, value.info?.error)
  }
  if (codes.some(code => String(code) === '4001' || code === 'ACTION_REJECTED')) return 'rejected'
  if (codes.some(code => String(code) === '-32601' || String(code) === '4200')) return 'unsupported'
  return 'unknown'
}
