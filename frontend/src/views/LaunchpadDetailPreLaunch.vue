<script setup>
import {
  computed,
  inject,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from 'vue'
import { useRoute } from 'vue-router'
import DOMPurify from 'dompurify'
import JcFloatingNavbar from '../components/JcFloatingNavbar.vue'
import HomeFooter from '../components/HomeFooter.vue'
import PixelGrid from '../components/PixelGrid.vue'
import LaunchpadActiveFundingPanel from '../components/LaunchpadActiveFundingPanel.vue'
import FairLaunchReadOnlyPanel from '../components/FairLaunchReadOnlyPanel.vue'
import {
  getProjectDetail,
  getProjectUpdates,
  getProjectUserStatus,
  joinProjectWhitelist,
  subscribeProject,
  unsubscribeProject,
  unwatchProject,
  watchProject,
} from '../services/api'
import { useWalletStore } from '../stores/wallet'
import {
  deriveProjectStage,
  getProjectCountdownTarget,
  isCappedFundingPanelStage,
  isStalePreLaunchSaleClaim,
  normalizeProjectStatus,
} from '../utils/projectStatus'
import { resolveProjectFundingRoute } from '../utils/fairLaunch.js'
import { toFundingPanelProject } from '../utils/fundingPanelProject.js'
import { assertV3Project, pinnedV3WalletRuntime } from '../services/idoV3Wallet.js'
import { normalizeProjectContext } from '../services/projectContext.js'
import { projectFaqItems, v3ProjectFaq } from '../utils/projectFaq.js'
import { formatRaisedAmount } from '../utils/raisedAmount.js'

const props = defineProps({
  project: { type: Object, default: null },
  language: { type: String, default: 'en' },
})

const route = useRoute()
const wallet = useWalletStore()
const isDark = inject('isHomeDark', ref(false))
// Backend serializes Sonyflake project IDs as strings. Converting a route ID
// to Number silently rounds values above Number.MAX_SAFE_INTEGER and turns a
// valid detail request into a generic server error (record not found).
const projectID = computed(() => String(route.params.id || ''))

const A = '/figma/v2'
const img = (n) => `${A}/${n}`

const isZh = computed(() => props.language && props.language !== 'en')
const t = computed(() => isZh.value ? {
  afterTge: 'TGE 後',
  amountRaised: '已募集',
  aboutProject: '關於項目',
  cliff: '鎖倉',
  distribution: '分配',
  documents: '文件',
  faq: '常見問題',
  file: '文件',
  getNotified: '項目上線時通知我。',
  fundingOpen: 'Fair Launch 募資狀態以鏈上與後端權威資料共同判定。',
  fundingEndsIn: '距離本輪結束',
  participateNow: '查看 Fair Launch',
  fairReadOnlyTag: 'FAIR LAUNCH',
  fairReadOnlyCta: '查看募資與結算',
  fairReadOnlyTerminal: '操作只在受信合約身份、權威鏈上狀態及後端可用性全部一致時開放。',
  joinWhitelist: '加入白名單',
  launchIn: '距離上線',
  liveOnchain: '目前生效',
  member: '位成員',
  members: '位成員',
  milestones: '里程碑',
  monthSuffix: ' 個月',
  noUpdates: '尚無更新。',
  notifyMe: '通知我',
  onWhitelist: '已加入白名單',
  onchain: '鏈上資訊',
  pending: '待公布',
  post: '篇更新',
  posts: '篇更新',
  readRisks: '參與前請仔細閱讀。',
  research: '研究與數據',
  risks: '風險與揭露',
  stageComingSoon: '即將開放',
  stageFundingLive: '募資進行中',
  stageWhitelistOpen: '白名單開放中',
  stageEnded: '募資已結束',
  stageSuccessful: '募資成功',
  stageFailed: '募資失敗',
  stageCanceled: '項目已取消',
  terminalSuccessful: '本輪募資已成功結束，後續代幣領取請前往 Dashboard 查看。',
  terminalFailed: '本輪募資未達軟頂，已參與用戶可在右側面板辦理退款。',
  terminalCanceled: '本項目已取消，已參與用戶可在右側面板辦理退款。',
  terminalEnded: '本輪募資已結束，最終鏈上結果顯示於右側面板。',
  status: { active: '進行中', done: '已完成', planned: '計劃中' },
  story: '故事',
  subscribed: '已訂閱',
  subscribeNotice: '訂閱後，團隊發布新消息時將通知你。',
  subscribers: '訂閱者',
  team: '團隊',
  tokenomics: '代幣經濟',
  updates: '更新',
  vest: '釋放',
  whitelist: '白名單',
  working: '處理中...',
  watchAction: '關注',
  watching: '已關注',
  watchlist: '關注列表',
} : {
  afterTge: 'AFTER TGE',
  amountRaised: 'RAISED',
  aboutProject: 'ABOUT THE PROJECT',
  cliff: 'Cliff',
  distribution: 'Distribution',
  documents: 'DOCUMENTS',
  faq: 'FAQ',
  file: 'FILE',
  getNotified: 'Get notified when this project goes live.',
  fundingOpen: 'Fair Launch availability is jointly gated by authoritative chain and Backend state.',
  fundingEndsIn: 'Funding ends in',
  participateNow: 'View Fair Launch',
  fairReadOnlyTag: 'FAIR LAUNCH',
  fairReadOnlyCta: 'View funding and settlement',
  fairReadOnlyTerminal: 'Actions open only when trusted contract identity, authoritative chain state and Backend availability all agree.',
  joinWhitelist: 'Join whitelist',
  launchIn: 'Launch in',
  liveOnchain: 'LIVE NOW',
  member: 'MEMBER',
  members: 'MEMBERS',
  milestones: 'MILESTONES',
  monthSuffix: 'mo',
  noUpdates: 'No updates yet.',
  notifyMe: 'Notify me',
  onWhitelist: "You're on whitelist",
  onchain: 'ON-CHAIN',
  pending: 'pending',
  post: 'POST',
  posts: 'POSTS',
  readRisks: 'Please read carefully before participating.',
  research: 'Research & Data',
  risks: 'RISKS & DISCLOSURES',
  stageComingSoon: 'COMING SOON',
  stageFundingLive: 'FUNDING LIVE',
  stageWhitelistOpen: 'WHITELIST OPEN',
  stageEnded: 'FUNDING ENDED',
  stageSuccessful: 'FUNDING SUCCESSFUL',
  stageFailed: 'FUNDING FAILED',
  stageCanceled: 'PROJECT CANCELED',
  terminalSuccessful: 'This funding round completed successfully. View token claims in Dashboard.',
  terminalFailed: 'This round did not reach its soft cap. Contributors can request a refund in the panel.',
  terminalCanceled: 'This project was canceled. Contributors can request a refund in the panel.',
  terminalEnded: 'This funding round has ended. The final on-chain result is shown in the panel.',
  status: { active: 'ACTIVE', done: 'DONE', planned: 'PLANNED' },
  story: 'Story',
  subscribed: "You're subscribed",
  subscribeNotice: 'Subscribe to be notified when the team posts news.',
  subscribers: 'Subscribers',
  team: 'TEAM',
  tokenomics: 'TOKENOMICS',
  updates: 'UPDATES',
  vest: 'Vest',
  whitelist: 'Whitelist',
  working: 'Working...',
  watchAction: 'Watch',
  watching: 'Watching',
  watchlist: 'Watchlist',
})
function countLabel(count, singular, plural) {
  if (isZh.value) return `${count} ${plural}`
  return `${count} ${count === 1 ? singular : plural}`
}
function statusLabel(status) {
  return t.value.status?.[status] || status
}

// ===== 1440 画布缩放（与 TaskHall / LaunchpadList / Home 一致）=====
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
  const trigger = window.innerHeight * 0.8
  els.forEach((el) => {
    if (el.getBoundingClientRect().top < trigger) el.classList.add('is-revealed')
  })
}

// ===== 数据 =====
const detail = ref(null)
const fundingPanelProject = computed(() => toFundingPanelProject(detail.value || props.project, projectID.value))
const loadError = ref(null)
const loading = ref(true)
const userStatus = ref({ subscribed: false, watched: false, whitelisted: false })
const updates = ref([])
const busy = ref({ subscribe: false, watch: false, whitelist: false })
const now = ref(Date.now())
let tickTimer = null
let detailRequest = 0

async function loadDetail() {
  const requestID = ++detailRequest
  loading.value = true
  loadError.value = null
  // Do not display the prior route's identity or wallet panel while a new
  // project detail is in flight (including a same-component route change).
  detail.value = null
  try {
    const response = await getProjectDetail(projectID.value)
    if (requestID !== detailRequest) return
    detail.value = response
  } catch (e) {
    if (requestID === detailRequest) loadError.value = e?.message || 'load failed'
  } finally {
    if (requestID === detailRequest) loading.value = false
  }
}
async function loadUpdates() {
  try {
    const r = await getProjectUpdates(projectID.value, 1, 10)
    updates.value = r?.items || []
  } catch (e) { console.warn('[updates]', e?.message) }
}
async function loadUserStatus() {
  try {
    userStatus.value = (await getProjectUserStatus(projectID.value)) || userStatus.value
  } catch { /* not logged in */ }
}

// ===== stage =====
const fundingRoute = computed(() => resolveProjectFundingRoute(detail.value || props.project))
const isFairLaunch = computed(() => fundingRoute.value.ok && fundingRoute.value.kind === 'fair_launch')
const isV3Project = computed(() => fundingRoute.value.ok && fundingRoute.value.kind === 'ido_v3')
const isV3SyncMaintenance = computed(() => isV3Project.value && normalizeProjectStatus(detail.value?.status) === 'v3_reorg_halted')
const isV3WalletAvailable = computed(() => {
  if (isV3SyncMaintenance.value) return false
  if (!isV3Project.value || !detail.value?.onchainLocked || !/^[1-9][0-9]*$/.test(projectID.value) || !detail.value?.paymentTokenAddress || !detail.value?.tokenAddress) return false
  try {
    const runtime = pinnedV3WalletRuntime()
    assertV3Project(normalizeProjectContext(fundingPanelProject.value, {}), runtime)
    return true
  } catch { return false }
})
const hasUnsupportedFundingIdentity = computed(() => !fundingRoute.value.ok)
const derivedStage = computed(() => deriveProjectStage(detail.value, now.value))
const stage = computed(() => {
  const s = derivedStage.value
  if (s === 'syncing') return {
    key: 'syncing', tag: isZh.value ? '同步維護中' : 'SYNC MAINTENANCE', ctaLabel: '', ctaKey: 'none',
  }
  if (isFairLaunch.value) {
    return {
      key: s === 'live' ? 'active' : s.startsWith('ended_') || ['successful_pending_release', 'successful', 'failed', 'refund', 'canceled', 'ended'].includes(s) ? 'ended' : s,
      outcome: s,
      tag: t.value.fairReadOnlyTag,
      ctaLabel: t.value.fairReadOnlyCta,
      ctaKey: 'fair_read_only',
    }
  }
  if (s === 'live') return {
    key: 'active', tag: t.value.stageFundingLive, ctaLabel: t.value.participateNow, ctaKey: 'participate',
  }
  if (s === 'upcoming') return {
    key: 'upcoming', tag: t.value.stageComingSoon, ctaLabel: t.value.notifyMe, ctaKey: 'subscribe',
  }
  if (s === 'pending') return {
    key: 'pending', tag: t.value.stageComingSoon, ctaLabel: t.value.notifyMe, ctaKey: 'subscribe',
  }
  if (s === 'warmup') return {
    key: 'warmup', tag: t.value.stageWhitelistOpen, ctaLabel: isV3Project.value ? '' : t.value.joinWhitelist, ctaKey: isV3Project.value ? 'none' : 'whitelist',
  }
  if (s === 'ended_success' || s === 'ended_failed') return {
    key: 'ended', tag: t.value.stageEnded, ctaLabel: '', ctaKey: 'none',
  }
  if (s === 'successful_pending_release') return {
    key: 'ended', outcome: 'pending_release', tag: t.value.stageEnded, ctaLabel: '', ctaKey: 'none',
  }
  if (s === 'successful') return {
    key: 'ended', outcome: 'successful', tag: t.value.stageSuccessful, ctaLabel: '', ctaKey: 'none',
  }
  if (s === 'failed' || s === 'refund') return {
    key: 'ended', outcome: 'failed', tag: t.value.stageFailed, ctaLabel: '', ctaKey: 'none',
  }
  if (s === 'canceled') return {
    key: 'ended', outcome: 'canceled', tag: t.value.stageCanceled, ctaLabel: '', ctaKey: 'none',
  }
  if (s === 'ended') return {
    key: 'ended', outcome: 'ended', tag: t.value.stageEnded, ctaLabel: '', ctaKey: 'none',
  }
  return {
    key: 'pre_launch', tag: t.value.stageComingSoon, ctaLabel: t.value.notifyMe, ctaKey: 'subscribe',
  }
})

// ===== structured parse =====
const team = computed(() => safeParse(detail.value?.teamJson, []))
const isActiveStage = computed(() => normalizeProjectStatus(detail.value?.status) === 'active')
const tokenomics = computed(() => {
  const value = safeParse(detail.value?.tokenomicsJson, { allocations: [] })
  if (!isActiveStage.value) return value
  return {
    ...value,
    allocations: (value.allocations || []).map((item) => {
      if (!/public sale|公開發售/i.test(`${item.label || ''} ${item.label_zh || ''}`)) return item
      return { ...item, label: 'Public sale (current round)', label_zh: '公開發售（當前輪次）' }
    }),
  }
})
const roadmap = computed(() => {
  const value = safeParse(detail.value?.roadmapJson, [])
  if (!isActiveStage.value) return value
  return value.map((item) => {
    if (!/pre-launch subscription|預啟動訂閱/i.test(`${item.title || ''} ${item.title_zh || ''}`)) return item
    if (isFairLaunch.value) {
      return {
        ...item,
        status: 'active',
        title: 'Fair Launch funding round',
        title_zh: 'Fair Launch 募資輪次',
        desc: 'Wallet actions require trusted runtime identity and authoritative Backend/chain availability.',
        desc_zh: '錢包操作要求受信執行身份及後端／鏈上權威可用性。',
      }
    }
    return {
      ...item,
      status: 'active',
      title: 'On-chain funding round live',
      title_zh: '鏈上募資輪次進行中',
      desc: 'The public-sale contract is live. Funding progress, wallet limits and transactions are available on this page.',
      desc_zh: '公開發售合約已啟用；本頁可查看募集進度、錢包限額並完成鏈上認購。',
    }
  })
})
const documents = computed(() => safeParse(detail.value?.documentsJson, []))
const faq = computed(() => {
  const value = projectFaqItems(detail.value?.faqJson)
  if (isV3Project.value) {
    return v3ProjectFaq(value)
  }
  if (isFairLaunch.value) {
    return [{
      q: 'When are Fair Launch wallet actions available?',
      q_zh: 'Fair Launch 錢包操作何時可用？',
      a: 'Only when exact mode/version, Backend-trusted contract identity, token runtime checks, authoritative lifecycle and action availability all agree. Unknown identity never falls back to legacy.',
      a_zh: '僅當模式／版本、後端受信合約身份、Token 執行檢查、權威生命週期及動作可用性全部一致時才開放；未知身份絕不回退舊合約。',
    }, ...value]
  }
  if (!isActiveStage.value) return value
  return value.map((item) => {
    if (!/phase-1|第一階段/i.test(`${item.q || ''} ${item.q_zh || ''}`)) return item
    return {
      q: 'How does the live on-chain funding round work?',
      q_zh: '目前的鏈上募資輪次如何運作？',
      a: `This project is now Active. Eligible wallets can participate with ${detail.value?.paymentToken || 'the payment token'} through the public-sale contract. The live panel shows the price, caps, wallet limit, balance, approval and transaction status.`,
      a_zh: `本項目現已進入 Active 階段。符合條件的錢包可使用 ${detail.value?.paymentToken || '支付代幣'} 透過公開發售合約參與；即時面板會顯示價格、軟硬頂、錢包限額、餘額、授權及交易狀態。`,
    }
  })
})
const onchain = computed(() => {
  const value = safeParse(detail.value?.onchainJson, [])
  if (!isActiveStage.value) return value
  const chain = Number(detail.value?.chainId) === 31337 ? 'Hardhat' : 'BSC'
  return value.map((item) => {
    if (/token|代幣/i.test(`${item.label || ''} ${item.label_zh || ''}`)) {
      return {
        ...item,
        label: `${detail.value?.tokenSymbol || 'Project'} token`,
        label_zh: `${detail.value?.tokenSymbol || '項目'} 代幣`,
        address: detail.value?.tokenAddress || item.address,
        chain,
      }
    }
    if (/public-sale contract|公開發售合約/i.test(`${item.label || ''} ${item.label_zh || ''}`)) {
      return { ...item, address: detail.value?.contractAddress || item.address, chain }
    }
    return item
  })
})
function safeParse(raw, fallback) {
  if (!raw) return fallback
  try { return JSON.parse(raw) } catch { return fallback }
}
function sanitize(html) { return html ? DOMPurify.sanitize(html) : '' }

// ===== 双语 pick =====
// language 非 'en' (zh / zh-TW / zh-CN) 时优先取 _zh / _Zh 版本，空则 fallback 到主字段
function pick(en, zh) {
  if (isZh.value && zh) return zh
  return en || ''
}
const pickTagline = computed(() => pick(detail.value?.tagline, detail.value?.taglineZh))
const pickName = computed(() => pick(detail.value?.name, detail.value?.nameCn))
const pickStoryHtml = computed(() => pick(detail.value?.storyHtml, detail.value?.storyHtmlZh))
const pickResearchHtml = computed(() => pick(detail.value?.researchHtml, detail.value?.researchHtmlZh))
const pickRisksHtml = computed(() => {
  const html = pick(detail.value?.risksHtml, detail.value?.risksHtmlZh)
  if (!isActiveStage.value || !html || typeof DOMParser === 'undefined') return html
  const doc = new DOMParser().parseFromString(html, 'text/html')
  doc.querySelectorAll('li').forEach((item) => {
    if (isStalePreLaunchSaleClaim(item.textContent)) item.remove()
  })
  return doc.body.innerHTML
})
// JSON 内嵌 _zh 字段的通用 helper（供 team / roadmap / documents / faq / onchain 等使用）
function pickField(obj, key) {
  if (!obj) return ''
  return pick(obj[key], obj[key + '_zh'])
}

// ===== countdown =====
const launchCountdown = computed(() => {
  if (isV3SyncMaintenance.value) return null
  if (stage.value.key === 'ended') return null
  const target = getProjectCountdownTarget(detail.value, derivedStage.value)
  if (!target) return null
  const ms = target * 1000 - now.value
  if (ms <= 0) return { done: true }
  const s = Math.floor(ms / 1000)
  return {
    done: false,
    d: Math.floor(s / 86400),
    h: Math.floor((s % 86400) / 3600),
    m: Math.floor((s % 3600) / 60),
    s: s % 60,
  }
})

const stageMessage = computed(() => {
  if (isV3SyncMaintenance.value) return isZh.value ? '資料同步維護中，錢包操作暫停' : 'Synchronization maintenance; wallet actions paused'
  if (isV3Project.value) return isV3WalletAvailable.value
    ? (isZh.value ? '報名不保證能認購。開放認購時，請連接您打算使用的錢包查看是否可參與。' : 'Signing up does not guarantee access. Connect the wallet you plan to use when participation opens.')
    : (isZh.value ? '暫時無法確認項目資料，錢包操作尚不可用。請稍後重新整理。' : 'We cannot confirm this project’s details right now. Wallet actions are paused; refresh later.')
  if (isFairLaunch.value) return t.value.fairReadOnlyTerminal
  if (stage.value.outcome === 'successful') return t.value.terminalSuccessful
  if (stage.value.outcome === 'failed') return t.value.terminalFailed
  if (stage.value.outcome === 'canceled') return t.value.terminalCanceled
  if (stage.value.key === 'ended') return t.value.terminalEnded
  return stage.value.key === 'active' ? t.value.fundingOpen : t.value.getNotified
})

// ===== actions =====
function ensureLogin(fn) {
  return async () => {
    if (!wallet.isLoggedIn) {
      try { await wallet.connectWallet() } catch { return }
      if (!wallet.isLoggedIn) return
    }
    await fn()
  }
}
const onSubscribe = ensureLogin(async () => {
  busy.value.subscribe = true
  try {
    if (userStatus.value.subscribed) await unsubscribeProject(projectID.value)
    else await subscribeProject(projectID.value)
    userStatus.value.subscribed = !userStatus.value.subscribed
  } catch (e) { console.warn(e?.message) }
  finally { busy.value.subscribe = false }
})
const onWatch = ensureLogin(async () => {
  busy.value.watch = true
  try {
    if (userStatus.value.watched) await unwatchProject(projectID.value)
    else await watchProject(projectID.value)
    userStatus.value.watched = !userStatus.value.watched
  } catch (e) { console.warn(e?.message) }
  finally { busy.value.watch = false }
})
const onWhitelist = ensureLogin(async () => {
  if (isV3Project.value) return
  if (userStatus.value.whitelisted) return
  busy.value.whitelist = true
  try {
    await joinProjectWhitelist(projectID.value, wallet.address || '')
    userStatus.value.whitelisted = true
  } catch (e) { console.warn(e?.message) }
  finally { busy.value.whitelist = false }
})
const primaryCTA = computed(() => {
  if (isV3Project.value) {
    return isV3WalletAvailable.value
      ? { label: isZh.value ? '查看認購詳情' : 'View participation details', done: false,
          handler: () => document.getElementById('active-funding-panel')?.scrollIntoView({ behavior: 'smooth', block: 'start' }), busy: false }
      : { label: '', done: true, handler: () => {}, busy: false }
  }
  if (stage.value.ctaKey === 'fair_read_only') {
    return {
      label: stage.value.ctaLabel,
      done: false,
      handler: () => document.getElementById('active-funding-panel')?.scrollIntoView({ behavior: 'smooth', block: 'start' }),
      busy: false,
    }
  }
  if (stage.value.ctaKey === 'participate') {
    return {
      label: stage.value.ctaLabel,
      done: false,
      handler: () => document.getElementById('active-funding-panel')?.scrollIntoView({ behavior: 'smooth', block: 'start' }),
      busy: false,
    }
  }
  if (stage.value.ctaKey === 'whitelist') {
    return {
      label: userStatus.value.whitelisted ? t.value.onWhitelist : stage.value.ctaLabel,
      done: userStatus.value.whitelisted,
      handler: onWhitelist,
      busy: busy.value.whitelist,
    }
  }
  if (stage.value.ctaKey === 'none') {
    return { label: '', done: true, handler: () => {}, busy: false }
  }
  return {
    label: userStatus.value.subscribed ? t.value.subscribed : stage.value.ctaLabel,
    done: userStatus.value.subscribed,
    handler: onSubscribe,
    busy: busy.value.subscribe,
  }
})

// ===== lifecycle =====
onMounted(async () => {
  window.scrollTo(0, 0)
  await loadDetail()
  await Promise.all([loadUpdates(), loadUserStatus()])
  nextTick(() => { measure(); scrollRevealHandler() })
  setTimeout(() => { measure(); scrollRevealHandler() }, 500)
  window.addEventListener('resize', applyScale)
  window.addEventListener('resize', scrollRevealHandler)
  window.addEventListener('scroll', scrollRevealHandler, { passive: true })
  tickTimer = setInterval(() => { now.value = Date.now() }, 1000)
})
watch(() => [props.language, projectID.value], loadDetail)
onBeforeUnmount(() => {
  window.removeEventListener('resize', applyScale)
  window.removeEventListener('resize', scrollRevealHandler)
  window.removeEventListener('scroll', scrollRevealHandler)
  if (tickTimer) clearInterval(tickTimer)
})
watch(() => wallet.isLoggedIn, loadUserStatus)

const openExplorer = (address, chain) => {
  if (!address) return
  if (chain?.toLowerCase().includes('hardhat')) return
  const base = chain?.toLowerCase().includes('eth')
    ? 'https://etherscan.io/address/'
    : 'https://bscscan.com/address/'
  window.open(base + address, '_blank', 'noopener')
}
const openedFaq = ref(new Set())
const toggleFaq = (i) => {
  if (openedFaq.value.has(i)) openedFaq.value.delete(i)
  else openedFaq.value.add(i)
  openedFaq.value = new Set(openedFaq.value)
  nextTick(measure)
}
</script>

<template>
  <JcFloatingNavbar :scale="scale" active="launchpad" />

  <div class="jc-canvas-host" :class="{ 'home-dark': isDark }" :style="{ height: hostHeight }">
    <div class="jc-scaled-wrapper" :style="{ width: `${1440 * scale}px`, height: hostHeight }">
      <main ref="canvasRef" class="jc-canvas-1440" :style="{ transform: `scale(${scale})` }">

        <div class="h-[64px] w-full shrink-0"></div>

        <template v-if="loading || !detail">
          <div class="flex items-center justify-center min-h-[560px] text-[16px] text-[#808080]">
            {{ loadError || 'Loading...' }}
          </div>
        </template>

        <template v-else>
          <!-- ============ HERO ============ -->
          <section data-reveal class="launchpad-detail-hero relative w-full px-[80px] pt-[60px] pb-[32px] flex flex-col gap-[36px]">
            <!-- PixelGrid 装饰（Hero 右上）-->
            <div class="absolute pointer-events-none z-10" style="top: 60px; right: 80px;">
              <PixelGrid :cols="12" :rows="12" cell-size="14px" gap="0" color="#9bff00"
                         :flash-duration="450" :idle-min="900" :idle-max="2800"
                         :start-stagger-max="2800" :jitter="0" :jitter-rotate="0" :jitter-scale="0" />
            </div>

            <!-- Eyebrow -->
            <div class="jc-hero-anim jc-hero-anim-1 flex items-center gap-[8px]">
              <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
              <span class="text-[20px] font-bold text-[#212121]">{{ stage.tag }}</span>
              <span v-if="detail.category" class="ml-[6px] text-[12px] text-[#808080] font-bold uppercase tracking-[1.4px]">
                · {{ detail.category }}
              </span>
            </div>

            <!-- 88px 大标题 -->
            <div class="font-bold text-[#212121] tracking-[-2.88px]" style="font-size: 88px; line-height: 0.95; width: 1180px;">
              <p class="jc-hero-headline-line jc-hero-headline-line-1 font-semibold mb-0">{{ pickName }}</p>
              <p v-if="pickTagline" class="jc-hero-headline-line jc-hero-headline-line-2 font-playfair italic font-normal" style="font-size: 40px; line-height: 1.15; letter-spacing: -0.6px; color: #5f6659;">
                {{ pickTagline }}
              </p>
            </div>

            <!-- sub + Stats 横排（rise-card）-->
            <div class="jc-hero-anim jc-hero-anim-3 flex items-end justify-between gap-[32px]">
              <div class="active-status-actions flex flex-wrap items-center gap-[12px]" style="width: 420px;">
                <div v-if="launchCountdown && !launchCountdown.done" class="flex items-center gap-[10px]">
                  <span class="text-[10px] font-bold uppercase tracking-[1.5px] text-[#808080]">{{ stage.key === 'active' ? t.fundingEndsIn : t.launchIn }}</span>
                  <div v-for="[lab, val] in [['d', launchCountdown.d], ['h', launchCountdown.h], ['m', launchCountdown.m], ['s', launchCountdown.s]]"
                       :key="lab"
                       class="flex flex-col items-center bg-white rounded-[8px] px-[10px] py-[4px] border border-[#eee]">
                    <span class="text-[18px] font-bold text-[#212121] leading-none tabular-nums">{{ val }}</span>
                    <span class="text-[9px] uppercase text-[#808080] tracking-[1px] mt-[2px]">{{ lab }}</span>
                  </div>
                </div>
                <p v-else class="text-[16px] font-bold leading-[1.5] text-[#212121]">{{ stageMessage }}</p>
              </div>

              <!-- Stats 紧凑横排 -->
              <div class="rise-card rise-card-1 bg-white flex items-stretch rounded-[12px] overflow-hidden shrink-0">
                <div class="flex flex-col items-start gap-[8px] px-[24px] py-[16px] relative overflow-hidden border-r border-[#eee] min-w-[140px]">
                  <div class="flex items-center gap-[6px]">
                    <span class="material-symbols-outlined text-[#1aa64a] text-[14px]" style="font-variation-settings: 'FILL' 1;">notifications_active</span>
                    <span class="text-[9px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.subscribers }}</span>
                  </div>
                  <span class="text-[30px] font-bold text-[#212121] leading-none tabular-nums">{{ detail.subscriberCount || 0 }}</span>
                  <div class="absolute -bottom-8 -right-8 w-[80px] h-[80px] bg-[#9bff00]/30 rounded-full blur-2xl pointer-events-none"></div>
                </div>
                <div class="flex flex-col items-start gap-[8px] px-[24px] py-[16px] border-r border-[#eee] min-w-[120px]">
                  <div class="flex items-center gap-[6px]">
                    <span class="material-symbols-outlined text-[#3884ff] text-[14px]" style="font-variation-settings: 'FILL' 1;">favorite</span>
                    <span class="text-[9px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.watchlist }}</span>
                  </div>
                  <span class="text-[30px] font-bold text-[#212121] leading-none tabular-nums">{{ detail.watchCount || 0 }}</span>
                </div>
                <div v-if="stage.key === 'warmup' && !isV3Project" class="flex flex-col items-start gap-[8px] px-[24px] py-[16px] min-w-[120px]">
                  <div class="flex items-center gap-[6px]">
                    <span class="material-symbols-outlined text-[#fa8c16] text-[14px]" style="font-variation-settings: 'FILL' 1;">key</span>
                    <span class="text-[9px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.whitelist }}</span>
                  </div>
                  <span class="text-[30px] font-bold text-[#212121] leading-none tabular-nums">{{ detail.whitelistCount || 0 }}</span>
                </div>
                <div v-else-if="stage.key === 'active' || stage.key === 'ended'" class="flex flex-col items-start gap-[8px] px-[24px] py-[16px] min-w-[160px]">
                  <div class="flex items-center gap-[6px]">
                    <svg class="h-[15px] w-[15px] shrink-0 text-[#1aa64a]" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                      <ellipse cx="8" cy="4" rx="5.25" ry="2.25" stroke="currentColor" stroke-width="1.4" />
                      <path d="M2.75 4v3c0 1.24 2.35 2.25 5.25 2.25S13.25 8.24 13.25 7V4M2.75 7v3c0 1.24 2.35 2.25 5.25 2.25s5.25-1.01 5.25-2.25V7" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" />
                    </svg>
                    <span class="text-[9px] font-bold tracking-[1.2px] uppercase text-[#808080]">{{ t.amountRaised }}</span>
                  </div>
                  <span class="text-[24px] font-bold text-[#212121] leading-none tabular-nums">{{ detail.fundingDataStatus === 'syncing' ? '—' : formatRaisedAmount(detail.totalRaised, detail.paymentToken) }} {{ detail.paymentToken }}</span>
                </div>
              </div>
            </div>

            <!-- CTA -->
            <div class="jc-hero-anim jc-hero-anim-4 flex items-center gap-[12px]">
              <button v-if="primaryCTA.label" class="white-pill-btn cta-pulse disabled:opacity-60 disabled:cursor-not-allowed"
                      :disabled="primaryCTA.busy || primaryCTA.done"
                      @click="primaryCTA.handler">
                <span class="text-[14px] font-bold text-[#212121]">{{ primaryCTA.busy ? t.working : primaryCTA.label }}</span>
                <span v-if="!primaryCTA.done && !primaryCTA.busy" class="arrow-icon arrow-diag">
                  <span class="arrow-img arrow-a" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                  <span class="arrow-img arrow-b" :style="{ '--mask': `url(${img('arrow-up-right.svg')})` }"></span>
                </span>
              </button>
              <button class="watchlist-pill inline-flex items-center gap-[8px] px-[16px] py-[12px] rounded-full bg-white border border-[#eee] hover:border-[#212121] transition-colors disabled:opacity-60"
                      :disabled="busy.watch"
                      @click="onWatch">
                <span class="material-symbols-outlined text-[16px]"
                      :class="userStatus.watched ? 'text-[#f5222d]' : 'text-[#808080]'"
                      :style="userStatus.watched ? { fontVariationSettings: `'FILL' 1` } : {}">favorite</span>
                <span class="text-[13px] font-bold text-[#212121]">{{ userStatus.watched ? t.watching : t.watchAction }}</span>
              </button>
            </div>

            <!-- 淡淡分隔线 -->
            <div class="pointer-events-none" style="height: 1px; background: linear-gradient(to right, transparent 0%, rgba(0,0,0,0.08) 12%, rgba(0,0,0,0.08) 88%, transparent 100%);"></div>
          </section>

          <!-- ============ BODY · 左右两栏 ============ -->
          <section data-reveal class="launchpad-detail-body relative w-full px-[80px] pt-[20px] pb-[80px]">
            <div class="launchpad-body-layout flex gap-[40px] items-start">

              <!-- ========== 左栏 · 内容 & 资料 ========== -->
              <div class="launchpad-body-main flex flex-col gap-[40px] flex-1 min-w-0">

                <!-- COVER 大图（放在 ABOUT 之前，让内容板块不再全是文字）-->
                <div v-if="detail.coverUrl" class="relative w-full overflow-hidden rounded-[16px] cover-hero">
                  <img :src="detail.coverUrl" :alt="pickName" class="w-full h-[360px] object-cover" />
                  <!-- 底部渐变遮罩 + 项目名叠印（防止图内容遮挡 title）-->
                  <div class="absolute inset-x-0 bottom-0 flex items-end justify-between p-[24px]"
                       style="background: linear-gradient(to top, rgba(0,0,0,0.72) 0%, rgba(0,0,0,0.35) 55%, rgba(0,0,0,0) 100%);">
                    <div class="flex flex-col gap-[6px] text-white">
                      <span v-if="detail.category" class="text-[10px] font-bold uppercase tracking-[1.6px] text-[#9bff00]">
                        {{ detail.category }}
                      </span>
                      <span class="text-[20px] font-bold leading-tight">{{ pickName }}</span>
                    </div>
                    <span class="inline-flex items-center gap-[6px] px-[12px] py-[6px] rounded-full bg-white/95 text-[10px] font-bold uppercase tracking-[1.2px] text-[#212121]">
                      <span class="w-[6px] h-[6px] rounded-full bg-[#9bff00]"></span>{{ stage.tag }}
                    </span>
                  </div>
                </div>

                <!-- STORY + RESEARCH -->
                <div v-if="pickStoryHtml || pickResearchHtml" class="flex flex-col gap-[24px]">
                  <div class="flex items-center gap-[8px]">
                    <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
                    <span class="text-[20px] font-bold text-[#212121]">{{ t.aboutProject }}</span>
                  </div>
                  <article v-if="pickStoryHtml" class="task-card bg-white p-[28px] rounded-[12px]">
                    <header class="flex items-center gap-[10px] mb-[14px]">
                      <span class="inline-flex w-[36px] h-[36px] items-center justify-center rounded-[8px] bg-[#9bff00]/30">
                        <span class="material-symbols-outlined text-[#1aa64a] text-[20px]" style="font-variation-settings: 'FILL' 1;">auto_stories</span>
                      </span>
                      <h3 class="text-[22px] font-bold text-[#212121] leading-none">{{ t.story }}</h3>
                    </header>
                    <div class="prose-jc" v-html="sanitize(pickStoryHtml)"></div>
                  </article>
                  <article v-if="pickResearchHtml" class="task-card bg-white p-[28px] rounded-[12px]">
                    <header class="flex items-center gap-[10px] mb-[14px]">
                      <span class="inline-flex w-[36px] h-[36px] items-center justify-center rounded-[8px] bg-[#3884ff]/15">
                        <span class="material-symbols-outlined text-[#3884ff] text-[20px]" style="font-variation-settings: 'FILL' 1;">science</span>
                      </span>
                      <h3 class="text-[22px] font-bold text-[#212121] leading-none">{{ t.research }}</h3>
                    </header>
                    <div class="prose-jc" v-html="sanitize(pickResearchHtml)"></div>
                  </article>
                </div>

                <!-- UPDATES -->
                <div class="flex flex-col gap-[24px]">
                  <div class="flex items-center gap-[8px]">
                    <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
                    <span class="text-[20px] font-bold text-[#212121]">{{ t.updates }}</span>
                    <span class="ml-[6px] text-[11px] font-bold text-[#808080]">{{ countLabel(updates.length, t.post, t.posts) }}</span>
                  </div>
                  <article v-if="!updates.length" class="task-card bg-white p-[28px] rounded-[12px] flex items-center gap-[14px]">
                    <span class="inline-flex w-[40px] h-[40px] items-center justify-center rounded-full bg-[#fafaf8] border border-[#eee]">
                      <span class="material-symbols-outlined text-[#808080] text-[20px]">campaign</span>
                    </span>
                    <div class="flex flex-col">
                      <span class="text-[15px] font-bold text-[#212121]">{{ t.noUpdates }}</span>
                      <span class="text-[12px] text-[#808080]">{{ t.subscribeNotice }}</span>
                    </div>
                  </article>
                  <article v-for="u in updates" :key="u.id" class="task-card bg-white p-[28px] rounded-[12px]">
                    <header class="flex items-baseline justify-between mb-[12px]">
                      <div class="flex items-center gap-[10px]">
                        <span class="inline-flex w-[10px] h-[10px] rounded-full bg-[#9bff00]"></span>
                        <h3 class="text-[18px] font-bold text-[#212121]">{{ u.title }}</h3>
                      </div>
                      <span class="text-[11px] font-bold uppercase tracking-[1.2px] text-[#808080]">
                        {{ (u.published_at || '').replace('T',' ').split('.')[0] }}
                      </span>
                    </header>
                    <div class="prose-jc text-[14px]" v-html="sanitize(u.content_html)"></div>
                  </article>
                </div>

                <!-- DOCUMENTS -->
                <div v-if="documents.length" class="flex flex-col gap-[24px]">
                  <div class="flex items-center gap-[8px]">
                    <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
                    <span class="text-[20px] font-bold text-[#212121]">{{ t.documents }}</span>
                  </div>
                  <article class="task-card bg-white p-[28px] rounded-[12px] flex flex-col gap-[10px]">
                    <a v-for="(d, i) in documents" :key="i" :href="d.url || '#'" :target="d.url ? '_blank' : '_self'" rel="noopener"
                       class="subtask-row flex items-center gap-[14px] py-[14px] px-[16px] rounded-[8px] bg-[#fafaf8] border border-[#eee] hover:bg-white hover:border-[#9bff00] transition-colors">
                      <span class="inline-flex w-[40px] h-[40px] items-center justify-center rounded-[8px] bg-white border border-[#eee] shrink-0">
                        <span class="material-symbols-outlined text-[#3884ff] text-[20px]" style="font-variation-settings: 'FILL' 1;">description</span>
                      </span>
                      <div class="flex flex-col flex-1 min-w-0">
                        <span class="text-[14px] font-bold text-[#212121] truncate">{{ pickField(d, 'title') }}</span>
                        <span class="text-[11px] font-bold uppercase tracking-[1.2px] text-[#808080] mt-[2px]">{{ pickField(d, 'kind') || t.file }}</span>
                      </div>
                      <span v-if="d.url" class="material-symbols-outlined text-[#808080] text-[18px] shrink-0">open_in_new</span>
                    </a>
                  </article>
                </div>

                <!-- FAQ -->
                <div v-if="faq.length" class="flex flex-col gap-[24px]">
                  <div class="flex items-center gap-[8px]">
                    <span class="w-[22px] h-[22px] bg-[#9bff00] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
                    <span class="text-[20px] font-bold text-[#212121]">{{ t.faq }}</span>
                  </div>
                  <article class="task-card bg-white p-[28px] rounded-[12px] flex flex-col gap-[10px]">
                    <div v-for="(q, i) in faq" :key="i"
                         :class="['subtask-row py-[14px] px-[16px] rounded-[8px] bg-[#fafaf8] border border-[#eee] cursor-pointer', { 'is-open': openedFaq.has(i) }]"
                         @click="toggleFaq(i)">
                      <div class="flex items-center gap-[14px]">
                        <span class="inline-flex w-[26px] h-[26px] items-center justify-center rounded-full bg-white border border-[#ddd] text-[12px] font-bold text-[#808080] shrink-0">Q</span>
                        <span class="text-[14px] font-bold text-[#212121] flex-1">{{ pickField(q, 'q') }}</span>
                        <span class="material-symbols-outlined text-[#808080] text-[20px] transition-transform"
                              :style="{ transform: openedFaq.has(i) ? 'rotate(45deg)' : 'rotate(0)' }">add</span>
                      </div>
                      <div v-if="openedFaq.has(i)" class="mt-[12px] pl-[40px] text-[13px] text-[#5f6659] leading-[1.6]">{{ pickField(q, 'a') }}</div>
                    </div>
                  </article>
                </div>

                <!-- RISKS -->
                <div v-if="pickRisksHtml" class="flex flex-col gap-[24px]">
                  <div class="flex items-center gap-[8px]">
                    <span class="w-[22px] h-[22px] bg-[#fa8c16] rounded-tr-[14px] rounded-bl-[4px] rounded-tl-[4px] rounded-br-[4px] inline-block" style="transform: rotate(15deg);"></span>
                    <span class="text-[20px] font-bold text-[#212121]">{{ t.risks }}</span>
                  </div>
                  <article class="task-card p-[28px] rounded-[12px] bg-[#fff7e6] border border-[#ffd591]">
                    <header class="flex items-center gap-[10px] mb-[14px]">
                      <span class="inline-flex w-[36px] h-[36px] items-center justify-center rounded-[8px] bg-white border border-[#ffd591]">
                        <span class="material-symbols-outlined text-[#fa8c16] text-[20px]" style="font-variation-settings: 'FILL' 1;">warning</span>
                      </span>
                      <h3 class="text-[18px] font-bold text-[#212121] leading-none">{{ t.readRisks }}</h3>
                    </header>
                    <div class="prose-jc" v-html="sanitize(pickRisksHtml)"></div>
                  </article>
                </div>

              </div>

              <!-- ========== 右栏 · 团队 / 进度 / 代币 / 链上 ========== -->
              <aside class="right-rail flex flex-col gap-[32px] shrink-0" style="width: 400px;">

                <article v-if="isV3SyncMaintenance" class="task-card rounded-[12px] border border-[#ffd591] bg-[#fff7e6] p-[20px] text-[12px] font-bold leading-[1.6] text-[#874d00]" role="status">
                  {{ isZh ? '項目資料同步維護中，錢包操作暫停。鏈上資產與歷史交易不受頁面展示影響，請稍後重新整理。' : 'Project synchronization is under maintenance. Wallet actions are paused. This display issue does not change on-chain assets or transaction history. Please refresh later.' }}
                </article>
                <FairLaunchReadOnlyPanel
                  v-else-if="(stage.key === 'active' || stage.key === 'ended') && isFairLaunch"
                  :project="{ ...(detail || props.project), id: projectID }"
                  :language="props.language"
                  :is-dark="isDark"
                />
                <LaunchpadActiveFundingPanel
                  v-else-if="fundingRoute.ok && (fundingRoute.kind === 'ido_v3' || (isCappedFundingPanelStage(stage.key) && fundingRoute.kind === 'capped_presale' && fundingRoute.contractVersion === 'v2_stablecoin'))"
                  :project="fundingPanelProject"
                  :language="props.language"
                  :is-dark="isDark"
                />
                <article v-else-if="(stage.key === 'active' || stage.key === 'ended') && hasUnsupportedFundingIdentity" class="task-card rounded-[12px] border border-[#ffd591] bg-[#fff7e6] p-[20px] text-[12px] font-bold leading-[1.6] text-[#874d00]">
                  {{ isZh ? '目前無法確認這個項目的資料，暫時不能操作錢包。請稍後重新整理；若問題持續，請聯絡客服。' : 'We cannot confirm this project’s details right now, so wallet actions are paused. Refresh later or contact support.' }}
                </article>

                <!-- TEAM -->
                <div v-if="team.length" class="flex flex-col gap-[16px]">
                  <div class="flex items-center gap-[8px]">
                    <span class="w-[18px] h-[18px] bg-[#9bff00] rounded-tr-[12px] rounded-bl-[3px] rounded-tl-[3px] rounded-br-[3px] inline-block" style="transform: rotate(15deg);"></span>
                    <span class="text-[16px] font-bold text-[#212121]">{{ t.team }}</span>
                    <span class="ml-[4px] text-[10px] font-bold text-[#808080]">{{ countLabel(team.length, t.member, t.members) }}</span>
                  </div>
                  <article class="task-card bg-white p-[20px] rounded-[12px] flex flex-col gap-[10px]">
                    <div v-for="(m, i) in team" :key="i"
                         class="subtask-row flex items-start gap-[12px] py-[12px] px-[12px] rounded-[8px] bg-[#fafaf8] border border-[#eee]">
                      <img v-if="m.avatar_url" :src="m.avatar_url" alt=""
                           class="w-[40px] h-[40px] rounded-full object-cover shrink-0" />
                      <span v-else class="inline-flex w-[40px] h-[40px] items-center justify-center rounded-full bg-white border border-[#ddd] text-[15px] font-bold text-[#212121] shrink-0">
                        {{ ((pickField(m, 'name') || '')[0] || '?').toUpperCase() }}
                      </span>
                      <div class="flex flex-col flex-1 min-w-0">
                        <span class="text-[13px] font-bold text-[#212121] leading-tight">{{ pickField(m, 'name') }}</span>
                        <span class="text-[9px] font-bold uppercase tracking-[1.2px] text-[#808080] mt-[2px]">{{ pickField(m, 'role') }}</span>
                        <span v-if="pickField(m, 'bio')" class="text-[11px] text-[#5f6659] leading-[1.5] mt-[6px]">{{ pickField(m, 'bio') }}</span>
                      </div>
                    </div>
                  </article>
                </div>

                <!-- ROADMAP -->
                <div v-if="roadmap.length" class="flex flex-col gap-[16px]">
                  <div class="flex items-center gap-[8px]">
                    <span class="w-[18px] h-[18px] bg-[#9bff00] rounded-tr-[12px] rounded-bl-[3px] rounded-tl-[3px] rounded-br-[3px] inline-block" style="transform: rotate(15deg);"></span>
                    <span class="text-[16px] font-bold text-[#212121]">{{ t.milestones }}</span>
                  </div>
                  <article class="task-card bg-white p-[20px] rounded-[12px] flex flex-col gap-[8px]">
                    <div v-for="(r, i) in roadmap" :key="i"
                         class="subtask-row flex items-start gap-[10px] py-[10px] px-[12px] rounded-[8px] bg-[#fafaf8] border border-[#eee]">
                      <span :class="[
                        'inline-flex w-[22px] h-[22px] items-center justify-center rounded-full text-[10px] font-bold shrink-0 mt-[1px]',
                        r.status === 'done' ? 'bg-[#9bff00] text-[#212121]' :
                        r.status === 'active' ? 'bg-[#fff7e6] border border-[#ffd591] text-[#fa8c16]' :
                                                'bg-white border border-[#ddd] text-[#808080]'
                      ]">
                        <span v-if="r.status === 'done'" class="material-symbols-outlined text-[13px]" style="font-variation-settings: 'FILL' 1;">check</span>
                        <span v-else>{{ i + 1 }}</span>
                      </span>
                      <div class="flex flex-col flex-1 min-w-0">
                        <div class="flex items-center justify-between gap-[6px]">
                          <span class="text-[9px] font-bold uppercase tracking-[1.2px] text-[#808080]">{{ pickField(r, 'quarter') }}</span>
                          <span :class="[
                            'text-[9px] font-bold uppercase tracking-[1px] px-[6px] py-[1px] rounded-full',
                            r.status === 'done' ? 'bg-[#9bff00]/30 text-[#1aa64a]' :
                            r.status === 'active' ? 'bg-[#fff7e6] border border-[#ffd591] text-[#fa8c16]' :
                                                    'bg-white border border-[#ddd] text-[#808080]'
                          ]">{{ statusLabel(r.status) }}</span>
                        </div>
                        <span class="text-[12px] font-bold text-[#212121] mt-[2px] leading-tight">{{ pickField(r, 'title') }}</span>
                        <span v-if="pickField(r, 'desc')" class="text-[10px] text-[#5f6659] mt-[4px] leading-[1.5]">{{ pickField(r, 'desc') }}</span>
                      </div>
                    </div>
                  </article>
                </div>

                <!-- TOKENOMICS -->
                <div v-if="tokenomics.allocations?.length" class="flex flex-col gap-[16px]">
                  <div class="flex items-center gap-[8px]">
                    <span class="w-[18px] h-[18px] bg-[#9bff00] rounded-tr-[12px] rounded-bl-[3px] rounded-tl-[3px] rounded-br-[3px] inline-block" style="transform: rotate(15deg);"></span>
                    <span class="text-[16px] font-bold text-[#212121]">{{ t.tokenomics }}</span>
                  </div>
                  <article class="task-card bg-white p-[20px] rounded-[12px] flex flex-col gap-[14px]">
                    <div class="grid grid-cols-3 gap-[8px]">
                      <div v-for="(kv, i) in [
                             { icon: 'rocket_launch', color: '#1aa64a', bg: '#9bff00/30', label: 'TGE', val: (tokenomics.tge_percent || 0) + '%' },
                             { icon: 'hourglass_top', color: '#fa8c16', bg: '#fff7e6', label: t.cliff, val: (tokenomics.cliff_months || 0) + t.monthSuffix },
                             { icon: 'timeline', color: '#3884ff', bg: '#3884ff/15', label: t.vest, val: (tokenomics.linear_months || 0) + t.monthSuffix },
                           ]" :key="i"
                           class="flex flex-col items-center py-[10px] px-[6px] rounded-[8px] bg-[#fafaf8] border border-[#eee]">
                        <span class="material-symbols-outlined text-[16px] mb-[4px]" :style="{ color: kv.color, fontVariationSettings: `'FILL' 1` }">{{ kv.icon }}</span>
                        <span class="text-[8px] font-bold uppercase tracking-[1.2px] text-[#808080]">{{ kv.label }}</span>
                        <span class="text-[15px] font-bold text-[#212121] tabular-nums mt-[2px]">{{ kv.val }}</span>
                      </div>
                    </div>
                    <div class="flex flex-col gap-[8px]">
                      <p class="text-[9px] font-bold uppercase tracking-[1.2px] text-[#808080]">{{ t.distribution }}</p>
                      <div v-for="(a, i) in tokenomics.allocations" :key="i" class="flex flex-col gap-[4px]">
                        <div class="flex items-center justify-between">
                          <span class="text-[11px] font-bold text-[#212121] truncate pr-[8px]">{{ pickField(a, 'label') }}</span>
                          <span class="text-[11px] font-bold text-[#212121] tabular-nums shrink-0">{{ a.percent }}%</span>
                        </div>
                        <div class="progress-track" style="height: 6px;">
                          <div class="progress-fill" :style="{ width: (a.percent || 0) + '%', background: a.color || '#9bff00' }"></div>
                        </div>
                      </div>
                    </div>
                  </article>
                </div>

                <!-- ONCHAIN -->
                <div v-if="onchain.length" class="flex flex-col gap-[16px]">
                  <div class="flex items-center gap-[8px]">
                    <span class="w-[18px] h-[18px] bg-[#9bff00] rounded-tr-[12px] rounded-bl-[3px] rounded-tl-[3px] rounded-br-[3px] inline-block" style="transform: rotate(15deg);"></span>
                    <span class="text-[16px] font-bold text-[#212121]">{{ t.onchain }}</span>
                    <span class="ml-[4px] text-[10px] font-bold text-[#808080]">{{ stage.key === 'active' ? t.liveOnchain : t.afterTge }}</span>
                  </div>
                  <article class="task-card bg-white p-[20px] rounded-[12px] flex flex-col gap-[8px]">
                    <div v-for="(o, i) in onchain" :key="i"
                         class="subtask-row flex items-center gap-[10px] py-[10px] px-[12px] rounded-[8px] bg-[#fafaf8] border border-[#eee] hover:bg-white hover:border-[#9bff00] transition-colors cursor-pointer"
                         @click="openExplorer(o.address, o.chain)">
                      <span class="material-symbols-outlined text-[#1aa64a] text-[16px] shrink-0" style="font-variation-settings: 'FILL' 1;">verified_user</span>
                      <div class="flex flex-col flex-1 min-w-0">
                        <span class="text-[12px] font-bold text-[#212121]">{{ pickField(o, 'label') }}</span>
                        <span class="text-[10px] font-mono text-[#808080] truncate">{{ o.address || '(' + t.pending + ')' }}</span>
                      </div>
                      <span class="text-[9px] font-bold uppercase tracking-[1.2px] text-[#808080] shrink-0">{{ o.chain || 'BSC' }}</span>
                    </div>
                  </article>
                </div>

              </aside>
            </div>
          </section>

          <HomeFooter :language="props.language" full-bleed :scale="scale" />
        </template>
      </main>
    </div>
  </div>
</template>

<style scoped>
/* 全局 jc-canvas-host / jc-canvas-1440 / [data-reveal] 由 src/styles/jc-shell.css 提供，此处不重写 */
.jc-canvas-host {
  max-width: 100vw;
  overflow-x: clip;
}

.jc-canvas-1440,
.task-card,
.subtask-row,
.prose-jc {
  max-width: 100%;
  min-width: 0;
  box-sizing: border-box;
}

.material-symbols-outlined {
  font-variation-settings: 'FILL' 0, 'wght' 400, 'GRAD' 0, 'opsz' 24;
}

/* section 需要显式 w-full 撑满 canvas 的 flex-column-center 父 */
main.jc-canvas-1440 > section,
main.jc-canvas-1440 > footer {
  width: 100%;
  align-self: stretch;
}

/* task-card hover 与 TaskHall 一致 */
.task-card {
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.04);
  transition: box-shadow 0.22s cubic-bezier(0, 0, 0.2, 1), transform 0.22s cubic-bezier(0, 0, 0.2, 1);
}

/* 左栏顶部 cover 大图：轻微阴影 + hover 微上浮 */
.cover-hero {
  box-shadow: 0 6px 20px -4px rgba(0, 0, 0, 0.10);
  transition: transform 0.28s cubic-bezier(0.22, 1, 0.36, 1), box-shadow 0.28s ease;
}
.cover-hero:hover {
  transform: translateY(-2px);
  box-shadow: 0 14px 30px -6px rgba(0, 0, 0, 0.18);
}
.jc-canvas-host.home-dark .cover-hero {
  box-shadow: 0 6px 24px -6px rgba(0, 0, 0, 0.55);
}
.task-card:hover {
  box-shadow: 0 10px 28px -4px rgba(0, 0, 0, 0.10);
}

/* subtask-row hover 微反馈 */
.subtask-row {
  transition: background-color 0.18s ease, border-color 0.18s ease, transform 0.18s ease;
}
.subtask-row:hover {
  background-color: #ffffff;
  border-color: #ddd;
}

/* progress-track 与 TaskHall 一致 */
.progress-track {
  position: relative;
  width: 100%;
  height: 8px;
  border-radius: 999px;
  background: #f0efec;
  overflow: hidden;
}
.progress-fill {
  height: 100%;
  border-radius: 999px;
  background: #9bff00;
  transition: width 0.7s cubic-bezier(0.22, 1, 0.36, 1);
}

/* Rich text */
.prose-jc :deep(h1) { font-size: 28px; font-weight: 700; margin: 0.9em 0 0.4em; color: #212121; }
.prose-jc :deep(h2) { font-size: 22px; font-weight: 700; margin: 0.8em 0 0.4em; color: #212121; }
.prose-jc :deep(h3) { font-size: 17px; font-weight: 700; margin: 0.7em 0 0.3em; color: #212121; }
.prose-jc :deep(p) { margin: 0.55em 0; line-height: 1.7; color: #212121; font-size: 15px; }
.prose-jc :deep(ul), .prose-jc :deep(ol) { padding-left: 24px; margin: 0.55em 0; }
.prose-jc :deep(li) { margin: 0.2em 0; line-height: 1.7; }
.prose-jc :deep(blockquote) { border-left: 3px solid #9bff00; padding-left: 12px; color: #5f6659; margin: 0.55em 0; }
.prose-jc :deep(code) { background: #fafaf8; padding: 1px 6px; border-radius: 4px; font-size: 90%; border: 1px solid #eee; }
.prose-jc :deep(img) { max-width: 100%; border-radius: 10px; margin: 0.8em 0; }
.prose-jc :deep(a) { color: #1aa64a; text-decoration: underline; font-weight: 700; }
.prose-jc :deep(strong) { color: #212121; font-weight: 700; }
.prose-jc :deep(table) { border-collapse: collapse; width: 100%; margin: 0.55em 0; font-size: 14px; }
.prose-jc :deep(th), .prose-jc :deep(td) { border: 1px solid #eee; padding: 8px 12px; }
.prose-jc :deep(th) { background: #fafaf8; font-weight: 700; }
.prose-jc :deep(*) {
  max-width: 100%;
  overflow-wrap: anywhere;
  word-break: break-word;
  box-sizing: border-box;
}
.prose-jc :deep(table) {
  display: block;
  overflow-x: auto;
}

/* ============================================================
   暗色主题（与 jc-shell.css / TaskHall 一致的深灰 + 半透白边）
   全部走 scoped 直接 selector（TaskHall 相同模式），Vue 编译后
   `.jc-canvas-host.home-dark[data-v-xxx] .task-card[data-v-xxx]`
   ============================================================ */

/* section 里 bg-[#faf7f0] zebra 板块在暗底换深灰 */
.jc-canvas-host.home-dark section[class*="bg-[#faf7f0]"] { background-color: #191919 !important; }

/* task-card 白底 → 深灰 + 半透白边 */
.jc-canvas-host.home-dark .task-card {
  background: #1f1f1f !important;
  border: 1px solid rgba(255, 255, 255, 0.06) !important;
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.35) !important;
}
.jc-canvas-host.home-dark .task-card:hover { box-shadow: 0 10px 28px -4px rgba(0, 0, 0, 0.5) !important; }

/* subtask-row 米白底 → 深灰；hover → 品牌绿描边 */
.jc-canvas-host.home-dark .subtask-row {
  background: #262626 !important;
  border-color: rgba(255, 255, 255, 0.08) !important;
}
.jc-canvas-host.home-dark .subtask-row:hover {
  background-color: #2f2f2f !important;
  border-color: rgba(155, 255, 0, 0.32) !important;
}

/* 通用米白底 (tokenomics 三小卡等，不带 .subtask-row 的 element) → 深灰 */
.jc-canvas-host.home-dark [class*="bg-[#fafaf8]"] {
  background-color: #262626 !important;
  border-color: rgba(255, 255, 255, 0.08) !important;
}

/* subtask-row 里的白底小圆（头像 / 序号 / 图标框）→ 深灰 */
.jc-canvas-host.home-dark .subtask-row [class*="bg-white"] {
  background: #333 !important;
  border-color: rgba(255, 255, 255, 0.15) !important;
}

/* progress-track 浅灰底 → 深灰 */
.jc-canvas-host.home-dark .progress-track { background: #333 !important; }

/* Hero 里的 countdown 数字块 bg-white → 深灰 */
.jc-canvas-host.home-dark [class*="bg-white"][class*="border-[#eee]"] {
  background: #262626 !important;
  border-color: rgba(255, 255, 255, 0.1) !important;
}

/* rise-card stats 卡片白底 */
.jc-canvas-host.home-dark .rise-card {
  background: #1f1f1f !important;
  border-color: rgba(255, 255, 255, 0.08) !important;
}
.jc-canvas-host.home-dark .rise-card [class*="border-r"] {
  border-color: rgba(255, 255, 255, 0.08) !important;
}

/* Watchlist pill bg-white rounded-full → 深灰 */
.jc-canvas-host.home-dark button[class*="bg-white"][class*="rounded-full"] {
  background: #262626 !important;
  border-color: rgba(255, 255, 255, 0.15) !important;
}

/* rounded-full bg-white/90 backdrop-blur → 深灰 (candidate 卡片 badges) */
.jc-canvas-host.home-dark [class*="bg-white/90"] {
  background-color: rgba(38, 38, 38, 0.85) !important;
  color: #fafaf7 !important;
}

/* Risks 卡片：暖米黄 → 暖深灰 + 橙描边 */
.jc-canvas-host.home-dark [class*="bg-[#fff7e6]"] {
  background-color: #2a221a !important;
  border-color: rgba(250, 140, 22, 0.4) !important;
}
.jc-canvas-host.home-dark [class*="bg-[#fff7e6]"] [class*="bg-white"] {
  background-color: #3a2f1f !important;
  border-color: rgba(250, 140, 22, 0.35) !important;
}

/* 文字颜色：主标题 / 正文 / 副色 dark 变体 */
.jc-canvas-host.home-dark [class*="text-[#212121]"] { color: #fafaf7 !important; }
.jc-canvas-host.home-dark [class*="text-[#5f6659]"] { color: #b8b8b0 !important; }

/* Hero 大标题里的 tagline (playfair italic 副标题) 保持副色，防止太抢戏 */
.jc-canvas-host.home-dark .font-playfair { color: #b8b8b0 !important; }

/* Hero 淡淡分隔线（linear-gradient rgba(0,0,0,0.08)）在暗底改成淡白色 */
.jc-canvas-host.home-dark [style*="rgba(0,0,0,0.08)"] {
  background: linear-gradient(to right, transparent 0%, rgba(255,255,255,0.15) 12%, rgba(255,255,255,0.15) 88%, transparent 100%) !important;
}

/* white-pill-btn 主 CTA（品牌绿）在暗底本身醒目，保持不动 */

/* prose-jc 富文本 dark 变体 */
.jc-canvas-host.home-dark .prose-jc :deep(h1),
.jc-canvas-host.home-dark .prose-jc :deep(h2),
.jc-canvas-host.home-dark .prose-jc :deep(h3) { color: #fafaf7 !important; }
.jc-canvas-host.home-dark .prose-jc :deep(p) { color: #dcdcd6 !important; }
.jc-canvas-host.home-dark .prose-jc :deep(strong) { color: #ffffff !important; }
.jc-canvas-host.home-dark .prose-jc :deep(blockquote) { color: #a0a09a !important; }
.jc-canvas-host.home-dark .prose-jc :deep(code) {
  background: #2a2a2a !important;
  border-color: rgba(255, 255, 255, 0.08) !important;
  color: #e8e8e0 !important;
}
.jc-canvas-host.home-dark .prose-jc :deep(th) {
  background: #2a2a2a !important;
  color: #fafaf7 !important;
}
.jc-canvas-host.home-dark .prose-jc :deep(td),
.jc-canvas-host.home-dark .prose-jc :deep(th) { border-color: rgba(255, 255, 255, 0.1) !important; }
.jc-canvas-host.home-dark .prose-jc :deep(a) { color: #9bff00 !important; }
.jc-canvas-host.home-dark .prose-jc :deep(li) { color: #dcdcd6 !important; }

/* Footer 是 jc-dark-strip 本身深色，保持 */

/* ============ Mobile overrides (<=768px) ============
   canvas 缩放已在 jc-shell.css 里禁用；此处覆盖所有 fixed pixel width
   和两栏 flex 布局，让页面在窄屏成为竖排流式版本 */
@media (max-width: 768px) {
  .jc-canvas-host {
    width: 100%;
    overflow-x: hidden;
  }
  .jc-canvas-host .jc-scaled-wrapper,
  .jc-canvas-host .jc-canvas-1440 {
    width: 100% !important;
    max-width: 100% !important;
  }
  .jc-canvas-host .jc-canvas-1440 > * {
    width: 100% !important;
    max-width: 100% !important;
    box-sizing: border-box !important;
    padding-left: 20px !important;
    padding-right: 20px !important;
    padding-top: 40px !important;
    padding-bottom: 40px !important;
  }
  /* Hero 88px 大标题 → 32px */
  .jc-canvas-1440 section > div[style*="font-size: 88px"] {
    font-size: 32px !important;
    line-height: 1.1 !important;
    letter-spacing: -0.5px !important;
    width: 100% !important;
  }
  .jc-canvas-1440 section > div[style*="font-size: 88px"] p[style*="font-size: 40px"] {
    font-size: 18px !important;
    line-height: 1.3 !important;
  }
  /* Hero 内 sub + Stats 横排 → 竖排 */
  .jc-canvas-1440 section > div.jc-hero-anim-3 {
    flex-direction: column !important;
    align-items: stretch !important;
    gap: 16px !important;
  }
  .jc-canvas-1440 section > div.jc-hero-anim-3 > div:first-child {
    width: 100% !important;
  }
  /* Stats card 保留但压缩 */
  .jc-canvas-1440 .rise-card-1[class*="items-stretch"] {
    flex-wrap: wrap !important;
  }
  .jc-canvas-1440 .rise-card-1[class*="items-stretch"] > div {
    min-width: 0 !important;
    flex: 1 1 45% !important;
    padding: 12px 14px !important;
  }
  .jc-canvas-1440 .rise-card-1[class*="items-stretch"] span[class*="text-[30px]"] {
    font-size: 20px !important;
  }
  /* Body 两栏 → 竖排 */
  .launchpad-body-layout {
    flex-direction: column !important;
    align-items: stretch !important;
    gap: 32px !important;
  }
  .launchpad-body-main {
    width: 100% !important;
    max-width: 100% !important;
    min-width: 0 !important;
    flex: 0 1 auto !important;
  }
  .right-rail {
    width: 100% !important;
    max-width: 100% !important;
    min-width: 0 !important;
  }
  /* Task-card padding 缩小 */
  .task-card {
    width: 100% !important;
    max-width: 100% !important;
    min-width: 0 !important;
    overflow: hidden !important;
    padding: 18px !important;
  }
  .task-card h3 { font-size: 16px !important; }
  .prose-jc,
  .prose-jc :deep(h1),
  .prose-jc :deep(h2),
  .prose-jc :deep(h3),
  .prose-jc :deep(p),
  .prose-jc :deep(li) {
    max-width: 100% !important;
    min-width: 0 !important;
  }
  .prose-jc :deep(h1) { font-size: 24px !important; line-height: 1.25 !important; }
  .prose-jc :deep(h2) { font-size: 20px !important; line-height: 1.3 !important; }
  .prose-jc :deep(p), .prose-jc :deep(li) { font-size: 14px !important; line-height: 1.75 !important; }
  /* Cover hero 高度压缩 */
  .cover-hero {
    max-width: 100% !important;
  }
  .cover-hero img {
    width: 100% !important;
    height: 200px !important;
  }
  .cover-hero .absolute {
    padding: 16px !important;
  }
  .cover-hero .absolute [class*="text-[20px]"] { font-size: 16px !important; }
  /* Section 板块 title 20 → 14 */
  .jc-canvas-1440 span[class*="text-[20px] font-bold"] { font-size: 14px !important; letter-spacing: 1.2px !important; }
  .jc-canvas-1440 span[class*="text-[16px] font-bold"] { font-size: 13px !important; letter-spacing: 1.2px !important; }
  /* PixelGrid 隐藏（装饰不重要 且会溢出）*/
  .jc-canvas-1440 [class*="pointer-events-none"][class*="z-10"]:has(.pixel-grid),
  .jc-canvas-1440 section > div.absolute[style*="top: 60px"] { display: none !important; }
  /* Footer 内的 3 列地址 → 竖排 */
  footer > div[class*="flex gap-[80px]"] {
    flex-direction: column !important;
    gap: 20px !important;
    padding-left: 8px !important;
    padding-right: 8px !important;
  }
  footer img[alt="JoinCare"] { height: 48px !important; }
  /* Tokenomics 3 列 → 保持 3 列但压缩 */
  .task-card .grid-cols-3 > div { padding: 6px 4px !important; }
  .task-card .grid-cols-3 span[class*="text-[15px]"] { font-size: 12px !important; }
  /* Roadmap 内每行 items-start */
  .subtask-row {
    max-width: 100% !important;
    min-width: 0 !important;
    flex-wrap: nowrap !important;
  }
  .subtask-row > * {
    min-width: 0 !important;
  }
}
</style>
