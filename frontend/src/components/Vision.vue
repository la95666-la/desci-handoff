<script setup>
import { computed, ref, onMounted } from 'vue'

const props = defineProps({
  language: {
    type: String,
    default: 'en'
  }
})

const content = {
  en: {
    mainTitle: 'We believe in code',
    mainSubtitle: 'that saves lives.',
    cards: [
      {
        label: 'Problem',
        title: 'Obsolete System',
        subtitle: 'Slow. Closed. Controlled.',
        colorClass: 'border-red-300 dark:border-red-900 hover:border-red-500 dark:hover:border-red-700',
        accentClass: 'bg-red-600 dark:bg-red-500',
        bgClass: 'bg-red-50 dark:bg-red-950/20'
      },
      {
        label: 'Response',
        title: 'We Refuse to Wait',
        subtitle: 'Permissionless revolution.',
        colorClass: 'border-orange-300 dark:border-orange-900 hover:border-orange-500 dark:hover:border-orange-700',
        accentClass: 'bg-orange-600 dark:bg-orange-500',
        bgClass: 'bg-orange-50 dark:bg-orange-950/20'
      },
      {
        label: 'Solution',
        title: 'Open Science',
        subtitle: 'Labs → Open Source. Milestones → NFTs.',
        colorClass: 'border-blue-300 dark:border-blue-900 hover:border-blue-500 dark:hover:border-blue-700',
        accentClass: 'bg-blue-600 dark:bg-blue-500',
        bgClass: 'bg-blue-50 dark:bg-blue-950/20'
      }
    ]
  },
  zh: {
    mainTitle: '我們信仰代碼',
    mainSubtitle: '更信仰拯救生命的代碼。',
    cards: [
      {
        label: '問題',
        title: '陳舊體系',
        subtitle: '緩慢。封閉。受控。',
        colorClass: 'border-red-300 dark:border-red-900 hover:border-red-500 dark:hover:border-red-700',
        accentClass: 'bg-red-600 dark:bg-red-500',
        bgClass: 'bg-red-50 dark:bg-red-950/20'
      },
      {
        label: '態度',
        title: '我們拒絕等待',
        subtitle: '無需許可的革命。',
        colorClass: 'border-orange-300 dark:border-orange-900 hover:border-orange-500 dark:hover:border-orange-700',
        accentClass: 'bg-orange-600 dark:bg-orange-500',
        bgClass: 'bg-orange-50 dark:bg-orange-950/20'
      },
      {
        label: '方案',
        title: '開放科學',
        subtitle: '實驗室 → 開源。里程碑 → NFT。',
        colorClass: 'border-blue-300 dark:border-blue-900 hover:border-blue-500 dark:hover:border-blue-700',
        accentClass: 'bg-blue-600 dark:bg-blue-500',
        bgClass: 'bg-blue-50 dark:bg-blue-950/20'
      }
    ]
  }
}

const isVisible = ref(false)
const t = computed(() => content[props.language] || content.en)
const primaryCard = computed(() => t.value.cards?.[0] || {})
const secondaryCards = computed(() => t.value.cards?.slice(1) || [])

onMounted(() => {
  setTimeout(() => {
    isVisible.value = true
  }, 200)
})
</script>

<template>
  <section id="vision-section" class="bg-white dark:bg-gray-950">
    <div class="mx-auto max-w-[1200px] border-x border-t border-gray-200 px-6 py-24 sm:px-10 lg:px-12 lg:py-32 dark:border-gray-800">
      <div
        class="transition-all duration-1000"
        :class="isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-8'"
      >
        <div class="mb-12">
          <p class="text-sm font-medium uppercase text-gray-500 dark:text-gray-400">
            {{ t.mainSubtitle }}
          </p>
          <h2 class="mt-4 max-w-3xl text-5xl font-medium tracking-normal text-gray-950 sm:text-6xl dark:text-white">
            {{ t.mainTitle }}
          </h2>
        </div>

        <div class="grid gap-4 lg:grid-cols-[minmax(0,1.8fr)_minmax(0,0.55fr)_minmax(0,0.55fr)] xl:grid-cols-[676px_198px_198px]">
          <div class="problem-card-gradient relative min-h-[356px] overflow-hidden rounded-2xl p-10">
            <img
              src="/figma/molecule-card.png"
              alt=""
              class="pointer-events-none absolute right-[-48px] top-0 h-[356px] w-[356px] object-cover mix-blend-luminosity opacity-75 sm:right-0 sm:opacity-100"
            />
            <div class="relative z-10 flex min-h-[276px] flex-col justify-between">
              <p class="text-base leading-5 text-gray-800">
                {{ primaryCard.label }}
              </p>

              <div>
                <img
                  src="/figma/vision-molecule-icon.svg"
                  alt=""
                  class="mb-[34px] h-[39px] w-[40px]"
                />
                <h3 class="max-w-[340px] text-[40px] font-medium leading-none tracking-normal text-gray-950 sm:text-[48px] lg:text-[40px]">
                  {{ primaryCard.title }}
                </h3>
                <span class="mt-8 inline-flex min-w-[177px] justify-center rounded-full bg-white/75 px-4 py-2 text-xs font-medium leading-4 text-gray-950">
                  {{ primaryCard.subtitle }}
                </span>
              </div>
            </div>
          </div>

          <div
            v-for="(card, index) in secondaryCards"
            :key="card.title"
            class="relative min-h-[356px] overflow-hidden rounded-2xl bg-[#f9fbf8] p-10 dark:bg-gray-900"
          >
            <p class="text-base leading-5 text-gray-500">{{ card.label }}</p>

            <img
              v-if="index === 0"
              src="/figma/vision-lightning.svg"
              alt=""
              class="absolute left-1/2 top-[142px] h-[59px] w-9 -translate-x-1/2"
            />
            <img
              v-else
              src="/figma/vision-flower.svg"
              alt=""
              class="absolute left-1/2 top-[145px] h-[52px] w-[52px] -translate-x-1/2"
            />

            <div class="absolute bottom-10 left-10 right-8">
              <h3 class="text-[32px] font-medium leading-[1.08] tracking-normal text-gray-950 dark:text-white">
                {{ card.title }}
              </h3>
              <p class="mt-5 text-base leading-6 text-gray-500 dark:text-gray-400">
                {{ card.subtitle }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.problem-card-gradient {
  background: linear-gradient(207.772deg, rgb(155, 255, 0) 0%, rgb(198, 254, 150) 50%, rgb(144, 254, 215) 100%);
}
</style>
