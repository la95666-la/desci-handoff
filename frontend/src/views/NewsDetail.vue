<script setup>
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Calendar, User, Tag, Share2 } from 'lucide-vue-next'
// 靜態新聞數據（前端固定內容）
import { mockNews } from '../mock/news'
import { getNewsDetail } from '../services/api'
import { sanitize } from '../utils/sanitize'

const props = defineProps({
  language: String
})

const route = useRoute()
const router = useRouter()
const news = ref(null)
const loading = ref(true)

const content = {
  en: {
    back: 'Back to News',
    author: 'Author',
    publishedOn: 'Published on',
    tags: 'Tags',
    share: 'Share',
    linkCopied: 'Link copied to clipboard!',
    relatedNews: 'Related News',
    notFound: 'News not found',
    goHome: 'Go to Homepage'
  },
  tw: {
    back: '返回新聞',
    author: '作者',
    publishedOn: '發布於',
    tags: '標籤',
    share: '分享',
    linkCopied: '連結已複製到剪貼板！',
    relatedNews: '相關新聞',
    notFound: '新聞未找到',
    goHome: '返回首頁'
  }
}

const ui = computed(() => props.language === 'en' ? content.en : content.tw)

const escapeHtml = (value = '') => String(value)
  .replaceAll('&', '&amp;')
  .replaceAll('<', '&lt;')
  .replaceAll('>', '&gt;')
  .replaceAll('"', '&quot;')
  .replaceAll("'", '&#039;')

const mapPublicNews = (item) => ({
  id: Number(item.id),
  image: item.cover_url,
  title: item.title,
  titleCn: item.title,
  excerpt: item.summary,
  excerptCn: item.summary,
  content: item.content || (item.summary ? `<p>${escapeHtml(item.summary)}</p>` : ''),
  contentCn: item.content || (item.summary ? `<p>${escapeHtml(item.summary)}</p>` : ''),
  category: item.category || 'News',
  categoryCn: item.category || 'News',
  tags: item.category ? [item.category] : [],
  tagsCn: item.category ? [item.category] : [],
  author: item.author || item.source_name || 'JoinCare',
  authorCn: item.author || item.source_name || 'JoinCare',
  date: item.publish_at,
  source: item.source_name || 'JoinCare',
  isRemote: true
})

async function loadNews() {
  const id = Number(route.params.id)
  news.value = null
  loading.value = true
  try {
    const item = await getNewsDetail(id, {
      language: props.language === 'en' ? 'en' : 'tw'
    })
    if (item?.id) news.value = mapPublicNews(item)
  } catch (error) {
    // Keep legacy static articles available while old links are still in circulation.
    news.value = mockNews.find(item => item.id === id) || null
    if (!news.value) console.warn('[NewsDetail] load failed', error?.message)
  } finally {
    loading.value = false
  }
}

watch(
  [() => route.params.id, () => props.language],
  loadNews,
  { immediate: true }
)

const newsTitle = computed(() => {
  return props.language === 'en' ? news.value?.title : news.value?.titleCn
})

const newsContent = computed(() => {
  return props.language === 'en' ? news.value?.content : news.value?.contentCn
})

const newsCategory = computed(() => {
  return props.language === 'en' ? news.value?.category : news.value?.categoryCn
})

const newsTags = computed(() => {
  return props.language === 'en' ? news.value?.tags : news.value?.tagsCn
})

const newsAuthor = computed(() => {
  return props.language === 'en' ? news.value?.author : news.value?.authorCn
})

const relatedNews = computed(() => {
  if (!news.value || news.value.isRemote) return []
  const currentID = Number(route.params.id)
  return mockNews
    .filter(n => n.id !== currentID && n.category === news.value.category)
    .slice(0, 3)
})

const formatDate = (dateStr) => {
  const date = new Date(dateStr)
  return date.toLocaleDateString(props.language === 'en' ? 'en-US' : 'zh-TW', {
    year: 'numeric',
    month: 'long',
    day: 'numeric'
  })
}

const goBack = () => {
  router.push('/')
}

const shareArticle = () => {
  if (navigator.share) {
    navigator.share({
      title: newsTitle.value,
      url: window.location.href
    })
  } else {
    navigator.clipboard.writeText(window.location.href)
    alert(ui.value.linkCopied)
  }
}

const viewRelatedNews = (id) => {
  router.push(`/news/${id}`)
  window.scrollTo(0, 0)
}
</script>

<template>
  <div class="min-h-screen pt-32 pb-20 bg-gradient-to-br from-gray-50 via-white to-blue-50/30 dark:from-gray-950 dark:via-gray-900 dark:to-blue-950/30">
    <div class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8">
      <!-- Back Button -->
      <button 
        @click="goBack"
        class="flex items-center gap-2 text-gray-600 dark:text-gray-400 hover:text-primary dark:hover:text-primary-400 mb-8 transition-colors"
      >
        <ArrowLeft :size="20" />
        <span>{{ ui.back }}</span>
      </button>

      <div v-if="loading" class="text-center py-20 text-gray-600 dark:text-gray-400">
        {{ language === 'en' ? 'Loading news...' : '正在載入新聞...' }}
      </div>

      <!-- News Not Found -->
      <div v-else-if="!news" class="text-center py-20">
        <div class="text-6xl mb-4">📰</div>
        <h2 class="text-2xl font-bold text-gray-900 dark:text-white mb-4">
          {{ ui.notFound }}
        </h2>
        <button
          @click="goBack"
          class="px-6 py-3 bg-primary hover:bg-primary-600 text-white rounded-xl font-semibold transition-all"
        >
          {{ ui.goHome }}
        </button>
      </div>

      <!-- News Content -->
      <article v-else class="bg-white dark:bg-gray-900 rounded-2xl overflow-hidden border border-gray-200 dark:border-gray-800">
        <!-- Featured Image -->
        <div class="relative h-96 overflow-hidden">
          <img 
            :src="news.image" 
            :alt="newsTitle"
            class="w-full h-full object-cover"
          />
          <div class="absolute inset-0 bg-gradient-to-t from-black/60 via-black/20 to-transparent"></div>
          
          <!-- Category Badge -->
          <div class="absolute top-6 left-6">
            <span class="px-4 py-2 bg-white/90 dark:bg-black/90 backdrop-blur-sm text-sm font-semibold text-gray-900 dark:text-white rounded-full">
              {{ newsCategory }}
            </span>
          </div>
        </div>

        <!-- Article Header -->
        <div class="p-8 sm:p-12">
          <h1 class="text-3xl sm:text-4xl md:text-5xl font-bold text-gray-900 dark:text-white mb-6 leading-tight">
            {{ newsTitle }}
          </h1>

          <!-- Meta Information -->
          <div class="flex flex-wrap items-center gap-6 text-sm text-gray-600 dark:text-gray-400 mb-8 pb-8 border-b border-gray-200 dark:border-gray-800">
            <div class="flex items-center gap-2">
              <User :size="16" />
              <span>{{ newsAuthor }}</span>
            </div>
            <div class="flex items-center gap-2">
              <Calendar :size="16" />
              <span>{{ formatDate(news.date) }}</span>
            </div>
            <div class="flex items-center gap-2">
              <span class="font-medium">{{ news.source }}</span>
            </div>
            <button
              @click="shareArticle"
              class="ml-auto flex items-center gap-2 px-4 py-2 bg-gray-100 dark:bg-gray-800 hover:bg-gray-200 dark:hover:bg-gray-700 rounded-lg transition-colors"
            >
              <Share2 :size="16" />
              <span>{{ ui.share }}</span>
            </button>
          </div>

          <!-- Article Content -->
          <div class="prose prose-lg dark:prose-invert max-w-none mb-12" v-html="sanitize(newsContent)"></div>

          <!-- Tags -->
          <div class="flex flex-wrap items-center gap-3 mb-8">
            <Tag :size="18" class="text-gray-400" />
            <span
              v-for="(tag, index) in newsTags"
              :key="index"
              class="px-3 py-1.5 bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300 rounded-full text-sm font-medium"
            >
              {{ tag }}
            </span>
          </div>
        </div>
      </article>

      <!-- Related News -->
      <div v-if="relatedNews.length > 0" class="mt-16">
        <h2 class="text-2xl sm:text-3xl font-bold text-gray-900 dark:text-white mb-8">
          {{ ui.relatedNews }}
        </h2>

        <div class="grid md:grid-cols-3 gap-6">
          <div
            v-for="item in relatedNews"
            :key="item.id"
            @click="viewRelatedNews(item.id)"
            class="group cursor-pointer border border-gray-200 dark:border-gray-800 rounded-xl overflow-hidden hover:border-gray-300 dark:hover:border-gray-700 transition-all bg-white dark:bg-gray-900"
          >
            <!-- Thumbnail -->
            <div class="relative h-48 overflow-hidden">
              <img
                :src="item.image"
                :alt="language === 'en' ? item.title : item.titleCn"
                class="w-full h-full object-cover transition-transform duration-500 group-hover:scale-105"
              />
              <div class="absolute inset-0 bg-gradient-to-t from-black/60 via-black/20 to-transparent"></div>
              <div class="absolute bottom-3 left-3">
                <span class="inline-block px-2 py-1 bg-white/90 dark:bg-black/90 backdrop-blur-sm text-xs font-medium text-gray-900 dark:text-white uppercase tracking-wider rounded">
                  {{ language === 'en' ? item.category : item.categoryCn }}
                </span>
              </div>
            </div>

            <!-- Content -->
            <div class="p-4">
              <h3 class="text-lg font-bold mb-2 text-gray-900 dark:text-white line-clamp-2">
                {{ language === 'en' ? item.title : item.titleCn }}
              </h3>
              <p class="text-sm text-gray-600 dark:text-gray-400 line-clamp-2">
                {{ language === 'en' ? item.excerpt : item.excerptCn }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Prose styling for article content */
:deep(.prose) {
  color: inherit;
}

:deep(.prose h3) {
  @apply text-2xl font-bold text-gray-900 dark:text-white mt-8 mb-4;
}

:deep(.prose p) {
  @apply text-gray-700 dark:text-gray-300 leading-relaxed mb-4;
}

:deep(.prose ul) {
  @apply list-disc list-inside space-y-2 mb-6;
}

:deep(.prose li) {
  @apply text-gray-700 dark:text-gray-300;
}

:deep(.prose a) {
  @apply text-teal-700 dark:text-teal-400 hover:underline;
}
</style>
