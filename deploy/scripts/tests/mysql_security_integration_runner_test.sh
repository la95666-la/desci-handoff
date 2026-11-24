#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
RUNNER="$ROOT/scripts/tests/mysql_security_integration_test.sh"
MIGRATION="$ROOT/migrations/2026_08_30_security_control_plane_and_event_fence.sql"
EXPECTED="2401bf1d558e125406e59b4bb76e3ad4acb724ba60de8867b979114e25c9b1e7"

[ -f "$RUNNER" ]
[ "$(sha256sum "$MIGRATION" | awk '{print $1}')" = "$EXPECTED" ]
bash -n "$RUNNER"
grep -Fq 'joincare_reorg_verify_' "$RUNNER"
grep -Fq 'EXPECTED_MIGRATION_COUNT=40' "$RUNNER"
grep -Fq "status='applying'" "$RUNNER"
grep -Fq 'JOINCARE_MYSQL_TEST_DSN="$dsn"' "$RUNNER"
grep -Fq 'DROP USER IF EXISTS' "$RUNNER"
grep -Fq 'DROP DATABASE IF EXISTS' "$RUNNER"
grep -Fq 'JOINCARE_KEEP_TEST_DB' "$RUNNER"
grep -Fq 'TestReorgAssessmentWorkerAgainstMySQL' "$RUNNER"
grep -Fq 'TestReorgGenerationSourceDriftAgainstMySQL' "$RUNNER"
! grep -Eq 'MYSQL_ROOT_PASSWORD[^$]|echo .*TEST_PASSWORD|echo .*dsn' "$RUNNER"

set +e
JOINCARE_TEST_DB_SUFFIX='bad-name' JOINCARE_BACKEND_ROOT="$ROOT" bash "$RUNNER" schema >/dev/null 2>&1
status=$?
set -e
[ "$status" -eq 2 ]

echo "MySQL security integration runner policy: PASS"
