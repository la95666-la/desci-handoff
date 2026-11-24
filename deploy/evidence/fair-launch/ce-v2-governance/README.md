# CE-V2 and governance DDL draft

`schema.sql.txt` is intentionally outside `migrations/`, uses a `.sql.txt`
suffix, and is mechanically non-executable: after the two-line sentinel/header,
every nonblank line starts with `SPEC> `. The verifier removes that prefix only
in memory for static inspection and never submits the text to a database.

The specimen covers the immutable Source/revision registry, chain/source
cursors and coverage, CE-V2 identity/placement/observation/conflict/application
lineage, shadow ledger, M5/M6 gates and final-event FK. Governance records form
an explicit approval → Safe nonce candidate → Timelock operation/calls →
executor/external receipt → target application chain.

`state-machine-vectors.json` contains named structured fixtures rather than an
inventory count. It executes all seven M5 equivalence families, submission/event
identity and sender/calldata/nonce convergence, ledger placement provenance,
Safe external-hash/reorg evidence, and governance canonical/finalized convergence.
The evaluator computes per-family SHA-256 result rows in the frozen order,
binds the validation-query version and cutover block/hash, then verifies the
aggregate cutover digest. Mutation negatives cover stale query versions, row and
aggregate digest corruption, every equivalence family, and each prior lineage/finality bypass.
Seven-family fixtures contain row-shaped legacy/V2/application/failure/ledger/coverage/receipt evidence; the evaluator derives each invariant rather than hashing caller assertions. Financial rows use canonical decimal strings above `2^53`; aggregation uses `BigInt` for ledger/project/wallet/entitlement buckets. Each fixture carries non-null reviewer-fixed expected/actual/evidence-root/result-row digests and the ordered aggregate digest, recomputed with the same colon/pipe serialization as the inert DDL gate. Safe vectors recompute the frozen Safe v1.4.1 EIP-712 transaction hash and separately bind the outer execution transaction, chain, CE application identity, canonicality and finality. Timelock vectors recompute OpenZeppelin-compatible `hashOperationBatch` from ABI-encoded ordered calls, predecessor and salt, with reorder/calldata/predecessor/salt negatives. The schema also binds operation recomputation and each call payload to its scheduled/executed application effect digest. The verifier uses `ethers` from the isolated contract audit dependency tree. `verify.js` checks
the corresponding composite FKs and versioned aggregate-digest gate before it
runs the transition evaluator.

Run:

```bash
node evidence/fair-launch/ce-v2-governance/verify.js
```

This remains a reviewed schema specimen, not an executable migration. Production
is still blocked on M0 inventory, exact legacy type/collation matching, zero
quarantine evidence, disposable-MySQL validation and a full M5/M6 rehearsal.
