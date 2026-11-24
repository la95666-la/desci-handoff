# Fair Launch LOCAL-ONLY Deploy transport blocker

Status: **BLOCK — Backend/full browser financial E2E remains NO-GO**

Production/default materialization remains independently closed by
`FairLaunchMaterializationEnabled=false` and returns
`fair_launch_materialization_unavailable` before database access.

The reserved Backend local seam has no non-test constructor. A zero-value
capability returns `fair_launch_local_e2e_capability_required`; the package-only
test capability returns
`fair_launch_local_e2e_transport_admission_unavailable`, also before database
access. No ordinary command can mint local authority.

## Accepted schema authority

Deploy commit `8eef5accc07dcd2540eb7f2d3a469b4e5a7b8431` is the schema authority for
LOCAL-ONLY migrations 0908-0912. The accepted migrations provide the required:

- immutable run/source/project/source-token bindings;
- project, position, governance and policy current/history projections;
- composite application lineage and append-only journals;
- reversal serialization, lineage/journal links, exact-set evidence manifests
  and last-inserted closure;
- typed project and governance ledgers with `uint256.max` bounds and exact
  compensation linkage;
- least-privilege `local_runtime`, `local_admin`, `local_job` and
  `local_ingest` grant policy plus startup-failure cleanup.

Backend v3 entity and privilege specifications mirror that accepted schema for
static review only. They are not registered by ordinary production commands.

## Exact integration blocker

The committed Deploy runtime transport is not host-connectable:

```text
scripts/fair-launch/local-e2e/start.sh
  docker run ... --network none ...
  no 127.0.0.1 MySQL port publish
  no run-owned Unix socket mounted outside the container
```

`apply-schema.sh` can administer MySQL through `docker exec ... mysql`, but a
Backend `database/sql`/GORM process cannot create the required independent
Runtime/Admin/Job/Ingest pools or hold multi-statement transactions and row
locks through that CLI path.

A docker-exec pseudo-driver is forbidden. It would weaken transaction
semantics, expose root/helper authority and bypass the reviewed per-role pool
contract.

## Required Deploy remediation

A new, separately reviewed Deploy transport slice must provide exactly one of:

1. an ownership-bound, run-specific MySQL TCP endpoint published to exact
   `127.0.0.1:<random-port>`; never `0.0.0.0`, `localhost` or `::1`; or
2. a run-owned MySQL Unix socket mounted outside the container.

It must also provide:

- separate mode-600 credentials for Runtime/Admin/Job/Ingest;
- no password in argv, environment, manifest, evidence or logs;
- exact endpoint/socket identity in the run manifest and immutable fingerprint;
- startup bind and ownership verification plus teardown closure;
- independent-pool, transaction, rollback and row-lock integration tests;
- exact `SHOW GRANTS` verification for each connected identity.

Deploy must remain owned by its own lane; Backend must not patch around the
missing transport.

## Safe Backend work retained

- lossless 32-event decoder and frozen Contract-manifest parity;
- immutable typed event/effect manifest and canonical digests;
- caller-supplied ledger semantics removed from `FairLaunchCEWrite`;
- exact 0908-0912 entity/type/column mirrors;
- exact accepted `local_job` privilege mirror;
- production/default and LOCAL-ONLY pre-I/O shutdown tests.

No DB-backed forward, replay, reversal, replacement, local command, MFA
provider, signer or browser authority is admitted until the Deploy transport
slice is independently reviewed and accepted.
