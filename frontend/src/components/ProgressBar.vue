<script setup>
import { computed } from 'vue'

const props = defineProps({
  current: {
    type: Number,
    required: true
  },
  total: {
    type: Number,
    required: true
  },
  showPercentage: {
    type: Boolean,
    default: true
  }
})

const percentage = computed(() => {
  if (props.total === 0) return 0
  return Math.min(Math.round((props.current / props.total) * 100), 100)
})

const progressColor = computed(() => {
  if (percentage.value >= 100) return 'bg-green-500'
  if (percentage.value >= 75) return 'bg-primary'
  if (percentage.value >= 50) return 'bg-secondary'
  return 'bg-accent'
})
</script>

<template>
  <div class="w-full">
    <div class="relative w-full h-3 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
      <div 
        :class="[progressColor, 'h-full transition-all duration-500 ease-out rounded-full']"
        :style="{ width: `${percentage}%` }"
      >
        <div class="absolute inset-0 bg-gradient-to-r from-transparent via-white/20 to-transparent animate-shimmer"></div>
      </div>
    </div>
    <div v-if="showPercentage" class="mt-1 text-sm text-gray-600 dark:text-gray-400 text-right">
      {{ percentage }}%
    </div>
  </div>
</template>

<style scoped>
@keyframes shimmer {
  0% {
    transform: translateX(-100%);
  }
  100% {
    transform: translateX(100%);
  }
}

.animate-shimmer {
  animation: shimmer 2s infinite;
}
</style>

