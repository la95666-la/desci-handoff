# IDO V3 M4 C-end development status

M4 implements an **independent, default-disabled** V3 wallet path. This is a
development checkpoint, not permission to deploy, migrate data, fund a project,
or enable production wallets. M0/M1 governance and chain-specific finality
approval, real V3 deployment identities, and M5 cross-repository acceptance
remain separate gates.

## Boundaries

- `capped_presale` + `ido_v3_stablecoin` + exact chain, locked project ID,
  distinct proxy/implementation and bytecode hashes, token addresses/decimals,
  V3 ABI and on-chain `contractVersion()` are mandatory. Unknown identities
  never fall back to V2; generic V3 write methods reject routing.
- V3 writes require the verified bound wallet, a fresh project detail, a pinned
  independent chain RPC, a live policy/status read, exact-call simulation/gas,
  and a final account/policy check before one wallet submission. Whitelist
  purchases obtain a new per-wallet proof; public purchases use the proof-free
  V3 method. Warmup signup has no proof authority.
- A per-project/account browser Web Lock and persistent local fence cover a
  wallet prompt, broadcast uncertainty, receipt timeout and page reload. The
  original hash, transaction destination/from/chain/calldata digest, canonical
  block and configured confirmations must reconcile before removing a fence.
  The V3 sender uses a single raw wallet API and never falls back to a second
  send channel when the provider reports an error.
  An unknown wallet result with no hash cannot be automatically retried; it
  requires independent investigation, **not** another send.
- Owner Pending project-token approve/deposit and successful claim or
  failed/cancelled refund use the same scoped path. Dashboard links V3 owners
  to the dedicated project panel; it does not reinterpret V3 as a V2 portfolio.
  V2 new purchases/approvals are closed when the V3 wallet feature is enabled,
  while historical V2 claim/refund remains available.

## Build-time gate (all required to enable)

`VITE_IDO_V3_WALLET_ENABLED=true`, `VITE_IDO_V3_PROOF_ENABLED=true` for
whitelist purchases, `VITE_IDO_V3_CHAIN_ID`, `VITE_IDO_V3_PROXY`,
`VITE_IDO_V3_IMPLEMENTATION`, `VITE_IDO_V3_PROXY_CODE_HASH`,
`VITE_IDO_V3_IMPLEMENTATION_CODE_HASH`, and
`VITE_IDO_V3_CONFIRMATIONS` (approved integer 2–100) must be pinned before a
separately authorized build. `VITE_ENVIRONMENT` must match the chain; local
31337 additionally requires `VITE_IDO_V3_LOCAL_RPC` on 127.0.0.1. No real
production V3 address or approved confirmation depth is supplied here. Never
put RPC tokens or private keys in these public Vite variables.

## Development validation

`npm run test:unit` (246/246), `npm run build`, JS syntax checks and
`git diff --check` passed after the M4 changes. The unit suite covers routing,
proof/root binding, owner/lifecycle authorization, pending/receipt failure,
confirmation depth and a replaced block. A build with the gate off does not
exercise a real wallet, real Safe execution, chain reorg recovery or financial
indexing. Those remain for M5 in an isolated Hardhat/MySQL environment.
