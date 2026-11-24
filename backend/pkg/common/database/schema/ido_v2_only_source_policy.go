package schema

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const v2OnlySourcePolicyMigration = "2026_08_29_chain_event_v2_only_source_policy.sql"
const v2OnlySourcePolicyChecksum = "dc383eb5b8422a09a0d1d95fa807bc99db40aa33de2ead5bb38f746f6037fb7f"

var approvedV2OnlySourcePolicyDefinitions = map[string]string{
	// Exported from a fresh disposable MySQL 8.0.45 schema after 08_29 postflight.
	"CHECK:chk_chain_event_reorg_recovery_run_identity":                 "61096d3aa7bf3b1aee5fb7745c0eb58375a283201526603c035aa80d83a12ff6",
	"CHECK:chk_chain_event_reorg_recovery_boundary":                     "2fa6aeab9444260c2af8a9b37731b648b96413186f8981825d86cfa538754640",
	"CHECK:chk_chain_event_reorg_recovery_target":                       "6746d10f02c759383f51eb82ec158a3a530e3a0f6f8fbda280ece983525ad11c",
	"CHECK:chk_chain_event_reorg_materialization_run_identity":          "a60ae7044671b9f6ffa0a25d0421117d748db65b4b6ac5c40676be240078dff1",
	"CHECK:chk_chain_event_reorg_materialization_run_range":             "b784f1e307a0d483880f96c888bfba0d4837a3926132b738776c68be1149e4a6",
	"CHECK:chk_chain_event_reorg_materialization_run_sources":           "0a7fd05b6b45cd60aaca1bcda211221547aa408aed16604c7554e20a704213c1",
	"CHECK:chk_chain_event_reorg_generation_identity":                   "1eb25923130924196b0928fb078bc891f5fa1d9437382e18d1f2be8783ae2830",
	"CHECK:chk_chain_event_reorg_generation_sources":                    "e0133613e86f72810fdcb0eed93cb07a6e83148d4e1f8345b6222826cef22470",
	"TRIGGER:trg_chain_event_reorg_materialization_run_validate_insert": "7c7b380784aeea22aa069986e608f18702ea853b5b6847f6b1d0ac77d12b6693",
	"TRIGGER:trg_chain_event_reorg_generation_validate_insert":          "695f569df40e82f17d5415f61729a2ddc676c4dd7ee41256a12be94de34f9ac1",
}

const v2OnlySourcePolicyDefinitionSQL = `
SELECT 'CHECK' object_type,tc.CONSTRAINT_NAME object_name,tc.TABLE_NAME object_table,
tc.ENFORCED enforced,'' action_timing,'' event_manipulation,'' action_orientation,cc.CHECK_CLAUSE definition
FROM information_schema.TABLE_CONSTRAINTS tc
JOIN information_schema.CHECK_CONSTRAINTS cc ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.TABLE_NAME IN (
'chain_event_reorg_recovery_run','chain_event_reorg_materialization_run','chain_event_reorg_generation')
UNION ALL
SELECT 'TRIGGER',TRIGGER_NAME,EVENT_OBJECT_TABLE,'',ACTION_TIMING,EVENT_MANIPULATION,ACTION_ORIENTATION,ACTION_STATEMENT
FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN (
'trg_chain_event_reorg_materialization_run_validate_insert','trg_chain_event_reorg_generation_validate_insert')`

func RequireV2OnlySourcePolicy(database *gorm.DB) error {
	if database == nil {
		return errors.New("V2-only source policy schema check requires a database")
	}
	database = primarySchemaDatabase(database)
	var ledger int64
	if err := database.Raw(`SELECT COUNT(*) FROM schema_migrations WHERE status='applied' AND filename=? AND checksum=?
AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE status='applying')`, v2OnlySourcePolicyMigration, v2OnlySourcePolicyChecksum).Scan(&ledger).Error; err != nil {
		return fmt.Errorf("check V2-only source policy migration ledger: %w", err)
	}
	if ledger != 1 {
		return fmt.Errorf("V2-only source policy migration ledger incomplete: got=%d want=1; apply 2026_08_29 before starting core/job", ledger)
	}
	var columns int64
	if err := database.Raw(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()
AND TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_materialization_run','chain_event_reorg_generation')
AND COLUMN_NAME='legacy_source_registered' AND COLUMN_TYPE='tinyint unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='1'`).Scan(&columns).Error; err != nil {
		return fmt.Errorf("check V2-only source policy columns: %w", err)
	}
	if columns != 3 {
		return fmt.Errorf("V2-only source policy columns incomplete: got=%d want=3", columns)
	}
	var definitions []chainEventReorgRecoveryDefinition
	if err := database.Raw(v2OnlySourcePolicyDefinitionSQL).Scan(&definitions).Error; err != nil {
		return fmt.Errorf("read V2-only source policy definitions: %w", err)
	}
	if len(definitions) != len(approvedV2OnlySourcePolicyDefinitions) {
		return fmt.Errorf("V2-only source policy definition count is not approved: got=%d want=%d", len(definitions), len(approvedV2OnlySourcePolicyDefinitions))
	}
	seen := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		key := strings.ToUpper(definition.ObjectType) + ":" + definition.ObjectName
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate V2-only source policy definition: %s", key)
		}
		seen[key] = struct{}{}
		digest, err := chainEventReorgRecoveryDefinitionDigest(definition)
		if err != nil {
			return fmt.Errorf("invalid V2-only source policy definition %s: %w", key, err)
		}
		if approved, ok := approvedV2OnlySourcePolicyDefinitions[key]; !ok || approved == "" || approved != digest {
			return fmt.Errorf("V2-only source policy definition is not approved: %s digest=%s", key, digest)
		}
	}
	return nil
}
