# V3 stream scanner periodic creation-history audit

## Boundary

This optimization is confined to the process-owned approved V3 **stream
scanner**. Bootstrap, Set creation, recovery, shadow/legacy scanning, Core
lifecycle/signing/broadcast, policy review and wallet proof entrypoints retain
their existing complete admission paths. No migration, new RPC endpoint,
concurrency increase or configuration secret is introduced. The global full
scan generation and 90-second proof freshness rules are unchanged.

The optimized data is repeated original-creation audit work, NOT project
policy, financial totals, canonical coverage, runtime trust or sender fences.
The scanner never signs, broadcasts, repairs a nonce or marks an attempt applied
because a cache entry exists. Canonical replay is still the only application
path for original successful lifecycle receipts.

## Lifetime and invalidation

- Each process starts cold and fully audits a locked project before reuse.
- Baseline validity is at most ten minutes, beginning at full audit START.
  Stable per-project staggering chooses 8..10 minutes; a new round refreshes
  early when less than its 50-second reserve remains. Hits never renew expiry.
- Scope includes process DB owner, RPC endpoint digest, proxy, independently
  configured owner/governor code trust hashes, stream epoch and anchor.
- Every turn rereads the executed Set/approval, creation journal/checkpoints,
  running cursor, exact original applied attempt and signed-evidence digest.
  The snapshot includes original calldata/raw bytes/signing timestamp/finality
  fields without retaining/logging signed bytes in the cache.
- Changed approval/identity/original attempt, changed epoch/anchor/source,
  halted/nonmonotone/revision-only cursor, failed live verification or expiry
  removes the baseline. A new successful full audit is required; no stale
  fallback. Removing/archiving a project prunes its entry. Leaving stream mode
  clears entries. A bounded 1024-entry map fails deferred at capacity.
- Recognized source failures remain deferred and never release transaction
  fences. A consumer budget yield does not extend the baseline; it may retain
  unexpired historical evidence because it is not proof of corrupted history.

## Checks retained on each reuse

1. Fresh DB approval and original evidence equality, plus contiguous cursor
   progress (block delta equals revision delta; same-height hash fixed).
2. Canonical creation/cursor headers and confirmation depth.
3. Current confirmed deployment, owner/governor/operator and independently
   pinned code identities. No deployment identity is cached across turns.
4. Current confirmed project identity/immutable economics and stable headers.
5. Final DB evidence reread and stream control check after RPC.

Creation receipt decoding/repeat retrieval, creation-block economics and
original signature replay audit are performed on the full path, not on normal
reuse. These historical facts are reusable only while their above bindings
and live creation anchor remain valid. The provider's current deployment and
project observations remain a nontrivial cost; this is NOT a promise of zero
RPC or a guaranteed production catch-up rate.

## Consumer commit fence

Only a private scanner context can carry a bounded audit grant. Its deadline
cannot exceed audit expiry. Before preparation and again **under the existing
stream->project transaction locks**, the consumer checks epoch/anchor, exact
binding, fresh executed approval/original-attempt digest and monotone cursor.
Existing checkpoint, full coverage, economic replay, CAS, postcommit canonical
checks and atomic journal/projection/outbox/cursor writes remain unchanged.
An audit expiry or rejected grant cannot be converted into a completed round.

The ten-minute history lifetime is separate from progress freshness. It does
not extend the 90-second full-scan proof window or allow partially processed
projects to publish full-round health. Core financial methods cannot use it.

## Observability and validation

`V3 scanner admission audit` logs only project ID, `full`/`reuse` mode, success,
elapsed milliseconds, logical RPC call count and maximum age. No endpoint,
raw error, calldata, wallet or signed transaction is logged. Consumer progress
logs continue to report actual committed cursor advancement separately.

Validation covers first full/repeated hit/no sliding expiry, periodic early
refresh, clock rollback, cancellation, RPC/DB failure, failed refresh without
fallback, scope/approval/signed-evidence changes, reorg/current deployment/
economics checks, restart/mode/archival invalidation, capacity, concurrency and
transaction-side grant rejection. SQL tests exercise the real approved candidate
reader and original-attempt digest; network-none MySQL 8.4 rehearsal rejects
expired/epoch/foreign grants with unchanged business facts, while retaining
financial history, canonical receipt->applied fence, CAS, contention and rollback
regressions. Mock RPC tests prove hot verification never requests a receipt.

Local validation cannot prove commercial RPC latency or net falling production
lag. A separately authorized release must compare full/reuse RPC counts and
latency, consumer advance vs producer advance, every project's lag slope,
expired/failed audits and full-scan freshness over multiple audit periods.
Startup may still require several rounds of cold full audits. No rollout,
package, restart, database repair or chain operation is authorized here.
