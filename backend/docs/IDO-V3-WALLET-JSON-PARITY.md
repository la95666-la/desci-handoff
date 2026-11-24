# V3 wallet JSON parity: post-transaction empty-window halt

## Evidence and scope

Project `43368658176704510` on Job source `a645229` successfully consumed
purchase `0x6b0fcb4d995430c40fe97bf1e87e868be187c1a9355e948a35300b7201181670`
at block `130106558`. Both read providers verified a successful canonical receipt.
The next empty window `130106559–130106686` failed at `commit_empty`, with
an active context and no recorded RPC/MySQL source error, then revoked publication.

`replayV3ProjectTx` used `actual != *expected`, comparing `Wallets` as raw text.
Go emits compact JSON; MySQL's native JSON column returns normalized formatting.
After a nonempty wallet appears, valid equal content can differ byte-for-byte.
The deployed `commit_empty` log did not identify its inner return, but the source,
read-only MySQL representation check and disposable regression establish this
concrete deterministic defect. It does not explain every historical pause.

## Fix and security boundaries

Use the same semantic projection comparator for read-only replay parity and full
coverage audits. Wallet JSON is decoded using a strict token reader:

- root and wallet values must be objects; reject duplicate wallet/field keys;
- wallet keys must be canonical lowercase nonzero full addresses;
- exactly payment/tokens/refunded/claimed, each a canonical decimal uint256 STRING;
- reject missing/unknown fields, null, arrays, numeric amounts, leading zeros,
  negative/overflow amounts and trailing JSON;
- ignore only object key order, legal JSON whitespace and equivalent string escapes;
- preserve exact strings and all aggregate counters/state; no float64 conversion.

The previous coverage comparator already ignored JSON order, but decoded into
`any`. It is now tightened to this same schema, avoiding duplicate-key acceptance
or numeric precision surprises. No canonical/raw evidence digest changes: public
coverage payloads are deliberately LONGTEXT with immutable byte digests. Journal
event comparisons already decode into typed structs. No other raw wallet JSON
equality was found in the V3 scanner/recovery paths.

The full journal replay, checkpoint checks, project identity, block counters,
financial/outbox transaction boundaries, epoch/revision CAS and reorg closure
remain intact. There is no migration, config change or new exception to pause
guards. Empty-range logs now distinguish replay/project-counter/creation and
checkpoint publication stages without logging financial or wallet payloads.

## Other transactions

All financial/lifecycle events share `applyChecked` -> replayed projection ->
subsequent empty-window validation. A nonempty wallet remains present after refund
or claim; therefore those flows could reproduce the same bug. Proceeds, fee and
token withdrawals do not create a wallet themselves but also fail this check if
a wallet already exists. Deposit/activation without any buyer leave `{}`, which
masked the original missing test coverage.

Disposable network-none MySQL 8.4 scenarios:

| History prefix | Follow-up |
|---|---|
| purchase | two prepared 128-block empty windows |
| purchase, successful end, claim | same |
| purchase, cancellation, refund | same |
| purchase below soft cap, failed end, refund | same |
| purchase, successful end, proceeds withdrawal | same |
| proceeds then platform fee withdrawal | same |
| purchase, successful end, unsold tokens withdrawal | same |

Each uses real journal decoding/application and MySQL JSON round-trip; confirms
byte mismatch; then exercises the production prepared consumer. Each corrupt
wallet counter and all eight aggregate counters are rejected without advancing
cursor/checkpoints or altering financial/journal/outbox facts. Large token values
exceed JS integer precision. Unit cases cover exact amount differences beyond
2^53, malformed/duplicate/unknown JSON, precision, counter parity and key order.

These are scanner/materialization regressions, not live on-chain refund/claim
transactions or a full browser/Safe E2E. No production data is copied into tests.
Packaging authorizes local Job and matching Backend artifacts only; no Admin or
Frontend source changes, no upload/deploy/recovery are implicit.
