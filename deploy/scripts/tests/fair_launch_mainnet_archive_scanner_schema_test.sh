#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MIG="$ROOT/migrations/2026_09_15_fair_launch_mainnet_archive_scanner.sql"
fail(){ echo "fair_launch_mainnet_archive_scanner_schema_test: FAIL: $*" >&2; exit 1; }

[ -f "$MIG" ] || fail '0915 migration missing'
grep -Fq '@JC_FAIR_LAUNCH_MAINNET_SCANNER_APPROVED' "$MIG" || fail 'explicit scanner migration gate missing'
for table in fair_launch_mainnet_archive_cursor_v1 fair_launch_mainnet_archive_batch_receipt_v1 fair_launch_mainnet_archive_batch_event_v1 fair_launch_mainnet_archive_dead_letter_v1; do
  grep -Fq "$table" "$MIG" || fail "$table missing"
done
grep -Fq "status IN('active','blocked','reorg_detected','revoked')" "$MIG" || fail 'scanner cursor state machine missing'
grep -Fq 'c.archived_through_block+1=NEW.from_block' "$MIG" || fail 'contiguous batch predecessor gate missing'
grep -Fq 'p.canonical_slot<>1 OR p.finalized_at IS NULL OR p.noncanonical_at IS NOT NULL' "$MIG" || fail 'finalized canonical event gate missing'
grep -Fq "'FairLaunchProjectCreated'" "$MIG" || fail 'frozen event allowlist missing'
grep -Fq "reason_code IN('unknown_event','malformed_log','receipt_mismatch','canonical_conflict','rpc_inconsistent','archive_conflict')" "$MIG" || fail 'dead-letter reason allowlist missing'
grep -Fq 'is append-only' "$MIG" || fail 'append-only protection missing'
! grep -Eq 'PRIVATE_KEY|private[_-]?key|mnemonic|raw[_-]?transaction' "$MIG" || fail 'secret/signing material field leaked into schema'
line=$(grep -F '*2026_09_15_fair_launch_mainnet_archive_scanner.sql' "$ROOT/migrations/FAIR_LAUNCH_SHA256SUMS" || true)
[ -n "$line" ] || fail '0915 checksum binding missing'
actual=$(sha256sum "$MIG" | awk '{print $1}')
[ "${line:0:64}" = "$actual" ] || fail '0915 checksum mismatch'
for frozen in "$ROOT"/migrations/2026_09_{08,09,10,11,12}_fair_launch_*.sql; do
  git -C "$ROOT" diff --quiet -- "$frozen" || fail "frozen migration modified: ${frozen##*/}"
done
echo 'Fair Launch mainnet archive scanner schema static validation: PASS'
