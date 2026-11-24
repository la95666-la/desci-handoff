# V3 cursor/checkpoint current-read fix

## Failure

Policy preview/review can fail at `cursor_transaction` despite an applied
creation and a healthy, advancing Job. Under MySQL REPEATABLE READ the earlier
incident COUNT establishes a consistent-read snapshot. The cursor's `FOR UPDATE`
sees a newer committed scanner tip, but an ordinary checkpoint SELECT still
uses the earlier snapshot and falsely reports that exact checkpoint missing.
This is not proof of a chain identity failure, absent checkpoint or reorg.

## Change

`LockV3RunningCursorTx` now selects the exact canonical checkpoint using
`FOR SHARE`, a current locking read like the preceding cursor `FOR UPDATE`.
The same chain/Proxy/project/version/height/hash/status predicates remain.
Cursor -> checkpoint lock order is unchanged and the shared checkpoint lock
lasts through commit. Orphaning/removal cannot invalidate the selected fact
while that transaction is authorizing a mutation. No broad isolation change,
retry, approval bypass, RPC caching, health edit or Job pause is introduced.

The shared gate covers policy preview/review and lifecycle signers. This does
not relax the independent approval, original receipt, runtime, chain/finality,
policy/Safe simulation or proof gates. Other independent failures still close.

## Regression

`scripts/test-v3-cursor-snapshot-mysql.sh` runs the existing network-none full
schema on MySQL 8.4 and the new two-physical-connection test. Explicit RR reader
snapshot is established before a scanner transaction atomically inserts height
91's checkpoint and advances the cursor from 90. The old snapshot demonstrably
sees zero matching checkpoints; the fixed gate accepts the current canonical
pair. Missing/orphaned/mismatched checkpoints and halted cursors still reject.
Coordination is deterministic without sleeps or public/production RPC.

Backend-only release; no database migration/config update required. Local
regression is not a successful production approval/Safe/proof observation.
