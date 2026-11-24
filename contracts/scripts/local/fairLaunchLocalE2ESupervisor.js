'use strict';

const childProcess = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const local = require('./fairLaunchPersistentLocalE2E');

async function main() {
  local.rejectForbiddenEnvironment();
  const args = local.parseArgs(process.argv.slice(2), ['--run-id','--rpc-url','--state-root']);
  const runId = local.exactRunId(args['--run-id']);
  const rpc = local.exactRpcUrl(args['--rpc-url']);
  const paths = local.runPaths(runId, args['--state-root']);
  local.verifyRoot(paths, runId, rpc.url);
  const bootstrap = path.join(local.REPOSITORY_ROOT, 'node_modules', 'hardhat', 'internal', 'cli', 'bootstrap.js');
  const config = path.join(local.REPOSITORY_ROOT, 'hardhat.local-e2e.config.js');
  const node = childProcess.spawn(process.execPath, [bootstrap, 'node', '--config', config, '--hostname', '127.0.0.1', '--port', String(rpc.port)], {
    cwd: local.REPOSITORY_ROOT,
    env: { ...process.env, JC_FAIR_LAUNCH_LOCAL_E2E_RPC_URL: rpc.url },
    stdio: 'ignore',
    windowsHide: true,
  });
  let stopping = false;
  async function cleanup() {
    if (stopping) return;
    stopping = true;
    try {
      const record = local.verifyProcessAuthority(paths, runId, rpc.url);
      const immediate = local.verifyProcessAuthority(paths, runId, rpc.url);
      if (record.node.pid === node.pid && local.sameProcess(record.node, immediate.node)) node.kill('SIGTERM');
    } catch (_) {
      // Ambiguity is intentionally fail-closed: never signal an unverified PID.
    }
    const deadline = Date.now() + 15000;
    while (node.exitCode === null && node.signalCode === null && Date.now() < deadline) await new Promise((resolve) => setTimeout(resolve, 100));
    let exactChildAlive = true;
    try {
      const record = local.readStrictJson(paths.processAuthority);
      exactChildAlive = local.recordedIdentityPresent(record.node);
    } catch (_) { exactChildAlive = true; }
    let endpointClosed = false;
    try { await local.waitForClosed(rpc.url, 10000); endpointClosed = true; } catch (_) {}
    if (exactChildAlive || !endpointClosed) {
      try { local.writeReplaceJson(paths.failure, { stage: 'shutdown', exactChildAlive, endpointClosed, observedAt: new Date().toISOString() }); } catch (_) {}
      process.exit(2);
    }
    process.exit(0);
  }
  process.once('SIGTERM', cleanup);
  process.once('SIGINT', cleanup);
  node.once('exit', (code, signal) => {
    if (!stopping) {
      try { local.writeReplaceJson(paths.failure, { stage: 'node-exit', code, signal, observedAt: new Date().toISOString() }); } catch (_) {}
      process.exit(code || 1);
    }
  });
  const authority = {
    schemaVersion: local.PROCESS_SCHEMA,
    authority: local.AUTHORITY,
    runId,
    rpcUrl: rpc.url,
    chainId: 31337,
    repositoryRoot: fs.realpathSync(local.REPOSITORY_ROOT),
    runDirectory: fs.realpathSync(paths.runDir),
    supervisor: local.processSnapshot(process.pid),
    node: local.processSnapshot(node.pid),
    createdAt: new Date().toISOString(),
  };
  local.writeOnceJson(paths.processAuthority, authority);
  try {
    const network = await local.waitForRpc(rpc.url, 50000);
    local.writeOnceJson(paths.ready, { schemaVersion: 'fair-launch-local-e2e-ready-v1', authority: local.AUTHORITY, runId, rpcUrl: rpc.url, ...network, readyAt: new Date().toISOString() });
    local.writeOnceJson(paths.log, { schemaVersion: 'fair-launch-local-e2e-sanitized-log-v1', authority: local.AUTHORITY, runId, rpcUrl: rpc.url, chainId: 31337, genesisHash: network.genesisHash, note: 'Hardhat stdout/stderr intentionally discarded; deterministic account private-key banner was never retained.', readyAt: new Date().toISOString() });
  } catch (error) {
    try { local.writeReplaceJson(paths.failure, { stage: 'rpc-readiness', error: error.message, observedAt: new Date().toISOString() }); } catch (_) {}
    await cleanup();
  }
}

main().catch((error) => {
  process.stderr.write(`${error.message || error}\n`);
  process.exitCode = 1;
});
