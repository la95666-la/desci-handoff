'use strict';

const local = require('./fairLaunchPersistentLocalE2E');

async function main() {
  local.rejectForbiddenEnvironment();
  const args = local.parseArgs(process.argv.slice(2), ['--run-id','--rpc-url','--state-root']);
  const runId = local.exactRunId(args['--run-id']);
  const rpc = local.exactRpcUrl(args['--rpc-url']);
  const paths = local.runPaths(runId, args['--state-root']);
  await local.stopVerifiedProcess(paths, runId, rpc.url);
  process.stdout.write(`${JSON.stringify({ status: 'stopped', runId, rpcUrl: rpc.url, endpointClosed: true, evidenceRetained: paths.runDir })}\n`);
}

main().catch((error) => { process.stderr.write(`${error.message || error}\n`); process.exitCode = 1; });
