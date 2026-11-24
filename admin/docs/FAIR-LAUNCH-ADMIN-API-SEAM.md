# Fair Launch V1 Admin API Seam

Status: pending Backend integration. The Admin implementation fails closed when these capabilities are absent (`404`, `405`, `501`), disabled, identity-mismatched, runtime-disabled, or runtime-unverified.

All routes are under the existing Admin base and trusted project identity:

```text
GET  /v1/projects/:id/fair-launch/capabilities
GET  /v1/projects/:id/fair-launch/onchain-params
POST /v1/projects/:id/fair-launch/onchain-params/preview
POST /v1/projects/:id/fair-launch/onchain-params

POST /v1/projects/:id/fair-launch/create-onchain[/preview]
POST /v1/projects/:id/fair-launch/activate-onchain[/preview]
POST /v1/projects/:id/fair-launch/end[/preview]
POST /v1/projects/:id/fair-launch/cancel[/preview]
POST /v1/projects/:id/fair-launch/release[/preview]
POST /v1/projects/:id/fair-launch/withdraw-project-proceeds[/preview]
POST /v1/projects/:id/fair-launch/withdraw-platform-fee[/preview]
POST /v1/projects/:id/fair-launch/withdraw-failed-project-tokens[/preview]

POST /v1/projects/:id/fair-launch/pause/:action[/preview]
POST /v1/projects/:id/fair-launch/governance/:action[/preview]
POST /v1/projects/:id/fair-launch/actions/:requestID/execute
```

`:action` is the kebab-case spelling of the typed action enum in `apps/web-antd/src/api/joincare/fair_launch.ts`.

## Capability envelope

The capability response must bind:

- exact `project_id`, `funding_mode=fair_launch`, `contract_version=fair_launch_v1`;
- `available`, `runtime_enabled`, trusted chain/proxy/runtime observation and `runtime_verified`;
- independent contribution/payout/claim pause state;
- display/raw financial state and reconciliation state;
- an action descriptor for each allowed Admin action.

An action descriptor supplies the approval operation, execution domain, capability-declared `execute_route`, enabled/reason state, four-eyes, step-up MFA, three-party separation requirements, and Backend-derived forbidden Admin IDs. The UI does not infer action authority from lifecycle status.

## Set payload

The Set payload is a strict discriminated object containing mode/version, project/payment Token semantic identities, project owner, verified proceeds receiver Safe, sale Token amount, Soft Cap, min/max contribution, UTC window, and vesting bps. It deliberately cannot contain `token_price`, `hard_cap`, arbitrary contract target, signer, nonce, calldata, raw transaction, or private key material.

Preview returns trusted Token/runtime/policy/beneficiary/fee snapshots, display/raw values, required exact deposit, implied minimum successful price as numerator/denominator raw strings, canonical digest, and expiry. Submit repeats the business payload plus the canonical digest.

## Action workflow

Each action uses an explicit three-stage control flow after preview:

1. `POST .../preview` with reason/evidence/timelock semantics where applicable.
2. `POST ...` with the exact preview `canonical_digest` to create the durable action request and four-eyes approval. The subsequent approval decision includes its own step-up MFA evidence reference and verification timestamp.
3. After approval, a distinct executor calls `POST .../actions/:requestID/execute` with fresh step-up MFA evidence and the immutable runtime/policy/source/config generations.

Approval never dispatches the action. The execute transaction enforces initiator, approver, and executor are pairwise distinct before durable Operator submission or external Safe/Timelock observation begins.

Backend must revalidate immutable payload, project state, runtime, policy/config generations, reorg fence, approval freshness, permissions, step-up MFA, and initiator/approver/executor separation. Preview expiry is 15 minutes; approval execution validity is governed by the frozen Backend contract (2 hours). Approval success is not chain success: Admin continues to show durable submission, receipt/finality, applied event, ledger and reconciliation separately.

No Admin route accepts a target contract address and no Admin UI signs or broadcasts wallet transactions. Project Token deposit and beneficiary withdrawals remain projectOwner/Safe wallet responsibilities; Admin only provides trusted guidance and observation.
