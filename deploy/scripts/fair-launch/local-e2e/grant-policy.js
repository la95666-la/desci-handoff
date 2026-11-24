'use strict';
// TEST ONLY. Exact disposable local identity policy; never imported by production paths.
const IDENTITIES=Object.freeze({runtime:'local_runtime',admin:'local_admin',job:'local_job',ingest:'local_ingest'});
const READ_COMMON=`settings projects project_tge chain_contract_source chain_contract_source_revision fair_launch_feature_watermark fair_launch_source_registry_snapshot fair_launch_local_e2e_run fair_launch_local_e2e_observation fair_launch_source_revision_binding_v3 fair_launch_project_binding_v3 fair_launch_source_token_binding_v3 fair_launch_project_state_v3 fair_launch_position_v3 fair_launch_governance_current_v3 fair_launch_policy_current_v3 fair_launch_application_lineage_v3 fair_launch_reversal_serialization_guard_v3 fair_launch_lineage_reversal_v3 fair_launch_effect_guard_v3 fair_launch_effect_journal_v3 fair_launch_journal_reversal_v3 fair_launch_reversal_evidence_item_v3 fair_launch_reversal_evidence_manifest_v3 fair_launch_reversal_closure_v3 fair_launch_effect_dependency_v3 fair_launch_effect_attestation_v3 fair_launch_cursor_checkpoint_v3 fair_launch_ledger_entry_v3 fair_launch_governance_ledger_entry_v3 fair_launch_governance_history_v3 fair_launch_policy_history_v3`.split(/\s+/);
const APPEND_ONLY=`fair_launch_application_lineage_v3 fair_launch_lineage_reversal_v3 fair_launch_effect_journal_v3 fair_launch_journal_reversal_v3 fair_launch_reversal_evidence_item_v3 fair_launch_reversal_evidence_manifest_v3 fair_launch_reversal_closure_v3 fair_launch_effect_dependency_v3 fair_launch_effect_attestation_v3 fair_launch_cursor_checkpoint_v3 fair_launch_ledger_entry_v3 fair_launch_governance_ledger_entry_v3 fair_launch_governance_history_v3 fair_launch_policy_history_v3`.split(/\s+/);
const JOB_MUTABLE=`chain_event_application_attempt_v2 chain_event_block_checkpoint chain_event_failure chain_event_identity_quarantine_v2 chain_event_payload_conflict_v2 chain_event_placement_observation_v2 chain_event_placement_v2 chain_event_record chain_event_sync_state fair_launch_effect_guard_v3 fair_launch_governance_current_v3 fair_launch_policy_current_v3 fair_launch_position_v3 fair_launch_project_state_v3 source_materialization_cursor`.split(/\s+/);
const INGEST_MUTABLE=`chain_event_block_checkpoint chain_event_failure chain_event_identity_quarantine_v2 chain_event_identity_v2 chain_event_payload_conflict_v2 chain_event_placement_observation_v2 chain_event_placement_v2 chain_event_record chain_event_sync_state chain_ingestion_cursor source_ingestion_coverage`.split(/\s+/);
const q=s=>`\`${s.replace(/`/g,'``')}\``;
const account=role=>`'${IDENTITIES[role]}'@'%'`;
function policy(db){
 const on=(priv,tables,role)=>tables.map(t=>`GRANT ${priv} ON ${q(db)}.${q(t)} TO ${account(role)}`);
 return Object.freeze({
  runtime:on('SELECT',READ_COMMON,'runtime'),
  admin:on('SELECT',READ_COMMON,'admin'),
  job:[...on('SELECT',Array.from(new Set([...READ_COMMON,...JOB_MUTABLE])),'job'),...on('INSERT',APPEND_ONLY,'job'),...on('INSERT, UPDATE',JOB_MUTABLE,'job')],
  ingest:[...on('SELECT',Array.from(new Set([...READ_COMMON,...INGEST_MUTABLE])),'ingest'),...on('INSERT, UPDATE',INGEST_MUTABLE,'ingest')],
 });
}
function normalize(lines){
 const grants=new Map(),other=[];
 for(let s of lines.map(x=>x.trim()).filter(Boolean)){s=s.replace(/ TO `([^`]+)`@`([^`]+)`$/, " TO '$1'@'$2'");if(/WITH GRANT OPTION/i.test(s))throw Error('unexpected WITH GRANT OPTION');if(s.startsWith('GRANT USAGE ON *.*'))continue;if(/^GRANT .+ ON \*\.\*/i.test(s)||/^GRANT .+ ON `?[^`.]+`?\.\*/i.test(s)||/\([^)]*\) ON /i.test(s)||/\b(ALL|SUPER|FILE|PROCESS|RELOAD|SHUTDOWN|CREATE USER|ROLE_ADMIN|SYSTEM_USER|SESSION_VARIABLES_ADMIN)\b/i.test(s))throw Error('unexpected broad or dynamic authority');const m=s.match(/^GRANT (.+) ON (.+) TO (.+)$/);if(!m){other.push(s);continue;}const key=`${m[2]} TO ${m[3]}`,set=grants.get(key)||new Set();for(const p of m[1].split(/,\s*/))set.add(p);grants.set(key,set);}
 const order=['SELECT','INSERT','UPDATE','DELETE','EXECUTE'];const rows=[];for(const [key,set] of grants)rows.push(`GRANT ${[...set].sort((a,b)=>order.indexOf(a)-order.indexOf(b)).join(', ')} ON ${key}`);return [...rows,...other].sort();
}
function statements(db){const p=policy(db),out=[];for(const role of Object.keys(IDENTITIES)){out.push(`REVOKE ALL PRIVILEGES, GRANT OPTION FROM ${account(role)}`,...p[role]);}return out;}
if(require.main===module){
 const [mode,db,role]=process.argv.slice(2);if(!/^[A-Za-z0-9_]+$/.test(db||''))throw Error('invalid database');
 if(mode==='sql')process.stdout.write(statements(db).join(';\n')+';\n');
 else if(mode==='expected'){if(!IDENTITIES[role])throw Error('invalid role');process.stdout.write(normalize(policy(db)[role]).join('\n')+'\n');}
 else if(mode==='tables')process.stdout.write(Array.from(new Set(Object.values(policy(db)).flatMap(rows=>rows.map(r=>r.match(/\.\`([^`]+)\` TO/)[1])))).sort().join('\n')+'\n');
 else throw Error('usage: grant-policy.js sql|expected|tables <database> [role]');
}
module.exports=Object.freeze({IDENTITIES,READ_COMMON,APPEND_ONLY,JOB_MUTABLE,INGEST_MUTABLE,policy,normalize,statements});
