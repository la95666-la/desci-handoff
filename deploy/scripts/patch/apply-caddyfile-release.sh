#!/usr/bin/env bash
# Patch-time remote uploads and service mutations are unavailable in this release.
set -euo pipefail
printf '%s\n' 'remote_config_upload_unavailable' >&2
exit 78
