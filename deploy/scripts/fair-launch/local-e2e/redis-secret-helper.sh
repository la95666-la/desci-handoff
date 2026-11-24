#!/usr/bin/env bash
# Intentionally separate and never traced: secret expansion and askpass transport.
set +x
set -euo pipefail
umask 077
case "${1:-}" in
  config)
    [ "$#" = 6 ] || exit 64
    secret_file="$2"; output="$3"; port="$4"; pidfile="$5"; dir="$6"
    IFS= read -r password < "$secret_file"
    cat > "$output" <<EOF
bind 127.0.0.1
protected-mode yes
port $port
daemonize yes
pidfile $pidfile
dir $dir
save ""
appendonly no
requirepass $password
logfile $dir/redis.log
EOF
    unset password
    chmod 600 "$output"
    ;;
  ping)
    [ "$#" = 4 ] || exit 64
    secret_file="$2"; host="$3"; port="$4"
    redis-cli -h "$host" -p "$port" --askpass ping < "$secret_file" 2>/dev/null | grep -Fxq PONG
    ;;
  *) exit 64;;
esac
