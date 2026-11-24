#!/usr/bin/env bash
set -euo pipefail

# TEST ONLY. Exercises controlled DDL interruption and exact-source reapplication
# on disposable MySQL 8. This is not a production resume runner, checkpoint
# collector, schema authority, or admission mechanism.

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
IMAGE='mysql@sha256:c831a0f11348d402b43d77453e17d770be2eef356615a2823fe0f5a0d6c8b9af'
DOCKER_SOCKET='/var/run/docker.sock'
RUN_ID="$(openssl rand -hex 16)"
NAME="jc-fl-mysql-resume-$RUN_ID"
OWNER_LABEL="joincare.fair-launch.interruption-resume-test=$RUN_ID"
DB=''
REFERENCE_DB="jc_fl_resume_reference_$RUN_ID"
SECRET_DIR=''
SECRET_FILE=''
INTERRUPT_OUTPUT="/tmp/jc-fl-mysql8-interrupt-$RUN_ID.out"
INTERRUPT_SQL="/tmp/jc-fl-mysql8-interrupt-$RUN_ID.sql"
EVIDENCE="${JOINCARE_FL_MYSQL8_RESUME_TEST_EVIDENCE:-/tmp/joincare-fair-launch-mysql8-interruption-resume-$RUN_ID.txt}"
CONTAINER_ID=''

MIGRATIONS=(
  '2026_09_03_fair_launch_project_model.sql'
  '2026_09_04_fair_launch_ce_v2_governance.sql'
  '2026_09_05_fair_launch_signing_release_control.sql'
  '2026_09_06_fair_launch_action_execution.sql'
)
POST_MIGRATIONS=(
  '2026_09_07_fair_launch_position_projection.sql'
)

fail() {
  echo "fair_launch_mysql8_interruption_resume_test: FAIL: $*" >&2
  exit 1
}

cleanup() {
  if [ -n "$CONTAINER_ID" ]; then
    observed_label="$(docker_local inspect --format '{{index .Config.Labels "joincare.fair-launch.interruption-resume-test"}}' "$CONTAINER_ID" 2>/dev/null || true)"
    if [ "$observed_label" = "$RUN_ID" ]; then
      docker_local rm -f "$CONTAINER_ID" >/dev/null 2>&1 || true
    fi
  fi
  [ -z "$SECRET_DIR" ] || rm -rf "$SECRET_DIR"
  rm -f "$INTERRUPT_OUTPUT" "$INTERRUPT_SQL"
}

for forbidden in JC_DEPLOY_RPC JC_DEPLOY_CHAIN_ID JC_DEPLOY_PRIVATE_KEY JC_MYSQL_ROOT_PASSWORD MYSQL_PWD FAIR_LAUNCH_DISPOSABLE_MYSQL_DSN DOCKER_HOST DOCKER_CONTEXT DOCKER_TLS_VERIFY DOCKER_CERT_PATH; do
  [ -z "${!forbidden:-}" ] || fail "refusing production/external database environment: $forbidden"
done

command -v docker >/dev/null || fail 'docker is required'
command -v openssl >/dev/null || fail 'openssl is required'
command -v sha256sum >/dev/null || fail 'sha256sum is required'
command -v awk >/dev/null || fail 'awk is required'
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

echo '[mysql8-resume] starting network-none, tmpfs-backed MySQL 8.4'
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
[ "$(docker_local inspect --format '{{index .Config.Labels "joincare.fair-launch.interruption-resume-test"}}' "$CONTAINER_ID")" = "$RUN_ID" ] || fail 'created container ownership label mismatch'

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
  [ -n "$DB" ] || fail 'database selector is empty'
  mysql_root "$DB"
}

query() {
  printf '%s\n' "$1" | mysql_db | tr -d '\r'
}

assert_eq() {
  local label="$1" actual="$2" expected="$3"
  [ "$actual" = "$expected" ] || fail "$label: got=$actual want=$expected"
}

echo '[mysql8-resume] verifying pinned SQL identities'
(cd "$ROOT/migrations/baseline" && sha256sum -c fair-launch-pre-2026-09-03.sql.sha256)
(cd "$ROOT/migrations" && sha256sum -c FAIR_LAUNCH_SHA256SUMS)
(cd "$ROOT/migrations" && sha256sum -c baseline/fair-launch-pre-2026-09-03-ledger.sha256)
[ "$(wc -l < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03-ledger.sha256" | tr -d ' ')" = '43' ] || fail 'pre-0903 ledger manifest must contain exactly 43 entries'

MYSQL_VERSION="$(printf 'SELECT VERSION();\n' | mysql_root)"
assert_eq 'MySQL version' "$MYSQL_VERSION" '8.4.10'

create_baseline_database() {
  local database="$1"
  printf 'DROP DATABASE IF EXISTS `%s`; CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;\n' "$database" "$database" | mysql_root
  DB="$database"
  printf '%s\n' "CREATE TABLE schema_migrations (filename VARCHAR(255) NOT NULL PRIMARY KEY, checksum CHAR(64) NOT NULL, status VARCHAR(16) NOT NULL DEFAULT 'applied', applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;" | mysql_db
  while read -r checksum marked_filename; do
    filename="${marked_filename#\*}"
    [ -f "$ROOT/migrations/$filename" ] || fail "pre-0903 ledger file missing: $filename"
    printf "INSERT INTO schema_migrations(filename,checksum,status) VALUES ('%s','%s','applied');\n" "$filename" "$checksum" | mysql_db
  done < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03-ledger.sha256"
  mysql_db < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03.sql"
}

migration_checksum() {
  sha256sum "$ROOT/migrations/$1" | awk '{print $1}'
}

ledger_begin() {
  local file="$1" checksum
  checksum="$(migration_checksum "$file")"
  printf "INSERT INTO schema_migrations(filename,checksum,status) VALUES ('%s','%s','applying');\n" "$file" "$checksum" | mysql_db
}

ledger_finish() {
  local file="$1" checksum updated
  checksum="$(migration_checksum "$file")"
  updated="$(query "UPDATE schema_migrations SET status='applied',applied_at=CURRENT_TIMESTAMP(6) WHERE filename='$file' AND checksum='$checksum' AND status='applying'; SELECT ROW_COUNT();")"
  assert_eq "$file applying-to-applied ledger transition" "$updated" '1'
}

apply_approved() {
  local file="$1"
  { printf 'SET @JC_FAIR_LAUNCH_MIGRATION_APPROVED=1;\n'; cat "$ROOT/migrations/$file"; } | mysql_db
}

apply_with_ledger() {
  local file="$1"
  ledger_begin "$file"
  apply_approved "$file"
  ledger_finish "$file"
}

table_ddl_sha() {
  local table="$1"
  query "SHOW CREATE TABLE \`$table\`;" | sha256sum | awk '{print $1}'
}

schema_fingerprint() {
  {
    query "SELECT CONCAT_WS('|','T',TABLE_NAME,TABLE_TYPE,COALESCE(ENGINE,''),COALESCE(TABLE_COLLATION,'')) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() ORDER BY TABLE_TYPE,TABLE_NAME;"
    query "SELECT CONCAT_WS('|','C',TABLE_NAME,COLUMN_NAME,ORDINAL_POSITION,COLUMN_TYPE,IS_NULLABLE,COALESCE(COLUMN_DEFAULT,'<NULL>'),COALESCE(CHARACTER_SET_NAME,'<NULL>'),COALESCE(COLLATION_NAME,'<NULL>'),EXTRA,COALESCE(GENERATION_EXPRESSION,'')) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() ORDER BY TABLE_NAME,ORDINAL_POSITION;"
    query "SELECT CONCAT_WS('|','I',TABLE_NAME,INDEX_NAME,SEQ_IN_INDEX,NON_UNIQUE,COALESCE(COLUMN_NAME,EXPRESSION),INDEX_TYPE) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() ORDER BY TABLE_NAME,INDEX_NAME,SEQ_IN_INDEX;"
    query "SELECT CONCAT_WS('|','F',k.TABLE_NAME,k.CONSTRAINT_NAME,k.ORDINAL_POSITION,k.COLUMN_NAME,k.REFERENCED_TABLE_NAME,k.REFERENCED_COLUMN_NAME,r.UPDATE_RULE,r.DELETE_RULE) FROM information_schema.KEY_COLUMN_USAGE k JOIN information_schema.REFERENTIAL_CONSTRAINTS r ON r.CONSTRAINT_SCHEMA=k.CONSTRAINT_SCHEMA AND r.CONSTRAINT_NAME=k.CONSTRAINT_NAME AND r.TABLE_NAME=k.TABLE_NAME WHERE k.CONSTRAINT_SCHEMA=DATABASE() AND k.REFERENCED_TABLE_NAME IS NOT NULL ORDER BY k.TABLE_NAME,k.CONSTRAINT_NAME,k.ORDINAL_POSITION;"
    query "SELECT CONCAT_WS('|','K',tc.TABLE_NAME,cc.CONSTRAINT_NAME,cc.CHECK_CLAUSE,tc.ENFORCED) FROM information_schema.CHECK_CONSTRAINTS cc JOIN information_schema.TABLE_CONSTRAINTS tc ON tc.CONSTRAINT_SCHEMA=cc.CONSTRAINT_SCHEMA AND tc.CONSTRAINT_NAME=cc.CONSTRAINT_NAME WHERE cc.CONSTRAINT_SCHEMA=DATABASE() ORDER BY tc.TABLE_NAME,cc.CONSTRAINT_NAME;"
    query "SELECT CONCAT_WS('|','V',TABLE_NAME,REPLACE(VIEW_DEFINITION,DATABASE(),'<DB>'),CHECK_OPTION,SECURITY_TYPE) FROM information_schema.VIEWS WHERE TABLE_SCHEMA=DATABASE() ORDER BY TABLE_NAME;"
    query "SELECT CONCAT_WS('|','G',TRIGGER_NAME,EVENT_MANIPULATION,ACTION_TIMING,EVENT_OBJECT_TABLE,ACTION_ORDER,DEFINER,REPLACE(ACTION_STATEMENT,DATABASE(),'<DB>')) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() ORDER BY TRIGGER_NAME;"
    query "SELECT CONCAT_WS('|','R',ROUTINE_NAME,ROUTINE_TYPE,SQL_DATA_ACCESS,SECURITY_TYPE,DEFINER,REPLACE(COALESCE(ROUTINE_DEFINITION,''),DATABASE(),'<DB>')) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA=DATABASE() ORDER BY ROUTINE_NAME;"
  } | sha256sum | awk '{print $1}'
}

emit_interrupted_sql() {
  local phase="$1" file="$2"
  case "$phase" in
    0903)
      awk '
        { print }
        /ALTER TABLE projects ADD COLUMN funding_mode VARCHAR\(32\)/ {
          print "    SIGNAL SQLSTATE '\''45000'\'' SET MESSAGE_TEXT='\''TEST_ONLY_CONTROLLED_INTERRUPTION_0903'\'';"
          inserted++
        }
        END { if (inserted != 1) exit 42 }
      ' "$ROOT/migrations/$file"
      ;;
    *) fail "unknown interruption phase: $phase" ;;
  esac
}

interrupt_current_migration() {
  local phase="$1" file="$2" rc target=''
  case "$phase" in
    0904) target='chain_contract_source' ;;
    0905) target='fair_launch_signing_generation' ;;
    0906) target='fair_launch_action_request' ;;
  esac
  ledger_begin "$file"
  if [ "$phase" = '0903' ]; then
    { printf 'SET @JC_FAIR_LAUNCH_MIGRATION_APPROVED=1;\n'; emit_interrupted_sql "$phase" "$file"; } > "$INTERRUPT_SQL"
  else
    {
      printf 'SET @JC_FAIR_LAUNCH_MIGRATION_APPROVED=1;\n'
      case "$phase" in
        0904) sed -n '1,29p' "$ROOT/migrations/$file" ;;
        0905) sed -n '1,44p' "$ROOT/migrations/$file" ;;
        0906) sed -n '1,142p' "$ROOT/migrations/$file" ;;
      esac
    } > "$INTERRUPT_SQL"
  fi
  if [ "$phase" != '0903' ]; then
    assert_eq "$phase partial SQL target count" "$(grep -Fc "CREATE TABLE IF NOT EXISTS $target (" "$INTERRUPT_SQL")" '1'
    assert_eq "$phase partial SQL terminal boundary" "$(tail -1 "$INTERRUPT_SQL" | tr -d '\r')" ') ENGINE=InnoDB;'
  fi
  set +e
  mysql_db < "$INTERRUPT_SQL" > "$INTERRUPT_OUTPUT" 2>&1
  rc=$?
  set -e
  if [ "$phase" = '0903' ]; then
    [ "$rc" -ne 0 ] || fail "$file did not stop at the controlled interruption point"
  else
    [ "$rc" -eq 0 ] || { cat "$INTERRUPT_OUTPUT" >&2; fail "$file controlled partial-prefix execution failed"; }
  fi
  if [ "$phase" = '0903' ]; then
    grep -Fq 'TEST_ONLY_CONTROLLED_INTERRUPTION_0903' "$INTERRUPT_OUTPUT" || fail '0903 interruption did not occur after the funding_mode DDL'
  fi
  assert_eq "$phase applying ledger row count" "$(query "SELECT COUNT(*) FROM schema_migrations WHERE filename='$file' AND status='applying';")" '1'
  assert_eq "$phase unrelated applying ledger rows" "$(query "SELECT COUNT(*) FROM schema_migrations WHERE status='applying' AND filename<>'$file';")" '0'
}

assert_interrupted_shape() {
  local phase="$1" reference_sha="$2"
  case "$phase" in
    0903)
      assert_eq '0903 exact nullable funding_mode column' "$(query "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='funding_mode' AND COLUMN_TYPE='varchar(32)' AND IS_NULLABLE='YES' AND COLUMN_DEFAULT IS NULL AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND EXTRA='';")" '1'
      assert_eq '0903 partial content' "$(query "SELECT COUNT(*) FROM projects WHERE funding_mode IS NOT NULL;")" '0'
      assert_eq '0903 same-phase suffix absent' "$(query "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_project_inventory';")" '0'
      assert_eq '0903 future suffix absent' "$(query "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN ('chain_contract_source','fair_launch_signing_generation','fair_launch_preview');")" '0'
      ;;
    0904)
      assert_eq '0904 exact partial table DDL' "$(table_ddl_sha chain_contract_source)" "$reference_sha"
      assert_eq '0904 partial content' "$(query 'SELECT COUNT(*) FROM chain_contract_source;')" '0'
      assert_eq '0904 same-phase suffix absent' "$(query "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_contract_source_revision';")" '0'
      assert_eq '0904 future suffix absent' "$(query "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN ('fair_launch_signing_generation','fair_launch_preview');")" '0'
      ;;
    0905)
      assert_eq '0905 exact partial table DDL' "$(table_ddl_sha fair_launch_signing_generation)" "$reference_sha"
      assert_eq '0905 partial content' "$(query 'SELECT COUNT(*) FROM fair_launch_signing_generation;')" '0'
      ;;
    0906)
      assert_eq '0906 exact partial table DDL' "$(table_ddl_sha fair_launch_action_request)" "$reference_sha"
      assert_eq '0906 partial content' "$(query 'SELECT COUNT(*) FROM fair_launch_action_request;')" '0'
      ;;
  esac
}

run_test_only_resume_precondition() {
  local phase="$1" file="$2" checksum phase_digest shape_sql content_sql
  checksum="$(migration_checksum "$file")"
  case "$phase" in
    0903)
      shape_sql="SELECT IF(COUNT(*)=1,0,1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='funding_mode' AND COLUMN_TYPE='varchar(32)' AND IS_NULLABLE='YES' AND COLUMN_DEFAULT IS NULL AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND EXTRA=''"
      content_sql="SELECT COUNT(*) FROM projects WHERE funding_mode IS NOT NULL"
      ;;
    0904)
      shape_sql="SELECT IF(COUNT(*)=1,0,1) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_contract_source' AND TABLE_TYPE='BASE TABLE' AND ENGINE='InnoDB'"
      content_sql='SELECT COUNT(*) FROM chain_contract_source'
      ;;
    0905)
      shape_sql="SELECT IF(COUNT(*)=1,0,1) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_signing_generation' AND TABLE_TYPE='BASE TABLE' AND ENGINE='InnoDB'"
      content_sql='SELECT COUNT(*) FROM fair_launch_signing_generation'
      ;;
    0906)
      shape_sql="SELECT IF(COUNT(*)=1,0,1) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_action_request' AND TABLE_TYPE='BASE TABLE' AND ENGINE='InnoDB'"
      content_sql='SELECT COUNT(*) FROM fair_launch_action_request'
      ;;
  esac
  phase_digest="$({ printf '%s\n' "$phase" "$file"; schema_fingerprint; } | sha256sum | awk '{print $1}')"
  {
    printf "SET @JC_FAIR_LAUNCH_RESUME_FILENAME='%s';\n" "$file"
    printf "SET @JC_FAIR_LAUNCH_RESUME_CHECKSUM='%s';\n" "$checksum"
    printf "SET @JC_FL_RESUME_PHASE_DIGEST='%s';\n" "$phase_digest"
    printf 'SET @JC_FL_RESUME_SHAPE_MISMATCHES=(%s);\n' "$shape_sql"
    printf 'SET @JC_FL_RESUME_NONEMPTY_PARTIALS=(%s);\n' "$content_sql"
    cat "$ROOT/migrations/verify/2026_09_fair_launch_resume.sql"
  } | mysql_db >/dev/null
}

echo '[mysql8-resume] building an independent full-schema reference in the same disposable server'
create_baseline_database "$REFERENCE_DB"
for file in "${MIGRATIONS[@]}"; do
  apply_with_ledger "$file"
done
for file in "${POST_MIGRATIONS[@]}"; do
  apply_with_ledger "$file"
done
REFERENCE_FINGERPRINT="$(schema_fingerprint)"
REFERENCE_0904_SHA="$(table_ddl_sha chain_contract_source)"
REFERENCE_0905_SHA="$(table_ddl_sha fair_launch_signing_generation)"
REFERENCE_0906_SHA="$(table_ddl_sha fair_launch_action_request)"
EXPECTED_CHECKPOINT_0903='db0b5ff9ef65f77d63cfe4857fcc7b9e11931a064977b9345301b0dc01de0417'
EXPECTED_CHECKPOINT_0904='3241ea445086ca73b8edd03ab20db7e8f564674a2c369c5d84046820d21672c1'
EXPECTED_CHECKPOINT_0905='2ef25986cddfdee511af43e335a24f0b99c36a9897d4e14824ae7da92e2cf27a'
EXPECTED_CHECKPOINT_0906='3715b939bbf6d2f0fb90190da44e0338b9496d933843d84a3bdb45704ab2bfed'

for index in "${!MIGRATIONS[@]}"; do
  file="${MIGRATIONS[$index]}"
  printf -v phase '09%02d' "$((index + 3))"
  database="jc_fl_resume_${phase}_$RUN_ID"
  echo "[mysql8-resume] controlled interruption and recovery: $phase $file"
  create_baseline_database "$database"
  for ((prefix=0; prefix<index; prefix++)); do
    apply_with_ledger "${MIGRATIONS[$prefix]}"
  done
  case "$phase" in
    0903) reference_sha='' ;;
    0904) reference_sha="$REFERENCE_0904_SHA" ;;
    0905) reference_sha="$REFERENCE_0905_SHA" ;;
    0906) reference_sha="$REFERENCE_0906_SHA" ;;
  esac
  interrupt_current_migration "$phase" "$file"
  assert_interrupted_shape "$phase" "$reference_sha"
  checkpoint_fingerprint="$(schema_fingerprint)"
  expected_checkpoint_variable="EXPECTED_CHECKPOINT_$phase"
  expected_checkpoint_fingerprint="${!expected_checkpoint_variable}"
  [ -n "$expected_checkpoint_fingerprint" ] || fail "$phase approved checkpoint fingerprint is not pinned"
  assert_eq "$phase exact checkpoint information_schema fingerprint" "$checkpoint_fingerprint" "$expected_checkpoint_fingerprint"
  run_test_only_resume_precondition "$phase" "$file"
  apply_approved "$file"
  ledger_finish "$file"
  for ((suffix=index+1; suffix<${#MIGRATIONS[@]}; suffix++)); do
    apply_with_ledger "${MIGRATIONS[$suffix]}"
  done
  for post_file in "${POST_MIGRATIONS[@]}"; do
    apply_with_ledger "$post_file"
  done
  assert_eq "$phase final schema fingerprint" "$(schema_fingerprint)" "$REFERENCE_FINGERPRINT"
  assert_eq "$phase final applying ledger rows" "$(query "SELECT COUNT(*) FROM schema_migrations WHERE status='applying';")" '0'
done

{
  echo 'authority=TEST_ONLY_NOT_PRODUCTION_ADMITTED'
  echo 'artifact_kind=SANITIZED_DISPOSABLE_MYSQL_INTERRUPTION_RESUME_TEST_LOG_NOT_CHECKPOINT_OR_SCHEMA_CANDIDATE'
  echo 'resume_authority=TEST_ONLY_DERIVED_PRECONDITION_NOT_REVIEWER_ADMISSION'
  echo "image=$IMAGE"
  echo "mysql_version=$MYSQL_VERSION"
  echo 'network=none'
  echo 'storage=tmpfs_disposable'
  echo "baseline_sha256=$(sha256sum "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03.sql" | awk '{print $1}')"
  for file in "${MIGRATIONS[@]}"; do
    echo "migration_sha256=$file:$(migration_checksum "$file")"
  done
  for file in "${POST_MIGRATIONS[@]}"; do
    echo "migration_sha256=$file:$(migration_checksum "$file")"
  done
  echo 'controlled_interruption_0903=PASS'
  echo 'controlled_interruption_0904=PASS'
  echo 'controlled_interruption_0905=PASS'
  echo 'controlled_interruption_0906=PASS'
  echo 'exact_source_resume_0903=PASS'
  echo 'exact_source_resume_0904=PASS'
  echo 'exact_source_resume_0905=PASS'
  echo 'exact_source_resume_0906=PASS'
  echo 'exact_checkpoint_information_schema_fingerprint_match=4/4'
  echo 'partial_content_empty_checks=4/4'
  echo 'final_information_schema_fingerprint_match=4/4'
  echo 'post_0907_position_projection_in_final_fingerprint=PASS'
  echo 'production_resume_authority=UNAVAILABLE_UNCHANGED'
  echo 'verdict=PASS'
} > "$EVIDENCE"
chmod 600 "$EVIDENCE" 2>/dev/null || true

echo "[mysql8-resume] PASS evidence=$EVIDENCE"
echo "[mysql8-resume] evidence_sha256=$(sha256sum "$EVIDENCE" | awk '{print $1}')"
