#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MIG="$ROOT/migrations/2026_09_17_fair_launch_mainnet_recovery_provider.sql"
fail(){ echo "fair_launch_mainnet_recovery_provider_schema_test: FAIL: $*" >&2; exit 1; }
[ -f "$MIG" ] || fail '0917 migration missing'
grep -Fq '@JC_FAIR_LAUNCH_MAINNET_RECOVERY_PROVIDER_APPROVED' "$MIG" || fail 'explicit provider migration gate missing'
grep -Fq 'fair_launch_mainnet_recovery_provider_v1' "$MIG" || fail 'provider admission table missing'
grep -Fq "provider_role IN('primary','independent')" "$MIG" || fail 'provider role boundary missing'
grep -Fq "SHA2(CONCAT('joincare:fair-launch:mainnet-rpc:v1',CHAR(0),NEW.provider_identity),256)" "$MIG" || fail 'domain-separated provider identity rehash missing'
grep -Fq "NEW.provider_role='primary' AND NEW.rpc_identity_digest<>v_primary_digest" "$MIG" || fail 'primary provider activation binding missing'
grep -Fq "NEW.provider_role='independent' AND NEW.rpc_identity_digest=v_primary_digest" "$MIG" || fail 'independent provider separation missing'
grep -Fq 'is append-only' "$MIG" || fail 'provider admission append-only protection missing'
grep -Fq 'trg_fl_mainnet_recovery_receipt_provider_gate' "$MIG" || fail 'recovery receipt provider gate missing'
grep -Fq 'NEW.approval_evidence_digest<>v_replacement_approval' "$MIG" || fail 'replacement lease approval binding missing'
grep -Fq 'v_primary_count<>1 OR v_independent_count<1' "$MIG" || fail 'primary/independent recovery provider quorum missing'
! grep -Eq 'rpc_url|endpoint|credential|PRIVATE_KEY|private[_-]?key|mnemonic|raw[_-]?transaction' "$MIG" || fail 'endpoint/secret/signing field leaked into provider schema'
line=$(grep -F '*2026_09_17_fair_launch_mainnet_recovery_provider.sql' "$ROOT/migrations/FAIR_LAUNCH_SHA256SUMS" || true)
[ -n "$line" ] || fail '0917 checksum binding missing'
actual=$(sha256sum "$MIG" | awk '{print $1}')
[ "${line:0:64}" = "$actual" ] || fail '0917 checksum mismatch'
echo 'Fair Launch mainnet recovery provider schema static validation: PASS'
