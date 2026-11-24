# Fair Launch LOCAL-ONLY Financial E2E Harness

## Authority and current limit

This harness is **TEST ONLY** and is never production authority. It does not enable the production migration entrypoint, activation, release, rollback, or chain mutation paths. `docker-compose.yml` remains empty and migrations 0908-0912 are deliberately absent from production command/release registries.

The Deploy repository supplies the isolated MySQL/Redis substrate and canonical schema. A complete browser-to-ledger run additionally requires the separately reviewed Contract persistent-Hardhat fixture and Backend local-only binaries. Source-level `ExcessTokensRecovered` facts require a pre-existing immutable `fair_launch_source_token_binding_v3` row for the exact run/chain/source/contract/token and decimals; the schema fixture can exercise this contract but is not an admitted producer. Until a separately reviewed bootstrap/runtime authority produces that metadata and the remaining artifacts produce and verify a runtime admission manifest, full financial E2E remains **NO-GO**.

Never use `.deploy.env`, SSH, a public RPC, remote MySQL/Redis, real keys/funds, or `154.26.137.111`.

## Local prerequisites

- WSL/Linux shell with `/var/run/docker.sock` resolving exactly to `/run/docker.sock`.
- Locally cached pinned MySQL image from `scripts/fair-launch/local-e2e/common.sh`; scripts do not pull.
- Local `redis-server`, `redis-cli`, `uuidgen`, OpenSSL, Node.js (static specification test), and SHA-256 tools.
- All relevant production/deployment environment variables unset. The controller rejects them.

The default state root is outside the repository:

```text
/tmp/joincare-fair-launch-local-e2e/<random-v4-run-id>/
```

Override only with an absolute local path, for example `/mnt/d/LA/desci/.local/fair-launch-e2e` in WSL. Directories use mode 0700 and secrets/state/evidence use 0600.

## Preflight/schema verification

```bash
cd /mnt/d/LA/desci/joincare-deploy
bash scripts/tests/fair_launch_local_e2e_schema_static_test.sh
bash scripts/tests/fair_launch_mysql8_local_e2e_schema_test.sh
bash scripts/tests/fair_launch_local_e2e_start_fault_test.sh
bash scripts/tests/fair_launch_local_e2e_redis_secret_test.sh
bash scripts/tests/fair_launch_local_e2e_mysql_secret_test.sh
bash scripts/tests/fair_launch_local_e2e_mysql_transport_test.sh
```

The disposable schema test applies the pinned baseline and 0903-0912 to MySQL 8.4 under `--network none`. On this WSL Docker daemon, bridge DNAT publishing is unavailable, so the persistent LOCAL-ONLY harness uses a run-owned labelled MySQL container with Docker `--network host`, no published-port mappings, and mysqld explicitly limited to `--bind-address=127.0.0.1 --port=<run-random> --mysqlx=0`. Exact IPv4 loopback is the transport security boundary; host-network is only a WSL-local compatibility mechanism and grants no production/general Docker authority. It tests every 0908+ fail-closed gate, reapplies the additive set only while the v3 effect/financial tables are empty, inspects FK/trigger/collation facts, tests append-only behavior, canonical uint256 bounds, exact inverse compensation semantics, final closure rejection of later journal/ledger/link/evidence additions, end-to-end run/source/chain provenance and cross-chain rejection, immutable run fingerprints, and removes only run-owned resources. Migration 0912 holds the named exclusive maintenance lock `jc_fl_0912:<database>` across trigger replacement and rejects reapply once any v3 lineage/journal/reversal/dependency/attestation/checkpoint row exists. Migration 0911 binds lineage placement generation through an ordinary four-column FK and binds predecessor/dependency/cursor facts to the same run, chain, source binding, source, revision, and effect manifest version.

## Start persistent local infrastructure

```bash
unset JC_DEPLOY_RPC JC_DEPLOY_CHAIN_ID JC_DEPLOY_PRIVATE_KEY \
  JC_MYSQL_ROOT_PASSWORD MYSQL_PWD FAIR_LAUNCH_DISPOSABLE_MYSQL_DSN \
  DOCKER_HOST DOCKER_CONTEXT DOCKER_TLS_VERIFY DOCKER_CERT_PATH SSH_AUTH_SOCK
bash scripts/fair-launch/local-e2e/phase.sh start
# copy the printed random RUN_ID only
bash scripts/fair-launch/local-e2e/phase.sh apply-schema <RUN_ID>
bash scripts/fair-launch/local-e2e/grant-negative-test.sh <RUN_ID>
bash scripts/fair-launch/local-e2e/phase.sh status <RUN_ID>
```

MySQL uses tmpfs for `/var/lib/mysql` and `/tmp`. A run-owned helper first reserves a kernel-assigned random socket bound only to `127.0.0.1`, records a temporary mode-600 token-bearing reservation control state, holds it through all pre-launch work, and releases it only on the immediately adjacent Docker launch path. The token-bearing state is deleted after release; only the token-free/no-secret mode-600 `reservation-released.json` evidence remains. Startup resolves and records both the digest-pinned image reference and the local immutable container image content ID, then verifies container ID/name/run+component labels/PID/start time, image ID, expected mysqld command, run-owned mounts, `HostConfig.NetworkMode=host`, absence of Docker port mappings and any separate run network, and the exact `127.0.0.1:<random-port>` listener. It reads server UUID/version independently through the owned container root path and through a short-lived protected host root profile, requires equality, deletes that host root profile immediately, then binds server/database/run identity into the deterministic `JC_FL_MYSQL_TRANSPORT_V1` digest before reporting ready. Root administration remains only inside the container through its mode-600 root profile; root credentials/config are never handed to host consumers.

After schema and observed-grant verification, all four host profiles are built and connection-verified in a private mode-0700 staging directory. Schema evidence is built and JSON-validated through a mode-0600 temporary file. Credentials/evidence remain not-ready until a final atomic readiness marker cross-binds the infrastructure evidence digest, schema evidence digest, image and transport identities, all four protected profile digests, baseline/migration/schema/grant and owner-label digests. `status.sh` revalidates that complete cross-binding before reporting ready. Failed publication removes the staged/live profiles, schema evidence and marker while state remains explicitly not-ready. The final profiles are published under the mode-0700 run directory:

```text
<run-dir>/credentials/mysql-runtime.cnf
<run-dir>/credentials/mysql-admin.cnf
<run-dir>/credentials/mysql-job.cnf
<run-dir>/credentials/mysql-ingest.cnf
```

Each profile is mode 0600 and contains only the role-specific MySQL client settings: exact `host=127.0.0.1`, random `port`, `protocol=tcp`, exact run database, role username, password, disabled TLS for this loopback-only disposable transport, caching-SHA2 server-public-key retrieval, and a bounded connect timeout. The Go probe uses the pinned `go-sql-driver/mysql` caching-SHA2 RSA exchange with `tls=false`; it does not enable `allowCleartextPasswords`. Consumers pass the profile path as `--defaults-extra-file` or parse the protected file internally; they must never produce a password-bearing DSN in argv/environment/log/evidence and must never use `MYSQL_PWD`. Disposable Runtime/Admin/Job/Ingest passwords are generated as mode-600 files and streamed to the container over stdin, never placed in argv or environment. Redis authentication is likewise supplied to `redis-cli --askpass` through protected stdin from a dedicated non-traced helper; the parent startup shell never reads or expands it, including when invoked with inherited `bash -x`. It is absent from argv, environment, process listings and retained evidence. Exact per-identity grants are applied, normalized from `SHOW GRANTS`, compared to the repository policy, and the digest is computed from observed grants rather than the policy source file. The host transport test runs an actual WSL host Go `database/sql` process using the Backend module's already-pinned MySQL driver as a read-only dependency consumer. It proves four independent role pools and connection IDs, exact `CURRENT_USER()`/database/server identity, multi-statement commit and rollback, bounded two-session `SELECT ... FOR UPDATE` blocking/serialization, accepted `local_job` mutation and exact Runtime/Admin/Ingest denial, `FOREIGN_KEY_CHECKS=1`, and zero listener/container/network/run-directory residue after teardown. Any observed `WITH GRANT OPTION` is rejected rather than normalized away. Redis binds only `127.0.0.1` on a random run port with a random password. RPC/core/admin/frontend ports are reserved as exact loopback URLs in run state; the Contract/Backend lanes own their actual processes.

## Chain and runtime admission handoff

The Contract fixture must start persistent Hardhat with `chainId=31337` at the exact printed `http://127.0.0.1:<RPC_PORT>` string. After admission, `localhost`, `::1`, another port, or any alternate spelling is a different identity and must be rejected.

The chain manifest is separate from the database/runtime manifest. It must be TEST ONLY and bind at least: run ID, exact RPC URL, chain ID, genesis hash, deployment observation block/hash, proxy/dispatcher/assets/Safes, deployment digest, ABI/selector/storage/code digests, and fixture source commit/tree digest. It must not invent a database fingerprint before schema/grants/source-registry verification.

The Backend local-only admission step must then independently consume and validate the exact transport/state/profile artifacts and bind the same run identity plus database/schema/grant/source-registry digests, source/revision IDs, contract/version/implementation code identity, placement/materialization generations, finalized coverage, and an expiry. This Deploy transport PASS is not Backend runtime admission: Backend materialization, persistent Hardhat, Admin writes, Frontend/MetaMask writes, browser financial E2E and Production all remain BLOCK/NO-GO. Production/default Backend binaries must continue returning unavailable capabilities.

## Scenario checklist (after all other lanes are admitted)

For each transaction record tx hash, block hash/number, transaction index, `tx_log_ordinal`, RPC `logIndex`, placement generation, application lineage ID, typed manifest digest, journal digest, ledger digest, and post-state digest.

1. Successful round: deposit project tokens, contribute, finalize successful, claim, pay project proceeds, pay platform fee, withdraw remaining project tokens.
2. Failed round: contribute, finalize failed, refund; no claim/proceeds/fee facts.
3. Cancelled round: contribute, cancel, refund; no claim/proceeds/fee facts.
4. Replay: replay the same `(chain_id, tx_hash, tx_log_ordinal)` and prove no second active effect or duplicate typed ledger fact.
5. Reorg: snapshot, apply a finalized branch, revert/mine replacement, observe placement generation increase, compensate descendants in dependency order, restore predecessor cursor, and reapply replacement canonical placement atomically.
6. Reconciliation: ordinary reads expose only active finalized canonical typed facts; audit reads include all placements, lineages, reversals and attestations and report zero unexplained drift.

`tx_log_ordinal` is immutable event identity; RPC `logIndex` is placement metadata only. Financial facts must not materialize before finality.

### Exact Backend leaf-reorg transaction contract

The Backend repository must treat these table contracts as one database transaction, never as independent best-effort calls:

1. Lock the active guard, affected projections and predecessor cursor/checkpoint rows.
2. Insert one reversal `fair_launch_application_lineage_v3` row whose `reverses_lineage_id` points to the applied forward lineage and whose event/effect/source revision provenance is verified equal by Backend before insert.
3. Insert exactly one `fair_launch_lineage_reversal_v3` pair per forward lineage. Primary/unique keys prevent a second reversal of the same forward lineage and reuse of one reversal lineage.
4. Lock the durable `fair_launch_reversal_serialization_guard_v3` row shared by every set-changing insert. Insert one `fair_launch_journal_reversal_v3` row for **every** source journal row and exact `fair_launch_reversal_evidence_item_v3` rows for journals, dependencies, attestations, cursor checkpoint, governance/policy history and original ledgers. Insert the immutable evidence manifest before closure. The source-journal primary key prevents duplicate linkage. Before commit, insert `fair_launch_reversal_closure_v3`; its trigger locks the same guard, proves evidence/actual-set equality and source/link plus source/compensation equality, including one and only one compensation for every original project and governance ledger row, and rejects duplicate-generation masking or any omission.
5. Restore projections from journal `before_payload_jcs` in reverse dependency/mutation order. Insert compensation project/governance ledger rows through the reversal lineage, each with `reversal_of_ledger_id` referencing its original row. Database triggers require the exact inverse tuple: paired lineage and run/source/chain, project/contract/token/subject/address dimensions, decimals, raw and display amounts, approved reversal entry type, and the reversal placement generation.
6. Transition the mutable active-effect guard, insert effect/ledger/mutation attestations and the explicit predecessor cursor checkpoint, restore the cursor, and (where applicable) apply replacement canonical placement.
7. Insert closure last and commit. Any failure rolls back guard/projections/links/compensation/checkpoint together. Once present, closure is final: the database rejects any later source or reversal journal, forward or compensation ledger, journal link, or lineage reversal addition that could change the proved set; unrelated lineage remains unaffected.

Forward lineage, source journal, both reversal-link tables, closure, typed ledgers, histories and attestations reject arbitrary UPDATE/DELETE. Evidence uses a reproducible SHA-256 chain: seed `SHA256("JC_FL_REVERSAL_EVIDENCE_V1" || u64be_hex(forwardLineageId) || u64be_hex(reversalLineageId))`; each ordered item hashes `"JC_FL_REVERSAL_EVIDENCE_ITEM_V1" || previousDigest || u64be_hex(ordinal) || u32be_hex(byteLength(kind)) || HEX(kind UTF-8 bytes) || u64be_hex(id) || evidenceDigest`. The manifest hashes domain `JC_FL_REVERSAL_MANIFEST_V1`, both lineage IDs, eight u64 count fields, total item count, and final chain digest. Closure hashes domain `JC_FL_REVERSAL_CLOSURE_V1`, lineage IDs, manifest digest, all twelve closure u64 counts, and UTC `YYYY-MM-DDTHH:mm:ss.ffffffZ`. SQL uses fixed-width uppercase HEX numeric fields and lowercase SHA-256 output; Backend must reproduce these bytes exactly. `fair_launch_local_e2e_run` fingerprint fields are immutable; only the explicit `created -> verified`, `created|verified -> expired`, and `created|verified|expired -> closed` transitions are accepted. Source/project bindings use composite source/run chain foreign keys that enforce chain 31337. Local Job grants therefore require `SELECT, INSERT` on evidence tables; current projections/cursor ingestion state may receive the narrowly required update operations. The reversal serialization guard is `SELECT`-only for Job: forward-lineage, reversal-link, and closure triggers create/bind/close it, and its transition validator requires the corresponding immutable link/closure row to be visible. The source-token binding is also `SELECT`-only for Job and append-only for its separate bootstrap authority; Job cannot mint, update, or delete token metadata. Governance recovery ledgers use `asset_role=governance_token`, carry the stable `token_binding_id`, and bind token address/decimals through the composite admission FK. The production grant policy is unchanged; `scripts/fair-launch/local-e2e/grant-policy.js` is a local-only contract that the Backend lane must mirror without widening evidence privileges.

Reversal repository contract: `fair_launch_application_lineage_v3` and `fair_launch_effect_journal_v3` are immutable forward evidence. Do not update lineage `state`/timestamps or a journal reversal column. In the same reorg transaction that writes compensation facts, insert exactly one `fair_launch_lineage_reversal_v3` row and one `fair_launch_journal_reversal_v3` row per reversed journal. Primary/unique keys make linkage one-time; the validation trigger requires an applied forward lineage and an applied reversal lineage whose `reverses_lineage_id` points back to it. Effective reversal status is read by joining these link tables. Job access to all four evidence tables is `SELECT, INSERT`, never broad `UPDATE`/`DELETE`.

## Teardown and residue proof

```bash
bash scripts/fair-launch/local-e2e/phase.sh teardown <RUN_ID>
docker -H unix:///var/run/docker.sock ps -a \
  --filter label=joincare.fair-launch.local-e2e-run=<RUN_ID>
# output must be empty
ss -lnt | grep -E '127[.]0[.]0[.]1:(<RPC>|<REDIS>|<CORE>|<ADMIN>|<FRONTEND>)' || true
# no run ports may remain
```

Startup installs an EXIT cleanup trap plus separate terminating INT/TERM handlers immediately after minting the run ID and retains them through atomic transport state/evidence publication; completion and trap removal are the final operations. Each finite automation lifecycle phase installs a fixed self-bound (`start=240s`, `apply-schema=280s`, `status=60s`, `teardown=120s`). `backend-serve` is intentionally not wrapped in the automation phase timeout: it is an admitted interactive HTTP service and has no separate 30-minute HTTP lifetime. Its HTTP authority inherits the immutable run admission expiry (at most 24 hours), fail-closes every request against that capability, and remains explicitly stoppable by SIGINT/SIGTERM and exact run teardown. For bounded phases, the recursion marker is accepted only when `/proc/$PPID` proves the root-owned, non-group/world-writable timeout executable resolved from the system default PATH and an exact NUL-separated argv sequence of `timeout --foreground --kill-after=5s <bound> bash <canonical-phase-script> <exact-child-args>`. Unordered membership, extra arguments, alternate bounds, and caller-controlled same-basename executables are rejected, so an inherited/caller-forged marker cannot bypass the bound; `phase.sh` creates the same mechanically verified parent contract rather than being an optional safety convention. Failed-start no-ID discovery removes a container only after exact name, run+component labels, pinned local image content ID, host/no-publish networking, loopback/port/mysqlx command and both run-owned mounts verify. Reservation controller+worker and Redis are signalled only under exact PID/start fingerprints and boundedly checked for exit; exact listeners are checked closed. Any ownership ambiguity, removal failure or residue retains the mode-700 run directory with mode-600 bounded failure evidence. Fault coverage includes port reservation, host-network container launch, endpoint discovery, transport state/evidence publication, and schema credential publication. Status and teardown fail closed unless the exact immutable transport fingerprint, labels, publish, listener, server identity and (when schema-ready) all four protected profiles verify. Teardown rechecks ownership immediately before destructive removal, removes the exact container before the now-empty exact network, and refuses unrelated or tampered resources while retaining bounded failure evidence. Redis ownership is a mode-600 fingerprint of PID, `/proc` start time, executable path and inode, exact run config path/digest and loopback endpoint. Status, failed-start cleanup and teardown all verify that fingerprint. Teardown re-verifies immediately before signaling, bounded-waits for the exact PID/start identity, proves the exact endpoint is closed, then removes the labelled MySQL container and run state. Ownership or residue failure returns nonzero and retains safe failure evidence; unrelated processes are never signaled.

A local PASS is evidence for this disposable run only: it is not production schema admission, release approval, or deployment approval.
