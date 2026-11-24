import { BrowserProvider, JsonRpcSigner, formatUnits, Contract, hexlify, toUtf8Bytes } from 'ethers'
import { ADDRESSES } from '../config/contracts.js'
import { walletManager, WalletType } from './walletManager.js'
import { normalizeChainId } from './networkPolicy.js'

// BSC Mainnet configuration
const BSC_MAINNET = {
  chainId: '0x38', // 56 in hex
  chainName: 'BNB Smart Chain',
  nativeCurrency: {
    name: 'BNB',
    symbol: 'BNB',
    decimals: 18
  },
  rpcUrls: ['https://bsc-dataseed1.binance.org'],
  blockExplorerUrls: ['https://bscscan.com']
}

// BSC Testnet configuration
const BSC_TESTNET = {
  chainId: '0x61', // 97 in hex
  chainName: 'BNB Smart Chain Testnet',
  nativeCurrency: {
    name: 'tBNB',
    symbol: 'tBNB',
    decimals: 18
  },
  rpcUrls: ['https://data-seed-prebsc-1-s1.binance.org:8545'],
  blockExplorerUrls: ['https://testnet.bscscan.com']
}

// ERC20 ABI (minimal for balance checking)
const ERC20_ABI = [
  'function balanceOf(address owner) view returns (uint256)',
  'function decimals() view returns (uint8)',
  'function symbol() view returns (string)'
]

class Web3Service {
  constructor() {
    this.provider = null
    this.signer = null
    this.chainId = null
    this._currentWalletType = null // 記錄當前使用的錢包類型
    this._externalEIP1193Provider = null // Reown/Wagmi 當前 connector 的標準 provider

    // ethers 會話綁定快照：記錄 provider/signer 是在哪條鏈、哪個賬戶下創建的。
    // ensureSigner() 每次寫前都會對比「當前鏈 / 當前賬戶」，不一致就重建，
    // 絕不復用綁定舊鏈/舊賬戶的陳舊 signer。
    this._signerChainId = null
    this._signerAccount = null
    this._signerProvider = null
  }

  /**
   * 清空緩存的 ethers 會話（provider/signer 與綁定快照）。
   * chainId 保留由調用方決定（handleChainChanged 會先更新 chainId 再清空）。
   */
  _clearSession() {
    this.provider = null
    this.signer = null
    this._signerChainId = null
    this._signerAccount = null
    this._signerProvider = null
  }

  /**
   * 用 EIP-1193 provider 重建 BrowserProvider + signer，並記錄綁定快照。
   * 只有在所有 RPC 都成功後才會覆寫 this.provider/signer，失敗不會留下半初始化狀態。
   * @returns {Promise<{chainId: number, address: string|null}>}
   */
  /**
   * 发送交易前的会话对齐：WalletConnect（OKX 等）对 eth_sendTransaction
   * 的 from 做严格校验，若签名器账户与会话授权账户不一致会静默丢弃请求
   * （不弹窗不报错）。发交易前先显式 requestAccounts 同步一次。
   */
  /**
   * 直接通过 EIP-1193 provider 发送交易（绕过 ethers signer）。
   * 某些 WalletConnect 钱包对 ethers JsonRpcSigner 的 sendTransaction 封装
   * （populate → sign → send 的组合请求）会静默挂起，但对裸
   * eth_sendTransaction 请求正常弹窗。返回 tx hash。
   */
  async sendRawTransaction(to, data, from, valueHex = '0x0', gasHex = null, options = {}) {
    const provider = this._getEIP1193Provider()
    if (!provider) throw new Error('No EIP-1193 provider for raw send')
    const payload = { from, to, data, value: valueHex }
    if (gasHex) payload.gas = gasHex
    if (options.chainId) payload.chainId = options.chainId
    return provider.request({ method: 'eth_sendTransaction', params: [payload] })
  }

  async alignSessionForSend() {
    const provider = this._getEIP1193Provider()
    if (!provider || !this.signer) return
    try {
      const accounts = await Promise.race([
        provider.request({ method: 'eth_requestAccounts' }),
        new Promise((resolve) => setTimeout(() => resolve(null), 6000)),
      ])
      if (Array.isArray(accounts) && accounts[0]) {
        const account = String(accounts[0]).toLowerCase()
        const signerAccount = this.signer.address ? String(this.signer.address).toLowerCase() : null
        if (signerAccount && signerAccount !== account) {
          console.warn('[web3] session account mismatch, rebuilding signer:', { signer: signerAccount, session: account })
          const browserProvider = this.provider || new BrowserProvider(provider)
          this.signer = new JsonRpcSigner(browserProvider, account)
          this._signerAccount = account
        }
      }
    } catch (e) {
      console.warn('[web3] alignSessionForSend failed (continuing):', e?.message || e)
    }
  }

  async createSession(eip1193Provider) {
    const provider = new BrowserProvider(eip1193Provider)
    // 某些钱包（OKX WalletConnect 等）对重复的 eth_requestAccounts 会永久挂起不弹窗；
    // getSigner 加超时兑底，超时后回退到 eth_accounts 结果并注入 JsonRpcSigner。
    const withTimeout = (p, ms, tag) => Promise.race([
      p,
      new Promise((_, rej) => setTimeout(() => rej(new Error(tag + ' timeout after ' + ms + 'ms')), ms)),
    ])
    const accounts = await withTimeout(
      eip1193Provider.request({ method: 'eth_accounts' }).catch(() => []),
      8000,
      'eth_accounts',
    )
    const account = Array.isArray(accounts) && accounts[0] ? String(accounts[0]).toLowerCase() : null
    let signer
    try {
      signer = await withTimeout(provider.getSigner(), 12000, 'getSigner')
    } catch (e) {
      console.warn('[web3] getSigner fallback (session may already be authorized):', e?.message || e)
      if (!account) throw e
      // 回退：直接用已知地址构造 JsonRpcSigner（发交易时会再走钱包签名确认）
      signer = new JsonRpcSigner(provider, account)
    }
    const network = await withTimeout(provider.getNetwork(), 8000, 'getNetwork')
    const chainId = Number(network.chainId)
    this.provider = provider
    this.signer = signer
    this.chainId = chainId
    this._signerChainId = chainId
    this._signerAccount = account || (signer && signer.address ? String(signer.address).toLowerCase() : null)
    this._signerProvider = eip1193Provider
    return { chainId, address: this._signerAccount }
  }

  /**
   * 鏈切換事件處理（EIP-1193 chainChanged）：
   * 1. 解析 hex/decimal chainId 並更新 this.chainId；
   * 2. 清理緩存的 BrowserProvider/signer —— 舊 BrowserProvider 可能有舊鏈緩存，
   *    必須重建才能讓後續寫交易綁定新鏈；
   * 3. 不在事件回調內異步重建。部分錢包會先發 chainChanged、稍後才讓 RPC 返回新鏈；
   *    立即重建可能被舊 RPC 結果覆寫。由下一次 ensureSigner 讀取實時 chain/account 後原子重建。
   * @param {string|number|null} nextChainIdHex
   * @returns {number|null} 解析後的 chainId；解析失敗返回 null（並清空會話）
   */
  handleChainChanged(nextChainIdHex) {
    const parsed = normalizeChainId(nextChainIdHex)
    if (parsed == null) {
      console.warn('[web3] chainChanged with unparsable chainId:', nextChainIdHex)
      this._clearSession()
      return null
    }
    this.chainId = parsed
    this._clearSession()
    return parsed
  }

  /**
   * 獲取當前錢包類型的 EIP-1193 provider
   */
  _getEIP1193Provider() {
    return this._externalEIP1193Provider || walletManager.getProvider(this._currentWalletType)
  }

  setExternalEIP1193Provider(provider) {
    if (provider != null && typeof provider.request !== 'function') {
      throw new Error('Selected wallet connector did not provide an EIP-1193 provider')
    }
    this._externalEIP1193Provider = provider || null
    this._clearSession()
  }

  /**
   * 檢查是否有任何錢包已安裝
   */
  isMetaMaskInstalled() {
    // 保持向後兼容：檢查是否有任何錢包可用
    return typeof window !== 'undefined' && (
      typeof window.ethereum !== 'undefined' ||
      typeof window.okxwallet !== 'undefined'
    )
  }

  /**
   * 檢查指定類型的錢包是否已安裝
   */
  isWalletInstalled(walletType) {
    const adapter = walletManager.getAdapter(walletType)
    return adapter?.isInstalled() ?? false
  }

  /**
   * 使用指定類型的錢包連接
   * @param {string} walletType - WalletType.METAMASK 或 WalletType.OKX
   */
  async connectWalletByType(walletType) {
    try {
      // 通過 walletManager 連接獲取地址
      const address = await walletManager.connect(walletType)
      this._currentWalletType = walletType

      // 獲取對應的 EIP-1193 provider
      const eip1193Provider = this._getEIP1193Provider()
      if (!eip1193Provider) {
        throw new Error('Failed to get wallet provider after connection')
      }

      // 重建 ethers provider + signer（記錄綁定快照，供 ensureSigner 校驗新鮮度）
      await this.createSession(eip1193Provider)

      // Get BNB balance
      const balance = await this.provider.getBalance(address)
      const bnbBalance = formatUnits(balance, 18)

      // Get JCC balance (if contract address is set)
      let jccBalance = '0'
      if (ADDRESSES.JCC && ADDRESSES.JCC !== '0x0000000000000000000000000000000000000000') {
        try {
          const jccContract = new Contract(ADDRESSES.JCC, ERC20_ABI, this.provider)
          const jccBal = await jccContract.balanceOf(address)
          const decimals = await jccContract.decimals()
          jccBalance = formatUnits(jccBal, decimals)
        } catch (error) {
          console.warn('Failed to fetch JCC balance:', error)
        }
      }

      return {
        address,
        chainId: this.chainId,
        balance: bnbBalance,
        jccBalance,
        walletType
      }
    } catch (error) {
      console.error('Error connecting wallet:', error)
      throw error
    }
  }

  /**
   * 舊版連接方法（向後兼容 - 不帶錢包選擇，直接用 window.ethereum）
   * 注意：新流程應使用 connectWalletByType
   */
  async connectWallet() {
    if (!this.isMetaMaskInstalled()) {
      throw new Error('MetaMask is not installed. Please install MetaMask to continue.')
    }

    try {
      // 使用當前錢包類型的 provider，如果沒有則回退到 window.ethereum
      const eip1193Provider = this._getEIP1193Provider() || window.ethereum

      // Force account picker popup
      await eip1193Provider.request({
        method: 'wallet_requestPermissions',
        params: [{ eth_accounts: {} }]
      })
      const accounts = await eip1193Provider.request({
        method: 'eth_accounts'
      })

      if (!accounts || accounts.length === 0) {
        throw new Error('No accounts found')
      }

      // 重建 ethers provider + signer
      await this.createSession(eip1193Provider)

      // Get address
      const address = accounts[0]

      // Get BNB balance
      const balance = await this.provider.getBalance(address)
      const bnbBalance = formatUnits(balance, 18)

      // Get JCC balance (if contract address is set)
      let jccBalance = '0'
      if (ADDRESSES.JCC && ADDRESSES.JCC !== '0x0000000000000000000000000000000000000000') {
        try {
          const jccContract = new Contract(ADDRESSES.JCC, ERC20_ABI, this.provider)
          const jccBal = await jccContract.balanceOf(address)
          const decimals = await jccContract.decimals()
          jccBalance = formatUnits(jccBal, decimals)
        } catch (error) {
          console.warn('Failed to fetch JCC balance:', error)
        }
      }

      return {
        address,
        chainId: this.chainId,
        balance: bnbBalance,
        jccBalance
      }
    } catch (error) {
      console.error('Error connecting wallet:', error)
      throw error
    }
  }

  /**
   * 確保 signer 可用（不彈窗）。每次調用都會從錢包重新讀取當前 chain 和 account：
   *  - 若已有 provider/signer 且仍綁定「當前鏈 + 當前賬戶 + 同一個 EIP-1193 provider」→ 復用；
   *  - 否則靜默重建 BrowserProvider + signer（絕不復用陳舊 signer）。
   * autoLogin 恢復後端登入後，錢包可能還沒初始化 signer，此方法會自動補上。
   * @returns {Promise<boolean>} 是否有可用 signer
   */
  async ensureSigner() {
    const eip1193Provider = this._getEIP1193Provider()
    if (!eip1193Provider) {
      // 回退到 window.ethereum / window.okxwallet
      if (typeof window === 'undefined') return false
      const fallback = window.ethereum || window.okxwallet
      if (!fallback) return false
      return this._ensureSession(fallback)
    }
    return this._ensureSession(eip1193Provider)
  }

  // V3 send checks must never request a new account authorization. Rebuild
  // directly from the accounts already exposed by the selected connector;
  // createSession/getSigner may otherwise call eth_requestAccounts.
  async ensureReadOnlySigner() {
    const provider = this._getEIP1193Provider() ||
      (typeof window !== 'undefined' && (window.ethereum || window.okxwallet))
    if (!provider) { this._clearSession(); return false }
    try {
      const [accounts, chain] = await Promise.all([
        provider.request({ method: 'eth_accounts' }), provider.request({ method: 'eth_chainId' }),
      ])
      const account = Array.isArray(accounts) && /^0x[0-9a-fA-F]{40}$/.test(String(accounts[0]))
        ? String(accounts[0]).toLowerCase() : null
      const chainId = normalizeChainId(chain)
      if (!account || chainId == null) { this._clearSession(); return false }
      if (!this.signer || !this.provider || this._signerProvider !== provider ||
          this._signerChainId !== chainId || this._signerAccount !== account) {
        this.provider = new BrowserProvider(provider)
        this.signer = new JsonRpcSigner(this.provider, account)
        this._signerProvider = provider
        this._signerAccount = account
        this._signerChainId = chainId
        this.chainId = chainId
      }
      return true
    } catch { this._clearSession(); return false }
  }

  /**
   * 校驗並（如需）重建會話的核心邏輯：
   * 同時讀取 eth_accounts + eth_chainId —— 這兩者都是 read-only，不會彈授權窗。
   */
  async _ensureSession(eip1193Provider) {
    try {
      const [accounts, chainIdHex] = await Promise.all([
        eip1193Provider.request({ method: 'eth_accounts' }).catch(() => []),
        eip1193Provider.request({ method: 'eth_chainId' }).catch(() => null),
      ])
      const account = Array.isArray(accounts) && accounts[0] ? String(accounts[0]).toLowerCase() : null
      const currentChainId = normalizeChainId(chainIdHex)

      const sessionFresh =
        !!this.signer && !!this.provider &&
        this._signerProvider === eip1193Provider &&
        this._signerChainId === currentChainId &&
        this._signerAccount === account
      if (!sessionFresh) {
        await this.createSession(eip1193Provider)
      }
      return !!this.signer && !!this._signerAccount
    } catch (e) {
      console.warn('ensureSigner failed:', e)
      this._clearSession()
      return false
    }
  }

  /**
   * 使用當前錢包對後端返回的登入/註冊 message 原文做 personal_sign。
   */
  async personalSign(message, walletAddress) {
    const eip1193Provider = this._getEIP1193Provider()
    const provider = eip1193Provider || window.ethereum || window.okxwallet
    if (!provider) {
      throw new Error('No wallet provider available')
    }

    const account = walletAddress || await this.getCurrentAccount()
    if (!account) {
      throw new Error('No wallet account connected')
    }

    const encodedMessage = typeof message === 'string' && message.startsWith('0x')
      ? message
      : hexlify(toUtf8Bytes(String(message)))
    return await provider.request({
      method: 'personal_sign',
      params: [encodedMessage, account]
    })
  }

  /**
   * 使用 eth_signTypedData_v4 原樣簽名後端返回的 EIP-712 typedData。
   */
  async signTypedDataV4(typedData, walletAddress) {
    const eip1193Provider = this._getEIP1193Provider()
    const provider = eip1193Provider || window.ethereum || window.okxwallet
    if (!provider) {
      throw new Error('No wallet provider available')
    }

    const account = walletAddress || await this.getCurrentAccount()
    if (!account) {
      throw new Error('No wallet account connected')
    }

    return await provider.request({
      method: 'eth_signTypedData_v4',
      params: [account, JSON.stringify(typedData)]
    })
  }

  // Disconnect wallet
  disconnect() {
    this._clearSession()
    this.chainId = null
    this._currentWalletType = null
    this._externalEIP1193Provider = null
    walletManager.disconnect()
  }

  /**
   * 切換到指定 chainId 的網絡（EIP-3326 wallet_switchEthereumChain）。
   * 只對有網絡參數定義的鏈（56 / 97）自動切換；Hardhat(31337) 等本地鏈
   * 不在此列 —— 本地構建的策略（autoSwitch=false）本來就不該自動切鏈，
   * 這裡再留一道硬性防線，避免任何代碼路徑把用戶強行切走。
   * @param {number|string} chainId - 目標鏈 ID（如 56 / 97）
   */
  async switchToChain(chainId) {
    const target = normalizeChainId(chainId)
    const eip1193Provider = this._getEIP1193Provider()
    if (!eip1193Provider && !this.isMetaMaskInstalled()) {
      throw new Error('No wallet is installed')
    }

    const network = target === 97 ? BSC_TESTNET : target === 56 ? BSC_MAINNET : null
    if (!network) {
      const err = new Error('Chain ' + target + ' is not auto-switchable')
      err.code = 'UNSUPPORTED_SWITCH_CHAIN'
      throw err
    }

    const provider = eip1193Provider || window.ethereum
    try {
      await provider.request({
        method: 'wallet_switchEthereumChain',
        params: [{ chainId: network.chainId }]
      })
    } catch (switchError) {
      if (switchError.code === 4902) {
        try {
          await provider.request({
            method: 'wallet_addEthereumChain',
            params: [network]
          })
        } catch (addError) {
          throw addError
        }
      } else {
        throw switchError
      }
    }
  }

  // Get current account
  async getCurrentAccount() {
    const eip1193Provider = this._getEIP1193Provider()
    if (!eip1193Provider && !this.isMetaMaskInstalled()) return null

    try {
      const provider = eip1193Provider || window.ethereum
      const accounts = await provider.request({ method: 'eth_accounts' })
      return accounts[0] || null
    } catch (error) {
      console.error('Error getting current account:', error)
      return null
    }
  }

  // Listen to account changes
  onAccountsChanged(callback) {
    const eip1193Provider = this._getEIP1193Provider()
    if (!eip1193Provider && !this.isMetaMaskInstalled()) return

    const provider = eip1193Provider || window.ethereum
    if (provider?.on) {
      provider.on('accountsChanged', callback)
    }
  }

  // Listen to chain changes
  onChainChanged(callback) {
    const eip1193Provider = this._getEIP1193Provider()
    if (!eip1193Provider && !this.isMetaMaskInstalled()) return

    const provider = eip1193Provider || window.ethereum
    if (provider?.on) {
      provider.on('chainChanged', callback)
    }
  }

  // Remove listeners
  removeListeners() {
    const eip1193Provider = this._getEIP1193Provider()
    if (!eip1193Provider && !this.isMetaMaskInstalled()) return

    const provider = eip1193Provider || window.ethereum
    if (provider?.removeAllListeners) {
      provider.removeAllListeners('accountsChanged')
      provider.removeAllListeners('chainChanged')
    }
  }

  /**
   * 獲取當前使用的錢包類型
   */
  getCurrentWalletType() {
    return this._currentWalletType
  }

  /**
   * 設置當前錢包類型（用於 autoLogin 恢復時）
   */
  setCurrentWalletType(type) {
    this._currentWalletType = type
  }
}

export const web3Service = new Web3Service()
// 调试暴露：控制台可用 window.__jcWeb3 排查钱包会话问题
if (typeof window !== 'undefined') {
  window.__jcWeb3 = web3Service
}

// 導出 class 供測試構造獨立實例
export { Web3Service }
