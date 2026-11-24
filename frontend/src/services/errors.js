/**
 * 錢包 / 鏈相關錯誤碼 —— 讓調用方能明確區分
 * 「網絡不匹配」（NETWORK_MISMATCH）與「RPC 失敗」（RPC_ERROR）等場景，
 * 而不是對所有錯誤一視同仁地提示「網絡錯誤」。
 */

export const WALLET_ERROR_CODES = {
    /** 錢包所在鏈與期望鏈不一致（fail-closed 阻止在錯誤鏈上簽名/寫交易） */
    NETWORK_MISMATCH: 'NETWORK_MISMATCH',
    /** 鏈上 RPC 請求失敗（節點不可達 / 限流 / 返回異常） */
    RPC_ERROR: 'RPC_ERROR',
    /** 無法獲得可用的 signer（錢包未連接 / 沒有已授權賬戶） */
    SIGNER_UNAVAILABLE: 'SIGNER_UNAVAILABLE',
    /** 錢包請求可能已提交，但未返回可驗證的標準交易哈希；禁止自動重發 */
    TRANSACTION_BROADCAST_UNCERTAIN: 'TRANSACTION_BROADCAST_UNCERTAIN',
    /** 已取得標準交易哈希，但在前端等待窗口內未查到回執 */
    TRANSACTION_RECEIPT_TIMEOUT: 'TRANSACTION_RECEIPT_TIMEOUT',
    /** 已取得回執，但鏈上執行狀態為失敗 / revert */
    TRANSACTION_REVERTED: 'TRANSACTION_REVERTED',
}

/** 判斷某個錯誤是否為「網絡不匹配」類型 */
export function isNetworkMismatchError(err) {
    return err?.code === WALLET_ERROR_CODES.NETWORK_MISMATCH
}

/** 判斷某個錯誤是否為「RPC 失敗」類型 */
export function isRpcError(err) {
    return err?.code === WALLET_ERROR_CODES.RPC_ERROR
}
