'use strict';

// TEST ONLY. Renders the repository-owned positive grant policy for disposable
// local identities. It is not a production privilege executor or authority.

const policy = require('../fair-launch/source-registry-privilege-policy');

const [,, mode, database, identityName] = process.argv;
if (!['grants', 'expected', 'tables'].includes(mode) || !/^[A-Za-z0-9_]+$/.test(database || '')) {
  throw new Error('invalid local database policy fixture arguments');
}

const identities = Object.freeze({
  runtime: {user: 'local_runtime', host: '%'},
  admin: {user: 'local_admin', host: '%'},
  job: {user: 'local_job', host: '%'},
  migration: {user: 'local_migration', host: 'localhost'},
  schemaAdmin: {user: 'local_schema_admin_unused', host: 'localhost'},
  auditor: {user: 'local_auditor_unused', host: '%'},
  definer: policy.DEFINER,
  ingest: {user: 'local_ingestion', host: '%'},
});

policy.assertDistinctIdentities(identities);
const exact = policy.exactGrantPolicy(database, identities);
const testedNames = Object.freeze(['runtime', 'admin', 'job', 'migration', 'definer', 'ingest']);

if (mode === 'tables') {
  const tables = new Set([
    ...policy.RUNTIME_READ_ONLY,
    ...policy.RUNTIME_MUTABLE,
    ...policy.ADMIN_MUTABLE,
    ...policy.JOB_MUTABLE,
    ...policy.AUDIT_READ_ONLY,
    policy.PROTECTED_TABLE,
    'chain_contract_source_revision',
  ]);
  process.stdout.write([...tables].sort().join('\n') + '\n');
} else if (mode === 'expected') {
  if (!testedNames.includes(identityName)) throw new Error('unknown tested local identity');
  process.stdout.write(exact[identityName].join('\n') + (exact[identityName].length ? '\n' : ''));
} else {
  const statements = [];
  for (const name of testedNames) {
    const account = policy.quoteAccount(identities[name]);
    statements.push(`REVOKE ALL PRIVILEGES, GRANT OPTION FROM ${account}`, ...exact[name]);
  }
  statements.push(`ALTER USER ${policy.quoteAccount(identities.migration)} ACCOUNT LOCK`);
  statements.push(`ALTER USER ${policy.quoteAccount(identities.definer)} ACCOUNT LOCK`);
  process.stdout.write(statements.join(';\n') + ';\n');
}
