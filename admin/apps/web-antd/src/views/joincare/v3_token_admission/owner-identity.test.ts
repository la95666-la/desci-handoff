import { describe, expect, it } from 'vitest';

import { validObservedOwner } from './owner-identity';
describe('observed V3 owner identity', () => {
  const proxy = '0x0D4040875742975269c55E16C8884d3Ee9510B8E';
  it('accepts a migrated owner without a deployment address allowlist', () => {
    expect(
      validObservedOwner('0x1111111111111111111111111111111111111111', proxy),
    ).toBe(true);
    expect(
      validObservedOwner('0x79c527B8Af5b1E27d5052f8dCe04D20733e72350', proxy),
    ).toBe(true);
  });
  it('rejects zero, malformed and proxy identities', () => {
    for (const value of [
      null,
      '',
      '0x1234',
      `0x${'0'.repeat(40)}`,
      proxy,
      proxy.toLowerCase(),
    ]) {
      expect(validObservedOwner(value, proxy)).toBe(false);
    }
  });
});
