#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
cat > "$TMP/colors.sh" <<'SH'
error(){ printf '%s\n' "$*" >&2; }
step(){ :; }
SH
cat > "$TMP/env.sh" <<'SH'
echo env-library-sourced >&2
exit 97
SH
cat > "$TMP/trap.env" <<'SH'
echo deploy-env-sourced >&2
exit 98
SH
run(){ env JC_LIB="$TMP" JC_ENV_FILE="$TMP/trap.env" bash "$1" "${@:2}"; }
for script in "$ROOT/scripts/cmd/contracts.sh" "$ROOT/scripts/cmd/ido-v2.sh"; do
  case "$script" in *contracts.sh) actions='deploy upgrade' ;; *) actions='deploy configure-project-token' ;; esac
  for action in $actions; do
    set +e; out="$(run "$script" "$action" 2>&1)"; rc=$?; set -e
    [ "$rc" -eq 78 ] && [ "$out" = chain_mutation_unavailable ] || { echo "$script $action: rc=$rc out=$out" >&2; exit 1; }
    [[ "$out" != *env-library-sourced* && "$out" != *deploy-env-sourced* ]]
  done
done
# Missing environment/config files must not change the stable mutation error.
for spec in "$ROOT/scripts/cmd/contracts.sh:deploy" "$ROOT/scripts/cmd/ido-v2.sh:configure-project-token"; do
  script="${spec%%:*}"; action="${spec#*:}"
  set +e; out="$(env JC_LIB="$TMP/missing" JC_ENV_FILE="$TMP/missing.env" bash "$script" "$action" 2>&1)"; rc=$?; set -e
  [ "$rc" -eq 78 ] && [ "$out" = chain_mutation_unavailable ] || exit 1
done
# The backup recovery stub must reject without loading the environment library or
# deployment environment file.
for spec in \
  "$ROOT/scripts/cmd/backup.sh:mysql_backup_credentials_transport_unavailable"; do
  script="${spec%%:*}"; expected="${spec#*:}"
  set +e; out="$(run "$script" 2>&1)"; rc=$?; set -e
  [ "$rc" -eq 78 ] && [ "$out" = "$expected" ] || { echo "$script: rc=$rc out=$out" >&2; exit 1; }
  [[ "$out" != *env-library-sourced* && "$out" != *deploy-env-sourced* ]]
  set +e; out="$(env JC_LIB="$TMP/missing" JC_ENV_FILE="$TMP/missing.env" bash "$script" 2>&1)"; rc=$?; set -e
  [ "$rc" -eq 78 ] && [ "$out" = "$expected" ] || { echo "$script missing-env: rc=$rc out=$out" >&2; exit 1; }
done
# Migration status/help are available but must not source either environment
# surface. Apply without explicit inputs remains fail-closed before environment read.
for envlib in "$TMP" "$TMP/missing"; do
  set +e; out="$(env JC_LIB="$envlib" JC_ENV_FILE="$TMP/missing.env" bash "$ROOT/scripts/cmd/migrate.sh" status 2>&1)"; rc=$?; set -e
  [ "$rc" -eq 0 ] && [[ "$out" == *database_migration_entrypoint=available_controlled* ]] || exit 1
  [[ "$out" != *env-library-sourced* && "$out" != *deploy-env-sourced* ]]
  set +e; out="$(env JC_LIB="$envlib" JC_ENV_FILE="$TMP/missing.env" bash "$ROOT/scripts/cmd/migrate.sh" apply 2>&1)"; rc=$?; set -e
  [ "$rc" -eq 78 ] && [ "$out" = 'database_migration_rejected: defaults_extra_file_required' ] || exit 1
done
# Operational wrappers must reject before sourcing either environment surface.
for spec in \
  "$ROOT/scripts/cmd/config.sh:remote_config_upload_unavailable" \
  "$ROOT/scripts/cmd/restart.sh:frontend:static_release_admission_unavailable" \
  "$ROOT/scripts/cmd/upgrade.sh:admin:static_release_admission_unavailable"; do
  script="${spec%%:*}"; rest="${spec#*:}"
  if [[ "$rest" == *:* ]]; then arg="${rest%%:*}"; expected="${rest#*:}"; else arg=""; expected="$rest"; fi
  set +e; out="$(run "$script" ${arg:+"$arg"} 2>&1)"; rc=$?; set -e
  [ "$rc" -eq 78 ] && [ "$out" = "$expected" ] || { echo "$script: rc=$rc out=$out" >&2; exit 1; }
  [[ "$out" != *env-library-sourced* && "$out" != *deploy-env-sourced* ]]
done
! grep -Rqs 'JC_DEPLOY_PRIVATE_KEY' "$ROOT/scripts/cmd/contracts.sh" "$ROOT/scripts/cmd/ido-v2.sh"
# Patch-time mutation/upload wrappers are also stable unavailable stubs.
for spec in \
  "$ROOT/scripts/patch/fix-hardhat-config.sh:remote_config_upload_unavailable" \
  "$ROOT/scripts/patch/apply-caddyfile-release.sh:remote_config_upload_unavailable" \
  "$ROOT/scripts/patch/deploy-checkin-fresh-0.8.30.sh:chain_mutation_unavailable" \
  "$ROOT/scripts/patch/upgrade-checkin-to-0.8.30.sh:chain_mutation_unavailable"; do
  script="${spec%%:*}"; expected="${spec#*:}"
  set +e; out="$(bash "$script" 2>&1)"; rc=$?; set -e
  [ "$rc" -eq 78 ] && [ "$out" = "$expected" ] || exit 1
done
echo 'Chain mutation and patch CLI unavailable before environment read: PASS'
