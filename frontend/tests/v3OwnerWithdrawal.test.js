import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { Interface } from 'ethers'
import { IDO_V3_ABI } from '../src/config/idoV3Abi.js'
import { v3ProjectActionState } from '../src/services/idoV3Wallet.js'
import { ContractService } from '../src/services/contract.js'

const owner = '0x2222222222222222222222222222222222222222'
const project = { id: '43235577608273921', fundingMode: 'capped_presale', contractVersion: 'ido_v3_stablecoin',
  chainId: 56, contractAddress: '0x1111111111111111111111111111111111111111',
  tokenAddress: '0x3333333333333333333333333333333333333333', paymentTokenAddress: '0x4444444444444444444444444444444444444444',
  tokenDecimals: 18, paymentTokenDecimals: 18, onchainLocked: true }
const runtime = { chainId: 56, proxy: project.contractAddress }
const action = 'withdrawProjectTokens'
const state = (status, patch = {}) => ({ status, project: { projectOwner: owner,
  projectTokensWithdrawn: false, totalTokensDeposited: 1000000n, totalTokensSold: 100n, ...patch } })

test('V3 owner withdrawal requires actual chain terminal status, owner, remaining tokens and not withdrawn', () => {
  for (const status of [2, 3, 4]) assert.equal(v3ProjectActionState(project, state(status), action, owner, runtime).status, status)
  for (const status of [0, 1, 5]) {
    assert.throws(() => v3ProjectActionState(project, state(status), action, owner, runtime), { code: 'IDO_V3_ACTION_UNAVAILABLE' })
  }
  for (const patch of [{ projectTokensWithdrawn: true }, { projectTokensWithdrawn: undefined },
    { projectOwner: project.tokenAddress }, { totalTokensDeposited: 0n }]) {
    assert.throws(() => v3ProjectActionState(project, state(3, patch), action, owner, runtime), { code: 'IDO_V3_ACTION_UNAVAILABLE' })
  }
  assert.throws(() => v3ProjectActionState(project, state(2, { totalTokensSold: 1000000n }), action, owner, runtime), { code: 'IDO_V3_ACTION_UNAVAILABLE' })
  assert.throws(() => v3ProjectActionState(project, state(2, { totalTokensSold: 1000001n }), action, owner, runtime), { code: 'IDO_V3_ACTION_UNAVAILABLE' })
  // Failed projects return deposits irrespective of historical purchase allocation.
  assert.equal(v3ProjectActionState(project, state(3, { totalTokensSold: 1000001n }), action, owner, runtime).status, 3)
})

test('V3 owner withdrawal encodes exact project ID and only enters dedicated guarded action', async () => {
  const service = new ContractService()
  service._v3Context = () => project
  let captured
  service._v3Action = async (...args) => { captured = args; return { hash: 'test' } }
  assert.deepEqual(await service.withdrawV3ProjectTokens(project, owner), { hash: 'test' })
  assert.deepEqual(captured, [project, action, owner, 'withdrawProjectTokens(uint256)', [43235577608273921n]])
  const abi = new Interface(IDO_V3_ABI)
  assert.equal(abi.decodeFunctionData(action, abi.encodeFunctionData(action, captured[4]))[0], 43235577608273921n)
  let legacy = false
  service._ensureProjectSigner = async () => { legacy = true }
  await assert.rejects(service.withdrawProjectTokens(project), { code: 'IDO_V3_ROUTING_UNAVAILABLE' })
  assert.equal(legacy, false)
})

test('withdrawal joins operation-local preflight and shared durable submission fence', () => {
  const source = readFileSync(new URL('../src/services/contract.js', import.meta.url), 'utf8')
  const submit = source.slice(source.indexOf('async _v3Submit'), source.indexOf('async _v3WalletSend'))
  assert.match(submit, /'withdrawProjectTokens\(uint256\)'\]\.includes\(action\)/)
  assert.match(submit, /await this\._v3OwnerPreflight\(context, to, data, from, action\)/)
  const preflight = source.slice(source.indexOf('async _v3OwnerPreflight'), source.indexOf('async _v3Estimate'))
  assert.match(preflight, /this\._v3Estimate\(context, to, data, from\)/)
  assert.match(preflight, /this\._v3FreshProject\(context\)/)
  assert.equal(preflight.match(/await this\.readV3State\(context\)/g)?.length, 2)
  assert.equal(preflight.match(/await this\._v3OwnerSnapshotChecks\(/g)?.length, 2)
  assert.match(preflight, /withdrawal \? 'withdrawProjectTokens' : 'depositProjectTokens'/)
  assert.match(submit, /navigator\.locks\.request/)
  assert.match(submit, /recordV3Pending\(identity, '', \{ \.\.\.v3PendingMetadata/)
  assert.match(submit, /if \(!isSuccessfulReceipt\(receipt\)\) requireSuccessfulReceipt/)
})

test('Dashboard routes V3 withdrawal correctly and does not pretend modal OK submits again', () => {
  const source = readFileSync(new URL('../src/views/Dashboard.vue', import.meta.url), 'utf8')
  const handler = source.slice(source.indexOf('async function handleWithdrawTokens'), source.indexOf('// ============ 狀態文案'))
  assert.match(handler, /isWithdrawingTokens\[projectId\] \|\| isWithdrawingFunds\[projectId\] \|\| projectWrongNetwork\(project\)/)
  assert.match(handler, /reconcileV3Pending\(project, address\)/)
  assert.match(handler, /withdrawV3ProjectTokens\(project, address\)/)
  assert.ok(handler.indexOf('withdrawV3ProjectTokens') < handler.indexOf('proj.tokensWithdrawn = true'))
  assert.match(handler, /unresolved \? 'warning' : 'error'/)
  assert.match(source, /getTxUrl\(modalConfig\.txHash, modalConfig\)/)
  assert.doesNotMatch(source, /@confirm="handleWithdrawTokens/)
})

test('V3 proceeds require Successful, exact owner and not withdrawn, independent of token/fee withdrawal flags', () => {
  const p = { projectOwner: owner, fundsWithdrawn: false, totalRaised: 130680000000000000000n,
    successFeeRate: 100n, projectTokensWithdrawn: true, platformFeeWithdrawn: false }
  const ready = { status: 2, project: p }
  assert.equal(v3ProjectActionState(project, ready, 'withdrawRaisedFunds', owner, runtime), ready)
  const fee = p.totalRaised * p.successFeeRate / 10000n
  assert.equal(fee, 1306800000000000000n)
  assert.equal(p.totalRaised - fee, 129373200000000000000n)
  for (const status of [0, 1, 3, 4]) assert.throws(() => v3ProjectActionState(project, { ...ready, status }, 'withdrawRaisedFunds', owner, runtime))
  for (const patch of [{ fundsWithdrawn: true }, { fundsWithdrawn: undefined }, { projectOwner: project.tokenAddress }, { totalRaised: 0n }]) {
    assert.throws(() => v3ProjectActionState(project, { ...ready, project: { ...p, ...patch } }, 'withdrawRaisedFunds', owner, runtime))
  }
})

test('dedicated V3 proceeds entry encodes only exact project ID; legacy entry still rejects V3', async () => {
  const s = new ContractService()
  s._v3Context = () => project
  let request
  s._v3Action = async (...args) => { request = args; return { hash: 'mock' } }
  await s.withdrawV3RaisedFunds(project, owner)
  assert.deepEqual(request, [project, 'withdrawRaisedFunds', owner, 'withdrawRaisedFunds(uint256)', [BigInt(project.id)]])
  const abi = new Interface(IDO_V3_ABI)
  assert.equal(abi.decodeFunctionData('withdrawRaisedFunds', abi.encodeFunctionData('withdrawRaisedFunds', request[4]))[0], BigInt(project.id))
  s._ensureProjectSigner = async () => { throw Error('Must reject before legacy signer') }
  await assert.rejects(s.withdrawRaisedFunds(project), { code: 'IDO_V3_ROUTING_UNAVAILABLE' })
})

test('Dashboard proceeds route, pending/error handling and receipt-bound success match V3 token withdrawal', () => {
  const source = readFileSync(new URL('../src/views/Dashboard.vue', import.meta.url), 'utf8')
  const handler = source.slice(source.indexOf('async function handleWithdrawFunds'), source.indexOf('async function handleWithdrawTokens'))
  assert.match(handler, /withdrawV3RaisedFunds\(project, address\)/)
  assert.match(handler, /reconcileV3Pending\(project, address\)/)
  assert.match(handler, /address !== walletStore.address/)
  assert.match(handler, /unresolved \? 'warning' : 'error'/)
  assert.match(handler, /chainId: project.chainId/)
  assert.ok(handler.indexOf('withdrawV3RaisedFunds') < handler.indexOf('proj.fundsWithdrawn = true'))
  assert.doesNotMatch(handler, /withdrawPlatformFee|feeReceiver\s*=/)
})
