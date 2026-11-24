#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MIG="$ROOT/migrations/2026_09_13_fair_launch_bsc_mainnet_activation.sql"
fail(){ echo "fair_launch_mainnet_activation_schema_test: FAIL: $*" >&2; exit 1; }

[ -f "$MIG" ] || fail '0913 migration missing'
grep -Fq '@JC_FAIR_LAUNCH_MAINNET_ACTIVATION_APPROVED' "$MIG" || fail 'explicit migration gate missing'
grep -Fq "environment='test-server' AND chain_id=56" "$MIG" || fail 'exact environment/chain binding missing'
grep -Fq "token_decimals=18" "$MIG" || fail 'BSC payment-token decimals are not pinned to 18'
grep -Fq 'fair_launch_authority_v1' "$MIG" || fail 'common V3 authority root missing'
grep -Fq 'fair_launch_mainnet_activation_lease_v1' "$MIG" || fail 'bounded activation lease missing'
grep -Fq 'fair_launch_mainnet_activation_history_v1' "$MIG" || fail 'append-only activation history missing'
grep -Fq 'fair_launch_mainnet_component_binding_v1' "$MIG" || fail 'release component binding missing'
grep -Fq 'fair_launch_mainnet_project_admission_v1' "$MIG" || fail 'project/value-cap admission missing'
grep -Fq 'fair_launch_mainnet_watermark_v1' "$MIG" || fail 'irreversible feature watermark missing'
grep -Fq 'fk_fl_local_run_authority' "$MIG" || fail 'historical local run is not retained under common authority root'
grep -Fq "authority_kind='local_e2e' AND environment='local-e2e' AND chain_id=31337" "$MIG" || fail 'LOCAL-ONLY authority identity weakened'
grep -Fq "authority_kind='test_server_bsc_mainnet' AND environment='test-server' AND chain_id=56" "$MIG" || fail 'mainnet authority identity is not exact'
grep -Fq "status IN('active','revoked','expired')" "$MIG" || fail 'lease lifecycle constraint missing'
grep -Fq "component IN('contract','deploy','backend','job','frontend','admin','release_tool','migration_bundle')" "$MIG" || fail 'component allowlist missing'
grep -Fq "token_symbol IN('USDT','USDC')" "$MIG" || fail 'payment-token symbol allowlist missing'
grep -Fq "wallet_submission_enabled" "$MIG" || fail 'wallet enablement watermark missing'
grep -Fq 'Fair Launch mainnet project binding is not admitted' "$MIG" || fail 'project binding trigger missing'
grep -Fq 'Fair Launch mainnet source token is not admitted' "$MIG" || fail 'token binding trigger missing'
! grep -Eq 'PRIVATE_KEY|private[_-]?key|mnemonic|raw[_-]?transaction' "$MIG" || fail 'secret/signing material field leaked into schema'

line=$(grep -F '*2026_09_13_fair_launch_bsc_mainnet_activation.sql' "$ROOT/migrations/FAIR_LAUNCH_SHA256SUMS" || true)
[ -n "$line" ] || fail '0913 checksum binding missing'
actual=$(sha256sum "$MIG" | awk '{print $1}')
[ "${line:0:64}" = "$actual" ] || fail '0913 checksum mismatch'

for frozen in "$ROOT"/migrations/2026_09_{08,09,10,11,12}_fair_launch_*.sql; do
  git -C "$ROOT" diff --quiet -- "$frozen" || fail "frozen migration modified: ${frozen##*/}"
done

echo 'Fair Launch BSC mainnet activation schema static validation: PASS'
