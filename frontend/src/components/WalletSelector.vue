<script setup>
/**
 * 錢包選擇彈窗組件
 * 
 * 參考 xtask_frontend 的 WalletSelectorModal/WalletSelector，
 * 適配 Vue 3 + JoinCare 項目的樣式體系。
 * 
 * 使用方式：
 *   <WalletSelector v-model:visible="showSelector" @select="onWalletSelected" />
 */
import { ref, onMounted, watch } from 'vue'
import { X, ChevronRight, Download, Loader2 } from 'lucide-vue-next'
import { walletManager } from '../services/walletManager'

const props = defineProps({
  visible: { type: Boolean, default: false },
  language: { type: String, default: 'en' },
  mode: { type: String, default: 'connect' } // 'connect' | 'payment'
})

const emit = defineEmits(['update:visible', 'select'])

const wallets = ref([])
const selectedWallet = ref(null)
const loading = ref(false)
const error = ref(null)

const content = {
  en: {
    selectWallet: 'Select Wallet',
    selectWalletDesc: 'Choose the wallet you want to connect',
    selectWalletForPayment: 'Select payment wallet',
    notInstalled: 'Not Installed',
    cancel: 'Cancel',
    connectTip: 'Connect your wallet to verify your identity. Your private keys are never shared.'
  },
  zh: {
    selectWallet: '選擇錢包',
    selectWalletDesc: '選擇您想要連接的錢包',
    selectWalletForPayment: '選擇支付錢包',
    notInstalled: '未安裝',
    cancel: '取消',
    connectTip: '連接您的錢包以完成身份驗證。您的私鑰永遠不會被分享。'
  }
}

onMounted(() => {
  wallets.value = walletManager.getAllAvailableWallets()
})

watch(() => props.visible, (val) => {
  if (val) {
    wallets.value = walletManager.getAllAvailableWallets()
    selectedWallet.value = null
    error.value = null
    loading.value = false
  }
})

const t = (key) => {
  return content[props.language]?.[key] || content['en'][key]
}

const close = () => {
  emit('update:visible', false)
}

const handleWalletClick = async (wallet) => {
  if (!wallet.isInstalled) {
    window.open(wallet.downloadUrl, '_blank')
    return
  }

  try {
    selectedWallet.value = wallet.type
    error.value = null
    loading.value = true
    emit('select', wallet.type)
  } catch (err) {
    error.value = err.message || 'Failed to connect wallet'
    selectedWallet.value = null
    loading.value = false
  }
}

// 外部調用以重置 loading 狀態
const resetLoading = () => {
  loading.value = false
  selectedWallet.value = null
}

defineExpose({ resetLoading })
</script>

<template>
  <Teleport to="body">
    <Transition name="wallet-modal">
      <div
        v-if="visible"
        class="fixed inset-0 z-[999] flex items-center justify-center"
      >
        <!-- 背景遮罩 -->
        <div
          class="absolute inset-0 bg-black/50 backdrop-blur-sm"
          @click="close"
        ></div>

        <!-- Modal 內容 -->
        <div
          class="relative z-[1000] bg-white dark:bg-gray-900 rounded-2xl shadow-2xl border border-gray-200 dark:border-gray-700/40 w-full max-w-sm mx-4 p-6 max-h-[90vh] overflow-y-auto animate-scaleIn"
        >
          <!-- 關閉按鈕 -->
          <button
            @click="close"
            :disabled="loading"
            class="absolute top-4 right-4 p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors disabled:opacity-50"
          >
            <X :size="20" />
          </button>

          <!-- 標題 -->
          <div class="text-center mb-6">
            <h2 class="text-xl font-bold text-gray-900 dark:text-white mb-2">
              {{ t('selectWallet') }}
            </h2>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ mode === 'payment' ? t('selectWalletForPayment') : t('selectWalletDesc') }}
            </p>
          </div>

          <!-- 錯誤提示 -->
          <div
            v-if="error"
            class="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/30 rounded-lg"
          >
            <p class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
          </div>

          <!-- 錢包列表 -->
          <div class="space-y-3">
            <button
              v-for="wallet in wallets"
              :key="wallet.type"
              @click="handleWalletClick(wallet)"
              :disabled="loading && selectedWallet !== wallet.type"
              :class="[
                'w-full p-4 rounded-xl border transition-all duration-200 flex items-center justify-between',
                wallet.isInstalled
                  ? 'border-gray-200 dark:border-gray-700 hover:border-primary dark:hover:border-primary-600 hover:bg-primary/5 dark:hover:bg-primary/10 cursor-pointer'
                  : 'border-gray-100 dark:border-gray-800 bg-gray-50 dark:bg-gray-800/30 cursor-default',
                selectedWallet === wallet.type ? 'ring-2 ring-primary dark:ring-primary-500' : '',
                loading && selectedWallet !== wallet.type ? 'opacity-50' : ''
              ]"
            >
              <!-- 左側：圖標和名稱 -->
              <div class="flex items-center gap-3 flex-1">
                <div class="w-10 h-10 flex items-center justify-center flex-shrink-0">
                  <img
                    v-if="wallet.icon.includes('/')"
                    :src="wallet.icon"
                    :alt="wallet.name"
                    class="w-10 h-10 object-contain rounded-lg"
                  />
                  <span v-else class="text-3xl">{{ wallet.icon }}</span>
                </div>
                <div class="text-left">
                  <div class="font-semibold text-gray-900 dark:text-white">{{ wallet.name }}</div>
                  <div
                    v-if="!wallet.isInstalled"
                    class="text-xs text-gray-500 dark:text-gray-400 flex items-center gap-1"
                  >
                    <Download :size="12" />
                    {{ t('notInstalled') }}
                  </div>
                </div>
              </div>

              <!-- 右側：狀態圖標 -->
              <div class="flex-shrink-0">
                <Loader2
                  v-if="loading && selectedWallet === wallet.type"
                  :size="20"
                  class="animate-spin text-primary"
                />
                <ChevronRight
                  v-else-if="wallet.isInstalled"
                  :size="20"
                  class="text-gray-400"
                />
                <Download
                  v-else
                  :size="20"
                  class="text-gray-400"
                />
              </div>
            </button>
          </div>

          <!-- 取消按鈕 -->
          <button
            @click="close"
            class="w-full mt-4 px-4 py-2.5 text-sm font-medium text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl transition-colors"
          >
            {{ t('cancel') }}
          </button>

          <!-- 提示資訊 -->
          <div class="mt-4 p-3 bg-blue-50 dark:bg-blue-900/10 border border-blue-100 dark:border-blue-800/30 rounded-xl">
            <p class="text-xs text-gray-500 dark:text-gray-400 leading-relaxed">
              {{ t('connectTip') }}
            </p>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
/* 彈窗入場/退場動畫 */
.wallet-modal-enter-active,
.wallet-modal-leave-active {
  transition: opacity 0.3s ease;
}

.wallet-modal-enter-from,
.wallet-modal-leave-to {
  opacity: 0;
}

/* 彈窗縮放入場動畫 */
@keyframes scaleIn {
  from {
    transform: scale(0.9);
    opacity: 0;
  }
  to {
    transform: scale(1);
    opacity: 1;
  }
}

.animate-scaleIn {
  animation: scaleIn 0.25s ease-out;
}
</style>
