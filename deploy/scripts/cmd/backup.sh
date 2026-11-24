#!/usr/bin/env bash
# MySQL backup is intentionally unavailable until the reviewed root-owned
# defaults-extra-file transport is provisioned. Passwords must never enter SSH,
# shell or process arguments.
set -euo pipefail
printf '%s\n' 'mysql_backup_credentials_transport_unavailable' >&2
exit 78
