#!/usr/bin/env bash
set -euo pipefail
backend="$(cd "$(dirname "$0")/.." && pwd -P)"
deploy="${JC_LOCAL_SCHEMA_FIXTURE_ROOT:-/mnt/d/la/desci/joincare-deploy-ido-v3}"
temp="$(mktemp)";trap 'rm -f "$temp"' EXIT
marker_sql="${JC_LOCAL_MARKER_INDEX_SQL:-/mnt/d/pi-tmp/jc-retirement-marker-deploy/migrations/2026_10_31_ido_v3_retirement_marker_index.sql}"
test -f "$marker_sql"
python3 - "$deploy" "$backend" "$temp" "$marker_sql" <<'PY'
import pathlib,sys
root,backend,target,marker_sql=sys.argv[1:]
s=(pathlib.Path(root)/'scripts/tests/ido_v3_candidate_mysql8_disposable_test.sh').read_text()
s=s.replace('root="$(cd "$(dirname "$0")/../.." && pwd -P)"','root="'+root+'"')
s=s.replace('backend="$(cd "$root/../joincare_code/joincare_code/_extracted/joincare_server-master" && pwd -P)"','backend="'+backend+'"')
s=s.replace('go1.25.11.linux-amd64','go1.26.5.linux-amd64')
marker='if [ "${JC_V3_APPROVED_SCHEMA_ONLY:-}" = 1 ]; then\n'
assert s.count(marker)==1
s=s.replace(marker,marker+'''  { echo 'SET @JC_V3_PROXY_STREAM_APPROVED=1;'; cat "$backend/pkg/job/job/chainevent/v3_proxy_stream_schema.sql"; } | db
  { echo 'SET @JC_V3_RETIRED_NONCE_APPROVED=1;'; cat "$backend/pkg/common/chaintx/v3_retired_nonce.sql"; } | db
  printf '%s\n' "INSERT INTO ido_v3_proxy_stream(chain_id,proxy_address,contract_version,mode,state,anchor_block,cursor_block,cursor_hash,revision) VALUES(31338,'0x6666666666666666666666666666666666666666','ido_v3_stablecoin','stream','running',1,1,CONCAT('0x',REPEAT('a',64)),1); INSERT INTO ido_v3_proxy_coverage(chain_id,proxy_address,contract_version,epoch,block_number,block_hash,parent_hash,digest,payload) VALUES(31338,'0x6666666666666666666666666666666666666666','ido_v3_stablecoin',1,1,CONCAT('0x',REPEAT('a',64)),CONCAT('0x',REPEAT('b',64)),REPEAT('c',64),'{\\\"RetiredTests\\\":[9007199254740999]}');" | db
  if cat "MARKER_SQL" | db > "$tmp/marker-index-approval.log" 2>&1; then echo 'unguarded marker migration accepted' >&2; exit 1; fi
  grep -q 'retirement marker index approval required' "$tmp/marker-index-approval.log"
  { echo 'SET @JC_V3_RETIREMENT_MARKER_INDEX_APPROVED=1;'; cat "MARKER_SQL"; } | db
  { echo 'SET @JC_V3_RETIREMENT_MARKER_INDEX_APPROVED=1;'; cat "MARKER_SQL"; } | db
  printf '%s\n' "SELECT retirement_marker FROM ido_v3_proxy_coverage WHERE chain_id=31338 AND proxy_address='0x6666666666666666666666666666666666666666' AND contract_version='ido_v3_stablecoin' AND epoch=1 AND block_number=1;" | db | grep -x 1
  docker -H unix:///var/run/docker.sock exec "$container" sh -c 'root="$(tr -d "\\r\\n" </run/secrets/mysql-password)"; export JOINCARE_IDO_V3_MYSQL_WRITER_DSN="root:${root}@tcp(127.0.0.1:3306)/'"$db_name"'?parseTime=true"; exec /opt/chaintx.test -test.v -test.run "^TestRetiredNonceDisposableMySQLGuardAndTriggerAgree$"'
  { echo 'SET @JC_V3_RETIRED_NONCE_ROLLBACK_APPROVED=1;'; cat "$backend/pkg/common/chaintx/v3_retired_nonce_rollback.sql"; } | db
  printf '%s\\n' "SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND EVENT_OBJECT_TABLE='chain_relayer_nonce' AND TRIGGER_NAME IN ('jc_ido_v3_nonce_insert_1020','jc_ido_v3_nonce_update_1020') AND ACTION_STATEMENT NOT LIKE '%retired_nonce_exemption%';" | db | grep -x 2
  echo 'Retired nonce conservative rollback: PASS'
'''.replace('MARKER_SQL',marker_sql))
pathlib.Path(target).write_bytes(s.encode())
PY
JC_V3_APPROVED_SCHEMA_ONLY=1 bash "$temp"
