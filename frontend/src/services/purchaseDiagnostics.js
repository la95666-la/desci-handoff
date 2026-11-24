import { Interface, keccak256 } from 'ethers'

// Only recognized contract errors are decoded. Never put a wallet/RPC's free-form
// message, calldata, signature, or nested provider object in a user-facing report.
const idoErrors = new Interface([
  'error InvalidProjectStatus(uint8)',
  'error ProjectNotStarted()',
  'error ProjectEndedAlready()',
  'error ExceedsHardCap()',
  'error ExceedsMaxPerUser()',
  'error InvalidAmount()',
  'error TokenAmountTooSmall()',
  'error InsufficientProjectTokens()',
  'error PaymentTokenNotAllowed(address)',
  'error ProjectTokenNotAllowed(address)',
  'error TokenDecimalsChanged(address,uint8,uint8)',
  'error UnsupportedTokenBehavior(address,uint256,uint256)',
  'error TokenReserveInsolvent(address,uint256,uint256)',
  'error EnforcedPause()',
  'error ERC20InsufficientAllowance(address,uint256,uint256)',
  'error ERC20InsufficientBalance(address,uint256,uint256)',
  'error Error(string)',
  'error Panic(uint256)',
])

const messages = {
  InvalidProjectStatus: ['项目链上状态不允许认购。', 'The project is not open for purchases on-chain.'],
  ProjectNotStarted: ['链上认购尚未开始。', 'The on-chain sale has not started.'],
  ProjectEndedAlready: ['链上认购时间已结束。', 'The on-chain sale has ended.'],
  ExceedsHardCap: ['认购金额超过项目剩余额度。', 'The amount exceeds the remaining project cap.'],
  ExceedsMaxPerUser: ['认购金额超过该钱包的剩余额度。', 'The amount exceeds this wallet’s remaining limit.'],
  InvalidAmount: ['认购金额无效。', 'Invalid purchase amount.'],
  TokenAmountTooSmall: ['金额过小，无法兑换项目代币。', 'The amount is too small to buy project tokens.'],
  InsufficientProjectTokens: ['项目代币储备不足。', 'Insufficient project token reserve.'],
  PaymentTokenNotAllowed: ['支付币当前不可用于认购。', 'The payment token is not currently allowed.'],
  ProjectTokenNotAllowed: ['项目代币当前不可用于认购。', 'The project token is not currently allowed.'],
  TokenDecimalsChanged: ['代币精度已变化，认购被阻止。', 'Token decimals changed; purchase blocked.'],
  UnsupportedTokenBehavior: ['支付币转账行为不符合合约要求。', 'The payment token transfer did not meet contract requirements.'],
  TokenReserveInsolvent: ['合约代币储备校验失败。', 'The contract token reserve check failed.'],
  EnforcedPause: ['合约目前已暂停。', 'The contract is paused.'],
  ERC20Allowance: ['支付币授权额度不足，请核对授权和认购金额。', 'Payment token allowance is insufficient; check the approval and amount.'],
  ERC20Balance: ['支付币余额不足，请核对钱包余额。', 'Payment token balance is insufficient; check your wallet balance.'],
  ERC20InsufficientAllowance: ['支付币授权额度不足，请核对授权和认购金额。', 'Payment token allowance is insufficient; check the approval and amount.'],
  ERC20InsufficientBalance: ['支付币余额不足，请核对钱包余额。', 'Payment token balance is insufficient; check your wallet balance.'],
  UnknownRevert: ['合约预检查未通过；请提供下方诊断信息，勿盲目重试。', 'Contract preflight failed. Share the diagnostic below; do not retry blindly.'],
  MinedRevert: ['交易已上链但执行失败；请查看下方交易哈希。', 'The transaction was mined but reverted. Check the transaction hash below.'],
  ReceiptTimeout: ['已获得交易哈希，但等待回执超时；请先在链上核对，勿重复提交。', 'The transaction has a hash but confirmation timed out. Check it on-chain before retrying.'],
  Cancelled: ['你已在钱包中取消认购。', 'You cancelled the purchase in your wallet.'],
  WrongNetwork: ['钱包网络与项目所在链不一致，请核对网络。', 'The wallet network does not match this project. Check the network.'],
  InsufficientGas: ['钱包的原生币不足以支付交易手续费。', 'Insufficient native coin to cover the transaction fee.'],
  EstimateUnavailable: ['钱包节点无法完成交易估算；请提供下方诊断信息。', 'The wallet RPC could not estimate this transaction. Share the diagnostic below.'],
  SendUnknown: ['钱包未返回明确结果；请先检查钱包活动和链上记录，勿立即重试。', 'The wallet returned no definitive result. Check wallet activity and on-chain records before retrying.'],
  PreflightUnknown: ['认购提交前检查失败；请提供下方诊断信息。', 'A pre-submission check failed. Share the diagnostic below.'],
}

const standardReason = (reason) => {
  const value = String(reason || '').toLowerCase()
  if (/^(erc20: insufficient allowance|erc20insufficientallowance)/.test(value)) return 'ERC20Allowance'
  if (/^(erc20: transfer amount exceeds balance|erc20: insufficient balance|erc20insufficientbalance)/.test(value)) return 'ERC20Balance'
  return null
}

function decodeRevert(error) {
  const queue = [error]
  const visited = new Set()
  for (let n = 0; queue.length && n < 16; n++) {
    const current = queue.shift()
    if (!current || typeof current !== 'object' || visited.has(current)) continue
    visited.add(current)
    const data = current.data
    if (typeof data === 'string' && /^0x[0-9a-fA-F]{8,2048}$/.test(data)) {
      try {
        const parsed = idoErrors.parseError(data)
        if (parsed?.name === 'Error') {
          const name = standardReason(parsed.args[0])
          if (name) return { name }
          return { name: 'UnknownRevert' }
        } else if (parsed?.name === 'Panic') {
          return { name: 'UnknownRevert' }
        } else if (parsed && Object.hasOwn(messages, parsed.name)) {
          // No addresses, token amounts or arbitrary revert text in the UI.
          const status = parsed.name === 'InvalidProjectStatus' ? Number(parsed.args[0]) : null
          return { name: parsed.name, status: Number.isInteger(status) && status >= 0 && status <= 4 ? status : null }
        }
      } catch { /* Unrecognized/malformed revert bytes. */ }
    }
    const named = standardReason(current.reason)
    if (named) return { name: named }
    for (const key of ['error', 'info', 'cause']) {
      if (current[key] && typeof current[key] === 'object') queue.push(current[key])
    }
  }
  return null
}

/** Safe, bounded, screenshot-friendly purchase diagnostic (no raw provider error). */
export function describePurchaseError(error, language = 'zh') {
  const zh = language !== 'en'
  const phase = ['estimate', 'send'].includes(error?.purchasePhase) ? error.purchasePhase : 'preflight'
  const code = error?.code
  if (code === 'TRANSACTION_REVERTED') return { message: messages.MinedRevert[zh ? 0 : 1], diagnostic: 'PURCHASE/RECEIPT/REVERTED' }
  if (code === 'TRANSACTION_RECEIPT_TIMEOUT') return { message: messages.ReceiptTimeout[zh ? 0 : 1], diagnostic: 'PURCHASE/RECEIPT/TIMEOUT' }
  if (code === 'ACTION_REJECTED' || code === 4001) return { message: messages.Cancelled[zh ? 0 : 1], diagnostic: `PURCHASE/${phase.toUpperCase()}/CANCELLED` }
  if (code === 'NETWORK_MISMATCH') return { message: messages.WrongNetwork[zh ? 0 : 1], diagnostic: 'PURCHASE/PREFLIGHT/NETWORK_MISMATCH' }
  if (code === 'INSUFFICIENT_FUNDS') return { message: messages.InsufficientGas[zh ? 0 : 1], diagnostic: `PURCHASE/${phase.toUpperCase()}/INSUFFICIENT_FUNDS` }
  // A wallet-owned send may already have broadcast even if it did not return a hash.
  if (phase === 'send' || code === 'TRANSACTION_BROADCAST_UNCERTAIN') {
    return { message: messages.SendUnknown[zh ? 0 : 1], diagnostic: 'PURCHASE/SEND/UNKNOWN' }
  }
  const decoded = decodeRevert(error)
  if (decoded) {
    const detail = decoded.status == null ? decoded.name : `${decoded.name}(${decoded.status})`
    return { message: messages[decoded.name][zh ? 0 : 1], diagnostic: `PURCHASE/${phase.toUpperCase()}/${detail}` }
  }
  const reverted = code === 'CALL_EXCEPTION' ||
    (typeof error?.data === 'string' && /^0x[0-9a-fA-F]{8,2048}$/.test(error.data)) ||
    /execution reverted|call_exception/i.test(String(error?.shortMessage || error?.message || ''))
  const kind = reverted ? 'UnknownRevert' : phase === 'estimate' ? 'EstimateUnavailable' : 'PreflightUnknown'
  return { message: messages[kind][zh ? 0 : 1], diagnostic: `PURCHASE/${phase.toUpperCase()}/${kind}` }
}

// Screenshot-safe, bounded evidence. Wallet error objects may contain request
// headers, signed payloads, calldata and arbitrary provider text: never render
// or JSON.stringify them. Retain only allowlisted scalar codes and revert selectors.
const SAFE_CODE = /^(?:-?\d{1,8}|[A-Z][A-Z0-9_]{0,39})$/
const KNOWN_CODES = new Set(['CALL_EXCEPTION', 'INSUFFICIENT_FUNDS', 'NETWORK_MISMATCH', 'RPC_ERROR', 'GAS_ESTIMATE_TIMEOUT', 'TRANSACTION_REVERTED', 'TRANSACTION_RECEIPT_TIMEOUT', 'TRANSACTION_BROADCAST_UNCERTAIN', 'ACTION_REJECTED'])
const HEX_DATA = /^0x[0-9a-fA-F]+$/
const PROJECT_ID = /^(?:0|[1-9]\d{0,19})$/
const ADDRESS = /^0x[0-9a-fA-F]{40}$/
const BSC_DIAGNOSTIC_RPC = 'https://bsc-dataseed.bnbchain.org'
// Public mainnet RPC is the primary read-only diagnostic observer. The wallet
// provider is consulted only when the official endpoint is unavailable or
// times out; neither path can sign or broadcast a transaction.
export async function comparePurchaseRpcPaths(walletProvider, transaction, expectedChainId, fetcher = fetch, timeoutMs = 8000) {
  const report = { walletNetwork: 'unavailable', walletBlock: 'unavailable', publicNetwork: 'unavailable', publicBlock: 'unavailable', publicCall: 'unavailable', blockAlignment: 'unavailable' }
  if (Number(expectedChainId) !== 56 || !ADDRESS.test(String(transaction?.from ?? '')) || !ADDRESS.test(String(transaction?.to ?? '')) || !/^0x[0-9a-fA-F]*$/.test(String(transaction?.data ?? '')) || transaction.value !== 0n) return report
  report.from = transaction.from
  report.to = transaction.to
  report.value = '0'
  report.dataHash = keccak256(transaction.data)
  let publicTimer
  const abort = new AbortController()
  const publicDeadline = new Promise((_, reject) => { publicTimer = setTimeout(() => { abort.abort(); reject(new Error('diagnostic timeout')) }, timeoutMs) })
  const boundedPublic = (promise) => Promise.race([promise, publicDeadline])
  try {
    const rpc = async (method, params) => {
      const response = await boundedPublic(fetcher(BSC_DIAGNOSTIC_RPC, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, signal: abort.signal,
        body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params }),
      }))
      if (!response.ok) throw new Error('diagnostic RPC unavailable')
      return boundedPublic(response.json())
    }
    const chain = await rpc('eth_chainId', [])
    if (chain?.result !== '0x38') throw new Error('diagnostic RPC chain mismatch')
    report.publicNetwork = '56'
    const latest = await rpc('eth_blockNumber', [])
    if (!/^0x[0-9a-fA-F]{1,16}$/.test(String(latest?.result ?? ''))) throw new Error('diagnostic block unavailable')
    const height = Number(BigInt(latest.result))
    if (!Number.isSafeInteger(height) || height <= 0) throw new Error('diagnostic block unavailable')
    report.publicBlock = String(height)
    report.blockAlignment = 'not comparable'
    const blockTag = latest.result
    report.comparedBlock = String(height)
    try {
      const observed = await rpc('eth_getBlockByNumber', [blockTag, false])
      if (/^0x[0-9a-fA-F]{1,16}$/.test(String(observed?.result?.timestamp ?? ''))) {
        const seconds = Number(BigInt(observed.result.timestamp))
        if (Number.isSafeInteger(seconds) && seconds > 0) report.publicTime = new Date(seconds * 1000).toISOString()
      }
    } catch { /* Continue with a number-only observation. */ }
    const result = await rpc('eth_call', [{ from: transaction.from, to: transaction.to, data: transaction.data, value: '0x0' }, blockTag])
    if (result?.error && typeof result.error === 'object') {
      // Reuse the existing allowlist; never retain the free-form RPC response.
      const safe = await comparePurchaseEstimateWithCall({ call: async () => { throw { code: result.error.code, data: result.error.data, message: result.error.message } } }, transaction, 1000)
      report.publicCall = safe.outcome
      if (safe.code) report.publicCode = safe.code
      if (safe.selector) report.publicSelector = safe.selector
      if (safe.decoded) report.publicDecoded = safe.decoded
    } else if (typeof result?.result === 'string' && (result.result === '0x' || HEX_DATA.test(result.result))) report.publicCall = 'passed'
    return report
  } catch { /* Fall through to the bounded wallet observer. */ }
  finally { clearTimeout(publicTimer); abort.abort() }

  const walletBudget = Math.min(3000, Math.max(1, Math.floor(timeoutMs / 2)))
  let walletTimer
  const walletDeadline = new Promise((_, reject) => { walletTimer = setTimeout(() => reject(new Error('wallet diagnostic timeout')), walletBudget) })
  const boundedWallet = (promise) => Promise.race([promise, walletDeadline])
  try {
    if (!walletProvider?.getNetwork || !walletProvider?.getBlockNumber) return report
    const network = await boundedWallet(walletProvider.getNetwork())
    if (Number(network?.chainId) !== 56) return report
    report.walletNetwork = '56'
    const height = await boundedWallet(walletProvider.getBlockNumber())
    if (!Number.isSafeInteger(height) || height <= 0) return report
    report.walletBlock = String(height)
    report.comparedBlock = String(height)
    report.blockAlignment = 'not comparable'
    try {
      const block = await boundedWallet(walletProvider.getBlock(height))
      if (Number.isSafeInteger(Number(block?.timestamp)) && Number(block.timestamp) > 0) report.walletTime = new Date(Number(block.timestamp) * 1000).toISOString()
    } catch { /* A block number is still useful if the timestamp is unavailable. */ }
    const walletAtBlock = await comparePurchaseEstimateWithCall(walletProvider, { ...transaction, blockTag: '0x' + height.toString(16) }, walletBudget)
    report.walletCallAtBlock = walletAtBlock.outcome
    if (walletAtBlock.code) report.walletCallAtBlockCode = walletAtBlock.code
  } catch { /* Wallet fallback remains unavailable. */ }
  finally { clearTimeout(walletTimer) }
  return report
}
function safeProviderReason(value) {
  const text = String(value ?? '').toLowerCase()
  if (/bep20: transfer amount exceeds allowance|erc20: insufficient allowance|erc20insufficientallowance/.test(text)) return 'payment allowance insufficient'
  if (/bep20: transfer amount exceeds balance|erc20: insufficient balance|erc20insufficientbalance/.test(text)) return 'payment balance insufficient'
  if (/insufficient funds|not enough funds/.test(text)) return 'native gas balance insufficient'
  if (/gas required exceeds allowance/.test(text)) return 'gas limit exceeded'
  if (/missing revert data/.test(text)) return 'revert data missing'
  if (/execution reverted|call_exception/.test(text)) return 'execution reverted (reason unavailable)'
  if (/timeout|timed out/.test(text)) return 'RPC timeout'
  if (/method not found|unsupported method/.test(text)) return 'RPC method unsupported'
  return null
}

/** Compare the wallet's read-only eth_call with its failed gas estimate.
 * Never treat a successful call as permission to send a transaction.
 */
export async function comparePurchaseEstimateWithCall(provider, transaction, timeoutMs = 8000) {
  if (!provider || typeof provider.call !== 'function') return { outcome: 'unavailable' }
  let timer
  const timeout = Symbol('wallet call probe timeout')
  try {
    // Ethers v6 call() takes one transaction object. Keep the same from/to/data/value
    // as estimateGas; do not supply a gas limit or use a separate RPC identity.
    const result = await Promise.race([
      provider.call(transaction),
      new Promise((_, reject) => { timer = setTimeout(() => reject(timeout), timeoutMs) }),
    ])
    return result === '0x' || (typeof result === 'string' && HEX_DATA.test(result))
      ? { outcome: 'passed' }
      : { outcome: 'unavailable' }
  } catch (error) {
    if (error === timeout) return { outcome: 'timeout' }
    let details
    try { details = purchaseErrorDetails(error) } catch { return { outcome: 'unavailable' } }
    const selector = details.find((row) => row.label.endsWith('.revert selector'))?.value
    const decoded = details.find((row) => row.label.endsWith('.decoded'))?.value
    const code = details.find((row) => row.label.endsWith('.code'))?.value
    const reason = details.find((row) => row.label.endsWith('.reason'))?.value
    return {
      outcome: selector || decoded ? 'reverted' : code === 'CALL_EXCEPTION' ? 'inconclusive' : 'unavailable',
      ...(code ? { code } : {}),
      ...(selector ? { selector } : {}),
      ...(decoded ? { decoded } : {}),
      ...(reason ? { reason } : {}),
    }
  } finally {
    clearTimeout(timer)
  }
}

export function purchaseErrorDetails(error, context = {}) {
  const entries = []
  const add = (label, value) => { if (entries.length < 30) entries.push({ label, value: String(value) }) }
  add('Stage', ['preflight', 'estimate', 'send'].includes(error?.purchasePhase) ? error.purchasePhase : 'preflight')
  add('Time (UTC)', new Date().toISOString())
  const projectId = String(context.projectId ?? '')
  if (PROJECT_ID.test(projectId)) add('Project ID', projectId)
  const chainId = Number(context.chainId)
  if (Number.isSafeInteger(chainId) && chainId > 0) add('Expected chain ID', chainId)
  const amount = String(context.amount ?? '')
  if (/^(?:0|[1-9]\d{0,17})(?:\.\d{1,18})?$/.test(amount)) add('Amount (payment token)', amount)
  const uiAddress = String(context.uiAddress ?? '')
  const signerAddress = String(context.signerAddress ?? '')
  if (ADDRESS.test(uiAddress)) add('UI account', uiAddress)
  if (ADDRESS.test(signerAddress)) add('Estimate from', signerAddress)
  if (ADDRESS.test(uiAddress) && ADDRESS.test(signerAddress)) {
    add('UI / signer account', uiAddress.toLowerCase() === signerAddress.toLowerCase() ? 'match' : 'mismatch')
  } else add('UI / signer account', 'not available')

  const comparison = context.walletCallComparison
  if (comparison && ['passed', 'reverted', 'inconclusive', 'unavailable', 'timeout'].includes(comparison.outcome)) {
    add('Wallet eth_call comparison', comparison.outcome)
    if (SAFE_CODE.test(String(comparison.code ?? '')) && (/^-?\d{1,8}$/.test(String(comparison.code)) || KNOWN_CODES.has(comparison.code))) add('Wallet eth_call code', comparison.code)
    if (/^0x[0-9a-fA-F]{8}$/.test(String(comparison.selector ?? ''))) add('Wallet eth_call selector', String(comparison.selector).toLowerCase())
    if (Object.hasOwn(messages, String(comparison.decoded ?? '')) || ['payment allowance insufficient', 'payment balance insufficient'].includes(comparison.decoded)) add('Wallet eth_call decoded', comparison.decoded)
    if (['payment allowance insufficient', 'payment balance insufficient', 'native gas balance insufficient', 'gas limit exceeded', 'revert data missing', 'execution reverted (reason unavailable)', 'RPC timeout', 'RPC method unsupported'].includes(comparison.reason)) add('Wallet eth_call reason', comparison.reason)
  }

  const paths = context.rpcPathComparison
  if (paths && typeof paths === 'object') {
    add('Wallet RPC endpoint', 'not exposed by wallet provider')
    add('Independent RPC', 'bsc-dataseed.bnbchain.org (BSC mainnet)')
    if (ADDRESS.test(String(paths.from ?? ''))) add('Read-only from', paths.from)
    if (ADDRESS.test(String(paths.to ?? ''))) add('Read-only contract', paths.to)
    if (paths.value === '0') add('Read-only value (wei)', '0')
    if (/^0x[0-9a-fA-F]{64}$/.test(String(paths.dataHash ?? ''))) add('Call data hash (keccak256)', paths.dataHash)
    for (const [label, value] of [['Wallet observed chain', paths.walletNetwork], ['Public RPC chain', paths.publicNetwork]]) {
      if (value === '56' || value === 'unavailable') add(label, value)
    }
    for (const [label, value] of [['Wallet block', paths.walletBlock], ['Public RPC block', paths.publicBlock], ['Compared block', paths.comparedBlock]]) {
      if (value === 'unavailable' || /^(?:0|[1-9]\d{0,15})$/.test(String(value ?? ''))) add(label, value)
    }
    for (const [label, value] of [['Wallet block time (UTC)', paths.walletTime], ['Public block time (UTC)', paths.publicTime]]) {
      if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.000Z$/.test(String(value ?? ''))) add(label, value)
    }
    if (['within 12 blocks', 'not comparable', 'unavailable'].includes(paths.blockAlignment)) add('Block alignment', paths.blockAlignment)
    if (['passed', 'reverted', 'inconclusive', 'unavailable', 'timeout'].includes(paths.walletCallAtBlock)) add('Wallet eth_call at compared block', paths.walletCallAtBlock)
    if (SAFE_CODE.test(String(paths.walletCallAtBlockCode ?? '')) && (/^-?\d{1,8}$/.test(String(paths.walletCallAtBlockCode)) || KNOWN_CODES.has(paths.walletCallAtBlockCode))) add('Wallet same-block code', paths.walletCallAtBlockCode)
    if (['passed', 'reverted', 'inconclusive', 'unavailable', 'timeout'].includes(paths.publicCall)) add('Public RPC eth_call (read-only)', paths.publicCall)
    if (SAFE_CODE.test(String(paths.publicCode ?? '')) && (/^-?\d{1,8}$/.test(String(paths.publicCode)) || KNOWN_CODES.has(paths.publicCode))) add('Public RPC code', paths.publicCode)
    if (/^0x[0-9a-fA-F]{8}$/.test(String(paths.publicSelector ?? ''))) add('Public RPC selector', paths.publicSelector)
    if (Object.hasOwn(messages, String(paths.publicDecoded ?? ''))) add('Public RPC decoded', paths.publicDecoded)
  }

  const queue = [{ value: error, path: 'error' }]
  const seen = new Set()
  for (let n = 0; queue.length && n < 16 && entries.length < 30; n++) {
    const { value, path } = queue.shift()
    if (!value || typeof value !== 'object' || seen.has(value)) continue
    seen.add(value)
    const code = String(value.code ?? '')
    if (SAFE_CODE.test(code) && (/^-?\d{1,8}$/.test(code) || KNOWN_CODES.has(code))) add(`${path}.code`, code)
    const reason = safeProviderReason(value.reason) || safeProviderReason(value.shortMessage) || safeProviderReason(value.message)
    if (reason) add(`${path}.reason`, reason)
    const data = value.data
    if (typeof data === 'string' && HEX_DATA.test(data) && data.length >= 10) {
      add(`${path}.revert selector`, data.slice(0, 10).toLowerCase())
      add(`${path}.revert bytes`, (data.length - 2) / 2)
      try {
        const parsed = idoErrors.parseError(data)
        const known = parsed?.name === 'Error'
          ? standardReason(parsed.args[0]) || safeProviderReason(parsed.args[0])
          : parsed?.name
        if (known && (Object.hasOwn(messages, known) || known === 'payment allowance insufficient' || known === 'payment balance insufficient')) add(`${path}.decoded`, known)
      } catch { /* Selector/length still useful for unrecognized data. */ }
    }
    for (const key of ['error', 'info', 'cause', 'data']) {
      try {
        if (value[key] && typeof value[key] === 'object') queue.push({ value: value[key], path: `${path}.${key}` })
      } catch { /* Some wallet proxies throw while reading nested metadata. */ }
    }
  }
  return entries
}
