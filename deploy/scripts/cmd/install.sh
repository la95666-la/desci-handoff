#!/usr/bin/env bash
# Installation is unavailable until database authority, activation lease, and
# secret transport are admitted. No configuration, private key, SSH, migration,
# Docker, or remote filesystem operation is attempted in this release.
set -euo pipefail
source "$JC_LIB/colors.sh"
error "fair_launch_activation_unavailable"
exit 78
