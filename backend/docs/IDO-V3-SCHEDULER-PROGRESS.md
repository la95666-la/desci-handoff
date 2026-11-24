# V3 effective-progress scheduling

Source/test only. No production restart/config/mode change, migration, release or
chain broadcast. Based on1dcd010; includes its maintenance-backlog change and the
27aeb9b whitelist request optimization. Backend/Job must be packaged separately.

## Observed failure

v180 normal terminal projects received an opportunity every4..5min, but roughly
half were successful full-history audits followed by audit_ready_yield. A512-block
consumer batch every8..10min cannot track a~2.22 blocks/s producer. The fairness
counter protected attempts, not useful progress. Pending lifecycle was also
accidentally copied from tracking.Urgent into the unresolved-transaction lane.

## Changes

- Tracking keeps lifecycle Urgent for maintenance safety. A separate DB-only
  Unresolved output covers actual transactions/nonce fences/dead letters; only
  it and recovery promote a Pending project's scheduling hint. Active, boundary,
  own events and real unresolved work retain priority. Pending ordinarily uses
  its45s scheduling interval, not a fictitious pending-transaction reason.
- Urgent projects do not join the normal pool before their30s due time. Unlocked
  Set polling honors its2min interval rather than spending normal opportunities
  on every public-tip increase. Backlogged locked projects stay eligible.
- Full audit success reserves ONE next normal consuming opportunity, across
  clipped rounds. It does not forge lastSuccess/freshness. Errors consume the
  reservation; expired/changed grants still fail closed. Active work preempts
  between atomic turns under the existing burst/aging rule.
- Normal ordering accounts for successful service/observed cursor advancement.
  Repeated zero-progress yields cannot pin peers; after180s they regain an aged
  opportunity, avoiding permanent starvation. Never-attempted peers are serviced.
- Known backlog gets a35s reservation from the outset; a successful turn resets
  old escalation. Total tick50s, project35s, consumer15s and publication8s reserves
  remain. No concurrent project writers or longer timeouts are introduced.
- Beyond the unchanged660-block freshness bound, an internal scheduler-only
  catchup context may prepare at most1536 covered heights per atomic window.
  Ordinary shared windows remain512 and legacy windows128. The same32MiB payload
  cap, per-height digest/parent/log/receipt checks, project/global routing,
  end-state/replay parity, audit grant, epoch, locks/CAS and canonical anchors
  apply. Unknown or eventful work still takes its checked replay path.
- Within the same bounded audited turn, at most3 windows may commit separately,
  with a15s floor before continuing. Every window rechecks the original grant;
  partial progress is real, but a later error is returned, never hidden as success.
  The scheduler records observed progress separately from operation success.

## Validation boundary

Deterministic throughput model: two urgent projects, six terminal projects with
6000-block backlog, three Sets,2.22blocks/s Producer,18s full audits,10s hot checks,
6s per window,5s cron gap,50s round/35s turns and8min audit expiry. All six backlogs
must decrease; Active service gaps are bounded by the model assertion. This is
not a production throughput guarantee or a hard SLA under arbitrary project counts,
event density or provider latency. No finite scheduler can hide capacity overload.

Real network-none MySQL compares1536 catchup to12x128 reference windows for exact
cursor/checkpoints/journal/effects/projection/outbox parity, in addition to prior
financial-event/closure/maintenance/reorg/audit-grant regressions. Tests also cover
audit-followup failure fairness, budget reset, repeated-window cap, caught-up/short
budget stop, later failure propagation and private catchup scope.

No changes to300s/660-block project freshness or public90s/120/135 limits, approval,
Safe, financial ownership, sender evidence or nonce/outbox rules. A partial round
still cannot publish a complete global-round success. New finite logs expose
advanced, audit_followup and no_progress_turns without SQL/RPC secrets.

## Rollout

Job behavior requires a new independently verified Job release. The shared
tracking reader change is additive/no schema change; Backend can retain its
existing payload contract. If deploying the inherited maintenance admission fix,
upgrade Backend and Job separately and preserve enabled production feature flags.
Acceptance must measure per-project advancement vs Producer growth, audit-only
turn ratios, Active delay, source health, nonce/outbox invariance and DB/RPC cost.
Do not declare success based only on readiness200 or simulated throughput.
