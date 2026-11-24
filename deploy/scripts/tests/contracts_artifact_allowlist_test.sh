#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
[ ! -d "$ROOT/artifacts/contracts" ] || { echo 'stale contract mutation artifacts remain' >&2; exit 1; }
for manifest in "$ROOT/release/manifests/contracts-mainnet.list" "$ROOT/release/manifests/contracts-testnet.list"; do
  ! grep -Ev '^[[:space:]]*(#|$)' "$manifest" | grep -q . || { echo "deployable contract manifest remains: $manifest" >&2; exit 1; }
done
printf '%s\n' 'contract deployment artifacts unavailable: PASS'
