import test from 'node:test'
import assert from 'node:assert/strict'
import { reactive } from 'vue'
import { readFileSync } from 'node:fs'
import { loadDashboardInvestors } from '../src/utils/dashboardInvestors.js'

test('V2 and V3 use the same reactive paginated list without losing string money or long IDs', async () => {
  const states = reactive({})
  const id = '43359438123302913'
  await loadDashboardInvestors(states, id, 2, async (project, page, size) => {
    assert.equal(project, id); assert.equal(page, 2); assert.equal(size, 10)
    return { items: [{ id: '43368658176704510', walletAddress: '0xabc', amountDisplay: '0.05', tokenSymbol: 'USDT' }], total: 11, page: 2, pageSize: 10 }
  })
  assert.equal(states[id].loaded, true)
  assert.equal(states[id].loading, false)
  assert.equal(states[id].error, false)
  assert.equal(states[id].items[0].amountDisplay, '0.05')
  assert.equal(states[id].page, 2)
})

test('API failure is unavailable, never a successfully loaded empty list; retry recovers', async () => {
  const states = reactive({})
  await loadDashboardInvestors(states, '1', 1, async () => { throw Error('V2-only / projection unavailable') })
  assert.equal(states['1'].error, true); assert.equal(states['1'].loaded, false)
  await loadDashboardInvestors(states, '1', 1, async () => ({ items: [], total: 0 }))
  assert.equal(states['1'].error, false); assert.equal(states['1'].loaded, true)
})

test('older page/account response cannot overwrite newer list', async () => {
  const states = reactive({}); let resolve
  const pending = loadDashboardInvestors(states, '1', 1, () => new Promise(r => { resolve = r }))
  await loadDashboardInvestors(states, '1', 2, async () => ({ items: [{ id: 'new' }], total: 11, page: 2 }))
  resolve({ items: [{ id: 'old' }], total: 11, page: 1 }); await pending
  assert.equal(states['1'].items[0].id, 'new')
  let current = true
  const oldAccount = loadDashboardInvestors(states, '2', 1, () => new Promise(r => { resolve = r }), () => current)
  current = false; delete states['2']
  resolve({ items: [{ id: 'private' }], total: 1 }); await oldAccount
  assert.equal(states['2'], undefined)
})

test('Dashboard renders error/retry before empty state and resets investor cache on account changes', () => {
  const s = readFileSync(new URL('../src/views/Dashboard.vue', import.meta.url), 'utf8')
  assert.ok(s.indexOf('v-else-if="projectInvestors[selectedProjectId].error"') < s.indexOf('v-else-if="!projectInvestors[selectedProjectId].items.length"'))
  assert.match(s, /loadDashboardInvestors\(projectInvestors, id, page, getProjectInvestors/)
  assert.match(s, /watch\(\(\) => walletStore.address,[\s\S]*?clearProjectInvestors\(\)/)
  assert.match(s, /t.retryInvestors/)
})
