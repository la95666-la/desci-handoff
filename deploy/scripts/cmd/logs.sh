#!/usr/bin/env bash
# The legacy SSH/Compose log transport is unavailable because the operational
# Compose model is intentionally empty. jc-release exposes separately bounded
# read-only Frontend/Admin/Agent logs.
set -euo pipefail
printf '%s\n' 'service_log_transport_unavailable' >&2
exit 78
