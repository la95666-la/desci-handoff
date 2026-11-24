#!/usr/bin/env bash
# Existing network-none full-schema harness, plus exact two-connection RR test.
set -euo pipefail
backend="$(cd "$(dirname "$0")/.." && pwd -P)"
deploy="$(cd "$backend/../../../.." && pwd -P)/joincare-deploy-ido-v3"
temp="$(mktemp)"; trap 'rm -f "$temp"' EXIT
python3 - "$deploy" "$temp" <<'PY'
import pathlib,sys
root,target=sys.argv[1:]
s=(pathlib.Path(root)/'scripts/tests/ido_v3_candidate_mysql8_disposable_test.sh').read_text()
s=s.replace('root="$(cd "$(dirname "$0")/../.." && pwd -P)"','root="'+root+'"')
s=s.replace('go1.25.11.linux-amd64','go1.26.5.linux-amd64')
marker='if [ "${JC_V3_APPROVED_SCHEMA_ONLY:-}" = 1 ]; then\n'
assert s.count(marker)==1
s=s.replace(marker,marker+'''  docker -H unix:///var/run/docker.sock exec "$container" sh -c 'root="$(tr -d "\\r\\n" </run/secrets/mysql-password)"; export JOINCARE_IDO_V3_MYSQL_WRITER_DSN="root:${root}@tcp(127.0.0.1:3306)/'"$db_name"'?parseTime=true"; exec /opt/chaintx.test -test.v -test.run "^TestV3CursorCurrentCheckpointDisposableMySQL$"'
''')
pathlib.Path(target).write_bytes(s.encode())
PY
JC_V3_APPROVED_SCHEMA_ONLY=1 bash "$temp"
