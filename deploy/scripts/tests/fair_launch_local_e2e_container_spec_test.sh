#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.."&&pwd)";. "$ROOT/scripts/tests/fair_launch_local_e2e_test_cleanup.sh";S="$(jc_fl_test_root_create JC_FL_TEST_CONTAINER_SPEC_V1)";RUN='';cleanup(){ local original=$? ok=1;set +e;if [ -n "$RUN" ]&&[ -d "$S/$RUN" ];then jc_fl_test_bounded_teardown "$S" "$RUN"||ok=0;fi;if [ "$ok" = 1 ];then jc_fl_test_root_finalize "$S"||ok=0;fi;[ "$ok" = 1 ]||echo "container_spec cleanup retained evidence at $S/$RUN" >&2;[ "$original" -ne 0 ]&&return "$original";[ "$ok" = 1 ];};trap cleanup EXIT
out="$(JC_FL_E2E_STATE_ROOT="$S" bash "$ROOT/scripts/fair-launch/local-e2e/start.sh")";RUN="$(echo "$out"|sed -n 's/^RUN_ID=//p')";JC_FL_E2E_STATE_ROOT="$S";jc_fl_e2e_load_state "$RUN";docker -H unix:///var/run/docker.sock inspect "$MYSQL_CONTAINER_ID">"$S/c.json";docker -H unix:///var/run/docker.sock image inspect "$MYSQL_IMAGE_REF">"$S/i.json";H="$ROOT/scripts/fair-launch/local-e2e/mysql-container-spec.py"
args=("$S/i.json" "$MYSQL_CONTAINER_ID" "$MYSQL_IMAGE_REF" "$MYSQL_IMAGE_ID" "$MYSQL_PORT" "$RUN_ID" "$MYSQL_NAME" "$RUN_DIR" "$ROOT");python3 "$H" "$S/c.json" "${args[@]}"|grep -Fxq verified
reject(){ local kind="$1";cp "$S/c.json" "$S/t.json";python3 - "$S/t.json" "$kind" <<'PY'
import json,sys
p,k=sys.argv[1:];o=json.load(open(p))[0]
if k=='component':o['Config']['Labels']['joincare.fair-launch.local-e2e-component']='foreign'
elif k=='image':o['Image']='sha256:'+'0'*64
elif k=='image-ref':o['Config']['Image']='mysql@sha256:'+'0'*64
elif k=='append-command':o['Config']['Cmd'].append('--bind-address=0.0.0.0')
elif k=='missing-command':o['Config']['Cmd'].remove('--skip-name-resolve')
elif k=='extra-mount':o['Mounts'].append({'Type':'bind','Source':'/tmp','Destination':'/foreign','Mode':'rw','RW':True,'Propagation':'rprivate'})
elif k=='mount-mode':o['Mounts'][0]['RW']=True;o['Mounts'][0]['Mode']='rw'
elif k=='tmpfs':o['HostConfig']['Tmpfs']['/tmp']='rw'
elif k=='env':o['Config']['Env'].append('MYSQL_ALLOW_EMPTY_PASSWORD=1')
elif k=='extra-label':o['Config']['Labels']['foreign']='1'
elif k=='privileged':o['HostConfig']['Privileged']=True
elif k=='cap':o['HostConfig']['CapAdd']=['SYS_ADMIN']
elif k=='device':o['HostConfig']['Devices']=[{'PathOnHost':'/dev/null','PathInContainer':'/dev/x','CgroupPermissions':'rwm'}]
elif k=='security':o['HostConfig']['SecurityOpt']=['seccomp=unconfined']
elif k=='pid':o['HostConfig']['PidMode']='host'
elif k=='ipc':o['HostConfig']['IpcMode']='host'
elif k=='restart':o['HostConfig']['RestartPolicy']={'Name':'always','MaximumRetryCount':0}
elif k=='group-add':o['HostConfig']['GroupAdd']=['12345']
elif k=='device-rule':o['HostConfig']['DeviceCgroupRules']=['c 1:3 r']
elif k=='sysctl':o['HostConfig']['Sysctls']={'net.ipv4.ip_local_port_range':'1024 65535'}
elif k=='ulimit':o['HostConfig']['Ulimits']=[{'Name':'nofile','Soft':1024,'Hard':1024}]
elif k=='memory':o['HostConfig']['Memory']=1048576
elif k=='entrypoint':o['Config']['Entrypoint']=['/bin/sh']
elif k=='user':o['Config']['User']='65534'
elif k=='workdir':o['Config']['WorkingDir']='/tmp'
elif k=='stop-signal':o['Config']['StopSignal']='SIGKILL'
elif k=='healthcheck':o['Config']['Healthcheck']={'Test':['CMD-SHELL','exit 0']}
elif k=='exposed':o['Config']['ExposedPorts']['9999/tcp']={}
elif k=='volume':o['Config']['Volumes']['/foreign']={}
elif k=='bind-declaration':o['HostConfig']['Binds'].append('/tmp:/foreign:ro')
elif k=='masked':o['HostConfig']['MaskedPaths']=[]
elif k=='readonly-paths':o['HostConfig']['ReadonlyPaths']=[]
elif k=='storage':o['HostConfig']['StorageOpt']={'size':'1G'}
elif k=='short-id':o['Id']=o['Id'][:12]
open(p,'w').write(json.dumps([o]))
PY
set +e;python3 "$H" "$S/t.json" "${args[@]}">"$S/e" 2>&1;rc=$?;set -e;[ "$rc" -ne 0 ]||{ echo "$kind accepted";exit 1;};docker -H unix:///var/run/docker.sock inspect "$MYSQL_CONTAINER_ID">/dev/null;}
for k in component image image-ref append-command missing-command extra-mount mount-mode tmpfs env extra-label privileged cap device security pid ipc restart group-add device-rule sysctl ulimit memory entrypoint user workdir stop-signal healthcheck exposed volume bind-declaration masked readonly-paths storage short-id;do reject "$k";done
echo 'fair_launch_local_e2e_container_spec_test: PASS exact cmd mounts tmpfs env image labels and mismatch preserves owned fixture'
