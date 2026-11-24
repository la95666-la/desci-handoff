#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

MYSQL_CONTAINER="jc-afdf9be-gate-mysql"
REDIS_CONTAINER="jc-afdf9be-gate-redis"
BACKEND_CONTAINER="jc-afdf9be-gate-backend"
NETWORK="jc-afdf9be-gate-net"
SUBNET="172.30.250.0/24"
MYSQL_IP="172.30.250.10"
REDIS_IP="172.30.250.11"
BACKEND_IP="172.30.250.12"
HOST_PORT="13014"

IMAGE="joincare/backend:candidate-afdf9be1"
EXPECTED_IMAGE_ID="sha256:c614a5272c12d4aab88e3523b0e49a06dac63ec96a8880f8df9e1eccf6f6cab2"
ARCHIVE="${ARCHIVE:-/mnt/d/LA/desci/joincare-deploy/dist/joincare-backend-release-20260920-063158-afdf9be1.tar.gz}"
EXPECTED_ARCHIVE_SHA="4544308a588308e9f8a18bfba633d4be85236b8acc7bfd8b6ab37efce58eeafe"
DUMP="${DUMP:-$HOME/.joincare-production-backups/20260920T055002Z-daqi-precreate/joincare-production-pre-daqi.sql}"
EXPECTED_DUMP_SHA="84f0ec230a2d3469cc1547b10eb4487698b7ac9d8a043af3f7be6fe38e4cfc26"
SOURCE_I18N="${SOURCE_I18N:-/mnt/d/LA/desci/joincare-deploy/config/common/i18n}"
REPORT="${REPORT:-/mnt/d/LA/desci/joincare-deploy/dist/backend-afdf9be-isolation-report.txt}"
TMP="$(mktemp -d /tmp/jc-afdf9be-gate.XXXXXX)"

MYSQL_ROOT_PASSWORD="$(openssl rand -hex 24)"
APP_PASSWORD="$(openssl rand -hex 24)"
JWT_SECRET="$(openssl rand -hex 32)"
ADMIN_JWT_SECRET="$(openssl rand -hex 32)"
CANDIDATE_ID="900001"
ARCHIVED_ID="900002"

cleanup_runtime() {
  docker rm -f "$BACKEND_CONTAINER" "$REDIS_CONTAINER" "$MYSQL_CONTAINER" >/dev/null 2>&1 || true
  docker network rm "$NETWORK" >/dev/null 2>&1 || true
}

cleanup() {
  cleanup_runtime
  rm -rf "$TMP"
}

redact_backend_logs() {
  docker logs --tail 160 "$BACKEND_CONTAINER" 2>&1 | sed -E \
    -e 's#(https?|mysql|redis)://[^/@[:space:]]+@#\1://<redacted>@#g' \
    -e 's#([^[:space:]:]+):[^@[:space:]]+@tcp#\1:<redacted>@tcp#g' \
    -e 's#((password|passwd|secret|token|private[_-]?key|api[_-]?key)["'"'"'= :]+)[^,"'"'"'[:space:]]+#\1<redacted>#Ig'
}

on_error() {
  local rc=$?
  trap - ERR
  set +e
  {
    printf 'verdict=FAIL\n'
    printf 'failed_line=%s\n' "${BASH_LINENO[0]:-unknown}"
    printf 'backend_state=%s\n' \
      "$(docker inspect "$BACKEND_CONTAINER" --format '{{.State.Status}}/{{.State.ExitCode}}' 2>/dev/null || printf absent)"
    printf '%s\n' '--- redacted backend log tail ---'
    redact_backend_logs
  } >>"$REPORT" 2>/dev/null || true
  cleanup
  exit "$rc"
}

trap on_error ERR
trap cleanup EXIT HUP INT TERM

for command in docker openssl sha256sum curl python3 sed awk seq xargs; do
  command -v "$command" >/dev/null 2>&1 || {
    printf 'required command is unavailable: %s\n' "$command" >&2
    exit 1
  }
done

: >"$REPORT"
chmod 0600 "$REPORT"
printf 'format=JOINCARE_BACKEND_AFDF9BE_ISOLATION_V1\n' >>"$REPORT"
printf 'started_utc=%s\n' "$(date -u +%FT%TZ)" >>"$REPORT"

actual_archive_sha="$(sha256sum "$ARCHIVE" | awk '{print $1}')"
[[ "$actual_archive_sha" == "$EXPECTED_ARCHIVE_SHA" ]]
actual_image_id="$(docker image inspect "$IMAGE" --format '{{.Id}}')"
[[ "$actual_image_id" == "$EXPECTED_IMAGE_ID" ]]
actual_dump_sha="$(sha256sum "$DUMP" | awk '{print $1}')"
[[ "$actual_dump_sha" == "$EXPECTED_DUMP_SHA" ]]
printf 'artifact_sha256=%s\nimage_id=%s\ndump_sha256=%s\n' \
  "$actual_archive_sha" "$actual_image_id" "$actual_dump_sha" >>"$REPORT"

cleanup_runtime
docker network create --subnet "$SUBNET" "$NETWORK" >/dev/null

docker run -d --name "$MYSQL_CONTAINER" --network "$NETWORK" --ip "$MYSQL_IP" \
  --restart no --tmpfs /var/lib/mysql:rw,nosuid,nodev,size=1536m \
  -e MYSQL_ROOT_PASSWORD="$MYSQL_ROOT_PASSWORD" mysql:8.4 \
  --character-set-server=utf8mb4 --collation-server=utf8mb4_0900_ai_ci >/dev/null

docker run -d --name "$REDIS_CONTAINER" --network "$NETWORK" --ip "$REDIS_IP" \
  --restart no --tmpfs /data:rw,nosuid,nodev,size=64m redis:7-alpine \
  redis-server --port 7000 --cluster-enabled yes --cluster-config-file /data/nodes.conf \
  --cluster-node-timeout 5000 --cluster-announce-ip "$REDIS_IP" \
  --cluster-announce-port 7000 --cluster-announce-bus-port 17000 \
  --appendonly no --protected-mode no >/dev/null

for attempt in $(seq 1 120); do
  if docker exec -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" "$MYSQL_CONTAINER" \
    mysql -N -B -uroot -e 'SELECT 1' >/dev/null 2>&1; then
    break
  fi
  [[ "$attempt" != 120 ]]
  sleep 1
done

for attempt in $(seq 1 60); do
  if docker exec "$REDIS_CONTAINER" redis-cli -p 7000 ping 2>/dev/null | grep -Fxq PONG; then
    break
  fi
  [[ "$attempt" != 60 ]]
  sleep 1
done

seq 0 16383 | xargs -n 1000 docker exec "$REDIS_CONTAINER" redis-cli -p 7000 cluster addslots >/dev/null
for attempt in $(seq 1 30); do
  if docker exec "$REDIS_CONTAINER" redis-cli -p 7000 cluster info 2>/dev/null | \
    tr -d '\r' | grep -Fxq 'cluster_state:ok'; then
    break
  fi
  [[ "$attempt" != 30 ]]
  sleep 1
done

docker exec -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" "$MYSQL_CONTAINER" mysql -uroot \
  -e 'CREATE DATABASE joincare CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci' >/dev/null
docker exec -i -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" "$MYSQL_CONTAINER" \
  mysql -uroot joincare <"$DUMP" 2>"$TMP/restore.err"

mysql_query() {
  docker exec -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" "$MYSQL_CONTAINER" \
    mysql -N -B --raw -uroot joincare -e "$1"
}

restore_version="$(docker exec -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" "$MYSQL_CONTAINER" \
  mysql -N -B -uroot -e 'SELECT VERSION()')"
restore_tables="$(mysql_query \
  "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='joincare' AND TABLE_TYPE='BASE TABLE'")"
restore_projects="$(mysql_query 'SELECT COUNT(*) FROM projects')"
restore_collation="$(mysql_query \
  "SELECT DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='joincare'")"
[[ "$restore_version" == 8.4.10 ]]
[[ "$restore_tables" == 185 ]]
[[ "$restore_projects" == 3 ]]
[[ "$restore_collation" == utf8mb4_unicode_ci ]]
printf 'restore_version=%s\nrestore_tables=%s\nrestore_projects=%s\nrestore_collation=%s\n' \
  "$restore_version" "$restore_tables" "$restore_projects" "$restore_collation" >>"$REPORT"

app_password_sql="${APP_PASSWORD//\'/\'\'}"
docker exec -i -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" "$MYSQL_CONTAINER" mysql -uroot <<SQL >/dev/null
CREATE USER 'jc_gate_ro'@'%' IDENTIFIED BY '$app_password_sql';
GRANT SELECT ON joincare.* TO 'jc_gate_ro'@'%';
GRANT TRIGGER, SHOW VIEW ON joincare.* TO 'jc_gate_ro'@'%';
FLUSH PRIVILEGES;
SQL

mkdir -p "$TMP/config/core" "$TMP/config/common/i18n" "$TMP/config/secret"
cp -a "$SOURCE_I18N"/. "$TMP/config/common/i18n/"

cat >"$TMP/config/core/config.yaml" <<YAML
application:
  name: core
  environment: test
protocol:
  http:
    host: ""
    port: $HOST_PORT
lark:
  opsWebhook: ""
  reviewHall: {appID: disabled, appSecret: disabled, appToken: disabled, tableID: disabled}
  applyHall: {appID: disabled, appSecret: disabled, appToken: disabled, tableID: disabled}
  tgeHall: {appID: disabled, appSecret: disabled, appToken: disabled, tableID: disabled}
tencent: {secretId: disabled, secretKey: disabled}
cos: {bucket: disabled, region: ap-hongkong, host: "http://localhost"}
cdn: {privateKey: disabled}
twitter: {clientID: "", clientSecret: "", redirectURL: "http://localhost/callback", bearerToken: ""}
app:
  domain: localhost:$HOST_PORT
  adminDomain: localhost:13001
  env: test
storage:
  driver: local
  local:
    root_dir: /app/uploads
    public_base: /uploads
YAML

cat >"$TMP/config/common/config.yaml" <<'YAML'
logger:
  level: info
  maxDay: 2
  dir: /app/logs
YAML

cat >"$TMP/config/secret/config.yaml" <<YAML
db:
  mysql:
    host: $MYSQL_IP:3306
    user: jc_gate_ro
    password: "$APP_PASSWORD"
    databaseName: joincare
    readHost: $MYSQL_IP:3306
    readUser: jc_gate_ro
    readPassword: "$APP_PASSWORD"
  redis:
    host: ["$REDIS_IP:7000"]
    password: ""
jwt:
  secret: "$JWT_SECRET"
admin:
  jwtSecret: "$ADMIN_JWT_SECRET"
lark:
  opsWebhook: ""
tencent: {secretId: disabled, secretKey: disabled}
cos: {bucket: disabled, region: ap-hongkong, host: "http://localhost"}
cdn: {privateKey: disabled}
twitter: {clientID: "", clientSecret: "", redirectURL: "http://localhost/callback", bearerToken: ""}
reown: {projectID: ""}
telegram: {botToken: "", botUsername: "", bindSecret: "", opsChatID: ""}
chain:
  project:
    url: ""
    chainID: 0
    private: ""
    jobPrivate: ""
    contractAddress: ""
    v1: {enabled: false}
    startBlock: 0
    confirmations: 20
    reconciliation: {maxParticipants: 10000}
    v2:
      contractAddress: ""
      startBlock: 0
      implementationAddress: ""
      implementationCodeHash: ""
      operator: ""
      feeReceiver: ""
      successFeeRate: 0
      paymentTokens:
        USDT: {address: "", decimals: 18}
        USDC: {address: "", decimals: 18}
  task:
    url: ""
    chainID: 0
    private: ""
    contractAddress: ""
    checkinAddress: ""
YAML
find "$TMP/config" -type d -exec chmod 0755 {} +
find "$TMP/config" -type f -exec chmod 0644 {} +
chmod 0600 "$TMP/config/secret/config.yaml"

# Run only the core binary. Configuration and the image root filesystem are read-only;
# logs/uploads are isolated tmpfs. No Job/scanner/materializer process is started.
docker run -d --name "$BACKEND_CONTAINER" --network "$NETWORK" --ip "$BACKEND_IP" \
  --restart no --read-only --cap-drop ALL --cap-add DAC_OVERRIDE --security-opt no-new-privileges \
  --mount "type=bind,src=$TMP/config/core,dst=/app/config/core,readonly" \
  --mount "type=bind,src=$TMP/config/common,dst=/app/config/common,readonly" \
  --mount "type=bind,src=$TMP/config/secret/config.yaml,dst=/app/config/secret/config.yaml,readonly" \
  --tmpfs /app/logs:rw,nosuid,nodev,noexec,size=64m \
  --tmpfs /app/uploads:rw,nosuid,nodev,noexec,size=64m \
  --label joincare.validation=backend-afdf9be-archived-guard \
  "$IMAGE" >/dev/null

for attempt in $(seq 1 60); do
  state="$(docker inspect "$BACKEND_CONTAINER" --format '{{.State.Status}}')"
  [[ "$state" == running ]]
  if docker exec "$BACKEND_CONTAINER" curl -fsS --max-time 2 \
    "http://127.0.0.1:$HOST_PORT/health" >"$TMP/health.json" 2>/dev/null; then
    break
  fi
  [[ "$attempt" != 60 ]]
  sleep 1
done

python3 - "$TMP/health.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    health = json.load(handle)
assert health.get("status") is True, health
PY

# Clone a production-restored non-chain project into two disposable fixture rows.
# Only the throwaway restore is mutated; the candidate's DB identity remains SELECT-only.
docker exec -i -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" "$MYSQL_CONTAINER" mysql -uroot joincare <<SQL >/dev/null
SET SESSION group_concat_max_len=1048576;
SET @columns=(SELECT GROUP_CONCAT(CONCAT('\`',COLUMN_NAME,'\`') ORDER BY ORDINAL_POSITION SEPARATOR ',') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects');
SET @candidate_exprs=(SELECT GROUP_CONCAT(CASE COLUMN_NAME
  WHEN 'id' THEN '$CANDIDATE_ID'
  WHEN 'project_id' THEN '$CANDIDATE_ID'
  WHEN 'name_en' THEN QUOTE('__gate_candidate_afdf9be')
  WHEN 'name_cn' THEN QUOTE('__gate_candidate_afdf9be')
  WHEN 'slug' THEN QUOTE('__gate-candidate-afdf9be')
  WHEN 'status' THEN QUOTE('candidate')
  WHEN 'pre_archive_status' THEN QUOTE('pre_launch')
  WHEN 'created_at' THEN 'NOW(6)'
  WHEN 'updated_at' THEN 'NOW(6)'
  ELSE CONCAT('\`',COLUMN_NAME,'\`') END ORDER BY ORDINAL_POSITION SEPARATOR ',') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects');
SET @archived_exprs=(SELECT GROUP_CONCAT(CASE COLUMN_NAME
  WHEN 'id' THEN '$ARCHIVED_ID'
  WHEN 'project_id' THEN '$ARCHIVED_ID'
  WHEN 'name_en' THEN QUOTE('__gate_archived_afdf9be')
  WHEN 'name_cn' THEN QUOTE('__gate_archived_afdf9be')
  WHEN 'slug' THEN QUOTE('__gate-archived-afdf9be')
  WHEN 'status' THEN QUOTE('archived')
  WHEN 'pre_archive_status' THEN QUOTE('pre_launch')
  WHEN 'created_at' THEN 'NOW(6)'
  WHEN 'updated_at' THEN 'NOW(6)'
  ELSE CONCAT('\`',COLUMN_NAME,'\`') END ORDER BY ORDINAL_POSITION SEPARATOR ',') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects');
SET @candidate_sql=CONCAT('INSERT INTO projects (',@columns,') SELECT ',@candidate_exprs,' FROM projects WHERE project_id=3');
SET @archived_sql=CONCAT('INSERT INTO projects (',@columns,') SELECT ',@archived_exprs,' FROM projects WHERE project_id=3');
PREPARE candidate_insert FROM @candidate_sql; EXECUTE candidate_insert; DEALLOCATE PREPARE candidate_insert;
PREPARE archived_insert FROM @archived_sql; EXECUTE archived_insert; DEALLOCATE PREPARE archived_insert;
SQL

fixture_count="$(mysql_query \
  "SELECT COUNT(*) FROM projects WHERE project_id IN ($CANDIDATE_ID,$ARCHIVED_ID)")"
[[ "$fixture_count" == 2 ]]

backend_get() {
  docker exec "$BACKEND_CONTAINER" curl -fsS --max-time 5 "http://127.0.0.1:$HOST_PORT$1"
}

backend_get "/client/project/v1/list" >"$TMP/list-default.json"
backend_get "/client/project/v1/list?status=archived" >"$TMP/list-archived.json"
backend_get "/client/project/v1/list?status=candidate" >"$TMP/list-candidate.json"
backend_get "/client/project/v1/list?status=not-a-real-status" >"$TMP/list-unknown.json"
backend_get "/client/project/v1/detail?projectID=$ARCHIVED_ID" >"$TMP/detail-archived.json"
backend_get "/client/project/v1/detail?projectID=$CANDIDATE_ID" >"$TMP/detail-candidate.json"

python3 - "$TMP" "$CANDIDATE_ID" "$ARCHIVED_ID" >"$TMP/assertions.txt" <<'PY'
import json
import os
import sys

root, candidate, archived = sys.argv[1:]

def load(name):
    with open(os.path.join(root, name), encoding="utf-8") as handle:
        return json.load(handle)

def ids(response):
    return [str(row.get("projectID")) for row in response.get("data", {}).get("list", [])]

def required_list(name):
    response = load(name)
    assert response.get("status") is True, (name, response)
    assert isinstance(response.get("data", {}).get("list"), list), (name, response)
    return response

assert load("health.json").get("status") is True
default = required_list("list-default.json")
archived_list = required_list("list-archived.json")
candidate_list = required_list("list-candidate.json")
unknown_list = required_list("list-unknown.json")

assert candidate not in ids(default), ids(default)
assert archived not in ids(default), ids(default)
assert ids(archived_list) == [], ids(archived_list)
assert ids(unknown_list) == [], ids(unknown_list)
assert ids(candidate_list) == [candidate], ids(candidate_list)

archived_detail = load("detail-archived.json")
assert archived_detail.get("status") is False, archived_detail
candidate_detail = load("detail-candidate.json")
assert candidate_detail.get("status") is True, candidate_detail
assert candidate_detail.get("data", {}).get("status") == "candidate", candidate_detail

print("health=PASS")
print("default_list_excludes_candidate_and_archived=PASS")
print("explicit_archived_list_empty=PASS")
print("unknown_status_list_empty=PASS")
print("explicit_candidate_list_allowed=PASS")
print("archived_detail_not_found=PASS")
print("candidate_detail_allowed=PASS")
print("default_list_count=" + str(len(default["data"]["list"])))
PY

backend_image_id="$(docker inspect "$BACKEND_CONTAINER" --format '{{.Image}}')"
backend_status="$(docker inspect "$BACKEND_CONTAINER" --format '{{.State.Status}}')"
backend_restarts="$(docker inspect "$BACKEND_CONTAINER" --format '{{.RestartCount}}')"
backend_readonly="$(docker inspect "$BACKEND_CONTAINER" --format '{{.HostConfig.ReadonlyRootfs}}')"
[[ "$backend_image_id" == "$EXPECTED_IMAGE_ID" ]]
[[ "$backend_status" == running ]]
[[ "$backend_restarts" == 0 ]]
[[ "$backend_readonly" == true ]]

{
  printf 'backend_status=%s\nbackend_restart_count=%s\nbackend_readonly_rootfs=%s\n' \
    "$backend_status" "$backend_restarts" "$backend_readonly"
  printf 'backend_db_data_authority=SELECT_ONLY\n'
  printf 'backend_db_metadata_authority=TRIGGER_SHOW_VIEW_FOR_SCHEMA_GUARDS\n'
  printf 'job_started=false\nscanner_started=false\nmaterializer_started=false\n'
  cat "$TMP/assertions.txt"
  for response in \
    health.json list-default.json list-archived.json list-candidate.json \
    list-unknown.json detail-archived.json detail-candidate.json; do
    printf 'response_sha256_%s=%s\n' "${response%.json}" \
      "$(sha256sum "$TMP/$response" | awk '{print $1}')"
  done
  printf 'finished_utc=%s\nverdict=PASS\n' "$(date -u +%FT%TZ)"
} >>"$REPORT"

report_sha="$(sha256sum "$REPORT" | awk '{print $1}')"
printf 'isolation_verdict=PASS\nreport=%s\nreport_sha256=%s\n' "$REPORT" "$report_sha"
cat "$TMP/assertions.txt"
