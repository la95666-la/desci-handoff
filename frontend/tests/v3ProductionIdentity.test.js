import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { pinnedV3WalletRuntime } from '../src/services/idoV3Wallet.js'

const config = Object.fromEntries(readFileSync(new URL('../.env.production', import.meta.url), 'utf8')
  .split(/\r?\n/).filter((line) => /^VITE_(?:IDO_V3_[A-Z_]+|ENVIRONMENT)=/.test(line)).map((line) => {
    const at = line.indexOf('=')
    return [line.slice(0, at), line.slice(at + 1)]
  }))

test('reviewed BSC mainnet V3 identity is pinned without toggles or credential URLs', () => {
  assert.equal(config.VITE_ENVIRONMENT, 'prod')
  const runtime = pinnedV3WalletRuntime(config)
  assert.equal(runtime.chainId, 56)
  assert.equal(runtime.confirmations, 15)
  assert.equal(runtime.proxy.toLowerCase(), '0x0d4040875742975269c55e16c8884d3ee9510b8e')
  assert.equal(runtime.implementation.toLowerCase(), '0x887fcc6e54fc8229919166ad90b16c19b4992560')
  assert.deepEqual(pinnedV3WalletRuntime({ ...config, VITE_IDO_V3_WALLET_ENABLED: 'false' }), runtime)
  assert.throws(() => pinnedV3WalletRuntime({ ...config, VITE_ENVIRONMENT: 'test' }), { code: 'IDO_V3_ROUTING_UNAVAILABLE' })
  assert.equal(Object.keys(config).some((key) => /RPC|TOKEN|KEY|SECRET/.test(key)), false)
})
