# V3 scan scheduling fairness and honest budget yields

## Production evidence

On 2026-10-28 01:56–02:01 Asia/Shanghai, project `43368658176704510`
stayed running but its cursor remained `130119358`. Logs showed parent deadlines
with project elapsed times of 22.969s, 8.089s and even 614ms, then later attempts
of 12–15s. The 50s round contained seven serial bindings, with this largest ID last.
Its next 128 covered blocks were complete and empty. No lock wait was observed
in the sampled database read. It later advanced intermittently to `130119614`.

The next-tick pointer used the last dispatched ID. If all IDs were dispatched,
the last ID (even a tail-budget failure) reset the next round back to the first.
Additionally, runV3ConsumerWindows returned nil if less than 15s remained,
allowing a zero-consumption project to be counted completed. Neither behavior
is a canonical conflict or reason to revoke history.

## Change

- Check the parent remaining budget at worker start, not just when dispatching.
  A project only starts when a full configured project opportunity fits (35s
  in production). Its original timeout and the 50s parent bound remain intact.
- Unstarted projects are skipped, never counted completed or revoked. They
  prevent full-round freshness publication. A budget skip does not call the
  source retry function or increase that project's backoff.
- Partial rounds resume after the last actually started project. The first
  skipped/unvisited project therefore gets priority on the next invocation.
- If every project started, rotate the FIRST slot by one project on the next
  round, even when all were dispatched but some failed. Persistent bad sources
  cannot pin first position, and high IDs cannot stay at the tail forever.
- Consumer pre-window budget yield returns typed deferred and has a distinct
  phase. It is incomplete, not success and not an integrity failure.
- Round logs include `not_started` and `deferred`; unstarted covers insufficient
  budget or parent cancellation. Attempted means actual
  started work, not channel handoffs. Existing completed/failed counters remain.

Scheduling state remains in memory and resets on restart. Enumeration is fresh
each round; priority is never a durable cursor or evidence. All started workers
and any required revocation finish before the writer mutex is released. The
stream consumer remains ONE worker. No RPC quota/concurrency increase, larger
transaction, signing retry, removed CAS/checkpoint/economic gate, or timeout
increase is introduced.

## Limits

This removes starvation from clipped budgets; it does not remove repeated
creation/runtime admission cost or guarantee catch-up throughput. Reserving a
full opportunity may leave part of the parent budget unused. With enough slow
projects, partial rounds will remain freshness-incomplete by design; changing
global freshness into per-project certificates is a separate security design.
Completed still does not mean a cursor necessarily grew: already-current or
Set-without-signed-create work may legitimately finish without growth.

## Validation and release boundary

Tests deterministically reproduce 50s parent / 35s project clipped rounds across
seven serial projects for two complete cycles; every project gets a full start
opportunity. Tests cover full-round first-slot rotation, typed yield, no-budget
no-revoke behavior, changing ID lists, existing parallel cancellation/failure
isolation, worker bounds, and no-work units no longer marked success. Existing
wallet JSON, financial, reorg, epoch and MySQL rollback regressions still apply.

Source/test/commit only. No package, production configuration change, restart,
recovery or deployment is implicit. Production validation must measure actual
confirmed head, each project cursor/lag, not-started/deferred counts and lack of
repeated tail-budget failures over multiple rounds; unit tests are not a claim
of production performance.
