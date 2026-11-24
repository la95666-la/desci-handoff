#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
fail(){ echo "Fair Launch activation static: $*" >&2; exit 1; }
cd "$ROOT"

node scripts/tests/fair_launch_activation_decision_test.js
node scripts/tests/fair_launch_authority_entrypoints_test.js
bash scripts/tests/fair_launch_logs_guard_test.sh
bash scripts/tests/fair_launch_remote_helper_test.sh
bash scripts/tests/checkin_preflight_cli_test.sh
bash scripts/tests/runtime_image_test.sh
bash scripts/tests/contracts_artifact_allowlist_test.sh
python scripts/tests/ido_prepare_set_shutdown_test.py
bash scripts/tests/github_actions_pin_policy_test.sh

# Production activation/evidence/partner-package entrypoints remain honest hard
# stops. Migration is the sole controlled database-mutation exception.
for file in scripts/deploy.sh pack.sh scripts/fair-launch/activation-gate.sh \
  scripts/fair-launch/collect-source-registry-candidate.sh \
  scripts/cmd/backup.sh scripts/backup-full.sh; do
  grep -Fq 'unavailable' "$file" || fail "unavailable boundary missing: $file"
  bash -n "$file"
done
bash -n scripts/cmd/migrate.sh
[[ "$(bash scripts/cmd/migrate.sh status)" == *database_migration_entrypoint=available_controlled* ]] || fail 'controlled migration entrypoint unavailable'
grep -Fq 'explicit_apply_confirmation_required' scripts/cmd/migrate.sh || fail 'controlled migration confirmation gate missing'
[ "$(wc -l < pack.sh)" -le 12 ] || fail 'partner package retired body retained'
set +e
pack_out="$(bash pack.sh --with-data 2>&1)"; pack_rc=$?
set -e
[ "$pack_rc" -eq 78 ] && [ "$pack_out" = 'partner_package_unavailable' ] || fail 'partner package stub contract failed'

# Exact single-token wrappers reject multiple services, aliases and shell syntax.
for file in scripts/cmd/restart.sh scripts/cmd/upgrade.sh scripts/cmd/config.sh scripts/cmd/contracts.sh scripts/cmd/ido-v2.sh build.sh; do
  bash -n "$file"
done

# Direct deployment has no upload, Docker, SSH, MySQL or Compose implementation.
! grep -Eq '(docker|ssh|scp|mysql|compose|rsync)' scripts/deploy.sh || fail 'legacy direct deploy implementation remains'

# Scan every tracked operator Markdown file rather than selected directories. Historical
# evidence must be explicitly non-executable and may not retain actionable protected
# service, database, package, or chain-mutation instructions.
mapfile -d '' markdown < <(git ls-files -z '*.md')
[ "${#markdown[@]}" -gt 0 ] || fail 'tracked Markdown inventory unavailable'
PROHIBITED_DOC='generate-schema-manifest\.js[^#]*(can|may|可以|可)[^#]*(produce|generate|生成)[^#]*candidate|generate-resume-checkpoint\.js[^#]*(can|may|可以|可)[^#]*(produce|generate|生成)[^#]*candidate|MYSQL_PWD|--mysql-cmd|ido_prepare_set\.py[^#\n]*($|[^-])--database|python3[[:space:]]+ido_prepare_set\.py[[:space:]]+[^#\n]*\.json[[:space:]]*$|tgbot[^#]*(remain available|可用|可发布)|apply-upgrade\.sh[[:space:]]+(backend|job|all)|pack\.sh[[:space:]]+--with-data|JC_IDO_JOB_ENABLED=1|\./deploy\.sh[[:space:]]+(install|migrate|backup)|\./deploy\.sh[[:space:]]+(contracts|ido-v2)[[:space:]]+(deploy|upgrade|configure-project-token)|\./deploy\.sh[[:space:]]+(restart|upgrade)[[:space:]]+(backend|job|all)|remote_docker[[:space:]]+[^#]*(restart|up|start)[^#]*(backend|job)|docker[[:space:]]+(start|restart)[[:space:]]+jc-(backend|job)|docker[[:space:]]+compose[^#]*(up|start|restart|recreate)[^#]*(backend|job)|load[^#]*(backend|job)[^#]*(retag|recreate)|retag[^#]*(backend|job)|create[^#]*(backend|job)[[:space:]]+container|Frontend[^#]*Admin[^#]*(only available release operations|可用的发布操作|remain releasable|可发布)'
! grep -nEih "$PROHIBITED_DOC" "${markdown[@]}" || fail 'unavailable operation is advertised as working'
SCHEMA_SPEC=migrations/verify/FAIR_LAUNCH_MYSQL8_SCHEMA_SNAPSHOT_SPEC.md
grep -Fq 'Repository generators, including `generate-schema-manifest.js` and `generate-resume-checkpoint.js`, are always unavailable' "$SCHEMA_SPEC" || fail 'schema authority spec does not retire repository candidate generators'
! grep -Fq 'generate-schema-manifest.js` can only produce a `.candidate.json`' "$SCHEMA_SPEC" || fail 'schema authority spec retains stale repository generator authority'
# Executable names are lowercase in command examples. Keep this case-sensitive so
# prose such as "MySQL ... fail-fast" and the safe MYSQL_PWD transport do not match.
! grep -nEh '(^|[[:space:]`/])(mysql|mysqladmin|mysqldump)[^[:cntrl:]]*(--password=|-p[^[:space:]]+)' "${markdown[@]}" || fail 'password-bearing MySQL operator instruction remains'

# Scan all tracked executable scripts, including top-level paths. Test fixtures are
# excluded because they intentionally contain attack strings.
mapfile -d '' tracked_scripts < <(git ls-files -z '*.sh')
active_scripts=()
for file in "${tracked_scripts[@]}"; do
  case "$file" in scripts/tests/*) ;; *) active_scripts+=("$file") ;; esac
done
! grep -nE '(^|[[:space:]`/])(mysql|mysqladmin|mysqldump)[^[:cntrl:]]*(--password=|-p[^[:space:]]+)' "${active_scripts[@]}" docker-compose.mysql.yml || fail 'password-bearing MySQL argv remains'
! grep -nEi 'GRANT[[:space:]]+ALL[[:space:]]+PRIVILEGES' "${active_scripts[@]}" || fail 'database-wide grant remains'
mapfile -d '' tracked_security < <(git ls-files -z '*.sh' '*.js' '*.mjs' '*.yml' '*.yaml' '*.md' 'Dockerfile*' '**/Dockerfile*')
filtered_security=(); for file in "${tracked_security[@]}"; do case "$file" in scripts/tests/*) ;; *) filtered_security+=("$file") ;; esac; done
if grep -nE '(^|[[:space:]])(-e|--env)[[:space:]]+[^[:space:]]*(PRIVATE_KEY|KEEPER_KEY|DEPLOYER_KEY)|ssh[^[:cntrl:]]*(PRIVATE_KEY|KEEPER_KEY|DEPLOYER_KEY)|JC_DEPLOY_PRIVATE_KEY|scripts/(deployCheckIn|upgradeCheckIn|deployIDOv2|configureIDOv2ProjectToken)\.js' "${filtered_security[@]}"; then fail 'private key transport or executable target remains'; fi
# Active Python/Node entrypoints may not retain direct Docker/MySQL/subprocess
# mutation. Test fixtures are excluded above and shell stubs are checked separately.
mapfile -d '' active_py_node < <(find scripts -maxdepth 2 -type f \( -name '*.py' -o -name '*.js' -o -name '*.mjs' \) ! -path 'scripts/tests/*' -print0)
if grep -nE '(^|[^A-Za-z])(subprocess|child_process|execFileSync|spawnSync|docker[[:space:]]+(inspect|exec)|MYSQL_ROOT_PASSWORD|MYSQL_PWD)' "${active_py_node[@]}"; then fail 'active Python/Node database or process mutation remains'; fi
for file in scripts/fair-launch/generate-schema-manifest.js scripts/fair-launch/generate-resume-checkpoint.js; do
  [ "$(wc -l < "$file")" -le 6 ] || fail "candidate generator retains active body: $file"
  grep -Fq 'fair_launch_candidate_collection_unavailable' "$file" || fail "candidate generator is not fail closed: $file"
done

# Generic remote shell/Compose names are hard stubs, and no active non-test shell
# may execute ssh or docker compose. Validators remain pure local helpers.
if grep -RqnE 'remote_docker|remote_pipe' scripts/lib/env.sh scripts/cmd --include='*.sh'; then fail 'generic remote shell bridge remains'; fi
[ "$(grep -RIl 'remote_ssh' scripts/cmd --include='*.sh' | wc -l)" -eq 0 ] || fail 'command wrapper calls generic SSH boundary'
for fn in remote_ssh remote_compose_command remote_compose_status remote_compose_logs; do
  awk -v fn="$fn" '$0 ~ "^" fn "\\(\\)" {inside=1} inside{print} inside && /^}/ {exit}' scripts/lib/env.sh | grep -Fq 'return 78' || fail "$fn is not a hard unavailable stub"
done
if grep -nE '^[[:space:]]*(ssh|docker[[:space:]]+compose)([[:space:]\\]|$)' "${active_scripts[@]}"; then fail 'active shell executes SSH or Docker Compose'; fi
grep -Fq 'service_log_transport_unavailable' scripts/cmd/logs.sh || fail 'shell logs transport remains active'
if grep -Rqn 'JC_DEPLOY_PRIVATE_KEY' scripts/cmd; then fail 'chain private key transport remains'; fi
# The remaining service wrappers revalidate a single token before interpolation.
for file in scripts/cmd/restart.sh scripts/cmd/upgrade.sh; do
  grep -Fq '[[ "$SERVICE" =~ ^[a-z]+$ ]]' "$file" || fail "service token validation missing: $file"
  grep -Fq 'case "$SERVICE" in' "$file" || fail "service allowlist missing: $file"
done


# All patch scripts are either non-mutating evidence tools or short unavailable stubs.
for file in scripts/patch/fix-hardhat-config.sh scripts/patch/deploy-checkin-fresh-0.8.30.sh scripts/patch/upgrade-checkin-to-0.8.30.sh scripts/patch/apply-caddyfile-release.sh; do
  [ "$(wc -l < "$file")" -le 6 ] || fail "active patch mutation/upload retained: $file"
  grep -Fq 'unavailable' "$file" || fail "patch path not unavailable: $file"
done

# Build and Hardhat operational targets cannot contain mutation scripts or private-key consumers.
set +e
build_out="$(env JC_ENV_FILE=/definitely/missing bash build.sh contracts 2>&1)"; build_rc=$?
set -e
[ "$build_rc" -eq 78 ] && [[ "$build_out" == *'chain_mutation_unavailable'* ]] || fail 'contracts build target is not stably unavailable'
! grep -nE 'build_contracts|build_tgbot|deployCheckIn|upgradeCheckIn|deployIDOv2|configureIDOv2ProjectToken|JC_DEPLOY_PRIVATE_KEY' build.sh deploy/Dockerfile.hardhat deploy/Dockerfile.hardhat.runtime deploy/hardhat-entrypoint.sh docker-compose.hardhat.yml || fail 'Hardhat/contract mutation artifact path remains'
grep -Fq 'hardhat_operational_target_unavailable' deploy/Dockerfile.hardhat deploy/Dockerfile.hardhat.runtime deploy/hardhat-entrypoint.sh || fail 'Hardhat unavailable boundary missing'
! grep -nE '(^|[|"[:space:]])hardhat([|"[:space:]]|$)' scripts/cmd/logs.sh scripts/lib/env.sh || fail 'Hardhat remains an operational helper target'
[ ! -d artifacts/contracts ] || fail 'stale contract mutation artifacts remain'

# No component has an operational strategy or API upload path.
for comp in Backend Frontend Admin TGBot Lang Job; do grep -Fq "shared.Comp${comp}:" tools/jc-release/internal/strategies/strategy.go || fail "missing strategy boundary: $comp"; done
[ "$(grep -c '&unavailableStrategy' tools/jc-release/internal/strategies/strategy.go)" -eq 6 ] || fail 'an operational production strategy remains'
! grep -RqnE 'exec\.Command|docker[[:space:]]+(load|tag|restart|rm)|docker[[:space:]]+compose' tools/jc-release/internal/strategies --include='*.go' || fail 'strategy package retains process/container mutation'
grep -Fq 'func ValidComponent(Component) bool { return false }' tools/jc-release/internal/shared/types.go || fail 'release API still admits a component'
grep -Fq 'static_release_admission_unavailable' deploy.sh scripts/cmd/restart.sh scripts/cmd/upgrade.sh tools/jc-release/README.md || fail 'static release boundary missing'
grep -Fq 'tgbot_release_unavailable' deploy.sh scripts/cmd/restart.sh scripts/cmd/upgrade.sh || fail 'TGBot shell boundary missing'
! grep -nE 'artifacts/tgbot|joincare-tgbot|build_tgbot|case CompFrontend, CompAdmin, CompTGBot' scripts/release/build-test-runtime.sh .github/workflows/release.yml build.sh tools/jc-release/internal/shared/types.go || fail 'TGBot release artifact or API path remains'

# Rendered help must accurately expose the release boundary.
help="$(bash deploy.sh help)"
[[ "$help" == *'Migration execution is available only through the fail-closed manifest'* ]] || fail 'rendered migration availability is stale'
[[ "$help" == *'Production service activation remains NO-GO through this CLI'* ]] || fail 'rendered service activation boundary is stale'
[[ "$help" == *'config                         remote_config_upload_unavailable'* ]] || fail 'rendered help omits config boundary'

# Stale generated repository dist and partner packaging are excluded. Final release tar
# remains an explicit allowlist and cannot include pack.sh or top-level operational docs.
grep -Fq -- '--exclude dist' scripts/release/build-test-runtime.sh || fail 'generated frontend dist exclusion missing'
grep -Fq -- "--exclude 'apps/*/dist'" scripts/release/build-test-runtime.sh || fail 'generated admin dist exclusion missing'
grep -Fq 'artifacts/backend/joincare-core artifacts/backend/joincare-job artifacts/backend/joincare-notice' scripts/release/build-test-runtime.sh || fail 'explicit final release allowlist missing'
grep -Fq 'scripts/release/lib/safe_tar.py' scripts/release/build-test-runtime.sh scripts/release/lib/release_common.sh || fail 'packaged safe archive validator missing from release allowlist'
! grep -Eq 'install .*pack\.sh|cp .*pack\.sh|tar .*pack\.sh|install .*DEPLOY(MENT_GUIDE)?\.md|cp .*PARTNER_DEPLOYMENT\.md|tar .*PARTNER_DEPLOYMENT\.md' scripts/release/build-test-runtime.sh || fail 'runtime package includes retired partner path or stale docs'
! grep -Eq 'docker[[:space:]]+(push|tag)|ghcr\.io/.*/joincare-backend|service:[[:space:]]*\[backend\]' .github/workflows/release.yml || fail 'Backend GHCR publication remains'
grep -Fq 'fair_launch_activation_static_test.sh' .github/workflows/build.yml || fail 'activation suite missing from CI'
grep -Fq 'working-directory: tools/jc-release' .github/workflows/build.yml && grep -Fq 'run: go test ./...' .github/workflows/build.yml || fail 'jc-release suite missing from CI'

# Every tracked Compose YAML is mechanically free of protected runnable services.
mapfile -t compose_files < <(git ls-files '*compose*.yml' '*compose*.yaml')
[ "${#compose_files[@]}" -gt 0 ] || fail 'compose inventory unavailable'
for file in "${compose_files[@]}"; do
  ! grep -Eq '^[[:space:]]{2}(backend|job|tgbot|hardhat|mysql):[[:space:]]*$' "$file" || fail "protected runnable Compose service: $file"
done

# Render the actual generated handoff template and inspect output, not source fragments.
HANDOFF_TMP="$(mktemp -d)"; trap 'rm -rf "$HANDOFF_TMP"' EXIT
export PROFILE=test DATE_TAG=20260908 VITE_CHECKIN_CHAIN_ID=56 VITE_CHECKIN_ADDRESS=0x0000000000000000000000000000000000000001 VITE_TRUSTED_PROJECT_CHAINS=56
export BACKEND_COMMIT=1111111111111111111111111111111111111111 FRONTEND_COMMIT=2222222222222222222222222222222222222222 ADMIN_COMMIT=3333333333333333333333333333333333333333
export AGENT_DIR=/archive BACKEND_IMAGE_FILE=backend.tar.gz BACKEND_IMG_PUB_SHA="$(printf 'a%.0s' {1..64})" BACKEND_IMAGE_SIZE=1 BACKEND_IMAGE_TAG=joincare/backend:archive
export FRONTEND_PACKAGE_FILE=frontend.tar.gz FRONTEND_PKG_PUB_SHA="$(printf 'b%.0s' {1..64})" FRONTEND_PACKAGE_SIZE=1
export ADMIN_PACKAGE_FILE=admin.tar.gz ADMIN_PKG_PUB_SHA="$(printf 'c%.0s' {1..64})" ADMIN_PACKAGE_SIZE=1
bash scripts/release/render-agent-handoff.sh "$HANDOFF_TMP/AGENT-HANDOFF.md"
grep -Fq 'fair_launch_activation_unavailable' "$HANDOFF_TMP/AGENT-HANDOFF.md" || fail 'handoff lacks backend shutdown'
grep -Fq 'static_release_admission_unavailable' "$HANDOFF_TMP/AGENT-HANDOFF.md" || fail 'handoff lacks static release shutdown'
! grep -Eqi 'docker[[:space:]]+cp|安装（agent|恢复上一版静态' "$HANDOFF_TMP/AGENT-HANDOFF.md" || fail 'handoff contains static install/rollback guidance'
grep -Fq '仅归档和离线校验' "$HANDOFF_TMP/AGENT-HANDOFF.md" || fail 'backend artifact not archive-only'
! grep -nEih 'docker[[:space:]]+(load|start|restart)[^#]*(backend|BACKEND)|compose[^#]*(up|start|restart|recreate)[^#]*(backend|BACKEND)|重打标|retag' "$HANDOFF_TMP/AGENT-HANDOFF.md" || fail 'handoff contains backend activation/rollback instructions'

for file in scripts/patch/reseed-rds*.sh; do
  [ "$(wc -l < "$file")" -le 5 ] || fail "disabled reseed implementation retained: $file"
  grep -Fq 'operational_reseed_activation_unavailable' "$file" || fail "reseed path active: $file"
done

echo 'Fair Launch activation/start/password static: PASS'
