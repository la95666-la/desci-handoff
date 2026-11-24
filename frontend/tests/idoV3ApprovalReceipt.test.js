import test from 'node:test'
import assert from 'node:assert/strict'
import { Interface, keccak256 } from 'ethers'
import { isV3WalletEditedApproval } from '../src/services/idoV3ApprovalReceipt.js'
import { ContractService } from '../src/services/contract.js'
import { recordV3Pending, v3PendingMetadata, v3SubmissionIdentity, listV3Pending } from '../src/services/idoV3Pending.js'

const owner = '0x6aaa882cce45ebf317a6342f317c6e92cba04f23'
const token = '0xf77c5b5faabb9945ac8701ad08ef09e12dfc768a'
const proxy = '0x0d4040875742975269c55e16c8884d3ee9510b8e'
const context = { id: '43359438123302913', chainId: 56, tokenAddress: token,
  paymentTokenAddress: '0x55d398326f99059ff775485246999027b3197955', contractAddress: proxy }
const hash = '0x5321e9349ed162655bec3a3844daf4a86014f8e8d7bca0adcf2ca9370174a8aa'
const blockHash = '0x' + 'a'.repeat(64)
const abi = new Interface(['function approve(address spender,uint256 amount)', 'event Approval(address indexed owner,address indexed spender,uint256 value)'])
const original = abi.encodeFunctionData('approve', [proxy, 800000n * 10n ** 18n])
const edited = abi.encodeFunctionData('approve', [proxy, 92001742053568917073788357n])
const event = abi.encodeEventLog('Approval', [owner, proxy, 92001742053568917073788357n])
const entry = v3PendingMetadata(token, owner, original)
const transaction = { hash, to: token, from: owner, chainId: 56n, data: edited, value: 0n, blockHash }
const receipt = { hash, to: token, from: owner, status: 1, blockHash, blockNumber: 130039951,
  logs: [{ address: token, ...event }] }

test('wallet-edited approval amount requires the original actor/token/spender and exact Approval event', () => {
  assert.equal(isV3WalletEditedApproval(context, 'approveProjectToken', entry, receipt, transaction), true)
  for (const [action, r, tx] of [
    ['depositProjectTokens(uint256,uint256)', receipt, transaction],
    ['approvePaymentToken', receipt, transaction],
    ['approveProjectToken', { ...receipt, status: 0 }, transaction],
    ['approveProjectToken', { ...receipt, logs: [] }, transaction],
    ['approveProjectToken', { ...receipt, logs: [{ address: proxy, ...event }] }, transaction],
    ['approveProjectToken', receipt, { ...transaction, from: proxy }],
    ['approveProjectToken', receipt, { ...transaction, to: proxy }],
    ['approveProjectToken', receipt, { ...transaction, value: 1n }],
    ['approveProjectToken', receipt, { ...transaction, data: abi.encodeFunctionData('approve', [owner, 5n]) }],
    ['approveProjectToken', receipt, { ...transaction, data: edited + '00' }],
    ['approveProjectToken', receipt, { ...transaction, data: original }],
  ]) assert.equal(isV3WalletEditedApproval(context, action, entry, r, tx), false)
})

test('confirmed original edited approve reconciles automatically; wrong hash, chain, reorg or missing confirmation stays fenced', async () => {
  const oldWindow = globalThis.window
  const values = new Map()
  globalThis.window = { localStorage: { getItem: (k) => values.get(k) || null, setItem: (k, v) => values.set(k, v) } }
  const service = new ContractService()
  service._v3Context = () => context
  service._v3Confirmations = () => 15
  let currentReceipt = receipt, currentTx = transaction, canonicalHash = blockHash, tip = receipt.blockNumber + 100
  service._v3ReadRunner = () => ({ getNetwork: async () => ({ chainId: 56n }),
    getTransactionReceipt: async () => currentReceipt, getTransaction: async () => currentTx,
    getBlock: async () => ({ hash: canonicalHash }), getBlockNumber: async () => tip })
  const identity = v3SubmissionIdentity(context, owner, 'approveProjectToken')
  const seed = () => { values.clear(); recordV3Pending(identity, '', entry); recordV3Pending(identity, hash) }
  try {
    assert.notEqual(keccak256(edited), entry.dataHash)
    for (const variant of ['wrongHash', 'wrongChain', 'reorg', 'unconfirmed', 'missingLog', 'missingReceipt']) {
      seed(); currentReceipt = receipt; currentTx = transaction; canonicalHash = blockHash; tip = receipt.blockNumber + 100
      if (variant === 'wrongHash') currentTx = { ...transaction, hash: '0x' + 'b'.repeat(64) }
      if (variant === 'wrongChain') currentTx = { ...transaction, chainId: 97n }
      if (variant === 'reorg') canonicalHash = '0x' + 'b'.repeat(64)
      if (variant === 'unconfirmed') tip = receipt.blockNumber + 2
      if (variant === 'missingLog') currentReceipt = { ...receipt, logs: [] }
      if (variant === 'missingReceipt') currentReceipt = null
      assert.equal(await service.reconcileV3Pending(context, owner), false)
      assert.equal(listV3Pending(context, owner).length, 1)
    }
    seed(); currentReceipt = receipt; currentTx = transaction; canonicalHash = blockHash; tip = receipt.blockNumber + 100
    assert.equal(await service.reconcileV3Pending(context, owner), true)
    assert.equal(listV3Pending(context, owner).length, 0)
  } finally { globalThis.window = oldWindow }
})
