<script setup>
import { ref, onMounted } from 'vue'
import { X, ArrowRight } from 'lucide-vue-next'

defineProps({
  language: String
})

const isVisible = ref(false)

onMounted(() => {
  setTimeout(() => {
    isVisible.value = true
  }, 300)
})

const content = {
  en: {
    title: 'Because true innovation',
    titleHighlight: 'never needs permission.',
    description: 'Traditional healthcare research is broken. JoinCare rebuilds it from first principles—open, transparent, and accessible to everyone.',
    comparison: [
      {
        old: '18-24 Month Cycles',
        new: 'Instant Deployment'
      },
      {
        old: 'Few Partners Decide',
        new: 'Community Governs'
      },
      {
        old: 'Research Black Box',
        new: 'Fully Transparent'
      },
      {
        old: '$1M+ Barrier',
        new: 'From $1'
      },
      {
        old: 'Centralized Capture',
        new: 'Shared RWA'
      }
    ]
  },
  zh: {
    title: '因為真正的創新',
    titleHighlight: '從不需要許可。',
    description: '傳統醫療研究已經失效。JoinCare 從第一性原理重建——開放、透明、人人可及。',
    comparison: [
      {
        old: '18-24個月周期',
        new: '即時部署'
      },
      {
        old: '少數人決策',
        new: '社區治理'
      },
      {
        old: '研究黑箱',
        new: '全程透明'
      },
      {
        old: '百萬門檻',
        new: '1美元起'
      },
      {
        old: '中心化捕獲',
        new: '共享RWA'
      }
    ]
  }
}
</script>

<template>
  <section class="relative min-h-screen flex items-center overflow-hidden bg-white dark:bg-black">
    <div class="relative z-10 w-full max-w-7xl mx-auto px-6 py-32">
      <!-- Title Section -->
      <div
        class="text-center mb-20 transition-all duration-1000"
        :class="isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-8'"
      >
        <div class="space-y-6">
          <h2 class="text-5xl sm:text-6xl md:text-7xl lg:text-8xl font-light text-gray-900 dark:text-white tracking-tight leading-[1.1]">
            {{ content[language].title }}
          </h2>
          <p class="text-2xl sm:text-3xl md:text-4xl font-medium text-gray-600 dark:text-gray-400 leading-tight">
            {{ content[language].titleHighlight }}
          </p>
          <div class="pt-4">
            <div class="inline-block w-16 h-px bg-gray-300 dark:bg-gray-700"></div>
          </div>
          <p class="text-lg text-gray-600 dark:text-gray-400 max-w-3xl mx-auto pt-6">
            {{ content[language].description }}
          </p>
        </div>
      </div>

      <!-- Comparison List -->
      <div class="max-w-6xl mx-auto space-y-6">
        <div
          v-for="(item, index) in content[language].comparison"
          :key="index"
          class="transition-all duration-700"
          :class="isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-8'"
          :style="{ transitionDelay: `${index * 100 + 200}ms` }"
        >
          <div class="grid grid-cols-1 md:grid-cols-[1fr_auto_1fr] gap-0 rounded-xl overflow-hidden border border-gray-200 dark:border-gray-800 hover:shadow-xl transition-all group">
            <!-- Old (Left) - Blue accent -->
            <div class="relative p-6 bg-white dark:bg-black border-l-4 border-blue-500">
              <div class="flex items-center gap-3">
                <X :size="20" class="text-blue-500 flex-shrink-0" />
                <p class="text-base sm:text-lg text-gray-500 dark:text-gray-500 line-through">
                  {{ item.old }}
                </p>
              </div>
            </div>

            <!-- VS (Center) -->
            <div class="hidden md:flex items-center justify-center px-6 bg-gray-50 dark:bg-gray-900/50">
              <div class="flex flex-col items-center gap-2">
                <span class="text-xs font-bold text-gray-400 dark:text-gray-600 uppercase tracking-wider">VS</span>
                <ArrowRight :size="20" class="text-gray-400 dark:text-gray-600 group-hover:translate-x-1 transition-transform" />
              </div>
            </div>

            <!-- Mobile VS -->
            <div class="flex md:hidden items-center justify-center py-3 bg-gray-50 dark:bg-gray-900/50">
              <div class="flex items-center gap-2">
                <span class="text-xs font-bold text-gray-400 dark:text-gray-600 uppercase tracking-wider">VS</span>
                <ArrowRight :size="16" class="text-gray-400 dark:text-gray-600" />
              </div>
            </div>

            <!-- New (Right) - Green accent -->
            <div class="relative p-6 bg-white dark:bg-black border-l-4 border-emerald-500">
              <div class="flex items-center gap-3">
                <div class="w-5 h-5 rounded-full bg-emerald-500 flex items-center justify-center flex-shrink-0">
                  <ArrowRight :size="14" class="text-white" />
                </div>
                <p class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white">
                  {{ item.new }}
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>


