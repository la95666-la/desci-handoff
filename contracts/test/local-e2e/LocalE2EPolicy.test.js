'use strict';
const { expect } = require('chai');
const childProcess=require('node:child_process');
const crypto=require('node:crypto');
const fs=require('node:fs');
const os=require('node:os');
const path=require('node:path');
const local=require('../../scripts/local/fairLaunchLocalE2E');
const fixture=require('./unadmittedFixtures');
const node=process.execPath;
function clone(v){return JSON.parse(JSON.stringify(v));}
function snapshot(root){if(!fs.existsSync(root))return null;const stat=fs.lstatSync(root);if(!stat.isDirectory())return[['.',crypto.createHash('sha256').update(fs.readFileSync(root)).digest('hex')]];const out=[];function visit(d){for(const e of fs.readdirSync(d,{withFileTypes:true}).sort((a,b)=>a.name.localeCompare(b.name))){const f=path.join(d,e.name),r=path.relative(root,f).replaceAll('\\','/');if(e.isDirectory()){out.push([`${r}/`]);visit(f);}else out.push([r,crypto.createHash('sha256').update(fs.readFileSync(f)).digest('hex')]);}}visit(root);return out;}
function commandFailure(command,args=[],env={}){try{childProcess.execFileSync(command,args,{env,encoding:'utf8',stdio:['ignore','pipe','pipe'],cwd:local.REPOSITORY_ROOT});throw Error('unexpected success');}catch(e){return{status:e.status,stdout:String(e.stdout||''),stderr:String(e.stderr||'')}}}
function child(script,args=[],env={},input){try{const stdout=childProcess.execFileSync(node,[script,...args],{env,encoding:'utf8',input,stdio:['pipe','pipe','pipe'],cwd:local.REPOSITORY_ROOT});return{status:0,stdout,stderr:''};}catch(e){return{status:e.status,stdout:String(e.stdout||''),stderr:String(e.stderr||'')}}}
function validate(type,payload,context={}){return local.validateStructuralPayload(payload,type,{nowMs:fixture.nowMs,...context});}
function bundle(){
  const chainRoot=fixture.root('chain'),dbRoot=fixture.root('database');
  const policyValue=fixture.policy({chainTrust:{trustRootSha256:chainRoot.digest,generation:7},databaseTrust:{trustRootSha256:dbRoot.digest,generation:7}});
  const policyResult=validate('policy',policyValue),chainValue=fixture.chain(policyValue),chainResult=validate('chain',chainValue,{policyResult});
  return{chainRoot,dbRoot,policyValue,policyResult,chainValue,chainResult,frontendValue:fixture.frontend(policyValue,chainValue),databaseValue:fixture.database(policyValue,chainValue)};
}
function signedEnvelope(type,payload,root){return fixture.envelope(type,payload,root);}
function verifyBundle(type,b,overrides={}){
  const payload={chain:b.chainValue,frontend:b.frontendValue,database:b.databaseValue}[type];
  const root=type==='database'?b.dbRoot:b.chainRoot,built=signedEnvelope(type,payload,root);
  return local.verifyStructuralEnvelope(built.envelope,{type,runId:fixture.runId,generation:7,trustRootSha256:root.digest,expectedPayloadSha256:local.sha256(local.canonicalJson(payload)),nowMs:fixture.nowMs,publicRoot:root.value,policyResult:b.policyResult,chainResult:b.chainResult,...overrides});
}
function removeAt(payload,pathParts){const x=clone(payload);let n=x;for(const k of pathParts.slice(0,-1))n=n[k];delete n[pathParts.at(-1)];return x;}
function unknownAt(payload,pathParts){const x=clone(payload);let n=x;for(const k of pathParts)n=n[k];n.unexpected=true;return x;}

function expectChainMutation(mutator, message){const b=bundle(),value=clone(b.chainValue);mutator(value);expect(()=>validate('chain',value,{policyResult:b.policyResult}),message).to.throw();}
function expectChainError(mutator,expectedMessage){const b=bundle(),value=clone(b.chainValue);mutator(value);let error;try{validate('chain',value,{policyResult:b.policyResult});}catch(caught){error=caught;}expect(error,expectedMessage).to.be.instanceOf(Error);expect(error.message).eq(expectedMessage);}
function creationEvent(value,index=0){return value.fixtureTransactions.find(tx=>tx.label===`create-${value.projects[index].key}`).events[0];}
function replaceCreationLog(value,argIndex,replacement,projectIndex=0){const event=creationEvent(value,projectIndex),values=event.decodedArgs.map(x=>x.value);values[argIndex]=replacement;const rebuilt=fixture.rawEvent('FairLaunchProjectCreated',values,event.txLogOrdinal,{projectKey:event.projectKey});rebuilt.emitter=value.contracts.proxy.address;Object.assign(event,rebuilt);}
function replaceActivatedLog(value,replacement){const tx=value.fixtureTransactions.find(x=>x.label==='observation'),event=tx.events[0],rebuilt=fixture.rawEvent('FairLaunchActivated',[replacement],event.txLogOrdinal);rebuilt.emitter=value.contracts.proxy.address;tx.events[0]=rebuilt;}
function moveCreation(value,{blockNumber,blockHash,blockTimestamp,transactionIndex},projectIndex=0){const p=value.projects[projectIndex],tx=value.fixtureTransactions.find(x=>x.txHash===p.creationWitness.transactionHash);Object.assign(tx,{blockNumber,blockHash,blockTimestamp,transactionIndex});Object.assign(p.creationWitness,{blockNumber,blockHash,blockTimestamp,transactionIndex});}

describe('Fair Launch seventh-round scope-reduced LOCAL-ONLY boundary',function(){
  it('fails all eleven unavailable commands directly and through package scripts without side effects',function(){
    const temp=fs.mkdtempSync(path.join(os.tmpdir(),'jc-fl-scope-')),run=path.join(temp,'run');fs.mkdirSync(run);fs.writeFileSync(path.join(temp,'sentinel'),'unchanged');
    const observed=[temp,path.join(local.REPOSITORY_ROOT,'artifacts'),path.join(local.REPOSITORY_ROOT,'cache'),path.join(local.REPOSITORY_ROOT,'.local-e2e'),path.join(local.REPOSITORY_ROOT,'artifacts-local-e2e'),path.join(local.REPOSITORY_ROOT,'cache-local-e2e'),path.join(local.REPOSITORY_ROOT,'scripts/validation/fair-launch-events.generated.json')],before=observed.map(snapshot);
    const entries=[['createFairLaunchLocalE2EAuthority.js','unavailable:fair-launch:signer-custody','fair_launch_local_e2e_signer_custody_unavailable'],['cleanupFairLaunchLocalE2ESignerCeremony.js','unavailable:fair-launch:signer-residue-cleanup','fair_launch_local_e2e_signer_custody_unavailable'],['assembleFairLaunchLocalE2ETrustRoot.js','unavailable:fair-launch:root-assembly','fair_launch_local_e2e_policy_admission_unavailable'],['signFairLaunchLocalE2EPayload.js','unavailable:fair-launch:artifact-signing','fair_launch_local_e2e_policy_admission_unavailable'],['combineFairLaunchLocalE2EArtifact.js','unavailable:fair-launch:artifact-combining','fair_launch_local_e2e_policy_admission_unavailable'],['runFairLaunchLocalE2ELifecycles.js','unavailable:fair-launch:persistent-lifecycle','fair_launch_local_e2e_process_ownership_unavailable']];
    try{for(const[file,command,error]of entries){for(const result of [child(path.join(local.REPOSITORY_ROOT,'scripts/local',file),['bad',temp],{PRIVATE_KEY:'forbidden',JC_FL_LOCAL_E2E_RUN_ROOT:run}),commandFailure(process.platform==='win32'?process.env.ComSpec:'npm',process.platform==='win32'?['/d','/s','/c','npm','--silent','run',command,'--','bad',temp]:['--silent','run',command,'--','bad',temp],{...process.env,PRIVATE_KEY:'forbidden',JC_FL_LOCAL_E2E_RUN_ROOT:run})]){expect(result.status,file).eq(1);expect(result.stdout,file).eq('');expect(result.stderr,file).eq(`${error}\n`);expect(observed.map(snapshot),file).deep.eq(before);}}}finally{fs.rmSync(temp,{recursive:true,force:true});}
  });

  it('keeps opaque validation capabilities immutable and unforgeable',function(){
    const policyValue=fixture.policy(),policyResult=validate('policy',policyValue),policyDigest=local.sha256(local.canonicalJson(policyValue));
    expect(policyResult).deep.eq({status:local.UNADMITTED_STATUS,type:'policy'});expect(Object.isFrozen(policyResult)).true;
    expect(()=>{policyResult.type='chain';}).to.throw(TypeError);expect(()=>{policyResult.payloadSha256='0'.repeat(64);}).to.throw(TypeError);expect(policyResult).deep.eq({status:local.UNADMITTED_STATUS,type:'policy'});
    policyValue.allowedOrigins[0]='http://127.0.0.1:9999/';policyValue.chainTrust.generation=99;policyValue.artifactSchemas.chain.signatureDomain='bad';
    const originalChain=fixture.chain(fixture.policy());expect(validate('chain',originalChain,{policyResult}).status).eq(local.UNADMITTED_STATUS);
    const immutablePolicy=fixture.policy(),immutablePolicyResult=validate('policy',immutablePolicy),immutableChain=fixture.chain(immutablePolicy);immutablePolicy.allowedOrigins[0]='http://127.0.0.1:9999/';immutablePolicy.chainTrust.trustRootSha256='f'.repeat(64);expect(validate('chain',immutableChain,{policyResult:immutablePolicyResult}).status).eq(local.UNADMITTED_STATUS);const immutableChainResult=validate('chain',immutableChain,{policyResult:immutablePolicyResult}),immutableFront=fixture.frontend(fixture.policy(),immutableChain);expect(validate('frontend',immutableFront,{policyResult:immutablePolicyResult,chainResult:immutableChainResult}).status).eq(local.UNADMITTED_STATUS);
    for(const forged of [{...policyResult},Object.assign(Object.create(policyResult),{}),new Proxy(policyResult,{})])expect(()=>validate('chain',originalChain,{policyResult:forged})).to.throw('validated policy result required');
    const chainValue=fixture.chain(fixture.policy()),chainResult=validate('chain',chainValue,{policyResult:validate('policy',fixture.policy())}),front=fixture.frontend();chainValue.network.chainId='56';chainValue.projects[1].projectId='999';chainValue.deployment.observationBlockHash='0x'+'f'.repeat(64);expect(validate('frontend',front,{policyResult:validate('policy',fixture.policy()),chainResult}).status).eq(local.UNADMITTED_STATUS);
    const custom=Object.create({});Object.assign(custom,fixture.policy());expect(()=>validate('policy',custom)).to.throw('prototype');const accessor=fixture.policy();Object.defineProperty(accessor,'runId',{enumerable:true,get(){return fixture.runId;}});expect(()=>validate('policy',accessor)).to.throw('accessor');const nestedAccessor=fixture.policy();Object.defineProperty(nestedAccessor.chainTrust,'generation',{enumerable:true,get(){return 7;}});expect(()=>validate('policy',nestedAccessor)).to.throw('accessor');
    const cycle=fixture.policy();cycle.chainTrust.self=cycle;expect(()=>validate('policy',cycle)).to.throw('cycle');const symbol=fixture.policy();symbol.chainTrust[Symbol('hidden')]=1;expect(()=>validate('policy',symbol)).to.throw('symbol');const sparse=fixture.policy();sparse.allowedOrigins=new Array(2);sparse.allowedOrigins[0]='http://127.0.0.1:4173/';expect(()=>validate('policy',sparse)).to.throw('sparse');const hidden=fixture.policy();Object.defineProperty(hidden.chainTrust,'hidden',{value:true,enumerable:false});expect(()=>validate('policy',hidden)).to.throw('accessor');const nestedPrototype=fixture.policy();nestedPrototype.chainTrust=Object.assign(Object.create({inherited:true}),nestedPrototype.chainTrust);expect(()=>validate('policy',nestedPrototype)).to.throw('prototype');
    expect(policyDigest).eq(local.sha256(local.canonicalJson(fixture.policy())));
  });

  it('propagates material policy constraints into chain, frontend, and database verification',function(){
    const b=bundle();expect(verifyBundle('chain',b).status).eq(local.UNADMITTED_STATUS);expect(verifyBundle('frontend',b).status).eq(local.UNADMITTED_STATUS);expect(verifyBundle('database',b).status).eq(local.UNADMITTED_STATUS);
    const immutable=bundle();immutable.policyValue.allowedOrigins[0]='http://127.0.0.1:9999/';immutable.policyValue.chainTrust.trustRootSha256='f'.repeat(64);immutable.policyValue.chainTrust.generation=99;expect(verifyBundle('chain',immutable).status).eq(local.UNADMITTED_STATUS);expect(verifyBundle('frontend',immutable).status).eq(local.UNADMITTED_STATUS);
    const unauthorized={...b,frontendValue:clone(b.frontendValue)};unauthorized.frontendValue.frontendOrigin='http://127.0.0.1:9999/';const signed=signedEnvelope('frontend',unauthorized.frontendValue,b.chainRoot);expect(()=>local.verifyStructuralEnvelope(signed.envelope,{type:'frontend',runId:fixture.runId,generation:7,trustRootSha256:b.chainRoot.digest,expectedPayloadSha256:local.sha256(local.canonicalJson(unauthorized.frontendValue)),nowMs:fixture.nowMs,publicRoot:b.chainRoot.value,policyResult:b.policyResult,chainResult:b.chainResult})).to.throw('not allowed');
    const otherChain=fixture.root('chain');expect(()=>verifyBundle('chain',b,{trustRootSha256:otherChain.digest,publicRoot:otherChain.value})).to.throw('policy trust projection');expect(()=>verifyBundle('chain',b,{generation:8})).to.throw('policy trust projection');const otherDb=fixture.root('database');expect(()=>verifyBundle('database',b,{trustRootSha256:otherDb.digest,publicRoot:otherDb.value})).to.throw('policy trust projection');expect(()=>verifyBundle('database',b,{generation:8})).to.throw('policy trust projection');
    const changedPolicy=fixture.policy({artifactSchemas:{chain:{schemaVersion:local.SCHEMAS.chain,status:local.UNADMITTED_STATUS,signatureDomain:'wrong'},frontend:{schemaVersion:local.SCHEMAS.frontend,status:local.UNADMITTED_STATUS,signatureDomain:local.DOMAINS.frontend},database:{schemaVersion:local.SCHEMAS.database,status:local.UNADMITTED_STATUS,signatureDomain:local.DOMAINS.database}}});expect(()=>validate('policy',changedPolicy)).to.throw('artifact schema identity');const badSchema=clone(b.chainValue);badSchema.schemaVersion='wrong';expect(()=>validate('chain',badSchema,{policyResult:b.policyResult})).to.throw(/header|schema/);const badDomain=clone(b.databaseValue);badDomain.signatureDomain=local.DOMAINS.frontend;expect(()=>validate('database',badDomain,{policyResult:b.policyResult,chainResult:b.chainResult})).to.throw(/header|schema/);
  });

  it('rejects duplicate public material and repeated same-key signatures',function(){
    const b=bundle(),built=signedEnvelope('chain',b.chainValue,b.chainRoot),duplicate=clone(b.chainRoot.value);duplicate.keys[1]=clone(duplicate.keys[0]);expect(()=>local.verifyStructuralEnvelope(built.envelope,{type:'chain',runId:fixture.runId,generation:7,trustRootSha256:local.sha256(local.canonicalJson(duplicate)),expectedPayloadSha256:local.sha256(local.canonicalJson(b.chainValue)),nowMs:fixture.nowMs,publicRoot:duplicate,policyResult:b.policyResult})).to.throw(/duplicates|projection/);
    built.envelope.signatures=[built.envelope.signatures[0],clone(built.envelope.signatures[0])];expect(()=>local.verifyStructuralEnvelope(built.envelope,{type:'chain',runId:fixture.runId,generation:7,trustRootSha256:b.chainRoot.digest,expectedPayloadSha256:local.sha256(local.canonicalJson(b.chainValue)),nowMs:fixture.nowMs,publicRoot:b.chainRoot.value,policyResult:b.policyResult})).to.throw('threshold');
  });

  it('mirrors static createProject validity and positive exact boundaries',function(){
    const zero='0x'+'0'.repeat(40),zeroHash='0x'+'0'.repeat(64);
    const cases=[
      ['zero id',p=>p.projectId='0'],['project token',p=>p.projectToken=zero],['payment token',p=>p.paymentToken=zero],['same tokens',p=>p.projectToken=p.paymentToken],['owner',p=>p.projectOwner=zero],['proceeds',p=>p.projectProceedsReceiver=zero],['fee receiver',p=>p.expectedPlatformFeeReceiver=zero],
      ['sale zero',p=>p.saleTokenAmountRaw='0'],['soft cap zero',p=>p.softCapRaw='0'],['minimum zero',p=>p.minContributionRaw='0'],['bad max',p=>p.maxContributionRaw=String(BigInt(p.minContributionRaw)-1n)],['fee 1001',p=>p.expectedPlatformFeeBps=1001],['fee uint16',p=>p.expectedPlatformFeeBps=65536],
      ['proceeds attestation uint32',p=>p.projectProceedsReceiverAttestationVersion=4294967296],['fee attestation uint32',p=>p.expectedPlatformFeeReceiverAttestationVersion=4294967296],['project policy uint32',p=>p.expectedProjectTokenPolicyVersion=4294967296],['payment policy uint32',p=>p.expectedPaymentTokenPolicyVersion=4294967296],['release policy uint32',p=>p.expectedReleasePolicyVersion=4294967296],
      ['attestation zero',p=>p.projectProceedsReceiverAttestationVersion=0],['fee attestation zero',p=>p.expectedPlatformFeeReceiverAttestationVersion=0],['project policy zero',p=>p.expectedProjectTokenPolicyVersion=0],['payment policy zero',p=>p.expectedPaymentTokenPolicyVersion=0],['release policy zero',p=>p.expectedReleasePolicyVersion=0],
      ['proceeds digest zero',p=>p.projectProceedsReceiverAttestationDigest=zeroHash],['fee digest zero',p=>p.expectedPlatformFeeReceiverAttestationDigest=zeroHash],['project policy digest zero',p=>p.expectedProjectTokenPolicyDigest=zeroHash],['payment policy digest zero',p=>p.expectedPaymentTokenPolicyDigest=zeroHash],['release digest zero',p=>p.expectedReleasePolicyDigest=zeroHash],
      ['end before',p=>p.endTime=p.startTime],['start creation',p=>p.startTime=p.creationWitness.blockTimestamp],['wrong initial',p=>p.initialStatus='active'],
    ];
    for(const[label,mutate]of cases)expectChainMutation(v=>mutate(v.projects[0]),label);
    expectChainMutation(v=>{v.assets.paymentToken.decimals=78;},'payment token decimals');expectChainMutation(v=>{v.assets.projectTokens[0].decimals=78;},'project token decimals');
    const boundary=bundle(),p=boundary.chainValue.projects[0];boundary.chainValue.safes.platformFeeBps=1000;boundary.chainValue.safes.proceeds.attestationVersion=4294967295;boundary.chainValue.safes.platformFee.attestationVersion=4294967295;boundary.chainValue.assets.paymentToken.policyVersion=4294967295;boundary.chainValue.assets.releasePolicy.version=4294967295;boundary.chainValue.assets.projectTokens[0].policyVersion=4294967295;for(const project of boundary.chainValue.projects){project.expectedPlatformFeeBps=1000;project.projectProceedsReceiverAttestationVersion=4294967295;project.expectedPlatformFeeReceiverAttestationVersion=4294967295;project.expectedPaymentTokenPolicyVersion=4294967295;project.expectedReleasePolicyVersion=4294967295;}p.maxContributionRaw=p.minContributionRaw;p.expectedProjectTokenPolicyVersion=4294967295;p.startTime=String(BigInt(p.creationWitness.blockTimestamp)+1n);fixture.refreshCreationEvents(boundary.chainValue);expect(validate('chain',boundary.chainValue,{policyResult:boundary.policyResult}).status).eq(local.UNADMITTED_STATUS);
  });

  it('requires five exact creation witnesses tied to fixture events and blocks',function(){
    const mutations=[v=>v.projects[0].creationWitness.transactionHash='0x'+'f'.repeat(64),v=>v.projects[0].creationWitness.blockNumber='99',v=>v.projects[0].creationWitness.blockHash='0x'+'f'.repeat(64),v=>v.projects[0].creationWitness.blockTimestamp='1',v=>v.projects[0].creationWitness.transactionIndex=9,v=>v.projects[0].creationWitness.txLogOrdinal=9,v=>v.projects[0].creationWitness.projectId='999',v=>v.projects[0].creationWitness.projectKey='successful',v=>creationEvent(v).projectKey='successful',v=>v.fixtureTransactions.splice(1,1)];
    for(const mutate of mutations)expectChainMutation(mutate,'creation witness mismatch');
  });

  it('proves exact proxy-emitted ABI logs and all 23 creation arguments',function(){
    const wrongHash='0x'+'f'.repeat(64),wrongAddress='0x'+'f'.repeat(40);
    const evidenceMutations=[
      ['emitter',v=>creationEvent(v).emitter=wrongAddress],['signature',v=>creationEvent(v).signature='FairLaunchProjectCreated(uint256)'],['topic0 field',v=>creationEvent(v).topic0=wrongHash],['raw topic0',v=>creationEvent(v).topics[0]=wrongHash],['indexed topic',v=>creationEvent(v).topics[1]=wrongHash],['missing topic',v=>creationEvent(v).topics.pop()],['raw data',v=>{const e=creationEvent(v);e.data=`${e.data.slice(0,-2)}${e.data.endsWith('00')?'01':'00'}`;}],['missing data',v=>delete creationEvent(v).data],['missing decoded argument',v=>creationEvent(v).decodedArgs.pop()],['extra decoded argument',v=>creationEvent(v).decodedArgs.push(clone(creationEvent(v).decodedArgs[0]))],['reordered decoded arguments',v=>{const a=creationEvent(v).decodedArgs;[a[0],a[1]]=[a[1],a[0]];}],['decoded name',v=>creationEvent(v).decodedArgs[0].name='wrong'],['decoded type',v=>creationEvent(v).decodedArgs[0].type='uint64'],
      ['activated missing projectId',v=>v.fixtureTransactions.find(x=>x.label==='observation').events[0].decodedArgs.pop()],['activated altered projectId',v=>replaceActivatedLog(v,v.projects[0].projectId)],
    ];
    for(const[label,mutate]of evidenceMutations)expectChainMutation(mutate,label);
    const row=fixture.eventDefinition('FairLaunchProjectCreated');expect(row.inputs).to.have.length(23);
    for(const input of row.inputs){expectChainMutation(v=>{const current=creationEvent(v).decodedArgs[input.ordinal].value;const replacement=input.type==='address'?(current===wrongAddress?'0x'+'e'.repeat(40):wrongAddress):input.type==='bytes32'?(current===wrongHash?'0x'+'e'.repeat(64):wrongHash):String(BigInt(current)+1n);replaceCreationLog(v,input.ordinal,replacement);},`creation argument ${input.ordinal} ${input.name}`);}
  });

  it('requires exact proxy, creation, and observation transaction placement',function(){
    const Hf='0x'+'f'.repeat(64);
    const mutations=[
      ['pre-deployment creation',v=>moveCreation(v,{blockNumber:'9',blockHash:Hf,blockTimestamp:'1799998990',transactionIndex:1})],
      ['creation at deployment placement',v=>moveCreation(v,{blockNumber:v.deployment.proxyBlockNumber,blockHash:v.deployment.proxyBlockHash,blockTimestamp:v.deployment.proxyBlockTimestamp,transactionIndex:v.deployment.proxyTransactionIndex})],
      ['creation at observation placement',v=>moveCreation(v,{blockNumber:v.deployment.observationBlockNumber,blockHash:v.deployment.observationBlockHash,blockTimestamp:v.deployment.observationBlockTimestamp,transactionIndex:v.deployment.observationTransactionIndex})],
      ['creation after observation placement',v=>moveCreation(v,{blockNumber:v.deployment.observationBlockNumber,blockHash:v.deployment.observationBlockHash,blockTimestamp:v.deployment.observationBlockTimestamp,transactionIndex:v.deployment.observationTransactionIndex+1})],
      ['same-block post-observation transaction',v=>v.fixtureTransactions.push({label:'post-observation',txHash:Hf,blockNumber:v.deployment.observationBlockNumber,blockHash:v.deployment.observationBlockHash,blockTimestamp:v.deployment.observationBlockTimestamp,transactionIndex:v.deployment.observationTransactionIndex+1,events:[]})],
      ['missing observation and identity',v=>{const tx=v.fixtureTransactions.find(x=>x.label==='observation');v.fixtureTransactions=v.fixtureTransactions.filter(x=>x!==tx);v.fixtureEventIdentities=v.fixtureEventIdentities.filter(x=>x.txHash!==tx.txHash);}],
      ['observation tx hash',v=>v.fixtureTransactions.find(x=>x.label==='observation').txHash=Hf],['observation block hash',v=>v.fixtureTransactions.find(x=>x.label==='observation').blockHash=Hf],['observation timestamp',v=>v.fixtureTransactions.find(x=>x.label==='observation').blockTimestamp='1799999999'],['observation index',v=>v.fixtureTransactions.find(x=>x.label==='observation').transactionIndex=2],
      ['proxy timestamp',v=>v.fixtureTransactions.find(x=>x.label==='proxy-deploy').blockTimestamp='1799998999'],['proxy index',v=>v.fixtureTransactions.find(x=>x.label==='proxy-deploy').transactionIndex=1],
    ];
    for(const[label,mutate]of mutations)expectChainMutation(mutate,label);
  });

  it('enforces distinct-block timestamp chronology and block-hash uniqueness',function(){
    const chronology='transaction block timestamp chronology invalid',hashReuse='transaction block hash reused';
    expectChainError(v=>{const p=v.projects[0],tx=v.fixtureTransactions.find(x=>x.txHash===p.creationWitness.transactionHash);tx.blockTimestamp='1799998999';p.creationWitness.blockTimestamp=tx.blockTimestamp;},chronology);
    expectChainError(v=>{const p=v.projects[0],tx=v.fixtureTransactions.find(x=>x.txHash===p.creationWitness.transactionHash);tx.blockTimestamp='1800000001';p.creationWitness.blockTimestamp=tx.blockTimestamp;p.startTime='1800000002';p.endTime='1800003602';p.vestingTimes=['1800003602'];fixture.refreshCreationEvents(v);},chronology);
    expectChainError(v=>{v.deployment.proxyBlockTimestamp='1800000001';v.fixtureTransactions.find(x=>x.label==='proxy-deploy').blockTimestamp=v.deployment.proxyBlockTimestamp;},chronology);
    expectChainError(v=>{v.deployment.observationBlockTimestamp='1799999049';v.fixtureTransactions.find(x=>x.label==='observation').blockTimestamp=v.deployment.observationBlockTimestamp;},chronology);
    expectChainError(v=>{const p=v.projects[0],tx=v.fixtureTransactions.find(x=>x.txHash===p.creationWitness.transactionHash);tx.blockHash=v.deployment.proxyBlockHash;p.creationWitness.blockHash=tx.blockHash;},hashReuse);
    expectChainError(v=>{const first=v.projects[0],second=v.projects[1],firstTx=v.fixtureTransactions.find(x=>x.txHash===first.creationWitness.transactionHash),secondTx=v.fixtureTransactions.find(x=>x.txHash===second.creationWitness.transactionHash);secondTx.blockHash=firstTx.blockHash;second.creationWitness.blockHash=secondTx.blockHash;},hashReuse);
    const b=bundle(),existing=b.chainValue.fixtureTransactions.find(x=>x.label==='create-failed');b.chainValue.fixtureTransactions.push({label:'empty-same-block',txHash:'0x'+'e'.repeat(64),blockNumber:existing.blockNumber,blockHash:existing.blockHash,blockTimestamp:existing.blockTimestamp,transactionIndex:existing.transactionIndex+1,events:[]});expect(validate('chain',b.chainValue,{policyResult:b.policyResult}).status).eq(local.UNADMITTED_STATUS);
  });

  it('canonicalizes numeric and string block numbers for identity and placement',function(){
    const duplicatePlacement='transaction placements duplicates',identityMismatch='transaction block identity mismatch';
    expectChainError(v=>{const existing=v.fixtureTransactions.find(x=>x.label==='create-failed');v.fixtureTransactions.push({label:'numeric-conflicting-block',txHash:'0x'+'d'.repeat(64),blockNumber:11,blockHash:'0x'+'c'.repeat(64),blockTimestamp:existing.blockTimestamp,transactionIndex:existing.transactionIndex+1,events:[]});},identityMismatch);
    expectChainError(v=>{const existing=v.fixtureTransactions.find(x=>x.label==='create-failed');v.fixtureTransactions.push({label:'numeric-duplicate-placement',txHash:'0x'+'d'.repeat(64),blockNumber:11,blockHash:existing.blockHash,blockTimestamp:existing.blockTimestamp,transactionIndex:existing.transactionIndex,events:[]});},duplicatePlacement);
    const b=bundle(),existing=b.chainValue.fixtureTransactions.find(x=>x.label==='create-failed');b.chainValue.fixtureTransactions.push({label:'numeric-same-block',txHash:'0x'+'d'.repeat(64),blockNumber:11,blockHash:existing.blockHash,blockTimestamp:existing.blockTimestamp,transactionIndex:existing.transactionIndex+1,events:[]});expect(validate('chain',b.chainValue,{policyResult:b.policyResult}).status).eq(local.UNADMITTED_STATUS);
  });

  it('rejects unsafe numeric uint inputs while preserving safe numbers and exact decimal strings',function(){
    const unsafe=Number.MAX_SAFE_INTEGER+1;expect(Number.isSafeInteger(unsafe)).false;expect(9007199254740993).eq(unsafe);expect(9007199254740993).eq(9007199254740992);
    expectChainError(v=>{v.fixtureTransactions.push({label:'unsafe-numeric-block',txHash:'0x'+'a'.repeat(64),blockNumber:unsafe,blockHash:'0x'+'b'.repeat(64),blockTimestamp:'1800000001',transactionIndex:7,events:[]});},'transaction block invalid');
    for(const [label,value] of [['fractional',11.5],['negative',-1]])expectChainError(v=>{v.fixtureTransactions.push({label:`${label}-numeric-block`,txHash:'0x'+'a'.repeat(64),blockNumber:value,blockHash:'0x'+'b'.repeat(64),blockTimestamp:'1800000001',transactionIndex:7,events:[]});},'transaction block invalid');
    const b=bundle(),p=b.chainValue.projects[0];p.maxContributionRaw=0;fixture.refreshCreationEvents(b.chainValue);expect(validate('chain',b.chainValue,{policyResult:b.policyResult}).status).eq(local.UNADMITTED_STATUS);expect(p.saleTokenAmountRaw).eq('1000000000000000000000');expect(BigInt(p.saleTokenAmountRaw)).greaterThan(BigInt(Number.MAX_SAFE_INTEGER));
  });

  it('accepts and rejects exact uint64 project and vesting time boundaries',function(){
    const max=(1n<<64n)-1n,b=bundle(),p=b.chainValue.projects[0];p.startTime=String(max-1n);p.endTime=String(max);p.vestingTimes=[String(max)];fixture.refreshCreationEvents(b.chainValue);expect(validate('chain',b.chainValue,{policyResult:b.policyResult}).status).eq(local.UNADMITTED_STATUS);
    for(const [label,mutate] of [['start overflow',p=>p.startTime=String(max+1n)],['end overflow',p=>p.endTime=String(max+1n)],['vesting overflow',p=>p.vestingTimes[0]=String(max+1n)]])expectChainMutation(v=>mutate(v.projects[0]),label);
  });

  it('preserves recursive schemas, event bijection, time containment, and DB linkage',function(){
    const b=bundle();expect(validate('database',b.databaseValue,{policyResult:b.policyResult,chainResult:b.chainResult}).status).eq(local.UNADMITTED_STATUS);
    b.chainValue.deployment.observationTxHash='0x'+'f'.repeat(64);b.chainValue.deployment.observationBlockHash='0x'+'e'.repeat(64);b.chainValue.deployment.observationBlockTimestamp='1';b.chainValue.deployment.observationTransactionIndex=99;expect(validate('database',b.databaseValue,{policyResult:b.policyResult,chainResult:b.chainResult}).status).eq(local.UNADMITTED_STATUS);
    const same=clone(b.databaseValue);same.captureTip.blockNumber=same.anchor.blockNumber;expect(()=>validate('database',same,{policyResult:b.policyResult,chainResult:b.chainResult})).to.throw('chronology');const fresh=bundle(),ev=clone(fresh.chainValue);ev.fixtureEventIdentities.pop();expect(()=>validate('chain',ev,{policyResult:fresh.policyResult})).to.throw('identity set');
    for(const[type,payload,context]of [['policy',fixture.policy(),{}],['chain',b.chainValue,{policyResult:b.policyResult}],['frontend',b.frontendValue,{policyResult:b.policyResult,chainResult:b.chainResult}],['database',b.databaseValue,{policyResult:b.policyResult,chainResult:b.chainResult}]]){expect(()=>validate(type,unknownAt(payload,[]),context)).to.throw();expect(()=>validate(type,removeAt(payload,['runId']),context)).to.throw();}
    const expired=clone(b.frontendValue);expired.expiresAt=new Date(fixture.nowMs).toISOString();expect(()=>validate('frontend',expired,{policyResult:b.policyResult,chainResult:b.chainResult})).to.throw('expired');
  });

  it('child-invokes retired Round1 and sanitizer and keeps normal scripts closed',function(){
    const retired=child(path.join(local.REPOSITORY_ROOT,'scripts/local/runFairLaunchRound1.js'));expect(retired.status).eq(1);expect(retired.stdout).eq('');expect(retired.stderr).include('retired');const p=fixture.pair(),script=path.join(local.REPOSITORY_ROOT,'scripts/local/sanitizeFairLaunchLocalE2EPublicDescriptor.js');const ok=child(script,[],{},JSON.stringify(p.descriptor));expect(ok.status).eq(0);expect(JSON.parse(ok.stdout)).deep.eq({status:local.UNADMITTED_STATUS,descriptor:p.descriptor});
    const normal=['createFairLaunchLocalE2EAuthority.js','cleanupFairLaunchLocalE2ESignerCeremony.js','assembleFairLaunchLocalE2ETrustRoot.js','signFairLaunchLocalE2EPayload.js','combineFairLaunchLocalE2EArtifact.js','runFairLaunchLocalE2ELifecycles.js'];for(const f of normal){const source=fs.readFileSync(path.join(local.REPOSITORY_ROOT,'scripts/local',f),'utf8');expect(source.split(/\r?\n/).filter(Boolean)).to.have.length(3);expect(source).not.match(/require\(|process\.argv|process\.env|readFile|writeFile|rmSync|unlink|mkdir/);}expect(fs.readFileSync(path.join(local.REPOSITORY_ROOT,'hardhat.config.js'),'utf8')).not.include('contracts-local-e2e');
  });
});
