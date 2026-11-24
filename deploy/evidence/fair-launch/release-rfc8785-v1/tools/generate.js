'use strict';

const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const { canonicalize } = require('./jcs');

const root = path.resolve(process.argv[2] || path.join(__dirname, '..'));
const schemas = path.join(root, 'schemas');
const vectors = path.join(root, 'vectors');
const detached = path.join(root, 'detached-release');
for (const dir of [schemas, vectors, detached]) fs.mkdirSync(dir, { recursive: true });
const sha = value => crypto.createHash('sha256').update(value).digest('hex');
const hex = char => char.repeat(64);
const ref = name => ({ $ref: `common.schema.json#/$defs/${name}` });
const common = {
  $schema: 'https://json-schema.org/draft/2020-12/schema',
  $id: 'https://joincare.example/schemas/fair-launch/common.schema.json',
  $defs: {
    digest256: { type: 'string', pattern: '^[0-9a-f]{64}$' },
    evmAddress: { type: 'string', pattern: '^0x[0-9A-Fa-f]{40}$' },
    blockHash: { type: 'string', pattern: '^0x[0-9a-f]{64}$' },
    timestamp: { type: 'string', pattern: '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$' },
    uint: { type: 'integer', minimum: 0, maximum: Number.MAX_SAFE_INTEGER },
    positiveUint: { type: 'integer', minimum: 1, maximum: Number.MAX_SAFE_INTEGER },
    nullableDigest: { type: ['string', 'null'], pattern: '^[0-9a-f]{64}$' }
  }
};
function schema(name, domain, required, properties, extra = {}) {
  return {
    $schema: common.$schema,
    $id: `https://joincare.example/schemas/fair-launch/${name}.schema.json`,
    type: 'object', additionalProperties: false, required,
    properties: { domain: { const: domain }, ...properties }, ...extra
  };
}
const digest = ref('digest256');
const uint = ref('uint');
const positiveUint = ref('positiveUint');
const timestamp = ref('timestamp');
const address = ref('evmAddress');
const nullableDigest = ref('nullableDigest');
const capabilityArray = { type: 'array', minItems: 1, maxItems: 32, uniqueItems: true, items: { type: 'string', pattern: '^fair-launch\\.[a-z0-9.-]+$' } };
const schemaFiles = {
  'common.schema.json': common,
  'image-capability.schema.json': schema('image-capability.v1', 'joincare.release.image-capability.v1', ['domain','environmentId','generation','issuedAt','expiresAt','keySetVersion','component','artifactSha256','containerImageId','sourceCommit','schemaMin','schemaMax','eventModel','capabilities','maintenanceModes','previousManifestDigest'], {
    environmentId:{type:'string',pattern:'^[a-z0-9-]{3,64}$'},generation:positiveUint,issuedAt:timestamp,expiresAt:timestamp,keySetVersion:positiveUint,component:{const:'backend-job'},artifactSha256:digest,containerImageId:{type:'string',pattern:'^sha256:[0-9a-f]{64}$'},sourceCommit:{type:'string',pattern:'^[0-9a-f]{40}$'},schemaMin:{type:'string',minLength:1},schemaMax:{type:'string',minLength:1},eventModel:{enum:['CE-V1','CE-V2-shadow','CE-V2-read','CE-V2-only']},capabilities:capabilityArray,maintenanceModes:{type:'array',minItems:1,uniqueItems:true,items:{enum:['signer-recovery','write-disabled-replay']}},previousManifestDigest:nullableDigest
  }),
  'environment-bootstrap.schema.json': schema('environment-bootstrap.v1', 'joincare.release.environment-bootstrap.v1', ['domain','environmentId','chainId','minimumDiscoveryBlock','releaseControlPlaneId','pinnedHttpsUrl','tlsSpkiSha256','watermarkRootKeySetDigest','sourceRegistryAuthorityKind','sourceRegistrySignerKeySetDigest','hostStateVolumeId','enrollmentGeneration','issuedAt','expiresAt','previousBootstrapDigest'], {
    environmentId:{type:'string',pattern:'^[a-z0-9-]{3,64}$'},chainId:uint,fairLaunchProxyAddress:address,approvedDeploymentFactoryAddress:address,minimumDiscoveryBlock:uint,releaseControlPlaneId:{type:'string',minLength:1},pinnedHttpsUrl:{type:'string',pattern:'^https://'},tlsSpkiSha256:digest,watermarkRootKeySetDigest:digest,sourceRegistryAuthorityKind:{const:'offchain-signed-registry'},sourceRegistrySignerKeySetDigest:digest,hostStateVolumeId:{type:'string',pattern:'^[a-zA-Z0-9._:-]{3,128}$'},enrollmentGeneration:positiveUint,issuedAt:timestamp,expiresAt:{type:['string','null'],pattern:'^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$'},previousBootstrapDigest:nullableDigest
  }, { oneOf: [{required:['fairLaunchProxyAddress'],not:{required:['approvedDeploymentFactoryAddress']}},{required:['approvedDeploymentFactoryAddress'],not:{required:['fairLaunchProxyAddress']}}] }),
  'key-set.schema.json': schema('key-set.v1', 'joincare.release.key-set.v1', ['domain','keySetVersion','threshold','keys','validFromGeneration','validThroughGeneration','supersedesKeySetVersion','previousKeySetDigest','revokedAtGeneration','revocationReason','issuedAt','signaturesByPreviousKeySet'], {
    keySetVersion:positiveUint,threshold:{type:'integer',minimum:2,maximum:3},keys:{type:'array',minItems:3,maxItems:3,uniqueItems:true,items:{type:'object',additionalProperties:false,required:['keyId','algorithm','publicKeyBase64'],properties:{keyId:{type:'string',pattern:'^[a-z0-9-]{1,32}$'},algorithm:{const:'Ed25519'},publicKeyBase64:{type:'string',pattern:'^[A-Za-z0-9+/]{43}=$'}}}},validFromGeneration:positiveUint,validThroughGeneration:positiveUint,supersedesKeySetVersion:{type:['integer','null'],minimum:1},previousKeySetDigest:nullableDigest,revokedAtGeneration:{type:['integer','null'],minimum:1},revocationReason:{type:['string','null'],maxLength:256},issuedAt:timestamp,signaturesByPreviousKeySet:{type:'array',minItems:0,maxItems:3,items:{type:'object',additionalProperties:false,required:['keyId','algorithm','signatureBase64'],properties:{keyId:{type:'string',pattern:'^[a-z0-9-]{1,32}$'},algorithm:{const:'Ed25519'},signatureBase64:{type:'string',pattern:'^[A-Za-z0-9+/]{86}==$'}}}}
  }),
  'key-revocation.schema.json': schema('key-revocation.v1', 'joincare.release.key-revocation.v1', ['domain','keySetVersion','revokedKeyIds','effectiveGeneration','reason','issuedAt','previousRevocationDigest'], {
    keySetVersion:positiveUint,revokedKeyIds:{type:'array',minItems:1,maxItems:3,uniqueItems:true,items:{type:'string',pattern:'^[a-z0-9-]{1,32}$'}},effectiveGeneration:positiveUint,reason:{type:'string',minLength:1,maxLength:256},issuedAt:timestamp,previousRevocationDigest:nullableDigest
  }),
  'fair-launch-watermark.schema.json': schema('fair-launch-watermark.v1', 'joincare.release.fair-launch-watermark.v1', ['domain','generation','environmentId','chainId','proxyAddress','implementationCodeHash','sourceRegistryRevision','firstEnabledBlock','firstProjectCreatedBlock','minimumEventModel','requiredCapabilities','minimumSchemaVersion','minimumBackendJobManifestGeneration','previousWatermarkDigest','issuedAt','keySetVersion'], {
    generation:positiveUint,environmentId:{type:'string',pattern:'^[a-z0-9-]{3,64}$'},chainId:uint,proxyAddress:address,implementationCodeHash:digest,sourceRegistryRevision:positiveUint,firstEnabledBlock:{type:['integer','null'],minimum:0},firstProjectCreatedBlock:{type:['integer','null'],minimum:0},minimumEventModel:{enum:['CE-V1','CE-V2-shadow','CE-V2-read','CE-V2-only']},requiredCapabilities:capabilityArray,minimumSchemaVersion:{type:'string',minLength:1},minimumBackendJobManifestGeneration:positiveUint,previousWatermarkDigest:nullableDigest,issuedAt:timestamp,keySetVersion:positiveUint
  }),
  'deployment-attestation.schema.json': null,
  'restore-recovery-attestation.schema.json': null,
  'signature-envelope.schema.json': schema('signature-envelope.v1', 'joincare.release.detached-signature-envelope.v1', ['domain','payloadDomain','payloadSha256','keySetVersion','signatures'], {
    payloadDomain:{enum:['joincare.release.image-capability.v1','joincare.release.environment-bootstrap.v1','joincare.release.key-set.v1','joincare.release.key-revocation.v1','joincare.release.fair-launch-watermark.v1','joincare.release.deployment-attestation.v1','joincare.restore-recovery-attestation.v1']},payloadSha256:digest,keySetVersion:positiveUint,signatures:{type:'array',minItems:2,maxItems:3,uniqueItems:true,items:{type:'object',additionalProperties:false,required:['keyId','algorithm','signatureBase64'],properties:{keyId:{type:'string',pattern:'^[a-z0-9-]{1,32}$'},algorithm:{const:'Ed25519'},signatureBase64:{type:'string',pattern:'^[A-Za-z0-9+/]{86}==$'}}}}
  })
};
const roots = ['ceIdentityRoot','cePlacementRoot','ceApplicationRoot','outboxRoot','gatewayJournalRoot','eoaNonceRoot','safeRoot','timelockRoot','executorRoot','financialProjectRoot','financialWalletRoot','financialContractTokenRoot'];
schemaFiles['deployment-attestation.schema.json'] = schema('deployment-attestation.v1', 'joincare.release.deployment-attestation.v1', ['domain','environmentId','chainId','proxyAddress','implementationCodeHash','sourceRegistryRevision','component','manifestGeneration','manifestDigest','artifactSha256','containerImageId','deployedAt','agentHostId','previousDeploymentAttestationDigest','keySetVersion'], {
  environmentId:{type:'string',pattern:'^[a-z0-9-]{3,64}$'},chainId:uint,proxyAddress:address,implementationCodeHash:digest,sourceRegistryRevision:positiveUint,component:{const:'backend-job'},manifestGeneration:positiveUint,manifestDigest:digest,artifactSha256:digest,containerImageId:{type:'string',pattern:'^sha256:[0-9a-f]{64}$'},deployedAt:timestamp,agentHostId:{type:'string',minLength:1},previousDeploymentAttestationDigest:nullableDigest,keySetVersion:positiveUint
});
schemaFiles['restore-recovery-attestation.schema.json'] = schema('restore-recovery-attestation.v1', 'joincare.restore-recovery-attestation.v1', ['domain','restoredSnapshotId','snapshotCreatedAt','bootstrapDigest','effectiveWatermarkDigest','deploymentAttestationDigest','replayFromCheckpoint','replayThroughBlock','replayThroughBlockHash',...roots,'result','initiatorId','approverIds','executorId','completedAt'], {
  restoredSnapshotId:{type:'string',minLength:1},snapshotCreatedAt:timestamp,bootstrapDigest:digest,effectiveWatermarkDigest:digest,deploymentAttestationDigest:digest,replayFromCheckpoint:uint,replayThroughBlock:uint,replayThroughBlockHash:ref('blockHash'),...Object.fromEntries(roots.map(name=>[name,digest])),result:{const:'BALANCED'},initiatorId:{type:'string',minLength:1},approverIds:{type:'array',minItems:1,uniqueItems:true,items:{type:'string',minLength:1}},executorId:{type:'string',minLength:1},completedAt:timestamp
});
for (const [name, value] of Object.entries(schemaFiles)) fs.writeFileSync(path.join(schemas, name), `${JSON.stringify(value, null, 2)}\n`);

const artifact = Buffer.from('joincare fair launch detached runtime artifact fixture v1\n');
fs.writeFileSync(path.join(detached, 'artifact.bin'), artifact);
const artifactHash = sha(artifact);
fs.writeFileSync(path.join(detached, 'artifact.sha256'), `${artifactHash}  artifact.bin\n`);
const capabilities = ['fair-launch.claim-refund-projection','fair-launch.e8-payment-token','fair-launch.external-end-convergence','fair-launch.governance-workflows','fair-launch.ingestion','fair-launch.materialization','fair-launch.outbox-recovery','fair-launch.reconciliation','fair-launch.source-registry'];
const makeKeyPairs = label => ['key-a','key-b','key-c'].map((keyId,index) => { const seed=crypto.createHash('sha256').update(`joincare-fair-launch-${label}-${index}`).digest().subarray(0,32); const privateKey=crypto.createPrivateKey({key:Buffer.concat([Buffer.from('302e020100300506032b657004220420','hex'),seed]),format:'der',type:'pkcs8'}); return {keyId,publicKey:crypto.createPublicKey(privateKey),privateKey}; });
const keyPairsV1=makeKeyPairs('v1'),keyPairsV2=makeKeyPairs('v2');
const publicKeyBase64 = key => key.export({format:'der',type:'spki'}).subarray(-32).toString('base64');
const records = {
'image-capability':{domain:'joincare.release.image-capability.v1',environmentId:'joincare-mainnet',generation:1,issuedAt:'2026-09-06T00:00:00Z',expiresAt:'2026-12-06T00:00:00Z',keySetVersion:1,component:'backend-job',artifactSha256:artifactHash,containerImageId:`sha256:${hex('1')}`,sourceCommit:'2'.repeat(40),schemaMin:'2026_09_fair_expand',schemaMax:'2026_09_fair_v2_constraints',eventModel:'CE-V2-only',capabilities,maintenanceModes:['signer-recovery','write-disabled-replay'],previousManifestDigest:null},
'key-set':{domain:'joincare.release.key-set.v1',keySetVersion:1,threshold:2,keys:keyPairsV1.map(x=>({keyId:x.keyId,algorithm:'Ed25519',publicKeyBase64:publicKeyBase64(x.publicKey)})),validFromGeneration:1,validThroughGeneration:100,supersedesKeySetVersion:null,previousKeySetDigest:null,revokedAtGeneration:null,revocationReason:null,issuedAt:'2026-09-06T00:00:00Z',signaturesByPreviousKeySet:[]},
'environment-bootstrap':{domain:'joincare.release.environment-bootstrap.v1',environmentId:'joincare-mainnet',chainId:56,fairLaunchProxyAddress:'0x1111111111111111111111111111111111111111',minimumDiscoveryBlock:100,releaseControlPlaneId:'joincare-release-mainnet',pinnedHttpsUrl:'https://release.joincarelabs.com',tlsSpkiSha256:hex('3'),watermarkRootKeySetDigest:hex('4'),sourceRegistryAuthorityKind:'offchain-signed-registry',sourceRegistrySignerKeySetDigest:hex('5'),hostStateVolumeId:'joincare-mainnet-host-01',enrollmentGeneration:1,issuedAt:'2026-09-06T00:00:00Z',expiresAt:null,previousBootstrapDigest:null},
'key-revocation':{domain:'joincare.release.key-revocation.v1',keySetVersion:1,revokedKeyIds:['key-c'],effectiveGeneration:50,reason:'deterministic emergency-revocation vector',issuedAt:'2026-09-06T01:00:00Z',previousRevocationDigest:null},
'fair-launch-watermark':{domain:'joincare.release.fair-launch-watermark.v1',generation:1,environmentId:'joincare-mainnet',chainId:56,proxyAddress:'0x1111111111111111111111111111111111111111',implementationCodeHash:hex('6'),sourceRegistryRevision:1,firstEnabledBlock:100,firstProjectCreatedBlock:null,minimumEventModel:'CE-V2-only',requiredCapabilities:capabilities,minimumSchemaVersion:'2026_09_fair_v2_constraints',minimumBackendJobManifestGeneration:1,previousWatermarkDigest:null,issuedAt:'2026-09-06T00:00:00Z',keySetVersion:1},
'deployment-attestation':{domain:'joincare.release.deployment-attestation.v1',environmentId:'joincare-mainnet',chainId:56,proxyAddress:'0x1111111111111111111111111111111111111111',implementationCodeHash:hex('6'),sourceRegistryRevision:1,component:'backend-job',manifestGeneration:1,manifestDigest:hex('a'),artifactSha256:artifactHash,containerImageId:`sha256:${hex('1')}`,deployedAt:'2026-09-06T01:30:00Z',agentHostId:'joincare-mainnet-host-01',previousDeploymentAttestationDigest:null,keySetVersion:1},
'restore-recovery-attestation':{domain:'joincare.restore-recovery-attestation.v1',restoredSnapshotId:'snapshot-2026-09-06-001',snapshotCreatedAt:'2026-09-06T00:00:00Z',bootstrapDigest:hex('7'),effectiveWatermarkDigest:hex('8'),deploymentAttestationDigest:hex('9'),replayFromCheckpoint:100,replayThroughBlock:200,replayThroughBlockHash:`0x${hex('a')}`,ceIdentityRoot:hex('b'),cePlacementRoot:hex('c'),ceApplicationRoot:hex('d'),outboxRoot:hex('e'),gatewayJournalRoot:hex('f'),eoaNonceRoot:hex('0'),safeRoot:hex('1'),timelockRoot:hex('2'),executorRoot:hex('3'),financialProjectRoot:hex('4'),financialWalletRoot:hex('5'),financialContractTokenRoot:hex('6'),result:'BALANCED',initiatorId:'admin-init',approverIds:['admin-approve'],executorId:'admin-execute',completedAt:'2026-09-06T02:00:00Z'}};
const keySetV1Canonical=canonicalize(records['key-set']);
records['environment-bootstrap'].watermarkRootKeySetDigest=sha(Buffer.from(keySetV1Canonical));records['environment-bootstrap'].sourceRegistrySignerKeySetDigest=sha(Buffer.from(keySetV1Canonical));records['deployment-attestation'].manifestDigest=sha(Buffer.from(canonicalize(records['image-capability'])));records['restore-recovery-attestation'].bootstrapDigest=sha(Buffer.from(canonicalize(records['environment-bootstrap'])));records['restore-recovery-attestation'].effectiveWatermarkDigest=sha(Buffer.from(canonicalize(records['fair-launch-watermark'])));records['restore-recovery-attestation'].deploymentAttestationDigest=sha(Buffer.from(canonicalize(records['deployment-attestation'])));
for(const [name,record]of Object.entries(records)){const canonical=canonicalize(record);fs.writeFileSync(path.join(vectors,`${name}.input.json`),`${JSON.stringify(record,null,2)}\n`);fs.writeFileSync(path.join(vectors,`${name}.canonical.json`),canonical);fs.writeFileSync(path.join(vectors,`${name}.sha256`),`${sha(Buffer.from(canonical))}  ${name}.canonical.json\n`)}
for(const name of Object.keys(records)){const payloadBytes=fs.readFileSync(path.join(vectors,`${name}.canonical.json`));const envelope={domain:'joincare.release.detached-signature-envelope.v1',payloadDomain:records[name].domain,payloadSha256:sha(payloadBytes),keySetVersion:1,signatures:keyPairsV1.slice(0,2).map(x=>({keyId:x.keyId,algorithm:'Ed25519',signatureBase64:crypto.sign(null,payloadBytes,x.privateKey).toString('base64')}))};const envelopeCanonical=canonicalize(envelope),envelopeName=name==='image-capability'?'signature-envelope':`${name}.signature-envelope`;fs.writeFileSync(path.join(vectors,`${envelopeName}.input.json`),`${JSON.stringify(envelope,null,2)}\n`);fs.writeFileSync(path.join(vectors,`${envelopeName}.canonical.json`),envelopeCanonical);fs.writeFileSync(path.join(vectors,`${envelopeName}.sha256`),`${sha(Buffer.from(envelopeCanonical))}  ${envelopeName}.canonical.json\n`)}
const keySetV1Digest=sha(Buffer.from(canonicalize(records['key-set']))),keySetV2={domain:'joincare.release.key-set.v1',keySetVersion:2,threshold:2,keys:keyPairsV2.map(x=>({keyId:x.keyId,algorithm:'Ed25519',publicKeyBase64:publicKeyBase64(x.publicKey)})),validFromGeneration:101,validThroughGeneration:200,supersedesKeySetVersion:1,previousKeySetDigest:keySetV1Digest,revokedAtGeneration:null,revocationReason:null,issuedAt:'2026-10-01T00:00:00Z',signaturesByPreviousKeySet:[]};
const keySetV2SigningBytes=Buffer.from(canonicalize(keySetV2));keySetV2.signaturesByPreviousKeySet=keyPairsV1.slice(0,2).map(x=>({keyId:x.keyId,algorithm:'Ed25519',signatureBase64:crypto.sign(null,keySetV2SigningBytes,x.privateKey).toString('base64')}));
const commonIdentity={environmentId:'joincare-mainnet',chainId:56,proxyAddress:'0x1111111111111111111111111111111111111111',implementationCodeHash:hex('6')};
const transitions = {
  schemaVersion: 2,
  keyRotation: {
    previousKeySet: records['key-set'], nextKeySet: keySetV2,
    signingPayloadSha256: sha(keySetV2SigningBytes), expected: {accepted:true},
    negative: [
      {id:'wrong-predecessor-digest',mutation:'previousKeySetDigest',value:hex('f'),accepted:false},
      {id:'insufficient-predecessor-signatures',mutation:'dropSignature',accepted:false},
      {id:'wrong-predecessor-signer',mutation:'unknownSigner',accepted:false},
      {id:'tampered-new-key-set',mutation:'validThroughGeneration',value:201,accepted:false},
      {id:'new-key-before-valid-from',manifestGeneration:100,accepted:false},
      {id:'old-key-after-valid-through',manifestGeneration:101,accepted:false}
    ]
  },
  revocation: [
    {id:'revoked-signer-after-effective-generation',keyId:'key-c',manifestGeneration:50,accepted:false},
    {id:'unrevoked-threshold-before-effective-generation',keyIds:['key-a','key-b'],manifestGeneration:49,accepted:true}
  ],
  previousDigest: [
    {id:'manifest-generation-two-requires-link',previousGeneration:1,nextGeneration:2,previousDigest:records['deployment-attestation'].manifestDigest,nextPreviousDigest:records['deployment-attestation'].manifestDigest,accepted:true},
    {id:'manifest-rollback-rejected',previousGeneration:2,nextGeneration:1,previousDigest:hex('a'),nextPreviousDigest:hex('a'),accepted:false},
    {id:'same-generation-different-digest-rejected',previousGeneration:2,nextGeneration:2,previousDigest:hex('a'),nextPreviousDigest:hex('b'),accepted:false}
  ],
  typedJoin: [
    {id:'capability-union-and-max',left:{...commonIdentity,sourceRegistryRevision:1,generation:2,eventModel:'CE-V2-shadow',schemaVersion:'2026_09_fair_expand',capabilities:['fair-launch.ingestion']},right:{...commonIdentity,sourceRegistryRevision:2,generation:3,eventModel:'CE-V2-only',schemaVersion:'2026_09_fair_v2_constraints',capabilities:['fair-launch.reconciliation']},expected:{accepted:true,generation:3,eventModel:'CE-V2-only',schemaVersion:'2026_09_fair_v2_constraints',implementationCodeHash:hex('6'),sourceRegistryRevision:2,capabilities:['fair-launch.ingestion','fair-launch.reconciliation']}},
    {id:'proxy-identity-conflict-fail-closed',conflictField:'proxyAddress',left:{...commonIdentity,sourceRegistryRevision:1},right:{...commonIdentity,proxyAddress:'0x2222222222222222222222222222222222222222',sourceRegistryRevision:1},expected:{accepted:false,reason:'identity-conflict:proxyAddress'}},
    {id:'implementation-conflict-fail-closed',conflictField:'implementationCodeHash',left:{...commonIdentity,sourceRegistryRevision:2},right:{...commonIdentity,implementationCodeHash:hex('7'),sourceRegistryRevision:2},expected:{accepted:false,reason:'identity-conflict:implementationCodeHash'}},
    {id:'source-revision-regression-fail-closed',conflictField:'sourceRegistryRevision',left:{...commonIdentity,sourceRegistryRevision:3},right:{...commonIdentity,sourceRegistryRevision:2},expected:{accepted:false,reason:'source-registry-regression'}}
  ],
  schemaOrder:['2026_09_fair_expand','2026_09_fair_v2_constraints'],
  eventOrder:['CE-V1','CE-V2-shadow','CE-V2-read','CE-V2-only']
};
fs.writeFileSync(path.join(vectors,'trust-transition-vectors.json'),`${JSON.stringify(transitions,null,2)}\n`);
console.log('generated Fair Launch RFC 8785 evidence');
