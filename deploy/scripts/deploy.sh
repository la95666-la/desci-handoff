#!/usr/bin/env bash
# Legacy direct deployment entrypoint is intentionally disabled.
# Use ./deploy.sh for reviewed non-application operations. Backend, Job and
# language activation require a database lease that this release does not have.
set -euo pipefail

echo 'fair_launch_activation_unavailable' >&2
exit 78
