# V3 Active-first bounded scheduling

## Scope

Approved stream scanning now uses a process-local priority scheduler instead
of one equal-ID round-robin queue. The shared producer, canonical consumer,
512-block ceiling, periodic admission audit, financial replay, signed outbox,
single writer and 90-second global proof freshness are unchanged. Legacy and
explicit operator consumer paths retain their existing scheduling. No schema,
RPC quota, concurrency or production configuration change is required.

## Selection and bounds

- Active projects, Pending/Active projects within five minutes of a start/end
  boundary, and projects with unresolved durable transaction slots are urgent.
- Known new public project events also promote an ended project, so refunds,
  claims and settlement are not permanently behind Active work. Promotion is
  a scheduling hint only; normal event/receipt/economic checks still apply.
- Metadata and newly covered public event hints are reread between atomic
  project turns. The poll examines at most 128 new coverage heights; cold or
  very large historical gaps use a bounded tail observation and ordinary
  aging, not an unbounded payload scan. Hints do not replace canonical replay
  or guarantee discovery of every historical backlog event in a single turn.
- Each project has independent next-due, attempt and successful-completion
  times. Urgent due interval is 30 seconds; Pending is 45 seconds; ended/normal
  work is 60 seconds; unsigned Set is 120 seconds. These are scheduling targets,
  not strict minimum poll spacing: unvisited projects may still use spare
  round capacity so a full enumeration can complete without false skipping.
- Reevaluate urgent work after EVERY normal batch. An urgent project may run
  again when due; it cannot inflate unique project counts or erase a failure.
- At most two urgent turns before one ordinary opportunity. A normal project
  waiting >=90 seconds gets aging preference after an urgent opportunity.
  Thus urgent work cannot starve all backlog, nor can many aged normal projects
  form an entire round in front of Active work.
- Urgent turn cap remains 35 seconds. Normal work has a 25-second quantum.
  Never start a selected project with a clipped parent slice: return to the
  five-second cron boundary if the full quantum cannot fit. A started atomic
  database transaction is never preempted to serve another project.
- Genuine source backoff is skipped by the selector. Budget/yield/capacity
  deferrals (including redacted nested error wrappers and own-quantum timeout)
  do not accumulate exponential provider backoff. Actual 429/RPC failures
  retain bounded project-local retry backoff. No signing/broadcast retries are
  added. Source integrity conflicts retain revocation semantics.

## Freshness and observability

Completion means every enumerated project succeeded in the SAME generation;
no skip, aging, queue deadline, cached prior success or repeat Active turn can
manufacture full-round health. A latest success earlier in a round is removed
if a repeated turn subsequently fails. Any incomplete round remains closed.

`V3 priority project turn` emits decimal-string project ID, urgency/status,
duration, actual success, finite yield reason, previous-success age, overdue
queue milliseconds and next-due timestamp. No RPC URL, SQL or wallet material.
State is process-local: restart resets scheduling history but not DB cursors,
audit authority, transaction slots or proof generation.

This is a bounded scheduling improvement, NOT a guaranteed 30-second Active
SLA. Cold full audit, provider rate limits, failures, metadata reads and atomic
transaction time may exceed the target. With several Active projects, capacity
must be measured. It does not solve the separate all-project global-freshness
coupling by relaxing that gate.

## Validation and release boundary

Deterministic-clock tests cover Active ordering independent of ID, live urgency
changes, repeat urgent turns vs ordinary fairness, multi-round bounded waiting,
no clipped-tail start, genuine RPC backoff, budget-only yield without backoff,
classification through trackers, partial/revoked/failed rounds, missing metadata,
restart/pruning, exact application-time boundary parameters, and public refund
event promotion until its consumer reaches the event. Real network-none MySQL
8.4 rehearsal exercises the priority Active turn through the unchanged canonical
consumer and confirms rollback preserves all journal/financial/outbox/cursor
facts, alongside existing shared-batch and financial/reorg/CAS regressions.

Source/test only. No upload, deployment, restart, database repair or chain
transaction is authorized. After separately authorized rollout measure Active
success P50/P95/max gaps and lag, ordinary starvation, RPC failure/backoff,
queue delay and full-scan freshness across multiple audit periods. A test target
is not evidence of production SLA compliance.
