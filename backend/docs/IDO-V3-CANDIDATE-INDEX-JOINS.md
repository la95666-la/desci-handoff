# Indexable canonical checkpoint joins

Source/test only, based on14d38ec. No production SQL/DDL, migration, data repair,
configuration, runtime restart, deployment, signature or chain send performed.

## Evidence

Production ReadApprovedLockedCandidate was observed executing for13s. Its two
checkpoint aliases ck/k used only chain_id (key_len8), estimated1,912,721 rows
each, despite the existing primary key:

    chain_id,proxy_address,project_id,block_number,block_hash

BINARY(proxy_address) exact equality was not a sargable proxy-key condition.
The checkpoint table had~3.82m estimated rows. No row-lock wait was observed in
the read-only diagnostic snapshot. Audit log timings include SQL as well as RPC;
rpc_reads=29/55 is not evidence all elapsed time belongs to the provider.

## Change

Add ONLY these ordinary index-locator predicates:

    ck.proxy_address=s.proxy_address
    k.proxy_address=j.proxy_address

Retain ALL original BINARY proxy/version/hash/status/action/receipt guards, scope,
canonical creation/current checkpoint, original submission/event association,
active-slot refusal, running/halted distinction and LIMIT2 ambiguity behavior.
The ordinary equality narrows the candidate set, while exact binary comparisons
remain the authority. Do not replace BINARY guards with collation-only matches.

No FORCE INDEX, new index, schema or SQL argument change. Uses current indexes.
Production read-only EXPLAIN of this additive form used key_len68 and estimated
one row for BOTH checkpoint aliases. This was not an actual production timing
benchmark or an application change; separately authorized release is required.

Shared reader covers Backend/Job original admission and repeat evidence/cursor
checks, scanner cache grant validation, lifecycle receipt admission and related
policy/display consumers. RPC/runtime/approval/finality/financial checks remain.

## Validation

Exact production SQL is loaded from source for MySQL tests (mounted read-only
inside a network-none disposable8.4 container), not rewritten as a surrogate.
30k decoy checkpoint records span foreign proxies/projects. Fixture mirrors
existing composite keys and deliberately uses permissive collation to prove
binary residual guards still reject proxy/hash/version/status case aliases.

Compare legacy and indexed raw rows under canonical, orphaned, missing creation
checkpoint, foreign project, wrong receipt/action, occupied slot, running/halted
and extra-event cases. Existing public candidate reader unit tests independently
retain ambiguity, field/digest and original-receipt refusal checks.

Fixture ANALYZE TABLE is local ONLY, needed to establish optimizer statistics
after bulk seed; it is not a proposed production operation. EXPLAIN must use the
existing PK prefix past chain_id and <=4 estimated rows for ck/k. Timing is
informational, not an unstable pass/fail threshold or production promise.

Rollback is code-only. To obtain this improvement on both sides, separately
package/upgrade Backend and Job. Preserve production flags and the temporary
five-project maintenance schedule; this change never alters cadence or evidence.
After release measure actual SQL latency, audit time, budget yields, normal-project
freshness and advancement versus head growth. Readiness200 alone is insufficient.

This lineage also contains signed-login binding14d38ec, Safe-handoff60f8bb9,
scheduler52c5e13, maintenance1dcd010 and whitelist-review27aeb9b fixes.
