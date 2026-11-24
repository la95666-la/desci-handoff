#!/usr/bin/env bash
# Full backup is intentionally unavailable in this release. The previous path
# placed MySQL credentials in Docker/process arguments. Re-enable only with a
# reviewed root-owned 0600 defaults-extra-file or secret-provider transport.
set -euo pipefail
echo 'mysql_backup_credentials_transport_unavailable' >&2
exit 78
