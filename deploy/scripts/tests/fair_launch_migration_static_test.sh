#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
fail(){ echo "fair-launch migration static: $*" >&2; exit 1; }
MIG=("$ROOT/migrations/2026_09_03_fair_launch_project_model.sql" "$ROOT/migrations/2026_09_04_fair_launch_ce_v2_governance.sql" "$ROOT/migrations/2026_09_05_fair_launch_signing_release_control.sql" "$ROOT/migrations/2026_09_06_fair_launch_action_execution.sql" "$ROOT/migrations/2026_09_07_fair_launch_position_projection.sql")
for f in "${MIG[@]}"; do
  [ -f "$f" ] || fail "missing $(basename "$f")"
  grep -Fq '@JC_FAIR_LAUNCH_MIGRATION_APPROVED' "$f" || fail 'migration gate missing'
done

# Migration execution is available only behind external manifest, backup,
# database identity, exact ledger and confirmation gates. Transformation,
# candidate collection, activation and credential rotation remain unavailable.
grep -Fq 'database_migration_entrypoint=available_controlled' "$ROOT/scripts/cmd/migrate.sh" || fail 'controlled migration entrypoint missing'
grep -Fq 'migration_repository_checksum_mismatch' "$ROOT/scripts/cmd/migrate.sh" || fail 'migration checksum gate missing'
grep -Fq 'migration_ledger_has_applying_incident' "$ROOT/scripts/cmd/migrate.sh" || fail 'applying incident gate missing'
grep -Fq 'fair_launch_transformation_unavailable' "$ROOT/scripts/fair-launch/provision-source-registry-privileges.sh" || fail 'transformation boundary missing'
grep -Fq 'fair_launch_candidate_collection_unavailable' "$ROOT/scripts/fair-launch/collect-source-registry-candidate.sh" || fail 'candidate boundary missing'
grep -Fq 'fair_launch_activation_unavailable' "$ROOT/scripts/fair-launch/activation-gate.sh" || fail 'activation boundary missing'
grep -Fq 'credential_rotation_unavailable' "$ROOT/scripts/fair-launch/rotate-source-registry-ingest-credential.sh" || fail 'rotation boundary missing'

grep -Fq "status:'UNADMITTED_PLACEHOLDER'" "$ROOT/scripts/fair-launch/signed-authority.js" || fail 'production trust placeholders must fail closed'
grep -Fq 'crypto.verify' "$ROOT/scripts/fair-launch/signed-authority.js" || fail 'Ed25519 verification missing'
grep -Fq '2-of-3 Ed25519 threshold not met' "$ROOT/scripts/fair-launch/signed-authority.js" || fail 'threshold enforcement missing'
pattern='GRANT ALL'' PRIVILEGES'; ! grep -R --include='*.sh' -n "$pattern" "$ROOT/scripts/cmd" "$ROOT/scripts/patch" || fail 'operational database-wide grant remains'
! grep -Fq 'MYSQL_USER: ${JC_MYSQL_USER' "$ROOT/docker-compose.mysql.yml" || fail 'docker bootstrap still uses runtime identity'

# Install exits unavailable before any remote/Docker/migration operation.
install="$ROOT/scripts/cmd/install.sh"
unavailable=$(grep -n 'fair_launch_activation_unavailable' "$install" | head -1 | cut -d: -f1)
remote=$(grep -nE 'check_remote_deps|remote_docker|scripts/deploy.sh' "$install" | awk -F: -v u="$unavailable" '$1>u{print $1;exit}' || true)
[ -n "$unavailable" ] || fail 'install unavailable boundary missing'
# Later unreachable legacy text may remain, but no operation may precede the boundary.
first_remote=$(grep -nE 'check_remote_deps|remote_docker|scripts/deploy.sh' "$install" | head -1 | cut -d: -f1 || true)
[ -z "$first_remote" ] || [ "$unavailable" -lt "$first_remote" ] || fail 'install reaches remote operation before unavailable exit'

for f in "$ROOT/scripts/cmd/migrate.sh" "$ROOT/scripts/cmd/install.sh" "$ROOT/scripts/fair-launch/provision-source-registry-privileges.sh" "$ROOT/scripts/fair-launch/rotate-source-registry-ingest-credential.sh"; do bash -n "$f"; done
node "$ROOT/scripts/tests/fair_launch_source_registry_privilege_test.js"
(cd "$ROOT/migrations" && sha256sum -c FAIR_LAUNCH_SHA256SUMS)
[ "$(grep -Fc 'fair_launch_position' "$ROOT/scripts/fair-launch/source-registry-privilege-policy.js")" -ge 2 ] || fail 'position table missing from runtime/job grant policy'
bash "$ROOT/scripts/tests/controlled_migration_entrypoint_test.sh"
echo 'Fair Launch executable migration static validation: PASS'
