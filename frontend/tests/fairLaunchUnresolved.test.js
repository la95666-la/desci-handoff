import assert from 'node:assert/strict'
import test from 'node:test'
import * as unresolved from '../src/services/fairLaunchUnresolved.js'

class MemoryStorage {
  constructor() { this.values = new Map() }
  getItem(k) { return this.values.get(k) ?? null }
  setItem(k, v) { this.values.set(k, v) }
  removeItem(k) { this.values.delete(k) }
}

const binding = { wallet: '0x1111111111111111111111111111111111111111', chainId: 56, target: '0x2222222222222222222222222222222222222222', projectId: 7, action: 'claim', calldata: '0xabcdef', stateBinding: null }

test('unresolved cache survives reload and has no client resolution or reviewed-reference bypass', () => {
  global.localStorage = new MemoryStorage()
  unresolved.recordFairLaunchUnresolved(binding, `0x${'a'.repeat(64)}`)
  assert.equal(unresolved.findFairLaunchUnresolved(binding).transactionHash, `0x${'a'.repeat(64)}`)
  assert.throws(() => unresolved.assertFairLaunchNotUnresolved(binding), /Backend canonical resolution is required/)
  assert.equal(unresolved.resolveFairLaunchUnresolved, undefined)
  assert.equal(unresolved.clearFairLaunchUnresolvedByReviewedRecovery, undefined)
})

test('malicious local clear is not treated as authority; Backend capability remains fail closed', () => {
  global.localStorage = new MemoryStorage()
  unresolved.recordFairLaunchUnresolved(binding, '')
  global.localStorage.removeItem('jc.fair-launch.wallet-submission-cache.v2')
  assert.equal(unresolved.findFairLaunchUnresolved(binding), null)
  // localStorage is only a cache. Actual signing is independently disabled by
  // Backend action reason durable_wallet_submission_unavailable.
})
