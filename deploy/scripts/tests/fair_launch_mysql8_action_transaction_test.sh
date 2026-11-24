#!/usr/bin/env bash
set -euo pipefail

# TEST ONLY. Exercises the Fair Launch three-person action state machine,
# transaction rollback, row locking, and append-only audit guards on a
# disposable MySQL 8 instance. This is not a production executor, migration,
# candidate collector, activation mechanism, or approval authority.

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
IMAGE='mysql@sha256:c831a0f11348d402b43d77453e17d770be2eef356615a2823fe0f5a0d6c8b9af'
DOCKER_SOCKET='/var/run/docker.sock'
RUN_ID="$(openssl rand -hex 16)"
NAME="jc-fl-mysql-actions-$RUN_ID"
OWNER_LABEL="joincare.fair-launch.action-transaction-test=$RUN_ID"
DB="jc_fl_action_tx_$RUN_ID"
SECRET_DIR=''
EVIDENCE="${JOINCARE_FL_MYSQL8_ACTION_TX_TEST_EVIDENCE:-/tmp/joincare-fair-launch-mysql8-action-transactions-$RUN_ID.txt}"
OUTPUT="/tmp/jc-fl-mysql8-action-tx-$RUN_ID.out"
LOCK_A_OUTPUT="/tmp/jc-fl-mysql8-action-lock-a-$RUN_ID.out"
LOCK_B_OUTPUT="/tmp/jc-fl-mysql8-action-lock-b-$RUN_ID.out"
CONTAINER_ID=''
LOCK_A_PID=''
LOCK_B_PID=''

MIGRATIONS=(
  '2026_09_03_fair_launch_project_model.sql'
  '2026_09_04_fair_launch_ce_v2_governance.sql'
  '2026_09_05_fair_launch_signing_release_control.sql'
  '2026_09_06_fair_launch_action_execution.sql'
)

fail() {
  echo "fair_launch_mysql8_action_transaction_test: FAIL: $*" >&2
  exit 1
}

docker_local() { docker -H "unix://$DOCKER_SOCKET" "$@"; }

cleanup() {
  [ -z "$LOCK_A_PID" ] || kill "$LOCK_A_PID" >/dev/null 2>&1 || true
  [ -z "$LOCK_B_PID" ] || kill "$LOCK_B_PID" >/dev/null 2>&1 || true
  if [ -n "$CONTAINER_ID" ]; then
    observed_label="$(docker_local inspect --format '{{index .Config.Labels "joincare.fair-launch.action-transaction-test"}}' "$CONTAINER_ID" 2>/dev/null || true)"
    if [ "$observed_label" = "$RUN_ID" ]; then
      docker_local rm -f "$CONTAINER_ID" >/dev/null 2>&1 || true
    fi
  fi
  [ -z "$SECRET_DIR" ] || rm -rf "$SECRET_DIR"
  rm -f "$OUTPUT" "$LOCK_A_OUTPUT" "$LOCK_B_OUTPUT"
}

for forbidden in JC_DEPLOY_RPC JC_DEPLOY_CHAIN_ID JC_DEPLOY_PRIVATE_KEY JC_MYSQL_ROOT_PASSWORD MYSQL_PWD FAIR_LAUNCH_DISPOSABLE_MYSQL_DSN DOCKER_HOST DOCKER_CONTEXT DOCKER_TLS_VERIFY DOCKER_CERT_PATH; do
  [ -z "${!forbidden:-}" ] || fail "refusing production/external database environment: $forbidden"
done
for key in $(env | cut -d= -f1); do
  case "${key^^}" in
    JC_FAIR_LAUNCH_TEST_*) fail "refusing test-authority environment input: $key" ;;
  esac
done

command -v docker >/dev/null || fail 'docker is required'
command -v openssl >/dev/null || fail 'openssl is required'
command -v sha256sum >/dev/null || fail 'sha256sum is required'
[ -S "$DOCKER_SOCKET" ] || fail "local Docker Unix socket is unavailable: $DOCKER_SOCKET"
[ "$(readlink -f "$DOCKER_SOCKET")" = '/run/docker.sock' ] || fail 'Docker socket does not resolve to the expected local WSL socket'
docker_local version --format '{{.Server.Version}}' >/dev/null 2>&1 || fail 'local Docker daemon is unavailable'
docker_local image inspect "$IMAGE" >/dev/null 2>&1 || fail "pinned local image is unavailable: $IMAGE"
[ ! -e "$EVIDENCE" ] || fail "evidence path already exists: $EVIDENCE"
trap cleanup EXIT INT TERM

umask 077
SECRET_DIR="$(mktemp -d)"
for identity in root local_admin; do
  openssl rand -hex 32 > "$SECRET_DIR/$identity"
  chmod 600 "$SECRET_DIR/$identity"
done

echo '[mysql8-action-tx] starting network-none, tmpfs-backed MySQL 8.4'
CONTAINER_ID="$(docker_local run -d \
  --name "$NAME" \
  --label "$OWNER_LABEL" \
  --network none \
  --tmpfs /var/lib/mysql:rw,nosuid,nodev,size=2g \
  --tmpfs /tmp:rw,nosuid,nodev,size=256m \
  -v "$SECRET_DIR:/run/secrets/jc-fl-action-tx:ro" \
  -v "$ROOT:/repo:ro" \
  -e MYSQL_ROOT_PASSWORD_FILE=/run/secrets/jc-fl-action-tx/root \
  "$IMAGE" \
  --skip-log-bin \
  --local-infile=OFF \
  --secure-file-priv=NULL)"
[ -n "$CONTAINER_ID" ] || fail 'Docker did not return an owned container ID'
[ "$(docker_local inspect --format '{{index .Config.Labels "joincare.fair-launch.action-transaction-test"}}' "$CONTAINER_ID")" = "$RUN_ID" ] || fail 'created container ownership label mismatch'

docker_local exec "$CONTAINER_ID" sh -c '
  set -eu
  umask 077
  for profile in root local_admin; do
    {
      printf "[client]\nuser=%s\npassword=" "$profile"
      cat "/run/secrets/jc-fl-action-tx/$profile"
      printf "host=127.0.0.1\nprotocol=tcp\n"
    } > "/tmp/jc-fl-$profile.cnf"
    chmod 600 "/tmp/jc-fl-$profile.cnf"
  done
' >/dev/null

for _ in $(seq 1 90); do
  if docker_local exec "$CONTAINER_ID" mysqladmin --defaults-extra-file=/tmp/jc-fl-root.cnf ping --silent >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker_local exec "$CONTAINER_ID" mysqladmin --defaults-extra-file=/tmp/jc-fl-root.cnf ping --silent >/dev/null || fail 'MySQL did not become ready'

mysql_root() {
  docker_local exec -i "$CONTAINER_ID" sh -c \
    'exec mysql --defaults-extra-file=/tmp/jc-fl-root.cnf --batch --raw --skip-column-names "$@"' sh "$@"
}

mysql_admin() {
  docker_local exec -i "$CONTAINER_ID" sh -c \
    'exec mysql --defaults-extra-file=/tmp/jc-fl-local_admin.cnf --unbuffered --batch --raw --skip-column-names "$@"' sh "$@"
}

mysql_admin_force() {
  docker_local exec -i "$CONTAINER_ID" sh -c \
    'exec mysql --defaults-extra-file=/tmp/jc-fl-local_admin.cnf --unbuffered --force --batch --raw --skip-column-names "$@"' sh "$@"
}

mysql_db() { mysql_root "$DB"; }
query_root() { printf '%s\n' "$1" | mysql_db | tr -d '\r'; }

assert_eq() {
  local label="$1" actual="$2" expected="$3"
  [ "$actual" = "$expected" ] || fail "$label: got=$actual want=$expected"
}

expect_error() {
  local label="$1" expected="$2" sql="$3"
  set +e
  printf '%s\n' "$sql" | mysql_admin "$DB" > "$OUTPUT" 2>&1
  local rc=$?
  set -e
  [ "$rc" -ne 0 ] || fail "$label unexpectedly succeeded"
  grep -Fq "$expected" "$OUTPUT" || { cat "$OUTPUT" >&2; fail "$label did not return the expected error"; }
}

echo '[mysql8-action-tx] verifying pinned SQL identities and building schema'
(cd "$ROOT/migrations/baseline" && sha256sum -c fair-launch-pre-2026-09-03.sql.sha256)
(cd "$ROOT/migrations" && sha256sum -c FAIR_LAUNCH_SHA256SUMS)
(cd "$ROOT/migrations" && sha256sum -c baseline/fair-launch-pre-2026-09-03-ledger.sha256)
[ "$(wc -l < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03-ledger.sha256" | tr -d ' ')" = '43' ] || fail 'pre-0903 ledger manifest must contain exactly 43 entries'
MYSQL_VERSION="$(printf 'SELECT VERSION();\n' | mysql_root)"
assert_eq 'MySQL version' "$MYSQL_VERSION" '8.4.10'

printf 'CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;\n' "$DB" | mysql_root
printf '%s\n' "CREATE TABLE schema_migrations (filename VARCHAR(255) NOT NULL PRIMARY KEY, checksum CHAR(64) NOT NULL, status VARCHAR(16) NOT NULL DEFAULT 'applied', applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;" | mysql_db
while read -r checksum marked_filename; do
  filename="${marked_filename#\*}"
  printf "INSERT INTO schema_migrations(filename,checksum,status) VALUES ('%s','%s','applied');\n" "$filename" "$checksum" | mysql_db
done < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03-ledger.sha256"
mysql_db < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03.sql"
for file in "${MIGRATIONS[@]}"; do
  { printf 'SET @JC_FAIR_LAUNCH_MIGRATION_APPROVED=1;\n'; cat "$ROOT/migrations/$file"; } | mysql_db
done

docker_local exec -i "$CONTAINER_ID" sh -c '
  pw="$(cat /run/secrets/jc-fl-action-tx/local_admin)"
  mysql --defaults-extra-file=/tmp/jc-fl-root.cnf <<SQL
CREATE USER '\''local_admin'\''@'\''%'\'' IDENTIFIED BY '\''$pw'\'';
SQL
' >/dev/null
for table in admin_approval fair_launch_action_request fair_launch_action_transition fair_launch_preview governance_approval; do
  printf 'GRANT SELECT, INSERT, UPDATE, DELETE ON `%s`.`%s` TO '\''local_admin'\''@'\''%%'\'';\n' "$DB" "$table" | mysql_root
done

echo '[mysql8-action-tx] installing three distinct admin fixtures and two pending actions'
printf "%s\n" \
  "INSERT INTO admin_users(id,username,password_hash,email,display_name,role_code,is_enable) VALUES (101,'fl-init','x','','Initiator','operator',1),(102,'fl-approve','x','','Approver','approver',1),(103,'fl-execute','x','','Executor','super_admin',1);" \
  "INSERT INTO admin_approval(id,operation,target_kind,target_id,payload,reason,initiator_id,approver_id,status,reviewed_at,expires_at) VALUES (5001,'fair_launch.action','project','9001','{}','local test',101,102,'approved',NOW(),DATE_ADD(NOW(),INTERVAL 1 HOUR)),(5002,'fair_launch.action','project','9002','{}','local rollback test',101,102,'approved',NOW(),DATE_ADD(NOW(),INTERVAL 1 HOUR));" \
  "INSERT INTO fair_launch_preview(id,token_digest,preview_kind,project_id,action,operation,admin_id,canonical_digest,base_digest,canonical_payload,issued_at,expires_at) VALUES (6001,REPEAT('1',64),'action',9001,'pause','fair_launch.action',101,REPEAT('2',64),REPEAT('3',64),'{}',NOW(6),DATE_ADD(NOW(6),INTERVAL 1 HOUR)),(6002,REPEAT('4',64),'action',9002,'pause','fair_launch.action',101,REPEAT('5',64),REPEAT('6',64),'{}',NOW(6),DATE_ADD(NOW(6),INTERVAL 1 HOUR));" \
  "INSERT INTO fair_launch_action_request(id,project_id,canonical_digest,canonical_payload,action,execution_domain,initiator_admin_id,status,preview_expires_at,approval_expires_at,execution_expires_at,initiator_mfa_evidence_ref,initiator_mfa_verified_at,runtime_generation,policy_generation,source_registry_generation,config_generation,preview_id) VALUES (7001,9001,REPEAT('a',64),'{}','pause','operator_durable_outbox',101,'pending_approval',DATE_ADD(NOW(6),INTERVAL 10 MINUTE),DATE_ADD(NOW(6),INTERVAL 20 MINUTE),DATE_ADD(NOW(6),INTERVAL 30 MINUTE),'mfa-init-1',NOW(6),1,1,1,1,6001),(7002,9002,REPEAT('b',64),'{}','pause','operator_durable_outbox',101,'pending_approval',DATE_ADD(NOW(6),INTERVAL 10 MINUTE),DATE_ADD(NOW(6),INTERVAL 20 MINUTE),DATE_ADD(NOW(6),INTERVAL 30 MINUTE),'mfa-init-2',NOW(6),1,1,1,1,6002);" | mysql_db

echo '[mysql8-action-tx] proving rollback of a transition plus rejected canonical mutation'
{
  echo 'START TRANSACTION;'
  echo "INSERT INTO fair_launch_action_transition(action_request_id,sequence_no,from_status,to_status,actor_admin_id,mfa_evidence_ref,evidence_digest) VALUES(7002,1,'pending_approval','approval_bound',102,'mfa-approve-2',REPEAT('c',64));"
  echo "SELECT CONCAT('ROLLBACK_INSERTED=',COUNT(*)) FROM fair_launch_action_transition WHERE action_request_id=7002 AND sequence_no=1;"
  echo "UPDATE fair_launch_action_request SET canonical_digest=REPEAT('d',64) WHERE id=7002;"
  echo 'ROLLBACK;'
} | mysql_admin_force "$DB" > "$OUTPUT" 2>&1 || true
grep -Fxq 'ROLLBACK_INSERTED=1' "$OUTPUT" || { cat "$OUTPUT" >&2; fail 'rollback path did not prove the transition insert succeeded'; }
grep -Fq 'action request canonical identity is immutable' "$OUTPUT" || { cat "$OUTPUT" >&2; fail 'rollback path did not exercise canonical identity guard'; }
assert_eq 'rollback path SQL error count' "$(grep -c '^ERROR ' "$OUTPUT")" '1'
assert_eq 'rolled-back transition count' "$(query_root 'SELECT COUNT(*) FROM fair_launch_action_transition WHERE action_request_id=7002;')" '0'
assert_eq 'rolled-back request state' "$(query_root 'SELECT CONCAT(status,"|",canonical_digest) FROM fair_launch_action_request WHERE id=7002;')" "pending_approval|$(printf 'b%.0s' {1..64})"

echo '[mysql8-action-tx] proving row-lock serialization of competing sequence 1 transitions'
{
  echo "SELECT CONCAT('HOLDER_CONNECTION=',CONNECTION_ID());"
  echo 'START TRANSACTION;'
  echo "INSERT INTO fair_launch_action_transition(action_request_id,sequence_no,from_status,to_status,actor_admin_id,mfa_evidence_ref,evidence_digest) VALUES(7001,1,'pending_approval','approval_bound',102,'mfa-approve-1',REPEAT('d',64));"
  echo 'SELECT SLEEP(8);'
  echo 'ROLLBACK;'
} | mysql_admin "$DB" > "$LOCK_A_OUTPUT" 2>&1 &
LOCK_A_PID=$!

holder_connection=''
for _ in $(seq 1 80); do
  holder_connection="$(sed -n 's/^HOLDER_CONNECTION=//p' "$LOCK_A_OUTPUT" | tail -1)"
  [ -n "$holder_connection" ] && break
  sleep 0.1
done
case "$holder_connection" in ''|*[!0-9]*) fail 'holder connection id was not captured' ;; esac

lock_granted='0'
for _ in $(seq 1 80); do
  lock_granted="$(query_root "SELECT COUNT(*) FROM performance_schema.data_locks l JOIN performance_schema.threads t ON t.THREAD_ID=l.THREAD_ID WHERE t.PROCESSLIST_ID=$holder_connection AND l.OBJECT_SCHEMA='$DB' AND l.OBJECT_NAME='fair_launch_action_request' AND l.LOCK_TYPE='RECORD' AND l.LOCK_STATUS='GRANTED' AND l.LOCK_DATA='7001';")"
  [ "$lock_granted" -gt 0 ] && break
  sleep 0.1
done
[ "$lock_granted" -gt 0 ] || fail 'first transition did not acquire the action-request record lock'

printf "%s\n" "SELECT CONCAT('WAITER_CONNECTION=',CONNECTION_ID());" "INSERT INTO fair_launch_action_transition(action_request_id,sequence_no,from_status,to_status,actor_admin_id,mfa_evidence_ref,evidence_digest) VALUES(7001,1,'pending_approval','approval_bound',102,'mfa-approve-1',REPEAT('d',64));" | mysql_admin "$DB" > "$LOCK_B_OUTPUT" 2>&1 &
LOCK_B_PID=$!

waiter_connection=''
for _ in $(seq 1 80); do
  waiter_connection="$(sed -n 's/^WAITER_CONNECTION=//p' "$LOCK_B_OUTPUT" | tail -1)"
  [ -n "$waiter_connection" ] && break
  sleep 0.1
done
case "$waiter_connection" in ''|*[!0-9]*) fail 'waiter connection id was not captured' ;; esac
[ "$holder_connection" != "$waiter_connection" ] || fail 'holder and waiter unexpectedly share a connection id'

lock_waits='0'
for _ in $(seq 1 80); do
  lock_waits="$(query_root "SELECT COUNT(*) FROM performance_schema.data_lock_waits w JOIN performance_schema.data_locks req ON req.ENGINE_LOCK_ID=w.REQUESTING_ENGINE_LOCK_ID JOIN performance_schema.data_locks blk ON blk.ENGINE_LOCK_ID=w.BLOCKING_ENGINE_LOCK_ID JOIN performance_schema.threads req_t ON req_t.THREAD_ID=req.THREAD_ID JOIN performance_schema.threads blk_t ON blk_t.THREAD_ID=blk.THREAD_ID WHERE req_t.PROCESSLIST_ID=$waiter_connection AND blk_t.PROCESSLIST_ID=$holder_connection AND req.OBJECT_SCHEMA='$DB' AND blk.OBJECT_SCHEMA='$DB' AND req.OBJECT_NAME='fair_launch_action_request' AND blk.OBJECT_NAME='fair_launch_action_request' AND req.LOCK_TYPE='RECORD' AND blk.LOCK_TYPE='RECORD' AND req.LOCK_STATUS='WAITING' AND blk.LOCK_STATUS='GRANTED' AND req.LOCK_DATA='7001' AND blk.LOCK_DATA='7001';")"
  [ "$lock_waits" -gt 0 ] && break
  sleep 0.1
done
[ "$lock_waits" -gt 0 ] || fail 'competing transition did not enter a row-lock wait'

set +e
wait "$LOCK_A_PID"; lock_a_rc=$?
LOCK_A_PID=''
wait "$LOCK_B_PID"; lock_b_rc=$?
LOCK_B_PID=''
set -e
[ "$lock_a_rc" -eq 0 ] || { cat "$LOCK_A_OUTPUT" >&2; fail 'lock holder transaction failed'; }
[ "$lock_b_rc" -eq 0 ] || { cat "$LOCK_B_OUTPUT" >&2; fail 'waiting transition failed after lock-holder rollback'; }
assert_eq 'serialized transition count' "$(query_root 'SELECT COUNT(*) FROM fair_launch_action_transition WHERE action_request_id=7001 AND sequence_no=1;')" '1'
assert_eq 'request remains pending until explicit state mutation' "$(query_root 'SELECT status FROM fair_launch_action_request WHERE id=7001;')" 'pending_approval'

echo '[mysql8-action-tx] advancing the complete three-person lifecycle transactionally'
printf "%s\n" \
  'START TRANSACTION;' \
  "UPDATE fair_launch_action_request SET approver_admin_id=102,approver_mfa_evidence_ref='mfa-approve-1',approver_mfa_verified_at=NOW(6),admin_approval_id=5001,status='approval_bound' WHERE id=7001;" \
  'COMMIT;' | mysql_admin "$DB"
printf "%s\n" \
  'START TRANSACTION;' \
  "INSERT INTO fair_launch_action_transition(action_request_id,sequence_no,from_status,to_status,actor_admin_id,mfa_evidence_ref,evidence_digest) VALUES(7001,2,'approval_bound','approved',102,'mfa-approve-1',REPEAT('e',64));" \
  "UPDATE fair_launch_action_request SET status='approved' WHERE id=7001;" \
  'COMMIT;' | mysql_admin "$DB"
printf "%s\n" \
  'START TRANSACTION;' \
  "INSERT INTO fair_launch_action_transition(action_request_id,sequence_no,from_status,to_status,actor_admin_id,mfa_evidence_ref,evidence_digest) VALUES(7001,3,'approved','execution_bound',103,'mfa-execute-1',REPEAT('f',64));" \
  "UPDATE fair_launch_action_request SET executor_admin_id=103,executor_mfa_evidence_ref='mfa-execute-1',executor_mfa_verified_at=NOW(6),status='execution_bound' WHERE id=7001;" \
  'COMMIT;' | mysql_admin "$DB"

sequence=4
from='execution_bound'
for to in executing dispatched observing applied; do
  printf "%s\n" \
    'START TRANSACTION;' \
    "INSERT INTO fair_launch_action_transition(action_request_id,sequence_no,from_status,to_status,actor_admin_id,mfa_evidence_ref,evidence_digest) VALUES(7001,$sequence,'$from','$to',103,'mfa-execute-1',SHA2(CONCAT('transition-',$sequence),256));" \
    "UPDATE fair_launch_action_request SET status='$to' WHERE id=7001;" \
    'COMMIT;' | mysql_admin "$DB"
  from="$to"
  sequence=$((sequence + 1))
done

assert_eq 'final applied status' "$(query_root 'SELECT status FROM fair_launch_action_request WHERE id=7001;')" 'applied'
assert_eq 'complete transition count' "$(query_root 'SELECT COUNT(*) FROM fair_launch_action_transition WHERE action_request_id=7001;')" '7'
assert_eq 'contiguous transition lineage' "$(query_root "SELECT COUNT(*) FROM (SELECT sequence_no,from_status,to_status,LAG(to_status) OVER (ORDER BY sequence_no) previous_to FROM fair_launch_action_transition WHERE action_request_id=7001) x WHERE sequence_no<>1 AND NOT (from_status<=>previous_to);")" '0'
assert_eq 'three identities pairwise distinct' "$(query_root 'SELECT COUNT(*) FROM fair_launch_action_request WHERE id=7001 AND initiator_admin_id>0 AND approver_admin_id>0 AND executor_admin_id>0 AND initiator_admin_id<>approver_admin_id AND initiator_admin_id<>executor_admin_id AND approver_admin_id<>executor_admin_id;')" '1'

echo '[mysql8-action-tx] proving abnormal transitions and append-only mutations are rejected'
expect_error 'same-person approval transition' 'approval binding requires distinct actor/MFA' "INSERT INTO fair_launch_action_transition(action_request_id,sequence_no,from_status,to_status,actor_admin_id,mfa_evidence_ref,evidence_digest) VALUES(7002,1,'pending_approval','approval_bound',101,'mfa-init-2',REPEAT('1',64));"
expect_error 'sequence mismatch' 'action transition lineage/sequence mismatch' "INSERT INTO fair_launch_action_transition(action_request_id,sequence_no,from_status,to_status,actor_admin_id,mfa_evidence_ref,evidence_digest) VALUES(7002,2,'pending_approval','approval_bound',102,'mfa-approve-2',REPEAT('2',64));"
expect_error 'status mutation without transition' 'action request status change requires latest valid transition' "UPDATE fair_launch_action_request SET approver_admin_id=102,approver_mfa_evidence_ref='mfa-approve-2',approver_mfa_verified_at=NOW(6),admin_approval_id=5002,status='approval_bound' WHERE id=7002;"
expect_error 'transition update append-only' 'fair_launch_action_transition is append-only' "UPDATE fair_launch_action_transition SET evidence_digest=REPEAT('3',64) WHERE action_request_id=7001 AND sequence_no=1;"
expect_error 'transition delete append-only' 'fair_launch_action_transition is append-only' "DELETE FROM fair_launch_action_transition WHERE action_request_id=7001 AND sequence_no=1;"
expect_error 'request delete append-only' 'fair_launch_action_request is append-only' 'DELETE FROM fair_launch_action_request WHERE id=7001;'
expect_error 'completed approval immutable' 'completed approval identity is immutable' "UPDATE fair_launch_action_request SET approver_mfa_evidence_ref='changed' WHERE id=7001;"
expect_error 'completed executor immutable' 'completed executor identity is immutable' "UPDATE fair_launch_action_request SET executor_mfa_evidence_ref='changed' WHERE id=7001;"

set +e
printf "%s\n" "INSERT INTO fair_launch_action_request(id,project_id,canonical_digest,canonical_payload,action,execution_domain,initiator_admin_id,approver_admin_id,status,preview_expires_at,approval_expires_at,execution_expires_at,initiator_mfa_evidence_ref,initiator_mfa_verified_at,approver_mfa_evidence_ref,approver_mfa_verified_at,runtime_generation,policy_generation,source_registry_generation,config_generation,admin_approval_id,preview_id) VALUES(7003,9003,REPEAT('7',64),'{}','pause','operator_durable_outbox',101,101,'approved',DATE_ADD(NOW(6),INTERVAL 10 MINUTE),DATE_ADD(NOW(6),INTERVAL 20 MINUTE),DATE_ADD(NOW(6),INTERVAL 30 MINUTE),'mfa-init',NOW(6),'mfa-approve',NOW(6),1,1,1,1,5001,6001);" | mysql_admin "$DB" > "$OUTPUT" 2>&1
same_identity_rc=$?
set -e
[ "$same_identity_rc" -ne 0 ] || fail 'equal initiator/approver request unexpectedly succeeded'
grep -Fq 'chk_fl_action_request' "$OUTPUT" || { cat "$OUTPUT" >&2; fail 'three-person CHECK rejection was not observed'; }

assert_eq 'abnormal tests left rollback action pending' "$(query_root 'SELECT status FROM fair_launch_action_request WHERE id=7002;')" 'pending_approval'
assert_eq 'abnormal tests left no transition rows' "$(query_root 'SELECT COUNT(*) FROM fair_launch_action_transition WHERE action_request_id=7002;')" '0'

{
  echo 'authority=TEST_ONLY_NOT_PRODUCTION_ADMITTED'
  echo 'artifact_kind=SANITIZED_DISPOSABLE_MYSQL_ACTION_TRANSACTION_AND_ROW_LOCK_TEST_LOG_NOT_POST_STATE_CANDIDATE'
  echo 'credential_material=EPHEMERAL_MODE_600_SECRET_AND_MYSQL_OPTION_FILES_ONLY_NOT_ARGV_ENV_OR_EVIDENCE'
  echo "image=$IMAGE"
  echo "mysql_version=$MYSQL_VERSION"
  echo 'network=none'
  echo 'storage=tmpfs_disposable'
  echo 'three_person_lifecycle=pending_approval_to_applied_7_TRANSITIONS_PASS'
  echo 'initiator_approver_executor_pairwise_distinct=PASS'
  echo 'transition_sequence_and_lineage=PASS'
  echo 'competing_transition_row_lock_wait=OBSERVED_PASS'
  echo 'lock_holder_rollback_waiter_recovery=PASS'
  echo 'transaction_rollback_after_guard_error=PASS'
  echo 'append_only_request_and_transition_guards=PASS'
  echo 'abnormal_transition_rejections=PASS'
  echo 'production_action_execution=NOT_EXERCISED_OR_MODIFIED_BY_THIS_TEST'
  echo 'production_activation=NOT_EXERCISED_OR_MODIFIED_BY_THIS_TEST'
  echo 'verdict=PASS'
} > "$EVIDENCE"
chmod 600 "$EVIDENCE" 2>/dev/null || true

echo "[mysql8-action-tx] PASS evidence=$EVIDENCE"
echo "[mysql8-action-tx] evidence_sha256=$(sha256sum "$EVIDENCE" | awk '{print $1}')"
