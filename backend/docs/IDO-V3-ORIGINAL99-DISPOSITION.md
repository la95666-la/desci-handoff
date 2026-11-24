# #99 controlled nonce disposition

Explicit operator authorization on2026-10-31 covers minimum schema change,
Backend recovery route deployment, and disposition of original99 only. No
subsequent project create, Job/Admin/frontend release or general replacement.

## Fixed transaction

chain56 / nonce81 / from=to `0xfd712061ee08e070a930ecd45e935ae5d7f5445c`;
value0 / data empty / gas21000 / gasPrice100000000 wei (0.1gwei).
Maximum gas cost2100000000000 wei=0.0000021 BNB. Original is NOT replayed.
No user private key export, CLI signer or SSH hand transaction. Only the
existing authenticated Core application wallet signs after live execute guard.

POST `/admin/v1/ido-v3/original99-disposition` accepts actions:
preflight,prepare,broadcast,reconcile,status. Every request must contain exact
chain_id,nonce,target,gas,gas_price_raw,value_raw,reason. prepare confirms original
hash; broadcast confirms newly persisted disposition hash. No UI auto retry.

## Safety and identity

Default behavior without migration fails closed. The route requires existing
Auth/RequirePerm(project:execute), repeat live guard under original/project/
shared nonce transaction, pinned approved mode, pinned Core signer, independent
primary/public RPC and pinned V3/Safe runtime on both nodes. Approval48 and
immutable Set identity authenticate original99. This is an explicit incident
exception to normal no-replacement behavior, not a permanent generic capability.

Read-only preflight verifies original signature/hash/nonce/calldata/approval,
expired confirmed chain time, exact chain56, original absent on both nodes,
latest=pending81, target EOA code empty, balance and exact self-call/21000 gas
simulation. Unknown != absent. No original signature or financial data changes.

Prepare signs only once under locks and persists separate disposition row,
linked by original hash/raw SHA256. Existing original and sender fence stay
unchanged. Unknown COMMIT is resolved by status/readback, not repeat signing.
Broadcast CAS persists outcome-unknown marker and commits before at-most-one
application send. Marked-send never retries. Original appears/pending nonce
changes -> stop, reconcile hashes; do not force replacement.

Reconcile reads exact persisted child transaction and receipt from both nodes,
checks signature identity, status1/gas21000/no logs,15 confirmations, repeated
headers/receipt and nonce82 at the same confirmed block. Requires producer
stream epoch+canonical receipt block coverage. Atomically records confirmed
child receipt, original state v3_nonce_disposed/active_slotNULL and clears sender
active99 ONLY (next_nonce stays82). Original receipt fields remain NULL, original
hash/raw/nonce unchanged. This is neither original success nor original revert.

SQL guards restrict incident99, original project, sender, nonce, target, gas and
fees; reject fabricated disposed INSERT, altered signed identity, extra send
markers, NULL confirmations, deletes and unanchored slot release. Epoch/coverage
change fences subsequent allocations. Reorg recovery intentionally stops for
manual review rather than silently re-signing. Existing schema CHECKs, triggers
and unique sender nonce remain; child is separate table, not a duplicate outbox.

## Rollout/rollback

Back up original99 full private row, sender, project, executed Set identity and
approval, schema ledger, DDL and triggers. Restore rehearsal on isolated MySQL8.4
before production migration. Apply migration as separate audited schema change,
then verified Backend-only jc-release upgrade. Preserve deployed8ad3f25 minimum
purchase policy and all prior ancestors. No production config enable required.

Pre-sign rollback: previous Backend can run with additive dormant schema;
never drop audit or signed rows. After preparation/sign/send, rollback means
retain fences and reconcile both hashes, NOT delete child or restore stale
nonce snapshot. After confirmation, restoring prior schema CHECKs is unsafe;
retain schema and all audit rows. Follow current reorg incident coordinator.

## Validation

Regression/race/vet and disposable MySQL apply twice with explicit guard;
full disposable transition signed->marked->confirmed->disposed validates
constraints, retains original receipt/hash/nonce, and reorg epoch blocks next
allocation. Failure outputs retained, including original missing stream fixture,
CRLF runner issue and duplicate preseeded runtime corrected in test setup.
Unit reads cover unknown/used nonce/pending original/code/gas mismatch; controller
rejects missing or altered confirmation before reaching signer.

Production success additionally requires receipt agreement, independent DB
readback, sender evidence refresh and readiness; a successful release or RPC
send alone is not recovered business service. Never describe blocked99 as
restored until those gates pass. Admin login must be valid; do not forge JWT.
