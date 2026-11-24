/**
 * 合約交互服務層
 * 
 * 封裝與 JoinCare IDO 合約和 JCC 代幣合約的所有交互。
 * 依賴 web3Service 提供 provider/signer。
 */

import { Contract, Interface, JsonRpcProvider, formatUnits, getAddress, keccak256, parseUnits, randomBytes, hexlify } from 'ethers'
import { web3Service } from './web3.js'
import { comparePurchaseRpcPaths } from './purchaseDiagnostics.js'
import { isV2Project, normalizeProjectContext } from './projectContext.js'
import { fairLaunchActionAvailable, resolveProjectFundingRoute } from '../utils/fairLaunch.js'
import { FAIR_LAUNCH_V1_ABI } from '../config/fairLaunchAbi.js'
import { WALLET_ERROR_CODES, isNetworkMismatchError } from './errors.js'
import { calculateFairLaunchRemainingRaw } from './fairLaunchWallet.js'
import { assertFairLaunchNotUnresolved, findFairLaunchUnresolved, recordFairLaunchUnresolved } from './fairLaunchUnresolved.js'
import { currentFairLaunchSubmissionPolicy, verifyFairLaunchLocalDeployment } from './fairLaunchSubmissionPolicy.js'
import { applyFairLaunchLocalTransaction, getProjectDetail, getVerifiedV3Wallet, getIdoV3Proof } from './api.js'
import { assertV3PolicyUnchanged, assertV3Proof, assertV3Project, pinnedV3WalletRuntime, readV3WalletState, v3ProjectActionState, v3PurchaseAuthority, v3Error } from './idoV3Wallet.js'
import { IDO_V3_ABI } from '../config/idoV3Abi.js'
import { readV3DisplaySnapshot, withDisplayDeadline } from './idoV3Display.js'
import { simulateV3Transaction } from './idoV3Simulation.js'
import { v3WalletPayload, v3WalletErrorKind } from './v3WalletCompatibility.js'
import { isV3WalletEditedApproval } from './idoV3ApprovalReceipt.js'
import { assertNoV3Pending, clearV3Pending, releaseV3Unbroadcast, listV3Pending, recordV3Pending, v3PendingMetadata, v3PendingSnapshot, v3SubmissionIdentity } from './idoV3Pending.js'
import {
    ADDRESSES,
    ERC20_ABI,
    JOINCARE_ABI,
    JOINCARE_V2_ABI,
    MAX_UINT256,
    ACTIVE_CHAIN,
} from '../config/contracts.js'

/** EVM transaction hashes are always 32 bytes (0x + 64 hex characters). */
export function isValidTransactionHash(value) {
    return typeof value === 'string' && /^0x[0-9a-fA-F]{64}$/.test(value)
}

function requireValidTransactionHash(value) {
    if (!isValidTransactionHash(value)) {
        const error = new Error(`Wallet returned an invalid transaction hash: ${String(value)}`)
        error.code = WALLET_ERROR_CODES.TRANSACTION_BROADCAST_UNCERTAIN
        throw error
    }
    return value
}

function gasEstimateTimeoutError() {
    const error = new Error('Gas estimation timed out.')
    error.code = 'GAS_ESTIMATE_TIMEOUT'
    return error
}

const TRUSTED_GAS_RPC_BY_CHAIN = Object.freeze({
    56: 'https://bsc-dataseed.bnbchain.org',
    97: 'https://data-seed-prebsc-1-s1.bnbchain.org:8545',
})

export function trustedGasRpcUrl(chainId) {
    return TRUSTED_GAS_RPC_BY_CHAIN[Number(chainId)] || ''
}

/**
 * Read-only primary gas simulation for trusted BSC chains. It validates the
 * independent endpoint's chain, runs the identical eth_call, and then estimates
 * the identical unsigned transaction. It never signs or broadcasts and never
 * returns arbitrary RPC text to the caller.
 */
export async function estimateGasViaTrustedRpc(transaction, expectedChainId, timeoutMs = 8000, fetcher = fetch) {
    const rpcUrl = trustedGasRpcUrl(expectedChainId)
    if (!rpcUrl) return null
    if (!transaction || !/^0x[0-9a-fA-F]{40}$/.test(String(transaction.from || ''))
        || !/^0x[0-9a-fA-F]{40}$/.test(String(transaction.to || ''))
        || !/^0x[0-9a-fA-F]*$/.test(String(transaction.data || ''))
        || transaction.value !== 0n) {
        const error = new Error('Trusted RPC transaction identity is invalid')
        error.code = 'TRUSTED_RPC_IDENTITY_INVALID'
        throw error
    }
    const controller = typeof AbortController === 'function' ? new AbortController() : null
    let timer
    const deadline = new Promise((_, reject) => {
        timer = setTimeout(() => {
            controller?.abort()
            const error = new Error('Trusted RPC gas simulation timed out')
            error.code = 'TRUSTED_RPC_TIMEOUT'
            reject(error)
        }, timeoutMs)
    })
    const bounded = (promise) => Promise.race([promise, deadline])
    let requestId = 0
    const rpc = async (method, params) => {
        let response
        try {
            response = await bounded(fetcher(rpcUrl, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ jsonrpc: '2.0', id: ++requestId, method, params }),
                signal: controller?.signal,
            }))
        } catch (cause) {
            if (cause?.code === 'TRUSTED_RPC_TIMEOUT') throw cause
            const error = new Error('Trusted RPC is unavailable')
            error.code = 'TRUSTED_RPC_UNAVAILABLE'
            throw error
        }
        if (!response?.ok) {
            const error = new Error('Trusted RPC is unavailable')
            error.code = 'TRUSTED_RPC_UNAVAILABLE'
            throw error
        }
        let payload
        try {
            payload = await bounded(response.json())
        } catch (cause) {
            if (cause?.code === 'TRUSTED_RPC_TIMEOUT') throw cause
            const error = new Error('Trusted RPC is unavailable')
            error.code = 'TRUSTED_RPC_UNAVAILABLE'
            throw error
        }
        if (payload?.error) {
            const error = new Error('Trusted RPC rejected the read-only simulation')
            const rpcCode = Number(payload.error.code)
            const rpcMessage = String(payload.error.message || '').toLowerCase()
            error.code = rpcCode === -32601 || rpcMessage.includes('method not found') || rpcMessage.includes('unsupported method')
                ? 'TRUSTED_RPC_METHOD_UNSUPPORTED'
                : 'TRUSTED_RPC_SIMULATION_FAILED'
            throw error
        }
        if (!Object.hasOwn(payload || {}, 'result')) {
            const error = new Error('Trusted RPC is unavailable')
            error.code = 'TRUSTED_RPC_UNAVAILABLE'
            throw error
        }
        return payload.result
    }

    try {
        const chainHex = await rpc('eth_chainId', [])
        if (!/^0x[0-9a-fA-F]+$/.test(String(chainHex || '')) || Number(BigInt(chainHex)) !== Number(expectedChainId)) {
            const error = new Error('Trusted RPC chain does not match the project')
            error.code = 'TRUSTED_RPC_CHAIN_MISMATCH'
            throw error
        }
        const rpcTransaction = {
            from: transaction.from,
            to: transaction.to,
            data: transaction.data,
            value: '0x0',
        }
        await rpc('eth_call', [rpcTransaction, 'latest'])
        const estimateHex = await rpc('eth_estimateGas', [rpcTransaction])
        if (!/^0x[0-9a-fA-F]+$/.test(String(estimateHex || ''))) {
            const error = new Error('Trusted RPC returned an invalid gas estimate')
            error.code = 'TRUSTED_RPC_ESTIMATE_INVALID'
            throw error
        }
        const gasLimit = addGasSafetyBuffer(BigInt(estimateHex))
        if (gasLimit <= 0n || gasLimit > 1000000n) {
            const error = new Error('Trusted RPC gas estimate exceeded the safety cap')
            error.code = 'TRUSTED_RPC_ESTIMATE_INVALID'
            throw error
        }
        return gasLimit
    } finally {
        clearTimeout(timer)
        controller?.abort()
    }
}

function traceFairLaunchLocalWallet(stage, details = {}) {
    if (import.meta.env?.DEV !== true || import.meta.env?.VITE_ENVIRONMENT !== 'local' || typeof window === 'undefined') return
    const previousStage = window.__JC_FL_WALLET_TRACE?.stage || ''
    const trace = {
        stage,
        previousStage,
        at: new Date().toISOString(),
        ...details,
    }
    window.__JC_FL_WALLET_TRACE = trace
    try {
        const history = JSON.parse(sessionStorage.getItem('jc-fl-local-wallet-trace') || '[]')
        const rows = Array.isArray(history) ? history.slice(-19) : []
        rows.push(trace)
        sessionStorage.setItem('jc-fl-local-wallet-trace', JSON.stringify(rows))
    } catch {}
}

/** Only an explicit unsupported-method response proves the raw request was not broadcast. */
export function canSafelyFallbackRawSend(error) {
    const code = Number(error?.code)
    const message = String(error?.shortMessage || error?.message || '').toLowerCase()
    return code === -32601 || message.includes('method not found') || message.includes('unsupported method')
}

/**
 * A failed estimate caused by a contract revert must stop the transaction.
 * Fallback is allowed only when the RPC cannot perform eth_estimateGas at all.
 */
export function canSafelyFallbackGasEstimate(error) {
    const code = error?.code
    const numericCode = Number(code)
    const message = String(error?.shortMessage || error?.message || '').toLowerCase()
    return numericCode === -32601
        || message.includes('method not found')
        || message.includes('unsupported method')
        || message.includes('eth_estimategas is not available')
}

/**
 * The trusted RPC is the primary read-only simulator on supported BSC chains.
 * Fall back to the wallet only for transport/service availability failures.
 * Chain mismatches, contract reverts and invalid estimates remain fail-closed.
 */
export function canSafelyFallbackTrustedGasRpc(error) {
    return [
        'TRUSTED_RPC_TIMEOUT',
        'TRUSTED_RPC_UNAVAILABLE',
        'TRUSTED_RPC_METHOD_UNSUPPORTED',
    ].includes(String(error?.code || ''))
}

/** Add a 25% safety margin and round up to the next whole gas unit. */
export function addGasSafetyBuffer(estimatedGas, bufferBps = 2500n) {
    const estimate = BigInt(estimatedGas)
    const bps = BigInt(bufferBps)
    if (estimate <= 0n || bps < 0n) throw new Error('Invalid gas estimate')
    return (estimate * (10000n + bps) + 9999n) / 10000n
}

export function isSuccessfulReceipt(receipt) {
    if (!receipt) return false
    try {
        return BigInt(receipt.status) === 1n
    } catch {
        return false
    }
}

function requireSuccessfulReceipt(receipt, txHash) {
    if (!isSuccessfulReceipt(receipt)) {
        const error = new Error(`Transaction reverted: ${txHash}`)
        error.code = WALLET_ERROR_CODES.TRANSACTION_REVERTED
        error.transactionHash = txHash
        throw error
    }
    return receipt
}

class ContractService {
    constructor({ fairLaunchSubmissionPolicy = currentFairLaunchSubmissionPolicy } = {}) {
        this._fairLaunchSubmissionPolicy = fairLaunchSubmissionPolicy
        this._trustedReadProviders = new Map()
    }

    // ============ 合約實例獲取 ============

    /** JoinCare 合約（只讀） */
    _getJoinCare() {
        if (!web3Service.provider) throw new Error('Wallet not connected')
        return new Contract(ADDRESSES.JoinCare, JOINCARE_ABI, web3Service.provider)
    }

    /** JoinCare 合約（可寫） */
    _getJoinCareSigner() {
        if (!web3Service.signer) throw new Error('Wallet not connected')
        return new Contract(ADDRESSES.JoinCare, JOINCARE_ABI, web3Service.signer)
    }

    /** JCC 合約（只讀） */
    _getJCC() {
        if (!web3Service.provider) throw new Error('Wallet not connected')
        return new Contract(ADDRESSES.JCC, ERC20_ABI, web3Service.provider)
    }

    /** JCC 合約（可寫） */
    _getJCCSigner() {
        if (!web3Service.signer) throw new Error('Wallet not connected')
        return new Contract(ADDRESSES.JCC, ERC20_ABI, web3Service.signer)
    }

    /** 任意 ERC20（只讀） */
    _getERC20(tokenAddress) {
        if (!web3Service.provider) throw new Error('Wallet not connected')
        return new Contract(tokenAddress, ERC20_ABI, web3Service.provider)
    }

    /** 任意 ERC20（可寫） */
    _getERC20Signer(tokenAddress) {
        if (!web3Service.signer) throw new Error('Wallet not connected')
        return new Contract(tokenAddress, ERC20_ABI, web3Service.signer)
    }

    _isV2(project) {
        return isV2Project(project)
    }

    _projectContext(project) {
        return normalizeProjectContext(project, {
            chainId: ACTIVE_CHAIN.chainId,
            contractAddress: ADDRESSES.JoinCare,
            paymentTokenAddress: ADDRESSES.JCC
        })
    }

    /**
     * Public chain reads must not inherit the wallet application's RPC route.
     * On supported BSC chains use the fixed JoinCare read-only endpoint; the
     * wallet provider remains authoritative only for accounts, chain identity,
     * signatures and final transaction broadcast. Local/unrecognised chains
     * retain their explicitly connected provider.
     */
    _readRunner(chainId) {
        const rpcUrl = trustedGasRpcUrl(chainId)
        if (!rpcUrl) {
            if (!web3Service.provider) throw new Error('Wallet not connected')
            return web3Service.provider
        }
        const normalizedChainId = Number(chainId)
        if (!this._trustedReadProviders.has(normalizedChainId)) {
            this._trustedReadProviders.set(
                normalizedChainId,
                new JsonRpcProvider(rpcUrl, normalizedChainId, { staticNetwork: true }),
            )
        }
        return this._trustedReadProviders.get(normalizedChainId)
    }

    _v3Context(project) {
        const context = this._projectContext(project)
        assertV3Project(context, pinnedV3WalletRuntime())
        return context
    }

    _v3ReadRunner(chainId) {
        const official = trustedGasRpcUrl(chainId)
        if (official) return this._readRunner(chainId)
        const env = import.meta.env || {}
        if (Number(chainId) === 31337 && env.VITE_ENVIRONMENT === 'local') {
            const url = String(env.VITE_IDO_V3_LOCAL_RPC || '')
            if (!/^http:\/\/127\.0\.0\.1:[0-9]+\/$/.test(url)) throw v3Error('IDO_V3_ROUTING_UNAVAILABLE')
            return new JsonRpcProvider(url, 31337, { staticNetwork: true })
        }
        throw v3Error('IDO_V3_ROUTING_UNAVAILABLE')
    }

    async readV3State(project) {
        const context = this._v3Context(project)
        return readV3WalletState(context, pinnedV3WalletRuntime(), this._v3ReadRunner(context.chainId))
    }

    // Read-only UI data; never used by the dedicated V3 write preflights.
    readV3Display(project, wallet = null) {
        const context = this._v3Context(project)
        return withDisplayDeadline(() => readV3DisplaySnapshot(context, wallet, pinnedV3WalletRuntime(), this._v3ReadRunner(context.chainId)))
    }

    assertV3StateUnchanged(previous, current, project) {
        const context = this._v3Context(project)
        return assertV3PolicyUnchanged(previous, current, this._v3ReadRunner(context.chainId))
    }

    _v3Confirmations() {
        return pinnedV3WalletRuntime().confirmations
    }

    async reconcileV3Pending(project, wallet, action = null) {
        const context = this._v3Context(project)
        if (!wallet) throw v3Error('IDO_V3_WALLET_CHANGED')
        // Claim entry points reconcile only their original claim. Unrelated
        // receipt availability must not delay a separately simulated claim.
        const relevant = entries => action === 'claim(uint256)'
            ? entries.filter(([identity]) => identity === v3SubmissionIdentity(context, wallet, action)) : entries
        const entries = relevant(listV3Pending(context, wallet))
        if (!entries.length) return true
        const provider = this._v3ReadRunner(context.chainId)
        if (Number((await provider.getNetwork()).chainId) !== context.chainId) throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE')
        const confirmations = this._v3Confirmations()
        for (const [identity, entry] of entries) {
            if (!isValidTransactionHash(entry?.hash)) continue
            try {
                const receipt = await provider.getTransactionReceipt(entry.hash)
                const tx = receipt ? await provider.getTransaction(entry.hash) : null
                const block = receipt ? await provider.getBlock(receipt.blockNumber) : null
                if (receipt && tx && block?.hash === receipt.blockHash &&
                    BigInt(tx.value ?? 0) === 0n &&
                    Number(await provider.getBlockNumber()) - receipt.blockNumber + 1 >= confirmations &&
                    tx.blockHash === receipt.blockHash && [0n, 1n].includes(BigInt(receipt.status)) &&
                    receipt.hash.toLowerCase() === entry.hash.toLowerCase() && tx.hash.toLowerCase() === entry.hash.toLowerCase() &&
                    receipt.to?.toLowerCase() === entry.to && receipt.from?.toLowerCase() === entry.from &&
                    tx.to?.toLowerCase() === entry.to && tx.from?.toLowerCase() === entry.from &&
                    Number(tx.chainId) === context.chainId &&
                    (keccak256(tx.data) === entry.dataHash ||
                        isV3WalletEditedApproval(context, identity.slice(v3SubmissionIdentity(context, wallet, '').length), entry, receipt, tx))) {
                    clearV3Pending(identity)
                }
            } catch { /* RPC unknown leaves the durable local fence intact. */ }
        }
        return relevant(listV3Pending(context, wallet)).length === 0
    }

    v3Pending(project, wallet) {
        return v3PendingSnapshot(this._v3Context(project), wallet)
    }

    async _v3PendingAllowsPurchase(context, from, action) {
        const entries = listV3Pending(context, from)
        if (!entries.length) return true
        // Claim eligibility belongs to the exact contract simulation, not to
        // historical approval/purchase/owner requests. Preserve every record;
        // an unresolved original claim still blocks a second claim attempt.
        if (action === 'claim(uint256)') {
            return !entries.some(([identity]) => identity === v3SubmissionIdentity(context, from, action))
        }
        if (!action.startsWith('purchase')) return false
        // An ERC-20 approval transfers no funds. A canonical mined original
        // approval may unlock purchase while its finality watcher continues.
        // Never clear that record early, and never admit hashless/other actions.
        const rpc = this._v3ReadRunner(context.chainId)
        try {
            if (Number((await rpc.getNetwork()).chainId) !== context.chainId) return false
            for (const [identity, entry] of entries) {
                if (identity !== v3SubmissionIdentity(context, from, 'approvePaymentToken') || !isValidTransactionHash(entry.hash)) return false
                const receipt = await rpc.getTransactionReceipt(entry.hash)
                const tx = receipt ? await rpc.getTransaction(entry.hash) : null
                if (!receipt || !tx || !isSuccessfulReceipt(receipt) ||
                    receipt.hash?.toLowerCase() !== entry.hash.toLowerCase() || tx.hash?.toLowerCase() !== entry.hash.toLowerCase() ||
                    Number(tx.chainId) !== context.chainId || BigInt(tx.value) !== 0n ||
                    tx.to?.toLowerCase() !== context.paymentTokenAddress.toLowerCase() || tx.to?.toLowerCase() !== entry.to ||
                    tx.from?.toLowerCase() !== from.toLowerCase() || tx.from?.toLowerCase() !== entry.from ||
                    receipt.to?.toLowerCase() !== entry.to || receipt.from?.toLowerCase() !== entry.from ||
                    tx.blockHash !== receipt.blockHash || (await rpc.getBlock(receipt.blockNumber))?.hash !== receipt.blockHash ||
                    (keccak256(tx.data) !== entry.dataHash && !isV3WalletEditedApproval(context, 'approvePaymentToken', entry, receipt, tx))) return false
            }
            // Purchase's independent simulation checks actual current allowance;
            // neither a wallet hash nor this read-only observation grants credit.
            return true
        } catch { return false }
    }

    async readV3PurchaseAuthority(project, wallet) {
        const context = this._v3Context(project)
        return v3PurchaseAuthority(context, wallet, this._v3ReadRunner(context.chainId))
    }

    async _v3Signer(project, wallet, verifyBinding = true) {
        const context = this._v3Context(project)
        if (!wallet) throw v3Error('IDO_V3_WALLET_CHANGED')
        // Existing sessions need read-only eth_accounts/eth_chainId, not a new
        // eth_requestAccounts round trip (which may stall WalletConnect).
        if (!(await withDisplayDeadline(() => web3Service.ensureReadOnlySigner(), 8000))) throw v3Error('IDO_V3_WALLET_CHANGED')
        const connectedChainId = Number(web3Service._signerChainId)
        if (connectedChainId !== context.chainId) {
            throw Object.assign(new Error('Wrong network'), { code: WALLET_ERROR_CODES.NETWORK_MISMATCH,
                expectedChainId: context.chainId, connectedChainId })
        }
        const from = await web3Service.signer?.getAddress?.() || web3Service.signer?.address || web3Service._signerAccount
        if (!from || (wallet && from.toLowerCase() !== wallet.toLowerCase())) throw v3Error('IDO_V3_WALLET_CHANGED')
        if (verifyBinding) {
            try { await getVerifiedV3Wallet(from) } catch { throw v3Error('IDO_V3_WALLET_CHANGED') }
        }
        return { context, from }
    }

    async _v3FreshProject(context) {
        let detail
        try { detail = await getProjectDetail(context.id, { fresh: true }) } catch { throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE') }
        try {
            const latest = normalizeProjectContext({ ...detail, id: String(context.id) }, {})
            assertV3Project(latest, pinnedV3WalletRuntime())
            if (latest.contractAddress.toLowerCase() !== context.contractAddress.toLowerCase() ||
                latest.paymentTokenAddress.toLowerCase() !== context.paymentTokenAddress.toLowerCase() ||
                latest.tokenAddress.toLowerCase() !== context.tokenAddress.toLowerCase() ||
                latest.tokenDecimals !== context.tokenDecimals || latest.paymentTokenDecimals !== context.paymentTokenDecimals ||
                latest.onchainLocked !== context.onchainLocked ||
                (context.ownerAddress && (latest.ownerAddress || '').toLowerCase() !== context.ownerAddress.toLowerCase()) ||
                (latest.feeReceiver || '').toLowerCase() !== (context.feeReceiver || '').toLowerCase()) throw Error()
        } catch { throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE') }
        return detail
    }

    async _v3Send(context, signature, args, from, expectedAuthority = null, expectedState = null) {
        const data = new Interface([`function ${signature}`]).encodeFunctionData(signature.slice(0, signature.indexOf('(')), args)
        return this._v3Submit(context, context.contractAddress, data, from, signature, expectedAuthority, expectedState)
    }

    async _v3Submit(context, to, data, from, action, expectedAuthority = null, expectedState = null) {
        if (!['approvePaymentToken', 'approveProjectToken', 'purchase(uint256,uint256)',
            'purchaseWithProof(uint256,uint256,uint64,bytes32[])', 'claim(uint256)', 'refund(uint256)',
            'depositProjectTokens(uint256,uint256)', 'withdrawRaisedFunds(uint256)', 'withdrawProjectTokens(uint256)'].includes(action)) throw v3Error('IDO_V3_ROUTING_UNAVAILABLE')
        if (to.toLowerCase() !== (action === 'approvePaymentToken' ? context.paymentTokenAddress :
            action === 'approveProjectToken' ? context.tokenAddress : context.contractAddress).toLowerCase()) throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE')
        try {
            const signature = action.startsWith('approve') ? 'approve(address,uint256)' : action
            const decoded = new Interface([`function ${signature}`]).decodeFunctionData(signature.slice(0, signature.indexOf('(')), data)
            if (action.startsWith('approve') && (decoded[0].toLowerCase() !== context.contractAddress.toLowerCase() || BigInt(decoded[1]) <= 0n)) throw Error()
            if (!action.startsWith('approve') && BigInt(decoded[0]) !== BigInt(context.id)) throw Error()
        } catch { throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE') }
        if (typeof navigator === 'undefined' || !navigator.locks?.request) throw v3Error('IDO_V3_BROWSER_UNSUPPORTED')
        const identity = v3SubmissionIdentity(context, from, action)
        const lock = v3SubmissionIdentity(context, from, 'wallet')
        return navigator.locks.request(lock, { ifAvailable: true }, async (held) => {
        if (!held) throw v3Error('IDO_V3_SUBMISSION_UNRESOLVED')
        assertNoV3Pending(identity)
        if (!await this._v3PendingAllowsPurchase(context, from, action)) throw v3Error('IDO_V3_SUBMISSION_UNRESOLVED')
        const ownerAction = ['approveProjectToken', 'depositProjectTokens(uint256,uint256)', 'withdrawRaisedFunds(uint256)', 'withdrawProjectTokens(uint256)'].includes(action)
        let gasLimit
        if (ownerAction) {
            // Owner flows have one operation-local baseline and one fresh final
            // snapshot. Never reuse display state or cache authority across sends.
            gasLimit = await this._v3OwnerPreflight(context, to, data, from, action)
        } else {
            gasLimit = await this._v3InvestorPreflight(context, to, data, from)
        }
        // Fail local payload construction before creating a durable send fence.
        v3WalletPayload(to, data, from, gasLimit, context.chainId)
        const requestId = hexlify(randomBytes(16))
        recordV3Pending(identity, '', { ...v3PendingMetadata(to, from, data), requestId })
        let hash
        try { hash = await this._v3WalletSend(to, data, from, gasLimit, context.chainId) }
        catch (error) {
            if (['IDO_V3_WALLET_REJECTED', 'IDO_V3_WALLET_UNSUPPORTED'].includes(error?.code)) releaseV3Unbroadcast(identity, requestId)
            throw error
        }
        // From this point onward only the original hash is observed; failures
        // leave the local fence in place for independent read-only recovery.
        recordV3Pending(identity, hash)
        if (action === 'approvePaymentToken') {
            // A wallet hash acknowledges submission, not execution. Return now;
            // original-hash finality checks continue without delaying the panel.
            const confirmation = this._v3WaitSubmission(context, to, data, from, hash, identity)
            confirmation.catch(() => {}) // Callers may unmount; keep fence on unknown outcomes.
            return { hash, status: 'submitted', confirmation }
        }
        return this._v3WaitSubmission(context, to, data, from, hash, identity)
        })
    }

    async _v3WaitSubmission(context, to, data, from, hash, identity) {
        let receipt
        try { receipt = await this._v3ReadRunner(context.chainId).waitForTransaction(hash, this._v3Confirmations(), 120000) }
        catch { throw v3Error('IDO_V3_SUBMISSION_UNRESOLVED') }
        const transaction = receipt ? await this._v3ReadRunner(context.chainId).getTransaction(hash) : null
        const action = identity.slice(v3SubmissionIdentity(context, from, '').length)
        const exactOrEditedApproval = transaction && (keccak256(transaction.data) === keccak256(data) ||
            isV3WalletEditedApproval(context, action, v3PendingMetadata(to, from, data), receipt, transaction))
        if (!receipt || !transaction || !exactOrEditedApproval || BigInt(transaction.value ?? 0) !== 0n || transaction.hash?.toLowerCase() !== hash.toLowerCase() ||
            transaction.to?.toLowerCase() !== to.toLowerCase() || transaction.from?.toLowerCase() !== from.toLowerCase() ||
            Number((await this._v3ReadRunner(context.chainId).getNetwork()).chainId) !== context.chainId ||
            Number(transaction.chainId) !== context.chainId || transaction.blockHash !== receipt.blockHash ||
            Number(await this._v3ReadRunner(context.chainId).getBlockNumber()) - receipt.blockNumber + 1 < this._v3Confirmations() ||
            receipt.hash?.toLowerCase() !== hash.toLowerCase() ||
            receipt.to?.toLowerCase() !== to.toLowerCase() || receipt.from?.toLowerCase() !== from.toLowerCase() ||
            (await this._v3ReadRunner(context.chainId).getBlock(receipt.blockNumber))?.hash !== receipt.blockHash) {
            throw v3Error('IDO_V3_SUBMISSION_UNRESOLVED')
        }
        if (!isSuccessfulReceipt(receipt)) requireSuccessfulReceipt(receipt, hash)
        clearV3Pending(identity)
        return { hash, status: 'success', blockNumber: receipt.blockNumber }
    }

    async _v3WalletSend(to, data, from, gasLimit, chainId) {
        // Exact standard ERC-20 calldata + explicit project chain. There is no
        // wallet-specific "amount" RPC field; 0 native value must stay zero.
        const expectedChain = Number(chainId ?? web3Service._signerChainId)
        const request = v3WalletPayload(to, data, from, gasLimit, expectedChain)
        const originalSigner = web3Service.signer
        const originalProvider = web3Service._getEIP1193Provider()
        try {
            let hash
            try {
                hash = await web3Service.sendRawTransaction(request.to, request.data, request.from, request.value, request.gas, { chainId: request.chainId })
            } catch (error) {
                const kind = v3WalletErrorKind(error)
                if (kind === 'rejected') throw v3Error('IDO_V3_WALLET_REJECTED')
                if (kind !== 'unsupported') throw v3Error('IDO_V3_SUBMISSION_UNRESOLVED')
                // V2-equivalent fallback only after explicit method rejection.
                // Same session, exact calldata/gas/value; never set a nonce.
                const signer = originalSigner
                if (!signer || !web3Service.provider) throw v3Error('IDO_V3_WALLET_UNSUPPORTED')
                if (signer !== web3Service.signer || originalProvider !== web3Service._getEIP1193Provider() ||
                    Number(web3Service._signerChainId) !== expectedChain ||
                    String(await signer.getAddress()).toLowerCase() !== from.toLowerCase()) throw v3Error('IDO_V3_WALLET_UNSUPPORTED')
                if (originalProvider !== web3Service._getEIP1193Provider() || signer !== web3Service.signer) throw v3Error('IDO_V3_WALLET_UNSUPPORTED')
                try {
                    const tx = await signer.sendTransaction({ to: request.to, data: request.data, value: 0n, gasLimit: BigInt(request.gas), chainId: BigInt(request.chainId) })
                    hash = tx.hash
                } catch (fallbackError) {
                    const fallbackKind = v3WalletErrorKind(fallbackError)
                    if (fallbackKind === 'rejected') throw v3Error('IDO_V3_WALLET_REJECTED')
                    if (fallbackKind === 'unsupported') throw v3Error('IDO_V3_WALLET_UNSUPPORTED')
                    throw v3Error('IDO_V3_SUBMISSION_UNRESOLVED')
                }
            }
            if (!isValidTransactionHash(hash)) throw v3Error('IDO_V3_SUBMISSION_UNRESOLVED')
            return hash
        } catch (error) {
            if (['IDO_V3_WALLET_REJECTED', 'IDO_V3_WALLET_UNSUPPORTED'].includes(error?.code)) throw error
            throw v3Error('IDO_V3_SUBMISSION_UNRESOLVED')
        }
    }

    async _v3InvestorPreflight(context, to, data, from) {
        const gas = await this._v3Simulate(context, to, data, from)
        // Only the live signing account/chain is rechecked after simulation.
        // Caps, balance, lifecycle and proof validity belong to the contract.
        if ((await this._v3Signer(context, from, false)).from.toLowerCase() !== from.toLowerCase()) throw v3Error('IDO_V3_WALLET_CHANGED')
        return gas
    }

    async _v3Simulate(context, to, data, from) {
        let url = trustedGasRpcUrl(context.chainId)
        if (!url && context.chainId === 31337) {
            pinnedV3WalletRuntime()
            url = String(import.meta.env?.VITE_IDO_V3_LOCAL_RPC || '')
        }
        if (!url) throw v3Error('IDO_V3_ROUTING_UNAVAILABLE')
        const transaction = { from, to, data, value: '0x0' }
        try { return await simulateV3Transaction(url, transaction) }
        catch (error) {
            // Read-only availability failover only; never retry a contract
            // revert, malformed estimate, or any wallet-owned send request.
            const secondary = context.chainId === 56 ? 'https://bsc-dataseed1.bnbchain.org' : ''
            if (!error?.rpcUnavailable || !secondary) throw error
            return simulateV3Transaction(secondary, transaction)
        }
    }

    async _v3PurchaseArguments(context, from, amount) {
        // Policy is read once solely to select the ABI and construct calldata.
        // No sale/asset/runtime snapshot is needed; simulation executes rules.
        const policy = await new Contract(context.contractAddress, IDO_V3_ABI, this._v3ReadRunner(context.chainId)).getWhitelistPolicy(BigInt(context.id))
        if (!policy.enabled) return { signature: 'purchase(uint256,uint256)', args: [BigInt(context.id), amount] }
        const state = { enabled: true, version: BigInt(policy.version).toString(), root: String(policy.root) }
        let response
        try { response = await getIdoV3Proof(context.id, from) } catch { throw v3Error('IDO_V3_PROOF_UNAVAILABLE') }
        const proof = assertV3Proof(response, context, state, from)
        return { signature: 'purchaseWithProof(uint256,uint256,uint64,bytes32[])', args: [BigInt(context.id), amount, BigInt(state.version), proof] }
    }

    async _v3OwnerPreflight(context, to, data, from, action) {
        const baseline = await this.readV3State(context)
        await this._v3OwnerSnapshotChecks(context, baseline, from, action, data)
        // Simulation and fresh backend identity are independent reads. Keep the
        // final wallet check after the last chain/asset reads, just before send.
        const [gasLimit] = await Promise.all([
            this._v3Estimate(context, to, data, from),
            this._v3FreshProject(context),
        ])
        const finalState = await this.readV3State(context)
        await assertV3PolicyUnchanged(baseline, finalState, this._v3ReadRunner(context.chainId))
        await this._v3OwnerSnapshotChecks(context, finalState, from, action, data)
        // Balance/allowance calls used finalState.blockNumber; close its reorg
        // window before handing the exact fenced calldata to the wallet.
        await assertV3PolicyUnchanged(finalState, finalState, this._v3ReadRunner(context.chainId))
        const signer = await this._v3Signer(context, from, false)
        if (signer.from.toLowerCase() !== from.toLowerCase()) throw v3Error('IDO_V3_WALLET_CHANGED')
        return gasLimit
    }

    async _v3OwnerSnapshotChecks(context, state, from, action, data) {
        if (action === 'withdrawRaisedFunds(uint256)') {
            v3ProjectActionState(context, state, 'withdrawRaisedFunds', from)
            return
        }
        const withdrawal = action === 'withdrawProjectTokens(uint256)'
        v3ProjectActionState(context, state, withdrawal ? 'withdrawProjectTokens' : 'depositProjectTokens', from)
        if (withdrawal) return
        const approval = action === 'approveProjectToken'
        const method = approval ? 'approve' : 'depositProjectTokens'
        const signature = approval ? 'approve(address,uint256)' : 'depositProjectTokens(uint256,uint256)'
        const decoded = new Interface([`function ${signature}`]).decodeFunctionData(method, data)
        const amount = BigInt(decoded[1])
        const p = state.project
        if (p.tokenPrice <= 0n || p.hardCap <= 0n) throw v3Error('IDO_V3_AMOUNT_INVALID')
        const required = p.hardCap * (10n ** BigInt(context.tokenDecimals)) / p.tokenPrice
        const shortfall = required > p.totalTokensDeposited ? required - p.totalTokensDeposited : 0n
        // Do not send a stale or freely chosen amount if another deposit lands.
        if (amount <= 0n || amount !== shortfall) throw v3Error('IDO_V3_AMOUNT_INVALID')
        const tokenAddress = getAddress(p.projectToken)
        if (tokenAddress !== getAddress(context.tokenAddress)) throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE')
        const token = new Contract(tokenAddress, ERC20_ABI, this._v3ReadRunner(context.chainId))
        const options = { blockTag: state.blockNumber }
        const [balance, allowance] = await Promise.all([
            token.balanceOf(from, options),
            approval ? Promise.resolve(0n) : token.allowance(from, context.contractAddress, options),
        ])
        if (balance < amount || (!approval && allowance < amount)) throw v3Error('IDO_V3_AMOUNT_INVALID')
    }

    async _v3Estimate(context, to, data, from) {
        // M4 never uses the wallet provider or a fallback gas limit as the
        // authority for a V3 simulation. The fixed independent RPC must be
        // available and accept the exact call before any wallet send.
        try {
            const rpc = this._v3ReadRunner(context.chainId)
            if (Number((await rpc.getNetwork()).chainId) !== context.chainId) throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE')
            const transaction = { from, to, data, value: 0n }
            const anchor = await rpc.getBlock('latest')
            if (!anchor?.hash) throw v3Error('IDO_V3_STATE_UNAVAILABLE')
            await rpc.call(transaction)
            const estimate = await rpc.estimateGas(transaction)
            if ((await rpc.getBlock(anchor.number))?.hash !== anchor.hash ||
                Number(await rpc.getBlockNumber()) < anchor.number) throw v3Error('IDO_V3_STATE_UNAVAILABLE')
            const gas = addGasSafetyBuffer(estimate)
            if (gas <= 0n || gas > 1000000n) throw v3Error('IDO_V3_STATE_UNAVAILABLE')
            return gas
        } catch (error) {
            if (String(error?.code || '').startsWith('IDO_V3_')) throw error
            throw v3Error('IDO_V3_STATE_UNAVAILABLE')
        }
    }

    async purchaseV3(project, amount, wallet) {
        const { context, from } = await this._v3Signer(project, wallet, false)
        let amountRaw
        try { amountRaw = parseUnits(String(amount), context.paymentTokenDecimals) } catch { throw v3Error('IDO_V3_AMOUNT_INVALID') }
        if (amountRaw <= 0n) throw v3Error('IDO_V3_AMOUNT_INVALID')
        const { signature, args } = await withDisplayDeadline(() => this._v3PurchaseArguments(context, from, amountRaw), 8000)
        return this._v3Send(context, signature, args, from)
    }

    async approveV3PaymentToken(project, amount, wallet) {
        const { context, from } = await this._v3Signer(project, wallet, false)
        let target
        try { target = parseUnits(String(amount), context.paymentTokenDecimals) } catch { throw v3Error('IDO_V3_AMOUNT_INVALID') }
        if (target <= 0n) throw v3Error('IDO_V3_AMOUNT_INVALID')
        const data = new Interface(['function approve(address,uint256)']).encodeFunctionData('approve', [context.contractAddress, target])
        return this._v3Submit(context, context.paymentTokenAddress, data, from, 'approvePaymentToken')
    }

    async _v3Action(project, action, wallet, signature, args) {
        const ownerAction = ['depositProjectTokens', 'withdrawProjectTokens', 'withdrawRaisedFunds'].includes(action)
        const { context, from } = await this._v3Signer(project, wallet, ownerAction)
        if (!['claim', 'refund', 'depositProjectTokens', 'withdrawProjectTokens', 'withdrawRaisedFunds'].includes(action)) throw v3Error('IDO_V3_ACTION_UNAVAILABLE')
        return this._v3Send(context, signature, args, from)
    }

    claimV3(project, wallet) {
        return this._v3Action(project, 'claim', wallet, 'claim(uint256)', [BigInt(this._v3Context(project).id)])
    }

    refundV3(project, wallet) {
        return this._v3Action(project, 'refund', wallet, 'refund(uint256)', [BigInt(this._v3Context(project).id)])
    }

    withdrawV3ProjectTokens(project, wallet) {
        return this._v3Action(project, 'withdrawProjectTokens', wallet, 'withdrawProjectTokens(uint256)', [BigInt(this._v3Context(project).id)])
    }

    withdrawV3RaisedFunds(project, wallet) {
        return this._v3Action(project, 'withdrawRaisedFunds', wallet, 'withdrawRaisedFunds(uint256)', [BigInt(this._v3Context(project).id)])
    }

    depositV3ProjectTokens(project, amount, wallet) {
        const context = this._v3Context(project)
        let raw
        try { raw = parseUnits(String(amount), context.tokenDecimals) } catch { throw v3Error('IDO_V3_AMOUNT_INVALID') }
        if (raw <= 0n) throw v3Error('IDO_V3_AMOUNT_INVALID')
        return this._v3Action(context, 'depositProjectTokens', wallet, 'depositProjectTokens(uint256,uint256)', [BigInt(context.id), raw])
    }

    async readV3ProjectTokenFunding(project, wallet) {
        const context = this._v3Context(project)
        const state = await this.readV3State(context)
        v3ProjectActionState(context, state, 'depositProjectTokens', wallet)
        // The token identity used for balance/allowance is the value verified from
        // getProject at this exact block, not a stale dashboard/API alias.
        const tokenAddress = getAddress(state.project.projectToken)
        if (tokenAddress !== getAddress(context.tokenAddress)) throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE')
        const token = new Contract(tokenAddress, ERC20_ABI, this._v3ReadRunner(context.chainId))
        const [balance, allowance] = await Promise.all([
            token.balanceOf(wallet, { blockTag: state.blockNumber }),
            token.allowance(wallet, context.contractAddress, { blockTag: state.blockNumber }),
        ])
        return { state, balance, allowance }
    }

    async approveV3ProjectToken(project, amount, wallet) {
        const { context, from } = await this._v3Signer(project, wallet)
        let raw
        try { raw = parseUnits(String(amount), context.tokenDecimals) } catch { throw v3Error('IDO_V3_AMOUNT_INVALID') }
        if (raw <= 0n) throw v3Error('IDO_V3_AMOUNT_INVALID')
        const data = new Interface(['function approve(address,uint256)']).encodeFunctionData('approve', [context.contractAddress, raw])
        return this._v3Submit(context, context.tokenAddress, data, from, 'approveProjectToken')
    }

    _getProjectContract(project, writable = false) {
        const context = this._projectContext(project)
        if (resolveProjectFundingRoute(context).kind === 'ido_v3') {
            // Generic callers may read V3 only after a fixed-block identity
            // check in their own flow; writable V3 actions have dedicated
            // guarded methods and must never inherit this V2-era entry point.
            if (writable) throw v3Error('IDO_V3_ROUTING_UNAVAILABLE')
            const runtime = pinnedV3WalletRuntime()
            assertV3Project(context, runtime)
            const runner = this._v3ReadRunner(context.chainId)
            if (!runner) throw v3Error('IDO_V3_STATE_UNAVAILABLE')
            return new Contract(context.contractAddress, IDO_V3_ABI, runner)
        }
        const runner = writable ? web3Service.signer : this._readRunner(context.chainId)
        if (!runner) throw new Error('Wallet not connected')
        const route = resolveProjectFundingRoute(context)
        const abi = route.kind === 'fair_launch' ? FAIR_LAUNCH_V1_ABI : this._isV2(context) ? JOINCARE_V2_ABI : JOINCARE_ABI
        return new Contract(context.contractAddress, abi, runner)
    }

    _rejectFairLaunchOnLegacyWrite(project) {
        // Inspect only the explicit funding identity. Do not normalize first: legacy
        // normalization may touch trusted runtime/asset fields that generic writes
        // must never inspect for an exact Fair Launch project.
        const route = resolveProjectFundingRoute(project)
        if (route.kind === 'ido_v3' || String(project?.contractVersion ?? project?.contract_version ?? '').trim().toLowerCase() === 'ido_v3_stablecoin') {
            const error = new Error('V3 wallet actions unavailable until governed proof routing is enabled')
            error.code = 'IDO_V3_ROUTING_UNAVAILABLE'
            throw error
        }
        if (route.kind === 'fair_launch') {
            const error = new Error('durable_wallet_submission_unavailable')
            error.code = 'DURABLE_WALLET_SUBMISSION_UNAVAILABLE'
            error.reason = 'durable_wallet_submission_unavailable'
            throw error
        }
    }

    _requireReleaseFairLaunchWalletAuthority(project) {
        const route = resolveProjectFundingRoute(project)
        if (!route.ok || route.kind !== 'fair_launch') throw new Error('Project is not an exact Fair Launch V1 identity')
        const policy = this._fairLaunchSubmissionPolicy(project)
        traceFairLaunchLocalWallet('policy:evaluated', { enabled: !!policy?.enabled, reason: String(policy?.reason || '') })
        if (!policy?.enabled) {
            const error = new Error('durable_wallet_submission_unavailable')
            error.code = 'DURABLE_WALLET_SUBMISSION_UNAVAILABLE'
            error.reason = 'durable_wallet_submission_unavailable'
            throw error
        }
        return policy
    }

    _requireFairLaunchAction(project, action) {
        this._requireReleaseFairLaunchWalletAuthority(project)
        const context = this._projectContext(project)
        const gate = fairLaunchActionAvailable(context, action)
        if (!gate.enabled) {
            const error = new Error(gate.reason === 'durable_wallet_submission_unavailable'
                ? 'durable_wallet_submission_unavailable'
                : `Fair Launch action unavailable: ${gate.reason}`)
            error.code = gate.reason === 'durable_wallet_submission_unavailable'
                ? 'DURABLE_WALLET_SUBMISSION_UNAVAILABLE'
                : 'FAIR_LAUNCH_ACTION_UNAVAILABLE'
            error.reason = gate.reason
            throw error
        }
        return context
    }

    /**
     * 寫入前的統一守門：確保 signer 可用，且**項目綁定的鏈（fail-closed）**與
     * 錢包當前所在鏈一致。錯誤帶 code，讓調用方能區分：
     *  - NETWORK_MISMATCH：錢包在錯誤鏈上（發起交易前直接攔截）
     *  - RPC_ERROR：signer 就緒但鏈上讀取/請求失敗
     *  - SIGNER_UNAVAILABLE：沒有可用 signer
     */
    async _ensureProjectSigner(project) {
        const context = this._projectContext(project)
        traceFairLaunchLocalWallet('signer:begin', { projectId: context.id, expectedChainId: context.chainId })
        console.warn('[approve-flow] ensureSigner...')
        if (!(await web3Service.ensureSigner())) {
            const err = new Error('Wallet signer is unavailable')
            err.code = WALLET_ERROR_CODES.SIGNER_UNAVAILABLE
            throw err
        }
        console.warn('[approve-flow] aligning wallet session for send...')
        await web3Service.alignSessionForSend()
        console.warn('[approve-flow] signer ok, reading network...')
        let connectedChainId
        try {
            const network = await web3Service.provider.getNetwork()
            connectedChainId = Number(network.chainId)
        } catch (rpcErr) {
            const err = new Error(`RPC call failed while reading network: ${rpcErr?.message || rpcErr}`)
            err.code = WALLET_ERROR_CODES.RPC_ERROR
            err.cause = rpcErr
            throw err
        }
        console.warn('[approve-flow] chain check', { connected: connectedChainId, expected: context.chainId })
        traceFairLaunchLocalWallet('signer:chain', { projectId: context.id, expectedChainId: context.chainId, connectedChainId })

        // fail-closed：項目綁定哪條鏈，就只能在該鏈上簽寫入交易
        if (connectedChainId !== context.chainId) {
            const err = new Error(`Wrong network: expected chain ${context.chainId}, connected ${connectedChainId}`)
            err.code = WALLET_ERROR_CODES.NETWORK_MISMATCH
            err.expectedChainId = context.chainId
            err.connectedChainId = connectedChainId
            throw err
        }
        return context
    }

    parsePaymentAmount(project, amount) {
        const context = this._projectContext(project)
        return parseUnits(String(amount), context.paymentTokenDecimals)
    }

    // ============ 鏈上讀取 ============

    /**
     * 查詢 JCC 對 JoinCare 合約的授權額度
     * @param {string} ownerAddress
     * @returns {{ formatted: string, raw: bigint }}
     */
    async getJCCAllowance(ownerAddress) {
        const allowance = await this._getJCC().allowance(ownerAddress, ADDRESSES.JoinCare)
        return { formatted: formatUnits(allowance, 18), raw: allowance }
    }

    /**
     * 查詢用戶在某項目的認購資訊
     * @param {number} projectId
     * @param {string} userAddress
     */
    async getUserPurchase(project, userAddress) {
        const context = this._projectContext(project)
        const route = resolveProjectFundingRoute(context)
        if (route.kind === 'ido_v3') {
            const state = await this.readV3State(context)
            const raw = await state.contract.getUserPurchase(BigInt(context.id), userAddress, { blockTag: state.blockNumber })
            return {
                paymentAmount: formatUnits(raw.paymentAmount, context.paymentTokenDecimals), paymentAmountRaw: raw.paymentAmount,
                totalTokensBought: formatUnits(raw.tokensBought, context.tokenDecimals), totalTokensBoughtRaw: raw.tokensBought,
                claimedTokens: formatUnits(raw.claimedTokens, context.tokenDecimals), claimedTokensRaw: raw.claimedTokens,
                refunded: raw.refunded,
            }
        }
        if (route.kind === 'fair_launch') {
            const contract = this._getProjectContract(context)
            const raw = await contract.getPosition(BigInt(context.id), userAddress)
            let finalAllocation = null
            let claimable = null
            if ([2, 3].includes(Number(context.chainStatus ?? context.chain_status ?? context.statusCode))) {
                [finalAllocation, claimable] = await Promise.all([
                    contract.calculateFinalAllocation(BigInt(context.id), userAddress),
                    contract.calculateClaimable(BigInt(context.id), userAddress),
                ])
            }
            return {
                paymentAmount: formatUnits(raw.contributionAmount, context.paymentTokenDecimals), paymentAmountRaw: raw.contributionAmount,
                totalTokensBought: finalAllocation == null ? null : formatUnits(finalAllocation, context.tokenDecimals), totalTokensBoughtRaw: finalAllocation,
                claimedTokens: formatUnits(raw.claimedTokens, context.tokenDecimals), claimedTokensRaw: raw.claimedTokens,
                claimableTokens: claimable == null ? null : formatUnits(claimable, context.tokenDecimals), claimableTokensRaw: claimable,
                refunded: raw.refunded,
            }
        }
        const raw = await this._getProjectContract(context).getUserPurchase(BigInt(context.id), userAddress)
        if (this._isV2(context) || route.kind === 'ido_v3') {
            return {
                paymentAmount: formatUnits(raw.paymentAmount, context.paymentTokenDecimals),
                paymentAmountRaw: raw.paymentAmount,
                totalTokensBought: formatUnits(raw.tokensBought, context.tokenDecimals),
                totalTokensBoughtRaw: raw.tokensBought,
                claimedTokens: formatUnits(raw.claimedTokens, context.tokenDecimals),
                claimedTokensRaw: raw.claimedTokens,
                refunded: raw.refunded
            }
        }
        return {
            paymentAmount: formatUnits(raw.totalJCCSpent, 18),
            paymentAmountRaw: raw.totalJCCSpent,
            totalJCCSpent: formatUnits(raw.totalJCCSpent, 18),
            totalJCCSpentRaw: raw.totalJCCSpent,
            totalTokensBought: formatUnits(raw.totalTokensBought, 18),
            totalTokensBoughtRaw: raw.totalTokensBought,
            claimedTokens: formatUnits(raw.claimedTokens, 18),
            claimedTokensRaw: raw.claimedTokens,
            refunded: raw.refunded
        }
    }

    /**
     * 計算用戶可領取的代幣數量
     * @param {number} projectId
     * @param {string} userAddress
     * @returns {{ formatted: string, raw: bigint }}
     */
    async calculateClaimable(project, userAddress) {
        const context = this._projectContext(project)
        if (resolveProjectFundingRoute(context).kind === 'ido_v3') {
            const state = await this.readV3State(context)
            const amount = await state.contract.calculateClaimable(BigInt(context.id), userAddress, { blockTag: state.blockNumber })
            return { formatted: formatUnits(amount, context.tokenDecimals), raw: amount }
        }
        const amount = await this._getProjectContract(context).calculateClaimable(BigInt(context.id), userAddress)
        return { formatted: formatUnits(amount, context.tokenDecimals), raw: amount }
    }

    /**
     * 查詢項目鏈上數據（fundsWithdrawn/projectTokensWithdrawn 等）
     * @param {number} projectId
     */
    async getProject(project) {
        const context = this._projectContext(project)
        if (resolveProjectFundingRoute(context).kind === 'ido_v3') {
            const state = await this.readV3State(context)
            const raw = state.project
            return {
                projectIdRaw: raw.projectId, exists: true, projectToken: raw.projectToken, projectOwner: raw.projectOwner,
                maxPerUser: formatUnits(raw.maxPerUser, context.paymentTokenDecimals), maxPerUserRaw: raw.maxPerUser,
                totalRaised: formatUnits(raw.totalRaised, context.paymentTokenDecimals), totalRaisedRaw: raw.totalRaised,
                softCap: formatUnits(raw.softCap, context.paymentTokenDecimals), softCapRaw: raw.softCap,
                hardCap: formatUnits(raw.hardCap, context.paymentTokenDecimals), hardCapRaw: raw.hardCap,
                tokenPrice: formatUnits(raw.tokenPrice, context.paymentTokenDecimals), tokenPriceRaw: raw.tokenPrice,
                totalTokensDeposited: formatUnits(raw.totalTokensDeposited, context.tokenDecimals), totalTokensDepositedRaw: raw.totalTokensDeposited,
                totalTokensSold: formatUnits(raw.totalTokensSold, context.tokenDecimals), totalTokensSoldRaw: raw.totalTokensSold,
                status: state.status,
            }
        }
        const raw = await this._getProjectContract(context).getProject(BigInt(context.id))
        const projectIdRaw = raw.projectId === undefined ? BigInt(context.id) : BigInt(raw.projectId)
        if (resolveProjectFundingRoute(context).kind === 'fair_launch') {
            return {
                projectIdRaw,
                exists: projectIdRaw === BigInt(context.id) && raw.projectToken !== '0x0000000000000000000000000000000000000000' && raw.projectOwner !== '0x0000000000000000000000000000000000000000',
                projectToken: raw.projectToken, paymentToken: raw.paymentToken, projectOwner: raw.projectOwner,
                projectProceedsReceiver: raw.projectProceedsReceiver, platformFeeReceiver: raw.platformFeeReceiver,
                status: Number(raw.status), terminalReason: Number(raw.terminalReason),
                saleTokenAmount: formatUnits(raw.saleTokenAmount, context.tokenDecimals), saleTokenAmountRaw: raw.saleTokenAmount,
                softCap: formatUnits(raw.softCap, context.paymentTokenDecimals), softCapRaw: raw.softCap,
                minContribution: formatUnits(raw.minContribution, context.paymentTokenDecimals), minContributionRaw: raw.minContribution,
                maxContribution: formatUnits(raw.maxContribution, context.paymentTokenDecimals), maxContributionRaw: raw.maxContribution,
                totalRaised: formatUnits(raw.totalRaised, context.paymentTokenDecimals), totalRaisedRaw: raw.totalRaised,
                totalTokensDeposited: formatUnits(raw.totalTokensDeposited, context.tokenDecimals), totalTokensDepositedRaw: raw.totalTokensDeposited,
                totalTokensClaimed: formatUnits(raw.totalTokensClaimed, context.tokenDecimals), totalTokensClaimedRaw: raw.totalTokensClaimed,
                finalTotalRaisedRaw: raw.finalTotalRaised, finalPriceNumeratorRaw: raw.finalPriceNumerator, finalPriceDenominatorRaw: raw.finalPriceDenominator,
                projectProceedsWithdrawn: raw.projectProceedsWithdrawn, platformFeeWithdrawn: raw.platformFeeWithdrawn,
                failedProjectTokensWithdrawn: raw.failedProjectTokensWithdrawn,
            }
        }
        return {
            projectIdRaw,
            exists: !this._isV2(context) || projectIdRaw === BigInt(context.id),
            projectToken: raw.projectToken,
            projectOwner: raw.projectOwner,
            maxPerUser: formatUnits(raw.maxPerUser, context.paymentTokenDecimals),
            maxPerUserRaw: raw.maxPerUser,
            totalRaised: formatUnits(raw.totalRaised, context.paymentTokenDecimals),
            totalRaisedRaw: raw.totalRaised,
            // 項目方 Pending 卡片所需（additive：不改變任何既有返回值）
            softCap: formatUnits(raw.softCap, context.paymentTokenDecimals),
            softCapRaw: raw.softCap,
            hardCap: formatUnits(raw.hardCap, context.paymentTokenDecimals),
            hardCapRaw: raw.hardCap,
            tokenPrice: formatUnits(raw.tokenPrice, context.paymentTokenDecimals),
            tokenPriceRaw: raw.tokenPrice,
            totalTokensDeposited: formatUnits(raw.totalTokensDeposited, context.tokenDecimals),
            totalTokensDepositedRaw: raw.totalTokensDeposited,
            totalTokensSold: formatUnits(raw.totalTokensSold, context.tokenDecimals),
            totalTokensSoldRaw: raw.totalTokensSold,
            status: Number(raw.status),
            fundsWithdrawn: raw.fundsWithdrawn,
            platformFeeWithdrawn: !!raw.platformFeeWithdrawn,
            projectTokensWithdrawn: raw.projectTokensWithdrawn
        }
    }

    /**
     * 查詢任意 ERC20 代幣余額
     */
    async getTokenBalance(tokenAddress, ownerAddress, decimals = 18) {
        const balance = await this._getERC20(tokenAddress).balanceOf(ownerAddress)
        return { formatted: formatUnits(balance, decimals), raw: balance }
    }

    async getTokenBalanceOnChain(tokenAddress, ownerAddress, decimals, chainId) {
        const balance = await new Contract(tokenAddress, ERC20_ABI, this._readRunner(chainId)).balanceOf(ownerAddress)
        return { formatted: formatUnits(balance, decimals), raw: balance }
    }

    /**
     * 查詢任意 ERC20 代幣對 JoinCare 的授權額度
     */
    async getTokenAllowance(tokenAddress, ownerAddress, spender = ADDRESSES.JoinCare, decimals = 18) {
        const allowance = await this._getERC20(tokenAddress).allowance(ownerAddress, spender)
        return { formatted: formatUnits(allowance, decimals), raw: allowance }
    }

    async getTokenAllowanceOnChain(tokenAddress, ownerAddress, spender, decimals, chainId) {
        const allowance = await new Contract(tokenAddress, ERC20_ABI, this._readRunner(chainId)).allowance(ownerAddress, spender)
        return { formatted: formatUnits(allowance, decimals), raw: allowance }
    }

    async getPaymentTokenBalance(project, ownerAddress) {
        const context = this._projectContext(project)
        if (resolveProjectFundingRoute(context).kind === 'ido_v3') {
            const state = await this.readV3State(context)
            const tokenAddress = getAddress(state.project.paymentToken)
            if (tokenAddress !== getAddress(context.paymentTokenAddress)) throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE')
            const raw = await new Contract(tokenAddress, ERC20_ABI, this._v3ReadRunner(context.chainId)).balanceOf(ownerAddress, { blockTag: state.blockNumber })
            return { raw, formatted: formatUnits(raw, context.paymentTokenDecimals) }
        }
        return this.getTokenBalanceOnChain(
            context.paymentTokenAddress,
            ownerAddress,
            context.paymentTokenDecimals,
            context.chainId,
        )
    }

    async getPaymentTokenAllowance(project, ownerAddress) {
        const context = this._projectContext(project)
        if (resolveProjectFundingRoute(context).kind === 'ido_v3') {
            const state = await this.readV3State(context)
            const tokenAddress = getAddress(state.project.paymentToken)
            if (tokenAddress !== getAddress(context.paymentTokenAddress)) throw v3Error('IDO_V3_IDENTITY_UNAVAILABLE')
            const raw = await new Contract(tokenAddress, ERC20_ABI, this._v3ReadRunner(context.chainId)).allowance(ownerAddress, context.contractAddress, { blockTag: state.blockNumber })
            return { raw, formatted: formatUnits(raw, context.paymentTokenDecimals) }
        }
        return this.getTokenAllowanceOnChain(
            context.paymentTokenAddress,
            ownerAddress,
            context.contractAddress,
            context.paymentTokenDecimals,
            context.chainId,
        )
    }

    // ============ Fair Launch V1 exact wallet flows ============

    approveFairLaunchPaymentToken(project, targetRaw) {
        traceFairLaunchLocalWallet('approval:entry')
        try {
            this._requireReleaseFairLaunchWalletAuthority(project)
            return this._approveExact(project, null, targetRaw, 'contribute')
        } catch (error) {
            traceFairLaunchLocalWallet('approval:entry-error', {
                code: String(error?.code || ''),
                reason: String(error?.reason || ''),
                message: String(error?.message || ''),
            })
            throw error
        }
    }

    hasFairLaunchUnresolved(project, action, signature, args, expectedState = null) {
        try {
            const context = this._projectContext(project)
            const from = web3Service.signer?.address || web3Service._signerAccount
            if (!from) return false
            const iface = new Interface([`function ${signature}`])
            const methodName = signature.slice(0, signature.indexOf('('))
            const data = iface.encodeFunctionData(methodName, args)
            return !!findFairLaunchUnresolved({ wallet: from, chainId: context.chainId, target: context.contractAddress, projectId: context.id, action, calldata: data, stateBinding: expectedState })
        } catch { return false }
    }

    contributeFairLaunch(project, paymentAmountRaw) {
        this._requireReleaseFairLaunchWalletAuthority(project)
        const context = this._projectContext(project)
        return this._sendFairLaunchEncoded(context, 'contribute', 'contribute(uint256,uint256)', [BigInt(context.id), BigInt(String(paymentAmountRaw))], 350000n)
    }

    claimFairLaunch(project) {
        this._requireReleaseFairLaunchWalletAuthority(project)
        const context = this._projectContext(project)
        return this._sendFairLaunchEncoded(context, 'claim', 'claim(uint256)', [BigInt(context.id)], 350000n)
    }

    claimFairLaunchFor(project, user) {
        this._requireReleaseFairLaunchWalletAuthority(project)
        const context = this._projectContext(project)
        return this._sendFairLaunchEncoded(context, 'claim', 'claimFor(uint256,address)', [BigInt(context.id), user], 350000n)
    }

    refundFairLaunch(project) {
        this._requireReleaseFairLaunchWalletAuthority(project)
        const context = this._projectContext(project)
        return this._sendFairLaunchEncoded(context, 'refund', 'refund(uint256)', [BigInt(context.id)], 300000n)
    }

    refundFairLaunchFor(project, user) {
        this._requireReleaseFairLaunchWalletAuthority(project)
        const context = this._projectContext(project)
        return this._sendFairLaunchEncoded(context, 'refund', 'refundFor(uint256,address)', [BigInt(context.id), user], 300000n)
    }

    async approveFairLaunchProjectToken(project) {
        this._requireReleaseFairLaunchWalletAuthority(project)
        const context = this._projectContext(project)
        const chainProject = await this._readFairLaunchDepositState(context)
        const remainingRaw = calculateFairLaunchRemainingRaw(chainProject.saleTokenAmount, chainProject.totalTokensDeposited)
        const result = await this._approveExact(context, context.tokenAddress, remainingRaw, 'depositProjectTokens', {
            expectedSaleTokenAmountRaw: chainProject.saleTokenAmount.toString(),
            expectedDepositedRaw: chainProject.totalTokensDeposited.toString(),
        })
        return { ...result, remainingRaw }
    }

    async depositFairLaunchProjectTokens(project) {
        this._requireReleaseFairLaunchWalletAuthority(project)
        const context = this._projectContext(project)
        const chainProject = await this._readFairLaunchDepositState(context)
        const remainingRaw = calculateFairLaunchRemainingRaw(chainProject.saleTokenAmount, chainProject.totalTokensDeposited)
        return this._sendFairLaunchEncoded(context, 'depositProjectTokens', 'depositProjectTokens(uint256,uint256)', [BigInt(context.id), BigInt(remainingRaw)], 350000n, {
            expectedSaleTokenAmountRaw: chainProject.saleTokenAmount.toString(),
            expectedDepositedRaw: chainProject.totalTokensDeposited.toString(),
        })
    }

    async _readFairLaunchDepositState(project) {
        const declared = this._requireFairLaunchAction(project, 'depositProjectTokens')
        const context = await this._ensureProjectSigner(declared)
        await this._verifyFairLaunchLocalDeployment(context)
        return this._preflightFairLaunch(context, 'depositProjectTokens')
    }

    withdrawFairLaunchProjectProceeds(project) {
        this._requireReleaseFairLaunchWalletAuthority(project)
        const context = this._projectContext(project)
        return this._sendFairLaunchEncoded(context, 'withdrawProjectProceeds', 'withdrawProjectProceeds(uint256)', [BigInt(context.id)], 300000n)
    }

    withdrawFairLaunchPlatformFee(project) {
        this._requireReleaseFairLaunchWalletAuthority(project)
        const context = this._projectContext(project)
        return this._sendFairLaunchEncoded(context, 'withdrawPlatformFee', 'withdrawPlatformFee(uint256)', [BigInt(context.id)], 300000n)
    }

    withdrawFairLaunchFailedProjectTokens(project) {
        this._requireReleaseFairLaunchWalletAuthority(project)
        const context = this._projectContext(project)
        return this._sendFairLaunchEncoded(context, 'withdrawFailedProjectTokens', 'withdrawFailedProjectTokens(uint256)', [BigInt(context.id)], 300000n)
    }

    // ============ C端寫入操作 ============

    /**
     * 授權 JCC 代幣給 JoinCare 合約
     * @param {string} amount - 可讀金額 或 'max'
     */
    async approveJCC(amount = 'max') {
        if (!(await web3Service.ensureSigner())) {
            const error = new Error('Wallet signer is unavailable')
            error.code = WALLET_ERROR_CODES.SIGNER_UNAVAILABLE
            throw error
        }
        const approveAmount = amount === 'max' ? MAX_UINT256 : parseUnits(amount, 18)
        const data = new Interface(['function approve(address,uint256)']).encodeFunctionData('approve', [ADDRESSES.JoinCare, approveAmount])
        const from = web3Service.signer?.address || web3Service._signerAccount
        if (!from) throw new Error('No signer address for JCC approval')
        const gasLimit = await this._estimateGasLimit(ADDRESSES.JCC, data, from, 120000n, { chainId: ACTIVE_CHAIN.chainId })
        const txHash = await this._dualChannelSend(ADDRESSES.JCC, data, from, gasLimit)
        const receipt = await this._waitForReceipt(txHash, { chainId: ACTIVE_CHAIN.chainId })
        requireSuccessfulReceipt(receipt, txHash)
        return { hash: txHash, status: 'success' }
    }

    /**
     * 認購項目代幣
     * @param {number} projectId
     * @param {string} jccAmount - JCC 數量（可讀）
     */
    async approvePaymentToken(project, amount = 'max') {
        this._rejectFairLaunchOnLegacyWrite(project)
        const context = await this._ensureProjectSigner(project)
        const approveAmount = amount === 'max' ? MAX_UINT256 : parseUnits(String(amount), context.paymentTokenDecimals)
        console.warn('[approve-flow] sending approve tx...', { token: context.paymentTokenAddress, spender: context.contractAddress })
        const erc20 = new Interface(['function approve(address,uint256)'])
        const data = erc20.encodeFunctionData('approve', [context.contractAddress, approveAmount])
        const from = web3Service.signer?.address || web3Service._signerAccount
        if (!from) throw new Error('No signer address for approve')
        // 双通道：先裸 eth_sendTransaction；仅明确“不支持该方法”时才回退 signer。
        // Once an EIP-1193 wallet owns the confirmation prompt there is no safe
        // client-side timeout: a person may approve after that timer fires, and
        // the request can still broadcast. Wait for the wallet's explicit hash,
        // rejection or RPC error. Invalid responses and ordinary RPC failures
        // remain uncertain and must never trigger an automatic resend.
        let txHash
        try {
          txHash = await web3Service.sendRawTransaction(
            context.paymentTokenAddress,
            data,
            from,
            '0x0',
            '0x' + (120000).toString(16),
          )
        } catch (rawErr) {
          if (!canSafelyFallbackRawSend(rawErr)) throw rawErr
          console.warn('[approve-flow] raw send unsupported, falling back to ethers signer:', rawErr?.message || rawErr)
          const tx = await web3Service.signer.sendTransaction({
            to: context.paymentTokenAddress,
            data,
            gasLimit: 120000n,
          })
          txHash = tx.hash
        }
        requireValidTransactionHash(txHash)
        console.warn('[approve-flow] send accepted', { hash: txHash })
        // 轮询回执（不再用 tx.wait，避免其内部等待也挂起）
        const receipt = await this._waitForReceipt(txHash, { chainId: context.chainId })
        console.warn('[approve-flow] tx done', { hash: txHash, status: receipt?.status })
        requireSuccessfulReceipt(receipt, txHash)
        return { hash: txHash, status: 'success' }
    }

    async _sendFairLaunchEncoded(project, action, signature, args, fallbackGasLimit, expectedState = null) {
        // _requireFairLaunchAction consumes only the Backend-authoritative action
        // descriptor. Until the durable wallet-submission service is admitted,
        // Backend returns durable_wallet_submission_unavailable and no signer is entered.
        const declared = this._requireFairLaunchAction(project, action)
        const context = await this._ensureProjectSigner(declared)
        await this._verifyFairLaunchLocalDeployment(context)
        const contract = await this._preflightFairLaunch(context, action, expectedState)
        const iface = new Interface([`function ${signature}`])
        const methodName = signature.slice(0, signature.indexOf('('))
        const data = iface.encodeFunctionData(methodName, args)
        const from = web3Service.signer?.address || web3Service._signerAccount
        if (!from) throw new Error('No signer address for Fair Launch transaction')
        const fence = { wallet: from, chainId: context.chainId, target: context.contractAddress, projectId: context.id, action, calldata: data, stateBinding: expectedState }
        assertFairLaunchNotUnresolved(fence)
        try {
            const gasLimit = await this._estimateFairLaunchGasLimit(
                context.contractAddress,
                data,
                from,
                fallbackGasLimit,
                { chainId: context.chainId },
            )
            const txHash = await this._dualChannelSend(context.contractAddress, data, from, gasLimit)
            try {
                const receipt = await this._waitForReceipt(txHash, { chainId: context.chainId })
                requireSuccessfulReceipt(receipt, txHash)
				await applyFairLaunchLocalTransaction(context.id, txHash)
                // A wallet receipt is not canonical CE-V2/application/reconciliation
                // evidence and therefore never clears a durable/unresolved fence.
                return { hash: txHash, status: 'success', receipt, contract }
            } catch (error) {
                if (error?.code === WALLET_ERROR_CODES.TRANSACTION_RECEIPT_TIMEOUT) recordFairLaunchUnresolved(fence, txHash)
                throw error
            }
        } catch (error) {
            if (error?.code === WALLET_ERROR_CODES.TRANSACTION_BROADCAST_UNCERTAIN) recordFairLaunchUnresolved(fence, error?.transactionHash || '')
            throw error
        }
    }

    async _verifyFairLaunchLocalDeployment(context) {
        traceFairLaunchLocalWallet('deployment:policy')
        const policy = this._requireReleaseFairLaunchWalletAuthority(context)
        // The wallet provider is authoritative for the selected account, live
        // chainId and transaction signatures. Deployment identity is a separate
        // read-only check against the exact canonical RPC already cross-bound by
        // the signed local manifest and submission policy. Do not depend on
        // ordinary platform-settings bootstrap or its optional global RPC hint:
        // local wallet connection is intentionally allowed to survive a failed
        // JoinCare token login, while Fair Launch writes must remain available
        // only through this run-bound authority.
        const canonicalProvider = new JsonRpcProvider(policy.rpcUrl)
        const verified = await verifyFairLaunchLocalDeployment(
            canonicalProvider,
            policy,
            context,
            policy.rpcUrl,
        )
        traceFairLaunchLocalWallet('deployment:verified', { verified })
        if (!verified) {
            const error = new Error('durable_wallet_submission_unavailable')
            error.code = 'DURABLE_WALLET_SUBMISSION_UNAVAILABLE'
            error.reason = 'durable_wallet_submission_unavailable'
            throw error
        }
    }

    async _preflightFairLaunch(context, action, expectedState = null) {
        const contract = this._getProjectContract(context)
        const chainProject = await contract.getProject(BigInt(context.id))
        const runtimeMatrix = {
            contribute: ['project', 'payment'],
            depositProjectTokens: ['project'],
            claim: ['project'],
            withdrawFailedProjectTokens: ['project'],
            withdrawProjectProceeds: ['payment'],
            withdrawPlatformFee: ['payment'],
            refund: [],
        }
        const requiredRuntimes = runtimeMatrix[action]
        if (!requiredRuntimes) throw new Error(`Fair Launch runtime matrix missing for ${action}`)
        const [projectTokenMatches, paymentTokenMatches] = await Promise.all([
            requiredRuntimes.includes('project') ? contract.projectTokenRuntimeMatches(BigInt(context.id)) : true,
            requiredRuntimes.includes('payment') ? contract.paymentTokenRuntimeMatches(BigInt(context.id)) : true,
        ])
        const same = (a, b) => String(a || '').toLowerCase() === String(b || '').toLowerCase()
        if (BigInt(chainProject.projectId) !== BigInt(context.id)
            || !same(chainProject.projectToken, context.tokenAddress)
            || !same(chainProject.paymentToken, context.paymentTokenAddress)
            || !projectTokenMatches || !paymentTokenMatches) {
            const error = new Error('Fair Launch trusted identity/runtime preflight failed')
            error.code = 'FAIR_LAUNCH_PREFLIGHT_FAILED'
            throw error
        }
        if (expectedState && (
            String(chainProject.saleTokenAmount) !== String(expectedState.expectedSaleTokenAmountRaw)
            || String(chainProject.totalTokensDeposited) !== String(expectedState.expectedDepositedRaw)
        )) {
            const error = new Error('Fair Launch deposit state changed; refresh before signing')
            error.code = 'FAIR_LAUNCH_DEPOSIT_STATE_CHANGED'
            throw error
        }
        const status = Number(chainProject.status)
        const signerAddress = getAddress(web3Service.signer?.address || web3Service._signerAccount)
        if (['depositProjectTokens', 'withdrawFailedProjectTokens'].includes(action)
            && signerAddress !== getAddress(chainProject.projectOwner)) {
            const error = new Error(`${action} requires the projectOwner wallet`)
            error.code = 'FAIR_LAUNCH_ROLE_BLOCKED'
            throw error
        }
        if (action === 'withdrawProjectProceeds' && signerAddress !== getAddress(chainProject.projectProceedsReceiver)) {
            const error = new Error('withdrawProjectProceeds requires the fixed project proceeds receiver Safe')
            error.code = 'FAIR_LAUNCH_ROLE_BLOCKED'
            throw error
        }
        if (action === 'withdrawPlatformFee' && signerAddress !== getAddress(chainProject.platformFeeReceiver)) {
            const error = new Error('withdrawPlatformFee requires the fixed platform fee receiver wallet')
            error.code = 'FAIR_LAUNCH_ROLE_BLOCKED'
            throw error
        }
        const permitted = {
            contribute: status === 1,
            claim: status === 3,
            refund: status === 4 || status === 5,
            depositProjectTokens: status === 0,
            withdrawProjectProceeds: status === 3,
            withdrawPlatformFee: status === 3,
            withdrawFailedProjectTokens: status === 4 || status === 5,
        }[action]
        if (!permitted) {
            const error = new Error(`Fair Launch chain status ${status} does not permit ${action}`)
            error.code = 'FAIR_LAUNCH_CHAIN_STATUS_BLOCKED'
            throw error
        }
        return chainProject
    }

    async _estimateFairLaunchGasLimit(to, data, from, fallbackGasLimit, options = {}) {
        const timeoutMs = typeof options === 'number' ? options : options.timeoutMs ?? 12000
        const expectedChainId = typeof options === 'object' ? Number(options.chainId) : NaN
        const trustedRpcTimeoutMs = typeof options === 'object' ? options.trustedRpcTimeoutMs ?? 8000 : 8000
        const fetcher = typeof options === 'object' ? options.fetcher ?? fetch : fetch
        const provider = web3Service.provider
        const transaction = { to, data, from, value: 0n }
        let trustedFailure = null
        if (trustedGasRpcUrl(expectedChainId)) {
            try {
                return await estimateGasViaTrustedRpc(transaction, expectedChainId, trustedRpcTimeoutMs, fetcher)
            } catch (trustedError) {
                if (!canSafelyFallbackTrustedGasRpc(trustedError)) throw trustedError
                trustedFailure = trustedError
                console.warn('[tx-flow] trusted RPC gas simulation unavailable; falling back to wallet estimate', {
                    trustedOutcome: trustedError.code,
                })
            }
        }
        if (!provider?.estimateGas) throw trustedFailure || new Error('Fair Launch requires eth_estimateGas support')
        let timer
        try {
            return addGasSafetyBuffer(await Promise.race([
                provider.estimateGas(transaction),
                new Promise((_, reject) => { timer = setTimeout(() => reject(gasEstimateTimeoutError()), timeoutMs) }),
            ]))
        } catch (error) {
            const walletTimedOut = error?.code === 'GAS_ESTIMATE_TIMEOUT'
            const walletUnsupported = canSafelyFallbackGasEstimate(error)
            if (!walletTimedOut && !walletUnsupported) throw error
            if (trustedFailure) throw error
            if (!walletUnsupported) throw error
            if (typeof provider.call !== 'function') throw error
            await provider.call(transaction, 'latest')
            const cap = BigInt(fallbackGasLimit)
            if (cap <= 0n || cap > 1000000n) throw new Error('Fair Launch fallback gas cap is invalid')
            return cap
        } finally {
            clearTimeout(timer)
        }
    }

    async _approveExact(project, tokenAddress, targetRaw, action, expectedState = null) {
        // The static release guard is deliberately repeated here so direct helper
        // callers and both zero-first approval generations remain fail closed.
        traceFairLaunchLocalWallet('approval:begin', { action })
        try {
            this._requireReleaseFairLaunchWalletAuthority(project)
            traceFairLaunchLocalWallet('approval:policy')
            const declared = this._requireFairLaunchAction(project, action)
            traceFairLaunchLocalWallet('approval:action')
            const context = await this._ensureProjectSigner(declared)
            tokenAddress ||= action === 'contribute' ? context.paymentTokenAddress : context.tokenAddress
            await this._verifyFairLaunchLocalDeployment(context)
            traceFairLaunchLocalWallet('approval:preflight')
            await this._preflightFairLaunch(context, action, expectedState)
            const target = BigInt(String(targetRaw))
            if (target < 0n) throw new Error('Exact allowance target must be unsigned')
            const owner = web3Service.signer?.address || web3Service._signerAccount
            if (!owner) throw new Error('No signer address for approval')
            const token = this._getERC20(tokenAddress)
            const current = BigInt(await token.allowance(owner, context.contractAddress))
            traceFairLaunchLocalWallet('approval:allowance', { currentRaw: current.toString(), targetRaw: target.toString() })
            const receipts = []
            const approve = async (approvalAmount) => {
                const iface = new Interface(['function approve(address,uint256)'])
                const data = iface.encodeFunctionData('approve', [context.contractAddress, approvalAmount])
                traceFairLaunchLocalWallet('approval:estimate', { amountRaw: approvalAmount.toString() })
                const gas = await this._estimateFairLaunchGasLimit(
                    tokenAddress,
                    data,
                    owner,
                    120000n,
                    { chainId: context.chainId },
                )
                traceFairLaunchLocalWallet('approval:send', { amountRaw: approvalAmount.toString() })
                const hash = await this._dualChannelSend(tokenAddress, data, owner, gas)
                traceFairLaunchLocalWallet('approval:receipt', { transactionHash: hash })
                const receipt = await this._waitForReceipt(hash, { chainId: context.chainId })
                requireSuccessfulReceipt(receipt, hash)
                receipts.push({ hash, amountRaw: approvalAmount.toString() })
            }
            if (current !== 0n && target !== 0n && current !== target) await approve(0n)
            if (current !== target) await approve(target)
            traceFairLaunchLocalWallet('approval:complete')
            return { status: 'success', targetRaw: target.toString(), transactions: receipts }
        } catch (error) {
            traceFairLaunchLocalWallet('approval:error', {
                code: String(error?.code || ''),
                reason: String(error?.reason || ''),
                message: String(error?.message || ''),
            })
            throw error
        }
    }

    async _waitForReceipt(txHash, options = {}) {
        requireValidTransactionHash(txHash)
        const timeoutMs = typeof options === 'number' ? options : options.timeoutMs ?? 120000
        const queryTimeoutMs = typeof options === 'object' ? options.queryTimeoutMs ?? 5000 : 5000
        const pollIntervalMs = typeof options === 'object' ? options.pollIntervalMs ?? 2000 : 2000
        const expectedChainId = typeof options === 'object' ? Number(options.chainId) : Number(ACTIVE_CHAIN.chainId)
        const fetcher = typeof options === 'object' ? options.fetcher ?? fetch : fetch
        const trustedRpcUrl = trustedGasRpcUrl(expectedChainId)
        const runtimeRpcUrl = (typeof window !== 'undefined' && window.__JC_ACTIVE_RPC_URL) || ''
        const rpcUrls = [...new Set([trustedRpcUrl, runtimeRpcUrl].filter(Boolean))]

        const queryRpc = async (rpcUrl) => {
          const controller = typeof AbortController === 'function' ? new AbortController() : null
          let timer
          const deadline = new Promise((resolve) => {
            timer = setTimeout(() => {
              controller?.abort()
              resolve(null)
            }, queryTimeoutMs)
          })
          const operation = (async () => {
            const res = await fetcher(rpcUrl, {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ jsonrpc: '2.0', id: Date.now(), method: 'eth_getTransactionReceipt', params: [txHash] }),
              signal: controller?.signal,
            })
            if (!res?.ok) return null
            const payload = await res.json()
            return payload?.result || null
          })()
          try {
            return await Promise.race([operation, deadline])
          } catch {
            return null
          } finally {
            clearTimeout(timer)
            controller?.abort()
          }
        }

        const queryWallet = async () => {
          if (!web3Service.provider?.getTransactionReceipt) return null
          let timer
          try {
            return await Promise.race([
              web3Service.provider.getTransactionReceipt(txHash),
              new Promise((resolve) => { timer = setTimeout(() => resolve(null), queryTimeoutMs) }),
            ])
          } catch {
            return null
          } finally {
            clearTimeout(timer)
          }
        }

        // Receipt reads are strictly ordered: fixed trusted chain RPC, optional
        // Backend runtime RPC, then the wallet provider. A successful earlier
        // path prevents later requests. These are read-only queries and can
        // never resubmit or otherwise authorize another transaction.
        const started = Date.now()
        while (Date.now() - started < timeoutMs) {
          for (const rpcUrl of rpcUrls) {
            const receipt = await queryRpc(rpcUrl)
            if (receipt) return receipt
          }
          const walletReceipt = await queryWallet()
          if (walletReceipt) return walletReceipt
          await new Promise((r) => setTimeout(r, pollIntervalMs))
        }
        const error = new Error('Transaction receipt timeout: ' + txHash)
        error.code = WALLET_ERROR_CODES.TRANSACTION_RECEIPT_TIMEOUT
        error.transactionHash = txHash
        throw error
    }

    async purchase(project, paymentAmount) {
        let phase = 'preflight'
        try {
            this._rejectFairLaunchOnLegacyWrite(project)
            const context = await this._ensureProjectSigner(project)
            const amountRaw = parseUnits(String(paymentAmount), context.paymentTokenDecimals)
            console.warn('[tx-flow] sending purchase tx...', { projectId: context.id, amount: paymentAmount })
            const ido = new Interface(['function purchase(uint256,uint256)'])
            const data = ido.encodeFunctionData('purchase', [BigInt(context.id), amountRaw])
            const from = web3Service.signer?.address || web3Service._signerAccount
            if (!from) throw new Error('No signer address for purchase')
            phase = 'estimate'
            let gasLimit
            try {
                gasLimit = await this._estimateGasLimit(context.contractAddress, data, from, 300000n, { chainId: context.chainId })
            } catch (estimateError) {
                if (estimateError && typeof estimateError === 'object') {
                    // Snapshot only scalar identity at the failure site. No RPC calls,
                    // calldata, signed payloads or raw provider objects are retained.
                    estimateError.purchaseEstimateContext = {
                        from,
                        chainId: context.chainId,
                        projectId: String(context.id),
                        amount: String(paymentAmount),
                    }
                    // Read-only diagnostic uses the official RPC first. Only if
                    // that bounded observer is unavailable may it consult the
                    // wallet provider; no diagnostic result can authorize sending.
                    try {
                        const transaction = { to: context.contractAddress, data, from, value: 0n }
                        estimateError.purchaseRpcPathComparison = await comparePurchaseRpcPaths(
                            web3Service.provider,
                            transaction,
                            context.chainId,
                        )
                    } catch { /* Diagnostic must never replace the original estimate failure. */ }
                }
                throw estimateError
            }
            phase = 'send'
            const txHash = await this._dualChannelSend(context.contractAddress, data, from, gasLimit)
            const receipt = await this._waitForReceipt(txHash, { chainId: context.chainId })
            console.warn('[tx-flow] purchase done', { hash: txHash, status: receipt?.status })
            requireSuccessfulReceipt(receipt, txHash)
            return { hash: txHash, status: 'success' }
        } catch (error) {
            if (error && typeof error === 'object') error.purchasePhase = phase
            throw error
        }
    }

    /**
     * Estimate against the live wallet session, then add a safety buffer.
     * Business reverts are propagated so an invalid transaction is never sent.
     */
    async _estimateGasLimit(to, data, from, fallbackGasLimit, options = {}) {
        const timeoutMs = typeof options === 'number' ? options : options.timeoutMs ?? 12000
        const expectedChainId = typeof options === 'object' ? Number(options.chainId) : NaN
        const trustedRpcTimeoutMs = typeof options === 'object' ? options.trustedRpcTimeoutMs ?? 8000 : 8000
        const fetcher = typeof options === 'object' ? options.fetcher ?? fetch : fetch
        const fallback = BigInt(fallbackGasLimit)
        const provider = web3Service.provider
        const transaction = { to, data, from, value: 0n }
        let trustedFailure = null
        if (trustedGasRpcUrl(expectedChainId)) {
            try {
                const gasLimit = await estimateGasViaTrustedRpc(transaction, expectedChainId, trustedRpcTimeoutMs, fetcher)
                console.warn('[tx-flow] trusted read-only RPC supplied gas', { gasLimit: gasLimit.toString() })
                return gasLimit
            } catch (trustedError) {
                if (!canSafelyFallbackTrustedGasRpc(trustedError)) throw trustedError
                trustedFailure = trustedError
                console.warn('[tx-flow] trusted RPC gas simulation unavailable; falling back to wallet estimate', {
                    trustedOutcome: trustedError.code,
                })
            }
        }
        if (!provider || typeof provider.estimateGas !== 'function') {
            if (trustedFailure) throw trustedFailure
            console.warn('[tx-flow] provider cannot estimate gas; using conservative fallback', { fallback: fallback.toString() })
            return fallback
        }

        let timer
        try {
            const estimated = await Promise.race([
                provider.estimateGas(transaction),
                new Promise((_, reject) => { timer = setTimeout(() => reject(gasEstimateTimeoutError()), timeoutMs) }),
            ])
            const gasLimit = addGasSafetyBuffer(estimated)
            console.warn('[tx-flow] gas estimated', {
                estimated: BigInt(estimated).toString(),
                gasLimit: gasLimit.toString(),
            })
            return gasLimit
        } catch (estimateError) {
            const walletTimedOut = estimateError?.code === 'GAS_ESTIMATE_TIMEOUT'
            if (!walletTimedOut && !canSafelyFallbackGasEstimate(estimateError)) throw estimateError
            if (trustedFailure) throw estimateError
            if (walletTimedOut) throw estimateError
            console.warn('[tx-flow] gas estimation unsupported; using conservative local fallback', { fallback: fallback.toString() })
            return fallback
        } finally {
            clearTimeout(timer)
        }
    }

    /** 双通道发送：裸请求优先；仅明确不支持 eth_sendTransaction 时回退 signer。 */
    async _dualChannelSend(to, data, from, gasLimit) {
        let txHash
        try {
            // Do not time out a wallet-owned confirmation prompt. The provider
            // may still broadcast after an arbitrary human delay, so racing it
            // with a timer would create a false unresolved result and tempt a
            // duplicate submission. Receipt polling remains separately bounded.
            txHash = await web3Service.sendRawTransaction(
                to,
                data,
                from,
                '0x0',
                '0x' + gasLimit.toString(16),
            )
        } catch (rawErr) {
            if (!canSafelyFallbackRawSend(rawErr)) throw rawErr
            console.warn('[tx-flow] raw send unsupported, falling back to ethers signer:', rawErr?.message || rawErr)
            const provider = web3Service.provider
            if (!provider || !web3Service.signer) throw rawErr
            // Let the wallet/signer choose the pending nonce. Supplying a manually read
            // latest nonce here could replace an unrelated pending user transaction.
            const tx = await web3Service.signer.sendTransaction({ to, data, gasLimit })
            txHash = tx.hash
        }
        requireValidTransactionHash(txHash)
        console.warn('[tx-flow] send accepted', { hash: txHash })
        return txHash
    }

    /**
     * 領取代幣
     * @param {number} projectId
     */
    async claim(project) {
        this._rejectFairLaunchOnLegacyWrite(project)
        const context = await this._ensureProjectSigner(project)
        console.warn('[tx-flow] sending claim tx...', { projectId: context.id })
        const ido = new Interface(['function claim(uint256)'])
        const data = ido.encodeFunctionData('claim', [BigInt(context.id)])
        const from = web3Service.signer?.address || web3Service._signerAccount
        if (!from) throw new Error('No signer address for claim')
        const gasLimit = await this._estimateGasLimit(context.contractAddress, data, from, 300000n, { chainId: context.chainId })
        const txHash = await this._dualChannelSend(context.contractAddress, data, from, gasLimit)
        const receipt = await this._waitForReceipt(txHash, { chainId: context.chainId })
        console.warn('[tx-flow] claim done', { hash: txHash, status: receipt?.status })
        requireSuccessfulReceipt(receipt, txHash)
        return { hash: txHash, status: 'success' }
    }

    /**
     * 申請退款
     * @param {number} projectId
     */
    async refund(project) {
        this._rejectFairLaunchOnLegacyWrite(project)
        const context = await this._ensureProjectSigner(project)
        console.warn('[tx-flow] sending refund tx...', { projectId: context.id })
        const ido = new Interface(['function refund(uint256)'])
        const data = ido.encodeFunctionData('refund', [BigInt(context.id)])
        const from = web3Service.signer?.address || web3Service._signerAccount
        if (!from) throw new Error('No signer address for refund')
        const gasLimit = await this._estimateGasLimit(context.contractAddress, data, from, 300000n, { chainId: context.chainId })
        const txHash = await this._dualChannelSend(context.contractAddress, data, from, gasLimit)
        const receipt = await this._waitForReceipt(txHash, { chainId: context.chainId })
        console.warn('[tx-flow] refund done', { hash: txHash, status: receipt?.status })
        requireSuccessfulReceipt(receipt, txHash)
        return { hash: txHash, status: 'success' }
    }

    // ============ B端寫入操作 ============

    /**
     * 授權項目代幣給 JoinCare 合約
     * @param {string} tokenAddress
     * @param {string} amount - 可讀金額 或 'max'
     */
    async approveProjectToken(project, amount = 'max') {
        this._rejectFairLaunchOnLegacyWrite(project)
        const context = await this._ensureProjectSigner(project)
        const approveAmount = amount === 'max' ? MAX_UINT256 : parseUnits(String(amount), context.tokenDecimals)
        const data = new Interface(['function approve(address,uint256)']).encodeFunctionData('approve', [context.contractAddress, approveAmount])
        const from = web3Service.signer?.address || web3Service._signerAccount
        if (!from) throw new Error('No signer address for project token approval')
        const gasLimit = await this._estimateGasLimit(context.tokenAddress, data, from, 120000n, { chainId: context.chainId })
        const txHash = await this._dualChannelSend(context.tokenAddress, data, from, gasLimit)
        const receipt = await this._waitForReceipt(txHash, { chainId: context.chainId })
        requireSuccessfulReceipt(receipt, txHash)
        return { hash: txHash, status: 'success' }
    }

    /**
     * 項目方充值代幣
     */
    async depositProjectTokens(project, amount) {
        this._rejectFairLaunchOnLegacyWrite(project)
        const context = await this._ensureProjectSigner(project)
        const data = new Interface(['function depositProjectTokens(uint256,uint256)']).encodeFunctionData('depositProjectTokens', [BigInt(context.id), parseUnits(String(amount), context.tokenDecimals)])
        const from = web3Service.signer?.address || web3Service._signerAccount
        if (!from) throw new Error('No signer address for project token deposit')
        const gasLimit = await this._estimateGasLimit(context.contractAddress, data, from, 350000n, { chainId: context.chainId })
        const txHash = await this._dualChannelSend(context.contractAddress, data, from, gasLimit)
        const receipt = await this._waitForReceipt(txHash, { chainId: context.chainId })
        requireSuccessfulReceipt(receipt, txHash)
        return { hash: txHash, status: 'success' }
    }

    /**
     * 項目方提取募集的 JCC
     */
    async withdrawRaisedFunds(project) {
        this._rejectFairLaunchOnLegacyWrite(project)
        const context = await this._ensureProjectSigner(project)
        const data = new Interface(['function withdrawRaisedFunds(uint256)']).encodeFunctionData('withdrawRaisedFunds', [BigInt(context.id)])
        const from = web3Service.signer?.address || web3Service._signerAccount
        if (!from) throw new Error('No signer address for raised-funds withdrawal')
        const gasLimit = await this._estimateGasLimit(context.contractAddress, data, from, 300000n, { chainId: context.chainId })
        const txHash = await this._dualChannelSend(context.contractAddress, data, from, gasLimit)
        const receipt = await this._waitForReceipt(txHash, { chainId: context.chainId })
        requireSuccessfulReceipt(receipt, txHash)
        return { hash: txHash, status: 'success' }
    }

    /**
     * 項目方提取項目代幣
     */
    async withdrawProjectTokens(project) {
        this._rejectFairLaunchOnLegacyWrite(project)
        const context = await this._ensureProjectSigner(project)
        const data = new Interface(['function withdrawProjectTokens(uint256)']).encodeFunctionData('withdrawProjectTokens', [BigInt(context.id)])
        const from = web3Service.signer?.address || web3Service._signerAccount
        if (!from) throw new Error('No signer address for project-token withdrawal')
        const gasLimit = await this._estimateGasLimit(context.contractAddress, data, from, 300000n, { chainId: context.chainId })
        const txHash = await this._dualChannelSend(context.contractAddress, data, from, gasLimit)
        const receipt = await this._waitForReceipt(txHash, { chainId: context.chainId })
        requireSuccessfulReceipt(receipt, txHash)
        return { hash: txHash, status: 'success' }
    }

    // ============ 工具方法 ============

    /** 按項目鏈產生交易瀏覽器連結；未提供項目時保留既有預設鏈行為。 */
    getTxUrl(txHash, project = null) {
        const chainId = Number(project?.chainId)
        const explorer = chainId === 56 ? 'https://bscscan.com'
            : chainId === 97 ? 'https://testnet.bscscan.com'
            : chainId === 31337 ? '' : ACTIVE_CHAIN.explorerUrl
        return explorer ? `${explorer}/tx/${txHash}` : '#'
    }
}

export const contractService = new ContractService()

// 導出 class 供測試；isNetworkMismatchError 供調用方區分「網絡不匹配」與「RPC 失敗」
export { ContractService }
export { isNetworkMismatchError }
