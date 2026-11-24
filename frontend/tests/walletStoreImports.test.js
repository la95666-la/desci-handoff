import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const walletStoreSource = readFileSync(
  new URL('../src/stores/wallet.js', import.meta.url),
  'utf8',
)

test('wallet connection imports the chain normalizer used before authentication', () => {
  assert.match(
    walletStoreSource,
    /import\s*\{[^}]*\bnormalizeChainId\b[^}]*\}\s*from\s*['"]\.\.\/services\/networkPolicy(?:\.js)?['"]/s,
  )
  assert.match(walletStoreSource, /normalizeChainId\(liveChainId\)/)
})
