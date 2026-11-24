import { createRouter, createWebHistory } from 'vue-router'

import { recordPageVisit } from '../services/api'

const routes = [
  {
    path: '/',
    name: 'Home',
    component: () => import('../views/Home.vue')
  },
  {
    path: '/launchpad',
    name: 'LaunchpadList',
    component: () => import('../views/LaunchpadList.vue')
  },
  {
    path: '/launchpad/apply',
    name: 'ProjectApply',
    component: () => import('../views/ProjectApply.vue')
  },
  {
    path: '/launchpad/:id',
    name: 'LaunchpadDetail',
    component: () => import('../views/LaunchpadDetail.vue')
  },
  {
    path: '/project/:id/setup',
    name: 'ProjectSetup',
    component: () => import('../views/ProjectSetup.vue')
  },
  {
    path: '/dashboard',
    name: 'Dashboard',
    component: () => import('../views/Dashboard.vue')
  },
  {
    path: '/tasks',
    name: 'TaskHall',
    component: () => import('../views/TaskHall.vue')
  },
  {
    path: '/team',
    name: 'Team',
    component: () => import('../views/Team.vue')
  },
  {
    path: '/ecosystem',
    name: 'Ecosystem',
    component: () => import('../views/Ecosystem.vue')
  },
  {
    path: '/callback',
    name: 'XCallback',
    redirect: (to) => ({
      path: '/tasks',
      query: to.query
    })
  },
  {
    path: '/news',
    name: 'NewsList',
    component: () => import('../views/NewsList.vue')
  },
  {
    path: '/news/:id',
    name: 'NewsDetail',
    component: () => import('../views/NewsDetail.vue')
  },
  {
    path: '/create-project',
    name: 'CreateProject',
    redirect: '/launchpad/apply'
  },
  {
    path: '/notifications',
    name: 'Notifications',
    component: () => import('../views/Notifications.vue')
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior(to, from, savedPosition) {
    if (savedPosition) {
      return savedPosition
    } else {
      return { top: 0 }
    }
  }
})

// === 路由切换时播放 splash 开幕动画（自下而上贯穿）===
// 用 Web Animations API 直接驱动 splash 元素，避免 CSS 类切换的时序坑
const CURTAIN_EASE = 'cubic-bezier(0.76, 0, 0.24, 1)'
const RISE_MS  = 520   // 自下升到全屏遮挡
const HOLD_MS  = 380   // 中间停留
const EXIT_MS  = 520   // 继续向上升出

let isFirstNavigation = true
let curtainAnim = null   // 记录正在播的动画，避免并发冲突

router.beforeEach((to, from, next) => {
  if (isFirstNavigation || from.path === to.path) {
    isFirstNavigation = false
    next()
    return
  }
  const body = document.body
  const splash = document.querySelector('.splash-intro')
  if (!splash) { next(); return }

  // 锁定 navigating 状态（让 CSS 短过渡覆盖默认 0.85s，且 pointer-events:auto）
  body.classList.add('is-navigating')
  body.classList.remove('is-ready')

  // 取消上一次未完成的动画
  if (curtainAnim) try { curtainAnim.cancel() } catch (_) {}

  // 1) 升起：从下方 +100% → 0（覆盖屏幕）
  curtainAnim = splash.animate(
    [
      { transform: 'translateY(100%)' },
      { transform: 'translateY(0%)' },
    ],
    { duration: RISE_MS, easing: CURTAIN_EASE, fill: 'forwards' },
  )

  // 1b) LOGO 背后的光晕：升起阶段隐藏 → 停留阶段从 0 长到最大
  // 注意：用"每帧 easing"，第一段 linear 才能真正锁死在 0；
  // 全局 easing 会把曲线时间扭曲，导致 0→0 那段也被跳过去
  const halo = splash.querySelector('.splash-intro__halo')
  if (halo) {
    const totalMs = RISE_MS + HOLD_MS
    halo.animate(
      [
        { opacity: 0, transform: 'translate(-50%, -50%) scale(0.5)', offset: 0,
          easing: 'linear' },
        { opacity: 0, transform: 'translate(-50%, -50%) scale(0.5)', offset: RISE_MS / totalMs,
          easing: 'cubic-bezier(0.22, 1, 0.36, 1)' },
        { opacity: 1, transform: 'translate(-50%, -50%) scale(1.0)', offset: 1 },
      ],
      { duration: totalMs, fill: 'forwards' },
    )
  }

  // 2) 升起完成 + 中间停留后再切换路由
  setTimeout(() => next(), RISE_MS + HOLD_MS)
})

router.afterEach((to, from) => {
  // 统计首次进入和后续 SPA 路由切换；失败不阻断页面导航。
  recordPageVisit(to.path).catch(() => {})

  if (!from || !from.name) return  // 首次导航不触发
  const splash = document.querySelector('.splash-intro')
  if (!splash) return

  // 等新页面挂稳一点，再升出
  setTimeout(() => {
    // 3) 升出：从 0 → -100%（继续向上拉离屏幕）
    curtainAnim = splash.animate(
      [
        { transform: 'translateY(0%)' },
        { transform: 'translateY(-100%)' },
      ],
      { duration: EXIT_MS, easing: CURTAIN_EASE, fill: 'forwards' },
    )
    curtainAnim.onfinish = () => {
      // 升出完成后把元素状态交还给 CSS（body.is-ready → translateY(-100%)）
      try { curtainAnim.commitStyles && curtainAnim.commitStyles() } catch (_) {}
      try { curtainAnim.cancel() } catch (_) {}
      curtainAnim = null
      document.body.classList.add('is-ready')
      document.body.classList.remove('is-navigating')
    }
  }, 40)
})

export default router
