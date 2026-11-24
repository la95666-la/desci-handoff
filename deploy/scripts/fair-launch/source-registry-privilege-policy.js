'use strict';
// Narrow reviewed policy. Omitted objects receive no privilege; incomplete
// application wiring therefore remains fail closed instead of gaining blanket CRUD.
const PROTECTED_TABLE='fair_launch_source_registry_snapshot';
const PROCEDURE='jc_insert_fair_launch_source_registry_snapshot';
const DEFINER=Object.freeze({user:'jc_fl_registry_definer',host:'localhost'});
const INGEST=Object.freeze({user:'jc_fl_registry_ingest',host:'%'});
const AUDITOR=Object.freeze({user:'jc_fl_registry_auditor',host:'%'});
const RUNTIME_READ_ONLY=Object.freeze(`settings projects project_tge chain_contract_source chain_contract_source_revision fair_launch_feature_watermark fair_launch_position fair_launch_signing_generation source_ingestion_coverage fair_launch_source_registry_snapshot`.split(/\s+/));
const RUNTIME_MUTABLE=Object.freeze(`announcement apply_project home_banner invite_lineage invite_record invite_user_code news news_translation notification_read_time notifications project_files project_subscribers project_updates project_watches project_whitelists task task_record task_round tg_daily_activity transaction_record upload_files user user_check_in_record user_claim_record user_login_log user_note user_point user_point_flow user_project user_refund_record user_wallet`.split(/\s+/));
const ADMIN_MUTABLE=Object.freeze(`admin_approval fair_launch_action_request fair_launch_action_transition fair_launch_preview governance_approval`.split(/\s+/));
const JOB_MUTABLE=Object.freeze(`chain_event_application_attempt_v2 chain_event_block_checkpoint chain_event_failure chain_event_identity_quarantine_v2 chain_event_identity_v2 chain_event_payload_conflict_v2 chain_event_placement_observation_v2 chain_event_placement_v2 chain_event_record chain_event_sync_state chain_ingestion_cursor chain_relayer_nonce chain_tx_submission fair_launch_effect_application fair_launch_gateway_signing_journal fair_launch_position fair_launch_source_registry_binding_quarantine fund_ledger fund_ledger_v2_shadow ido_purchase_entitlement ido_reconciliation_check ido_reconciliation_event ido_reconciliation_job ido_reconciliation_manifest ido_reconciliation_run ido_reconciliation_wallet source_materialization_cursor`.split(/\s+/));
const AUDIT_READ_ONLY=Object.freeze(`schema_migrations fair_launch_source_registry_snapshot fair_launch_feature_watermark`.split(/\s+/));
const quoteIdent=value=>`\`${String(value).replaceAll('`','``')}\``;
const quoteAccount=({user,host})=>`'${String(user).replaceAll("'","''")}'@'${String(host).replaceAll("'","''")}'`;
const normalizeGrant=line=>String(line).trim().replace(/ TO `([^`]+)`@`([^`]+)`$/, " TO '$1'@'$2'").replace(/ FROM `([^`]+)`@`([^`]+)`$/, " FROM '$1'@'$2'").replace(/ WITH GRANT OPTION$/, '');
const tableGrants=(db,account,tables,privileges)=>tables.map(t=>`GRANT ${privileges} ON ${db}.${quoteIdent(t)} TO ${account}`);
const excluding=(tables,mutable)=>tables.filter(table=>!mutable.includes(table));
function exactGrantPolicy(database,identities){const db=quoteIdent(database),account=name=>quoteAccount(identities[name]);return{
 runtime:[...tableGrants(db,account('runtime'),RUNTIME_READ_ONLY,'SELECT'),...tableGrants(db,account('runtime'),RUNTIME_MUTABLE,'SELECT, INSERT, UPDATE, DELETE')].sort(),
 admin:[...tableGrants(db,account('admin'),RUNTIME_READ_ONLY,'SELECT'),...tableGrants(db,account('admin'),ADMIN_MUTABLE,'SELECT, INSERT, UPDATE, DELETE')].sort(),
 job:[...tableGrants(db,account('job'),excluding(RUNTIME_READ_ONLY,JOB_MUTABLE),'SELECT'),...tableGrants(db,account('job'),JOB_MUTABLE,'SELECT, INSERT, UPDATE, DELETE')].sort(),
 migration:[],schemaAdmin:[],
 auditor:tableGrants(db,account('auditor'),AUDIT_READ_ONLY,'SELECT').sort(),
 definer:[`GRANT SELECT, INSERT, TRIGGER ON ${db}.${quoteIdent(PROTECTED_TABLE)} TO ${account('definer')}`,`GRANT SELECT ON ${db}.${quoteIdent('chain_contract_source_revision')} TO ${account('definer')}`,`GRANT EXECUTE ON PROCEDURE ${db}.${quoteIdent(PROCEDURE)} TO ${account('definer')}`].sort(),
 ingest:[`GRANT SELECT ON ${db}.${quoteIdent(PROTECTED_TABLE)} TO ${account('ingest')}`,`GRANT EXECUTE ON PROCEDURE ${db}.${quoteIdent(PROCEDURE)} TO ${account('ingest')}`].sort(),
};}
function assertDistinctIdentities(identities){const values=Object.entries(identities).map(([name,x])=>[name,`${x.user}@${x.host}`]);if(new Set(values.map(x=>x[1])).size!==values.length)throw Error('database security identities must have exact distinct user+host');}
function grantStatements(database,identities){assertDistinctIdentities(identities);const policy=exactGrantPolicy(database,identities),out=[];for(const name of ['runtime','admin','job','migration','schemaAdmin','auditor','definer','ingest'])out.push(`REVOKE ALL PRIVILEGES, GRANT OPTION FROM ${quoteAccount(identities[name])}`,...policy[name]);for(const name of ['migration','schemaAdmin','definer'])out.push(`ALTER USER ${quoteAccount(identities[name])} ACCOUNT LOCK`);return out;}
module.exports={PROTECTED_TABLE,PROCEDURE,DEFINER,INGEST,AUDITOR,RUNTIME_READ_ONLY,RUNTIME_MUTABLE,ADMIN_MUTABLE,JOB_MUTABLE,AUDIT_READ_ONLY,quoteIdent,quoteAccount,normalizeGrant,exactGrantPolicy,grantStatements,assertDistinctIdentities};
