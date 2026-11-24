'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const childProcess = require('node:child_process');
const { buildPaymentTokenGovernanceBundle, parseJsonNoDuplicateKeys } = require('./lib/fairLaunchPaymentTokenGovernanceBundle');

function regularFile(file, label) {
  const resolved = path.resolve(file);
  const stat = fs.lstatSync(resolved);
  if (!stat.isFile() || stat.isSymbolicLink()) throw new Error(`${label} must be a regular non-symlink file`);
  return resolved;
}
async function main() {
  if (process.argv.length !== 4) throw new Error('usage: node scripts/prepareFairLaunchPaymentTokenGovernance.js <input.json> <output.json>');
  const repository = path.resolve(__dirname, '..');
  const inputPath = regularFile(process.argv[2], 'governance input');
  const outputPath = path.resolve(process.argv[3]);
  if (fs.existsSync(outputPath) || fs.existsSync(`${outputPath}.sha256`)) throw new Error('governance output already exists');
  const status = childProcess.execFileSync('git', ['status', '--porcelain=v1'], { cwd: repository, encoding: 'utf8' });
  if (status !== '') throw new Error('governance bundle requires a clean contract worktree');
  const sourceCommit = childProcess.execFileSync('git', ['rev-parse', 'HEAD'], { cwd: repository, encoding: 'utf8' }).trim();
  const input = parseJsonNoDuplicateKeys(fs.readFileSync(inputPath, 'utf8'), 'governance input');
  const bundle = await buildPaymentTokenGovernanceBundle(input, sourceCommit);
  const bytes = Buffer.from(`${JSON.stringify(bundle)}\n`);
  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
  fs.writeFileSync(outputPath, bytes, { flag: 'wx', mode: 0o600 });
  const sha256 = crypto.createHash('sha256').update(bytes).digest('hex');
  fs.writeFileSync(`${outputPath}.sha256`, `${sha256}  ${path.basename(outputPath)}\n`, { flag: 'wx', mode: 0o600 });
  process.stdout.write(`${JSON.stringify({ status: bundle.status, broadcastPermitted: bundle.broadcastPermitted, operationId: bundle.operationId, bundleDigest: bundle.bundleDigest, sha256, outputPath })}\n`);
}
main().catch((error) => { console.error(error.message); process.exitCode = 1; });
