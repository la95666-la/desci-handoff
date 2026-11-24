<script setup>
import { ref, onMounted } from 'vue'
import { Award, Globe, Building2, Sparkles } from 'lucide-vue-next'

defineProps({
  language: String
})

const content = {
  en: {
    title: 'Founder',
    name: 'Prof. Peter Sachsenmeier',
    positions: [
      {
        icon: Award,
        title: 'Academician of the German National Academy of Science and Engineering',
        org: 'acatech'
      },
      {
        icon: Globe,
        title: 'Member of the Asia-Pacific Artificial Intelligence Association',
        org: 'AAIA'
      },
      {
        icon: Building2,
        title: 'Previously held positions at several international organizations',
        orgs: ['World Bank', 'UNESCO', 'World Health Organization (WHO)', 'United Nations Environment Programme (UNEP)']
      },
      {
        icon: Sparkles,
        title: 'In 2025, founded the Joincare Foundation',
        org: ''
      }
    ],
    quote: '"The future of medicine belongs to those who dare to reimagine it — open, decentralized, and owned by humanity."'
  },
  zh: {
    title: '創始人',
    name: '皮特·薩赫遜教授',
    positions: [
      {
        icon: Award,
        title: '德國國家科學技術研究院院士',
        org: 'acatech'
      },
      {
        icon: Globe,
        title: '亞太人工智能協會會員',
        org: 'AAIA'
      },
      {
        icon: Building2,
        title: '曾先後在世界銀行、聯合國教科文組織、世界衛生組織和聯合國環境規劃署等機構任職',
        orgs: ['世界銀行', '聯合國教科文組織', '世界衛生組織', '聯合國環境規劃署']
      },
      {
        icon: Sparkles,
        title: '2025年，創辦 Joincare基金會',
        org: ''
      }
    ],
    quote: '"醫學的未來屬於那些敢於重新想象它的人——開放、去中心化、屬於全人類。"'
  }
}

const isVisible = ref(false)

onMounted(() => {
  const observer = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
      if (entry.isIntersecting) {
        isVisible.value = true
      }
    })
  }, { threshold: 0.1 })

  const element = document.querySelector('#founder-section')
  if (element) observer.observe(element)
})
</script>

<template>
  <section id="founder-section" class="relative py-32 overflow-hidden bg-white dark:bg-black">
    <!-- Subtle Background Pattern -->
    <div class="absolute inset-0 opacity-[0.02] dark:opacity-[0.05]">
      <div class="absolute inset-0" style="background-image: repeating-linear-gradient(0deg, transparent, transparent 2px, currentColor 2px, currentColor 4px); background-size: 100% 40px;"></div>
    </div>

    <div class="relative z-10 max-w-6xl mx-auto px-6">
      <!-- Header -->
      <div
        class="text-center mb-20 space-y-6 transition-all duration-1000"
        :class="isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-10'"
      >
        <h2 class="text-5xl sm:text-6xl md:text-7xl lg:text-8xl font-light text-gray-900 dark:text-white tracking-tight leading-[1.1]">
          {{ content[language].title }}
        </h2>
        <div class="pt-4">
          <div class="inline-block w-16 h-px bg-gray-300 dark:bg-gray-700"></div>
        </div>
      </div>

      <!-- Founder Card -->
      <div
        class="border border-gray-200 dark:border-gray-800 rounded-2xl p-8 sm:p-10 bg-white/50 dark:bg-black/50 backdrop-blur-sm transition-all duration-1000 delay-200"
        :class="isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-10'"
      >
        <div class="grid md:grid-cols-5 gap-8 md:gap-12">
          <!-- Left: Photo -->
          <div class="md:col-span-2 flex justify-center md:justify-end">
            <div class="relative">
              <img
                src="/peter.png"
                alt="Prof. Peter Sachsenmeier"
                class="w-64 h-64 md:w-80 md:h-80 object-cover rounded-xl border border-gray-200 dark:border-gray-700"
              />
            </div>
          </div>

          <!-- Right: Info -->
          <div class="md:col-span-3 space-y-6">
            <!-- Name -->
            <div class="mb-8">
              <h3 class="text-2xl sm:text-3xl font-bold text-gray-900 dark:text-white">
                {{ content[language].name }}
              </h3>
            </div>

            <!-- Positions -->
            <div class="space-y-5">
              <div
                v-for="(position, index) in content[language].positions"
                :key="index"
                class="flex items-start gap-3 transition-all duration-1000"
                :style="{ transitionDelay: `${(index + 3) * 100}ms` }"
              >
                <component
                  :is="position.icon"
                  class="w-5 h-5 text-emerald-600 dark:text-emerald-400 flex-shrink-0 mt-0.5"
                />
                <div class="flex-1">
                  <p class="text-base text-gray-700 dark:text-gray-300 leading-relaxed">
                    {{ position.title }}
                  </p>
                  <p v-if="position.org" class="text-sm text-emerald-600 dark:text-emerald-400 mt-1 font-medium">
                    {{ position.org }}
                  </p>
                  <div v-if="position.orgs" class="flex flex-wrap gap-2 mt-2">
                    <span
                      v-for="(org, i) in position.orgs"
                      :key="i"
                      class="text-xs px-2 py-1 border border-gray-300 dark:border-gray-600 text-gray-600 dark:text-gray-400 rounded"
                    >
                      {{ org }}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            <!-- Quote -->
            <div class="mt-8 pt-6 border-t border-gray-200 dark:border-gray-800">
              <p class="text-base sm:text-lg italic text-gray-500 dark:text-gray-500 leading-relaxed">
                {{ content[language].quote }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

