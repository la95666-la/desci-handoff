# Admin V3 policy/status: observation, not execution authority

## Incident

Project `43359438123302913` returned `V3_POLICY_STATUS_UNAVAILABLE` when
opening the whitelist page. Its own cursor initially ran near the public tip,
but global full-enumeration health was `checking`, last success roughly
46 minutes old because another project was halted. Later this project itself
also halted, suspending its original applied creation #62. Both conditions
closed the old status reader independently of whether its chain policy was
readable and unchanged.

The old handler used `ApprovedV3ModeReady`, requiring Core signing readiness,
then a full project admission + global freshness, block-pinned policy/Safe
reads and a second full admission + global freshness (40-second budget).
That is inappropriate for a display which never grants a signature or proof.

## Change and scope

Only Admin `GET /projects/:id/ido-v3/policy/status` is routed to the new
read-only `ReadApprovedV3PolicyDisplayStatusContext`. Review, approval,
Safe handoff/execution, observation and wallet proof continue through their
existing independent gates. No schema, config, nonce or project changes.

- Request cancellation propagates; bounded 120-second budget fits the
  existing Admin 170s / policy proxy 175s limits. HTTP is no-store/private.
- No Core private key or unrelated full-enumeration healthy flag is needed.
- Full original create signature/calldata/applied-event binding, executed
  two-admin Set, canonical original receipt, immutable economics and reviewed
  historical/current runtime remain authenticated.
- Running projects permit independently canonical continuous forward cursor
  progress. A final cheap project/receipt recheck replaces duplicate full
  historical admission. A rewind, identity change or halt/resume fails.
- Halted projects are DISPLAY ONLY: the same original audit accepts only
  their exact suspended creation and unchanged halted cursor/checkpoint.
  The original receipt and stopped anchor must still be canonical. This does
  not resume the project or convert suspended outbox entries to applied.
- Project identity, policy and Owner/Governor Safe code/addresses are pinned
  to a 15-confirmed block. Policy is repeated, final depth and canonical hash
  checked. Head growth is allowed; replacement/unknown/rewind fails closed.

Existing response fields remain; `scanner_state` explicitly reports `running`
or `reorg_halted`. `approval_state` remains `unavailable_for_proof` in all
cases. A matching root/version is a point-in-time chain observation, NOT
approved membership, global scanner health or Safe execution authority.

## Evidence / remaining boundaries

Production reproduction uses explicit opt-in test harness, DB sessions set
`transaction_read_only=1`, no signer and read-only RPC. While global health
was stale and this project halted, a local candidate returned a confirmed
Pending / disabled / version 0 / zero-root observation. This does not mean
the deployed HTTP route is already fixed. Deployment is separately authorized.

Unit tests cover changing head, zero-root initial policy, changed policy,
runtime/Safe mismatch, replaced anchor, canceled calls, missing Set/original
creation, immutable halted revision and route separation from mutation gates.

This deliberately does not fix intermittent scanner halts, shared sender
fences or first-enable/review/Safe availability. Those may still block actual
policy changes. Frontend should label a halted scanner and must not promote
this diagnostic response into mutation/proof authorization. No browser login
or user token is needed or read for reproduction.
