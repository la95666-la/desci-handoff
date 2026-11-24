#!/usr/bin/env bash
# Builds the pinned production-like pre-Fair-Launch baseline fixture offline.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT="$ROOT/migrations/baseline/fair-launch-pre-2026-09-03.sql"
mkdir -p "$(dirname "$OUT")"
{
  printf '%s\n' '-- joincare Fair Launch pinned production-like baseline v1'
  printf '%s\n' '-- source: joincare.sql + ordered migrations through 2026_09_02'
  cat "$ROOT/joincare.sql"
  for f in "$ROOT"/migrations/*.sql; do
    base=$(basename "$f")
    [[ $base < 2026_09_03_fair_launch_project_model.sql ]] || continue
    printf '\n-- BEGIN %s\n' "$base"
    cat "$f"
    printf '\n-- END %s\n' "$base"
  done
} > "$OUT"
printf '%s  %s\n' "$(sha256sum "$OUT" | awk '{print $1}')" "$(basename "$OUT")" > "$OUT.sha256"
echo "pinned baseline fixture written: $OUT"
