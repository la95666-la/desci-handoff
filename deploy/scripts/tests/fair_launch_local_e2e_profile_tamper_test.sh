#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.."&&pwd)";D="$(mktemp -d)";trap 'rm -rf "$D"' EXIT;chmod 700 "$D";P="$D/p.cnf";PW="$D/pw";echo x>"$PW";chmod 600 "$PW";H="$ROOT/scripts/fair-launch/local-e2e/mysql-secret-helper.sh"
bash "$H" write-profile "$PW" "$P" 24491 jc_fl_e2e_0123456789abcdef0123456789abcdef local_runtime 123e4567-e89b-42d3-a456-426614174000;[ "$(bash "$H" parse-profile "$P" 24491 jc_fl_e2e_0123456789abcdef0123456789abcdef local_runtime)" = parsed ]
reject(){ local expected="$1";shift;cp "$P" "$D/t";"$@" "$D/t";chmod 600 "$D/t";set +e;out="$(bash "$H" parse-profile "$D/t" 24491 jc_fl_e2e_0123456789abcdef0123456789abcdef local_runtime 2>&1)";rc=$?;set -e;[ "$rc" -ne 0 ]&&printf '%s' "$out"|grep -Fq "$expected"||{ echo "tamper not parser-rejected: $*";exit 1;};}
replace(){ sed -i "$1" "$2";};append(){ echo "$1">>"$2";}
for spec in 's/^port=.*/port=24492/' 's/^host=.*/host=localhost/' 's/^host=.*/host=::1/' 's/^host=.*/host=0.0.0.0/' 's/^database=.*/database=jc_fl_e2e_ffffffffffffffffffffffffffffffff/' 's/^user=.*/user=local_admin/' 's/^ssl-mode=.*/ssl-mode=PREFERRED/' 's/^get-server-public-key=.*/get-server-public-key=FALSE/';do reject 'profile contract mismatch' replace "$spec";done
for extra in 'host=127.0.0.1' 'port=24491' 'ssl-mode=DISABLED' 'get-server-public-key=TRUE' 'unexpected-option=1';do reject 'profile contract mismatch' append "$extra";done
echo 'fair_launch_local_e2e_profile_tamper_test: PASS valid parser and exact parser rejection classes'
