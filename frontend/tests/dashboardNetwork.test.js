import test from 'node:test'
import assert from 'node:assert/strict'
import { dashboardNetworkState } from '../src/utils/dashboardNetwork.js'

test('unconfigured drafts do not cause Connected 56 / Expected 56 warnings', () => {
  assert.deepEqual(dashboardNetworkState([
    { status: 'Active', chainId: 56 },
    { status: 'Setup', chainId: 0 },
    { status: 'Set', chainId: 97 },
    { status: 'Pending' },
  ], 56, 56), { wrongNetwork: false, expectedChainId: 56 })
})

test('mixed-chain warning names the mismatched chain, not the first matching chain', () => {
  const items = [{ status: 'Active', chainId: 56 }, { status: 'Pending', chainId: 97 }]
  assert.deepEqual(dashboardNetworkState(items, 56, 56), { wrongNetwork: true, expectedChainId: 97 })
  assert.deepEqual(dashboardNetworkState(items, 97, 56), { wrongNetwork: true, expectedChainId: 56 })
})

test('missing chain identity is not a wrong-network diagnosis', () => {
  for (const chainId of [undefined, null, '', 0, -1, 'bad', 1.5]) {
    assert.deepEqual(dashboardNetworkState([{ chainId }], 56, 56), { wrongNetwork: false, expectedChainId: 56 })
  }
  assert.deepEqual(dashboardNetworkState([{ chainId: 97 }], null, 56), { wrongNetwork: false, expectedChainId: 97 })
})
