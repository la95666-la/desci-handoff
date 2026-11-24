<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick, inject } from 'vue'
import { useRouter } from 'vue-router'
import JcFloatingNavbar from '../components/JcFloatingNavbar.vue'
import HomeFooter from '../components/HomeFooter.vue'
import { getNewsList } from '../services/api'
import { useIsMobile } from '../composables/useIsMobile'
const { isMobile } = useIsMobile()

const props = defineProps({ language: { type: String, default: 'en' } })
const router = useRouter()
const isDark = inject('isHomeDark', ref(false))

// ===== 1440 画布缩放（与首页 / LaunchpadList 一致）=====
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
  hostHeight.value = `${naturalHeight.value * scale.value}px`
}

// ===== 滚动初现（与首页一致）=====
const scrollRevealHandler = () => {
  const els = document.querySelectorAll('[data-reveal]:not(.is-revealed)')
  if (!els.length) return
  const trigger = window.innerHeight * 0.80
  els.forEach((el) => {
    if (el.getBoundingClientRect().top < trigger) el.classList.add('is-revealed')
  })
}

// ===== 文案（EN / 繁中）=====
const COPY = {
  en: {
    kicker: 'JoinCare Media',
    h1L1: 'News, essays &',
    h1L2: 'field notes.',
    sub: 'Updates from the JoinCare platform, our incubated projects, and the wider DeSci ecosystem.',
    listKicker: 'All updates',
    loading: 'Loading news…',
    empty: 'No news published yet. Check back soon.',
    error: 'Failed to load news. Please try again later.',
    external: 'External',
    viewArticle: 'Read',
  },
  tw: {
    kicker: 'JoinCare 媒體',
    h1L1: '新聞、文章與',
    h1L2: '現場筆記。',
    sub: '來自 JoinCare 平台、孵化項目與更廣泛 DeSci 生態的最新動態。',
    listKicker: '全部動態',
    loading: '正在載入新聞…',
    empty: '暫無已發布的新聞，敬請期待。',
    error: '新聞載入失敗，請稍後再試。',
    external: '外部來源',
    viewArticle: '閱讀',
  },
}
const t = computed(() => COPY[props.language === 'en' ? 'en' : 'tw'])

// ===== 数据 =====
const items = ref([])
const loading = ref(true)
const failed = ref(false)

const backendLanguage = (lang) => (lang === 'tw' ? 'tw' : 'en')

async function loadNews() {
  loading.value = true
  failed.value = false
  try {
    const res = await getNewsList({ language: backendLanguage(props.language), limit: 50 })
    items.value = (res && res.items) || []
  } catch (e) {
    console.warn('[NewsList] load failed:', e?.message)
    failed.value = true
    items.value = []
  } finally {
    loading.value = false
  }
}

function formatDate(value) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const locale = props.language === 'tw' ? 'zh-TW' : 'en-US'
  try {
    return new Intl.DateTimeFormat(locale, { year: 'numeric', month: 'short', day: 'numeric' }).format(date)
  } catch (_) {
    return date.toISOString().slice(0, 10)
  }
}

function openItem(item) {
  if (item.source_url) {
    window.open(item.source_url, '_blank', 'noopener')
    return
  }
  if (item.id) router.push(`/news/${item.id}`)
}

onMounted(async () => {
  window.scrollTo(0, 0)
  document.title = props.language === 'tw' ? 'JoinCare 媒體中心' : 'JoinCare Media'
  nextTick(() => { measure(); scrollRevealHandler() })
  window.addEventListener('resize', applyScale)
  window.addEventListener('resize', scrollRevealHandler)
  window.addEventListener('scroll', scrollRevealHandler, { passive: true })

  await loadNews()
  // 内容到位后重测画布高度
  setTimeout(() => { measure(); scrollRevealHandler() }, 300)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', applyScale)
  window.removeEventListener('resize', scrollRevealHandler)
  window.removeEventListener('scroll', scrollRevealHandler)
})

watch(() => props.language, async () => {
  document.title = props.language === 'tw' ? 'JoinCare 媒體中心' : 'JoinCare Media'
  await loadNews()
  setTimeout(() => { measure(); scrollRevealHandler() }, 300)
})
</script>

<template>
  <JcFloatingNavbar :scale="scale" active="" />

  <!-- ========== MOBILE ========== -->
  <div v-if="isMobile" class="jc-mobile-page m-news" :class="{ 'home-dark': isDark }">
    <section class="m-hero">
      <div class="m-eyebrow">
        <span class="m-eyebrow-tag"></span>
        <span>{{ t.kicker }}</span>
      </div>
      <h1 class="m-hero-title">
        <span class="font-semibold">{{ t.h1L1 }}</span>
        <span class="font-playfair italic font-normal"> {{ t.h1L2 }}</span>
      </h1>
      <p class="m-hero-sub">{{ t.sub }}</p>
    </section>

    <section class="m-section">
      <div class="m-section-head">
        <span class="m-eyebrow-tag"></span>
        <h2 class="m-section-title">{{ t.listKicker }}</h2>
      </div>

      <p v-if="loading" class="m-news-state">{{ t.loading }}</p>
      <p v-else-if="failed" class="m-news-state">{{ t.error }}</p>
      <p v-else-if="!items.length" class="m-news-state">{{ t.empty }}</p>

      <div v-else class="m-news-list">
        <article
          v-for="item in items"
          :key="item.id"
          class="m-news-card"
          role="link"
          tabindex="0"
          @click="openItem(item)"
          @keydown.enter="openItem(item)"
        >
          <div class="m-news-cover">
            <img v-if="item.cover_url" :src="item.cover_url" :alt="item.title" loading="lazy" />
            <span v-if="item.source_url" class="m-news-external">{{ t.external }} ↗</span>
          </div>
          <div class="m-news-body">
            <div class="m-news-meta">
              <span v-if="item.category" class="m-news-category">{{ item.category }}</span>
              <span>{{ formatDate(item.publish_at) }}</span>
            </div>
            <h3 class="m-news-title">{{ item.title }}</h3>
            <p v-if="item.summary" class="m-news-summary">{{ item.summary }}</p>
            <div class="m-news-foot">
              <span class="m-news-author">{{ item.author || item.source_name || 'JoinCare' }}</span>
              <span class="m-news-read">{{ t.viewArticle }} →</span>
            </div>
          </div>
        </article>
      </div>
    </section>

    <HomeFooter :language="props.language" />
  </div>

  <!-- ========== DESKTOP（1440 canvas 等比缩放，与首页一致）========== -->
  <div v-else class="jc-canvas-host" :class="{ 'home-dark': isDark }" :style="{ height: hostHeight }">
    <div class="jc-scaled-wrapper" :style="{ width: `${1440 * scale}px`, height: hostHeight }">
      <main ref="canvasRef" class="jc-canvas-1440" :style="{ transform: `scale(${scale})` }">

        <div class="h-[64px] w-full shrink-0"></div>

        <section data-reveal class="relative w-full px-[80px] pt-[60px] pb-[120px] flex flex-col gap-[40px]">
          <!-- eyebrow -->
          <div class="flex items-center gap-[8px]">
            <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
            <span class="text-[20px] font-bold text-[#212121]">{{ t.kicker }}</span>
          </div>

          <!-- 大标题（首页同款 88px 两行）-->
          <div class="font-bold text-[#212121] tracking-[-2.88px]" style="font-size: 88px; line-height: 0.95; width: 1180px;">
            <p class="mb-0">
              <span class="font-semibold">{{ t.h1L1 }}</span>
            </p>
            <p class="font-semibold mb-0">
              <span class="font-playfair italic font-normal">{{ t.h1L2 }}</span>
            </p>
          </div>

          <!-- 副文案 -->
          <p class="text-[18px] font-bold text-[#212121] leading-[1.4]" style="width: 540px;">{{ t.sub }}</p>

          <!-- 分隔线 -->
          <div class="pointer-events-none mt-[8px] mb-[4px]" style="height: 1px; background: linear-gradient(to right, transparent 0%, rgba(0,0,0,0.08) 12%, rgba(0,0,0,0.08) 88%, transparent 100%);"></div>

          <!-- 列表小标题行 -->
          <div class="flex items-center justify-between w-full">
            <span class="text-[13px] font-bold tracking-[1.5px] uppercase text-[#808080]">{{ t.listKicker }}</span>
            <span v-if="!loading && !failed" class="text-[13px] font-bold text-[#808080]">{{ items.length }}</span>
          </div>

          <!-- 状态 -->
          <p v-if="loading" class="news-state text-[16px] text-[#808080]">{{ t.loading }}</p>
          <p v-else-if="failed" class="news-state text-[16px] text-[#808080]">{{ t.error }}</p>
          <p v-else-if="!items.length" class="news-state text-[16px] text-[#808080]">{{ t.empty }}</p>

          <!-- 新闻卡片网格（3 列，lp-card 同款白卡风格）-->
          <div v-else class="flex flex-wrap gap-[20px] items-stretch w-full">
            <article
              v-for="item in items"
              :key="item.id"
              class="news-card-d bg-white flex flex-col rounded-[16px] overflow-hidden relative cursor-pointer"
              style="width: 413px; height: 500px;"
              role="link"
              tabindex="0"
              @click="openItem(item)"
              @keydown.enter="openItem(item)"
            >
              <div class="relative h-[220px] w-full overflow-hidden bg-[#f0efec]">
                <img
                  v-if="item.cover_url"
                  :src="item.cover_url"
                  :alt="item.title"
                  class="w-full h-full object-cover news-card-cover"
                  loading="lazy"
                />
                <span
                  v-if="item.source_url"
                  class="absolute top-[12px] right-[12px] px-[10px] py-[4px] rounded-full text-[12px] font-bold"
                  style="background: rgba(24,24,24,0.82); color: #9bff00;"
                >{{ t.external }} ↗</span>
              </div>
              <div class="flex flex-col gap-[10px] p-[26px] flex-1">
                <div class="flex items-center gap-[10px] text-[12px] tracking-[0.6px] uppercase text-[#808080]">
                  <span v-if="item.category" class="news-category-chip">{{ item.category }}</span>
                  <span>{{ formatDate(item.publish_at) }}</span>
                </div>
                <h3 class="news-title-d text-[20px] font-bold text-[#212121] leading-[1.35] mb-0">{{ item.title }}</h3>
                <p v-if="item.summary" class="news-summary-d text-[14px] text-[#808080] leading-[1.6] mb-0">{{ item.summary }}</p>
                <div class="mt-auto flex items-center justify-between gap-[12px] pt-[8px]">
                  <span class="text-[13px] font-bold text-[#212121] truncate">{{ item.author || item.source_name || 'JoinCare' }}</span>
                  <span class="text-[13px] font-bold text-[#212121] whitespace-nowrap">{{ t.viewArticle }} →</span>
                </div>
              </div>
            </article>
          </div>
        </section>

        <HomeFooter :language="props.language" />

      </main>
    </div>
  </div>
</template>

<style scoped>
/* ===== 桌面卡片细节 ===== */
.news-card-d {
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.05);
  transition: transform 0.25s ease, box-shadow 0.25s ease;
}
.news-card-d:hover,
.news-card-d:focus-visible {
  transform: translateY(-4px);
  box-shadow: 0 14px 36px rgba(0, 0, 0, 0.1);
  outline: none;
}
.news-card-cover { transition: transform 0.4s ease; }
.news-card-d:hover .news-card-cover { transform: scale(1.04); }

.news-title-d {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.news-summary-d {
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.news-category-chip {
  padding: 3px 10px;
  border-radius: 999px;
  background: rgba(155, 255, 0, 0.22);
  color: #3f6600;
  font-weight: 700;
}

/* 桌面暗色（bg-white/text-[#212121] 由 jc-shell.css 全局反色，这里只补自定义元素）*/
.home-dark .news-card-d { box-shadow: 0 4px 24px rgba(0, 0, 0, 0.35); }
.home-dark .news-card-d:hover,
.home-dark .news-card-d:focus-visible { box-shadow: 0 14px 36px rgba(0, 0, 0, 0.5); }
.home-dark .news-category-chip {
  background: rgba(155, 255, 0, 0.14);
  color: #9bff00;
}
.home-dark .news-summary-d { color: #b8b8b0; }

/* ===== 移动端 ===== */
.m-news-list { display: flex; flex-direction: column; gap: 16px; }
.m-news-card {
  background: white;
  border-radius: 16px;
  overflow: hidden;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.05);
  cursor: pointer;
}
.m-news-cover {
  position: relative;
  aspect-ratio: 16 / 9;
  background: #ecebe6;
  overflow: hidden;
}
.m-news-cover img { width: 100%; height: 100%; object-fit: cover; display: block; }
.m-news-external {
  position: absolute;
  top: 10px;
  right: 10px;
  padding: 3px 9px;
  border-radius: 999px;
  background: rgba(24, 24, 24, 0.82);
  color: #9bff00;
  font-size: 11px;
  font-weight: 700;
}
.m-news-body { display: flex; flex-direction: column; gap: 8px; padding: 18px; }
.m-news-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  letter-spacing: 0.6px;
  text-transform: uppercase;
  color: #8a8a84;
}
.m-news-category {
  padding: 2px 9px;
  border-radius: 999px;
  background: rgba(155, 255, 0, 0.22);
  color: #3f6600;
  font-weight: 700;
}
.m-news-title {
  font-size: 17px;
  font-weight: 700;
  line-height: 1.4;
  margin: 0;
  color: #181818;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.m-news-summary {
  font-size: 13px;
  line-height: 1.6;
  color: #5c5c56;
  margin: 0;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.m-news-foot {
  margin-top: 4px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
.m-news-author {
  font-size: 12px;
  font-weight: 700;
  color: #333;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.m-news-read { font-size: 12px; font-weight: 700; color: #181818; white-space: nowrap; }
.m-news-state { padding: 48px 0; text-align: center; color: #6b6b6b; font-size: 14px; }

/* 移动端暗色 */
.home-dark .m-news-card { background: #1c1c1c; box-shadow: 0 4px 20px rgba(0, 0, 0, 0.35); }
.home-dark .m-news-title { color: #fafaf7; }
.home-dark .m-news-summary { color: #b8b8b0; }
.home-dark .m-news-author { color: #e8e8e2; }
.home-dark .m-news-read { color: #fafaf7; }
.home-dark .m-news-category { background: rgba(155, 255, 0, 0.14); color: #9bff00; }
.home-dark .m-news-state { color: #b8b8b0; }
</style>
