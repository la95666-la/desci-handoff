import assert from 'node:assert/strict'
import test from 'node:test'
import { Interface, keccak256 } from 'ethers'
import { comparePurchaseEstimateWithCall, comparePurchaseRpcPaths, describePurchaseError, purchaseErrorDetails } from '../src/services/purchaseDiagnostics.js'

const errors = new Interface([
  'error InvalidProjectStatus(uint8)',
  'error ExceedsMaxPerUser()',
  'error Error(string)',
])

test('a recognized contract revert is decoded without leaking wallet metadata', () => {
  const error = new Error('execution reverted (unknown custom error)')
  error.code = 'CALL_EXCEPTION'
  error.purchasePhase = 'estimate'
  error.info = { error: { data: errors.encodeErrorResult('InvalidProjectStatus', [2]), message: 'secret wallet metadata' } }
  assert.deepEqual(describePurchaseError(error), {
    message: '项目链上状态不允许认购。',
    diagnostic: 'PURCHASE/ESTIMATE/InvalidProjectStatus(2)',
  })
  assert.equal(describePurchaseError(error, 'en').message, 'The project is not open for purchases on-chain.')
})

test('recognized ERC20 string revert identifies allowance but never echoes arbitrary text', () => {
  const error = { purchasePhase: 'estimate', data: errors.encodeErrorResult('Error', ['ERC20: insufficient allowance']) }
  assert.match(describePurchaseError(error).message, /授权额度不足/)
  assert.equal(describePurchaseError(error).diagnostic, 'PURCHASE/ESTIMATE/ERC20Allowance')
  assert.deepEqual(describePurchaseError({ purchasePhase: 'estimate', data: errors.encodeErrorResult('Error', ['private secret']) }), {
    message: '合约预检查未通过；请提供下方诊断信息，勿盲目重试。',
    diagnostic: 'PURCHASE/ESTIMATE/UnknownRevert',
  })
})

test('preflight and estimate have different diagnostic stages', () => {
  const error = { data: errors.encodeErrorResult('ExceedsMaxPerUser', []) }
  assert.equal(describePurchaseError(error).diagnostic, 'PURCHASE/PREFLIGHT/ExceedsMaxPerUser')
  assert.equal(describePurchaseError({ purchasePhase: 'estimate', code: 'RPC_ERROR' }).diagnostic, 'PURCHASE/ESTIMATE/EstimateUnavailable')
})

test('send failures remain uncertain even if the wallet disguises them as CALL_EXCEPTION', () => {
  const error = { purchasePhase: 'send', code: 'CALL_EXCEPTION', data: errors.encodeErrorResult('InvalidProjectStatus', [2]) }
  assert.equal(describePurchaseError(error).diagnostic, 'PURCHASE/SEND/UNKNOWN')
  assert.match(describePurchaseError(error).message, /勿立即重试/)
  assert.equal(describePurchaseError({ purchasePhase: 'send', code: 'TRANSACTION_REVERTED' }).diagnostic, 'PURCHASE/RECEIPT/REVERTED')
  assert.equal(describePurchaseError({ purchasePhase: 'send', code: 'TRANSACTION_RECEIPT_TIMEOUT' }).diagnostic, 'PURCHASE/RECEIPT/TIMEOUT')
  assert.equal(describePurchaseError({ purchasePhase: 'send', code: 4001 }).diagnostic, 'PURCHASE/SEND/CANCELLED')
  assert.equal(describePurchaseError({ purchasePhase: 'preflight', code: 'NETWORK_MISMATCH' }).diagnostic, 'PURCHASE/PREFLIGHT/NETWORK_MISMATCH')
  assert.equal(describePurchaseError({ purchasePhase: 'estimate', code: 'INSUFFICIENT_FUNDS' }).diagnostic, 'PURCHASE/ESTIMATE/INSUFFICIENT_FUNDS')
})

test('malformed and hostile errors produce only fixed strings', () => {
  const error = { purchasePhase: 'estimate', code: 'CALL_EXCEPTION', message: 'execution reverted private-key: very-secret', data: '0x1234' }
  const result = describePurchaseError(error)
  assert.equal(result.diagnostic, 'PURCHASE/ESTIMATE/UnknownRevert')
  assert.doesNotMatch(JSON.stringify(result), /very-secret|private-key/)
  assert.equal(describePurchaseError({ purchasePhase: 'estimate', data: '0x12345678' }).diagnostic, 'PURCHASE/ESTIMATE/UnknownRevert')
  const cyclic = { purchasePhase: 'estimate', data: '0x' }
  cyclic.cause = cyclic
  assert.equal(describePurchaseError(cyclic).diagnostic, 'PURCHASE/ESTIMATE/EstimateUnavailable')
})

test('expanded details expose bounded estimate evidence, not wallet metadata or raw revert strings', () => {
  const error = {
    code: 'CALL_EXCEPTION', purchasePhase: 'estimate',
    message: 'execution reverted with private-key: secret-token',
    info: { error: { code: -32000, data: errors.encodeErrorResult('Error', ['BEP20: transfer amount exceeds allowance']), message: 'Bearer secret-token' } },
    transaction: { data: '0xdeadbeef', signature: 'private-key: secret-token' },
  }
  const context = {
    projectId: '41242200037654529', chainId: 56, amount: '5',
    uiAddress: '0xDB17d1787955f8d49e9D3797a60EDf861Dd4A646',
    signerAddress: '0xdb17d1787955f8d49e9d3797a60edf861dd4a646',
  }
  const details = purchaseErrorDetails(error, context)
  assert.ok(details.some((row) => row.label === 'UI / signer account' && row.value === 'match'))
  assert.ok(details.some((row) => row.label === 'error.info.error.revert selector' && row.value === '0x08c379a0'))
  assert.ok(details.some((row) => row.label === 'error.info.error.code' && row.value === '-32000'))
  assert.ok(details.some((row) => row.value === 'payment allowance insufficient'))
  assert.doesNotMatch(JSON.stringify(details), /secret-token|private-key|deadbeef|BEP20: transfer amount/i)
  assert.ok(details.length <= 30)
})

test('details never display arbitrary RPC text, unknown calldata, circular graphs or accounts as errors', () => {
  const error = { code: 'CALL_EXCEPTION', purchasePhase: 'estimate', message: 'secret-token', data: '0x12345678' }
  error.cause = { code: 'ENOTREAL', message: 'private-key: secret-token', data: { originalError: { signature: 'secret-token' } } }
  error.cause.error = error
  const details = purchaseErrorDetails(error, { projectId: 'not-a-project-id', amount: '0xsecret', signerAddress: 'fake' })
  assert.ok(details.some((row) => row.value === '0x12345678'))
  assert.ok(details.some((row) => row.value === 'not available'))
  assert.doesNotMatch(JSON.stringify(details), /secret-token|private-key|not-a-project-id|fake|ENOTREAL/)
})

test('wallet eth_call comparison is read-only, same transaction, and cannot authorize sending', async () => {
  const tx = { from: '0xDB17d1787955f8d49e9D3797a60EDf861Dd4A646', to: '0x962256B9f09Dfd6f2C20891FeDb5163Dd03183C1', data: '0x12345678', value: 0n }
  const observed = []
  const provider = { async call(request) { observed.push(request); return '0x' }, async sendTransaction() { throw new Error('must not send') } }
  const result = await comparePurchaseEstimateWithCall(provider, tx)
  assert.deepEqual(result, { outcome: 'passed' })
  assert.deepEqual(observed, [tx])
  const details = purchaseErrorDetails({ purchasePhase: 'estimate', code: 'CALL_EXCEPTION' }, { walletCallComparison: result })
  assert.ok(details.some((row) => row.label === 'Wallet eth_call comparison' && row.value === 'passed'))
  assert.doesNotMatch(JSON.stringify(details), /12345678/)
})

test('wallet eth_call comparison distinguishes revert, unavailable and timeout without leaking RPC text', async () => {
  const nested = { code: 'CALL_EXCEPTION', info: { error: { data: errors.encodeErrorResult('ExceedsMaxPerUser', []), message: 'Bearer private-token' } } }
  const reverted = await comparePurchaseEstimateWithCall({ call: async () => { throw nested } }, {})
  assert.equal(reverted.outcome, 'reverted')
  assert.equal(reverted.decoded, 'ExceedsMaxPerUser')
  assert.ok(purchaseErrorDetails({}, { walletCallComparison: reverted }).some((row) => row.value === 'ExceedsMaxPerUser'))
  assert.doesNotMatch(JSON.stringify(reverted), /private-token/)
  const missingData = await comparePurchaseEstimateWithCall({ call: async () => { throw { code: 'CALL_EXCEPTION', message: 'missing revert data' } } }, {})
  assert.equal(missingData.outcome, 'inconclusive')
  assert.ok(purchaseErrorDetails({}, { walletCallComparison: missingData }).some((row) => row.value === 'inconclusive'))
  assert.deepEqual(await comparePurchaseEstimateWithCall({}, {}), { outcome: 'unavailable' })
  const failing = await comparePurchaseEstimateWithCall({ call: async () => { throw { code: -32000, message: 'Bearer private-token' } } }, {})
  assert.equal(failing.outcome, 'unavailable')
  assert.doesNotMatch(JSON.stringify(failing), /private-token/)
  const timedOut = await comparePurchaseEstimateWithCall({ call: () => new Promise(() => {}) }, {}, 5)
  assert.deepEqual(timedOut, { outcome: 'timeout' })
})

test('read-only public RPC diagnostic is primary and never touches the wallet when healthy', async () => {
  const tx = { from: '0xDB17d1787955f8d49e9D3797a60EDf861Dd4A646', to: '0x962256B9f09Dfd6f2C20891FeDb5163Dd03183C1', data: '0x12345678', value: 0n }
  const requests = []
  const walletCalls = []
  const provider = { getNetwork: async () => ({ chainId: 56n }), getBlockNumber: async () => 100, getBlock: async () => ({ timestamp: 1791871203 }), call: async (request) => { walletCalls.push(request); throw { code: 'CALL_EXCEPTION', message: 'missing revert data' } } }
  const fetcher = async (url, options) => {
    assert.equal(url, 'https://bsc-dataseed.bnbchain.org')
    assert.equal(options.method, 'POST')
    const req = JSON.parse(options.body); requests.push(req)
    return { ok: true, json: async () => ({ result: ({ eth_chainId: '0x38', eth_blockNumber: '0x65', eth_getBlockByNumber: { timestamp: '0x6acb9073' }, eth_call: '0x' })[req.method] }) }
  }
  const comparison = await comparePurchaseRpcPaths(provider, tx, 56, fetcher)
  assert.equal(comparison.walletBlock, 'unavailable')
  assert.equal(comparison.publicBlock, '101')
  assert.equal(comparison.comparedBlock, '101')
  assert.equal(comparison.blockAlignment, 'not comparable')
  assert.equal(comparison.walletCallAtBlock, undefined)
  assert.equal(comparison.publicCall, 'passed')
  assert.deepEqual(walletCalls, [])
  assert.equal(comparison.dataHash, keccak256(tx.data))
  assert.deepEqual(requests.map((x) => x.method), ['eth_chainId', 'eth_blockNumber', 'eth_getBlockByNumber', 'eth_call'])
  assert.deepEqual(requests.at(-1).params, [{ from: tx.from, to: tx.to, data: tx.data, value: '0x0' }, '0x65'])
  const details = purchaseErrorDetails({ code: 'CALL_EXCEPTION', purchasePhase: 'estimate' }, { rpcPathComparison: comparison })
  assert.ok(details.some((row) => row.label === 'Call data hash (keccak256)' && row.value === keccak256(tx.data)))
  assert.ok(details.some((row) => row.label === 'Public RPC eth_call (read-only)' && row.value === 'passed'))
  assert.ok(!details.some((row) => row.label === 'Wallet eth_call at compared block'))
  assert.doesNotMatch(JSON.stringify(details), /12345678/)
  assert.ok(details.length <= 30)
})

test('unmatched public block is clearly labeled, and untrusted RPC content is not displayed', async () => {
  const tx = { from: '0xDB17d1787955f8d49e9D3797a60EDf861Dd4A646', to: '0x962256B9f09Dfd6f2C20891FeDb5163Dd03183C1', data: '0x12345678', value: 0n }
  const methods = []
  const result = await comparePurchaseRpcPaths({ getNetwork: async () => ({ chainId: 56 }), getBlockNumber: async () => 1, getBlock: async () => ({ timestamp: 100 }) }, tx, 56, async (_, opts) => {
    const method = JSON.parse(opts.body).method; methods.push(method)
    return { ok: true, json: async () => ({ result: ({ eth_chainId: '0x38', eth_blockNumber: '0x80', eth_getBlockByNumber: { timestamp: '0x100' } })[method], error: method === 'eth_call' ? { code: -32000, message: 'Bearer private-token', data: '0x12345678' } : undefined }) }
  })
  assert.equal(result.blockAlignment, 'not comparable')
  assert.equal(result.publicCall, 'reverted')
  assert.deepEqual(methods, ['eth_chainId', 'eth_blockNumber', 'eth_getBlockByNumber', 'eth_call'])
  const filtered = JSON.stringify(purchaseErrorDetails({}, { rpcPathComparison: result }))
  assert.doesNotMatch(filtered, /private-token|Bearer/)
  assert.match(filtered, /Public RPC selector/)
  assert.deepEqual((await comparePurchaseRpcPaths({}, tx, 97, () => { throw new Error('must not call') })).publicCall, 'unavailable')
})

test('a stalled wallet observer cannot prevent the independent public RPC observation', async () => {
  const tx = { from: '0xDB17d1787955f8d49e9D3797a60EDf861Dd4A646', to: '0x962256B9f09Dfd6f2C20891FeDb5163Dd03183C1', data: '0x12345678', value: 0n }
  const methods = []
  const result = await comparePurchaseRpcPaths(
    { getNetwork: () => new Promise(() => {}), getBlockNumber: () => new Promise(() => {}) },
    tx,
    56,
    async (_, opts) => {
      const method = JSON.parse(opts.body).method; methods.push(method)
      return { ok: true, json: async () => ({ result: ({ eth_chainId: '0x38', eth_blockNumber: '0x64', eth_getBlockByNumber: { timestamp: '0x6acb9073' }, eth_call: '0x' })[method] }) }
    },
    20,
  )
  assert.equal(result.walletNetwork, 'unavailable')
  assert.equal(result.publicNetwork, '56')
  assert.equal(result.publicBlock, '100')
  assert.equal(result.blockAlignment, 'not comparable')
  assert.equal(result.publicCall, 'passed')
  assert.deepEqual(methods, ['eth_chainId', 'eth_blockNumber', 'eth_getBlockByNumber', 'eth_call'])
})

test('public RPC pending falls back to the bounded wallet read-only observer', async () => {
  const tx = { from: '0xDB17d1787955f8d49e9D3797a60EDf861Dd4A646', to: '0x962256B9f09Dfd6f2C20891FeDb5163Dd03183C1', data: '0x12345678', value: 0n }
  const walletCalls = []
  const provider = {
    getNetwork: async () => ({ chainId: 56 }),
    getBlockNumber: async () => 100,
    getBlock: async () => ({ timestamp: 1791871203 }),
    call: async (request) => { walletCalls.push(request); return '0x' },
  }
  const result = await comparePurchaseRpcPaths(provider, tx, 56, () => new Promise(() => {}), 20)
  assert.equal(result.publicNetwork, 'unavailable')
  assert.equal(result.publicBlock, 'unavailable')
  assert.equal(result.walletNetwork, '56')
  assert.equal(result.walletBlock, '100')
  assert.equal(result.walletCallAtBlock, 'passed')
  assert.deepEqual(walletCalls, [{ ...tx, blockTag: '0x64' }])
})
