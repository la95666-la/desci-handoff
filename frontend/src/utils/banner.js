/**
 * 启动 banner —— 控制台打印 ASCII 大字 + 环境信息。
 * 所有 __APP_*__ 常量由 vite.config.js `define` 注入。
 */

/* eslint-disable no-undef */
const VERSION = typeof __APP_VERSION__ !== 'undefined' ? __APP_VERSION__ : 'dev'
const GIT_SHA = typeof __APP_GIT_SHA__ !== 'undefined' ? __APP_GIT_SHA__ : 'unknown'
const BUILD_TIME = typeof __APP_BUILD_TIME__ !== 'undefined' ? __APP_BUILD_TIME__ : new Date().toISOString()
const ENV_NAME = typeof __APP_ENV_NAME__ !== 'undefined' ? __APP_ENV_NAME__ : 'local'
const API_BASE = typeof __APP_API_BASE__ !== 'undefined' ? __APP_API_BASE__ : ''
/* eslint-enable no-undef */

const ART = [
'     ██╗ ██████╗ ██╗███╗   ██╗ ██████╗ █████╗ ██████╗ ███████╗',
'     ██║██╔═══██╗██║████╗  ██║██╔════╝██╔══██╗██╔══██╗██╔════╝',
'     ██║██║   ██║██║██╔██╗ ██║██║     ███████║██████╔╝█████╗  ',
'██   ██║██║   ██║██║██║╚██╗██║██║     ██╔══██║██╔══██╗██╔══╝  ',
'╚█████╔╝╚██████╔╝██║██║ ╚████║╚██████╗██║  ██║██║  ██║███████╗',
' ╚════╝  ╚═════╝ ╚═╝╚═╝  ╚═══╝ ╚═════╝╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝',
]

export function printBanner() {
  if (typeof window === 'undefined') return
  const green = 'color:#9bff00;font-weight:bold;'
  const dim   = 'color:#8a9188;'
  const value = 'color:#212121;font-weight:bold;background:#9bff00;padding:2px 6px;border-radius:3px;'

  console.log('%c' + ART.join('\n'), green)
  console.log(
    `%cJoinCare Frontend  %c${ENV_NAME}%c  v${VERSION}  ·  ${GIT_SHA}  ·  ${BUILD_TIME}`,
    green, value, dim,
  )
  if (API_BASE) console.log('%cAPI  %c' + API_BASE, dim, 'color:#3884ff;')
  console.log('%cFund Medicine, Own the Future.', dim)
}
