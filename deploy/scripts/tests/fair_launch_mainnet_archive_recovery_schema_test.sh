#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MIG="$ROOT/migrations/2026_09_16_fair_launch_mainnet_archive_recovery.sql"
fail(){ echo "fair_launch_mainnet_archive_recovery_schema_test: FAIL: $*" >&2; exit 1; }

[ -f "$MIG" ] || fail '0916 migration missing'
grep -Fq '@JC_FAIR_LAUNCH_MAINNET_RECOVERY_APPROVED' "$MIG" || fail 'explicit recovery migration gate missing'
grep -Fq 'fair_launch_mainnet_archive_recovery_receipt_v1' "$MIG" || fail 'recovery receipt table missing'
grep -Fq 'last_recovery_receipt_id' "$MIG" || fail 'cursor recovery receipt linkage missing'
grep -Fq "OLD.status IN('blocked','reorg_detected') AND NEW.status='active'" "$MIG" || fail 'terminal cursor recovery transition missing'
grep -Fq 'new_l.lease_generation>old_l.lease_generation AND new_l.fence_epoch>old_l.fence_epoch' "$MIG" || fail 'replacement lease monotonicity missing'
grep -Fq 'verification_count>=2' "$MIG" || fail 'provider/canonical re-verification quorum missing'
grep -Fq 'is append-only' "$MIG" || fail 'recovery receipt append-only protection missing'
! grep -Eq 'PRIVATE_KEY|private[_-]?key|mnemonic|raw[_-]?transaction' "$MIG" || fail 'secret/signing material field leaked into schema'
line=$(grep -F '*2026_09_16_fair_launch_mainnet_archive_recovery.sql' "$ROOT/migrations/FAIR_LAUNCH_SHA256SUMS" || true)
[ -n "$line" ] || fail '0916 checksum binding missing'
actual=$(sha256sum "$MIG" | awk '{print $1}')
[ "${line:0:64}" = "$actual" ] || fail '0916 checksum mismatch'
for frozen in "$ROOT"/migrations/2026_09_{08,09,10,11,12}_fair_launch_*.sql; do
  git -C "$ROOT" diff --quiet -- "$frozen" || fail "frozen migration modified: ${frozen##*/}"
done
echo 'Fair Launch mainnet archive recovery schema static validation: PASS'
