<script setup>
import { ref, onMounted, computed } from 'vue'
import { Calendar, ExternalLink } from 'lucide-vue-next'
import { mockNews } from '../mock/news'

const props = defineProps({
  language: String
})

const content = {
  en: {
    title: 'Media & News',
    subtitle: 'Latest Updates from the Decentralized Science Revolution',
    viewAll: 'View All News',
    readMore: 'Read More'
  },
  zh: {
    title: '媒體新聞',
    subtitle: '去中心化科學革命的最新動態',
    viewAll: '查看全部新聞',
    readMore: '閱讀更多'
  }
}

const displayNews = computed(() => {
  return [...mockNews]
    .filter((item) => item.id === 4)
    .sort((a, b) => new Date(b.date) - new Date(a.date))
    .map((item) => ({
      id: item.id,
      category: props.language === 'en' ? item.category : item.categoryCn,
      title: props.language === 'en' ? item.title : item.titleCn,
      excerpt: props.language === 'en' ? item.excerpt : item.excerptCn,
      date: item.date,
      source: item.source,
      image: item.image,
      externalUrl: item.externalUrl
    }))
})

const isVisible = ref(false)
const hoveredCard = ref(null)

onMounted(() => {
  const observer = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
      if (entry.isIntersecting) {
        isVisible.value = true
      }
    })
  }, { threshold: 0.1 })

  const element = document.querySelector('#media-section')
  if (element) observer.observe(element)
})

const formatDate = (dateStr) => {
  const date = new Date(dateStr)
  return date.toLocaleDateString(props.language === 'zh' ? 'zh-TW' : 'en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric'
  })
}
</script>

<template>
  <section id="media-section" class="bg-white dark:bg-gray-950">
    <div class="mx-auto max-w-[1200px] border-x border-t border-gray-200 px-6 py-24 sm:px-10 lg:px-12 lg:py-32 dark:border-gray-800">
      <div
        class="text-center transition-all duration-1000"
        :class="isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-8'"
      >
        <h2 class="text-5xl font-medium tracking-normal text-gray-950 sm:text-6xl dark:text-white">
          {{ content[language].title }}
        </h2>
      </div>

      <div class="mt-14 grid gap-5 md:grid-cols-3">
        <article
          v-for="(item, index) in displayNews"
          :key="item.id"
          @mouseenter="hoveredCard = index"
          @mouseleave="hoveredCard = null"
          class="group transition-all duration-700 md:col-start-2"
          :class="isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-8'"
          :style="{ transitionDelay: `${index * 90}ms` }"
        >
          <div class="overflow-hidden rounded-xl bg-gray-100">
            <img
              :src="item.image"
              :alt="item.title"
              class="h-[200px] w-full object-cover transition duration-500 group-hover:scale-105"
            />
          </div>

          <div class="pt-5">
            <div class="mb-4 flex flex-wrap items-center gap-2 text-xs text-gray-500">
              <span>{{ item.category }}</span>
              <span>·</span>
              <span class="inline-flex items-center gap-1">
                <Calendar :size="13" />
                {{ formatDate(item.date) }}
              </span>
            </div>

            <h3 class="line-clamp-2 min-h-[52px] text-lg font-medium leading-7 text-gray-950 dark:text-white">
              {{ item.title }}
            </h3>
            <p class="mt-3 line-clamp-2 text-sm leading-6 text-gray-500 dark:text-gray-400">
              {{ item.excerpt }}
            </p>

            <div class="mt-5 flex items-center justify-between text-sm">
              <span class="text-gray-500">{{ item.source }}</span>
              <a
                :href="item.externalUrl"
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex items-center gap-2 font-medium text-gray-950 transition hover:text-gray-600 dark:text-white dark:hover:text-gray-300"
              >
                {{ content[language].readMore }}
                <ExternalLink
                  :size="15"
                  class="transition"
                  :class="hoveredCard === index ? 'translate-x-0.5 -translate-y-0.5' : ''"
                />
              </a>
            </div>
          </div>
        </article>
      </div>

      <div
        class="mt-14 text-center transition-all duration-1000"
        :class="isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-8'"
      >
        <button class="rounded-full bg-[#89ff00] px-7 py-3 text-sm font-medium text-gray-950 transition hover:bg-[#78e800]">
          {{ content[language].viewAll }}
        </button>
      </div>
    </div>
  </section>
</template>
