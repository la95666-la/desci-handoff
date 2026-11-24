import assert from 'node:assert/strict'
import test from 'node:test'

import {
  scaleDecimals,
  tokensRequiredForCap,
  computeSoftAndHardRequirement,
  allowanceSufficient,
  balanceSufficient,
  formatToken,
  toInputAmount,
} from '../src/utils/depositMath.js'

const ONE = 10n ** 18n
const PRICE = 2n * 10n ** 17n

test('scaleDecimals builds 10^n safely', () => {
  assert.equal(scaleDecimals(6), 10n ** 6n)
  assert.equal(scaleDecimals(18), 10n ** 18n)
  assert.equal(scaleDecimals(-3), 1n)
  assert.equal(scaleDecimals(99), 10n ** 27n)
})

test('tokensRequiredForCap converts payment amount to project tokens (BigInt, floor)', () => {
  assert.equal(tokensRequiredForCap(ONE, PRICE, 18), 5n * ONE)
  assert.equal(tokensRequiredForCap(0n, PRICE, 18), 0n)
  assert.equal(tokensRequiredForCap(ONE, 0n, 18), 0n)
  assert.equal(tokensRequiredForCap(ONE, 3n * 10n ** 17n, 18), 3n * ONE + ONE / 3n)
})

test('tokensRequiredForCap tolerates malformed raw inputs', () => {
  assert.equal(tokensRequiredForCap('1e18', '2e17', 18), 0n)
  assert.equal(tokensRequiredForCap(undefined, undefined, 18), 0n)
  assert.equal(tokensRequiredForCap(null, PRICE, 18), 0n)
})

test('computeSoftAndHardRequirement derives soft/hard/shortfall', () => {
  const r = computeSoftAndHardRequirement({
    softCapRaw: String(ONE),
    hardCapRaw: String(3n * ONE),
    tokenPriceRaw: String(PRICE),
    depositedRaw: String(5n * ONE),
    tokenDecimals: 18,
  })
  assert.equal(r.softCapRequiredRaw, 5n * ONE)
  assert.equal(r.hardCapRequiredRaw, 15n * ONE)
  assert.equal(r.shortfallRaw, 10n * ONE)
})

test('shortfall is clamped at zero when already funded', () => {
  const r = computeSoftAndHardRequirement({
    softCapRaw: ONE,
    hardCapRaw: 3n * ONE,
    tokenPriceRaw: PRICE,
    depositedRaw: 20n * ONE,
    tokenDecimals: 18,
  })
  assert.equal(r.shortfallRaw, 0n)
})

test('allowanceSufficient / balanceSufficient compare BigInt raws', () => {
  assert.equal(allowanceSufficient(5n * ONE, 5n * ONE), true)
  assert.equal(allowanceSufficient(5n * ONE - 1n, 5n * ONE), false)
  assert.equal(allowanceSufficient('0', '0'), true)
  assert.equal(balanceSufficient(10n ** 15n, 10n ** 18n), false)
})

test('formatToken groups thousands and trims decimals', () => {
  assert.equal(formatToken(String(1234n * ONE), 18), '1,234')
  assert.equal(formatToken(String(1234n * ONE + 123n * 10n ** 15n), 18, { maxDecimals: 4 }), '1,234.123')
  assert.equal(formatToken('0', 18), '0')
  assert.equal(formatToken('1' + '0'.repeat(6), 6, { maxDecimals: 2 }), '1')
})

test('toInputAmount round-trips through parseUnits-compatible string', () => {
  assert.equal(toInputAmount(1230000000000000000n, 18), '1.23')
  assert.equal(toInputAmount(5n * ONE, 18), '5')
})