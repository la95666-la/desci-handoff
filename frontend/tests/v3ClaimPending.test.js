import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { Interface } from 'ethers'
import { ContractService } from '../src/services/contract.js'
import { recordV3Pending, listV3Pending, v3PendingSnapshot, v3PendingMetadata, v3SubmissionIdentity } from '../src/services/idoV3Pending.js'
import { v3ClaimPending } from '../src/utils/v3ClaimPending.js'

const wallet = '0xd204d17b80cc2eca3000f17b96e640b457dc5798'
const project = { id: '43879281503240193', chainId: 56,
  contractAddress: '0x0d4040875742975269c55e16c8884d3ee9510b8e',
  paymentTokenAddress: '0x55d398326f99059ff775485246999027b3197955' }
const hash = '0x' + 'a'.repeat(64)
const claim = new Interface(['function claim(uint256)']).encodeFunctionData('claim', [project.id])

test('claim simulates once and sends despite unrelated hashless/hash-bearing records; originals stay intact', async () => {
  const savedWindow = globalThis.window, savedNavigator = Object.getOwnPropertyDescriptor(globalThis, 'navigator')
  const values = new Map()
  globalThis.window = { localStorage: { getItem: k => values.get(k) || null, setItem: (k, v) => values.set(k, v) } }
  let lockHeld = true
  Object.defineProperty(globalThis, 'navigator', { configurable: true,
    value: { locks: { request: async (_, __, run) => run(lockHeld ? {} : null) } } })
  const service = new ContractService()
  let simulations = 0, sends = 0, simulationError = false, walletChanged = false
  service._v3Simulate = async (_, to, data, from) => {
    simulations++; assert.equal(to, project.contractAddress); assert.equal(data, claim); assert.equal(from, wallet)
    if (simulationError) throw Object.assign(new Error('NoTokensToClaim'), { code: 'IDO_V3_ACTION_UNAVAILABLE' })
    return 125000n
  }
  service._v3Signer = async () => ({ context: project, from: walletChanged ? project.paymentTokenAddress : wallet })
  service._v3ReadRunner = () => { throw Error('Claim must not query unrelated receipts') }
  service._v3WalletSend = async () => { sends++; return hash }
  service._v3WaitSubmission = async () => ({ hash, status: 'success' })
  const run = () => service._v3Submit(project, project.contractAddress, claim, wallet, 'claim(uint256)')
  try {
    const actions = ['approvePaymentToken', 'purchase(uint256,uint256)',
      'purchaseWithProof(uint256,uint256,uint64,bytes32[])', 'refund(uint256)',
      'approveProjectToken', 'depositProjectTokens(uint256,uint256)', 'withdrawRaisedFunds(uint256)']
    for (const [index, action] of actions.entries()) {
      recordV3Pending(v3SubmissionIdentity(project, wallet, action), index % 2 ? hash : '',
        { ...v3PendingMetadata(project.contractAddress, wallet, claim), requestId: action })
    }
    const before = listV3Pending(project, wallet)
    assert.deepEqual(v3ClaimPending(v3PendingSnapshot(project, wallet)), [])
    simulationError = true
    await assert.rejects(run(), { code: 'IDO_V3_ACTION_UNAVAILABLE' })
    assert.equal(sends, 0); assert.deepEqual(listV3Pending(project, wallet), before)
    simulationError = false; walletChanged = true
    await assert.rejects(run(), { code: 'IDO_V3_WALLET_CHANGED' })
    assert.equal(sends, 0); assert.deepEqual(listV3Pending(project, wallet), before)
    walletChanged = false; lockHeld = false
    await assert.rejects(run(), { code: 'IDO_V3_SUBMISSION_UNRESOLVED' })
    assert.equal(sends, 0)
    lockHeld = true
    const previousSimulations = simulations
    assert.equal((await run()).status, 'success')
    assert.equal(simulations - previousSimulations, 1); assert.equal(sends, 1)
    for (const [identity, entry] of before) assert.deepEqual(listV3Pending(project, wallet).find(([key]) => key === identity)?.[1], entry)
    // Even when simulation would still pass (before mining), never resend an
    // original claim. Both hash-bearing and hashless claim records are fenced.
    const snapshot = values.get('jc-ido-v3-wallet-pending-v1')
    for (const pendingHash of [hash, '']) {
      const entries = JSON.parse(snapshot)
      entries[v3SubmissionIdentity(project, wallet, 'claim(uint256)')].hash = pendingHash
      values.set('jc-ido-v3-wallet-pending-v1', JSON.stringify(entries))
      const count = simulations
      await assert.rejects(run(), { code: 'IDO_V3_SUBMISSION_UNRESOLVED' })
      assert.equal(simulations, count); assert.equal(sends, 1)
    }
    for (const action of ['refund(uint256)', 'withdrawRaisedFunds(uint256)', 'approveProjectToken']) {
      assert.equal(await service._v3PendingAllowsPurchase(project, wallet, action), false)
    }
  } finally {
    globalThis.window = savedWindow
    if (savedNavigator) Object.defineProperty(globalThis, 'navigator', savedNavigator)
    else delete globalThis.navigator
  }
})

test('detail and Dashboard scope claim pending gates without weakening other operations', () => {
  assert.equal(v3ClaimPending([{ action: 'claim(uint256)', hash }, { action: 'approvePaymentToken' }]).length, 1)
  const panel = readFileSync(new URL('../src/components/LaunchpadActiveFundingPanel.vue', import.meta.url), 'utf8')
  const canClaim = panel.slice(panel.indexOf('const canClaimV3'), panel.indexOf('function friendlyError'))
  assert.match(canClaim, /!claimPending.value.length/)
  assert.doesNotMatch(canClaim, /!v3Unresolved/)
  assert.match(panel, /action === 'claim' \? claimPending.value : v3Pending.value/)
  assert.match(panel, /v3ClaimStage.value \? claimPending.value : v3Pending.value/)
  assert.match(panel, /recoveryPending.map/)
  const dashboard = readFileSync(new URL('../src/views/Dashboard.vue', import.meta.url), 'utf8')
  assert.match(dashboard, /reconcilePortfolioSubmission\(project, address, 'claim\(uint256\)'\)/)
  const recovery = dashboard.slice(dashboard.indexOf('async function reconcilePortfolioSubmission'), dashboard.indexOf('// 项目状态'))
  assert.match(recovery, /if \(!relevant\(contractService.v3Pending/)
  assert.match(recovery, /pending.length/)
  assert.match(dashboard, /await reconcilePortfolioSubmission\(project, address\)/) // refund retains full recovery
})

test('claim recovery reads only its exact original hash and retains unrelated unknown records', async () => {
  const savedWindow = globalThis.window, values = new Map()
  globalThis.window = { localStorage: { getItem: k => values.get(k) || null, setItem: (k, v) => values.set(k, v) } }
  const service = new ContractService(), blockHash = '0x' + 'b'.repeat(64)
  service._v3Context = () => project
  service._v3Confirmations = () => 2
  const queried = []
  service._v3ReadRunner = () => ({ getNetwork: async () => ({ chainId: 56n }),
    getTransactionReceipt: async txHash => {
      queried.push(txHash); assert.equal(txHash, hash)
      return { hash, status: 1, blockNumber: 10, blockHash, to: project.contractAddress, from: wallet }
    },
    getTransaction: async () => ({ hash, data: claim, value: 0n, chainId: 56n, blockHash, to: project.contractAddress, from: wallet }),
    getBlock: async () => ({ hash: blockHash }), getBlockNumber: async () => 12 })
  try {
    const old = v3SubmissionIdentity(project, wallet, 'purchase(uint256,uint256)')
    recordV3Pending(old, '', { ...v3PendingMetadata(project.contractAddress, wallet, '0x12345678'), requestId: 'old' })
    const original = listV3Pending(project, wallet)
    assert.equal(await service.reconcileV3Pending(project, wallet, 'claim(uint256)'), true)
    assert.deepEqual(queried, [])
    recordV3Pending(v3SubmissionIdentity(project, wallet, 'claim(uint256)'), hash,
      { ...v3PendingMetadata(project.contractAddress, wallet, claim), requestId: 'claim' })
    assert.equal(await service.reconcileV3Pending(project, wallet, 'claim(uint256)'), true)
    assert.deepEqual(queried, [hash]); assert.deepEqual(listV3Pending(project, wallet), original)
    assert.equal(await service.reconcileV3Pending(project, wallet), false)
  } finally { globalThis.window = savedWindow }
})
