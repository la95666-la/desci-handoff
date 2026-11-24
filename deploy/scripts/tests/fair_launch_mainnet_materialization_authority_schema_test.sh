#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MIG="$ROOT/migrations/2026_09_18_fair_launch_mainnet_materialization_authority.sql"
fail(){ echo "fair_launch_mainnet_materialization_authority_schema_test: FAIL: $*" >&2; exit 1; }
[ -f "$MIG" ] || fail '0918 migration missing'
grep -Fq '@JC_FAIR_LAUNCH_MAINNET_MATERIALIZATION_APPROVED' "$MIG" || fail 'explicit materialization migration gate missing'
for table in fair_launch_mainnet_archive_event_payload_v1 fair_launch_mainnet_materialization_profile_v1 fair_launch_mainnet_materialization_cursor_v1 fair_launch_mainnet_materialization_receipt_v1; do
  grep -Fq "$table" "$MIG" || fail "$table missing"
done
grep -Fq 'requires an empty archive event set' "$MIG" || fail 'pre-archive admission boundary missing'
grep -Fq "data_hex REGEXP '^0x([0-9a-f]{2})*$'" "$MIG" || fail 'durable raw ABI payload boundary missing'
grep -Fq 'SHA2(UNHEX(v_topics_hex),256)' "$MIG" || fail 'topics+data digest rehash missing'
grep -Fq "component='job'" "$MIG" || fail 'job component binding missing'
grep -Fq "event_name IN('FairLaunchTokensDeposited'" "$MIG" || fail 'financial event allowlist missing'
grep -Fq "source_cursor.lease_token=c.lease_token_hash" "$MIG" || fail 'generic V3 cursor hash fence missing'
grep -Fq 'materialization receipt predecessor mismatch' "$MIG" || fail 'ordered predecessor gate missing'
grep -Fq 'archive cursor payload set mismatch' "$MIG" || fail 'archive payload completeness gate missing'
grep -Fq 'is append-only' "$MIG" || fail 'immutable evidence protection missing'
! grep -Eq 'rpc_url|endpoint|credential|PRIVATE_KEY|private[_-]?key|mnemonic|raw[_-]?transaction' "$MIG" || fail 'endpoint/secret/signing field leaked into materialization schema'
line=$(grep -F '*2026_09_18_fair_launch_mainnet_materialization_authority.sql' "$ROOT/migrations/FAIR_LAUNCH_SHA256SUMS" || true)
[ -n "$line" ] || fail '0918 checksum binding missing'
actual=$(sha256sum "$MIG" | awk '{print $1}')
[ "${line:0:64}" = "$actual" ] || fail '0918 checksum mismatch'
for frozen in "$ROOT"/migrations/2026_09_{08,09,10,11,12}_fair_launch_*.sql; do
  git -C "$ROOT" diff --quiet -- "$frozen" || fail "frozen migration modified: ${frozen##*/}"
done
echo 'Fair Launch mainnet materialization authority schema static validation: PASS'
