#!/usr/bin/env bash
# Local secret editing followed by an unavailable remote upload is misleading and
# unsafe. Configuration mutation is disabled until the reviewed config transport exists.
set -euo pipefail
printf '%s\n' 'remote_config_upload_unavailable' >&2
exit 78
