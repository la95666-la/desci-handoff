#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
source "$ROOT/lib/runtime-image.sh"
set +e
out="$(require_hardhat_runtime_image 2>&1)"; rc=$?
set -e
[ "$rc" -eq 78 ] && [ "$out" = hardhat_operational_target_unavailable ]
for file in "$ROOT/../deploy/Dockerfile.hardhat" "$ROOT/../deploy/Dockerfile.hardhat.runtime" "$ROOT/../deploy/hardhat-entrypoint.sh"; do
  grep -Fq hardhat_operational_target_unavailable "$file"
done
printf '%s\n' 'Hardhat operational runtime unavailable: PASS'
