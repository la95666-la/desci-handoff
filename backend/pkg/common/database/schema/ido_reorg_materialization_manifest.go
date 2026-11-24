package schema

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const chainEventReorgMaterializationDefinitionSQL = `
SELECT 'CHECK' object_type,tc.CONSTRAINT_NAME object_name,tc.TABLE_NAME object_table,
tc.ENFORCED enforced,'' action_timing,'' event_manipulation,'' action_orientation,cc.CHECK_CLAUSE definition
FROM information_schema.TABLE_CONSTRAINTS tc
JOIN information_schema.CHECK_CONSTRAINTS cc
  ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK'
AND LOWER(tc.TABLE_NAME) IN ('chain_event_reorg_materialization_run','chain_event_reorg_materialization_job',
'chain_event_reorg_materialization_old_source',
'chain_event_reorg_materialization_effect','chain_event_reorg_materialization_branch',
'chain_event_reorg_materialization_event','chain_event_reorg_materialization_manifest')
AND tc.TABLE_NAME<>'chain_event_reorg_materialization_run'
UNION ALL
SELECT 'TRIGGER',TRIGGER_NAME,EVENT_OBJECT_TABLE,'',ACTION_TIMING,EVENT_MANIPULATION,ACTION_ORIENTATION,ACTION_STATEMENT
FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND LOWER(EVENT_OBJECT_TABLE) IN (
'chain_event_reorg_materialization_run','chain_event_reorg_materialization_job',
'chain_event_reorg_materialization_old_source',
'chain_event_reorg_materialization_effect','chain_event_reorg_materialization_branch',
'chain_event_reorg_materialization_event','chain_event_reorg_materialization_manifest')
AND NOT (EVENT_OBJECT_TABLE='chain_event_reorg_materialization_run'
  AND TRIGGER_NAME='trg_chain_event_reorg_materialization_run_validate_insert')`

var approvedChainEventReorgMaterializationDefinitions = map[string]string{
	"CHECK:chk_chain_event_reorg_materialization_branch_fields":              "7f9eb51adba0092dbe48bbe7740dde1a8599241583f048806b924edf5ed77a11",
	"CHECK:chk_chain_event_reorg_materialization_effect_identity":            "5476430299f5bffb8a74103ecc80f8d9d77902e1f14dad292be18e1f948708ed",
	"CHECK:chk_chain_event_reorg_materialization_effect_semantics":           "2f8777a0ab66fe885083394976bb3ceeec7aabebce0f524cfe7279a50c67549c",
	"CHECK:chk_chain_event_reorg_materialization_event_fields":               "51a6141bfcb796c9f4788c34f013ef3040681e3d2181d037b40fa4e2220f4835",
	"CHECK:chk_chain_event_reorg_materialization_job_checkpoint":             "18c3fd3c53b0ac564ad3e400fa8702733bf040b4b123e5e3e9d5deda1244779c",
	"CHECK:chk_chain_event_reorg_materialization_job_state":                  "fa9a7b494df9848686194e7115b8d3d68e3e9afb7b0185fb65b31ff2e8a32990",
	"CHECK:chk_chain_event_reorg_materialization_manifest_fields":            "c3eed88888d32216fc03a08724cc08aed33e9afa48a624d1380b8d81abd92222",
	"CHECK:chk_chain_event_reorg_materialization_old_source":                 "1b6dee3dd575314fbecda93463cd5332d3da8d427e384d3a9ea659ff91ab0347",
	"TRIGGER:trg_chain_event_reorg_materialization_branch_no_delete":         "38544f855e14d7be5b4a29eda9d1582bad4bee2f31343e39b917176030ce0b93",
	"TRIGGER:trg_chain_event_reorg_materialization_branch_no_update":         "b7286259a523d9698bb9eeb78bac55d8012e00f43e625a99205e5b0e298da73c",
	"TRIGGER:trg_chain_event_reorg_materialization_branch_validate_insert":   "11a28087f2b4e0efafeb89680abfb92db6982a2c6f938c209dd5f66e99e7e6f9",
	"TRIGGER:trg_chain_event_reorg_materialization_effect_no_delete":         "8910f636da7eaedb001f37f6e135a9ff985e52a1858bcf2483d03215fccb3cd6",
	"TRIGGER:trg_chain_event_reorg_materialization_effect_no_update":         "5036d2ebebc1d73c168a13be33c0173c1f76409434a04cd7ad84c0edfbe60e90",
	"TRIGGER:trg_chain_event_reorg_materialization_effect_validate_insert":   "61e3fb5a8f7a868caf9fb41e1d64f717295d758ce2dbf48142fe84792dec29d1",
	"TRIGGER:trg_chain_event_reorg_materialization_event_no_delete":          "60ba3c3bb8a4855ffcecc37b2d045687019e0ef29922ec3eb31e26c8c67b8530",
	"TRIGGER:trg_chain_event_reorg_materialization_event_no_update":          "767d9f2743975604b2b102aef24cfa9281bb4204e102a595c3fc946f18bff06a",
	"TRIGGER:trg_chain_event_reorg_materialization_event_validate_insert":    "6c226e4b4f476b7a9deabce8dd429e8e617c46f401510cc252341547cc7677be",
	"TRIGGER:trg_chain_event_reorg_materialization_job_no_delete":            "efa05812cf934c55790a8aa0e66023c3e0fd5a9405731e530ac850a07999cd25",
	"TRIGGER:trg_chain_event_reorg_materialization_job_validate_insert":      "f08a22d96d68c3fb587e34af91e79b24160bbbfadc457d60f84484fb8ea842eb",
	"TRIGGER:trg_chain_event_reorg_materialization_job_validate_update":      "360ce6d9fa781ff50aff855e6e3e1565180111858c8c4174c6f32b48fdfaeb9a",
	"TRIGGER:trg_chain_event_reorg_materialization_manifest_no_delete":       "a7fa32245c6cfabd075b7c63907daf9a8059caa6aa97a727624c1c223ac4c422",
	"TRIGGER:trg_chain_event_reorg_materialization_manifest_no_update":       "0357e2d4615482be9a55c779e1026c217a6667c63b14d00535992f9af3880214",
	"TRIGGER:trg_chain_event_reorg_materialization_manifest_validate_insert": "c9d091ac83f1427353f618c952995de3bf5695fe54dfb13ef5dee4271e051649",
	"TRIGGER:trg_chain_event_reorg_materialization_old_no_delete":            "923356dd091cf6b317d94e45ff6620c49966ba7f245f7e3315e87d8bf3ff5bb0",
	"TRIGGER:trg_chain_event_reorg_materialization_old_no_update":            "2e9cb8db2b9b1095ddcd30923d4cd34a1c16e34236e0afe65cafc66f13fe3eef",
	"TRIGGER:trg_chain_event_reorg_materialization_old_validate_insert":      "1ee4f1c3662c9bc246a2c4c71dd43a2abe5bdd96c19e65dee56a31624c01d79e",
	"TRIGGER:trg_chain_event_reorg_materialization_run_no_delete":            "adff8d327b00ea3c6968db849b0c17d2b449172a96cc234badd96ea5d2b0f425",
	"TRIGGER:trg_chain_event_reorg_materialization_run_no_update":            "4e109299bb05e99569a8cf37a04c69fce39cbfb2643d4695ed0240c19dd33932",
}

func requireApprovedChainEventReorgMaterializationDefinitions(database *gorm.DB) error {
	database = primarySchemaDatabase(database)
	var definitions []chainEventReorgRecoveryDefinition
	if err := database.Raw(chainEventReorgMaterializationDefinitionSQL).Scan(&definitions).Error; err != nil {
		return fmt.Errorf("read chain event reorg materialization definitions: %w", err)
	}
	if len(definitions) != len(approvedChainEventReorgMaterializationDefinitions) {
		return fmt.Errorf("chain event reorg materialization definition count is not approved: got=%d want=%d", len(definitions), len(approvedChainEventReorgMaterializationDefinitions))
	}
	seen := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		key := strings.ToUpper(definition.ObjectType) + ":" + definition.ObjectName
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("duplicate chain event reorg materialization definition: %s", key)
		}
		seen[key] = struct{}{}
		digest, err := chainEventReorgRecoveryDefinitionDigest(definition)
		if err != nil {
			return fmt.Errorf("invalid chain event reorg materialization definition %s: %w", key, err)
		}
		if approved, ok := approvedChainEventReorgMaterializationDefinitions[key]; !ok || approved != digest {
			return fmt.Errorf("chain event reorg materialization definition is not approved: %s", key)
		}
	}
	return nil
}
