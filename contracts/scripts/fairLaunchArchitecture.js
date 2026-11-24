'use strict';
const { ethers, artifacts } = require('hardhat');
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const childProcess = require('child_process');

const facets = [
  ['core', 'JoinCareFairLaunchV1CoreFacet'],
  ['release', 'JoinCareFairLaunchV1ReleaseFacet'],
  ['funds', 'JoinCareFairLaunchV1FundsFacet'],
  ['admin', 'JoinCareFairLaunchV1AdminFacet'],
  ['views', 'JoinCareFairLaunchV1ViewsFacet'],
];
const directNames = new Set(['initialize', 'renounceOwnership', 'owner', 'pendingOwner', 'transferOwnership', 'acceptOwnership', 'proxiableUUID', 'upgradeToAndCall', 'UPGRADE_INTERFACE_VERSION']);
const directCollisionNames = new Set(directNames);
const ROUTER_SOURCE = 'contracts/JoinCareFairLaunchV1.sol';
const PRODUCTION_COMPILER = Object.freeze({ solcVersion: '0.8.22', solcLongVersion: '0.8.22+commit.4fc1097e', viaIR: true, optimizer: { enabled: true, runs: 200 } });
const APPROVAL_SCHEMA = 'fair-launch-full-set-approval-v2';
const APPROVAL_DOMAIN = 'joincare-fair-launch-full-set-approval-v2';
const AUTHORITY_SCHEMA = 'fair-launch-reviewer-authority-v1';
const AUTHORITY_DOMAIN = 'joincare-fair-launch-reviewer-authority-v1';
const STATE_SCHEMA = 'fair-launch-reviewer-state-v1';
const STATE_DOMAIN = 'joincare-fair-launch-reviewer-state-v1';
const PRODUCTION_AUTHORITY_ID = 'joincare-fair-launch-review-board-v1';
const TEST_AUTHORITY_ID = 'joincare-fair-launch-test-review-board-v1';
const WINDOWS_PRODUCTION_AUTHORITY_PATH = 'C:\\ProgramData\\JoinCare\\fair-launch\\reviewer-authority.json';
const UNIX_PRODUCTION_AUTHORITY_PATH = '/etc/joincare/fair-launch/reviewer-authority.json';
const ZERO_DIGEST = `0x${'00'.repeat(32)}`;
const IMPLEMENTATION_ROOT = path.resolve(__dirname, '..');
const UNNAMED_POLICY_PATH = path.resolve(__dirname, 'validation/fair-launch-abi-unnamed-policy.json');
const SOURCE_FILES = [
  'contracts/JoinCareFairLaunchV1.sol',
  ...facets.map(([, name]) => `contracts/fairlaunch/${name}.sol`),
  'contracts/fairlaunch/JoinCareFairLaunchV1Shared.sol',
  'contracts/libraries/JoinCareFairLaunchV1Logic.sol',
  'contracts/libraries/JoinCareFairLaunchV1Runtime.sol',
];
const INTERNAL_TYPE_NAMESPACE_MAP = new Map([
  ['JoinCareFairLaunchV1ArtifactBaseline', 'JoinCareFairLaunchV1Canonical'],
  ['JoinCareFairLaunchV1Shared', 'JoinCareFairLaunchV1Canonical'],
]);

function signature(fragment) { return fragment.format('sighash'); }
function selectorOf(fragment) { return ethers.id(signature(fragment)).slice(0, 10); }
function canonical(value) {
  if (Array.isArray(value)) return value.map(canonical);
  if (value && typeof value === 'object') return Object.fromEntries(Object.keys(value).sort().map(k => [k, canonical(value[k])]));
  return value;
}
function canonicalJson(value) { return JSON.stringify(canonical(value)); }
function digest(value) { return ethers.keccak256(ethers.toUtf8Bytes(canonicalJson(value))); }
function sha256Bytes(value) { return crypto.createHash('sha256').update(value).digest('hex'); }
function wireEntry(entry) {
  const clean = {};
  for (const key of ['type', 'name', 'stateMutability', 'anonymous', 'indexed']) if (entry[key] !== undefined) clean[key] = entry[key];
  if (entry.inputs) clean.inputs = entry.inputs.map(wireParam);
  if (entry.outputs) clean.outputs = entry.outputs.map(wireParam);
  return clean;
}
function normalizeInternalType(internalType) {
  if (internalType === undefined) return undefined;
  return internalType.replace(/^(struct|enum)\s+([A-Za-z0-9_$]+)\.([A-Za-z0-9_$]+)(.*)$/, (all, kind, namespace, typeName, suffix) => {
    const mapped = INTERNAL_TYPE_NAMESPACE_MAP.get(namespace);
    return mapped ? `${kind} ${mapped}.${typeName}${suffix}` : all;
  });
}
function wireParam(param) {
  const clean = { type: param.type, name: param.name === undefined ? '' : param.name };
  const internalType = normalizeInternalType(param.internalType);
  if (internalType !== undefined) clean.internalType = internalType;
  if (param.indexed !== undefined) clean.indexed = param.indexed;
  if (param.components) clean.components = param.components.map(wireParam);
  return clean;
}
function abiKey(entry) {
  if (entry.type === 'function' || entry.type === 'error' || entry.type === 'event') {
    const tupleType = p => p.type.startsWith('tuple') ? `(${(p.components || []).map(tupleType).join(',')})${p.type.slice(5)}` : p.type;
    return `${entry.type}:${entry.name}(${(entry.inputs || []).map(tupleType).join(',')})`;
  }
  return entry.type;
}
function sortedWireAbi(abi) { return abi.map(wireEntry).sort((a, b) => abiKey(a).localeCompare(abiKey(b))); }
function paramPath(entry, side, index) { return `${abiKey(entry)}.${side}[${index}]`; }
function walkParam(param, basePath, visit) {
  visit(param, basePath);
  for (let i = 0; i < (param.components || []).length; i++) walkParam(param.components[i], `${basePath}.components[${i}]`, visit);
}
function reviewerExpectedAbi(golden, policy = require(UNNAMED_POLICY_PATH)) {
  if (!policy || policy.schemaVersion !== 'fair-launch-abi-intentionally-unnamed-policy-v1' || policy.authority !== 'reviewer-approved-freeze-policy') throw new Error('reviewer-approved intentionally-unnamed ABI policy required');
  const approved = new Map();
  for (const entry of policy.entries || []) {
    if (!entry.path || !entry.expectedProductionName || approved.has(entry.path)) throw new Error(`invalid or duplicate intentionally-unnamed ABI policy entry: ${entry.path || '<missing>'}`);
    approved.set(entry.path, entry.expectedProductionName);
  }
  const expected = structuredClone(golden);
  const consumed = new Set();
  for (const entry of expected) {
    for (const side of ['inputs', 'outputs']) for (let i = 0; i < (entry[side] || []).length; i++) {
      walkParam(entry[side][i], paramPath(entry, side, i), (param, fieldPath) => {
        if ((param.name || '') === '' && approved.has(fieldPath)) {
          param.name = approved.get(fieldPath);
          consumed.add(fieldPath);
        }
      });
    }
  }
  const unused = [...approved.keys()].filter(key => !consumed.has(key));
  if (unused.length) throw new Error(`intentionally-unnamed ABI policy contains unknown fields: ${unused.join(',')}`);
  return expected;
}
function assertAggregateAbiExact(actual, golden, policy) {
  const expected = reviewerExpectedAbi(golden, policy);
  const a = sortedWireAbi(actual), e = sortedWireAbi(expected);
  if (canonicalJson(a) !== canonicalJson(e)) throw new Error('production aggregate ABI structurally differs from reviewer-approved named ABI contract');
  return actual;
}

async function aggregateInterface() {
  const golden = require('../scripts/audit/golden/abi.golden.json');
  return new ethers.Interface(Array.isArray(golden) ? golden : golden.abi);
}
async function selectorPlan() {
  const aggregate = await aggregateInterface(); const plan = []; const seen = new Map();
  for (const [key, name] of facets) {
    const artifact = await artifacts.readArtifact(name);
    const own = new Map(artifact.abi.filter(x => x.type === 'function').map(x => [new ethers.Interface([x]).getFunction(x.name).format('sighash'), x]));
    for (const f of aggregate.fragments.filter(x => x.type === 'function' && !directNames.has(x.name))) {
      const sig = signature(f);
      if (!own.has(sig)) continue;
      const selector = selectorOf(f);
      if (seen.has(selector)) throw new Error(`duplicate selector ${selector}: ${seen.get(selector)} / ${sig}`);
      seen.set(selector, sig); plan.push({ key, name, signature: sig, selector });
    }
  }
  const missing = aggregate.fragments.filter(x => x.type === 'function' && !directNames.has(x.name) && !seen.has(selectorOf(x)));
  if (missing.length) throw new Error(`missing selectors: ${missing.map(signature).join(',')}`);
  return plan.sort((a, b) => a.selector.localeCompare(b.selector));
}
function actualRoutingPlan() {
  const source = fs.readFileSync(ROUTER_SOURCE, 'utf8');
  const fallback = source.match(/fallback\(\)[\s\S]*?\{([\s\S]*)assembly\s*\{/);
  if (!fallback) throw new Error('dispatcher fallback body not found');
  const body = fallback[1]; const result = []; let from = 0;
  for (const [key] of facets) {
    const marker = `target = ${key}Facet`;
    const at = body.indexOf(marker, from);
    if (at < 0) throw new Error(`router target missing: ${key}`);
    const clause = body.slice(from, at);
    for (const match of clause.matchAll(/keccak256\("([^"]+)"\)/g)) {
      const sig = match[1]; result.push({ key, signature: sig, selector: ethers.id(sig).slice(0, 10) });
    }
    from = at + marker.length;
  }
  return result.sort((a, b) => a.selector.localeCompare(b.selector));
}
function validateRoutingRows(actual, expected, directSelectors = new Map()) {
  const simple = rows => rows.map(({key, signature, selector}) => ({key, signature, selector})).sort((a,b)=>a.selector.localeCompare(b.selector));
  const normalized = simple(actual), planned = simple(expected);
  if (new Set(normalized.map(x=>x.selector)).size !== normalized.length) throw new Error('actual fallback contains duplicate selector');
  if (JSON.stringify(normalized) !== JSON.stringify(planned)) throw new Error('actual fallback selector routing differs from reviewer-owned aggregate plan');
  for (const row of normalized) if (directSelectors.has(row.selector)) throw new Error(`routed selector collides with direct surface: ${row.signature} / ${directSelectors.get(row.selector)}`);
  return normalized;
}
async function verifyActualRouting() {
  const expected = await selectorPlan(); const actual = actualRoutingPlan();
  const direct = new Map(); const dispatcher = await artifacts.readArtifact('JoinCareFairLaunchV1');
  for (const entry of dispatcher.abi.filter(x => x.type === 'function' && directCollisionNames.has(x.name))) {
    const f = new ethers.Interface([entry]).getFunction(entry.name); direct.set(selectorOf(f), signature(f));
  }
  return validateRoutingRows(actual, expected, direct);
}
async function productionAggregateAbi() {
  const golden = require('../scripts/audit/golden/abi.golden.json');
  const sources = [await artifacts.readArtifact('JoinCareFairLaunchV1'), ...(await Promise.all(facets.map(([, name]) => artifacts.readArtifact(name))))];
  const byKey = new Map();
  for (const source of sources) for (const entry of source.abi) if (entry.type !== 'constructor' && !byKey.has(abiKey(entry))) byKey.set(abiKey(entry), entry);
  const aggregate = [];
  for (const expected of golden) {
    if (expected.type === 'constructor') { aggregate.push(expected); continue; }
    const actual = byKey.get(abiKey(expected));
    if (!actual) throw new Error(`production aggregate ABI missing ${abiKey(expected)}`);
    aggregate.push(actual);
  }
  return aggregate;
}
async function verifyAggregateAbi() {
  const golden = require('../scripts/audit/golden/abi.golden.json');
  return assertAggregateAbiExact(await productionAggregateAbi(), golden);
}
async function verifyProductionCompilerSettings() {
  const contracts=[['dispatcher','JoinCareFairLaunchV1','contracts/JoinCareFairLaunchV1.sol'],...facets.map(([key,name])=>[key,name,`contracts/fairlaunch/${name}.sol`]),['logic','JoinCareFairLaunchV1Logic','contracts/libraries/JoinCareFairLaunchV1Logic.sol'],['runtime','JoinCareFairLaunchV1Runtime','contracts/libraries/JoinCareFairLaunchV1Runtime.sol']];
  for(const [key,name,source] of contracts){
    const build=await artifacts.getBuildInfo(`${source}:${name}`);if(!build)throw new Error(`build info missing for ${key}`);
    const observed={solcVersion:build.solcVersion,solcLongVersion:build.solcLongVersion,viaIR:build.input.settings.viaIR===true,optimizer:{enabled:build.input.settings.optimizer&&build.input.settings.optimizer.enabled===true,runs:build.input.settings.optimizer&&build.input.settings.optimizer.runs}};
    if(canonicalJson(observed)!==canonicalJson(PRODUCTION_COMPILER))throw new Error(`${key} compiler settings differ from frozen production baseline: ${JSON.stringify(observed)}`);
  }
  return PRODUCTION_COMPILER;
}
async function deployFacetSet() {
  const Logic = await ethers.getContractFactory('JoinCareFairLaunchV1Logic'); const logic = await Logic.deploy(); await logic.waitForDeployment();
  const Runtime = await ethers.getContractFactory('JoinCareFairLaunchV1Runtime'); const runtime = await Runtime.deploy(); await runtime.waitForDeployment();
  const libraries = { JoinCareFairLaunchV1Logic: await logic.getAddress(), JoinCareFairLaunchV1Runtime: await runtime.getAddress() };
  const deployed = {};
  for (const [key,name] of facets) {
    const artifact=await artifacts.readArtifact(name); const refs=artifact.linkReferences; const needed={};
    for(const file of Object.keys(refs)) for(const lib of Object.keys(refs[file])) needed[lib]=libraries[lib];
    const F=await ethers.getContractFactory(name,{libraries:needed}); deployed[key]=await F.deploy(); await deployed[key].waitForDeployment();
  }
  const Dispatcher=await ethers.getContractFactory('JoinCareFairLaunchV1');
  const dispatcher=await Dispatcher.deploy(...await Promise.all(facets.map(([key]) => deployed[key].getAddress()))); await dispatcher.waitForDeployment();
  return { logic, runtime, dispatcher, ...deployed };
}
function currentSourceIdentity() {
  const sourceCommit=childProcess.execFileSync('git',['rev-parse','HEAD'],{cwd:IMPLEMENTATION_ROOT,encoding:'utf8'}).trim().toLowerCase();
  if(!/^[0-9a-f]{40}$/.test(sourceCommit))throw new Error('current git commit is invalid');
  const entries=SOURCE_FILES.map(file=>({path:file,sha256:sha256Bytes(fs.readFileSync(path.resolve(IMPLEMENTATION_ROOT,file)))}));
  return {sourceCommit,sourceTreeDigest:digest(entries),sourceFiles:entries};
}
async function manifest(deployed) {
  const components=[];
  for (const [key,name] of [['dispatcher','JoinCareFairLaunchV1'], ...facets, ['logic','JoinCareFairLaunchV1Logic'], ['runtime','JoinCareFairLaunchV1Runtime']]) {
    const address=await deployed[key].getAddress(); components.push({key,name,address,runtimeCodeHash:ethers.keccak256(await ethers.provider.getCode(address))});
  }
  const compiler=await verifyProductionCompilerSettings(),abi=await verifyAggregateAbi(),plan=await verifyActualRouting(); const storage=require('../scripts/audit/golden/storage-layout.golden.json');
  const identity = {schemaVersion:'fair-launch-full-set-v1', compiler, components, aggregateAbiDigest:digest(sortedWireAbi(abi)), selectorMapDigest:digest(plan), storageLayoutDigest:digest(storage)};
  return {...identity, fullSetDigest:digest(identity)};
}
function validateLocalFixture(actual, expected) {
  if (!expected || expected.schemaVersion !== 'fair-launch-full-set-v1' || expected.authority !== 'local-non-authoritative-fixture') throw new Error('explicit non-authoritative local fixture required');
  const comparable = structuredClone(expected); delete comparable.authority; delete comparable.notice;
  if (canonicalJson(actual) !== canonicalJson(comparable)) throw new Error('deployed full-set manifest does not match local non-authoritative fixture');
  return true;
}
function approvalPayload(actual, sourceIdentity, generation, previousApprovalDigest, previousManifestDigest) {
  if (!sourceIdentity || !/^[0-9a-f]{40}$/.test(sourceIdentity.sourceCommit || '') || !/^0x[0-9a-f]{64}$/.test(sourceIdentity.sourceTreeDigest || '')) throw new Error('approval source identity is invalid');
  if (!Number.isSafeInteger(generation) || generation < 1) throw new Error('approval generation must be a positive safe integer');
  for(const [label,value] of [['previous approval digest',previousApprovalDigest],['previous manifest digest',previousManifestDigest]])if(!/^0x[0-9a-f]{64}$/.test(value || ''))throw new Error(`${label} must be bytes32`);
  return {
    schemaVersion: APPROVAL_SCHEMA, domain: APPROVAL_DOMAIN,
    sourceCommit: sourceIdentity.sourceCommit, sourceTreeDigest: sourceIdentity.sourceTreeDigest,
    generation, previousApprovalDigest: previousApprovalDigest.toLowerCase(), previousManifestDigest: previousManifestDigest.toLowerCase(),
    compiler: actual.compiler, components: actual.components, aggregateAbiDigest: actual.aggregateAbiDigest,
    selectorMapDigest: actual.selectorMapDigest, storageLayoutDigest: actual.storageLayoutDigest, fullSetDigest: actual.fullSetDigest,
  };
}
const WINDOWS_POWERSHELL_PATH = 'C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe';
const WINDOWS_PRIVILEGED_SIDS = new Set([
  'S-1-5-18',
  'S-1-5-32-544',
  'S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464',
]);
const WINDOWS_DANGEROUS_RIGHTS = 2 | 4 | 16 | 64 | 256 | 65536 | 262144 | 524288;
const MAX_AUTHORITY_FILE_BYTES = 16 * 1024 * 1024;

function fixedAuthorityPath(platform=process.platform) {
  return platform==='win32'?WINDOWS_PRODUCTION_AUTHORITY_PATH:UNIX_PRODUCTION_AUTHORITY_PATH;
}
const TEST_ONLY_VALIDATOR_ENV_KEYS = Object.freeze([
  'JC_FAIR_LAUNCH_VALIDATOR_TEST_MODE',
  'JC_FAIR_LAUNCH_TEST_AUTHORITY_FILE',
  'JC_FAIR_LAUNCH_TEST_APPROVAL_MODE',
  'JC_FAIR_LAUNCH_TEST_APPROVAL_FILE',
]);
function assertProductionValidatorEnvironment(env=process.env) {
  const forbidden=Object.keys(env).filter(key => {
    const normalizedKey=key.toUpperCase();
    return (TEST_ONLY_VALIDATOR_ENV_KEYS.includes(normalizedKey) || normalizedKey.startsWith('JC_FAIR_LAUNCH_TEST_')) && env[key] !== undefined && env[key] !== '';
  });
  if(forbidden.length)throw new Error(`production validator rejects test-only environment: ${forbidden.sort().join(',')}`);
  return true;
}
function authoritySelection(options={},_env=process.env,platform=process.platform) {
  const testMode=options.testMode===true;
  return {testMode,configured:testMode?options.authorityPath:fixedAuthorityPath(platform)};
}
function exactKeys(value, expected, label) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${label} must be an object`);
  const actual = Object.keys(value).sort(), wanted = [...expected].sort();
  if (actual.length !== wanted.length || actual.some((key, index) => key !== wanted[index])) throw new Error(`${label} has unknown or missing fields`);
}
function canonicalWindowsSid(value, label) {
  if (typeof value !== 'string' || !/^S-(?:0|[1-9][0-9]*)-(?:0|[1-9][0-9]*)(?:-(?:0|[1-9][0-9]*)){1,15}$/.test(value)) throw new Error(`${label} is not a canonical SID`);
  const parts = value.slice(2).split('-').map(Number);
  if (parts.some(part => !Number.isSafeInteger(part) || part < 0) || parts[0] !== 1 || parts[1] > 281474976710655 || parts.slice(2).some(part => part > 0xffffffff)) throw new Error(`${label} is not a canonical SID`);
  return value;
}
function validateWindowsAclJson(acl, expectedPath, expectedDirectory, label) {
  exactKeys(acl, ['schemaVersion','path','ownerSid','isDirectory','isReparsePoint','accessRulesProtected','rules'], `${label} ACL root`);
  if (acl.schemaVersion !== 'joincare-windows-acl-v1' || typeof acl.path !== 'string' || !pathEqual(acl.path, expectedPath, 'win32')) throw new Error(`${label} ACL identity invalid`);
  if (typeof acl.isDirectory !== 'boolean' || acl.isDirectory !== expectedDirectory || typeof acl.isReparsePoint !== 'boolean' || typeof acl.accessRulesProtected !== 'boolean') throw new Error(`${label} ACL file type/protection metadata invalid`);
  if (acl.isReparsePoint) throw new Error(`${label} must not be a Windows reparse point`);
  const ownerSid = canonicalWindowsSid(acl.ownerSid, `${label} owner SID`);
  if (!WINDOWS_PRIVILEGED_SIDS.has(ownerSid)) throw new Error(`${label} owner is not a machine privileged principal`);
  if (!Array.isArray(acl.rules) || acl.rules.length === 0) throw new Error(`${label} ACL rules must be a non-empty complete array`);
  for (let index = 0; index < acl.rules.length; index++) {
    const rule = acl.rules[index], ruleLabel = `${label} ACL rule ${index}`;
    exactKeys(rule, ['identitySid','type','rights','isInherited','inheritContainer','inheritObject','propagateNoPropagate','propagateInheritOnly'], ruleLabel);
    const identitySid = canonicalWindowsSid(rule.identitySid, `${ruleLabel} SID`);
    if (rule.type !== 'Allow' && rule.type !== 'Deny') throw new Error(`${ruleLabel} type invalid`);
    if (!Number.isSafeInteger(rule.rights) || rule.rights < 0 || rule.rights > 0x7fffffff) throw new Error(`${ruleLabel} rights must be a non-negative safe integer`);
    for (const field of ['isInherited','inheritContainer','inheritObject','propagateNoPropagate','propagateInheritOnly']) if (typeof rule[field] !== 'boolean') throw new Error(`${ruleLabel} ${field} must be boolean`);
    if (rule.type === 'Allow' && (rule.rights & WINDOWS_DANGEROUS_RIGHTS) !== 0 && !WINDOWS_PRIVILEGED_SIDS.has(identitySid)) throw new Error(`${label} ACL grants write/create/delete/control rights to untrusted principal ${identitySid}`);
  }
  return acl;
}
function queryWindowsAcl(targetPath, expectedDirectory, label, dependencies={}) {
  const encodedPath=Buffer.from(targetPath,'utf8').toString('base64');
  const script=[
    "$ErrorActionPreference='Stop'",
    `$target=[System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('${encodedPath}'))`,
    '$item=Get-Item -LiteralPath $target -Force',
    '$acl=Get-Acl -LiteralPath $target',
    '$ownerSid=([System.Security.Principal.NTAccount]$acl.Owner).Translate([System.Security.Principal.SecurityIdentifier]).Value',
    "$rules=@($acl.GetAccessRules($true,$true,[System.Security.Principal.SecurityIdentifier])|ForEach-Object{[pscustomobject]@{identitySid=$_.IdentityReference.Value;type=$_.AccessControlType.ToString();rights=[int64]$_.FileSystemRights;isInherited=[bool]$_.IsInherited;inheritContainer=[bool](($_.InheritanceFlags -band [System.Security.AccessControl.InheritanceFlags]::ContainerInherit) -ne 0);inheritObject=[bool](($_.InheritanceFlags -band [System.Security.AccessControl.InheritanceFlags]::ObjectInherit) -ne 0);propagateNoPropagate=[bool](($_.PropagationFlags -band [System.Security.AccessControl.PropagationFlags]::NoPropagateInherit) -ne 0);propagateInheritOnly=[bool](($_.PropagationFlags -band [System.Security.AccessControl.PropagationFlags]::InheritOnly) -ne 0)}})",
    "[pscustomobject]@{schemaVersion='joincare-windows-acl-v1';path=$item.FullName;ownerSid=$ownerSid;isDirectory=[bool]$item.PSIsContainer;isReparsePoint=[bool](($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0);accessRulesProtected=[bool]$acl.AreAccessRulesProtected;rules=$rules}|ConvertTo-Json -Compress -Depth 4",
  ].join(';');
  const execFileSync=dependencies.execFileSync||childProcess.execFileSync;
  let parsed;
  try {
    if (!fs.existsSync(WINDOWS_POWERSHELL_PATH) && !dependencies.execFileSync) throw new Error(`trusted PowerShell binary unavailable at ${WINDOWS_POWERSHELL_PATH}`);
    const output=execFileSync(WINDOWS_POWERSHELL_PATH,['-NoLogo','-NoProfile','-NonInteractive','-EncodedCommand',Buffer.from(script,'utf16le').toString('base64')],{encoding:'utf8',windowsHide:true,env:{}});
    parsed=JSON.parse(output);
  } catch(error) { throw new Error(`${label} Windows ACL could not be securely verified: ${error.message}`); }
  return validateWindowsAclJson(parsed,targetPath,expectedDirectory,label);
}
function validateUnixSecurityStat(stat, expectedDirectory, label) {
  if (!stat || typeof stat.uid !== 'number' || stat.uid !== 0 || typeof stat.mode !== 'number' || (stat.mode & 0o022) !== 0) throw new Error(`${label} must be root-owned and not group/world writable`);
  const isExpected = expectedDirectory ? stat.isDirectory() : stat.isFile();
  if (!isExpected) throw new Error(`${label} has an unexpected file type`);
}
function identityFromStat(stat,label) {
  for (const field of ['dev','ino','nlink','size','mtimeNs','ctimeNs']) if (typeof stat[field] !== 'bigint') throw new Error(`${label} file identity is not provable by this Node/filesystem combination`);
  if (stat.dev < 0n || stat.ino <= 0n || stat.nlink !== 1n || stat.size < 0n || stat.size > BigInt(MAX_AUTHORITY_FILE_BYTES)) throw new Error(`${label} file identity/link-count/size is unsafe`);
  return {dev:stat.dev,ino:stat.ino,nlink:stat.nlink,size:stat.size,mtimeNs:stat.mtimeNs,ctimeNs:stat.ctimeNs};
}
function sameIdentity(a,b) { return ['dev','ino','nlink','size','mtimeNs','ctimeNs'].every(field => a[field] === b[field]); }
function serializedIdentity(identity){return Object.fromEntries(Object.entries(identity).map(([key,value])=>[key,value.toString()]));}
function leafSessionKey(value,platform=process.platform){const normalized=normalizedPath(value,platform);return platform==='win32'?normalized.toLowerCase():normalized;}
function rememberLeafSnapshot(snapshot,session,label,platform=process.platform){
  if(!session)return;
  const key=leafSessionKey(snapshot.path,platform),prior=session.get(key);
  if(prior&&canonicalJson(prior)!==canonicalJson(snapshot))throw new Error(`${label} protected leaf identity, ACL or content changed across authority validation: ${snapshot.path}`);
  if(!prior)session.set(key,snapshot);
}
function pathApi(platform=process.platform){return platform==='win32'?path.win32:path.posix;}
function normalizedPath(value,platform=process.platform){const api=pathApi(platform);return api.resolve(value);}
function relativePath(root,candidate,platform=process.platform){return pathApi(platform).relative(root,candidate);}
function pathEqual(a,b,platform=process.platform) { const left=normalizedPath(a,platform),right=normalizedPath(b,platform);return platform==='win32'?left.toLowerCase()===right.toLowerCase():left===right; }
function containedBy(root,candidate,platform=process.platform) {
  const api=pathApi(platform),rel=relativePath(normalizedPath(root,platform),normalizedPath(candidate,platform),platform),sep=api.sep;return rel===''||(!rel.startsWith(`..${sep}`)&&rel!=='..'&&!api.isAbsolute(rel) && (platform!=='win32'||!rel.includes(':')));
}
function machineTrustChainRoot(configuredPath,testMode,platform=process.platform) {
  if(testMode)return normalizedPath(pathApi(platform).dirname(normalizedPath(configuredPath,platform)),platform);
  return platform==='win32'?path.win32.parse(normalizedPath(configuredPath,platform)).root:'/';
}
function directoryChain(trustChainRoot,parent,platform=process.platform) {
  const api=pathApi(platform),root=normalizedPath(trustChainRoot,platform),leafParent=normalizedPath(parent,platform);
  if(!containedBy(root,leafParent,platform))throw new Error('leaf parent escapes the fixed machine trust-chain root');
  const relative=api.relative(root,leafParent).split(api.sep).filter(Boolean),result=[root];let current=root;
  for(const component of relative){current=api.join(current,component);result.push(current);}return result;
}
function rememberDirectoryChain(results,session,label,platform=process.platform) {
  if(!session)return;
  for(const entry of results){const key=platform==='win32'?entry.path.toLowerCase():entry.path,prior=session.get(key);if(prior&&canonicalJson(prior)!==canonicalJson(entry))throw new Error(`${label} protected ancestor identity or ACL changed across authority validation: ${entry.path}`);if(!prior)session.set(key,entry);}
}
function assertDirectoryChain(parent,label,options={}) {
  const platform=options.platform||process.platform, results=[];
  for(const directory of directoryChain(options.trustChainRoot||options.trustedRoot,parent,platform)) {
    const stat=fs.lstatSync(directory,{bigint:true});
    if (!stat.isDirectory() || stat.isSymbolicLink()) throw new Error(`${label} directory chain contains a non-directory or symlink: ${directory}`);
    if (options.productionSecurity && platform!=='win32') validateUnixSecurityStat(stat,true,`${label} directory ${directory}`);
    const identity={dev:stat.dev,ino:stat.ino,nlink:stat.nlink,size:stat.size,mtimeNs:stat.mtimeNs,ctimeNs:stat.ctimeNs};
    if(Object.values(identity).some(value=>typeof value!=='bigint')||identity.dev<0n||identity.ino<=0n||identity.nlink<1n)throw new Error(`${label} directory identity is not provable: ${directory}`);
    const acl=options.productionSecurity&&platform==='win32'?queryWindowsAcl(directory,true,`${label} directory ${directory}`,options.dependencies):null;
    results.push({path:normalizedPath(directory,platform),identity:Object.fromEntries(Object.entries(identity).map(([key,value])=>[key,value.toString()])),acl});
  }
  rememberDirectoryChain(results,options.chainSession,label,platform);
  return results;
}
function verifyDirectoryChainSession(session,options={}) {
  if(!session||session.size===0)throw new Error('complete authority directory-chain session is empty');
  for(const expected of session.values()){
    const actual=assertDirectoryChain(expected.path,'authority validation final ancestor check',{...options,trustChainRoot:expected.path,chainSession:null});
    if(actual.length!==1||canonicalJson(actual[0])!==canonicalJson(expected))throw new Error(`protected ancestor identity or ACL changed across complete authority validation: ${expected.path}`);
  }
  return true;
}
function readDescriptorTwice(fd,size,label,afterFirstRead) {
  const length=Number(size);if(!Number.isSafeInteger(length))throw new Error(`${label} size is not safely readable`);
  const read=()=>{const buffer=Buffer.alloc(length);let offset=0;while(offset<length){const count=fs.readSync(fd,buffer,offset,length-offset,offset);if(count===0)break;offset+=count;}if(offset!==length)throw new Error(`${label} changed size during read`);return buffer;};
  const first=read();if(afterFirstRead)afterFirstRead();const second=read();if(!first.equals(second))throw new Error(`${label} content changed while held open`);return first;
}
function secureReadFile(inputPath,label,options={}) {
  const platform=options.platform||process.platform,api=pathApi(platform);
  if(!inputPath||!api.isAbsolute(inputPath))throw new Error(`${label} requires an absolute path`);
  const lexical=normalizedPath(inputPath,platform),trustedRoot=normalizedPath(options.trustedRoot||api.dirname(lexical),platform);
  if(!containedBy(trustedRoot,lexical,platform))throw new Error(`${label} must be contained by the fixed protected authority root`);
  const directoriesBefore=assertDirectoryChain(api.dirname(lexical),label,{...options,trustedRoot});
  const prePathStat=fs.lstatSync(lexical,{bigint:true});if(!prePathStat.isFile()||prePathStat.isSymbolicLink())throw new Error(`${label} must be a non-symlink regular file`);
  const prePathIdentity=identityFromStat(prePathStat,label);
  let resolved=lexical,aclBefore=null;
  if(platform==='win32'){
    resolved=fs.realpathSync.native?fs.realpathSync.native(lexical):fs.realpathSync(lexical);
    if(!pathEqual(lexical,resolved,platform)||!containedBy(trustedRoot,resolved,platform))throw new Error(`${label} path is reparse-point/symlinked or outside its protected root`);
    if(options.productionSecurity)aclBefore=queryWindowsAcl(resolved,false,label,options.dependencies);
  }
  if(resolved===IMPLEMENTATION_ROOT||resolved.startsWith(IMPLEMENTATION_ROOT+path.sep))throw new Error(`${label} must be external to the implementation tree`);
  const noFollow=platform==='win32'?0:(fs.constants.O_NOFOLLOW||0),closeExec=fs.constants.O_CLOEXEC||0;let fd;
  try {
    fd=fs.openSync(resolved,fs.constants.O_RDONLY|noFollow|closeExec);
    const beforeStat=fs.fstatSync(fd,{bigint:true});
    if(platform!=='win32'&&options.productionSecurity)validateUnixSecurityStat(beforeStat,false,label);
    else if(!beforeStat.isFile())throw new Error(`${label} must be a regular file`);
    const before=identityFromStat(beforeStat,label);
    if(!sameIdentity(prePathIdentity,before))throw new Error(`${label} pathname identity changed before descriptor open`);
    const content=readDescriptorTwice(fd,before.size,label,options._testAfterFirstRead);
    const after=identityFromStat(fs.fstatSync(fd,{bigint:true}),label);
    if(!sameIdentity(before,after))throw new Error(`${label} identity or metadata changed while held open`);
    const pathAfter=fs.lstatSync(resolved,{bigint:true}),pathIdentity=identityFromStat(pathAfter,label);
    if(!sameIdentity(after,pathIdentity))throw new Error(`${label} pathname identity changed while held open`);
    const directoriesAfter=assertDirectoryChain(api.dirname(resolved),label,{...options,trustedRoot});
    if(canonicalJson(directoriesBefore)!==canonicalJson(directoriesAfter))throw new Error(`${label} protected directory identity or ACL changed while held open`);
    const aclAfter=options.productionSecurity&&platform==='win32'?queryWindowsAcl(resolved,false,label,options.dependencies):null;
    if(options.productionSecurity&&platform==='win32'&&canonicalJson(aclBefore)!==canonicalJson(aclAfter))throw new Error(`${label} ACL changed while held open`);
    const securityIdentity=platform==='win32'?{acl:aclAfter}:{uid:pathAfter.uid,gid:pathAfter.gid,mode:pathAfter.mode};
    const snapshot={path:normalizedPath(resolved,platform),identity:serializedIdentity(after),securityIdentity:canonical(securityIdentity),contentDigest:`0x${sha256Bytes(content)}`,metadata:{size:after.size.toString(),mode:String(pathAfter.mode),uid:String(pathAfter.uid),gid:String(pathAfter.gid)}};
    rememberLeafSnapshot(snapshot,options.leafSession,label,platform);
    return {path:resolved,content,snapshot};
  } finally { if(fd!==undefined)fs.closeSync(fd); }
}
function verifyLeafSession(session,options={}){
  if(!session||session.size===0)throw new Error('complete authority leaf session is empty');
  for(const expected of session.values()){
    const actual=secureReadFile(expected.path,'authority validation final leaf check',{...options,leafSession:null,_testAfterFirstRead:undefined}).snapshot;
    if(canonicalJson(actual)!==canonicalJson(expected))throw new Error(`protected leaf identity, ACL or content changed across complete authority validation: ${expected.path}`);
  }
  return true;
}
function parseSecureJson(read,label){try{return JSON.parse(read.content.toString('utf8'));}catch(error){throw new Error(`${label} JSON invalid: ${error.message}`);}}
function authorityRootFor(authorityPath,platform=process.platform){const api=pathApi(platform);return normalizedPath(api.dirname(normalizedPath(authorityPath,platform)),platform);}
function resolveAuthorityMember(root,configuredPath,label,platform=process.platform){
  const api=pathApi(platform);if(typeof configuredPath!=='string'||configuredPath.length===0)throw new Error(`${label} path invalid`);
  const candidate=api.isAbsolute(configuredPath)?normalizedPath(configuredPath,platform):normalizedPath(api.join(root,configuredPath),platform);
  if(!containedBy(root,candidate,platform))throw new Error(`${label} must be contained by the fixed protected authority root`);
  return candidate;
}
function verifySignature(payloadDigest,signature,keyPem,keyId,label) {
  if(!signature||signature.algorithm!=='ed25519'||signature.keyId!==keyId||!signature.value)throw new Error(`${label} Ed25519 signature metadata invalid`);
  if(!crypto.verify(null,Buffer.from(payloadDigest.slice(2),'hex'),keyPem,Buffer.from(signature.value,'base64')))throw new Error(`${label} Ed25519 signature verification failed`);
}
function readAuthority(options={}) {
  const platform=options.platform||process.platform,{testMode,configured}=authoritySelection(options,process.env,platform),productionSecurity=!testMode,root=authorityRootFor(configured,platform),trustChainRoot=machineTrustChainRoot(configured,testMode,platform),chainSession=options.chainSession||new Map(),leafSession=options.leafSession||new Map(),security={productionSecurity,trustedRoot:root,trustChainRoot,chainSession,leafSession,platform,dependencies:options.dependencies};
  const authorityRead=secureReadFile(configured,'reviewer authority config',security),authority=parseSecureJson(authorityRead,'reviewer authority config');
  const expectedId=testMode?TEST_AUTHORITY_ID:PRODUCTION_AUTHORITY_ID;
  if(authority.schemaVersion!==AUTHORITY_SCHEMA||authority.domain!==AUTHORITY_DOMAIN||authority.authorityId!==expectedId)throw new Error('reviewer authority identity invalid');
  const keyRead=secureReadFile(resolveAuthorityMember(root,authority.publicKeyFile,'reviewer public key',platform),'reviewer public key',security),stateRead=secureReadFile(resolveAuthorityMember(root,authority.stateFile,'reviewer state',platform),'reviewer state',security);
  const keyPem=keyRead.content.toString('utf8'),keyDigest=`0x${sha256Bytes(keyRead.content)}`;
  if(authority.publicKeySha256!==keyDigest||!authority.keyId)throw new Error('reviewer trust-root key digest/identity mismatch');
  const state=parseSecureJson(stateRead,'reviewer state');
  if(!state.payload||state.payload.schemaVersion!==STATE_SCHEMA||state.payload.domain!==STATE_DOMAIN||state.payload.authorityId!==expectedId)throw new Error('reviewer state identity invalid');
  const stateDigest=digest(state.payload);if(state.payloadDigest!==stateDigest)throw new Error('reviewer state payload digest mismatch');
  verifySignature(stateDigest,state.signature,keyPem,authority.keyId,'reviewer state');
  return {testMode,root,security,authorityPath:authorityRead.path,authority,keyPath:keyRead.path,statePath:stateRead.path,state,keyPem};
}
function readSignedApprovalSecure(inputPath,keyPem,keyId,label,security) {
  const read=secureReadFile(inputPath,label,security),artifact=parseSecureJson(read,label);
  if(!artifact.payload||artifact.payload.schemaVersion!==APPROVAL_SCHEMA||artifact.payload.domain!==APPROVAL_DOMAIN)throw new Error(`${label} domain/schema invalid`);
  const payloadDigest=digest(artifact.payload);if(artifact.payloadDigest!==payloadDigest)throw new Error(`${label} payload digest mismatch`);
  verifySignature(payloadDigest,artifact.signature,keyPem,keyId,label);
  return {file:read.path,artifact,payloadDigest};
}
function validateApprovalArtifact(actual, approvalPath, options={}) {
  const sourceIdentity=currentSourceIdentity();const trusted=readAuthority(options);
  const current=readSignedApprovalSecure(resolveAuthorityMember(trusted.root,approvalPath,'approval artifact',trusted.security.platform),trusted.keyPem,trusted.authority.keyId,'approval artifact',trusted.security);
  const approvalFile=current.file,{artifact,payloadDigest}=current;
  const state=trusted.state.payload;let expectedGeneration,previousApprovalDigest,previousManifestDigest;
  if(state.lastApprovalFile){
    const previous=readSignedApprovalSecure(resolveAuthorityMember(trusted.root,state.lastApprovalFile,'previous signed approval',trusted.security.platform),trusted.keyPem,trusted.authority.keyId,'previous signed approval',trusted.security);
    const previousFile=previous.file;if(pathEqual(previousFile,approvalFile))throw new Error('approval cannot be its own predecessor');
    if(previous.payloadDigest!==state.lastApprovalDigest||previous.artifact.payload.fullSetDigest!==state.lastManifestDigest||previous.artifact.payload.generation!==state.lastGeneration)throw new Error('reviewer state does not bind the actual previous signed approval');
    expectedGeneration=state.lastGeneration+1;previousApprovalDigest=previous.payloadDigest;previousManifestDigest=previous.artifact.payload.fullSetDigest;
  }else{
    if(state.lastGeneration!==0||state.lastApprovalDigest!==ZERO_DIGEST||state.lastManifestDigest!==ZERO_DIGEST)throw new Error('genesis reviewer state invalid');
    expectedGeneration=1;previousApprovalDigest=ZERO_DIGEST;previousManifestDigest=ZERO_DIGEST;
  }
  if(expectedGeneration<state.minimumGeneration)throw new Error('approval generation below external reviewer minimum');
  const expected=approvalPayload(actual,sourceIdentity,expectedGeneration,previousApprovalDigest,previousManifestDigest);
  if(canonicalJson(artifact.payload)!==canonicalJson(expected))throw new Error('approval payload does not bind current source/full-set identity or previous chain');
  if(options._testBeforeFinalChainVerification)options._testBeforeFinalChainVerification(trusted);
  verifyDirectoryChainSession(trusted.security.chainSession,trusted.security);
  verifyLeafSession(trusted.security.leafSession,trusted.security);
  return {approvalPath:approvalFile,payloadDigest,sourceCommit:artifact.payload.sourceCommit,sourceTreeDigest:artifact.payload.sourceTreeDigest,generation:artifact.payload.generation,previousApprovalDigest,previousManifestDigest,authorityId:trusted.authority.authorityId,keyId:trusted.authority.keyId,testMode:trusted.testMode,productionAuthorized:!trusted.testMode};
}
module.exports={facets,directNames,PRODUCTION_COMPILER,APPROVAL_DOMAIN,AUTHORITY_DOMAIN,STATE_DOMAIN,PRODUCTION_AUTHORITY_ID,TEST_AUTHORITY_ID,WINDOWS_PRODUCTION_AUTHORITY_PATH,UNIX_PRODUCTION_AUTHORITY_PATH,ZERO_DIGEST,aggregateInterface,selectorPlan,actualRoutingPlan,validateRoutingRows,verifyActualRouting,productionAggregateAbi,verifyAggregateAbi,verifyProductionCompilerSettings,deployFacetSet,manifest,validateLocalFixture,approvalPayload,validateApprovalArtifact,sortedWireAbi,digest,wireEntry,normalizeInternalType,reviewerExpectedAbi,assertAggregateAbiExact,currentSourceIdentity,canonical,canonicalJson,secureReadFile,fixedAuthorityPath,authoritySelection,assertProductionValidatorEnvironment,validateWindowsAclJson,queryWindowsAcl,assertDirectoryChain,verifyDirectoryChainSession,verifyLeafSession,machineTrustChainRoot,WINDOWS_POWERSHELL_PATH};
