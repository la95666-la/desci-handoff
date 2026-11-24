#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MIG="$ROOT/migrations/2026_09_28_twitter_verification_policy.sql"

fail() { printf 'twitter_verification_policy_migration_test: FAIL: %s\n' "$*" >&2; exit 1; }

[ -f "$MIG" ] || fail 'migration missing'
grep -Fq "VALUES ('twitter.verification_disabled', '0', 'int')" "$MIG" || fail 'strict default setting missing'
grep -Fq 'duplicate twitter verification settings must be resolved before 0928' "$MIG" || fail 'duplicate setting preflight missing'
grep -Fq 'invalid twitter verification setting must be resolved before 0928' "$MIG" || fail 'invalid value preflight missing'
grep -Fq 'ON DUPLICATE KEY UPDATE `key` = VALUES(`key`)' "$MIG" || fail 'non-destructive config seed missing'
! grep -Eqi 'START[[:space:]]+(JOB|TRANSACTION)|executeBatch|CREATE[[:space:]]+EVENT|INSERT[[:space:]]+INTO[[:space:]].*(authority|cursor|lease)|private[_ -]?key' "$MIG" || fail 'migration widens forbidden runtime authority'

printf 'twitter_verification_policy_migration_test: PASS\n'
