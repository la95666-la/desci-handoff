/**
 * 環境配置
 * 支援：local（本地開發）、test（測試）、prod（線上）
 *
 * 參照 xtask_frontend 的環境管理模式：
 * - 通過 VITE_ENVIRONMENT 環境變量選擇環境
 * - 集中管理各環境的 API 地址
 */

// 优先级：VITE_API_BASE_URL / VITE_WS_BASE_URL (build 时注入) > environments 默认值
// 默认全部走同源（空字符串），前置的 nginx / Caddy 会把 /client/* 反代到 backend:13004
// 跨域部署（前端与 API 不同域）才需要显式设 VITE_API_BASE_URL
const buildEnv = import.meta.env || {}
const injectedApi = buildEnv.VITE_API_BASE_URL ?? ''
const injectedWs = buildEnv.VITE_WS_BASE_URL ?? ''

const environments = {
    local: {
        env: 'local',
        apiBaseUrl: injectedApi,   // 空字符串 → 走同源
        // 本地 compose 只啟動 core API，未啟動獨立 notice WebSocket 服務。
        // 未顯式配置時靜默降級到 redPoint HTTP API，避免把 HTTP 端口當成 WS 重連。
        wsBaseUrl: injectedWs,
        name: '本地環境',
        description: 'Local development environment'
    },
    test: {
        env: 'test',
        apiBaseUrl: injectedApi,
        wsBaseUrl: injectedWs,
        name: '測試環境',
        description: 'Testing environment'
    },
    prod: {
        env: 'prod',
        apiBaseUrl: injectedApi,
        wsBaseUrl: injectedWs,
        name: '線上環境',
        description: 'Production environment'
    }
}

/**
 * 獲取當前環境配置
 * @returns {object} EnvironmentConfig
 */
export function getEnvironmentConfig() {
    const env = buildEnv.VITE_ENVIRONMENT || 'local'
    return environments[env] || environments.local
}

/**
 * 獲取所有可用的環境
 * @returns {object[]} EnvironmentConfig[]
 */
export function getAllEnvironments() {
    return Object.values(environments)
}

/**
 * 獲取當前環境名稱
 * @returns {string} 'local' | 'test' | 'prod'
 */
export function getCurrentEnvironment() {
    return buildEnv.VITE_ENVIRONMENT || 'local'
}

export default environments
