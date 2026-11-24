#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MODE="${1:-all}"
BACKEND_ROOT="${JOINCARE_BACKEND_ROOT:-${2:-}}"
MYSQL_CONTAINER="${JOINCARE_MYSQL_CONTAINER:-jc-mysql}"
KEEP_DB="${JOINCARE_KEEP_TEST_DB:-0}"
DB_SUFFIX="${JOINCARE_TEST_DB_SUFFIX:-$(date +%s)_$$}"
DB="joincare_reorg_verify_assessment_worker_${DB_SUFFIX}"
TEST_USER="jc_verify_${DB_SUFFIX//[^a-zA-Z0-9]/_}"
TEST_PASSWORD="$(openssl rand -hex 24)"
OLD_SECURITY_CHECKSUM="4ee32387ab98d9ffd061a9eef6a58582e5ed8b5fcaf80872190160cb4b4ea20a"
NEW_SECURITY_CHECKSUM="2401bf1d558e125406e59b4bb76e3ad4acb724ba60de8867b979114e25c9b1e7"
SECURITY_MIGRATION="2026_08_30_security_control_plane_and_event_fence.sql"
EXPECTED_MIGRATION_COUNT=40

case "$MODE" in all|schema|workers) ;; *) echo "usage: $0 [all|schema|workers] [backend-root]" >&2; exit 2 ;; esac
[[ "$DB" =~ ^joincare_reorg_verify_[a-zA-Z0-9_]+$ ]] || { echo "unsafe test database name" >&2; exit 2; }
[[ "$TEST_USER" =~ ^jc_verify_[a-zA-Z0-9_]+$ ]] || { echo "unsafe test user name" >&2; exit 2; }
[ "${#DB}" -le 64 ] || { echo "test database name exceeds MySQL limit" >&2; exit 2; }
[ "${#TEST_USER}" -le 32 ] || { echo "test user name exceeds MySQL limit" >&2; exit 2; }
[ -n "$BACKEND_ROOT" ] || { echo "set JOINCARE_BACKEND_ROOT or pass backend-root" >&2; exit 2; }
[ -f "$BACKEND_ROOT/go.mod" ] || { echo "backend root does not contain go.mod: $BACKEND_ROOT" >&2; exit 2; }
[ -f "$ROOT/joincare.sql" ] || { echo "missing $ROOT/joincare.sql" >&2; exit 2; }
command -v docker >/dev/null
command -v go >/dev/null
command -v openssl >/dev/null
docker inspect "$MYSQL_CONTAINER" >/dev/null

mysql_root() { docker exec -i "$MYSQL_CONTAINER" sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -uroot "$@"' sh "$@"; }
mysql_db() { mysql_root "$DB"; }

cleanup_user() {
    printf "DROP USER IF EXISTS '%s'@'%%';\n" "$TEST_USER" | mysql_root >/dev/null 2>&1 || true
}
cleanup() {
    cleanup_user
    if [ "$KEEP_DB" != "1" ]; then
        printf "DROP DATABASE IF EXISTS %s;\n" "$DB" | mysql_root >/dev/null 2>&1 || true
    else
        echo "kept verification database: $DB"
    fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

create_test_user() {
    cleanup_user
    printf "CREATE USER '%s'@'%%' IDENTIFIED BY '%s'; GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, INDEX, REFERENCES, TRIGGER, EXECUTE ON %s.* TO '%s'@'%%'; FLUSH PRIVILEGES;\n" "$TEST_USER" "$TEST_PASSWORD" "$DB" "$TEST_USER" | mysql_root >/dev/null
}

assert_security_checksum_policy() {
    local actual
    actual="$(sha256sum "$ROOT/migrations/$SECURITY_MIGRATION" | awk '{print $1}')"
    [ "$actual" = "$NEW_SECURITY_CHECKSUM" ] || { echo "unexpected $SECURITY_MIGRATION checksum: $actual" >&2; exit 1; }
    [ "$actual" != "$OLD_SECURITY_CHECKSUM" ] || { echo "refuse obsolete security migration checksum" >&2; exit 1; }
}

rebuild_database() {
    echo "==> rebuild $DB"
    printf "DROP DATABASE IF EXISTS %s; CREATE DATABASE %s CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;\n" "$DB" "$DB" | mysql_root >/dev/null
    mysql_db < "$ROOT/joincare.sql"
    printf "%s\n" "CREATE TABLE schema_migrations (filename VARCHAR(255) NOT NULL PRIMARY KEY, checksum CHAR(64) NOT NULL, status VARCHAR(16) NOT NULL DEFAULT 'applied', applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;" | mysql_db >/dev/null
    local file name checksum updated expected total applying ledger_checksum
    while IFS= read -r file; do
        name="$(basename "$file")"
        checksum="$(sha256sum "$file" | awk '{print $1}')"
        printf "INSERT INTO schema_migrations(filename,checksum,status) VALUES ('%s','%s','applying');\n" "$name" "$checksum" | mysql_db >/dev/null
        echo "    apply $name"
        if ! mysql_db < "$file"; then echo "migration failed; ledger retained as applying: $name" >&2; return 1; fi
        updated="$(printf "UPDATE schema_migrations SET status='applied',applied_at=CURRENT_TIMESTAMP(6) WHERE filename='%s' AND checksum='%s' AND status='applying'; SELECT ROW_COUNT();\n" "$name" "$checksum" | mysql_db | tr -d '\r[:space:]')"
        [ "$updated" = "1" ] || { echo "ledger transition failed for $name" >&2; return 1; }
    done < <(find "$ROOT/migrations" -maxdepth 1 -type f -name '*.sql' | sort)
    expected="$(find "$ROOT/migrations" -maxdepth 1 -type f -name '*.sql' | wc -l | tr -d ' ')"
    [ "$expected" = "$EXPECTED_MIGRATION_COUNT" ] || { echo "unexpected migration set size: got=$expected want=$EXPECTED_MIGRATION_COUNT" >&2; return 1; }
    [ "$(find "$ROOT/migrations" -maxdepth 1 -type f -name '*.sql' | sort | tail -1 | xargs basename)" = "$SECURITY_MIGRATION" ] || { echo "unexpected final migration" >&2; return 1; }
    total="$(printf "SELECT COUNT(*) FROM schema_migrations WHERE status='applied';\n" | mysql_db | tr -d '\r[:space:]')"
    applying="$(printf "SELECT COUNT(*) FROM schema_migrations WHERE status='applying';\n" | mysql_db | tr -d '\r[:space:]')"
    ledger_checksum="$(printf "SELECT checksum FROM schema_migrations WHERE filename='%s' AND status='applied';\n" "$SECURITY_MIGRATION" | mysql_db | tr -d '\r[:space:]')"
    [ "$total" = "$expected" ] || { echo "migration count mismatch: got=$total want=$expected" >&2; return 1; }
    [ "$applying" = "0" ] || { echo "incomplete migration ledger" >&2; return 1; }
    [ "$ledger_checksum" = "$NEW_SECURITY_CHECKSUM" ] || { echo "security migration ledger checksum mismatch" >&2; return 1; }
    echo "    migrations applied: $total; applying: 0"
}

run_go_test() {
    local pattern="$1" package="$2" mysql_ip dsn
    mysql_ip="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$MYSQL_CONTAINER")"
    [ -n "$mysql_ip" ] || { echo "cannot resolve MySQL bridge IP" >&2; return 1; }
    create_test_user
    dsn="$TEST_USER:$TEST_PASSWORD@tcp($mysql_ip:3306)/$DB?charset=utf8mb4&parseTime=True&loc=UTC"
    (cd "$BACKEND_ROOT" && JOINCARE_MYSQL_TEST_DSN="$dsn" GOEXPERIMENT=jsonv2 go test "$package" -count=1 -run "$pattern" -v)
    cleanup_user
}

run_schema() {
    rebuild_database
    run_go_test 'TestIDOSchema(Gates|NegativeGuards)AgainstMySQL' ./pkg/common/database/schema
}
run_workers() {
    local test
    for test in TestReorgAssessmentWorkerAgainstMySQL TestReorgPrerequisiteSourceDriftAgainstMySQL TestReorgGenerationFoundationAgainstMySQL TestReorgGenerationResourceGateAgainstMySQL TestReorgGenerationSourceDriftAgainstMySQL; do
        rebuild_database
        echo "==> $test"
        run_go_test "^${test}$" ./pkg/job/job/chainevent
    done
}

assert_security_checksum_policy
case "$MODE" in schema) run_schema ;; workers) run_workers ;; all) run_schema; run_workers ;; esac
echo "MySQL migration/schema/reorg verification: PASS"
