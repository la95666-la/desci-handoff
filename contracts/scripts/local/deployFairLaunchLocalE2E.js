'use strict';

const childProcess = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const local = require('./fairLaunchPersistentLocalE2E');

function scrubEnvironment(runId, rpcUrl, stateRoot) {
  const env = {};
  for (const [key, value] of Object.entries(process.env)) {
    const upper = key.toUpperCase();
    if (upper.startsWith('JC_DEPLOY_') || upper.includes('PRIVATE_KEY') || upper.includes('MNEMONIC') || upper === 'BSCSCAN_API_KEY') continue;
    env[key] = value;
  }
  env.JC_FAIR_LAUNCH_LOCAL_E2E_RUN_ID = runId;
  env.JC_FAIR_LAUNCH_LOCAL_E2E_RPC_URL = rpcUrl;
  env.JC_FAIR_LAUNCH_LOCAL_E2E_STATE_ROOT = stateRoot;
  return env;
}
async function main() {
  local.rejectForbiddenEnvironment();
  const args = local.parseArgs(process.argv.slice(2), ['--run-id','--rpc-url','--state-root','--trust-pin']);
  const runId = local.exactRunId(args['--run-id']);
  const rpc = local.exactRpcUrl(args['--rpc-url']);
  const paths = local.runPaths(runId, args['--state-root']);
  local.verifyProcessAuthority(paths, runId, rpc.url);
  const network = await local.verifyRpc(rpc.url);
  const trustPin = local.exactExternalPinPath(args['--trust-pin'], paths);
  if (fs.existsSync(paths.manifest) || fs.existsSync(paths.trustRoot) || fs.existsSync(trustPin)) throw new Error('Fair Launch persistent LOCAL-ONLY: deployment/trust/pin authority already exists');
  const pair = crypto.generateKeyPairSync('ed25519');
  const trust = local.createPublicTrustAuthority(paths, runId, rpc.url, network.genesisHash, pair);
  const { pinSha256 } = local.createExternalTrustPin(paths, trustPin, runId, rpc.url, network.genesisHash, trust);
  const bootstrap = path.join(local.REPOSITORY_ROOT, 'node_modules', 'hardhat', 'internal', 'cli', 'bootstrap.js');
  const worker = path.join(local.REPOSITORY_ROOT, 'scripts', 'local', 'deployFairLaunchLocalE2EWorker.js');
  const privateDer = pair.privateKey.export({ type: 'pkcs8', format: 'der' });
  const signingPipe = JSON.stringify({ privateKeyPkcs8: privateDer.toString('base64') });
  privateDer.fill(0);
  const result = childProcess.spawnSync(process.execPath, [bootstrap, 'run', worker, '--config', path.join(local.REPOSITORY_ROOT, 'hardhat.local-e2e.config.js'), '--network', 'localE2E', '--no-compile'], {
    cwd: local.REPOSITORY_ROOT, env: scrubEnvironment(runId, rpc.url, paths.stateRoot), input: signingPipe, encoding: 'utf8', windowsHide: true, timeout: 180000, maxBuffer: 16 * 1024 * 1024,
  });
  if (result.error || result.status !== 0) {
    const diagnostic = local.sanitizeDiagnostic(result.stderr || result.error?.message || '');
    local.writeReplaceJson(paths.failure, { stage: 'deployment', status: result.status, error: local.sanitizeDiagnostic(result.error?.message || ''), stderr: diagnostic, observedAt: new Date().toISOString() });
    let cleanup = 'owned node stopped';
    try { await local.stopVerifiedProcess(paths, runId, rpc.url); } catch (error) { cleanup = `cleanup retained: ${local.sanitizeDiagnostic(error.message)}`; }
    throw new Error(`Fair Launch persistent LOCAL-ONLY: deployment failed (${cleanup}): ${diagnostic.trim()}`);
  }
  const publishedSnapshot = local.stableCanonicalJsonSnapshot(paths.manifest);
  const manifestSha256 = publishedSnapshot.sha256;
  const currentNetwork = await local.waitForRpc(rpc.url, 10000);
  const verified = await local.strictRuntimeAdmission(paths, trustPin, pinSha256, manifestSha256, runId, rpc.url, currentNetwork);
  local.noPrivateMaterial(paths.manifest);
  local.assertAdmittedManifestCurrent(paths.manifest, verified.manifestSnapshot);
  process.stdout.write(`${JSON.stringify({ status: 'deployed', runId, rpcUrl: rpc.url, manifest: paths.manifest, manifestSha256, trustPin, trustPinSha256: pinSha256, payloadSha256: verified.payloadSha256, projectId: verified.manifestSnapshot.value.payload.projects.find((p) => p.key === 'successful').projectId })}\n`);
}

main().catch((error) => { process.stderr.write(`${error.message || error}\n`); process.exitCode = 1; });
