<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick, watch, inject } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Lock } from 'lucide-vue-next'
import {
  readContract,
  writeContract,
  waitForTransactionReceipt,
  switchChain,
  getAccount,
} from '@wagmi/core'
import { useWalletStore } from '../stores/wallet'
import { wagmiConfig } from '../services/reownAppKit'
import { ADDRESSES, CHECKIN_CHAIN } from '../config/contracts'
import { JoinCareCheckInAbi } from '../abi/JoinCareCheckIn'
import * as api from '../services/api'
import JcFloatingNavbar from '../components/JcFloatingNavbar.vue'
import HomeFooter from '../components/HomeFooter.vue'
import PixelGrid from '../components/PixelGrid.vue'
import { TRANSLATIONS } from '../i18n/taskhall.js'
import { useIsMobile } from '../composables/useIsMobile'
const { isMobile } = useIsMobile()

const props = defineProps({ language: { type: String, default: 'en' } })
const route = useRoute()
const router = useRouter()
const wallet = useWalletStore()
const isDark = inject('isHomeDark', ref(false))

// ===== 1440 画布缩放 =====
const canvasRef = ref(null)
const scale = ref(1)
const hostHeight = ref('auto')
const naturalHeight = ref(0)
const DESIGN_WIDTH = 1440
const MIN_SCALE = 0.35
const MAX_SCALE = 1.25

const measure = () => {
  if (!canvasRef.value) return
  const prev = canvasRef.value.style.transform
  canvasRef.value.style.transform = 'scale(1)'
  naturalHeight.value = canvasRef.value.scrollHeight
  canvasRef.value.style.transform = prev
  applyScale()
}
const applyScale = () => {
  const w = window.innerWidth
  const raw = w / DESIGN_WIDTH
  scale.value = Math.min(Math.max(raw, MIN_SCALE), MAX_SCALE)
  hostHeight.value = `${naturalHeight.value * scale.value}px`
}

const scrollRevealHandler = () => {
  const els = document.querySelectorAll('[data-reveal]:not(.is-revealed)')
  if (!els.length) return
  const trigger = window.innerHeight * 0.80
  els.forEach((el) => {
    if (el.getBoundingClientRect().top < trigger) el.classList.add('is-revealed')
  })
}

const A = '/figma/v2'
const img = (n) => `${A}/${n}`

const t = computed(() => TRANSLATIONS[props.language] || TRANSLATIONS.en)

// 默认全 0 / 空 — 不再用 mock；未登录时模板用 isLoggedIn + placeholder("—") 显示
const info = ref({
  totalPoints: 0, checkInCount: 0, streak: 0,
  checkedToday: false, inviteCode: '',
  invitedCount: 0, invitedPoints: 0,
  // 本月已签到的日期数组（如 [1,2,3]）
  checkedDates: [],
  // X 完成状态按 task id 保存；兼容旧 Backend 时再回退到 task_key。
  x: {},
  // Telegram 子任务完成状态
  tg: { join: false, chat: false },
  tgChatEarnedPoints: 0,
})
const busyDaily = ref(false)
const chainCheckInOverride = ref({ wallet: '', day: '' })
const copyState = ref({ code: false, link: false })
const busyTask = ref({})  // 通用任务 verify 中状态

const currentUtcDay = () => new Date().toISOString().slice(0, 10)
const normalizedWallet = () => String(wallet.address || '').toLowerCase()
const pendingCheckInStorageKey = () => [
  'jc_checkin_pending',
  CHECKIN_CHAIN.chainId,
  String(ADDRESSES.CheckIn || '').toLowerCase(),
  normalizedWallet(),
].join(':')
const hasCurrentChainCheckInOverride = () => (
  chainCheckInOverride.value.wallet === normalizedWallet() &&
  chainCheckInOverride.value.day === currentUtcDay()
)
const markCurrentChainCheckIn = () => {
  chainCheckInOverride.value = { wallet: normalizedWallet(), day: currentUtcDay() }
}
const getPendingCheckInHash = () => {
  if (!normalizedWallet()) return ''
  try {
    const hash = localStorage.getItem(pendingCheckInStorageKey()) || ''
    return /^0x[0-9a-f]{64}$/i.test(hash) ? hash : ''
  } catch (_) {
    return ''
  }
}
const savePendingCheckInHash = (hash) => {
  try { localStorage.setItem(pendingCheckInStorageKey(), hash) } catch (_) {}
}
const clearPendingCheckInHash = () => {
  try { localStorage.removeItem(pendingCheckInStorageKey()) } catch (_) {}
}

// === 日历方格：当月每一天，标记签到/今天/未来 ===
const today = new Date()
const calMonth = computed(() => {
  const y = today.getFullYear()
  const m = today.getMonth()
  const daysInMonth = new Date(y, m + 1, 0).getDate()
  const firstWeekday = new Date(y, m, 1).getDay()    // 0=Sun
  const todayDate = today.getDate()
  // 若后端没回填具体日期，用 streak 近似最近 N 天为已签到
  const checked = new Set(
    info.value.checkedDates?.length
      ? info.value.checkedDates
      : Array.from({ length: Math.min(info.value.streak, todayDate) }, (_, i) => todayDate - i)
  )
  const cells = []
  for (let i = 0; i < firstWeekday; i++) cells.push({ blank: true })
  for (let d = 1; d <= daysInMonth; d++) {
    cells.push({
      day: d,
      isToday: d === todayDate,
      isChecked: checked.has(d),
      isFuture: d > todayDate,
    })
  }
  return { cells, monthLabel: today.toLocaleString('en-US', { month: 'long', year: 'numeric' }) }
})

// === X / TG 任务列表 ===
// task 表里的标题、描述和 CTA 由 Admin 针对每条任务单独配置，是任务列表的
// 权威文案。静态/copywriting_config 文案只在后台字段为空时作为旧数据兜底。
const remoteRaw = ref(null)
const twitterVerificationDisabled = ref(false)
async function loadTasks() {
	// Reset before every read so a stale bypass policy is never retained after
	// an error or partial response.
	twitterVerificationDisabled.value = false
  try {
    const res = await api.getTaskList?.()
    const items = (res && res.items) || []
	// Missing/invalid policy data is strict by default. Backend also enforces
	// the authoritative policy independently on every completion request.
	twitterVerificationDisabled.value = res?.verificationDisabled === true && items.length > 0
    if (items.length === 0) return
    remoteRaw.value = items
  } catch (e) {
	console.warn('[TaskHall] task list fetch failed:', e?.message)
  }
}

// admin 里 task_key 可能是 "follow" 或带平台前缀 "x_follow" / "twitter_follow"，归一
function normalizeTaskKey(k) {
  return String(k || '').toLowerCase().replace(/^(x_|tg_|twitter_|telegram_)/, '')
}

function resolveTaskCopy(remoteValue, localizedValue) {
  return remoteValue || localizedValue
}

function itemToTask(i, prefix) {
  const suffix = normalizeTaskKey(i.task_key)
  const base = `${prefix}_${suffix}`
  return {
    id: Number(i.id) || 0,
    version: Number(i.version) || 1,
    targetSlot: Number(i.target_slot) || 1,
    key: i.task_key,
    title:  resolveTaskCopy(i.title, t.value[`${base}_title`]),
    desc:   resolveTaskCopy(i.description, t.value[`${base}_desc`]),
    url:    i.url,
    cta:    resolveTaskCopy(i.cta_label, t.value[`${base}_cta`]),
    points: i.amount_point,
  }
}

const xTasksFallback = computed(() => [
  { id: 0, version: 0, key: 'follow',   title: t.value.xTask_follow_title,   desc: t.value.xTask_follow_desc,   url: 'https://x.com/joincare',                        cta: t.value.xTask_follow_cta,   points: 25 },
  { id: 0, version: 0, key: 'like',     title: t.value.xTask_like_title,     desc: t.value.xTask_like_desc,     url: 'https://x.com/joincare/status/1800000000000000001', cta: t.value.xTask_like_cta,     points: 25 },
  { id: 0, version: 0, key: 'bookmark', title: t.value.xTask_bookmark_title, desc: t.value.xTask_bookmark_desc, url: 'https://x.com/joincare/status/1800000000000000002', cta: t.value.xTask_bookmark_cta, points: 25 },
  { id: 0, version: 0, key: 'reply',    title: t.value.xTask_reply_title,    desc: t.value.xTask_reply_desc,    url: 'https://x.com/joincare/status/1800000000000000003', cta: t.value.xTask_reply_cta,    points: 25 },
])
const tgTasksFallback = computed(() => [
  { key: 'join', title: t.value.tgTask_join_title, desc: t.value.tgTask_join_desc, url: 'https://t.me/joincare',      cta: t.value.tgTask_join_cta, points: 50 },
  { key: 'chat', title: t.value.tgTask_chat_title, desc: t.value.tgTask_chat_desc, url: 'https://t.me/joincare_chat', cta: t.value.tgTask_chat_cta, points: 50 },
])

const xTasks = computed(() => {
  if (remoteRaw.value?.length) {
    return remoteRaw.value
      .filter(i => i.platform === 'twitter')
      .sort((a, b) => a.sort - b.sort || (Number(a.target_slot) || 1) - (Number(b.target_slot) || 1) || a.id - b.id)
      .map(i => itemToTask(i, 'xTask'))
  }
  return xTasksFallback.value
})
const tgTasks = computed(() => {
  if (remoteRaw.value?.length) {
    return remoteRaw.value
      .filter(i => i.platform === 'telegram')
      .sort((a, b) => {
        const order = { join: 0, chat: 1 }
        const aOrder = order[normalizeTaskKey(a.task_key)] ?? 2
        const bOrder = order[normalizeTaskKey(b.task_key)] ?? 2
        return aOrder - bOrder || a.sort - b.sort
      })
      .map(i => itemToTask(i, 'tgTask'))
  }
  return tgTasksFallback.value
})
const taskStateKey = task => task?.id ? `id:${task.id}` : `key:${normalizeTaskKey(task?.key)}`
const taskBusyPath = (group, task) => `${group}.${taskStateKey(task)}`
const isTaskDone = (group, task) => Boolean(
  info.value[group]?.[taskStateKey(task)] ||
  (group === 'x' && info.value.x?.[`key:${normalizeTaskKey(task?.key)}`])
)
const xProgress = computed(() => xTasks.value.filter(task => isTaskDone('x', task)).length)
const xRewardTotal = computed(() => xTasks.value.reduce((sum, task) => sum + (Number(task.points) || 0), 0))
const isXPostTask = task => ['like', 'reply', 'retweet'].includes(normalizeTaskKey(task?.key))
const xTargetLabel = task => Number(task?.targetSlot) === 2 ? t.value.xTask_target2 : t.value.xTask_target1
const tgProgress = computed(() => Object.values(info.value.tg).filter(Boolean).length)

function taskDisplayPoints(group, task) {
  const key = normalizeTaskKey(task?.key)
  if (group === 'tg' && key === 'chat' && info.value.tgChatEarnedPoints > 0) {
    return info.value.tgChatEarnedPoints
  }
  return task?.points ?? 0
}

function isTgTaskLocked(task) {
  return normalizeTaskKey(task?.key) === 'chat' && !info.value.tg.join
}

// ===== X (Twitter) 绑定状态 —— 决定 verify 逻辑走绑定弹窗还是直接调 completeTask =====
const xBinding = ref({ isBound: false, tokenValid: false, twitterHandle: '' })
async function loadXBinding() {
  if (!wallet.isLoggedIn) { xBinding.value = { isBound: false, tokenValid: false, twitterHandle: '' }; return }
  try {
    xBinding.value = await api.getXBinding() || xBinding.value
  } catch (e) { console.warn('[TaskHall] getXBinding failed:', e?.message) }
}

// ===== X read-only verification modal =====
// JoinCare never performs follow/like/repost/reply for the user. The modal
// separates opening X from the explicit read-only verification request.
const xVerifyModal = ref({
  open: false,
  url: '',
  points: 0,
  path: '',
  key: '',
  taskId: 0,
  taskVersion: 0,
  taskStateKey: '',
  submitting: false,
})
function openXVerifyModal(url, points, path, key, taskId, taskVersion, stateKey) {
  xVerifyModal.value = { open: true, url, points, path, key, taskId, taskVersion, taskStateKey: stateKey, submitting: false }
}
function closeXVerifyModal() {
  xVerifyModal.value.open = false
}
async function submitXVerification() {
  const { path, key, points, taskId, taskVersion, taskStateKey: stateKey } = xVerifyModal.value
  xVerifyModal.value.submitting = true
  busyTask.value[path] = true
  try {
    await api.completeTwitterTask(key, '', taskId, taskVersion)
    info.value.x[stateKey] = true
    info.value.totalPoints += points
    recentlyDone.value = path
    setTimeout(() => { if (recentlyDone.value === path) recentlyDone.value = '' }, 1600)
    closeXVerifyModal()
  } catch (e) {
    const backendMessage = e?.message || ''
    console.warn('[TaskHall] X read-only verification failed:', backendMessage)
    if (e?.code === 'ERR.001.00021') {
      await Promise.all([loadTasks(), loadInfo()])
      alert(TRANSLATIONS.en.xTaskAlreadyCompleted)
      return
    }
    if (e?.code === 'ERR.001.00019') {
      alert(TRANSLATIONS.en.xVerifyModal_notFound)
    } else {
      // Native system alerts for X task errors are intentionally English-only.
      // Do not append Backend-localized text, which may be Chinese.
      alert(TRANSLATIONS.en.xVerifyModal_failed + (e?.code || 'UNKNOWN'))
    }
  } finally {
    xVerifyModal.value.submitting = false
    busyTask.value[path] = false
  }
}


// 移动端使用标准 HTTPS OAuth 地址在当前页跳转，让系统有机会通过
// Universal Link / App Link 拉起 X App；未命中时自然回退到网页授权。
//
// 安全校验（审计 0.5）：跳转 URL 由后端返回，不能盲信。
// 只允许跳到 X 官方授权域，且 redirect_uri 必须指向本站，
// 防后端配置被污染时把用户带去钓鱼页 / 授权码被窃。
function assertSafeXAuthUrl(raw) {
  const u = new URL(raw)
  const hostOk = ['twitter.com', 'www.twitter.com', 'x.com', 'www.x.com'].includes(u.hostname)
  if (u.protocol !== 'https:' || !hostOk) {
    throw new Error('unexpected X auth host')
  }
  const cb = u.searchParams.get('redirect_uri')
  if (cb && new URL(cb).origin !== window.location.origin) {
    throw new Error('redirect_uri origin mismatch')
  }
}

async function openXBindingFlow() {
  try {
    const { url } = await api.getXLoginUrl()
    if (!url) return
    assertSafeXAuthUrl(url)
    if (isMobile.value) {
      window.location.assign(url)
    } else {
      window.open(url, '_blank', 'noopener')
    }
  } catch (e) {
    console.warn('[TaskHall] getXLoginUrl failed:', e?.message)
    alert('X authorization failed: ' + (e?.code || 'UNKNOWN'))
  }
}

// verify 函数：TG 走原 bot deep-link 流程；X 一律由用户手工完成，再由 Backend 只读验证。
const recentlyDone = ref('')
const openAndVerify = async (group, key, url, points, openTarget = true, taskId = 0, taskVersion = 0, stateKey = '') => {
  const resolvedStateKey = stateKey || (group === 'x' ? `key:${normalizeTaskKey(key)}` : key)
  const path = `${group}.${resolvedStateKey}`
  if (info.value[group][resolvedStateKey] || busyTask.value[path]) return
  if (group === 'tg' && normalizeTaskKey(key) === 'chat' && !info.value.tg.join) return

  // === X 任务：先 check 绑定状态 ===
  if (group === 'x') {
    if (!xBinding.value.isBound || !xBinding.value.tokenValid) {
      // 未绑定 or token 失效 → 触发绑定 OAuth，不做假 setTimeout 加分
      if (!confirm(t.value.bindXConfirm)) return
      await openXBindingFlow()
      return
    }

    openXVerifyModal(url, points, path, key, taskId, taskVersion, resolvedStateKey)
    if (openTarget) {
      try { window.open(url, '_blank', 'noopener') } catch (_) {}
    }
    return
  }

  // === TG 任务：deep-link + bot 回调 report ===
  busyTask.value[path] = true
  let openUrl = url
  if (group === 'tg') {
    try {
      const link = await api.getTgBindLink()
      if (link?.url) openUrl = link.url
    } catch (e) {
      console.warn('[TaskHall] tgBindLink failed, fallback to raw url:', e?.message)
    }
  }
  try { window.open(openUrl, '_blank', 'noopener') } catch (_) {}

  if (key === 'join') {
    // Join 任务：进入"检测中"状态 poll backend 检查是否已加群（最多 60 秒，每 3 秒查一次）
    let stopped = false
    const startTs = Date.now()
    const poll = async () => {
      if (stopped) return
      try {
        const r = await api.checkTgJoin()
        if (r?.verified) {
          info.value[group][key] = true
          if (r.awarded > 0) info.value.totalPoints += r.awarded
          recentlyDone.value = path
          setTimeout(() => { if (recentlyDone.value === path) recentlyDone.value = '' }, 1600)
          busyTask.value[path] = false
          return
        }
      } catch (e) { console.warn('[TaskHall] checkTgJoin poll error:', e?.message) }
      // 未检测到 → 继续或超时
      if (Date.now() - startTs > 60000) {
        stopped = true
        busyTask.value[path] = false
        return
      }
      setTimeout(poll, 3000)
    }
    setTimeout(poll, 3000)
  } else {
    // chat 等其他 TG 任务：走 completeTask 加分（chat 可重复加分不锁 UI）
    setTimeout(async () => {
      try {
        await api.completeTelegramTask(key)
        info.value.totalPoints += points
        if (key === 'chat') {
          info.value.tgChatEarnedPoints += points
          info.value[group][key] = true
        }
        recentlyDone.value = path
        setTimeout(() => { if (recentlyDone.value === path) recentlyDone.value = '' }, 1600)
        if (key !== 'chat') info.value[group][key] = true
      } catch (e) { console.warn('[TaskHall] TG task complete failed:', e?.message) }
      finally { busyTask.value[path] = false }
    }, 1200)
  }
}

// ===== 处理 X OAuth callback：/callback?state=&code= → redirect 到 /tasks?state=&code= =====
async function handleXCallback() {
  const { state, code } = route.query
  if (!state || !code) return
  try {
    await api.bindXLogin(state, code)
    await loadXBinding()
    // 清 URL 上的 state/code query，避免刷新时重复触发
    router.replace({ path: route.path, query: {} })
  } catch (e) {
    console.warn('[TaskHall] X callback bind failed:', e?.message)
    // backend message 已跟当前语言匹配（Jc-Language header）；无 message 时用 i18n fallback
    const msg = (e?.message || '').trim() || t.value.xBindFailed || 'X binding failed'
    showPageToast(msg, 'error', 6000)
    // 清 query，避免刷新页面时又 trigger 一次同样的 callback → 同样的错
    router.replace({ path: route.path, query: {} })
  }
}

// 钱包登录态 / 地址变化时重 load info（未登录→重置 0，登录后→拉真后端）
// 同时监听 address —— 在 navbar 走 connect 时，isLoggedIn 从 false→true 之间
// 若 token/uid 落地时机稍晚可能漏 fire；wallet.address 变化是 authenticate 完成前后必然发生的信号。
watch(
  () => [wallet.isLoggedIn, wallet.address],
  () => {
    loadInfo().then(() => { void resumePendingCheckIn() })
    loadTasks()
    loadXBinding()
  },
)

// 未登录时遮罩按钮点击 → 弹钱包选择器
const onLockedConnect = async () => {
  try {
    await wallet.connectWallet()
    // 显式 reload —— 不依赖 watch(isLoggedIn) 触发（有时机不稳）
    await Promise.all([loadInfo(), loadTasks(), loadXBinding()])
  } catch (e) {
    if (e?.message !== 'User cancelled wallet selection') {
      console.warn('[TaskHall] connect failed:', e?.message)
    }
  }
}

// Total XP 数值平滑 count-up（积分变化时 ease-out 0.8s 走数字）
const displayPoints = ref(info.value.totalPoints)
let pointsRaf = null
watch(() => info.value.totalPoints, (next) => {
  const start = displayPoints.value
  const startTime = performance.now()
  const dur = 800
  if (pointsRaf) cancelAnimationFrame(pointsRaf)
  const tick = (now) => {
    const t = Math.min(1, (now - startTime) / dur)
    const eased = 1 - Math.pow(1 - t, 3)
    displayPoints.value = Math.round(start + (next - start) * eased)
    if (t < 1) pointsRaf = requestAnimationFrame(tick)
  }
  pointsRaf = requestAnimationFrame(tick)
})

// 分享：将邀请链接抛到 X / TG
const shareX = () => {
  const txt = encodeURIComponent(`I'm earning XP on @joincare — join with my link and we both earn XP 🧬\n${refLink.value}`)
  window.open(`https://x.com/intent/tweet?text=${txt}`, '_blank', 'noopener')
}
const shareTg = () => {
  const u = encodeURIComponent(refLink.value)
  const t = encodeURIComponent('Earn $JC on JoinCare — bio research, on-chain.')
  window.open(`https://t.me/share/url?url=${u}&text=${t}`, '_blank', 'noopener')
}

const refLink = computed(() => {
  const base = typeof window !== 'undefined' ? window.location.origin : 'https://joincarelabs.com'
  return info.value.inviteCode ? `${base}/?invite_code=${info.value.inviteCode}` : `${base}/`
})

const shortAddr = computed(() => {
  const a = wallet.address || ''
  return a ? `${a.slice(0, 6)}...${a.slice(-4)}` : ''
})

// 未登录时直接重置为 0；登录后从后端 /taskhall/v1/info 拉取并按 InfoResponse 嵌套结构映射。
// 后端字段：
//   pointOverview = { totalPoint, totalCheckInDays, continuousCheckInDays, walletAddress }
//   checkInTask   = { status }  // AVAILABLE / COMPLETED / DISABLED
//   inviteTask    = { inviteCode, successfulInviteCount, earnedPoint }
const loadInfo = async () => {
  const requestedWallet = normalizedWallet()
  if (busyDaily.value) return
  if (!wallet.isLoggedIn) {
    info.value = {
      totalPoints: 0, checkInCount: 0, streak: 0,
      checkedToday: false, inviteCode: '',
      invitedCount: 0, invitedPoints: 0,
      checkedDates: [],
      x: {},
      tg: { join: false, chat: false },
      tgChatEarnedPoints: 0,
    }
    return
  }
  try {
    const d = await api.getTaskHallInfo?.()
    if (!d || typeof d !== 'object') return
    // An already-running display request can complete after the user clicks
    // check-in or changes wallet. Never let that stale response clear a
    // pending tx or overwrite the wallet-owned check-in state.
    if (busyDaily.value || requestedWallet !== normalizedWallet() || !wallet.isLoggedIn) return
    const pov = d.pointOverview || {}
    const ci  = d.checkInTask   || {}
    const inv = d.inviteTask    || {}
    const tw  = d.twitterTask   || {}
    const tg  = d.telegramTask  || {}
    // 新 Backend 返回 task id，可区分同一互动类型的两个帖子；旧 Backend 回退 task_key。
    const xState = {}
    if (Array.isArray(tw.completedTaskIds) && tw.completedTaskIds.length) {
      for (const id of tw.completedTaskIds) xState[`id:${Number(id)}`] = true
    } else if (Array.isArray(tw.completedKeys)) {
      for (const k of tw.completedKeys) xState[`key:${normalizeTaskKey(k)}`] = true
    }
    // TG 任务同步（join 完成即锁 UI；chat 是可重复的，后端不会记 chat 为完成态）
    const tgState = { join: false, chat: false }
    if (Array.isArray(tg.completedKeys)) {
      for (const k of tg.completedKeys) tgState[k] = true
    }
    const tgChatEarnedPoints = Number(tg.chatEarnedPoint) || 0
    if (tgChatEarnedPoints > 0) tgState.chat = true
    const backendCheckedToday = ci.status === 'COMPLETED'
    if (backendCheckedToday) clearPendingCheckInHash()
    info.value = {
      totalPoints:   pov.totalPoint            ?? 0,
      checkInCount:  pov.totalCheckInDays      ?? 0,
      streak:        pov.continuousCheckInDays ?? 0,
      checkedToday:  backendCheckedToday || hasCurrentChainCheckInOverride(),
      // 后端 checkInTask.checkedDates 返回本月已签到的号数字数组 [1, 3, 7, ...]
      checkedDates:  Array.isArray(ci.checkedDates) ? ci.checkedDates : [],
      inviteCode:    inv.inviteCode ?? '',
      invitedCount:  inv.successfulInviteCount ?? 0,
      invitedPoints: inv.earnedPoint ?? 0,
      x: xState,
      tg: tgState,
      tgChatEarnedPoints,
    }
  } catch (e) {
    console.warn('[TaskHall] loadInfo failed:', e?.message)
  }
}

// 签到错误反馈：短暂展示在按钮附近；也 console.warn 便于排查
// TG messages and referral rewards are credited outside this page. Refresh while the
// task center is visible, and refresh immediately when the user returns from TG/X.
// /taskhall/v1/info includes a global leaderboard. Do not flood Backend
// with overlapping full aggregates while the page is visible.
const TASK_INFO_POLL_INTERVAL = 30000
let taskInfoPollTimer = null
let taskInfoRefreshInFlight = false

const refreshTaskInfo = async () => {
  if (!wallet.isLoggedIn || busyDaily.value || document.visibilityState === 'hidden' || taskInfoRefreshInFlight) return
  taskInfoRefreshInFlight = true
  try {
    await loadInfo()
    void resumePendingCheckIn()
  } finally {
    taskInfoRefreshInFlight = false
  }
}

function handleXTaskLink(event, task) {
  const path = taskBusyPath('x', task)
  if (!task.url || isTaskDone('x', task) || busyTask.value[path]) {
    event.preventDefault()
    return
  }
  event.preventDefault()
  // The Admin bypass controls only Backend external X API verification.
  // The user-facing flow remains two-stage: open the target, then explicitly
  // confirm completion in the modal. Backend records the confirmation as manual_pass.
  openXVerifyModal(
    task.url,
    task.points,
    path,
    task.key,
    task.id,
    task.version,
    taskStateKey(task),
  )
  try { window.open(task.url, '_blank', 'noopener') } catch (_) {}
}

const stopTaskInfoPolling = () => {
  if (taskInfoPollTimer === null) return
  window.clearInterval(taskInfoPollTimer)
  taskInfoPollTimer = null
}

const startTaskInfoPolling = () => {
  stopTaskInfoPolling()
  if (document.visibilityState === 'hidden') return
  taskInfoPollTimer = window.setInterval(refreshTaskInfo, TASK_INFO_POLL_INTERVAL)
}

const handleTaskPageVisibility = () => {
  if (document.visibilityState === 'visible') {
    refreshTaskInfo()
    startTaskInfoPolling()
  } else {
    stopTaskInfoPolling()
  }
}

const handleTaskPageFocus = () => {
  refreshTaskInfo()
  startTaskInfoPolling()
}

const checkInMsg = ref('')
const checkInMsgKind = ref('info')
const checkInText = (key, vars = {}) => {
  let message = t.value[key] || TRANSLATIONS.en[key] || key
  for (const [name, value] of Object.entries(vars)) {
    message = message.replaceAll(`{${name}}`, String(value))
  }
  return message
}
const showCheckInMsg = (msg, ms = 4000, kind = 'info') => {
  checkInMsg.value = msg
  checkInMsgKind.value = kind
  setTimeout(() => { if (checkInMsg.value === msg) checkInMsg.value = '' }, ms)
}

// 页面级 toast：给 X OAuth 回跳、重复绑定等需要用户可见但不阻断的反馈用。
// message 内容优先用 backend 返回的原文（backend 已经根据 Jc-Language 头返回对应语种）。
const pageToast = ref({ msg: '', kind: 'info', open: false })
let pageToastTimer = null
function showPageToast(msg, kind = 'info', ms = 5000) {
  pageToast.value = { msg: String(msg || ''), kind, open: true }
  if (pageToastTimer) clearTimeout(pageToastTimer)
  pageToastTimer = setTimeout(() => { pageToast.value.open = false }, ms)
}
function closePageToast() {
  pageToast.value.open = false
  if (pageToastTimer) { clearTimeout(pageToastTimer); pageToastTimer = null }
}
// 三态颜色由流程明确指定，不能依赖英文关键词，否则繁中提示会被错误着色。
const checkInMsgColorClass = computed(() => {
  if (checkInMsgKind.value === 'success') return 'text-[#1aa64a]'
  if (checkInMsgKind.value === 'error') return 'text-[#c0392b]'
  return 'text-[#808080]'
})

const applyCheckInResult = (res) => {
  const point = Number(res?.point)
  const totalPoint = Number(res?.totalPoint)
  const totalDays = Number(res?.totalCheckInDays)
  const streak = Number(res?.continuousCheckInDays)
  const checkedToday = !res?.checkInDate || res.checkInDate === currentUtcDay()

  if (checkedToday) {
    info.value.checkedToday = true
    markCurrentChainCheckIn()
  }
  if (Number.isFinite(totalPoint)) info.value.totalPoints = totalPoint
  if (Number.isFinite(totalDays) && totalDays > 0) info.value.checkInCount = totalDays
  if (Number.isFinite(streak) && streak > 0) info.value.streak = streak
  return {
    point: Number.isFinite(point) ? point : 0,
    streak: Number.isFinite(streak) ? streak : info.value.streak,
    checkedToday,
  }
}

let pendingCheckInRecoveryInFlight = false
let lastPendingCheckInRecoveryAt = 0
async function resumePendingCheckIn() {
  const hash = getPendingCheckInHash()
  const shouldReconcile = !hash && hasCurrentChainCheckInOverride()
  const now = Date.now()
  if (!wallet.isLoggedIn || (!hash && !shouldReconcile) || busyDaily.value || pendingCheckInRecoveryInFlight ||
      now - lastPendingCheckInRecoveryAt < 15000) return false

  pendingCheckInRecoveryInFlight = true
  lastPendingCheckInRecoveryAt = now
  try {
    const res = hash ? await api.submitCheckIn(hash) : await api.reconcileCheckIn()
    clearPendingCheckInHash()
    applyCheckInResult(res)
    return true
  } catch (e) {
    console.warn('[TaskHall] check-in settlement retry failed:', e?.message)
    return false
  } finally {
    pendingCheckInRecoveryInFlight = false
  }
}

const reconcileCurrentCheckIn = async () => {
  markCurrentChainCheckIn()
  info.value.checkedToday = true
  showCheckInMsg(checkInText('checkInRestoringConfirmed'), 60000)
  const res = await api.reconcileCheckIn()
  clearPendingCheckInHash()
  return applyCheckInResult(res)
}

const doCheckIn = async () => {
  console.log('[doCheckIn] click', {
    isLoggedIn: wallet.isLoggedIn,
    checkedToday: info.value.checkedToday,
    busyDaily: busyDaily.value,
    walletAddress: wallet.address,
  })
  if (!wallet.isLoggedIn) {
    console.warn('[doCheckIn] early-return: not eligible')
    showCheckInMsg(checkInText('checkInConnectWallet'), 4000, 'error')
    return
  }
  if (info.value.checkedToday) {
    showCheckInMsg(checkInText('checkInAlready'), 4000)
    return
  }
  if (busyDaily.value) {
    return
  }
  busyDaily.value = true
  checkInMsg.value = ''
  let submittedHash = ''
  let chainConfirmed = false

  try {
    // Display aggregates are not a preflight authority. A slow/504 task info
    // request must never hold the wallet action hostage. The contract read
    // below is the authoritative current-day check before any wallet send.

    const account = getAccount(wagmiConfig)
    const targetChainId = CHECKIN_CHAIN.chainId
    if (!account.isConnected || !account.address ||
        account.address.toLowerCase() !== normalizedWallet()) {
      throw new Error('CHECKIN_WALLET_MISMATCH')
    }
    console.log('[doCheckIn] chain state', {
      current: account.chainId,
      target: targetChainId,
      connector: account.connector?.id,
      isConnected: account.isConnected,
    })

    if (account.chainId !== targetChainId) {
      showCheckInMsg(checkInText('checkInSwitching'), 8000)
      try {
        console.log('[doCheckIn] switchChain →', targetChainId)
        await switchChain(wagmiConfig, { chainId: targetChainId })
        console.log('[doCheckIn] switchChain OK, new =', getAccount(wagmiConfig).chainId)
      } catch (switchErr) {
        console.error('[doCheckIn] switchChain failed', switchErr)
        if (/reject|denied|user/i.test(switchErr?.message || '')) {
          throw new Error('CHECKIN_SWITCH_REQUIRED')
        }
        throw switchErr
      }
    } else {
      console.log('[doCheckIn] already on target chain, skip switch')
    }

    const currentAccount = getAccount(wagmiConfig)
    if (!currentAccount.isConnected || currentAccount.chainId !== targetChainId ||
        currentAccount.address?.toLowerCase() !== normalizedWallet()) {
      throw new Error('CHECKIN_WALLET_MISMATCH')
    }

    const pendingHash = getPendingCheckInHash()
    if (pendingHash) {
      showCheckInMsg(checkInText('checkInRestoringPrevious'), 60000)
      const res = await api.submitCheckIn(pendingHash)
      clearPendingCheckInHash()
      const restored = applyCheckInResult(res)
      if (restored.checkedToday) {
        showCheckInMsg(checkInText('checkInSuccess', restored), 4000, 'success')
        return
      }
    }

    // 2. 先读链上状态，避免后端结算延迟时再次调用 checkIn()。
    showCheckInMsg(checkInText('checkInCheckingChain'), 15000)
    const checkedOnChain = await readContract(wagmiConfig, {
      address: ADDRESSES.CheckIn,
      abi: JoinCareCheckInAbi,
      functionName: 'hasCheckedInToday',
      args: [wallet.address],
      chainId: targetChainId,
    })
    if (checkedOnChain) {
      const restored = await reconcileCurrentCheckIn()
      showCheckInMsg(checkInText('checkInSuccess', restored), 4000, 'success')
      return
    }

    // 3. 链上未签到才发交易调 checkIn()。
    const signingAccount = getAccount(wagmiConfig)
    if (!wallet.isLoggedIn || !signingAccount.isConnected ||
        signingAccount.chainId !== targetChainId ||
        signingAccount.address?.toLowerCase() !== normalizedWallet()) {
      throw new Error('CHECKIN_WALLET_MISMATCH')
    }
    showCheckInMsg(checkInText('checkInSignWallet'), 30000)
    console.log('[doCheckIn] writeContract →', ADDRESSES.CheckIn)
    const hash = await writeContract(wagmiConfig, {
      address: ADDRESSES.CheckIn,
      abi: JoinCareCheckInAbi,
      functionName: 'checkIn',
      args: [],
      account: wallet.address,
      chainId: targetChainId,
    })
    submittedHash = hash
    savePendingCheckInHash(hash)
    console.log('[doCheckIn] tx submitted, hash =', hash)

    // 4. 等 receipt 落块（testnet 3-15s，主网 10-30s）
    showCheckInMsg(checkInText('checkInWaiting'), 60000)
    const receipt = await waitForTransactionReceipt(wagmiConfig, { hash, confirmations: 1 })
    if (receipt.status !== 'success') {
      clearPendingCheckInHash()
      throw new Error('CHECKIN_TX_REVERTED')
    }
    chainConfirmed = true

    // 5. 通知后端结算积分；成功后才移除待恢复 hash。
    const res = await api.submitCheckIn?.(hash)
    clearPendingCheckInHash()
    const settled = applyCheckInResult(res)
    showCheckInMsg(checkInText('checkInSuccess', settled), 4000, 'success')
  } catch (e) {
    const raw = e?.shortMessage || e?.message || ''
    let msg = checkInText('checkInFailed')
    let msgKind = 'error'
    if (/reject|denied|user rejected/i.test(raw)) {
      msg = checkInText('checkInCancelled')
    } else if (/insufficient funds|insufficient balance/i.test(raw)) {
      msg = checkInText('checkInNotEnoughGas', { symbol: CHECKIN_CHAIN.currency?.symbol || 'BNB' })
    } else if (raw === 'CHECKIN_SWITCH_REQUIRED') {
      msg = checkInText('checkInSwitchRequired', { network: CHECKIN_CHAIN.name })
    } else if (raw === 'CHECKIN_WALLET_MISMATCH') {
      msg = checkInText('checkInConnectWallet')
    } else if (raw === 'CHECKIN_TX_REVERTED') {
      msg = checkInText('checkInTxReverted')
    } else if (/Already checked in/i.test(raw)) {
      msg = checkInText('checkInAlready')
      msgKind = 'info'
      markCurrentChainCheckIn()
      info.value.checkedToday = true
      try {
        const restored = await reconcileCurrentCheckIn()
        msg = checkInText('checkInSuccess', restored)
        msgKind = 'success'
      } catch (reconcileError) {
        console.warn('[doCheckIn] reconcile after duplicate failed:', reconcileError?.message)
      }
    } else if (submittedHash && chainConfirmed) {
      markCurrentChainCheckIn()
      info.value.checkedToday = true
      msg = checkInText('checkInSettlementRetry')
      msgKind = 'info'
    }
    console.warn('[doCheckIn] failed:', raw)
    showCheckInMsg(msg, 5000, msgKind)
  } finally {
    busyDaily.value = false
  }
}

// clipboard 在 HTTP + 非 localhost 域下 Chrome/Safari 禁用 navigator.clipboard，
// 需要 execCommand('copy') 作 fallback；再失败时提示用户手动复制
const copy = async (val, key) => {
  const text = String(val || '')
  if (!text) return
  const flash = () => {
    copyState.value[key] = true
    setTimeout(() => (copyState.value[key] = false), 1500)
  }
  // 优先走 Clipboard API（HTTPS / localhost 场景）
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      flash()
      return
    } catch (e) {
      console.warn('[copy] clipboard API failed:', e?.message)
    }
  }
  // Fallback: execCommand('copy') via detached textarea
  try {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.setAttribute('readonly', '')
    ta.style.position = 'fixed'
    ta.style.top = '-9999px'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    ta.setSelectionRange(0, text.length)
    const ok = document.execCommand('copy')
    document.body.removeChild(ta)
    if (ok) { flash(); return }
    throw new Error('execCommand returned false')
  } catch (e) {
    console.warn('[copy] execCommand fallback failed:', e?.message)
    // 最终 fallback: 弹 window.prompt 让用户手动 Ctrl+C
    try { window.prompt('复制以下内容 (Ctrl+C / Cmd+C)', text) } catch (_) {}
  }
}

onMounted(async () => {
  window.scrollTo(0, 0)
  const inv = route.query.invite_code
  if (typeof inv === 'string' && inv.trim()) {
    sessionStorage.setItem('jc_invite_code', inv.trim())
  }
  nextTick(() => { measure(); scrollRevealHandler() })
  setTimeout(() => { measure(); scrollRevealHandler() }, 500)
  window.addEventListener('resize', applyScale)
  window.addEventListener('resize', scrollRevealHandler)
  window.addEventListener('scroll', scrollRevealHandler, { passive: true })
  window.addEventListener('focus', handleTaskPageFocus)
  document.addEventListener('visibilitychange', handleTaskPageVisibility)
  await Promise.all([loadInfo(), loadTasks(), loadXBinding()])
  void resumePendingCheckIn()
  startTaskInfoPolling()
  // 处理 X OAuth callback (/callback → redirect /tasks?state=&code=)
  await handleXCallback()
  setTimeout(() => { measure(); scrollRevealHandler() }, 800)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', applyScale)
  window.removeEventListener('resize', scrollRevealHandler)
  window.removeEventListener('scroll', scrollRevealHandler)
  window.removeEventListener('focus', handleTaskPageFocus)
  document.removeEventListener('visibilitychange', handleTaskPageVisibility)
  stopTaskInfoPolling()
})
</script>

<template>
  <!-- 页面级 toast (X OAuth 回跳错误等) -->
  <transition name="jc-page-toast-fade">
    <div v-if="pageToast.open"
         class="jc-page-toast"
         :class="'jc-page-toast--' + pageToast.kind"
         @click="closePageToast">
      <span class="jc-page-toast__msg">{{ pageToast.msg }}</span>
      <button type="button" class="jc-page-toast__close" @click.stop="closePageToast" aria-label="close">×</button>
    </div>
  </transition>

  <JcFloatingNavbar :scale="scale" active="quest" />

  <!-- ========== MOBILE ========== -->
  <div v-if="isMobile" class="jc-mobile-page m-taskhall" :class="{ 'home-dark': isDark }">
    <!-- Hero -->
    <section class="m-hero">
      <div class="m-eyebrow">
        <span class="m-eyebrow-tag"></span>
        <span>{{ t.eyebrow }}</span>
      </div>
      <h1 class="m-hero-title">
        <span class="font-semibold">{{ t.h1L1 }}</span>
        <span class="font-playfair italic font-normal"> {{ t.h1L2 }}</span>
      </h1>
      <p class="m-hero-sub">{{ t.sub }}</p>
      <!-- Stats 3 格竖排 -->
      <div class="m-th-stats">
        <div class="m-th-stat">
          <span class="material-symbols-outlined" style="font-variation-settings:'FILL' 1;color:#1aa64a;">stars</span>
          <div>
            <p class="m-th-stat-label">{{ t.totalPoints }}</p>
            <p class="m-th-stat-value">{{ wallet.isLoggedIn ? displayPoints : '—' }}</p>
          </div>
        </div>
        <div class="m-th-stat">
          <span class="material-symbols-outlined" style="font-variation-settings:'FILL' 1;color:#1aa64a;">event_available</span>
          <div>
            <p class="m-th-stat-label">{{ t.checkIns }}</p>
            <p class="m-th-stat-value">{{ wallet.isLoggedIn ? info.checkInCount : '—' }}</p>
          </div>
        </div>
        <div class="m-th-stat">
          <span class="material-symbols-outlined" style="font-variation-settings:'FILL' 1;color:#3884ff;">local_fire_department</span>
          <div>
            <p class="m-th-stat-label">{{ t.researchStreak }}</p>
            <p class="m-th-stat-value">{{ wallet.isLoggedIn ? info.streak : '—' }} <small>{{ t.days }}</small></p>
          </div>
        </div>
      </div>
    </section>

    <!-- Task 1: Daily Sign-in -->
    <section class="m-th-section">
      <article class="m-th-card m-th-card-daily">
        <div class="m-th-card-head">
          <div>
            <h3>{{ t.dailyTitle }}</h3>
            <p class="m-th-card-sub">{{ t.dailySub }}</p>
          </div>
          <span v-if="info.checkedToday" class="m-th-done-pill">
            <span class="w-dot"></span>{{ t.completed }}
          </span>
        </div>
        <p class="m-th-card-desc">{{ t.dailyDesc }}</p>
        <div class="m-th-streak-badge">
          <span class="material-symbols-outlined" style="font-variation-settings:'FILL' 1;">local_fire_department</span>
          {{ wallet.isLoggedIn ? info.streak : '—' }} {{ t.streakBadge }}
        </div>
        <!-- 日历方格 -->
        <div class="m-th-cal">
          <p class="m-th-cal-label">{{ calMonth.monthLabel }}</p>
          <div class="m-th-cal-grid">
            <div v-for="(c, i) in calMonth.cells" :key="i"
                :class="[
                  'm-th-cal-cell',
                  c.blank ? 'is-blank' : c.isToday ? 'is-today' : c.isChecked ? 'is-checked' : c.isFuture ? 'is-future' : ''
                ]"></div>
          </div>
          <div class="m-th-cal-legend">
            <span><span class="dot dot-signed"></span>Signed</span>
            <span><span class="dot dot-today"></span>Today</span>
          </div>
        </div>
        <div class="m-th-card-foot">
          <div class="m-th-reward">
            <span class="m-th-reward-label">{{ t.reward }}</span>
            <span class="m-th-reward-val">+10 <small>XP</small></span>
          </div>
          <button v-if="!info.checkedToday"
                  type="button"
                  :disabled="!wallet.isLoggedIn || busyDaily"
                  @click="doCheckIn"
                  class="m-th-check-btn">
            <span v-if="busyDaily" class="material-symbols-outlined animate-spin">progress_activity</span>
            {{ busyDaily ? t.checking : t.check }}
          </button>
          <button v-else class="m-th-done-btn" disabled>
            <span class="material-symbols-outlined">check_circle</span>{{ t.checkedIn }}
          </button>
        </div>
        <div v-if="checkInMsg"
             class="px-[18px] pb-[14px] text-[12px] font-semibold text-right break-words"
             :class="checkInMsgColorClass"
             role="status"
             aria-live="polite">{{ checkInMsg }}</div>
        <div v-if="!wallet.isLoggedIn" class="m-th-lock" @click.stop="onLockedConnect">
          Connect Wallet
        </div>
      </article>
    </section>

    <!-- Task 2: Telegram -->
    <section class="m-th-section">
      <article class="m-th-card">
        <div class="m-th-card-head">
          <div class="m-th-card-title-row">
            <span class="m-th-brand m-th-brand-tg">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="white"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm4.64 6.8c-.15 1.58-.8 5.42-1.13 7.19-.14.75-.42 1-.68 1.03-.58.05-1.02-.38-1.58-.75-.88-.58-1.38-.94-2.23-1.5-.99-.65-.35-1.01.22-1.59.15-.15 2.71-2.48 2.76-2.69.01-.03.01-.14-.05-.2-.06-.06-.16-.04-.23-.02-.1.02-1.61 1.02-4.55 3.01-.43.3-.82.44-1.17.43-.39-.01-1.13-.22-1.69-.4-.68-.22-1.22-.34-1.17-.72.02-.2.3-.4.83-.61 3.25-1.42 5.42-2.36 6.51-2.81 3.1-1.29 3.74-1.51 4.16-1.52.09 0 .31.02.44.13.1.09.13.21.14.30.01.10.02.32.02.49z"/></svg>
            </span>
            <div>
              <h3>{{ t.tgKicker }}</h3>
              <p class="m-th-card-sub">{{ t.progress }} {{ tgProgress }}/{{ tgTasks.length }}</p>
            </div>
          </div>
          <div class="m-th-reward">
            <span class="m-th-reward-label">{{ t.reward }}</span>
            <span class="m-th-reward-val m-th-reward-dark">+{{ tgTasks.length * 50 }} <small>XP</small></span>
          </div>
        </div>
        <div class="m-th-progress"><div :style="{ width: (tgProgress / tgTasks.length * 100) + '%' }"></div></div>
        <ol class="m-th-subtasks">
          <li v-for="(task, i) in tgTasks" :key="task.key" class="m-th-sub" :class="{ 'is-done': info.tg[task.key], 'is-locked': isTgTaskLocked(task) }">
            <span class="m-th-sub-num">
              <span v-if="info.tg[task.key]" class="material-symbols-outlined">check</span>
              <span v-else>{{ i + 1 }}</span>
            </span>
            <div class="m-th-sub-body">
              <p class="m-th-sub-title">{{ task.title }}</p>
              <p class="m-th-sub-desc">{{ task.desc }}</p>
              <span class="m-th-sub-pts">+{{ taskDisplayPoints('tg', task) }} XP</span>
            </div>
            <button v-if="!info.tg[task.key]"
                    :disabled="busyTask[`tg.${task.key}`] || isTgTaskLocked(task)"
                    :title="isTgTaskLocked(task) ? t.tgTask_chat_locked : ''"
                    @click="openAndVerify('tg', task.key, task.url, task.points)"
                    class="m-th-sub-btn m-th-sub-btn-tg"
                    :class="{ 'is-prerequisite-locked': isTgTaskLocked(task) }">
              <span v-if="busyTask[`tg.${task.key}`]" class="material-symbols-outlined animate-spin">progress_activity</span>
              <Lock v-else-if="isTgTaskLocked(task)" :size="14" :stroke-width="2.2" aria-hidden="true" />
              {{ busyTask[`tg.${task.key}`] ? t.verifying : (isTgTaskLocked(task) ? t.tgTask_chat_locked_cta : task.cta) }}
            </button>
            <span v-else class="m-th-sub-done"><span class="material-symbols-outlined">check_circle</span>{{ t.done }}</span>
          </li>
        </ol>
        <div v-if="!wallet.isLoggedIn" class="m-th-lock" @click.stop="onLockedConnect">
          Connect Wallet
        </div>
      </article>
    </section>

    <!-- Task 3: X -->
    <section class="m-th-section">
      <article class="m-th-card">
        <div class="m-th-card-head">
          <div class="m-th-card-title-row">
            <span class="m-th-brand m-th-brand-x">𝕏</span>
            <div>
              <h3>{{ t.xKicker }}</h3>
              <p class="m-th-card-sub">{{ t.progress }} {{ xProgress }}/{{ xTasks.length }}</p>
            </div>
          </div>
          <div class="m-th-reward">
            <span class="m-th-reward-label">{{ t.reward }}</span>
            <span class="m-th-reward-val m-th-reward-dark">+{{ xRewardTotal }} <small>XP</small></span>
          </div>
        </div>
        <div v-if="!twitterVerificationDisabled && xBinding.isBound && xBinding.tokenValid" class="m-th-x-binding">
          <span class="m-th-x-bound">
            <span class="material-symbols-outlined" style="font-variation-settings:'FILL' 1;">verified</span>
            @{{ xBinding.twitterHandle }}
          </span>
        </div>
        <div class="m-th-progress"><div :style="{ width: (xProgress / xTasks.length * 100) + '%' }"></div></div>
        <ol class="m-th-subtasks">
          <li v-for="(task, i) in xTasks" :key="taskStateKey(task)" class="m-th-sub" :class="{ 'is-done': isTaskDone('x', task) }">
            <span class="m-th-sub-num">
              <span v-if="isTaskDone('x', task)" class="material-symbols-outlined">check</span>
              <span v-else>{{ i + 1 }}</span>
            </span>
            <div class="m-th-sub-body">
              <p class="m-th-sub-title">{{ task.title }} <small v-if="isXPostTask(task)">· {{ xTargetLabel(task) }}</small></p>
              <p class="m-th-sub-desc">{{ task.desc }}</p>
              <span class="m-th-sub-pts">+{{ task.points }} XP</span>
            </div>
            <button v-if="!twitterVerificationDisabled && !isTaskDone('x', task) && (!xBinding.isBound || !xBinding.tokenValid)"
                    @click.stop="openXBindingFlow"
                    class="m-th-sub-btn m-th-sub-btn-x">
              𝕏 {{ xBinding.isBound ? t.rebindX : t.bindX }}
            </button>
            <template v-else-if="!isTaskDone('x', task)">
              <a :href="task.url"
                 rel="noopener"
                 :aria-disabled="busyTask[taskBusyPath('x', task)] ? 'true' : 'false'"
                 @click="handleXTaskLink($event, task)"
                 class="m-th-sub-btn m-th-sub-btn-x"
                 :class="{ 'is-disabled': busyTask[taskBusyPath('x', task)] }">
                <span v-if="busyTask[taskBusyPath('x', task)]" class="material-symbols-outlined animate-spin">progress_activity</span>
                {{ busyTask[taskBusyPath('x', task)] ? (twitterVerificationDisabled ? t.completing : t.verifying) : (twitterVerificationDisabled ? t.xTask_complete : t.xTask_start) }}
              </a>
            </template>
            <span v-else class="m-th-sub-done"><span class="material-symbols-outlined">check_circle</span>{{ t.done }}</span>
          </li>
        </ol>
        <div v-if="!wallet.isLoggedIn" class="m-th-lock" @click.stop="onLockedConnect">
          Connect Wallet
        </div>
      </article>
    </section>

    <!-- Task 4: Refer Friends -->
    <section class="m-th-section">
      <article class="m-th-card m-th-refer-card">
        <span class="m-th-kicker">{{ t.referKicker }}</span>
        <h3 class="m-th-refer-head">
          <span class="font-semibold">{{ t.referHeadL1 }}</span>
          <span class="font-playfair italic font-normal m-th-refer-accent"> {{ t.referHeadL2 }}</span>
        </h3>
        <p class="m-th-refer-desc">{{ t.referDesc }}</p>
        <div class="m-th-refer-stats">
          <div>
            <p class="m-th-stat-label">{{ t.succRef }}</p>
            <p class="m-th-stat-value">{{ wallet.isLoggedIn ? info.invitedCount : '—' }}</p>
          </div>
          <div class="m-th-refer-earned">
            <p class="m-th-stat-label">{{ t.earnedPoints }}</p>
            <p class="m-th-stat-value">{{ wallet.isLoggedIn ? '+' + info.invitedPoints : '—' }}</p>
          </div>
        </div>
        <div class="m-th-refer-box">
          <div class="m-th-refer-field">
            <label>{{ t.refCode }}</label>
            <div class="m-th-refer-input">
              <input readonly type="text" :value="info.inviteCode || '—'" />
              <button class="m-th-refer-copy-btn" @click="copy(info.inviteCode, 'code')" :disabled="!info.inviteCode">
                <span class="material-symbols-outlined">{{ copyState.code ? 'check' : 'content_copy' }}</span>
                {{ copyState.code ? t.copied : t.copy }}
              </button>
            </div>
          </div>
          <div class="m-th-refer-field">
            <label>{{ t.refLink }}</label>
            <div class="m-th-refer-input">
              <input readonly type="text" :value="refLink" />
              <button class="m-th-refer-copy-btn" @click="copy(refLink, 'link')" :disabled="!info.inviteCode">
                <span class="material-symbols-outlined">{{ copyState.link ? 'check' : 'content_copy' }}</span>
                {{ copyState.link ? t.copied : t.copy }}
              </button>
            </div>
          </div>
          <div class="m-th-refer-share">
            <button @click="shareX" class="m-th-share-x">𝕏 {{ t.shareX }}</button>
            <button @click="shareTg" class="m-th-share-tg">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="white"><path d="M9.78 18.65l.28-4.23 7.68-6.92c.34-.31-.07-.46-.52-.19L7.74 13.24 3.64 11.95c-.88-.25-.89-.86.2-1.3l15.97-6.16c.73-.33 1.43.18 1.15 1.3l-2.72 12.81c-.19.91-.74 1.13-1.5.71L12.6 16.3l-1.99 1.93c-.23.23-.42.42-.83.42z"/></svg>
              {{ t.shareTg }}
            </button>
          </div>
        </div>
        <div v-if="!wallet.isLoggedIn" class="m-th-lock" @click.stop="onLockedConnect">
          Connect Wallet
        </div>
      </article>
    </section>

    <!-- Points / Rewards Mechanics -->
    <section class="m-section">
      <div class="m-section-head">
        <span class="m-eyebrow-tag"></span>
        <h2 class="m-section-title">{{ t.howKicker }}</h2>
      </div>
      <h3 class="m-section-headline">{{ t.howHead }}</h3>
      <p class="m-section-copy">{{ t.howDesc }}</p>
      <div class="m-th-formula">
        <span class="m-th-formula-label">{{ t.howFormula }}</span>
        <span class="m-th-formula-val">{{ t.howFormulaVal }}</span>
      </div>
      <div class="m-card-grid">
        <article class="m-card"><span class="m-card-icon-tag" style="background:#9bff00;">⚡</span><h4>{{ t.howBullet1 }}</h4><p>{{ t.howBullet1Desc }}</p></article>
        <article class="m-card"><span class="m-card-icon-tag" style="background:#212121;color:white;">🥧</span><h4>{{ t.howBullet2 }}</h4><p>{{ t.howBullet2Desc }}</p></article>
        <article class="m-card"><span class="m-card-icon-tag" style="background:#fff7e6;">♾️</span><h4>{{ t.howBullet3 }}</h4><p>{{ t.howBullet3Desc }}</p></article>
        <article class="m-card"><span class="m-card-icon-tag" style="background:#e6f0ff;">🛡️</span><h4>{{ t.howBullet4 }}</h4><p>{{ t.howBullet4Desc }}</p></article>
      </div>
    </section>

    <HomeFooter :language="props.language" />
  </div>

  <!-- ========== DESKTOP ========== -->
  <div v-else class="jc-canvas-host" :class="{ 'home-dark': isDark }" :style="{ height: hostHeight }">
    <div class="jc-scaled-wrapper" :style="{ width: `${1440 * scale}px`, height: hostHeight }">
      <main ref="canvasRef" class="jc-canvas-1440" :style="{ transform: `scale(${scale})` }">

        <div class="h-[64px] w-full shrink-0"></div>

        <!-- ============ 合并：HERO + WORKSPACE（eyebrow + headline + sub/CTA + stats + 任务卡） ============ -->
        <section data-reveal class="relative w-full px-[80px] pt-[60px] pb-[120px] flex flex-col gap-[40px]">
          <!-- 方块组：上移到主标题右侧 -->
          <div class="absolute pointer-events-none z-10" style="top: 80px; right: 80px;">
            <PixelGrid :cols="12" :rows="12" cell-size="14px" gap="0" color="#9bff00"
                       :flash-duration="450" :idle-min="900" :idle-max="2800"
                       :start-stagger-max="2800" :jitter="0" :jitter-rotate="0" :jitter-scale="0" />
          </div>

          <!-- Eyebrow -->
          <div class="jc-hero-anim jc-hero-anim-1 flex items-center gap-[8px]">
            <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
            <span class="text-[20px] font-bold text-[#212121]">{{ t.eyebrow }}</span>
          </div>

          <!-- 88px 大标题 -->
          <div class="font-bold text-[#212121] tracking-[-2.88px]" style="font-size: 88px; line-height: 0.95; width: 1180px;">
            <p class="jc-hero-headline-line jc-hero-headline-line-1 mb-0">
              <span class="font-semibold">{{ t.h1L1 }}</span>
            </p>
            <p class="jc-hero-headline-line jc-hero-headline-line-2 font-semibold">
              <span class="font-playfair italic font-normal">{{ t.h1L2 }}</span>
            </p>
          </div>

          <!-- sub + 紧凑 stats + Connect Wallet（三列同行）-->
          <div class="jc-hero-anim jc-hero-anim-3 flex items-end justify-between gap-[32px]">
            <p class="text-[18px] font-bold text-[#212121] leading-[1.4]" style="width: 420px;">{{ t.sub }}</p>

            <!-- Stats 紧凑横排 -->
            <div class="rise-card rise-card-1 bg-white flex items-stretch rounded-[12px] overflow-hidden shrink-0">
              <div class="flex flex-col items-start gap-[8px] px-[24px] py-[16px] relative overflow-hidden border-r border-[#eee] min-w-[140px]">
                <div class="flex items-center gap-[6px]">
                  <span class="material-symbols-outlined text-[#1aa64a] text-[14px]" style="font-variation-settings: 'FILL' 1;">stars</span>
                  <span class="text-[9px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.totalPoints }}</span>
                </div>
                <span class="text-[30px] font-bold text-[#212121] leading-none tabular-nums">{{ wallet.isLoggedIn ? displayPoints : '—' }}</span>
                <div class="absolute -bottom-8 -right-8 w-[80px] h-[80px] bg-[#9bff00]/30 rounded-full blur-2xl pointer-events-none"></div>
              </div>
              <div class="flex flex-col items-start gap-[8px] px-[24px] py-[16px] border-r border-[#eee] min-w-[120px]">
                <div class="flex items-center gap-[6px]">
                  <span class="material-symbols-outlined text-[#1aa64a] text-[14px]" style="font-variation-settings: 'FILL' 1;">event_available</span>
                  <span class="text-[9px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.checkIns }}</span>
                </div>
                <span class="text-[30px] font-bold text-[#212121] leading-none">{{ wallet.isLoggedIn ? info.checkInCount : '—' }}</span>
              </div>
              <div class="flex flex-col items-start gap-[8px] px-[24px] py-[16px] min-w-[120px]">
                <div class="flex items-center gap-[6px]">
                  <span class="material-symbols-outlined text-[#3884ff] text-[14px]" style="font-variation-settings: 'FILL' 1;">local_fire_department</span>
                  <span class="text-[9px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.researchStreak }}</span>
                </div>
                <div class="flex items-baseline gap-[6px]">
                  <span class="text-[30px] font-bold text-[#212121] leading-none">{{ wallet.isLoggedIn ? info.streak : '—' }}</span>
                  <span class="text-[9px] font-bold tracking-[1px] uppercase text-[#808080]">{{ t.days }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- 视觉分隔（淡淡 1px 线） -->
          <div class="pointer-events-none" style="height: 1px; background: linear-gradient(to right, transparent 0%, rgba(0,0,0,0.08) 12%, rgba(0,0,0,0.08) 88%, transparent 100%);"></div>

          <!-- ====== Task 1：Daily Sign-in（带本月日历方格）====== -->
          <article class="task-card rise-card rise-card-2 bg-white flex items-stretch p-[28px] rounded-[12px] gap-[32px]">
            <!-- 左：标题 + 描述 + 连签徽章 -->
            <div class="flex flex-col gap-[10px] w-[260px] shrink-0">
              <div class="flex items-center gap-[10px]">
                <h3 class="text-[22px] font-bold text-[#212121] leading-none">{{ t.dailyTitle }}</h3>
                <span v-if="info.checkedToday" class="inline-flex items-center gap-[6px] px-[10px] py-[4px] rounded-full bg-[#9bff00]/30 text-[10px] font-bold uppercase tracking-[1.2px] text-[#1aa64a]">
                  <span class="w-[6px] h-[6px] rounded-full bg-[#1aa64a] animate-pulse"></span>{{ t.completed }}
                </span>
              </div>
              <p class="text-[11px] font-bold uppercase tracking-[1.2px] text-[#808080]">{{ t.dailySub }}</p>
              <p class="text-[13px] font-medium text-[#212121] leading-[1.5]">{{ t.dailyDesc }}</p>
              <!-- 连签徽章 -->
              <div class="mt-[4px] inline-flex items-center gap-[6px] px-[10px] py-[6px] rounded-full bg-[#fff7e6] border border-[#ffd591] self-start">
                <span class="material-symbols-outlined text-[#fa8c16] text-[14px]" style="font-variation-settings: 'FILL' 1;">local_fire_department</span>
                <span class="text-[12px] font-bold text-[#fa8c16]">{{ wallet.isLoggedIn ? info.streak : '—' }} {{ t.streakBadge }}</span>
              </div>
            </div>

            <!-- 中部弹性间隔（让日历贴右）-->
            <div class="flex-1"></div>

            <!-- 右侧前：本月日历方格（贴右排列，将来添加历史月份会向左延伸）-->
            <!-- 多月容器：水平排列，items-end，新月在右、旧月会被插到 *最前面* (unshift) -->
            <div class="flex items-end gap-[16px] shrink-0">
              <!-- 本月 -->
              <div class="flex flex-col gap-[6px] items-center">
                <span class="text-[10px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ calMonth.monthLabel }}</span>
                <div class="grid grid-cols-7 gap-[3px]" style="width: fit-content;">
                  <div v-for="(c, i) in calMonth.cells" :key="i"
                    :data-day="c.day"
                    :class="[
                      'cal-cell w-[14px] h-[14px] rounded-[2px]',
                      c.blank
                        ? 'cal-blank opacity-0'
                        : c.isToday
                          ? 'bg-[#212121] ring-[1.5px] ring-[#9bff00] ring-offset-[1px]'
                          : c.isChecked
                            ? 'bg-[#9bff00]'
                            : c.isFuture
                              ? 'bg-[#f0efec]'
                              : 'bg-[#e2e2de]'
                    ]"
                  ></div>
                </div>
                <div class="flex items-center gap-[10px] text-[9px] font-medium text-[#808080]">
                  <span class="inline-flex items-center gap-[3px]"><span class="w-[6px] h-[6px] rounded-[1px] bg-[#9bff00]"></span>Signed</span>
                  <span class="inline-flex items-center gap-[3px]"><span class="w-[6px] h-[6px] rounded-[1px] bg-[#212121]"></span>Today</span>
                </div>
              </div>
            </div>

            <!-- 右：reward + sign 按钮 -->
            <div class="flex flex-col items-end justify-between shrink-0 w-[160px]">
              <div class="text-right">
                <div class="text-[10px] font-bold tracking-[1.5px] uppercase text-[#808080]">{{ t.reward }}</div>
                <div class="text-[20px] font-bold text-[#1aa64a]">+10 <span class="text-[12px] text-[#808080]">XP</span></div>
              </div>
              <button
                v-if="!info.checkedToday"
                type="button"
                :disabled="!wallet.isLoggedIn || busyDaily"
                @click="doCheckIn"
                class="white-pill-btn cta-pulse disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <span v-if="busyDaily" class="material-symbols-outlined animate-spin text-[18px]">progress_activity</span>
                <span class="text-[14px] font-bold text-[#212121]">{{ busyDaily ? t.checking : t.check }}</span>
                <span class="arrow-icon arrow-diag">
                  <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                  <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                </span>
              </button>
              <button v-else class="inline-flex items-center gap-[8px] px-[14px] py-[10px] rounded-[7px] bg-[#9bff00]/30 cursor-not-allowed" disabled>
                <span class="material-symbols-outlined text-[#1aa64a] text-[16px]">check_circle</span>
                <span class="text-[13px] font-bold text-[#1aa64a]">{{ t.checkedIn }}</span>
              </button>
              <div v-if="checkInMsg"
                   class="mt-[6px] text-[11px] font-medium text-right break-words"
                   :class="checkInMsgColorClass"
                   role="status"
                   aria-live="polite">{{ checkInMsg }}</div>
            </div>
            <div v-if="!wallet.isLoggedIn" class="lock-overlay" @click.stop="onLockedConnect">
              <button class="cw-btn" @click.stop="onLockedConnect">
                <span class="cw-deco cw-wave-wrap">
                  <span class="cw-wave"><img :src="img('wave-back.svg')" alt="" /></span>
                </span>
                <span class="cw-deco cw-crest cw-crest-r"><img :src="img('crest-r.svg')" alt="" /></span>
                <span class="cw-deco cw-crest cw-crest-l"><img :src="img('crest-r.svg')" alt="" /></span>
                <span class="cw-label">Connect Wallet</span>
              </button>
            </div>
          </article>

          <!-- ====== Task 2：Telegram 任务组 ====== -->
          <article class="task-card rise-card rise-card-3 bg-white flex flex-col gap-[18px] p-[28px] rounded-[12px]">
            <header class="flex items-center justify-between gap-[24px]">
              <div class="flex items-center gap-[12px]">
                <span class="inline-flex w-[36px] h-[36px] items-center justify-center rounded-[8px] bg-[#229ED9]">
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="white" xmlns="http://www.w3.org/2000/svg"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm4.64 6.8c-.15 1.58-.8 5.42-1.13 7.19-.14.75-.42 1-.68 1.03-.58.05-1.02-.38-1.58-.75-.88-.58-1.38-.94-2.23-1.5-.99-.65-.35-1.01.22-1.59.15-.15 2.71-2.48 2.76-2.69.01-.03.01-.14-.05-.2-.06-.06-.16-.04-.23-.02-.1.02-1.61 1.02-4.55 3.01-.43.3-.82.44-1.17.43-.39-.01-1.13-.22-1.69-.4-.68-.22-1.22-.34-1.17-.72.02-.2.3-.4.83-.61 3.25-1.42 5.42-2.36 6.51-2.81 3.1-1.29 3.74-1.51 4.16-1.52.09 0 .31.02.44.13.1.09.13.21.14.30.01.10.02.32.02.49z"/></svg>
                </span>
                <div class="flex flex-col gap-[2px]">
                  <div class="flex items-center gap-[10px]">
                    <h3 class="text-[22px] font-bold text-[#212121] leading-none">{{ t.tgKicker }}</h3>
                    <span class="text-[11px] font-bold text-[#808080]">{{ t.progress }} {{ tgProgress }}/{{ tgTasks.length }}</span>
                  </div>
                  <p class="text-[13px] font-medium text-[#808080] leading-[1.4]">{{ t.tgDesc }}</p>
                </div>
              </div>
              <div class="text-right">
                <div class="text-[10px] font-bold tracking-[1.5px] uppercase text-[#808080]">{{ t.reward }}</div>
                <div class="text-[18px] font-bold text-[#212121]">+{{ tgTasks.length * 50 }} <span class="text-[11px] text-[#808080]">XP</span></div>
              </div>
            </header>

            <!-- TG 进度条 -->
            <div class="progress-track">
              <div class="progress-fill" :style="{ width: (tgProgress / tgTasks.length * 100) + '%' }"></div>
            </div>

            <ol class="flex flex-col gap-[10px]">
              <li v-for="(task, i) in tgTasks" :key="task.key"
                  :class="['subtask-row flex items-center gap-[16px] py-[12px] px-[14px] rounded-[8px] bg-[#fafaf8] border border-[#eee]', { 'just-done': recentlyDone === `tg.${task.key}`, 'done-row': info.tg[task.key], 'is-locked': isTgTaskLocked(task) }]">
                <span :class="[
                  'inline-flex w-[26px] h-[26px] items-center justify-center rounded-full text-[12px] font-bold shrink-0',
                  info.tg[task.key] ? 'bg-[#229ED9] text-white' : 'bg-white border border-[#ddd] text-[#808080]'
                ]">
                  <span v-if="info.tg[task.key]" class="material-symbols-outlined text-[14px]" style="font-variation-settings: 'FILL' 1;">check</span>
                  <span v-else>{{ i + 1 }}</span>
                </span>
                <div class="flex flex-col flex-1 min-w-0">
                  <div class="flex items-center gap-[8px]">
                    <span class="text-[14px] font-bold text-[#212121]">{{ task.title }}</span>
                    <a :href="task.url" target="_blank" rel="noopener" class="text-[11px] font-medium text-[#3884ff] truncate hover:underline max-w-[260px]">{{ task.url }}</a>
                  </div>
                  <span class="text-[12px] text-[#808080]">{{ task.desc }}</span>
                </div>
                <span class="text-[11px] font-bold text-[#1aa64a]">+{{ taskDisplayPoints('tg', task) }} XP</span>
                <button
                  v-if="!info.tg[task.key]"
                  :disabled="busyTask[`tg.${task.key}`] || isTgTaskLocked(task)"
                  :title="isTgTaskLocked(task) ? t.tgTask_chat_locked : ''"
                  @click="openAndVerify('tg', task.key, task.url, task.points)"
                  class="tg-task-btn inline-flex items-center gap-[6px] px-[12px] py-[8px] rounded-[6px] bg-[#229ED9] hover:bg-[#1e8bbe] text-[12px] font-bold text-white disabled:opacity-60"
                  :class="{ 'is-prerequisite-locked': isTgTaskLocked(task) }"
                >
                  <span v-if="busyTask[`tg.${task.key}`]" class="material-symbols-outlined animate-spin text-[14px]">progress_activity</span>
                  <Lock v-else-if="isTgTaskLocked(task)" :size="14" :stroke-width="2.2" aria-hidden="true" />
                  {{ busyTask[`tg.${task.key}`] ? t.verifying : (isTgTaskLocked(task) ? t.tgTask_chat_locked_cta : task.cta) }}
                </button>
                <span v-else class="inline-flex items-center gap-[4px] px-[10px] py-[6px] rounded-[6px] bg-[#9bff00]/30 text-[11px] font-bold text-[#1aa64a]">
                  <span class="material-symbols-outlined text-[14px]">check_circle</span>{{ t.done }}
                </span>
              </li>
            </ol>
            <div v-if="!wallet.isLoggedIn" class="lock-overlay" @click.stop="onLockedConnect">
              <button class="cw-btn" @click.stop="onLockedConnect">
                <span class="cw-deco cw-wave-wrap">
                  <span class="cw-wave"><img :src="img('wave-back.svg')" alt="" /></span>
                </span>
                <span class="cw-deco cw-crest cw-crest-r"><img :src="img('crest-r.svg')" alt="" /></span>
                <span class="cw-deco cw-crest cw-crest-l"><img :src="img('crest-r.svg')" alt="" /></span>
                <span class="cw-label">Connect Wallet</span>
              </button>
            </div>
          </article>

          <!-- ====== Task 3：X / Twitter 多步任务组 ====== -->
          <article class="task-card rise-card rise-card-4 bg-white flex flex-col gap-[18px] p-[28px] rounded-[12px]">
            <header class="flex items-center justify-between gap-[24px]">
              <div class="flex items-center gap-[12px]">
                <span class="inline-flex w-[36px] h-[36px] items-center justify-center rounded-[8px] bg-[#212121]">
                  <span class="text-[14px] font-bold text-white">𝕏</span>
                </span>
                <div class="flex flex-col gap-[2px]">
                  <div class="flex items-center gap-[10px]">
                    <h3 class="text-[22px] font-bold text-[#212121] leading-none">{{ t.xKicker }}</h3>
                    <span class="text-[11px] font-bold text-[#808080]">{{ t.progress }} {{ xProgress }}/{{ xTasks.length }}</span>
                  </div>
                  <p class="text-[13px] font-medium text-[#808080] leading-[1.4]">{{ t.xDesc }}</p>
                </div>
              </div>
              <div class="flex items-center gap-[16px]">
                <!-- X 绑定状态 · pill -->
                <div v-if="!twitterVerificationDisabled && xBinding.isBound && xBinding.tokenValid" class="flex flex-col items-end gap-[4px]">
                  <span class="inline-flex items-center gap-[6px] px-[10px] py-[5px] rounded-full bg-[#9bff00]/30 text-[11px] font-bold uppercase tracking-[1.2px] text-[#1aa64a]">
                    <span class="material-symbols-outlined text-[13px]" style="font-variation-settings: 'FILL' 1;">verified</span>
                    @{{ xBinding.twitterHandle }}
                  </span>
                </div>
                <div class="text-right">
                  <div class="text-[10px] font-bold tracking-[1.5px] uppercase text-[#808080]">{{ t.reward }}</div>
                  <div class="text-[18px] font-bold text-[#212121]">+{{ xRewardTotal }} <span class="text-[11px] text-[#808080]">XP</span></div>
                </div>
              </div>
            </header>

            <!-- X 进度条 -->
            <div class="progress-track">
              <div class="progress-fill" :style="{ width: (xProgress / xTasks.length * 100) + '%' }"></div>
            </div>

            <ol class="flex flex-col gap-[10px]">
              <li v-for="(task, i) in xTasks" :key="taskStateKey(task)"
                  :class="['subtask-row flex items-center gap-[16px] py-[12px] px-[14px] rounded-[8px] bg-[#fafaf8] border border-[#eee]', { 'just-done': recentlyDone === taskBusyPath('x', task), 'done-row': isTaskDone('x', task) }]">
                <span :class="[
                  'inline-flex w-[26px] h-[26px] items-center justify-center rounded-full text-[12px] font-bold shrink-0',
                  isTaskDone('x', task) ? 'bg-[#9bff00] text-[#212121]' : 'bg-white border border-[#ddd] text-[#808080]'
                ]">
                  <span v-if="isTaskDone('x', task)" class="material-symbols-outlined text-[14px]" style="font-variation-settings: 'FILL' 1;">check</span>
                  <span v-else>{{ i + 1 }}</span>
                </span>
                <div class="flex flex-col flex-1 min-w-0">
                  <div class="flex items-center gap-[8px]">
                    <span class="text-[14px] font-bold text-[#212121]">{{ task.title }}</span>
                    <span v-if="isXPostTask(task)" class="rounded-full bg-[#eef4ff] px-[7px] py-[2px] text-[10px] font-bold text-[#3884ff]">{{ xTargetLabel(task) }}</span>
                    <a :href="task.url" target="_blank" rel="noopener" class="text-[11px] font-medium text-[#3884ff] truncate hover:underline max-w-[260px]">{{ task.url }}</a>
                  </div>
                  <span class="text-[12px] text-[#808080]">{{ task.desc }}</span>
                </div>
                <span class="text-[11px] font-bold text-[#1aa64a]">+{{ task.points }} XP</span>
                <!-- 未绑定 X → 每个子任务按钮变成"绑定 X"，点击直接触发 OAuth；已绑定后恢复原 verify -->
                <button
                  v-if="!twitterVerificationDisabled && !isTaskDone('x', task) && (!xBinding.isBound || !xBinding.tokenValid)"
                  @click.stop="openXBindingFlow"
                  class="inline-flex items-center gap-[6px] px-[12px] py-[8px] rounded-[6px] bg-[#212121] hover:bg-black text-[12px] font-bold text-white"
                >
                  <span class="text-[13px]">𝕏</span>
                  {{ xBinding.isBound ? t.rebindX : t.bindX }}
                </button>
                <template v-else-if="!isTaskDone('x', task)">
                  <a
                    :href="task.url"
                    rel="noopener"
                    :aria-disabled="busyTask[taskBusyPath('x', task)] ? 'true' : 'false'"
                    @click="handleXTaskLink($event, task)"
                    class="inline-flex items-center gap-[6px] px-[12px] py-[8px] rounded-[6px] bg-[#212121] hover:bg-black text-[12px] font-bold text-white no-underline"
                    :class="{ 'pointer-events-none opacity-60': busyTask[taskBusyPath('x', task)] }"
                  >
                    <span v-if="busyTask[taskBusyPath('x', task)]" class="material-symbols-outlined animate-spin text-[14px]">progress_activity</span>
                    {{ busyTask[taskBusyPath('x', task)] ? (twitterVerificationDisabled ? t.completing : t.verifying) : (twitterVerificationDisabled ? t.xTask_complete : t.xTask_start) }}
                  </a>
                </template>
                <span v-else class="inline-flex items-center gap-[4px] px-[10px] py-[6px] rounded-[6px] bg-[#9bff00]/30 text-[11px] font-bold text-[#1aa64a]">
                  <span class="material-symbols-outlined text-[14px]">check_circle</span>{{ t.done }}
                </span>
              </li>
            </ol>
            <div v-if="!wallet.isLoggedIn" class="lock-overlay" @click.stop="onLockedConnect">
              <button class="cw-btn" @click.stop="onLockedConnect">
                <span class="cw-deco cw-wave-wrap">
                  <span class="cw-wave"><img :src="img('wave-back.svg')" alt="" /></span>
                </span>
                <span class="cw-deco cw-crest cw-crest-r"><img :src="img('crest-r.svg')" alt="" /></span>
                <span class="cw-deco cw-crest cw-crest-l"><img :src="img('crest-r.svg')" alt="" /></span>
                <span class="cw-label">Connect Wallet</span>
              </button>
            </div>
          </article>

          <!-- ====== Task 4：Refer Friends（视觉增强：渐变背景 + 分享卡 + 社交按钮）====== -->
          <article class="rise-card refer-card relative bg-white flex flex-col gap-[24px] p-[36px] rounded-[16px] overflow-hidden">
            <!-- 装饰渐变 -->
            <div class="refer-bloom"></div>

            <!-- 方块组：Refer 区域标题右侧空白处 -->
            <div class="absolute pointer-events-none z-[5]" style="top: 36px; right: 360px;">
              <PixelGrid :cols="10" :rows="6" cell-size="12px" gap="0" color="#9bff00"
                         :flash-duration="450" :idle-min="900" :idle-max="2800"
                         :start-stagger-max="2800" :jitter="0" :jitter-rotate="0" :jitter-scale="0" />
            </div>

            <div class="flex items-start justify-between gap-[24px] relative z-10">
              <div class="flex flex-col gap-[10px] flex-1">
                <span class="inline-flex items-center gap-[6px] px-[10px] py-[4px] rounded-full bg-[#212121] text-[10px] font-bold uppercase tracking-[1.2px] text-[#9bff00] self-start">
                  <span class="w-[6px] h-[6px] rounded-full bg-[#9bff00] animate-pulse"></span>{{ t.referKicker }}
                </span>
                <h3 class="text-[28px] font-bold text-[#212121] leading-tight">
                  <span class="font-semibold">{{ t.referHeadL1 }}</span>
                  <span class="font-playfair italic font-normal text-[#1aa64a]"> {{ t.referHeadL2 }}</span>
                </h3>
                <p class="text-[14px] font-medium text-[#212121] leading-[1.5] max-w-[600px]">{{ t.referDesc }}</p>
              </div>
              <!-- 战绩计数器 -->
              <div class="flex gap-[20px] shrink-0">
                <div class="flex flex-col items-end gap-[2px] bg-white/80 backdrop-blur rounded-[10px] px-[18px] py-[14px] shadow-[0_4px_14px_rgba(0,0,0,0.04)]">
                  <span class="text-[10px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.succRef }}</span>
                  <span class="text-[28px] font-bold text-[#212121] leading-none">{{ wallet.isLoggedIn ? info.invitedCount : '—' }}</span>
                </div>
                <div class="earned-pill flex flex-col items-end gap-[2px] bg-[#9bff00] rounded-[10px] px-[18px] py-[14px]">
                  <span class="text-[10px] font-bold tracking-[1.2px] uppercase text-[#212121]/70">{{ t.earnedPoints }}</span>
                  <span class="text-[28px] font-bold text-[#212121] leading-none">{{ wallet.isLoggedIn ? '+' + info.invitedPoints : '—' }}</span>
                </div>
              </div>
            </div>

            <!-- 邀请代码 + 链接 + 一键分享 -->
            <div class="relative z-10 flex flex-col gap-[14px] bg-[#212121] rounded-[14px] p-[20px]">
              <div class="flex gap-[12px]">
                <div class="flex flex-col gap-[6px] w-[180px] shrink-0">
                  <label class="text-[10px] font-bold tracking-[1.5px] uppercase text-[#9bff00]">{{ t.refCode }}</label>
                  <div class="relative">
                    <input
                      class="w-full bg-white/10 rounded-[8px] py-[12px] pl-[14px] pr-[70px] text-[15px] text-white font-bold tracking-[1px] focus:outline-none focus:ring-2 focus:ring-[#9bff00]/50"
                      readonly type="text" :value="info.inviteCode || '—'"
                    />
                    <button
                      @click="copy(info.inviteCode, 'code')"
                      :disabled="!info.inviteCode"
                      class="refer-copy-btn absolute right-[5px] top-[5px] bottom-[5px] inline-flex items-center gap-[4px] px-[10px] bg-[#9bff00] rounded-[6px] text-[11px] font-bold text-[#212121] hover:bg-[#5fd189] transition-colors disabled:opacity-50"
                    >
                      <span class="material-symbols-outlined text-[13px]">{{ copyState.code ? 'check' : 'content_copy' }}</span>
                      {{ copyState.code ? t.copied : t.copy }}
                    </button>
                  </div>
                </div>
                <div class="flex flex-col gap-[6px] flex-1">
                  <label class="text-[10px] font-bold tracking-[1.5px] uppercase text-[#9bff00]">{{ t.refLink }}</label>
                  <div class="relative">
                    <input
                      class="w-full bg-white/10 rounded-[8px] py-[12px] pl-[14px] pr-[90px] text-[13px] text-white font-medium focus:outline-none focus:ring-2 focus:ring-[#9bff00]/50 truncate"
                      readonly type="text" :value="refLink"
                    />
                    <button
                      @click="copy(refLink, 'link')"
                      :disabled="!info.inviteCode"
                      class="refer-copy-btn absolute right-[5px] top-[5px] bottom-[5px] inline-flex items-center gap-[4px] px-[10px] bg-[#9bff00] rounded-[6px] text-[11px] font-bold text-[#212121] hover:bg-[#5fd189] transition-colors disabled:opacity-50"
                    >
                      <span class="material-symbols-outlined text-[13px]">{{ copyState.link ? 'check' : 'content_copy' }}</span>
                      {{ copyState.link ? t.copied : t.copy }}
                    </button>
                  </div>
                </div>
              </div>
              <!-- 一键分享到 X / TG -->
              <div class="flex items-center gap-[12px] pt-[6px]">
                <span class="text-[11px] font-medium text-[#999]">One-tap share →</span>
                <button @click="shareX" class="inline-flex items-center gap-[6px] px-[14px] py-[8px] rounded-[7px] bg-white hover:bg-[#f5f5f3] text-[12px] font-bold text-[#212121] transition-colors">
                  <span class="text-[14px] font-bold">𝕏</span>{{ t.shareX }}
                </button>
                <button @click="shareTg" class="inline-flex items-center gap-[6px] px-[14px] py-[8px] rounded-[7px] bg-[#229ED9] hover:bg-[#1e8bbe] text-[12px] font-bold text-white transition-colors">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="white" xmlns="http://www.w3.org/2000/svg"><path d="M9.78 18.65l.28-4.23 7.68-6.92c.34-.31-.07-.46-.52-.19L7.74 13.24 3.64 11.95c-.88-.25-.89-.86.2-1.3l15.97-6.16c.73-.33 1.43.18 1.15 1.3l-2.72 12.81c-.19.91-.74 1.13-1.5.71L12.6 16.3l-1.99 1.93c-.23.23-.42.42-.83.42z"/></svg>
                  {{ t.shareTg }}
                </button>
              </div>
            </div>
            <div v-if="!wallet.isLoggedIn" class="lock-overlay" @click.stop="onLockedConnect">
              <button class="cw-btn" @click.stop="onLockedConnect">
                <span class="cw-deco cw-wave-wrap">
                  <span class="cw-wave"><img :src="img('wave-back.svg')" alt="" /></span>
                </span>
                <span class="cw-deco cw-crest cw-crest-r"><img :src="img('crest-r.svg')" alt="" /></span>
                <span class="cw-deco cw-crest cw-crest-l"><img :src="img('crest-r.svg')" alt="" /></span>
                <span class="cw-label">Connect Wallet</span>
              </button>
            </div>
          </article>
        </section>

        <!-- ============ Points / Reward Mechanics 说明 ============ -->
        <section data-reveal class="relative w-full flex flex-col gap-[48px] px-[80px] pt-[100px] pb-[120px]">
          <div class="reveal-eyebrow flex items-center gap-[8px]">
            <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
            <span class="text-[20px] font-bold text-[#212121]">{{ t.howKicker }}</span>
          </div>

          <p class="text-[32px] font-bold text-[#212121] tracking-[-0.64px] leading-[1.25]" style="width: 980px;">
            {{ t.howHead }}
          </p>

          <p class="text-[15px] font-medium text-[#212121] leading-[1.7]" style="width: 900px;">
            {{ t.howDesc }}
          </p>

          <!-- 公式条：暗底高亮，单独留呼吸空间 -->
          <div class="rise-card rise-card-1 bg-[#212121] flex items-center gap-[24px] px-[32px] py-[22px] rounded-[12px] self-start mt-[8px]">
            <span class="text-[11px] font-bold tracking-[1.5px] uppercase text-[#9bff00]">{{ t.howFormula }}</span>
            <span class="font-mono text-[16px] font-bold text-white">{{ t.howFormulaVal }}</span>
          </div>

          <!-- 4 个机制卡：卡间 24px，卡内 28px gap + 32px padding -->
          <div class="grid grid-cols-4 gap-[24px] w-full mt-[16px]">
            <article class="rise-card rise-card-2 bg-white flex flex-col gap-[18px] p-[32px] rounded-[12px]">
              <span class="inline-flex w-[48px] h-[48px] items-center justify-center rounded-[12px] bg-[#9bff00]">
                <span class="text-[26px]">⚡</span>
              </span>
              <h4 class="text-[17px] font-bold text-[#212121] leading-tight">{{ t.howBullet1 }}</h4>
              <p class="text-[13px] font-medium text-[#212121] leading-[1.7] opacity-80">{{ t.howBullet1Desc }}</p>
            </article>
            <article class="rise-card rise-card-3 bg-white flex flex-col gap-[18px] p-[32px] rounded-[12px]">
              <span class="inline-flex w-[48px] h-[48px] items-center justify-center rounded-[12px] bg-[#212121]">
                <span class="text-[26px]">🥧</span>
              </span>
              <h4 class="text-[17px] font-bold text-[#212121] leading-tight">{{ t.howBullet2 }}</h4>
              <p class="text-[13px] font-medium text-[#212121] leading-[1.7] opacity-80">{{ t.howBullet2Desc }}</p>
            </article>
            <article class="rise-card rise-card-4 bg-white flex flex-col gap-[18px] p-[32px] rounded-[12px]">
              <span class="inline-flex w-[48px] h-[48px] items-center justify-center rounded-[12px] bg-[#fff7e6] border border-[#ffd591]">
                <span class="text-[24px]">♾️</span>
              </span>
              <h4 class="text-[17px] font-bold text-[#212121] leading-tight">{{ t.howBullet3 }}</h4>
              <p class="text-[13px] font-medium text-[#212121] leading-[1.7] opacity-80">{{ t.howBullet3Desc }}</p>
            </article>
            <article class="rise-card rise-card-4 bg-white flex flex-col gap-[18px] p-[32px] rounded-[12px]">
              <span class="inline-flex w-[48px] h-[48px] items-center justify-center rounded-[12px] bg-[#e6f0ff] border border-[#91b8ff]">
                <span class="text-[24px]">🛡️</span>
              </span>
              <h4 class="text-[17px] font-bold text-[#212121] leading-tight">{{ t.howBullet4 }}</h4>
              <p class="text-[13px] font-medium text-[#212121] leading-[1.7] opacity-80">{{ t.howBullet4Desc }}</p>
            </article>
          </div>
        </section>

        <HomeFooter :language="props.language" full-bleed :scale="scale" />

      </main>
    </div>
  </div>

  <!-- === X 任務 Modal：所有動作均由使用者手動完成，JoinCare 僅做唯讀驗證 === -->
  <div v-if="xVerifyModal.open" class="reply-modal-overlay" :class="{ 'is-dark': isDark }" @click.self="closeXVerifyModal">
    <div class="reply-modal">
      <div class="reply-modal-header">
        <span class="material-symbols-outlined" style="color:#3884ff; font-size:22px;">verified_user</span>
        <span class="reply-modal-title">{{ t.xVerifyModal_title }}</span>
        <button class="reply-modal-close" @click="closeXVerifyModal">×</button>
      </div>
      <p class="reply-modal-hint">{{ t.xVerifyModal_hint }}</p>
      <ol class="reply-steps">
        <li>{{ t.xVerifyModal_step1 }}</li>
        <li>{{ t.xVerifyModal_step2 }}</li>
      </ol>
      <div class="reply-modal-footer">
        <button class="reply-btn-cancel" @click="closeXVerifyModal" :disabled="xVerifyModal.submitting">
          {{ t.xVerifyModal_cancel }}
        </button>
        <a class="reply-btn-open" :href="xVerifyModal.url" target="_blank" rel="noopener">
          <span class="material-symbols-outlined text-[16px]">open_in_new</span>
          {{ t.xVerifyModal_open }}
        </a>
        <button class="reply-btn-submit"
                :disabled="xVerifyModal.submitting"
                @click="submitXVerification">
          <span v-if="xVerifyModal.submitting" class="material-symbols-outlined animate-spin text-[16px]">progress_activity</span>
          {{ xVerifyModal.submitting ? t.xVerifyModal_verifying : t.xVerifyModal_verify }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.material-symbols-outlined { font-variation-settings: 'FILL' 0, 'wght' 400, 'GRAD' 0, 'opsz' 24; }

/* Refer Friends 视觉增强：右上角绿色光晕 + 内层 noise */
.refer-card {
  background:
    radial-gradient(120% 100% at 100% 0%, rgba(155, 255, 0, 0.35) 0%, rgba(155, 255, 0, 0.08) 35%, transparent 65%),
    #ffffff;
}
.refer-bloom {
  position: absolute;
  right: -120px;
  top: -120px;
  width: 320px;
  height: 320px;
  border-radius: 50%;
  background: radial-gradient(circle at center, rgba(155, 255, 0, 0.55), transparent 70%);
  filter: blur(40px);
  pointer-events: none;
  z-index: 0;
  animation: refer-bloom-pulse 4s ease-in-out infinite;
}
@keyframes refer-bloom-pulse {
  0%, 100% { transform: scale(1); opacity: 1; }
  50%      { transform: scale(1.15); opacity: 0.7; }
}

/* === 微交互：鼓励用户操作 === */

/* 任务卡 hover：轻微抬升 + 阴影加深；用 scale 不抢 transform 通道 */
.task-card,
.refer-card {
  position: relative;
  transition: scale 0.28s cubic-bezier(0, 0, 0.2, 1),
              box-shadow 0.28s cubic-bezier(0, 0, 0.2, 1);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.04);
}
.task-card:hover {
  scale: 1.005;
  box-shadow: 0 14px 36px -4px rgba(0, 0, 0, 0.12);
}

/* ====== 未登录遮罩 ====== */
/* 未登录时的遮罩 + Connect Wallet 按钮；仅 !wallet.isLoggedIn 渲染 */
.lock-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: inherit;
  background: rgba(255, 255, 255, 0.55);
  backdrop-filter: blur(8px) saturate(140%);
  -webkit-backdrop-filter: blur(8px) saturate(140%);
  /* 默认 semi-transparent 让用户直观感知未登录；hover 时全不透明 */
  opacity: 0.85;
  pointer-events: auto;
  transition: opacity 0.25s ease;
  z-index: 30;
  cursor: pointer;
}
.task-card:hover .lock-overlay,
.refer-card:hover .lock-overlay {
  opacity: 1;
  pointer-events: auto;
}
/* navbar Connect Wallet 同款：浪涌动效 + 绿色光晕 + label 反色 */
.cw-btn {
  position: relative;
  width: 160px;
  height: 40px;
  background: #9bff00;
  border: none;
  border-radius: 22px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: box-shadow 0.4s ease, transform 0.4s ease;
}
.cw-btn:hover {
  box-shadow: 0 8px 28px rgba(155, 255, 0, 0.55);
  transform: translateY(-1px);
}
.cw-deco {
  position: absolute;
  display: block;
  pointer-events: none;
  transition: transform 0.6s cubic-bezier(0.65, 0, 0.35, 1);
}
.cw-deco img { width: 100%; height: 100%; display: block; }
/* 按 160×40 缩放 navbar 130×32 的几何（×1.23）*/
.cw-wave-wrap {
  width: 278px;
  height: 146px;
  left: -57px;
  top: -136px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.cw-wave {
  width: 268px;
  height: 123px;
  transform: rotate(5deg);
}
.cw-crest {
  width: 125px;
  height: 69px;
}
.cw-crest-r { left: 79px;  top: -120px; }
.cw-crest-l { left: -28px; top: -78px; }
.cw-btn:hover .cw-wave-wrap { transform: translateY(88px); }
.cw-btn:hover .cw-crest-r  { transform: translateY(71px); }
.cw-btn:hover .cw-crest-l  { transform: translateY(52px); }
.cw-label {
  position: relative;
  z-index: 2;
  font-size: 14px;
  color: #0b1020;
  font-family: 'Inter', sans-serif;
  font-weight: 600;
  letter-spacing: 0.3px;
  transition: color 0.45s ease;
}
.cw-btn:hover .cw-label { color: #9bff00; }

/* ====== 暗色配色覆盖 ====== */
/* 中性黑灰 + 微蓝调（R=16 G=18 B=24）*/
.jc-canvas-host.home-dark .lock-overlay {
  background: rgba(16, 18, 24, 0.62);
  backdrop-filter: blur(10px) saturate(105%);
  -webkit-backdrop-filter: blur(10px) saturate(105%);
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.06);
}
/* 深底下绿色光晕加亮 */
.jc-canvas-host.home-dark .cw-btn {
  box-shadow: 0 6px 22px rgba(155, 255, 0, 0.28);
}
.jc-canvas-host.home-dark .cw-btn:hover {
  box-shadow: 0 10px 34px rgba(155, 255, 0, 0.65);
}

/* X / TG 子任务行 hover：背景变白、边框变绿、右移 4px；已完成的不再 hover */
.subtask-row {
  transition: background 0.2s ease, transform 0.2s ease, border-color 0.2s ease;
}
.subtask-row:not(.done-row):not(.is-locked):hover {
  background: #ffffff;
  border-color: #9bff00;
  transform: translateX(4px);
}
.subtask-row.is-locked {
  background: #f1f2f2 !important;
  border-color: #dfe2e3 !important;
}
.tg-task-btn.is-prerequisite-locked {
  background: #dfe3e5 !important;
  border: 1px solid #cbd1d5;
  color: #747d84 !important;
  cursor: not-allowed;
  opacity: 1 !important;
  box-shadow: none !important;
  white-space: nowrap;
}
/* 刚完成的高亮 flash：绿色一闪然后回淡 */
.subtask-row.just-done {
  animation: subtask-flash 1.4s cubic-bezier(0.22, 1, 0.36, 1);
}
@keyframes subtask-flash {
  0%   { background: #9bff00; border-color: #1aa64a; box-shadow: 0 0 0 0 rgba(155, 255, 0, 0.5); }
  40%  { background: rgba(155, 255, 0, 0.4); border-color: #9bff00; box-shadow: 0 0 0 14px rgba(155, 255, 0, 0); }
  100% { background: #fafaf8; border-color: #eee; box-shadow: 0 0 0 0 rgba(155, 255, 0, 0); }
}

/* X / TG 任务组进度条 */
.progress-track {
  width: 100%;
  height: 4px;
  background: #f0efec;
  border-radius: 999px;
  overflow: hidden;
}
.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, #9bff00 0%, #1aa64a 100%);
  border-radius: 999px;
  transition: width 0.6s cubic-bezier(0.22, 1, 0.36, 1);
  box-shadow: 0 0 12px rgba(155, 255, 0, 0.5);
}

/* 未签到 Daily 按钮：呼吸动画唤起注意（绿色光圈不断扩散） */
.cta-pulse {
  animation: cta-breath 2.4s ease-in-out infinite;
}
@keyframes cta-breath {
  0%, 100% { box-shadow: 0 0 0 0 rgba(155, 255, 0, 0.45); }
  50%      { box-shadow: 0 0 0 14px rgba(155, 255, 0, 0); }
}

/* 日历方格 hover：放大 + tooltip 浮出显示日期 */
.cal-cell {
  position: relative;
  transition: transform 0.18s cubic-bezier(0, 0, 0.2, 1), box-shadow 0.18s ease;
  cursor: default;
}
.cal-cell:not(.cal-blank):hover {
  transform: scale(1.6);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  z-index: 3;
}
.cal-cell:not(.cal-blank):hover::after {
  content: attr(data-day);
  position: absolute;
  bottom: calc(100% + 4px);
  left: 50%;
  transform: translateX(-50%);
  background: #212121;
  color: #fff;
  padding: 3px 7px;
  border-radius: 4px;
  font-size: 9px;
  font-weight: 700;
  white-space: nowrap;
  pointer-events: none;
  z-index: 4;
}

/* 一键分享按钮 hover：轻微抬升 + 阴影 */
.refer-card button {
  transition: transform 0.2s ease, box-shadow 0.2s ease, filter 0.2s ease;
}
.refer-card button:hover {
  transform: translateY(-1px);
  box-shadow: 0 6px 18px rgba(0, 0, 0, 0.15);
}

.refer-card .refer-copy-btn {
  min-width: 78px;
  justify-content: center;
  background: linear-gradient(180deg, #b9ff36 0%, #8cff00 100%);
  border: 1px solid rgba(255, 255, 255, 0.34);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.45),
    0 0 0 1px rgba(10, 16, 6, 0.16),
    0 8px 18px rgba(155, 255, 0, 0.18);
  color: #101410;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.01em;
}

.refer-card .refer-copy-btn:hover:not(:disabled) {
  background: linear-gradient(180deg, #d2ff72 0%, #9bff00 100%);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.55),
    0 0 0 1px rgba(10, 16, 6, 0.18),
    0 10px 24px rgba(155, 255, 0, 0.34);
}

.refer-card .refer-copy-btn .material-symbols-outlined {
  font-size: 16px !important;
  font-variation-settings: 'FILL' 0, 'wght' 700, 'GRAD' 0, 'opsz' 20;
}

/* Copy controls stay dark on the lime button in both themes. */
.jc-canvas-host.home-dark .refer-card .refer-copy-btn,
.jc-canvas-host.home-dark .refer-card .refer-copy-btn .material-symbols-outlined,
.m-taskhall.home-dark .m-th-refer-copy-btn,
.m-taskhall.home-dark .m-th-refer-copy-btn .material-symbols-outlined {
  color: #101410 !important;
}

/* 子任务 verify 按钮 hover：scale 强调"可操作" */
.subtask-row :is(button, a):not(:disabled):not([aria-disabled="true"]) {
  transition: transform 0.18s ease, box-shadow 0.18s ease;
}
.subtask-row :is(button, a):not(:disabled):not([aria-disabled="true"]):hover {
  transform: translateY(-1px) scale(1.04);
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.18);
}

/* ============================================================
   暗色主题（与 jc-shell.css 的全局规则配合，只补 TaskHall 专属元素）
   ============================================================ */

/* TG / X 子任务行：浅米色 bg-[#fafaf8] → 深灰，灰边框 → 半透白边 */
.jc-canvas-host.home-dark .subtask-row {
  background: #1f1f1f !important;
  border-color: rgba(255, 255, 255, 0.08) !important;
}
/* 行内"序号圈"白底 → 深灰 */
.jc-canvas-host.home-dark .subtask-row .bg-white {
  background: #2a2a2a !important;
  border-color: rgba(255, 255, 255, 0.12) !important;
}
/* 已完成的子任务行：bg-[#9bff00]/30 在深底上保留绿调（不动） */

/* Daily Sign-in 日历：未签到方格在深底上要换更深的灰，否则太亮抢戏 */
.jc-canvas-host.home-dark .cal-cell.bg-\[\#f0efec\],
.jc-canvas-host.home-dark .cal-cell[class*="bg-[#f0efec]"] {
  background-color: #2a2a2a !important;  /* future 日 */
}
.jc-canvas-host.home-dark .cal-cell.bg-\[\#e2e2de\],
.jc-canvas-host.home-dark .cal-cell[class*="bg-[#e2e2de]"] {
  background-color: #3a3a3a !important;  /* past 未签到 */
}
/* "Today" 方块在暗底上 bg-[#212121] 几乎不可见 —— 翻成白色块 */
.jc-canvas-host.home-dark .cal-cell.bg-\[\#212121\],
.jc-canvas-host.home-dark .cal-cell[class*="bg-[#212121]"] {
  background-color: #ffffff !important;
}
/* 图例小方块：Today（[#212121]）也翻白；Signed（[#9bff00]）保持绿 */
.jc-canvas-host.home-dark .text-\[\#808080\] .bg-\[\#212121\],
.jc-canvas-host.home-dark [class*="text-[#808080]"] [class*="bg-[#212121]"] {
  background-color: #ffffff !important;
}

/* 连签徽章（米黄色 streak）在暗底上太刺眼 → 深灰底 + 暖橙描边保留 */
.jc-canvas-host.home-dark [class*="bg-[#fff7e6]"] {
  background-color: #2a221a !important;
  border-color: rgba(250, 140, 22, 0.4) !important;
}

/* TG / X 任务卡里的进度条轨道（白色 #f0efec）在暗底上像条亮白线 */
.jc-canvas-host.home-dark .progress-track {
  background: #2a2a2a !important;
}

/* === 子任务行 hover：背景提亮 + 绿色描边 === */
.jc-canvas-host.home-dark .subtask-row {
  transition: background-color 0.18s ease, border-color 0.18s ease,
              transform 0.18s ease, box-shadow 0.18s ease;
}
.jc-canvas-host.home-dark .subtask-row:not(.is-locked):hover {
  background-color: #262626 !important;
  border-color: rgba(155, 255, 0, 0.32) !important;
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.25);
}
.jc-canvas-host.home-dark .subtask-row.is-locked {
  background: #1a1d20 !important;
  border-color: rgba(255, 255, 255, 0.08) !important;
}
.jc-canvas-host.home-dark .tg-task-btn.is-prerequisite-locked {
  background: #30343a !important;
  border-color: rgba(255, 255, 255, 0.1);
  color: #8d959d !important;
}

/* === X 任务 Verify 按钮：暗底纯黑 → 白底黑字（X 暗模式品牌写法），hover → 品牌绿 === */
.jc-canvas-host.home-dark .subtask-row :is(button, a)[class*="bg-[#212121]"] {
  background-color: #ffffff !important;
  color: #212121 !important;
}
.jc-canvas-host.home-dark .subtask-row :is(button, a)[class*="bg-[#212121]"]:hover {
  background-color: #9bff00 !important;
  color: #212121 !important;
}

/* ============ Reply Modal ============ */
.reply-modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background: rgba(0, 0, 0, 0.55);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
}
.reply-modal {
  background: #ffffff;
  border-radius: 16px;
  padding: 24px;
  width: 92%;
  max-width: 520px;
  box-shadow: 0 24px 60px -12px rgba(0, 0, 0, 0.35);
  font-family: 'Inter', sans-serif;
}
.reply-modal-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}
.reply-modal-title {
  font-size: 18px;
  font-weight: 700;
  color: #212121;
  flex: 1;
}
.reply-modal-close {
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 22px;
  color: #808080;
  padding: 0 6px;
  line-height: 1;
}
.reply-modal-close:hover { color: #212121; }
.reply-modal-hint {
  font-size: 13px;
  color: #5f6659;
  margin: 0 0 14px;
  line-height: 1.5;
}
.reply-modal-textarea {
  width: 100%;
  border: 1px solid #ececea;
  border-radius: 10px;
  padding: 12px 14px;
  font-size: 14px;
  font-family: inherit;
  color: #212121;
  resize: vertical;
  min-height: 100px;
  outline: none;
  transition: border-color 0.18s ease;
}
.reply-modal-textarea:focus { border-color: #9bff00; }
.reply-modal-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 14px;
}
.reply-modal-count { font-size: 12px; color: #808080; font-weight: 600; }
.reply-modal-count.over { color: #f5222d; }
.reply-modal-actions { display: flex; gap: 10px; }
.reply-btn-cancel {
  background: white;
  border: 1px solid #ececea;
  padding: 8px 16px;
  border-radius: 999px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 700;
  color: #212121;
}
.reply-btn-cancel:hover:not(:disabled) { border-color: #808080; }
.reply-btn-submit {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: #9bff00;
  border: none;
  padding: 8px 18px;
  border-radius: 999px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 700;
  color: #212121;
  box-shadow: 0 4px 14px rgba(155, 255, 0, 0.35);
}
.reply-btn-submit:hover:not(:disabled) { transform: translateY(-1px); }
.reply-btn-submit:disabled { opacity: 0.55; cursor: not-allowed; }
.reply-btn-open {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: #212121;
  border: none;
  padding: 8px 16px;
  border-radius: 999px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 700;
  color: white;
  text-decoration: none;
}
.reply-btn-open:hover { background: #000; }
.reply-steps {
  list-style: none;
  padding: 0;
  margin: 0 0 14px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.reply-steps li {
  font-size: 13px;
  line-height: 1.55;
  color: #4c5148;
  padding: 10px 14px;
  background: #fafaf8;
  border: 1px solid #ececea;
  border-radius: 8px;
}

/* X verification modal follows the page theme even though it is rendered
   outside the .home-dark page wrapper so it can remain viewport-fixed. */
.reply-modal-overlay.is-dark {
  background: rgba(5, 7, 10, 0.72);
}
.reply-modal-overlay.is-dark .reply-modal {
  background: #1f1f1f;
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0 24px 64px -12px rgba(0, 0, 0, 0.72);
}
.reply-modal-overlay.is-dark .reply-modal-title { color: #fafaf7; }
.reply-modal-overlay.is-dark .reply-modal-close { color: #9a9a92; }
.reply-modal-overlay.is-dark .reply-modal-close:hover { color: #fafaf7; }
.reply-modal-overlay.is-dark .reply-modal-hint { color: #b8b8b0; }
.reply-modal-overlay.is-dark .reply-steps li {
  color: #d0d0ca;
  background: #282828;
  border-color: rgba(255, 255, 255, 0.09);
}
.reply-modal-overlay.is-dark .reply-btn-cancel {
  color: #f0f0ec;
  background: #282828;
  border-color: rgba(255, 255, 255, 0.14);
}
.reply-modal-overlay.is-dark .reply-btn-cancel:hover:not(:disabled) {
  border-color: #9bff00;
}
.reply-modal-overlay.is-dark .reply-btn-open {
  color: #212121;
  background: #ffffff;
}
.reply-modal-overlay.is-dark .reply-btn-open:hover { background: #9bff00; }
.reply-steps strong { color: #212121; margin-right: 4px; }

/* ============ Mobile-only styles ============ */
@media (max-width: 768px) {
  .reply-modal-overlay {
    padding: 16px;
    align-items: flex-end;
  }
  .reply-modal {
    width: 100%;
    max-width: none;
    max-height: calc(100dvh - 32px);
    overflow-y: auto;
    padding: 20px 18px;
    border-radius: 16px;
  }
  .reply-modal-footer {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
  }
  .reply-btn-cancel {
    grid-column: 1 / -1;
    order: 3;
  }
  .reply-btn-open,
  .reply-btn-submit {
    justify-content: center;
    min-height: 42px;
    text-align: center;
  }
  .m-th-stats {
    display: flex; flex-direction: column; gap: 10px; margin-top: 8px;
  }
  .m-th-stat {
    background: white; border: 1px solid #ececea;
    border-radius: 12px; padding: 14px 16px;
    display: flex; align-items: center; gap: 12px;
  }
  .home-dark .m-th-stat {
    background: #1f1f1f; border-color: rgba(255,255,255,0.06);
  }
  .m-th-stat .material-symbols-outlined { font-size: 26px; }
  .m-th-stat-label {
    font-size: 10px; font-weight: 700; letter-spacing: 1.2px;
    text-transform: uppercase; color: #808080; margin: 0;
  }
  .m-th-stat-value {
    font-size: 22px; font-weight: 700; color: #212121; margin: 2px 0 0;
    font-variant-numeric: tabular-nums;
  }
  .home-dark .m-th-stat-value { color: #fafaf7; }
  .m-th-stat-value small { font-size: 10px; font-weight: 700; color: #808080; }

  .m-th-section { padding: 12px 20px; }

  .m-th-card {
    background: white; border: 1px solid #ececea; border-radius: 14px;
    padding: 20px; display: flex; flex-direction: column; gap: 14px;
    position: relative;
  }
  .home-dark .m-th-card {
    background: #1f1f1f; border-color: rgba(255,255,255,0.06);
  }
  .m-th-card-head {
    display: flex; justify-content: space-between; align-items: flex-start; gap: 12px;
  }
  .m-th-card-head h3 {
    font-size: 18px; font-weight: 700; color: #212121; margin: 0;
  }
  .home-dark .m-th-card-head h3 { color: #fafaf7; }
  .m-th-card-sub {
    font-size: 10px; font-weight: 700; letter-spacing: 1.2px;
    text-transform: uppercase; color: #808080; margin: 2px 0 0;
  }
  .m-th-card-desc {
    font-size: 13px; line-height: 1.5; color: #4c5148; margin: 0;
  }
  .home-dark .m-th-card-desc { color: #b8b8b0; }
  .m-th-card-title-row {
    display: flex; align-items: center; gap: 10px;
  }
  .m-th-brand {
    width: 32px; height: 32px; border-radius: 8px;
    display: inline-flex; align-items: center; justify-content: center;
    flex-shrink: 0;
  }
  .m-th-brand-tg { background: #229ED9; }
  .m-th-brand-x { background: #212121; color: white; font-weight: 700; font-size: 14px; }

  .m-th-done-pill {
    display: inline-flex; align-items: center; gap: 4px;
    background: rgba(155, 255, 0, 0.3);
    padding: 4px 10px; border-radius: 999px;
    font-size: 10px; font-weight: 700; letter-spacing: 1.2px;
    text-transform: uppercase; color: #1aa64a;
  }
  .m-th-done-pill .w-dot {
    width: 6px; height: 6px; border-radius: 50%; background: #1aa64a;
    animation: pulse 1.6s ease-in-out infinite;
  }
  .m-th-streak-badge {
    display: inline-flex; align-items: center; gap: 6px;
    background: #fff7e6; border: 1px solid #ffd591;
    padding: 6px 12px; border-radius: 999px;
    font-size: 12px; font-weight: 700; color: #fa8c16;
    align-self: flex-start;
  }
  .m-th-streak-badge .material-symbols-outlined { font-size: 14px; }

  .m-th-cal {
    display: flex; flex-direction: column; gap: 6px; align-items: center;
    padding: 12px 0;
  }
  .m-th-cal-label {
    font-size: 10px; font-weight: 700; letter-spacing: 1.2px;
    text-transform: uppercase; color: #808080; margin: 0;
  }
  .m-th-cal-grid {
    display: grid; grid-template-columns: repeat(7, 20px); gap: 4px;
  }
  .m-th-cal-cell {
    width: 20px; height: 20px; border-radius: 3px; background: #e2e2de;
  }
  .m-th-cal-cell.is-blank { opacity: 0; }
  .m-th-cal-cell.is-checked { background: #9bff00; }
  .m-th-cal-cell.is-today {
    background: #212121;
    box-shadow: 0 0 0 2px #9bff00, 0 0 0 3px white;
  }
  .m-th-cal-cell.is-future { background: #f0efec; }
  .m-th-cal-legend {
    display: flex; gap: 12px;
    font-size: 10px; font-weight: 500; color: #808080;
  }
  .m-th-cal-legend > span { display: inline-flex; align-items: center; gap: 4px; }
  .m-th-cal-legend .dot { width: 8px; height: 8px; border-radius: 2px; }
  .m-th-cal-legend .dot-signed { background: #9bff00; }
  .m-th-cal-legend .dot-today { background: #212121; }

  .m-th-card-foot {
    display: flex; justify-content: space-between; align-items: center;
    padding-top: 8px; border-top: 1px solid #ececea; margin-top: 4px;
  }
  .home-dark .m-th-card-foot { border-color: rgba(255,255,255,0.06); }
  .m-th-reward {
    display: flex; flex-direction: column;
  }
  .m-th-reward-label {
    font-size: 9px; font-weight: 700; letter-spacing: 1.2px;
    text-transform: uppercase; color: #808080;
  }
  .m-th-reward-val {
    font-size: 18px; font-weight: 700; color: #1aa64a;
  }
  .m-th-reward-val small { font-size: 10px; color: #808080; }
  .m-th-reward-dark { color: #212121; }
  .home-dark .m-th-reward-dark { color: #fafaf7; }
  .m-th-check-btn {
    display: inline-flex; align-items: center; gap: 6px;
    background: #9bff00; color: #212121; border: none;
    height: 40px; padding: 0 20px; border-radius: 999px;
    font-size: 13px; font-weight: 700; cursor: pointer;
    box-shadow: 0 4px 12px rgba(155, 255, 0, 0.35);
  }
  .m-th-check-btn:disabled { opacity: 0.5; cursor: not-allowed; box-shadow: none; }
  .m-th-done-btn {
    display: inline-flex; align-items: center; gap: 6px;
    background: rgba(155, 255, 0, 0.3); border: none;
    height: 40px; padding: 0 20px; border-radius: 999px;
    font-size: 13px; font-weight: 700; color: #1aa64a; cursor: not-allowed;
  }
  .m-th-done-btn .material-symbols-outlined { font-size: 16px; }

  .m-th-progress {
    height: 4px; background: #ececea; border-radius: 999px;
    overflow: hidden;
  }
  .home-dark .m-th-progress { background: #262626; }
  .m-th-progress > div {
    height: 100%; background: #9bff00; transition: width 0.7s ease;
  }

  .m-th-subtasks {
    list-style: none; padding: 0; margin: 0;
    display: flex; flex-direction: column; gap: 10px;
  }
  .m-th-sub {
    display: grid; grid-template-columns: 26px 1fr auto;
    align-items: center; gap: 10px;
    padding: 12px; background: #fafaf8; border: 1px solid #ececea;
    border-radius: 10px;
  }
  .home-dark .m-th-sub {
    background: #262626; border-color: rgba(255,255,255,0.06);
  }
  .m-th-sub.is-done { opacity: 0.7; }
  .m-th-sub.is-locked {
    background: #f1f2f2; border-color: #dfe2e3;
  }
  .home-dark .m-th-sub.is-locked {
    background: #1a1d20; border-color: rgba(255,255,255,0.08);
  }
  .m-th-sub-num {
    width: 26px; height: 26px; border-radius: 50%;
    display: inline-flex; align-items: center; justify-content: center;
    background: white; border: 1px solid #ddd;
    font-size: 12px; font-weight: 700; color: #808080;
  }
  .m-th-sub.is-done .m-th-sub-num {
    background: #9bff00; color: #212121; border-color: transparent;
  }
  .m-th-sub-num .material-symbols-outlined { font-size: 14px; }
  .m-th-sub-body { min-width: 0; display: flex; flex-direction: column; gap: 2px; }
  .m-th-sub-title { font-size: 13px; font-weight: 700; color: #212121; margin: 0; }
  .home-dark .m-th-sub-title { color: #fafaf7; }
  .m-th-sub-desc {
    font-size: 11px; color: #808080; margin: 0;
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .m-th-sub-pts { font-size: 10px; font-weight: 700; color: #1aa64a; }
  .m-th-sub-btn {
    display: inline-flex; align-items: center; gap: 4px;
    border: none; padding: 8px 12px; border-radius: 6px;
    font-size: 11px; font-weight: 700; color: white;
    cursor: pointer;
  }
  .m-th-sub-btn:disabled, .m-th-sub-btn.is-disabled { opacity: 0.6; cursor: not-allowed; }
  a.m-th-sub-btn { text-decoration: none; }
  .m-th-sub-btn-tg { background: #229ED9; }
  .m-th-sub-btn-tg.is-prerequisite-locked {
    background: #dfe3e5; border: 1px solid #cbd1d5;
    color: #747d84; opacity: 1; box-shadow: none; white-space: nowrap;
  }
  .home-dark .m-th-sub-btn-tg.is-prerequisite-locked {
    background: #30343a; border-color: rgba(255,255,255,0.1); color: #8d959d;
  }
  .m-th-sub-btn-x { background: #212121; }
  .m-th-sub-done {
    display: inline-flex; align-items: center; gap: 4px;
    background: rgba(155, 255, 0, 0.3);
    padding: 6px 10px; border-radius: 6px;
    font-size: 11px; font-weight: 700; color: #1aa64a;
  }
  .m-th-sub-done .material-symbols-outlined { font-size: 14px; }

  .m-th-x-binding {
    display: flex; justify-content: flex-end;
  }
  .m-th-x-bound {
    display: inline-flex; align-items: center; gap: 6px;
    background: rgba(155, 255, 0, 0.3);
    padding: 6px 12px; border-radius: 999px;
    font-size: 11px; font-weight: 700; letter-spacing: 1px;
    text-transform: uppercase; color: #1aa64a;
  }
  .m-th-refer-card {
    background: linear-gradient(160deg, #f7fbe9 0%, #ffffff 60%);
  }
  .home-dark .m-th-refer-card {
    background: linear-gradient(160deg, #1f2b1e 0%, #1f1f1f 60%);
  }
  .m-th-kicker {
    display: inline-flex; align-items: center; gap: 6px;
    background: #212121; color: #9bff00;
    padding: 4px 10px; border-radius: 999px;
    font-size: 10px; font-weight: 700; letter-spacing: 1.2px;
    text-transform: uppercase; align-self: flex-start;
  }
  .m-th-refer-head {
    font-size: 24px; font-weight: 700; color: #212121;
    margin: 0; line-height: 1.2;
  }
  .home-dark .m-th-refer-head { color: #fafaf7; }
  .m-th-refer-accent { color: #1aa64a; }
  .m-th-refer-desc {
    font-size: 13px; line-height: 1.5; color: #4c5148; margin: 0;
  }
  .home-dark .m-th-refer-desc { color: #b8b8b0; }
  .m-th-refer-stats {
    display: grid; grid-template-columns: 1fr 1fr; gap: 10px;
  }
  .m-th-refer-stats > div {
    background: rgba(255,255,255,0.85);
    padding: 12px 16px; border-radius: 10px;
  }
  .m-th-refer-earned { background: #9bff00 !important; }
  .m-th-refer-box {
    background: #212121; color: white; border-radius: 12px; padding: 16px;
    display: flex; flex-direction: column; gap: 12px;
  }
  .m-th-refer-field {
    display: flex; flex-direction: column; gap: 4px;
  }
  .m-th-refer-field label {
    font-size: 10px; font-weight: 700; letter-spacing: 1.5px;
    text-transform: uppercase; color: #9bff00;
  }
  .m-th-refer-input {
    display: flex; gap: 8px;
    background: rgba(255,255,255,0.1); border-radius: 8px;
    padding: 4px;
  }
  .m-th-refer-input input {
    flex: 1; background: transparent; border: none; outline: none;
    color: white; font-size: 13px; font-weight: 700;
    padding: 8px 12px;
    min-width: 0;
  }
  .m-th-refer-input button {
    display: inline-flex; align-items: center; gap: 4px;
    background: linear-gradient(180deg, #b9ff36 0%, #8cff00 100%);
    color: #101410; border: 1px solid rgba(255,255,255,0.34);
    padding: 8px 12px; border-radius: 6px;
    font-size: 11px; font-weight: 900; cursor: pointer;
    flex-shrink: 0;
    box-shadow: inset 0 1px 0 rgba(255,255,255,0.42), 0 6px 14px rgba(155,255,0,0.18);
  }
  .m-th-refer-input button:disabled { opacity: 0.5; cursor: not-allowed; }
  .m-th-refer-input button .material-symbols-outlined {
    font-size: 16px;
    font-variation-settings: 'FILL' 0, 'wght' 700, 'GRAD' 0, 'opsz' 20;
  }
  .m-th-refer-share {
    display: flex; gap: 8px;
  }
  .m-th-share-x, .m-th-share-tg {
    flex: 1; display: inline-flex; align-items: center; justify-content: center; gap: 6px;
    border: none; padding: 10px; border-radius: 8px;
    font-size: 12px; font-weight: 700; cursor: pointer;
  }
  .m-th-share-x { background: white; color: #212121; }
  .m-th-share-tg { background: #229ED9; color: white; }

  .m-th-lock {
    position: absolute; inset: 0;
    background: rgba(255, 255, 255, 0.75); backdrop-filter: blur(4px);
    border-radius: 14px;
    display: flex; align-items: center; justify-content: center;
    font-size: 14px; font-weight: 700; color: #212121;
    cursor: pointer; z-index: 5;
  }
  .home-dark .m-th-lock {
    background: rgba(31,31,31,0.85);
    color: #fafaf7;
  }

  .m-th-formula {
    background: #212121; color: white; border-radius: 12px;
    padding: 16px 20px;
    display: flex; flex-direction: column; gap: 4px;
    margin: 12px 0;
  }
  .m-th-formula-label {
    font-size: 10px; font-weight: 700; letter-spacing: 1.5px;
    text-transform: uppercase; color: #9bff00;
  }
  .m-th-formula-val {
    font-family: monospace; font-size: 14px; font-weight: 700;
  }
}

/* 绿色胶囊卡：无论亮/暗模式都强制黑字（避免 dark mode 全局 text-[#212121]→白 反色 */
.earned-pill [class*="text-[#212121]"] { color: #212121 !important; }
.canvas-host.home-dark .earned-pill [class*="text-[#212121]"] { color: #212121 !important; }

/* 页面级 toast：X OAuth 回跳错误、重复绑定等 */
.jc-page-toast {
  position: fixed;
  top: 88px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 9999;
  max-width: 90vw;
  min-width: 280px;
  padding: 14px 18px 14px 20px;
  border-radius: 12px;
  font-size: 14px;
  line-height: 1.5;
  color: #fff;
  display: flex;
  align-items: center;
  gap: 14px;
  box-shadow: 0 10px 30px rgba(0,0,0,0.18);
  cursor: pointer;
  user-select: none;
}
.jc-page-toast--error   { background: #c0392b; }
.jc-page-toast--success { background: #1aa64a; }
.jc-page-toast--info    { background: #212121; }
.jc-page-toast__msg { flex: 1; word-break: break-word; }
.jc-page-toast__close {
  background: transparent; border: 0; color: #fff;
  font-size: 22px; line-height: 1; cursor: pointer;
  padding: 0 4px; opacity: 0.75;
}
.jc-page-toast__close:hover { opacity: 1; }
.jc-page-toast-fade-enter-active,
.jc-page-toast-fade-leave-active { transition: opacity .25s ease, transform .25s ease; }
.jc-page-toast-fade-enter-from,
.jc-page-toast-fade-leave-to { opacity: 0; transform: translate(-50%, -12px); }
@media (max-width: 640px) {
  .jc-page-toast { top: 72px; min-width: auto; width: calc(100vw - 32px); }
}
</style>
