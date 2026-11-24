import test from 'node:test'
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { mkdtempSync, rmSync, readFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { pathToFileURL, fileURLToPath } from 'node:url'
import { Interface } from 'ethers'

// Exercise the real production service with build-time runtime metadata, but
// fake all network/wallet seams. No RPC, signing or broadcasting in this test.
const require = createRequire(import.meta.url)
const esbuild = createRequire(require.resolve('vite/package.json'))('esbuild')
const temp = mkdtempSync(path.join(tmpdir(), 'jc-owner-preflight-'))
const proxy = '0x1111111111111111111111111111111111111111'
const owner = '0x2222222222222222222222222222222222222222'
const token = '0x3333333333333333333333333333333333333333'
const payment = '0x4444444444444444444444444444444444444444'
const hash = (c) => '0x' + c.repeat(64)
const config = { VITE_ENVIRONMENT: 'prod', VITE_IDO_V3_CHAIN_ID: '56', VITE_IDO_V3_PROXY: proxy,
  VITE_IDO_V3_IMPLEMENTATION: owner, VITE_IDO_V3_PROXY_CODE_HASH: hash('a'),
  VITE_IDO_V3_IMPLEMENTATION_CODE_HASH: hash('b'), VITE_IDO_V3_CONFIRMATIONS: '2' }
await esbuild.build({ entryPoints: [fileURLToPath(new URL('../src/services/contract.js', import.meta.url))],
  bundle: true, platform: 'node', format: 'esm', outfile: path.join(temp, 'service.mjs'),
  define: { 'import.meta.env': JSON.stringify(config) }, logLevel: 'silent' })
const { ContractService } = await import(pathToFileURL(path.join(temp, 'service.mjs')).href)
rmSync(temp, { recursive: true, force: true })
const context = { id: '43264371706560513', fundingMode: 'capped_presale', contractVersion: 'ido_v3_stablecoin',
  chainId: 56, contractAddress: proxy, tokenAddress: token, paymentTokenAddress: payment,
  tokenDecimals: 18, paymentTokenDecimals: 18, onchainLocked: true }
const erc = new Interface(['function balanceOf(address) view returns(uint256)', 'function allowance(address,address) view returns(uint256)'])
const amount = 24000n * 10n ** 18n

function fixture(action, patchFinal = {}, options = {}) {
  const service = new ContractService()
  const counts = { fullReads: 0, signer: 0, fresh: 0, simulations: 0, tokenReads: 0, sends: 0 }
  const state = { blockNumber: 10, blockHash: hash('a'), status: action === 'withdrawV3RaisedFunds' ? 2 : action === 'withdrawV3ProjectTokens' ? 3 : 0,
    enabled: false, root: hash('0'), version: '0', project: { projectOwner: owner, projectToken: token,
      projectTokensWithdrawn: false, totalTokensDeposited: action === 'withdrawV3ProjectTokens' ? amount : 0n,
      fundsWithdrawn: false, totalRaised: 130680000000000000000n, successFeeRate: 100n,
      totalTokensSold: 0n, hardCap: 6n * 10n ** 18n, tokenPrice: 250000000000000n } }
  service._v3Signer = async () => { counts.signer++; return { context, from: options.changedWallet && counts.signer > 1 ? payment : owner } }
  service._v3FreshProject = async () => { counts.fresh++; if (options.freshError) throw Error('FRESH_REJECTED') }
  service._v3Estimate = async () => { counts.simulations++; if (options.simulationError) throw Error('SIMULATION_REJECTED'); return 100000n }
  service.readV3State = async () => {
    counts.fullReads++
    return counts.fullReads === 1 ? state : { ...state, ...options.finalState, project: { ...state.project, ...patchFinal } }
  }
  service._v3ReadRunner = () => ({
    getBlock: async () => ({ hash: options.reorg ? hash('b') : hash('a') }),
    call: async (tx) => {
      counts.tokenReads++
      assert.equal(tx.blockTag, 10)
      assert.equal(tx.to.toLowerCase(), token)
      const f = erc.getFunction(tx.data.slice(0, 10))
      const decoded = erc.decodeFunctionData(f, tx.data)
      assert.equal(decoded[0].toLowerCase(), owner)
      if (f.name === 'allowance') assert.equal(decoded[1].toLowerCase(), proxy)
      const final = counts.fullReads > 1
      return erc.encodeFunctionResult(f, [f.name === 'balanceOf'
        ? (final ? options.finalBalance ?? options.balance ?? amount : options.balance ?? amount)
        : (final ? options.finalAllowance ?? options.allowance ?? amount : options.allowance ?? amount)])
    },
  })
  service._v3WalletSend = async (to, data, from) => {
    counts.sends++
    assert.ok([...storage.values()].some((value) => value.includes('dataHash')))
    if (action === 'withdrawV3RaisedFunds') {
      assert.equal(to.toLowerCase(), proxy)
      assert.equal(from.toLowerCase(), owner)
      const abi = new Interface(['function withdrawRaisedFunds(uint256)'])
      assert.deepEqual([...abi.decodeFunctionData('withdrawRaisedFunds', data)], [BigInt(context.id)])
    }
    throw Error('STOP_AT_WALLET_BOUNDARY')
  }
  const run = () => action === 'withdrawV3ProjectTokens' || action === 'withdrawV3RaisedFunds'
    ? service[action](context, owner) : service[action](context, '24000', owner)
  return { run, counts }
}

const storage = new Map()
test('owner preflights use exactly two full snapshots and one simulation, with no display-state bypass', async (t) => {
  const oldWindow = globalThis.window
  const oldNavigator = Object.getOwnPropertyDescriptor(globalThis, 'navigator')
  globalThis.window = { localStorage: { getItem: (k) => storage.get(k) || null, setItem: (k, v) => storage.set(k, v) } }
  Object.defineProperty(globalThis, 'navigator', { configurable: true, value: { locks: { request: async (_, __, fn) => fn({}) } } })
  try {
    const empty = new ContractService()
    empty._v3Context = () => context
    empty._v3ReadRunner = () => { throw Error('No pending means no RPC reconciliation') }
    storage.clear()
    assert.equal(await empty.reconcileV3Pending(context, owner), true)
    for (const action of ['approveV3ProjectToken', 'depositV3ProjectTokens', 'withdrawV3ProjectTokens', 'withdrawV3RaisedFunds']) {
      await t.test(action, async () => {
        storage.clear()
        const f = fixture(action)
        await assert.rejects(f.run(), /STOP_AT_WALLET_BOUNDARY/)
        assert.deepEqual(f.counts, { fullReads: 2, signer: 2, fresh: 1, simulations: 1,
          tokenReads: action === 'approveV3ProjectToken' ? 2 : action === 'depositV3ProjectTokens' ? 4 : 0, sends: 1 })
        await assert.rejects(f.run(), { code: 'IDO_V3_SUBMISSION_UNRESOLVED' })
        assert.equal(f.counts.fullReads, 2)
        assert.equal(f.counts.sends, 1)
      })
    }
    const failures = [
      ['depositV3ProjectTokens', { projectOwner: payment }, {}],
      ['withdrawV3ProjectTokens', { projectTokensWithdrawn: true }, {}],
      ['withdrawV3ProjectTokens', {}, { finalState: { status: 1 } }],
      ['depositV3ProjectTokens', { totalTokensDeposited: 1n }, {}],
      ['approveV3ProjectToken', {}, { balance: 0n }],
      ['depositV3ProjectTokens', {}, { allowance: 0n }],
      ['approveV3ProjectToken', {}, { finalBalance: 0n }],
      ['depositV3ProjectTokens', {}, { finalAllowance: 0n }],
      ['depositV3ProjectTokens', {}, { changedWallet: true }],
      ['withdrawV3ProjectTokens', {}, { reorg: true }],
      ['approveV3ProjectToken', {}, { freshError: true }],
      ['depositV3ProjectTokens', {}, { simulationError: true }],
      ['depositV3ProjectTokens', {}, { finalState: { version: '1' } }],
      ['withdrawV3RaisedFunds', { projectOwner: payment }, {}],
      ['withdrawV3RaisedFunds', { fundsWithdrawn: true }, {}],
      ['withdrawV3RaisedFunds', { fundsWithdrawn: undefined }, {}],
      ['withdrawV3RaisedFunds', {}, { finalState: { status: 3 } }],
      ['withdrawV3RaisedFunds', {}, { changedWallet: true }],
      ['withdrawV3RaisedFunds', {}, { reorg: true }],
      ['withdrawV3RaisedFunds', {}, { simulationError: true }],
    ]
    for (const [action, patch, options] of failures) {
      storage.clear()
      const f = fixture(action, patch, options)
      await assert.rejects(f.run())
      assert.equal(f.counts.sends, 0)
      assert.equal(storage.size, 0)
    }
    storage.clear()
    globalThis.navigator.locks.request = async (_, __, fn) => fn(null)
    const blocked = fixture('withdrawV3ProjectTokens')
    await assert.rejects(blocked.run(), { code: 'IDO_V3_SUBMISSION_UNRESOLVED' })
    assert.equal(blocked.counts.fullReads, 0)
  } finally {
    globalThis.window = oldWindow
    if (oldNavigator) Object.defineProperty(globalThis, 'navigator', oldNavigator)
    else delete globalThis.navigator
  }
})

test('Dashboard approvals have active primary styling, explicit disabled colors and completed labels', () => {
  const s = readFileSync(new URL('../src/views/Dashboard.vue', import.meta.url), 'utf8')
  assert.match(s, /class="dash-btn-lime dash-owner-approve"/)
  assert.match(s, /class="m-btn-primary m-dash-row-btn dash-owner-approve"/)
  assert.match(s, /\.dash-owner-approve:disabled/)
  assert.equal(s.match(/allowanceCovers\(project\) \? t\.approveReady : t\.approveExact/g)?.length, 2)
  assert.match(s, /if \(!canApprove\(project\)\) return/)
  assert.match(s, /if \(!canDeposit\(project\)\) return/)
})
