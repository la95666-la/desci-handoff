import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { projectFaqItems, v3ProjectFaq } from '../src/utils/projectFaq.js'

test('V3 preserves all Admin FAQ entries, ordering and bilingual answers alongside participation notice', () => {
  const items = [
    { q: 'Research?', a: 'Trial details', q_zh: '研究？', a_zh: '試驗資料' },
    { q: 'Token release?', a: 'Two tranches', q_zh: '釋放？', a_zh: '分兩期' },
    { q: 'Risks?', a: 'See disclosure' },
  ]
  for (const raw of [items, JSON.stringify(items)]) {
    const result = v3ProjectFaq(raw)
    assert.equal(result.length, 4)
    assert.match(result[0].a, /Not necessarily/)
    assert.deepEqual(result.slice(1), items)
  }
  assert.equal(items.length, 3)
})

test('empty/malformed FAQ never crashes; non-FAQ objects ignored, translations kept', () => {
  for (const raw of [undefined, null, '', 'bad', '{}', {}, 'null', []]) {
    assert.deepEqual(projectFaqItems(raw), [])
    assert.equal(v3ProjectFaq(raw).length, 1)
  }
  assert.deepEqual(projectFaqItems([null, 1, {}, { q: ' ' }, { q_zh: '問題', a_zh: '答案' }]), [{ q_zh: '問題', a_zh: '答案' }])
})

test('shared detail renders configured V3 FAQ without replacing it or rendering unsafe HTML', () => {
  const s = readFileSync(new URL('../src/views/LaunchpadDetailPreLaunch.vue', import.meta.url), 'utf8')
  assert.match(s, /projectFaqItems\(detail.value\?\.faqJson\)/)
  assert.match(s, /if \(isV3Project.value\) \{\s+return v3ProjectFaq\(value\)/)
  assert.match(s, /v-for="\(q, i\) in faq"/)
  assert.match(s, /\{\{ pickField\(q, 'a'\) \}\}/)
})
