'use strict';

const local = require('./fairLaunchPersistentLocalE2E');

async function main() {
  local.rejectForbiddenEnvironment();
  const args = local.parseArgs(process.argv.slice(2), ['--run-id','--rpc-url','--state-root','--trust-pin','--trust-pin-sha256','--manifest-sha256']);
  const runId = local.exactRunId(args['--run-id']);
  const rpc = local.exactRpcUrl(args['--rpc-url']);
  const paths = local.runPaths(runId, args['--state-root']);
  local.verifyProcessAuthority(paths, runId, rpc.url);
  const network = await local.verifyRpc(rpc.url);
  const verified = await local.strictRuntimeAdmission(paths, args['--trust-pin'], args['--trust-pin-sha256'], args['--manifest-sha256'], runId, rpc.url, network);
  local.noPrivateMaterial(paths.manifest);
  local.assertAdmittedManifestCurrent(paths.manifest, verified.manifestSnapshot);
  process.stdout.write(`${JSON.stringify({ status: 'TEST_ONLY_NOT_PRODUCTION_ADMITTED', runId, rpcUrl: rpc.url, manifest: paths.manifest, manifestSha256: verified.manifestSha256, payloadSha256: verified.payloadSha256, keyId: verified.keyId })}\n`);
}

main().catch((error) => { process.stderr.write(`${error.message || error}\n`); process.exitCode = 1; });
