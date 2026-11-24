import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dashboardOwnerIdentity, ownerFundingReady, ownerFundingVisible, ownerDepositComplete } from '../src/utils/dashboardOwnerFunding.js'
import { normalizeProjectContext } from '../src/services/projectContext.js'
import { assertV3Project } from '../src/services/idoV3Wallet.js'
import { computeSoftAndHardRequirement, formatToken } from '../src/utils/depositMath.js'

const api = {
  id: '43264371706560513', fundingMode: 'capped_presale', contractVersion: 'ido_v3_stablecoin',
  chainId: 56, contractAddress: '0x0d4040875742975269c55e16c8884d3ee9510b8e',
  onchainLocked: true, feeReceiver: '0xc9e415dbe1b25f8a6e574505bf63bf44de385fb0',
  tokenAddress: '0x5aeec1e427586922d676f08b453ce692b13c7ed4', tokenDecimals: 18,
  paymentTokenAddress: '0x55d398326f99059ff775485246999027b3197955', paymentTokenDecimals: 18,
}

test('Dashboard preserves V3 lock and fee receiver through list mapping and normalization', () => {
  const context = normalizeProjectContext({ ...api, ...dashboardOwnerIdentity(api) }, {})
  assert.equal(assertV3Project(context, { chainId: 56, proxy: api.contractAddress }), context)
  assert.equal(context.feeReceiver, api.feeReceiver)
  for (const onchainLocked of [false, undefined, 1, 'true']) {
    const invalid = { ...context, ...dashboardOwnerIdentity({ ...api, onchainLocked }) }
    assert.throws(() => assertV3Project(invalid, { chainId: 56, proxy: api.contractAddress }), { code: 'IDO_V3_IDENTITY_UNAVAILABLE' })
  }
})

test('TESTDOG funding fixture is 24000 tokens at 0.00025 USDT, not zero', () => {
  const tokenPriceRaw = 250000000000000n
  const req = computeSoftAndHardRequirement({ softCapRaw: 2n * 10n ** 18n,
    hardCapRaw: 6n * 10n ** 18n, tokenPriceRaw, depositedRaw: 0n, tokenDecimals: 18 })
  assert.equal(formatToken(tokenPriceRaw, 18, { maxDecimals: 8 }), '0.00025')
  assert.equal(req.softCapRequiredRaw, 8000n * 10n ** 18n)
  assert.equal(req.hardCapRequiredRaw, 24000n * 10n ** 18n)
  assert.equal(formatToken(req.shortfallRaw, 18), '24,000')
})

test('V3 funding remains unavailable during initial, failed, refreshing and unresolved reads', () => {
  const p = { ...api, deposit: { statusLoading: false }, v3FundingLoaded: true }
  assert.equal(ownerFundingReady(p), true) // Actual zero balances remain displayable.
  for (const patch of [{ v3FundingLoaded: false }, { v3FundingLoaded: undefined },
    { deposit: { statusLoading: true } }, { v3Failure: 'IDO_V3_IDENTITY_UNAVAILABLE' },
    { v3Pending: [{ hash: 'pending' }] }]) {
    assert.equal(ownerFundingReady({ ...p, ...patch }), false)
  }
  assert.equal(ownerFundingReady({ contractVersion: 'v2_stablecoin' }), true)
})

test('both Dashboard layouts hide unverified values and prevent stale funding submissions', () => {
  const s = readFileSync(new URL('../src/views/Dashboard.vue', import.meta.url), 'utf8')
  assert.match(s, /\.\.\.dashboardOwnerIdentity\(item\)/)
  assert.equal(s.match(/v-if="!ownerFundingVisible\(project\)"/g)?.length, 2)
  assert.match(s, /if \(!ownerFundingReady\(project\)\) return null/)
  assert.match(s, /project\.v3FundingLoaded = false/)
  assert.match(s, /project\.v3FundingLoaded = true/)
})

test('pending approval does not hide verified funding data or authorize another send', () => {
  const project = { ...api, deposit: { statusLoading: false }, v3FundingLoaded: true,
    v3Failure: 'IDO_V3_SUBMISSION_UNRESOLVED', v3Pending: [{ hash: 'original' }] }
  assert.equal(ownerFundingVisible(project), true)
  assert.equal(ownerFundingReady(project), false)
  assert.equal(ownerFundingVisible({ ...project, v3Failure: 'IDO_V3_IDENTITY_UNAVAILABLE' }), false)
  const source = readFileSync(new URL('../src/views/Dashboard.vue', import.meta.url), 'utf8')
  const refresh = source.slice(source.indexOf('async function refreshV3ProjectChainData'), source.indexOf('async function refreshAllProjectsChain'))
  assert.doesNotMatch(refresh, /v3Failure = 'IDO_V3_SUBMISSION_UNRESOLVED'\s+return/)
})

test('Pending badge completes only after verified deposits cover exact Hard Cap token requirement', () => {
  const p = { ...api, status: 'Pending', v3FundingLoaded: true, deposit: { statusLoading: false },
    hardCapRequired: { raw: 24000n * 10n ** 18n, valid: true }, depositedRaw: 24000n * 10n ** 18n }
  assert.equal(ownerDepositComplete(p), true)
  assert.equal(p.status, 'Pending') // Still awaiting activation, not Active.
  assert.equal(ownerDepositComplete({ ...p, depositedRaw: p.depositedRaw + 1n }), true)
  for (const patch of [{ depositedRaw: p.depositedRaw - 1n }, { depositedRaw: 8000n * 10n ** 18n },
    { v3FundingLoaded: false }, { deposit: { statusLoading: true } }, { v3Failure: 'IDO_V3_STATE_UNAVAILABLE' },
    { hardCapRequired: { raw: 0n, valid: true } }, { hardCapRequired: { raw: p.depositedRaw, valid: false } },
    { depositedRaw: undefined }, { status: 'Setup' }, { status: 'Active' }, { status: 'Successful' }]) {
    assert.equal(ownerDepositComplete({ ...p, ...patch }), false)
  }
  assert.equal(ownerDepositComplete({ ...p, contractVersion: 'v2_stablecoin', fundingSnapshotLoaded: true }), true)
  assert.equal(ownerDepositComplete({ ...p, contractVersion: 'v2_stablecoin', fundingSnapshotLoaded: false }), false)
  assert.equal(ownerDepositComplete({ ...p, contractVersion: 'fair_launch_v1' }), false)
})

test('both owner layouts share green completed-deposit badge without altering lifecycle', () => {
  const s = readFileSync(new URL('../src/views/Dashboard.vue', import.meta.url), 'utf8')
  assert.equal(s.match(/:class="statusBadgeClass\(project\)"/g)?.length, 2)
  assert.match(s, /if \(ownerDepositComplete\(project\)\) return t.value.statusDepositComplete/)
  assert.match(s, /if \(ownerDepositComplete\(project\)\) return 'badge-green'/)
})
