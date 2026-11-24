# Current Release Boundary

> **CURRENT RELEASE — NON-EXECUTABLE / PRODUCTION NO-GO.** All operational release and deployment actions are unavailable. Frontend/Admin/Backend artifacts are archive/offline-verification only; no upload, Apply, restart, upgrade or rollback is admitted. Frontend/Admin release writes return `static_release_admission_unavailable`; protected services and all database, Hardhat, config, package and chain mutation paths remain fail closed. Local status and jc-release limited read-only logs do not grant release authority; the shell log transport is unavailable. Do not reconstruct or run historical commands.

Re-enable operations only after the activation lease, separated database identities, reviewer authority, safe credentials, durable wallet authority, MFA provider and independent production review are complete.

The operator-approved 2026-10-04 application-host `jc-agent` binary-only compatibility upgrade from reviewed commit `9aabb5031de0230f8a8448550ef8c44e292e300e` is an exception to this blanket shutdown. `upgrade-jc-release.sh --component agent` must verify the checksum-bound package, pass dry-run, back up the original, restart only `jc-agent` and verify its running hash. No Job retry, Server/Backend deployment, database/chain change or Fair Launch activation is authorized by this exception; see [the release record](IDO-BSC-MAINNET-DEPLOYMENT-RECORD.md).

The targeted follow-up `5a5e423635b1bdbdc6fd0210ab04df406aa9daf3` gives the first Job deployment a bounded catch-up window under RPC rate limiting without weakening the health probe. Its installation requires separate operator approval; the earlier approval was specific to `9aabb503`. All other operation exclusions remain unchanged.
