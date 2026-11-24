import { test } from 'node:test'
import assert from 'node:assert/strict'
import { parseUnits } from 'ethers'
import { readFileSync } from 'node:fs'
import { meetsMinPurchase } from '../src/utils/minPurchase.js'

test('official page minimum compares every purchase in exact payment-token units', () => {
  const project = { minPurchaseAmountRaw: '10000001' } // 10.000001 (6 decimals)
  assert.equal(meetsMinPurchase(parseUnits('10', 6), project), false)
  assert.equal(meetsMinPurchase(parseUnits('10.000001', 6), project), true)
  assert.equal(meetsMinPurchase(parseUnits('10.000002', 6), project), true)
  assert.equal(meetsMinPurchase(parseUnits('1', 6), { minPurchaseAmountRaw: '' }), true)
  assert.equal(meetsMinPurchase(parseUnits('1', 6), {}), true)
  assert.equal(meetsMinPurchase(parseUnits('1', 6), { minPurchaseAmountRaw: 'not-a-number' }), false)
})

test('V3 minimum applies per transaction at 6/18 decimals and does not restore blocking detail preflights', () => {
  for (const decimals of [6, 18]) {
    const p = { minPurchaseAmountRaw: parseUnits('10.5', decimals).toString() }
    assert.equal(meetsMinPurchase(parseUnits('10.499999', decimals), p), false)
    assert.equal(meetsMinPurchase(parseUnits('10.5', decimals), p), true)
    assert.equal(meetsMinPurchase(parseUnits('11', decimals), p), true)
    assert.equal(meetsMinPurchase(parseUnits('1', decimals), {}), true)
  }
  const s = readFileSync(new URL('../src/components/LaunchpadActiveFundingPanel.vue', import.meta.url), 'utf8')
  assert.match(s, /if \(isV3.value\) return v3CanPurchase.value && raw > 0n && meetsMinPurchase/)
  assert.match(s, /v-else-if="belowMinPurchase"/)
  const handler = s.slice(s.indexOf('async function handlePurchase'), s.indexOf('async function handleRefund'))
  assert.match(handler, /if \(isV3.value && !meetsMinPurchase/)
  assert.match(handler, /if \(!isV3.value\) \{\s+const latest = await getProjectDetail/)
  assert.doesNotMatch(handler, /readV3State|readV3PurchaseAuthority/)
})
