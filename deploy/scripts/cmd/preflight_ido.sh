#!/usr/bin/env bash
# Remote Backend preflight is unavailable while protected services are shut down.
set -euo pipefail
printf '%s\n' 'fair_launch_activation_unavailable' >&2
exit 78
