<script setup>
import { computed, inject, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowUpRight, X } from 'lucide-vue-next'
import { getImportantNewsAlert } from '../services/api'
import { appKit } from '../services/reownAppKit'

const props = defineProps({
  language: { type: String, default: 'en' }
})

const route = useRoute()
const router = useRouter()
const isHomeDark = inject('isHomeDark', ref(true))
const legacyTheme = inject('theme', ref('light'))
const alert = ref(null)
const visible = ref(false)
const walletModalOpen = ref(false)
let showTimer = null
let readinessTimer = null
let unsubscribeWalletModal = null

const copy = computed(() => props.language === 'en'
  ? {
      eyebrow: 'Important update',
      read: 'Read story',
      dismiss: 'Dismiss',
      notToday: 'Not today',
      later: 'Maybe later',
      close: 'Close important news'
    }
  : {
      eyebrow: '重要消息',
      read: '閱讀全文',
      dismiss: '不再提醒',
      notToday: '今日不再提醒',
      later: '稍後再看',
      close: '關閉重要消息'
    })

const secondaryLabel = computed(() => {
  if (alert.value?.frequency === 'once') return copy.value.dismiss
  if (alert.value?.frequency === 'daily') return copy.value.notToday
  return copy.value.later
})

const normalizeLanguage = (value) => value === 'en' ? 'en' : 'tw'

function routeGroup(path) {
  if (path === '/') return 'home'
  if (path.startsWith('/ecosystem')) return 'ecosystem'
  if (path.startsWith('/launchpad/apply')) return ''
  if (path.startsWith('/launchpad')) return 'launchpad'
  if (path.startsWith('/tasks')) return 'tasks'
  if (path.startsWith('/team')) return 'team'
  if (path.startsWith('/news')) return 'news'
  return ''
}

const isHomeStylePage = computed(() =>
  ['/', '/ecosystem', '/tasks', '/team'].includes(route.path)
  || route.path.startsWith('/launchpad')
)

const isDarkAppearance = computed(() =>
  isHomeStylePage.value ? isHomeDark.value : legacyTheme.value === 'dark'
)

function storageKey(item) {
  return `jc_news_spotlight_${item.id}_v${item.revision || 1}`
}

function todayKey() {
  const now = new Date()
  return `${now.getFullYear()}-${now.getMonth() + 1}-${now.getDate()}`
}

function alreadySeen(item) {
  try {
    const key = storageKey(item)
    if (item.frequency === 'session') return sessionStorage.getItem(key) === '1'
    if (item.frequency === 'daily') return localStorage.getItem(key) === todayKey()
    return localStorage.getItem(key) === '1'
  } catch {
    return false
  }
}

function markSeen(item) {
  if (!item) return
  try {
    const key = storageKey(item)
    if (item.frequency === 'session') sessionStorage.setItem(key, '1')
    else if (item.frequency === 'daily') localStorage.setItem(key, todayKey())
    else localStorage.setItem(key, '1')
  } catch {
    // Private browsing may disable storage; closing still works for this render.
  }
}

function isAllowedOnCurrentPage(item) {
  if (!item) return false
  const group = routeGroup(route.path)
  if (!group) return false
  const pages = Array.isArray(item.pages) ? item.pages : []
  if (!pages.includes('all') && !pages.includes(group)) return false
  return route.path !== `/news/${item.id}`
}

function pageIsReady() {
  if (document.hidden || walletModalOpen.value) return false
  if (!document.body.classList.contains('is-ready')) return false
  if (document.body.classList.contains('is-navigating')) return false
  return !document.querySelector('[aria-modal="true"]')
}

function clearTimers() {
  window.clearTimeout(showTimer)
  window.clearTimeout(readinessTimer)
  showTimer = null
  readinessTimer = null
}

function revealWhenReady(attempt = 0) {
  if (!alert.value || alreadySeen(alert.value) || !isAllowedOnCurrentPage(alert.value)) return
  if (pageIsReady()) {
    visible.value = true
    return
  }
  if (attempt < 40) {
    readinessTimer = window.setTimeout(() => revealWhenReady(attempt + 1), 500)
  }
}

function schedule() {
  clearTimers()
  visible.value = false
  if (!alert.value || alreadySeen(alert.value) || !isAllowedOnCurrentPage(alert.value)) return
  const delay = Math.max(0, Math.min(15, Number(alert.value.delay_seconds) || 0)) * 1000
  showTimer = window.setTimeout(() => revealWhenReady(), delay)
}

async function load() {
  try {
    alert.value = await getImportantNewsAlert({ language: normalizeLanguage(props.language) })
  } catch (error) {
    alert.value = null
    console.warn('[ImportantNews] load failed', error?.message)
  }
  schedule()
}

function close() {
  markSeen(alert.value)
  visible.value = false
}

function openStory() {
  const target = alert.value?.target_url
  markSeen(alert.value)
  visible.value = false
  if (!target) return
  if (/^https?:\/\//i.test(target)) {
    window.open(target, '_blank', 'noopener,noreferrer')
  } else {
    router.push(target)
  }
}

onMounted(() => {
  unsubscribeWalletModal = appKit.subscribeState?.((state) => {
    walletModalOpen.value = state?.open === true
    if (!walletModalOpen.value && !visible.value) schedule()
  })
  load()
})

watch(() => props.language, load)
watch(() => route.path, schedule)

onBeforeUnmount(() => {
  clearTimers()
  unsubscribeWalletModal?.()
})
</script>

<template>
  <Transition name="news-spotlight">
    <aside
      v-if="alert && visible"
      class="news-spotlight"
      :class="{ 'news-spotlight--light': !isDarkAppearance }"
      role="dialog"
      aria-modal="false"
      :aria-labelledby="`news-spotlight-title-${alert.id}`"
    >
      <div v-if="alert.cover_url" class="news-spotlight__media">
        <img :src="alert.cover_url" :alt="alert.title" />
      </div>

      <div class="news-spotlight__content">
        <div class="news-spotlight__eyebrow">{{ copy.eyebrow }}</div>
        <h2 :id="`news-spotlight-title-${alert.id}`">{{ alert.title }}</h2>
        <p v-if="alert.summary">{{ alert.summary }}</p>
        <div class="news-spotlight__actions">
          <button type="button" class="news-spotlight__later" @click="close">
            {{ secondaryLabel }}
          </button>
          <button type="button" class="news-spotlight__primary" @click="openStory">
            {{ copy.read }}
            <ArrowUpRight :size="18" aria-hidden="true" />
          </button>
        </div>
      </div>

      <button
        type="button"
        class="news-spotlight__close"
        :aria-label="copy.close"
        :title="copy.close"
        @click="close"
      >
        <X :size="18" aria-hidden="true" />
      </button>
    </aside>
  </Transition>
</template>

<style scoped>
.news-spotlight {
  position: fixed;
  z-index: 8200;
  right: 24px;
  bottom: 24px;
  width: min(430px, calc(100vw - 48px));
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.14);
  border-radius: 8px;
  background: #171717;
  color: #f7f7f5;
  box-shadow: 0 22px 55px rgba(0, 0, 0, 0.36);
}

.news-spotlight__media {
  width: 100%;
  aspect-ratio: 16 / 7;
  overflow: hidden;
  background: #090909;
}

.news-spotlight__media img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.news-spotlight__content {
  padding: 20px;
}

.news-spotlight__eyebrow {
  margin-bottom: 8px;
  color: #9bff00;
  font-size: 11px;
  font-weight: 750;
  letter-spacing: 0;
  text-transform: uppercase;
}

.news-spotlight h2 {
  margin: 0;
  padding-right: 28px;
  font-size: 22px;
  line-height: 1.2;
  letter-spacing: 0;
  font-weight: 720;
}

.news-spotlight p {
  margin: 10px 0 0;
  color: rgba(247, 247, 245, 0.72);
  font-size: 14px;
  line-height: 1.55;
}

.news-spotlight__actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
  margin-top: 18px;
}

.news-spotlight__actions button,
.news-spotlight__close {
  border: 0;
  cursor: pointer;
  font: inherit;
}

.news-spotlight__later {
  padding: 10px 12px;
  background: transparent;
  color: rgba(247, 247, 245, 0.72);
  font-size: 13px;
  font-weight: 650;
}

.news-spotlight__primary {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  min-height: 42px;
  padding: 10px 15px;
  border-radius: 6px;
  background: #9bff00;
  color: #121212;
  font-size: 13px;
  font-weight: 750;
}

.news-spotlight__close {
  position: absolute;
  top: 10px;
  right: 10px;
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.68);
  color: #fff;
}

.news-spotlight__later:hover { color: #fff; }
.news-spotlight__primary:hover { background: #b3ff3e; }
.news-spotlight__close:hover { background: #000; }

.news-spotlight--light {
  border-color: rgba(19, 30, 22, 0.14);
  background: #fff;
  color: #182019;
  box-shadow: 0 22px 55px rgba(20, 35, 24, 0.18);
}

.news-spotlight--light p,
.news-spotlight--light .news-spotlight__later {
  color: rgba(24, 32, 25, 0.68);
}

.news-spotlight--light .news-spotlight__later:hover { color: #182019; }

.news-spotlight-enter-active,
.news-spotlight-leave-active {
  transition: opacity 0.22s ease, transform 0.28s ease;
}

.news-spotlight-enter-from,
.news-spotlight-leave-to {
  opacity: 0;
  transform: translateY(18px);
}

@media (max-width: 640px) {
  .news-spotlight {
    right: 10px;
    bottom: max(10px, env(safe-area-inset-bottom));
    width: calc(100vw - 20px);
    max-height: min(78vh, 620px);
    overflow-y: auto;
  }

  .news-spotlight__media { aspect-ratio: 16 / 8; }
  .news-spotlight__content { padding: 17px; }
  .news-spotlight h2 { font-size: 20px; }
  .news-spotlight__actions button { min-height: 44px; }
}

@media (prefers-reduced-motion: reduce) {
  .news-spotlight-enter-active,
  .news-spotlight-leave-active { transition: opacity 0.01ms linear; }
}
</style>
