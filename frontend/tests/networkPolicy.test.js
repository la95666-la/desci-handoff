import assert from 'node:assert/strict'
import test from 'node:test'

import {
  CHAIN_IDS,
  createLoginNetworkPolicy,
  isChainTrusted,
  normalizeChainId,
  parseChainIdList,
} from '../src/services/networkPolicy.js'

test('normalizeChainId handles every wire format', () => {
  assert.equal(normalizeChainId(97), 97)
  assert.equal(normalizeChainId(56n), 56)
  assert.equal(normalizeChainId('97'), 97)
  assert.equal(normalizeChainId('31337'), 31337)
  assert.equal(normalizeChainId('0x61'), 97)
  assert.equal(normalizeChainId('0X38'), 56)
  assert.equal(normalizeChainId(' 0x7a69 '), 31337)
  assert.equal(normalizeChainId(null), null)
  assert.equal(normalizeChainId(undefined), null)
  assert.equal(normalizeChainId(NaN), null)
  assert.equal(normalizeChainId(''), null)
  assert.equal(normalizeChainId('0x'), null)
  assert.equal(normalizeChainId('abc'), null)
  assert.equal(normalizeChainId({}), null)
})

test('production build trusts BSC mainnet only, and may auto-switch to 56', () => {
  const p = createLoginNetworkPolicy({ environment: 'prod' })
  assert.deepEqual(p.trustedChainIds, [56])
  assert.equal(p.preferredChainId, 56)
  assert.equal(p.autoSwitch, true)

  // 生產構建即使被塞了 testnet chain id，也不放寬
  const strict = createLoginNetworkPolicy({ environment: 'prod', checkinChainId: '97' })
  assert.deepEqual(strict.trustedChainIds, [56])
})

test('local hardhat build trusts 56/97/31337 and never force-switches to 97', () => {
  const p = createLoginNetworkPolicy({ environment: 'local', checkinChainId: 31337 })
  assert.deepEqual(p.trustedChainIds, [56, 97, CHAIN_IDS.HARDHAT])
  assert.equal(p.preferredChainId, CHAIN_IDS.HARDHAT)
  assert.equal(p.autoSwitch, false)

  // local 構建未聲明 chain id → 默認硬編碼測試鏈，同樣不自動切鏈
  const defaulted = createLoginNetworkPolicy({ environment: 'local' })
  assert.deepEqual(defaulted.trustedChainIds, [56, 97, CHAIN_IDS.HARDHAT])
  assert.equal(defaulted.autoSwitch, false)
})

test('test build trusts 56/97 and prefers 97 with auto-switch', () => {
  const p = createLoginNetworkPolicy({ environment: 'test' })
  assert.deepEqual(p.trustedChainIds, [56, 97])
  assert.equal(p.preferredChainId, 97)
  assert.equal(p.autoSwitch, true)
})

test('test env pointing at a remote hardhat trusts the hardhat chain and disables auto-switch', () => {
  const p = createLoginNetworkPolicy({ environment: 'test', checkinChainId: 31337 })
  assert.deepEqual(p.trustedChainIds, [56, 97, 31337])
  assert.equal(p.autoSwitch, false)
})

test('explicit mainnet chain id wins even in a local build', () => {
  const p = createLoginNetworkPolicy({ environment: 'local', checkinChainId: 56 })
  assert.deepEqual(p.trustedChainIds, [56])
  assert.equal(p.autoSwitch, true)
})

test('explicit testnet chain id in a local build behaves like the test build', () => {
  const p = createLoginNetworkPolicy({ environment: 'local', checkinChainId: 97 })
  assert.deepEqual(p.trustedChainIds, [56, 97])
  assert.equal(p.preferredChainId, 97)
  assert.equal(p.autoSwitch, true)
})

test('fully unconfigured build fails closed to mainnet-only', () => {
  const p = createLoginNetworkPolicy({})
  assert.deepEqual(p.trustedChainIds, [56])
  assert.equal(p.autoSwitch, true)
})

// 回歸（登入守門判定的直接回歸）：
// 舊 signAndSubmitAuth 硬編碼 allowedChainIds = [56, 97]，本地跑 31337 會被拒登；
// 現在登入守門統一走 isChainTrusted(policy, chainId)，local 構建必須信任 31337
test('login gate: local hardhat build trusts 31337 (regression: old code rejected it)', () => {
  const local = createLoginNetworkPolicy({ environment: 'local', checkinChainId: 31337 })
  assert.equal(isChainTrusted(local, 31337), true)
  assert.equal(isChainTrusted(local, 97), true)
  assert.equal(isChainTrusted(local, 56), true)
  // hex 形態也要能判
  assert.equal(isChainTrusted(local, '0x7a69'), true)
  // 非受信鏈（如自定義 100）拒絕，保持 fail-closed
  assert.equal(isChainTrusted(local, 100), false)
})

test('login gate: production build only ever trusts BSC mainnet 56', () => {
  const prod = createLoginNetworkPolicy({ environment: 'prod' })
  assert.equal(isChainTrusted(prod, 56), true)
  assert.equal(isChainTrusted(prod, 97), false)
  assert.equal(isChainTrusted(prod, 31337), false)
  // 即使錢包在 97，prod 也決不信任（「生產仍只允許 56」）
  assert.equal(isChainTrusted(prod, '0x61'), false)
})

// ============ 雙鏈發布形態（主網 CheckIn + 顯式聲明的項目鏈）============

test('dual-chain build: trustedProjectChains widens trust while keeping mainnet preferred', () => {
  const p = createLoginNetworkPolicy({
    environment: 'prod',
    checkinChainId: 56,
    trustedProjectChains: ['97'],
  })
  // 信任集 = [56, 97]，偏好鏈保持 56：登入/簽到落地主網，Launchpad 按需提示切 97
  assert.deepEqual(p.trustedChainIds, [56, 97])
  assert.equal(p.preferredChainId, 56)
  assert.equal(p.autoSwitch, true)
  assert.equal(isChainTrusted(p, 97), true)
  assert.equal(isChainTrusted(p, '0x61'), true)
  assert.equal(isChainTrusted(p, 31337), false)
  // 未聲明的鏈絕不放寬
  assert.equal(isChainTrusted(p, 100), false)
})

test('dual-chain build: duplicates and garbage entries are normalized away', () => {
  const p = createLoginNetworkPolicy({
    environment: 'prod',
    checkinChainId: 56,
    trustedProjectChains: ['97', 97, '0x61', 'not-a-chain', '', null, undefined],
  })
  assert.deepEqual(p.trustedChainIds, [56, 97])
})

test('dual-chain build: empty trustedProjectChains keeps fail-closed mainnet-only default', () => {
  const empty = createLoginNetworkPolicy({ environment: 'prod', trustedProjectChains: [] })
  assert.deepEqual(empty.trustedChainIds, [56])

  const garbageOnly = createLoginNetworkPolicy({
    environment: 'prod',
    trustedProjectChains: ['abc', ''],
  })
  assert.deepEqual(garbageOnly.trustedChainIds, [56])
  assert.equal(isChainTrusted(garbageOnly, 97), false)
})

test('test build ignores trustedProjectChains (already trusts both BSC chains)', () => {
  const p = createLoginNetworkPolicy({
    environment: 'test',
    checkinChainId: 97,
    trustedProjectChains: ['97'],
  })
  assert.deepEqual(p.trustedChainIds, [56, 97])
  assert.equal(p.preferredChainId, 97)
})

test('parseChainIdList normalizes, dedupes and drops garbage entries', () => {
  assert.deepEqual(parseChainIdList('97, 56,0x61, abc,,') , [97, 56])
  assert.deepEqual(parseChainIdList(''), [])
  assert.deepEqual(parseChainIdList(null), [])
  assert.deepEqual(parseChainIdList(undefined), [])
})
