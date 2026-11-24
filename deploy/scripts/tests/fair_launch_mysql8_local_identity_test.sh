#!/usr/bin/env bash
set -euo pipefail

# TEST ONLY. Builds disposable local database identities and proves the current
# positive grant policy and Source Registry stored-procedure boundary on MySQL 8.
# This is not a production identity/grant executor, collector, checkpoint,
# credential transport, transformation, activation, or admission mechanism.

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
IMAGE='mysql@sha256:c831a0f11348d402b43d77453e17d770be2eef356615a2823fe0f5a0d6c8b9af'
DOCKER_SOCKET='/var/run/docker.sock'
RUN_ID="$(openssl rand -hex 16)"
NAME="jc-fl-mysql-identities-$RUN_ID"
OWNER_LABEL="joincare.fair-launch.local-identity-test=$RUN_ID"
DB="jc_fl_local_identities_$RUN_ID"
SECRET_DIR=''
POLICY_DIR=''
EVIDENCE="${JOINCARE_FL_MYSQL8_IDENTITY_TEST_EVIDENCE:-/tmp/joincare-fair-launch-mysql8-local-identities-$RUN_ID.txt}"
OUTPUT="/tmp/jc-fl-mysql8-identities-$RUN_ID.out"
CONTAINER_ID=''
NODE_BIN=''
NODE_ROOT=''

MIGRATIONS=(
  '2026_09_03_fair_launch_project_model.sql'
  '2026_09_04_fair_launch_ce_v2_governance.sql'
  '2026_09_05_fair_launch_signing_release_control.sql'
  '2026_09_06_fair_launch_action_execution.sql'
  '2026_09_07_fair_launch_position_projection.sql'
)

fail() {
  echo "fair_launch_mysql8_local_identity_test: FAIL: $*" >&2
  exit 1
}

docker_local() { docker -H "unix://$DOCKER_SOCKET" "$@"; }

cleanup() {
  if [ -n "$CONTAINER_ID" ]; then
    observed_label="$(docker_local inspect --format '{{index .Config.Labels "joincare.fair-launch.local-identity-test"}}' "$CONTAINER_ID" 2>/dev/null || true)"
    if [ "$observed_label" = "$RUN_ID" ]; then
      docker_local rm -f "$CONTAINER_ID" >/dev/null 2>&1 || true
    fi
  fi
  [ -z "$SECRET_DIR" ] || rm -rf "$SECRET_DIR"
  [ -z "$POLICY_DIR" ] || rm -rf "$POLICY_DIR"
  rm -f "$OUTPUT"
}

for forbidden in JC_DEPLOY_RPC JC_DEPLOY_CHAIN_ID JC_DEPLOY_PRIVATE_KEY JC_MYSQL_ROOT_PASSWORD MYSQL_PWD FAIR_LAUNCH_DISPOSABLE_MYSQL_DSN DOCKER_HOST DOCKER_CONTEXT DOCKER_TLS_VERIFY DOCKER_CERT_PATH; do
  [ -z "${!forbidden:-}" ] || fail "refusing production/external database environment: $forbidden"
done
for key in $(env | cut -d= -f1); do
  case "${key^^}" in
    JC_FAIR_LAUNCH_TEST_*) fail "refusing test-authority environment input: $key" ;;
  esac
done

command -v docker >/dev/null || fail 'docker is required'
command -v openssl >/dev/null || fail 'openssl is required'
command -v sha256sum >/dev/null || fail 'sha256sum is required'
command -v diff >/dev/null || fail 'diff is required'
if command -v node >/dev/null; then
  NODE_BIN="$(command -v node)"
  NODE_ROOT="$ROOT"
elif command -v node.exe >/dev/null; then
  NODE_BIN="$(command -v node.exe)"
  command -v wslpath >/dev/null || fail 'wslpath is required when only node.exe is available'
  NODE_ROOT="$(wslpath -m "$ROOT")"
else
  fail 'node or node.exe is required to render the repository grant policy'
fi
[ -S "$DOCKER_SOCKET" ] || fail "local Docker Unix socket is unavailable: $DOCKER_SOCKET"
[ "$(readlink -f "$DOCKER_SOCKET")" = '/run/docker.sock' ] || fail 'Docker socket does not resolve to the expected local WSL socket'
docker_local version --format '{{.Server.Version}}' >/dev/null 2>&1 || fail 'local Docker daemon is unavailable'
docker_local image inspect "$IMAGE" >/dev/null 2>&1 || fail "pinned local image is unavailable: $IMAGE"
[ ! -e "$EVIDENCE" ] || fail "evidence path already exists: $EVIDENCE"
trap cleanup EXIT INT TERM

umask 077
SECRET_DIR="$(mktemp -d)"
POLICY_DIR="$(mktemp -d)"
for identity in root local_runtime local_admin local_job local_ingestion; do
  openssl rand -hex 32 > "$SECRET_DIR/$identity"
  chmod 600 "$SECRET_DIR/$identity"
done

POLICY_FIXTURE="$NODE_ROOT/scripts/tests/fair_launch_local_db_policy_fixture.js"
"$NODE_BIN" "$POLICY_FIXTURE" tables "$DB" > "$POLICY_DIR/tables"
"$NODE_BIN" "$POLICY_FIXTURE" grants "$DB" > "$POLICY_DIR/grants.sql"
for identity in runtime admin job migration definer ingest; do
  "$NODE_BIN" "$POLICY_FIXTURE" expected "$DB" "$identity" | sort > "$POLICY_DIR/expected-$identity"
done

echo '[mysql8-identities] starting network-none, tmpfs-backed MySQL 8.4'
CONTAINER_ID="$(docker_local run -d \
  --name "$NAME" \
  --label "$OWNER_LABEL" \
  --network none \
  --tmpfs /var/lib/mysql:rw,nosuid,nodev,size=2g \
  --tmpfs /tmp:rw,nosuid,nodev,size=256m \
  -v "$SECRET_DIR:/run/secrets/jc-fl-local-db:ro" \
  -v "$ROOT:/repo:ro" \
  -e MYSQL_ROOT_PASSWORD_FILE=/run/secrets/jc-fl-local-db/root \
  "$IMAGE" \
  --skip-log-bin \
  --local-infile=OFF \
  --secure-file-priv=NULL)"
[ -n "$CONTAINER_ID" ] || fail 'Docker did not return an owned container ID'
[ "$(docker_local inspect --format '{{index .Config.Labels "joincare.fair-launch.local-identity-test"}}' "$CONTAINER_ID")" = "$RUN_ID" ] || fail 'created container ownership label mismatch'

# Keep client credentials out of process argv and environment. The raw random
# secret files are mounted read-only; MySQL clients consume mode-600 option
# files generated inside the container's disposable /tmp tmpfs.
docker_local exec "$CONTAINER_ID" sh -c '
  set -eu
  umask 077
  for profile in root local_runtime local_admin local_job local_ingestion; do
    {
      printf "[client]\nuser=%s\npassword=" "$profile"
      cat "/run/secrets/jc-fl-local-db/$profile"
      printf "host=127.0.0.1\nprotocol=tcp\n"
    } > "/tmp/jc-fl-$profile.cnf"
    chmod 600 "/tmp/jc-fl-$profile.cnf"
  done
' >/dev/null

for _ in $(seq 1 90); do
  if docker_local exec "$CONTAINER_ID" sh -c \
      'exec mysqladmin --defaults-extra-file=/tmp/jc-fl-root.cnf ping --silent' \
      >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker_local exec "$CONTAINER_ID" sh -c \
  'exec mysqladmin --defaults-extra-file=/tmp/jc-fl-root.cnf ping --silent' \
  >/dev/null || fail 'MySQL did not become ready'

mysql_root() {
  docker_local exec -i "$CONTAINER_ID" sh -c \
    'exec mysql --defaults-extra-file=/tmp/jc-fl-root.cnf --batch --raw --skip-column-names "$@"' \
    sh "$@"
}

mysql_as() {
  local user="$1" profile="$2"
  shift 2
  docker_local exec -i "$CONTAINER_ID" sh -c \
    'profile="$1"; user="$2"; shift 2; exec mysql --defaults-extra-file="/tmp/jc-fl-$profile.cnf" -u"$user" --batch --raw --skip-column-names "$@"' \
    sh "$profile" "$user" "$@"
}

mysql_db() { mysql_root "$DB"; }
query_root() { printf '%s\n' "$1" | mysql_db | tr -d '\r'; }

assert_eq() {
  local label="$1" actual="$2" expected="$3"
  [ "$actual" = "$expected" ] || fail "$label: got=$actual want=$expected"
}

expect_denied() {
  local label="$1" user="$2" secret="$3" sql="$4"
  set +e
  printf '%s\n' "$sql" | mysql_as "$user" "$secret" "$DB" > "$OUTPUT" 2>&1
  local rc=$?
  set -e
  [ "$rc" -ne 0 ] || fail "$label unexpectedly succeeded"
  grep -Eq 'command denied|execute command denied|INSERT command denied|UPDATE command denied|DELETE command denied' "$OUTPUT" || {
    cat "$OUTPUT" >&2
    fail "$label did not fail with an authorization denial"
  }
}

normalize_show_grants() {
  sed -E \
    -e "s/ TO \x60([^\x60]+)\x60@\x60([^\x60]+)\x60$/ TO '\\1'@'\\2'/" \
    -e 's/ WITH GRANT OPTION$//' \
    -e '/^GRANT USAGE ON \*\.\*/d' | sort
}

verify_exact_grants() {
  local suffix="$1" identity account
  for identity in runtime admin job migration definer ingest; do
    case "$identity" in
      runtime) account="'local_runtime'@'%'" ;;
      admin) account="'local_admin'@'%'" ;;
      job) account="'local_job'@'%'" ;;
      migration) account="'local_migration'@'localhost'" ;;
      definer) account="'jc_fl_registry_definer'@'localhost'" ;;
      ingest) account="'local_ingestion'@'%'" ;;
    esac
    printf 'SHOW GRANTS FOR %s;\n' "$account" | mysql_root | tr -d '\r' | normalize_show_grants > "$POLICY_DIR/actual-$identity-$suffix"
    diff -u "$POLICY_DIR/expected-$identity" "$POLICY_DIR/actual-$identity-$suffix" || fail "$identity normalized SHOW GRANTS mismatch after $suffix"
  done
}

echo '[mysql8-identities] verifying pinned migration identities and building schema'
(cd "$ROOT/migrations/baseline" && sha256sum -c fair-launch-pre-2026-09-03.sql.sha256)
(cd "$ROOT/migrations" && sha256sum -c FAIR_LAUNCH_SHA256SUMS)
(cd "$ROOT/migrations" && sha256sum -c baseline/fair-launch-pre-2026-09-03-ledger.sha256)
[ "$(wc -l < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03-ledger.sha256" | tr -d ' ')" = '43' ] || fail 'pre-0903 ledger manifest must contain exactly 43 entries'
MYSQL_VERSION="$(printf 'SELECT VERSION();\n' | mysql_root)"
assert_eq 'MySQL version' "$MYSQL_VERSION" '8.4.10'

printf 'CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;\n' "$DB" | mysql_root
printf '%s\n' "CREATE TABLE schema_migrations (filename VARCHAR(255) NOT NULL PRIMARY KEY, checksum CHAR(64) NOT NULL, status VARCHAR(16) NOT NULL DEFAULT 'applied', applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;" | mysql_db
while read -r checksum marked_filename; do
  filename="${marked_filename#\*}"
  printf "INSERT INTO schema_migrations(filename,checksum,status) VALUES ('%s','%s','applied');\n" "$filename" "$checksum" | mysql_db
done < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03-ledger.sha256"
mysql_db < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03.sql"
for file in "${MIGRATIONS[@]}"; do
  { printf 'SET @JC_FAIR_LAUNCH_MIGRATION_APPROVED=1;\n'; cat "$ROOT/migrations/$file"; } | mysql_db
done

missing_tables=''
while read -r table; do
  [ "$(query_root "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='$table';")" = '1' ] || missing_tables="$missing_tables $table"
done < "$POLICY_DIR/tables"
[ -z "$missing_tables" ] || fail "grant-policy tables missing:$missing_tables"

echo '[mysql8-identities] creating disposable identities without password argv/env transport'
docker_local exec -i "$CONTAINER_ID" sh -c '
  root_pw="$(cat /run/secrets/jc-fl-local-db/root)"
  runtime_pw="$(cat /run/secrets/jc-fl-local-db/local_runtime)"
  admin_pw="$(cat /run/secrets/jc-fl-local-db/local_admin)"
  job_pw="$(cat /run/secrets/jc-fl-local-db/local_job)"
  ingest_pw="$(cat /run/secrets/jc-fl-local-db/local_ingestion)"
  mysql --defaults-extra-file=/tmp/jc-fl-root.cnf <<SQL
CREATE USER '\''local_runtime'\''@'\''%'\'' IDENTIFIED BY '\''$runtime_pw'\'';
CREATE USER '\''local_admin'\''@'\''%'\'' IDENTIFIED BY '\''$admin_pw'\'';
CREATE USER '\''local_job'\''@'\''%'\'' IDENTIFIED BY '\''$job_pw'\'';
CREATE USER '\''local_ingestion'\''@'\''%'\'' IDENTIFIED BY '\''$ingest_pw'\'';
CREATE USER '\''local_migration'\''@'\''localhost'\'' ACCOUNT LOCK;
CREATE USER '\''jc_fl_registry_definer'\''@'\''localhost'\'' ACCOUNT LOCK;
SQL
' >/dev/null

mysql_db < "$ROOT/scripts/fair-launch/source-registry-security-objects.sql"
mysql_root < "$POLICY_DIR/grants.sql"

echo '[mysql8-identities] proving exact grants, locked identities, definers, and host matching'
verify_exact_grants initial_install

assert_eq 'locked migration account' "$(query_root "SELECT account_locked FROM mysql.user WHERE user='local_migration' AND host='localhost';")" 'Y'
assert_eq 'locked definer account' "$(query_root "SELECT account_locked FROM mysql.user WHERE user='jc_fl_registry_definer' AND host='localhost';")" 'Y'
assert_eq 'service accounts unlocked' "$(query_root "SELECT COUNT(*) FROM mysql.user WHERE user IN ('local_runtime','local_admin','local_job','local_ingestion') AND host='%' AND account_locked='N';")" '4'
assert_eq 'service authentication populated' "$(query_root "SELECT COUNT(*) FROM mysql.user WHERE user IN ('local_runtime','local_admin','local_job','local_ingestion') AND host='%' AND plugin='caching_sha2_password' AND authentication_string<>'';")" '4'
assert_eq 'default roles absent' "$(query_root "SELECT COUNT(*) FROM mysql.default_roles WHERE USER IN ('local_runtime','local_admin','local_job','local_ingestion','local_migration','jc_fl_registry_definer');")" '0'
assert_eq 'security trigger definers' "$(query_root "SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN ('trg_fl_registry_payload_verify','trg_fl_registry_payload_no_update') AND DEFINER='jc_fl_registry_definer@localhost';")" '2'
assert_eq 'security procedure definer' "$(query_root "SELECT COUNT(*) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA=DATABASE() AND ROUTINE_NAME='jc_insert_fair_launch_source_registry_snapshot' AND DEFINER='jc_fl_registry_definer@localhost' AND SECURITY_TYPE='DEFINER';")" '1'

for spec in 'local_runtime local_runtime' 'local_admin local_admin' 'local_job local_job' 'local_ingestion local_ingestion'; do
  set -- $spec
  observed="$(printf 'SELECT CURRENT_USER();\n' | mysql_as "$1" "$2" "$DB" | tr -d '\r')"
  assert_eq "$1 CURRENT_USER host match" "$observed" "$1@%"
done
set +e
printf 'SELECT 1;\n' | mysql_as local_migration local_runtime "$DB" > "$OUTPUT" 2>&1
locked_rc=$?
set -e
[ "$locked_rc" -ne 0 ] || fail 'locked local_migration unexpectedly authenticated'
grep -Eqi 'Account is locked|Access denied' "$OUTPUT" || { cat "$OUTPUT" >&2; fail 'local_migration authentication denial was not observed'; }

echo '[mysql8-identities] proving least privilege and Source Registry direct-DML denial'
for spec in 'local_runtime local_runtime' 'local_admin local_admin' 'local_job local_job' 'local_ingestion local_ingestion'; do
  set -- $spec
  assert_eq "$1 protected-table SELECT" "$(printf 'SELECT COUNT(*) FROM fair_launch_source_registry_snapshot;\n' | mysql_as "$1" "$2" "$DB" | tr -d '\r')" '0'
done
expect_denied 'runtime protected INSERT' local_runtime local_runtime "INSERT INTO fair_launch_source_registry_snapshot(id) VALUES(1);"
expect_denied 'admin protected INSERT' local_admin local_admin "INSERT INTO fair_launch_source_registry_snapshot(id) VALUES(1);"
expect_denied 'job protected INSERT' local_job local_job "INSERT INTO fair_launch_source_registry_snapshot(id) VALUES(1);"
expect_denied 'ingestion protected INSERT' local_ingestion local_ingestion "INSERT INTO fair_launch_source_registry_snapshot(id) VALUES(1);"
expect_denied 'ingestion protected UPDATE' local_ingestion local_ingestion "UPDATE fair_launch_source_registry_snapshot SET revision=revision;"
expect_denied 'ingestion protected DELETE' local_ingestion local_ingestion "DELETE FROM fair_launch_source_registry_snapshot;"
expect_denied 'runtime procedure EXECUTE' local_runtime local_runtime "CALL jc_insert_fair_launch_source_registry_snapshot('',0,'','','', '',0,'','','','',0,0,0,0,0,0,0,0,NOW(6),NULL,NOW(6));"
expect_denied 'admin cannot mutate Job table' local_admin local_admin "UPDATE chain_tx_submission SET state=state WHERE 1=0;"
expect_denied 'job cannot mutate Admin table' local_job local_job "UPDATE fair_launch_action_request SET state=state WHERE 1=0;"
expect_denied 'runtime cannot mutate Admin table' local_runtime local_runtime "UPDATE fair_launch_action_request SET state=state WHERE 1=0;"
assert_eq 'runtime position SELECT' "$(printf 'SELECT COUNT(*) FROM fair_launch_position;\n' | mysql_as local_runtime local_runtime "$DB" | tr -d '\r')" '0'
expect_denied 'runtime cannot mutate position projection' local_runtime local_runtime "UPDATE fair_launch_position SET status=status WHERE 1=0;"
printf 'UPDATE fair_launch_position SET status=status WHERE 1=0;\n' | mysql_as local_job local_job "$DB" >/dev/null
printf "INSERT INTO fair_launch_effect_application(event_identity_id,effect_version,application_attempt_id,observation_id,event_name,payload_digest,effect_digest,applied_at) SELECT 1,1,1,1,'FairLaunchRefunded',REPEAT('0',64),REPEAT('0',64),NOW(6) FROM DUAL WHERE 1=0;\n" | mysql_as local_job local_job "$DB" >/dev/null

echo '[mysql8-identities] exercising authorized procedure, JCS read-back, reused-session cleanup, and rollback'
PROXY="0x$(printf 'a%.0s' {1..40})"
IMPLEMENTATION="0x$(printf 'b%.0s' {1..40})"
CODE_HASH="$(printf 'c%.0s' {1..64})"
ABI_DIGEST="$(printf 'd%.0s' {1..64})"
ENVELOPE_DIGEST="$(printf 'e%.0s' {1..64})"
SOURCE_ID='900001'
REVISION_ID='900002'
printf "INSERT INTO chain_contract_source(id,chain_id,contract_address,contract_version,funding_mode,registered_at,start_block,enabled) VALUES (%s,31337,'%s','fair_launch_v1','fair_launch',NOW(6),1,1);\n" "$SOURCE_ID" "$PROXY" | mysql_db
printf "INSERT INTO chain_contract_source_revision(id,source_id,revision_no,activation_block_hash,proxy_kind,abi_digest,confirmation_policy,governance_tx_hash,governance_operation_id,implementation_address,implementation_code_hash,dependency_manifest_digest,abi_version,from_block_number,from_transaction_index,from_tx_log_ordinal_inclusive,created_at) VALUES (%s,%s,1,'0x%s','uups','%s','20_blocks','0x%s','0x%s','%s','%s','%s','fair-launch-v1',1,0,0,NOW(6));\n" "$REVISION_ID" "$SOURCE_ID" "$(printf '1%.0s' {1..64})" "$ABI_DIGEST" "$(printf '2%.0s' {1..64})" "$(printf '3%.0s' {1..64})" "$IMPLEMENTATION" "$CODE_HASH" "$(printf '4%.0s' {1..64})" | mysql_db

payload() {
  local revision="$1"
  printf '{"abi_digest":"0x%s","chain_id":"31337","config_generation":"1","environment":"local","implementation_address":"%s","implementation_code_hash":"0x%s","policy_generation":"1","proxy_address":"%s","revision":"%s","runtime_generation":"1","source_id":"%s","source_registry_generation":"1","source_revision_id":"%s","source_revision_no":"1"}' "$ABI_DIGEST" "$IMPLEMENTATION" "$CODE_HASH" "$PROXY" "$revision" "$SOURCE_ID" "$REVISION_ID"
}
call_sql() {
  local revision="$1" canonical="$2" digest="$3"
  printf "CALL jc_insert_fair_launch_source_registry_snapshot('local',%s,'%s','%s','%s','%s',31337,'%s','%s','0x%s','0x%s',%s,%s,1,1,1,1,1,1,'2026-01-01 00:00:00.000000',NULL,'2026-01-01 00:00:00.000000');" "$revision" "$canonical" "$digest" "$digest" "$ENVELOPE_DIGEST" "$PROXY" "$IMPLEMENTATION" "$CODE_HASH" "$ABI_DIGEST" "$SOURCE_ID" "$REVISION_ID"
}

PAYLOAD_1="$(payload 1)"
DIGEST_1="$(printf '%s' "$PAYLOAD_1" | sha256sum | awk '{print $1}')"
BAD_PAYLOAD="$(payload 999)"
BAD_DIGEST="$(printf '%s' "$BAD_PAYLOAD" | sha256sum | awk '{print $1}')"
{
  echo "SELECT CONCAT('PRE=',IF(@JC_FAIR_LAUNCH_VERIFIED_REGISTRY_PAYLOAD_DIGEST IS NULL,'NULL','SET'));"
  call_sql 1 "$PAYLOAD_1" "$DIGEST_1"; echo
  echo "SELECT CONCAT('POST_SUCCESS=',IF(@JC_FAIR_LAUNCH_VERIFIED_REGISTRY_PAYLOAD_DIGEST IS NULL,'NULL','SET'));"
  call_sql 2 "$BAD_PAYLOAD" "$BAD_DIGEST"; echo
  echo "SELECT CONCAT('POST_ERROR=',IF(@JC_FAIR_LAUNCH_VERIFIED_REGISTRY_PAYLOAD_DIGEST IS NULL,'NULL','SET'));"
} | docker_local exec -i "$CONTAINER_ID" sh -c \
  'exec mysql --defaults-extra-file=/tmp/jc-fl-local_ingestion.cnf --force --batch --raw --skip-column-names "$1"' \
  sh "$DB" > "$OUTPUT" 2>&1 || true
grep -Fxq 'PRE=NULL' "$OUTPUT" || { cat "$OUTPUT" >&2; fail 'reused ingestion session was dirty before procedure call'; }
grep -Fxq 'POST_SUCCESS=NULL' "$OUTPUT" || { cat "$OUTPUT" >&2; fail 'procedure success did not clear reused-session evidence'; }
grep -Fq 'canonical payload relational binding mismatch' "$OUTPUT" || { cat "$OUTPUT" >&2; fail 'procedure error path was not exercised'; }
grep -Fxq 'POST_ERROR=NULL' "$OUTPUT" || { cat "$OUTPUT" >&2; fail 'procedure error did not clear reused-session evidence'; }
assert_eq 'successful Source Registry insert count' "$(query_root "SELECT COUNT(*) FROM fair_launch_source_registry_snapshot WHERE environment='local' AND revision=1;")" '1'
READ_BACK="$(query_root "SELECT canonical_payload FROM fair_launch_source_registry_snapshot WHERE environment='local' AND revision=1;")"
assert_eq 'canonical payload byte-for-byte read-back digest' "$(printf '%s' "$READ_BACK" | sha256sum | awk '{print $1}')" "$DIGEST_1"

PAYLOAD_2="$(payload 2)"
DIGEST_2="$(printf '%s' "$PAYLOAD_2" | sha256sum | awk '{print $1}')"
{
  echo 'START TRANSACTION;'
  call_sql 2 "$PAYLOAD_2" "$DIGEST_2"; echo
  echo "SELECT CONCAT('ROLLBACK_SESSION=',IF(@JC_FAIR_LAUNCH_VERIFIED_REGISTRY_PAYLOAD_DIGEST IS NULL,'NULL','SET'));"
  echo 'ROLLBACK;'
  echo "SELECT CONCAT('ROLLBACK_ROWS=',COUNT(*)) FROM fair_launch_source_registry_snapshot WHERE environment='local' AND revision=2;"
} | mysql_as local_ingestion local_ingestion "$DB" > "$OUTPUT" 2>&1
grep -Fxq 'ROLLBACK_SESSION=NULL' "$OUTPUT" || { cat "$OUTPUT" >&2; fail 'transactional procedure call left session evidence'; }
grep -Fxq 'ROLLBACK_ROWS=0' "$OUTPUT" || { cat "$OUTPUT" >&2; fail 'procedure insert did not roll back'; }

set +e
printf "UPDATE fair_launch_source_registry_snapshot SET envelope_digest='%s' WHERE environment='local' AND revision=1;\n" "$(printf 'f%.0s' {1..64})" | mysql_db > "$OUTPUT" 2>&1
append_only_rc=$?
set -e
[ "$append_only_rc" -ne 0 ] || fail 'append-only Source Registry UPDATE unexpectedly succeeded as root'
grep -Fq 'accepted Source Registry snapshot is append-only' "$OUTPUT" || fail 'append-only trigger error was not observed'

BEFORE_REINSTALL="$(query_root "SELECT GROUP_CONCAT(CONCAT(TRIGGER_NAME,'=',DEFINER) ORDER BY TRIGGER_NAME SEPARATOR ',') FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME LIKE 'trg_fl_registry_payload_%'; SELECT CONCAT(ROUTINE_NAME,'=',DEFINER,'=',SECURITY_TYPE) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA=DATABASE() AND ROUTINE_NAME='jc_insert_fair_launch_source_registry_snapshot';")"
mysql_db < "$ROOT/scripts/fair-launch/source-registry-security-objects.sql"
# Replacing the routine removes routine-level EXECUTE grants in MySQL. Treat a
# reinstall as object replacement plus exact repository-policy reprovisioning.
mysql_root < "$POLICY_DIR/grants.sql"
AFTER_REINSTALL="$(query_root "SELECT GROUP_CONCAT(CONCAT(TRIGGER_NAME,'=',DEFINER) ORDER BY TRIGGER_NAME SEPARATOR ',') FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME LIKE 'trg_fl_registry_payload_%'; SELECT CONCAT(ROUTINE_NAME,'=',DEFINER,'=',SECURITY_TYPE) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA=DATABASE() AND ROUTINE_NAME='jc_insert_fair_launch_source_registry_snapshot';")"
assert_eq 'security object reinstall identity' "$AFTER_REINSTALL" "$BEFORE_REINSTALL"
assert_eq 'accepted row preserved across security-object reinstall' "$(query_root "SELECT COUNT(*) FROM fair_launch_source_registry_snapshot WHERE environment='local' AND revision=1;")" '1'
verify_exact_grants post_reinstall
printf '%s\n' "$(call_sql 2 "$PAYLOAD_2" "$DIGEST_2")" | mysql_as local_ingestion local_ingestion "$DB"
assert_eq 'post-reinstall ingestion procedure insert' "$(query_root "SELECT COUNT(*) FROM fair_launch_source_registry_snapshot WHERE environment='local' AND revision=2;")" '1'
expect_denied 'post-reinstall runtime procedure EXECUTE' local_runtime local_runtime "CALL jc_insert_fair_launch_source_registry_snapshot('',0,'','','', '',0,'','','','',0,0,0,0,0,0,0,0,NOW(6),NULL,NOW(6));"

{
  echo 'authority=TEST_ONLY_NOT_PRODUCTION_ADMITTED'
  echo 'artifact_kind=SANITIZED_DISPOSABLE_MYSQL_LOCAL_IDENTITY_AND_PRIVILEGE_TEST_LOG_NOT_POST_STATE_CANDIDATE'
  echo 'credential_material=EPHEMERAL_MODE_600_SECRET_AND_MYSQL_OPTION_FILES_ONLY_NOT_ARGV_ENV_OR_EVIDENCE'
  echo "image=$IMAGE"
  echo "mysql_version=$MYSQL_VERSION"
  echo 'network=none'
  echo 'storage=tmpfs_disposable'
  echo 'local_service_identities=runtime,admin,job,ingestion,migration'
  echo 'exact_normalized_show_grants_initial=6/6'
  echo 'exact_normalized_show_grants_post_reinstall=6/6'
  echo 'account_lock_and_authentication_state=PASS'
  echo 'default_roles_empty=PASS'
  echo 'current_user_host_matching=4/4'
  echo 'cross_role_mutation_denials=3/3'
  echo 'position_runtime_read_job_write_boundary=PASS'
  echo 'protected_snapshot_direct_dml_denials=6/6'
  echo 'runtime_procedure_execute_denial=PASS'
  echo 'ingestion_procedure_execution=PASS'
  echo 'source_registry_append_only_update=PASS'
  echo 'success_error_session_cleanup_same_connection=PASS'
  echo 'procedure_transaction_rollback=PASS'
  echo 'canonical_payload_readback_digest=PASS'
  echo 'security_object_reinstall_and_privilege_reprovision=PASS'
  echo 'post_reinstall_ingestion_execute_and_runtime_denial=PASS'
  echo 'production_privilege_transformation=UNAVAILABLE_UNCHANGED'
  echo 'production_post_state_admission=UNAVAILABLE_UNCHANGED'
  echo 'verdict=PASS'
} > "$EVIDENCE"
chmod 600 "$EVIDENCE" 2>/dev/null || true

echo "[mysql8-identities] PASS evidence=$EVIDENCE"
echo "[mysql8-identities] evidence_sha256=$(sha256sum "$EVIDENCE" | awk '{print $1}')"
