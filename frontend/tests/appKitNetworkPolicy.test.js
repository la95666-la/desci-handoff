import assert from 'node:assert/strict'
import test from 'node:test'

import { orderAppKitNetworks } from '../src/services/appKitNetworkPolicy.js'

const bsc = { id: 56 }
const bscTestnet = { id: 97 }
const hardhat = { id: 31337 }

test('local Hardhat build opens AppKit on 31337, never BSC 56', () => {
  assert.deepEqual(
    orderAppKitNetworks({ isMainnetBuild: false, checkInChainId: 31337, bsc, bscTestnet, hardhat }).map((n) => n.id),
    [31337, 97, 56],
  )
})

test('AppKit source suppresses unsupported-chain auto modal only for explicit local Hardhat builds', async () => {
  const { readFile } = await import('node:fs/promises')
  const source = await readFile(new URL('../src/services/reownAppKit.js', import.meta.url), 'utf8')
  assert.match(source, /const allowUnsupportedChain = checkInEnvironment === 'local' && checkInChainId === 31337/)
  assert.match(source, /createAppKit\(\{[\s\S]*allowUnsupportedChain,/)
  assert.doesNotMatch(source, /allowUnsupportedChain:\s*true/)
})

test('local wallet RPC is a separate exact HTTPS loopback authority and never replaces canonical admission RPC', async () => {
  const { readFile } = await import('node:fs/promises')
  const appKitSource = await readFile(new URL('../src/services/reownAppKit.js', import.meta.url), 'utf8')
  const admissionSource = await readFile(new URL('../src/services/fairLaunchSubmissionPolicy.js', import.meta.url), 'utf8')
  assert.match(appKitSource, /VITE_HARDHAT_WALLET_RPC/)
  assert.match(appKitSource, /parsed\.protocol !== 'https:'/)
  assert.match(appKitSource, /parsed\.hostname !== '127\.0\.0\.1'/)
  assert.match(admissionSource, /rpcUrl: env\?\.VITE_HARDHAT_RPC/)
  assert.doesNotMatch(admissionSource, /VITE_HARDHAT_WALLET_RPC/)
})

test('test build opens AppKit on BSC testnet 97', () => {
  assert.deepEqual(
    orderAppKitNetworks({ isMainnetBuild: false, checkInChainId: 97, bsc, bscTestnet, hardhat }).map((n) => n.id),
    [97, 56, 31337],
  )
})

test('production AppKit exposes and defaults to BSC mainnet only', () => {
  assert.deepEqual(
    orderAppKitNetworks({ isMainnetBuild: true, checkInChainId: 56, bsc, bscTestnet, hardhat }).map((n) => n.id),
    [56],
  )
})

// ============ 雙鏈發布形態：AppKit 可選網絡 ============

test('dual-chain build exposes mainnet (default) plus the declared testnet project chain', () => {
  assert.deepEqual(
    orderAppKitNetworks({ isMainnetBuild: true, checkInChainId: 56, bsc, bscTestnet, hardhat, trustedProjectChains: [97] }).map((n) => n.id),
    [56, 97],
  )
})

test('mainnet build without declared project chains stays mainnet-only', () => {
  assert.deepEqual(
    orderAppKitNetworks({ isMainnetBuild: true, checkInChainId: 56, bsc, bscTestnet, hardhat, trustedProjectChains: [] }).map((n) => n.id),
    [56],
  )
  assert.deepEqual(
    orderAppKitNetworks({ isMainnetBuild: true, checkInChainId: 56, bsc, bscTestnet, hardhat }).map((n) => n.id),
    [56],
  )
})

test('garbage or mainnet-duplicate project chain entries do not widen the network list', () => {
  assert.deepEqual(
    orderAppKitNetworks({ isMainnetBuild: true, checkInChainId: 56, bsc, bscTestnet, hardhat, trustedProjectChains: [56, 56] }).map((n) => n.id),
    [56],
  )
  assert.deepEqual(
    orderAppKitNetworks({ isMainnetBuild: true, checkInChainId: 56, bsc, bscTestnet, hardhat, trustedProjectChains: 'not-a-list' }).map((n) => n.id),
    [56],
  )
})
