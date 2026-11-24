'use strict';
const fs=require('fs'),path=require('path'),crypto=require('crypto'),assert=require('assert');
const {canonicalize}=require('./jcs');
const root=path.resolve(process.argv[2]||path.join(__dirname,'..')), schemasDir=path.join(root,'schemas'), vectorsDir=path.join(root,'vectors'), detachedDir=path.join(root,'detached-release');
const sha=value=>crypto.createHash('sha256').update(value).digest('hex');
function resolveRef(ref){const [file,pointer]=ref.split('#');let value=JSON.parse(fs.readFileSync(path.join(schemasDir,file),'utf8'));if(pointer)for(const token of pointer.split('/').slice(1))value=value[token.replace(/~1/g,'/').replace(/~0/g,'~')];return value;}
function same(a,b){return JSON.stringify(a)===JSON.stringify(b)}
function validate(value,rule,at='$'){
 if(rule.$ref)return validate(value,resolveRef(rule.$ref),at);
 if(rule.const!==undefined)assert(same(value,rule.const),`${at}: const`);if(rule.enum)assert(rule.enum.some(x=>same(x,value)),`${at}: enum`);
 if(rule.oneOf)assert.equal(rule.oneOf.filter(x=>{try{validate(value,x,at);return true}catch{return false}}).length,1,`${at}: oneOf`);
 if(rule.not){let failed=false;try{validate(value,rule.not,at)}catch{failed=true}assert(failed,`${at}: not`)}
 if(rule.required)for(const key of rule.required)assert(Object.hasOwn(value,key),`${at}.${key}: required`);
 if(rule.type){const types=Array.isArray(rule.type)?rule.type:[rule.type];const actual=value===null?'null':Array.isArray(value)?'array':Number.isInteger(value)?'integer':typeof value;assert(types.includes(actual),`${at}: type ${actual}`)}
 if(typeof value==='string'){if(rule.pattern)assert(new RegExp(rule.pattern).test(value),`${at}: pattern`);if(rule.minLength!==undefined)assert(value.length>=rule.minLength);if(rule.maxLength!==undefined)assert(value.length<=rule.maxLength)}
 if(typeof value==='number'){if(rule.minimum!==undefined)assert(value>=rule.minimum);if(rule.maximum!==undefined)assert(value<=rule.maximum)}
 if(Array.isArray(value)){if(rule.minItems!==undefined)assert(value.length>=rule.minItems);if(rule.maxItems!==undefined)assert(value.length<=rule.maxItems);if(rule.uniqueItems)assert.equal(new Set(value.map(canonicalize)).size,value.length);if(rule.items)value.forEach((x,i)=>validate(x,rule.items,`${at}[${i}]`))}
 if(value&&typeof value==='object'&&!Array.isArray(value)){if(rule.additionalProperties===false)for(const key of Object.keys(value))assert(rule.properties&&key in rule.properties,`${at}.${key}: additional property`);if(rule.properties)for(const [key,child]of Object.entries(rule.properties))if(key in value)validate(value[key],child,`${at}.${key}`)}
}
const mapping={'image-capability':'image-capability.schema.json','environment-bootstrap':'environment-bootstrap.schema.json','key-set':'key-set.schema.json','key-revocation':'key-revocation.schema.json','fair-launch-watermark':'fair-launch-watermark.schema.json','deployment-attestation':'deployment-attestation.schema.json','restore-recovery-attestation':'restore-recovery-attestation.schema.json','signature-envelope':'signature-envelope.schema.json'};
for(const [name,schemaName]of Object.entries(mapping)){const input=JSON.parse(fs.readFileSync(path.join(vectorsDir,`${name}.input.json`),'utf8')),canonical=fs.readFileSync(path.join(vectorsDir,`${name}.canonical.json`),'utf8');assert.equal(canonicalize(input),canonical,`${name}: canonical bytes`);const [hash,file]=fs.readFileSync(path.join(vectorsDir,`${name}.sha256`),'utf8').trim().split(/  /);assert.equal(file,`${name}.canonical.json`);assert.equal(sha(Buffer.from(canonical)),hash);validate(input,JSON.parse(fs.readFileSync(path.join(schemasDir,schemaName),'utf8')));}
const read=name=>JSON.parse(fs.readFileSync(path.join(vectorsDir,`${name}.input.json`),'utf8'));
const image=read('image-capability'),keySet=read('key-set'),bootstrap=read('environment-bootstrap'),watermark=read('fair-launch-watermark'),deployment=read('deployment-attestation'),restore=read('restore-recovery-attestation'),revocation=read('key-revocation');
assert.equal(new Set(keySet.keys.map(x=>x.keyId)).size,keySet.keys.length);assert.equal(new Set(keySet.keys.map(x=>x.publicKeyBase64)).size,keySet.keys.length);assert.equal(keySet.signaturesByPreviousKeySet.length,0);
const spkiPrefix=Buffer.from('302a300506032b6570032100','hex');
const publicKeyForRaw=record=>crypto.createPublicKey({key:Buffer.concat([spkiPrefix,Buffer.from(record.publicKeyBase64,'base64')]),format:'der',type:'spki'});
const signedFamilies=['image-capability','environment-bootstrap','key-set','key-revocation','fair-launch-watermark','deployment-attestation','restore-recovery-attestation'];
for(const name of signedFamilies){
 const envelopeName=name==='image-capability'?'signature-envelope':`${name}.signature-envelope`;
 const envelope=JSON.parse(fs.readFileSync(path.join(vectorsDir,`${envelopeName}.input.json`),'utf8'));
 const envelopeCanonical=fs.readFileSync(path.join(vectorsDir,`${envelopeName}.canonical.json`),'utf8');
 const [envelopeHash,envelopeFile]=fs.readFileSync(path.join(vectorsDir,`${envelopeName}.sha256`),'utf8').trim().split(/  /);
 assert.equal(canonicalize(envelope),envelopeCanonical,`${envelopeName}: canonical bytes`);assert.equal(envelopeFile,`${envelopeName}.canonical.json`);assert.equal(sha(Buffer.from(envelopeCanonical)),envelopeHash);validate(envelope,JSON.parse(fs.readFileSync(path.join(schemasDir,'signature-envelope.schema.json'),'utf8')));
 const payload=fs.readFileSync(path.join(vectorsDir,`${name}.canonical.json`)),payloadObject=read(name);
 assert.equal(envelope.payloadSha256,sha(payload));assert.equal(envelope.payloadDomain,payloadObject.domain);assert.equal(envelope.keySetVersion,keySet.keySetVersion);assert.equal(new Set(envelope.signatures.map(x=>x.keyId)).size,envelope.signatures.length);
 let valid=0;for(const signature of envelope.signatures){const key=keySet.keys.find(x=>x.keyId===signature.keyId);assert(key);const rawKey=Buffer.from(key.publicKeyBase64,'base64'),rawSig=Buffer.from(signature.signatureBase64,'base64');assert.equal(rawKey.length,32);assert.equal(rawSig.length,64);const publicKey=crypto.createPublicKey({key:Buffer.concat([spkiPrefix,rawKey]),format:'der',type:'spki'});if(crypto.verify(null,payload,publicKey,rawSig))valid++;}assert(valid>=keySet.threshold,`${name}: threshold`);const tampered=Buffer.concat([payload,Buffer.from('x')]);for(const signature of envelope.signatures){const key=keySet.keys.find(x=>x.keyId===signature.keyId);assert.equal(crypto.verify(null,tampered,publicKeyForRaw(key),Buffer.from(signature.signatureBase64,'base64')),false,`${name}: tampered payload`);}
}
const signedPayload=fs.readFileSync(path.join(vectorsDir,'image-capability.canonical.json'));
assert(keySet.validFromGeneration<=image.generation&&image.generation<=keySet.validThroughGeneration);assert(keySet.revokedAtGeneration===null||image.generation<keySet.revokedAtGeneration);assert.equal(revocation.revokedKeyIds.includes('key-c'),true);assert(revocation.effectiveGeneration>image.generation);
const artifact=fs.readFileSync(path.join(detachedDir,'artifact.bin')),artifactHash=sha(artifact),[sidecarHash]=fs.readFileSync(path.join(detachedDir,'artifact.sha256'),'utf8').trim().split(/  /);assert.equal(artifactHash,sidecarHash);assert.equal(artifactHash,image.artifactSha256);
const allCanonical=fs.readdirSync(vectorsDir).filter(name=>name.endsWith('.canonical.json')).map(name=>fs.readFileSync(path.join(vectorsDir,name)));const domains=['joincare.release.image-capability.v1','joincare.release.environment-bootstrap.v1','joincare.release.key-set.v1','joincare.release.key-revocation.v1','joincare.release.fair-launch-watermark.v1','joincare.release.deployment-attestation.v1','joincare.restore-recovery-attestation.v1','joincare.release.detached-signature-envelope.v1'];for(const needle of [...allCanonical,...domains.map(Buffer.from)])assert.equal(artifact.includes(needle),false,'artifact contains detached material');
const requiredCapabilities=['fair-launch.claim-refund-projection','fair-launch.e8-payment-token','fair-launch.external-end-convergence','fair-launch.governance-workflows','fair-launch.ingestion','fair-launch.materialization','fair-launch.outbox-recovery','fair-launch.reconciliation','fair-launch.source-registry'];assert.deepEqual(image.capabilities,requiredCapabilities);assert.deepEqual(watermark.requiredCapabilities,requiredCapabilities);
assert.equal(bootstrap.watermarkRootKeySetDigest,sha(fs.readFileSync(path.join(vectorsDir,'key-set.canonical.json'))));assert.equal(bootstrap.sourceRegistrySignerKeySetDigest,sha(fs.readFileSync(path.join(vectorsDir,'key-set.canonical.json'))));
assert.equal(bootstrap.environmentId,image.environmentId);assert.equal(watermark.environmentId,image.environmentId);assert.equal(bootstrap.chainId,watermark.chainId);assert.equal(bootstrap.fairLaunchProxyAddress,watermark.proxyAddress);assert.equal(watermark.minimumBackendJobManifestGeneration,image.generation);
const eventOrder=['CE-V1','CE-V2-shadow','CE-V2-read','CE-V2-only'];assert(eventOrder.indexOf(image.eventModel)>=eventOrder.indexOf(watermark.minimumEventModel));
assert.equal(restore.bootstrapDigest,sha(fs.readFileSync(path.join(vectorsDir,'environment-bootstrap.canonical.json'))));assert.equal(restore.effectiveWatermarkDigest,sha(fs.readFileSync(path.join(vectorsDir,'fair-launch-watermark.canonical.json'))));assert.equal(deployment.manifestDigest,sha(signedPayload));assert.equal(deployment.artifactSha256,image.artifactSha256);assert.equal(deployment.containerImageId,image.containerImageId);assert.equal(deployment.environmentId,watermark.environmentId);assert.equal(deployment.chainId,watermark.chainId);assert.equal(deployment.proxyAddress,watermark.proxyAddress);assert.equal(deployment.implementationCodeHash,watermark.implementationCodeHash);assert.equal(deployment.sourceRegistryRevision,watermark.sourceRegistryRevision);assert.equal(restore.deploymentAttestationDigest,sha(fs.readFileSync(path.join(vectorsDir,'deployment-attestation.canonical.json'))));assert(restore.replayFromCheckpoint<=restore.replayThroughBlock);assert(Date.parse(restore.snapshotCreatedAt)<=Date.parse(restore.completedAt));const roles=[restore.initiatorId,restore.executorId,...restore.approverIds];assert.equal(new Set(roles).size,roles.length);
const transitions=JSON.parse(fs.readFileSync(path.join(vectorsDir,'trust-transition-vectors.json'),'utf8'));
assert.equal(transitions.schemaVersion,2);validate(transitions.keyRotation.previousKeySet,JSON.parse(fs.readFileSync(path.join(schemasDir,'key-set.schema.json'),'utf8')));validate(transitions.keyRotation.nextKeySet,JSON.parse(fs.readFileSync(path.join(schemasDir,'key-set.schema.json'),'utf8')));
for(const v of transitions.previousDigest){const expected=v.nextGeneration>v.previousGeneration&&v.nextPreviousDigest===v.previousDigest;assert.equal(expected,v.accepted,v.id);}
const publicKeyFor = record => crypto.createPublicKey({key:Buffer.concat([spkiPrefix,Buffer.from(record.publicKeyBase64,'base64')]),format:'der',type:'spki'});
function verifyRotation(previous,next){
 if(next.supersedesKeySetVersion!==previous.keySetVersion)return false;
 if(next.previousKeySetDigest!==sha(Buffer.from(canonicalize(previous))))return false;
 if(next.keySetVersion<=previous.keySetVersion||next.validFromGeneration<=previous.validThroughGeneration)return false;
 const unsigned=structuredClone(next);unsigned.signaturesByPreviousKeySet=[];const bytes=Buffer.from(canonicalize(unsigned));
 const unique=new Set();let valid=0;for(const sig of next.signaturesByPreviousKeySet){if(unique.has(sig.keyId))continue;unique.add(sig.keyId);const key=previous.keys.find(x=>x.keyId===sig.keyId);if(key&&crypto.verify(null,bytes,publicKeyFor(key),Buffer.from(sig.signatureBase64,'base64')))valid++;}
 return valid>=previous.threshold;
}
assert.equal(sha(Buffer.from(canonicalize({...transitions.keyRotation.nextKeySet,signaturesByPreviousKeySet:[]}))),transitions.keyRotation.signingPayloadSha256);
assert.equal(verifyRotation(transitions.keyRotation.previousKeySet,transitions.keyRotation.nextKeySet),transitions.keyRotation.expected.accepted);
for(const vector of transitions.keyRotation.negative){
 if(vector.id==='new-key-before-valid-from'){assert.equal(vector.manifestGeneration>=transitions.keyRotation.nextKeySet.validFromGeneration,vector.accepted);continue;}
 if(vector.id==='old-key-after-valid-through'){assert.equal(vector.manifestGeneration<=transitions.keyRotation.previousKeySet.validThroughGeneration,vector.accepted);continue;}
 const mutated=structuredClone(transitions.keyRotation.nextKeySet);
 if(vector.mutation==='previousKeySetDigest')mutated.previousKeySetDigest=vector.value;
 if(vector.mutation==='dropSignature')mutated.signaturesByPreviousKeySet=mutated.signaturesByPreviousKeySet.slice(0,1);
 if(vector.mutation==='unknownSigner')mutated.signaturesByPreviousKeySet[0].keyId='unknown-key';
 if(vector.mutation==='validThroughGeneration')mutated.validThroughGeneration=vector.value;
 assert.equal(verifyRotation(transitions.keyRotation.previousKeySet,mutated),vector.accepted,vector.id);
}
for(const v of transitions.revocation){const signerIds=v.keyIds||[v.keyId];const accepted=v.manifestGeneration<revocation.effectiveGeneration||!signerIds.some(id=>revocation.revokedKeyIds.includes(id));assert.equal(accepted,v.accepted,v.id);}
const schemaRank=x=>transitions.schemaOrder.indexOf(x),eventRank=x=>transitions.eventOrder.indexOf(x);
function typedJoin(v){const identityFields=['environmentId','chainId','proxyAddress','implementationCodeHash'];for(const field of identityFields)if(v.left[field]!==v.right[field])return{accepted:false,reason:`identity-conflict:${field}`};if(v.right.sourceRegistryRevision<v.left.sourceRegistryRevision)return{accepted:false,reason:'source-registry-regression'};return{accepted:true,generation:Math.max(v.left.generation,v.right.generation),eventModel:eventRank(v.left.eventModel)>eventRank(v.right.eventModel)?v.left.eventModel:v.right.eventModel,schemaVersion:schemaRank(v.left.schemaVersion)>schemaRank(v.right.schemaVersion)?v.left.schemaVersion:v.right.schemaVersion,implementationCodeHash:v.left.implementationCodeHash,sourceRegistryRevision:Math.max(v.left.sourceRegistryRevision,v.right.sourceRegistryRevision),capabilities:[...new Set([...v.left.capabilities,...v.right.capabilities])].sort()}}
for(const vector of transitions.typedJoin)assert.deepEqual(vector.id==='capability-union-and-max'?typedJoin(vector):typedJoin({...vector,left:{generation:0,eventModel:'CE-V1',schemaVersion:transitions.schemaOrder[0],capabilities:[],...vector.left},right:{generation:0,eventModel:'CE-V1',schemaVersion:transitions.schemaOrder[0],capabilities:[],...vector.right}}),vector.expected,vector.id);
assert.equal(canonicalize({numbers:[333333333.33333329,1e30,4.50,2e-3,1e-27]}),'{"numbers":[333333333.3333333,1e+30,4.5,0.002,1e-27]}');
const unicodeObject={};unicodeObject['€']='Euro';unicodeObject['\r']='CR';unicodeObject['1']='One';unicodeObject['😀']='Emoji';unicodeObject['ö']='Latin';
const unicodeCanonical=canonicalize(unicodeObject);assert(unicodeCanonical.indexOf('\"\\r\"')<unicodeCanonical.indexOf('\"1\"'));assert(unicodeCanonical.indexOf('\"1\"')<unicodeCanonical.indexOf('\"ö\"'));assert(unicodeCanonical.indexOf('\"ö\"')<unicodeCanonical.indexOf('\"€\"'));assert(unicodeCanonical.indexOf('\"€\"')<unicodeCanonical.indexOf('\"😀\"'));
assert.throws(()=>canonicalize({[String.fromCharCode(0xd800)]:'bad'}),/surrogate/);
assert.throws(()=>canonicalize(String.fromCharCode(0xdc00)),/surrogate/);
const mutated=Buffer.from(artifact);mutated[0]^=1;assert.notEqual(sha(mutated),image.artifactSha256);
const embedded=Buffer.concat([artifact,...allCanonical]);assert(allCanonical.some(x=>embedded.includes(x)));
console.log('Fair Launch RFC 8785 evidence: PASS');
