import { formatDecimalString } from './fairLaunch.js'

// Presentation only. Never feed this rounded value into amounts or transactions.
export function formatRaisedAmount(value, token) {
  if (value === null || value === undefined || value === '') return '—'
  if (String(token).toUpperCase() !== 'USDT') return formatDecimalString(value, 6)
  const match = /^(0|[1-9][0-9]*)(?:\.([0-9]+))?$/.exec(String(value))
  if (!match) return '—'
  const fraction = (match[2] || '').padEnd(4, '0')
  let milli = BigInt(match[1]) * 1000n + BigInt(fraction.slice(0, 3))
  if (fraction[3] >= '5') milli += 1n
  const whole = (milli / 1000n).toString().replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  return `${whole}.${(milli % 1000n).toString().padStart(3, '0')}`
}
