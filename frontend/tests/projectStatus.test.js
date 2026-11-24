import assert from 'node:assert/strict'
import test from 'node:test'

import {
  deriveProjectStage,
  getProjectCountdownTarget,
  isCappedFundingPanelStage,
  getFundingProgress,
  isLaunchpadVisibleStatus,
  isRefundableProjectStage,
  isStalePreLaunchSaleClaim,
  normalizeProjectStatus,
  usesUnifiedProjectDetail,
} from '../src/utils/projectStatus.js'

test('normalizes project status values returned with inconsistent casing or whitespace', () => {
  assert.equal(normalizeProjectStatus('Active'), 'active')
  assert.equal(normalizeProjectStatus(' WARMUP '), 'warmup')
  assert.equal(normalizeProjectStatus(' Setup '), 'pre_launch')
  assert.equal(normalizeProjectStatus(null), '')
})

test('halted published V3 stays visible as syncing, never becomes live from its sale times', () => {
  assert.equal(isLaunchpadVisibleStatus('V3ReorgHalted'), true)
  assert.equal(usesUnifiedProjectDetail('V3ReorgHalted'), true)
  assert.equal(deriveProjectStage({status:'V3ReorgHalted',startTime:1,endTime:9999999999}), 'syncing')
  assert.equal(isCappedFundingPanelStage('syncing'), false)
  assert.equal(isRefundableProjectStage('syncing'), false)
})

test('shows every public launchpad status regardless of API casing', () => {
  for (const status of [
    'Active', 'active', 'Warmup', 'PRE_LAUNCH', ' pre_launch ', 'Setup', 'Set',
    'Successful', 'SuccessfulPendingRelease', 'Failed', 'Canceled', 'Cancelled', 'ended', 'refund',
    'closed', 'sold_out', 'vesting',
  ]) {
    assert.equal(isLaunchpadVisibleStatus(status), true, `${status} should be visible`)
  }

  for (const status of ['candidate', 'archived', '', null]) {
    assert.equal(isLaunchpadVisibleStatus(status), false, `${status} should be hidden`)
  }
})

test('calculates active funding progress from backend amounts and clamps invalid values', () => {
  assert.equal(getFundingProgress('0', '1000'), 0)
  assert.equal(getFundingProgress('250', '1000'), 25)
  assert.equal(getFundingProgress('1200', '1000'), 100)
  assert.equal(getFundingProgress('-10', '1000'), 0)
  assert.equal(getFundingProgress('10', '0'), 0)
  assert.equal(getFundingProgress('invalid', '1000'), 0)
})

test('uses the content-rich detail template across the public launchpad lifecycle', () => {
  for (const status of [
    'pre_launch',
    'Setup',
    'Warmup',
    'Set',
    'Active',
    'Pending',
    'Successful',
    'SuccessfulPendingRelease',
    'Failed',
    'Canceled',
    'Cancelled',
    'sold_out',
    'vesting',
    'closed',
    'refund',
  ]) {
    assert.equal(usesUnifiedProjectDetail(status), true)
  }
  for (const status of ['candidate', 'archived', '', null]) {
    assert.equal(usesUnifiedProjectDetail(status), false)
  }
})

test('normalizes terminal aliases into terminal funding stages', () => {
  const base = { raised: '500', softCap: '500', startTime: '1', endTime: '2' }
  assert.equal(deriveProjectStage({ ...base, status: 'Successful' }, 3_000), 'successful')
  assert.equal(deriveProjectStage({ ...base, status: 'SuccessfulPendingRelease' }, 3_000), 'successful_pending_release')
  assert.equal(deriveProjectStage({ ...base, status: 'Failed' }, 3_000), 'failed')
  assert.equal(deriveProjectStage({ ...base, status: 'Cancelled' }, 3_000), 'canceled')
  assert.equal(deriveProjectStage({ ...base, status: 'vesting' }, 3_000), 'successful')
  assert.equal(deriveProjectStage({ ...base, status: 'closed' }, 3_000), 'ended_success')
  assert.equal(
    deriveProjectStage({ ...base, status: 'closed', raised: '100' }, 3_000),
    'ended_failed',
  )
})

test('derives fair-launch lifecycle above 2^53 without Number coercion', () => {
  const project = {
    status: 'Active',
    startTime: '1',
    endTime: '9007199254740993',
    totalRaisedRaw: '9007199254740993',
    softCapRaw: '9007199254740992',
  }
  assert.equal(deriveProjectStage(project, 9007199254740992999n), 'live')
  assert.equal(deriveProjectStage(project, 9007199254740993000n), 'ended_success')
  assert.equal(deriveProjectStage({ ...project, totalRaisedRaw: '9007199254740991' }, 9007199254740993000n), 'ended_failed')
})

test('derives backend Setup as the public pre-launch stage', () => {
  assert.equal(deriveProjectStage({ status: 'Setup' }), 'pre_launch')
})

test('refunds stay closed until the authoritative project status is failed or canceled', () => {
  assert.equal(isRefundableProjectStage('ended_failed'), false)
  assert.equal(isRefundableProjectStage('failed'), true)
  assert.equal(isRefundableProjectStage('canceled'), true)
  assert.equal(isRefundableProjectStage('refund'), true)
  assert.equal(isRefundableProjectStage('active'), false)
  assert.equal(isRefundableProjectStage('successful'), false)
})

test('active projects count down to start before sale, then to end once live', () => {
  const project = {
    status: 'Active',
    startTime: 1_788_235_825,
    endTime: 1_788_256_825,
    launchAt: 0,
  }

  const beforeStart = deriveProjectStage(project, 1_788_234_653_000)
  assert.equal(beforeStart, 'upcoming')
  assert.equal(getProjectCountdownTarget(project, beforeStart), project.startTime)

  const afterStart = deriveProjectStage(project, 1_788_236_000_000)
  assert.equal(afterStart, 'live')
  assert.equal(getProjectCountdownTarget(project, afterStart), project.endTime)

  assert.equal(
    getProjectCountdownTarget({ status: 'Setup', launchAt: 1_800_000_000 }, 'pre_launch'),
    1_800_000_000,
  )
})

test('Pending V2 shows terms and starts countdown from funding start, never marketing launch date', () => {
  const project = { status: 'Pending', startTime: 1_900_000_000, launchAt: 1_800_000_000 }
  assert.equal(deriveProjectStage(project, 1_850_000_000_000), 'pending')
  assert.equal(getProjectCountdownTarget(project, 'pending'), project.startTime)
  assert.equal(isCappedFundingPanelStage('pending'), true)
  assert.equal(isCappedFundingPanelStage('upcoming'), true)
  assert.equal(isCappedFundingPanelStage('pre_launch'), false)
  assert.equal(isCappedFundingPanelStage('active'), true)
  assert.equal(isCappedFundingPanelStage('ended'), true)
  assert.equal(deriveProjectStage(project, 1_950_000_000_000), 'pending') // not activated, even after start
})

test('detects stale phase-one no-sale claims after a project becomes active', () => {
  assert.equal(isStalePreLaunchSaleClaim('Phase 1 — no on-chain sale is live.'), true)
  assert.equal(isStalePreLaunchSaleClaim('Phase-1 has no on-chain sale.'), true)
  assert.equal(isStalePreLaunchSaleClaim('本次 Phase 1 僅提供內容，無鏈上發售。'), true)
  assert.equal(isStalePreLaunchSaleClaim('第一階段尚未開放鏈上發售。'), true)
  assert.equal(isStalePreLaunchSaleClaim('On-chain funding is live.'), false)
})
