import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const read = (path) => fs.readFileSync(new URL(path, import.meta.url), 'utf8')
const footer = read('../src/components/HomeFooter.vue')

test('scaled desktop pages render the footer background at viewport width', () => {
  for (const path of [
    '../src/views/Ecosystem.vue',
    '../src/views/LaunchpadList.vue',
    '../src/views/LaunchpadDetailPreLaunch.vue',
    '../src/views/TaskHall.vue',
  ]) {
    const source = read(path)
    assert.match(source, /<HomeFooter[^>]*full-bleed[^>]*:scale="scale"/)
  }

  assert.match(footer, /width:\s*calc\(100vw \/ var\(--jc-footer-scale, 1\)\)/)
  assert.match(footer, /\.jc-hf-full-bleed\s*\{\s*overflow:\s*visible;/)
  assert.match(footer, /jc-hf-dark\.jc-hf-full-bleed::after/)
})
