# Fair Launch local materialization prerequisites

Status: **NO-GO for full local financial E2E**

This document defines the additive domain and schema contract required before a
mechanically local-only Fair Launch CE-V2 admission seam may be implemented.
It does not enable persistence, wallet submission, migration application, RPC
broadcasting, or production runtime. The production/default behavior remains:

```text
fair_launch_materialization_unavailable
```

A local admission seam must not be controlled by an ordinary YAML value,
environment variable, command-line flag, HTTP request, build tag accepted by a
release workflow, or a private-key API. It may be designed only after all
requirements below exist and pass independent review.

### Ordinary financial reads versus audit reads

Ordinary capped-presale V2 outputs must validate the exact parent identity and
scope child `fund_ledger` / `ido_purchase_entitlement` facts to
`contract_version=v2_stablecoin`; monetary entitlement totals additionally use
only `status=canonical`. An endpoint explicitly displaying ledger history may
retain canonical, reversed and reconciliation statuses, but never another
contract version.

Audit and reconciliation loaders are intentionally different. They must read
all child contract versions and statuses so identity drift and noncanonical
facts remain observable and produce integrity mismatches. In particular, do
not add ordinary-read filters to `loadFundFactsTx`, fund/entitlement fact
commitments, `GetProjectFundReconciliation`, on-chain reconciliation integrity
checks, or reconciliation task/worker loaders.

## 1. Canonical event effect manifest

Add an immutable, versioned manifest in Backend source. Every frozen event name
must map to exactly one typed effect definition. The definition must include:

- event name and effect manifest version;
- lifecycle mutation, if any, with allowed predecessor and successor states;
- governance projection mutation, if any;
- position mutation, if any;
- ordered ledger entries, if any;
- exact ABI argument used for token, amount, account and project identity;
- whether the amount is an inflow, outflow, claim, refund or compensation;
- canonical digest field order and domain separator.

The repository must derive effects from the lossless `FairLaunchTypedEvent`.
Callers must not supply `[]FundLedgerV2Shadow`. Supplied ledger rows are not a
trusted input and must be removed from `FairLaunchCEWrite` before admission.

At minimum the financial mapping is:

| Event | Entry type | Token | Amount | Counterparty |
|---|---|---|---|---|
| `FairLaunchContribution` | `payment_contributed` | indexed `arg2` | `arg3` | indexed user `arg1` |
| `FairLaunchClaimed` | `project_token_claimed` | indexed `arg2` | `arg3` | indexed user `arg1` |
| `FairLaunchRefunded` | `payment_refunded` | indexed `arg2` | `arg3` | indexed user `arg1` |
| `FairLaunchProjectProceedsWithdrawn` | `project_proceeds_paid` | indexed `arg1` | `arg3` | indexed receiver `arg2` |
| `FairLaunchPlatformFeeWithdrawn` | `platform_fee_paid` | indexed `arg1` | `arg3` | indexed receiver `arg2` |
| `FairLaunchFailedProjectTokensWithdrawn` | `project_tokens_withdrawn` | indexed `arg1` | `arg3` | indexed owner `arg2` |

Amounts must be canonical uint256 decimal strings and addresses must be
lowercase, nonzero EVM addresses. The event project ID, emitting proxy, project
contract identity and token identity must agree with the locked project row.

## 2. Additive canonical ledger schema

`fund_ledger_v2_shadow` is currently insufficient because it does not persist
entry semantics or all financial identity. Add a later migration; do not modify
frozen migrations 0903-0907.

Required additive columns (either on a new immutable table or a versioned
replacement table) are:

```text
chain_id                    BIGINT UNSIGNED NOT NULL
project_id                  BIGINT UNSIGNED NOT NULL
contract_address            CHAR(42) ascii_bin NOT NULL
contract_version            VARCHAR(32) ascii_bin NOT NULL
entry_type                  VARCHAR(64) ascii_bin NOT NULL
flow_direction              ENUM('inflow','outflow','noncash','compensation') NOT NULL
token_address               CHAR(42) ascii_bin NOT NULL
amount_raw                  VARCHAR(78) ascii_bin NOT NULL
counterparty_address        CHAR(42) ascii_bin NOT NULL
source_event_identity_id    BIGINT UNSIGNED NOT NULL
source_placement_id         BIGINT UNSIGNED NOT NULL
source_application_attempt_id BIGINT UNSIGNED NOT NULL
entry_ordinal               INT UNSIGNED NOT NULL
effect_manifest_version     INT UNSIGNED NOT NULL
reversal_of_ledger_id       BIGINT UNSIGNED NULL
compensation_generation     INT UNSIGNED NULL
row_digest                  CHAR(64) ascii_bin NOT NULL
created_at                  DATETIME(6) NOT NULL
```

Required constraints:

- unique forward row `(source_application_attempt_id, entry_ordinal)`;
- unique compensation `(reversal_of_ledger_id, compensation_generation)`;
- same-identity composite foreign keys from ledger to application, placement
  and event identity;
- `project_id` FK to `projects(project_id)` with `RESTRICT` actions;
- canonical uint256 check for `amount_raw`;
- lowercase nonzero EVM checks for contract, token and counterparty;
- allowed `entry_type` set pinned to the manifest version;
- a forward row has no reversal reference/generation;
- a compensation row has both reversal reference and positive generation;
- row digest covers every semantic and provenance field, not only amount and
  counterparty.

The digest must use a documented canonical encoding (length-prefixed binary or
RFC 8785 JCS), a domain separator such as
`joincare:fair-launch:ledger:v1`, and SHA-256. `fmt.Sprintf` concatenation is not
an admissible financial digest format.

## 3. Governance projection

Add one current-state row per trusted source revision, plus immutable history.
A current-state table must include:

```text
source_id, source_revision_id, chain_id, contract_address
operator_address, pause_guardian_address, unpause_timelock_address
treasury_address, platform_fee_receiver, platform_fee_bps
contributions_paused, payouts_paused, claims_pause_until
state_generation, last_event_identity_id, last_application_attempt_id
state_digest, updated_at
```

Policy/admission events are keyed data and require versioned child projections:

```text
payment token policy revision (token, policy_version, decimals,
  runtime_code_hash, policy_digest)
payment token admission (token, current_policy_version, enabled_for_new_projects)
payment token emergency block (token, emergency_blocked)
project token policy (token, enabled, emergency_blocked, decimals,
  policy_version, runtime_code_hash, policy_digest)
beneficiary Safe admission (account, approved, attestation_digest,
  attestation_version)
release policy (policy_digest, policy_version, enabled)
```

Every row needs source/application provenance and a canonical state digest.
Updating a current row and inserting its effect journal record must occur in the
same transaction as the exactly-once guard and cursor advance.

## 4. Reversible effect journal

Add an immutable forward journal that stores sufficient typed before/after
state to reverse an event without querying or guessing historical state:

```text
id, event_identity_id, placement_id, source_revision_id
source_application_lineage_id, effect_manifest_version
effect_kind, aggregate_kind, aggregate_key, mutation_ordinal
before_payload_jcs, before_digest
after_payload_jcs, after_digest, created_at
```

Do not mutate the forward journal to record reversal. Append-only reversal
proof uses three separate tables:

```text
fair_launch_lineage_reversal_v3
  (reversed_lineage_id, reversal_lineage_id, reversed_at, created_at)
fair_launch_journal_reversal_v3
  (source_journal_id, reversed_lineage_id, reversal_lineage_id, created_at)
fair_launch_reversal_closure_v3
  (reversed_lineage_id, reversal_lineage_id, source/linked journal counts,
   source/compensation ledger counts, closure_digest, created_at)
```

`aggregate_key` must identify the exact project, wallet position, governance
row or policy child row. Payload schemas are versioned and field-complete for
the mutation. For project lifecycle this includes at least status, terminal
reason, totals, final price components, settlement coordinates, participant
count, and all withdrawal flags. For position it includes contribution,
claimed, refunded, event provenance and status. For governance it includes all
changed fields and generation/provenance.

Required uniqueness and closure:

- `(source_application_lineage_id, mutation_ordinal)`;
- one lineage-reversal pair per forward lineage and reversal lineage;
- one journal-reversal link per source journal;
- closure is inserted last and proves exact journal and ledger set equality,
  not merely equal counts;
- a reversal application references the exact forward lineage, source revision,
  event identity, effect version and placement generation.

## 5. Effect and transaction digest attestation

Before inserting the exactly-once guard, compute:

```text
payload_digest = digest(lossless typed event + emitting source identity)
effect_digest  = digest(ordered project/governance/position journal mutations
                        + ordered canonical ledger rows)
```

The guard, application attempt and effect attestation row must agree on the
same manifest version and digest. Add an attestation row containing event,
placement, source revision, manifest version, payload digest, effect digest,
ledger digest, mutation digest, row counts and created timestamp.

Replay must reconstruct the expected plan and compare all digests and counts.
It may record a no-op observation/attempt only after exact equality; it must not
mutate project, governance, position or ledger state.

## 6. Transaction-level reorg reversal

Implement a repository entry point that receives the forward event identity,
canonical placement generation, source lease token and fence epoch. In one
MySQL transaction it must:

1. lock source cursor, placement, forward application, guard, effect journal,
   affected aggregate rows and forward ledger rows in deterministic order;
2. reject a lost lease/fence, already reversed effect, wrong placement, or any
   canonical dependent event whose before-state is based on this after-state;
3. create the noncanonical placement observation/evidence;
4. create one reversal application attempt referencing the forward attempt;
5. compare every current aggregate digest with the journal after-digest;
6. restore every journal before-payload and verify the resulting digest;
7. insert ordered compensating ledger rows with the same token and amount,
   reversal entry type/direction, `reversal_of_ledger_id`, and a monotonically
   locked compensation generation;
8. mark the forward placement noncanonical, clear its canonical slot, transition
   the mutable guard, and append lineage/journal reversal links; never update
   immutable forward lineage or journal rows;
9. rewind or replace the materialization cursor to the previous canonical
   placement using an explicit predecessor, never by `MAX(block_number)`
   guessing;
10. commit all changes together. Any failure rolls back all rows.

A later canonical placement of the same event identity must create a new
placement generation and a new exact application whose predecessor is the
reversal application. It must not reuse the reversed exactly-once guard as a
permanent no-op.

## 7. Backend files and wiring

Implementation dependency order:

1. **Domain and entity contract**
   - `pkg/common/entity/fair_launch_control.go`
   - new versioned ledger, governance, effect-journal and attestation entities
   - additive Deploy migration reviewed against exact entity columns
2. **Pure effect construction**
   - `pkg/job/job/chainevent/fair_launch_materializer.go`
   - new canonical ledger/governance/project/position planners
   - uint256/address/state-transition validation and digest package
3. **Forward transaction repository**
   - `pkg/job/job/chainevent/fair_launch_ce_repository.go`
   - remove caller-built ledger input
   - apply all effect classes and attest before guard/cursor commit
4. **Reversal repository**
   - new `fair_launch_ce_reversal_repository.go`
   - deterministic locks, dependency detection, compensation and cursor rewind
5. **Trusted ingestion bridge**
   - source routing must honor full block/transaction/intra-transaction revision
     boundaries and exact signed revision identity
   - decode archived logs, assign `tx_log_ordinal`, finalize, then call the
     forward repository under a held source lease/fence
6. **Job lifecycle wiring**
   - add a dedicated Fair Launch materialization job to `pkg/job/job/starter.go`
   - bridge generic reorg detection to the reversal repository
   - extend ProjectEnd candidate collection without weakening legacy selection;
     reserve the existing durable unsigned Fair Launch outbox only after fresh
     chain preflight
7. **Local-only admission design**
   - only after steps 1-6 and review, create a separate test binary under a
     test-only package that accepts an unexported capability minted in that
     binary
   - capability construction must require chain ID 31337, loopback RPC, a
     disposable database fingerprint, and test Source Registry trust root
   - normal `cmd/core`, `cmd/job`, config loader and release artifacts must not
     import the constructor or contain the capability
   - `ApplyFairLaunchCEV2Tx` in production remains unavailable

## 8. Required tests before any local admission

Pure/unit tests:

- all frozen events have exactly one manifest definition;
- ABI argument/type and ledger mapping tests for every financial event;
- every lifecycle predecessor/successor, including invalidation and release;
- every governance event before/after projection;
- canonical encoding and golden digest vectors;
- malformed/noncanonical uint256, zero/wrong token/address and project mismatch;
- no caller-controlled ledger semantics.

Disposable MySQL 8.4 integration tests:

- first apply commits identity, placement, observation, application, guard,
  all projections, journal, attestation, ledger and cursor atomically;
- failure injected at every stage leaves none of those changes committed;
- exact replay creates observation/no-op attempt only;
- conflicting payload/placement/source/effect fails closed;
- concurrent duplicate application produces one forward effect;
- lease/fence loss before and during transaction rolls back;
- governance and all lifecycle paths materialize exact state;
- contribution participant counting is idempotent across first and repeat
  contribution;
- reorg with dependants blocks with no changes;
- leaf reorg restores all before-state, compensates ledger and rewinds cursor;
- reversal failure injection rolls back every change;
- replacement canonical placement reapplies exactly once;
- raw/display position invariants remain valid after forward and reversal.

Job/local harness tests:

- local chain 31337 only and loopback-only RPC rejection tests;
- signed test Source Registry revision and exact DB binding;
- ingestion order for multiple logs in one transaction;
- finalized placement policy;
- Fair Launch automatic end reservation/recovery without a private-key API;
- full successful, failed and cancelled lifecycles through projections and
  reconciliation;
- production binaries and ordinary env/config can never mint local admission.

## 9. Existing model/schema mismatches to resolve first

The current Backend source models do not exactly match frozen migration 0904:

- `ChainContractSource` omits `start_block` present in schema;
- `ChainContractSourceRevision` uses `effective_from_*`/`effective_to_*` and a
  JSON confirmation policy, while schema uses `from_*`/`to_*` and a scalar
  confirmation policy; it also omits dependency manifest and predecessor/event
  identity columns;
- `ChainEventIdentityQuarantineV2` represents a chain/tx quarantine, while 0904
  defines a legacy-table/legacy-id quarantine.

These differences must be resolved through new additive/versioned schema and
entities. Frozen migration files must not be edited, and neither side may be
silently treated as authoritative until independently reviewed.

## 10. Accepted 0908-0912 schema and current Deploy transport blocker

Deploy migrations 0908-0912 at commit
`8eef5accc07dcd2540eb7f2d3a469b4e5a7b8431` are independently accepted. They
resolve the earlier exact-set reversal evidence, `uint256.max`, composite
lineage provenance, immutable run fingerprint, run-chain source binding, exact
grant policy and startup-cleanup findings.

The remaining cross-repository blocker is transport, not schema. The committed
`scripts/fair-launch/local-e2e/start.sh` starts MySQL with `--network none`,
does not publish a loopback TCP port, and does not mount a run-owned Unix socket
outside the container. `docker exec ... mysql` is sufficient for the Deploy
schema harness but cannot provide independent Backend Runtime/Admin/Job/Ingest
`database/sql` pools with real multi-statement transaction and row-lock
semantics.

Backend must not implement a docker-exec pseudo-driver or expose root/helper
authority. A separately reviewed Deploy slice must provide either exact
`127.0.0.1:<random-port>` binding or a run-owned Unix socket, separate mode-600
per-role credentials, endpoint/socket identity in the immutable run
fingerprint, bind/ownership and teardown checks, exact `SHOW GRANTS`, and
independent-pool transaction/row-lock integration tests.

Until that transport slice is accepted, no DB-backed admission verifier, usable
capability constructor, local command, materializer repository, MFA provider or
test signer may be added. The reserved local seam returns
`fair_launch_local_e2e_transport_admission_unavailable` before database access.
