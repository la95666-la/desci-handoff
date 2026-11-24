<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick, inject } from 'vue'
import { useRouter } from 'vue-router'
import { useWalletStore } from '../stores/wallet'
import JcFloatingNavbar from '../components/JcFloatingNavbar.vue'
import HomeFooter from '../components/HomeFooter.vue'
import { TRANSLATIONS } from '../i18n/ecosystem.js'
import { useIsMobile } from '../composables/useIsMobile'
import { SHOW_TASKS_ENTRY } from '../config/featureVisibility.js'
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

onMounted(() => {
  window.scrollTo(0, 0)
  nextTick(() => {
    measure()
    scrollRevealHandler()
  })
  setTimeout(() => { measure(); scrollRevealHandler() }, 500)
  window.addEventListener('resize', applyScale)
  window.addEventListener('resize', scrollRevealHandler)
  window.addEventListener('scroll', scrollRevealHandler, { passive: true })
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', applyScale)
  window.removeEventListener('resize', scrollRevealHandler)
  window.removeEventListener('scroll', scrollRevealHandler)
})

// ===== 资源 / 导航 =====
const A = '/figma/v2'
const img = (n) => `${A}/${n}`

const t = computed(() => TRANSLATIONS[props.language] || TRANSLATIONS.en)

const pillars = computed(() => [
  { title: t.value.pillarResearchersTitle,  body: t.value.pillarResearchersBody },
  { title: t.value.pillarAdvisorsTitle,     body: t.value.pillarAdvisorsBody },
  { title: t.value.pillarAgentsTitle,       body: t.value.pillarAgentsBody },
  { title: t.value.pillarContributorsTitle, body: t.value.pillarContributorsBody },
])

const utilities = computed(() => [
  { icon: 'rocket_launch',          title: t.value.utilLaunchpadTitle,  body: t.value.utilLaunchpadBody },
  { icon: 'verified_user',          title: t.value.utilGovernanceTitle, body: t.value.utilGovernanceBody },
  { icon: 'account_balance_wallet', title: t.value.utilPaymentsTitle,   body: t.value.utilPaymentsBody },
  { icon: 'stars',                  title: t.value.utilPerksTitle,      body: t.value.utilPerksBody },
])

const handleJoin    = () => { if (!wallet.isLoggedIn) wallet.connectWallet(); else router.push('/tasks') }
const handlePartner = () => window.open('mailto:partners@joincare.network', '_blank', 'noopener')
</script>

<template>
  <JcFloatingNavbar :scale="scale" active="ecosystem" />

  <!-- ==========  MOBILE 独立布局 ========== -->
  <div v-if="isMobile" class="jc-mobile-page m-ecosystem" :class="{ 'home-dark': isDark }">
    <!-- Hero -->
    <section class="m-hero">
      <div class="m-eyebrow">
        <span class="m-eyebrow-tag"></span>
        <span>{{ t.eyebrow }}</span>
      </div>
      <h1 class="m-hero-title">
        <span class="font-semibold">{{ t.h1Line1 }}</span>
        <span class="font-playfair italic font-normal"> {{ t.h1Line2 }}</span>
      </h1>
      <p class="m-hero-sub">{{ t.sub }}</p>
      <button v-if="SHOW_TASKS_ENTRY" class="m-btn-primary" @click="handleJoin">
        {{ t.joinBtn }}
        <span class="material-symbols-outlined">arrow_forward</span>
      </button>
    </section>

    <!-- Network Pillars -->
    <section class="m-section">
      <div class="m-section-head">
        <span class="m-eyebrow-tag"></span>
        <h2 class="m-section-title">{{ t.pillarsKicker }}</h2>
      </div>
      <h3 class="m-section-headline">
        <span class="font-semibold">{{ t.pillarsHeadL1 }}</span>
        <span class="font-playfair italic font-normal"> {{ t.pillarsHeadL2 }}</span>
      </h3>
      <p class="m-section-copy">{{ t.pillarsBody }}</p>
      <div class="m-card-list">
        <div v-for="(p, i) in pillars" :key="i" class="m-card">
          <span class="m-card-icon-tag">
            <span class="material-symbols-outlined" style="font-variation-settings:'FILL' 1;">{{ ['science','psychology','smart_toy','groups'][i] }}</span>
          </span>
          <h4>{{ p.title }}</h4>
          <p>{{ p.body }}</p>
        </div>
      </div>
    </section>

    <!-- Infrastructure -->
    <section class="m-section">
      <div class="m-section-head">
        <span class="m-eyebrow-tag"></span>
        <h2 class="m-section-title">{{ t.infraKicker }}</h2>
      </div>
      <h3 class="m-section-headline">
        <span class="font-semibold">{{ t.infraHeadL1 }}</span>
        <span class="font-playfair italic font-normal"> {{ t.infraHeadL2 }}</span>
      </h3>
      <div class="m-card-list">
        <article class="m-card m-card-hero">
          <span class="m-card-pill">
            <span class="wallet-dot"></span>{{ t.coreProtocol }}
          </span>
          <h4>{{ t.coordTitle }}</h4>
          <p>{{ t.coordBody }}</p>
        </article>
        <article class="m-card">
          <span class="material-symbols-outlined m-card-icon" style="font-variation-settings:'FILL' 1;">verified_user</span>
          <h4>{{ t.verifTitle }}</h4>
          <p>{{ t.verifBody }}</p>
        </article>
        <article class="m-card">
          <span class="material-symbols-outlined m-card-icon" style="font-variation-settings:'FILL' 1;">rocket_launch</span>
          <h4>{{ t.incubTitle }}</h4>
          <p>{{ t.incubBody }}</p>
        </article>
      </div>
    </section>

    <!-- JC Token -->
    <section class="m-section">
      <div class="m-section-head">
        <span class="m-eyebrow-tag"></span>
        <h2 class="m-section-title">{{ t.jcKicker }}</h2>
      </div>
      <h3 class="m-section-headline">
        <span class="font-semibold">{{ t.jcHeadL1 }}</span>
        <span class="font-playfair italic font-normal"> {{ t.jcHeadL2 }}</span>
      </h3>
      <p class="m-utility-kicker">{{ t.utilityKicker }}</p>
      <div class="m-card-grid">
        <article v-for="u in utilities" :key="u.title" class="m-card">
          <span class="m-card-icon-tag">
            <span class="material-symbols-outlined" style="font-variation-settings:'FILL' 1;">{{ u.icon }}</span>
          </span>
          <h4>{{ u.title }}</h4>
          <p>{{ u.body }}</p>
        </article>
      </div>
    </section>

    <!-- CTA -->
    <section class="m-cta-section">
      <div class="m-cta-inner">
        <h2 class="m-cta-title">{{ t.ctaHeadL1 }} <span class="font-playfair italic font-normal">{{ t.ctaHeadL2 }}</span></h2>
        <p class="m-cta-sub">{{ t.ctaBody }}</p>
        <button v-if="SHOW_TASKS_ENTRY" class="m-btn-primary" @click="handleJoin">
          {{ t.joinBtn }}
          <span class="material-symbols-outlined">arrow_forward</span>
        </button>
      </div>
    </section>

    <HomeFooter :language="props.language" />
  </div>

  <!-- ========== DESKTOP 原有 1440 canvas ========== -->
  <div v-else class="jc-canvas-host" :class="{ 'home-dark': isDark }" :style="{ height: hostHeight }">
    <div class="jc-scaled-wrapper" :style="{ width: `${1440 * scale}px`, height: hostHeight }">
      <main ref="canvasRef" class="jc-canvas-1440" :style="{ transform: `scale(${scale})` }">

        <!-- Navbar spacer -->
        <div class="h-[64px] w-full shrink-0"></div>

        <!-- ============ HERO ============ -->
        <section class="relative w-full px-[80px] pt-[60px] pb-[120px] flex flex-col gap-[40px]">
          <!-- Eyebrow -->
          <div class="jc-hero-anim jc-hero-anim-1 flex items-center gap-[8px]">
            <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
            <span class="text-[20px] font-bold text-[#212121]">{{ t.eyebrow }}</span>
          </div>

          <!-- Big headline -->
          <div class="font-bold text-[#212121] tracking-[-2.88px]" style="font-size: 88px; line-height: 0.95; width: 1180px;">
            <p class="jc-hero-headline-line jc-hero-headline-line-1 mb-0">
              <span class="font-semibold">{{ t.h1Line1 }}</span>
            </p>
            <p class="jc-hero-headline-line jc-hero-headline-line-2 font-semibold">
              <span class="font-playfair italic font-normal">{{ t.h1Line2 }}</span>
            </p>
          </div>

          <!-- Sub + CTA row -->
          <div class="jc-hero-anim jc-hero-anim-3 flex items-end justify-between">
            <p class="text-[18px] font-bold text-[#212121] leading-[1.4]" style="width: 540px;">{{ t.sub }}</p>
            <button v-if="SHOW_TASKS_ENTRY" class="white-pill-btn" @click="handleJoin">
              <span class="text-[15px] font-bold text-[#212121]">{{ t.joinBtn }}</span>
              <span class="arrow-icon arrow-diag">
                <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
              </span>
            </button>
          </div>

          <!-- 地平线 -->
          <div class="absolute left-0 right-0 bottom-0 pointer-events-none" style="height: 1px; background: linear-gradient(to right, transparent 0%, rgba(0,0,0,0.08) 12%, rgba(0,0,0,0.08) 88%, transparent 100%);"></div>
        </section>

        <!-- ============ NETWORK PILLARS ============ -->
        <section data-reveal class="relative w-full flex flex-col gap-[56px] px-[80px] py-[120px]">
          <div class="reveal-eyebrow flex items-center gap-[8px]">
            <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
            <span class="text-[20px] font-bold text-[#212121]">{{ t.pillarsKicker }}</span>
          </div>

          <div class="flex gap-[64px] items-start">
            <div class="font-bold text-[#212121] tracking-[-1.28px]" style="font-size: 64px; line-height: 1; width: 760px;">
              <p class="reveal-headline-line reveal-headline-line-1 mb-0">
                <span class="font-semibold">{{ t.pillarsHeadL1 }}</span>
              </p>
              <p class="reveal-headline-line reveal-headline-line-2">
                <span class="font-playfair italic font-normal">{{ t.pillarsHeadL2 }}</span>
              </p>
            </div>
            <p class="text-[16px] font-medium text-[#212121] leading-[1.45]" style="width: 360px;">{{ t.pillarsBody }}</p>
          </div>

          <div class="flex gap-[5px] items-stretch w-full">
            <div
              v-for="(p, i) in pillars" :key="i"
              :class="['scale-pop bg-white flex flex-col gap-[16px] flex-1 h-[260px] p-[28px] rounded-[8px]', `scale-pop-${i + 1}`]"
            >
              <span class="w-[44px] h-[44px] rounded-[10px] bg-[#9bff00] flex items-center justify-center">
                <span class="material-symbols-outlined text-[#212121]" style="font-variation-settings: 'FILL' 1;">{{ ['science','psychology','smart_toy','groups'][i] }}</span>
              </span>
              <h3 class="text-[24px] font-bold text-[#212121] leading-none">{{ p.title }}</h3>
              <p class="text-[14px] font-medium text-[#212121] leading-[1.45]">{{ p.body }}</p>
            </div>
          </div>
        </section>

        <!-- ============ INFRASTRUCTURE ============ -->
        <section data-reveal class="relative w-full flex flex-col gap-[56px] px-[80px] py-[120px]">
          <div class="reveal-eyebrow flex items-center gap-[8px]">
            <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
            <span class="text-[20px] font-bold text-[#212121]">{{ t.infraKicker }}</span>
          </div>

          <div class="font-bold text-[#212121] tracking-[-1.28px]" style="font-size: 64px; line-height: 1; width: 1100px;">
            <p class="reveal-headline-line reveal-headline-line-1 mb-0">
              <span class="font-semibold">{{ t.infraHeadL1 }}</span>
            </p>
            <p class="reveal-headline-line reveal-headline-line-2">
              <span class="font-playfair italic font-normal">{{ t.infraHeadL2 }}</span>
            </p>
          </div>

          <div class="flex gap-[16px] items-stretch w-full">
            <!-- Large feature -->
            <article class="rise-card rise-card-1 bg-white flex flex-col gap-[20px] p-[32px] rounded-[12px]" style="width: 760px; height: 380px;">
              <span class="inline-flex items-center gap-[8px] px-[12px] py-[6px] rounded-full bg-[#9bff00]/30 self-start">
                <span class="w-[8px] h-[8px] rounded-full bg-[#9bff00] animate-pulse"></span>
                <span class="text-[12px] font-bold tracking-[1.2px] uppercase text-[#212121]">{{ t.coreProtocol }}</span>
              </span>
              <h3 class="text-[36px] font-bold text-[#212121] leading-tight">{{ t.coordTitle }}</h3>
              <p class="text-[16px] font-medium text-[#212121] leading-[1.5]" style="width: 540px;">{{ t.coordBody }}</p>
              <div class="flex-1 rounded-[8px] bg-[radial-gradient(circle_at_center,_rgba(155,255,0,0.18),_transparent_70%)] flex items-center justify-center">
                <span class="material-symbols-outlined text-[80px] text-[#4a8c00] animate-spin" style="animation-duration:14s;">share_windows</span>
              </div>
            </article>
            <div class="flex flex-col gap-[16px] flex-1">
              <article class="rise-card rise-card-2 bg-white flex flex-col gap-[12px] p-[28px] rounded-[12px] flex-1">
                <span class="material-symbols-outlined text-[32px] text-[#212121]" style="font-variation-settings: 'FILL' 1;">verified_user</span>
                <h4 class="text-[22px] font-bold text-[#212121] leading-tight">{{ t.verifTitle }}</h4>
                <p class="text-[14px] font-medium text-[#212121] leading-[1.5]">{{ t.verifBody }}</p>
              </article>
              <article class="rise-card rise-card-3 bg-white flex flex-col gap-[12px] p-[28px] rounded-[12px] flex-1">
                <span class="material-symbols-outlined text-[32px] text-[#212121]" style="font-variation-settings: 'FILL' 1;">rocket_launch</span>
                <h4 class="text-[22px] font-bold text-[#212121] leading-tight">{{ t.incubTitle }}</h4>
                <p class="text-[14px] font-medium text-[#212121] leading-[1.5]">{{ t.incubBody }}</p>
              </article>
            </div>
          </div>
        </section>

        <!-- ============ JC TOKEN ============ -->
        <section data-reveal class="relative w-full flex flex-col gap-[56px] px-[80px] py-[120px]">
          <div class="reveal-eyebrow flex items-center gap-[8px]">
            <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
            <span class="text-[20px] font-bold text-[#212121]">{{ t.jcKicker }}</span>
          </div>

          <div class="font-bold text-[#212121] tracking-[-1.28px]" style="font-size: 64px; line-height: 1; width: 1100px;">
            <p class="reveal-headline-line reveal-headline-line-1 mb-0 whitespace-nowrap">
              <span class="font-semibold">{{ t.jcHeadL1 }}</span>
              <span class="font-playfair italic font-normal"> {{ t.jcHeadL2 }}</span>
            </p>
          </div>

          <!-- Token utilities：4 张高级用途卡（具体代币经济学/分配方案待后期确定，故不展示）-->
          <div class="flex flex-col gap-[20px] w-full">
            <div class="text-[12px] font-bold tracking-[1.5px] uppercase text-[#808080]">{{ t.utilityKicker }}</div>
            <div class="grid grid-cols-4 gap-[16px]">
              <article
                v-for="(u, i) in utilities" :key="u.title"
                :class="['rise-card bg-white flex flex-col gap-[12px] p-[28px] rounded-[12px] h-[220px]', `rise-card-${i + 1}`]"
              >
                <span class="inline-flex h-[44px] w-[44px] items-center justify-center rounded-[10px] bg-[#9bff00]/30">
                  <span class="material-symbols-outlined text-[#212121]" style="font-variation-settings: 'FILL' 1;">{{ u.icon }}</span>
                </span>
                <h4 class="text-[20px] font-bold text-[#212121] leading-none">{{ u.title }}</h4>
                <p class="text-[14px] font-medium text-[#212121] leading-[1.5]">{{ u.body }}</p>
              </article>
            </div>
          </div>
        </section>

        <!-- ============ CTA ============ -->
        <section data-reveal class="relative w-full flex flex-col gap-[40px] px-[80px] py-[120px] items-center text-center">
          <div class="reveal-eyebrow flex items-center gap-[8px] justify-center">
            <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
            <span class="text-[20px] font-bold text-[#212121]">{{ t.ctaKicker }}</span>
          </div>

          <div class="font-bold text-[#212121] tracking-[-1.28px] text-center" style="font-size: 64px; line-height: 1; width: 1000px;">
            <p class="reveal-headline-line reveal-headline-line-1 mb-0">
              <span class="font-semibold">{{ t.ctaHeadL1 }}</span>
            </p>
            <p class="reveal-headline-line reveal-headline-line-2">
              <span class="font-playfair italic font-normal">{{ t.ctaHeadL2 }}</span>
            </p>
          </div>

          <p class="text-[18px] font-medium text-[#212121] leading-[1.5]" style="width: 720px;">{{ t.ctaBody }}</p>

          <div class="flex gap-[16px]">
            <button v-if="SHOW_TASKS_ENTRY" class="white-pill-btn" @click="handleJoin">
              <span class="text-[15px] font-bold text-[#212121]">{{ t.joinBtn }}</span>
              <span class="arrow-icon arrow-diag">
                <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
              </span>
            </button>
            <button class="white-pill-btn" @click="handlePartner">
              <span class="text-[15px] font-bold text-[#212121]">{{ t.partnerBtn }}</span>
              <span class="arrow-icon arrow-diag">
                <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
              </span>
            </button>
          </div>
        </section>

        <HomeFooter :language="props.language" full-bleed :scale="scale" />

      </main>
    </div>
  </div>
</template>

<style scoped>
.material-symbols-outlined { font-variation-settings: 'FILL' 0, 'wght' 400, 'GRAD' 0, 'opsz' 24; }
</style>
