#!/usr/bin/env bash
# Direct service restart is unavailable. Static artifacts may be built and verified
# offline, but no admitted operational release authority exists in this release.
set -euo pipefail

SERVICE="${1:-}"
[ "$#" -eq 1 ] || { printf '%s\n' 'service token required' >&2; exit 2; }
[[ "$SERVICE" =~ ^[a-z]+$ ]] || { printf '%s\n' 'service token invalid' >&2; exit 2; }
case "$SERVICE" in
  backend|job|lang) printf '%s\n' 'fair_launch_activation_unavailable' >&2 ;;
  tgbot) printf '%s\n' 'tgbot_release_unavailable' >&2 ;;
  hardhat) printf '%s\n' 'hardhat_operational_target_unavailable' >&2 ;;
  frontend|admin) printf '%s\n' 'static_release_admission_unavailable' >&2 ;;
  *) printf '%s\n' 'service_operation_unavailable' >&2 ;;
esac
exit 78
