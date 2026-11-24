#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.."&&pwd)";. "$ROOT/scripts/tests/fair_launch_local_e2e_test_cleanup.sh";H="$ROOT/scripts/fair-launch/local-e2e/mysql-port-reservation.py";BASE="$(jc_fl_test_root_create JC_FL_TEST_MYSQL_RESERVATION_V1)";D="$BASE/$(cat /proc/sys/kernel/random/uuid)";mkdir -p "$D/evidence";chmod 700 "$D" "$D/evidence";jc_fl_e2e_phase_write "$D";C_RECORD='';W_RECORD=''
fail(){ echo "fair_launch_local_e2e_mysql_reservation_test: FAIL: $*" >&2;exit 1;}
cleanup(){ local original=$? ok=1;set +e;if [ -n "$W_RECORD" ]&&[ -f "$W_RECORD" ];then jc_fl_test_process_remove "$W_RECORD" TERM||jc_fl_test_process_remove "$W_RECORD" KILL||ok=0;fi;if [ -n "$C_RECORD" ]&&[ -f "$C_RECORD" ];then jc_fl_test_process_remove "$C_RECORD" TERM||ok=0;fi;if [ "$ok" = 1 ]&&[ -n "${P:-}" ]&&[ -d "$P" ];then jc_fl_test_root_finalize "$P"||ok=0;fi;if [ "$ok" = 1 ]&&[ -d "$D" ];then jc_fl_test_run_cleanup "$D"||ok=0;fi;if [ "$ok" = 1 ];then jc_fl_test_root_finalize "$BASE"||ok=0;fi;if [ "$ok" != 1 ];then echo "reservation_test: cleanup retained evidence at $D" >&2;fi;[ "$original" -ne 0 ]&&return "$original";[ "$ok" = 1 ];};trap cleanup EXIT
STATE="$D/reservation.json";FIFO="$D/worker.fifo";CONTROL="$D/control.fifo";LOG="$D/log";OUT="$D/out"
reset_paths(){ rm -f "$CONTROL" "$FIFO" "$STATE" "$LOG" "$OUT";mkfifo -m 600 "$CONTROL";}
wait_worker(){ for _ in $(seq 1 100);do grep -q '^WORKER_PID=' "$OUT" 2>/dev/null&&return 0;sleep .05;done;return 1;}
make_records(){ local tag="$1" wp ws ep;wp="$(sed -n 's/^WORKER_PID=//p' "$OUT")";ws="$(sed -n 's/^WORKER_START_TIME=//p' "$OUT")";ep="$(sed -n 's/^ENDPOINT=//p' "$OUT")";[ "$(jc_fl_e2e_proc_start "$wp")" = "$ws" ]||fail "$tag worker identity";printf 'role=controller\nstate=%s\ncontrol=%s\nworker=%s\nlog=%s\nout=%s\ntag=%s\n' "$STATE" "$CONTROL" "$FIFO" "$LOG" "$OUT" "$tag">"$D/controller.marker";printf 'role=worker\nstate=%s\ncontrol=%s\nworker=%s\nlog=%s\nout=%s\nendpoint=%s\ntag=%s\n' "$STATE" "$CONTROL" "$FIFO" "$LOG" "$OUT" "$ep" "$tag">"$D/worker.marker";chmod 600 "$D/controller.marker" "$D/worker.marker";C_RECORD="$D/evidence/controller-$tag.json";W_RECORD="$D/evidence/worker-$tag.json";jc_fl_test_process_record "$C" "$D/controller.marker" none "$D" "$C_RECORD";jc_fl_test_process_record "$wp" "$D/worker.marker" "$ep" "$D" "$W_RECORD";}
start_controller(){ local tag="$1";reset_paths;bash "$ROOT/scripts/fair-launch/local-e2e/mysql-port-reservation.sh" start "$STATE" "$FIFO" "$LOG" "$H"<"$CONTROL">"$OUT" 2>&1&C=$!;exec 8>"$CONTROL";wait_worker||fail "$tag worker start";python3 "$H" verify "$STATE">/dev/null;make_records "$tag";}
release_controller(){ local c_record="$C_RECORD" w_record="$W_RECORD";printf 'RELEASE\n'>&8;exec 8>&-;wait "$C";python3 "$JC_FL_TEST_CLEANUP_PY" process-absence "$c_record">/dev/null;python3 "$JC_FL_TEST_CLEANUP_PY" process-absence "$w_record">/dev/null;rm -f "$c_record" "$w_record";C_RECORD='';W_RECORD='';}
start_controller normal;ep="$(sed -n 's/^ENDPOINT=//p' "$OUT")";p="${ep#*:}";python3 - "$p" <<'PY'
import socket,sys
s=socket.socket()
try:s.bind(('127.0.0.1',int(sys.argv[1])))
except OSError:raise SystemExit(0)
raise SystemExit(1)
PY
for key in endpoint pid startTime token digest;do cp "$STATE" "$D/t";python3 - "$D/t" "$key" <<'PY'
import json,sys
p,k=sys.argv[1:];o=json.load(open(p));o[k]=('0'*64 if k in ('token','digest') else (999999 if k=='pid' else ('1' if k=='startTime' else '127.0.0.1:1')));open(p,'w').write(json.dumps(o)+'\n')
PY
chmod 600 "$D/t";set +e;python3 "$H" verify "$D/t">/dev/null 2>&1;rc=$?;set -e;[ "$rc" -ne 0 ]||fail "tamper $key";done
release_controller;python3 "$H" release-evidence "$STATE" "$D/released.json";python3 "$H" verify-release "$D/released.json">/dev/null;jc_fl_test_endpoint_closed "$ep"||fail release-endpoint
# Abnormal controller TERM uses its immutable record; worker must then disappear and close the endpoint.
start_controller abnormal;cr="$C_RECORD";wr="$W_RECORD";jc_fl_test_process_remove "$cr" TERM;C_RECORD='';exec 8>&-;for _ in $(seq 1 100);do python3 "$JC_FL_TEST_CLEANUP_PY" process-absence "$wr">/dev/null 2>&1&&break;sleep .05;done;python3 "$JC_FL_TEST_CLEANUP_PY" process-absence "$wr">/dev/null;rm -f "$wr";W_RECORD=''
# Early controller failure publishes identity and exits without a live listener.
reset_paths;set +e;JC_FL_E2E_TEST_RESERVATION_FAIL_EARLY=1 bash "$ROOT/scripts/fair-launch/local-e2e/mysql-port-reservation.sh" start "$STATE" "$FIFO" "$LOG" "$H"<"$CONTROL">"$OUT" 2>&1&C=$!;exec 8>"$CONTROL";wait "$C";early_rc=$?;exec 8>&-;set -e;[ "$early_rc" -ne 0 ]||fail early-rc;grep -q '^EARLY_WORKER_PID=' "$OUT"||fail early-worker
# Injected controller signal failure leaves worker live; both destructive signals use records.
JC_FL_E2E_TEST_RESERVATION_SIGNAL_FAIL=1 start_controller signal-fail;cr="$C_RECORD";wr="$W_RECORD";set +e;jc_fl_test_process_remove "$cr" TERM;signal_rc=$?;set -e;C_RECORD='';exec 8>&-;[ "$signal_rc" = 0 ]||fail controller-term;set +e;wait "$C";controller_rc=$?;set -e;[ "$controller_rc" = 125 ]||fail signal-rc;python3 "$JC_FL_TEST_CLEANUP_PY" process-absence "$wr">/dev/null;rm -f "$wr";W_RECORD=''
# Resistant worker KILL requires the same complete record to verify twice immediately before KILL.
JC_FL_E2E_TEST_RESERVATION_DELAY_EXIT=1 start_controller resistant;cr="$C_RECORD";wr="$W_RECORD";jc_fl_test_process_remove "$cr" TERM;C_RECORD='';exec 8>&-;set +e;wait "$C";residue_rc=$?;set -e;[ "$residue_rc" -ne 0 ]||fail resistant-controller;jc_fl_test_process_remove "$wr" KILL;W_RECORD=''
# PID/start-only mismatch seam must not signal replacement; test cleanup itself uses a complete immutable record.
sleep 60&replacement=$!;printf 'replacement\n'>"$D/replacement.marker";chmod 600 "$D/replacement.marker";replacement_record="$D/evidence/replacement.json";jc_fl_test_process_record "$replacement" "$D/replacement.marker" none "$D" "$replacement_record";out_identity="$(JC_FL_E2E_TEST_CLEANUP_IDENTITY=1 bash "$ROOT/scripts/fair-launch/local-e2e/mysql-port-reservation.sh" test-cleanup-identity "$replacement" 0)";[ "$out_identity" = IDENTITY_CHANGED ]||fail mismatch-result;kill -0 "$replacement";jc_fl_test_process_remove "$replacement_record" TERM
# Parent failure retained run is deleted only after exact Redis cleanup succeeds.
P="$D/parent";mkdir "$P";chmod 700 "$P";set +e;JC_FL_E2E_STATE_ROOT="$P" JC_FL_E2E_TEST_RESERVATION_FAIL_EARLY=1 timeout --foreground 240s bash "$ROOT/scripts/fair-launch/local-e2e/start.sh">"$D/parent.out" 2>&1;parent_rc=$?;set -e;[ "$parent_rc" -ne 0 ]||fail parent-rc;pd="$(find "$P" -mindepth 1 -maxdepth 1 -type d|head -1)";[ "$(stat -Lc '%a' "$pd" "$pd/evidence/start-failure.json")" = $'700\n600' ];grep -Fq reservation-controller-early-failure "$pd/evidence/start-failure.json";jc_fl_test_redis_remove "$pd"||fail parent-redis-cleanup;jc_fl_test_run_delete "$pd"||fail parent-run-delete;jc_fl_test_root_finalize "$P";P=''
C_RECORD='';W_RECORD='';jc_fl_test_run_cleanup "$D";jc_fl_test_root_finalize "$BASE";trap - EXIT
echo 'fair_launch_local_e2e_mysql_reservation_test: PASS complete controller/worker records, resistant KILL authority, digest tamper, release, signal failure, mismatch, and parent early failure'
