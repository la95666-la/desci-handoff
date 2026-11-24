# One-use Job V3-only migration recovery

Explicit operator-authorized exception, NOT automatic healthy rollback and NOT a
general pending bypass. This release-specific implementation supports only the
known production Agent/root/dataDir, generation39 with failed generation40 intent
release d17301c7-7871-49ed-afe6-4f8afe04efdf, exact restored legacy container/image,
exact existing config hash, and target release197 c3d6fbb6-7a23-4fba-8433-cb7a0ede2255
with archive SHA256 9c84d34939b28eddacdc42d374e4f100c34f50739e75d7b20c106d8f837c89a0.

The operator installs a root-only, command-specific permit under the protected
agent data directory with <=30minute expiry. The permit is consumed by rename
before admission, with restricted state/config backups. Existing command failed
states remain failed. The original JobPending is NOT cleared: target pending and
an explicit previous-intent recovery journal are committed atomically after the
normal download/backup and immediately before apply. Generation/deployed are not
advanced until successful normal release health and state recording.

Old-runtime preflight requires exact old container/compose binding, known legacy
keyless RPC failure (no dead letters, blocked submissions or active reorg), running
and caught-up V3 stream epoch3 and target43879281503240193 gap<=120. Legacy health
is recorded as degraded, never forged healthy. New configuration changes only
jobMode ido -> ido-v3 under exact config CAS. Failure restores old config before
normal image rollback; unhealthy rollback retains the target pending and journal.

A crash after permit consumption blocks re-use and requires independent review.
Unrelated commands/releases/states use unchanged fail-closed behavior. No database
mutation, financial/project recovery, transaction send or manual container update
is part of this path. Rollout uses the dedicated checksum-bound Agent binary
upgrade script, and the business Job is still applied solely through jc-release.
