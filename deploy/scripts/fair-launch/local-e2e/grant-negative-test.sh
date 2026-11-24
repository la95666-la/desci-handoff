#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"; . "$HERE/common.sh"
[ "$#" = 1 ] || jc_fl_e2e_fail 'usage: grant-negative-test.sh <run-id>'
jc_fl_e2e_load_state "$1"
C="$MYSQL_CONTAINER_ID"; DB="$DB_NAME"; POLICY="$HERE/grant-policy.js"; NODE_BIN="$(command -v node || command -v node.exe || true)"; [ -n "$NODE_BIN" ] || jc_fl_e2e_fail 'node is required'; NODE_POLICY="$(wslpath -m "$POLICY" 2>/dev/null || printf '%s' "$POLICY")"
mysql_root(){ jc_fl_e2e_docker exec -i "$C" mysql --defaults-extra-file=/tmp/root.cnf --port="$MYSQL_PORT" --batch --raw --skip-column-names "$@"; }
for role in runtime admin job ingest; do
  show="$(printf "SHOW GRANTS FOR 'local_%s'@'%%';\n" "$role" | mysql_root | tr -d '\r')"
  if printf '%s\n' "$show" | grep -Fq 'WITH GRANT OPTION'; then jc_fl_e2e_fail "$role unexpectedly has grant option"; fi
  printf '%s\n' "$show" | "$NODE_BIN" -e "const p=require(process.argv[1]);let s='';process.stdin.on('data',x=>s+=x).on('end',()=>p.normalize(s.split(/\\r?\\n/)))" "$NODE_POLICY"
done
set +e
"$NODE_BIN" -e "const p=require(process.argv[1]);p.normalize([\"GRANT SELECT ON \\\`x\\\`.\\\`y\\\` TO 'local_runtime'@'%' WITH GRANT OPTION\"])" "$NODE_POLICY" >/tmp/jc-fl-grant-option.out 2>&1
rc=$?
set -e
[ "$rc" -ne 0 ] && grep -Fq 'unexpected WITH GRANT OPTION' /tmp/jc-fl-grant-option.out || jc_fl_e2e_fail 'grant-option normalization bypass accepted'
for dangerous in \
 "GRANT ALL PRIVILEGES ON *.* TO 'local_runtime'@'%'" \
 "GRANT SELECT ON \`$DB\`.* TO 'local_runtime'@'%'" \
 "GRANT SYSTEM_USER ON *.* TO 'local_runtime'@'%'" \
 "GRANT SELECT (secret) ON \`$DB\`.\`projects\` TO 'local_runtime'@'%'"; do
 set +e; "$NODE_BIN" -e "const p=require(process.argv[1]);p.normalize([process.argv[2]])" "$NODE_POLICY" "$dangerous" >>/tmp/jc-fl-grant-option.out 2>&1; rc=$?; set -e
 [ "$rc" -ne 0 ] || jc_fl_e2e_fail "dangerous grant normalized: $dangerous"
done
for sql in \
 "INSERT INTO fair_launch_reversal_serialization_guard_v3(reversed_lineage_id,run_row_id,chain_id,source_binding_id,source_id,source_revision_id,state,created_at) VALUES(1,1,31337,1,1,1,'open',NOW(6))" \
 "UPDATE fair_launch_reversal_serialization_guard_v3 SET reversal_lineage_id=2,state='bound',bound_at=NOW(6) WHERE reversed_lineage_id=1" \
 "UPDATE fair_launch_reversal_serialization_guard_v3 SET state='closed',closed_at=NOW(6) WHERE reversed_lineage_id=1"; do
 set +e; printf '%s;\n' "$sql" | jc_fl_e2e_docker exec -i "$C" mysql --defaults-extra-file=/tmp/local_job.cnf --port="$MYSQL_PORT" --batch --raw "$DB" >/tmp/jc-fl-job-guard.out 2>&1; rc=$?; set -e
 [ "$rc" -ne 0 ] && grep -Eqi 'command denied|denied to user|access denied' /tmp/jc-fl-job-guard.out || { cat /tmp/jc-fl-job-guard.out >&2; jc_fl_e2e_fail "local_job mutated reversal guard: $sql"; }
done
for sql in \
 "INSERT INTO fair_launch_source_token_binding_v3(id,run_row_id,chain_id,source_id,contract_address,token_address,token_decimals,metadata_version,admission_generation,runtime_code_hash,metadata_digest,admitted_at) VALUES(1,1,31337,1,'0x1111111111111111111111111111111111111111','0x2222222222222222222222222222222222222222',18,1,1,REPEAT('a',64),REPEAT('b',64),NOW(6))" \
 "UPDATE fair_launch_source_token_binding_v3 SET token_decimals=6 WHERE id=1" \
 "DELETE FROM fair_launch_source_token_binding_v3 WHERE id=1"; do
 set +e; printf '%s;\n' "$sql" | jc_fl_e2e_docker exec -i "$C" mysql --defaults-extra-file=/tmp/local_job.cnf --port="$MYSQL_PORT" --batch --raw "$DB" >/tmp/jc-fl-job-token-binding.out 2>&1; rc=$?; set -e
 [ "$rc" -ne 0 ] && grep -Eqi 'command denied|denied to user|access denied' /tmp/jc-fl-job-token-binding.out || { cat /tmp/jc-fl-job-token-binding.out >&2; jc_fl_e2e_fail "local_job mutated source token admission: $sql"; }
done
show_job="$(printf "SHOW GRANTS FOR 'local_job'@'%%';\n" | mysql_root | tr -d '\r')"
printf '%s\n' "$show_job" | grep -Fq "GRANT SELECT ON \`$DB\`.\`fair_launch_reversal_serialization_guard_v3\`" || jc_fl_e2e_fail 'local_job guard SELECT grant missing'
! printf '%s\n' "$show_job" | grep -Eq "GRANT .*(INSERT|UPDATE).* ON \`$DB\`.\`fair_launch_reversal_serialization_guard_v3\`" || jc_fl_e2e_fail 'local_job guard mutation grant present'
printf '%s\n' "$show_job" | grep -Fq "GRANT SELECT ON \`$DB\`.\`fair_launch_source_token_binding_v3\`" || jc_fl_e2e_fail 'local_job source token binding SELECT grant missing'
! printf '%s\n' "$show_job" | grep -Eq "GRANT .*(INSERT|UPDATE|DELETE).* ON \`$DB\`.\`fair_launch_source_token_binding_v3\`" || jc_fl_e2e_fail 'local_job source token binding mutation grant present'
rm -f /tmp/jc-fl-grant-option.out /tmp/jc-fl-job-guard.out /tmp/jc-fl-job-token-binding.out
echo 'fair_launch_local_e2e_grant_negative_test: PASS roles=4 guard_direct_mutations=3 token_binding_direct_mutations=3 grant-option/global/schema/dynamic/column rejected'
