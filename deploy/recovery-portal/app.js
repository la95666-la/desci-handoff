(() => {
  'use strict';
  const EXPECTED_DOMAIN='joincare.recovery-portal.deployment-manifest.v1';
  const ANCHOR_DOMAIN='joincare.recovery-portal.external-anchor.v1';
  const ENVELOPE_DOMAIN='joincare.release.detached-signature-envelope.v1';
  const REVIEWER_ROOT={domain:'joincare.recovery-portal.reviewer-root.v1',keySetVersion:1,keys:[{algorithm:'Ed25519',keyId:'reviewer-a',publicKeyBase64:'0EqyMnQrtKs6E2i9RhXk5tAiSrcaAWuvhSCjMsl3hzc='},{algorithm:'Ed25519',keyId:'reviewer-b',publicKeyBase64:'oJql9HpnWYAv+VX43C0qFKXJnSO+l/hkEn/5ODRVpPA='},{algorithm:'Ed25519',keyId:'reviewer-c',publicKeyBase64:'F8t5+ytBIPKx7GXkGY1uCLKOgT/rAeSkAIObheGAgM4='}],revokedAtGeneration:null,revokedKeyIds:[],threshold:2,validFromGeneration:1,validThroughGeneration:Number.MAX_SAFE_INTEGER};
  const REVIEWER_ROOT_SHA256='50b66bae76d8433e6cbe3c404ea2d37506ef3089275ed227514f8232fed864a6';
  // Fixture keys exercise verification only. Production packaging remains disabled
  // until an independently reviewed immutable root replaces this explicit marker.
  const REVIEWER_ROOT_STATUS='UNADMITTED_PLACEHOLDER';
  // Immutable trust checkpoint. Fixture values are deliberately unadmitted; a
  // production build must replace them under independent review as one bundle.
  const CHECKPOINT={environmentId:'mainnet',manifestDigest:'7558720e7be25894159c22cba1b1d374e5beecb71413af9475bc26bfb405728f',manifestGeneration:6,anchorDigest:'471c74cf87d66591c2c18edf2a710effd70254cb6cea81cc2ec24722e0449761',anchorGeneration:8};
  const EXTERNAL_ANCHOR_ORIGIN='https://recovery-anchor.joincarelabs.com';
  const te=new TextEncoder();
  const hex=bytes=>Array.from(bytes,b=>b.toString(16).padStart(2,'0')).join('');
  const validString=s=>{for(let i=0;i<s.length;i++){const c=s.charCodeAt(i);if(c>=0xd800&&c<=0xdbff){const n=s.charCodeAt(++i);if(!(n>=0xdc00&&n<=0xdfff))throw Error('invalid surrogate')}else if(c>=0xdc00&&c<=0xdfff)throw Error('invalid surrogate')}};
  const canonicalize=v=>{if(v===null)return'null';if(typeof v==='string'){validString(v);return JSON.stringify(v)}if(typeof v==='boolean')return String(v);if(typeof v==='number'){if(!Number.isSafeInteger(v))throw Error('unsafe/non-canonical number');return JSON.stringify(v)}if(Array.isArray(v))return`[${v.map(canonicalize).join(',')}]`;if(v&&typeof v==='object')return`{${Object.keys(v).sort().map(k=>`${JSON.stringify(k)}:${canonicalize(v[k])}`).join(',')}}`;throw Error('unsupported JSON value')};
  const sha256=async bytes=>hex(new Uint8Array(await crypto.subtle.digest('SHA-256',bytes)));
  const b64=s=>Uint8Array.from(atob(s),c=>c.charCodeAt(0));
  const parseStrict=(text,label)=>{const value=JSON.parse(text);if(canonicalize(value)!==text.trim())throw Error(`${label} is not RFC 8785 canonical`);return value};
  const assertTimeWindow=(record,now,label)=>{const issued=Date.parse(record.issuedAt),expires=Date.parse(record.expiresAt);if(!Number.isFinite(issued)||!Number.isFinite(expires)||issued>now||expires<=now||issued>=expires)throw Error(`${label} validity window rejected`)};
  async function verifyEnvelope(payloadText,envelope,keySet,expectedDomain,now){
    const payload=parseStrict(payloadText,expectedDomain); const bytes=te.encode(payloadText.trim());
    if(payload.domain!==expectedDomain)throw Error(`${expectedDomain} domain mismatch`);
    if(envelope.domain!==ENVELOPE_DOMAIN||envelope.payloadDomain!==expectedDomain)throw Error('signature envelope domain mismatch');
    if(await sha256(bytes)!==envelope.payloadSha256)throw Error('payload digest mismatch');
    if(envelope.keySetVersion!==keySet.keySetVersion||payload.keySetVersion!==keySet.keySetVersion)throw Error('key set version mismatch');
    if(!Number.isSafeInteger(payload.generation)||payload.generation<keySet.validFromGeneration||payload.generation>keySet.validThroughGeneration)throw Error('generation outside key-set validity');
    if(!Number.isSafeInteger(keySet.threshold)||keySet.threshold!==2||!Array.isArray(keySet.keys)||keySet.keys.length!==3)throw Error('trust root must be fixed 2-of-3');
    if(keySet.revokedAtGeneration!==null&&payload.generation>=keySet.revokedAtGeneration)throw Error('key set revoked for generation');
    assertTimeWindow(payload,now,expectedDomain);
    const revoked=new Set((keySet.revokedKeyIds||[])); const seen=new Set(); let valid=0;
    for(const sig of envelope.signatures||[]){const key=keySet.keys.find(k=>k.keyId===sig.keyId);if(!key||revoked.has(sig.keyId)||seen.has(sig.keyId)||sig.algorithm!=='Ed25519')continue;seen.add(sig.keyId);const publicKey=await crypto.subtle.importKey('raw',b64(key.publicKeyBase64),{name:'Ed25519'},false,['verify']);if(await crypto.subtle.verify('Ed25519',publicKey,b64(sig.signatureBase64),bytes))valid++}
    if(valid<2||valid<keySet.threshold)throw Error('2-of-3 Ed25519 threshold not met'); return {payload,digest:await sha256(bytes)};
  }
  async function verifyManifest(manifestText,envelope,keySet,externalAnchorText,externalAnchorEnvelope,now=Date.now(),history={}){
    if(canonicalize(keySet)!==canonicalize(REVIEWER_ROOT)||await sha256(te.encode(canonicalize(keySet)))!==REVIEWER_ROOT_SHA256)throw Error('reviewer trust root substitution rejected');
    const currentManifest=await verifyEnvelope(manifestText,envelope,keySet,EXPECTED_DOMAIN,now);
    const currentAnchor=await verifyEnvelope(externalAnchorText,externalAnchorEnvelope,keySet,ANCHOR_DOMAIN,now);
    if(!Array.isArray(history.manifests)||!Array.isArray(history.anchors))throw Error('complete signed history arrays required');
    const manifests=[currentManifest],anchors=[currentAnchor];
    for(const row of history.manifests)manifests.push(await verifyEnvelope(row.text,row.envelope,keySet,EXPECTED_DOMAIN,now));
    for(const row of history.anchors)anchors.push(await verifyEnvelope(row.text,row.envelope,keySet,ANCHOR_DOMAIN,now));
    if(manifests.length!==anchors.length)throw Error('manifest/anchor history length mismatch');
    for(let i=0;i<manifests.length;i++){
      const manifest=manifests[i],anchor=anchors[i],nextManifest=manifests[i+1],nextAnchor=anchors[i+1];
      if(!/^0x[0-9a-f]{40}$/.test(manifest.payload.proxyAddress)||manifest.payload.chainId!==56)throw Error('manifest chain/proxy identity invalid');
      if(manifest.payload.environmentId!==CHECKPOINT.environmentId||anchor.payload.environmentId!==CHECKPOINT.environmentId)throw Error('history environment mismatch');
      if(manifest.payload.generation<anchor.payload.minimumGeneration)throw Error('external anti-rollback minimum generation rejected');
      if(anchor.payload.acceptedManifestDigest!==manifest.digest)throw Error('external anchor does not accept manifest digest');
      const expectedManifestPredecessor=nextManifest?nextManifest.digest:null,expectedAnchorPredecessor=nextAnchor?nextAnchor.digest:null;
      if((manifest.payload.previousManifestDigest||null)!==expectedManifestPredecessor||(anchor.payload.previousManifestDigest||null)!==expectedManifestPredecessor)throw Error('manifest predecessor chain mismatch');
      if((anchor.payload.previousAnchorDigest||null)!==expectedAnchorPredecessor)throw Error('anchor predecessor chain mismatch');
      if(nextManifest&&nextManifest.payload.generation>=manifest.payload.generation)throw Error('manifest generation rollback rejected');
      if(nextAnchor&&nextAnchor.payload.generation>=anchor.payload.generation)throw Error('anchor generation rollback rejected');
    }
    const terminalManifest=manifests.at(-1),terminalAnchor=anchors.at(-1);
    if(terminalManifest.digest!==CHECKPOINT.manifestDigest||terminalManifest.payload.generation!==CHECKPOINT.manifestGeneration||terminalAnchor.digest!==CHECKPOINT.anchorDigest||terminalAnchor.payload.generation!==CHECKPOINT.anchorGeneration)throw Error('history does not terminate at immutable checkpoint');
    const seen=localStorage.getItem(`jc-recovery-generation:${currentManifest.payload.environmentId}`);if(seen&&currentManifest.payload.generation<Number(seen))throw Error('local anti-rollback watermark rejected');
    localStorage.setItem(`jc-recovery-generation:${currentManifest.payload.environmentId}`,String(currentManifest.payload.generation));
    return currentManifest.payload;
  }
  const projectId=()=>{const v=document.querySelector('#project-id').value.trim();if(!/^\d+$/.test(v)||v==='0')throw Error('project id must be a positive integer');return BigInt(v)};
  async function connect(){if(!window.ethereum)throw Error('No injected wallet. Use a wallet browser.');const [address]=await ethereum.request({method:'eth_requestAccounts'});const chain=await ethereum.request({method:'eth_chainId'});if(Number(BigInt(chain))!==state.manifest.chainId)throw Error('Wrong chain');state.account=address;document.querySelector('#wallet').textContent=address;document.querySelector('#actions').hidden=false}
  const selector={claim:'0x379607f5',refund:'0x278ecde1'};
  async function send(kind){if(!state.account)throw Error('connect wallet first');const id=projectId().toString(16).padStart(64,'0');const tx={from:state.account,to:state.manifest.proxyAddress,data:selector[kind]+id};const hash=await ethereum.request({method:'eth_sendTransaction',params:[tx]});document.querySelector('#tx').textContent=`Submitted ${hash}. Verify the receipt and canonical event before treating funds as recovered.`}
  const state={manifest:null,account:null};
  async function bootstrap(){
    try{
      if(REVIEWER_ROOT_STATUS!=='REVIEWER_ADMITTED')throw Error('reviewer trust root is not admitted');
      const manifestText=await (await fetch('./recovery-deployment-manifest.json',{cache:'no-store'})).text();
      const manifestSig=parseStrict(await (await fetch('./recovery-deployment-manifest.sig.json',{cache:'no-store'})).text(),'manifest envelope');
      const anchorUrl=new URL('/recovery-external-anchor.json',EXTERNAL_ANCHOR_ORIGIN),anchorSigUrl=new URL('/recovery-external-anchor.sig.json',EXTERNAL_ANCHOR_ORIGIN),historyUrl=new URL('/recovery-signed-history.json',EXTERNAL_ANCHOR_ORIGIN);
      for(const u of [anchorUrl,anchorSigUrl,historyUrl])if(u.origin!==EXTERNAL_ANCHOR_ORIGIN)throw Error('external anchor origin mismatch');
      const [anchorText,anchorSigText,historyText]=await Promise.all([anchorUrl,anchorSigUrl,historyUrl].map(async u=>{const r=await fetch(u.href,{cache:'no-store',credentials:'omit',referrerPolicy:'no-referrer'});if(!r.ok)throw Error(`missing external ${u.pathname}`);return r.text()}));
      const history=parseStrict(historyText,'signed history index');
      state.manifest=await verifyManifest(manifestText,manifestSig,REVIEWER_ROOT,anchorText,parseStrict(anchorSigText,'anchor envelope'),Date.now(),history);
      document.querySelector('#status').textContent=`Verified manifest generation ${state.manifest.generation} and signed external anchor. Wallet actions remain local.`;document.querySelector('#connect').disabled=false;
    }catch(e){document.querySelector('#status').textContent=`FAIL CLOSED: ${e.message}`;document.querySelector('#connect').disabled=true}
  }
  window.JCRecovery={canonicalize,verifyManifest,bootstrap};
  document.addEventListener('DOMContentLoaded',()=>{document.querySelector('#connect').onclick=()=>connect().catch(e=>alert(e.message));document.querySelector('#claim').onclick=()=>send('claim').catch(e=>alert(e.message));document.querySelector('#refund').onclick=()=>send('refund').catch(e=>alert(e.message));bootstrap()});
})();
