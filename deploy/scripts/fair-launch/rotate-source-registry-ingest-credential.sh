#!/usr/bin/env bash
set -euo pipefail
echo 'credential_rotation_unavailable' >&2
echo 'This release has no signed single-use dual-credential rotation protocol.' >&2
exit 78
