import { AbiCoder, Contract, getAddress, id, keccak256, solidityPackedKeccak256 } from 'ethers'
import { IDO_V3_ABI } from '../config/idoV3Abi.js'
import { getIdoV3Proof } from './api.js'

const env = import.meta.env || {}
const addressPattern = /^0x[0-9a-fA-F]{40}$/
const hashPattern = /^0x[0-9a-fA-F]{64}$/
const positiveDecimal = /^[1-9][0-9]*$/
const implementationSlot = '0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc'
const versionID = id('joincare.ido_v3_stablecoin.v1')

export function v3Error(code) {
  const error = new Error(code)
  error.code = code
  return error
}

const same = (a, b) => String(a || '').toLowerCase() === String(b || '').toLowerCase()
const validAddress = (value) => addressPattern.test(String(value || '')) && !/^0x0{40}$/i.test(value)
const validHash = (value) => hashPattern.test(String(value || '')) && !/^0x0{64}$/i.test(value)

// An independently pinned build identity is required for writes. Public
// project JSON and wallet RPC may narrow it but cannot create authority.
export function pinnedV3WalletRuntime(config = env) {
  const chainId = Number(config.VITE_IDO_V3_CHAIN_ID)
  const proxy = config.VITE_IDO_V3_PROXY
  const implementation = config.VITE_IDO_V3_IMPLEMENTATION
  const proxyCodeHash = config.VITE_IDO_V3_PROXY_CODE_HASH
  const implementationCodeHash = config.VITE_IDO_V3_IMPLEMENTATION_CODE_HASH
  const confirmations = Number(config.VITE_IDO_V3_CONFIRMATIONS)
  if (!Number.isSafeInteger(chainId) || ![56, 97, 31337].includes(chainId) ||
      !Number.isSafeInteger(confirmations) || confirmations < 2 || confirmations > 100 ||
      !validAddress(proxy) || !validAddress(implementation) || same(proxy, implementation) ||
      !validHash(proxyCodeHash) || !validHash(implementationCodeHash) ||
      (chainId === 56 && config.VITE_ENVIRONMENT !== 'prod') ||
      (chainId === 97 && config.VITE_ENVIRONMENT !== 'test') ||
      (chainId === 31337 && config.VITE_ENVIRONMENT !== 'local') ||
      (chainId === 31337 && !/^http:\/\/127\.0\.0\.1:[0-9]+\/$/.test(String(config.VITE_IDO_V3_LOCAL_RPC || '')))) throw v3Error('IDO_V3_ROUTING_UNAVAILABLE')
  return { chainId, proxy: getAddress(proxy), implementation: getAddress(implementation), proxyCodeHash, implementationCodeHash, confirmations }
}

export function assertV3Project(project, runtime) {
  if (project?.fundingMode !== 'capped_presale' || project?.contractVersion !== 'ido_v3_stablecoin' ||
      (typeof project.id !== 'string' && typeof project.id !== 'bigint') || !positiveDecimal.test(String(project.id)) ||
      Number(project.chainId) !== runtime.chainId ||
      !same(project.contractAddress, runtime.proxy) || !validAddress(project.paymentTokenAddress) ||
      !validAddress(project.tokenAddress) || same(project.paymentTokenAddress, project.tokenAddress) ||
      (project.ownerAddress && !validAddress(project.ownerAddress)) ||
      (project.feeReceiver && !validAddress(project.feeReceiver)) ||
      same(project.paymentTokenAddress, runtime.proxy) || same(project.tokenAddress, runtime.proxy) ||
      project.onchainLocked !== true || !Number.isInteger(project.paymentTokenDecimals) ||
      project.paymentTokenDecimals < 1 || project.paymentTokenDecimals > 18 ||
      !Number.isInteger(project.tokenDecimals) || project.tokenDecimals < 1 || project.tokenDecimals > 18) throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE')
  return project
}

function decodeProject(raw, project) {
  if (BigInt(raw.projectId) !== BigInt(project.id) || !same(raw.paymentToken, project.paymentTokenAddress) ||
      !same(raw.projectToken, project.tokenAddress) || Number(raw.paymentTokenDecimals) !== project.paymentTokenDecimals ||
      Number(raw.projectTokenDecimals) !== project.tokenDecimals ||
      !validAddress(raw.projectOwner) || !validAddress(raw.feeReceiver) ||
      (project.ownerAddress && !same(raw.projectOwner, project.ownerAddress)) ||
      (project.feeReceiver && !same(raw.feeReceiver, project.feeReceiver))) throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE')
  return raw
}

export function v3SaleOpen(status, startTime, endTime, timestamp) {
  try {
    return Number(status) === 1 && BigInt(timestamp) >= BigInt(startTime) && BigInt(timestamp) < BigInt(endTime)
  } catch { return false }
}

// No cached proof or state: all reads are from a pinned, independent read
// provider at one block. Header and tip checks close a changed-block window.
export async function readV3WalletState(project, runtime, provider) {
  assertV3Project(project, runtime)
  try {
    if (Number((await provider.getNetwork()).chainId) !== runtime.chainId) throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE')
    const head = await provider.getBlock('latest')
    if (!head || !Number.isSafeInteger(head.number) || !validHash(head.hash)) throw v3Error('IDO_V3_STATE_UNAVAILABLE')
    const blockTag = head.number
    const [proxyCode, slot, implementationCode, paymentCode, projectTokenCode] = await Promise.all([
      provider.getCode(runtime.proxy, blockTag), provider.getStorage(runtime.proxy, implementationSlot, blockTag),
      provider.getCode(runtime.implementation, blockTag), provider.getCode(project.paymentTokenAddress, blockTag),
      provider.getCode(project.tokenAddress, blockTag),
    ])
    if (proxyCode === '0x' || implementationCode === '0x' || paymentCode === '0x' || projectTokenCode === '0x' ||
        !same(keccak256(proxyCode), runtime.proxyCodeHash) || !same(keccak256(implementationCode), runtime.implementationCodeHash) ||
        !hashPattern.test(slot) || !same(`0x${slot.slice(26)}`, runtime.implementation)) throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE')
    const contract = new Contract(runtime.proxy, IDO_V3_ABI, provider)
    const paymentToken = new Contract(project.paymentTokenAddress, ['function decimals() view returns (uint8)'], provider)
    const projectToken = new Contract(project.tokenAddress, ['function decimals() view returns (uint8)'], provider)
    const [version, rawPolicy, rawProject, paymentDecimals, projectTokenDecimals] = await Promise.all([
      contract.contractVersion({ blockTag }), contract.getWhitelistPolicy(BigInt(project.id), { blockTag }),
      contract.getProject(BigInt(project.id), { blockTag }), paymentToken.decimals({ blockTag }), projectToken.decimals({ blockTag }),
    ])
    if (!same(version, versionID) || Number(paymentDecimals) !== project.paymentTokenDecimals ||
        Number(projectTokenDecimals) !== project.tokenDecimals) throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE')
    const chainProject = decodeProject(rawProject, project)
    const enabled = rawPolicy.enabled === true
    const root = String(rawPolicy.root).toLowerCase()
    const policyVersion = BigInt(rawPolicy.version)
    if (!hashPattern.test(root) || enabled === /^0x0{64}$/.test(root) || (enabled && policyVersion === 0n) || policyVersion > (1n << 64n) - 1n ||
        !Number.isInteger(Number(chainProject.status)) || Number(chainProject.status) < 0 || Number(chainProject.status) > 4) throw v3Error('IDO_V3_STATE_UNAVAILABLE')
    const again = await provider.getBlock(blockTag)
    if (!again || !same(again.hash, head.hash) || (await provider.getBlockNumber()) < blockTag) throw v3Error('IDO_V3_STATE_UNAVAILABLE')
    const saleOpen = v3SaleOpen(chainProject.status, chainProject.startTime, chainProject.endTime, head.timestamp)
    return { blockNumber: blockTag, blockHash: head.hash, enabled, root, version: policyVersion.toString(), status: Number(chainProject.status), saleOpen, project: chainProject, contract }
  } catch (error) {
    if (String(error?.code || '').startsWith('IDO_V3_')) throw error
    throw v3Error('IDO_V3_STATE_UNAVAILABLE')
  }
}

export function assertV3Proof(proof, project, state, wallet) {
  if (!proof || !state.enabled || !validAddress(wallet) || !positiveDecimal.test(String(proof.version)) ||
      !same(proof.wallet, wallet) || !same(proof.proxy, project.contractAddress) ||
      String(proof.chain_id) !== String(project.chainId) || String(proof.project_id) !== String(project.id) ||
      String(proof.version) !== state.version || !same(proof.root, state.root) ||
      !Array.isArray(proof.proof) || proof.proof.length > 32 || proof.proof.some((hash) => !hashPattern.test(hash))) {
    throw v3Error('IDO_V3_PROOF_UNAVAILABLE')
  }
  const leaf = keccak256(AbiCoder.defaultAbiCoder().encode(
    ['bytes32', 'uint256', 'address', 'uint256', 'address'],
    [id('JOINCARE_IDO_V3_WHITELIST_LEAF_V1'), BigInt(project.chainId), project.contractAddress, BigInt(project.id), wallet],
  ))
  let node = leaf
  for (const sibling of proof.proof) {
    node = BigInt(node) < BigInt(sibling)
      ? solidityPackedKeccak256(['bytes32', 'bytes32'], [node, sibling])
      : solidityPackedKeccak256(['bytes32', 'bytes32'], [sibling, node])
  }
  if (!same(node, state.root)) throw v3Error('IDO_V3_PROOF_UNAVAILABLE')
  return proof.proof
}

export async function assertV3PolicyUnchanged(previous, current, provider) {
  if (!previous || !current || current.blockNumber < previous.blockNumber ||
      current.version !== previous.version || current.root !== previous.root ||
      current.enabled !== previous.enabled || current.status !== previous.status) throw v3Error('IDO_V3_POLICY_CHANGED')
  try {
    const oldBlock = await provider.getBlock(previous.blockNumber)
    if (!oldBlock || !same(oldBlock.hash, previous.blockHash)) throw v3Error('IDO_V3_POLICY_CHANGED')
  } catch { throw v3Error('IDO_V3_POLICY_CHANGED') }
  return current
}

// A submission-local final identity fence, not another full business snapshot.
// Never cache this across operations or substitute a display snapshot. Policy
// only matters for purchase/approval; claim and refund do not consume a proof.
export async function assertV3SendIdentity(project, state, provider, checkPolicy, runtime = pinnedV3WalletRuntime()) {
  assertV3Project(project, runtime)
  try {
    const head = await provider.getBlock('latest')
    if (!head || !Number.isSafeInteger(head.number) || head.number < state.blockNumber || !validHash(head.hash)) throw Error()
    const blockTag = head.number
    const [proxyCode, slot, code, anchor, policy] = await Promise.all([
      provider.getCode(runtime.proxy, blockTag), provider.getStorage(runtime.proxy, implementationSlot, blockTag),
      provider.getCode(runtime.implementation, blockTag), provider.getBlock(state.blockNumber),
      checkPolicy ? new Contract(runtime.proxy, IDO_V3_ABI, provider).getWhitelistPolicy(BigInt(project.id), { blockTag }) : null,
    ])
    if (!same(keccak256(proxyCode), runtime.proxyCodeHash) || !same(keccak256(code), runtime.implementationCodeHash) ||
        !hashPattern.test(slot) || !same(`0x${slot.slice(26)}`, runtime.implementation) ||
        !same(anchor?.hash, state.blockHash) || !same((await provider.getBlock(blockTag))?.hash, head.hash)) {
      throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE')
    }
    if (checkPolicy && (policy.enabled !== state.enabled || !same(policy.root, state.root) ||
        BigInt(policy.version).toString() !== state.version)) throw v3Error('IDO_V3_POLICY_CHANGED')
  } catch (error) {
    if (String(error?.code || '').startsWith('IDO_V3_')) throw error
    throw v3Error('IDO_V3_STATE_UNAVAILABLE')
  }
}

export async function v3PurchaseAuthority(project, wallet, provider, config = env, fetchProof = getIdoV3Proof, readState = readV3WalletState) {
  const runtime = pinnedV3WalletRuntime(config)
  const state = await readState(project, runtime, provider)
  if (!state.saleOpen) throw v3Error('IDO_V3_PURCHASE_UNAVAILABLE')
  if (!state.enabled) return { runtime, state, proof: null }
  let proof
  try { proof = await fetchProof(project.id, wallet) } catch { throw v3Error('IDO_V3_PROOF_UNAVAILABLE') }
  return { runtime, state, proof: assertV3Proof(proof, project, state, wallet) }
}

export function v3ProjectActionState(project, state, action, wallet, runtime = pinnedV3WalletRuntime()) {
  assertV3Project(project, runtime)
  if (!state || !validAddress(wallet)) throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE')
  if (action === 'depositProjectTokens') {
    if (state.status !== 0 || !same(state.project.projectOwner, wallet)) throw v3Error('IDO_V3_ACTION_UNAVAILABLE')
  } else if (action === 'withdrawProjectTokens') {
    if (![2, 3, 4].includes(state.status) || !same(state.project?.projectOwner, wallet) ||
        state.project.projectTokensWithdrawn !== false) throw v3Error('IDO_V3_ACTION_UNAVAILABLE')
    const deposited = BigInt(state.project.totalTokensDeposited)
    const reserved = state.status === 2 ? BigInt(state.project.totalTokensSold) : 0n
    if (deposited <= reserved) throw v3Error('IDO_V3_ACTION_UNAVAILABLE')
  } else if (action === 'withdrawRaisedFunds') {
    if (state.status !== 2 || !same(state.project?.projectOwner, wallet) || state.project.fundsWithdrawn !== false) {
      throw v3Error('IDO_V3_ACTION_UNAVAILABLE')
    }
    const raised = BigInt(state.project.totalRaised)
    const feeRate = BigInt(state.project.successFeeRate)
    if (raised <= 0n || feeRate < 0n || feeRate > 10000n || raised - raised * feeRate / 10000n <= 0n) {
      throw v3Error('IDO_V3_ACTION_UNAVAILABLE')
    }
  } else if (action === 'claim') {
    if (state.status !== 2) throw v3Error('IDO_V3_ACTION_UNAVAILABLE')
  } else if (action === 'refund') {
    if (state.status !== 3 && state.status !== 4) throw v3Error('IDO_V3_ACTION_UNAVAILABLE')
  } else throw v3Error('IDO_V3_ACTION_UNAVAILABLE')
  return state
}
