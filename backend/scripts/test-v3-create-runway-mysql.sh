#!/usr/bin/env bash
set -euo pipefail
backend="$(cd "$(dirname "$0")/.." && pwd -P)"
deploy="${JC_LOCAL_SCHEMA_FIXTURE_ROOT:-/mnt/d/la/desci/joincare-deploy-ido-v3}"
temp="$(mktemp)";trap 'rm -f "$temp"' EXIT
python3 - "$deploy" "$backend" "$temp" <<'PY'
import pathlib,sys
root,backend,target=sys.argv[1:]
s=(pathlib.Path(root)/'scripts/tests/ido_v3_candidate_mysql8_disposable_test.sh').read_text()
s=s.replace('root="$(cd "$(dirname "$0")/../.." && pwd -P)"','root="'+root+'"')
s=s.replace('backend="$(cd "$root/../joincare_code/joincare_code/_extracted/joincare_server-master" && pwd -P)"','backend="'+backend+'"')
s=s.replace('go1.25.11.linux-amd64','go1.26.5.linux-amd64')
marker='if [ "${JC_V3_APPROVED_SCHEMA_ONLY:-}" = 1 ]; then\n'
assert s.count(marker)==1
s=s.replace(marker,marker+'''  { echo 'SET @JC_V3_PROXY_STREAM_APPROVED=1;'; cat "$backend/pkg/job/job/chainevent/v3_proxy_stream_schema.sql"; } | db
  if cat "$backend/pkg/common/chaintx/v3_original99_disposition.sql" | db > "$tmp/original99-refused.log" 2>&1; then echo 'unguarded original99 migration accepted' >&2; exit 1; fi
  grep -q 'original99 disposition approval required' "$tmp/original99-refused.log"
  { echo 'SET @JC_ORIGINAL99_DISPOSITION_APPROVED=1;'; cat "$backend/pkg/common/chaintx/v3_original99_disposition.sql"; } | db
  { echo 'SET @JC_ORIGINAL99_DISPOSITION_APPROVED=1;'; cat "$backend/pkg/common/chaintx/v3_original99_disposition.sql"; } | db
  docker -H unix:///var/run/docker.sock exec "$container" sh -c 'root="$(tr -d "\\r\\n" </run/secrets/mysql-password)"; export JOINCARE_IDO_V3_MYSQL_WRITER_DSN="root:${root}@tcp(127.0.0.1:3306)/'"$db_name"'?parseTime=true"; exec /opt/chaintx.test -test.v -test.run "^TestV3SignedTimeDisposableMySQLReadyTransition$"'
  docker -H unix:///var/run/docker.sock exec "$container" sh -c 'root="$(tr -d "\\r\\n" </run/secrets/mysql-password)"; export JOINCARE_IDO_V3_MYSQL_WRITER_DSN="root:${root}@tcp(127.0.0.1:3306)/'"$db_name"'?parseTime=true"; exec /opt/chaintx.test -test.v -test.run "^TestOriginal99DispositionDisposableMySQLGuards$"'
''')
pathlib.Path(target).write_bytes(s.encode())
PY
JC_V3_APPROVED_SCHEMA_ONLY=1 bash "$temp"
