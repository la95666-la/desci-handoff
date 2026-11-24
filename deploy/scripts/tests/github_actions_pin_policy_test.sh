#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# No locally verified immutable action SHAs are available in this repository.
# Therefore action-bearing workflows must fail closed before any job can run,
# while still rejecting every mutable tag/branch action reference mechanically.
for workflow in "$ROOT"/.github/workflows/*.yml "$ROOT"/.github/workflows/*.yaml; do
  [ -e "$workflow" ] || continue
  grep -Fq 'if: ${{ false }}' "$workflow" || { echo "workflow is not mechanically archive-disabled: $workflow" >&2; exit 1; }
  if grep -nE 'uses:[[:space:]]+' "$workflow"; then
    echo "external GitHub Action dependency remains without a locally verified immutable SHA: $workflow" >&2
    exit 1
  fi
done
echo 'GitHub Actions immutable-pin fail-closed policy: PASS'
