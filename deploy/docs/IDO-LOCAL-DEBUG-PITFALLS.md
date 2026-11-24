# IDO Local Debug Pitfalls — Historical Evidence Only

> **ENTIRE DOCUMENT IS NON-EXECUTABLE IN THE CURRENT RELEASE.** This condensed record preserves incident lessons only. It is not an operations manual. Do not run historical migration, database, Redis, Docker restart, Backend/Job activation, contract deployment, private-key, nonce-repair or chain-broadcast commands. Current release entrypoints intentionally fail closed.

## Preserved lessons

- Treat chain state as authoritative; UI-derived time/status is not a chain terminal state.
- Database timestamps require explicit container-timezone verification.
- Relayer nonce and outbox slots must remain durable and consistent; never use the relayer key for manual transactions.
- Migration ledger success alone is not schema authority; real MySQL 8 information-schema evidence and independently admitted checkpoints are required.
- Public RPC historical capabilities must be verified before event replay or runtime verification.
- Container health, resolved image identity and process identity are distinct facts.
- Recovery or replay must not bypass append-only evidence, finality, runtime identity or reconciliation.

Historical executable procedures were removed to prevent accidental use while Production remains NO-GO.
