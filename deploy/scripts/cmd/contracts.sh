#!/usr/bin/env bash
# CheckIn mutation is unavailable in this release. Mutation actions are rejected
# before any environment/configuration file is read.
set -euo pipefail

ACTION="${1:-status}"
[[ "$ACTION" =~ ^[a-z]+$ ]] || { printf '%s\n' '合约操作必须是单一受支持 token' >&2; exit 2; }
[ "$#" -le 1 ] || { printf '%s\n' '合约操作参数无效' >&2; exit 2; }

case "$ACTION" in
  deploy|upgrade)
    printf '%s\n' 'chain_mutation_unavailable' >&2
    exit 78
    ;;
  status)
    printf '%s\n' 'CheckIn local configuration status (read-only)'
    if [ -n "${JC_ENV_FILE:-}" ] && [ -f "$JC_ENV_FILE" ]; then
      printf '%s\n' 'configuration_file=present (contents not read)'
    else
      printf '%s\n' 'configuration_file=absent'
    fi
    ;;
  *)
    printf '未知合约操作: %s\n' "$ACTION" >&2
    exit 2
    ;;
esac
