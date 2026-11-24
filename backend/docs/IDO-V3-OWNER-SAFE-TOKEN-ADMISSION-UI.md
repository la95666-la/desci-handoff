# V3 token admission — Admin handoff (development candidate)

`/joincare/ido-v3/token-admission` is a **read-only handoff to the external
Owner Safe**, not a wallet/signing page and not a V3 project Set action. It is
limited to BSC mainnet V3 Proxy, USDT payment token and TESTCAT project token.
The Admin API requires authenticated `m2_project:read` permission and exposes
only GET endpoints. It never has a signer, executes a Safe proposal, broadcasts
a transaction or modifies the database. The Owner Safe's actual 2/3 signers
must sign and execute independently in Safe.

To make the live preflight available, a **separate reviewed Core configuration
release** must supply `idoV3TokenAdmission.enabled=true`, the operator-approved
commercial BSC HTTPS read RPC under `primaryRPC`,
and the fixed Owner Safe runtime code hash under `ownerSafeCodeHash`. These
values are not accepted from the Admin request. Do not commit credentialed RPC
URLs or turn on this read gate merely to make a page look green. Missing
configuration, missing historical blocks, unexpected
Safe identity, Proxy/Implementation identity or failed simulation
causes the page to fail closed. The V3 Set, scanner and wallet gates remain
separately disabled.

The commercial provider may return HTTP 429 during a full preflight even when
an isolated `eth_call` succeeds. Token-admission reads in one Core process now
share a bounded request-spacing budget; only HTTP 429 is retried with bounded
backoff under the request deadline. Persistent 429, a changed chain/code
identity, malformed decimals or a failed Safe simulation still fail closed.
This does not alter any Core/Job RPC outside the token-admission handoff.

For this **token-admission page only**, USDT and TESTCAT are pinned to their
reviewed BSC 56 addresses. The page reads each token's `decimals()` and the
V3 Proxy's corresponding `*TokenConfigs(address)` at the confirmed block,
but does not request either token's bytecode or compare token code hashes.
This does **not** alter the separately signed V3 Set/scanner admission rules
or authorize a transaction.

Operator sequence after the read gate is independently admitted:

1. Refresh preflight; check BSC 56, block hash, V3 Proxy, Owner Safe, current
   Safe nonce, 2/3 threshold, token identity and Safe-call simulation. This
   snapshot is not an evergreen permit: signers independently refresh it.
2. In Safe, prepare only `configurePaymentToken(USDT,true)` against the V3
   Proxy, value 0 and `CALL`. Two Safe signers verify the *whole* transaction
   and sign independently. Do not use V2 Proxy, Implementation or relayer key.
3. After the real chain transaction lands, enter its **original on-chain tx
   hash**, not the Safe proposal hash, into the USDT verification field. The
   server checks its signed outer Safe transaction, exact inner call, a single
   Safe success event, a single V3 token configuration event, canonical receipt,
   15 confirmations and confirmed getter. This is a single-provider observation;
   it must not be described as independently corroborated.
   Then refresh the page; if status remains unknown, stop and reconcile the
   original hash, never submit another transaction based only on a UI timeout.
4. Only after USDT is confirmed allowed, refresh nonce and repeat as a new,
   separate Safe operation for `configureProjectToken(TESTCAT,true)`. USDC is
   intentionally not part of this page. A historical success receipt cannot
   overrule a subsequent disable: the verifier checks a current confirmed
   snapshot as well.

This page does **not** certify project `42630852370235393` for V3 Set or
automatically create its binding. The separate signed Set manifest, project
terms, scanner and subsequent financial actions remain outstanding.
