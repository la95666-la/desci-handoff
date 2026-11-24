/**
 * API 服務層 — 封裝後端 HTTP 請求
 *
 * 功能：
 * 1. 統一的請求/響應處理
 * 2. 自動附加簽名 Headers（Jc-Sign, Jc-Signature, Jc-Time, Jc-Person, Jc-Request-Id, Jc-Language）
 * 3. JWT Token 管理
 * 4. HMAC-SHA256 請求簽名
 */

import { getEnvironmentConfig, getCurrentEnvironment } from '../config/environments.js'
import { backendLanguage } from '../utils/language.js'
import { normalizeFairLaunchPortfolioDto, normalizeFairLaunchPositionDto } from './fairLaunchApiContract.js'

// ============ 配置 ============

// 後端 API 基礎地址
// - local 環境：使用 ''，由 vite proxy 匹配 /client 直接轉發到後端
// - test/prod 環境：直連後端 API 地址
const envConfig = getEnvironmentConfig()
const API_BASE_URL = getCurrentEnvironment() === 'local' ? '' : (envConfig.apiBaseUrl || '')

function exactLocalFairLaunchApiAuthority() {
	try {
		if (import.meta.env?.DEV !== true || import.meta.env?.PROD === true || import.meta.env?.VITE_ENVIRONMENT !== 'local') return null
		const url = new URL(String(import.meta.env?.VITE_FAIR_LAUNCH_LOCAL_API_URL || ''))
		const runId = String(import.meta.env?.VITE_FAIR_LAUNCH_LOCAL_API_RUN_ID || '')
		const token = String(import.meta.env?.VITE_FAIR_LAUNCH_LOCAL_API_TOKEN || '')
		const expiresAt = String(import.meta.env?.VITE_FAIR_LAUNCH_LOCAL_API_EXPIRES_AT || '')
		if (url.protocol !== 'http:' || url.hostname !== '127.0.0.1' || !/^\d+$/.test(url.port) || url.pathname !== '/' || url.search || url.hash || url.username || url.password) return null
		if (!/^[0-9a-f-]{36}$/.test(runId) || !/^[0-9a-f]{64}$/.test(token) || Date.now() >= Date.parse(expiresAt)) return null
		return { baseUrl: url.origin, runId, token }
	} catch { return null }
}

export function hasFairLaunchLocalApiAuthority() {
	return exactLocalFairLaunchApiAuthority() !== null
}

async function localFairLaunchRequest(path, query = null) {
	const authority = exactLocalFairLaunchApiAuthority()
	if (!authority) throw new ApiError('durable_wallet_submission_unavailable', 'DURABLE_WALLET_SUBMISSION_UNAVAILABLE', 503)
	const url = new URL(path, authority.baseUrl)
	for (const [key, value] of Object.entries(query || {})) url.searchParams.set(key, String(value))
	const response = await fetch(url, { headers: { 'X-JC-FL-Run-ID': authority.runId, 'X-JC-FL-Capability': authority.token, 'X-JC-FL-Origin': String(import.meta.env?.VITE_FAIR_LAUNCH_LOCAL_FRONTEND_ORIGIN || '') } })
	const result = await response.json()
	if (!response.ok || result?.status !== true) throw new ApiError(result?.message || 'durable_wallet_submission_unavailable', result?.code || 'DURABLE_WALLET_SUBMISSION_UNAVAILABLE', response.status)
	return result.data
}

export async function applyFairLaunchLocalTransaction(projectID, transactionHash) {
	const authority = exactLocalFairLaunchApiAuthority()
	if (!authority) throw new ApiError('durable_wallet_submission_unavailable', 'DURABLE_WALLET_SUBMISSION_UNAVAILABLE', 503)
	const nonce = Array.from(crypto.getRandomValues(new Uint8Array(16)), (value) => value.toString(16).padStart(2, '0')).join('')
	const response = await fetch(new URL('/local/v1/transactions/apply', authority.baseUrl), {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', 'X-JC-FL-Run-ID': authority.runId, 'X-JC-FL-Capability': authority.token, 'X-JC-FL-Origin': String(import.meta.env?.VITE_FAIR_LAUNCH_LOCAL_FRONTEND_ORIGIN || '') },
		body: JSON.stringify({ nonce, projectId: Number(projectID), runId: authority.runId, transactionHash }),
	})
	const result = await response.json()
	if (!response.ok || result?.status !== true) throw new ApiError(result?.message || 'durable_wallet_submission_unavailable', result?.code || 'DURABLE_WALLET_SUBMISSION_UNAVAILABLE', response.status)
	return result.data
}

// ============ 工具方法 ============

/**
 * 生成 UUID 作為 Request ID
 */
function generateRequestId() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0
    const v = c === 'x' ? r : (r & 0x3) | 0x8
    return v.toString(16)
  })
}

// ============ 認證狀態（模塊級別緩存）============

const browserStorage = typeof localStorage === 'undefined' ? { getItem: () => null, setItem: () => {}, removeItem: () => {} } : localStorage
let _token = browserStorage.getItem('jc_token') || ''
let _uid = browserStorage.getItem('jc_uid') || ''
let _language = backendLanguage(browserStorage.getItem('language'))

/**
 * 設置認證資訊
 */
export function setAuth(token, uid) {
  _token = token
  _uid = String(uid)
  browserStorage.setItem('jc_token', token)
  browserStorage.setItem('jc_uid', _uid)
}

/**
 * 清除認證資訊
 */
export function clearAuth() {
  _token = ''
  _uid = ''
  browserStorage.removeItem('jc_token')
  browserStorage.removeItem('jc_uid')
}

/**
 * 獲取當前認證狀態
 */
export function getAuth() {
  return { token: _token, uid: _uid }
}

/**
 * 判斷是否已登入（直接讀 localStorage，避免 HMR 導致模塊變量不同步）
 */
export function isAuthenticated() {
  const token = browserStorage.getItem('jc_token')
  const uid = browserStorage.getItem('jc_uid')
  return !!token && !!uid
}

/**
 * 設置語言
 */
export function setLanguage(lang) {
  _language = backendLanguage(lang)
}

// ============ 簽名生成 ============

/**
 * 生成請求簽名
 * 算法: HMAC-SHA256(key=JWT, body + requestURI + requestId + token後8位 + uid + timestamp)
 *
 * @param {string} method - HTTP 方法
 * @param {string} url - 請求路徑（含 query string）
 * @param {string} body - 請求體字符串
 * @param {string} requestId - 請求 ID
 * @param {string} timestamp - 時間戳（秒）
 */
function generateSignature(method, url, body, requestId, timestamp) {
  let signStr = ''

  // body: GET 請求不含 body
  if (method !== 'GET' && body && body !== '""') {
    signStr += body
  }

  // requestURI: 包含 query string 的路徑
  signStr += url

  // requestId
  signStr += requestId

  // token 後 8 位
  if (_token) {
    signStr += _token.slice(-8)
  }

  // uid
  signStr += _uid

  // timestamp
  signStr += timestamp

  if (!globalThis.crypto?.subtle) {
    throw new Error('Secure Web Crypto is required for authenticated requests')
  }
  return hmacSha256Hex(_token, signStr)
}

// HMAC-SHA256 hex（WebCrypto，需安全上下文 https / localhost）
async function hmacSha256Hex(key, message) {
  const enc = new TextEncoder()
  const cryptoKey = await crypto.subtle.importKey(
    'raw',
    enc.encode(key),
    { name: 'HMAC', hash: 'SHA-256' },
    false,
    ['sign']
  )
  const sig = await crypto.subtle.sign('HMAC', cryptoKey, enc.encode(message))
  return Array.from(new Uint8Array(sig), (b) => b.toString(16).padStart(2, '0')).join('')
}

// ============ 核心請求方法 ============

/**
 * 通用請求方法
 * @param {string} method - HTTP 方法
 * @param {string} path - 請求路徑（不含 base URL）
 * @param {object} options - 配置
 * @param {object} options.body - 請求體（POST/PUT）
 * @param {object} options.query - 查詢參數
 * @param {boolean} options.withSign - 是否需要簽名（預設 true）
 * @returns {Promise<any>} - 響應數據
 */
async function request(method, path, options = {}) {
  const { body = null, query = null, withSign = true, noStore = false } = options

  // 構建 URL
  let url = path
  if (query) {
    const params = new URLSearchParams()
    Object.entries(query).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== '') {
        params.append(key, String(value))
      }
    })
    const queryStr = params.toString()
    if (queryStr) {
      url += '?' + queryStr
    }
  }

  const fullUrl = API_BASE_URL + url

  // 構建 Headers
  const headers = {
    'Content-Type': 'application/json',
    'Jc-Language': _language
  }

  // 添加簽名 Headers
  if (withSign && _token) {
    const requestId = generateRequestId()
    const timestamp = String(Math.floor(Date.now() / 1000))
    const bodyStr = body ? JSON.stringify(body) : ''

    headers['Jc-Sign'] = _token
    headers['Jc-Person'] = _uid
    headers['Jc-Request-Id'] = requestId
    headers['Jc-Time'] = timestamp
    headers['Jc-Signature'] = await generateSignature(method, url, bodyStr, requestId, timestamp)
  }

  // 發起請求
  const fetchOptions = {
    method,
    headers
  }
  if (noStore) fetchOptions.cache = 'no-store'

  if (body && method !== 'GET') {
    fetchOptions.body = JSON.stringify(body)
  }

  try {
    const response = await fetch(fullUrl, fetchOptions)

    // 處理 HTTP 錯誤狀態
    if (response.status === 401) { clearAuth(); throw new ApiError('未授權，請重新登入', '401', 401) }
    if (response.status === 428) { clearAuth(); throw new ApiError('登入已過期，請重新登入', '428', 428) }
    if (response.status === 403) { throw new ApiError('訪問被拒絕', '403', 403) }

    // 解析響應
    const result = await response.json()

    // 業務錯誤處理
    if (!result.status) {
      throw new ApiError(
        result.message || '請求失敗',
        result.code || 'UNKNOWN',
        response.status
      )
    }

    return result.data
  } catch (error) {
    if (error instanceof ApiError) throw error
    // 网络层错误依然报警 —— 网络不通是运维要能看到的信号
    console.warn(`[API] ${method} ${path} network error:`, error?.message)
    throw error
  }
}

/**
 * API 錯誤類
 */
export class ApiError extends Error {
  constructor(message, code, httpStatus) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.httpStatus = httpStatus
  }
}

// ============ API 方法 ============

/**
 * 獲取錢包簽名 nonce 和後端簽名文案 — 不需要簽名
 * GET /auth/v1/nonce
 */
export async function getAuthNonce(walletAddress, action = '') {
  return await request('GET', '/client/auth/v1/nonce', {
    query: { walletAddress, action },
    withSign: false
  })
}

/**
 * 查詢錢包是否已註冊 — 不需要簽名
 * GET /login/v1/registrationStatus
 */
export async function getRegistrationStatus(walletAddress) {
  return await request('GET', '/client/login/v1/registrationStatus', {
    query: { walletAddress },
    withSign: false
  })
}

/**
 * 註冊錢包用戶 — 不需要請求簽名，body 內包含錢包簽名
 * POST /login/v1/register
 */
export async function registerWallet(payload) {
  const data = await request('POST', '/client/login/v1/register', {
    body: payload,
    withSign: false
  })
  if (data && data.signature) {
    setAuth(data.signature, data.uid)
  }
  return data
}

/**
 * 錢包登入 — 不需要請求簽名，body 內包含錢包簽名
 * POST /login/v1/login
 */
export async function loginWallet(payload) {
  const data = await request('POST', '/client/login/v1/login', {
    body: payload,
    withSign: false
  })
  if (data && data.signature) {
    setAuth(data.signature, data.uid)
  }
  return data
}

/**
 * 自動登入 — 需要簽名
 * POST /login/v1/autoLogin
 */
export async function autoLogin() {
  const data = await request('POST', '/client/login/v1/autoLogin', {
    body: {},
    withSign: true
  })
  // 刷新 Token
  if (data && data.signature) {
    setAuth(data.signature, data.uid)
  }
  return data
}

/**
 * 獲取當前用戶資料 — 需要簽名
 * GET /auth/v1/profile
 */
export async function getProfile() {
  return await request('GET', '/client/auth/v1/profile', {
    withSign: true
  })
}

/**
 * 獲取項目列表 — 不需要簽名
 * GET /project/v1/list
 */
export async function getProjectList(cursor = 0, status = '') {
  const query = { cursor }
  if (status) {
    query.status = status
  }
	if (exactLocalFairLaunchApiAuthority()) {
		return await localFairLaunchRequest('/client/project/v1/list', query)
	}
  return await request('GET', '/client/project/v1/list', {
    query,
    withSign: false
  })
}

/**
 * 獲取項目詳情 — 不需要簽名
 * GET /project/v1/detail
 */
export async function getProjectDetail(projectID, { fresh = false } = {}) {
	if (!fresh && exactLocalFairLaunchApiAuthority()) {
		return await localFairLaunchRequest('/client/project/v1/detail', { projectID })
	}
  return await request('GET', '/client/project/v1/detail', {
    query: { projectID },
    withSign: false,
    noStore: fresh,
  })
}

/**
 * 獲取用戶在指定項目的認購狀態（支付幣餘額 / 授權額度 / 已貢獻等），
 * 由後端按登錄錢包地址解析，不依賴瀏覽器錢包 provider。
 * GET /project/v1/:projectID/fundingState
 */
export async function getFundingState(projectID) {
  return await request('GET', `/client/project/v1/${projectID}/fundingState`, {
    withSign: true
  })
}

/** M3 single-wallet proof; never cache or substitute a Warmup signup. */
export async function getVerifiedV3Wallet(wallet) {
  if (!/^0x[0-9a-fA-F]{40}$/.test(String(wallet))) throw new ApiError('V3 wallet identity unavailable', 'IDO_V3_WALLET_CHANGED', 400)
  const response = await request('GET', '/client/auth/v1/wallets', { withSign: true, noStore: true })
  const matches = (Array.isArray(response?.items) ? response.items : []).filter((item) => item?.chain === 'evm' &&
    String(item.wallet_address || '').toLowerCase() === wallet.toLowerCase() && item.verified_at && item.verification_method)
  if (matches.length !== 1) throw new ApiError('V3 wallet binding unavailable', 'IDO_V3_WALLET_CHANGED', 403)
  return true
}

/** M3 single-wallet proof; never cache or substitute a Warmup signup. */
export async function getIdoV3Proof(projectID, wallet) {
  if (!/^[1-9][0-9]*$/.test(String(projectID)) || !/^0x[0-9a-fA-F]{40}$/.test(String(wallet))) {
    throw new ApiError('IDO V3 proof identity unavailable', 'IDO_V3_PROOF_UNAVAILABLE', 400)
  }
  return request('GET', `/client/project/v1/ido-v3/${projectID}/proof`, {
    query: { wallet }, withSign: true, noStore: true,
  })
}

/**
 * Authoritative Fair Launch wallet position/action gate.
 *
 * The authenticated wallet is sent explicitly until Backend derives it from the
 * authenticated session. Backend still owns authorization and must reject any
 * query/session mismatch. The normal `{status:true,data}` API envelope is
 * consumed by `request`; this function validates the typed data contract.
	*/
export async function getFairLaunchPosition(projectID, walletAddress) {
	  const authority = exactLocalFairLaunchApiAuthority()
	  const data = authority ? await localFairLaunchRequest(`/client/project/v1/fair-launch/${projectID}/position`, { wallet: walletAddress }) : await request('GET', `/client/project/v1/fair-launch/${projectID}/position`, {
    query: { wallet: walletAddress },
    withSign: true,
  })
  return normalizeFairLaunchPositionDto(data, walletAddress)
}

/** Fair Launch portfolio/history; unavailable or identity-mismatched data fails closed. */
export async function getFairLaunchPortfolio(walletAddress) {
	  const authority = exactLocalFairLaunchApiAuthority()
	  const data = authority ? await localFairLaunchRequest('/client/project/v1/fair-launch/portfolio', { wallet: walletAddress }) : await request('GET', '/client/project/v1/fair-launch/portfolio', {
    query: { wallet: walletAddress },
    withSign: true,
  })
  return normalizeFairLaunchPortfolioDto(data, walletAddress)
}

/**
 * Sprint D/E: 項目公開 Updates 列表（不需要簽名）
 * GET /project/v1/updates?id=&page=&size=
 */
export async function getProjectUpdates(projectID, page = 1, size = 20) {
	if (exactLocalFairLaunchApiAuthority()) {
		return await localFairLaunchRequest('/client/project/v1/updates', { id: projectID, page, size })
	}
  return await request('GET', '/client/project/v1/updates', {
    query: { id: projectID, page, size },
    withSign: false,
  })
}

/**
 * Sprint D/E: 當前用戶對該項目的 subscribe/watch/whitelist 狀態
 * 未登錄會返 3 個 false（後端 uid=0 直接返 default）
 */
export async function getProjectUserStatus(projectID) {
	if (exactLocalFairLaunchApiAuthority()) {
		return await localFairLaunchRequest('/client/project/v1/userStatus', { id: projectID })
	}
  return await request('GET', '/client/project/v1/userStatus', {
    query: { id: projectID },
    withSign: true,
  })
}

export async function subscribeProject(projectID) {
  return await request('POST', '/client/project/v1/subscribe', {
    body: { project_id: projectID },
    withSign: true,
  })
}

export async function unsubscribeProject(projectID) {
  return await request('DELETE', '/client/project/v1/subscribe', {
    query: { id: projectID },
    withSign: true,
  })
}

export async function watchProject(projectID) {
  return await request('POST', '/client/project/v1/watch', {
    body: { project_id: projectID },
    withSign: true,
  })
}

export async function unwatchProject(projectID) {
  return await request('DELETE', '/client/project/v1/watch', {
    query: { id: projectID },
    withSign: true,
  })
}

export async function joinProjectWhitelist(projectID, walletAddress, note = '') {
  return await request('POST', '/client/project/v1/whitelist', {
    body: { project_id: projectID, wallet_address: walletAddress, note },
    withSign: true,
  })
}

/**
 * 獲取待領取列表 — 需要簽名（當前後端未加 sign 中間件，但需要 UID）
 * GET /project/v1/pendingClaimList
 */
export async function getPendingClaimList(cursor = 0) {
  return await request('GET', '/client/project/v1/pendingClaimList', {
    query: { cursor },
    withSign: true
  })
}

/**
 * 獲取認購記錄 — 需要簽名
 * GET /project/v1/purchaseRecord
 */
export async function getPurchaseRecord(cursor = 0) {
  return await request('GET', '/client/project/v1/purchaseRecord', {
    query: { cursor },
    withSign: true
  })
}

/**
 * 獲取領取記錄 — 需要簽名
 * GET /project/v1/claimRecord
 */
export async function getClaimRecord(cursor = 0) {
  return await request('GET', '/client/project/v1/claimRecord', {
    query: { cursor },
    withSign: true
  })
}

/**
 * 獲取我的項目 — 需要簽名
 * GET /project/v1/myProject
 */
export async function getMyProject() {
  return await request('GET', '/client/project/v1/myProject', {
    withSign: true
  })
}

/**
 * 獲取項目投資記錄（項目方視角）— 需要簽名 + 必須是項目 owner
 * GET /project/v1/:projectID/investors
 */
export async function getProjectInvestors(projectId, page = 1, pageSize = 10) {
  return await request('GET', `/client/project/v1/${projectId}/investors`, {
    query: { page, page_size: pageSize },
    withSign: true
  })
}

/**
 * 獲取上傳簽名 URL — 需要簽名
 * POST /cos/v1/signedUploadUrl
 */
export async function getSignedUploadUrl(type, ext) {
  return await request('POST', '/client/cos/v1/signedUploadUrl', {
    body: { type, ext },
    withSign: true
  })
}

/**
 * 獲取退款記錄 — 需要簽名
 * GET /project/v1/refundRecord
 */
export async function getRefundRecord(cursor = 0) {
  return await request('GET', '/client/project/v1/refundRecord', {
    query: { cursor },
    withSign: true
  })
}

/**
 * 獲取通知紅點狀態 — 需要簽名
 * GET /client/notice/v1/redPoint
 */
export async function getRedPointStatus() {
  return await request('GET', '/client/notice/v1/redPoint', {
    withSign: true
  })
}

/**
 * 獲取通知列表 — 需要簽名（cursor 分頁）
 * GET /client/notice/v1/list
 */
export async function getNotificationList(cursor = 0) {
  return await request('GET', '/client/notice/v1/list', {
    query: { cursor },
    withSign: true
  })
}

/**
 * 獲取平臺配置 — 需要簽名
 * GET /platform/v1/settings
 * 返回: { ChainEnvironment, ChainID, ChainUrl, ContractAddress, JccAddress, PlatformFee }
 */
export async function getPlatformSettings() {
  return await request('GET', '/client/platform/v1/settings', {
    withSign: true
  })
}

/**
 * 獲取 X OAuth 授權連結 — 需要簽名
 * POST /login/v1/xLoginUrl
 */
export async function getXLoginUrl() {
  return await request('POST', '/client/login/v1/xLoginUrl', {
    withSign: true
  })
}

/**
 * 綁定 X OAuth 回調 state/code — 需要簽名
 * POST /login/v1/xLogin
 */
export async function bindXLogin(state, code) {
  return await request('POST', '/client/login/v1/xLogin', {
    body: { state, code },
    withSign: true
  })
}

/**
 * 獲取任務中心聚合數據 — 需要簽名
 * GET /taskhall/v1/info
 */
export async function getTaskHallInfo() {
  return await request('GET', '/client/taskhall/v1/info', {
    withSign: true
  })
}

/**
 * 获取当前用户的积分获取记录 — 需要签名
 * GET /taskhall/v1/pointFlows
 */
export async function getPointFlowList({ page = 1, pageSize = 8 } = {}) {
  return await request('GET', '/client/taskhall/v1/pointFlows', {
    query: { page, page_size: pageSize },
    withSign: true
  })
}

/**
 * 獲取每日簽到 typedData — 需要簽名
 * GET /taskhall/v1/checkInNonce
 */
export async function getCheckInNonce() {
  return await request('GET', '/client/taskhall/v1/checkInNonce', {
    withSign: true
  })
}

/**
 * V2 self-paid check-in：用戶自己調合約付 gas，前端把 txHash 傳給後端結算積分。
 * POST /taskhall/v1/checkIn { txHash }
 */
export async function submitCheckIn(txHash) {
  return await request('POST', '/client/taskhall/v1/checkIn', {
    body: { txHash },
    withSign: true
  })
}

/** Recover today's confirmed on-chain check-in after an interrupted settlement request. */
export async function reconcileCheckIn() {
  return await request('POST', '/client/taskhall/v1/checkIn/reconcile', {
    withSign: true
  })
}

/**
 * 提交 X 任務完成 — 需要簽名。
 * 預設由 Backend 使用唯讀 X API 驗證；若 Admin 明確啟用不驗證策略，
 * Backend 會跳過外部 X API，但仍以任務輪次與資料庫唯一約束完成計分。
 * POST /taskhall/v1/completeTask
 */
export async function completeTwitterTask(taskKey = '', replyText = '', taskId = 0, taskVersion = 0) {
  return await request('POST', '/client/taskhall/v1/completeTask', {
    body: {
      platform: 'twitter',
      task_id: taskId,
      task_version: taskVersion,
      task_key: taskKey,
      reply_text: replyText
    },
    withSign: true
  })
}

/**
 * 完成 Telegram 任务（加群 / 打卡）— 需要簽名
 * 与 completeTwitterTask 共用 backend /completeTask endpoint，platform 区分
 */
export async function completeTelegramTask(taskKey = '') {
  return await request('POST', '/client/taskhall/v1/completeTask', {
    body: { platform: 'telegram', task_key: taskKey },
    withSign: true
  })
}

/**
 * 主动检查用户是否已加入 TG 群 —— 用于 join 任务点击后 poll 直到检测到
 * 返回：{ verified, already_done, not_bound, awarded }
 */
export async function checkTgJoin() {
  return await request('POST', '/client/taskhall/v1/checkTgJoin', {
    body: {},
    withSign: true
  })
}

/**
 * 查詢 X 綁定狀態 —— 需要簽名
 * GET /taskhall/v1/xBinding
 */
export async function getXBinding() {
  return await request('GET', '/client/taskhall/v1/xBinding', {
    withSign: true
  })
}

/**
 * 提交項目申請 (Form 1) — 需要登入簽名，walletAddress 必須與主錢包一致
 * POST /project/v1/new
 *
 * @param {object} data - NewProjectRequest
 *   { name, city, email, wechat, telegram, major, identity, organization,
 *     projectName, projectDesc, projectCategory, currentStep, possessions[],
 *     applyReason, supportWanted[], projectLink, additionalInfo }
 */
export async function newProject(data) {
  return await request('POST', '/client/project/v1/new', {
    body: data,
    withSign: true
  })
}

/** Stablecoin V2 wallet positions and immutable purchase/claim/refund facts. */
export async function getV2Portfolio() {
  return await request('GET', '/client/project/v1/v2Portfolio', {
    withSign: true
  })
}

/** Canonical IDO V3 wallet positions and immutable purchase/claim/refund facts. */
export async function getIdoV3Portfolio(walletAddress) {
  return await request('GET', '/client/project/v1/ido-v3/portfolio', {
    query: { wallet: walletAddress },
    withSign: true,
    noStore: true,
  })
}

/**
 * 提交項目代幣審核資訊 (Form 2) — 需要簽名
 * POST /project/v1/submitProjectReview
 *
 * @param {object} data - SubmitProjectReviewRequest
 *   { projectID, nameCh, nameEn, logo, banner, tagline, tags[],
 *     description, teamInfo, roadmap, partners, walletAddress,
 *     tokenAddress, tokenSymbol, tokenDecimals, tokenPrice,
 *     hardCap, softCap, maxPerUser, startTime, endTime,
 *     vestingTimes[], vestingPercent[] }
 */
export async function submitProjectReview(data) {
  return await request('POST', '/client/project/v1/submitProjectReview', {
    body: data,
    withSign: true
  })
}

/**
 * 拿 TG bot 绑定 deep link（需登录态，要带签名）
 * GET /client/auth/v1/tgBindLink
 *   返回 { url, token, bot_name, expires_at }
 *   url = "https://t.me/<bot>?start=joincare_<uid>_<ts>_<sig>"
 *   bot /start 收到后用同一 secret 验签 → 完成 TG→uid 绑定
 */
export async function getTgBindLink() {
  return await request('GET', '/client/auth/v1/tgBindLink', {
    withSign: true
  })
}

/**
 * 拉取指定语言的所有文案 key→value 映射 — 不需要签名
 * GET /client/copywriting/v1/list?language=en
 *   返回 { "navbar.ecosystem": "Ecosystem ▾", "home.heroEyebrow": "Desci", ... }
 *
 * @param {object} opts - { language: 'en'|'tw'|'zh-cn' }
 */
export async function getCopywritingList({ language = 'en' } = {}) {
  return await request('GET', '/client/copywriting/v1/list', {
    query: { language },
    withSign: false
  })
}

/**
 * 拉取任务列表 — 不需要签名
 * GET /client/taskhall/v1/tasks?platform=twitter
 *   返回 { items: [{ id, platform, task_key, title, description, url, cta_label, amount_point, sort }, ...] }
 *
 * @param {object} opts - { platform? }
 */
export async function getTaskList({ platform = '' } = {}) {
  return await request('GET', '/client/taskhall/v1/tasks', {
    query: platform ? { platform } : {},
    withSign: false
  })
}

/**
 * 拉取新闻列表（首页 Latest Media） — 不需要签名
 * GET /client/news/v1/list?language=en&featured=1&limit=10
 * 后端字段 (NewsTranslation join):
 *   { id, slug, category, cover_url, source_url, source_name, is_featured, publish_at,
 *     title, summary, author, author_role }
 *
 * @param {object} opts - { language?, featured?, limit? }
 */
export async function getNewsList({ language = 'en', featured = false, limit = 10 } = {}) {
  return await request('GET', '/client/news/v1/list', {
    query: { language, featured: featured ? '1' : '', limit },
    withSign: false
  })
}

/**
 * 当前生效的重要新闻焦点提醒 — 不需要签名
 * GET /client/news/v1/important-alert?language=en
 */
export async function getImportantNewsAlert({ language = 'en' } = {}) {
  return await request('GET', '/client/news/v1/important-alert', {
    query: { language },
    withSign: false
  })
}

/**
 * 新闻详情 — 不需要签名
 * GET /client/news/v1/:id?language=en
 */
export async function getNewsDetail(id, { language = 'en' } = {}) {
  return await request('GET', `/client/news/v1/${id}`, {
    query: { language },
    withSign: false
  })
}

/**
 * 拉取当前生效首页 Banner（含 hero_animation：dandelion / dna / none）
 * GET /client/banner/v1/current?language=en
 *   返回 { title, subtitle, cta_label, cta_link, image_url, hero_animation, language }
 *   无匹配时返 fallback（hero_animation=dandelion，文案为空字符串）
 *
 * @param {object} opts - { language? }
 */
export async function getCurrentBanner({ language = 'en' } = {}) {
  return await request('GET', '/client/banner/v1/current', {
    query: { language },
    withSign: false
  })
}

/**
 * 首页数据看板 — 60s 服务端缓存
 * GET /client/stats/v1/overview
 *   返回 { users, tasks, total_point }
 */
export async function getStatsOverview() {
  return await request('GET', '/client/stats/v1/overview', {
    withSign: false
  })
}

/**
 * 记录 C 端 SPA 路由访问量（PV / UV），不上传查询参数或用户身份。
 * POST /client/stats/v1/visit
 */
const ANALYTICS_VISITOR_STORAGE_KEY = 'jc_analytics_visitor_id'
const ANALYTICS_VISITOR_ID_PATTERN = /^[A-Za-z0-9_-]{16,64}$/
let analyticsVisitorId = ''

function getAnonymousVisitorId() {
  if (analyticsVisitorId) return analyticsVisitorId

  try {
    const stored = localStorage.getItem(ANALYTICS_VISITOR_STORAGE_KEY) || ''
    if (ANALYTICS_VISITOR_ID_PATTERN.test(stored)) {
      analyticsVisitorId = stored
      return analyticsVisitorId
    }
  } catch (_) {}

  analyticsVisitorId = globalThis.crypto?.randomUUID?.()
    || `anon_${Date.now().toString(36)}_${Math.random().toString(36).slice(2)}`
  try {
    localStorage.setItem(ANALYTICS_VISITOR_STORAGE_KEY, analyticsVisitorId)
  } catch (_) {}
  return analyticsVisitorId
}

export async function recordPageVisit(path) {
  return await request('POST', '/client/stats/v1/visit', {
    body: { path, visitor_id: getAnonymousVisitorId() },
    withSign: false
  })
}

/**
 * 当前生效公告（popup + banner 各最多 1 条）
 * GET /client/announcement/v1/current?language=en
 *   返回 { popup: {id,title,content,...} | null, banner: {...} | null }
 */
export async function getCurrentAnnouncement({ language = 'en' } = {}) {
  return await request('GET', '/client/announcement/v1/current', {
    query: { language },
    withSign: false
  })
}
