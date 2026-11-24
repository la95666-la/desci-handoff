#!/usr/bin/env python3
"""Build/verify the final LOCAL-ONLY schema publication cross-binding."""
import hashlib,json,os,re,stat,sys
H=re.compile(r'^(?:sha256:)?[0-9a-f]{64}$')
def fail(x): print('schema-readiness: FAIL: '+x,file=sys.stderr);raise SystemExit(1)
def mode600(p): return os.path.isfile(p) and stat.S_IMODE(os.stat(p).st_mode)==0o600
def sha(p):
 h=hashlib.sha256()
 with open(p,'rb') as f:
  for b in iter(lambda:f.read(65536),b''):h.update(b)
 return h.hexdigest()
def expected(root,args):
 run,transport,image,baseline,migration,schema,grant,owner=args
 infra=os.path.join(root,'evidence','infrastructure.json'); evidence=os.path.join(root,'evidence','database-schema.json')
 if not mode600(infra) or not mode600(evidence):fail('evidence missing/mode')
 profiles={}
 for role in ('runtime','admin','job','ingest'):
  p=os.path.join(root,'credentials',f'mysql-{role}.cnf')
  if not mode600(p):fail('profile missing/mode '+role)
  profiles[role]=sha(p)
 values=(transport,image,baseline,migration,schema,grant,owner)
 if not all(H.fullmatch(x) for x in values):fail('digest shape')
 o={'authority':'TEST_ONLY_NOT_PRODUCTION_ADMITTED','runId':run,'transportDigest':transport,'mysqlImageId':image,'infrastructureDigest':sha(infra),'schemaEvidenceDigest':sha(evidence),'profileDigests':profiles,'baselineDigest':baseline,'migrationManifestDigest':migration,'schemaDigest':schema,'grantPolicyDigest':grant,'ownerLabelDigest':owner,'status':'SCHEMA_READY_NOT_RUNTIME_ADMITTED'}
 canonical=json.dumps(o,sort_keys=True,separators=(',',':'))
 o['readinessDigest']=hashlib.sha256(('JC_FL_SCHEMA_READINESS_V1|'+canonical).encode()).hexdigest()
 return o
def main():
 if len(sys.argv)!=11 or sys.argv[1] not in ('build','verify'):fail('usage')
 mode,root=sys.argv[1:3]; args=sys.argv[3:]
 o=expected(root,args); target=os.path.join(root,'evidence','schema-readiness.json')
 if mode=='build':
  tmp=target+'.tmp.'+str(os.getpid());fd=os.open(tmp,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
  try:os.write(fd,(json.dumps(o,sort_keys=True,separators=(',',':'))+'\n').encode());os.fsync(fd)
  finally:os.close(fd)
  os.replace(tmp,target);os.chmod(target,0o600)
 else:
  if not mode600(target):fail('marker missing/mode')
  with open(target,encoding='utf-8') as f:seen=json.load(f)
  if seen!=o:fail('cross-binding mismatch')
if __name__=='__main__':main()
