<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick, inject, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useWalletStore } from '../stores/wallet'
import { getProjectList } from '../services/api'
import JcFloatingNavbar from '../components/JcFloatingNavbar.vue'
import HomeFooter from '../components/HomeFooter.vue'
import { TRANSLATIONS } from '../i18n/launchpad.js'
import { useIsMobile } from '../composables/useIsMobile'
import { getFundingProgress, isLaunchpadVisibleStatus, normalizeProjectStatus, deriveProjectStage } from '../utils/projectStatus'
import { clampPercentForBar, decimalRatioPercent, formatCanonicalSaleTokenAmount, formatDecimalString, resolveProjectFundingRoute } from '../utils/fairLaunch.js'
import { localizedProjectName } from '../utils/projectLocalization.js'
import { publicFundingProgress } from '../utils/publicFundingProgress.js'
const { isMobile } = useIsMobile()

const props = defineProps({ language: { type: String, default: 'en' } })
const router = useRouter()
const wallet = useWalletStore()
const isDark = inject('isHomeDark', ref(false))

// ===== 1440 画布缩放 =====
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

// ===== 滚动初现 =====
const scrollRevealHandler = () => {
  const els = document.querySelectorAll('[data-reveal]:not(.is-revealed)')
  if (!els.length) return
  const trigger = window.innerHeight * 0.80
  els.forEach((el) => {
    if (el.getBoundingClientRect().top < trigger) el.classList.add('is-revealed')
  })
}

let projectsRequest = 0
let projectsLoading = false
let fundingRefreshTimer
let disposed = false
async function loadProjects() {
  const requestID = ++projectsRequest
  const requestLanguage = props.language
  projectsLoading = true
  // Load projects from the backend. Empty/error responses remain empty; no mock fallback is rendered.
  try {
    const isZh = requestLanguage && requestLanguage !== 'en'
    const pickTag = (p) => (isZh && p.taglineZh) ? p.taglineZh : (p.tagline || '')

    const resp = await getProjectList?.()
    if (requestID !== projectsRequest) return
    const list = Array.isArray(resp) ? resp : (resp?.list || [])
    if (Array.isArray(list)) {
      const a = list.filter((p) => isLaunchpadVisibleStatus(p.status))
      initiatives.value = a.map((p, i) => {
        const stage = deriveProjectStage(p)
        const fundingRoute = resolveProjectFundingRoute(p)
        const isFairLaunch = fundingRoute.ok && fundingRoute.kind === 'fair_launch'
        const isV3Project = fundingRoute.ok && fundingRoute.kind === 'ido_v3'
        const publicFunding = publicFundingProgress(p)
        const isLive = stage === 'live'
        const isSuccessful = stage === 'successful'
        const isPendingRelease = stage === 'successful_pending_release'
        const isFailed = stage === 'failed' || stage === 'ended_failed' || stage === 'refund'
        const isCanceled = stage === 'canceled'
        const isEnded = isSuccessful || isPendingRelease || isFailed || isCanceled || stage === 'ended_success' || stage === 'ended'
        let progress = 0
        if (isLive || isEnded) {
          if (isFairLaunch) {
            try { progress = decimalRatioPercent(p.totalRaised ?? '0', p.softCap ?? '0', 1) } catch { progress = '0.0' }
          } else if (isV3Project) {
            progress = publicFunding?.ready ? publicFunding.percent : null
          } else if (fundingRoute.ok && fundingRoute.kind === 'capped_presale') {
            progress = getFundingProgress(p.totalRaised, p.hardCap)
          }
        } else {
          progress = stage === 'warmup' ? 65 : 25
        }
        return {
          title: localizedProjectName(p, requestLanguage),
          category: p.category || 'DeSci',
          img: p.coverUrl || (i === 0 ? '/figma/v2/project-card-image-3.png' : '/figma/v2/project-card-image-4.png'),
          statusKind: isLive
            ? 'live'
            : isSuccessful
              ? 'successful'
              : isFailed
                ? 'failed'
                : isCanceled
                  ? 'canceled'
                  : isEnded
                    ? 'ended'
                    : stage === 'syncing'
                      ? 'syncing'
                    : stage === 'upcoming'
                      ? 'upcoming'
                      : 'research',
          desc: pickTag(p),
          milestone: isLive
            ? isFairLaunch ? 'Phase-1 read-only preview · No transactions' : isV3Project ? 'Check if your wallet can participate on the project page' : 'Live - Funding open'
            : stage === 'syncing'
              ? (isZh ? '資料同步維護中；項目資料可查看，錢包操作暫停' : 'Synchronization maintenance; project information remains available, wallet actions paused')
            : stage === 'upcoming'
              ? 'Upcoming - Not started'
              : stage === 'successful_pending_release'
                ? 'Ended - Awaiting release'
                : stage === 'ended_success'
                ? 'Ended - Awaiting settlement'
                : stage === 'ended_failed'
                  ? 'Ended - Below soft cap'
                  : stage === 'successful'
                    ? 'Ended - Funded'
                    : stage === 'failed' || stage === 'refund'
                      ? 'Ended - Below soft cap'
                      : stage === 'canceled'
                        ? 'Ended - Canceled'
                  : stage === 'warmup'
                    ? isV3Project ? 'Signing up does not guarantee access' : 'Warmup - Whitelist open'
                    : 'Pre-launch - Subscribe for updates',
          fundingKind: fundingRoute.kind,
          fundingRouteOk: fundingRoute.ok,
          // Unknown/unassigned funding identities remain fail-closed in the
          // action layer, but are not a useful customer-facing list label.
          modeBadge: isFairLaunch ? 'Fair Launch' : '',
          progress,
          fundingSyncing: isV3Project && !publicFunding?.ready,
          progressBar: stage === 'syncing' || (isV3Project && !publicFunding?.ready) ? 0 : clampPercentForBar(progress),
          grants: stage === 'syncing' ? '—' : isFairLaunch
            ? `Read-only · No hard cap · Soft cap ${formatDecimalString(p.softCap)} ${p.paymentToken || ''}`.trim()
            : fundingRoute.ok && (fundingRoute.kind === 'capped_presale' || isV3Project) && p.hardCap
              ? `${formatDecimalString(p.hardCap)} ${p.paymentToken || 'USDT'}`
              : 'Unavailable',
          stake: isFairLaunch
            ? `${formatCanonicalSaleTokenAmount(p)} ${p.tokenSymbol || ''}`.trim()
            : p.tokenSymbol ? `${p.tokenSymbol}` : 'TBD',
          onClick: () => router.push(`/launchpad/${p.projectID}`),
        }
      })
    }

    const candidateResp = await getProjectList?.(0, 'candidate')
    if (requestID !== projectsRequest) return
    const candidateList = Array.isArray(candidateResp) ? candidateResp : (candidateResp?.list || [])
    candidates.value = Array.isArray(candidateList) ? candidateList.map((p) => ({
      title: localizedProjectName(p, requestLanguage),
      category: p.category || 'DeSci',
      img: p.coverUrl || '/figma/v2/case-study-thumb.png',
      desc: pickTag(p),
      stage: 'Candidate',
      onClick: () => router.push(`/launchpad/${p.projectID}`),
    })) : []
  } catch (e) {
    if (requestID === projectsRequest) console.warn('[launchpad list]', e?.message)
  } finally {
    if (requestID === projectsRequest) projectsLoading = false
  }
  // 重测：异步内容到位后再 measure 一次
  setTimeout(() => { measure(); scrollRevealHandler() }, 800)
}

onMounted(async () => {
  window.scrollTo(0, 0)
  nextTick(() => { measure(); scrollRevealHandler() })
  setTimeout(() => { measure(); scrollRevealHandler() }, 500)
  window.addEventListener('resize', applyScale)
  window.addEventListener('resize', scrollRevealHandler)
  window.addEventListener('scroll', scrollRevealHandler, { passive: true })
  await loadProjects()
  if (disposed) return
  fundingRefreshTimer = setInterval(() => {
    if (!document.hidden && !projectsLoading) loadProjects()
  }, 60000)
})
watch(() => props.language, loadProjects)
onBeforeUnmount(() => {
  disposed = true
  clearInterval(fundingRefreshTimer)
  projectsRequest++
  window.removeEventListener('resize', applyScale)
  window.removeEventListener('resize', scrollRevealHandler)
  window.removeEventListener('scroll', scrollRevealHandler)
})

// ===== 资源 / 导航 =====
const A = '/figma/v2'
const img = (n) => `${A}/${n}`
const GUIDELINES_URL = 'https://joincare.gitbook.io/joincare-docs/incubation-model-and-project-workflow'
const handleApply       = () => router.push('/launchpad/apply')
const handleGuidelines  = () => window.open(GUIDELINES_URL, '_blank', 'noopener')

const t = computed(() => TRANSLATIONS[props.language] || TRANSLATIONS.en)

const initiatives = ref([])

const candidates = ref([])
</script>

<template>
  <JcFloatingNavbar :scale="scale" active="launchpad" />

  <!-- ========== MOBILE ========== -->
  <div v-if="isMobile" class="jc-mobile-page m-launchpad" :class="{ 'home-dark': isDark }">
    <!-- Hero -->
    <section class="m-hero">
      <div class="m-eyebrow">
        <span class="m-eyebrow-tag"></span>
        <span>{{ t.eyebrow }}</span>
      </div>
      <h1 class="m-hero-title">
        <span class="font-semibold">{{ t.h1L1 }}</span>
        <span class="font-playfair italic font-normal"> {{ t.h1L2 }}</span>
      </h1>
      <p class="m-hero-sub">{{ t.sub }}</p>
      <button class="m-btn-secondary" @click="handleGuidelines">
        {{ t.guidelines }}
        <span class="material-symbols-outlined">arrow_outward</span>
      </button>
    </section>

    <!-- Active Initiatives -->
    <section v-if="initiatives.length" class="m-section">
      <div class="m-section-head">
        <span class="m-eyebrow-tag"></span>
        <h2 class="m-section-title">{{ t.activeKicker }}</h2>
      </div>
      <div class="m-lp-list">
        <article v-for="(p, i) in initiatives" :key="i" class="m-lp-card" @click="p.onClick">
          <div class="m-lp-cover">
            <img :src="p.img" :alt="p.title" class="m-lp-cover-img" />
            <span v-if="p.statusKind === 'syncing'" class="m-lp-status m-lp-status-upcoming">{{ t.statusSyncing }}</span>
            <span v-else-if="p.statusKind === 'live'" class="m-lp-status m-lp-status-live">
              <span class="status-live-dot"></span>{{ t.statusLive }}
            </span>
            <span v-else-if="p.statusKind === 'successful'" class="m-lp-status m-lp-status-successful">{{ t.statusSuccessful }}</span>
            <span v-else-if="p.statusKind === 'failed'" class="m-lp-status m-lp-status-failed">{{ t.statusFailed }}</span>
            <span v-else-if="p.statusKind === 'canceled'" class="m-lp-status m-lp-status-canceled">{{ t.statusCanceled }}</span>
            <span v-else-if="p.statusKind === 'ended'" class="m-lp-status m-lp-status-ended">{{ t.statusEnded }}</span>
            <span v-else-if="p.statusKind === 'upcoming'" class="m-lp-status m-lp-status-upcoming">{{ t.statusUpcoming }}</span>
            <span v-else class="m-lp-status m-lp-status-research">{{ t.statusResearch }}</span>
          </div>
          <div class="m-lp-body">
            <div class="flex items-center gap-2"><span class="m-lp-cat">{{ p.categoryKey ? t[p.categoryKey] : p.category }}</span><span v-if="p.modeBadge" class="m-lp-mode">{{ p.modeBadge }}</span></div>
            <h3 class="m-lp-title">{{ p.title }}</h3>
            <p class="m-lp-desc">{{ p.descKey ? t[p.descKey] : p.desc }}</p>
            <div class="m-lp-milestone">
              <span>{{ p.milestoneKey ? t[p.milestoneKey] : p.milestone }}</span>
              <span class="m-lp-progress-num">{{ p.statusKind === 'syncing' || p.fundingSyncing ? (language === 'en' ? 'Syncing' : '同步中') : p.progress + '%' }}</span>
            </div>
            <div class="m-lp-progressbar">
              <div :style="{ width: p.progressBar + '%' }"></div>
            </div>
            <div class="m-lp-stats">
              <div>
                <p class="m-lp-stats-label">{{ t.grants }}</p>
                <p class="m-lp-stats-value">{{ p.grants }}</p>
              </div>
              <div>
                <p class="m-lp-stats-label">{{ t.stake }}</p>
                <p class="m-lp-stats-value">{{ p.stake }}</p>
              </div>
            </div>
            <div class="m-lp-cta">{{ t.viewDetail }} →</div>
          </div>
        </article>
      </div>
    </section>

    <!-- Candidate Projects -->
    <section v-if="candidates.length" class="m-section">
      <div class="m-section-head">
        <span class="m-eyebrow-tag"></span>
        <h2 class="m-section-title">{{ t.candidateKicker }}</h2>
      </div>
      <h3 class="m-section-headline">
        <span class="font-semibold">{{ t.candidateHeadL1 }}</span>
        <span class="font-playfair italic font-normal"> {{ t.candidateHeadL2 }}</span>
      </h3>
      <div class="m-card-list">
        <article v-for="(c, i) in candidates" :key="i" class="m-lp-mini">
          <img :src="c.img" :alt="c.title" class="m-lp-mini-cover" />
          <div class="m-lp-mini-body">
            <div class="m-lp-mini-meta">
              <span class="m-lp-cat">{{ c.categoryKey ? t[c.categoryKey] : c.category }}</span>
              <span class="m-lp-stage">{{ c.stageKey ? t[c.stageKey] : c.stage }}</span>
            </div>
            <h4>{{ c.title }}</h4>
            <p>{{ c.descKey ? t[c.descKey] : c.desc }}</p>
            <div class="m-lp-cta m-lp-cta-mini">{{ t.learn }} →</div>
          </div>
        </article>
      </div>
    </section>

    <!-- Researcher CTA -->
    <section class="m-cta-section">
      <div class="m-cta-inner">
        <h2 class="m-cta-title">{{ t.researcherHeadL1 }} <span class="font-playfair italic font-normal">{{ t.researcherHeadL2 }}</span></h2>
        <p class="m-cta-sub">{{ t.researcherBody }}</p>
        <ul class="m-researcher-bullets">
          <li v-for="b in t.researcherBullets" :key="b">
            <span class="material-symbols-outlined" style="font-variation-settings:'FILL' 1;">check_circle</span>
            {{ b }}
          </li>
        </ul>
        <button class="m-btn-primary" @click="handleApply">
          {{ t.startApp }}
          <span class="material-symbols-outlined">arrow_forward</span>
        </button>
      </div>
    </section>

    <HomeFooter :language="props.language" />
  </div>

  <!-- ========== DESKTOP ========== -->
  <div v-else class="jc-canvas-host" :class="{ 'home-dark': isDark }" :style="{ height: hostHeight }">
    <div class="jc-scaled-wrapper" :style="{ width: `${1440 * scale}px`, height: hostHeight }">
      <main ref="canvasRef" class="jc-canvas-1440" :style="{ transform: `scale(${scale})` }">

        <div class="h-[64px] w-full shrink-0"></div>

        <!-- ============ Launchpad + Active Initiatives 合并 ============ -->
        <section data-reveal class="relative w-full px-[80px] pt-[60px] pb-[120px] flex flex-col gap-[40px]">
          <div class="jc-hero-anim jc-hero-anim-1 flex items-center gap-[8px]">
            <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
            <span class="text-[20px] font-bold text-[#212121]">{{ t.eyebrow }}</span>
          </div>

          <div class="font-bold text-[#212121] tracking-[-2.88px]" style="font-size: 88px; line-height: 0.95; width: 1180px;">
            <p class="jc-hero-headline-line jc-hero-headline-line-1 mb-0">
              <span class="font-semibold">{{ t.h1L1 }}</span>
            </p>
            <p class="jc-hero-headline-line jc-hero-headline-line-2 font-semibold">
              <span class="font-playfair italic font-normal">{{ t.h1L2 }}</span>
            </p>
          </div>

          <div class="jc-hero-anim jc-hero-anim-3 flex items-end justify-between">
            <p class="text-[18px] font-bold text-[#212121] leading-[1.4]" style="width: 540px;">{{ t.sub }}</p>
            <div class="flex gap-[12px]">
              <button class="white-pill-btn" @click="handleGuidelines">
                <span class="text-[15px] font-bold text-[#212121]">{{ t.guidelines }}</span>
                <span class="arrow-icon arrow-diag">
                  <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                  <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                </span>
              </button>
            </div>
          </div>

          <template v-if="initiatives.length">
            <!-- 1px 分隔线，Hero ↔ Active Initiatives 视觉过渡 -->
            <div class="pointer-events-none mt-[24px] mb-[16px]" style="height: 1px; background: linear-gradient(to right, transparent 0%, rgba(0,0,0,0.08) 12%, rgba(0,0,0,0.08) 88%, transparent 100%);"></div>

            <!-- Active Initiatives 小标题行 -->
            <div class="flex items-center justify-between w-full">
              <span class="text-[13px] font-bold tracking-[1.5px] uppercase text-[#808080]">{{ t.activeKicker }}</span>
              <a class="text-[13px] font-bold text-[#212121] cursor-pointer hover:underline">{{ t.viewAll }} →</a>
            </div>

            <div class="grid grid-cols-2 gap-[20px] items-stretch w-full">
            <article
              v-for="(p, i) in initiatives" :key="i"
              :class="['train-item lp-card bg-white flex flex-col rounded-[16px] overflow-hidden relative cursor-pointer flex-1']"
              style="height: 480px;"
              @click="p.onClick"
            >
              <div class="relative h-[220px] w-full overflow-hidden">
                <img :alt="p.title" :src="p.img" class="lp-project-cover-img" loading="lazy" />
                <div class="absolute top-[16px] right-[16px]">
                  <span v-if="p.statusKind === 'syncing'" class="inline-flex items-center px-[12px] py-[6px] rounded-full bg-[#fff7e6] text-[#ad6800] text-[12px] font-bold">{{ t.statusSyncing }}</span>
                  <span v-else-if="p.statusKind === 'live'" class="funding-active-tag inline-flex items-center gap-[6px] px-[12px] py-[6px] rounded-full bg-[#9bff00] text-[12px] font-bold uppercase tracking-[1.2px] text-[#212121]">
                    <span class="status-live-dot w-[6px] h-[6px] rounded-full bg-[#212121]"></span>{{ t.statusLive }}
                  </span>
                  <span v-else-if="p.statusKind === 'successful'" class="inline-flex items-center px-[12px] py-[6px] rounded-full bg-[#dcfce7] text-[#166534] text-[12px] font-bold uppercase tracking-[1.2px]">{{ t.statusSuccessful }}</span>
                  <span v-else-if="p.statusKind === 'failed'" class="inline-flex items-center px-[12px] py-[6px] rounded-full bg-[#fee2e2] text-[#991b1b] text-[12px] font-bold uppercase tracking-[1.2px]">{{ t.statusFailed }}</span>
                  <span v-else-if="p.statusKind === 'canceled'" class="inline-flex items-center px-[12px] py-[6px] rounded-full bg-[#ffedd5] text-[#9a3412] text-[12px] font-bold uppercase tracking-[1.2px]">{{ t.statusCanceled }}</span>
                  <span v-else-if="p.statusKind === 'ended'" class="inline-flex items-center px-[12px] py-[6px] rounded-full bg-gray-400/80 text-white text-[12px] font-bold uppercase tracking-[1.2px]">{{ t.statusEnded }}</span>
                  <span v-else-if="p.statusKind === 'upcoming'" class="inline-flex items-center px-[12px] py-[6px] rounded-full bg-white/90 backdrop-blur text-[12px] font-bold uppercase tracking-[1.2px] text-[#212121]">{{ t.statusUpcoming }}</span>
                  <span v-else class="inline-flex items-center px-[12px] py-[6px] rounded-full bg-white/90 backdrop-blur text-[12px] font-bold uppercase tracking-[1.2px] text-[#212121]">{{ t.statusResearch }}</span>
                </div>
              </div>
              <div class="flex flex-col gap-[14px] p-[24px] flex-1">
                <div class="flex items-center gap-[8px]"><span class="text-[11px] font-bold tracking-[1.5px] uppercase text-[#808080]">{{ p.categoryKey ? t[p.categoryKey] : p.category }}</span><span v-if="p.modeBadge" class="lp-mode-badge">{{ p.modeBadge }}</span></div>
                <h3 class="text-[28px] font-bold text-[#212121] leading-none">{{ p.title }}</h3>
                <p class="text-[14px] font-medium text-[#212121] leading-[1.5]">{{ p.descKey ? t[p.descKey] : p.desc }}</p>
                <div class="mt-auto">
                  <div class="flex justify-between items-center mb-[8px]">
                    <span class="text-[11px] font-bold tracking-[1px] uppercase text-[#808080]">{{ p.milestoneKey ? t[p.milestoneKey] : p.milestone }}</span>
                    <span class="text-[12px] font-bold text-[#1aa64a]">{{ p.statusKind === 'syncing' || p.fundingSyncing ? (language === 'en' ? 'Syncing' : '同步中') : p.progress + '%' }}</span>
                  </div>
                  <div class="w-full bg-[#eee] rounded-full h-[6px] overflow-hidden">
                    <div class="bg-[#9bff00] h-[6px] rounded-full transition-all duration-700" :style="{ width: p.progressBar + '%' }"></div>
                  </div>
                  <div class="grid grid-cols-2 gap-[12px] mt-[16px]">
                    <div class="bg-[#f5f5f3] rounded-[8px] p-[12px]">
                      <div class="text-[10px] font-bold tracking-[1px] uppercase text-[#808080]">{{ t.grants }}</div>
                      <div class="text-[15px] font-bold text-[#212121] mt-[2px]">{{ p.grants }}</div>
                    </div>
                    <div class="bg-[#f5f5f3] rounded-[8px] p-[12px]">
                      <div class="text-[10px] font-bold tracking-[1px] uppercase text-[#808080]">{{ t.stake }}</div>
                      <div class="text-[15px] font-bold text-[#212121] mt-[2px]">{{ p.stake }}</div>
                    </div>
                  </div>
                </div>
              </div>
              <div class="lp-detail-btn absolute right-[16px] bottom-[16px] inline-flex items-center gap-[8px] bg-white rounded-[7px] pl-[14px] pr-[7px] py-[7px] shadow-[0_4px_16px_rgba(0,0,0,0.08)]">
                <span class="text-[13px] font-bold text-[#212121]">{{ t.viewDetail }}</span>
                <span class="arrow-icon arrow-diag" style="width:30px;height:30px;">
                  <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                  <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                </span>
              </div>
            </article>
            </div>
          </template>
        </section>

        <!-- ============ CANDIDATE PROJECTS ============ -->
        <section v-if="candidates.length" data-reveal class="relative w-full flex flex-col gap-[56px] px-[80px] py-[120px]">
          <div class="reveal-eyebrow flex items-center gap-[8px]">
            <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
            <span class="text-[20px] font-bold text-[#212121]">{{ t.candidateKicker }}</span>
          </div>

          <div class="font-bold text-[#212121] tracking-[-1.28px]" style="font-size: 64px; line-height: 1; width: 900px;">
            <p class="reveal-headline-line reveal-headline-line-1 mb-0">
              <span class="font-semibold">{{ t.candidateHeadL1 }}</span>
            </p>
            <p class="reveal-headline-line reveal-headline-line-2">
              <span class="font-playfair italic font-normal">{{ t.candidateHeadL2 }}</span>
            </p>
          </div>

          <div class="flex gap-[16px] items-stretch w-full">
            <article
              v-for="(c, i) in candidates" :key="i"
              :class="['rise-card lp-mini bg-white flex flex-col rounded-[12px] overflow-hidden flex-1 cursor-pointer', `rise-card-${i + 1}`]"
              style="height: 380px;"
            >
              <div class="relative h-[160px] w-full overflow-hidden">
                <img :alt="c.title" :src="c.img" class="lp-candidate-cover-img" loading="lazy" />
              </div>
              <div class="flex flex-col gap-[10px] p-[20px] flex-1">
                <div class="flex items-center justify-between">
                  <span class="text-[10px] font-bold tracking-[1.5px] uppercase text-[#808080]">{{ c.categoryKey ? t[c.categoryKey] : c.category }}</span>
                  <span class="text-[11px] font-bold text-[#1aa64a]">{{ c.stageKey ? t[c.stageKey] : c.stage }}</span>
                </div>
                <h4 class="text-[20px] font-bold text-[#212121] leading-tight">{{ c.title }}</h4>
                <p class="text-[13px] font-medium text-[#212121] leading-[1.5]">{{ c.descKey ? t[c.descKey] : c.desc }}</p>
                <div class="mt-auto pt-[8px] flex items-center gap-[6px] text-[12px] font-bold text-[#212121]">
                  {{ t.learn }} <span class="material-symbols-outlined text-[16px]">arrow_forward</span>
                </div>
              </div>
            </article>
          </div>
        </section>

        <!-- ============ RESEARCHER CTA ============ -->
        <section data-reveal class="relative w-full flex flex-col gap-[40px] px-[80px] py-[120px] items-center text-center">
          <div class="reveal-eyebrow flex items-center gap-[8px] justify-center">
            <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
            <span class="text-[20px] font-bold text-[#212121]">{{ t.researcherKicker }}</span>
          </div>

          <div class="font-bold text-[#212121] tracking-[-1.28px] text-center" style="font-size: 64px; line-height: 1; width: 1000px;">
            <p class="reveal-headline-line reveal-headline-line-1 mb-0">
              <span class="font-semibold">{{ t.researcherHeadL1 }}</span>
            </p>
            <p class="reveal-headline-line reveal-headline-line-2">
              <span class="font-playfair italic font-normal">{{ t.researcherHeadL2 }}</span>
            </p>
          </div>

          <p class="text-[18px] font-medium text-[#212121] leading-[1.5]" style="width: 720px;">{{ t.researcherBody }}</p>

          <ul class="flex gap-[32px]">
            <li v-for="b in t.researcherBullets" :key="b" class="flex items-center gap-[8px] text-[14px] font-bold text-[#212121]">
              <span class="material-symbols-outlined text-[#1aa64a]" style="font-variation-settings: 'FILL' 1;">check_circle</span>
              {{ b }}
            </li>
          </ul>

          <button class="white-pill-btn mt-[8px]" @click="handleApply">
            <span class="text-[15px] font-bold text-[#212121]">{{ t.startApp }}</span>
            <span class="arrow-icon arrow-diag">
              <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
              <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
            </span>
          </button>
        </section>

        <HomeFooter :language="props.language" full-bleed :scale="scale" />

      </main>
    </div>
  </div>
</template>

<style scoped>
.lp-project-cover-img,
.lp-candidate-cover-img {
  display: block;
  width: 100%;
  height: auto;
  max-width: none;
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
}
.material-symbols-outlined { font-variation-settings: 'FILL' 0, 'wght' 400, 'GRAD' 0, 'opsz' 24; }

/* Launchpad 卡片 hover：放大 + View Detail 按钮弹出（同首页 fp-card 模式） */
[data-reveal] .lp-card {
  transition:
    opacity 0.7s cubic-bezier(0.22, 1, 0.36, 1),
    transform 0.7s cubic-bezier(0.22, 1, 0.36, 1),
    scale 0.22s cubic-bezier(0, 0, 0.2, 1),
    box-shadow 0.22s cubic-bezier(0, 0, 0.2, 1);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.05);
}
[data-reveal] .lp-card:hover {
  scale: 1.018;
  box-shadow: 0 14px 36px -4px rgba(0, 0, 0, 0.18);
  z-index: 5;
}
.lp-detail-btn {
  opacity: 0;
  transform: translateY(8px);
  transition: opacity 0.16s ease-out, transform 0.16s ease-out;
  pointer-events: none;
}
[data-reveal] .lp-card:hover .lp-detail-btn {
  opacity: 1;
  transform: translateY(0);
  pointer-events: auto;
}
[data-reveal] .lp-mini {
  transition:
    opacity 0.75s cubic-bezier(0.22, 1, 0.36, 1),
    transform 0.75s cubic-bezier(0.22, 1, 0.36, 1),
    scale 0.22s cubic-bezier(0, 0, 0.2, 1),
    box-shadow 0.22s cubic-bezier(0, 0, 0.2, 1);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.05);
}
[data-reveal] .lp-mini:hover {
  scale: 1.02;
  box-shadow: 0 14px 36px -4px rgba(0, 0, 0, 0.16);
}

/* Funding Live 黑点：节奏稍快的明显闪烁 + 光晕 */
.status-live-dot {
  position: relative;
  animation: live-pulse 1.1s cubic-bezier(0.22, 1, 0.36, 1) infinite;
}
.status-live-dot::after {
  content: '';
  position: absolute;
  inset: -3px;
  border-radius: 50%;
  background: #212121;
  opacity: 0;
  animation: live-halo 1.1s cubic-bezier(0.22, 1, 0.36, 1) infinite;
  pointer-events: none;
}
@keyframes live-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50%      { opacity: 0.35; transform: scale(0.78); }
}
@keyframes live-halo {
  0%   { opacity: 0.45; transform: scale(0.55); }
  70%  { opacity: 0;    transform: scale(2.2);  }
  100% { opacity: 0;    transform: scale(2.2);  }
}

/* ============ Mobile-only styles ============ */
@media (max-width: 768px) {
  .m-lp-list {
    display: flex; flex-direction: column; gap: 16px; margin-top: 12px;
  }
  .m-lp-card {
    background: white;
    border: 1px solid #ececea;
    border-radius: 16px;
    overflow: hidden;
    cursor: pointer;
  }
  .home-dark .m-lp-card {
    background: #1f1f1f;
    border-color: rgba(255,255,255,0.06);
  }
  .m-lp-cover {
    position: relative; height: 180px; width: 100%; overflow: hidden;
  }
  .m-lp-cover-img { display: block; width: 100%; height: auto; max-width: none; position: absolute; left: 0; top: 50%; transform: translateY(-50%); }
  .m-lp-status {
    position: absolute; top: 12px; right: 12px;
    display: inline-flex; align-items: center; gap: 6px;
    padding: 4px 10px; border-radius: 999px;
    font-size: 10px; font-weight: 700; letter-spacing: 1px;
    text-transform: uppercase;
  }
  .m-lp-status-live { background: #9bff00; color: #212121; }
  .m-lp-status-successful { background: #dcfce7; color: #166534; }
  .m-lp-status-failed { background: #fee2e2; color: #991b1b; }
  .m-lp-status-canceled { background: #ffedd5; color: #9a3412; }
  .m-lp-status-ended { background: #d1d5db; color: #374151; }
  .m-lp-status-upcoming { background: #fef3c7; color: #92400e; }
  .m-lp-status-live .status-live-dot {
    width: 6px; height: 6px; border-radius: 50%; background: #212121;
  }
  .m-lp-status-research {
    background: rgba(255,255,255,0.85); color: #212121;
    backdrop-filter: blur(4px);
  }
  .m-lp-body {
    padding: 18px;
    display: flex; flex-direction: column; gap: 10px;
  }
  .m-lp-cat {
    font-size: 10px; font-weight: 700; letter-spacing: 1.5px;
    text-transform: uppercase; color: #808080;
  }
  .m-lp-mode {
    display: inline-flex; align-items: center; border: 1px solid rgba(26,166,74,.22);
    border-radius: 999px; background: rgba(155,255,0,.14); padding: 3px 7px;
    color: #16853e; font-size: 9px; font-weight: 800; letter-spacing: .8px; text-transform: uppercase;
  }
  .m-lp-title {
    font-size: 20px; font-weight: 700; color: #212121; margin: 0;
    line-height: 1.15;
  }
  .home-dark .m-lp-title { color: #fafaf7; }
  .m-lp-desc {
    font-size: 13px; color: #4c5148; line-height: 1.5; margin: 0;
  }
  .home-dark .m-lp-desc { color: #b8b8b0; }
  .m-lp-milestone {
    display: flex; justify-content: space-between; align-items: center;
    margin-top: 8px;
  }
  .m-lp-milestone > span:first-child {
    font-size: 10px; font-weight: 700; letter-spacing: 1px;
    text-transform: uppercase; color: #808080;
  }
  .m-lp-progress-num { font-size: 12px; font-weight: 700; color: #1aa64a; }
  .m-lp-progressbar {
    height: 6px; border-radius: 999px; overflow: hidden;
    background: #ececea;
  }
  .m-lp-progressbar > div {
    height: 100%; background: #9bff00;
    transition: width 0.7s ease;
  }
  .m-lp-stats {
    display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-top: 4px;
  }
  .m-lp-stats > div {
    background: #f5f5f3; border-radius: 8px; padding: 10px 12px;
  }
  .home-dark .m-lp-stats > div { background: #262626; }
  .m-lp-stats-label {
    font-size: 9px; font-weight: 700; letter-spacing: 1px;
    text-transform: uppercase; color: #808080; margin: 0 0 2px;
  }
  .m-lp-stats-value {
    font-size: 14px; font-weight: 700; color: #212121; margin: 0;
  }
  .home-dark .m-lp-stats-value { color: #fafaf7; }
  .m-lp-cta {
    margin-top: 6px;
    font-size: 12px; font-weight: 700; color: #1aa64a;
  }
  .m-lp-cta-mini { margin-top: 0; }

  .m-lp-mini {
    background: white;
    border: 1px solid #ececea;
    border-radius: 14px;
    overflow: hidden;
  }
  .home-dark .m-lp-mini {
    background: #1f1f1f;
    border-color: rgba(255,255,255,0.06);
  }
  .m-lp-mini-cover {
    display: block;
    width: auto;
    height: 140px;
    max-width: none;
    position: relative;
    left: 50%;
    transform: translateX(-50%);
  }
  .m-lp-mini-body {
    padding: 16px;
    display: flex; flex-direction: column; gap: 8px;
  }
  .m-lp-mini-body h4 {
    font-size: 16px; font-weight: 700; color: #212121; margin: 0;
  }
  .home-dark .m-lp-mini-body h4 { color: #fafaf7; }
  .m-lp-mini-body p {
    font-size: 13px; color: #4c5148; line-height: 1.5; margin: 0;
  }
  .home-dark .m-lp-mini-body p { color: #b8b8b0; }
  .m-lp-mini-meta {
    display: flex; justify-content: space-between; align-items: center;
  }
  .m-lp-stage {
    font-size: 11px; font-weight: 700; color: #1aa64a;
  }

  .m-researcher-bullets {
    list-style: none; padding: 0; margin: 4px 0 0;
    display: flex; flex-direction: column; gap: 8px;
    text-align: left; width: 100%;
  }
  .m-researcher-bullets li {
    display: flex; align-items: center; gap: 8px;
    font-size: 13px; font-weight: 700; color: #212121;
  }
  .m-researcher-bullets li .material-symbols-outlined {
    color: #1aa64a; font-size: 18px;
  }
}

.lp-mode-badge {
  display: inline-flex; align-items: center; border: 1px solid rgba(26,166,74,.22);
  border-radius: 999px; background: rgba(155,255,0,.14); padding: 3px 7px;
  color: #16853e; font-size: 9px; font-weight: 800; letter-spacing: .8px; text-transform: uppercase;
}

/* jc-shell 的暗色通用文字反色会把 text-[#212121] 改成白色；Active 状态
   使用专用高对比规则，始终保持荧光绿底黑字。 */
.home-dark .m-lp-mode,
.home-dark .lp-mode-badge { color: #9bff00; border-color: rgba(155,255,0,.28); background: rgba(155,255,0,.1); }
.home-dark .funding-active-tag,
.home-dark .funding-active-tag * {
  color: #212121 !important;
}
.home-dark .funding-active-tag {
  background-color: #9bff00 !important;
}
</style>
