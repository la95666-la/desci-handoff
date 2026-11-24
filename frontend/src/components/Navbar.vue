<script setup>
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useWalletStore } from '../stores/wallet'
import { SHOW_TASKS_ENTRY } from '../config/featureVisibility.js'

const props = defineProps({
  theme: { type: String, default: 'light' },
  language: { type: String, default: 'en' },
  hasNotification: { type: Boolean, default: false },
})
const emit = defineEmits(['toggle-theme', 'toggle-language', 'clear-notification'])

const router = useRouter()
const route = useRoute()
const walletStore = useWalletStore()

const mobileOpen = ref(false)
const showWalletMenu = ref(false)

const content = computed(() => {
  // App.vue 只在 'en' / 'tw' 之间切换；非 en 即繁体
  return props.language !== 'en'
    ? {
        overview: '首頁',  ecosystem: '生態',  launchpad: '啟動平臺',
        quest: '任務中心', docs: '文檔',
        connect: '連接錢包', connecting: '連接中…', signing: '簽名驗證中…',
        disconnect: '斷開連接', copy: '複製地址',
      }
    : {
        overview: 'Overview',  ecosystem: 'Ecosystem',  launchpad: 'Launchpad',
        quest: 'Quest Center', docs: 'Docs',
        connect: 'Connect Wallet', connecting: 'Connecting…', signing: 'Sign to verify…',
        disconnect: 'Disconnect', copy: 'Copy Address',
      }
})

const navItems = computed(() => [
  { key: 'overview',  label: content.value.overview,  to: '/',          match: (p) => p === '/' },
  { key: 'ecosystem', label: content.value.ecosystem, to: '/ecosystem', match: (p) => p.startsWith('/ecosystem') },
  { key: 'launchpad', label: content.value.launchpad, to: '/launchpad', match: (p) => p.startsWith('/launchpad') },
  { key: 'quest',     label: content.value.quest,     to: '/tasks',     match: (p) => p.startsWith('/tasks') },
  { key: 'docs',      label: content.value.docs,      to: '/docs',      match: (p) => p.startsWith('/docs') },
].filter((item) => SHOW_TASKS_ENTRY || item.key !== 'quest'))

const DOCS_URL = 'https://joincare.gitbook.io/joincare-docs'

const goto = (to) => {
  mobileOpen.value = false
  if (to === '/docs') {
    window.open(DOCS_URL, '_blank', 'noopener')
    return
  }
  router.push(to)
}

const onConnect = async () => {
  if (walletStore.isConnected) {
    showWalletMenu.value = !showWalletMenu.value
    return
  }
  try { await walletStore.connectWallet() } catch (_) {}
}
const onDisconnect = () => { walletStore.disconnect(); showWalletMenu.value = false }
const onCopy = async () => {
  try { await navigator.clipboard.writeText(walletStore.address || '') } catch (_) {}
  showWalletMenu.value = false
}

const handleClickOutside = (e) => {
  if (!showWalletMenu.value) return
  if (!e.target.closest('[data-wallet-root]')) showWalletMenu.value = false
}
onMounted(() => document.addEventListener('click', handleClickOutside))
onUnmounted(() => document.removeEventListener('click', handleClickOutside))

const buttonLabel = computed(() => {
  if (walletStore.isAuthenticating) return content.value.signing
  if (walletStore.isConnecting) return content.value.connecting
  if (walletStore.isConnected) return walletStore.shortAddress
  return content.value.connect
})
</script>

<template>
  <nav class="sticky top-0 z-50 w-full bg-white/80 backdrop-blur-xl border-b border-gray-200 shadow-sm">
    <div class="flex justify-between items-center px-margin-mobile md:px-margin-desktop h-20 max-w-container-max mx-auto">
      <!-- Logo -->
      <div class="flex items-center gap-4">
        <button class="flex items-center" @click="router.push('/')" aria-label="JoinCare home">
          <img
            src="/brand/joincare-logo-horizontal-green-black.svg"
            alt="JoinCare"
            class="h-6 md:h-7 w-auto select-none"
            draggable="false"
          />
        </button>
      </div>

      <!-- Desktop nav -->
      <div class="hidden md:flex items-center gap-8">
        <a
          v-for="item in navItems" :key="item.key"
          @click.prevent="goto(item.to)"
          href="#"
          :class="[
            'font-label-caps text-label-caps transition-all duration-300 px-2',
            item.match(route.path)
              ? 'text-green-700 font-bold border-b-2 border-green-700 pb-1 rounded-t-sm hover:bg-gray-100'
              : 'text-gray-600 hover:text-green-700 hover:bg-gray-100 rounded-sm',
          ]"
        >
          {{ item.label }}
        </a>
      </div>

      <!-- Right cluster -->
      <div class="flex items-center gap-4">
        <button
          @click="emit('toggle-language')"
          class="flex items-center gap-1 text-gray-700 hover:text-green-700 hover:bg-gray-100 px-2 py-1 rounded-sm transition-all duration-300 font-label-caps text-label-caps"
        >
          <span class="material-symbols-outlined text-[20px]">language</span>
          <span>{{ props.language === 'en' ? 'EN' : '中文' }}</span>
          <span class="material-symbols-outlined text-[16px]">keyboard_arrow_down</span>
        </button>
        <button
          @click="emit('toggle-theme')"
          class="flex items-center justify-center text-gray-700 hover:text-green-700 hover:bg-gray-100 p-2 rounded-full transition-all duration-300"
          :title="props.theme === 'dark' ? 'Switch to light' : 'Switch to dark'"
        >
          <span class="material-symbols-outlined text-[20px]">
            {{ props.theme === 'dark' ? 'dark_mode' : 'light_mode' }}
          </span>
        </button>

        <!-- Connect Wallet -->
        <div class="relative" data-wallet-root>
          <button
            @click="onConnect"
            :disabled="walletStore.isConnecting || walletStore.isAuthenticating"
            class="bg-vibrant-lime text-slate-void px-6 py-2 rounded-full font-label-caps text-label-caps hover:shadow-[0_0_15px_rgba(190,242,100,0.5)] transition-all active:scale-95 duration-150 ease-in-out font-bold disabled:opacity-70"
          >
            {{ buttonLabel }}
          </button>
          <div
            v-if="showWalletMenu && walletStore.isConnected"
            class="absolute right-0 mt-2 w-56 rounded-xl bg-white shadow-lg ring-1 ring-gray-200 overflow-hidden"
          >
            <button class="w-full text-left px-4 py-3 hover:bg-gray-50 flex items-center gap-3" @click="onCopy">
              <span class="material-symbols-outlined" style="font-size:18px">content_copy</span>
              <span class="text-sm text-gray-900">{{ content.copy }}</span>
            </button>
            <button class="w-full text-left px-4 py-3 hover:bg-gray-50 flex items-center gap-3 border-t border-gray-100" @click="onDisconnect">
              <span class="material-symbols-outlined text-red-600" style="font-size:18px">logout</span>
              <span class="text-sm text-red-600">{{ content.disconnect }}</span>
            </button>
          </div>
        </div>

        <!-- Mobile toggle -->
        <button
          class="md:hidden h-9 w-9 rounded-full bg-gray-100 inline-flex items-center justify-center text-gray-700"
          @click="mobileOpen = !mobileOpen"
        >
          <span class="material-symbols-outlined" style="font-size:20px">{{ mobileOpen ? 'close' : 'menu' }}</span>
        </button>
      </div>
    </div>

    <!-- Mobile menu -->
    <div v-if="mobileOpen" class="md:hidden border-t border-gray-200 bg-white/95 backdrop-blur-xl">
      <div class="max-w-container-max mx-auto px-margin-mobile py-4 grid gap-1">
        <a
          v-for="item in navItems" :key="item.key"
          @click.prevent="goto(item.to)"
          href="#"
          :class="[
            'text-left py-3 px-3 rounded-lg font-label-caps text-label-caps',
            item.match(route.path) ? 'bg-gray-100 text-green-700 font-bold' : 'text-gray-600',
          ]"
        >
          {{ item.label }}
        </a>
      </div>
    </div>
  </nav>
</template>

<style scoped>
.material-symbols-outlined { font-variation-settings: 'FILL' 0, 'wght' 400, 'GRAD' 0, 'opsz' 24; }
</style>
