# V3 action-specific evidence and original first broadcast

## Scope

Source-only change after v176. No migration required; existing outbox, nonce,
approval, policy journal, freshness and tracking schema is retained. No
production transaction, deployment or config enable is performed by this change.

## Why

Submissions 72/74 stopped at `signed_project_scan_canonical`: the consumer had
advanced its durable cursor while postcommit anchor/certificate publication
was incomplete. A valid old certificate no longer matched the new revision.
Increasing TTL cannot fix that exact identity mismatch. The signed nonce
remained correctly fenced, but there was no general first-broadcast continuation.

## Evidence boundaries

| Operation | Evidence |
|---|---|
| Create | executed Set, deadline, immutable calldata/token/fee terms, runtime/governance, simulation, outbox/nonce |
| Activate/end/platform fee | full original creation audit, running canonical checkpoint/recovery fence, current project/caller/runtime and exact simulation |
| Policy preview/review/approval/handoff/proposal | original creation, current root/version/state, complete reviewed member snapshot, predecessor canonical receipt/journal, live distinct admins, approval digest/expiry/supersession, Safe nonce/signature |
| Policy observation/publication | original outer/Safe hash, canonical successful receipt and exact policy event, finality/runtime/governance, latest policy, immutable review and execution digest |
| Finished/archive/maintenance admission | existing complete project coverage, ledger and obligation evidence; unchanged |

The first three action families no longer require financial project certificate
TTL or exact latest cursor/certificate revision equality. A halted/rewound/
replaced canonical source or active reorg is still rejected. Canonical forward
cursor progress is independently checked rather than compared as a frozen tip.
Policy updates retain their precise prior version snapshot and canonical Safe
receipt/journal requirements. This is not permission to invent an old list from
a root or accept an unapproved list. Wallet proof and Admin display routes are
not changed. Source/project TTL/lag constants are unchanged.

## Original continuation

Admin chain transaction page provides two explicit POST operations:

- `/v1/chain-transactions/:id/v3-original/preflight`
- `/v1/chain-transactions/:id/v3-original/broadcast`

Body: `{tx_hash, reason}`; reason 3–500 characters, exact original hash, project
execute permission. Existing authenticated audit middleware records requests.
Preflight and actual broadcast independently repeat checks; preflight is not a
reusable token or permission to skip the actual check.

Only exact V3 create/activate/end/platform fee `signed`, `broadcast_count=0`,
no prior broadcast/receipt/final-event, immutable signing time and original
signature are eligible. Two endpoint sources must affirm chain ID, original
hash/receipt absence, exact latest/pending nonce, sufficient balance and current
`eth_call` with ORIGINAL gas/gasPrice/value/calldata. Unknown is never absence.
Create must still satisfy the original executed Set and current creation terms;
an expired or changed Set cannot be recovered by resigning. Lifecycle repeats
current project simulation and original creation admission.

The marker transaction locks incident -> cursor/checkpoint (except pre-create)
-> project -> permission/approval and original submission -> sender nonce.
It checks exact persisted bytes, original hash, current project state and
`active_submission_id`, then conditionally persists `broadcast_unknown`,
`broadcast_count=1` BEFORE COMMIT and the one send. A lost CAS, rollback or
unknown COMMIT cannot call the provider. Provider failure never grants a retry.
No signer/key, new intent, nonce allocation, replacement or cancellation exists
in the continuation path. Job only audits original receipts/events; it does
not automatically broadcast. Same-action new intent remains fenced.

If marker commits and process dies before send, outcome remains unknown and
this entrypoint deliberately refuses a second invocation. Operator incident
review is required; do not clear the marker, release nonce or send manually.
This provides at-most-one application send, not an exactly-once network claim.

Normal create/lifecycle calls also use exact dual-provider simulation and the
guarded persisted transport. Most expensive checks run before signing; final
post-sign runtime/state/nonce checks remain. New signed failures are visible
and resumable explicitly; automatic success or guaranteed immediate execution
is not claimed. Receipt reconciliation and canonical business application
remain separate; UI says submitted, not completed.

## Scanner budget

Scheduled consumers reserve 8 seconds before cursor commit for postcommit
anchors/certificate publication. Insufficient reserve yields before writes.
`publication_budget_yield` is a scheduling reason, not source failure; next
turn increases consumer allowance. This reduces but cannot eliminate network
failure after commit. No certificate is published before independent anchors.

## Validation and rollout

Tests cover exact signatures/gas, nonce drift, absent versus unknown, existing
receipt/pending transaction, balance and simulation failure, dry-run rollback,
lost CAS, COMMIT ambiguity, send ambiguity, permission revocation, canonical
cursor with no certificate, policy identity/version/reorg checks, and budget
yield. Disposable network-none MySQL tests exercise the shared outbox marker,
unknown send followed by refusal, receipt/event reconciliation, financial
projection parity, settlement obligations and tracking fences.

Deploy compatible Backend and Admin for continuation UI. Job budget change is
separate; source does not run any recovery on startup. No new config flag or
SQL migration is required. Existing production submission 74 is NOT sent by
tests, build, deployment, or this source change; an explicit authorized action
is required and current checks may reject it. Preserve old failure logs.

Rollback is component binary rollback; preserve outbox/nonce/audit facts. Once
broadcast_count=1, neither old nor new code may resend that original transaction.
