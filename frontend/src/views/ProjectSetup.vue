<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useWalletStore } from '../stores/wallet'
import { ArrowLeft, Upload, Plus, X, AlertCircle, Calculator, Clock, Coins, Info, CheckCircle, Trash2 } from 'lucide-vue-next'
import { getProjectDetail, getSignedUploadUrl, submitProjectReview } from '../services/api'

const props = defineProps({
  language: String
})

const router = useRouter()
const route = useRoute()
const walletStore = useWalletStore()

const projectId = computed(() => route.params.id)

const content = {
  en: {
    title: 'Submit Token Information',
    subtitle: 'Configure your project\'s token parameters and fundraising details',
    back: 'Back to Dashboard',
    // Basic Display Info
    sectionBasic: 'Project Display Information',
    projectNameCn: 'Project Name (Traditional Chinese)',
    projectNameCnPlaceholder: 'Traditional Chinese project name',
    projectNameEn: 'Project Name (English)',
    projectNameEnPlaceholder: 'English project name',
    logo: 'Project Logo',
    banner: 'Project Banner',
    uploadImage: 'Upload Image',
    uploading: 'Uploading...',
    tagline: 'One-line Introduction',
    taglinePlaceholder: 'One sentence describing your project',
    description: 'Project Description (max 150 characters)',
    descriptionPlaceholder: 'Detailed project description...',
    tags: 'Project Tags',
    tagsPlaceholder: 'Add a tag...',
    addTag: 'Add',
    teamInfo: 'Team Information',
    teamInfoPlaceholder: 'Describe your team background and members...',
    roadmap: 'Project Roadmap',
    roadmapPlaceholder: 'Describe your project development roadmap...',
    partners: 'Partners',
    partnersPlaceholder: 'List your partners and collaborators...',
    // Token & Fundraising Params
    sectionToken: 'Token & Fundraising Parameters',
    tokenAddress: 'Token Contract Address',
    tokenAddressPlaceholder: '0x... (BNB Chain ERC20 token)',
    tokenAddressHint: 'Only BNB Chain ERC20 tokens with 18 decimals are supported',
    tokenSymbol: 'Token Symbol',
    tokenSymbolPlaceholder: 'e.g., ABC',
    tokenDecimals: 'Token Decimals',
    tokenDecimalsHint: 'Fixed at 18 decimals',
    tokenPrice: 'Token Price',
    tokenPricePlaceholder: '1 token = ?',
    hardCap: 'Hard Cap',
    hardCapPlaceholder: 'Maximum fundraising amount',
    softCap: 'Soft Cap',
    softCapPlaceholder: 'Minimum fundraising amount',
    maxPerUser: 'Max Per User',
    maxPerUserPlaceholder: 'Per user subscription limit',
    paymentToken: 'Fundraising Currency',
    paymentTokenHint: 'Select the currency for fundraising',
    startTime: 'Start Time',
    endTime: 'End Time',
    // Vesting Schedule
    sectionVesting: 'Token Release Schedule (Vesting)',
    vestingTime: 'Release Time',
    vestingPercent: 'Release Percentage (%)',
    addVesting: 'Add Release Schedule',
    vestingTotalPercent: 'Total Percentage',
    vestingTotalHint: 'Must equal exactly 100%',
    // Validation messages
    errSoftCapGtHardCap: 'Soft Cap must be ≤ Hard Cap',
    errCapMustBePositive: 'Cap values must be > 0',
    errMaxPerUserGtHardCap: 'Max Per User must be ≤ Hard Cap',
    errStartTimeInPast: 'Start Time must be in the future',
    errEndTimeBeforeStart: 'End Time must be after Start Time',
    errTokenNotErc20: 'JoinCare currently only supports 18 decimals ERC20 tokens. Please verify or switch your token.',
    errVestingBeforeEnd: 'First release time must be after End Time',
    errVestingOrder: 'Release times must be in strict ascending order',
    errVestingTotalNot100: 'All release percentages must sum to exactly 100%',
    errLogoRequired: 'Project Logo is required',
    errDescriptionRequired: 'Project Description is required',
    // Calculator
    requiredTokensTitle: 'Required Token Deposit Estimate',
    requiredTokensMsg: 'Based on your Hard Cap and token price, you will need to deposit approximately',
    requiredTokensUnit: 'tokens to activate this project after it passes review.',
    // Actions
    submit: 'Submit Token Information',
    submitting: 'Submitting...',
    // Success
    successTitle: 'Submitted Successfully!',
    successMessage: 'Parameters are under smart contract evaluation and review...',
    backToDashboard: 'Back to Dashboard',
    // Verification status
    verifying: 'Verifying token...',
    verified: 'Token verified ✓',
    verifyFailed: 'Verification failed'
  },
  zh: {
    title: '提交項目的代幣資訊',
    subtitle: '配置項目的代幣參數和募資詳情',
    back: '返回 Dashboard',
    // Basic Display Info
    sectionBasic: '項目展示資料',
    projectNameCn: '項目名稱（繁中）',
    projectNameCnPlaceholder: '繁中項目名稱',
    projectNameEn: '項目名稱（英文）',
    projectNameEnPlaceholder: '英文項目名稱',
    logo: '項目 Logo',
    banner: '項目 Banner',
    uploadImage: '上傳圖片',
    uploading: '上傳中...',
    tagline: '一句話介紹項目',
    taglinePlaceholder: '用一句話介紹你的項目',
    description: '項目描述（不超過150字）',
    descriptionPlaceholder: '詳細的項目描述...',
    tags: '項目標籤',
    tagsPlaceholder: '添加標籤...',
    addTag: '添加',
    teamInfo: '團隊資訊描述',
    teamInfoPlaceholder: '描述你的團隊背景和成員...',
    roadmap: '項目發展路線 (Roadmap)',
    roadmapPlaceholder: '描述你的項目發展路線...',
    partners: '合作伙伴 (Partners)',
    partnersPlaceholder: '列出你的合作伙伴...',
    // Token & Fundraising Params
    sectionToken: '代幣及募資參數',
    tokenAddress: '項目代幣合約地址',
    tokenAddressPlaceholder: '0x... (BNB Chain ERC20 代幣)',
    tokenAddressHint: '僅支援 BNB Chain 上 18 位精度的 ERC20 標準代幣',
    tokenSymbol: '代幣符號 (Symbol)',
    tokenSymbolPlaceholder: '例如：ABC',
    tokenDecimals: '代幣精度',
    tokenDecimalsHint: '固定為 18 位',
    tokenPrice: '代幣價格',
    tokenPricePlaceholder: '1 個代幣 = ?',
    hardCap: '硬頂價格 (Hard Cap)',
    hardCapPlaceholder: '最大募資金額',
    softCap: '軟頂價格 (Soft Cap)',
    softCapPlaceholder: '最小募資金額',
    maxPerUser: '每用戶認購上限',
    maxPerUserPlaceholder: '單用戶最大認購額度',
    paymentToken: '募資貨幣',
    paymentTokenHint: '選擇募資所使用的貨幣',
    startTime: '項目開始時間',
    endTime: '項目結束時間',
    // Vesting Schedule
    sectionVesting: '項目釋放時間表 (Vesting Schedule)',
    vestingTime: '釋放時間',
    vestingPercent: '釋放百分比 (%)',
    addVesting: '添加釋放計劃',
    vestingTotalPercent: '釋放比例合計',
    vestingTotalHint: '必須恰好等於 100%',
    // Validation messages
    errSoftCapGtHardCap: '軟頂必須 ≤ 硬頂',
    errCapMustBePositive: '軟頂和硬頂必須 > 0',
    errMaxPerUserGtHardCap: '每用戶上限必須 ≤ 硬頂',
    errStartTimeInPast: '開始時間必須晚於當前時間',
    errEndTimeBeforeStart: '結束時間必須晚於開始時間',
    errTokenNotErc20: 'JoinCare 當前僅支援 18 decimals 的標準代幣，請查證或更換代幣參數。',
    errVestingBeforeEnd: '第一個釋放時間必須晚於結束時間',
    errVestingOrder: '釋放時間節點必須嚴格遞增排序',
    errVestingTotalNot100: '所有釋放比例之和必須恰好等於 100%',
    errLogoRequired: '項目 Logo 為必填項',
    errDescriptionRequired: '項目描述為必填項',
    // Calculator
    requiredTokensTitle: '必充代幣估算',
    requiredTokensMsg: '基於您設定的硬頂和價格，本項目在審核通過後需要您向合約系統先期充值約',
    requiredTokensUnit: '枚項目代幣以激活項目。',
    // Actions
    submit: '提交代幣資訊',
    submitting: '提交中...',
    // Success
    successTitle: '提交成功！',
    successMessage: '參數正在智能合約評估與審核中...',
    backToDashboard: '返回 Dashboard',
    // Verification status
    verifying: '正在驗證代幣...',
    verified: '代幣已驗證 ✓',
    verifyFailed: '驗證失敗'
  }
}

const t = computed(() => content[props.language] || content.en)

// Form Data
const form = ref({
  // Basic Display
  nameCn: '',
  nameEn: '',
  logoUrl: '',
  bannerUrl: '',
  tagline: '',
  description: '',
  tags: [],
  teamInfo: '',
  roadmap: '',
  partners: '',
  // Token & Fundraising
  tokenAddress: '',
  tokenSymbol: '',
  tokenDecimals: 18,
  tokenPrice: '',
  hardCap: '',
  softCap: '',
  maxPerUser: '',
  paymentToken: 'USDT',  // 募資貨幣選擇，預設為 USDT
  startTime: '',
  endTime: '',
  // Vesting Schedule
  vestingSchedule: [
    { time: '', percent: '' }
  ]
})

const newTag = ref('')
const isSubmitting = ref(false)
const isSubmitted = ref(false)
const isLoadingProject = ref(false)
const submitError = ref('')

// Token verification
const tokenVerifyStatus = ref('') // 'verifying' | 'verified' | 'failed' | ''
const tokenVerifyError = ref('')

// Upload states
const isUploadingLogo = ref(false)
const isUploadingBanner = ref(false)

// Validation errors
const validationErrors = ref([])

// Tags
function addTag() {
  const tag = newTag.value.trim()
  if (tag && !form.value.tags.includes(tag)) {
    form.value.tags.push(tag)
    newTag.value = ''
  }
}
function removeTag(index) {
  form.value.tags.splice(index, 1)
}

// Vesting Schedule
function addVestingRow() {
  form.value.vestingSchedule.push({ time: '', percent: '' })
}
function removeVestingRow(index) {
  if (form.value.vestingSchedule.length > 1) {
    form.value.vestingSchedule.splice(index, 1)
  }
}

// Computed: Total vesting percentage
const vestingTotalPercent = computed(() => {
  return form.value.vestingSchedule.reduce((sum, item) => {
    return sum + (parseFloat(item.percent) || 0)
  }, 0)
})

// Computed: Required tokens estimate
const requiredTokens = computed(() => {
  const hardCap = parseFloat(form.value.hardCap)
  const price = parseFloat(form.value.tokenPrice)
  if (hardCap > 0 && price > 0) {
    return (hardCap / price).toLocaleString(undefined, { maximumFractionDigits: 4 })
  }
  return null
})

// Token address verification (RPC)
let tokenVerifyTimer = null
watch(() => form.value.tokenAddress, (addr) => {
  clearTimeout(tokenVerifyTimer)
  tokenVerifyStatus.value = ''
  tokenVerifyError.value = ''

  if (!addr || addr.length !== 42 || !addr.startsWith('0x')) return

  tokenVerifyStatus.value = 'verifying'
  tokenVerifyTimer = setTimeout(async () => {
    try {
      // 使用 ethers 或直接 RPC 調用來查詢 decimals
      const provider = walletStore.provider
      if (!provider) {
        // 如果沒有 provider，仍然允許提交，後端會驗證
        tokenVerifyStatus.value = ''
        return
      }

      const { ethers } = await import('ethers')
      const ethersProvider = new ethers.BrowserProvider(provider)
      const tokenContract = new ethers.Contract(addr, [
        'function decimals() view returns (uint8)',
        'function symbol() view returns (string)'
      ], ethersProvider)

      const [decimals, symbol] = await Promise.all([
        tokenContract.decimals(),
        tokenContract.symbol()
      ])

      if (Number(decimals) !== 18) {
        tokenVerifyStatus.value = 'failed'
        tokenVerifyError.value = t.value.errTokenNotErc20
      } else {
        tokenVerifyStatus.value = 'verified'
        // 自動填充 symbol
        if (!form.value.tokenSymbol && symbol) {
          form.value.tokenSymbol = symbol
        }
      }
    } catch (err) {
      console.warn('Token verification failed:', err)
      tokenVerifyStatus.value = 'failed'
      tokenVerifyError.value = t.value.errTokenNotErc20
    }
  }, 800)
})

// Validation
function validate() {
  const errors = []
  const f = form.value

  const softCap = parseFloat(f.softCap)
  const hardCap = parseFloat(f.hardCap)
  const maxPerUser = parseFloat(f.maxPerUser)

  // Rule 0: Required fields
  if (!f.logoUrl) {
    errors.push(t.value.errLogoRequired)
  }
  if (!f.description || !f.description.trim()) {
    errors.push(t.value.errDescriptionRequired)
  }

  // Rule 1: Cap validity
  if (softCap <= 0 || hardCap <= 0) {
    errors.push(t.value.errCapMustBePositive)
  }
  if (softCap > hardCap) {
    errors.push(t.value.errSoftCapGtHardCap)
  }

  // Rule 2: Max per user
  if (maxPerUser > hardCap) {
    errors.push(t.value.errMaxPerUserGtHardCap)
  }

  // Rule 3: Timeline
  const now = new Date()
  const startTime = new Date(f.startTime)
  const endTime = new Date(f.endTime)

  if (startTime <= now) {
    errors.push(t.value.errStartTimeInPast)
  }
  if (endTime <= startTime) {
    errors.push(t.value.errEndTimeBeforeStart)
  }

  // Rule 4: Token decimals (already checked via RPC)
  if (tokenVerifyStatus.value === 'failed') {
    errors.push(t.value.errTokenNotErc20)
  }

  // Rule 5: Vesting Schedule
  const validSchedule = f.vestingSchedule.filter(item => item.time && item.percent)
  if (validSchedule.length > 0) {
    // First vesting time must be after end time
    if (new Date(validSchedule[0].time) <= endTime) {
      errors.push(t.value.errVestingBeforeEnd)
    }
    // Strict ascending order
    for (let i = 1; i < validSchedule.length; i++) {
      if (new Date(validSchedule[i].time) <= new Date(validSchedule[i - 1].time)) {
        errors.push(t.value.errVestingOrder)
        break
      }
    }
    // Total must be 100%
    const total = validSchedule.reduce((s, item) => s + (parseFloat(item.percent) || 0), 0)
    if (Math.abs(total - 100) > 0.01) {
      errors.push(t.value.errVestingTotalNot100)
    }
  }

  validationErrors.value = errors
  return errors.length === 0
}

// Image upload helper
async function handleImageUpload(event, type) {
  const file = event.target.files[0]
  if (!file) return

  const ext = file.name.split('.').pop().toLowerCase()
  const stateRef = type === 'logo' ? isUploadingLogo : isUploadingBanner

  stateRef.value = true
  try {
    // 獲取簽名上傳 URL
    const data = await getSignedUploadUrl(type, ext)
    if (!data || !data.signedUrl) {
      throw new Error(props.language === 'zh' ? '獲取上傳 URL 失敗' : 'Failed to get upload URL')
    }

    // 上傳文件到騰訊云 COS
    const uploadResponse = await fetch(data.signedUrl, {
      method: 'PUT',
      body: file,
      headers: { 'Content-Type': file.type }
    })

    if (!uploadResponse.ok) {
      throw new Error(`Upload failed with status ${uploadResponse.status}`)
    }

    // 設置展示 URL（使用後端返回的 url 字段）
    const displayUrl = data.url || data.signedUrl.split('?')[0]
    if (type === 'logo') {
      form.value.logoUrl = displayUrl
    } else {
      form.value.bannerUrl = displayUrl
    }

    console.log(`✅ ${type} uploaded successfully:`, displayUrl)
  } catch (err) {
    console.error(`❌ Upload ${type} failed:`, err)
    const errorMsg = props.language === 'zh'
      ? `${type === 'logo' ? 'Logo' : 'Banner'} 上傳失敗: ${err.message}`
      : `${type === 'logo' ? 'Logo' : 'Banner'} upload failed: ${err.message}`
    submitError.value = errorMsg
  } finally {
    stateRef.value = false
  }
}

// Submit
async function handleSubmit() {
  if (!validate()) return

  isSubmitting.value = true
  submitError.value = ''
  try {
    const f = form.value

    // 將 datetime-local 轉為 Unix 時間戳（秒）
    const toUnix = (dtStr) => Math.floor(new Date(dtStr).getTime() / 1000)

    // 拆分 vestingSchedule -> vestingTimes[] 和 vestingPercent[]
    const validSchedule = f.vestingSchedule.filter(item => item.time && item.percent)
    const vestingTimes = validSchedule.map(item => toUnix(item.time))
    // 後端期望的是整數百分比（如 20 代表 20%，後端內部會 Shift(-2)）
    const vestingPercent = validSchedule.map(item => parseFloat(item.percent))

    // 映射前端字段 -> 後端 SubmitProjectReviewRequest json key
    const requestBody = {
      projectID: projectId.value,  // 保持為字符串，避免 parseInt 導致的精度丟失
      nameCh: f.nameCn,
      nameEn: f.nameEn,
      logo: f.logoUrl || '',
      banner: f.bannerUrl || '',
      tagline: f.tagline,
      tags: f.tags,
      description: f.description || '',
      teamInfo: f.teamInfo || '',
      roadmap: f.roadmap || '',
      partners: f.partners || '',
      walletAddress: walletStore.address,
      tokenAddress: f.tokenAddress,
      tokenSymbol: f.tokenSymbol,
      tokenDecimals: 18,
      tokenPrice: parseFloat(f.tokenPrice) || 0,
      hardCap: parseFloat(f.hardCap) || 0,
      softCap: parseFloat(f.softCap) || 0,
      maxPerUser: parseFloat(f.maxPerUser) || 0,
      paymentToken: f.paymentToken,
      startTime: toUnix(f.startTime),
      endTime: toUnix(f.endTime),
      vestingTimes,
      vestingPercent
    }

    await submitProjectReview(requestBody)
    isSubmitted.value = true
  } catch (err) {
    console.error('Submit failed:', err)
    submitError.value = err.message || (props.language === 'zh' ? '提交失敗，請重試' : 'Submission failed, please try again')
  } finally {
    isSubmitting.value = false
  }
}

// Load existing project data
onMounted(async () => {
  if (projectId.value) {
    isLoadingProject.value = true
    try {
      const data = await getProjectDetail(projectId.value)
      if (data) {
        form.value.nameCn = data.name || ''
        form.value.nameEn = data.nameEn || data.name || ''
        form.value.logoUrl = data.logo?.url || ''
        form.value.tagline = data.tagline || ''
        form.value.tokenSymbol = data.tokenSymbol || ''
      }
    } catch (err) {
      console.warn('Failed to load project detail:', err)
    } finally {
      isLoadingProject.value = false
    }
  }
})

function goBack() {
  router.push('/dashboard')
}
</script>

<template>
  <div class="min-h-screen pt-32 pb-20 bg-gradient-to-br from-gray-50 via-white to-blue-50/30 dark:from-gray-950 dark:via-gray-900 dark:to-blue-950/30">
    <div class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8">

      <!-- Back -->
      <button @click="goBack" class="flex items-center gap-2 text-gray-600 dark:text-gray-400 hover:text-primary dark:hover:text-primary-400 mb-8 transition-colors">
        <ArrowLeft :size="20" />
        <span>{{ t.back }}</span>
      </button>

      <!-- Header -->
      <div class="text-center mb-12">
        <h1 class="text-4xl sm:text-5xl font-bold text-gray-900 dark:text-white mb-4">{{ t.title }}</h1>
        <p class="text-lg text-gray-600 dark:text-gray-400">{{ t.subtitle }}</p>
      </div>

      <!-- ═══ Success State ═══ -->
      <div v-if="isSubmitted" class="text-center py-16">
        <div class="mx-auto w-24 h-24 rounded-full bg-green-100 dark:bg-green-900/30 flex items-center justify-center mb-6 animate-bounce-in">
          <CheckCircle class="text-green-600 dark:text-green-400" :size="48" />
        </div>
        <h2 class="text-3xl font-bold text-gray-900 dark:text-white mb-4">{{ t.successTitle }}</h2>
        <p class="text-lg text-gray-600 dark:text-gray-400 mb-8 max-w-md mx-auto">{{ t.successMessage }}</p>
        <button @click="goBack" class="px-8 py-3 bg-primary hover:bg-primary-600 text-white rounded-xl font-semibold transition-all hover:scale-105">
          {{ t.backToDashboard }}
        </button>
      </div>

      <!-- ═══ Form ═══ -->
      <form v-else @submit.prevent="handleSubmit" class="space-y-8">

        <!-- Validation Errors -->
        <div v-if="validationErrors.length > 0" class="bg-red-50 dark:bg-red-950/20 border border-red-200 dark:border-red-900 rounded-xl p-5">
          <div class="flex items-start gap-3">
            <AlertCircle :size="20" class="text-red-500 flex-shrink-0 mt-0.5" />
            <div>
              <ul class="space-y-1">
                <li v-for="(err, idx) in validationErrors" :key="idx" class="text-sm text-red-700 dark:text-red-400">• {{ err }}</li>
              </ul>
            </div>
          </div>
        </div>

        <!-- Submit Error -->
        <div v-if="submitError" class="bg-red-50 dark:bg-red-950/20 border border-red-200 dark:border-red-900 rounded-xl p-4 text-center">
          <p class="text-sm text-red-700 dark:text-red-400 font-medium">{{ submitError }}</p>
        </div>

        <!-- ═══ Section 1: Basic Display Info ═══ -->
        <div class="bg-white dark:bg-gray-900 rounded-2xl p-8 border border-gray-200 dark:border-gray-800">
          <h2 class="text-2xl font-bold text-gray-900 dark:text-white mb-6 flex items-center gap-3">
            <Info :size="24" class="text-primary" />
            {{ t.sectionBasic }}
          </h2>

          <div class="space-y-5">
            <!-- Names -->
            <div class="grid md:grid-cols-2 gap-5">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.projectNameCn }} <span class="text-red-500">*</span></label>
                <input v-model="form.nameCn" type="text" required :placeholder="t.projectNameCnPlaceholder"
                  class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent" />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.projectNameEn }} <span class="text-red-500">*</span></label>
                <input v-model="form.nameEn" type="text" required :placeholder="t.projectNameEnPlaceholder"
                  class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent" />
              </div>
            </div>

            <!-- Logo & Banner Upload -->
            <div class="grid md:grid-cols-2 gap-5">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.logo }} <span class="text-red-500">*</span></label>
                <div class="flex items-center gap-3">
                  <img v-if="form.logoUrl" :src="form.logoUrl" class="w-16 h-16 rounded-xl object-cover border border-gray-300 dark:border-gray-700" />
                  <div v-else class="w-16 h-16 rounded-xl bg-gray-100 dark:bg-gray-800 border-2 border-dashed border-gray-300 dark:border-gray-700 flex items-center justify-center">
                    <Upload :size="20" class="text-gray-400" />
                  </div>
                  <label class="cursor-pointer px-4 py-2 bg-gray-100 dark:bg-gray-800 hover:bg-gray-200 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-xl text-sm font-medium transition-colors">
                    {{ isUploadingLogo ? t.uploading : t.uploadImage }}
                    <input type="file" class="hidden" accept="image/*" @change="e => handleImageUpload(e, 'logo')" :disabled="isUploadingLogo" />
                  </label>
                </div>
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.banner }}</label>
                <div class="flex items-center gap-3">
                  <div v-if="form.bannerUrl" class="w-32 h-16 rounded-xl overflow-hidden border border-gray-300 dark:border-gray-700">
                    <img :src="form.bannerUrl" class="w-full h-full object-cover" />
                  </div>
                  <div v-else class="w-32 h-16 rounded-xl bg-gray-100 dark:bg-gray-800 border-2 border-dashed border-gray-300 dark:border-gray-700 flex items-center justify-center">
                    <Upload :size="20" class="text-gray-400" />
                  </div>
                  <label class="cursor-pointer px-4 py-2 bg-gray-100 dark:bg-gray-800 hover:bg-gray-200 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-xl text-sm font-medium transition-colors">
                    {{ isUploadingBanner ? t.uploading : t.uploadImage }}
                    <input type="file" class="hidden" accept="image/*" @change="e => handleImageUpload(e, 'banner')" :disabled="isUploadingBanner" />
                  </label>
                </div>
              </div>
            </div>

            <!-- Tagline -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.tagline }} <span class="text-red-500">*</span></label>
              <input v-model="form.tagline" type="text" required :placeholder="t.taglinePlaceholder"
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent" />
            </div>

            <!-- Description -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.description }} <span class="text-red-500">*</span></label>
              <textarea v-model="form.description" rows="3" maxlength="150" required :placeholder="t.descriptionPlaceholder"
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent resize-none" />
              <p class="text-xs text-gray-400 mt-1 text-right">{{ form.description.length }}/150</p>
            </div>

            <!-- Tags -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.tags }}</label>
              <div class="flex flex-wrap gap-2 mb-3" v-if="form.tags.length > 0">
                <span v-for="(tag, index) in form.tags" :key="index" class="inline-flex items-center gap-1.5 px-3 py-1.5 bg-primary/10 dark:bg-primary/20 text-primary dark:text-primary-400 rounded-full text-sm font-medium">
                  {{ tag }}
                  <button type="button" @click="removeTag(index)" class="hover:text-red-500"><X :size="14" /></button>
                </span>
              </div>
              <div class="flex gap-2">
                <input v-model="newTag" type="text" @keyup.enter.prevent="addTag" :placeholder="t.tagsPlaceholder"
                  class="flex-1 px-4 py-2.5 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent text-sm" />
                <button type="button" @click="addTag" class="px-4 py-2.5 bg-primary hover:bg-primary-600 text-white rounded-xl text-sm font-medium transition-colors flex items-center gap-1">
                  <Plus :size="16" /> {{ t.addTag }}
                </button>
              </div>
            </div>

            <!-- Team Info -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.teamInfo }}</label>
              <textarea v-model="form.teamInfo" rows="3" :placeholder="t.teamInfoPlaceholder"
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent resize-none" />
            </div>

            <!-- Roadmap -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.roadmap }}</label>
              <textarea v-model="form.roadmap" rows="3" :placeholder="t.roadmapPlaceholder"
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent resize-none" />
            </div>

            <!-- Partners -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.partners }}</label>
              <textarea v-model="form.partners" rows="2" :placeholder="t.partnersPlaceholder"
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent resize-none" />
            </div>
          </div>
        </div>

        <!-- ═══ Section 2: Token & Fundraising Parameters ═══ -->
        <div class="bg-white dark:bg-gray-900 rounded-2xl p-8 border border-gray-200 dark:border-gray-800">
          <h2 class="text-2xl font-bold text-gray-900 dark:text-white mb-6 flex items-center gap-3">
            <Coins :size="24" class="text-primary" />
            {{ t.sectionToken }}
          </h2>

          <div class="space-y-5">
            <!-- Token Address -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.tokenAddress }} <span class="text-red-500">*</span></label>
              <div class="relative">
                <input v-model="form.tokenAddress" type="text" required :placeholder="t.tokenAddressPlaceholder"
                  class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent font-mono text-sm pr-32"
                  :class="tokenVerifyStatus === 'failed' ? 'border-red-500' : 'border-gray-300 dark:border-gray-700'" />
                <span v-if="tokenVerifyStatus" class="absolute right-4 top-1/2 -translate-y-1/2 text-xs font-medium"
                  :class="{
                    'text-amber-500': tokenVerifyStatus === 'verifying',
                    'text-green-500': tokenVerifyStatus === 'verified',
                    'text-red-500': tokenVerifyStatus === 'failed'
                  }">
                  {{ tokenVerifyStatus === 'verifying' ? t.verifying
                     : tokenVerifyStatus === 'verified' ? t.verified
                     : t.verifyFailed }}
                </span>
              </div>
              <p class="text-xs text-gray-400 mt-1.5">{{ t.tokenAddressHint }}</p>
              <p v-if="tokenVerifyError" class="text-xs text-red-500 mt-1">{{ tokenVerifyError }}</p>
            </div>

            <!-- Token Symbol & Decimals -->
            <div class="grid md:grid-cols-2 gap-5">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.tokenSymbol }} <span class="text-red-500">*</span></label>
                <input v-model="form.tokenSymbol" type="text" required :placeholder="t.tokenSymbolPlaceholder" maxlength="10"
                  class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent uppercase" />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.tokenDecimals }}</label>
                <input type="text" value="18" disabled
                  class="w-full px-4 py-3 bg-gray-100 dark:bg-gray-800/50 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-500 dark:text-gray-400 cursor-not-allowed" />
                <p class="text-xs text-gray-400 mt-1">{{ t.tokenDecimalsHint }}</p>
              </div>
            </div>

            <!-- Token Price & Max Per User -->
            <div class="grid md:grid-cols-2 gap-5">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.tokenPrice }} ({{ form.paymentToken }}) <span class="text-red-500">*</span></label>
                <input v-model="form.tokenPrice" type="number" step="any" min="0" required :placeholder="t.tokenPricePlaceholder"
                  class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent" />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.maxPerUser }} ({{ form.paymentToken }}) <span class="text-red-500">*</span></label>
                <input v-model="form.maxPerUser" type="number" step="any" min="0" required :placeholder="t.maxPerUserPlaceholder"
                  class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent" />
              </div>
            </div>

            <!-- Fundraising Currency -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.paymentToken }} <span class="text-red-500">*</span></label>
              <div class="grid grid-cols-3 gap-3">
                <label class="flex items-center p-3 border-2 rounded-xl transition-all opacity-50 cursor-not-allowed"
                  :class="'border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50'">
                  <input type="radio" v-model="form.paymentToken" value="JCC" disabled class="w-4 h-4" />
                  <span class="ml-2 font-medium text-gray-400 dark:text-gray-500">JCC</span>
                </label>
                <label class="flex items-center p-3 border-2 rounded-xl cursor-pointer transition-all"
                  :class="form.paymentToken === 'USDT' ? 'border-primary bg-primary/5 dark:bg-primary/10' : 'border-gray-300 dark:border-gray-700 hover:border-primary/50'">
                  <input type="radio" v-model="form.paymentToken" value="USDT" class="w-4 h-4" />
                  <span class="ml-2 font-medium text-gray-900 dark:text-white">USDT</span>
                </label>
                <label class="flex items-center p-3 border-2 rounded-xl cursor-pointer transition-all"
                  :class="form.paymentToken === 'USDC' ? 'border-primary bg-primary/5 dark:bg-primary/10' : 'border-gray-300 dark:border-gray-700 hover:border-primary/50'">
                  <input type="radio" v-model="form.paymentToken" value="USDC" class="w-4 h-4" />
                  <span class="ml-2 font-medium text-gray-900 dark:text-white">USDC</span>
                </label>
              </div>
              <p class="text-xs text-gray-400 mt-1.5">{{ t.paymentTokenHint }}</p>
            </div>

            <!-- Hard Cap & Soft Cap -->
            <div class="grid md:grid-cols-2 gap-5">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.hardCap }} ({{ form.paymentToken }}) <span class="text-red-500">*</span></label>
                <input v-model="form.hardCap" type="number" step="any" min="0" required :placeholder="t.hardCapPlaceholder"
                  class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent" />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.softCap }} ({{ form.paymentToken }}) <span class="text-red-500">*</span></label>
                <input v-model="form.softCap" type="number" step="any" min="0" required :placeholder="t.softCapPlaceholder"
                  class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent" />
              </div>
            </div>

            <!-- Start Time & End Time -->
            <div class="grid md:grid-cols-2 gap-5">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.startTime }} <span class="text-red-500">*</span></label>
                <input v-model="form.startTime" type="datetime-local" required
                  class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent" />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.endTime }} <span class="text-red-500">*</span></label>
                <input v-model="form.endTime" type="datetime-local" required
                  class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent" />
              </div>
            </div>
          </div>
        </div>

        <!-- ═══ Section 3: Vesting Schedule ═══ -->
        <div class="bg-white dark:bg-gray-900 rounded-2xl p-8 border border-gray-200 dark:border-gray-800">
          <h2 class="text-2xl font-bold text-gray-900 dark:text-white mb-6 flex items-center gap-3">
            <Clock :size="24" class="text-primary" />
            {{ t.sectionVesting }}
          </h2>

          <div class="space-y-4">
            <div v-for="(row, index) in form.vestingSchedule" :key="index" class="flex items-end gap-3">
              <div class="flex-1">
                <label v-if="index === 0" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.vestingTime }}</label>
                <input v-model="row.time" type="datetime-local" required
                  class="w-full px-4 py-2.5 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent text-sm" />
              </div>
              <div class="w-32">
                <label v-if="index === 0" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t.vestingPercent }}</label>
                <input v-model="row.percent" type="number" step="0.01" min="0" max="100" required placeholder="e.g. 20"
                  class="w-full px-4 py-2.5 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent text-sm" />
              </div>
              <button
                type="button"
                @click="removeVestingRow(index)"
                :disabled="form.vestingSchedule.length <= 1"
                class="p-2.5 text-gray-400 hover:text-red-500 transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
              >
                <Trash2 :size="18" />
              </button>
            </div>

            <!-- Add Row -->
            <button type="button" @click="addVestingRow"
              class="w-full py-2.5 border-2 border-dashed border-gray-300 dark:border-gray-700 rounded-xl text-gray-500 dark:text-gray-400 hover:border-primary hover:text-primary transition-colors flex items-center justify-center gap-2 text-sm font-medium">
              <Plus :size="18" /> {{ t.addVesting }}
            </button>

            <!-- Vesting Total -->
            <div class="flex items-center justify-between p-4 rounded-xl"
              :class="Math.abs(vestingTotalPercent - 100) < 0.01 ? 'bg-green-50 dark:bg-green-950/20' : 'bg-amber-50 dark:bg-amber-950/20'">
              <span class="text-sm font-medium" :class="Math.abs(vestingTotalPercent - 100) < 0.01 ? 'text-green-700 dark:text-green-400' : 'text-amber-700 dark:text-amber-400'">
                {{ t.vestingTotalPercent }}: {{ vestingTotalPercent.toFixed(2) }}%
              </span>
              <span class="text-xs" :class="Math.abs(vestingTotalPercent - 100) < 0.01 ? 'text-green-600 dark:text-green-500' : 'text-amber-600 dark:text-amber-500'">
                {{ t.vestingTotalHint }}
              </span>
            </div>
          </div>
        </div>

        <!-- ═══ Calculator: Required Tokens ═══ -->
        <div v-if="requiredTokens" class="bg-gradient-to-r from-primary/5 to-teal-500/5 dark:from-primary/10 dark:to-teal-500/10 rounded-2xl p-6 border border-primary/20 dark:border-primary/30">
          <div class="flex items-start gap-4">
            <div class="w-12 h-12 rounded-xl bg-primary/10 dark:bg-primary/20 flex items-center justify-center flex-shrink-0">
              <Calculator :size="24" class="text-primary" />
            </div>
            <div>
              <h3 class="font-bold text-gray-900 dark:text-white mb-2">{{ t.requiredTokensTitle }}</h3>
              <p class="text-sm text-gray-600 dark:text-gray-400">
                {{ t.requiredTokensMsg }}
                <span class="font-bold text-xl text-primary dark:text-primary-400 mx-1">{{ requiredTokens }}</span>
                {{ form.tokenSymbol || '' }} {{ t.requiredTokensUnit }}
              </p>
            </div>
          </div>
        </div>

        <!-- ═══ Submit ═══ -->
        <div class="flex justify-end gap-4">
          <button type="button" @click="goBack"
            class="px-8 py-3 border border-gray-300 dark:border-gray-700 text-gray-700 dark:text-gray-300 rounded-xl font-semibold hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors">
            {{ t.back }}
          </button>
          <button type="submit" :disabled="isSubmitting || tokenVerifyStatus === 'failed'"
            class="px-8 py-3 bg-teal-600 hover:bg-teal-700 text-white rounded-xl font-semibold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100">
            {{ isSubmitting ? t.submitting : t.submit }}
          </button>
        </div>
      </form>

    </div>
  </div>
</template>

<style scoped>
.animate-bounce-in {
  animation: bounceIn 0.6s ease-out;
}
@keyframes bounceIn {
  0% { transform: scale(0.3); opacity: 0; }
  50% { transform: scale(1.05); }
  70% { transform: scale(0.9); }
  100% { transform: scale(1); opacity: 1; }
}
</style>
