#!/usr/bin/env bash
set -euo pipefail
backend="$(cd "$(dirname "$0")/.." && pwd -P)"
deploy="${JC_LOCAL_SCHEMA_FIXTURE_ROOT:-/mnt/d/la/desci/joincare-deploy-ido-v3}"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
python3 - "$deploy" "$backend" "$tmp" <<'PY'
import pathlib,sys
root,backend,target=sys.argv[1:]
s=(pathlib.Path(root)/'scripts/tests/ido_v3_candidate_mysql8_disposable_test.sh').read_text()
s=s.replace('root="$(cd "$(dirname "$0")/../.." && pwd -P)"','root="'+root+'"')
s=s.replace('backend="$(cd "$root/../joincare_code/joincare_code/_extracted/joincare_server-master" && pwd -P)"','backend="'+backend+'"')
s=s.replace('go1.25.11.linux-amd64','go1.26.5.linux-amd64')
line=next(x for x in s.splitlines() if 'exec /opt/project.test' in x and 'TestApprovedV3SetDisposableMySQLTransaction' in x)
s=s.replace(line,line+'\n'+line.replace('TestApprovedV3SetDisposableMySQLTransaction','(TestPolicyReviewRequestDisposableMySQLDuplicate|TestPolicyHandoffDisposableMySQLApproval)'))
# The old base fixture's optional empty-range stream regression requires the
# later stream/freshness/tracking fixtures. Keep this runner narrowly scoped
# to approved Set + policy request SQL; their standalone runners cover stream.
s=s.replace('if [ "${JC_V3_APPROVED_SCHEMA_ONLY:-}" = 1 ]; then\n','if [ "${JC_V3_APPROVED_SCHEMA_ONLY:-}" = 1 ]; then\n  echo "Approved Set and policy request MySQL checks: PASS"\n  exit 0\n')
pathlib.Path(target).write_bytes(s.encode())
PY
JC_V3_APPROVED_SCHEMA_ONLY=1 bash "$tmp"
