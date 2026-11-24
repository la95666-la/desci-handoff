# IDO Job phase 2 — offline review boundary

> 2026-10-03 update: production app-host identity was independently confirmed, a current DB backup was isolated-restored, and the 10-02 migration was separately authorized and verified applied. The Job signer was replaced; its latest public address is `0x4434c744708656783D42ea6463A1fe52Dc8F78a2` (previous signed identities are stale). Deploy commit `4fb0c3d` removes the need for Core private material on the Job host; its review-only Agent is not installed. Read-only public BSC observations identified the documented proxy/operator and implementation, but the tested public RPC returned `-32005 limit exceeded` on recent `eth_getLogs`. Historical RPC capacity, deployment start block, Core control and nonce/outbox evidence, complete runtime configuration and signed receipts remain unproven. No Job service, production upgrade or startup has been approved. Historical inventory below is retained as the evidence available at that earlier checkpoint, not today's production state; see [preflight addendum](SECOND-PHASE-PREFLIGHT.md).

**Development candidate only; production NO-GO.** The package made by
`scripts/build-job-review-candidate.sh` includes Linux static Server/Agent
binaries and a checksum manifest but deliberately excludes an upgrade script,
secrets, Compose, signed release declaration, and production authorization.
`REVIEW.txt` contains the repository HEAD for orientation, not a source commit
attestation: the worktree is dirty. Never run `upgrade-jc-release.sh` from the
earlier `production-core-services-job-runtime-disabled` package as a Job
upgrade. Before any real upgrade, review and create a separately approved
source-provenance-bound, rollback-tested Server/Agent release.

The earlier offline review binaries predate the protected-inode and Compose
binding changes below and are **not** a hash of the current worktree. Do not
treat those packages as the final candidate. The current offline review bundle
contains `SOURCE-SHA256SUMS` for tracked and untracked, non-ignored
`tools/jc-release` files; verify it from the repository root with
`sha256sum -c`. It binds review inputs but is not a clean source commit,
external provenance attestation, or permission to install. Files outside this
tool (including backend, migrations and deployment configuration) are not in
that inventory. A formal release requires full multi-repository provenance,
reviewed boundaries, reproducible build and separate authorization.
The builder snapshots source hashes before Go tests/build and compares the
inventory after compilation, failing if inputs changed; ignored files, Go
toolchain provenance and outside-tool release dependencies are not attested.

### Local audit addendum

The Job strategy now rejects an existing `jc-job` unless its Docker Compose
service label is `job` **and** its project working-directory label equals the
configured deployment root. Apply and health checks repeat the binding check;
first-deploy cleanup and later rollback check the attempt label plus project
working directory before touching a replacement container. A foreign/legacy
container is a blocker, not a rollback artifact to adopt. An isolated WSL
Docker Compose `config --format json` fixture confirmed the image override,
read-only Job config mount and attempt label resolve as expected without
contacting a daemon or modifying a container. This does **not** prove the
effective Compose model on the application host, nor a production Docker rollback;
both remain separate production prerequisites. The repository has broad
uncommitted release changes (server, agent, UI, strategies and new files);
their source provenance and authorized deployment boundary remain open.

The Job strategy now runs `docker compose config --format json` with the
exact, temporary image/attempt override **before** `up`, rejecting effective
Job environment, privilege, command, extra networks, unsafe mounts, changed
readiness probe, unreviewed nested health/mount/network options and unreviewed
service fields. The default Compose network must resolve to this project's
default network, not a shared/external one. This catches interpolation or
other Compose resolution not visible in the source YAML checks; regression
tests cover rejected effective models and an opt-in local Docker mock
deploy/rollback. The check is still a pre-mutation snapshot, not an atomic
guard against post-check file/environment changes or a substitute for
reviewing the real application host's effective model.

Latest local inventory: `joincare-deploy` HEAD `2df66a4` with 62 dirty
entries, Backend HEAD `3d44d3b` with 30 dirty entries, SmartContract HEAD
`874abbc` with one untracked entry; `joincare-admin` HEAD `f50d334f` was
clean at the time of this check. This is an inventory, **not** a completed
manual multi-repository provenance/diff review. A wiki observation describes
a **2026-09-17** production DB backup/restore and migrations through 09-18;
that historical result does not verify current production state or establish
the 10-02 signing-metadata migration, so the current DB/backup gate stays
blocked. Current review-only bundle:
`D:/pi-tmp/jc-release-second-phase-review-current-20261003T123355Z`.

An opt-in **local Docker daemon** integration (`go test -tags
docker_integration ./internal/strategies -run
TestJobLocalDockerFirstDeployAndFailedUpgradeRollback`, with
`JC_JOB_DOCKER_INTEGRATION=1`) now tests the actual Job strategy against two
isolated nginx-based mock images: first healthy deploy, backing up the exact
image, unhealthy upgrade and restoration, then unhealthy first deploy and
attempt-scoped container removal. The test skips whenever `jc-job` already
exists and cleans up only its own Compose-bound fixture; no production
containers or signer keys are used. This is real local Docker behavior, **not**
a live IDO runtime, database migration, production rollback test or release
authorization. Real production Compose/permissions/backup remain blockers.

If the command context is canceled after Job mutation, automatic recovery
now uses a separate, at-most-three-minute context for rollback and its
verification. Failure or timeout retains `job_pending`; a canceled request
must not silently skip Docker restore. A local regression exercises both
successful canceled-context recovery and an unverified recovery that stays
blocked. This is not automatic recovery from a crashed Agent process.

The Agent derives the Ethereum Job address locally from its **plaintext, valid
secp256k1** Job private scalar without logging key material. It requires the
signed Job signer to equal the derived Job address; the signed public Core
address must differ from Job and equal the Job V2 `coreOperatorAddress`
configuration. The Core private key need not be on this host; Core key custody
and the on-chain `operator()` must instead be independently evidenced by the
signer and chain/RPC receipts before an assertion can be signed. Public-address
matching alone cannot prove Core control. It rejects differently cased duplicate identity keys that Viper
could merge, and also pins the canonical Job chain ID, RPC URL and V2
contract-address key paths rather than allowing a later common/secret
override. The signed chain ID and V2 contract address must match the local
Job config. This *local* binding is not proof of live container mounts or the
on-chain `operator()`; these remain external evidence.

## Agent readiness contract (not a producer of evidence)

The dedicated Job path fails closed unless `job_environment`, 3 independent
artifact Ed25519 public keys (`job_trusted_keys`), 3 **different** evidence
public keys (`job_readiness_keys`), `job_host_key_sha256`,
`job_readiness_file`, and `job_evidence_dir` are configured. Both files/dir
must be outside deployment and Agent data directories. On Linux the Agent is
root, and the assertion, six receipts, and their path ancestors must be
root-owned and not group/world-writable; individual files must be mode 0600 or
stricter. They must not be symlinks. The Agent compares the configured SSH
Ed25519 fingerprint with the *local* public host key. The fingerprint must
first be confirmed by an independent channel; `known_hosts` alone is not
confirmation. No keys, assertions or receipts are included in this package.

The canonical JSON envelope uses `declaration` (the exact canonical JSON
object bytes) and exactly two `signatures` with distinct evidence key IDs.
Ed25519 signs SHA-256 of `joincare.ido.job.readiness.v1` + LF + declaration.
The declaration structure is `internal/activation/readiness.go`; each field
is mandatory. It binds environment, application Agent ID/root, actual SSH
host fingerprint, release ID/SHA-256, immutable Docker image ID, generation,
1002 migration checksum, database server UUID, BSC chain ID, V2 contract
address, separate Core operator/Job signer addresses, six receipt SHA-256
digests and a <=30-minute
UTC validity window. The six files in `job_evidence_dir` are named
`schema.receipt`, `backup.receipt`, `restore.receipt`, `signer.receipt`,
`chain-rpc.receipt` and `permissions.receipt`. The Agent checks each digest
and canonical `EvidenceReceipt` metadata (`internal/activation/readiness.go`):
kind, result `verified`, host/Agent/release/generation/DB UUID/migration/chain,
external evidence reference digest, and observation no older than 24 hours.
It **cannot authenticate the referenced evidence or run live DB/RPC checks**.
Evidence signers must independently inspect trustworthy, recent
records and refuse missing/contradictory evidence. A signature is not a
release decision or authority to migrate, upgrade or start Job.

### External evidence needed before signing

1. Independently confirmed host identity and correct app Agent; historical
   control-host Job state is not an app-host rollback target.
2. Current MySQL server UUID, migration ledger, real column shape/constraints
   and time zone from authorized *read-only* queries; execute no SQL migration
   here. Verify checksum against the exact proposed migration file.
3. Current database backup bound to UUID/migration manifest and a successful
   **isolated restore** with recorded recovery checks. The July archive is not
   sufficient; retain receipts outside the release/Agent mutable trees.
4. Job/Core distinct signer custody and on-chain operator, nonce slots and
   outbox ownership; BSC chain ID, contract/proxy/code identity, confirmed
   block depth, historical log RPC, reorg/failure handling and least-privilege
   runtime/database identity. Do not expose private keys or execute manual
   relayer transactions.
5. Reviewed effective `docker compose config`, isolated readonly Job mount,
   health probe and rollback artifact. Production currently lacks the Job
   service/config, so static success against fixtures cannot pass this step.

The readiness assertion is checked before download and again after backup,
immediately before intent persistence. A root-owned, mode 0700 Agent data
directory and flock-protected Job command lock prevent two local Agent
processes from racing the generation and recovery journal; a failed lock
acquisition refuses Job work. Filesystem permissions and a lock are not a
substitute for independent host/process identity verification. Windows tests
use a test-only sentinel; the Linux-root positive gate is exercised with a
test-only SSH host-key path in WSL, while production binaries use the fixed
`/etc/ssh/ssh_host_ed25519_key.pub` path. No production host key was accessed
by that test. On Linux, readiness and receipt files are opened with
`O_NOFOLLOW` and their *opened inodes* must be regular, root-owned and mode
0600 or stricter; reads are bounded. This protects the final path component,
not arbitrary ancestor replacement, concurrent in-place writes by root, or
an untrusted signer of the external evidence. The Agent derives addresses
from local plaintext config but does not inspect live Core/Job container
secrets or the on-chain contract `operator()`; operators must reconcile signer
and chain identities independently against the actual production runtime.
After download the Agent persists `job_pending` before apply. Failure to
verify rollback leaves the pending intent and blocks retries. TESTCAT #110
and Fair Launch financial materialization are out of scope. Human approvals
for Server/Agent upgrade, migration and Job startup must remain separate.
