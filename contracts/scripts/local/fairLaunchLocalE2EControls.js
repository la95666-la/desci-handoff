'use strict';

const fs = require('node:fs');
const local = require('./fairLaunchPersistentLocalE2E');

async function main() {
  local.rejectForbiddenEnvironment();
  const args = local.parseArgs(process.argv.slice(2), ['--run-id','--rpc-url','--state-root','--trust-pin','--trust-pin-sha256','--manifest-sha256']);
  const runId = local.exactRunId(args['--run-id']);
  const rpc = local.exactRpcUrl(args['--rpc-url']);
  const paths = local.runPaths(runId, args['--state-root']);
  const record = local.verifyProcessAuthority(paths, runId, rpc.url);
  const network = await local.verifyRpc(rpc.url);
  const ready = local.readStrictJson(paths.ready);
  if (ready.runId !== runId || ready.rpcUrl !== rpc.url || ready.genesisHash !== network.genesisHash) throw new Error('Fair Launch persistent LOCAL-ONLY: readiness identity mismatch');
  let deployment = null;
  if (fs.existsSync(paths.manifest)) {
    const verified = await local.strictRuntimeAdmission(paths, args['--trust-pin'], args['--trust-pin-sha256'], args['--manifest-sha256'], runId, rpc.url, network);
    local.assertAdmittedManifestCurrent(paths.manifest, verified.manifestSnapshot);
    deployment = { manifest: paths.manifest, manifestSha256: verified.manifestSha256, payloadSha256: verified.payloadSha256, projectId: verified.manifestSnapshot.value.payload.projects.find((p) => p.key === 'successful').projectId };
  }
  process.stdout.write(`${JSON.stringify({ status: 'running', runId, rpcUrl: rpc.url, chainId: 31337, genesisHash: network.genesisHash, supervisorPid: record.supervisor.pid, nodePid: record.node.pid, deployment })}\n`);
}

main().catch((error) => { process.stderr.write(`${error.message || error}\n`); process.exitCode = 1; });
