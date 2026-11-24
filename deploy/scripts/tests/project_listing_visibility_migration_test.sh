#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MIG="$ROOT/migrations/2026_10_09_project_listing_visibility.sql"
fail(){ echo "project listing visibility migration: $*" >&2; exit 1; }

[ -f "$MIG" ] || fail 'migration missing'
grep -Fq 'ADD COLUMN is_listed TINYINT(1) NOT NULL DEFAULT 1' "$MIG" || fail 'safe visible default missing'
grep -Fq 'chk_projects_is_listed CHECK (is_listed IN (0,1))' "$MIG" || fail 'boolean check constraint missing'
grep -Fq 'idx_projects_public_list (is_listed, status, id)' "$MIG" || fail 'public list index missing'
grep -Fq "TABLE_NAME = 'projects'" "$MIG" || fail 'projects table guard missing'
grep -Fq "COLUMN_NAME = 'is_listed'" "$MIG" || fail 'idempotent column guard missing'
grep -Fq "CONSTRAINT_NAME = 'chk_projects_is_listed'" "$MIG" || fail 'idempotent constraint guard missing'
grep -Fq "INDEX_NAME = 'idx_projects_public_list'" "$MIG" || fail 'idempotent index guard missing'
! grep -Eq 'UPDATE[[:space:]]+projects|DELETE[[:space:]]+FROM[[:space:]]+projects' "$MIG" || fail 'migration must not rewrite existing projects'

echo 'Project listing visibility migration: PASS'
