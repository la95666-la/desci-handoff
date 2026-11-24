import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

test('owner panels retain Setup status copy without token-info submission buttons in either layout', () => {
  const source = readFileSync(new URL('../src/views/Dashboard.vue', import.meta.url), 'utf8')
  assert.equal(source.match(/v-else-if="project\.status === 'Setup'"/g)?.length, 2)
  assert.equal(source.match(/\{\{ t\.setupPrompt \}\}/g)?.length, 2)
  assert.doesNotMatch(source, /t\.submitTokenInfo|goSetup|<Settings/)
})
