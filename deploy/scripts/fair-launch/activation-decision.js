'use strict';

// This release intentionally has no production activation proof. A safe proof
// requires an admitted activation-auditor identity, endpoint binding to the
// application DB, complete partial-schema detection, and a MySQL advisory lease
// held until service startup finishes. None is partially emulated here.
const ERROR = 'fair_launch_activation_unavailable';

function rejectTestAuthorityEnvironment(env = process.env) {
  for (const name of Object.keys(env)) {
    if (/^jc_fair_launch_test_/i.test(name)) throw new Error(`${name} is forbidden in production activation`);
  }
}

function decide() {
  rejectTestAuthorityEnvironment();
  throw new Error(ERROR);
}

if (require.main === module) {
  try {
    decide();
  } catch (_) {
    process.stderr.write(`${ERROR}\n`);
    process.exit(78);
  }
}

module.exports = { ERROR, rejectTestAuthorityEnvironment, decide };
