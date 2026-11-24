import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const source = fs.readFileSync(new URL('../src/views/Team.vue', import.meta.url), 'utf8')
const publicDir = fileURLToPath(new URL('../public', import.meta.url))

test('team page includes Natasha Sachsenmeier CTO profile in both languages', () => {
  assert.equal((source.match(/name: 'Natasha Sachsenmeier'/g) || []).length, 2)
  assert.equal((source.match(/position: 'CTO'/g) || []).length, 2)
  assert.match(source, /role: 'Chief Technology Officer'/)
  assert.match(source, /role: '技術長'/)
  assert.match(source, /machine learning engineer, entrepreneur and interdisciplinary thinker/)
  assert.match(source, /highest marks in her cohort/)
  assert.match(source, /Bayesian optimisation competition/)
  assert.match(source, /platform’s CareAgents/)
  assert.match(source, /機器學習工程師、創業家與跨領域思考者/)
  assert.match(source, /取得該屆最高成績/)
  assert.match(source, /貝葉斯最佳化競賽中獲得冠軍/)
  assert.match(source, /平台 CareAgents 的開發與部署/)
})

test('Natasha profile uses the supplied photo and LinkedIn URL', () => {
  assert.equal((source.match(/\/uploads\/2026-09\/b60a289e18443cbc4b88f31743b77dac\.png/g) || []).length, 2)
  assert.equal((source.match(/https:\/\/www\.linkedin\.com\/in\/natasha-sachsenmeier\//g) || []).length, 2)
})

test('team SPA route does not collide with a public/team directory', () => {
  assert.equal(fs.existsSync(path.join(publicDir, 'team')), false)
  assert.match(source, /\/team-assets\/peter-sachsenmeier\.jpg/)
  assert.match(source, /\/team-assets\/quentin-liu\.jpg/)
})
