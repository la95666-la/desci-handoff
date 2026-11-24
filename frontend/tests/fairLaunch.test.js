import assert from 'node:assert/strict'
import test from 'node:test'

import {
  clampPercentForBar,
  compareDecimalStrings,
  decimalRatioPercent,
  fairLaunchSettlementState,
  formatCanonicalSaleTokenAmount,
  formatDecimalString,
  formatRawUnits,
  hasReachedSoftCap,
  hasUnixTimePassed,
  resolveProjectFundingRoute,
} from '../src/utils/fairLaunch.js'
import { localizedProjectName } from '../src/utils/projectLocalization.js'
import { backendLanguage, normalizeUiLanguage } from '../src/utils/language.js'

import fs from 'node:fs'

test('legacy launchpad detail preserves the Traditional Chinese project name', () => {
  const source = fs.readFileSync(new URL('../src/views/LaunchpadDetail.vue', import.meta.url), 'utf8')

  assert.match(source, /nameCn:\s*data\.nameCn\s*\|\|\s*data\.name/)
  assert.doesNotMatch(source, /nameCn:\s*data\.name,/)
  assert.match(source, /logo:\s*data\.logo\?\.url\s*\|\|\s*data\.coverUrl/)
  assert.match(source, /banner:\s*data\.banner\?\.url\s*\|\|\s*data\.coverUrl/)
  assert.match(source, /watch\(\(\)\s*=>\s*props\.language,[\s\S]*!isUnifiedDetail\.value[\s\S]*loadProject\(\)/)
})

test('all public launchpad surfaces refresh or select localized project names', () => {
  const home = fs.readFileSync(new URL('../src/views/Home.vue', import.meta.url), 'utf8')
  const list = fs.readFileSync(new URL('../src/views/LaunchpadList.vue', import.meta.url), 'utf8')
  const detail = fs.readFileSync(new URL('../src/views/LaunchpadDetailPreLaunch.vue', import.meta.url), 'utf8')

  assert.match(home, /title:\s*localizedProjectName\(p,\s*requestLanguage\)/)
  assert.match(home, /loadFeaturedProjects\(\)/)
  assert.match(list, /watch\(\(\)\s*=>\s*props\.language,\s*loadProjects\)/)
  assert.match(list, /localizedProjectName\(p,\s*requestLanguage\)/)
  assert.match(detail, /watch\(\(\)\s*=>\s*\[props\.language, projectID\.value\],\s*loadDetail\)/)
})

test('localized project names cover English, Traditional Chinese and safe fallbacks', () => {
  const project = { name: 'Header localized', nameEn: 'English', nameCn: '繁體中文' }
  assert.equal(localizedProjectName(project, 'en'), 'English')
  assert.equal(localizedProjectName(project, 'en-US'), 'English')
  assert.equal(localizedProjectName(project, 'tw'), '繁體中文')
  assert.equal(localizedProjectName(project, 'zh-TW'), '繁體中文')
  assert.equal(localizedProjectName({ name: 'API fallback' }, 'tw'), 'API fallback')
  assert.equal(localizedProjectName({ nameCn: '僅中文' }, 'en'), '僅中文')
})

test('stored language initializes the UI and Backend request header consistently', () => {
  assert.equal(normalizeUiLanguage('tw'), 'tw')
  assert.equal(normalizeUiLanguage('zh-TW'), 'tw')
  assert.equal(normalizeUiLanguage('en-US'), 'en')
  assert.equal(backendLanguage('tw'), 'zh-tw')
  assert.equal(backendLanguage('en'), 'en-us')
})

test('localized project loaders discard stale responses from a previous language', () => {
  const home = fs.readFileSync(new URL('../src/views/Home.vue', import.meta.url), 'utf8')
  const list = fs.readFileSync(new URL('../src/views/LaunchpadList.vue', import.meta.url), 'utf8')
  const detail = fs.readFileSync(new URL('../src/views/LaunchpadDetailPreLaunch.vue', import.meta.url), 'utf8')

  assert.match(home, /requestID\s*!==\s*featuredProjectsRequest/)
  assert.match(list, /requestID\s*!==\s*projectsRequest/)
  assert.match(detail, /requestID\s*!==\s*detailRequest/)
})

test('routes only exact funding-mode and contract-version pairs', () => {
  assert.deepEqual(resolveProjectFundingRoute({ fundingMode: 'fair_launch', contractVersion: 'fair_launch_v1' }), {
    ok: true, kind: 'fair_launch', fundingMode: 'fair_launch', contractVersion: 'fair_launch_v1', readOnly: false,
  })
  assert.equal(resolveProjectFundingRoute({ fundingMode: 'capped_presale', contractVersion: 'v2_stablecoin' }).kind, 'capped_presale')
  assert.equal(resolveProjectFundingRoute({ contractVersion: 'v2_stablecoin' }).kind, 'capped_presale')
  assert.deepEqual(resolveProjectFundingRoute({ fundingMode: 'capped_presale', contractVersion: 'ido_v3_stablecoin' }), {
    ok: true, kind: 'ido_v3', fundingMode: 'capped_presale', contractVersion: 'ido_v3_stablecoin', readOnly: false,
  })
  assert.equal(resolveProjectFundingRoute({ contractVersion: 'ido_v3_stablecoin' }).ok, false)
})

test('fails closed on mismatched, missing, and unknown funding identity', () => {
  for (const project of [
    { fundingMode: 'fair_launch', contractVersion: 'v2_stablecoin' },
    { fundingMode: 'fair_launch', contractVersion: 'ido_v3_stablecoin' },
    { fundingMode: 'capped_presale', contractVersion: 'fair_launch_v1' },
    { fundingMode: 'fair_launch', contractVersion: 'future_v2' },
    { contractVersion: 'future_v2' },
    {},
  ]) {
    const route = resolveProjectFundingRoute(project)
    assert.equal(route.ok, false)
    assert.equal(route.kind, 'unsupported')
    assert.equal(route.readOnly, true)
  }
})

test('launchpad list hides unsupported funding identity labels', () => {
  const list = fs.readFileSync(new URL('../src/views/LaunchpadList.vue', import.meta.url), 'utf8')

  assert.doesNotMatch(list, /Unsupported mode/)
  assert.match(list, /modeBadge:\s*isFairLaunch\s*\?\s*'Fair Launch'\s*:\s*''/)
})

test('V3 list and detail do not advertise a purchase or signup as eligibility', () => {
  const list = fs.readFileSync(new URL('../src/views/LaunchpadList.vue', import.meta.url), 'utf8')
  const detail = fs.readFileSync(new URL('../src/views/LaunchpadDetailPreLaunch.vue', import.meta.url), 'utf8')
  assert.match(list, /isV3Project\s*\?\s*'Check if your wallet can participate on the project page'/)
  assert.match(list, /isV3Project\s*\?\s*'Signing up does not guarantee access'/)
  assert.match(detail, /if\s*\(isV3Project\.value\)\s*\{\s*return isV3WalletAvailable\.value/)
  assert.match(detail, /fundingRoute\.kind === 'ido_v3' \|\|/)
  assert.match(detail, /stage\.key === 'warmup' && !isV3Project/)
  assert.match(detail, /if \(isV3Project\.value\) return isV3WalletAvailable\.value/)
  assert.match(detail, /if \(isV3Project\.value\) \{\s*return v3ProjectFaq\(value\)/)
  const faqSource = fs.readFileSync(new URL('../src/utils/projectFaq.js', import.meta.url), 'utf8')
  assert.match(faqSource, /connect the wallet you plan to use/)
  assert.doesNotMatch(detail, /IdoV3FundingPanel/)
})

test('computes and formats financial values above Number safe integer with bigint', () => {
  assert.equal(compareDecimalStrings('9007199254740993', '9007199254740992'), 1)
  assert.equal(decimalRatioPercent('9007199254740993', '3002399751580331', 1), '300.0')
  assert.equal(formatDecimalString('9007199254740993.123456789', 6), '9,007,199,254,740,993.123456')
  assert.equal(formatRawUnits('9007199254740993123456789', 6, 6), '9,007,199,254,740,993,123.456789')
})

test('shows progress above 100 percent while clamping only the visual bar', () => {
  assert.equal(decimalRatioPercent('125.5', '100', 1), '125.5')
  assert.equal(clampPercentForBar('125.5'), '100')
  assert.equal(clampPercentForBar('99.9'), '99.9')
})

test('soft cap reached remains live and never implies settlement or claimability', () => {
  const project = { status: 'Active', totalRaised: '150', softCap: '100', endTime: '2000000000' }
  assert.equal(fairLaunchSettlementState(project, 1_900_000_000_000), 'soft_cap_reached_live')
  assert.equal(fairLaunchSettlementState({ ...project, endTime: 1_800_000_000 }, 1_900_000_000_000), 'ended_success_pending_settlement')
  assert.equal(fairLaunchSettlementState({ ...project, status: 'SuccessfulPendingRelease' }), 'pending_release')
  assert.equal(fairLaunchSettlementState({ ...project, status: 'Successful' }), 'released_success')
})

test('routes lifecycle comparisons through bigint decimal and unix helpers above 2^53', () => {
  assert.equal(hasReachedSoftCap({ totalRaisedRaw: '9007199254740993', softCapRaw: '9007199254740992' }), true)
  assert.equal(hasReachedSoftCap({ totalRaisedRaw: '9007199254740992', softCapRaw: '9007199254740993' }), false)
  assert.equal(hasUnixTimePassed('9007199254740993', 9007199254740993000n), true)
  assert.equal(hasUnixTimePassed('9007199254740993', 9007199254740992999n), false)
})

test('canonical sale-token fields never treat missing or snake-case data as zero', () => {
  assert.equal(formatCanonicalSaleTokenAmount({ tokenDecimals: 18 }), '—')
  assert.equal(formatCanonicalSaleTokenAmount({ sale_token_amount: '100', tokenDecimals: 18 }), '—')
  assert.equal(formatCanonicalSaleTokenAmount({ saleTokenAmount: '9007199254740993', tokenDecimals: 18 }), '9,007,199,254,740,993')
  assert.equal(formatCanonicalSaleTokenAmount({ saleTokenAmountRaw: '1000000000000000000', tokenDecimals: 18 }), '1')
})

test('active Fair Launch copy advertises only authoritatively gated wallet actions', () => {
  const source = fs.readFileSync(new URL('../src/views/LaunchpadDetailPreLaunch.vue', import.meta.url), 'utf8')
  assert.match(source, /trusted contract identity, authoritative chain state and Backend availability all agree/)
  assert.match(source, /Fair Launch availability is jointly gated by authoritative chain and Backend state/)
  assert.doesNotMatch(source, /Contribution and other on-chain transactions are not available/)
})
