#!/usr/bin/env bash
set -Eeuo pipefail
umask 077
PATH=/usr/sbin:/usr/bin:/sbin:/bin
export PATH

COMMON_CONFIG="/opt/joincare-prod/config/common/config.yaml"
SECRET_CONFIG="/opt/joincare-prod/config/secret/config.yaml"
PRODUCTION_CONTAINER="jc-backend"
BACKUP_ROOT="/var/backups/joincare-config"
LOCK_FILE="/run/lock/joincare-jwt-rotation.lock"

die() {
  printf '[jwt-rotation] ERROR: %s\n' "$*" >&2
  exit 1
}

log() {
  printf '[jwt-rotation] %s\n' "$*"
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "$1 is required"
}

container_health() {
  docker inspect "$PRODUCTION_CONTAINER" \
    --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}'
}

restore_configs() {
  [ "${CONFIGS_CHANGED:-0}" -eq 1 ] || return 0
  log 'restoring original configuration files'
  install -o "$COMMON_UID" -g "$COMMON_GID" -m "$COMMON_MODE" \
    "$BACKUP_DIR/common-config.yaml.before" "$COMMON_CONFIG"
  install -o "$SECRET_UID" -g "$SECRET_GID" -m "$SECRET_MODE" \
    "$BACKUP_DIR/secret-config.yaml.before" "$SECRET_CONFIG"
  sync -f "$COMMON_CONFIG" 2>/dev/null || true
  sync -f "$SECRET_CONFIG" 2>/dev/null || true
}

cleanup() {
  [ -z "${TEMP_DIR:-}" ] || rm -rf -- "$TEMP_DIR"
}

on_exit() {
  local rc=$?
  set +e
  if [ "$rc" -ne 0 ]; then
    restore_configs
  fi
  cleanup
  exit "$rc"
}

on_error() {
  local line="$1" code="$2"
  trap - ERR
  printf '[jwt-rotation] ERROR: failed at line %s (exit %s)\n' "$line" "$code" >&2
  exit "$code"
}

trap 'on_error "$LINENO" "$?"' ERR
trap on_exit EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

[ "$(id -u)" -eq 0 ] || die 'run as root: sudo bash rotate-production-jwt-secrets-acf2d461.sh'
require_command docker
require_command flock
require_command openssl
require_command python3
require_command sha256sum
require_command stat
require_command install

for path in "$COMMON_CONFIG" "$SECRET_CONFIG"; do
  [ -f "$path" ] && [ ! -L "$path" ] || die "config must be a regular non-symlink file: $path"
done

docker inspect "$PRODUCTION_CONTAINER" >/dev/null 2>&1 || die "$PRODUCTION_CONTAINER does not exist"
[ "$(docker inspect "$PRODUCTION_CONTAINER" --format '{{.State.Status}}')" = "running" ] || \
  die 'production backend is not running'
[ "$(container_health)" = "healthy" ] || die 'production backend is not healthy'

install -d -m 0755 "$(dirname "$LOCK_FILE")"
exec 9>"$LOCK_FILE"
flock -n 9 || die 'another JWT rotation is running'

COMMON_UID="$(stat -c '%u' "$COMMON_CONFIG")"
COMMON_GID="$(stat -c '%g' "$COMMON_CONFIG")"
COMMON_MODE="$(stat -c '%a' "$COMMON_CONFIG")"
SECRET_UID="$(stat -c '%u' "$SECRET_CONFIG")"
SECRET_GID="$(stat -c '%g' "$SECRET_CONFIG")"
SECRET_MODE="$(stat -c '%a' "$SECRET_CONFIG")"
COMMON_TARGET_MODE="600"
SECRET_TARGET_MODE="600"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
BACKUP_DIR="$BACKUP_ROOT/$timestamp-jwt-rotation"
install -d -m 0700 "$BACKUP_DIR"
install -m 0600 "$COMMON_CONFIG" "$BACKUP_DIR/common-config.yaml.before"
install -m 0600 "$SECRET_CONFIG" "$BACKUP_DIR/secret-config.yaml.before"
sha256sum "$BACKUP_DIR/common-config.yaml.before" "$BACKUP_DIR/secret-config.yaml.before" \
  >"$BACKUP_DIR/before.sha256"
chmod 0600 "$BACKUP_DIR/before.sha256"

TEMP_DIR="$(mktemp -d /root/jc-jwt-rotation.XXXXXX)"
chmod 0700 "$TEMP_DIR"
client_secret_file="$TEMP_DIR/client.secret"
admin_secret_file="$TEMP_DIR/admin.secret"
common_candidate="$TEMP_DIR/common.config.yaml.new"
secret_candidate="$TEMP_DIR/secret.config.yaml.new"

openssl rand -hex 32 >"$client_secret_file"
openssl rand -hex 32 >"$admin_secret_file"
chmod 0600 "$client_secret_file" "$admin_secret_file"

python3 - \
  "$COMMON_CONFIG" "$SECRET_CONFIG" \
  "$client_secret_file" "$admin_secret_file" \
  "$common_candidate" "$secret_candidate" <<'PY'
import os
import re
import sys

common_path, secret_path, client_path, admin_path, common_out, secret_out = sys.argv[1:]

def read(path):
    with open(path, "r", encoding="utf-8", newline="") as handle:
        return handle.read()

def secret(path):
    with open(path, "r", encoding="ascii") as handle:
        value = handle.read().strip()
    if not re.fullmatch(r"[0-9a-f]{64}", value):
        raise SystemExit("generated secret has invalid shape")
    return value

def replace_or_add(text, section, key, value, require_existing_section=False):
    newline = "\r\n" if "\r\n" in text else "\n"
    lines = text.splitlines()
    section_re = re.compile(r"^" + re.escape(section) + r":\s*(?:#.*)?$")
    starts = [i for i, line in enumerate(lines) if section_re.fullmatch(line)]
    if len(starts) > 1:
        raise SystemExit(f"duplicate top-level section: {section}")
    if not starts:
        if require_existing_section:
            raise SystemExit(f"required section missing: {section}")
        if lines and lines[-1].strip():
            lines.append("")
        lines.extend([section + ":", f'  {key}: "{value}"'])
        return newline.join(lines) + newline

    start = starts[0]
    end = len(lines)
    for i in range(start + 1, len(lines)):
        line = lines[i]
        if line.strip() and not line.lstrip().startswith("#") and not line[0].isspace():
            end = i
            break

    key_re = re.compile(r"^([ \t]+)" + re.escape(key) + r"\s*:")
    matches = []
    for i in range(start + 1, end):
        match = key_re.match(lines[i])
        if match:
            matches.append((i, match.group(1)))
    if len(matches) > 1:
        raise SystemExit(f"duplicate key: {section}.{key}")
    if matches:
        index, indent = matches[0]
        lines[index] = f'{indent}{key}: "{value}"'
    else:
        lines.insert(start + 1, f'  {key}: "{value}"')
    return newline.join(lines) + newline

common = read(common_path)
secret_config = read(secret_path)
client = secret(client_path)
admin = secret(admin_path)
if client == admin or admin == client + "::admin":
    raise SystemExit("generated secrets are not independent")

common_new = replace_or_add(common, "jwt", "secret", client, require_existing_section=True)
secret_new = replace_or_add(secret_config, "admin", "jwtSecret", admin)

for path, content in ((common_out, common_new), (secret_out, secret_new)):
    with open(path, "x", encoding="utf-8", newline="") as handle:
        handle.write(content)
    os.chmod(path, 0o600)
PY

python3 - "$common_candidate" "$secret_candidate" <<'PY'
import re
import sys

def get(path, section, key):
    lines = open(path, encoding="utf-8").read().splitlines()
    section_re = re.compile(r"^" + re.escape(section) + r":\s*(?:#.*)?$")
    starts = [i for i, line in enumerate(lines) if section_re.fullmatch(line)]
    if len(starts) != 1:
        return None
    start = starts[0]
    for line in lines[start + 1:]:
        if line.strip() and not line.lstrip().startswith("#") and not line[0].isspace():
            break
        match = re.match(r"^[ \t]+" + re.escape(key) + r"\s*:\s*(.*?)\s*$", line)
        if match:
            value = match.group(1).strip()
            if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
                value = value[1:-1]
            return value
    return None

client = get(sys.argv[1], "jwt", "secret")
admin = get(sys.argv[2], "admin", "jwtSecret")
if client is None or len(client.encode("utf-8")) < 32:
    raise SystemExit("jwt.secret validation failed")
if admin is None or len(admin.encode("utf-8")) < 32:
    raise SystemExit("admin.jwtSecret validation failed")
if admin == client or admin == client + "::admin":
    raise SystemExit("secret independence validation failed")
print("jwt_secret_validation=PASS")
PY

common_stage="$(mktemp "$(dirname "$COMMON_CONFIG")/.config.yaml.jwt.XXXXXX")"
secret_stage="$(mktemp "$(dirname "$SECRET_CONFIG")/.config.yaml.jwt.XXXXXX")"
install -o "$COMMON_UID" -g "$COMMON_GID" -m "$COMMON_TARGET_MODE" "$common_candidate" "$common_stage"
install -o "$SECRET_UID" -g "$SECRET_GID" -m "$SECRET_TARGET_MODE" "$secret_candidate" "$secret_stage"

CONFIGS_CHANGED=1
mv -fT "$common_stage" "$COMMON_CONFIG"
mv -fT "$secret_stage" "$SECRET_CONFIG"
if command -v restorecon >/dev/null 2>&1; then
  restorecon "$COMMON_CONFIG" "$SECRET_CONFIG" >/dev/null 2>&1 || true
fi
sync -f "$COMMON_CONFIG" 2>/dev/null || sync
sync -f "$SECRET_CONFIG" 2>/dev/null || sync

install -m 0600 "$COMMON_CONFIG" "$BACKUP_DIR/common-config.yaml.after"
install -m 0600 "$SECRET_CONFIG" "$BACKUP_DIR/secret-config.yaml.after"
sha256sum "$BACKUP_DIR/common-config.yaml.after" "$BACKUP_DIR/secret-config.yaml.after" \
  >"$BACKUP_DIR/after.sha256"
chmod 0600 "$BACKUP_DIR/after.sha256"

client_fingerprint="$(sha256sum "$client_secret_file" | awk '{print $1}')"
admin_fingerprint="$(sha256sum "$admin_secret_file" | awk '{print $1}')"
rm -f "$client_secret_file" "$admin_secret_file"

[ "$(docker inspect "$PRODUCTION_CONTAINER" --format '{{.State.Status}}')" = "running" ] || \
  die 'production backend stopped after config update'
[ "$(container_health)" = "healthy" ] || \
  die 'production backend became unhealthy after config update'

CONFIGS_CHANGED=0
log "backup=$BACKUP_DIR"
log "common_config_sha256=$(sha256sum "$COMMON_CONFIG" | awk '{print $1}')"
log "secret_config_sha256=$(sha256sum "$SECRET_CONFIG" | awk '{print $1}')"
log "client_secret_fingerprint=$client_fingerprint"
log "admin_secret_fingerprint=$admin_fingerprint"
log 'JWT configuration rotated without restarting any service'
log 'production backend remains running and healthy'
log 'after the new Backend is deployed, all C-end and Admin JWT sessions must log in again'
