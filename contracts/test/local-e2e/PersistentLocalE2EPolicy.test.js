'use strict';

const { expect } = require('chai');
const childProcess = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const local = require('../../scripts/local/fairLaunchPersistentLocalE2E');
const { windowsArgumentList } = require('../../scripts/local/startFairLaunchLocalE2E');

function invoke(script, args, env = {}) {
  return childProcess.spawnSync(process.execPath, [path.join(local.REPOSITORY_ROOT, 'scripts/local', script), ...args], { cwd: local.REPOSITORY_ROOT, env: { ...process.env, ...env }, encoding: 'utf8', timeout: 10000 });
}
function authorityFixture() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'jc-fl-authority-'));
  if (process.platform !== 'win32') fs.chmodSync(root, 0o700);
  const runId = 'run-test-12345678', rpcUrl = 'http://127.0.0.1:23456', paths = local.runPaths(runId, root);
  local.ensurePrivateDirectory(paths.runDir, true);
  local.writeOnceJson(paths.rootAuthority, local.rootAuthority(runId, rpcUrl, root));
  const processRecord = { schemaVersion: local.PROCESS_SCHEMA, authority: local.AUTHORITY, runId, rpcUrl, chainId: 31337, repositoryRoot: fs.realpathSync(local.REPOSITORY_ROOT), runDirectory: fs.realpathSync(paths.runDir), supervisor: local.processSnapshot(process.ppid), node: local.processSnapshot(process.pid), createdAt: new Date().toISOString() };
  local.writeOnceJson(paths.processAuthority, processRecord);
  const pair = crypto.generateKeyPairSync('ed25519'), genesisHash = `0x${'a'.repeat(64)}`;
  const trust = local.createPublicTrustAuthority(paths, runId, rpcUrl, genesisHash, pair);
  const pinPath = path.join(root, '..', `${path.basename(root)}-external-pin.json`);
  const { pinSha256 } = local.createExternalTrustPin(paths, pinPath, runId, rpcUrl, genesisHash, trust);
  return { root, runId, rpcUrl, paths, pair, trust, genesisHash, pinPath, pinSha256 };
}
function addr(n) { return `0x${n.toString(16).padStart(40, '0')}`; }
function hash(n) { return `0x${n.toString(16).slice(-1).repeat(64)}`; }
function publishManifest(f, envelope) {
  fs.rmSync(f.paths.manifest,{force:true});
  return local.createCanonicalJsonOnce(f.paths.manifest,envelope).sha256;
}
function payload(f, now = Date.now()) {
  let i = 1; const contracts = {};
  for (const key of ['proxy','dispatcher','coreFacet','releaseFacet','fundsFacet','adminFacet','viewsFacet','logicLibrary','runtimeLibrary']) contracts[key] = { address: addr(i), runtimeCodeHash: hash((i++ % 15) + 1) };
  const payment = { symbol: 'tUSDT', decimals: 6, address: addr(i++), runtimeCodeHash: hash(1) };
  const projectTokens = ['failed','successful','cancelled'].map((key) => ({ key, symbol: {failed:'BSFAIL',successful:'BSPASS',cancelled:'BSCAN'}[key], decimals:18, address:addr(i++), runtimeCodeHash:hash(2) }));
  const proceeds=addr(i++), platformFee=addr(i++);
  const roles={governance:addr(i++),operator:addr(i++),pauseGuardian:addr(i++),unpauseTimelock:addr(i++),treasury:addr(i++),browserSeedAccount:addr(i++)};
  const receipt = (n,from,to,contractAddress=null) => ({ txHash: hash(n), blockNumber: String(n), blockHash: hash(n + 5), transactionIndex: 0, blockTimestamp: '1000', from, to, contractAddress, valueRaw:'0', inputHash:hash(15) });
  const zeroHash=`0x${'0'.repeat(64)}`;
  const projects = projectTokens.map((t, n) => {
    const projectId={failed:'101',successful:'102',cancelled:'103'}[t.key], owner=addr(i++), status=1, terminal=0;
    const row={key:t.key,projectId,projectToken:t.address,paymentToken:payment.address,projectOwner:owner,projectProceedsReceiver:proceeds,platformFeeReceiver:platformFee,platformFeeBps:250,startTime:'1000',endTime:'44200',saleTokenAmountRaw:'1000000000000000000000000',softCapRaw:t.key==='failed'?'100000000000':'100000000',minContributionRaw:'1000000',maxContributionRaw:'0',totalTokensDepositedRaw:'1000000000000000000000000',creation:receipt(n+2,roles.operator,contracts.proxy.address),creationEventLogIndex:n,initialStatus:'active',statusCode:status,terminalReasonCode:terminal};
    row.pristineStorage={projectId,projectToken:t.address,paymentToken:payment.address,projectOwner:owner,projectProceedsReceiver:proceeds,projectProceedsReceiverAttestationDigest:hash(30),projectProceedsReceiverAttestationVersion:'1',platformFeeReceiver:platformFee,platformFeeReceiverAttestationDigest:hash(31),platformFeeReceiverAttestationVersion:'1',projectTokenDecimals:'18',paymentTokenDecimals:'6',platformFeeBps:'250',status:String(status),terminalReason:String(terminal),projectTokenPolicyVersion:'1',paymentTokenPolicyVersion:'1',startTime:'1000',endTime:'44200',saleTokenAmount:row.saleTokenAmountRaw,softCap:row.softCapRaw,minContribution:'1000000',maxContribution:'0',totalRaised:'0',totalTokensDeposited:row.totalTokensDepositedRaw,totalTokensClaimed:'0',participantCount:'0',finalTotalRaised:'0',finalPriceNumerator:'0',finalPriceDenominator:'0',settlementBlockNumber:'0',settlementAnchorBlockNumber:'0',settlementAnchorBlockHash:zeroHash,settlementTxHash:zeroHash,settlementTxLogOrdinal:'0',releaseManifestDigest:zeroHash,projectTokenCodeHash:t.runtimeCodeHash,projectTokenPolicyDigest:hash(32+n),paymentTokenCodeHash:payment.runtimeCodeHash,paymentTokenPolicyDigest:hash(36),releasePolicyDigest:hash(37),releasePolicyVersion:'1',projectProceedsWithdrawn:false,platformFeeWithdrawn:false,failedProjectTokensWithdrawn:false};
    return row;
  });
  const observation=receipt(30,roles.browserSeedAccount,roles.browserSeedAccount); observation.inputHash='0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470';
  return { schemaVersion:local.MANIFEST_SCHEMA, authority:local.AUTHORITY, runId:f.runId, createdAt:new Date(now+1000).toISOString(), expiresAt:new Date(now+61000).toISOString(), network:{name:'hardhat-local-persistent',chainId:31337,rpcUrl:f.rpcUrl,genesisHash:f.genesisHash}, contracts, assets:{paymentToken:payment,projectTokens,faucet:{strategy:'unrestricted-token-mint',nativeGasStrategy:'frontend-local-hardhat_setBalance',storesPrivateKeys:false}}, safes:{proceeds:{address:proceeds,runtimeCodeHash:hash(3),threshold:2},platformFee:{address:platformFee,runtimeCodeHash:hash(3),threshold:2},owners:[addr(i++),addr(i++),addr(i++)]}, projects, roles, deployment:{proxy:receipt(1,roles.governance,null,contracts.proxy.address),observation}, source:{commit:'a'.repeat(40),sourceTreeDigest:hash(10),aggregateAbiDigest:hash(11),selectorMapDigest:hash(12),storageLayoutDigest:hash(13),fullSetDigest:hash(14),dirty:true} };
}

describe('Fair Launch persistent strict LOCAL-ONLY policy', function () {
  it('accepts only exact IPv4 loopback URLs and original absolute canonical pin paths', function () {
    expect(local.exactRpcUrl('http://127.0.0.1:19141')).deep.equal({ url:'http://127.0.0.1:19141', port:19141 });
    for (const value of ['http://localhost:1','http://[::1]:1','http://0.0.0.0:1','http://127.0.0.1']) expect(() => local.exactRpcUrl(value)).to.throw();
    const f=authorityFixture(); try { for(const value of ['relative.json',path.join('..','pin.json'),`${f.root}${path.sep}.${path.sep}pin.json`]) expect(()=>local.exactExternalPinPath(value,f.paths)).to.throw(); } finally { fs.rmSync(f.root,{recursive:true,force:true});fs.rmSync(f.pinPath,{force:true}); }
  });
  it('rejects Windows remote, device, ADS and ambiguous pin path syntax', function () {
    for(const value of ['\\\\server\\share\\pin.json','\\\\?\\C:\\pin.json','\\\\.\\C:\\pin.json','C:relative.json','C:\\pin.json:ads','C:\\dir.\\pin.json','C:\\dir \\pin.json','C:/pin.json']) expect(()=>local.validateExternalPinPathSyntax(value,'win32')).to.throw();
    expect(local.validateExternalPinPathSyntax('C:\\local\\pin.json','win32')).eq('C:\\local\\pin.json');
  });
  it('rejects production and key environment before side effects', function () {
    for (const env of [{JC_DEPLOY_RPC:'x'},{PRIVATE_KEY:'x'},{WALLET_MNEMONIC:'x'}]) expect(() => local.rejectForbiddenEnvironment(env)).to.throw('forbidden');
  });
  it('rejects duplicate JSON keys at every depth', function () {
    for (const text of ['{"a":1,"a":2}','{"a":{"b":1,"b":2}}','{"a":[{"b":1,"b":2}]}']) expect(() => local.parseJsonNoDuplicateKeys(text)).to.throw('duplicate');
    expect(local.parseJsonNoDuplicateKeys('{"a":[{"b":1}]}')).deep.eq({a:[{b:1}]});
  });
  it('anchors signatures to create-once external trust and rejects counterfeit/cross-links', async function () {
    const f = authorityFixture(), now = Date.now();
    try {
      const good = local.signManifestPayload(payload(f, now), f.pair.privateKey, f.trust); const goodDigest=publishManifest(f,good);
      expect((await local.strictRuntimeAdmission(f.paths,f.pinPath,f.pinSha256,goodDigest,f.runId,f.rpcUrl,{genesisHash:f.genesisHash},{verifyCode:false,verifyLive:false,nowMs:now+2000})).keyId).eq(f.trust.keyId);
      const counterfeitPair = crypto.generateKeyPairSync('ed25519');
      const counterfeitTrust = { keyId:`local-e2e-${local.sha256(counterfeitPair.publicKey.export({type:'spki',format:'der'})).slice(0,24)}`, publicKeyJwk:counterfeitPair.publicKey.export({format:'jwk'}) };
      const counterfeit = local.signManifestPayload(payload(f,now),counterfeitPair.privateKey,counterfeitTrust);
      await expect(local.strictRuntimeAdmission(f.paths,f.pinPath,f.pinSha256,publishManifest(f,counterfeit),f.runId,f.rpcUrl,{genesisHash:f.genesisHash},{verifyCode:false,verifyLive:false,nowMs:now+2000})).to.be.rejected;
      for (const mutate of [e=>e.payload.runId='wrong-run-12345678',e=>e.payload.network.rpcUrl='http://127.0.0.1:23457',e=>e.payload.network.genesisHash=hash(4),e=>e.payload.projects[0].paymentToken=addr(999),e=>e.payload.assets.projectTokens[0].key='successful',e=>e.payload.safes.proceeds.threshold=1,e=>e.payload.expiresAt=new Date(now+25*3600000).toISOString()]) {
        const changed=payload(f,now); mutate({payload:changed}); const signed=local.signManifestPayload(changed,f.pair.privateKey,f.trust);
        await expect(local.strictRuntimeAdmission(f.paths,f.pinPath,f.pinSha256,publishManifest(f,signed),f.runId,f.rpcUrl,{genesisHash:f.genesisHash},{verifyCode:false,verifyLive:false,nowMs:now+2000})).to.be.rejected;
      }
    } finally { fs.rmSync(f.root,{recursive:true,force:true}); fs.rmSync(f.pinPath,{force:true}); }
  });
  it('makes external trust/pin create-once and rejects replacement or absent explicit pin', async function () {
    const f=authorityFixture(), now=Date.now();
    try {
      expect(()=>local.createPublicTrustAuthority(f.paths,f.runId,f.rpcUrl,f.genesisHash,f.pair)).to.throw();
      expect(()=>local.createExternalTrustPin(f.paths,f.pinPath,f.runId,f.rpcUrl,f.genesisHash,f.trust)).to.throw();
      const good=local.signManifestPayload(payload(f,now),f.pair.privateKey,f.trust); const goodDigest=publishManifest(f,good);
      await expect(local.strictRuntimeAdmission(f.paths,undefined,f.pinSha256,goodDigest,f.runId,f.rpcUrl,{genesisHash:f.genesisHash},{verifyCode:false,verifyLive:false,nowMs:now+2000})).to.be.rejected;
      for (const digest of [undefined,'0'.repeat(63),'A'.repeat(64),'f'.repeat(64)]) await expect(local.strictRuntimeAdmission(f.paths,f.pinPath,digest,goodDigest,f.runId,f.rpcUrl,{genesisHash:f.genesisHash},{verifyCode:false,verifyLive:false,nowMs:now+2000})).to.be.rejected;
      const oldPin=fs.readFileSync(f.pinPath,'utf8'); fs.unlinkSync(f.pinPath); fs.writeFileSync(f.pinPath,oldPin.replace(f.trust.keyId,'local-e2e-000000000000000000000000')); if(process.platform!=='win32')fs.chmodSync(f.pinPath,0o600);
      await expect(local.strictRuntimeAdmission(f.paths,f.pinPath,f.pinSha256,goodDigest,f.runId,f.rpcUrl,{genesisHash:f.genesisHash},{verifyCode:false,verifyLive:false,nowMs:now+2000})).to.be.rejected;
      fs.writeFileSync(f.pinPath,oldPin); if(process.platform!=='win32')fs.chmodSync(f.pinPath,0o600);
      const replacementPair=crypto.generateKeyPairSync('ed25519');
      const replacementTrust=structuredClone(f.trust); replacementTrust.publicKeyJwk=replacementPair.publicKey.export({format:'jwk'}); replacementTrust.keyId=`local-e2e-${local.sha256(replacementPair.publicKey.export({type:'spki',format:'der'})).slice(0,24)}`;
      fs.unlinkSync(f.paths.trustRoot); fs.writeFileSync(f.paths.trustRoot,`${JSON.stringify(replacementTrust,null,2)}\n`); if(process.platform!=='win32')fs.chmodSync(f.paths.trustRoot,0o600);
      const replacementPin={schemaVersion:local.PIN_SCHEMA,authority:local.AUTHORITY,runId:f.runId,rpcUrl:f.rpcUrl,chainId:31337,genesisHash:f.genesisHash,keyId:replacementTrust.keyId,trustRecordSha256:local.sha256(local.canonicalJson(replacementTrust)),publicKeySha256:local.sha256(local.canonicalJson(replacementTrust.publicKeyJwk)),trustCreatedAt:replacementTrust.createdAt,createdAt:new Date().toISOString()};
      fs.unlinkSync(f.pinPath); fs.writeFileSync(f.pinPath,`${local.canonicalJson(replacementPin)}\n`); if(process.platform!=='win32')fs.chmodSync(f.pinPath,0o600);
      const replacementManifest=local.signManifestPayload(payload(f,now),replacementPair.privateKey,replacementTrust);
      await expect(local.strictRuntimeAdmission(f.paths,f.pinPath,f.pinSha256,publishManifest(f,replacementManifest),f.runId,f.rpcUrl,{genesisHash:f.genesisHash},{verifyCode:false,verifyLive:false,nowMs:now+2000})).to.be.rejected;
    } finally { fs.rmSync(f.root,{recursive:true,force:true}); fs.rmSync(f.pinPath,{force:true}); }
  });
  it('rejects manifest digest/byte/identity replacement races', async function () {
    const f=authorityFixture(),now=Date.now();
    try { const envelope=local.signManifestPayload(payload(f,now),f.pair.privateKey,f.trust),digest=publishManifest(f,envelope);
      expect(()=>local.stableCanonicalJsonSnapshot(f.paths.manifest,'A'.repeat(64))).to.throw();
      const semantic=JSON.stringify(envelope,null,2)+'\n';fs.writeFileSync(f.paths.manifest,semantic);if(process.platform!=='win32')fs.chmodSync(f.paths.manifest,0o600);expect(()=>local.stableCanonicalJsonSnapshot(f.paths.manifest,local.sha256(semantic))).to.throw('canonical');
      publishManifest(f,envelope);expect(()=>local.stableCanonicalJsonSnapshot(f.paths.manifest,digest,()=>{fs.unlinkSync(f.paths.manifest);local.createCanonicalJsonOnce(f.paths.manifest,envelope);})).to.throw();
      publishManifest(f,envelope);const admitted=await local.strictRuntimeAdmission(f.paths,f.pinPath,f.pinSha256,digest,f.runId,f.rpcUrl,{genesisHash:f.genesisHash},{verifyCode:false,verifyLive:false,nowMs:now+2000});
      expect(()=>local.assertAdmittedManifestCurrent(f.paths.manifest,admitted.manifestSnapshot,()=>{fs.unlinkSync(f.paths.manifest);local.createCanonicalJsonOnce(f.paths.manifest,envelope);})).to.throw('identity');
    } finally { fs.rmSync(f.root,{recursive:true,force:true});fs.rmSync(f.pinPath,{force:true}); }
  });
  it('uses TCP closure rather than JSON-RPC response validity', async function () {
    const net=require('node:net'); const server=net.createServer((socket)=>{socket.on('error',()=>{});socket.write('malformed');}); await new Promise((r)=>server.listen(0,'127.0.0.1',r));
    const url=`http://127.0.0.1:${server.address().port}`;
    try { expect((await local.tcpProbe(url)).status).eq('open'); await expect(local.waitForClosed(url,500)).to.be.rejected; } finally { await new Promise((r)=>server.close(r)); }
    expect(await local.waitForClosed(url,1000)).true;
    await expect(local.waitForClosed(url,500,async()=>({status:'ambiguous',code:'TIMEOUT'}))).to.be.rejected;
  });
  it('quotes Windows native argv deterministically', function () {
    expect(windowsArgumentList(['plain','with space','ends with slash\\','quote"inside','space slash\\'])).eq('plain "with space" "ends with slash\\\\" "quote\\"inside" "space slash\\\\"');
  });
  it('requires exact process identity and preserves production closure', function () {
    const snapshot=local.processSnapshot(process.pid); expect(local.sameProcess(snapshot,local.processSnapshot(process.pid))).true; expect(local.sameProcess(snapshot,{...snapshot,startIdentity:'bad'})).false;
    expect(fs.readFileSync(path.join(local.REPOSITORY_ROOT,'hardhat.config.js'),'utf8')).not.include('contracts-local-e2e');
    const retired=invoke('runFairLaunchRound1.js',[]); expect(retired.status).eq(1);
  });
  it('loads in-process config without RPC authority but rejects absent/invalid external network', function () {
    const bootstrap=path.join(local.REPOSITORY_ROOT,'node_modules/hardhat/internal/cli/bootstrap.js'), env={...process.env}; delete env.JC_FAIR_LAUNCH_LOCAL_E2E_RPC_URL;
    expect(childProcess.spawnSync(process.execPath,[bootstrap,'help','--config',path.join(local.REPOSITORY_ROOT,'hardhat.local-e2e.config.js')],{cwd:local.REPOSITORY_ROOT,env}).status).eq(0);
    expect(childProcess.spawnSync(process.execPath,[bootstrap,'run',__filename,'--config',path.join(local.REPOSITORY_ROOT,'hardhat.local-e2e.config.js'),'--network','localE2E','--no-compile'],{cwd:local.REPOSITORY_ROOT,env}).status).not.eq(0);
    for(const value of ['', 'http://localhost:1']) expect(childProcess.spawnSync(process.execPath,[bootstrap,'help','--config',path.join(local.REPOSITORY_ROOT,'hardhat.local-e2e.config.js')],{cwd:local.REPOSITORY_ROOT,env:{...env,JC_FAIR_LAUNCH_LOCAL_E2E_RPC_URL:value}}).status).not.eq(0);
  });
});
