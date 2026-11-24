package schema

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const chainEventReorgPrerequisiteDefinitionSQL = `
SELECT 'CHECK' object_type,tc.CONSTRAINT_NAME object_name,tc.TABLE_NAME object_table,
tc.ENFORCED enforced,'' action_timing,'' event_manipulation,'' action_orientation,cc.CHECK_CLAUSE definition
FROM information_schema.TABLE_CONSTRAINTS tc
JOIN information_schema.CHECK_CONSTRAINTS cc
  ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK'
AND LOWER(tc.TABLE_NAME) IN ('chain_event_reorg_prerequisite_run','chain_event_reorg_prerequisite_job',
'chain_event_reorg_prerequisite_artifact','chain_event_reorg_prerequisite_requirement',
'chain_event_reorg_prerequisite_event','chain_event_reorg_prerequisite_manifest')
UNION ALL
SELECT 'TRIGGER',TRIGGER_NAME,EVENT_OBJECT_TABLE,'',ACTION_TIMING,EVENT_MANIPULATION,ACTION_ORIENTATION,ACTION_STATEMENT
FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND LOWER(EVENT_OBJECT_TABLE) IN (
'chain_event_reorg_prerequisite_run','chain_event_reorg_prerequisite_job',
'chain_event_reorg_prerequisite_artifact','chain_event_reorg_prerequisite_requirement',
'chain_event_reorg_prerequisite_event','chain_event_reorg_prerequisite_manifest')`

var approvedChainEventReorgPrerequisiteDefinitions = map[string]string{
	"CHECK:chk_chain_event_reorg_prerequisite_artifact_fields":               "c2b32b997b26f452d50eef2e41a049a575bbe82b91d13225aa6065e11641cd8f",
	"CHECK:chk_chain_event_reorg_prerequisite_event_fields":                  "c61c78f26211fa04e325de4504b78d34bf11584700f3197a27ef37ac6d827763",
	"CHECK:chk_chain_event_reorg_prerequisite_job_checkpoint":                "edf114630f5c3016e983fb1fd7f94171aad04882b227ab8756a18fa8f3e36ccc",
	"CHECK:chk_chain_event_reorg_prerequisite_job_state":                     "eb172b39404efe3e4832ca02810ddb35f1c6a10b329808b129a913afd62109fb",
	"CHECK:chk_chain_event_reorg_prerequisite_manifest_fields":               "76de65912341242f807b069e37c2ce40b2fcf773593ad56ab11f38257a6d8b80",
	"CHECK:chk_chain_event_reorg_prerequisite_requirement_fields":            "05a244f02a297390c68bab90d543655e9aee7aec1a7f687f3e1f60c847c4f51a",
	"CHECK:chk_chain_event_reorg_prerequisite_run_identity":                  "b53b15aec3a094341f8de86e393abe546187815019c9844cfa741739dbb0ad84",
	"CHECK:chk_chain_event_reorg_prerequisite_run_source":                    "c7c2abb556056ded08cacb58515b8d5d8f7393c72802f976b1ec0963c6b093a8",
	"TRIGGER:trg_chain_event_reorg_prerequisite_artifact_no_delete":          "63c4005c013ad40cce7f4cfc304f2743c95beeadfbca473e115cf38b3cf0659c",
	"TRIGGER:trg_chain_event_reorg_prerequisite_artifact_no_update":          "c4ddba220a760b9d6f6f4a5bac271c74af12d06cf361429d1fde2b21755227b6",
	"TRIGGER:trg_chain_event_reorg_prerequisite_artifact_validate_insert":    "1a7999d0de8cc233b4de37c4c542ce885cb1b113f158933786e2a5b9e9415a0b",
	"TRIGGER:trg_chain_event_reorg_prerequisite_event_no_delete":             "59fc35ca43fce25c60126c141ac4d8bb806163df03b9deac20133613b5287e71",
	"TRIGGER:trg_chain_event_reorg_prerequisite_event_no_update":             "20ecab2dc573d61918fff7ca19cc9e6ba658c025a577808ee846dc4170168b68",
	"TRIGGER:trg_chain_event_reorg_prerequisite_event_validate_insert":       "b8330eeef8e4e0b77d3364f14dcc266adbf940912a3399535d9cf92641016524",
	"TRIGGER:trg_chain_event_reorg_prerequisite_job_no_delete":               "f899349cd3da1b2b45c652cfe9b66836f3d77cb58db12c7d144b8afe05d951a3",
	"TRIGGER:trg_chain_event_reorg_prerequisite_job_validate_insert":         "e9e6bbbd58a311f7cb5392ac2737623f7b8470c7d1a5068945f5764898f8014a",
	"TRIGGER:trg_chain_event_reorg_prerequisite_job_validate_update":         "77a0e851aa3aa44e11db0f284d7b3bf50e5eeec75594562b1064298b1b9d6bc2",
	"TRIGGER:trg_chain_event_reorg_prerequisite_manifest_no_delete":          "7278945861854d939ff3bd2bd24355e391602e7d114621e1c2a5611d90991f6c",
	"TRIGGER:trg_chain_event_reorg_prerequisite_manifest_no_update":          "0f4ff5323993fa74902b839d43e1871bb98214c736abde01d9dbd9454269060e",
	"TRIGGER:trg_chain_event_reorg_prerequisite_manifest_validate_insert":    "11b7c249556ce8259e0fb61b45b0bb423860b28f04de7b902c823f2d67f12867",
	"TRIGGER:trg_chain_event_reorg_prerequisite_requirement_no_delete":       "c64122f196a9faa3159ff903409d80d91f8b074bd2a312e9128e67ac33828e69",
	"TRIGGER:trg_chain_event_reorg_prerequisite_requirement_no_update":       "39d27fd7181a760ad3d0d8d48e20e0b5e48fc888f662be0352bb3b868a63b7a0",
	"TRIGGER:trg_chain_event_reorg_prerequisite_requirement_validate_insert": "670b236ecd2547c6626e3418487dd0ea1601c6880cfdf59484c33fc1d5fde944",
	"TRIGGER:trg_chain_event_reorg_prerequisite_run_no_delete":               "7a9f0e63be5f5b263d53f262f9b3a04d2132d02c01cecea4d492ff82fc5be8b8",
	"TRIGGER:trg_chain_event_reorg_prerequisite_run_no_update":               "822a564fef3df841aea62bda5d30759894abd69d3e847cdf5fc9c050810f4df0",
	"TRIGGER:trg_chain_event_reorg_prerequisite_run_validate_insert":         "517741134838c19e412a572c62519f5b4e152ad011bd6bdd0e51c70cd5762f0e",
}

func requireApprovedChainEventReorgPrerequisiteDefinitions(database *gorm.DB) error {
	database = primarySchemaDatabase(database)
	var definitions []chainEventReorgRecoveryDefinition
	if err := database.Raw(chainEventReorgPrerequisiteDefinitionSQL).Scan(&definitions).Error; err != nil {
		return fmt.Errorf("read chain event reorg prerequisite definitions: %w", err)
	}
	if len(definitions) != len(approvedChainEventReorgPrerequisiteDefinitions) {
		return fmt.Errorf("chain event reorg prerequisite definition count is not approved: got=%d want=%d", len(definitions), len(approvedChainEventReorgPrerequisiteDefinitions))
	}
	seen := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		key := strings.ToUpper(definition.ObjectType) + ":" + definition.ObjectName
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("duplicate chain event reorg prerequisite definition: %s", key)
		}
		seen[key] = struct{}{}
		digest, err := chainEventReorgRecoveryDefinitionDigest(definition)
		if err != nil {
			return fmt.Errorf("invalid chain event reorg prerequisite definition %s: %w", key, err)
		}
		if approved, ok := approvedChainEventReorgPrerequisiteDefinitions[key]; !ok || approved != digest {
			return fmt.Errorf("chain event reorg prerequisite definition is not approved: %s", key)
		}
	}
	return nil
}
