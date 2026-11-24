#!/usr/bin/env bash
set -euo pipefail

NAME="jc-daqi-rehearsal"
PASSWORD="rehearsal-only-not-production"
DUMP="${DUMP:-$HOME/.joincare-production-backups/20260920T055002Z-daqi-precreate/joincare-production-pre-daqi.sql}"
SQL_DIR="${SQL_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../dist/daqi-production-draft-sql" && pwd)}"
REPORT="${REPORT:-$SQL_DIR/rehearsal-report.txt}"
PRODUCTION_UUID="78dbed8b-77a9-11f1-ac1c-06eab0296b63"
CREATE_TOKEN="CREATE_DAQI_ARCHIVED_DRAFT_84f0ec230a2d3469"
ROLLBACK_TOKEN="ROLLBACK_DAQI_ARCHIVED_DRAFT_84f0ec230a2d3469"
TMP_SQL="$(mktemp -d /tmp/jc-daqi-sql.XXXXXX)"

cleanup() {
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  rm -rf "$TMP_SQL"
}
trap cleanup EXIT
docker rm -f "$NAME" >/dev/null 2>&1 || true

test -s "$DUMP"
for file in preflight.sql create-daqi-project.sql verify-daqi-project.sql rollback-daqi-project.sql; do
  test -s "$SQL_DIR/$file"
done

: >"$REPORT"
printf 'rehearsal_started_utc=%s\n' "$(date -u +%FT%TZ)" >>"$REPORT"
printf 'dump_sha256=%s\n' "$(sha256sum "$DUMP" | awk '{print $1}')" >>"$REPORT"

docker run -d --name "$NAME" \
  -e MYSQL_ROOT_PASSWORD="$PASSWORD" \
  mysql:8.4 \
  --character-set-server=utf8mb4 \
  --collation-server=utf8mb4_0900_ai_ci >/dev/null

for attempt in $(seq 1 120); do
  if docker exec "$NAME" mysql -N -B -uroot -p"$PASSWORD" -e "SELECT 1" >/dev/null 2>&1; then
    break
  fi
  if [[ "$attempt" == "120" ]]; then
    docker logs "$NAME" >&2
    exit 1
  fi
  sleep 1
done

docker exec "$NAME" mysql -uroot -p"$PASSWORD" \
  -e "CREATE DATABASE joincare CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci" \
  >/dev/null 2>&1
docker exec -i "$NAME" mysql -uroot -p"$PASSWORD" joincare <"$DUMP" 2>>"$REPORT"

REHEARSAL_UUID="$(docker exec "$NAME" mysql -N -B -uroot -p"$PASSWORD" -e "SELECT @@server_uuid" 2>/dev/null)"
for file in preflight.sql create-daqi-project.sql verify-daqi-project.sql rollback-daqi-project.sql; do
  sed "s/$PRODUCTION_UUID/$REHEARSAL_UUID/g" "$SQL_DIR/$file" >"$TMP_SQL/$file"
done

query() {
  docker exec "$NAME" mysql -N -B -uroot -p"$PASSWORD" joincare -e "$1" 2>/dev/null
}

run_sql() {
  local label="$1"
  local file="$2"
  local token="${3:-}"
  printf '[%s]\n' "$label" >>"$REPORT"
  if [[ -n "$token" ]]; then
    docker exec -i "$NAME" mysql -N -B -uroot -p"$PASSWORD" \
      --init-command="SET @jc_confirm_token='$token'" joincare \
      <"$file" >>"$REPORT" 2>&1
  else
    docker exec -i "$NAME" mysql -N -B -uroot -p"$PASSWORD" joincare \
      <"$file" >>"$REPORT" 2>&1
  fi
}

printf 'restore_version=%s\n' "$(docker exec "$NAME" mysql -N -B -uroot -p"$PASSWORD" -e 'SELECT VERSION()' 2>/dev/null)" >>"$REPORT"
printf 'restore_server_uuid=%s\n' "$REHEARSAL_UUID" >>"$REPORT"
printf 'production_server_uuid_binding=%s\n' "$PRODUCTION_UUID" >>"$REPORT"
printf 'uuid_override_scope=temporary_rehearsal_copies_only\n' >>"$REPORT"
printf 'restore_tables=%s\n' "$(query 'SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA="joincare" AND TABLE_TYPE="BASE TABLE"')" >>"$REPORT"
printf 'restore_projects=%s\n' "$(query 'SELECT COUNT(*) FROM projects')" >>"$REPORT"

run_sql preflight "$TMP_SQL/preflight.sql"
run_sql create_first "$TMP_SQL/create-daqi-project.sql" "$CREATE_TOKEN"
run_sql create_second "$TMP_SQL/create-daqi-project.sql" "$CREATE_TOKEN"
run_sql verify "$TMP_SQL/verify-daqi-project.sql"
run_sql rollback_first "$TMP_SQL/rollback-daqi-project.sql" "$ROLLBACK_TOKEN"
run_sql rollback_second "$TMP_SQL/rollback-daqi-project.sql" "$ROLLBACK_TOKEN"

printf 'final_projects=%s\n' "$(query 'SELECT COUNT(*) FROM projects')" >>"$REPORT"
printf 'final_project4=%s\n' "$(query 'SELECT COUNT(*) FROM projects WHERE project_id=4')" >>"$REPORT"
printf 'rehearsal_finished_utc=%s\n' "$(date -u +%FT%TZ)" >>"$REPORT"
printf 'report_sha256=%s\n' "$(sha256sum "$REPORT" | awk '{print $1}')"
cat "$REPORT"
