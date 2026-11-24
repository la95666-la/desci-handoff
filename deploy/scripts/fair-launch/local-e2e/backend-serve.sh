#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "$0")"&&pwd)"; . "$HERE/common.sh"
jc_fl_e2e_test_phase_hold backend-serve
jc_fl_e2e_trusted_timeout >/dev/null||jc_fl_e2e_fail 'trusted system timeout unavailable'
[ "$#" -ge 4 ]&&[ "$#" -le 5 ]||jc_fl_e2e_fail 'usage: backend-serve.sh <run-id> <absolute-backend-binary> <frontend-origin> <admin-origin> [127.0.0.1:0]'
jc_fl_e2e_reject_external
jc_fl_e2e_load_state "$1"
jc_fl_e2e_mysql_transport_owned||jc_fl_e2e_fail 'transport ownership mismatch'
jc_fl_e2e_mysql_profiles_ready||jc_fl_e2e_fail 'schema/profile authority mismatch'
binary="$2"; [ "${binary#/}" != "$binary" ]&&[ -f "$binary" ]&&[ ! -L "$binary" ]&&[ -x "$binary" ]||jc_fl_e2e_fail 'Backend binary path invalid'
printf '%s\n' "$3" "$4"|grep -Eq '^http://127\.0\.0\.1:[1-9][0-9]{0,4}$'||jc_fl_e2e_fail 'frontend/admin origin invalid'
[ "$3" != "$4" ]||jc_fl_e2e_fail 'frontend/admin origins must differ'
bind="${5:-127.0.0.1:0}"
printf '%s' "$bind"|grep -Eq '^127\.0\.0\.1:(0|[1-9][0-9]{0,4})$'||jc_fl_e2e_fail 'Backend bind address invalid'
python3 "$HERE/backend-admission-bundle.py" "$RUN_ID" | "$binary" serve "$3" "$4" "$bind"
