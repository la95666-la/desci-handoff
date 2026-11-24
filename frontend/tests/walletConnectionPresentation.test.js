import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const read = (path) => readFile(new URL(path, import.meta.url), 'utf8')

test('navigation presents an authorized wallet independently from JoinCare token authentication', async () => {
  const [floatingNavbar, navbar, walletStore] = await Promise.all([
    read('../src/components/JcFloatingNavbar.vue'),
    read('../src/components/Navbar.vue'),
    read('../src/stores/wallet.js'),
  ])

  assert.match(floatingNavbar, /v-if="!wallet\.isConnected"/)
  assert.doesNotMatch(floatingNavbar, /v-if="!wallet\.isLoggedIn"/)
  assert.match(navbar, /if \(walletStore\.isConnected\)/)
  assert.match(navbar, /if \(walletStore\.isConnected\) return walletStore\.shortAddress/)
  assert.match(navbar, /showWalletMenu && walletStore\.isConnected/)
  assert.doesNotMatch(navbar, /showWalletMenu && walletStore\.isLoggedIn/)
  assert.match(walletStore, /Token-gated routes continue to use/)
  assert.doesNotMatch(walletStore, /Auth 失败[\s\S]{0,500}await wagmiDisconnect/)
})

test('site authentication remains token gated', async () => {
  const walletStore = await read('../src/stores/wallet.js')
  assert.match(walletStore, /return isConnected\.value && !!uid\.value && !!token\.value/)
})

test('connected-only position reads are restricted to an explicit local authority', async () => {
  const [api, panel] = await Promise.all([
    read('../src/services/api.js'),
    read('../src/components/FairLaunchReadOnlyPanel.vue'),
  ])
  assert.match(api, /export function hasFairLaunchLocalApiAuthority/)
  assert.match(api, /return exactLocalFairLaunchApiAuthority\(\) !== null/)
  assert.match(panel, /localAuthorityRead: hasFairLaunchLocalApiAuthority\(\)/)
})
