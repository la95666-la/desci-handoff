/**,
 * depositMath.js — BigInt / decimals 安全計算（Dashboard 項目方 Pending 卡片）
 *
 * 純函數、無 Vue / DOM / import.meta 依賴，可在 node --test 下直接測試。
 *
 * 公式說明
 *   softCap / hardCap（付款幣）轉成項目代幣需求量：
 *     requiredTokens = capPaymentRaw * 10^tokenDecimals / tokenPriceRaw
 *   其中 tokenPriceRaw 為「單價：付款幣 / 整顆代幣」的 raw 值。
 *   shortfall = max(hardCapRequiredRaw - 已充值 raw, 0)
 */

import { formatUnits } from 'ethers'

/** 把任意形態的 raw 值安全轉成 BigInt；非法輸入回 0n（防止後端字段污染崩潰） */
function toBigInt(v) {
  if (typeof v === 'bigint') return v
  if (v == null) return 0n
  if (typeof v === 'number') return Number.isFinite(v) ? BigInt(Math.trunc(v)) : 0n
  const s = String(v).trim()
  if (!s) return 0n
  try { return BigInt(s) } catch { return 0n }
}

/** 由 decimals 生成 10^n（BigInt），decimals 限制在 0..27 */
export function scaleDecimals(decimals) {
  const d = Number(decimals)
  const safe = Number.isFinite(d) ? Math.max(0, Math.min(Math.trunc(d), 27)) : 18
  return 10n ** BigInt(safe)
}

/** 付款幣目標金額（raw）→ 需要的項目代幣數量（raw）；tokenPrice<=0 或 cap<=0 時回 0n */
export function tokensRequiredForCap(capPaymentRaw, tokenPriceRaw, tokenDecimals = 18) {
  const cap = toBigInt(capPaymentRaw)
  const price = toBigInt(tokenPriceRaw)
  if (cap <= 0n || price <= 0n) return 0n
  return (cap * scaleDecimals(tokenDecimals)) / price
}

/** softCap / hardCap 代幣需求量 + shortfall 一併計算 */
export function computeSoftAndHardRequirement({ softCapRaw, hardCapRaw, tokenPriceRaw, depositedRaw, tokenDecimals = 18 }) {
  const soft = tokensRequiredForCap(softCapRaw, tokenPriceRaw, tokenDecimals)
  const hard = tokensRequiredForCap(hardCapRaw, tokenPriceRaw, tokenDecimals)
  const deposited = toBigInt(depositedRaw)
  const shortfall = hard > deposited ? hard - deposited : 0n
  return { softCapRequiredRaw: soft, hardCapRequiredRaw: hard, shortfallRaw: shortfall }
}

/** 授權是否足以支付指定數量的代幣 */
export function allowanceSufficient(allowanceRaw, neededRaw) {
  return toBigInt(allowanceRaw) >= toBigInt(neededRaw)
}

/** 餘額是否足以支付指定數量 */
export function balanceSufficient(balanceRaw, neededRaw) {
  return toBigInt(balanceRaw) >= toBigInt(neededRaw)
}

/**
 * raw -> 可讀字串（千分位 + 最多 maxDecimals 位小數，去尾零）。
 * opts.maxDecimals 預設 6；opts.noGrouping 關閉千分位（輸入框用）。
 */
export function formatToken(raw, decimals = 18, opts = {}) {
  const maxDecimals = Number(opts.maxDecimals ?? 6)
  const value = toBigInt(raw)
  const negative = value < 0n
  const abs = negative ? -value : value
  const safeDecimals = Number.isFinite(Number(decimals)) ? Number(decimals) : 18
  let s = formatUnits(abs, safeDecimals)
  if (s.includes('.')) {
    let [int, frac] = s.split('.')
    frac = frac.slice(0, Math.max(0, maxDecimals)).replace(/0+$/, '')
    s = frac ? `${int}.${frac}` : int
  }
  const [intPart, fracPart] = s.split('.')
  const grouped = opts.noGrouping ? intPart : intPart.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  return (negative ? '-' : '') + (fracPart ? `${grouped}.${fracPart}` : grouped)
}

/** 輸入框用：raw -> 全精度字符串（可直接被 parseUnits 還原），去尾零 */
export function toInputAmount(raw, decimals = 18) {
  return formatToken(raw, decimals, { maxDecimals: 24, noGrouping: true })
}