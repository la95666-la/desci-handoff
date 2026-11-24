#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
migration="$root/migrations/2026_10_05_allow_x_post_cross_slot_reuse.sql"
grep -Fq 'ALTER TABLE task DROP INDEX uk_platform_key_target' "$migration"
grep -Fq 'ALTER TABLE task_target_history DROP INDEX uk_task_target_history' "$migration"
grep -Fq 'ADD INDEX idx_task_target_history_lookup (platform, task_key, target_ref)' "$migration"
! grep -Eiq 'DROP INDEX (uk_platform_key_slot|uk_task_target_history_round|uk_task_target)' "$migration"
grep -Fq 'FROM information_schema.STATISTICS' "$migration"
printf 'X cross-slot migration guard passed\n'
