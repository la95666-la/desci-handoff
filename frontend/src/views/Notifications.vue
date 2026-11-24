<script setup>
import { ref, computed, onMounted, inject, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useWalletStore } from '../stores/wallet'
import { getNotificationList, getRedPointStatus } from '../services/api'
import {
  CheckCircle2,
  XCircle,
  Coins,
  Bell,
  Loader2,
  Clock,
  ChevronRight,
  ArrowDownToLine,
  AlertTriangle,
  RefreshCw
} from 'lucide-vue-next'

const props = defineProps({
  language: {
    type: String,
    default: 'en'
  }
})

const router = useRouter()
const walletStore = useWalletStore()

const notifications = ref([])
const loading = ref(true)
const cursor = ref(0)
const hasMore = ref(true)
const loadingMore = ref(false)

// i18n 文案
const content = {
  en: {
    title: 'Notifications',
    unread: 'unread',
    empty: 'No notifications yet',
    emptyDesc: 'You will receive notifications about your purchases, token releases, claims and refunds here.',
    loadMore: 'Load More',
    loading: 'Loading...',
    loadError: 'Failed to load notifications',
    justNow: 'Just now',
    minutesAgo: 'minutes ago',
    hoursAgo: 'hours ago',
    daysAgo: 'days ago',
    loginRequired: 'Please connect your wallet to view notifications',
    connectWallet: 'Connect Wallet'
  },
  zh: {
    title: '消息通知',
    unread: '條未讀',
    empty: '暫無通知',
    emptyDesc: '您將在這里收到認購、代幣釋放、領取和退款等通知。',
    loadMore: '加載更多',
    loading: '加載中...',
    loadError: '加載通知失敗',
    justNow: '剛剛',
    minutesAgo: '分鐘前',
    hoursAgo: '小時前',
    daysAgo: '天前',
    loginRequired: '請先連接錢包以查看通知',
    connectWallet: '連接錢包'
  }
}

const t = computed(() => content[props.language] || content.en)

const unreadCount = computed(() => notifications.value.filter(n => !n.isRead).length)

// 通知類型圖標映射
function getNotificationIcon(type) {
  switch (type) {
    case 'PurchaseSuccessful':
      return CheckCircle2
    case 'TokensReleased':
      return ArrowDownToLine
    case 'ClaimSuccessful':
      return Coins
    case 'ProjectFundingFailed':
      return AlertTriangle
    case 'RefundSuccessful':
      return RefreshCw
    default:
      return Bell
  }
}

function getNotificationIconColor(type) {
  switch (type) {
    case 'PurchaseSuccessful':
      return 'text-green-500'
    case 'TokensReleased':
      return 'text-blue-500'
    case 'ClaimSuccessful':
      return 'text-emerald-500'
    case 'ProjectFundingFailed':
      return 'text-red-500'
    case 'RefundSuccessful':
      return 'text-amber-500'
    default:
      return 'text-gray-400'
  }
}

function getNotificationCardClass(type, isRead) {
  let base = 'border rounded-xl p-4 transition-all duration-200 '

  switch (type) {
    case 'PurchaseSuccessful':
      base += 'border-green-200 bg-green-50/50 dark:bg-green-950/20 dark:border-green-800/50 '
      break
    case 'TokensReleased':
      base += 'border-blue-200 bg-blue-50/50 dark:bg-blue-950/20 dark:border-blue-800/50 '
      break
    case 'ClaimSuccessful':
      base += 'border-emerald-200 bg-emerald-50/50 dark:bg-emerald-950/20 dark:border-emerald-800/50 '
      break
    case 'ProjectFundingFailed':
      base += 'border-red-200 bg-red-50/50 dark:bg-red-950/20 dark:border-red-800/50 '
      break
    case 'RefundSuccessful':
      base += 'border-amber-200 bg-amber-50/50 dark:bg-amber-950/20 dark:border-amber-800/50 '
      break
    default:
      base += 'border-gray-200 bg-gray-50/50 dark:bg-gray-800/20 dark:border-gray-700 '
  }

  if (!isRead) {
    base += 'border-l-4 border-l-primary '
  }

  return base
}

// 時間格式化
function formatTimeAgo(timestamp) {
  const now = Math.floor(Date.now() / 1000)
  const diff = now - timestamp

  if (diff < 60) return t.value.justNow
  if (diff < 3600) return `${Math.floor(diff / 60)} ${t.value.minutesAgo}`
  if (diff < 86400) return `${Math.floor(diff / 3600)} ${t.value.hoursAgo}`
  if (diff < 604800) return `${Math.floor(diff / 86400)} ${t.value.daysAgo}`

  const date = new Date(timestamp * 1000)
  return date.toLocaleDateString()
}

// 加載通知
async function loadNotifications(currentCursor) {
  try {
    const isInitialLoad = currentCursor === 0
    if (isInitialLoad) {
      loading.value = true
    } else {
      loadingMore.value = true
    }

    const data = await getNotificationList(currentCursor)

    if (data) {
      if (isInitialLoad) {
        notifications.value = data.list || []
      } else {
        notifications.value = [...notifications.value, ...(data.list || [])]
      }

      cursor.value = data.cursor
      hasMore.value = data.cursor !== -1
    }
  } catch (error) {
    console.error('Failed to load notifications:', error)
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

function loadMore() {
  if (!loadingMore.value && hasMore.value) {
    loadNotifications(cursor.value)
  }
}

// 點擊通知跳轉
function handleNotificationClick(notification) {
  // 針對不同類型跳轉到 Dashboard 對應區域
  switch (notification.type) {
    case 'PurchaseSuccessful':
    case 'TokensReleased':
    case 'ClaimSuccessful':
    case 'ProjectFundingFailed':
    case 'RefundSuccessful':
      router.push('/dashboard')
      break
    default:
      break
  }
}

// 初始化
onMounted(() => {
  if (walletStore.isLoggedIn) {
    loadNotifications(0)
  }
})

// 監聽登入狀態，登入後自動加載
watch(() => walletStore.isLoggedIn, (loggedIn) => {
  if (loggedIn) {
    loadNotifications(0)
  } else {
    notifications.value = []
  }
})

// 監聽語言切換，重新請求數據（後端根據 Jc-Language 返回對應語言的通知）
watch(() => props.language, () => {
  if (walletStore.isLoggedIn) {
    loadNotifications(0)
  }
})
</script>

<template>
  <div class="min-h-screen pt-24 pb-16">
    <div class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 space-y-6">
      <!-- Header -->
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-3xl md:text-4xl font-bold text-gray-900 dark:text-white mb-2">
            {{ t.title }}
          </h1>
          <p v-if="unreadCount > 0" class="text-gray-500 dark:text-gray-400">
            {{ unreadCount }} {{ t.unread }}
          </p>
        </div>
      </div>

      <!-- Not Logged In -->
      <div v-if="!walletStore.isLoggedIn" class="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-800 rounded-2xl p-12 text-center">
        <Bell :size="64" class="mx-auto mb-4 text-gray-300 dark:text-gray-600" />
        <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-2">
          {{ t.loginRequired }}
        </h3>
        <button
          @click="walletStore.connectWallet()"
          class="mt-4 px-6 py-2.5 bg-accent hover:bg-accent-600 text-white rounded-full font-medium transition-all hover:scale-105"
        >
          {{ t.connectWallet }}
        </button>
      </div>

      <!-- Loading -->
      <div v-else-if="loading" class="flex flex-col items-center justify-center py-20 space-y-4">
        <Loader2 :size="48" class="animate-spin text-primary" />
        <p class="text-gray-500 dark:text-gray-400">{{ t.loading }}</p>
      </div>

      <!-- Empty -->
      <div v-else-if="notifications.length === 0" class="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-800 rounded-2xl p-12 text-center">
        <Bell :size="64" class="mx-auto mb-4 text-gray-300 dark:text-gray-600 opacity-50" />
        <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-2">
          {{ t.empty }}
        </h3>
        <p class="text-gray-500 dark:text-gray-400 mb-6">
          {{ t.emptyDesc }}
        </p>
      </div>

      <!-- Notifications List -->
      <div v-else class="space-y-3">
        <div
          v-for="(notification, index) in notifications"
          :key="`${notification.relatedId}-${notification.createdAt}-${index}`"
          :class="getNotificationCardClass(notification.type, notification.isRead)"
          class="cursor-pointer hover:shadow-md hover:border-primary/50"
          @click="handleNotificationClick(notification)"
        >
          <div class="flex items-start gap-4">
            <!-- Icon -->
            <div class="flex-shrink-0 mt-1">
              <component
                :is="getNotificationIcon(notification.type)"
                :size="24"
                :class="getNotificationIconColor(notification.type)"
              />
            </div>

            <!-- Content -->
            <div class="flex-1 min-w-0">
              <div class="flex items-start justify-between gap-2 mb-2">
                <h3 class="font-semibold text-gray-900 dark:text-white">
                  {{ notification.title }}
                </h3>
                <div class="flex items-center gap-1 text-xs text-gray-400 dark:text-gray-500 flex-shrink-0">
                  <Clock :size="12" />
                  {{ formatTimeAgo(notification.createdAt) }}
                </div>
              </div>
              <p class="text-sm text-gray-600 dark:text-gray-400 line-clamp-2">
                {{ notification.content }}
              </p>
            </div>

            <!-- Arrow -->
            <ChevronRight :size="20" class="flex-shrink-0 mt-1 text-gray-400 dark:text-gray-500" />
          </div>
        </div>

        <!-- Load More -->
        <button
          v-if="hasMore"
          @click="loadMore"
          :disabled="loadingMore"
          class="w-full py-4 text-sm font-medium text-primary hover:bg-primary/5 rounded-lg transition-colors disabled:opacity-50"
        >
          <span v-if="loadingMore" class="flex items-center justify-center gap-2">
            <Loader2 :size="16" class="animate-spin" />
            {{ t.loading }}
          </span>
          <span v-else>{{ t.loadMore }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.line-clamp-2 {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
</style>
