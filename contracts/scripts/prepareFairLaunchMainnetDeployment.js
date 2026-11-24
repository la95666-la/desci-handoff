'use strict';

// Produces review evidence only. There is intentionally no network or signer.
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const childProcess = require('node:child_process');
const { buildUnsignedDeploymentBundle, parseJsonNoDuplicateKeys } = require('./lib/fairLaunchUnsignedDeploymentBundle');

function regularFile(file, label) {
  const resolved = path.resolve(file);
  const stat = fs.lstatSync(resolved);
  if (!stat.isFile() || stat.isSymbolicLink()) throw new Error(`${label} must be a regular non-symlink file`);
  return resolved;
}
async function main() {
  if (process.argv.length !== 4) throw new Error('usage: npm run prepare:fair-launch:mainnet -- <input.json> <output.json>');
  const inputPath = regularFile(process.argv[2], 'deployment input');
  const outputPath = path.resolve(process.argv[3]);
  if (fs.existsSync(outputPath) || fs.existsSync(`${outputPath}.sha256`)) throw new Error('deployment output already exists');
  const status = childProcess.execFileSync('git', ['status', '--porcelain=v1'], { cwd: path.resolve(__dirname, '..'), encoding: 'utf8' });
  if (status !== '') throw new Error('deployment bundle requires a clean contract worktree');
  const input = parseJsonNoDuplicateKeys(fs.readFileSync(inputPath, 'utf8'));
  const bundle = await buildUnsignedDeploymentBundle(input);
  const bytes = Buffer.from(`${JSON.stringify(bundle)}\n`);
  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
  fs.writeFileSync(outputPath, bytes, { flag: 'wx', mode: 0o600 });
  const sha256 = crypto.createHash('sha256').update(bytes).digest('hex');
  fs.writeFileSync(`${outputPath}.sha256`, `${sha256}  ${path.basename(outputPath)}\n`, { flag: 'wx', mode: 0o600 });
  process.stdout.write(`${JSON.stringify({ status: bundle.status, broadcastPermitted: bundle.broadcastPermitted, bundleDigest: bundle.bundleDigest, sha256, outputPath })}\n`);
}
main().catch((error) => { console.error(error.message); process.exitCode = 1; });
