import test from 'node:test'
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { mkdtempSync, rmSync, readFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { pathToFileURL, fileURLToPath } from 'node:url'
import { Interface, keccak256 } from 'ethers'
import { assertV3SendIdentity } from '../src/services/idoV3Wallet.js'

const proxy = '0x1111111111111111111111111111111111111111'
const wallet = '0x2222222222222222222222222222222222222222'
const token = '0x3333333333333333333333333333333333333333'
const payment = '0x4444444444444444444444444444444444444444'
const hash = c => '0x' + c.repeat(64)
const code = '0x60006000'
const runtime = { chainId: 56, proxy, implementation: wallet, proxyCodeHash: keccak256(code), implementationCodeHash: keccak256(code) }
const config = { VITE_ENVIRONMENT: 'prod', VITE_IDO_V3_CHAIN_ID: '56', VITE_IDO_V3_PROXY: proxy,
  VITE_IDO_V3_IMPLEMENTATION: wallet, VITE_IDO_V3_PROXY_CODE_HASH: runtime.proxyCodeHash,
  VITE_IDO_V3_IMPLEMENTATION_CODE_HASH: runtime.implementationCodeHash, VITE_IDO_V3_CONFIRMATIONS: '2' }
const context = { id: '43368658176704510', fundingMode: 'capped_presale', contractVersion: 'ido_v3_stablecoin',
  chainId: 56, contractAddress: proxy, tokenAddress: token, paymentTokenAddress: payment,
  tokenDecimals: 18, paymentTokenDecimals: 18, onchainLocked: true }
const require = createRequire(import.meta.url)
const esbuild = createRequire(require.resolve('vite/package.json'))('esbuild')
const temp = mkdtempSync(path.join(tmpdir(), 'jc-investor-preflight-'))
await esbuild.build({ stdin: { contents: `export { ContractService } from './src/services/contract.js'; export { web3Service } from './src/services/web3.js'`,
  resolveDir: fileURLToPath(new URL('..', import.meta.url)) }, bundle: true, platform: 'node', format: 'esm',
  outfile: path.join(temp, 'service.mjs'), define: { 'import.meta.env': JSON.stringify(config) }, logLevel: 'silent' })
const { ContractService, web3Service } = await import(pathToFileURL(path.join(temp, 'service.mjs')).href)
rmSync(temp, { recursive: true, force: true })
const policyAbi = new Interface(['function getWhitelistPolicy(uint256)view returns(tuple(bool enabled,bytes32 root,uint64 version))'])
const one = 10n ** 18n
const storage = new Map()

function fixture(action, options = {}) {
  const service = new ContractService()
  const counts = { fullReads: 0, proof: 0, binding: 0, signer: 0, fresh: 0, simulation: 0, token: 0, position: 0, final: 0, sends: 0 }
  const enabled = options.enabled !== false
  service._v3Signer = async (_, __, binding = true) => {
    counts.signer++; if (binding) counts.binding++
    return { context, from: options.changedWallet && counts.signer > 1 ? payment : wallet }
  }
  service.readV3State = async () => { throw Error('Investor sends must not read full state') }
  service.readV3PurchaseAuthority = async () => { throw Error('No purchase authority audits') }
  service._v3PurchaseArguments = async (_, from, amount) => {
    assert.equal(from, wallet)
    if (enabled) { counts.proof++; if (options.proofError) throw Error('PROOF') }
    return enabled ? { signature: 'purchaseWithProof(uint256,uint256,uint64,bytes32[])', args: [BigInt(context.id), amount, 1n, []] }
      : { signature: 'purchase(uint256,uint256)', args: [BigInt(context.id), amount] }
  }
  service._v3FreshProject = async () => { throw Error('No fresh backend gate') }
  service._v3Estimate = async () => { throw Error('No multi-call simulation') }
  service._v3Simulate = async () => { counts.simulation++; if (options.simulationError) throw Error('SIMULATION'); return 100000n }
  service._v3ReadRunner = () => { throw Error('No extra chain reads before wallet') }
  service._v3WalletSend = async (to, data, from) => {
    counts.sends++; assert.equal(from, wallet)
    assert.ok([...storage.values()].some(value => value.includes('dataHash')))
    if (action === 'approveV3PaymentToken') {
      assert.equal(to, payment)
      const decoded = new Interface(['function approve(address,uint256)']).decodeFunctionData('approve', data)
      assert.equal(decoded[0].toLowerCase(), proxy); assert.equal(decoded[1], one)
    } else {
      assert.equal(to, proxy)
      const signature = action === 'purchaseV3'
        ? enabled ? 'purchaseWithProof(uint256,uint256,uint64,bytes32[])' : 'purchase(uint256,uint256)'
        : action === 'claimV3' ? 'claim(uint256)' : 'refund(uint256)'
      const decoded = new Interface([`function ${signature}`]).decodeFunctionData(signature.split('(')[0], data)
      assert.equal(decoded[0], BigInt(context.id))
      if (action === 'purchaseV3') assert.equal(decoded[1], one)
    }
    throw Error('STOP_BEFORE_WALLET')
  }
  const run = () => ['purchaseV3', 'approveV3PaymentToken'].includes(action)
    ? service[action](context, '1', wallet) : service[action](context, wallet)
  return { service, counts, run }
}

test('four investor flows use only one simulation, zero audits/binding APIs, then fence the exact send', async t => {
  const oldWindow = globalThis.window, oldNavigator = Object.getOwnPropertyDescriptor(globalThis, 'navigator')
  globalThis.window = { localStorage: { getItem: k => storage.get(k) || null, setItem: (k, v) => storage.set(k, v) } }
  Object.defineProperty(globalThis, 'navigator', { configurable: true, value: { locks: { request: async (_, __, fn) => fn({}) } } })
  try {
    for (const action of ['approveV3PaymentToken', 'purchaseV3', 'claimV3', 'refundV3']) {
      await t.test(action, async () => {
        storage.clear()
        const f = fixture(action)
        await assert.rejects(f.run(), /STOP_BEFORE_WALLET/)
        assert.deepEqual(f.counts, { fullReads: 0, proof: action === 'purchaseV3' ? 1 : 0,
          binding: 0, signer: 2, fresh: 0, simulation: 1, token: 0,
          position: 0, final: 0, sends: 1 })
        await assert.rejects(f.run(), { code: 'IDO_V3_SUBMISSION_UNRESOLVED' })
        assert.equal(f.counts.sends, 1)
        // Business data is not reread when a previous send remains uncertain.
        assert.equal(f.counts.simulation, 1)
      })
      for (const options of [{ changedWallet: true }, { simulationError: true }]) {
        storage.clear()
        const f = fixture(action, options)
        await assert.rejects(f.run())
        assert.equal(f.counts.sends, 0); assert.equal(storage.size, 0)
      }
    }
    for (const options of [{ proofError: true }]) {
      storage.clear(); const f = fixture('purchaseV3', options)
      await assert.rejects(f.run()); assert.equal(f.counts.sends, 0); assert.equal(storage.size, 0)
    }
    storage.clear()
    const publicSale = fixture('purchaseV3', { enabled: false })
    await assert.rejects(publicSale.run(), /STOP_BEFORE_WALLET/)
    assert.equal(publicSale.counts.proof, 0)
    // Do not mock the simulator here: observe the actual HTTP JSON-RPC payload
    // through to the durable send boundary, without broadcasting anything.
    const originalFetch = globalThis.fetch
    try {
      for (const action of ['approveV3PaymentToken', 'purchaseV3', 'claimV3', 'refundV3']) {
        storage.clear()
        const requests = []
        globalThis.fetch = async (url, options) => {
          assert.equal(url, 'https://bsc-dataseed.bnbchain.org')
          requests.push(JSON.parse(options.body))
          return { ok: true, json: async () => ({ id: 1, result: '0x186a0' }) }
        }
        const f = fixture(action)
        delete f.service._v3Simulate
        await assert.rejects(f.run(), /STOP_BEFORE_WALLET/)
        assert.equal(requests.length, 1)
        assert.equal(requests[0].method, 'eth_estimateGas')
        assert.equal(requests[0].params[0].from, wallet)
        assert.equal(requests[0].params[0].to, action === 'approveV3PaymentToken' ? payment : proxy)
        assert.equal(requests[0].params[0].value, '0x0')
      }
    } finally { globalThis.fetch = originalFetch }
    storage.clear()
    const acknowledged = fixture('approveV3PaymentToken')
    let finishConfirmation
    acknowledged.service._v3WalletSend = async () => hash('f')
    acknowledged.service._v3WaitSubmission = () => new Promise(resolve => { finishConfirmation = resolve })
    const result = await acknowledged.run()
    assert.equal(result.status, 'submitted')
    assert.equal(result.hash, hash('f'))
    assert.equal(typeof finishConfirmation, 'function')
    assert.ok([...storage.values()].some(value => value.includes(hash('f'))))
    await assert.rejects(acknowledged.run(), { code: 'IDO_V3_SUBMISSION_UNRESOLVED' })
    finishConfirmation({ hash: hash('f'), status: 'success', blockNumber: 10 })
    assert.equal((await result.confirmation).status, 'success')
    storage.clear()
    const wrongTarget = fixture('claimV3')
    await assert.rejects(wrongTarget.service._v3Submit(context, token, '0x', wallet, 'claim(uint256)'), { code: 'IDO_V3_IDENTITY_UNAVAILABLE' })
    assert.equal(wrongTarget.counts.simulation, 0)
    globalThis.navigator.locks.request = async (_, __, fn) => fn(null)
    await assert.rejects(wrongTarget.run(), { code: 'IDO_V3_SUBMISSION_UNRESOLVED' })
    assert.equal(wrongTarget.counts.fullReads, 0)
  } finally {
    globalThis.window = oldWindow
    if (oldNavigator) Object.defineProperty(globalThis, 'navigator', oldNavigator)
    else delete globalThis.navigator
  }
})

test('final identity fence detects upgrades, changed policy and reorg without another full snapshot', async () => {
  const baseline = { blockNumber: 10, blockHash: hash('a'), enabled: true, root: hash('b'), version: '1' }
  function rpc(options = {}) {
    return { getBlock: async tag => tag === 'latest' ? { number: 11, hash: hash('c') }
      : { hash: tag === 10 ? options.reorg ? hash('d') : hash('a') : options.finalReorg ? hash('d') : hash('c') },
    getCode: async () => options.badCode ? '0x6001' : code,
    getStorage: async () => '0x' + '0'.repeat(24) + (options.upgrade ? token : wallet).slice(2),
    call: async tx => {
      assert.equal(tx.blockTag, 11)
      return policyAbi.encodeFunctionResult('getWhitelistPolicy', [[true, options.policy ? hash('d') : hash('b'), 1]])
    } }
  }
  await assertV3SendIdentity(context, baseline, rpc(), true, runtime)
  for (const options of [{ upgrade: true }, { badCode: true }, { reorg: true }, { finalReorg: true }, { policy: true }]) {
    await assert.rejects(assertV3SendIdentity(context, baseline, rpc(options), true, runtime))
  }
  const noProofReads = rpc({ policy: true })
  noProofReads.call = async () => { throw Error('claim/refund must not read whitelist') }
  await assertV3SendIdentity(context, baseline, noProofReads, false, runtime)
})

test('purchase calldata preparation reads only policy and never audits runtime or calls proof for public sale', async () => {
  const s = new ContractService()
  const requests = []
  s._v3ReadRunner = () => ({ call: async tx => {
    requests.push(tx)
    assert.equal(tx.to.toLowerCase(), proxy)
    assert.equal(policyAbi.parseTransaction({ data: tx.data }).name, 'getWhitelistPolicy')
    return policyAbi.encodeFunctionResult('getWhitelistPolicy', [[false, hash('0'), 0]])
  } })
  const result = await s._v3PurchaseArguments(context, wallet, one)
  assert.deepEqual(result, { signature: 'purchase(uint256,uint256)', args: [BigInt(context.id), one] })
  assert.equal(requests.length, 1)
})

test('investor button uses local minimum policy without display RPC gates', () => {
  const s = readFileSync(new URL('../src/components/LaunchpadActiveFundingPanel.vue', import.meta.url), 'utf8')
  const amount = s.slice(s.indexOf('const amountIsValid'), s.indexOf('const belowMinPurchase'))
  assert.match(amount, /if \(isV3.value\) return v3CanPurchase.value && raw > 0n && meetsMinPurchase\(raw, purchasePolicy.value\)/)
  assert.doesNotMatch(s, /v-else-if="isV3 && !v3WalletReady"/)
  assert.match(s, /v-else-if="belowMinPurchase"/)
})

test('final signer check is read-only and blocks changed wallet/chain without another binding request', async () => {
  const original = { ensure: web3Service.ensureReadOnlySigner, signer: web3Service.signer, chain: web3Service._signerChainId }
  try {
    web3Service.ensureReadOnlySigner = async () => true
    web3Service.signer = { getAddress: async () => wallet }
    web3Service._signerChainId = 56
    const service = new ContractService()
    assert.equal((await service._v3Signer(context, wallet, false)).from, wallet)
    web3Service._signerChainId = 97
    await assert.rejects(service._v3Signer(context, wallet, false), { code: 'NETWORK_MISMATCH' })
    web3Service._signerChainId = 56
    web3Service.signer = { getAddress: async () => payment }
    await assert.rejects(service._v3Signer(context, wallet, false), { code: 'IDO_V3_WALLET_CHANGED' })
  } finally {
    web3Service.ensureReadOnlySigner = original.ensure; web3Service.signer = original.signer; web3Service._signerChainId = original.chain
  }
  const source = readFileSync(new URL('../src/services/contract.js', import.meta.url), 'utf8')
  const signer = source.slice(source.indexOf('async _v3Signer'), source.indexOf('async _v3FreshProject'))
  assert.doesNotMatch(signer.replace(/\/\/[^\n]*/g, ''), /alignSessionForSend|eth_requestAccounts|_ensureProjectSigner/)
  assert.equal(signer.match(/await getVerifiedV3Wallet\(/g).length, 1)
})

test('read-only session never prompts, rebuilds on switches and drops disconnected sessions', async () => {
  const oldProvider = web3Service._getEIP1193Provider
  let account = wallet, chain = '0x38'
  const methods = []
  const provider = { request: async ({ method }) => {
    methods.push(method)
    if (method === 'eth_accounts') return account ? [account] : []
    if (method === 'eth_chainId') return chain
    throw Error('Unexpected interactive request')
  } }
  web3Service._getEIP1193Provider = () => provider
  try {
    assert.equal(await web3Service.ensureReadOnlySigner(), true)
    assert.equal((await web3Service.signer.getAddress()).toLowerCase(), wallet)
    account = payment; chain = '0x61'
    assert.equal(await web3Service.ensureReadOnlySigner(), true)
    assert.equal((await web3Service.signer.getAddress()).toLowerCase(), payment)
    assert.equal(web3Service._signerChainId, 97)
    account = null
    assert.equal(await web3Service.ensureReadOnlySigner(), false)
    assert.equal(web3Service.signer, null)
    assert.deepEqual([...new Set(methods)].sort(), ['eth_accounts', 'eth_chainId'])
  } finally { web3Service._getEIP1193Provider = oldProvider; web3Service._clearSession() }
})

test('simulation retries only transport availability on a second trusted RPC, never a contract revert', async () => {
  const oldFetch=globalThis.fetch
  const s=new ContractService(),requests=[]
  try {
    globalThis.fetch=async(url,options)=>{
      requests.push({url,body:JSON.parse(options.body)})
      if(requests.length===1)throw Error('transport offline')
      return {ok:true,json:async()=>({id:1,result:'0x186a0'})}
    }
    assert.equal(await s._v3Simulate(context,proxy,'0x12345678',wallet),125000n)
    assert.deepEqual(requests.map(r=>r.url),['https://bsc-dataseed.bnbchain.org','https://bsc-dataseed1.bnbchain.org'])
    assert.deepEqual(requests[0].body,requests[1].body)
    requests.length=0
    globalThis.fetch=async url=>{requests.push({url});return {ok:true,json:async()=>({id:1,error:{code:3,message:'execution reverted'}})}}
    await assert.rejects(s._v3Simulate(context,proxy,'0x12345678',wallet),{code:'IDO_V3_STATE_UNAVAILABLE'})
    assert.equal(requests.length,1)
  } finally {globalThis.fetch=oldFetch}
})
