import assert from 'node:assert/strict'
import test from 'node:test'

import { Web3Service } from '../src/services/web3.js'

const ACCOUNT_A = '0x1111111111111111111111111111111111111111'
const ACCOUNT_B = '0x2222222222222222222222222222222222222222'

/**
 * 迷你 EIP-1193 provider：只有測試需要的 eth_chainId / eth_accounts。
 * state 可以在測試中途改動，模擬錢包切鏈 / 換賬戶。
 */
function createFakeProvider({ chainId = 97, accounts = [ACCOUNT_A] } = {}) {
  const state = { chainId, accounts, switches: [] }
  return {
    state,
    setChainId(c) { state.chainId = c },
    setAccounts(a) { state.accounts = a },
    async request({ method, params }) {
      switch (method) {
        case 'eth_chainId': return '0x' + state.chainId.toString(16)
        case 'eth_accounts': return [...state.accounts]
        case 'wallet_switchEthereumChain': {
          state.switches.push(params[0])
          state.chainId = Number.parseInt(params[0].chainId, 16)
          return null
        }
        default: throw new Error('unexpected method ' + method)
      }
    },
    on() {},
    off() {},
    removeListener() {},
    removeAllListeners() {},
  }
}

function makeService(fake) {
  const srv = new Web3Service()
  srv._getEIP1193Provider = () => fake
  return srv
}

const tick = () => new Promise((r) => setTimeout(r, 0))

test('ensureSigner builds a session and reuses it while chain/account are unchanged', async () => {
  const fake = createFakeProvider()
  const srv = makeService(fake)

  assert.equal(await srv.ensureSigner(), true)
  assert.equal(srv.chainId, 97)
  const signer1 = srv.signer
  const provider1 = srv.provider
  assert.ok(signer1)
  assert.ok(provider1)

  // 相同鏈 + 相同賬戶 → 直接復用，絕不重建
  assert.equal(await srv.ensureSigner(), true)
  assert.equal(srv.signer, signer1)
  assert.equal(srv.provider, provider1)
})

test('ensureSigner rebuilds (never reuses a stale signer) after the wallet chain changes', async () => {
  const fake = createFakeProvider()
  const srv = makeService(fake)

  await srv.ensureSigner()
  const signer1 = srv.signer
  const provider1 = srv.provider
  assert.equal(srv.chainId, 97)

  // 錢包在外部切到 Hardhat(31337)
  fake.setChainId(31337)
  assert.equal(await srv.ensureSigner(), true)

  assert.equal(srv.chainId, 31337)
  assert.notEqual(srv.signer, signer1)
  assert.notEqual(srv.provider, provider1)
  assert.equal(srv._signerChainId, 31337)
})

test('handleChainChanged clears and rebuilds the ethers session on the new chain', async () => {
  const fake = createFakeProvider()
  const srv = makeService(fake)

  await srv.ensureSigner()
  const staleSigner = srv.signer
  assert.equal(srv.chainId, 97)

  // 真實錢包：先完成切鏈（underlying state 已變），再廣播 chainChanged 事件
  fake.setChainId(31337)

  // 同步：更新 chainId + 清理緩存 session
  const parsed = srv.handleChainChanged('0x7a69')
  assert.equal(parsed, 31337)
  assert.equal(srv.chainId, 31337)
  assert.equal(srv.signer, null)
  assert.equal(srv.provider, null)
  assert.notEqual(staleSigner, srv.signer)

  // 等異步重建完成（失敗也無妨，下面 ensureSigner 會兜底）
  await tick()
  assert.equal(await srv.ensureSigner(), true)
  assert.equal(srv.chainId, 31337)
  assert.ok(srv.signer)
  assert.ok(srv.provider)
})

test('handleChainChanged with an unparsable chainId clears the session defensively', async () => {
  const fake = createFakeProvider()
  const srv = makeService(fake)

  await srv.ensureSigner()
  assert.equal(srv.handleChainChanged('not-a-chain'), null)
  assert.equal(srv.signer, null)
  assert.equal(srv.provider, null)
})

test('ensureSigner rebuilds when the wallet switches accounts', async () => {
  const fake = createFakeProvider()
  const srv = makeService(fake)

  await srv.ensureSigner()
  const signer1 = srv.signer
  fake.setAccounts([ACCOUNT_B])

  await srv.ensureSigner()
  assert.notEqual(srv.signer, signer1)
  assert.equal(srv._signerAccount, ACCOUNT_B)
  assert.equal(srv.chainId, 97)
})

test('ensureSigner rebuilds when the EIP-1193 provider object changes (wallet swap)', async () => {
  const fake1 = createFakeProvider({ chainId: 56 })
  const fake2 = createFakeProvider({ chainId: 97, accounts: [ACCOUNT_B] })
  const srv = new Web3Service()
  srv._getEIP1193Provider = () => fake1

  await srv.ensureSigner()
  assert.equal(srv._signerProvider, fake1)
  srv._getEIP1193Provider = () => fake2

  await srv.ensureSigner()
  assert.equal(srv._signerProvider, fake2)
  assert.equal(srv.chainId, 97)
  assert.equal(srv._signerAccount, ACCOUNT_B)
})

test('ensureSigner returns false when no account is authorized', async () => {
  const fake = createFakeProvider({ accounts: [] })
  const srv = makeService(fake)

  assert.equal(await srv.ensureSigner(), false)
  assert.equal(srv.signer, null)
  assert.equal(srv.provider, null)
})

test('existing logged-in session survives 97->31337 chainChanged without a chain reset', async () => {
  // 用戶場景：之前 session 建在 testnet(97)，後續錢包被切到 Hardhat(31337)。
  // chainChanged 到達後，下一筆寫入必須自動適配新鏈，而不是要求用戶重置錢包/重連。
  const fake = createFakeProvider({ chainId: 97 })
  const srv = makeService(fake)

  await srv.ensureSigner()
  assert.equal(srv.chainId, 97)
  const oldSigner = srv.signer
  const oldProvider = srv.provider

  // 錢包先完成切鏈（真實錢包的順序），再廣播 chainChanged
  fake.setChainId(31337)
  assert.equal(srv.handleChainChanged('0x7a69'), 31337)

  // chainChanged 同步清理了緩存 session；下一筆寫入的 ensureSigner 自動重建，
  // 綁定新鏈，整個過程中不需要 disconnect / 重置任何東西
  assert.equal(srv.provider, null)
  assert.equal(await srv.ensureSigner(), true)
  assert.equal(srv.chainId, 31337)
  assert.notEqual(srv.signer, oldSigner)
  assert.notEqual(srv.provider, oldProvider)
  assert.equal(srv._signerChainId, 31337)
  assert.equal(srv._signerAccount, ACCOUNT_A)
})

test('switchToChain switches by chainId (56 -> 0x38, 97 -> 0x61), never by a hardcoded default', async () => {
  const fake = createFakeProvider()
  const srv = makeService(fake)

  await srv.switchToChain(97)
  assert.deepEqual(fake.state.switches, [{ chainId: '0x61' }])

  // 錢包切鏈後廣播 chainChanged（生產裡 store 監聽這個事件更新 chainId + 重建 session）
  srv.handleChainChanged('0x61')
  assert.equal(srv.chainId, 97)
  await tick()
  assert.equal(await srv.ensureSigner(), true)
  assert.equal(srv.chainId, 97)

  await srv.switchToChain(56)
  assert.deepEqual(fake.state.switches, [{ chainId: '0x61' }, { chainId: '0x38' }])
})

// 回歸：本地 31337 構建不該有任何「自動切到 97」路徑 —— switchToChain 對 31337 直接拒絕，
// 與策略 autoSwitch=false 雙保險（之前 switchToBSC(bool) 只能 56/97，且調用方把非主網硬編為 97）
test('switchToChain refuses local hardhat (31337) — it is never auto-switched', async () => {
  const fake = createFakeProvider()
  const srv = makeService(fake)

  await assert.rejects(
    () => srv.switchToChain(31337),
    (err) => err.code === 'UNSUPPORTED_SWITCH_CHAIN' && /31337/.test(err.message)
  )
  assert.deepEqual(fake.state.switches, [])
})

test('disconnect clears provider, signer and chainId; ensureSigner can rebuild afterwards', async () => {
  const fake = createFakeProvider()
  const srv = makeService(fake)

  await srv.ensureSigner()
  srv.disconnect()
  assert.equal(srv.provider, null)
  assert.equal(srv.signer, null)
  assert.equal(srv.chainId, null)

  assert.equal(await srv.ensureSigner(), true)
  assert.equal(srv.chainId, 97)
})

test('external Reown connector provider is authoritative and cleared on disconnect', () => {
  const srv = new Web3Service()
  const selectedProvider = { request: async () => null }
  srv.setCurrentWalletType('okx')
  srv.setExternalEIP1193Provider(selectedProvider)
  assert.equal(srv._getEIP1193Provider(), selectedProvider)
  srv.disconnect()
  assert.equal(srv._externalEIP1193Provider, null)
})

test('personalSign uses selected connector provider without requesting a chain switch', async () => {
  const calls = []
  const selectedProvider = {
    async request(payload) {
      calls.push(payload)
      if (payload.method === 'personal_sign') return '0xsigned'
      throw new Error('unexpected method: ' + payload.method)
    },
  }
  const srv = new Web3Service()
  srv.setExternalEIP1193Provider(selectedProvider)
  const signature = await srv.personalSign('login nonce', '0x1111111111111111111111111111111111111111')
  assert.equal(signature, '0xsigned')
  assert.deepEqual(calls, [{
    method: 'personal_sign',
    params: ['0x6c6f67696e206e6f6e6365', '0x1111111111111111111111111111111111111111'],
  }])
  assert.equal(calls.some((call) => call.method === 'wallet_switchEthereumChain'), false)
})

