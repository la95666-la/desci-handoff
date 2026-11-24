<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick, inject } from 'vue'
import { useRouter } from 'vue-router'
import { newProject } from '../services/api'
import { useWalletStore } from '../stores/wallet'
import JcFloatingNavbar from '../components/JcFloatingNavbar.vue'
import HomeFooter from '../components/HomeFooter.vue'
import { TRANSLATIONS } from '../i18n/projectapply.js'
import { useIsMobile } from '../composables/useIsMobile'

const props = defineProps({ language: { type: String, default: 'en' } })
const router = useRouter()
const walletStore = useWalletStore()
const isDark = inject('isHomeDark', ref(false))
const { isMobile } = useIsMobile()

// ===== 1440 画布缩放 (same pattern as TaskHall) =====
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
  if (isMobile.value) {
    scale.value = 1
    hostHeight.value = 'auto'
    return
  }
  const w = window.innerWidth
  const raw = w / DESIGN_WIDTH
  scale.value = Math.min(Math.max(raw, MIN_SCALE), MAX_SCALE)
  hostHeight.value = `${naturalHeight.value * scale.value}px`
}

const scrollRevealHandler = () => {
  const els = document.querySelectorAll('[data-reveal]:not(.is-revealed)')
  if (!els.length) return
  const trigger = window.innerHeight * 0.80
  els.forEach((el) => {
    if (el.getBoundingClientRect().top < trigger) el.classList.add('is-revealed')
  })
}

const A = '/figma/v2'
const img = (n) => `${A}/${n}`

const t = computed(() => TRANSLATIONS[props.language] || TRANSLATIONS.en)

// ============================================================
// FORM LOGIC — preserved 100% from original (do not modify)
// ============================================================
const currentStep = ref(1)
const totalSteps = 6

const form = ref({
  // Section 1
  name: '',
  city: '',
  email: '',
  wechat: '',
  telegram: '',
  // Section 2
  field: '',
  identity: '',
  institution: '',
  // Section 3
  projectName: '',
  oneLiner: '',
  direction: '',
  // Section 4
  stage: '',
  assets: [],
  // Section 5
  reason: '',
  support: [],
  // Section 6
  pdfLink: '',
  otherInfo: ''
})

const isSubmitting = ref(false)
const isSubmitted = ref(false)
const submitError = ref('')

const canProceed = computed(() => {
  const f = form.value
  switch (currentStep.value) {
    case 1: return f.name && f.email && (f.wechat || f.telegram)
    case 2: return f.field && f.identity
    case 3: return f.projectName && f.oneLiner && f.direction
    case 4: return f.stage
    case 5: return f.reason && f.support.length > 0
    case 6: return true
    default: return false
  }
})

const stepTitles = computed(() => [
  t.value.section1,
  t.value.section2,
  t.value.section3,
  t.value.section4,
  t.value.section5,
  t.value.section6,
])
const stepKickers = computed(() => [
  t.value.section1Kicker,
  t.value.section2Kicker,
  t.value.section3Kicker,
  t.value.section4Kicker,
  t.value.section5Kicker,
  t.value.section6Kicker,
])
// Icons must be in the index.html allowlist
const stepIconNames = ['description', 'psychology', 'science', 'hub', 'groups', 'share']

const progressPct = computed(() => (currentStep.value / totalSteps) * 100)

function nextStep() {
  if (currentStep.value < totalSteps) {
    currentStep.value++
    nextTick(() => { measure(); scrollRevealHandler() })
  }
}

function prevStep() {
  if (currentStep.value > 1) {
    currentStep.value--
    nextTick(() => { measure(); scrollRevealHandler() })
  }
}

function jumpStep(step) {
  if (step <= currentStep.value) {
    currentStep.value = step
    nextTick(() => { measure(); scrollRevealHandler() })
  }
}

function toggleMultiSelect(arr, value) {
  const idx = arr.indexOf(value)
  if (idx === -1) {
    arr.push(value)
  } else {
    arr.splice(idx, 1)
  }
}

async function handleSubmit() {
  isSubmitting.value = true
  submitError.value = ''
  try {
    const f = form.value
    // 映射前端字段 -> 後端 NewProjectRequest json key
    const requestBody = {
      name: f.name,
      city: f.city,
      email: f.email,
      wechat: f.wechat,
      telegram: f.telegram,
      major: f.field,
      identity: f.identity,
      organization: f.institution || '',
      projectName: f.projectName,
      projectDesc: f.oneLiner,
      projectCategory: f.direction,
      currentStep: f.stage,
      possessions: f.assets,
      applyReason: f.reason,
      supportWanted: f.support,
      projectLink: f.pdfLink || '',
      additionalInfo: f.otherInfo || '',
      walletAddress: walletStore.address
    }
    await newProject(requestBody)
    isSubmitted.value = true
    nextTick(() => { measure(); scrollRevealHandler() })
  } catch (err) {
    console.error('Submit failed:', err)
    submitError.value = err.message || (props.language === 'tw' ? '提交失敗，請重試' : 'Submission failed, please try again')
  } finally {
    isSubmitting.value = false
  }
}

function goBack() {
  router.push('/launchpad')
}

onMounted(() => {
  window.scrollTo(0, 0)
  nextTick(() => { measure(); scrollRevealHandler() })
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
</script>

<template>
  <JcFloatingNavbar :scale="scale" active="launchpad" />

  <div class="jc-canvas-host project-apply-host" :class="{ 'home-dark': isDark, 'is-mobile': isMobile }" :style="{ height: hostHeight }">
    <div class="jc-scaled-wrapper project-apply-wrapper" :style="{ width: isMobile ? '100%' : `${1440 * scale}px`, height: hostHeight }">
      <main ref="canvasRef" class="jc-canvas-1440 project-apply-page" :style="{ transform: isMobile ? 'none' : `scale(${scale})` }">

        <div class="apply-nav-spacer h-[64px] w-full shrink-0"></div>

        <!-- ============ HERO ============ -->
        <section data-reveal class="apply-hero relative w-full px-[80px] pt-[60px] pb-[60px] flex flex-col gap-[40px]">
          <!-- Eyebrow -->
          <div class="jc-hero-anim jc-hero-anim-1 flex items-center gap-[8px]">
            <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
            <span class="text-[20px] font-bold text-[#212121]">{{ t.eyebrow }}</span>
          </div>

          <!-- 88px 大标题 -->
          <div class="apply-hero-title font-bold text-[#212121] tracking-[-2.88px]" style="font-size: 88px; line-height: 0.95; width: 1180px;">
            <p class="jc-hero-headline-line jc-hero-headline-line-1 mb-0">
              <span class="font-semibold">{{ t.h1L1 }}</span>
            </p>
            <p class="jc-hero-headline-line jc-hero-headline-line-2 font-semibold">
              <span class="font-playfair italic font-normal">{{ t.h1L2 }}</span>
            </p>
          </div>

          <!-- subtitle + back CTA -->
          <div class="apply-hero-actions jc-hero-anim jc-hero-anim-3 flex items-end justify-between gap-[32px]">
            <p class="apply-hero-sub text-[18px] font-bold text-[#212121] leading-[1.4]" style="width: 620px;">{{ t.sub }}</p>
            <button class="white-pill-btn" @click="goBack">
              <span class="text-[15px] font-bold text-[#212121]">{{ t.back }}</span>
              <span class="arrow-icon arrow-diag">
                <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
              </span>
            </button>
          </div>

          <!-- 视觉分隔 -->
          <div class="pointer-events-none" style="height: 1px; background: linear-gradient(to right, transparent 0%, rgba(0,0,0,0.08) 12%, rgba(0,0,0,0.08) 88%, transparent 100%);"></div>
        </section>

        <!-- ============ SUCCESS STATE ============ -->
        <section v-if="isSubmitted" data-reveal class="apply-success-section relative w-full px-[80px] pt-[20px] pb-[120px] flex flex-col items-center">
          <article class="apply-success-card rise-card rise-card-1 bg-white flex flex-col items-center text-center gap-[24px] p-[60px] rounded-[28px] shadow-[0_4px_28px_rgba(0,0,0,0.04)]" style="width: 720px;">
            <span class="inline-flex w-[96px] h-[96px] items-center justify-center rounded-full bg-[#9bff00] success-pop">
              <span class="material-symbols-outlined text-[#212121] text-[56px]" style="font-variation-settings: 'FILL' 1;">check_circle</span>
            </span>
            <h2 class="text-[44px] font-bold text-[#212121] leading-tight tracking-[-0.88px]">
              <span class="font-playfair italic font-normal">{{ t.successTitle }}</span>
            </h2>
            <p class="text-[16px] font-medium text-[#212121] leading-[1.6] max-w-[480px]">{{ t.successMessage }}</p>
            <button class="white-pill-btn mt-[12px]" @click="goBack">
              <span class="text-[15px] font-bold text-[#212121]">{{ t.backToLaunchpad }}</span>
              <span class="arrow-icon arrow-diag">
                <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
              </span>
            </button>
          </article>
        </section>

        <!-- ============ STEP FORM ============ -->
        <section v-else data-reveal class="apply-form-section relative w-full px-[80px] pt-[20px] pb-[120px] flex flex-col gap-[28px]">

          <!-- Continuous progress bar -->
          <div class="apply-progress-card rise-card rise-card-1 bg-white flex flex-col gap-[14px] p-[24px] rounded-[12px]">
            <div class="apply-progress-head flex items-center justify-between">
              <div class="apply-progress-title flex items-center gap-[10px]">
                <span class="material-symbols-outlined text-[#1aa64a] text-[18px]" style="font-variation-settings: 'FILL' 1;">{{ stepIconNames[currentStep - 1] }}</span>
                <span class="text-[11px] font-bold tracking-[1.5px] uppercase text-[#808080]">{{ t.progressLabel }} {{ currentStep }} / {{ totalSteps }}</span>
                <span class="text-[14px] font-bold text-[#212121]">· {{ stepTitles[currentStep - 1] }}</span>
              </div>
              <span class="text-[12px] font-bold text-[#1aa64a]">{{ Math.round(progressPct) }}%</span>
            </div>
            <div class="progress-track">
              <div class="progress-fill" :style="{ width: progressPct + '%' }"></div>
            </div>
            <!-- step nav dots -->
            <div class="flex items-center gap-[8px]">
              <button
                v-for="step in totalSteps"
                :key="step"
                type="button"
                @click="jumpStep(step)"
                :disabled="step > currentStep"
                :class="[
                  'apply-step-pill',
                  step === currentStep ? 'is-active' : step < currentStep ? 'is-done' : 'is-future'
                ]"
              >
                <span v-if="step < currentStep" class="material-symbols-outlined text-[12px]" style="font-variation-settings: 'FILL' 1;">check</span>
                <span v-else>{{ step }}</span>
              </button>
            </div>
          </div>

          <!-- ===== Step 1: Personal Info ===== -->
          <article v-show="currentStep === 1" class="apply-form-card task-card rise-card rise-card-2 bg-white flex flex-col gap-[24px] p-[28px] rounded-[12px]">
            <header class="flex items-start justify-between gap-[24px]">
              <div class="flex items-center gap-[14px]">
                <span class="inline-flex w-[44px] h-[44px] items-center justify-center rounded-[10px] bg-[#9bff00]">
                  <span class="material-symbols-outlined text-[#212121] text-[22px]" style="font-variation-settings: 'FILL' 1;">description</span>
                </span>
                <div class="flex flex-col gap-[2px]">
                  <span class="text-[11px] font-bold tracking-[1.5px] uppercase text-[#808080]">{{ t.section1Kicker }}</span>
                  <h3 class="text-[22px] font-bold text-[#212121] leading-tight">{{ t.section1 }}</h3>
                </div>
              </div>
              <span class="inline-flex items-center gap-[6px] px-[10px] py-[4px] rounded-full bg-[#f5f5f3] text-[10px] font-bold uppercase tracking-[1.2px] text-[#808080] shrink-0">
                {{ currentStep }} / {{ totalSteps }}
              </span>
            </header>

            <div class="apply-two-grid grid grid-cols-2 gap-[18px]">
              <div class="flex flex-col gap-[8px]">
                <label class="text-[12px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.name }} <span class="text-[#1aa64a]">*</span></label>
                <input v-model="form.name" type="text" required :placeholder="t.namePlaceholder" class="apply-input" />
              </div>
              <div class="flex flex-col gap-[8px]">
                <label class="text-[12px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.city }}</label>
                <input v-model="form.city" type="text" :placeholder="t.cityPlaceholder" class="apply-input" />
              </div>
              <div class="flex flex-col gap-[8px] col-span-2">
                <label class="text-[12px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.email }} <span class="text-[#1aa64a]">*</span></label>
                <input v-model="form.email" type="email" required :placeholder="t.emailPlaceholder" class="apply-input" />
              </div>

              <div class="flex flex-col gap-[8px] col-span-2">
                <div class="flex items-center gap-[10px]">
                  <label class="text-[12px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.wechat }} / {{ t.telegram }} <span class="text-[#1aa64a]">*</span></label>
                  <span class="text-[11px] font-medium text-[#808080]">{{ t.contactHint }}</span>
                </div>
                <div class="apply-contact-grid grid grid-cols-2 gap-[12px]">
                  <div class="relative">
                    <span class="absolute left-[14px] top-1/2 -translate-y-1/2 text-[11px] font-bold text-[#1aa64a] uppercase tracking-[1px]">{{ t.wechat }}</span>
                    <input v-model="form.wechat" type="text" :placeholder="t.wechatPlaceholder"
                      class="apply-input"
                      :class="!form.wechat && !form.telegram ? 'is-error' : ''"
                      style="padding-left: 78px;" />
                  </div>
                  <div class="relative">
                    <span class="absolute left-[14px] top-1/2 -translate-y-1/2 text-[11px] font-bold text-[#3884ff] uppercase tracking-[1px]">{{ t.telegram }}</span>
                    <input v-model="form.telegram" type="text" :placeholder="t.telegramPlaceholder"
                      class="apply-input"
                      :class="!form.wechat && !form.telegram ? 'is-error' : ''"
                      style="padding-left: 92px;" />
                  </div>
                </div>
              </div>
            </div>
          </article>

          <!-- ===== Step 2: Professional Background ===== -->
          <article v-show="currentStep === 2" class="apply-form-card task-card rise-card rise-card-2 bg-white flex flex-col gap-[24px] p-[28px] rounded-[12px]">
            <header class="flex items-start justify-between gap-[24px]">
              <div class="flex items-center gap-[14px]">
                <span class="inline-flex w-[44px] h-[44px] items-center justify-center rounded-[10px] bg-[#9bff00]">
                  <span class="material-symbols-outlined text-[#212121] text-[22px]" style="font-variation-settings: 'FILL' 1;">psychology</span>
                </span>
                <div class="flex flex-col gap-[2px]">
                  <span class="text-[11px] font-bold tracking-[1.5px] uppercase text-[#808080]">{{ t.section2Kicker }}</span>
                  <h3 class="text-[22px] font-bold text-[#212121] leading-tight">{{ t.section2 }}</h3>
                </div>
              </div>
              <span class="inline-flex items-center gap-[6px] px-[10px] py-[4px] rounded-full bg-[#f5f5f3] text-[10px] font-bold uppercase tracking-[1.2px] text-[#808080] shrink-0">
                {{ currentStep }} / {{ totalSteps }}
              </span>
            </header>

            <div class="flex flex-col gap-[18px]">
              <div class="flex flex-col gap-[8px]">
                <label class="text-[12px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.field }} <span class="text-[#1aa64a]">*</span></label>
                <input v-model="form.field" type="text" required :placeholder="t.fieldPlaceholder" class="apply-input" />
              </div>
              <div class="flex flex-col gap-[10px]">
                <label class="text-[12px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.identity }} <span class="text-[#1aa64a]">*</span></label>
                <div class="apply-option-grid grid grid-cols-3 gap-[10px]">
                  <button
                    v-for="option in t.identityOptions"
                    :key="option"
                    type="button"
                    @click="form.identity = option"
                    :class="['apply-chip', form.identity === option ? 'is-selected' : '']"
                  >{{ option }}</button>
                </div>
              </div>
              <div class="flex flex-col gap-[8px]">
                <label class="text-[12px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.institution }}</label>
                <input v-model="form.institution" type="text" :placeholder="t.institutionPlaceholder" class="apply-input" />
              </div>
            </div>
          </article>

          <!-- ===== Step 3: Project Basics ===== -->
          <article v-show="currentStep === 3" class="apply-form-card task-card rise-card rise-card-2 bg-white flex flex-col gap-[24px] p-[28px] rounded-[12px]">
            <header class="flex items-start justify-between gap-[24px]">
              <div class="flex items-center gap-[14px]">
                <span class="inline-flex w-[44px] h-[44px] items-center justify-center rounded-[10px] bg-[#9bff00]">
                  <span class="material-symbols-outlined text-[#212121] text-[22px]" style="font-variation-settings: 'FILL' 1;">science</span>
                </span>
                <div class="flex flex-col gap-[2px]">
                  <span class="text-[11px] font-bold tracking-[1.5px] uppercase text-[#808080]">{{ t.section3Kicker }}</span>
                  <h3 class="text-[22px] font-bold text-[#212121] leading-tight">{{ t.section3 }}</h3>
                </div>
              </div>
              <span class="inline-flex items-center gap-[6px] px-[10px] py-[4px] rounded-full bg-[#f5f5f3] text-[10px] font-bold uppercase tracking-[1.2px] text-[#808080] shrink-0">
                {{ currentStep }} / {{ totalSteps }}
              </span>
            </header>

            <div class="flex flex-col gap-[18px]">
              <div class="flex flex-col gap-[8px]">
                <label class="text-[12px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.projectName }} <span class="text-[#1aa64a]">*</span></label>
                <input v-model="form.projectName" type="text" required :placeholder="t.projectNamePlaceholder" class="apply-input" />
              </div>
              <div class="flex flex-col gap-[8px]">
                <label class="text-[12px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.oneLiner }} <span class="text-[#1aa64a]">*</span></label>
                <textarea v-model="form.oneLiner" rows="2" required :placeholder="t.oneLinerPlaceholder" class="apply-input apply-textarea" />
              </div>
              <div class="flex flex-col gap-[10px]">
                <label class="text-[12px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.direction }} <span class="text-[#1aa64a]">*</span></label>
                <div class="apply-option-grid grid grid-cols-3 gap-[10px]">
                  <button
                    v-for="option in t.directionOptions"
                    :key="option"
                    type="button"
                    @click="form.direction = option"
                    :class="['apply-chip', form.direction === option ? 'is-selected' : '']"
                  >{{ option }}</button>
                </div>
              </div>
            </div>
          </article>

          <!-- ===== Step 4: Current Progress ===== -->
          <article v-show="currentStep === 4" class="apply-form-card task-card rise-card rise-card-2 bg-white flex flex-col gap-[24px] p-[28px] rounded-[12px]">
            <header class="flex items-start justify-between gap-[24px]">
              <div class="flex items-center gap-[14px]">
                <span class="inline-flex w-[44px] h-[44px] items-center justify-center rounded-[10px] bg-[#9bff00]">
                  <span class="material-symbols-outlined text-[#212121] text-[22px]" style="font-variation-settings: 'FILL' 1;">hub</span>
                </span>
                <div class="flex flex-col gap-[2px]">
                  <span class="text-[11px] font-bold tracking-[1.5px] uppercase text-[#808080]">{{ t.section4Kicker }}</span>
                  <h3 class="text-[22px] font-bold text-[#212121] leading-tight">{{ t.section4 }}</h3>
                </div>
              </div>
              <span class="inline-flex items-center gap-[6px] px-[10px] py-[4px] rounded-full bg-[#f5f5f3] text-[10px] font-bold uppercase tracking-[1.2px] text-[#808080] shrink-0">
                {{ currentStep }} / {{ totalSteps }}
              </span>
            </header>

            <div class="flex flex-col gap-[20px]">
              <div class="flex flex-col gap-[10px]">
                <label class="text-[12px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.stage }} <span class="text-[#1aa64a]">*</span></label>
                <div class="apply-stage-grid grid grid-cols-2 gap-[10px]">
                  <button
                    v-for="option in t.stageOptions"
                    :key="option"
                    type="button"
                    @click="form.stage = option"
                    :class="['apply-row-pill', form.stage === option ? 'is-selected' : '']"
                  >
                    <span class="apply-row-pill-dot"></span>
                    <span>{{ option }}</span>
                  </button>
                </div>
              </div>
              <div class="flex flex-col gap-[10px]">
                <label class="text-[12px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.assets }}</label>
                <div class="apply-option-grid grid grid-cols-3 gap-[10px]">
                  <button
                    v-for="option in t.assetsOptions"
                    :key="option"
                    type="button"
                    @click="toggleMultiSelect(form.assets, option)"
                    :class="['apply-chip', form.assets.includes(option) ? 'is-selected' : '']"
                  >{{ option }}</button>
                </div>
              </div>
            </div>
          </article>

          <!-- ===== Step 5: Application Purpose ===== -->
          <article v-show="currentStep === 5" class="apply-form-card task-card rise-card rise-card-2 bg-white flex flex-col gap-[24px] p-[28px] rounded-[12px]">
            <header class="flex items-start justify-between gap-[24px]">
              <div class="flex items-center gap-[14px]">
                <span class="inline-flex w-[44px] h-[44px] items-center justify-center rounded-[10px] bg-[#9bff00]">
                  <span class="material-symbols-outlined text-[#212121] text-[22px]" style="font-variation-settings: 'FILL' 1;">groups</span>
                </span>
                <div class="flex flex-col gap-[2px]">
                  <span class="text-[11px] font-bold tracking-[1.5px] uppercase text-[#808080]">{{ t.section5Kicker }}</span>
                  <h3 class="text-[22px] font-bold text-[#212121] leading-tight">{{ t.section5 }}</h3>
                </div>
              </div>
              <span class="inline-flex items-center gap-[6px] px-[10px] py-[4px] rounded-full bg-[#f5f5f3] text-[10px] font-bold uppercase tracking-[1.2px] text-[#808080] shrink-0">
                {{ currentStep }} / {{ totalSteps }}
              </span>
            </header>

            <div class="flex flex-col gap-[18px]">
              <div class="flex flex-col gap-[8px]">
                <label class="text-[12px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.reason }} <span class="text-[#1aa64a]">*</span></label>
                <textarea v-model="form.reason" rows="4" required :placeholder="t.reasonPlaceholder" class="apply-input apply-textarea" />
              </div>
              <div class="flex flex-col gap-[10px]">
                <label class="text-[12px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.support }} <span class="text-[#1aa64a]">*</span></label>
                <div class="apply-option-grid grid grid-cols-3 gap-[10px]">
                  <button
                    v-for="option in t.supportOptions"
                    :key="option"
                    type="button"
                    @click="toggleMultiSelect(form.support, option)"
                    :class="['apply-chip', form.support.includes(option) ? 'is-selected' : '']"
                  >{{ option }}</button>
                </div>
              </div>
            </div>
          </article>

          <!-- ===== Step 6: Supplementary Material ===== -->
          <article v-show="currentStep === 6" class="apply-form-card task-card rise-card rise-card-2 bg-white flex flex-col gap-[24px] p-[28px] rounded-[12px]">
            <header class="flex items-start justify-between gap-[24px]">
              <div class="flex items-center gap-[14px]">
                <span class="inline-flex w-[44px] h-[44px] items-center justify-center rounded-[10px] bg-[#9bff00]">
                  <span class="material-symbols-outlined text-[#212121] text-[22px]" style="font-variation-settings: 'FILL' 1;">share</span>
                </span>
                <div class="flex flex-col gap-[2px]">
                  <span class="text-[11px] font-bold tracking-[1.5px] uppercase text-[#808080]">{{ t.section6Kicker }}</span>
                  <h3 class="text-[22px] font-bold text-[#212121] leading-tight">{{ t.section6 }}</h3>
                </div>
              </div>
              <span class="inline-flex items-center gap-[6px] px-[10px] py-[4px] rounded-full bg-[#f5f5f3] text-[10px] font-bold uppercase tracking-[1.2px] text-[#808080] shrink-0">
                {{ currentStep }} / {{ totalSteps }}
              </span>
            </header>

            <div class="flex flex-col gap-[18px]">
              <div class="flex flex-col gap-[8px]">
                <label class="text-[12px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.pdfLink }}</label>
                <input v-model="form.pdfLink" type="url" :placeholder="t.pdfLinkPlaceholder" class="apply-input" />
              </div>
              <div class="flex flex-col gap-[8px]">
                <label class="text-[12px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.otherInfo }}</label>
                <textarea v-model="form.otherInfo" rows="4" :placeholder="t.otherInfoPlaceholder" class="apply-input apply-textarea" />
              </div>
            </div>
          </article>

          <!-- Submit Error -->
          <div v-if="submitError" class="rise-card rise-card-3 bg-white p-[18px] rounded-[12px] border-l-[4px] border-[#ff4d4f] flex items-center gap-[10px]">
            <span class="material-symbols-outlined text-[#ff4d4f] text-[20px]">close</span>
            <p class="text-[14px] font-bold text-[#ff4d4f]">{{ submitError }}</p>
          </div>

          <!-- Navigation buttons -->
          <div class="apply-nav-actions rise-card rise-card-3 flex items-center justify-between gap-[12px] pt-[8px]">
            <button
              v-if="currentStep > 1"
              @click="prevStep"
              type="button"
              class="white-pill-btn"
            >
              <span class="text-[15px] font-bold text-[#212121]">{{ t.prev }}</span>
            </button>
            <div v-else></div>

            <button
              v-if="currentStep < totalSteps"
              @click="nextStep"
              :disabled="!canProceed"
              type="button"
              class="white-pill-btn"
              :class="{ 'apply-disabled': !canProceed }"
            >
              <span class="text-[15px] font-bold text-[#212121]">{{ t.next }}</span>
              <span class="arrow-icon arrow-diag">
                <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
              </span>
            </button>

            <button
              v-else
              @click="handleSubmit"
              :disabled="isSubmitting"
              type="button"
              class="apply-submit-btn"
              :class="{ 'apply-disabled': isSubmitting }"
            >
              <span v-if="isSubmitting" class="material-symbols-outlined animate-spin text-[18px]">progress_activity</span>
              <span class="text-[15px] font-bold">{{ isSubmitting ? t.submitting : t.submit }}</span>
              <span class="arrow-icon arrow-diag">
                <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
              </span>
            </button>
          </div>
        </section>

        <HomeFooter :language="props.language" />

      </main>
    </div>
  </div>
</template>

<style scoped>
.material-symbols-outlined { font-variation-settings: 'FILL' 0, 'wght' 400, 'GRAD' 0, 'opsz' 24; }

/* === Task card hover (same as TaskHall) === */
.task-card {
  transition: scale 0.28s cubic-bezier(0, 0, 0.2, 1),
              box-shadow 0.28s cubic-bezier(0, 0, 0.2, 1);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.04);
}
.task-card:hover {
  scale: 1.005;
  box-shadow: 0 14px 36px -4px rgba(0, 0, 0, 0.12);
}

/* === Progress bar (same as TaskHall) === */
.progress-track {
  width: 100%;
  height: 6px;
  background: #f0efec;
  border-radius: 999px;
  overflow: hidden;
}
.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, #9bff00 0%, #1aa64a 100%);
  border-radius: 999px;
  transition: width 0.6s cubic-bezier(0.22, 1, 0.36, 1);
  box-shadow: 0 0 12px rgba(155, 255, 0, 0.5);
}

/* === Step pill (numbered jump nav) === */
.apply-step-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 700;
  background: #f0efec;
  color: #808080;
  border: 1px solid transparent;
  transition: all 0.18s ease;
}
.apply-step-pill.is-active {
  background: #212121;
  color: #9bff00;
  border-color: #9bff00;
  box-shadow: 0 0 0 3px rgba(155, 255, 0, 0.2);
}
.apply-step-pill.is-done {
  background: #9bff00;
  color: #212121;
}
.apply-step-pill.is-done:hover {
  background: #5fd189;
  cursor: pointer;
}
.apply-step-pill.is-future {
  cursor: not-allowed;
}

/* === Inputs / textarea === */
.apply-input {
  width: 100%;
  background: #fafaf8;
  border: 1px solid #eee;
  border-radius: 12px;
  padding: 12px 14px;
  font-size: 14px;
  font-weight: 500;
  color: #212121;
  transition: border-color 0.18s ease, background 0.18s ease, box-shadow 0.18s ease;
}
.apply-input::placeholder {
  color: #b0b0b0;
  font-weight: 400;
}
.apply-input:focus {
  outline: none;
  border-color: #9bff00;
  background: #ffffff;
  box-shadow: 0 0 0 3px rgba(155, 255, 0, 0.18);
}
.apply-input.is-error {
  border-color: rgba(255, 77, 79, 0.5);
}
.apply-textarea {
  resize: vertical;
  min-height: 84px;
  line-height: 1.5;
}

/* === Chip selector (multi/single select for grid options) === */
.apply-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 12px 14px;
  border-radius: 14px;
  background: #fafaf8;
  border: 1px solid #eee;
  color: #212121;
  font-size: 13px;
  font-weight: 600;
  line-height: 1.3;
  text-align: center;
  transition: all 0.18s ease;
}
.apply-chip:hover:not(.is-selected) {
  border-color: rgba(155, 255, 0, 0.5);
  background: #ffffff;
  transform: translateY(-1px);
}
.apply-chip.is-selected {
  background: #9bff00;
  border-color: #1aa64a;
  color: #212121;
  box-shadow: 0 4px 14px rgba(155, 255, 0, 0.3);
}

/* === Row pill (radio-style single select) === */
.apply-row-pill {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  border-radius: 12px;
  background: #fafaf8;
  border: 1px solid #eee;
  color: #212121;
  font-size: 13px;
  font-weight: 600;
  text-align: left;
  transition: all 0.18s ease;
}
.apply-row-pill:hover:not(.is-selected) {
  border-color: rgba(155, 255, 0, 0.5);
  background: #ffffff;
  transform: translateX(2px);
}
.apply-row-pill-dot {
  width: 14px;
  height: 14px;
  border-radius: 999px;
  border: 2px solid #ddd;
  background: #ffffff;
  flex-shrink: 0;
  transition: all 0.18s ease;
}
.apply-row-pill.is-selected {
  background: #9bff00;
  border-color: #1aa64a;
  color: #212121;
  box-shadow: 0 4px 14px rgba(155, 255, 0, 0.3);
}
.apply-row-pill.is-selected .apply-row-pill-dot {
  border-color: #212121;
  background: #212121;
  box-shadow: inset 0 0 0 3px #9bff00;
}

/* === Disabled button === */
.apply-disabled {
  opacity: 0.4;
  cursor: not-allowed !important;
  pointer-events: none;
}

/* === Submit button — dark filled CTA with green hover === */
.apply-submit-btn {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 12px 20px;
  border-radius: 999px;
  background: #212121;
  color: #9bff00;
  border: 1px solid #212121;
  transition: background 0.18s ease, color 0.18s ease, transform 0.18s ease, box-shadow 0.18s ease;
}
.apply-submit-btn .arrow-icon { background: #9bff00; }
.apply-submit-btn .arrow-img { background-color: #212121; }
.apply-submit-btn:hover:not(.apply-disabled) {
  background: #9bff00;
  color: #212121;
  transform: translateY(-1px);
  box-shadow: 0 12px 28px -4px rgba(155, 255, 0, 0.45);
}
.apply-submit-btn:hover:not(.apply-disabled) .arrow-icon { background: #212121; }
.apply-submit-btn:hover:not(.apply-disabled) .arrow-img { background-color: #9bff00; }
.apply-submit-btn:hover:not(.apply-disabled) .arrow-diag .arrow-img { transform: translate(30px, -30px); }

@media (max-width: 768px) {
  .project-apply-host.is-mobile {
    height: auto !important;
    min-height: 100vh;
    max-width: 100vw;
    overflow-x: hidden;
  }

  .project-apply-wrapper {
    width: 100% !important;
    height: auto !important;
  }

  .project-apply-page {
    width: 100% !important;
    min-height: 100vh;
    overflow-x: hidden;
    transform: none !important;
    transform-origin: top left;
  }

  .apply-nav-spacer {
    height: 72px !important;
  }

  .project-apply-page > section {
    padding-right: 20px !important;
    padding-left: 20px !important;
  }

  .apply-hero {
    gap: 20px !important;
    padding-top: 24px !important;
    padding-bottom: 30px !important;
  }

  .apply-hero-title {
    width: 100% !important;
    font-size: clamp(40px, 13vw, 56px) !important;
    line-height: 1.08 !important;
    letter-spacing: 0 !important;
  }

  .apply-hero-title .font-playfair {
    display: inline;
    font-size: 0.92em;
    line-height: 1.08;
  }

  .apply-hero-actions {
    align-items: flex-start !important;
    flex-direction: column;
    gap: 18px !important;
  }

  .apply-hero-actions .white-pill-btn,
  .apply-nav-actions .white-pill-btn,
  .apply-nav-actions .apply-submit-btn {
    flex-shrink: 0;
    white-space: nowrap;
  }

  .apply-hero-sub {
    width: 100% !important;
    max-width: 100%;
    font-size: 16px !important;
    line-height: 1.55 !important;
  }

  .apply-form-section,
  .apply-success-section {
    gap: 22px !important;
    padding-top: 0 !important;
    padding-bottom: 64px !important;
  }

  .apply-progress-card,
  .apply-form-card,
  .apply-success-card {
    width: 100% !important;
    max-width: 520px;
    margin-right: auto;
    margin-left: auto;
    border-radius: 20px !important;
  }

  .apply-progress-card {
    padding: 18px !important;
  }

  .apply-progress-head {
    align-items: flex-start !important;
    gap: 10px;
  }

  .apply-progress-title {
    min-width: 0;
    flex-wrap: wrap;
    row-gap: 4px;
  }

  .apply-progress-title > span:last-child {
    width: 100%;
    font-size: 13px !important;
    line-height: 1.35;
  }

  .apply-step-pill {
    width: 34px;
    height: 34px;
    font-size: 13px;
  }

  .apply-form-card {
    gap: 22px !important;
    padding: 22px !important;
  }

  .apply-form-card header {
    align-items: flex-start !important;
    gap: 14px !important;
  }

  .apply-form-card h3 {
    font-size: 24px !important;
    line-height: 1.18 !important;
    letter-spacing: 0 !important;
  }

  .apply-two-grid,
  .apply-contact-grid,
  .apply-option-grid,
  .apply-stage-grid {
    grid-template-columns: minmax(0, 1fr) !important;
  }

  .apply-two-grid > .col-span-2 {
    grid-column: span 1 / span 1 !important;
  }

  .apply-input {
    min-height: 54px;
    border-radius: 16px;
    padding: 14px 16px;
    font-size: 16px;
  }

  .apply-textarea {
    min-height: 116px;
  }

  .apply-chip,
  .apply-row-pill {
    justify-content: flex-start;
    min-height: 48px;
    padding: 13px 15px;
    font-size: 14px;
    line-height: 1.35;
    text-align: left;
  }

  .apply-chip {
    justify-content: center;
    text-align: center;
  }

  .apply-nav-actions {
    position: sticky;
    bottom: 12px;
    z-index: 20;
    width: min(100%, 520px);
    margin-right: auto;
    margin-left: auto;
    padding: 12px;
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 22px;
    background: rgba(18, 18, 18, 0.72);
    backdrop-filter: blur(14px);
  }

  .apply-nav-actions .white-pill-btn,
  .apply-nav-actions .apply-submit-btn {
    min-height: 52px;
    padding: 12px 16px;
  }

  .apply-success-card {
    padding: 34px 22px !important;
  }

  .apply-success-card h2 {
    font-size: 34px !important;
    line-height: 1.12 !important;
  }

}

/* === Success card pop-in === */
.success-pop {
  animation: success-pop-anim 0.6s cubic-bezier(0.34, 1.56, 0.64, 1);
}
@keyframes success-pop-anim {
  0%   { transform: scale(0.3); opacity: 0; }
  50%  { transform: scale(1.1); }
  100% { transform: scale(1); opacity: 1; }
}

/* ============================================================
   DARK MODE — extends jc-shell.css home-dark cascade
   ============================================================ */
.jc-canvas-host.home-dark .apply-input {
  background: #1f1f1f !important;
  border-color: rgba(255, 255, 255, 0.08) !important;
  color: #f0f0f0 !important;
}
.jc-canvas-host.home-dark .apply-input::placeholder { color: #666 !important; }
.jc-canvas-host.home-dark .apply-input:focus {
  background: #2a2a2a !important;
  border-color: #9bff00 !important;
  box-shadow: 0 0 0 3px rgba(155, 255, 0, 0.25) !important;
}
.jc-canvas-host.home-dark .apply-input.is-error {
  border-color: rgba(255, 77, 79, 0.55) !important;
}

.jc-canvas-host.home-dark .apply-chip {
  background: #1f1f1f !important;
  border-color: rgba(255, 255, 255, 0.08) !important;
  color: #f0f0f0 !important;
}
.jc-canvas-host.home-dark .apply-chip:hover:not(.is-selected) {
  background: #262626 !important;
  border-color: rgba(155, 255, 0, 0.35) !important;
}
.jc-canvas-host.home-dark .apply-chip.is-selected {
  background: #9bff00 !important;
  border-color: #1aa64a !important;
  color: #212121 !important;
}

.jc-canvas-host.home-dark .apply-row-pill {
  background: #1f1f1f !important;
  border-color: rgba(255, 255, 255, 0.08) !important;
  color: #f0f0f0 !important;
}
.jc-canvas-host.home-dark .apply-row-pill:hover:not(.is-selected) {
  background: #262626 !important;
  border-color: rgba(155, 255, 0, 0.35) !important;
}
.jc-canvas-host.home-dark .apply-row-pill.is-selected {
  background: #9bff00 !important;
  border-color: #1aa64a !important;
  color: #212121 !important;
}
.jc-canvas-host.home-dark .apply-row-pill-dot {
  background: #2a2a2a;
  border-color: #444;
}
.jc-canvas-host.home-dark .apply-row-pill.is-selected .apply-row-pill-dot {
  background: #212121;
  border-color: #212121;
  box-shadow: inset 0 0 0 3px #9bff00;
}

.jc-canvas-host.home-dark .progress-track { background: #2a2a2a !important; }

.jc-canvas-host.home-dark .apply-step-pill {
  background: #2a2a2a !important;
  color: #999 !important;
}
.jc-canvas-host.home-dark .apply-step-pill.is-active {
  background: #9bff00 !important;
  color: #212121 !important;
  border-color: #ffffff !important;
}
.jc-canvas-host.home-dark .apply-step-pill.is-done {
  background: #1aa64a !important;
  color: #ffffff !important;
}

/* Submit btn in dark: dark→green hover flips to white→black */
.jc-canvas-host.home-dark .apply-submit-btn {
  background: #ffffff !important;
  color: #212121 !important;
  border-color: #ffffff !important;
}
.jc-canvas-host.home-dark .apply-submit-btn .arrow-icon { background: #212121 !important; }
.jc-canvas-host.home-dark .apply-submit-btn .arrow-img { background-color: #9bff00 !important; }
.jc-canvas-host.home-dark .apply-submit-btn:hover:not(.apply-disabled) {
  background: #9bff00 !important;
  color: #212121 !important;
}
.jc-canvas-host.home-dark .apply-submit-btn:hover:not(.apply-disabled) .arrow-icon { background: #212121 !important; }
.jc-canvas-host.home-dark .apply-submit-btn:hover:not(.apply-disabled) .arrow-img { background-color: #9bff00 !important; }

/* The bg-[#f5f5f3] kicker chip "X / N" auto-handled by jc-shell.css home-dark cascade */

/* Soft greys for label text + kicker uppercase already covered by global text rules */
</style>
