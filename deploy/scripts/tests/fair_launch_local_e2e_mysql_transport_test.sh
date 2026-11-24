#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)";. "$ROOT/scripts/tests/fair_launch_local_e2e_test_cleanup.sh"; BACKEND_ROOT="${JOINCARE_BACKEND_ROOT:-/mnt/d/LA/desci/joincare_code/joincare_code/_extracted/joincare_server-master}"; STATE_ROOT="$(jc_fl_test_root_create JC_FL_TEST_MYSQL_TRANSPORT_V1)"; RUN_ID=''; MYSQL_PORT=''
fail(){ echo "fair_launch_local_e2e_mysql_transport_test: FAIL: $*" >&2; exit 1; }
cleanup(){ local original=$? ok=1;set +e;if [ -n "$RUN_ID" ]&&[ -d "$STATE_ROOT/$RUN_ID" ];then jc_fl_test_bounded_teardown "$STATE_ROOT" "$RUN_ID"||ok=0;fi;if [ "$ok" = 1 ];then jc_fl_test_root_finalize "$STATE_ROOT"||ok=0;fi;if [ "$ok" != 1 ];then echo "mysql_transport cleanup retained evidence at $STATE_ROOT/$RUN_ID" >&2;fi;[ "$original" -ne 0 ]&&return "$original";[ "$ok" = 1 ];}
trap cleanup EXIT INT TERM
[ -f "$BACKEND_ROOT/go.mod" ] || fail 'Backend Go module unavailable for read-only driver consumption'
grep -Eq '^[[:space:]]*github[.]com/go-sql-driver/mysql[[:space:]]+v1[.]8[.]1([[:space:]]|$)' "$BACKEND_ROOT/go.mod" || fail 'Backend MySQL driver must be exactly v1.8.1'
before_containers="$(docker -H unix:///var/run/docker.sock ps -aq --filter label=joincare.fair-launch.local-e2e-run | sort)"; before_networks="$(docker -H unix:///var/run/docker.sock network ls -q --filter label=joincare.fair-launch.local-e2e-run | sort)"
out="$(JC_FL_E2E_STATE_ROOT="$STATE_ROOT" bash "$ROOT/scripts/fair-launch/local-e2e/start.sh")"; RUN_ID="$(printf '%s\n' "$out" | sed -n 's/^RUN_ID=//p')"; [ -n "$RUN_ID" ] || fail 'start did not return run id'; DIR="$STATE_ROOT/$RUN_ID"
JC_FL_E2E_STATE_ROOT="$STATE_ROOT" bash "$ROOT/scripts/fair-launch/local-e2e/apply-schema.sh" "$RUN_ID"
JC_FL_E2E_STATE_ROOT="$STATE_ROOT";jc_fl_e2e_load_state "$RUN_ID"
[ "$MYSQL_ENDPOINT" = "127.0.0.1:$MYSQL_PORT" ] || fail 'state endpoint mismatch'
[ "$(docker -H unix:///var/run/docker.sock inspect --format '{{.HostConfig.NetworkMode}}|{{json .HostConfig.PortBindings}}|{{json .NetworkSettings.Ports}}' "$MYSQL_CONTAINER_ID")" = 'host|{}|{}' ] || fail 'Docker host-network/no-publish identity mismatch'
ss -H -ltn "sport = :$MYSQL_PORT" | awk -v ep="127.0.0.1:$MYSQL_PORT" 'NF && $4 != ep {exit 1} END {if(NR==0)exit 1}' || fail 'listener is not exact loopback'
[ "$MYSQL_NETWORK_ID|$MYSQL_NETWORK_NAME" = 'network-not-used|network-not-used' ] || fail 'unexpected Docker network identity'
for role in runtime admin job ingest; do [ "$(stat -Lc '%a' "$DIR/credentials/mysql-$role.cnf")" = 600 ] || fail "$role profile mode"; done
[ "$(stat -Lc '%a' "$DIR" "$DIR/credentials" | tr '\n' ' ')" = '700 700 ' ] || fail 'run/profile directory mode'
# Run a real WSL host Go process. It imports only the already-pinned Backend module driver;
# no password/DSN is placed in argv or environment and the Backend tree remains read-only.
(cd "$BACKEND_ROOT" && GOPROXY=off GOFLAGS=-mod=readonly GOEXPERIMENT=jsonv2 timeout --foreground 240s go run "$ROOT/scripts/tests/fair_launch_local_e2e_host_pool_probe.go" "$DIR/credentials")
JC_FL_E2E_STATE_ROOT="$STATE_ROOT" bash "$ROOT/scripts/fair-launch/local-e2e/status.sh" "$RUN_ID" | grep -Fq 'mysql_credential_state=ready' || fail 'status did not verify profiles'
JC_FL_E2E_STATE_ROOT="$STATE_ROOT" bash "$ROOT/scripts/fair-launch/local-e2e/teardown.sh" "$RUN_ID" >/dev/null; RUN_ID=''
[ ! -d "$DIR" ] || fail 'run directory residue'
! ss -H -ltn "sport = :$MYSQL_PORT" | grep -q . || fail 'MySQL listener residue'
after_containers="$(docker -H unix:///var/run/docker.sock ps -aq --filter label=joincare.fair-launch.local-e2e-run | sort)"; after_networks="$(docker -H unix:///var/run/docker.sock network ls -q --filter label=joincare.fair-launch.local-e2e-run | sort)"
[ "$before_containers" = "$after_containers" ] || fail 'labelled container residue'; [ "$before_networks" = "$after_networks" ] || fail 'labelled network residue'
echo 'fair_launch_local_e2e_mysql_transport_test: PASS exact-loopback host-network-no-publish profiles=4 host-go-pools=4 commit rollback for-update policy cleanup'
