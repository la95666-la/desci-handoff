#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.."&&pwd)";. "$ROOT/scripts/tests/fair_launch_local_e2e_test_cleanup.sh";S="$(jc_fl_test_root_create JC_FL_TEST_CLEANUP_ADVERSARIAL_V1)";ACTIVE_DIR='';ACTIVE_PROCESS=''
cleanup(){ local original=$? ok=1;set +e;if [ -n "$ACTIVE_PROCESS" ]&&[ -f "$ACTIVE_PROCESS" ];then jc_fl_test_process_remove "$ACTIVE_PROCESS" TERM||ok=0;fi;if [ -n "$ACTIVE_DIR" ]&&[ -d "$ACTIVE_DIR" ];then jc_fl_test_run_cleanup "$ACTIVE_DIR"||ok=0;fi;if [ "$ok" = 1 ];then jc_fl_test_root_finalize "$S"||ok=0;fi;if [ "$ok" != 1 ];then echo "cleanup_adversarial_test: cleanup retained evidence at $ACTIVE_DIR" >&2;fi;[ "$original" -ne 0 ]&&return "$original";[ "$ok" = 1 ];};trap cleanup EXIT
mode="${JC_FL_E2E_ADVERSARIAL_MODE:-all}";case "$mode" in all)kinds=(privileged extra-label command mount wrong-component wrong-image image-ref cap device security pid ipc restart group-add device-rule sysctl ulimit entrypoint user workdir stop-signal healthcheck);;replacement-a)kinds=(privileged extra-label command mount wrong-component wrong-image image-ref);;replacement-b)kinds=(cap device security pid ipc restart group-add);;replacement-c)kinds=(device-rule sysctl ulimit entrypoint user workdir stop-signal healthcheck);;core)kinds=();;*)echo "cleanup_adversarial_test: invalid mode: $mode" >&2;exit 1;;esac
for kind in "${kinds[@]}";do
 set +e;JC_FL_E2E_STATE_ROOT="$S" JC_FL_E2E_TEST_REPLACE_CONTAINER="$kind" timeout --foreground 240s bash "$ROOT/scripts/fair-launch/local-e2e/start.sh">"$S/out-$kind" 2>&1;rc=$?;set -e;[ "$rc" -ne 0 ];d="$(find "$S" -mindepth 1 -maxdepth 1 -type d|head -1)";ACTIVE_DIR="$d";[ "$(stat -Lc '%a' "$d" "$d/evidence/start-failure.json" "$d/evidence/replacement-created.source.json")" = $'700\n600\n600' ];grep -Fq mysql-ownership-ambiguous "$d/evidence/start-failure.json";id="$(python3 "$JC_FL_TEST_CLEANUP_PY" container-static-verify "$d/evidence/replacement-created.source.json")";if [ "$kind" = privileged ];then cp "$d/evidence/replacement-created.source.json" "$d/relocated-container-record.json";chmod 600 "$d/relocated-container-record.json";set +e;jc_fl_test_container_remove "$d/relocated-container-record.json";relocated_rc=$?;set -e;[ "$relocated_rc" -ne 0 ];docker inspect "$id">/dev/null;fi;jc_fl_test_run_cleanup "$d";ACTIVE_DIR=''
done
if [ "$mode" != all ]&&[ "$mode" != core ];then jc_fl_test_root_finalize "$S";trap - EXIT;echo "fair_launch_local_e2e_cleanup_adversarial_test: PASS $mode replacement authority and cleanup";exit 0;fi
# Strict source -> removal -> exact-absence chain rejects tampering at every authority boundary.
out="$(JC_FL_E2E_STATE_ROOT="$S" bash "$ROOT/scripts/fair-launch/local-e2e/start.sh")";run="$(printf '%s\n' "$out"|sed -n 's/^RUN_ID=//p')";d="$S/$run";ACTIVE_DIR="$d";source="$d/evidence/docker-created.source.json";removal="${source}.removal.json";query="${source}.post-rm-query";query_raw="${source}.post-rm-query.raw";absence="$d/evidence/docker-created.absence.json";id="$(python3 "$JC_FL_TEST_CLEANUP_PY" container-static-verify "$source")";cp "$source" "$S/source.good";chmod 600 "$S/source.good"
for variant in projection container run-inode evidence-inode record-path duplicate mode symlink;do rm -f "$source";cp "$S/source.good" "$source";chmod 600 "$source";case "$variant" in projection|container|run-inode|evidence-inode|record-path)python3 - "$source" "$variant" <<'PY'
import json,sys
p,k=sys.argv[1:];o=json.load(open(p))
if k=='projection':o['projectionDigest']='0'*64
elif k=='container':o['containerId']='0'*64
elif k=='run-inode':o['runInode']+=1
elif k=='evidence-inode':o['evidenceInode']+=1
else:o['recordPath']=p+'.moved'
open(p,'w').write(json.dumps(o)+'\n')
PY
;;duplicate)python3 - "$source" <<'PY'
import sys
p=sys.argv[1];s=open(p).read().rstrip();open(p,'w').write(s[:-1]+',"containerId":"'+'0'*64+'"}\n')
PY
;;mode)chmod 640 "$source";;symlink)rm "$source";ln -s "$S/source.good" "$source";;esac;set +e;jc_fl_test_run_cleanup "$d";rc=$?;set -e;[ "$rc" -ne 0 ]||fail "tampered source accepted: $variant";docker inspect "$id">/dev/null;done
rm -f "$source";cp "$S/source.good" "$source";chmod 600 "$source";python3 "$JC_FL_TEST_CLEANUP_PY" removal-write "$source" "$removal";cp "$removal" "$S/removal.good";chmod 600 "$S/removal.good"
for variant in source-digest projection container run-inode stage duplicate mode symlink;do rm -f "$removal";cp "$S/removal.good" "$removal";chmod 600 "$removal";case "$variant" in source-digest|projection|container|run-inode|stage)python3 - "$removal" "$variant" <<'PY'
import json,sys
p,k=sys.argv[1:];o=json.load(open(p))
if k=='source-digest':o['sourceRecordDigest']='0'*64
elif k=='projection':o['projectionDigest']='0'*64
elif k=='container':o['containerId']='0'*64
elif k=='run-inode':o['runInode']+=1
else:o['stage']='UNKNOWN'
open(p,'w').write(json.dumps(o)+'\n')
PY
;;duplicate)python3 - "$removal" <<'PY'
import sys
p=sys.argv[1];s=open(p).read().rstrip();open(p,'w').write(s[:-1]+',"stage":"AUTHORIZED_BEFORE_RM"}\n')
PY
;;mode)chmod 640 "$removal";;symlink)rm "$removal";ln -s "$S/removal.good" "$removal";;esac;set +e;jc_fl_test_run_cleanup "$d";rc=$?;set -e;[ "$rc" -ne 0 ]||fail "tampered removal accepted: $variant";docker inspect "$id">/dev/null;done
rm -f "$removal";cp "$S/removal.good" "$removal";chmod 600 "$removal";set +e;JC_FL_E2E_TEST_LIFECYCLE_POST_RM_QUERY_FAIL=1 jc_fl_test_run_cleanup "$d";rc=$?;set -e;[ "$rc" -ne 0 ]&&[ "$(python3 "$JC_FL_TEST_CLEANUP_PY" removal-stage "$removal" "$source")" = RM_SUCCEEDED_QUERY_PENDING ]||fail 'post-rm uncertainty transaction';set +e;docker inspect "$id">/dev/null 2>&1;inspect_rc=$?;set -e;[ "$inspect_rc" -ne 0 ]||fail 'post-rm fixture still live'
for variant in empty generic wrong-id substring extra-line timeout-status generic-status;do case "$variant" in empty):>"$query_raw";;generic|generic-status)printf '%s\n' 'Error response from daemon: internal failure'>"$query_raw";;wrong-id)printf 'Error: No such object: %064d\n' 0>"$query_raw";;substring)printf 'Error: No such object: prefix%s\n' "$id">"$query_raw";;extra-line)printf 'Error: No such object: %s\nadditional diagnostic\n' "$id">"$query_raw";;timeout-status)printf 'Error: No such object: %s\n' "$id">"$query_raw";;esac;chmod 600 "$query_raw";exit_code=1;[ "$variant" = timeout-status ]&&exit_code=124;set +e;python3 "$JC_FL_TEST_CLEANUP_PY" query-write "$source" "$query_raw" "$exit_code" "$query">/dev/null 2>&1;rc=$?;set -e;[ "$rc" -ne 0 ]&&[ ! -e "$query" ]&&[ ! -e "$absence" ]||fail "non-exact query accepted: $variant";done
printf 'Error: No such object: %s\n' "$id">"$query_raw";chmod 600 "$query_raw";python3 "$JC_FL_TEST_CLEANUP_PY" query-write "$source" "$query_raw" 1 "$query";cp "$query" "$S/query.good";chmod 600 "$S/query.good"
rm -f "$query";printf 'error: no such object: %s\n' "$id">"$query_raw";chmod 600 "$query_raw";python3 "$JC_FL_TEST_CLEANUP_PY" query-write "$source" "$query_raw" 1 "$query";cp "$query" "$S/query.good";chmod 600 "$S/query.good"
for variant in source-digest raw-digest container exit status duplicate mode symlink;do rm -f "$query";cp "$S/query.good" "$query";chmod 600 "$query";case "$variant" in source-digest|raw-digest|container|exit|status)python3 - "$query" "$variant" <<'PY'
import json,sys
p,k=sys.argv[1:];o=json.load(open(p))
if k=='source-digest':o['sourceRecordDigest']='0'*64
elif k=='raw-digest':o['rawEvidenceDigest']='0'*64
elif k=='container':o['containerId']='0'*64
elif k=='exit':o['exitCode']=124
else:o['status']='UNKNOWN'
open(p,'w').write(json.dumps(o)+'\n')
PY
;;duplicate)python3 - "$query" <<'PY'
import sys
p=sys.argv[1];s=open(p).read().rstrip();open(p,'w').write(s[:-1]+',"exitCode":1}\n')
PY
;;mode)chmod 640 "$query";;symlink)rm "$query";ln -s "$S/query.good" "$query";;esac;set +e;python3 "$JC_FL_TEST_CLEANUP_PY" absence-write "$source" "$removal" "$query" "$absence">/dev/null 2>&1;rc=$?;set -e;[ "$rc" -ne 0 ]&&[ ! -e "$absence" ]||fail "tampered query accepted: $variant";done
rm -f "$query";cp "$S/query.good" "$query";chmod 600 "$query";python3 "$JC_FL_TEST_CLEANUP_PY" absence-write "$source" "$removal" "$query" "$absence";python3 "$JC_FL_TEST_CLEANUP_PY" absence-verify "$absence">/dev/null;cp "$absence" "$S/absence.good";chmod 600 "$S/absence.good";jc_fl_test_redis_remove "$d"
for variant in source-digest removal-digest query-digest projection container run-inode duplicate mode symlink;do rm -f "$absence";cp "$S/absence.good" "$absence";chmod 600 "$absence";case "$variant" in source-digest|removal-digest|query-digest|projection|container|run-inode)python3 - "$absence" "$variant" <<'PY'
import json,sys
p,k=sys.argv[1:];o=json.load(open(p))
if k=='source-digest':o['sourceRecordDigest']='0'*64
elif k=='removal-digest':o['removalRecordDigest']='0'*64
elif k=='query-digest':o['queryEvidenceDigest']='0'*64
elif k=='projection':o['projectionDigest']='0'*64
elif k=='container':o['containerId']='0'*64
else:o['runInode']+=1
open(p,'w').write(json.dumps(o)+'\n')
PY
;;duplicate)python3 - "$absence" <<'PY'
import sys
p=sys.argv[1];s=open(p).read().rstrip();open(p,'w').write(s[:-1]+',"status":"ABSENT_EXACT_ID"}\n')
PY
;;mode)chmod 640 "$absence";;symlink)rm "$absence";ln -s "$S/absence.good" "$absence";;esac;set +e;jc_fl_test_run_delete "$d";rc=$?;set -e;[ "$rc" -ne 0 ]&&[ -d "$d" ]||fail "tampered absence accepted: $variant";done
rm -f "$absence";cp "$S/absence.good" "$absence";chmod 600 "$absence"
for orphan in orphan.rm-failure orphan.post-rm-query.raw;do printf 'orphan\n'>"$d/evidence/$orphan";chmod 600 "$d/evidence/$orphan";set +e;jc_fl_test_run_delete "$d";rc=$?;set -e;[ "$rc" -ne 0 ]&&[ -d "$d" ]||fail "orphan artifact accepted: $orphan";rm "$d/evidence/$orphan";done
ln -s "$S/source.good" "$d/evidence/orphan.source.json.inspect.1";set +e;jc_fl_test_run_delete "$d";rc=$?;set -e;[ "$rc" -ne 0 ]&&[ -d "$d" ]||fail 'orphan symlink artifact accepted';rm "$d/evidence/orphan.source.json.inspect.1"
jc_fl_test_run_delete "$d";ACTIVE_DIR='';rm -f "$S/source.good" "$S/removal.good" "$S/query.good" "$S/absence.good"
# Controlled listener cleanup uses a protected marker plus PID/start/exe/inode/cmdline fingerprint.
set +e;JC_FL_E2E_STATE_ROOT="$S" JC_FL_E2E_TEST_FOREIGN_LISTENER_AFTER_RELEASE=1 timeout --foreground 240s bash "$ROOT/scripts/fair-launch/local-e2e/start.sh">"$S/out-listener" 2>&1;rc=$?;set -e;[ "$rc" -ne 0 ];d="$(find "$S" -mindepth 1 -maxdepth 1 -type d|head -1)";ACTIVE_DIR="$d";pid="$(cat "$d/mysql-transport/foreign-listener.pid")";marker="$d/mysql-transport/foreign-listener.marker";grep -Fq listener-residue "$d/evidence/start-failure.json";endpoint="$(cat "$d/mysql-transport/foreign-listener.endpoint")";jc_fl_test_process_record "$pid" "$marker" "$endpoint" "$d" "$d/evidence/listener-cleanup.json";ACTIVE_PROCESS="$d/evidence/listener-cleanup.json"
# Tampered marker and mismatched endpoint records must not signal the still-live listener.
for variant in marker endpoint;do stale="$d/evidence/listener-$variant.json";cp "$d/evidence/listener-cleanup.json" "$stale";python3 - "$stale" "$variant" <<'PY'
import json,sys
p,v=sys.argv[1:];o=json.load(open(p));o['recordPath']=p
if v=='marker':o['markerDigest']='0'*64
else:o['endpoint']='127.0.0.1:1';o['socketInode']='0'
open(p,'w').write(json.dumps(o)+'\n')
PY
chmod 600 "$stale";set +e;jc_fl_test_process_remove "$stale";stale_rc=$?;set -e;[ "$stale_rc" -ne 0 ];kill -0 "$pid";done
jc_fl_test_process_remove "$d/evidence/listener-cleanup.json";ACTIVE_PROCESS='';jc_fl_test_run_cleanup "$d";ACTIVE_DIR=''
jc_fl_test_root_finalize "$S";ACTIVE_DIR='';trap - EXIT
echo 'fair_launch_local_e2e_cleanup_adversarial_test: PASS exact immutable fixture cleanup, marker/endpoint identity refusal, and target survival'
