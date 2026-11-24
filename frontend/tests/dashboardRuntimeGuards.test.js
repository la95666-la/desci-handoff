import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const readSource = (path) => readFileSync(new URL(path, import.meta.url), 'utf8')

test('Reown account connection initializes the ethers session before publishing connected state', () => {
  const source = readSource('../src/stores/wallet.js')
  const createSessionAt = source.indexOf('await web3Service.createSession(connectorProvider)')
  const connectedAt = source.indexOf('isConnected.value = true', createSessionAt)
  assert.ok(createSessionAt >= 0)
  assert.ok(connectedAt > createSessionAt)
})

test('Dashboard rebuilds wallet sessions for legacy reads without blocking public V3 reads', () => {
  const source = readSource('../src/views/Dashboard.vue')
  assert.match(source, /if \(!sessionReady && !\(await web3Service\.ensureSigner\(\)\)\) return/)
  assert.match(source, /other.length && await web3Service\.ensureSigner\(\)/)
  assert.ok(source.indexOf('const publicReads = Promise.all') < source.indexOf('other.length && await web3Service.ensureSigner()'))
})

test('Dashboard does not expose V2 deposits for a project that is not created on-chain', () => {
  const dashboard = readSource('../src/views/Dashboard.vue')
  const contract = readSource('../src/services/contract.js')
  assert.match(contract, /projectIdRaw === BigInt\(context\.id\)/)
  assert.match(dashboard, /chainData\.exists === false\) return/)
  assert.ok(dashboard.indexOf('chainData.exists === false') < dashboard.indexOf('V2_STATUS_MAP[chainData.status]'))
  assert.match(dashboard, /getTokenBalanceOnChain\(tokenAddr, walletStore\.address, project\.tokenDecimals, project\.chainId\)/)
  assert.match(dashboard, /getTokenAllowanceOnChain\(tokenAddr, walletStore\.address, spender, project\.tokenDecimals, project\.chainId\)/)
})

test('Dashboard gives V3 owners the same Pending deposit workflow through guarded V3 services', () => {
  const source = readSource('../src/views/Dashboard.vue')
  assert.match(source, /if \(resolveProjectFundingRoute\(project\)\.kind === 'ido_v3'\) return refreshV3ProjectChainData\(project\)/)
  assert.match(source, /await getVerifiedV3Wallet\(address\)/)
  assert.match(source, /contractService\.readV3ProjectTokenFunding\(project, address\)/)
  assert.match(source, /contractService\.approveV3ProjectToken\(project, amount, walletStore\.address\)/)
  assert.match(source, /contractService\.depositV3ProjectTokens\(project, amount, walletStore\.address\)/)
  assert.match(source, /const raw = project\.shortfall\?\.raw/)
  assert.equal(source.match(/<strong>\{\{ project\.shortfall\.display \}\} \{\{ project\.tokenSymbol \}\}<\/strong>/g)?.length, 2)
  assert.doesNotMatch(source, /v-model="project\.deposit\.amount"/)
  assert.doesNotMatch(source, /fillShortfall\(project\)/)
  assert.match(source, /project\.v3Pending = contractService\.v3Pending\(project, address\)/)
  const contract = readSource('../src/services/contract.js')
  assert.match(contract, /const tokenAddress = getAddress\(state\.project\.projectToken\)/)
  assert.match(contract, /token\.balanceOf\(wallet, \{ blockTag: state\.blockNumber \}\)/)
  assert.equal(contract.match(/const tokenAddress = getAddress\(state\.project\.paymentToken\)/g)?.length, 2)
  assert.match(contract, /new Contract\(tokenAddress, ERC20_ABI, this\._v3ReadRunner\(context\.chainId\)\)\.balanceOf\(ownerAddress, \{ blockTag: state\.blockNumber \}\)/)
  assert.doesNotMatch(source, /To add project tokens, claim or request a refund, open the project page/)
  assert.equal(source.match(/v-else-if="project\.status === 'Pending'"/g)?.length, 2)
  assert.equal(source.match(/@click="handleApproveExact\(project\)"/g)?.length, 2)
  assert.equal(source.match(/@click="handleDepositExact\(project\)"/g)?.length, 2)
})

test('Dashboard gives V3 investors the V2 claim, refund and history experience through guarded V3 services', () => {
  const source = readSource('../src/views/Dashboard.vue')
  const api = readSource('../src/services/api.js')
  assert.match(api, /export async function getIdoV3Portfolio\(walletAddress\)/)
  assert.match(source, /getIdoV3Portfolio\(address\)/)
  assert.match(source, /const cappedPositions = \[\.\.\.\(v2Portfolio\.positions \|\| \[\]\), \.\.\.\(v3Portfolio\.positions \|\| \[\]\)\]/)
  assert.match(source, /contractService\.claimV3\(project, address\)/)
  assert.match(source, /contractService\.refundV3\(project, address\)/)
  assert.match(source, /const target = \{ \.\.\.item, \.\.\.refundable \}/)
  assert.match(source, /refundAmount: target\.invested/)
  assert.match(source, /confirmedCappedClaims/)
  assert.equal(source.match(/claimIsPending\(item\) \? t\.claiming/g)?.length, 2)
  assert.match(source, /const refundIsPending = \(item\) => activeRefundKey\.value === refundKey\(item\)/)
  assert.equal(source.match(/refundIsPending\(item\) \? t\.refunding/g)?.length, 4)
})

test('V3 detail approval and purchase use the same UI lifecycle as V2 while retaining V3 services', () => {
  const source = readSource('../src/components/LaunchpadActiveFundingPanel.vue')
  const approval = source.slice(source.indexOf('async function handleApprove()'), source.indexOf('async function handlePurchase()'))
  const purchase = source.slice(source.indexOf('async function handlePurchase()'), source.indexOf('async function handleRefund()'))
  assert.match(approval, /approveV3PaymentToken\(v3Context\.value, approvedAmount, wallet\.address\)/)
  assert.match(approval, /paymentAllowance\.value = approvedRaw/)
  assert.match(approval, /v3Pending\.value = contractService\.v3Pending\(v3Context\.value, wallet\.address\)/)
  assert.match(approval, /IDO_V3_SUBMISSION_UNRESOLVED/)
  assert.doesNotMatch(approval, /purchaseAmount\.value = ''/)
  assert.match(purchase, /purchaseV3\(v3Context\.value, purchaseAmount\.value, address\)/)
  assert.match(purchase, /confirmedUserFloor = expectedUserRaw/)
  assert.match(purchase, /PURCHASE\/SEND\/IDO_V3_SUBMISSION_UNRESOLVED/)
  assert.match(source, /computeSoftAndHardRequirement\(/)
  assert.match(source, /const v3OwnerAmount = computed\(\(\) => formatUnits\(v3OwnerRequiredRaw\.value, v3Context\.value\.tokenDecimals\)\)/)
  assert.doesNotMatch(source, /v-model="v3OwnerAmount"/)
  assert.match(source, /isLoadingChain \|\| \(isV3 && v3Loading && !v3DisplayCurrent\)/)
  assert.match(source, /\.funding-owner-amount \{ border: 1px solid var\(--funding-border\); background: var\(--funding-cell\); \}/)
  assert.match(source, /isClaiming \? \(isZh \? '領取中…' : 'Claiming…'\)/)
  assert.match(source, /isRefunding \? t\.refunding : t\.refund/)
  assert.match(source, /\.funding-action-section \{ border-color: var\(--funding-border\); \}/)
})

test('Dashboard distinguishes project loading failures from an empty project list', () => {
  const source = readSource('../src/views/Dashboard.vue')
  assert.match(source, /const myProjectsError = ref\(''\)/)
  assert.match(source, /myProjectsError\.value = err\?\.message/)
  assert.match(source, /v-if="myProjectsError"/)
  assert.match(source, /v-else-if="!myProjects\.length"/)
})

test('Dashboard does not label an unresolved refund receipt as a failed refund', () => {
  const source = readSource('../src/views/Dashboard.vue')
  const translations = readSource('../src/i18n/dashboard.js')
  assert.match(source, /title: unresolved \? t\.value\.refundPending : t\.value\.refundError/)
  assert.match(translations, /refundPending: '退款狀態待確認'/)
  assert.match(translations, /refundPending: 'Refund Confirmation Pending'/)
})

test('Dashboard hides investor records and expands the project workspace before fundraising', () => {
  const source = readSource('../src/views/Dashboard.vue')
  assert.match(source, /INVESTOR_RECORD_STATUSES = new Set\(\['Active', 'Ended', 'SuccessfulPendingRelease', 'Successful', 'Failed', 'Canceled'\]\)/)
  assert.match(source, /const showSelectedProjectInvestors = computed/)
  assert.match(source, /:style="projectWorkspaceStyle"/)
  assert.match(source, /<section v-if="showSelectedProjectInvestors" class="min-w-0">/)
  assert.doesNotMatch(source, /selectedProjectId\.value = project\.projectId\s+loadProjectInvestors\(project, 1\)/)
})

test('Dashboard claim pending label belongs only to the selected wallet/project in both layouts', () => {
  const source = readSource('../src/views/Dashboard.vue')
  assert.match(source, /const pendingClaimKey = claimKey\(item\)/)
  assert.match(source, /if \(activeClaimKey\.value !== null \|\| userActionWrongNetwork\(item\) \|\| claimState\(item\)\.disabled\) return/)
  assert.match(source, /const claimIsPending = \(item\) => claimInProgressFor\(claimKey\(item\), activeClaimKey\.value\)/)
  assert.equal(source.match(/claimIsPending\(item\) \? t\.claiming/g)?.length, 2)
  assert.equal(source.match(/:disabled="claimState\(item\)\.disabled \|\| activeClaimKey !== null \|\| userActionWrongNetwork\(item\)"/g)?.length, 2)
  assert.doesNotMatch(source, /isClaiming/)
  assert.match(source, /if \(activeClaimKey\.value === pendingClaimKey\) activeClaimKey\.value = null/)
})

test('Dashboard gates each investor action by its own project chain', () => {
  const source = readSource('../src/views/Dashboard.vue')
  assert.match(source, /function userActionWrongNetwork\(project\)/)
  assert.equal(source.match(/:disabled="activeRefundKey !== null \|\| userActionWrongNetwork\(item\)"/g)?.length, 4)
  assert.match(source, /const refundable = refundableProjectFor\(item\)/)
  assert.match(source, /if \(!refundable \|\| activeRefundKey\.value !== null \|\| userActionWrongNetwork\(refundable\)\) return/)
})

test('Dashboard banner shares one network result without weakening write guards', () => {
  const source = readSource('../src/views/Dashboard.vue')
  assert.match(source, /const expectedChainId = computed\(\(\) => networkState\.value\.expectedChainId\)/)
  assert.match(source, /walletStore\.isConnected && networkState\.value\.wrongNetwork/)
  assert.match(source, /if \(!Number\.isInteger\(expected\) \|\| expected <= 0\) return true/)
  assert.doesNotMatch(source, /myProjects\.value\.some\(\(project\) => projectWrongNetwork\(project\)\)/)
})

test('Funding detail treats unresolved V3 claim and refund submissions as warnings', () => {
  const source = readSource('../src/components/LaunchpadActiveFundingPanel.vue')
  assert.match(source, /const actionPending = action === 'claim' \? claimPending.value : v3Pending.value/)
  assert.match(source, /const unresolved = actionPending.length > 0 \|\| isUnresolvedWalletTransactionError\(error\)/)
  assert.match(source, /modalConfig\.value = \{ type: unresolved \? 'warning' : 'error'/)
  assert.match(source, /unresolved \? t\.value\.refundPending : t\.value\.refundError/)
  assert.match(source, /unresolved \? t\.value\.claimPending/)
  assert.match(source, /t\.refund \+ ' · ' \+ userInvested\.toLocaleString\(\) \+ ' ' \+ project\.paymentToken/)
})

test('Dashboard investor records use readable semantic colors in dark mode', () => {
  const source = readSource('../src/views/Dashboard.vue')
  assert.match(source, /class="dash-investor-row /)
  assert.match(source, /class="dash-investor-wallet /)
  assert.match(source, /class="dash-investor-amount /)
  assert.match(source, /\.home-dark \.dash-investor-row \{[\s\S]*background: #1c1c1c;/)
  assert.match(source, /\.home-dark \.dash-investor-wallet \{ color: #f0f0f0; \}/)
  assert.match(source, /\.home-dark \.dash-investor-amount \{ color: #9bff00; \}/)
})

test('local environment does not connect the HTTP API port as a WebSocket by default', () => {
  const source = readSource('../src/config/environments.js')
  assert.doesNotMatch(source, /wsBaseUrl:\s*injectedWs\s*\|\|\s*['"]ws:\/\/localhost:13004['"]/)
})

test('local Reown metadata follows the actual local page origin', () => {
  const source = readSource('../src/services/reownAppKit.js')
  assert.match(source, /metadataUrl\s*=\s*isLocalMetadata\s*\?\s*window\.location\.origin/)
  assert.match(source, /url:\s*metadataUrl/)
})

test('expired below-soft-cap projects wait for authoritative settlement before exposing refunds', () => {
  const source = readSource('../src/components/LaunchpadActiveFundingPanel.vue')
  assert.match(source, /const settlementPending = computed\(\(\) => stageKey\.value === 'ended_failed'\)/)
  assert.match(source, /isRefundableProjectStage\(stageKey\.value\)/)
  assert.match(source, /v-if="settlementPending"/)
  assert.doesNotMatch(source, /const refundable =[^\n]*ended_failed/)
})

test('launchpad detail counts an activated upcoming project down to start, not end', () => {
  const source = readSource('../src/views/LaunchpadDetailPreLaunch.vue')
  assert.match(source, /deriveProjectStage\(detail\.value, now\.value\)/)
  assert.match(source, /getProjectCountdownTarget\(detail\.value, derivedStage\.value\)/)
  assert.doesNotMatch(source, /const target = isActiveStage\.value \? detail\.value\?\.endTime/)
})
