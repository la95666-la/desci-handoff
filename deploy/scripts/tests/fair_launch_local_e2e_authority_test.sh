#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.."&&pwd)";. "$ROOT/scripts/tests/fair_launch_local_e2e_test_cleanup.sh";BASE="$(mktemp -d)";chmod 700 "$BASE";LIVE_RECORD='';LIVE_PID=''
fail(){ echo "authority_test: FAIL: $*" >&2;exit 1;}
cleanup(){ local original=$?;set +e;if [ -n "$LIVE_RECORD" ]&&[ -f "$LIVE_RECORD" ];then jc_fl_test_process_remove "$LIVE_RECORD" TERM||true;fi;rm -rf "$BASE";return "$original";};trap cleanup EXIT
refuse(){ set +e;"$@";rc=$?;set -e;[ "$rc" -ne 0 ]||fail "expected refusal: $*";}
# Missing Redis authority with launch-attempted state never authorizes absence or deletion.
R="$(jc_fl_test_root_create JC_FL_TEST_AUTHORITY_LIVE_CWD_V1)";run="$(cat /proc/sys/kernel/random/uuid)";D="$R/$run";mkdir -p "$D/redis" "$D/evidence";chmod 700 "$D" "$D/redis" "$D/evidence";jc_fl_e2e_phase_write "$D";jc_fl_e2e_phase_update "$D" redis-launch-attempted true false
marker="$D/evidence/harmless.marker";printf 'owned harmless cwd fixture\n'>"$marker";chmod 600 "$marker";(cd "$D/redis"&&sleep 120)&LIVE_PID=$!;for _ in $(seq 1 50);do [ "$(readlink "/proc/$LIVE_PID/cwd" 2>/dev/null)" = "$D/redis" ]&&break;sleep .05;done;LIVE_RECORD="$D/evidence/harmless-process.json";jc_fl_test_process_record "$LIVE_PID" "$marker" none "$D" "$LIVE_RECORD"
refuse jc_fl_test_run_absent "$D";refuse jc_fl_test_run_delete "$D";refuse jc_fl_test_run_cleanup "$D";refuse jc_fl_test_root_finalize "$R";kill -0 "$LIVE_PID";[ -d "$D/redis" ]&&[ -f "$LIVE_RECORD" ]||fail retained-authority
old_pid="$LIVE_PID";jc_fl_test_process_remove "$LIVE_RECORD" TERM;LIVE_RECORD='';LIVE_PID='';[ ! -r "/proc/$old_pid/stat" ]||fail harmless-process-residue;rm -rf "$R"
# Legitimate immutable pre-launch state permits cleanup.
R="$(jc_fl_test_root_create JC_FL_TEST_AUTHORITY_NEVER_LAUNCHED_V1)";run="$(cat /proc/sys/kernel/random/uuid)";D="$R/$run";mkdir -p "$D/redis" "$D/evidence";chmod 700 "$D" "$D/redis" "$D/evidence";jc_fl_e2e_phase_write "$D";jc_fl_test_run_cleanup "$D";jc_fl_test_root_finalize "$R";[ ! -e "$R" ]||fail never-launched-root
# Root marker and root-object attacks refuse without touching sentinels.
T="$(jc_fl_test_root_create JC_FL_TEST_AUTHORITY_ROOT_V1)";echo sentinel>"$T/sentinel";ln -s "$T" "$BASE/root-link";refuse jc_fl_test_root_finalize "$BASE/root-link";[ -f "$T/sentinel" ]||fail symlink-target
chmod 755 "$T";refuse jc_fl_test_root_finalize "$T";[ -f "$T/sentinel" ]||fail mode-target;chmod 700 "$T"
cp "$T/.jc-fl-test-root.json" "$BASE/marker-copy";python3 - "$T/.jc-fl-test-root.json" <<'PY'
import json,sys
p=sys.argv[1];o=json.load(open(p));o['rootInode']+=1;open(p,'w').write(json.dumps(o)+'\n')
PY
chmod 600 "$T/.jc-fl-test-root.json";refuse jc_fl_test_root_finalize "$T";[ -f "$T/sentinel" ]||fail marker-target;mv "$BASE/marker-copy" "$T/.jc-fl-test-root.json";chmod 600 "$T/.jc-fl-test-root.json"
U="$BASE/unmarked";mkdir "$U";chmod 700 "$U";echo sentinel>"$U/sentinel";refuse jc_fl_test_root_finalize "$U";[ -f "$U/sentinel" ]||fail unmarked-target
Q="$(jc_fl_test_root_create JC_FL_TEST_AUTHORITY_INODE_V1)";echo original>"$Q/sentinel";mv "$Q" "$Q.old";mkdir "$Q";chmod 700 "$Q";cp "$Q.old/.jc-fl-test-root.json" "$Q/.jc-fl-test-root.json";chmod 600 "$Q/.jc-fl-test-root.json";echo replacement>"$Q/replacement-sentinel";refuse jc_fl_test_root_finalize "$Q";[ -f "$Q/replacement-sentinel" ]&&[ -f "$Q.old/sentinel" ]||fail inode-replacement-target;rm -rf "$Q" "$Q.old"
mkdir "$T/not-a-run";refuse jc_fl_test_root_finalize "$T";[ -f "$T/sentinel" ]||fail child-target;rmdir "$T/not-a-run"
ln -s "$BASE" "$T/child-link";refuse jc_fl_test_root_finalize "$T";[ -f "$T/sentinel" ]||fail child-link-target;rm "$T/child-link"
# Duplicate/unknown/weakened/relocated phase authority refuses.
run="$(cat /proc/sys/kernel/random/uuid)";D="$T/$run";mkdir -p "$D/evidence";chmod 700 "$D" "$D/evidence";jc_fl_e2e_phase_write "$D";P="$D/evidence/lifecycle-phase.json";cp "$P" "$BASE/phase-good"
refuse jc_fl_e2e_phase_write "$D";cmp -s "$P" "$BASE/phase-good"||fail phase-overwrite-mutated
mkdir -p "$D/redis";chmod 700 "$D/redis";printf 'port 16379\n' >"$D/redis/redis.conf";chmod 600 "$D/redis/redis.conf";printf '%s\n' "$$">"$D/redis/redis.pid";chmod 600 "$D/redis/redis.pid";refuse jc_fl_test_redis_publish_recovery_authority "$D";[ ! -e "$D/redis/process.json" ]||fail prelaunch-recovery-authority
jc_fl_e2e_phase_update "$D" redis-launch-attempted true false;cp "$BASE/phase-good" "$P";chmod 600 "$P";phase_state="$(python3 "$JC_FL_TEST_AUTHORITY_PY" phase-verify "$P")";printf '%s\n' "$phase_state"|grep -Fxq REDIS_ATTEMPTED=true||fail phase-ledger-rollback
python3 - "$P" <<'PY'
import sys
p=sys.argv[1];s=open(p).read().rstrip();open(p,'w').write(s[:-1]+',"stage":"scaffolded"}\n')
PY
refuse python3 "$JC_FL_TEST_AUTHORITY_PY" phase-verify "$P";cp "$BASE/phase-good" "$P";python3 - "$P" <<'PY'
import json,sys
p=sys.argv[1];o=json.load(open(p));o['unknown']=1;open(p,'w').write(json.dumps(o)+'\n')
PY
refuse python3 "$JC_FL_TEST_AUTHORITY_PY" phase-verify "$P";cp "$BASE/phase-good" "$P";python3 - "$P" <<'PY'
import json,sys
p=sys.argv[1];o=json.load(open(p));o['redisLaunchAttempted']=True;open(p,'w').write(json.dumps(o)+'\n')
PY
chmod 600 "$P";refuse python3 "$JC_FL_TEST_AUTHORITY_PY" phase-verify "$P";cp "$BASE/phase-good" "$P";chmod 644 "$P";refuse python3 "$JC_FL_TEST_AUTHORITY_PY" phase-verify "$P";chmod 600 "$P";cp "$P" "$D/phase-relocated.json";refuse python3 "$JC_FL_TEST_AUTHORITY_PY" phase-verify "$D/phase-relocated.json"
rm -rf "$T" "$U"
trap - EXIT;rm -rf "$BASE"
echo 'fair_launch_local_e2e_authority_test: PASS missing-authority live-cwd refusal, never-launched cleanup, strict phase records, and root-object refusal'
