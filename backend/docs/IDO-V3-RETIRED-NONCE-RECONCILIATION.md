# Retired V3 original-hash nonce reconciliation

## Scope

This candidate narrows the global suspended-signature fence only for original
transactions of explicitly retired tests whose nonce is independently proved
consumed on the confirmed canonical chain. It does NOT change outbox state to
`applied`, restore a project/financial/proof entitlement, send/sign a transaction,
clear a nonce slot or ignore an unknown/replaced transaction.

`v3_reorg_suspended` remains the historical outbox state. The separate evidence
table/view represents **retired and nonce-consumed**, not project recovery.

## Necessary conditions (all required)

- Archived, locked, same-chain/Proxy/version project and immutable public
  baseline payload containing this exact project ID in `RetiredTests`.
- Suspended signed original attempt, no active slot, exact successful persisted
  receipt and canonical original journal fact.
- Original bytes validate signer/chain/nonce/calldata/target. The provider's
  original transaction bytes must equal those persisted bytes, not a replacement
  transaction at the same nonce.
- Two independently fetched successful receipts agree on full contents and
  match the expected action's exact journal event; receipt block is canonical.
- At least 15 confirmations; pinned confirmed header remains stable across
  reads; sender nonce AT that confirmed block has passed the original nonce.
- Live latest and pending nonce agree and match the existing DB allocator fence;
  no active submission exists. No allocator counter is changed by reconciliation.
- Exact identity evidence, canonical anchors and consumed nonce recorded with a
  SHA256 digest and DB-derived five-minute expiry. Evidence cannot be edited or
  deleted; revocation is allowed. Latest revoked evidence is not auto-renewed.

## Application and DB agreement

`ido_v3_retired_nonce_exemption` is the single DB identity/freshness predicate
used by both nonce triggers and the Go signing guard. Address comparisons for
suspended sender lookup are case-insensitive by canonical lowercase, preventing
an EIP-55 casing bypass. No exemption exists solely because a project is archived.

Core feature `idoV3RetiredNonce.enabled` is absent/false by default. Without
suspended rows old behavior is unchanged and no new tables are queried. With
suspended rows and feature off the signer still refuses. With feature on, every
historical suspension must have been explicitly enrolled. The signer re-audits
each enrolled original transaction live and appends refreshed evidence inside
the signing transaction before the shared SQL predicate admits nonce allocation.
Unknown/missing/reverted/replaced receipt, unsupported original bytes, revoked
enrollment or drift fails before signing. Ordinary RPC transport has no write
retry. A DB/RPC failure rolls back new evidence/signature together.

DB triggers can enforce identities and bounded freshness, not independently call
RPC. Their five-minute predicate protects legacy non-V3 nonce allocators as well;
deployment must quiesce those allocators during enrollment and restore the
conservative trigger or revoke evidence on any reorg alarm. Do not represent a
short-lived attestation as permanent finality. Production consensus/custody
evidence and broad legacy allocator live rechecks remain operational boundaries.

## Operator tooling and rollout

- Draft gated migration: `pkg/common/chaintx/v3_retired_nonce.sql`, requiring
  `@JC_V3_RETIRED_NONCE_APPROVED=1`. Includes insert TTL/source guards, immutable
  history/revocation triggers, view and conservative-trigger replacement.
- `cmd/ido-v3-retired-nonce`: requires exact `--submission-id --confirm-hash`
  plus existing isolated `--config-dir`; defaults to read-only dry-run.
  `--execute` persists evidence only. Pins production DB UUID and chain 56;
  does not register/configure a signer or install schema.
- Install through checksum-bound migration and verified backups/restore. Deploy
  the Core signer code independently from Job; a Job-only upgrade cannot change
  Core's signing check. Enable feature separately after migration/health gates.
- Dry-run each exact old attempt, review original receipt/nonce and then enroll
  explicitly with `--execute`. Only subsequently use the controlled create API
  for a NEW unsigned attempt. Never rebroadcast retired signed bytes.
- Rollback: disable feature, quiesce allocators, use separately gated
  `v3_retired_nonce_rollback.sql` to revoke evidence and restore strict triggers.
  Evidence and original nonce/outbox history remain retained.

## Validation

`scripts/test-v3-retired-nonce-mysql.sh` uses network-none MySQL 8.4 and the full
existing schema. It exercises default-off/unenrolled denial, dry-run no-write,
explicit enrollment, SQL/Go agreement, immutable evidence, revocation, expiry,
live re-audit, original-receipt reorg rejection and unchanged original outbox.
Pure tests cover successful exact consumption and missing/failed/mutable receipt,
wrong original bytes/nonce/binding, pending tx/drift and cancellation.

Local tests are not production enrollment or a successful new project creation.
