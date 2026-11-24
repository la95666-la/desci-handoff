#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"; MANIFEST="$ROOT/config/fair-launch-dependency-manifest.example.json"
fail(){ echo "fair-launch dependency manifest: $*" >&2; exit 1; }
node "$ROOT/scripts/fair-launch/verify-dependency-manifest.js" "$MANIFEST" >/dev/null
for poison in 0 1 yes; do if JC_FAIR_LAUNCH_TEST_AUTHORITY="$poison" node "$ROOT/scripts/fair-launch/verify-dependency-manifest.js" "$MANIFEST" --require-admission >/dev/null 2>&1; then fail "production verifier accepted test authority poison $poison"; fi; done
if JC_FAIR_LAUNCH_TEST_AUTHORITY=1 bash "$ROOT/scripts/fair-launch/prepare-deployment.sh" /tmp/forbidden-plan.json >/dev/null 2>&1; then fail 'deployment entrypoint accepted test authority poison'; fi
if JC_FAIR_LAUNCH_TEST_AUTHORITY=1 bash "$ROOT/scripts/fair-launch/prepare-upgrade.sh" /tmp/forbidden-upgrade.json >/dev/null 2>&1; then fail 'upgrade entrypoint accepted test authority poison'; fi
# Production admission is intentionally impossible while the independently
# distributed authority root remains UNADMITTED_PLACEHOLDER.
if node "$ROOT/scripts/fair-launch/verify-dependency-manifest.js" "$MANIFEST" --require-admission >/dev/null 2>&1; then fail 'unadmitted production dependency authority unexpectedly passed'; fi
grep -Fq "status:'UNADMITTED_PLACEHOLDER'" "$ROOT/scripts/fair-launch/signed-authority.js"||fail 'production authority must remain unadmitted'
# Test-only adapter proves fixtures without creating a production switch.
TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT
DIGEST=$(node "$ROOT/scripts/fair-launch/verify-dependency-manifest.js" "$MANIFEST")
node - "$TMP/a.json" "$DIGEST" <<'NODE'
const fs=require('fs'),{jcs}=require('./scripts/fair-launch/verify-dependency-manifest');fs.writeFileSync(process.argv[2],jcs({domain:'joincare.fair-launch.reviewer-release-authority.v1',generation:1,status:'REVIEWER_ADMITTED',subjects:{dependencyManifest:process.argv[3]}}));
NODE
chmod 600 "$TMP/a.json"
NODE_ENV=test JC_FAIR_LAUNCH_TEST_AUTHORITY=1 node "$ROOT/scripts/tests/fair-launch-test-authority.js" authority "$TMP/a.json" dependencyManifest "$DIGEST" >/dev/null
if node -e "require(process.argv[1])" "$ROOT/scripts/tests/fair-launch-test-authority.js" >/dev/null 2>&1; then fail 'test adapter was importable'; fi
! grep -R -Fq "fair-launch-test-authority" "$ROOT/scripts/fair-launch" "$ROOT/scripts/cmd" "$ROOT/scripts/release"||fail 'production scripts reference test authority adapter'
echo 'Fair Launch production authority poison rejection and test-only fixture adapter: PASS'
