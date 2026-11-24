#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
fail(){ echo "controlled migration entrypoint test: $*" >&2; exit 1; }

out="$(bash "$ROOT/scripts/cmd/migrate.sh" status)"
grep -Fq 'database_migration_entrypoint=available_controlled' <<<"$out" || fail 'status does not report controlled availability'
grep -Fq 'requires_external_manifest_backup_receipt_and_confirmation' <<<"$out" || fail 'status omits apply gates'

help="$(bash "$ROOT/scripts/cmd/migrate.sh" --help)"
for required in --defaults-extra-file --manifest --backup-receipt --evidence-dir APPLY-CHECKSUM-BOUND-MIGRATIONS; do
  grep -Fq -- "$required" <<<"$help" || fail "help missing $required"
done
grep -Fq 'never auto-resumes' <<<"$help" || fail 'incident resume boundary missing'
grep -Fq 'does not start Backend, Job, scanner or materializer' <<<"$help" || fail 'activation separation missing'

set +e
rejected="$(bash "$ROOT/scripts/cmd/migrate.sh" apply 2>&1)"; rc=$?
set -e
[ "$rc" -eq 78 ] || fail "empty apply returned $rc"
[ "$rejected" = 'database_migration_rejected: defaults_extra_file_required' ] || fail 'empty apply rejection changed'

grep -Fq 'schema_migrations' "$ROOT/scripts/cmd/migrate.sh" || fail 'ledger gate missing'
grep -Fq "status='applying'" "$ROOT/scripts/cmd/migrate.sh" || fail 'applying incident gate missing'
grep -Fq 'backup_dump_checksum_mismatch' "$ROOT/scripts/cmd/migrate.sh" || fail 'backup digest gate missing'
grep -Fq 'linux_permission_semantics_required' "$ROOT/scripts/cmd/migrate.sh" || fail 'Linux permission-semantics gate missing'
grep -Fq 'require_external_file backup_dump' "$ROOT/scripts/cmd/migrate.sh" || fail 'protected external dump gate missing'
grep -Fq 'migration_repository_checksum_mismatch' "$ROOT/scripts/cmd/migrate.sh" || fail 'SQL digest gate missing'
grep -Fq 'migration_manifest_not_strictly_ordered' "$ROOT/scripts/cmd/migrate.sh" || fail 'strict ordering gate missing'
! grep -Eq 'MYSQL_PWD|--password=|-p[^[:space:]]+' "$ROOT/scripts/cmd/migrate.sh" || fail 'password-bearing transport found'
bash -n "$ROOT/scripts/cmd/migrate.sh"
echo 'Controlled migration entrypoint: PASS'
