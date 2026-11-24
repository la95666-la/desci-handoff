<script setup>
import { ref, onMounted } from 'vue'
import { Microscope, Users, ArrowRight, Code2, DollarSign, FlaskConical, Database, FileText, TrendingUp } from 'lucide-vue-next'

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
    title: 'The JoinCare Protocol',
    subtitle: 'DeFi for Real-World Science',
    definition: 'A decentralized funding and value-distribution protocol designed specifically for frontier biomedical R&D.',
    researchers: {
      title: 'For Researchers',
      subtitle: 'Permissionless Innovation',
      features: [
        'Propose and raise funds directly on-chain without gatekeepers',
        'Funds released automatically upon verifiable results',
        'Access technical validation & community support worldwide'
      ]
    },
    supporters: {
      title: 'For Supporters',
      subtitle: 'Transparent Value Capture',
      features: [
        'Project RWAs integrate with DeFi for instant liquidity',
        'Token holders steer protocol and project decisions',
        'Share long-term upside from successful research'
      ]
    },
    valueFlow: [
      { label: 'CODE', icon: 'Code2', color: 'emerald' },
      { label: 'FUNDS', icon: 'DollarSign', color: 'blue' },
      { label: 'RESEARCH', icon: 'FlaskConical', color: 'emerald' },
      { label: 'DATA', icon: 'Database', color: 'blue' },
      { label: 'RWA', icon: 'FileText', color: 'emerald' },
      { label: 'VALUE', icon: 'TrendingUp', color: 'blue' }
    ]
  },
  zh: {
    title: 'JoinCare 協議',
    subtitle: '真實世界科學的 DeFi',
    definition: '一個為前沿生物醫藥研發量身定制的去中心化資助與價值分配底層協議。',
    researchers: {
      title: '對於研究者',
      subtitle: '無需許可的創新',
      features: [
        '通過標準化提案框架直接上鏈融資，無需中介',
        '資金按可驗證成果自動釋放',
        '獲得全球社區的技術驗證與支援'
      ]
    },
    supporters: {
      title: '對於支援者',
      subtitle: '透明的價值捕獲',
      features: [
        '項目RWA可在DeFi生態中產生即時流動性',
        '持有治理代幣決定協議與項目方向',
        '共享項目成功的長期收益'
      ]
    },
    valueFlow: [
      { label: '代碼', icon: 'Code2', color: 'emerald' },
      { label: '資金', icon: 'DollarSign', color: 'blue' },
      { label: '研究', icon: 'FlaskConical', color: 'emerald' },
      { label: '數據', icon: 'Database', color: 'blue' },
      { label: 'RWA', icon: 'FileText', color: 'emerald' },
      { label: '價值', icon: 'TrendingUp', color: 'blue' }
    ]
  }
}

const iconComponents = {
  Code2,
  DollarSign,
  FlaskConical,
  Database,
  FileText,
  TrendingUp
}
</script>

<template>
  <section id="protocol-section" class="relative min-h-screen flex items-center justify-center py-32 overflow-hidden bg-white dark:bg-black">
    <!-- Minimalist Background -->
    <div class="absolute inset-0">
      <div class="absolute inset-0 bg-gradient-to-br from-gray-50 via-white to-gray-100 dark:from-gray-950 dark:via-black dark:to-gray-900"></div>
      <div class="absolute top-1/3 left-0 right-0 h-px bg-gradient-to-r from-transparent via-gray-200 dark:via-gray-800 to-transparent"></div>
    </div>

    <div class="relative z-10 max-w-7xl mx-auto px-6 w-full">
      <div
        class="transition-all duration-1000"
        :class="isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-8'"
      >
        <!-- Header -->
        <div class="text-center mb-24 space-y-6">
          <h2 class="text-5xl sm:text-6xl md:text-7xl lg:text-8xl font-light text-gray-900 dark:text-white tracking-tight leading-[1.1]">
            {{ content[language].title }}
          </h2>
          <p class="text-2xl sm:text-3xl md:text-4xl font-medium text-gray-600 dark:text-gray-400 leading-tight">
            {{ content[language].subtitle }}
          </p>
          <div class="pt-4">
            <div class="inline-block w-16 h-px bg-gray-300 dark:bg-gray-700"></div>
          </div>
          <p class="text-base sm:text-lg text-gray-500 dark:text-gray-500 max-w-2xl mx-auto leading-relaxed">
            {{ content[language].definition }}
          </p>
        </div>

        <!-- Two Column Layout -->
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-16 lg:gap-24 mb-32 max-w-6xl mx-auto">
          <!-- Left: Researchers -->
          <div
            class="relative transition-all duration-700 delay-100"
            :class="isVisible ? 'opacity-100 translate-x-0' : 'opacity-0 -translate-x-12'"
          >
            <div class="space-y-8">
              <!-- Header -->
              <div>
                <div class="inline-flex items-center gap-3 mb-4">
                  <Microscope :size="24" class="text-gray-900 dark:text-white" />
                  <h3 class="text-3xl font-bold text-gray-900 dark:text-white">
                    {{ content[language].researchers.title }}
                  </h3>
                </div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-500 uppercase tracking-wider">
                  {{ content[language].researchers.subtitle }}
                </p>
              </div>

              <!-- Features List -->
              <div class="space-y-6 pl-1">
                <div
                  v-for="(feature, index) in content[language].researchers.features"
                  :key="index"
                  class="group relative pl-6 border-l-2 border-gray-200 dark:border-gray-800 hover:border-gray-900 dark:hover:border-white transition-all duration-300"
                >
                  <div class="absolute left-0 top-0 w-2 h-2 -translate-x-[5px] rounded-full bg-gray-900 dark:bg-white opacity-0 group-hover:opacity-100 transition-opacity duration-300"></div>
                  <p class="text-base text-gray-700 dark:text-gray-300 leading-relaxed">
                    {{ feature }}
                  </p>
                </div>
              </div>
            </div>
          </div>

          <!-- Right: Supporters -->
          <div
            class="relative transition-all duration-700 delay-200"
            :class="isVisible ? 'opacity-100 translate-x-0' : 'opacity-0 translate-x-12'"
          >
            <div class="space-y-8">
              <!-- Header -->
              <div>
                <div class="inline-flex items-center gap-3 mb-4">
                  <Users :size="24" class="text-gray-900 dark:text-white" />
                  <h3 class="text-3xl font-bold text-gray-900 dark:text-white">
                    {{ content[language].supporters.title }}
                  </h3>
                </div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-500 uppercase tracking-wider">
                  {{ content[language].supporters.subtitle }}
                </p>
              </div>

              <!-- Features List -->
              <div class="space-y-6 pl-1">
                <div
                  v-for="(feature, index) in content[language].supporters.features"
                  :key="index"
                  class="group relative pl-6 border-l-2 border-gray-200 dark:border-gray-800 hover:border-gray-900 dark:hover:border-white transition-all duration-300"
                >
                  <div class="absolute left-0 top-0 w-2 h-2 -translate-x-[5px] rounded-full bg-gray-900 dark:bg-white opacity-0 group-hover:opacity-100 transition-opacity duration-300"></div>
                  <p class="text-base text-gray-700 dark:text-gray-300 leading-relaxed">
                    {{ feature }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Value Flow Section -->
        <div
          class="transition-all duration-1000 delay-400"
          :class="isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-8'"
        >
          <!-- Flow Chain -->
          <div class="flex items-center justify-center gap-3 flex-wrap max-w-5xl mx-auto">
            <div
              v-for="(step, index) in content[language].valueFlow"
              :key="index"
              class="flex items-center gap-3"
            >
              <!-- Step -->
              <div
                class="group relative px-6 py-3 rounded-full border-2 transition-all duration-300 hover:shadow-lg hover:-translate-y-1"
                :class="step.color === 'emerald'
                  ? 'border-emerald-500 dark:border-emerald-400 bg-emerald-50 dark:bg-emerald-950/30 hover:shadow-emerald-500/30 dark:hover:shadow-emerald-400/30'
                  : 'border-blue-500 dark:border-blue-400 bg-blue-50 dark:bg-blue-950/30 hover:shadow-blue-500/30 dark:hover:shadow-blue-400/30'"
              >
                <div class="flex items-center gap-2">
                  <component
                    :is="iconComponents[step.icon]"
                    :size="18"
                    :class="step.color === 'emerald'
                      ? 'text-emerald-600 dark:text-emerald-400'
                      : 'text-blue-600 dark:text-blue-400'"
                  />
                  <span
                    class="text-sm font-bold tracking-wide"
                    :class="step.color === 'emerald'
                      ? 'text-emerald-700 dark:text-emerald-300'
                      : 'text-blue-700 dark:text-blue-300'"
                  >
                    {{ step.label }}
                  </span>
                </div>
              </div>

              <!-- Arrow -->
              <ArrowRight
                v-if="index < content[language].valueFlow.length - 1"
                :size="20"
                class="text-gray-400 dark:text-gray-600 hidden sm:block"
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
