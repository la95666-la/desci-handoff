#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "$0")"&&pwd)"; . "$HERE/common.sh"
[ "$#" -ge 1 ] || { echo 'usage: phase.sh start|apply-schema|bootstrap-runtime|backend-admit|backend-serve|backend-wait-apply|status|teardown [args]' >&2; exit 2; }
phase="$1";shift
case "$phase" in
 start) script=start.sh; bound=240s;;
 apply-schema) script=apply-schema.sh; bound=280s;;
 bootstrap-runtime) script=bootstrap-runtime.sh; bound=240s;;
 backend-admit) script=backend-admit.sh; bound=60s;;
 backend-serve) script=backend-serve.sh;;
 backend-wait-apply) script=backend-wait-apply.sh; bound=240s;;
 status) script=status.sh; bound=60s;;
 teardown) script=teardown.sh; bound=120s;;
 *) echo 'unsupported phase' >&2;exit 2;;
esac
script_path="$(readlink -f "$HERE/$script")"
if [ "$phase" = backend-serve ]; then
 exec bash "$script_path" "$@"
fi
timeout_bin="$(jc_fl_e2e_trusted_timeout)"||jc_fl_e2e_fail 'trusted system timeout unavailable'
export JC_FL_E2E_PHASE_ACTIVE="$phase"
exec "$timeout_bin" --foreground --kill-after=5s "$bound" bash "$script_path" "$@"
