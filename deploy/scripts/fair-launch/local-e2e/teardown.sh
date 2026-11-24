#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"; . "$HERE/common.sh"; jc_fl_e2e_self_bound teardown 120s "$@"; jc_fl_e2e_test_phase_hold teardown
[ "$#" = 1 ] || jc_fl_e2e_fail 'usage: teardown.sh <run-id>'; jc_fl_e2e_reject_external; jc_fl_e2e_load_state "$1"
jc_fl_e2e_phase_requires "$RUN_DIR" REDIS_ATTEMPTED=true DOCKER_ATTEMPTED=true STAGE=transport-published||jc_fl_e2e_fail 'run lifecycle phase not teardown-authorized'
write_failure(){ local status="$1" detail="$2"; printf '{"runId":"%s","status":"%s","detail":"%s","observedAt":"%s"}\n' "$RUN_ID" "$status" "$detail" "$(date -u +%FT%TZ)" > "$RUN_DIR/evidence/teardown-failure.json"; chmod 600 "$RUN_DIR/evidence/teardown-failure.json"; }
if ! jc_fl_e2e_mysql_transport_owned; then write_failure TEARDOWN_BLOCKED_MYSQL_TRANSPORT_OWNERSHIP 'container/host-network/listener/server/fingerprint mismatch'; jc_fl_e2e_fail 'refusing teardown: MySQL transport ownership mismatch'; fi
redis_identity="$(jc_fl_e2e_redis_identity "$REDIS_FINGERPRINT")"||{ write_failure TEARDOWN_BLOCKED_REDIS_OWNERSHIP 'strict process record mismatch'; jc_fl_e2e_fail 'refusing teardown: Redis process ownership mismatch'; };REDIS_PID="$(printf '%s\n' "$redis_identity"|sed -n 's/^PID=//p')";REDIS_START_TIME="$(printf '%s\n' "$redis_identity"|sed -n 's/^STARTTIME=//p')"
jc_fl_e2e_redis_owned "$REDIS_FINGERPRINT" && jc_fl_e2e_redis_owned "$REDIS_FINGERPRINT" || { write_failure TEARDOWN_BLOCKED_REDIS_OWNERSHIP 'strict record changed before signal';jc_fl_e2e_fail 'refusing teardown: Redis fingerprint changed before signal'; }
python3 "$JC_FL_E2E_AUTHORITY_HELPER" root-verify "$JC_FL_E2E_STATE_ROOT">/dev/null&&jc_fl_e2e_phase_requires "$RUN_DIR" REDIS_ATTEMPTED=true DOCKER_ATTEMPTED=true STAGE=transport-published&&jc_fl_e2e_redis_owned "$REDIS_FINGERPRINT"||jc_fl_e2e_fail 'Redis signal authority changed'
kill "$REDIS_PID" || jc_fl_e2e_fail 'Redis signal failed'
for _ in $(seq 1 100); do [ ! -e "/proc/$REDIS_PID" ] && break; [ "$(jc_fl_e2e_proc_start "$REDIS_PID")" != "$REDIS_START_TIME" ] && break; sleep .1; done
if [ -e "/proc/$REDIS_PID" ] && [ "$(jc_fl_e2e_proc_start "$REDIS_PID")" = "$REDIS_START_TIME" ]; then write_failure TEARDOWN_BLOCKED_REDIS_RESIDUE "pid=$REDIS_PID"; jc_fl_e2e_fail 'owned Redis did not exit'; fi
if ! jc_fl_e2e_redis_absent "$REDIS_FINGERPRINT"; then write_failure TEARDOWN_BLOCKED_REDIS_PORT_RESIDUE "endpoint=127.0.0.1:$REDIS_PORT"; jc_fl_e2e_fail 'Redis endpoint remains open'; fi
# The labelled container is the only MySQL process authority. Re-validate its
# host-network/start/listener/server identity immediately before removal.
jc_fl_e2e_mysql_transport_owned || { write_failure TEARDOWN_BLOCKED_MYSQL_TRANSPORT_CHANGED 'identity changed before container removal'; jc_fl_e2e_fail 'refusing teardown: MySQL transport changed before removal'; }
if ! jc_fl_e2e_docker_remove_exact "$MYSQL_CONTAINER_ID" "$RUN_DIR/evidence/teardown-container-removal.json";then write_failure TEARDOWN_BLOCKED_MYSQL_REMOVAL 'exact-ID removal or absence proof failed';jc_fl_e2e_fail 'MySQL container removal unproven';fi
for _ in $(seq 1 100); do ! ss -H -ltn "sport = :$MYSQL_PORT" 2>/dev/null | grep -q . && break; sleep .1; done
if ss -H -ltn "sport = :$MYSQL_PORT" 2>/dev/null | grep -q .; then write_failure TEARDOWN_BLOCKED_MYSQL_PORT_RESIDUE "endpoint=$MYSQL_ENDPOINT"; jc_fl_e2e_fail 'MySQL endpoint remains open'; fi
set +e; residue_containers="$(jc_fl_e2e_docker ps -aq --no-trunc --filter "label=$JC_FL_E2E_RUN_LABEL=$RUN_ID")"; containers_rc=$?; residue_networks="$(jc_fl_e2e_docker network ls -q --filter "label=$JC_FL_E2E_RUN_LABEL=$RUN_ID")"; networks_rc=$?; set -e
if [ "$containers_rc" != 0 ] || [ "$networks_rc" != 0 ]; then write_failure TEARDOWN_BLOCKED_DOCKER_RESIDUE_QUERY "containers_rc=$containers_rc networks_rc=$networks_rc"; jc_fl_e2e_fail 'Docker residue query failed'; fi
[ -z "$residue_containers" ] || { write_failure TEARDOWN_BLOCKED_CONTAINER_RESIDUE "$residue_containers"; jc_fl_e2e_fail 'owned containers remain'; }
[ -z "$residue_networks" ] || { write_failure TEARDOWN_BLOCKED_NETWORK_RESIDUE "$residue_networks"; jc_fl_e2e_fail 'unexpected owned networks remain'; }
python3 "$JC_FL_E2E_AUTHORITY_HELPER" root-verify "$JC_FL_E2E_STATE_ROOT">/dev/null&&jc_fl_e2e_phase_requires "$RUN_DIR" REDIS_ATTEMPTED=true DOCKER_ATTEMPTED=true STAGE=transport-published&&[ "$(stat -Lc '%u:%a' "$RUN_DIR")" = "$(id -u):700" ]||jc_fl_e2e_fail 'run deletion authority changed'
rm -rf "$RUN_DIR"
echo "Fair Launch local E2E teardown complete: run_id=$RUN_ID"
