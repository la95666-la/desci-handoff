'use strict';
const fs = require('fs');
const path = require('path');
const assert = require('assert');
const root = path.resolve(process.argv[2] || __dirname);
const lines = fs.readFileSync(path.join(root, 'schema.sql.txt'), 'utf8').split(/\r?\n/);
assert.equal(lines[0], 'DO_NOT_EXECUTE: FAIR LAUNCH CE-V2 / GOVERNANCE MECHANICAL DDL SPECIMEN ONLY');
assert(lines[1].startsWith('SPECIMEN_FORMAT:'));
for (const [index, line] of lines.entries()) {
  if (index > 1 && line) assert(line.startsWith('SPEC> '), `line ${index + 1}`);
}
const ddl = lines.slice(2).filter(Boolean).map(line => line.slice(6)).join('\n');
const required = [
  'chain_contract_source_revision', 'chain_ingestion_cursor',
  'source_materialization_cursor', 'source_ingestion_coverage',
  'fk_ce_placement_source_revision', 'fk_ce_conflict_first_observation',
  'fk_ce_conflict_second_observation', 'fk_ce_application_placement_identity',
  'fk_ce_application_replay_audit', 'fk_chain_tx_final_application_v2',
  'ADD COLUMN chain_id_v2', 'ADD COLUMN tx_log_ordinal_v2',
  'ce_v2_m5_gate_v', 'ce_v2_m6_final_event_gate_v',
  'governance_approval', 'fk_safe_tx_approval', 'fk_safe_nonce_canonical_tx',
  'fk_timelock_safe_tx', 'fk_timelock_target_application',
  'governance_timelock_call_evidence_gap_v', 'fk_executor_execution_application',
  'governance_convergence_gate_v', 'fk_chain_event_record_application_same_lineage_v2',
  'fk_chain_event_failure_same_lineage_v2', 'validation_query_version',
  'fund_ledger_v2_provenance_gap_v', 'correlation_calldata_digest',
  'i.chain_id=s.chain_id AND i.tx_hash=s.tx_hash', 'safe_place.canonical_slot=1', 'safe_identity.chain_id=slot.chain_id',
  'safe_identity.tx_hash=s.external_execution_tx_hash', 's.executed_safe_tx_hash=s.safe_tx_hash', 'canonical_call_payload_digest',
  'scheduled.effect_digest<>c.expected_schedule_effect_digest',
  'executed.effect_digest<>c.expected_execute_effect_digest',
  'o.recomputed_operation_id=o.operation_id',
  'o.recomputed_operation_payload_digest=o.canonical_operation_payload_digest'
];
for (const token of required) assert(ddl.includes(token), token);
assert(!/\b(DROP|TRUNCATE|DELETE\s+FROM|GRANT|REVOKE)\b/i.test(ddl));
const data = require(path.join(root, 'state-machine-vectors.json'));
assert.equal(data.schemaVersion, 4);
const requiredIds = ['identity-reinclusion','single-canonical-placement','payload-conflict','application-replay','shadow-ledger-multi-entry','shadow-ledger-reversal','m5-quarantine','m0-m5-final-reference','m5-seven-family-equivalence','m5-digest-negative-query-version','m5-digest-negative-row-digest','m5-digest-negative-aggregate-digest','m6-final-reference','m6-final-reference-negative','m6-no-fallback','source-coverage-gap','safe-same-nonce-conflict','safe-external-execution','safe-hash-negative-wrong-domain','safe-execution-reorg','timelock-operation-hash','timelock-hash-negative-reordered-calls','timelock-hash-negative-calldata','timelock-hash-negative-predecessor','timelock-hash-negative-salt','timelock-batch-missing-index','timelock-ready-time','timelock-complete-batch','timelock-partial-execution','executor-one-active','executor-crash-boundary','external-after-raw-commit','target-event-absent','governance-convergence','e8-selector-isolation'];
const ids=[...data.ceV2,...data.governance].map(x=>x.id);for(const id of requiredIds)assert(ids.includes(id),id);assert.equal(new Set(ids).size,ids.length);
for (const vector of [...data.ceV2, ...data.governance]) {
  assert(vector.initial && Array.isArray(vector.actions) && vector.actions.length && vector.expected, `${vector.id}: structured`);
}
const byId = id => [...data.ceV2, ...data.governance].find(x => x.id === id);
assert.equal(byId('safe-same-nonce-conflict').expected.winner, null);
assert.equal(byId('timelock-batch-missing-index').expected.missing[0], 1);
assert.equal(byId('m6-no-fallback').expected.sourceBlocked, true);
assert.equal(byId('executor-crash-boundary').expected.fenceHeld, true);
assert.equal(byId('target-event-absent').expected.applied, false);
assert.equal(byId('governance-convergence').expected.applied, true);
assert.equal(byId('e8-selector-isolation').expected.genericAccepted, false);
require('./state-machine-verify.js');
console.log('Fair Launch CE-V2/governance evidence: PASS');
