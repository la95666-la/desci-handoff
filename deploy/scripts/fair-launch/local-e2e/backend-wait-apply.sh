#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "$0")"&&pwd)"; . "$HERE/common.sh"
jc_fl_e2e_self_bound backend-wait-apply 240s "$@"; jc_fl_e2e_test_phase_hold backend-wait-apply
[ "$#" -ge 2 ]&&[ "$#" -le 3 ]||jc_fl_e2e_fail 'usage: backend-wait-apply.sh <run-id> <absolute-backend-binary> [replay|next|fence-loss|project-tamper|position-tamper|attestation-tamper|archive-retry|sequence-N]'
jc_fl_e2e_reject_external
jc_fl_e2e_load_state "$1"
jc_fl_e2e_mysql_transport_owned||jc_fl_e2e_fail 'transport ownership mismatch'
jc_fl_e2e_mysql_profiles_ready||jc_fl_e2e_fail 'schema/profile authority mismatch'
binary="$2"; [ "${binary#/}" != "$binary" ]&&[ -f "$binary" ]&&[ ! -L "$binary" ]&&[ -x "$binary" ]||jc_fl_e2e_fail 'Backend binary path invalid'
mode=wait-apply
mode_arg=
if [ "$#" = 3 ]; then
  case "$3" in
    replay) mode=wait-apply-replay;;
    next) mode=wait-apply-next;;
    fence-loss) mode=wait-apply-fence-loss;;
    project-tamper) mode=wait-apply-project-tamper;;
    position-tamper) mode=wait-apply-position-tamper;;
    attestation-tamper) mode=wait-apply-attestation-tamper;;
    archive-retry) mode=wait-apply-archive-retry;;
    sequence-[2-8]) mode=wait-apply-sequence; mode_arg="${3#sequence-}";;
    *) jc_fl_e2e_fail 'backend wait mode invalid';;
  esac
fi
if [ -n "$mode_arg" ]; then
  python3 "$HERE/backend-admission-bundle.py" "$RUN_ID" | "$binary" "$mode" "$mode_arg"
else
  python3 "$HERE/backend-admission-bundle.py" "$RUN_ID" | "$binary" "$mode"
fi
