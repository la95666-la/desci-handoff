import test from 'node:test'
import assert from 'node:assert/strict'
import { realpathSync } from 'node:fs'
import { mkdtemp, readFile, readdir, rm, writeFile } from 'node:fs/promises'
import { spawnSync } from 'node:child_process'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

// Vite's HTML inline-proxy key on Windows uses the canonical drive casing.
// A test invoked through d:/la while the real volume is D:/LA must not split
// its HTML transform across two module identities.
const root = realpathSync.native(new URL('..', import.meta.url).pathname.replace(/^\/(?:[A-Za-z]:)/, value => value.slice(1)))
const sentinelAddress = '0x9999999999999999999999999999999999999999'
const viteCli = join(root, 'node_modules', 'vite', 'bin', 'vite.js')

async function filesUnder(path) {
  const output = []
  for (const entry of await readdir(path, { withFileTypes: true })) {
    const child = join(path, entry.name)
    if (entry.isDirectory()) output.push(...await filesUnder(child))
    else output.push(child)
  }
  return output
}

const cleanEnvironment = () => Object.fromEntries(Object.entries(process.env).filter(([key]) =>
  !/^VITE_FAIR_LAUNCH_LOCAL_/i.test(key) && key !== 'VITE_FAIR_LAUNCH_TEST_MODE'
))

const build = ({ cwd = root, env = {}, outDir = '' } = {}) => spawnSync(
  process.execPath,
  [viteCli, 'build', ...(outDir ? ['--outDir', outDir] : [])],
  {
    cwd,
    encoding: 'utf8',
    env: { ...cleanEnvironment(), VITE_ENVIRONMENT: 'prod', VITE_CHECKIN_CHAIN_ID: '56', ...env },
  },
)

const outputOf = result => `${result.stdout}\n${result.stderr}`

test('production build rejects local wallet and manifest markers from process environment', () => {
  const rejected = build({ env: {
    VITE_FAIR_LAUNCH_LOCAL_WALLET_SUBMISSION: 'true',
    VITE_FAIR_LAUNCH_LOCAL_PROXY_ADDRESS: sentinelAddress,
  } })
  assert.notEqual(rejected.status, 0)
  assert.match(outputOf(rejected), /local wallet submission or local manifest identity is forbidden/i)

  const rejectedManifestOnly = build({ env: {
    VITE_FAIR_LAUNCH_LOCAL_WALLET_SUBMISSION: 'false',
    VITE_FAIR_LAUNCH_LOCAL_RUN_ID: 'run-20260910-local-001',
  } })
  assert.notEqual(rejectedManifestOnly.status, 0)
  assert.match(outputOf(rejectedManifestOnly), /local wallet submission or local manifest identity is forbidden/i)
})

test('production guard scans loadEnv files without touching repository env files', async () => {
  const fixture = await mkdtemp(join(root, '.tmp-fair-launch-production-env-'))
  try {
    await writeFile(join(fixture, 'package.json'), JSON.stringify({ type: 'module' }))
    await writeFile(join(fixture, 'index.html'), '<div id="app"></div>')
    await writeFile(join(fixture, 'main.js'), 'document.querySelector("#app").textContent="fixture"')
    await writeFile(join(fixture, '.env.production'), [
      'VITE_ENVIRONMENT=prod',
      'VITE_CHECKIN_CHAIN_ID=56',
      'VITE_FAIR_LAUNCH_LOCAL_MANIFEST_PAYLOAD_SHA256=' + 'a'.repeat(64),
      'VITE_FAIR_LAUNCH_LOCAL_WALLET_SUBMISSION=false',
      '',
    ].join('\n'))
    await writeFile(join(fixture, '.env.production.local'), [
      'VITE_FAIR_LAUNCH_TEST_MODE=local_test',
      'VITE_FAIR_LAUNCH_LOCAL_FRONTEND_ORIGIN=http://127.0.0.1:4173',
      '',
    ].join('\n'))
    await writeFile(join(fixture, 'vite.config.js'), `export { default } from ${JSON.stringify(new URL('../vite.config.js', import.meta.url).href)}\n`)
    const rejected = build({ cwd: fixture })
    assert.notEqual(rejected.status, 0)
    assert.match(outputOf(rejected), /local wallet submission or local manifest identity is forbidden/i)
  } finally {
    await rm(fixture, { recursive: true, force: true })
  }
})

test('clean production build remains mainnet-only and contains no local sentinel identity', async () => {
  const outDir = await mkdtemp(join(tmpdir(), 'jc-fair-launch-prod-'))
  try {
    const built = build({ outDir, env: { VITE_FAIR_LAUNCH_LOCAL_WALLET_SUBMISSION: 'false' } })
    assert.equal(built.status, 0, outputOf(built))
    const output = (await Promise.all((await filesUnder(outDir)).map(file => readFile(file, 'utf8')))).join('\n')
    assert.equal(output.toLowerCase().includes(sentinelAddress.toLowerCase()), false)
    assert.equal(output.includes('RUN run-20260910-local-001'), false)
    assert.equal(output.includes('local-e2e-0123456789abcdef01234567'), false)
  } finally {
    await rm(outDir, { recursive: true, force: true })
  }
})
