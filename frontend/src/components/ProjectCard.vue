<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { localizedProjectName } from '../utils/projectLocalization.js'

const props = defineProps({
  project: {
    type: Object,
    required: true
  },
  language: {
    type: String,
    default: 'en'
  }
})

const router = useRouter()

const content = {
  en: {
    candidate: 'Candidate',
    comingSoon: 'Coming Soon',
    live: 'Live',
    joinNow: 'Join Now',
    closed: 'Closed',
    startsIn: 'Starts in',
    endsIn: 'Ends in',
    price: 'Price',
    progress: 'Progress',
    viewDetails: 'View Details'
  },
  zh: {
    candidate: '候選',
    comingSoon: '即將開始',
    live: '進行中',
    joinNow: '立即認購',
    closed: '已結束',
    startsIn: '開始倒計時',
    endsIn: '結束倒計時',
    price: '價格',
    progress: '進度',
    viewDetails: '查看詳情'
  }
}

const statusConfig = computed(() => {
  const now = Date.now()
  const { status, startTime, endTime } = props.project

  if (status === 'candidate') {
    return {
      label: content[props.language].candidate,
      tone: 'candidate',
      buttonText: content[props.language].viewDetails,
      buttonDisabled: false,
      countdownTarget: null,
      countdownLabel: ''
    }
  } else if (status === 'coming-soon' || (startTime && now < startTime)) {
    return {
      label: content[props.language].comingSoon,
      tone: 'comingSoon',
      buttonText: content[props.language].comingSoon,
      buttonDisabled: true,
      countdownTarget: startTime,
      countdownLabel: content[props.language].startsIn
    }
  } else if (status === 'live' && startTime && endTime && now >= startTime && now < endTime) {
    return {
      label: content[props.language].live,
      tone: 'live',
      buttonText: content[props.language].joinNow,
      buttonDisabled: false,
      countdownTarget: endTime,
      countdownLabel: content[props.language].endsIn
    }
  } else {
    return {
      label: content[props.language].closed,
      tone: 'closed',
      buttonText: content[props.language].closed,
      buttonDisabled: true,
      countdownTarget: null,
      countdownLabel: ''
    }
  }
})

const projectName = computed(() => {
  return localizedProjectName(props.project, props.language)
})

const projectDescription = computed(() => {
  return props.language === 'en' ? props.project.shortDescription : props.project.shortDescriptionCn
})

const projectTags = computed(() => {
  return props.language === 'en' ? props.project.tags : props.project.tagsCn
})

const progressPercent = computed(() => {
  if (!props.project.hardCap) return 0
  return Math.min((props.project.raised / props.project.hardCap) * 100, 100)
})

const hasFundingInfo = computed(() => {
  return props.project.tokenSymbol &&
    props.project.tokenPrice !== null &&
    props.project.tokenPrice !== undefined &&
    props.project.hardCap !== null &&
    props.project.hardCap !== undefined
})

const statusBadgeClass = computed(() => {
  if (statusConfig.value.tone === 'live') return 'bg-[#9bff00] text-[#050b00]'
  if (statusConfig.value.tone === 'candidate') return 'bg-[#eef1ec] text-[#050b00] dark:bg-gray-800 dark:text-gray-200'
  if (statusConfig.value.tone === 'comingSoon') return 'bg-[#ffd665] text-[#050b00]'
  return 'bg-[#f0efec] text-[#050b00] dark:bg-gray-800 dark:text-gray-200'
})

const actionButtonClass = computed(() => {
  if (!statusConfig.value.buttonDisabled) {
    return 'bg-[#9bff00] text-[#050b00] hover:bg-[#8bed00]'
  }
  if (statusConfig.value.tone === 'comingSoon') {
    return 'border border-gray-200 bg-white text-[#050b00] dark:border-gray-800 dark:bg-gray-950 dark:text-white'
  }
  return 'bg-[#f6f6f6] text-[#8a9188] dark:bg-gray-900 dark:text-gray-500'
})

const goToDetail = () => {
  if (props.project.disableDetail) return
  router.push(`/launchpad/${props.project.id}`)
}
</script>

<template>
  <div 
    @click="goToDetail"
    :class="[
      'group flex min-h-[306px] flex-col rounded-[10px] border p-6 transition-colors',
      project.disableDetail ? 'cursor-default' : 'cursor-pointer',
      statusConfig.tone === 'live'
        ? 'border-[#d8ffb0] bg-[#fafff5] hover:border-[#9bff00] dark:border-lime-500/40 dark:bg-lime-950/10'
        : 'border-gray-200 bg-[#fdfdfd] hover:border-gray-300 dark:border-gray-800 dark:bg-gray-950 dark:hover:border-gray-700'
    ]"
  >
    <div class="mb-4 flex items-start gap-3">
      <img :src="project.logo" :alt="projectName" class="h-[48px] w-[48px] flex-shrink-0 rounded-[6px] bg-[#8f8f8f] object-cover" />
      <div class="flex-1 min-w-0">
        <h3 class="truncate text-base font-medium leading-5 text-[#050b00] dark:text-white">
          {{ projectName }}
        </h3>
        <div class="mt-2 flex flex-wrap gap-1.5">
          <span
            v-for="(tag, index) in projectTags"
            :key="index"
            class="inline-flex h-5 items-center rounded-full bg-[#f3ffe4] px-3 text-xs font-normal text-[#3d5a00] dark:bg-lime-500/15 dark:text-lime-300"
          >
            {{ tag }}
          </span>
          <span :class="['inline-flex h-5 items-center rounded-full px-3 text-xs font-normal', statusBadgeClass]">
            {{ statusConfig.label }}
          </span>
        </div>
      </div>
    </div>

    <p class="mb-4 line-clamp-2 min-h-[52px] text-base leading-[1.6] text-[#5f6659] dark:text-gray-400">
      {{ projectDescription }}
    </p>

    <div v-if="hasFundingInfo" class="mt-auto space-y-3">
      <div class="flex justify-between gap-4 text-sm leading-none">
        <span class="text-[#8a9188] dark:text-gray-500">
          {{ content[language].price }}
        </span>
        <span class="text-right font-normal text-[#050b00] dark:text-white">
          1 {{ project.tokenSymbol }} = {{ project.tokenPrice }} JCC
        </span>
      </div>
      <div class="space-y-2">
        <div class="flex justify-between gap-4 text-sm leading-none">
          <span class="text-[#8a9188] dark:text-gray-500">
            {{ content[language].progress }}
          </span>
          <span class="text-right font-normal text-[#050b00] dark:text-white">
            {{ project.raised.toLocaleString() }} / {{ project.hardCap.toLocaleString() }} JCC
          </span>
        </div>
        <div class="h-1.5 overflow-hidden rounded-full bg-[#050b00]/15 dark:bg-white/15">
          <div
            class="h-full rounded-full bg-[#9bff00] transition-all duration-500"
            :style="{ width: `${progressPercent}%` }"
          ></div>
        </div>
      </div>
    </div>

    <button
      :disabled="statusConfig.buttonDisabled"
      :class="['mt-6 h-10 w-full rounded-full text-base font-normal transition-colors disabled:cursor-not-allowed', actionButtonClass]"
      @click.stop="!statusConfig.buttonDisabled && goToDetail()"
    >
      {{ statusConfig.buttonText }}
    </button>
  </div>
</template>
