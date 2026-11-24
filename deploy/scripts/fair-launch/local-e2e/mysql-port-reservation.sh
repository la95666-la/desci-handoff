#!/usr/bin/env bash
set +x
set -euo pipefail
fail(){ echo "fair-launch-local-e2e reservation helper: FAIL: $*" >&2; exit 1; }
proc_start(){ awk '{print $22}' "/proc/$1/stat" 2>/dev/null; }
wait_gone(){ local pid="$1" start="$2"; for _ in $(seq 1 100); do [ ! -r "/proc/$pid/stat" ]&&return 0; [ "$(proc_start "$pid")" != "$start" ]&&return 0; sleep .05; done; return 1; }
[ "$#" -ge 1 ]||fail 'mode required'; mode="$1";shift
case "$mode" in
 start)
  [ "$#" = 4 ]||fail 'usage: start <state> <fifo> <log> <python-helper>';state="$1";fifo="$2";log="$3";helper="$4";dir="$(dirname "$state")";[ "$(stat -Lc '%a' "$dir")" = 700 ]||fail 'reservation directory mode'
  mkfifo -m 600 "$fifo";:>"$log";chmod 600 "$log";python3 "$helper" reserve "$state"<"$fifo">"$log" 2>&1&worker=$!;worker_start="$(proc_start "$worker")";[ -n "$worker_start" ]||fail 'cannot fingerprint worker immediately';printf 'EARLY_WORKER_PID=%s\nEARLY_WORKER_START_TIME=%s\n' "$worker" "$worker_start"
  exec 9>"$fifo";cleanup(){ local rc=$? failed=0;exec 9>&- 2>/dev/null||true;if [ "$(proc_start "$worker")" = "$worker_start" ];then [ "${JC_FL_E2E_TEST_RESERVATION_SIGNAL_FAIL:-0}" != 1 ]&&kill "$worker" 2>/dev/null||failed=1;[ "${JC_FL_E2E_TEST_RESERVATION_DELAY_EXIT:-0}" = 1 ]||wait_gone "$worker" "$worker_start"||failed=1;fi;[ "$failed" = 0 ]||return 125;return "$rc";};trap cleanup EXIT;trap 'exit 130' INT;trap 'exit 143' TERM
  [ "${JC_FL_E2E_TEST_RESERVATION_FAIL_EARLY:-0}" != 1 ]||fail 'injected early controller failure'
  for _ in $(seq 1 100);do [ -s "$state" ]&&grep -Fq 'READY 127.0.0.1:' "$log"&&break;[ "$(proc_start "$worker")" = "$worker_start" ]||fail 'worker exited';sleep .05;done
  [ -s "$state" ]&&grep -Fq 'READY 127.0.0.1:' "$log"||fail 'reservation startup timeout';verified="$(python3 "$helper" verify "$state")"||fail 'state verify';state_pid="$(echo "$verified"|sed -n 's/^PID=//p')";state_start="$(echo "$verified"|sed -n 's/^STARTTIME=//p')";endpoint="$(echo "$verified"|sed -n 's/^ENDPOINT=//p')";digest="$(echo "$verified"|sed -n 's/^DIGEST=//p')";[ "$state_pid|$state_start" = "$worker|$worker_start" ]||fail 'worker identity mismatch'
  printf 'CONTROLLER_PID=%s\nCONTROLLER_START_TIME=%s\nWORKER_PID=%s\nWORKER_START_TIME=%s\nENDPOINT=%s\nDIGEST=%s\n' "$$" "$(proc_start $$)" "$worker" "$worker_start" "$endpoint" "$digest";IFS= read -r command||fail 'controller channel closed';[ "$command" = RELEASE ]||fail 'invalid release';verified="$(python3 "$helper" verify "$state")"||fail 'state changed';token="$(echo "$verified"|sed -n 's/^TOKEN=//p')";printf 'RELEASE %s\n' "$token">&9;exec 9>&-;wait "$worker";trap - EXIT INT TERM
  ;;
 test-cleanup-identity)
  [ "${JC_FL_E2E_TEST_CLEANUP_IDENTITY:-0}" = 1 ]||fail 'test mode unavailable';[ "$#" = 2 ]||fail 'usage: test-cleanup-identity <pid> <start>';pid="$1";expected="$2";if [ "$(proc_start "$pid")" = "$expected" ];then kill "$pid";echo SIGNALED;else echo IDENTITY_CHANGED;fi
  ;;
 *) fail 'unsupported mode';;
esac
