package schema

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const chainEventReorgAssessmentDefinitionSQL = `
SELECT 'CHECK' object_type,tc.CONSTRAINT_NAME object_name,tc.TABLE_NAME object_table,
tc.ENFORCED enforced,'' action_timing,'' event_manipulation,'' action_orientation,cc.CHECK_CLAUSE definition
FROM information_schema.TABLE_CONSTRAINTS tc
JOIN information_schema.CHECK_CONSTRAINTS cc
  ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK'
AND LOWER(tc.TABLE_NAME) IN ('chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker',
'chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest')
UNION ALL
SELECT 'TRIGGER',TRIGGER_NAME,EVENT_OBJECT_TABLE,'',ACTION_TIMING,EVENT_MANIPULATION,ACTION_ORIENTATION,ACTION_STATEMENT
FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND LOWER(EVENT_OBJECT_TABLE) IN (
'chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker',
'chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest')`

var approvedChainEventReorgAssessmentDefinitions = map[string]string{
	"CHECK:chk_chain_event_reorg_assessment_blocker_fields":                "6b534a9ce2e7daf48b6c3c13fa02c2464ed465ae495559797c01156908bf699c",
	"CHECK:chk_chain_event_reorg_assessment_disposition_identity":          "2d9003c6a7b3efe6639fa7b61caa975d6fa973d30474fcf84405d0fa94784396",
	"CHECK:chk_chain_event_reorg_assessment_disposition_semantics":         "3e88afdf09b966346e6df1ccb6fcf144f9678de295faac43156fde5726ea2519",
	"CHECK:chk_chain_event_reorg_assessment_event_fields":                  "af122d4221047dfb14df1d917e714c70b33a064bd9e1f8c9c55d60f27919fade",
	"CHECK:chk_chain_event_reorg_assessment_job_checkpoint":                "62158cb0bb87c07a9972c656cee8b8dc6c8c45afb13b4ad517b15a4fd848f495",
	"CHECK:chk_chain_event_reorg_assessment_job_state":                     "4375f2a84a219d92f236e2ae47d5a6e7912f78d5b83038be941a79490a1d3569",
	"CHECK:chk_chain_event_reorg_assessment_manifest_fields":               "1d088aca46104438ae4b3d7a024ecccda15a75323f736cf6a284dd5dc5237f81",
	"CHECK:chk_chain_event_reorg_assessment_run_identity":                  "bdd57f6d4d0f64ca8b9a635fb49e0add53874f6df765281b06a3d8e477706f93",
	"CHECK:chk_chain_event_reorg_assessment_run_range":                     "72df0eacdde8082f91c603d553bda17c150bf630585f249d298c9c57163f6a9e",
	"CHECK:chk_chain_event_reorg_assessment_run_source":                    "d74c4318dffb21fa41c3c9a8c9c0c2fea229738f8ff125cb520fd8981814e2e7",
	"TRIGGER:trg_chain_event_reorg_assessment_blocker_no_delete":           "76f3227c5948cdbadc67d7eb2bc7242860f82f6392b92b6b3fb12ad0bf5bbdf2",
	"TRIGGER:trg_chain_event_reorg_assessment_blocker_no_update":           "1917695b82bf0abadf2cf5aba4c9dde469b5a265e5e405b1eb54d0256fdb033f",
	"TRIGGER:trg_chain_event_reorg_assessment_blocker_validate_insert":     "7c7d2dff80f998290d44df0633b344beb69ac432b22a25f2d1e1991fd8d67fda",
	"TRIGGER:trg_chain_event_reorg_assessment_disposition_no_delete":       "a475cf5f102e135c203d5a5b3815fbece2875a1de226539c4875b2ec5b1b0e03",
	"TRIGGER:trg_chain_event_reorg_assessment_disposition_no_update":       "a9684256c823d29db81c6998a37e0c141ca34f6784247535c792777cc1fd2909",
	"TRIGGER:trg_chain_event_reorg_assessment_disposition_validate_insert": "672347f54df15c28c06e482d7471826e41855f5ff0b3c56dbe0ba3e36016b54f",
	"TRIGGER:trg_chain_event_reorg_assessment_event_no_delete":             "8f1edfe83e13d12f6694a65aaeb9f2ac8e9828e219c7df70989e2de9166c4192",
	"TRIGGER:trg_chain_event_reorg_assessment_event_no_update":             "b4b67b51c27085ae6a03695117d0dce2623243dfa6da8a8bb26e0addfdb7f67f",
	"TRIGGER:trg_chain_event_reorg_assessment_event_validate_insert":       "70ce1c3b42fcef738dd2f10608d0dca68dd2675bd1b79f769f34ff34adf11bd2",
	"TRIGGER:trg_chain_event_reorg_assessment_job_no_delete":               "82b8bb7e01d91890a65ddf2ab8b1e0e71a7a9d23cac2a5b81d6d94915c7ec22a",
	"TRIGGER:trg_chain_event_reorg_assessment_job_validate_insert":         "e6b6ccc1141d6effd25a814d9408ad615f0a5d4de3c98074dbc90a2b99fa8fb2",
	"TRIGGER:trg_chain_event_reorg_assessment_job_validate_update":         "3872a002f5f3ab70815a03f20a20b8f881b96156c92840a66f055779a6e51a3e",
	"TRIGGER:trg_chain_event_reorg_assessment_manifest_no_delete":          "b52bb3b62551ca17881b5a13222211621747b6671759811ddad19c6035875827",
	"TRIGGER:trg_chain_event_reorg_assessment_manifest_no_update":          "aa7162b985204caf4d998d51331d2a6c8ecb83b346a4d68db05cb32547dd26ea",
	"TRIGGER:trg_chain_event_reorg_assessment_manifest_validate_insert":    "061968711368e92652aa6685aba9d632246efbe31f5e1e961f6b7e8519128f7d",
	"TRIGGER:trg_chain_event_reorg_assessment_run_no_delete":               "5bf46119fec05172a3b5abee4a3b23d0e11896b4aebbd4f39f9d1d585172a570",
	"TRIGGER:trg_chain_event_reorg_assessment_run_no_update":               "646af33ae8010941e39c717b28168b71f585171d792b7a50eb98e8c67033bb77",
	"TRIGGER:trg_chain_event_reorg_assessment_run_validate_insert":         "fbd2cf84119bacdc5d9d942854007f1e7941f84f8f9774eec781b6776b7282a6",
}

func requireApprovedChainEventReorgAssessmentDefinitions(database *gorm.DB) error {
	database = primarySchemaDatabase(database)
	var definitions []chainEventReorgRecoveryDefinition
	if err := database.Raw(chainEventReorgAssessmentDefinitionSQL).Scan(&definitions).Error; err != nil {
		return fmt.Errorf("read chain event reorg assessment definitions: %w", err)
	}
	if len(definitions) != len(approvedChainEventReorgAssessmentDefinitions) {
		return fmt.Errorf("chain event reorg assessment definition count is not approved: got=%d want=%d", len(definitions), len(approvedChainEventReorgAssessmentDefinitions))
	}
	seen := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		key := strings.ToUpper(definition.ObjectType) + ":" + definition.ObjectName
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("duplicate chain event reorg assessment definition: %s", key)
		}
		seen[key] = struct{}{}
		digest, err := chainEventReorgRecoveryDefinitionDigest(definition)
		if err != nil {
			return fmt.Errorf("invalid chain event reorg assessment definition %s: %w", key, err)
		}
		if approved, ok := approvedChainEventReorgAssessmentDefinitions[key]; !ok || approved != digest {
			return fmt.Errorf("chain event reorg assessment definition is not approved: %s", key)
		}
	}
	return nil
}
