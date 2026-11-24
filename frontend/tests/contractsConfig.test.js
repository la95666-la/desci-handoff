import assert from 'node:assert/strict'
import test from 'node:test'

import {
  LOGIN_NETWORK_POLICY,
  CHECKIN_CHAIN,
} from '../src/config/contracts.js'
import {
  isNetworkMismatchError,
  WALLET_ERROR_CODES,
} from '../src/services/errors.js'
import { contractService } from '../src/services/contract.js'
import { web3Service } from '../src/services/web3.js'

// node --test 環境沒有任何 VITE_* 注入 ⇒ 完全未配置 ⇒ 必須 fail-closed 到主網
test('unconfigured build fails closed: login policy trusts BSC mainnet only', () => {
  assert.deepEqual(LOGIN_NETWORK_POLICY.trustedChainIds, [56])
  assert.equal(LOGIN_NETWORK_POLICY.preferredChainId, 56)
  assert.equal(LOGIN_NETWORK_POLICY.autoSwitch, true)
})

test('unconfigured build keeps check-in chain fail-closed on mainnet', () => {
  assert.equal(CHECKIN_CHAIN.chainId, 56)
})

test('error codes distinguish network mismatch from other wallet errors', () => {
  const mismatch = new Error('Wrong network: ...')
  mismatch.code = WALLET_ERROR_CODES.NETWORK_MISMATCH
  assert.equal(isNetworkMismatchError(mismatch), true)

  const rpc = new Error('boom')
  rpc.code = WALLET_ERROR_CODES.RPC_ERROR
  assert.equal(isNetworkMismatchError(rpc), false)
  assert.equal(isNetworkMismatchError(new Error('plain')), false)
})

const v2Project = {
  id: 9,
  contractVersion: 'v2_stablecoin',
  chainId: 97,
  contractAddress: '0x3333333333333333333333333333333333333333',
  paymentTokenAddress: '0x4444444444444444444444444444444444444444',
  paymentTokenDecimals: 6,
  tokenAddress: '0x5555555555555555555555555555555555555555',
  tokenDecimals: 18,
  paymentToken: 'USDT',
}

test('V3 wallet and ABI entry points fail closed before signer or legacy address fallback', async () => {
  const v3Project = { ...v2Project, fundingMode: 'capped_presale', contractVersion: 'ido_v3_stablecoin', id: '41376991395446785' }
  assert.throws(() => contractService._getProjectContract(v3Project), (error) => error.code === 'IDO_V3_ROUTING_UNAVAILABLE')
  assert.throws(() => contractService._getProjectContract(v3Project, true), (error) => error.code === 'IDO_V3_ROUTING_UNAVAILABLE')
  await assert.rejects(() => contractService.purchase(v3Project, '1'), (error) => error.code === 'IDO_V3_ROUTING_UNAVAILABLE')
  await assert.rejects(() => contractService.approvePaymentToken(v3Project, '1'), (error) => error.code === 'IDO_V3_ROUTING_UNAVAILABLE')
  await assert.rejects(() => contractService.claim(v3Project), (error) => error.code === 'IDO_V3_ROUTING_UNAVAILABLE')
  await assert.rejects(() => contractService.refund(v3Project), (error) => error.code === 'IDO_V3_ROUTING_UNAVAILABLE')
})

test('project write guard stays fail-closed: chain mismatch blocks before signing', async () => {
  const origEnsure = web3Service.ensureSigner
  const origProvider = web3Service.provider
  web3Service.ensureSigner = async () => true
  web3Service.provider = { getNetwork: async () => ({ chainId: 31337n }) }
  try {
    await assert.rejects(
      () => contractService._ensureProjectSigner(v2Project),
      (err) =>
        err.code === WALLET_ERROR_CODES.NETWORK_MISMATCH &&
        err.expectedChainId === 97 &&
        err.connectedChainId === 31337 &&
        /Wrong network/.test(err.message)
    )
  } finally {
    web3Service.ensureSigner = origEnsure
    web3Service.provider = origProvider
  }
})

const project31337 = {
  id: 3,
  contractVersion: 'v2_stablecoin',
  chainId: 31337,
  contractAddress: '0x3333333333333333333333333333333333333333',
  paymentTokenAddress: '0x4444444444444444444444444444444444444444',
  paymentTokenDecimals: 6,
  tokenAddress: '0x5555555555555555555555555555555555555555',
  tokenDecimals: 18,
  paymentToken: 'USDT',
}

// 用戶場景（project_id=3 active on 31337）：現有登入態下錢包 97->31337 之後，
// 對綁定 31337 的項目寫入必須直接通過 fail-closed 守門，不需要切鏈/重置
test('write for a 31337-bound project passes when the rebuilt session is on 31337', async () => {
  const origEnsure = web3Service.ensureSigner
  const origProvider = web3Service.provider
  web3Service.ensureSigner = async () => true
  web3Service.provider = { getNetwork: async () => ({ chainId: 31337n }) }
  try {
    const context = await contractService._ensureProjectSigner(project31337)
    assert.equal(context.chainId, 31337)
  } finally {
    web3Service.ensureSigner = origEnsure
    web3Service.provider = origProvider
  }
})

test('project write guard passes when the wallet is on the project chain', async () => {
  const origEnsure = web3Service.ensureSigner
  const origProvider = web3Service.provider
  web3Service.ensureSigner = async () => true
  web3Service.provider = { getNetwork: async () => ({ chainId: 97n }) }
  try {
    const context = await contractService._ensureProjectSigner(v2Project)
    assert.equal(context.chainId, 97)
    assert.equal(context.contractAddress, v2Project.contractAddress)
  } finally {
    web3Service.ensureSigner = origEnsure
    web3Service.provider = origProvider
  }
})

test('runtime settings reject a non-loopback RPC for Hardhat 31337', () => {
  const before = ACTIVE_CHAIN.chainId
  const result = applyPlatformSettings({
    ProjectChain: { ChainID: '31337', ChainUrl: 'http://192.168.1.20:8545', ContractAddress: '0x1111111111111111111111111111111111111111' },
  })
  assert.equal(result, before)
  assert.equal(ACTIVE_CHAIN.chainId, before)
})

test('project write guard reports signer-unavailable distinctly', async () => {
  const origEnsure = web3Service.ensureSigner
  web3Service.ensureSigner = async () => false
  try {
    await assert.rejects(
      () => contractService._ensureProjectSigner(v2Project),
      (err) => err.code === WALLET_ERROR_CODES.SIGNER_UNAVAILABLE
    )
  } finally {
    web3Service.ensureSigner = origEnsure
  }
})

// ============ 雙鏈 settings 拆分（task 鏈與 project 鏈獨立消費）============

import {
  ACTIVE_CHAIN,
  ADDRESSES,
  applyPlatformSettings,
} from '../src/config/contracts.js'

function snapshotChainState() {
  return {
    active: JSON.parse(JSON.stringify(ACTIVE_CHAIN)),
    checkin: JSON.parse(JSON.stringify(CHECKIN_CHAIN)),
    joinCare: ADDRESSES.JoinCare,
    checkIn: ADDRESSES.CheckIn,
  }
}

function restoreChainState(snap) {
  Object.assign(ACTIVE_CHAIN, snap.active)
  Object.assign(CHECKIN_CHAIN, snap.checkin)
  ADDRESSES.JoinCare = snap.joinCare
  ADDRESSES.CheckIn = snap.checkIn
}

const MAINNET_CHECKIN = '0xE029161be55922edf3eC9D222142edf057d196ee'

// 測試服務器雙鏈形態：TaskChain=56（主網 CheckIn）、ProjectChain=97（測試網 IDO）。
// 關鍵不變量：CheckIn 地址落在 56，ACTIVE_CHAIN（Launchpad）保持 97，
// 兩者絕不互相污染 —— 這正是舊扁平字段的核心缺陷。
test('dual-chain settings: task 56 / project 97 keep each chain scoped', () => {
  const snap = snapshotChainState()
  try {
    const returned = applyPlatformSettings({
      ChainEnvironment: 'test',
      TaskChain: {
        Enabled: true,
        ChainID: '56',
        ChainUrl: 'https://bsc-dataseed.bnbchain.org',
        CheckinAddress: MAINNET_CHECKIN,
      },
      ProjectChain: {
        Enabled: true,
        ChainID: '97',
        ChainUrl: 'https://data-seed-prebsc-1-s1.binance.org:8545',
        ContractAddress: '0xe6F01eC4dEA815c9dAAb79d7A7D0FA16b2cB368A',
      },
      // 舊扁平字段故意留空，證明前端不再依賴它們拼接雙鏈
    })
    assert.equal(returned, 97)
    assert.equal(ACTIVE_CHAIN.chainId, 97)
    assert.equal(ACTIVE_CHAIN.name, 'BNB Smart Chain Testnet')
    assert.equal(ADDRESSES.JoinCare, '0xe6F01eC4dEA815c9dAAb79d7A7D0FA16b2cB368A')
    assert.equal(CHECKIN_CHAIN.chainId, 56)
    assert.equal(CHECKIN_CHAIN.name, 'BNB Smart Chain')
    assert.equal(ADDRESSES.CheckIn, MAINNET_CHECKIN)
  } finally {
    restoreChainState(snap)
  }
})

// 未受信的任務鏈（本構建 fail-closed 只信任 56）：地址絕不落地，
// CHECKIN_CHAIN 保持構建期默認 —— 防禦後端配置漂移/緩存投毒。
test('untrusted backend task chain is rejected entirely', () => {
  const snap = snapshotChainState()
  try {
    applyPlatformSettings({
      TaskChain: {
        Enabled: true,
        ChainID: '43114',
        ChainUrl: 'https://evil-rpc.example',
        CheckinAddress: '0x9999999999999999999999999999999999999999',
      },
      ChainID: '56',
    })
    assert.equal(ADDRESSES.CheckIn, snap.checkIn)
    assert.equal(CHECKIN_CHAIN.chainId, snap.checkin.chainId)
    assert.notEqual(CHECKIN_CHAIN.rpcUrl, 'https://evil-rpc.example')
  } finally {
    restoreChainState(snap)
  }
})

// 舊後端（無 TaskChain/ProjectChain 結構）：扁平字段行為完全保持
test('legacy flat settings keep working (backward compat)', () => {
  const snap = snapshotChainState()
  try {
    const returned = applyPlatformSettings({
      ChainEnvironment: 'test',
      ChainID: '97',
      ChainUrl: 'https://data-seed-prebsc-1-s1.binance.org:8545',
      ContractAddress: '0xe6F01eC4dEA815c9dAAb79d7A7D0FA16b2cB368A',
      CheckinAddress: MAINNET_CHECKIN,
      JccAddress: '0xFc6C84ac73c11f6C162B9dd5B4A94e7bfb3308EA',
    })
    assert.equal(returned, 97)
    assert.equal(ACTIVE_CHAIN.chainId, 97)
    assert.equal(ADDRESSES.JoinCare, '0xe6F01eC4dEA815c9dAAb79d7A7D0FA16b2cB368A')
    assert.equal(ADDRESSES.CheckIn, MAINNET_CHECKIN)
  } finally {
    restoreChainState(snap)
  }
})

// 主網構建（node --test 無 VITE_ 注入 ⇒ fail-closed 主網）下，
// 後端下發 56 任務鏈 + 主網 CheckIn 地址必須正常落地
test('mainnet build applies trusted mainnet task chain', () => {
  const snap = snapshotChainState()
  try {
    applyPlatformSettings({
      TaskChain: {
        Enabled: true,
        ChainID: '56',
        ChainUrl: 'https://bsc-dataseed.bnbchain.org',
        CheckinAddress: MAINNET_CHECKIN,
      },
      ProjectChain: { Enabled: true, ChainID: '56', ChainUrl: '', ContractAddress: '0xabc' },
    })
    assert.equal(CHECKIN_CHAIN.chainId, 56)
    assert.equal(ADDRESSES.CheckIn, MAINNET_CHECKIN)
    // 主網構建 rpcUrl 故意為空（走錢包內置鏈），後端 URL 只填充不覆蓋
    // 構建期注入的顯式 RPC
  } finally {
    restoreChainState(snap)
  }
})
