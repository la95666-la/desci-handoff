import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import { watchAccount, disconnect as wagmiDisconnect, getAccount } from '@wagmi/core'
import { web3Service } from '../services/web3'
import { appKit, wagmiConfig } from '../services/reownAppKit'
import {
  autoLogin,
  clearAuth,
  getAuth,
  getAuthNonce,
  getPlatformSettings,
  getRegistrationStatus,
  isAuthenticated,
  loginWallet,
  registerWallet,
  setAuth,
  setLanguage
} from '../services/api'
import { applyPlatformSettings, LOGIN_NETWORK_POLICY } from '../config/contracts'
import { isChainTrusted, normalizeChainId } from '../services/networkPolicy'
import { WALLET_ERROR_CODES } from '../services/errors'
import { shouldAuthenticateAccountTransition } from '../services/accountTransitionPolicy.js'

export const useWalletStore = defineStore('wallet', () => {
  // State
  const address = ref(null)
  const chainId = ref(null)
  const isConnected = ref(false)
  const balance = ref('0')
  // 每重建一次 ethers 會話就 +1，讓 Launchpad/Dashboard 重新讀取鏈上數據。
  const sessionRevision = ref(0)
  const jccBalance = ref('0')
  const isConnecting = ref(false)
  const error = ref(null)
  const currentWalletType = ref(null) // 當前連接的錢包類型

  // 認證狀態
  const uid = ref(getAuth().uid || null)
  const token = ref(getAuth().token || '')
  const userType = ref(null) // 'backer' | 'creator'
  const registrationStatus = ref(null)
  const isCheckingRegistration = ref(false)
  const isAuthenticating = ref(false)

  // 錢包選擇彈窗狀態
  const showWalletSelector = ref(false)
  const walletSelectorMode = ref('connect') // 'connect' | 'payment'
  const _walletSelectResolve = ref(null)
  const _walletSelectReject = ref(null)

  function normalizeWalletType(type) {
    const raw = String(type || '').trim()
    const lower = raw.toLowerCase()
    if (!lower) return ''
    if (lower.includes('okx') || lower.includes('okex')) return 'okx'
    if (lower.includes('metamask')) return 'metamask'
    if (lower.includes('coinbase')) return 'coinbase'
    return raw
  }

  // Getters
  const shortAddress = computed(() => {
    if (!address.value) return ''
    return `${address.value.slice(0, 6)}...${address.value.slice(-4)}`
  })

  const isCorrectNetwork = computed(() => {
    // 與 LOGIN_NETWORK_POLICY（顯式受信配置）保持一致：
    // 本地構建信任 56/97/31337，生產只信任 56
    return LOGIN_NETWORK_POLICY.trustedChainIds.includes(chainId.value)
  })

  const isLoggedIn = computed(() => {
    return isConnected.value && !!uid.value && !!token.value
  })

  const isRegistered = computed(() => {
    return !!registrationStatus.value?.registered || (!!uid.value && !!token.value)
  })

  const needsRegistration = computed(() => {
    return isConnected.value && !isLoggedIn.value && registrationStatus.value?.registered === false
  })

  // ============ 錢包選擇彈窗 API ============

  /**
   * 打開錢包選擇彈窗，返回 Promise<WalletType>
   * 用戶選擇後 resolve，取消則 reject
   * @param {string} mode - 'connect' 或 'payment'
   */
  function requestWalletSelection(mode = 'connect') {
    return new Promise((resolve, reject) => {
      walletSelectorMode.value = mode
      _walletSelectResolve.value = resolve
      _walletSelectReject.value = reject
      showWalletSelector.value = true
    })
  }

  /**
   * 用戶選擇了錢包（由 WalletSelector 組件調用）
   */
  function onWalletSelected(walletType) {
    if (_walletSelectResolve.value) {
      _walletSelectResolve.value(walletType)
      _walletSelectResolve.value = null
      _walletSelectReject.value = null
    }
    showWalletSelector.value = false
  }

  /**
   * 用戶取消了錢包選擇
   */
  function onWalletSelectionCancelled() {
    if (_walletSelectReject.value) {
      _walletSelectReject.value(new Error('User cancelled wallet selection'))
      _walletSelectResolve.value = null
      _walletSelectReject.value = null
    }
    showWalletSelector.value = false
  }

  // 監聽彈窗關閉：當用戶通過遮罩層/取消按鈕關閉彈窗時
  // v-model:visible 會直接把 showWalletSelector 設為 false
  // 此時需要自動 reject 掛起的 Promise，否則 isConnecting 會永遠卡住
  watch(showWalletSelector, (visible) => {
    if (!visible && _walletSelectReject.value) {
      _walletSelectReject.value(new Error('User cancelled wallet selection'))
      _walletSelectResolve.value = null
      _walletSelectReject.value = null
    }
  })

  // ============ JoinCare 註冊 / 登入 ============

  function getStoredInviteCode(fallback = '') {
    if (fallback) return fallback
    if (typeof sessionStorage === 'undefined') return ''
    return sessionStorage.getItem('jc_invite_code') || ''
  }

  function clearStoredInviteCode() {
    if (typeof sessionStorage === 'undefined') return
    sessionStorage.removeItem('jc_invite_code')
  }

  function normalizeLoginData(data) {
    if (!data) return null

    const authToken = data.signature || data.token || data.sign || data.authToken || data.accessToken || data.jcSign || ''
    const authUid = data.uid ?? data.userId ?? data.userID ?? data.person ?? ''

    return {
      uid: authUid ? String(authUid) : '',
      token: authToken,
      userType: data.type || data.userType || data.role || null,
      walletAddress: data.walletAddress || data.address || data.wallet_address || address.value,
      inviteCode: data.inviteCode || data.invite_code || registrationStatus.value?.inviteCode || ''
    }
  }

  function normalizeRegistrationStatus(data, walletAddress) {
    if (!data) return null

    const registeredValue = data.registered ?? data.isRegistered ?? data.exists ?? false
    const registered = registeredValue === true || registeredValue === 'true' || registeredValue === 1 || registeredValue === '1'

    return {
      ...data,
      registered,
      walletAddress: data.walletAddress || data.address || data.wallet_address || walletAddress
    }
  }

  function applyLoginData(data) {
    const loginData = normalizeLoginData(data)
    if (!loginData?.uid || !loginData?.token) {
      throw new Error('Login response missing auth token')
    }

    uid.value = loginData.uid
    token.value = loginData.token
    userType.value = loginData.userType

    setAuth(loginData.token, loginData.uid)
    // 登入態就緒後觸發一次重新載入，避免 address 先於 token 就緒時，
    // 后端签名接口因未登录返回 401 导致余额/授权读成默认值。
    sessionRevision.value += 1

    registrationStatus.value = {
      walletAddress: loginData.walletAddress,
      registered: true,
      uid: loginData.uid,
      inviteCode: loginData.inviteCode
    }

    return loginData
  }

  async function checkRegistration(walletAddress = address.value) {
    if (!walletAddress) return null

    isCheckingRegistration.value = true
    try {
      const data = await getRegistrationStatus(walletAddress)
      const status = normalizeRegistrationStatus(data, walletAddress)
      registrationStatus.value = status
      return status
    } finally {
      isCheckingRegistration.value = false
    }
  }

  function resetLoginState() {
    uid.value = null
    token.value = ''
    userType.value = null
    registrationStatus.value = null
    clearAuth()
  }

  async function signAndSubmitAuth(action, inviteCode = '') {
    if (!address.value) {
      throw new Error('Please connect wallet first')
    }

    isAuthenticating.value = true
    error.value = null

    try {
      // 安全審計 0.8：簽名前強制鏈校驗。受信鏈 / 自動切鏈策略統一從
      // LOGIN_NETWORK_POLICY（顯式受信配置）讀取：
      //  - 生產構建只認 56（可自動切到 56）
      //  - 本地構建信任 56/97/31337 且**不自動切鏈**（絕不把用戶強行拉到 97）
      //  - 測試構建放行 56/97（偏好 97，可自動切鏈）
      const { preferredChainId, autoSwitch } = LOGIN_NETWORK_POLICY
      if (!isChainTrusted(LOGIN_NETWORK_POLICY, chainId.value)) {
        // 本地(31337)構建 autoSwitch=false ⇒ 不嘗試任何自動切鏈，直接放行/攔截
        if (autoSwitch) {
          try {
            await switchNetwork(preferredChainId)
          } catch (switchErr) {
            console.warn('[signAndSubmit] switchNetwork failed:', switchErr?.message)
          }
        }
        if (!isChainTrusted(LOGIN_NETWORK_POLICY, chainId.value)) {
          const err = new Error('Wrong network. Please switch to the supported network and try again.')
          err.code = WALLET_ERROR_CODES.NETWORK_MISMATCH
          throw err
        }
      }

      const nonceData = await getAuthNonce(address.value, action)
      const message = nonceData.message
      // 登录消息本身与链无关：始终对用户实际选择的 connector provider 调用
      // EIP-1193 personal_sign，避免 wagmi signMessage 为匹配 active chain 触发切链。
      const signature = await web3Service.personalSign(message, address.value)
      const payload = {
        walletAddress: address.value,
        message,
        signature
      }

      const data = action === 'register'
        ? await registerWallet({
          ...payload,
          inviteCode: inviteCode || undefined
        })
        : await loginWallet(payload)

      const loginData = applyLoginData(data)
      if (action === 'register') {
        clearStoredInviteCode()
      }
      await loadPlatformSettings()
      return loginData
    } catch (err) {
      error.value = err.message
      throw err
    } finally {
      isAuthenticating.value = false
    }
  }

  async function loginCurrentWallet() {
    const status = await checkRegistration()
    if (!status?.registered) {
      throw new Error('This wallet has not registered JoinCare yet')
    }
    return await signAndSubmitAuth('login')
  }

  async function registerCurrentWallet(inviteCode = '') {
    const status = await checkRegistration()
    if (status?.registered) {
      return await signAndSubmitAuth('login')
    }
    return await signAndSubmitAuth('register', getStoredInviteCode(inviteCode))
  }

  async function authenticateCurrentWallet(inviteCode = '', walletAddress = address.value) {
    let status = null
    try {
      status = await checkRegistration(walletAddress)
    } catch (registrationErr) {
      console.warn('[authenticateCurrentWallet] Registration status check failed:', registrationErr.message)
      error.value = registrationErr.message
      throw registrationErr
    }

    const action = status?.registered ? 'login' : 'register'
    try {
      const data = await signAndSubmitAuth(action, getStoredInviteCode(inviteCode))
      return { action, data }
    } catch (authErr) {
      console.warn('[authenticateCurrentWallet] Wallet connected but JoinCare auth was not completed:', authErr.message)
      throw authErr
    }
  }

  // ============ 連接流程 ============

  /**
   * 連接錢包（Reown AppKit + Wagmi）
   *   - 弹出 AppKit 模态（MetaMask / OKX / WalletConnect / Coinbase / Phantom / ...）
   *   - 用户选完后 wagmi 触发 watchAccount → 进入 JoinCare nonce 签名流程
   *   - 旧 walletManager 流程被替换；保留 web3Service 用作 ethers 兼容层
   */
  async function connectWallet(inviteCode = '') {
    isConnecting.value = true
    error.value = null

    // 已连接 → 直接进 auth；否则弹 AppKit 模态
    const initial = getAccount(wagmiConfig)
    if (initial.isConnected && initial.address) {
      try {
        await applyWagmiAccount(initial, inviteCode)
      } finally {
        isConnecting.value = false
      }
      return { address: initial.address, chainId: initial.chainId }
    }

    return new Promise((resolve, reject) => {
      // 一次性 watch：用户在 AppKit 模态选钱包并签名后，wagmi 状态会变
      let stop = () => {}
      stop = watchAccount(wagmiConfig, {
        async onChange(acc, prev) {
          if (shouldAuthenticateAccountTransition(acc, prev)) {
            stop()
            try {
              const data = await applyWagmiAccount(acc, inviteCode)
              isConnecting.value = false
              resolve({ address: acc.address, chainId: acc.chainId, authData: data })
            } catch (e) {
              isConnecting.value = false
              error.value = e.message
              // Wallet authorization and JoinCare authentication are separate
              // states. A nonce/API/signature failure must not disconnect an
              // already authorized EIP-1193 session or make the UI ask the
              // user to connect again. Token-gated routes continue to use
              // isLoggedIn and therefore remain fail closed.
              reject(e)
            }
          }
        },
      })
      // 用户关闭 modal 也要把状态复位（AppKit 没有直接 cancel callback，用 modal state 监听）
      const unsubModal = appKit.subscribeState?.((s) => {
        if (s.open === false) {
          // modal 关闭时若还没连上，按 cancel 处理
          const cur = getAccount(wagmiConfig)
          if (!cur.isConnected) {
            stop()
            unsubModal?.()
            isConnecting.value = false
            resolve(null)
          } else {
            unsubModal?.()
          }
        }
      })
      // 打开 AppKit modal
      appKit.open()
    })
  }

  /** wagmi account → walletStore state + 触发 JoinCare auth */
  async function applyWagmiAccount(acc, inviteCode = '') {
    const connectorProvider = await acc.connector?.getProvider?.()
    if (!connectorProvider?.request) {
      throw new Error('Selected wallet connector did not expose an EIP-1193 provider')
    }
    web3Service.setExternalEIP1193Provider(connectorProvider)
    const [liveChainId, liveAccounts] = await Promise.all([
      connectorProvider.request({ method: 'eth_chainId' }),
      connectorProvider.request({ method: 'eth_accounts' }),
    ])
    const liveAddress = Array.isArray(liveAccounts) && liveAccounts[0] ? liveAccounts[0] : acc.address
    const normalizedLiveChainId = normalizeChainId(liveChainId)
    if (!liveAddress || normalizedLiveChainId == null) {
      web3Service.setExternalEIP1193Provider(null)
      throw new Error('Selected wallet connector returned an invalid account or chain')
    }
    // Reown/Wagmi 只維護 connector 狀態；Dashboard/合約服務使用 ethers
    // BrowserProvider。連接完成後立即建立同一 provider 的 ethers 會話，避免
    // 登入成功後 Dashboard 首次刷新落入「Wallet not connected」的半初始化狀態。
    await web3Service.createSession(connectorProvider)
    address.value = liveAddress
    chainId.value = normalizedLiveChainId
    isConnected.value = true
    currentWalletType.value = normalizeWalletType(acc.connector?.id || acc.connector?.name || 'wagmi')
    // 余额 / 网络切换走 wagmi/viem，这里 Phase 1 先留空避免阻塞登录
    balance.value = '0'
    jccBalance.value = '0'

    localStorage.setItem('walletConnected', 'true')
    localStorage.setItem('walletAddress', acc.address)
    if (currentWalletType.value) {
      localStorage.setItem('walletType', currentWalletType.value)
    }
    resetLoginState()

    const authResult = await authenticateCurrentWallet(inviteCode, address.value)
    return authResult?.data
  }

  /**
   * 使用指定類型錢包連接（跳過選擇彈窗）
   */
  async function connectWalletByType(walletType, options = {}) {
    const { authenticate = true, inviteCode = '' } = options
    isConnecting.value = true
    error.value = null

    try {
      const walletData = await web3Service.connectWalletByType(walletType)

      address.value = walletData.address
      chainId.value = walletData.chainId
      balance.value = walletData.balance
      jccBalance.value = walletData.jccBalance
      isConnected.value = true
      currentWalletType.value = walletType

      localStorage.setItem('walletConnected', 'true')
      localStorage.setItem('walletAddress', walletData.address)
      localStorage.setItem('walletType', walletType)

      setupListeners()

      let authResult = null
      if (authenticate) {
        resetLoginState()
        authResult = await authenticateCurrentWallet(inviteCode, walletData.address)
      }

      return {
        ...walletData,
        authAction: authResult?.action,
        authData: authResult?.data
      }
    } catch (err) {
      error.value = err.message
      throw err
    } finally {
      isConnecting.value = false
    }
  }

  function disconnect() {
    // Clear state
    address.value = null
    chainId.value = null
    isConnected.value = false
    balance.value = '0'
    jccBalance.value = '0'
    error.value = null
    uid.value = null
    token.value = ''
    userType.value = null
    currentWalletType.value = null
    registrationStatus.value = null
    isCheckingRegistration.value = false
    isAuthenticating.value = false

    // Clear localStorage
    localStorage.removeItem('walletConnected')
    localStorage.removeItem('walletAddress')
    localStorage.removeItem('walletType')

    // 清除認證資訊
    clearAuth()

    // Disconnect from web3 service (legacy)
    web3Service.disconnect()
    web3Service.removeListeners()

    // 通过 wagmi 断开当前 AppKit 连接（覆盖 WalletConnect / Coinbase 等远端 session）
    wagmiDisconnect(wagmiConfig).catch((err) => {
      console.warn('[disconnect] wagmi disconnect failed:', err?.message)
    })

    // 扩展钱包额外撤销账户授权（让下次连接重新弹账户选择器）
    try {
      const provider = web3Service._getEIP1193Provider() || window.ethereum
      if (provider) {
        provider.request({
          method: 'wallet_revokePermissions',
          params: [{ eth_accounts: {} }]
        }).catch(err => {
          console.warn('[disconnect] wallet_revokePermissions not supported:', err.message)
        })
      }
    } catch (e) {
      // 忽略
    }
  }

  async function switchNetwork(chainId) {
    try {
      // 按目標 chainId 切鏈（內部只支援有網絡參數的 56/97；31337 等本地鏈
      // 由策略 autoSwitch=false 擋在門外，這裡不會被調到）
      await web3Service.switchToChain(chainId)
      // Reconnect to update chain info
      if (currentWalletType.value) {
        await connectWalletByType(currentWalletType.value, { authenticate: false })
      } else {
        await connectWallet()
      }
    } catch (err) {
      error.value = err.message
      throw err
    }
  }

  async function syncChainId() {
    const provider = web3Service._getEIP1193Provider()
    if (!provider?.request) return chainId.value
    try {
      const liveChainId = normalizeChainId(await provider.request({ method: 'eth_chainId' }))
      if (liveChainId != null) {
        chainId.value = liveChainId
        web3Service.chainId = liveChainId
      }
      return liveChainId
    } catch (err) {
      console.warn('[syncChainId] 無法讀取錢包實時鏈:', err.message)
      return chainId.value
    }
  }

  async function refreshBalances() {
    if (!isConnected.value || !address.value) return

    try {
      if (currentWalletType.value) {
        const walletData = await web3Service.connectWalletByType(currentWalletType.value)
        balance.value = walletData.balance
        jccBalance.value = walletData.jccBalance
      } else {
        const walletData = await web3Service.connectWallet()
        balance.value = walletData.balance
        jccBalance.value = walletData.jccBalance
      }
    } catch (err) {
      console.error('Failed to refresh balances:', err)
    }
  }

  function setupListeners() {
    // Listen for account changes
    web3Service.onAccountsChanged(async (accounts) => {
      if (accounts.length === 0) {
        disconnect()
      } else if (accounts[0] !== address.value) {
        try {
          if (currentWalletType.value) {
            await connectWalletByType(currentWalletType.value)
          } else {
            await connectWallet()
          }
        } catch (err) {
          console.error('Failed to reconnect after account change:', err)
        }
      }
    })

    // Listen for chain changes. Do not reload here: TaskHall check-in calls
    // wagmi.switchChain before writeContract, and a hard reload interrupts
    // the transaction flow before the wallet signature prompt appears.
    // web3Service.handleChainChanged 會清理並重建 BrowserProvider/signer——
    // 舊 BrowserProvider 可能緩存舊鏈，不重建的話後續寫交易會綁錯鏈。
    web3Service.onChainChanged((nextChainId) => {
      const parsed = web3Service.handleChainChanged(nextChainId)
      if (parsed != null) {
        chainId.value = parsed
      }
      // Balance refresh goes through legacy connect paths that can request
      // accounts again. Keep this event lightweight; wagmi owns the active
      // transaction flow and page data will refresh through normal watchers.
    })
  }

  async function autoConnect() {
    // 禁用自動重連 — 每次由用戶手動選擇賬戶
    localStorage.removeItem('walletConnected')
    localStorage.removeItem('walletAddress')
  }

  /**
   * 嘗試使用現有 token 自動登入刷新
   */
  async function tryAutoLogin() {
    if (!isAuthenticated()) {
      console.log('[tryAutoLogin] 跳過：localStorage 中無 token/uid')
      return false
    }

    // 確保模塊變量與 localStorage 同步（HMR 可能導致不同步）
    const savedToken = localStorage.getItem('jc_token')
    const savedUid = localStorage.getItem('jc_uid')
    setAuth(savedToken, savedUid)

    console.log('[tryAutoLogin] 發起 autoLogin 請求...')
    try {
      const data = await autoLogin()
      const loginData = applyLoginData(data)

      // 恢復錢包連接狀態
      if (loginData.walletAddress) {
        address.value = loginData.walletAddress
        isConnected.value = true
      }

      // 恢復錢包類型
      const savedWalletType = normalizeWalletType(localStorage.getItem('walletType'))
      if (savedWalletType) {
        currentWalletType.value = savedWalletType
        web3Service.setCurrentWalletType(savedWalletType)
      }

      // 靜默重連錢包（不彈窗），獲取 chainId 和余額
      // 根据 savedWalletType 精确选择 provider —— 避免 MetaMask 抢占 window.ethereum
      // 导致用户其实用 OKX 登录但每次刷新弹 MetaMask 请求授权
      try {
        // 刷新後靜默重連：統一走 wagmi/AppKit 當前會話的 connector provider
        // （WalletConnect 注入的 OKX 等）。不要按 savedWalletType 去抓
        // window.ethereum / window.okxwallet —— 對 WalletConnect 連接而言那是
        // 未授權的另一套 injected provider，會讓餘額讀成 0。
        let provider = web3Service._getEIP1193Provider() || null
        if (!provider) {
          const wagmiAccount = getAccount(wagmiConfig)
          if (wagmiAccount.isConnected && wagmiAccount.connector) {
            try {
              const connectorProvider = await wagmiAccount.connector.getProvider?.()
              if (connectorProvider && typeof connectorProvider.request === 'function') {
                provider = connectorProvider
              }
            } catch (e) {
              console.warn('[tryAutoLogin] 獲取 wagmi connector provider 失敗:', e.message)
            }
          }
        }
        // 兜底：wagmi 未重連（WalletConnect 會話可能丟失/未恢復）時，
        // 改用瀏覽器注入的 provider（OKX/MetaMask 插件本身）。
        if (!provider) {
          if (savedWalletType === 'okx') provider = window.okxwallet || null
          else if (savedWalletType === 'metamask') provider = window.ethereum || null
          else if (savedWalletType === 'coinbase') provider = window.coinbaseWalletExtension || null
        }
        if (provider) {
          // eth_accounts 是 read-only（不弹授权窗）；如果这里仍弹窗说明该 provider 状态异常
          const accounts = await provider.request({ method: 'eth_accounts' })
          if (accounts && accounts.length > 0) {
            const { BrowserProvider, formatUnits } = await import('ethers')
            const ethersProvider = new BrowserProvider(provider)
            const network = await ethersProvider.getNetwork()
            chainId.value = Number(network.chainId)

            // 獲取余額
            const bal = await ethersProvider.getBalance(accounts[0])
            balance.value = formatUnits(bal, 18)

            // 初始化 web3Service（重建 BrowserProvider + signer 並記錄綁定快照，
            // 之後 ensureSigner 才能準確判斷該不該重建）
            await web3Service.createSession(provider)

            setupListeners()
            sessionRevision.value += 1
          }
        }
      } catch (walletErr) {
        console.warn('[tryAutoLogin] 靜默重連錢包失敗（不影響登入）:', walletErr.message)
      }

      // 登入成功後加載平臺配置（鏈地址等）
      await loadPlatformSettings()

      console.log('[tryAutoLogin] 自動登入成功, uid:', loginData.uid)
      return true
    } catch (err) {
      console.warn('[tryAutoLogin] 自動登入失敗:', err)
      clearAuth()
      uid.value = null
      token.value = ''
      registrationStatus.value = null
      return false
    }
  }

  function updateBalance(newBalance) {
    balance.value = newBalance
  }

  function updateJccBalance(newBalance) {
    jccBalance.value = newBalance
  }

  /**
   * 更新語言設置并同步到 API 層
   */
  function updateLanguage(lang) {
    setLanguage(lang)
  }

  /**
   * 從後端加載平臺配置（ChainUrl / PlatformFee），
   * 根據 ChainUrl 自動切換 testnet / mainnet。
   */
  let _settingsLoaded = false
  // 平台鏈默認值 / 是否自動切鏈，統一從 LOGIN_NETWORK_POLICY 讀取：
  // 本地構建（31337）autoSwitch=false ⇒ 加載設置後絕不會自動切到 97。
  const shouldAutoSwitchPlatformChain = (targetChainId) => {
    return LOGIN_NETWORK_POLICY.autoSwitch && LOGIN_NETWORK_POLICY.trustedChainIds.includes(targetChainId)
  }
  async function loadPlatformSettings() {
    if (_settingsLoaded) return


    const CACHE_KEY = 'jc_platform_settings'
    const CACHE_TTL = 24 * 60 * 60 * 1000 // 24小時
    try {
      const cached = localStorage.getItem(CACHE_KEY)
      if (cached) {
        const { data, ts } = JSON.parse(cached)
        if (Date.now() - ts < CACHE_TTL && data) {
          // 不受信鏈的緩存（如 prod 構建裡的 testnet 設置）直接丟棄，避免汙染配置
          if (!LOGIN_NETWORK_POLICY.trustedChainIds.includes(parseInt(data?.ChainID, 10))) {
            localStorage.removeItem(CACHE_KEY)
          } else {
            applyPlatformSettings(data)
            _settingsLoaded = true
            console.log('[loadPlatformSettings] 使用本地緩存')
            // 雙鏈形態：applyPlatformSettings 返回的是項目鏈 ID，不能作為自動切換目標。
            // 登入/簽到落地鏈固定用策略偏好鏈（單鏈部署兩者相同，行為不變）。
            const preferredChainId = LOGIN_NETWORK_POLICY.preferredChainId
            if (shouldAutoSwitchPlatformChain(preferredChainId)) {
              await ensureCorrectNetwork(preferredChainId)
            }
            return
          }
        }
      }
    } catch (e) { /* 緩存解析失敗，繼續請求 */ }

    try {
      const settings = await getPlatformSettings()
      applyPlatformSettings(settings)
      _settingsLoaded = true
      localStorage.setItem(CACHE_KEY, JSON.stringify({ data: settings, ts: Date.now() }))
    } catch (err) {
      console.warn('[loadPlatformSettings] 獲取平臺配置失敗（使用預設配置）:', err.message)
    }

    // 同上：自動切換目標是策略偏好鏈（簽到/登入鏈），不是 applyPlatformSettings 返回的項目鏈。
    const preferredChainId = LOGIN_NETWORK_POLICY.preferredChainId
    if (shouldAutoSwitchPlatformChain(preferredChainId)) {
      await ensureCorrectNetwork(preferredChainId)
    }
  }

  async function ensureCorrectNetwork(targetChainId) {
    if (!isConnected.value) return

    // 只有策略允許自動切鏈（非本地構建）且目標鏈受信時才動手：
    // 本地 31337 構建 autoSwitch=false ⇒ 即使被直接調用也絕不切鏈、絕不強切 97
    if (!shouldAutoSwitchPlatformChain(targetChainId)) return

    // 檢查當前錢包是否安裝
    const eip1193Provider = web3Service._getEIP1193Provider()
    if (!eip1193Provider && !web3Service.isMetaMaskInstalled()) return

    if (chainId.value !== targetChainId) {
      console.log(`[ensureCorrectNetwork] 當前鏈 ${chainId.value}，目標鏈 ${targetChainId}，自動切換...`)
      try {
        await web3Service.switchToChain(targetChainId)
        chainId.value = targetChainId
      } catch (err) {
        console.warn('[ensureCorrectNetwork] 自動切鏈失敗（用戶可能拒絕）:', err.message)
      }
    }
  }

  // 刷新後 wagmi（WalletConnect 等）會自動重連：acc.isConnected 變 true，
  // 但 ethers 會話還沒綁到新的 connector provider。重建會話（不重新簽名），
  // 並用 sessionRevision 通知 Launchpad/Dashboard 重新讀鏈上數據。
  async function rebindWagmiSession(acc) {
    if (!acc.isConnected || !acc.address || !acc.connector) return
    try {
      const connectorProvider = await acc.connector.getProvider?.()
      if (!connectorProvider || typeof connectorProvider.request !== 'function') return
      web3Service.setExternalEIP1193Provider(connectorProvider)
      const [liveChainId, liveAccounts] = await Promise.all([
        connectorProvider.request({ method: 'eth_chainId' }),
        connectorProvider.request({ method: 'eth_accounts' }),
      ])
      const liveAddress = Array.isArray(liveAccounts) && liveAccounts[0] ? liveAccounts[0] : acc.address
      const nextChainId = normalizeChainId(liveChainId)
      if (!liveAddress || nextChainId == null) return
      await web3Service.createSession(connectorProvider)
      // 若 store 地址尚未恢復，或與 wagmi 地址一致，則同步地址與連接態。
      if (!address.value || String(liveAddress).toLowerCase() === String(address.value).toLowerCase()) {
        address.value = liveAddress
        isConnected.value = true
      }
      chainId.value = nextChainId
      sessionRevision.value += 1
    } catch (e) {
      console.warn('[wallet] 刷新後重建 wagmi 會話失敗:', e.message)
    }
  }

  watchAccount(wagmiConfig, { onChange: rebindWagmiSession })
  // 立即檢查一次：若 wagmi 在 store 初始化前已完成重連，onChange 不會再觸發。
  const _initialWagmi = getAccount(wagmiConfig)
  if (_initialWagmi.isConnected) {
    rebindWagmiSession(_initialWagmi)
  }

  return {
    // State
    address,
    chainId,
    isConnected,
    sessionRevision,
    balance,
    jccBalance,
    isConnecting,
    error,
    uid,
    token,
    userType,
    currentWalletType,
    registrationStatus,
    isCheckingRegistration,
    isAuthenticating,
    // 錢包選擇器狀態
    showWalletSelector,
    walletSelectorMode,
    // Getters
    shortAddress,
    isCorrectNetwork,
    isLoggedIn,
    isRegistered,
    needsRegistration,
    // Actions
    connectWallet,
    connectWalletByType,
    disconnect,
    switchNetwork,
    refreshBalances,
    syncChainId,
    autoConnect,
    tryAutoLogin,
    checkRegistration,
    authenticateCurrentWallet,
    loginCurrentWallet,
    registerCurrentWallet,
    updateBalance,
    updateJccBalance,
    updateLanguage,
    // 錢包選擇器 Actions
    requestWalletSelection,
    onWalletSelected,
    onWalletSelectionCancelled
  }
})
