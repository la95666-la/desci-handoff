#!/usr/bin/env bash
# Mandatory wrapper before any Backend/Job start or restart.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
bash "$ROOT/scripts/fair-launch/activation-gate.sh" verify
