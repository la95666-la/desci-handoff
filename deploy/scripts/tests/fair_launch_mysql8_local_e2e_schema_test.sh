#!/usr/bin/env bash
set -euo pipefail
# TEST ONLY. Starts a disposable network-none MySQL 8.4 instance and applies the
# pinned baseline plus 0903-0912. It is not a production migration or authority.
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
IMAGE='mysql@sha256:c831a0f11348d402b43d77453e17d770be2eef356615a2823fe0f5a0d6c8b9af'
SOCKET='/var/run/docker.sock'
RUN_ID="$(uuidgen | tr '[:upper:]' '[:lower:]')"
RUN_HEX="${RUN_ID//-/}"
NAME="jc-fl-local-e2e-schema-$RUN_HEX"
LABEL_KEY='joincare.fair-launch.local-e2e-run'
DB="jc_fl_e2e_${RUN_HEX}"
SECRET_DIR=''; CONTAINER_ID=''; OUTPUT="/tmp/jc-fl-e2e-$RUN_HEX.out"
fail(){ echo "fair_launch_mysql8_local_e2e_schema_test: FAIL: $*" >&2; exit 1; }
docker_local(){ docker -H "unix://$SOCKET" "$@"; }
cleanup(){
  if [ -n "$CONTAINER_ID" ]; then
    observed="$(docker_local inspect --format "{{index .Config.Labels \"$LABEL_KEY\"}}" "$CONTAINER_ID" 2>/dev/null || true)"
    [ "$observed" != "$RUN_ID" ] || docker_local rm -f "$CONTAINER_ID" >/dev/null 2>&1 || true
  fi
  [ -z "$SECRET_DIR" ] || rm -rf "$SECRET_DIR"
  rm -f "$OUTPUT"
}
for forbidden in JC_DEPLOY_RPC JC_DEPLOY_CHAIN_ID JC_DEPLOY_PRIVATE_KEY JC_MYSQL_ROOT_PASSWORD MYSQL_PWD FAIR_LAUNCH_DISPOSABLE_MYSQL_DSN DOCKER_HOST DOCKER_CONTEXT DOCKER_TLS_VERIFY DOCKER_CERT_PATH; do [ -z "${!forbidden:-}" ] || fail "refusing external/production environment: $forbidden"; done
command -v docker >/dev/null || fail 'docker required'; command -v openssl >/dev/null || fail 'openssl required'; command -v sha256sum >/dev/null || fail 'sha256sum required'
[ -S "$SOCKET" ] || fail 'local Docker Unix socket unavailable'
[ "$(readlink -f "$SOCKET")" = '/run/docker.sock' ] || fail 'unexpected Docker socket'
docker_local version >/dev/null 2>&1 || fail 'local Docker daemon unavailable'
docker_local image inspect "$IMAGE" >/dev/null 2>&1 || fail 'pinned MySQL image unavailable locally'
trap cleanup EXIT INT TERM
umask 077; SECRET_DIR="$(mktemp -d)"; openssl rand -hex 32 > "$SECRET_DIR/root"; chmod 600 "$SECRET_DIR/root"
CONTAINER_ID="$(docker_local run -d --name "$NAME" --label "$LABEL_KEY=$RUN_ID" --network none --tmpfs /var/lib/mysql:rw,nosuid,nodev,size=2g --tmpfs /tmp:rw,nosuid,nodev,size=256m -v "$SECRET_DIR/root:/run/secrets/root:ro" -v "$ROOT:/repo:ro" -e MYSQL_ROOT_PASSWORD_FILE=/run/secrets/root "$IMAGE" --skip-log-bin --local-infile=OFF --secure-file-priv=NULL)"
[ "$(docker_local inspect --format "{{index .Config.Labels \"$LABEL_KEY\"}}" "$CONTAINER_ID")" = "$RUN_ID" ] || fail 'container ownership mismatch'
docker_local exec "$CONTAINER_ID" sh -c 'umask 077; { echo "[client]"; echo "user=root"; printf "password="; cat /run/secrets/root; echo "host=127.0.0.1"; echo "protocol=tcp"; } >/tmp/root.cnf; chmod 600 /tmp/root.cnf'
for _ in $(seq 1 90); do docker_local exec "$CONTAINER_ID" mysqladmin --defaults-extra-file=/tmp/root.cnf ping --silent >/dev/null 2>&1 && break; sleep 1; done
docker_local exec "$CONTAINER_ID" mysqladmin --defaults-extra-file=/tmp/root.cnf ping --silent >/dev/null || fail 'MySQL not ready'
mysql_root(){ docker_local exec -i "$CONTAINER_ID" mysql --defaults-extra-file=/tmp/root.cnf --batch --raw --skip-column-names "$@"; }
mysql_db(){ mysql_root "$DB"; }
query(){ printf '%s\n' "$1" | mysql_db | tr -d '\r'; }
assert_eq(){ [ "$2" = "$3" ] || fail "$1 got=$2 want=$3"; }
(cd "$ROOT/migrations" && sha256sum -c FAIR_LAUNCH_SHA256SUMS)
assert_eq 'MySQL version' "$(printf 'SELECT VERSION();\n'|mysql_root)" '8.4.10'
printf 'CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;\n' "$DB" | mysql_root
printf '%s\n' "CREATE TABLE schema_migrations(filename VARCHAR(255) PRIMARY KEY,checksum CHAR(64) NOT NULL,status VARCHAR(16) NOT NULL,applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;" | mysql_db
while read -r checksum marked; do file="${marked#\*}"; printf "INSERT INTO schema_migrations VALUES('%s','%s','applied',NOW(6));\n" "$file" "$checksum" | mysql_db; done < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03-ledger.sha256"
mysql_db < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03.sql"
FROZEN=(2026_09_03_fair_launch_project_model.sql 2026_09_04_fair_launch_ce_v2_governance.sql 2026_09_05_fair_launch_signing_release_control.sql 2026_09_06_fair_launch_action_execution.sql 2026_09_07_fair_launch_position_projection.sql)
LOCAL=(2026_09_08_fair_launch_local_e2e_binding.sql 2026_09_09_fair_launch_financial_projection_v3.sql 2026_09_10_fair_launch_governance_projection_v3.sql 2026_09_11_fair_launch_effect_lineage_v3.sql 2026_09_12_fair_launch_typed_ledger_v3.sql)
for f in "${FROZEN[@]}"; do { echo 'SET @JC_FAIR_LAUNCH_MIGRATION_APPROVED=1;'; cat "$ROOT/migrations/$f"; } | mysql_db; done
for f in "${LOCAL[@]}"; do
  set +e; mysql_db < "$ROOT/migrations/$f" >"$OUTPUT" 2>&1; rc=$?; set -e
  [ "$rc" -ne 0 ] && grep -Fq 'Fair Launch local E2E migration requires local-only execution gate' "$OUTPUT" || fail "$f accepted without local gate"
  { echo 'SET @JC_FAIR_LAUNCH_LOCAL_E2E_APPROVED=1;'; printf "SET @JC_FAIR_LAUNCH_LOCAL_E2E_RUN_ID='%s';\n" "$RUN_ID"; cat "$ROOT/migrations/$f"; } | mysql_db
done
# Reapply on the still-empty disposable schema. Migration 0912 holds an exclusive
# named maintenance lock and rejects reapply after any v3 effect/financial row exists.
for f in "${LOCAL[@]}"; do { echo 'SET @JC_FAIR_LAUNCH_LOCAL_E2E_APPROVED=1;'; printf "SET @JC_FAIR_LAUNCH_LOCAL_E2E_RUN_ID='%s';\n" "$RUN_ID"; cat "$ROOT/migrations/$f"; } | mysql_db; done
assert_eq 'v3/local table count' "$(query "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE' AND (TABLE_NAME LIKE '%_v3' OR TABLE_NAME LIKE 'fair_launch_local_e2e%');")" '25'
assert_eq 'v3 engine/collation' "$(query "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE' AND (TABLE_NAME LIKE '%_v3' OR TABLE_NAME LIKE 'fair_launch_local_e2e%') AND ENGINE='InnoDB' AND TABLE_COLLATION='utf8mb4_0900_ai_ci';")" '25'
assert_eq 'immutable trigger count' "$(query "SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN('trg_fl_ledger_v3_no_update','trg_fl_ledger_v3_no_delete','trg_fl_gov_ledger_v3_no_update','trg_fl_gov_ledger_v3_no_delete','trg_fl_lineage_v3_no_update','trg_fl_lineage_v3_no_delete','trg_fl_journal_v3_no_update','trg_fl_journal_v3_no_delete','trg_fl_lineage_reversal_v3_no_update','trg_fl_lineage_reversal_v3_no_delete','trg_fl_journal_reversal_v3_no_update','trg_fl_journal_reversal_v3_no_delete','trg_fl_attestation_v3_no_update','trg_fl_attestation_v3_no_delete','trg_fl_gov_history_v3_no_update','trg_fl_gov_history_v3_no_delete','trg_fl_policy_history_v3_no_update','trg_fl_policy_history_v3_no_delete','trg_fl_observation_v3_no_update','trg_fl_observation_v3_no_delete');")" '20'
assert_eq 'local run guarded triggers' "$(query "SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN('trg_fl_local_run_v3_immutable','trg_fl_local_run_v3_no_delete');")" '2'
assert_eq 'reversal closure immutable triggers' "$(query "SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN('trg_fl_reversal_closure_v3_no_update','trg_fl_reversal_closure_v3_no_delete');")" '2'
assert_eq 'reversal validation triggers' "$(query "SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN('trg_fl_lineage_reversal_v3_validate','trg_fl_reversal_closure_v3_validate','trg_fl_ledger_v3_validate_compensation','trg_fl_gov_ledger_v3_validate_compensation');")" '4'
assert_eq 'temporary routines' "$(query "SELECT COUNT(*) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA=DATABASE() AND ROUTINE_NAME LIKE 'jc_fair_launch_local_e2e_%';")" '0'
assert_eq 'project binding FK columns' "$(query "SELECT COUNT(*) FROM information_schema.KEY_COLUMN_USAGE WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_project_binding_v3' AND CONSTRAINT_NAME='fk_fl_project_binding_project';")" '2'
assert_eq 'lineage CE FK columns' "$(query "SELECT COUNT(*) FROM information_schema.KEY_COLUMN_USAGE WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_application_lineage_v3' AND CONSTRAINT_NAME='fk_fl_lineage_ce_application';")" '4'
assert_eq 'ledger lineage FK columns' "$(query "SELECT COUNT(*) FROM information_schema.KEY_COLUMN_USAGE WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_ledger_entry_v3' AND CONSTRAINT_NAME='fk_fl_ledger_lineage';")" '8'
assert_eq 'project compensation one-per-original index' "$(query "SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_ledger_entry_v3' AND INDEX_NAME='uk_fl_ledger_compensation';")" 'reversal_of_ledger_id'
assert_eq 'governance compensation one-per-original index' "$(query "SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_governance_ledger_entry_v3' AND INDEX_NAME='uk_fl_gov_ledger_compensation';")" 'reversal_of_ledger_id'
assert_eq 'projection lineage FKs' "$(query "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_NAME IN('fk_fl_project_state_lineage','fk_fl_position_v3_lineage','fk_fl_gov_current_lineage','fk_fl_gov_history_lineage','fk_fl_policy_current_lineage','fk_fl_policy_history_lineage');")" '6'
# Third-round structural facts are verified with ordinary FK checks; transactional closure/concurrency has a dedicated two-session test.
assert_eq 'v3/local table count' "$(query "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE' AND (TABLE_NAME LIKE '%_v3' OR TABLE_NAME LIKE 'fair_launch_local_e2e%');")" '25'
assert_eq 'placement generation FK columns' "$(query "SELECT COUNT(*) FROM information_schema.KEY_COLUMN_USAGE WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_application_lineage_v3' AND CONSTRAINT_NAME='fk_fl_lineage_placement_revision';")" '4'
assert_eq 'cursor reversal FK columns' "$(query "SELECT COUNT(*) FROM information_schema.KEY_COLUMN_USAGE WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_cursor_checkpoint_v3' AND CONSTRAINT_NAME='fk_fl_cursor_checkpoint_reversal';")" '6'
assert_eq 'dependency predecessor FK columns' "$(query "SELECT COUNT(*) FROM information_schema.KEY_COLUMN_USAGE WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_effect_dependency_v3' AND CONSTRAINT_NAME='fk_fl_dependency_predecessor';")" '7'
assert_eq 'dependency stable primary key' "$(query "SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_effect_dependency_v3' AND INDEX_NAME='PRIMARY';")" 'id'
assert_eq 'project binding token decimal columns' "$(query "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_project_binding_v3' AND COLUMN_NAME IN('project_token_decimals','payment_token_decimals');")" '2'
assert_eq 'serialization trigger uses row lock' "$(query "SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND ACTION_STATEMENT LIKE '%FOR UPDATE%' AND TRIGGER_NAME IN('trg_fl_journal_v3_set_guard','trg_fl_journal_reversal_v3_validate','trg_fl_dependency_v3_set_guard','trg_fl_attestation_v3_set_guard','trg_fl_cursor_checkpoint_v3_set_guard','trg_fl_gov_history_v3_set_guard','trg_fl_policy_history_v3_set_guard','trg_fl_ledger_v3_validate_compensation','trg_fl_gov_ledger_v3_validate_compensation','trg_fl_reversal_evidence_item_v3_validate','trg_fl_reversal_evidence_manifest_v3_validate','trg_fl_reversal_closure_v3_validate');")" '12'
# Ordinary FK mismatch: a genuine placement generation cannot be relabelled by lineage.
cat <<SQL | mysql_db
INSERT INTO chain_contract_source(id,chain_id,contract_address,contract_version,funding_mode,registered_at,start_block,enabled) VALUES(880001,31337,'0x1111111111111111111111111111111111111111','fair_launch_v1','fair_launch',NOW(6),1,1);
INSERT INTO chain_contract_source_revision(id,source_id,revision_no,activation_block_hash,proxy_kind,abi_digest,confirmation_policy,governance_tx_hash,governance_operation_id,implementation_address,implementation_code_hash,dependency_manifest_digest,abi_version,from_block_number,from_transaction_index,from_tx_log_ordinal_inclusive,created_at) VALUES(880002,880001,1,CONCAT('0x',REPEAT('1',64)),'uups',REPEAT('2',64),'20_blocks',CONCAT('0x',REPEAT('3',64)),CONCAT('0x',REPEAT('4',64)),'0x2222222222222222222222222222222222222222',REPEAT('5',64),REPEAT('6',64),'fair-launch-v1',1,0,0,NOW(6));
INSERT INTO chain_event_identity_v2(id,chain_id,tx_hash,tx_log_ordinal,created_at) VALUES(880003,31337,CONCAT('0x',REPEAT('7',64)),0,NOW(6));
INSERT INTO chain_event_placement_v2(id,event_identity_id,generation,block_number,block_hash,transaction_index,rpc_log_index,source_revision_id,canonical_slot,finalized_at) VALUES(880004,880003,1,1,CONCAT('0x',REPEAT('8',64)),0,0,880002,1,NOW(6));
INSERT INTO chain_event_application_attempt_v2(id,event_identity_id,placement_id,effect_version,expected_effect,attempt_no,disposition,effect_digest,applied_at) VALUES(880005,880003,880004,1,1,1,'exact',REPEAT('9',64),NOW(6));
INSERT INTO fair_launch_local_e2e_run(run_id,environment,exact_rpc_url,redis_endpoint,chain_id,database_name,mysql_server_uuid,mysql_version,container_id,owner_label_digest,schema_generation,baseline_digest,migration_manifest_digest,schema_digest,grant_digest,source_registry_digest,hardhat_genesis_hash,chain_manifest_digest,deployment_manifest_digest,runtime_admission_digest,created_at,expires_at,state) VALUES('$RUN_ID','local-e2e','http://127.0.0.1:18545','127.0.0.1:16379',31337,'$DB',LOWER('00000000-0000-0000-0000-000000000001'),'8.4.10','$CONTAINER_ID',REPEAT('a',64),12,REPEAT('b',64),REPEAT('c',64),REPEAT('d',64),REPEAT('e',64),REPEAT('f',64),CONCAT('0x',REPEAT('1',64)),REPEAT('2',64),REPEAT('3',64),REPEAT('4',64),NOW(6),DATE_ADD(NOW(6),INTERVAL 1 HOUR),'verified');
INSERT INTO fair_launch_source_revision_binding_v3(id,run_row_id,chain_id,source_id,source_revision_id,from_block_number,from_transaction_index,from_tx_log_ordinal_inclusive,source_binding_digest,created_at) SELECT 880006,id,31337,880001,880002,1,0,0,REPEAT('a',64),NOW(6) FROM fair_launch_local_e2e_run WHERE run_id='$RUN_ID';
SQL
set +e; printf "INSERT INTO fair_launch_application_lineage_v3(id,ce_application_attempt_id,run_row_id,chain_id,source_binding_id,source_id,event_identity_id,placement_id,source_revision_id,effect_manifest_version,placement_generation,disposition,expected_effect,payload_digest,effect_digest,ledger_digest,mutation_digest,state,applied_at,created_at) SELECT 880007,880005,id,31337,880006,880001,880003,880004,880002,1,2,'forward',1,REPEAT('a',64),REPEAT('b',64),REPEAT('c',64),REPEAT('d',64),'applied',NOW(6),NOW(6) FROM fair_launch_local_e2e_run WHERE run_id='$RUN_ID';
" | mysql_db >"$OUTPUT" 2>&1; rc=$?; set -e
[ "$rc" -ne 0 ] || fail 'lineage accepted mismatched placement generation'
# Once materialization rows exist, 0912 must fail before replacing triggers.
printf "INSERT INTO fair_launch_application_lineage_v3(id,ce_application_attempt_id,run_row_id,chain_id,source_binding_id,source_id,event_identity_id,placement_id,source_revision_id,effect_manifest_version,placement_generation,disposition,expected_effect,payload_digest,effect_digest,ledger_digest,mutation_digest,state,applied_at,created_at) SELECT 880007,880005,id,31337,880006,880001,880003,880004,880002,1,1,'forward',1,REPEAT('a',64),REPEAT('b',64),REPEAT('c',64),REPEAT('d',64),'applied',NOW(6),NOW(6) FROM fair_launch_local_e2e_run WHERE run_id='$RUN_ID';\n" | mysql_db
set +e; { echo 'SET @JC_FAIR_LAUNCH_LOCAL_E2E_APPROVED=1;'; cat "$ROOT/migrations/2026_09_12_fair_launch_typed_ledger_v3.sql"; } | mysql_db >"$OUTPUT" 2>&1; rc=$?; set -e
[ "$rc" -ne 0 ] && grep -Fq 'Fair Launch 0912 reapply requires fresh disposable schema' "$OUTPUT" || fail '0912 replaced triggers on nonempty schema'
echo "fair_launch_mysql8_local_e2e_schema_test: PASS run_id=$RUN_ID container_id=$CONTAINER_ID tables=25 serialization_triggers=12 maintenance_reapply_guard=1"
