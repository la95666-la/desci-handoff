#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)";. "$ROOT/scripts/tests/fair_launch_local_e2e_test_cleanup.sh"; STATE_ROOT="$(jc_fl_test_root_create JC_FL_TEST_REDIS_SECRET_V1)"; RUN_ID=''; TRACE="$(mktemp)"
cleanup(){ local original=$? ok=1;set +e;if [ -n "$RUN_ID" ]&&[ -d "$STATE_ROOT/$RUN_ID" ];then jc_fl_test_bounded_teardown "$STATE_ROOT" "$RUN_ID"||ok=0;fi;if [ "$ok" = 1 ];then jc_fl_test_root_finalize "$STATE_ROOT"||ok=0;fi;if [ "$ok" = 1 ];then rm -f "$TRACE";else echo "redis_secret cleanup retained evidence at $STATE_ROOT/$RUN_ID" >&2;fi;[ "$original" -ne 0 ]&&return "$original";[ "$ok" = 1 ];}
trap cleanup EXIT INT TERM
set +e; JC_FL_E2E_STATE_ROOT="$STATE_ROOT" bash -x "$ROOT/scripts/fair-launch/local-e2e/start.sh" >"$TRACE" 2>&1; rc=$?; set -e
[ "$rc" = 0 ] || { cat "$TRACE" >&2; exit 1; }
RUN_ID="$(sed -n 's/^RUN_ID=//p' "$TRACE" | tail -1)"; DIR="$STATE_ROOT/$RUN_ID"; JC_FL_E2E_STATE_ROOT="$STATE_ROOT";jc_fl_e2e_load_state "$RUN_ID";SECRET="$(cat "$DIR/secrets/redis-password")"; MYSQL_SECRET="$(cat "$DIR/secrets/mysql-root")";redis_identity="$(python3 "$ROOT/scripts/fair-launch/local-e2e/redis-process-record.py" verify "$DIR/redis/process.json")";REDIS_PID="$(printf '%s\n' "$redis_identity"|sed -n 's/^PID=//p')"
for target in "$TRACE" "$DIR/evidence/infrastructure.json" "$DIR/redis/redis.log" "$DIR/redis/process.json" "$DIR/state.env"; do ! grep -Fq "$SECRET" "$target" || { echo "Redis secret leaked to $target" >&2; exit 1; }; ! grep -Fq "$MYSQL_SECRET" "$target" || { echo "MySQL root secret leaked to $target" >&2; exit 1; }; done
! tr '\0' '\n' < "/proc/$REDIS_PID/cmdline" | grep -Fq "$SECRET"
! tr '\0' '\n' < "/proc/$REDIS_PID/environ" | grep -Fq "$SECRET"
! ps eww -p "$REDIS_PID" | grep -Fq "$SECRET"
! docker -H unix:///var/run/docker.sock inspect "$MYSQL_CONTAINER_ID" | grep -Fq "$MYSQL_SECRET"
[ "$(docker -H unix:///var/run/docker.sock inspect --format '{{.HostConfig.NetworkMode}}|{{json .HostConfig.PortBindings}}|{{json .NetworkSettings.Ports}}' "$MYSQL_CONTAINER_ID")" = 'host|{}|{}' ]
# Closed stdin/wrong password must fail boundedly rather than hang.
set +e; printf 'wrong\n' | timeout 3 redis-cli -h 127.0.0.1 -p "$REDIS_PORT" --askpass ping >/tmp/jc-fl-wrong.out 2>&1; wrong_rc=$?; timeout 3 redis-cli -h 127.0.0.1 -p "$REDIS_PORT" --askpass ping </dev/null >/tmp/jc-fl-closed.out 2>&1; closed_rc=$?; set -e
[ "$wrong_rc" -ne 124 ] && [ "$closed_rc" -ne 124 ]
grep -Eq 'AUTH failed|NOAUTH' /tmp/jc-fl-wrong.out; grep -Eq 'AUTH failed|NOAUTH' /tmp/jc-fl-closed.out; rm -f /tmp/jc-fl-wrong.out /tmp/jc-fl-closed.out
JC_FL_E2E_STATE_ROOT="$STATE_ROOT" bash "$ROOT/scripts/fair-launch/local-e2e/teardown.sh" "$RUN_ID" >/dev/null; [ ! -e "$DIR" ]; RUN_ID=''
echo 'fair_launch_local_e2e_redis_secret_test: PASS trace argv env evidence bounded-failure'
