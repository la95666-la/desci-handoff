#!/usr/bin/env bash
set -euo pipefail
# TEST ONLY. Disposable pinned MySQL 8.4; no RPC, wallet, server or external
# credentials are accepted. Proves 0915 stays inert and receipts fence progress.
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
IMAGE='mysql@sha256:c831a0f11348d402b43d77453e17d770be2eef356615a2823fe0f5a0d6c8b9af'
SOCKET='/var/run/docker.sock'; RUN_ID="$(uuidgen | tr '[:upper:]' '[:lower:]')"; HEX="${RUN_ID//-/}"; NAME="jc-fl-mainnet-scanner-$HEX"; LABEL='joincare.fair-launch.mainnet-scanner-test'; DB="jc_fl_scan_${HEX:0:16}"; SECRET_DIR=''; CONTAINER_ID=''; OUT="/tmp/jc-fl-mainnet-scanner-$HEX.out"
fail(){ echo "fair_launch_mysql8_mainnet_archive_scanner_schema_test: FAIL: $*" >&2; exit 1; }
docker_local(){ docker -H "unix://$SOCKET" "$@"; }
cleanup(){ if [ -n "$CONTAINER_ID" ]; then observed="$(docker_local inspect --format "{{index .Config.Labels \"$LABEL\"}}" "$CONTAINER_ID" 2>/dev/null || true)"; [ "$observed" != "$RUN_ID" ] || docker_local rm -f "$CONTAINER_ID" >/dev/null 2>&1 || true; fi; [ -z "$SECRET_DIR" ] || rm -rf "$SECRET_DIR"; rm -f "$OUT"; }
for forbidden in JC_DEPLOY_RPC JC_DEPLOY_CHAIN_ID JC_DEPLOY_PRIVATE_KEY JC_MYSQL_ROOT_PASSWORD MYSQL_PWD FAIR_LAUNCH_DISPOSABLE_MYSQL_DSN DOCKER_HOST DOCKER_CONTEXT DOCKER_TLS_VERIFY DOCKER_CERT_PATH; do [ -z "${!forbidden:-}" ] || fail "refusing external/production environment: $forbidden"; done
command -v docker >/dev/null || fail 'docker required'; command -v openssl >/dev/null || fail 'openssl required'; command -v uuidgen >/dev/null || fail 'uuidgen required'; [ -S "$SOCKET" ] || fail 'local Docker socket unavailable'; docker_local image inspect "$IMAGE" >/dev/null 2>&1 || fail 'pinned MySQL image unavailable locally'; trap cleanup EXIT INT TERM
umask 077; SECRET_DIR="$(mktemp -d)"; openssl rand -hex 32 > "$SECRET_DIR/root"; chmod 600 "$SECRET_DIR/root"
CONTAINER_ID="$(docker_local run -d --name "$NAME" --label "$LABEL=$RUN_ID" --network none --tmpfs /var/lib/mysql:rw,nosuid,nodev,size=2g --tmpfs /tmp:rw,nosuid,nodev,size=256m -v "$SECRET_DIR/root:/run/secrets/root:ro" -v "$ROOT:/repo:ro" -e MYSQL_ROOT_PASSWORD_FILE=/run/secrets/root "$IMAGE" --skip-log-bin --local-infile=OFF --secure-file-priv=NULL)"
docker_local exec "$CONTAINER_ID" sh -c 'umask 077; { echo "[client]"; echo "user=root"; printf "password="; cat /run/secrets/root; echo "host=127.0.0.1"; echo "protocol=tcp"; } >/tmp/root.cnf; chmod 600 /tmp/root.cnf'
for _ in $(seq 1 90); do docker_local exec "$CONTAINER_ID" mysqladmin --defaults-extra-file=/tmp/root.cnf ping --silent >/dev/null 2>&1 && break; sleep 1; done
docker_local exec "$CONTAINER_ID" mysqladmin --defaults-extra-file=/tmp/root.cnf ping --silent >/dev/null || fail 'MySQL not ready'
mysql_root(){ docker_local exec -i "$CONTAINER_ID" mysql --defaults-extra-file=/tmp/root.cnf --batch --raw --skip-column-names "$@"; }
mysql_db(){ mysql_root "$DB"; }
query(){ printf '%s\n' "$1" | mysql_db | tr -d '\r'; }
assert_eq(){ [ "$2" = "$3" ] || fail "$1 got=$2 want=$3"; }
expect_fail(){ local message="$1"; shift; set +e; "$@" >"$OUT" 2>&1; local rc=$?; set -e; [ "$rc" -ne 0 ] && grep -Fq "$message" "$OUT" || fail "expected rejection: $message"; }
(cd "$ROOT/migrations" && sha256sum -c FAIR_LAUNCH_SHA256SUMS)
printf 'CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;\n' "$DB" | mysql_root
printf '%s\n' "CREATE TABLE schema_migrations(filename VARCHAR(255) PRIMARY KEY,checksum CHAR(64) NOT NULL,status VARCHAR(16) NOT NULL,applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;" | mysql_db
while read -r checksum marked; do file="${marked#\*}"; printf "INSERT INTO schema_migrations VALUES('%s','%s','applied',NOW(6));\n" "$file" "$checksum" | mysql_db; done < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03-ledger.sha256"
mysql_db < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03.sql"
FROZEN=(2026_09_03_fair_launch_project_model.sql 2026_09_04_fair_launch_ce_v2_governance.sql 2026_09_05_fair_launch_signing_release_control.sql 2026_09_06_fair_launch_action_execution.sql 2026_09_07_fair_launch_position_projection.sql)
LOCAL=(2026_09_08_fair_launch_local_e2e_binding.sql 2026_09_09_fair_launch_financial_projection_v3.sql 2026_09_10_fair_launch_governance_projection_v3.sql 2026_09_11_fair_launch_effect_lineage_v3.sql 2026_09_12_fair_launch_typed_ledger_v3.sql)
for f in "${FROZEN[@]}"; do { echo 'SET @JC_FAIR_LAUNCH_MIGRATION_APPROVED=1;'; cat "$ROOT/migrations/$f"; } | mysql_db; done
for f in "${LOCAL[@]}"; do { echo 'SET @JC_FAIR_LAUNCH_LOCAL_E2E_APPROVED=1;'; printf "SET @JC_FAIR_LAUNCH_LOCAL_E2E_RUN_ID='%s';\n" "$RUN_ID"; cat "$ROOT/migrations/$f"; } | mysql_db; done
{ echo 'SET @JC_FAIR_LAUNCH_MAINNET_ACTIVATION_APPROVED=1;'; cat "$ROOT/migrations/2026_09_13_fair_launch_bsc_mainnet_activation.sql"; } | mysql_db
{ echo 'SET @JC_FAIR_LAUNCH_MAINNET_BOOTSTRAP_APPROVED=1;'; cat "$ROOT/migrations/2026_09_14_fair_launch_mainnet_bootstrap_receipt.sql"; } | mysql_db
expect_fail 'approved execution gate' mysql_db < "$ROOT/migrations/2026_09_15_fair_launch_mainnet_archive_scanner.sql"
{ echo 'SET @JC_FAIR_LAUNCH_MAINNET_SCANNER_APPROVED=1;'; cat "$ROOT/migrations/2026_09_15_fair_launch_mainnet_archive_scanner.sql"; } | mysql_db
assert_eq 'scanner tables' "$(query "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN('fair_launch_mainnet_archive_cursor_v1','fair_launch_mainnet_archive_batch_receipt_v1','fair_launch_mainnet_archive_batch_event_v1','fair_launch_mainnet_archive_dead_letter_v1');")" '4'
assert_eq 'scanner rows inert' "$(query "SELECT CONCAT((SELECT COUNT(*) FROM fair_launch_mainnet_archive_cursor_v1),'/',(SELECT COUNT(*) FROM fair_launch_mainnet_archive_batch_receipt_v1),'/',(SELECT COUNT(*) FROM fair_launch_mainnet_archive_batch_event_v1),'/',(SELECT COUNT(*) FROM fair_launch_mainnet_archive_dead_letter_v1));")" '0/0/0/0'
if [ "${JC_TEST_FAIR_LAUNCH_MAINNET_MATERIALIZATION_0918:-0}" = '1' ]; then
  { echo 'SET @JC_FAIR_LAUNCH_MAINNET_RECOVERY_APPROVED=1;'; cat "$ROOT/migrations/2026_09_16_fair_launch_mainnet_archive_recovery.sql"; } | mysql_db
  { echo 'SET @JC_FAIR_LAUNCH_MAINNET_RECOVERY_PROVIDER_APPROVED=1;'; cat "$ROOT/migrations/2026_09_17_fair_launch_mainnet_recovery_provider.sql"; } | mysql_db
  expect_fail 'approved execution gate' mysql_db < "$ROOT/migrations/2026_09_18_fair_launch_mainnet_materialization_authority.sql"
  { echo 'SET @JC_FAIR_LAUNCH_MAINNET_MATERIALIZATION_APPROVED=1;'; cat "$ROOT/migrations/2026_09_18_fair_launch_mainnet_materialization_authority.sql"; } | mysql_db
  assert_eq 'materialization tables' "$(query "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN('fair_launch_mainnet_archive_event_payload_v1','fair_launch_mainnet_materialization_profile_v1','fair_launch_mainnet_materialization_cursor_v1','fair_launch_mainnet_materialization_receipt_v1');")" '4'
  assert_eq 'materialization rows inert' "$(query "SELECT CONCAT((SELECT COUNT(*) FROM fair_launch_mainnet_archive_event_payload_v1),'/',(SELECT COUNT(*) FROM fair_launch_mainnet_materialization_profile_v1),'/',(SELECT COUNT(*) FROM fair_launch_mainnet_materialization_cursor_v1),'/',(SELECT COUNT(*) FROM fair_launch_mainnet_materialization_receipt_v1));")" '0/0/0/0'
fi
cat <<SQL | mysql_db
INSERT INTO chain_contract_source(id,chain_id,contract_address,contract_version,funding_mode,registered_at,start_block,enabled) VALUES(560001,56,'0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','fair_launch_v1','fair_launch',NOW(6),100,1);
INSERT INTO chain_contract_source_revision(id,source_id,revision_no,activation_block_hash,proxy_kind,abi_digest,confirmation_policy,governance_tx_hash,governance_operation_id,implementation_address,implementation_code_hash,dependency_manifest_digest,abi_version,from_block_number,from_transaction_index,from_tx_log_ordinal_inclusive,created_at) VALUES(560002,560001,1,CONCAT('0x',REPEAT('1',64)),'uups',REPEAT('2',64),'20_blocks',CONCAT('0x',REPEAT('3',64)),CONCAT('0x',REPEAT('4',64)),'0x2222222222222222222222222222222222222222',REPEAT('5',64),REPEAT('6',64),'fair-launch-v1',100,0,0,NOW(6));
INSERT INTO fair_launch_mainnet_activation_v1(activation_id,environment,chain_id,rpc_genesis_hash,rpc_identity_digest,proxy_address,proxy_runtime_code_hash,implementation_address,implementation_code_hash,abi_digest,source_id,source_revision_id,source_start_block,source_start_block_hash,source_registry_digest,payment_token_allowlist_digest,project_allowlist_digest,release_manifest_digest,migration_manifest_digest,activation_manifest_digest,signing_key_id,signature_digest,governance_evidence_digest,valid_from,valid_until,created_at) VALUES('$RUN_ID','test-server',56,CONCAT('0x',REPEAT('0',64)),REPEAT('1',64),'0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',REPEAT('7',64),'0x2222222222222222222222222222222222222222',REPEAT('5',64),REPEAT('2',64),560001,560002,100,CONCAT('0x',REPEAT('1',64)),REPEAT('8',64),REPEAT('9',64),REPEAT('a',64),REPEAT('b',64),REPEAT('c',64),REPEAT('d',64),'test-key',REPEAT('e',64),REPEAT('f',64),NOW(6),DATE_ADD(NOW(6),INTERVAL 1 HOUR),NOW(6));
SET @a=(SELECT authority_row_id FROM fair_launch_mainnet_activation_v1 WHERE activation_id='$RUN_ID');
INSERT INTO fair_launch_mainnet_activation_lease_v1(activation_row_id,chain_id,lease_generation,fence_epoch,lease_token_hash,component_set_digest,approval_evidence_digest,valid_from,valid_until,status,active_slot,created_at) VALUES(@a,56,1,1,REPEAT('1',64),REPEAT('2',64),REPEAT('3',64),NOW(6),DATE_ADD(NOW(6),INTERVAL 1 HOUR),'active',1,NOW(6));
INSERT INTO fair_launch_mainnet_archive_cursor_v1(activation_row_id,chain_id,source_id,source_revision_id,archived_through_block,archived_through_block_hash,lease_generation,fence_epoch,lease_token_hash,status,cursor_generation,created_at,updated_at) VALUES(@a,56,560001,560002,99,CONCAT('0x',REPEAT('a',64)),1,1,REPEAT('1',64),'active',1,NOW(6),NOW(6));
INSERT INTO fair_launch_mainnet_archive_batch_receipt_v1(activation_row_id,chain_id,source_id,source_revision_id,lease_generation,fence_epoch,from_block,through_block,from_parent_block_hash,through_block_hash,provider_identity_digest,block_count,event_count,coverage_digest,previous_receipt_digest,receipt_digest,created_at) VALUES(@a,56,560001,560002,1,1,100,100,CONCAT('0x',REPEAT('a',64)),CONCAT('0x',REPEAT('b',64)),REPEAT('4',64),1,0,REPEAT('5',64),NULL,REPEAT('6',64),NOW(6));
SET @b1=LAST_INSERT_ID();
UPDATE fair_launch_mainnet_archive_cursor_v1 SET archived_through_block=100,archived_through_block_hash=CONCAT('0x',REPEAT('b',64)),last_batch_receipt_id=@b1,cursor_generation=2,updated_at=NOW(6) WHERE activation_row_id=@a;
SQL
expect_fail 'batch predecessor mismatch' mysql_db <<SQL
INSERT INTO fair_launch_mainnet_archive_batch_receipt_v1(activation_row_id,chain_id,source_id,source_revision_id,lease_generation,fence_epoch,from_block,through_block,from_parent_block_hash,through_block_hash,provider_identity_digest,block_count,event_count,coverage_digest,previous_receipt_digest,receipt_digest,created_at) VALUES((SELECT authority_row_id FROM fair_launch_mainnet_activation_v1 WHERE activation_id='$RUN_ID'),56,560001,560002,1,1,102,102,CONCAT('0x',REPEAT('b',64)),CONCAT('0x',REPEAT('c',64)),REPEAT('4',64),1,0,REPEAT('5',64),REPEAT('6',64),REPEAT('7',64),NOW(6));
SQL
cat <<SQL | mysql_db
SET @a=(SELECT authority_row_id FROM fair_launch_mainnet_activation_v1 WHERE activation_id='$RUN_ID');
INSERT INTO fair_launch_mainnet_archive_batch_receipt_v1(activation_row_id,chain_id,source_id,source_revision_id,lease_generation,fence_epoch,from_block,through_block,from_parent_block_hash,through_block_hash,provider_identity_digest,block_count,event_count,coverage_digest,previous_receipt_digest,receipt_digest,created_at) VALUES(@a,56,560001,560002,1,1,101,101,CONCAT('0x',REPEAT('b',64)),CONCAT('0x',REPEAT('c',64)),REPEAT('4',64),1,1,REPEAT('7',64),REPEAT('6',64),REPEAT('8',64),NOW(6));
SET @b2=LAST_INSERT_ID();
INSERT INTO chain_event_identity_v2(chain_id,tx_hash,tx_log_ordinal,created_at) VALUES(56,CONCAT('0x',REPEAT('d',64)),0,NOW(6)); SET @i=LAST_INSERT_ID();
INSERT INTO chain_event_placement_v2(event_identity_id,generation,block_number,block_hash,transaction_index,rpc_log_index,source_revision_id,canonical_slot,finalized_at) VALUES(@i,1,101,CONCAT('0x',REPEAT('c',64)),0,0,560002,1,NOW(6)); SET @p=LAST_INSERT_ID();
INSERT INTO chain_event_placement_observation_v2(placement_id,event_identity_id,provider_identity,emitting_address,topics_data_digest,raw_log_digest,observed_at) VALUES(@p,@i,'provider-a','0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',IF(${JC_TEST_FAIR_LAUNCH_MAINNET_MATERIALIZATION_0918:-0}=1,SHA2(UNHEX(REPEAT('e',64)),256),REPEAT('9',64)),IF(${JC_TEST_FAIR_LAUNCH_MAINNET_MATERIALIZATION_0918:-0}=1,SHA2(UNHEX(CONCAT(REPEAT('aa',20),REPEAT('e',64))),256),REPEAT('a',64)),NOW(6)); SET @o=LAST_INSERT_ID();
INSERT INTO fair_launch_mainnet_archive_batch_event_v1(batch_receipt_id,activation_row_id,event_ordinal,event_identity_id,placement_id,observation_id,event_name,payload_digest,row_digest,created_at) VALUES(@b2,@a,0,@i,@p,@o,'FairLaunchProjectCreated',REPEAT('b',64),REPEAT('c',64),NOW(6));
SET @archive_event=LAST_INSERT_ID();
${JC_TEST_FAIR_LAUNCH_MAINNET_MATERIALIZATION_0918:+SET @payload_row_digest=SHA2(CONCAT('joincare:fair-launch:mainnet-archive-payload:v1',CHAR(0),@archive_event,CHAR(0),@a,CHAR(0),@i,CHAR(0),@p,CHAR(0),@o,CHAR(0),1,CHAR(0),CONCAT('0x',REPEAT('e',64)),CHAR(0),'',CHAR(0),'',CHAR(0),'',CHAR(0),'0x'),256); INSERT INTO fair_launch_mainnet_archive_event_payload_v1(archive_event_id,activation_row_id,event_identity_id,placement_id,observation_id,topic_count,topic0,data_hex,payload_row_digest,created_at) SELECT @archive_event,@a,@i,@p,@o,1,CONCAT('0x',REPEAT('e',64)),'0x',@payload_row_digest,NOW(6) WHERE ${JC_TEST_FAIR_LAUNCH_MAINNET_MATERIALIZATION_0918:-0}=1;}
UPDATE fair_launch_mainnet_archive_cursor_v1 SET archived_through_block=101,archived_through_block_hash=CONCAT('0x',REPEAT('c',64)),last_batch_receipt_id=@b2,cursor_generation=3,updated_at=NOW(6) WHERE activation_row_id=@a;
SQL
assert_eq 'receipt-backed cursor' "$(query "SELECT CONCAT(archived_through_block,'/',cursor_generation,'/',(SELECT COUNT(*) FROM fair_launch_mainnet_archive_batch_receipt_v1),'/',(SELECT COUNT(*) FROM fair_launch_mainnet_archive_batch_event_v1)) FROM fair_launch_mainnet_archive_cursor_v1;")" '101/3/2/1'
if [ "${JC_TEST_FAIR_LAUNCH_MAINNET_MATERIALIZATION_0918:-0}" = '1' ]; then
  assert_eq 'archive payload complete' "$(query "SELECT CONCAT((SELECT COUNT(*) FROM fair_launch_mainnet_archive_batch_event_v1),'/',(SELECT COUNT(*) FROM fair_launch_mainnet_archive_event_payload_v1));")" '1/1'
  TEST_PROJECT_ID="$(query 'SELECT MIN(project_id) FROM projects;')"; [ -n "$TEST_PROJECT_ID" ] || fail 'baseline has no project row for materialization bootstrap'
  cat <<SQL | mysql_db
SET @a=(SELECT authority_row_id FROM fair_launch_mainnet_activation_v1 WHERE activation_id='$RUN_ID');
SET @archive_event=(SELECT id FROM fair_launch_mainnet_archive_batch_event_v1 WHERE activation_row_id=@a AND event_name='FairLaunchProjectCreated');
SET @i=(SELECT event_identity_id FROM fair_launch_mainnet_archive_batch_event_v1 WHERE id=@archive_event);
SET @p=(SELECT placement_id FROM fair_launch_mainnet_archive_batch_event_v1 WHERE id=@archive_event);
SET @o=(SELECT observation_id FROM fair_launch_mainnet_archive_batch_event_v1 WHERE id=@archive_event);
INSERT INTO fair_launch_source_revision_binding_v3(run_row_id,chain_id,source_id,source_revision_id,from_block_number,from_transaction_index,from_tx_log_ordinal_inclusive,source_binding_digest,created_at) VALUES(@a,56,560001,560002,100,0,0,REPEAT('1',64),NOW(6)); SET @source_binding=LAST_INSERT_ID();
INSERT INTO fair_launch_mainnet_activation_history_v1(activation_row_id,chain_id,sequence_no,previous_state,state,previous_entry_digest,entry_digest,evidence_digest,created_at) VALUES(@a,56,1,NULL,'prepared',NULL,REPEAT('1',64),REPEAT('2',64),NOW(6)),(@a,56,2,'prepared','admitted',REPEAT('1',64),REPEAT('3',64),REPEAT('4',64),NOW(6)),(@a,56,3,'admitted','source_activated',REPEAT('3',64),REPEAT('5',64),REPEAT('6',64),NOW(6));
INSERT INTO fair_launch_mainnet_component_binding_v1(activation_row_id,chain_id,component,git_commit,artifact_sha256,artifact_size_bytes,build_manifest_digest,sbom_digest,created_at) VALUES(@a,56,'job',REPEAT('1',40),REPEAT('2',64),1,REPEAT('3',64),REPEAT('4',64),NOW(6));
INSERT INTO fair_launch_mainnet_payment_token_v1(activation_row_id,chain_id,token_symbol,token_address,token_decimals,runtime_code_hash,policy_digest,policy_version,max_activation_value_raw,binding_digest,created_at) VALUES(@a,56,'USDT','0x2222222222222222222222222222222222222222',18,REPEAT('7',64),REPEAT('4',64),5,'5000000000000000000',REPEAT('9',64),NOW(6));
INSERT INTO fair_launch_mainnet_project_admission_v1(activation_row_id,chain_id,project_row_id,project_id,payment_token_address,payment_token_decimals,project_token_address,project_token_decimals,project_token_runtime_code_hash,max_total_contribution_raw,max_wallet_contribution_raw,admission_digest,created_at) SELECT @a,56,id,project_id,'0x2222222222222222222222222222222222222222',18,'0x4444444444444444444444444444444444444444',18,REPEAT('4',64),'1000000000000000000','100000000000000000',REPEAT('5',64),NOW(6) FROM projects WHERE project_id=$TEST_PROJECT_ID;
INSERT INTO fair_launch_project_binding_v3(run_row_id,project_row_id,chain_id,contract_address,contract_version,project_id,project_id_raw,source_binding_id,source_id,source_revision_id,project_token_address,project_token_decimals,payment_token_address,payment_token_decimals,binding_digest,created_at) SELECT @a,p.id,56,'0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','fair_launch_v1',p.project_id,CAST(p.project_id AS CHAR),@source_binding,560001,560002,'0x4444444444444444444444444444444444444444',18,'0x2222222222222222222222222222222222222222',18,REPEAT('6',64),NOW(6) FROM projects p WHERE p.project_id=$TEST_PROJECT_ID; SET @project_binding=LAST_INSERT_ID();
INSERT INTO fair_launch_mainnet_project_bootstrap_receipt_v1(activation_row_id,chain_id,project_binding_id,project_id,event_identity_id,placement_id,observation_id,source_id,source_revision_id,placement_generation,effect_manifest_version,payload_digest,binding_digest,initial_state_digest,receipt_digest,created_at) VALUES(@a,56,@project_binding,$TEST_PROJECT_ID,@i,@p,@o,560001,560002,1,1,REPEAT('b',64),REPEAT('6',64),REPEAT('7',64),REPEAT('8',64),NOW(6)); SET @bootstrap_receipt=LAST_INSERT_ID();
INSERT INTO fair_launch_mainnet_materialization_profile_v1(activation_row_id,chain_id,source_id,source_revision_id,source_binding_id,bootstrap_receipt_id,bootstrap_archive_event_id,initial_event_identity_id,effect_manifest_version,effect_manifest_digest,initial_block_number,initial_block_hash,initial_transaction_index,initial_tx_log_ordinal,profile_digest,approval_evidence_digest,valid_from,valid_until,created_at) VALUES(@a,56,560001,560002,@source_binding,@bootstrap_receipt,@archive_event,@i,1,REPEAT('7',64),101,CONCAT('0x',REPEAT('c',64)),0,0,REPEAT('8',64),REPEAT('9',64),NOW(6),DATE_ADD(NOW(6),INTERVAL 1 HOUR),NOW(6));
INSERT INTO source_materialization_cursor(source_id,source_revision_id,materialized_through_block,materialized_through_block_hash,materialized_through_tx_index,materialized_through_tx_log_ordinal,last_event_identity_id,lease_token,fence_epoch,status,last_error,updated_at,cursor_generation) VALUES(560001,560002,101,CONCAT('0x',REPEAT('c',64)),0,0,@i,REPEAT('1',64),1,'active','',NOW(6),1);
SQL
  expect_fail 'materialization cursor admission mismatch' mysql_db <<SQL
SET @a=(SELECT authority_row_id FROM fair_launch_mainnet_activation_v1 WHERE activation_id='$RUN_ID'); SET @source_binding=(SELECT id FROM fair_launch_source_revision_binding_v3 WHERE run_row_id=@a AND source_id=560001); SET @i=(SELECT event_identity_id FROM fair_launch_mainnet_project_bootstrap_receipt_v1 WHERE activation_row_id=@a); SET @archive_event=(SELECT id FROM fair_launch_mainnet_archive_batch_event_v1 WHERE activation_row_id=@a);
INSERT INTO fair_launch_mainnet_materialization_cursor_v1(activation_row_id,chain_id,source_id,source_revision_id,source_binding_id,materialized_through_block,materialized_through_block_hash,materialized_through_tx_index,materialized_through_tx_log_ordinal,last_event_identity_id,last_archive_event_id,lease_generation,fence_epoch,lease_token_hash,status,cursor_generation,created_at,updated_at) VALUES(@a,56,560001,560002,@source_binding,101,CONCAT('0x',REPEAT('c',64)),0,0,@i,@archive_event,1,1,REPEAT('0',64),'active',1,NOW(6),NOW(6));
SQL
  cat <<SQL | mysql_db
SET @a=(SELECT authority_row_id FROM fair_launch_mainnet_activation_v1 WHERE activation_id='$RUN_ID'); SET @source_binding=(SELECT id FROM fair_launch_source_revision_binding_v3 WHERE run_row_id=@a AND source_id=560001); SET @i=(SELECT event_identity_id FROM fair_launch_mainnet_project_bootstrap_receipt_v1 WHERE activation_row_id=@a); SET @archive_event=(SELECT id FROM fair_launch_mainnet_archive_batch_event_v1 WHERE activation_row_id=@a);
INSERT INTO fair_launch_mainnet_materialization_cursor_v1(activation_row_id,chain_id,source_id,source_revision_id,source_binding_id,materialized_through_block,materialized_through_block_hash,materialized_through_tx_index,materialized_through_tx_log_ordinal,last_event_identity_id,last_archive_event_id,lease_generation,fence_epoch,lease_token_hash,status,cursor_generation,created_at,updated_at) VALUES(@a,56,560001,560002,@source_binding,101,CONCAT('0x',REPEAT('c',64)),0,0,@i,@archive_event,1,1,REPEAT('1',64),'active',1,NOW(6),NOW(6));
SQL
  expect_fail 'materialization cursor receipt mismatch' mysql_db <<SQL
UPDATE fair_launch_mainnet_materialization_cursor_v1 SET materialized_through_block=102,cursor_generation=2,updated_at=NOW(6);
SQL
  assert_eq 'materialization authority initialized' "$(query "SELECT CONCAT((SELECT COUNT(*) FROM fair_launch_mainnet_materialization_profile_v1),'/',(SELECT COUNT(*) FROM fair_launch_mainnet_materialization_cursor_v1),'/',(SELECT COUNT(*) FROM fair_launch_mainnet_materialization_receipt_v1));")" '1/1/0'
  echo "fair_launch_mysql8_mainnet_materialization_authority_schema_test: PASS run_id=$RUN_ID container_id=$CONTAINER_ID profile=1 cursor=active/1 payloads=1 receipts=0"
fi
if [ "${JC_TEST_FAIR_LAUNCH_MAINNET_RECOVERY_0916:-0}" = '1' ]; then
  expect_fail 'approved execution gate' mysql_db < "$ROOT/migrations/2026_09_16_fair_launch_mainnet_archive_recovery.sql"
  { echo 'SET @JC_FAIR_LAUNCH_MAINNET_RECOVERY_APPROVED=1;'; cat "$ROOT/migrations/2026_09_16_fair_launch_mainnet_archive_recovery.sql"; } | mysql_db
  assert_eq 'recovery rows inert' "$(query "SELECT CONCAT((SELECT COUNT(*) FROM fair_launch_mainnet_archive_recovery_receipt_v1),'/',(SELECT COUNT(last_recovery_receipt_id) FROM fair_launch_mainnet_archive_cursor_v1));")" '0/0'
  cat <<SQL | mysql_db
SET @a=(SELECT authority_row_id FROM fair_launch_mainnet_activation_v1 WHERE activation_id='$RUN_ID');
INSERT INTO fair_launch_mainnet_archive_dead_letter_v1(activation_row_id,chain_id,source_id,source_revision_id,reason_code,evidence_digest,created_at) VALUES(@a,56,560001,560002,'canonical_conflict',REPEAT('d',64),NOW(6)); SET @d=LAST_INSERT_ID();
UPDATE fair_launch_mainnet_archive_cursor_v1 SET status='reorg_detected',last_error_code='canonical_conflict',last_error_digest=REPEAT('d',64),cursor_generation=4,updated_at=NOW(6) WHERE activation_row_id=@a;
SET @old_lease=(SELECT id FROM fair_launch_mainnet_activation_lease_v1 WHERE activation_row_id=@a AND active_slot=1);
UPDATE fair_launch_mainnet_activation_lease_v1 SET status='revoked',active_slot=NULL,revoked_at=NOW(6) WHERE id=@old_lease;
INSERT INTO fair_launch_mainnet_activation_lease_v1(activation_row_id,chain_id,lease_generation,fence_epoch,lease_token_hash,component_set_digest,approval_evidence_digest,valid_from,valid_until,status,active_slot,created_at) VALUES(@a,56,2,2,REPEAT('2',64),REPEAT('2',64),REPEAT('4',64),NOW(6),DATE_ADD(NOW(6),INTERVAL 1 HOUR),'active',1,NOW(6)); SET @new_lease=LAST_INSERT_ID();
SQL
  expect_fail 'cursor recovery mismatch' mysql_db <<SQL
SET @a=(SELECT authority_row_id FROM fair_launch_mainnet_activation_v1 WHERE activation_id='$RUN_ID');
UPDATE fair_launch_mainnet_archive_cursor_v1 SET status='active',last_error_code=NULL,last_error_digest=NULL,lease_generation=2,fence_epoch=2,lease_token_hash=REPEAT('2',64),cursor_generation=5,updated_at=NOW(6) WHERE activation_row_id=@a;
SQL
  expect_fail 'recovery action mismatch' mysql_db <<SQL
SET @a=(SELECT authority_row_id FROM fair_launch_mainnet_activation_v1 WHERE activation_id='$RUN_ID'); SET @d=(SELECT id FROM fair_launch_mainnet_archive_dead_letter_v1 WHERE activation_row_id=@a AND evidence_digest=REPEAT('d',64)); SET @old_lease=(SELECT id FROM fair_launch_mainnet_activation_lease_v1 WHERE activation_row_id=@a AND lease_generation=1); SET @new_lease=(SELECT id FROM fair_launch_mainnet_activation_lease_v1 WHERE activation_row_id=@a AND lease_generation=2); SET @b=(SELECT last_batch_receipt_id FROM fair_launch_mainnet_archive_cursor_v1 WHERE activation_row_id=@a);
INSERT INTO fair_launch_mainnet_archive_recovery_receipt_v1(activation_row_id,chain_id,source_id,source_revision_id,dead_letter_id,previous_status,reason_code,previous_error_digest,archived_through_block,archived_through_block_hash,last_batch_receipt_id,previous_cursor_generation,previous_lease_id,replacement_lease_id,recovery_action,confirmation_policy,verification_provider_set_digest,verification_count,canonical_chain_digest,approval_evidence_digest,recovery_manifest_digest,receipt_digest,created_at) VALUES(@a,56,560001,560002,@d,'reorg_detected','canonical_conflict',REPEAT('d',64),101,CONCAT('0x',REPEAT('c',64)),@b,4,@old_lease,@new_lease,'canonical_reverified','20_blocks',REPEAT('e',64),1,REPEAT('f',64),REPEAT('1',64),REPEAT('2',64),REPEAT('3',64),NOW(6));
SQL
  cat <<SQL | mysql_db
SET @a=(SELECT authority_row_id FROM fair_launch_mainnet_activation_v1 WHERE activation_id='$RUN_ID'); SET @d=(SELECT id FROM fair_launch_mainnet_archive_dead_letter_v1 WHERE activation_row_id=@a AND evidence_digest=REPEAT('d',64)); SET @old_lease=(SELECT id FROM fair_launch_mainnet_activation_lease_v1 WHERE activation_row_id=@a AND lease_generation=1); SET @new_lease=(SELECT id FROM fair_launch_mainnet_activation_lease_v1 WHERE activation_row_id=@a AND lease_generation=2); SET @b=(SELECT last_batch_receipt_id FROM fair_launch_mainnet_archive_cursor_v1 WHERE activation_row_id=@a);
START TRANSACTION;
INSERT INTO fair_launch_mainnet_archive_recovery_receipt_v1(activation_row_id,chain_id,source_id,source_revision_id,dead_letter_id,previous_status,reason_code,previous_error_digest,archived_through_block,archived_through_block_hash,last_batch_receipt_id,previous_cursor_generation,previous_lease_id,replacement_lease_id,recovery_action,confirmation_policy,verification_provider_set_digest,verification_count,canonical_chain_digest,approval_evidence_digest,recovery_manifest_digest,receipt_digest,created_at) VALUES(@a,56,560001,560002,@d,'reorg_detected','canonical_conflict',REPEAT('d',64),101,CONCAT('0x',REPEAT('c',64)),@b,4,@old_lease,@new_lease,'canonical_reverified','20_blocks',REPEAT('e',64),2,REPEAT('f',64),REPEAT('1',64),REPEAT('2',64),REPEAT('3',64),NOW(6)); SET @r=LAST_INSERT_ID();
UPDATE fair_launch_mainnet_archive_cursor_v1 SET status='active',last_error_code=NULL,last_error_digest=NULL,lease_generation=2,fence_epoch=2,lease_token_hash=REPEAT('2',64),last_recovery_receipt_id=@r,cursor_generation=5,updated_at=NOW(6) WHERE activation_row_id=@a;
COMMIT;
SQL
  assert_eq 'receipt-backed recovery' "$(query "SELECT CONCAT(c.status,'/',c.cursor_generation,'/',c.lease_generation,'/',c.fence_epoch,'/',COUNT(r.id)) FROM fair_launch_mainnet_archive_cursor_v1 c LEFT JOIN fair_launch_mainnet_archive_recovery_receipt_v1 r ON r.id=c.last_recovery_receipt_id AND r.activation_row_id=c.activation_row_id GROUP BY c.activation_row_id;")" 'active/5/2/2/1'
  expect_fail 'is append-only' mysql_db <<SQL
UPDATE fair_launch_mainnet_archive_recovery_receipt_v1 SET verification_count=3;
SQL
  if [ "${JC_TEST_FAIR_LAUNCH_MAINNET_RECOVERY_PROVIDER_0917:-0}" = '1' ]; then
    expect_fail 'approved execution gate' mysql_db < "$ROOT/migrations/2026_09_17_fair_launch_mainnet_recovery_provider.sql"
    { echo 'SET @JC_FAIR_LAUNCH_MAINNET_RECOVERY_PROVIDER_APPROVED=1;'; cat "$ROOT/migrations/2026_09_17_fair_launch_mainnet_recovery_provider.sql"; } | mysql_db
    assert_eq 'provider rows inert' "$(query 'SELECT COUNT(*) FROM fair_launch_mainnet_recovery_provider_v1;')" '0'
    INDEPENDENT_ID='provider-b'; INDEPENDENT_DIGEST="$(printf 'joincare:fair-launch:mainnet-rpc:v1\0%s' "$INDEPENDENT_ID" | sha256sum | awk '{print $1}')"
    expect_fail 'identity digest mismatch' mysql_db <<SQL
SET @a=(SELECT authority_row_id FROM fair_launch_mainnet_activation_v1 WHERE activation_id='$RUN_ID');
INSERT INTO fair_launch_mainnet_recovery_provider_v1(activation_row_id,chain_id,source_id,source_revision_id,provider_identity,rpc_identity_digest,provider_role,admission_manifest_digest,approval_evidence_digest,valid_from,valid_until,created_at) VALUES(@a,56,560001,560002,'provider-b',REPEAT('0',64),'independent',REPEAT('1',64),REPEAT('2',64),NOW(6),DATE_ADD(NOW(6),INTERVAL 1 HOUR),NOW(6));
SQL
    mysql_db <<SQL
SET @a=(SELECT authority_row_id FROM fair_launch_mainnet_activation_v1 WHERE activation_id='$RUN_ID');
INSERT INTO fair_launch_mainnet_recovery_provider_v1(activation_row_id,chain_id,source_id,source_revision_id,provider_identity,rpc_identity_digest,provider_role,admission_manifest_digest,approval_evidence_digest,valid_from,valid_until,created_at) VALUES(@a,56,560001,560002,'provider-b','$INDEPENDENT_DIGEST','independent',REPEAT('1',64),REPEAT('2',64),NOW(6),DATE_ADD(NOW(6),INTERVAL 1 HOUR),NOW(6));
SQL
    assert_eq 'provider admission count' "$(query 'SELECT COUNT(*) FROM fair_launch_mainnet_recovery_provider_v1;')" '1'
    echo "fair_launch_mysql8_mainnet_recovery_provider_schema_test: PASS run_id=$RUN_ID container_id=$CONTAINER_ID providers=1"
  fi
  echo "fair_launch_mysql8_mainnet_archive_recovery_schema_test: PASS run_id=$RUN_ID container_id=$CONTAINER_ID cursor=active/5 lease=2/2 recoveries=1"
else
  echo "fair_launch_mysql8_mainnet_archive_scanner_schema_test: PASS run_id=$RUN_ID container_id=$CONTAINER_ID cursor=101/3 batches=2 events=1"
fi
