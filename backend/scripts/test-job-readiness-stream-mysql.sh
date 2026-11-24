#!/usr/bin/env bash
# Local disposable MySQL only. The pinned schema runner refuses remote Docker,
# external DSNs and production databases; this wrapper only selects our source.
set -euo pipefail
backend="$(cd "$(dirname "$0")/.." && pwd -P)"
deploy="${JC_LOCAL_SCHEMA_FIXTURE_ROOT:-/mnt/d/la/desci/joincare-deploy-ido-v3}"
original="$deploy/scripts/tests/ido_v3_candidate_mysql8_disposable_test.sh"
test -f "$original"
temp="$(mktemp)"
trap 'rm -f "$temp"' EXIT
python3 - "$original" "$deploy" "$backend" "$temp" <<'PY'
import pathlib,sys
source,root,backend,target=sys.argv[1:]
s=pathlib.Path(source).read_text()
old='root="$(cd "$(dirname "$0")/../.." && pwd -P)"'
assert s.count(old)==1
s=s.replace(old,'root="'+root+'"')
old='backend="$(cd "$root/../joincare_code/joincare_code/_extracted/joincare_server-master" && pwd -P)"'
assert s.count(old)==1
s=s.replace(old,'backend="'+backend+'"')
s=s.replace('go1.25.11.linux-amd64','go1.26.5.linux-amd64')
marker='if [ "${JC_V3_APPROVED_SCHEMA_ONLY:-}" = 1 ]; then\n'
assert s.count(marker)==1
receipt_lines=[line for line in s.splitlines() if 'exec /opt/chaintx.test' in line and 'TestV3LifecycleReceiptDisposableMySQLFence' in line]
assert len(receipt_lines)==1
receipt=receipt_lines[0].replace('(TestV3SignedTimeDisposableMySQLReadyTransition|TestV3LifecycleReceiptDisposableMySQLFence)', 'TestV3LifecycleReceiptDisposableMySQLFence')
s=s.replace(marker,marker+'''  { echo 'SET @JC_V3_PROXY_STREAM_APPROVED=1;'; cat "$backend/pkg/job/job/chainevent/v3_proxy_stream_schema.sql"; } | db
'''+receipt+'\n')
s=s.replace('exec /opt/journal.test -test.v -test.run','JOINCARE_V3_PROXY_STREAM_TEST=1 exec /opt/journal.test -test.v -test.run')
pathlib.Path(target).write_text(s)
PY
JC_V3_APPROVED_SCHEMA_ONLY=1 bash "$temp"
