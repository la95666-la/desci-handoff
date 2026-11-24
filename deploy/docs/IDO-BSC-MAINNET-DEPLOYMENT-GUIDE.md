# Current Release Boundary

> **CURRENT RELEASE — NON-EXECUTABLE / PRODUCTION NO-GO.** All operational release and deployment actions are unavailable. Frontend/Admin/Backend artifacts are archive/offline-verification only; no upload, Apply, restart, upgrade or rollback is admitted. Frontend/Admin release writes return `static_release_admission_unavailable`; protected services and all database, Hardhat, config, package and chain mutation paths remain fail closed. Local status and jc-release limited read-only logs do not grant release authority; the shell log transport is unavailable. Do not reconstruct or run historical commands.

Re-enable operations only after the activation lease, separated database identities, reviewer authority, safe credentials, durable wallet authority, MFA provider and independent production review are complete.

Exception limited to the independently reviewed `9aabb5031de0230f8a8448550ef8c44e292e300e` **application-host jc-agent binary upgrade** approved by the operator on 2026-10-04: run the checksum-bound upgrade script with `--component agent` after verify-only, dry-run and backup, then verify the running binary and Backend health. This is not Job Apply/retry or a Fair Launch activation lease; all other protected operations remain subject to the boundary above. See [the release record](IDO-BSC-MAINNET-DEPLOYMENT-RECORD.md).
