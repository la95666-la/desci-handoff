/**
 * 合約地址、ABI 和鏈配置
 * 
 * 所有鏈上相關配置由後端 /platform/v1/settings 接口提供：
 *   - ChainEnvironment: 'local' | 'test' | 'prod'
 *   - ChainID:          鏈 ID（字符串）
 *   - ChainUrl:         RPC 地址
 *   - ContractAddress:  JoinCare 合約地址
 *   - JccAddress:       JCC 代幣合約地址
 *   - PlatformFee:      平臺手續費
 * 
 * 使用方式：
 *   import { ADDRESSES, ACTIVE_CHAIN, applyPlatformSettings } from '../config/contracts'
 *   // ADDRESSES 和 ACTIVE_CHAIN 是可變對象，applyPlatformSettings() 會就地更新屬性
 */

import { createLoginNetworkPolicy, isChainTrusted, normalizeChainId, parseChainIdList } from '../services/networkPolicy.js'
import { isLoopbackHttpUrl } from '../services/fairLaunchSubmissionPolicy.js'

// ============ 瀏覽器 URL 映射 ============

const EXPLORER_MAP = {
    97: 'https://testnet.bscscan.com',
    56: 'https://bscscan.com'
}

// 後端 TaskChain.ChainID 運行期校準 CHECKIN_CHAIN 用的鏈名 / 幣種映射（雙鏈拆分）。
const CHAIN_NAME_MAP = {
    56: 'BNB Smart Chain',
    97: 'BNB Smart Chain Testnet',
    31337: 'JoinCare Test (Hardhat)'
}

const CURRENCY_MAP = {
    56: { name: 'BNB', symbol: 'BNB', decimals: 18 },
    97: { name: 'tBNB', symbol: 'tBNB', decimals: 18 },
    31337: { name: 'ETH', symbol: 'ETH', decimals: 18 }
}

// Vite 下 import.meta.env 一定存在；node --test 直接 import 本模塊時為 undefined，
// 退化成空對象 ⇒ 完全未配置 ⇒ 走 fail-closed 主網判定（與生產行為一致）。
const viteEnv = import.meta.env || {}
const checkInChainId = Number(viteEnv.VITE_CHECKIN_CHAIN_ID)
const checkInEnvironment = viteEnv.VITE_ENVIRONMENT || ''
const isExplicitCheckInTestBuild =
    checkInEnvironment === 'test' ||
    checkInEnvironment === 'local' ||
    checkInChainId === 97 ||
    checkInChainId === 31337
const isMainnetBuild = checkInEnvironment === 'prod' || checkInChainId === 56 || !isExplicitCheckInTestBuild
const MAINNET_CHECKIN_ADDRESS = '0xE029161be55922edf3eC9D222142edf057d196ee'

// ============ 當前活躍配置（可變對象，applyPlatformSettings 就地更新） ============

/** 當前鏈配置 */
export const ACTIVE_CHAIN = {
    chainId: 97,
    chainIdHex: '0x61',
    name: 'BNB Smart Chain Testnet',
    rpcUrl: 'https://bsc-testnet.drpc.org',
    explorerUrl: 'https://testnet.bscscan.com',
    currency: { name: 'tBNB', symbol: 'tBNB', decimals: 18 }
}

/** 當前合約地址 */
export const ADDRESSES = {
    JCC: '0xFc6C84ac73c11f6C162B9dd5B4A94e7bfb3308EA',
    JoinCare: '0xe6F01eC4dEA815c9dAAb79d7A7D0FA16b2cB368A',
    // JoinCareCheckIn (V2 self-paid) proxy 地址。
    // 优先级：VITE_CHECKIN_ADDRESS (build-time) > hardhat 默认地址
    // 生产 BSC: build 时用 JC_CHECKIN_CONTRACT 注入实际部署地址
    CheckIn: viteEnv.VITE_CHECKIN_ADDRESS || (isMainnetBuild ? MAINNET_CHECKIN_ADDRESS : '0xe7f1725E7734CE288F8367e1Bb143E90bb3F0512')
}

/** JoinCareCheckIn 所在的链（跟 IDO 用的 ACTIVE_CHAIN 可以不同）。
 * build 时用 VITE_CHECKIN_CHAIN_ID/NAME/CURRENCY/RPC 注入：
 *   BSC mainnet:  56 / 'BNB Smart Chain' / 'BNB' / 'https://bsc-dataseed.bnbchain.org'
 *   BSC testnet:  97 / 'BSC Testnet' / 'tBNB' / 'https://data-seed-prebsc-1-s1.binance.org:8545'
 *   Hardhat:      31337 / 'JoinCare Test (Hardhat)' / 'ETH' / (来自 JC_CHAIN_RPC_PUBLIC)
 */
export const CHECKIN_CHAIN = {
    chainId: isMainnetBuild ? 56 : checkInChainId || 31337,
    name: isMainnetBuild ? 'BNB Smart Chain' : viteEnv.VITE_CHECKIN_CHAIN_NAME || 'JoinCare Test (Hardhat)',
    rpcUrl: isMainnetBuild ? '' : viteEnv.VITE_HARDHAT_RPC || '',
    currency: { symbol: isMainnetBuild ? 'BNB' : viteEnv.VITE_CHECKIN_CURRENCY || 'ETH' }
}

/**
 * 登入網絡策略（從上面的顯式受信配置推導，詳見 services/networkPolicy.js）。
 * - 生產構建：只信任 56，允許自動切到 56
 * - 本地構建（31337 / local 默認）：信任 56/97/31337，不自動切鏈（絕不強切 97）
 * - 測試構建：信任 56/97，偏好 97，允許自動切鏈
 */
// 顯式信任的項目鏈（雙鏈發布形態注入，如 VITE_TRUSTED_PROJECT_CHAINS='97'；逗號分隔）。
// 不注入 = 主網構建只信任 56（fail-closed 默認完全不變）。
const trustedProjectChains = parseChainIdList(viteEnv.VITE_TRUSTED_PROJECT_CHAINS)

export const LOGIN_NETWORK_POLICY = createLoginNetworkPolicy({
    environment: checkInEnvironment,
    checkinChainId: checkInChainId,
    trustedProjectChains,
})

/** 平臺配置緩存 */
export const platformConfig = {
    chainEnvironment: '',
    chainId: '',
    chainUrl: '',
    platformFee: ''
}

/**
 * 用後端 settings 更新所有鏈配置
 * 登入成功後調用此函數
 * 
 * @param {object} settings - 後端返回的 settings 數據
 * @param {string} settings.ChainEnvironment - 'local' | 'test' | 'prod'
 * @param {string} settings.ChainID          - 鏈 ID，如 '97'
 * @param {string} settings.ChainUrl         - RPC 地址
 * @param {string} settings.ContractAddress  - JoinCare 合約地址
 * @param {string} settings.JccAddress       - JCC 代幣合約地址
 * @param {string} settings.PlatformFee      - 平臺手續費百分比
 * @returns {number} chainId
 */
export function applyPlatformSettings(settings) {
    if (!settings) return ACTIVE_CHAIN.chainId
    const result = applyPlatformSettingsInner(settings)
    // 暴露当前生效的 RPC（交易回执轮询直连用；WalletConnect 读请求可能挂起）
    if (typeof window !== 'undefined') window.__JC_ACTIVE_RPC_URL = ACTIVE_CHAIN.rpcUrl || null
    return result
}

function applyPlatformSettingsInner(settings) {

    // ============ 項目鏈（Launchpad / IDO）：ACTIVE_CHAIN ============
    // 新結構 ProjectChain 與舊扁平字段並存一個發布週期；新結構優先。
    const project = settings.ProjectChain || null
    const projectChainIdRaw = project?.ChainID ?? settings.ChainID
    const projectContract = project?.ContractAddress ?? settings.ContractAddress
    const projectUrl = project?.ChainUrl ?? settings.ChainUrl

    const chainId = parseInt(projectChainIdRaw) || ACTIVE_CHAIN.chainId
    if (chainId === 31337 && !isLoopbackHttpUrl(projectUrl)) {
        console.warn('[Chain Config] refusing non-loopback RPC for local Hardhat project chain')
        return ACTIVE_CHAIN.chainId
    }
    const chainIdHex = '0x' + chainId.toString(16)
    const isTestnet = chainId === 97

    ACTIVE_CHAIN.chainId = chainId
    ACTIVE_CHAIN.chainIdHex = chainIdHex
    ACTIVE_CHAIN.name = isTestnet ? 'BNB Smart Chain Testnet' : 'BNB Smart Chain'
    ACTIVE_CHAIN.rpcUrl = projectUrl || ACTIVE_CHAIN.rpcUrl
    ACTIVE_CHAIN.explorerUrl = EXPLORER_MAP[chainId] || ACTIVE_CHAIN.explorerUrl
    ACTIVE_CHAIN.currency = isTestnet
        ? { name: 'tBNB', symbol: 'tBNB', decimals: 18 }
        : { name: 'BNB', symbol: 'BNB', decimals: 18 }

    if (projectContract) ADDRESSES.JoinCare = projectContract
    if (settings.JccAddress) ADDRESSES.JCC = settings.JccAddress

    // ============ 任務鏈（CheckIn）：CHECKIN_CHAIN + ADDRESSES.CheckIn ============
    // 雙鏈拆分：CheckIn 地址只屬於任務鏈。後端下發 TaskChain.ChainID 時，
    // 地址僅在該鏈受信（構建期 CheckIn 鏈或顯式聲明的項目鏈）才落地 ——
    // 絕不把任務鏈地址配到項目鏈上（舊扁平字段的核心混淆點）。
    const task = settings.TaskChain || null
    const taskCheckinAddress = task?.CheckinAddress ?? settings.CheckinAddress
    const taskChainId = task ? normalizeChainId(task.ChainID) : null

    if (taskChainId != null) {
        if (isChainTrusted(LOGIN_NETWORK_POLICY, taskChainId)) {
            CHECKIN_CHAIN.chainId = taskChainId
            CHECKIN_CHAIN.name = CHAIN_NAME_MAP[taskChainId] || CHECKIN_CHAIN.name
            CHECKIN_CHAIN.currency = CURRENCY_MAP[taskChainId] || CHECKIN_CHAIN.currency
            // 主網構建 rpcUrl 故意為空（走錢包內置鏈）；僅填充為空的 rpcUrl，
            // 絕不用後端 RPC 覆蓋已注入的顯式構建配置。
            if (!CHECKIN_CHAIN.rpcUrl && task.ChainUrl) CHECKIN_CHAIN.rpcUrl = task.ChainUrl
            if (taskCheckinAddress) ADDRESSES.CheckIn = taskCheckinAddress
        } else {
            console.warn(
                `[Chain Config] backend task chain ${taskChainId} is not trusted by this build; keeping build-time CHECKIN_CHAIN (chain ${CHECKIN_CHAIN.chainId})`
            )
        }
    } else if (taskCheckinAddress) {
        // 舊後端（無 TaskChain 結構）：維持歷史行為直接落地地址。
        ADDRESSES.CheckIn = taskCheckinAddress
    }

    // 緩存
    platformConfig.chainEnvironment = settings.ChainEnvironment || ''
    platformConfig.chainId = projectChainIdRaw || ''
    platformConfig.chainUrl = projectUrl || ''
    platformConfig.platformFee = settings.PlatformFee || ''

    console.log(
        `%c[Chain Config] Env: ${settings.ChainEnvironment}, ProjectChain: ${chainId}, JoinCare: ${ADDRESSES.JoinCare}, JCC: ${ADDRESSES.JCC}, TaskChain/CheckIn: ${CHECKIN_CHAIN.chainId} ${ADDRESSES.CheckIn}`,
        'color: #10B981; font-weight: bold'
    )

    return chainId
}

// ============ ABI 定義 ============

export const ERC20_ABI = [
    'function balanceOf(address owner) view returns (uint256)',
    'function decimals() view returns (uint8)',
    'function symbol() view returns (string)',
    'function name() view returns (string)',
    'function allowance(address owner, address spender) view returns (uint256)',
    'function approve(address spender, uint256 amount) returns (bool)',
    'event Approval(address indexed owner, address indexed spender, uint256 value)'
]

export const JOINCARE_ABI = [
    // 查詢
    'function getProject(uint256 projectId) view returns (tuple(uint256 projectId, address projectToken, address projectOwner, uint256 tokenPrice, uint256 hardCap, uint256 softCap, uint256 maxPerUser, uint256 startTime, uint256 endTime, uint256[] vestingTimes, uint256[] vestingPercents, uint256 vestingDuration, uint8 status, uint256 totalRaised, uint256 totalTokensSold, uint256 totalTokensDeposited, bool fundsWithdrawn, bool projectTokensWithdrawn))',
    'function getUserPurchase(uint256 projectId, address user) view returns (tuple(uint256 totalJCCSpent, uint256 totalTokensBought, uint256 claimedTokens, bool refunded))',
    'function calculateClaimable(uint256 projectId, address user) view returns (uint256)',
    'function getParticipantCount(uint256 projectId) view returns (uint256)',
    // C端寫入
    'function purchase(uint256 projectId, uint256 jccAmount)',
    'function claim(uint256 projectId)',
    'function refund(uint256 projectId)',
    // B端寫入
    'function depositProjectTokens(uint256 projectId, uint256 amount)',
    'function withdrawRaisedFunds(uint256 projectId)',
    'function withdrawProjectTokens(uint256 projectId)',
    // 事件
    'event Purchase(uint256 indexed projectId, address indexed user, uint256 jccAmount, uint256 tokenAmount)',
    'event Claimed(uint256 indexed projectId, address indexed user, uint256 amount)',
    'event Refunded(uint256 indexed projectId, address indexed user, uint256 jccAmount)',
    'event ProjectTokensDeposited(uint256 indexed projectId, uint256 amount)',
    'event RaisedFundsWithdrawn(uint256 indexed projectId, address indexed projectOwner, uint256 amount, uint256 platformFee)',
    'event ProjectTokensWithdrawn(uint256 indexed projectId, address indexed projectOwner, uint256 amount)'
]

// Fresh-proxy stablecoin IDO. Keep this separate from JOINCARE_ABI: tuple layouts
// and event names are intentionally incompatible with the legacy JCC contract.
export const JOINCARE_V2_ABI = [
    'function getProject(uint256 projectId) view returns (tuple(uint256 projectId,address projectToken,address paymentToken,address projectOwner,address feeReceiver,uint8 projectTokenDecimals,uint8 paymentTokenDecimals,uint16 successFeeRate,uint8 status,uint256 tokenPrice,uint256 hardCap,uint256 softCap,uint256 maxPerUser,uint256 startTime,uint256 endTime,uint256 totalRaised,uint256 totalTokensSold,uint256 totalTokensDeposited,bool fundsWithdrawn,bool platformFeeWithdrawn,bool projectTokensWithdrawn,uint256[] vestingTimes,uint256[] vestingPercents))',
    'function getUserPurchase(uint256 projectId,address user) view returns (tuple(uint256 paymentAmount,uint256 tokensBought,uint256 claimedTokens,bool refunded))',
    'function calculateClaimable(uint256 projectId,address user) view returns (uint256)',
    'function purchase(uint256 projectId,uint256 paymentAmount)',
    'function claim(uint256 projectId)',
    'function refund(uint256 projectId)',
    'function depositProjectTokens(uint256 projectId,uint256 amount)',
    'function withdrawRaisedFunds(uint256 projectId)',
    'function withdrawProjectTokens(uint256 projectId)'
]

export const CONTRACT_VERSION_V2 = 'v2_stablecoin'

// Fair Launch intentionally has no address here. Backend supplies and attests
// the project-bound runtime identity before the frontend enables any write.
export { FAIR_LAUNCH_V1_ABI, FAIR_LAUNCH_STATUS } from './fairLaunchAbi.js'

// ============ 常量 ============

export const MAX_UINT256 = '0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff'
