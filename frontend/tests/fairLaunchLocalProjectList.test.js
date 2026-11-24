import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

test('local project list uses only the admitted Fair Launch HTTP authority', async () => {
  const source = await readFile(new URL('../src/services/api.js', import.meta.url), 'utf8')
  const start = source.indexOf('export async function getProjectList')
  const end = source.indexOf('export async function getProjectDetail', start)
  const implementation = source.slice(start, end)
  assert.match(implementation, /exactLocalFairLaunchApiAuthority\(\)/)
  assert.match(implementation, /localFairLaunchRequest\('\/client\/project\/v1\/list', query\)/)
  assert.match(implementation, /request\('GET', '\/client\/project\/v1\/list'/)
})
