'use strict';
// Structural sanitizer only; output cannot establish trust or admission.
const fs = require('node:fs');
const local = require('./fairLaunchLocalE2E');
function main() {
  const input = process.argv[2] ? fs.readFileSync(process.argv[2], 'utf8') : fs.readFileSync(0, 'utf8');
  const descriptor = local.sanitizePublicDescriptor(local.parseExactJson(input, 'public descriptor'));
  process.stdout.write(local.canonicalJson({ status: local.UNADMITTED_STATUS, descriptor }));
}
try { main(); } catch (error) { console.error(error && error.message ? error.message : 'public descriptor sanitization failed'); process.exitCode = 1; }
