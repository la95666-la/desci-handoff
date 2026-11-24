#!/usr/bin/env bash
set -euo pipefail
# TEST ONLY. Builds a disposable MySQL 8.4 schema and runs the Backend's
# ProjectCreated bootstrap atomicity test over container bridge connectivity.
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BACKEND="$(cd "$ROOT/../joincare_code/joincare_code/_extracted/joincare_server-master" && pwd)"
IMAGE='mysql@sha256:c831a0f11348d402b43d77453e17d770be2eef356615a2823fe0f5a0d6c8b9af'
SOCKET='/var/run/docker.sock'; RUN_ID="$(uuidgen | tr '[:upper:]' '[:lower:]')"; RUN_HEX="${RUN_ID//-/}"
NAME="jc-fl-mainnet-bootstrap-$RUN_HEX"; LABEL_KEY='joincare.fair-launch.mainnet-bootstrap-test'
DB="joincare_fl_bootstrap_${RUN_HEX:0:16}"; SECRET_DIR=''; CONTAINER_ID=''; CONTAINER_IP=''
fail(){ echo "fair_launch_mysql8_mainnet_bootstrap_transaction_test: FAIL: $*" >&2; exit 1; }
docker_local(){ docker -H "unix://$SOCKET" "$@"; }
cleanup(){ if [ -n "$CONTAINER_ID" ]; then observed="$(docker_local inspect --format "{{index .Config.Labels \"$LABEL_KEY\"}}" "$CONTAINER_ID" 2>/dev/null || true)"; [ "$observed" != "$RUN_ID" ] || docker_local rm -f "$CONTAINER_ID" >/dev/null 2>&1 || true; fi; [ -z "$SECRET_DIR" ] || rm -rf "$SECRET_DIR"; }
for forbidden in JC_DEPLOY_RPC JC_DEPLOY_CHAIN_ID JC_DEPLOY_PRIVATE_KEY JC_MYSQL_ROOT_PASSWORD MYSQL_PWD FAIR_LAUNCH_DISPOSABLE_MYSQL_DSN JOINCARE_FAIR_LAUNCH_BOOTSTRAP_MYSQL_TEST_DSN JOINCARE_FAIR_LAUNCH_BOOTSTRAP_MYSQL_TEST_DSN_FILE DOCKER_HOST DOCKER_CONTEXT DOCKER_TLS_VERIFY DOCKER_CERT_PATH; do [ -z "${!forbidden:-}" ] || fail "refusing external/production environment: $forbidden"; done
command -v docker >/dev/null || fail 'docker required'; command -v openssl >/dev/null || fail 'openssl required'; command -v uuidgen >/dev/null || fail 'uuidgen required'; command -v go >/dev/null || fail 'Go required'; [ -S "$SOCKET" ] || fail 'local Docker socket unavailable'; trap cleanup EXIT INT TERM
docker_local image inspect "$IMAGE" >/dev/null 2>&1 || fail 'pinned MySQL image unavailable locally'
umask 077; SECRET_DIR="$(mktemp -d)"; openssl rand -hex 32 > "$SECRET_DIR/root"; openssl rand -hex 32 > "$SECRET_DIR/test"; chmod 600 "$SECRET_DIR/root" "$SECRET_DIR/test"
CONTAINER_ID="$(docker_local run -d --name "$NAME" --label "$LABEL_KEY=$RUN_ID" --network bridge --tmpfs /var/lib/mysql:rw,nosuid,nodev,size=2g --tmpfs /tmp:rw,nosuid,nodev,size=256m -v "$SECRET_DIR/root:/run/secrets/root:ro" -v "$ROOT:/repo:ro" -e MYSQL_ROOT_PASSWORD_FILE=/run/secrets/root "$IMAGE" --skip-log-bin --local-infile=OFF --secure-file-priv=NULL)"
[ "$(docker_local inspect --format "{{index .Config.Labels \"$LABEL_KEY\"}}" "$CONTAINER_ID")" = "$RUN_ID" ] || fail 'container ownership mismatch'
CONTAINER_IP="$(docker_local inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$CONTAINER_ID")"; [[ "$CONTAINER_IP" =~ ^172[.][0-9]+[.][0-9]+[.][0-9]+$ ]] || fail 'disposable bridge address unavailable'
docker_local exec "$CONTAINER_ID" sh -c 'umask 077; { echo "[client]"; echo "user=root"; printf "password="; cat /run/secrets/root; echo "host=127.0.0.1"; echo "protocol=tcp"; } >/tmp/root.cnf; chmod 600 /tmp/root.cnf'
for _ in $(seq 1 90); do docker_local exec "$CONTAINER_ID" mysqladmin --defaults-extra-file=/tmp/root.cnf ping --silent >/dev/null 2>&1 && break; sleep 1; done
docker_local exec "$CONTAINER_ID" mysqladmin --defaults-extra-file=/tmp/root.cnf ping --silent >/dev/null || fail 'MySQL not ready'
mysql_root(){ docker_local exec -i "$CONTAINER_ID" mysql --defaults-extra-file=/tmp/root.cnf --batch --raw --skip-column-names "$@"; }; mysql_db(){ mysql_root "$DB"; }; query(){ printf '%s\n' "$1" | mysql_db | tr -d '\r'; }; assert_eq(){ [ "$2" = "$3" ] || fail "$1 got=$2 want=$3"; }
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
assert_eq 'migration activation rows' "$(query 'SELECT COUNT(*) FROM fair_launch_mainnet_activation_v1;')" '0'
assert_eq 'migration bootstrap receipt rows' "$(query 'SELECT COUNT(*) FROM fair_launch_mainnet_project_bootstrap_receipt_v1;')" '0'
assert_eq 'bootstrap receipt table' "$(query "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_mainnet_project_bootstrap_receipt_v1';")" '1'
assert_eq 'project state receipt FK' "$(query "SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_NAME='fk_fl_project_state_bootstrap_receipt';")" '1'
assert_eq 'mainnet bootstrap state check' "$(query "SELECT COUNT(*) FROM information_schema.CHECK_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_NAME='chk_fl_project_state' AND CHECK_CLAUSE LIKE '%bootstrap_receipt_id%' AND CHECK_CLAUSE LIKE '%31337%' AND CHECK_CLAUSE LIKE '%56%';")" '1'
TEST_PASSWORD="$(cat "$SECRET_DIR/test")"
cat <<SQL | mysql_db
INSERT INTO chain_contract_source(id,chain_id,contract_address,contract_version,funding_mode,registered_at,start_block,enabled) VALUES(560001,56,'0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','fair_launch_v1','fair_launch',NOW(6),90,1);
INSERT INTO chain_contract_source_revision(id,source_id,revision_no,activation_block_hash,proxy_kind,abi_digest,confirmation_policy,governance_tx_hash,governance_operation_id,implementation_address,implementation_code_hash,dependency_manifest_digest,abi_version,from_block_number,from_transaction_index,from_tx_log_ordinal_inclusive,created_at) VALUES(560002,560001,1,CONCAT('0x',REPEAT('1',64)),'uups',REPEAT('2',64),'20_blocks',CONCAT('0x',REPEAT('3',64)),CONCAT('0x',REPEAT('4',64)),'0x2222222222222222222222222222222222222222',REPEAT('5',64),REPEAT('6',64),'fair-launch-v1',90,0,0,NOW(6));
INSERT INTO fair_launch_mainnet_activation_v1(activation_id,environment,chain_id,rpc_genesis_hash,rpc_identity_digest,proxy_address,proxy_runtime_code_hash,implementation_address,implementation_code_hash,abi_digest,source_id,source_revision_id,source_start_block,source_start_block_hash,source_registry_digest,payment_token_allowlist_digest,project_allowlist_digest,release_manifest_digest,migration_manifest_digest,activation_manifest_digest,signing_key_id,signature_digest,governance_evidence_digest,valid_from,valid_until,created_at) VALUES('$RUN_ID','test-server',56,CONCAT('0x',REPEAT('0',64)),REPEAT('1',64),'0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',REPEAT('7',64),'0x2222222222222222222222222222222222222222',REPEAT('5',64),REPEAT('2',64),560001,560002,90,CONCAT('0x',REPEAT('1',64)),REPEAT('8',64),REPEAT('9',64),REPEAT('a',64),REPEAT('b',64),REPEAT('c',64),REPEAT('d',64),'test-key-1',REPEAT('e',64),REPEAT('f',64),NOW(6),DATE_ADD(NOW(6),INTERVAL 1 HOUR),NOW(6));
SET @activation=(SELECT authority_row_id FROM fair_launch_mainnet_activation_v1 WHERE activation_id='$RUN_ID');
INSERT INTO fair_launch_source_revision_binding_v3(run_row_id,chain_id,source_id,source_revision_id,from_block_number,from_transaction_index,from_tx_log_ordinal_inclusive,source_binding_digest,created_at) VALUES(@activation,56,560001,560002,90,0,0,REPEAT('1',64),NOW(6));
INSERT INTO fair_launch_mainnet_payment_token_v1(activation_row_id,chain_id,token_symbol,token_address,token_decimals,runtime_code_hash,policy_digest,policy_version,max_activation_value_raw,binding_digest,created_at) VALUES(@activation,56,'USDT','0x2222222222222222222222222222222222222222',18,REPEAT('7',64),REPEAT('4',64),5,'5000000000000000000',REPEAT('9',64),NOW(6));
INSERT INTO fair_launch_source_token_binding_v3(run_row_id,chain_id,source_id,contract_address,token_address,token_decimals,metadata_version,admission_generation,runtime_code_hash,metadata_digest,admitted_at) VALUES(@activation,56,560001,'0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','0x2222222222222222222222222222222222222222',18,1,1,REPEAT('7',64),REPEAT('8',64),NOW(6));
CREATE USER 'fl_bootstrap_test'@'%' IDENTIFIED BY '$TEST_PASSWORD';
SET @grant_sql=CONCAT('GRANT SELECT,INSERT,UPDATE,DELETE,CREATE,DROP,TRIGGER ON ',DATABASE(),'.* TO ''fl_bootstrap_test''@''%''');
PREPARE grant_statement FROM @grant_sql; EXECUTE grant_statement; DEALLOCATE PREPARE grant_statement;
FLUSH PRIVILEGES;
SQL
printf 'fl_bootstrap_test:%s@tcp(%s:3306)/%s?charset=utf8mb4&parseTime=True&loc=UTC\n' "$TEST_PASSWORD" "$CONTAINER_IP" "$DB" > "$SECRET_DIR/test-dsn"
chmod 600 "$SECRET_DIR/test-dsn"
(cd "$BACKEND" && JOINCARE_FAIR_LAUNCH_BOOTSTRAP_MYSQL_TEST_DSN_FILE="$SECRET_DIR/test-dsn" GOTOOLCHAIN=auto GOEXPERIMENT=jsonv2 go test ./pkg/job/job/chainevent -run '^TestFairLaunchMainnetProjectCreatedMySQLAtomicity$' -count=1 -v)
echo "fair_launch_mysql8_mainnet_bootstrap_transaction_test: PASS run_id=$RUN_ID container_id=$CONTAINER_ID"
