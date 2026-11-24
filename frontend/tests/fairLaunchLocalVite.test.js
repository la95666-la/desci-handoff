import test from 'node:test'
import assert from 'node:assert/strict'
import { parseFairLaunchLocalEnv } from '../scripts/fair-launch-local-vite.js'

test('local Vite env parser preserves deployment identity JSON as inert data', () => {
  const json = '[{"address":"0x1111111111111111111111111111111111111111","identity":"asset.paymentToken"}]'
  const parsed = parseFairLaunchLocalEnv(`# generated\r\nVITE_ENVIRONMENT=local\r\nVITE_FAIR_LAUNCH_LOCAL_DEPLOYMENT_IDENTITIES_JSON=${json}\r\n`)
  assert.equal(parsed.VITE_ENVIRONMENT, 'local')
  assert.equal(parsed.VITE_FAIR_LAUNCH_LOCAL_DEPLOYMENT_IDENTITIES_JSON, json)
  assert.deepEqual(JSON.parse(parsed.VITE_FAIR_LAUNCH_LOCAL_DEPLOYMENT_IDENTITIES_JSON), JSON.parse(json))
})

test('local Vite env parser never evaluates shell syntax and rejects malformed authority input', () => {
  const marker = '$(touch should-never-execute)'
  assert.equal(parseFairLaunchLocalEnv(`VITE_SENTINEL=${marker}\n`).VITE_SENTINEL, marker)
  for (const input of ['PATH=/tmp\n', 'VITE_A=1\nVITE_A=2\n', 'VITE_A\n']) {
    assert.throws(() => parseFairLaunchLocalEnv(input))
  }
})
