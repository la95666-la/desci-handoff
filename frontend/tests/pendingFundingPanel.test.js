import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const detail = readFileSync(new URL('../src/views/LaunchpadDetailPreLaunch.vue', import.meta.url), 'utf8')
const panel = readFileSync(new URL('../src/components/LaunchpadActiveFundingPanel.vue', import.meta.url), 'utf8')

test('Pending V2 is routed to the existing themed funding panel without opening wallet actions', () => {
  assert.match(detail, /if \(s === 'pending'\) return \{\s*key: 'pending'/)
  assert.match(detail, /isCappedFundingPanelStage\(stage\.key\).*fundingRoute\.kind === 'capped_presale' && fundingRoute\.contractVersion === 'v2_stablecoin'/)
  assert.match(detail, /:language="props\.language"\s*:is-dark="isDark"/)
  assert.match(panel, /const isLive = computed\(\(\) => stageKey\.value === 'live'\)/)
  assert.match(panel, /const canPurchase = computed\(\(\) => wallet\.isConnected && isLive\.value && amountIsValid\.value &&/)
  assert.doesNotMatch(panel, /VITE_IDO_V3_WALLET_ENABLED/)
  assert.match(panel, /v-if="isLive \|\| v3LocalPurchaseBusy" class="funding-purchase-zone/)
  assert.match(panel, /if \(!canPurchase\.value \|\| !needsApproval\.value \|\| paymentActionBusy\.value\) return/)
  assert.match(panel, /if \(!canPurchase\.value \|\| needsApproval\.value \|\| paymentActionBusy\.value\) return/)
})

test('Pending countdown is localized and uses the panel dark/light palette', () => {
  assert.match(panel, /getProjectCountdownTarget\(props\.project, stageKey\.value\)/)
  assert.match(panel, /v-if="isPending \|\| isUpcoming" class="funding-term-cell/)
  assert.match(panel, /startCountdown \? t\.startsIn : t\.comingSoon/)
  assert.match(panel, /pendingActivation: '等待鏈上激活/)
  assert.match(panel, /pendingActivation: 'Awaiting on-chain activation/)
  assert.match(panel, /funding-countdown-unit \{ border: 1px solid var\(--funding-border\); background: var\(--funding-card\); color: var\(--funding-text\); \}/)
})

test('activated upcoming V2 shows read-only terms and start countdown until live', () => {
  assert.match(detail, /if \(s === 'upcoming'\) return \{\s*key: 'upcoming'/)
  assert.match(detail, /isCappedFundingPanelStage\(stage\.key\).*fundingRoute\.contractVersion === 'v2_stablecoin'/)
  assert.match(panel, /const isUpcoming = computed\(\(\) => stageKey\.value === 'upcoming'\)/)
  assert.match(panel, /if \(!isPending\.value && !isUpcoming\.value\) return null/)
  assert.match(panel, /isPending \? t\.pendingActivation : t\.awaitingSaleStart/)
  assert.match(panel, /awaitingSaleStart: '等待募資開始 · 暫不可認購'/)
  assert.match(panel, /awaitingSaleStart: 'Funding has not started · Purchases unavailable'/)
  assert.match(panel, /v-if="isLive \|\| v3LocalPurchaseBusy" class="funding-purchase-zone/)
  assert.match(panel, /v-if="wallet\.isConnected && isLive" class="funding-wallet-cell/)
})
