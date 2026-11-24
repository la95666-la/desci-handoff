import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { cappedPositionContext, loadPortfolioSource, positiveRaw, verifiedPortfolioContext, newestPortfolioRecords } from '../src/utils/dashboardPortfolio.js'

const dto = { projectID: '43359438123302913', fundingMode: 'capped_presale', contractVersion: 'ido_v3_stablecoin',
  contractAddress: '0x' + '1'.repeat(40), chainId: 56, projectTokenAddress: '0x' + '2'.repeat(40),
  projectTokenDecimals: 18, paymentTokenAddress: '0x' + '3'.repeat(40), paymentTokenDecimals: 18,
  onchainLocked: true, feeReceiver: '0x' + '4'.repeat(40) }

test('one missing portfolio source never erases other generations; error is explicit', async () => {
  const errors = []
  const [v2, v3] = await Promise.all([
    loadPortfolioSource(async () => ({ purchases: [{ id: 1 }] }), 'V2', errors),
    loadPortfolioSource(async () => { throw Error('404') }, 'IDO V3', errors),
  ])
  assert.equal(v2.purchases.length, 1)
  assert.deepEqual(v3, { positions: [], purchases: [], claims: [], refunds: [] })
  assert.deepEqual(errors, ['IDO V3'])
})

test('capped position maps complete V3 identity without inventing lock evidence', async () => {
  const item = cappedPositionContext(dto)
  assert.equal(item.projectId, dto.projectID)
  assert.equal(item.onchainLocked, true)
  assert.equal(item.feeReceiver, dto.feeReceiver)
  assert.equal(cappedPositionContext({ ...dto, onchainLocked: undefined }).onchainLocked, undefined)
  let calls = 0
  const detail = { ...item, onchainLocked: true }
  const result = await verifiedPortfolioContext(item, async (id, options) => {
    calls++; assert.equal(id, dto.projectID); assert.equal(options.fresh, true); return detail
  })
  assert.equal(result.id, dto.projectID)
  assert.equal(calls, 1)
  for (const patch of [{ onchainLocked: false }, { tokenAddress: dto.paymentTokenAddress }, { chainId: 97 }, { contractVersion: 'v2_stablecoin' }]) {
    await assert.rejects(verifiedPortfolioContext(item, async () => ({ ...detail, ...patch })), { code: 'IDO_V3_IDENTITY_UNAVAILABLE' })
  }
  const deferred = await verifiedPortfolioContext(item, async () => { throw Error('service owns fresh identity') }, { deferFreshIdentity: true })
  assert.equal(deferred.id, dto.projectID)
  let hydrated = 0
  await verifiedPortfolioContext({ ...item, onchainLocked: undefined }, async () => { hydrated++; return detail }, { deferFreshIdentity: true })
  assert.equal(hydrated, 1)
})

test('raw availability does not treat missing amounts as claimable or refundable', () => {
  for (const raw of [null, undefined, '', '0', '-1', 'bad']) assert.equal(positiveRaw(raw), false)
  assert.equal(positiveRaw('1'), true)
  assert.equal(positiveRaw('1000000000000000000000000000'), true)
  assert.deepEqual(newestPortfolioRecords([{ time: 1 }, { time: 3 }, { time: 2 }]).map(x => x.time), [3, 2, 1])
})

test('V3 and V2 share Dashboard rows and theme-safe warnings; claim/refund use fresh exact identity', () => {
  const s = readFileSync(new URL('../src/views/Dashboard.vue', import.meta.url), 'utf8')
  assert.match(s, /loadPortfolioSource\(\(\) => getIdoV3Portfolio\(address\), 'IDO V3', sourceErrors\)/)
  assert.equal(s.match(/\.\.\.cappedPositionContext\(item\)/g)?.length, 2)
  assert.match(s, /claimV3\(project, address\)/)
  assert.match(s, /refundV3\(project, address\)/)
  assert.doesNotMatch(s, /Claim read network mismatch/)
  assert.equal(s.match(/v-if="portfolioErrors.length"/g)?.length, 2)
  assert.match(s, /\.home-dark \.dash-portfolio-notice/)
  assert.match(s, /\.home-dark \.dash-refund-available/)
  assert.match(s, /confirmedCappedRefunds.value\[pendingRefundKey\] = true/)
})

test('Dashboard does not probe disabled Fair Launch portfolio while V2/V3 remain enabled', () => {
  const s = readFileSync(new URL('../src/views/Dashboard.vue', import.meta.url), 'utf8')
  assert.doesNotMatch(s, /getFairLaunchPortfolio|fair-launch\/portfolio/)
  assert.match(s, /loadPortfolioSource\(\(\) => getV2Portfolio\(\), 'V2', sourceErrors\)/)
  assert.match(s, /loadPortfolioSource\(\(\) => getIdoV3Portfolio\(address\), 'IDO V3', sourceErrors\)/)
  assert.match(s, /const fairPortfolio = \{ positions: \[\], claims: \[\], refunds: \[\] \}/)
})
