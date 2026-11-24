#!/usr/bin/env python3
"""Verify the complete security-relevant disposable MySQL container configuration."""
import json, os, sys

def fail(message): print("mysql-container-spec: FAIL: "+message,file=sys.stderr);raise SystemExit(1)
def envmap(values):
 out={}
 for item in values or []:
  if '=' not in item: fail('malformed environment')
  key,value=item.split('=',1)
  if not key or key in out: fail('duplicate environment key')
  out[key]=value
 return out
if len(sys.argv)!=11: fail('usage')
cp,ip,cid,image_ref,image_id,port,run_id,name,run_dir,repo_root=sys.argv[1:]
container=json.load(open(cp,encoding='utf-8'))[0];image=json.load(open(ip,encoding='utf-8'))[0];config=container['Config'];host=container['HostConfig'];network=container['NetworkSettings']
if container.get('Id')!=cid or container.get('Image')!=image_id or config.get('Image')!=image_ref: fail('container/image mismatch')
image_labels=image.get('Config',{}).get('Labels') or {}; expected_labels=dict(image_labels);expected_labels.update({'joincare.fair-launch.local-e2e-run':run_id,'joincare.fair-launch.local-e2e-component':'mysql'})
if config.get('Labels')!=expected_labels: fail('label map mismatch')
image_config=image.get('Config',{})
expected_config={'Entrypoint':image_config.get('Entrypoint'),'User':image_config.get('User') or '','WorkingDir':image_config.get('WorkingDir') or '','StopSignal':image_config.get('StopSignal'),'Healthcheck':image_config.get('Healthcheck'),'ExposedPorts':image_config.get('ExposedPorts') or {},'Volumes':image_config.get('Volumes') or {},'AttachStderr':False,'AttachStdin':False,'AttachStdout':False,'Domainname':'','OpenStdin':False,'StdinOnce':False,'Tty':False}
for key,value in expected_config.items():
 if config.get(key)!=value: fail('container config mismatch: '+key)
cmd=['--skip-log-bin','--local-infile=OFF','--secure-file-priv=NULL','--skip-name-resolve','--bind-address=127.0.0.1',f'--port={port}','--mysqlx=0']
if config.get('Cmd')!=cmd or container.get('Name')!='/'+name: fail('command/name mismatch')
if host.get('NetworkMode')!='host' or (host.get('PortBindings') or {})!={} or (network.get('Ports') or {})!={}: fail('network/publish mismatch')
if (host.get('Tmpfs') or {})!={'/tmp':'rw,nosuid,nodev,size=256m','/var/lib/mysql':'rw,nosuid,nodev,size=2g'}: fail('tmpfs mismatch')
expected_mounts=sorted([{'Type':'bind','Source':os.path.realpath(os.path.join(run_dir,'secrets/mysql-root')),'Destination':'/run/secrets/root','Mode':'ro','RW':False,'Propagation':'rprivate'},{'Type':'bind','Source':os.path.realpath(repo_root),'Destination':'/repo','Mode':'ro','RW':False,'Propagation':'rprivate'}],key=lambda x:x['Destination'])
actual=sorted([{k:m.get(k) for k in ('Type','Source','Destination','Mode','RW','Propagation')} for m in container.get('Mounts',[])],key=lambda x:x['Destination'])
if actual!=expected_mounts: fail('mount set mismatch')
expected_env=envmap(image.get('Config',{}).get('Env'));expected_env['MYSQL_ROOT_PASSWORD_FILE']='/run/secrets/root'
if envmap(config.get('Env'))!=expected_env: fail('environment mismatch')
expected_binds=sorted([os.path.realpath(os.path.join(run_dir,'secrets/mysql-root'))+':/run/secrets/root:ro',os.path.realpath(repo_root)+':/repo:ro'])
if sorted(host.get('Binds') or [])!=expected_binds: fail('bind declaration mismatch')
expected_security={'Privileged':False,'ReadonlyRootfs':False,'CapAdd':None,'CapDrop':None,'SecurityOpt':None,'Devices':[],'DeviceRequests':None,'DeviceCgroupRules':None,'GroupAdd':None,'Sysctls':None,'Ulimits':[],'PidMode':'','IpcMode':'private','UTSMode':'','UsernsMode':'','CgroupnsMode':'private','Cgroup':'','CgroupParent':'','RestartPolicy':{'Name':'no','MaximumRetryCount':0},'AutoRemove':False,'OomKillDisable':None,'Init':None,'ShmSize':67108864,'Runtime':'runc','PublishAllPorts':False,'Memory':0,'MemoryReservation':0,'MemorySwap':0,'MemorySwappiness':None,'NanoCpus':0,'CpuShares':0,'CpuPeriod':0,'CpuQuota':0,'CpuRealtimePeriod':0,'CpuRealtimeRuntime':0,'CpusetCpus':'','CpusetMems':'','BlkioWeight':0,'BlkioWeightDevice':[],'BlkioDeviceReadBps':[],'BlkioDeviceWriteBps':[],'BlkioDeviceReadIOps':[],'BlkioDeviceWriteIOps':[],'PidsLimit':None,'OomScoreAdj':0,'Dns':None,'DnsOptions':[],'DnsSearch':[],'ExtraHosts':None,'Links':None,'VolumesFrom':None,'LogConfig':{'Type':'json-file','Config':{}},'Isolation':'','VolumeDriver':'','ContainerIDFile':'','StorageOpt':None,'MaskedPaths':['/proc/acpi','/proc/asound','/proc/interrupts','/proc/kcore','/proc/keys','/proc/latency_stats','/proc/sched_debug','/proc/scsi','/proc/timer_list','/proc/timer_stats','/sys/devices/virtual/powercap','/sys/firmware'],'ReadonlyPaths':['/proc/bus','/proc/fs','/proc/irq','/proc/sys','/proc/sysrq-trigger']}
for key,value in expected_security.items():
 if host.get(key)!=value: fail('host security mismatch: '+key)
print('verified')
