import { Interface } from 'ethers'
import { v3Error } from './idoV3Wallet.js'

const errors = new Interface([
  'error ERC20InsufficientAllowance(address spender,uint256 allowance,uint256 needed)',
  'error ERC20InsufficientBalance(address sender,uint256 balance,uint256 needed)',
  'error ExceedsMaxPerUser()', 'error ExceedsHardCap()',
  'error WhitelistVersionChanged(uint64 expected,uint64 actual)',
  'error WhitelistProofRequired()', 'error WhitelistNotEnabled()', 'error WhitelistProofInvalid()',
  'error NoTokensToClaim()', 'error NoPurchase()', 'error AlreadyRefunded()',
])

function simulationError(error) {
  const data = typeof error?.data === 'string' ? error.data : error?.data?.data
  let name
  try { name = errors.parseError(data)?.name } catch { /* Some tokens only supply a reason string. */ }
  const codes = {
    ERC20InsufficientAllowance: 'IDO_V3_ALLOWANCE_INSUFFICIENT', ERC20InsufficientBalance: 'IDO_V3_BALANCE_INSUFFICIENT',
    ExceedsMaxPerUser: 'IDO_V3_USER_LIMIT_EXCEEDED', ExceedsHardCap: 'IDO_V3_HARD_CAP_EXCEEDED',
    WhitelistVersionChanged: 'IDO_V3_POLICY_CHANGED', WhitelistProofRequired: 'IDO_V3_POLICY_CHANGED',
    WhitelistNotEnabled: 'IDO_V3_POLICY_CHANGED', WhitelistProofInvalid: 'IDO_V3_PROOF_UNAVAILABLE',
    NoTokensToClaim: 'IDO_V3_ACTION_UNAVAILABLE', NoPurchase: 'IDO_V3_ACTION_UNAVAILABLE', AlreadyRefunded: 'IDO_V3_ACTION_UNAVAILABLE',
  }
  if (codes[name]) return v3Error(codes[name])
  if (/insufficient allowance|allowance exceeded/i.test(String(error?.message || ''))) return v3Error('IDO_V3_ALLOWANCE_INSUFFICIENT')
  if (/transfer amount exceeds balance|insufficient balance/i.test(String(error?.message || ''))) return v3Error('IDO_V3_BALANCE_INSUFFICIENT')
  return v3Error('IDO_V3_STATE_UNAVAILABLE')
}

// eth_estimateGas executes the exact unsigned transaction and rejects reverts.
// One request is both simulation and gas calculation: no preceding eth_call,
// runtime audit, business-state reads or automatic wallet/RPC fallback.
export async function simulateV3Transaction(url, transaction, fetcher = fetch, timeoutMs = 8000) {
  if (!/^0x[0-9a-fA-F]{40}$/.test(String(transaction?.from || '')) ||
      !/^0x[0-9a-fA-F]{40}$/.test(String(transaction?.to || '')) ||
      !/^0x[0-9a-fA-F]{8,}$/.test(String(transaction?.data || '')) || transaction.value !== '0x0') {
    throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE')
  }
  const controller = new AbortController()
  const unavailable = () => Object.assign(v3Error('IDO_V3_STATE_UNAVAILABLE'), { rpcUnavailable: true })
  let timer
  try {
    const operation = (async () => {
      let response
      try { response = await fetcher(url, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, signal: controller.signal,
        body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'eth_estimateGas', params: [transaction] }),
      }) } catch { throw unavailable() }
      if (!response?.ok) {
        if (response?.status === 429 || response?.status >= 500) throw unavailable()
        throw v3Error('IDO_V3_STATE_UNAVAILABLE')
      }
      const payload = await response.json()
      if (payload?.error) throw simulationError(payload.error)
      if (payload?.id !== 1 || !/^0x[0-9a-fA-F]+$/.test(String(payload?.result || ''))) throw v3Error('IDO_V3_STATE_UNAVAILABLE')
      const estimate = BigInt(payload.result)
      const gas = (estimate * 125n + 99n) / 100n
      if (estimate <= 0n || gas > 1000000n) throw v3Error('IDO_V3_STATE_UNAVAILABLE')
      return gas
    })()
    return await Promise.race([operation, new Promise((_, reject) => {
      timer = setTimeout(() => { controller.abort(); reject(unavailable()) }, timeoutMs)
    })])
  } catch (error) {
    if (String(error?.code || '').startsWith('IDO_V3_')) throw error
    throw v3Error('IDO_V3_STATE_UNAVAILABLE')
  } finally { clearTimeout(timer); controller.abort() }
}
