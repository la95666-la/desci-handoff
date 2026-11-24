// Display/entry-point scoping only. ContractService retains the original-claim
// fence and serializes wallet sends independently; this never clears a record.
export function v3ClaimPending(entries) {
  return entries.filter(entry => entry.action === 'claim(uint256)')
}
