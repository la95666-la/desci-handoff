/**
 * Reown AppKit + Wagmi 适配层
 *
 * - 一个 AppKit 实例：弹出"选择钱包"模态，覆盖 MetaMask / OKX / WalletConnect /
 *   Coinbase / Phantom / 硬件钱包等几十种
 * - WagmiAdapter：把 AppKit 状态桥到 wagmi（@wagmi/vue + @wagmi/core）
 * - 网络白名单：BSC mainnet + BSC testnet
 *
 * 项目 ID 来自 import.meta.env.VITE_REOWN_PROJECT_ID，未配置时用 placeholder
 * （WalletConnect 通道在 placeholder 下不可用，但浏览器扩展钱包仍可正常连接）。
 */
import { createAppKit } from '@reown/appkit/vue'
import { bsc, bscTestnet, defineChain } from '@reown/appkit/networks'
import { WagmiAdapter } from '@reown/appkit-adapter-wagmi'
import { orderAppKitNetworks } from './appKitNetworkPolicy.js'
import { parseChainIdList } from './networkPolicy.js'

const projectId =
  import.meta.env.VITE_REOWN_PROJECT_ID || 'placeholder-no-walletconnect'
const checkInChainId = Number(import.meta.env.VITE_CHECKIN_CHAIN_ID)
const checkInEnvironment = import.meta.env.VITE_ENVIRONMENT || ''
const isExplicitCheckInTestBuild =
  checkInEnvironment === 'test' ||
  checkInEnvironment === 'local' ||
  checkInChainId === 97 ||
  checkInChainId === 31337
const isMainnetBuild = checkInEnvironment === 'prod' || checkInChainId === 56 || !isExplicitCheckInTestBuild
// Browser wallets such as OKX reject wallet_addEthereumChain when a local
// Hardhat network advertises its required http://127.0.0.1 RPC (they require
// HTTPS). AppKit otherwise reopens UnsupportedChain on every refresh while the
// wallet remains on its previous chain. Suppress only that automatic local
// modal; transaction guards still require the exact project chain before any
// wallet write. Production and shared test deployments remain strict.
const allowUnsupportedChain = checkInEnvironment === 'local' && checkInChainId === 31337

// 测试环境用的 hardhat 本地链。RPC URL 从 env 注入 —— 保持 prod build 不含此配置。
// 用户在测试环境要签到时，会被 wagmi.switchChain 请求切到该网络（MetaMask 会弹窗添加）。
const HARDHAT_RPC = isMainnetBuild ? '' : import.meta.env.VITE_HARDHAT_RPC || ''
const configuredWalletRpc = allowUnsupportedChain ? import.meta.env.VITE_HARDHAT_WALLET_RPC || '' : ''
const walletRpc = (() => {
  if (!configuredWalletRpc) return HARDHAT_RPC
  try {
    const parsed = new URL(configuredWalletRpc)
    if (parsed.protocol !== 'https:' || parsed.hostname !== '127.0.0.1' || !parsed.port || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
      throw new Error('wallet RPC must be exact https://127.0.0.1:<explicit-port>')
    }
    return parsed.toString().replace(/\/$/, '')
  } catch (error) {
    console.warn('[reownAppKit] ignoring invalid local wallet RPC:', error?.message)
    return HARDHAT_RPC
  }
})()
const hardhatTest = walletRpc
  ? defineChain({
      id: 31337,
      name: 'JoinCare Test (Hardhat)',
      nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
      rpcUrls: {
        default: { http: [walletRpc] },
      },
      testnet: true,
    })
  : null

// 跟 App.vue 的 isHomeDark 同源；初始化时读一次，运行时 App.vue 的 watch 会持续 sync
function readInitialTheme() {
  try {
    const v = localStorage.getItem('jc-theme')
    if (v === 'dark') return 'dark'
    if (v === 'light') return 'light'
  } catch {}
  return 'dark' // 跟 App.vue 一样未设置默认 dark
}

export const appKitNetworks = orderAppKitNetworks({
  isMainnetBuild,
  checkInChainId,
  bsc,
  bscTestnet,
  hardhat: hardhatTest,
  trustedProjectChains: parseChainIdList(import.meta.env.VITE_TRUSTED_PROJECT_CHAINS),
})

const wagmiAdapter = new WagmiAdapter({
  networks: appKitNetworks,
  projectId,
  ssr: false,
})

export const wagmiConfig = wagmiAdapter.wagmiConfig

const isLocalMetadata = import.meta.env.VITE_ENVIRONMENT === 'local' && typeof window !== 'undefined'
const metadataUrl = isLocalMetadata ? window.location.origin : 'https://joincarelabs.com'

export const appKit = createAppKit({
  adapters: [wagmiAdapter],
  networks: appKitNetworks,
  defaultNetwork: appKitNetworks[0],
  allowUnsupportedChain,
  chainImages: {
    [bsc.id]: '/networks/bnb-smart-chain.svg',
    [bscTestnet.id]: '/networks/bnb-smart-chain.svg',
  },
  projectId,
  metadata: {
    name: 'JoinCare',
    description: 'DeSci Bio Research Platform',
    url: metadataUrl,
    icons: [`${metadataUrl}/brand/joincare-avatar-green-black.png`],
  },
  features: {
    analytics: false,
    email: false,
    socials: false, // 纯钱包，不要 Google / Apple / Discord 等社交登录
  },
  themeMode: readInitialTheme(),
  themeVariables: {
    '--w3m-accent': '#9bff00', // JoinCare 荧光绿
    '--w3m-color-mix': '#9bff00',
    '--w3m-color-mix-strength': 8,
    '--w3m-border-radius-master': '6px',
  },
})

/** 兼容 .env 未配置时给出明显提示，避免 placeholder 上线 */
if (projectId === 'placeholder-no-walletconnect') {
  // eslint-disable-next-line no-console
  console.warn(
    '[reownAppKit] VITE_REOWN_PROJECT_ID 未配置，WalletConnect 通道不可用。' +
      '到 https://cloud.reown.com 免费注册项目 ID 后填入 .env'
  )
}
