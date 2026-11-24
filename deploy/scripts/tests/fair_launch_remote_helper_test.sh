#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
CALLS="$TMP/calls"; : > "$CALLS"
for command in ssh docker; do
  cat > "$TMP/$command" <<'SH'
#!/usr/bin/env bash
printf '%s\n' "$0 $*" >> "$CALLS"
exit 99
SH
  chmod +x "$TMP/$command"
done
(
  source "$ROOT/scripts/lib/env.sh"
  export CALLS PATH="$TMP:$PATH"
  unset JC_REMOTE JC_COMPOSE_PROJECT JC_SSH_USER JC_HOST JC_ENV_FILE || true

  for spec in \
    'remote_ssh|remote_service_status_unavailable' \
    'remote_compose_command|remote_service_status_unavailable' \
    'remote_compose_status|remote_service_status_unavailable' \
    'remote_compose_logs|service_log_transport_unavailable'; do
    fn="${spec%%|*}"; expected="${spec#*|}"
    for args in '' 'frontend 10 0' '-- -oProxyCommand=id ; rm -rf /'; do
      set +e
      # shellcheck disable=SC2086
      out="$($fn $args 2>&1)"; rc=$?
      set -e
      [ "$rc" -eq 78 ] && [ "$out" = "$expected" ] || {
        printf '%s args=%q rc=%s out=%s\n' "$fn" "$args" "$rc" "$out" >&2
        exit 1
      }
    done
  done

  # Validators remain pure unit-testable helpers and never invoke a process.
  error(){ :; }
  JC_REMOTE=/opt/joincare-prod JC_COMPOSE_PROJECT=joincare-prod JC_SSH_USER=deploy JC_HOST=release.example.com
  validate_deploy_target
  for bad in '/opt/joincare prod' '/opt/joincare;id' '/opt/../root' 'relative/path' '/opt//joincare'; do
    JC_REMOTE="$bad"; ! validate_deploy_target
  done
  JC_REMOTE=/opt/joincare-prod
  for bad in 'JoinCare' 'joincare prod' 'joincare;id' '-joincare' 'joincare/prod'; do JC_COMPOSE_PROJECT="$bad"; ! validate_deploy_target; done
  JC_COMPOSE_PROJECT=joincare-prod
  for bad in '-oProxyCommand=id' 'root;id' 'root user' 'Root' 'root@evil'; do JC_SSH_USER="$bad"; ! validate_deploy_target; done
  JC_SSH_USER=deploy
  for bad in '-oProxyCommand=id' 'host;id' 'host name' 'user@host' 'host:22' '$(id)' '.host' 'host.' 'host..name' 'bad_name' '256.1.1.1' '1.2.3.999' '[abcdef]' '[::1%eth0]'; do JC_HOST="$bad"; ! validate_deploy_target; done
  JC_HOST='[2001:db8::1]'; validate_deploy_target
)
[ ! -s "$CALLS" ]
echo 'Remote helper zero-process boundary: PASS'
