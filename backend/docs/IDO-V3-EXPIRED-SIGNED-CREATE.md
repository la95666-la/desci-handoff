# Expired signed create: prevention and recovery boundary

## Incident99, production read-only evidence

Project43880023173627905, original nonce81, original hash
`0x5bb02ae9cc1e61cf0f352c83ad536b776622b0ca8cdd9002974f18ef92d3158b`.
Signature persisted2026-10-31 18:45:31.915912 Shanghai; approved start/deadline
18:45:49. Block130843385 at18:45:48: identical calldata/gas simulation succeeds.
Block130843386 at18:45:49: InvalidProjectParameters(), selector0x124f0efa.
Failure log18:45:49.616 in original_exact_simulation. Contract rejects
startTime<=block.timestamp. Primary historical call verifies both adjacent
blocks; independent provider rejects historical eth_call (not a second proof).
Both current providers return null tx/receipt and latest=pending nonce81.

Private evidence (outside repository):
`/home/la956/.joincare-production-backups/20261031T115923Z-submission99-readonly/`.
No chain mutation, signature, evidence renewal or nonce/outbox UPDATE performed.

## Prevention

- Initial approved create request requires strictly>180 seconds until approved
  start. Reservation/unsigned/sign preparation requires strictly>90 seconds.
- Signing also requires>60 seconds remaining on the request deadline, checked
  before RPC and immediately before signing. This covers post-sign checks;
  slow/outage requests still fail closed. It is not an availability guarantee.
- Atomic raw persistence compares DB UTC to approved start minus90 seconds.
  SQL rejection rolls back nonce changes; existing unsigned cleanup applies.
  Signed-time receipt admission remains original start, NOT start minus90:
  existing signatures/receipts remain valid for historical reconciliation.
- Create first-broadcast continuation after expiry explicitly returns
  CREATE_DEADLINE_EXPIRED with durable signed/original reconciliation outcome.
- Active sender slot takes diagnostic precedence over expired evidence while
  retaining suspended/evidence->nonce lock order and all original deny gates.
- Sender180s/30s, TTL300s, canonical/financial/approval checks unchanged.

## Priority receipt work

Default-off `idoV3OriginalReconciliation.enabled`. Separate5s cron with30s due
time, and priority at start of scanner Run to avoid cron starvation.25s child
deadline; the scanner's50s parent budget remains authoritative when inline.
Same job mutex as project consumer: no second project/cursor writer. Independent
producer still runs separately. Endpoint/runtime/production DB UUID checks
remain. Fixed sender, chain56, proxy, exact original active slot and nonce join;
ambiguous selections fail closed. Cheap original receipt probe does not sign,
send, refresh evidence, release a slot or declare success. Existing full
reconcilers alone can finalize. Success still needs canonical event application.

This can help an already mined original (like incident89) but CANNOT release
an original with no receipt (like99). Normal scanner reconciliation stays
enabled for compatibility and eventual application; no financial scan bypass.

## What this patch does NOT fix in production

No production deployment/enable, no new disposition action, no migration.
Original99 is still fenced until an approved protocol disposition completes.
Null lookup, elapsed time, local signature deletion, or failed eth_call cannot
prove no raw bytes exist elsewhere and cannot safely reset/reuse its nonce.
Sending the expired original just to force a revert is prohibited. An unrelated
manual relayer transaction or hand-edited final_revert/applied is prohibited.

## Required separately reviewed recovery design (not executable)

A controlled nonce-disposition protocol would need an immutable child linked
to99, distinct from its project create semantics, live permission/independent
approval, exact target/value/data/chain/nonce/fee cap, current dual-provider
original absence or original-receipt takeover, verified original raw identity,
simulation of the disposition, reorg/nonce locks and atomic durable persistence
before at-most-one application send. A canonical confirmed nonce-consuming
receipt would be required before superseding the old signature or releasing
its fence; reorg recovery must fence both. Lost COMMIT/send outcomes always
reconcile original or disposition hashes, never blind resend. DB CHECKs and
nonce/immutability triggers and all existing consumers must understand this
new state; an informal reuse of cancelled/final_revert would be false evidence.

This introduces a same-nonce replacement capability, currently absent and
outside ordinary original-hash recovery rules. It needs explicit reviewed
exception/authorization before implementation or signing. Do not activate a
partial implementation to restore service. Expected business value0 and
estimated gas/fee must be simulated and approved at execution time, not guessed
from this document. Until then, service remains blocked honestly.

Tests: runway/request boundaries; source-shared SQL prevents17s signature
persistence on disposable network-none MySQL8.4; expired original diagnostic;
active slot precedence; receipt unknown/mismatch/refusal cannot finalize;
fixed original selection, default-off and single-writer scheduling; inherited
receipt/financial/immutable signing regression. First disposable run passed
runway test but failed unrelated empty-range due missing stream fixture schema;
runner fixed to install stream schema; complete rerun passes. Logs retained.
