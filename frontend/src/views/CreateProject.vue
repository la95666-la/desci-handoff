<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useWalletStore } from '../stores/wallet'
import { ArrowLeft, Upload, Plus, X, AlertCircle } from 'lucide-vue-next'
import { createProject } from '../services/api'

const props = defineProps({
  language: String
})

const router = useRouter()
const walletStore = useWalletStore()

const content = {
  en: {
    title: 'Create New Project',
    subtitle: 'Submit your research project for community funding',
    connectWallet: 'Please connect your wallet to create a project',
    connectWalletDesc: 'You need to connect your wallet before creating a project.',
    basicInfo: 'Basic Information',
    projectName: 'Project Name',
    projectNamePlaceholder: 'Enter project name',
    tokenSymbol: 'Token Symbol',
    tokenSymbolPlaceholder: 'e.g., P2X1',
    tokenAddress: 'Token Contract Address',
    tokenAddressPlaceholder: 'e.g., 0xbCA8400D...',
    shortDesc: 'Short Description',
    shortDescPlaceholder: 'Brief description of your project...',
    fullDesc: 'Full Description',
    fullDescPlaceholder: 'Detailed description of your research project...',
    fundingInfo: 'Funding Information',
    tokenPrice: 'Token Price (USD)',
    softCap: 'Soft Cap (USD)',
    hardCap: 'Hard Cap (USD)',
    maxPerUser: 'Max Per User (USD)',
    timeline: 'Timeline',
    saleStart: 'Sale Start Date',
    saleEnd: 'Sale End Date',
    tgeDate: 'TGE Date',
    tgePercent: 'TGE Unlock %',
    vestingCycle: 'Vesting Cycle (days)',
    vestingPercent: 'Vesting % per cycle',
    media: 'Media & Links',
    projectLogo: 'Project Logo URL',
    website: 'Website',
    twitter: 'Twitter',
    telegram: 'Telegram',
    discord: 'Discord',
    tags: 'Tags',
    addTag: 'Add Tag',
    submit: 'Submit Project',
    cancel: 'Cancel',
    submitting: 'Submitting...',
    success: 'Project submitted successfully!',
    error: 'Failed to submit project'
  },
  zh: {
    title: '創建新項目',
    subtitle: '提交您的研究項目以獲得社區資助',
    connectWallet: '請連接錢包以創建項目',
    connectWalletDesc: '您需要先連接錢包才能創建項目。',
    basicInfo: '基本資訊',
    projectName: '項目名稱',
    projectNamePlaceholder: '輸入項目名稱',
    tokenSymbol: '代幣符號',
    tokenSymbolPlaceholder: '例如：P2X1',
    tokenAddress: '代幣合約地址',
    tokenAddressPlaceholder: '例如：0xbCA8400D...',
    shortDesc: '簡短描述',
    shortDescPlaceholder: '項目簡短描述...',
    fullDesc: '完整描述',
    fullDescPlaceholder: '研究項目的詳細描述...',
    fundingInfo: '融資資訊',
    tokenPrice: '代幣價格（美元）',
    softCap: '軟頂（美元）',
    hardCap: '硬頂（美元）',
    maxPerUser: '單人上限（美元）',
    timeline: '時間線',
    saleStart: '銷售開始日期',
    saleEnd: '銷售結束日期',
    tgeDate: 'TGE日期',
    tgePercent: 'TGE解鎖百分比',
    vestingCycle: '釋放周期（天）',
    vestingPercent: '每周期釋放百分比',
    media: '媒體與連結',
    projectLogo: '項目Logo URL',
    website: '網站',
    twitter: 'Twitter',
    telegram: 'Telegram',
    discord: 'Discord',
    tags: '標籤',
    addTag: '添加標籤',
    submit: '提交項目',
    cancel: '取消',
    submitting: '提交中...',
    success: '項目提交成功！',
    error: '項目提交失敗'
  }
}

// Form data
const formData = ref({
  name: '',
  tokenSymbol: '',
  tokenAddress: '',
  shortDesc: '',
  fullDesc: '',
  logo: '',
  tokenPrice: '',
  softCap: '',
  hardCap: '',
  maxPerUser: '',
  saleStart: '',
  saleEnd: '',
  tgeDate: '',
  tgePercent: 20,
  vestingCycle: 30,
  vestingPercent: 10,
  website: '',
  twitter: '',
  telegram: '',
  discord: '',
  tags: []
})

const newTag = ref('')
const isSubmitting = ref(false)
const submitMessage = ref('')
const submitError = ref(false)

const addTag = () => {
  if (newTag.value.trim() && !formData.value.tags.includes(newTag.value.trim())) {
    formData.value.tags.push(newTag.value.trim())
    newTag.value = ''
  }
}

const removeTag = (index) => {
  formData.value.tags.splice(index, 1)
}

const goBack = () => {
  router.push('/dashboard')
}

/**
 * 根據 TGE 日期和 vesting 配置生成 vestingTimes 和 vestingPercent 字符串
 */
function buildVesting(tgeDate, tgePercent, vestingCycle, vestingPercent) {
  const vestingTimes = []
  const vestingPercents = []
  const tge = new Date(tgeDate)

  // TGE 首次釋放
  vestingTimes.push(tge.toISOString().split('T')[0].replace(/-/g, '/'))
  vestingPercents.push(tgePercent / 100) // 轉為小數

  // 後續 vesting 周期
  let remaining = 100 - tgePercent
  let currentDate = new Date(tge)
  while (remaining > 0) {
    currentDate = new Date(currentDate.getTime() + vestingCycle * 86400000) // 加天數
    const percent = Math.min(vestingPercent, remaining)
    vestingTimes.push(currentDate.toISOString().split('T')[0].replace(/-/g, '/'))
    vestingPercents.push(percent / 100)
    remaining -= percent
  }

  return {
    vestingTimes: vestingTimes.join(','),
    vestingPercent: vestingPercents.join(',')
  }
}

const submitProject = async () => {
  if (!walletStore.isConnected) {
    submitError.value = true
    submitMessage.value = content[props.language].connectWallet
    return
  }

  isSubmitting.value = true
  submitError.value = false
  submitMessage.value = ''

  try {
    const f = formData.value
    const { vestingTimes, vestingPercent } = buildVesting(
      f.tgeDate, f.tgePercent, f.vestingCycle, f.vestingPercent
    )

    // 構建後端 CreateRequest 格式
    const requestBody = {
      nameEn: f.name,
      nameCh: f.name,
      logo: f.logo || '',
      banner: '',
      slug: f.name.toLowerCase().replace(/\s+/g, '-'),
      tagline: f.shortDesc,
      tags: f.tags.join(','),
      description: f.fullDesc,
      teamInfo: '',
      roadmap: '',
      partners: '',
      walletAddress: walletStore.address,
      tokenAddress: f.tokenAddress || '',
      tokenSymbol: f.tokenSymbol,
      tokenDecimals: 18,
      tokenPrice: parseFloat(f.tokenPrice) || 0,
      hardCap: parseFloat(f.hardCap) || 0,
      softCap: parseFloat(f.softCap) || 0,
      maxPerUser: parseFloat(f.maxPerUser) || 0,
      startTime: f.saleStart.replace(/-/g, '/'),
      endTime: f.saleEnd.replace(/-/g, '/'),
      vestingTimes,
      vestingPercent,
      vestingDuration: 0
    }

    await createProject(requestBody)

    submitMessage.value = content[props.language].success

    // Redirect to dashboard after 2 seconds
    setTimeout(() => {
      router.push('/dashboard')
    }, 2000)
  } catch (err) {
    console.error('Failed to create project:', err)
    submitError.value = true
    submitMessage.value = err.message || content[props.language].error
  } finally {
    isSubmitting.value = false
  }
}
</script>

<template>
  <div class="min-h-screen pt-32 pb-20 bg-gradient-to-br from-gray-50 via-white to-blue-50/30 dark:from-gray-950 dark:via-gray-900 dark:to-blue-950/30">
    <div class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8">
      <!-- Back Button -->
      <button
        @click="goBack"
        class="flex items-center gap-2 text-gray-600 dark:text-gray-400 hover:text-primary dark:hover:text-primary-400 mb-8 transition-colors"
      >
        <ArrowLeft :size="20" />
        <span>{{ content[language].cancel }}</span>
      </button>

      <!-- Header -->
      <div class="text-center mb-12">
        <h1 class="text-4xl sm:text-5xl font-bold text-gray-900 dark:text-white mb-4">
          {{ content[language].title }}
        </h1>
        <p class="text-lg text-gray-600 dark:text-gray-400">
          {{ content[language].subtitle }}
        </p>
      </div>

      <!-- Wallet Connection Warning -->
      <div v-if="!walletStore.isConnected" class="mb-8 p-6 bg-amber-50 dark:bg-amber-950/20 border border-amber-200 dark:border-amber-900 rounded-xl">
        <div class="flex items-start gap-3">
          <AlertCircle :size="24" class="text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5" />
          <div>
            <h3 class="text-lg font-semibold text-amber-900 dark:text-amber-300 mb-1">
              {{ content[language].connectWallet }}
            </h3>
            <p class="text-sm text-amber-700 dark:text-amber-400">
              {{ content[language].connectWalletDesc }}
            </p>
          </div>
        </div>
      </div>

      <!-- Success/Error Message -->
      <div v-if="submitMessage" class="mb-8 p-6 rounded-xl border" :class="submitError ? 'bg-red-50 dark:bg-red-950/20 border-red-200 dark:border-red-900' : 'bg-green-50 dark:bg-green-950/20 border-green-200 dark:border-green-900'">
        <p class="text-center font-semibold" :class="submitError ? 'text-red-900 dark:text-red-300' : 'text-green-900 dark:text-green-300'">
          {{ submitMessage }}
        </p>
      </div>

      <!-- Form -->
      <form @submit.prevent="submitProject" class="space-y-8">
        <!-- Basic Information -->
        <div class="bg-white dark:bg-gray-900 rounded-2xl p-8 border border-gray-200 dark:border-gray-800">
          <h2 class="text-2xl font-bold text-gray-900 dark:text-white mb-6">
            {{ content[language].basicInfo }}
          </h2>

          <div class="space-y-6">
            <!-- Project Name -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ content[language].projectName }}
              </label>
              <input
                v-model="formData.name"
                type="text"
                required
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent"
                :placeholder="content[language].projectNamePlaceholder"
              />
            </div>

            <!-- Token Symbol & Logo -->
            <div class="grid md:grid-cols-2 gap-6">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  {{ content[language].tokenSymbol }}
                </label>
                <input
                  v-model="formData.tokenSymbol"
                  type="text"
                  required
                  class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent uppercase"
                  :placeholder="content[language].tokenSymbolPlaceholder"
                  maxlength="10"
                />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  {{ content[language].projectLogo }}
                </label>
                <input
                  v-model="formData.logo"
                  type="url"
                  class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent"
                  placeholder="https://..."
                />
              </div>
            </div>

            <!-- Short Description -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ content[language].shortDesc }}
              </label>
              <textarea
                v-model="formData.shortDesc"
                required
                rows="3"
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent resize-none"
                :placeholder="content[language].shortDescPlaceholder"
              ></textarea>
            </div>

            <!-- Full Description -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ content[language].fullDesc }}
              </label>
              <textarea
                v-model="formData.fullDesc"
                required
                rows="6"
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent resize-none"
                :placeholder="content[language].fullDescPlaceholder"
              ></textarea>
            </div>

            <!-- Tags -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ content[language].tags }}
              </label>
              <div class="flex flex-wrap gap-2 mb-3">
                <span
                  v-for="(tag, index) in formData.tags"
                  :key="index"
                  class="inline-flex items-center gap-2 px-3 py-1.5 bg-primary/10 dark:bg-primary/20 text-primary dark:text-primary-400 rounded-full text-sm font-medium"
                >
                  {{ tag }}
                  <button type="button" @click="removeTag(index)" class="hover:text-primary-600">
                    <X :size="14" />
                  </button>
                </span>
              </div>
              <div class="flex gap-2">
                <input
                  v-model="newTag"
                  type="text"
                  @keyup.enter="addTag"
                  class="flex-1 px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent"
                  placeholder="Oncology, DeSci, etc."
                />
                <button
                  type="button"
                  @click="addTag"
                  class="px-6 py-3 bg-primary hover:bg-primary-600 text-white rounded-xl font-medium transition-colors flex items-center gap-2"
                >
                  <Plus :size="18" />
                  {{ content[language].addTag }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Funding Information -->
        <div class="bg-white dark:bg-gray-900 rounded-2xl p-8 border border-gray-200 dark:border-gray-800">
          <h2 class="text-2xl font-bold text-gray-900 dark:text-white mb-6">
            {{ content[language].fundingInfo }}
          </h2>

          <div class="grid md:grid-cols-2 gap-6">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ content[language].tokenPrice }}
              </label>
              <input
                v-model.number="formData.tokenPrice"
                type="number"
                step="0.01"
                min="0"
                required
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent"
                placeholder="0.10"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ content[language].maxPerUser }}
              </label>
              <input
                v-model.number="formData.maxPerUser"
                type="number"
                min="0"
                required
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent"
                placeholder="10000"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ content[language].softCap }}
              </label>
              <input
                v-model.number="formData.softCap"
                type="number"
                min="0"
                required
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent"
                placeholder="500000"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ content[language].hardCap }}
              </label>
              <input
                v-model.number="formData.hardCap"
                type="number"
                min="0"
                required
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent"
                placeholder="1500000"
              />
            </div>
          </div>
        </div>

        <!-- Timeline -->
        <div class="bg-white dark:bg-gray-900 rounded-2xl p-8 border border-gray-200 dark:border-gray-800">
          <h2 class="text-2xl font-bold text-gray-900 dark:text-white mb-6">
            {{ content[language].timeline }}
          </h2>

          <div class="grid md:grid-cols-2 gap-6">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ content[language].saleStart }}
              </label>
              <input
                v-model="formData.saleStart"
                type="date"
                required
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ content[language].saleEnd }}
              </label>
              <input
                v-model="formData.saleEnd"
                type="date"
                required
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ content[language].tgeDate }}
              </label>
              <input
                v-model="formData.tgeDate"
                type="date"
                required
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ content[language].tgePercent }}
              </label>
              <input
                v-model.number="formData.tgePercent"
                type="number"
                min="0"
                max="100"
                required
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent"
                placeholder="20"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ content[language].vestingCycle }}
              </label>
              <input
                v-model.number="formData.vestingCycle"
                type="number"
                min="1"
                required
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent"
                placeholder="30"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ content[language].vestingPercent }}
              </label>
              <input
                v-model.number="formData.vestingPercent"
                type="number"
                min="0"
                max="100"
                required
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent"
                placeholder="10"
              />
            </div>
          </div>
        </div>

        <!-- Social Links -->
        <div class="bg-white dark:bg-gray-900 rounded-2xl p-8 border border-gray-200 dark:border-gray-800">
          <h2 class="text-2xl font-bold text-gray-900 dark:text-white mb-6">
            {{ content[language].media }}
          </h2>

          <div class="grid md:grid-cols-2 gap-6">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ content[language].website }}
              </label>
              <input
                v-model="formData.website"
                type="url"
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent"
                placeholder="https://..."
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ content[language].twitter }}
              </label>
              <input
                v-model="formData.twitter"
                type="url"
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent"
                placeholder="https://twitter.com/..."
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ content[language].telegram }}
              </label>
              <input
                v-model="formData.telegram"
                type="url"
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent"
                placeholder="https://t.me/..."
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ content[language].discord }}
              </label>
              <input
                v-model="formData.discord"
                type="url"
                class="w-full px-4 py-3 bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-xl text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent"
                placeholder="https://discord.gg/..."
              />
            </div>
          </div>
        </div>

        <!-- Submit Buttons -->
        <div class="flex gap-4 justify-end">
          <button
            type="button"
            @click="goBack"
            class="px-8 py-4 border border-gray-300 dark:border-gray-700 text-gray-900 dark:text-white rounded-xl font-semibold hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
          >
            {{ content[language].cancel }}
          </button>
          <button
            type="submit"
            :disabled="isSubmitting || !walletStore.isConnected"
            class="px-8 py-4 bg-teal-700 hover:bg-teal-800 text-white rounded-xl font-semibold transition-all disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {{ isSubmitting ? content[language].submitting : content[language].submit }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

