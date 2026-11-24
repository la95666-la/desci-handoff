<script setup>
import { ref, reactive, computed, watch, inject, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useWalletStore } from '../stores/wallet'
import { ExternalLink, RefreshCw, AlertTriangle, ShieldCheck, ArrowDownToLine, Loader2, Coins, Sparkles, ArrowUpRight, Users, Wallet } from 'lucide-vue-next'
import Modal from '../components/Modal.vue'
import FairLaunchOwnerPanel from '../components/FairLaunchOwnerPanel.vue'
import JcFloatingNavbar from '../components/JcFloatingNavbar.vue'
import HomeFooter from '../components/HomeFooter.vue'
import PixelGrid from '../components/PixelGrid.vue'
import { contractService } from '../services/contract'
import { web3Service } from '../services/web3'
import { CHECKIN_CHAIN, ADDRESSES } from '../config/contracts'
import { getPendingClaimList, getPurchaseRecord, getClaimRecord, getRefundRecord, getMyProject, getPlatformSettings, getV2Portfolio, getIdoV3Portfolio, getFairLaunchPosition, getPointFlowList, getProjectInvestors, getVerifiedV3Wallet, getProjectDetail } from '../services/api'
import { TRANSLATIONS } from '../i18n/dashboard.js'
import { loadDashboardInvestors } from '../utils/dashboardInvestors.js'
import { initialOwnerStats, applyOwnerStats, ownerStatText } from '../utils/dashboardOwnerStats.js'
import { formatRaisedAmount } from '../utils/raisedAmount.js'
import { watchWalletResume } from '../utils/walletResume.js'
import { useIsMobile } from '../composables/useIsMobile'
import { decimalRatioPercent, formatDecimalString, resolveProjectFundingRoute } from '../utils/fairLaunch.js'
import { isUnresolvedWalletTransactionError } from '../services/fairLaunchWallet.js'
import { SHOW_TASKS_ENTRY } from '../config/featureVisibility.js'
import { fairLaunchPositionWalletContext, mergeFairLaunchPositionIdentity } from '../services/fairLaunchApiContract.js'
import { applyVerifiedClaimPosition, claimActionState, claimInProgressFor } from '../utils/claimAvailability.js'
import { v3UserMessage } from '../utils/v3UserMessages.js'
import { dashboardNetworkState } from '../utils/dashboardNetwork.js'
import { dashboardOwnerIdentity, ownerFundingReady, ownerFundingVisible, ownerDepositComplete } from '../utils/dashboardOwnerFunding.js'
import { withDisplayDeadline } from '../services/idoV3Display.js'
import { cappedPositionContext, loadPortfolioSource, positiveRaw, verifiedPortfolioContext, newestPortfolioRecords } from '../utils/dashboardPortfolio.js'
import {
  computeSoftAndHardRequirement,
  allowanceSufficient,
  balanceSufficient,
  formatToken,
  toInputAmount,
} from '../utils/depositMath'

const { isMobile } = useIsMobile()

const props = defineProps({ language: { type: String, default: 'en' } })
const t = computed(() => TRANSLATIONS[props.language] || TRANSLATIONS.en)
const router = useRouter()
const walletStore = useWalletStore()
// 首頁風格頁面：全局亮/暗由 App.vue 提供（同 TaskHall）
const isDark = inject('isHomeDark', ref(false))

// ============ 1440 画布缩放（与 TaskHall 相同的机制） ============
const canvasRef = ref(null)
const scale = ref(1)
const hostHeight = ref('auto')
const naturalHeight = ref(0)
const DESIGN_WIDTH = 1440
const MIN_SCALE = 0.35
const MAX_SCALE = 1.25

const measure = () => {
  if (!canvasRef.value) return
  const prev = canvasRef.value.style.transform
  canvasRef.value.style.transform = 'scale(1)'
  naturalHeight.value = canvasRef.value.scrollHeight
  canvasRef.value.style.transform = prev
  applyScale()
}
const applyScale = () => {
  const w = window.innerWidth
  const raw = w / DESIGN_WIDTH
  scale.value = Math.min(Math.max(raw, MIN_SCALE), MAX_SCALE)
  hostHeight.value = naturalHeight.value * scale.value + 'px'
}
const onResize = () => { measure(); applyScale() }
let stopWalletResume = null
let dashboardDisposed = false

const scrollRevealHandler = () => {
  const els = document.querySelectorAll('[data-reveal]:not(.is-revealed)')
  if (!els.length) return
  const trigger = window.innerHeight * 0.80
  els.forEach((el) => {
    if (el.getBoundingClientRect().top < trigger) el.classList.add('is-revealed')
  })
}
onMounted(() => {
  stopWalletResume = watchWalletResume(async () => {
    if (!walletStore.isLoggedIn || dashboardDisposed) return
    if (activeTab.value === 'projects') await refreshAllProjectsChain()
    else if (!isLoadingData.value) await loadUserData()
  })
  walletStore.syncChainId()
  measure(); applyScale()
  window.addEventListener('resize', onResize)
  window.addEventListener('scroll', scrollRevealHandler, { passive: true })
  window.setTimeout(scrollRevealHandler, 80)
})
onBeforeUnmount(() => {
  dashboardDisposed = true
  stopWalletResume?.()
  window.removeEventListener('resize', onResize)
  window.removeEventListener('scroll', scrollRevealHandler)
})

// ============ 網絡狀態（按當前業務項目的鏈判斷，不使用全局平台鏈） ============
const connectedChainId = computed(() => (walletStore.chainId == null ? null : Number(walletStore.chainId)))
const networkState = computed(() => dashboardNetworkState(
  activeTab.value === 'projects' ? myProjects.value : [...claimableTokens.value, ...refundableProjects.value],
  connectedChainId.value,
  CHECKIN_CHAIN.chainId,
))
const expectedChainId = computed(() => networkState.value.expectedChainId)
const wrongNetwork = computed(() => walletStore.isConnected && networkState.value.wrongNetwork)
function userActionWrongNetwork(project) {
  const configured = Number(project?.chainId)
  const expected = Number.isInteger(configured) && configured > 0 ? configured : Number(CHECKIN_CHAIN.chainId)
  return !walletStore.isConnected || connectedChainId.value == null || connectedChainId.value !== expected
}
function projectWrongNetwork(project) {
  const expected = Number(project?.chainId)
  if (!Number.isInteger(expected) || expected <= 0) return true
  return !walletStore.isConnected || connectedChainId.value == null || connectedChainId.value !== expected
}

const CHAIN_NAMES = { 56: 'BNB Smart Chain', 97: 'BNB Smart Chain Testnet', 31337: 'Hardhat' }
function chainName(id) {
  const n = Number(id)
  if (CHAIN_NAMES[n]) return CHAIN_NAMES[n]
  return n === Number(CHECKIN_CHAIN.chainId) && CHECKIN_CHAIN.name ? CHECKIN_CHAIN.name : 'Chain ID ' + n
}

const activeTab = ref('investments')
const historyTab = ref('purchase')
// Keep wallet submissions serialized, but show “Claiming” only on the selected project.
const activeClaimKey = ref(null)
const activeRefundKey = ref(null)
const refundKey = (item) => `${String(walletStore.address || '').toLowerCase()}:${String(item.projectId)}`
const refundIsPending = (item) => activeRefundKey.value === refundKey(item)
const isRefreshingAll = ref(false)
const showModal = ref(false)
const modalConfig = ref({ type: 'success', title: '', message: '', txHash: '' })

// ============ 後端 API 數據（投资端） ============
const claimableTokens = ref([])
// A confirmed capped-presale claim must not be re-enabled by a lagging index.
const confirmedCappedClaims = ref({})
const claimKey = (item) => `${String(walletStore.address || '').toLowerCase()}:${String(item.projectId)}`
const claimIsPending = (item) => claimInProgressFor(claimKey(item), activeClaimKey.value)
const claimState = (item) => claimActionState(item, confirmedCappedClaims.value[claimKey(item)], Date.now())
const purchaseHistory = ref([])
const claimHistory = ref([])
const refundHistory = ref([])
const refundableProjects = ref([])
const refundStateByProject = ref({})
const pointFlows = ref([])
const pointTotal = ref(0)
const isLoadingData = ref(false)
const portfolioErrors = ref([])
let portfolioRevision = 0
const confirmedCappedRefunds = ref({})

async function loadUserData() {
  if (!walletStore.isLoggedIn) return
  const revision = ++portfolioRevision
  const address = walletStore.address
  const current = () => revision === portfolioRevision && walletStore.isLoggedIn &&
    String(address || '').toLowerCase() === String(walletStore.address || '').toLowerCase()
  const sourceErrors = []
  await walletStore.syncChainId()
  if (!current()) return
  isLoadingData.value = true
  try {
    const [pendingClaim, purchaseRec, claimRec, refundRec, v2Portfolio, v3Portfolio, settings, pointFlowData] = await Promise.all([
      getPendingClaimList(0).catch((err) => { console.warn('pendingClaimList error:', err); return { list: [] } }),
      getPurchaseRecord(0).catch((err) => { console.warn('purchaseRecord error:', err); return { list: [] } }),
      getClaimRecord(0).catch((err) => { console.warn('claimRecord error:', err); return { list: [] } }),
      getRefundRecord(0).catch((err) => { console.warn('refundRecord error:', err); return { list: [] } }),
      loadPortfolioSource(() => getV2Portfolio(), 'V2', sourceErrors),
      loadPortfolioSource(() => getIdoV3Portfolio(address), 'IDO V3', sourceErrors),
      getPlatformSettings().catch((err) => { console.warn('platformSettings error:', err); return {} }),
      getPointFlowList({ page: 1, pageSize: 8 }).catch((err) => { console.warn('pointFlows error:', err); return { items: [], totalPoint: 0 } })
    ])
    if (!current()) return
    portfolioErrors.value = sourceErrors
    // Fair Launch investor runtime is out of the current release scope. Do not
    // probe its disabled endpoint or turn its 503 into a V2/V3 portfolio warning.
    // Keep its separate DTO/write boundaries intact for a future scoped release.
    const fairPortfolio = { positions: [], claims: [], refunds: [] }
    platformSettings.value = settings
    pointFlows.value = pointFlowData.items || []
    pointTotal.value = Number(pointFlowData.totalPoint || pointFlowData.items?.[0]?.remain || 0)
    const fairPositions = (fairPortfolio.positions || []).map((item) => fairLaunchPositionWalletContext(item, walletStore.address))
    const fairClaims = fairPortfolio.claims || []
    const fairRefunds = fairPortfolio.refunds || []
    const cappedPositions = [...(v2Portfolio.positions || []), ...(v3Portfolio.positions || [])]
    const cappedPurchases = [...(v2Portfolio.purchases || []), ...(v3Portfolio.purchases || [])]
    const cappedClaims = [...(v2Portfolio.claims || []), ...(v3Portfolio.claims || [])]
    const cappedRefunds = [...(v2Portfolio.refunds || []), ...(v3Portfolio.refunds || [])]

    const nextClaims = (pendingClaim.list || []).map((item) => ({
      projectId: item.projectID || 0,
      projectName: item.name || '',
      projectNameCn: item.name || '',
      tokenSymbol: item.tokenSymbol || '',
      tokenAddress: item.tokenAddress || '',
      tokenDecimals: item.tokenDecimals || 18,
      claimableAmount: parseFloat(item.claimNum) || 0,
      nextUnlockTime: item.nextClaimTime ? item.nextClaimTime * 1000 : 0,
      nextUnlockAmount: parseFloat(item.nextClaimNum) || 0,
      totalClaimed: 0,
      totalTokens: 0,
      logo: '/brand/joincare-avatar-green-black.png'
    })).concat(cappedPositions.filter((item) => ['v2_stablecoin', 'ido_v3_stablecoin'].includes(item.contractVersion) && (positiveRaw(item.claimableRaw) || item.nextClaimTime)).map((item) => ({
      ...cappedPositionContext(item),
      projectId: item.projectID || 0,
      projectName: item.name || '',
      projectNameCn: item.name || '',
      tokenSymbol: item.projectToken || '',
      tokenAddress: item.projectTokenAddress || '',
      tokenDecimals: item.projectTokenDecimals || 18,
      claimableAmount: parseFloat(item.claimableDisplay) || 0,
      claimableRaw: item.claimableRaw,
      claimedRaw: item.claimedRaw,
      nextUnlockTime: item.nextClaimTime ? item.nextClaimTime * 1000 : 0,
      nextUnlockAmount: parseFloat(item.nextClaimDisplay) || 0,
      contractVersion: item.contractVersion,
      contractAddress: item.contractAddress,
      chainId: item.chainId,
      fundingMode: 'capped_presale',
      tokenAddress: item.projectTokenAddress,
      tokenDecimals: item.projectTokenDecimals,
      paymentToken: item.paymentToken,
      paymentTokenAddress: item.paymentTokenAddress,
      paymentTokenDecimals: item.paymentTokenDecimals,
      logo: '/brand/joincare-avatar-green-black.png'
    }))).concat(fairPositions.filter((item) => item.actions?.claim?.available === true).map((item) => ({
      ...item,
      projectId: item.projectId || item.projectID,
      fundingMode: item.fundingMode, contractVersion: item.contractVersion,
      claimableAmount: item.claimable?.display || '0', totalClaimed: item.claimed?.display || '0',
      nextUnlockAmount: item.claimable?.display || '0', tokenSymbol: item.projectTokenSymbol || item.tokenSymbol || '',
      logo: item.logo?.url || '/brand/joincare-avatar-green-black.png',
    })))
    // Portfolio claims can lag a mined receipt. Verify V2 and V3 claimability
    // directly on-chain before enabling any button; an RPC failure stays closed.
    const claimWallet = address
    // WalletConnect read requests can hang. Keep the dashboard refresh bounded;
    // an unverified position stays disabled until a later successful refresh.
    await Promise.all(nextClaims.filter((item) => ['v2_stablecoin', 'ido_v3_stablecoin'].includes(item.contractVersion)).map(async (item) => {
      let timeout
      try {
        const readClaimState = (async () => {
          const project = await verifiedPortfolioContext(item, getProjectDetail)
          if (item.contractVersion === 'ido_v3_stablecoin') {
            const snapshot = await contractService.readV3Display(project, claimWallet)
            if (!snapshot.claimable || !snapshot.position || Object.keys(snapshot.errors).length) throw Error('Claim display unavailable')
            return [{ raw: snapshot.claimable.raw, formatted: toInputAmount(snapshot.claimable.raw, item.tokenDecimals) }, snapshot.position]
          }
          return Promise.all([
            contractService.calculateClaimable(project, claimWallet),
            contractService.getUserPurchase(project, claimWallet),
          ])
        })()
        const deadline = new Promise((_, reject) => {
          timeout = setTimeout(() => reject(new Error('Claim state read timed out')), 12000)
        })
        const [available, purchase] = await Promise.race([readClaimState, deadline])
        Object.assign(item, applyVerifiedClaimPosition(item, available.raw, purchase.claimedTokensRaw, available.formatted))
        // Chain data is authoritative when another tranche has vested. The
        // local guard is only for the just-confirmed tranche during index lag.
        if (available.raw > 0n && item.nextUnlockTime && item.nextUnlockTime <= Date.now()) {
          delete confirmedCappedClaims.value[claimKey(item)]
        }
      } catch (error) {
        console.warn('[dashboard] capped-presale claim availability could not be verified:', error?.code || 'RPC_ERROR')
        item.claimCheckUnavailable = true
        item.claimableRaw = '0'
        item.claimableAmount = 0
      } finally {
        clearTimeout(timeout)
      }
    }))
    if (!current()) return
    if (String(walletStore.address || '').toLowerCase() === String(claimWallet || '').toLowerCase()) {
      claimableTokens.value = nextClaims
    }

    purchaseHistory.value = (purchaseRec.list || []).map((item) => ({
      id: Date.now() + Math.random(),
      time: item.createdAt ? item.createdAt * 1000 : Date.now(),
      projectId: item.projectID || 0,
      projectName: item.name || '',
      projectNameCn: item.name || '',
      tokenSymbol: item.tokenSymbol || '',
      investedJcc: parseFloat(item.purchaseToken) || 0,
      tokensBought: parseFloat(item.claimNum) || 0,
      status: item.status || 'completed',
      logo: '/brand/joincare-avatar-green-black.png',
      paymentToken: 'JCC'
    })).concat(cappedPurchases.map((item) => ({
      id: item.contractVersion + '-purchase-' + item.id,
      time: item.createdAt ? item.createdAt * 1000 : Date.now(),
      projectId: item.projectID || 0,
      projectName: item.name || '',
      projectNameCn: item.name || '',
      tokenSymbol: item.projectTokenSymbol || '',
      investedJcc: item.amountDisplay || '0',
      tokensBought: item.projectTokenAmountDisplay ?? item.projectTokenDisplay ?? '0',
      paymentToken: item.tokenSymbol || '',
      paymentTokenAddress: item.tokenAddress || '',
      paymentTokenDecimals: item.tokenDecimals || 18,
      tokenAddress: item.projectTokenAddress || '',
      tokenDecimals: item.projectTokenDecimals || 18,
      contractVersion: item.contractVersion,
      contractAddress: item.contractAddress,
      chainId: item.chainId,
      status: item.status || 'completed',
      logo: '/brand/joincare-avatar-green-black.png'
    }))).concat(fairPositions.filter((item) => item.contribution?.state === 'available').map((item) => ({
      ...item,
      id: `fair-position-${item.projectId || item.projectID}`,
      projectId: item.projectId || item.projectID,
      fundingMode: item.fundingMode, contractVersion: item.contractVersion,
      investedJcc: item.contribution.display || '0',
      tokensBought: item.finalAllocation?.state === 'available' ? item.finalAllocation.display : '—',
      paymentToken: item.paymentTokenSymbol || item.paymentToken || '',
      tokenSymbol: item.projectTokenSymbol || item.tokenSymbol || '',
      status: item.chainStatus || 'pending_settlement',
      logo: item.logo?.url || '/brand/joincare-avatar-green-black.png',
    })))

    claimHistory.value = (claimRec.list || []).map((item) => ({
      id: Date.now() + Math.random(),
      time: item.createdAt ? item.createdAt * 1000 : Date.now(),
      projectId: item.projectID || 0,
      projectName: item.name || '',
      projectNameCn: item.name || '',
      tokenSymbol: item.tokenSymbol || '',
      amount: parseFloat(item.claimNum) || 0,
      txHash: item.txHash || '',
      logo: '/brand/joincare-avatar-green-black.png'
    })).concat(cappedClaims.map((item) => ({
      id: item.contractVersion + '-claim-' + item.id,
      time: item.createdAt ? item.createdAt * 1000 : Date.now(),
      projectId: item.projectID || 0,
      projectName: item.name || '',
      projectNameCn: item.name || '',
      tokenSymbol: item.tokenSymbol || '',
      amount: item.amountDisplay || '0',
      txHash: item.txHash || '',
      chainId: item.chainId,
      logo: '/brand/joincare-avatar-green-black.png'
    }))).concat(fairClaims.map((item) => ({
      ...item,
      id: `fair-claim-${item.id || item.txHash}`,
      projectId: item.projectId || item.projectID,
      fundingMode: item.fundingMode, contractVersion: item.contractVersion,
      amount: item.amount?.display || item.amountDisplay || '0',
      tokenSymbol: item.projectTokenSymbol || item.tokenSymbol || '',
      time: item.createdAt ? Number(item.createdAt) * 1000 : Date.now(),
      logo: item.logo?.url || '/brand/joincare-avatar-green-black.png',
    })))

    refundHistory.value = (refundRec.list || []).map((item) => ({
      id: Date.now() + Math.random(),
      time: item.createdAt ? item.createdAt * 1000 : Date.now(),
      projectId: item.projectID || 0,
      projectName: item.name || '',
      projectNameCn: item.name || '',
      tokenSymbol: item.tokenSymbol || '',
      refundAmount: parseFloat(item.claimNum) || 0,
      txHash: item.txHash || '',
      logo: '/brand/joincare-avatar-green-black.png',
      paymentToken: 'JCC'
    })).concat(cappedRefunds.map((item) => ({
      id: item.contractVersion + '-refund-' + item.id,
      time: item.createdAt ? item.createdAt * 1000 : Date.now(),
      projectId: item.projectID || 0,
      projectName: item.name || '',
      projectNameCn: item.name || '',
      tokenSymbol: item.tokenSymbol || '',
      paymentToken: item.tokenSymbol || '',
      refundAmount: item.amountDisplay || '0',
      txHash: item.txHash || '',
      chainId: item.chainId,
      logo: '/brand/joincare-avatar-green-black.png'
    }))).concat(fairRefunds.map((item) => ({
      ...item,
      id: `fair-refund-${item.id || item.txHash}`,
      projectId: item.projectId || item.projectID,
      fundingMode: item.fundingMode, contractVersion: item.contractVersion,
      refundAmount: item.amount?.display || item.amountDisplay || '0',
      paymentToken: item.paymentTokenSymbol || item.tokenSymbol || '',
      time: item.createdAt ? Number(item.createdAt) * 1000 : Date.now(),
      logo: item.logo?.url || '/brand/joincare-avatar-green-black.png',
    })))

    const stateMap = {}
    ;cappedPositions.forEach((item) => {
      if (confirmedCappedRefunds.value[`${String(address).toLowerCase()}:${item.projectID}`]) stateMap[item.projectID] = 'refunded'
      else if (positiveRaw(item.refundableRaw)) stateMap[item.projectID] = 'refundable'
      else if (item.refundedRaw && item.refundedRaw !== '0') stateMap[item.projectID] = 'refunded'
    })
    fairPositions.forEach((item) => {
      const projectId = item.projectId || item.projectID
      if (item.actions?.refund?.available === true) stateMap[projectId] = 'refundable'
      else if (item.refund?.state === 'available' && item.refund.raw === '0') stateMap[projectId] = 'refunded'
    })
    refundStateByProject.value = stateMap

    refundableProjects.value = cappedPositions.filter((item) => ['v2_stablecoin', 'ido_v3_stablecoin'].includes(item.contractVersion) && positiveRaw(item.refundableRaw) &&
      !confirmedCappedRefunds.value[`${String(address).toLowerCase()}:${item.projectID}`]).map((item) => ({
      ...cappedPositionContext(item),
      id: item.projectID,
      projectId: item.projectID,
      projectName: item.name || '',
      projectNameCn: item.name || '',
      invested: item.refundableDisplay || '0',
      paymentToken: item.paymentToken || '',
      contractVersion: item.contractVersion,
      contractAddress: item.contractAddress,
      chainId: item.chainId,
      paymentTokenAddress: item.paymentTokenAddress,
      paymentTokenDecimals: item.paymentTokenDecimals,
      tokenAddress: item.projectTokenAddress,
      tokenDecimals: item.projectTokenDecimals,
      logo: '/brand/joincare-avatar-green-black.png'
    })).concat(fairPositions.filter((item) => item.actions?.refund?.available === true).map((item) => ({
      ...item,
      id: item.projectId || item.projectID,
      projectId: item.projectId || item.projectID,
      fundingMode: item.fundingMode, contractVersion: item.contractVersion,
      invested: item.contribution?.display || '0',
      paymentToken: item.paymentTokenSymbol || item.paymentToken || '',
      tokenAddress: item.projectTokenAddress || item.tokenAddress || '',
      logo: item.logo?.url || '/brand/joincare-avatar-green-black.png',
    })))
  } catch (err) {
    if (current()) portfolioErrors.value = ['Portfolio']
    console.error('Failed to load user data:', err)
  } finally {
    if (current()) {
      newestPortfolioRecords(purchaseHistory.value)
      newestPortfolioRecords(claimHistory.value)
      newestPortfolioRecords(refundHistory.value)
      isLoadingData.value = false
      nextTick(() => { measure(); scrollRevealHandler() })
    }
  }
}

const isRefreshingClaim = ref(false)
const isRefreshingPurchase = ref(false)
const isRefreshingClaimHistory = ref(false)
const isRefreshingRefund = ref(false)

async function refreshClaimable() {
  if (!walletStore.isLoggedIn) return
  isRefreshingClaim.value = true
  try { await loadUserData() } catch (err) { console.warn('refreshClaimable error:', err) }
  finally { isRefreshingClaim.value = false }
}
async function refreshPurchase() {
  if (!walletStore.isLoggedIn) return
  isRefreshingPurchase.value = true
  try { await loadUserData() } catch (err) { console.warn('refreshPurchase error:', err) }
  finally { isRefreshingPurchase.value = false }
}
async function refreshClaimHistory() {
  if (!walletStore.isLoggedIn) return
  isRefreshingClaimHistory.value = true
  try { await loadUserData() } catch (err) { console.warn('refreshClaimHistory error:', err) }
  finally { isRefreshingClaimHistory.value = false }
}
async function refreshRefundHistory() {
  if (!walletStore.isLoggedIn) return
  isRefreshingRefund.value = true
  try { await loadUserData() } catch (err) { console.warn('refreshRefundHistory error:', err) }
  finally { isRefreshingRefund.value = false }
}
async function refreshAll() {
  if (!walletStore.isLoggedIn) return
  isRefreshingAll.value = true
  try {
    await loadUserData()
    if (isCreator.value) await loadMyProjects()
  } catch (err) { console.warn('refreshAll error:', err) }
  finally { isRefreshingAll.value = false }
}

const handleClaim = async (item) => {
  if (activeClaimKey.value !== null || userActionWrongNetwork(item) || claimState(item).disabled) return
  const pendingClaimKey = claimKey(item)
  activeClaimKey.value = pendingClaimKey
  const address = walletStore.address
  try {
    const project = await verifiedPortfolioContext(item, getProjectDetail, { deferFreshIdentity: true })
    if (item.contractVersion === 'ido_v3_stablecoin') await reconcilePortfolioSubmission(project, address, 'claim(uint256)')
    if (address !== walletStore.address) return
    const route = resolveProjectFundingRoute(item)
    if (item.contractVersion === 'v2_stablecoin') {
      // A stale portfolio may still advertise the previous tranche. Use the
      // contract's current claimable value before any wallet-owned send.
      const current = await contractService.calculateClaimable(project, address)
      if (current.raw <= 0n) {
        item.claimableAmount = 0
        item.claimableRaw = '0'
        return
      }
      Object.assign(item, { claimableRaw: current.raw.toString(), claimableAmount: Number(current.formatted) })
    }
    const result = route.kind === 'fair_launch'
      ? await contractService.claimFairLaunch({ ...item, id: item.projectId })
      : route.kind === 'ido_v3'
        ? await contractService.claimV3(project, address)
        : await contractService.claim({ ...item, id: item.projectId })
    if (address !== walletStore.address) return
    if (route.kind === 'fair_launch') {
      modalConfig.value = {
        type: 'info', title: t.value.claimSuccess,
        message: 'Transaction submitted. Claim history and balances update only after canonical event application and reconciliation.',
        txHash: result.hash,
      }
    } else {
      claimHistory.value.unshift({
        id: Date.now(), time: Date.now(),
        projectId: item.projectId, projectName: item.projectName, projectNameCn: item.projectNameCn,
        amount: item.claimableAmount, tokenSymbol: item.tokenSymbol, txHash: result.hash, chainId: item.chainId, logo: item.logo
      })
      if (['v2_stablecoin', 'ido_v3_stablecoin'].includes(item.contractVersion)) {
        // Only a successful receipt reaches this branch. Failed/cancelled
        // submissions never mark a tranche as claimed.
        confirmedCappedClaims.value[pendingClaimKey] = Date.now()
      }
      item.totalClaimed = (item.totalClaimed || 0) + item.claimableAmount
      item.claimableAmount = 0
      if (['v2_stablecoin', 'ido_v3_stablecoin'].includes(item.contractVersion)) item.claimableRaw = '0'
      modalConfig.value = { type: 'success', title: t.value.claimSuccess, message: t.value.claimSuccessMessage, txHash: result.hash, chainId: item.chainId }
    }
    showModal.value = true
    setTimeout(() => { loadUserData().catch((e) => console.warn('refresh after claim failed:', e)) }, 2000)
    setTimeout(() => { loadUserData().catch(() => {}) }, 8000)
  } catch (error) {
    if (address !== walletStore.address) return
    console.error('Claim failed:', error)
    const unresolved = error?.code === 'IDO_V3_SUBMISSION_UNRESOLVED' || isUnresolvedWalletTransactionError(error)
    modalConfig.value = { type: unresolved ? 'warning' : 'error', title: unresolved ? t.value.claimUnavailable : t.value.claimError, message: friendlyError(error, props.language), txHash: error?.transactionHash || '', chainId: item.chainId }
    showModal.value = true
  } finally {
    if (activeClaimKey.value === pendingClaimKey) activeClaimKey.value = null
  }
}

const refundableProjectFor = (item) => {
  const projectId = typeof item === 'object' ? item?.projectId : item
  const version = typeof item === 'object' ? item?.contractVersion : ''
  return refundableProjects.value.find((candidate) => candidate.projectId === projectId &&
    (!version || !candidate.contractVersion || candidate.contractVersion === version)) || null
}
const isRefundableProject = (item) => refundableProjectFor(item) !== null
const refundStateOf = (projectId) => refundStateByProject.value[projectId] || ''

async function reconcilePortfolioSubmission(project, address, action = null) {
  const relevant = entries => action === 'claim(uint256)' ? entries.filter(entry => entry.action === action) : entries
  if (!relevant(contractService.v3Pending(project, address)).length) return
  const settled = await withDisplayDeadline(() => contractService.reconcileV3Pending(project, address, action))
  const pending = relevant(contractService.v3Pending(project, address))
  if ((!action && !settled) || pending.length) throw Object.assign(new Error('IDO_V3_SUBMISSION_UNRESOLVED'), {
    code: 'IDO_V3_SUBMISSION_UNRESOLVED', transactionHash: pending.find((entry) => entry.hash)?.hash || '',
  })
}
// 项目状态 → 徽章色（浅/暗模式均有对应样式）
const statusPillClass = (status) => {
  if (status === 'Active') return 'pill-green'
  if (status === 'Successful') return 'pill-green'
  if (status === 'Failed') return 'pill-red'
  if (status === 'Canceled' || status === 'Cancelled') return 'pill-orange'
  return 'pill-blue'
}
const mStatusPillClass = (status) => statusPillClass(status).replace('pill-', 'm-dash-pill-')

const handleRefund = async (item) => {
  const refundable = refundableProjectFor(item)
  if (!refundable || activeRefundKey.value !== null || userActionWrongNetwork(refundable)) return
  // Purchase-history rows are presentation records and may not contain the
  // canonical refundable amount or complete routing identity. Always submit
  // and render the result from the portfolio's refundable position.
  const target = { ...item, ...refundable }
  const pendingRefundKey = refundKey(target)
  activeRefundKey.value = pendingRefundKey
  const route = resolveProjectFundingRoute(target)
  const address = walletStore.address
  try {
    const project = await verifiedPortfolioContext(target, getProjectDetail, { deferFreshIdentity: true })
    if (target.contractVersion === 'ido_v3_stablecoin') await reconcilePortfolioSubmission(project, address)
    if (address !== walletStore.address) return
    const result = route.kind === 'fair_launch'
      ? await contractService.refundFairLaunch({ ...target, id: target.projectId })
      : route.kind === 'ido_v3'
        ? await contractService.refundV3(project, address)
        : await contractService.refund({ ...target, id: target.projectId })
    if (address !== walletStore.address) return
    if (route.kind === 'fair_launch') {
      modalConfig.value = {
        type: 'info', title: t.value.refundSuccess,
        message: 'Transaction submitted. Refund history and balances update only after canonical event application and reconciliation.',
        txHash: result.hash,
      }
      setTimeout(() => { loadUserData().catch((e) => console.warn('refresh after refund failed:', e)) }, 2000)
      setTimeout(() => { loadUserData().catch(() => {}) }, 8000)
    } else {
      refundHistory.value.unshift({
        id: Date.now(), time: Date.now(),
        projectId: target.projectId, projectName: target.projectName, projectNameCn: target.projectNameCn,
        refundAmount: target.invested, paymentToken: target.paymentToken, txHash: result.hash, chainId: target.chainId, logo: target.logo
      })
      purchaseHistory.value.forEach((p) => { if (p.projectId === target.projectId) p.refunded = true })
      refundStateByProject.value = { ...refundStateByProject.value, [target.projectId]: 'refunded' }
      if (['v2_stablecoin', 'ido_v3_stablecoin'].includes(target.contractVersion)) confirmedCappedRefunds.value[pendingRefundKey] = true
      const index = refundableProjects.value.findIndex((r) => r.projectId === target.projectId)
      if (index !== -1) refundableProjects.value.splice(index, 1)
      modalConfig.value = { type: 'success', title: t.value.refundSuccess, message: t.value.refundSuccessMessage, txHash: result.hash, chainId: target.chainId }
    }
    showModal.value = true
    if (route.kind !== 'fair_launch') {
      setTimeout(() => { loadUserData().catch(() => {}) }, 2000)
    }
  } catch (error) {
    if (address !== walletStore.address) return
    console.error('Refund failed:', error)
    const unresolved = error?.code === 'IDO_V3_SUBMISSION_UNRESOLVED' || isUnresolvedWalletTransactionError(error)
    modalConfig.value = { type: unresolved ? 'warning' : 'error', title: unresolved ? t.value.refundPending : t.value.refundError, message: friendlyError(error, props.language), txHash: error?.transactionHash || '', chainId: target.chainId }
    showModal.value = true
  } finally {
    if (activeRefundKey.value === pendingRefundKey) activeRefundKey.value = null
  }
}

const formatDate = (timestamp) => {
  return new Date(timestamp).toLocaleString(props.language === 'en' ? 'en-US' : 'zh-TW')
}
const getProjectName = (item) => {
  return item.projectName || item.name || ''
}
const fmtNum = (n) => formatDecimalString(n, 6)
const ownerRaisedText = (project) => ownerStatText(project, 'totalRaised', (amount) => formatRaisedAmount(amount, project.paymentToken))
const pointFlowSource = (item) => {
  if (item?.type === 'CHECK_IN') return t.value.dailyCheckInReward
  if (item?.type === 'INVITE') return t.value.inviteReward
  if (item?.type === 'ADJUST') return t.value.pointsAdjustment
  return t.value.taskReward
}

function friendlyError(error, lang) {
  if (String(error?.code || '').startsWith('IDO_V3_')) return v3UserMessage(error.code, lang !== 'en')
  const msg = error.reason || error.message || ''
  if (isUnresolvedWalletTransactionError(error)) {
    return lang === 'en'
      ? 'Transaction status is unresolved. Do not resend; verify the transaction hash or wallet history before trying again.'
      : '交易狀態未決，禁止重送；再次操作前請用交易雜湊或錢包記錄查證。'
  }
  if (msg.includes('user rejected') || msg.includes('ACTION_REJECTED') || error.code === 'ACTION_REJECTED') {
    return lang === 'en' ? 'Transaction was cancelled by user.' : '用戶取消了交易。'
  }
  if (msg.includes('insufficient funds') || msg.includes('INSUFFICIENT_FUNDS')) {
    return lang === 'en' ? 'Insufficient balance to complete this transaction.' : '餘額不足，無法完成交易。'
  }
  if (msg.includes('BUFFER_OVERRUN') || msg.includes('UNCONFIGURED_NAME') || msg.includes('could not decode result data')) {
    return lang === 'en' ? 'Contract not available. Please check your network and try again.' : '合約不可用，請檢查網路後重試。'
  }
  if (msg.includes('network') || msg.includes('NETWORK_ERROR') || msg.includes('could not detect network') || /chain/i.test(msg)) {
    return lang === 'en' ? 'Wrong network or connection issue. Please check your network and try again.' : '網絡錯誤或網絡不匹配，請檢查網路後重試。'
  }
  if (msg.includes('execution reverted') || msg.includes('CALL_EXCEPTION')) {
    const reason = error.reason || ''
    return lang === 'en'
      ? 'Transaction failed: ' + (reason || 'Contract rejected the operation.')
      : '交易失敗：' + (reason || '合約拒絕了此操作。')
  }
  if (msg.includes('gas') || msg.includes('UNPREDICTABLE_GAS_LIMIT')) {
    return lang === 'en' ? 'Transaction may fail. Please check your input and try again.' : '交易可能失敗，請檢查輸入後重試。'
  }
  if (msg.length > 100) {
    return lang === 'en' ? 'Transaction failed. Please try again later.' : '交易失敗，請稍後重試。'
  }
  return msg || (lang === 'en' ? 'Unknown error occurred.' : '發生未知錯誤。')
}

// ============ B端項目方：我的項目 ============
const myProjects = ref([])
const myProjectsError = ref('')
let myProjectsRevision = 0
const projectInvestors = reactive({})
const selectedProjectId = ref(null)
const selectedProject = computed(() => myProjects.value.find((p) => p.projectId === selectedProjectId.value) || null)
// Tab content and API-backed cards are inserted after the initial reveal pass.
// Re-measure the scaled canvas after Vue renders; never wait for user scrolling.
watch(() => [activeTab.value, myProjects.value, selectedProjectId.value, isMobile.value], async () => {
  await nextTick()
  measure()
  scrollRevealHandler()
}, { flush: 'post' })
const INVESTOR_RECORD_STATUSES = new Set(['Active', 'Ended', 'SuccessfulPendingRelease', 'Successful', 'Failed', 'Canceled'])
const showSelectedProjectInvestors = computed(() => INVESTOR_RECORD_STATUSES.has(selectedProject.value?.status))
const projectWorkspaceStyle = computed(() => ({
  gridTemplateColumns: showSelectedProjectInvestors.value
    ? 'minmax(0, 0.9fr) minmax(380px, 1.3fr)'
    : 'minmax(0, 1fr)',
}))
const platformSettings = ref({})
const isCreator = computed(() => walletStore.userType === 'CREATOR' || walletStore.userType === 'ADMIN')

const STATUS_MAP = { Set: 'Set', Setup: 'Setup', Pending: 'Pending', Active: 'Active', SuccessfulPendingRelease: 'SuccessfulPendingRelease', Ended: 'Ended', Successful: 'Successful', Failed: 'Failed', Canceled: 'Canceled', Cancelled: 'Canceled' }
const V1_STATUS_MAP = ['Pending', 'Active', 'Ended', 'Successful', 'Failed', 'Canceled']
const V2_STATUS_MAP = ['Pending', 'Active', 'Successful', 'Failed', 'Canceled']
const ZERO_ADDR = '0x0000000000000000000000000000000000000000'
const isHexAddress = (v) => typeof v === 'string' && /^0x[0-9a-fA-F]{40}$/.test(v)

function newDepositState() {
  return {
    approveStatus: 'idle', approveMsg: '', approveTxHash: '',
    depositStatus: 'idle', depositMsg: '', depositTxHash: '',
    busyApprove: false, busyDeposit: false, statusLoading: false,
  }
}

async function loadMyProjects() {
  if (!walletStore.isLoggedIn) return
  const revision = ++myProjectsRevision
  const address = walletStore.address
  const current = () => revision === myProjectsRevision && walletStore.isLoggedIn && walletStore.address === address
  await walletStore.syncChainId()
  if (!current()) return
  myProjectsError.value = ''
  try {
    const data = await getMyProject()
    if (!current()) return
    const projects = (data.list || []).map((item) => ({
      ...dashboardOwnerIdentity(item),
      projectId: item.projectID || item.projectId || item.id,
      projectName: item.name || '',
      projectNameCn: item.name || '',
      tokenSymbol: item.tokenSymbol || '',
      id: item.projectID || item.projectId || item.id,
      projectOwner: item.projectOwner || item.project_owner || '',
      projectToken: item.tokenAddress || '',
      tokenAddress: item.tokenAddress || '',
      tokenDecimals: Number(item.tokenDecimals || 18),
      paymentToken: item.paymentToken || 'JCC',
      paymentTokenAddress: item.paymentTokenAddress || '',
      paymentTokenDecimals: Number(item.paymentTokenDecimals || 18),
      chainId: Number(item.chainId || 0),
      contractAddress: item.contractAddress || '',
      contractVersion: item.contractVersion || '',
      fundingMode: item.fundingMode || item.funding_mode || '',
      contractIdentityTrusted: item.contractIdentityTrusted ?? item.contract_identity_trusted,
      contractIdentityState: item.contractIdentityState || item.contract_identity_state || '',
      contractRuntimeState: item.contractRuntimeState || item.contract_runtime_state || '',
      stateSource: item.stateSource || item.state_source || '',
      finality: item.finality || item.source?.finality || '',
      reconciliationState: item.reconciliationState || item.source?.reconciliationState || '',
      actions: item.actions || {},
      saleTokenAmount: item.saleTokenAmount || item.sale_token_amount || '',
      saleTokenAmountRaw: item.saleTokenAmountRaw || item.sale_token_amount_raw || '',
      totalTokensDeposited: item.totalTokensDeposited || item.total_tokens_deposited || '',
      totalTokensDepositedRaw: item.totalTokensDepositedRaw || item.total_tokens_deposited_raw || '',
      status: item.status ? (STATUS_MAP[item.status] || item.status) : 'Setup',
      ...initialOwnerStats(item),
      hardCap: String(item.hardCap ?? '0'),
      softCap: String(item.softCap ?? '0'),
      deposited: '0',
      fundsWithdrawn: !!item.fundWithdrawn,
      tokensWithdrawn: !!item.tokenWithdrawn,
      logo: item.logo?.url || '/brand/joincare-avatar-green-black.png',
      // Pending 卡片狀態（BigInt raw + display）
      deposit: newDepositState(),
      balance: { raw: 0n, display: '0' },
      allowance: { raw: 0n, display: '0' },
      deposited: { raw: 0n, display: '0' },
      depositedRaw: 0n,
      v3State: null,
      v3FundingLoaded: false,
      fundingSnapshotLoaded: false,
      v3Pending: [],
      v3Failure: '',
      softCapRequired: { raw: 0n, display: '0', valid: false },
      hardCapRequired: { raw: 0n, display: '0', valid: false },
      shortfall: { raw: 0n, display: '0', zero: true },
      tokenPriceRaw: 0n,
      tokenPriceDisplay: '0',
      zeroToken: false,
    }))
    const loadedProjects = await Promise.all(projects.map(async (project) => {
      if (project.fundingMode !== 'fair_launch' || project.contractVersion !== 'fair_launch_v1') return project
      try {
        const position = await getFairLaunchPosition(project.projectId, walletStore.address)
        return mergeFairLaunchPositionIdentity(project, position)
      } catch (error) {
        console.warn(`Fair Launch owner runtime unavailable for project ${project.projectId}:`, error)
        return { ...project, actions: {}, runtimeIdentity: null }
      }
    }))
    if (!current()) return
    clearProjectInvestors()
    myProjects.value = loadedProjects
    if (myProjects.value.length) {
      selectProject(myProjects.value[0])
    } else {
      selectedProjectId.value = null
    }
    await refreshAllProjectsChain()
  } catch (err) {
    if (!current()) return
    console.warn('Failed to load my projects:', err)
    myProjects.value = []
    selectedProjectId.value = null
    myProjectsError.value = err?.message || t.value.projectLoadFailed
  }
}

/** 選中項目；充值前階段不加載投資記錄，讓項目操作區使用完整寬度。 */
function selectProject(project) {
  if (!project || !project.projectId) return
  selectedProjectId.value = project.projectId
  // Explicitly load even when refresh selects the same first project: Vue's
  // batched watcher may see an unchanged id after the cache was cleared.
  if (INVESTOR_RECORD_STATUSES.has(project.status) && !projectInvestors[project.projectId]?.loading) loadProjectInvestors(project, 1)
}

watch(
  () => [selectedProjectId.value, selectedProject.value?.status],
  () => {
    const project = selectedProject.value
    if (!project || !showSelectedProjectInvestors.value) return
    const state = projectInvestors[project.projectId]
    if (!state?.loaded && !state?.loading) loadProjectInvestors(project, 1)
  },
)

/** V2 ledger / V3 canonical effects share the same owner-only paginated API. */
async function loadProjectInvestors(project, page = 1) {
  const id = project?.projectId
  if (!id) return
  const address = walletStore.address
  await loadDashboardInvestors(projectInvestors, id, page, getProjectInvestors,
    () => walletStore.isLoggedIn && walletStore.address === address)
}

function clearProjectInvestors() {
  for (const id of Object.keys(projectInvestors)) delete projectInvestors[id]
  selectedProjectId.value = null
}

/** 投資記錄總頁數 */
function totalPages(projectId) {
  const s = projectInvestors[projectId]
  if (!s || !s.pageSize) return 1
  return Math.max(1, Math.ceil(s.total / s.pageSize))
}

/** 從鏈上刷新單個項目：狀態 + Pending 卡片的餘額 / 授權 / 充值進度（BigInt 安全計算） */
async function refreshProjectChainData(project, { sessionReady = false } = {}) {
  if (!project) return
  if (resolveProjectFundingRoute(project).kind === 'ido_v3') return refreshV3ProjectChainData(project)
  if (!walletStore.isConnected || !walletStore.address) return
  // chainChanged 會按設計清空舊 ethers 會話。刷新前用當前 connector
  // 靜默重建，避免網絡切換後讀合約時誤報「Wallet not connected」。
  if (!sessionReady && !(await web3Service.ensureSigner())) return
  const dep = project.deposit
  dep.statusLoading = true
  project.fundingSnapshotLoaded = false
  try {
    const chainData = await contractService.getProject(project)
    // V2 mapping getters return an all-zero tuple for an unknown project ID.
    // Keep the authoritative API Set/Setup state until ProjectCreated is on-chain;
    // otherwise status=0 would be mistaken for Pending and expose the deposit flow.
    if (project.contractVersion === 'v2_stablecoin' && chainData.exists === false) return
    const fundingRoute = resolveProjectFundingRoute(project)
    project.status = fundingRoute.kind === 'fair_launch'
      ? (['Pending', 'Active', 'SuccessfulPendingRelease', 'Successful', 'Failed', 'Canceled'][chainData.status] || project.status || 'Pending')
      : project.contractVersion === 'v2_stablecoin'
        ? (V2_STATUS_MAP[chainData.status] || 'Pending')
        : (V1_STATUS_MAP[chainData.status] || project.status || 'Pending')
    project.totalRaised = chainData.totalRaised
    if (chainData.totalTokensSoldRaw != null) project.tokensSold = toInputAmount(chainData.totalTokensSoldRaw, project.tokenDecimals)
    if (fundingRoute.kind === 'fair_launch') {
      project.saleTokenAmount = chainData.saleTokenAmount
      project.saleTokenAmountRaw = String(chainData.saleTokenAmountRaw || project.saleTokenAmountRaw || '')
      project.fundsWithdrawn = !!chainData.projectProceedsWithdrawn
      project.tokensWithdrawn = !!chainData.failedProjectTokensWithdrawn
      return
    }
    project.fundsWithdrawn = chainData.fundsWithdrawn
    project.tokensWithdrawn = chainData.projectTokensWithdrawn
    if (chainData.projectToken && chainData.projectToken !== ZERO_ADDR) {
      project.projectToken = chainData.projectToken
    }

    project.tokenPriceRaw = chainData.tokenPriceRaw || 0n
    project.tokenPriceDisplay = formatToken(chainData.tokenPriceRaw, project.paymentTokenDecimals, { maxDecimals: 8 })

    const req = computeSoftAndHardRequirement({
      softCapRaw: chainData.softCapRaw,
      hardCapRaw: chainData.hardCapRaw,
      tokenPriceRaw: chainData.tokenPriceRaw,
      depositedRaw: chainData.totalTokensDepositedRaw,
      tokenDecimals: project.tokenDecimals,
    })
    project.softCapRequired = {
      raw: req.softCapRequiredRaw,
      display: formatToken(req.softCapRequiredRaw, project.tokenDecimals),
      valid: (chainData.softCapRaw || 0n) > 0n && (chainData.tokenPriceRaw || 0n) > 0n,
    }
    project.hardCapRequired = {
      raw: req.hardCapRequiredRaw,
      display: formatToken(req.hardCapRequiredRaw, project.tokenDecimals),
      valid: (chainData.hardCapRaw || 0n) > 0n && (chainData.tokenPriceRaw || 0n) > 0n,
    }
    project.depositedRaw = chainData.totalTokensDepositedRaw || 0n
    project.deposited = {
      raw: chainData.totalTokensDepositedRaw || 0n,
      display: formatToken(chainData.totalTokensDepositedRaw, project.tokenDecimals),
    }
    project.shortfall = {
      raw: req.shortfallRaw,
      display: formatToken(req.shortfallRaw, project.tokenDecimals),
      zero: req.shortfallRaw <= 0n,
    }

    const tokenAddr = project.projectToken || project.tokenAddress || ''
    project.zeroToken = !tokenAddr || tokenAddr === ZERO_ADDR
    if (walletStore.address && !project.zeroToken) {
      const spender = isHexAddress(project.contractAddress) ? project.contractAddress : ADDRESSES.JoinCare
      const [bal, allow] = await Promise.all([
        contractService.getTokenBalanceOnChain(tokenAddr, walletStore.address, project.tokenDecimals, project.chainId),
        contractService.getTokenAllowanceOnChain(tokenAddr, walletStore.address, spender, project.tokenDecimals, project.chainId),
      ])
      project.balance = { raw: bal.raw, display: formatToken(bal.raw, project.tokenDecimals) }
      project.allowance = { raw: allow.raw, display: formatToken(allow.raw, project.tokenDecimals) }
    } else {
      project.balance = { raw: 0n, display: '0' }
      project.allowance = { raw: 0n, display: '0' }
    }
    project.fundingSnapshotLoaded = true
  } catch (err) {
    console.warn('Failed to load chain data for project ' + project.projectId + ':', err.message)
  } finally {
    dep.statusLoading = false
  }
}

/** V3 使用與詳情頁相同的固定 runtime、鏈上 owner 與未決交易門禁，但投影成原 V2 Dashboard 卡片。 */
async function refreshV3ProjectChainData(project) {
  const dep = project.deposit
  const address = walletStore.address
  const projectId = String(project.projectId)
  dep.statusLoading = true
  project.v3FundingLoaded = false
  project.v3Failure = ''
  try {
    // Active/terminal financial totals are public project-chain display facts.
    // No wallet binding, signer, proof or pending reconciliation can block them.
    if (project.status !== 'Pending') {
      const display = await contractService.readV3Display(project)
      if (!myProjects.value.includes(project)) return
      const state = display.state
      applyOwnerStats(project, state.project)
      project.status = V2_STATUS_MAP[state.status] || project.status
      project.fundsWithdrawn = !!state.project.fundsWithdrawn
      project.tokensWithdrawn = !!state.project.projectTokensWithdrawn
      // Keep display state separate from the transaction-grade owner snapshot.
      // Pending funding still enters its dedicated validation below.
      if (state.status !== 0) return
    }
    await getVerifiedV3Wallet(address)
    project.v3Pending = contractService.v3Pending(project, address)
    let settled = !project.v3Pending.length
    if (!settled) {
      try { settled = await withDisplayDeadline(() => contractService.reconcileV3Pending(project, address)) }
      catch { /* Keep the original fence; read-only funding may still load. */ }
    }
    project.v3Pending = contractService.v3Pending(project, address)
    if (!settled || project.v3Pending.length) {
      project.v3Failure = 'IDO_V3_SUBMISSION_UNRESOLVED'
    }
    const observed = await contractService.readV3State(project)
    if (address !== walletStore.address || projectId !== String(project.projectId)) return
    const funding = observed.status === 0
      ? await contractService.readV3ProjectTokenFunding(project, address)
      : null
    if (funding) await contractService.assertV3StateUnchanged(observed, funding.state, project)
    if (address !== walletStore.address || projectId !== String(project.projectId)) return
    const state = funding?.state || observed
    project.v3State = state
    project.tokenAddress = state.project.projectToken
    project.status = V2_STATUS_MAP[state.status] || project.status || 'Pending'
    applyOwnerStats(project, state.project)
    project.fundsWithdrawn = !!state.project.fundsWithdrawn
    project.tokensWithdrawn = !!state.project.projectTokensWithdrawn
    project.projectToken = state.project.projectToken
    project.tokenPriceRaw = state.project.tokenPrice || 0n
    project.tokenPriceDisplay = formatToken(project.tokenPriceRaw, project.paymentTokenDecimals, { maxDecimals: 8 })
    const req = computeSoftAndHardRequirement({
      softCapRaw: state.project.softCap,
      hardCapRaw: state.project.hardCap,
      tokenPriceRaw: state.project.tokenPrice,
      depositedRaw: state.project.totalTokensDeposited,
      tokenDecimals: project.tokenDecimals,
    })
    project.softCapRequired = {
      raw: req.softCapRequiredRaw,
      display: formatToken(req.softCapRequiredRaw, project.tokenDecimals),
      valid: state.project.softCap > 0n && state.project.tokenPrice > 0n,
    }
    project.hardCapRequired = {
      raw: req.hardCapRequiredRaw,
      display: formatToken(req.hardCapRequiredRaw, project.tokenDecimals),
      valid: state.project.hardCap > 0n && state.project.tokenPrice > 0n,
    }
    project.depositedRaw = state.project.totalTokensDeposited || 0n
    project.deposited = { raw: project.depositedRaw, display: formatToken(project.depositedRaw, project.tokenDecimals) }
    project.shortfall = {
      raw: req.shortfallRaw,
      display: formatToken(req.shortfallRaw, project.tokenDecimals),
      zero: req.shortfallRaw <= 0n,
    }
    project.zeroToken = false
    if (funding) {
      project.balance = { raw: funding.balance, display: formatToken(funding.balance, project.tokenDecimals) }
      project.allowance = { raw: funding.allowance, display: formatToken(funding.allowance, project.tokenDecimals) }
    }
    project.v3FundingLoaded = true
  } catch (error) {
    project.ownerStatsError = true
    if (address !== walletStore.address || projectId !== String(project.projectId)) return
    try { project.v3Pending = contractService.v3Pending(project, address) } catch { project.v3Pending = [] }
    project.v3Failure = project.v3Pending.length
      ? 'IDO_V3_SUBMISSION_UNRESOLVED'
      : String(error?.code || '').startsWith('IDO_V3_') ? error.code : 'IDO_V3_STATE_UNAVAILABLE'
    console.warn('Failed to load V3 chain data for project ' + project.projectId + ':', project.v3Failure)
  } finally {
    dep.statusLoading = false
  }
}

async function refreshAllProjectsChain() {
  if (myProjects.value.length === 0) return
  const v3 = myProjects.value.filter(p => resolveProjectFundingRoute(p).kind === 'ido_v3' && p.onchainLocked)
  const other = myProjects.value.filter(p => resolveProjectFundingRoute(p).kind !== 'ido_v3')
  const publicReads = Promise.all(v3.map(p => refreshV3ProjectChainData(p)))
  if (walletStore.isConnected && walletStore.address && other.length && await web3Service.ensureSigner()) {
    await Promise.all(other.map(p => refreshProjectChainData(p, { sessionReady: true })))
  }
  await publicReads
}

function refreshPendingData(project) {
  return refreshProjectChainData(project)
}

// 錢包連接 / 登入後自動加載
watch(() => walletStore.isLoggedIn, (loggedIn) => {
  if (loggedIn) {
    loadUserData()
    if (isCreator.value) loadMyProjects()
  } else {
    myProjectsRevision++
    clearProjectInvestors()
    portfolioRevision++
    isLoadingData.value = false
    portfolioErrors.value = []
    claimableTokens.value = []
    purchaseHistory.value = []
    claimHistory.value = []
    refundHistory.value = []
    refundableProjects.value = []
    pointFlows.value = []
    pointTotal.value = 0
    myProjects.value = []
  }
}, { immediate: true })

// userType 可能晚於 isLoggedIn 恢復（autoLogin / 異步 CheckUserType），
// 當其變為 CREATOR/ADMIN 時補加載「我的項目」。
watch(() => walletStore.userType, (type) => {
  if (walletStore.isLoggedIn && (type === 'CREATOR' || type === 'ADMIN')) {
    loadMyProjects()
  }
})

// 網絡切換後刷新鏈上數據（餘額 / 授權 / 充值進度與鏈相關）
watch(() => walletStore.chainId, () => {
  if (walletStore.isLoggedIn && isCreator.value && myProjects.value.length) {
    refreshAllProjectsChain()
  }
})

watch(() => walletStore.address, () => {
  myProjectsRevision++
  clearProjectInvestors()
  myProjects.value = []
  portfolioRevision++
  claimableTokens.value = []; purchaseHistory.value = []; claimHistory.value = []
  refundHistory.value = []; refundableProjects.value = []; refundStateByProject.value = {}
  portfolioErrors.value = []; isLoadingData.value = false
  if (walletStore.isLoggedIn) loadUserData()
  if (walletStore.isLoggedIn && isCreator.value) loadMyProjects()
})

// ============ Pending 卡片：Approval / Deposit 雙步驟 ============

/** Owner funding is always the exact live hard-cap shortfall; it is never operator-entered. */
function parseAmount(project) {
  if (!ownerFundingReady(project)) return null
  const raw = project.shortfall?.raw
  return typeof raw === 'bigint' && raw > 0n ? raw : null
}

function canApprove(project) {
  if (project.deposit.busyApprove || project.deposit.busyDeposit || allowanceCovers(project)) return false
  if (projectWrongNetwork(project) || project.zeroToken || project.v3Failure || project.v3Pending?.length) return false
  const amt = parseAmount(project)
  if (!amt || amt <= 0n) return false
  return balanceSufficient(project.balance.raw, amt)
}
function canDeposit(project) {
  if (project.deposit.busyApprove || project.deposit.busyDeposit) return false
  if (projectWrongNetwork(project) || project.zeroToken || project.v3Failure || project.v3Pending?.length) return false
  const amt = parseAmount(project)
  if (!amt || amt <= 0n) return false
  return balanceSufficient(project.balance.raw, amt) && allowanceSufficient(project.allowance.raw, amt)
}
function allowanceCovers(project) {
  const amt = parseAmount(project)
  if (amt == null) return false
  return allowanceSufficient(project.allowance.raw, amt)
}
function amountInvalid(project) {
  return !project.hardCapRequired?.valid || !hasShortfall(project)
}
function amountTooBig(project) {
  const amt = parseAmount(project)
  if (amt == null || amt <= 0n) return false
  return !balanceSufficient(project.balance.raw, amt)
}

/** 第 1 步：精確授權（不做隱藏 max approve，允許精確授權） */
async function handleApproveExact(project) {
  if (!canApprove(project)) return
  const dep = project.deposit
  const amt = parseAmount(project)
  if (!amt || amt <= 0n) {
    dep.approveStatus = 'error'; dep.approveMsg = t.value.invalidAmount
    return
  }
  if (projectWrongNetwork(project)) return
  if (!balanceSufficient(project.balance.raw, amt)) {
    dep.approveStatus = 'error'; dep.approveMsg = t.value.amountExceedsBalance
    return
  }
  dep.busyApprove = true
  dep.approveStatus = 'pending'
  dep.approveMsg = ''
  dep.approveTxHash = ''
  try {
    // Exact approval: the current hard-cap shortfall, never a free-form amount.
    const amount = toInputAmount(amt, project.tokenDecimals)
    const result = resolveProjectFundingRoute(project).kind === 'ido_v3'
      ? await contractService.approveV3ProjectToken(project, amount, walletStore.address)
      : await contractService.approveProjectToken(project, amount)
    dep.approveStatus = 'success'
    dep.approveTxHash = result.hash
    dep.approveMsg = t.value.approveSuccess
    // 授權後刷新 allowance 與餘額
    await refreshProjectChainData(project)
  } catch (error) {
    console.error('Approve failed:', error)
    dep.approveStatus = 'error'
    dep.approveMsg = friendlyError(error, props.language)
    if (resolveProjectFundingRoute(project).kind === 'ido_v3') await refreshV3ProjectChainData(project)
  } finally {
    dep.busyApprove = false
  }
}

/** 第 2 步：充值（要求授權已覆蓋） */
async function handleDepositExact(project) {
  if (!canDeposit(project)) return
  const dep = project.deposit
  const amt = parseAmount(project)
  if (!amt || amt <= 0n) {
    dep.depositStatus = 'error'; dep.depositMsg = t.value.invalidAmount
    return
  }
  if (projectWrongNetwork(project)) return
  if (!allowanceSufficient(project.allowance.raw, amt)) {
    dep.depositStatus = 'error'; dep.depositMsg = t.value.insufficientAllowance
    return
  }
  dep.busyDeposit = true
  dep.depositStatus = 'pending'
  dep.depositMsg = ''
  dep.depositTxHash = ''
  try {
    const amount = toInputAmount(amt, project.tokenDecimals)
    const result = resolveProjectFundingRoute(project).kind === 'ido_v3'
      ? await contractService.depositV3ProjectTokens(project, amount, walletStore.address)
      : await contractService.depositProjectTokens(project, amount)
    dep.depositStatus = 'success'
    dep.depositTxHash = result.hash
    dep.depositMsg = t.value.depositSuccess
    await refreshProjectChainData(project)
  } catch (error) {
    console.error('Deposit failed:', error)
    dep.depositStatus = 'error'
    dep.depositMsg = friendlyError(error, props.language)
    if (resolveProjectFundingRoute(project).kind === 'ido_v3') await refreshV3ProjectChainData(project)
  } finally {
    dep.busyDeposit = false
  }
}

/** 充值進度（%）：deposited / hardCapRequired */
function hasShortfall(project) {
  return (project.shortfall?.raw || 0n) > 0n
}
function depositPercent(project) {
  const hard = project.hardCapRequired.raw || 0n
  const dep = project.depositedRaw || 0n
  if (hard <= 0n) return 0
  const pct = Number((dep * 10000n) / hard) / 100
  return Math.min(Math.max(pct, 0), 100)
}
function raisedPercent(project) {
  try {
    const percent = decimalRatioPercent(project.totalRaised, project.hardCap, 2)
    const scaled = BigInt(String(percent).replace('.', '').padEnd(3, '0'))
    return Number(scaled > 10000n ? 10000n : scaled) / 100
  } catch { return 0 }
}

// ============ B端寫入：提取資金 / 代幣 ============
const isWithdrawingFunds = reactive({})
const isWithdrawingTokens = reactive({})

async function handleWithdrawFunds(project) {
  const projectId = project.projectId
  if (isWithdrawingFunds[projectId] || isWithdrawingTokens[projectId] || projectWrongNetwork(project)) return
  const address = walletStore.address
  const isV3 = resolveProjectFundingRoute(project).kind === 'ido_v3'
  isWithdrawingFunds[projectId] = true
  try {
    if (isV3) {
      const settled = await contractService.reconcileV3Pending(project, address)
      project.v3Pending = contractService.v3Pending(project, address)
      if (!settled || project.v3Pending.length) throw Object.assign(new Error('IDO_V3_SUBMISSION_UNRESOLVED'), { code: 'IDO_V3_SUBMISSION_UNRESOLVED' })
    }
    const result = isV3
      ? await contractService.withdrawV3RaisedFunds(project, address)
      : await contractService.withdrawRaisedFunds(project)
    if (address !== walletStore.address || !myProjects.value.includes(project)) return
    modalConfig.value = { type: 'success', title: t.value.withdrawFundsSuccess, message: t.value.withdrawFundsSuccessMsg, txHash: result.hash, chainId: project.chainId }
    showModal.value = true
    const proj = myProjects.value.find((p) => p.projectId === projectId)
    if (proj) proj.fundsWithdrawn = true
    refreshProjectChainData(project).catch(() => {})
  } catch (error) {
    if (address !== walletStore.address || !myProjects.value.includes(project)) return
    if (isV3) {
      try { project.v3Pending = contractService.v3Pending(project, address) } catch { /* retain service fence */ }
    }
    const unresolved = error?.code === 'IDO_V3_SUBMISSION_UNRESOLVED' || project.v3Pending?.length || isUnresolvedWalletTransactionError(error)
    modalConfig.value = { type: unresolved ? 'warning' : 'error',
      title: unresolved ? (props.language === 'en' ? 'Withdrawal confirmation pending' : '提款狀態待確認') : t.value.withdrawFundsError,
      message: unresolved && isV3 ? v3UserMessage('IDO_V3_SUBMISSION_UNRESOLVED', props.language !== 'en') : friendlyError(error, props.language),
      txHash: error?.transactionHash || project.v3Pending?.find(entry => entry.hash)?.hash || '', chainId: project.chainId }
    showModal.value = true
  } finally {
    isWithdrawingFunds[projectId] = false
  }
}

async function handleWithdrawTokens(project) {
  const projectId = project.projectId
  if (isWithdrawingTokens[projectId] || isWithdrawingFunds[projectId] || projectWrongNetwork(project)) return
  const address = walletStore.address
  const isV3 = resolveProjectFundingRoute(project).kind === 'ido_v3'
  isWithdrawingTokens[projectId] = true
  try {
    if (isV3) {
      // Observe the original fenced request before any new submission.
      // The dedicated service verifies wallet binding; do not duplicate it here.
      const settled = await contractService.reconcileV3Pending(project, address)
      project.v3Pending = contractService.v3Pending(project, address)
      if (!settled || project.v3Pending.length) throw Object.assign(new Error('IDO_V3_SUBMISSION_UNRESOLVED'), { code: 'IDO_V3_SUBMISSION_UNRESOLVED' })
    }
    const result = isV3
      ? await contractService.withdrawV3ProjectTokens(project, address)
      : await contractService.withdrawProjectTokens(project)
    if (address !== walletStore.address) return
    modalConfig.value = { type: 'success', title: t.value.withdrawTokensSuccess, message: t.value.withdrawTokensSuccessMsg, txHash: result.hash, chainId: project.chainId }
    showModal.value = true
    const proj = myProjects.value.find((p) => p.projectId === projectId)
    if (proj) proj.tokensWithdrawn = true
    refreshProjectChainData(project).catch(() => {})
  } catch (error) {
    if (address !== walletStore.address) return
    if (isV3) {
      try { project.v3Pending = contractService.v3Pending(project, address) } catch { /* service remains fail-closed */ }
    }
    const unresolved = error?.code === 'IDO_V3_SUBMISSION_UNRESOLVED' || project.v3Pending?.length || isUnresolvedWalletTransactionError(error)
    modalConfig.value = { type: unresolved ? 'warning' : 'error',
      title: unresolved ? (props.language === 'en' ? 'Withdrawal confirmation pending' : '提幣狀態待確認') : t.value.withdrawTokensError,
      message: unresolved && isV3 ? v3UserMessage('IDO_V3_SUBMISSION_UNRESOLVED', props.language !== 'en') : friendlyError(error, props.language),
      txHash: error?.transactionHash || project.v3Pending?.find((entry) => entry.hash)?.hash || '', chainId: project.chainId }
    showModal.value = true
  } finally {
    isWithdrawingTokens[projectId] = false
  }
}

// ============ 狀態文案 / 徽章 ============
function statusText(project) {
  if (ownerDepositComplete(project)) return t.value.statusDepositComplete
  const map = {
    Set: t.value.statusSet, Setup: t.value.statusSetup, Pending: t.value.statusPending,
    Active: t.value.statusActive, Ended: t.value.statusEnded, Successful: t.value.statusSuccessful,
    Canceled: t.value.statusCanceled, Failed: t.value.statusFailed,
  }
  return map[project.status] || project.status || ''
}
function statusBadgeClass(project) {
  if (ownerDepositComplete(project)) return 'badge-green'
  switch (project.status) {
    case 'Set': return 'badge-bw'
    case 'Setup': return 'badge-purple'
    case 'Pending': return 'badge-amber'
    case 'Active': return 'badge-teal'
    case 'Ended': return 'badge-blue'
    case 'Successful': return 'badge-green'
    default: return 'badge-red'
  }
}

const spenderLabel = (project) => isHexAddress(project.contractAddress) ? project.contractAddress : ADDRESSES.JoinCare
</script>


<template>
  <JcFloatingNavbar :scale="scale" active="" />

  <!-- ========== MOBILE (<= 768px) ========== -->
  <div v-if="isMobile" class="jc-mobile-page m-dash" :class="{ 'home-dark': isDark }">
    <!-- Hero -->
    <header class="m-hero m-dash-hero">
      <div class="m-eyebrow">
        <span class="m-eyebrow-tag"></span>
        <span>{{ t.eyebrow }}</span>
      </div>
      <h1 class="m-dash-hero-title">
        <span class="font-semibold">{{ t.h1L1 }}</span>
        <span class="font-playfair italic font-normal"> {{ t.h1Accent }}</span>
      </h1>
      <p class="m-dash-hero-sub">{{ t.subtitle }}</p>

      <!-- 錢包 / 網絡 / 刷新 -->
      <div class="m-dash-chips">
        <span v-if="walletStore.isConnected" class="m-dash-chip">
          <span class="dash-dot"></span>{{ walletStore.shortAddress }}
        </span>
        <button v-else class="m-btn-primary m-dash-connectbtn" @click="walletStore.connectWallet()">
          {{ t.connectWallet }}
        </button>
        <span class="m-dash-chip" :class="wrongNetwork ? 'chip-bad' : 'chip-ok'">
          <span v-if="connectedChainId != null" class="dash-dot" :class="{ 'is-bad': wrongNetwork }"></span>
          <span v-if="connectedChainId != null">{{ chainName(connectedChainId) }}</span>
          <span v-else>{{ t.networkUnknown }}</span>
        </span>
        <button v-if="walletStore.isLoggedIn" class="m-dash-iconbtn" :disabled="isRefreshingAll" @click="refreshAll" :title="t.refresh">
          <RefreshCw :size="16" :class="{ 'animate-spin': isRefreshingAll }" />
        </button>
      </div>
    </header>

    <!-- 錯誤網絡：禁用所有鏈上操作 -->
    <div v-if="wrongNetwork" class="m-dash-netwarn" role="alert">
      <AlertTriangle :size="18" class="shrink-0" />
      <div>
        <p class="m-dash-netwarn-title">{{ t.wrongNetwork }}</p>
        <p class="m-dash-netwarn-desc">
          {{ t.expectedChain }}: {{ chainName(expectedChainId) }} ({{ expectedChainId }}) ·
          {{ t.connectedChain }}: {{ connectedChainId != null ? chainName(connectedChainId) + ' (' + connectedChainId + ')' : t.networkUnknown }}
        </p>
      </div>
    </div>

    <!-- 未連接 -->
    <section v-if="!walletStore.isConnected" class="m-section">
      <div class="m-card m-dash-empty">
        <span class="m-dash-empty-icon"><Wallet :size="34" /></span>
        <p class="m-dash-empty-text">{{ t.connectPrompt }}</p>
        <button class="m-btn-primary" @click="walletStore.connectWallet()">{{ t.connectWallet }}</button>
      </div>
    </section>

    <template v-else>
      <!-- Tabs -->
      <div class="m-dash-tabs">
        <button type="button" class="m-dash-tab" :class="{ 'is-on': activeTab === 'investments' }" @click="activeTab = 'investments'">
          {{ t.tabInvestments }}
        </button>
        <button v-if="isCreator" type="button" class="m-dash-tab" :class="{ 'is-on': activeTab === 'projects' }" @click="activeTab = 'projects'">
          {{ t.tabProjects }}
        </button>
      </div>

      <!-- ========== 投資端 ========== -->
      <div v-if="activeTab === 'investments'">
        <div v-if="portfolioErrors.length" class="dash-portfolio-notice m-section" role="alert">
          <strong>{{ t.portfolioUnavailable }} · {{ portfolioErrors.join(', ') }}</strong>
          <p>{{ t.portfolioUnavailableHint }}</p>
          <button class="m-btn-secondary" :disabled="isLoadingData" @click="refreshAll">{{ t.refresh }}</button>
        </div>
        <p v-if="isLoadingData" class="dash-portfolio-loading m-section" role="status">{{ t.loading }}</p>

        <!-- Claimable -->
        <section class="m-section">
          <div class="m-section-head">
            <span class="m-eyebrow-tag"></span>
            <h2 class="m-section-title">{{ t.claimableTokens }}</h2>
            <button v-if="walletStore.isLoggedIn" class="m-dash-section-refresh" :disabled="isRefreshingClaim" @click="refreshClaimable">
              <RefreshCw :size="14" :class="{ 'animate-spin': isRefreshingClaim }" />
            </button>
          </div>
          <div v-if="claimableTokens.length" class="m-card-list">
            <article v-for="item in claimableTokens" :key="item.projectId" class="m-card m-dash-row">
              <div class="m-dash-row-head">
                <img :src="item.logo" :alt="getProjectName(item)" class="m-dash-row-avatar" />
                <div class="m-dash-row-title">
                  <h4>{{ getProjectName(item) }}</h4>
                  <p>{{ t.tokenSymbol }}: {{ item.tokenSymbol }}</p>
                </div>
                <span class="m-dash-pill-ok">{{ item.claimCheckUnavailable ? '—' : item.claimableAmount.toLocaleString() }}</span>
              </div>
              <div class="m-dash-row-meta">
                <div>
                  <p class="m-dash-row-label">{{ t.nextUnlock }}</p>
                  <p class="m-dash-row-value">
                    {{ item.nextUnlockTime ? formatDate(item.nextUnlockTime) : '-' }}
                    <small v-if="item.nextUnlockAmount"> · {{ item.nextUnlockAmount.toLocaleString() }} {{ item.tokenSymbol }}</small>
                  </p>
                </div>
              </div>
              <div class="m-dash-row-actions">
                <button
                  class="m-btn-primary m-dash-row-btn"
                  :disabled="claimState(item).disabled || activeClaimKey !== null || userActionWrongNetwork(item)"
                  @click="handleClaim(item)"
                >
                  {{ claimIsPending(item) ? t.claiming : claimState(item).unavailable ? t.claimUnavailable : claimState(item).claimed ? t.claimed : t.claim }}
                </button>
              </div>
            </article>
          </div>
          <div v-else class="m-card m-dash-empty">
            <span class="dash-section-icon is-large"><Coins :size="22" /></span>
            <p class="m-dash-empty-text">{{ isLoadingData ? t.loading : t.noClaimable }}</p>
          </div>
        </section>

        <!-- Task points activity -->
        <section class="m-section">
          <div class="m-card dash-points-card">
            <header class="dash-points-head">
              <span class="dash-section-icon"><Sparkles :size="17" /></span>
              <div>
                <p class="dash-points-kicker">{{ t.pointsBalance }}</p>
                <p class="dash-points-total">{{ pointTotal.toLocaleString() }} <small>XP</small></p>
              </div>
              <button v-if="SHOW_TASKS_ENTRY" class="dash-points-link" type="button" @click="router.push('/tasks')">
                {{ t.exploreTasks }} <ArrowUpRight :size="14" />
              </button>
            </header>
            <div v-if="pointFlows.length" class="dash-point-list">
              <div v-for="item in pointFlows.slice(0, 5)" :key="item.id" class="dash-point-row">
                <span class="dash-point-dot"></span>
                <div class="dash-point-copy">
                  <strong>{{ pointFlowSource(item) }}</strong>
                  <span>{{ formatDate(item.createdAt) }} · {{ t.pointsAfter }} {{ Number(item.remain || 0).toLocaleString() }}</span>
                </div>
                <span class="dash-point-change">+{{ Number(item.changePoint || 0).toLocaleString() }}</span>
              </div>
            </div>
            <p v-else class="dash-points-empty">{{ t.noPointRecords }}</p>
          </div>
        </section>

        <!-- History -->
        <section class="m-section">
          <div class="m-dash-tabs m-dash-his-tabs">
            <button type="button" class="m-dash-tab" :class="{ 'is-on': historyTab === 'purchase' }" @click="historyTab = 'purchase'">{{ t.purchaseHistory }}</button>
            <button type="button" class="m-dash-tab" :class="{ 'is-on': historyTab === 'claim' }" @click="historyTab = 'claim'">{{ t.claimHistory }}</button>
            <button type="button" class="m-dash-tab" :class="{ 'is-on': historyTab === 'refund' }" @click="historyTab = 'refund'">{{ t.refundHistory }}</button>
          </div>

          <!-- Purchase -->
          <div v-if="historyTab === 'purchase'">
            <div v-if="purchaseHistory.length" class="m-card-list">
              <article v-for="item in purchaseHistory" :key="item.id" class="m-card m-dash-row">
                <div class="m-dash-row-head">
                  <img :src="item.logo" :alt="getProjectName(item)" class="m-dash-row-avatar" />
                  <div class="m-dash-row-title">
                    <h4>{{ getProjectName(item) }}</h4>
                    <p>{{ formatDate(item.time) }}</p>
                  </div>
                </div>
                <div class="m-dash-row-cols">
                  <div><p class="m-dash-row-label">{{ t.invested }}</p><p class="m-dash-row-value">{{ fmtNum(item.investedJcc) }} {{ item.paymentToken || 'JCC' }}</p></div>
                  <div><p class="m-dash-row-label">{{ t.tokensBought }}</p><p class="m-dash-row-value">{{ fmtNum(item.tokensBought) }} {{ item.tokenSymbol }}</p></div>
                  <div><p class="m-dash-row-label">{{ t.projectStatus }}</p><p class="m-dash-row-value"><span :class="mStatusPillClass(item.status)">{{ item.status }}</span></p></div>
                  <div><p class="m-dash-row-label">{{ t.refundStatus }}</p><p class="m-dash-row-value">
                    <span v-if="isRefundableProject(item)" class="dash-refund-available">{{ t.refundableLabel }}</span>
                    <span v-else-if="item.refunded || refundStateOf(item.projectId) === 'refunded'" class="dash-refund-completed">{{ t.refunded }}</span>
                    <span v-else>—</span>
                  </p></div>
                </div>
                <div class="m-dash-row-actions">
                  <button
                    v-if="isRefundableProject(item)"
                    class="m-dash-btn-danger m-dash-row-btn"
                    :disabled="activeRefundKey !== null || userActionWrongNetwork(item)"
                    @click="handleRefund(item)"
                  >{{ refundIsPending(item) ? t.refunding : t.refund }}</button>
                  <span v-else-if="item.refunded || refundStateOf(item.projectId) === 'refunded'" class="m-dash-pill-muted">{{ t.refunded }}</span>
                  <span v-else-if="item.status === 'Successful'" class="m-dash-pill-ok">{{ t.completed }}</span>
                  <span v-else class="m-dash-pill-blue">{{ item.status }}</span>
                </div>
              </article>
            </div>
            <div v-else class="m-card m-dash-empty"><p class="m-dash-empty-text">{{ isLoadingData ? t.loading : t.noPurchases }}</p></div>
          </div>

          <!-- Claim -->
          <div v-else-if="historyTab === 'claim'">
            <div v-if="claimHistory.length" class="m-card-list">
              <article v-for="item in claimHistory" :key="item.id" class="m-card m-dash-row">
                <div class="m-dash-row-head">
                  <img :src="item.logo" :alt="getProjectName(item)" class="m-dash-row-avatar" />
                  <div class="m-dash-row-title">
                    <h4>{{ getProjectName(item) }}</h4>
                    <p>{{ formatDate(item.time) }}</p>
                  </div>
                </div>
                <div class="m-dash-row-cols">
                  <div><p class="m-dash-row-label">{{ t.amount }}</p><p class="m-dash-row-value">{{ fmtNum(item.amount) }} {{ item.tokenSymbol }}</p></div>
                  <div v-if="item.txHash"><p class="m-dash-row-label">{{ t.txHash }}</p>
                    <a :href="contractService.getTxUrl(item.txHash, item)" target="_blank" rel="noopener" class="m-dash-txlink">
                      {{ item.txHash.slice(0, 8) }}…
                      <ExternalLink :size="12" style="display:inline-block;vertical-align:-2px" />
                    </a>
                  </div>
                </div>
              </article>
            </div>
            <div v-else class="m-card m-dash-empty"><p class="m-dash-empty-text">{{ isLoadingData ? t.loading : t.noClaims }}</p></div>
          </div>

          <!-- Refund -->
          <div v-else>
            <div v-if="refundHistory.length" class="m-card-list">
              <article v-for="item in refundHistory" :key="item.id" class="m-card m-dash-row">
                <div class="m-dash-row-head">
                  <img :src="item.logo" :alt="getProjectName(item)" class="m-dash-row-avatar" />
                  <div class="m-dash-row-title">
                    <h4>{{ getProjectName(item) }}</h4>
                    <p>{{ formatDate(item.time) }}</p>
                  </div>
                </div>
                <div class="m-dash-row-cols">
                  <div><p class="m-dash-row-label">{{ t.refundAmount }}</p><p class="m-dash-row-value">{{ fmtNum(item.refundAmount) }} {{ item.paymentToken || 'JCC' }}</p></div>
                  <div v-if="item.txHash"><p class="m-dash-row-label">{{ t.txHash }}</p>
                    <a :href="contractService.getTxUrl(item.txHash, item)" target="_blank" rel="noopener" class="m-dash-txlink">
                      {{ item.txHash.slice(0, 8) }}…
                      <ExternalLink :size="12" style="display:inline-block;vertical-align:-2px" />
                    </a>
                  </div>
                </div>
              </article>
            </div>
            <div v-else class="m-card m-dash-empty"><p class="m-dash-empty-text">{{ isLoadingData ? t.loading : t.noRefunds }}</p></div>
          </div>
        </section>

        <!-- Refundable -->
        <section v-if="refundableProjects.length" class="m-section">
          <div class="m-section-head">
            <span class="m-eyebrow-tag" style="background:#ff7875"></span>
            <h2 class="m-section-title" style="color:#cf1322">{{ t.refundableProjects }}</h2>
          </div>
          <div class="m-card-list">
            <article v-for="item in refundableProjects" :key="item.projectId" class="m-card m-dash-row">
              <div class="m-dash-row-head">
                <img :src="item.logo" :alt="getProjectName(item)" class="m-dash-row-avatar" />
                <div class="m-dash-row-title">
                  <h4>{{ getProjectName(item) }}</h4>
                  <p>{{ t.invested }}: {{ fmtNum(item.invested) }} {{ item.paymentToken || 'JCC' }}</p>
                </div>
              </div>
              <div class="m-dash-row-actions">
                <button class="m-dash-btn-danger m-dash-row-btn" :disabled="activeRefundKey !== null || userActionWrongNetwork(item)" @click="handleRefund(item)">
                  {{ refundIsPending(item) ? t.refunding : t.refund }}
                </button>
              </div>
            </article>
          </div>
        </section>
      </div>

      <!-- ========== 項目方端（Mobile） ========== -->
      <div v-else>
        <section class="m-section">
          <div class="m-section-head">
            <span class="m-eyebrow-tag"></span>
            <h2 class="m-section-title">{{ t.myProjects }}</h2>
          </div>
          <div v-if="myProjectsError" class="m-card m-dash-empty">
            <p class="m-dash-empty-text">{{ t.projectLoadFailed }}</p>
            <p class="m-dash-hint is-warn">{{ myProjectsError }}</p>
          </div>
          <div v-else-if="myProjects.length" class="m-card-list">
            <article v-for="project in myProjects" :key="project.projectId" class="m-card m-dash-proj">
              <!-- head -->
              <div class="m-dash-row-head">
                <img :src="project.logo" :alt="getProjectName(project)" class="m-dash-row-avatar" />
                <div class="m-dash-row-title">
                  <h4>{{ getProjectName(project) }}</h4>
                  <p>ID: {{ project.projectId }} · {{ project.tokenSymbol }}</p>
                </div>
                <span class="m-dash-badge" :class="statusBadgeClass(project)">{{ statusText(project) }}</span>
              </div>

              <FairLaunchOwnerPanel
                v-if="resolveProjectFundingRoute(project).kind === 'fair_launch'"
                :project="project"
                :language="props.language"
                @refresh="refreshPendingData(project)"
              />

              <template v-else>
              <!-- Set -->
              <p v-if="project.status === 'Set'" class="m-dash-proj-note note-orange">{{ t.setPrompt }}</p>

              <!-- Setup -->
              <div v-else-if="project.status === 'Setup'">
                <p class="m-dash-proj-note note-purple">{{ t.setupPrompt }}</p>
              </div>

              <!-- Pending: stats + approve/deposit 兩步 -->
              <div v-else-if="project.status === 'Pending'">
                <div v-if="resolveProjectFundingRoute(project).kind === 'ido_v3' && project.v3Failure" class="m-dash-proj-note note-orange" role="status">
                  {{ v3UserMessage(project.v3Failure, props.language !== 'en') }}
                  <details class="mt-1"><summary>{{ props.language === 'en' ? 'Error code for support' : '供客服排查的錯誤代碼' }}</summary>{{ project.v3Failure }}</details>
                </div>
                <details v-if="resolveProjectFundingRoute(project).kind === 'ido_v3' && project.v3Pending?.length" class="m-dash-hint is-warn break-all">
                  <summary>{{ props.language === 'en' ? 'View pending transaction IDs' : '查看待確認的交易編號' }}</summary>
                  {{ project.v3Pending.map((item) => item.hash || 'unknown').join(' · ') }}
                </details>
                <button v-if="project.v3Pending?.length" class="m-btn-secondary m-dash-row-btn" :disabled="project.deposit.statusLoading" @click="refreshPendingData(project)">{{ props.language === 'en' ? 'Recheck original transaction' : '重新核對原交易' }}</button>
                <div v-if="!ownerFundingVisible(project)" class="m-dash-proj-note note-orange" role="status">
                  <p>{{ project.deposit.statusLoading ? t.loading : t.loadFailed }}</p>
                  <button class="m-btn-secondary m-dash-row-btn" :disabled="project.deposit.statusLoading" @click="refreshPendingData(project)">{{ t.refresh }}</button>
                </div>
                <template v-else>
                <div class="m-dash-pending-stats">
                  <div><p class="m-dash-row-label">{{ t.currencyBalance }}</p><p class="m-dash-row-value">{{ project.balance.display }} {{ project.tokenSymbol }}</p></div>
                  <div><p class="m-dash-row-label">{{ t.allowance }}</p><p class="m-dash-row-value">{{ project.allowance.display }} {{ project.tokenSymbol }}</p></div>
                  <div><p class="m-dash-row-label">{{ t.currentDeposited }}</p><p class="m-dash-row-value">{{ project.deposited.display }} {{ project.tokenSymbol }}</p></div>
                  <div><p class="m-dash-row-label">{{ t.softCapRequired }}</p><p class="m-dash-row-value">{{ project.softCapRequired.valid ? project.softCapRequired.display : '-' }} {{ project.tokenSymbol }}</p></div>
                  <div><p class="m-dash-row-label">{{ t.hardCapRequired }}</p><p class="m-dash-row-value">{{ project.hardCapRequired.valid ? project.hardCapRequired.display : '-' }} {{ project.tokenSymbol }}</p></div>
                  <div class="is-shortfall"><p class="m-dash-row-label">{{ t.shortfall }}</p><p class="m-dash-row-value" :class="project.shortfall.zero ? '' : 'text-short'">{{ project.shortfall.display }} {{ project.tokenSymbol }}</p></div>
                </div>

                <div class="m-dash-progress">
                  <div class="m-dash-progress-track"><div class="m-dash-progress-fill" :style="{ width: depositPercent(project) + '%' }"></div></div>
                  <p class="m-dash-progress-label">{{ Math.round(depositPercent(project)) }}% · {{ t.currentDeposited }} / {{ t.hardCapRequired }}</p>
                </div>

                <div class="m-dash-pending-form">
                  <div class="m-dash-auto-amount">
                    <span>{{ t.depositAmount }}</span>
                    <strong>{{ project.shortfall.display }} {{ project.tokenSymbol }}</strong>
                  </div>
                  <p class="m-dash-hint">{{ props.language === 'en' ? 'The exact token allowance is already filled in your wallet request. 0 BNB is the native transfer value, not the token allowance; gas is separate.' : '已按充值數量自動填入代幣授權額度，無需在錢包重新輸入。0 BNB 是原生幣轉帳額，不是代幣授權額；手續費另計。' }}</p>

                  <!-- Step 1: Approve -->
                  <div class="m-dash-step" :class="'step-' + project.deposit.approveStatus">
                    <div class="m-dash-step-head">
                      <span class="m-dash-step-num"><ShieldCheck :size="14" /></span>
                      <span class="m-dash-step-title">{{ t.approveStep }}</span>
                      <span v-if="allowanceCovers(project) && project.deposit.approveStatus !== 'pending'" class="m-dash-pill-ok">{{ t.approveReady }}</span>
                    </div>
                    <button
                      class="m-btn-primary m-dash-row-btn dash-owner-approve"
                      :disabled="!canApprove(project) || project.deposit.busyApprove"
                      @click="handleApproveExact(project)"
                    >
                      <Loader2 v-if="project.deposit.busyApprove" :size="14" class="animate-spin" />
                      {{ project.deposit.busyApprove ? t.approving : allowanceCovers(project) ? t.approveReady : t.approveExact }}<span v-if="!allowanceCovers(project)"> · {{ project.shortfall.display }} {{ project.tokenSymbol }}</span>
                    </button>
                    <p v-if="project.contractVersion === 'ido_v3_stablecoin' && !allowanceCovers(project)" class="m-dash-hint">{{ props.language === 'en' ? 'Check the exact token allowance in your wallet. If it shows 0, cancel and reconnect; 0 BNB is not the allowance.' : '請核對錢包內精確授權量。若數量為 0，請取消並重新連接；0 BNB 不是授權數量。' }}</p>
                    <div v-if="project.deposit.approveTxHash" class="m-dash-step-tx">
                      <a :href="contractService.getTxUrl(project.deposit.approveTxHash, project)" target="_blank" rel="noopener" class="m-dash-txlink">
                        {{ t.viewTx }} · {{ project.deposit.approveTxHash.slice(0, 10) }}…
                        <ExternalLink :size="12" style="display:inline-block;vertical-align:-2px" />
                      </a>
                    </div>
                    <p v-if="project.deposit.approveMsg" class="m-dash-step-msg" :class="{ 'is-err': project.deposit.approveStatus === 'error' }">{{ project.deposit.approveMsg }}</p>
                  </div>

                  <!-- Step 2: Deposit -->
                  <div class="m-dash-step" :class="'step-' + project.deposit.depositStatus">
                    <div class="m-dash-step-head">
                      <span class="m-dash-step-num step2"><ArrowDownToLine :size="14" /></span>
                      <span class="m-dash-step-title">{{ t.depositStep }}</span>
                    </div>
                    <button
                      class="m-btn-primary m-dash-row-btn"
                      :disabled="!canDeposit(project) || project.deposit.busyDeposit"
                      @click="handleDepositExact(project)"
                    >
                      <Loader2 v-if="project.deposit.busyDeposit" :size="14" class="animate-spin" />
                      {{ project.deposit.busyDeposit ? t.depositing : t.deposit }}
                    </button>
                    <div v-if="project.deposit.depositTxHash" class="m-dash-step-tx">
                      <a :href="contractService.getTxUrl(project.deposit.depositTxHash, project)" target="_blank" rel="noopener" class="m-dash-txlink">
                        {{ t.viewTx }} · {{ project.deposit.depositTxHash.slice(0, 10) }}…
                        <ExternalLink :size="12" style="display:inline-block;vertical-align:-2px" />
                      </a>
                    </div>
                    <p v-if="project.deposit.depositMsg" class="m-dash-step-msg" :class="{ 'is-err': project.deposit.depositStatus === 'error' }">{{ project.deposit.depositMsg }}</p>
                  </div>

                  <p v-if="amountInvalid(project) && !projectWrongNetwork(project) && !project.zeroToken" class="m-dash-hint is-err">{{ t.invalidAmount }}</p>
                  <p v-else-if="amountTooBig(project) && !projectWrongNetwork(project) && !project.zeroToken" class="m-dash-hint is-err">{{ t.amountExceedsBalance }}</p>
                  <p v-if="project.zeroToken" class="m-dash-hint is-warn">{{ t.tokenAddressNotSet }}</p>
                </div>
                </template>
              </div>

              <!-- Active -->
              <div v-else-if="project.status === 'Active'">
                <div class="m-dash-progress">
                  <div class="m-dash-progress-track"><div class="m-dash-progress-fill" :style="{ width: raisedPercent(project) + '%' }"></div></div>
                  <p class="m-dash-progress-label">{{ Math.round(raisedPercent(project)) }}% · {{ ownerRaisedText(project) }} / {{ fmtNum(project.hardCap) }} {{ project.paymentToken }}</p>
                </div>
                <p class="m-dash-hint">{{ t.softCap }}: {{ fmtNum(project.softCap) }} {{ project.paymentToken }}</p>
              </div>

              <!-- Successful -->
              <div v-else-if="project.status === 'Successful'">
                <div class="m-dash-pending-stats">
                  <div><p class="m-dash-row-label">{{ t.totalRaisedLabel }}</p><p class="m-dash-row-value">{{ ownerRaisedText(project) }} {{ project.paymentToken }}</p></div>
                  <div><p class="m-dash-row-label">{{ t.tokensSold }}</p><p class="m-dash-row-value">{{ ownerStatText(project, 'tokensSold', fmtNum) }} {{ project.tokenSymbol }}</p></div>
                </div>
                <p v-if="project.ownerStatsError" class="m-dash-hint is-warn">{{ t.ownerStatsUnavailable }} <button type="button" @click="refreshProjectChainData(project)">{{ t.retryInvestors }}</button></p>
                <div class="m-dash-row-actions m-dash-row-actions-col">
                  <button v-if="!project.fundsWithdrawn" class="m-btn-primary m-dash-row-btn" :disabled="isWithdrawingFunds[project.projectId] || isWithdrawingTokens[project.projectId] || projectWrongNetwork(project)" @click="handleWithdrawFunds(project)">
                    {{ isWithdrawingFunds[project.projectId] ? t.withdrawingBtn : t.withdrawFundsBtn }}
                  </button>
                  <span v-else class="m-dash-pill-ok">{{ t.fundsWithdrawnLabel }}</span>
                  <button v-if="!project.tokensWithdrawn" class="m-btn-secondary m-dash-row-btn" :disabled="isWithdrawingTokens[project.projectId] || isWithdrawingFunds[project.projectId] || projectWrongNetwork(project)" @click="handleWithdrawTokens(project)">
                    {{ isWithdrawingTokens[project.projectId] ? t.withdrawingBtn : t.withdrawTokensBtn }}
                  </button>
                  <span v-else class="m-dash-pill-ok">{{ t.tokensWithdrawnLabel }}</span>
                </div>
              </div>

              <!-- Failed / Canceled -->
              <div v-else-if="project.status === 'Failed' || project.status === 'Canceled'">
                <p class="m-dash-proj-note note-red">{{ t.failedMessage }}</p>
                <button v-if="!project.tokensWithdrawn" class="m-dash-btn-danger m-dash-row-btn" :disabled="isWithdrawingTokens[project.projectId] || projectWrongNetwork(project)" @click="handleWithdrawTokens(project)">
                  {{ isWithdrawingTokens[project.projectId] ? t.withdrawingBtn : t.withdrawAllTokens }}
                </button>
                <span v-else class="m-dash-pill-ok">{{ t.tokensWithdrawnLabel }}</span>
              </div>
              </template>
            </article>
          </div>
          <div v-else class="m-card m-dash-empty"><p class="m-dash-empty-text">{{ t.loadFailed }}</p></div>
        </section>
      </div>
    </template>

    <HomeFooter :language="props.language" />
  </div>
  <!-- ========== DESKTOP (> 768px, 1440 畫布縮放) ========== -->
  <div v-else class="jc-canvas-host" :class="{ 'home-dark': isDark }" :style="{ height: hostHeight }">
    <div class="jc-scaled-wrapper" :style="{ width: (1440 * scale) + 'px', height: hostHeight }">
      <main ref="canvasRef" class="jc-canvas-1440" :style="{ transform: 'scale(' + scale + ')' }">

        <div class="h-[64px] w-full shrink-0"></div>

        <!-- ============ HERO ============ -->
        <section data-reveal class="dash-hero relative w-full px-[80px] pt-[44px] pb-[52px] flex flex-col gap-[26px]">
          <div class="absolute pointer-events-none z-10" style="top: 52px; right: 44px;">
            <PixelGrid :cols="11" :rows="11" cell-size="14px" gap="0" color="#9bff00" :mouse-disturb-radius="0" :step-ms="420" />
          </div>

          <div class="jc-hero-anim jc-hero-anim-1 flex items-center gap-[10px]">
            <span class="dash-eyebrow-tag"></span>
            <span class="text-[20px] font-bold text-[#212121] uppercase tracking-[1px]">{{ t.eyebrow }}</span>
          </div>

          <div class="font-bold text-[#212121] tracking-[-2.4px]" style="font-size: 72px; line-height: 0.98;">
            <p class="jc-hero-headline-line jc-hero-headline-line-1">
              <span class="font-semibold">{{ t.h1L1 }}</span>
            </p>
            <p class="jc-hero-headline-line jc-hero-headline-line-2 font-semibold">
              <span class="font-playfair italic font-normal">{{ t.h1Accent }}</span>
            </p>
          </div>

          <div class="jc-hero-anim jc-hero-anim-3 flex items-center justify-between gap-[32px]">
            <p class="text-[18px] font-bold text-[#212121] leading-[1.4]" style="width: 460px;">{{ t.subtitle }}</p>

            <div class="flex items-center gap-[10px]">
              <!-- 錢包 -->
              <button v-if="!walletStore.isConnected" @click="walletStore.connectWallet()" class="white-pill-btn">
                <span class="text-[14px] font-bold text-[#212121]">{{ t.connectWallet }}</span>
                <span class="arrow-icon arrow-diag"><span class="arrow-img arrow-a" style="--mask:url('/figma/v2/arrow-up-right.svg')"></span><span class="arrow-img arrow-b" style="--mask:url('/figma/v2/arrow-up-right.svg')"></span></span>
              </button>
              <span v-else class="dash-chip" :class="wrongNetwork ? 'chip-bad' : 'chip-ok'">
                <span class="dash-dot" :class="{ 'is-bad': wrongNetwork }"></span>
                <span class="dash-chip-main">{{ walletStore.shortAddress }}</span>
                <span class="dash-chip-sub">{{ walletStore.isLoggedIn ? '✓' : t.networkUnknown }}</span>
              </span>
              <!-- 網絡 -->
              <span class="dash-chip" :class="wrongNetwork ? 'chip-bad' : 'chip-ok'">
                <span class="dash-dot" :class="{ 'is-bad': wrongNetwork }"></span>
                <span class="dash-chip-main">{{ connectedChainId != null ? chainName(connectedChainId) : t.networkUnknown }}</span>
                <span class="dash-chip-sub">{{ t.expectedChain }}: {{ chainName(expectedChainId) }} ({{ expectedChainId }})</span>
              </span>
              <!-- 刷新 -->
              <button v-if="walletStore.isLoggedIn" class="dash-iconbtn" :disabled="isRefreshingAll" @click="refreshAll" :title="t.refresh">
                <RefreshCw :size="18" :class="{ 'animate-spin': isRefreshingAll }" />
              </button>
            </div>
          </div>
        </section>

        <!-- 錯誤網絡橫幅 -->
        <div v-if="wrongNetwork" class="w-full px-[80px] mb-[28px]" role="alert">
          <div class="flex items-start gap-[14px] rounded-[12px] border border-[#ffccc7] bg-[#fff1f0] px-[24px] py-[16px]">
          <AlertTriangle :size="22" class="shrink-0 text-[#ff4d4f]" />
          <div>
            <p class="text-[15px] font-bold text-[#ff4d4f]">{{ t.wrongNetwork }} — {{ t.connectedChain }}: {{ connectedChainId != null ? chainName(connectedChainId) + ' (' + connectedChainId + ')' : t.networkUnknown }}</p>
            <p class="text-[13px] font-medium text-[#a8071a]">
              {{ t.wrongNetworkDesc }} {{ t.expectedChain }}: {{ chainName(expectedChainId) }} ({{ expectedChainId }})
            </p>
          </div>
          </div>
        </div>

        <!-- 未連接 -->
        <section v-if="!walletStore.isConnected" data-reveal class="w-full px-[80px] mb-[72px]">
          <div class="rise-card rise-card-1 flex flex-col items-center gap-[18px] rounded-[16px] border border-[#ececea] bg-white px-[40px] py-[84px] text-center">
            <span class="dash-section-icon is-large"><Wallet :size="23" /></span>
            <p class="text-[20px] font-bold text-[#212121]">{{ t.connectPrompt }}</p>
            <button @click="walletStore.connectWallet()" class="dash-btn-dark">
              {{ t.connectWallet }}
            </button>
          </div>
        </section>

        <template v-else>
          <!-- Tabs -->
          <nav class="w-full px-[80px] mb-[28px] flex items-center gap-[8px]">
            <button type="button" class="dash-tab" :class="{ 'is-on': activeTab === 'investments' }" @click="activeTab = 'investments'">{{ t.tabInvestments }}</button>
            <button v-if="isCreator" type="button" class="dash-tab" :class="{ 'is-on': activeTab === 'projects' }" @click="activeTab = 'projects'">{{ t.tabProjects }}</button>
          </nav>

          <!-- ============ 投資端 ============ -->
          <div v-if="activeTab === 'investments'" class="w-full">
            <div v-if="portfolioErrors.length" class="px-[80px] mb-[24px]" role="alert">
              <div class="dash-portfolio-notice">
                <strong>{{ t.portfolioUnavailable }} · {{ portfolioErrors.join(', ') }}</strong>
                <p>{{ t.portfolioUnavailableHint }}</p>
                <button class="dash-btn-soft" :disabled="isLoadingData" @click="refreshAll">{{ t.refresh }}</button>
              </div>
            </div>
            <p v-if="isLoadingData" class="dash-portfolio-loading px-[80px] mb-[24px]" role="status">{{ t.loading }}</p>

            <div class="dash-investment-overview w-full px-[80px] mb-[36px]">

            <!-- Claimable -->
            <section data-reveal>
              <article class="rise-card rise-card-1 dash-card">
                <header class="flex items-center justify-between border-b border-[#eee] px-[28px] py-[20px]">
                  <h3 class="flex items-center gap-[10px] text-[20px] font-bold text-[#212121]">
                    <span class="dash-section-icon"><Coins :size="17" /></span>{{ t.claimableTokens }}
                    <span class="dash-count-pill">{{ claimableTokens.length }}</span>
                  </h3>
                  <button v-if="walletStore.isLoggedIn" class="dash-iconbtn" :disabled="isRefreshingClaim" @click="refreshClaimable" :title="t.refresh">
                    <RefreshCw :size="16" :class="{ 'animate-spin': isRefreshingClaim }" />
                  </button>
                </header>

                <div v-if="claimableTokens.length" class="overflow-x-auto">
                  <table class="dash-table w-full min-w-[880px]">
                    <thead>
                      <tr>
                        <th>{{ t.project }}</th>
                        <th>{{ t.tokenSymbol }}</th>
                        <th>{{ t.claimableAmount }}</th>
                        <th>{{ t.nextUnlock }}</th>
                        <th style="text-align:right">{{ t.action }}</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="item in claimableTokens" :key="item.projectId">
                        <td>
                          <div class="flex items-center gap-[12px]">
                            <img :src="item.logo" :alt="getProjectName(item)" class="dash-avatar" />
                            <span class="font-semibold text-[#212121]">{{ getProjectName(item) }}</span>
                          </div>
                        </td>
                        <td class="text-[#212121]">{{ item.tokenSymbol }}</td>
                        <td class="font-bold tabular-nums text-[#212121]">{{ item.claimCheckUnavailable ? '—' : item.claimableAmount.toLocaleString() }}</td>
                        <td>
                          <div class="text-[#4c5148]">{{ item.nextUnlockTime ? formatDate(item.nextUnlockTime) : '-' }}</div>
                          <div v-if="item.nextUnlockAmount" class="text-[11px] text-[#808080]">{{ item.nextUnlockAmount.toLocaleString() }} {{ item.tokenSymbol }}</div>
                        </td>
                        <td style="text-align:right">
                          <button
                            class="dash-btn-lime"
                            :disabled="claimState(item).disabled || activeClaimKey !== null || userActionWrongNetwork(item)"
                            @click="handleClaim(item)"
                          >{{ claimIsPending(item) ? t.claiming : claimState(item).unavailable ? t.claimUnavailable : claimState(item).claimed ? t.claimed : t.claim }}</button>
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
                <div v-else class="dash-empty">
                  <span class="dash-section-icon is-large"><Coins :size="23" /></span>
                  <p class="dash-empty-text">{{ isLoadingData ? t.loading : t.noClaimable }}</p>
                </div>
              </article>
            </section>

            <section data-reveal>
              <article class="rise-card rise-card-2 dash-card dash-points-card">
                <header class="dash-points-head">
                  <span class="dash-section-icon"><Sparkles :size="17" /></span>
                  <div>
                    <p class="dash-points-kicker">{{ t.pointsBalance }}</p>
                    <p class="dash-points-total">{{ pointTotal.toLocaleString() }} <small>XP</small></p>
                  </div>
                  <button v-if="SHOW_TASKS_ENTRY" class="dash-points-link" type="button" @click="router.push('/tasks')">
                    {{ t.exploreTasks }} <ArrowUpRight :size="14" />
                  </button>
                </header>
                <div v-if="pointFlows.length" class="dash-point-list">
                  <div v-for="item in pointFlows.slice(0, 6)" :key="item.id" class="dash-point-row">
                    <span class="dash-point-dot"></span>
                    <div class="dash-point-copy">
                      <strong>{{ pointFlowSource(item) }}</strong>
                      <span>{{ formatDate(item.createdAt) }} · {{ t.pointsAfter }} {{ Number(item.remain || 0).toLocaleString() }}</span>
                    </div>
                    <span class="dash-point-change">+{{ Number(item.changePoint || 0).toLocaleString() }}</span>
                  </div>
                </div>
                <p v-else class="dash-points-empty">{{ t.noPointRecords }}</p>
              </article>
            </section>
            </div>

            <!-- History -->
            <section data-reveal class="w-full px-[80px] mb-[40px]">
              <article class="rise-card rise-card-2 dash-card">
                <header class="flex items-center gap-[4px] border-b border-[#eee] px-[20px]">
                  <button type="button" class="dash-subtab" :class="{ 'is-on': historyTab === 'purchase' }" @click="historyTab = 'purchase'">{{ t.purchaseHistory }}</button>
                  <button type="button" class="dash-subtab" :class="{ 'is-on': historyTab === 'claim' }" @click="historyTab = 'claim'">{{ t.claimHistory }}</button>
                  <button type="button" class="dash-subtab" :class="{ 'is-on': historyTab === 'refund' }" @click="historyTab = 'refund'">{{ t.refundHistory }}</button>
                </header>

                <div v-if="historyTab === 'purchase'">
                  <div v-if="purchaseHistory.length" class="overflow-x-auto">
                    <table class="dash-table w-full min-w-[900px]">
                      <thead><tr><th>{{ t.time }}</th><th>{{ t.project }}</th><th>{{ t.invested }}</th><th>{{ t.tokensBought }}</th><th>{{ t.projectStatus }}</th><th style="text-align:right">{{ t.refundStatus }}</th></tr></thead>
                      <tbody>
                        <tr v-for="item in purchaseHistory" :key="item.id">
                          <td class="text-[#4c5148]">{{ formatDate(item.time) }}</td>
                          <td>
                            <div class="flex items-center gap-[12px]">
                              <img :src="item.logo" :alt="getProjectName(item)" class="dash-avatar-sm" />
                              <span class="font-medium text-[#212121]">{{ getProjectName(item) }}</span>
                            </div>
                          </td>
                          <td class="tabular-nums text-[#212121]">{{ fmtNum(item.investedJcc) }} {{ item.paymentToken || 'JCC' }}</td>
                          <td class="tabular-nums text-[#212121]">{{ fmtNum(item.tokensBought) }} {{ item.tokenSymbol }}</td>
                          <td><span class="dash-pill" :class="statusPillClass(item.status)">{{ item.status }}</span></td>
                          <td style="text-align:right">
                            <button v-if="isRefundableProject(item)" class="dash-btn-danger" :disabled="activeRefundKey !== null || userActionWrongNetwork(item)" @click="handleRefund(item)">
                              {{ refundIsPending(item) ? t.refunding : t.refund }}
                            </button>
                            <span v-else-if="item.refunded || refundStateOf(item.projectId) === 'refunded'" class="dash-pill pill-gray">{{ t.refunded }}</span>
                            <span v-else>—</span>
                          </td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                  <div v-else class="dash-empty"><p class="dash-empty-text">{{ isLoadingData ? t.loading : t.noPurchases }}</p></div>
                </div>

                <div v-else-if="historyTab === 'claim'">
                  <div v-if="claimHistory.length" class="overflow-x-auto">
                    <table class="dash-table w-full min-w-[780px]">
                      <thead><tr><th>{{ t.time }}</th><th>{{ t.project }}</th><th>{{ t.amount }}</th><th>{{ t.txHash }}</th></tr></thead>
                      <tbody>
                        <tr v-for="item in claimHistory" :key="item.id">
                          <td class="text-[#4c5148]">{{ formatDate(item.time) }}</td>
                          <td>
                            <div class="flex items-center gap-[12px]">
                              <img :src="item.logo" :alt="getProjectName(item)" class="dash-avatar-sm" />
                              <span class="font-medium text-[#212121]">{{ getProjectName(item) }}</span>
                            </div>
                          </td>
                          <td class="tabular-nums text-[#212121]">{{ fmtNum(item.amount) }} {{ item.tokenSymbol }}</td>
                          <td>
                            <a v-if="item.txHash" :href="contractService.getTxUrl(item.txHash, item)" target="_blank" rel="noopener" class="dash-txlink">
                              {{ item.txHash.slice(0, 10) }}…{{ item.txHash.slice(-8) }}
                              <ExternalLink :size="14" />
                            </a>
                            <span v-else class="text-[#808080]">—</span>
                          </td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                  <div v-else class="dash-empty"><p class="dash-empty-text">{{ isLoadingData ? t.loading : t.noClaims }}</p></div>
                </div>

                <div v-else>
                  <div v-if="refundHistory.length" class="overflow-x-auto">
                    <table class="dash-table w-full min-w-[780px]">
                      <thead><tr><th>{{ t.time }}</th><th>{{ t.project }}</th><th>{{ t.refundAmount }}</th><th>{{ t.txHash }}</th></tr></thead>
                      <tbody>
                        <tr v-for="item in refundHistory" :key="item.id">
                          <td class="text-[#4c5148]">{{ formatDate(item.time) }}</td>
                          <td>
                            <div class="flex items-center gap-[12px]">
                              <img :src="item.logo" :alt="getProjectName(item)" class="dash-avatar-sm" />
                              <span class="font-medium text-[#212121]">{{ getProjectName(item) }}</span>
                            </div>
                          </td>
                          <td class="tabular-nums text-[#212121]">{{ fmtNum(item.refundAmount) }} {{ item.paymentToken || 'JCC' }}</td>
                          <td>
                            <a v-if="item.txHash" :href="contractService.getTxUrl(item.txHash, item)" target="_blank" rel="noopener" class="dash-txlink">
                              {{ item.txHash.slice(0, 10) }}…{{ item.txHash.slice(-8) }}
                              <ExternalLink :size="14" />
                            </a>
                            <span v-else class="text-[#808080]">—</span>
                          </td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                  <div v-else class="dash-empty"><p class="dash-empty-text">{{ isLoadingData ? t.loading : t.noRefunds }}</p></div>
                </div>
              </article>
            </section>

            <!-- Refundable -->
            <section v-if="refundableProjects.length" data-reveal class="w-full px-[80px] mb-[40px]">
              <article class="rise-card rise-card-3 dash-card dash-card-danger">
                <header class="flex items-center gap-[10px] border-b border-[#ffd8bf] px-[28px] py-[20px]">
                  <h3 class="text-[20px] font-bold text-[#cf1322]">{{ t.refundableProjects }}</h3>
                  <span class="dash-count-pill">{{ refundableProjects.length }}</span>
                </header>
                <div class="overflow-x-auto">
                  <table class="dash-table w-full min-w-[680px]">
                    <thead><tr><th>{{ t.project }}</th><th>{{ t.invested }}</th><th style="text-align:right">{{ t.action }}</th></tr></thead>
                    <tbody>
                      <tr v-for="item in refundableProjects" :key="item.projectId">
                        <td>
                          <div class="flex items-center gap-[12px]">
                            <img :src="item.logo" :alt="getProjectName(item)" class="dash-avatar-sm" />
                            <span class="font-medium text-[#212121]">{{ getProjectName(item) }}</span>
                          </div>
                        </td>
                        <td class="font-semibold tabular-nums text-[#212121]">{{ fmtNum(item.invested) }} {{ item.paymentToken || 'JCC' }}</td>
                        <td style="text-align:right">
                          <button class="dash-btn-danger" :disabled="activeRefundKey !== null || userActionWrongNetwork(item)" @click="handleRefund(item)">
                            {{ refundIsPending(item) ? t.refunding : t.refund }}
                          </button>
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </article>
            </section>
          </div>

          <!-- ============ 項目方端（Desktop） ============ -->
          <div v-else class="w-full px-[80px] mb-[40px]">
            <section v-if="myProjectsError" class="rise-card rise-card-1 dash-card">
              <div class="flex flex-col items-center gap-[10px] px-[28px] py-[40px] text-center">
                <span class="text-[32px]">⚠️</span>
                <p class="text-[15px] font-medium text-[#4c5148]">{{ t.projectLoadFailed }}</p>
                <p class="text-[13px] text-[#808080]">{{ myProjectsError }}</p>
              </div>
            </section>
            <section v-else-if="!myProjects.length" class="rise-card rise-card-1 dash-card">
              <div class="flex flex-col items-center gap-[10px] px-[28px] py-[40px] text-center">
                <span class="text-[32px]">📋</span>
                <p class="text-[15px] font-medium text-[#4c5148]">{{ t.noProjects }}</p>
              </div>
            </section>

            <div class="grid gap-[24px] items-start" :style="projectWorkspaceStyle">
              <!-- 左：項目列表（點擊選中） -->
              <div class="flex flex-col gap-[12px]">
                <section v-for="project in myProjects" :key="project.projectId" class="cursor-pointer transition-shadow" :class="{ 'dash-project-picked': selectedProjectId === project.projectId }" @click="selectProject(project)">
                  <article class="rise-card rise-card-1 dash-card" :class="'dash-card-' + (project.status || 'unknown').toLowerCase()">
                <!-- head -->
                <header class="flex items-center justify-between gap-[16px] border-b border-[#eee] px-[28px] py-[20px]">
                  <div class="flex items-center gap-[14px]">
                    <img :src="project.logo" :alt="getProjectName(project)" class="dash-avatar" />
                    <div>
                      <h3 class="text-[18px] font-bold text-[#212121]">{{ getProjectName(project) }}</h3>
                      <p class="text-[12px] text-[#808080]">ID: {{ project.projectId }} · {{ project.tokenSymbol }} · Chain {{ project.chainId || expectedChainId }}</p>
                    </div>
                  </div>
                  <span class="dash-badge" :class="statusBadgeClass(project)">{{ statusText(project) }}</span>
                </header>

                <div class="px-[28px] py-[24px]">
                  <FairLaunchOwnerPanel
                    v-if="resolveProjectFundingRoute(project).kind === 'fair_launch'"
                    :project="project"
                    :language="props.language"
                    @refresh="refreshPendingData(project)"
                  />
                  <template v-else>
                  <!-- Set -->
                  <p v-if="project.status === 'Set'" class="dash-note note-orange">{{ t.setPrompt }}</p>

                  <!-- Setup -->
                  <div v-else-if="project.status === 'Setup'" class="flex flex-col items-start gap-[16px]">
                    <p class="dash-note note-purple">{{ t.setupPrompt }}</p>
                  </div>

                  <!-- Pending：餘額 / 授權 / 已充值 / 軟頂 / 硬頂 / 缺口 + 兩步充值 -->
                  <div v-else-if="project.status === 'Pending'">
                    <div v-if="resolveProjectFundingRoute(project).kind === 'ido_v3' && project.v3Failure" class="dash-note note-orange mb-[16px]" role="status">
                      {{ v3UserMessage(project.v3Failure, props.language !== 'en') }}
                      <details class="mt-1"><summary>{{ props.language === 'en' ? 'Error code for support' : '供客服排查的錯誤代碼' }}</summary>{{ project.v3Failure }}</details>
                    </div>
                    <details v-if="resolveProjectFundingRoute(project).kind === 'ido_v3' && project.v3Pending?.length" class="dash-hint is-warn mb-[16px] break-all">
                      <summary>{{ props.language === 'en' ? 'View pending transaction IDs' : '查看待確認的交易編號' }}</summary>
                      {{ project.v3Pending.map((item) => item.hash || 'unknown').join(' · ') }}
                    </details>
                    <button v-if="project.v3Pending?.length" class="dash-btn-soft mb-[16px]" :disabled="project.deposit.statusLoading" @click="refreshPendingData(project)">{{ props.language === 'en' ? 'Recheck original transaction' : '重新核對原交易' }}</button>
                    <div v-if="!ownerFundingVisible(project)" class="dash-note note-orange mb-[16px]" role="status">
                      <p>{{ project.deposit.statusLoading ? t.loading : t.loadFailed }}</p>
                      <button class="dash-btn-soft" :disabled="project.deposit.statusLoading" @click="refreshPendingData(project)">{{ t.refresh }}</button>
                    </div>
                    <div v-else class="grid grid-cols-12 gap-[24px]">
                      <!-- 左：鏈上狀態（BigInt 精確計算） -->
                      <div class="col-span-7">
                        <div class="grid grid-cols-2 gap-[12px]">
                          <div class="dash-stat">
                            <p class="dash-stat-label">{{ t.currencyBalance }}</p>
                            <p class="dash-stat-value tabular-nums">{{ project.balance.display }} <small>{{ project.tokenSymbol }}</small></p>
                          </div>
                          <div class="dash-stat" :class="{ 'stat-ok': allowanceCovers(project) }">
                            <p class="dash-stat-label">{{ t.allowance }}</p>
                            <p class="dash-stat-value tabular-nums">{{ project.allowance.display }} <small>{{ project.tokenSymbol }}</small>
                              <span v-if="allowanceCovers(project) && !project.zeroToken" class="dash-stat-tick">✓ {{ t.approveReady }}</span>
                            </p>
                          </div>
                          <div class="dash-stat">
                            <p class="dash-stat-label">{{ t.currentDeposited }}</p>
                            <p class="dash-stat-value tabular-nums">{{ project.deposited.display }} <small>{{ project.tokenSymbol }}</small></p>
                          </div>
                          <div class="dash-stat">
                            <p class="dash-stat-label">{{ t.softCapRequired }}</p>
                            <p class="dash-stat-value tabular-nums">{{ project.softCapRequired.valid ? project.softCapRequired.display : '-' }} <small>{{ project.tokenSymbol }}</small></p>
                          </div>
                          <div class="dash-stat">
                            <p class="dash-stat-label">{{ t.hardCapRequired }}</p>
                            <p class="dash-stat-value tabular-nums">{{ project.hardCapRequired.valid ? project.hardCapRequired.display : '-' }} <small>{{ project.tokenSymbol }}</small></p>
                          </div>
                          <div class="dash-stat" :class="project.shortfall.zero ? '' : 'stat-shortfall'">
                            <p class="dash-stat-label">{{ t.shortfall }}</p>
                            <p class="dash-stat-value tabular-nums">{{ project.shortfall.display }} <small>{{ project.tokenSymbol }}</small></p>
                          </div>
                        </div>

                        <!-- 充值進度 -->
                        <div class="mt-[20px]">
                          <div class="dash-progress-track">
                            <div class="dash-progress-fill" :style="{ width: depositPercent(project) + '%' }"></div>
                          </div>
                          <p class="dash-progress-label">
                            {{ Math.round(depositPercent(project)) }}% · {{ t.currentDeposited }}: {{ project.deposited.display }} / {{ t.hardCapRequired }}: {{ project.hardCapRequired.display }} {{ project.tokenSymbol }}
                          </p>
                        </div>

                        <div class="mt-[12px] flex items-center gap-[10px]">
                          <span class="dash-price-chip">{{ t.softCap }}: {{ fmtNum(project.softCap) }} · {{ t.hardCap }}: {{ fmtNum(project.hardCap) }} {{ project.paymentToken }}</span>
                          <span class="dash-price-chip">{{ t.tokenSymbol }} Price: {{ project.tokenPriceDisplay }} {{ project.paymentToken }}</span>
                          <button class="dash-iconbtn" :disabled="project.deposit.statusLoading" @click="refreshPendingData(project)" :title="t.refresh">
                            <RefreshCw :size="14" :class="{ 'animate-spin': project.deposit.statusLoading }" />
                          </button>
                        </div>
                      </div>

                      <!-- 右：Deposit flow（明確兩步） -->
                      <div class="col-span-5">
                        <div class="dash-deposit-panel">
                          <div class="flex items-center justify-between">
                            <p class="dash-deposit-title">{{ t.depositPrompt }}</p>
                          </div>

                          <div class="dash-auto-amount">
                            <span>{{ t.depositAmount }}</span>
                            <strong>{{ project.shortfall.display }} {{ project.tokenSymbol }}</strong>
                          </div>
                          <p class="dash-hint">{{ props.language === 'en' ? 'The exact token allowance is already filled in your wallet request. 0 BNB is the native transfer value, not the token allowance; gas is separate.' : '已按充值數量自動填入代幣授權額度，無需在錢包重新輸入。0 BNB 是原生幣轉帳額，不是代幣授權額；手續費另計。' }}</p>

                          <p v-if="project.zeroToken" class="dash-hint is-warn">{{ t.tokenAddressNotSet }}</p>
                          <p v-else-if="amountInvalid(project) && !projectWrongNetwork(project)" class="dash-hint is-err">{{ t.invalidAmount }}</p>
                          <p v-else-if="amountTooBig(project) && !projectWrongNetwork(project)" class="dash-hint is-err">{{ t.amountExceedsBalance }}</p>

                          <!-- Step 1: Approve -->
                          <div class="dash-step" :class="'step-' + project.deposit.approveStatus">
                            <div class="dash-step-head">
                              <span class="dash-step-num"><ShieldCheck :size="15" /></span>
                              <span class="dash-step-title">{{ t.approveStep }}</span>
                              <span class="dash-step-sub">{{ t.approveExact }}</span>
                              <span v-if="allowanceCovers(project) && project.deposit.approveStatus !== 'pending'" class="dash-step-ok-pill">{{ t.approveReady }}</span>
                            </div>
                            <button
                              class="dash-btn-lime dash-owner-approve"
                              :disabled="!canApprove(project) || project.deposit.busyApprove"
                              @click="handleApproveExact(project)"
                            >
                              <Loader2 v-if="project.deposit.busyApprove" :size="16" class="animate-spin" />
                              {{ project.deposit.busyApprove ? t.approving : allowanceCovers(project) ? t.approveReady : t.approveExact }}<span v-if="!allowanceCovers(project)"> · {{ project.shortfall.display }} {{ project.tokenSymbol }}</span>
                            </button>
                            <p v-if="project.contractVersion === 'ido_v3_stablecoin' && !allowanceCovers(project)" class="dash-hint">{{ props.language === 'en' ? 'Check the exact token allowance in your wallet. If it shows 0, cancel and reconnect; 0 BNB is not the allowance.' : '請核對錢包內精確授權量。若數量為 0，請取消並重新連接；0 BNB 不是授權數量。' }}</p>
                            <div v-if="project.deposit.approveTxHash" class="dash-step-tx">
                              <span class="dash-step-txlabel">{{ t.txHash }}</span>
                              <a :href="contractService.getTxUrl(project.deposit.approveTxHash, project)" target="_blank" rel="noopener" class="dash-txlink">
                                {{ project.deposit.approveTxHash.slice(0, 12) }}…{{ project.deposit.approveTxHash.slice(-6) }}
                                <ExternalLink :size="13" />
                              </a>
                            </div>
                            <p v-if="project.deposit.approveMsg" class="dash-step-msg" :class="{ 'is-err': project.deposit.approveStatus === 'error' }">{{ project.deposit.approveMsg }}</p>
                            <p v-if="!allowanceCovers(project) && project.deposit.approveStatus === 'success'" class="dash-step-msg is-warn">{{ t.insufficientAllowance }}</p>
                          </div>

                          <!-- Step 2: Deposit -->
                          <div class="dash-step" :class="'step-' + project.deposit.depositStatus">
                            <div class="dash-step-head">
                              <span class="dash-step-num step2"><ArrowDownToLine :size="15" /></span>
                              <span class="dash-step-title">{{ t.depositStep }}</span>
                              <span class="dash-step-sub">{{ t.deposit }}</span>
                            </div>
                            <button
                              class="dash-btn-lime"
                              :disabled="!canDeposit(project) || project.deposit.busyDeposit"
                              @click="handleDepositExact(project)"
                            >
                              <Loader2 v-if="project.deposit.busyDeposit" :size="16" class="animate-spin" />
                              {{ project.deposit.busyDeposit ? t.depositing : t.deposit }}
                            </button>
                            <div v-if="project.deposit.depositTxHash" class="dash-step-tx">
                              <span class="dash-step-txlabel">{{ t.txHash }}</span>
                              <a :href="contractService.getTxUrl(project.deposit.depositTxHash, project)" target="_blank" rel="noopener" class="dash-txlink">
                                {{ project.deposit.depositTxHash.slice(0, 12) }}…{{ project.deposit.depositTxHash.slice(-6) }}
                                <ExternalLink :size="13" />
                              </a>
                            </div>
                            <p v-if="project.deposit.depositMsg" class="dash-step-msg" :class="{ 'is-err': project.deposit.depositStatus === 'error' }">{{ project.deposit.depositMsg }}</p>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>

                  <!-- Active -->
                  <div v-else-if="project.status === 'Active'">
                    <div class="flex items-center justify-between text-[14px] mb-[10px]">
                      <span class="font-medium text-[#4c5148]">{{ t.fundraisingProgress }}</span>
                      <span class="font-bold tabular-nums text-[#212121]">{{ ownerRaisedText(project) }} / {{ fmtNum(project.hardCap) }} {{ project.paymentToken }}</span>
                    </div>
                    <div class="dash-progress-track">
                      <div class="dash-progress-fill" :style="{ width: raisedPercent(project) + '%' }"></div>
                    </div>
                    <p class="dash-progress-label mt-[8px]">{{ Math.round(raisedPercent(project)) }}% · {{ t.softCap }}: {{ fmtNum(project.softCap) }} {{ project.paymentToken }}</p>
                  </div>

                  <!-- Successful -->
                  <div v-else-if="project.status === 'Successful'">
                    <div class="grid grid-cols-2 gap-[12px] mb-[18px]">
                      <div class="dash-stat"><p class="dash-stat-label">{{ t.totalRaisedLabel }}</p><p class="dash-stat-value tabular-nums">{{ ownerRaisedText(project) }} <small>{{ project.paymentToken }}</small></p></div>
                      <div class="dash-stat"><p class="dash-stat-label">{{ t.tokensSold }}</p><p class="dash-stat-value tabular-nums">{{ ownerStatText(project, 'tokensSold', fmtNum) }} <small>{{ project.tokenSymbol }}</small></p></div>
                    </div>
                    <p v-if="project.ownerStatsError" class="dash-note note-orange">{{ t.ownerStatsUnavailable }} <button type="button" @click="refreshProjectChainData(project)">{{ t.retryInvestors }}</button></p>
                    <div class="flex gap-[12px]">
                      <button v-if="!project.fundsWithdrawn" class="dash-btn-lime flex-1" :disabled="isWithdrawingFunds[project.projectId] || isWithdrawingTokens[project.projectId] || projectWrongNetwork(project)" @click="handleWithdrawFunds(project)">
                        {{ isWithdrawingFunds[project.projectId] ? t.withdrawingBtn : t.withdrawFundsBtn }}
                      </button>
                      <span v-else class="dash-done-strip">✅ {{ t.fundsWithdrawnLabel }}</span>
                      <button v-if="!project.tokensWithdrawn" class="dash-btn-soft flex-1" :disabled="isWithdrawingTokens[project.projectId] || isWithdrawingFunds[project.projectId] || projectWrongNetwork(project)" @click="handleWithdrawTokens(project)">
                        {{ isWithdrawingTokens[project.projectId] ? t.withdrawingBtn : t.withdrawTokensBtn }}
                      </button>
                      <span v-else class="dash-done-strip">✅ {{ t.tokensWithdrawnLabel }}</span>
                    </div>
                  </div>

                  <!-- Failed / Canceled -->
                  <div v-else-if="project.status === 'Failed' || project.status === 'Canceled'">
                    <p class="dash-note note-red mb-[16px]">{{ t.failedMessage }}</p>
                    <button v-if="!project.tokensWithdrawn" class="dash-btn-danger" :disabled="isWithdrawingTokens[project.projectId] || projectWrongNetwork(project)" @click="handleWithdrawTokens(project)">
                      {{ isWithdrawingTokens[project.projectId] ? t.withdrawingBtn : t.withdrawAllTokens }}
                    </button>
                    <span v-else class="dash-done-strip">✅ {{ t.tokensWithdrawnLabel }}</span>
                  </div>
                  </template>
                </div>

              </article>
              </section>
              </div>

              <!-- 右：選中項目投資記錄（分頁） -->
              <section v-if="showSelectedProjectInvestors" class="min-w-0">
                <article class="rise-card rise-card-2 dash-card dash-investors-card">
                  <header class="dash-investors-head flex items-center justify-between border-b border-[#eee] px-[20px] py-[16px]">
                    <h3 class="dash-investors-title flex items-center gap-[8px] text-[16px] font-bold text-[#212121] min-w-0">
                      <Users :size="15" class="shrink-0" /> <span class="truncate">{{ selectedProject ? getProjectName(selectedProject) + ' · ' + t.investors : t.investors }}</span>
                    </h3>
                    <button type="button" class="dash-iconbtn shrink-0" :title="t.refresh" :disabled="projectInvestors[selectedProjectId]?.loading" @click="loadProjectInvestors(selectedProject, 1)"><RefreshCw :size="14" /></button>
                  </header>
                  <div class="px-[20px] py-[16px]">
                    <p v-if="!selectedProject" class="dash-empty-text">{{ t.selectProjectHint }}</p>
                    <p v-else-if="!projectInvestors[selectedProjectId] || projectInvestors[selectedProjectId].loading" class="dash-investors-loading text-[12px] text-[#808080]">{{ t.loading }}</p>
                    <div v-else-if="projectInvestors[selectedProjectId].error" class="dash-portfolio-notice" role="alert">
                      <p>{{ t.investorsLoadFailed }}</p>
                      <button type="button" class="dash-iconbtn mt-2" @click="loadProjectInvestors(selectedProject, projectInvestors[selectedProjectId].page)">{{ t.retryInvestors }}</button>
                    </div>
                    <p v-else-if="!projectInvestors[selectedProjectId].items.length" class="dash-empty-text">{{ t.noInvestors }}</p>
                    <div v-else class="flex flex-col gap-[8px]">
                      <div v-for="inv in projectInvestors[selectedProjectId].items" :key="inv.id" class="dash-investor-row flex items-center justify-between gap-[12px] rounded-[8px] bg-[#fafaf8] border border-[#eee] px-[12px] py-[10px]">
                        <div class="flex items-center gap-[8px] shrink-0">
                          <img v-if="inv.twitterAvatar" :src="inv.twitterAvatar" :title="inv.twitterHandle ? 'X @' + inv.twitterHandle : ''" class="dash-investor-avatar w-[20px] h-[20px] rounded-full border border-[#eee] bg-[#fff]" alt="X" />
                          <img v-if="inv.telegramAvatar" :src="inv.telegramAvatar" :title="inv.telegramUsername ? 'Telegram @' + inv.telegramUsername : ''" class="dash-investor-avatar w-[20px] h-[20px] rounded-full border border-[#eee] bg-[#fff]" alt="TG" />
                        </div>
                        <div class="flex flex-col min-w-0 flex-1">
                          <span class="dash-investor-wallet text-[12px] font-mono text-[#212121] truncate" :title="inv.walletAddress">{{ inv.walletAddress.slice(0, 10) }}…{{ inv.walletAddress.slice(-6) }}</span>
                          <div class="flex items-center gap-[8px] flex-wrap">
                            <span class="dash-investor-date text-[11px] text-[#808080]">{{ formatDate(inv.createdAt * 1000) }}</span>
                            <span v-if="inv.twitterHandle" class="dash-investor-social is-twitter text-[11px] font-medium text-[#3884ff]">@{{ inv.twitterHandle }}</span>
                            <span v-if="inv.telegramUsername" class="dash-investor-social is-telegram text-[11px] font-medium text-[#229ED9]">@{{ inv.telegramUsername }}</span>
                          </div>
                        </div>
                        <span class="dash-investor-amount text-[13px] font-bold tabular-nums text-[#212121] shrink-0">+{{ fmtNum(inv.amountDisplay) }} {{ inv.tokenSymbol }}</span>
                      </div>

                      <div class="dash-investors-footer flex items-center justify-between pt-[10px] border-t border-[#eee] mt-[4px]">
                        <span class="dash-investors-total text-[11px] text-[#808080]">{{ t.totalCount }} {{ projectInvestors[selectedProjectId].total }}</span>
                        <div class="flex items-center gap-[8px]">
                          <button type="button" class="dash-iconbtn" :disabled="projectInvestors[selectedProjectId].page <= 1" @click="loadProjectInvestors(selectedProject, projectInvestors[selectedProjectId].page - 1)">‹</button>
                          <span class="dash-investors-page text-[12px] tabular-nums text-[#4c5148]">{{ projectInvestors[selectedProjectId].page }} / {{ totalPages(selectedProjectId) }}</span>
                          <button type="button" class="dash-iconbtn" :disabled="projectInvestors[selectedProjectId].page >= totalPages(selectedProjectId)" @click="loadProjectInvestors(selectedProject, projectInvestors[selectedProjectId].page + 1)">›</button>
                        </div>
                      </div>
                    </div>
                  </div>
                </article>
              </section>
            </div>
          </div>
        </template>

        <HomeFooter :language="props.language" />
      </main>
    </div>
  </div>

  <!-- Modal -->
  <Modal
    :show="showModal"
    :type="modalConfig.type"
    :title="modalConfig.title"
    :message="modalConfig.message"
    :confirmText="t.ok"
    :isDark="isDark"
    @close="showModal = false"
  >
    <div v-if="modalConfig.txHash" class="text-center">
      <div class="dashboard-modal-tx rounded-lg p-4 mb-4" :class="{ 'is-dark': isDark }">
        <p class="text-xs text-[#808080] mb-1">{{ t.txHash }}</p>
        <p class="dashboard-modal-hash text-sm font-mono break-all">{{ modalConfig.txHash }}</p>
      </div>
      <a
        :href="contractService.getTxUrl(modalConfig.txHash, modalConfig)"
        target="_blank"
        rel="noopener noreferrer"
        class="inline-flex items-center gap-2 text-[#4a8c00] text-sm font-semibold transition-colors hover:underline"
      >
        <span>{{ t.viewOnBscscan }}</span>
        <ExternalLink :size="16" />
      </a>
    </div>
  </Modal>

</template>

<style scoped>
/* ============================================================
   Dashboard — jc-shell 設計語言（桌面 1440 畫布 + 移動端 m-* 風格）
   ============================================================ */

/* ---------- 共用小件 ---------- */
.dash-eyebrow-tag {
  width: 22px; height: 22px;
  background: #9bff00;
  border-radius: 14px 3px 3px 3px;
  transform: rotate(15deg);
  display: inline-block;
  flex-shrink: 0;
}
.dash-dot {
  width: 7px; height: 7px; border-radius: 50%;
  background: #9bff00;
  box-shadow: 0 0 0 2px rgba(155, 255, 0, 0.25);
  display: inline-block;
  flex-shrink: 0;
}
.dash-dot.is-bad { background: #ff4d4f; box-shadow: 0 0 0 2px rgba(255, 77, 79, 0.25); }
.dash-portfolio-notice { border: 1px solid #d99b39; border-radius: 12px; background: #fff7e6; color: #754a12; padding: 18px; }
.dash-portfolio-notice p { margin: 8px 0 12px; line-height: 1.6; }
.dash-portfolio-loading, .dash-refund-completed { color: #595f58; }
.dash-refund-available { color: #b32222; }
.home-dark .dash-portfolio-notice { background: #302719; border-color: #916d32; color: #ffd18a; }
.home-dark .dash-portfolio-loading, .home-dark .dash-refund-completed { color: #b8b8b0; }
.home-dark .dash-refund-available { color: #ffa39e; }
.dash-investment-overview {
  display: grid;
  grid-template-columns: minmax(0, 2.15fr) minmax(330px, 0.85fr);
  align-items: stretch;
  gap: 24px;
}
.dash-investment-overview > section,
.dash-investment-overview > section > article { min-width: 0; height: 100%; }
.dash-section-icon {
  display: inline-flex;
  width: 32px;
  height: 32px;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border: 1px solid rgba(74, 140, 0, 0.18);
  border-radius: 9px;
  background: rgba(155, 255, 0, 0.22);
  color: #397900;
}
.dash-section-icon.is-large { width: 44px; height: 44px; border-radius: 12px; }
.dash-points-card { overflow: hidden; }
.dash-points-head {
  display: flex;
  min-height: 86px;
  align-items: center;
  gap: 12px;
  border-bottom: 1px solid #eee;
  padding: 16px 18px;
}
.dash-points-kicker { margin: 0; color: #7f857b; font-size: 10px; font-weight: 700; letter-spacing: 1px; text-transform: uppercase; }
.dash-points-total { margin: 2px 0 0; color: #212121; font-size: 25px; font-weight: 750; line-height: 1; font-variant-numeric: tabular-nums; }
.dash-points-total small { color: #4a8c00; font-size: 10px; letter-spacing: 1px; }
.dash-points-link {
  display: inline-flex;
  margin-left: auto;
  align-items: center;
  gap: 4px;
  border: 0;
  background: transparent;
  color: #4a8c00;
  font-size: 10px;
  font-weight: 700;
  cursor: pointer;
}
.dash-point-list { display: flex; flex-direction: column; padding: 8px 16px 12px; }
.dash-point-row { display: flex; min-height: 56px; align-items: center; gap: 10px; border-bottom: 1px solid #f0f0ec; }
.dash-point-row:last-child { border-bottom: 0; }
.dash-point-dot { width: 7px; height: 7px; flex: 0 0 auto; border-radius: 50%; background: #9bff00; box-shadow: 0 0 0 4px rgba(155,255,0,.15); }
.dash-point-copy { display: flex; min-width: 0; flex: 1; flex-direction: column; gap: 2px; }
.dash-point-copy strong { color: #292d27; font-size: 12px; }
.dash-point-copy span { overflow: hidden; color: #8a9188; font-size: 9px; text-overflow: ellipsis; white-space: nowrap; }
.dash-point-change { color: #1a9f45; font-size: 13px; font-weight: 800; font-variant-numeric: tabular-nums; }
.dash-points-empty { margin: 0; padding: 34px 22px; color: #8a9188; font-size: 12px; line-height: 1.55; text-align: center; }
.dashboard-modal-tx { border: 1px solid #e6e8e2; background: #f6f6f4; }
.dashboard-modal-hash { color: #212121; }
.dashboard-modal-tx.is-dark { border-color: rgba(255,255,255,.1); background: #242622; }
.dashboard-modal-tx.is-dark .dashboard-modal-hash { color: #fafaf7; }

/* ---------- 桌面 chip / 按鈕 ---------- */
.dash-chip {
  display: inline-flex; align-items: center; gap: 10px;
  background: #fff; border: 1px solid #ececea; border-radius: 999px;
  padding: 10px 16px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.05);
}
.dash-chip.chip-bad { border-color: #ffccc7; background: #fff1f0; }
.dash-chip-main { font-size: 14px; font-weight: 700; color: #212121; }
.dash-chip-sub { font-size: 11px; font-weight: 600; color: #808080; }
.dash-iconbtn {
  display: inline-flex; align-items: center; justify-content: center;
  width: 40px; height: 40px;
  border: 1px solid #ececea; border-radius: 999px;
  background: #fff; color: #808080; cursor: pointer;
  transition: color 0.2s ease, border-color 0.2s ease;
}
.dash-iconbtn:hover { color: #212121; border-color: #9bff00; }
.dash-iconbtn:disabled { opacity: 0.5; cursor: not-allowed; }

.dash-btn-lime,
.dash-btn-dark,
.dash-btn-soft,
.dash-btn-danger {
  display: inline-flex; align-items: center; justify-content: center; gap: 8px;
  height: 44px; padding: 0 22px;
  border-radius: 999px; border: none;
  font-size: 14px; font-weight: 700; cursor: pointer;
  transition: transform 0.15s ease, box-shadow 0.2s ease, opacity 0.2s ease;
}
.dash-btn-lime { background: #9bff00; color: #212121; }
.dash-btn-lime:hover:not(:disabled) { box-shadow: 0 8px 20px rgba(155, 255, 0, 0.35); transform: translateY(-1px); }
.dash-btn-dark { background: #212121; color: #fff; }
.dash-btn-dark:hover:not(:disabled) { box-shadow: 0 8px 20px rgba(33, 33, 33, 0.3); transform: translateY(-1px); }
.dash-btn-soft { background: #f0efec; color: #212121; }
.dash-btn-soft:hover:not(:disabled) { transform: translateY(-1px); }
.dash-btn-danger { background: #ff4d4f; color: #fff; }
.dash-btn-danger:hover:not(:disabled) { transform: translateY(-1px); }
.dash-btn-lime:disabled,
.dash-btn-dark:disabled,
.dash-btn-soft:disabled,
.dash-btn-danger:disabled { opacity: 0.45; cursor: not-allowed; }
.dash-owner-approve:disabled,
.jc-mobile-page .dash-owner-approve:disabled { background: #e5e5e1; color: #85857f; opacity: 1; box-shadow: none; cursor: not-allowed; }
.home-dark .dash-owner-approve:disabled,
.jc-mobile-page.home-dark .dash-owner-approve:disabled { background: #30302d; color: #92928a; }

/* ---------- 桌面 tabs ---------- */
.dash-tab {
  display: inline-flex; align-items: center; justify-content: center;
  height: 46px; padding: 0 28px;
  border: 1px solid #ececea; border-radius: 999px;
  background: #fff; color: #4c5148;
  font-size: 14px; font-weight: 700; cursor: pointer;
  transition: all 0.2s ease;
}
.dash-tab:hover { border-color: #9bff00; color: #212121; }
.dash-tab.is-on { background: #212121; color: #fff; border-color: #212121; }
.dash-subtab {
  position: relative; display: inline-flex; align-items: center;
  height: 54px; padding: 0 20px;
  background: none; border: none; cursor: pointer;
  font-size: 13px; font-weight: 700; color: #8a9188;
  transition: color 0.2s ease;
}
.dash-subtab:hover { color: #212121; }
.dash-subtab.is-on { color: #212121; }
.dash-subtab.is-on::after {
  content: ''; position: absolute; left: 16px; right: 16px; bottom: 0; height: 3px;
  background: #9bff00; border-radius: 3px 3px 0 0;
}

/* ---------- 桌面卡 ---------- */
.dash-card {
  background: #fff; border: 1px solid #ececea; border-radius: 12px;
  overflow: visible; box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
}
.dash-project-picked .dash-card { border-color: #9bff00; box-shadow: 0 0 0 2px rgba(155, 255, 0, 0.35); }
.dash-card-danger { border-color: #ffd8bf; }
.dash-card-pending { border-color: #ffd591; }
.dash-card-active { border-color: #b7eb8f; }
.dash-card-successful { border-color: #b7eb8f; }
.dash-card-failed, .dash-card-canceled { border-color: #ffa39e; }
.dash-card-setup { border-color: #d3adf7; }
.dash-card-set { border-color: #ececea; }

.dash-count-pill {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 22px; height: 22px; padding: 0 8px;
  background: rgba(155, 255, 0, 0.35); color: #1aa64a;
  border-radius: 999px; font-size: 11px; font-weight: 700;
}

/* ---------- 表格 ---------- */
.dash-table { border-collapse: collapse; }
.dash-table th {
  padding: 12px 20px; text-align: left;
  font-size: 10px; font-weight: 700; letter-spacing: 1.2px; text-transform: uppercase;
  color: #808080; background: #fafaf8;
  border-bottom: 1px solid #eee;
}
.dash-table td { padding: 16px 20px; border-bottom: 1px solid #f0f0ec; font-size: 14px; }
.dash-table tbody tr:last-child td { border-bottom: none; }
.dash-table tbody tr { transition: background 0.15s ease; }
.dash-table tbody tr:hover { background: #fafaf8; }
.dash-avatar { width: 44px; height: 44px; border-radius: 10px; background: #f0efec; object-fit: cover; }
.dash-avatar-sm { width: 32px; height: 32px; border-radius: 8px; background: #f0efec; object-fit: cover; }
.dash-pill {
  display: inline-flex; align-items: center;
  padding: 4px 12px; border-radius: 999px;
  font-size: 11px; font-weight: 700;
}
.pill-green { background: rgba(155, 255, 0, 0.3); color: #1aa64a; }
.pill-gray { background: #f0efec; color: #8a9188; }
.pill-blue { background: #e6f0ff; color: #3884ff; }
.pill-red { background: #fff1f0; color: #cf1322; }
.pill-orange { background: #fff7e6; color: #d46b08; }
.dash-txlink {
  display: inline-flex; align-items: center; gap: 6px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 13px; font-weight: 600; color: #4a8c00;
  text-decoration: none;
}
.dash-txlink:hover { text-decoration: underline; }
.dash-empty { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 56px 20px; }
.dash-empty-icon { font-size: 36px; }
.dash-empty-text { font-size: 14px; color: #8a9188; margin: 0; }

/* ---------- 狀態徽章 ---------- */
.dash-badge, .m-dash-badge {
  display: inline-flex; align-items: center;
  padding: 5px 12px; border-radius: 999px;
  font-size: 11px; font-weight: 700; letter-spacing: 0.4px; white-space: nowrap;
}
.badge-bw { background: #f0efec; color: #595f58; }
.badge-purple { background: #f3e8ff; color: #722ed1; }
.badge-amber { background: #fff7e6; color: #d46b08; }
.badge-teal { background: #e6fffb; color: #08979c; }
.badge-blue { background: #e6f0ff; color: #1d39c4; }
.badge-green { background: rgba(155, 255, 0, 0.3); color: #1aa64a; }
.badge-red { background: #fff1f0; color: #cf1322; }

/* ---------- 項目卡 note ---------- */
.dash-note, .m-dash-proj-note {
  font-size: 13px; line-height: 1.55; padding: 12px 16px;
  border-radius: 10px; margin: 0 0 12px;
}
.note-orange { background: #fff7e6; color: #d46b08; border: 1px solid #ffd591; }
.note-purple { background: #f3e8ff; color: #722ed1; border: 1px solid #d3adf7; }
.note-red { background: #fff1f0; color: #cf1322; border: 1px solid #ffa39e; }

/* ---------- 統計格 ---------- */
.dash-stat {
  background: #fafaf8; border: 1px solid #f0f0ec; border-radius: 10px;
  padding: 14px 16px; display: flex; flex-direction: column; gap: 6px;
  min-height: 74px;
}
.dash-stat.stat-ok { border-color: #b7eb8f; background: #fbfff5; }
.dash-stat.stat-shortfall { border-color: #ffd8bf; background: #fff7f5; }
.dash-stat-label {
  font-size: 10px; font-weight: 700; letter-spacing: 1.2px; text-transform: uppercase;
  color: #808080; margin: 0;
}
.dash-stat-value { font-size: 19px; font-weight: 700; color: #212121; margin: 0; }
.dash-stat-value small { font-size: 11px; font-weight: 700; color: #808080; }
.dash-stat-tick { display: inline-flex; margin-left: 8px; font-size: 10px; font-weight: 700; color: #1aa64a; background: rgba(155,255,0,0.3); padding: 2px 8px; border-radius: 999px; }

/* ---------- 進度條 ---------- */
.dash-progress-track {
  height: 8px; border-radius: 999px; background: #f0efec; overflow: hidden;
}
.dash-progress-fill {
  height: 100%; border-radius: 999px; background: #9bff00;
  transition: width 0.5s ease;
}
.dash-progress-label { font-size: 11px; font-weight: 600; color: #8a9188; margin: 6px 0 0; }
.dash-price-chip {
  display: inline-flex; align-items: center;
  background: #fafaf8; border: 1px solid #f0f0ec; border-radius: 999px;
  padding: 5px 12px; font-size: 11px; font-weight: 600; color: #595f58;
}
.dash-done-strip {
  display: flex; flex: 1; align-items: center; justify-content: center;
  height: 44px; border-radius: 999px;
  background: #f0efec; color: #8a9188;
  font-size: 13px; font-weight: 700;
}

/* ---------- Deposit 面板（兩步） ---------- */
.dash-deposit-panel {
  background: #fafaf8; border: 1px solid #f0f0ec; border-radius: 12px;
  padding: 20px; display: flex; flex-direction: column; gap: 12px;
}
.dash-deposit-title { font-size: 13px; font-weight: 700; color: #212121; margin: 0; }
.dash-auto-amount { width: 100%; min-height: 46px; padding: 0 14px; border: 1px solid #e2e2de; border-radius: 10px; background: #fafaf8; display: flex; align-items: center; justify-content: space-between; gap: 12px; color: #808080; font-size: 12px; font-weight: 700; }
.dash-auto-amount strong { color: #212121; font-size: 15px; font-variant-numeric: tabular-nums; text-align: right; overflow-wrap: anywhere; }
.dash-fill-btn {
  background: none; border: 1px solid #d9d9d9; border-radius: 999px;
  padding: 5px 12px; font-size: 11px; font-weight: 700; color: #595f58; cursor: pointer;
  transition: border-color 0.2s ease, color 0.2s ease;
}
.dash-fill-btn:hover:not(:disabled) { border-color: #9bff00; color: #212121; }
.dash-fill-btn:disabled { opacity: 0.45; cursor: not-allowed; }
.dash-input {
  width: 100%; height: 46px;
  border: 1px solid #e2e2de; border-radius: 10px;
  padding: 0 14px; font-size: 15px; font-weight: 600; color: #212121;
  background: #fff; outline: none;
  font-variant-numeric: tabular-nums;
  transition: border-color 0.2s ease;
}
.dash-input:focus { border-color: #9bff00; }
.dash-input:disabled { opacity: 0.6; }
.dash-hint { font-size: 11px; font-weight: 600; margin: 0; }
.dash-hint.is-err { color: #cf1322; }
.dash-hint.is-warn { color: #d46b08; }

.dash-step {
  border: 1px solid #f0f0ec; border-radius: 10px;
  background: #fff; padding: 14px;
  display: flex; flex-direction: column; gap: 10px;
}
.dash-step.step-pending { border-color: #91caff; }
.dash-step.step-success { border-color: #b7eb8f; background: #fbfff5; }
.dash-step.step-error { border-color: #ffa39e; background: #fff7f5; }
.dash-step-head { display: flex; align-items: center; gap: 8px; }
.dash-step-num {
  display: inline-flex; align-items: center; justify-content: center;
  width: 26px; height: 26px; border-radius: 8px;
  background: #212121; color: #fff; flex-shrink: 0;
}
.dash-step-num.step2 { background: #1aa64a; }
.dash-step-title { font-size: 13px; font-weight: 700; color: #212121; }
.dash-step-sub { font-size: 11px; font-weight: 600; color: #8a9188; margin-left: auto; }
.dash-step-ok-pill {
  display: inline-flex; align-items: center;
  background: rgba(155, 255, 0, 0.3); color: #1aa64a;
  padding: 3px 10px; border-radius: 999px;
  font-size: 10px; font-weight: 700; white-space: nowrap;
}
.dash-step-tx { display: flex; align-items: center; gap: 8px; }
.dash-step-txlabel { font-size: 10px; font-weight: 700; color: #8a9188; text-transform: uppercase; letter-spacing: 1px; }
.dash-step-msg { font-size: 12px; font-weight: 600; margin: 0; color: #1aa64a; }
.dash-step-msg.is-err { color: #cf1322; }
.dash-step-msg.is-warn { color: #d46b08; }

/* ============================================================
   MOBILE (<= 768px)
   ============================================================ */
@media (max-width: 768px) {
  .m-dash-hero { padding-top: 56px; }
  .m-dash-hero-title {
    font-size: 36px; line-height: 1.12; letter-spacing: -0.8px;
    color: #212121; margin: 8px 0 0; font-weight: 700;
  }
  .m-dash-hero-sub { font-size: 14px; line-height: 1.55; color: #4c5148; margin: 4px 0 0; }
  .m-dash-chips { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-top: 6px; }
  .m-dash-chip {
    display: inline-flex; align-items: center; gap: 8px;
    background: #fff; border: 1px solid #ececea; border-radius: 999px;
    padding: 8px 14px; font-size: 12px; font-weight: 700; color: #212121;
  }
  .m-dash-chip.chip-ok { background: #fff; }
  .m-dash-chip.chip-bad { background: #fff1f0; border-color: #ffccc7; color: #cf1322; }
  .m-dash-connectbtn { height: 40px; padding: 0 18px; }
  .m-dash-iconbtn {
    display: inline-flex; align-items: center; justify-content: center;
    width: 38px; height: 38px; border-radius: 999px;
    border: 1px solid #ececea; background: #fff; color: #595f58;
  }
  .m-dash-iconbtn:disabled { opacity: 0.5; }

  .m-dash-netwarn {
    display: flex; align-items: flex-start; gap: 10px;
    margin: 4px 20px 0; padding: 14px 16px;
    background: #fff1f0; border: 1px solid #ffccc7; border-radius: 12px;
  }
  .m-dash-netwarn .material-symbols-outlined, .m-dash-netwarn svg { color: #ff4d4f; }
  .m-dash-netwarn-title { font-size: 14px; font-weight: 700; color: #cf1322; margin: 0; }
  .m-dash-netwarn-desc { font-size: 12px; line-height: 1.5; color: #a8071a; margin: 2px 0 0; }

  .m-dash-empty { display: flex; flex-direction: column; align-items: center; gap: 10px; padding: 32px 20px; text-align: center; }
  .m-dash-empty-icon { font-size: 34px; }
  .m-dash-empty-text { font-size: 13px; color: #8a9188; margin: 0; }
  .m-dash .dash-points-card { display: block; padding: 0; }
  .m-dash .dash-points-head { min-height: 82px; padding: 14px; }
  .m-dash .dash-points-link { max-width: 112px; justify-content: flex-end; text-align: right; }
  .m-dash .dash-point-list { padding: 6px 14px 10px; }
  .m-dash .dash-point-row { min-height: 58px; }

  .m-dash-tabs {
    display: flex; gap: 8px; padding: 0 20px; margin-top: 20px; flex-wrap: wrap;
  }
  .m-dash-tab {
    flex: 1; min-width: max-content; height: 42px; padding: 0 18px;
    border: 1px solid #ececea; border-radius: 999px;
    background: #fff; color: #4c5148;
    font-size: 13px; font-weight: 700; cursor: pointer;
  }
  .m-dash-tab.is-on { background: #212121; color: #fff; border-color: #212121; }
  .m-dash-his-tabs { margin-top: 8px; }

  .m-section-head { position: relative; }
  .m-dash-section-refresh {
    display: inline-flex; align-items: center; justify-content: center;
    width: 30px; height: 30px; margin-left: auto;
    border: 1px solid #ececea; border-radius: 999px; background: #fff; color: #595f58;
  }

  .m-dash-row { position: relative; }
  .m-dash-row-head { display: flex; align-items: flex-start; gap: 12px; }
  .m-dash-row-avatar { width: 40px; height: 40px; border-radius: 10px; background: #f0efec; object-fit: cover; flex-shrink: 0; }
  .m-dash-row-title { flex: 1; min-width: 0; }
  .m-dash-row-title h4 { font-size: 15px; font-weight: 700; color: #212121; margin: 0; line-height: 1.3; }
  .m-dash-row-title p { font-size: 12px; color: #808080; margin: 2px 0 0; line-height: 1.4; }
  .m-dash-row-meta { display: flex; flex-direction: column; gap: 8px; }
  .m-dash-row-cols { display: flex; gap: 12px; }
  .m-dash-row-cols > div { flex: 1; }
  .m-dash-row-label {
    font-size: 10px; font-weight: 700; letter-spacing: 1px; text-transform: uppercase;
    color: #808080; margin: 0;
  }
  .m-dash-row-value { font-size: 15px; font-weight: 700; color: #212121; margin: 2px 0 0; font-variant-numeric: tabular-nums; line-height: 1.35; }
  .m-dash-row-value small { font-size: 11px; font-weight: 600; color: #808080; }
  .m-dash-pill-ok {
    display: inline-flex; align-items: center; gap: 5px;
    background: rgba(155, 255, 0, 0.3); color: #1aa64a;
    padding: 4px 10px; border-radius: 999px;
    font-size: 10px; font-weight: 700; letter-spacing: 0.6px; text-transform: uppercase; white-space: nowrap;
  }
  .m-dash-pill-muted { display: inline-flex; align-items: center; padding: 4px 10px; border-radius: 999px; background: #f0efec; color: #8a9188; font-size: 10px; font-weight: 700; }
  .m-dash-pill-blue { display: inline-flex; align-items: center; padding: 4px 10px; border-radius: 999px; background: #e6f0ff; color: #3884ff; font-size: 10px; font-weight: 700; }
  .m-dash-pill-red { display: inline-flex; align-items: center; padding: 4px 10px; border-radius: 999px; background: #fff1f0; color: #cf1322; font-size: 10px; font-weight: 700; }
  .m-dash-pill-orange { display: inline-flex; align-items: center; padding: 4px 10px; border-radius: 999px; background: #fff7e6; color: #d46b08; font-size: 10px; font-weight: 700; }
  .m-dash-row-actions { display: flex; gap: 8px; margin-top: 4px; }
  .m-dash-row-actions-col { flex-direction: column; }
  .m-dash-row-btn { width: 100%; height: 42px; }
  .m-dash-btn-danger {
    display: inline-flex; align-items: center; justify-content: center;
    height: 42px; padding: 0 20px; border: none; border-radius: 999px;
    background: #ff4d4f; color: #fff; font-size: 13px; font-weight: 700; cursor: pointer;
  }
  .m-dash-btn-danger:disabled { opacity: 0.45; }
  .m-dash-txlink {
    display: inline-flex; align-items: center; gap: 4px;
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 12px; font-weight: 600; color: #4a8c00; text-decoration: none;
  }

  /* 項目卡 */
  .m-dash-proj { gap: 14px; }
  .m-dash-proj .m-dash-badge { align-self: center; }
  .m-dash-pending-stats {
    display: flex; flex-direction: column; gap: 8px;
    border-top: 1px solid #f0f0ec; padding-top: 12px;
  }
  .m-dash-pending-stats > div {
    display: flex; align-items: baseline; justify-content: space-between; gap: 10px;
  }
  .m-dash-pending-stats .is-shortfall { padding: 8px 10px; background: #fff7f5; border: 1px solid #ffd8bf; border-radius: 10px; }
  .m-dash-pending-stats .text-short { color: #cf1322; }
  .m-dash-progress { display: flex; flex-direction: column; gap: 6px; }
  .m-dash-progress-track { height: 7px; border-radius: 999px; background: #f0efec; overflow: hidden; }
  .m-dash-progress-fill { height: 100%; border-radius: 999px; background: #9bff00; transition: width 0.5s ease; }
  .m-dash-progress-label { font-size: 11px; font-weight: 600; color: #8a9188; margin: 0; }
  .m-dash-pending-form { display: flex; flex-direction: column; gap: 10px; }
.m-dash-auto-amount { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 42px; padding: 0 12px; border: 1px solid #e2e2de; border-radius: 10px; background: #fafaf8; color: #595f58; font-size: 12px; font-weight: 700; }
.m-dash-auto-amount strong { color: #212121; font-size: 14px; font-variant-numeric: tabular-nums; text-align: right; overflow-wrap: anywhere; }
  .m-dash-input {
    flex: 1; min-width: 0; height: 42px;
    border: 1px solid #e2e2de; border-radius: 10px;
    padding: 0 12px; font-size: 14px; font-weight: 600; color: #212121; background: #fff;
    outline: none; font-variant-numeric: tabular-nums;
  }
  .m-dash-input:disabled { opacity: 0.6; }
  .m-dash-input-fill {
    height: 42px; padding: 0 14px; border: 1px solid #d9d9d9; border-radius: 10px;
    background: #fff; color: #595f58; font-size: 12px; font-weight: 700; cursor: pointer; white-space: nowrap;
  }
  .m-dash-input-fill:disabled { opacity: 0.45; }
  .m-dash-step {
    border: 1px solid #eceae6; border-radius: 12px;
    background: #fafaf8; padding: 12px;
    display: flex; flex-direction: column; gap: 10px;
  }
  .m-dash-step.step-pending { border-color: #91caff; }
  .m-dash-step.step-success { border-color: #b7eb8f; background: #fbfff5; }
  .m-dash-step.step-error { border-color: #ffa39e; background: #fff7f5; }
  .m-dash-step-head { display: flex; align-items: center; gap: 8px; }
  .m-dash-step-num {
    display: inline-flex; align-items: center; justify-content: center;
    width: 24px; height: 24px; border-radius: 7px; background: #212121; color: #fff;
  }
  .m-dash-step-num.step2 { background: #1aa64a; }
  .m-dash-step-title { font-size: 13px; font-weight: 700; color: #212121; }
  .m-dash-step-msg { font-size: 12px; font-weight: 600; color: #1aa64a; margin: 0; }
  .m-dash-step-msg.is-err { color: #cf1322; }
  .m-dash-step-tx { font-size: 12px; }
  .m-dash-hint { font-size: 11px; font-weight: 600; margin: 0; }
  .m-dash-hint.is-err { color: #cf1322; }
  .m-dash-hint.is-warn { color: #d46b08; }

  /* 移動端 dark */
  .home-dark .m-dash-hero-title { color: #fafaf7; }
  .home-dark .m-dash-hero-sub { color: #b8b8b0; }
  .home-dark .m-dash-chip { background: #1f1f1f; border-color: rgba(255,255,255,0.08); color: #fafaf7; }
  .home-dark .m-dash-chip.chip-bad { background: #3a2222; border-color: rgba(255, 120, 117, 0.5); color: #ffa39e; }
  .home-dark .m-dash-iconbtn { background: #1f1f1f; border-color: rgba(255,255,255,0.1); color: #b8b8b0; }
  .home-dark .m-dash-netwarn { background: #3a2222; border-color: rgba(255, 120, 117, 0.45); }
  .home-dark .m-dash-netwarn-title { color: #ffa39e; }
  .home-dark .m-dash-netwarn-desc { color: #ffccc7; }
  .home-dark .m-dash-tab { background: #1f1f1f; border-color: rgba(255,255,255,0.1); color: #b8b8b0; }
  .home-dark .m-dash-tab.is-on { background: #9bff00; color: #212121; border-color: #9bff00; }
  .home-dark .m-dash-row-title h4 { color: #fafaf7; }
  .home-dark .m-dash-row-title p { color: #9a9a92; }
  .home-dark .m-dash-row-value { color: #fafaf7; }
  .home-dark .m-dash-row-value small { color: #9a9a92; }
  .home-dark .m-dash-pill-muted { background: #2a2a2a; color: #9a9a92; }
  .home-dark .m-dash-pill-blue { background: #14233f; color: #7cb2ff; }
  .home-dark .m-dash-pill-red { background: #3a1d1d; color: #ff9c9c; }
  .home-dark .m-dash-pill-orange { background: #3a2c14; color: #ffc069; }
  .home-dark .m-dash-pending-stats { border-top-color: rgba(255,255,255,0.08); }
  .home-dark .m-dash-pending-stats .is-shortfall { background: #3a2222; border-color: rgba(255, 120, 117, 0.4); }
  .home-dark .m-dash-pending-stats .text-short { color: #ffa39e; }
  .home-dark .m-dash-progress-track { background: #2a2a2a; }
  .home-dark .m-dash-progress-label { color: #9a9a92; }
.home-dark .m-dash-auto-amount { background: #1f1f1f; border-color: rgba(255,255,255,0.12); color: #9a9a92; }
.home-dark .m-dash-auto-amount strong { color: #fafaf7; }
  .home-dark .m-dash-input { background: #1f1f1f; border-color: rgba(255,255,255,0.12); color: #fafaf7; }
  .home-dark .m-dash-input-fill { background: #1f1f1f; border-color: rgba(255,255,255,0.15); color: #b8b8b0; }
  .home-dark .m-dash-step { background: #1f1f1f; border-color: rgba(255,255,255,0.08); }
  .home-dark .m-dash-step.step-success { background: #1f2b1e; border-color: rgba(155,255,0,0.35); }
  .home-dark .m-dash-step.step-error { background: #3a2222; border-color: rgba(255, 120, 117, 0.4); }
  .home-dark .m-dash-step-title { color: #fafaf7; }
  .home-dark .m-dash-step-msg { color: #9bff00; }
  .home-dark .m-dash-step-msg.is-err { color: #ffa39e; }
  .home-dark .m-dash-hint.is-warn { color: #ffc069; }
}

/* ============================================================
   桌面 dark（jc-shell home-dark 全局已處理 bg-white/text-[#212121]，
   這裡補自定義元素）
   ============================================================ */
.home-dark .dash-card { background: #161616; border-color: rgba(255,255,255,0.08); }
.home-dark .dash-section-icon { border-color: rgba(155,255,0,.22); background: rgba(155,255,0,.1); color: #9bff00; }
.home-dark .dash-points-head { border-bottom-color: rgba(255,255,255,.08); }
.home-dark .dash-points-kicker,
.home-dark .dash-point-copy span,
.home-dark .dash-points-empty { color: #9a9a92; }
.home-dark .dash-points-total,
.home-dark .dash-point-copy strong { color: #fafaf7; }
.home-dark .dash-points-total small,
.home-dark .dash-points-link,
.home-dark .dash-point-change { color: #9bff00; }
.home-dark .dash-point-row { border-bottom-color: rgba(255,255,255,.07); }
.home-dark .dash-card-danger { border-color: rgba(255, 120, 117, 0.4); }
.home-dark .dash-card-pending { border-color: rgba(255, 190, 90, 0.4); }
.home-dark .dash-card-active, .home-dark .dash-card-successful { border-color: rgba(155, 255, 0, 0.3); }
.home-dark .dash-card-failed, .home-dark .dash-card-canceled { border-color: rgba(255, 120, 117, 0.4); }
.home-dark .dash-chip { background: #1f1f1f; border-color: rgba(255,255,255,0.1); }
.home-dark .dash-chip.chip-bad { background: #3a2222; border-color: rgba(255,120,117,0.5); }
.home-dark .dash-chip-main { color: #f0f0f0; }
.home-dark .dash-chip-sub { color: #9a9aa0; }
.home-dark .dash-iconbtn { background: #1f1f1f; border-color: rgba(255,255,255,0.1); color: #9a9aa0; }
.home-dark .dash-iconbtn:hover { color: #f0f0f0; border-color: #9bff00; }
.home-dark .dash-tab { background: #1f1f1f; border-color: rgba(255,255,255,0.1); color: #b8b8b0; }
.home-dark .dash-tab.is-on { background: #9bff00; color: #212121; border-color: #9bff00; }
.home-dark .dash-subtab { color: #9a9aa0; }
.home-dark .dash-subtab:hover, .home-dark .dash-subtab.is-on { color: #f0f0f0; }
.home-dark .dash-table th { color: #9a9aa0; background: #1c1c1c; border-bottom-color: rgba(255,255,255,0.08); }
.home-dark .dash-table td { border-bottom-color: rgba(255,255,255,0.06); }
.home-dark .dash-table tbody tr:hover { background: #1c1c1c; }
.home-dark .dash-investors-card { background: #161616; }
.home-dark .dash-investors-head,
.home-dark .dash-investors-footer { border-color: rgba(255,255,255,0.08); }
.home-dark .dash-investors-title,
.home-dark .dash-investor-wallet { color: #f0f0f0; }
.home-dark .dash-investor-row {
  background: #1c1c1c;
  border-color: rgba(255,255,255,0.08);
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.02);
}
.home-dark .dash-investor-row:hover { background: #222320; border-color: rgba(155,255,0,0.22); }
.home-dark .dash-investor-avatar { background: #2a2a2a; border-color: rgba(255,255,255,0.12); }
.home-dark .dash-investors-loading,
.home-dark .dash-investor-date,
.home-dark .dash-investors-total { color: #9a9aa0; }
.home-dark .dash-investors-page { color: #b8b8b0; }
.home-dark .dash-investor-amount { color: #9bff00; }
.home-dark .dash-investor-social.is-twitter { color: #7cb2ff; }
.home-dark .dash-investor-social.is-telegram { color: #66c7ed; }
.home-dark .dash-empty-text { color: #9a9aa0; }
.home-dark .dash-stat { background: #1c1c1c; border-color: rgba(255,255,255,0.07); }
.home-dark .dash-stat.stat-ok { background: #1f2b1e; border-color: rgba(155,255,0,0.3); }
.home-dark .dash-stat.stat-shortfall { background: #3a2222; border-color: rgba(255,120,117,0.4); }
.home-dark .dash-stat-label { color: #9a9aa0; }
.home-dark .dash-stat-value { color: #f0f0f0; }
.home-dark .dash-stat-value small { color: #9a9aa0; }
.home-dark .dash-progress-track { background: #2a2a2a; }
.home-dark .dash-progress-label { color: #9a9aa0; }
.home-dark .dash-price-chip { background: #1c1c1c; border-color: rgba(255,255,255,0.07); color: #b8b8b0; }
.home-dark .dash-done-strip { background: #2a2a2a; color: #9a9aa0; }
.home-dark .dash-deposit-panel { background: #1c1c1c; border-color: rgba(255,255,255,0.08); }
.home-dark .dash-deposit-title { color: #f0f0f0; }
.home-dark .dash-auto-amount { background: #161616; border-color: rgba(255,255,255,0.12); color: #9a9aa0; }
.home-dark .dash-auto-amount strong { color: #f0f0f0; }
.home-dark .dash-fill-btn { border-color: rgba(255,255,255,0.15); color: #b8b8b0; }
.home-dark .dash-fill-btn:hover:not(:disabled) { border-color: #9bff00; color: #f0f0f0; }
.home-dark .dash-input { background: #161616; border-color: rgba(255,255,255,0.12); color: #f0f0f0; }
.home-dark .dash-step { background: #161616; border-color: rgba(255,255,255,0.08); }
.home-dark .dash-step.step-success { background: #1f2b1e; border-color: rgba(155,255,0,0.3); }
.home-dark .dash-step.step-error { background: #3a2222; border-color: rgba(255,120,117,0.4); }
.home-dark .dash-step-title { color: #f0f0f0; }
.home-dark .dash-step-msg { color: #9bff00; }
.home-dark .dash-step-msg.is-err { color: #ffa39e; }
.home-dark .dash-step-msg.is-warn { color: #ffc069; }
.home-dark .dash-btn-soft { background: #2a2a2a; color: #f0f0f0; }
.home-dark .dash-badge.badge-bw, .home-dark .m-dash-badge.badge-bw { background: #2a2a2a; color: #c9c9c9; }
.home-dark .dash-badge.badge-purple, .home-dark .m-dash-badge.badge-purple { background: #2a1f3d; color: #c8a2ff; }
.home-dark .dash-badge.badge-amber, .home-dark .m-dash-badge.badge-amber { background: #3a2f1a; color: #ffc069; }
.home-dark .dash-badge.badge-teal, .home-dark .m-dash-badge.badge-teal { background: #14302e; color: #6fd8d2; }
.home-dark .dash-badge.badge-blue, .home-dark .m-dash-badge.badge-blue { background: #14233f; color: #7cb2ff; }
.home-dark .dash-badge.badge-green, .home-dark .m-dash-badge.badge-green { background: #1f2b1e; color: #9bff00; }
.home-dark .dash-badge.badge-red, .home-dark .m-dash-badge.badge-red { background: #3a2222; color: #ffa39e; }
.home-dark .dash-note.note-orange { background: #3a2f1a; border-color: rgba(255,190,90,0.4); color: #ffc069; }
.home-dark .dash-note.note-purple { background: #2a1f3d; border-color: rgba(200,162,255,0.35); color: #c8a2ff; }
.home-dark .dash-note.note-red { background: #3a2222; border-color: rgba(255,120,117,0.4); color: #ffa39e; }
.home-dark .dash-txlink { color: #9bff00; }
.home-dark .dash-pill.pill-green { background: #1f2b1e; color: #9bff00; }
.home-dark .dash-pill.pill-gray { background: #2a2a2a; color: #9a9aa0; }
.home-dark .dash-pill.pill-blue { background: #14233f; color: #7cb2ff; }
.home-dark .dash-pill.pill-red { background: #3a1d1d; color: #ff9c9c; }
.home-dark .dash-pill.pill-orange { background: #3a2c14; color: #ffc069; }
</style>
