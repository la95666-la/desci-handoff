# Job readiness and bounded stream scheduling

## Incident and preserved safety contract

The v168 Job upgrade reached container startup, then failed readiness; the
restored v164 image failed the same business gate. A successful `ido_v3_end`
receipt was still `final_success`, awaiting canonical project consumption and
outbox application. An image rollback cannot remove that durable dependency.
The historical recovery consumed the event through the canonical path; this
change performs no recovery, signing, broadcasting, nonce repair or migration.

`/health/job-ready` continues to reject unresolved old final-success attempts.
Count and diagnostics share the exact same predicate and application-time
cutoff. Bare application DATETIME values must not be compared to a new DB-UTC
cutoff by accident. Tests cover UTC and UTC+8 application clocks.

## Diagnostic additions

When blocking count is nonzero, readiness includes at most five diagnostic
entries (one-second best-effort query budget): decimal-string submission and
project IDs, allowlisted action/state, wait seconds, receipt height, project
consumer height and blocks to receipt. Reason codes distinguish consumer
backlog, missing canonical journal event, and pending application. These are
observations, NOT repair instructions or substitutes for exact canonical
receipt/journal verification. Missing diagnostics never clears the count.
No calldata, signed bytes, keys, raw provider/SQL messages or wallet data is
included in these new diagnostic fields.

## Stream-only budget split

The 50-second tick and 35-second maximum project timeout remain unchanged.
Legacy scans retain their full start reservation. The serialized stream path
uses a five-second admission floor instead of requiring all 35 seconds to
remain before every project. After admission and durable tip reads, a lagging
consumer still requires 15 seconds before starting its bounded work window.
An already-current consumer verifies checkpoint and anchors without reserving
a nonexistent write window. Explicit lower-level recovery consumers retain
their original caller-controlled budget semantics.

No new concurrency, multi-window transaction, RPC quota increase, cached
admission authority or freshness relaxation is introduced. Started work still
has the parent deadline and source/CAS/epoch/economic checks. Deferred or
unvisited projects cannot complete the global generation. Fair rotation and
per-project retry state remain in place. A round can still be incomplete when
RPC latency exceeds its budget; this patch does not promise arbitrary-scale
catch-up or change the global proof architecture.

Successful postcommit verification now emits project cursor-before/after,
advanced blocks, producer snapshot, consumer lag and caught-up-to-snapshot.
These facts do not claim that the current chain head has been reached or that
the full round was admitted. Durable progress can coexist with a closed global
proof gate.

## Local validation

- Job/chain/chaintx/idov3whitelist/schema regressions.
- Deterministic seven-project scheduler and partial/deferred-round tests.
- UTC/UTC+8 cutoff, diagnostic redaction and reason classification tests.
- `scripts/test-job-readiness-stream-mysql.sh` wraps the existing pinned,
  network-none disposable MySQL 8.4 fixture. It requires the local deployment
  fixture checkout and pinned Linux Go toolchain. It refuses external DSNs or
  Docker endpoints through the authoritative fixture runner.
- That rehearsal includes seven independent durable consumers, idle/short
  window checks, seven wallet financial histories followed by empty windows,
  legacy/prepared parity, rollback and producer/consumer contention tests.
- The original receipt/canonical-application MySQL regression verifies that
  `final_success` retains the sender fence until the exact canonical journal
  applies the attempt and releases it; no readiness exemption is introduced.

Source/test changes only. No upload, Agent update, Job deployment or production
configuration/data change is authorized by this document.
