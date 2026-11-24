# V3 shared Proxy scanning: staged implementation

## Status and boundaries

Stage 1 is tick-local first-observation sharing in `v3_tick_reads.go`. It does
not change schema, project cursors, financial application, health authority,
nonce recovery, approval or proof gates. It has no signing/broadcast methods.
Local tests do not establish production catch-up. Do not deploy a migration
or infer authority from this design document.

Stage 2 has a default-off implementation in `v3_proxy_{stream,consumer,runtime,
recovery,operator}.go` and a separately gated draft schema alongside it. A
public scan cursor alone cannot admit a wallet. Consumers reuse the existing
project-specific canonical journal/projection/effect/outbox transaction. This
is a local candidate, not production migration/switch authorization.

## Implemented candidate

### Explicit test-only retirement and current confirmed baseline

`retire-tests-rebaseline` is a distinct operator decision, NOT recovery or
proof of skipped history. Requires exact sorted list of ALL current bindings,
SHA256 confirmation of its JSON ID array, expected public revision and a
fresh confirmed block/hash (within 128 heights of live head). The atomic
transaction archives those test projects, freezes old project cursors,
revokes projection/financial/policy publication, retains every journal and
original signed/outbox/nonce row unchanged, and creates a new public epoch at
the supplied baseline in `stream` mode. Immutable baseline payload records
the exact retired IDs. Earlier epochs remain auditable and are not considered
coverage for the new epoch. This entrypoint requires separately authorized
test retirement, backups and quiesced writers. It never silently retires newly
appearing bindings or emits successful application events for historical txs.
Archived projects are excluded from scheduled scans and common Set admission;
old consumer cursors stay halted. Future non-archived bindings retain full
creation/admission/proof gates. Existing suspended signed nonce fences are NOT
cleared by retirement. No chain transaction, asset withdrawal or successful
economic settlement is inferred from the operator's retirement confirmation.

### Provider-limit and switch-completion revision

Read-only production preflight exposed QuickNode plan HTTP 413 / -32615:
`eth_getLogs` accepts at most five block heights per query. Both legacy empty
windows and public collection now segment the COMPLETE query into <=5-height
pieces and repeat the full segmented pass; errors never mean empty results.
First production measurements: verified 4-height public collection ~2.2s,
16-height segmented collection ~4.5s (RPC only, not DB/whole-tick acceptance).

The public producer is now a separate bounded scheduled lane (20s turn every
5s with overlap skipped), so slow project admission no longer limits public
backfill to one fixed 128-height/10s request. Atomic units start at four heights,
adapt between 1 and 32, have an 8s cap, and commit at most 512 heights per turn.
Unit deadline retries require a fresh independent durable anchor/runtime check;
prior committed units are retained. Explicit evidence conflicts are never
shrunk/retried as benign deadlines. A transient timeout with revalidated tip
does not permanently halt immutable evidence, but cannot publish full health.
Source/chain uncertainty without revalidation remains fail-closed. Real throughput
and total backfill time still require measured acceptance.

`switch-stream` now additionally replays FULL creation-to-tip public coverage
and compares exact journal payloads, every economic counter/wallet, financial
effects and policy facts/current pinned policy. Recorded Safe observations
are rechecked against the original Safe execution/receipt and distinct-actor
decision. A bootstrap marker at creation is NOT full creation evidence: choose
the independently verified pre-creation anchor and backfill every height.

`audit-project --project-id` performs the same read-only economic audit.
`check-project` and `recover-project` require exact project/revision and full
coverage, plus independent approved original-create/runtime checks and original
suspended transaction verification. Only unchanged canonical tips can resume;
replaced tips still require explicit ancestor reconciliation. Resume uses the
existing atomic projection/effect/policy/outbox recovery transaction, never SQL
status patches, transaction resends or nonce resets. The former recovery-only
512-height limit is not increased: this is a separate full-public-coverage path.
`backfill --execute --expected-revision` runs bounded shadow-only evidence units.

The production inventory at this revision included a 455,320-height and a
69,351-height halted history; they cannot be silently skipped for a quick switch.
Migration, long backfill, independent restore rehearsal, authenticated original
approval/Safe evidence and sustained lag convergence remain rollout barriers,
not things a local test or matching endpoint hash proves.

Local regression now includes complete pre-creation coverage -> canonical
creation receipt -> wallet/economic/financial/policy parity -> guarded switch,
and rejects tampered counters or stale pinned policy. Unknown-tip recovery is
tested through the existing journal resume transaction with stale-revision
replay refused. Production policy execution observations additionally require
original Safe outer transaction, receipt and distinct-actor decision checks;
absence of an observation is not converted into approval. The toolkit cannot
fabricate an execution record, whitelist member, applied attempt or healthy scan.

- `idoV3ProxyStream.mode`: absent/`off` keeps the existing scanner without
  querying new tables; `shadow` produces bounded public coverage and compares
  legacy checkpoint/event placement while legacy scanning remains the writer;
  `stream` requires an already-installed, explicitly switched DB mode.
- `anchorBlock` and `anchorHash` are independently reviewed, explicit common
  anchors. Shadow bootstrap refuses an anchor above an existing project cursor.
  No startup action switches a DB row to stream mode.
- Producer validates contiguous headers twice, the complete Proxy log set twice,
  and each unique original transaction receipt twice. It verifies receipt log
  inclusion/completeness, runtime code/implementation/version and live finality.
  A batch is at most 128 blocks, 4096 logs, 1024 receipts and 8 MiB encoded per
  coverage block; limit failures do not skip blocks. Header passes use bounded
  parallel reads. Public payloads preserve full receipt metadata and have
  canonical-encoding SHA256 digests checked on every load.
- Consumer replays immutable evidence only inside a locked stream transaction,
  checks every covered block/hash/parent/digest and retains live state calls,
  independent original-create admission and live producer/consumer boundaries.
  It uses existing complete-block journal/finance/outbox application. Coverage
  replay is explicitly NOT represented as a second provider observation.
- Stream mode fences legacy cursor INSERT/UPDATE via schema triggers; only the
  pinned stream SQL transaction sets/restores its session writer marker. Locks
  and exact revision/CAS prevent conflicting producer commits. Session marker
  is a cooperating-writer fence, not a defense against a hostile DB account.
- Producer failure in stream mode atomically halts public progress and revokes
  all affected project publication/effects through existing halt functions.
  No event/checkpoint is guessed orphaned on unknown RPC. Shadow failure halts
  evidence only and preserves the legacy scan path. Producer runs independently
  in bounded turns; transient slice expiry with freshly revalidated tip remains
  retryable, while conflicts and unverifiable anchors remain fail-closed.
- Explicit recovery starts a new immutable epoch at a live-verified, recorded
  ancestor. Old epoch payloads remain unchanged. Project projections stay halted
  until replacement coverage and existing original-create/reorg admission pass.
- `/health/ido-v3-stream` reports producer head/lag, consumer gaps and pending
  final-success counts separately; `status` is liveness, NOT caught-up authority.
- `cmd/ido-v3-proxy-stream` defaults to read-only inspection. Mutation requires
  `--execute`, exact revision, fixed production DB identity and separately
  authorized mode switch or explicit ancestor/hash recovery. It cannot install
  schema, sign/send, clear nonce slots or automatically select an ancestor.

Local DB validation: `scripts/test-v3-proxy-stream-mysql.sh` reuses the existing
network-none MySQL 8.4 full-schema rehearsal, installs the gated draft there and
exercises producer/consumer/mode-fence/idempotence, event application, injected
transaction failure, halt and epoch retention. No external DSN is accepted.

Configuration template (do not copy an unreviewed anchor into production):

```yaml
idoV3ProxyStream:
  mode: off  # shadow only after schema + explicit anchor review
  anchorBlock: 0
  anchorHash: ''
```

Schema source: `pkg/job/job/chainevent/v3_proxy_stream_schema.sql`. It requires
`@JC_V3_PROXY_STREAM_APPROVED=1` and refuses an existing installation rather
than trusting `CREATE TABLE IF NOT EXISTS` against an older/different schema.
It is a draft outside the deployed migration ledger; promote it through the
normal independently approved migration/backup workflow before rollout.
The operator tool's `inspect` is read-only; `switch-stream`, `switch-shadow`
and `recover` require `--execute --expected-revision`. Recovery additionally
requires `--ancestor --ancestor-hash`. Stop/quiesce all scanner writers and
capture preflight/backups before a separately authorized switch/recovery.

### Remaining rollout gates / limitations

No production throughput/lag-convergence measurement, long-running shadow
parity, operator approval, migration backup/restore or DB permission review is
established by these local tests. Producer units remain bounded and adapt to
real RPC latency; catch-up must be proved by observation. Consumers still
perform expensive per-project original-create/state audits and currently
serialize on the stream row to make revocation/switch atomic. This first
persistent implementation removes duplicate block/log/receipt scans, not every
per-project RPC or database cost. The guarded switch now independently rebuilds
the full economic snapshot and compares financial/policy facts; production
rollout must finish that FULL historical coverage and gather runtime evidence,
including policy/Safe flows and reorg after financial effects, before switching.
Recovery never guesses the ancestor; a deep ancestor before the configured
anchor requires a separately reviewed earlier bootstrap rather than a reset.

## Stage 2: public canonical evidence plus independent consumers

One producer per `(chain_id, proxy_address, contract_version)` reads contiguous
confirmed ranges, complete Proxy logs and unique original transaction receipts.
Persist candidates for:

- Stream cursor: height/hash/revision/generation/state, under a writer lease.
- Block coverage: exact height/hash/parent, confirmation observation and revision.
  Include empty blocks: an absent project event is not proof of full coverage.
- Complete event set: block hash + tx hash + log index, event signature, raw
  topics/data, original receipt digest and versioned canonicality status.
- Project consumer cursor: contiguous coverage revision consumed, exact hash,
  last applied event, independent failure/reorg state and application generation.

Every key includes chain, Proxy and version. A receipt containing multiple
events is fetched once by the producer, but every log and complete receipt
binding must be verified. Unknown Proxy topics are not silently dropped.
Global governance/configuration events require explicit dependent-project
invalidation, not guessed project IDs.

The producer commits complete block coverage and its exact event set atomically
with cursor advance. Consumers require a continuous canonical prefix, not a
claim that no event exists between two heights. Consumption uses expected-old
revision/hash guards, journal uniqueness and existing project/economic replay.
Event, projection, business state and outbox effect application must remain
atomic/idempotent; nonce release happens only through the original controlled
effect path. A failed consumer closes that project's publication without
stopping other consumers or forging global full-application health.

## Bootstrap and transition

1. Inventory canonical project cursors/checkpoints and authenticated creation
   evidence. Do not set the public cursor to the largest existing project tip.
2. Start from a verified common anchor at/before the earliest consumer's needed
   prefix. Preserve independently verified creation facts for later projects.
3. Backfill in shadow mode without project financial/business writes. Compare
   complete blocks/events, consumer replay and effects with the old scanner.
4. Test a writer fence that prevents simultaneous legacy and stream writers
   for the same project. Switch only after parity, rollback and recovery pass.
5. Retain old checkpoints/journals for rollback. A reader switch must not make
   old and new effect versions applicable twice. Schema rollout and writer
   switch require separate reviewed release/backup authorization.

## Reorg protocol

On proven anchor mismatch, halt affected stream and project publications first.
Find an ancestor only from independently verified headers/checkpoints. Mark
superseded coverage/events noncanonical with append-only reconciliation evidence;
revoke affected consumer projections and financial effects through the existing
controlled reconciliation protocol. Unknown RPC is not proof of an ancestor.
Do not delete event history, clear signed nonce slots or resend transactions.
Republish only after canonical replacement coverage and each project's replay
are independently checked. Test crashes between every halt/revoke/replay step.

## Health and acceptance

Expose separately: producer cursor/confirmed lag, each consumer cursor/lag,
pending event count, oldest unapplied final-success transaction, revocation
failures and full eligible-binding coverage. A fresh iteration timestamp is
not catch-up. UI/API compatibility must keep proof and fee gates fail-closed.

Required tests: same-transaction multi-project logs, empty prefix then event,
global policy/runtime events, log omission/duplication/order changes, 429 and
missing receipts, late bindings, independent consumer failure, restarts,
concurrent writers, reorg across applied effects and crash recovery. Run with
disposable MySQL constraints and deterministic RPC history. Measure unique RPC
counts and cursor progress under provider-latency models; production acceptance
requires sustained scan rate above measured chain growth and decreasing lag
for every project, not only fewer requests or container health.

## Stage 1 limitations

Sharing exact overlapping first reads helps projects on overlapping heights.
Widely separated cursors and repeated creation/runtime audits remain expensive.
Do not increase concurrency, suppress rechecks, skip terminal projects, or raise
batch/time budgets without evidence. Use stage 1 counters and measured scan
progress to determine the stage 2 batch/provider budget, not a fixed percentage
derived from a synthetic overlapping fixture.
