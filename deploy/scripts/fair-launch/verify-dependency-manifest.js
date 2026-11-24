'use strict';
const fs = require('fs');
const crypto = require('crypto');

function validString(s) {
  for (let i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i);
    if (c >= 0xd800 && c <= 0xdbff) {
      const n = s.charCodeAt(++i);
      if (!(n >= 0xdc00 && n <= 0xdfff)) throw new Error('invalid surrogate');
    } else if (c >= 0xdc00 && c <= 0xdfff) throw new Error('invalid surrogate');
  }
}
function jcs(v) {
  if (v === null) return 'null';
  if (typeof v === 'string') { validString(v); return JSON.stringify(v); }
  if (typeof v === 'boolean') return String(v);
  if (typeof v === 'number') {
    if (!Number.isSafeInteger(v)) throw new Error('unsafe/non-canonical number');
    return JSON.stringify(v);
  }
  if (Array.isArray(v)) return `[${v.map(jcs).join(',')}]`;
  if (v && typeof v === 'object') return `{${Object.keys(v).sort().map(k => `${JSON.stringify(k)}:${jcs(v[k])}`).join(',')}}`;
  throw new Error('unsupported JSON value');
}
const sha256 = value => crypto.createHash('sha256').update(value).digest('hex');
const expectedNames = [
  'JoinCareFairLaunchV1',
  'JoinCareFairLaunchV1CoreFacet',
  'JoinCareFairLaunchV1ReleaseFacet',
  'JoinCareFairLaunchV1FundsFacet',
  'JoinCareFairLaunchV1AdminFacet',
  'JoinCareFairLaunchV1ViewsFacet',
  'JoinCareFairLaunchV1Logic',
  'JoinCareFairLaunchV1Runtime',
];
function verify(file,options={}) {
  const raw = fs.readFileSync(file, 'utf8').trim();
  const value = JSON.parse(raw);
  if (value.domain !== 'joincare.contract.dependency-manifest.v1') throw new Error('dependency manifest domain mismatch');
  if (value.chainId !== 56) throw new Error('dependency manifest chainId mismatch');
  if (!Number.isSafeInteger(value.generation) || value.generation < 1) throw new Error('dependency manifest generation invalid');
  if (!['mainnet','testnet','staging'].includes(value.environment)) throw new Error('dependency manifest environment invalid');
  if (!value.solc || value.solc.version !== '0.8.22' || value.solc.build !== '0.8.22+commit.4fc1097e' || value.solc.optimizerEnabled !== true || value.solc.optimizerRuns !== 200 || value.solc.viaIR !== true) throw new Error('dependency compiler baseline mismatch');
  if (!value.source || !/^[0-9a-f]{40}$/.test(value.source.commit||'') || !/^[0-9a-f]{64}$/.test(value.source.treeDigest||'')) throw new Error('dependency source identity invalid');
  if (!value.sourceRegistry || !Number.isSafeInteger(value.sourceRegistry.revision) || value.sourceRegistry.revision<1 || !/^[0-9a-f]{64}$/.test(value.sourceRegistry.digest||'')) throw new Error('dependency Source Registry identity invalid');
  if (value.previousManifestDigest!==null && !/^[0-9a-f]{64}$/.test(value.previousManifestDigest||'')) throw new Error('dependency previous manifest digest invalid');
  if (value.generation===1 && value.previousManifestDigest!==null) throw new Error('generation one must not have predecessor');
  if (value.generation>1 && value.previousManifestDigest===null) throw new Error('later generation requires predecessor');
  for (const field of ['aggregateAbiDigest','selectorMapDigest','storageLayoutDigest','deploymentAttestationDigest','releaseAttestationDigest']) if (!/^[0-9a-f]{64}$/.test(value[field]||'')) throw new Error(`${field} invalid`);
  if (!Array.isArray(value.components) || value.components.length !== expectedNames.length) throw new Error('dependency component count mismatch');
  const addresses = new Set();
  value.components.forEach((entry, index) => {
    if (entry.order !== index || entry.name !== expectedNames[index]) throw new Error(`dependency order/name mismatch at ${index}`);
    if (!/^0x[0-9a-f]{40}$/.test(entry.address) || /^0x0{40}$/.test(entry.address)) throw new Error(`invalid dependency address for ${entry.name}`);
    if (!/^[0-9a-f]{64}$/.test(entry.runtimeCodeHash) || /^0{64}$/.test(entry.runtimeCodeHash)) throw new Error(`invalid dependency codehash for ${entry.name}`);
    if (addresses.has(entry.address)) throw new Error(`duplicate dependency address ${entry.address}`);
    addresses.add(entry.address);
  });
  const canonical = jcs(value);
  if (raw !== canonical) throw new Error('dependency manifest is not RFC 8785 canonical JSON');
  const digest=sha256(Buffer.from(canonical));
  if(options.requireAdmission) throw new Error('fair_launch_authority_unavailable');
  return { manifest: value, canonical, digest, bindings:{deploymentValidation:digest,sourceRegistry:value.sourceRegistry.digest,watermark:digest,releaseAdmission:value.releaseAttestationDigest} };
}
if (require.main === module) {
  try {
    const admitted=process.argv.includes('--require-admission');
    if (Object.keys(process.env).some((name)=>/^jc_fair_launch_test_/i.test(name))) throw new Error('JC_FAIR_LAUNCH_TEST_* is forbidden in production dependency verification');
    const result = verify(process.argv[2],admitted?{requireAdmission:true}:{});
    if (process.argv.includes('--json')) process.stdout.write(JSON.stringify({ digest: result.digest, manifest: result.manifest }));
    else process.stdout.write(`${result.digest}\n`);
  } catch (error) { console.error(error.message); process.exit(1); }
}
module.exports = { verify, jcs, expectedNames };
