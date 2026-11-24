#!/usr/bin/env bash
# Disposable, network-none MySQL rehearsal of the exact candidate triggers.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd -P)"
docker image inspect mysql:8.4.10 >/dev/null
name="jc-sender-isolation-test-$$"
temp="$(mktemp -d)"
container=''
cleanup() {
  if [ -n "$container" ]; then docker rm -f "$container" >/dev/null 2>&1 || :; fi
  rm -rf "$temp"
}
trap cleanup EXIT
umask 077
openssl rand -hex 32 > "$temp/password"
container="$(docker run -d --name "$name" --label joincare.sender-isolation-test=true --network none --tmpfs /var/lib/mysql:rw,nosuid,nodev,size=1g -v "$temp/password:/run/secrets/mysql-password:ro" -e MYSQL_ROOT_PASSWORD_FILE=/run/secrets/mysql-password mysql:8.4.10 --skip-log-bin)"
for _ in $(seq 1 90); do
 if docker exec "$container" sh -c 'MYSQL_PWD="$(cat /run/secrets/mysql-password)" mysql --protocol=tcp -h127.0.0.1 -uroot -N -e "SELECT 1"' >/dev/null 2>&1; then break; fi
 sleep 1
done
db() { docker exec -i "$container" sh -c 'MYSQL_PWD="$(cat /run/secrets/mysql-password)" exec mysql --protocol=tcp -h127.0.0.1 -uroot --batch --skip-column-names "$@"' sh "$@"; }
db <<'SQL'
CREATE DATABASE jc_v3_sender_isolation_test;
USE jc_v3_sender_isolation_test;
CREATE TABLE chain_tx_submission(id BIGINT UNSIGNED PRIMARY KEY,chain_id BIGINT UNSIGNED,sender_address VARCHAR(42),project_id BIGINT UNSIGNED,state VARCHAR(32),active_slot TINYINT NULL,nonce BIGINT UNSIGNED NULL);
CREATE TABLE chain_relayer_nonce(chain_id BIGINT UNSIGNED,sender_address VARCHAR(42),next_nonce BIGINT UNSIGNED,active_submission_id BIGINT UNSIGNED NULL,PRIMARY KEY(chain_id,sender_address));
CREATE TRIGGER jc_ido_v3_nonce_insert_1020 BEFORE INSERT ON chain_relayer_nonce FOR EACH ROW SET @fixture=1;
CREATE TRIGGER jc_ido_v3_nonce_update_1020 BEFORE UPDATE ON chain_relayer_nonce FOR EACH ROW SET @fixture=1;
SQL
if cat "$root/pkg/common/chaintx/v3_sender_isolation.sql" | db jc_v3_sender_isolation_test > "$temp/unapproved.log" 2>&1; then echo 'unguarded migration accepted';exit 1;fi
grep -q 'sender isolation migration approval required' "$temp/unapproved.log"
{ echo 'SET @JC_V3_SENDER_ISOLATION_APPROVED=1;'; cat "$root/pkg/common/chaintx/v3_sender_isolation.sql"; } | db jc_v3_sender_isolation_test
{ echo 'SET @JC_V3_SENDER_ISOLATION_APPROVED=1;'; cat "$root/pkg/common/chaintx/v3_sender_isolation.sql"; } | db jc_v3_sender_isolation_test
printf '%s\n' "SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA='jc_v3_sender_isolation_test' AND TRIGGER_NAME IN ('jc_ido_v3_nonce_insert_1020','jc_ido_v3_nonce_update_1020') AND ACTION_STATEMENT LIKE '%V3 nonce isolation v1%';" | db | grep -x 2
sql() { printf '%s\n' "$1" | db jc_v3_sender_isolation_test; }
reject() {
 if sql "$1" > "$temp/reject.log" 2>&1; then echo "unsafe transition accepted: $2";exit 1;fi
 grep -q 'V3 unresolved signature fences shared sender' "$temp/reject.log"
 echo "rejected: $2"
}
sender='0x1111111111111111111111111111111111111111'
for i in $(seq 1 46); do sql "INSERT INTO chain_tx_submission VALUES($i,56,'$sender',$i,'v3_reorg_suspended',NULL,$i);"; done
sql "INSERT INTO chain_relayer_nonce VALUES(56,'$sender',85,NULL); INSERT INTO chain_tx_submission VALUES(100,56,'$sender',999,'signed',1,85); UPDATE chain_relayer_nonce SET next_nonce=86,active_submission_id=100 WHERE chain_id=56;"
sql "SELECT COUNT(*) FROM chain_tx_submission WHERE state='v3_reorg_suspended';" | grep -x 46
echo 'PASS: 46 consumed historical projects do not block new project'
sql "INSERT INTO chain_tx_submission VALUES(101,56,'$sender',999,'signed',1,86);"
reject "UPDATE chain_relayer_nonce SET next_nonce=87,active_submission_id=101 WHERE chain_id=56;" 'existing active sender slot'
sql "UPDATE chain_tx_submission SET state='applied',active_slot=NULL WHERE id=100; UPDATE chain_relayer_nonce SET active_submission_id=NULL WHERE chain_id=56; INSERT INTO chain_tx_submission VALUES(200,56,'$sender',1,'v3_reorg_suspended',NULL,86);"
reject "UPDATE chain_relayer_nonce SET next_nonce=87,active_submission_id=101 WHERE chain_id=56;" 'unconsumed same nonce'
sql 'UPDATE chain_tx_submission SET nonce=NULL WHERE id=200;'
reject "UPDATE chain_relayer_nonce SET next_nonce=87,active_submission_id=101 WHERE chain_id=56;" 'unknown historical nonce'
sql 'UPDATE chain_tx_submission SET nonce=90 WHERE id=200;'
reject "UPDATE chain_relayer_nonce SET next_nonce=87,active_submission_id=101 WHERE chain_id=56;" 'unconsumed future nonce'
sql 'UPDATE chain_tx_submission SET nonce=47 WHERE id=200;'
reject "UPDATE chain_relayer_nonce SET next_nonce=88,active_submission_id=101 WHERE chain_id=56;" 'nonce drift'
sql "INSERT INTO chain_tx_submission VALUES(201,56,'$sender',2,'signed',1,87);"
reject "UPDATE chain_relayer_nonce SET next_nonce=87,active_submission_id=101 WHERE chain_id=56;" 'other pending signed original'
sql 'DELETE FROM chain_tx_submission WHERE id=201;'
sql "UPDATE chain_relayer_nonce SET next_nonce=87,active_submission_id=101 WHERE chain_id=56;"
sql "INSERT INTO chain_tx_submission VALUES(300,57,'$sender',999,'signed',1,5); INSERT INTO chain_relayer_nonce VALUES(57,'$sender',6,300);"
reject "INSERT INTO chain_relayer_nonce VALUES(58,'$sender',6,300);" 'wrong chain binding'
echo 'PASS: exact trigger SQL, guarded/idempotent install, shared nonce fences; no production database used'
