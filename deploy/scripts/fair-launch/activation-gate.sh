#!/usr/bin/env bash
# Single production entrypoint before Backend/Job/Lang Apply, Rollback, start,
# restart or health-start. Production activation is intentionally unavailable
# until an admitted DB auditor and a MySQL advisory lease held through startup
# are implemented and independently reviewed.
set -euo pipefail
[ "${1:-verify}" = verify ] || { echo 'usage: activation-gate.sh verify' >&2; exit 2; }
for name in $(env | cut -d= -f1); do
  [[ "$name" =~ ^[Jj][Cc]_[Ff][Aa][Ii][Rr]_[Ll][Aa][Uu][Nn][Cc][Hh]_[Tt][Ee][Ss][Tt]_ ]] || continue
  echo 'fair_launch_activation_unavailable' >&2
  exit 78
done
echo 'fair_launch_activation_unavailable' >&2
exit 78
