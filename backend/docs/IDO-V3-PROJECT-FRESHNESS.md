# Independent project freshness and split ordinary budgets

## Behavior / rollout boundary

Ordinary projects no longer squeeze a cold/periodic full audit and a 15-second
consumer reservation into one fixed 25-second turn. A project needing a full
audit receives a 35-second audit opportunity, yields back to the priority
scheduler after success, and is NOT counted consumed or fresh. A subsequent
reuse turn retains the ordinary quantum; a consumer-budget yield raises its
next allowance to 35 seconds. Active priority/fairness, parent deadline and
serial transactions remain. Repeated audit/yield never updates success time.

Project-level evidence is an **explicit default-off feature**:

```yaml
idoV3ProjectFreshness:
  enabled: false
```

False keeps the current global scan gates and requires no new production
schema. True requires the independently gated migration
`2026_10_29_ido_v3_project_freshness.sql` and coordinated Backend/Job builds.
Missing or incomplete schema/evidence fails closed; there is no legacy-global
fallback when enabled. This source change does NOT execute the migration or
enable the flag. Job-only packaging cannot activate the complete solution.

## New evidence model

Global `Begin/FinishApprovedV3Scan` stays truthful: all enumerated projects must
succeed in one generation before a global complete observation is published.
Priority scheduling can still leave it checking. It is retained as an operator
metric/legacy gate, not made artificially healthy by combining ticks.

Enabled business gates instead require:

1. A running, confirmed public source with recent successful production and
   independent deployment/anchor verification (<=90s), no active global reorg.
2. THIS project's <=300s successful verified observation, at most **660 blocks**
   behind the current public tip. The source itself may be at most 135 blocks
   behind its observed head, including the 15-confirmation policy. These are
   explicit conservative bounds, not inferred from a runtime health Boolean.
3. Exact process token, epoch, approval/original-create identity digest, running
   cursor/hash/revision, canonical checkpoint and matching projection revision.
4. Existing independent live project admission, policy, Safe receipt/review,
   wallet membership and action simulation/signing/nonce gates at the caller.

The 660-block project allowance approximates five minutes at the observed
2.2 blocks/second; it is NOT a wall-clock guarantee. The independent project
certificate TTL is **300 seconds from verified_at**, checked on each use. The
public source TTL remains 90 seconds. Public source limits remain 120 confirmed blocks
and 135 blocks behind observed head; they are not derived from the project cap.
Only code thresholds change; no schema migration/configuration enable is needed.
New Backend/Job artifacts require separately authorized packaging/deployment.

`ido_v3_source_freshness` names a process generation and source state. Each Job
process establishes a new random token in `checking` before publishing source
health, invalidating prior process project certificates. Schema readiness checks
the tables, writer triggers and enforced keys/FKs/CHECKs. The token is not a
credential and is not an instruction to modify signed attempts.

`ido_v3_project_freshness` is created only by the canonical scanner after its
postcommit live checks (or equivalent verified idle anchors). It binds the
consumer's actual public producer snapshot, NOT an uncovered future confirmed
height. Backfill before that snapshot cannot publish fresh evidence. A normal
rescan leaves the existing project row and verified_at untouched. Budget yield,
cancellation and recognized temporary failures do not extend or delete its
established lifetime. A determined identity/evidence failure deletes the
current process's certificate immediately using an independent bounded cleanup
context. Existing cursor/epoch/process/approval reader fences still reject
changed evidence even if deletion fails. Atomicity protects the
certificate plus exact identity/cursor reread in its own stream->project locked
transaction AFTER business publication: a certificate failure closes
availability but never rolls back/rewrites already committed financial facts.
Publication uses one transactional UPSERT: old committed evidence remains
visible until replacement commits (no delete-then-insert). New cursor commits
still make old certificates unusable until matching postcommit publication;
that necessary exact-revision safety gap is not bypassed.
Consumer metadata timestamps are not refreshed for idle polling; only the
separate verified-observation certificate is refreshed.

No expiry, audit-only success, source error, epoch change, projection/cursor
rewrite, failed postcommit anchor, startup, missing row or unrelated project's
old certificate can satisfy the gate. Source RPC failures mark source checking.
Old evidence is unusable after reorg/mode/epoch/process change even if retained
for diagnostics. No automatic successful-row backfill is performed.

## Backend affected paths

- Approved wallet policy proof: target-project gate before/after existing full
  admission and policy/receipt/member checks.
- Approved policy preview/review/status: target-project gate, including initial
  off-chain review; read-only candidate exceptions retain their existing scope.
- Safe policy execution observation: waits on the target project, not all other
  projects. It still independently validates the original Safe receipt.
- End/fee operations: target-project evidence when enabled; settlement retains
  its independent live canonical cursor checks too. Activation's existing
  independent full/live checks remain unchanged, not newly bypassed.

Unrelated project failure no longer blocks a correctly certified project, but
global source failure affects all projects by design. This does not promise
stable availability if THIS project's success gaps exceed the freshness bound.

## Validation

Unit/SQL fixtures cover split audit-only turns, adaptive consumer allowance,
no false completion, unchanged disabled gates, enabled no-fallback, complete
publication with exact original evidence, missing schema, process-token reset,
unrelated-project isolation, target identity and Backend caller wiring.

`scripts/test-project-freshness-mysql.sh` wraps the existing network-none pinned
MySQL8.4 fixture: independently tests migration approval and unguarded writer
rejection, repeat execution, actual freshness query using isolated temporary
tables (expired/future timestamps, epoch/restart/source/cursor/checkpoint/
projection/coverage/approval/reorg/lag failures), and existing canonical consumer
financial/recovery/CAS regression. It accepts only the local deployment fixture
and migration path, never a production DSN. Counter/receipt fixtures do not
impersonate a real production admission.

## Controlled production sequence (not authorization)

1. Separate DB identity/schema/backup/restore preflight and migration approval.
2. Install migration; verify actual schema/constraints/triggers; no row backfill.
3. Deploy matching Backend and Job with feature disabled; confirm legacy health.
4. Under separate configuration authorization, enable Job first. Verify new
   process source health and per-project certificate generation/expiry.
5. Enable Backend; validate Active proof/review happy paths and all fail-closed
   scenarios; measure Active intervals/lag and ordinary progression over
   multiple full-audit periods.
6. Rollback: disable Backend's project gate first (legacy compatibility), then
   Job if required; keep evidence tables/rows, do not DROP during incident or
   alter outbox/nonce/ledger facts. Global legacy freshness may itself be stale;
   reverting is not evidence of restored availability.

No production writes, migration, configuration enable, signing, deployment or
packaging is implicit in this document. The schema and Backend/Job releases
must be handled separately with exact source and artifact identities.
