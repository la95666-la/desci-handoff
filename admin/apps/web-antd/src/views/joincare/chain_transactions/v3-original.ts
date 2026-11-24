import type { ChainTransaction } from '#/api/joincare/chain_transaction';

// UI visibility only; backend repeats all identity, permission, chain and
// nonce guards. A hash or signed state alone is never send authority.
export function canContinueV3Original(row: ChainTransaction) {
  return (
    row.chain_id === 56 &&
    row.contract_address.toLowerCase() ===
      '0x0d4040875742975269c55e16c8884d3ee9510b8e' &&
    [
      'ido_v3_activate',
      'ido_v3_create',
      'ido_v3_end',
      'ido_v3_withdraw_platform_fee',
    ].includes(row.action) &&
    row.state === 'signed' &&
    row.broadcast_count === 0 &&
    !row.last_broadcast_at &&
    !row.receipt_status &&
    !row.receipt_block_number &&
    !!row.tx_hash &&
    /^0x[\da-f]{64}$/i.test(row.tx_hash)
  );
}
