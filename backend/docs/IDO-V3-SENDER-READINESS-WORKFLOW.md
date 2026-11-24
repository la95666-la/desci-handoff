# V3 sender readiness and independent maintenance

Source-only change; no production refresh, create retry, config enable or
deployment performed. Includes fee-button fix dfcb220 and recovery27c19ba.
No migration, expiry extension, permanent exemption or retired state rewrite.

## Request path

Creation and lifecycle actions call `RequireV3SenderReady` before expensive
RPC preflight and again before intent reservation. It locks the incident gate,
reads suspended originals/current enrollment and the exact SQL exemption view,
requires >=180s evidence runway and no active sender slot, then releases locks.
No intent, signature or broadcast is created when this preflight is not ready.

Signing repeats the DB-only check under existing global/cursor/project/outbox
locks, with >=30s runway. No historical RPC or evidence INSERT occurs under
signing locks. Gas, latest/pending nonce, exact original calldata, nonce CAS,
SQL triggers and original first-broadcast simulation checks remain unchanged.
Evidence is not project freshness: these checks protect historical suspended
signatures and must not be deleted merely because a project is archived.

Codes: `SENDER_EVIDENCE_PENDING`, `SENDER_EVIDENCE_UNAVAILABLE`,
`SENDER_RETIREMENT_REVIEW_REQUIRED`; stage `sender_readiness`. Before intent,
outcome is `not_reserved`, next action `check_sender_maintenance` rather than
pretending a signed attempt exists. Revoked/unenrolled originals require review.
`GET /admin/v1/ido-v3/sender-readiness` is project-read permission gated and
read-only, exposes no secrets and grants no reusable signing authority.

## Independent lane

Job's `idoV3RetiredNonce.maintenance.enabled` is DEFAULT OFF. Existing
`idoV3RetiredNonce.enabled` must also be true. Explicit production rollout
authorization is needed for this evidence-writing job. Core alone does NOT
repair expired evidence, so deploy/enable Job before switching Core requests.
If disabled or unavailable, requests fail fast and safely.

Every30s, non-overlapping maintenance checks >=270s evidence runway (refreshing
proactively before the180s request threshold), and skips
ordinary polls without sliding timestamps. Idle sender only; active submissions
are skipped, never cancelled/resent/released. At most32 already-enrolled,
latest-non-revoked, archived retired originals are eligible. A primary approved
reader and independent BSC RPC re-audit exact raw transaction, sender, consumed
nonce, successful canonical original receipt/log/journal, retirement marker,
and current nonces. No private key or send interface is accepted.

All RPCs occur BEFORE SQL row locks, with100s overall/90s audit-age bound.
The short commit serializes incident -> retired originals/current enrollment
-> sender. It rechecks complete set, immutable row/journal/project, enrollment
IDs/revocation and idle next_nonce. New signatures/nonce changes/revocation
abort the whole batch. Only new five-minute independently audited evidence is
appended. The SQL exemption view must accept each result before commit.

Finite stages distinguish original transaction, receipt lookup, receipt anchor,
consumed nonce, journal mismatch, repeated receipt, final anchor, provider
disagreement, enrollment and commit. Logs identify failed retired submission;
no raw RPC URL/SQL error/transaction bytes are logged. Failures back off30/60/
120/240 seconds, capped; healthy checks resume every30s. No permanent privilege
is inferred from nonce increase. Current active-original recovery stays separate.

`cmd/ido-v3-sender-maintenance` provides explicit one-shot idle refresh, fixed
production sender confirmation and RDS identity check; it cannot create/sign/send.
Use only after backup and independent authorization, never in a request retry.

## Release/rollback

1. Run unit/race/vet and network-none MySQL8.4 tests.
2. Separately authorize Job rollout and enable maintenance; verify evidence
   refreshed, no active slot mutation, sender ready and bounded audit latency.
3. Roll out Core and Admin diagnostic update. Check project Set/create deadline
   and sale window afresh before user retries;75/76 remain cancelled records.
4. Rollback: disable maintenance then revert components as reviewed; retain all
   evidence and outbox facts. Default-off maintenance is not an unqualified fix
   until rollout config is explicitly enabled and audited.

Tests cover DB-only signing with nil RPC, expiry/revocation/unenrolled/schema,
active slot, pre-intent ordering, new diagnostics, idle double-provider refresh,
concurrent enrollment/nonce/revocation refusal, SQL trigger/view parity and
active-original recovery. No claim of permanent retirement closure is made.
