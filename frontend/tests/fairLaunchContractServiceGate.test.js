import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { ContractService } from '../src/services/contract.js'

const trustedFairProject = {
  id: 7,
  fundingMode: 'fair_launch',
  contractVersion: 'fair_launch_v1',
  chainId: 56,
  contractAddress: '0x2222222222222222222222222222222222222222',
  paymentTokenAddress: '0x6666666666666666666666666666666666666666',
  paymentTokenDecimals: 18,
  tokenAddress: '0x7777777777777777777777777777777777777777',
  tokenDecimals: 18,
  projectOwner: '0x1111111111111111111111111111111111111111',
  projectProceedsReceiver: '0x8888888888888888888888888888888888888888',
  chainStatus: 'successful',
  finality: 'finalized',
  reconciliationState: 'BALANCED',
  stateSource: 'canonical_chain_event',
  runtimeIdentity: {
    chainId: 56,
    proxyAddress: '0x2222222222222222222222222222222222222222',
    implementationAddress: '0x3333333333333333333333333333333333333333',
    implementationCodeHash: `0x${'a'.repeat(64)}`,
    sourceRegistryRevision: 1,
    sourceRegistryDigest: `0x${'b'.repeat(64)}`,
    observedBlockNumber: '1',
    observedBlockHash: `0x${'c'.repeat(64)}`,
    observedAt: '2026-09-07T00:00:00Z',
    runtimeVerified: true,
  },
  actions: Object.fromEntries([
    'contribute', 'claim', 'refund', 'depositProjectTokens', 'withdrawFailedProjectTokens',
    'withdrawProjectProceeds', 'withdrawPlatformFee',
  ].map(action => [action, { available: true, reason: '' }])),
}

const genericMethodNames = [
  ['approvePaymentToken', ['1']], ['purchase', ['1']], ['claim', []], ['refund', []],
  ['approveProjectToken', ['1']], ['depositProjectTokens', ['1']],
  ['withdrawRaisedFunds', []], ['withdrawProjectTokens', []],
]

function assertDurableUnavailable(error) {
  assert.equal(error?.code, 'DURABLE_WALLET_SUBMISSION_UNAVAILABLE')
  assert.equal(error?.reason, 'durable_wallet_submission_unavailable')
  assert.equal(error?.message, 'durable_wallet_submission_unavailable')
  return true
}

function walletTouchSentinel(service) {
  let touched = 0
  const touch = () => { touched += 1; throw new Error('wallet boundary touched') }
  for (const name of [
    '_ensureProjectSigner', '_getERC20', '_getERC20Signer', '_getProjectContract',
    '_preflightFairLaunch', '_estimateGasLimit', '_estimateFairLaunchGasLimit', '_dualChannelSend',
  ]) service[name] = touch
  return () => touched
}

test('every generic write rejects exact Fair Launch before any wallet or contract access', async () => {
  const service = new ContractService()
  const touched = walletTouchSentinel(service)
  for (const [method, extra] of genericMethodNames) {
    await assert.rejects(Promise.resolve().then(() => service[method](trustedFairProject, ...extra)), assertDurableUnavailable, method)
  }
  assert.equal(touched(), 0)
})

test('default release guard rejects forged enabled Fair actions and both approval flows before every wallet seam', async () => {
  const service = new ContractService()
  const touched = walletTouchSentinel(service)
  const calls = [
    ['approveFairLaunchPaymentToken', [trustedFairProject, '100']],
    ['contributeFairLaunch', [trustedFairProject, '100']],
    ['claimFairLaunch', [trustedFairProject]],
    ['claimFairLaunchFor', [trustedFairProject, '0x1111111111111111111111111111111111111111']],
    ['refundFairLaunch', [trustedFairProject]],
    ['refundFairLaunchFor', [trustedFairProject, '0x1111111111111111111111111111111111111111']],
    ['approveFairLaunchProjectToken', [trustedFairProject]],
    ['depositFairLaunchProjectTokens', [trustedFairProject]],
    ['withdrawFairLaunchProjectProceeds', [trustedFairProject]],
    ['withdrawFairLaunchPlatformFee', [trustedFairProject]],
    ['withdrawFairLaunchFailedProjectTokens', [trustedFairProject]],
    ['_approveExact', [trustedFairProject, trustedFairProject.paymentTokenAddress, '0', 'contribute']],
    ['_approveExact', [trustedFairProject, trustedFairProject.paymentTokenAddress, '100', 'contribute']],
  ]
  for (const [method, args] of calls) {
    await assert.rejects(Promise.resolve().then(() => service[method](...args)), assertDurableUnavailable, method)
  }
  assert.equal(touched(), 0)
})

test('admitted local policy opens only the dedicated path and still reaches signer defenses', async () => {
  const localProject = {
    ...trustedFairProject,
    chainId: 31337,
    runtimeIdentity: { ...trustedFairProject.runtimeIdentity, chainId: 31337 },
  }
  const service = new ContractService({ fairLaunchSubmissionPolicy: () => ({ enabled: true, mode: 'local_hardhat_ephemeral' }) })
  let reached = 0
  service._ensureProjectSigner = async () => { reached += 1; throw new Error('LOCAL_SIGNER_SENTINEL') }
  await assert.rejects(service.contributeFairLaunch(localProject, '100'), error => error?.message === 'LOCAL_SIGNER_SENTINEL')
  assert.equal(reached, 1)

  const genericService = new ContractService({ fairLaunchSubmissionPolicy: () => ({ enabled: true }) })
  const touched = walletTouchSentinel(genericService)
  await assert.rejects(genericService.purchase(localProject, '1'), assertDurableUnavailable)
  assert.equal(touched(), 0)
})

test('direct exact approval retains local policy gate before signer access', async () => {
  const service = new ContractService({ fairLaunchSubmissionPolicy: () => ({ enabled: false, reason: 'durable_wallet_submission_unavailable' }) })
  const touched = walletTouchSentinel(service)
  await assert.rejects(service._approveExact(trustedFairProject, trustedFairProject.paymentTokenAddress, '100', 'contribute'), assertDurableUnavailable)
  assert.equal(touched(), 0)
})

test('local deployment preflight uses the manifest-bound canonical RPC rather than optional platform settings state', async () => {
  const source = await readFile(new URL('../src/services/contract.js', import.meta.url), 'utf8')
  const method = source.slice(source.indexOf('async _verifyFairLaunchLocalDeployment'), source.indexOf('async _preflightFairLaunch'))
  assert.match(method, /new JsonRpcProvider\(policy\.rpcUrl\)/)
  assert.match(method, /policy\.rpcUrl,\s*\)/)
  assert.doesNotMatch(method, /__JC_ACTIVE_RPC_URL/)
  assert.doesNotMatch(method, /verifyFairLaunchLocalDeployment\(\s*web3Service\.provider/)
})

test('valid capped-presale V1 and V2 generic methods still reach the pre-existing signer seam', async () => {
  const projects = [
    { id: 1, fundingMode: 'capped_presale', contractVersion: 'v1_legacy' },
    { id: 2, fundingMode: 'capped_presale', contractVersion: 'v2_stablecoin' },
  ]
  for (const project of projects) {
    for (const [method, extra] of genericMethodNames) {
      const service = new ContractService()
      let reached = 0
      service._ensureProjectSigner = async () => { reached += 1; throw new Error('LEGACY_SIGNER_SENTINEL') }
      await assert.rejects(service[method](project, ...extra), error => error?.message === 'LEGACY_SIGNER_SENTINEL', `${project.contractVersion}:${method}`)
      assert.equal(reached, 1, `${project.contractVersion}:${method}`)
    }
  }
})

test('Backend-shaped action fixture enumerates the complete dedicated wallet surface as disabled', () => {
  const actions = Object.fromEntries([
    'contribute', 'claim', 'refund', 'depositProjectTokens', 'withdrawFailedProjectTokens',
    'withdrawProjectProceeds', 'withdrawPlatformFee',
  ].map(action => [action, { available: false, reason: 'durable_wallet_submission_unavailable' }]))
  actions.release = { available: false, reason: 'operator_only' }
  assert.deepEqual(Object.keys(actions).sort(), [
    'claim', 'contribute', 'depositProjectTokens', 'refund', 'release',
    'withdrawFailedProjectTokens', 'withdrawPlatformFee', 'withdrawProjectProceeds',
  ])
})
