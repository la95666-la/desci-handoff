<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { formatUnits } from 'ethers'
import Modal from './Modal.vue'
import { getProjectDetail, getFundingState } from '../services/api'
import { deriveProjectStage, getProjectCountdownTarget, isRefundableProjectStage } from '../utils/projectStatus'
import { contractService } from '../services/contract'
import { describePurchaseError, purchaseErrorDetails } from '../services/purchaseDiagnostics.js'
import { meetsMinPurchase } from '../utils/minPurchase.js'
import { useWalletStore } from '../stores/wallet'
import { resolveProjectFundingRoute } from '../utils/fairLaunch.js'
import { normalizeProjectContext } from '../services/projectContext.js'
import { createDisplaySingleFlight, withDisplayDeadline } from '../services/idoV3Display.js'
import { isUnresolvedWalletTransactionError } from '../services/fairLaunchWallet.js'
import { v3FundingStage, v3WalletActions } from '../utils/v3FundingStage.js'
import { v3UserMessage } from '../utils/v3UserMessages.js'
import { computeSoftAndHardRequirement } from '../utils/depositMath.js'
import { paymentApprovalState, reconcilePaymentAllowance } from '../utils/paymentApproval.js'
import { watchWalletResume } from '../utils/walletResume.js'
import { purchaseFeedbackKey, isLocalPurchasePending } from '../utils/v3PurchaseFeedback.js'
import { formatRaisedAmount } from '../utils/raisedAmount.js'
import { v3ClaimPending } from '../utils/v3ClaimPending.js'

const props = defineProps({
  project: { type: Object, required: true },
  language: { type: String, default: 'en' },
  isDark: { type: Boolean, default: false },
})

const wallet = useWalletStore()
const isV3 = computed(() => resolveProjectFundingRoute(props.project).kind === 'ido_v3')
const v3Context = computed(() => normalizeProjectContext(props.project, {}))
const v3State = ref(null)
const v3Failure = ref('')
const v3Qualified = ref(false)
const v3Pending = ref([])
const claimPending = computed(() => v3ClaimPending(v3Pending.value))
const v3ClaimStage = computed(() => v3State.value?.status === 2 || props.project?.status === 'Successful')
const recoveryPending = computed(() => v3ClaimStage.value ? claimPending.value : v3Pending.value)
const activePurchaseFeedbackKey = ref('')
const currentPurchaseFeedbackKey = computed(() => purchaseFeedbackKey(props.project, wallet.address))
const v3LocalPurchaseBusy = computed(() => isV3.value && isPurchasing.value &&
  activePurchaseFeedbackKey.value === currentPurchaseFeedbackKey.value)
const pendingNeedsRecovery = computed(() => recoveryPending.value.some(entry =>
  !(entry.action === 'approvePaymentToken' && entry.hash) &&
  !isLocalPurchasePending(entry, activePurchaseFeedbackKey.value, currentPurchaseFeedbackKey.value)))
const v3Claimable = ref(0n)
const v3OwnerFunding = ref(null)
const v3Busy = ref(false)
const v3Loading = ref(false)
let v3Revision = 0
let disposed = false
const approvalRefreshTimers = new Set()
let approvalReconcileFlight = null
let stopWalletResume = null
let v3ReadEpoch = 0
const v3DisplayFlight = createDisplaySingleFlight()
const v3DisplayedKey = ref('')
const v3ReadError = ref('')
const v3FieldErrors = ref({})
const v3ReadKey = computed(() => JSON.stringify([
  props.project?.id, props.project?.contractVersion, props.project?.fundingMode,
  props.project?.contractAddress, props.project?.chainId, props.project?.onchainLocked,
  props.project?.paymentTokenAddress, props.project?.tokenAddress,
  props.project?.paymentTokenDecimals, props.project?.tokenDecimals, props.project?.feeReceiver, props.project?.ownerAddress,
  wallet.address, wallet.chainId, wallet.isConnected, wallet.isLoggedIn, wallet.sessionRevision,
]))
const v3DisplayCurrent = computed(() => v3DisplayedKey.value === v3ReadKey.value && !v3ReadError.value)
const v3WalletReady = computed(() => v3DisplayCurrent.value && !Object.keys(v3FieldErrors.value).length)
const v3Unresolved = computed(() => v3Pending.value.length > 0)
const v3ApprovalSubmitted = computed(() => v3Pending.value.length > 0 &&
  v3Pending.value.every(entry => entry.action === 'approvePaymentToken' && entry.hash))
const v3ApprovalReadyForPurchase = computed(() => v3ApprovalSubmitted.value &&
  !v3FieldErrors.value.allowance && !v3ReadError.value && !needsApproval.value)
const v3CanPurchase = computed(() => isV3.value && wallet.isConnected && wallet.isLoggedIn &&
  (!v3Unresolved.value || v3ApprovalReadyForPurchase.value) && !v3Busy.value)
const v3Actions = computed(() => v3WalletActions(v3State.value, userPurchaseInfo.value, v3Claimable.value))
const v3Refundable = computed(() => v3Actions.value.refund)
const v3OwnerRequiredRaw = computed(() => {
  if (!v3State.value) return 0n
  return computeSoftAndHardRequirement({
    softCapRaw: v3State.value.project.softCap,
    hardCapRaw: v3State.value.project.hardCap,
    tokenPriceRaw: v3State.value.project.tokenPrice,
    depositedRaw: v3State.value.project.totalTokensDeposited,
    tokenDecimals: v3Context.value.tokenDecimals,
  }).shortfallRaw
})
const v3OwnerAmount = computed(() => formatUnits(v3OwnerRequiredRaw.value, v3Context.value.tokenDecimals))
const v3OwnerValid = computed(() => v3OwnerRequiredRaw.value > 0n &&
  v3WalletReady.value &&
  v3OwnerRequiredRaw.value <= (v3OwnerFunding.value?.balance ?? 0n))
const v3OwnerNeedsApproval = computed(() => v3OwnerValid.value &&
  v3OwnerFunding.value.allowance < v3OwnerRequiredRaw.value)
const v3FailureMessage = computed(() => v3UserMessage(v3Failure.value, isZh.value))
const v3ActionHint = computed(() => {
  if (!wallet.isLoggedIn) return isZh.value ? '請連接錢包並登入' : 'Connect your wallet and sign in'
  if (v3ApprovalSubmitted.value) return isZh.value ? '授權已提交，正在更新…' : 'Approval submitted, updating…'
  if (v3Failure.value) return v3FailureMessage.value
  return isZh.value ? '目前尚未開放認購' : 'Participation is not open yet'
})
const paymentAllowance = ref(0n)
const paymentBalance = ref({ formatted: '0', raw: 0n })
const userPurchaseInfo = ref(null)
const purchaseAmount = ref('')
const pendingPaymentApprovalAmount = ref('')
const latestMinPolicy = ref(null)
const purchasePolicy = computed(() => latestMinPolicy.value || props.project)
const raised = ref(Number(props.project?.raised || 0))
const isLoadingChain = ref(false)
const isApproving = ref(false)
const isPurchasing = ref(false)
const isRefunding = ref(false)
const isClaiming = ref(false)
const now = ref(Date.now())
const showModal = ref(false)
const modalConfig = ref({ type: 'success', title: '', message: '', txHash: '', diagnostic: '', details: [] })
const showErrorDetails = ref(false)
let timer = null
let fundingRefreshTimer = null
let dataRevision = 0
let confirmedRaisedFloor = null
let confirmedUserFloor = null
let confirmedAllowanceFloor = null
let v3AllowanceConfirmation = null
let fundingReadSequence = 0
let raisedReadSequence = 0

const isZh = computed(() => props.language && props.language !== 'en')
const t = computed(() => isZh.value ? {
  approve: '授權支付幣',
  approving: '授權中...',
  approveError: '授權失敗',
  approveSuccess: '授權成功',
  approveSuccessMessage: '支付幣已授權，現在可以進行認購。',
  balance: '可用餘額',
  connect: '連接錢包並參與',
  enterAmount: '輸入認購金額',
  estimated: '預計獲得',
  fundingTerms: 'FUNDING TERMS',
  hardCap: '募資硬頂',
  insufficientBalance: '支付幣餘額不足',
  exceedsQuota: '超過可認購額度',
  invested: '我的投入',
  live: '鏈上募資進行中',
  comingSoon: '即將開始',
  pendingActivation: '等待鏈上激活 · 暫不可認購',
  awaitingSaleStart: '等待募資開始 · 暫不可認購',
  startsIn: '距離募資開始',
  day: '天', hour: '時', minute: '分', second: '秒',
  awaitingStart: '募資尚未開放，請等待鏈上激活。',
  closed: '募資已結束',
  result: '募資結果',
  success: '募資成功',
  failed: '未達軟頂',
  canceled: '項目已取消',
  loading: '正在讀取鏈上資料...',
  maxPerUser: '單錢包上限',
  minPurchase: '每筆最低參與額',
  belowMinPurchase: '低於官方頁面每筆最低參與額',
  ok: '確定',
  price: '代幣價格',
  progress: '募集進度',
  purchase: '立即認購',
  purchaseError: '認購失敗',
  purchasing: '認購中...',
  purchaseSuccess: '認購成功',
  purchaseSuccessMessage: '交易已提交並完成，鏈上資料正在更新。',
  refund: '退款',
  refunding: '退款中...',
  refunded: '已退款',
  refundSuccess: '退款成功',
  refundSuccessMessage: '認購款已退回您的錢包。',
  refundPending: '退款狀態待確認',
  refundError: '退款失敗',
  claimPending: '領取狀態待確認',
  settlementPending: '等待鏈上結算',
  settlementPendingMessage: '募資時間已結束且未達軟頂。項目需先在鏈上結算為失敗狀態，結算完成後才可退款。',
  raised: '已募集',
  remaining: '剩餘額度',
  risk: '鏈上交易不可撤銷。認購前請核對網路、金額與風險揭露。',
  softCap: '募資軟頂',
  transaction: '交易雜湊',
  diagnostic: '診斷代碼（可截圖提供客服）',
  showDetails: '顯示詳細錯誤',
  hideDetails: '收起詳細錯誤',
  detailsNotice: '僅包含篩選後的診斷欄位；可截圖提供客服。',
  viewTransaction: '查看鏈上交易',
} : {
  approve: 'Approve payment token',
  approving: 'Approving...',
  approveError: 'Approval failed',
  approveSuccess: 'Approval successful',
  approveSuccessMessage: 'The payment token is approved. You can now participate.',
  balance: 'Available balance',
  connect: 'Connect wallet to participate',
  enterAmount: 'Enter participation amount',
  estimated: 'Estimated tokens',
  fundingTerms: 'FUNDING TERMS',
  hardCap: 'Hard cap',
  insufficientBalance: 'Insufficient payment balance',
  exceedsQuota: 'Exceeds remaining quota',
  invested: 'My contribution',
  live: 'On-chain funding is live',
  comingSoon: 'Coming soon',
  pendingActivation: 'Awaiting on-chain activation · Purchases unavailable',
  awaitingSaleStart: 'Funding has not started · Purchases unavailable',
  startsIn: 'Funding starts in',
  day: 'd', hour: 'h', minute: 'm', second: 's',
  awaitingStart: 'Funding is not open yet. Await on-chain activation.',
  closed: 'Funding ended',
  result: 'Funding result',
  success: 'Funding succeeded',
  failed: 'Below soft cap',
  canceled: 'Project canceled',
  loading: 'Loading on-chain data...',
  maxPerUser: 'Per-wallet limit',
  minPurchase: 'Minimum per purchase',
  belowMinPurchase: 'Below official-page minimum per purchase',
  ok: 'OK',
  price: 'Token price',
  progress: 'Funding progress',
  purchase: 'Participate now',
  purchaseError: 'Participation failed',
  purchasing: 'Submitting...',
  purchaseSuccess: 'Participation successful',
  purchaseSuccessMessage: 'The transaction completed successfully. On-chain data is being refreshed.',
  refund: 'Refund',
  refunding: 'Refunding...',
  refunded: 'Refunded',
  refundSuccess: 'Refund successful',
  refundSuccessMessage: 'Your contribution has been refunded to your wallet.',
  refundPending: 'Refund confirmation pending',
  refundError: 'Refund failed',
  claimPending: 'Claim confirmation pending',
  settlementPending: 'Awaiting on-chain settlement',
  settlementPendingMessage: 'Funding has ended below the soft cap. Refunds will become available after the project is settled as failed on-chain.',
  raised: 'Raised',
  remaining: 'Remaining quota',
  risk: 'On-chain transactions are irreversible. Verify the network, amount and risk disclosures before participating.',
  softCap: 'Soft cap',
  transaction: 'Transaction hash',
  diagnostic: 'Diagnostic code (share a screenshot with support)',
  showDetails: 'Show error details',
  hideDetails: 'Hide error details',
  detailsNotice: 'Filtered diagnostic fields only. You can screenshot these for support.',
  viewTransaction: 'View transaction',
})

const stageKey = computed(() => {
  if (isV3.value) return v3FundingStage(props.project, v3State.value, now.value)
  // Without a verified V3 chain read the marketing stage is display-only;
  // wallet writes below remain closed.
  return deriveProjectStage(props.project, now.value)
})

const isLive = computed(() => stageKey.value === 'live')
const isPending = computed(() => stageKey.value === 'pending')
const isUpcoming = computed(() => stageKey.value === 'upcoming')
const startCountdown = computed(() => {
  if (!isPending.value && !isUpcoming.value) return null
  const start = getProjectCountdownTarget(props.project, stageKey.value)
  if (!Number.isSafeInteger(start) || start <= 0) return null
  const seconds = Math.max(0, Math.ceil((start * 1000 - now.value) / 1000))
  if (seconds === 0) return null
  return {
    d: Math.floor(seconds / 86400),
    h: Math.floor((seconds % 86400) / 3600),
    m: Math.floor((seconds % 3600) / 60),
    s: seconds % 60,
  }
})

// 认购状态：进行中 / 即将开始 / 已结束（用于 badge 与购买区显隐）
const statusInfo = computed(() => {
  if (stageKey.value === 'live') return { live: true, label: t.value.live }
  if (stageKey.value === 'upcoming') return { live: false, label: t.value.comingSoon }
  if (isPending.value) return { live: false, label: t.value.comingSoon }
  return { live: false, label: t.value.closed }
})

// 募资结果：达到软顶视为成功（已结束态展示用）
const fundingSucceeded = computed(() => {
  const soft = Number(props.project?.softCap || 0)
  return soft > 0 && raised.value >= soft
})

const isTerminal = computed(() => [
  'ended_success', 'ended_failed', 'successful', 'failed', 'canceled', 'ended', 'refund',
].includes(stageKey.value))

const resultInfo = computed(() => {
  if (stageKey.value === 'successful' || stageKey.value === 'ended_success') {
    return { label: t.value.success, className: 'text-[#1aa64a]' }
  }
  if (stageKey.value === 'canceled') {
    return { label: t.value.canceled, className: 'text-[#d46b08]' }
  }
  if (stageKey.value === 'failed' || stageKey.value === 'ended_failed' || stageKey.value === 'refund') {
    return { label: t.value.failed, className: 'text-[#cf1322]' }
  }
  return fundingSucceeded.value
    ? { label: t.value.success, className: 'text-[#1aa64a]' }
    : { label: t.value.failed, className: 'text-[#cf1322]' }
})

// terms 格子：进行中/未开始显示全部 4 项，已结束只显示软顶硬顶
const termItems = computed(() => {
  const p = props.project || {}
  const items = [
    [t.value.softCap, `${Number(p.softCap || 0).toLocaleString()} ${p.paymentToken || ''}`],
    [t.value.hardCap, `${Number(p.hardCap || 0).toLocaleString()} ${p.paymentToken || ''}`],
  ]
  if (!isTerminal.value) {
    items.unshift(
      [t.value.maxPerUser, `${Number(p.maxPerUser || 0).toLocaleString()} ${p.paymentToken || ''}`],
      [t.value.price, `1 ${p.tokenSymbol || ''} = ${p.tokenPrice || ''} ${p.paymentToken || ''}`],
    )
    if (purchasePolicy.value.minPurchaseAmountRaw) items.push([t.value.minPurchase, `${purchasePolicy.value.minPurchaseAmount} ${p.paymentToken || ''}`])
  }
  return items
})
const fundingProgress = computed(() => {
  const cap = Number(props.project?.hardCap || 0)
  if (!cap) return 0
  return Math.min(100, Math.max(0, (raised.value / cap) * 100))
})
const userInvested = computed(() => Number(
  userPurchaseInfo.value?.paymentAmount || userPurchaseInfo.value?.totalJCCSpent || 0,
))
const remainingQuota = computed(() => Math.max(0, Number(props.project?.maxPerUser || 0) - userInvested.value))
const tokensToReceive = computed(() => {
  const amount = Number(purchaseAmount.value)
  const price = Number(props.project?.tokenPrice || 0)
  return amount > 0 && price > 0 ? amount / price : 0
})
const amountIsValid = computed(() => {
  try {
    const raw = contractService.parsePaymentAmount(props.project, purchaseAmount.value)
    const invested = userPurchaseInfo.value?.paymentAmountRaw || 0n
    if (isV3.value) return v3CanPurchase.value && raw > 0n && meetsMinPurchase(raw, purchasePolicy.value)
    return raw > 0n && meetsMinPurchase(raw, purchasePolicy.value) &&
      raw <= paymentBalance.value.raw && raw + invested <= BigInt(props.project.maxPerUserRaw || 0)
  } catch { return false }
})
const belowMinPurchase = computed(() => {
  try {
    const raw = contractService.parsePaymentAmount(props.project, purchaseAmount.value)
    return raw > 0n && !meetsMinPurchase(raw, purchasePolicy.value)
  } catch { return false }
})
const needsApproval = computed(() => {
  try {
    return paymentApprovalState(contractService.parsePaymentAmount(props.project, purchaseAmount.value), paymentAllowance.value).needsApproval
  } catch {
    return false
  }
})
const v3NeedsApproval = computed(() => {
  return isV3.value && needsApproval.value
})
const canPurchase = computed(() => wallet.isConnected && isLive.value && amountIsValid.value && (!isV3.value || v3CanPurchase.value))
const paymentActionBusy = computed(() => isApproving.value || isPurchasing.value || v3Busy.value)
const settlementPending = computed(() => stageKey.value === 'ended_failed')

// 退款入口：仅链上/后端已同步为失败或取消态时开放。
// ended_failed 只是时间派生态，链上可能仍是 Active，必须等待 endProject 结算。
const showRefund = computed(() => isV3.value ? v3Refundable.value :
  isRefundableProjectStage(stageKey.value) && userInvested.value > 0 && !userPurchaseInfo.value?.refunded)
const canRefund = computed(() => wallet.isConnected && showRefund.value && (!isV3.value || (wallet.isLoggedIn && !v3Unresolved.value && !v3Busy.value)))
const canClaimV3 = computed(() => isV3.value && wallet.isConnected && wallet.isLoggedIn &&
  v3Actions.value.claim && !claimPending.value.length && !v3Busy.value)

function friendlyError(error) {
  if (String(error?.code || '').startsWith('IDO_V3_')) return v3UserMessage(error.code, isZh.value)
  const message = error?.reason || error?.message || ''
  if (error?.code === 'TRANSACTION_BROADCAST_UNCERTAIN') {
    return isZh.value
      ? '錢包未返回有效交易哈希，廣播狀態不明。請先檢查錢包活動與鏈上記錄，確認無交易後再重試。'
      : 'The wallet did not return a valid transaction hash. Check wallet activity and on-chain records before trying again.'
  }
  if (error?.code === 'TRANSACTION_RECEIPT_TIMEOUT') {
    return isZh.value
      ? '交易已取得哈希，但回執確認超時。請先在區塊瀏覽器確認狀態，切勿立即重複提交。'
      : 'The transaction has a hash, but receipt confirmation timed out. Check the explorer before submitting again.'
  }
  if (error?.code === 'TRANSACTION_REVERTED') {
    return isZh.value
      ? '交易已上鏈但執行失敗，資產未完成認購。請在區塊瀏覽器查看失敗交易後再處理。'
      : 'The transaction was mined but reverted. The purchase did not complete; inspect it in the explorer before proceeding.'
  }
  if (message.includes('user rejected') || message.includes('ACTION_REJECTED') || error?.code === 'ACTION_REJECTED') {
    return isZh.value ? '您已取消交易。' : 'The transaction was cancelled.'
  }
  if (message.includes('insufficient funds') || message.includes('INSUFFICIENT_FUNDS')) {
    return isZh.value ? '餘額不足，無法完成交易。' : 'Insufficient balance to complete this transaction.'
  }
  if (message.includes('execution reverted') || message.includes('CALL_EXCEPTION')) {
    return isZh.value ? '合約拒絕了本次操作，請檢查金額與網路。' : 'The contract rejected this operation. Check the amount and network.'
  }
  return message && message.length <= 120
    ? message
    : isZh.value ? '交易失敗，請稍後重試。' : 'Transaction failed. Please try again.'
}

async function loadOnChainData() {
  if (isV3.value) return refreshV3()
  const sequence = ++fundingReadSequence
  const projectId = String(props.project?.id ?? '')
  const address = wallet.address
  const revision = dataRevision
  if (!wallet.address || !props.project) {
    paymentAllowance.value = 0n
    paymentBalance.value = { formatted: '0', raw: 0n }
    userPurchaseInfo.value = null
    return
  }
  isLoadingChain.value = true
  try {
    const state = await getFundingState(projectId)
    if (sequence !== fundingReadSequence || revision !== dataRevision || projectId !== String(props.project?.id ?? '') || address?.toLowerCase() !== wallet.address?.toLowerCase()) return
    const nextAllowance = BigInt(state.allowanceRaw || '0')
    paymentAllowance.value = confirmedAllowanceFloor !== null && nextAllowance < confirmedAllowanceFloor
      ? confirmedAllowanceFloor
      : nextAllowance
    paymentBalance.value = { formatted: state.balanceDisplay || '0', raw: BigInt(state.balanceRaw || '0') }
    const nextPurchase = {
      paymentAmount: Number(state.contributionDisplay || 0),
      paymentAmountRaw: BigInt(state.contributionRaw || '0'),
      totalTokensBought: Number(state.tokensBoughtDisplay || 0),
      totalTokensBoughtRaw: BigInt(state.tokensBoughtRaw || '0'),
      claimedTokens: Number(state.claimedDisplay || 0),
      claimedTokensRaw: BigInt(state.claimedRaw || '0'),
      refunded: Boolean(state.refundedRaw && state.refundedRaw !== '0'),
    }
    // The backend may serve a pre-receipt position briefly even though its
    // chain read normally wins. Do not roll back a confirmed wallet read.
    if (confirmedUserFloor !== null && nextPurchase.paymentAmountRaw < confirmedUserFloor) return
    userPurchaseInfo.value = nextPurchase
  } catch (error) {
    console.warn('[active funding] failed to load on-chain data:', error)
  } finally {
    isLoadingChain.value = false
  }
}

// Presentation requests share one fixed-block snapshot. A display snapshot is
// never a purchase authority: investor calls are independently simulated.
function refreshV3(force = false) {
  if (disposed) return Promise.resolve()
  if (force === true) v3ReadEpoch++
  const key = v3ReadKey.value
  return v3DisplayFlight(`${key}:${v3ReadEpoch}`, () => loadV3Display(key))
}

async function loadV3Display(key) {
  if (disposed || key !== v3ReadKey.value) return
  const revision = ++v3Revision
  const current = () => !disposed && revision === v3Revision && key === v3ReadKey.value
  if (!current()) return
  v3Loading.value = true
  const address = wallet.address
  try {
    const context = v3Context.value
    const displayWallet = wallet.isConnected && address ? address : null
    if (displayWallet) {
      v3Pending.value = contractService.v3Pending(context, address)
      // A restored hash-bearing approval is normal pending work, not a display
      // error. Reconcile independently so slow receipt RPC cannot delay allowance.
      if (v3ApprovalSubmitted.value && !approvalReconcileFlight) {
        approvalReconcileFlight = contractService.reconcileV3Pending(context, address)
          .then(settled => { if (settled && !disposed && key === v3ReadKey.value) refreshV3(true).catch(() => {}) })
          .catch(() => {}).finally(() => { approvalReconcileFlight = null })
      }
      // No pending journal means no receipt/RPC reconciliation work to do.
      if (pendingNeedsRecovery.value) {
        await withDisplayDeadline(() => contractService.reconcileV3Pending(context, address, v3ClaimStage.value ? 'claim(uint256)' : null))
        if (!current()) return
        v3Pending.value = contractService.v3Pending(context, address)
      }
    }
    const result = await contractService.readV3Display(context, displayWallet)
    if (!current()) return
    v3State.value = result.state
    v3OwnerFunding.value = result.ownerFunding
    v3Claimable.value = result.claimable?.raw ?? 0n
    // Eligibility is checked at the dedicated Approve/Purchase preflight, not
    // by fetching proof for a visitor who is merely reading the page.
    v3Qualified.value = result.state.saleOpen === true
    v3Failure.value = pendingNeedsRecovery.value ? 'IDO_V3_SUBMISSION_UNRESOLVED' : ''
    v3ReadError.value = ''
    v3FieldErrors.value = result.errors
    v3DisplayedKey.value = key
    paymentBalance.value = result.balance || { formatted: '0', raw: 0n }
    const displayAllowance = result.allowance?.raw ?? 0n
    if (!result.errors.allowance) {
      const reconciled = reconcilePaymentAllowance(displayAllowance, result.state.blockNumber, v3AllowanceConfirmation)
      paymentAllowance.value = reconciled.raw
      v3AllowanceConfirmation = reconciled.confirmation
    }
    if (confirmedUserFloor === null || !result.position || result.position.paymentAmountRaw >= confirmedUserFloor) {
      userPurchaseInfo.value = result.position
    }
    const nextRaised = Number(formatUnits(result.state.project.totalRaised, context.paymentTokenDecimals))
    raised.value = confirmedRaisedFloor === null ? nextRaised : Math.max(confirmedRaisedFloor, nextRaised)
  } catch (error) {
    if (!current()) return
    try { v3Pending.value = address ? contractService.v3Pending(v3Context.value, address) : [] } catch { v3Pending.value = [] }
    // Keep the last display values, mark them stale and close action gates.
    v3ReadError.value = String(error?.code || '').startsWith('IDO_V3_') ? error.code : 'IDO_V3_STATE_UNAVAILABLE'
    if (pendingNeedsRecovery.value) v3Failure.value = 'IDO_V3_SUBMISSION_UNRESOLVED'
  } finally {
    if (current()) v3Loading.value = false
  }
}

async function refreshRaised() {
  if (isV3.value) return refreshV3()
  const sequence = ++raisedReadSequence
  const projectId = String(props.project?.id ?? '')
  const revision = dataRevision
  // 优先后端接口（可靠且不依赖钱包通道）；链读仅作为后备
  try {
    const detail = await getProjectDetail(projectId)
    if (sequence !== raisedReadSequence || revision !== dataRevision || projectId !== String(props.project?.id ?? '')) return
    const nextRaised = Number(detail?.totalRaised)
    if (Number.isFinite(nextRaised)) {
      if (confirmedRaisedFloor !== null && nextRaised < confirmedRaisedFloor) return
      raised.value = nextRaised
    }
    return
  } catch { /* fallthrough */ }

  if (wallet.isConnected) {
    try {
      const chainProject = await contractService.getProject(props.project)
      if (sequence !== raisedReadSequence || revision !== dataRevision || projectId !== String(props.project?.id ?? '')) return
      const nextRaised = Number(chainProject?.totalRaised)
      if (Number.isFinite(nextRaised)) {
        if (confirmedRaisedFloor !== null && nextRaised < confirmedRaisedFloor) return
        raised.value = nextRaised
      }
    } catch (chainError) {
      console.warn('[active funding] failed to refresh raised amount from chain:', chainError)
    }
  }
}

async function connectWallet() {
  try {
    await wallet.connectWallet()
  } catch (error) {
    console.warn('[active funding] wallet connect failed:', error)
  }
}

async function handleApprove() {
  if (!canPurchase.value || !needsApproval.value || paymentActionBusy.value) return
  const approvedKey = v3ReadKey.value
  const approvedAddress = wallet.address
  isApproving.value = true
  try {
    const approvedAmount = purchaseAmount.value
    pendingPaymentApprovalAmount.value = approvedAmount
    console.warn('[approve-flow] start', { projectId: props.project?.id, amount: approvedAmount, connected: wallet.isConnected })
    const t0 = Date.now()
    const result = isV3.value
      ? await contractService.approveV3PaymentToken(v3Context.value, approvedAmount, wallet.address)
      : await contractService.approvePaymentToken(props.project, approvedAmount)
    if (disposed || approvedKey !== v3ReadKey.value || approvedAddress !== wallet.address) return
    console.warn('[approve-flow] tx done', { hash: result.hash, status: result.status, ms: Date.now() - t0 })
    if (isV3.value && result.status === 'submitted') {
      // No optimistic allowance from a wallet acknowledgement. Display only
      // actual chain reads; receipt failure/unknown must not create fake credit.
      v3AllowanceConfirmation = null
      v3Pending.value = contractService.v3Pending(v3Context.value, approvedAddress)
      v3Failure.value = ''
      modalConfig.value = {
        type: 'info', title: isZh.value ? '操作已確認' : 'Operation confirmed',
        message: isZh.value ? '錢包已提交授權，投資面板與授權餘額正在更新。' : 'Your wallet submitted the approval. The investment panel and allowance are refreshing.',
        txHash: result.hash,
      }
      showModal.value = true
      const current = () => !disposed && approvedKey === v3ReadKey.value && approvedAddress === wallet.address
      refreshV3(true).catch(() => {})
      // Refresh actual allowance while mined confirmations accumulate. Timers
      // cannot send transactions, and stale account/project results are ignored.
      for (const delay of [2000, 5000, 10000, 20000, 40000]) {
        const timer = setTimeout(() => {
          approvalRefreshTimers.delete(timer)
          if (current()) refreshV3(true).catch(() => {})
        }, delay)
        approvalRefreshTimers.add(timer)
      }
      result.confirmation.then(() => { if (current()) refreshV3(true).catch(() => {}) }, () => {
        if (!current()) return
        // Observe the original hash only. Unknown/reverted outcomes retain the
        // service fence; do not turn the wallet acknowledgement into a failure popup.
        contractService.reconcileV3Pending(v3Context.value, approvedAddress)
          .catch(() => {}).finally(() => { if (current()) refreshV3(true).catch(() => {}) })
      })
      return
    }
    // The successful receipt proves this exact allowance was applied. Update
    // the action gate immediately; the independent official-RPC read below is
    // confirmation only and must never leave the button waiting on TP's RPC.
    const approvedRaw = contractService.parsePaymentAmount(isV3.value ? v3Context.value : props.project, approvedAmount)
    if (approvedRaw > paymentAllowance.value) paymentAllowance.value = approvedRaw
    confirmedAllowanceFloor = approvedRaw
    if (isV3.value) {
      // Optimistic allowance is valid only until a read reaches this receipt.
      v3AllowanceConfirmation = { raw: approvedRaw, blockNumber: result.blockNumber }
      refreshV3(true).catch(() => {})
      showModal.value = false
      return
    }
    // Background confirmation uses the fixed official BSC RPC, not the wallet
    // provider/runtime BlockRazor route, and never blocks button recovery.
    contractService.getPaymentTokenAllowance(isV3.value ? v3Context.value : props.project, wallet.address)
      .then((a) => { if (a.raw >= approvedRaw) paymentAllowance.value = a.raw })
      .catch(() => {})
    setTimeout(() => {
      contractService.getPaymentTokenAllowance(isV3.value ? v3Context.value : props.project, wallet.address)
        .then((a) => { if (a.raw >= approvedRaw) paymentAllowance.value = a.raw })
        .catch(() => {})
    }, 2000)
    showModal.value = false
  } catch (error) {
    if (disposed || approvedKey !== v3ReadKey.value || approvedAddress !== wallet.address) return
    console.warn('[approve-flow] failed:', error)
    if (isV3.value) {
      try { v3Pending.value = contractService.v3Pending(v3Context.value, wallet.address) } catch { v3Pending.value = [] }
      if (v3Pending.value.length) v3Failure.value = 'IDO_V3_SUBMISSION_UNRESOLVED'
    }
    modalConfig.value = {
      type: v3Pending.value.length ? 'warning' : error?.code === 'IDO_V3_WALLET_REJECTED' ? 'info' : 'error',
      title: error?.code === 'IDO_V3_WALLET_REJECTED' ? (isZh.value ? '已取消授權' : 'Approval cancelled') : t.value.approveError,
      message: v3Pending.value.length ? v3UserMessage('IDO_V3_SUBMISSION_UNRESOLVED', isZh.value) : friendlyError(error), txHash: error?.transactionHash || '',
    }
    showModal.value = true
  } finally {
    isApproving.value = false
    pendingPaymentApprovalAmount.value = ''
  }
}

async function handlePurchase() {
  if (!canPurchase.value || needsApproval.value || paymentActionBusy.value) return
  const purchaseKey = v3ReadKey.value
  activePurchaseFeedbackKey.value = currentPurchaseFeedbackKey.value
  if (isV3.value) v3Failure.value = ''
  isPurchasing.value = true
  try {
    const projectId = String(props.project.id)
    const address = wallet.address
    const project = props.project
    // V3 uses the already loaded official-page policy locally. This is not
    // contract authority and must not add RPC or a blocking identity/API audit.
    if (isV3.value && !meetsMinPurchase(contractService.parsePaymentAmount(v3Context.value, purchaseAmount.value), purchasePolicy.value)) {
      throw new Error(t.value.belowMinPurchase)
    }
    // Keep V2's existing latest-detail policy check unchanged.
    if (!isV3.value) {
      const latest = await getProjectDetail(props.project.id)
      latestMinPolicy.value = latest
      if (!meetsMinPurchase(contractService.parsePaymentAmount(props.project, purchaseAmount.value), latest)) {
        throw new Error(t.value.belowMinPurchase)
      }
    }
    const confirmedAmountRaw = contractService.parsePaymentAmount(isV3.value ? v3Context.value : props.project, purchaseAmount.value)
    const userBeforeRaw = userPurchaseInfo.value?.paymentAmountRaw || 0n
    const result = isV3.value
      ? await contractService.purchaseV3(v3Context.value, purchaseAmount.value, address)
      : await contractService.purchase(props.project, purchaseAmount.value)
    if (disposed || projectId !== String(props.project?.id ?? '') || address?.toLowerCase() !== wallet.address?.toLowerCase()) return
    if (isV3.value) {
      v3Pending.value = contractService.v3Pending(v3Context.value, address)
      v3Failure.value = ''
    }
    // Receipt has succeeded. Start direct chain reads immediately, independently
    // of backend indexing and wallet balance refresh; never delay the modal.
    const revision = ++dataRevision
    const stillCurrent = () => !disposed && revision === dataRevision && projectId === String(props.project?.id ?? '') && address?.toLowerCase() === wallet.address?.toLowerCase()
    const expectedUserRaw = (confirmedUserFloor && confirmedUserFloor > userBeforeRaw ? confirmedUserFloor : userBeforeRaw) + confirmedAmountRaw
    // A successful purchase receipt guarantees both counters increased by the
    // submitted payment amount. Apply that confirmed floor synchronously so a
    // stale Backend response or a pending wallet RPC cannot leave the panel at
    // zero while the official-RPC reads catch up.
    const confirmedDisplayAmount = Number(purchaseAmount.value)
    const previousUserDisplay = Number(userPurchaseInfo.value?.paymentAmount || userPurchaseInfo.value?.totalJCCSpent || 0)
    if (Number.isFinite(confirmedDisplayAmount) && confirmedDisplayAmount > 0) {
      const nextRaisedFloor = raised.value + confirmedDisplayAmount
      raised.value = nextRaisedFloor
      confirmedRaisedFloor = confirmedRaisedFloor === null
        ? nextRaisedFloor
        : Math.max(confirmedRaisedFloor, nextRaisedFloor)
      const nextUserDisplay = previousUserDisplay + confirmedDisplayAmount
      userPurchaseInfo.value = {
        ...(userPurchaseInfo.value || {}),
        paymentAmount: String(nextUserDisplay),
        paymentAmountRaw: expectedUserRaw,
      }
      confirmedUserFloor = expectedUserRaw
      const remainingAllowance = paymentAllowance.value > confirmedAmountRaw
        ? paymentAllowance.value - confirmedAmountRaw
        : 0n
      paymentAllowance.value = remainingAllowance
      confirmedAllowanceFloor = remainingAllowance
      if (isV3.value) v3AllowanceConfirmation = null
    }
    const readConfirmedChain = () => {
      if (isV3.value) {
        refreshV3(true).catch(() => {})
        return
      }
      contractService.getProject(isV3.value ? v3Context.value : project).then((chainProject) => {
        if (!stillCurrent() || typeof chainProject?.totalRaisedRaw !== 'bigint') return
        const next = Number(chainProject.totalRaised)
        if (Number.isFinite(next) && (confirmedRaisedFloor === null || next >= confirmedRaisedFloor)) {
          raised.value = next
          confirmedRaisedFloor = next
        }
      }).catch(() => {})
      contractService.getUserPurchase(isV3.value ? v3Context.value : project, address).then((position) => {
        if (!stillCurrent() || typeof position?.paymentAmountRaw !== 'bigint' || position.paymentAmountRaw < expectedUserRaw) return
        userPurchaseInfo.value = position
        confirmedUserFloor = position.paymentAmountRaw
      }).catch(() => {})
    }
    readConfirmedChain()
    if (!isV3.value) {
      loadOnChainData().catch(() => {})
      refreshRaised().catch(() => {})
    }
    setTimeout(() => { if (stillCurrent()) { readConfirmedChain(); if (!isV3.value) { loadOnChainData().catch(() => {}); refreshRaised().catch(() => {}) } } }, 2000)
    modalConfig.value = {
      type: 'success', title: t.value.purchaseSuccess, message: t.value.purchaseSuccessMessage, txHash: result.hash,
    }
    purchaseAmount.value = ''
  } catch (error) {
    if (disposed || purchaseKey !== v3ReadKey.value) return
    if (isV3.value) {
      try { v3Pending.value = contractService.v3Pending(v3Context.value, wallet.address) } catch { v3Pending.value = [] }
      if (v3Pending.value.length) v3Failure.value = 'IDO_V3_SUBMISSION_UNRESOLVED'
      else if (error?.code === 'IDO_V3_ALLOWANCE_INSUFFICIENT') {
        // A fresh preflight beats any local receipt floor. Preserve the user's
        // amount, return to approval, and reconcile the exact spender on-chain.
        confirmedAllowanceFloor = null
        v3AllowanceConfirmation = null
        paymentAllowance.value = 0n
        refreshV3(true).catch(() => {})
      }
    }
    const detail = v3Pending.value.length
      ? { message: v3UserMessage('IDO_V3_SUBMISSION_UNRESOLVED', isZh.value), diagnostic: 'PURCHASE/SEND/IDO_V3_SUBMISSION_UNRESOLVED' }
      : String(error?.code || '').startsWith('IDO_V3_')
      ? { message: v3UserMessage(error.code, isZh.value), diagnostic: `PURCHASE/PREFLIGHT/${error.code}` }
      : describePurchaseError(error, props.language)
    showErrorDetails.value = false
    modalConfig.value = {
      type: v3Pending.value.length ? 'warning' : 'error', title: t.value.purchaseError, message: detail.message,
      txHash: error?.transactionHash || '', diagnostic: detail.diagnostic,
      details: purchaseErrorDetails(error, {
        projectId: props.project?.id,
        chainId: props.project?.chainId,
        amount: purchaseAmount.value,
        uiAddress: wallet.address,
        signerAddress: error?.purchaseEstimateContext?.from,
        walletCallComparison: error?.purchaseWalletCallComparison,
        rpcPathComparison: error?.purchaseRpcPathComparison,
      }),
    }
  } finally {
    isPurchasing.value = false
    activePurchaseFeedbackKey.value = ''
    if (!disposed && purchaseKey === v3ReadKey.value) showModal.value = true
  }
}

async function handleRefund() {
  if (!canRefund.value || isRefunding.value) return
  if (isV3.value) return actV3('refund')
  isRefunding.value = true
  try {
    const result = await contractService.refund(props.project)
    await Promise.all([loadOnChainData(), wallet.refreshBalances?.()])
    modalConfig.value = {
      type: 'success', title: t.value.refundSuccess, message: t.value.refundSuccessMessage, txHash: result.hash,
    }
  } catch (error) {
    const unresolved = isUnresolvedWalletTransactionError(error)
    modalConfig.value = {
      type: unresolved ? 'warning' : 'error', title: unresolved ? t.value.refundPending : t.value.refundError,
      message: friendlyError(error), txHash: error?.transactionHash || '',
    }
  } finally {
    isRefunding.value = false
    showModal.value = true
  }
}

async function actV3(action) {
  const pending = action === 'claim' ? claimPending.value : v3Pending.value
  if (!isV3.value || v3Busy.value || pending.length || !wallet.isConnected || !wallet.isLoggedIn || !wallet.address) return
  if ((action === 'approveOwner' || action === 'depositOwner') && !v3WalletReady.value) return
  if (action === 'refund' && !canRefund.value) return
  if (action === 'claim' && !canClaimV3.value) return
  if ((action === 'approveOwner' || action === 'depositOwner') && v3State.value?.status !== 0) return
  if (action === 'approveOwner' && (!v3OwnerFunding.value || !v3OwnerValid.value || !v3OwnerNeedsApproval.value)) return
  if (action === 'depositOwner' && (!v3OwnerFunding.value || !v3OwnerValid.value || v3OwnerNeedsApproval.value)) return
  const address = wallet.address
  const projectId = String(props.project?.id ?? '')
  v3Busy.value = true
  if (action === 'refund') isRefunding.value = true
  if (action === 'claim') isClaiming.value = true
  try {
    const context = v3Context.value
    // The service checks identity, simulates and fences the exact request under
    // its wallet lock. Claim/refund do not require purchase proof.
    const result = action === 'refund' ? await contractService.refundV3(context, address)
      : action === 'claim' ? await contractService.claimV3(context, address)
      : action === 'approveOwner' ? await contractService.approveV3ProjectToken(context, v3OwnerAmount.value, address)
      : await contractService.depositV3ProjectTokens(context, v3OwnerAmount.value, address)
    if (address !== wallet.address || projectId !== String(props.project?.id ?? '')) return
    modalConfig.value = { type: 'success',
      title: action === 'refund' ? t.value.refundSuccess : action === 'claim' ? (isZh.value ? '領取成功！' : 'Claim Successful!') : t.value.purchaseSuccess,
      message: action === 'refund' ? t.value.refundSuccessMessage : action === 'claim' ? (isZh.value ? '您的代幣已成功領取。' : 'Your tokens have been successfully claimed.') : t.value.purchaseSuccessMessage,
      txHash: result.hash }
    await refreshV3(true)
  } catch (error) {
    if (address !== wallet.address || projectId !== String(props.project?.id ?? '')) return
    try { v3Pending.value = contractService.v3Pending(v3Context.value, address) } catch { v3Pending.value = [] }
    const actionPending = action === 'claim' ? claimPending.value : v3Pending.value
    if (actionPending.length) v3Failure.value = 'IDO_V3_SUBMISSION_UNRESOLVED'
    const actionError = String(error?.code || '').startsWith('IDO_V3_') ? v3UserMessage(error.code, isZh.value) : friendlyError(error)
    const unresolved = actionPending.length > 0 || isUnresolvedWalletTransactionError(error)
    modalConfig.value = { type: unresolved ? 'warning' : 'error',
      title: action === 'refund' ? (unresolved ? t.value.refundPending : t.value.refundError) : action === 'claim' ? (unresolved ? t.value.claimPending : (isZh.value ? '領取失敗' : 'Claim Failed')) : t.value.purchaseError,
      message: actionPending.length ? v3FailureMessage.value : actionError,
      txHash: error?.transactionHash || '' }
  } finally {
    v3Busy.value = false
    if (action === 'refund') isRefunding.value = false
    if (action === 'claim') isClaiming.value = false
    // A settled receipt for the previous route/account must not open a modal
    // on the currently selected project or wallet.
    if (address === wallet.address && projectId === String(props.project?.id ?? '')) showModal.value = true
  }
}

function resetV3Display() {
  for (const timer of approvalRefreshTimers) clearTimeout(timer)
  approvalRefreshTimers.clear()
  v3Revision++
  v3ReadEpoch++
  v3State.value = null
  v3Qualified.value = false
  v3DisplayedKey.value = ''
  v3ReadError.value = ''
  v3FieldErrors.value = {}
  v3Failure.value = ''
  v3Pending.value = []
  v3OwnerFunding.value = null
  v3Claimable.value = 0n
  paymentBalance.value = { formatted: '0', raw: 0n }
  paymentAllowance.value = 0n
  v3AllowanceConfirmation = null
  v3Loading.value = true
}

// One watcher for all V3 identity/account/session changes; Vue batches changes
// from the same connector event instead of launching several full refreshes.
watch(v3ReadKey, () => {
  if (!isV3.value) return
  dataRevision++
  confirmedRaisedFloor = null
  confirmedUserFloor = null
  confirmedAllowanceFloor = null
  userPurchaseInfo.value = null
  resetV3Display()
  loadOnChainData()
}, { immediate: true })
watch(() => [wallet.address, wallet.chainId, wallet.isConnected, wallet.isLoggedIn], () => {
  if (isV3.value) return
  dataRevision++; confirmedUserFloor = null; confirmedAllowanceFloor = null; userPurchaseInfo.value = null; loadOnChainData()
}, { immediate: true })
watch(() => [props.project?.id, props.project?.contractVersion, props.project?.contractAddress,
  props.project?.chainId, props.project?.onchainLocked, props.project?.paymentTokenAddress, props.project?.tokenAddress], () => {
  dataRevision++
  confirmedRaisedFloor = null
  confirmedUserFloor = null
  confirmedAllowanceFloor = null
  raised.value = Number(props.project?.raised || 0)
  userPurchaseInfo.value = null
  latestMinPolicy.value = null
  purchaseAmount.value = ''
  if (isV3.value) return
  loadOnChainData()
  if (!isV3.value) refreshRaised()
})
watch(() => wallet.sessionRevision, () => { if (isV3.value) return; dataRevision++; confirmedUserFloor = null; confirmedAllowanceFloor = null; loadOnChainData() })
watch(() => props.project?.raised, (value) => {
  if (isV3.value) return
  const nextRaised = Number(value)
  if (Number.isFinite(nextRaised)) raised.value = Math.max(raised.value, nextRaised)
})
function scheduleFundingRefresh() {
  fundingRefreshTimer = window.setTimeout(() => {
    if (isV3.value) {
      if (document.visibilityState === 'visible' && !v3Loading.value && !v3Busy.value && !isApproving.value && !isPurchasing.value) refreshV3()
    } else if (isLive.value && !isPurchasing.value) refreshRaised()
    if (!disposed) scheduleFundingRefresh()
  }, isV3.value ? 120000 : 5000)
}
onMounted(() => {
  stopWalletResume = watchWalletResume(() => {
    if (isV3.value && wallet.address) return refreshV3(true)
  })
  timer = window.setInterval(() => { now.value = Date.now() }, 1000)
  if (!isV3.value) refreshRaised()
  scheduleFundingRefresh()
})
onBeforeUnmount(() => {
  stopWalletResume?.()
  disposed = true
  for (const timer of approvalRefreshTimers) clearTimeout(timer)
  approvalRefreshTimers.clear()
  v3Revision++
  if (timer) window.clearInterval(timer)
  if (fundingRefreshTimer) window.clearTimeout(fundingRefreshTimer)
})
</script>

<template>
  <div id="active-funding-panel" class="active-funding-panel flex flex-col gap-[16px] scroll-mt-[110px]" :class="{ 'is-dark': isDark }">
    <div class="funding-section-title flex items-center gap-[8px]">
      <span class="w-[18px] h-[18px] bg-[#9bff00] rounded-tr-[12px] rounded-bl-[3px] rounded-tl-[3px] rounded-br-[3px] inline-block" style="transform: rotate(15deg);"></span>
      <span class="text-[16px] font-bold">{{ t.fundingTerms }}</span>
    </div>

    <article class="funding-space-card task-card flex flex-col gap-[18px] rounded-[12px] p-[20px]">
      <p v-if="isV3" class="funding-muted text-[11px]">
        {{ !v3State ? (isZh ? '正在讀取參與條件…' : 'Loading participation terms…') : v3State.enabled ? (isZh ? '僅限符合資格的錢包；提交前將核驗資格' : 'Eligible wallets only; eligibility is checked before submission') : (isZh ? '所有錢包均可參與' : 'Open to all wallets') }}
      </p>
      <p v-if="isV3 && v3Loading" class="funding-muted text-[11px]" role="status">{{ t.loading }}</p>
      <div v-if="isV3 && (v3ReadError || Object.keys(v3FieldErrors).length)" class="funding-term-cell rounded p-3 text-[11px]" role="status">
        <p>{{ isZh ? '部分資料暫時無法更新；已顯示的資料可能不是最新。交易將以合約模擬結果為準。' : 'Some data could not be updated. Displayed values may be out of date; transactions are checked by contract simulation.' }}</p>
        <button class="active-secondary-button mt-2" :disabled="v3Loading" @click="refreshV3()">{{ isZh ? '重新讀取' : 'Refresh data' }}</button>
      </div>
      <div v-if="isV3 && v3Failure && !(v3LocalPurchaseBusy && !pendingNeedsRecovery)" role="status" class="rounded border border-amber-300 bg-amber-50 p-3 text-[11px] text-amber-900">
        {{ v3FailureMessage }}
        <details class="mt-1"><summary>{{ isZh ? '供客服排查的錯誤代碼' : 'Error code for support' }}</summary>{{ v3Failure }}</details>
      </div>
      <p v-if="isV3 && v3ApprovalSubmitted && !v3ClaimStage" class="funding-muted text-[11px]" role="status">
        {{ isZh ? '授權已提交，正在更新授權餘額。' : 'Approval submitted. Updating allowance.' }}
        <button class="underline" @click="refreshV3(true)">{{ isZh ? '刷新' : 'Refresh' }}</button>
      </p>
      <details v-if="isV3 && recoveryPending.length" class="break-all text-[11px] text-amber-900"><summary>{{ isZh ? '查看待確認的交易編號' : 'View pending transaction IDs' }}</summary>{{ recoveryPending.map((item) => item.hash || 'unknown').join(' · ') }}</details>
      <p v-if="isV3 && v3ClaimStage && v3Pending.length > claimPending.length" class="funding-muted text-[11px]" role="status">
        {{ isZh ? '此裝置保留有其他操作的待核對紀錄，不影響本次領取資格；領取以合約模擬結果為準。' : 'This device retains other operations for reconciliation. They do not block claiming; eligibility is checked by contract simulation.' }}
      </p>
      <div class="funding-live-row">
        <span class="funding-live-badge" :class="{ 'is-inactive': !statusInfo.live }">
          <span
            class="h-[6px] w-[6px] rounded-full"
            :class="statusInfo.live ? 'bg-[#1aa64a] animate-pulse' : 'bg-[#9ca3af]'"
          ></span>
          {{ statusInfo.label }}
        </span>
      </div>

      <div v-if="isPending || isUpcoming" class="funding-term-cell flex flex-col gap-[8px] rounded-[8px] p-[12px]" role="status">
        <p class="funding-label text-[9px]">{{ startCountdown ? t.startsIn : t.comingSoon }}</p>
        <div v-if="startCountdown" class="funding-value flex items-center gap-[8px] font-bold tabular-nums" aria-live="off">
          <span v-for="[unit, label] in [['d', t.day], ['h', t.hour], ['m', t.minute], ['s', t.second]]" :key="unit" class="funding-countdown-unit rounded-[8px] px-[8px] py-[6px] text-center text-[13px]">
            {{ String(startCountdown[unit]).padStart(2, '0') }}<small class="funding-muted ml-[2px] text-[9px]">{{ label }}</small>
          </span>
        </div>
        <p class="funding-muted text-[11px] leading-[1.5]">{{ isPending ? t.pendingActivation : t.awaitingSaleStart }}</p>
        <p v-if="!startCountdown && isPending" class="funding-muted text-[11px] leading-[1.5]">{{ t.awaitingStart }}</p>
      </div>

      <div class="funding-progress-block">
        <div class="mb-[8px] flex items-end justify-between gap-[10px]">
          <div>
            <p class="funding-label">{{ t.raised }}</p>
            <p class="funding-raised mt-[3px] text-[24px] font-bold leading-none tabular-nums">
              {{ formatRaisedAmount(raised, project.paymentToken) }} <span class="funding-muted text-[12px]">{{ project.paymentToken }}</span>
            </p>
          </div>
          <span class="text-[13px] font-bold text-[#1aa64a]">{{ fundingProgress.toFixed(1) }}%</span>
        </div>
        <div class="progress-track"><div class="progress-fill" :style="{ width: fundingProgress + '%' }"></div></div>
        <p class="funding-muted mt-[6px] text-right text-[10px]">
          {{ t.progress }} · {{ formatRaisedAmount(raised, project.paymentToken) }} / {{ Number(project.hardCap || 0).toLocaleString() }} {{ project.paymentToken }}
        </p>
      </div>

      <div class="grid grid-cols-2 gap-[8px]">
        <div v-for="item in termItems" :key="item[0]" class="funding-term-cell rounded-[8px] p-[10px]">
          <p class="funding-label text-[8px]">{{ item[0] }}</p>
          <p class="funding-value mt-[4px] text-[11px] font-bold leading-[1.35]">{{ item[1] }}</p>
        </div>
      </div>

      <div
        v-if="isTerminal"
        class="funding-term-cell flex items-center justify-between rounded-[8px] p-[12px]"
      >
        <p class="funding-label text-[8px]">{{ t.result }}</p>
        <p
          class="text-[13px] font-bold"
          :class="resultInfo.className"
        >
          {{ resultInfo.label }}
        </p>
      </div>

      <div v-if="showRefund" class="funding-action-section flex flex-col gap-[10px] border-t pt-[12px]">
        <button
          class="active-primary-button"
          :disabled="!canRefund || isRefunding || v3Busy"
          @click="handleRefund"
        >
          {{ isRefunding ? t.refunding : t.refund + ' · ' + userInvested.toLocaleString() + ' ' + project.paymentToken }}
        </button>
      </div>
      <div v-if="isV3 && v3Actions.claim" class="funding-action-section border-t pt-3">
        <button class="active-primary-button" :disabled="!canClaimV3" @click="actV3('claim')">
          {{ isClaiming ? (isZh ? '領取中…' : 'Claiming…') : (isZh ? '領取代幣' : 'Claim tokens') + ' · ' + formatUnits(v3Claimable, v3Context.tokenDecimals) }}
        </button>
      </div>
      <div v-if="isV3 && v3OwnerFunding && v3State?.status === 0" class="funding-action-section border-t pt-3">
        <p class="funding-muted text-[11px]">{{ isZh ? '項目幣充值' : 'Project token deposit' }} · {{ formatUnits(v3OwnerFunding.balance, v3Context.tokenDecimals) }}</p>
        <div class="funding-owner-amount mt-2 flex items-center justify-between gap-3 rounded p-3">
          <span class="funding-muted text-[11px]">{{ isZh ? '按硬頂自動計算' : 'Automatically calculated from hard cap' }}</span>
          <strong class="funding-value text-[13px]">{{ v3OwnerAmount }} {{ project.tokenSymbol }}</strong>
        </div>
        <button class="active-primary-button mt-2" :disabled="!v3WalletReady || v3Busy || v3Unresolved || !v3OwnerValid" @click="actV3(v3OwnerNeedsApproval ? 'approveOwner' : 'depositOwner')">
          {{ v3OwnerNeedsApproval ? (isZh ? '授權項目幣' : 'Approve project token') : (isZh ? '充值項目幣' : 'Deposit project tokens') }}
        </button>
        <p v-if="v3OwnerNeedsApproval" class="funding-muted text-[11px]">
          {{ isZh ? '錢包請求的精確授權量' : 'Exact allowance in wallet request' }}: {{ v3OwnerAmount }} {{ project.tokenSymbol }}.
          {{ isZh ? '若錢包授權數量仍為 0，請取消並重新連接；不要把 0 BNB 當作授權數量。' : 'If the wallet allowance field still shows 0, cancel and reconnect. 0 BNB is not the token allowance.' }}
        </p>
      </div>

      <div
        v-if="settlementPending"
        class="funding-settlement-pending flex flex-col gap-[5px] rounded-[8px] border border-[#ffd591] bg-[#fff7e6] p-[12px]"
      >
        <strong class="text-[11px] text-[#ad4e00]">{{ t.settlementPending }}</strong>
        <p class="text-[10px] leading-[1.5] text-[#874d00]">{{ t.settlementPendingMessage }}</p>
      </div>

      <div v-if="wallet.isConnected && isLive" class="funding-wallet-cell flex flex-col gap-[7px] rounded-[8px] p-[12px]">
        <div v-if="isLoadingChain || (isV3 && v3Loading && !v3DisplayCurrent)" class="funding-muted text-[11px]">{{ t.loading }}</div>
        <template v-else>
          <div class="flex justify-between gap-[12px] text-[11px]"><span class="funding-muted">{{ t.balance }}</span><strong class="funding-value">{{ isV3 && (!v3DisplayCurrent || v3FieldErrors.balance) ? '—' : Number(paymentBalance.formatted || 0).toLocaleString() }} {{ project.paymentToken }}</strong></div>
          <div class="flex justify-between gap-[12px] text-[11px]"><span class="funding-muted">{{ t.invested }}</span><strong class="funding-value">{{ isV3 && (!v3DisplayCurrent || v3FieldErrors.position) ? '—' : userInvested.toLocaleString() }} {{ project.paymentToken }}</strong></div>
          <div class="flex justify-between gap-[12px] text-[11px]"><span class="funding-muted">{{ t.remaining }}</span><strong class="funding-value">{{ isV3 && (!v3DisplayCurrent || v3FieldErrors.position) ? '—' : remainingQuota.toLocaleString() }} {{ project.paymentToken }}</strong></div>
        </template>
      </div>

      <div v-if="isLive || v3LocalPurchaseBusy" class="funding-purchase-zone flex flex-col gap-[10px] pt-[16px]">
        <div class="relative">
          <input v-model="purchaseAmount" type="number" min="0" :max="remainingQuota" :placeholder="t.enterAmount" :disabled="!wallet.isConnected || paymentActionBusy"
                 class="funding-input h-[48px] w-full rounded-[9px] px-[14px] pr-[64px] text-[13px] font-bold outline-none transition-colors placeholder:font-normal disabled:cursor-not-allowed disabled:opacity-55" />
          <span class="funding-muted absolute right-[14px] top-1/2 -translate-y-1/2 text-[11px] font-bold">{{ project.paymentToken }}</span>
        </div>
        <div v-if="tokensToReceive > 0" class="funding-estimate flex items-center justify-between rounded-[8px] px-[12px] py-[10px]">
          <span class="text-[10px] font-bold uppercase tracking-[1px]">{{ t.estimated }}</span>
          <strong class="text-[15px]">{{ tokensToReceive.toLocaleString() }} {{ project.tokenSymbol }}</strong>
        </div>

        <button v-if="v3LocalPurchaseBusy" class="active-primary-button" disabled aria-busy="true" data-testid="v3-purchase-waiting">
          <span class="funding-purchase-spinner" aria-hidden="true"></span>{{ t.purchasing }}
        </button>
        <button v-else-if="!wallet.isConnected" class="active-primary-button" @click="connectWallet">{{ t.connect }}</button>
        <button v-else-if="isV3 && (!wallet.isLoggedIn || (v3Unresolved && !v3ApprovalReadyForPurchase))" class="active-secondary-button" disabled>{{ v3ActionHint }}</button>
        <button v-else-if="!isV3 && Number(purchaseAmount) > Number(paymentBalance.formatted || 0)" class="active-secondary-button" disabled>{{ t.insufficientBalance }}</button>
        <button v-else-if="!isV3 && Number(purchaseAmount) > remainingQuota" class="active-secondary-button" disabled>{{ t.exceedsQuota }}</button>
        <button v-else-if="belowMinPurchase" class="active-secondary-button" disabled>{{ t.belowMinPurchase }}</button>
        <template v-else>
          <div class="funding-payment-steps" :aria-label="isZh ? '授權與認購' : 'Approval and purchase'">
            <span :class="{ 'is-current': needsApproval, 'is-complete': !needsApproval && Number(purchaseAmount) > 0 }">1 · {{ isZh ? '授權' : 'Approve' }}{{ !needsApproval && Number(purchaseAmount) > 0 ? ' ✓' : '' }}</span>
            <span :class="{ 'is-current': !needsApproval }">2 · {{ isZh ? '認購' : 'Purchase' }}</span>
          </div>
          <p class="funding-muted text-[11px]">{{ isZh ? '已授權額度' : 'Current allowance' }}: {{ formatUnits(paymentAllowance, project.paymentTokenDecimals) }} {{ project.paymentToken }}</p>
          <p v-if="needsApproval || isApproving" class="funding-approval-summary rounded-[8px] p-3 text-[12px]" role="status">
            <strong>{{ isZh ? '本次精確授權' : 'Exact approval' }}: {{ pendingPaymentApprovalAmount || purchaseAmount }} {{ project.paymentToken }}</strong>
            <span class="funding-muted mt-1 block text-[11px]">{{ isZh ? '請求已填入本次精確授權額度。請在錢包核對代幣、Spender 與授權數量；若數量欄仍為 0，請勿確認，取消後重新連接錢包。0 BNB 僅表示沒有原生幣轉帳，手續費另計。' : 'The request includes this exact allowance. Verify token, spender and token amount in your wallet. If its allowance field still shows 0, do not confirm; cancel and reconnect. 0 BNB only means no native transfer; gas is separate.' }}</span>
          </p>
          <button v-if="isV3 ? v3NeedsApproval : needsApproval" class="active-primary-button" :disabled="!canPurchase || paymentActionBusy" @click="handleApprove">{{ isApproving ? t.approving : '1 · ' + t.approve }} · {{ pendingPaymentApprovalAmount || purchaseAmount }} {{ project.paymentToken }}</button>
          <button v-else class="active-primary-button" :disabled="!canPurchase || paymentActionBusy" @click="handlePurchase">{{ isPurchasing ? t.purchasing : '2 · ' + t.purchase }}</button>
        </template>
      </div>

      <p v-if="isLive" class="funding-risk flex gap-[7px] rounded-[8px] p-[10px] text-[10px] leading-[1.5]">
        <span class="material-symbols-outlined mt-[1px] text-[14px]">warning</span>{{ t.risk }}
      </p>
    </article>

    <Modal :show="showModal" :type="modalConfig.type" :title="modalConfig.title" :message="modalConfig.message" :confirmText="t.ok" :isDark="isDark" @close="showModal = false; showErrorDetails = false">
      <div v-if="modalConfig.diagnostic" class="funding-modal-tx mb-4 rounded-[10px] border p-3 text-center" :class="{ 'is-dark': isDark }">
        <p class="funding-modal-muted mb-1 text-xs">{{ t.diagnostic }}</p>
        <p class="funding-modal-hash break-all font-mono text-sm">{{ modalConfig.diagnostic }}</p>
      </div>
      <div v-if="modalConfig.diagnostic && modalConfig.details?.length" class="funding-modal-tx mb-4 rounded-[10px] border p-3" :class="{ 'is-dark': isDark }">
        <button type="button" class="funding-modal-hash w-full text-left text-sm font-bold underline" :aria-expanded="showErrorDetails" @click="showErrorDetails = !showErrorDetails">
          {{ showErrorDetails ? t.hideDetails : t.showDetails }}
        </button>
        <div v-if="showErrorDetails" class="mt-3 max-h-[25vh] space-y-1 overflow-y-auto text-xs break-all" role="status">
          <p class="funding-modal-muted">{{ t.detailsNotice }}</p>
          <p v-for="(item, index) in modalConfig.details" :key="index" class="funding-modal-hash font-mono">{{ item.label }}: {{ item.value }}</p>
        </div>
      </div>
      <div v-if="modalConfig.txHash" class="text-center">
        <div class="funding-modal-tx mb-4 rounded-[10px] border p-4" :class="{ 'is-dark': isDark }">
          <p class="funding-modal-muted mb-1 text-xs">{{ t.transaction }}</p>
          <p class="funding-modal-hash break-all font-mono text-sm">{{ modalConfig.txHash }}</p>
        </div>
        <a :href="contractService.getTxUrl(modalConfig.txHash, project)" target="_blank" rel="noopener noreferrer" class="text-sm font-bold text-[#1aa64a] underline">{{ t.viewTransaction }}</a>
      </div>
    </Modal>
  </div>
</template>

<style scoped>
.active-funding-panel {
  --funding-card: #ffffff;
  --funding-cell: #fafaf8;
  --funding-cell-hover: #ffffff;
  --funding-border: #ececea;
  --funding-text: #212121;
  --funding-muted: #808080;
  --funding-track: #ececea;
  --funding-input: #ffffff;
  --funding-risk: #fff7e6;
  --funding-risk-border: #ffd591;
  --funding-risk-text: #7a4c00;
  --funding-shadow: 0 10px 30px rgba(25, 32, 20, 0.05);
}
.funding-section-title { color: var(--funding-text); }
.funding-space-card {
  position: relative;
  isolation: isolate;
  overflow: hidden;
  border: 1px solid var(--funding-border);
  background:
    radial-gradient(130% 90% at 100% 0%, rgba(155, 255, 0, 0.16) 0%, transparent 48%),
    var(--funding-card);
  box-shadow: var(--funding-shadow);
}
.funding-space-card::before {
  position: absolute;
  z-index: -1;
  top: 18px;
  right: 18px;
  width: 70px;
  height: 70px;
  border: 1px solid rgba(26, 166, 74, 0.18);
  border-radius: 50%;
  content: '';
  animation: funding-orbit-breathe 3.8s ease-in-out infinite;
}
.funding-space-card::after {
  position: absolute;
  z-index: -1;
  top: 52px;
  right: 52px;
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: #9bff00;
  box-shadow: 20px -25px 0 rgba(26, 166, 74, 0.35), -27px 17px 0 rgba(26, 166, 74, 0.2);
  content: '';
  animation: funding-orbit-spin 7s linear infinite;
}
.funding-live-row { display: flex; align-items: center; justify-content: space-between; min-height: 25px; }
.funding-live-badge { display: inline-flex; align-items: center; gap: 6px; border: 1px solid rgba(26, 166, 74, 0.16); border-radius: 999px; background: rgba(234, 255, 204, 0.78); padding: 5px 10px; color: #1aa64a; font-size: 9px; font-weight: 700; letter-spacing: 1px; text-transform: uppercase; }
.funding-live-badge.is-inactive { border-color: #e5e7eb; background: #f3f4f6; color: #6b7280; }
.funding-label { color: var(--funding-muted); font-weight: 700; letter-spacing: 1.2px; text-transform: uppercase; }
.funding-muted { color: var(--funding-muted); }
.funding-raised,
.funding-value { color: var(--funding-text); }
.funding-term-cell,
.funding-wallet-cell { border: 1px solid var(--funding-border); background: var(--funding-cell); transition: 0.18s ease; }
.funding-term-cell:hover { border-color: rgba(155, 255, 0, 0.72); background: var(--funding-cell-hover); transform: translateY(-1px); }
.funding-countdown-unit { border: 1px solid var(--funding-border); background: var(--funding-card); color: var(--funding-text); }
.funding-purchase-zone { border-top: 1px solid var(--funding-border); }
.funding-payment-steps { display: flex; gap: 12px; color: var(--funding-muted); font-size: 11px; font-weight: 700; }
.funding-payment-steps .is-current { color: var(--funding-text); }
.funding-payment-steps .is-complete { color: #1aa64a; }
.funding-action-section { border-color: var(--funding-border); }
.funding-owner-amount { border: 1px solid var(--funding-border); background: var(--funding-cell); }
.funding-approval-summary { border: 1px solid var(--funding-border); background: var(--funding-cell); color: var(--funding-text); overflow-wrap: anywhere; }
.funding-purchase-spinner { display: inline-block; width: 15px; height: 15px; margin-right: 8px; vertical-align: middle; border: 2px solid currentColor; border-right-color: transparent; border-radius: 50%; animation: funding-purchase-spin .8s linear infinite; }
@keyframes funding-purchase-spin { to { transform: rotate(360deg); } }
.funding-input { border: 1px solid var(--funding-border); background: var(--funding-input); color: var(--funding-text); }
.funding-input::placeholder { color: var(--funding-muted); }
.funding-input:focus { border-color: #9bff00; box-shadow: 0 0 0 3px rgba(155,255,0,.12); }
.funding-estimate { border: 1px solid rgba(26, 166, 74, 0.14); background: rgba(234, 255, 204, 0.72); color: var(--funding-text); }
.funding-risk { border: 1px solid var(--funding-risk-border); background: var(--funding-risk); color: var(--funding-risk-text); }
.funding-risk .material-symbols-outlined { color: #fa8c16; }
.progress-track { height: 8px; overflow: hidden; border-radius: 999px; background: var(--funding-track); }
.progress-fill {
  position: relative;
  height: 100%;
  overflow: hidden;
  border-radius: inherit;
  background: linear-gradient(90deg, #72d900 0%, #9bff00 42%, #c7ff69 58%, #9bff00 100%);
  background-size: 220% 100%;
  box-shadow: 0 0 14px rgba(155, 255, 0, 0.3);
  transition: width 0.7s cubic-bezier(0.22, 1, 0.36, 1);
  animation: funding-progress-flow 2.4s linear infinite;
}
.funding-modal-tx { border-color: #e6e8e2; background: #f6f7f3; }
.funding-modal-muted { color: #808078; }
.funding-modal-hash { color: #212121; }
.funding-modal-tx.is-dark { border-color: rgba(255,255,255,.1); background: #242622; }
.funding-modal-tx.is-dark .funding-modal-muted { color: #a5a79f; }
.funding-modal-tx.is-dark .funding-modal-hash { color: #fafaf7; }
.active-primary-button,
.active-secondary-button { width: 100%; min-height: 46px; border-radius: 999px; padding: 10px 16px; font-size: 13px; font-weight: 700; transition: 0.2s ease; }
.active-primary-button { background: #9bff00; color: #212121; }
.active-primary-button:hover:not(:disabled) { background: #8bed00; transform: translateY(-1px); }
.active-primary-button:disabled { cursor: not-allowed; border: 1px solid #e3b647; background: #fff0b8; color: #795400; box-shadow: inset 0 0 0 1px rgba(255,255,255,.45); }
.active-secondary-button { border: 1px solid var(--funding-border); background: var(--funding-cell); color: var(--funding-text); }
.active-secondary-button:disabled { cursor: not-allowed; border-color: #e69a9a; background: #fff0f0; color: #a62f2f; opacity: 1; }

.active-funding-panel.is-dark {
  --funding-card: #1f1f1f;
  --funding-cell: #262626;
  --funding-cell-hover: #2b2d29;
  --funding-border: rgba(255, 255, 255, 0.09);
  --funding-text: #fafaf7;
  --funding-muted: #a5a79f;
  --funding-track: #33352f;
  --funding-input: #262626;
  --funding-risk: #2a221a;
  --funding-risk-border: rgba(250, 140, 22, 0.38);
  --funding-risk-text: #f2c377;
  --funding-shadow: 0 16px 34px rgba(0, 0, 0, 0.28);
}
.active-funding-panel.is-dark .funding-space-card {
  background:
    radial-gradient(135% 95% at 100% 0%, rgba(155, 255, 0, 0.11) 0%, transparent 50%),
    var(--funding-card) !important;
}
.active-funding-panel.is-dark .funding-live-badge { border-color: rgba(155, 255, 0, .2); background: rgba(155, 255, 0, .1); color: #9bff00; }
.active-funding-panel.is-dark .funding-estimate { border-color: rgba(155,255,0,.18); background: rgba(155,255,0,.1); }
.active-funding-panel.is-dark .active-primary-button:disabled { border-color: #9b7624; background: #3d3218; color: #ffd56a; box-shadow: none; }
.active-funding-panel.is-dark .active-secondary-button:disabled { border-color: #874848; background: #3b2424; color: #ff9d9d; }

@keyframes funding-progress-flow {
  from { background-position: 100% 0; }
  to { background-position: -120% 0; }
}

@keyframes funding-orbit-spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@keyframes funding-orbit-breathe {
  0%, 100% { transform: scale(0.96); opacity: 0.55; box-shadow: 0 0 0 rgba(155, 255, 0, 0); }
  50% { transform: scale(1.05); opacity: 1; box-shadow: 0 0 24px rgba(155, 255, 0, 0.13); }
}

@media (prefers-reduced-motion: reduce) {
  .progress-fill,
  .funding-space-card::before,
  .funding-space-card::after { animation: none; }
}

@media (max-width: 768px) {
  .active-funding-panel { width: 100%; max-width: 100%; }
  .funding-space-card { padding: 18px !important; }
}
</style>
