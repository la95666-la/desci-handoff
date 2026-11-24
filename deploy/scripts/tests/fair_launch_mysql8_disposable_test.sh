#!/usr/bin/env bash
set -euo pipefail

# TEST ONLY. This is a disposable MySQL semantics test, not a migration runner,
# schema candidate collector, admission authority, or production operation.

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
IMAGE='mysql@sha256:c831a0f11348d402b43d77453e17d770be2eef356615a2823fe0f5a0d6c8b9af'
DOCKER_SOCKET='/var/run/docker.sock'
RUN_ID="$(openssl rand -hex 16)"
NAME="jc-fl-mysql-disposable-$RUN_ID"
OWNER_LABEL="joincare.fair-launch.disposable-test=$RUN_ID"
DB='joincare_fl_disposable_test'
SECRET_DIR=''
SECRET_FILE=''
NO_GATE_OUTPUT="/tmp/jc-fl-mysql8-no-gate-$RUN_ID.out"
EVIDENCE="${JOINCARE_FL_MYSQL8_TEST_EVIDENCE:-/tmp/joincare-fair-launch-mysql8-disposable-$RUN_ID.txt}"
CONTAINER_ID=''

fail() {
  echo "fair_launch_mysql8_disposable_test: FAIL: $*" >&2
  exit 1
}

cleanup() {
  if [ -n "$CONTAINER_ID" ]; then
    observed_label="$(docker_local inspect --format '{{index .Config.Labels "joincare.fair-launch.disposable-test"}}' "$CONTAINER_ID" 2>/dev/null || true)"
    if [ "$observed_label" = "$RUN_ID" ]; then
      docker_local rm -f "$CONTAINER_ID" >/dev/null 2>&1 || true
    fi
  fi
  [ -z "$SECRET_DIR" ] || rm -rf "$SECRET_DIR"
  rm -f "$NO_GATE_OUTPUT"
}

for forbidden in JC_DEPLOY_RPC JC_DEPLOY_CHAIN_ID JC_DEPLOY_PRIVATE_KEY JC_MYSQL_ROOT_PASSWORD MYSQL_PWD FAIR_LAUNCH_DISPOSABLE_MYSQL_DSN DOCKER_HOST DOCKER_CONTEXT DOCKER_TLS_VERIFY DOCKER_CERT_PATH; do
  [ -z "${!forbidden:-}" ] || fail "refusing production/external database environment: $forbidden"
done

command -v docker >/dev/null || fail 'docker is required'
command -v openssl >/dev/null || fail 'openssl is required'
command -v sha256sum >/dev/null || fail 'sha256sum is required'
[ -S "$DOCKER_SOCKET" ] || fail "local Docker Unix socket is unavailable: $DOCKER_SOCKET"
[ "$(readlink -f "$DOCKER_SOCKET")" = '/run/docker.sock' ] || fail 'Docker socket does not resolve to the expected local WSL socket'
docker_local() { docker -H "unix://$DOCKER_SOCKET" "$@"; }
docker_local version --format '{{.Server.Version}}' >/dev/null 2>&1 || fail 'local Docker daemon is unavailable'
docker_local image inspect "$IMAGE" >/dev/null 2>&1 || fail "pinned local image is unavailable: $IMAGE"
[ ! -e "$EVIDENCE" ] || fail "evidence path already exists: $EVIDENCE"
trap cleanup EXIT INT TERM

umask 077
SECRET_DIR="$(mktemp -d)"
SECRET_FILE="$SECRET_DIR/root-password"
openssl rand -hex 32 > "$SECRET_FILE"
chmod 600 "$SECRET_FILE"

echo '[mysql8-disposable] starting network-none, tmpfs-backed MySQL 8.4'
CONTAINER_ID="$(docker_local run -d \
  --name "$NAME" \
  --label "$OWNER_LABEL" \
  --network none \
  --tmpfs /var/lib/mysql:rw,nosuid,nodev,size=2g \
  --tmpfs /tmp:rw,nosuid,nodev,size=256m \
  -v "$SECRET_FILE:/run/secrets/jc-fl-root-password:ro" \
  -v "$ROOT:/repo:ro" \
  -e MYSQL_ROOT_PASSWORD_FILE=/run/secrets/jc-fl-root-password \
  "$IMAGE" \
  --skip-log-bin \
  --local-infile=OFF \
  --secure-file-priv=NULL)"
[ -n "$CONTAINER_ID" ] || fail 'Docker did not return an owned container ID'
[ "$(docker_local inspect --format '{{index .Config.Labels "joincare.fair-launch.disposable-test"}}' "$CONTAINER_ID")" = "$RUN_ID" ] || fail 'created container ownership label mismatch'

for _ in $(seq 1 90); do
  if docker_local exec "$CONTAINER_ID" sh -c \
      'MYSQL_PWD="$(cat /run/secrets/jc-fl-root-password)" exec mysqladmin --protocol=tcp -h127.0.0.1 -uroot ping --silent' \
      >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker_local exec "$CONTAINER_ID" sh -c \
  'MYSQL_PWD="$(cat /run/secrets/jc-fl-root-password)" exec mysqladmin --protocol=tcp -h127.0.0.1 -uroot ping --silent' \
  >/dev/null || fail 'MySQL did not become ready'

mysql_root() {
  docker_local exec -i "$CONTAINER_ID" sh -c \
    'MYSQL_PWD="$(cat /run/secrets/jc-fl-root-password)" exec mysql --protocol=tcp -h127.0.0.1 --batch --raw --skip-column-names -uroot "$@"' \
    sh "$@"
}

mysql_db() {
  mysql_root "$DB"
}

query() {
  printf '%s\n' "$1" | mysql_db | tr -d '\r'
}

assert_eq() {
  local label="$1" actual="$2" expected="$3"
  [ "$actual" = "$expected" ] || fail "$label: got=$actual want=$expected"
}

echo '[mysql8-disposable] verifying pinned SQL identities'
(cd "$ROOT/migrations/baseline" && sha256sum -c fair-launch-pre-2026-09-03.sql.sha256)
(cd "$ROOT/migrations" && sha256sum -c FAIR_LAUNCH_SHA256SUMS)
(cd "$ROOT/migrations" && sha256sum -c baseline/fair-launch-pre-2026-09-03-ledger.sha256)
[ "$(wc -l < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03-ledger.sha256" | tr -d ' ')" = '43' ] || fail 'pre-0903 ledger manifest must contain exactly 43 entries'
[ "$(tail -1 "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03-ledger.sha256" | awk '{print $2}')" = '*2026_09_02_global_pause_actions.sql' ] || fail 'pre-0903 ledger manifest final entry mismatch'

MYSQL_VERSION="$(printf 'SELECT VERSION();\n' | mysql_root)"
assert_eq 'MySQL version' "$MYSQL_VERSION" '8.4.10'

printf 'DROP DATABASE IF EXISTS `%s`; CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;\n' "$DB" "$DB" | mysql_root

# The baseline is a concatenation of joincare.sql plus migrations through 0902.
# The real migration runner owns schema_migrations outside those SQL files, and
# late baseline migrations verify exact predecessor identities. Reconstructing
# that ledger is part of the disposable fixture; it is not production evidence.
printf '%s\n' "CREATE TABLE schema_migrations (filename VARCHAR(255) NOT NULL PRIMARY KEY, checksum CHAR(64) NOT NULL, status VARCHAR(16) NOT NULL DEFAULT 'applied', applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;" | mysql_db
while read -r checksum marked_filename; do
  filename="${marked_filename#\*}"
  [ -f "$ROOT/migrations/$filename" ] || fail "pre-0903 ledger file missing: $filename"
  printf "INSERT INTO schema_migrations(filename,checksum,status) VALUES ('%s','%s','applied');\n" "$filename" "$checksum" | mysql_db
done < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03-ledger.sha256"

echo '[mysql8-disposable] loading pinned pre-0903 baseline'
mysql_db < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03.sql"

MIGRATIONS=(
  '2026_09_03_fair_launch_project_model.sql'
  '2026_09_04_fair_launch_ce_v2_governance.sql'
  '2026_09_05_fair_launch_signing_release_control.sql'
  '2026_09_06_fair_launch_action_execution.sql'
  '2026_09_07_fair_launch_position_projection.sql'
)
GATE_MESSAGES=(
  'Fair Launch migration requires approved execution gate'
  'Fair Launch CE-V2 migration requires approved execution gate'
  'Fair Launch control migration requires approved execution gate'
  'Fair Launch action execution migration requires approved execution gate'
  'Fair Launch position migration requires approved execution gate'
)

apply_approved() {
  local file="$1"
  { printf 'SET @JC_FAIR_LAUNCH_MIGRATION_APPROVED=1;\n'; cat "$ROOT/migrations/$file"; } | mysql_db
}

for index in "${!MIGRATIONS[@]}"; do
  file="${MIGRATIONS[$index]}"
  expected_message="${GATE_MESSAGES[$index]}"
  echo "[mysql8-disposable] proving no-gate rejection: $file"
  set +e
  mysql_db < "$ROOT/migrations/$file" > "$NO_GATE_OUTPUT" 2>&1
  no_gate_rc=$?
  set -e
  [ "$no_gate_rc" -ne 0 ] || fail "$file unexpectedly accepted an unapproved session"
  grep -Fq "$expected_message" "$NO_GATE_OUTPUT" || {
    cat "$NO_GATE_OUTPUT" >&2
    fail "$file did not return its exact gate error"
  }
  echo "[mysql8-disposable] applying in fresh approved session: $file"
  apply_approved "$file"
done

echo '[mysql8-disposable] proving 0907 rejects a same-count malformed pre-existing guard table'
DRIFT_DB="${DB}_drift"
printf 'CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;\n' "$DRIFT_DB" | mysql_root
printf 'USE `%s`; CREATE TABLE schema_migrations (filename VARCHAR(255) NOT NULL PRIMARY KEY, checksum CHAR(64) NOT NULL, status VARCHAR(16) NOT NULL DEFAULT '\''applied'\'', applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;\n' "$DRIFT_DB" | mysql_root
while read -r checksum marked_filename; do
  filename="${marked_filename#\*}"
  printf "USE \`%s\`; INSERT INTO schema_migrations(filename,checksum,status) VALUES ('%s','%s','applied');\n" "$DRIFT_DB" "$filename" "$checksum" | mysql_root
done < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03-ledger.sha256"
{ printf 'USE `%s`;\n' "$DRIFT_DB"; cat "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03.sql"; } | mysql_root
for file in "${MIGRATIONS[@]:0:4}"; do
  { printf 'USE `%s`; SET @JC_FAIR_LAUNCH_MIGRATION_APPROVED=1;\n' "$DRIFT_DB"; cat "$ROOT/migrations/$file"; } | mysql_root
done
printf 'USE `%s`; CREATE TABLE fair_launch_effect_application (event_identity_id BIGINT UNSIGNED NOT NULL,effect_version INT UNSIGNED NOT NULL,application_attempt_id BIGINT UNSIGNED NOT NULL,observation_id BIGINT UNSIGNED NOT NULL,event_name VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,payload_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,effect_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,applied_at DATETIME(6) NOT NULL,PRIMARY KEY(event_identity_id,effect_version),UNIQUE KEY uk_fl_effect_application_attempt(application_attempt_id,event_identity_id),UNIQUE KEY uk_fl_effect_application_observation(observation_id,event_identity_id),CONSTRAINT chk_fl_effect_application CHECK(event_identity_id>0 AND effect_version>0 AND application_attempt_id>0 AND observation_id>0 AND event_name REGEXP '\''^FairLaunch[A-Za-z]+$'\'' AND payload_digest REGEXP '\''^[0-9a-f]{64}$'\'' AND effect_digest REGEXP '\''^[0-9a-f]{64}$'\'') ENFORCED) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;\n' "$DRIFT_DB" | mysql_root
set +e
{ printf 'USE `%s`; SET @JC_FAIR_LAUNCH_MIGRATION_APPROVED=1;\n' "$DRIFT_DB"; cat "$ROOT/migrations/2026_09_07_fair_launch_position_projection.sql"; } | mysql_root > "$NO_GATE_OUTPUT" 2>&1
drift_rc=$?
set -e
[ "$drift_rc" -ne 0 ] || fail '0907 accepted malformed pre-existing guard table'
grep -Fq 'fair_launch_effect_application exact foreign key set mismatch' "$NO_GATE_OUTPUT" || { cat "$NO_GATE_OUTPUT" >&2; fail '0907 missing-FK rejection was not exact'; }
printf 'USE `%s`; DROP PROCEDURE IF EXISTS jc_fair_launch_position_projection_0907; DROP TABLE fair_launch_effect_application; CREATE TABLE fair_launch_effect_application (event_identity_id BIGINT UNSIGNED NOT NULL,effect_version INT UNSIGNED NOT NULL,application_attempt_id BIGINT UNSIGNED NOT NULL,observation_id BIGINT UNSIGNED NOT NULL,event_name VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,payload_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,effect_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,applied_at DATETIME(6) NOT NULL,CONSTRAINT chk_fl_effect_application CHECK(event_identity_id>0 AND effect_version>0 AND application_attempt_id>0 AND observation_id>0 AND event_name REGEXP '\''^FairLaunch[A-Za-z]+$'\'' AND payload_digest REGEXP '\''^[0-9a-f]{64}$'\'' AND effect_digest REGEXP '\''^[0-9a-f]{64}$'\'') ENFORCED) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;\n' "$DRIFT_DB" | mysql_root
set +e
{ printf 'USE `%s`; SET @JC_FAIR_LAUNCH_MIGRATION_APPROVED=1;\n' "$DRIFT_DB"; cat "$ROOT/migrations/2026_09_07_fair_launch_position_projection.sql"; } | mysql_root > "$NO_GATE_OUTPUT" 2>&1
drift_rc=$?
set -e
[ "$drift_rc" -ne 0 ] || fail '0907 accepted pre-existing guard table without indexes'
grep -Fq 'fair_launch_effect_application exact index set mismatch' "$NO_GATE_OUTPUT" || { cat "$NO_GATE_OUTPUT" >&2; fail '0907 missing-index rejection was not exact'; }
printf 'DROP DATABASE `%s`;\n' "$DRIFT_DB" | mysql_root

echo '[mysql8-disposable] reapplying the complete 0903-0907 set for idempotence'
for file in "${MIGRATIONS[@]}"; do
  apply_approved "$file"
done

FAIR_LAUNCH_TABLES="$(query "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE' AND TABLE_NAME LIKE 'fair_launch%';")"
CE_V2_TABLES="$(query "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE' AND (TABLE_NAME LIKE 'chain_%_v2' OR TABLE_NAME LIKE 'ce_v2%' OR TABLE_NAME LIKE 'source_materialization%');")"
FAIR_LAUNCH_VIEWS="$(query "SELECT COUNT(*) FROM information_schema.VIEWS WHERE TABLE_SCHEMA=DATABASE() AND (TABLE_NAME LIKE '%gate%' OR TABLE_NAME LIKE '%gap%');")"
FAIR_LAUNCH_TRIGGERS="$(query "SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND (TRIGGER_NAME LIKE 'trg_fl_%' OR TRIGGER_NAME LIKE 'trg_ce_%');")"
ENFORCED_CHECKS="$(query "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS tc JOIN information_schema.CHECK_CONSTRAINTS cc ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK' AND tc.ENFORCED='YES' AND (tc.TABLE_NAME LIKE 'fair_launch%' OR tc.TABLE_NAME LIKE 'chain_%_v2' OR tc.TABLE_NAME LIKE 'ce_v2%' OR tc.TABLE_NAME LIKE 'governance_%');")"
TEMPORARY_ROUTINES="$(query "SELECT COUNT(*) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA=DATABASE() AND ROUTINE_NAME IN ('jc_fair_launch_project_model_0903','jc_require_fair_launch_gate_0904','jc_fair_launch_control_0905','jc_fair_launch_action_execution_0906');")"
FUNDING_MODE_NULLABLE="$(query "SELECT IS_NULLABLE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='funding_mode';")"
SOURCE_REGISTRY_COLUMNS="$(query "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_source_registry_snapshot';")"
ACTION_STATUS_CHECK="$(query "SELECT COUNT(*) FROM information_schema.CHECK_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_NAME='chk_fl_action_request';")"

assert_eq 'Fair Launch table count' "$FAIR_LAUNCH_TABLES" '15'
assert_eq 'CE-V2 table count' "$CE_V2_TABLES" '9'
assert_eq 'gate/gap view count' "$FAIR_LAUNCH_VIEWS" '5'
assert_eq 'Fair Launch trigger count' "$FAIR_LAUNCH_TRIGGERS" '9'
assert_eq 'enforced check count' "$ENFORCED_CHECKS" '28'
assert_eq 'temporary migration routine count' "$TEMPORARY_ROUTINES" '0'
assert_eq 'projects.funding_mode nullability' "$FUNDING_MODE_NULLABLE" 'NO'
assert_eq 'Source Registry snapshot column count' "$SOURCE_REGISTRY_COLUMNS" '22'
assert_eq 'action request check count' "$ACTION_STATUS_CHECK" '1'
assert_eq 'position exact column count' "$(query "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_position';")" '22'
assert_eq 'position identity unique index' "$(query "SELECT COUNT(*) FROM (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_position' AND NON_UNIQUE=0 GROUP BY INDEX_NAME HAVING GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',')='chain_id,contract_address,contract_version,project_id,wallet_address') x;")" '1'
assert_eq 'position foreign key count' "$(query "SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_position';")" '5'
assert_eq 'effect application exact column count' "$(query "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_effect_application';")" '8'
assert_eq 'effect application foreign key count' "$(query "SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_effect_application';")" '3'

{
  echo 'authority=TEST_ONLY_NOT_PRODUCTION_ADMITTED'
  echo 'artifact_kind=SANITIZED_DISPOSABLE_MYSQL_TEST_LOG_NOT_SCHEMA_CANDIDATE'
  echo "image=$IMAGE"
  echo "mysql_version=$MYSQL_VERSION"
  echo "database=$DB"
  echo 'network=none'
  echo 'storage=tmpfs_disposable'
  echo "baseline_sha256=$(sha256sum "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03.sql" | awk '{print $1}')"
  for file in "${MIGRATIONS[@]}"; do
    echo "migration_sha256=$file:$(sha256sum "$ROOT/migrations/$file" | awk '{print $1}')"
  done
  echo 'migration_gate_rejections=5/5'
  echo 'fresh_session_prefix_apply=PASS'
  echo 'complete_set_idempotent_reapply=PASS'
  echo "fair_launch_tables=$FAIR_LAUNCH_TABLES"
  echo "ce_v2_tables=$CE_V2_TABLES"
  echo "fair_launch_views=$FAIR_LAUNCH_VIEWS"
  echo "fair_launch_triggers=$FAIR_LAUNCH_TRIGGERS"
  echo "enforced_checks=$ENFORCED_CHECKS"
  echo "temporary_migration_routines=$TEMPORARY_ROUTINES"
  echo "funding_mode_nullable=$FUNDING_MODE_NULLABLE"
  echo "source_registry_snapshot_columns=$SOURCE_REGISTRY_COLUMNS"
  echo "action_status_constraint=$ACTION_STATUS_CHECK"
  echo 'fair_launch_position_schema=PASS'
  echo 'fair_launch_effect_application_schema=PASS'
  echo 'verdict=PASS'
} > "$EVIDENCE"
chmod 600 "$EVIDENCE" 2>/dev/null || true

echo "[mysql8-disposable] PASS evidence=$EVIDENCE"
echo "[mysql8-disposable] evidence_sha256=$(sha256sum "$EVIDENCE" | awk '{print $1}')"
