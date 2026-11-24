<script setup>
import { ref, onMounted } from 'vue'
import { ArrowRight } from 'lucide-vue-next'

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
    title: 'Join the Movement',
    subtitle: 'Stop Betting on Trends. Start Building the Future.',
    message: 'Every vote you cast, every contribution you make, becomes a line of code written into the history of human health.',
    buttons: [
      { text: 'Connect Wallet', primary: true, url: '#' },
      { text: 'Browse Projects', primary: false, url: '#projects-section' },
      { text: 'Read Whitepaper', primary: false, url: 'https://joincare.gitbook.io/joincare-docs' }
    ]
  },
  zh: {
    title: '加入運動',
    subtitle: '停止押注趨勢。開始構建未來。',
    message: '您的每一次投票、每一份資助，都是一行寫入人類健康史的代碼。',
    buttons: [
      { text: '連接錢包', primary: true, url: '#' },
      { text: '瀏覽項目', primary: false, url: '#projects-section' },
      { text: '閱讀白皮書', primary: false, url: 'https://joincare.gitbook.io/joincare-docs' }
    ]
  }
}
</script>

<template>
  <section class="bg-white dark:bg-gray-950">
    <div class="mx-auto max-w-[1200px] border-x border-t border-gray-200 px-6 py-24 sm:px-10 lg:px-12 dark:border-gray-800">
      <div
        class="transition-all duration-1000"
        :class="isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-8'"
      >
        <h2 class="text-center text-5xl font-medium tracking-normal text-gray-950 sm:text-6xl dark:text-white">
          {{ content[language].title }}
        </h2>

        <div class="relative mt-14 min-h-[416px] overflow-hidden rounded-none bg-gray-950">
          <video
            class="absolute inset-0 h-full w-full object-cover"
            autoplay
            muted
            loop
            playsinline
            preload="metadata"
            aria-hidden="true"
          >
            <source src="/figma/join.mp4" type="video/mp4" />
          </video>
          <div class="absolute inset-0 bg-black/20"></div>

          <div class="relative z-10 flex min-h-[416px] max-w-2xl flex-col justify-center px-8 py-14 text-white sm:px-12 lg:px-16">
            <p class="max-w-xl text-3xl font-medium leading-tight tracking-normal sm:text-4xl">
              {{ content[language].subtitle }}
            </p>
            <p class="mt-6 max-w-lg text-sm leading-7 text-white/85">
              {{ content[language].message }}
            </p>

            <div class="mt-8 flex flex-wrap gap-3">
              <a
                v-for="(btn, index) in content[language].buttons"
                :key="index"
                :href="btn.url"
                :target="btn.url.startsWith('http') ? '_blank' : '_self'"
                :rel="btn.url.startsWith('http') ? 'noopener noreferrer' : ''"
                class="group inline-flex h-11 items-center justify-center gap-2 rounded-full px-5 text-sm font-medium transition"
                :class="btn.primary
                  ? 'bg-white text-gray-950 hover:bg-gray-100'
                  : 'border border-white/30 bg-white/20 text-white backdrop-blur hover:bg-white/30'"
              >
                <span>{{ btn.text }}</span>
                <ArrowRight
                  v-if="btn.primary"
                  :size="16"
                  class="transition-transform group-hover:translate-x-0.5"
                />
              </a>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
