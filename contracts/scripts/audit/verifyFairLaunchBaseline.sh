#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
EVIDENCE="${1:-$ROOT/../../../../joincare-deploy/evidence/fair-launch/contract-baseline}"
DO_NPM_CI="${FAIR_LAUNCH_AUDIT_NPM_CI:-0}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

cd "$ROOT"
test "$(node -p "require('./package-lock.json').lockfileVersion")" = 3
if [[ "$DO_NPM_CI" == 1 ]]; then
  npm ci --ignore-scripts --no-audit --no-fund
fi
rm -rf artifacts-audit cache-audit
node node_modules/hardhat/internal/cli/cli.js compile --config hardhat.audit.config.js --force
node scripts/audit/generateFairLaunchBaseline.js "$TMP"
node scripts/audit/checkFairLaunchBaseline.js "$TMP"
diff -ru --strip-trailing-cr "$EVIDENCE" "$TMP"
echo 'Fair Launch clean-build reproducibility: PASS'
