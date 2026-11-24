<script setup>
import { computed } from 'vue'

const props = defineProps({ language: { type: String, default: 'en' } })

const t = computed(() =>
  props.language === 'zh'
    ? {
        tagline: '\u00a9 2024 JoinCare DeSci Network. \u70ba\u4eba\u985e\u672a\u4f86\u7684\u7cbe\u6e96\u5b75\u5316.',
        links: [
          { label: 'Privacy Policy', to: '/privacy', external: false },
          { label: 'Terms of Service', to: '/terms', external: false },
          { label: 'Research Ethics', to: '/ethics', external: false },
          { label: 'X / Twitter', to: 'https://x.com/TheJoinCare', external: true },
        ],
      }
    : {
        tagline: '\u00a9 2024 JoinCare DeSci Network. Precision Incubation for the Future of Humanity.',
        links: [
          { label: 'Privacy Policy', to: '/privacy', external: false },
          { label: 'Terms of Service', to: '/terms', external: false },
          { label: 'Research Ethics', to: '/ethics', external: false },
          { label: 'X / Twitter', to: 'https://x.com/TheJoinCare', external: true },
        ],
      },
)
</script>

<template>
  <footer class="jc-global-footer w-full flat">
    <div class="jc-global-footer-panel mx-auto max-w-container-max py-stack-lg">
      <div class="jc-global-footer-noise" aria-hidden="true"></div>
      <div class="relative z-[1] grid grid-cols-1 md:grid-cols-3 gap-gutter px-margin-mobile md:px-margin-desktop">
        <div class="flex flex-col gap-4">
          <img src="/figma/v2/footer-logo.svg" alt="JoinCare" class="jc-global-footer-logo" />
          <p class="jc-global-footer-text font-body-sm text-body-sm max-w-sm">{{ t.tagline }}</p>
        </div>
        <div class="flex flex-col gap-2 md:col-span-2 md:flex-row md:justify-end md:gap-8 items-start md:items-center">
          <a
            v-for="l in t.links"
            :key="l.label"
            :href="l.to"
            :target="l.external ? '_blank' : undefined"
            :rel="l.external ? 'noopener' : undefined"
            class="jc-global-footer-link transition-colors hover:opacity-80 font-label-caps text-label-caps flex items-center gap-1"
          >
            {{ l.label }}
            <span v-if="l.external" class="material-symbols-outlined text-sm">open_in_new</span>
          </a>
        </div>
      </div>
    </div>
  </footer>
</template>

<style scoped>
.jc-global-footer {
  background: transparent;
}
.jc-global-footer-panel {
  position: relative;
  isolation: isolate;
  overflow: hidden;
  background: #222;
  color: #fff;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  transition:
    background-color 0.35s ease,
    color 0.35s ease,
    border-color 0.35s ease;
}
.jc-global-footer-noise {
  position: absolute;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  background-image: url('/figma/v2/noise-bg.png');
  background-size: 500px 500px;
  background-position: top left;
  opacity: 0.12;
}
.jc-global-footer-logo {
  width: 160px;
  height: auto;
  aspect-ratio: 399.638 / 111;
  display: block;
}
.jc-global-footer-text,
.jc-global-footer-link {
  color: #999;
}
.jc-global-footer-link:hover {
  color: #9bff00;
}
.material-symbols-outlined {
  font-variation-settings: 'FILL' 0, 'wght' 400, 'GRAD' 0, 'opsz' 24;
}
:global(.dark) .jc-global-footer-panel {
  background: #f5f5f3;
  color: #212121;
  border-top-color: rgba(0, 0, 0, 0.08);
}
:global(.dark) .jc-global-footer-logo {
  filter: brightness(0);
}
:global(.dark) .jc-global-footer-text,
:global(.dark) .jc-global-footer-link {
  color: #6a6a6a;
}
:global(.dark) .jc-global-footer-link:hover {
  color: #4a8c00;
}
@media (max-width: 768px) {
  .jc-global-footer-logo {
    width: 144px;
  }
}
</style>
