/**
 * Vue 3 Composable: WebSocket 連接管理
 * 
 * 參照 xtask_frontend 的 useWebSocket hook，適配為 Vue 3 composable
 * 
 * @example
 * const { isConnected } = useWebSocket({
 *   enabled: walletStore.isLoggedIn,
 *   onReadPoint: (data) => {
 *     console.log('收到通知紅點:', data)
 *   }
 * })
 */

import { ref, watch, onUnmounted } from 'vue'
import { getEnvironmentConfig } from '../config/environments'
import { getWebSocketManager, clearWebSocketManager } from '../utils/websocket'
import { useWalletStore } from '../stores/wallet'

export function useWebSocket(options = {}) {
    const { onReadPoint, onError, onClose } = options

    const isConnected = ref(false)
    const walletStore = useWalletStore()

    let manager = null

    /**
     * 建立連接
     */
    function connect() {
        const uid = walletStore.uid
        const token = walletStore.token

        if (!uid || !token) {
            return
        }

        const envConfig = getEnvironmentConfig()
        const wsUrl = envConfig.wsBaseUrl

        // build 时 env=local 但访问远端 → 服务端未部署 notice/WS handler，
        // 兜底同源 URL 也会失败反复刷 console。此时直接静默不连接；
        // 红点由 App.vue 的 getRedPointStatus() API polling 兜住。
        try {
            const pageHost = window.location.hostname
            const isPageLocal = pageHost === 'localhost' || pageHost === '127.0.0.1'
            if (!isPageLocal && wsUrl && /localhost|127\.0\.0\.1/.test(wsUrl)) {
                return
            }
        } catch (_) { /* SSR / 无 window，跳过 */ }

        if (!wsUrl) {
            // 未配置 → 直接静默跳过，不再打 warn（避免生产控制台噪音）
            return
        }

        const config = {
            url: wsUrl,
            uid: parseInt(uid),
            token,
            onReadPoint: (data) => {
                console.log('[useWebSocket] onReadPoint callback triggered with data:', data)
                onReadPoint?.(data)
            },
            onError: (error) => onError?.(error),
            onClose: (event) => {
                isConnected.value = false
                onClose?.(event)
            }
        }

        manager = getWebSocketManager(config)

        // 如果是重用已存在的 manager，更新其配置中的回調函數
        manager.updateConfig({
            onReadPoint: config.onReadPoint,
            onError: config.onError,
            onClose: config.onClose
        })

        // 如果還沒有連接，則連接
        if (!manager.isConnected()) {
            manager.connect()
                .then(() => {
                    console.log('[useWebSocket] Connected successfully')
                    isConnected.value = true
                })
                .catch((error) => {
                    console.error('[useWebSocket] Failed to connect:', error)
                    isConnected.value = false
                })
        } else {
            isConnected.value = true
        }
    }

    /**
     * 斷開連接
     */
    function disconnect() {
        clearWebSocketManager()
        manager = null
        isConnected.value = false
    }

    // 監聽登入狀態變化，自動連接/斷開
    watch(
        () => walletStore.isLoggedIn,
        (loggedIn) => {
            if (loggedIn) {
                connect()
            } else {
                disconnect()
            }
        },
        { immediate: true }
    )

    // 組件卸載時不斷開（全局連接，由 App.vue 管理）
    // 如果需要在某個頁面卸載時斷開，可以手動調用 disconnect()

    return {
        isConnected,
        connect,
        disconnect
    }
}
