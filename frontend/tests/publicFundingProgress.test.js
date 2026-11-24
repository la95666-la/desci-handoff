import test from 'node:test'
import assert from 'node:assert/strict'
import { publicFundingProgress } from '../src/utils/publicFundingProgress.js'

const project = { fundingMode: 'capped_presale', contractVersion: 'ido_v3_stablecoin',
  totalRaised: '16188.559724667414530874', hardCap: '100000', fundingDataStatus: 'ready' }
test('V3 progress uses exact public projected decimal amounts', () => {
  assert.deepEqual(publicFundingProgress(project), { ready: true, percent: '16.18' })
  assert.deepEqual(publicFundingProgress({ ...project, totalRaised: '0' }), { ready: true, percent: '0.00' })
})
test('missing or failed projection is unknown, not zero', () => {
  for (const patch of [{ fundingDataStatus: 'syncing' }, { totalRaised: null }, { totalRaised: undefined },
    { totalRaised: '' }, { totalRaised: '-1' }, { hardCap: '0' }, { totalRaised: 'invalid' }]) {
    assert.deepEqual(publicFundingProgress({ ...project, ...patch }), { ready: false, percent: null })
  }
})
test('V2 and Fair Launch retain separate progress routes', () => {
  assert.equal(publicFundingProgress({ ...project, contractVersion: 'v2_stablecoin' }), null)
  assert.equal(publicFundingProgress({ ...project, fundingMode: 'fair_launch' }), null)
})
