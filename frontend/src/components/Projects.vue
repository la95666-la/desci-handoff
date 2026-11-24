<script setup>
const props = defineProps({
  language: String
})

const content = {
  en: {
    title: 'Featured Projects',
    subtitle: 'Funding the Science That Will Change the World',
    description: 'Real research. Real impact. Real ownership.',
    comingSoon: 'Coming Soon'
  },
  zh: {
    title: '精選項目',
    subtitle: '資助正在改變世界的科學',
    description: '真實的研究。真實的影響。真實的所有權。',
    comingSoon: '即將推出'
  }
}
</script>

<template>
  <section id="projects-section" class="bg-white dark:bg-gray-950">
    <div class="mx-auto max-w-[1200px] border-x border-t border-gray-200 py-24 dark:border-gray-800">
      <div class="px-6 text-center sm:px-10 lg:px-12">
        <p class="text-sm font-medium uppercase text-gray-500 dark:text-gray-400">
          {{ content[language].subtitle }}
        </p>
        <h2 class="mt-5 text-5xl font-medium tracking-normal text-gray-950 sm:text-6xl dark:text-white">
          {{ content[language].title }}
        </h2>
        <p class="mt-5 text-sm text-gray-500 dark:text-gray-400">
          {{ content[language].description }}
        </p>
      </div>

      <div class="mt-20 border-y border-gray-200 bg-[#f8faf7] px-6 py-24 text-center dark:border-gray-800 dark:bg-gray-900 sm:px-10">
        <p class="text-4xl font-medium tracking-normal text-gray-950 dark:text-white sm:text-5xl">
          {{ content[language].comingSoon }}
        </p>
      </div>
    </div>
  </section>
</template>
