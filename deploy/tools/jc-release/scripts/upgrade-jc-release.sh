#!/usr/bin/env bash
# Upgrade installed jc-release binaries from a checksum-bound release directory.
#
# Expected package layout:
#   upgrade-jc-release.sh
#   RELEASE.txt
#   SHA256SUMS
#   bin/jc-release-server
#   bin/jc-agent
#
# Default behavior is "auto": upgrade only consistently installed components.
# Configuration and state files are never modified.

set -Eeuo pipefail
umask 077
PATH=/usr/sbin:/usr/bin:/sbin:/bin
export PATH

readonly SCRIPT_NAME="$(basename "$0")"
readonly SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)"
readonly RELEASE_FILE="$SCRIPT_DIR/RELEASE.txt"
readonly SUM_FILE="$SCRIPT_DIR/SHA256SUMS"
readonly BACKUP_BASE="/var/backups/jc-release"
readonly LOCK_FILE="/run/lock/jc-release-upgrade.lock"

MODE="auto"
VERIFY_ONLY=0
DRY_RUN=0
BACKUP_DIR=""
SOURCE_COMMIT=""
RELEASE_ID=""

declare -a SELECTED=()
declare -a CHANGED=()
declare -A TARGET=(
  [server]="/usr/local/bin/jc-release-server"
  [agent]="/usr/local/bin/jc-agent"
)
declare -A CANDIDATE=(
  [server]="$SCRIPT_DIR/bin/jc-release-server"
  [agent]="$SCRIPT_DIR/bin/jc-agent"
)
declare -A UNIT=(
  [server]="jc-release-server.service"
  [agent]="jc-agent.service"
)
declare -A CONFIG=(
  [server]="/etc/jc-release/server.yaml"
  [agent]="/etc/jc-agent/agent.yaml"
)
declare -A WAS_ACTIVE=()

log() {
  printf '[jc-release-upgrade] %s\n' "$*"
}

warn() {
  printf '[jc-release-upgrade] WARNING: %s\n' "$*" >&2
}

die() {
  printf '[jc-release-upgrade] ERROR: %s\n' "$*" >&2
  return 1
}

usage() {
  cat <<'EOF'
Usage:
  sudo ./upgrade-jc-release.sh [--component auto|server|agent|all] [--dry-run]
  ./upgrade-jc-release.sh --verify-only

Options:
  --component MODE  auto (default), server, agent, or all
  --dry-run         verify package and installed services without changing files
  --verify-only     verify package integrity/architecture; root is not required
  -h, --help        show this help

The script preserves existing YAML configuration and state directories. It backs
up installed binaries, performs same-filesystem atomic replacement, restarts only
services that were active before the upgrade, verifies the running executable
hash, and rolls back every changed component if any verification fails.
EOF
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --component)
        [ "$#" -ge 2 ] || die "--component requires a value"
        MODE="$2"
        shift 2
        ;;
      --dry-run)
        DRY_RUN=1
        shift
        ;;
      --verify-only)
        VERIFY_ONLY=1
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "unknown argument: $1"
        ;;
    esac
  done
  case "$MODE" in
    auto|server|agent|all) ;;
    *) die "invalid component mode: $MODE" ;;
  esac
  if [ "$VERIFY_ONLY" -eq 1 ] && [ "$DRY_RUN" -eq 1 ]; then
    die "--verify-only and --dry-run cannot be combined"
  fi
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

manifest_value() {
  local key="$1" line count
  count="$(grep -cE "^${key}=" "$RELEASE_FILE" || true)"
  [ "$count" = "1" ] || die "RELEASE.txt must contain exactly one ${key}= entry"
  line="$(grep -E "^${key}=" "$RELEASE_FILE")"
  printf '%s' "${line#*=}"
}

verify_checksum_manifest_shape() {
  local count=0 hash path
  declare -A seen=()
  while read -r hash path; do
    [ -n "${hash:-}" ] || continue
    path="${path#\*}"
    [[ "$hash" =~ ^[0-9a-f]{64}$ ]] || die "invalid SHA-256 entry in SHA256SUMS"
    case "$path" in
      "bin/jc-release-server"|"bin/jc-agent"|"upgrade-jc-release.sh"|"RELEASE.txt") ;;
      *) die "unexpected path in SHA256SUMS: $path" ;;
    esac
    [ -z "${seen[$path]:-}" ] || die "duplicate SHA256SUMS path: $path"
    seen[$path]=1
    count=$((count + 1))
  done < "$SUM_FILE"
  [ "$count" -eq 4 ] || die "SHA256SUMS must contain exactly four approved entries"
  for path in bin/jc-release-server bin/jc-agent upgrade-jc-release.sh RELEASE.txt; do
    [ "${seen[$path]:-}" = "1" ] || die "SHA256SUMS is missing $path"
  done
}

verify_elf_amd64() {
  local file="$1" description
  [ -f "$file" ] && [ ! -L "$file" ] || die "candidate must be a regular non-symlink file: $file"
  [ -x "$file" ] || die "candidate is not executable: $file"
  description="$(file -b "$file")"
  [[ "$description" == *"ELF 64-bit LSB executable"* ]] || die "candidate is not an ELF64 LSB executable: $file"
  [[ "$description" == *"x86-64"* ]] || die "candidate is not x86-64: $file"
  [[ "$description" == *"statically linked"* ]] || die "candidate is not statically linked: $file"
  "$file" -h >/dev/null 2>&1 || die "candidate help probe failed: $file"
}

verify_package() {
  require_command sha256sum
  require_command file
  require_command awk
  require_command grep

  [ -f "$RELEASE_FILE" ] && [ ! -L "$RELEASE_FILE" ] || die "missing regular RELEASE.txt"
  [ -f "$SUM_FILE" ] && [ ! -L "$SUM_FILE" ] || die "missing regular SHA256SUMS"

  [ "$(manifest_value schema)" = "joincare.jc-release-binary-upgrade.v1" ] || die "unsupported release schema"
  SOURCE_COMMIT="$(manifest_value source_commit)"
  RELEASE_ID="$(manifest_value release_id)"
  [[ "$SOURCE_COMMIT" =~ ^[0-9a-f]{40}$ ]] || die "invalid source_commit in RELEASE.txt"
  [[ "$RELEASE_ID" =~ ^jc-release-[0-9]{8}T[0-9]{6}Z-[0-9a-f]{12}$ ]] || die "invalid release_id in RELEASE.txt"
  [ "$(manifest_value target_os)" = "linux" ] || die "release target_os is not linux"
  [ "$(manifest_value target_arch)" = "amd64" ] || die "release target_arch is not amd64"
  case "$(manifest_value operational_boundary)" in
    production-core-services-job-signed-readiness-gated|production-core-services-job-checksum-runtime-gated) ;;
    *) die "unexpected operational boundary" ;;
  esac

  verify_checksum_manifest_shape
  (
    cd "$SCRIPT_DIR"
    sha256sum --check --strict --quiet SHA256SUMS
  ) || die "package checksum verification failed"

  verify_elf_amd64 "${CANDIDATE[server]}"
  verify_elf_amd64 "${CANDIDATE[agent]}"

  log "package verified: $RELEASE_ID"
  log "source commit: $SOURCE_COMMIT"
  log "operational boundary: Backend/Frontend/Admin retained; Job requires immutable image identity and runtime checks; Fair Launch materializer disabled"
}

unit_load_state() {
  systemctl show "$1" --property=LoadState --value 2>/dev/null || true
}

inspect_component() {
  local component="$1" target unit load
  target="${TARGET[$component]}"
  unit="${UNIT[$component]}"
  load="$(unit_load_state "$unit")"
  if [ -e "$target" ] || [ -L "$target" ] || [ "$load" != "not-found" -a -n "$load" ]; then
    [ -f "$target" ] && [ ! -L "$target" ] || return 3
    [ "$load" = "loaded" ] || return 4
    [ -f "${CONFIG[$component]}" ] && [ ! -L "${CONFIG[$component]}" ] || return 5
    return 0
  fi
  return 2
}

inspect_component_result() {
  local component="$1" result="$2"
  case "$result" in
    0) printf 'installed' ;;
    2) printf 'absent' ;;
    3) die "$component target is missing, non-regular, or a symlink: ${TARGET[$component]}" ;;
    4) die "$component binary exists but systemd unit is not loaded: ${UNIT[$component]}" ;;
    5) die "$component config is missing, non-regular, or a symlink: ${CONFIG[$component]}" ;;
    *) die "unexpected $component installation inspection result: $result" ;;
  esac
}

select_components() {
  local server_result agent_result server_state agent_state server_installed=0 agent_installed=0
  if inspect_component server; then server_result=0; else server_result=$?; fi
  if inspect_component agent; then agent_result=0; else agent_result=$?; fi

  case "$MODE" in
    auto)
      server_state="$(inspect_component_result server "$server_result")"
      agent_state="$(inspect_component_result agent "$agent_result")"
      [ "$server_state" != "installed" ] || server_installed=1
      [ "$agent_state" != "installed" ] || agent_installed=1
      [ "$server_installed" -eq 0 ] || SELECTED+=(server)
      [ "$agent_installed" -eq 0 ] || SELECTED+=(agent)
      [ "${#SELECTED[@]}" -gt 0 ] || die "no consistently installed jc-release component found"
      ;;
    server)
      server_state="$(inspect_component_result server "$server_result")"
      [ "$server_state" != "installed" ] || server_installed=1
      [ "$server_installed" -eq 1 ] || die "jc-release-server is not consistently installed"
      SELECTED=(server)
      ;;
    agent)
      agent_state="$(inspect_component_result agent "$agent_result")"
      [ "$agent_state" != "installed" ] || agent_installed=1
      [ "$agent_installed" -eq 1 ] || die "jc-agent is not consistently installed"
      SELECTED=(agent)
      ;;
    all)
      server_state="$(inspect_component_result server "$server_result")"
      agent_state="$(inspect_component_result agent "$agent_result")"
      [ "$server_state" != "installed" ] || server_installed=1
      [ "$agent_state" != "installed" ] || agent_installed=1
      [ "$server_installed" -eq 1 ] || die "jc-release-server is not consistently installed"
      [ "$agent_installed" -eq 1 ] || die "jc-agent is not consistently installed"
      SELECTED=(server agent)
      ;;
  esac

  local component state current_hash candidate_hash
  for component in "${SELECTED[@]}"; do
    state="$(systemctl is-active "${UNIT[$component]}" 2>/dev/null || true)"
    if [ "$state" = "active" ]; then WAS_ACTIVE[$component]=1; else WAS_ACTIVE[$component]=0; fi
    current_hash="$(sha256sum "${TARGET[$component]}" | awk '{print $1}')"
    candidate_hash="$(sha256sum "${CANDIDATE[$component]}" | awk '{print $1}')"
    log "selected $component: service=$state current=$current_hash candidate=$candidate_hash"
  done
}

create_backup() {
  local timestamp component
  timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
  BACKUP_DIR="$BACKUP_BASE/${timestamp}-${SOURCE_COMMIT:0:12}"
  install -d -m 0700 "$BACKUP_DIR"
  for component in "${SELECTED[@]}"; do
    install -m 0755 "${TARGET[$component]}" "$BACKUP_DIR/$(basename "${TARGET[$component]}")"
    sha256sum "$BACKUP_DIR/$(basename "${TARGET[$component]}")" >> "$BACKUP_DIR/original-binaries.sha256"
    sha256sum "${CONFIG[$component]}" >> "$BACKUP_DIR/config-digests.sha256"
    systemctl show "${UNIT[$component]}" \
      --property=LoadState,ActiveState,SubState,UnitFileState,MainPID,NRestarts \
      >> "$BACKUP_DIR/systemd-before.txt"
  done
  chmod 0600 "$BACKUP_DIR"/*.sha256 "$BACKUP_DIR/systemd-before.txt"
  log "backup created: $BACKUP_DIR"
}

verify_candidate_runtime_compatibility() {
  local component
  for component in "${SELECTED[@]}"; do
    if [ "$component" = "agent" ]; then
      log "checking candidate agent against existing config and state"
      "${CANDIDATE[$component]}" -config "${CONFIG[$component]}" status >/dev/null || die "candidate jc-agent cannot load the existing config/state"
    fi
  done
}

verify_config_unchanged() {
  local component expected actual
  for component in "${SELECTED[@]}"; do
    expected="$(grep -F "  ${CONFIG[$component]}" "$BACKUP_DIR/config-digests.sha256" | awk '{print $1}')"
    [ -n "$expected" ] || die "missing backed-up config digest for ${CONFIG[$component]}"
    actual="$(sha256sum "${CONFIG[$component]}" | awk '{print $1}')"
    [ "$actual" = "$expected" ] || die "configuration changed unexpectedly: ${CONFIG[$component]}"
  done
}

atomic_install() {
  local source="$1" target="$2" expected tmp actual
  expected="$(sha256sum "$source" | awk '{print $1}')"
  tmp="$(mktemp "${target}.new.XXXXXX")" || { die "cannot create same-filesystem staging file for $target"; return 1; }
  install -o root -g root -m 0755 "$source" "$tmp" || { rm -f "$tmp"; die "cannot stage candidate for $target"; return 1; }
  actual="$(sha256sum "$tmp" | awk '{print $1}')"
  [ "$actual" = "$expected" ] || { rm -f "$tmp"; die "staged binary checksum mismatch for $target"; return 1; }
  mv -fT "$tmp" "$target" || { rm -f "$tmp"; die "atomic replacement failed for $target"; return 1; }
  if command -v restorecon >/dev/null 2>&1; then restorecon "$target" >/dev/null 2>&1 || true; fi
  sync -f "$target" 2>/dev/null || sync || { die "filesystem sync failed for $target"; return 1; }
}

install_selected() {
  local component
  for component in "${SELECTED[@]}"; do
    atomic_install "${CANDIDATE[$component]}" "${TARGET[$component]}"
    CHANGED+=("$component")
    log "installed $component binary"
  done
}

wait_for_active() {
  local unit="$1" attempt pid
  for attempt in $(seq 1 30); do
    pid="$(systemctl show "$unit" --property=MainPID --value 2>/dev/null || true)"
    if [ "$(systemctl is-active "$unit" 2>/dev/null || true)" = "active" ] && [[ "$pid" =~ ^[1-9][0-9]*$ ]]; then return 0; fi
    sleep 1
  done
  return 1
}

systemd_failure_summary() {
  local unit="$1"
  systemctl show "$unit" --property=LoadState,ActiveState,SubState,Result,ExecMainCode,ExecMainStatus,MainPID,NRestarts --no-pager 2>/dev/null || true
}

verify_running_component() {
  local component="$1" unit target expected pid running_hash restarts_before restarts_after
  unit="${UNIT[$component]}"
  target="${TARGET[$component]}"
  expected="$(sha256sum "${CANDIDATE[$component]}" | awk '{print $1}')"

  if ! wait_for_active "$unit"; then
    warn "$unit failed to reach active state with a valid MainPID"
    systemd_failure_summary "$unit" >&2
    die "$unit did not become active with a valid MainPID"
  fi
  pid="$(systemctl show "$unit" --property=MainPID --value)"
  [[ "$pid" =~ ^[1-9][0-9]*$ ]] || die "$unit has no valid MainPID"
  [ -r "/proc/$pid/exe" ] || die "$unit executable is not readable via /proc/$pid/exe"
  running_hash="$(sha256sum "/proc/$pid/exe" | awk '{print $1}')"
  [ "$running_hash" = "$expected" ] || die "$unit is not running the candidate binary"

  restarts_before="$(systemctl show "$unit" --property=NRestarts --value)"
  sleep 8
  [ "$(systemctl is-active "$unit" 2>/dev/null || true)" = "active" ] || die "$unit stopped during stability check"
  restarts_after="$(systemctl show "$unit" --property=NRestarts --value)"
  [ "$restarts_after" = "$restarts_before" ] || die "$unit restarted during stability check"

  if [ "$component" = "agent" ]; then
    "$target" -config "${CONFIG[$component]}" status >/dev/null || die "jc-agent status self-check failed"
  fi
  log "verified $unit: active, stable, running_hash=$running_hash"
}

restart_and_verify() {
  local component
  # The control plane is restarted before its agent when both live on one host.
  for component in server agent; do
    if [[ " ${SELECTED[*]} " != *" $component "* ]]; then continue; fi
    if [ "${WAS_ACTIVE[$component]}" -eq 1 ]; then
      systemctl restart "${UNIT[$component]}"
      verify_running_component "$component"
    else
      warn "${UNIT[$component]} was inactive; binary replaced but service state was preserved"
    fi
  done
}

rollback() {
  local component backup target unit expected pid running_hash attempt
  [ -n "$BACKUP_DIR" ] && [ -d "$BACKUP_DIR" ] || return 0
  [ "${#CHANGED[@]}" -gt 0 ] || return 0
  warn "rolling back changed jc-release binaries"
  for component in "${CHANGED[@]}"; do
    target="${TARGET[$component]}"
    backup="$BACKUP_DIR/$(basename "$target")"
    if [ -f "$backup" ]; then
      atomic_install "$backup" "$target" || warn "failed to restore $target"
    fi
  done
  for component in server agent; do
    if [[ " ${CHANGED[*]} " != *" $component "* ]]; then continue; fi
    if [ "${WAS_ACTIVE[$component]:-0}" -eq 1 ]; then
      unit="${UNIT[$component]}"
      expected="$(sha256sum "$BACKUP_DIR/$(basename "${TARGET[$component]}")" | awk '{print $1}')"
      if systemctl restart "$unit"; then
        for attempt in $(seq 1 30); do
          [ "$(systemctl is-active "$unit" 2>/dev/null || true)" = "active" ] && break
          sleep 1
        done
        pid="$(systemctl show "$unit" --property=MainPID --value 2>/dev/null || true)"
        if [[ "$pid" =~ ^[1-9][0-9]*$ ]] && [ -r "/proc/$pid/exe" ]; then
          running_hash="$(sha256sum "/proc/$pid/exe" | awk '{print $1}')"
          [ "$running_hash" = "$expected" ] || warn "$unit rollback hash verification failed"
        else
          warn "$unit rollback process verification failed"
        fi
      else
        warn "failed to restart restored $unit"
      fi
    fi
  done
  warn "rollback attempted; backup retained at $BACKUP_DIR"
}

on_error() {
  local line="$1" code="$2"
  trap - ERR
  set +e
  warn "upgrade failed at line $line (exit $code)"
  rollback
  exit "$code"
}

main() {
  parse_args "$@"
  verify_package
  if [ "$VERIFY_ONLY" -eq 1 ]; then
    log "verification-only completed successfully"
    return 0
  fi

  [ "$(id -u)" -eq 0 ] || die "run the upgrade as root (sudo)"
  [ "$(uname -s)" = "Linux" ] || die "this upgrade supports Linux only"
  case "$(uname -m)" in x86_64|amd64) ;; *) die "this release supports amd64 only" ;; esac

  require_command systemctl
  require_command flock
  require_command install
  require_command mktemp
  require_command sync
  require_command seq

  install -d -m 0755 "$(dirname "$LOCK_FILE")"
  exec 9>"$LOCK_FILE"
  flock -n 9 || die "another jc-release upgrade is running"

  select_components
  verify_candidate_runtime_compatibility
  if [ "$DRY_RUN" -eq 1 ]; then
    log "dry-run completed; no files or services were changed"
    return 0
  fi

  create_backup
  install_selected
  restart_and_verify
  verify_config_unchanged

  log "upgrade completed successfully: $RELEASE_ID"
  log "backup retained for manual recovery: $BACKUP_DIR"
  log "existing YAML configuration and state directories were not modified"
}

trap 'on_error "$LINENO" "$?"' ERR
main "$@"
