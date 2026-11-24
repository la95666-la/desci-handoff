import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { formatRaisedAmount } from '../src/utils/raisedAmount.js'

test('USDT raised amounts display exactly three decimals with decimal rounding', () => {
  for (const [input, expected] of [
    ['20173.90245173614558782', '20,173.902'], ['20173.9025', '20,173.903'],
    ['999.9995', '1,000.000'], ['0', '0.000'], ['1.2', '1.200'],
    ['900719925474099312345.1235', '900,719,925,474,099,312,345.124'],
  ]) assert.equal(formatRaisedAmount(input, 'USDT'), expected)
})
test('unknown amounts are not zero and other assets retain their precision', () => {
  for (const value of [null, undefined, '', 'NaN', '-1', '1e18']) assert.equal(formatRaisedAmount(value, 'USDT'), '—')
  assert.equal(formatRaisedAmount('1.123456', 'USDC'), '1.123456')
})
test('detail and funding panel use the shared raised formatter', () => {
  for (const file of ['components/LaunchpadActiveFundingPanel.vue', 'views/LaunchpadDetail.vue', 'views/LaunchpadDetailPreLaunch.vue', 'views/Dashboard.vue']) {
    const source = readFileSync(new URL(`../src/${file}`, import.meta.url), 'utf8')
    assert.match(source, /formatRaisedAmount/)
    assert.doesNotMatch(source, /raised\.toLocaleString\(\)/)
  }
})
