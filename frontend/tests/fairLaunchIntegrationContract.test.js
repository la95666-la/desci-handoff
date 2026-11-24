import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import {
  canBootstrapFairLaunchPosition,
  fairLaunchPositionWalletContext,
  mergeFairLaunchPositionIdentity,
  normalizeFairLaunchPortfolioDto,
  normalizeFairLaunchPositionDto,
} from '../src/services/fairLaunchApiContract.js'
import { fairLaunchActionAvailable, trustedFairLaunchRuntime } from '../src/utils/fairLaunch.js'
import {
  assertFairLaunchAuthorizedActor,
  calculateFairLaunchRemainingRaw,
  unresolvedWalletTransaction,
} from '../src/services/fairLaunchWallet.js'
import { WALLET_ERROR_CODES } from '../src/services/errors.js'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const read = (name) => fs.readFileSync(path.join(ROOT, name), 'utf8')
const wallet = '0x1111111111111111111111111111111111111111'
const hash = (ch) => `0x${ch.repeat(64)}`

function position(overrides = {}) {
  return {
    projectId: '7', wallet, fundingMode: 'fair_launch', contractVersion: 'fair_launch_v1', chainStatus: 'Active',
    assets: {
      paymentToken: { address: '0x6666666666666666666666666666666666666666', decimals: 18 },
      projectToken: { address: '0x7777777777777777777777777777777777777777', decimals: 18 },
    },
    projection: {
      contribution: { state: 'available', raw: '12', display: '0.000012' },
      estimatedAllocation: { state: 'available', raw: '34', display: '0.000000000000000034' },
      claimed: { state: 'available', raw: '0', display: '0' },
      claimable: { state: 'not_settled', raw: null, display: null },
      finalAllocation: { state: 'not_settled', raw: null, display: null },
      refund: { state: 'not_refundable', raw: '0', display: '0' },
    },
    actions: { contribute: { available: false, reason: 'durable_wallet_submission_unavailable' } },
    runtime: {
      chainId: 56, proxyAddress: '0x2222222222222222222222222222222222222222',
      implementationAddress: '0x3333333333333333333333333333333333333333', implementationCodeHash: hash('a'),
      sourceRegistryRevision: 4, sourceRegistryDigest: hash('b'), observedBlockNumber: '12345678901234567890',
      observedBlockHash: hash('c'), observedAt: '2026-09-07T10:00:00Z', runtimeVerified: true,
    },
    source: { finality: 'finalized', reconciliationState: 'balanced', stateSource: 'canonical_chain_event' },
    ...overrides,
  }
}

test('typed Fair Launch DTO consumes projection and exact backend identity without synthesis', () => {
  const dto = normalizeFairLaunchPositionDto(position(), wallet.toUpperCase())
  assert.equal(dto.projection.contribution.raw, '12')
  assert.equal(dto.fundingMode, 'fair_launch')
  assert.equal(dto.runtime.observedBlockNumber, '12345678901234567890')
  assert.throws(() => normalizeFairLaunchPositionDto(position({ fundingMode: undefined }), wallet), /funding identity mismatch/)
  assert.throws(() => normalizeFairLaunchPositionDto(position({ runtime: { ...position().runtime, runtimeVerified: false } }), wallet), /not verified/)
})

test('detail-page bootstrap uses exact Fair identity plus authenticated connected wallet, then trusts only normalized position evidence', () => {
  const ordinaryDetail = { id: 7, fundingMode: 'fair_launch', contractVersion: 'fair_launch_v1' }
  assert.equal(canBootstrapFairLaunchPosition(ordinaryDetail, { address: wallet, isConnected: true, isLoggedIn: true }), true)
  assert.equal(canBootstrapFairLaunchPosition(ordinaryDetail, { address: wallet, isConnected: true, isLoggedIn: false }, { localAuthorityRead: true }), true)
  assert.equal(trustedFairLaunchRuntime(ordinaryDetail).enabled, false)

  const merged = mergeFairLaunchPositionIdentity(ordinaryDetail, position(), wallet)
  assert.equal(trustedFairLaunchRuntime(merged).enabled, true)
  assert.equal(merged.paymentTokenAddress, position().assets.paymentToken.address)
  assert.equal(merged.tokenAddress, position().assets.projectToken.address)
  assert.equal(fairLaunchActionAvailable(merged, 'contribute').enabled, false)
  assert.equal(fairLaunchActionAvailable(merged, 'contribute').reason, 'durable_wallet_submission_unavailable')
})

test('detail-page bootstrap and every action fail closed for wrong identity, unauthenticated wallet, or rejected evidence', () => {
  const exact = { id: 7, fundingMode: 'fair_launch', contractVersion: 'fair_launch_v1' }
  for (const project of [
    { ...exact, fundingMode: 'capped_presale' },
    { ...exact, contractVersion: 'v2_stablecoin' },
    { id: 7 },
  ]) assert.equal(canBootstrapFairLaunchPosition(project, { address: wallet, isConnected: true, isLoggedIn: true }), false)
  for (const state of [
    { address: wallet, isConnected: false, isLoggedIn: true },
    { address: wallet, isConnected: true, isLoggedIn: false },
    { address: '', isConnected: true, isLoggedIn: true },
  ]) assert.equal(canBootstrapFairLaunchPosition(exact, state), false)

  assert.equal(canBootstrapFairLaunchPosition(exact, { address: wallet, isConnected: false, isLoggedIn: false }, { localAuthorityRead: true }), false)

  assert.throws(() => mergeFairLaunchPositionIdentity(exact, position({ projectId: '8' }), wallet), /project identity mismatch/)
  assert.throws(() => mergeFairLaunchPositionIdentity(exact, position({ assets: { ...position().assets, paymentToken: { address: '', decimals: 18 } } }), wallet), /assets.paymentToken.address/)
  assert.throws(() => mergeFairLaunchPositionIdentity(exact, position({ source: { ...position().source, finality: 'latest' } }), wallet), /finality/)
  assert.throws(() => mergeFairLaunchPositionIdentity(exact, position({ actions: { contribute: { available: 'yes' } } }), wallet), /available/)
  assert.equal(fairLaunchActionAvailable(exact, 'contribute').enabled, false)
})

test('transaction amount conversion and estimates never use drifting ordinary-detail decimals/totals', () => {
  const panel = read('src/components/FairLaunchReadOnlyPanel.vue')
  assert.match(panel, /paymentDecimals = computed\(\(\) => effectiveProject\.value/)
  assert.match(panel, /estimatedAllocation/)
  assert.doesNotMatch(panel, /decimalToScaledInteger\(amount\.value, props\.project/)
  assert.doesNotMatch(panel, /BigInt\(String\(props\.project\.totalRaisedRaw/)
  assert.doesNotMatch(panel, /calculateProportionalAllocation/)
  assert.match(panel, /v-if="effectiveProject && actionGate\('contribute'\)\.enabled"/)
})

test('panel source fetches position before token reads and never gates bootstrap on untrusted detail runtime', () => {
  const panel = read('src/components/FairLaunchReadOnlyPanel.vue')
  assert.match(panel, /if \(!bootstrapEligible\.value\) return/)
  assert.doesNotMatch(panel, /if \(!runtime\.value\.enabled \|\| !wallet\.address\) return/)
  assert.match(panel, /const p = await getFairLaunchPosition[\s\S]*mergeFairLaunchPositionIdentity[\s\S]*getPaymentTokenAllowance/)
  assert.match(panel, /if \(!effectiveProject\.value \|\| !runtime\.value\.enabled\) throw/)
  assert.match(panel, /wallet\.sessionRevision/)
  assert.match(panel, /balanceRaw\.value = BigInt\(balance\.raw\)[\s\S]*notice\.value = ''/)
})

test('portfolio fails closed on wallet mismatch or incomplete trusted runtime', () => {
  assert.throws(() => normalizeFairLaunchPortfolioDto({ positions: [position({ wallet: '0x4444444444444444444444444444444444444444' })] }, wallet), /wallet mismatch/)
  assert.throws(() => normalizeFairLaunchPortfolioDto({ positions: [position({ runtime: { ...position().runtime, sourceRegistryDigest: '' } })] }, wallet), /sourceRegistryDigest/)
})

test('partial deposit always uses fresh exact remaining raw amount', () => {
  assert.equal(calculateFairLaunchRemainingRaw('100000000000000000000', '35000000000000000000'), '65000000000000000000')
  assert.throws(() => calculateFairLaunchRemainingRaw('100', '100'), /already complete/)
  assert.throws(() => calculateFairLaunchRemainingRaw('100', '101'), /already complete/)
})

test('authorized project proceeds actor is the fixed Safe rather than projectOwner', () => {
  const chainProject = { projectOwner: wallet, projectProceedsReceiver: '0x5555555555555555555555555555555555555555' }
  assert.equal(assertFairLaunchAuthorizedActor('withdrawProjectProceeds', chainProject.projectProceedsReceiver, chainProject), true)
  assert.throws(() => assertFairLaunchAuthorizedActor('withdrawProjectProceeds', wallet, chainProject), /authorized wallet or fixed Safe/)
})

test('broadcast uncertainty and receipt timeout remain unresolved and do-not-resend', () => {
  for (const code of [WALLET_ERROR_CODES.TRANSACTION_BROADCAST_UNCERTAIN, WALLET_ERROR_CODES.TRANSACTION_RECEIPT_TIMEOUT]) {
    const unresolved = unresolvedWalletTransaction({ code, transactionHash: hash('d') })
    assert.deepEqual(unresolved, { status: 'unresolved', transactionHash: hash('d'), doNotResend: true })
  }
})

test('backend-shaped position maps nested assets into exact Dashboard wallet context', () => {
  const mapped = fairLaunchPositionWalletContext(position(), wallet)
  assert.equal(mapped.paymentTokenAddress, position().assets.paymentToken.address)
  assert.equal(mapped.paymentTokenDecimals, position().assets.paymentToken.decimals)
  assert.equal(mapped.tokenAddress, position().assets.projectToken.address)
  assert.equal(mapped.tokenDecimals, position().assets.projectToken.decimals)
  const dashboard = read('src/views/Dashboard.vue')
  assert.match(dashboard, /fairLaunchPositionWalletContext\(item, walletStore\.address\)/)
})

test('all Fair Launch wallet writes and every ERC20 approval generation remain disabled without Backend durable authority', () => {
  const disabled = position({
    actions: Object.fromEntries(['contribute', 'claim', 'refund', 'depositProjectTokens', 'withdrawFailedProjectTokens'].map((action) => [action, { available: false, reason: 'durable_wallet_submission_unavailable' }]))
  })
  const mapped = fairLaunchPositionWalletContext(disabled, wallet)
  for (const action of ['contribute', 'claim', 'refund', 'depositProjectTokens', 'withdrawFailedProjectTokens']) {
    const gate = fairLaunchActionAvailable(mapped, action)
    assert.equal(gate.enabled, false)
    assert.equal(gate.reason, 'durable_wallet_submission_unavailable')
  }
  const contract = read('src/services/contract.js')
  assert.match(contract, /DURABLE_WALLET_SUBMISSION_UNAVAILABLE/)
  assert.match(contract, /async _approveExact[\s\S]*const declared = this\._requireFairLaunchAction\(project, action\)[\s\S]*const current =/)
  assert.match(contract, /if \(current !== 0n && target !== 0n && current !== target\) await approve\(0n\)/)
  assert.match(contract, /if \(current !== target\) await approve\(target\)/)
})

test('wallet runtime preflight retains the reviewed action-specific recovery matrix', () => {
  const contract = read('src/services/contract.js')
  assert.match(contract, /claim: \['project'\]/)
  assert.match(contract, /withdrawFailedProjectTokens: \['project'\]/)
  assert.match(contract, /withdrawProjectProceeds: \['payment'\]/)
  assert.match(contract, /withdrawPlatformFee: \['payment'\]/)
  assert.match(contract, /refund: \[\]/)
  assert.match(contract, /recordFairLaunchUnresolved/)
  assert.match(contract, /assertFairLaunchNotUnresolved/)
})

test('wallet API sends authenticated wallet query and dashboard does not synthesize Fair Launch identity', () => {
  const api = read('src/services/api.js')
  const dashboard = read('src/views/Dashboard.vue')
  assert.match(api, /query: \{ wallet: walletAddress \}/)
  assert.match(api, /normalizeFairLaunchPositionDto\(data, walletAddress\)/)
  assert.doesNotMatch(dashboard, /fundingMode: 'fair_launch', contractVersion: 'fair_launch_v1'/)
  assert.match(dashboard, /fundingMode: item\.fundingMode, contractVersion: item\.contractVersion/)
})

test('Fair Launch Safe receiver and canonical facts are guarded without widening legacy behavior', () => {
  const contract = read('src/services/contract.js')
  const dashboard = read('src/views/Dashboard.vue')
  const owner = read('src/components/FairLaunchOwnerPanel.vue')
  assert.match(contract, /signerAddress !== getAddress\(chainProject\.projectProceedsReceiver\)/)
  assert.doesNotMatch(owner, /withdrawFairLaunchProjectProceeds/)
  assert.match(dashboard, /if \(route\.kind === 'fair_launch'\)/)
  assert.match(dashboard, /else \{\s*claimHistory\.value\.unshift/)
  assert.match(dashboard, /else \{\s*refundHistory\.value\.unshift/)
  assert.match(dashboard, /Transaction submitted\. Claim history and balances update only after canonical event application and reconciliation/)
})
