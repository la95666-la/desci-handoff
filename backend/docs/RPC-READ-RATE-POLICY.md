# Ethereum RPC read-rate policy

## Scope

`pkg/common/chainrpc` is the HTTP JSON-RPC construction point for the shared
Core/Job chain client, token inspection, V3 Set/create/lifecycle/policy/Safe
readers, V3 scanner and receipt recovery, Fair Launch mainnet collector, and
the fixed #45 recovery command. No business authorization is added by a
successful read or retry.

## Read and write separation

- Only an explicit single-request read allowlist is replayable: chain/header,
  receipt, transaction, nonce, balance, code, storage, logs, `eth_call`, gas
  estimation and fee queries.
- Unknown methods, batches, malformed/oversized bodies and non-replayable
  bodies are attempted at most once. `eth_sendRawTransaction`, transaction
  submission, signing and personal methods are **never automatically retried**.
- Only HTTP 429 is retried, at most four total transport attempts, with the
  same payload and fixed block tag. Other HTTP failures, transport errors,
  JSON-RPC errors and contract reverts stay fail-closed. In particular a
  transport failure must not trigger a transaction resend or another signature.
- Short numeric `Retry-After` is respected. Unsupported/long delay leaves the
  original 429 as a failure instead of waiting past the operation's budget.
- Redirects are refused so an endpoint credential or request body is not
  forwarded to another endpoint.

## Rate and time budgets

Interactive clients in the **same process**, targeting the same exact endpoint, share a
cancelable read lane. QuickNode reads use a 350ms minimum interval. Other
nodes do not acquire an artificial minimum delay. New HTTP clients cannot
reset the endpoint's rate budget. Queued reads and backoffs honor cancellation;
each HTTP call also has a 12s cap, including its local queue/backoff time.

The approved multi-project Job uses `DialScanner`, an independent lane shared
by exact endpoint with at most four simultaneous read transport attempts and
no artificial 350ms spacing. This is a concurrency bound, **not a requests-per-
second quota**. It retains the same read allowlist, four-attempt HTTP 429 bound,
payload/block preservation, cancellation, redirect refusal and write-no-retry
rules. Other Job clients and Core keep their existing interactive policy.

Within the 50s tick, three project workers receive independent 35s budgets.
Every tick enumerates all bindings afresh, rotates its initial dispatch after
the last attempted ID, and wraps; the priority is memory-only, never a chain
cursor or an authorization checkpoint. One failed project cannot stop the
other workers. Started failures retain fail-closed projection revocation;
partial coverage or any failure cannot publish `healthy`.

Each project scans at most 128 blocks per turn. An event-free range is at
most 16 blocks and is committed atomically only after two independent header
passes, ordered parent/hash validation, both log queries, runtime/project/
policy verification and the existing journal/projection checks. Header waits
overlap four at a time; every height and independent reorg recheck remains.
After committed progress, the driver yields before another unit if the remaining
time cannot cover twice the previous unit's duration plus 10s for final checks
(minimum total reserve 15s). Actual errors/deadlines are never treated as success;
final canonicality/depth verification still runs after a voluntary yield.
Event blocks retain the original receipt-aware path. Post-commit canonicality
or depth failure still revokes publication. Longer runs and database revocation
cleanup can outlive the scan context; overlap remains prohibited until all
workers have exited.

### Tick-local first-observation sharing (stage 1)

Only the approved multi-project scanner wraps its exact source for one tick.
Explicit first-observation call sites may coalesce/retain numeric header reads,
complete log queries (keyed by both bounds, addresses and topics), and receipts
(keyed by original hash). Callers receive deep copies, including nested log data,
topics and receipt integers. The namespace never crosses endpoints or ticks.
There is no TTL-based authority cache, no cached approval, and no sharing of
latest/head, project state calls, runtime calls or creation admission results.

Second header/log/receipt passes, cursor checks, pre/post-commit fences and
final confirmation-depth reads remain direct source reads. First observations
alone cannot publish anything. Policy-event receipts without an equivalent independent second receipt pass
remain direct reads as well. Missing/error results are not retained; source
HTTP 429 policy remains unchanged. One waiter's cancellation cannot poison
another project's read: the owner tick and a 12s per-read cap bound the source.
Tick teardown cancels and joins any remaining shared reads before releasing
the single-writer turn. Storage is bounded to 4096 entries, 32 MiB of encoded
response payload estimates and 1 MiB per retained item (not a hard RSS cap);
saturation falls back to direct reads rather than skipping verification.

Coverage logs include `shared_first_reads` and `source_first_reads` for marked
reads only, not total provider traffic. A five-project overlapping 16-height
fixture reduces header requests from 160 to 96 (40%) while retaining all five
independent second passes. Disjoint ranges have no sharing benefit; this is
not a production throughput or lag-convergence claim. The persistent shared
Proxy stream is a separately gated default-off candidate described in
`IDO-V3-PROXY-SCAN-PLAN.md`. Its schema is never installed by startup and its
writer mode never enabled implicitly. It stores verified public coverage and
unique receipts, then projects them with live state/anchor checks; tick-local
sharing is not used as persistent canonical authority.

V3 approval-backed create, activate, end and platform-fee withdrawal have a
150s whole-operation budget. The matching Admin calls use 160s and the
dedicated Nginx route uses 165s, below Core's 180s write limit. Other unrelated
Admin APIs retain their generic timeout. Updating static Admin assets alone
does **not** update the running Nginx configuration.

Lifecycle actions audit the original create once, then recheck its immutable
evidence, canonical receipt/headers and contiguous scanner progress before
the original signed call is submitted. Method-specific state, current
runtime/governance, simulation, transaction locks and nonce fences remain.
Fee actions still require full-scan freshness; activation retains its existing
per-project scanner gate. Settlement instead authenticates its own running
cursor/checkpoint and canonical header with confirmation depth, before signing
and again before broadcasting. Scanner lag is not substituted for current
settlement authority: the independently pinned confirmed-chain end simulation,
project economics and stored sale end must still agree. This does not relax
proof, financial projection, refund or fee-withdrawal publication gates.
No old signed row can be discarded.

Approved wallet proof reads get 50s rather than 15s for paced historical
verification, still below the C-side API proxy's 60s deadline. No proof,
canonicality result or wallet authority is cached or substituted.

## Known operational boundaries

- Core and Job are different processes. Their in-memory lanes are **not a
  distributed rate limiter**. Separate provider endpoints/quotas are recommended
  when combined traffic approaches the subscription's account-wide quota.
  Bounded 429 handling cannot make a persistently over-quota provider healthy.
- WebSocket/IPC retain the native geth transport without HTTP pacing/retry.
- The approved Job retains a 50s scan context and never publishes a healthy
  scan for an incomplete enumeration. Four concurrent reads cannot guarantee
  sufficient throughput under arbitrary provider latency/quota or project count.
  Deployment acceptance must measure each cursor, confirmed lag, complete tick
  success and 429 volume; lower request volume or a healthy container is not
  evidence of scan progress. Retries do not grant freshness.
- The scheduled automatic end job currently selects legacy V1 or exact V2,
  not V3. This change does not add V3 auto-settlement. V3 end is the controlled
  Admin operator entrypoint.
- On ambiguous transaction results, reconcile the original hash. Never clear
  a nonce slot, mark an observation successful, switch to a different RPC for
  a new send, or allocate another attempt solely because of an RPC failure.

## Validation

The common transport tests cover shared-client pacing, queued cancellation,
unchanged read payloads, bounded persistent 429, short/long delay, body/batch
rejection, redirect refusal and non-retry of writes/transport errors. An HTTP
test server exercises the real geth RPC client. Source wiring tests prevent
direct Ethereum dial calls from returning to the reviewed Core/Job paths.
Scanner tests cover endpoint-lane isolation/concurrency, fair rotation after
cancellation, failure isolation, project deadlines and two complete ordered
header passes with missing/wrong/cancelled/oversized responses rejected.
These are local tests, not evidence of a successful production transaction.
