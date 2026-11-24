import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('../src/views/TaskHall.vue', import.meta.url), 'utf8')

test('check-in never waits for global leaderboard before wallet-owned chain preflight', () => {
  const flow = source.slice(source.indexOf('const doCheckIn = async () => {'), source.indexOf('// clipboard 在 HTTP'))
  assert.ok(flow.startsWith('const doCheckIn = async () => {'))
  assert.doesNotMatch(flow, /await loadInfo\(\)/)
  assert.match(flow, /if \(pendingHash\)[\s\S]*?await api\.submitCheckIn\(pendingHash\)/)
  assert.match(flow, /await readContract\(wagmiConfig,[\s\S]*?hasCheckedInToday[\s\S]*?if \(checkedOnChain\)[\s\S]*?await reconcileCurrentCheckIn\(\)[\s\S]*?await writeContract\(wagmiConfig/)
  assert.match(flow, /savePendingCheckInHash\(hash\)/)
})

test('periodic display refresh cannot stack or race a check-in', () => {
  assert.match(source, /const TASK_INFO_POLL_INTERVAL = 30000/)
  assert.match(source, /if \(!wallet\.isLoggedIn \|\| busyDaily\.value \|\| document\.visibilityState === 'hidden' \|\| taskInfoRefreshInFlight\) return/)
  assert.match(source, /if \(busyDaily\.value \|\| requestedWallet !== normalizedWallet\(\) \|\| !wallet\.isLoggedIn\) return/)
})

test('check-in refuses a disconnected or changed connector before any wallet send', () => {
  const flow = source.slice(source.indexOf('const doCheckIn = async () => {'), source.indexOf('// clipboard 在 HTTP'))
  assert.match(flow, /if \(!account\.isConnected \|\| !account\.address \|\|[\s\S]*?account\.address\.toLowerCase\(\) !== normalizedWallet\(\)\)/)
  assert.match(flow, /currentAccount\.chainId !== targetChainId \|\|[\s\S]*?currentAccount\.address\?\.toLowerCase\(\) !== normalizedWallet\(\)/)
  assert.match(flow, /const signingAccount = getAccount\(wagmiConfig\)[\s\S]*?signingAccount\.address\?\.toLowerCase\(\) !== normalizedWallet\(\)[\s\S]*?await writeContract\(wagmiConfig/)
  assert.match(flow, /CHECKIN_WALLET_MISMATCH[\s\S]*?await writeContract\(wagmiConfig/)
})
