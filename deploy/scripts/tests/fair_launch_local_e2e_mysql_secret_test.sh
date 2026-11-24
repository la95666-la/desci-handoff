#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)";. "$ROOT/scripts/tests/fair_launch_local_e2e_test_cleanup.sh"; STATE_ROOT="$(jc_fl_test_root_create JC_FL_TEST_MYSQL_SECRET_V1)"; RUN_ID=''; TRACE="$(mktemp)"; APPLY_TRACE="$(mktemp)"
fail(){ echo "fair_launch_local_e2e_mysql_secret_test: FAIL: $*" >&2; exit 1; }
cleanup(){ local original=$? ok=1;set +e;if [ -n "$RUN_ID" ]&&[ -d "$STATE_ROOT/$RUN_ID" ];then jc_fl_test_bounded_teardown "$STATE_ROOT" "$RUN_ID"||ok=0;fi;if [ "$ok" = 1 ];then jc_fl_test_root_finalize "$STATE_ROOT"||ok=0;fi;if [ "$ok" = 1 ];then rm -f "$TRACE" "$APPLY_TRACE";else echo "mysql_secret cleanup retained evidence at $STATE_ROOT/$RUN_ID" >&2;fi;[ "$original" -ne 0 ]&&return "$original";[ "$ok" = 1 ];}
trap cleanup EXIT INT TERM
set +e; JC_FL_E2E_STATE_ROOT="$STATE_ROOT" bash -x "$ROOT/scripts/fair-launch/local-e2e/start.sh" >"$TRACE" 2>&1; rc=$?; set -e
[ "$rc" = 0 ] || { cat "$TRACE" >&2; exit 1; }
RUN_ID="$(sed -n 's/^RUN_ID=//p' "$TRACE" | tail -1)"; DIR="$STATE_ROOT/$RUN_ID"; JC_FL_E2E_STATE_ROOT="$STATE_ROOT";jc_fl_e2e_load_state "$RUN_ID"
set +e; JC_FL_E2E_STATE_ROOT="$STATE_ROOT" bash -x "$ROOT/scripts/fair-launch/local-e2e/apply-schema.sh" "$RUN_ID" >"$APPLY_TRACE" 2>&1; rc=$?; set -e
[ "$rc" = 0 ] || { cat "$APPLY_TRACE" >&2; exit 1; }; jc_fl_e2e_load_state "$RUN_ID"
for role in root runtime admin job ingest; do secret="$(cat "$DIR/secrets/mysql-$role")"; for target in "$TRACE" "$APPLY_TRACE" "$DIR/state.env" "$DIR/evidence/infrastructure.json" "$DIR/evidence/database-schema.json" "$DIR/redis/redis.log"; do ! grep -Fq "$secret" "$target" || fail "$role secret leaked to $target"; done; ! docker -H unix:///var/run/docker.sock inspect "$MYSQL_CONTAINER_ID" | grep -Fq "$secret" || fail "$role secret leaked to Docker inspect"; done
for role in runtime admin job ingest; do profile="$DIR/credentials/mysql-$role.cnf"; [ "$(stat -Lc '%a' "$profile")" = 600 ] || fail "$role profile mode"; grep -Fxq 'host=127.0.0.1' "$profile" || fail "$role host"; grep -Fxq "port=$MYSQL_PORT" "$profile" || fail "$role port"; grep -Fxq "database=$DB_NAME" "$profile" || fail "$role database"; grep -Fxq "user=local_$role" "$profile" || fail "$role user"; done
! grep -R -E 'MYSQL_PWD|mysql://|@tcp\(' "$DIR/state.env" "$DIR/evidence" >/dev/null || fail 'secret-bearing transport syntax retained in durable artifacts'
! grep -E 'MYSQL_PWD=|mysql://[^[:space:]]+@' "$TRACE" "$APPLY_TRACE" >/dev/null || fail 'secret-bearing transport value retained in trace'
JC_FL_E2E_STATE_ROOT="$STATE_ROOT" bash "$ROOT/scripts/fair-launch/local-e2e/teardown.sh" "$RUN_ID" >/dev/null; RUN_ID=''
echo 'fair_launch_local_e2e_mysql_secret_test: PASS inherited-xtrace argv env state evidence docker-inspect profiles=0600 exact-loopback'
