<script setup>
/**
 * 系统公告 popup + 顶部横幅 banner
 *   - 进入页面时拉 /client/announcement/v1/current
 *   - popup 用 localStorage 'jc_ann_dismissed_<id>' 记关闭，点关闭后不再弹
 *   - banner 持续展示直到 end_at / is_enable=0
 *   - 内容后端 sanitize.RichHTML 清洗 + 前端 DOMPurify 二次消毒后 v-html 输出
 */
import { ref, onMounted, watch } from 'vue'
import { getCurrentAnnouncement } from '../services/api'
import { sanitize } from '../utils/sanitize'

const props = defineProps({
  language: { type: String, default: 'en' }
})

const popup = ref(null)
const banner = ref(null)
const popupVisible = ref(false)

const DISMISS_KEY = (id) => `jc_ann_dismissed_${id}`

async function load() {
  try {
    const data = await getCurrentAnnouncement({ language: props.language || 'en' })
    popup.value = data?.popup || null
    banner.value = data?.banner || null
    if (popup.value && popup.value.id) {
      const dismissed = localStorage.getItem(DISMISS_KEY(popup.value.id))
      popupVisible.value = !dismissed
    } else {
      popupVisible.value = false
    }
  } catch (e) {
    console.warn('[Announcement] load failed', e?.message)
  }
}

function closePopup() {
  if (popup.value?.id) {
    try { localStorage.setItem(DISMISS_KEY(popup.value.id), '1') } catch (_) { /* private mode */ }
  }
  popupVisible.value = false
}

onMounted(load)
watch(() => props.language, load)
</script>

<template>
  <!-- 顶部横幅：始终显示（按 admin 配置），不可关闭 -->
  <div v-if="banner" class="jc-ann-banner" role="status">
    <span class="jc-ann-banner-title">{{ banner.title }}</span>
    <span v-if="banner.content" class="jc-ann-banner-body" v-html="sanitize(banner.content)"></span>
  </div>

  <!-- 弹窗：进入页面弹一次，关闭后不再弹（localStorage 持久化） -->
  <div v-if="popup && popupVisible" class="jc-ann-popup-mask" @click.self="closePopup">
    <div class="jc-ann-popup-dialog" role="dialog" aria-modal="true">
      <button class="jc-ann-popup-close" @click="closePopup" aria-label="Close">✕</button>
      <h3 class="jc-ann-popup-title">{{ popup.title }}</h3>
      <div class="jc-ann-popup-body" v-html="sanitize(popup.content)"></div>
      <button class="jc-ann-popup-cta" @click="closePopup">OK</button>
    </div>
  </div>
</template>

<style scoped>
.jc-ann-banner {
  position: relative;
  z-index: 90;
  padding: 10px 24px;
  background: rgba(155, 255, 0, 0.10);
  border-bottom: 1px solid rgba(155, 255, 0, 0.32);
  color: rgba(255,255,255,0.84);
  text-align: center;
  font-size: 13px;
}
.jc-ann-banner-title {
  font-weight: 600;
  margin-right: 6px;
}
.jc-ann-banner-body { opacity: 0.78; }
.jc-ann-banner-body :deep(a) { color: #9bff00; text-decoration: underline; }

.jc-ann-popup-mask {
  position: fixed; inset: 0; z-index: 9000;
  background: rgba(0, 0, 0, 0.62);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  display: flex; align-items: center; justify-content: center;
  animation: jcAnnFadeIn .25s ease;
}
@keyframes jcAnnFadeIn { from { opacity: 0; } to { opacity: 1; } }
.jc-ann-popup-dialog {
  position: relative;
  width: min(560px, calc(100vw - 32px));
  max-height: 80vh; overflow-y: auto;
  background: #161a1f; color: #e7ecf3;
  border: 1px solid rgba(155, 255, 0, 0.22);
  border-radius: 16px;
  padding: 28px 28px 22px;
  box-shadow: 0 24px 64px rgba(0,0,0,0.45);
}
.jc-ann-popup-close {
  position: absolute; top: 12px; right: 14px;
  background: transparent; border: 0; color: rgba(255,255,255,0.5);
  font-size: 18px; cursor: pointer; padding: 4px 6px;
}
.jc-ann-popup-close:hover { color: #fff; }
.jc-ann-popup-title {
  font-size: 22px; font-weight: 700; margin: 0 0 14px 0;
  color: #9bff00; padding-right: 30px;
}
.jc-ann-popup-body {
  font-size: 14px; line-height: 1.65;
  color: rgba(231, 236, 243, 0.86);
}
.jc-ann-popup-body :deep(p) { margin: 0 0 10px 0; }
.jc-ann-popup-body :deep(a) { color: #9bff00; text-decoration: underline; }
.jc-ann-popup-body :deep(ul), .jc-ann-popup-body :deep(ol) { padding-left: 22px; margin: 8px 0; }
.jc-ann-popup-cta {
  margin-top: 18px;
  padding: 10px 22px; border: 0; border-radius: 8px;
  background: #9bff00; color: #0f2f1a;
  font-weight: 600; cursor: pointer;
  transition: filter .15s ease;
}
.jc-ann-popup-cta:hover { filter: brightness(1.08); }

/* 浅色模式 */
:global(html:not(.home-dark-root)) .jc-ann-banner {
  background: rgba(15, 47, 26, 0.04);
  border-bottom-color: rgba(15, 47, 26, 0.14);
  color: rgba(15, 47, 26, 0.78);
}
:global(html:not(.home-dark-root)) .jc-ann-popup-dialog {
  background: #ffffff; color: #1b2330;
  border-color: rgba(15, 47, 26, 0.16);
}
:global(html:not(.home-dark-root)) .jc-ann-popup-title { color: #1aa64a; }
:global(html:not(.home-dark-root)) .jc-ann-popup-body { color: rgba(27,35,48,0.84); }
:global(html:not(.home-dark-root)) .jc-ann-popup-close { color: rgba(27,35,48,0.5); }
:global(html:not(.home-dark-root)) .jc-ann-popup-close:hover { color: #1b2330; }
</style>
