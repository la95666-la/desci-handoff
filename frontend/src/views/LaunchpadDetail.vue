<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Globe, Twitter, Send, MessageCircle, ArrowLeft } from 'lucide-vue-next'
import CountdownTimer from '../components/CountdownTimer.vue'
import Modal from '../components/Modal.vue'
import { useWalletStore } from '../stores/wallet'
import { contractService } from '../services/contract'
import { ACTIVE_CHAIN } from '../config/contracts'
import { getProjectDetail } from '../services/api'
import { meetsMinPurchase } from '../utils/minPurchase.js'
import { sanitize } from '../utils/sanitize'
import { mockProjects } from '../mock/projects'
import { usesUnifiedProjectDetail } from '../utils/projectStatus'
import { resolveProjectFundingRoute } from '../utils/fairLaunch.js'
import { formatRaisedAmount } from '../utils/raisedAmount.js'
// Launchpad 全生命周期共用新版内容详情；仅 candidate 保留候选项目旧语义。
import LaunchpadDetailPreLaunch from './LaunchpadDetailPreLaunch.vue'

const props = defineProps({
  language: String
})

const route = useRoute()
const router = useRouter()
const walletStore = useWalletStore()

const project = ref(null)
const isUnifiedDetail = computed(() => usesUnifiedProjectDetail(
  project.value?.backendStatus || project.value?.status,
))
const activeTab = ref('overview')
const purchaseAmount = ref('')
const isPurchasing = ref(false)
const isApproving = ref(false)
const isLoading = ref(false)
const loadError = ref(null)
let projectRequest = 0

// 鏈上狀態
const paymentAllowance = ref(0n)
const paymentBalance = ref({ formatted: '0', raw: 0n })
const userPurchaseInfo = ref(null)  // 用戶已認購資訊

// Modal 狀態
const showModal = ref(false)
const modalConfig = ref({ type: 'success', title: '', message: '', txHash: '' })

async function loadProject() {
  const requestID = ++projectRequest
  const projectId = route.params.id
  isLoading.value = true
  loadError.value = null

  try {
    const mockProject = mockProjects.find(item => String(item.id) === String(projectId))
    if (mockProject) {
      project.value = mapMockDetail(mockProject)
    } else {
      const data = await getProjectDetail(projectId)
      if (requestID !== projectRequest) return
      const mapped = mapDetailData(data)
      const fundingRoute = resolveProjectFundingRoute(mapped)
      project.value = fundingRoute.kind === 'ido_v3'
        ? { ...mapped, onchainLocked: data.onchainLocked === true, ownerAddress: data.ownerAddress || data.projectOwner || '', feeReceiver: data.feeReceiver || '', id: String(projectId), fundingRoute }
        : { ...mapped, fundingRoute }
    }
  } catch (err) {
    if (requestID !== projectRequest) return
    console.error('Failed to load project detail:', err)
    loadError.value = err.message
    router.push('/launchpad')
  } finally {
    if (requestID === projectRequest) isLoading.value = false
  }
}

/**
 * 將後端詳情數據映射為前端格式
 */
function mapDetailData(data) {
  const now = Date.now()
  const startMs = data.startTime * 1000
  const endMs = data.endTime * 1000

  let status = (data.status || 'Pending')
  const backendStatus = status.toLowerCase()

  if (backendStatus === 'pending') {
    // Pending: 代幣未充足，不能進入 live
    status = now >= endMs ? 'closed' : 'coming-soon'
  } else if (backendStatus === 'active') {
    // Active: 代幣已充足，按時間判斷
    if (now < startMs) status = 'coming-soon'
    else if (now >= startMs && now < endMs) status = 'live'
    else status = 'closed'
  }

  return {
    id: route.params.id,
    name: data.name,
    nameCn: data.nameCn || data.name,
    logo: data.logo?.url || data.coverUrl || '/brand/joincare-avatar-green-black.png',
    banner: data.banner?.url || data.coverUrl || '',
    overview: data.description || '',
    overviewCn: data.description || '',
    tags: data.tags || [],
    tagsCn: data.tags || [],
    tokenSymbol: data.tokenSymbol || '',
    tokenAddress: data.tokenAddress || '',
    tokenDecimals: data.tokenDecimals || 18,
    tokenPrice: data.tokenPrice ?? '0',
    hardCap: data.hardCap ?? '0',
    softCap: data.softCap ?? '0',
    raised: data.fundingDataStatus === 'syncing' ? null : (data.totalRaised ?? null),
    totalRaised: data.fundingDataStatus === 'syncing' ? null : (data.totalRaised ?? null),
    fundingDataStatus: data.fundingDataStatus || '',
    saleTokenAmount: data.saleTokenAmount ?? '',
    saleTokenAmountRaw: data.saleTokenAmountRaw ?? '',
    maxPerUser: data.maxPerUser ?? '0',
    startTime: startMs,
    endTime: endMs,
    paymentToken: data.paymentToken || '',
    paymentTokenAddress: data.paymentTokenAddress || '',
    paymentTokenDecimals: Number(data.paymentTokenDecimals || 0),
    chainId: Number(data.chainId || 0),
    contractAddress: data.contractAddress || '',
    contractVersion: data.contractVersion || '',
    fundingMode: data.fundingMode || data.funding_mode || '',
    tokenPriceRaw: data.tokenPriceRaw || '',
    hardCapRaw: data.hardCapRaw || '',
    maxPerUserRaw: data.maxPerUserRaw || '',
    minPurchaseAmount: data.minPurchaseAmount || '',
    minPurchaseAmountRaw: data.minPurchaseAmountRaw || '',
    backendStatus,
    status,
    teamInfo: data.teamInfo || '',
    roadmap: data.roadmap || '',
    partners: data.partners || '',
    tagline: data.tagline || '',
    // 轉換 tgeList 為 vestingTimes/vestingPercents
    vestingTimes: (data.tgeList || []).map(t => t.tgeTime * 1000),
    vestingPercents: (data.tgeList || []).map(t => parseFloat(t.vestingPercent) * 100),
    tgeList: data.tgeList || [],
    logoBg: '#FFFFFF',
    socialLinks: {
      website: '',
      twitter: '',
      telegram: '',
      discord: ''
    }
  }
}

function mapMockDetail(data) {
  return {
    ...data,
    fundingMode: data.fundingMode || data.funding_mode || '',
    banner: data.banner || '',
    paymentToken: data.paymentToken || 'JCC',
    teamInfo: data.teamInfo || '',
    roadmap: data.roadmap || '',
    partners: data.partners || '',
    tagline: data.shortDescription || '',
    taglineCn: data.shortDescriptionCn || '',
    tgeList: data.tgeList || [],
    logoBg: data.logoBg || '#FFFFFF',
    socialLinks: data.socialLinks || {
      website: '',
      twitter: '',
      telegram: '',
      discord: ''
    }
  }
}

onMounted(async () => {
  await loadProject()

  // 錢包已連接時，加載鏈上狀態
  if (walletStore.isConnected && project.value?.status !== 'candidate' && !isUnifiedDetail.value && resolveProjectFundingRoute(project.value).kind !== 'ido_v3') {
    loadOnChainData()
  }
})

watch(() => props.language, () => {
  // Unified details own their localized fetch; only the legacy candidate view reloads here.
  if (!isUnifiedDetail.value) loadProject()
})
watch(() => route.params.id, loadProject)

// 監聽錢包連接狀態變化，自動加載鏈上數據
watch(() => walletStore.address, (newAddr) => {
  if (newAddr && project.value && project.value.status !== 'candidate' && !isUnifiedDetail.value && resolveProjectFundingRoute(project.value).kind !== 'ido_v3') {
    loadOnChainData()
  } else {
    paymentAllowance.value = 0n
    paymentBalance.value = { formatted: '0', raw: 0n }
    userPurchaseInfo.value = null
  }
})

/** 加載鏈上數據：Allowance + 用戶已認購資訊 */
async function loadOnChainData() {
  if (!walletStore.address || !project.value || resolveProjectFundingRoute(project.value).kind === 'ido_v3') return
  try {
    const [allowance, balance, purchase] = await Promise.all([
      contractService.getPaymentTokenAllowance(project.value, walletStore.address),
      contractService.getPaymentTokenBalance(project.value, walletStore.address),
      contractService.getUserPurchase(project.value, walletStore.address)
    ])
    paymentAllowance.value = allowance.raw
    paymentBalance.value = balance
    userPurchaseInfo.value = purchase
  } catch (err) {
    console.warn('Failed to load on-chain data:', err)
  }
}

const content = {
  en: {
    back: 'Back to Candidate Project',
    overview: 'Overview',
    vesting: 'Vesting Schedule',
    tokenomics: 'Tokenomics',
    price: 'Price',
    hardCap: 'Hard Cap',
    softCap: 'Soft Cap',
    maxPerUser: 'Max Per User',
    progress: 'Progress',
    startsIn: 'Starts in',
    endsIn: 'Ends in',
    tgeDate: 'TGE Date',
    yourBalance: 'Your Balance',
    yourInvested: 'You Invested',
    remaining: 'Remaining Quota',
    enterAmount: 'Enter payment amount',
    youWillGet: 'You will get',
    approveJcc: 'Approve Payment Token',
    approving: 'Approving...',
    purchase: 'Purchase',
    purchasing: 'Purchasing...',
    insufficientBalance: 'Insufficient Balance',
    buyJcc: 'Get Payment Token',
    connectWallet: 'Connect Wallet',
    purchaseSuccess: 'Purchase Successful!',
    purchaseSuccessMsg: 'Your tokens have been purchased successfully.',
    purchaseError: 'Purchase Failed',
    approveSuccess: 'Approval Successful!',
    approveSuccessMsg: 'Payment token approved. You can now proceed to purchase.',
    approveError: 'Approval Failed',
    candidate: 'Candidate',
    comingSoon: 'Coming Soon',
    closed: 'Closed',
    fundingPending: 'Funding details pending',
    noVestingSchedule: 'Release schedule pending.',
    vestingInfo: 'Vesting Schedule',
    vestingPhase: 'Phase',
    vestingTime: 'Unlock Time',
    vestingRatio: 'Ratio',
    vestingCumulative: 'Cumulative',
    vestingStatus: 'Status',
    vestingUnlocked: 'Unlocked',
    vestingLocked: 'Locked',
    vestingTge: 'TGE',
    vestingRound: 'Round',
    viewOnBscscan: 'View on BscScan',
    ok: 'OK',
    loading: 'Loading...',
    buyJccLink: 'Need payment tokens?',
    transaction: 'Transaction',
    tokenomicsComingSoon: 'Tokenomics information coming soon...',
    totalVestingPhases: 'Total Vesting Phases',
    phases: 'phases'
  },
  zh: {
    back: '返回候選項目',
    overview: '項目概覽',
    vesting: '釋放計劃',
    tokenomics: '代幣經濟',
    price: '價格',
    hardCap: '硬頂',
    softCap: '軟頂',
    maxPerUser: '單用戶上限',
    progress: '進度',
    startsIn: '開始倒計時',
    endsIn: '結束倒計時',
    tgeDate: 'TGE日期',
    yourBalance: '您的支付幣余額',
    yourInvested: '已投入',
    remaining: '剩余額度',
    enterAmount: '輸入支付數量',
    youWillGet: '您將獲得',
    approveJcc: '授權支付幣',
    approving: '授權中...',
    purchase: '立即認購',
    purchasing: '認購中...',
    insufficientBalance: '余額不足',
    buyJcc: '獲取支付幣',
    connectWallet: '連接錢包',
    purchaseSuccess: '認購成功！',
    purchaseSuccessMsg: '您已成功認購項目代幣。',
    purchaseError: '認購失敗',
    approveSuccess: '授權成功！',
    approveSuccessMsg: '支付幣已授權，您可以繼續認購。',
    approveError: '授權失敗',
    candidate: '候選',
    comingSoon: '即將開始',
    closed: '已結束',
    fundingPending: '融資資訊待公布',
    noVestingSchedule: '釋放計劃待公布。',
    vestingInfo: '釋放計劃',
    vestingPhase: '階段',
    vestingTime: '解鎖時間',
    vestingRatio: '比例',
    vestingCumulative: '累計',
    vestingStatus: '狀態',
    vestingUnlocked: '已解鎖',
    vestingLocked: '未解鎖',
    vestingTge: 'TGE',
    vestingRound: '第{n}次釋放',
    viewOnBscscan: '在BscScan查看',
    ok: '確定',
    loading: '加載中...',
    buyJccLink: '需要支付幣？',
    transaction: '交易',
    tokenomicsComingSoon: '代幣經濟資訊即將推出...',
    totalVestingPhases: '總釋放階段',
    phases: '期'
  }
}
// App 当前使用 tw，兼容旧存储值 zh-TW / zh-CN；旧详情模板历史上只定义了 zh；
// 加载占位和兜底模板必须共用繁体文案，避免状态切换时读取 undefined。
content.tw = content.zh
content['zh-TW'] = content.zh
content['zh-CN'] = content.zh

const projectName = computed(() => {
  if (!project.value) return ''
  return props.language === 'en'
    ? project.value.name
    : project.value.nameCn || project.value.name
})

/** 生成 Vesting 時間表 */
const vestingSchedule = computed(() => {
  if (!project.value?.vestingTimes || !project.value?.vestingPercents) return []
  const now = Date.now()
  let cumulative = 0
  return project.value.vestingTimes.map((time, i) => {
    const percent = project.value.vestingPercents[i] / 100 // 5000 → 50
    cumulative += percent
    const label = i === 0
      ? content[props.language].vestingTge
      : props.language === 'en'
        ? `${content[props.language].vestingRound} ${i}`
        : content[props.language].vestingRound.replace('{n}', i)
    return {
      label,
      time,
      percent,
      cumulative: Math.round(cumulative),
      unlocked: now >= time
    }
  })
})

const projectTags = computed(() => {
  if (!project.value) return []
  return props.language === 'en'
    ? project.value.tags || []
    : project.value.tagsCn || project.value.tags || []
})

const projectOverview = computed(() => {
  if (!project.value) return ''
  return props.language === 'en'
    ? project.value.overview || ''
    : project.value.overviewCn || project.value.overview || ''
})

const projectTagline = computed(() => {
  if (!project.value) return ''
  return props.language === 'en'
    ? project.value.tagline || ''
    : project.value.taglineCn || project.value.tagline || ''
})

const hasFundingInfo = computed(() => {
  return project.value?.tokenSymbol &&
    project.value.tokenPrice !== null &&
    project.value.tokenPrice !== undefined &&
    project.value.hardCap !== null &&
    project.value.hardCap !== undefined &&
    project.value.maxPerUser !== null &&
    project.value.maxPerUser !== undefined
})

const statusConfig = computed(() => {
  if (!project.value) return null
  
  const now = Date.now()
  const { status, startTime, endTime } = project.value

  if (status === 'candidate') {
    return {
      label: content[props.language].candidate,
      color: 'bg-gray-500',
      canPurchase: false,
      countdownTarget: null,
      countdownLabel: ''
    }
  } else if (status === 'coming-soon' || (startTime && now < startTime)) {
    return {
      label: content[props.language].comingSoon,
      color: 'bg-gray-500',
      canPurchase: false,
      countdownTarget: startTime,
      countdownLabel: content[props.language].startsIn
    }
  } else if (status === 'live' && startTime && endTime && now >= startTime && now < endTime) {
    return {
      label: 'Live',
      color: 'bg-green-500',
      canPurchase: true,
      countdownTarget: endTime,
      countdownLabel: content[props.language].endsIn
    }
  } else {
    return {
      label: content[props.language].closed,
      color: 'bg-gray-400',
      canPurchase: false,
      countdownTarget: null,
      countdownLabel: ''
    }
  }
})

const tokensToReceive = computed(() => {
  if (!purchaseAmount.value || !project.value) return 0
  return parseFloat(purchaseAmount.value) / project.value.tokenPrice
})

const progressPercent = computed(() => {
  if (!project.value?.hardCap) return 0
  return Math.min((project.value.raised / project.value.hardCap) * 100, 100)
})

const statusBadgeClass = computed(() => {
  if (!statusConfig.value) return ''
  if (statusConfig.value.canPurchase) return 'bg-[#9bff00] text-[#050b00]'
  if (project.value?.status === 'candidate') return 'bg-[#eef1ec] text-[#050b00] dark:bg-gray-800 dark:text-gray-200'
  if (project.value?.status === 'coming-soon') return 'bg-[#ffd665] text-[#050b00]'
  return 'bg-[#f0efec] text-[#050b00] dark:bg-gray-800 dark:text-gray-200'
})

/** 用戶已投入的項目支付幣 */
const userInvested = computed(() => {
  if (!userPurchaseInfo.value) return 0
  return parseFloat(userPurchaseInfo.value.paymentAmount || userPurchaseInfo.value.totalJCCSpent || '0')
})

/** 用戶剩余可買額度 */
const remainingQuota = computed(() => {
  if (!project.value) return 0
  return project.value.maxPerUser - userInvested.value
})

/** 是否需要先 Approve */
const needsApproval = computed(() => {
  if (!purchaseAmount.value || parseFloat(purchaseAmount.value) <= 0) return false
  try {
    const raw = contractService.parsePaymentAmount(project.value, purchaseAmount.value)
    return paymentAllowance.value < raw
  } catch {
    return false
  }
})

/** 是否可以認購 */
const canPurchase = computed(() => {
  if (!walletStore.isConnected || !statusConfig.value?.canPurchase) return false
  if (!purchaseAmount.value || parseFloat(purchaseAmount.value) <= 0) return false
  try {
    const amountRaw = contractService.parsePaymentAmount(project.value, purchaseAmount.value)
    const investedRaw = userPurchaseInfo.value?.paymentAmountRaw || userPurchaseInfo.value?.totalJCCSpentRaw || 0n
    const maxRaw = BigInt(project.value.maxPerUserRaw || 0)
    return meetsMinPurchase(amountRaw, project.value) && amountRaw <= paymentBalance.value.raw && amountRaw + investedRaw <= maxRaw
  } catch {
    return false
  }
})
const belowMinPurchase = computed(() => {
  try {
    const raw = contractService.parsePaymentAmount(project.value, purchaseAmount.value)
    return raw > 0n && !meetsMinPurchase(raw, project.value)
  } catch { return false }
})

/** 將合約錯誤轉換為友好提示 */
function friendlyError(error, lang) {
  const msg = error.reason || error.message || ''
  if (error.code === 'TRANSACTION_BROADCAST_UNCERTAIN') {
    return lang === 'en'
      ? 'The wallet did not return a valid transaction hash. Check wallet activity and on-chain records before trying again.'
      : '錢包未返回有效交易哈希，廣播狀態不明。請先檢查錢包活動與鏈上記錄，確認無交易後再重試。'
  }
  if (error.code === 'TRANSACTION_RECEIPT_TIMEOUT') {
    return lang === 'en'
      ? 'The transaction has a hash, but receipt confirmation timed out. Check the explorer before submitting again.'
      : '交易已取得哈希，但回執確認超時。請先在區塊瀏覽器確認狀態，切勿立即重複提交。'
  }
  if (error.code === 'TRANSACTION_REVERTED') {
    return lang === 'en'
      ? 'The transaction was mined but reverted. The purchase did not complete; inspect it in the explorer before proceeding.'
      : '交易已上鏈但執行失敗，資產未完成認購。請在區塊瀏覽器查看失敗交易後再處理。'
  }
  if (msg.includes('user rejected') || msg.includes('ACTION_REJECTED') || error.code === 'ACTION_REJECTED') {
    return lang === 'en' ? 'Transaction was cancelled by user.' : '用戶取消了交易。'
  }
  if (msg.includes('insufficient funds') || msg.includes('INSUFFICIENT_FUNDS')) {
    return lang === 'en' ? 'Insufficient balance to complete this transaction.' : '余額不足，無法完成交易。'
  }
  if (msg.includes('BUFFER_OVERRUN') || msg.includes('UNCONFIGURED_NAME') || msg.includes('could not decode result data')) {
    return lang === 'en' ? 'Contract not available. Please check your network and try again.' : '合約不可用，請檢查網路後重試。'
  }
  if (msg.includes('execution reverted') || msg.includes('CALL_EXCEPTION')) {
    const reason = error.reason || ''
    return lang === 'en' ? `Transaction failed: ${reason || 'Contract rejected the operation.'}` : `交易失敗：${reason || '合約拒絕了此操作。'}`
  }
  if (msg.length > 100) {
    return lang === 'en' ? 'Transaction failed. Please try again later.' : '交易失敗，請稍後重試。'
  }
  return msg || (lang === 'en' ? 'Unknown error occurred.' : '發生未知錯誤。')
}

/** 處理 Approve */
const handleApprove = async () => {
  if (resolveProjectFundingRoute(project.value).kind === 'ido_v3') return
  if (isApproving.value) return
  isApproving.value = true

  try {
    const result = await contractService.approvePaymentToken(project.value, purchaseAmount.value)
    
    // 刷新 Allowance
    const allowance = await contractService.getPaymentTokenAllowance(project.value, walletStore.address)
    paymentAllowance.value = allowance.raw

    modalConfig.value = {
      type: 'success',
      title: content[props.language].approveSuccess,
      message: content[props.language].approveSuccessMsg,
      txHash: result.hash
    }
    showModal.value = true
  } catch (error) {
    console.error('Approve failed:', error)
    modalConfig.value = {
      type: 'error',
      title: content[props.language].approveError,
      message: friendlyError(error, props.language),
      txHash: error?.transactionHash || ''
    }
    showModal.value = true
  } finally {
    isApproving.value = false
  }
}

/** 處理 Purchase */
const handlePurchase = async () => {
  if (resolveProjectFundingRoute(project.value).kind === 'ido_v3') return
  if (!canPurchase.value || isPurchasing.value || needsApproval.value) return
  isPurchasing.value = true

  try {
    const latest = await getProjectDetail(project.value.id)
    project.value.minPurchaseAmountRaw = latest.minPurchaseAmountRaw
    project.value.minPurchaseAmount = latest.minPurchaseAmount
    if (!meetsMinPurchase(contractService.parsePaymentAmount(project.value, purchaseAmount.value), latest)) {
      throw new Error(props.language === 'en' ? 'Below official-page minimum per purchase' : '低於官方頁面每筆最低參與額')
    }
    const result = await contractService.purchase(project.value, purchaseAmount.value)

    // 刷新鏈上數據
    await loadOnChainData()
    
    // 刷新錢包余額
    if (walletStore.address) {
      await walletStore.refreshBalances()
    }

    modalConfig.value = {
      type: 'success',
      title: content[props.language].purchaseSuccess,
      message: content[props.language].purchaseSuccessMsg,
      txHash: result.hash
    }
    showModal.value = true
    purchaseAmount.value = ''
  } catch (error) {
    console.error('Purchase failed:', error)
    modalConfig.value = {
      type: 'error',
      title: content[props.language].purchaseError,
      message: friendlyError(error, props.language),
      txHash: error?.transactionHash || ''
    }
    showModal.value = true
  } finally {
    isPurchasing.value = false
  }
}

const goBack = () => {
  router.push('/launchpad')
}
</script>

<template>
  <!-- Launchpad 全生命周期共用同一套新版内容详情；右栏按阶段展示认购/结果/退款。 -->
  <LaunchpadDetailPreLaunch
    v-if="!isLoading && project && (isUnifiedDetail || resolveProjectFundingRoute(project).kind === 'ido_v3')"
    :project="project"
    :language="language"
  />

  <main v-else-if="isLoading" class="min-h-screen bg-white pt-28 text-gray-950 dark:bg-gray-950 dark:text-white">
    <div class="mx-auto flex min-h-[420px] max-w-[1200px] items-center justify-center px-5">
      <div class="text-center text-[#5f6659] dark:text-gray-400">
        <div class="mx-auto mb-4 h-8 w-8 animate-spin rounded-full border-2 border-gray-200 border-t-[#050b00] dark:border-gray-800 dark:border-t-white"></div>
        <p class="text-lg">{{ content[language].loading }}</p>
      </div>
    </div>
  </main>

  <main v-else-if="project" class="detail-page min-h-screen bg-white pt-28 text-[#050b00] dark:bg-gray-950 dark:text-white">
    <div class="mx-auto max-w-[1200px] px-5 py-8 sm:px-6 lg:px-0">
      <button
        @click="goBack"
        class="mb-10 inline-flex items-center gap-2 text-sm text-[#8a9188] transition-colors hover:text-[#050b00] dark:text-gray-400 dark:hover:text-white"
      >
        <ArrowLeft :size="18" />
        <span>{{ content[language].back }}</span>
      </button>

      <section class="mb-12 rounded-[10px] border border-gray-200 bg-[#f6f6f6] p-6 dark:border-gray-800 dark:bg-gray-900">
        <div class="flex flex-col gap-6 md:flex-row md:items-center">
          <img :src="project.logo" :alt="projectName" class="h-[130px] w-[130px] rounded-[10px] bg-[#8f8f8f] object-cover" />
          <div class="min-w-0 flex-1">
            <h1 class="truncate text-[24px] sm:text-[32px] font-medium leading-tight tracking-normal text-[#050b00] dark:text-white">
              {{ projectName }}
            </h1>
            <div class="mt-3 flex flex-wrap items-center gap-2">
              <span
                v-for="(tag, index) in projectTags"
                :key="index"
                class="inline-flex h-5 items-center rounded-full bg-[#f3ffe4] px-3 text-xs font-normal text-[#3d5a00] dark:bg-lime-500/15 dark:text-lime-300"
              >
                {{ tag }}
              </span>
              <span :class="['inline-flex h-5 items-center rounded-full px-3 text-xs font-normal', statusBadgeClass]">
                {{ statusConfig.label }}
              </span>
            </div>
            <p v-if="projectTagline" class="mt-5 max-w-5xl text-base leading-[1.6] text-[#5f6659] dark:text-gray-400">
              {{ projectTagline }}
            </p>

            <div class="mt-5 flex flex-wrap items-center gap-3">
              <a
                v-if="project.socialLinks.website"
                :href="project.socialLinks.website"
                target="_blank"
                class="flex h-9 w-9 items-center justify-center rounded-full border border-gray-200 bg-white text-[#5f6659] transition-colors hover:border-[#050b00] hover:text-[#050b00] dark:border-gray-700 dark:bg-gray-950 dark:text-gray-400 dark:hover:border-gray-500 dark:hover:text-white"
              >
                <Globe :size="16" />
              </a>
              <a
                v-if="project.socialLinks.twitter"
                :href="project.socialLinks.twitter"
                target="_blank"
                class="flex h-9 w-9 items-center justify-center rounded-full border border-gray-200 bg-white text-[#5f6659] transition-colors hover:border-[#050b00] hover:text-[#050b00] dark:border-gray-700 dark:bg-gray-950 dark:text-gray-400 dark:hover:border-gray-500 dark:hover:text-white"
              >
                <Twitter :size="16" />
              </a>
              <a
                v-if="project.socialLinks.telegram"
                :href="project.socialLinks.telegram"
                target="_blank"
                class="flex h-9 w-9 items-center justify-center rounded-full border border-gray-200 bg-white text-[#5f6659] transition-colors hover:border-[#050b00] hover:text-[#050b00] dark:border-gray-700 dark:bg-gray-950 dark:text-gray-400 dark:hover:border-gray-500 dark:hover:text-white"
              >
                <Send :size="16" />
              </a>
              <a
                v-if="project.socialLinks.discord"
                :href="project.socialLinks.discord"
                target="_blank"
                class="flex h-9 w-9 items-center justify-center rounded-full border border-gray-200 bg-white text-[#5f6659] transition-colors hover:border-[#050b00] hover:text-[#050b00] dark:border-gray-700 dark:bg-gray-950 dark:text-gray-400 dark:hover:border-gray-500 dark:hover:text-white"
              >
                <MessageCircle :size="16" />
              </a>
            </div>
          </div>
        </div>
      </section>

      <div class="grid gap-12 lg:grid-cols-[minmax(0,1fr)_360px]">
        <section class="min-w-0">
          <div class="border-b border-gray-200 dark:border-gray-800 overflow-x-auto -mx-5 sm:mx-0">
            <div class="flex gap-0 whitespace-nowrap px-5 sm:px-0 sm:flex-wrap">
              <button
                @click="activeTab = 'overview'"
                :class="[
                  'border-b-2 px-5 py-4 text-base font-normal transition-colors',
                  activeTab === 'overview'
                    ? 'border-[#9bff00] text-[#050b00] dark:text-white'
                    : 'border-transparent text-[#8a9188] hover:text-[#050b00] dark:text-gray-400 dark:hover:text-white'
                ]"
              >
                {{ content[language].overview }}
              </button>
              <button
                @click="activeTab = 'vesting'"
                :class="[
                  'border-b-2 px-5 py-4 text-base font-normal transition-colors',
                  activeTab === 'vesting'
                    ? 'border-[#9bff00] text-[#050b00] dark:text-white'
                    : 'border-transparent text-[#8a9188] hover:text-[#050b00] dark:text-gray-400 dark:hover:text-white'
                ]"
              >
                {{ content[language].vesting }}
              </button>
              <button
                @click="activeTab = 'tokenomics'"
                :class="[
                  'border-b-2 px-5 py-4 text-base font-normal transition-colors',
                  activeTab === 'tokenomics'
                    ? 'border-[#9bff00] text-[#050b00] dark:text-white'
                    : 'border-transparent text-[#8a9188] hover:text-[#050b00] dark:text-gray-400 dark:hover:text-white'
                ]"
              >
                {{ content[language].tokenomics }}
              </button>
            </div>
          </div>

          <div class="min-h-[220px] py-12">
            <div v-if="activeTab === 'overview'" class="detail-rich-text max-w-none text-base leading-7 text-[#5f6659] dark:text-gray-300">
              <div v-html="sanitize(projectOverview)"></div>
            </div>

            <div v-else-if="activeTab === 'vesting'" class="space-y-6">
              <h3 class="text-lg font-normal text-[#050b00] dark:text-white">
                {{ content[language].vestingInfo }}
              </h3>

              <div v-if="vestingSchedule.length" class="overflow-x-auto rounded-[10px] border border-gray-200 dark:border-gray-800">
                <table class="w-full min-w-[760px] text-sm">
                  <thead class="bg-[#f6f6f6] dark:bg-gray-900">
                    <tr>
                      <th class="px-4 py-3 text-left text-xs font-normal text-[#8a9188] dark:text-gray-400">
                        {{ content[language].vestingPhase }}
                      </th>
                      <th class="px-4 py-3 text-left text-xs font-normal text-[#8a9188] dark:text-gray-400">
                        {{ content[language].vestingTime }}
                      </th>
                      <th class="px-4 py-3 text-center text-xs font-normal text-[#8a9188] dark:text-gray-400">
                        {{ content[language].vestingRatio }}
                      </th>
                      <th class="px-4 py-3 text-center text-xs font-normal text-[#8a9188] dark:text-gray-400">
                        {{ content[language].vestingCumulative }}
                      </th>
                      <th class="px-4 py-3 text-center text-xs font-normal text-[#8a9188] dark:text-gray-400">
                        {{ content[language].vestingStatus }}
                      </th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 bg-white dark:divide-gray-800 dark:bg-gray-950">
                    <tr v-for="(item, index) in vestingSchedule" :key="index">
                      <td class="px-4 py-4">
                        <div class="flex items-center gap-3">
                          <span
                            class="flex h-8 w-8 items-center justify-center rounded-full text-xs font-medium"
                            :class="item.unlocked
                              ? 'bg-[#f3ffe4] text-[#3d5a00] dark:bg-lime-500/15 dark:text-lime-300'
                              : 'bg-[#f0efec] text-[#8a9188] dark:bg-gray-800 dark:text-gray-400'"
                          >
                            {{ index + 1 }}
                          </span>
                          <span class="font-medium text-[#050b00] dark:text-white">{{ item.label }}</span>
                        </div>
                      </td>
                      <td class="px-4 py-4 text-[#5f6659] dark:text-gray-300">
                        {{ new Date(item.time).toLocaleString() }}
                      </td>
                      <td class="px-4 py-4 text-center text-[#050b00] dark:text-white">
                        {{ item.percent }}%
                      </td>
                      <td class="px-4 py-4 text-center">
                        <div class="mx-auto flex max-w-[120px] flex-col items-center gap-2">
                          <span class="text-[#050b00] dark:text-white">{{ item.cumulative }}%</span>
                          <div class="h-1.5 w-full overflow-hidden rounded-full bg-[#050b00]/15 dark:bg-white/15">
                            <div class="h-full rounded-full bg-[#9bff00]" :style="{ width: item.cumulative + '%' }"></div>
                          </div>
                        </div>
                      </td>
                      <td class="px-4 py-4 text-center">
                        <span
                          class="inline-flex h-6 items-center rounded-full px-3 text-xs font-normal"
                          :class="item.unlocked
                            ? 'bg-[#f3ffe4] text-[#3d5a00] dark:bg-lime-500/15 dark:text-lime-300'
                            : 'bg-[#f0efec] text-[#8a9188] dark:bg-gray-800 dark:text-gray-400'"
                        >
                          {{ item.unlocked ? content[language].vestingUnlocked : content[language].vestingLocked }}
                        </span>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>

              <div v-else class="rounded-[10px] border border-gray-200 bg-[#f6f6f6] p-4 text-sm text-[#5f6659] dark:border-gray-800 dark:bg-gray-900 dark:text-gray-400">
                {{ content[language].noVestingSchedule }}
              </div>

              <div v-if="vestingSchedule.length" class="flex items-center justify-between rounded-[10px] border border-gray-200 bg-[#f6f6f6] p-4 dark:border-gray-800 dark:bg-gray-900">
                <span class="text-sm text-[#5f6659] dark:text-gray-400">{{ content[language].totalVestingPhases }}</span>
                <span class="font-medium text-[#050b00] dark:text-white">{{ vestingSchedule.length }} {{ content[language].phases }}</span>
              </div>
            </div>

            <div v-else-if="activeTab === 'tokenomics'" class="py-16 text-center text-[#8a9188] dark:text-gray-400">
              {{ content[language].tokenomicsComingSoon }}
            </div>
          </div>
        </section>

        <aside class="lg:pt-0">
          <div class="sticky top-24 rounded-[10px] border border-gray-200 bg-[#fdfdfd] p-6 dark:border-gray-800 dark:bg-gray-950">
            <div class="mb-7 flex items-center gap-3">
              <img :src="project.logo" :alt="projectName" class="h-[52px] w-[52px] rounded-full bg-[#c4c4c4] object-cover" />
              <h2 class="truncate text-base font-medium text-[#050b00] dark:text-white">{{ projectName }}</h2>
            </div>

            <div v-if="statusConfig.countdownTarget" class="mb-6 rounded-[10px] border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-gray-900">
              <div class="mb-2 text-sm text-[#8a9188] dark:text-gray-400">
                {{ statusConfig.countdownLabel }}
              </div>
              <CountdownTimer :target-time="statusConfig.countdownTarget" :language="language" />
            </div>

            <div v-if="hasFundingInfo" class="space-y-4">
              <div class="flex items-center justify-between gap-4 text-sm">
                <span class="text-[#8a9188] dark:text-gray-400">{{ content[language].price }}</span>
                <span class="text-right font-normal text-[#050b00] dark:text-white">
                  1 {{ project.tokenSymbol }} = {{ project.tokenPrice }} {{ project.paymentToken }}
                </span>
              </div>
              <div class="flex items-center justify-between gap-4 text-sm">
                <span class="text-[#8a9188] dark:text-gray-400">{{ content[language].hardCap }}</span>
                <span class="text-right font-normal text-[#050b00] dark:text-white">
                  {{ project.hardCap.toLocaleString() }} {{ project.paymentToken }}
                </span>
              </div>
              <div class="flex items-center justify-between gap-4 text-sm">
                <span class="text-[#8a9188] dark:text-gray-400">{{ content[language].maxPerUser }}</span>
                <span class="text-right font-normal text-[#050b00] dark:text-white">
                  {{ project.maxPerUser.toLocaleString() }} {{ project.paymentToken }}
                </span>
              </div>
              <div v-if="project.minPurchaseAmountRaw" class="flex items-center justify-between gap-4 text-sm">
                <span class="text-[#8a9188] dark:text-gray-400">{{ language === 'en' ? 'Minimum per purchase' : '每筆最低參與額' }}</span>
                <span>{{ project.minPurchaseAmount }} {{ project.paymentToken }}</span>
              </div>
              <div>
                <div class="mb-2 flex items-center justify-between gap-4 text-sm">
                  <span class="text-[#8a9188] dark:text-gray-400">{{ content[language].progress }}</span>
                  <span class="text-right font-normal text-[#050b00] dark:text-white">
                    {{ formatRaisedAmount(project.raised, project.paymentToken) }} / {{ project.hardCap.toLocaleString() }} {{ project.paymentToken }}
                  </span>
                </div>
                <div class="h-1.5 overflow-hidden rounded-full bg-[#050b00]/15 dark:bg-white/15">
                  <div class="h-full rounded-full bg-[#9bff00] transition-all duration-500" :style="{ width: `${progressPercent}%` }"></div>
                </div>
              </div>
            </div>

            <div v-else class="rounded-[10px] border border-gray-200 bg-[#f6f6f6] p-4 text-center text-sm text-[#5f6659] dark:border-gray-800 dark:bg-gray-900 dark:text-gray-400">
              {{ content[language].fundingPending }}
            </div>

            <div v-if="hasFundingInfo && statusConfig.canPurchase" class="mt-7 space-y-4 border-t border-gray-200 pt-6 dark:border-gray-800">
              <div v-if="walletStore.isConnected" class="space-y-3">
                <div class="flex justify-between gap-4 text-sm">
                  <span class="text-[#8a9188] dark:text-gray-400">
                    {{ content[language].yourBalance }}
                    <a href="https://pancakeswap.finance" target="_blank" rel="noopener noreferrer" class="ml-2 text-[#3d5a00] underline-offset-2 hover:underline dark:text-lime-300">
                      {{ content[language].buyJccLink }}
                    </a>
                  </span>
                  <span class="text-right font-normal text-[#050b00] dark:text-white">
                    {{ parseFloat(paymentBalance.formatted).toLocaleString() }} {{ project.paymentToken }}
                  </span>
                </div>
                <div v-if="userPurchaseInfo && userInvested > 0" class="flex justify-between gap-4 text-sm">
                  <span class="text-[#8a9188] dark:text-gray-400">{{ content[language].yourInvested }}</span>
                  <span class="text-right font-normal text-[#050b00] dark:text-white">
                    {{ userInvested.toLocaleString() }} {{ project.paymentToken }}
                  </span>
                </div>
                <div v-if="userPurchaseInfo" class="flex justify-between gap-4 text-sm">
                  <span class="text-[#8a9188] dark:text-gray-400">{{ content[language].remaining }}</span>
                  <span class="text-right font-normal text-[#050b00] dark:text-white">
                    {{ remainingQuota.toLocaleString() }} {{ project.paymentToken }}
                  </span>
                </div>
              </div>

              <input
                v-model="purchaseAmount"
                type="number"
                :placeholder="content[language].enterAmount"
                :disabled="!walletStore.isConnected"
                class="h-12 w-full rounded-[10px] border border-gray-200 bg-white px-4 text-sm text-[#050b00] outline-none transition-colors placeholder:text-[#8a9188] focus:border-[#050b00] disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:bg-gray-900 dark:text-white dark:focus:border-gray-500"
              />
              <p v-if="belowMinPurchase" class="text-sm text-red-600">{{ language === 'en' ? 'Below official-page minimum per purchase' : '低於官方頁面每筆最低參與額' }}</p>

              <div v-if="tokensToReceive > 0" class="rounded-[10px] border border-[#d8ffb0] bg-[#fafff5] p-4 dark:border-lime-500/30 dark:bg-lime-950/10">
                <div class="mb-1 text-sm text-[#5f6659] dark:text-gray-400">{{ content[language].youWillGet }}</div>
                <div class="text-2xl font-medium text-[#050b00] dark:text-white">
                  {{ tokensToReceive.toLocaleString() }} {{ project.tokenSymbol }}
                </div>
              </div>

              <button
                v-if="!walletStore.isConnected"
                @click="$emit('connect-wallet')"
                class="h-12 w-full rounded-full bg-[#9bff00] px-6 text-sm font-medium text-[#050b00] transition-colors hover:bg-[#8bed00]"
              >
                {{ content[language].connectWallet }}
              </button>
              <button
                v-else-if="parseFloat(purchaseAmount) > parseFloat(paymentBalance.formatted)"
                @click="window.open('https://pancakeswap.finance', '_blank')"
                class="h-12 w-full rounded-full border border-gray-200 bg-white px-6 text-sm font-medium text-[#050b00] transition-colors hover:border-[#050b00] dark:border-gray-700 dark:bg-gray-950 dark:text-white dark:hover:border-gray-500"
              >
                {{ content[language].buyJcc }}
              </button>
              <button
                v-else-if="needsApproval"
                @click="handleApprove"
                :disabled="isApproving || !canPurchase"
                :class="[
                  'h-12 w-full rounded-full px-6 text-sm font-medium transition-colors',
                  !isApproving && canPurchase
                    ? 'bg-[#9bff00] text-[#050b00] hover:bg-[#8bed00]'
                    : 'cursor-not-allowed bg-[#f0efec] text-[#8a9188] dark:bg-gray-800 dark:text-gray-500'
                ]"
              >
                {{ isApproving ? content[language].approving : content[language].approveJcc }}
              </button>
              <button
                v-else
                @click="handlePurchase"
                :disabled="!canPurchase || isPurchasing"
                :class="[
                  'h-12 w-full rounded-full px-6 text-sm font-medium transition-colors',
                  canPurchase && !isPurchasing
                    ? 'bg-[#9bff00] text-[#050b00] hover:bg-[#8bed00]'
                    : 'cursor-not-allowed bg-[#f0efec] text-[#8a9188] dark:bg-gray-800 dark:text-gray-500'
                ]"
              >
                {{ isPurchasing ? content[language].purchasing : content[language].purchase }}
              </button>
            </div>

            <div v-else class="mt-7 border-t border-gray-200 pt-6 text-center text-sm text-[#8a9188] dark:border-gray-800 dark:text-gray-400">
              {{ statusConfig.label }}
            </div>
          </div>
        </aside>
      </div>
    </div>

    <Modal
      :show="showModal"
      :type="modalConfig.type"
      :title="modalConfig.title"
      :message="modalConfig.message"
      :confirmText="content[language].ok"
      @close="showModal = false"
    >
      <div v-if="modalConfig.txHash" class="text-center">
        <div class="mb-4 rounded-[10px] border border-gray-200 bg-[#f6f6f6] p-4 dark:border-gray-800 dark:bg-gray-900">
          <p class="mb-1 text-xs text-[#8a9188] dark:text-gray-400">{{ content[language].transaction }}</p>
          <p class="break-all font-mono text-sm text-[#050b00] dark:text-white">
            {{ modalConfig.txHash }}
          </p>
        </div>
        <a
          :href="contractService.getTxUrl(modalConfig.txHash)"
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-2 text-sm font-medium text-[#050b00] underline-offset-2 hover:underline dark:text-white"
        >
          <span>{{ content[language].viewOnBscscan }}</span>
        </a>
      </div>
    </Modal>
  </main>
</template>

<style scoped>
.detail-rich-text :deep(p) {
  margin: 0 0 1em;
}

.detail-rich-text :deep(a) {
  color: #050b00;
  text-decoration: underline;
  text-underline-offset: 3px;
}

:global(.dark) .detail-rich-text :deep(a) {
  color: #ffffff;
}
</style>
