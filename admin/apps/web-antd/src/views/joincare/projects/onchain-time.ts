import type { Dayjs } from 'dayjs';

// V3's immutable calldata uses Unix seconds and the projects table stores
// DATETIME(0). Format the exact same seconds for preview and approval submit.
// Keep the existing V2 request format unchanged.
export function formatOnchainTime(
  value: Dayjs | undefined,
  wholeSeconds: boolean,
): string {
  return value
    ? (wholeSeconds ? value.millisecond(0) : value).toISOString()
    : '';
}
