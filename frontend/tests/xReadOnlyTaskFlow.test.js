import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const taskHall = fs.readFileSync(new URL('../src/views/TaskHall.vue', import.meta.url), 'utf8')
const api = fs.readFileSync(new URL('../src/services/api.js', import.meta.url), 'utf8')
const translations = fs.readFileSync(new URL('../src/i18n/taskhall.js', import.meta.url), 'utf8')

test('X task UI uses a two-stage manual action and read-only verification modal', () => {
  assert.match(taskHall, /openXVerifyModal\(url, points, path, key, taskId, taskVersion, stateKey\)/)
  assert.match(taskHall, /submitXVerification/)
  assert.match(taskHall, /t\.xVerifyModal_hint/)
  assert.doesNotMatch(taskHall, /backend (?:will )?represent(?:s)? the user|backend 會代表使用者|backend 会代表用户/i)
})

test('two X posts with the same action keep independent completion and busy state', () => {
  assert.match(taskHall, /targetSlot: Number\(i\.target_slot\) \|\| 1/)
  assert.match(taskHall, /const taskStateKey = task => task\?\.id \? `id:\$\{task\.id\}`/)
  assert.match(taskHall, /completedTaskIds/)
  assert.match(taskHall, /info\.value\.x\?\.\[`key:\$\{normalizeTaskKey\(task\?\.key\)\}`\]/)
  assert.match(taskHall, /:key="taskStateKey\(task\)"/)
  assert.match(taskHall, /taskBusyPath\('x', task\)/)
  assert.match(taskHall, /xRewardTotal/)
})

test('opening a task never directly calls completeTwitterTask', () => {
  const start = taskHall.indexOf('const openAndVerify = async')
  const end = taskHall.indexOf('// ===== 处理 X OAuth callback', start)
  const flow = taskHall.slice(start, end)
  assert.match(flow, /openXVerifyModal/)
  assert.doesNotMatch(flow, /completeTwitterTask/)
})

test('Admin bypass keeps the two-stage X action and confirmation modal', () => {
  assert.match(taskHall, /const twitterVerificationDisabled = ref\(false\)/)
  assert.match(taskHall, /res\?\.verificationDisabled === true/)
  const start = taskHall.indexOf('function handleXTaskLink')
  const end = taskHall.indexOf('const stopTaskInfoPolling', start)
  const flow = taskHall.slice(start, end)
  assert.match(flow, /openXVerifyModal/)
  assert.match(flow, /window\.open\(task\.url, '_blank', 'noopener'\)/)
  assert.doesNotMatch(flow, /completeTwitterTask/)
  assert.doesNotMatch(taskHall, /submitXCompletionWithoutVerification/)
  assert.match(taskHall, /submitXVerification[\s\S]*completeTwitterTask\(key, '', taskId, taskVersion\)/)
})

test('verification endpoint remains explicit and signed', () => {
  assert.match(api, /completeTwitterTask[\s\S]*platform: 'twitter'/)
  assert.match(api, /completeTwitterTask[\s\S]*withSign: true/)
})

test('native X error alerts are English-only and never show Backend-localized messages', () => {
  assert.match(taskHall, /alert\(TRANSLATIONS\.en\.xTaskAlreadyCompleted\)/)
  assert.match(taskHall, /alert\(TRANSLATIONS\.en\.xVerifyModal_notFound\)/)
  assert.match(taskHall, /alert\(TRANSLATIONS\.en\.xVerifyModal_failed \+ \(e\?\.code \|\| 'UNKNOWN'\)\)/)
  assert.match(taskHall, /alert\('X authorization failed: ' \+ \(e\?\.code \|\| 'UNKNOWN'\)\)/)
  assert.doesNotMatch(taskHall, /alert\(msg\)/)
  assert.doesNotMatch(taskHall, /alert\('X 授权失败/)
})

test('all locales disclose that JoinCare cannot act on the user behalf', () => {
  assert.match(translations, /JoinCare cannot follow, unfollow, like, unlike, post, or repost for you/)
  assert.match(translations, /JoinCare 无法替你关注、取消关注、点赞、取消点赞、发帖或转发/)
  assert.match(translations, /JoinCare 無法替你追蹤、取消追蹤、按讚、取消按讚、發文或轉發/)
})

test('read-only verification modal follows dark mode and has a mobile layout', () => {
  assert.match(taskHall, /reply-modal-overlay" :class="\{ 'is-dark': isDark \}"/)
  assert.match(taskHall, /\.reply-modal-overlay\.is-dark \.reply-modal/)
  assert.match(taskHall, /\.reply-modal-overlay\.is-dark \.reply-steps li/)
  assert.match(taskHall, /@media \(max-width: 768px\)[\s\S]*\.reply-modal-footer[\s\S]*grid-template-columns: 1fr 1fr/)
})
