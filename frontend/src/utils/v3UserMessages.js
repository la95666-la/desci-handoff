// Error codes remain available for support, but the primary copy explains
// what the visitor can do without asserting why a remote check failed.
const messages = {
  IDO_V3_WALLET_REJECTED: {
    zh: '您已在錢包取消本次操作，未提交交易。需要時可重新發起。',
    en: 'You cancelled this request in your wallet. No transaction was submitted.',
  },
  IDO_V3_WALLET_UNSUPPORTED: {
    zh: '目前錢包連線不支援此請求。請重新連接錢包，或使用錢包內建瀏覽器。',
    en: 'This wallet connection does not support the request. Reconnect or use your wallet’s built-in browser.',
  },
  IDO_V3_BROWSER_UNSUPPORTED: {
    zh: '此瀏覽器缺少安全交易所需功能。請更新錢包瀏覽器或改用最新版 Chrome / Safari。',
    en: 'This browser lacks required safe-transaction features. Update the wallet browser or use current Chrome / Safari.',
  },
  IDO_V3_AMOUNT_INVALID: {
    zh: '操作金額無效，或已不符合目前可用額度。請核對金額並更新資料後重試。',
    en: 'The amount is invalid or no longer fits the available limit. Check the amount and refresh the data.',
  },
  IDO_V3_ALLOWANCE_INSUFFICIENT: {
    zh: '支付幣授權額度不足，已返回授權步驟。請先完成授權，確認成功後再認購。',
    en: 'Payment token allowance is insufficient. Return to approval, confirm it succeeds, then purchase.',
  },
  IDO_V3_BALANCE_INSUFFICIENT: {
    zh: '支付幣餘額不足，請確認目前錢包的餘額並調整認購金額。',
    en: 'Payment token balance is insufficient. Check this wallet’s balance and adjust the purchase amount.',
  },
  IDO_V3_USER_LIMIT_EXCEEDED: {
    zh: '認購金額超過此錢包的剩餘額度，請更新資料並減少金額。',
    en: 'The purchase exceeds this wallet’s remaining limit. Refresh the data and reduce the amount.',
  },
  IDO_V3_HARD_CAP_EXCEEDED: {
    zh: '認購金額超過項目剩餘募資額度，請更新資料並減少金額。',
    en: 'The purchase exceeds the project’s remaining cap. Refresh the data and reduce the amount.',
  },
  IDO_V3_ROUTING_UNAVAILABLE: {
    zh: '目前無法安全開啟這個項目的錢包操作。請稍後重試；若問題持續，請聯絡客服。',
    en: 'Wallet actions for this project are unavailable right now. Try again later or contact support.',
  },
  IDO_V3_IDENTITY_UNAVAILABLE: {
    zh: '目前無法確認這個項目的資料，暫時不能操作錢包。請稍後重新整理；若問題持續，請聯絡客服。',
    en: 'We cannot confirm this project’s details right now, so wallet actions are paused. Refresh later or contact support.',
  },
  IDO_V3_STATE_UNAVAILABLE: {
    zh: '暫時無法取得項目的最新狀態。請稍後重新整理；若您已提交交易，先確認結果，請勿重複提交。',
    en: 'We cannot load the latest project status. Refresh later. If you already submitted a transaction, check its result before trying again.',
  },
  IDO_V3_WALLET_CHANGED: {
    zh: '請連接錢包並登入；如果剛切換錢包，請重新確認後再操作。',
    en: 'Connect your wallet and sign in. If you switched wallets, check the selected wallet before continuing.',
  },
  IDO_V3_PROOF_UNAVAILABLE: {
    zh: '目前無法確認這個錢包的認購資格。請確認使用報名時的錢包並稍後重試；報名本身不保證能認購。',
    en: 'We cannot confirm this wallet’s eligibility yet. Check that you are using the intended wallet and try again later. Signing up alone does not guarantee access.',
  },
  IDO_V3_SUBMISSION_UNRESOLVED: {
    zh: '上一筆交易的結果尚未確認。請先在錢包或交易紀錄中查看，切勿再次提交。',
    en: 'The result of your previous transaction is not confirmed yet. Check your wallet or transaction history; do not submit it again.',
  },
}

export function v3UserMessage(code, isZh = false) {
  const fallback = messages.IDO_V3_STATE_UNAVAILABLE
  return (messages[code] || fallback)[isZh ? 'zh' : 'en']
}
