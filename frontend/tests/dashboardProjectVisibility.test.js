import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const source = readFileSync(new URL('../src/views/Dashboard.vue', import.meta.url), 'utf8')

test('owner project cards and investor panel never wait for scroll-reveal', () => {
  const cards = source.match(/<section v-for="project in myProjects"[^>]+>/)?.[0]
  const investors = source.match(/<section v-if="showSelectedProjectInvestors"[^>]+>/)?.[0]
  assert.ok(cards)
  assert.ok(investors)
  assert.doesNotMatch(cards, /data-reveal/)
  assert.doesNotMatch(investors, /data-reveal/)
})

test('tab switches and asynchronous owner projects refresh canvas after DOM updates', () => {
  assert.match(source, /watch\(\(\) => \[activeTab\.value, myProjects\.value, selectedProjectId\.value, isMobile\.value\], async \(\) => \{\s*await nextTick\(\)\s*measure\(\)\s*scrollRevealHandler\(\)/)
})
