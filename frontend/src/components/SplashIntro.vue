<script setup>
import { onMounted, onUnmounted } from 'vue'

const readyDelay = 1900
let readyTimer = null

const startIntro = () => {
  const prefersReduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  document.body.classList.remove('is-ready')
  window.clearTimeout(readyTimer)
  readyTimer = window.setTimeout(() => {
    document.body.classList.add('is-ready')
  }, prefersReduced ? 0 : readyDelay)
}

onMounted(() => {
  startIntro()
})

onUnmounted(() => {
  window.clearTimeout(readyTimer)
  document.body.classList.add('is-ready')
})
</script>

<template>
  <div class="splash-intro" aria-hidden="true">
    <div class="splash-intro__logo-stack">
      <!-- 路由切换时由 router 用 WAAPI 控制 opacity/scale 来做光晕 -->
      <div class="splash-intro__halo" aria-hidden="true"></div>
      <div class="splash-intro__mark">
        <svg viewBox="0 0 400 400" role="img" aria-label="JoinCare mark">
          <circle class="mark-bg" cx="200" cy="200" r="200" />
          <g class="mark-network">
            <path class="mark-stem" d="M186.106 217.888C155.56 256.343 140.832 305.971 139.235 354.583C139.235 361.621 139.526 368.659 140.106 375.769C140.541 381.066 141.049 386.29 141.702 391.587C132.052 388.684 122.765 384.984 113.841 380.703C113.913 379.034 113.913 377.293 114.058 375.624C114.349 373.012 114.711 369.167 115.002 365.031C115.655 356.615 116.453 346.747 117.686 341.668C119.5 331.22 121.749 318.523 125.16 308.51C129.223 294.507 135.245 279.56 142.138 266.718C152.803 247.055 165.863 228.626 181.898 213.39L184.075 212.446L187.63 216.147L186.179 217.888H186.106Z" />
            <line class="branch branch--1" x1="200" y1="200" x2="200" y2="77" />
            <line class="branch branch--2" x1="200" y1="200" x2="270" y2="99" />
            <line class="branch branch--3" x1="200" y1="200" x2="316" y2="158" />
            <line class="branch branch--4" x1="200" y1="200" x2="319" y2="232" />
            <line class="branch branch--5" x1="200" y1="200" x2="279" y2="294" />
            <line class="branch branch--6" x1="200" y1="200" x2="211" y2="323" />
            <line class="branch branch--7" x1="200" y1="200" x2="129" y2="99" />
            <line class="branch branch--8" x1="200" y1="200" x2="84" y2="158" />
            <line class="branch branch--9" x1="200" y1="200" x2="81" y2="232" />
            <circle class="node node--0" cx="200" cy="200" r="22.5" />
            <circle class="node node--1" cx="200" cy="77" r="14.5" />
            <circle class="node node--2" cx="270" cy="99" r="14.5" />
            <circle class="node node--3" cx="316" cy="158" r="14.5" />
            <circle class="node node--4" cx="319" cy="232" r="14.5" />
            <circle class="node node--5" cx="279" cy="294" r="14.5" />
            <circle class="node node--6" cx="211" cy="323" r="14.5" />
            <circle class="node node--7" cx="129" cy="99" r="14.5" />
            <circle class="node node--8" cx="84" cy="158" r="14.5" />
            <circle class="node node--9" cx="81" cy="232" r="14.5" />
          </g>
        </svg>
      </div>
      <div class="splash-intro__wordmark" aria-label="JoinCare">
        <img src="/brand/joincare-logo-vertical-green-black.svg" alt="" />
      </div>
    </div>
  </div>
</template>

<style scoped>
:global(.joincare-app-shell) {
  opacity: 0;
  transform: translateY(16px);
  filter: blur(10px);
  transition: opacity 0.8s var(--splash-ease), transform 0.8s var(--splash-ease), filter 0.8s var(--splash-ease);
  animation: app-shell-auto-show 0.8s var(--splash-ease) 1.9s forwards;
}

:global(body.is-ready .joincare-app-shell) {
  opacity: 1;
  transform: none;
  filter: none;
  visibility: visible;
  pointer-events: auto;
}

:global(:root) {
  --splash-green: #9bff00;
  --splash-ink: #050b00;
  --splash-ease: cubic-bezier(.2, .85, .2, 1);
  --splash-logo-width: 127px;
  --splash-mark-size: calc(var(--splash-logo-width) * 0.9514);
  --splash-logo-gap: calc(var(--splash-logo-width) * 0.1424);
  --splash-word-height: calc(var(--splash-logo-width) * 0.1806);
}

.splash-intro {
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: grid;
  place-items: center;
  background: #222;                    /* footer 风格深底 */
  overflow: hidden;
  transform: translateY(0);
  will-change: transform;
  /* 仅用 transition + body.is-ready 触发，避免 animation 冲突 */
  transition: transform 0.85s cubic-bezier(0.76, 0, 0.24, 1);
}

/* === 暗色主题：splash 翻成浅底 + 黑色 logo ===
   注意：必须把整个选择器都放进 :global() 里。Vue 的 scoped parser 对
   `:global(A) B` 写法处理不当 —— 会把后面的 B 丢掉，只剩 `A { ... }`，
   结果 filter / background 直接打到 <html>，把整页变成白屏。 */
:global(html.home-dark-root .splash-intro) {
  background: #f5f5f3;
}
:global(html.home-dark-root .splash-intro__wordmark img) {
  filter: brightness(0) !important;   /* 黑色 wordmark（覆盖默认 invert(1)） */
}
:global(html.home-dark-root .splash-intro::before) {
  opacity: 0.06;
}
/* 路由切换：由 router 用 Web Animations API 直接驱动 splash 元素。
   这里只关闭 CSS transition，避免和 WAAPI 抢"transform"通道。 */
:global(body.is-navigating) .splash-intro {
  transition: none !important;
  pointer-events: auto;
}
/* 噪点底纹叠加层 */
.splash-intro::before {
  content: '';
  position: absolute;
  inset: 0;
  background-image: url('/figma/v2/noise-bg.png');
  background-size: 500px 500px;
  background-position: top left;
  opacity: 0.10;
  pointer-events: none;
}

:global(body.is-ready .splash-intro) {
  transform: translateY(-100%);
  pointer-events: none;
}

/* 文字 logo 在深底上要反白 */
.splash-intro__wordmark img {
  filter: brightness(0) invert(1);
}

.splash-intro__logo-stack {
  width: var(--splash-logo-width);
  display: grid;
  justify-items: center;
  transform: translateY(-4px);
  position: relative;
}

/* 路由切换时的光晕（默认不可见，停留阶段由 WAAPI 拉起 opacity/scale） */
.splash-intro__halo {
  position: absolute;
  left: 50%;
  /* logo-stack 高 ≈ mark(120) + gap(18) + word(23) = 161；
     mark 中心在 60px，stack 中心在 80px，差 ~20px — 把 halo 中心上移 22px 对准圆形 mark */
  top: calc(50% - 22px);
  width: 220px;
  height: 220px;
  border-radius: 50%;
  background: radial-gradient(closest-side,
    rgba(155, 255, 0, 0.85) 0%,
    rgba(155, 255, 0, 0.40) 35%,
    rgba(155, 255, 0, 0.10) 65%,
    transparent 100%);
  filter: blur(24px);
  opacity: 0;
  transform: translate(-50%, -50%) scale(0.55);
  pointer-events: none;
  z-index: 0;
  will-change: opacity, transform;
}
.splash-intro__mark,
.splash-intro__wordmark {
  position: relative;
  z-index: 1;     /* logo 浮在光晕之上 */
}

.splash-intro__mark {
  width: var(--splash-mark-size);
  aspect-ratio: 1;
  animation: mark-settle 1.8s var(--splash-ease) forwards;
}

.splash-intro__mark svg {
  width: 100%;
  height: 100%;
  overflow: visible;
  filter: drop-shadow(0 22px 55px rgba(155, 255, 0, .22));
}

.mark-bg {
  fill: var(--splash-green);
  transform-box: fill-box;
  transform-origin: center;
  animation: bg-pop .8s var(--splash-ease) forwards, bg-pulse 1.2s ease-in-out .62s forwards;
}

.mark-stem {
  fill: var(--splash-ink);
  opacity: 0;
  transform-box: fill-box;
  transform-origin: top center;
  animation: stem-in .48s var(--splash-ease) .42s forwards;
}

.branch {
  stroke: var(--splash-ink);
  stroke-width: 7;
  stroke-linecap: round;
  stroke-dasharray: 160;
  stroke-dashoffset: 160;
  transform-origin: 200px 200px;
  animation: draw-line .62s var(--splash-ease) forwards;
}

.branch--1 { animation-delay: .36s; }
.branch--2 { animation-delay: .43s; }
.branch--3 { animation-delay: .50s; }
.branch--4 { animation-delay: .57s; }
.branch--5 { animation-delay: .64s; }
.branch--6 { animation-delay: .71s; }
.branch--7 { animation-delay: .48s; }
.branch--8 { animation-delay: .55s; }
.branch--9 { animation-delay: .62s; }

.node {
  fill: var(--splash-ink);
  transform-box: fill-box;
  transform-origin: center;
  opacity: 0;
  animation: node-in .48s var(--splash-ease) forwards;
}

.node--0 { animation-delay: .24s; }
.node--1 { animation-delay: .72s; }
.node--2 { animation-delay: .79s; }
.node--3 { animation-delay: .86s; }
.node--4 { animation-delay: .93s; }
.node--5 { animation-delay: 1s; }
.node--6 { animation-delay: 1.07s; }
.node--7 { animation-delay: .84s; }
.node--8 { animation-delay: .91s; }
.node--9 { animation-delay: .98s; }

.splash-intro__wordmark {
  width: var(--splash-logo-width);
  height: var(--splash-word-height);
  margin-top: var(--splash-logo-gap);
  overflow: hidden;
  opacity: 0;
  transform: translateY(10px);
  animation: word-in .55s var(--splash-ease) 1.04s forwards;
}

.splash-intro__wordmark img {
  display: block;
  width: var(--splash-logo-width);
  height: auto;
  transform: translateY(-85.831%);
}

@keyframes bg-pop {
  from { opacity: 0; transform: scale(.62); }
  to { opacity: 1; transform: scale(1); }
}

@keyframes bg-pulse {
  0% { transform: scale(1); }
  45% { transform: scale(1.035); }
  100% { transform: scale(1); }
}

@keyframes draw-line {
  to { stroke-dashoffset: 0; }
}

@keyframes node-in {
  from { opacity: 0; transform: scale(.35); }
  to { opacity: 1; transform: scale(1); }
}

@keyframes stem-in {
  from { opacity: 0; transform: translateY(14px) scaleY(.72); }
  to { opacity: 1; transform: none; }
}

@keyframes word-in {
  to { opacity: 1; transform: none; }
}

@keyframes mark-settle {
  0% { transform: translateY(-8px) scale(.92); }
  65% { transform: translateY(0) scale(1); }
  100% { transform: translateY(0) scale(1); }
}

@keyframes app-shell-auto-show {
  to {
    opacity: 1;
    transform: none;
    filter: none;
    visibility: visible;
    pointer-events: auto;
  }
}

@keyframes splash-auto-hide {
  to {
    transform: translateY(-100%);
    pointer-events: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .splash-intro {
    display: none;
  }

  :global(.joincare-app-shell) {
    opacity: 1;
    transform: none;
    filter: none;
  }
}
</style>
