#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.."&&pwd)";. "$ROOT/scripts/tests/fair_launch_local_e2e_test_cleanup.sh";ACTIVE_RUN='';ACTIVE_ROOT=''
cleanup(){ local original=$? ok=1;set +e;if [ -n "$ACTIVE_RUN" ]&&[ -d "$ACTIVE_ROOT/$ACTIVE_RUN" ];then jc_fl_test_bounded_teardown "$ACTIVE_ROOT" "$ACTIVE_RUN"||ok=0;fi;if [ "$ok" = 1 ]&&[ -n "$ACTIVE_ROOT" ]&&[ -d "$ACTIVE_ROOT" ];then jc_fl_test_root_finalize "$ACTIVE_ROOT"||ok=0;fi;[ "$ok" = 1 ]||echo "schema publication cleanup retained evidence at $ACTIVE_ROOT/$ACTIVE_RUN" >&2;trap - EXIT INT TERM;if [ "$original" -ne 0 ];then exit "$original";fi;[ "$ok" = 1 ]||exit 1;}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
fail(){ echo "schema_publication_fault_test: FAIL: $*" >&2;exit 1;}
POINTS="${JC_FL_E2E_SCHEMA_FAULT_POINTS:-after_profile_runtime after_profile_admin after_profile_job after_profile_ingest after_profile_staging before_schema_evidence during_schema_evidence after_schema_evidence before_final_marker after_final_marker after_ready_state_rename before_final_verification fail_final_verification tamper_final_profile before_publish_complete}"
for point in $POINTS;do
 S="$(jc_fl_test_root_create JC_FL_TEST_SCHEMA_PUBLICATION_V1)";chmod 700 "$S";ACTIVE_ROOT="$S";out="$(JC_FL_E2E_STATE_ROOT="$S" bash "$ROOT/scripts/fair-launch/local-e2e/start.sh")";run="$(echo "$out"|sed -n 's/^RUN_ID=//p')";ACTIVE_RUN="$run";d="$S/$run"
 set +e;JC_FL_E2E_STATE_ROOT="$S" JC_FL_E2E_TEST_FAIL_AT="$point" bash "$ROOT/scripts/fair-launch/local-e2e/apply-schema.sh" "$run">"$S/schema-fault.out" 2>&1;rc=$?;set -e
 [ "$rc" -ne 0 ]||fail "$point did not fail";JC_FL_E2E_STATE_ROOT="$S";jc_fl_e2e_load_state "$run";[ "$MYSQL_SCHEMA_READY" = 0 ]||fail "$point state ready";[ ! -e "$d/evidence/schema-readiness.json" ]||fail "$point marker visible";[ ! -e "$d/evidence/database-schema.json" ]||fail "$point schema evidence visible";[ ! -e "$d/credentials" ]||fail "$point profiles visible"
 set +e;JC_FL_E2E_STATE_ROOT="$S" bash "$ROOT/scripts/fair-launch/local-e2e/status.sh" "$run"|grep -Fq 'mysql_credential_state=not_ready';ok=$?;set -e;[ "$ok" = 0 ]||fail "$point status"
 jc_fl_test_bounded_teardown "$S" "$run"||fail "$point teardown";ACTIVE_RUN='';jc_fl_test_root_finalize "$S"||fail "$point root-finalize";ACTIVE_ROOT=''
done
echo 'fair_launch_local_e2e_schema_publication_fault_test: PASS staged profiles evidence final-marker fail closed with bounded teardown/root finalization'
