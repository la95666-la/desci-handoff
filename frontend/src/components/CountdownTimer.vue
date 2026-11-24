<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'

const props = defineProps({
  targetTime: {
    type: Number,
    required: true
  },
  language: {
    type: String,
    default: 'en'
  }
})

const now = ref(Date.now())
let interval = null

onMounted(() => {
  interval = setInterval(() => {
    now.value = Date.now()
  }, 1000)
})

onUnmounted(() => {
  if (interval) {
    clearInterval(interval)
  }
})

const timeLeft = computed(() => {
  const diff = props.targetTime - now.value
  
  if (diff <= 0) {
    return { days: 0, hours: 0, minutes: 0, seconds: 0, isExpired: true }
  }

  const days = Math.floor(diff / (1000 * 60 * 60 * 24))
  const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60))
  const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60))
  const seconds = Math.floor((diff % (1000 * 60)) / 1000)

  return { days, hours, minutes, seconds, isExpired: false }
})

const content = {
  en: { days: 'Days', hours: 'Hours', minutes: 'Minutes', seconds: 'Seconds', expired: 'Expired' },
  zh: { days: '天', hours: '時', minutes: '分', seconds: '秒', expired: '已結束' }
}

const labels = computed(() => {
  return content[props.language] || content['en']
})
</script>

<template>
  <div v-if="!timeLeft.isExpired" class="flex items-center gap-2 sm:gap-4">
    <div class="flex flex-col items-center">
      <div class="text-2xl sm:text-3xl font-bold text-primary dark:text-primary-400">
        {{ String(timeLeft.days).padStart(2, '0') }}
      </div>
      <div class="text-xs text-gray-500 dark:text-gray-400">{{ labels.days }}</div>
    </div>
    <div class="text-2xl font-bold text-gray-400">:</div>
    <div class="flex flex-col items-center">
      <div class="text-2xl sm:text-3xl font-bold text-primary dark:text-primary-400">
        {{ String(timeLeft.hours).padStart(2, '0') }}
      </div>
      <div class="text-xs text-gray-500 dark:text-gray-400">{{ labels.hours }}</div>
    </div>
    <div class="text-2xl font-bold text-gray-400">:</div>
    <div class="flex flex-col items-center">
      <div class="text-2xl sm:text-3xl font-bold text-primary dark:text-primary-400">
        {{ String(timeLeft.minutes).padStart(2, '0') }}
      </div>
      <div class="text-xs text-gray-500 dark:text-gray-400">{{ labels.minutes }}</div>
    </div>
    <div class="text-2xl font-bold text-gray-400">:</div>
    <div class="flex flex-col items-center">
      <div class="text-2xl sm:text-3xl font-bold text-primary dark:text-primary-400">
        {{ String(timeLeft.seconds).padStart(2, '0') }}
      </div>
      <div class="text-xs text-gray-500 dark:text-gray-400">{{ labels.seconds }}</div>
    </div>
  </div>
  <div v-else class="text-lg font-semibold text-gray-500 dark:text-gray-400">
    {{ labels.expired }}
  </div>
</template>

