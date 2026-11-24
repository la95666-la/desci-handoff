import { readFileSync } from 'node:fs'
import { spawn } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'

const ENV_KEY = /^VITE_[A-Z0-9_]+$/

export function parseFairLaunchLocalEnv(text) {
  const values = {}
  for (const rawLine of String(text).split(/\r?\n/)) {
    if (!rawLine || rawLine.startsWith('#')) continue
    const separator = rawLine.indexOf('=')
    if (separator < 1) throw new Error('invalid Fair Launch local env line')
    const key = rawLine.slice(0, separator)
    const value = rawLine.slice(separator + 1)
    if (!ENV_KEY.test(key) || Object.hasOwn(values, key)) throw new Error('invalid Fair Launch local env key')
    if (value.includes('\0') || value.includes('\n') || value.includes('\r')) throw new Error('invalid Fair Launch local env value')
    values[key] = value
  }
  return values
}

export function startFairLaunchLocalVite(envPath, inherited = process.env) {
  if (!envPath) throw new Error('usage: node scripts/fair-launch-local-vite.js <absolute-env-file>')
  const values = parseFairLaunchLocalEnv(readFileSync(envPath, 'utf8'))
  const child = spawn(process.execPath, [resolve('node_modules/vite/bin/vite.js'), '--host', '127.0.0.1', '--port', '15170', '--strictPort'], {
    env: { ...inherited, ...values },
    stdio: 'inherit',
    windowsHide: true,
  })
  child.on('error', (error) => { console.error(error); process.exitCode = 1 })
  child.on('exit', (code, signal) => { process.exitCode = signal ? 1 : (code ?? 1) })
  const stop = (signal) => { if (!child.killed) child.kill(signal) }
  process.once('SIGINT', () => stop('SIGINT'))
  process.once('SIGTERM', () => stop('SIGTERM'))
  return child
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(fileURLToPath(import.meta.url))) {
  startFairLaunchLocalVite(process.argv[2])
}
