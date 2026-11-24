# Active-original retirement evidence refresh

Bounded operational recovery for signed, never-broadcast originals blocked by
expired retired-test sender exemptions. No application component deployment,
SQL migration, signing key or transaction send is included in this command.

`cmd/ido-v3-refresh-fee74-retirement` is fixed to submission74/hash/retired set
41,43,44,45,48,49. Dry-run is default; `--execute` appends fresh independently
audited evidence only. It never enrolls a new retired record or revives revoked
records; latest enrollment must remain valid, although expired. It does not
extend or edit prior evidence and never touches outbox/project/nonce rows.

Two read-only RPCs verify each retired original's exact bytes, consumed nonce,
canonical status1 receipt/logs and full project journal match. The active
original must independently pass the exact nonce58/hash-absence/balance/gas
simulation guard on both RPCs. RPC audits occur before locking SQL. Audits must
complete within a two-minute bound before timestamping the five-minute evidence.

The commit transaction locks the global incident gate and running active
cursor, rechecks exact active original bytes/state/nonce and the complete
sender retired set, locks enrolled evidence and retired project identity, then
locks the sender fence and requires active_submission_id=74,next_nonce=59.
Any revocation, changed retired set, receipt/journal or active marker race
aborts the entire evidence batch. Dry-run rolls back. Evidence expiry alone
is not authority: successful independent re-audit is mandatory. The normal
Admin preflight/broadcast must be rerun after refresh and still commits its
one-shot broadcast marker before its single send. Failures never authorize
resigning, changing nonce, clearing a marker or manual raw transaction send.

Validation: synthetic guards, race/vet, disposable network-none MySQL8.4 tests
with an expired enrolled predecessor and separate signed active nonce; dry-run
no writes, revoked evidence, nonce drift and already-marked active original
are rejected; positive refresh adds one evidence row and leaves original/fence
unchanged. Production execution requires verified identity, private backups,
explicit original recovery authorization and independent readback.
