# V3 policy parameter export and Safe proposal

Source/test only. No migration, production configuration/review edit, Safe API
request, signature, proposal or chain broadcast performed by this change.

## GET safe-handoff

Approved-mode parameter export is DB-only, with5s budget. It reconstructs the
immutable complete member set/root/digest and exact calldata; checks live read
permission, chain56/pinned proxy, distinct approval, scope, expiry, supersession
and absence of a recorded execution. The Governor is pinned application config.
No RPC, original receipt audit, project financial freshness, scanner cursor or
relayer signing key is needed. Response explicitly sets chain_verified=false and
execution_authority=false. Old policy/project state fields describe the approved
candidate, not today's chain; export is not a promise that execution will succeed.

## GET/POST safe-proposal

Approved-mode proposal construction has one50s total deadline. Both prepare and
POST use exact call simulation from the Governor at a canonical latest block.
Minimal target/Safe/runtime code binding prevents a call to an EOA/unknown proxy
returning0x from being mistaken for success. No historical create audit, project
projection/freshness or full status reader is invoked. The contract checks its
own current version, state, pause, role and policy transition rules.

POST retains live project-execute permission, immutable approved content/expiry,
Safe owner signature, nonce-derived Safe hash, queue conflict refusal and exact
service readback. A final DB recheck and simulation occur after nonce/queue checks
immediately before the POST. Off-chain approval cannot be inferred from eth_call.
Simulation is momentary, not guaranteed later execution; Safe signers still review
the exact transaction. Only their external execution changes chain state.

Unknown Safe POST/readback results retain the original-hash reconciliation rule.
No automatic proposal retry and no relayer signing/broadcast added. Existing token
configuration proposals and legacy non-approved policy paths keep their gates.

## Policy status display

Admin display budget30s instead of120s. Use canonical DB-bound identity, fixed
confirmed block project/policy/runtime reads, DB identity recheck and final anchor.
Remove full original receipt audit, governance roundtrip and repeated policy pass.
The result remains unavailable_for_proof. Halted display is explicitly historical.
Review initiation, second approval, execution observation and wallet proof checks
remain independent and are not substituted by this lighter reader.

No schema/API input changes; two additive handoff booleans. Backend only, includes
52c5e13 ancestors. Admin currently warns signers to independently verify arguments;
no live execution/eligibility assertion may be inferred from parameter export.

Validation includes DB-only handoff without RPC, member/digest tampering, revoked
reader, missing/expired/superseded/executed/same-admin decisions, exact simulated
caller/target/data/value, wrong chain/runtime/Safe/reorg/revert, failed beforePost
check preventing service POST, existing signature/queue/readback tests, and actual
network-none MySQL8.4 approval predicates. No production performance guarantee;
separately authorized deployment must measure real latency.
