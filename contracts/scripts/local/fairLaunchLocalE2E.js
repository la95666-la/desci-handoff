'use strict';

// Structural validation only. Nothing in this module establishes authority or
// admits an artifact. Key generation and signing exist only in test fixtures.
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { ethers } = require('ethers');

const REPOSITORY_ROOT = path.resolve(__dirname, '../..');
const DEFAULT_RUN_ROOT = path.join(os.tmpdir(), 'joincare-fair-launch-local-e2e');
const RPC_URL = 'http://127.0.0.1:8545';
const UNADMITTED_STATUS = 'UNADMITTED_OFFLINE_EVIDENCE_ONLY';
const MAX_ARTIFACT_LIFETIME_MS = 24 * 60 * 60 * 1000;
const MAX_CLOCK_SKEW_MS = 5 * 60 * 1000;
const SIGNED_ENVELOPE_SCHEMA = 'fair-launch-unadmitted-structural-envelope-v1';
const POLICY_SCHEMA = 'fair-launch-unadmitted-policy-v1';
const CHAIN_SCHEMA = 'fair-launch-unadmitted-chain-v1';
const FRONTEND_SCHEMA = 'fair-launch-unadmitted-frontend-v1';
const DB_ACK_SCHEMA = 'fair-launch-unadmitted-db-ack-v1';
const ROOT_SCHEMA = 'fair-launch-unadmitted-public-root-v1';
const DOMAINS = Object.freeze({
  policy: 'joincare.fair-launch.unadmitted.policy.v1',
  chain: 'joincare.fair-launch.unadmitted.chain.v1',
  frontend: 'joincare.fair-launch.unadmitted.frontend.v1',
  database: 'joincare.fair-launch.unadmitted.database.v1',
});
const SCHEMAS = Object.freeze({ policy: POLICY_SCHEMA, chain: CHAIN_SCHEMA, frontend: FRONTEND_SCHEMA, database: DB_ACK_SCHEMA });
const ROLE_NAMES = Object.freeze(['deployer','governanceOwner','operator','pauseGuardian','unpauseTimelock','treasury','failedProjectOwner','successfulProjectOwner','cancelledProjectOwner','sharedProjectOwnerA','sharedProjectOwnerB','failedUser','successfulUserA','successfulUserB','successfulUserC','cancelledUser','permissionlessCaller','unauthorizedCaller','safeOwnerA','safeOwnerB']);
const CONTRACT_IDENTITIES = Object.freeze({
  proxy:'FairLaunchERC1967Proxy', dispatcher:'JoinCareFairLaunchV1', coreFacet:'JoinCareFairLaunchV1CoreFacet',
  releaseFacet:'JoinCareFairLaunchV1ReleaseFacet', fundsFacet:'JoinCareFairLaunchV1FundsFacet',
  adminFacet:'JoinCareFairLaunchV1AdminFacet', viewsFacet:'JoinCareFairLaunchV1ViewsFacet',
  logicLibrary:'JoinCareFairLaunchV1Logic', runtimeLibrary:'JoinCareFairLaunchV1Runtime',
});
const PROJECT_KEYS = Object.freeze(['failed','successful','cancelled','sharedA','sharedB']);
const PROJECT_OWNER_ROLES = Object.freeze({failed:'failedProjectOwner',successful:'successfulProjectOwner',cancelled:'cancelledProjectOwner',sharedA:'sharedProjectOwnerA',sharedB:'sharedProjectOwnerB'});
const TOKEN_KEYS = Object.freeze(['failed','successful','cancelled','shared']);
const UINT16_MAX = 65_535;
const UINT32_MAX = 4_294_967_295;
const MAX_FEE_BPS = 1_000;
const UINT64_MAX = (1n << 64n) - 1n;
const FROZEN_EVENT_MANIFEST_SHA256 = '6d8981815a144bce6ce7938eb1d9ba52f7935888284c5ff222c3217ba21a536a';
const capabilityMetadata = new WeakMap();

function clonePlainData(value, label='value', seen=new Set()) {
  if (value === null || typeof value === 'string' || typeof value === 'boolean') return value;
  if (typeof value === 'number') {
    if (!Number.isFinite(value)) throw new Error(`${label} contains a non-finite number`);
    return value;
  }
  if (typeof value !== 'object') throw new Error(`${label} contains unsupported data`);
  if (seen.has(value)) throw new Error(`${label} contains a cycle`);
  seen.add(value);
  try {
    if (Array.isArray(value)) {
      if (Object.getPrototypeOf(value) !== Array.prototype) throw new Error(`${label} array prototype invalid`);
      const names = Object.getOwnPropertyNames(value);
      if (Object.getOwnPropertySymbols(value).length || names.some((name) => name !== 'length' && !/^(0|[1-9]\d*)$/.test(name))) throw new Error(`${label} array properties invalid`);
      if (names.length !== value.length + 1) throw new Error(`${label} array is sparse`);
      const out = [];
      for (let index = 0; index < value.length; index += 1) {
        const descriptor = Object.getOwnPropertyDescriptor(value, String(index));
        if (!descriptor || !Object.hasOwn(descriptor, 'value') || !descriptor.enumerable) throw new Error(`${label} array accessor invalid`);
        out.push(clonePlainData(descriptor.value, `${label}[${index}]`, seen));
      }
      return out;
    }
    if (Object.getPrototypeOf(value) !== Object.prototype) throw new Error(`${label} object prototype invalid`);
    if (Object.getOwnPropertySymbols(value).length) throw new Error(`${label} symbol properties invalid`);
    const descriptors = Object.getOwnPropertyDescriptors(value);
    const out = {};
    for (const key of Object.keys(descriptors)) {
      const descriptor = descriptors[key];
      if (!Object.hasOwn(descriptor, 'value') || !descriptor.enumerable) throw new Error(`${label} accessor invalid`);
      if (['__proto__','prototype','constructor'].includes(key)) throw new Error(`${label} unsafe field`);
      out[key] = clonePlainData(descriptor.value, `${label}.${key}`, seen);
    }
    return out;
  } finally {
    seen.delete(value);
  }
}
function deepFreeze(value) {
  if (value && typeof value === 'object' && !Object.isFrozen(value)) {
    for (const child of Object.values(value)) deepFreeze(child);
    Object.freeze(value);
  }
  return value;
}
function immutableCopy(value, label) { return deepFreeze(clonePlainData(value, label)); }
function canonicalFromPlain(value) {
  if (Array.isArray(value)) return value.map(canonicalFromPlain);
  if (value && typeof value === 'object') return Object.fromEntries(Object.keys(value).sort().map((key) => [key, canonicalFromPlain(value[key])]));
  return value;
}
function canonical(value) { return canonicalFromPlain(clonePlainData(value, 'JCS input')); }
function canonicalJson(value) { return JSON.stringify(canonical(value)); }
function sha256(value) { return crypto.createHash('sha256').update(value).digest('hex'); }
function parseExactJson(text, label='JSON') {
  let i=0;
  const ws=()=>{while(/\s/.test(text[i]||''))i++;};
  const str=()=>{const s=i;if(text[i++]!=='"')throw Error(`${label} invalid JSON`);let escaped=false;while(i<text.length){const c=text[i++];if(escaped){escaped=false;continue;}if(c==='\\'){escaped=true;continue;}if(c==='"')return JSON.parse(text.slice(s,i));}throw Error(`${label} invalid JSON`);};
  const val=()=>{ws();const c=text[i];if(c==='"')return str();if(c==='{')return obj();if(c==='[')return arr();const m=text.slice(i).match(/^(true|false|null|-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?)/);if(!m)throw Error(`${label} invalid JSON`);i+=m[0].length;return JSON.parse(m[0]);};
  const obj=()=>{i++;const out={},seen=new Set();ws();if(text[i]==='}'){i++;return out;}while(true){ws();const k=str();if(seen.has(k))throw Error(`${label} duplicate field: ${k}`);if(['__proto__','prototype','constructor'].includes(k))throw Error(`${label} unsafe field: ${k}`);seen.add(k);ws();if(text[i++]!==':')throw Error(`${label} invalid JSON`);Object.defineProperty(out,k,{value:val(),enumerable:true,writable:true,configurable:true});ws();if(text[i]==='}'){i++;return out;}if(text[i++]!==',')throw Error(`${label} invalid JSON`);}};
  const arr=()=>{i++;const out=[];ws();if(text[i]===']'){i++;return out;}while(true){out.push(val());ws();if(text[i]===']'){i++;return out;}if(text[i++]!==',')throw Error(`${label} invalid JSON`);}};
  const result=val();ws();if(i!==text.length)throw Error(`${label} trailing data`);return result;
}
function readJson(file) { return parseExactJson(fs.readFileSync(file,'utf8'),path.basename(file)); }
function exact(value, keys, label) { if(!value||typeof value!=='object'||Array.isArray(value))throw Error(`${label} must be an object`);const actual=Object.keys(value);if(actual.length!==keys.length||actual.some(k=>!keys.includes(k)))throw Error(`${label} schema invalid`); }
function scalarString(v,label,min=1,max=256){if(typeof v!=='string'||v.length<min||v.length>max)throw Error(`${label} invalid`);return v;}
function enumValue(v,values,label){if(!values.includes(v))throw Error(`${label} invalid`);return v;}
function address(v,label){if(!/^0x[0-9a-f]{40}$/.test(v||'')||/^0x0{40}$/.test(v))throw Error(`${label} invalid`);return v;}
function hash(v,label){if(!/^0x[0-9a-f]{64}$/.test(v||'')||/^0x0{64}$/.test(v))throw Error(`${label} invalid`);return v;}
function digest(v,label){if(!/^[0-9a-f]{64}$/.test(v||'')||/^0{64}$/.test(v))throw Error(`${label} invalid`);return v;}
function uint(v,label,max=(1n<<256n)-1n,min=0n){const text=typeof v==='string'?v:typeof v==='number'&&Number.isSafeInteger(v)?String(v):'';if(!/^(0|[1-9]\d*)$/.test(text))throw Error(`${label} invalid`);const value=BigInt(text);if(value>max||value<min)throw Error(`${label} invalid`);return v;}
function integer(v,label,min=0,max=Number.MAX_SAFE_INTEGER){if(!Number.isSafeInteger(v)||v<min||v>max)throw Error(`${label} invalid`);return v;}
function unique(values,label){if(new Set(values).size!==values.length)throw Error(`${label} duplicates`);}
function denseArray(value,length,label){if(!Array.isArray(value)||(length!==undefined&&value.length!==length)||Object.keys(value).length!==value.length)throw Error(`${label} invalid`);return value;}
function loadFrozenEventDefinitions(){
  const file=path.join(REPOSITORY_ROOT,'scripts/validation/fair-launch-events.generated.json'),bytes=fs.readFileSync(file);
  if(sha256(bytes)!==FROZEN_EVENT_MANIFEST_SHA256)throw Error('frozen event manifest file digest invalid');
  const manifest=parseExactJson(bytes.toString('utf8'),'fair-launch-events.generated.json');
  exact(manifest,['schemaVersion','status','sourceClassification','source','input','eventCount','events','sha256'],'event manifest');
  if(manifest.schemaVersion!=='fair-launch-event-signature-index-manifest-v1'||manifest.status!==UNADMITTED_STATUS||manifest.sourceClassification!=='GENERATED_FROM_REVIEWER_FROZEN_ABI'||manifest.source!=='scripts/audit/golden/abi.golden.json'||manifest.eventCount!==32)throw Error('frozen event manifest identity invalid');
  exact(manifest.input,['path','sha256','selection','frozenEventCount'],'event manifest input');
  if(manifest.input.path!==manifest.source||manifest.input.selection!=='type=event AND name startsWith FairLaunch'||manifest.input.frozenEventCount!==32||manifest.input.sha256!==sha256(fs.readFileSync(path.join(REPOSITORY_ROOT,manifest.source))))throw Error('frozen event manifest input invalid');
  const unsigned={...manifest};delete unsigned.sha256;if(manifest.sha256!==sha256(canonicalJson(unsigned)))throw Error('frozen event manifest payload digest invalid');
  denseArray(manifest.events,32,'event manifest rows');const definitions=new Map(),names=[];
  for(const row of manifest.events){
    exact(row,['name','signature','topic0','indexedMask','inputs'],'event definition');scalarString(row.name,'event name');scalarString(row.signature,'event signature');hash(row.topic0,'event topic0');denseArray(row.inputs,undefined,'event inputs');denseArray(row.indexedMask,row.inputs.length,'event indexed mask');
    const inputs=row.inputs.map((input,index)=>{exact(input,['ordinal','name','type','indexed'],'event input');if(input.ordinal!==index||typeof input.indexed!=='boolean'||row.indexedMask[index]!==input.indexed)throw Error('event input projection invalid');scalarString(input.name,'event input name');scalarString(input.type,'event input type');return{name:input.name,type:input.type,indexed:input.indexed};});
    const abi={type:'event',name:row.name,anonymous:false,inputs},iface=new ethers.Interface([abi]),fragment=iface.getEvent(row.name);
    if(fragment.format('sighash')!==row.signature||fragment.topicHash.toLowerCase()!==row.topic0)throw Error('event ABI identity invalid');
    if(definitions.has(row.name))throw Error('event definitions duplicates');definitions.set(row.name,Object.freeze({name:row.name,signature:row.signature,topic0:row.topic0,inputs:deepFreeze(row.inputs),iface,fragment}));names.push(row.name);
  }
  if(canonicalJson(names)!==canonicalJson([...names].sort((a,b)=>a.localeCompare(b))))throw Error('event manifest ordering invalid');
  return definitions;
}
const FROZEN_EVENT_DEFINITIONS=loadFrozenEventDefinitions();
function runId(v){if(!/^[a-z0-9][a-z0-9-]{7,63}$/.test(v||''))throw Error('run id invalid');return v;}
function isoMs(v,label){const n=Date.parse(v);if(typeof v!=='string'||!Number.isFinite(n)||new Date(n).toISOString()!==v)throw Error(`${label} invalid`);return n;}
function interval(value, nowMs, label, containers=[]) {
  if(!Number.isFinite(nowMs))throw Error(`${label} trusted current time required`);
  const created=isoMs(value.createdAt,`${label} createdAt`),expires=isoMs(value.expiresAt,`${label} expiresAt`);
  if(expires<=created||expires-created>MAX_ARTIFACT_LIFETIME_MS)throw Error(`${label} lifetime invalid`);
  if(created>nowMs+MAX_CLOCK_SKEW_MS)throw Error(`${label} created in the future`);
  if(nowMs>=expires)throw Error(`${label} expired`);
  for(const c of containers){const start=isoMs(c.createdAt,`${label} container createdAt`),end=isoMs(c.expiresAt,`${label} container expiresAt`);if(created<start||expires>end)throw Error(`${label} interval not contained`);}
  return{createdAt:value.createdAt,expiresAt:value.expiresAt};
}
function header(v,type){
  exact(v,['schemaVersion','status','signatureDomain','runId','createdAt','expiresAt',...({policy:['generation','chainTrust','databaseTrust','allowedOrigins','allowedRpcUrls','artifactSchemas'],chain:['policyPayloadSha256','watermark','network','roles','contracts','assets','safes','projects','source','deployment','fixtureTransactions','fixtureEventIdentities'],frontend:['policyPayloadSha256','chainPayloadSha256','watermark','frontendOrigin','network','roles','project','contracts','assets','safes','deployment'],database:['policyPayloadSha256','snapshotId','captureDigest','chainPayloadSha256','anchor','captureTip','databaseFingerprintSha256','checkpointSha256','state']}[type])],`${type} payload`);
  if(v.schemaVersion!==SCHEMAS[type]||v.status!==UNADMITTED_STATUS||v.signatureDomain!==DOMAINS[type])throw Error(`${type} payload header invalid`);
  runId(v.runId);
}
function derivedKeyId(publicKeyJwk){
  exact(publicKeyJwk,['kty','crv','x'],'public JWK');
  if(publicKeyJwk.kty!=='OKP'||publicKeyJwk.crv!=='Ed25519'||!/^[A-Za-z0-9_-]{43}$/.test(publicKeyJwk.x||''))throw Error('public descriptor invalid');
  const key=crypto.createPublicKey({key:publicKeyJwk,format:'jwk'});
  return `unadmitted-${sha256(key.export({type:'spki',format:'der'})).slice(0,24)}`;
}
function publicDescriptor(v){exact(v,['keyId','algorithm','publicKeyJwk'],'public descriptor');if(v.algorithm!=='ed25519'||v.keyId!==derivedKeyId(v.publicKeyJwk))throw Error('public descriptor invalid');return v;}
function publicRoot(v, expected){
  exact(v,['schemaVersion','status','signatureDomain','runId','generation','threshold','createdAt','expiresAt','keys'],'public root');
  if(v.schemaVersion!==ROOT_SCHEMA||v.status!==UNADMITTED_STATUS||v.signatureDomain!==DOMAINS[expected.rootType]||v.runId!==expected.runId||v.generation!==expected.generation||v.threshold!==2)throw Error('public root invalid');
  denseArray(v.keys,3,'public root keys').forEach(publicDescriptor);unique(v.keys.map(k=>derivedKeyId(k.publicKeyJwk)),'root public keys');
  const rootInterval=interval(v,expected.nowMs,'public root');
  const rootDigest=sha256(canonicalJson(v));if(rootDigest!==expected.trustRootSha256)throw Error('expected trust root mismatch');
  return{value:v,interval:rootInterval,digest:rootDigest,generation:v.generation,keyIds:v.keys.map(k=>derivedKeyId(k.publicKeyJwk))};
}
function network(v){exact(v,['name','chainId','rpcUrl','genesisHash'],'network');if(v.name!=='hardhat-local-persistent'||v.chainId!=='31337'||v.rpcUrl!==RPC_URL)throw Error('network identity invalid');hash(v.genesisHash,'genesis hash');}
function roles(v){exact(v,ROLE_NAMES,'roles');ROLE_NAMES.forEach(k=>address(v[k],`role ${k}`));unique(ROLE_NAMES.map(k=>v[k]),'role addresses');}
function identity(v,key){exact(v,['contractName','address','runtimeCodeHash'],'contract identity');if(v.contractName!==CONTRACT_IDENTITIES[key])throw Error(`contract identity ${key} invalid`);address(v.address,`${key} address`);hash(v.runtimeCodeHash,`${key} runtime code hash`);}
function contracts(v){exact(v,Object.keys(CONTRACT_IDENTITIES),'contracts');for(const key of Object.keys(CONTRACT_IDENTITIES))identity(v[key],key);unique(Object.values(v).map(x=>x.address),'contract addresses');}
function token(v,label,key){exact(v,['key','address','symbol','decimals','runtimeCodeHash','policyDigest','policyVersion'],'token');if(v.key!==key)throw Error(`${label} key invalid`);address(v.address,`${label} address`);scalarString(v.symbol,`${label} symbol`,1,16);integer(v.decimals,`${label} decimals`,0,77);hash(v.runtimeCodeHash,`${label} runtime hash`);hash(v.policyDigest,`${label} policy digest`);integer(v.policyVersion,`${label} policy version`,1,UINT32_MAX);}
function assets(v){
  exact(v,['paymentToken','projectTokens','releasePolicy'],'assets');token(v.paymentToken,'payment token','payment');
  if(v.paymentToken.symbol!=='tUSDT'||v.paymentToken.decimals!==6)throw Error('payment token metadata invalid');
  denseArray(v.projectTokens,4,'project token set');v.projectTokens.forEach((x,i)=>{token(x,'project token',TOKEN_KEYS[i]);if(x.decimals!==18)throw Error('project token decimals invalid');});
  exact(v.releasePolicy,['digest','version'],'release policy');hash(v.releasePolicy.digest,'release policy digest');integer(v.releasePolicy.version,'release policy version',1,UINT32_MAX);
  unique([v.paymentToken,...v.projectTokens].map(x=>x.address),'asset addresses');
}
function safe(v,label){exact(v,['address','runtimeCodeHash','attestationDigest','attestationVersion'],'safe');address(v.address,`${label} address`);hash(v.runtimeCodeHash,`${label} runtime hash`);hash(v.attestationDigest,`${label} attestation digest`);integer(v.attestationVersion,`${label} attestation version`,1,UINT32_MAX);}
function safes(v){exact(v,['owners','threshold','proceeds','platformFee','platformFeeBps'],'safes');if(v.threshold!==2)throw Error('safe threshold invalid');denseArray(v.owners,3,'safe owners').forEach((x,i)=>address(x,`safe owner ${i}`));unique(v.owners,'safe owners');safe(v.proceeds,'proceeds safe');safe(v.platformFee,'platform fee safe');integer(v.platformFeeBps,'platform fee bps',0,MAX_FEE_BPS);if(v.proceeds.address===v.platformFee.address)throw Error('safe addresses must differ');}
const CREATION_WITNESS_FIELDS=['projectKey','projectId','transactionHash','txLogOrdinal','blockNumber','blockHash','blockTimestamp','transactionIndex','eventName'];
const PROJECT_FIELDS=['key','projectId','projectToken','paymentToken','projectOwner','projectProceedsReceiver','projectProceedsReceiverAttestationDigest','projectProceedsReceiverAttestationVersion','expectedPlatformFeeReceiver','expectedPlatformFeeReceiverAttestationDigest','expectedPlatformFeeReceiverAttestationVersion','expectedPlatformFeeBps','saleTokenAmountRaw','softCapRaw','minContributionRaw','maxContributionRaw','startTime','endTime','expectedProjectTokenPolicyDigest','expectedProjectTokenPolicyVersion','expectedPaymentTokenPolicyDigest','expectedPaymentTokenPolicyVersion','expectedReleasePolicyDigest','expectedReleasePolicyVersion','vestingTimes','vestingBps','initialStatus','creationWitness'];
function creationWitness(v){exact(v,CREATION_WITNESS_FIELDS,'project creation witness');enumValue(v.projectKey,PROJECT_KEYS,'creation project key');uint(v.projectId,'creation project id',(1n<<256n)-1n,1n);hash(v.transactionHash,'creation tx hash');integer(v.txLogOrdinal,'creation log ordinal');uint(v.blockNumber,'creation block number');hash(v.blockHash,'creation block hash');uint(v.blockTimestamp,'creation block timestamp',UINT64_MAX);integer(v.transactionIndex,'creation transaction index');if(v.eventName!=='FairLaunchProjectCreated')throw Error('creation event name invalid');}
function project(v,expectedKey){
  exact(v,PROJECT_FIELDS,'project');if(v.key!==expectedKey)throw Error('project key invalid');
  uint(v.projectId,'project id',(1n<<256n)-1n,1n);address(v.projectToken,'project token');address(v.paymentToken,'payment token');address(v.projectOwner,'project owner');if(v.projectToken===v.paymentToken)throw Error('project token identities invalid');
  address(v.projectProceedsReceiver,'project proceeds receiver');hash(v.projectProceedsReceiverAttestationDigest,'project proceeds attestation digest');integer(v.projectProceedsReceiverAttestationVersion,'project proceeds attestation version',1,UINT32_MAX);
  address(v.expectedPlatformFeeReceiver,'platform fee receiver');hash(v.expectedPlatformFeeReceiverAttestationDigest,'platform fee attestation digest');integer(v.expectedPlatformFeeReceiverAttestationVersion,'platform fee attestation version',1,UINT32_MAX);integer(v.expectedPlatformFeeBps,'platform fee bps',0,UINT16_MAX);if(v.expectedPlatformFeeBps>MAX_FEE_BPS)throw Error('platform fee bps invalid');
  for(const k of ['saleTokenAmountRaw','softCapRaw','minContributionRaw'])uint(v[k],k,(1n<<256n)-1n,1n);uint(v.maxContributionRaw,'maxContributionRaw');if(BigInt(v.maxContributionRaw)!==0n&&BigInt(v.maxContributionRaw)<BigInt(v.minContributionRaw))throw Error('maximum contribution invalid');
  for(const k of ['startTime','endTime'])uint(v[k],k,UINT64_MAX);if(BigInt(v.endTime)<=BigInt(v.startTime))throw Error('project time window invalid');
  for(const k of ['expectedProjectTokenPolicyDigest','expectedPaymentTokenPolicyDigest','expectedReleasePolicyDigest'])hash(v[k],k);
  for(const k of ['expectedProjectTokenPolicyVersion','expectedPaymentTokenPolicyVersion','expectedReleasePolicyVersion'])integer(v[k],k,1,UINT32_MAX);
  denseArray(v.vestingTimes,undefined,'vesting times');denseArray(v.vestingBps,v.vestingTimes.length,'vesting bps');if(v.vestingTimes.length<1||v.vestingTimes.length>10)throw Error('vesting schedule invalid');
  let previous=0n,totalBps=0;for(let i=0;i<v.vestingTimes.length;i++){uint(v.vestingTimes[i],'vesting time',UINT64_MAX);const t=BigInt(v.vestingTimes[i]);if(t<=previous||t<BigInt(v.endTime))throw Error('vesting schedule invalid');previous=t;integer(v.vestingBps[i],'vesting bps',1,UINT16_MAX);totalBps+=v.vestingBps[i];}if(totalBps!==10000)throw Error('vesting bps invalid');
  if(v.initialStatus!=='pending')throw Error('project initial status invalid');creationWitness(v.creationWitness);
  if(v.creationWitness.projectKey!==v.key||v.creationWitness.projectId!==v.projectId||BigInt(v.startTime)<=BigInt(v.creationWitness.blockTimestamp))throw Error('project creation witness mismatch');
}
function projects(v){denseArray(v,5,'project set');v.forEach((x,i)=>project(x,PROJECT_KEYS[i]));unique(v.map(x=>x.projectId),'project ids');}
function linkProject(v,rolesValue,assetsValue,safesValue){
  if(v.projectOwner!==rolesValue[PROJECT_OWNER_ROLES[v.key]])throw Error('project owner role linkage invalid');if(v.paymentToken!==assetsValue.paymentToken.address)throw Error('project payment token linkage invalid');
  const tokenKey=v.key.startsWith('shared')?'shared':v.key,projectToken=assetsValue.projectTokens.find(t=>t.key===tokenKey);if(!projectToken||v.projectToken!==projectToken.address)throw Error('project token linkage invalid');
  if(v.projectProceedsReceiver!==safesValue.proceeds.address||v.projectProceedsReceiverAttestationDigest!==safesValue.proceeds.attestationDigest||v.projectProceedsReceiverAttestationVersion!==safesValue.proceeds.attestationVersion)throw Error('project proceeds Safe linkage invalid');
  if(v.expectedPlatformFeeReceiver!==safesValue.platformFee.address||v.expectedPlatformFeeReceiverAttestationDigest!==safesValue.platformFee.attestationDigest||v.expectedPlatformFeeReceiverAttestationVersion!==safesValue.platformFee.attestationVersion||v.expectedPlatformFeeBps!==safesValue.platformFeeBps)throw Error('platform fee Safe linkage invalid');
  if(v.expectedProjectTokenPolicyDigest!==projectToken.policyDigest||v.expectedProjectTokenPolicyVersion!==projectToken.policyVersion)throw Error('project token policy linkage invalid');if(v.expectedPaymentTokenPolicyDigest!==assetsValue.paymentToken.policyDigest||v.expectedPaymentTokenPolicyVersion!==assetsValue.paymentToken.policyVersion)throw Error('payment token policy linkage invalid');if(v.expectedReleasePolicyDigest!==assetsValue.releasePolicy.digest||v.expectedReleasePolicyVersion!==assetsValue.releasePolicy.version)throw Error('release policy linkage invalid');
}
function source(v){exact(v,['commit','dirty','trackedTreeSha256','localFixtureTreeSha256','aggregateAbiDigest','selectorMapDigest','storageLayoutDigest','fullSetDigest'],'source');if(!/^[0-9a-f]{40}$/.test(v.commit||'')||typeof v.dirty!=='boolean')throw Error('source identity invalid');for(const k of Object.keys(v).filter(k=>k.endsWith('Sha256')))digest(v[k],k);for(const k of ['aggregateAbiDigest','selectorMapDigest','storageLayoutDigest','fullSetDigest'])hash(v[k],k);}
function deployment(v){exact(v,['proxyTxHash','proxyBlockNumber','proxyBlockHash','proxyBlockTimestamp','proxyTransactionIndex','observationTxHash','observationBlockNumber','observationBlockHash','observationBlockTimestamp','observationTransactionIndex','fixtureSnapshotId'],'deployment');hash(v.proxyTxHash,'proxy tx');uint(v.proxyBlockNumber,'proxy block number');hash(v.proxyBlockHash,'proxy block hash');uint(v.proxyBlockTimestamp,'proxy block timestamp',UINT64_MAX);integer(v.proxyTransactionIndex,'proxy transaction index');hash(v.observationTxHash,'observation tx');uint(v.observationBlockNumber,'observation block number');hash(v.observationBlockHash,'observation block hash');uint(v.observationBlockTimestamp,'observation block timestamp',UINT64_MAX);integer(v.observationTransactionIndex,'observation transaction index');if(comparePlacement(v.proxyBlockNumber,v.proxyTransactionIndex,v.observationBlockNumber,v.observationTransactionIndex)>=0)throw Error('deployment chronology invalid');if(!/^0x[0-9a-f]+$/.test(v.fixtureSnapshotId||''))throw Error('snapshot id invalid');}
function comparePlacement(aBlock,aIndex,bBlock,bIndex){const a=BigInt(aBlock),b=BigInt(bBlock);return a<b?-1:a>b?1:aIndex-bIndex;}
function normalizeDecodedValue(type,value,label){if(type==='address')return address(String(value).toLowerCase(),label);if(type==='bytes32')return hash(String(value).toLowerCase(),label);if(/^uint(16|32|64|256)$/.test(type))return uint(String(value),label,(1n<<BigInt(type.slice(4)))-1n);throw Error(`${label} unsupported ABI type`);}
function expectedCreationArguments(p){return{projectId:p.projectId,projectToken:p.projectToken,paymentToken:p.paymentToken,projectOwner:p.projectOwner,projectProceedsReceiver:p.projectProceedsReceiver,projectProceedsReceiverAttestationDigest:p.projectProceedsReceiverAttestationDigest,projectProceedsReceiverAttestationVersion:String(p.projectProceedsReceiverAttestationVersion),platformFeeReceiver:p.expectedPlatformFeeReceiver,platformFeeReceiverAttestationDigest:p.expectedPlatformFeeReceiverAttestationDigest,platformFeeReceiverAttestationVersion:String(p.expectedPlatformFeeReceiverAttestationVersion),platformFeeBps:String(p.expectedPlatformFeeBps),saleTokenAmount:p.saleTokenAmountRaw,softCap:p.softCapRaw,minContribution:p.minContributionRaw,maxContribution:String(p.maxContributionRaw),startTime:p.startTime,endTime:p.endTime,projectTokenPolicyDigest:p.expectedProjectTokenPolicyDigest,projectTokenPolicyVersion:String(p.expectedProjectTokenPolicyVersion),paymentTokenPolicyDigest:p.expectedPaymentTokenPolicyDigest,paymentTokenPolicyVersion:String(p.expectedPaymentTokenPolicyVersion),releasePolicyDigest:p.expectedReleasePolicyDigest,releasePolicyVersion:String(p.expectedReleasePolicyVersion)};}
function validateRawEvent(v,context){
  const fields=['name','signature','topic0','emitter','txLogOrdinal','topics','data','decodedArgs'];if(v&&v.name==='FairLaunchProjectCreated')fields.push('projectKey');exact(v,fields,'transaction event');integer(v.txLogOrdinal,'event ordinal');address(v.emitter,'event emitter');if(v.emitter!==context.proxyAddress)throw Error('event emitter mismatch');
  const definition=FROZEN_EVENT_DEFINITIONS.get(v.name);if(!definition||!['FairLaunchProjectCreated','FairLaunchActivated'].includes(v.name))throw Error('unsupported fixture event');if(v.signature!==definition.signature||v.topic0!==definition.topic0)throw Error('event manifest identity mismatch');
  denseArray(v.topics,1+definition.inputs.filter(x=>x.indexed).length,'event topics');v.topics.forEach((topic,index)=>hash(topic,`event topic ${index}`));if(v.topics[0]!==definition.topic0)throw Error('event topic0 mismatch');if(!/^0x(?:[0-9a-f]{2})*$/.test(v.data||''))throw Error('event data invalid');denseArray(v.decodedArgs,definition.inputs.length,'decoded arguments');
  const suppliedValues=v.decodedArgs.map((argument,index)=>{const input=definition.inputs[index];exact(argument,['ordinal','name','type','value'],'decoded argument');if(argument.ordinal!==index||argument.name!==input.name||argument.type!==input.type)throw Error('decoded argument projection invalid');return normalizeDecodedValue(input.type,argument.value,`decoded ${input.name}`);});
  let decoded;try{decoded=definition.iface.decodeEventLog(definition.fragment,v.data,v.topics);}catch{throw Error('raw event ABI decode invalid');}const rawValues=definition.inputs.map((input,index)=>normalizeDecodedValue(input.type,decoded[index].toString(),`raw ${input.name}`));if(canonicalJson(rawValues)!==canonicalJson(suppliedValues))throw Error('decoded arguments do not match raw log');
  const encoded=definition.iface.encodeEventLog(definition.fragment,suppliedValues);if(canonicalJson(encoded.topics.map(x=>x.toLowerCase()))!==canonicalJson(v.topics)||encoded.data.toLowerCase()!==v.data)throw Error('raw event encoding mismatch');
  if(v.name==='FairLaunchProjectCreated'){if(v.projectKey===undefined)throw Error('creation evidence label required');enumValue(v.projectKey,PROJECT_KEYS,'event project key');const project=context.projectsById.get(suppliedValues[0]);if(!project||project.key!==v.projectKey)throw Error('creation project evidence label mismatch');const expected=expectedCreationArguments(project),actual=Object.fromEntries(definition.inputs.map((input,index)=>[input.name,suppliedValues[index]]));if(canonicalJson(actual)!==canonicalJson(expected))throw Error('project creation arguments mismatch');}
  if(v.name==='FairLaunchActivated'){const project=context.projectsById.get(suppliedValues[0]);if(!project)throw Error('activated project id mismatch');}
}
function transaction(v,context){exact(v,['label','txHash','blockNumber','blockHash','blockTimestamp','transactionIndex','events'],'fixture transaction');scalarString(v.label,'transaction label');hash(v.txHash,'transaction hash');uint(v.blockNumber,'transaction block');hash(v.blockHash,'transaction block hash');uint(v.blockTimestamp,'transaction block timestamp',UINT64_MAX);integer(v.transactionIndex,'transaction index');denseArray(v.events,undefined,'transaction events');const ordinals=[];v.events.forEach(e=>{validateRawEvent(e,context);ordinals.push(e.txLogOrdinal);});unique(ordinals,'event ordinals');}
function eventIdentity(v){exact(v,['chainId','txHash','txLogOrdinal','eventName'],'event identity');if(v.chainId!=='31337')throw Error('event chain invalid');hash(v.txHash,'event tx hash');integer(v.txLogOrdinal,'event ordinal');scalarString(v.eventName,'event name');}
function validateFixtureEvidence(v){
  denseArray(v.fixtureTransactions,undefined,'fixture transactions');denseArray(v.fixtureEventIdentities,undefined,'fixture event identities');const context={proxyAddress:v.contracts.proxy.address,projectsById:new Map(v.projects.map(p=>[p.projectId,p]))};v.fixtureTransactions.forEach(tx=>transaction(tx,context));v.fixtureEventIdentities.forEach(eventIdentity);unique(v.fixtureTransactions.map(x=>x.label),'transaction labels');unique(v.fixtureTransactions.map(x=>x.txHash),'transaction hashes');
  const blockFacts=new Map(),blockNumberByHash=new Map(),placements=[];for(const tx of v.fixtureTransactions){const canonicalBlockNumber=BigInt(tx.blockNumber).toString(),existing=blockFacts.get(canonicalBlockNumber);if(existing&&(existing.blockHash!==tx.blockHash||existing.blockTimestamp!==tx.blockTimestamp))throw Error('transaction block identity mismatch');if(!existing){const priorBlockNumber=blockNumberByHash.get(tx.blockHash);if(priorBlockNumber!==undefined&&priorBlockNumber!==canonicalBlockNumber)throw Error('transaction block hash reused');blockFacts.set(canonicalBlockNumber,{blockNumber:canonicalBlockNumber,blockHash:tx.blockHash,blockTimestamp:tx.blockTimestamp});blockNumberByHash.set(tx.blockHash,canonicalBlockNumber);}placements.push(`${canonicalBlockNumber}:${tx.transactionIndex}`);}unique(placements,'transaction placements');
  const distinctBlocks=[...blockFacts.values()].sort((a,b)=>BigInt(a.blockNumber)<BigInt(b.blockNumber)?-1:BigInt(a.blockNumber)>BigInt(b.blockNumber)?1:0);for(let i=1;i<distinctBlocks.length;i++){if(BigInt(distinctBlocks[i].blockTimestamp)<=BigInt(distinctBlocks[i-1].blockTimestamp))throw Error('transaction block timestamp chronology invalid');}
  const proxies=v.fixtureTransactions.filter(x=>x.label==='proxy-deploy'),observations=v.fixtureTransactions.filter(x=>x.label==='observation');if(proxies.length!==1||observations.length!==1)throw Error('required transaction label mismatch');const proxy=proxies[0],observation=observations[0];
  if(proxy.txHash!==v.deployment.proxyTxHash||proxy.blockNumber!==v.deployment.proxyBlockNumber||proxy.blockHash!==v.deployment.proxyBlockHash||proxy.blockTimestamp!==v.deployment.proxyBlockTimestamp||proxy.transactionIndex!==v.deployment.proxyTransactionIndex)throw Error('proxy deployment transaction mismatch');
  if(observation.txHash!==v.deployment.observationTxHash||observation.blockNumber!==v.deployment.observationBlockNumber||observation.blockHash!==v.deployment.observationBlockHash||observation.blockTimestamp!==v.deployment.observationBlockTimestamp||observation.transactionIndex!==v.deployment.observationTransactionIndex)throw Error('observation transaction mismatch');
  const expectedActivated=v.projects.find(p=>p.key==='successful');if(observation.events.length!==1||observation.events[0].name!=='FairLaunchActivated'||observation.events[0].decodedArgs[0].value!==expectedActivated.projectId)throw Error('observation activation project mismatch');
  if(proxy.events.length!==0)throw Error('proxy deployment transaction emitted fixture events');for(const tx of v.fixtureTransactions){const fromProxy=comparePlacement(tx.blockNumber,tx.transactionIndex,proxy.blockNumber,proxy.transactionIndex),toObservation=comparePlacement(tx.blockNumber,tx.transactionIndex,observation.blockNumber,observation.transactionIndex);if(tx!==proxy&&(fromProxy<=0||toObservation>0))throw Error('fixture transaction placement outside observation');for(const event of tx.events){if(fromProxy<=0||toObservation>0)throw Error('proxy event placement outside observation');}}
  const flattened=v.fixtureTransactions.flatMap(tx=>tx.events.map(event=>({chainId:v.network.chainId,txHash:tx.txHash,txLogOrdinal:event.txLogOrdinal,eventName:event.name})));unique(flattened.map(x=>`${x.chainId}:${x.txHash}:${x.txLogOrdinal}`),'flattened event identities');unique(v.fixtureEventIdentities.map(x=>`${x.chainId}:${x.txHash}:${x.txLogOrdinal}`),'declared event identities');if(canonicalJson(flattened)!==canonicalJson(v.fixtureEventIdentities))throw Error('fixture event identity set mismatch');
  for(const p of v.projects){const w=p.creationWitness,tx=v.fixtureTransactions.find(x=>x.txHash===w.transactionHash);if(!tx||tx.blockNumber!==w.blockNumber||tx.blockHash!==w.blockHash||tx.blockTimestamp!==w.blockTimestamp||tx.transactionIndex!==w.transactionIndex)throw Error('project creation transaction mismatch');if(comparePlacement(tx.blockNumber,tx.transactionIndex,proxy.blockNumber,proxy.transactionIndex)<=0||comparePlacement(tx.blockNumber,tx.transactionIndex,observation.blockNumber,observation.transactionIndex)>=0)throw Error('project creation placement invalid');const event=tx.events.find(x=>x.txLogOrdinal===w.txLogOrdinal);if(!event||event.name!==w.eventName||event.projectKey!==p.key)throw Error('project creation event mismatch');}
}
function policy(v){header(v,'policy');integer(v.generation,'policy generation',1);for(const k of ['chainTrust','databaseTrust']){exact(v[k],['trustRootSha256','generation'],k);digest(v[k].trustRootSha256,`${k} digest`);integer(v[k].generation,`${k} generation`,1);}denseArray(v.allowedOrigins,undefined,'origins');if(v.allowedOrigins.length<1)throw Error('origins invalid');v.allowedOrigins.forEach((x)=>{const u=new URL(x);if(u.protocol!=='http:'||u.hostname!=='127.0.0.1'||!u.port||u.pathname!=='/'||u.search||u.hash)throw Error('origin invalid');});unique(v.allowedOrigins,'origins');if(JSON.stringify(v.allowedRpcUrls)!==JSON.stringify([RPC_URL]))throw Error('RPC policy invalid');exact(v.artifactSchemas,['chain','frontend','database'],'artifact schemas');for(const t of ['chain','frontend','database']){exact(v.artifactSchemas[t],['schemaVersion','status','signatureDomain'],'artifact schema');if(v.artifactSchemas[t].schemaVersion!==SCHEMAS[t]||v.artifactSchemas[t].status!==UNADMITTED_STATUS||v.artifactSchemas[t].signatureDomain!==DOMAINS[t])throw Error('artifact schema identity invalid');}}
function chain(v){header(v,'chain');digest(v.policyPayloadSha256,'policy digest');if(v.watermark!==UNADMITTED_STATUS)throw Error('watermark invalid');network(v.network);roles(v.roles);contracts(v.contracts);assets(v.assets);safes(v.safes);projects(v.projects);v.projects.forEach(p=>linkProject(p,v.roles,v.assets,v.safes));if(JSON.stringify(v.safes.owners)!==JSON.stringify(['safeOwnerA','safeOwnerB'].map(k=>v.roles[k]).concat(v.roles.governanceOwner)))throw Error('Safe owner role linkage invalid');source(v.source);deployment(v.deployment);validateFixtureEvidence(v);}
function frontend(v){header(v,'frontend');digest(v.policyPayloadSha256,'policy digest');digest(v.chainPayloadSha256,'chain digest');if(v.watermark!==UNADMITTED_STATUS)throw Error('watermark invalid');const o=new URL(v.frontendOrigin);if(o.protocol!=='http:'||o.hostname!=='127.0.0.1'||!o.port||o.pathname!=='/'||o.search||o.hash)throw Error('frontend origin invalid');network(v.network);roles(v.roles);project(v.project,'successful');contracts(v.contracts);assets(v.assets);safes(v.safes);linkProject(v.project,v.roles,v.assets,v.safes);deployment(v.deployment);}
function tip(v,label){exact(v,['blockNumber','blockHash','blockTimestamp'],'tip');uint(v.blockNumber,`${label} number`);hash(v.blockHash,`${label} hash`);uint(v.blockTimestamp,`${label} timestamp`,UINT64_MAX);}
function observationAnchor(v){exact(v,['transactionHash','transactionIndex','blockNumber','blockHash','blockTimestamp'],'DB anchor');hash(v.transactionHash,'DB anchor transaction hash');integer(v.transactionIndex,'DB anchor transaction index');uint(v.blockNumber,'DB anchor number');hash(v.blockHash,'DB anchor hash');uint(v.blockTimestamp,'DB anchor timestamp',UINT64_MAX);}
function database(v){header(v,'database');digest(v.policyPayloadSha256,'policy digest');if(!/^db-checkpoint:[a-z0-9][a-z0-9-]{7,63}:[1-9]\d*$/.test(v.snapshotId||''))throw Error('DB snapshot invalid');digest(v.captureDigest,'capture digest');digest(v.chainPayloadSha256,'chain digest');observationAnchor(v.anchor);tip(v.captureTip,'DB capture tip');if(BigInt(v.captureTip.blockNumber)<=BigInt(v.anchor.blockNumber)||BigInt(v.captureTip.blockTimestamp)<BigInt(v.anchor.blockTimestamp))throw Error('DB capture chronology invalid');digest(v.databaseFingerprintSha256,'DB fingerprint');digest(v.checkpointSha256,'DB checkpoint');if(v.state!=='checkpointed')throw Error('DB state invalid');}
function artifactIdentity(payload,type){return{schemaVersion:payload.schemaVersion,status:payload.status,signatureDomain:payload.signatureDomain};}
function createCapability(type,metadata){const token=Object.freeze({status:UNADMITTED_STATUS,type});capabilityMetadata.set(token,immutableCopy(metadata,`${type} capability metadata`));return token;}
function metadataFor(token,type){const metadata=token&&capabilityMetadata.get(token);if(!metadata||metadata.type!==type)throw Error(`validated ${type} result required`);return metadata;}
function requirePolicyResult(v,policyResult){
  const metadata=metadataFor(policyResult,'policy');if(metadata.runId!==v.runId||metadata.payloadSha256!==v.policyPayloadSha256)throw Error('policy payload linkage invalid');const projected=metadata.artifactSchemas[v.signatureDomain===DOMAINS.chain?'chain':v.signatureDomain===DOMAINS.frontend?'frontend':v.signatureDomain===DOMAINS.database?'database':''];if(!projected||canonicalJson({schemaVersion:projected.schemaVersion,status:projected.status,signatureDomain:projected.signatureDomain})!==canonicalJson(artifactIdentity(v)))throw Error('policy artifact schema linkage invalid');return metadata;
}
function requireChainResult(v,chainResult){const metadata=metadataFor(chainResult,'chain');if(metadata.runId!==v.runId||metadata.policyPayloadSha256!==v.policyPayloadSha256||metadata.payloadSha256!==v.chainPayloadSha256)throw Error('chain payload linkage invalid');return metadata;}
function validateStructuralPayload(input,type,{nowMs,policyResult,rootInterval,chainResult}={}){
  const v=clonePlainData(input,`${type} payload`);const validator={policy,chain,frontend,database}[type];if(!validator)throw Error('payload type invalid');validator(v);
  const containers=[];let policyMetadata;
  if(type!=='policy'){policyMetadata=requirePolicyResult(v,policyResult);containers.push(policyMetadata.interval);}
  if(rootInterval)containers.push(clonePlainData(rootInterval,'root interval'));
  let chainMetadata;
  if(type==='frontend'){
    chainMetadata=requireChainResult(v,chainResult);if(!policyMetadata.allowedOrigins.includes(v.frontendOrigin))throw Error('frontend origin is not allowed by policy');
    const projection={network:v.network,roles:v.roles,project:v.project,contracts:v.contracts,assets:v.assets,safes:v.safes,deployment:v.deployment};if(canonicalJson(projection)!==canonicalJson(chainMetadata.frontendProjection))throw Error('frontend projection differs from validated chain');
  }
  if(type==='database'){
    chainMetadata=requireChainResult(v,chainResult);if(v.anchor.transactionHash!==chainMetadata.observation.transactionHash||v.anchor.transactionIndex!==chainMetadata.observation.transactionIndex||v.anchor.blockNumber!==chainMetadata.observation.blockNumber||v.anchor.blockHash!==chainMetadata.observation.blockHash||v.anchor.blockTimestamp!==chainMetadata.observation.blockTimestamp||chainMetadata.chainId!=='31337')throw Error('DB anchor is not the validated chain observation');
  }
  const validInterval=interval(v,nowMs,`${type} payload`,containers);const payloadSha256=sha256(canonicalJson(v));
  const metadata={status:UNADMITTED_STATUS,type,runId:v.runId,payloadSha256,interval:validInterval};
  if(type==='policy')Object.assign(metadata,{allowedOrigins:v.allowedOrigins,chainTrust:v.chainTrust,databaseTrust:v.databaseTrust,artifactSchemas:Object.fromEntries(Object.entries(v.artifactSchemas).map(([key,value])=>[key,{type:key,...value}]))});
  if(type==='chain')Object.assign(metadata,{policyPayloadSha256:v.policyPayloadSha256,chainId:v.network.chainId,genesisHash:v.network.genesisHash,observation:{transactionHash:v.deployment.observationTxHash,transactionIndex:v.deployment.observationTransactionIndex,blockNumber:v.deployment.observationBlockNumber,blockHash:v.deployment.observationBlockHash,blockTimestamp:v.deployment.observationBlockTimestamp},frontendProjection:{network:v.network,roles:v.roles,project:v.projects.find(p=>p.key==='successful'),contracts:v.contracts,assets:v.assets,safes:v.safes,deployment:v.deployment}});
  return createCapability(type,metadata);
}
function verifyStructuralEnvelope(envelopeInput,{type,runId:expectedRunId,generation,trustRootSha256,expectedPayloadSha256,nowMs,policyResult,chainResult,publicRoot:rootInput}={}){
  const envelope=clonePlainData(envelopeInput,'structural envelope'),root=clonePlainData(rootInput,'public root');exact(envelope,['schemaVersion','status','signatureDomain','payload','trust','signatures'],'structural envelope');if(envelope.schemaVersion!==SIGNED_ENVELOPE_SCHEMA||envelope.status!==UNADMITTED_STATUS||envelope.signatureDomain!==DOMAINS[type])throw Error('structural envelope invalid');
  let linkedTrust={trustRootSha256,generation};if(type!=='policy'){const policyMetadata=metadataFor(policyResult,'policy');linkedTrust=type==='database'?policyMetadata.databaseTrust:policyMetadata.chainTrust;if(trustRootSha256!==linkedTrust.trustRootSha256||generation!==linkedTrust.generation)throw Error('policy trust projection mismatch');}
  const rootType=type==='frontend'?'chain':type;const validatedRoot=publicRoot(root,{rootType,runId:expectedRunId,generation:linkedTrust.generation,trustRootSha256:linkedTrust.trustRootSha256,nowMs});
  const payloadResult=validateStructuralPayload(envelope.payload,type,{nowMs,policyResult,chainResult,rootInterval:validatedRoot.interval});const payloadMetadata=metadataFor(payloadResult,type);if(payloadMetadata.runId!==expectedRunId)throw Error('expected run mismatch');if(payloadMetadata.payloadSha256!==expectedPayloadSha256)throw Error('expected payload digest mismatch');
  exact(envelope.trust,['trustRootSha256','generation','threshold','keyIds'],'envelope trust');if(envelope.trust.trustRootSha256!==linkedTrust.trustRootSha256||envelope.trust.generation!==linkedTrust.generation||envelope.trust.threshold!==2)throw Error('expected trust facts mismatch');if(JSON.stringify(envelope.trust.keyIds)!==JSON.stringify(validatedRoot.keyIds))throw Error('root key projection mismatch');
  denseArray(envelope.signatures,undefined,'signature set');const bytes=Buffer.from(canonicalJson({signatureDomain:DOMAINS[type],payload:envelope.payload})),keys=new Map(root.keys.map(k=>[derivedKeyId(k.publicKeyJwk),crypto.createPublicKey({key:k.publicKeyJwk,format:'jwk'})])),validKeyDigests=new Set();
  for(const s of envelope.signatures){exact(s,['algorithm','signatureDomain','keyId','payloadSha256','value'],'signature');const key=keys.get(s.keyId);if(key&&s.algorithm==='ed25519'&&s.signatureDomain===DOMAINS[type]&&s.payloadSha256===payloadMetadata.payloadSha256&&crypto.verify(null,bytes,key,Buffer.from(s.value,'base64')))validKeyDigests.add(s.keyId);}
  if(validKeyDigests.size<2)throw Error('structural threshold signature invalid');return payloadResult;
}

module.exports={CHAIN_AUTHORITY:UNADMITTED_STATUS,DB_ACK_SCHEMA,DEFAULT_RUN_ROOT,DOMAINS,FRONTEND_ADMISSION_SCHEMA:FRONTEND_SCHEMA,MANIFEST_SCHEMA:CHAIN_SCHEMA,MAX_ARTIFACT_LIFETIME_MS,MAX_CLOCK_SKEW_MS,POLICY_APPROVAL_SCHEMA:POLICY_SCHEMA,REPOSITORY_ROOT,ROLE_NAMES,ROOT_SCHEMA,RPC_URL,SCHEMAS,SIGNED_ENVELOPE_SCHEMA,UNADMITTED_STATUS,canonical,canonicalJson,derivedKeyId,parseExactJson,readJson,sanitizePublicDescriptor:publicDescriptor,sha256,validateStructuralPayload,verifyStructuralEnvelope};
