import assert from 'node:assert/strict'
import test from 'node:test'
import { AbiCoder, Interface, id, keccak256 } from 'ethers'
import { IDO_V3_ABI } from '../src/config/idoV3Abi.js'
import { assertV3PolicyUnchanged, assertV3Proof, assertV3Project, pinnedV3WalletRuntime, readV3WalletState, v3ProjectActionState, v3SaleOpen } from '../src/services/idoV3Wallet.js'
import { assertNoV3Pending, clearV3Pending, listV3Pending, recordV3Pending, v3PendingMetadata, v3SubmissionIdentity } from '../src/services/idoV3Pending.js'
import { ContractService } from '../src/services/contract.js'
import { web3Service } from '../src/services/web3.js'

const proxy = '0x1111111111111111111111111111111111111111'
const buyer = '0x2222222222222222222222222222222222222222'
const token = '0x3333333333333333333333333333333333333333'
const payment = '0x4444444444444444444444444444444444444444'
const h = (digit) => `0x${digit.repeat(64)}`
const project = { id: '94713', fundingMode: 'capped_presale', contractVersion: 'ido_v3_stablecoin', chainId: 31337,
  contractAddress: proxy, paymentTokenAddress: payment, tokenAddress: token, paymentTokenDecimals: 6, tokenDecimals: 18, onchainLocked: true }
const config = { VITE_ENVIRONMENT: 'local', VITE_IDO_V3_WALLET_ENABLED: 'true', VITE_IDO_V3_LOCAL_RPC: 'http://127.0.0.1:8545/',
  VITE_IDO_V3_CONFIRMATIONS: '2',
  VITE_IDO_V3_CHAIN_ID: '31337', VITE_IDO_V3_PROXY: proxy, VITE_IDO_V3_IMPLEMENTATION: buyer,
  VITE_IDO_V3_PROXY_CODE_HASH: h('1'), VITE_IDO_V3_IMPLEMENTATION_CODE_HASH: h('2') }

test('V3 requires exact pinned deployment, project type, decimals and locked identity', () => {
  const runtime = pinnedV3WalletRuntime(config)
  assert.equal(assertV3Project(project, runtime), project)
  for (const patch of [{ contractVersion: 'v2_stablecoin' }, { fundingMode: 'fair_launch' }, { chainId: 56 },
    { contractAddress: payment }, { paymentTokenDecimals: 18.1 }, { onchainLocked: false }]) {
    assert.throws(() => assertV3Project({ ...project, ...patch }, runtime), { code: 'IDO_V3_IDENTITY_UNAVAILABLE' })
  }
  assert.deepEqual(pinnedV3WalletRuntime({ ...config, VITE_IDO_V3_WALLET_ENABLED: 'false' }), runtime)
  assert.throws(() => pinnedV3WalletRuntime({ ...config, VITE_IDO_V3_PROXY: buyer }), { code: 'IDO_V3_ROUTING_UNAVAILABLE' })
  assert.throws(() => pinnedV3WalletRuntime({ ...config, VITE_IDO_V3_PROXY_CODE_HASH: h('z') }), { code: 'IDO_V3_ROUTING_UNAVAILABLE' })
  assert.throws(() => pinnedV3WalletRuntime({ ...config, VITE_IDO_V3_CONFIRMATIONS: '1' }), { code: 'IDO_V3_ROUTING_UNAVAILABLE' })
  for (const signature of ['purchase(uint256,uint256)', 'purchaseWithProof(uint256,uint256,uint64,bytes32[])', 'claim(uint256)',
    'refund(uint256)', 'depositProjectTokens(uint256,uint256)']) assert.ok(new Interface(IDO_V3_ABI).getFunction(signature))
})

test('one-wallet proof binds chain, proxy, project, version, root and wallet', () => {
  const root = keccak256(AbiCoder.defaultAbiCoder().encode(['bytes32', 'uint256', 'address', 'uint256', 'address'],
    [id('JOINCARE_IDO_V3_WHITELIST_LEAF_V1'), 31337n, proxy, BigInt(project.id), buyer]))
  const state = { enabled: true, root, version: '1' }
  const proof = { chain_id: '31337', project_id: project.id, proxy, wallet: buyer, root, version: '1', proof: [] }
  assert.deepEqual(assertV3Proof(proof, project, state, buyer), [])
  for (const patch of [{ version: '2' }, { wallet: payment }, { root: h('9') }, { chain_id: '56' }, { project_id: '1' }]) {
    assert.throws(() => assertV3Proof({ ...proof, ...patch }, project, state, buyer), { code: 'IDO_V3_PROOF_UNAVAILABLE' })
  }
  assert.throws(() => assertV3Proof(proof, project, { ...state, enabled: false }, buyer), { code: 'IDO_V3_PROOF_UNAVAILABLE' })
})

test('V3 proof acquisition is governed by live policy, not a build toggle', async () => {
  const { v3PurchaseAuthority } = await import('../src/services/idoV3Wallet.js')
  const localConfig = { ...config, VITE_IDO_V3_PROOF_ENABLED: 'false' }
  const readPublic = async () => ({ enabled: false, saleOpen: true, version: '0', root: h('0') })
  const publicAuthority = await v3PurchaseAuthority(project, buyer, {}, localConfig, async () => { throw Error('proof not needed') }, readPublic)
  assert.equal(publicAuthority.proof, null)
  const readEnabled = async () => ({ enabled: true, saleOpen: true, version: '1', root: h('1') })
  await assert.rejects(v3PurchaseAuthority(project, buyer, {}, localConfig, async () => { throw Error('proof unavailable') }, readEnabled), { code: 'IDO_V3_PROOF_UNAVAILABLE' })
  // A malformed proof cannot become an authority merely because the build
  // toggle was removed; state/root/wallet/version are still verified.
  await assert.rejects(v3PurchaseAuthority(project, buyer, {}, localConfig, async () => ({ wallet: payment }), readEnabled), { code: 'IDO_V3_PROOF_UNAVAILABLE' })
})

test('new blocks are allowed, changed policy and old-block reorg fail closed', async () => {
  const before = { blockNumber: 10, blockHash: h('a'), version: '1', root: h('b'), enabled: true, status: 1 }
  const after = { ...before, blockNumber: 11, blockHash: h('c') }
  const rpc = { getBlock: async (height) => ({ hash: height === 10 ? h('a') : h('c') }) }
  assert.equal(await assertV3PolicyUnchanged(before, after, rpc), after)
  await assert.rejects(assertV3PolicyUnchanged(before, { ...after, version: '2' }, rpc), { code: 'IDO_V3_POLICY_CHANGED' })
  await assert.rejects(assertV3PolicyUnchanged(before, after, { getBlock: async () => ({ hash: h('d') }) }), { code: 'IDO_V3_POLICY_CHANGED' })
})

test('owner deposit and terminal claim/refund are scoped to exact lifecycle', () => {
  const runtime = pinnedV3WalletRuntime(config)
  const pending = { status: 0, project: { projectOwner: buyer } }
  assert.equal(v3ProjectActionState(project, pending, 'depositProjectTokens', buyer, runtime), pending)
  assert.throws(() => v3ProjectActionState(project, pending, 'depositProjectTokens', payment, runtime), { code: 'IDO_V3_ACTION_UNAVAILABLE' })
  for (const [status, allowed] of [[0, []], [1, []], [2, ['claim']], [3, ['refund']], [4, ['refund']]]) {
    for (const action of ['claim', 'refund']) {
      if (allowed.includes(action)) assert.equal(v3ProjectActionState(project, { status }, action, buyer, runtime).status, status)
      else assert.throws(() => v3ProjectActionState(project, { status }, action, buyer, runtime), { code: 'IDO_V3_ACTION_UNAVAILABLE' })
    }
  }
})

test('V3 sale opens at start and closes at end, regardless of stale page status', () => {
  assert.equal(v3SaleOpen(1, 100n, 200n, 99n), false)
  assert.equal(v3SaleOpen(1, 100n, 200n, 100n), true)
  assert.equal(v3SaleOpen(1, 100n, 200n, 199n), true)
  assert.equal(v3SaleOpen(1, 100n, 200n, 200n), false)
  assert.equal(v3SaleOpen(0, 100n, 200n, 150n), false)
  assert.equal(v3SaleOpen(2, 100n, 200n, 150n), false)
  assert.equal(v3SaleOpen(1, 'bad', 200n, 150n), false)
})

test('V3 fixed-block RPC validates implementation, assets, policy and canonical block', async () => {
  const iface = new Interface(IDO_V3_ABI)
  const decimal = new Interface(['function decimals() view returns (uint8)'])
  const code = '0x60006000'
  const runtime = pinnedV3WalletRuntime({ ...config,
    VITE_IDO_V3_PROXY_CODE_HASH: keccak256(code), VITE_IDO_V3_IMPLEMENTATION_CODE_HASH: keccak256(code) })
  const header = { number: 12, hash: h('a'), timestamp: 150 }
  let slot = `0x${'0'.repeat(24)}${buyer.slice(2)}`
  let paymentDecimals = 6
  let reorg = false
  const calls = []
  const provider = {
    getNetwork: async () => ({ chainId: 31337n }),
    getBlock: async (tag) => tag === 'latest' ? header : { ...header, hash: reorg ? h('c') : header.hash },
    getBlockNumber: async () => 13,
    getStorage: async () => slot,
    getCode: async () => code,
    call: async (tx) => {
      calls.push(tx)
      if (tx.to.toLowerCase() === payment || tx.to.toLowerCase() === token) {
        return decimal.encodeFunctionResult('decimals', [tx.to.toLowerCase() === payment ? paymentDecimals : 18])
      }
      const selector = tx.data.slice(0, 10)
      if (selector === iface.getFunction('contractVersion').selector) return iface.encodeFunctionResult('contractVersion', [id('joincare.ido_v3_stablecoin.v1')])
      if (selector === iface.getFunction('getWhitelistPolicy').selector) return iface.encodeFunctionResult('getWhitelistPolicy', [[false, h('0'), 0]])
      if (selector === iface.getFunction('getProject').selector) return iface.encodeFunctionResult('getProject', [[
        BigInt(project.id), token, payment, buyer, buyer, 18, 6, 200, 1, 1000000n,
        100000000n, 10000000n, 10000000n, 100n, 200n, 0n, 0n, 0n, false, false, false, [], [],
      ]])
      throw Error('unexpected method')
    },
  }
  const contractProbe = new (await import('ethers')).Contract(proxy, IDO_V3_ABI, provider)
  await contractProbe.getProject(BigInt(project.id), { blockTag: 12 })
  const result = await readV3WalletState(project, runtime, provider)
  assert.equal(result.saleOpen, true)
  assert.equal(result.blockHash, header.hash)
  assert.equal(result.status, 1)
  assert.ok(calls.length >= 5 && calls.every((call) => call.blockTag === 12))
  paymentDecimals = 18
  await assert.rejects(readV3WalletState(project, runtime, provider), { code: 'IDO_V3_IDENTITY_UNAVAILABLE' })
  paymentDecimals = 6
  slot = `0x${'0'.repeat(24)}${token.slice(2)}`
  await assert.rejects(readV3WalletState(project, runtime, provider), { code: 'IDO_V3_IDENTITY_UNAVAILABLE' })
  slot = `0x${'0'.repeat(24)}${buyer.slice(2)}`
  reorg = true
  await assert.rejects(readV3WalletState(project, runtime, provider), { code: 'IDO_V3_STATE_UNAVAILABLE' })
})

test('local pending fence blocks repeat action, persists exact metadata and cannot be silently cleared by missing hash', () => {
  const values = new Map()
  const old = globalThis.window
  globalThis.window = { localStorage: { getItem: (key) => values.get(key) || null, setItem: (key, value) => values.set(key, value) } }
  const identity = v3SubmissionIdentity({ ...project, id: project.id }, buyer, 'purchase(uint256,uint256)')
  const data = '0x12345678'
  try {
    const meta = v3PendingMetadata(proxy, buyer, data)
    recordV3Pending(identity, '', meta)
    assert.throws(() => assertNoV3Pending(identity), { code: 'IDO_V3_SUBMISSION_UNRESOLVED' })
    assert.equal(listV3Pending({ ...project, id: project.id }, buyer).length, 1)
    assert.equal(meta.dataHash, keccak256(data))
    recordV3Pending(identity, h('e'))
    assert.equal(listV3Pending(project, buyer)[0][1].hash, h('e'))
    clearV3Pending(identity)
    assertNoV3Pending(identity)
  } finally { globalThis.window = old }
})

test('V3 submission sends once; uncertain receipt remains fenced until original transaction is reconciled', async () => {
  const originalWindow = globalThis.window
  const navigatorDescriptor = Object.getOwnPropertyDescriptor(globalThis, 'navigator')
  const values = new Map()
  globalThis.window = { localStorage: { getItem: (key) => values.get(key) || null, setItem: (key, value) => values.set(key, value) } }
  Object.defineProperty(globalThis, 'navigator', { configurable: true, value: { locks: { request: async (_, __, fn) => fn({}) } } })
  const service = new ContractService()
  const context = { ...project, id: '94713' }
  const state = { blockNumber: 12, blockHash: h('a'), status: 1, saleOpen: true, enabled: false, root: h('0'), version: '0' }
  const data = new Interface(['function approve(address,uint256)']).encodeFunctionData('approve', [proxy, 5n])
  const identity = v3SubmissionIdentity(context, buyer, 'approvePaymentToken')
  let sends = 0
  const rpc = { getBlock: async (number) => ({ hash: number === 12 ? h('a') : h('b') }), getNetwork: async () => ({ chainId: 31337n }),
    getBlockNumber: async () => 16, waitForTransaction: async () => null }
  service._v3Confirmations = () => 2
  service._v3Estimate = async () => 100000n
  service._v3Simulate = async () => 100000n
  service._v3FreshProject = async () => {}
  service._v3Signer = async () => ({ context, from: buyer })
  service.readV3PurchaseAuthority = async () => ({ state: { ...state, blockNumber: 13, blockHash: h('b') }, proof: null })
  service.readV3State = async () => ({ ...state, blockNumber: 13, blockHash: h('b') })
  service._v3ReadRunner = () => rpc
  service._v3FinalIdentity = async (_, baseline, checkPolicy) => {
    assert.equal(checkPolicy, true)
    await assertV3PolicyUnchanged(baseline, await service.readV3State(), rpc)
  }
  service._v3WalletSend = async () => { sends++; return h('f') }
  // Approval acknowledges the wallet hash immediately. These receipt/finality
  // assertions exercise the background confirmation, not the UI return value.
  const submitAndConfirm = async (...args) => {
    const result = await service._v3Submit(...args)
    assert.equal(result.status, 'submitted')
    return result.confirmation
  }
  try {
    service._v3Simulate = async () => { throw Object.assign(new Error('revert'), { code: 'IDO_V3_POLICY_CHANGED' }) }
    await assert.rejects(service._v3Submit(context, payment, data, buyer, 'approvePaymentToken', { state, proof: null }, state), { code: 'IDO_V3_POLICY_CHANGED' })
    assert.equal(sends, 0)
    assert.deepEqual(listV3Pending(context, buyer), [])
    service._v3Simulate = async () => 100000n
    globalThis.navigator.locks.request = async (_, __, fn) => fn(null)
    await assert.rejects(submitAndConfirm(context, payment, data, buyer, 'approvePaymentToken', { state, proof: null }, state), { code: 'IDO_V3_SUBMISSION_UNRESOLVED' })
    assert.equal(sends, 0)
    globalThis.navigator.locks.request = async (_, __, fn) => fn({})
    await assert.rejects(submitAndConfirm(context, payment, data, buyer, 'approvePaymentToken', { state, proof: null }, state), { code: 'IDO_V3_SUBMISSION_UNRESOLVED' })
    assert.equal(sends, 1)
    assert.equal(listV3Pending(context, buyer)[0][1].hash, h('f'))
    await assert.rejects(submitAndConfirm(context, payment, data, buyer, 'approvePaymentToken', { state, proof: null }, state), { code: 'IDO_V3_SUBMISSION_UNRESOLVED' })
    assert.equal(sends, 1)
    rpc.getTransactionReceipt = async () => ({ hash: h('f'), status: 1, blockNumber: 14, blockHash: h('c'), from: buyer, to: payment })
    rpc.getTransaction = async () => ({ hash: h('f'), chainId: 31337n, blockHash: h('c'), from: buyer, to: payment, data })
    rpc.getBlock = async (number) => ({ hash: number === 14 ? h('c') : number === 13 ? h('b') : h('a') })
    rpc.getBlockNumber = async () => 14
    assert.equal(await service.reconcileV3Pending.call({ _v3Context: () => context, _v3ReadRunner: () => rpc, _v3Confirmations: () => 2 }, context, buyer), false)
    rpc.getBlockNumber = async () => 16
    rpc.getBlock = async (number) => ({ hash: number === 14 ? h('d') : number === 13 ? h('b') : h('a') })
    assert.equal(await service.reconcileV3Pending.call({ _v3Context: () => context, _v3ReadRunner: () => rpc, _v3Confirmations: () => 2 }, context, buyer), false)
    rpc.getBlock = async (number) => ({ hash: number === 14 ? h('c') : number === 13 ? h('b') : h('a') })
    assert.equal(await service.reconcileV3Pending.call({ _v3Context: () => context, _v3ReadRunner: () => rpc, _v3Confirmations: () => 2 }, context, buyer), true)
    assert.deepEqual(listV3Pending(context, buyer), [])
    rpc.waitForTransaction = async () => ({ hash: h('f'), status: 1, blockNumber: 14, blockHash: h('c'), from: buyer, to: payment })
    assert.deepEqual(await submitAndConfirm(context, payment, data, buyer, 'approvePaymentToken', { state, proof: null }, state), { hash: h('f'), status: 'success', blockNumber: 14 })
    assert.equal(sends, 2)
    assert.deepEqual(listV3Pending(context, buyer), [])
    rpc.waitForTransaction = async () => ({ hash: h('f'), status: 0, blockNumber: 14, blockHash: h('c'), from: buyer, to: payment })
    await assert.rejects(submitAndConfirm(context, payment, data, buyer, 'approvePaymentToken', { state, proof: null }, state), { code: 'TRANSACTION_REVERTED' })
    assert.equal(sends, 3)
    assert.equal(listV3Pending(context, buyer).length, 1)
    rpc.getTransactionReceipt = async () => ({ hash: h('f'), status: 0, blockNumber: 14, blockHash: h('c'), from: buyer, to: payment })
    assert.equal(await service.reconcileV3Pending.call({ _v3Context: () => context, _v3ReadRunner: () => rpc, _v3Confirmations: () => 2 }, context, buyer), true)
    assert.deepEqual(listV3Pending(context, buyer), [])
    rpc.waitForTransaction = async () => ({ hash: h('f'), status: 1, blockNumber: 14, blockHash: h('c'), from: buyer, to: payment })
    rpc.getTransaction = async () => ({ hash: h('f'), chainId: 31337n, blockHash: h('c'), from: buyer, to: payment, data: '0xdeadbeef' })
    await assert.rejects(submitAndConfirm(context, payment, data, buyer, 'approvePaymentToken', { state, proof: null }, state), { code: 'IDO_V3_SUBMISSION_UNRESOLVED' })
    assert.equal(sends, 4)
    assert.equal(listV3Pending(context, buyer).length, 1)
    assert.equal(await service.reconcileV3Pending.call({ _v3Context: () => context, _v3ReadRunner: () => rpc, _v3Confirmations: () => 2 }, context, buyer), false)
  } finally {
    clearV3Pending(identity)
    globalThis.window = originalWindow
    if (navigatorDescriptor) Object.defineProperty(globalThis, 'navigator', navigatorDescriptor)
    else delete globalThis.navigator
  }
})

test('V3 wallet send never logs provider errors and rejects an invalid or uncertain hash', async () => {
  const service = new ContractService()
  const original = web3Service.sendRawTransaction
  const originalSigner = web3Service.signer
  const originalWarn = console.warn
  let warnings = 0
  console.warn = () => { warnings++ }
  let fallbackCalls = 0
  web3Service.signer = { sendTransaction: async () => { fallbackCalls++; return { hash: h('a') } } }
  try {
    web3Service.sendRawTransaction = async () => '0xbad'
    await assert.rejects(service._v3WalletSend(proxy, '0x12345678', buyer, 100000n, 31337), { code: 'IDO_V3_SUBMISSION_UNRESOLVED' })
    web3Service.sendRawTransaction = async () => { throw new Error('wallet error with sensitive transaction payload') }
    await assert.rejects(service._v3WalletSend(proxy, '0x12345678', buyer, 100000n, 31337), { code: 'IDO_V3_SUBMISSION_UNRESOLVED' })
    web3Service.sendRawTransaction = async () => { throw Object.assign(new Error('method not found'), { code: -32601 }) }
    await assert.rejects(service._v3WalletSend(proxy, '0x12345678', buyer, 100000n, 31337), { code: 'IDO_V3_WALLET_UNSUPPORTED' })
    assert.equal(fallbackCalls, 0)
    assert.equal(warnings, 0)
    web3Service.sendRawTransaction = async () => h('f')
    assert.equal(await service._v3WalletSend(proxy, '0x12345678', buyer, 100000n, 31337), h('f'))
  } finally {
    web3Service.sendRawTransaction = original
    web3Service.signer = originalSigner
    console.warn = originalWarn
  }
})

test('V3 contract simulation rejects stale policy before wallet submission without a fresh audit', async () => {
  const originalWindow = globalThis.window
  const navigatorDescriptor = Object.getOwnPropertyDescriptor(globalThis, 'navigator')
  const values = new Map()
  globalThis.window = { localStorage: { getItem: (key) => values.get(key) || null, setItem: (key, value) => values.set(key, value) } }
  Object.defineProperty(globalThis, 'navigator', { configurable: true, value: { locks: { request: async (_, __, fn) => fn({}) } } })
  const context = { ...project, id: '94714' }
  const base = { blockNumber: 12, blockHash: h('a'), status: 1, saleOpen: true, enabled: true, root: h('b'), version: '1' }
  const expected = { state: base, proof: [] }
  const service = new ContractService()
  let sends = 0
  service._v3Estimate = async () => 100000n
  service._v3Simulate = async () => { throw Object.assign(new Error('contract rejected policy'), { code: 'IDO_V3_POLICY_CHANGED' }) }
  service._v3FreshProject = async () => {}
  service._v3Signer = async () => ({ context, from: buyer })
  service._v3ReadRunner = () => ({ getBlock: async () => ({ hash: h('a') }) })
  service.getPaymentTokenBalance = async () => ({ raw: 1000000n })
  service.getPaymentTokenAllowance = async () => ({ raw: 1000000n })
  service.getUserPurchase = async () => ({ paymentAmountRaw: 0n })
  service._v3PurchaseSnapshotChecks = async () => {}
  service._v3WalletSend = async () => { sends++; return h('f') }
  const data = new Interface(IDO_V3_ABI).encodeFunctionData('purchaseWithProof', [BigInt(context.id), 5n, 1n, []])
  try {
    service._v3FinalIdentity = async (_, state) => assertV3PolicyUnchanged(state, { ...base, root: h('c') }, service._v3ReadRunner())
    await assert.rejects(service._v3Submit(context, proxy, data, buyer, 'purchaseWithProof(uint256,uint256,uint64,bytes32[])', expected), { code: 'IDO_V3_POLICY_CHANGED' })
    await assert.rejects(service._v3Submit(context, proxy, data, buyer, 'purchaseWithProof(uint256,uint256,uint64,bytes32[])', { state: base, proof: null }), { code: 'IDO_V3_POLICY_CHANGED' })
    assert.equal(sends, 0)
    assert.deepEqual(listV3Pending(context, buyer), [])
  } finally {
    globalThis.window = originalWindow
    if (navigatorDescriptor) Object.defineProperty(globalThis, 'navigator', navigatorDescriptor)
    else delete globalThis.navigator
  }
})
