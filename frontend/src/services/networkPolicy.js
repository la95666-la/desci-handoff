/**
 * 網絡策略 —— 登入 / 簽名前允許的鏈，統一從「顯式受信配置」推導。
 *
 * 顯式受信配置（build 時注入，與 config/contracts.js 的 CHECKIN_CHAIN 同一來源）：
 *   - VITE_ENVIRONMENT:             'local' | 'test' | 'prod'
 *   - VITE_CHECKIN_CHAIN_ID:        簽到合約所在鏈（56 / 97 / 31337，hex 或十進制）
 *   - VITE_TRUSTED_PROJECT_CHAINS:  顯式信任的 Launchpad/IDO 項目鏈（逗號分隔，如 '97'）。
 *                                   雙鏈發布形態（主網 CheckIn + 測試網項目鏈）時注入；
 *                                   不注入 = 完全不擴展信任面（fail-closed 默認不變）。
 *
 * 結論規則（fail-closed，與 CHECKIN_CHAIN 的 isMainnetBuild 判定保持一致）：
 *   - 生產構建（prod、或顯式 56、或完全未配置 ⇒ 視為主網）只信任 56，可自動切到 56；
 *     顯式聲明了 trustedProjectChains 時信任集 = [56, ...聲明鏈]，偏好鏈仍是 56
 *     （簽到/登入落地主網；Launchpad 寫交易按項目鏈按需提示切換）
 *   - 本地構建（local + 顯式 31337 / local 默認）信任 56 / 97 / 31337，且強制不自動切鏈
 *     （本地開發跑 Hardhat(31337) 或測試網都可以，絕不把用戶強行拉到 97）
 *   - 測試構建（test / 顯式 97）信任 56 / 97，偏好 97，可自動切鏈
 *
 * 本模塊為純函數，不依賴 import.meta.env / window，可直接在 node --test 中測試。
 */

export const CHAIN_IDS = {
    BSC_MAINNET: 56,
    BSC_TESTNET: 97,
    HARDHAT: 31337,
}

/**
 * 解析逗號分隔的鏈 ID 列表（構建期 VITE_TRUSTED_PROJECT_CHAINS='56,97'），
 * 標準化、去重、丟棄不可解析項。contracts.js 與 reownAppKit.js 共用，
 * 保證兩處消費的信任集完全一致。
 */
export function parseChainIdList(raw) {
    return [...new Set(
        String(raw ?? '')
            .split(',')
            .map(normalizeChainId)
            .filter((v) => v != null)
    )]
}

/**
 * 把各種形態的 chain id 標準化成十進制 number。
 * 支援：number、bigint、'0x38' / '97' / '0x7a69' 等字符串。
 * 解析失敗返回 null（調用方應按「未知 ⇒ 不信任 / fail-closed」處理）。
 */
export function normalizeChainId(raw) {
    if (raw == null) return null
    if (typeof raw === 'number') return Number.isFinite(raw) ? raw : null
    if (typeof raw === 'bigint') return raw <= BigInt(Number.MAX_SAFE_INTEGER) ? Number(raw) : null
    if (typeof raw === 'string') {
        const s = raw.trim()
        if (!s) return null
        const base = s.toLowerCase().startsWith('0x') ? 16 : 10
        const n = Number.parseInt(s, base)
        return Number.isFinite(n) ? n : null
    }
    return null
}

/**
 * 判斷某個 chain id 是否被策略信任（登入/寫交易前守門用）。
 * 這是 walletStore.signAndSubmitAuth / isCorrectNetwork 用的唯一判定入口，
 * 避免散落 56/97 硬編碼。hex 字符串（如 '0x7a69'）也會先標準化再比較。
 */
export function isChainTrusted(policy, chainId) {
    if (!policy?.trustedChainIds) return false
    const parsed = normalizeChainId(chainId)
    if (parsed == null) return false
    return policy.trustedChainIds.includes(parsed)
}

/**
 * 構建登入網絡策略。
 * @param {object} opts
 * @param {string} [opts.environment]  - 'local' | 'test' | 'prod'（空串 = 未配置）
 * @param {number|string} [opts.checkinChainId] - 顯式受信鏈 ID（NaN/undefined = 未配置）
 * @param {Array<number|string>} [opts.trustedProjectChains] - 顯式信任的項目鏈 ID 列表
 *   （來自構建期 VITE_TRUSTED_PROJECT_CHAINS；只擴展主網構建的信任集，不改偏好鏈）
 * @returns {{ trustedChainIds: number[], preferredChainId: number, autoSwitch: boolean }}
 */
export function createLoginNetworkPolicy({ environment = '', checkinChainId, trustedProjectChains } = {}) {
    const env = String(environment || '').trim().toLowerCase()
    const explicit = normalizeChainId(checkinChainId)
    const extraTrusted = [...new Set(
        (Array.isArray(trustedProjectChains) ? trustedProjectChains : [])
            .map(normalizeChainId)
            .filter((v) => v != null)
    )]

    // 與 contracts.js 相同的「顯式測試構建」判定，保持單一事實來源
    const isExplicitTestBuild =
        env === 'test' || env === 'local' ||
        explicit === CHAIN_IDS.BSC_TESTNET || explicit === CHAIN_IDS.HARDHAT
    const isMainnetBuild =
        env === 'prod' || explicit === CHAIN_IDS.BSC_MAINNET || !isExplicitTestBuild

    if (isMainnetBuild) {
        if (extraTrusted.length === 0) {
            return {
                trustedChainIds: [CHAIN_IDS.BSC_MAINNET],
                preferredChainId: CHAIN_IDS.BSC_MAINNET,
                autoSwitch: true,
            }
        }
        // 雙鏈發布形態：主網 CheckIn（簽到/登入落地）+ 顯式聲明的項目鏈（Launchpad 按需切換）。
        // 偏好鏈保持主網：絕不在 settings 加載後把用戶自動拉到項目鏈。
        const trusted = [...new Set([CHAIN_IDS.BSC_MAINNET, ...extraTrusted])]
        return {
            trustedChainIds: trusted,
            preferredChainId: CHAIN_IDS.BSC_MAINNET,
            autoSwitch: true,
        }
    }

    // 本地 / 默認 Hardhat 構建：信任本地測試鏈，不自動切鏈
    if (explicit === CHAIN_IDS.HARDHAT || (env === 'local' && explicit == null)) {
        return {
            trustedChainIds: [CHAIN_IDS.BSC_MAINNET, CHAIN_IDS.BSC_TESTNET, CHAIN_IDS.HARDHAT],
            preferredChainId: CHAIN_IDS.HARDHAT,
            autoSwitch: false,
        }
    }

    // 測試網構建（test 環境或顯式 97）
    return {
        trustedChainIds: [CHAIN_IDS.BSC_MAINNET, CHAIN_IDS.BSC_TESTNET],
        preferredChainId: CHAIN_IDS.BSC_TESTNET,
        autoSwitch: true,
    }
}
