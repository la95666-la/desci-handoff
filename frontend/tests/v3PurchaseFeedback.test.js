import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { purchaseFeedbackKey, isLocalPurchasePending } from '../src/utils/v3PurchaseFeedback.js'

const p = { id: '43368660559134718', contractAddress: '0x'+'1'.repeat(40), chainId: 56 }
const key = purchaseFeedbackKey(p, '0x'+'2'.repeat(40))
test('only currently awaited local purchase is normal pending, not historical unknown work', () => {
  for (const action of ['purchase(uint256,uint256)', 'purchaseWithProof(uint256,uint256,uint64,bytes32[])']) {
    assert.equal(isLocalPurchasePending({ action, hash: '' }, key, key), true)
    assert.equal(isLocalPurchasePending({ action, hash: 'submitted' }, key, key), true)
    assert.equal(isLocalPurchasePending({ action }, '', key), false)
    assert.equal(isLocalPurchasePending({ action }, key, purchaseFeedbackKey(p, 'another-wallet')), false)
    assert.equal(isLocalPurchasePending({ action }, key, purchaseFeedbackKey({ ...p, id: 'other' }, '0x'+'2'.repeat(40))), false)
  }
  assert.equal(isLocalPurchasePending({ action: 'refund(uint256)' }, key, key), false)
})

test('busy purchase branch wins before unavailable/approval routes and ignores display reconciliation', () => {
  const s = readFileSync(new URL('../src/components/LaunchpadActiveFundingPanel.vue', import.meta.url), 'utf8')
  const button = s.indexOf('data-testid="v3-purchase-waiting"')
  assert.ok(button > 0 && button < s.indexOf('v-else-if="isV3 && (!wallet.isLoggedIn'))
  assert.match(s, /v-if="v3LocalPurchaseBusy"[^>]*disabled aria-busy="true"/)
  assert.match(s, /funding-purchase-spinner/)
  assert.match(s, /v-if="isLive \|\| v3LocalPurchaseBusy"/)
  const refresh = s.slice(s.indexOf('async function loadV3Display'), s.indexOf('async function refreshRaised'))
  assert.match(refresh, /if \(pendingNeedsRecovery.value\)/)
  assert.doesNotMatch(refresh, /if \(v3Pending.value.length && !v3ApprovalSubmitted.value\)/)
  assert.match(s, /activePurchaseFeedbackKey.value = ''/)
})
