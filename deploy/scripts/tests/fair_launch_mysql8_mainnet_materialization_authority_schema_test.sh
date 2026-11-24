#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
JC_TEST_FAIR_LAUNCH_MAINNET_MATERIALIZATION_0918=1 exec bash "$ROOT/scripts/tests/fair_launch_mysql8_mainnet_archive_scanner_schema_test.sh"
