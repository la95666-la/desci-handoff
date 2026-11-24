#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BACKEND_ROOT="$(cd "$ROOT/../joincare_code/joincare_code/_extracted/joincare_server-master" && pwd)"
fail(){ echo "fair_launch_local_e2e_schema_static_test: FAIL: $*" >&2; exit 1; }
FROZEN=(
 '2026_09_03_fair_launch_project_model.sql'
 '2026_09_04_fair_launch_ce_v2_governance.sql'
 '2026_09_05_fair_launch_signing_release_control.sql'
 '2026_09_06_fair_launch_action_execution.sql'
 '2026_09_07_fair_launch_position_projection.sql'
)
LOCAL=(
 '2026_09_08_fair_launch_local_e2e_binding.sql'
 '2026_09_09_fair_launch_financial_projection_v3.sql'
 '2026_09_10_fair_launch_governance_projection_v3.sql'
 '2026_09_11_fair_launch_effect_lineage_v3.sql'
 '2026_09_12_fair_launch_typed_ledger_v3.sql'
)
(cd "$ROOT/migrations" && sha256sum -c FAIR_LAUNCH_SHA256SUMS)
for f in "${LOCAL[@]}"; do
  p="$ROOT/migrations/$f"
  [ -f "$p" ] || fail "missing $f"
  grep -Fq '@JC_FAIR_LAUNCH_LOCAL_E2E_APPROVED' "$p" || fail "$f lacks local-only gate"
  grep -Fq 'local E2E migration requires local-only execution gate' "$p" || fail "$f lacks exact local-only rejection"
  ! grep -Eq 'JC_FAIR_LAUNCH_MIGRATION_APPROVED|mainnet|chain_id[[:space:]]*=[[:space:]]*56' "$p" || fail "$f contains production gate/identity"
done
for f in "${LOCAL[@]}"; do
  ! grep -R -Fq "$f" "$ROOT/scripts/cmd" "$ROOT/scripts/release" "$ROOT/.github/workflows" || fail "$f leaked into production command/release/workflow"
done
[ "$(grep -Ec '^[0-9a-f]{64} \*2026_09_(0[3-9]|1[0-2])_fair_launch_' "$ROOT/migrations/FAIR_LAUNCH_SHA256SUMS")" = '10' ] || fail 'checksum manifest does not pin exactly 0903-0912'
grep -Fq "2026_09_07_fair_launch_position_projection.sql" "$ROOT/scripts/fair-launch/migration-inputs.js" || fail 'frozen production migration input list missing'
! grep -Eq '2026_09_(08|09|10|11|12)' "$ROOT/scripts/fair-launch/migration-inputs.js" || fail 'local migration leaked into production migration input policy'
grep -Fq "const APPEND_ONLY" "$ROOT/scripts/fair-launch/local-e2e/grant-policy.js" || fail 'local append-only grant partition missing'
grep -Fq "fair_launch_reversal_closure_v3" "$ROOT/scripts/fair-launch/local-e2e/grant-policy.js" || fail 'local closure grant missing'
grep -Fq "...on('INSERT',APPEND_ONLY,'job')" "$ROOT/scripts/fair-launch/local-e2e/grant-policy.js" || fail 'Job append-only INSERT policy missing'
! grep -Eq "(UPDATE|DELETE).*APPEND_ONLY" "$ROOT/scripts/fair-launch/local-e2e/grant-policy.js" || fail 'append-only evidence gained mutation privilege'
grep -Fq "trap cleanup_failed_start EXIT" "$ROOT/scripts/fair-launch/local-e2e/start.sh" || fail 'startup EXIT cleanup trap missing'
grep -Fq "trap 'exit 130' INT" "$ROOT/scripts/fair-launch/local-e2e/start.sh" || fail 'startup INT termination handler missing'
grep -Fq "trap 'exit 143' TERM" "$ROOT/scripts/fair-launch/local-e2e/start.sh" || fail 'startup TERM termination handler missing'
for script in apply-schema.sh mysql-port-reservation.sh; do
  grep -Fq "trap cleanup EXIT" "$ROOT/scripts/fair-launch/local-e2e/$script" || fail "$script EXIT cleanup trap missing"
  grep -Fq "trap 'exit 130' INT" "$ROOT/scripts/fair-launch/local-e2e/$script" || fail "$script INT termination handler missing"
  grep -Fq "trap 'exit 143' TERM" "$ROOT/scripts/fair-launch/local-e2e/$script" || fail "$script TERM termination handler missing"
done
for phase in 'start.sh:start:240s' 'apply-schema.sh:apply-schema:280s' 'status.sh:status:60s' 'teardown.sh:teardown:120s'; do
  IFS=: read -r script name bound <<<"$phase"
  grep -Fq "jc_fl_e2e_self_bound $name $bound" "$ROOT/scripts/fair-launch/local-e2e/$script" || fail "$script direct invocation is not self-bounded"
done
! grep -Fq 'jc_fl_e2e_self_bound backend-serve' "$ROOT/scripts/fair-launch/local-e2e/backend-serve.sh" || fail 'backend serve still has the automation phase lifetime bound'
grep -Fq "jc_fl_e2e_trusted_timeout >/dev/null||jc_fl_e2e_fail 'trusted system timeout unavailable'" "$ROOT/scripts/fair-launch/local-e2e/backend-serve.sh" || fail 'backend serve no longer verifies the trusted execution surface'
grep -Fq 'if [ "$phase" = backend-serve ]; then' "$ROOT/scripts/fair-launch/local-e2e/phase.sh" || fail 'backend serve is not separated from bounded automation phases'
grep -Fq 'exec bash "$script_path" "$@"' "$ROOT/scripts/fair-launch/local-e2e/phase.sh" || fail 'backend serve direct execution contract missing'
! grep -Fq 'fairLaunchLocalHTTPMaxLifetime' "$BACKEND_ROOT/pkg/job/job/chainevent/fair_launch_local_http.go" || fail 'Backend HTTP authority still has a separate maximum lifetime'
! grep -Fq 'Add(30 * time.Minute)' "$BACKEND_ROOT/cmd/fair-launch-local-e2e/main.go" || fail 'Backend HTTP entry point still mints a 30-minute authority'
grep -Fq 'expiresAt := admission.ExpiresAt.UTC()' "$BACKEND_ROOT/pkg/job/job/chainevent/fair_launch_local_http.go" || fail 'Backend HTTP authority does not inherit the run admission expiry'
grep -Fq "mapfile -d '' -t actual" "$ROOT/scripts/fair-launch/local-e2e/common.sh" || fail 'timeout parent argv is not NUL-exact compared'
grep -Fq '"${#actual[@]}" = "${#expected[@]}"' "$ROOT/scripts/fair-launch/local-e2e/common.sh" || fail 'timeout parent argv cardinality is not exact'
grep -Fq '[ "$exe" = "$timeout_bin" ]' "$ROOT/scripts/fair-launch/local-e2e/common.sh" || fail 'trusted timeout executable identity not exact'
grep -Fq -- '--network host' "$ROOT/scripts/fair-launch/local-e2e/start.sh" || fail 'WSL local host-network MySQL transport missing'
grep -Fq -- '--bind-address=127.0.0.1' "$ROOT/scripts/fair-launch/local-e2e/start.sh" || fail 'exact loopback mysqld bind missing'
grep -Fq -- '--mysqlx=0' "$ROOT/scripts/fair-launch/local-e2e/start.sh" || fail 'MySQL X listener disable missing'
! grep -Eq -- 'jc_fl_e2e_docker run[^;]*(^|[[:space:]])(-p|--publish)([[:space:]]|=)' "$ROOT/scripts/fair-launch/local-e2e/start.sh" || fail 'Docker published-port mapping must be absent in host-network mode'
grep -Fq 'JC_FL_MYSQL_PORT_RESERVATION_V1' "$ROOT/scripts/fair-launch/local-e2e/mysql-port-reservation.py" || fail 'port reservation digest domain missing'
! grep -R -E -- '(-p|--publish)[ =]+(0[.]0[.]0[.]0|localhost|::1)|MYSQL_PWD=|mysql://[^[:space:]]+@' "$ROOT/scripts/fair-launch/local-e2e" >/dev/null || fail 'forbidden MySQL bind or secret transport detected'
grep -Fq 'JC_FL_MYSQL_TRANSPORT_V1' "$ROOT/scripts/fair-launch/local-e2e/common.sh" || fail 'MySQL transport digest domain missing'
grep -Fq 'jc_fl_e2e_mysql_transport_owned' "$ROOT/scripts/fair-launch/local-e2e/status.sh" || fail 'status transport ownership gate missing'
grep -Fq 'jc_fl_e2e_mysql_transport_owned' "$ROOT/scripts/fair-launch/local-e2e/teardown.sh" || fail 'teardown transport ownership gate missing'
grep -Fq 'mysql-runtime.cnf' "$ROOT/docs/FAIR-LAUNCH-LOCAL-E2E-RUNBOOK.md" || fail 'credential handoff documentation missing'
grep -Fq "fair_launch_lineage_reversal_v3" "$ROOT/scripts/fair-launch/local-e2e/grant-policy.js" || fail 'local reversal grant contract missing'
grep -Fq 'services: {}' "$ROOT/docker-compose.yml" || fail 'production compose gained services'
grep -Fq 'database_migration_entrypoint=available_controlled' "$ROOT/scripts/cmd/migrate.sh" || fail 'controlled production migration entrypoint missing'
grep -Fq 'explicit_apply_confirmation_required' "$ROOT/scripts/cmd/migrate.sh" || fail 'production migration confirmation gate missing'
frozen_paths=(); for f in "${FROZEN[@]}"; do frozen_paths+=("migrations/$f"); done
! git -C "$ROOT" diff --name-only -- "${frozen_paths[@]}" | grep -q . || fail 'frozen migration modified'
if command -v node >/dev/null && ! node -p 'process.platform' 2>/dev/null | grep -Fq win32; then
  node "$ROOT/scripts/tests/fair_launch_local_e2e_schema_spec_test.js"
elif command -v node.exe >/dev/null && command -v wslpath >/dev/null; then
  node.exe "$(wslpath -m "$ROOT/scripts/tests/fair_launch_local_e2e_schema_spec_test.js")"
else
  fail 'Node.js is required'
fi
! grep -R -E 'redis-cli .* (-a|--pass)|REDISCLI_AUTH=' "$ROOT/scripts/fair-launch/local-e2e" >/dev/null || { echo 'Redis secret-bearing CLI/environment usage detected' >&2; exit 1; }
echo 'Fair Launch LOCAL-ONLY E2E schema static validation: PASS'
