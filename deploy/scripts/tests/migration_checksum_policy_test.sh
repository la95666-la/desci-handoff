#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OLD="4ee32387ab98d9ffd061a9eef6a58582e5ed8b5fcaf80872190160cb4b4ea20a"
NEW="2401bf1d558e125406e59b4bb76e3ad4acb724ba60de8867b979114e25c9b1e7"
MIGRATION="$ROOT/migrations/2026_08_30_security_control_plane_and_event_fence.sql"

[ "$(sha256sum "$MIGRATION" | awk '{print $1}')" = "$NEW" ]
grep -Fq "$OLD" "$ROOT/docs/MIGRATION-CHECKSUM-POLICY.md"
grep -Fq "$NEW" "$ROOT/docs/MIGRATION-CHECKSUM-POLICY.md"

# Migration execution is controlled by exact repository/ledger checksums. The old
# direct deploy compatibility implementation remains unavailable.
grep -Fq 'migration_repository_checksum_mismatch' "$ROOT/scripts/cmd/migrate.sh"
grep -Fq 'migration_ledger_checksum_mismatch' "$ROOT/scripts/cmd/migrate.sh"
grep -Fq 'fair_launch_activation_unavailable' "$ROOT/scripts/deploy.sh"
! grep -Fq 'legacy_security_checksum=' "$ROOT/scripts/deploy.sh"
! grep -Eq 'mysql(admin|dump)?[^[:cntrl:]]*(--password=|-p[^[:space:]]+)' "$ROOT/scripts/deploy.sh"
bash -n "$ROOT/scripts/cmd/migrate.sh"
bash -n "$ROOT/scripts/deploy.sh"

echo "security migration checksum compatibility policy: PASS"
