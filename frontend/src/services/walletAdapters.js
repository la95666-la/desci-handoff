/**
 * 錢包適配器模塊
 * 
 * 參考 xtask_frontend 的架構，統一 MetaMask 和 OKX 錢包的接口。
 * 每個適配器緩存原始 provider 引用，并創建隔離的 provider 對象，
 * 防止兩個錢包互相覆蓋 window.ethereum。
 */

import { BrowserProvider } from 'ethers'

// ============ 錢包類型枚舉 ============
export const WalletType = {
    METAMASK: 'metamask',
    OKX: 'okx'
}

// ============ MetaMask 適配器 ============
export class MetaMaskAdapter {
    constructor() {
        this.ethereum = typeof window !== 'undefined' ? window.ethereum : null
        this.eip6963Provider = null
        this.cachedMetaMaskProvider = null
        this.cachedRequestMethod = null
        this.isolatedProvider = null

        // 保存對真正的 MetaMask provider 的引用
        if (this.ethereum?.isMetaMask === true) {
            this.cachedMetaMaskProvider = this.ethereum

            this.cachedRequestMethod = async (...args) => {
                const method = args[0]?.method
                console.log('[MetaMaskAdapter] 🦊 REQUEST:', method)
                const provider = this._getProviderForRequest()
                if (!provider || !provider.request) {
                    throw new Error('MetaMask request method not available')
                }
                return provider.request(...args)
            }

            this._createIsolatedProvider()
        }
    }

    setEIP6963Provider(provider) {
        if (provider && typeof provider.request === 'function') {
            this.eip6963Provider = provider
            console.log('[MetaMaskAdapter] Set EIP-6963 provider')
            this._createIsolatedProvider()
        }
    }

    _getProviderForRequest() {
        // 優先 EIP-6963
        if (this.eip6963Provider) return this.eip6963Provider

        // 檢查當前 window.ethereum 是否仍為 MetaMask
        const currentEthereum = typeof window !== 'undefined' ? window.ethereum : null
        if (currentEthereum?.isMetaMask === true && !currentEthereum?.isOKXWallet && typeof currentEthereum.request === 'function') {
            return currentEthereum
        }

        return this.cachedMetaMaskProvider
    }

    _createIsolatedProvider() {
        if (!this.cachedMetaMaskProvider || !this.cachedRequestMethod) return

        this.isolatedProvider = {
            isMetaMask: true,
            request: this.cachedRequestMethod,
            on: (...args) => {
                const provider = this._getProviderForRequest()
                if (provider?.on) provider.on(...args)
            },
            removeListener: (...args) => {
                const provider = this._getProviderForRequest()
                if (provider?.removeListener) provider.removeListener(...args)
            },
            removeAllListeners: (...args) => {
                const provider = this._getProviderForRequest()
                if (provider?.removeAllListeners) provider.removeAllListeners(...args)
            }
        }
    }

    isInstalled() {
        if (this.cachedMetaMaskProvider?.isMetaMask === true) return true
        const ethereum = typeof window !== 'undefined' ? window.ethereum : null
        const isMetaMask = ethereum?.isMetaMask === true
        if (isMetaMask && !this.cachedMetaMaskProvider) {
            this.cachedMetaMaskProvider = ethereum
        }
        return isMetaMask
    }

    _getRealProvider() {
        if (this.isolatedProvider) return this.isolatedProvider

        let provider = null
        if (this.cachedMetaMaskProvider?.isMetaMask === true) {
            provider = this.cachedMetaMaskProvider
        } else {
            const ethereum = typeof window !== 'undefined' ? window.ethereum : null
            if (ethereum?.isMetaMask === true) {
                provider = ethereum
                this.cachedMetaMaskProvider = ethereum
            }
        }

        if (!provider) return null

        if (this.cachedRequestMethod) {
            this._createIsolatedProvider()
            return this.isolatedProvider
        }

        if (!this.cachedRequestMethod && provider.request) {
            this.cachedRequestMethod = provider.request.bind(provider)
            this._createIsolatedProvider()
            return this.isolatedProvider
        }

        return provider
    }

    getName() { return 'MetaMask' }
    getIcon() { return '🦊' }
    getDownloadUrl() { return 'https://metamask.io/download/' }

    getEIP1193Provider() {
        return this._getRealProvider()
    }

    async connect() {
        if (!this.isInstalled()) {
            throw new Error('MetaMask is not installed. Please install MetaMask extension.')
        }

        const provider = this._getRealProvider()
        if (!provider || !provider.request) {
            throw new Error('MetaMask provider not available. Please refresh the page.')
        }

        try {
            const accounts = await provider.request({ method: 'eth_requestAccounts' })
            if (!accounts || accounts.length === 0) {
                throw new Error('No accounts found. Please unlock MetaMask.')
            }
            return accounts[0]
        } catch (error) {
            if (error.code === 4001) {
                throw new Error('User rejected the connection request.')
            }
            if (error.code === -32002) {
                throw new Error('A MetaMask request is already pending. Please check your MetaMask extension.')
            }
            throw error
        }
    }

    async getCurrentAddress() {
        if (!this.isInstalled()) return null
        try {
            const provider = this._getRealProvider()
            if (!provider) return null
            const accounts = await provider.request({ method: 'eth_accounts' })
            return accounts?.[0] || null
        } catch {
            return null
        }
    }

    async switchNetwork(chainId) {
        if (!this.isInstalled()) throw new Error('MetaMask is not installed.')
        const provider = this._getRealProvider()
        if (!provider) throw new Error('MetaMask provider not available')
        try {
            await provider.request({
                method: 'wallet_switchEthereumChain',
                params: [{ chainId }]
            })
        } catch (error) {
            if (error.code === 4902) {
                throw new Error('Please add this network to MetaMask first.')
            }
            throw error
        }
    }

    async getChainId() {
        if (!this.isInstalled()) return null
        try {
            const provider = this._getRealProvider()
            if (!provider) return null
            return await provider.request({ method: 'eth_chainId' })
        } catch {
            return null
        }
    }
}

// ============ OKX 適配器 ============
export class OKXAdapter {
    constructor() {
        this.okxwallet = typeof window !== 'undefined' ? window.okxwallet : null
        this.eip6963Provider = null
        this.cachedOKXProvider = null
        this.cachedRequestMethod = null
        this.isolatedProvider = null

        if (this.okxwallet) {
            this.cachedOKXProvider = this.okxwallet

            this.cachedRequestMethod = async (...args) => {
                console.log('[OKXAdapter] 🟪 REQUEST:', args[0]?.method)
                const provider = this._getProviderForRequest()
                if (!provider || !provider.request) {
                    throw new Error('OKX request method not available')
                }
                return provider.request(...args)
            }

            this._createIsolatedProvider()
        }
    }

    setEIP6963Provider(provider) {
        if (provider && typeof provider.request === 'function') {
            this.eip6963Provider = provider
            console.log('[OKXAdapter] Set EIP-6963 provider')
            this._createIsolatedProvider()
        }
    }

    _getProviderForRequest() {
        if (this.eip6963Provider) return this.eip6963Provider
        return this.cachedOKXProvider
    }

    _createIsolatedProvider() {
        if (!this.cachedOKXProvider || !this.cachedRequestMethod) return

        this.isolatedProvider = {
            isOKXWallet: true,
            request: this.cachedRequestMethod,
            on: (...args) => {
                const provider = this._getProviderForRequest()
                if (provider?.on) provider.on(...args)
            },
            removeListener: (...args) => {
                const provider = this._getProviderForRequest()
                if (provider?.removeListener) provider.removeListener(...args)
            },
            removeAllListeners: (...args) => {
                const provider = this._getProviderForRequest()
                if (provider?.removeAllListeners) provider.removeAllListeners(...args)
            }
        }
    }

    isInstalled() {
        if (this.cachedOKXProvider) return true
        const okxwallet = typeof window !== 'undefined' ? window.okxwallet : null
        if (okxwallet && !this.cachedOKXProvider) {
            this.cachedOKXProvider = okxwallet
        }
        return !!okxwallet
    }

    _getRealProvider() {
        if (this.isolatedProvider) return this.isolatedProvider

        let provider = null
        if (this.cachedOKXProvider) {
            provider = this.cachedOKXProvider
        } else {
            const okxwallet = typeof window !== 'undefined' ? window.okxwallet : null
            if (okxwallet) {
                provider = okxwallet
                this.cachedOKXProvider = okxwallet
            }
        }

        if (!provider) return null

        if (this.cachedRequestMethod) {
            this._createIsolatedProvider()
            return this.isolatedProvider
        }

        if (!this.cachedRequestMethod && provider.request) {
            this.cachedRequestMethod = provider.request.bind(provider)
            this._createIsolatedProvider()
            return this.isolatedProvider
        }

        return provider
    }

    getName() { return 'OKX Wallet' }
    getIcon() { return '/okx-wallet-icon.svg' }
    getDownloadUrl() { return 'https://www.okx.com/web3/wallet' }

    getEIP1193Provider() {
        return this._getRealProvider()
    }

    async connect() {
        if (!this.isInstalled()) {
            throw new Error('OKX Wallet is not installed. Please install OKX Wallet extension.')
        }

        const provider = this._getRealProvider()
        if (!provider || !provider.request) {
            throw new Error('OKX Wallet provider not available. Please refresh the page.')
        }

        try {
            const accounts = await provider.request({ method: 'eth_requestAccounts' })
            if (!accounts || accounts.length === 0) {
                throw new Error('No accounts found. Please unlock OKX Wallet.')
            }
            return accounts[0]
        } catch (error) {
            if (error.code === 4001) {
                throw new Error('User rejected the connection request.')
            }
            if (error.code === -32002) {
                throw new Error('An OKX Wallet request is already pending. Please check your OKX Wallet extension.')
            }
            throw error
        }
    }

    async getCurrentAddress() {
        if (!this.isInstalled()) return null
        try {
            const provider = this._getRealProvider()
            if (!provider) return null
            const accounts = await provider.request({ method: 'eth_accounts' })
            return accounts?.[0] || null
        } catch {
            return null
        }
    }

    async switchNetwork(chainId) {
        if (!this.isInstalled()) throw new Error('OKX Wallet is not installed.')
        const provider = this._getRealProvider()
        if (!provider) throw new Error('OKX Wallet provider not available')
        try {
            await provider.request({
                method: 'wallet_switchEthereumChain',
                params: [{ chainId }]
            })
        } catch (error) {
            if (error.code === 4902) {
                throw new Error('Please add this network to OKX Wallet first.')
            }
            throw error
        }
    }

    async getChainId() {
        if (!this.isInstalled()) return null
        try {
            const provider = this._getRealProvider()
            if (!provider) return null
            return await provider.request({ method: 'eth_chainId' })
        } catch {
            return null
        }
    }
}
