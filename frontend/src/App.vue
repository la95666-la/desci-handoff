<script setup>
import { ref, onMounted, provide, watch, computed } from 'vue'
import { useRoute } from 'vue-router'
import Navbar from './components/Navbar.vue'
import HomeFooter from './components/HomeFooter.vue'
import WalletSelector from './components/WalletSelector.vue'
import SplashIntro from './components/SplashIntro.vue'
import MousePixelTrail from './components/MousePixelTrail.vue'
import AnnouncementPopup from './components/AnnouncementPopup.vue'
import ImportantNewsSpotlight from './components/ImportantNewsSpotlight.vue'
import { useWalletStore } from './stores/wallet'
import { setLanguage, getRedPointStatus } from './services/api'
import { useWebSocket } from './composables/useWebSocket'
import { currentFairLaunchLocalTestWatermark } from './services/fairLaunchSubmissionPolicy.js'
import { normalizeUiLanguage } from './utils/language.js'

const theme = ref('light')
const readSavedLanguage = () => {
  try { return normalizeUiLanguage(localStorage.getItem('language')) } catch { return 'en' }
}
const language = ref(readSavedLanguage())
setLanguage(language.value)
const walletStore = useWalletStore()
const route = useRoute()
const fairLaunchLocalTestWatermark = currentFairLaunchLocalTestWatermark()

// 首页风格的路由（自带浮动 navbar / 深色 footer，需隐藏全局 chrome）
// 精确匹配 + /launchpad/* 前缀匹配（涵盖列表、apply、详情）
const HOMEPAGE_STYLE_ROUTES = ['/', '/ecosystem', '/tasks', '/team', '/news', '/dashboard']
const isHomeRoute = computed(() =>
  HOMEPAGE_STYLE_ROUTES.includes(route.path) || route.path.startsWith('/launchpad'),
)

// === 首页风格页面的"亮 / 暗"主题（与 Tailwind .dark 是两套独立机制）===
// 持久化到 localStorage('jc-theme')，并同步把 .home-dark-root class 打到 <html>，
// 这样 body / 视口大屏空白区也变暗，跨页保留
const THEME_KEY = 'jc-theme'
// 默认暗色：localStorage 没值时返回 true
const readSavedTheme = () => {
  try {
    const v = localStorage.getItem(THEME_KEY)
    if (v === 'dark') return true
    if (v === 'light') return false
    return true   // 未设置 → 默认暗色
  } catch { return true }
}
const isHomeDark = ref(readSavedTheme())
// 切换主题：临时给 <html> 打 .home-dark-transition，激活 0.35s 全局颜色过渡
const TRANSITION_MS = 350
let transitionTimer = null
const toggleHomeDark = () => {
  document.documentElement.classList.add('home-dark-transition')
  window.clearTimeout(transitionTimer)
  transitionTimer = window.setTimeout(() => {
    document.documentElement.classList.remove('home-dark-transition')
  }, TRANSITION_MS + 80)
  isHomeDark.value = !isHomeDark.value
}
watch(isHomeDark, (v) => {
  document.documentElement.classList.toggle('home-dark-root', v)
  try { localStorage.setItem(THEME_KEY, v ? 'dark' : 'light') } catch {}
  // 同步 Reown AppKit 模态主题（async 拿避免循环依赖）
  import('./services/reownAppKit').then(({ appKit }) => {
    try { appKit.setThemeMode(v ? 'dark' : 'light') } catch (e) { console.warn('[appKit] setThemeMode failed:', e?.message) }
  })
}, { immediate: true })
provide('isHomeDark', isHomeDark)
provide('toggleHomeDark', toggleHomeDark)

// 通知紅點狀態
const hasNotification = ref(false)

// WebSocket 連接 —— nice-to-have（红点通知），连不上时静默降级
const { isConnected: wsConnected } = useWebSocket({
  onReadPoint: () => { hasNotification.value = true },
  onError: () => { /* 静默：由 WebSocketManager 内部处理重连；连不上不影响核心功能 */ },
  onClose: () => { /* 静默 */ },
})

// 登入後查詢一次紅點狀態
watch(() => walletStore.isLoggedIn, async (loggedIn) => {
  if (loggedIn) {
    try {
      const data = await getRedPointStatus()
      if (data && data.light) {
        hasNotification.value = true
      }
    } catch (e) {
      console.warn('[App] Failed to check red point:', e)
    }
  } else {
    hasNotification.value = false
  }
})

// 進入通知頁時清除紅點
watch(() => route.path, (path) => {
  if (path === '/notifications') {
    hasNotification.value = false
  }
})

const clearNotification = () => {
  hasNotification.value = false
}

const rememberInviteCode = (query) => {
  const inviteCode = query?.invite_code
  if (typeof inviteCode === 'string' && inviteCode.trim()) {
    sessionStorage.setItem('jc_invite_code', inviteCode.trim())
  }
}

onMounted(async () => {
  // Check for saved theme preference or default to the Figma-aligned light theme.
  const savedTheme = localStorage.getItem('theme') || 'light'
  const savedLanguage = readSavedLanguage()
  theme.value = savedTheme
  language.value = savedLanguage
  localStorage.setItem('language', savedLanguage)
  updateTheme(savedTheme)
  setLanguage(savedLanguage)

  // 邀请码必须**先**存 sessionStorage —— autoConnect 会触发 register，
  // register 时 wallet.js 会从 sessionStorage 读 jc_invite_code 送 backend
  rememberInviteCode(route.query)

  // 嘗試使用已有 token 自動登入（刷新頁面時恢復登入狀態）
  await walletStore.tryAutoLogin()

  // Auto-connect wallet if previously connected
  await walletStore.autoConnect()
})

watch(() => route.query.invite_code, () => {
  rememberInviteCode(route.query)
})

const toggleTheme = () => {
  theme.value = theme.value === 'dark' ? 'light' : 'dark'
  updateTheme(theme.value)
  localStorage.setItem('theme', theme.value)
}

// 切换：EN ↔ TW（首页风格页面 navbar 走这条；其它非首页页面用旧 zh 由用户手动设置 localStorage）
const toggleLanguage = () => {
  language.value = language.value === 'en' ? 'tw' : 'en'
  localStorage.setItem('language', language.value)
  setLanguage(language.value)
  walletStore.updateLanguage(language.value)
}

const updateTheme = (newTheme) => {
  if (newTheme === 'dark') {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}

// Provide theme and language to all child components
provide('theme', theme)
provide('language', language)
provide('toggleTheme', toggleTheme)
provide('toggleLanguage', toggleLanguage)
provide('hasNotification', hasNotification)
provide('clearNotification', clearNotification)

// 錢包選擇器事件處理
const onWalletSelected = (walletType) => {
  walletStore.onWalletSelected(walletType)
}

const onWalletSelectionCancelled = () => {
  walletStore.onWalletSelectionCancelled()
}
</script>

<template>
  <SplashIntro />
  <MousePixelTrail />
  <div v-if="fairLaunchLocalTestWatermark" class="fair-launch-local-watermark" role="status" aria-live="polite">
    {{ fairLaunchLocalTestWatermark }}
  </div>
  <AnnouncementPopup :language="language" />
  <ImportantNewsSpotlight :language="language" />
  <div :class="['joincare-app-shell min-h-screen text-gray-900 dark:text-gray-100 transition-colors duration-300', isHomeRoute ? 'bg-transparent overflow-x-clip is-home' : 'bg-white dark:bg-gray-950 overflow-x-hidden']">
    <Navbar
      v-if="!isHomeRoute"
      :theme="theme"
      :language="language"
      :hasNotification="hasNotification"
      @toggle-theme="toggleTheme"
      @toggle-language="toggleLanguage"
      @clear-notification="clearNotification"
    />
    <router-view v-slot="{ Component }">
      <component :is="Component" :language="language" />
    </router-view>
    <HomeFooter v-if="!isHomeRoute" :language="language" :tailwind-dark="theme === 'dark'" />

    <!-- Reown AppKit 是正式钱包入口；自研选择器仅保留为未挂载 fallback。 -->
    <!--
    <WalletSelector
      v-model:visible="walletStore.showWalletSelector"
      :language="language"
      :mode="walletStore.walletSelectorMode"
      @select="onWalletSelected"
    />
    -->

    <!-- 首页风格页面：右侧 0.382 位置的亮/暗切换标签（sun / moon 图标） -->
    <button
      v-if="isHomeRoute"
      class="theme-toggle-tab"
      :class="{ 'is-dark': isHomeDark }"
      @click="toggleHomeDark"
      :aria-label="isHomeDark ? 'Switch to light mode' : 'Switch to dark mode'"
      :title="isHomeDark ? 'Light mode' : 'Dark mode'"
    >
      <svg v-if="isHomeDark" class="ttt-icon" viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
        <circle cx="12" cy="12" r="4.2" fill="currentColor"/>
        <g stroke="currentColor" stroke-width="1.9" stroke-linecap="round">
          <line x1="12" y1="2.5" x2="12" y2="5.4"/>
          <line x1="12" y1="18.6" x2="12" y2="21.5"/>
          <line x1="2.5" y1="12" x2="5.4" y2="12"/>
          <line x1="18.6" y1="12" x2="21.5" y2="12"/>
          <line x1="5.2" y1="5.2" x2="7.3" y2="7.3"/>
          <line x1="16.7" y1="16.7" x2="18.8" y2="18.8"/>
          <line x1="5.2" y1="18.8" x2="7.3" y2="16.7"/>
          <line x1="16.7" y1="7.3" x2="18.8" y2="5.2"/>
        </g>
      </svg>
      <svg v-else class="ttt-icon" viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
        <path d="M21 13.5a8.5 8.5 0 1 1-10.5-10.5 7 7 0 0 0 10.5 10.5z" fill="currentColor"/>
      </svg>
    </button>
  </div>
</template>

<style>
.fair-launch-local-watermark {
  position: fixed;
  inset: 0 0 auto 0;
  z-index: 2147483647;
  padding: 7px 12px;
  background: #b91c1c;
  color: #fff;
  border-bottom: 2px solid #fecaca;
  font: 800 12px/1.3 ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  letter-spacing: 0.08em;
  text-align: center;
  pointer-events: none;
}

/* 防御性覆盖：首页强制 shell 无 transform/filter，
   否则 SplashIntro 残留的全局样式会把 position:fixed 的 navbar 锁在 shell 内部 */
.joincare-app-shell.is-home {
  opacity: 1 !important;
  transform: none !important;
  filter: none !important;
  animation: none !important;
  visibility: visible !important;
}
</style>
