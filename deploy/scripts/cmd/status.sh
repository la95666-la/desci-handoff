#!/usr/bin/env bash
# Local-only status. Remote status is unavailable until every remote operation is
# routed through the reviewed activation/transport boundary.
set -euo pipefail
printf '%s\n' 'JoinCare local deployment configuration status (read-only)'
if [ -n "${JC_ENV_FILE:-}" ] && [ -f "$JC_ENV_FILE" ]; then
  printf '%s\n' 'configuration_file=present (contents not read)'
else
  printf '%s\n' 'configuration_file=absent'
fi
printf '%s\n' 'remote_service_status=unavailable'
