'use strict';
const fs=require('fs'),path=require('path'),crypto=require('crypto');
const ROOT_REAL=fs.realpathSync(path.resolve(__dirname,'../..'));
const ENVELOPE_DOMAIN='joincare.release.detached-signature-envelope.v1';
const ROOT_DOMAIN='joincare.fair-launch.database-reviewer-root.v1';
const PRODUCTION_TRUST_ROOT=Object.freeze({status:'UNADMITTED_PLACEHOLDER',domain:ROOT_DOMAIN,keySetVersion:0,threshold:2,keys:[]});
const PRODUCTION_CHECKPOINT=Object.freeze({status:'UNADMITTED_PLACEHOLDER',generation:0,digest:'UNADMITTED_PLACEHOLDER'});
const PRODUCTION_LATEST_CHECKPOINT=Object.freeze({status:'UNADMITTED_PLACEHOLDER',generation:0,digest:'UNADMITTED_PLACEHOLDER'});
const sha256=value=>crypto.createHash('sha256').update(value).digest('hex');
function validateString(value){for(let i=0;i<value.length;i++){const c=value.charCodeAt(i);if(c>=0xd800&&c<=0xdbff){const n=value.charCodeAt(++i);if(!(n>=0xdc00&&n<=0xdfff))throw Error('unpaired surrogate');}else if(c>=0xdc00&&c<=0xdfff)throw Error('unpaired surrogate');}}
// RFC 8785 / JCS uses ECMAScript JSON number serialization and UTF-16 code-unit
// property ordering. JSON.stringify already supplies the required finite-number,
// escaping and -0 behavior; this wrapper rejects I-JSON-invalid input first.
function canonical(value){
 if(value===null)return'null';
 if(typeof value==='string'){validateString(value);return JSON.stringify(value);}
 if(typeof value==='boolean')return value?'true':'false';
 if(typeof value==='number'){if(!Number.isFinite(value))throw Error('non-finite JSON number');return JSON.stringify(value);}
 if(Array.isArray(value))return`[${value.map((item)=>{if(item===undefined||typeof item==='function'||typeof item==='symbol')throw Error('unsupported authority value');return canonical(item);}).join(',')}]`;
 if(value&&typeof value==='object'){
  const keys=Object.keys(value);for(const key of keys){validateString(key);if(value[key]===undefined||typeof value[key]==='function'||typeof value[key]==='symbol')throw Error('unsupported authority value');}
  keys.sort();return`{${keys.map(k=>`${canonical(k)}:${canonical(value[k])}`).join(',')}}`;
 }
 throw Error('unsupported authority value');
}
function rejectTestAuthorityEnvironment(){for(const key of Object.keys(process.env)){if(/^JC_FAIR_LAUNCH_TEST_/i.test(key))throw Error(`${key} is forbidden in production authority entrypoints`);}}
function hardenedRead(file,label,options={}){
 if(!file)throw Error(`${label} path required`);
 const platform=options.platform||process.platform;
 if(platform==='win32'&&!options.testOnly)throw Error('windows_authority_native_validator_unavailable');
 const lexical=path.resolve(file),fixedUnix=path.resolve(options.unixRoot||'/etc/joincare/fair-launch');
 if(platform!=='win32'&&!options.testOnly&&lexical!==fixedUnix&&!lexical.startsWith(fixedUnix+path.sep))throw Error(`${label} must use the fixed Unix machine authority root`);
 let cursor=lexical;
 while(true){const st=fs.lstatSync(cursor);if(st.isSymbolicLink())throw Error(`${label} symlink/reparse path rejected`);if(platform!=='win32'){const stickyRootDir=st.isDirectory()&&st.uid===0&&(st.mode&0o1000)!==0;if((st.mode&0o022)!==0&&!stickyRootDir)throw Error(`${label} path is group/world writable`);if(st.uid!==process.getuid()&&st.uid!==0)throw Error(`${label} path owner rejected`);}const parent=path.dirname(cursor);if(parent===cursor)break;cursor=parent;}
 const real=fs.realpathSync(lexical);if(real===ROOT_REAL||real.startsWith(ROOT_REAL+path.sep))throw Error(`${label} must be repository-external`);
 const flags=fs.constants.O_RDONLY|(fs.constants.O_NOFOLLOW||0),fd=fs.openSync(real,flags);
 try{const before=fs.fstatSync(fd);if(!before.isFile())throw Error(`${label} must be a file`);if(platform!=='win32'&&(before.mode&0o077)!==0)throw Error(`${label} must be mode 0600 or stricter`);const bytes=fs.readFileSync(fd),after=fs.fstatSync(fd),final=fs.lstatSync(real);if(before.dev!==after.dev||before.ino!==after.ino||before.size!==after.size||final.dev!==after.dev||final.ino!==after.ino||final.size!==after.size)throw Error(`${label} changed during read`);const raw=bytes.toString('utf8');if(Buffer.from(raw,'utf8').length!==bytes.length)throw Error(`${label} must be UTF-8`);const canonicalBytes=raw.endsWith('\n')?raw.slice(0,-1):raw;return{path:real,raw:canonicalBytes,digest:sha256(canonicalBytes)};}finally{fs.closeSync(fd);}
}
function readCanonical(file,label,options={}){const read=hardenedRead(file,label,options),value=JSON.parse(read.raw);if(canonical(value)!==read.raw)throw Error(`${label} must be RFC 8785 canonical JSON`);return{...read,value};}
function publicKey(key){const raw=Buffer.from(key.publicKeyBase64||'','base64');if(raw.length!==32)throw Error('invalid Ed25519 public key');return crypto.createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),raw]),format:'der',type:'spki'});}
function validateRoot(root){if(root.status!=='REVIEWER_ADMITTED'||root.domain!==ROOT_DOMAIN||root.threshold!==2||root.keySetVersion<1||!Array.isArray(root.keys)||root.keys.length!==3)throw Error('production reviewer trust root is not admitted');const ids=new Set();for(const key of root.keys){if(key.algorithm!=='Ed25519'||!/^[a-z0-9-]{1,32}$/.test(key.keyId||'')||ids.has(key.keyId))throw Error('reviewer trust root invalid');ids.add(key.keyId);publicKey(key);}return root;}
function verifyBundleValue(bundle,expected,options={}){
 const root=validateRoot(options.root||PRODUCTION_TRUST_ROOT);if(!bundle||Object.keys(bundle).sort().join(',')!=='envelope,payload')throw Error('signed authority bundle shape invalid');
 const payloadText=canonical(bundle.payload),payloadDigest=sha256(payloadText),envelope=bundle.envelope;
 if(envelope.domain!==ENVELOPE_DOMAIN||envelope.payloadDomain!==bundle.payload.domain||envelope.payloadSha256!==payloadDigest||envelope.keySetVersion!==root.keySetVersion)throw Error('signature envelope binding mismatch');
 for(const field of ['domain','subject','environment','deploymentId','generation'])if(expected[field]!==undefined&&bundle.payload[field]!==expected[field])throw Error('signed authority subject/continuity mismatch');
 if(Object.prototype.hasOwnProperty.call(expected,'previousAdmissionDigest')&&bundle.payload.previousAdmissionDigest!==expected.previousAdmissionDigest)throw Error('signed authority subject/continuity mismatch');
 if(!Number.isSafeInteger(bundle.payload.generation)||bundle.payload.generation<1||(bundle.payload.generation===1&&bundle.payload.previousAdmissionDigest!==null)||(bundle.payload.generation>1&&!/^[0-9a-f]{64}$/.test(bundle.payload.previousAdmissionDigest||'')))throw Error('signed authority generation/predecessor invalid');
 const seen=new Set();let valid=0;for(const sig of envelope.signatures||[]){if(seen.has(sig.keyId))throw Error('duplicate reviewer signature');seen.add(sig.keyId);const key=root.keys.find(k=>k.keyId===sig.keyId);if(!key||sig.algorithm!=='Ed25519')throw Error('unknown reviewer key');const raw=Buffer.from(sig.signatureBase64||'','base64');if(raw.length!==64)throw Error('invalid Ed25519 signature');if(crypto.verify(null,Buffer.from(payloadText),publicKey(key),raw))valid++;}
 if(valid<root.threshold)throw Error('2-of-3 Ed25519 threshold not met');return{payload:bundle.payload,payloadDigest,bundleDigest:sha256(canonical(bundle))};
}
function verifyBundleFile(file,expected,options={}){if(!options.testOnly)rejectTestAuthorityEnvironment();const read=readCanonical(file,'signed authority bundle',options);return{...verifyBundleValue(read.value,expected,options),path:read.path,fileDigest:read.digest};}
function historyLoader(rootDir,expected,options={}){if(!rootDir)throw Error('signed authority history root unavailable');const fixed=path.resolve(rootDir);return(digest,generation)=>{if(!/^[0-9a-f]{64}$/.test(digest)||!Number.isSafeInteger(generation)||generation<1)throw Error('authority history request invalid');const file=path.join(fixed,`${generation}-${digest}.json`);if(path.dirname(file)!==fixed)throw Error('authority history path escaped fixed root');const item=verifyBundleFile(file,{domain:expected.domain,subject:expected.subject,environment:expected.environment,deploymentId:expected.deploymentId,generation},{...options});if(item.bundleDigest!==digest)throw Error('authority predecessor digest mismatch');return item;};}
function verifyPinnedChain(current,loadPrevious,expected,options={}){
 const checkpoint=options.checkpoint||PRODUCTION_CHECKPOINT,latest=options.latestCheckpoint||PRODUCTION_LATEST_CHECKPOINT;
 for(const [name,item] of [['checkpoint',checkpoint],['latest activation checkpoint',latest]])if(item.status!=='REVIEWER_ADMITTED'||item.generation<1||!/^[0-9a-f]{64}$/.test(item.digest||''))throw Error(`production anti-rollback ${name} is not admitted`);
 if(current.payload.generation!==latest.generation||current.bundleDigest!==latest.digest)throw Error('authority is not the fixed latest activated generation');
 let item=current;while(item.payload.generation>checkpoint.generation){const predecessor=item.payload.previousAdmissionDigest;if(!/^[0-9a-f]{64}$/.test(predecessor||''))throw Error('authority predecessor missing');const previous=loadPrevious(predecessor,item.payload.generation-1);if(previous.bundleDigest!==predecessor||previous.payload.generation!==item.payload.generation-1||previous.payload.domain!==expected.domain||previous.payload.subject!==expected.subject||previous.payload.environment!==expected.environment||previous.payload.deploymentId!==expected.deploymentId)throw Error('authority predecessor identity mismatch');item=previous;}
 if(item.payload.generation!==checkpoint.generation||item.bundleDigest!==checkpoint.digest)throw Error('authority chain does not reach fixed checkpoint');return current;
}
function verifyBundleChainFile(file,expected,options={}){const current=verifyBundleFile(file,expected,options),load=options.loadPrevious||historyLoader(options.historyRoot,expected,options);return verifyPinnedChain(current,load,expected,options);}
module.exports={ENVELOPE_DOMAIN,PRODUCTION_TRUST_ROOT,PRODUCTION_CHECKPOINT,PRODUCTION_LATEST_CHECKPOINT,canonical,sha256,hardenedRead,readCanonical,verifyBundleValue,verifyBundleFile,verifyBundleCanonicalFile:verifyBundleFile,historyLoader,verifyPinnedChain,verifyBundleChainFile,rejectTestAuthorityEnvironment};
