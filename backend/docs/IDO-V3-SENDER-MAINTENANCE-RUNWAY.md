# Sender maintenance runway reliability

Source/test only. No production enable, evidence insert, release, proposal,
signature or transaction send. Based on82ab3a1; inherited fixes retained.

## Scheduling

The independent default-off Job lane checks its in-memory due time every5s,
instead of a30s cron quantization. Healthy/idle polls still wait30s and reuse
sufficient evidence; non-overlap/mutex and100s total/90s audit age bounds remain.
It still skips active sender slots, never changes their nonce or broadcasts.

Scheduling reads the minimum latest valid evidence runway from SQL UTC, bounded
to32 suspended originals/3s DB deadline. The result is a hint, NOT authority; it
does not bypass the exact exemption view, enrollment/revocation or live gates.
Unknown/revoked evidence cannot become ready via this snapshot.

Transient failure retry uses the original30/60/120/240s cap but clips it against
expiry minus180s request reserve minus95s audit/scheduler allowance. Urgent/unknown
runway retry floor10s plus deterministic0..2s jitter. Enrollment/revocation review
failures retain exponential backoff, not an aggressive loop. Every attempt still
must re-audit the whole set on both independent providers and atomically publish.
Under sustained outages no scheduler can guarantee admission availability.

## Diagnostics and retries

Read-only wrapper logs only primary/independent label, finite RPC method/error
kind, elapsed milliseconds and attempts; never endpoint, RPC payload, signature
or raw transaction. The failed audit's final read is preserved on the result.
Successful but invalid observations are classified as verification, not transport.

One additional200ms delayed retry is permitted for transient network/timeouts or
HTTP5xx with enough remaining deadline. No retry for semantic mismatches, missing
transactions, cancellation, RPC rejection or429 (the scanner HTTP layer already
has bounded429 handling). There is no send/sign interface. Failure leaves old
evidence untouched; expiry/TTL is not extended. Skip reason is explicit; success
logs say OK rather than misleading UNAVAILABLE on nil error.

## Backend/Admin

Readiness response additively includes minimum required reserve, current evidence
remaining seconds and exact string active submission ID when available. Active
slot takes diagnostic precedence with original-hash reconcile guidance. Current
RequireV3SenderReady remains the decision, snapshot never turns false into true.

Companion Admin shows unknown/refreshing/ready/unresolved/abnormal states, polls
read-only10s while not ready/30s when ready, and checks again before V3 create,
activate,end or platform-fee submit. In-flight state reads block cached-ready use.
No automatic transaction submission/retry, maintenance wake write or authority
token. Backend rechecks all gates; the UI cannot prevent every concurrent race.

## Unchanged / future work

180s pre-intent reserve,30s signing reserve,270s proactive threshold and300s evidence
TTL unchanged. No permanent exemptions, fabricated timestamps or request-inline
historical RPC audit. Independent lifecycle receipt scheduling is NOT implemented
in this patch and requires its own reviewed recovery/canonical event path. This
patch does not automatically resolve active signed/broadcast_unknown attempts.

Tests cover urgency/backoff/revocation, safe retry classification, secret-free
diagnostics, SQL UTC null/expired/revoked/active snapshots, real network-none
MySQL evidence expiry/refresh, exact view/trigger parity and original nonce/outbox
fences. Deploy Backend/Job/Admin independently after separate authorization.
