#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
for args in 'frontend' 'admin --tail 10' 'backend' 'frontend;id'; do
  set +e; out="$(env JC_LIB=/missing JC_ENV_FILE=/missing.env bash "$ROOT/scripts/cmd/logs.sh" $args 2>&1)"; rc=$?; set -e
  [ "$rc" -eq 78 ] && [ "$out" = service_log_transport_unavailable ] || { echo "logs $args: rc=$rc out=$out" >&2; exit 1; }
done
! grep -Ev '^[[:space:]]*#' "$ROOT/scripts/cmd/logs.sh" | grep -Eq 'load_env|remote_|ssh|docker|compose'
echo 'Fair Launch shell log transport unavailable before environment read: PASS'
