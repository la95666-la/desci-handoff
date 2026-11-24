// Display policy only; not a Set parameter or a transaction permission.
export function supportsMinPurchasePolicy(mode: string, version: string) {
  return (
    mode === 'capped_presale' &&
    ['ido_v3_stablecoin', 'v2_stablecoin'].includes(version)
  );
}
