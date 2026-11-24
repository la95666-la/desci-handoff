// Backend supplies the block-verified on-chain owner. No deployment-era address
// allowlist: Safe proposal/readback must still match this exact observed owner.
export function validObservedOwner(
  owner: unknown,
  proxy: string,
): owner is string {
  return (
    typeof owner === 'string' &&
    /^0x[\da-fA-F]{40}$/.test(owner) &&
    !/^0x0{40}$/i.test(owner) &&
    owner.toLowerCase() !== proxy.toLowerCase()
  );
}
