import { hasReachedSoftCap, hasUnixTimePassed } from './fairLaunch.js'

// Launchpad 与首页陈列整个公开生命周期；终结项目仍是可回顾、可领取/退款的
// 正式项目，不应从列表消失。后端 Setup 是尚未 Set 的内部状态，但对外属于
// PreLaunch；candidate/archived 等候选或封存状态仍排除。
const LAUNCHPAD_VISIBLE_STATUSES = new Set([
  'active',
  'warmup',
  'pre_launch',
  'set',
  'pending',
  'upcoming',
  'live',
  'successful',
  'successful_pending_release',
  'failed',
  'canceled',
  'cancelled',
  'ended',
  'refund',
  'closed',
  'sold_out',
  'vesting',
  'v3_reorg_halted',
])
// Launchpad 的公开生命周期统一使用同一套新版详情 UI。candidate 仍保留候选项目
// 语义；其余历史别名在这里兼容，避免终结态回落到旧模板。
const UNIFIED_DETAIL_STATUSES = new Set([
  'active',
  'warmup',
  'pre_launch',
  'set',
  'pending',
  'upcoming',
  'live',
  'successful',
  'successful_pending_release',
  'failed',
  'canceled',
  'cancelled',
  'ended',
  'refund',
  'closed',
  'sold_out',
  'vesting',
  'v3_reorg_halted',
])

export function normalizeProjectStatus(status) {
  const normalized = String(status ?? '').trim().replace(/([a-z0-9])([A-Z])/g, '$1_$2').toLowerCase()
  return normalized === 'setup' ? 'pre_launch' : normalized
}

// 统一的项目阶段派生（唯一事实源）：列表页 / 详情页 / 卡片 / 面板都用它。
// 后端 startTime/endTime 为 Unix 秒，此处转毫秒。
// 返回：pre_launch | warmup | pending | candidate | upcoming | live |
//       ended_success | ended_failed | successful_pending_release | successful | failed | canceled | ended | refund
const TERMINAL_STAGES = new Set(['successful', 'successful_pending_release', 'failed', 'canceled', 'ended', 'refund'])

export function deriveProjectStage(project, nowMs = Date.now()) {
  let status = normalizeProjectStatus(project?.status)
  const start = project?.startTime ?? project?.start_time
  const end = project?.endTime ?? project?.end_time
  const started = hasUnixTimePassed(start, nowMs)
  const ended = hasUnixTimePassed(end, nowMs)
  const softCapReached = hasReachedSoftCap(project)

  if (status === 'cancelled') status = 'canceled'
  if (status === 'v3_reorg_halted') return 'syncing'
  if (status === 'sold_out' || status === 'vesting') return 'successful'
  if (status === 'closed') {
    return softCapReached ? 'ended_success' : 'ended_failed'
  }
  if (TERMINAL_STAGES.has(status)) return status

  if (status === 'active') {
    if (String(start ?? '').trim() !== '' && String(start).trim() !== '0' && !started) return 'upcoming'
    if (ended) return softCapReached ? 'ended_success' : 'ended_failed'
    return 'live'
  }

  return status || 'pre_launch'
}

// 倒计时目标必须跟随派生阶段，而不是只看后端原始状态。
// 已激活但尚未到 startTime 的项目属于 upcoming，应倒计时到开始时间；
// 真正进入 live 后才倒计时到结束时间。
export function getProjectCountdownTarget(project, stage = deriveProjectStage(project)) {
  if (stage === 'upcoming') return Number(project?.startTime || 0)
  // Pending means the on-chain project exists but has not been activated.
  // Its funding start is authoritative; launchAt is only a marketing date.
  if (stage === 'pending') return Number(project?.startTime || 0)
  if (stage === 'live') return Number(project?.endTime || 0)
  return Number(project?.launchAt || 0)
}

// Pending and activated-but-not-started projects show terms + a start countdown,
// never wallet actions. The panel itself gates actions on the live stage.
export function isCappedFundingPanelStage(stage) {
  return stage === 'pending' || stage === 'upcoming' || stage === 'active' || stage === 'ended'
}

// 退款必须等待链上生命周期真正进入 Failed/Cancelled。
// ended_failed 只表示“销售时间已结束且展示数据低于软顶”，链上仍可能是 Active。
export function isRefundableProjectStage(stage) {
  return stage === 'failed' || stage === 'canceled' || stage === 'refund'
}

export function isLaunchpadVisibleStatus(status) {
  return LAUNCHPAD_VISIBLE_STATUSES.has(normalizeProjectStatus(status))
}

export function usesUnifiedProjectDetail(status) {
  return UNIFIED_DETAIL_STATUSES.has(normalizeProjectStatus(status))
}

export function isStalePreLaunchSaleClaim(value) {
  const text = String(value ?? '')
  const staleEnglish = /phase[\s-]?1.*no on-chain sale/i.test(text)
  const staleChinese = /(第一階段|本次\s*phase\s*1).*(無鏈上發售|無鏈上銷售|尚未開放鏈上發售)/i.test(text)
  return staleEnglish || staleChinese
}

export function getFundingProgress(totalRaised, hardCap) {
  const raised = Number(totalRaised)
  const cap = Number(hardCap)
  if (!Number.isFinite(raised) || !Number.isFinite(cap) || cap <= 0) return 0
  return Math.min(100, Math.max(0, Math.round((raised / cap) * 100)))
}
