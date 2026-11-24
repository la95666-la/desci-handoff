<script setup>
import { ref } from 'vue'
import { Plus, Minus } from 'lucide-vue-next'

defineProps({
  language: String
})

const openIndex = ref(null)

const toggleQuestion = (index) => {
  openIndex.value = openIndex.value === index ? null : index
}

const content = {
  en: {
    title: 'Frequently Asked Questions',
    subtitle: 'Everything you need to know about JoinCare',
    questions: [
      {
        question: 'Is JoinCare a medical service platform?',
        answer: 'No. JoinCare does not provide medical diagnoses, treatment recommendations, clinical services, or guarantees of therapeutic outcomes.'
      },
      {
        question: 'What are CareAgents?',
        answer: 'CareAgents are AI-assisted tools for document organization, literature analysis, milestone tracking, and risk identification. Key outputs require human review.'
      },
      {
        question: 'What is $JC?',
        answer: '$JC is JoinCare’s platform governance token, used for contribution records, incentives, governance participation, and selected platform features. It does not represent equity or guaranteed returns, and related information does not constitute investment advice.'
      },
      {
        question: 'What is JoinCare Candidate Project?',
        answer: 'JoinCare Candidate Project connects qualified incubated projects with ecosystem participation, progress tracking, feedback, and resource support. It does not guarantee returns, exchange listings, project success, or medical outcomes.'
      }
    ]
  },
  zh: {
    title: '常見問題',
    subtitle: '關於 JoinCare 您需要了解的一切',
    questions: [
      {
        question: 'JoinCare是醫療服務平台嗎？',
        answer: '不是。JoinCare不提供醫療診斷、治療建議或臨床服務，也不對任何治療效果作出保證。'
      },
      {
        question: '什麼是 CareAgents？',
        answer: 'CareAgents是用於資料整理、文獻分析、里程碑追蹤和風險識別的AI輔助工具。重要的結論須經過人工複核。'
      },
      {
        question: '什麼是 $JC？',
        answer: '$JC是JoinCare平台的治理Token，用於貢獻記錄、激勵、治理參與及部分平台功能。$JC不代表股權或任何保證性收益，相關信息不構成投資建議。'
      },
      {
        question: '什麼是 JoinCare 候選項目？',
        answer: 'JoinCare 候選項目為符合條件的孵化項目提供生態參與、進展追蹤、反饋及資源支持。候選項目不保證投資回報、交易所上線、項目成功或醫療結果。'
      }
    ]
  }
}
</script>

<template>
  <section id="faq-section" class="bg-white dark:bg-gray-950">
    <div class="mx-auto max-w-[1200px] border-x border-t border-gray-200 py-24 dark:border-gray-800">
      <div class="px-6 sm:px-10 lg:px-12">
        <h2 class="text-center text-5xl font-medium tracking-normal text-gray-950 sm:text-6xl dark:text-white">
          {{ content[language].title }}
        </h2>
        <p class="mt-5 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ content[language].subtitle }}
        </p>
      </div>

      <div class="mt-16 border-y border-gray-200 dark:border-gray-800">
        <div
          v-for="(item, index) in content[language].questions"
          :key="index"
          class="border-b border-gray-200 last:border-b-0 dark:border-gray-800"
          :class="openIndex === index ? 'bg-[#f7faf6] dark:bg-gray-900/70' : 'bg-white dark:bg-gray-950'"
        >
          <button
            @click="toggleQuestion(index)"
            class="flex w-full items-center justify-between gap-6 px-6 py-7 text-left transition-colors sm:px-10 lg:px-12"
          >
            <span class="text-base font-medium text-gray-950 dark:text-white">
              {{ item.question }}
            </span>
            <Minus v-if="openIndex === index" :size="18" class="shrink-0 text-gray-600 dark:text-gray-400" />
            <Plus v-else :size="18" class="shrink-0 text-gray-600 dark:text-gray-400" />
          </button>

          <div
            class="overflow-hidden transition-all duration-300"
            :class="openIndex === index ? 'max-h-80' : 'max-h-0'"
          >
            <p class="max-w-4xl px-6 pb-8 text-sm leading-7 text-gray-500 sm:px-10 lg:px-12 dark:text-gray-400">
              {{ item.answer }}
            </p>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
