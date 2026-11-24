#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
fail(){ echo "fair-launch implementation policy: $*" >&2; exit 1; }
for f in "$ROOT/config/fair-launch-v1.env.example" "$ROOT/config/template/frontend.env.tpl" "$ROOT/config/template/admin.env.tpl"; do [ -f "$f" ] || fail "missing $f"; done
grep -Fxq 'JC_FAIR_LAUNCH_ENABLED=0' "$ROOT/config/fair-launch-v1.env.example"
grep -Fxq 'JC_FAIR_LAUNCH_WRITES_ENABLED=0' "$ROOT/config/fair-launch-v1.env.example"
grep -Fxq 'JC_FAIR_LAUNCH_MIGRATIONS_ENABLED=0' "$ROOT/config/fair-launch-v1.env.example"
! grep -Eq '0x[0-9a-fA-F]{40}' "$ROOT/config/fair-launch-v1.env.example" || fail 'live address in config example'
for f in "$ROOT/scripts/fair-launch/lib/gate.sh" "$ROOT/scripts/fair-launch/prepare-deployment.sh" "$ROOT/scripts/fair-launch/prepare-upgrade.sh"; do bash -n "$f"; done
set +e
out=$(bash "$ROOT/scripts/fair-launch/prepare-deployment.sh" 2>&1)
rc=$?
set -e
[ $rc -ne 0 ] || fail 'ungated deployment plan accepted'
[[ $out == *'deployment approval flag is not 1'* ]] || fail 'wrong deployment gate failure'
for f in index.html app.js style.css deployment-manifest.example.json external-anchor.example.json; do [ -f "$ROOT/recovery-portal/$f" ] || fail "missing recovery portal $f"; done
! grep -RqiE 'axios|/admin|/api|mysql|redis' "$ROOT/recovery-portal" || fail 'recovery portal has backend dependency'
! grep -RqiE 'private.?key|mnemonic|seed phrase|eth_sendRawTransaction' "$ROOT/recovery-portal" || fail 'recovery portal contains signer/key path'
for needle in "joincare.release.detached-signature-envelope.v1" "joincare.recovery-portal.deployment-manifest.v1" "joincare.recovery-portal.external-anchor.v1" "Ed25519" "threshold" "validThroughGeneration" "anti-rollback" "payload digest mismatch" "UNADMITTED_PLACEHOLDER" "reviewer trust root is not admitted" "manifestDigest" "anchorDigest" "history does not terminate at immutable checkpoint" "recovery-signed-history.json"; do grep -Fq "$needle" "$ROOT/recovery-portal/app.js" || fail "recovery verifier missing $needle"; done
! grep -Fq 'publicKeyBase64' "$ROOT/recovery-portal/deployment-manifest.example.json" || fail 'manifest-carried key material must not be a trust root'
grep -Fq 'dependencyManifestDigest' "$ROOT/scripts/fair-launch/prepare-deployment.sh" || fail 'deployment plan missing full dependency manifest binding'
grep -Fq 'dependencyManifestDigest' "$ROOT/scripts/fair-launch/prepare-upgrade.sh" || fail 'upgrade plan missing full dependency manifest binding'
node "$ROOT/scripts/fair-launch/verify-dependency-manifest.js" "$ROOT/config/fair-launch-dependency-manifest.example.json" >/dev/null || fail 'dependency manifest invalid'
TMP_DEP=$(mktemp); trap 'rm -f "$TMP_DEP"' EXIT
node - "$ROOT/config/fair-launch-dependency-manifest.example.json" "$TMP_DEP" <<'NODE'
const fs=require('fs');const [src,dst]=process.argv.slice(2);const v=JSON.parse(fs.readFileSync(src));v.components[1].address=v.components[0].address;fs.writeFileSync(dst,JSON.stringify(v));
NODE
if node "$ROOT/scripts/fair-launch/verify-dependency-manifest.js" "$TMP_DEP" >/dev/null 2>&1; then fail 'duplicate dependency address accepted'; fi
node "$ROOT/scripts/tests/recovery_portal_verifier_test.js" "$ROOT"
grep -Fq '部署门禁' "$ROOT/docs/IDO-FAIR-LAUNCH-DEVELOPMENT-DESIGN.md"
grep -Fq '不阻断代码开发' "$ROOT/docs/IDO-FAIR-LAUNCH-DEVELOPMENT-DESIGN.md"
grep -Fq '部署/生产仍为 NO-GO' "$ROOT/docs/IDO-FAIR-LAUNCH-DEVELOPMENT-DESIGN.md"
# Release policy is explicit and never includes migrations/deploy tooling.
! grep -Eq 'migrations|scripts/fair-launch|contracts' "$ROOT/release/manifests/fair-launch-runtime-allowlist.txt" || fail 'unsafe release allowlist'
grep -Fq 'migrations/' "$ROOT/release/manifests/fair-launch-runtime-denylist.txt"
# CI fails closed until independently reviewed production commits are supplied.
for var in FAIR_LAUNCH_REVIEWED_BACKEND_REF FAIR_LAUNCH_REVIEWED_FRONTEND_REF FAIR_LAUNCH_REVIEWED_ADMIN_REF FAIR_LAUNCH_REVIEWED_CONTRACT_REF; do grep -Fq "$var" "$ROOT/.github/workflows/build.yml" || fail "CI missing required reviewed ref $var"; done
for stale in 00ecde91f5d231893f715851a981282c3b666a2f be90c37be3c39664ea3a273fed31eb6b95bc05f3 06f131f5dd2ef6427546e84f0cd63ce9ba747658 07742d94c06e4a514441d14396fd04a90c6c195c; do ! grep -Fq "$stale" "$ROOT/.github/workflows/build.yml" || fail "stale Phase-1 ref blessed by CI: $stale"; done
grep -Fq 'is not admitted' "$ROOT/.github/workflows/build.yml" || fail 'CI reviewed-ref fail-closed gate missing'
for input in backend_sha frontend_sha admin_sha contract_sha; do grep -Fq "$input:" "$ROOT/.github/workflows/release.yml" || fail "release input missing: $input"; done
! grep -Eq "default: '[0-9a-f]{40}'" "$ROOT/.github/workflows/release.yml" || fail 'manual release embeds stale/default commit SHA'
for var in FAIR_LAUNCH_REVIEWED_BACKEND_REF FAIR_LAUNCH_REVIEWED_FRONTEND_REF FAIR_LAUNCH_REVIEWED_ADMIN_REF FAIR_LAUNCH_REVIEWED_CONTRACT_REF; do grep -Fq "$var" "$ROOT/.github/workflows/release.yml" || fail "manual release does not bind $var"; done
grep -Fq 'do not match the four admitted immutable refs' "$ROOT/.github/workflows/release.yml" || fail 'manual release admitted-ref mismatch rejection missing'
# Project #5 guides are out of scope and must not be staged by this work.
if git -C "$ROOT" diff --cached --name-only | grep -Fq 'PROJECT-5-MANAGER-TEST-GUIDE'; then fail 'Project #5 guide staged'; fi
echo 'Fair Launch implementation/deployment-boundary policy: PASS'
