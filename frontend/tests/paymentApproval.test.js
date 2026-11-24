import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { paymentApprovalState, reconcilePaymentAllowance } from '../src/utils/paymentApproval.js'
import { assertV3PurchaseAmounts } from '../src/utils/v3PurchaseChecks.js'
import { v3UserMessage } from '../src/utils/v3UserMessages.js'
import { assertV3Proof } from '../src/services/idoV3Wallet.js'
import { ContractService } from '../src/services/contract.js'

test('V2 and V3 payment steps use exact allowance including dust and changed input', () => {
  for (const decimals of [6, 18]) {
    const one = 10n ** BigInt(decimals)
    assert.equal(paymentApprovalState(one, 0n).needsApproval, true)
    assert.equal(paymentApprovalState(one, one - 1n).needsApproval, true)
    assert.equal(paymentApprovalState(one, one).approved, true)
    assert.equal(paymentApprovalState(2n * one, one).needsApproval, true)
  }
  assert.equal(paymentApprovalState(1n, null).needsApproval, true)
  assert.equal(paymentApprovalState(0n, 0n).valid, false)
  assert.equal(paymentApprovalState('invalid', 1n).approved, false)
})

test('confirmed allowance bridges only pre-receipt blocks and cannot mask later zero allowance', () => {
  const confirmation = { raw: 1000000n, blockNumber: 20 }
  assert.deepEqual(reconcilePaymentAllowance(0n, 19, confirmation), { raw: 1000000n, confirmation })
  assert.deepEqual(reconcilePaymentAllowance(0n, 20, confirmation), { raw: 0n, confirmation: null })
  assert.deepEqual(reconcilePaymentAllowance(0n, 21, confirmation), { raw: 0n, confirmation: null })
  assert.deepEqual(reconcilePaymentAllowance(5n, 21, null), { raw: 5n, confirmation: null })
  assert.deepEqual(reconcilePaymentAllowance(0n, 21, { raw: 1n }), { raw: 0n, confirmation: null })
})

test('purchase preflight separates allowance, balance and cap failures with actionable messages', () => {
  const base = [1n, 1n, 5n, 0n, 6n, 0n, 6n]
  assert.doesNotThrow(() => assertV3PurchaseAmounts(...base))
  for (const [index, value, code] of [[1, 0n, 'IDO_V3_ALLOWANCE_INSUFFICIENT'],
    [2, 0n, 'IDO_V3_BALANCE_INSUFFICIENT'], [4, 0n, 'IDO_V3_USER_LIMIT_EXCEEDED'], [6, 0n, 'IDO_V3_HARD_CAP_EXCEEDED']]) {
    const args = [...base]; args[index] = value
    assert.throws(() => assertV3PurchaseAmounts(...args), { code })
    assert.notEqual(v3UserMessage(code), v3UserMessage('IDO_V3_STATE_UNAVAILABLE'))
    assert.notEqual(v3UserMessage(code, true), v3UserMessage('IDO_V3_STATE_UNAVAILABLE', true))
  }
})

test('reported single-wallet empty proof remains valid; approval is a separate requirement', () => {
  const p = { id: '43264392090877953', chainId: 56, contractAddress: '0x0d4040875742975269c55e16c8884d3ee9510b8e' }
  const wallet = '0xc7b4bf323c94ceb2e6e9bb7fbf89d06140941e5f'
  const root = '0x35204797193da0ae9a7365611e8630c5db9222db1eea86468daaacc9c7e19558'
  assert.deepEqual(assertV3Proof({ chain_id: '56', project_id: p.id, proxy: p.contractAddress,
    wallet, version: '1', root, proof: [] }, p, { enabled: true, version: '1', root }, wallet), [])
  assert.throws(() => assertV3PurchaseAmounts(10n ** 18n, 0n, 499n * 10n ** 16n, 0n, 6n * 10n ** 18n, 0n, 6n * 10n ** 18n), { code: 'IDO_V3_ALLOWANCE_INSUFFICIENT' })
})

test('panel retains input, returns to approval on fresh insufficient allowance, and serializes actions', () => {
  const source = readFileSync(new URL('../src/components/LaunchpadActiveFundingPanel.vue', import.meta.url), 'utf8')
  assert.match(source, /return isV3\.value && needsApproval\.value/)
  assert.match(source, /reconcilePaymentAllowance\(displayAllowance, result\.state\.blockNumber, v3AllowanceConfirmation\)/)
  assert.doesNotMatch(source, /paymentAllowance\.value = confirmedAllowanceFloor !== null && displayAllowance/)
  assert.match(source, /v3AllowanceConfirmation = \{ raw: approvedRaw, blockNumber: result\.blockNumber \}/)
  const approval = source.slice(source.indexOf('async function handleApprove'), source.indexOf('async function handlePurchase'))
  assert.doesNotMatch(approval, /purchaseAmount\.value = ''/)
  const failure = source.slice(source.indexOf('async function handlePurchase'), source.indexOf('async function handleRefund')).split('} catch (error) {')[1]
  assert.match(failure, /IDO_V3_ALLOWANCE_INSUFFICIENT[\s\S]*?v3AllowanceConfirmation = null[\s\S]*?paymentAllowance\.value = 0n[\s\S]*?refreshV3\(true\)/)
  assert.doesNotMatch(failure, /purchaseAmount\.value = ''/)
  assert.match(source, /funding-payment-steps/)
  assert.match(source, /isApproving\.value \|\| isPurchasing\.value \|\| v3Busy\.value/)
  const contract = readFileSync(new URL('../src/services/contract.js', import.meta.url), 'utf8')
  assert.doesNotMatch(contract, /assertV3PurchaseAmounts\(/)
})

test('V3 purchase uses proof calldata, leaving allowance enforcement to the single simulation', async () => {
  const s = new ContractService()
  const wallet = '0xc7b4bf323c94ceb2e6e9bb7fbf89d06140941e5f'
  const context = { id: '43264392090877953', paymentTokenDecimals: 18 }
  const one = 10n ** 18n
  let allowance = 0n, sends = 0
  s._v3Signer = async () => ({ context, from: wallet })
  s._v3PurchaseArguments = async () => ({ signature: 'purchaseWithProof(uint256,uint256,uint64,bytes32[])', args: [43264392090877953n, one, 1n, []] })
  s.getPaymentTokenAllowance = async () => ({ raw: allowance })
  s.getPaymentTokenBalance = async () => ({ raw: 499n * 10n ** 16n })
  s.getUserPurchase = async () => ({ paymentAmountRaw: 0n })
  s._v3Send = async (p, signature, args, from) => {
    // Mock the contract's revert at the send/preflight seam, not a separate
    // front-end balance/allowance/caps query.
    if (allowance < one) throw Object.assign(new Error('revert'), { code: 'IDO_V3_ALLOWANCE_INSUFFICIENT' })
    sends++
    assert.equal(signature, 'purchaseWithProof(uint256,uint256,uint64,bytes32[])')
    assert.deepEqual(args, [43264392090877953n, one, 1n, []])
    assert.equal(from, wallet)
    return { hash: 'mock-only' }
  }
  await assert.rejects(s.purchaseV3(context, '1', wallet), { code: 'IDO_V3_ALLOWANCE_INSUFFICIENT' })
  assert.equal(sends, 0)
  allowance = one
  await s.purchaseV3(context, '1', wallet)
  assert.equal(sends, 1)
  // A later revocation must force approval again even after a prior success.
  allowance = 0n
  await assert.rejects(s.purchaseV3(context, '1', wallet), { code: 'IDO_V3_ALLOWANCE_INSUFFICIENT' })
  assert.equal(sends, 1)
})

test('wallet approval hash immediately shows acknowledgement, refreshes real allowance without optimistic credit', () => {
  const s = readFileSync(new URL('../src/components/LaunchpadActiveFundingPanel.vue', import.meta.url), 'utf8')
  const handler = s.slice(s.indexOf('async function handleApprove'), s.indexOf('async function handlePurchase'))
  const submitted = handler.slice(handler.indexOf("if (isV3.value && result.status === 'submitted')"), handler.indexOf('// The successful receipt'))
  assert.match(submitted, /操作已確認/)
  assert.match(submitted, /showModal.value = true/)
  assert.match(submitted, /refreshV3\(true\)/)
  assert.match(submitted, /result.confirmation.then/)
  assert.doesNotMatch(submitted, /paymentAllowance.value = approvedRaw|confirmedAllowanceFloor = approvedRaw/)
  assert.match(s, /v3ApprovalSubmitted.value/)
  assert.match(s, /pendingNeedsRecovery.value/)
})
