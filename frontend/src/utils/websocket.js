/**
 * WebSocket 管理工具
 * 處理連接、認證、心跳和自動重連
 * 
 * 參照 xtask_frontend 的 WebSocket 實現，適配 JoinCare Vue 3 項目
 */

/**
 * @typedef {Object} WebSocketMessage
 * @property {number} time
 * @property {string} event
 * @property {*} data
 * @property {number} uid
 * @property {string} [token]
 */

/**
 * @typedef {Object} WebSocketConfig
 * @property {string} url
 * @property {number} uid
 * @property {string} token
 * @property {Function} [onReadPoint]
 * @property {Function} [onError]
 * @property {Function} [onClose]
 */

export class WebSocketManager {
    constructor(config) {
        this.ws = null
        this.config = config
        this.heartbeatInterval = null
        this.reconnectTimer = null
        this.isManualClose = false
        this.reconnectAttempts = 0
        this.maxReconnectAttempts = 10
        this.reconnectDelay = 5000 // 5 seconds
        this.eventHandlers = new Map()
        this.connectingPromise = null
    }

    /**
     * 更新配置回調（用於全局實例）
     */
    updateConfig(config) {
        this.config = { ...this.config, ...config }
        console.log('[WebSocket] Config updated')
    }

    /**
     * 連接到 WebSocket
     */
    connect() {
        // 如果已經在連接，返回現有的 Promise
        if (this.connectingPromise) {
            console.log('[WebSocket] Connection already in progress, returning existing promise')
            return this.connectingPromise
        }

        // 如果已經連接，直接返回
        if (this.isConnected()) {
            console.log('[WebSocket] Already connected')
            return Promise.resolve()
        }

        // 創建新的連接 Promise
        this.connectingPromise = new Promise((resolve, reject) => {
            try {
                this.ws = new WebSocket(this.config.url)

                this.ws.onopen = () => {
                    console.log('[WebSocket] Connected')
                    this.isManualClose = false
                    this.reconnectAttempts = 0

                    // 發送登入資訊
                    this.sendLoginMessage()

                    // 設置心跳
                    this.setupHeartbeat()

                    // 給服務器 3 秒時間來處理登入和認證
                    setTimeout(() => {
                        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
                            console.log('[WebSocket] Authentication successful, connection stable')
                            this.connectingPromise = null
                            resolve()
                        } else {
                            console.warn('[WebSocket] Connection closed during authentication phase')
                            this.connectingPromise = null
                            reject(new Error('WebSocket authentication failed'))
                        }
                    }, 3000)
                }

                this.ws.onmessage = (event) => {
                    try {
                        const message = JSON.parse(event.data)
                        console.log('[WebSocket] Received message:', message.event, message.data)

                        // 處理心跳響應
                        if (message.event === 'heartbeat') {
                            return
                        }

                        // 處理通知紅點
                        if (message.event === 'readPoint') {
                            this.config.onReadPoint?.(message.data)
                            this.emit('readPoint', message.data)
                            return
                        }

                        // 觸發對應事件的所有處理函數
                        this.emit(message.event, message.data)
                    } catch (error) {
                        console.error('[WebSocket] Error parsing message:', error)
                    }
                }

                this.ws.onerror = (event) => {
                    // WebSocket 属于 nice-to-have（红点通知）；连不上时不刷 console.error
                    this.config.onError?.(event)
                }

                this.ws.onclose = (event) => {
                    console.log('[WebSocket] Closed', event.code, event.reason)

                    // 清除連接 Promise 緩存
                    this.connectingPromise = null

                    // 立即停止心跳
                    this.stopHeartbeat()

                    // 清除 WebSocket 引用
                    this.ws = null
                    console.log('[WebSocket] Cleared WebSocket reference')

                    if (!this.isManualClose) {
                        if (event.code === 1000) {
                            console.log('[WebSocket] Normal closure, not reconnecting')
                            reject(new Error('WebSocket closed normally'))
                        } else if (event.code === 1006 && this.reconnectAttempts >= 3) {
                            // 服务端不可达，非致命 —— 静默放弃重连
                            reject(new Error('WebSocket disconnected due to server issues'))
                        } else if (event.code === 4001 || event.code === 4002 || event.code === 4003) {
                            console.error('[WebSocket] Authentication failed (code: ' + event.code + '), not reconnecting')
                            reject(new Error(`Server authentication error: ${event.code}`))
                        } else {
                            console.log('[WebSocket] Attempting automatic reconnect')
                            this.attemptReconnect()
                        }
                    }

                    this.config.onClose?.(event)
                }
            } catch (error) {
                this.connectingPromise = null
                console.error('[WebSocket] Connection error:', error)
                reject(error)
            }
        })

        return this.connectingPromise
    }

    /**
     * 發送登入消息
     */
    sendLoginMessage() {
        const message = {
            time: Date.now(),
            event: 'login',
            data: {},
            uid: this.config.uid,
            token: this.config.token
        }
        this.send(message)
    }

    /**
     * 設置心跳
     */
    setupHeartbeat() {
        this.stopHeartbeat()

        console.log('[WebSocket] Starting heartbeat (30s interval)')
        this.heartbeatInterval = setInterval(() => {
            if (this.ws && this.ws.readyState === WebSocket.OPEN) {
                const message = {
                    time: Date.now(),
                    event: 'heartbeat',
                    data: {},
                    uid: this.config.uid,
                    token: this.config.token
                }
                this.send(message)
                console.log('[WebSocket] Heartbeat sent')
            }
        }, 30000)
    }

    /**
     * 停止心跳
     */
    stopHeartbeat() {
        if (this.heartbeatInterval) {
            clearInterval(this.heartbeatInterval)
            this.heartbeatInterval = null
        }
    }

    /**
     * 嘗試重新連接（指數退避）
     */
    attemptReconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.error('[WebSocket] Max reconnect attempts reached, giving up')
            return
        }

        this.reconnectAttempts++
        const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1)

        console.log(
            `[WebSocket] Attempting reconnect ${this.reconnectAttempts}/${this.maxReconnectAttempts} in ${delay}ms`
        )

        if (this.reconnectTimer) {
            clearTimeout(this.reconnectTimer)
        }

        this.reconnectTimer = setTimeout(() => {
            this.connect().catch(() => {
                if (this.reconnectAttempts < this.maxReconnectAttempts) {
                    this.attemptReconnect()
                }
            })
        }, delay)
    }

    /**
     * 發送消息
     */
    send(message) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(message))
        } else {
            console.warn('[WebSocket] WebSocket is not open, cannot send message')
        }
    }

    /**
     * 監聽事件
     */
    on(event, handler) {
        if (!this.eventHandlers.has(event)) {
            this.eventHandlers.set(event, new Set())
        }
        this.eventHandlers.get(event).add(handler)
    }

    /**
     * 移除事件監聽
     */
    off(event, handler) {
        const handlers = this.eventHandlers.get(event)
        if (handlers) {
            handlers.delete(handler)
        }
    }

    /**
     * 觸發事件
     */
    emit(event, data) {
        const handlers = this.eventHandlers.get(event)
        if (handlers) {
            handlers.forEach((handler) => {
                try {
                    handler(data)
                } catch (error) {
                    console.error(`[WebSocket] Error in event handler for '${event}':`, error)
                }
            })
        }
    }

    /**
     * 斷開連接
     */
    disconnect() {
        console.log('[WebSocket] Disconnecting...')
        this.isManualClose = true
        this.connectingPromise = null

        this.stopHeartbeat()
        console.log('[WebSocket] Heartbeat stopped')

        if (this.reconnectTimer) {
            clearTimeout(this.reconnectTimer)
            this.reconnectTimer = null
            console.log('[WebSocket] Reconnect timer cleared')
        }

        if (this.ws) {
            this.ws.close()
            this.ws = null
            console.log('[WebSocket] WebSocket connection closed')
        }

        this.eventHandlers.clear()
        console.log('[WebSocket] All event listeners cleared')

        this.reconnectAttempts = 0

        console.log('[WebSocket] Disconnection complete')
    }

    /**
     * 檢查連接狀態
     */
    isConnected() {
        return this.ws !== null && this.ws.readyState === WebSocket.OPEN
    }
}

// 全局 WebSocket 實例
let globalWebSocketManager = null

/**
 * 獲取或創建全局 WebSocket 實例
 */
export function getWebSocketManager(config) {
    if (!globalWebSocketManager) {
        console.log('[WebSocket] Creating new global WebSocket manager instance')
        globalWebSocketManager = new WebSocketManager(config)
    } else {
        console.log('[WebSocket] Reusing existing global WebSocket manager instance')
    }
    return globalWebSocketManager
}

/**
 * 清除全局 WebSocket 實例
 */
export function clearWebSocketManager() {
    if (globalWebSocketManager) {
        globalWebSocketManager.disconnect()
        globalWebSocketManager = null
    }
}
