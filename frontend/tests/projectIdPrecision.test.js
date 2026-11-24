import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

test('unified project detail keeps Sonyflake route IDs as decimal strings', () => {
  const source = readFileSync(new URL('../src/views/LaunchpadDetailPreLaunch.vue', import.meta.url), 'utf8')
  assert.match(source, /const projectID = computed\(\(\) => String\(route\.params\.id \|\| ''\)\)/)
  assert.doesNotMatch(source, /Number\(route\.params\.id\)/)
  const projectID = '40104297089728513'
  assert.notEqual(String(Number(projectID)), projectID)
  assert.equal(String(projectID), projectID)
})
