#!/usr/bin/env bash
# Local-only schema contract test. Never points at a shared or production DB.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
NAME="jc-ido-outbox-schema-$$"
LABEL="joincare.ido-outbox-schema-test=$NAME"
IMAGE='mysql:8.4.10'
DB='jc_ido_outbox_schema_it'
CID=''
REDIS_CID=''
HARDHAT_CID=''
TMP=''
fail() { echo "ido outbox schema disposable: $*" >&2; exit 1; }
cleanup() {
  if [ -n "$HARDHAT_CID" ] && [ "$(docker_local inspect --format '{{index .Config.Labels "joincare.ido-outbox-schema-test"}}' "$HARDHAT_CID" 2>/dev/null || true)" = "$NAME" ]; then
    docker_local rm -f "$HARDHAT_CID" >/dev/null 2>&1 || true
  fi
  if [ -n "$REDIS_CID" ] && [ "$(docker_local inspect --format '{{index .Config.Labels "joincare.ido-outbox-schema-test"}}' "$REDIS_CID" 2>/dev/null || true)" = "$NAME" ]; then
    docker_local rm -f "$REDIS_CID" >/dev/null 2>&1 || true
  fi
  if [ -n "$CID" ] && [ "$(docker_local inspect --format '{{index .Config.Labels "joincare.ido-outbox-schema-test"}}' "$CID" 2>/dev/null || true)" = "$NAME" ]; then
    docker rm -f "$CID" >/dev/null 2>&1 || true
  fi
  [ -z "$TMP" ] || rm -rf "$TMP"
}
trap cleanup EXIT INT TERM
for forbidden in DOCKER_HOST DOCKER_CONTEXT DOCKER_TLS_VERIFY JC_DEPLOY_RPC JC_DEPLOY_PRIVATE_KEY MYSQL_PWD JC_IDO_FULL_SCHEMA_DSN; do
  [ -z "${!forbidden:-}" ] || fail "external environment forbidden: $forbidden"
done
if [ "${JC_IDO_FULL_SCHEMA_CRON_TEST:-0}" = 1 ] && { [ "${JC_IDO_FULL_SCHEMA_GO_TEST:-0}" != 1 ] || [ "${JC_IDO_FULL_SCHEMA_REDIS_TEST:-0}" != 1 ]; }; then
  fail 'scanner cron test requires both disposable Go and Redis tests'
fi
if [ "${JC_IDO_FULL_SCHEMA_REDIS_TEST:-0}" = 1 ] && [ "${JC_IDO_FULL_SCHEMA_GO_TEST:-0}" != 1 ]; then
  fail 'scanner Redis test requires disposable Go test binary'
fi
if [ "${JC_IDO_FULL_SCHEMA_HARDHAT_TEST:-0}" = 1 ] && { [ "${JC_IDO_FULL_SCHEMA_GO_TEST:-0}" != 1 ] || [ "${JC_IDO_FULL_SCHEMA_REDIS_TEST:-0}" != 1 ]; }; then
  fail 'full schema Hardhat end/scanner requires disposable Go and Redis tests'
fi
command -v docker >/dev/null && command -v openssl >/dev/null && command -v sha256sum >/dev/null || fail 'docker, openssl and sha256sum required'
[ -S /var/run/docker.sock ] && [ "$(readlink -f /var/run/docker.sock)" = /run/docker.sock ] || fail 'expected local Docker socket unavailable'
docker -H unix:///var/run/docker.sock image inspect "$IMAGE" >/dev/null 2>&1 || fail 'pinned local MySQL image unavailable'
docker_local() { docker -H unix:///var/run/docker.sock "$@"; }
(cd "$ROOT/migrations/baseline" && sha256sum -c fair-launch-pre-2026-09-03.sql.sha256 >/dev/null) || fail 'baseline identity mismatch'
(cd "$ROOT/migrations" && sha256sum -c baseline/fair-launch-pre-2026-09-03-ledger.sha256 >/dev/null) || fail 'pinned ledger source mismatch'
(cd "$ROOT/migrations" && sha256sum -c FAIR_LAUNCH_SHA256SUMS >/dev/null) || fail 'Fair Launch migration identity mismatch'
umask 077
TMP="$(mktemp -d)"
openssl rand -hex 32 > "$TMP/mysql-password"
CID="$(docker_local run -d --name "$NAME" --label "$LABEL" --network none --tmpfs /var/lib/mysql:rw,nosuid,nodev,size=2g --tmpfs /tmp:rw,nosuid,nodev,size=256m -v "$TMP/mysql-password:/run/secrets/jc-ido-schema-password:ro" -e MYSQL_ROOT_PASSWORD_FILE=/run/secrets/jc-ido-schema-password "$IMAGE" --skip-log-bin --local-infile=OFF)"
mysql_db() {
  docker_local exec -i "$CID" sh -c 'MYSQL_PWD="$(cat /run/secrets/jc-ido-schema-password)" exec mysql --protocol=tcp -h127.0.0.1 -uroot --batch --skip-column-names "$@"' sh "$DB"
}
for i in $(seq 1 90); do
  if docker_local exec "$CID" sh -c 'MYSQL_PWD="$(cat /run/secrets/jc-ido-schema-password)" exec mysqladmin --protocol=tcp -h127.0.0.1 -uroot ping --silent' >/dev/null 2>&1; then break; fi
  sleep 1
done
printf 'CREATE DATABASE `%s`; USE `%s`; CREATE TABLE schema_migrations(filename VARCHAR(255) NOT NULL PRIMARY KEY, checksum CHAR(64) NOT NULL, status VARCHAR(16) NOT NULL DEFAULT '\''applied'\'', applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)) ENGINE=InnoDB;\n' "$DB" "$DB" | docker_local exec -i "$CID" sh -c 'MYSQL_PWD="$(cat /run/secrets/jc-ido-schema-password)" exec mysql --protocol=tcp -h127.0.0.1 -uroot' || fail 'create disposable database'
while read -r checksum marked; do
  file="${marked#\*}"
  [[ "$checksum" =~ ^[0-9a-f]{64}$ && "$file" =~ ^[0-9A-Za-z_]+\.sql$ ]] || fail 'invalid pinned ledger'
  printf "INSERT INTO schema_migrations(filename,checksum,status) VALUES ('%s','%s','applied');\n" "$file" "$checksum" | mysql_db
done < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03-ledger.sha256"
mysql_db < "$ROOT/migrations/baseline/fair-launch-pre-2026-09-03.sql" > "$TMP/baseline.log" || { cat "$TMP/baseline.log" >&2; fail 'baseline load'; }
for file in \
  2026_09_03_fair_launch_project_model.sql \
  2026_09_04_fair_launch_ce_v2_governance.sql \
  2026_09_05_fair_launch_signing_release_control.sql \
  2026_09_06_fair_launch_action_execution.sql \
  2026_09_07_fair_launch_position_projection.sql; do
  if mysql_db < "$ROOT/migrations/$file" > "$TMP/error" 2>&1; then fail "$file accepted missing migration gate"; fi
  grep -Fq 'requires approved execution gate' "$TMP/error" || { cat "$TMP/error" >&2; fail "$file rejected for wrong reason"; }
  { echo 'SET @JC_FAIR_LAUNCH_MIGRATION_APPROVED=1;'; cat "$ROOT/migrations/$file"; } | mysql_db > "$TMP/migration.log" || { cat "$TMP/migration.log" >&2; fail "could not apply $file"; }
  checksum="$(sha256sum "$ROOT/migrations/$file" | cut -d' ' -f1)"
  printf "INSERT INTO schema_migrations(filename,checksum,status) VALUES ('%s','%s','applied');\n" "$file" "$checksum" | mysql_db || fail "could not record disposable $file ledger"
done
echo 'post-0907 project and CE-V2 schema loaded in disposable database'
INSERT="INSERT INTO chain_tx_submission (project_id,action,attempt_no,chain_id,sender_address,contract_address,call_data,value_raw,state,active_slot,unsigned_tx_digest,signing_digest,replacement_generation,signing_recovery_reason,last_error) VALUES (9,'project_end',1,31337,'0x1111111111111111111111111111111111111111','0x2222222222222222222222222222222222222222',X'01','0','intent',1,'','',0,'','');"
if printf '%s\n' "$INSERT" | mysql_db > "$TMP/error" 2>&1; then fail 'old schema unexpectedly accepted current outbox INSERT'; fi
grep -Fq "Unknown column 'unsigned_tx_digest'" "$TMP/error" || { cat "$TMP/error" >&2; fail 'wrong baseline failure'; }
MIGRATION="$ROOT/migrations/2026_10_02_chain_tx_submission_signing_metadata.sql"
if mysql_db < "$MIGRATION" > "$TMP/error" 2>&1; then fail 'migration accepted missing execution gate'; fi
grep -Fq 'chain tx signing metadata migration requires approved execution gate' "$TMP/error" || { cat "$TMP/error" >&2; fail 'wrong gate failure'; }
for _ in 1 2; do
  { echo 'SET @JC_CHAIN_TX_SIGNING_METADATA_APPROVED=1;'; cat "$MIGRATION"; } | mysql_db > "$TMP/migration.log" || { cat "$TMP/migration.log" >&2; fail 'migration apply/retry'; }
done
CHECKSUM="$(sha256sum "$MIGRATION" | awk '{print $1}')"
printf "INSERT INTO schema_migrations(filename,checksum,status) VALUES ('2026_10_02_chain_tx_submission_signing_metadata.sql','%s','applied');\n" "$CHECKSUM" | mysql_db
printf '%s\n' "$INSERT" | mysql_db || fail 'current outbox INSERT failed on migrated schema'
[ "$(printf 'SELECT state,active_slot FROM chain_tx_submission WHERE project_id=9;\n' | mysql_db)" = $'intent\t1' ] || fail 'outbox row mismatch'
printf "DELETE FROM chain_tx_submission WHERE project_id=9 AND action='project_end';\n" | mysql_db
if [ "${JC_IDO_FULL_SCHEMA_GO_TEST:-0}" = 1 ]; then
  TEST_BINARY="${JC_IDO_FULL_SCHEMA_TEST_BINARY:-}"
  [ -f "$TEST_BINARY" ] && [ -x "$TEST_BINARY" ] || fail 'provide the locally built Linux Go test binary via JC_IDO_FULL_SCHEMA_TEST_BINARY'
  docker_local cp "$TEST_BINARY" "$CID:/usr/local/bin/jc-chainevent-fullschema.test" || fail 'copy local Go test binary into disposable container'
  [ "$(docker_local exec "$CID" sha256sum /usr/local/bin/jc-chainevent-fullschema.test | cut -d' ' -f1)" = "$(sha256sum "$TEST_BINARY" | cut -d' ' -f1)" ] || fail 'copied test binary digest mismatch'
  # The Go test runs inside the network-none MySQL container. Root password is
  # sourced from its mounted secret; neither database nor RPC leaves this namespace.
  docker_local exec "$CID" test -x /usr/local/bin/jc-chainevent-fullschema.test || fail 'copied test binary missing'
  docker_local exec "$CID" sh -c 'MYSQL_PWD="$(cat /run/secrets/jc-ido-schema-password)"; JC_IDO_FULL_SCHEMA_DSN="root:${MYSQL_PWD}@tcp(127.0.0.1:3306)/jc_ido_outbox_schema_it?parseTime=true&loc=UTC"; export JC_IDO_FULL_SCHEMA_DSN; exec /usr/local/bin/jc-chainevent-fullschema.test -test.run "^(TestIDOV2JobStartupSchemaGatesOnFullMigratedMySQL|TestIDOV2EndReservesOnFullMigratedMySQL|TestIDOV2SyncLeaseAndAlertOnFullMigratedMySQL|TestV2NonMaterialRetrySharesScannerBatchTransactionOnFullSchema)$" -test.v' || fail 'real Go outbox or scanner DB health on full schema failed'
  if [ "${JC_IDO_FULL_SCHEMA_REDIS_TEST:-0}" = 1 ]; then
    REDIS_IMAGE='redis:7.4-alpine'
    docker_local image inspect "$REDIS_IMAGE" >/dev/null 2>&1 || fail 'pinned local Redis image unavailable'
    REDIS_CID="$(docker_local run -d --name "$NAME-redis" --label "$LABEL" --network "container:$CID" --user redis --cap-drop ALL --read-only --tmpfs /data:rw,nosuid,nodev,size=32m,uid=999,gid=999 "$REDIS_IMAGE" redis-server --port 16379 --bind 127.0.0.1 --cluster-enabled yes --cluster-config-file /data/nodes.conf --appendonly no --save '' --protected-mode no)" || fail 'start disposable Redis'
    for _ in $(seq 1 30); do
      if docker_local exec "$REDIS_CID" redis-cli -h 127.0.0.1 -p 16379 ping 2>/dev/null | grep -Fq PONG; then break; fi
      sleep 1
    done
    docker_local exec "$REDIS_CID" redis-cli -h 127.0.0.1 -p 16379 cluster addslots $(seq 0 16383) >/dev/null || fail 'assign isolated Redis cluster slots'
    for _ in $(seq 1 30); do
      if [ "$(docker_local exec "$REDIS_CID" redis-cli -h 127.0.0.1 -p 16379 cluster info | sed -n 's/^cluster_state:\(.*\)/\1/p' | tr -d '\r')" = ok ]; then break; fi
      sleep 1
    done
    [ "$(docker_local exec "$REDIS_CID" redis-cli -h 127.0.0.1 -p 16379 cluster info | sed -n 's/^cluster_state:\(.*\)/\1/p' | tr -d '\r')" = ok ] || fail 'isolated Redis cluster did not become ready'
    docker_local exec "$CID" sh -c 'MYSQL_PWD="$(cat /run/secrets/jc-ido-schema-password)"; JC_IDO_FULL_SCHEMA_DSN="root:${MYSQL_PWD}@tcp(127.0.0.1:3306)/jc_ido_outbox_schema_it?parseTime=true&loc=UTC"; JC_IDO_FULL_SCHEMA_REDIS="127.0.0.1:16379"; export JC_IDO_FULL_SCHEMA_DSN JC_IDO_FULL_SCHEMA_REDIS; exec /usr/local/bin/jc-chainevent-fullschema.test -test.run "^TestIDOV2ScannerRunPersistsRPCHeadFailureOnFullSchema$" -test.v' || fail 'actual Redis/DB scanner failure path did not fail closed'
    for stage in first successor; do
      docker_local exec -e "JC_IDO_SCANNER_RESTART_STAGE=$stage" "$CID" sh -c 'MYSQL_PWD="$(cat /run/secrets/jc-ido-schema-password)"; JC_IDO_FULL_SCHEMA_DSN="root:${MYSQL_PWD}@tcp(127.0.0.1:3306)/jc_ido_outbox_schema_it?parseTime=true&loc=UTC"; JC_IDO_FULL_SCHEMA_REDIS="127.0.0.1:16379"; export JC_IDO_FULL_SCHEMA_DSN JC_IDO_FULL_SCHEMA_REDIS; exec /usr/local/bin/jc-chainevent-fullschema.test -test.run "^TestIDOV2ScannerSuccessorProcessRetainsFailureOnFullSchema$" -test.v' || fail "isolated $stage scanner process did not preserve fail-closed state"
    done
    if [ "${JC_IDO_FULL_SCHEMA_CRON_TEST:-0}" = 1 ]; then
      docker_local exec "$CID" sh -c 'MYSQL_PWD="$(cat /run/secrets/jc-ido-schema-password)"; JC_IDO_FULL_SCHEMA_DSN="root:${MYSQL_PWD}@tcp(127.0.0.1:3306)/jc_ido_outbox_schema_it?parseTime=true&loc=UTC"; JC_IDO_FULL_SCHEMA_REDIS="127.0.0.1:16379"; export JC_IDO_FULL_SCHEMA_DSN JC_IDO_FULL_SCHEMA_REDIS; exec /usr/local/bin/jc-chainevent-fullschema.test -test.run "^TestIDOV2ScannerCronDispatchFailsClosedOnFullSchema$" -test.timeout 68s -test.v' || fail 'real scanner cron scheduling did not persist the isolated RPC failure'
    fi
    if [ "${JC_IDO_FULL_SCHEMA_HARDHAT_TEST:-0}" = 1 ]; then
      NODE_IMAGE='node:22-bookworm-slim'
      CONTRACT_ROOT="${JC_IDO_FULL_SCHEMA_CONTRACT_ROOT:-$ROOT/../joincare_code/joincare_code/_extracted/JoinCare_SmartContract-main}"
      [ -f "$CONTRACT_ROOT/scripts/local/idoV2JobEndFixture.js" ] && [ -d "$CONTRACT_ROOT/node_modules" ] && [ -f "$CONTRACT_ROOT/cache/validations.json" ] || fail 'disposable contract fixture, validation cache and offline dependencies missing'
      docker_local image inspect "$NODE_IMAGE" >/dev/null 2>&1 || fail 'pinned local Node image unavailable'
      HARDHAT_CID="$(docker_local run -d --name "$NAME-hardhat" --label "$LABEL" --network "container:$CID" --workdir /app --read-only --tmpfs /tmp:rw,nosuid,nodev,size=256m --tmpfs /root:rw,nosuid,nodev,size=64m --tmpfs /app/cache:rw,nosuid,nodev,size=64m -v "$CONTRACT_ROOT:/app:ro" "$NODE_IMAGE" node node_modules/hardhat/internal/cli/bootstrap.js node --hostname 127.0.0.1 --port 18546)" || fail 'start network-isolated local Hardhat'
      docker_local exec -i "$HARDHAT_CID" sh -c 'cat > /app/cache/validations.json' < "$CONTRACT_ROOT/cache/validations.json" || fail 'seed offline validation cache into owned tmpfs'
      for _ in $(seq 1 60); do
        if docker_local exec "$HARDHAT_CID" node -e 'fetch("http://127.0.0.1:18546",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({jsonrpc:"2.0",id:1,method:"eth_chainId",params:[]})}).then(r=>r.json()).then(x=>process.exit(x.result==="0x7a69"?0:1)).catch(()=>process.exit(1))' >/dev/null 2>&1; then break; fi
        sleep 1
      done
      docker_local exec "$HARDHAT_CID" node -e 'fetch("http://127.0.0.1:18546",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({jsonrpc:"2.0",id:1,method:"eth_chainId",params:[]})}).then(r=>r.json()).then(x=>process.exit(x.result==="0x7a69"?0:1)).catch(()=>process.exit(1))' || fail 'local Hardhat chain 31337 unavailable'
      docker_local exec -e JC_DEPLOY_RPC=http://127.0.0.1:18546 -e JC_IDO_END_FIXTURE=/tmp/jc-ido-end-fixture.json -e JC_IDO_FULL_SCHEMA_HARDHAT_FIXTURE=1 "$HARDHAT_CID" node node_modules/hardhat/internal/cli/bootstrap.js run --no-compile scripts/local/idoV2JobEndFixture.js --network jcTarget || fail 'deploy disposable Hardhat V2 fixture'
      docker_local exec "$HARDHAT_CID" cat /tmp/jc-ido-end-fixture.json > "$TMP/ido-fixture.json" || fail 'read disposable fixture into protected temp'
      docker_local exec -i "$CID" sh -c 'umask 077; cat > /tmp/jc-ido-end-fixture.json' < "$TMP/ido-fixture.json" || fail 'stream fixture into isolated Go test container'
      # End and scanner have separate once-per-minute ticks. Depending on the
      # phase at process start, both ticks can consume almost 120 seconds.
      docker_local exec "$CID" sh -c 'MYSQL_PWD="$(cat /run/secrets/jc-ido-schema-password)"; JC_IDO_FULL_SCHEMA_DSN="root:${MYSQL_PWD}@tcp(127.0.0.1:3306)/jc_ido_outbox_schema_it?parseTime=true&loc=UTC"; JC_IDO_FULL_SCHEMA_REDIS="127.0.0.1:16379"; JC_IDO_FULL_SCHEMA_HARDHAT_FIXTURE=/tmp/jc-ido-end-fixture.json; export JC_IDO_FULL_SCHEMA_DSN JC_IDO_FULL_SCHEMA_REDIS JC_IDO_FULL_SCHEMA_HARDHAT_FIXTURE; exec /usr/local/bin/jc-chainevent-fullschema.test -test.run "^TestIDOV2FullSchemaHardhatEndAndScanner$" -test.timeout 155s -test.v' || fail 'real full-schema Hardhat end and scanner did not pass'
      docker_local exec -e JC_IDO_FULL_SCHEMA_HARDHAT_STAGE=successor "$CID" sh -c 'MYSQL_PWD="$(cat /run/secrets/jc-ido-schema-password)"; JC_IDO_FULL_SCHEMA_DSN="root:${MYSQL_PWD}@tcp(127.0.0.1:3306)/jc_ido_outbox_schema_it?parseTime=true&loc=UTC"; JC_IDO_FULL_SCHEMA_REDIS="127.0.0.1:16379"; JC_IDO_FULL_SCHEMA_HARDHAT_FIXTURE=/tmp/jc-ido-end-fixture.json; export JC_IDO_FULL_SCHEMA_DSN JC_IDO_FULL_SCHEMA_REDIS JC_IDO_FULL_SCHEMA_HARDHAT_FIXTURE; exec /usr/local/bin/jc-chainevent-fullschema.test -test.run "^TestIDOV2FullSchemaHardhatEndAndScanner$" -test.timeout 40s -test.v' || fail 'independent successor scanner did not preserve the canonical end event'
    fi
  fi
fi
if [ -n "${JC_IDO_APPLICATION_TEST_BINARY:-}" ]; then
  APP_BINARY="$JC_IDO_APPLICATION_TEST_BINARY"
  [ -f "$APP_BINARY" ] && [ -x "$APP_BINARY" ] || fail 'provide a locally built Linux Core project test binary'
  docker_local cp "$APP_BINARY" "$CID:/usr/local/bin/jc-application-fullschema.test" || fail 'copy local Core test binary'
  [ "$(docker_local exec "$CID" sha256sum /usr/local/bin/jc-application-fullschema.test | cut -d' ' -f1)" = "$(sha256sum "$APP_BINARY" | cut -d' ' -f1)" ] || fail 'copied Core test binary digest mismatch'
  docker_local exec "$CID" sh -c 'MYSQL_PWD="$(cat /run/secrets/jc-ido-schema-password)"; JC_IDO_FULL_SCHEMA_DSN="root:${MYSQL_PWD}@tcp(127.0.0.1:3306)/jc_ido_outbox_schema_it?parseTime=true&loc=UTC"; export JC_IDO_FULL_SCHEMA_DSN; exec /usr/local/bin/jc-application-fullschema.test -test.run "^(TestIDOV2CoreStartupSchemaGatesOnFullMigratedMySQL|TestApplicationApprovalInsertOnFullMigratedMySQL)$" -test.v' || fail 'core startup gates or real approval INSERT on full schema failed'
fi
echo 'PASS: pinned baseline rejects model INSERT; gated migration is idempotent and repaired INSERT succeeds'
