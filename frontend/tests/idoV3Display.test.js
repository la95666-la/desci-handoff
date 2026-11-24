import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { Interface } from 'ethers'
import { IDO_V3_ABI } from '../src/config/idoV3Abi.js'
import { createDisplaySingleFlight, readV3DisplaySnapshot, withDisplayDeadline } from '../src/services/idoV3Display.js'

const proxy = '0x1111111111111111111111111111111111111111'
const owner = '0x2222222222222222222222222222222222222222'
const token = '0x3333333333333333333333333333333333333333'
const payment = '0x4444444444444444444444444444444444444444'
const project = { id: '43264371706560513', chainId: 56, contractAddress: proxy,
  fundingMode: 'capped_presale', contractVersion: 'ido_v3_stablecoin', onchainLocked: true,
  tokenAddress: token, paymentTokenAddress: payment, tokenDecimals: 18, paymentTokenDecimals: 18 }

function fixture(status = 1, failBalance = false) {
  const abi = new Interface(IDO_V3_ABI)
  const erc = new Interface(['function balanceOf(address) view returns (uint256)', 'function allowance(address,address) view returns (uint256)'])
  const calls = []
  let reorg = false
  const provider = {
    getNetwork: async () => ({ chainId: 56n }),
    getBlock: async (tag) => ({ number: 10, timestamp: 150, hash: `0x${(reorg && tag === 10 ? 'b' : 'a').repeat(64)}` }),
    call: async (tx) => {
      calls.push(tx)
      if (tx.to.toLowerCase() !== proxy) {
        const f = erc.getFunction(tx.data.slice(0, 10))
        if (f.name === 'balanceOf' && failBalance) throw Error('RPC unavailable')
        return erc.encodeFunctionResult(f, [24000n * 10n ** 18n])
      }
      const f = abi.getFunction(tx.data.slice(0, 10))
      if (f.name === 'getProject') return abi.encodeFunctionResult(f, [[
        BigInt(project.id), token, payment, owner, owner, 18, 18, 200, status, 250000000000000n,
        6000000000000000000n, 2000000000000000000n, 6000000000000000000n, 100n, 200n,
        0n, 0n, 0n, false, false, false, [], [],
      ]])
      if (f.name === 'getWhitelistPolicy') return abi.encodeFunctionResult(f, [[true, `0x${'1'.repeat(64)}`, 1]])
      if (f.name === 'getUserPurchase') return abi.encodeFunctionResult(f, [[1n, 2n, 0n, false]])
      if (f.name === 'calculateClaimable') return abi.encodeFunctionResult(f, [2n])
      throw Error('Unexpected full-identity call: ' + f.name)
    },
  }
  return { provider, calls, reorg: () => { reorg = true } }
}

test('public V3 display performs two contract calls without runtime, wallet, proof or signer work', async () => {
  const f = fixture()
  const r = await readV3DisplaySnapshot(project, null, { chainId: 56, proxy }, f.provider)
  assert.equal(r.state.saleOpen, true)
  assert.equal(r.position, null)
  assert.equal(f.calls.length, 2)
  assert.ok(f.calls.every((c) => c.blockTag === 10))
})

test('live wallet display shares one fixed block across exactly five calls', async () => {
  const f = fixture()
  const r = await readV3DisplaySnapshot(project, owner, { chainId: 56, proxy }, f.provider)
  assert.equal(f.calls.length, 5)
  assert.ok(f.calls.every((c) => c.blockTag === 10))
  assert.equal(r.position.paymentAmountRaw, 1n)
  assert.equal(r.balance.formatted, '24000.0')
  assert.deepEqual(r.errors, {})
})

test('one failed wallet field stays null without discarding project or other wallet fields', async () => {
  const f = fixture(1, true)
  const r = await readV3DisplaySnapshot(project, owner, { chainId: 56, proxy }, f.provider)
  assert.equal(r.balance, null)
  assert.equal(r.errors.balance, 'IDO_V3_STATE_UNAVAILABLE')
  assert.equal(r.position.paymentAmountRaw, 1n)
  assert.equal(r.allowance.raw, 24000n * 10n ** 18n)
  assert.equal(r.state.status, 1)
})

test('owner display reads project token and terminal claim display skips payment balance', async () => {
  const f = fixture(0)
  const r = await readV3DisplaySnapshot(project, owner, { chainId: 56, proxy }, f.provider)
  assert.equal(r.ownerFunding.balance, 24000n * 10n ** 18n)
  assert.equal(f.calls.filter((c) => c.to.toLowerCase() === token).length, 2)
  const terminal = fixture(2)
  const c = await readV3DisplaySnapshot(project, owner, { chainId: 56, proxy }, terminal.provider)
  assert.equal(c.claimable.raw, 2n)
  assert.equal(terminal.calls.length, 4)
})

test('display still rejects mismatched project identity and changed block', async () => {
  const f = fixture()
  await assert.rejects(readV3DisplaySnapshot({ ...project, tokenAddress: payment }, owner, { chainId: 56, proxy }, f.provider))
  f.reorg()
  await assert.rejects(readV3DisplaySnapshot(project, null, { chainId: 56, proxy }, f.provider), { code: 'IDO_V3_STATE_UNAVAILABLE' })
})

test('same display key joins in flight, different wallet isolates, completed and failed reads are not cached', async () => {
  const flight = createDisplaySingleFlight()
  let release, calls = 0
  const a = flight('project:walletA', () => { calls++; return new Promise((r) => { release = r }) })
  assert.equal(flight('project:walletA', () => { throw Error('duplicate') }), a)
  await flight('project:walletB', async () => { calls++; return 2 })
  release(1)
  assert.equal(await a, 1)
  assert.equal(calls, 2)
  await assert.rejects(flight('project:walletA', async () => { throw Error('fail') }))
  assert.equal(await flight('project:walletA', async () => 3), 3)
})

test('panel uses display reads only, preserves values on refresh and separates display errors', () => {
  const panel = readFileSync(new URL('../src/components/LaunchpadActiveFundingPanel.vue', import.meta.url), 'utf8')
  const refresh = panel.slice(panel.indexOf('async function loadV3Display'), panel.indexOf('async function refreshRaised'))
  assert.match(refresh, /readV3Display\(context, displayWallet\)/)
  assert.doesNotMatch(refresh, /readV3State|getVerifiedV3Wallet|readV3PurchaseAuthority|v3State\.value = null/)
  assert.doesNotMatch(refresh, /v3Failure\.value = 'IDO_V3_STATE_UNAVAILABLE'/)
  assert.match(panel, /watch\(v3ReadKey/)
  assert.match(panel, /document\.visibilityState === 'visible'/)
  assert.match(panel, /isV3\.value \? 120000 : 5000/)
  const contract = readFileSync(new URL('../src/services/contract.js', import.meta.url), 'utf8')
  const writes = contract.slice(contract.indexOf('async _v3Submit'), contract.indexOf('async readV3ProjectTokenFunding'))
  assert.doesNotMatch(writes, /readV3Display/)
  assert.match(writes, /_v3FreshProject/)
  assert.match(writes, /_v3PurchaseArguments/)
  assert.match(writes, /recordV3Pending/)
})

test('stalled display reads time out without authorizing or submitting anything', async () => {
  await assert.rejects(withDisplayDeadline(() => new Promise(() => {}), 5), { code: 'IDO_V3_STATE_UNAVAILABLE' })
  assert.equal(await withDisplayDeadline(async () => 42, 100), 42)
})
