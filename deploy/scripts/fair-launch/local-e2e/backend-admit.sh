#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "$0")"&&pwd)"; . "$HERE/common.sh"
jc_fl_e2e_self_bound backend-admit 60s "$@"; jc_fl_e2e_test_phase_hold backend-admit
[ "$#" -ge 2 ]&&[ "$#" -le 3 ]||jc_fl_e2e_fail 'usage: backend-admit.sh <run-id> <absolute-backend-binary> [0x-transaction-hash]'
jc_fl_e2e_reject_external
jc_fl_e2e_load_state "$1"
jc_fl_e2e_mysql_transport_owned||jc_fl_e2e_fail 'transport ownership mismatch'
jc_fl_e2e_mysql_profiles_ready||jc_fl_e2e_fail 'schema/profile authority mismatch'
binary="$2"; [ "${binary#/}" != "$binary" ]&&[ -f "$binary" ]&&[ ! -L "$binary" ]&&[ -x "$binary" ]||jc_fl_e2e_fail 'Backend binary path invalid'
if [ "$#" = 3 ]; then
  if [ "$3" = wait-apply ]; then
    python3 "$HERE/backend-admission-bundle.py" "$RUN_ID" | "$binary" wait-apply
    exit 0
  fi
  tx_hash="$3"; printf '%s' "$tx_hash"|grep -Eq '^0x[0-9a-f]{64}$'||jc_fl_e2e_fail 'transaction hash or wait-apply invalid'
  python3 "$HERE/backend-admission-bundle.py" "$RUN_ID" | "$binary" apply-tx "$tx_hash"
  exit 0
fi
python3 "$HERE/backend-admission-bundle.py" "$RUN_ID" | "$binary" admit
