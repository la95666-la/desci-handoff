#!/usr/bin/env bash
# Local network-none MySQL only. Reuse the existing full-schema fixture;
# install the separately gated stream draft only in its disposable database.
set -euo pipefail
backend="$(cd "$(dirname "$0")/.." && pwd -P)"
deploy="$(cd "$backend/../../../.." && pwd -P)/joincare-deploy-ido-v3"
original="$deploy/scripts/tests/ido_v3_candidate_mysql8_disposable_test.sh"
test -f "$original"
temp="$(mktemp)"
trap 'rm -f "$temp"' EXIT
python3 - "$original" "$deploy" "$temp" <<'PY'
import pathlib,sys
source,root,target=sys.argv[1:]
s=pathlib.Path(source).read_text()
s=s.replace('root="$(cd "$(dirname "$0")/../.." && pwd -P)"','root="'+root+'"')
s=s.replace('go1.25.11.linux-amd64','go1.26.5.linux-amd64')
marker='if [ "${JC_V3_APPROVED_SCHEMA_ONLY:-}" = 1 ]; then\n'
assert s.count(marker)==1
s=s.replace(marker,marker+'''  { echo 'SET @JC_V3_PROXY_STREAM_APPROVED=1;'; cat "$backend/pkg/job/job/chainevent/v3_proxy_stream_schema.sql"; } | db
''')
s=s.replace('exec /opt/journal.test -test.v -test.run','JOINCARE_V3_PROXY_STREAM_TEST=1 exec /opt/journal.test -test.v -test.run')
pathlib.Path(target).write_bytes(s.encode())
PY
JC_V3_APPROVED_SCHEMA_ONLY=1 bash "$temp"
