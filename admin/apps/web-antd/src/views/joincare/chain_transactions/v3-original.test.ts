import type { ChainTransaction } from '#/api/joincare/chain_transaction';

import { describe, expect, it } from 'vitest';

import { canContinueV3Original } from './v3-original';
const fixture = () =>
  ({
    action: 'ido_v3_end',
    chain_id: 56,
    contract_address: '0x0d4040875742975269c55e16c8884d3ee9510b8e',
    state: 'signed',
    broadcast_count: 0,
    tx_hash: `0x${'a'.repeat(64)}`,
  }) as ChainTransaction;
describe('original first broadcast visibility', () => {
  it('offers only unbroadcast V3 originals', () => {
    expect(canContinueV3Original(fixture())).toBe(true);
  });
  it.each([
    'submitted',
    'broadcast_unknown',
    'applied',
    'final_revert',
  ] as const)('never offers %s', (state) => {
    const r = fixture();
    r.state = state;
    expect(canContinueV3Original(r)).toBe(false);
  });
  it('never offers a second send or V2 identity', () => {
    const r = fixture();
    r.broadcast_count = 1;
    expect(canContinueV3Original(r)).toBe(false);
    r.broadcast_count = 0;
    r.action = 'v2_create';
    expect(canContinueV3Original(r)).toBe(false);
  });
});
