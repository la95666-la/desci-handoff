# V3 whitelist candidate and review request

Source/test change only. No production config, migration, release, approval,
Safe proposal, signature or transaction broadcast is performed by this change.

## Responsibilities

- Candidate preview: canonical executed-Set/create DB identity, current runtime,
  project/policy, exact address set/tree/diff and pinned canonical blocks. No full
  original transaction audit or governance/execution simulation for this display.
  Enabled predecessors still need the full approved/scanned snapshot and receipt.
  The response remains diagnostic, never proof or execution authority.
- Review initiation: recompute the pinned preview/digest on the server; audit the
  original creation once per request; retain canonical original receipt/cursor,
  current policy/project, governance and exact call simulation at final recheck.
  The request object is private, explicitly passed, and bound to DB/RPC/project/
  governance/deadline. It is not a context grant or cross-request cache.
- Approval, handoff, execution, canonical event publication and wallet proofs keep
  their existing independent gates. A review candidate grants no chain authority.

The review used to recompute a full preview, then perform a full admission again,
then invoke a status reader with another full admission. That nested status read
and duplicate current-policy pass have been removed from review initiation.

## Deadline and failure semantics

One 150-second deadline covers approved review setup, recomputation and commit,
below the existing 170-second Admin boundary. Recompute retains its 75-second
slice. Final work requires 35 seconds remaining; transaction entry requires 5.
No later layer creates a fresh independent execution budget or owns another RPC.
This is not a production latency guarantee: actual provider latency must be
measured after separately authorized deployment.

Expired contexts survive sanitized lower-level errors as CHECK_TIMEOUT or
REQUEST_CANCELLED, with the finite failure phase. Original audit/final check
timings contain no RPC URL, transaction bytes, member lists or secrets.

Failures before transaction entry report `not_reserved`. A transaction/commit
failure remains `unknown`; clients must read the review list, not infer failure
from timeout. No automatic retry is introduced.

After locking incident/project cursor and checking live initiator permission,
an identical, unexpired, undecided/unexecuted draft by the same initiator is
read and fully reconstructed before returning its ID. Its expiry never slides.
The original 24-hour approval window must also still be open.
Ambiguous/corrupt duplicates fail closed. Different digests remain distinct
immutable drafts; version conflict at approval remains independently fenced.

The final transaction re-reads the approved Set and original canonical candidate
under the existing lock order; immutable change, halt, rewind or noncontiguous
revision change rejects the write. Ordinary canonical forward progress is valid.

## Scope

No schema/API input changes. Backend only; no Job/Admin/frontend source changes.
No fee/freshness thresholds, whitelist mode-freeze, member count cap, approval
TTL, distinct approver, Safe identity or nonce/outbox safety changes.

Tests cover request-only reuse, failed audits, scope/cancellation, budget and
deadline classification, exact duplicate reconstruction, growing head, policy/
state/governance/reorg/simulation/member tampering and valid Pending first enable,
Active enabled update and Pending disable. Existing approval/receipt/member/
execution tests remain authoritative; lightweight candidate is not a gate bypass.
