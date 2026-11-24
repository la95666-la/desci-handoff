#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "$0")"&&pwd)"; . "$HERE/common.sh"
jc_fl_e2e_self_bound bootstrap-runtime 240s "$@"; jc_fl_e2e_test_phase_hold bootstrap-runtime
[ "$#" = 1 ]||jc_fl_e2e_fail 'usage: bootstrap-runtime.sh <run-id>'
jc_fl_e2e_reject_external
jc_fl_e2e_load_state "$1"
jc_fl_e2e_mysql_transport_owned||jc_fl_e2e_fail 'transport ownership mismatch'
jc_fl_e2e_mysql_profiles_ready||jc_fl_e2e_fail 'schema/profile authority mismatch'
[ -f "$RUN_DIR/evidence/contract-runtime-admission-v1.json" ]||jc_fl_e2e_fail 'Contract runtime admission missing'
exec python3 "$HERE/bootstrap-runtime.py" "$RUN_ID"
