'use strict';
const assert=require('assert');
const fs=require('fs');
const path=require('path');
const crypto=require('crypto');
const root=path.resolve(__dirname,'../..');
const files=[
 '2026_09_08_fair_launch_local_e2e_binding.sql',
 '2026_09_09_fair_launch_financial_projection_v3.sql',
 '2026_09_10_fair_launch_governance_projection_v3.sql',
 '2026_09_11_fair_launch_effect_lineage_v3.sql',
 '2026_09_12_fair_launch_typed_ledger_v3.sql',
];
const sql=Object.fromEntries(files.map(f=>[f,fs.readFileSync(path.join(root,'migrations',f),'utf8')]));
const all=Object.values(sql).join('\n');
const tables=[
 'fair_launch_local_e2e_run','fair_launch_local_e2e_observation','fair_launch_source_revision_binding_v3','fair_launch_project_binding_v3',
 'fair_launch_project_state_v3','fair_launch_position_v3','fair_launch_governance_current_v3','fair_launch_governance_history_v3',
 'fair_launch_policy_current_v3','fair_launch_policy_history_v3','fair_launch_application_lineage_v3','fair_launch_reversal_serialization_guard_v3','fair_launch_effect_guard_v3',
 'fair_launch_effect_journal_v3','fair_launch_lineage_reversal_v3','fair_launch_journal_reversal_v3','fair_launch_reversal_evidence_item_v3','fair_launch_reversal_evidence_manifest_v3','fair_launch_reversal_closure_v3','fair_launch_effect_dependency_v3','fair_launch_effect_attestation_v3','fair_launch_cursor_checkpoint_v3',
 'fair_launch_ledger_entry_v3','fair_launch_source_token_binding_v3','fair_launch_governance_ledger_entry_v3',
];
for(const table of tables){assert.equal((all.match(new RegExp(`CREATE TABLE IF NOT EXISTS ${table} \\(`,'g'))||[]).length,1,`${table} definition count`);}
for(const term of ['source_application_lineage_id','last_source_revision_id','last_effect_manifest_version','placement_generation','before_payload_jcs','after_payload_jcs','fair_launch_lineage_reversal_v3','fair_launch_journal_reversal_v3','fair_launch_reversal_closure_v3','reversal_of_ledger_id','compensation_generation','predecessor_block_hash','platform_fee_reserved_raw','exact_rpc_url','mysql_server_uuid','owner_label_digest']) assert(all.includes(term),`missing ${term}`);
assert(all.includes("environment='local-e2e' AND chain_id=31337"));
assert(all.includes("exact_rpc_url REGEXP '^http://127[.]0[.]0[.]1:"));
assert(all.includes("contract_version='fair_launch_v1'"));
assert(all.includes("entry_type IN('project_token_deposited','payment_contributed','project_token_claimed','payment_refunded','project_proceeds_paid','platform_fee_paid','project_tokens_withdrawn'"));
assert(all.includes("entry_type IN('excess_tokens_recovered','excess_tokens_recovered_reversal')"));
assert(!/REFERENCES projects\(project_id\)/.test(all),'v3 relation must reference row+project identity, not project_id alone');
assert(all.includes('REFERENCES projects(id,project_id)'));
assert(all.includes('REFERENCES chain_event_application_attempt_v2(id,event_identity_id,placement_id,effect_version)'));
assert(!/MAX\s*\(\s*block_number\s*\)/i.test(all),'cursor predecessor may not be guessed with MAX(block_number)');
const m0911=sql['2026_09_11_fair_launch_effect_lineage_v3.sql'];
const tableBlock=name=>m0911.match(new RegExp(`CREATE TABLE IF NOT EXISTS ${name} \\([\\s\\S]*?\\n\\) ENGINE=InnoDB`))[0];
assert(!tableBlock('fair_launch_application_lineage_v3').includes('reversed_at'),'lineage base row must not carry mutable reversal state');
assert(!tableBlock('fair_launch_effect_journal_v3').includes('reversal_application_lineage_id'),'journal base row must not carry mutable reversal link');
const immutables=['source_token_binding_v3','ledger_v3','gov_ledger_v3','lineage_v3','journal_v3','lineage_reversal_v3','journal_reversal_v3','reversal_closure_v3','attestation_v3','gov_history_v3','policy_history_v3','observation_v3'];
for(const name of immutables){assert(all.includes(`trg_fl_${name}_no_update`));assert(all.includes(`trg_fl_${name}_no_delete`));}
assert(all.includes('trg_fl_lineage_reversal_v3_validate'));
assert(all.includes('trg_fl_reversal_closure_v3_validate'));
assert(all.includes("MESSAGE_TEXT='incomplete Fair Launch reversal closure'"));
assert(all.includes('trg_fl_ledger_v3_validate_compensation'));
assert(all.includes('trg_fl_gov_ledger_v3_validate_compensation'));
for(const trigger of ['trg_fl_lineage_v3_before_insert','trg_fl_journal_v3_set_guard','trg_fl_journal_reversal_v3_validate','trg_fl_reversal_evidence_item_v3_validate','trg_fl_reversal_evidence_manifest_v3_validate'])assert(all.includes(trigger),`missing ${trigger}`);
assert(all.includes("MESSAGE_TEXT='Fair Launch reversal closure is final'"));
assert(all.includes("NEW.entry_type=CONCAT(f.entry_type,'_reversal')"));
for(const field of ['NEW.wallet_address=BINARY f.wallet_address','NEW.counterparty_address=BINARY f.counterparty_address','NEW.token_address=BINARY f.token_address','NEW.token_decimals=f.token_decimals','NEW.amount_raw=BINARY f.amount_raw','NEW.amount_display=BINARY f.amount_display','NEW.asset_role=f.asset_role','NEW.receiver_address=BINARY f.receiver_address','NEW.project_binding_id=f.project_binding_id','NEW.source_id=f.source_id'])assert(all.includes(field),`missing exact compensation field ${field}`);
assert(all.includes('COUNT(DISTINCT reversal_of_ledger_id)'));
assert(all.includes('NOT EXISTS(SELECT 1 FROM fair_launch_ledger_entry_v3 c WHERE c.reversal_of_ledger_id=f.id'));
assert(all.includes('uk_ce_placement_lineage_generation_v3'));
assert(all.includes('FOREIGN KEY(placement_id,event_identity_id,source_revision_id,placement_generation)'));
assert(all.includes('FOR UPDATE'));
assert(all.includes('fair_launch_reversal_serialization_guard_v3'));
assert(all.includes('trg_fl_local_run_v3_immutable'));
assert(all.includes("MESSAGE_TEXT='Fair Launch local run fingerprint is immutable'"));
assert(all.includes("CHAR_LENGTH(amount_raw)<78 OR amount_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935'"));
for(const fk of ['fk_fl_project_state_lineage','fk_fl_position_v3_lineage','fk_fl_gov_current_lineage','fk_fl_gov_history_lineage','fk_fl_policy_current_lineage','fk_fl_policy_history_lineage'])assert(all.includes(fk),`missing ${fk}`);
assert(all.includes('fk_fl_source_binding_source_chain'));
assert(all.includes('fk_fl_project_binding_source_chain'));
for(const fk of ['fk_fl_lineage_source_binding','fk_fl_lineage_event_chain','fk_fl_gov_current_source_chain','fk_fl_policy_current_source_chain','fk_fl_gov_ledger_source','fk_fl_cursor_checkpoint_source_chain'])assert(all.includes(fk),`missing chain provenance ${fk}`);
for(const binding of ["r.reverses_lineage_id=f.id","r.run_row_id=f.run_row_id","r.chain_id=f.chain_id","r.source_binding_id=f.source_binding_id","r.source_id=f.source_id","r.event_identity_id=f.event_identity_id","r.source_revision_id=f.source_revision_id","r.effect_manifest_version=f.effect_manifest_version"])assert(all.includes(binding),`missing reversal binding ${binding}`);
const grants=fs.readFileSync(path.join(root,'scripts/fair-launch/local-e2e/grant-policy.js'),'utf8');
assert(!grants.includes("replace(/ WITH GRANT OPTION$/,''"),'grant option must not be normalized away');
assert(grants.includes("throw Error('unexpected WITH GRANT OPTION')"));
const jobMutable=grants.match(/const JOB_MUTABLE=`([^`]*)`/)[1].split(/\s+/);
assert(!jobMutable.includes('fair_launch_reversal_serialization_guard_v3'),'Job must not mutate reversal guard directly');
assert(!jobMutable.includes('fair_launch_source_token_binding_v3'),'Job must not mint source token admission');
for(const term of ['token_binding_id','fk_fl_gov_ledger_token_binding',"asset_role='governance_token'",'metadata_version','admission_generation','runtime_code_hash','metadata_digest'])assert(all.includes(term),`source token admission binding missing: ${term}`);
assert(all.includes('PRIMARY KEY(id)')&&all.includes('uk_fl_dependency_natural'),'dependency stable identity missing');
assert(all.includes('effect_manifest_version INT UNSIGNED NOT NULL'),'dependency effect version missing');
for(const domain of ['JC_FL_REVERSAL_EVIDENCE_V1','JC_FL_REVERSAL_EVIDENCE_ITEM_V1','JC_FL_REVERSAL_MANIFEST_V1','JC_FL_REVERSAL_CLOSURE_V1'])assert(all.includes(domain),`digest domain missing: ${domain}`);
for(const term of ['project_token_decimals','payment_token_decimals','invalid Fair Launch canonical amount display','invalid Fair Launch typed financial semantics','Fair Launch ordinary ledger requires forward lineage'])assert(all.includes(term),`typed financial contract missing ${term}`);
const manifest=fs.readFileSync(path.join(root,'migrations/FAIR_LAUNCH_SHA256SUMS'),'utf8').trim().split(/\r?\n/);
for(const file of files){const line=manifest.find(x=>x.endsWith(`*${file}`));assert(line,`missing checksum ${file}`);const actual=crypto.createHash('sha256').update(fs.readFileSync(path.join(root,'migrations',file))).digest('hex');assert.equal(line.slice(0,64),actual,`${file} checksum`);}
const migrate=fs.readFileSync(path.join(root,'scripts/cmd/migrate.sh'),'utf8');for(const file of files)assert(!migrate.includes(file));
console.log('Fair Launch LOCAL-ONLY E2E schema specification: PASS');
