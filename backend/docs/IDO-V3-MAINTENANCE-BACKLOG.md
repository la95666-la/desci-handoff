# V3 maintenance is a scheduling choice, not closure

Source-only Backend + Job change, based on 27aeb9b (includes whitelist review
optimization). No migration, production configuration/mode change or chain send.

## Admission

A locked terminal V3 project may enter maintenance with stale/missing Finished
evidence and historical consumer backlog. There is no 120-block/fresh certificate
gate for this cadence change. Archive retains its strict healthy-source proximity,
independently Finished/current evidence and no-pending-work conditions.

Maintenance still requires live execute permission, reason, revision CAS, running
stream/cursor, no active reorg, no unresolved signed/broadcast/nonce work and no
source dead letter. Unclaimed/refundable financial obligations are NOT the same
as active transaction reconciliation. Pending/Active lifecycle cannot be deferred.

Only mode, next scan time, existing wake hint and tracking revision are touched by
the audited action. Consumer/checkpoints/projections/financial facts and original
closure evidence remain unchanged. An existing wake is acknowledged by scheduling
the backlog for 12h, not deleted. Repeating maintenance is rejected rather than
sliding the due time indefinitely. Explicit sync advances a deferred wake to NOW.

## Scheduling and preservation

- A not-due maintenance row may have stale/no closure evidence and a historical
  wake. It is genuinely excluded from ordinary rounds.
- New project/governance events still write durable wake + next_scan_at=NOW in
  the public coverage transaction; they can cause earlier processing than 12h.
- Missing/halted source/cursor, known epoch mismatch, unresolved transactions,
  dead letters and Pending/Active lifecycle override deferral.
- Once due, partial batches and errors leave it due. Continue from the ORIGINAL
  cursor through every covered block until caught up; do not schedule another
  12h after each 512-block batch. The existing fully-caught-up closure transaction
  schedules the next 12h. Eligibility at 12h is not a promise to drain arbitrarily
  large backlog instantly; throughput, bounded fairness and RPC availability apply.
- Finished is never inferred from cadence. Consumer freshness thresholds, archive
  gates, proof authority, approval/Safe/nonce fences and financial raw values are
  unchanged. Stale Finished remains unavailable through public API masking.

Backend and Job must both be upgraded independently before using this behavior.
An older Job will safely over-scan stale maintenance rather than lose data, but
will not deliver the expected load reduction. Existing API input and Admin buttons
are compatible. No database migration or manual evidence/backfill is needed.

Tests: stale/missing evidence admission, preservation of wake and exact due-time
SQL, revoked admin/revision/live transaction refusal, archive strictness, repeated
maintenance refusal, explicit sync, due backlog selection, epoch/missing cursor
override and real network-none MySQL financial/closure/event-wake regressions.
