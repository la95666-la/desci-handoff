/**
 * EIP-6963: 多錢包標準
 * 允許多個錢包共存且不互相干擾
 * https://eips.ethereum.org/EIPS/eip-6963
 */

/**
 * 發現所有可用錢包（EIP-6963）
 */
export async function discoverEIP6963Providers() {
    const providers = new Map()

    if (typeof window === 'undefined') {
        return providers
    }

    const handleAnnouncement = (event) => {
        if (event.detail?.info && event.detail?.provider) {
            const { info, provider } = event.detail
            console.log('[EIP-6963] Discovered wallet:', info.name)
            providers.set(info.uuid, { info, provider })
        }
    }

    window.addEventListener('eip6963:announceProvider', handleAnnouncement)
    window.dispatchEvent(new Event('eip6963:requestProvider'))

    // 等待錢包響應
    await new Promise((resolve) => setTimeout(resolve, 100))

    window.removeEventListener('eip6963:announceProvider', handleAnnouncement)

    return providers
}

/**
 * 通過名稱獲取指定錢包
 */
export async function getWalletByName(name) {
    const providers = await discoverEIP6963Providers()

    for (const [_, detail] of providers) {
        if (detail.info.name.toLowerCase().includes(name.toLowerCase())) {
            console.log(`[EIP-6963] Found wallet: ${detail.info.name}`)
            return detail
        }
    }

    console.log(`[EIP-6963] Wallet not found: ${name}`)
    return null
}

/**
 * 獲取 MetaMask provider
 */
export async function getMetaMaskProvider() {
    const detail = await getWalletByName('MetaMask')
    if (detail) {
        return detail.provider
    }

    // 回退到 window.ethereum
    if (typeof window !== 'undefined' && window.ethereum?.isMetaMask === true) {
        console.log('[EIP-6963] Using fallback window.ethereum (MetaMask)')
        return window.ethereum
    }

    return null
}

/**
 * 獲取 OKX Wallet provider
 */
export async function getOKXWalletProvider() {
    const detail = await getWalletByName('OKX')
    if (detail) {
        return detail.provider
    }

    // 回退到 window.okxwallet
    if (typeof window !== 'undefined' && window.okxwallet) {
        console.log('[EIP-6963] Using fallback window.okxwallet')
        return window.okxwallet
    }

    return null
}
