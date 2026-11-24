import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import { v3UserMessage } from '../src/utils/v3UserMessages.js'

test('V3 user-facing errors describe a safe next step in both languages', () => {
  for (const code of [
    'IDO_V3_ROUTING_UNAVAILABLE', 'IDO_V3_IDENTITY_UNAVAILABLE', 'IDO_V3_STATE_UNAVAILABLE',
    'IDO_V3_WALLET_CHANGED', 'IDO_V3_PROOF_UNAVAILABLE', 'IDO_V3_SUBMISSION_UNRESOLVED',
    'IDO_V3_WALLET_REJECTED', 'IDO_V3_WALLET_UNSUPPORTED', 'IDO_V3_BROWSER_UNSUPPORTED',
  ]) {
    for (const isZh of [false, true]) {
      const message = v3UserMessage(code, isZh)
      assert.ok(message.length > 20)
      assert.doesNotMatch(message, /IDO_V3_|proof|proxy|合約|證明|鏈上策略|快照/i)
    }
  }
  assert.match(v3UserMessage('IDO_V3_SUBMISSION_UNRESOLVED'), /do not submit it again/i)
  assert.match(v3UserMessage('IDO_V3_SUBMISSION_UNRESOLVED', true), /切勿再次提交/)
  assert.equal(v3UserMessage('unexpected'), v3UserMessage('IDO_V3_STATE_UNAVAILABLE'))
})

test('V3 panel displays friendly copy first and keeps the code in support details', () => {
  const panel = readFileSync(new URL('../src/components/LaunchpadActiveFundingPanel.vue', import.meta.url), 'utf8')
  assert.match(panel, /message: actionPending.length \? v3FailureMessage\.value : actionError/)
  assert.match(panel, /\{\{ v3FailureMessage \}\}[\s\S]*?<details[^>]*>.*?Error code for support/)
  assert.doesNotMatch(panel, /\{\{ v3FailureMessage \}\} \(\{\{ v3Failure \}\}\)/)
})
