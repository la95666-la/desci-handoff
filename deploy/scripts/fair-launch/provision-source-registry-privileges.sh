#!/usr/bin/env bash
# Source Registry identity creation, security-object replacement, exact grants,
# candidate collection and activation are intentionally unavailable. They require
# an independently reviewed, signed, single-use external schema-authority flow.
set -euo pipefail

case "${1:-}" in
  identities|objects|grants)
    echo 'fair_launch_transformation_unavailable' >&2
    exit 78
    ;;
  collect)
    echo 'fair_launch_candidate_collection_unavailable' >&2
    exit 78
    ;;
  activate)
    echo 'fair_launch_activation_unavailable' >&2
    exit 78
    ;;
  *)
    echo 'usage: provision-source-registry-privileges.sh identities|objects|grants|collect|activate' >&2
    exit 2
    ;;
esac
