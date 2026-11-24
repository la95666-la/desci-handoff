package schema

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

type requiredSchemaCheck struct {
	name string
	want int64
	sql  string
}

func primarySchemaDatabase(database *gorm.DB) *gorm.DB {
	return database.Clauses(dbresolver.Write)
}

var chainEventCollationCompatibilitySchemaChecks = []requiredSchemaCheck{
	{name: "migration ledger", want: 2, sql: `SELECT COUNT(*) FROM schema_migrations WHERE status='applied' AND (
(filename='2026_08_16_zz_chain_event_collation_compat.sql' AND checksum='1c708627f8bb643ff31660ec64fa8c7e1d7536e4389ede2e3acf06be8bd6a71b')
OR (filename='2026_08_17_chain_tx_outbox.sql' AND checksum='c0d392343fdb3608117afaf8a2d245d986b4a7f3b0ed2206873bc1230cdd8003'))
AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE status='applying')`},
	{name: "comparison column definitions", want: 4, sql: `SELECT COUNT(*) FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA=DATABASE() AND (
(TABLE_NAME='chain_event_record' AND COLUMN_NAME='event' AND COLUMN_TYPE='varchar(50)' AND IS_NULLABLE='NO' AND CHARACTER_SET_NAME='utf8mb4' AND COLLATION_NAME='utf8mb4_unicode_ci')
OR (TABLE_NAME='chain_event_record' AND COLUMN_NAME='tx_hash' AND COLUMN_TYPE='varchar(100)' AND IS_NULLABLE='NO' AND CHARACTER_SET_NAME='utf8mb4' AND COLLATION_NAME='utf8mb4_unicode_ci')
OR (TABLE_NAME='transaction_record' AND COLUMN_NAME='type' AND COLUMN_TYPE='varchar(20)' AND IS_NULLABLE='NO' AND CHARACTER_SET_NAME='utf8mb4' AND COLLATION_NAME='utf8mb4_unicode_ci')
OR (TABLE_NAME='transaction_record' AND COLUMN_NAME='tx_hash' AND COLUMN_TYPE='varchar(66)' AND IS_NULLABLE='NO' AND CHARACTER_SET_NAME='utf8mb4' AND COLLATION_NAME='utf8mb4_unicode_ci'))`},
	{name: "chain event identity", want: 1, sql: `SELECT COUNT(*) FROM (
SELECT INDEX_NAME,GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') columns_in_order,MIN(NON_UNIQUE) non_unique
FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_event_record' GROUP BY INDEX_NAME
) s WHERE columns_in_order='block_number,tx_hash,log_index' AND non_unique=0`},
}

var idoPurchaseEntitlementSchemaChecks = []requiredSchemaCheck{
	{name: "migration ledger", want: 2, sql: `SELECT COUNT(*) FROM schema_migrations
WHERE status='applied' AND filename IN ('2026_08_15_ido_v2_fund_ledger.sql','2026_08_20_ido_purchase_entitlement.sql')
AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE status='applying')`},
	{name: "column definitions", want: 24, sql: `SELECT COUNT(*) FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='ido_purchase_entitlement' AND (
(COLUMN_NAME='id' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO' AND EXTRA LIKE '%auto_increment%')
OR (COLUMN_NAME IN ('fund_ledger_id','chain_id','project_id','block_number') AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO')
OR (COLUMN_NAME IN ('contract_address','wallet_address','payment_token_address','project_token_address') AND COLUMN_TYPE='varchar(42)' AND IS_NULLABLE='NO')
OR (COLUMN_NAME='contract_version' AND COLUMN_TYPE='varchar(32)' AND IS_NULLABLE='NO')
OR (COLUMN_NAME IN ('payment_token_symbol','project_token_symbol') AND COLUMN_TYPE='varchar(20)' AND IS_NULLABLE='NO')
OR (COLUMN_NAME IN ('payment_token_decimals','project_token_decimals') AND COLUMN_TYPE='tinyint unsigned' AND IS_NULLABLE='NO')
OR (COLUMN_NAME IN ('payment_amount_raw','project_token_amount_raw') AND COLUMN_TYPE='varchar(78)' AND IS_NULLABLE='NO')
OR (COLUMN_NAME IN ('payment_amount_display','project_token_amount_display') AND COLUMN_TYPE='varchar(160)' AND IS_NULLABLE='NO')
OR (COLUMN_NAME IN ('tx_hash','block_hash') AND COLUMN_TYPE='varchar(66)' AND IS_NULLABLE='NO')
OR (COLUMN_NAME='log_index' AND COLUMN_TYPE='int unsigned' AND IS_NULLABLE='NO')
OR (COLUMN_NAME='status' AND COLUMN_TYPE='varchar(20)' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='canonical')
OR (COLUMN_NAME='event_data' AND DATA_TYPE='json' AND IS_NULLABLE='NO')
OR (COLUMN_NAME='created_at' AND COLUMN_TYPE='datetime(6)' AND IS_NULLABLE='NO' AND LOWER(COLUMN_DEFAULT)='current_timestamp(6)'))`},
	{name: "parent ledger definitions", want: 19, sql: `SELECT COUNT(*) FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fund_ledger' AND (
(COLUMN_NAME='id' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO' AND EXTRA LIKE '%auto_increment%')
OR (COLUMN_NAME IN ('chain_id','project_id','block_number') AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO')
OR (COLUMN_NAME IN ('contract_address','wallet_address','counterparty_address','token_address') AND COLUMN_TYPE='varchar(42)' AND IS_NULLABLE='NO')
OR (COLUMN_NAME IN ('contract_version','entry_type') AND COLUMN_TYPE='varchar(32)' AND IS_NULLABLE='NO')
OR (COLUMN_NAME='asset_role' AND COLUMN_TYPE='varchar(16)' AND IS_NULLABLE='NO')
OR (COLUMN_NAME='token_symbol' AND COLUMN_TYPE='varchar(20)' AND IS_NULLABLE='NO')
OR (COLUMN_NAME='token_decimals' AND COLUMN_TYPE='tinyint unsigned' AND IS_NULLABLE='NO')
OR (COLUMN_NAME='amount_raw' AND COLUMN_TYPE='varchar(78)' AND IS_NULLABLE='NO')
OR (COLUMN_NAME='amount_display' AND COLUMN_TYPE='varchar(160)' AND IS_NULLABLE='NO')
OR (COLUMN_NAME IN ('tx_hash','block_hash') AND COLUMN_TYPE='varchar(66)' AND IS_NULLABLE='NO')
OR (COLUMN_NAME='log_index' AND COLUMN_TYPE='int unsigned' AND IS_NULLABLE='NO')
OR (COLUMN_NAME='status' AND COLUMN_TYPE='varchar(20)' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='canonical'))`},
	{name: "parent ledger event identity", want: 1, sql: `SELECT COUNT(*) FROM (
SELECT INDEX_NAME,GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') columns_in_order,MIN(NON_UNIQUE) non_unique
FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fund_ledger' GROUP BY INDEX_NAME
) s WHERE non_unique=0 AND columns_in_order='chain_id,tx_hash,log_index'`},
	{name: "parent ledger enforced checks", want: 13, sql: `SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fund_ledger' AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES'
AND CONSTRAINT_NAME IN ('chk_fund_ledger_token_decimals','chk_fund_ledger_positive_identity','chk_fund_ledger_contract_address',
'chk_fund_ledger_token_address','chk_fund_ledger_tx_hash','chk_fund_ledger_block_hash','chk_fund_ledger_amount_raw',
'chk_fund_ledger_amount_display','chk_fund_ledger_contract_version','chk_fund_ledger_entry_type',
'chk_fund_ledger_asset_role','chk_fund_ledger_entry_asset_role','chk_fund_ledger_status')`},
	{name: "unique identities", want: 2, sql: `SELECT COUNT(*) FROM (
SELECT INDEX_NAME,GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') columns_in_order,MIN(NON_UNIQUE) non_unique
FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='ido_purchase_entitlement' GROUP BY INDEX_NAME
) s WHERE non_unique=0 AND columns_in_order IN ('fund_ledger_id','chain_id,tx_hash,log_index')`},
	{name: "ledger foreign key", want: 1, sql: `SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS
WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='ido_purchase_entitlement'
AND CONSTRAINT_NAME='fk_ido_purchase_entitlement_ledger' AND REFERENCED_TABLE_NAME='fund_ledger' AND DELETE_RULE='RESTRICT'`},
	{name: "enforced checks", want: 10, sql: `SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='ido_purchase_entitlement' AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES'
AND CONSTRAINT_NAME IN ('chk_ido_purchase_entitlement_positive_identity','chk_ido_purchase_entitlement_addresses',
'chk_ido_purchase_entitlement_tokens','chk_ido_purchase_entitlement_payment_raw','chk_ido_purchase_entitlement_project_raw',
'chk_ido_purchase_entitlement_payment_display','chk_ido_purchase_entitlement_project_display','chk_ido_purchase_entitlement_hashes',
'chk_ido_purchase_entitlement_version','chk_ido_purchase_entitlement_status')`},
	{name: "check clauses", want: 10, sql: `SELECT COUNT(*) FROM information_schema.CHECK_CONSTRAINTS cc
JOIN information_schema.TABLE_CONSTRAINTS tc ON tc.CONSTRAINT_SCHEMA=cc.CONSTRAINT_SCHEMA AND tc.CONSTRAINT_NAME=cc.CONSTRAINT_NAME
WHERE cc.CONSTRAINT_SCHEMA=DATABASE() AND tc.TABLE_NAME='ido_purchase_entitlement' AND (
(cc.CONSTRAINT_NAME='chk_ido_purchase_entitlement_positive_identity' AND cc.CHECK_CLAUSE LIKE '%fund_ledger_id%' AND cc.CHECK_CLAUSE LIKE '%block_number%')
OR (cc.CONSTRAINT_NAME='chk_ido_purchase_entitlement_addresses' AND cc.CHECK_CLAUSE LIKE '%payment_token_address%' AND cc.CHECK_CLAUSE LIKE '%project_token_address%')
OR (cc.CONSTRAINT_NAME='chk_ido_purchase_entitlement_tokens' AND cc.CHECK_CLAUSE LIKE '%USDT%' AND cc.CHECK_CLAUSE LIKE '%USDC%' AND cc.CHECK_CLAUSE LIKE '%18%')
OR (cc.CONSTRAINT_NAME='chk_ido_purchase_entitlement_payment_raw' AND cc.CHECK_CLAUSE LIKE '%115792089237316195423570985008687907853269984665640564039457584007913129639935%')
OR (cc.CONSTRAINT_NAME='chk_ido_purchase_entitlement_project_raw' AND cc.CHECK_CLAUSE LIKE '%115792089237316195423570985008687907853269984665640564039457584007913129639935%')
OR (cc.CONSTRAINT_NAME='chk_ido_purchase_entitlement_payment_display' AND cc.CHECK_CLAUSE LIKE '%payment_amount_raw%' AND cc.CHECK_CLAUSE LIKE '%payment_token_decimals%')
OR (cc.CONSTRAINT_NAME='chk_ido_purchase_entitlement_project_display' AND cc.CHECK_CLAUSE LIKE '%project_token_amount_raw%' AND cc.CHECK_CLAUSE LIKE '%project_token_decimals%')
OR (cc.CONSTRAINT_NAME='chk_ido_purchase_entitlement_hashes' AND cc.CHECK_CLAUSE LIKE '%tx_hash%' AND cc.CHECK_CLAUSE LIKE '%block_hash%')
OR (cc.CONSTRAINT_NAME='chk_ido_purchase_entitlement_version' AND cc.CHECK_CLAUSE LIKE '%v2_stablecoin%')
OR (cc.CONSTRAINT_NAME='chk_ido_purchase_entitlement_status' AND cc.CHECK_CLAUSE LIKE '%canonical%'))`},
	{name: "trigger definitions", want: 3, sql: `SELECT COUNT(*) FROM information_schema.TRIGGERS
WHERE TRIGGER_SCHEMA=DATABASE() AND EVENT_OBJECT_TABLE='ido_purchase_entitlement' AND ACTION_TIMING='BEFORE' AND (
(TRIGGER_NAME='trg_ido_purchase_entitlement_validate_insert' AND EVENT_MANIPULATION='INSERT' AND ACTION_STATEMENT LIKE '%purchase_payment%' AND ACTION_STATEMENT LIKE '%fund_ledger_id%' AND ACTION_STATEMENT LIKE '%SIGNAL%')
OR (TRIGGER_NAME='trg_ido_purchase_entitlement_no_update' AND EVENT_MANIPULATION='UPDATE' AND ACTION_STATEMENT LIKE '%append-only%' AND ACTION_STATEMENT LIKE '%SIGNAL%')
OR (TRIGGER_NAME='trg_ido_purchase_entitlement_no_delete' AND EVENT_MANIPULATION='DELETE' AND ACTION_STATEMENT LIKE '%append-only%' AND ACTION_STATEMENT LIKE '%SIGNAL%'))`},
}

var idoReconciliationEvidenceSchemaChecks = []requiredSchemaCheck{
	{name: "migration ledger", want: 3, sql: `SELECT COUNT(*) FROM schema_migrations
WHERE status='applied' AND (
(filename='2026_08_15_ido_v2_fund_ledger.sql' AND checksum='024cba01a1a679783332ad83bfd8ea37588bb2dbd08f4b3112d1cbfade42a8e9')
OR (filename='2026_08_20_ido_purchase_entitlement.sql' AND checksum='d63a352d886e37287370b49d15645d14cb7bc8ebef7780860e7de0c02ceee135')
OR (filename='2026_08_21_ido_reconciliation_evidence.sql' AND checksum='741cee24965e05d546b5d5e19f65d08e56c2e300bf35222e9228749e02844b13'))
AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE status='applying')`},
	{name: "evidence tables", want: 6, sql: `SELECT COUNT(*) FROM information_schema.TABLES
WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN ('ido_reconciliation_run','ido_reconciliation_job','ido_reconciliation_event','ido_reconciliation_wallet','ido_reconciliation_check','ido_reconciliation_manifest') AND TABLE_TYPE='BASE TABLE'`},
	{name: "exact column counts", want: 6, sql: `SELECT COUNT(*) FROM (
SELECT TABLE_NAME,COUNT(*) column_count FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()
AND TABLE_NAME IN ('ido_reconciliation_run','ido_reconciliation_job','ido_reconciliation_event','ido_reconciliation_wallet','ido_reconciliation_check','ido_reconciliation_manifest') GROUP BY TABLE_NAME
) c WHERE (TABLE_NAME='ido_reconciliation_run' AND column_count=24)
OR (TABLE_NAME='ido_reconciliation_job' AND column_count=19)
OR (TABLE_NAME='ido_reconciliation_event' AND column_count=12)
OR (TABLE_NAME='ido_reconciliation_wallet' AND column_count=16)
OR (TABLE_NAME='ido_reconciliation_check' AND column_count=13)
OR (TABLE_NAME='ido_reconciliation_manifest' AND column_count=13)`},
	{name: "binary evidence definitions", want: 29, sql: `SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND (
(TABLE_NAME='ido_reconciliation_run' AND COLUMN_NAME IN ('run_uuid','idempotency_key','source_digest','expected_wallet_identity_digest','expected_check_identity_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='ido_reconciliation_run' AND COLUMN_NAME='source_payload' AND COLUMN_TYPE='mediumblob' AND IS_NULLABLE='NO')
OR (TABLE_NAME='ido_reconciliation_job' AND COLUMN_NAME IN ('state','lease_token') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='ido_reconciliation_wallet' AND COLUMN_NAME IN ('wallet_address','expected_payment_raw','actual_payment_raw','expected_tokens_raw','actual_tokens_raw','expected_claimed_raw','actual_claimed_raw','previous_row_digest','identity_digest','row_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='ido_reconciliation_check' AND COLUMN_NAME IN ('scope','code','previous_row_digest','identity_digest','row_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='ido_reconciliation_manifest' AND COLUMN_NAME='payload' AND COLUMN_TYPE='mediumblob' AND IS_NULLABLE='NO')
OR (TABLE_NAME='ido_reconciliation_manifest' AND COLUMN_NAME IN ('payload_digest','wallet_evidence_digest','check_evidence_digest','previous_manifest_digest','manifest_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO'))`},
	{name: "exact named indexes", want: 23, sql: `SELECT COUNT(*) FROM (
SELECT TABLE_NAME,INDEX_NAME,GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') columns_in_order,MIN(NON_UNIQUE) non_unique
FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN ('ido_reconciliation_run','ido_reconciliation_job','ido_reconciliation_event','ido_reconciliation_wallet','ido_reconciliation_check','ido_reconciliation_manifest') GROUP BY TABLE_NAME,INDEX_NAME
) s WHERE CONCAT(TABLE_NAME,'|',INDEX_NAME,'|',columns_in_order,'|',non_unique) IN (
'ido_reconciliation_run|PRIMARY|id|0','ido_reconciliation_run|uk_ido_reconciliation_run_uuid|run_uuid|0','ido_reconciliation_run|uk_ido_reconciliation_request_idempotency|requested_by_admin_id,idempotency_key|0','ido_reconciliation_run|idx_ido_reconciliation_project_time|project_id,requested_at,id|1',
'ido_reconciliation_job|PRIMARY|run_id|0','ido_reconciliation_job|uk_ido_reconciliation_active_project|project_id,active_slot|0','ido_reconciliation_job|idx_ido_reconciliation_job_claim|state,next_attempt_at,lease_expires_at,run_id|1','ido_reconciliation_job|idx_ido_reconciliation_job_lease|lease_expires_at,state|1',
'ido_reconciliation_event|PRIMARY|id|0','ido_reconciliation_event|uk_ido_reconciliation_event_sequence|run_id,sequence_no|0','ido_reconciliation_event|uk_ido_reconciliation_event_hash|event_hash|0','ido_reconciliation_event|idx_ido_reconciliation_event_type|run_id,event_type,created_at|1',
'ido_reconciliation_wallet|PRIMARY|id|0','ido_reconciliation_wallet|uk_ido_reconciliation_wallet_ordinal|run_id,ordinal|0','ido_reconciliation_wallet|uk_ido_reconciliation_wallet_address|run_id,wallet_address|0','ido_reconciliation_wallet|idx_ido_reconciliation_wallet_lookup|wallet_address,run_id|1',
'ido_reconciliation_check|PRIMARY|id|0','ido_reconciliation_check|uk_ido_reconciliation_check_ordinal|run_id,ordinal|0','ido_reconciliation_check|uk_ido_reconciliation_check_code|run_id,scope,code|0',
'ido_reconciliation_manifest|PRIMARY|run_id|0','ido_reconciliation_manifest|uk_ido_reconciliation_manifest_digest|manifest_digest|0','ido_reconciliation_manifest|uk_ido_reconciliation_manifest_parent|project_id,previous_manifest_digest|0','ido_reconciliation_manifest|idx_ido_reconciliation_manifest_project|project_id,run_id|1')`},
	{name: "exact restricted foreign keys", want: 5, sql: `SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS rc
JOIN information_schema.KEY_COLUMN_USAGE k ON k.CONSTRAINT_SCHEMA=rc.CONSTRAINT_SCHEMA AND k.CONSTRAINT_NAME=rc.CONSTRAINT_NAME AND k.TABLE_NAME=rc.TABLE_NAME
WHERE rc.CONSTRAINT_SCHEMA=DATABASE() AND CONCAT(rc.TABLE_NAME,'|',rc.CONSTRAINT_NAME,'|',k.COLUMN_NAME,'|',rc.REFERENCED_TABLE_NAME,'|',k.REFERENCED_COLUMN_NAME,'|',rc.DELETE_RULE) IN (
'ido_reconciliation_job|fk_ido_reconciliation_job_run|run_id|ido_reconciliation_run|id|RESTRICT',
'ido_reconciliation_event|fk_ido_reconciliation_event_run|run_id|ido_reconciliation_run|id|RESTRICT',
'ido_reconciliation_wallet|fk_ido_reconciliation_wallet_run|run_id|ido_reconciliation_run|id|RESTRICT',
'ido_reconciliation_check|fk_ido_reconciliation_check_run|run_id|ido_reconciliation_run|id|RESTRICT',
'ido_reconciliation_manifest|fk_ido_reconciliation_manifest_run|run_id|ido_reconciliation_run|id|RESTRICT')`},
	{name: "enforced check semantics", want: 16, sql: `SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS tc
JOIN information_schema.CHECK_CONSTRAINTS cc ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK' AND tc.ENFORCED='YES' AND (
(tc.CONSTRAINT_NAME='chk_ido_reconciliation_run_identity' AND REPLACE(cc.CHECK_CLAUSE,' ','') LIKE '%project_id%>0%' AND REPLACE(cc.CHECK_CLAUSE,' ','') LIKE '%evidence_block_number%>0%')
OR (tc.CONSTRAINT_NAME='chk_ido_reconciliation_run_addresses' AND cc.CHECK_CLAUSE LIKE '%regexp_like%contract_address%' AND cc.CHECK_CLAUSE LIKE '%regexp_like%implementation_address%')
OR (tc.CONSTRAINT_NAME='chk_ido_reconciliation_run_hashes' AND cc.CHECK_CLAUSE LIKE '%regexp_like%source_digest%' AND cc.CHECK_CLAUSE LIKE '%regexp_like%expected_wallet_identity_digest%' AND cc.CHECK_CLAUSE LIKE '%regexp_like%expected_check_identity_digest%')
OR (tc.CONSTRAINT_NAME='chk_ido_reconciliation_run_request' AND cc.CHECK_CLAUSE LIKE '%idempotency_key%' AND cc.CHECK_CLAUSE LIKE '%reason%')
OR (tc.CONSTRAINT_NAME='chk_ido_reconciliation_run_versions' AND cc.CHECK_CLAUSE LIKE '%joincare-ido-reconciliation-v1%' AND cc.CHECK_CLAUSE LIKE '%canonical-json-v1%')
OR (tc.CONSTRAINT_NAME='chk_ido_reconciliation_job_state' AND cc.CHECK_CLAUSE LIKE '%finalizing%' AND cc.CHECK_CLAUSE LIKE '%invalidated%')
OR (tc.CONSTRAINT_NAME='chk_ido_reconciliation_job_outcome' AND cc.CHECK_CLAUSE LIKE '%BALANCED%' AND REPLACE(LOWER(cc.CHECK_CLAUSE),' ','') LIKE '%outcome%isnull%')
OR (tc.CONSTRAINT_NAME='chk_ido_reconciliation_job_active' AND cc.CHECK_CLAUSE LIKE '%active_slot%' AND cc.CHECK_CLAUSE LIKE '%completed_at%')
OR (tc.CONSTRAINT_NAME='chk_ido_reconciliation_job_lease_fields' AND cc.CHECK_CLAUSE LIKE '%regexp_like%lease_token%' AND cc.CHECK_CLAUSE LIKE '%lease_expires_at%')
OR (tc.CONSTRAINT_NAME='chk_ido_reconciliation_job_cancel' AND cc.CHECK_CLAUSE LIKE '%cancel_reason%' AND cc.CHECK_CLAUSE LIKE '%cancel_requested_at%')
OR (tc.CONSTRAINT_NAME='chk_ido_reconciliation_event_fields' AND cc.CHECK_CLAUSE LIKE '%FINALIZED%' AND cc.CHECK_CLAUSE LIKE '%previous_event_hash%regexp%')
OR (tc.CONSTRAINT_NAME='chk_ido_reconciliation_wallet_identity' AND cc.CHECK_CLAUSE LIKE '%regexp_like%wallet_address%')
OR (tc.CONSTRAINT_NAME='chk_ido_reconciliation_wallet_raw' AND REPLACE(cc.CHECK_CLAUSE,' ','') LIKE '%expected_refunded%in(0,1)%' AND REPLACE(cc.CHECK_CLAUSE,' ','') LIKE '%actual_refunded%in(0,1)%' AND cc.CHECK_CLAUSE LIKE '%115792089237316195423570985008687907853269984665640564039457584007913129639935%' AND cc.CHECK_CLAUSE LIKE '%identity_digest%regexp%')
OR (tc.CONSTRAINT_NAME='chk_ido_reconciliation_check_fields' AND REPLACE(cc.CHECK_CLAUSE,' ','') LIKE '%ok%in(0,1)%' AND cc.CHECK_CLAUSE LIKE '%identity_digest%regexp%')
OR (tc.CONSTRAINT_NAME='chk_ido_reconciliation_manifest_outcome' AND REPLACE(cc.CHECK_CLAUSE,' ','') LIKE '%failed_check_count%=0%' AND REPLACE(cc.CHECK_CLAUSE,' ','') LIKE '%failed_check_count%>0%')
OR (tc.CONSTRAINT_NAME='chk_ido_reconciliation_manifest_hashes' AND cc.CHECK_CLAUSE LIKE '%wallet_evidence_digest%regexp%' AND cc.CHECK_CLAUSE LIKE '%check_evidence_digest%regexp%' AND cc.CHECK_CLAUSE LIKE '%previous_manifest_digest%regexp%'))`},
	{name: "trigger definitions", want: 18, sql: `SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND (
(TRIGGER_NAME='trg_ido_reconciliation_run_validate_insert' AND EVENT_MANIPULATION='INSERT' AND ACTION_STATEMENT LIKE '%JSON_VALID%' AND ACTION_STATEMENT LIKE '%SHA2%' AND ACTION_STATEMENT LIKE '%source_digest%')
OR (TRIGGER_NAME IN ('trg_ido_reconciliation_run_no_update','trg_ido_reconciliation_event_no_update','trg_ido_reconciliation_wallet_no_update','trg_ido_reconciliation_check_no_update','trg_ido_reconciliation_manifest_no_update') AND EVENT_MANIPULATION='UPDATE' AND ACTION_STATEMENT LIKE '%append-only%')
OR (TRIGGER_NAME IN ('trg_ido_reconciliation_run_no_delete','trg_ido_reconciliation_event_no_delete','trg_ido_reconciliation_wallet_no_delete','trg_ido_reconciliation_check_no_delete','trg_ido_reconciliation_manifest_no_delete','trg_ido_reconciliation_job_no_delete') AND EVENT_MANIPULATION='DELETE' AND ACTION_STATEMENT LIKE '%SIGNAL%')
OR (TRIGGER_NAME='trg_ido_reconciliation_event_validate_insert' AND EVENT_MANIPULATION='INSERT' AND ACTION_STATEMENT LIKE '%terminal IDO reconciliation event%' AND ACTION_STATEMENT LIKE '%does not match job state%' AND ACTION_STATEMENT LIKE '%event chain%' AND ACTION_STATEMENT LIKE '%SHA2%')
OR (TRIGGER_NAME='trg_ido_reconciliation_wallet_validate_insert' AND EVENT_MANIPULATION='INSERT' AND REPLACE(ACTION_STATEMENT,' ','') LIKE '%lease_expires_at>CURRENT_TIMESTAMP(6)%' AND ACTION_STATEMENT LIKE '%chain is not contiguous%' AND ACTION_STATEMENT LIKE '%wallet-identity.v1%' AND ACTION_STATEMENT LIKE '%NEW.expected_payment_raw%' AND ACTION_STATEMENT LIKE '%NEW.actual_payment_raw%' AND ACTION_STATEMENT LIKE '%NEW.expected_refunded%' AND ACTION_STATEMENT LIKE '%NEW.actual_refunded%' AND ACTION_STATEMENT LIKE '%NEW.previous_row_digest%' AND ACTION_STATEMENT LIKE '%SHA2%')
OR (TRIGGER_NAME='trg_ido_reconciliation_check_validate_insert' AND EVENT_MANIPULATION='INSERT' AND REPLACE(ACTION_STATEMENT,' ','') LIKE '%lease_expires_at>CURRENT_TIMESTAMP(6)%' AND ACTION_STATEMENT LIKE '%check evidence requires%' AND ACTION_STATEMENT LIKE '%chain is not contiguous%' AND ACTION_STATEMENT LIKE '%check-identity.v1%' AND ACTION_STATEMENT LIKE '%NEW.ok%' AND ACTION_STATEMENT LIKE '%NEW.expected_exact%' AND ACTION_STATEMENT LIKE '%NEW.actual_exact%' AND ACTION_STATEMENT LIKE '%NEW.message%' AND ACTION_STATEMENT LIKE '%NEW.previous_row_digest%' AND ACTION_STATEMENT LIKE '%SHA2%')
OR (TRIGGER_NAME='trg_ido_reconciliation_manifest_validate_insert' AND EVENT_MANIPULATION='INSERT' AND ACTION_STATEMENT LIKE '%JSON_VALID%' AND REPLACE(ACTION_STATEMENT,' ','') LIKE '%lease_expires_at>CURRENT_TIMESTAMP(6)%' AND REPLACE(ACTION_STATEMENT,' ','') LIKE '%expected_payment_raw<>actual_payment_raw%' AND REPLACE(ACTION_STATEMENT,' ','') LIKE '%expected_tokens_raw<>actual_tokens_raw%' AND REPLACE(ACTION_STATEMENT,' ','') LIKE '%expected_claimed_raw<>actual_claimed_raw%' AND REPLACE(ACTION_STATEMENT,' ','') LIKE '%expected_refunded<>actual_refunded%' AND REPLACE(ACTION_STATEMENT,' ','') LIKE '%actual_wallets*4%' AND ACTION_STATEMENT LIKE '%wallet identity set%' AND ACTION_STATEMENT LIKE '%check identity set%' AND ACTION_STATEMENT LIKE '%NEW.payload_digest%' AND ACTION_STATEMENT LIKE '%NEW.wallet_evidence_digest%' AND ACTION_STATEMENT LIKE '%NEW.check_evidence_digest%' AND ACTION_STATEMENT LIKE '%NEW.previous_manifest_digest%' AND ACTION_STATEMENT LIKE '%SHA2%')
OR (TRIGGER_NAME='trg_ido_reconciliation_job_validate_insert' AND EVENT_MANIPULATION='INSERT' AND ACTION_STATEMENT LIKE '%canonical queued state%' AND ACTION_STATEMENT LIKE '%next_wallet_ordinal%')
OR (TRIGGER_NAME='trg_ido_reconciliation_job_validate_update' AND EVENT_MANIPULATION='UPDATE' AND ACTION_STATEMENT LIKE '%terminal%' AND ACTION_STATEMENT LIKE '%matching manifest%'))`},
}

var chainEventReorgEvidenceSchemaChecks = []requiredSchemaCheck{
	{name: "migration ledger", want: 1, sql: `SELECT COUNT(*) FROM schema_migrations
WHERE status='applied' AND filename='2026_08_22_chain_event_reorg_incident.sql'
AND checksum='ba1590f9d1005b1548e785c176ecb8da146fc7244c5d8ea7e552a983d16db512'
AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE status='applying')`},
	{name: "evidence tables", want: 3, sql: `SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
AND TABLE_NAME IN ('chain_event_block_checkpoint','chain_event_reorg_incident','chain_event_reorg_event') AND TABLE_TYPE='BASE TABLE'`},
	{name: "exact column counts", want: 3, sql: `SELECT COUNT(*) FROM (
SELECT TABLE_NAME,COUNT(*) column_count FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()
AND TABLE_NAME IN ('chain_event_block_checkpoint','chain_event_reorg_incident','chain_event_reorg_event') GROUP BY TABLE_NAME
) c WHERE (TABLE_NAME='chain_event_block_checkpoint' AND column_count=7)
OR (TABLE_NAME='chain_event_reorg_incident' AND column_count=17)
OR (TABLE_NAME='chain_event_reorg_event' AND column_count=8)`},
	{name: "exact named indexes", want: 13, sql: `SELECT COUNT(*) FROM (
SELECT TABLE_NAME,INDEX_NAME,GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') columns_in_order,MIN(NON_UNIQUE) non_unique
FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE()
AND TABLE_NAME IN ('chain_event_block_checkpoint','chain_event_reorg_incident','chain_event_reorg_event') GROUP BY TABLE_NAME,INDEX_NAME
) s WHERE CONCAT(TABLE_NAME,'|',INDEX_NAME,'|',columns_in_order,'|',non_unique) IN (
'chain_event_block_checkpoint|PRIMARY|id|0','chain_event_block_checkpoint|uk_chain_event_checkpoint_block|chain_id,block_number|0','chain_event_block_checkpoint|uk_chain_event_checkpoint_hash|checkpoint_hash|0','chain_event_block_checkpoint|uk_chain_event_checkpoint_parent|previous_checkpoint_hash|0',
'chain_event_reorg_incident|PRIMARY|id|0','chain_event_reorg_incident|uk_chain_event_reorg_incident_key|incident_key|0','chain_event_reorg_incident|uk_chain_event_reorg_active|active_slot|0','chain_event_reorg_incident|idx_chain_event_reorg_status_time|status,detected_at,id|1','chain_event_reorg_incident|idx_chain_event_reorg_checkpoint|cursor_checkpoint_id|1',
'chain_event_reorg_event|PRIMARY|id|0','chain_event_reorg_event|uk_chain_event_reorg_event_sequence|incident_id,sequence_no|0','chain_event_reorg_event|uk_chain_event_reorg_event_hash|event_hash|0','chain_event_reorg_event|idx_chain_event_reorg_event_incident|incident_id|1')`},
	{name: "restricted foreign keys", want: 2, sql: `SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
AND CONSTRAINT_NAME IN ('fk_chain_event_reorg_checkpoint','fk_chain_event_reorg_event_incident') AND DELETE_RULE='RESTRICT'`},
	{name: "enforced checks", want: 6, sql: `SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES' AND CONSTRAINT_NAME IN (
'chk_chain_event_checkpoint_identity','chk_chain_event_checkpoint_digests','chk_chain_event_reorg_identity',
'chk_chain_event_reorg_hashes','chk_chain_event_reorg_state','chk_chain_event_reorg_event_fields')`},
	{name: "trigger definitions", want: 9, sql: `SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND (
(TRIGGER_NAME='trg_chain_event_checkpoint_validate_insert' AND EVENT_MANIPULATION='INSERT' AND ACTION_STATEMENT LIKE '%checkpoint chain%' AND ACTION_STATEMENT LIKE '%SHA2%')
OR (TRIGGER_NAME='trg_chain_event_reorg_incident_validate_insert' AND EVENT_MANIPULATION='INSERT' AND ACTION_STATEMENT LIKE '%cursor checkpoint identity%' AND ACTION_STATEMENT LIKE '%SHA2%')
OR (TRIGGER_NAME='trg_chain_event_reorg_event_validate_insert' AND EVENT_MANIPULATION='INSERT' AND ACTION_STATEMENT LIKE '%event chain%' AND ACTION_STATEMENT LIKE '%SHA2%')
OR (TRIGGER_NAME IN ('trg_chain_event_checkpoint_no_update','trg_chain_event_reorg_incident_no_update','trg_chain_event_reorg_event_no_update') AND EVENT_MANIPULATION='UPDATE' AND ACTION_STATEMENT LIKE '%append-only%')
OR (TRIGGER_NAME IN ('trg_chain_event_checkpoint_no_delete','trg_chain_event_reorg_incident_no_delete','trg_chain_event_reorg_event_no_delete') AND EVENT_MANIPULATION='DELETE' AND ACTION_STATEMENT LIKE '%append-only%'))`},
}

var chainEventReorgRecoverySchemaChecks = []requiredSchemaCheck{
	{name: "migration ledger", want: 1, sql: `SELECT COUNT(*) FROM schema_migrations
WHERE status='applied' AND filename='2026_08_23_chain_event_reorg_recovery_scan.sql'
AND checksum='2093bc1489c5d2af38219bd6732b954981cabadfb4375b419e9d47e1b8d1994f'
AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE status='applying')`},
	{name: "recovery tables", want: 7, sql: `SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
AND TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_recovery_job','chain_event_reorg_recovery_event',
'chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch','chain_event_reorg_recovery_manifest',
'chain_event_reorg_recovery_definition') AND TABLE_TYPE='BASE TABLE'`},
	{name: "exact column counts", want: 7, sql: `SELECT COUNT(*) FROM (
SELECT TABLE_NAME,COUNT(*) column_count FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()
AND TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_recovery_job','chain_event_reorg_recovery_event',
'chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch','chain_event_reorg_recovery_manifest',
'chain_event_reorg_recovery_definition') GROUP BY TABLE_NAME
) c WHERE (TABLE_NAME='chain_event_reorg_recovery_run' AND column_count=20)
OR (TABLE_NAME='chain_event_reorg_recovery_job' AND column_count=18)
OR (TABLE_NAME='chain_event_reorg_recovery_event' AND column_count=10)
OR (TABLE_NAME='chain_event_reorg_recovery_log' AND column_count=15)
OR (TABLE_NAME='chain_event_reorg_recovery_batch' AND column_count=13)
OR (TABLE_NAME='chain_event_reorg_recovery_manifest' AND column_count=8)
OR (TABLE_NAME='chain_event_reorg_recovery_definition' AND column_count=6)`},
	{name: "binary evidence definitions", want: 39, sql: `SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND (
(TABLE_NAME='chain_event_reorg_recovery_run' AND COLUMN_NAME IN ('run_uuid','contract_address','legacy_contract_address','contract_version','algorithm_version','boundary_kind','boundary_block_hash','target_safe_block_hash') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_recovery_job' AND COLUMN_NAME IN ('state','batch_evidence_digest','log_evidence_digest','lease_token','last_error_code') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_recovery_job' AND COLUMN_NAME='fence_epoch' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_recovery_event' AND COLUMN_NAME IN ('event_type','stage','error_code','previous_event_hash','event_hash') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_recovery_log' AND COLUMN_NAME IN ('block_hash','tx_hash','contract_address','contract_version','raw_log_digest','previous_row_digest','row_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_recovery_log' AND COLUMN_NAME='raw_log' AND COLUMN_TYPE='mediumblob' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_recovery_batch' AND COLUMN_NAME IN ('from_block_hash','to_block_hash','batch_log_digest','previous_batch_digest','batch_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_recovery_manifest' AND COLUMN_NAME IN ('batch_evidence_digest','log_evidence_digest','manifest_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_recovery_definition' AND COLUMN_NAME IN ('object_type','object_name','definition_digest','row_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO'))`},
	{name: "exact named indexes", want: 27, sql: `SELECT COUNT(*) FROM (
SELECT TABLE_NAME,INDEX_NAME,GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') columns_in_order,MIN(NON_UNIQUE) non_unique
FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
'chain_event_reorg_recovery_run','chain_event_reorg_recovery_job','chain_event_reorg_recovery_event',
'chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch','chain_event_reorg_recovery_manifest',
'chain_event_reorg_recovery_definition') GROUP BY TABLE_NAME,INDEX_NAME
) s WHERE CONCAT(TABLE_NAME,'|',INDEX_NAME,'|',columns_in_order,'|',non_unique) IN (
'chain_event_reorg_recovery_run|PRIMARY|id|0','chain_event_reorg_recovery_run|uk_chain_event_reorg_recovery_uuid|run_uuid|0','chain_event_reorg_recovery_run|uk_chain_event_reorg_recovery_attempt|incident_id,attempt_no|0','chain_event_reorg_recovery_run|idx_chain_event_reorg_recovery_incident|incident_id,id|1','chain_event_reorg_recovery_run|idx_chain_event_reorg_recovery_checkpoint|boundary_checkpoint_id|1',
'chain_event_reorg_recovery_job|PRIMARY|run_id|0','chain_event_reorg_recovery_job|uk_chain_event_reorg_recovery_active|incident_id,active_slot|0','chain_event_reorg_recovery_job|idx_chain_event_reorg_recovery_job_state|state,updated_at,run_id|1',
'chain_event_reorg_recovery_event|PRIMARY|id|0','chain_event_reorg_recovery_event|uk_chain_event_reorg_recovery_event_sequence|run_id,sequence_no|0','chain_event_reorg_recovery_event|uk_chain_event_reorg_recovery_event_hash|event_hash|0','chain_event_reorg_recovery_event|idx_chain_event_reorg_recovery_event_type|run_id,event_type,created_at|1',
'chain_event_reorg_recovery_log|PRIMARY|id|0','chain_event_reorg_recovery_log|uk_chain_event_reorg_recovery_log_ordinal|run_id,ordinal|0','chain_event_reorg_recovery_log|uk_chain_event_reorg_recovery_log_identity|run_id,block_number,tx_hash,log_index|0','chain_event_reorg_recovery_log|uk_chain_event_reorg_recovery_log_digest|run_id,row_digest|0','chain_event_reorg_recovery_log|idx_chain_event_reorg_recovery_log_block|run_id,block_number,tx_index,log_index|1',
'chain_event_reorg_recovery_batch|PRIMARY|id|0','chain_event_reorg_recovery_batch|uk_chain_event_reorg_recovery_batch_ordinal|run_id,ordinal|0','chain_event_reorg_recovery_batch|uk_chain_event_reorg_recovery_batch_range|run_id,from_block,to_block|0','chain_event_reorg_recovery_batch|uk_chain_event_reorg_recovery_batch_digest|run_id,batch_digest|0',
'chain_event_reorg_recovery_manifest|PRIMARY|run_id|0','chain_event_reorg_recovery_manifest|uk_chain_event_reorg_recovery_manifest_digest|manifest_digest|0','chain_event_reorg_recovery_manifest|idx_chain_event_reorg_recovery_manifest_incident|incident_id,run_id|1',
'chain_event_reorg_recovery_definition|PRIMARY|id|0','chain_event_reorg_recovery_definition|uk_chain_event_reorg_recovery_definition_object|object_type,object_name|0','chain_event_reorg_recovery_definition|uk_chain_event_reorg_recovery_definition_digest|row_digest|0')`},
	{name: "restricted foreign keys", want: 7, sql: `SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
AND CONSTRAINT_NAME IN ('fk_chain_event_reorg_recovery_incident','fk_chain_event_reorg_recovery_checkpoint',
'fk_chain_event_reorg_recovery_job_run','fk_chain_event_reorg_recovery_event_run','fk_chain_event_reorg_recovery_log_run',
'fk_chain_event_reorg_recovery_batch_run','fk_chain_event_reorg_recovery_manifest_run') AND DELETE_RULE='RESTRICT'`},
	{name: "enforced check semantics", want: 13, sql: `SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS tc
JOIN information_schema.CHECK_CONSTRAINTS cc ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK' AND tc.ENFORCED='YES' AND (
(tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_run_identity' AND cc.CHECK_CLAUSE LIKE '%run_uuid%regexp%' AND cc.CHECK_CLAUSE LIKE '%legacy_contract_address%' AND cc.CHECK_CLAUSE LIKE '%joincare-reorg-recovery-scan-v1%')
OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_boundary' AND cc.CHECK_CLAUSE LIKE '%last_canonical_checkpoint%' AND cc.CHECK_CLAUSE LIKE '%boundary_checkpoint_id%is not null%')
OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_target' AND cc.CHECK_CLAUSE LIKE '%expected_scanned_blocks%' AND cc.CHECK_CLAUSE LIKE '%expected_batch_count%' AND cc.CHECK_CLAUSE LIKE '%100%')
OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_job_state' AND cc.CHECK_CLAUSE LIKE '%scanning%' AND cc.CHECK_CLAUSE LIKE '%completed%' AND cc.CHECK_CLAUSE LIKE '%invalidated%' AND cc.CHECK_CLAUSE NOT LIKE '%failed%')
OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_job_digests' AND cc.CHECK_CLAUSE LIKE '%lease_token%regexp%' AND cc.CHECK_CLAUSE LIKE '%lease_expires_at%' AND cc.CHECK_CLAUSE LIKE '%fence_epoch%')
OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_event_fields' AND cc.CHECK_CLAUSE LIKE '%BATCH_COMMITTED%' AND cc.CHECK_CLAUSE LIKE '%INVALIDATED%' AND cc.CHECK_CLAUSE LIKE '%previous_event_hash%regexp%')
OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_log_identity' AND cc.CHECK_CLAUSE LIKE '%v1_legacy%' AND cc.CHECK_CLAUSE LIKE '%v2_stablecoin%' AND cc.CHECK_CLAUSE LIKE '%raw_log%')
OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_log_digests' AND cc.CHECK_CLAUSE LIKE '%raw_log_digest%regexp%' AND cc.CHECK_CLAUSE LIKE '%previous_row_digest%regexp%')
OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_batch_identity' AND cc.CHECK_CLAUSE LIKE '%to_block%from_block%' AND cc.CHECK_CLAUSE LIKE '%100%' AND cc.CHECK_CLAUSE LIKE '%first_log_ordinal%')
OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_batch_digests' AND cc.CHECK_CLAUSE LIKE '%batch_log_digest%regexp%' AND cc.CHECK_CLAUSE LIKE '%previous_batch_digest%regexp%')
OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_manifest_fields' AND cc.CHECK_CLAUSE LIKE '%regexp_like%batch_evidence_digest%' AND cc.CHECK_CLAUSE LIKE '%regexp_like%log_evidence_digest%' AND cc.CHECK_CLAUSE LIKE '%regexp_like%manifest_digest%')
OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_definition_identity' AND cc.CHECK_CLAUSE LIKE '%CHECK%' AND cc.CHECK_CLAUSE LIKE '%TRIGGER%' AND cc.CHECK_CLAUSE LIKE '%object_name%')
OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_definition_digests' AND cc.CHECK_CLAUSE LIKE '%regexp_like%definition_digest%' AND cc.CHECK_CLAUSE LIKE '%regexp_like%row_digest%'))`},
	{name: "trigger definitions", want: 21, sql: `SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND (
(TRIGGER_NAME='trg_chain_event_reorg_recovery_run_validate_insert' AND EVENT_MANIPULATION='INSERT' AND ACTION_STATEMENT LIKE '%matching active incident%' AND ACTION_STATEMENT LIKE '%attempt is not contiguous%' AND ACTION_STATEMENT LIKE '%boundary checkpoint identity%')
OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_job_validate_insert' AND EVENT_MANIPULATION='INSERT' AND ACTION_STATEMENT LIKE '%canonical initial state%')
OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_job_validate_update' AND EVENT_MANIPULATION='UPDATE' AND ACTION_STATEMENT LIKE '%scanning transition bypassed lease or batch checkpoint%' AND ACTION_STATEMENT LIKE '%terminal transition lacks leased matching event%' AND ACTION_STATEMENT LIKE '%lease_expires_at%' AND ACTION_STATEMENT LIKE '%batch_digest%' AND ACTION_STATEMENT LIKE '%requires manifest%')
OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_event_validate_insert' AND EVENT_MANIPULATION='INSERT'
AND ACTION_STATEMENT LIKE '%event chain%' AND ACTION_STATEMENT LIKE '%matching active leased job%'
AND ACTION_STATEMENT LIKE '%NEW.event_type=''CREATED''%' AND ACTION_STATEMENT LIKE '%next_batch_ordinal=1%'
AND ACTION_STATEMENT LIKE '%lease_token=''''%' AND ACTION_STATEMENT LIKE '%NEW.event_type<>''CREATED''%'
AND ACTION_STATEMENT LIKE '%active_slot=1%' AND ACTION_STATEMENT LIKE '%lease_token REGEXP%'
AND ACTION_STATEMENT LIKE '%lease_expires_at>CURRENT_TIMESTAMP(6)%' AND ACTION_STATEMENT LIKE '%SHA2%')
OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_log_validate_insert' AND EVENT_MANIPULATION='INSERT' AND ACTION_STATEMENT LIKE '%active leased scan range%' AND ACTION_STATEMENT LIKE '%raw_log_digest%' AND ACTION_STATEMENT LIKE '%row digest%' AND ACTION_STATEMENT LIKE '%SHA2%')
OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_batch_validate_insert' AND EVENT_MANIPULATION='INSERT' AND ACTION_STATEMENT LIKE '%range is not contiguous%' AND ACTION_STATEMENT LIKE '%first_log_ordinal%' AND ACTION_STATEMENT LIKE '%exact_logs%' AND ACTION_STATEMENT LIKE '%batch log evidence%' AND ACTION_STATEMENT LIKE '%SHA2%')
OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_manifest_validate_insert' AND EVENT_MANIPULATION='INSERT' AND ACTION_STATEMENT LIKE '%batch set is incomplete%' AND ACTION_STATEMENT LIKE '%range or log set is incomplete%' AND ACTION_STATEMENT LIKE '%evidence roots%' AND ACTION_STATEMENT LIKE '%active complete leased job%' AND ACTION_STATEMENT LIKE '%SHA2%')
OR (TRIGGER_NAME IN ('trg_chain_event_reorg_recovery_run_no_update','trg_chain_event_reorg_recovery_event_no_update','trg_chain_event_reorg_recovery_log_no_update','trg_chain_event_reorg_recovery_batch_no_update','trg_chain_event_reorg_recovery_manifest_no_update') AND EVENT_MANIPULATION='UPDATE' AND ACTION_STATEMENT LIKE '%append-only%')
OR (TRIGGER_NAME IN ('trg_chain_event_reorg_recovery_run_no_delete','trg_chain_event_reorg_recovery_job_no_delete','trg_chain_event_reorg_recovery_event_no_delete','trg_chain_event_reorg_recovery_log_no_delete','trg_chain_event_reorg_recovery_batch_no_delete','trg_chain_event_reorg_recovery_manifest_no_delete') AND EVENT_MANIPULATION='DELETE' AND ACTION_STATEMENT LIKE '%SIGNAL%')
OR (TRIGGER_NAME='trg_chain_event_reorg_recovery_definition_validate_insert' AND EVENT_MANIPULATION='INSERT' AND ACTION_STATEMENT LIKE '%definition attestation digest mismatch%' AND ACTION_STATEMENT LIKE '%SHA2%')
OR (TRIGGER_NAME IN ('trg_chain_event_reorg_recovery_definition_no_update','trg_chain_event_reorg_recovery_definition_no_delete') AND ACTION_STATEMENT LIKE '%definition attestations are append-only%' AND ACTION_STATEMENT LIKE '%SIGNAL%'))`},
	{name: "exact definition count", want: 34, sql: `SELECT
(SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
AND CONSTRAINT_TYPE='CHECK' AND TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_recovery_job',
'chain_event_reorg_recovery_event','chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch',
'chain_event_reorg_recovery_manifest','chain_event_reorg_recovery_definition'))
+(SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
AND EVENT_OBJECT_TABLE IN ('chain_event_reorg_recovery_run','chain_event_reorg_recovery_job',
'chain_event_reorg_recovery_event','chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch',
'chain_event_reorg_recovery_manifest','chain_event_reorg_recovery_definition'))`},
	{name: "definition attestation count", want: 34, sql: `SELECT COUNT(*) FROM chain_event_reorg_recovery_definition`},
	{name: "exact definition attestations", want: 31, sql: `SELECT COUNT(*) FROM chain_event_reorg_recovery_definition d
JOIN (
SELECT 'CHECK' object_type,tc.CONSTRAINT_NAME object_name,LOWER(SHA2(CONCAT(
'joincare.mysql-check-definition.v1',CHAR(10),tc.TABLE_SCHEMA,CHAR(10),tc.TABLE_NAME,CHAR(10),
tc.ENFORCED,CHAR(10),cc.CHECK_CLAUSE
),256)) definition_digest
FROM information_schema.TABLE_CONSTRAINTS tc
JOIN information_schema.CHECK_CONSTRAINTS cc ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK'
AND tc.TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_recovery_job',
'chain_event_reorg_recovery_event','chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch',
'chain_event_reorg_recovery_manifest','chain_event_reorg_recovery_definition')
AND tc.TABLE_NAME<>'chain_event_reorg_recovery_run'
UNION ALL
SELECT 'TRIGGER',TRIGGER_NAME,LOWER(SHA2(CONCAT(
'joincare.mysql-trigger-definition.v1',CHAR(10),EVENT_OBJECT_SCHEMA,CHAR(10),EVENT_OBJECT_TABLE,CHAR(10),
ACTION_TIMING,CHAR(10),EVENT_MANIPULATION,CHAR(10),ACTION_ORIENTATION,CHAR(10),ACTION_STATEMENT,CHAR(10),
SQL_MODE,CHAR(10),DEFINER,CHAR(10),CHARACTER_SET_CLIENT,CHAR(10),COLLATION_CONNECTION,CHAR(10),DATABASE_COLLATION
),256))
FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND EVENT_OBJECT_TABLE IN (
'chain_event_reorg_recovery_run','chain_event_reorg_recovery_job','chain_event_reorg_recovery_event',
'chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch','chain_event_reorg_recovery_manifest',
'chain_event_reorg_recovery_definition')
) current_definition ON current_definition.object_type=d.object_type AND current_definition.object_name=d.object_name
AND current_definition.definition_digest=d.definition_digest
WHERE d.row_digest=LOWER(SHA2(CONCAT('joincare.chain-event.reorg-recovery-definition.v1',CHAR(10),
d.object_type,CHAR(10),d.object_name,CHAR(10),d.definition_digest),256))
AND d.object_name NOT IN ('chk_chain_event_reorg_recovery_run_identity','chk_chain_event_reorg_recovery_boundary','chk_chain_event_reorg_recovery_target')
AND (SELECT COUNT(*) FROM chain_event_reorg_recovery_definition)=34
AND (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
  AND CONSTRAINT_TYPE='CHECK' AND TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_recovery_job',
  'chain_event_reorg_recovery_event','chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch',
  'chain_event_reorg_recovery_manifest','chain_event_reorg_recovery_definition'))
 +(SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
  AND EVENT_OBJECT_TABLE IN ('chain_event_reorg_recovery_run','chain_event_reorg_recovery_job',
  'chain_event_reorg_recovery_event','chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch',
  'chain_event_reorg_recovery_manifest','chain_event_reorg_recovery_definition'))=34`},
}

var chainEventReorgMaterializationSchemaChecks = []requiredSchemaCheck{
	{name: "migration ledger", want: 1, sql: `SELECT COUNT(*) FROM schema_migrations
WHERE status='applied' AND filename='2026_08_24_chain_event_reorg_materialization_prepare.sql'
AND checksum='6ed7ad3d6a0fc817ba05a3119717fb45d1410c12acd9ee41811af9659a3b7d8b'
AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE status='applying')`},
	{name: "materialization tables", want: 7, sql: `SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
AND TABLE_TYPE='BASE TABLE' AND TABLE_NAME IN ('chain_event_reorg_materialization_run','chain_event_reorg_materialization_job',
'chain_event_reorg_materialization_old_source',
'chain_event_reorg_materialization_effect','chain_event_reorg_materialization_branch',
'chain_event_reorg_materialization_event','chain_event_reorg_materialization_manifest')`},
	{name: "exact column counts", want: 7, sql: `SELECT COUNT(*) FROM (
SELECT TABLE_NAME,COUNT(*) column_count FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()
AND TABLE_NAME IN ('chain_event_reorg_materialization_run','chain_event_reorg_materialization_job',
'chain_event_reorg_materialization_old_source',
'chain_event_reorg_materialization_effect','chain_event_reorg_materialization_branch',
'chain_event_reorg_materialization_event','chain_event_reorg_materialization_manifest') GROUP BY TABLE_NAME
) c WHERE (TABLE_NAME='chain_event_reorg_materialization_run' AND column_count=21)
OR (TABLE_NAME='chain_event_reorg_materialization_old_source' AND column_count=16)
OR (TABLE_NAME='chain_event_reorg_materialization_job' AND column_count=23)
OR (TABLE_NAME='chain_event_reorg_materialization_effect' AND column_count=21)
OR (TABLE_NAME='chain_event_reorg_materialization_branch' AND column_count=8)
OR (TABLE_NAME='chain_event_reorg_materialization_event' AND column_count=10)
OR (TABLE_NAME='chain_event_reorg_materialization_manifest' AND column_count=10)`},
	{name: "critical evidence definitions", want: 52, sql: `SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND (
(TABLE_NAME='chain_event_reorg_materialization_run' AND COLUMN_NAME IN ('run_uuid','algorithm_version','recovery_manifest_digest','contract_address','legacy_contract_address','boundary_block_hash','old_cursor_block_hash','target_safe_block_hash','old_record_source_digest','replacement_log_source_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_materialization_run' AND COLUMN_NAME IN ('expected_old_record_count','expected_replacement_log_count') AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_materialization_old_source' AND COLUMN_NAME IN ('event','block_hash','tx_hash','contract_address','contract_version','event_data_digest','previous_row_digest','row_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_materialization_old_source' AND COLUMN_NAME='event_data' AND COLUMN_TYPE='mediumblob' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_materialization_job' AND COLUMN_NAME IN ('state','next_branch','old_effect_digest','replacement_effect_digest','lease_token','last_error_code') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_materialization_job' AND COLUMN_NAME='fence_epoch' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_materialization_effect' AND COLUMN_NAME IN ('branch','source_table','block_hash','tx_hash','contract_address','contract_version','event_type','classification','semantic_digest','previous_effect_digest','effect_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_materialization_effect' AND COLUMN_NAME='semantic_payload' AND COLUMN_TYPE='mediumblob' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_materialization_branch' AND COLUMN_NAME IN ('branch','effect_digest','summary_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_materialization_event' AND COLUMN_NAME IN ('event_type','stage','error_code','previous_event_hash','event_hash') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_materialization_manifest' AND COLUMN_NAME IN ('recovery_manifest_digest','old_effect_digest','replacement_effect_digest','manifest_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO'))`},
	{name: "exact named indexes", want: 26, sql: `SELECT COUNT(*) FROM (
SELECT TABLE_NAME,INDEX_NAME,GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') columns_in_order,MIN(NON_UNIQUE) non_unique
FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
'chain_event_reorg_materialization_run','chain_event_reorg_materialization_job','chain_event_reorg_materialization_effect',
'chain_event_reorg_materialization_old_source',
'chain_event_reorg_materialization_branch','chain_event_reorg_materialization_event','chain_event_reorg_materialization_manifest')
GROUP BY TABLE_NAME,INDEX_NAME) s WHERE CONCAT(TABLE_NAME,'|',INDEX_NAME,'|',columns_in_order,'|',non_unique) IN (
'chain_event_reorg_materialization_run|PRIMARY|id|0','chain_event_reorg_materialization_run|uk_chain_event_reorg_materialization_uuid|run_uuid|0','chain_event_reorg_materialization_run|uk_chain_event_reorg_materialization_recovery|recovery_run_id|0','chain_event_reorg_materialization_run|uk_chain_event_reorg_materialization_incident|incident_id|0',
'chain_event_reorg_materialization_old_source|PRIMARY|id|0','chain_event_reorg_materialization_old_source|uk_chain_event_reorg_materialization_old_ordinal|run_id,ordinal|0','chain_event_reorg_materialization_old_source|uk_chain_event_reorg_materialization_old_record|run_id,chain_event_record_id|0','chain_event_reorg_materialization_old_source|uk_chain_event_reorg_materialization_old_digest|run_id,row_digest|0','chain_event_reorg_materialization_old_source|idx_chain_event_reorg_materialization_old_block|run_id,block_number,chain_event_record_id|1',
'chain_event_reorg_materialization_job|PRIMARY|run_id|0','chain_event_reorg_materialization_job|uk_chain_event_reorg_materialization_active|incident_id,active_slot|0','chain_event_reorg_materialization_job|idx_chain_event_reorg_materialization_job_state|state,updated_at,run_id|1',
'chain_event_reorg_materialization_effect|PRIMARY|id|0','chain_event_reorg_materialization_effect|uk_chain_event_reorg_materialization_effect_ordinal|run_id,ordinal|0','chain_event_reorg_materialization_effect|uk_chain_event_reorg_materialization_effect_branch|run_id,branch,branch_ordinal|0','chain_event_reorg_materialization_effect|uk_chain_event_reorg_materialization_effect_source|run_id,branch,source_table,source_id|0','chain_event_reorg_materialization_effect|uk_chain_event_reorg_materialization_effect_digest|run_id,effect_digest|0','chain_event_reorg_materialization_effect|idx_chain_event_reorg_materialization_effect_project|run_id,branch,project_id,ordinal|1',
'chain_event_reorg_materialization_branch|PRIMARY|id|0','chain_event_reorg_materialization_branch|uk_chain_event_reorg_materialization_branch|run_id,branch|0','chain_event_reorg_materialization_branch|uk_chain_event_reorg_materialization_branch_digest|summary_digest|0',
'chain_event_reorg_materialization_event|PRIMARY|id|0','chain_event_reorg_materialization_event|uk_chain_event_reorg_materialization_event_sequence|run_id,sequence_no|0','chain_event_reorg_materialization_event|uk_chain_event_reorg_materialization_event_hash|event_hash|0',
'chain_event_reorg_materialization_manifest|PRIMARY|run_id|0','chain_event_reorg_materialization_manifest|uk_chain_event_reorg_materialization_manifest_digest|manifest_digest|0')`},
	{name: "restricted foreign keys", want: 10, sql: `SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
AND CONSTRAINT_NAME REGEXP '^fk_chain_event_reorg_materialization_' AND DELETE_RULE='RESTRICT'`},
	{name: "enforced checks", want: 11, sql: `SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES' AND TABLE_NAME IN ('chain_event_reorg_materialization_run',
'chain_event_reorg_materialization_old_source',
'chain_event_reorg_materialization_job','chain_event_reorg_materialization_effect','chain_event_reorg_materialization_branch',
'chain_event_reorg_materialization_event','chain_event_reorg_materialization_manifest')`},
	{name: "attached triggers", want: 21, sql: `SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
AND EVENT_OBJECT_TABLE IN ('chain_event_reorg_materialization_run','chain_event_reorg_materialization_job',
'chain_event_reorg_materialization_old_source',
'chain_event_reorg_materialization_effect','chain_event_reorg_materialization_branch',
'chain_event_reorg_materialization_event','chain_event_reorg_materialization_manifest')`},
	{name: "attached definition count", want: 32, sql: `SELECT
(SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_TYPE='CHECK'
AND TABLE_NAME IN ('chain_event_reorg_materialization_run','chain_event_reorg_materialization_job',
'chain_event_reorg_materialization_old_source',
'chain_event_reorg_materialization_effect','chain_event_reorg_materialization_branch',
'chain_event_reorg_materialization_event','chain_event_reorg_materialization_manifest'))
+(SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND EVENT_OBJECT_TABLE IN (
'chain_event_reorg_materialization_run','chain_event_reorg_materialization_job','chain_event_reorg_materialization_effect',
'chain_event_reorg_materialization_old_source',
'chain_event_reorg_materialization_branch','chain_event_reorg_materialization_event','chain_event_reorg_materialization_manifest'))`},
}

var chainEventReorgAssessmentSchemaChecks = []requiredSchemaCheck{
	{name: "migration ledger", want: 3, sql: `SELECT COUNT(*) FROM schema_migrations
WHERE status='applied' AND ((filename='2026_08_25_chain_event_reorg_reversibility_assessment.sql'
AND checksum='31764df81b1f74ef8a00a8bd785b81f9835dd9132a3e02361c217c8ca98ab68e')
OR (filename='2026_08_25_zz_chain_event_reorg_assessment_fk_restrict.sql'
AND checksum='ae2387d475ca19edf3292592e051b7d00bc4ed2e167b3be7d788bb431c799575')
OR (filename='2026_08_26_chain_event_reorg_assessment_plan_json.sql'
AND checksum='32f952eb708663ed381a407ccea827a53f4c8e7436cc48efde0be8d4201b467b'))
AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE status='applying')`},
	{name: "assessment tables", want: 6, sql: `SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
AND TABLE_TYPE='BASE TABLE' AND TABLE_NAME IN ('chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker',
'chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest')`},
	{name: "exact column counts", want: 6, sql: `SELECT COUNT(*) FROM (
SELECT TABLE_NAME,COUNT(*) column_count FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()
AND TABLE_NAME IN ('chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker',
'chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest') GROUP BY TABLE_NAME
) c WHERE (TABLE_NAME='chain_event_reorg_assessment_run' AND column_count=14)
OR (TABLE_NAME='chain_event_reorg_assessment_job' AND column_count=21)
OR (TABLE_NAME='chain_event_reorg_assessment_disposition' AND column_count=17)
OR (TABLE_NAME='chain_event_reorg_assessment_blocker' AND column_count=10)
OR (TABLE_NAME='chain_event_reorg_assessment_event' AND column_count=9)
OR (TABLE_NAME='chain_event_reorg_assessment_manifest' AND column_count=14)`},
	{name: "critical evidence definitions", want: 37, sql: `SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND (
(TABLE_NAME='chain_event_reorg_assessment_run' AND COLUMN_NAME IN ('run_uuid','materialization_manifest_digest','algorithm_version','boundary_block_hash','target_safe_block_hash','materialization_effect_root') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_assessment_run' AND COLUMN_NAME='expected_effect_count' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_assessment_job' AND COLUMN_NAME IN ('state','disposition_digest','blocker_digest','lease_token','last_error_code') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_assessment_job' AND COLUMN_NAME='fence_epoch' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_assessment_disposition' AND COLUMN_NAME IN ('source_effect_digest','branch','contract_version','event_type','replayability','action_kind','blocker_code','plan_payload_digest','previous_row_digest','row_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_assessment_disposition' AND COLUMN_NAME='plan_payload' AND COLUMN_TYPE='mediumblob' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_assessment_blocker' AND COLUMN_NAME IN ('scope','blocker_code','previous_row_digest','row_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_assessment_event' AND COLUMN_NAME IN ('event_type','error_code','previous_event_hash','event_hash') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_assessment_manifest' AND COLUMN_NAME IN ('materialization_manifest_digest','outcome','disposition_digest','blocker_digest','manifest_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO'))`},
	{name: "exact named indexes", want: 22, sql: `SELECT COUNT(*) FROM (
SELECT TABLE_NAME,INDEX_NAME,GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') columns_in_order,MIN(NON_UNIQUE) non_unique
FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
'chain_event_reorg_assessment_run','chain_event_reorg_assessment_job','chain_event_reorg_assessment_disposition',
'chain_event_reorg_assessment_blocker','chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest')
GROUP BY TABLE_NAME,INDEX_NAME) s WHERE CONCAT(TABLE_NAME,'|',INDEX_NAME,'|',columns_in_order,'|',non_unique) IN (
'chain_event_reorg_assessment_run|PRIMARY|id|0','chain_event_reorg_assessment_run|uk_chain_event_reorg_assessment_uuid|run_uuid|0','chain_event_reorg_assessment_run|uk_chain_event_reorg_assessment_materialization|materialization_run_id|0','chain_event_reorg_assessment_run|uk_chain_event_reorg_assessment_incident|incident_id|0',
'chain_event_reorg_assessment_job|PRIMARY|run_id|0','chain_event_reorg_assessment_job|uk_chain_event_reorg_assessment_active|incident_id,active_slot|0','chain_event_reorg_assessment_job|idx_chain_event_reorg_assessment_job_state|state,updated_at,run_id|1',
'chain_event_reorg_assessment_disposition|PRIMARY|id|0','chain_event_reorg_assessment_disposition|uk_chain_event_reorg_assessment_disposition_ordinal|run_id,ordinal|0','chain_event_reorg_assessment_disposition|uk_chain_event_reorg_assessment_disposition_effect|run_id,materialization_effect_id|0','chain_event_reorg_assessment_disposition|uk_chain_event_reorg_assessment_disposition_digest|run_id,row_digest|0','chain_event_reorg_assessment_disposition|uk_chain_event_reorg_assessment_disposition_run_id|run_id,id|0','chain_event_reorg_assessment_disposition|idx_chain_event_reorg_assessment_disposition_project|run_id,project_id,ordinal|1',
'chain_event_reorg_assessment_blocker|PRIMARY|id|0','chain_event_reorg_assessment_blocker|uk_chain_event_reorg_assessment_blocker_ordinal|run_id,ordinal|0','chain_event_reorg_assessment_blocker|uk_chain_event_reorg_assessment_blocker_disposition|run_id,disposition_id|0','chain_event_reorg_assessment_blocker|uk_chain_event_reorg_assessment_blocker_digest|run_id,row_digest|0',
'chain_event_reorg_assessment_event|PRIMARY|id|0','chain_event_reorg_assessment_event|uk_chain_event_reorg_assessment_event_sequence|run_id,sequence_no|0','chain_event_reorg_assessment_event|uk_chain_event_reorg_assessment_event_hash|event_hash|0',
'chain_event_reorg_assessment_manifest|PRIMARY|run_id|0','chain_event_reorg_assessment_manifest|uk_chain_event_reorg_assessment_manifest_digest|manifest_digest|0')`},
	{name: "attached foreign key count", want: 10, sql: `SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
AND CONSTRAINT_TYPE='FOREIGN KEY' AND TABLE_NAME IN ('chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker','chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest')`},
	{name: "exact restricted foreign keys", want: 10, sql: `SELECT COUNT(*) FROM (
SELECT rc.TABLE_NAME,rc.CONSTRAINT_NAME,rc.REFERENCED_TABLE_NAME,rc.UPDATE_RULE,rc.DELETE_RULE,
GROUP_CONCAT(k.COLUMN_NAME ORDER BY k.ORDINAL_POSITION SEPARATOR ',') child_columns,
GROUP_CONCAT(k.REFERENCED_COLUMN_NAME ORDER BY k.ORDINAL_POSITION SEPARATOR ',') parent_columns
FROM information_schema.REFERENTIAL_CONSTRAINTS rc
JOIN information_schema.KEY_COLUMN_USAGE k ON k.CONSTRAINT_SCHEMA=rc.CONSTRAINT_SCHEMA
AND k.CONSTRAINT_NAME=rc.CONSTRAINT_NAME AND k.TABLE_NAME=rc.TABLE_NAME
WHERE rc.CONSTRAINT_SCHEMA=DATABASE() AND rc.UNIQUE_CONSTRAINT_SCHEMA=DATABASE()
AND k.REFERENCED_TABLE_SCHEMA=DATABASE()
GROUP BY rc.TABLE_NAME,rc.CONSTRAINT_NAME,rc.REFERENCED_TABLE_NAME,rc.UPDATE_RULE,rc.DELETE_RULE
) f WHERE CONCAT(TABLE_NAME,'|',CONSTRAINT_NAME,'|',child_columns,'|',REFERENCED_TABLE_NAME,'|',parent_columns,'|',UPDATE_RULE,'|',DELETE_RULE) IN (
'chain_event_reorg_assessment_run|fk_chain_event_reorg_assessment_run_incident|incident_id|chain_event_reorg_incident|id|RESTRICT|RESTRICT',
'chain_event_reorg_assessment_run|fk_chain_event_reorg_assessment_run_materialization|materialization_run_id|chain_event_reorg_materialization_run|id|RESTRICT|RESTRICT',
'chain_event_reorg_assessment_job|fk_chain_event_reorg_assessment_job_run|run_id|chain_event_reorg_assessment_run|id|RESTRICT|RESTRICT',
'chain_event_reorg_assessment_job|fk_chain_event_reorg_assessment_job_incident|incident_id|chain_event_reorg_incident|id|RESTRICT|RESTRICT',
'chain_event_reorg_assessment_disposition|fk_chain_event_reorg_assessment_disposition_run|run_id|chain_event_reorg_assessment_run|id|RESTRICT|RESTRICT',
'chain_event_reorg_assessment_disposition|fk_chain_event_reorg_assessment_disposition_effect|materialization_effect_id|chain_event_reorg_materialization_effect|id|RESTRICT|RESTRICT',
'chain_event_reorg_assessment_blocker|fk_chain_event_reorg_assessment_blocker_run|run_id|chain_event_reorg_assessment_run|id|RESTRICT|RESTRICT',
'chain_event_reorg_assessment_blocker|fk_chain_event_reorg_assessment_blocker_disposition|run_id,disposition_id|chain_event_reorg_assessment_disposition|run_id,id|RESTRICT|RESTRICT',
'chain_event_reorg_assessment_event|fk_chain_event_reorg_assessment_event_run|run_id|chain_event_reorg_assessment_run|id|RESTRICT|RESTRICT',
'chain_event_reorg_assessment_manifest|fk_chain_event_reorg_assessment_manifest_run|run_id|chain_event_reorg_assessment_run|id|RESTRICT|RESTRICT')`},
	{name: "enforced checks", want: 10, sql: `SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES' AND TABLE_NAME IN ('chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker','chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest')`},
	{name: "attached triggers", want: 18, sql: `SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
AND EVENT_OBJECT_TABLE IN ('chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker','chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest')`},
	{name: "attached definition count", want: 28, sql: `SELECT
(SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_TYPE='CHECK'
AND TABLE_NAME IN ('chain_event_reorg_assessment_run','chain_event_reorg_assessment_job','chain_event_reorg_assessment_disposition',
'chain_event_reorg_assessment_blocker','chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest'))
+(SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND EVENT_OBJECT_TABLE IN (
'chain_event_reorg_assessment_run','chain_event_reorg_assessment_job','chain_event_reorg_assessment_disposition',
'chain_event_reorg_assessment_blocker','chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest'))`},
}

var chainEventReorgPrerequisiteSchemaChecks = []requiredSchemaCheck{
	{name: "migration ledger", want: 1, sql: `SELECT COUNT(*) FROM schema_migrations
WHERE status='applied' AND filename='2026_08_27_chain_event_reorg_prerequisite_inventory.sql'
AND checksum='fa41cc6655bc2be808ca3b45e4ff42590d0126f6ed8dedc37141d553a8704e0a'
AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE status='applying')`},
	{name: "prerequisite tables", want: 6, sql: `SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
AND TABLE_TYPE='BASE TABLE' AND TABLE_NAME IN ('chain_event_reorg_prerequisite_run','chain_event_reorg_prerequisite_job',
'chain_event_reorg_prerequisite_artifact','chain_event_reorg_prerequisite_requirement',
'chain_event_reorg_prerequisite_event','chain_event_reorg_prerequisite_manifest')`},
	{name: "exact column counts", want: 6, sql: `SELECT COUNT(*) FROM (
SELECT TABLE_NAME,COUNT(*) column_count FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()
AND TABLE_NAME IN ('chain_event_reorg_prerequisite_run','chain_event_reorg_prerequisite_job',
'chain_event_reorg_prerequisite_artifact','chain_event_reorg_prerequisite_requirement',
'chain_event_reorg_prerequisite_event','chain_event_reorg_prerequisite_manifest') GROUP BY TABLE_NAME
) c WHERE (TABLE_NAME='chain_event_reorg_prerequisite_run' AND column_count=11)
OR (TABLE_NAME='chain_event_reorg_prerequisite_job' AND column_count=20)
OR (TABLE_NAME='chain_event_reorg_prerequisite_artifact' AND column_count=13)
OR (TABLE_NAME='chain_event_reorg_prerequisite_requirement' AND column_count=10)
OR (TABLE_NAME='chain_event_reorg_prerequisite_event' AND column_count=9)
OR (TABLE_NAME='chain_event_reorg_prerequisite_manifest' AND column_count=16)`},
	{name: "critical evidence definitions", want: 54, sql: `SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND (
(TABLE_NAME='chain_event_reorg_prerequisite_run' AND COLUMN_NAME IN ('run_uuid','assessment_manifest_digest','algorithm_version','boundary_block_hash') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_prerequisite_run' AND COLUMN_NAME IN ('incident_id','assessment_run_id','chain_id','boundary_block','expected_requirement_count') AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_prerequisite_job' AND COLUMN_NAME IN ('state','requirement_digest','artifact_digest','lease_token','last_error_code') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_prerequisite_job' AND COLUMN_NAME='active_slot' AND COLUMN_TYPE='tinyint unsigned' AND IS_NULLABLE='YES')
OR (TABLE_NAME='chain_event_reorg_prerequisite_job' AND COLUMN_NAME IN ('next_requirement_ordinal','requirement_count','unmet_count','untrusted_count','deferred_count','artifact_count','fence_epoch') AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_prerequisite_job' AND COLUMN_NAME='attempt_count' AND COLUMN_TYPE='int unsigned' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_prerequisite_job' AND COLUMN_NAME='lease_expires_at' AND COLUMN_TYPE='datetime(6)' AND IS_NULLABLE='YES')
OR (TABLE_NAME='chain_event_reorg_prerequisite_artifact' AND COLUMN_NAME IN ('requirement_code','artifact_kind','schema_version','source_kind','source_digest','previous_row_digest','row_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_prerequisite_artifact' AND COLUMN_NAME='source_payload' AND COLUMN_TYPE='mediumblob' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_prerequisite_requirement' AND COLUMN_NAME IN ('requirement_code','evidence_status','previous_row_digest','row_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_prerequisite_requirement' AND COLUMN_NAME='artifact_id' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='YES')
OR (TABLE_NAME='chain_event_reorg_prerequisite_event' AND COLUMN_NAME IN ('event_type','error_code','previous_event_hash','event_hash') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_prerequisite_manifest' AND COLUMN_NAME IN ('assessment_manifest_digest','outcome','requirement_digest','artifact_digest','event_digest','manifest_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_prerequisite_manifest' AND COLUMN_NAME='candidate_ready' AND COLUMN_TYPE='tinyint unsigned' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_prerequisite_manifest' AND COLUMN_NAME IN ('requirement_count','satisfied_count','unmet_count','untrusted_count','deferred_count','artifact_count') AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO'))`},
	{name: "exact named indexes", want: 21, sql: `SELECT COUNT(*) FROM (
SELECT TABLE_NAME,INDEX_NAME,GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') columns_in_order,MIN(NON_UNIQUE) non_unique
FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (
'chain_event_reorg_prerequisite_run','chain_event_reorg_prerequisite_job','chain_event_reorg_prerequisite_artifact',
'chain_event_reorg_prerequisite_requirement','chain_event_reorg_prerequisite_event','chain_event_reorg_prerequisite_manifest')
GROUP BY TABLE_NAME,INDEX_NAME) s WHERE CONCAT(TABLE_NAME,'|',INDEX_NAME,'|',columns_in_order,'|',non_unique) IN (
'chain_event_reorg_prerequisite_run|PRIMARY|id|0','chain_event_reorg_prerequisite_run|uk_chain_event_reorg_prerequisite_uuid|run_uuid|0','chain_event_reorg_prerequisite_run|uk_chain_event_reorg_prerequisite_assessment|assessment_run_id|0','chain_event_reorg_prerequisite_run|uk_chain_event_reorg_prerequisite_incident|incident_id|0',
'chain_event_reorg_prerequisite_job|PRIMARY|run_id|0','chain_event_reorg_prerequisite_job|uk_chain_event_reorg_prerequisite_active|incident_id,active_slot|0','chain_event_reorg_prerequisite_job|idx_chain_event_reorg_prerequisite_job_state|state,updated_at,run_id|1',
'chain_event_reorg_prerequisite_artifact|PRIMARY|id|0','chain_event_reorg_prerequisite_artifact|uk_chain_event_reorg_prerequisite_artifact_ordinal|run_id,ordinal|0','chain_event_reorg_prerequisite_artifact|uk_chain_event_reorg_prerequisite_artifact_digest|run_id,row_digest|0','chain_event_reorg_prerequisite_artifact|uk_chain_event_reorg_prerequisite_artifact_run_id|run_id,id|0',
'chain_event_reorg_prerequisite_requirement|PRIMARY|id|0','chain_event_reorg_prerequisite_requirement|uk_chain_event_reorg_prerequisite_requirement_ordinal|run_id,ordinal|0','chain_event_reorg_prerequisite_requirement|uk_chain_event_reorg_prerequisite_requirement_code|run_id,requirement_code|0','chain_event_reorg_prerequisite_requirement|uk_chain_event_reorg_prerequisite_requirement_digest|run_id,row_digest|0','chain_event_reorg_prerequisite_requirement|uk_chain_event_reorg_prerequisite_requirement_artifact|run_id,artifact_id|0',
'chain_event_reorg_prerequisite_event|PRIMARY|id|0','chain_event_reorg_prerequisite_event|uk_chain_event_reorg_prerequisite_event_sequence|run_id,sequence_no|0','chain_event_reorg_prerequisite_event|uk_chain_event_reorg_prerequisite_event_hash|event_hash|0',
'chain_event_reorg_prerequisite_manifest|PRIMARY|run_id|0','chain_event_reorg_prerequisite_manifest|uk_chain_event_reorg_prerequisite_manifest_digest|manifest_digest|0')`},
	{name: "exact restricted foreign keys", want: 9, sql: `SELECT COUNT(*) FROM (
SELECT rc.TABLE_NAME,rc.CONSTRAINT_NAME,rc.REFERENCED_TABLE_NAME,rc.UPDATE_RULE,rc.DELETE_RULE,
GROUP_CONCAT(k.COLUMN_NAME ORDER BY k.ORDINAL_POSITION SEPARATOR ',') child_columns,
GROUP_CONCAT(k.REFERENCED_COLUMN_NAME ORDER BY k.ORDINAL_POSITION SEPARATOR ',') parent_columns
FROM information_schema.REFERENTIAL_CONSTRAINTS rc
JOIN information_schema.KEY_COLUMN_USAGE k ON k.CONSTRAINT_SCHEMA=rc.CONSTRAINT_SCHEMA
AND k.CONSTRAINT_NAME=rc.CONSTRAINT_NAME AND k.TABLE_NAME=rc.TABLE_NAME
WHERE rc.CONSTRAINT_SCHEMA=DATABASE() AND rc.UNIQUE_CONSTRAINT_SCHEMA=DATABASE()
AND k.REFERENCED_TABLE_SCHEMA=DATABASE() AND rc.CONSTRAINT_NAME REGEXP '^fk_chain_event_reorg_prerequisite_'
GROUP BY rc.TABLE_NAME,rc.CONSTRAINT_NAME,rc.REFERENCED_TABLE_NAME,rc.UPDATE_RULE,rc.DELETE_RULE
) f WHERE CONCAT(TABLE_NAME,'|',CONSTRAINT_NAME,'|',child_columns,'|',REFERENCED_TABLE_NAME,'|',parent_columns,'|',UPDATE_RULE,'|',DELETE_RULE) IN (
'chain_event_reorg_prerequisite_run|fk_chain_event_reorg_prerequisite_run_incident|incident_id|chain_event_reorg_incident|id|RESTRICT|RESTRICT',
'chain_event_reorg_prerequisite_run|fk_chain_event_reorg_prerequisite_run_assessment|assessment_run_id|chain_event_reorg_assessment_run|id|RESTRICT|RESTRICT',
'chain_event_reorg_prerequisite_job|fk_chain_event_reorg_prerequisite_job_run|run_id|chain_event_reorg_prerequisite_run|id|RESTRICT|RESTRICT',
'chain_event_reorg_prerequisite_job|fk_chain_event_reorg_prerequisite_job_incident|incident_id|chain_event_reorg_incident|id|RESTRICT|RESTRICT',
'chain_event_reorg_prerequisite_artifact|fk_chain_event_reorg_prerequisite_artifact_run|run_id|chain_event_reorg_prerequisite_run|id|RESTRICT|RESTRICT',
'chain_event_reorg_prerequisite_requirement|fk_chain_event_reorg_prerequisite_requirement_run|run_id|chain_event_reorg_prerequisite_run|id|RESTRICT|RESTRICT',
'chain_event_reorg_prerequisite_requirement|fk_chain_event_reorg_prerequisite_requirement_artifact|run_id,artifact_id|chain_event_reorg_prerequisite_artifact|run_id,id|RESTRICT|RESTRICT',
'chain_event_reorg_prerequisite_event|fk_chain_event_reorg_prerequisite_event_run|run_id|chain_event_reorg_prerequisite_run|id|RESTRICT|RESTRICT',
'chain_event_reorg_prerequisite_manifest|fk_chain_event_reorg_prerequisite_manifest_run|run_id|chain_event_reorg_prerequisite_run|id|RESTRICT|RESTRICT')`},
	{name: "enforced checks", want: 8, sql: `SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES' AND TABLE_NAME IN ('chain_event_reorg_prerequisite_run',
'chain_event_reorg_prerequisite_job','chain_event_reorg_prerequisite_artifact','chain_event_reorg_prerequisite_requirement',
'chain_event_reorg_prerequisite_event','chain_event_reorg_prerequisite_manifest')`},
	{name: "attached triggers", want: 18, sql: `SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
AND EVENT_OBJECT_TABLE IN ('chain_event_reorg_prerequisite_run','chain_event_reorg_prerequisite_job',
'chain_event_reorg_prerequisite_artifact','chain_event_reorg_prerequisite_requirement',
'chain_event_reorg_prerequisite_event','chain_event_reorg_prerequisite_manifest')`},
	{name: "attached definition count", want: 26, sql: `SELECT
(SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_TYPE='CHECK'
AND TABLE_NAME IN ('chain_event_reorg_prerequisite_run','chain_event_reorg_prerequisite_job',
'chain_event_reorg_prerequisite_artifact','chain_event_reorg_prerequisite_requirement',
'chain_event_reorg_prerequisite_event','chain_event_reorg_prerequisite_manifest'))
+(SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND EVENT_OBJECT_TABLE IN (
'chain_event_reorg_prerequisite_run','chain_event_reorg_prerequisite_job','chain_event_reorg_prerequisite_artifact',
'chain_event_reorg_prerequisite_requirement','chain_event_reorg_prerequisite_event','chain_event_reorg_prerequisite_manifest'))`},
}

var chainEventReorgGenerationSchemaChecks = []requiredSchemaCheck{
	{name: "migration ledger", want: 2, sql: `SELECT COUNT(*) FROM schema_migrations
WHERE status='applied' AND ((filename='2026_08_28_chain_event_reorg_generation_foundation.sql'
AND checksum='467eebfacca807f72271b0414624c787e48fd4f0e0cebb834fcfaba2a4b04b15')
OR (filename='2026_09_03_fair_launch_project_model.sql'
AND checksum='5b8a9c561b7defa7d8221578312e1343a2ac914f740fd6a227869d8aee319f1b'))
AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE status='applying')`},
	{name: "generation tables", want: 15, sql: `SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
AND TABLE_TYPE='BASE TABLE' AND (TABLE_NAME LIKE 'chain_event_reorg_generation%' OR TABLE_NAME LIKE 'chain_event_reorg_candidate%')`},
	{name: "exact column counts", want: 15, sql: `SELECT COUNT(*) FROM (
SELECT TABLE_NAME,COUNT(*) column_count FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()
AND (TABLE_NAME LIKE 'chain_event_reorg_generation%' OR TABLE_NAME LIKE 'chain_event_reorg_candidate%') GROUP BY TABLE_NAME
) c WHERE CONCAT(TABLE_NAME,'|',column_count) IN (
'chain_event_reorg_generation|25','chain_event_reorg_generation_job|24','chain_event_reorg_generation_sender|11',
'chain_event_reorg_generation_submission|42','chain_event_reorg_generation_nonce|15','chain_event_reorg_generation_requirement|11',
'chain_event_reorg_generation_event|12','chain_event_reorg_generation_manifest|35','chain_event_reorg_generation_checkpoint|18',
'chain_event_reorg_candidate_project|14','chain_event_reorg_candidate_ledger|16','chain_event_reorg_candidate_v2_entitlement|16',
'chain_event_reorg_candidate_v1_fact|12','chain_event_reorg_candidate_wallet_aggregate|17','chain_event_reorg_candidate_delta|19')`},
	{name: "critical column definitions", want: 106, sql: `SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND (
(TABLE_NAME='chain_event_reorg_generation' AND COLUMN_NAME IN ('generation_uuid','generation_kind','prerequisite_manifest_digest','assessment_manifest_digest','materialization_manifest_digest','boundary_block_hash','target_safe_block_hash','legacy_contract_address','v2_contract_address','old_effect_root','replacement_effect_root','algorithm_version','schema_version','descriptor_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_generation' AND COLUMN_NAME IN ('id','incident_id','prerequisite_run_id','assessment_run_id','materialization_run_id','chain_id','boundary_block','target_safe_block') AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_generation' AND COLUMN_NAME='attempt_no' AND COLUMN_TYPE='int unsigned' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_generation' AND COLUMN_NAME='created_at' AND COLUMN_TYPE='datetime(6)' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='CURRENT_TIMESTAMP(6)')
OR (TABLE_NAME='chain_event_reorg_generation_job' AND COLUMN_NAME IN ('state','stage','sender_root','submission_root','nonce_root','requirement_root','lease_token','last_error_code') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_generation_job' AND COLUMN_NAME IN ('run_id','incident_id','sender_count','submission_count','submission_payload_bytes','nonce_count','requirement_count','unmet_count','verified_count','fence_epoch') AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_generation_job' AND COLUMN_NAME='active_slot' AND COLUMN_TYPE='tinyint unsigned' AND IS_NULLABLE='YES')
OR (TABLE_NAME='chain_event_reorg_generation_submission' AND COLUMN_NAME IN ('call_data_digest','raw_tx_digest','last_error_digest','cancel_reason_digest','generation_disposition','source_snapshot_digest','writer_lease_token_digest','previous_row_digest','row_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_generation_submission' AND COLUMN_NAME='call_data' AND DATA_TYPE='mediumblob' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_generation_submission' AND COLUMN_NAME='raw_tx' AND DATA_TYPE='mediumblob' AND IS_NULLABLE='YES')
OR (TABLE_NAME='chain_event_reorg_generation_nonce' AND COLUMN_NAME='sender_address' AND CHARACTER_SET_NAME='utf8mb4' AND COLLATION_NAME='utf8mb4_unicode_ci' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_generation_requirement' AND COLUMN_NAME IN ('requirement_code','evidence_status','writer_lease_token_digest','previous_row_digest','row_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_generation_event' AND COLUMN_NAME IN ('event_type','stage','error_code','writer_lease_token_digest','previous_event_hash','event_hash') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_generation_manifest' AND COLUMN_NAME IN ('descriptor_digest','outcome','sender_root','submission_root','nonce_root','requirement_root','candidate_root','delta_root','checkpoint_root','event_root','writer_lease_token_digest','manifest_digest') AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_generation_manifest' AND COLUMN_NAME='candidate_ready' AND COLUMN_TYPE='tinyint unsigned' AND IS_NULLABLE='NO')
OR (TABLE_NAME='chain_event_reorg_candidate_delta' AND COLUMN_NAME IN ('boundary_raw','old_branch_delta','replacement_branch_delta','candidate_raw','switch_delta') AND DATA_TYPE='mediumtext' AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
OR (TABLE_NAME IN ('chain_event_reorg_generation_sender','chain_event_reorg_generation_submission','chain_event_reorg_generation_nonce','chain_event_reorg_generation_requirement','chain_event_reorg_generation_event','chain_event_reorg_generation_manifest','chain_event_reorg_candidate_project','chain_event_reorg_candidate_ledger','chain_event_reorg_candidate_v2_entitlement','chain_event_reorg_candidate_v1_fact','chain_event_reorg_candidate_wallet_aggregate','chain_event_reorg_candidate_delta','chain_event_reorg_generation_checkpoint') AND COLUMN_NAME='writer_fence_epoch' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO')
OR (TABLE_NAME IN ('chain_event_reorg_generation_sender','chain_event_reorg_generation_submission','chain_event_reorg_generation_nonce','chain_event_reorg_generation_requirement','chain_event_reorg_generation_event','chain_event_reorg_generation_manifest','chain_event_reorg_candidate_project','chain_event_reorg_candidate_ledger','chain_event_reorg_candidate_v2_entitlement','chain_event_reorg_candidate_v1_fact','chain_event_reorg_candidate_wallet_aggregate','chain_event_reorg_candidate_delta','chain_event_reorg_generation_checkpoint') AND COLUMN_NAME='writer_lease_token_digest' AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin' AND IS_NULLABLE='NO')
)`},
	{name: "exact index inventory", want: 56, sql: `SELECT COUNT(*) FROM (
SELECT TABLE_NAME,INDEX_NAME,GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') columns_in_order,MIN(NON_UNIQUE) non_unique
FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE()
AND (TABLE_NAME LIKE 'chain_event_reorg_generation%' OR TABLE_NAME LIKE 'chain_event_reorg_candidate%')
GROUP BY TABLE_NAME,INDEX_NAME) s WHERE CONCAT(TABLE_NAME,'|',INDEX_NAME,'|',columns_in_order,'|',non_unique) IN (
'chain_event_reorg_generation|PRIMARY|id|0','chain_event_reorg_generation|uk_chain_event_reorg_generation_uuid|generation_uuid|0','chain_event_reorg_generation|uk_chain_event_reorg_generation_attempt|incident_id,attempt_no|0','chain_event_reorg_generation|uk_chain_event_reorg_generation_descriptor|descriptor_digest|0','chain_event_reorg_generation|idx_chain_event_reorg_generation_prerequisite|prerequisite_run_id|1','chain_event_reorg_generation|fk_chain_event_reorg_generation_assessment|assessment_run_id|1','chain_event_reorg_generation|fk_chain_event_reorg_generation_materialization|materialization_run_id|1',
'chain_event_reorg_generation_job|PRIMARY|run_id|0','chain_event_reorg_generation_job|uk_chain_event_reorg_generation_active|incident_id,active_slot|0','chain_event_reorg_generation_job|idx_chain_event_reorg_generation_job_state|state,updated_at,run_id|1',
'chain_event_reorg_generation_sender|PRIMARY|id|0','chain_event_reorg_generation_sender|uk_chain_event_reorg_generation_sender_ordinal|run_id,ordinal|0','chain_event_reorg_generation_sender|uk_chain_event_reorg_generation_sender_identity|run_id,chain_id,sender_address|0','chain_event_reorg_generation_sender|uk_chain_event_reorg_generation_sender_digest|run_id,row_digest|0',
'chain_event_reorg_generation_submission|PRIMARY|id|0','chain_event_reorg_generation_submission|uk_chain_event_reorg_generation_submission_ordinal|run_id,ordinal|0','chain_event_reorg_generation_submission|uk_chain_event_reorg_generation_submission_source|run_id,source_submission_id|0','chain_event_reorg_generation_submission|uk_chain_event_reorg_generation_submission_digest|run_id,row_digest|0','chain_event_reorg_generation_submission|fk_chain_event_reorg_generation_submission_source|source_submission_id|1',
'chain_event_reorg_generation_nonce|PRIMARY|id|0','chain_event_reorg_generation_nonce|uk_chain_event_reorg_generation_nonce_ordinal|run_id,ordinal|0','chain_event_reorg_generation_nonce|uk_chain_event_reorg_generation_nonce_source|run_id,chain_id,sender_address|0','chain_event_reorg_generation_nonce|uk_chain_event_reorg_generation_nonce_digest|run_id,row_digest|0','chain_event_reorg_generation_nonce|fk_chain_event_reorg_generation_nonce_source|chain_id,sender_address|1','chain_event_reorg_generation_nonce|fk_chain_event_reorg_generation_nonce_active|run_id,active_submission_id|1',
'chain_event_reorg_generation_requirement|PRIMARY|id|0','chain_event_reorg_generation_requirement|uk_chain_event_reorg_generation_requirement_ordinal|run_id,ordinal|0','chain_event_reorg_generation_requirement|uk_chain_event_reorg_generation_requirement_code|run_id,requirement_code|0','chain_event_reorg_generation_requirement|uk_chain_event_reorg_generation_requirement_digest|run_id,row_digest|0',
'chain_event_reorg_generation_event|PRIMARY|id|0','chain_event_reorg_generation_event|uk_chain_event_reorg_generation_event_sequence|run_id,sequence_no|0','chain_event_reorg_generation_event|uk_chain_event_reorg_generation_event_hash|event_hash|0',
'chain_event_reorg_generation_manifest|PRIMARY|run_id|0','chain_event_reorg_generation_manifest|uk_chain_event_reorg_generation_manifest_digest|manifest_digest|0','chain_event_reorg_generation_manifest|fk_chain_event_reorg_generation_manifest_incident|incident_id|1',
'chain_event_reorg_generation_checkpoint|PRIMARY|id|0','chain_event_reorg_generation_checkpoint|uk_chain_event_reorg_generation_checkpoint_run|run_id|0','chain_event_reorg_generation_checkpoint|uk_chain_event_reorg_generation_checkpoint_digest|checkpoint_digest|0',
'chain_event_reorg_candidate_project|PRIMARY|id|0','chain_event_reorg_candidate_project|uk_chain_event_reorg_candidate_project_ordinal|run_id,ordinal|0','chain_event_reorg_candidate_project|uk_chain_event_reorg_candidate_project_identity|run_id,chain_id,contract_address,contract_version,project_id_raw|0',
'chain_event_reorg_candidate_ledger|PRIMARY|id|0','chain_event_reorg_candidate_ledger|uk_chain_event_reorg_candidate_ledger_ordinal|run_id,ordinal|0','chain_event_reorg_candidate_ledger|uk_chain_event_reorg_candidate_ledger_identity|run_id,chain_id,contract_address,tx_hash,log_index|0',
'chain_event_reorg_candidate_v2_entitlement|PRIMARY|id|0','chain_event_reorg_candidate_v2_entitlement|uk_chain_event_reorg_candidate_entitlement_ordinal|run_id,ordinal|0','chain_event_reorg_candidate_v2_entitlement|uk_chain_event_reorg_candidate_entitlement_identity|run_id,chain_id,contract_address,tx_hash,log_index|0',
'chain_event_reorg_candidate_v1_fact|PRIMARY|id|0','chain_event_reorg_candidate_v1_fact|uk_chain_event_reorg_candidate_v1_ordinal|run_id,ordinal|0','chain_event_reorg_candidate_v1_fact|uk_chain_event_reorg_candidate_v1_identity|run_id,stable_identity_digest|0',
'chain_event_reorg_candidate_wallet_aggregate|PRIMARY|id|0','chain_event_reorg_candidate_wallet_aggregate|uk_chain_event_reorg_candidate_wallet_ordinal|run_id,ordinal|0','chain_event_reorg_candidate_wallet_aggregate|uk_chain_event_reorg_candidate_wallet_identity|run_id,chain_id,contract_address,project_id_raw,wallet_address,token_address,aggregate_kind|0',
'chain_event_reorg_candidate_delta|PRIMARY|id|0','chain_event_reorg_candidate_delta|uk_chain_event_reorg_candidate_delta_ordinal|run_id,ordinal|0','chain_event_reorg_candidate_delta|uk_chain_event_reorg_candidate_delta_identity|run_id,domain,stable_identity_digest,asset_role,token_address|0')`},
	{name: "source range index", want: 1, sql: `SELECT COUNT(*) FROM (
SELECT INDEX_NAME,GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') columns_in_order,MIN(NON_UNIQUE) non_unique
FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_tx_submission'
AND INDEX_NAME='idx_chain_tx_generation_sender' GROUP BY INDEX_NAME
) s WHERE columns_in_order='chain_id,sender_address,id' AND non_unique=1`},
	{name: "restricted foreign keys", want: 23, sql: `SELECT COUNT(*) FROM (
SELECT rc.TABLE_NAME,rc.CONSTRAINT_NAME,rc.REFERENCED_TABLE_NAME,rc.UPDATE_RULE,rc.DELETE_RULE,
GROUP_CONCAT(k.COLUMN_NAME ORDER BY k.ORDINAL_POSITION SEPARATOR ',') child_columns,
GROUP_CONCAT(k.REFERENCED_COLUMN_NAME ORDER BY k.ORDINAL_POSITION SEPARATOR ',') parent_columns
FROM information_schema.REFERENTIAL_CONSTRAINTS rc JOIN information_schema.KEY_COLUMN_USAGE k
ON k.CONSTRAINT_SCHEMA=rc.CONSTRAINT_SCHEMA AND k.CONSTRAINT_NAME=rc.CONSTRAINT_NAME AND k.TABLE_NAME=rc.TABLE_NAME
WHERE rc.CONSTRAINT_SCHEMA=DATABASE() AND rc.UNIQUE_CONSTRAINT_SCHEMA=DATABASE() AND k.REFERENCED_TABLE_SCHEMA=DATABASE()
AND rc.CONSTRAINT_NAME REGEXP '^fk_chain_event_reorg_(generation|candidate)_' GROUP BY rc.TABLE_NAME,rc.CONSTRAINT_NAME,
rc.REFERENCED_TABLE_NAME,rc.UPDATE_RULE,rc.DELETE_RULE
) f WHERE CONCAT(TABLE_NAME,'|',CONSTRAINT_NAME,'|',child_columns,'|',REFERENCED_TABLE_NAME,'|',parent_columns,'|',UPDATE_RULE,'|',DELETE_RULE) IN (
'chain_event_reorg_generation|fk_chain_event_reorg_generation_incident|incident_id|chain_event_reorg_incident|id|RESTRICT|RESTRICT',
'chain_event_reorg_generation|fk_chain_event_reorg_generation_prerequisite|prerequisite_run_id|chain_event_reorg_prerequisite_run|id|RESTRICT|RESTRICT',
'chain_event_reorg_generation|fk_chain_event_reorg_generation_assessment|assessment_run_id|chain_event_reorg_assessment_run|id|RESTRICT|RESTRICT',
'chain_event_reorg_generation|fk_chain_event_reorg_generation_materialization|materialization_run_id|chain_event_reorg_materialization_run|id|RESTRICT|RESTRICT',
'chain_event_reorg_generation_job|fk_chain_event_reorg_generation_job_run|run_id|chain_event_reorg_generation|id|RESTRICT|RESTRICT',
'chain_event_reorg_generation_job|fk_chain_event_reorg_generation_job_incident|incident_id|chain_event_reorg_incident|id|RESTRICT|RESTRICT',
'chain_event_reorg_generation_sender|fk_chain_event_reorg_generation_sender_run|run_id|chain_event_reorg_generation|id|RESTRICT|RESTRICT',
'chain_event_reorg_generation_submission|fk_chain_event_reorg_generation_submission_run|run_id|chain_event_reorg_generation|id|RESTRICT|RESTRICT',
'chain_event_reorg_generation_submission|fk_chain_event_reorg_generation_submission_source|source_submission_id|chain_tx_submission|id|RESTRICT|RESTRICT',
'chain_event_reorg_generation_nonce|fk_chain_event_reorg_generation_nonce_run|run_id|chain_event_reorg_generation|id|RESTRICT|RESTRICT',
'chain_event_reorg_generation_nonce|fk_chain_event_reorg_generation_nonce_source|chain_id,sender_address|chain_relayer_nonce|chain_id,sender_address|RESTRICT|RESTRICT',
'chain_event_reorg_generation_nonce|fk_chain_event_reorg_generation_nonce_active|run_id,active_submission_id|chain_event_reorg_generation_submission|run_id,source_submission_id|RESTRICT|RESTRICT',
'chain_event_reorg_generation_requirement|fk_chain_event_reorg_generation_requirement_run|run_id|chain_event_reorg_generation|id|RESTRICT|RESTRICT',
'chain_event_reorg_generation_event|fk_chain_event_reorg_generation_event_run|run_id|chain_event_reorg_generation|id|RESTRICT|RESTRICT',
'chain_event_reorg_generation_manifest|fk_chain_event_reorg_generation_manifest_run|run_id|chain_event_reorg_generation|id|RESTRICT|RESTRICT',
'chain_event_reorg_generation_manifest|fk_chain_event_reorg_generation_manifest_incident|incident_id|chain_event_reorg_incident|id|RESTRICT|RESTRICT',
'chain_event_reorg_candidate_project|fk_chain_event_reorg_candidate_project_run|run_id|chain_event_reorg_generation|id|RESTRICT|RESTRICT',
'chain_event_reorg_candidate_ledger|fk_chain_event_reorg_candidate_ledger_run|run_id|chain_event_reorg_generation|id|RESTRICT|RESTRICT',
'chain_event_reorg_candidate_v2_entitlement|fk_chain_event_reorg_candidate_entitlement_run|run_id|chain_event_reorg_generation|id|RESTRICT|RESTRICT',
'chain_event_reorg_candidate_v1_fact|fk_chain_event_reorg_candidate_v1_run|run_id|chain_event_reorg_generation|id|RESTRICT|RESTRICT',
'chain_event_reorg_candidate_wallet_aggregate|fk_chain_event_reorg_candidate_wallet_run|run_id|chain_event_reorg_generation|id|RESTRICT|RESTRICT',
'chain_event_reorg_candidate_delta|fk_chain_event_reorg_candidate_delta_run|run_id|chain_event_reorg_generation|id|RESTRICT|RESTRICT',
'chain_event_reorg_generation_checkpoint|fk_chain_event_reorg_generation_checkpoint_run|run_id|chain_event_reorg_generation|id|RESTRICT|RESTRICT')`},
	{name: "enforced checks", want: 20, sql: `SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
AND CONSTRAINT_TYPE='CHECK' AND ENFORCED='YES' AND (TABLE_NAME LIKE 'chain_event_reorg_generation%' OR TABLE_NAME LIKE 'chain_event_reorg_candidate%')`},
	{name: "attached triggers", want: 45, sql: `SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
AND (EVENT_OBJECT_TABLE LIKE 'chain_event_reorg_generation%' OR EVENT_OBJECT_TABLE LIKE 'chain_event_reorg_candidate%')`},
	{name: "attached definition count", want: 65, sql: `SELECT
(SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_TYPE='CHECK'
AND (TABLE_NAME LIKE 'chain_event_reorg_generation%' OR TABLE_NAME LIKE 'chain_event_reorg_candidate%'))
+(SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
AND (EVENT_OBJECT_TABLE LIKE 'chain_event_reorg_generation%' OR EVENT_OBJECT_TABLE LIKE 'chain_event_reorg_candidate%'))`},
}

// RequireIDOPurchaseEntitlement blocks core/job startup until the migration that
// makes V2 Purchased payment facts and project-token entitlements atomic is complete.
func RequireIDOPurchaseEntitlement(database *gorm.DB) error {
	if database == nil {
		return errors.New("purchase entitlement schema check requires a database")
	}
	database = primarySchemaDatabase(database)
	for _, check := range idoPurchaseEntitlementSchemaChecks {
		var got int64
		if err := database.Raw(check.sql).Scan(&got).Error; err != nil {
			return fmt.Errorf("check purchase entitlement schema %s: %w", check.name, err)
		}
		if got != check.want {
			return fmt.Errorf("purchase entitlement schema %s incomplete: got=%d want=%d; apply 2026_08_20 before starting core/job", check.name, got, check.want)
		}
	}
	return nil
}

// RequireChainEventCollationCompatibility keeps the outbox preflight and
// legacy event lookups deterministic across the original mixed-collation
// tables before any relayer or chain-event worker starts.
func RequireChainEventCollationCompatibility(database *gorm.DB) error {
	if database == nil {
		return errors.New("chain event collation compatibility check requires a database")
	}
	database = primarySchemaDatabase(database)
	for _, check := range chainEventCollationCompatibilitySchemaChecks {
		var got int64
		if err := database.Raw(check.sql).Scan(&got).Error; err != nil {
			return fmt.Errorf("check chain event collation compatibility %s: %w", check.name, err)
		}
		if got != check.want {
			return fmt.Errorf("chain event collation compatibility %s incomplete: got=%d want=%d; apply 2026_08_16_zz before starting core/job", check.name, got, check.want)
		}
	}
	return nil
}

// RequireIDOReconciliationEvidence blocks both core and job until durable
// reconciliation queue and evidence protections are fully installed.
func RequireIDOReconciliationEvidence(database *gorm.DB) error {
	if database == nil {
		return errors.New("IDO reconciliation evidence schema check requires a database")
	}
	database = primarySchemaDatabase(database)
	for _, check := range idoReconciliationEvidenceSchemaChecks {
		var got int64
		if err := database.Raw(check.sql).Scan(&got).Error; err != nil {
			return fmt.Errorf("check IDO reconciliation evidence schema %s: %w", check.name, err)
		}
		if got != check.want {
			return fmt.Errorf("IDO reconciliation evidence schema %s incomplete: got=%d want=%d; apply 2026_08_21 before starting core/job", check.name, got, check.want)
		}
	}
	return nil
}

// RequireChainEventReorgEvidence keeps every service fail-closed until durable
// post-commit canonicality incidents and cursor checkpoints are installed.
func RequireChainEventReorgEvidence(database *gorm.DB) error {
	if database == nil {
		return errors.New("chain event reorg evidence schema check requires a database")
	}
	database = primarySchemaDatabase(database)
	for _, check := range chainEventReorgEvidenceSchemaChecks {
		var got int64
		if err := database.Raw(check.sql).Scan(&got).Error; err != nil {
			return fmt.Errorf("check chain event reorg evidence schema %s: %w", check.name, err)
		}
		if got != check.want {
			return fmt.Errorf("chain event reorg evidence schema %s incomplete: got=%d want=%d; apply 2026_08_22 before starting core/job", check.name, got, check.want)
		}
	}
	return nil
}

// RequireChainEventReorgRecoveryScan blocks core/job until replacement-branch
// staging has durable, append-only and database-verifiable evidence tables.
func RequireChainEventReorgRecoveryScan(database *gorm.DB) error {
	if database == nil {
		return errors.New("chain event reorg recovery schema check requires a database")
	}
	database = primarySchemaDatabase(database)
	for _, check := range chainEventReorgRecoverySchemaChecks {
		var got int64
		if err := database.Raw(check.sql).Scan(&got).Error; err != nil {
			return fmt.Errorf("check chain event reorg recovery schema %s: %w", check.name, err)
		}
		if got != check.want {
			return fmt.Errorf("chain event reorg recovery schema %s incomplete: got=%d want=%d; apply 2026_08_23 before starting core/job", check.name, got, check.want)
		}
	}
	return requireApprovedChainEventReorgRecoveryDefinitions(database)
}

// RequireChainEventReorgMaterialization blocks core/job until the B2-A
// append-only semantic effect preparation layer and its exact physical
// definitions match the approved binary manifest.
func RequireChainEventReorgMaterialization(database *gorm.DB) error {
	if database == nil {
		return errors.New("chain event reorg materialization schema check requires a database")
	}
	database = primarySchemaDatabase(database)
	for _, check := range chainEventReorgMaterializationSchemaChecks {
		var got int64
		if err := database.Raw(check.sql).Scan(&got).Error; err != nil {
			return fmt.Errorf("check chain event reorg materialization schema %s: %w", check.name, err)
		}
		if got != check.want {
			return fmt.Errorf("chain event reorg materialization schema %s incomplete: got=%d want=%d; apply 2026_08_24 before starting core/job", check.name, got, check.want)
		}
	}
	return requireApprovedChainEventReorgMaterializationDefinitions(database)
}

// RequireChainEventReorgAssessment blocks core/job until the B2-B0
// reversibility assessment remains physically fail-closed and cannot be
// mistaken for generation-switch authorization.
func RequireChainEventReorgAssessment(database *gorm.DB) error {
	if database == nil {
		return errors.New("chain event reorg assessment schema check requires a database")
	}
	database = primarySchemaDatabase(database)
	for _, check := range chainEventReorgAssessmentSchemaChecks {
		var got int64
		if err := database.Raw(check.sql).Scan(&got).Error; err != nil {
			return fmt.Errorf("check chain event reorg assessment schema %s: %w", check.name, err)
		}
		if got != check.want {
			return fmt.Errorf("chain event reorg assessment schema %s incomplete: got=%d want=%d; apply through 2026_08_26 before starting core/job", check.name, got, check.want)
		}
	}
	return requireApprovedChainEventReorgAssessmentDefinitions(database)
}

// RequireChainEventReorgPrerequisite keeps B2-B1-A physically evidence-only:
// seven unresolved requirements, no trusted artifacts, no candidate readiness,
// and no generation-switch authority.
func RequireChainEventReorgPrerequisite(database *gorm.DB) error {
	if database == nil {
		return errors.New("chain event reorg prerequisite schema check requires a database")
	}
	database = primarySchemaDatabase(database)
	for _, check := range chainEventReorgPrerequisiteSchemaChecks {
		var got int64
		if err := database.Raw(check.sql).Scan(&got).Error; err != nil {
			return fmt.Errorf("check chain event reorg prerequisite schema %s: %w", check.name, err)
		}
		if got != check.want {
			return fmt.Errorf("chain event reorg prerequisite schema %s incomplete: got=%d want=%d; apply 2026_08_27 before starting core/job", check.name, got, check.want)
		}
	}
	return requireApprovedChainEventReorgPrerequisiteDefinitions(database)
}

// RequireChainEventReorgGeneration keeps B1-B0 physically BLOCKED: complete
// runtime outbox/nonce snapshots are allowed, but trusted artifacts, candidate
// facts, signed deltas, checkpoints, switching, and incident resolution are not.
func RequireChainEventReorgGeneration(database *gorm.DB) error {
	if database == nil {
		return errors.New("chain event reorg generation schema check requires a database")
	}
	database = primarySchemaDatabase(database)
	for _, check := range chainEventReorgGenerationSchemaChecks {
		var got int64
		if err := database.Raw(check.sql).Scan(&got).Error; err != nil {
			return fmt.Errorf("check chain event reorg generation schema %s: %w", check.name, err)
		}
		if got != check.want {
			return fmt.Errorf("chain event reorg generation schema %s incomplete: got=%d want=%d; apply 2026_08_28 before starting core/job", check.name, got, check.want)
		}
	}
	return requireApprovedChainEventReorgGenerationDefinitions(database)
}
