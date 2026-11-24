import { describe, expect, it } from 'vitest';

import { supportsMinPurchasePolicy } from '../min-purchase-policy';

describe('official-page minimum purchase policy', () => {
  it('supports exact V2/V3 only, not mixed or Fair Launch identities', () => {
    expect(supportsMinPurchasePolicy('capped_presale', 'v2_stablecoin')).toBe(
      true,
    );
    expect(
      supportsMinPurchasePolicy('capped_presale', 'ido_v3_stablecoin'),
    ).toBe(true);
    for (const [mode, version] of [
      ['fair_launch', 'ido_v3_stablecoin'],
      ['fair_launch', 'v2_stablecoin'],
      ['capped_presale', 'fair_launch_v1'],
      ['', 'ido_v3_stablecoin'],
      ['capped_presale', 'unassigned'],
    ]) {
      expect(supportsMinPurchasePolicy(mode || '', version || '')).toBe(false);
    }
  });
});
