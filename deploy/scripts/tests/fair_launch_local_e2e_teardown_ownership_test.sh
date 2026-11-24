#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)";. "$ROOT/scripts/tests/fair_launch_local_e2e_test_cleanup.sh"; STATE_ROOT="$(jc_fl_test_root_create JC_FL_TEST_TEARDOWN_OWNERSHIP_V1)"; RUN_ID=''
cleanup(){ local original=$? ok=1;set +e;if [ -n "$RUN_ID" ]&&[ -d "$STATE_ROOT/$RUN_ID" ];then jc_fl_test_bounded_teardown "$STATE_ROOT" "$RUN_ID"||ok=0;fi;if [ "$ok" = 1 ];then jc_fl_test_root_finalize "$STATE_ROOT"||ok=0;fi;if [ "$ok" != 1 ];then echo "teardown_ownership cleanup retained evidence at $STATE_ROOT/$RUN_ID" >&2;fi;[ "$original" -ne 0 ]&&return "$original";[ "$ok" = 1 ];}
trap cleanup EXIT INT TERM
start(){ local out identity; out="$(JC_FL_E2E_STATE_ROOT="$STATE_ROOT" bash "$ROOT/scripts/fair-launch/local-e2e/start.sh")"; RUN_ID="$(printf '%s\n' "$out"|sed -n 's/^RUN_ID=//p')"; DIR="$STATE_ROOT/$RUN_ID"; JC_FL_E2E_STATE_ROOT="$STATE_ROOT";jc_fl_e2e_load_state "$RUN_ID";identity="$(python3 "$ROOT/scripts/fair-launch/local-e2e/redis-process-record.py" verify "$DIR/redis/process.json")";REDIS_PID="$(printf '%s\n' "$identity"|sed -n 's/^PID=//p')"; }
expect_reject(){ local expected="$1"; set +e; JC_FL_E2E_STATE_ROOT="$STATE_ROOT" bash "$ROOT/scripts/fair-launch/local-e2e/teardown.sh" "$RUN_ID" >/tmp/jc-fl-teardown.out 2>&1; rc=$?; set -e; [ "$rc" -ne 0 ] && [ -d "$DIR" ] && kill -0 "$REDIS_PID" && grep -Fq "$expected" /tmp/jc-fl-teardown.out; }
start
# state.env is parsed as inert data: syntax/duplicate/unknown/mode/symlink attacks fail before ownership checks and never execute payloads.
cp "$DIR/state.env" "$DIR/state.good";printf '%s\n' "PWN='\$(touch $DIR/state-parser-executed)'">>"$DIR/state.env";expect_reject 'run state authority invalid';[ ! -e "$DIR/state-parser-executed" ];mv "$DIR/state.good" "$DIR/state.env"
cp "$DIR/state.env" "$DIR/state.good";printf "RUN_ID='%s'\n" "$RUN_ID">>"$DIR/state.env";expect_reject 'run state authority invalid';mv "$DIR/state.good" "$DIR/state.env"
chmod 640 "$DIR/state.env";expect_reject 'run state authority invalid';chmod 600 "$DIR/state.env"
cp "$DIR/state.env" "$DIR/state.good";rm "$DIR/state.env";ln -s "$DIR/state.good" "$DIR/state.env";expect_reject 'run state authority invalid';rm "$DIR/state.env";mv "$DIR/state.good" "$DIR/state.env"
# Strict Redis record refuses relocation and mode weakening without signalling the target.
cp "$DIR/redis/process.json" "$DIR/redis/relocated.json";chmod 600 "$DIR/redis/relocated.json";set +e;jc_fl_test_redis_remove "$DIR" "$DIR/redis/relocated.json";rc=$?;set -e;[ "$rc" -ne 0 ]&&kill -0 "$REDIS_PID"&&[ -f "$DIR/redis/relocated.json" ]
chmod 640 "$DIR/redis/process.json";expect_reject 'refusing teardown: Redis process ownership mismatch';kill -0 "$REDIS_PID";chmod 600 "$DIR/redis/process.json"
cp "$DIR/state.env" "$DIR/state.good"; sed -i "s/^MYSQL_IMAGE_ID=.*/MYSQL_IMAGE_ID='sha256:0000000000000000000000000000000000000000000000000000000000000000'/" "$DIR/state.env"; expect_reject 'refusing teardown: MySQL transport ownership mismatch'; mv "$DIR/state.good" "$DIR/state.env"
cp "$DIR/mysql-transport/reservation-released.json" "$DIR/mysql-transport/reservation.good"; python3 - "$DIR/mysql-transport/reservation-released.json" <<'PY'
import json,sys
p=sys.argv[1];o=json.load(open(p));o['reservationDigest']='0'*64;open(p,'w').write(json.dumps(o)+'\n')
PY
expect_reject 'refusing teardown: MySQL transport ownership mismatch'; mv "$DIR/mysql-transport/reservation.good" "$DIR/mysql-transport/reservation-released.json"
cp "$DIR/state.env" "$DIR/state.good"; sed -i "s/^MYSQL_TRANSPORT_DIGEST=.*/MYSQL_TRANSPORT_DIGEST='0000000000000000000000000000000000000000000000000000000000000000'/" "$DIR/state.env"; expect_reject 'refusing teardown: MySQL transport ownership mismatch'; mv "$DIR/state.good" "$DIR/state.env"
cp "$DIR/state.env" "$DIR/state.good"; sed -i "s/^MYSQL_ENDPOINT=.*/MYSQL_ENDPOINT='localhost:3306'/" "$DIR/state.env"; expect_reject 'run state authority invalid'; mv "$DIR/state.good" "$DIR/state.env"
cp "$DIR/state.env" "$DIR/state.good"; sed -i "s/^MYSQL_NETWORK_MODE=.*/MYSQL_NETWORK_MODE='bridge'/" "$DIR/state.env"; expect_reject 'run state authority invalid'; mv "$DIR/state.good" "$DIR/state.env"
cp "$DIR/state.env" "$DIR/state.good"; sed -i "s/^MYSQL_SERVER_UUID=.*/MYSQL_SERVER_UUID='00000000-0000-0000-0000-000000000000'/" "$DIR/state.env"; expect_reject 'refusing teardown: MySQL transport ownership mismatch'; mv "$DIR/state.good" "$DIR/state.env"
cp "$DIR/state.env" "$DIR/state.good"; sed -i "s/^MYSQL_CONTAINER_ID=.*/MYSQL_CONTAINER_ID='foreign-container'/" "$DIR/state.env"; expect_reject 'run state authority invalid'; mv "$DIR/state.good" "$DIR/state.env"
for key in startTime exeInode configPath cmdlineHex endpoint socketInode;do cp "$DIR/redis/process.json" "$DIR/redis/process.good";python3 - "$DIR/redis/process.json" "$key" <<'PY'
import json,sys
p,k=sys.argv[1:];o=json.load(open(p));o[k]=('/tmp/foreign.conf' if k=='configPath' else ('0'*64 if k=='cmdlineHex' else ('0:0' if k=='exeInode' else ('127.0.0.1:1' if k=='endpoint' else '0'))));open(p,'w').write(json.dumps(o)+'\n')
PY
chmod 600 "$DIR/redis/process.json";expect_reject 'refusing teardown: Redis process ownership mismatch';kill -0 "$REDIS_PID";[ "$(stat -Lc %a "$DIR/evidence/teardown-failure.json")" = 600 ];mv "$DIR/redis/process.good" "$DIR/redis/process.json";done
JC_FL_E2E_STATE_ROOT="$STATE_ROOT" bash "$ROOT/scripts/fair-launch/local-e2e/teardown.sh" "$RUN_ID" >/dev/null; [ ! -e "$DIR" ]; RUN_ID=''; rm -f /tmp/jc-fl-teardown.out
echo 'fair_launch_local_e2e_teardown_ownership_test: PASS mysql-endpoint network fingerprint stale-pid redis-fingerprint owned-teardown'
