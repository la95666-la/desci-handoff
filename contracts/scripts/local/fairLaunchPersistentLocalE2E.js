'use strict';

const childProcess = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const net = require('node:net');
const { keccak256 } = require('ethers');

const AUTHORITY = 'TEST_ONLY_NOT_PRODUCTION_ADMITTED';
const PROCESS_SCHEMA = 'fair-launch-local-e2e-process-v1';
const ROOT_SCHEMA = 'fair-launch-local-e2e-root-v1';
const MANIFEST_SCHEMA = 'fair-launch-local-e2e-deployment-v1';
const TRUST_SCHEMA = 'fair-launch-local-e2e-public-trust-root-v1';
const PIN_SCHEMA = 'fair-launch-local-e2e-external-trust-pin-v1';
const RUN_ID_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._-]{7,127}$/;
const KEY_ID_PATTERN = /^local-e2e-[0-9a-f]{24}$/;
const SHA256_PATTERN = /^[0-9a-f]{64}$/;
const ADDRESS_PATTERN = /^0x[0-9a-f]{40}$/;
const HASH_PATTERN = /^0x[0-9a-f]{64}$/;
const MAX_LIFETIME_MS = 24 * 60 * 60 * 1000;
const REPOSITORY_ROOT = path.resolve(__dirname, '../..');
const DEFAULT_STATE_ROOT = path.join(os.tmpdir(), 'joincare-fair-launch-contract-e2e');
const admittedManifestSnapshots = new WeakSet();
const LOCAL_ENV_KEYS = Object.freeze([
  'JC_FAIR_LAUNCH_LOCAL_E2E_RPC_URL',
  'JC_FAIR_LAUNCH_LOCAL_E2E_RUN_ID',
  'JC_FAIR_LAUNCH_LOCAL_E2E_STATE_ROOT',
]);

function fail(message) { throw new Error(`Fair Launch persistent LOCAL-ONLY: ${message}`); }
function canonical(value) {
  if (Array.isArray(value)) return value.map(canonical);
  if (value && typeof value === 'object') return Object.fromEntries(Object.keys(value).sort().map((key) => [key, canonical(value[key])]));
  return value;
}
function canonicalJson(value) { return JSON.stringify(canonical(value)); }
function sha256(value) { return crypto.createHash('sha256').update(value).digest('hex'); }
function exactRpcUrl(value) {
  let url;
  try { url = new URL(String(value || '')); } catch { fail('RPC URL is invalid'); }
  const port = Number(url.port);
  if (url.protocol !== 'http:' || url.hostname !== '127.0.0.1' || !/^\d+$/.test(url.port) || port < 1 || port > 65535 ||
      url.pathname !== '/' || url.username || url.password || url.search || url.hash || url.origin !== String(value)) {
    fail('RPC URL must be exact http://127.0.0.1:<explicit-port>');
  }
  return { url: url.origin, port };
}
function exactRunId(value) {
  const runId = String(value || '');
  if (!RUN_ID_PATTERN.test(runId)) fail('run ID is invalid');
  return runId;
}
function rejectForbiddenEnvironment(env = process.env) {
  const forbidden = Object.keys(env).filter((key) => {
    if (env[key] === undefined || env[key] === '') return false;
    const upper = key.toUpperCase();
    return upper.startsWith('JC_DEPLOY_') || upper.includes('PRIVATE_KEY') || upper.includes('MNEMONIC') || upper === 'BSCSCAN_API_KEY';
  });
  if (forbidden.length) fail(`forbidden production/key environment: ${forbidden.sort().join(', ')}`);
}
function absoluteStateRoot(value = DEFAULT_STATE_ROOT) {
  const resolved = path.resolve(String(value || ''));
  if (!path.isAbsolute(resolved) || resolved !== path.normalize(resolved)) fail('state root must be an absolute canonical path');
  return resolved;
}
function modeOf(stat) { return stat.mode & 0o777; }
function assertOwnedPath(target, kind, expectedMode) {
  const stat = fs.lstatSync(target);
  if ((kind === 'directory' && !stat.isDirectory()) || (kind === 'file' && !stat.isFile())) fail(`${path.basename(target)} type is invalid`);
  if (stat.isSymbolicLink() || (kind === 'file' && stat.nlink !== 1)) fail(`${path.basename(target)} link identity is invalid`);
  if (typeof process.getuid === 'function' && stat.uid !== process.getuid()) fail(`${path.basename(target)} owner is invalid`);
  if (process.platform !== 'win32' && modeOf(stat) !== expectedMode) fail(`${path.basename(target)} mode is invalid`);
  return stat;
}
function ensurePrivateDirectory(target, create = false) {
  if (create) fs.mkdirSync(target, { recursive: false, mode: 0o700 });
  assertOwnedPath(target, 'directory', 0o700);
}
function ensureBaseDirectory(target) {
  if (!fs.existsSync(target)) fs.mkdirSync(target, { recursive: true, mode: 0o700 });
  assertOwnedPath(target, 'directory', 0o700);
}
function writeOnceJson(target, value) {
  const bytes = `${JSON.stringify(value, null, 2)}\n`;
  const fd = fs.openSync(target, 'wx', 0o600);
  try { fs.writeFileSync(fd, bytes, 'utf8'); fs.fsyncSync(fd); } finally { fs.closeSync(fd); }
  if (process.platform !== 'win32') fs.chmodSync(target, 0o600);
  assertOwnedPath(target, 'file', 0o600);
}
function writeReplaceJson(target, value) {
  const temp = `${target}.tmp-${process.pid}-${crypto.randomBytes(6).toString('hex')}`;
  writeOnceJson(temp, value);
  fs.renameSync(temp, target);
  if (process.platform !== 'win32') fs.chmodSync(target, 0o600);
}
function parseJsonNoDuplicateKeys(text, label = 'JSON') {
  let index = 0;
  const error = () => fail(`${label} is invalid or contains duplicate object keys`);
  const whitespace = () => { while (/\s/.test(text[index] || '')) index += 1; };
  function string() {
    if (text[index] !== '"') error();
    const start = index++;
    let escaped = false;
    while (index < text.length) {
      const c = text[index++];
      if (escaped) { escaped = false; continue; }
      if (c === '\\') { escaped = true; continue; }
      if (c === '"') { try { return JSON.parse(text.slice(start, index)); } catch { error(); } }
      if (c.charCodeAt(0) < 0x20) error();
    }
    error();
  }
  function value() {
    whitespace();
    const c = text[index];
    if (c === '"') return string();
    if (c === '{') {
      index += 1; whitespace();
      const out = {}, keys = new Set();
      if (text[index] === '}') { index += 1; return out; }
      while (true) {
        whitespace(); const key = string(); whitespace();
        if (keys.has(key) || text[index++] !== ':') error();
        keys.add(key); out[key] = value(); whitespace();
        if (text[index] === '}') { index += 1; return out; }
        if (text[index++] !== ',') error();
      }
    }
    if (c === '[') {
      index += 1; whitespace(); const out = [];
      if (text[index] === ']') { index += 1; return out; }
      while (true) {
        out.push(value()); whitespace();
        if (text[index] === ']') { index += 1; return out; }
        if (text[index++] !== ',') error();
      }
    }
    const match = text.slice(index).match(/^(?:true|false|null|-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?)/);
    if (!match) error();
    index += match[0].length;
    try { return JSON.parse(match[0]); } catch { error(); }
  }
  const parsed = value(); whitespace();
  if (index !== text.length) error();
  return parsed;
}
function readStrictJson(target) {
  assertOwnedPath(target, 'file', 0o600);
  const value = parseJsonNoDuplicateKeys(fs.readFileSync(target, 'utf8'), `${path.basename(target)} JSON`);
  if (!value || typeof value !== 'object' || Array.isArray(value)) fail(`${path.basename(target)} JSON is invalid`);
  return value;
}
function createCanonicalJsonOnce(target, value) {
  const bytes = `${canonicalJson(value)}\n`;
  const fd = fs.openSync(target, 'wx', 0o600);
  try { fs.writeFileSync(fd, bytes, 'utf8'); fs.fsyncSync(fd); } finally { fs.closeSync(fd); }
  if (process.platform !== 'win32') fs.chmodSync(target, 0o600);
  assertOwnedPath(target, 'file', 0o600);
  return { sha256: sha256(bytes), bytes };
}
function stableCanonicalJsonSnapshot(target, expectedSha256, beforeFinalCheck) {
  const expected = expectedSha256 === undefined ? null : exactPinSha256(expectedSha256);
  const before = assertOwnedPath(target, 'file', 0o600);
  const bytes = fs.readFileSync(target, 'utf8');
  const digest = sha256(bytes);
  if (expected !== null && digest !== expected) fail(`${path.basename(target)} SHA-256 mismatch`);
  const value = parseJsonNoDuplicateKeys(bytes, `${path.basename(target)} JSON`);
  if (!value || typeof value !== 'object' || Array.isArray(value) || `${canonicalJson(value)}\n` !== bytes) fail(`${path.basename(target)} canonical bytes mismatch`);
  if (beforeFinalCheck) beforeFinalCheck();
  const afterBytes = fs.readFileSync(target, 'utf8');
  const after = assertOwnedPath(target, 'file', 0o600);
  for (const key of ['dev','ino','size','mtimeMs','ctimeMs']) if (before[key] !== after[key]) fail(`${path.basename(target)} file identity changed`);
  if (afterBytes !== bytes || sha256(afterBytes) !== digest) fail(`${path.basename(target)} snapshot changed`);
  return { value, bytes, sha256: digest, stat: before };
}
function sameFileIdentity(a, b) {
  return ['dev','ino','size','mtimeMs','ctimeMs','birthtimeMs'].every((key) => a?.[key] === b?.[key]);
}
function assertAdmittedManifestCurrent(target, snapshot, beforeFinalCheck) {
  if (!admittedManifestSnapshots.has(snapshot)) fail('manifest snapshot is not an admitted capability');
  try {
    if (beforeFinalCheck) beforeFinalCheck();
    const handleStat = fs.fstatSync(snapshot.fd);
    const current = stableCanonicalJsonSnapshot(target, snapshot.sha256);
    if (!sameFileIdentity(snapshot.stat, handleStat) || !sameFileIdentity(snapshot.stat, current.stat) || current.bytes !== snapshot.bytes) fail('admitted manifest file identity changed');
    return snapshot;
  } finally {
    try { fs.closeSync(snapshot.fd); } catch (_) {}
    admittedManifestSnapshots.delete(snapshot);
  }
}
function openAdmittedManifestSnapshot(target, expectedSha256) {
  const expected = exactPinSha256(expectedSha256);
  const fd = fs.openSync(target, 'r');
  try {
    const stat = fs.fstatSync(fd);
    const bytes = fs.readFileSync(fd, 'utf8');
    const pathStat = assertOwnedPath(target, 'file', 0o600);
    if (!sameFileIdentity(stat, pathStat) || sha256(bytes) !== expected) fail('manifest admitted snapshot identity mismatch');
    const value = parseJsonNoDuplicateKeys(bytes, `${path.basename(target)} JSON`);
    if (!value || typeof value !== 'object' || Array.isArray(value) || `${canonicalJson(value)}\n` !== bytes) fail('manifest canonical bytes mismatch');
    return { fd, stat, bytes, value, sha256: expected };
  } catch (error) { try { fs.closeSync(fd); } catch (_) {} throw error; }
}
function parseArgs(argv, allowed) {
  const out = {};
  for (let i = 0; i < argv.length; i += 2) {
    const key = argv[i];
    if (!allowed.includes(key) || i + 1 >= argv.length || Object.hasOwn(out, key)) fail('arguments are invalid');
    out[key] = argv[i + 1];
  }
  if (argv.length % 2) fail('arguments are invalid');
  return out;
}
function validateExternalPinPathSyntax(value, platform = process.platform) {
  if (typeof value !== 'string' || value.length === 0) fail('trust pin path must be an original absolute canonical file path');
  if (platform === 'win32') {
    if (/^(?:\\\\|\/\/|\\\?\?\\)/.test(value) || !/^[A-Z]:\\/.test(value) || value.includes('/') || value.slice(2).includes(':') || value.endsWith('\\')) fail('Windows trust pin must be a normal local drive path');
    const segments = value.slice(3).split('\\');
    if (segments.some((part) => !part || part === '.' || part === '..' || /[ .]$/.test(part)) || path.win32.normalize(value) !== value) fail('Windows trust pin path is noncanonical');
  } else if (!path.posix.isAbsolute(value) || path.posix.normalize(value) !== value || value.endsWith('/') || value.split('/').some((part) => part === '.' || part === '..')) {
    fail('trust pin path must be an original absolute canonical file path');
  }
  return value;
}
function exactExternalPinPath(value, paths) {
  validateExternalPinPathSyntax(value);
  const target = path.resolve(value);
  if (target !== value) fail('trust pin path must be an original absolute canonical file path');
  const runDir = fs.realpathSync(paths.runDir);
  const relative = path.relative(runDir, target);
  if (relative === '' || (!relative.startsWith('..' + path.sep) && relative !== '..')) fail('trust pin must be outside the Contract run directory');
  const parent = fs.realpathSync(path.dirname(target));
  if (path.join(parent, path.basename(target)) !== target) fail('trust pin parent identity mismatch');
  return target;
}
function runPaths(runId, stateRoot = DEFAULT_STATE_ROOT) {
  const root = absoluteStateRoot(stateRoot);
  return {
    stateRoot: root,
    runDir: path.join(root, exactRunId(runId)),
    rootAuthority: path.join(root, runId, 'root-authority.json'),
    processAuthority: path.join(root, runId, 'process-authority.json'),
    ready: path.join(root, runId, 'ready.json'),
    log: path.join(root, runId, 'node.log'),
    failure: path.join(root, runId, 'failure.json'),
    trustRoot: path.join(root, runId, 'public-trust-root.json'),
    manifest: path.join(root, runId, 'deployment-manifest.json'),
  };
}
function rootAuthority(runId, rpcUrl, stateRoot) {
  const paths = runPaths(runId, stateRoot);
  const stat = assertOwnedPath(paths.runDir, 'directory', 0o700);
  return {
    schemaVersion: ROOT_SCHEMA, authority: AUTHORITY, runId, rpcUrl, chainId: 31337,
    repositoryRoot: fs.realpathSync(REPOSITORY_ROOT), runDirectory: fs.realpathSync(paths.runDir),
    runDirectoryIdentity: { dev: String(stat.dev), ino: String(stat.ino), uid: typeof stat.uid === 'number' ? stat.uid : null, mode: modeOf(stat) },
    createdAt: new Date().toISOString(),
  };
}
function verifyRoot(paths, expectedRunId, expectedRpcUrl) {
  assertOwnedPath(paths.stateRoot, 'directory', 0o700);
  const runStat = assertOwnedPath(paths.runDir, 'directory', 0o700);
  const root = readStrictJson(paths.rootAuthority);
  if (root.schemaVersion !== ROOT_SCHEMA || root.authority !== AUTHORITY || root.runId !== expectedRunId || root.rpcUrl !== expectedRpcUrl || root.chainId !== 31337 ||
      root.repositoryRoot !== fs.realpathSync(REPOSITORY_ROOT) || root.runDirectory !== fs.realpathSync(paths.runDir) ||
      root.runDirectoryIdentity?.dev !== String(runStat.dev) || root.runDirectoryIdentity?.ino !== String(runStat.ino) ||
      (typeof process.getuid === 'function' && root.runDirectoryIdentity?.uid !== process.getuid())) fail('root authority identity mismatch');
  return root;
}
function linuxProcessSnapshot(pid) {
  const statText = fs.readFileSync(`/proc/${pid}/stat`, 'utf8');
  const close = statText.lastIndexOf(')');
  const fields = statText.slice(close + 2).split(' ');
  const argv = fs.readFileSync(`/proc/${pid}/cmdline`).toString('utf8').split('\0').filter(Boolean);
  return { pid, parentPid: Number(fields[1]), startIdentity: fields[19], executable: fs.realpathSync(`/proc/${pid}/exe`), argv };
}
function windowsProcessSnapshot(pid) {
  const ps = path.join(process.env.SystemRoot || 'C:\\Windows', 'System32', 'WindowsPowerShell', 'v1.0', 'powershell.exe');
  const script = `$p=Get-CimInstance Win32_Process -Filter \"ProcessId = ${Number(pid)}\"; if($null -eq $p){exit 3}; [pscustomobject]@{pid=[int]$p.ProcessId;parentPid=[int]$p.ParentProcessId;startIdentity=$p.CreationDate.ToUniversalTime().ToString('o');executable=$p.ExecutablePath;commandLine=$p.CommandLine}|ConvertTo-Json -Compress`;
  const result = childProcess.spawnSync(ps, ['-NoProfile', '-NonInteractive', '-Command', script], { encoding: 'utf8', windowsHide: true, timeout: 10000 });
  if (result.status !== 0) fail(`process ${pid} is unavailable`);
  return JSON.parse(result.stdout.trim());
}
function processSnapshot(pid) {
  if (!Number.isSafeInteger(Number(pid)) || Number(pid) < 1) fail('process PID is invalid');
  return process.platform === 'win32' ? windowsProcessSnapshot(Number(pid)) : linuxProcessSnapshot(Number(pid));
}
function sameProcess(a, b) {
  return a && b && a.pid === b.pid && a.parentPid === b.parentPid && a.startIdentity === b.startIdentity && a.executable === b.executable &&
    (process.platform === 'win32' ? a.commandLine === b.commandLine : JSON.stringify(a.argv) === JSON.stringify(b.argv));
}
function verifyProcessAuthority(paths, runId, rpcUrl) {
  verifyRoot(paths, runId, rpcUrl);
  const record = readStrictJson(paths.processAuthority);
  if (record.schemaVersion !== PROCESS_SCHEMA || record.authority !== AUTHORITY || record.runId !== runId || record.rpcUrl !== rpcUrl || record.chainId !== 31337 ||
      record.repositoryRoot !== fs.realpathSync(REPOSITORY_ROOT) || record.runDirectory !== fs.realpathSync(paths.runDir)) fail('process authority fields mismatch');
  const supervisorNow = processSnapshot(record.supervisor?.pid);
  const nodeNow = processSnapshot(record.node?.pid);
  if (!sameProcess(record.supervisor, supervisorNow) || !sameProcess(record.node, nodeNow) || record.node.parentPid !== record.supervisor.pid) fail('process authority identity mismatch');
  return record;
}
async function rpcCall(rpcUrl, method, params = [], timeoutMs = 3000) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const response = await fetch(rpcUrl, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params }), signal: controller.signal });
    const body = await response.json();
    if (!response.ok || body.error || body.id !== 1) fail(`RPC ${method} failed`);
    return body.result;
  } catch (error) {
    if (String(error.message || error).startsWith('Fair Launch persistent')) throw error;
    fail(`RPC ${method} unavailable`);
  } finally { clearTimeout(timer); }
}
async function verifyRpc(rpcUrl) {
  const chainId = await rpcCall(rpcUrl, 'eth_chainId');
  if (chainId !== '0x7a69') fail('RPC chain ID is not 31337');
  const genesis = await rpcCall(rpcUrl, 'eth_getBlockByNumber', ['0x0', false]);
  if (!genesis || !HASH_PATTERN.test(String(genesis.hash || '').toLowerCase()) || /^0x0{64}$/.test(genesis.hash)) fail('RPC genesis block is invalid');
  return { chainId: 31337, genesisHash: genesis.hash.toLowerCase() };
}
async function waitForRpc(rpcUrl, timeoutMs = 45000) {
  const deadline = Date.now() + timeoutMs;
  let last;
  while (Date.now() < deadline) {
    try { return await verifyRpc(rpcUrl); } catch (error) { last = error; await new Promise((resolve) => setTimeout(resolve, 300)); }
  }
  fail(`RPC readiness timed out${last ? `: ${last.message}` : ''}`);
}
function tcpProbe(rpcUrl, timeoutMs = 700) {
  const rpc = exactRpcUrl(rpcUrl);
  return new Promise((resolve) => {
    const socket = net.createConnection({ host: '127.0.0.1', port: rpc.port });
    let done = false;
    const finish = (status, code) => { if (done) return; done = true; socket.destroy(); resolve({ status, code }); };
    socket.once('connect', () => finish('open','CONNECTED'));
    socket.once('error', (error) => finish(error.code === 'ECONNREFUSED' ? 'refused' : 'ambiguous', error.code || 'ERROR'));
    socket.setTimeout(timeoutMs, () => finish('ambiguous','TIMEOUT'));
  });
}
async function waitForClosed(rpcUrl, timeoutMs = 20000, probe = tcpProbe) {
  const deadline = Date.now() + timeoutMs;
  let consecutive = 0;
  while (Date.now() < deadline) {
    const result = await probe(rpcUrl);
    if (result.status === 'refused' && result.code === 'ECONNREFUSED') consecutive += 1;
    else if (result.status === 'ambiguous') fail(`TCP closure probe is ambiguous: ${result.code}`);
    else consecutive = 0;
    if (consecutive >= 3) return true;
    await new Promise((resolve) => setTimeout(resolve, 150));
  }
  fail('TCP endpoint did not produce stable ECONNREFUSED closure');
}
function recordedIdentityPresent(recorded) {
  try { return sameProcess(recorded, processSnapshot(recorded.pid)); } catch { return false; }
}
async function waitForRecordedIdentitiesGone(record, timeoutMs = 30000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (!recordedIdentityPresent(record.supervisor) && !recordedIdentityPresent(record.node)) return true;
    await new Promise((resolve) => setTimeout(resolve, 150));
  }
  fail('recorded supervisor/node identity remains alive');
}
async function stopVerifiedProcess(paths, runId, rpcUrl) {
  const first = verifyProcessAuthority(paths, runId, rpcUrl);
  const immediate = verifyProcessAuthority(paths, runId, rpcUrl);
  if (!sameProcess(first.supervisor, immediate.supervisor) || !sameProcess(first.node, immediate.node)) fail('process identity drift');
  process.kill(first.supervisor.pid, 'SIGTERM');
  await waitForRecordedIdentitiesGone(first, 30000);
  await waitForClosed(rpcUrl, 30000);
  return first;
}
function sanitizeDiagnostic(value) {
  return String(value || '')
    .replace(/(Account\s+#\d+[\s\S]{0,120}?Private Key\s*:\s*)\S+/gi, '$1[REDACTED]')
    .replace(/-----BEGIN[\s\S]*?PRIVATE KEY-----/gi, '[REDACTED PRIVATE KEY]')
    .slice(-12000);
}
function exactKeys(value, expected, label) {
  if (!value || typeof value !== 'object' || Array.isArray(value) || JSON.stringify(Object.keys(value).sort()) !== JSON.stringify([...expected].sort())) fail(`${label} fields are invalid`);
}
function publicKeyIdentity(jwk) {
  exactKeys(jwk, ['crv','kty','x'], 'public key');
  if (jwk.kty !== 'OKP' || jwk.crv !== 'Ed25519' || typeof jwk.x !== 'string') fail('public key is invalid');
  const key = crypto.createPublicKey({ key: jwk, format: 'jwk' });
  return { key, keyId: `local-e2e-${sha256(key.export({ type: 'spki', format: 'der' })).slice(0, 24)}` };
}
function authorityDigest(target) { return sha256(canonicalJson(readStrictJson(target))); }
function createPublicTrustAuthority(paths, runId, rpcUrl, genesisHash, pair) {
  verifyProcessAuthority(paths, runId, rpcUrl);
  if (!HASH_PATTERN.test(genesisHash)) fail('trust genesis is invalid');
  const publicJwk = pair.publicKey.export({ format: 'jwk' });
  const { keyId } = publicKeyIdentity(publicJwk);
  const record = { schemaVersion: TRUST_SCHEMA, authority: AUTHORITY, runId, rpcUrl, chainId: 31337, genesisHash,
    rootAuthoritySha256: authorityDigest(paths.rootAuthority), processAuthoritySha256: authorityDigest(paths.processAuthority),
    keyId, publicKeyJwk: publicJwk, createdAt: new Date().toISOString() };
  writeOnceJson(paths.trustRoot, record);
  verifyPublicTrustAuthority(paths, runId, rpcUrl, genesisHash);
  return record;
}
function verifyPublicTrustAuthority(paths, runId, rpcUrl, genesisHash) {
  verifyProcessAuthority(paths, runId, rpcUrl);
  const record = readStrictJson(paths.trustRoot);
  exactKeys(record, ['schemaVersion','authority','runId','rpcUrl','chainId','genesisHash','rootAuthoritySha256','processAuthoritySha256','keyId','publicKeyJwk','createdAt'], 'public trust authority');
  const identity = publicKeyIdentity(record.publicKeyJwk);
  if (record.schemaVersion !== TRUST_SCHEMA || record.authority !== AUTHORITY || record.runId !== runId || record.rpcUrl !== rpcUrl || record.chainId !== 31337 ||
      record.genesisHash !== genesisHash || record.rootAuthoritySha256 !== authorityDigest(paths.rootAuthority) || record.processAuthoritySha256 !== authorityDigest(paths.processAuthority) ||
      record.keyId !== identity.keyId || !Number.isFinite(Date.parse(record.createdAt))) fail('public trust authority identity mismatch');
  return record;
}
function exactPinSha256(value) {
  const digest = String(value || '');
  if (!SHA256_PATTERN.test(digest)) fail('explicit trust pin SHA-256 must be 64 lowercase hexadecimal characters');
  return digest;
}
function createExternalTrustPin(paths, pinPath, runId, rpcUrl, genesisHash, trustRecord) {
  const target = exactExternalPinPath(pinPath, paths);
  if (fs.existsSync(target)) fail('external trust pin target already exists');
  const trustDigest = sha256(canonicalJson(trustRecord));
  const publicKeyDigest = sha256(canonicalJson(trustRecord.publicKeyJwk));
  const pin = { schemaVersion: PIN_SCHEMA, authority: AUTHORITY, runId, rpcUrl, chainId: 31337, genesisHash,
    keyId: trustRecord.keyId, trustRecordSha256: trustDigest, publicKeySha256: publicKeyDigest,
    trustCreatedAt: trustRecord.createdAt, createdAt: new Date().toISOString() };
  const canonicalBytes = `${canonicalJson(pin)}\n`;
  const fd = fs.openSync(target, 'wx', 0o600);
  try { fs.writeFileSync(fd, canonicalBytes, 'utf8'); fs.fsyncSync(fd); } finally { fs.closeSync(fd); }
  if (process.platform !== 'win32') fs.chmodSync(target, 0o600);
  assertOwnedPath(target, 'file', 0o600);
  return { pin, pinSha256: sha256(canonicalBytes) };
}
function verifyExternalTrustPin(paths, pinPath, expectedPinSha256, runId, rpcUrl, genesisHash, trustRecord) {
  const target = exactExternalPinPath(pinPath, paths);
  const expectedDigest = exactPinSha256(expectedPinSha256);
  const pinStatBefore = assertOwnedPath(target, 'file', 0o600);
  const pinBytes = fs.readFileSync(target, 'utf8');
  if (sha256(pinBytes) !== expectedDigest) fail('external trust pin SHA-256 mismatch');
  const pin = parseJsonNoDuplicateKeys(pinBytes, `${path.basename(target)} JSON`);
  if (`${canonicalJson(pin)}\n` !== pinBytes) fail('external trust pin bytes are not canonical');
  const pinStatAfter = assertOwnedPath(target, 'file', 0o600);
  exactKeys(pin, ['schemaVersion','authority','runId','rpcUrl','chainId','genesisHash','keyId','trustRecordSha256','publicKeySha256','trustCreatedAt','createdAt'], 'external trust pin');
  if (pinStatBefore.dev !== pinStatAfter.dev || pinStatBefore.ino !== pinStatAfter.ino || pinStatBefore.size !== pinStatAfter.size || pinStatBefore.mtimeMs !== pinStatAfter.mtimeMs || pinStatBefore.ctimeMs !== pinStatAfter.ctimeMs || pin.schemaVersion !== PIN_SCHEMA || pin.authority !== AUTHORITY ||
      pin.runId !== runId || pin.rpcUrl !== rpcUrl || pin.chainId !== 31337 || pin.genesisHash !== genesisHash || pin.keyId !== trustRecord.keyId ||
      pin.trustRecordSha256 !== sha256(canonicalJson(trustRecord)) || pin.publicKeySha256 !== sha256(canonicalJson(trustRecord.publicKeyJwk)) || pin.trustCreatedAt !== trustRecord.createdAt ||
      !Number.isFinite(Date.parse(pin.createdAt)) || Date.parse(pin.createdAt) < Date.parse(trustRecord.createdAt)) fail('external trust pin identity mismatch');
  return pin;
}
// Signature integrity only. This does not admit a manifest; callers must use strictRuntimeAdmission.
function verifyManifestSignatureIntegrity(envelope, trustedJwk, trustedKeyId) {
  exactKeys(envelope, ['payload','signature','testTrustRoot'], 'manifest envelope');
  exactKeys(envelope.signature, ['algorithm','keyId','payloadSha256','value'], 'manifest signature');
  exactKeys(envelope.testTrustRoot, ['authority','keyId','publicKeyJwk'], 'manifest embedded trust projection');
  const trusted = publicKeyIdentity(trustedJwk);
  if (trusted.keyId !== trustedKeyId || envelope.signature.algorithm !== 'ed25519' || envelope.signature.keyId !== trustedKeyId || !SHA256_PATTERN.test(envelope.signature.payloadSha256 || '') ||
      envelope.testTrustRoot.authority !== AUTHORITY || envelope.testTrustRoot.keyId !== trustedKeyId || canonicalJson(envelope.testTrustRoot.publicKeyJwk) !== canonicalJson(trustedJwk)) fail('manifest external trust mismatch');
  const bytes = canonicalJson(envelope.payload);
  let signature;
  try { signature = Buffer.from(envelope.signature.value, 'base64'); } catch { fail('manifest signature encoding is invalid'); }
  if (sha256(bytes) !== envelope.signature.payloadSha256 || !crypto.verify(null, Buffer.from(bytes), trusted.key, signature)) fail('manifest signature verification failed');
  return { payloadSha256: envelope.signature.payloadSha256, keyId: trustedKeyId };
}
function addressIdentity(value, label, extraKeys = []) {
  exactKeys(value, ['address','runtimeCodeHash',...extraKeys], label);
  if (!ADDRESS_PATTERN.test(value.address || '') || !HASH_PATTERN.test(value.runtimeCodeHash || '')) fail(`${label} identity is invalid`);
}
function receiptIdentity(value, label) {
  exactKeys(value, ['txHash','blockNumber','blockHash','transactionIndex','blockTimestamp','from','to','contractAddress','valueRaw','inputHash'], label);
  if (!HASH_PATTERN.test(value.txHash || '') || !/^(?:0|[1-9]\d*)$/.test(value.blockNumber || '') || !HASH_PATTERN.test(value.blockHash || '') || !Number.isSafeInteger(value.transactionIndex) || value.transactionIndex < 0 || !/^(?:0|[1-9]\d*)$/.test(value.blockTimestamp || '')) fail(`${label} is invalid`);
  if (!ADDRESS_PATTERN.test(value.from || '') || (value.to !== null && !ADDRESS_PATTERN.test(value.to || '')) || (value.contractAddress !== null && !ADDRESS_PATTERN.test(value.contractAddress || '')) || !/^(?:0|[1-9]\d*)$/.test(value.valueRaw || '') || !HASH_PATTERN.test(value.inputHash || '')) fail(`${label} transaction identity is invalid`);
}
function artifactAbi(relative) { return JSON.parse(fs.readFileSync(path.join(REPOSITORY_ROOT, 'artifacts-local-e2e', relative), 'utf8')).abi; }
async function verifyReceipt(provider, facts, expectedTo, expectedFrom, expectedContractAddress, eventCheck) {
  const receipt = await provider.getTransactionReceipt(facts.txHash), tx = await provider.getTransaction(facts.txHash), block = await provider.getBlock(Number(facts.blockNumber));
  if (!receipt || !tx || !block || receipt.status !== 1 || String(receipt.blockNumber) !== facts.blockNumber || receipt.blockHash.toLowerCase() !== facts.blockHash || receipt.index !== facts.transactionIndex || String(block.timestamp) !== facts.blockTimestamp || block.hash.toLowerCase() !== facts.blockHash ||
      tx.from.toLowerCase() !== facts.from || (tx.to ? tx.to.toLowerCase() : null) !== facts.to || (receipt.contractAddress ? receipt.contractAddress.toLowerCase() : null) !== facts.contractAddress ||
      String(tx.value) !== facts.valueRaw || keccak256(tx.data).toLowerCase() !== facts.inputHash ||
      (expectedTo === null ? tx.to !== null : tx.to?.toLowerCase() !== expectedTo) || (expectedFrom && tx.from.toLowerCase() !== expectedFrom) ||
      (expectedContractAddress === null ? receipt.contractAddress !== null : receipt.contractAddress?.toLowerCase() !== expectedContractAddress)) fail('live receipt/block lineage mismatch');
  if (eventCheck) eventCheck(receipt.logs);
}
const PROJECT_STORAGE_FIELDS = Object.freeze([
  'projectId','projectToken','paymentToken','projectOwner','projectProceedsReceiver','projectProceedsReceiverAttestationDigest','projectProceedsReceiverAttestationVersion',
  'platformFeeReceiver','platformFeeReceiverAttestationDigest','platformFeeReceiverAttestationVersion','projectTokenDecimals','paymentTokenDecimals','platformFeeBps','status','terminalReason',
  'projectTokenPolicyVersion','paymentTokenPolicyVersion','startTime','endTime','saleTokenAmount','softCap','minContribution','maxContribution','totalRaised','totalTokensDeposited',
  'totalTokensClaimed','participantCount','finalTotalRaised','finalPriceNumerator','finalPriceDenominator','settlementBlockNumber','settlementAnchorBlockNumber','settlementAnchorBlockHash',
  'settlementTxHash','settlementTxLogOrdinal','releaseManifestDigest','projectTokenCodeHash','projectTokenPolicyDigest','paymentTokenCodeHash','paymentTokenPolicyDigest','releasePolicyDigest',
  'releasePolicyVersion','projectProceedsWithdrawn','platformFeeWithdrawn','failedProjectTokensWithdrawn',
]);
const PROJECT_ADDRESS_FIELDS = new Set(['projectToken','paymentToken','projectOwner','projectProceedsReceiver','platformFeeReceiver']);
const PROJECT_HASH_FIELDS = new Set(['projectProceedsReceiverAttestationDigest','platformFeeReceiverAttestationDigest','settlementAnchorBlockHash','settlementTxHash','releaseManifestDigest','projectTokenCodeHash','projectTokenPolicyDigest','paymentTokenCodeHash','paymentTokenPolicyDigest','releasePolicyDigest']);
const PROJECT_BOOL_FIELDS = new Set(['projectProceedsWithdrawn','platformFeeWithdrawn','failedProjectTokensWithdrawn']);
function normalizedProjectStorage(value) {
  const out = {};
  for (const field of PROJECT_STORAGE_FIELDS) {
    const current = value?.[field];
    if (PROJECT_ADDRESS_FIELDS.has(field) || PROJECT_HASH_FIELDS.has(field)) out[field] = String(current || '').toLowerCase();
    else if (PROJECT_BOOL_FIELDS.has(field)) out[field] = Boolean(current);
    else out[field] = String(current);
  }
  return out;
}
function validatePristineProjectStorage(storage, row) {
  exactKeys(storage, PROJECT_STORAGE_FIELDS, 'pristine project storage');
  for (const field of PROJECT_ADDRESS_FIELDS) if (!ADDRESS_PATTERN.test(storage[field] || '')) fail('pristine project address is invalid');
  for (const field of PROJECT_HASH_FIELDS) if (!HASH_PATTERN.test(storage[field] || '')) fail('pristine project hash is invalid');
  for (const field of PROJECT_STORAGE_FIELDS) if (!PROJECT_ADDRESS_FIELDS.has(field) && !PROJECT_HASH_FIELDS.has(field) && !PROJECT_BOOL_FIELDS.has(field) && !/^(?:0|[1-9]\d*)$/.test(storage[field] || '')) fail('pristine project numeric field is invalid');
  if (storage.projectId !== row.projectId || storage.projectToken !== row.projectToken || storage.paymentToken !== row.paymentToken || storage.projectOwner !== row.projectOwner ||
      storage.projectProceedsReceiver !== row.projectProceedsReceiver || storage.platformFeeReceiver !== row.platformFeeReceiver || storage.startTime !== row.startTime || storage.endTime !== row.endTime ||
      storage.saleTokenAmount !== row.saleTokenAmountRaw || storage.softCap !== row.softCapRaw || storage.minContribution !== row.minContributionRaw || storage.maxContribution !== row.maxContributionRaw ||
      storage.totalTokensDeposited !== row.totalTokensDepositedRaw || storage.platformFeeBps !== String(row.platformFeeBps) || storage.status !== String(row.statusCode) || storage.terminalReason !== String(row.terminalReasonCode)) fail('pristine project storage cross-link mismatch');
  for (const field of ['totalRaised','totalTokensClaimed','participantCount','finalTotalRaised','finalPriceNumerator','finalPriceDenominator','settlementBlockNumber','settlementAnchorBlockNumber','settlementTxLogOrdinal']) if (storage[field] !== '0') fail(`pristine project ${field} is not zero`);
  for (const field of ['settlementAnchorBlockHash','settlementTxHash','releaseManifestDigest']) if (storage[field] !== `0x${'0'.repeat(64)}`) fail(`pristine project ${field} is not empty`);
  for (const field of PROJECT_BOOL_FIELDS) if (storage[field] !== false) fail(`pristine project ${field} is not false`);
}
async function verifyLiveFixture(rpcUrl, p) {
  const { Contract, Interface, JsonRpcProvider } = require('ethers');
  const provider = new JsonRpcProvider(rpcUrl, 31337, { staticNetwork: true });
  const viewsAbi = artifactAbi('contracts/fairlaunch/JoinCareFairLaunchV1ViewsFacet.sol/JoinCareFairLaunchV1ViewsFacet.json');
  const coreAbi = artifactAbi('contracts/fairlaunch/JoinCareFairLaunchV1CoreFacet.sol/JoinCareFairLaunchV1CoreFacet.json');
  const tokenAbi = artifactAbi('contracts/mocks/FairLaunchTokenMock.sol/FairLaunchTokenMock.json');
  const safeAbi = artifactAbi('contracts-local-e2e/FairLaunchSafeExecutor.sol/FairLaunchSafeExecutor.json');
  const proxy = new Contract(p.contracts.proxy.address, [...viewsAbi, 'function owner() view returns(address)'], provider);
  if ((await proxy.owner()).toLowerCase() !== p.roles.governance || (await proxy.operator()).toLowerCase() !== p.roles.operator || (await proxy.pauseGuardian()).toLowerCase() !== p.roles.pauseGuardian || (await proxy.unpauseTimelock()).toLowerCase() !== p.roles.unpauseTimelock || (await proxy.treasury()).toLowerCase() !== p.roles.treasury) fail('live role/storage mismatch');
  const fee = await proxy.platformFeeConfig();
  if (Number(fee.feeBps) !== 250 || fee.receiver.toLowerCase() !== p.safes.platformFee.address) fail('live platform fee mismatch');
  for (const safeRow of [p.safes.proceeds,p.safes.platformFee]) {
    const safe = new Contract(safeRow.address, safeAbi, provider);
    if (Number(await safe.threshold()) !== safeRow.threshold) fail('live Safe threshold mismatch');
    const owners=[]; for(let i=0;i<3;i++) owners.push((await safe.owners(i)).toLowerCase());
    if (JSON.stringify(owners)!==JSON.stringify(p.safes.owners)) fail('live Safe owners mismatch');
  }
  for (const tokenRow of [p.assets.paymentToken,...p.assets.projectTokens]) {
    const token = new Contract(tokenRow.address, tokenAbi, provider);
    if ((await token.symbol()) !== tokenRow.symbol || Number(await token.decimals()) !== tokenRow.decimals) fail('live token metadata mismatch');
  }
  const eventInterface = new Interface(coreAbi);
  for (const row of p.projects) {
    const live = await proxy.getProject(row.projectId);
    if (canonicalJson(normalizedProjectStorage(live)) !== canonicalJson(row.pristineStorage)) fail(`live project ${row.projectId} mismatch`);
    await verifyReceipt(provider,row.creation,p.contracts.proxy.address,p.roles.operator,null,(logs)=>{
      const found=logs.map((log)=>{try{const parsed=eventInterface.parseLog(log);return parsed?.name==='FairLaunchProjectCreated'?{parsed,log}:null;}catch{return null;}}).filter(Boolean);
      if(found.length!==1||found[0].log.address.toLowerCase()!==p.contracts.proxy.address||found[0].log.index!==row.creationEventLogIndex)fail('project creation event identity mismatch');
      const args=found[0].parsed.args;
      const expected={projectId:row.projectId,projectToken:row.projectToken,paymentToken:row.paymentToken,projectOwner:row.projectOwner,projectProceedsReceiver:row.projectProceedsReceiver,projectProceedsReceiverAttestationDigest:row.pristineStorage.projectProceedsReceiverAttestationDigest,projectProceedsReceiverAttestationVersion:row.pristineStorage.projectProceedsReceiverAttestationVersion,platformFeeReceiver:row.platformFeeReceiver,platformFeeReceiverAttestationDigest:row.pristineStorage.platformFeeReceiverAttestationDigest,platformFeeReceiverAttestationVersion:row.pristineStorage.platformFeeReceiverAttestationVersion,platformFeeBps:String(row.platformFeeBps),saleTokenAmount:row.saleTokenAmountRaw,softCap:row.softCapRaw,minContribution:row.minContributionRaw,maxContribution:row.maxContributionRaw,startTime:row.startTime,endTime:row.endTime,projectTokenPolicyDigest:row.pristineStorage.projectTokenPolicyDigest,projectTokenPolicyVersion:row.pristineStorage.projectTokenPolicyVersion,paymentTokenPolicyDigest:row.pristineStorage.paymentTokenPolicyDigest,paymentTokenPolicyVersion:row.pristineStorage.paymentTokenPolicyVersion,releasePolicyDigest:row.pristineStorage.releasePolicyDigest,releasePolicyVersion:row.pristineStorage.releasePolicyVersion};
      for(const [key,value] of Object.entries(expected)){const actual=typeof args[key]==='string'?args[key].toLowerCase():String(args[key]);if(actual!==String(value).toLowerCase())fail(`project creation event ${key} mismatch`);}
    });
  }
  await verifyReceipt(provider,p.deployment.proxy,null,p.roles.governance,p.contracts.proxy.address,null);
  await verifyReceipt(provider,p.deployment.observation,p.roles.browserSeedAccount,p.roles.browserSeedAccount,null,null);
  const observation=await provider.getBlock(Number(p.deployment.observation.blockNumber));
  const latest=await provider.getBlock('latest');
  if(!observation||!latest||observation.hash.toLowerCase()!==p.deployment.observation.blockHash||String(observation.timestamp)!==p.deployment.observation.blockTimestamp||latest.number!==observation.number||latest.hash.toLowerCase()!==observation.hash.toLowerCase()||String(latest.timestamp)!==String(observation.timestamp))fail('pristine observation tip mismatch');
}
async function strictRuntimeAdmission(paths, pinPath, expectedPinSha256, expectedManifestSha256, expectedRunId, expectedRpcUrl, network, options = {}) {
  const manifestSnapshot = openAdmittedManifestSnapshot(paths.manifest, expectedManifestSha256);
  let admitted = false;
  try {
  const envelope = manifestSnapshot.value;
  const trust = verifyPublicTrustAuthority(paths, expectedRunId, expectedRpcUrl, network.genesisHash);
  const pin = verifyExternalTrustPin(paths, pinPath, expectedPinSha256, expectedRunId, expectedRpcUrl, network.genesisHash, trust);
  const verified = verifyManifestSignatureIntegrity(envelope, trust.publicKeyJwk, trust.keyId);
  const p = envelope.payload;
  exactKeys(p, ['schemaVersion','authority','runId','createdAt','expiresAt','network','contracts','assets','safes','projects','roles','deployment','source'], 'manifest payload');
  if (p.schemaVersion !== MANIFEST_SCHEMA || p.authority !== AUTHORITY || p.runId !== expectedRunId) fail('manifest payload identity mismatch');
  exactKeys(p.network, ['name','chainId','rpcUrl','genesisHash'], 'manifest network');
  if (p.network.name !== 'hardhat-local-persistent' || p.network.chainId !== 31337 || p.network.rpcUrl !== expectedRpcUrl || p.network.genesisHash !== network.genesisHash) fail('manifest network identity mismatch');
  const created = Date.parse(p.createdAt), expires = Date.parse(p.expiresAt), now = options.nowMs ?? Date.now();
  if (!Number.isFinite(created) || !Number.isFinite(expires) || expires <= created || expires - created > MAX_LIFETIME_MS || now < created || now >= expires || created < Date.parse(pin.createdAt)) fail('manifest validity window is invalid');
  const contractKeys = ['proxy','dispatcher','coreFacet','releaseFacet','fundsFacet','adminFacet','viewsFacet','logicLibrary','runtimeLibrary'];
  exactKeys(p.contracts, contractKeys, 'manifest contracts');
  for (const key of contractKeys) addressIdentity(p.contracts[key], `manifest contract ${key}`);
  exactKeys(p.assets, ['paymentToken','projectTokens','faucet'], 'manifest assets');
  addressIdentity(p.assets.paymentToken, 'payment token', ['symbol','decimals']);
  if (p.assets.paymentToken.symbol !== 'tUSDT' || p.assets.paymentToken.decimals !== 6) fail('payment token metadata mismatch');
  exactKeys(p.assets.faucet, ['strategy','nativeGasStrategy','storesPrivateKeys'], 'faucet');
  if (p.assets.faucet.strategy !== 'unrestricted-token-mint' || p.assets.faucet.nativeGasStrategy !== 'frontend-local-hardhat_setBalance' || p.assets.faucet.storesPrivateKeys !== false) fail('faucet strategy mismatch');
  const expectedProjects = { failed: '101', successful: '102', cancelled: '103' }, symbols = { failed: 'BSFAIL', successful: 'BSPASS', cancelled: 'BSCAN' };
  if (!Array.isArray(p.assets.projectTokens) || p.assets.projectTokens.length !== 3 || !Array.isArray(p.projects) || p.projects.length !== 3) fail('project collection mismatch');
  const tokens = new Map();
  for (const token of p.assets.projectTokens) { addressIdentity(token, 'project token', ['key','symbol','decimals']); if (!(token.key in expectedProjects) || tokens.has(token.key) || token.symbol !== symbols[token.key] || token.decimals !== 18) fail('project token identity mismatch'); tokens.set(token.key, token); }
  exactKeys(p.safes, ['proceeds','platformFee','owners'], 'safes');
  addressIdentity(p.safes.proceeds, 'proceeds safe', ['threshold']); addressIdentity(p.safes.platformFee, 'platform fee safe', ['threshold']);
  if (p.safes.proceeds.threshold !== 2 || p.safes.platformFee.threshold !== 2 || !Array.isArray(p.safes.owners) || p.safes.owners.length !== 3 || new Set(p.safes.owners).size !== 3 || p.safes.owners.some((a) => !ADDRESS_PATTERN.test(a))) fail('Safe threshold/owners mismatch');
  exactKeys(p.roles, ['governance','operator','pauseGuardian','unpauseTimelock','treasury','browserSeedAccount'], 'roles');
  if (new Set(Object.values(p.roles)).size !== 6 || Object.values(p.roles).some((a) => !ADDRESS_PATTERN.test(a))) fail('role identities mismatch');
  const seenProjects = new Set();
  const receiptHashes = new Set([p.deployment?.proxy?.txHash, p.deployment?.observation?.txHash]);
  const placement = (facts) => [BigInt(facts.blockNumber), BigInt(facts.transactionIndex)];
  const beforePlacement = (a, b) => a[0] < b[0] || (a[0] === b[0] && a[1] < b[1]);
  for (const project of p.projects) {
    exactKeys(project, ['key','projectId','projectToken','paymentToken','projectOwner','projectProceedsReceiver','platformFeeReceiver','platformFeeBps','startTime','endTime','saleTokenAmountRaw','softCapRaw','minContributionRaw','maxContributionRaw','totalTokensDepositedRaw','creation','creationEventLogIndex','initialStatus','statusCode','terminalReasonCode','pristineStorage'], 'project');
    if (!(project.key in expectedProjects) || seenProjects.has(project.key) || project.projectId !== expectedProjects[project.key] || project.projectToken !== tokens.get(project.key)?.address || project.paymentToken !== p.assets.paymentToken.address || project.projectProceedsReceiver !== p.safes.proceeds.address || project.platformFeeReceiver !== p.safes.platformFee.address || project.platformFeeBps !== 250 || !ADDRESS_PATTERN.test(project.projectOwner || '')) fail('project cross-link mismatch');
    if (project.saleTokenAmountRaw !== '1000000000000000000000000' || project.totalTokensDepositedRaw !== project.saleTokenAmountRaw || project.softCapRaw !== (project.key === 'failed' ? '100000000000' : '100000000') || project.minContributionRaw !== '1000000' || project.maxContributionRaw !== '0' || project.initialStatus !== 'active' || project.statusCode !== 1 || project.terminalReasonCode !== 0 || !/^\d+$/.test(project.startTime) || !/^\d+$/.test(project.endTime) || BigInt(project.endTime) - BigInt(project.startTime) !== 43200n) fail('project financial/timing mismatch');
    if (!Number.isSafeInteger(project.creationEventLogIndex) || project.creationEventLogIndex < 0) fail('project creation event log index is invalid');
    receiptIdentity(project.creation, 'project creation');
    if (project.creation.from !== p.roles.operator || project.creation.to !== p.contracts.proxy.address || project.creation.contractAddress !== null) fail('project creation transaction identity mismatch');
    if (receiptHashes.has(project.creation.txHash)) fail('fixture receipt transaction hash is not unique'); receiptHashes.add(project.creation.txHash);
    validatePristineProjectStorage(project.pristineStorage, project); seenProjects.add(project.key);
  }
  exactKeys(p.deployment, ['proxy','observation'], 'deployment'); receiptIdentity(p.deployment.proxy, 'proxy deployment'); receiptIdentity(p.deployment.observation, 'observation transaction');
  if (p.deployment.proxy.from !== p.roles.governance || p.deployment.proxy.to !== null || p.deployment.proxy.contractAddress !== p.contracts.proxy.address ||
      p.deployment.observation.from !== p.roles.browserSeedAccount || p.deployment.observation.to !== p.roles.browserSeedAccount || p.deployment.observation.contractAddress !== null ||
      p.deployment.observation.valueRaw !== '0' || p.deployment.observation.inputHash !== keccak256('0x').toLowerCase() || p.deployment.proxy.valueRaw !== '0' || p.deployment.proxy.inputHash === keccak256('0x').toLowerCase() ||
      p.deployment.proxy.txHash === p.deployment.observation.txHash || p.projects.some((row) => row.creation.txHash === p.deployment.observation.txHash) ||
      !beforePlacement(placement(p.deployment.proxy), placement(p.deployment.observation)) || p.projects.some((row) => !beforePlacement(placement(p.deployment.proxy), placement(row.creation)) || !beforePlacement(placement(row.creation), placement(p.deployment.observation)) || row.startTime !== p.deployment.observation.blockTimestamp)) fail('deployment observation mismatch');
  exactKeys(p.source, ['commit','sourceTreeDigest','aggregateAbiDigest','selectorMapDigest','storageLayoutDigest','fullSetDigest','dirty'], 'source');
  if (!/^[0-9a-f]{40}$/.test(p.source.commit || '') || ![p.source.sourceTreeDigest,p.source.aggregateAbiDigest,p.source.selectorMapDigest,p.source.storageLayoutDigest,p.source.fullSetDigest].every((v) => HASH_PATTERN.test(v || '')) || typeof p.source.dirty !== 'boolean') fail('source identity mismatch');
  const identities = [...contractKeys.map((k) => p.contracts[k].address), p.assets.paymentToken.address, ...p.assets.projectTokens.map((x) => x.address), p.safes.proceeds.address, p.safes.platformFee.address, ...p.safes.owners, ...Object.values(p.roles)];
  if (new Set(identities).size !== identities.length || new Set(p.projects.map((x) => x.projectOwner)).size !== 3 || p.projects.some((x) => identities.includes(x.projectOwner))) fail('manifest address identities are not unique');
  if (options.verifyCode !== false) for (const item of [...contractKeys.map((k) => [k,p.contracts[k]]), ['paymentToken',p.assets.paymentToken], ...p.assets.projectTokens.map((x) => [`projectToken.${x.key}`,x]), ['safe.proceeds',p.safes.proceeds], ['safe.platformFee',p.safes.platformFee]]) {
    const code = await rpcCall(expectedRpcUrl, 'eth_getCode', [item[1].address, 'latest']);
    if (code === '0x' || keccak256(code).toLowerCase() !== item[1].runtimeCodeHash) fail(`${item[0]} live code mismatch`);
  }
  if (options.verifyLive !== false) await verifyLiveFixture(expectedRpcUrl, p);
  const finalSnapshot = stableCanonicalJsonSnapshot(paths.manifest, expectedManifestSha256, options.beforeManifestFinalCheck);
  if (!sameFileIdentity(manifestSnapshot.stat, finalSnapshot.stat) || manifestSnapshot.bytes !== finalSnapshot.bytes) { try { fs.closeSync(manifestSnapshot.fd); } catch (_) {} fail('manifest identity changed during admission'); }
  admittedManifestSnapshots.add(manifestSnapshot);
  admitted = true;
  return { ...verified, manifestSha256: manifestSnapshot.sha256, manifestSnapshot };
  } finally {
    if (!admitted) { try { fs.closeSync(manifestSnapshot.fd); } catch (_) {} }
  }
}
function signManifestPayload(payload, privateKey, trustRecord) {
  const bytes = canonicalJson(payload), payloadSha256 = sha256(bytes);
  const envelope = { payload, signature: { algorithm: 'ed25519', keyId: trustRecord.keyId, payloadSha256, value: crypto.sign(null, Buffer.from(bytes), privateKey).toString('base64') }, testTrustRoot: { authority: AUTHORITY, keyId: trustRecord.keyId, publicKeyJwk: trustRecord.publicKeyJwk } };
  verifyManifestSignatureIntegrity(envelope, trustRecord.publicKeyJwk, trustRecord.keyId);
  return envelope;
}
function noPrivateMaterial(target) {
  const text = fs.readFileSync(target, 'utf8');
  if (/private\s*key\s*[:=]|BEGIN\s+(?:RSA |EC |OPENSSH )?PRIVATE KEY|Account\s+#\d+.*Private Key/i.test(text)) fail(`${path.basename(target)} contains private-key material`);
}

module.exports = {
  ADDRESS_PATTERN, AUTHORITY, DEFAULT_STATE_ROOT, HASH_PATTERN, LOCAL_ENV_KEYS, MANIFEST_SCHEMA, PIN_SCHEMA, PROCESS_SCHEMA, REPOSITORY_ROOT, ROOT_SCHEMA, TRUST_SCHEMA,
  absoluteStateRoot, assertAdmittedManifestCurrent, canonical, canonicalJson, createCanonicalJsonOnce, createExternalTrustPin, createPublicTrustAuthority, ensureBaseDirectory, ensurePrivateDirectory, exactExternalPinPath, exactPinSha256, exactRpcUrl, exactRunId, noPrivateMaterial,
  parseArgs, parseJsonNoDuplicateKeys, processSnapshot, readStrictJson, recordedIdentityPresent, rejectForbiddenEnvironment, rootAuthority, rpcCall, runPaths,
  sameProcess, sanitizeDiagnostic, sha256, signManifestPayload, stableCanonicalJsonSnapshot, stopVerifiedProcess, strictRuntimeAdmission, tcpProbe, validateExternalPinPathSyntax, normalizedProjectStorage, verifyExternalTrustPin, verifyLiveFixture, verifyManifestSignatureIntegrity, verifyProcessAuthority, verifyPublicTrustAuthority, verifyRoot, verifyRpc, waitForClosed, waitForRecordedIdentitiesGone, waitForRpc, writeOnceJson, writeReplaceJson,
};
