# Fair Launch mechanical freeze evidence

This tree contains review-only mechanical artifacts for
`IDO-FAIR-LAUNCH-DEVELOPMENT-PLAN-SECURITY-AUDIT.md §18.4`.

It is **not** a production migration bundle, deployable contract bundle,
release authorization, signer POC, or mainnet readiness proof.

Contents:

- `contract-baseline/`: ABI/selectors/topics/errors/storage layout generated
  from the isolated Solidity 0.8.22 audit specimen.
- `ce-v2-governance/`: non-executable DDL text and state-machine vectors.
- `phase1-project-read-model/`: non-executable discriminator/read-model draft;
  it is not a migration or authorization to backfill/cut over production data.
- `release-rfc8785-v1/`: detached trust-record schemas, canonical vectors and
  offline verification tools.

Run:

```bash
bash scripts/tests/fair_launch_phase1_policy_test.sh
bash scripts/tests/fair_launch_mechanical_evidence_test.sh
```
