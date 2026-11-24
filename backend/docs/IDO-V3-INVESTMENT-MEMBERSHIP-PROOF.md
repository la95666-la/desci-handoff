# V3 investment proof: DB-only approved membership

## Current routed proof

`ProveApprovedV3Wallet` now routes directly to `v3_membership_proof.go`.
It requires the proof feature, authenticated uid/session and verified ownership
of the requested EVM wallet. It reads a non-archived locked V3 binding, selects
the latest approved EXECUTED policy version (disabled versions included), and
uses the existing immutable review/complete leaf validation plus execution
digest reader. It rebuilds the full Merkle tree, verifies snapshot/root/digests
and proves membership for only the requested wallet.

There is NO RPC client or RPC interface on this route. It reads an authenticated
published DB snapshot, computes the wallet's proof locally, then rechecks wallet/
session and selected publication in a fresh DB transaction. No proof cache,
allow-all wallet, Warmup authority or signer readiness is substituted. The budget
is 10s for database work rather than 50s for repeated chain reads. No runtime
config, RPC availability/quota, moving chain head or scanner cursor is required.

The returned version/root is the latest approved EXECUTED publication known to
the DB, not a certificate of current on-chain eligibility. If chain policy has
changed but observation is not recorded yet, an old proof MAY be returned; the
client's current-policy comparison and purchase contract must reject it. This
tradeoff deliberately moves live chain enforcement out of the proof hot path.
The contract remains responsible for state/time/caps/payment and actual current
whitelist version/proof at execution. A DB proof can exist for a Pending/terminal
project but cannot make an otherwise invalid purchase succeed.

Removed from routed investment requests: original creation receipt/history
audit, per-request Safe execution replay, previous policy history replay,
getProject/governance audit, scan cursor/checkpoint and full-tick health checks.
Also removed: per-request chainId/runtime code, implementation storage slot,
contract version, policy/root/version calls and header/reorg rechecks. Those
belong to deployment/publication monitoring and transaction execution.
Those remain publication, indexing, financial/reorg or controlled write concerns,
not prerequisites for proving membership against the current deployed policy.
Proofs may be generated while the scanner advances or has lag; contract checks
still decide whether the user's actual transaction can execute. A proof is not
an applied payment or a financial balance.

## Stationary-read inventory and changes

- Shared `AdmitApprovedLockedProject`: every caller now accepts canonical
  contiguous forward progress; previously only preview/lifecycle variants did.
  A new tip is independently checked against current canonical header/depth.
- Policy preview, first-enable/full-list review and version-update head checks:
  normal monotone growth accepted, backwards head still refused.
- Routed approved investment proof has no cursor checks at all. Legacy proof's
  final frozen-head condition was removed; it retains legacy release fences.
- Retired legacy Safe observation's frozen-head condition removed. Approved
  observation's equivalent was already removed in a1c0622.

Retained comparisons are NOT stationary requirements: pinned height -> hash,
immutable Set/create/approval evidence, unchanged-height hash/revision integrity,
forward revision delta matching height delta, original tx receipt consistency,
exact transaction intent/nonce fence, scanner contiguous publication, consumer
writer CAS and controlled recovery expected revision. Removing these would allow
rewinds, double effects, mixed identities or unsafe transaction recovery.
No V1/V2/Fair Launch write-fence redesign is included.

## Validation / release

Tests cover nonmember/cross-project/root/incomplete-member rejection, wallet
unbinding/session revocation between reads, disabled/newer publication selection,
cancellation and single-member `proof: []` wire format. There is no RPC dependency
to fake; source guards refuse any reintroduced RPC construction/read. Negative
DB mocks retain identity/publication rejection and do not permit cursor/health/
receipt lookups. General admission regression proves
forward progress succeeds for former strict callers while rewinds/halt/rewrite
still fail.

Backend-only source change. No DB migration/config alteration is required.
Local tests are not production deployment or successful real investment.
