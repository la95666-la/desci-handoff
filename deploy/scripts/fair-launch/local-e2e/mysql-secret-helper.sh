#!/usr/bin/env bash
# LOCAL-ONLY secret boundary. The caller may be traced; this helper disables xtrace
# before it reads a role password/profile and never exports it.
set +x
set -euo pipefail
fail(){ echo "fair-launch-local-e2e mysql helper: FAIL: $*" >&2; exit 1; }
[ "$#" -ge 1 ] || fail 'mode required'
mode="$1"; shift
case "$mode" in
  emit-role-passwords)
    [ "$#" = 1 ] || fail 'usage: emit-role-passwords <secret-dir>'
    secret_dir="$1"
    [ "$(stat -Lc '%a' "$secret_dir" 2>/dev/null)" = 700 ] || fail 'secret directory mode mismatch'
    for role in runtime admin job ingest; do printf '%s\t%s\n' "$role" "$(cat "$secret_dir/mysql-$role")"; done
    ;;
  write-profile)
    [ "$#" = 6 ] || fail 'usage: write-profile <password-file> <profile> <port> <database> <user> <run-id>'
    password_file="$1"; profile="$2"; port="$3"; database="$4"; user="$5"; run_id="$6"
    [ -f "$password_file" ] || fail 'password file missing'
    [[ "$port" =~ ^[0-9]+$ ]] && [ "$port" -ge 1024 ] && [ "$port" -le 65535 ] || fail 'invalid port'
    [[ "$database" =~ ^jc_fl_e2e_[0-9a-f]{32}$ ]] || fail 'invalid database'
    [[ "$user" =~ ^local_(runtime|admin|job|ingest)$ ]] || fail 'invalid role user'
    [[ "$run_id" =~ ^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$ ]] || fail 'invalid run id'
    password="$(cat "$password_file")"; [ -n "$password" ] || fail 'empty password'
    umask 077
    tmp="$profile.tmp.$$"
    trap 'rm -f "$tmp"' EXIT INT TERM
    printf '[client]\nhost=127.0.0.1\nport=%s\nprotocol=tcp\ndatabase=%s\nuser=%s\npassword=%s\nssl-mode=DISABLED\nget-server-public-key=TRUE\nconnect-timeout=5\n# local-only-run-id=%s\n' "$port" "$database" "$user" "$password" "$run_id" > "$tmp"
    chmod 600 "$tmp"; mv "$tmp" "$profile"; trap - EXIT INT TERM
    ;;
  verify-root-host)
    [ "$#" = 4 ] || fail 'usage: verify-root-host <password-file> <port> <uuid> <version>'
    password_file="$1"; port="$2"; uuid="$3"; version="$4"
    [ -f "$password_file" ] || fail 'root password file missing'
    [[ "$port" =~ ^[0-9]+$ ]] || fail 'invalid root verification port'
    password="$(cat "$password_file")"; tmp="$(dirname "$password_file")/.root-host-verify.$$"
    umask 077; trap 'rm -f "$tmp"' EXIT INT TERM
    printf '[client]\nhost=127.0.0.1\nport=%s\nprotocol=tcp\nuser=root\npassword=%s\nssl-mode=DISABLED\nget-server-public-key=TRUE\nconnect-timeout=5\n' "$port" "$password" > "$tmp"; chmod 600 "$tmp"
    observed="$(timeout --foreground 20s mysql --defaults-extra-file="$tmp" --batch --raw --skip-column-names -e 'SELECT CONCAT(LOWER(@@server_uuid),"|",VERSION());' 2>/dev/null)" || fail 'root host verification connection failed'
    [ "$observed" = "$uuid|$version" ] || fail 'root host verification identity mismatch'
    rm -f "$tmp"; trap - EXIT INT TERM; echo verified
    ;;
  parse-profile)
    [ "$#" = 4 ] || fail 'usage: parse-profile <profile> <port> <database> <user>'
    profile="$1"; expected_port="$2"; database="$3"; user="$4"
    [ "$(stat -Lc '%a' "$profile" 2>/dev/null)" = 600 ] || fail 'profile mode mismatch'
    python3 - "$profile" "$expected_port" "$database" "$user" <<'PY' || fail 'profile contract mismatch'
import sys
path,port,database,user=sys.argv[1:]
expected={'host':'127.0.0.1','port':port,'protocol':'tcp','database':database,'user':user,'ssl-mode':'DISABLED','get-server-public-key':'TRUE','connect-timeout':'5'}
allowed=set(expected)|{'password'};seen={}
with open(path,encoding='utf-8') as f:
 for raw in f:
  line=raw.rstrip('\n')
  if not line or line=='[client]' or line.startswith('# local-only-run-id='): continue
  if '=' not in line: raise SystemExit(1)
  k,v=line.split('=',1)
  if k not in allowed or k in seen: raise SystemExit(1)
  seen[k]=v
if set(seen)!=allowed or any(seen[k]!=v for k,v in expected.items()) or not seen['password']: raise SystemExit(1)
PY
    echo parsed
    ;;
  verify-profile)
    [ "$#" = 6 ] || fail 'usage: verify-profile <profile> <port> <uuid> <version> <database> <user>'
    profile="$1"; expected_port="$2"; uuid="$3"; version="$4"; database="$5"; user="$6"
    [ "$(stat -Lc '%a' "$profile" 2>/dev/null)" = 600 ] || fail 'profile mode mismatch'
    [ "$(bash "$0" parse-profile "$profile" "$expected_port" "$database" "$user")" = parsed ] || fail 'profile contract mismatch'
    observed="$(timeout --foreground 20s mysql --defaults-extra-file="$profile" --batch --raw --skip-column-names -e 'SELECT CONCAT(CURRENT_USER(),"|",DATABASE(),"|",LOWER(@@server_uuid),"|",VERSION(),"|",@@FOREIGN_KEY_CHECKS);' 2>/dev/null)" || fail 'profile connection failed'
    [ "$observed" = "$user@%|$database|$uuid|$version|1" ] || fail 'profile server identity mismatch'
    echo verified
    ;;
  exec)
    [ "$#" -ge 1 ] || fail 'usage: exec <profile> [mysql args...]'
    profile="$1"; shift
    [ "$(stat -Lc '%a' "$profile" 2>/dev/null)" = 600 ] || fail 'profile mode mismatch'
    exec timeout --foreground 60s mysql --defaults-extra-file="$profile" --batch --raw --skip-column-names "$@"
    ;;
  *) fail 'unsupported mode' ;;
esac
