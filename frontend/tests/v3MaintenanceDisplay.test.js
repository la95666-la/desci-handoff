import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

test('syncing V3 details show explicit read-only maintenance instead of a transaction panel', () => {
  const s = readFileSync(new URL('../src/views/LaunchpadDetailPreLaunch.vue', import.meta.url), 'utf8')
  assert.match(s, /if \(isV3SyncMaintenance.value\) return false/)
  assert.match(s, /if \(isV3SyncMaintenance.value\) return null/)
  assert.match(s, /v-if="isV3SyncMaintenance"/)
  assert.match(s, /錢包操作暫停/)
  assert.ok(s.indexOf('<article v-if="isV3SyncMaintenance"') < s.indexOf('<LaunchpadActiveFundingPanel'))
  assert.match(s, /key: 'syncing'.*ctaKey: 'none'/)
})

test('desktop and mobile cards label maintenance without misleading zero progress', () => {
  const s = readFileSync(new URL('../src/views/LaunchpadList.vue', import.meta.url), 'utf8')
  assert.equal(s.match(/\{\{ t.statusSyncing \}\}/g)?.length, 2)
  assert.equal(s.match(/p.statusKind === 'syncing' \|\| p.fundingSyncing \? \(language === 'en' \? 'Syncing' : '同步中'\) : p.progress \+ '%'/g)?.length, 2)
})
