import { keccak256 } from 'ethers'
const v3Error = (code) => Object.assign(new Error(code), { code })

const key = 'jc-ido-v3-wallet-pending-v1'

function store() {
  if (typeof window === 'undefined' || !window.localStorage) throw v3Error('IDO_V3_STATE_UNAVAILABLE')
  return window.localStorage
}

export function v3SubmissionIdentity(context, wallet, action) {
  return `${context.chainId}:${context.contractAddress.toLowerCase()}:${context.id}:${wallet.toLowerCase()}:${action}`
}

function read() {
  try {
    const value = JSON.parse(store().getItem(key) || '{}')
    if (!value || typeof value !== 'object' || Array.isArray(value)) throw Error()
    return value
  } catch { throw v3Error('IDO_V3_STATE_UNAVAILABLE') }
}

export function assertNoV3Pending(identity) {
  if (read()[identity]) throw v3Error('IDO_V3_SUBMISSION_UNRESOLVED')
}

export function listV3Pending(context, wallet) {
  const prefix = `${context.chainId}:${context.contractAddress.toLowerCase()}:${context.id}:${wallet.toLowerCase()}:`
  return Object.entries(read()).filter(([identity]) => identity.startsWith(prefix))
}

export function recordV3Pending(identity, hash = '', meta = null) {
  const entries = read()
  if (entries[identity] && !hash) throw v3Error('IDO_V3_SUBMISSION_UNRESOLVED')
  if (!entries[identity] && (!meta?.to || !meta?.from || !meta?.dataHash)) throw v3Error('IDO_V3_STATE_UNAVAILABLE')
  entries[identity] = { ...(entries[identity] || meta), hash, at: entries[identity]?.at || Date.now() }
  try { store().setItem(key, JSON.stringify(entries)) } catch { throw v3Error('IDO_V3_STATE_UNAVAILABLE') }
}

export function v3PendingMetadata(to, from, data) {
  return { to: to.toLowerCase(), from: from.toLowerCase(), dataHash: keccak256(data) }
}

export function v3PendingSnapshot(context, wallet) {
  const prefix = v3SubmissionIdentity(context, wallet, '')
  return listV3Pending(context, wallet).map(([identity, entry]) => ({ hash: entry.hash || '', at: entry.at, action: identity.slice(prefix.length) }))
}

export function clearV3Pending(identity) {
  const entries = read()
  delete entries[identity]
  try { store().setItem(key, JSON.stringify(entries)) } catch { throw v3Error('IDO_V3_STATE_UNAVAILABLE') }
}

// Compare-and-delete only this invocation's unbroadcast placeholder. An old
// hashless record (or a concurrent hash write) must never be cleared by a cancel.
export function releaseV3Unbroadcast(identity, requestId) {
  const entries = read()
  if (!requestId || entries[identity]?.requestId !== requestId || entries[identity]?.hash) return false
  delete entries[identity]
  try { store().setItem(key, JSON.stringify(entries)) } catch { throw v3Error('IDO_V3_STATE_UNAVAILABLE') }
  return true
}
