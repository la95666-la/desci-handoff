<script setup>
import { computed, ref, onMounted, onUnmounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowRight } from 'lucide-vue-next'

const props = defineProps({
  language: String
})

const router = useRouter()

const content = {
  en: {
    title: 'Empowering Bio-Medical Innovation with Web3',
    subtitle: 'Frontier Medicine, Built by the World. Value, Shared by All.',
    descriptionLines: [
      'A biomedical innovation protocol owned and driven by a global community.'
    ],
    cta: 'View Candidate Project',
    explore: 'Explore',
    features: [
      { title: 'DeSci', desc: 'Decentralized Science', detail: 'Open research, global collaboration' }
    ]
  },
  zh: {
    title: '用Web3賦能生物醫學創新',
    subtitle: '前沿醫學，全球共建。價值，共享。',
    descriptionLines: [
      '我們正在構建一個由全球社區擁有和驅動的生物醫藥創新協議。'
    ],
    cta: '查看候選項目',
    explore: '探索',
    features: [
      { title: 'DeSci', desc: '去中心化科學', detail: '開放研究，全球協作' }
    ]
  }
}

const activeFeature = ref(0)
const currentDescriptionIndex = ref(0)
let rotateInterval = null
let descriptionInterval = null

const t = computed(() => content[props.language] || content.en)

const activeFeatureIndex = computed(() => {
  const featureCount = t.value.features?.length || 1
  return activeFeature.value % featureCount
})

const activeFeatureTitle = computed(() => {
  return t.value.features?.[activeFeatureIndex.value]?.title || ''
})

const featureItems = computed(() => t.value.features || [])

const currentDescription = computed(() => {
  const lines = t.value.descriptionLines || []
  return lines[currentDescriptionIndex.value % (lines.length || 1)] || ''
})

watch(() => props.language, () => {
  activeFeature.value = activeFeatureIndex.value
  const descriptionCount = t.value.descriptionLines?.length || 1
  currentDescriptionIndex.value = currentDescriptionIndex.value % descriptionCount
})

onMounted(() => {
  rotateInterval = setInterval(() => {
    const featureCount = t.value.features?.length || 1
    activeFeature.value = (activeFeature.value + 1) % featureCount
  }, 4000)

  descriptionInterval = setInterval(() => {
    const descriptionCount = t.value.descriptionLines?.length || 1
    currentDescriptionIndex.value = (currentDescriptionIndex.value + 1) % descriptionCount
  }, 4000)
})

onUnmounted(() => {
  if (rotateInterval) clearInterval(rotateInterval)
  if (descriptionInterval) clearInterval(descriptionInterval)
})

const scrollToFunding = () => {
  document.getElementById('funding-section')?.scrollIntoView({ behavior: 'smooth' })
}
</script>

<template>
  <section class="bg-white pt-[88px] dark:bg-gray-950">
    <div class="mx-auto max-w-[1440px] px-5 sm:px-8 lg:px-9">
      <div
        class="splash-hero-stage relative min-h-[720px] overflow-hidden rounded-[18px] bg-gray-900 shadow-sm sm:min-h-[560px] lg:h-[710px]"
      >
        <video
          class="splash-hero-video absolute inset-0 h-full w-full object-cover"
          autoplay
          muted
          loop
          playsinline
          preload="metadata"
          aria-hidden="true"
        >
          <source src="/figma/hero.mp4" type="video/mp4" />
        </video>
        <div class="absolute inset-0 bg-black/10"></div>

        <div class="relative z-10 flex h-full min-h-[500px] flex-col items-center justify-end px-5 pb-0 pt-24 text-center sm:min-h-[560px] sm:justify-center sm:pb-36 sm:pt-16 lg:min-h-0">
          <h1 class="max-w-5xl break-words text-[36px] font-medium leading-[1.08] tracking-normal text-white sm:text-6xl lg:text-[72px]">
            {{ t.title }}
          </h1>
          <p class="mt-6 max-w-3xl text-base font-normal leading-relaxed text-white/90 sm:text-xl">
            {{ t.subtitle }}
          </p>

          <div class="mt-8 flex flex-wrap items-center justify-center gap-3">
            <button
              @click="router.push('/launchpad')"
              class="group inline-flex h-12 items-center gap-3 rounded-full bg-white px-6 text-sm font-medium text-gray-950 shadow-sm transition hover:bg-gray-100"
            >
              <span>{{ t.cta }}</span>
              <ArrowRight :size="16" class="transition-transform group-hover:translate-x-0.5" />
            </button>
            <button
              @click="scrollToFunding"
              class="inline-flex h-12 items-center rounded-full border border-white/35 bg-white/20 px-6 text-sm font-medium text-white backdrop-blur-md transition hover:bg-white/30"
            >
              {{ t.explore }}
            </button>
          </div>
        </div>

        <div class="relative z-20 mx-4 mb-6 mt-8 max-w-none rounded-2xl border border-white/70 bg-white/92 p-3 shadow-xl backdrop-blur-md sm:absolute sm:bottom-6 sm:left-1/2 sm:right-auto sm:mx-0 sm:mb-0 sm:mt-0 sm:w-[calc(100%-32px)] sm:max-w-[428px] sm:-translate-x-1/2 sm:p-4">
          <div class="grid grid-cols-[96px_minmax(0,1fr)] gap-3 sm:grid-cols-[128px_minmax(0,1fr)] sm:gap-4">
            <div class="h-28 overflow-hidden rounded-xl bg-gray-100 sm:h-32">
              <img src="/figma/hero-biomedical.png" alt="" class="h-full w-full object-cover" />
            </div>
            <div class="flex min-w-0 flex-col justify-center">
              <p class="text-lg font-semibold leading-tight text-gray-950">
                {{ activeFeatureTitle }}
              </p>
              <p class="mt-2 break-words text-sm leading-relaxed text-gray-600">
                {{ currentDescription }}
              </p>
              <div class="mt-5 flex items-center gap-2">
                <span
                  v-for="(_, index) in featureItems"
                  :key="index"
                  class="h-1 rounded-full transition-all"
                  :class="activeFeatureIndex === index ? 'w-9 bg-gray-950' : 'w-1.5 bg-gray-300'"
                ></span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.splash-hero-stage {
  opacity: 0;
  transform: translateY(12px) scale(.982);
  clip-path: inset(0 7% 0 7% round 18px);
  transition:
    transform 1.05s var(--splash-ease, cubic-bezier(.2, .85, .2, 1)) .22s,
    opacity .8s var(--splash-ease, cubic-bezier(.2, .85, .2, 1)) .18s,
    clip-path 1.05s var(--splash-ease, cubic-bezier(.2, .85, .2, 1)) .2s;
  will-change: opacity, transform, clip-path;
}

.splash-hero-video {
  transform: scale(1.035);
  transform-origin: center;
  transition: transform 1.4s var(--splash-ease, cubic-bezier(.2, .85, .2, 1)) .12s;
  will-change: transform;
}

:global(body.is-ready .splash-hero-stage) {
  opacity: 1;
  transform: none;
  clip-path: inset(0 0 0 0 round 18px);
}

:global(body.is-ready .splash-hero-video) {
  transform: none;
}

@media (prefers-reduced-motion: reduce) {
  .splash-hero-stage,
  .splash-hero-video {
    opacity: 1;
    transform: none;
    clip-path: inset(0 0 0 0 round 18px);
    transition: none;
  }
}
</style>
