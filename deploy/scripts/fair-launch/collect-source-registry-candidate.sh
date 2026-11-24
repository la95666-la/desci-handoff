#!/usr/bin/env bash
# Candidate collection requires a separately reviewed external database auditor.
# The application auditor deliberately cannot inspect mysql.user/default roles/grants.
set -euo pipefail
echo 'fair_launch_candidate_collection_unavailable: run the reviewer-owned external MySQL auditor; this repository cannot self-generate or self-admit post-state evidence' >&2
exit 78
