#!/usr/bin/env bash
# Partner packaging is intentionally unavailable in this release. The retired
# implementation could copy stale operational guidance and expose database
# credentials through remote process arguments. Re-enable only with a reviewed,
# explicit package allowlist and secret-safe data export.
set -euo pipefail

echo 'partner_package_unavailable' >&2
exit 78
