#!/usr/bin/env python3
"""Strict immutable Redis process authority for the disposable LOCAL-ONLY harness."""
import hashlib,json,os,re,stat,sys,tempfile
UUID=re.compile(r'^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$')
ENDPOINT=re.compile(r'^127[.]0[.]0[.]1:([0-9]{1,5})$')
DOMAIN='JC_FL_E2E_REDIS_PROCESS_V2'
FIELDS={'domain','version','runId','runDir','recordPath','pid','startTime','exe','exeDevice','exeInode','expectedExe','cmdlineHex','configPath','configDevice','configInode','configMode','configDigest','endpoint','socketInode','projectionDigest'}
PROJECTION_FIELDS=('runId','runDir','recordPath','pid','startTime','exe','exeDevice','exeInode','expectedExe','cmdlineHex','configPath','configDevice','configInode','configMode','configDigest','endpoint','socketInode')
def fail(msg): print('redis-process-record: '+msg,file=sys.stderr);raise SystemExit(1)
def digest(path):
 h=hashlib.sha256()
 with open(path,'rb') as f:
  for b in iter(lambda:f.read(65536),b''):h.update(b)
 return h.hexdigest()
def atomic(path,obj):
 raw=(json.dumps(obj,sort_keys=True,separators=(',',':'))+'\n').encode();fd,tmp=tempfile.mkstemp(prefix='.redis-record.',dir=os.path.dirname(path));os.fchmod(fd,0o600)
 try:os.write(fd,raw);os.fsync(fd)
 finally:os.close(fd)
 os.replace(tmp,path);os.chmod(path,0o600)
def start(pid):
 try:return open(f'/proc/{pid}/stat',encoding='ascii').read().split()[21]
 except (OSError,IndexError):return ''
def fd_sockets(pid):
 result=set()
 try:names=os.listdir(f'/proc/{pid}/fd')
 except OSError:return result
 for name in names:
  try:value=os.readlink(f'/proc/{pid}/fd/{name}')
  except OSError:continue
  if value.startswith('socket:[') and value.endswith(']'):result.add(value[8:-1])
 return result
def endpoint_rows(endpoint):
 m=ENDPOINT.fullmatch(endpoint)
 if not m or not 1024<=int(m.group(1))<=65535:fail('invalid endpoint')
 wanted=f'0100007F:{int(m.group(1)):04X}';rows=[]
 for path in ('/proc/net/tcp','/proc/net/tcp6'):
  try:lines=open(path,encoding='ascii').read().splitlines()[1:]
  except OSError:fail('socket table unavailable')
  for line in lines:
   f=line.split()
   if len(f)>9 and f[1].upper()==wanted and f[3]=='0A':rows.append(f[9])
 return rows
def protected(path,mode,kind='dir'):
 supplied=os.path.abspath(path)
 try:st=os.lstat(supplied)
 except OSError:fail(f'missing protected object: {path}')
 wanted=stat.S_ISDIR(st.st_mode) if kind=='dir' else stat.S_ISREG(st.st_mode)
 if stat.S_ISLNK(st.st_mode) or not wanted or stat.S_IMODE(st.st_mode)!=mode or st.st_uid!=os.getuid() or kind=='file' and st.st_nlink!=1:fail(f'unsafe protected object: {path}')
 canonical=os.path.realpath(supplied);post=os.lstat(canonical)
 if supplied!=canonical or (st.st_dev,st.st_ino)!=(post.st_dev,post.st_ino):fail(f'protected object relocation: {path}')
 return canonical,st
def trusted_executable(path):
 if path!=os.path.abspath(path):fail('executable path not absolute canonical')
 canonical=os.path.realpath(path)
 try:st=os.lstat(canonical)
 except OSError:fail('expected executable missing')
 if path!=canonical or stat.S_ISLNK(st.st_mode) or not stat.S_ISREG(st.st_mode) or st.st_uid!=0 or st.st_mode&0o022 or not st.st_mode&0o111:fail('untrusted expected executable')
 return canonical,st
def projection_digest(obj):
 raw=json.dumps({key:obj.get(key) for key in PROJECTION_FIELDS},sort_keys=True,separators=(',',':')).encode()
 return hashlib.sha256(raw).hexdigest()
def authority(record_path,obj):
 if not isinstance(obj,dict) or set(obj)!=FIELDS:fail('record fields')
 if obj.get('domain')!=DOMAIN or obj.get('version')!=2 or not UUID.fullmatch(str(obj.get('runId',''))):fail('record domain')
 run,_=protected(obj['runDir'],0o700);record,_=protected(record_path,0o600,'file');expected=os.path.join(run,'redis','process.json')
 if record!=expected or obj.get('recordPath')!=record:fail('record relocation')
 if os.path.basename(run)!=obj['runId'] or obj.get('configPath')!=os.path.join(run,'redis','redis.conf'):fail('run placement')
 protected(os.path.dirname(run),0o700);protected(run,0o700);protected(os.path.join(run,'redis'),0o700)
 expected,expected_st=trusted_executable(obj.get('expectedExe',''));config,config_st=protected(obj.get('configPath',''),0o600,'file')
 if obj.get('exe')!=expected or obj.get('exeDevice')!=expected_st.st_dev or obj.get('exeInode')!=expected_st.st_ino:fail('executable authority')
 if obj.get('configDevice')!=config_st.st_dev or obj.get('configInode')!=config_st.st_ino or obj.get('configMode')!=0o600 or obj.get('configDigest')!=digest(config):fail('config authority')
 if type(obj.get('pid')) is not int or obj['pid']<=1 or not isinstance(obj.get('startTime'),str) or not obj['startTime'].isdigit():fail('pid/start authority')
 if not isinstance(obj.get('cmdlineHex'),str) or not obj['cmdlineHex'] or len(obj['cmdlineHex'])%2 or re.fullmatch(r'[0-9a-f]+',obj['cmdlineHex']) is None:fail('cmdline authority')
 ENDPOINT.fullmatch(str(obj.get('endpoint',''))) or fail('endpoint authority')
 if not isinstance(obj.get('socketInode'),str) or not obj['socketInode'].isdigit():fail('socket authority')
 if obj.get('projectionDigest')!=projection_digest(obj):fail('projection digest')
 return run
def strict_pairs(pairs):
 obj={}
 for key,value in pairs:
  if key in obj:fail('duplicate record field')
  obj[key]=value
 return obj
def load(path):
 try:
  protected(path,0o600,'file')
  with open(path,encoding='utf-8') as f:obj=json.load(f,object_pairs_hook=strict_pairs)
 except (OSError,json.JSONDecodeError) as exc:fail(f'invalid record: {exc}')
 authority(path,obj);return obj
def live(obj):
 pid=obj['pid']
 try:exst=os.stat(f'/proc/{pid}/exe');exe=os.path.realpath(f'/proc/{pid}/exe');cmd=open(f'/proc/{pid}/cmdline','rb').read().hex();cfg=os.stat(obj['configPath'])
 except OSError:fail('process/config missing')
 expected,_=trusted_executable(obj['expectedExe'])
 try:argv=[part.decode('utf-8','strict') for part in bytes.fromhex(cmd).split(b'\0') if part]
 except (ValueError,UnicodeError):fail('process cmdline invalid')
 if start(pid)!=obj['startTime'] or exe!=expected or exe!=obj['exe'] or exst.st_dev!=obj['exeDevice'] or exst.st_ino!=obj['exeInode'] or cmd!=obj['cmdlineHex']:fail('process identity changed')
 if obj['configPath'] not in argv and obj['endpoint'] not in ' '.join(argv):fail('process config/endpoint argument changed')
 if os.path.realpath(obj['configPath'])!=obj['configPath'] or cfg.st_dev!=obj['configDevice'] or cfg.st_ino!=obj['configInode'] or stat.S_IMODE(cfg.st_mode)!=obj['configMode'] or obj['configMode']!=0o600 or digest(obj['configPath'])!=obj['configDigest']:fail('config identity changed')
 rows=endpoint_rows(obj['endpoint'])
 if len(rows)!=1 or rows[0]!=obj['socketInode'] or rows[0] not in fd_sockets(pid):fail('socket ownership changed')
def write(pid_s,run,config,endpoint,expected_exe,path):
 pid=int(pid_s);run,_=protected(run,0o700);config,_=protected(config,0o600,'file');expected_exe,_=trusted_executable(expected_exe);path=os.path.abspath(path);run_id=os.path.basename(run)
 if path!=os.path.join(run,'redis','process.json') or not UUID.fullmatch(run_id) or os.path.lexists(path):fail('write placement')
 protected(os.path.dirname(run),0o700);protected(run,0o700);protected(os.path.join(run,'redis'),0o700)
 try:exst=os.stat(f'/proc/{pid}/exe');exe=os.path.realpath(f'/proc/{pid}/exe');cfg=os.stat(config);cmd=open(f'/proc/{pid}/cmdline','rb').read().hex()
 except OSError:fail('write identity missing')
 try:argv=[part.decode('utf-8','strict') for part in bytes.fromhex(cmd).split(b'\0') if part]
 except (ValueError,UnicodeError):fail('write cmdline invalid')
 if config not in argv and endpoint not in ' '.join(argv):fail('write config/endpoint argument mismatch')
 rows=endpoint_rows(endpoint)
 if len(rows)!=1 or rows[0] not in fd_sockets(pid):fail('write socket ownership')
 if exe!=expected_exe:fail('write executable mismatch')
 obj={'domain':DOMAIN,'version':2,'runId':run_id,'runDir':run,'recordPath':path,'pid':pid,'startTime':start(pid),'exe':exe,'exeDevice':exst.st_dev,'exeInode':exst.st_ino,'expectedExe':expected_exe,'cmdlineHex':cmd,'configPath':config,'configDevice':cfg.st_dev,'configInode':cfg.st_ino,'configMode':stat.S_IMODE(cfg.st_mode),'configDigest':digest(config),'endpoint':endpoint,'socketInode':rows[0]}
 obj['projectionDigest']=projection_digest(obj)
 atomic(path,obj);load(path);live(obj)
def verify(path):
 obj=load(path);live(obj)
 for key in ('pid','startTime','endpoint'):print(f'{key.upper()}={obj[key]}')
def absence(path):
 obj=load(path)
 live_start=start(obj['pid'])
 if live_start and live_start==obj['startTime']:fail('recorded pid still exists')
 if endpoint_rows(obj['endpoint']):fail('recorded endpoint still listening')
 print('ABSENT=1')
def main():
 a=sys.argv[1:]
 if len(a)==7 and a[0]=='write':write(*a[1:])
 elif len(a)==2 and a[0]=='verify':verify(a[1])
 elif len(a)==2 and a[0]=='absence':absence(a[1])
 else:fail('usage')
if __name__=='__main__':main()
