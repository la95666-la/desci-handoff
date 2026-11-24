# V3 consumer explicit preparation

## Scope

Replaces request-local RPC transcript discovery by repeated SQL rollback with
an explicit immutable window plan. No migration/configuration/production write
is needed for the source change. This document does not assert production
throughput, readiness or rollout completion.

## Execution

1. Read the running public stream and exact project cursor. Read and validate
   the complete contiguous coverage/digests/receipts once for up to 128 heights.
2. Check the live project/public anchors. Prepare all state observations and
   policy facts outside writer transactions. Complete-proxy empty stretches
   use boundary identity/policy checks. Event blocks retain full event/receipt
   validation, pinned end-of-block state and explicit policy preparation.
3. Recheck live target/public anchors, then start ONE writer transaction.
   Lock stream then project. Require same epoch/mode/anchor, monotone producer
   progress and exact project cursor/hash/revision. Recheck the immutable
   epoch-scoped coverage boundaries. No network occurs inside publication.
4. Apply the prepared units using existing journal, economic, financial,
   policy and canonical-outbox logic, then publish cursor/consumer metadata.
5. Live postcommit anchors/depth are checked outside the stream lock. Failure
   revokes only the exact published project revision. A failed revocation
   is reported, never acknowledged as healthy.

Eventful plans yield between complete units based on last-unit cost and
remaining budget. Started-unit failures are not converted into successful
yield. A public stream advance is not a CAS conflict; mode/epoch/rewind/halt
or project rewrite is.

## Query reductions

- No speculative writer transaction per missing RPC: preparation makes no
  tentative SQL writes and does not repeat the same prefix after every read.
- Empty checkpoints use one multi-row INSERT (up to 128), retaining unique
  checkpoint collision/atomic rollback behavior.
- Empty projection parity is read-only: no unchanged projection UPSERT then
  SELECT merely to compare it. The complete historical replay is RETAINED to
  reject corrupt projections; its checkpoint validation is one LEFT JOIN
  batch query, not N individual SELECTs. Missing facts are not filtered out.
- Identical coverage ranges can share verified decoded payloads within one
  tick, exact DB/scope/epoch only. Entries are deep-copied and bounded at
  128 ranges / 32 MiB, with 8 MiB per range. Errors and transaction-local reads
  are not cached. Publication still checks current stream and boundaries.

The old transcript consumer remains private as a differential test reference,
not a configurable fallback. It can be removed when no longer needed by tests.

## Evidence

Network-none disposable MySQL tests compare both engines after rollback:
cursor/revision, checkpoints, event journal, financial effects, policy facts,
business status and available outbox state. Fixtures include complete-proxy
empty coverage and deposit/cancellation/policy events in the same block.
SQL trace counts are fixture measurements, not universal production ratios.

Measured in the same disposable fixture (complete SQL trace, including
transaction/session statements):

| Fixture | Legacy transcript SQL | Explicit prepared SQL |
| --- | ---: | ---: |
| 3 empty heights | 1310 | 29 |
| 3 heights with deposit/cancel/policy events | 5355 | 63 |
| 128 empty heights | not used as a new production baseline | 29 |

For the 3-height empty case, public-coverage SELECTs drop from 53 to 4.
The eventful case drops from 117 to 4. The final writer performs no RPC.

The 128-height empty fixture requires ONE checkpoint INSERT and constant
publication SQL count. Two-connection tests prove the producer can commit
during live RPC; epoch mutation and project CAS changes reject the plan;
postcommit depth loss revokes the exact published revision. Corrupt projection,
checkpoint collision, crash rollback, idempotence and retained-epoch tests
remain fail-closed.

## Deliberately not changed yet

Follow-up incident / deferred work: [V3 halt, lock ordering and shared sender
fence](IDO-V3-HALT-SENDER-FENCE-FOLLOWUP.md). Includes 2026-10-27 production
evidence and temporary recovery receipts; not a permanent-fix claim.

Full approved original-create/runtime admission still runs each project tick.
No cross-tick admission cache or durable projection-provenance certificate is
introduced. Eventful projection rebuilds still use the existing audited replay.
Removing those requires separate invalidation/provenance and economic parity
work, not an unaudited TTL shortcut. Coverage sharing benefits exact overlap;
disjoint project ranges do not magically share reads.

V2 RPC routing/timeouts are outside this change. Production acceptance must
measure each project's cursor vs actual confirmed chain head over time, RPC
traffic/429/timeouts, lock waits, transaction duration and business parity.
Container healthy and recent successful rounds are not catch-up acceptance.
