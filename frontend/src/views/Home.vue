<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick, inject, watch } from 'vue'
import { useRouter } from 'vue-router'
// 3D 动效组件由 admin Banner 配置决定（dandelion / dna / none）
import DandelionModel from '../components/DandelionModel.vue'
import DnaModel from '../components/DnaModel.vue'
import JcFloatingNavbar from '../components/JcFloatingNavbar.vue'
import PixelGrid from '../components/PixelGrid.vue'
import { useWalletStore } from '../stores/wallet'
import { TRANSLATIONS } from '../i18n/home.js'
import { getNewsList, getCurrentBanner, getTaskList, getProjectList } from '../services/api'
import { useIsMobile } from '../composables/useIsMobile'
import { isLaunchpadVisibleStatus } from '../utils/projectStatus'
import { localizedProjectName } from '../utils/projectLocalization.js'
import { pickHeroFeaturedNews } from '../utils/heroFeaturedNews.js'
import { SHOW_TASKS_ENTRY } from '../config/featureVisibility.js'
const { isMobile } = useIsMobile()

const props = defineProps({ language: { type: String, default: 'en' } })
const router = useRouter()
const wallet = useWalletStore()

const handleConnect = async () => {
  try { await wallet.connectWallet() } catch (e) { console.warn('[connect]', e) }
}
const METHODOLOGY_URL = 'https://joincare.gitbook.io/joincare-docs/incubation-model-and-project-workflow'
const goToSubmitProject = () => { router.push('/launchpad/apply') }
const goToProject = () => { router.push('/launchpad') }
const goToMethodology = () => { window.open(METHODOLOGY_URL, '_blank', 'noopener') }

// Temporarily hidden for launch. Set to true to restore the Partners section
// on both mobile and desktop without reintroducing its markup or styles.
const SHOW_PARTNERS = false

const partners = [
  { name: 'Al Ghurair', href: 'https://www.alghurair.com', logo: '/partners/al-ghurair.svg' },
  { name: 'Animoca Ventures', href: 'https://animoca.ventures', logo: '/partners/animoca.png' },
  { name: 'World Health Organization', href: 'https://www.who.int', logo: '/partners/who.svg' },
  { name: 'University of Southern California', href: 'https://www.usc.edu', logo: '/partners/usc.svg' },
  { name: 'University of Freiburg', href: 'https://uni-freiburg.de/en', logo: '/partners/freiburg.svg' },
  { name: 'DPE', href: 'https://www.dpe.de/', logo: '/partners/dpe.svg' },
  { name: 'BankM', href: 'https://www.bankm.de/', logo: '/partners/bankm.webp' },
]

// Footer 社群链接 —— tg URL 从后端任务配置拉（与任务中心 tg chat 任务同源）
// footer 想指向社区讨论群（chat），不是主账号频道（join）
const telegramUrl = ref('https://t.me/joincare_chat')
async function loadTelegramUrl() {
  try {
    const res = await getTaskList({ platform: 'telegram' })
    const list = Array.isArray(res) ? res : (res?.list || [])
    const chat = list.find(t => t.task_key === 'chat' || t.key === 'chat')
    const join = list.find(t => t.task_key === 'join' || t.key === 'join')
    const target = chat || join || list[0]
    if (target?.url) telegramUrl.value = target.url
  } catch (e) { /* 静默：回退到 fallback URL */ }
}

// Featured Projects —— 与 Launchpad 同源，展示整个公开生命周期（含终结项目）。
const featuredProjects = ref([])
let featuredProjectsRequest = 0
async function loadFeaturedProjects() {
  const requestID = ++featuredProjectsRequest
  const requestLanguage = props.language
  featuredProjects.value = []
  try {
    const resp = await getProjectList?.()
    if (requestID !== featuredProjectsRequest) return
    const list = Array.isArray(resp) ? resp : (resp?.list || [])
    const isZh = requestLanguage && requestLanguage !== 'en'
    const pickTag = (p) => (isZh && p.taglineZh) ? p.taglineZh : (p.tagline || '')
    const picks = list.filter(p => isLaunchpadVisibleStatus(p.status))
    featuredProjects.value = picks.map((p, i) => ({
      projectID: p.projectID,
      chip:  p.category || 'DeSci',
      title: localizedProjectName(p, requestLanguage),
      desc:  pickTag(p),
      img:   p.coverUrl || (i === 0 ? '/figma/v2/project-card-image-3.png' : '/figma/v2/project-card-image-4.png'),
    }))
  } catch (e) {
    if (requestID === featuredProjectsRequest) console.warn('[Home] featured projects load failed', e?.message)
  }
}
const openFeatured = (p) => { if (p.projectID) router.push(`/launchpad/${p.projectID}`) }

// 手动管理 FP 卡片 hover 状态 —— 浏览器在 transform:scale 的 flex 子项 + auto-scroll
// 容器下 hit-test 会错位，CSS :hover 无法可靠触发；用 @mouseenter/@mouseleave 直接绑
const hoveredFpIndex = ref(-1)
const onFpEnter = (i) => { hoveredFpIndex.value = i }
const onFpLeave = (i) => { if (hoveredFpIndex.value === i) hoveredFpIndex.value = -1 }

// 主题状态从 App.vue 注入（toggle 按钮也在 App.vue 全局渲染）
const isDark = inject('isHomeDark', ref(false))

// === Banner 配置（含 3D 动效选择）===
// 后台 admin 配置当前 banner（按语种/排期/sort 取最优），缺省 fallback 蒲公英
const currentBanner = ref({
  title: '', subtitle: '', cta_label: '', cta_link: '',
  image_url: '', hero_animation: 'dandelion', language: 'en'
})
const heroComponent = computed(() => {
  switch (currentBanner.value.hero_animation) {
    case 'dandelion': return DandelionModel
    case 'dna':       return DnaModel
    case 'none':      return null
    default:          return DandelionModel
  }
})
async function loadBanner() {
  try {
    const data = await getCurrentBanner({ language: props.language || 'en' })
    if (data && typeof data === 'object') {
      currentBanner.value = { ...currentBanner.value, ...data }
    }
  } catch (e) {
    console.warn('[Home] banner load failed (fallback to dandelion)', e?.message)
  }
}
watch(() => props.language, () => {
  loadBanner()
  loadLatestMedia()
  loadFeaturedProjects()
})

// === 1440 canvas scaling logic ===
const canvasRef = ref(null)
const scale = ref(1)
const hostHeight = ref('auto')
const naturalHeight = ref(0)

// === Entrance "settle" effect: 自下而上 — translateY(40)+scale(0.94) → translateY(0)+scale(1.0) ===
const entranceScale = ref(0.94)
const entranceY = ref(40)              // px，正值=比最终位置低
const enableEntranceTransition = ref(true)
// 进入方式：true = 路由切换进来（应用快速 hero 入场，不再等 1.9s 的 splash 节奏）
const isNavEntry = ref(false)

const DESIGN_WIDTH = 1440
const MIN_SCALE = 0.35    // 最小缩到 35%（防止移动端太挤）
const MAX_SCALE = 1.25    // 视口 ≥ 1800px 时封顶，大屏自然居中带留白

const measure = () => {
  if (!canvasRef.value) return
  // measure at scale=1 first
  const prevTransform = canvasRef.value.style.transform
  canvasRef.value.style.transform = 'scale(1)'
  naturalHeight.value = canvasRef.value.scrollHeight
  canvasRef.value.style.transform = prevTransform
  applyScale()
}

// stretch 模式：画布始终按视口宽度等比缩放，无论比 1440 大还是小
const applyScale = () => {
  const w = window.innerWidth
  const raw = w / DESIGN_WIDTH
  scale.value = Math.min(Math.max(raw, MIN_SCALE), MAX_SCALE)
  hostHeight.value = `${naturalHeight.value * scale.value}px`
}

onMounted(() => {
  // 拉首页 Latest Media（后端有新闻则覆盖 i18n 静态文案；失败静默回退）
  loadLatestMedia()
  // 拉首页 Banner 配置（hero_animation 决定 3D 组件，文案后续可覆盖 i18n）
  loadBanner()
  // Footer 社群 tg URL 从任务中心配置读（与 TG 任务同源）
  loadTelegramUrl()
  // Featured Projects 从后端 project list 拉（与 Launchpad 同源）
  loadFeaturedProjects()
  // 强制顶到顶：measure() 会把文档高度撑起来，浏览器此时可能延迟恢复滚动；再保险一次
  window.scrollTo(0, 0)
  nextTick(() => {
    measure()
    window.scrollTo(0, 0)
    updateFeaturedState()
    measureDarkStrips()
    measureFeaturedCarousel()
    // 初次扫描 + 注册 scroll/resize 监听
    scrollRevealHandler()
    setTimeout(scrollRevealHandler, 100)
    setTimeout(scrollRevealHandler, 500)
  })
  window.addEventListener('resize', applyScale)
  window.addEventListener('resize', updateFeaturedState)
  window.addEventListener('resize', measureDarkStrips)
  window.addEventListener('resize', measureFeaturedCarousel)
  window.addEventListener('scroll', scrollRevealHandler, { passive: true })
  window.addEventListener('resize', scrollRevealHandler)
  setTimeout(() => { measure(); updateFeaturedState(); measureDarkStrips(); measureFeaturedCarousel() }, 500)

  // 路由切换进入（vs 首次刷新）：body.is-navigating 还在 = 由 router curtain 触发的
  // 这种情况下幕布已经覆盖屏幕 900ms+，幕布退出时 hero 必须已经就位 ——
  // 跳过 entrance scale/translate 动画，直接落在最终值；并立刻关掉 transition，
  // 否则幕布升到一半时 canvas 还在缩放，会看到"放大到一半→卡一下→继续"
  isNavEntry.value = document.body.classList.contains('is-navigating')
  if (isNavEntry.value) {
    enableEntranceTransition.value = false
    entranceScale.value = 1.0
    entranceY.value = 0
    setTimeout(() => { measureDarkStrips(); measureFeaturedCarousel() }, 100)
  } else {
    // 首次刷新：保留原 splash → entrance 节奏
    setTimeout(() => {
      entranceScale.value = 1.0
      entranceY.value = 0
    }, 1900)
    setTimeout(() => {
      enableEntranceTransition.value = false
      measureDarkStrips()
      measureFeaturedCarousel()
    }, 3000)
  }
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', applyScale)
  window.removeEventListener('resize', updateFeaturedState)
  window.removeEventListener('resize', measureDarkStrips)
  window.removeEventListener('resize', measureFeaturedCarousel)
  window.removeEventListener('scroll', scrollRevealHandler)
  window.removeEventListener('resize', scrollRevealHandler)
})

// === Asset base ===
const A = '/figma/v2'
const img = (n) => `${A}/${n}`

// === Navigation ===
const goLaunchpad   = () => router.push('/launchpad')
const goEcosystem   = () => router.push('/ecosystem')
const goTeam        = () => router.push('/team')
const goNews        = () => router.push('/news')

// === 滚动初现：section 顶部进入视口下 80% 时触发 ===
const scrollRevealHandler = () => {
  const els = document.querySelectorAll('[data-reveal]:not(.is-revealed)')
  if (!els.length) return
  const trigger = window.innerHeight * 0.80
  els.forEach((el) => {
    if (el.getBoundingClientRect().top < trigger) el.classList.add('is-revealed')
  })
}

// === Featured Projects 轮播 ===
const featuredCarouselRef = ref(null)
const canScrollFeaturedPrev = ref(false)
const canScrollFeaturedNext = ref(true)
const SCROLL_STEP_BASE = 556   // 一个大卡片宽（540）+ 间距（16）pre-scale

const updateFeaturedState = () => {
  const el = featuredCarouselRef.value
  if (!el) return
  canScrollFeaturedPrev.value = el.scrollLeft > 2
  canScrollFeaturedNext.value = el.scrollLeft + el.clientWidth < el.scrollWidth - 2
}
const scrollFeaturedPrev = () => {
  // 卡片视觉宽 = base * scale，按视觉宽滚动
  const step = SCROLL_STEP_BASE * scale.value
  featuredCarouselRef.value?.scrollBy({ left: -step, behavior: 'smooth' })
}
const scrollFeaturedNext = () => {
  const step = SCROLL_STEP_BASE * scale.value
  featuredCarouselRef.value?.scrollBy({ left: step, behavior: 'smooth' })
}

// === Featured Projects 轮播：渲染在 canvas-host 层，全视口宽 ===
const totalCardsNatural = computed(() => {
  const count = featuredProjects.value.length
  return count > 0 ? count * 540 + Math.max(0, count - 1) * 16 : 0
})
const carouselSlotRef = ref(null)
const featuredCarousel = ref({ top: 0, firstCardLeftPx: 0, ready: false })

const measureFeaturedCarousel = () => {
  if (!canvasRef.value || !carouselSlotRef.value) return
  const canvasRect = canvasRef.value.getBoundingClientRect()
  const slotRect = carouselSlotRef.value.getBoundingClientRect()
  featuredCarousel.value = {
    top: slotRect.top - canvasRect.top,
    // 首卡左缘 = canvas 视觉左 + 80*scale（与上方 headline 对齐）
    firstCardLeftPx: canvasRect.left + 80 * scale.value,
    ready: true,
  }
}

// === 深色 section 的全视口背景条带（避免 transform 影响） ===
const partnersRef = ref(null)
const footerRef = ref(null)
const darkStripPartners = ref({ top: 0, height: 0, ready: false })
const darkStripFooter = ref({ top: 0, height: 0, ready: false })

const measureDarkStrips = () => {
  if (!canvasRef.value) return
  const canvasRect = canvasRef.value.getBoundingClientRect()
  if (partnersRef.value) {
    const r = partnersRef.value.getBoundingClientRect()
    darkStripPartners.value = {
      top: r.top - canvasRect.top,
      height: r.height,
      ready: true,
    }
  }
  if (footerRef.value) {
    const r = footerRef.value.getBoundingClientRect()
    darkStripFooter.value = {
      top: r.top - canvasRect.top,
      height: r.height,
      ready: true,
    }
  }
}

const t = computed(() => TRANSLATIONS[props.language] || TRANSLATIONS.en)
const traditionalList = computed(() => t.value.traditionalList)
const joinCareList = computed(() => t.value.joinCareList)

// Section 显隐（走后端 copywriting override）：
// 后台把对应 key 的值清空 或 填 "Disable" (不区分大小写) → 前端隐藏整个 section
// 恢复：后台把值填回原文（如 "Research collective"）即可
function sectionEnabled(v) {
  const s = (v || '').trim()
  return s !== '' && s.toLowerCase() !== 'disable'
}
const showTeamSection = computed(() => sectionEnabled(t.value.teamEyebrow))

// ============ Latest Media — 从后端动态拉，媒体区沿用 i18n fallback；Hero 不回退 ============
const remoteMediaCards = ref(null) // null=未加载/失败 → 走 fallback；array=已加载
let latestMediaRequestVersion = 0

function langToBackend(lang) {
  if (lang === 'tw') return 'tw'
  if (lang === 'cn') return 'zh-cn'
  return 'en'
}

async function loadLatestMedia() {
  const requestVersion = ++latestMediaRequestVersion
  remoteMediaCards.value = null
  try {
    const res = await getNewsList({
      language: langToBackend(props.language),
      featured: true,
      limit: 6
    })
    const items = (res && res.items) || []
    if (requestVersion !== latestMediaRequestVersion) return
    remoteMediaCards.value = items.slice(0, 6)
  } catch (e) {
    if (requestVersion !== latestMediaRequestVersion) return
    // 静默回退到 i18n 静态文案
    console.warn('[Home] loadLatestMedia failed, using fallback:', e?.message)
  }
}

function newsTarget(news) {
  if (news?.source_url) return news.source_url
  return news?.id ? `/news/${news.id}` : ''
}

function openNewsTarget(target) {
  if (!target) return
  if (/^https?:\/\//i.test(target)) {
    window.open(target, '_blank', 'noopener')
    return
  }
  router.push(target)
}

const heroNewsCard = computed(() => pickHeroFeaturedNews(remoteMediaCards.value))

const openHeroNews = () => openNewsTarget(heroNewsCard.value?.target)

const mediaCards = computed(() => {
  if (remoteMediaCards.value && remoteMediaCards.value.length > 0) {
    return remoteMediaCards.value.slice(0, 3).map(n => ({
      title: n.title,
      author: n.author || n.source_name,
      role: n.author_role,
      url: newsTarget(n),
      cover: n.cover_url
    }))
  }
  // i18n 静态 fallback
  return [
    { title: t.value.mediaCard1Title, author: t.value.mediaCard1Author, role: t.value.mediaCard1Role, url: t.value.mediaCard1Url, cover: '/figma/v2/news/joincare-token2049.jpg' },
    { title: t.value.mediaCard2Title, author: t.value.mediaCard2Author, role: t.value.mediaCard2Role, url: t.value.mediaCard2Url, cover: '/figma/v2/news/innoblock-2024.webp' },
    { title: t.value.mediaCard3Title, author: t.value.mediaCard3Author, role: t.value.mediaCard3Role, url: t.value.mediaCard3Url, cover: '/figma/v2/news/jiuge-p2x1.jpg' }
  ]
})
</script>

<template>
  <!-- 浮动 navbar：抽到共享组件 -->
  <JcFloatingNavbar :scale="scale * entranceScale" active="home" />

  <!-- ============================================================
       MOBILE HOME · 独立布局（非缩放版桌面 canvas）
       ============================================================ -->
  <div v-if="isMobile" class="jc-mobile-page m-home" :class="{ 'home-dark': isDark }">
    <!-- Hero -->
    <section class="m-hero" data-reveal>
      <!-- 3D 蒲公英：单株，作为 hero 右侧背景装饰；不遮挡文字（低 z-index + right offset） -->
      <div v-if="heroComponent" class="m-hero-dandelion">
        <DandelionModel :single-stalk="true" :model-scale="0.34" />
      </div>
      <div class="m-eyebrow">
        <span class="m-eyebrow-tag"></span>
        <span>{{ t.heroEyebrow }}</span>
      </div>
      <h1 class="m-hero-title">
        <span class="m-hero-title-line-1">
          <span class="font-semibold">{{ t.heroFundWord }}</span>
          <span class="font-playfair italic font-normal">{{ t.heroSciencePhrase }}</span>
        </span>
        <span class="m-hero-title-line-2 font-semibold">{{ t.heroLine2 }}</span>
      </h1>
      <p class="m-hero-sub">{{ t.heroSubtitle }}</p>
      <div class="m-hero-ctas">
        <button class="m-btn-primary" @click="goToSubmitProject">
          {{ t.submitProject || 'Submit Project' }}
          <span class="material-symbols-outlined">arrow_forward</span>
        </button>
        <button class="m-btn-secondary" @click="goToProject">
          {{ t.ctaProject || 'Explore Launchpad' }}
        </button>
      </div>
    </section>

    <!-- Featured Projects -->
    <section v-if="featuredProjects.length" class="m-section" data-reveal>
      <div class="m-section-head">
        <span class="m-eyebrow-tag"></span>
        <h2 class="m-section-title">{{ t.fpKicker || 'FEATURED PROJECTS' }}</h2>
      </div>
      <div class="m-fp-list">
        <article v-for="(p, i) in featuredProjects" :key="p.projectID || i" class="m-fp-card" @click="openFeatured(p)">
          <div class="m-fp-img-frame">
            <img :src="p.img" alt="" class="m-fp-img" />
          </div>
          <div class="m-fp-body">
            <span class="m-fp-cat">{{ p.chip }}</span>
            <h3 class="m-fp-title">{{ p.title }}</h3>
            <p class="m-fp-desc">{{ p.desc }}</p>
            <div class="m-fp-cta">{{ t.viewDetail }} →</div>
          </div>
        </article>
      </div>
    </section>

    <!-- Incubation Model -->
    <section class="m-section" data-reveal>
      <div class="m-section-head">
        <span class="m-eyebrow-tag"></span>
        <h2 class="m-section-title">{{ t.incubEyebrow || 'INCUBATION MODEL' }}</h2>
      </div>
      <h3 class="m-section-headline">
        <span class="font-semibold">{{ t.incubHeadL1 || 'A better way to' }}</span>
        <span class="font-semibold"> {{ t.incubHeadImprove || 'improve' }} </span>
        <span class="font-playfair italic font-normal">{{ t.incubHeadROI || 'research ROI' }}</span>
      </h3>
      <p class="m-section-copy">{{ t.incubRightCopy || 'JoinCare turns lab-stage biotech into shippable, community-owned assets — from data to CDMO.' }}</p>
      <div class="m-incub-steps">
        <div class="m-incub-card">
          <span class="m-incub-index">01</span>
          <p class="m-incub-title">{{ t.incubStep1 || 'Select' }}</p>
          <p class="m-incub-desc">{{ t.incubStep1Desc || 'We screen only DeSci-ready programs with published data and clear IP.' }}</p>
        </div>
        <div class="m-incub-card">
          <span class="m-incub-index">02</span>
          <p class="m-incub-title">{{ t.incubStep2 || 'Incubate' }}</p>
          <p class="m-incub-desc">{{ t.incubStep2Desc || 'Legal, tokenomics, CRO/CDMO and regulatory partners plugged in from day one.' }}</p>
        </div>
        <div class="m-incub-card">
          <span class="m-incub-index">03</span>
          <p class="m-incub-title">{{ t.incubStep3 || 'Launch' }}</p>
          <p class="m-incub-desc">{{ t.incubStep3Desc || 'Public sale via the Launchpad with points, whitelist and post-listing support.' }}</p>
        </div>
      </div>
    </section>

    <!-- Dream Team (Research collective) —— 后台 teamEyebrow 清空/填 Disable 时整段隐藏 -->
    <section v-if="showTeamSection" class="m-section" data-reveal>
      <div class="m-section-head">
        <span class="m-eyebrow-tag"></span>
        <h2 class="m-section-title">{{ t.teamEyebrow || 'THE PEOPLE' }}</h2>
      </div>
      <h3 class="m-section-headline">
        <span>{{ t.teamHeadL1 || 'Built by a' }} </span>
        <span class="font-playfair italic font-normal">{{ t.teamHeadWorldClass || 'world-class' }}</span>
        <span> {{ t.teamHeadL2 || 'DeSci team' }}</span>
      </h3>
      <div class="m-team-photos">
        <img :src="img('portrait-1.png')" alt="" />
        <img :src="img('portrait-2.png')" alt="" />
        <img :src="img('portrait-3.png')" alt="" />
      </div>
      <div class="m-team-tags">
        <span>Genomics</span>
        <span>Immunology</span>
        <span>Bioinformatics</span>
        <span>Longevity</span>
        <span>Oncology</span>
        <span>Regulatory</span>
      </div>
      <p class="m-section-copy">{{ t.teamRightCopy || 'Scientists, regulators and DeSci operators from Harvard, A*STAR, and biotech-native funds.' }}</p>
    </section>

    <!-- Latest Media -->
    <section class="m-section" data-reveal>
      <div class="m-section-head">
        <span class="m-eyebrow-tag"></span>
        <h2 class="m-section-title">{{ t.mediaEyebrow || 'LATEST MEDIA' }}</h2>
      </div>
      <h3 class="m-section-headline">
        <span class="font-semibold">{{ t.mediaHeadPart1 || 'News, essays &' }}</span>
        <span class="font-playfair italic font-normal">{{ t.mediaHeadPart2 || 'field notes' }}</span>
      </h3>
      <div class="m-media-list">
        <a v-for="(card, idx) in mediaCards" :key="idx" :href="card.url"
           target="_blank" rel="noopener" class="m-media-card">
          <img :src="card.cover" alt="" class="m-media-cover" />
          <div class="m-media-body">
            <p class="m-media-title">{{ card.title }}</p>
            <div class="m-media-author">
              <span class="media-source-badge">›</span>
              <div>
                <p class="m-media-name">{{ card.author }}</p>
                <p class="m-media-role">{{ card.role }}</p>
              </div>
            </div>
          </div>
        </a>
      </div>
      <button class="m-btn-secondary" type="button" @click="$router.push('/news')">
        {{ t.viewAllNews }}
        <span class="material-symbols-outlined" aria-hidden="true">arrow_outward</span>
      </button>
    </section>

    <!-- Comparison -->
    <section class="m-section m-compare" data-reveal>
      <div class="m-section-head">
        <span class="m-eyebrow-tag"></span>
        <h2 class="m-section-title">{{ t.compareEyebrow || 'THE COMPARISON' }}</h2>
      </div>
      <h3 class="m-section-headline m-compare-headline">
        <span class="font-playfair italic font-normal">{{ t.compareHeadPart1 || 'Traditional biotech' }}</span>
        <span class="font-bold not-italic"> {{ t.compareHeadVs || 'vs.' }} </span>
        <span class="font-semibold not-italic">{{ t.compareHeadJoinCare || 'JoinCare' }}</span>
      </h3>
      <p class="m-section-copy">{{ t.compareIntro || 'Why the DeSci route is faster, cheaper and better aligned for scientists and communities.' }}</p>
      <div class="m-compare-col">
        <p class="m-compare-col-title">{{ t.compareTradHeader || 'Traditional' }}</p>
        <div v-for="(row, i) in traditionalList" :key="'t'+i" class="m-compare-row m-compare-row-neg">
          <span class="m-compare-icon">−</span>
          <p>{{ row }}</p>
        </div>
      </div>
      <div class="m-compare-col">
        <p class="m-compare-col-title">{{ t.compareJoinHeader || 'JoinCare' }}</p>
        <div v-for="(row, i) in joinCareList" :key="'j'+i" class="m-compare-row m-compare-row-pos">
          <span class="m-compare-icon">+</span>
          <p>{{ row }}</p>
        </div>
      </div>
    </section>

    <!-- Partners -->
    <section v-if="SHOW_PARTNERS" class="m-section m-partners-section" data-reveal>
      <div class="m-section-head">
        <span class="m-eyebrow-tag"></span>
        <h2 class="m-section-title">{{ t.partnersEyebrow || 'PARTNERS' }}</h2>
      </div>
      <h3 class="m-section-headline m-partners-headline">
        <span>{{ t.partnersHeadPart1 || 'Backed by leading' }} </span>
        <span class="font-playfair italic font-normal">{{ t.partnersHeadPart2 || 'DeSci networks' }}</span>
      </h3>
      <p class="m-partners-copy">{{ t.partnersCopy || 'Working alongside the DAOs, protocols and funds that already move DeSci forward.' }}</p>
      <div class="m-partners-grid m-partners-logos">
        <a v-for="partner in partners" :key="partner.name" :href="partner.href" target="_blank" rel="noopener">
          <img :src="partner.logo" :alt="partner.name" />
        </a>
      </div>
    </section>

    <!-- Footer —— 走 i18n key，与桌面版共享后台管理配置 -->
    <footer class="m-footer">
      <img src="/figma/v2/footer-logo.svg" alt="JoinCare" class="m-footer-logo" />
      <div class="m-footer-links">
        <div v-if="t.footerLabSG && t.footerLabSG.trim()">
          <p class="m-footer-label">{{ t.footerLabSG }}</p>
          <p class="m-footer-value">{{ t.footerLabSGLine1 }}<template v-if="t.footerLabSGLine2"><br />{{ t.footerLabSGLine2 }}</template></p>
        </div>
        <div v-if="t.footerResBos && t.footerResBos.trim()">
          <p class="m-footer-label">{{ t.footerResBos }}</p>
          <p class="m-footer-value">{{ t.footerResBosLine1 }}<template v-if="t.footerResBosLine2"><br />{{ t.footerResBosLine2 }}</template></p>
        </div>
        <div v-if="t.footerContact && t.footerContact.trim()">
          <p class="m-footer-label">{{ t.footerContact }}</p>
          <p class="m-footer-value">{{ t.footerContactLine1 }}<template v-if="t.footerContactLine2"><br />{{ t.footerContactLine2 }}</template></p>
        </div>
      </div>
      <p class="m-footer-copy">{{ t.footerCopyright || `© ${new Date().getFullYear()} joincare · Open biomedical infrastructure` }}</p>
    </footer>
  </div>

  <!-- ============================================================
       DESKTOP HOME · 原有 1440 canvas 设计（isMobile=false 时渲染）
       ============================================================ -->
  <div v-else class="canvas-host" :class="{ 'home-dark': isDark }" :style="{ height: hostHeight }">
    <!-- 深色条带：JS 测量后定位，全视口宽，在 canvas-host 层级绕开 transform 干扰 -->
    <div
      v-if="SHOW_PARTNERS && darkStripPartners.ready"
      class="dark-strip partners-strip"
      :style="{ top: darkStripPartners.top + 'px', height: darkStripPartners.height + 'px', background: '#131313' }"
    >
      <div class="dark-strip-noise" :style="`background-image: url(${img('noise-bg.png')});`"></div>
    </div>
    <div
      v-if="darkStripFooter.ready"
      class="dark-strip footer-strip"
      :style="{ top: darkStripFooter.top + 'px', height: darkStripFooter.height + 'px', background: '#222' }"
    >
      <div class="dark-strip-noise" :style="`background-image: url(${img('noise-bg.png')}); opacity: 0.12;`"></div>
    </div>

    <!-- Featured Projects carousel：渲染在 canvas-host 层级，全视口宽，绕开 transform 链 -->
    <div
      v-if="featuredCarousel.ready && featuredProjects.length"
      data-reveal
      class="featured-carousel-host"
      :style="{ top: featuredCarousel.top + 'px', height: (424 * scale) + 'px' }"
    >
      <div
        ref="featuredCarouselRef"
        @scroll="updateFeaturedState"
        class="featured-carousel-scroll"
        :style="{ height: (424 * scale) + 'px' }"
      >
        <!-- wrap 持有"视觉尺寸"的 layout，让 overflow 计算与视觉一致 -->
        <div
          class="cards-row-wrap"
          :style="{
            paddingLeft: featuredCarousel.firstCardLeftPx + 'px',
            width: (featuredCarousel.firstCardLeftPx + totalCardsNatural * scale) + 'px',
            height: (424 * scale) + 'px',
          }"
        >
          <!-- 内层 transform: scale 缩放视觉 -->
          <div class="cards-row-scaled" :style="{ transform: `scale(${scale})`, transformOrigin: 'top left' }">
            <!-- Featured projects: 仅展示后端 project list 返回的数据 -->
            <article
              v-for="(p, i) in featuredProjects" :key="p.projectID || i"
              class="train-item fp-card shrink-0 bg-white flex flex-col gap-[22px] h-[424px] p-[24px] rounded-[16px] overflow-hidden relative cursor-pointer"
              style="width: 540px;"
              :class="{ 'is-hovered': hoveredFpIndex === i }"
              @mouseenter="onFpEnter(i)" @mouseleave="onFpLeave(i)"
              @click="openFeatured(p)"
            >
              <span class="project-chip">{{ p.chip }}</span>
              <h3 class="text-[28px] font-bold text-[#212121] leading-none" style="width: 476px;">{{ p.title }}</h3>
              <p class="text-[16px] font-medium text-[#212121] leading-[1.45]" style="width: 476px;">{{ p.desc }}</p>
              <div class="absolute overflow-hidden" style="left: 0; top: 237px; width: 540px; height: 187px;">
                <img :src="p.img" alt="" class="fp-project-img" />
              </div>
              <div class="fp-detail-btn absolute right-[16px] bottom-[16px] inline-flex items-center gap-[8px] bg-white rounded-[7px] pl-[14px] pr-[7px] py-[7px]">
                <span class="text-[14px] font-semibold text-[#212121]">{{ t.viewDetail }}</span>
                <span class="arrow-icon arrow-diag">
                  <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                  <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                </span>
              </div>
            </article>

          </div>
        </div>
      </div>
    </div>

    <!-- 3D 模型：抬到 .canvas-host 层级 → 全 viewport 宽（左右出血到屏幕边缘）
         不在 .canvas-1440 内 → 不受 1440px / scaled 约束
         由 admin Banner 配置的 hero_animation 决定渲染哪个组件（none 时不挂载）-->
    <div
      v-if="heroComponent"
      class="dandelion-bleed"
      :style="{ height: (813 * scale) + 'px' }"
    >
      <div class="dna-late-reveal w-full h-full">
        <component :is="heroComponent" />
      </div>
    </div>

    <div class="scaled-wrapper" :style="{ width: `${1440 * scale}px`, height: hostHeight }">
    <main
      ref="canvasRef"
      class="canvas-1440"
      :class="{ 'with-entrance-transition': enableEntranceTransition, 'fast-hero-enter': isNavEntry }"
      :style="{ transform: `translateY(${entranceY}px) scale(${scale * entranceScale})` }"
    >
      <!-- 背景与噪点已移至 .canvas-host，覆盖大屏留白区 -->

      <!-- Spacer：占住浮动 navbar 的位置 -->
      <div class="h-[64px] w-full shrink-0"></div>

      <!-- ============ HERO (h: 749) ============ -->
      <section class="relative w-full h-[749px] overflow-hidden">

        <!-- Eyebrow -->
        <div class="hero-anim hero-anim-1 absolute z-10 flex items-center justify-center gap-[8px]" style="left: 80px; top: 50px;">
          <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
          <span class="text-[20px] font-bold text-[#212121] leading-none">{{ t.heroEyebrow }}</span>
        </div>

        <!-- Big headline -->
        <div
          class="absolute z-10 font-bold text-[#212121] tracking-[-2.52px]"
          style="left: 80px; top: 110px; width: 1100px; font-size: 84px; line-height: .95;"
        >
          <p class="mb-0 headline-line headline-line-1">
            <span class="font-semibold">{{ t.heroFundWord }}</span>
            <span class="font-playfair italic font-normal">{{ t.heroSciencePhrase }}</span>
          </p>
          <p class="font-semibold headline-line headline-line-2">{{ t.heroLine2 }}</p>
        </div>

        <!-- Hero CTA pills：Connect Wallet + Join Airdrop -->
        <div class="hero-anim hero-anim-3 absolute z-10 flex items-center gap-[14px]"
             style="left: 80px; top: 470px;">
          <button v-if="!wallet.address" class="hero-cta-pill hero-cta-primary" @click="handleConnect">
            <span>{{ t.connectWallet }}</span>
          </button>
          <button class="hero-cta-pill hero-cta-dark" @click="goToSubmitProject">
            <span class="hero-cta-label">{{ t.submitProject || 'Submit Project' }}</span>
            <span class="arrow-icon arrow-diag">
              <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
              <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
            </span>
          </button>
        </div>

        <!-- Subtitle + Discover CTA -->
        <div class="hero-anim hero-anim-3 absolute z-10 flex items-end" style="left: 80px; bottom: 40px; width: 1200px; height: 140px;">
          <div class="flex flex-col items-start gap-[28px]">
            <p class="text-[18px] font-bold text-[#212121] leading-[1.4]" style="width: 470px;">{{ t.heroSubtitle }}</p>
            <button class="discover-btn">
              <span class="text-[15px] font-bold text-[#212121]">{{ t.discoverMore }}</span>
              <span class="arrow-icon arrow-vert">
                <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-a.svg')})` }"></span>
                <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-a.svg')})` }"></span>
              </span>
            </button>
          </div>
        </div>

        <!-- Case study card (floating, right-bottom) -->
        <article
          v-if="heroNewsCard"
          class="case-study-card absolute z-20 bg-white flex h-[184px] rounded-[5px] shadow-[0px_8px_32px_0px_rgba(0,0,0,0.08)] overflow-hidden"
          :class="{ 'cursor-pointer': heroNewsCard.target }"
          :role="heroNewsCard.target ? 'link' : undefined"
          :tabindex="heroNewsCard.target ? 0 : undefined"
          style="right: 80px; bottom: 40px; width: 348px;"
          @click="openHeroNews"
          @keydown.enter.prevent="openHeroNews"
          @keydown.space.prevent="openHeroNews"
        >
          <div class="flex flex-col gap-[12px] p-[18px] pb-[22px] flex-1 justify-between">
            <p class="text-[13px] font-bold leading-[1.3] text-[#212121]" style="width: 135px;">{{ heroNewsCard.title }}</p>
            <div class="flex items-center justify-between gap-[8px]">
              <p class="text-[13px] text-black tracking-[1px] uppercase whitespace-nowrap">{{ t.viewDetail }}</p>
              <span class="arrow-icon arrow-diag shrink-0">
                <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
              </span>
            </div>
          </div>
          <!-- 正方形图片遮罩：148×148 + m-[18px] 四周留白，卡片高 184 = 148+18*2 -->
          <div class="w-[148px] h-[148px] m-[18px] overflow-hidden rounded-[4px] shrink-0">
            <img :src="heroNewsCard.cover" :alt="heroNewsCard.title" class="w-full h-full object-cover" />
          </div>
        </article>

        <!-- Hero 底部细线：作为 3D 模型的视觉地平线，淡灰、低存在感 -->
        <div class="hero-horizon absolute left-0 right-0 bottom-0 z-10 pointer-events-none" style="height: 1px; background: linear-gradient(to right, transparent 0%, rgba(0,0,0,0.08) 12%, rgba(0,0,0,0.08) 88%, transparent 100%);"></div>
      </section>

      <!-- divider 浮层：Hero → Featured Projects（高度 0，浮在两段之间，不占流） -->
      <!-- 生命方块下移到 STORYTEQ 卡片位置（右下）；之前 top:-580 在 hero 偏上 -->
      <div class="relative w-full" style="height: 0;">
        <div class="absolute pointer-events-none z-10" style="top: -210px; right: 60px;">
          <PixelGrid :cols="12" :rows="12" cell-size="14px" gap="0" color="#9bff00"
                     :flash-duration="450" :idle-min="900" :idle-max="2800"
                     :start-stagger-max="2800" :jitter="0" :jitter-rotate="0" :jitter-scale="0" />
        </div>
      </div>

      <!-- ============ FEATURED PROJECTS ============ -->
      <section
        data-reveal
        class="relative w-full flex flex-col gap-[56px] pl-[80px] pt-[120px] pb-[120px]"
      >
        <!-- FP 区方块组：水平居中、上方偏上 -->
        <div class="absolute pointer-events-none z-10" style="top: 60px; left: 50%; transform: translateX(-50%);">
          <PixelGrid :cols="12" :rows="12" cell-size="14px" gap="0" color="#9bff00"
                     :flash-duration="450" :idle-min="900" :idle-max="2800"
                     :start-stagger-max="2800" :jitter="0" :jitter-rotate="0" :jitter-scale="0" />
        </div>

        <!-- Eyebrow -->
        <div class="fp-anim fp-anim-1 flex items-center gap-[8px]">
          <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px]" style="transform: rotate(15deg);"></span>
          <span class="text-[20px] font-bold text-[#212121]">{{ t.fpEyebrow }}</span>
        </div>

        <!-- Headline + right text + cta -->
        <div class="flex gap-[64px] items-start">
          <div class="font-bold text-[#212121] tracking-[-1.28px]" style="font-size: 64px; line-height: 1; width: 760px;">
            <p class="reveal-headline-line reveal-headline-line-1 mb-0">
              <span class="font-semibold">{{ t.fpHeadDiscover }}</span>
              <span class="font-playfair italic font-normal">{{ t.fpHeadImpact }}</span>
            </p>
            <p class="reveal-headline-line reveal-headline-line-2 font-semibold">{{ t.fpHeadLine2 }}</p>
          </div>
          <div class="fp-anim fp-anim-3 flex flex-col gap-[24px]">
            <p class="text-[16px] font-medium leading-[1.45] text-[#212121]" style="width: 320px;">{{ t.fpRightCopy }}</p>
            <button class="white-pill-btn w-[218px]" @click="goToProject">
              <span class="text-[14px] font-light text-[#212121] flex-1">{{ t.viewMethodology }}</span>
              <span class="arrow-icon arrow-diag">
                <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
              </span>
            </button>
          </div>
        </div>

        <!-- Carousel：跳出 section 的 pl-[80px]，跨满整个 canvas 宽 -->
        <div class="fp-anim fp-anim-4 flex flex-col gap-[24px] -ml-[80px]" style="width: calc(100% + 80px);">
          <div class="flex items-center justify-end gap-[12px] pr-[80px] w-full">
            <button
              @click="scrollFeaturedPrev"
              :disabled="!canScrollFeaturedPrev"
              :class="['carousel-btn', canScrollFeaturedPrev ? 'bg-[#212121]' : 'bg-[#c9c9c9] cursor-not-allowed']"
            >
              <img :src="img('carousel-prev-arrow.svg')" alt="" />
            </button>
            <button
              @click="scrollFeaturedNext"
              :disabled="!canScrollFeaturedNext"
              :class="['carousel-btn', canScrollFeaturedNext ? 'bg-[#212121]' : 'bg-[#c9c9c9] cursor-not-allowed']"
            >
              <img :src="img('carousel-next-arrow.svg')" alt="" />
            </button>
          </div>

          <!-- 占位：实际 carousel 渲染在 canvas-host 层级（全视口宽）-->
          <div ref="carouselSlotRef" class="h-[424px] w-full"></div>
        </div>
      </section>

      <!-- divider 浮层：Featured Projects → Incubation Model（偏左） -->
      <div class="relative w-full" style="height: 0;">
        <div class="absolute pointer-events-none z-10" style="top: -40px; left: 160px;">
          <PixelGrid :cols="10" :rows="10" cell-size="14px" gap="0" color="#9bff00"
                     :flash-duration="450" :idle-min="900" :idle-max="2800"
                     :start-stagger-max="2800" :jitter="0" :jitter-rotate="0" :jitter-scale="0" />
        </div>
      </div>

      <!-- ============ INCUBATION MODEL ============ -->
      <section data-reveal class="relative w-full flex flex-col gap-[64px] px-[80px] py-[120px]">
        <div class="flex items-center gap-[8px]">
          <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px]" style="transform: rotate(15deg);"></span>
          <span class="text-[20px] font-bold text-[#212121]">{{ t.incubEyebrow }}</span>
        </div>

        <div class="flex gap-[64px] items-start relative">
          <div class="font-bold text-[#212121] tracking-[-1.28px]" style="font-size: 64px; line-height: 1; width: 740px;">
            <p class="reveal-headline-line reveal-headline-line-1 font-semibold mb-0">{{ t.incubHeadL1 }}</p>
            <p class="reveal-headline-line reveal-headline-line-2">
              <span class="font-semibold">{{ t.incubHeadImprove }}</span>
              <span class="font-playfair italic font-normal">{{ t.incubHeadROI }}</span>
            </p>
          </div>
          <div class="flex flex-col items-start">
            <p class="text-[16px] font-medium text-[#212121] leading-[1.45]" style="width: 360px;">{{ t.incubRightCopy }}</p>
          </div>
          <button class="absolute white-pill-btn h-[50px]" style="left: 804px; top: 133px; width: 224px;" @click="goToMethodology">
            <span class="text-[14px] font-light text-[#212121] flex-1">{{ t.viewMethodology }}</span>
            <span class="arrow-icon arrow-diag">
              <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
              <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
            </span>
          </button>
        </div>

        <div class="incub-grid w-full">
          <div class="incub-card scale-pop scale-pop-1 bg-white flex flex-col h-[280px] p-[32px] rounded-[5px] overflow-hidden">
            <p class="text-[24px] font-bold text-[#212121] whitespace-nowrap">{{ t.incubStep1 }}</p>
            <p class="incub-detail text-[14px] font-medium text-[#212121] leading-[1.5] mt-[18px]">{{ t.incubStep1Desc }}</p>
          </div>
          <div class="incub-card scale-pop scale-pop-2 bg-white flex flex-col h-[280px] p-[32px] rounded-[5px] overflow-hidden">
            <p class="text-[24px] font-bold text-[#212121] whitespace-nowrap">{{ t.incubStep2 }}</p>
            <p class="incub-detail text-[14px] font-medium text-[#212121] leading-[1.5] mt-[18px]">{{ t.incubStep2Desc }}</p>
          </div>
          <div class="incub-card scale-pop scale-pop-3 bg-white flex flex-col h-[280px] p-[32px] rounded-[5px] overflow-hidden">
            <p class="text-[24px] font-bold text-[#212121] whitespace-nowrap">{{ t.incubStep3 }}</p>
            <p class="incub-detail text-[14px] font-medium text-[#212121] leading-[1.5] mt-[18px]">{{ t.incubStep3Desc }}</p>
          </div>
        </div>
      </section>

      <!-- ============ DREAM TEAM (h: 1226) —— 后台 teamEyebrow 清空/填 Disable 时整段隐藏 ============ -->
      <section v-if="showTeamSection" data-reveal class="relative w-full flex flex-col gap-[56px] px-[80px] py-[120px]" style="height: 1226px;">
        <!-- 第 4 组方块：Regulatory tag 左侧空白区域 -->
        <div class="absolute pointer-events-none z-10" style="left: 90px; top: 540px;">
          <PixelGrid :cols="12" :rows="12" cell-size="14px" gap="0" color="#9bff00"
                     :flash-duration="450" :idle-min="900" :idle-max="2800"
                     :start-stagger-max="2800" :jitter="0" :jitter-rotate="0" :jitter-scale="0" />
        </div>

        <!-- Floating tags layer -->
        <div class="absolute" style="left: 40px; top: 494px; width: 1360px; height: 540px;">
          <div class="burst-tag burst-tag-1 absolute flex items-center justify-center bg-[#9bff00] rounded-[10px] px-[16px] py-[10px]" style="left: 630px; top: -71px; width: 100px; height: 100px;">
            <span class="text-[14px] font-bold text-[#212121]">Genomics</span>
          </div>
          <div class="burst-tag burst-tag-2 absolute flex items-center justify-center bg-[#9bff00] rounded-[10px] px-[16px] py-[10px]" style="left: 876px; top: 43px; width: 100px; height: 100px;">
            <span class="text-[14px] font-medium text-[#212121]">Immunology</span>
          </div>
          <div class="burst-tag burst-tag-3 absolute flex items-center justify-center bg-[#9bff00] rounded-[10px] px-[16px] py-[10px]" style="left: 876px; top: 478px; width: 100px; height: 100px;">
            <span class="text-[14px] font-medium text-[#212121]">Bioinformatics</span>
          </div>
          <div class="burst-tag burst-tag-4 absolute flex items-center justify-center bg-[#9bff00] rounded-[10px] px-[16px] py-[10px]" style="left: 647px; top: 566px; width: 100px; height: 100px;">
            <span class="text-[14px] font-medium text-[#212121]">Longevity</span>
          </div>
          <div class="burst-tag burst-tag-5 absolute flex items-center justify-center bg-[#9bff00] rounded-[10px] px-[16px] py-[10px]" style="left: 364px; top: 486px; width: 100px; height: 100px;">
            <span class="text-[14px] font-medium text-[#212121]">Oncology</span>
          </div>
          <div class="burst-tag burst-tag-6 absolute flex items-center justify-center bg-[#9bff00] rounded-[10px] px-[16px] py-[10px]" style="left: 364px; top: 43px; width: 100px; height: 100px;">
            <span class="text-[14px] font-medium text-[#212121]">Regulatory</span>
          </div>
        </div>

        <div class="flex items-center gap-[8px] relative z-10">
          <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px]" style="transform: rotate(15deg);"></span>
          <span class="text-[20px] font-bold text-[#212121]">{{ t.teamEyebrow }}</span>
        </div>

        <div class="font-bold text-[#212121] tracking-[-1.28px] relative z-10" style="font-size: 64px; line-height: 1; width: 765px; height: 376px;">
          <p class="reveal-headline-line reveal-headline-line-1 mb-0">
            {{ t.teamHeadL1 }}
            <span class="font-playfair italic font-normal">{{ t.teamHeadWorldClass }}</span>
          </p>
          <p class="reveal-headline-line reveal-headline-line-2">{{ t.teamHeadL2 }}</p>
        </div>

        <div class="flex gap-[32px] items-center justify-center h-[337px] w-full relative z-10">
          <p class="burst-main font-playfair italic text-[#212121] whitespace-nowrap" style="font-size: 112px; line-height: 0.95;">{{ t.teamWordDream }}</p>

          <div class="fan-stack relative" style="width: 440px; height: 320px;">
            <div class="burst-card burst-card-1 fan-right absolute" style="left: 220px; top: 5px; width: 173px; height: 235px;">
              <div class="border-[4px] border-white rounded-[16px] shadow-[0px_8px_22px_-2px_rgba(0,0,0,0.12)] overflow-hidden" style="width: 150px; height: 219px; transform: rotate(6.35deg); transform-origin: center;">
                <img :src="img('portrait-3.png')" alt="" class="w-full h-full object-cover" />
              </div>
            </div>
            <div class="burst-card burst-card-2 fan-left absolute" style="left: 56px; top: 5px; width: 179px; height: 240px;">
              <div class="border-[4px] border-white rounded-[16px] shadow-[0px_8px_22px_-2px_rgba(0,0,0,0.12)] overflow-hidden" style="width: 150px; height: 222px; transform: rotate(-7.78deg); transform-origin: center;">
                <img :src="img('portrait-1.png')" alt="" class="w-full h-full object-cover" />
              </div>
            </div>
            <div class="burst-card burst-card-3 fan-center absolute border-[4px] border-white rounded-[16px] shadow-[0px_8px_22px_-2px_rgba(0,0,0,0.12)] overflow-hidden" style="left: 145px; top: 10px; width: 165px; height: 230px;">
              <img :src="img('portrait-2.png')" alt="" class="w-full h-full object-cover" />
            </div>
          </div>

          <p class="burst-main font-semibold text-[#212121] tracking-[-3.36px]" style="font-size: 112px; line-height: 0.95; width: 340px;">{{ t.teamWordTeam }}</p>
        </div>

        <!-- Meet team button (absolute, anchored to section) -->
        <div class="meet-team-btn absolute white-pill-btn h-[54px] z-30" style="left: 630px; top: 853px; width: 179px;">
          <span class="text-[16px] font-bold text-[#212121] flex-1">{{ t.meetTeam }}</span>
          <span class="arrow-icon arrow-diag">
            <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
            <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
          </span>
        </div>

        <!-- Right-side description text -->
        <p class="absolute text-[18px] font-medium text-[#212121] leading-[1.45]" style="left: 889px; top: 326px; width: 447px; height: 157px;">{{ t.teamRightCopy }}</p>
      </section>

      <!-- ============ LATEST MEDIA (h: 1185) ============ -->
      <section data-reveal class="relative w-full flex flex-col gap-[56px] pl-[80px] pt-[120px] pb-[120px]" style="overflow: hidden;">
        <div class="flex items-center gap-[8px]">
          <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px]" style="transform: rotate(15deg);"></span>
          <span class="text-[20px] font-bold text-[#212121]">{{ t.mediaEyebrow }}</span>
        </div>

        <div class="flex gap-[64px] items-start">
          <p class="reveal-headline font-bold text-[#212121] tracking-[-1.28px]" style="font-size: 64px; line-height: 1; width: 740px;">
            <span class="font-semibold">{{ t.mediaHeadPart1 }}</span>
            <span class="font-playfair italic font-normal">{{ t.mediaHeadPart2 }}</span>
          </p>
          <div class="flex flex-col gap-[24px]">
            <p class="text-[16px] font-medium text-[#212121] leading-[1.45]" style="width: 360px;">{{ t.mediaRightCopy }}</p>
            <button class="white-pill-btn self-start" type="button" @click="$router.push('/news')">
              <span class="text-[14px] font-light text-[#212121]">{{ t.viewAllNews }}</span>
              <span class="arrow-icon arrow-diag">
                <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
              </span>
            </button>
          </div>
        </div>

        <!-- 左右翻页按钮 —— 整体上移，靠近上方描述列；下方卡片随同上移 -->
        <div class="flex items-center justify-end gap-[12px] pr-[80px] w-full" style="margin-top: -88px;">
          <button class="carousel-btn bg-[#c9c9c9]">
            <img :src="img('carousel-prev-arrow.svg')" alt="" />
          </button>
          <button class="carousel-btn bg-[#212121]">
            <img :src="img('carousel-next-arrow.svg')" alt="" />
          </button>
        </div>

        <div class="flex gap-[16px] items-start w-full">
          <a v-for="(card, idx) in mediaCards" :key="idx" :href="card.url"
             target="_blank" rel="noopener"
             :class="['rise-card', `rise-card-${idx + 1}`, 'media-card bg-white flex flex-col items-start justify-between overflow-hidden pb-[32px] px-[32px] rounded-[16px]']"
             style="width: 440px; height: 360px;">
            <div class="h-[160px] w-[440px] -mx-[32px]">
              <img :src="card.cover" alt="" class="w-full h-full object-cover" />
            </div>
            <p class="text-[22px] font-bold text-[#212121] leading-[1.35]" style="width: 376px;">{{ card.title }}</p>
            <div class="flex items-center justify-center gap-[12px]">
              <span class="media-source-badge">›</span>
              <div class="flex flex-col gap-[2px]">
                <p class="text-[14px] font-bold text-[#212121]">{{ card.author }}</p>
                <p class="text-[13px] text-[#808080]">{{ card.role }}</p>
              </div>
            </div>
          </a>
        </div>
      </section>

      <!-- divider 浮层：Compare 区域右上，压住白色圆角矩形的右边缘 -->
      <div class="relative w-full" style="height: 0;">
        <div class="absolute pointer-events-none z-10" style="top: 230px; right: 20px;">
          <PixelGrid :cols="12" :rows="12" cell-size="14px" gap="0" color="#9bff00"
                     :flash-duration="450" :idle-min="900" :idle-max="2800"
                     :start-stagger-max="2800" :jitter="0" :jitter-rotate="0" :jitter-scale="0" />
        </div>
      </div>

      <!-- ============ COMPARISON ============ -->
      <section data-reveal class="relative w-full px-[80px] py-[120px]">
        <!-- 白色圆角卡片包裹整个对比模块 -->
        <div class="w-full bg-white rounded-[28px] shadow-[0_4px_28px_rgba(0,0,0,0.04)] flex flex-col gap-[48px] px-[64px] py-[72px] items-center">
          <div class="flex items-center gap-[8px]">
            <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px]" style="transform: rotate(15deg);"></span>
            <span class="text-[20px] font-bold text-[#212121]">{{ t.compareEyebrow }}</span>
          </div>

          <p class="reveal-headline font-playfair italic text-[#212121] text-center tracking-[-1.28px]" style="font-size: 64px; line-height: 1; width: 1100px;">
            {{ t.compareHeadPart1 }}
            <span class="font-bold text-[#212121] not-italic font-['Inter']">{{ t.compareHeadVs }}</span>
            <span class="font-semibold text-[#212121] not-italic font-['Inter']">{{ t.compareHeadJoinCare }}</span>
          </p>

          <p class="text-[18px] font-medium text-[#212121] leading-[1.45] text-center" style="width: 900px;">{{ t.compareIntro }}</p>

          <!-- 表格独立 data-reveal：滚动到这里时才触发瀑布动效 -->
          <div data-reveal class="compare-table flex gap-[24px] items-start w-full">
            <!-- Traditional column -->
            <div class="flex flex-1 flex-col gap-[8px] items-start">
              <p class="h-[43px] text-[20px] font-bold text-[#212121] text-center w-full">{{ t.compareTradHeader }}</p>
              <div
                v-for="(row, i) in traditionalList" :key="i"
                :style="{ transitionDelay: (0.08 + i * 0.09) + 's' }"
                class="compare-row bg-[#f5f5f3] flex gap-[12px] items-center px-[18px] py-[16px] rounded-[12px] w-full"
              >
                <span class="bg-[#212121] w-[28px] h-[28px] flex items-center justify-center rounded-[4px] text-[18px] font-bold text-white">−</span>
                <p class="flex-1 text-[16px] font-medium text-[#212121] leading-[1.4]">{{ row }}</p>
              </div>
            </div>

            <!-- JoinCare column -->
            <div class="flex flex-1 flex-col gap-[8px] items-start">
              <p class="h-[40px] text-[20px] font-bold text-[#212121] text-center w-full">{{ t.compareJoinHeader }}</p>
              <div
                v-for="(row, i) in joinCareList" :key="i"
                :style="{ transitionDelay: (0.08 + i * 0.09) + 's' }"
                class="compare-row bg-[#f5f5f3] flex gap-[12px] items-center px-[18px] py-[16px] rounded-[12px] w-full"
              >
                <span class="bg-[#9bff00] w-[28px] h-[28px] flex items-center justify-center rounded-[4px] text-[18px] font-bold text-[#212121]">+</span>
                <p class="flex-1 text-[16px] font-medium text-[#212121] leading-[1.4]">{{ row }}</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- divider 浮层：Compare → Partners（偏右） -->
      <div v-if="SHOW_PARTNERS" class="relative w-full" style="height: 0;">
        <div class="absolute pointer-events-none z-10" style="top: -40px; right: 180px;">
          <PixelGrid :cols="10" :rows="10" cell-size="14px" gap="0" color="#9bff00"
                     :flash-duration="450" :idle-min="900" :idle-max="2800"
                     :start-stagger-max="2800" :jitter="0" :jitter-rotate="0" :jitter-scale="0" />
        </div>
      </div>

      <!-- ============ PARTNERS (dark) ============ -->
      <section v-if="SHOW_PARTNERS" ref="partnersRef" data-reveal class="partners-block relative w-full flex flex-col gap-[56px] px-[80px] pt-[140px] pb-[140px]">
        <!-- 深色背景由 canvas-host 层的 .dark-strip 全视口宽渲染 -->

        <div class="flex items-center gap-[8px] relative z-10">
          <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px]" style="transform: rotate(15deg);"></span>
          <span class="text-[20px] font-bold text-white">{{ t.partnersEyebrow }}</span>
        </div>

        <p class="reveal-headline font-bold text-white tracking-[-1.28px] relative z-10" style="font-size: 64px; line-height: 1.05; width: 1100px;">
          {{ t.partnersHeadPart1 }}
          <span class="font-playfair italic font-normal">{{ t.partnersHeadPart2 }}</span>.
        </p>

        <p class="text-[18px] font-medium text-[#c7c7c7] leading-[1.5] relative z-10" style="width: 760px;">{{ t.partnersCopy }}</p>

        <div class="partner-logo-marquee relative z-10" aria-label="JoinCare partners">
          <div class="partner-logo-track">
            <div class="partner-logo-group">
              <a
                v-for="partner in partners"
                :key="partner.name"
                :href="partner.href"
                target="_blank"
                rel="noopener"
                class="partner-logo-cell"
              >
                <img :src="partner.logo" :alt="partner.name" />
              </a>
            </div>
            <div class="partner-logo-group" aria-hidden="true">
              <a
                v-for="partner in partners"
                :key="`duplicate-${partner.name}`"
                :href="partner.href"
                target="_blank"
                rel="noopener"
                class="partner-logo-cell"
                tabindex="-1"
              >
                <img :src="partner.logo" alt="" />
              </a>
            </div>
          </div>
        </div>
      </section>

      <!-- ============ FOOTER (dark) ============ -->
      <footer ref="footerRef" class="footer-block relative w-full flex flex-col gap-[80px] pt-[80px] pb-[64px] px-[80px]">
        <!-- 深色背景由 canvas-host 层的 .dark-strip 全视口宽渲染 -->

        <div class="flex items-center justify-start w-full relative z-10">
          <img :src="img('footer-logo.svg')" alt="JoinCare" class="h-[64px]" style="width: 230px;" />
        </div>

        <div class="flex gap-[48px] items-start text-[14px] w-full relative z-10">
          <!-- Addresses —— label 为空时整块不渲染（后台清空即消失） -->
          <div class="flex flex-col gap-[24px]">
            <div v-if="t.footerLabSG && t.footerLabSG.trim()" class="flex flex-col gap-[6px]">
              <p class="font-bold text-white">{{ t.footerLabSG }}</p>
              <div class="text-[#999]">
                <p v-if="t.footerLabSGLine1" class="leading-[1.5] mb-0">{{ t.footerLabSGLine1 }}</p>
                <p v-if="t.footerLabSGLine2" class="leading-[1.5]">{{ t.footerLabSGLine2 }}</p>
              </div>
            </div>
            <div v-if="t.footerResBos && t.footerResBos.trim()" class="flex flex-col gap-[6px]">
              <p class="font-bold text-white">{{ t.footerResBos }}</p>
              <div class="text-[#999]">
                <p v-if="t.footerResBosLine1" class="leading-[1.5] mb-0">{{ t.footerResBosLine1 }}</p>
                <p v-if="t.footerResBosLine2" class="leading-[1.5]">{{ t.footerResBosLine2 }}</p>
              </div>
            </div>
            <div v-if="t.footerContact && t.footerContact.trim()" class="flex flex-col gap-[6px]">
              <p class="font-bold text-white">{{ t.footerContact }}</p>
              <div class="text-[#999]">
                <p v-if="t.footerContactLine1" class="leading-[1.5] mb-0">{{ t.footerContactLine1 }}</p>
                <p v-if="t.footerContactLine2" class="leading-[1.5]">{{ t.footerContactLine2 }}</p>
              </div>
            </div>
          </div>

          <!-- Explore：核心站内路由 -->
          <div class="flex flex-1 flex-col gap-[16px] items-start">
            <p class="font-bold text-white">{{ t.footerColExplore }}</p>
            <router-link to="/" class="footer-link">{{ t.footerLinkHome }}</router-link>
            <router-link to="/ecosystem" class="footer-link">{{ t.footerLinkEcosystem }}</router-link>
            <router-link to="/launchpad" class="footer-link">{{ t.footerLinkLaunchpad }}</router-link>
            <router-link v-if="SHOW_TASKS_ENTRY" to="/tasks" class="footer-link">{{ t.footerLinkQuest }}</router-link>
          </div>
          <!-- For Builders：项目方入口 -->
          <div class="flex flex-1 flex-col gap-[16px] items-start">
            <p class="font-bold text-white">{{ t.footerColBuilders }}</p>
            <router-link to="/launchpad/apply" class="footer-link">{{ t.footerLinkSubmit }}</router-link>
          </div>
          <!-- About：关于 & 文档 -->
          <div class="flex flex-1 flex-col gap-[16px] items-start">
            <p class="font-bold text-white">{{ t.footerColAbout }}</p>
            <router-link to="/team" class="footer-link">{{ t.footerLinkTeam }}</router-link>
            <a href="https://joincare.gitbook.io/joincare-docs" target="_blank" rel="noopener" class="footer-link">{{ t.footerLinkDocs }}</a>
          </div>
          <!-- Community：外链社群 -->
          <div class="flex flex-1 flex-col gap-[16px] items-start">
            <p class="font-bold text-white">{{ t.footerColCommunity }}</p>
            <a href="https://x.com/TheJoinCare" target="_blank" rel="noopener" class="footer-link">X (Twitter)</a>
            <a :href="telegramUrl" target="_blank" rel="noopener" class="footer-link">Telegram</a>
          </div>
        </div>

        <div class="flex items-center justify-between pt-[40px] text-[13px] text-[#999] w-full relative z-10">
          <p>{{ t.footerCopyright }}</p>
          <p>{{ t.footerLegal }}</p>
        </div>
      </footer>
    </main>
    </div>
  </div>
</template>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Playfair+Display:ital,wght@1,400;1,700&family=Inter:wght@300;400;500;600;700&display=swap');

.font-playfair { font-family: 'Playfair Display', Georgia, serif; }

.canvas-host {
  width: 100%;
  overflow-x: clip;     /* clip 不创建滚动容器，sticky 才能贴到视口顶部 */
  position: relative;
  display: flex;
  justify-content: center;
  background: #ffffff;  /* 大屏留白区也是白底 */
}
/* 噪点底纹覆盖整个 canvas-host，确保大屏留白区也带纹理 */
.canvas-host::before {
  content: '';
  position: absolute;
  inset: 0;
  background-image: url('/figma/v2/noise-bg.png');
  background-size: 500px 500px;
  background-repeat: repeat;
  background-position: top left;
  opacity: 0.05;        /* 浅色下噪点压低，让白底更亮 */
  pointer-events: none;
  z-index: 0;
}
.canvas-host > * {
  position: relative;
  z-index: 1;           /* 内容在噪点之上 */
}

/* === 深色条带：渲染在 canvas-host 层级，全视口宽，JS 测量 top/height ===
   避开 canvas-1440 的 transform 干扰（之前 bleed-bg 在 scaled 上下文里
   宽度受 transform 影响导致计算不一致） */
.dark-strip {
  position: absolute;
  left: 0;
  right: 0;            /* canvas-host 是 100vw 宽，自动撑满视口 */
  z-index: 0;          /* 在 .canvas-host > * (z:1) 之下，noise (z:0) 之上 */
  pointer-events: none;
  overflow: hidden;
}
.dark-strip-noise {
  position: absolute;
  inset: 0;
  background-size: 500px 500px;
  background-repeat: repeat;
  background-position: top left;
  opacity: 0.10;
}

.scaled-wrapper {
  position: relative;
  overflow: clip;   /* 用 clip 而不是 hidden，允许内部 position: sticky 生效 */
}

.canvas-1440 {
  width: 1440px;
  position: relative;
  transform-origin: top left;
  display: flex;
  flex-direction: column;
  align-items: center;
  font-family: 'Inter', sans-serif;
  color: #212121;
}

/* === Featured carousel：在 canvas-host 层级渲染，全视口宽 === */
.featured-carousel-host {
  position: absolute;
  left: 0;
  width: 100vw;
  z-index: 50;       /* 提高 z-index，确保盖在 canvas-1440 内的 carouselSlot 占位之上 */
  pointer-events: auto;
}
/* 蒲公英 3D 模型层：全 viewport 宽，覆盖 hero 范围；不受 1440 限制 */
.dandelion-bleed {
  position: absolute;
  top: 0;
  left: 0;
  width: 100vw;
  z-index: 0;              /* 在 hero 文字下方 */
  pointer-events: none;
  overflow: hidden;
}

.featured-carousel-scroll {
  width: 100%;
  overflow-x: auto;
  overflow-y: hidden;
  scroll-behavior: smooth;
  scrollbar-width: none;
  -ms-overflow-style: none;
}
.featured-carousel-scroll::-webkit-scrollbar { display: none; }

/* wrap：layout 持有"视觉尺寸"，保证 overflow 计算和实际视觉一致 */
.cards-row-wrap {
  display: flex;
  align-items: flex-start;
  flex-shrink: 0;
}

/* 内层：用 transform 把卡片视觉缩放到当前 scale，与画布一致 */
.cards-row-scaled {
  display: flex;
  gap: 16px;
  height: 424px;
  flex-shrink: 0;
}

/* 轮播按钮 disabled 状态 */
.carousel-btn:disabled { pointer-events: none; }

/* 轮播按钮 disabled 状态 */
.carousel-btn:disabled {
  pointer-events: none;
}

/* === Floating navbar: fixed 定位独立缩放，永远贴视口顶部 === */
.floating-navbar {
  position: fixed;
  top: 24px;                 /* 与设计稿一致的距顶 24px */
  left: 50%;
  z-index: 100;
  transform-origin: top center;
  transform: translateX(-50%) scale(1);
  /* 无背景、无磨砂——只是个浮起的白胶囊 */
}

/* === 安置动画 transition：仅在初次进入时开启，结束后由 JS 关掉避免影响 resize === */
.with-entrance-transition {
  transition: transform 0.85s cubic-bezier(0.76, 0, 0.24, 1);
}

/* === Hero 文字逐项快速出现（与 splash 上拉同步）=== */
@keyframes hero-rise {
  from { opacity: 0; transform: translateY(18px); }
  to   { opacity: 1; transform: translateY(0); }
}
.hero-anim {
  opacity: 0;
  animation: hero-rise 0.42s cubic-bezier(0.22, 1, 0.36, 1) both;
}

/* DNA 3D：splash + hero 文字落定（~3s）后从左下方斜向插入到现位置；
   外层 overflow:hidden 把地平线以下的部分裁掉，所以 DNA 看起来从 1px 线"钻出来" */
.dna-late-reveal {
  opacity: 0;
  transform: translate(-260px, 900px);
  animation: dna-shoot-up 1.15s cubic-bezier(0.16, 1, 0.3, 1) 2.55s forwards;
  /* easing：expo-out — 起步极快、临近终点强力减速 */
  will-change: transform, opacity;
}
@keyframes dna-shoot-up {
  0%   { opacity: 1; transform: translate(-260px, 900px); }
  100% { opacity: 1; transform: translate(0, 0); }
}
.hero-anim-1 { animation-delay: 1.95s; }
.hero-anim-2 { animation-delay: 2.05s; }
.hero-anim-3 { animation-delay: 2.55s; }
/* 路由切换进入：压短 hero 入场延迟，让 hero 跟上幕布退出，不再卡 1+ 秒 */
.fast-hero-enter .hero-anim-1 { animation-delay: 0.05s !important; }
.fast-hero-enter .hero-anim-2 { animation-delay: 0.15s !important; }
.fast-hero-enter .hero-anim-3 { animation-delay: 0.30s !important; }
.fast-hero-enter .dna-late-reveal { animation-delay: 0.05s !important; }

/* 大标题：
   ① 每一行从下方 60px 整体上浮起（translateY）
   ② 行内字"瀑布"自上而下显现（clip-path 从底 100% 收到 0）
   ③ 行与行之间错峰 280ms
   ④ 缓动用陡 S-curve，节奏硬一点 */
@keyframes headline-waterfall {
  from {
    opacity: 0;
    transform: translateY(60px);
    clip-path: inset(0 0 100% 0);
    -webkit-clip-path: inset(0 0 100% 0);
  }
  to {
    opacity: 1;
    transform: translateY(0);
    clip-path: inset(0 0 0 0);
    -webkit-clip-path: inset(0 0 0 0);
  }
}
.headline-line {
  display: block;
  opacity: 0;
  line-height: 1.03 !important;
  padding-bottom: 0.035em;
  will-change: transform, clip-path, opacity;
  animation: headline-waterfall 0.72s cubic-bezier(0.83, 0, 0.17, 1) both;
}
.headline-line-1 { animation-delay: 2.10s; }
.headline-line-2 { animation-delay: 2.38s; }
.fast-hero-enter .headline-line-1 { animation-delay: 0.20s !important; }
.fast-hero-enter .headline-line-2 { animation-delay: 0.46s !important; }

/* === Featured Projects 滚动初现（与 hero 同款节奏，gating 在 .is-revealed） === */
.fp-anim {
  opacity: 0;
  transform: translateY(28px);
  will-change: opacity, transform;
}
.fp-headline {
  opacity: 0;
  transform: translateY(60px);
  clip-path: inset(0 0 100% 0);
  -webkit-clip-path: inset(0 0 100% 0);
  will-change: opacity, transform, clip-path;
}
.is-revealed .fp-anim {
  animation: hero-rise 0.5s cubic-bezier(0.22, 1, 0.36, 1) both;
}
.is-revealed .fp-headline {
  animation: headline-waterfall 0.78s cubic-bezier(0.83, 0, 0.17, 1) both;
  animation-delay: 0.18s;
}
.is-revealed .fp-anim-1 { animation-delay: 0s; }
.is-revealed .fp-anim-3 { animation-delay: 0.45s; }
.is-revealed .fp-anim-4 { animation-delay: 0.62s; }

/* Case study 卡片单独处理：用 opacity 渐入，避免覆盖原有 float-y 动画 */
.case-study-card {
  opacity: 0;
  animation: case-fade-in 0.5s cubic-bezier(0.22, 1, 0.36, 1) 2.75s forwards,
             float-y 6s ease-in-out infinite;
}
@keyframes case-fade-in {
  to { opacity: 1; }
}

/* Hero CTA 胶囊：Connect Wallet + Join Airdrop */
.hero-cta-pill {
  display: inline-flex;
  align-items: center;
  gap: 14px;
  height: 56px;
  padding: 7px 22px 7px 28px;      /* arrow-icon 右侧 22px 呼吸 */
  border-radius: 999px;
  font-size: 16px;
  font-weight: 700;
  letter-spacing: -0.2px;
  cursor: pointer;
  border: 1px solid transparent;
  line-height: 1;
  transition: transform 0.25s cubic-bezier(0.22, 1, 0.36, 1),
              box-shadow 0.25s cubic-bezier(0.22, 1, 0.36, 1),
              background 0.45s cubic-bezier(0.65, 0, 0.35, 1),
              color    0.45s cubic-bezier(0.65, 0, 0.35, 1);
}
.hero-cta-pill:hover {
  transform: translateY(-2px);
  box-shadow: 0 10px 24px rgba(0, 0, 0, 0.12);
}
.hero-cta-primary {
  background: #9bff00;
  color: #212121;
  padding: 0 26px;                 /* 无 arrow-icon，左右等距 */
}
.hero-cta-primary:hover {
  background: #212121;
  color: #fff;
}

/* Join Airdrop 胶囊（浅色背景 hero 区域用）：
   默认浅绿色，hover → 黑色；arrow-icon 颜色反转 + diag 飞出 */
.hero-cta-dark {
  background: #9bff00;
  color: #212121;
}
.hero-cta-dark .arrow-icon {
  background: #212121;             /* 默认黑色方块 */
  transition: background 0.45s cubic-bezier(0.65, 0, 0.35, 1);
}
.hero-cta-dark .arrow-img {
  background-color: #9bff00;       /* 默认绿色箭头 */
}
.hero-cta-dark:hover {
  background: #212121;
  color: #fff;
}
.hero-cta-dark:hover .arrow-icon { background: #9bff00; }
.hero-cta-dark:hover .arrow-img  { background-color: #212121; }
/* 触发 .arrow-diag 滑出 / 第二枚箭头从左下滑入 */
.hero-cta-dark:hover .arrow-diag .arrow-img { transform: translate(30px, -30px); }

/* Discover button (hero) */
.discover-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 7px 7px 7px 0;
  border-radius: 7px;
  filter: drop-shadow(0px 4px 8px rgba(0,0,0,0.06));
}

/* White pill button (View Full Methodology / Meet team) */
.white-pill-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 48px;
  background: #fff;
  border-radius: 7px;
  padding: 7px 7px 7px 14px;
  gap: 8px;
}

.arrow-icon {
  display: inline-block;
  position: relative;
  width: 36px;
  height: 36px;
  background: #9bff00;
  border-radius: 4px;
  flex-shrink: 0;
  overflow: hidden;
  transition: background 0.45s cubic-bezier(0.65, 0, 0.35, 1);
}
.arrow-img {
  position: absolute;
  width: 16px;
  height: 16px;
  background-color: #212121;           /* 默认箭头颜色（黑） */
  -webkit-mask-image: var(--mask);
  mask-image: var(--mask);
  -webkit-mask-size: contain;
  mask-size: contain;
  -webkit-mask-repeat: no-repeat;
  mask-repeat: no-repeat;
  -webkit-mask-position: center;
  mask-position: center;
  transition: transform 0.45s cubic-bezier(0.65, 0, 0.35, 1),
              background-color 0.45s cubic-bezier(0.65, 0, 0.35, 1);
}

/* === Variant: vertical flip (Discover button) === */
.arrow-vert .arrow-a { left: 10px; top: 10px; }
.arrow-vert .arrow-b { left: 10px; top: -20px; }
.discover-btn:hover  .arrow-vert .arrow-img { transform: translateY(30px); }

/* === Variant: diagonal flip (up-right pill buttons + case study) === */
.arrow-diag .arrow-a { left: 10px; top: 10px; }
.arrow-diag .arrow-b { left: -20px; top: 40px; }
.white-pill-btn:hover   .arrow-diag .arrow-img { transform: translate(30px, -30px); }
.discover-btn:hover     .arrow-diag .arrow-img { transform: translate(30px, -30px); }
.case-study-card:hover  .arrow-diag .arrow-img { transform: translate(30px, -30px); }

/* === Color flip on hover: green bg → dark bg, dark arrow → GREEN arrow === */
.discover-btn:hover    .arrow-icon,
.white-pill-btn:hover  .arrow-icon,
.case-study-card:hover .arrow-icon {
  background: #212121;
}
.discover-btn:hover    .arrow-img,
.white-pill-btn:hover  .arrow-img,
.case-study-card:hover .arrow-img {
  background-color: #9bff00;          /* 黑底+绿色箭头 */
}

/* === Subtle elevation on pill button hover === */
.white-pill-btn { transition: box-shadow .3s ease, transform .3s ease; }
.white-pill-btn:hover {
  box-shadow: 0 8px 20px rgba(0,0,0,0.08);
  transform: translateY(-1px);
}
.white-pill-btn:disabled,
.white-pill-btn.is-disabled {
  cursor: not-allowed;
  opacity: 0.52;
  filter: grayscale(1);
}
.white-pill-btn:disabled:hover,
.white-pill-btn.is-disabled:hover {
  box-shadow: none;
  transform: none;
}
.white-pill-btn:disabled .arrow-icon,
.white-pill-btn.is-disabled .arrow-icon {
  background: #8a8a8a;
}
.white-pill-btn:disabled .arrow-img,
.white-pill-btn.is-disabled .arrow-img {
  background-color: #2d2d2d;
  transform: none !important;
}
.discover-btn { cursor: pointer; transition: filter .3s ease; }
.discover-btn:hover { filter: drop-shadow(0px 8px 16px rgba(0,0,0,0.10)); }

/* === Connect Wallet button: 装饰图层 + hover 下滑 swoosh === */
.connect-wallet-btn {
  position: relative;
  width: 130px;
  height: 32px;
  background: #9bff00;
  border: none;
  border-radius: 18px;
  box-shadow: 0px 2px 8px 0px rgba(0,0,0,0.1);
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: box-shadow 0.4s ease, transform 0.4s ease;
}
.connect-wallet-btn:hover {
  box-shadow: 0 8px 24px rgba(155, 255, 0, 0.55);
  transform: translateY(-1px);
}
.cw-deco {
  position: absolute;
  display: block;
  pointer-events: none;
  transition: transform 0.6s cubic-bezier(0.65, 0, 0.35, 1);
}
.cw-deco img { width: 100%; height: 100%; display: block; }
.cw-wave-wrap {
  width: 225.388px;
  height: 118.576px;
  left: -45.97px;
  top: -111px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.cw-wave {
  width: 217.5px;
  height: 100px;
  transform: rotate(5deg);
}
.cw-crest {
  width: 101.5px;
  height: 56px;
}
.cw-crest-r { left: 64.25px;  top: -97px; }
.cw-crest-l { left: -22.75px; top: -63px; }
.connect-wallet-btn:hover .cw-wave-wrap { transform: translateY(72px); }
.connect-wallet-btn:hover .cw-crest-r  { transform: translateY(58px); }
.connect-wallet-btn:hover .cw-crest-l  { transform: translateY(42px); }
.cw-label {
  position: relative;
  z-index: 2;
  font-size: 14px;
  color: #0b1020;
  font-family: 'Inter', sans-serif;
  font-weight: 400;
  transition: color 0.45s ease;
}
.connect-wallet-btn:hover .cw-label { color: #9bff00; }

/* Carousel buttons */
.carousel-btn {
  width: 48px;
  height: 48px;
  border-radius: 5px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.carousel-btn img {
  width: 14.5px;
  height: 12.7px;
  filter: brightness(0) invert(1);
}

/* Project chip */
.project-chip {
  display: inline-block;
  width: fit-content;
  background: #9bff00;
  border-radius: 16px;
  padding: 4px 14px;
  font-family: 'Playfair Display', serif;
  font-weight: 700;
  font-size: 14px;
  color: #212121;
  line-height: 1;
}

/* float-y keyframes (consumed by .case-study-card animation chain above) */
@keyframes float-y {
  0%, 100% { transform: translateY(0); }
  50%      { transform: translateY(-8px); }
}

/* fp-card hover 块在下方（必须放到 train-item 规则之后，否则 transition 被盖） */

/* === Incubation Model 3 frame hover ===
   用 CSS Grid + grid-template-columns transition，
   动画只动列宽（一次布局重计算后由浏览器插值），不像 flex-grow 那样每帧重排，
   配合 :has() 精确选中被 hover 的列，丝滑无卡顿。 */
.incub-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 5px;
  align-items: start;
  transition: grid-template-columns 0.32s cubic-bezier(0, 0, 0.2, 1);
}
.incub-grid:has(.incub-card:nth-child(1):hover) { grid-template-columns: 1.4fr 0.8fr 0.8fr; }
.incub-grid:has(.incub-card:nth-child(2):hover) { grid-template-columns: 0.8fr 1.4fr 0.8fr; }
.incub-grid:has(.incub-card:nth-child(3):hover) { grid-template-columns: 0.8fr 0.8fr 1.4fr; }
.incub-card { min-width: 0; }

/* 步骤详情：默认可见（不再依赖 hover 才显示） */
.incub-detail {
  opacity: 1;
  transform: translateY(0);
  transition: opacity 0.28s cubic-bezier(0.22, 1, 0.36, 1) 0.05s,
              transform 0.28s cubic-bezier(0.22, 1, 0.36, 1) 0.05s;
  pointer-events: auto;
}
/* 标题色：hover 时变成品牌绿 */
.incub-card > p:first-child {
  transition: color 0.22s ease;
}
.incub-card:hover > p:first-child {
  color: #4a8c00 !important;
}

/* === Research collective 三张照片 hover：扇形展开
   - 缓动加入轻微回弹（cubic-bezier 末段 1.18），给出阻尼感
   - 触发区域扩展到 Meet team 按钮：用 :has() 让 button hover 也保持扇形展开 */
.fan-left,
.fan-right,
.fan-center {
  transition: translate 1.1s cubic-bezier(0.25, 1.18, 0.45, 1),
              rotate 1.1s cubic-bezier(0.25, 1.18, 0.45, 1);
  will-change: translate, rotate;
}
.fan-stack:hover .fan-left,
[data-reveal]:has(.meet-team-btn:hover) .fan-left   { translate: -32px 4px;  rotate: -7deg; }
.fan-stack:hover .fan-right,
[data-reveal]:has(.meet-team-btn:hover) .fan-right  { translate:  32px 4px;  rotate:  7deg; }
.fan-stack:hover .fan-center,
[data-reveal]:has(.meet-team-btn:hover) .fan-center { translate:   0   -10px; }

/* Footer 站内 / 外链链接 —— 沿用之前 <p> 的灰色 + hover 品牌绿高亮 */
.footer-link {
  font-size: 14px;
  font-weight: 500;
  color: #999;
  text-decoration: none;
  transition: color 0.18s ease;
  cursor: pointer;
}
.footer-link:hover { color: #9bff00; }

/* Latest Media 卡片 —— 新闻来源 badge：圆形黑底白 › */
.media-source-badge {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  background: #212121;
  color: #fff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
  font-weight: 700;
  line-height: 1;
  flex-shrink: 0;
}
</style>

<!-- 通用滚动初现：放在非 scoped 块，[data-reveal] 选择器避免被 Vue 加 hash -->
<style>
/* === Section wrapper：透明度淡入（避免 transform 与子动画冲突） === */
[data-reveal] {
  opacity: 0;
  transition: opacity 0.4s ease-out;
}
[data-reveal].is-revealed {
  opacity: 1;
}

/* === Eyebrow（小标签 + 绿点）：上浮淡入 === */
[data-reveal] .reveal-eyebrow {
  opacity: 0;
  transform: translateY(18px);
  transition: opacity 0.55s cubic-bezier(0.22, 1, 0.36, 1),
              transform 0.55s cubic-bezier(0.22, 1, 0.36, 1);
}
[data-reveal].is-revealed .reveal-eyebrow {
  opacity: 1;
  transform: translateY(0);
}

/* === 大标题：复用 hero 的瀑布揭开 === */
@keyframes reveal-headline-waterfall {
  from {
    opacity: 0;
    transform: translateY(60px);
    clip-path: inset(0 0 100% 0);
    -webkit-clip-path: inset(0 0 100% 0);
  }
  to {
    opacity: 1;
    transform: translateY(0);
    clip-path: inset(0 0 0 0);
    -webkit-clip-path: inset(0 0 0 0);
  }
}
[data-reveal] .reveal-headline {
  opacity: 0;
  line-height: 1.03 !important;
  padding-bottom: 0.035em;
  will-change: opacity, transform, clip-path;
}
[data-reveal].is-revealed .reveal-headline {
  animation: reveal-headline-waterfall 0.85s cubic-bezier(0.83, 0, 0.17, 1) both;
  animation-delay: 0.10s;
}
/* 多行标题（每个 p 都是一行）：错峰 */
[data-reveal] .reveal-headline-line {
  display: block;
  opacity: 0;
  line-height: 1.03 !important;
  padding-bottom: 0.035em;
  will-change: opacity, transform, clip-path;
}
[data-reveal].is-revealed .reveal-headline-line {
  animation: reveal-headline-waterfall 0.78s cubic-bezier(0.83, 0, 0.17, 1) both;
}
[data-reveal].is-revealed .reveal-headline-line-1 { animation-delay: 0.10s; }
[data-reveal].is-revealed .reveal-headline-line-2 { animation-delay: 0.30s; }

/* === Featured Projects 列表：从右侧火车式滑入 === */
[data-reveal] .train-item {
  opacity: 0;
  transform: translateX(180px);
  transition: opacity 0.7s cubic-bezier(0.22, 1, 0.36, 1),
              transform 0.7s cubic-bezier(0.22, 1, 0.36, 1);
}
[data-reveal].is-revealed .train-item {
  opacity: 1;
  transform: translateX(0);
}
[data-reveal].is-revealed .train-item:nth-child(1) { transition-delay: 0.12s; }
[data-reveal].is-revealed .train-item:nth-child(2) { transition-delay: 0.24s; }
[data-reveal].is-revealed .train-item:nth-child(3) { transition-delay: 0.36s; }
[data-reveal].is-revealed .train-item:nth-child(4) { transition-delay: 0.48s; }

/* === Featured Projects 卡片 hover：放大 + 抬起 + 阴影，跳转胶囊滑入 ===
   注意：必须排在 [data-reveal] .train-item 规则之后，否则 transition 被覆盖。
   不要改 .featured-carousel-scroll 的 overflow-y！当 overflow-x:auto + overflow-y:visible
   时浏览器会把 visible 升级为 auto，把单轴滚动容器变成双轴，导致命中区与可视区错位 */
[data-reveal] .train-item.fp-card,
.fp-card {
  transition:
    opacity 0.7s cubic-bezier(0.22, 1, 0.36, 1),
    transform 0.7s cubic-bezier(0.22, 1, 0.36, 1),
    scale 0.28s cubic-bezier(0.22, 1, 0.36, 1),
    translate 0.28s cubic-bezier(0.22, 1, 0.36, 1),
    box-shadow 0.28s cubic-bezier(0.22, 1, 0.36, 1) !important;
  will-change: scale, translate;
}
[data-reveal] .train-item.fp-card:hover,
.fp-card:hover,
.fp-card.is-hovered {
  scale: 1.04 !important;
  translate: 0 -3px !important;
  box-shadow: 0 18px 40px -6px rgba(0, 0, 0, 0.26) !important;
  z-index: 10 !important;
}
/* 第三张占位骨架：仅一个虚线方块，无内容；不参与 hover 缩放 */
.fp-skeleton {
  background: transparent !important;
  border: 1.5px dashed #d8d6cf;
  pointer-events: none;
  cursor: default;
}
.fp-skeleton:hover {
  scale: 1 !important;
  translate: 0 0 !important;
  box-shadow: none !important;
}
/* 暗色主题：虚线压暗、变细，不再抢眼 */
.canvas-host.home-dark .fp-skeleton {
  border-color: rgba(255, 255, 255, 0.10) !important;
  border-width: 1px !important;
}

/* 跳转胶囊：默认隐藏，hover 卡片时滑入；按钮自身 hover 再小幅 lift */
.fp-detail-btn {
  cursor: pointer;
  opacity: 0;
  transform: translateY(10px) scale(0.92);
  pointer-events: none;
  box-shadow: 0 4px 10px rgba(0, 0, 0, 0.08);
  transition: opacity 0.24s cubic-bezier(0.22, 1, 0.36, 1),
              transform 0.28s cubic-bezier(0.22, 1, 0.36, 1),
              box-shadow 0.2s ease;
}
.fp-card:hover .fp-detail-btn,
.fp-card.is-hovered .fp-detail-btn {
  opacity: 1;
  transform: translateY(0) scale(1);
  pointer-events: auto;
}
.fp-detail-btn:hover {
  transform: translateY(-2px) scale(1.03);
  box-shadow: 0 10px 22px rgba(0, 0, 0, 0.18);
}

/* === Incubation Model 3 方块：scale up（带轻微回弹） === */
[data-reveal] .scale-pop {
  opacity: 0;
  transform: scale(0.55);
  transform-origin: center;
  transition: opacity 0.6s cubic-bezier(0.22, 1.6, 0.4, 1),
              transform 0.6s cubic-bezier(0.22, 1.6, 0.4, 1);
}
[data-reveal].is-revealed .scale-pop {
  opacity: 1;
  transform: scale(1);
}
[data-reveal].is-revealed .scale-pop-1 { transition-delay: 0.28s; }
[data-reveal].is-revealed .scale-pop-2 { transition-delay: 0.42s; }
[data-reveal].is-revealed .scale-pop-3 { transition-delay: 0.56s; }

/* === Research collective: 中心爆破（Dream 放大 + 头像 fan-out） === */
[data-reveal] .burst-main {
  opacity: 0;
  line-height: 1.02 !important;
  padding-bottom: 0.03em;
  transform: scale(0.35);
  transform-origin: center;
  transition: opacity 0.85s cubic-bezier(0.22, 1.3, 0.4, 1),
              transform 0.85s cubic-bezier(0.22, 1.3, 0.4, 1);
}
[data-reveal].is-revealed .burst-main {
  opacity: 1;
  transform: scale(1);
  transition-delay: 0.85s;
}
[data-reveal] .burst-card {
  opacity: 0;
  transform: scale(0.5) translateY(40px);
  transition: opacity 0.65s cubic-bezier(0.22, 1.2, 0.5, 1),
              transform 0.65s cubic-bezier(0.22, 1.2, 0.5, 1);
}
[data-reveal].is-revealed .burst-card {
  opacity: 1;
  transform: scale(1) translateY(0);
}
[data-reveal].is-revealed .burst-card-1 { transition-delay: 1.05s; }
[data-reveal].is-revealed .burst-card-2 { transition-delay: 1.20s; }
[data-reveal].is-revealed .burst-card-3 { transition-delay: 1.35s; }
/* 浮动标签从中心向外散开 */
[data-reveal] .burst-tag {
  opacity: 0;
  transform: scale(0.3);
  transform-origin: center;
  transition: opacity 0.55s cubic-bezier(0.22, 1.4, 0.4, 1),
              transform 0.55s cubic-bezier(0.22, 1.4, 0.4, 1);
}
[data-reveal].is-revealed .burst-tag { opacity: 1; transform: scale(1); }
[data-reveal].is-revealed .burst-tag-1 { transition-delay: 1.50s; }
[data-reveal].is-revealed .burst-tag-2 { transition-delay: 1.62s; }
[data-reveal].is-revealed .burst-tag-3 { transition-delay: 1.74s; }
[data-reveal].is-revealed .burst-tag-4 { transition-delay: 1.62s; }
[data-reveal].is-revealed .burst-tag-5 { transition-delay: 1.50s; }
[data-reveal].is-revealed .burst-tag-6 { transition-delay: 1.74s; }

/* === Latest Media 3 卡：从下方升起 stagger === */
[data-reveal] .rise-card {
  opacity: 0;
  transform: translateY(80px);
  transition: opacity 0.75s cubic-bezier(0.22, 1, 0.36, 1),
              transform 0.75s cubic-bezier(0.22, 1, 0.36, 1);
}
[data-reveal].is-revealed .rise-card {
  opacity: 1;
  transform: translateY(0);
}
[data-reveal].is-revealed .rise-card-1 { transition-delay: 0.22s; }
[data-reveal].is-revealed .rise-card-2 { transition-delay: 0.36s; }
[data-reveal].is-revealed .rise-card-3 { transition-delay: 0.50s; }

/* === Latest Media 卡片：可点击，hover 抬起 + 阴影加深 === */
a.media-card {
  text-decoration: none;
  color: inherit;
  cursor: pointer;
  transition: scale 0.28s cubic-bezier(0.22, 1, 0.36, 1),
              translate 0.28s cubic-bezier(0.22, 1, 0.36, 1),
              box-shadow 0.28s cubic-bezier(0.22, 1, 0.36, 1);
}
a.media-card:hover {
  scale: 1.025;
  translate: 0 -4px;
  box-shadow: 0 16px 38px -6px rgba(0, 0, 0, 0.18);
}
/* 标题 hover 变品牌绿（与 Incubation Model 一致） */
a.media-card > p { transition: color 0.22s ease; }
a.media-card:hover > p { color: #4a8c00 !important; }

/* Featured Projects 卡片标题 hover 变品牌绿 */
.fp-card h3 { transition: color 0.22s ease; }
.fp-card:hover h3 { color: #4a8c00 !important; }

/* === Compare 表格：行自上而下瀑布出现（独立 .compare-table 触发，不被外层 section 提前触发）=== */
/* compare-table 本身不需要整段 fade（它只做 trigger），覆盖通用 [data-reveal] 规则 */
.compare-table[data-reveal] {
  opacity: 1;
  transition: none;
}
.compare-table .compare-row {
  opacity: 0;
  transform: translateY(-32px);
  transition: opacity 0.5s cubic-bezier(0.22, 1, 0.36, 1),
              transform 0.5s cubic-bezier(0.22, 1, 0.36, 1);
}
.compare-table.is-revealed .compare-row {
  opacity: 1;
  transform: translateY(0);
}

/* ============================================================
   首页专属暗色覆盖（共享暗色规则已在 jc-shell.css）
   ============================================================ */

/* 首页两条全幅 dark-strip：Partners 翻白底，Footer 翻浅色保留噪音 */
.canvas-host.home-dark .partners-strip { background: #ffffff !important; }
.canvas-host.home-dark .partners-strip .dark-strip-noise { display: none; }
.canvas-host.home-dark .footer-strip { background: #f5f5f3 !important; }

/* Partners 区段：白字 → 黑，灰描述加深 */
.canvas-host.home-dark .partners-block .text-white,
.canvas-host.home-dark .partners-block [class*="text-white"] { color: #212121 !important; }
.canvas-host.home-dark .partners-block [class*="text-[#c7c7c7]"] { color: #4a4a4a !important; }

/* 桌面版合作伙伴 logo 展示 */
.partner-logo-marquee {
  width: 100%;
  overflow: hidden;
  -webkit-mask-image: linear-gradient(to right, transparent 0%, #000 8%, #000 92%, transparent 100%);
          mask-image: linear-gradient(to right, transparent 0%, #000 8%, #000 92%, transparent 100%);
}
.partner-logo-track {
  display: flex;
  width: max-content;
  gap: 48px;
  animation: partner-marquee 60s linear infinite;
  will-change: transform;
}
.partner-logo-marquee:hover .partner-logo-track { animation-play-state: paused; }
.partner-logo-group {
  display: flex;
  align-items: center;
  gap: 48px;
  flex: 0 0 auto;
}
.partner-logo-cell {
  width: 200px;
  min-height: 64px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 8px 16px;
  flex: 0 0 auto;
}
.partner-logo-cell img {
  max-height: 56px;
  max-width: 180px;
  object-fit: contain;
  filter: brightness(0) invert(1);
  opacity: 0.85;
  transition: opacity 0.2s;
}
.partner-logo-cell:hover img { opacity: 1; }
@keyframes partner-marquee {
  from { transform: translateX(0); }
  to { transform: translateX(calc(-50% - 24px)); }
}
/* 亮色模式下反回黑色 */
.canvas-host.home-dark .partner-logo-cell img {
  filter: none;
  opacity: 0.9;
}

/* Footer 区段（Home 专用 footer-block）：白字 → 黑，灰文字加深，白 logo → 黑 */
.canvas-host.home-dark .footer-block .text-white,
.canvas-host.home-dark .footer-block [class*="text-white"] { color: #212121 !important; }
.canvas-host.home-dark .footer-block [class*="text-[#999]"] { color: #6a6a6a !important; }
.canvas-host.home-dark .footer-block img[alt="JoinCare"] { filter: brightness(0); }

/* Hero 底部 1px 地平线 → 浅色渐变 */
.canvas-host.home-dark .hero-horizon {
  background: linear-gradient(to right,
    transparent 0%,
    rgba(255,255,255,0.18) 12%,
    rgba(255,255,255,0.18) 88%,
    transparent 100%) !important;
}

/* burst-tag（Regulatory 等绿底标签）：始终保持黑字 */
.canvas-host.home-dark .burst-tag,
.canvas-host.home-dark .burst-tag * { color: #212121 !important; }

/* Hero CTA 按钮：暗色下「绿→白」hover */
.canvas-host.home-dark .hero-cta-primary {
  background: #9bff00;
  color: #212121 !important;
}
.canvas-host.home-dark .hero-cta-primary:hover {
  background: #ffffff;
  color: #212121 !important;
}
.canvas-host.home-dark .hero-cta-dark {
  background: #9bff00;
  color: #212121 !important;
}
.canvas-host.home-dark .hero-cta-dark .arrow-icon { background: #212121; }
.canvas-host.home-dark .hero-cta-dark .arrow-img  { background-color: #9bff00; }
.canvas-host.home-dark .hero-cta-dark:hover {
  background: #ffffff;
  color: #212121 !important;
}
.canvas-host.home-dark .hero-cta-dark:hover .arrow-icon { background: #9bff00; }
.canvas-host.home-dark .hero-cta-dark:hover .arrow-img  { background-color: #212121; }

/* ==============================================================
   MOBILE HOME  —— 独立布局，不复用 desktop scale
   ============================================================== */
.m-home {
  font-family: 'Inter', sans-serif;
  color: #212121;
}
.m-home.home-dark { color: #fafaf7; }

.m-hero {
  padding: 72px 20px 32px;   /* 顶部 72：让开左上角浮动 FAB(top12+44) */
  display: flex;
  flex-direction: column;
  gap: 20px;
  position: relative;
  overflow: hidden;
}
/* 3D 蒲公英：单株居中显示；容器占满 hero 全宽，蒲公英本体居中可见 */
.m-hero-dandelion {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  width: 100%;
  height: 380px;
  z-index: 0;
  opacity: 0.55;   /* 降透明度避免遮文字 */
  pointer-events: none;
}
.m-hero > :not(.m-hero-dandelion) { position: relative; z-index: 1; }

/* ============ Mobile scroll reveal 动效 ============
   给 mobile section 加 data-reveal 属性，视口内滚动时触发 fade-up */
.jc-mobile-page [data-reveal] {
  opacity: 0;
  transform: translateY(28px);
  transition: opacity 0.55s cubic-bezier(0.22, 1, 0.36, 1),
              transform 0.55s cubic-bezier(0.22, 1, 0.36, 1);
}
.jc-mobile-page [data-reveal].is-revealed {
  opacity: 1;
  transform: translateY(0);
}
/* section 内子级依次错开 —— 制造层次感 */
.jc-mobile-page [data-reveal].is-revealed > * {
  animation: mReveal 0.55s cubic-bezier(0.22, 1, 0.36, 1) both;
}
.jc-mobile-page [data-reveal].is-revealed > *:nth-child(2) { animation-delay: 0.08s; }
.jc-mobile-page [data-reveal].is-revealed > *:nth-child(3) { animation-delay: 0.16s; }
.jc-mobile-page [data-reveal].is-revealed > *:nth-child(4) { animation-delay: 0.24s; }
.jc-mobile-page [data-reveal].is-revealed > *:nth-child(5) { animation-delay: 0.32s; }
.jc-mobile-page [data-reveal].is-revealed > *:nth-child(6) { animation-delay: 0.40s; }
@keyframes mReveal {
  from { opacity: 0; transform: translateY(14px); }
  to   { opacity: 1; transform: translateY(0); }
}
.m-eyebrow {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 1.4px;
  text-transform: uppercase;
  color: #212121;
}
.m-home.home-dark .m-eyebrow { color: #fafaf7; }
.m-eyebrow-tag {
  display: inline-block;
  width: 14px;
  height: 14px;
  background: #9bff00;
  border-top-right-radius: 10px;
  border-top-left-radius: 3px;
  border-bottom-left-radius: 3px;
  border-bottom-right-radius: 3px;
  transform: rotate(15deg);
}
.m-hero-title {
  font-size: 40px;
  line-height: 1.1;
  letter-spacing: -0.8px;
  font-weight: 700;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.m-hero-title-line-2 { font-weight: 400; }
.m-hero-sub {
  font-size: 15px;
  line-height: 1.55;
  color: #5f6659;
  margin: 4px 0 0;
  max-width: 100%;
}
.m-home.home-dark .m-hero-sub { color: #b8b8b0; }
.m-hero-ctas {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 8px;
}
.m-btn-primary {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 52px;
  border: none;
  border-radius: 999px;
  background: #9bff00;
  color: #212121;
  font-size: 15px;
  font-weight: 700;
  cursor: pointer;
  box-shadow: 0 8px 24px rgba(155, 255, 0, 0.35);
  padding: 0 24px;
}
.m-btn-primary .material-symbols-outlined { font-size: 20px; }
.m-btn-secondary {
  height: 52px;
  border: 1px solid #212121;
  border-radius: 999px;
  background: transparent;
  color: #212121;
  font-size: 15px;
  font-weight: 700;
  cursor: pointer;
}
.m-home.home-dark .m-btn-secondary {
  border-color: #fafaf7;
  color: #fafaf7;
}

/* Section */
.m-section {
  padding: 32px 20px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.m-section-head { display: flex; align-items: center; gap: 8px; }
.m-section-title {
  font-size: 15px;
  font-weight: 700;
  letter-spacing: 1.4px;
  text-transform: uppercase;
  margin: 0;
}

/* Featured Projects cards */
.m-fp-list { display: flex; flex-direction: column; gap: 16px; }
.m-fp-card {
  background: white;
  border-radius: 16px;
  overflow: hidden;
  box-shadow: 0 4px 16px rgba(0,0,0,0.04);
  cursor: pointer;
  border: 1px solid #ececea;
}
.m-home.home-dark .m-fp-card {
  background: #1f1f1f;
  border-color: rgba(255,255,255,0.06);
  box-shadow: 0 4px 14px rgba(0,0,0,0.35);
}
.fp-project-img {
  display: block;
  width: 100%;
  height: auto;
  max-width: none;
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
}
.m-fp-img-frame {
  width: 100%;
  height: 180px;
  overflow: hidden;
  position: relative;
}
.m-fp-img {
  display: block;
  width: 100%;
  height: auto;
  max-width: none;
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
}
.m-fp-body { padding: 16px; display: flex; flex-direction: column; gap: 6px; }
.m-fp-cat {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 1.4px;
  text-transform: uppercase;
  color: #808080;
}
.m-fp-title {
  font-size: 20px;
  font-weight: 700;
  line-height: 1.15;
  margin: 4px 0 0;
}
.m-fp-desc {
  font-size: 14px;
  line-height: 1.5;
  color: #5f6659;
  margin: 4px 0 0;
}
.m-home.home-dark .m-fp-desc { color: #b8b8b0; }
.m-fp-cta {
  font-size: 13px;
  font-weight: 700;
  color: #1aa64a;
  margin-top: 6px;
}

/* About grid */
.m-about-grid {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.m-about-card {
  background: white;
  border: 1px solid #ececea;
  border-radius: 14px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.m-home.home-dark .m-about-card {
  background: #1f1f1f;
  border-color: rgba(255,255,255,0.06);
}
.m-about-icon {
  font-size: 24px;
  font-variation-settings: 'FILL' 1;
}
.m-about-card h4 { font-size: 16px; font-weight: 700; margin: 4px 0 0; }
.m-about-card p { font-size: 13px; color: #5f6659; line-height: 1.5; margin: 0; }
.m-home.home-dark .m-about-card p { color: #b8b8b0; }

/* Shared: 段落大标题 + 说明文 */
.m-section-headline {
  font-size: 28px;
  line-height: 1.18;
  letter-spacing: -0.5px;
  color: #212121;
  margin: 4px 0 0;
  font-weight: 600;
}
.m-home.home-dark .m-section-headline { color: #fafaf7; }
.m-section-copy {
  font-size: 14px;
  line-height: 1.55;
  color: #4c5148;
  margin: 8px 0 0;
}
.m-home.home-dark .m-section-copy { color: #b8b8b0; }

/* Incubation Model */
.m-incub-steps {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 12px;
}
.m-incub-card {
  background: white;
  border: 1px solid #ececea;
  border-radius: 14px;
  padding: 20px;
  position: relative;
}
.m-home.home-dark .m-incub-card {
  background: #1f1f1f;
  border-color: rgba(255,255,255,0.06);
}
.m-incub-index {
  display: inline-block;
  font-size: 12px;
  font-weight: 700;
  color: #1aa64a;
  letter-spacing: 1px;
}
.m-incub-title { font-size: 18px; font-weight: 700; color: #212121; margin: 6px 0 8px; }
.m-home.home-dark .m-incub-title { color: #fafaf7; }
.m-incub-desc { font-size: 13px; color: #5f6659; line-height: 1.55; margin: 0; }
.m-home.home-dark .m-incub-desc { color: #b8b8b0; }

/* Dream Team */
.m-team-photos {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  margin-top: 16px;
}
.m-team-photos img {
  width: 100%;
  aspect-ratio: 3 / 4;
  object-fit: cover;
  border-radius: 12px;
  border: 3px solid white;
  box-shadow: 0 4px 14px rgba(0,0,0,0.08);
}
.m-team-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 16px;
}
.m-team-tags span {
  background: #9bff00;
  color: #212121;
  font-size: 12px;
  font-weight: 600;
  padding: 6px 12px;
  border-radius: 20px;
}

/* Latest Media */
.m-media-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-top: 12px;
}
.m-media-card {
  background: white;
  border: 1px solid #ececea;
  border-radius: 16px;
  overflow: hidden;
  display: block;
  text-decoration: none;
  color: inherit;
}
.m-home.home-dark .m-media-card {
  background: #1f1f1f;
  border-color: rgba(255,255,255,0.06);
}
.m-media-cover {
  width: 100%;
  height: 160px;
  object-fit: cover;
  display: block;
}
.m-media-body { padding: 16px; }
.m-media-title {
  font-size: 16px;
  font-weight: 700;
  color: #212121;
  line-height: 1.35;
  margin: 0 0 12px;
}
.m-home.home-dark .m-media-title { color: #fafaf7; }
.m-media-author { display: flex; align-items: center; gap: 10px; }
.m-media-author img { width: 32px; height: 32px; border-radius: 50%; }
.m-media-author .media-source-badge { width: 32px; height: 32px; font-size: 16px; }
.m-media-name { font-size: 12px; font-weight: 700; color: #212121; margin: 0; }
.m-home.home-dark .m-media-name { color: #fafaf7; }
.m-media-role { font-size: 11px; color: #808080; margin: 0; }

/* Comparison */
.m-compare-headline .font-playfair { display: block; }
.m-compare-col { margin-top: 20px; display: flex; flex-direction: column; gap: 8px; }
.m-compare-col-title { font-size: 16px; font-weight: 700; color: #212121; margin: 0 0 4px; }
.m-home.home-dark .m-compare-col-title { color: #fafaf7; }
.m-compare-row {
  background: #f5f5f3;
  border-radius: 10px;
  padding: 12px 14px;
  display: flex;
  align-items: center;
  gap: 10px;
}
.m-home.home-dark .m-compare-row { background: #1f1f1f; }
.m-compare-row p { flex: 1; font-size: 13px; color: #212121; line-height: 1.4; margin: 0; }
.m-home.home-dark .m-compare-row p { color: #e5e5e0; }
.m-compare-icon {
  width: 22px; height: 22px;
  display: flex; align-items: center; justify-content: center;
  border-radius: 4px;
  font-weight: 700; font-size: 14px;
  flex-shrink: 0;
}
.m-compare-row-neg .m-compare-icon { background: #212121; color: white; }
.m-compare-row-pos .m-compare-icon { background: #9bff00; color: #212121; }

/* Partners */
.m-partners-section {
  background: #191919;
  margin: 32px 0 0;
  padding: 40px 24px !important;
}
.m-partners-section .m-section-title { color: #fafaf7; }
.m-partners-headline { color: white !important; }
.m-partners-copy {
  font-size: 14px;
  color: #c7c7c7;
  line-height: 1.55;
  margin: 8px 0 20px;
}
.m-partners-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 20px 12px;
  padding: 8px 0;
}
.m-partners-grid span {
  color: white;
  font-size: 18px;
  font-weight: 700;
  text-align: center;
  letter-spacing: -0.2px;
}
/* 合作伙伴 logo 在移动端使用三列；末行只有一个时居中。 */
.m-partners-logos {
  grid-template-columns: repeat(3, 1fr);
  gap: 24px 16px;
  align-items: center;
}
.m-partners-logos a {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 40px;
}
.m-partners-logos a:last-child:nth-child(3n + 1) { grid-column: 2; }
.m-partners-logos img {
  max-height: 32px;
  max-width: 100%;
  object-fit: contain;
  /* 大多数官方 logo 是深色设计，在暗底上得反色成白色以可见 */
  filter: brightness(0) invert(1);
  opacity: 0.85;
}

/* CTA section */
.m-cta-section {
  margin: 32px 20px;
  padding: 32px 24px;
  background: linear-gradient(135deg, #9bff00 0%, #7ce000 100%);
  border-radius: 20px;
  text-align: center;
}
.m-cta-title { font-size: 24px; font-weight: 700; margin: 0; color: #212121; }
.m-cta-sub { font-size: 14px; color: #1a1a1a; margin: 12px 0 20px; line-height: 1.5; }
.m-cta-section .m-btn-primary {
  background: #212121;
  color: white;
  box-shadow: 0 6px 18px rgba(0,0,0,0.25);
}

/* Footer */
.m-footer {
  background: #222;
  color: white;
  padding: 40px 24px 32px;
  display: flex;
  flex-direction: column;
  gap: 24px;
}
.m-footer-logo { height: 40px; width: auto; align-self: center; }
.m-footer-links { display: flex; flex-direction: column; gap: 20px; }
.m-footer-label { font-size: 13px; font-weight: 700; color: white; margin: 0 0 4px; }
.m-footer-value { font-size: 12px; color: #999; margin: 0; line-height: 1.5; }
.m-footer-copy { font-size: 11px; color: #666; text-align: center; margin: 0; }

/* 暗色模式：footer/partners 反色（与桌面版规范一致：浅底 + 深字） */
.m-home.home-dark .m-footer {
  background: #f5f5f3;
  color: #212121;
}
.m-home.home-dark .m-footer-logo { filter: brightness(0); }
.m-home.home-dark .m-footer-label { color: #212121; }
.m-home.home-dark .m-footer-value { color: #6a6a6a; }
.m-home.home-dark .m-footer-copy { color: #6a6a6a; }

.m-home.home-dark .m-partners-section { background: #f5f5f3; }
.m-home.home-dark .m-partners-section .m-section-title { color: #212121 !important; }
.m-home.home-dark .m-partners-headline { color: #212121 !important; }
.m-home.home-dark .m-partners-grid span { color: #212121 !important; }
.m-home.home-dark .m-partners-copy { color: #4a4a4a; }
.m-home.home-dark .m-partners-logos img {
  filter: none;
  opacity: 0.9;
}
</style>
