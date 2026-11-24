import assert from 'node:assert/strict'
import test from 'node:test'

import { shouldAuthenticateAccountTransition } from '../src/services/accountTransitionPolicy.js'

test('same-address reconnect authenticates when connection changes from disconnected to connected', () => {
  assert.equal(shouldAuthenticateAccountTransition(
    { isConnected: true, address: '0xAbC', connector: { id: 'okx' } },
    { isConnected: false, address: '0xAbC', connector: { id: 'okx' } },
  ), true)
})

test('connected account or connector changes authenticate', () => {
  assert.equal(shouldAuthenticateAccountTransition(
    { isConnected: true, address: '0xdef', connector: { id: 'okx' } },
    { isConnected: true, address: '0xabc', connector: { id: 'okx' } },
  ), true)
  assert.equal(shouldAuthenticateAccountTransition(
    { isConnected: true, address: '0xabc', connector: { id: 'walletConnect' } },
    { isConnected: true, address: '0xAbC', connector: { id: 'okx' } },
  ), true)
})

test('unchanged connected account and invalid disconnected states do not authenticate again', () => {
  assert.equal(shouldAuthenticateAccountTransition(
    { isConnected: true, address: '0xabc', connector: { id: 'okx' } },
    { isConnected: true, address: '0xAbC', connector: { id: 'okx' } },
  ), false)
  assert.equal(shouldAuthenticateAccountTransition({ isConnected: false, address: '0xabc' }, {}), false)
  assert.equal(shouldAuthenticateAccountTransition({ isConnected: true, address: null }, {}), false)
})
