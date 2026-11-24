'use strict';

const childProcess = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const local = require('./fairLaunchPersistentLocalE2E');

function windowsCommandLineArg(value) {
  const text = String(value);
  if (!/[\s"]/u.test(text)) return text;
  let out = '"', backslashes = 0;
  for (const c of text) {
    if (c === '\\') { backslashes += 1; continue; }
    if (c === '"') { out += '\\'.repeat(backslashes * 2 + 1) + '"'; backslashes = 0; continue; }
    out += '\\'.repeat(backslashes) + c; backslashes = 0;
  }
  return out + '\\'.repeat(backslashes * 2) + '"';
}
function windowsArgumentList(values) { return values.map(windowsCommandLineArg).join(' '); }

async function main() {
  local.rejectForbiddenEnvironment();
  const args = local.parseArgs(process.argv.slice(2), ['--run-id','--rpc-url','--state-root']);
  const runId = local.exactRunId(args['--run-id']);
  const rpc = local.exactRpcUrl(args['--rpc-url']);
  const paths = local.runPaths(runId, args['--state-root']);
  local.ensureBaseDirectory(paths.stateRoot);
  if (fs.existsSync(paths.runDir)) throw new Error('Fair Launch persistent LOCAL-ONLY: run directory already exists');
  local.ensurePrivateDirectory(paths.runDir, true);
  local.writeOnceJson(paths.rootAuthority, local.rootAuthority(runId, rpc.url, paths.stateRoot));
  const supervisor = path.join(local.REPOSITORY_ROOT, 'scripts', 'local', 'fairLaunchLocalE2ESupervisor.js');
  let launchedPid;
  if (process.platform === 'win32') {
    const powershell = path.join(process.env.SystemRoot || 'C:\\Windows', 'System32', 'WindowsPowerShell', 'v1.0', 'powershell.exe');
    const command = '& { param($encoded); $spec=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($encoded))|ConvertFrom-Json; $p=Start-Process -FilePath $spec.exe -ArgumentList $spec.argumentList -WorkingDirectory $spec.cwd -WindowStyle Hidden -PassThru; $p.Id }';
    const argumentList = windowsArgumentList([supervisor,'--run-id',runId,'--rpc-url',rpc.url,'--state-root',paths.stateRoot]);
    const encoded = Buffer.from(JSON.stringify({ exe: process.execPath, argumentList, cwd: local.REPOSITORY_ROOT }), 'utf8').toString('base64');
    const launch = childProcess.spawnSync(powershell, ['-NoProfile','-NonInteractive','-Command',command,encoded], { encoding: 'utf8', windowsHide: true, timeout: 10000 });
    launchedPid = Number(String(launch.stdout || '').trim());
    if (launch.status !== 0 || !Number.isSafeInteger(launchedPid) || launchedPid < 1) throw new Error(`Fair Launch persistent LOCAL-ONLY: supervisor launch failed: ${String(launch.stderr || '').trim()}`);
  } else {
    const child = childProcess.spawn(process.execPath, [supervisor, '--run-id', runId, '--rpc-url', rpc.url, '--state-root', paths.stateRoot], {
      cwd: local.REPOSITORY_ROOT, env: process.env, detached: true, stdio: 'ignore',
    });
    launchedPid = child.pid;
    child.unref();
  }
  const deadline = Date.now() + 60000;
  while (Date.now() < deadline) {
    if (fs.existsSync(paths.failure)) throw new Error(`Fair Launch persistent LOCAL-ONLY: ${JSON.stringify(local.readStrictJson(paths.failure))}`);
    if (fs.existsSync(paths.ready) && fs.existsSync(paths.processAuthority)) {
      const record = local.verifyProcessAuthority(paths, runId, rpc.url);
      if (record.supervisor.pid !== launchedPid) throw new Error('Fair Launch persistent LOCAL-ONLY: launched supervisor identity mismatch');
      const network = await local.verifyRpc(rpc.url);
      local.noPrivateMaterial(paths.log);
      process.stdout.write(`${JSON.stringify({ status: 'ready', runId, rpcUrl: rpc.url, chainId: 31337, genesisHash: network.genesisHash, supervisorPid: record.supervisor.pid, stateDirectory: paths.runDir })}\n`);
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error('Fair Launch persistent LOCAL-ONLY: start readiness timed out; retained run evidence');
}

if (require.main === module) main().catch((error) => { process.stderr.write(`${error.message || error}\n`); process.exitCode = 1; });
module.exports = { windowsCommandLineArg, windowsArgumentList };
