#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
EXAMPLE="$ROOT/config/fair-launch-phase1.env.example"
DRAFT="$ROOT/evidence/fair-launch/phase1-project-read-model/schema.sql.txt"

fail() { echo "fair-launch phase1 policy: $*" >&2; exit 1; }

[ -f "$EXAMPLE" ] || fail "missing disabled example config"
[ -f "$DRAFT" ] || fail "missing read-model DDL specimen"
case "$DRAFT" in *.sql.txt) ;; *) fail "DDL specimen must use .sql.txt" ;; esac

grep -Fxq 'DO_NOT_EXECUTE: FAIR LAUNCH PHASE-1 PROJECT DISCRIMINATOR / READ-MODEL SPECIMEN ONLY' "$DRAFT"
awk 'NR>2 && NF && $0 !~ /^SPEC> / {print "executable-shaped DDL line " NR ": " $0; bad=1} END{exit bad}' "$DRAFT"
bash "$ROOT/scripts/tests/fair_launch_migration_static_test.sh" >/dev/null

for expected in \
  'JC_FAIR_LAUNCH_PHASE1_ENABLED=0' \
  'JC_FAIR_LAUNCH_WRITES_ENABLED=0' \
  'JC_FAIR_LAUNCH_CHAIN_ID=0' \
  'JC_FAIR_LAUNCH_PROXY_ADDRESS=' \
  'JC_FAIR_LAUNCH_IMPLEMENTATION_ADDRESS=' \
  'JC_FAIR_LAUNCH_OPERATOR_ADDRESS=' \
  'JC_FAIR_LAUNCH_GOVERNANCE_TIMELOCK_ADDRESS=' \
  'JC_FAIR_LAUNCH_START_BLOCK=0' \
  'JC_FAIR_LAUNCH_SOURCE_REGISTRY_REVISION=' \
  'JC_FAIR_LAUNCH_DEPLOYMENT_ATTESTATION_DIGEST='; do
  grep -Fxq "$expected" "$EXAMPLE" || fail "unsafe/missing example value: $expected"
done

# Runtime templates must expose only the disabled placeholder and renderer must
# reject attempts to turn it into an active source/writer.
grep -Fq 'enabled: ${JC_FAIR_LAUNCH_ENABLED_YAML}' "$ROOT/config/template/core.yaml.tpl"
grep -Fq 'writesEnabled: ${JC_FAIR_LAUNCH_WRITES_ENABLED_YAML}' "$ROOT/config/template/core.yaml.tpl"
grep -Fq 'enabled: ${JC_FAIR_LAUNCH_ENABLED_YAML}' "$ROOT/config/template/job.yaml.tpl"
grep -Fq 'remote_config_upload_unavailable' "$ROOT/scripts/cmd/render_config.sh"
grep -Fq 'fair_launch_activation_unavailable' "$ROOT/scripts/cmd/install.sh"

# Executable migrations/offline scaffolding are now allowed, but no release
# profile, runtime output or contract allowlist may enable Fair Launch.
for path in \
  "$ROOT/scripts/deploy" \
  "$ROOT/scripts/upgrade" \
  "$ROOT/release/manifests/contracts-mainnet.list" \
  "$ROOT/release/manifests/contracts-testnet.list" \
  "$ROOT/release/out"; do
  [ -e "$path" ] || continue
  if grep -RIlE 'JoinCareFairLaunch|fair_launch_v1|JC_FAIR_LAUNCH_' "$path" 2>/dev/null | grep -v '/evidence/' | grep -q .; then
    fail "forbidden production-shaped Fair Launch artifact under ${path#$ROOT/}"
  fi
done

# An address-shaped value next to a Fair Launch key is forbidden outside inert
# evidence and tests. Zero/unset placeholders are allowed.
if grep -RInE "JC_FAIR_LAUNCH_[A-Z_]*(ADDRESS|CONTRACT|OPERATOR|TIMELOCK)[A-Z_]*[=:][[:space:]]*['\"]?0x[0-9a-fA-F]{40}" \
  "$ROOT/config" "$ROOT/scripts" "$ROOT/release" "$ROOT/.github" 2>/dev/null | grep -v '/tests/' | grep -q .; then
  fail "live Fair Launch address found"
fi

# Audit specimen must remain outside every production contract release allowlist.
! grep -R -Fq 'JoinCareFairLaunchV1ArtifactBaseline' \
  "$ROOT/release/manifests/contracts-testnet.list" \
  "$ROOT/release/manifests/contracts-mainnet.list" || fail "audit specimen is release-allowlisted"

# Current Phase-1 documentation must preserve the bounded gate distinction.
grep -Fq '候选技术 Freeze：FROZEN / CONDITIONAL GO' "$ROOT/docs/IDO-FAIR-LAUNCH-DEVELOPMENT-DESIGN.md"
grep -Fq '当前总体结论：NO-GO' "$ROOT/docs/IDO-FAIR-LAUNCH-DEVELOPMENT-DESIGN.md"

# CI is deliberately non-operational until immutable third-party Action SHAs and
# reviewed cross-repository refs are admitted. It must not pretend to provision an
# audit checkout while every job is mechanically disabled.
grep -Fq 'CURRENT RELEASE: mechanically disabled' "$ROOT/.github/workflows/build.yml"
grep -Fq 'CONTRACT_REF: ${{ vars.FAIR_LAUNCH_REVIEWED_CONTRACT_REF }}' "$ROOT/.github/workflows/build.yml"
grep -Fq 'FAIR_LAUNCH_REVIEWED_CONTRACT_REF is not admitted' "$ROOT/.github/workflows/build.yml"
grep -Fq 'github_action_dependency_unadmitted' "$ROOT/.github/workflows/build.yml"
! grep -Eq '^[[:space:]]+uses:' "$ROOT/.github/workflows/build.yml" || fail 'disabled workflow retains mutable external action execution'

echo 'Fair Launch Phase-1 non-production policy: PASS'
