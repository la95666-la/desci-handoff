#!/usr/bin/env bash
# Adds independently gated freshness migration to the existing network-none
# schema fixture. Never accepts a production DSN or touches an existing DB.
set -euo pipefail
backend="$(cd "$(dirname "$0")/.." && pwd -P)"
deploy="${JC_LOCAL_SCHEMA_FIXTURE_ROOT:-/mnt/d/la/desci/joincare-deploy-ido-v3}"
freshness_sql="${JC_LOCAL_FRESHNESS_SQL:-/mnt/d/pi-tmp/jc-project-freshness-deploy/migrations/2026_10_29_ido_v3_project_freshness.sql}"
test -f "$freshness_sql"
tracking_sql="${JC_LOCAL_TRACKING_SQL:-/mnt/d/pi-tmp/jc-project-closure-deploy/migrations/2026_10_30_ido_v3_project_closure_tracking.sql}"
test -f "$tracking_sql"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
python3 - "$deploy/scripts/tests/ido_v3_candidate_mysql8_disposable_test.sh" "$deploy" "$backend" "$freshness_sql" "$tracking_sql" "$tmp" <<'PY'
import pathlib,sys
source,root,backend,migration,tracking,target=sys.argv[1:]
s=pathlib.Path(source).read_text()
s=s.replace('root="$(cd "$(dirname "$0")/../.." && pwd -P)"','root="'+root+'"')
s=s.replace('backend="$(cd "$root/../joincare_code/joincare_code/_extracted/joincare_server-master" && pwd -P)"','backend="'+backend+'"')
s=s.replace('go1.25.11.linux-amd64','go1.26.5.linux-amd64')
marker='if [ "${JC_V3_APPROVED_SCHEMA_ONLY:-}" = 1 ]; then\n'
assert s.count(marker)==1
extra='''  { echo 'SET @JC_V3_PROXY_STREAM_APPROVED=1;'; cat "$backend/pkg/job/job/chainevent/v3_proxy_stream_schema.sql"; } | db
  if cat "MIGRATION" | db > "$tmp/freshness-gate.log" 2>&1; then echo 'freshness migration bypassed approval gate' >&2; exit 1; fi
  grep -q 'migration approval required' "$tmp/freshness-gate.log"
  { echo 'SET @JC_V3_PROJECT_FRESHNESS_APPROVED=1;'; cat "MIGRATION"; } | db
  { echo 'SET @JC_V3_PROJECT_FRESHNESS_APPROVED=1;'; cat "MIGRATION"; } | db
  if printf "INSERT INTO ido_v3_source_freshness VALUES(56,'0x0d4040875742975269c55e16c8884d3ee9510b8e','ido_v3_stablecoin',1,REPEAT('a',64),'healthy',1,CONCAT('0x',REPEAT('b',64)),20,UTC_TIMESTAMP(6));\\n" | db > "$tmp/freshness-writer-gate.log" 2>&1; then echo 'unguarded freshness writer accepted' >&2; exit 1; fi
  grep -q 'freshness writer required' "$tmp/freshness-writer-gate.log"
'''.replace('MIGRATION',migration)
extra+=f'''  {{ echo 'SET @JC_V3_PROJECT_TRACKING_APPROVED=1;'; cat "{tracking}"; }} | db
  {{ echo 'SET @JC_V3_PROJECT_TRACKING_APPROVED=1;'; cat "{tracking}"; }} | db
'''
# Build a separate whitebox gate test binary, mounted read-only in the same
# disposable MySQL container. Nothing is installed on the host.
build='(cd "$backend" && CGO_ENABLED=0 GOEXPERIMENT=jsonv2 GOTOOLCHAIN=local "$go_bin" test -c -o "$tmp/fresh.test" ./pkg/common/idov3whitelist)\n(cd "$backend" && CGO_ENABLED=0 GOEXPERIMENT=jsonv2 GOTOOLCHAIN=local "$go_bin" test -c -o "$tmp/admin.test" ./pkg/core/admin/controller)\n'
point='container="$(docker -H unix:///var/run/docker.sock run'
assert s.count(point)==1
s=s.replace(point,build+point)
s=s.replace('-v "$tmp/audit.test:/opt/audit.test:ro"','-v "$tmp/admin.test:/opt/admin.test:ro" -v "$tmp/fresh.test:/opt/fresh.test:ro" -v "$tmp/audit.test:/opt/audit.test:ro"')
line=next(x for x in s.splitlines() if 'exec /opt/journal.test' in x and 'TestV3EmptyRangeDisposableMySQL$' in x)
extra+=line.replace('/opt/journal.test','/opt/fresh.test').replace('TestV3EmptyRangeDisposableMySQL','TestProjectFreshnessDisposableMySQLIsolationAndExpiry')+'\n'
extra+=line.replace('/opt/journal.test','/opt/admin.test').replace('TestV3EmptyRangeDisposableMySQL','TestV3FeeStatusDisposableMySQL')+'\n'
s=s.replace(marker,marker+extra)
s=s.replace('TestV3EmptyRangeDisposableMySQL$', '(TestV3EmptyRangeDisposableMySQL|TestV3TrackingDisposableMySQL)$')
s=s.replace('exec /opt/journal.test -test.v -test.run','JOINCARE_V3_PROXY_STREAM_TEST=1 exec /opt/journal.test -test.v -test.run')
pathlib.Path(target).write_text(s)
PY
JC_V3_APPROVED_SCHEMA_ONLY=1 bash "$tmp"
