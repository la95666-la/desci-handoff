import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { initialOwnerStats, applyOwnerStats, ownerStatText } from '../src/utils/dashboardOwnerStats.js'
import { formatDecimalString } from '../src/utils/fairLaunch.js'

test('43368660559134718 ignores stale API zeros and displays 1.08 USDT / 4320 TESTDOG from chain raw units', () => {
  const project = { contractVersion: 'ido_v3_stablecoin', paymentTokenDecimals: 18, tokenDecimals: 18,
    ...initialOwnerStats({ contractVersion: 'ido_v3_stablecoin', totalRaised: '0', soldToken: '0' }) }
  assert.equal(ownerStatText(project, 'totalRaised', formatDecimalString), '—')
  assert.equal(ownerStatText(project, 'tokensSold', formatDecimalString), '—')
  applyOwnerStats(project, { totalRaised: 1080000000000000000n, totalTokensSold: 4320000000000000000000n })
  assert.equal(project.totalRaised, '1.08')
  assert.equal(project.tokensSold, '4320.0')
  assert.equal(ownerStatText(project, 'tokensSold', formatDecimalString), '4,320')
  assert.equal(project.ownerStatsLoaded, true)
})

test('different asset decimals and huge values retain precision; actual zero remains distinguishable from unknown', () => {
  const p = { contractVersion: 'ido_v3_stablecoin', paymentTokenDecimals: 6, tokenDecimals: 18 }
  applyOwnerStats(p, { totalRaised: '50000', totalTokensSold: '900719925474099312345678901234567890' })
  assert.equal(p.totalRaised, '0.05')
  assert.equal(p.tokensSold, '900719925474099312.34567890123456789')
  applyOwnerStats(p, { totalRaised: 0n, totalTokensSold: 0n })
  assert.equal(ownerStatText(p, 'totalRaised', formatDecimalString), '0')
  assert.deepEqual(initialOwnerStats({ contractVersion: 'v2_stablecoin', totalRaised: '1.08', soldToken: '4320' }),
    { totalRaised: '1.08', tokensSold: '4320', ownerStatsLoaded: true, ownerStatsError: false })
})

test('public V3 owner totals run before wallet binding; both layouts show chain values or unavailable, not placeholder zeros', () => {
  const s = readFileSync(new URL('../src/views/Dashboard.vue', import.meta.url), 'utf8')
  const refresh = s.slice(s.indexOf('async function refreshV3ProjectChainData'), s.indexOf('function refreshPendingData'))
  assert.ok(refresh.indexOf('contractService.readV3Display(project)') < refresh.indexOf('getVerifiedV3Wallet(address)'))
  assert.match(refresh, /applyOwnerStats\(project, state.project\)/)
  assert.match(refresh, /if \(state.status !== 0\) return/)
  assert.ok(refresh.indexOf('const publicReads') < refresh.indexOf('web3Service.ensureSigner()'))
  assert.equal(s.match(/ownerStatText\(project, 'tokensSold', fmtNum\)/g).length, 2)
  assert.equal(s.match(/\{\{ ownerRaisedText\(project\) \}\}/g).length, 4)
  assert.match(s, /ownerStatText\(project, 'totalRaised', \(amount\) => formatRaisedAmount\(amount, project.paymentToken\)\)/)
  assert.equal(s.match(/t.ownerStatsUnavailable/g).length, 2)
})

test('actual Dashboard refresh updates successful owner totals without connected wallet, signer or binding', async () => {
  const source = readFileSync(new URL('../src/views/Dashboard.vue', import.meta.url), 'utf8')
  const body = source.slice(source.indexOf('async function refreshV3ProjectChainData'), source.indexOf('async function refreshAllProjectsChain'))
  const p = { projectId: '43368660559134718', contractVersion: 'ido_v3_stablecoin', status: 'Successful',
    paymentTokenDecimals: 18, tokenDecimals: 18, deposit: {}, ...initialOwnerStats({ contractVersion: 'ido_v3_stablecoin' }) }
  let fail = false
  const contract = { readV3Display: async (project, wallet) => {
    assert.equal(project, p); assert.equal(wallet, undefined)
    if (fail) throw Object.assign(new Error('RPC unavailable'), { code: 'IDO_V3_STATE_UNAVAILABLE' })
    return { state: { status: 2, project: { totalRaised: 1080000000000000000n,
      totalTokensSold: 4320000000000000000000n, fundsWithdrawn: true, projectTokensWithdrawn: false } } }
  }, v3Pending: () => [] }
  const refresh = new Function('contractService', 'walletStore', 'myProjects', 'getVerifiedV3Wallet', 'applyOwnerStats', 'V2_STATUS_MAP',
    body + ';return refreshV3ProjectChainData')(contract, { address: null, isConnected: false }, { value: [p] },
      () => { throw Error('Display must not check wallet binding') }, applyOwnerStats,
      ['Pending', 'Active', 'Successful', 'Failed', 'Canceled'])
  await refresh(p)
  assert.equal(p.totalRaised, '1.08'); assert.equal(p.tokensSold, '4320.0')
  assert.equal(p.fundsWithdrawn, true); assert.equal(p.deposit.statusLoading, false)
  fail = true
  await refresh(p)
  assert.equal(p.ownerStatsError, true)
  assert.equal(p.totalRaised, '1.08') // Refresh failure preserves last known facts.
  assert.equal(p.tokensSold, '4320.0')
})
