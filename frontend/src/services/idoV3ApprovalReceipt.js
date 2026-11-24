import { Interface } from 'ethers'

const abi = new Interface([
  'function approve(address spender,uint256 amount)',
  'event Approval(address indexed owner,address indexed spender,uint256 value)',
])
const same = (a, b) => typeof a === 'string' && typeof b === 'string' && a.toLowerCase() === b.toLowerCase()

// Some wallets let their user edit only the approval amount. Such a confirmed
// original transaction is terminal, not an unbroadcast deposit. Reconcile it
// only with exact actor/token/spender and an agreeing canonical Approval log.
// Callers MUST independently verify hash, chain, canonical block and finality.
// This never grants allowance or permission to deposit; fresh reads do that.
export function isV3WalletEditedApproval(context, action, entry, receipt, transaction) {
  try {
    const token = action === 'approveProjectToken' ? context.tokenAddress
      : action === 'approvePaymentToken' ? context.paymentTokenAddress : null
    if (!token || BigInt(receipt.status) !== 1n || BigInt(transaction.value) !== 0n ||
        !same(entry.to, token) || !same(transaction.to, token) ||
        !same(transaction.from, entry.from)) return false
    const decoded = abi.decodeFunctionData('approve', transaction.data)
    if (!same(decoded.spender, context.contractAddress)) return false
    // Reject trailing bytes / alternate selectors rather than accepting a
    // partial decode of unrelated wallet-mutated calldata.
    if (!same(abi.encodeFunctionData('approve', [decoded.spender, decoded.amount]), transaction.data)) return false
    return receipt.logs.some((log) => {
      if (!same(log.address, token)) return false
      const event = abi.parseLog(log)
      return event?.name === 'Approval' && same(event.args.owner, entry.from) &&
        same(event.args.spender, context.contractAddress) && event.args.value === decoded.amount
    })
  } catch { return false }
}
