#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MIG="$ROOT/migrations/2026_09_14_fair_launch_mainnet_bootstrap_receipt.sql"
fail(){ echo "fair_launch_mainnet_bootstrap_schema_test: FAIL: $*" >&2; exit 1; }

[ -f "$MIG" ] || fail '0914 migration missing'
grep -Fq '@JC_FAIR_LAUNCH_MAINNET_BOOTSTRAP_APPROVED' "$MIG" || fail 'explicit bootstrap migration gate missing'
grep -Fq 'fair_launch_mainnet_project_bootstrap_receipt_v1' "$MIG" || fail 'bootstrap receipt table missing'
grep -Fq 'bootstrap_receipt_id' "$MIG" || fail 'project state receipt binding missing'
grep -Fq 'chain_id=56 AND bootstrap_receipt_id IS NOT NULL' "$MIG" || fail 'mainnet-only bootstrap provenance branch missing'
grep -Fq 'chain_id=31337 AND bootstrap_receipt_id IS NULL' "$MIG" || fail 'local provenance branch weakened'
grep -Fq 'p.canonical_slot=1 AND p.finalized_at IS NOT NULL AND p.noncanonical_at IS NULL' "$MIG" || fail 'canonical finalized placement validation missing'
grep -Fq 'o.placement_id=p.id' "$MIG" || fail 'observation placement binding missing'
grep -Fq 'Fair Launch mainnet project bootstrap receipt evidence mismatch' "$MIG" || fail 'receipt evidence validation trigger missing'
grep -Fq 'Fair Launch mainnet project bootstrap receipt mismatch' "$MIG" || fail 'state-to-receipt validation trigger missing'
grep -Fq 'is append-only' "$MIG" || fail 'receipt append-only protection missing'
! grep -Eq 'PRIVATE_KEY|private[_-]?key|mnemonic|raw[_-]?transaction' "$MIG" || fail 'secret/signing material field leaked into schema'

line=$(grep -F '*2026_09_14_fair_launch_mainnet_bootstrap_receipt.sql' "$ROOT/migrations/FAIR_LAUNCH_SHA256SUMS" || true)
[ -n "$line" ] || fail '0914 checksum binding missing'
actual=$(sha256sum "$MIG" | awk '{print $1}')
[ "${line:0:64}" = "$actual" ] || fail '0914 checksum mismatch'

for frozen in "$ROOT"/migrations/2026_09_{08,09,10,11,12}_fair_launch_*.sql; do
  git -C "$ROOT" diff --quiet -- "$frozen" || fail "frozen migration modified: ${frozen##*/}"
done

echo 'Fair Launch mainnet bootstrap receipt schema static validation: PASS'
