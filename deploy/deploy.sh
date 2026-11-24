#!/usr/bin/env bash
# JoinCare release-boundary CLI. Production remains NO-GO.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
export JC_ROOT="$SCRIPT_DIR" JC_LIB="$SCRIPT_DIR/scripts/lib" JC_CMD="$SCRIPT_DIR/scripts/cmd" JC_TEMPLATE="$SCRIPT_DIR/config/template" JC_ENV_FILE="$SCRIPT_DIR/.deploy.env"
source "$JC_LIB/colors.sh"

usage() {
  cat <<'EOF'
JoinCare deployment CLI — current release availability

Available:
  status                         local-only configuration-file presence; no secrets read
  contracts status              local-only file-presence status; no secrets read
  ido-v2 status                 local-only file-presence status; no secrets read
  migrate status                controlled migration entrypoint status
  migrate preflight             checksum/backup/identity/ledger validation only
  migrate apply                 explicit checksum-bound migration execution

Unavailable (stable fail-closed errors):
  frontend/admin release        static_release_admission_unavailable
  shell service logs            service_log_transport_unavailable
  config                         remote_config_upload_unavailable
  Backend/Job/Lang activation   fair_launch_activation_unavailable
  TGBot release                 tgbot_release_unavailable
  Hardhat operation             hardhat_operational_target_unavailable
  install/backup/reseed/uninstall/partner package
  restart/upgrade/rollback/apply/upload and all service activation
  contract deploy/upgrade and IDO V2 deploy/configure-project-token

Migration execution is available only through the fail-closed manifest, backup,
identity, ledger and confirmation gates documented by `migrate --help`. Other
operational release and deployment actions remain unavailable. Production service
activation remains NO-GO through this CLI.
EOF
}

[ $# -gt 0 ] || { usage; exit 0; }
CMD="$1"; shift
[[ "$CMD" =~ ^[a-z0-9_-]+$ ]] || { error "子命令必须是单一受支持 token"; exit 2; }
case "$CMD" in
  help|--help|-h) [ "$#" -eq 0 ] || exit 2; usage ;;
  status|logs|restart|upgrade|contracts|ido-v2|install|migrate|backup|uninstall|config)
    script="$JC_CMD/$CMD.sh"
    [ -f "$script" ] || { error "未知子命令: $CMD"; exit 2; }
    bash "$script" "$@"
    ;;
  preflight_ido) error "fair_launch_activation_unavailable"; exit 78 ;;
  *) error "未知子命令: $CMD"; usage; exit 2 ;;
esac
