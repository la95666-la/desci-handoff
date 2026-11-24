<script setup>
import { ref, onMounted, onBeforeUnmount, inject, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useWalletStore } from '../stores/wallet'
import { TRANSLATIONS } from '../i18n/navbar.js'
import { useIsMobile } from '../composables/useIsMobile'
import { SHOW_TASKS_ENTRY } from '../config/featureVisibility.js'
const { isMobile } = useIsMobile()
const mobileDrawer = ref(false)
const closeDrawer = () => (mobileDrawer.value = false)

// 全局语言状态（App.vue 提供）
const language = inject('language', ref('en'))
const toggleLanguage = inject('toggleLanguage', () => {})
const t = computed(() => TRANSLATIONS[language.value] || TRANSLATIONS.en)

const props = defineProps({
  scale: { type: Number, default: 1 },
  // 当前激活项：'home' | 'ecosystem' | 'launchpad' | 'quest' | 'docs'
  active: { type: String, default: '' },
})

// Navbar 自己的缩放下限：主画布可以缩到 0.35，但导航条到 0.7 就 stop —— 再小菜单项会挤爆换行。
// 这样窄屏下 navbar 视觉上"比主内容大"一点，但比换行体验好。
const NAVBAR_MIN_SCALE = 0.7
const effectiveScale = computed(() => Math.max(props.scale, NAVBAR_MIN_SCALE))

const router = useRouter()
const wallet = useWalletStore()

const A = '/figma/v2'
const img = (n) => `${A}/${n}`

const goHome        = () => router.push('/')
const goEcosystem   = () => router.push('/ecosystem')
const goLaunchpad   = () => router.push('/launchpad')
const goQuestCenter = () => router.push('/tasks')
const goDashboard   = () => {
  showMenu.value = false
  router.push('/dashboard')
}
const goDocs        = () => window.open('https://joincare.gitbook.io/joincare-docs', '_blank', 'noopener')

const cls = (k) =>
  k === props.active
    ? 'px-[11px] text-[13px] font-bold text-[#212121] cursor-pointer'
    : 'px-[11px] text-[13px] font-medium text-[#212121] cursor-pointer'

// === 钱包 ===
// 未登录：点击触发 wallet.connectWallet()（弹出全局 WalletSelector 模态）
// 已登录：显示地址胶囊，点击展开小菜单 → Disconnect
const showMenu = ref(false)
const menuRoot = ref(null)

const handleConnect = async () => {
  try { await wallet.connectWallet() } catch (e) { console.warn('[connect]', e) }
}
const handleDisconnect = () => {
  showMenu.value = false
  try { wallet.disconnect() } catch (e) { console.warn('[disconnect]', e) }
}
const onDocClick = (e) => {
  if (menuRoot.value && !menuRoot.value.contains(e.target)) showMenu.value = false
}
onMounted(()  => document.addEventListener('click', onDocClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocClick))
</script>

<template>
  <!-- === 移动版 navbar：左上角单浮动按钮 === -->
  <button
    v-if="isMobile"
    class="mobile-hamburger-fab"
    @click="mobileDrawer = true"
    aria-label="menu">
    <span></span><span></span><span></span>
  </button>

  <!-- Drawer overlay -->
  <transition name="drawer-fade">
    <div v-if="isMobile && mobileDrawer" class="mobile-drawer-overlay" @click.self="closeDrawer">
      <transition name="drawer-slide" appear>
        <aside v-if="mobileDrawer" class="mobile-drawer">
          <div class="drawer-header">
            <img :src="img('nav-logo.svg')" alt="JoinCare" class="drawer-logo" />
            <button class="drawer-close" @click="closeDrawer" aria-label="close">×</button>
          </div>
          <nav class="drawer-nav">
            <a :class="['drawer-link', props.active === 'home' && 'is-active']" @click="goHome(); closeDrawer()">{{ t.home }}</a>
            <a :class="['drawer-link', props.active === 'ecosystem' && 'is-active']" @click="goEcosystem(); closeDrawer()">{{ t.ecosystem }}</a>
            <a :class="['drawer-link', props.active === 'launchpad' && 'is-active']" @click="goLaunchpad(); closeDrawer()">{{ t.launchpad }}</a>
            <a v-if="SHOW_TASKS_ENTRY" :class="['drawer-link', props.active === 'quest' && 'is-active']" @click="goQuestCenter(); closeDrawer()">{{ t.quest }}</a>
            <a :class="['drawer-link', props.active === 'docs' && 'is-active']" @click="goDocs(); closeDrawer()">{{ t.docs }}</a>
          </nav>
          <div class="drawer-footer">
            <!-- 钱包区：满宽（未连接=连接按钮，已连接=地址胶囊+断开） -->
            <button v-if="!wallet.isConnected"
                    class="drawer-connect"
                    @click="handleConnect(); closeDrawer()">
              {{ t.connectWallet }}
            </button>
            <div v-else class="drawer-wallet-block">
              <div class="drawer-wallet-addr">
                <span class="wallet-dot"></span>
                <span class="drawer-wallet-addr-text">{{ wallet.shortAddress }}</span>
              </div>
              <button class="drawer-wallet-dashboard" @click="goDashboard(); closeDrawer()">
                {{ t.dashboard }}
              </button>
              <button class="drawer-wallet-disconnect" @click="handleDisconnect(); closeDrawer()">
                {{ t.disconnect }}
              </button>
            </div>
            <!-- 语言切换：底部小胶囊，独立成行 -->
            <div class="drawer-lang-row">
              <button class="drawer-lang-pill"
                      :class="{ 'is-on': language !== 'tw' }"
                      @click="language !== 'tw' || toggleLanguage()">EN</button>
              <button class="drawer-lang-pill"
                      :class="{ 'is-on': language === 'tw' }"
                      @click="language === 'tw' || toggleLanguage()">繁體中文</button>
            </div>
          </div>
        </aside>
      </transition>
    </div>
  </transition>

  <!-- === 桌面版 (isMobile=false) === -->
  <header
    v-if="!isMobile"
    class="floating-navbar"
    :style="{ transform: `translateX(-50%) scale(${effectiveScale})` }"
  >
    <div class="bg-white flex items-center justify-center gap-[4px] pl-[24px] pr-[6px] py-[5px] rounded-[7px] shadow-[0px_8px_24px_0px_rgba(0,0,0,0.06)]">
      <img :src="img('nav-logo.svg')" alt="JoinCare" class="h-[28px] w-[122px] cursor-pointer" @click="goHome" />
      <a :class="['ml-[64px]', cls('ecosystem')]" @click="goEcosystem">{{ t.ecosystem }}</a>
      <a :class="cls('launchpad')" @click="goLaunchpad">{{ t.launchpad }}</a>
      <a v-if="SHOW_TASKS_ENTRY" :class="cls('quest')" @click="goQuestCenter">{{ t.quest }}</a>
      <a :class="cls('docs')" @click="goDocs">{{ t.docs }}</a>

      <div class="flex items-center gap-[6px] pl-[10px] pr-[4px] py-[4px]">
        <!-- 语言切换：EN ↔ TW，胶囊圈替代原地球 icon -->
        <button
          class="lang-toggle w-[44px] h-[32px] rounded-[16px] bg-[#f0efec] flex items-center justify-center cursor-pointer"
          @click="toggleLanguage"
          :aria-label="`Switch language (current: ${language === 'tw' ? 'TW' : 'EN'})`"
          :title="language === 'tw' ? '繁體中文 → English' : 'English → 繁體中文'"
        >
          <span class="text-[11px] font-bold tracking-[0.5px] text-[#212121]">
            {{ language === 'tw' ? 'EN' : 'TW' }}
          </span>
        </button>

        <!-- 未登录：原浪涌动效绿胶囊 → 触发 wallet 选择器 -->
        <button v-if="!wallet.isConnected" class="connect-wallet-btn" @click="handleConnect">
          <span class="cw-deco cw-wave-wrap">
            <span class="cw-wave">
              <img :src="img('wave-back.svg')" alt="" />
            </span>
          </span>
          <span class="cw-deco cw-crest cw-crest-r">
            <img :src="img('crest-r.svg')" alt="" />
          </span>
          <span class="cw-deco cw-crest cw-crest-l">
            <img :src="img('crest-r.svg')" alt="" />
          </span>
          <span class="cw-label">{{ t.connectWallet }}</span>
        </button>

        <!-- 已登录：地址胶囊 + 下拉菜单（Disconnect） -->
        <div v-else ref="menuRoot" class="wallet-menu-root">
          <button class="wallet-pill" @click.stop="showMenu = !showMenu">
            <span class="wallet-dot"></span>
            <span class="wallet-addr">{{ wallet.shortAddress }}</span>
            <span class="material-symbols-outlined wallet-caret"
                  :class="{ 'rotate-180': showMenu }">keyboard_arrow_down</span>
          </button>
          <div v-if="showMenu" class="wallet-menu">
            <button class="wallet-menu-item" @click="goDashboard">
              <svg class="wallet-menu-svg" viewBox="0 0 18 18" fill="none" aria-hidden="true">
                <rect x="2.25" y="2.25" width="5.5" height="5.5" rx="1.25" stroke="currentColor" stroke-width="1.5" />
                <rect x="10.25" y="2.25" width="5.5" height="5.5" rx="1.25" stroke="currentColor" stroke-width="1.5" />
                <rect x="2.25" y="10.25" width="5.5" height="5.5" rx="1.25" stroke="currentColor" stroke-width="1.5" />
                <rect x="10.25" y="10.25" width="5.5" height="5.5" rx="1.25" stroke="currentColor" stroke-width="1.5" />
              </svg>
              {{ t.dashboard }}
            </button>
            <button class="wallet-menu-item" @click="handleDisconnect">
              <span class="material-symbols-outlined text-[16px]">logout</span>
              {{ t.disconnect }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </header>
</template>

<style scoped>
.floating-navbar {
  position: fixed;
  top: 24px;
  left: 50%;
  z-index: 100;
  transform-origin: top center;
  transform: translateX(-50%) scale(1);
  /* 视口比 navbar 内容窄时，让胶囊自然溢出屏幕两侧（左右各多几十像素），
     而不是被压缩到内容突破胶囊背景。 */
  white-space: nowrap;
}
/* 胶囊背景 = header 唯一 direct child，锁定为 max-content，跟随内部菜单宽度 */
.floating-navbar > div {
  min-width: max-content;
  flex-wrap: nowrap;
}
.floating-navbar a,
.floating-navbar button,
.floating-navbar img {
  flex-shrink: 0;
}

.connect-wallet-btn {
  position: relative;
  width: 130px;
  height: 32px;
  background: #9bff00;
  border: none;
  border-radius: 18px;
  box-shadow: 0px 2px 8px 0px rgba(0,0,0,0.1);
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: box-shadow 0.4s ease, transform 0.4s ease;
}
.connect-wallet-btn:hover {
  box-shadow: 0 8px 24px rgba(155, 255, 0, 0.55);
  transform: translateY(-1px);
}
.cw-deco {
  position: absolute;
  display: block;
  pointer-events: none;
  transition: transform 0.6s cubic-bezier(0.65, 0, 0.35, 1);
}
.cw-deco img { width: 100%; height: 100%; display: block; }
.cw-wave-wrap {
  width: 225.388px;
  height: 118.576px;
  left: -45.97px;
  top: -111px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.cw-wave {
  width: 217.5px;
  height: 100px;
  transform: rotate(5deg);
}
.cw-crest {
  width: 101.5px;
  height: 56px;
}
.cw-crest-r { left: 64.25px;  top: -97px; }
.cw-crest-l { left: -22.75px; top: -63px; }
.connect-wallet-btn:hover .cw-wave-wrap { transform: translateY(72px); }
.connect-wallet-btn:hover .cw-crest-r  { transform: translateY(58px); }
.connect-wallet-btn:hover .cw-crest-l  { transform: translateY(42px); }
.cw-label {
  position: relative;
  z-index: 2;
  font-size: 14px;
  color: #0b1020;
  font-family: 'Inter', sans-serif;
  font-weight: 400;
  transition: color 0.45s ease;
}
.connect-wallet-btn:hover .cw-label { color: #9bff00; }

/* === 已连接：钱包地址胶囊 + 下拉菜单 === */
.wallet-menu-root {
  position: relative;
}
.wallet-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 32px;
  padding: 0 10px 0 12px;
  background: #212121;
  border: none;
  border-radius: 18px;
  box-shadow: 0px 2px 8px 0px rgba(0,0,0,0.10);
  cursor: pointer;
  transition: background 0.3s ease, box-shadow 0.3s ease;
  color: #fff;
  font-family: 'Inter', sans-serif;
}
.wallet-pill:hover {
  background: #000;
  box-shadow: 0 4px 14px rgba(0,0,0,0.20);
}
.wallet-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #9bff00;
  box-shadow: 0 0 0 3px rgba(155, 255, 0, 0.25);
  display: inline-block;
}
.wallet-addr {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.3px;
  font-variant-numeric: tabular-nums;
}
.wallet-caret {
  font-size: 16px !important;
  color: #999;
  transition: transform 0.2s ease;
}
.wallet-caret.rotate-180 {
  transform: rotate(180deg);
}
.wallet-menu {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  min-width: 160px;
  background: #fff;
  border-radius: 10px;
  box-shadow: 0 12px 32px rgba(0,0,0,0.12), 0 2px 6px rgba(0,0,0,0.06);
  padding: 6px;
  z-index: 120;
  animation: wallet-menu-in 0.18s cubic-bezier(0.22, 1, 0.36, 1) both;
}
@keyframes wallet-menu-in {
  from { opacity: 0; transform: translateY(-4px); }
  to   { opacity: 1; transform: translateY(0); }
}
.wallet-menu-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 8px 12px;
  background: transparent;
  border: none;
  border-radius: 7px;
  font-size: 13px;
  font-weight: 600;
  color: #212121;
  cursor: pointer;
  transition: background 0.15s ease;
}
.wallet-menu-item:hover {
  background: #f5f5f3;
}
.wallet-menu-svg {
  width: 16px;
  height: 16px;
  flex: 0 0 16px;
}
.material-symbols-outlined {
  font-family: 'Material Symbols Outlined', sans-serif;
  font-variation-settings: 'FILL' 0, 'wght' 400, 'GRAD' 0, 'opsz' 24;
}

/* 语言切换胶囊 */
.lang-toggle {
  border: none;
  transition: background-color 0.18s ease, transform 0.18s ease;
}
.lang-toggle:hover {
  background-color: #e6e5e1;
  transform: scale(1.05);
}

/* ================================================
   MOBILE NAVBAR + DRAWER
   ================================================ */
/* 左上角浮动圆形汉堡按钮（唯一移动 nav 入口） */
.mobile-hamburger-fab {
  position: fixed;
  top: 16px;
  left: 16px;
  width: 44px;
  height: 44px;
  border: none;
  background: white;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 4px;
  cursor: pointer;
  z-index: 100;
  box-shadow: 0 6px 20px rgba(0,0,0,0.15);
}
.mobile-hamburger-fab span {
  display: block;
  width: 18px;
  height: 2px;
  background: #212121;
  border-radius: 1px;
}
/* dark 模式下 FAB 反色 */
.home-dark .mobile-hamburger-fab,
body.dark-mode .mobile-hamburger-fab {
  background: #212121;
}
.home-dark .mobile-hamburger-fab span,
body.dark-mode .mobile-hamburger-fab span {
  background: white;
}
/* Drawer footer 中的钱包按钮 */
.drawer-connect {
  height: 44px;
  padding: 0 18px;
  border-radius: 999px;
  background: #9bff00;
  border: none;
  font-size: 14px;
  font-weight: 700;
  color: #212121;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(155, 255, 0, 0.35);
  width: 100%;
  margin-bottom: 12px;
}
.drawer-wallet-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 44px;
  padding: 0 18px;
  border-radius: 999px;
  background: #212121;
  border: none;
  cursor: pointer;
  color: white;
  font-size: 13px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  width: 100%;
  margin-bottom: 12px;
}
.drawer-wallet-pill .wallet-dot {
  width: 8px; height: 8px; border-radius: 50%; background: #9bff00;
  box-shadow: 0 0 0 2px rgba(155,255,0,0.25);
}

/* Drawer overlay + panel */
.mobile-drawer-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.4);
  backdrop-filter: blur(4px);
  z-index: 200;
  display: flex;
  justify-content: flex-end;
}
.mobile-drawer {
  width: 80%;
  max-width: 340px;
  height: 100vh;
  background: white;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 20px;
  box-shadow: -12px 0 32px rgba(0,0,0,0.15);
}
.drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.drawer-logo {
  /* nav-logo.svg width="100%" height="100%" 无固有尺寸，img 会被拉成 300×150。
     按 viewBox 157.5×36 (≈ 4.375:1) 固定 width，height 自适应，比例正确。*/
  width: 140px;
  height: auto;
  aspect-ratio: 157.506 / 36;
  display: block;
}
.drawer-close {
  width: 32px;
  height: 32px;
  border: none;
  background: #f0efec;
  border-radius: 8px;
  cursor: pointer;
  font-size: 20px;
  color: #212121;
}
.drawer-nav {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 8px;
}
.drawer-link {
  display: block;
  padding: 14px 12px;
  border-radius: 10px;
  font-size: 15px;
  font-weight: 600;
  color: #212121;
  cursor: pointer;
  transition: background 0.15s ease;
}
.drawer-link:hover {
  background: #f5f5f3;
}
.drawer-link.is-active {
  background: rgba(155, 255, 0, 0.2);
  color: #1aa64a;
}
.drawer-footer {
  margin-top: auto;
  border-top: 1px solid #ececea;
  padding-top: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
/* wallet 已连接：地址显示 + 断开按钮 */
.drawer-wallet-block {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.drawer-wallet-addr {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border: 1px solid #ececea;
  border-radius: 10px;
  font-size: 13px;
  font-weight: 700;
  color: #212121;
  font-variant-numeric: tabular-nums;
  background: #f5f5f3;
}
.drawer-wallet-addr-text { flex: 1; }
.drawer-wallet-addr .wallet-dot {
  width: 8px; height: 8px; border-radius: 50%; background: #9bff00;
  box-shadow: 0 0 0 2px rgba(155,255,0,0.25);
}
.drawer-wallet-dashboard,
.drawer-wallet-disconnect {
  height: 36px;
  border: 1px solid #ececea;
  background: white;
  border-radius: 10px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
}
.drawer-wallet-dashboard {
  color: #212121;
}
.drawer-wallet-disconnect {
  color: #d9534f;
}
/* 底部语言双胶囊 */
.drawer-lang-row {
  display: flex;
  gap: 6px;
  padding: 4px;
  background: #f5f5f3;
  border-radius: 999px;
}
.drawer-lang-pill {
  flex: 1;
  height: 34px;
  border: none;
  background: transparent;
  border-radius: 999px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  color: #6a6a6a;
  transition: background .18s ease, color .18s ease;
}
.drawer-lang-pill.is-on {
  background: white;
  color: #212121;
  box-shadow: 0 2px 8px rgba(0,0,0,0.06);
}

/* transitions */
.drawer-fade-enter-active, .drawer-fade-leave-active {
  transition: opacity 0.22s ease;
}
.drawer-fade-enter-from, .drawer-fade-leave-to { opacity: 0; }
.drawer-slide-enter-active, .drawer-slide-leave-active {
  transition: transform 0.28s cubic-bezier(0.22, 1, 0.36, 1);
}
.drawer-slide-enter-from, .drawer-slide-leave-to {
  transform: translateX(100%);
}
</style>
