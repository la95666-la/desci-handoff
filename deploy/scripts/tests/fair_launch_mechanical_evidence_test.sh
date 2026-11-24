#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
EVIDENCE="$ROOT/evidence/fair-launch"
CONTRACT_REPO="${FAIR_LAUNCH_CONTRACT_REPO:-$ROOT/../joincare_code/joincare_code/_extracted/JoinCare_SmartContract-main}"

test -f "$EVIDENCE/contract-baseline/provenance.json"
test -f "$EVIDENCE/ce-v2-governance/schema.sql.txt"
test -f "$EVIDENCE/phase1-project-read-model/schema.sql.txt"
test -f "$EVIDENCE/release-rfc8785-v1/tools/verify.js"

echo '== contract clean-build baseline and independent golden diff =='
(cd "$CONTRACT_REPO" && bash scripts/audit/verifyFairLaunchBaseline.sh "$EVIDENCE/contract-baseline")
grep -Fq 'AUDIT_ONLY_NON_PRODUCTION_SPECIMEN' "$EVIDENCE/contract-baseline/provenance.json"
! grep -R -Fq 'JoinCareFairLaunchV1ArtifactBaseline' "$ROOT/release/manifests/contracts-testnet.list" "$ROOT/release/manifests/contracts-mainnet.list"

echo '== mechanically inert CE-V2/governance schema and vectors =='
DDL="$EVIDENCE/ce-v2-governance/schema.sql.txt"
case "$DDL" in *.sql.txt) ;; *) echo 'draft DDL must use .sql.txt' >&2; exit 1 ;; esac
# Executable migrations may now exist, but the frozen specimen remains inert and
# the implementation migration gate verifies those files separately.
awk 'NR>2 && NF && $0 !~ /^SPEC> / {print "executable-shaped DDL line " NR ": " $0; bad=1} END{exit bad}' "$DDL"
PHASE1_DDL="$EVIDENCE/phase1-project-read-model/schema.sql.txt"
case "$PHASE1_DDL" in *.sql.txt) ;; *) echo 'phase1 draft DDL must use .sql.txt' >&2; exit 1 ;; esac
grep -Fqx 'DO_NOT_EXECUTE: FAIR LAUNCH PHASE-1 PROJECT DISCRIMINATOR / READ-MODEL SPECIMEN ONLY' "$PHASE1_DDL"
awk 'NR>2 && NF && $0 !~ /^SPEC> / {print "executable-shaped phase1 DDL line " NR ": " $0; bad=1} END{exit bad}' "$PHASE1_DDL"
CE_NODE_PATH="$CONTRACT_REPO/node_modules"
NODE_PATH="$CE_NODE_PATH" node "$EVIDENCE/ce-v2-governance/verify.js" "$EVIDENCE/ce-v2-governance"

echo '== detached RFC 8785 records and real threshold signatures =='
RFC_ROOT="$EVIDENCE/release-rfc8785-v1"
node "$RFC_ROOT/tools/verify.js" "$RFC_ROOT"
RFC_TMP="$(mktemp -d)"
trap 'rm -rf "$RFC_TMP"' EXIT
node "$RFC_ROOT/tools/generate.js" "$RFC_TMP" >/dev/null
node "$RFC_ROOT/tools/verify.js" "$RFC_TMP"
diff -ru --exclude tools --exclude README.md "$RFC_ROOT" "$RFC_TMP"
if grep -R -Fq 'joincare.release.image-capability.v1' "$ROOT/release/out" 2>/dev/null; then
  echo 'release/out contains detached capability material' >&2
  exit 1
fi

echo 'Fair Launch mechanical evidence: PASS'
