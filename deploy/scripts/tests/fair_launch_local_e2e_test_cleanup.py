#!/usr/bin/env python3
"""Immutable mode-0600 ownership records for LOCAL-ONLY test cleanup."""
import hashlib,json,os,re,stat,sys,tempfile
CONTAINER_FIELDS={'domain','version','runId','component','runDir','runDevice','runInode','runUid','runMode','evidenceDir','evidenceDevice','evidenceInode','evidenceUid','evidenceMode','recordPath','containerId','projectionDigest','projection'}
REMOVAL_FIELDS={'domain','version','runId','runDir','runDevice','runInode','runUid','runMode','evidenceDir','evidenceDevice','evidenceInode','evidenceUid','evidenceMode','recordPath','sourceRecordPath','sourceRecordDigest','containerId','projectionDigest','stage'}
ABSENCE_FIELDS={'domain','version','runId','runDir','runDevice','runInode','runUid','runMode','evidenceDir','evidenceDevice','evidenceInode','evidenceUid','evidenceMode','recordPath','sourceRecordPath','sourceRecordDigest','removalRecordPath','removalRecordDigest','queryEvidencePath','queryEvidenceDigest','containerId','projectionDigest','status'}
QUERY_FIELDS={'domain','version','runId','runDir','runDevice','runInode','runUid','runMode','evidenceDir','evidenceDevice','evidenceInode','evidenceUid','evidenceMode','recordPath','sourceRecordPath','sourceRecordDigest','rawEvidencePath','rawEvidenceDigest','containerId','exitCode','status'}
PROCESS_FIELDS={'domain','version','pid','startTime','exe','exeDevice','exeInode','markerPath','markerDevice','markerInode','markerDigest','endpoint','socketInode','cmdlineHex','runDir','recordPath','projectionDigest'}
PROCESS_PROJECTION_FIELDS=('pid','startTime','exe','exeDevice','exeInode','markerPath','markerDevice','markerInode','markerDigest','endpoint','socketInode','cmdlineHex','runDir','recordPath')
REMOVAL_STAGES={'AUTHORIZED_BEFORE_RM','RM_SUCCEEDED_QUERY_PENDING'}
def strict_pairs(items):
 out={}
 for k,v in items:
  if k in out:fail('duplicate record field')
  out[k]=v
 return out
HEX64=re.compile(r'^[0-9a-f]{64}$');UUID=re.compile(r'^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$');ENDPOINT=re.compile(r'^127[.]0[.]0[.]1:([0-9]{1,5})$')
def fail(m):print('fair-launch test cleanup: EXPECTED_REFUSAL: '+m,file=sys.stderr);raise SystemExit(1)
def atomic(path,obj):
 if path!=os.path.abspath(path) or os.path.dirname(path)!=os.path.realpath(os.path.dirname(path)):fail('atomic path authority')
 parent=os.lstat(os.path.dirname(path))
 if stat.S_ISLNK(parent.st_mode) or not stat.S_ISDIR(parent.st_mode) or stat.S_IMODE(parent.st_mode)!=0o700 or parent.st_uid!=os.getuid():fail('atomic parent authority')
 raw=(json.dumps(obj,sort_keys=True,separators=(',',':'))+'\n').encode();fd,tmp=tempfile.mkstemp(prefix='.cleanup.',dir=os.path.dirname(path));os.fchmod(fd,0o600)
 try:os.write(fd,raw);os.fsync(fd)
 finally:os.close(fd)
 os.replace(tmp,path);os.chmod(path,0o600)
def load(path):
 if path!=os.path.abspath(path) or path!=os.path.realpath(path):fail('record canonical path')
 st=os.lstat(path)
 if stat.S_ISLNK(st.st_mode) or not stat.S_ISREG(st.st_mode) or stat.S_IMODE(st.st_mode)!=0o600 or st.st_uid!=os.getuid() or st.st_nlink!=1:fail('record mode/type')
 return json.load(open(path,encoding='utf-8'),object_pairs_hook=strict_pairs)
def digest_file(path):
 h=hashlib.sha256()
 with open(path,'rb') as f:
  for block in iter(lambda:f.read(65536),b''):h.update(block)
 return h.hexdigest()
def projection(obj):
 config=dict(obj.get('Config') or {});config.pop('Hostname',None)
 return {'Id':obj.get('Id'),'Image':obj.get('Image'),'Name':obj.get('Name'),'Config':config,'HostConfig':obj.get('HostConfig') or {},'Mounts':sorted(obj.get('Mounts') or [],key=lambda x:x.get('Destination','')),'NetworkPorts':(obj.get('NetworkSettings') or {}).get('Ports'),'Created':obj.get('Created'),'Path':obj.get('Path'),'Args':obj.get('Args')}
def load_inspect(path):
 st=os.lstat(path)
 if stat.S_ISLNK(st.st_mode) or not stat.S_ISREG(st.st_mode) or stat.S_IMODE(st.st_mode)!=0o600 or st.st_uid!=os.getuid() or st.st_nlink!=1:fail('inspect evidence type/mode')
 with open(path,encoding='utf-8') as f:obj=json.load(f,object_pairs_hook=strict_pairs)
 if not isinstance(obj,list) or len(obj)!=1 or not isinstance(obj[0],dict):fail('inspect evidence shape')
 return obj[0]
def protected_dir(path,mode=0o700):
 if path!=os.path.abspath(path):fail('protected directory path')
 supplied=path
 try:st=os.lstat(supplied)
 except OSError:fail('protected directory missing')
 if stat.S_ISLNK(st.st_mode) or not stat.S_ISDIR(st.st_mode) or stat.S_IMODE(st.st_mode)!=mode or st.st_uid!=os.getuid():fail('protected directory identity')
 canonical=os.path.realpath(supplied);post=os.lstat(canonical)
 if supplied!=canonical or (st.st_dev,st.st_ino)!=(post.st_dev,post.st_ino):fail('protected directory canonical identity')
 return canonical,post
def run_identity(run_dir):
 run_dir,run_st=protected_dir(run_dir);run_id=os.path.basename(run_dir)
 if not UUID.fullmatch(run_id):fail('run id')
 evidence,evidence_st=protected_dir(os.path.join(run_dir,'evidence'))
 return run_dir,run_id,run_st,evidence,evidence_st
def validate_record_path(out,run_dir):
 run_dir,run_id,run_st,evidence,evidence_st=run_identity(run_dir)
 if out!=os.path.abspath(out):fail('record path not absolute canonical')
 supplied=out
 if os.path.lexists(supplied) and os.path.realpath(supplied)!=supplied:fail('record symlink/relocation')
 if os.path.dirname(supplied)!=evidence:fail('record placement')
 return supplied,run_dir,run_id,run_st,evidence,evidence_st
def chain_paths(source_path):
 source_path=os.path.abspath(source_path);name=os.path.basename(source_path);evidence=os.path.dirname(source_path)
 if name=='test-container-cleanup.json':absence=os.path.join(evidence,'docker-absence.json')
 elif name.endswith('.source.json'):absence=source_path[:-len('.source.json')]+'.absence.json'
 else:fail('container source naming')
 return source_path+'.removal.json',source_path+'.post-rm-query',absence
def identity_fields(run_dir,run_id,run_st,evidence,evidence_st):
 return {'runId':run_id,'runDir':run_dir,'runDevice':run_st.st_dev,'runInode':run_st.st_ino,'runUid':run_st.st_uid,'runMode':448,'evidenceDir':evidence,'evidenceDevice':evidence_st.st_dev,'evidenceInode':evidence_st.st_ino,'evidenceUid':evidence_st.st_uid,'evidenceMode':448}
def verify_identity(rec):
 run_dir,run_id,run_st,evidence,evidence_st=run_identity(rec.get('runDir',''))
 expected=identity_fields(run_dir,run_id,run_st,evidence,evidence_st)
 if any(rec.get(k)!=v for k,v in expected.items()):fail('record run identity')
 return run_dir,run_id,run_st,evidence,evidence_st
def container_record(inspect_path,run_id,component,run_dir,out):
 if not UUID.fullmatch(run_id):fail('run id')
 out,run_dir,actual_run_id,run_st,evidence,evidence_st=validate_record_path(out,run_dir)
 chain_paths(out)
 if run_id!=actual_run_id or os.path.lexists(out):fail('container record target')
 obj=load_inspect(inspect_path);p=projection(obj);cid=p['Id'];labels=(p['Config'] or {}).get('Labels') or {};runhex=run_id.replace('-','')
 if not isinstance(cid,str) or not HEX64.fullmatch(cid):fail('full container id')
 if labels.get('joincare.fair-launch.local-e2e-run')!=run_id or labels.get('joincare.fair-launch.local-e2e-component')!=component:fail('labels')
 if p.get('Name')!='/jc-fl-e2e-mysql-'+runhex:fail('name')
 sources={os.path.realpath(m.get('Source','')) for m in p.get('Mounts') or [] if m.get('Type')=='bind'}
 if not any(s.startswith(run_dir+os.sep) for s in sources):fail('test-owned mount')
 raw=json.dumps(p,sort_keys=True,separators=(',',':')).encode();record={'domain':'JC_FL_TEST_CONTAINER_CLEANUP_V3','version':3,'component':component,'recordPath':out,'containerId':cid,'projectionDigest':hashlib.sha256(raw).hexdigest(),'projection':p};record.update(identity_fields(run_dir,run_id,run_st,evidence,evidence_st));atomic(out,record)
def container_static_verify(record_path,emit=True):
 if record_path!=os.path.abspath(record_path):fail('container record path')
 rec=load(record_path)
 if not isinstance(rec,dict) or set(rec)!=CONTAINER_FIELDS:fail('container record fields')
 supplied=os.path.abspath(record_path);record_path=os.path.realpath(record_path);run_dir,run_id,run_st,evidence,evidence_st=verify_identity(rec);p=rec.get('projection');component=rec.get('component')
 chain_paths(record_path)
 if supplied!=record_path or rec.get('domain')!='JC_FL_TEST_CONTAINER_CLEANUP_V3' or rec.get('version')!=3 or rec.get('recordPath')!=record_path or os.path.dirname(record_path)!=evidence:fail('container record authority')
 if not isinstance(p,dict):fail('container projection')
 raw=json.dumps(p,sort_keys=True,separators=(',',':')).encode();cid=rec.get('containerId');labels=((p.get('Config') or {}).get('Labels') or {})
 if not isinstance(cid,str) or not HEX64.fullmatch(cid) or cid!=p.get('Id') or rec.get('projectionDigest')!=hashlib.sha256(raw).hexdigest():fail('container static identity')
 if labels.get('joincare.fair-launch.local-e2e-run')!=run_id or labels.get('joincare.fair-launch.local-e2e-component')!=component or p.get('Name')!='/jc-fl-e2e-mysql-'+run_id.replace('-',''):fail('container labels/name')
 sources={os.path.realpath(m.get('Source','')) for m in p.get('Mounts') or [] if m.get('Type')=='bind'}
 if not any(s.startswith(run_dir+os.sep) for s in sources):fail('container run mount')
 if emit:print(cid)
 return rec

def removal_verify(record_path,source_path,required_stage=None,emit=True):
 if record_path!=os.path.abspath(record_path) or source_path!=os.path.abspath(source_path):fail('removal path authority')
 rec=load(record_path)
 if not isinstance(rec,dict) or set(rec)!=REMOVAL_FIELDS:fail('removal record fields')
 run_dir,run_id,run_st,evidence,evidence_st=verify_identity(rec);record_path=os.path.realpath(record_path);source_path=os.path.realpath(source_path);source=container_static_verify(source_path,False)
 expected_removal,_,_=chain_paths(source_path)
 if rec.get('domain')!='JC_FL_TEST_CONTAINER_REMOVAL_V1' or rec.get('version')!=1 or rec.get('recordPath')!=record_path or record_path!=expected_removal or os.path.dirname(record_path)!=evidence:fail('removal record authority')
 if rec.get('sourceRecordPath')!=source_path or rec.get('sourceRecordDigest')!=digest_file(source_path) or rec.get('containerId')!=source['containerId'] or rec.get('projectionDigest')!=source['projectionDigest']:fail('removal source identity')
 if rec.get('stage') not in REMOVAL_STAGES or required_stage is not None and rec.get('stage')!=required_stage:fail('removal stage')
 if emit:print(rec['containerId'])
 return rec
def removal_write(source_path,record_path):
 source=container_static_verify(source_path,False);record_path,run_dir,run_id,run_st,evidence,evidence_st=validate_record_path(record_path,source['runDir'])
 expected_removal,_,_=chain_paths(source_path)
 if record_path!=expected_removal or os.path.lexists(record_path):fail('removal record exists')
 rec={'domain':'JC_FL_TEST_CONTAINER_REMOVAL_V1','version':1,'recordPath':record_path,'sourceRecordPath':os.path.realpath(source_path),'sourceRecordDigest':digest_file(source_path),'containerId':source['containerId'],'projectionDigest':source['projectionDigest'],'stage':'AUTHORIZED_BEFORE_RM'};rec.update(identity_fields(run_dir,run_id,run_st,evidence,evidence_st));atomic(record_path,rec);removal_verify(record_path,source_path,'AUTHORIZED_BEFORE_RM',False)
def removal_update(record_path,source_path,old_stage,new_stage):
 if (old_stage,new_stage)!=('AUTHORIZED_BEFORE_RM','RM_SUCCEEDED_QUERY_PENDING'):fail('removal transition')
 rec=removal_verify(record_path,source_path,old_stage,False);rec['stage']=new_stage;atomic(os.path.realpath(record_path),rec);removal_verify(record_path,source_path,new_stage,False)
def removal_stage(record_path,source_path):
 rec=removal_verify(record_path,source_path,None,False);print(rec['stage'])
def accepted_not_found(path,container_id,evidence):
 supplied=os.path.abspath(path);canonical=os.path.realpath(path);st=os.lstat(path)
 if supplied!=canonical or os.path.dirname(canonical)!=evidence or stat.S_ISLNK(st.st_mode) or not stat.S_ISREG(st.st_mode) or stat.S_IMODE(st.st_mode)!=0o600 or st.st_uid!=os.getuid() or st.st_nlink!=1 or st.st_size>4096:fail('query evidence identity')
 lines=[line for line in open(path,encoding='utf-8',errors='strict').read().splitlines() if line!='']
 accepted=len(lines)==1 and re.fullmatch(r'(?:(?:Error response from daemon: |Error: )?No such (?:object|container): |error: no such (?:object|container): )'+re.escape(container_id),lines[0]) is not None
 if not accepted:fail('query evidence not exact not-found')
 return canonical
def query_write(source_path,raw_path,exit_code_s,out):
 source=container_static_verify(source_path,False);out,run_dir,run_id,run_st,evidence,evidence_st=validate_record_path(out,source['runDir'])
 _,expected_query,_=chain_paths(source_path)
 if out!=expected_query or os.path.abspath(raw_path)!=expected_query+'.raw' or os.path.lexists(out) or exit_code_s!='1':fail('query exit status')
 raw_path=accepted_not_found(raw_path,source['containerId'],evidence)
 rec={'domain':'JC_FL_TEST_DOCKER_QUERY_V1','version':1,'recordPath':out,'sourceRecordPath':os.path.realpath(source_path),'sourceRecordDigest':digest_file(source_path),'rawEvidencePath':raw_path,'rawEvidenceDigest':digest_file(raw_path),'containerId':source['containerId'],'exitCode':1,'status':'ABSENT_EXACT_ID_QUERY'};rec.update(identity_fields(run_dir,run_id,run_st,evidence,evidence_st));atomic(out,rec);query_verify(out,source_path,False)
def query_verify(record_path,source_path=None,emit=True):
 if record_path!=os.path.abspath(record_path) or source_path is not None and source_path!=os.path.abspath(source_path):fail('query path authority')
 rec=load(record_path)
 if not isinstance(rec,dict) or set(rec)!=QUERY_FIELDS:fail('query record fields')
 run_dir,run_id,run_st,evidence,evidence_st=verify_identity(rec);record_path=os.path.realpath(record_path);stored_source=rec.get('sourceRecordPath','')
 if stored_source!=os.path.abspath(stored_source) or stored_source!=os.path.realpath(stored_source):fail('query source canonical path')
 bound_source=stored_source
 if source_path is not None and os.path.realpath(source_path)!=bound_source:fail('query source path')
 source=container_static_verify(bound_source,False);raw=accepted_not_found(rec.get('rawEvidencePath',''),source['containerId'],evidence)
 _,expected_query,_=chain_paths(bound_source)
 if rec.get('domain')!='JC_FL_TEST_DOCKER_QUERY_V1' or rec.get('version')!=1 or rec.get('recordPath')!=record_path or record_path!=expected_query or raw!=expected_query+'.raw' or os.path.dirname(record_path)!=evidence:fail('query record authority')
 if rec.get('sourceRecordPath')!=bound_source or rec.get('sourceRecordDigest')!=digest_file(bound_source) or rec.get('rawEvidencePath')!=raw or rec.get('rawEvidenceDigest')!=digest_file(raw):fail('query evidence identity')
 if rec.get('containerId')!=source['containerId'] or rec.get('exitCode')!=1 or rec.get('status')!='ABSENT_EXACT_ID_QUERY':fail('query result identity')
 if emit:print(rec['containerId'])
 return rec
def absence_write(source_path,removal_path,query_path,out):
 removal=removal_verify(removal_path,source_path,'RM_SUCCEEDED_QUERY_PENDING',False);source=container_static_verify(source_path,False);out,run_dir,run_id,run_st,evidence,evidence_st=validate_record_path(out,source['runDir'])
 _,_,expected_absence=chain_paths(source_path)
 if out!=expected_absence or os.path.lexists(out):fail('absence record exists')
 query=query_verify(query_path,source_path,False);query_path=os.path.realpath(query_path)
 rec={'domain':'JC_FL_TEST_DOCKER_ABSENCE_V2','version':2,'recordPath':out,'sourceRecordPath':os.path.realpath(source_path),'sourceRecordDigest':digest_file(source_path),'removalRecordPath':os.path.realpath(removal_path),'removalRecordDigest':digest_file(removal_path),'queryEvidencePath':query_path,'queryEvidenceDigest':digest_file(query_path),'containerId':source['containerId'],'projectionDigest':source['projectionDigest'],'status':'ABSENT_EXACT_ID'};rec.update(identity_fields(run_dir,run_id,run_st,evidence,evidence_st));atomic(out,rec);absence_verify(out,False)
def absence_promote(source_path,removal_path,query_path,out):
 rec=removal_verify(removal_path,source_path,'AUTHORIZED_BEFORE_RM',False);source=container_static_verify(source_path,False);run_dir,run_id,run_st,evidence,evidence_st=verify_identity(rec);query_verify(query_path,source_path,False);rec['stage']='RM_SUCCEEDED_QUERY_PENDING';atomic(os.path.realpath(removal_path),rec);absence_write(source_path,removal_path,query_path,out)
def absence_verify(record_path,emit=True):
 if record_path!=os.path.abspath(record_path):fail('absence path authority')
 rec=load(record_path)
 if not isinstance(rec,dict) or set(rec)!=ABSENCE_FIELDS:fail('absence record fields')
 run_dir,run_id,run_st,evidence,evidence_st=verify_identity(rec);record_path=os.path.realpath(record_path);source_path=rec.get('sourceRecordPath','');removal_path=rec.get('removalRecordPath','');query_path=rec.get('queryEvidencePath','')
 if any(p!=os.path.abspath(p) or p!=os.path.realpath(p) for p in (source_path,removal_path,query_path)):fail('absence canonical paths')
 source=container_static_verify(source_path,False);removal=removal_verify(removal_path,source_path,'RM_SUCCEEDED_QUERY_PENDING',False);query_verify(query_path,source_path,False)
 _,_,expected_absence=chain_paths(source_path)
 if rec.get('domain')!='JC_FL_TEST_DOCKER_ABSENCE_V2' or rec.get('version')!=2 or rec.get('recordPath')!=record_path or record_path!=expected_absence or os.path.dirname(record_path)!=evidence:fail('absence record authority')
 if rec.get('sourceRecordPath')!=source_path or rec.get('sourceRecordDigest')!=digest_file(source_path) or rec.get('removalRecordPath')!=removal_path or rec.get('removalRecordDigest')!=digest_file(removal_path) or rec.get('queryEvidencePath')!=query_path or rec.get('queryEvidenceDigest')!=digest_file(query_path):fail('absence evidence identity')
 if rec.get('containerId')!=source['containerId'] or rec.get('projectionDigest')!=source['projectionDigest'] or rec.get('status')!='ABSENT_EXACT_ID':fail('absence container identity')
 if emit:print(rec['containerId'])
 return rec
def absence_covers(record_path,source_path):
 rec=absence_verify(record_path,False);source=container_static_verify(source_path,False)
 if rec['runDir']!=source['runDir'] or rec['evidenceDir']!=source['evidenceDir'] or rec['containerId']!=source['containerId'] or rec['projectionDigest']!=source['projectionDigest']:fail('absence does not cover source')
 print(rec['containerId'])

def container_verify(record_path,inspect_path):
 rec=container_static_verify(record_path,False);obj=load_inspect(inspect_path);p=projection(obj);raw=json.dumps(p,sort_keys=True,separators=(',',':')).encode()
 labels=(p.get('Config') or {}).get('Labels') or {}
 if labels.get('joincare.fair-launch.local-e2e-run')!=rec['runId'] or labels.get('joincare.fair-launch.local-e2e-component')!=rec.get('component') or p.get('Name')!='/jc-fl-e2e-mysql-'+rec['runId'].replace('-',''):fail('container labels/name changed')
 if rec.get('containerId')!=p.get('Id') or rec.get('projection')!=p or rec.get('projectionDigest')!=hashlib.sha256(raw).hexdigest():fail('container identity changed')
 print(rec['containerId'])
def container_evidence_verify(record_path,inspect_path):
 if inspect_path!=os.path.abspath(inspect_path) or inspect_path!=record_path+'.inspect.new':fail('container publication evidence path')
 container_verify(record_path,inspect_path)
def proc_start(pid):
 try:return open(f'/proc/{pid}/stat',encoding='ascii').read().split()[21]
 except (OSError,IndexError):return ''
def socket_inodes(pid):
 out=set()
 try:
  for name in os.listdir(f'/proc/{pid}/fd'):
   try:target=os.readlink(f'/proc/{pid}/fd/{name}')
   except OSError:continue
   if target.startswith('socket:['):out.add(target[8:-1])
 except OSError:pass
 return out
def endpoint_matches(endpoint):
 m=ENDPOINT.fullmatch(endpoint)
 if not m or not 1024<=int(m.group(1))<=65535:fail('endpoint shape')
 wanted=f'0100007F:{int(m.group(1)):04X}';matches=[]
 for path in ('/proc/net/tcp','/proc/net/tcp6'):
  try:lines=open(path,encoding='ascii').read().splitlines()[1:]
  except OSError:fail('endpoint socket table unavailable')
  for line in lines:
   fields=line.split()
   if len(fields)>9 and fields[1].upper()==wanted and fields[3]=='0A':matches.append(fields[9])
 return matches
def endpoint_inode(endpoint):
 matches=endpoint_matches(endpoint)
 if len(matches)!=1:fail('endpoint listener cardinality')
 return matches[0]
def process_scope(scope,out):
 if scope!=os.path.abspath(scope) or out!=os.path.abspath(out):fail('process scope path')
 scope,scope_st=protected_dir(scope);evidence,evidence_st=protected_dir(os.path.join(scope,'evidence'))
 if os.path.dirname(out)!=evidence or os.path.lexists(out) and os.path.realpath(out)!=out:fail('process record placement')
 return scope,out
def process_record(pid_s,marker,endpoint,run_dir,out):
 pid=int(pid_s);run_dir,out=process_scope(run_dir,out)
 if os.path.lexists(out):fail('process record exists')
 if marker!=os.path.abspath(marker) or os.path.realpath(marker)!=marker:fail('process marker path')
 st=os.lstat(marker);exe=os.path.realpath(f'/proc/{pid}/exe');inode=os.stat(f'/proc/{pid}/exe');start=proc_start(pid);cmd=open(f'/proc/{pid}/cmdline','rb').read().hex()
 if not start or stat.S_ISLNK(st.st_mode) or not stat.S_ISREG(st.st_mode) or stat.S_IMODE(st.st_mode)!=0o600 or st.st_uid!=os.getuid() or not marker.startswith(run_dir+os.sep):fail('process marker')
 socket_inode=None
 if endpoint!='none':
  socket_inode=endpoint_inode(endpoint)
  if socket_inode not in socket_inodes(pid):fail('process does not own endpoint')
 obj={'domain':'JC_FL_TEST_PROCESS_CLEANUP_V3','version':3,'pid':pid,'startTime':start,'exe':exe,'exeDevice':inode.st_dev,'exeInode':inode.st_ino,'markerPath':marker,'markerDevice':st.st_dev,'markerInode':st.st_ino,'markerDigest':digest_file(marker),'endpoint':endpoint,'socketInode':socket_inode,'cmdlineHex':cmd,'runDir':run_dir,'recordPath':out}
 raw=json.dumps({key:obj.get(key) for key in PROCESS_PROJECTION_FIELDS},sort_keys=True,separators=(',',':')).encode();obj['projectionDigest']=hashlib.sha256(raw).hexdigest();atomic(out,obj)
def record_authority(path,o):
 if path!=os.path.abspath(path):fail('process record path')
 run_dir,path=process_scope(o.get('runDir',''),path);marker=o.get('markerPath')
 if not isinstance(o,dict) or set(o)!=PROCESS_FIELDS or o.get('domain')!='JC_FL_TEST_PROCESS_CLEANUP_V3' or o.get('version')!=3 or o.get('recordPath')!=path or not isinstance(marker,str) or marker!=os.path.abspath(marker) or os.path.realpath(marker)!=marker or not marker.startswith(run_dir+os.sep):fail('process record authority')
 try:st=os.lstat(marker)
 except OSError:fail('process marker missing')
 if stat.S_ISLNK(st.st_mode) or not stat.S_ISREG(st.st_mode) or st.st_uid!=os.getuid() or st.st_dev!=o.get('markerDevice') or st.st_ino!=o.get('markerInode') or stat.S_IMODE(st.st_mode)!=0o600 or digest_file(marker)!=o.get('markerDigest'):fail('process marker changed')
 raw=json.dumps({key:o.get(key) for key in PROCESS_PROJECTION_FIELDS},sort_keys=True,separators=(',',':')).encode()
 if o.get('projectionDigest')!=hashlib.sha256(raw).hexdigest():fail('process projection changed')
 return path,run_dir,marker
def process_verify(path):
 o=load(path);record_authority(path,o);pid=o.get('pid')
 try:ex=os.stat(f'/proc/{pid}/exe');exe=os.path.realpath(f'/proc/{pid}/exe');cmd=open(f'/proc/{pid}/cmdline','rb').read().hex()
 except OSError:fail('process missing')
 if proc_start(pid)!=o.get('startTime') or exe!=o.get('exe') or ex.st_dev!=o.get('exeDevice') or ex.st_ino!=o.get('exeInode') or cmd!=o.get('cmdlineHex'):fail('process identity changed')
 endpoint=o.get('endpoint')
 if endpoint!='none':
  inode=endpoint_inode(endpoint)
  if inode!=o.get('socketInode') or inode not in socket_inodes(pid):fail('process endpoint changed')
 print(f"{pid}|{o['startTime']}|{endpoint}")
def process_scope_verify(path):
 o=load(path);record_authority(path,o);print(o['runDir'])
def process_absence(path):
 o=load(path);record_authority(path,o);pid=o.get('pid');endpoint=o.get('endpoint')
 if proc_start(pid):fail('pid still present')
 if endpoint!='none' and endpoint_matches(endpoint):fail('endpoint still listening')
 print('absent')
def main():
 a=sys.argv[1:]
 if len(a)==6 and a[0]=='container-record':container_record(*a[1:])
 elif len(a)==2 and a[0]=='container-static-verify':container_static_verify(a[1])
 elif len(a)==3 and a[0]=='container-verify':container_verify(*a[1:])
 elif len(a)==3 and a[0]=='container-evidence-verify':container_evidence_verify(*a[1:])
 elif len(a)==3 and a[0]=='removal-write':removal_write(a[1],a[2])
 elif len(a)==4 and a[0]=='removal-verify':removal_verify(a[1],a[2],a[3])
 elif len(a)==3 and a[0]=='removal-stage':removal_stage(a[1],a[2])
 elif len(a)==5 and a[0]=='removal-update':removal_update(a[1],a[2],a[3],a[4])
 elif len(a)==5 and a[0]=='query-write':query_write(a[1],a[2],a[3],a[4])
 elif len(a)==3 and a[0]=='query-verify':query_verify(a[1],a[2])
 elif len(a)==5 and a[0]=='absence-write':absence_write(a[1],a[2],a[3],a[4])
 elif len(a)==5 and a[0]=='absence-promote':absence_promote(a[1],a[2],a[3],a[4])
 elif len(a)==2 and a[0]=='absence-verify':absence_verify(a[1])
 elif len(a)==3 and a[0]=='absence-covers':absence_covers(a[1],a[2])
 elif len(a)==6 and a[0]=='process-record':process_record(*a[1:])
 elif len(a)==2 and a[0]=='process-verify':process_verify(a[1])
 elif len(a)==2 and a[0]=='process-scope':process_scope_verify(a[1])
 elif len(a)==2 and a[0]=='process-absence':process_absence(a[1])
 else:fail('usage')
if __name__=='__main__':main()
