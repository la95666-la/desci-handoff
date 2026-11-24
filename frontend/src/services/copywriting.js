/**
 * C 端运行时文案覆盖层：app boot 时拉取后端 copywriting_config（en + tw），
 * 按 "<page>.<key>" 前缀路由到对应 i18n/*.js 的 TRANSLATIONS 对象中覆盖。
 *
 * 用法：在 main.js mount 前调用 bootstrapCopywriting()。
 * 失败静默 — 任何错误都回退到内置 TRANSLATIONS 静态文案。
 *
 * key 命名规范（必须）：
 *   navbar.foo → src/i18n/navbar.js  TRANSLATIONS[lang].foo
 *   home.bar   → src/i18n/home.js    TRANSLATIONS[lang].bar
 *   ...同 ecosystem / launchpad / taskhall / projectapply
 *
 * 不带前缀或前缀不在白名单的 key 会被忽略（不会污染任何文件）。
 */
import { TRANSLATIONS as ECOSYSTEM_T } from '../i18n/ecosystem.js'
import { TRANSLATIONS as HOME_T } from '../i18n/home.js'
import { TRANSLATIONS as LAUNCHPAD_T } from '../i18n/launchpad.js'
import { TRANSLATIONS as NAVBAR_T } from '../i18n/navbar.js'
import { TRANSLATIONS as PROJECTAPPLY_T } from '../i18n/projectapply.js'
import { TRANSLATIONS as TASKHALL_T } from '../i18n/taskhall.js'
import { getCopywritingList } from './api'

const REGISTRY = {
  navbar: NAVBAR_T,
  home: HOME_T,
  ecosystem: ECOSYSTEM_T,
  launchpad: LAUNCHPAD_T,
  taskhall: TASKHALL_T,
  projectapply: PROJECTAPPLY_T,
}

function applyOverride(map, lang) {
  if (!map || typeof map !== 'object') return 0
  let applied = 0
  for (const [fullKey, value] of Object.entries(map)) {
    const dot = fullKey.indexOf('.')
    if (dot < 0) continue
    const prefix = fullKey.slice(0, dot)
    const localKey = fullKey.slice(dot + 1)
    const target = REGISTRY[prefix]
    if (!target || !target[lang]) continue
    target[lang][localKey] = value
    applied++
  }
  return applied
}

/**
 * 拉两份语言，合并进 TRANSLATIONS。
 * 失败不抛错，仅 console.warn。
 *
 * @returns {Promise<{en: number, tw: number}>} 各语言成功覆盖的 key 数
 */
export async function bootstrapCopywriting() {
  // The dedicated Fair Launch browser fixture intentionally does not start or
  // trust the normal Core API. Its signed, run-bound API serves only Fair
  // Launch routes, so waiting for ordinary copywriting requests here would
  // leave the Vue root unmounted until an unrelated network timeout. Static
  // translations are the authoritative fallback for this LOCAL-ONLY lane.
  if (import.meta.env?.VITE_FAIR_LAUNCH_TEST_MODE === 'local_test' &&
      import.meta.env?.VITE_FAIR_LAUNCH_LOCAL_WALLET_SUBMISSION === 'true' &&
      /^http:\/\/127\.0\.0\.1:\d+$/.test(String(import.meta.env?.VITE_FAIR_LAUNCH_LOCAL_API_URL || ''))) {
    return { en: 0, tw: 0 }
  }
  try {
    const [enMap, twMap] = await Promise.all([
      getCopywritingList({ language: 'en' }).catch(() => ({})),
      getCopywritingList({ language: 'tw' }).catch(() => ({})),
    ])
    const en = applyOverride(enMap, 'en')
    const tw = applyOverride(twMap, 'tw')
    if (en > 0 || tw > 0) {
      console.info(`[copywriting] 后端覆盖：EN ${en} keys · TW ${tw} keys`)
    }
    return { en, tw }
  } catch (e) {
    console.warn('[copywriting] bootstrap failed (using static fallback):', e?.message)
    return { en: 0, tw: 0 }
  }
}
