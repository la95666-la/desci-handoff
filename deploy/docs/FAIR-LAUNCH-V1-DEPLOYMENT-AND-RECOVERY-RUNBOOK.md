# Fair Launch V1 Deployment and Recovery Boundary

> **CURRENT RELEASE — NON-EXECUTABLE / PRODUCTION NO-GO.** Repository-local migration, transformation, candidate collection, admission, activation and recovery operations are unavailable.

Only an external reviewer-owned collector may eventually observe target-equivalent MySQL 8 state and produce a candidate. The repository collector intentionally returns `fair_launch_candidate_collection_unavailable` and emits no candidate. No candidate, schema snapshot, checkpoint, reviewer root or activation lease is currently admitted.

Frontend and Admin static artifacts are archive/offline-verification evidence only; no upload, Apply, restart, upgrade, rollback, installation or other release operation is available. Backend, Job, Lang, TGBot, Hardhat and every database or chain mutation remain unavailable.
