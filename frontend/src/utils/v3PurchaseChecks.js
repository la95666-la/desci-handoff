// Safe error codes carry no provider payloads. Called once using the operation's
// fixed-block values under the V3 submission lock, without recursive preflights.
export function assertV3PurchaseAmounts(amount, allowance, balance, invested, maxPerUser, raised, hardCap) {
  const fail = (code) => { throw Object.assign(new Error(code), { code }) }
  if (amount <= 0n) fail('IDO_V3_AMOUNT_INVALID')
  if (allowance < amount) fail('IDO_V3_ALLOWANCE_INSUFFICIENT')
  if (balance < amount) fail('IDO_V3_BALANCE_INSUFFICIENT')
  if (invested + amount > BigInt(maxPerUser)) fail('IDO_V3_USER_LIMIT_EXCEEDED')
  if (BigInt(raised) + amount > BigInt(hardCap)) fail('IDO_V3_HARD_CAP_EXCEEDED')
}
