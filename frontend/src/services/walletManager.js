/**
 * 錢包管理器 - 統一管理所有錢包適配器
 * 
 * 參考 xtask_frontend 的 walletManager 架構，
 * 為 JoinCare 項目提供多錢包支援（MetaMask + OKX）。
 */

import { MetaMaskAdapter, OKXAdapter, WalletType } from './walletAdapters.js'
import { getMetaMaskProvider, getOKXWalletProvider } from './eip6963.js'

export { WalletType } from './walletAdapters.js'

class WalletManager {
    constructor() {
        this.adapters = new Map()
        this.currentAdapter = null
        this.currentWalletType = null
        this.eip6963Initialized = false
        this.eip6963Providers = new Map()

        // 初始化所有可用的錢包適配器
        this.adapters.set(WalletType.METAMASK, new MetaMaskAdapter())
        this.adapters.set(WalletType.OKX, new OKXAdapter())

        // 初始化 EIP-6963 錢包發現
        if (typeof window !== 'undefined') {
            this._initEIP6963()
        }
    }

    async _initEIP6963() {
        try {
            const metaMaskProvider = await getMetaMaskProvider()
            const okxProvider = await getOKXWalletProvider()

            if (metaMaskProvider) {
                this.eip6963Providers.set('metamask', metaMaskProvider)
                const adapter = this.adapters.get(WalletType.METAMASK)
                if (adapter?.setEIP6963Provider) {
                    adapter.setEIP6963Provider(metaMaskProvider)
                }
                console.log('[WalletManager] EIP-6963: Found MetaMask')
            }

            if (okxProvider) {
                this.eip6963Providers.set('okx', okxProvider)
                const adapter = this.adapters.get(WalletType.OKX)
                if (adapter?.setEIP6963Provider) {
                    adapter.setEIP6963Provider(okxProvider)
                }
                console.log('[WalletManager] EIP-6963: Found OKX Wallet')
            }

            this.eip6963Initialized = true
        } catch (error) {
            console.error('[WalletManager] EIP-6963 initialization error:', error)
        }
    }

    /**
     * 獲取特定類型的適配器
     */
    getAdapter(type) {
        return this.adapters.get(type) || null
    }

    /**
     * 獲取所有可用錢包（包括未安裝的），用於錢包選擇彈窗
     */
    getAllAvailableWallets() {
        const wallets = []
        this.adapters.forEach((adapter, type) => {
            wallets.push({
                type,
                name: adapter.getName(),
                icon: adapter.getIcon(),
                isInstalled: adapter.isInstalled(),
                downloadUrl: adapter.getDownloadUrl()
            })
        })
        return wallets
    }

    /**
     * 連接到指定的錢包
     */
    async connect(type) {
        const adapter = this.getAdapter(type)
        if (!adapter) {
            throw new Error(`Wallet adapter for ${type} not found`)
        }
        if (!adapter.isInstalled()) {
            const error = new Error(`${adapter.getName()} is not installed`)
            error.type = type
            error.downloadUrl = adapter.getDownloadUrl()
            throw error
        }

        const address = await adapter.connect()
        this.currentAdapter = adapter
        this.currentWalletType = type
        return address
    }

    /**
     * 獲取 EIP-1193 提供商對象（用於 ethers.js）
     * 根據當前選定的錢包類型返回對應的 provider
     */
    getProvider(walletType) {
        if (typeof window === 'undefined') return null

        const type = walletType || this.currentWalletType

        if (type === WalletType.METAMASK) {
            if (this.eip6963Initialized && this.eip6963Providers.has('metamask')) {
                return this.eip6963Providers.get('metamask')
            }
            const adapter = this.adapters.get(WalletType.METAMASK)
            if (adapter?.isInstalled()) return adapter.getEIP1193Provider()
            return null
        }

        if (type === WalletType.OKX) {
            if (this.eip6963Initialized && this.eip6963Providers.has('okx')) {
                return this.eip6963Providers.get('okx')
            }
            const adapter = this.adapters.get(WalletType.OKX)
            if (adapter?.isInstalled()) return adapter.getEIP1193Provider()
            return null
        }

        // 如果有當前適配器，使用它
        if (this.currentAdapter?.isInstalled()) {
            return this.currentAdapter.getEIP1193Provider()
        }

        // 自動檢測 - 優先 MetaMask
        if (this.eip6963Initialized) {
            if (this.eip6963Providers.has('metamask')) return this.eip6963Providers.get('metamask')
            if (this.eip6963Providers.has('okx')) return this.eip6963Providers.get('okx')
        }

        const metaMaskAdapter = this.adapters.get(WalletType.METAMASK)
        if (metaMaskAdapter?.isInstalled()) return metaMaskAdapter.getEIP1193Provider()

        const okxAdapter = this.adapters.get(WalletType.OKX)
        if (okxAdapter?.isInstalled()) return okxAdapter.getEIP1193Provider()

        return null
    }

    /**
     * 獲取當前錢包類型
     */
    getCurrentWalletType() {
        return this.currentWalletType
    }

    /**
     * 清除當前連接
     */
    disconnect() {
        this.currentAdapter = null
        this.currentWalletType = null
    }
}

// 創建全局單例
export const walletManager = new WalletManager()
