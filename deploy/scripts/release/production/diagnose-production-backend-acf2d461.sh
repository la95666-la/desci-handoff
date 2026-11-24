#!/usr/bin/env bash
set -Eeuo pipefail
umask 077
PATH=/usr/sbin:/usr/bin:/sbin:/bin
export PATH

PRODUCTION_CONTAINER="jc-backend"
DIAGNOSTIC_CONTAINER="jc-backend-diag-acf2d461"
CANDIDATE_IMAGE="joincare/backend:candidate-acf2d46"
EXPECTED_IMAGE_ID="sha256:bbe904f16a65f2bb2c79cef87ac20baab026eea42dd51eac8f0ed8173d827cde"
CONFIG_DIR="/opt/joincare-prod/config"
WORK_DIR="/root/jc-backend-diag-acf2d461"
ENV_FILE="$WORK_DIR/current.env"
RAW_LOG="$WORK_DIR/candidate.log"
REPORT="$WORK_DIR/report.txt"
MAX_SECONDS=45

die() {
  printf '[backend-diag] ERROR: %s\n' "$*" >&2
  exit 1
}

log() {
  printf '[backend-diag] %s\n' "$*"
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "$1 is required"
}

container_health() {
  docker inspect "$1" \
    --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}'
}

container_status() {
  docker inspect "$1" --format '{{.State.Status}}'
}

sanitize_log() {
  sed -E \
    -e 's#(https?|mysql|redis)://[^/@[:space:]]+@#\1://<redacted>@#g' \
    -e 's#([^[:space:]:]+):[^@[:space:]]+@tcp#\1:<redacted>@tcp#g' \
    -e 's#((password|passwd|secret|token|private[_-]?key|api[_-]?key)["'"'"'= :]+)[^,"'"'"'[:space:]]+#\1<redacted>#Ig' \
    "$RAW_LOG"
}

cleanup() {
  docker rm -f "$DIAGNOSTIC_CONTAINER" >/dev/null 2>&1 || true
  rm -f "$ENV_FILE"
}

on_exit() {
  local rc=$?
  cleanup
  if docker inspect "$PRODUCTION_CONTAINER" >/dev/null 2>&1; then
    local final_status final_health
    final_status="$(container_status "$PRODUCTION_CONTAINER" 2>/dev/null || true)"
    final_health="$(container_health "$PRODUCTION_CONTAINER" 2>/dev/null || true)"
    printf '[backend-diag] production_after: status=%s health=%s\n' \
      "$final_status" "$final_health"
  fi
  exit "$rc"
}

trap on_exit EXIT HUP INT TERM

[ "$(id -u)" -eq 0 ] || die 'run as root: sudo bash diagnose-production-backend-acf2d461.sh'
require_command docker
require_command sed
require_command grep
require_command sha256sum

docker inspect "$PRODUCTION_CONTAINER" >/dev/null 2>&1 || \
  die "$PRODUCTION_CONTAINER does not exist"

production_status="$(container_status "$PRODUCTION_CONTAINER")"
production_health="$(container_health "$PRODUCTION_CONTAINER")"
production_image="$(docker inspect "$PRODUCTION_CONTAINER" --format '{{.Config.Image}}')"
production_restarts="$(docker inspect "$PRODUCTION_CONTAINER" --format '{{.RestartCount}}')"

log "production_before: image=$production_image status=$production_status health=$production_health restart=$production_restarts"
[ "$production_status" = "running" ] || die 'production backend is not running'
[ "$production_health" = "healthy" ] || die 'production backend is not healthy'

actual_image_id="$(docker image inspect "$CANDIDATE_IMAGE" --format '{{.Id}}' 2>/dev/null || true)"
[ -n "$actual_image_id" ] || die "candidate image is missing: $CANDIDATE_IMAGE"
[ "$actual_image_id" = "$EXPECTED_IMAGE_ID" ] || \
  die "candidate image ID mismatch: $actual_image_id"
log "candidate_image_verified=$actual_image_id"

[ -d "$CONFIG_DIR" ] && [ ! -L "$CONFIG_DIR" ] || \
  die "production config directory is missing or a symlink: $CONFIG_DIR"

mapfile -t networks < <(
  docker inspect "$PRODUCTION_CONTAINER" \
    --format '{{range $name, $config := .NetworkSettings.Networks}}{{println $name}}{{end}}' |
    sed '/^[[:space:]]*$/d'
)
[ "${#networks[@]}" -eq 1 ] || \
  die "expected exactly one production Docker network, found ${#networks[@]}"
network="${networks[0]}"
log "diagnostic_network=$network"

install -d -m 0700 "$WORK_DIR"
rm -f "$ENV_FILE" "$RAW_LOG" "$REPORT"
cleanup

docker inspect "$PRODUCTION_CONTAINER" \
  --format '{{range .Config.Env}}{{println .}}{{end}}' >"$ENV_FILE"
chmod 0600 "$ENV_FILE"
[ -s "$ENV_FILE" ] || die 'failed to capture production container environment'

log 'starting isolated diagnostic container (no published ports, restart disabled)'
diagnostic_id="$({
  docker run -d \
    --name "$DIAGNOSTIC_CONTAINER" \
    --restart no \
    --network "$network" \
    --env-file "$ENV_FILE" \
    --mount "type=bind,src=$CONFIG_DIR,dst=/app/config,readonly" \
    --tmpfs /app/logs:rw,noexec,nosuid,nodev,size=64m \
    --tmpfs /app/uploads:rw,noexec,nosuid,nodev,size=64m \
    --label joincare.diagnostic=backend-acf2d461 \
    "$CANDIDATE_IMAGE"
} 2>>"$RAW_LOG")" || die 'docker run failed; inspect the report directory'

[ -n "$diagnostic_id" ] || die 'docker run returned an empty container ID'
log "diagnostic_container_id=${diagnostic_id:0:12}"

health_result="not_reached"
exit_code="running_at_timeout"
elapsed=0

while [ "$elapsed" -lt "$MAX_SECONDS" ]; do
  sleep 2
  elapsed=$((elapsed + 2))
  state="$(container_status "$DIAGNOSTIC_CONTAINER" 2>/dev/null || true)"
  if [ "$state" != "running" ]; then
    exit_code="$(docker inspect "$DIAGNOSTIC_CONTAINER" --format '{{.State.ExitCode}}' 2>/dev/null || printf unknown)"
    log "candidate_exited_after_seconds=$elapsed exit_code=$exit_code"
    break
  fi

  if docker exec "$DIAGNOSTIC_CONTAINER" sh -c \
    'curl -sSf --max-time 2 http://127.0.0.1:13004/health >/dev/null' \
    >/dev/null 2>&1; then
    health_result="passed"
    log "candidate_health_passed_after_seconds=$elapsed"
    break
  fi
done

docker logs --timestamps "$DIAGNOSTIC_CONTAINER" >>"$RAW_LOG" 2>&1 || true
chmod 0600 "$RAW_LOG"

{
  printf 'format=JOINCARE_BACKEND_ACF2D461_DIAGNOSTIC_V1\n'
  printf 'candidate_image=%s\n' "$CANDIDATE_IMAGE"
  printf 'candidate_image_id=%s\n' "$actual_image_id"
  printf 'network=%s\n' "$network"
  printf 'elapsed_seconds=%s\n' "$elapsed"
  printf 'candidate_health=%s\n' "$health_result"
  printf 'candidate_exit_code=%s\n' "$exit_code"
  printf 'production_before_status=%s\n' "$production_status"
  printf 'production_before_health=%s\n' "$production_health"
  printf 'raw_log_sha256=%s\n' "$(sha256sum "$RAW_LOG" | awk '{print $1}')"
  printf '\n--- redacted candidate log ---\n'
  sanitize_log | tail -n 300
} >"$REPORT"
chmod 0600 "$REPORT"

rm -f "$ENV_FILE"
docker rm -f "$DIAGNOSTIC_CONTAINER" >/dev/null 2>&1 || true

final_status="$(container_status "$PRODUCTION_CONTAINER")"
final_health="$(container_health "$PRODUCTION_CONTAINER")"
[ "$final_status" = "running" ] || die 'production backend stopped during diagnosis'
[ "$final_health" = "healthy" ] || die 'production backend became unhealthy during diagnosis'

log "candidate_health=$health_result"
log "candidate_exit_code=$exit_code"
log "report=$REPORT"
log "report_sha256=$(sha256sum "$REPORT" | awk '{print $1}')"
log 'production backend remained running and healthy'
log 'show the redacted report with:'
printf 'sudo cat %q\n' "$REPORT"
