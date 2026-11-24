package schema

import (
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestSchemaGateQueriesForcePrimary(t *testing.T) {
	database, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "user:password@tcp(127.0.0.1:3306)/joincare",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	primary := primarySchemaDatabase(database)
	if _, ok := primary.Statement.Settings.Load("gorm:db_resolver:write"); !ok {
		t.Fatal("schema gate database is not forced to dbresolver write source")
	}
}

func TestChainEventCollationCompatibilityChecksStayFailClosed(t *testing.T) {
	if len(chainEventCollationCompatibilitySchemaChecks) != 3 {
		t.Fatalf("unexpected collation compatibility check count: %d", len(chainEventCollationCompatibilitySchemaChecks))
	}
	var productionSQL strings.Builder
	for _, check := range chainEventCollationCompatibilitySchemaChecks {
		if check.want <= 0 || strings.TrimSpace(check.name) == "" || strings.TrimSpace(check.sql) == "" {
			t.Fatalf("invalid collation compatibility check: %+v", check)
		}
		productionSQL.WriteString(check.sql)
		productionSQL.WriteByte('\n')
	}
	for _, required := range []string{
		"2026_08_16_zz_chain_event_collation_compat.sql", "1c708627f8bb643ff31660ec64fa8c7e1d7536e4389ede2e3acf06be8bd6a71b",
		"2026_08_17_chain_tx_outbox.sql", "c0d392343fdb3608117afaf8a2d245d986b4a7f3b0ed2206873bc1230cdd8003",
		"status='applying'", "chain_event_record", "transaction_record", "utf8mb4_unicode_ci", "block_number,tx_hash,log_index",
	} {
		if !strings.Contains(productionSQL.String(), required) {
			t.Fatalf("required collation compatibility identity %q is not covered", required)
		}
	}
}

func TestPurchaseEntitlementSchemaChecksStayFailClosed(t *testing.T) {
	if len(idoPurchaseEntitlementSchemaChecks) != 10 {
		t.Fatalf("unexpected schema check count: %d", len(idoPurchaseEntitlementSchemaChecks))
	}
	var productionSQL strings.Builder
	for _, check := range idoPurchaseEntitlementSchemaChecks {
		if check.want <= 0 || strings.TrimSpace(check.name) == "" || strings.TrimSpace(check.sql) == "" {
			t.Fatalf("invalid production schema check: %+v", check)
		}
		productionSQL.WriteString(check.sql)
		productionSQL.WriteByte('\n')
	}
	for _, required := range []string{
		"schema_migrations", "status='applying'", "2026_08_20_ido_purchase_entitlement.sql", "ido_purchase_entitlement", "fund_ledger", "fund_ledger_id", "chain_id,tx_hash,log_index", "chk_fund_ledger_status",
		"fk_ido_purchase_entitlement_ledger", "chk_ido_purchase_entitlement_status", "trg_ido_purchase_entitlement_validate_insert", "trg_ido_purchase_entitlement_no_update", "trg_ido_purchase_entitlement_no_delete",
	} {
		if !strings.Contains(productionSQL.String(), required) {
			t.Fatalf("required schema identity %q is not covered", required)
		}
	}
}

func TestChainEventReorgEvidenceSchemaChecksStayFailClosed(t *testing.T) {
	if len(chainEventReorgEvidenceSchemaChecks) != 7 {
		t.Fatalf("unexpected chain event reorg schema check count: %d", len(chainEventReorgEvidenceSchemaChecks))
	}
	var productionSQL strings.Builder
	for _, check := range chainEventReorgEvidenceSchemaChecks {
		if check.want <= 0 || strings.TrimSpace(check.name) == "" || strings.TrimSpace(check.sql) == "" {
			t.Fatalf("invalid chain event reorg schema check: %+v", check)
		}
		productionSQL.WriteString(check.sql)
		productionSQL.WriteByte('\n')
	}
	for _, required := range []string{
		"2026_08_22_chain_event_reorg_incident.sql", "ba1590f9d1005b1548e785c176ecb8da146fc7244c5d8ea7e552a983d16db512", "status='applying'",
		"chain_event_block_checkpoint", "chain_event_reorg_incident", "chain_event_reorg_event", "uk_chain_event_checkpoint_parent", "uk_chain_event_reorg_active",
		"fk_chain_event_reorg_checkpoint", "DELETE_RULE", "ENFORCED", "checkpoint chain", "cursor checkpoint identity", "SHA2", "append-only",
	} {
		if !strings.Contains(productionSQL.String(), required) {
			t.Fatalf("required chain event reorg schema identity %q is not covered", required)
		}
	}
}

func TestChainEventReorgRecoverySchemaChecksStayFailClosed(t *testing.T) {
	if len(chainEventReorgRecoverySchemaChecks) != 11 {
		t.Fatalf("unexpected chain event reorg recovery schema check count: %d", len(chainEventReorgRecoverySchemaChecks))
	}
	var productionSQL strings.Builder
	for _, check := range chainEventReorgRecoverySchemaChecks {
		if check.want <= 0 || strings.TrimSpace(check.name) == "" || strings.TrimSpace(check.sql) == "" {
			t.Fatalf("invalid chain event reorg recovery schema check: %+v", check)
		}
		productionSQL.WriteString(check.sql)
		productionSQL.WriteByte('\n')
	}
	for _, required := range []string{
		"2026_08_23_chain_event_reorg_recovery_scan.sql", "2093bc1489c5d2af38219bd6732b954981cabadfb4375b419e9d47e1b8d1994f", "status='applying'",
		"chain_event_reorg_recovery_run", "chain_event_reorg_recovery_job", "chain_event_reorg_recovery_event", "chain_event_reorg_recovery_log", "chain_event_reorg_recovery_batch", "chain_event_reorg_recovery_manifest",
		"chain_event_reorg_recovery_definition", "definition_digest=d.definition_digest", "joincare.mysql-check-definition.v1", "joincare.mysql-trigger-definition.v1", "EVENT_OBJECT_TABLE", "ACTION_TIMING", "EVENT_MANIPULATION", "ACTION_ORIENTATION", "SQL_MODE", "DEFINER", "DATABASE_COLLATION", "lease_token", "fence_epoch", "raw_log", "first_log_ordinal", "uk_chain_event_reorg_recovery_active", "CHECK_CLAUSE", "lease_expires_at>CURRENT_TIMESTAMP(6)", "DELETE_RULE", "ENFORCED", "SHA2", "append-only",
	} {
		if !strings.Contains(productionSQL.String(), required) {
			t.Fatalf("required chain event reorg recovery schema identity %q is not covered", required)
		}
	}
}

func TestChainEventReorgMaterializationSchemaChecksStayFailClosed(t *testing.T) {
	if len(chainEventReorgMaterializationSchemaChecks) != 9 {
		t.Fatalf("unexpected materialization schema check count: %d", len(chainEventReorgMaterializationSchemaChecks))
	}
	var productionSQL strings.Builder
	for _, check := range chainEventReorgMaterializationSchemaChecks {
		if check.want <= 0 || strings.TrimSpace(check.name) == "" || strings.TrimSpace(check.sql) == "" {
			t.Fatalf("invalid materialization schema check: %+v", check)
		}
		productionSQL.WriteString(check.sql)
		productionSQL.WriteByte('\n')
	}
	for _, required := range []string{
		"2026_08_24_chain_event_reorg_materialization_prepare.sql", "6ed7ad3d6a0fc817ba05a3119717fb45d1410c12acd9ee41811af9659a3b7d8b", "status='applying'",
		"chain_event_reorg_materialization_run", "chain_event_reorg_materialization_old_source", "chain_event_reorg_materialization_job", "chain_event_reorg_materialization_effect", "chain_event_reorg_materialization_branch", "chain_event_reorg_materialization_event", "chain_event_reorg_materialization_manifest",
		"semantic_payload", "fence_epoch", "expected_replacement_log_count", "uk_chain_event_reorg_materialization_effect_source", "DELETE_RULE", "ENFORCED", "EVENT_OBJECT_TABLE",
	} {
		if !strings.Contains(productionSQL.String(), required) {
			t.Fatalf("required materialization schema identity %q is not covered", required)
		}
	}
}

func TestChainEventReorgAssessmentSchemaChecksStayFailClosed(t *testing.T) {
	if len(chainEventReorgAssessmentSchemaChecks) != 10 {
		t.Fatalf("unexpected assessment schema check count: %d", len(chainEventReorgAssessmentSchemaChecks))
	}
	var productionSQL strings.Builder
	for _, check := range chainEventReorgAssessmentSchemaChecks {
		if check.want <= 0 || strings.TrimSpace(check.name) == "" || strings.TrimSpace(check.sql) == "" {
			t.Fatalf("invalid assessment schema check: %+v", check)
		}
		productionSQL.WriteString(check.sql)
		productionSQL.WriteByte('\n')
	}
	for _, required := range []string{
		"2026_08_25_chain_event_reorg_reversibility_assessment.sql", "31764df81b1f74ef8a00a8bd785b81f9835dd9132a3e02361c217c8ca98ab68e", "2026_08_25_zz_chain_event_reorg_assessment_fk_restrict.sql", "ae2387d475ca19edf3292592e051b7d00bc4ed2e167b3be7d788bb431c799575", "2026_08_26_chain_event_reorg_assessment_plan_json.sql", "32f952eb708663ed381a407ccea827a53f4c8e7436cc48efde0be8d4201b467b", "status='applying'",
		"chain_event_reorg_assessment_run", "chain_event_reorg_assessment_job", "chain_event_reorg_assessment_disposition", "chain_event_reorg_assessment_blocker", "chain_event_reorg_assessment_event", "chain_event_reorg_assessment_manifest",
		"materialization_effect_id", "plan_payload", "fence_epoch", "uk_chain_event_reorg_assessment_disposition_effect", "DELETE_RULE", "UPDATE_RULE", "REFERENCED_COLUMN_NAME", "UNIQUE_CONSTRAINT_SCHEMA=DATABASE()", "REFERENCED_TABLE_SCHEMA=DATABASE()", "fk_chain_event_reorg_assessment_blocker_disposition", "run_id,disposition_id", "ENFORCED", "EVENT_OBJECT_TABLE",
	} {
		if !strings.Contains(productionSQL.String(), required) {
			t.Fatalf("required assessment schema identity %q is not covered", required)
		}
	}
}

func TestChainEventReorgPrerequisiteSchemaChecksStayFailClosed(t *testing.T) {
	if len(chainEventReorgPrerequisiteSchemaChecks) != 9 {
		t.Fatalf("unexpected prerequisite schema check count: %d", len(chainEventReorgPrerequisiteSchemaChecks))
	}
	var productionSQL strings.Builder
	for _, check := range chainEventReorgPrerequisiteSchemaChecks {
		if check.want <= 0 || strings.TrimSpace(check.name) == "" || strings.TrimSpace(check.sql) == "" {
			t.Fatalf("invalid prerequisite schema check: %+v", check)
		}
		productionSQL.WriteString(check.sql)
		productionSQL.WriteByte('\n')
	}
	for _, required := range []string{
		"2026_08_27_chain_event_reorg_prerequisite_inventory.sql", "fa41cc6655bc2be808ca3b45e4ff42590d0126f6ed8dedc37141d553a8704e0a", "status='applying'",
		"chain_event_reorg_prerequisite_run", "chain_event_reorg_prerequisite_job", "chain_event_reorg_prerequisite_artifact",
		"chain_event_reorg_prerequisite_requirement", "chain_event_reorg_prerequisite_event", "chain_event_reorg_prerequisite_manifest",
		"candidate_ready", "artifact_count", "uk_chain_event_reorg_prerequisite_requirement_artifact",
		"DELETE_RULE", "UPDATE_RULE", "UNIQUE_CONSTRAINT_SCHEMA=DATABASE()", "REFERENCED_TABLE_SCHEMA=DATABASE()", "ENFORCED", "EVENT_OBJECT_TABLE",
	} {
		if !strings.Contains(productionSQL.String(), required) {
			t.Fatalf("required prerequisite schema identity %q is not covered", required)
		}
	}
}

func TestChainEventReorgGenerationSchemaChecksStayFailClosed(t *testing.T) {
	if len(chainEventReorgGenerationSchemaChecks) != 10 {
		t.Fatalf("unexpected generation schema check count: %d", len(chainEventReorgGenerationSchemaChecks))
	}
	var productionSQL strings.Builder
	for _, check := range chainEventReorgGenerationSchemaChecks {
		if check.want <= 0 || strings.TrimSpace(check.name) == "" || strings.TrimSpace(check.sql) == "" {
			t.Fatalf("invalid generation schema check: %+v", check)
		}
		productionSQL.WriteString(check.sql)
		productionSQL.WriteByte('\n')
	}
	for _, required := range []string{
		"2026_08_28_chain_event_reorg_generation_foundation.sql", "467eebfacca807f72271b0414624c787e48fd4f0e0cebb834fcfaba2a4b04b15",
		"2026_09_03_fair_launch_project_model.sql", "5b8a9c561b7defa7d8221578312e1343a2ac914f740fd6a227869d8aee319f1b", "status='applying'",
		"chain_event_reorg_generation_submission", "chain_event_reorg_generation_nonce", "chain_event_reorg_generation_requirement",
		"chain_event_reorg_generation_manifest", "chain_event_reorg_candidate_delta", "chain_event_reorg_generation_checkpoint",
		"idx_chain_tx_generation_sender", "writer_fence_epoch", "writer_lease_token_digest", "candidate_ready",
		"DELETE_RULE", "UPDATE_RULE", "UNIQUE_CONSTRAINT_SCHEMA=DATABASE()", "REFERENCED_TABLE_SCHEMA=DATABASE()", "ENFORCED", "EVENT_OBJECT_TABLE",
	} {
		if !strings.Contains(productionSQL.String(), required) {
			t.Fatalf("required generation schema identity %q is not covered", required)
		}
	}
}

func TestIDOReconciliationEvidenceSchemaChecksStayFailClosed(t *testing.T) {
	if len(idoReconciliationEvidenceSchemaChecks) != 8 {
		t.Fatalf("unexpected reconciliation evidence schema check count: %d", len(idoReconciliationEvidenceSchemaChecks))
	}
	var productionSQL strings.Builder
	for _, check := range idoReconciliationEvidenceSchemaChecks {
		if check.want <= 0 || strings.TrimSpace(check.name) == "" || strings.TrimSpace(check.sql) == "" {
			t.Fatalf("invalid reconciliation evidence schema check: %+v", check)
		}
		productionSQL.WriteString(check.sql)
		productionSQL.WriteByte('\n')
	}
	for _, required := range []string{
		"2026_08_21_ido_reconciliation_evidence.sql", "741cee24965e05d546b5d5e19f65d08e56c2e300bf35222e9228749e02844b13", "status='applying'", "ido_reconciliation_run", "ido_reconciliation_job", "ido_reconciliation_event", "ido_reconciliation_wallet", "ido_reconciliation_check", "ido_reconciliation_manifest",
		"uk_ido_reconciliation_active_project", "uk_ido_reconciliation_event_sequence", "uk_ido_reconciliation_wallet_ordinal", "uk_ido_reconciliation_check_ordinal", "uk_ido_reconciliation_manifest_parent",
		"DELETE_RULE", "CHECK_CLAUSE", "expected_refunded", "wallet_evidence_digest", "check_evidence_digest", "canonical queued state", "wallet-identity.v1", "check-identity.v1", "NEW.message", "actual_wallets*4", "SHA2", "matching manifest", "append-only",
	} {
		if !strings.Contains(productionSQL.String(), required) {
			t.Fatalf("required reconciliation evidence schema identity %q is not covered", required)
		}
	}
}
