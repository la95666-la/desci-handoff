<script setup>
import { ref, computed, onMounted } from 'vue'

const props = defineProps({
  language: {
    type: String,
    default: 'en'
  }
})

const isVisible = ref(false)
const currentAmount = ref(0)
const targetAmount = 6000000 // Pre-A 目標募資 $6M
const actualAmount = 2000000 // 已募 $2M，完成 33%

onMounted(() => {
  setTimeout(() => {
    isVisible.value = true
  }, 100)

  setTimeout(() => {
    animateProgress()
  }, 800)
})

const animateProgress = () => {
  if (currentAmount.value > 0) return

  const duration = 2000
  const steps = 60
  const increment = actualAmount / steps
  let current = 0

  const timer = setInterval(() => {
    current += increment
    if (current >= actualAmount) {
      currentAmount.value = actualAmount
      clearInterval(timer)
    } else {
      currentAmount.value = Math.floor(current)
    }
  }, duration / steps)
}

const progressPercentage = computed(() => (actualAmount / targetAmount) * 100)

const content = {
  en: {
    title: 'Pre-A Funding',
    description: 'JoinCare is opening its Pre-A funding round to support protocol development, research partnerships, and ecosystem growth.',
    target: 'Target Raise',
    current: 'Current',
    completed: 'Completed',
    remaining: 'Remaining'
  },
  zh: {
    title: 'Pre-A 融資',
    description: 'JoinCare 正開啟 Pre-A 輪融資，用於支援協議開發、研究合作伙伴關係與生態成長。',
    target: '目標募資',
    current: '當前',
    completed: '已完成',
    remaining: '剩餘'
  }
}

const t = computed(() => content[props.language] || content.en)

const formatCurrency = (amount) => {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0
  }).format(amount)
}

const formatCurrentAmount = (amount) => {
  if (amount === 0) return '$0'
  const value = amount / 1000000
  return `$${Number.isInteger(value) ? value : value.toFixed(1)}M`
}

const formatMillionAmount = (amount) => {
  const value = amount / 1000000
  return `$${Number.isInteger(value) ? value : value.toFixed(1)}M`
}
</script>

<template>
  <section id="funding-section" class="bg-white dark:bg-gray-950">
    <div class="mx-auto max-w-[1200px] border-x border-gray-200 px-6 py-24 sm:px-10 lg:px-12 lg:py-32 dark:border-gray-800">
      <div
        class="transition-all duration-1000"
        :class="isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-8'"
      >
        <div class="mx-auto max-w-4xl text-center">
          <h2 class="text-5xl font-medium tracking-normal text-gray-950 sm:text-7xl dark:text-white">
            {{ t.title }}
          </h2>
          <p class="mx-auto mt-8 max-w-3xl text-xl leading-9 text-gray-600 dark:text-gray-400">
            {{ t.description }}
          </p>
        </div>

        <div class="mt-16 grid gap-6 md:grid-cols-3">
          <div class="flex min-h-[280px] flex-col justify-between rounded-2xl bg-[#fbfcfa] p-12 shadow-sm dark:bg-gray-900">
            <div class="text-5xl font-medium tracking-tight text-gray-950 dark:text-white">{{ formatMillionAmount(targetAmount) }}</div>
            <div>
              <div class="text-base font-semibold uppercase text-gray-950 dark:text-white">{{ t.target }}</div>
              <p class="mt-3 text-lg text-gray-500">{{ formatCurrency(targetAmount) }}</p>
            </div>
          </div>

          <div class="flex min-h-[280px] items-center justify-center rounded-2xl border border-gray-200 bg-white p-8 dark:border-gray-800 dark:bg-gray-950">
            <div class="relative h-52 w-52">
              <svg viewBox="0 0 120 120" class="h-full w-full -rotate-90">
                <circle cx="60" cy="60" r="45" pathLength="100" fill="none" stroke="#dff8cf" stroke-width="8" stroke-linecap="round" />
                <circle
                  cx="60"
                  cy="60"
                  r="45"
                  pathLength="100"
                  fill="none"
                  stroke="#89ff00"
                  stroke-width="8"
                  stroke-linecap="round"
                  stroke-dasharray="18 82"
                  stroke-dashoffset="-2"
                  class="transition-all duration-1000"
                />
                <circle
                  cx="60"
                  cy="60"
                  r="45"
                  pathLength="100"
                  fill="none"
                  stroke="#3f7df2"
                  stroke-width="8"
                  stroke-linecap="round"
                  stroke-dasharray="18 82"
                  stroke-dashoffset="-21"
                />
              </svg>
              <div class="absolute inset-0 flex flex-col items-center justify-center px-8 text-center">
                <span class="whitespace-nowrap text-sm font-medium text-gray-400">{{ formatMillionAmount(targetAmount - actualAmount) }} {{ t.remaining }}</span>
                <span class="mt-2 whitespace-nowrap text-base font-medium text-gray-950 dark:text-white">{{ Math.round(progressPercentage) }}% {{ t.completed }}</span>
              </div>
            </div>
          </div>

          <div class="flex min-h-[280px] flex-col justify-between rounded-2xl bg-[#fbfcfa] p-12 shadow-sm dark:bg-gray-900">
            <div class="text-5xl font-medium tracking-tight text-gray-950 dark:text-white">{{ formatCurrentAmount(currentAmount) }}</div>
            <div>
              <div class="text-base font-semibold uppercase text-gray-950 dark:text-white">{{ t.current }}</div>
              <p class="mt-3 text-lg text-gray-500">{{ formatCurrency(actualAmount) }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
