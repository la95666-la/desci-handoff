import { WALLET_ERROR_CODES } from './errors.js'

const RAW_PATTERN = /^(0|[1-9]\d*)$/
const ADDRESS_PATTERN = /^0x[0-9a-fA-F]{40}$/

export function requireFairLaunchWalletAddress(walletAddress) {
  const wallet = String(walletAddress ?? '').trim().toLowerCase()
  if (!ADDRESS_PATTERN.test(wallet)) throw new Error('Authenticated Fair Launch wallet is invalid')
  return wallet
}

export function assertFairLaunchAuthorizedActor(action, signerAddress, chainProject) {
  const signer = requireFairLaunchWalletAddress(signerAddress)
  const receiverByAction = {
    depositProjectTokens: chainProject?.projectOwner,
    withdrawFailedProjectTokens: chainProject?.projectOwner,
    withdrawProjectProceeds: chainProject?.projectProceedsReceiver,
    withdrawPlatformFee: chainProject?.platformFeeReceiver,
  }
  const required = receiverByAction[action]
  if (required == null) return true
  if (signer !== requireFairLaunchWalletAddress(required)) {
    const error = new Error(`${action} requires its authorized wallet or fixed Safe receiver`)
    error.code = 'FAIR_LAUNCH_ROLE_BLOCKED'
    throw error
  }
  return true
}

export function calculateFairLaunchRemainingRaw(saleTokenAmountRaw, totalTokensDepositedRaw) {
  const saleText = String(saleTokenAmountRaw ?? '').trim()
  const depositedText = String(totalTokensDepositedRaw ?? '').trim()
  if (!RAW_PATTERN.test(saleText) || !RAW_PATTERN.test(depositedText)) throw new Error('Fair Launch deposit amounts are invalid')
  const remaining = BigInt(saleText) - BigInt(depositedText)
  if (remaining <= 0n) throw new Error('Fair Launch project token deposit is already complete')
  return remaining.toString()
}

export function isUnresolvedWalletTransactionError(error) {
  return error?.code === WALLET_ERROR_CODES.TRANSACTION_BROADCAST_UNCERTAIN
    || error?.code === WALLET_ERROR_CODES.TRANSACTION_RECEIPT_TIMEOUT
}

export function unresolvedWalletTransaction(error) {
  if (!isUnresolvedWalletTransactionError(error)) return null
  return {
    status: 'unresolved',
    transactionHash: typeof error?.transactionHash === 'string' ? error.transactionHash : '',
    doNotResend: true,
  }
}
