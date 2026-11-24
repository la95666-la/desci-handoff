# V3 Job: transient observations are not persistent project halts

## Scope

This patch changes the approved stream Job's failure handling, not the sender
nonce admission rules, contract state, proof policy or automatic reorg recovery.
No migration/configuration switch. Deployment is separately authorized.

## Classification and scheduling

- Recognized typed source timeouts/cancellation, EOF/connection reset/refused,
  HTTP 429/500/502/503/504 are deferred. Error strings are not classifiers.
  Provider JSON-RPC codes are NOT universally treated as retryable (codes can
  represent different failures). Unknown/malformed/identity failures remain
  conservatively closed. Explicit evidence conflicts always take precedence.
- Typed MySQL deadlock/lock timeout means discard the rolled-back work, not
  orphan the chain. A logger wrapper preserves the type through redacting
  journal helpers, without parsing SQL text.
- Stream/project CAS changes reject the prepared writer. Another writer's
  advance or recovery is not evidence that historical chain facts are corrupt.
- Deferred project attempts remain failed/incomplete for full-tick accounting.
  They never call FinishApprovedV3Scan, do not advance global success time, delete
  projections or suspend applied outbox. Other projects are still dispatched.
- Project-local scheduling backoff is 2/4/8/16/30 seconds plus <1 second stable
  jitter, no in-turn sleep. It clears on success and grants no authority. It is
  in-memory scheduling only; restart cannot manufacture a completed scan.
- Producer transient failure preserves immutable committed units and fences
  global completion by setting scan health checking and incrementing its
  generation. An overlapping older consumer tick cannot publish healthy over
  that failure. A later independently complete scan may finish its own generation.

The existing per-consumer `last_success_at` is still written in the verified
publication transaction before the postcommit check. It is NOT newly treated
as a live-freshness certificate by this patch. Postcommit failure closes global
completion; durable per-project degraded state remains separate work.

## Publication and safety

The prepared consumer runs RPC before the writer. Precommit source failures
leave the window uncommitted. SQL commit outcome uncertainty is not blindly
retried inside the same writer: next attempt re-reads cursor/CAS and existing
events. Generic/unknown write errors still close conservatively.

After a complete publication, a recognized transient final anchor read returns
incomplete without destroying the already checked prefix. A valid different
hash, insufficient confirmations, corrupt projection/coverage, wrong runtime,
or unclassified error retains the existing halt/recovery behavior. Retained
projection data is HISTORICAL, not proof of current freshness or authority.

Prepared stream canonical checks no longer invoke the legacy side-effecting
cursor halt helper. Stream-mode single-project revoke and postcommit revoke
acquire the public stream lock BEFORE project locks, matching global halt and
publication. Legacy non-stream scanners are not silently converted to this
policy. Their behavior must be audited separately before broadening.

No new send/retry signing/broadcast/nonce relaxation. Current reads and each
mutation's independent chain/runtime/approval/nonce guards still apply.
DB-only membership proofs retain their own semantics and are not certified
current-chain permissions by a running scanner or this patch.

## Tests / limits

Unit coverage: typed classifications vs text-only errors, explicit conflict
precedence, cancellation/project deadlines without revocation, partial-tick
failure, isolation of other projects, bounded backoff and cleanup lock order.
Network-none MySQL fixture: producer source timeout retains prefix; consumer
precommit timeout leaves cursor unchanged; postcommit timeout retains the exact
verified window/projection; real depth loss still revokes; financial/policy
parity, CAS/epoch rejection, crash rollback and idempotence remain tested.

Two disposable MySQL connections also race global halt against ordered
single-project revoke and require both transactions to complete. This exercises
the real cursor trigger stream read, not only mocked SQL ordering assertions.

Remaining work: durable per-project degraded/stale/error reporting and API/UI
labels, independent nonce-consumed certificates for genuinely suspended
non-retired attempts, legacy scanner paths, all trigger/DB lock interactions,
and commit-uncertainty fault injection. This is not a claim that every possible
transient provider error or every deadlock path has been eliminated. Existing
freshness gates remain, but a retained historical projection must never be
presented as fresh merely because its business status remains Pending/Active.

Production recovery performed separately under operator authorization uses
original-hash receipts and full coverage audit, preserves epoch and nonce, and
does not mean this new Job source is deployed. Final acceptance requires actual
confirmed-head/project-cursor time series and fault injection, not container
healthy alone. See IDO-V3-HALT-SENDER-FENCE-FOLLOWUP.md for incident history.
