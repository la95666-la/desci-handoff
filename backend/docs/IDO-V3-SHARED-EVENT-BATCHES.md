# V3 event-driven shared batches

## Scope and removed duplication

The running approved stream scanner now follows V2's shared-batch/event-routing
principle without replacing per-project durable history with a guessed global
cursor. No migration, signer, financial API, Agent, RPC limit, concurrency or
proof-freshness setting changes. Bootstrap, recovery and reference legacy paths
retain their existing bounded consumers.

1. Read and validate immutable public coverage in aligned **128-block pages**.
   Projects with different cursor offsets can reuse overlapping pages. The
   cache is one tick/connection-pool/stream-scope/epoch only, deep-copies
   results, refuses nested transaction reuse and stays size bounded. Production
   GORM tracing Session wrappers no longer accidentally disable sharing.
2. Pick one confirmed public deployment height per round. Cache only successful
   code/storage and argument-free deployment calls at an exact height+live
   header hash. Creation/policy/project state, receipts, current heads and
   canonical header rereads are NOT cached. New tick means a new cache. A
   changed live header selects another namespace; source failures never cache.
3. Route the complete known Proxy log set by indexed project ID. Own financial
   or policy events and global/governance events use the existing checked
   per-block replay path. Other-project events do not force this project's
   state replay. Their stored receipts remain checked. Unknown/malformed
   topics never become a quiet interval.
4. A project-quiet interval uses **one pinned end-of-interval project state**
   observation. The four repeated start/end project+policy observations are
   removed. Current policy is not reconstructed from absence of a project
   event; policy journal/proof checks remain on the affected-event path.
5. A scanner-only batch may advance up to **512 blocks** with one bounded
   quiet-window transaction/checkpoint INSERT. Public producer request sizes
   remain unchanged. Payload is capped at 32 MiB, and preparation still yields
   between atomic units. Event-dense work can commit a shorter verified prefix.
6. An already-current project verifies anchors/checkpoint but does not perform
   a business transaction or update the durable consumer timestamp simply for
   being polled. Global full-round success is still separately checked.

## What remains necessary

- Exact executed approval/original-create audit baseline and periodic live
  checks (previous optimization), with expiry bounded through submission.
- Complete continuous coverage/digests, event/receipt placement and epoch.
- Project checkpoint and cursor CAS; stream->project lock ordering.
- Existing journal replay comparison rejects altered financial/wallet totals.
- End-state counters/immutable terms must match replayed projection. An omitted
  material event cannot be turned into progress merely because topics route.
- Atomic journal/effects/outbox/business projection/checkpoint/cursor changes.
- Original-hash lifecycle application and sender-fence rules, postcommit live
  anchors, integrity revocation, recovery and proof freshness remain in force.

This is deliberately NOT a project-audit bypass or a conversion to trusting
`eth_getLogs` alone. The goal is to remove duplicate same-evidence work, not
independent checks against corrupted state or an uncontrolled reorg.

## Evidence and rollout limits

Local real MySQL 8.4 differential rehearsal compares one 512-block quiet batch
to four old 128-block windows: identical durable financial/journal/outbox/cursor
facts, checkpoint INSERTs **4 -> 1**, SQL **112 -> 32**, and only three end-state
contract calls (version/policy/project) for the new quiet window. Tests also
compare shared vs reference eventful facts, post-purchase/claim/refund/proceeds/
fee/token-withdraw histories, corrupt projections, immutable coverage sharing,
CAS/epoch/depth changes, competing producer, crash rollback and idle timestamps.
Unit tests cover other-project routing, malformed topics, receipt checks, live
header cache invalidation, deep copies, errors and cancelled contexts.

`V3 shared event batch reads` logs public cache hits/source reads, batch ceiling
and confirmed height; existing logs report actual project advancement. No raw
RPC URL, call payload, wallet or error string is logged. Audit `rpc_reads`
counts logical calls, not necessarily source calls after sharing; use the new
public cache counters alongside elapsed time and actual project lag slopes.

This is still a serialized writer with per-project checkpoints and financial
replay. It is not a full global-consumer schema rewrite or a guarantee of
commercial-RPC throughput. Independently authorize packaging/deployment, then
validate per-project lag falling, bounded transaction time, no unhandled
events/deadletters and global scan freshness across multiple audit periods.
No production operations are authorized by this document.
