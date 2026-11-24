#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"; . "$HERE/common.sh"; jc_fl_e2e_self_bound status 60s "$@"; jc_fl_e2e_test_phase_hold status
[ "$#" = 1 ] || jc_fl_e2e_fail 'usage: status.sh <run-id>'; jc_fl_e2e_load_state "$1"
set +e;MYSQL_STATE="$(jc_fl_e2e_docker inspect --format '{{.State.Status}}' "$MYSQL_CONTAINER_ID" 2>/dev/null)";MYSQL_STATE_RC=$?;set -e
if [ "$MYSQL_STATE_RC" != 0 ]; then
 set +e;MYSQL_ENUM="$(jc_fl_e2e_docker ps -aq --no-trunc --filter "id=$MYSQL_CONTAINER_ID" 2>/dev/null)";MYSQL_ENUM_RC=$?;set -e
 if [ "$MYSQL_ENUM_RC" = 0 ]&&[ -z "$MYSQL_ENUM" ];then MYSQL_STATE=missing;else MYSQL_STATE=query_failed;fi
elif [ -z "$MYSQL_STATE" ];then MYSQL_STATE=query_failed
fi
MYSQL_TRANSPORT_STATE=ownership_mismatch; jc_fl_e2e_mysql_transport_owned && MYSQL_TRANSPORT_STATE=running
MYSQL_CREDENTIAL_STATE=not_ready
if [ "${MYSQL_SCHEMA_READY:-0}" = 1 ]; then MYSQL_CREDENTIAL_STATE=ownership_mismatch; jc_fl_e2e_mysql_transport_owned && jc_fl_e2e_mysql_profiles_ready && MYSQL_CREDENTIAL_STATE=ready; fi
REDIS_STATE=ownership_mismatch; jc_fl_e2e_redis_owned "$REDIS_FINGERPRINT" && REDIS_STATE=running
printf 'run_id=%s\nmysql_container=%s\nmysql_network=%s\nmysql_endpoint=%s\nmysql_state=%s\nmysql_transport_state=%s\nmysql_schema_ready=%s\nmysql_credential_state=%s\nredis_endpoint=127.0.0.1:%s\nredis_state=%s\nrpc_url=http://127.0.0.1:%s\n' "$RUN_ID" "$MYSQL_CONTAINER_ID" "$MYSQL_NETWORK_ID" "$MYSQL_ENDPOINT" "$MYSQL_STATE" "$MYSQL_TRANSPORT_STATE" "${MYSQL_SCHEMA_READY:-0}" "$MYSQL_CREDENTIAL_STATE" "$REDIS_PORT" "$REDIS_STATE" "$RPC_PORT"
status_rc=0
[ "$MYSQL_STATE" = running ] && [ "$MYSQL_TRANSPORT_STATE" = running ] && [ "$REDIS_STATE" = running ] || status_rc=1
if [ "${MYSQL_SCHEMA_READY:-0}" = 1 ]; then [ "$MYSQL_CREDENTIAL_STATE" = ready ] || status_rc=1; fi
exit "$status_rc"
