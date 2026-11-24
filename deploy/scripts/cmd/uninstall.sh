#!/usr/bin/env bash
# Destructive remote uninstall is unavailable until an argv-safe, independently
# reviewed removal protocol exists.
set -euo pipefail
source "$JC_LIB/colors.sh"
error "uninstall_unavailable"
exit 78
