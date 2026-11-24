# Fair Launch mainnet unsigned deployment bundle

This entrypoint constructs review evidence only. It does not load a private key,
create a signer, call RPC, estimate gas, sign, or broadcast.

```text
npm run prepare:fair-launch:mainnet -- <input.json> <output.json>
```

The input must come from a separately controlled BSC mainnet observation and
approval process. The repository example is non-authoritative and must never be
used as deployment authority.

Hard gates include:

- exact BSC mainnet `chainId=56`;
- a clean contract worktree and the frozen production compiler settings;
- one observed deployer EOA whose starting/latest/pending nonces are identical;
- contract-backed governance owner and unpause timelock;
- independent governance owner, operator, guardian and unpause-timelock address
  domains;
- contract-backed Safe authorities for treasury, platform fees and project
  proceeds, with fee and proceeds receivers kept separate;
- an explicitly decided platform fee, bound to a non-zero decision digest;
- role and deployer code identities bound to external attestation digests.

The output fixes all nine CREATE transactions in nonce order:

```text
logic library → runtime library → five facets → dispatcher implementation → ERC1967 proxy
```

It includes predicted addresses, fully linked creation bytecode, constructor
arguments, initializer calldata, input hashes, expected runtime code hashes,
aggregate ABI/selector/storage digests, source identity and the ERC-1967
implementation slot.

Every output is permanently marked:

```text
status = UNSIGNED_OFFLINE_REVIEW_REQUIRED
broadcastPermitted = false
```

The output and its SHA-256 sidecar are review inputs only. A future signer
gateway must independently verify the canonical bundle, current chain, nonce,
gas limits, role attestations, approvals and policy caps before any transaction
can be admitted. This command is not that gateway.
