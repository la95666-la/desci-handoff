#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "$0")"&&pwd)"; . "$HERE/common.sh"; jc_fl_e2e_self_bound start 240s "$@"; jc_fl_e2e_test_phase_hold start; jc_fl_e2e_reject_external
for c in docker redis-server redis-cli openssl uuidgen sha256sum ss awk python3; do command -v "$c">/dev/null||jc_fl_e2e_fail "$c required"; done
[ -S "$JC_FL_E2E_DOCKER_SOCKET" ]&&[ "$(readlink -f "$JC_FL_E2E_DOCKER_SOCKET")" = /run/docker.sock ]||jc_fl_e2e_fail 'local WSL Docker socket required'
MYSQL_IMAGE_REF="$JC_FL_E2E_MYSQL_IMAGE"; MYSQL_IMAGE_ID="$(jc_fl_e2e_image_id)"; [ -n "$MYSQL_IMAGE_ID" ]||jc_fl_e2e_fail 'pinned MySQL image not available locally';jc_fl_e2e_state_root_prepare||jc_fl_e2e_fail 'state root authority invalid'
RUN_ID="$(uuidgen|tr '[:upper:]' '[:lower:]')"; RUN_HEX="${RUN_ID//-/}"; DIR="$(jc_fl_e2e_run_dir "$RUN_ID")"; RUN_DIR="$DIR"; MYSQL_NAME="jc-fl-e2e-mysql-$RUN_HEX"; DB_NAME="jc_fl_e2e_$RUN_HEX"
MYSQL_NETWORK_MODE=host; MYSQL_NETWORK_ID=network-not-used; MYSQL_NETWORK_NAME=network-not-used; CONTAINER_ID=''; MYSQL_CONTAINER_ID=''; MYSQL_CONTAINER_PID=''; MYSQL_CONTAINER_STARTED_AT=''; MYSQL_PORT=''; MYSQL_ENDPOINT=''; MYSQL_SERVER_UUID=''; MYSQL_VERSION=''; MYSQL_TRANSPORT_DIGEST=''; MYSQL_RESERVATION_DIGEST=''; RESERVATION_CONTROLLER_PID=''; RESERVATION_CONTROLLER_START=''; RESERVATION_WORKER_PID=''; RESERVATION_WORKER_START=''; RESERVATION_RELEASED=0; DOCKER_LAUNCH_ATTEMPTED=0; DOCKER_RUN_RC='not-run'; DOCKER_RUN_RAW=''; DOCKER_CREATED_ID=''; DOCKER_EXACT_ID_OUTCOME='not-queried'; DOCKER_NAME_DISCOVERY_OUTCOME='not-queried'; REDIS_PORT=''; REDIS_PID=''; START_COMPLETE=0
REDIS_CONFIG="$DIR/redis/redis.conf"; REDIS_FINGERPRINT="$DIR/redis/process.json"; RESERVATION_DIR="$DIR/mysql-transport"; RESERVATION_STATE="$RESERVATION_DIR/reservation.json"; RESERVATION_FIFO="$RESERVATION_DIR/control.fifo"; RESERVATION_LOG="$RESERVATION_DIR/reservation.log"; RESERVATION_CONTROL="$RESERVATION_DIR/controller.fifo"; RESERVATION_CONTROLLER_LOG="$RESERVATION_DIR/controller.log"; FAILURE_REASONS=()
record_failure(){ local detail="$1"; mkdir -p "$DIR/evidence"; chmod 700 "$DIR" "$DIR/evidence"; python3 - "$DIR/evidence/start-failure.json" "$RUN_ID" "$detail" "$DOCKER_RUN_RC" "$DOCKER_CREATED_ID" "$DOCKER_EXACT_ID_OUTCOME" "$DOCKER_NAME_DISCOVERY_OUTCOME" <<'PY'
import json,os,sys,tempfile,datetime
p,run,detail,rc,cid,exact,name=sys.argv[1:];o={'runId':run,'status':'START_CLEANUP_BLOCKED','detail':detail,'dockerRunRc':rc,'dockerCreatedId':cid or None,'exactIdOutcome':exact,'nameDiscoveryOutcome':name,'observedAt':datetime.datetime.now(datetime.timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')};fd,tmp=tempfile.mkstemp(dir=os.path.dirname(p));os.fchmod(fd,0o600);os.write(fd,(json.dumps(o,separators=(',',':'))+'\n').encode());os.fsync(fd);os.close(fd);os.replace(tmp,p)
PY
 chmod 600 "$DIR/evidence/start-failure.json"; }
wait_identity_exit(){ local pid="$1" start="$2"; [ -n "$pid" ]&&[ -n "$start" ]||return 0; for _ in $(seq 1 100); do [ ! -r "/proc/$pid/stat" ]&&return 0; [ "$(jc_fl_e2e_proc_start "$pid")" != "$start" ]&&return 0; sleep .05; done; return 1; }
cleanup_failed_start(){ local original=$? candidate='' phase_state='' phase_redis=''; [ "$START_COMPLETE" = 0 ]||return "$original"; set +e
 phase_state="$(jc_fl_e2e_phase_verify "$DIR" 2>/dev/null)"||FAILURE_REASONS+=(lifecycle-phase-authority-ambiguous);phase_redis="$(printf '%s\n' "$phase_state"|sed -n 's/^REDIS_ATTEMPTED=//p')";[ "$phase_redis" = true ]||[ "$phase_redis" = false ]||FAILURE_REASONS+=(lifecycle-phase-state-ambiguous)
 exec 8>&- 2>/dev/null
 if [ -n "$RESERVATION_CONTROLLER_PID" ]&&[ "$(jc_fl_e2e_proc_start "$RESERVATION_CONTROLLER_PID")" = "$RESERVATION_CONTROLLER_START" ]; then if [ "${JC_FL_E2E_TEST_PARENT_RESERVATION_SIGNAL_FAIL:-0}" = 1 ]; then FAILURE_REASONS+=(reservation-controller-signal-failed); else kill "$RESERVATION_CONTROLLER_PID" 2>/dev/null||FAILURE_REASONS+=(reservation-controller-signal-failed); fi; wait_identity_exit "$RESERVATION_CONTROLLER_PID" "$RESERVATION_CONTROLLER_START"||FAILURE_REASONS+=(reservation-controller-residue); fi
 if [ -n "$RESERVATION_WORKER_PID" ]&&[ "$(jc_fl_e2e_proc_start "$RESERVATION_WORKER_PID")" = "$RESERVATION_WORKER_START" ]; then if [ "${JC_FL_E2E_TEST_PARENT_RESERVATION_PID_MISMATCH:-0}" = 1 ]; then RESERVATION_WORKER_START=0; FAILURE_REASONS+=(reservation-worker-identity-changed); elif [ "${JC_FL_E2E_TEST_PARENT_RESERVATION_SIGNAL_FAIL:-0}" = 1 ]; then FAILURE_REASONS+=(reservation-worker-signal-failed); else kill "$RESERVATION_WORKER_PID" 2>/dev/null||FAILURE_REASONS+=(reservation-worker-signal-failed); fi; wait_identity_exit "$RESERVATION_WORKER_PID" "$RESERVATION_WORKER_START"||FAILURE_REASONS+=(reservation-worker-residue); fi
 candidate="$DOCKER_CREATED_ID"; [ -n "$candidate" ]||candidate="$CONTAINER_ID"; if [ -n "$candidate" ]; then set +e; exact_out="$(jc_fl_e2e_docker inspect --format '{{.Id}}' "$candidate" 2>/dev/null)"; exact_rc=$?; set -e; if [ "$exact_rc" != 0 ];then DOCKER_EXACT_ID_OUTCOME="inspect-failed-rc-$exact_rc";FAILURE_REASONS+=(mysql-exact-id-query-failed);candidate='';elif [ "$exact_out" != "$candidate" ];then DOCKER_EXACT_ID_OUTCOME=inspect-mismatch;FAILURE_REASONS+=(mysql-exact-id-ambiguous);candidate='';else DOCKER_EXACT_ID_OUTCOME=exact-match;fi; elif [ "$DOCKER_LAUNCH_ATTEMPTED" = 1 ]; then candidate="$(jc_fl_e2e_docker ps -aq --no-trunc --filter "name=^/${MYSQL_NAME}$" 2>/dev/null)"; query_rc=$?; if [ "$query_rc" != 0 ]; then DOCKER_NAME_DISCOVERY_OUTCOME="query-failed-rc-$query_rc"; FAILURE_REASONS+=(mysql-discovery-query-failed); candidate=''; elif [ -z "$candidate" ]; then DOCKER_NAME_DISCOVERY_OUTCOME=empty;FAILURE_REASONS+=(mysql-discovery-empty)
 elif [ "$(printf '%s\n' "$candidate"|sed '/^$/d'|wc -l)" != 1 ]; then DOCKER_NAME_DISCOVERY_OUTCOME=ambiguous;FAILURE_REASONS+=(mysql-discovery-ambiguous); candidate='';else DOCKER_NAME_DISCOVERY_OUTCOME=one; fi; fi
 if [ -n "$candidate" ]; then
  if jc_fl_e2e_mysql_container_static_owned "$candidate" "$MYSQL_IMAGE_REF" "$MYSQL_IMAGE_ID" "$MYSQL_PORT" "$RUN_ID" "$MYSQL_NAME" "$RUN_DIR"; then
   if [ "${JC_FL_E2E_TEST_DOCKER_RM_FAIL:-0}" = 1 ]||! jc_fl_e2e_docker_remove_exact "$candidate" "$DIR/evidence/start-container-removal.json"; then FAILURE_REASONS+=(mysql-remove-failed); else CONTAINER_ID=''; fi
  else FAILURE_REASONS+=(mysql-ownership-ambiguous); fi
 fi
 if [ -n "$MYSQL_PORT" ]&&ss -H -ltn "sport = :$MYSQL_PORT"|grep -q .; then FAILURE_REASONS+=(mysql-listener-residue); fi
 if [ -f "$REDIS_FINGERPRINT" ]; then
  redis_identity="$(jc_fl_e2e_redis_identity "$REDIS_FINGERPRINT")"||redis_identity=''; if [ -n "$redis_identity" ]&&python3 "$JC_FL_E2E_AUTHORITY_HELPER" root-verify "$JC_FL_E2E_STATE_ROOT">/dev/null 2>&1&&jc_fl_e2e_phase_requires "$DIR" REDIS_ATTEMPTED=true&&jc_fl_e2e_redis_owned "$REDIS_FINGERPRINT"&&jc_fl_e2e_redis_owned "$REDIS_FINGERPRINT"; then REDIS_PID="$(printf '%s\n' "$redis_identity"|sed -n 's/^PID=//p')";REDIS_START_TIME="$(printf '%s\n' "$redis_identity"|sed -n 's/^STARTTIME=//p')";kill "$REDIS_PID" 2>/dev/null||FAILURE_REASONS+=(redis-signal-failed);wait_identity_exit "$REDIS_PID" "$REDIS_START_TIME"||FAILURE_REASONS+=(redis-process-residue);jc_fl_e2e_redis_absent "$REDIS_FINGERPRINT"||FAILURE_REASONS+=(redis-listener-residue)
  else FAILURE_REASONS+=(redis-ownership-ambiguous); fi
 elif [ "$phase_redis" = true ];then FAILURE_REASONS+=(redis-authority-missing-after-launch-attempt)
 elif [ "$phase_redis" != false ];then FAILURE_REASONS+=(redis-launch-authority-ambiguous)
 fi
 if [ -n "$REDIS_PORT" ]&&ss -H -ltn "sport = :$REDIS_PORT"|grep -Fq "127.0.0.1:$REDIS_PORT"; then FAILURE_REASONS+=(redis-listener-residue); fi
 if [ ${#FAILURE_REASONS[@]} -eq 0 ]; then python3 "$JC_FL_E2E_AUTHORITY_HELPER" root-verify "$JC_FL_E2E_STATE_ROOT">/dev/null 2>&1&&jc_fl_e2e_phase_verify "$DIR">/dev/null 2>&1&&[ "$(stat -Lc '%u:%a' "$DIR" 2>/dev/null)" = "$(id -u):700" ]&&rm -rf "$DIR"||FAILURE_REASONS+=(run-dir-remove-failed); fi
 if [ ${#FAILURE_REASONS[@]} -ne 0 ]; then record_failure "$(IFS=,;echo "${FAILURE_REASONS[*]}")"; fi
 set -e; return "$original"
}
trap cleanup_failed_start EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir -p "$DIR/secrets" "$DIR/redis" "$DIR/evidence" "$RESERVATION_DIR"; chmod 700 "$DIR" "$DIR/secrets" "$DIR/redis" "$DIR/evidence" "$RESERVATION_DIR";jc_fl_e2e_phase_write "$DIR"
[ "${JC_FL_E2E_TEST_FAIL_AT:-}" != after_run_dir ]||jc_fl_e2e_fail 'injected failure after run directory'
REDIS_PORT="$(jc_fl_e2e_choose_port 16379)"; RPC_PORT="$(jc_fl_e2e_choose_port 18545)"; CORE_PORT="$(jc_fl_e2e_choose_port 13040)"; ADMIN_PORT="$(jc_fl_e2e_choose_port 15660)"; FRONTEND_PORT="$(jc_fl_e2e_choose_port 15170)"
umask 077; openssl rand -hex 32 >"$DIR/secrets/mysql-root"; openssl rand -hex 32 >"$DIR/secrets/redis-password"; chmod 600 "$DIR/secrets/"*
bash "$HERE/redis-secret-helper.sh" config "$DIR/secrets/redis-password" "$REDIS_CONFIG" "$REDIS_PORT" "$DIR/redis/redis.pid" "$DIR/redis";jc_fl_e2e_phase_update "$DIR" redis-launch-attempted true false
[ "${JC_FL_E2E_TEST_FAIL_AT:-}" != after_redis_launch_attempted ]||jc_fl_e2e_fail 'injected failure after Redis launch-attempt publication'
redis-server "$REDIS_CONFIG"
[ "${JC_FL_E2E_TEST_FAIL_AT:-}" != after_redis_daemonized ]||jc_fl_e2e_fail 'injected failure after Redis daemonization'
for _ in $(seq 1 50); do [ -s "$DIR/redis/redis.pid" ]&&break;sleep .1;done; REDIS_PID="$(cat "$DIR/redis/redis.pid")"
[ "${JC_FL_E2E_TEST_FAIL_AT:-}" != after_redis_pid_read ]||jc_fl_e2e_fail 'injected failure after Redis PID read'
jc_fl_e2e_redis_fingerprint_write "$REDIS_PID" "$REDIS_FINGERPRINT"
[ "${JC_FL_E2E_TEST_FAIL_AT:-}" != after_redis_authority_invalid ]||{ chmod 640 "$REDIS_FINGERPRINT";jc_fl_e2e_fail 'injected invalid Redis authority'; }
[ "${JC_FL_E2E_TEST_FAIL_AT:-}" != after_redis_authority_written ]||jc_fl_e2e_fail 'injected failure after Redis authority publication'
jc_fl_e2e_phase_update "$DIR" redis-authority-published true false; jc_fl_e2e_redis_owned "$REDIS_FINGERPRINT"||jc_fl_e2e_fail 'Redis ownership fingerprint failed'
[ "${JC_FL_E2E_TEST_FAIL_AT:-}" != after_redis ]||jc_fl_e2e_fail 'injected failure after Redis'; for _ in $(seq 1 50); do bash "$HERE/redis-secret-helper.sh" ping "$DIR/secrets/redis-password" 127.0.0.1 "$REDIS_PORT"&&break;sleep .1;done; bash "$HERE/redis-secret-helper.sh" ping "$DIR/secrets/redis-password" 127.0.0.1 "$REDIS_PORT"||jc_fl_e2e_fail 'Redis startup failed'
mkfifo -m 600 "$RESERVATION_CONTROL"; bash "$HERE/mysql-port-reservation.sh" start "$RESERVATION_STATE" "$RESERVATION_FIFO" "$RESERVATION_LOG" "$HERE/mysql-port-reservation.py"<"$RESERVATION_CONTROL">"$RESERVATION_CONTROLLER_LOG" 2>&1 & RESERVATION_CONTROLLER_PID=$!; RESERVATION_CONTROLLER_START="$(jc_fl_e2e_proc_start "$RESERVATION_CONTROLLER_PID")"; exec 8>"$RESERVATION_CONTROL"
for _ in $(seq 1 100); do grep -q '^WORKER_PID=' "$RESERVATION_CONTROLLER_LOG" 2>/dev/null&&break; if grep -Fq 'EARLY_WORKER_PID=' "$RESERVATION_CONTROLLER_LOG" 2>/dev/null; then RESERVATION_WORKER_PID="$(sed -n 's/^EARLY_WORKER_PID=//p' "$RESERVATION_CONTROLLER_LOG")"; RESERVATION_WORKER_START="$(sed -n 's/^EARLY_WORKER_START_TIME=//p' "$RESERVATION_CONTROLLER_LOG")"; fi; kill -0 "$RESERVATION_CONTROLLER_PID" 2>/dev/null||{ FAILURE_REASONS+=(reservation-controller-early-failure); jc_fl_e2e_fail 'reservation controller exited'; };sleep .05;done
grep -q '^WORKER_PID=' "$RESERVATION_CONTROLLER_LOG"||jc_fl_e2e_fail 'reservation timeout'; RESERVATION_WORKER_PID="$(sed -n 's/^WORKER_PID=//p' "$RESERVATION_CONTROLLER_LOG")"; RESERVATION_WORKER_START="$(sed -n 's/^WORKER_START_TIME=//p' "$RESERVATION_CONTROLLER_LOG")"; MYSQL_ENDPOINT="$(sed -n 's/^ENDPOINT=//p' "$RESERVATION_CONTROLLER_LOG")"; MYSQL_RESERVATION_DIGEST="$(sed -n 's/^DIGEST=//p' "$RESERVATION_CONTROLLER_LOG")"; MYSQL_PORT="${MYSQL_ENDPOINT#127.0.0.1:}"
verified="$(python3 "$HERE/mysql-port-reservation.py" verify "$RESERVATION_STATE")"; [ "$(printf '%s\n' "$verified"|sed -n 's/^PID=//p')" = "$RESERVATION_WORKER_PID" ]&&[ "$(printf '%s\n' "$verified"|sed -n 's/^STARTTIME=//p')" = "$RESERVATION_WORKER_START" ]&&[ "$(printf '%s\n' "$verified"|sed -n 's/^ENDPOINT=//p')" = "$MYSQL_ENDPOINT" ]&&[ "$(printf '%s\n' "$verified"|sed -n 's/^DIGEST=//p')" = "$MYSQL_RESERVATION_DIGEST" ]||jc_fl_e2e_fail 'independent reservation verification mismatch'; jc_fl_e2e_mysql_listener_exact||jc_fl_e2e_fail 'reserved endpoint mismatch'
[ "${JC_FL_E2E_TEST_FAIL_AT:-}" != after_port_reservation ]||jc_fl_e2e_fail 'injected failure after reservation'
printf 'RELEASE\n'>&8
DOCKER_LAUNCH_ATTEMPTED=1;jc_fl_e2e_phase_update "$DIR" docker-launch-attempted true true; set +e; DOCKER_RUN_RAW="$(jc_fl_e2e_docker run -d --name "$MYSQL_NAME" --label "$JC_FL_E2E_RUN_LABEL=$RUN_ID" --label "$JC_FL_E2E_COMPONENT_LABEL=mysql" --network host --tmpfs /var/lib/mysql:rw,nosuid,nodev,size=2g --tmpfs /tmp:rw,nosuid,nodev,size=256m -v "$DIR/secrets/mysql-root:/run/secrets/root:ro" -v "$JC_FL_E2E_ROOT:/repo:ro" -e MYSQL_ROOT_PASSWORD_FILE=/run/secrets/root "$MYSQL_IMAGE_REF" --skip-log-bin --local-infile=OFF --secure-file-priv=NULL --skip-name-resolve --bind-address=127.0.0.1 --port="$MYSQL_PORT" --mysqlx=0)"; DOCKER_RUN_RC=$?; docker_run_rc="$DOCKER_RUN_RC"; set -e
if [ "$DOCKER_RUN_RC" = 0 ]&&[[ "$DOCKER_RUN_RAW" =~ ^[0-9a-f]{64}$ ]];then jc_fl_e2e_container_source_publish "$DOCKER_RUN_RAW" mysql "$DIR/evidence/docker-created.source.json"||jc_fl_e2e_fail 'cannot publish Docker-created container authority';fi
case "${JC_FL_E2E_TEST_DOCKER_RUN_OUTPUT:-}" in '') ;; short|multiple|malformed) if [ "$DOCKER_RUN_RC" = 0 ]&&[[ "$DOCKER_RUN_RAW" =~ ^[0-9a-f]{64}$ ]];then case "$JC_FL_E2E_TEST_DOCKER_RUN_OUTPUT" in short)DOCKER_RUN_RAW="${DOCKER_RUN_RAW:0:12}";;multiple)DOCKER_RUN_RAW="$DOCKER_RUN_RAW"$'\n'"$DOCKER_RUN_RAW";;malformed)DOCKER_RUN_RAW='not-a-container-id';;esac;fi;;*)jc_fl_e2e_fail 'invalid Docker run output injection';;esac
if [ "$DOCKER_RUN_RC" = 0 ];then if [[ "$DOCKER_RUN_RAW" =~ ^[0-9a-f]{64}$ ]];then DOCKER_CREATED_ID="$DOCKER_RUN_RAW";else FAILURE_REASONS+=(docker-run-output-malformed);DOCKER_EXACT_ID_OUTCOME=returned-id-malformed;DOCKER_NAME_DISCOVERY_OUTCOME=not-attempted-malformed-authority;record_failure "$(IFS=,;echo "${FAILURE_REASONS[*]}")";START_COMPLETE=2;trap - EXIT INT TERM;jc_fl_e2e_fail 'Docker returned malformed full container ID';fi;fi
JC_FL_E2E_DOCKER_RUN_OUTPUT="$DOCKER_RUN_RAW"
RESERVATION_RELEASED=1; exec 8>&-; wait "$RESERVATION_CONTROLLER_PID"||jc_fl_e2e_fail 'reservation release failed'; wait_identity_exit "$RESERVATION_CONTROLLER_PID" "$RESERVATION_CONTROLLER_START"||jc_fl_e2e_fail 'reservation controller residue'; wait_identity_exit "$RESERVATION_WORKER_PID" "$RESERVATION_WORKER_START"||jc_fl_e2e_fail 'reservation worker residue'; python3 "$HERE/mysql-port-reservation.py" release-evidence "$RESERVATION_STATE" "$RESERVATION_DIR/reservation-released.json"; rm -f "$RESERVATION_STATE" "$RESERVATION_FIFO" "$RESERVATION_CONTROL"
if [ "${JC_FL_E2E_TEST_REPLACE_CONTAINER:-}" != '' ]; then
 original="$DOCKER_CREATED_ID"
 jc_fl_e2e_mysql_container_static_owned "$original" "$MYSQL_IMAGE_REF" "$MYSQL_IMAGE_ID" "$MYSQL_PORT" "$RUN_ID" "$MYSQL_NAME" "$RUN_DIR"||jc_fl_e2e_fail 'fixture source container identity changed'
 jc_fl_e2e_docker_remove_exact "$original" "$DIR/evidence/replacement-source-removal.json"||jc_fl_e2e_fail 'fixture source container removal unproven'
 DOCKER_CREATED_ID='';CONTAINER_ID='';extra=();launch_ref="$MYSQL_IMAGE_REF";cmd=(--skip-log-bin --local-infile=OFF --secure-file-priv=NULL --skip-name-resolve --bind-address=127.0.0.1 --port="$MYSQL_PORT" --mysqlx=0);component=mysql
 case "$JC_FL_E2E_TEST_REPLACE_CONTAINER" in privileged) extra+=(--privileged);; extra-label) extra+=(--label foreign=1);; command) cmd+=(--bind-address=0.0.0.0);; mount) extra+=(-v /tmp:/foreign:ro);; wrong-component) extra+=(--label "$JC_FL_E2E_COMPONENT_LABEL=foreign");component=foreign;; wrong-image) MYSQL_IMAGE_ID="sha256:$(printf '0%.0s' {1..64})";; image-ref) launch_ref="$MYSQL_IMAGE_ID";; cap) extra+=(--cap-add SYS_ADMIN);; device) extra+=(--device /dev/null:/dev/jc-fl-null:r);; security) extra+=(--security-opt seccomp=unconfined);; pid) extra+=(--pid host);; ipc) extra+=(--ipc host);; restart) extra+=(--restart always);; group-add) extra+=(--group-add 12345);; device-rule) extra+=(--device-cgroup-rule 'c 1:3 r');; sysctl) extra+=(--sysctl net.ipv4.ip_local_port_range='1024 65535');; ulimit) extra+=(--ulimit nofile=1024:1024);; entrypoint) extra+=(--entrypoint /bin/sh);cmd=(-c 'sleep 60');; user) extra+=(--user 65534);; workdir) extra+=(--workdir /tmp);; stop-signal) extra+=(--stop-signal SIGKILL);; healthcheck) extra+=(--health-cmd 'exit 0');; *) jc_fl_e2e_fail 'invalid replacement fixture';; esac
 set +e;replacement_raw="$(jc_fl_e2e_docker run -d --name "$MYSQL_NAME" --label "$JC_FL_E2E_RUN_LABEL=$RUN_ID" --label "$JC_FL_E2E_COMPONENT_LABEL=mysql" --network host --tmpfs /var/lib/mysql:rw,nosuid,nodev,size=2g --tmpfs /tmp:rw,nosuid,nodev,size=256m -v "$DIR/secrets/mysql-root:/run/secrets/root:ro" -v "$JC_FL_E2E_ROOT:/repo:ro" -e MYSQL_ROOT_PASSWORD_FILE=/run/secrets/root "${extra[@]}" "$launch_ref" "${cmd[@]}")";replacement_rc=$?;set -e
 if [ "$replacement_rc" != 0 ]||! [[ "$replacement_raw" =~ ^[0-9a-f]{64}$ ]];then
  set +e;replacement_discovered="$(jc_fl_e2e_docker ps -aq --no-trunc --filter "name=^/${MYSQL_NAME}$")";replacement_query_rc=$?;set -e
  [ "$replacement_query_rc" = 0 ]&&[ "$(printf '%s\n' "$replacement_discovered"|sed '/^$/d'|wc -l)" = 1 ]&&[[ "$replacement_discovered" =~ ^[0-9a-f]{64}$ ]]||jc_fl_e2e_fail 'replacement create authority unavailable'
  replacement_raw="$replacement_discovered"
 fi
 jc_fl_e2e_container_source_publish "$replacement_raw" "$component" "$DIR/evidence/replacement-created.source.json"||jc_fl_e2e_fail 'replacement source authority publication failed'
 DOCKER_CREATED_ID="$replacement_raw";CONTAINER_ID="$replacement_raw"
 [ "$replacement_rc" = 0 ]||jc_fl_e2e_fail 'replacement create failed after authority publication'
 jc_fl_e2e_fail 'injected mismatched no-ID replacement'
fi
if [ "${JC_FL_E2E_TEST_FOREIGN_LISTENER_AFTER_RELEASE:-0}" = 1 ]; then python3 - "$MYSQL_PORT" "$RESERVATION_DIR/foreign-listener.marker" "$RESERVATION_DIR/foreign-listener.pid" <<'PY' &
import os,socket,sys,time
port=int(sys.argv[1]);marker=sys.argv[2];pidfile=sys.argv[3];fd=os.open(marker,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600);os.write(fd,b'JC_FL_TEST_FOREIGN_LISTENER_V1\n');os.fsync(fd);os.close(fd);s=socket.socket();s.bind(('127.0.0.1',port));s.listen();fd=os.open(pidfile,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600);os.write(fd,str(os.getpid()).encode());os.fsync(fd);os.close(fd);time.sleep(60)
PY
sleep .2; printf '127.0.0.1:%s\n' "$MYSQL_PORT">"$RESERVATION_DIR/foreign-listener.endpoint";chmod 600 "$RESERVATION_DIR/foreign-listener.endpoint";jc_fl_e2e_fail 'injected foreign listener after release'; fi
[ "${JC_FL_E2E_TEST_FAIL_AT:-}" != docker_created_no_id ]||{ CONTAINER_ID='';jc_fl_e2e_fail 'injected failure after Docker create'; }
[ "$docker_run_rc" = 0 ]||jc_fl_e2e_fail 'MySQL container create failed'; CONTAINER_ID="$JC_FL_E2E_DOCKER_RUN_OUTPUT"; MYSQL_CONTAINER_ID="$CONTAINER_ID"; [ -n "$CONTAINER_ID" ]||jc_fl_e2e_fail 'container ID missing'; if [ "${JC_FL_E2E_TEST_FAIL_AT:-}" = after_mysql ]; then [ "${JC_FL_E2E_TEST_DOCKER_RM_FAIL:-0}" != 1 ] || FAILURE_REASONS+=(mysql-remove-failed); jc_fl_e2e_fail 'injected failure after MySQL'; fi
jc_fl_e2e_mysql_container_static_owned "$CONTAINER_ID" "$MYSQL_IMAGE_REF" "$MYSQL_IMAGE_ID" "$MYSQL_PORT" "$RUN_ID" "$MYSQL_NAME" "$RUN_DIR"||jc_fl_e2e_fail 'container static identity mismatch'
jc_fl_e2e_docker exec "$CONTAINER_ID" sh -c 'umask 077; { echo "[client]"; echo "user=root"; printf "password="; cat /run/secrets/root; echo "host=127.0.0.1"; echo "protocol=tcp"; echo "connect-timeout=5"; } >/tmp/root.cnf; chmod 600 /tmp/root.cnf'
for _ in $(seq 1 90);do jc_fl_e2e_docker exec "$CONTAINER_ID" mysqladmin --defaults-extra-file=/tmp/root.cnf --port="$MYSQL_PORT" ping --silent>/dev/null 2>&1&&break;sleep 1;done; jc_fl_e2e_docker exec "$CONTAINER_ID" mysqladmin --defaults-extra-file=/tmp/root.cnf --port="$MYSQL_PORT" ping --silent>/dev/null||jc_fl_e2e_fail 'MySQL startup failed'; jc_fl_e2e_mysql_listener_exact||jc_fl_e2e_fail 'MySQL listener unavailable'
MYSQL_SERVER_UUID="$(jc_fl_e2e_docker exec "$CONTAINER_ID" mysql --defaults-extra-file=/tmp/root.cnf --port="$MYSQL_PORT" -Nse 'SELECT LOWER(@@server_uuid)')"; MYSQL_VERSION="$(jc_fl_e2e_docker exec "$CONTAINER_ID" mysql --defaults-extra-file=/tmp/root.cnf --port="$MYSQL_PORT" -Nse 'SELECT VERSION()')"; [ "$(bash "$HERE/mysql-secret-helper.sh" verify-root-host "$DIR/secrets/mysql-root" "$MYSQL_PORT" "$MYSQL_SERVER_UUID" "$MYSQL_VERSION")" = verified ]||jc_fl_e2e_fail 'host identity mismatch'; MYSQL_CONTAINER_PID="$(jc_fl_e2e_docker inspect --format '{{.State.Pid}}' "$CONTAINER_ID")"; MYSQL_CONTAINER_STARTED_AT="$(jc_fl_e2e_docker inspect --format '{{.State.StartedAt}}' "$CONTAINER_ID")"; MYSQL_SCHEMA_READY=0; MYSQL_TRANSPORT_DIGEST="$(jc_fl_e2e_transport_digest)"
[ "${JC_FL_E2E_TEST_FAIL_AT:-}" != after_endpoint_discovery ]||jc_fl_e2e_fail 'injected failure after endpoint'
tmp="$DIR/.state.env.tmp"; cat >"$tmp" <<EOF
RUN_ID='$RUN_ID'
RUN_HEX='$RUN_HEX'
RUN_DIR='$DIR'
MYSQL_NAME='$MYSQL_NAME'
MYSQL_CONTAINER_ID='$CONTAINER_ID'
MYSQL_CONTAINER_PID='$MYSQL_CONTAINER_PID'
MYSQL_CONTAINER_STARTED_AT='$MYSQL_CONTAINER_STARTED_AT'
MYSQL_IMAGE_REF='$MYSQL_IMAGE_REF'
MYSQL_IMAGE_ID='$MYSQL_IMAGE_ID'
MYSQL_NETWORK_MODE='host'
MYSQL_NETWORK_NAME='network-not-used'
MYSQL_NETWORK_ID='network-not-used'
MYSQL_PORT='$MYSQL_PORT'
MYSQL_ENDPOINT='$MYSQL_ENDPOINT'
MYSQL_CONTAINER_PORT='$MYSQL_PORT'
MYSQL_PROTOCOL='tcp'
MYSQL_SERVER_UUID='$MYSQL_SERVER_UUID'
MYSQL_VERSION='$MYSQL_VERSION'
MYSQL_RESERVATION_DIGEST='$MYSQL_RESERVATION_DIGEST'
MYSQL_TRANSPORT_DIGEST='$MYSQL_TRANSPORT_DIGEST'
MYSQL_SCHEMA_READY='0'
DB_NAME='$DB_NAME'
REDIS_PORT='$REDIS_PORT'
RPC_PORT='$RPC_PORT'
CORE_PORT='$CORE_PORT'
ADMIN_PORT='$ADMIN_PORT'
FRONTEND_PORT='$FRONTEND_PORT'
EOF
chmod 600 "$tmp"; [ "${JC_FL_E2E_TEST_FAIL_AT:-}" != during_transport_state ]||jc_fl_e2e_fail 'injected state failure'; mv "$tmp" "$DIR/state.env";jc_fl_e2e_phase_update "$DIR" transport-published true true; jc_fl_e2e_mysql_transport_owned||jc_fl_e2e_fail 'transport ownership failed'
tmp="$DIR/evidence/.infrastructure.tmp"; printf '{"authority":"TEST_ONLY_NOT_PRODUCTION_ADMITTED","runId":"%s","status":"INFRASTRUCTURE_STARTED_NOT_ADMITTED","mysql":{"endpoint":"%s","containerId":"%s","containerPid":"%s","containerStartedAt":"%s","imageRef":"%s","imageId":"%s","networkMode":"host","publishedPortMappings":false,"database":"%s","serverUuid":"%s","version":"%s","reservationDigest":"%s","transportDigest":"%s"}}\n' "$RUN_ID" "$MYSQL_ENDPOINT" "$CONTAINER_ID" "$MYSQL_CONTAINER_PID" "$MYSQL_CONTAINER_STARTED_AT" "$MYSQL_IMAGE_REF" "$MYSQL_IMAGE_ID" "$DB_NAME" "$MYSQL_SERVER_UUID" "$MYSQL_VERSION" "$MYSQL_RESERVATION_DIGEST" "$MYSQL_TRANSPORT_DIGEST">"$tmp"; chmod 600 "$tmp"; [ "${JC_FL_E2E_TEST_FAIL_AT:-}" != during_transport_evidence ]||jc_fl_e2e_fail 'injected evidence failure'; mv "$tmp" "$DIR/evidence/infrastructure.json"; [ "${JC_FL_E2E_TEST_FAIL_AT:-}" != after_evidence ]||jc_fl_e2e_fail 'injected after evidence'; if [ "${JC_FL_E2E_TEST_HOLD_AT:-}" = after_infrastructure_evidence ]; then echo 'HOLD_AFTER_INFRASTRUCTURE_EVIDENCE' >&2; while :; do sleep 1; done; fi
START_COMPLETE=1; trap - EXIT INT TERM; echo "RUN_ID=$RUN_ID"; echo "STATE=$DIR/state.env"; echo "MYSQL_ENDPOINT=$MYSQL_ENDPOINT"; echo "RPC_URL=http://127.0.0.1:$RPC_PORT"; echo "REDIS_ENDPOINT=127.0.0.1:$REDIS_PORT"
