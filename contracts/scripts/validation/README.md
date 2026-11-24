# Fair reminders: local fixtures are not approval

`fair-launch-local-deployment-manifest.json` and `fair-launch-local-upgrade-manifest.json`
are deterministic implementation-tree fixtures for local regression tests only. They are
explicitly marked `local-non-authoritative-fixture` and MUST NOT be treated as reviewer
approval, release evidence, or a trust root.

The production validators fail closed unless a signed approval file is supplied outside this
implementation repository:

- deployment: `JC_FAIR_LAUNCH_APPROVAL_FILE`
- upgrade: `JC_FAIR_LAUNCH_UPGRADE_APPROVAL_FILE`

Production trust authority is not caller-selectable. The validator reads the fixed protected
reviewer authority configuration from:

- Windows: `C:\ProgramData\JoinCare\fair-launch\reviewer-authority.json`
- Unix: `/etc/joincare/fair-launch/reviewer-authority.json`

The Windows location is a literal absolute machine-wide path. The validator does not expand or
read `ProgramData`, `PROGRAMDATA`, `ALLUSERSPROFILE`, or any Fair Launch authority override from
the process environment in production mode. CLI/options cannot replace the production authority.

That config pins the reviewer authority ID, Ed25519 key ID and public-key SHA-256 digest, and
points to a separately distributed signed reviewer state. All authority, key, state, current
approval and previous approval paths are resolved through `realpath`; symlinks, non-regular files,
and files whose resolved path is inside this repository are rejected.

Production admission also requires machine ACL/ownership verification for the authority config,
public key, signed state, current approval and previous approval. The complete fixed machine trust
chain is checked before, during, and after the multi-artifact validation: Windows checks the drive
root, `C:\ProgramData`, `C:\ProgramData\JoinCare`, `fair-launch`, all subdirectories and leaves;
Unix checks `/`, `/etc`, `/etc/joincare`, `fair-launch`, all subdirectories and leaves. On Windows,
each component must be owned by `SYSTEM`, `Administrators`, or `TrustedInstaller`, cannot be a
reparse point, and no other principal may have write/create/delete-child/delete/ownership or
ACL-control rights. On Unix, every component must be root-owned, non-symlink and not group/world
writable. Directory identity and ACL snapshots are compared across all authority artifacts to
reject ancestor replacement during validation. If the platform ACL cannot be read or securely
established, validation fails closed.
Provisioning and independently recording these ownership/ACL facts is a deployment gate; the
validator does not create or repair trust files or permissions.

The v2 domain-separated approval binds the exact `0.8.22+commit.4fc1097e` compiler identity,
optimizer/viaIR settings, current Git commit, computed reviewed source-file tree digest,
ordered dispatcher + five facets + two libraries, runtime code hashes, named ABI digest,
selector-map digest, storage-layout digest, full-set digest, monotonic generation, previous
signed-approval digest and previous manifest digest. The signed external reviewer state binds
the actual previous signed approval and minimum generation.

Production entrypoints categorically reject `JC_FAIR_LAUNCH_VALIDATOR_TEST_MODE`, every
`JC_FAIR_LAUNCH_TEST_*` variable, and any test-authority result with a nonzero exit. Test authority
is available only through the separate `validate:fair-launch:deployment:test-only` and
`validate:fair-launch:upgrade:test-only` entrypoints. It has a distinct fixed identity and reports
`productionAuthorized: false`; it can never authorize deployment. Legacy caller-provided
golden-digest or public-key environment variables are ignored.

`fair-launch-abi-unnamed-policy.json` is the explicit reviewer-approved ABI naming policy for
golden fields intentionally left unnamed. It provides the authoritative production name for
each such field. All other names compare exactly. `internalType` normalization only maps the
known audit-baseline and production Shared namespaces to one canonical namespace.
