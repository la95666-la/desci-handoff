# Fair Launch V1 Migration and M0-M6 Cutover Runbook

## Boundary

Development artifacts may be completed and reviewed. Applying DDL, enabling listeners/writes, signing, broadcasting, or deploying remains **NO-GO** until §18.5 deployment gates and a signed change ticket pass.

## Artifacts

- `2026_09_03_fair_launch_project_model.sql`: additive discriminator/economic facts.
- `2026_09_04_fair_launch_ce_v2_governance.sql`: identity/placement/observation/application, shadow ledger, Safe/Timelock/executor.
- `2026_09_05_fair_launch_signing_release_control.sql`: signing generations, Gateway journal, Source Registry, watermark/recovery.
- `migrations/verify/*`: read-only pre/post checks.
- `migrations/rollback/*`: rollback blockers only; no automatic DROP.

## External authority and exact Source Registry binding

Repository schema/dependency files are contract/spec or candidate evidence only. Production migration and resume require repo-outside, non-symlink, protected reviewer/release authority plus independently admitted normalized `information_schema` snapshots produced only after the pinned baseline and migrations execute on real MySQL 8. Every final/phase snapshot is re-bound at render time to the actual pinned baseline SHA-256, each actual 0903-0907 migration SHA-256, and the canonical `FAIR_LAUNCH_SHA256SUMS` manifest digest; the same external authority must admit that digest as `mysql8MigrationChecksumManifest`. No real snapshot/checkpoint is currently admitted, so production intentionally fails closed. Each frozen 0903-0906 resume checkpoint has its own admitted digest and proves the exact completed prefix, the approved partial object, absent future suffix (including additive 0907), unexpected presence/absence, complete definitions and phase digest; final-schema or first-table shortcuts are forbidden. Migration 0907 is additive and has no admitted partial-object resume checkpoint.

Source Registry canonical payload keys are snake_case. Snapshot `implementation_code_hash` and `abi_digest` are lowercase `0x`-prefixed `CHAR(66)`; `chain_contract_source_revision` stores the corresponding lowercase unprefixed `CHAR(64)`. SQL and application lookup must compare `LOWER(SUBSTRING(snapshot_value,3))` to the revision value. Snapshots bind the RFC 8785 canonical payload digest to exact environment, chain, proxy, implementation/codehash, ABI, source/revision and runtime/policy/source-registry/config generations. MySQL does not implement JCS; application/stored verification must recompute and compare the digest before persistence and fail closed on mismatch.

## Mandatory gates

1. Restore a production-like backup into disposable MySQL 8.0.
2. Record schema/data digests and run preflight; every blocker count must be zero.
3. Produce a mode-0600 manifest binding each migration filename to SHA-256, approver identity and ticket.
4. Apply only to disposable MySQL first. Any `applying` migration is an incident; never replay blindly.
5. Run post-apply checks and archive output/digests. The runner may mark `applied` only after the full schema manifest (tables, columns, indexes, constraints and views) passes.
6. M0 inventory: table/FK/index/trigger inventory, image digests, active reorg/outbox zero.
7. M1 expand; M2 receipt-derived ordinal backfill; quarantine unknown identity.
8. M3 dual-write V1/V2; M4 shadow read; M5 seven-family equivalence and zero quarantine/conflict.
9. M6 atomically change reader capability/watermark. No per-request V1 fallback.
10. Only after deployment gates: enable Source, then listener, then writes; never all at once.

## Interrupted DDL and safe resume

- Never edit an `applying` ledger row to `applied` by hand.
- Resume requires `migrations/verify/2026_09_fair_launch_resume.sql` to match the approved filename/checksum and validate the predecessor plus exact interrupted-object shape. The admitted phase artifact also supplies executable COUNT/content assertions for the exact partial object; callers may not assign `JC_FL_RESUME_NONEMPTY_PARTIALS`.
- Resume only by idempotently replaying the same migration. Structurally incomplete placeholder tables are forbidden.
- Disposable validation is rooted in the pinned `migrations/baseline/fair-launch-pre-2026-09-03.sql` fixture and its checked-in SHA-256, and must exercise `09_03` through `09_06` including controlled interruption.

## Rollback

Before M6, disable new Fair Launch work and forward-fix or restore the pre-M1 backup only after proving no Fair Launch source/project/application/ledger/signing rows exist. After M6 or first Fair Launch source/project, old images are ineligible; use a Fair-capable maintenance image and forward-fix.

## Evidence to archive

Migration checksums, disposable validation log, M0 inventory root, quarantine rows, seven M5 row digests and ordered aggregate, cutover block/hash, writer/reader image digests, approval identities, capability/source/watermark/deployment-attestation digests.


## Resume and exact-shape rule

An `applying` ledger row never authorizes a manual mark-applied. Resume validates every reviewed object by normalized column type/null/default/charset/collation/generated expression, ordered/unique indexes, FK target/actions, CHECK clause, view definition and trigger identity. Non-empty or non-exact partial objects are quarantined for manual adjudication; only migration-owned empty exact partial objects may converge by idempotent replay. Final postconditions always run, including the all-skipped path. Disposable MySQL 8 testing must include corrupt column type, ordered index, CHECK and FK cases.

## Source Registry database privilege gate

Source Registry admission is a separate database security boundary, not a trigger-only convention. This release is intentionally **candidate-only and transformation/activation unavailable**:

1. A future repository-external, 2-of-3 Ed25519 signed POLICY/work-order must bind the exact environment, deployment, generation, predecessor, identities and positive grant-policy digest.
2. Backend and Job must remain stopped while a separately reviewed external schema authority applies DDL, fixed-definer objects and exact grants. This repository does not provide that executor.
3. Post-state observation may only be generated by a reviewer-owned external collector. The repository collector is intentionally unavailable and cannot create any `.candidate.json`; the application auditor is deliberately unable to inspect `mysql.user`, authentication state, default roles, or grants for other accounts.
4. Production must remain stopped until reviewers sign a POST-STATE ADMISSION over that exact external candidate and activation acquires the same MySQL advisory lease used by transformation, holds it through service startup, and verifies a fresh live observation. This repository's activation/lease path remains unavailable.

Production trust-root/checkpoint constants remain `UNADMITTED_PLACEHOLDER`; schema/resume/dependency authority entrypoints return `fair_launch_authority_unavailable`; transformation returns `fair_launch_transformation_unavailable`; candidate collection returns `fair_launch_candidate_collection_unavailable`; and protected-schema activation returns `fair_launch_activation_unavailable`. Real MySQL execution, external auditing and reviewer-owned signatures remain mandatory.

Real disposable MySQL 8 evidence remains mandatory. Review `SHOW CREATE PROCEDURE`, trigger/routine definers, normalized `SHOW GRANTS`, global/database/table/routine grants, default roles, account lock/authentication state, `CURRENT_USER()` host matching, direct-DML/EXECUTE denial, authorized procedure execution, success/error session cleanup on a reused connection, reinstall/all-skipped behavior and interruption recovery. Static tests do not substitute for this gate.

Fixed identities are `jc_fl_registry_definer@localhost` (locked procedure definer) and `jc_fl_registry_ingest@%` (procedure caller). The ordinary runtime application user is explicitly partially revoked from `INSERT`, `UPDATE`, and `DELETE` on `fair_launch_source_registry_snapshot` and from procedure `EXECUTE`. The ingestion identity receives only table `SELECT` and exact procedure `EXECUTE`, never direct DML. Do not store either administrator or ingestion credentials in Git, manifests, logs, release archives, or application configuration.

Reviewer admission must verify MySQL 8 `SHOW CREATE PROCEDURE`, exact routine and trigger definers, normalized `SHOW GRANTS` across global/database/table/routine scopes, role/default-role membership, account lock/authentication state, exact positive allowlists, direct-DML denial for runtime and ingestion identities, runtime procedure-EXECUTE denial, and successful procedure invocation only from the ingestion identity. Lower-level partial revokes are not an authority mechanism. Test-only identities/authority must remain outside production and any `JC_FAIR_LAUNCH_TEST_*` input remains forbidden.

## Fair Launch wallet-write gate

No production Backend durable wallet-submission/recovery authority is implemented in this release. Therefore every public Fair Launch wallet-write descriptor returns `available=false` with `reason=durable_wallet_submission_unavailable`. This includes contribution, Claim, Refund, project-token deposit and failed-project-token withdrawal, and consequently every ERC-20 approval generation including zero-first `approve(0)` and exact `approve(target)`. Browser `localStorage` is only a non-authoritative display cache and cannot clear or authorize a retry. Enabling these actions requires a separately reviewed Backend durable intent service and signed reviewer recovery integration; a wallet receipt alone is insufficient.
