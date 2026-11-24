import { Contract, formatUnits, getAddress } from 'ethers'
import { IDO_V3_ABI } from '../config/idoV3Abi.js'
import { assertV3Project, v3Error, v3SaleOpen } from './idoV3Wallet.js'

const tokenAbi = ['function balanceOf(address) view returns (uint256)',
  'function allowance(address,address) view returns (uint256)']
const same = (a, b) => String(a || '').toLowerCase() === String(b || '').toLowerCase()

// Presentation only. Never feed this snapshot into _v3Submit, proof acquisition
// or transaction authorization. Those paths retain fresh full runtime checks.
export async function readV3DisplaySnapshot(project, wallet, runtime, provider) {
  assertV3Project(project, runtime)
  if (Number((await provider.getNetwork()).chainId) !== project.chainId) throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE')
  const head = await provider.getBlock('latest')
  if (!head?.hash || !Number.isSafeInteger(head.number)) throw v3Error('IDO_V3_STATE_UNAVAILABLE')
  const options = { blockTag: head.number }
  const contract = new Contract(runtime.proxy, IDO_V3_ABI, provider)
  const [raw, policy] = await Promise.all([
    contract.getProject(BigInt(project.id), options),
    contract.getWhitelistPolicy(BigInt(project.id), options),
  ])
  if (BigInt(raw.projectId) !== BigInt(project.id) || !same(raw.projectToken, project.tokenAddress) ||
      !same(raw.paymentToken, project.paymentTokenAddress) ||
      Number(raw.projectTokenDecimals) !== project.tokenDecimals ||
      Number(raw.paymentTokenDecimals) !== project.paymentTokenDecimals ||
      (project.feeReceiver && !same(raw.feeReceiver, project.feeReceiver)) ||
      (project.ownerAddress && !same(raw.projectOwner, project.ownerAddress))) throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE')
  const status = Number(raw.status)
  if (!Number.isInteger(status) || status < 0 || status > 4) throw v3Error('IDO_V3_STATE_UNAVAILABLE')
  const state = { blockNumber: head.number, blockHash: head.hash, status, project: raw,
    enabled: policy.enabled, root: policy.root, version: String(policy.version),
    saleOpen: v3SaleOpen(status, raw.startTime, raw.endTime, head.timestamp) }
  const result = { state, position: null, balance: null, allowance: null, claimable: null,
    ownerFunding: null, errors: {} }
  if (wallet) {
    wallet = getAddress(wallet)
    const owner = status === 0 && same(raw.projectOwner, wallet)
    const token = new Contract(owner ? raw.projectToken : raw.paymentToken, tokenAbi, provider)
    const decimals = owner ? project.tokenDecimals : project.paymentTokenDecimals
    const reads = {
      position: async () => {
        const p = await contract.getUserPurchase(BigInt(project.id), wallet, options)
        return { paymentAmount: formatUnits(p.paymentAmount, project.paymentTokenDecimals), paymentAmountRaw: p.paymentAmount,
          totalTokensBought: formatUnits(p.tokensBought, project.tokenDecimals), totalTokensBoughtRaw: p.tokensBought,
          claimedTokens: formatUnits(p.claimedTokens, project.tokenDecimals), claimedTokensRaw: p.claimedTokens, refunded: p.refunded }
      },
    }
    if (status === 1 || owner) {
      reads.balance = async () => {
        const value = await token.balanceOf(wallet, options)
        return { raw: value, formatted: formatUnits(value, decimals) }
      }
      reads.allowance = async () => ({ raw: await token.allowance(wallet, runtime.proxy, options) })
    }
    if (status === 2) reads.claimable = async () => ({ raw: await contract.calculateClaimable(BigInt(project.id), wallet, options) })
    // A failed wallet field must not erase successfully read public terms or
    // turn a failed balance request into an apparent zero balance.
    await Promise.all(Object.entries(reads).map(async ([key, read]) => {
      try { result[key] = await read() }
      catch { result.errors[key] = 'IDO_V3_STATE_UNAVAILABLE' }
    }))
    if (owner && result.balance && result.allowance) {
      result.ownerFunding = { balance: result.balance.raw, allowance: result.allowance.raw }
    }
  }
  const anchored = await provider.getBlock(head.number)
  if (!same(anchored?.hash, head.hash)) throw v3Error('IDO_V3_STATE_UNAVAILABLE')
  return result
}

// Coalesce only in-flight presentation requests, never cache transaction state.
export function createDisplaySingleFlight() {
  const pending = new Map()
  return (key, read) => {
    if (pending.has(key)) return pending.get(key)
    const request = Promise.resolve().then(read).finally(() => {
      if (pending.get(key) === request) pending.delete(key)
    })
    pending.set(key, request)
    return request
  }
}

export async function withDisplayDeadline(read, timeoutMs = 15000) {
  let timer
  try {
    return await Promise.race([
      Promise.resolve().then(read),
      new Promise((_, reject) => { timer = setTimeout(() => reject(v3Error('IDO_V3_STATE_UNAVAILABLE')), timeoutMs) }),
    ])
  } finally { clearTimeout(timer) }
}
