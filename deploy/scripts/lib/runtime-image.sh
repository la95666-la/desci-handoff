#!/usr/bin/env bash
# Hardhat runtime inspection/use is unavailable in this release.
set -euo pipefail
require_hardhat_runtime_image() {
  printf '%s\n' 'hardhat_operational_target_unavailable' >&2
  return 78
}
