#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MIG="$ROOT/migrations/2026_09_19_x_task_target_slots.sql"

fail() { printf 'x_task_target_slots_migration_test: FAIL: %s\n' "$*" >&2; exit 1; }

[ -f "$MIG" ] || fail 'migration missing'
grep -Fq 'ADD COLUMN target_slot TINYINT UNSIGNED NOT NULL DEFAULT 1' "$MIG" || fail 'target_slot column missing'
grep -Fq 'UNIQUE INDEX uk_platform_key_slot (platform, task_key, target_slot)' "$MIG" || fail 'slot uniqueness missing'
grep -Fq 'UNIQUE INDEX uk_platform_key_target (platform, task_key, target_ref)' "$MIG" || fail 'duplicate target uniqueness missing'
grep -Fq 'CREATE TABLE IF NOT EXISTS `task_target_history`' "$MIG" || fail 'cross-slot history table missing'
grep -Fq 'UNIQUE INDEX `uk_task_target_history` (`platform`, `task_key`, `target_ref`)' "$MIG" || fail 'cross-slot history uniqueness missing'
grep -Fq 'duplicate historical X action targets must be resolved before 0919' "$MIG" || fail 'historical duplicate preflight missing'
grep -Fq "LOWER(NEW.task_key) IN ('like','reply','retweet')" "$MIG" || fail 'slot 2 allowlist missing'
grep -Fq 'bi_task_target_slot_0919' "$MIG" || fail 'insert trigger missing'
grep -Fq 'bu_task_target_slot_0919' "$MIG" || fail 'update trigger missing'
grep -Fq 'duplicate task platform/key/slot rows must be resolved before 0919' "$MIG" || fail 'preflight duplicate guard missing'
! grep -Eqi 'START[[:space:]]+(JOB|TRANSACTION)|executeBatch|CREATE[[:space:]]+EVENT|INSERT[[:space:]]+INTO[[:space:]].*(authority|cursor|lease)|private[_ -]?key' "$MIG" || fail 'migration widens forbidden runtime authority'

printf 'x_task_target_slots_migration_test: PASS\n'
