#!/usr/bin/env python3
"""Strict immutable LOCAL-ONLY test-root and lifecycle-phase authority records."""
import json,os,re,stat,sys,tempfile
UUID=re.compile(r'^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$')
ANY_UUID=re.compile(r'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')
HEX64=re.compile(r'^[0-9a-f]{64}$')
DOMAIN=re.compile(r'^JC_FL_TEST_[A-Z0-9_]+_V1$')
ROOT_FIELDS={'domain','version','testDomain','rootPath','rootDevice','rootInode','rootUid','rootMode','markerPath'}
PHASE_FIELDS={'domain','version','runId','runPath','runDevice','runInode','runUid','runMode','recordPath','stage','redisLaunchAttempted','dockerLaunchAttempted'}
STAGES={'scaffolded','redis-launch-attempted','redis-authority-published','docker-launch-attempted','transport-published'}
STAGE_ORDER=('scaffolded','redis-launch-attempted','redis-authority-published','docker-launch-attempted','transport-published')
STAGE_STATE={'scaffolded':(False,False),'redis-launch-attempted':(True,False),'redis-authority-published':(True,False),'docker-launch-attempted':(True,True),'transport-published':(True,True)}
NEXT_STAGE={'scaffolded':'redis-launch-attempted','redis-launch-attempted':'redis-authority-published','redis-authority-published':'docker-launch-attempted','docker-launch-attempted':'transport-published'}
STATE_BASE=('RUN_ID','RUN_HEX','RUN_DIR','MYSQL_NAME','MYSQL_CONTAINER_ID','MYSQL_CONTAINER_PID','MYSQL_CONTAINER_STARTED_AT','MYSQL_IMAGE_REF','MYSQL_IMAGE_ID','MYSQL_NETWORK_MODE','MYSQL_NETWORK_NAME','MYSQL_NETWORK_ID','MYSQL_PORT','MYSQL_ENDPOINT','MYSQL_CONTAINER_PORT','MYSQL_PROTOCOL','MYSQL_SERVER_UUID','MYSQL_VERSION','MYSQL_RESERVATION_DIGEST','MYSQL_TRANSPORT_DIGEST','MYSQL_SCHEMA_READY','DB_NAME','REDIS_PORT','RPC_PORT','CORE_PORT','ADMIN_PORT','FRONTEND_PORT')
STATE_READY=('BASELINE_DIGEST','MIGRATION_MANIFEST_DIGEST','SCHEMA_DIGEST','GRANT_POLICY_DIGEST','OWNER_LABEL_DIGEST')
def fail(msg): print('fair-launch test authority: EXPECTED_REFUSAL: '+msg,file=sys.stderr);raise SystemExit(1)
def pairs(items):
 out={}
 for k,v in items:
  if k in out:fail('duplicate field')
  out[k]=v
 return out
def load(path,fields):
 try:
  if path!=os.path.abspath(path):fail('record path not absolute canonical')
  st=os.lstat(path)
  if not stat.S_ISREG(st.st_mode) or stat.S_IMODE(st.st_mode)!=0o600 or stat.S_ISLNK(st.st_mode) or st.st_uid!=os.getuid() or st.st_nlink!=1:fail('record mode/type')
  if os.path.abspath(path)!=os.path.realpath(path):fail('record canonical path')
  with open(path,encoding='utf-8') as f:o=json.load(f,object_pairs_hook=pairs)
 except (OSError,json.JSONDecodeError) as e:fail('invalid record: '+str(e))
 if not isinstance(o,dict) or set(o)!=fields:fail('record fields')
 return o
def atomic(path,obj):
 if path!=os.path.abspath(path) or os.path.dirname(path)!=os.path.realpath(os.path.dirname(path)):fail('atomic path authority')
 parent=os.lstat(os.path.dirname(path))
 if stat.S_ISLNK(parent.st_mode) or not stat.S_ISDIR(parent.st_mode) or stat.S_IMODE(parent.st_mode)!=0o700 or parent.st_uid!=os.getuid():fail('atomic parent authority')
 raw=(json.dumps(obj,sort_keys=True,separators=(',',':'))+'\n').encode();fd,tmp=tempfile.mkstemp(prefix='.authority.',dir=os.path.dirname(path));os.fchmod(fd,0o600)
 try:os.write(fd,raw);os.fsync(fd)
 finally:os.close(fd)
 os.replace(tmp,path);os.chmod(path,0o600)
def dir_identity(path,mode):
 if path!=os.path.abspath(path):fail('directory path not absolute canonical')
 try:st=os.lstat(path)
 except OSError:fail('directory missing')
 if stat.S_ISLNK(st.st_mode) or not stat.S_ISDIR(st.st_mode):fail('directory type')
 if stat.S_IMODE(st.st_mode)!=mode or st.st_uid!=os.getuid():fail('directory protection')
 canonical=os.path.realpath(path);post=os.lstat(canonical)
 if path!=canonical or (st.st_dev,st.st_ino)!=(post.st_dev,post.st_ino):fail('directory canonical identity')
 return canonical,st
def root_write(root,test_domain):
 root,st=dir_identity(root,0o700)
 if not DOMAIN.fullmatch(test_domain):fail('test domain')
 marker=os.path.join(root,'.jc-fl-test-root.json')
 if os.path.lexists(marker):fail('root marker exists')
 atomic(marker,{'domain':'JC_FL_TEST_ROOT_AUTHORITY','version':1,'testDomain':test_domain,'rootPath':root,'rootDevice':st.st_dev,'rootInode':st.st_ino,'rootUid':st.st_uid,'rootMode':448,'markerPath':marker})
def root_verify(root):
 if root!=os.path.abspath(root):fail('root path not absolute canonical')
 supplied=root
 try:pre=os.lstat(supplied)
 except OSError:fail('root missing')
 if stat.S_ISLNK(pre.st_mode) or not stat.S_ISDIR(pre.st_mode):fail('root type')
 canonical=os.path.realpath(supplied);post=os.lstat(canonical)
 if supplied!=canonical or (pre.st_dev,pre.st_ino)!=(post.st_dev,post.st_ino):fail('root canonical identity')
 marker=os.path.join(canonical,'.jc-fl-test-root.json');o=load(marker,ROOT_FIELDS)
 if o['domain']!='JC_FL_TEST_ROOT_AUTHORITY' or o['version']!=1 or not DOMAIN.fullmatch(o['testDomain']):fail('root domain')
 if o['rootPath']!=canonical or o['markerPath']!=marker or o['rootDevice']!=post.st_dev or o['rootInode']!=post.st_ino or o['rootUid']!=post.st_uid or o['rootMode']!=448:fail('root identity')
 dir_identity(canonical,0o700);print(f'{canonical}|{post.st_dev}|{post.st_ino}|{marker}')
def phase_identity(run):
 run,st=dir_identity(run,0o700);rid=os.path.basename(run)
 if not UUID.fullmatch(rid):fail('run id')
 return run,rid,st
def phase_write(run):
 run,rid,st=phase_identity(run);record=os.path.join(run,'evidence','lifecycle-phase.json')
 evidence,evidence_st=dir_identity(os.path.dirname(record),0o700)
 if evidence!=os.path.join(run,'evidence') or os.path.lexists(record):fail('phase placement/exists')
 marker=os.path.join(evidence,'lifecycle-stage-scaffolded.json')
 if os.path.lexists(marker):fail('phase marker exists')
 base={'domain':'JC_FL_LIFECYCLE_PHASE_AUTHORITY','version':1,'runId':rid,'runPath':run,'runDevice':st.st_dev,'runInode':st.st_ino,'runUid':st.st_uid,'runMode':448,'stage':'scaffolded','redisLaunchAttempted':False,'dockerLaunchAttempted':False}
 marker_obj=dict(base,recordPath=marker);atomic(marker,marker_obj);atomic(record,dict(base,recordPath=record))
def phase_object(o,path,run,rid,st):
 if o['recordPath']!=path or o['domain']!='JC_FL_LIFECYCLE_PHASE_AUTHORITY' or o['version']!=1:fail('phase authority')
 if o['runPath']!=run or o['runId']!=rid or o['runDevice']!=st.st_dev or o['runInode']!=st.st_ino or o['runUid']!=st.st_uid or o['runMode']!=448:fail('phase run identity')
 if o['stage'] not in STAGES or type(o['redisLaunchAttempted']) is not bool or type(o['dockerLaunchAttempted']) is not bool:fail('phase state')
 if (o['redisLaunchAttempted'],o['dockerLaunchAttempted'])!=STAGE_STATE[o['stage']]:fail('phase stage/state mismatch')
def phase_verify(record):
 o=load(record,PHASE_FIELDS);run,rid,st=phase_identity(o['runPath']);canonical=os.path.join(run,'evidence','lifecycle-phase.json')
 evidence,evidence_st=dir_identity(os.path.dirname(canonical),0o700)
 if evidence!=os.path.join(run,'evidence') or os.path.abspath(record)!=canonical or os.path.realpath(record)!=canonical:fail('phase authority')
 phase_object(o,canonical,run,rid,st)
 present=[]
 for name in os.listdir(evidence):
  if name.startswith('lifecycle-stage-') and name.endswith('.json'):
   stage=name[len('lifecycle-stage-'):-len('.json')]
   if stage not in STAGES:fail('unknown phase marker')
   present.append(stage)
 ordered=sorted(present,key=STAGE_ORDER.index)
 if not ordered:fail('phase marker continuity')
 expected=list(STAGE_ORDER[:STAGE_ORDER.index(ordered[-1])+1])
 if ordered!=expected or o['stage'] not in expected:fail('phase marker continuity')
 authoritative=None
 for stage in expected:
  marker=os.path.join(evidence,'lifecycle-stage-'+stage+'.json');marker_obj=load(marker,PHASE_FIELDS);phase_object(marker_obj,marker,run,rid,st)
  if marker_obj['stage']!=stage:fail('phase marker stage')
  authoritative=marker_obj
 print(f"REDIS_ATTEMPTED={str(authoritative['redisLaunchAttempted']).lower()}\nDOCKER_ATTEMPTED={str(authoritative['dockerLaunchAttempted']).lower()}\nSTAGE={authoritative['stage']}")
 return authoritative
def phase_update(record,stage,redis_s,docker_s):
 old=phase_verify(record);redis=redis_s=='true';docker=docker_s=='true'
 if redis_s not in ('true','false') or docker_s not in ('true','false') or stage not in STAGES:fail('phase update')
 if NEXT_STAGE.get(old['stage'])!=stage or (redis,docker)!=STAGE_STATE[stage]:fail('phase transition')
 record=os.path.realpath(record);marker=os.path.join(os.path.dirname(record),'lifecycle-stage-'+stage+'.json')
 if os.path.lexists(marker):fail('phase marker exists')
 old.update(stage=stage,redisLaunchAttempted=redis,dockerLaunchAttempted=docker,recordPath=marker);atomic(marker,old);old['recordPath']=record;atomic(record,old);phase_verify(record)
def state_port(value):
 if not value.isdigit() or not 1024<=int(value)<=65535:fail('state port')
 return value
def state_read(path,expected_run,expected_dir):
 if path!=os.path.abspath(path) or expected_dir!=os.path.abspath(expected_dir) or path!=os.path.join(expected_dir,'state.env'):fail('state path')
 run,rid,run_st=phase_identity(expected_dir)
 if expected_run!=rid:fail('state expected run')
 try:st=os.lstat(path)
 except OSError:fail('state missing')
 if stat.S_ISLNK(st.st_mode) or not stat.S_ISREG(st.st_mode) or stat.S_IMODE(st.st_mode)!=0o600 or st.st_uid!=os.getuid() or st.st_nlink!=1 or os.path.realpath(path)!=path:fail('state type/mode')
 values={}
 try:lines=open(path,encoding='ascii',errors='strict').read().splitlines()
 except (OSError,UnicodeError) as exc:fail('state unreadable: '+str(exc))
 for line in lines:
  match=re.fullmatch(r"([A-Z][A-Z0-9_]*)='([^']*)'",line)
  if not match or match.group(1) in values:fail('state syntax/duplicate')
  values[match.group(1)]=match.group(2)
 ready=values.get('MYSQL_SCHEMA_READY')
 expected=set(STATE_BASE)|(set(STATE_READY) if ready=='1' else set())
 if ready not in ('0','1') or set(values)!=expected:fail('state fields')
 run_hex=expected_run.replace('-','')
 if values['RUN_ID']!=expected_run or values['RUN_HEX']!=run_hex or values['RUN_DIR']!=expected_dir:fail('state run identity')
 if values['MYSQL_NAME']!='jc-fl-e2e-mysql-'+run_hex or not HEX64.fullmatch(values['MYSQL_CONTAINER_ID']):fail('state container identity')
 if not values['MYSQL_CONTAINER_PID'].isdigit() or int(values['MYSQL_CONTAINER_PID'])<=1 or re.fullmatch(r'[0-9TZ:.+\-]+',values['MYSQL_CONTAINER_STARTED_AT']) is None:fail('state container process')
 if re.fullmatch(r'mysql@sha256:[0-9a-f]{64}',values['MYSQL_IMAGE_REF']) is None or re.fullmatch(r'sha256:[0-9a-f]{64}',values['MYSQL_IMAGE_ID']) is None:fail('state image')
 if (values['MYSQL_NETWORK_MODE'],values['MYSQL_NETWORK_NAME'],values['MYSQL_NETWORK_ID'])!=('host','network-not-used','network-not-used'):fail('state network')
 mysql_port=state_port(values['MYSQL_PORT'])
 if values['MYSQL_ENDPOINT']!='127.0.0.1:'+mysql_port or values['MYSQL_CONTAINER_PORT']!=mysql_port or values['MYSQL_PROTOCOL']!='tcp':fail('state endpoint')
 if not ANY_UUID.fullmatch(values['MYSQL_SERVER_UUID']) or re.fullmatch(r'[A-Za-z0-9._+\-]{1,128}',values['MYSQL_VERSION']) is None:fail('state server identity')
 if not HEX64.fullmatch(values['MYSQL_RESERVATION_DIGEST']) or not HEX64.fullmatch(values['MYSQL_TRANSPORT_DIGEST']):fail('state transport digest')
 if values['DB_NAME']!='jc_fl_e2e_'+run_hex:fail('state database')
 for key in ('REDIS_PORT','RPC_PORT','CORE_PORT','ADMIN_PORT','FRONTEND_PORT'):state_port(values[key])
 if ready=='1' and any(not HEX64.fullmatch(values[key]) for key in STATE_READY):fail('state readiness digest')
 output=sys.stdout.buffer
 for key in STATE_BASE+STATE_READY:
  if key in values:output.write(key.encode()+b'\0'+values[key].encode()+b'\0')
 output.write(b'__JC_FL_STATE_VERIFIED__\0yes\0')
def main():
 a=sys.argv[1:]
 if len(a)==3 and a[0]=='root-write':root_write(a[1],a[2])
 elif len(a)==2 and a[0]=='root-verify':root_verify(a[1])
 elif len(a)==2 and a[0]=='phase-write':phase_write(a[1])
 elif len(a)==2 and a[0]=='phase-verify':phase_verify(a[1])
 elif len(a)==5 and a[0]=='phase-update':phase_update(a[1],a[2],a[3],a[4])
 elif len(a)==4 and a[0]=='state-read':state_read(a[1],a[2],a[3])
 else:fail('usage')
if __name__=='__main__':main()
