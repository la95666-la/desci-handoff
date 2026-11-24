const STORAGE_KEY = 'jc.fair-launch.wallet-submission-cache.v2'

function storage() {
  try { return typeof localStorage === 'undefined' ? null : localStorage } catch { return null }
}

function key(binding) {
  return [binding.wallet, binding.chainId, binding.target, binding.projectId, binding.action, binding.calldata, JSON.stringify(binding.stateBinding ?? null)]
    .map((value) => String(value ?? '').toLowerCase()).join('|')
}

function entries() {
  const raw = storage()?.getItem(STORAGE_KEY)
  if (!raw) return {}
  try { const value = JSON.parse(raw); return value && typeof value === 'object' ? value : {} } catch { return {} }
}

function save(value) { storage()?.setItem(STORAGE_KEY, JSON.stringify(value)) }

// This module is deliberately only a non-authoritative UI cache. The Backend
// durable submission service is unavailable in this release, so public action
// descriptors disable every Fair Launch wallet write before these helpers can
// be reached. No client API may clear an unresolved safety record.
export function recordFairLaunchUnresolved(binding, transactionHash = '') {
  const all = entries()
  all[key(binding)] = { transactionHash: String(transactionHash || ''), observedAt: new Date().toISOString(), authoritative: false }
  save(all)
  return all[key(binding)]
}

export function findFairLaunchUnresolved(binding) { return entries()[key(binding)] || null }

export function assertFairLaunchNotUnresolved(binding) {
  if (findFairLaunchUnresolved(binding)) throw new Error('Fair Launch transaction remains unresolved. Do not resend; Backend canonical resolution is required.')
}
