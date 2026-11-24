<script setup>
import { ref, onMounted, inject, computed } from 'vue'
import { TRANSLATIONS } from '../i18n/home.js'
import { getTaskList } from '../services/api'
import { SHOW_TASKS_ENTRY } from '../config/featureVisibility.js'

const props = defineProps({
  language: { type: String, default: 'en' },
  // 非首页风格页面（走 Tailwind html.dark 主题）传入 true/false；首页风格页面不传（默认 null，跟随 isHomeDark）
  tailwindDark: { type: Boolean, default: null },
  // 1440 缩放画布内使用：内容保持画布布局，背景条带按实际视口宽度出血。
  fullBleed: { type: Boolean, default: false },
  scale: { type: Number, default: 1 },
})
const t = computed(() => TRANSLATIONS[props.language] || TRANSLATIONS.en || {})
// 与 App.vue 同源的深色主题状态 —— dark 模式下 footer 反色（浅底深字）
const homeDark = inject('isHomeDark', ref(false))
const isDark = computed(() => (props.tailwindDark === null ? homeDark.value : props.tailwindDark))
const footerStyle = computed(() => props.fullBleed
  ? { '--jc-footer-scale': String(Math.max(Number(props.scale) || 1, 0.01)) }
  : undefined)

const telegramUrl = ref('https://t.me/JoinCareOfficial')
onMounted(async () => {
  try {
    const res = await getTaskList({ platform: 'telegram' })
    const list = Array.isArray(res) ? res : (res?.list || [])
    const join = list.find(x => x.task_key === 'join' || x.key === 'join')
    const chat = list.find(x => x.task_key === 'chat' || x.key === 'chat')
    const target = join || chat || list[0]
    if (target?.url) telegramUrl.value = target.url
  } catch (_) { /* silent */ }
})
</script>

<template>
  <footer
    class="jc-hf"
    :class="{ 'jc-hf-dark': isDark, 'jc-hf-full-bleed': props.fullBleed }"
    :style="footerStyle"
  >
    <div class="jc-hf-inner">
      <img src="/figma/v2/footer-logo.svg" alt="JoinCare" class="jc-hf-logo" />

      <div class="jc-hf-cols">
        <!-- Addresses -->
        <div class="jc-hf-col jc-hf-addr">
          <div v-if="t.footerLabSG && t.footerLabSG.trim()" class="jc-hf-addr-item">
            <p class="jc-hf-h">{{ t.footerLabSG }}</p>
            <p class="jc-hf-p">{{ t.footerLabSGLine1 }}<template v-if="t.footerLabSGLine2"><br />{{ t.footerLabSGLine2 }}</template></p>
          </div>
          <div v-if="t.footerResBos && t.footerResBos.trim()" class="jc-hf-addr-item">
            <p class="jc-hf-h">{{ t.footerResBos }}</p>
            <p class="jc-hf-p">{{ t.footerResBosLine1 }}<template v-if="t.footerResBosLine2"><br />{{ t.footerResBosLine2 }}</template></p>
          </div>
          <div v-if="t.footerContact && t.footerContact.trim()" class="jc-hf-addr-item">
            <p class="jc-hf-h">{{ t.footerContact }}</p>
            <p class="jc-hf-p">{{ t.footerContactLine1 }}<template v-if="t.footerContactLine2"><br />{{ t.footerContactLine2 }}</template></p>
          </div>
        </div>

        <div class="jc-hf-col">
          <p class="jc-hf-h">{{ t.footerColExplore }}</p>
          <router-link to="/" class="jc-hf-link">{{ t.footerLinkHome }}</router-link>
          <router-link to="/ecosystem" class="jc-hf-link">{{ t.footerLinkEcosystem }}</router-link>
          <router-link to="/launchpad" class="jc-hf-link">{{ t.footerLinkLaunchpad }}</router-link>
          <router-link v-if="SHOW_TASKS_ENTRY" to="/tasks" class="jc-hf-link">{{ t.footerLinkQuest }}</router-link>
        </div>

        <div class="jc-hf-col">
          <p class="jc-hf-h">{{ t.footerColBuilders }}</p>
          <router-link to="/launchpad/apply" class="jc-hf-link">{{ t.footerLinkSubmit }}</router-link>
        </div>

        <div class="jc-hf-col">
          <p class="jc-hf-h">{{ t.footerColAbout }}</p>
          <router-link to="/team" class="jc-hf-link">{{ t.footerLinkTeam }}</router-link>
          <router-link to="/news" class="jc-hf-link">{{ t.footerLinkNews }}</router-link>
          <a href="https://joincare.gitbook.io/joincare-docs" target="_blank" rel="noopener" class="jc-hf-link">{{ t.footerLinkDocs }}</a>
        </div>

        <div class="jc-hf-col">
          <p class="jc-hf-h">{{ t.footerColCommunity }}</p>
          <a href="https://x.com/TheJoinCare" target="_blank" rel="noopener" class="jc-hf-link">X (Twitter)</a>
          <a :href="telegramUrl" target="_blank" rel="noopener" class="jc-hf-link">Telegram</a>
        </div>
      </div>

      <div class="jc-hf-bottom">
        <p v-if="t.footerCopyright">{{ t.footerCopyright }}</p>
        <p v-if="t.footerLegal">{{ t.footerLegal }}</p>
      </div>
    </div>
  </footer>
</template>

<style scoped>
.jc-hf {
  position: relative;
  isolation: isolate;
  overflow: hidden;
  width: 100%;
  background: #222;
  color: white;
  padding: 60px 40px 40px;
  box-sizing: border-box;
}
.jc-hf::before {
  content: "";
  position: absolute;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  background-image: url('/figma/v2/noise-bg.png');
  background-size: 500px 500px;
  background-position: top left;
  opacity: 0.12;
}
.jc-hf-full-bleed::after {
  content: "";
  position: absolute;
  inset: 0 auto 0 50%;
  z-index: -2;
  width: calc(100vw / var(--jc-footer-scale, 1));
  transform: translateX(-50%);
  background: #222;
  pointer-events: none;
}
.jc-hf-full-bleed { overflow: visible; }
.jc-hf-full-bleed::before {
  z-index: -1;
  inset: 0 auto 0 50%;
  width: calc(100vw / var(--jc-footer-scale, 1));
  transform: translateX(-50%);
}
.jc-hf-inner {
  position: relative;
  z-index: 1;
  max-width: 1440px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 48px;
}
.jc-hf-logo {
  /* footer-logo.svg width=100% height=100% 无固有尺寸；按 viewBox 399.6:111 (≈3.6:1) 固定 width */
  width: 200px;
  height: auto;
  aspect-ratio: 399.638 / 111;
  display: block;
}
.jc-hf-cols {
  display: grid;
  grid-template-columns: 1.4fr 1fr 1fr 1fr 1fr;
  gap: 32px;
  align-items: start;
}
.jc-hf-col {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.jc-hf-addr {
  gap: 20px;
}
.jc-hf-addr-item { display: flex; flex-direction: column; gap: 4px; }
.jc-hf-h {
  font-size: 14px;
  font-weight: 700;
  color: white;
  margin: 0;
}
.jc-hf-p {
  font-size: 12px;
  color: #999;
  line-height: 1.5;
  margin: 0;
}
.jc-hf-link {
  font-size: 13px;
  color: #999;
  text-decoration: none;
  transition: color 0.18s ease;
  cursor: pointer;
}
.jc-hf-link:hover { color: #9bff00; }
.jc-hf-bottom {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: #666;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  padding-top: 24px;
}
.jc-hf-bottom p { margin: 0; }

/* Dark 模式反色：浅底 + 深字 —— 与首页桌面 footer 深色反色规则一致 */
.jc-hf.jc-hf-dark {
  background: #f5f5f3;
  color: #212121;
}
.jc-hf.jc-hf-dark.jc-hf-full-bleed::after { background: #f5f5f3; }
.jc-hf.jc-hf-dark .jc-hf-logo { filter: brightness(0); }
.jc-hf.jc-hf-dark .jc-hf-h { color: #212121; }
.jc-hf.jc-hf-dark .jc-hf-p,
.jc-hf.jc-hf-dark .jc-hf-link,
.jc-hf.jc-hf-dark .jc-hf-bottom { color: #6a6a6a; }
.jc-hf.jc-hf-dark .jc-hf-bottom {
  border-top-color: rgba(0, 0, 0, 0.08);
}
.jc-hf.jc-hf-dark .jc-hf-link:hover { color: #4a8c00; }

@media (max-width: 768px) {
  .jc-hf { padding: 40px 20px 24px; }
  .jc-hf-inner { gap: 28px; }
  /* 移动端保持 3.6:1 比例：显式指定 width，让 aspect-ratio 反算 height */
  .jc-hf-logo { width: 144px; height: auto; align-self: center; }
  .jc-hf-cols {
    grid-template-columns: 1fr;
    gap: 24px;
  }
  .jc-hf-bottom {
    flex-direction: column;
    gap: 8px;
    text-align: center;
    font-size: 11px;
  }
}
</style>
