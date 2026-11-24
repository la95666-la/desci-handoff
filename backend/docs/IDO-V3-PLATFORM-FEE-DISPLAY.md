# Admin V3 platform-fee button status

V3 fee events are materialized in `ido_v3_event_journal` and
`ido_v3_financial_effect` / `ido_v3_project_projection`, not the legacy
`projects.platform_fee_withdrawn` flag. The Admin list previously exposed that
legacy zero, allowing a repeat click after submission74 had already applied.

The list now batches a read for the page's locked exact V3 identities and
returns `platform_fee_withdrawn=1` only when a canonical `PlatformFeeWithdrawn`
event and matching payment effect agree on project/chain/proxy/version,
receipt block/hash/tx/log, token, snapshot fee recipient and projected amount.
The cursor/projection revision/hash and canonical checkpoints must agree.
Orphaned facts or halted/inconsistent projections do not become payout facts.
A failed query rejects the list instead of falling back to a stale flag.

The event, not `amount > 0`, establishes the action was executed: zero-value
fee withdrawals are included. No new submission, chain call or financial write
is involved. V2 list semantics remain unchanged, and the legacy database flag
is not backfilled. This read is display-only and cannot authorize withdrawal,
Finished, archival, or certify general project freshness.

Existing Admin `isPlatformFeeWithdrawable` and conditional rendering hide the
button once this field is 1, including after refreshing the list. No UI runtime
change or new migration is necessary. The Admin companion commit only adds a
regression assertion for the existing hide behavior.

Validation includes list wiring with a stale zero legacy flag, V2 compatibility,
zero-amount execution, query failure, long project IDs, and actual MySQL8.4
query tests for orphan/reorg/cursor mismatch/recipient/amount/cross-project
boundaries. `scripts/test-v3-fee-status-mysql.sh` uses network-none disposable
MySQL and refuses production DSNs.

Source-only fix: no production DB writes, rollout, signing or broadcast.
