import assert from 'node:assert/strict'
import test from 'node:test'
import { applyVerifiedClaimPosition, claimActionState, claimInProgressFor } from '../src/utils/claimAvailability.js'

const now = 1_000_000
const v2 = { contractVersion: 'v2_stablecoin', claimableRaw: '500', claimableAmount: 500, claimedRaw: '0', nextUnlockTime: now + 1000 }

test('only the selected wallet/project claim displays Claiming across layouts', () => {
  const first = '0xabc:41320065328480257'
  const second = '0xabc:41340259090300929'
  assert.equal(claimInProgressFor(first, first), true)
  assert.equal(claimInProgressFor(second, first), false)
  assert.equal(claimInProgressFor('0xdef:41320065328480257', first), false)
  assert.equal(claimInProgressFor(first, null), false)
})

test('confirmed claim remains disabled and labeled Claimed despite a stale portfolio refresh', () => {
  assert.deepEqual(claimActionState(v2, now - 10, now), { claimed: true, disabled: true })
  assert.deepEqual(claimActionState({ ...v2, claimableRaw: '0', claimedRaw: '500' }, 0, now), { claimed: true, disabled: true })
})

test('next unlock releases the local guard and still requires a positive claimable amount', () => {
  assert.deepEqual(claimActionState(v2, now - 10, now + 1000), { claimed: false, disabled: false })
  assert.deepEqual(claimActionState({ ...v2, claimableRaw: '0' }, now - 10, now + 1000), { claimed: false, disabled: true })
  assert.deepEqual(claimActionState({ ...v2, nextUnlockTime: 0 }, now - 10, now), { claimed: true, disabled: true })
})

test('pending first tranche stays disabled but is not mislabeled as Claimed', () => {
  assert.deepEqual(claimActionState({ ...v2, claimableRaw: '0' }, 0, now), { claimed: false, disabled: true })
})

test('raw V2 amount takes priority over rounded display without changing legacy or fair-launch actions', () => {
  assert.deepEqual(claimActionState({ ...v2, claimableRaw: '1', claimableAmount: 0 }, 0, now), { claimed: false, disabled: false })
  assert.deepEqual(claimActionState({ ...v2, claimableRaw: '0', claimableAmount: 10 }, 0, now), { claimed: false, disabled: true })
  assert.deepEqual(claimActionState({ contractVersion: 'v1_legacy', claimableAmount: 10, nextUnlockTime: now + 1000 }, now - 10, now), { claimed: false, disabled: false })
})

test('refresh uses verified chain position instead of lagging indexed claim totals', () => {
  const stale = { ...v2, claimableRaw: '500', claimedRaw: '0' }
  const checked = applyVerifiedClaimPosition(stale, 0n, 500n, '0')
  assert.deepEqual(claimActionState(checked, 0, now), { claimed: true, disabled: true })
  assert.deepEqual(claimActionState(applyVerifiedClaimPosition(stale, 250n, 500n, '250'), 0, now + 1000), { claimed: false, disabled: false })
  assert.deepEqual(claimActionState({ ...stale, claimCheckUnavailable: true }, 0, now), { claimed: false, disabled: true, unavailable: true })
  assert.deepEqual(claimActionState(applyVerifiedClaimPosition({ ...stale, claimCheckUnavailable: true }, 0n, 500n, '0'), 0, now), { claimed: true, disabled: true })
})
