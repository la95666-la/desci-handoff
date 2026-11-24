import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { Interface } from 'ethers'
import { FAIR_LAUNCH_V1_ABI } from '../src/config/fairLaunchAbi.js'
import {
  calculateProportionalAllocation,
  fairLaunchActionAvailable,
  trustedFairLaunchRuntime,
} from '../src/utils/fairLaunch.js'

const address = '0x1111111111111111111111111111111111111111'
const hash = (ch) => `0x${ch.repeat(64)}`
const base = {
  id: 8, fundingMode: 'fair_launch', contractVersion: 'fair_launch_v1', chainId: 56,
  contractAddress: address,
  runtimeIdentity: {
    chainId: 56, proxyAddress: address, implementationAddress: '0x2222222222222222222222222222222222222222',
    implementationCodeHash: hash('a'), sourceRegistryRevision: 2, sourceRegistryDigest: hash('b'),
    observedBlockNumber: '12345', observedBlockHash: hash('c'), observedAt: '2026-09-07T10:00:00Z', runtimeVerified: true,
  },
  stateSource: 'canonical_chain_event', finality: 'finalized', reconciliationState: 'BALANCED',
  actions: { contribute: { available: true }, claim: { available: true }, refund: { available: true } },
}

test('audited ABI exposes exact Fair Launch functions without a deployment address', () => {
  const iface = new Interface(FAIR_LAUNCH_V1_ABI)
  for (const signature of ['contribute(uint256,uint256)', 'claimFor(uint256,address)', 'refundFor(uint256,address)', 'withdrawProjectProceeds(uint256)']) {
    assert.ok(iface.getFunction(signature))
  }
  const source = readFileSync(new URL('../src/config/fairLaunchAbi.js', import.meta.url), 'utf8')
  assert.doesNotMatch(source, /0x[0-9a-fA-F]{40}/)
})

test('runtime requires exact backend trust, verified runtime, canonical source and valid identity', () => {
  assert.equal(trustedFairLaunchRuntime(base).enabled, true)
  for (const patch of [
    { runtimeIdentity: null }, { runtimeIdentity: { ...base.runtimeIdentity, runtimeVerified: false } }, { stateSource: 'cache' },
    { contractAddress: '0x3333333333333333333333333333333333333333' }, { chainId: 97 }, { contractVersion: 'v2_stablecoin' },
  ]) assert.equal(trustedFairLaunchRuntime({ ...base, ...patch }).enabled, false)
})

test('authoritative backend action, finality and reconciliation gates fail closed', () => {
  assert.equal(fairLaunchActionAvailable(base, 'claim').enabled, true)
  assert.equal(fairLaunchActionAvailable({ ...base, actions: {} }, 'claim').enabled, false)
  assert.equal(fairLaunchActionAvailable({ ...base, finality: 'confirming' }, 'claim').enabled, false)
  assert.equal(fairLaunchActionAvailable({ ...base, reconciliationState: 'DRIFT' }, 'claim').enabled, false)
  assert.equal(fairLaunchActionAvailable({ ...base, reconciliationState: 'DRIFT' }, 'refund').enabled, true)
})

test('proportional estimate uses bigint above 2^53 and floors deterministically', () => {
  assert.equal(calculateProportionalAllocation('9007199254740993', '1000000000000000000000000', '18014398509481987').toString(), '499999999999999972244424')
  assert.equal(calculateProportionalAllocation('1', '10', '3'), 3n)
})

test('wallet service contains zero-first exact approvals, pending nonce and broadcast-unknown safeguards', () => {
  const source = readFileSync(new URL('../src/services/contract.js', import.meta.url), 'utf8')
  assert.match(source, /current !== 0n && target !== 0n && current !== target\) await approve\(0n\)/)
  assert.match(source, /if \(current !== target\) await approve\(target\)/)
  assert.match(source, /Let the wallet\/signer choose the pending nonce/)
  assert.match(source, /TRANSACTION_BROADCAST_UNCERTAIN/)
  assert.match(source, /projectTokenRuntimeMatches/)
  assert.match(source, /paymentTokenRuntimeMatches/)
  assert.match(source, /requires the projectOwner wallet/)
  assert.match(source, /await this\._preflightFairLaunch\(context, action, expectedState\)/)
  assert.match(source, /requires eth_estimateGas support/)
  assert.match(source, /claim: \['project'\]/)
  assert.match(source, /refund: \[\]/)
  assert.match(source, /recordFairLaunchUnresolved/)
})

test('C-end and owner panels expose accessible gated actions and release distinction', () => {
  const publicSource = readFileSync(new URL('../src/components/FairLaunchReadOnlyPanel.vue', import.meta.url), 'utf8')
  const ownerSource = readFileSync(new URL('../src/components/FairLaunchOwnerPanel.vue', import.meta.url), 'utf8')
  assert.match(publicSource, /aria-live="polite"/)
  assert.match(publicSource, /Non-binding estimated allocation/)
  assert.match(publicSource, /awaiting Release; claim and payouts remain closed/)
  assert.doesNotMatch(ownerSource, /withdrawFairLaunchProjectProceeds/)
  assert.match(ownerSource, /fixed Safes/)
  assert.match(ownerSource, /depositFairLaunchProjectTokens/)
  assert.match(ownerSource, /min-height:44px/)
})
