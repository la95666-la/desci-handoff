import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import { v3FundingStage, v3WalletActions } from '../src/utils/v3FundingStage.js'

const now = 1_800_000_000_000
const project = { status: 'Active', startTime: 1_799_999_000, endTime: 1_800_001_000 }
const active = { status: 1, saleOpen: true, project: { startTime: 1_799_999_000n, endTime: 1_800_001_000n } }

test('V3 shared panel covers Pending, upcoming, live, awaiting settlement and terminal states', () => {
  assert.equal(v3FundingStage(project, { status: 0 }, now), 'pending')
  assert.equal(v3FundingStage(project, { ...active, saleOpen: false, project: { startTime: 1_800_000_100n, endTime: 1_800_001_000n } }, now), 'upcoming')
  assert.equal(v3FundingStage(project, active, now), 'live')
  assert.equal(v3FundingStage(project, { ...active, saleOpen: false }, 1_800_001_000_000), 'ended')
  for (const [status, expected] of [[2, 'successful'], [3, 'failed'], [4, 'canceled']]) {
    assert.equal(v3FundingStage(project, { status }, now), expected)
  }
  assert.notEqual(v3FundingStage({ status: 'Warmup' }, null, now), 'live')
})

test('V3 terminal actions require an actual wallet position and chain terminal state', () => {
  const position = { paymentAmountRaw: 1n, refunded: false }
  assert.deepEqual(v3WalletActions(active, position, 1n), { claim: false, refund: false })
  assert.deepEqual(v3WalletActions({ status: 2 }, position, 1n), { claim: true, refund: false })
  assert.deepEqual(v3WalletActions({ status: 2 }, position, 0n), { claim: false, refund: false })
  assert.deepEqual(v3WalletActions({ status: 3 }, position), { claim: false, refund: true })
  assert.deepEqual(v3WalletActions({ status: 4 }, { ...position, refunded: true }), { claim: false, refund: false })
  assert.deepEqual(v3WalletActions({ status: 3 }, { paymentAmountRaw: 0n, refunded: false }), { claim: false, refund: false })
  assert.deepEqual(v3WalletActions(null, position), { claim: false, refund: false })
})

test('single shared panel routes V3 away from V2 funding state and never takes signup as proof', () => {
  const panel = readFileSync(new URL('../src/components/LaunchpadActiveFundingPanel.vue', import.meta.url), 'utf8')
  const detail = readFileSync(new URL('../src/views/LaunchpadDetailPreLaunch.vue', import.meta.url), 'utf8')
  assert.match(detail, /fundingRoute\.kind === 'ido_v3' \|\|/)
  assert.doesNotMatch(detail, /<IdoV3FundingPanel/)
  assert.match(panel, /if \(isV3\.value\) return refreshV3\(\)[\s\S]*?getFundingState\(projectId\)/)
  for (const action of ['purchaseV3', 'approveV3PaymentToken', 'claimV3', 'refundV3', 'depositV3ProjectTokens']) {
    assert.ok(panel.includes(`contractService.${action}(`), action)
  }
  assert.match(panel, /contractService\.readV3Display\(context, displayWallet\)/)
  assert.doesNotMatch(panel, /contractService\.readV3PurchaseAuthority/)
  assert.match(panel, /if \(address === wallet\.address && projectId === String\(props\.project\?\.id \?\? ''\)\) showModal\.value = true/)
  assert.doesNotMatch(panel, /userStatus\.whitelisted|joinProjectWhitelist/)
})
