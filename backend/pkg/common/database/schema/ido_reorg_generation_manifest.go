package schema

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const chainEventReorgGenerationDefinitionSQL = `
SELECT 'CHECK' object_type,tc.CONSTRAINT_NAME object_name,tc.TABLE_NAME object_table,
tc.ENFORCED enforced,'' action_timing,'' event_manipulation,'' action_orientation,cc.CHECK_CLAUSE definition
FROM information_schema.TABLE_CONSTRAINTS tc
JOIN information_schema.CHECK_CONSTRAINTS cc
  ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK'
AND (LOWER(tc.TABLE_NAME) LIKE 'chain_event_reorg_generation%' OR LOWER(tc.TABLE_NAME) LIKE 'chain_event_reorg_candidate%')
AND tc.TABLE_NAME<>'chain_event_reorg_generation'
UNION ALL
SELECT 'TRIGGER',TRIGGER_NAME,EVENT_OBJECT_TABLE,'',ACTION_TIMING,EVENT_MANIPULATION,ACTION_ORIENTATION,ACTION_STATEMENT
FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
AND (LOWER(EVENT_OBJECT_TABLE) LIKE 'chain_event_reorg_generation%' OR LOWER(EVENT_OBJECT_TABLE) LIKE 'chain_event_reorg_candidate%')
AND NOT (EVENT_OBJECT_TABLE='chain_event_reorg_generation'
  AND TRIGGER_NAME='trg_chain_event_reorg_generation_validate_insert')`

// Generated from a fresh disposable MySQL 8 schema only after 08_28 is frozen.
var approvedChainEventReorgGenerationDefinitions = map[string]string{
	"CHECK:chk_chain_event_reorg_candidate_delta_fields":                   "e194c1a2fbe6258a1a7cb4de3957b357a4a9592c10a692101ed79aa61c17a231",
	"CHECK:chk_chain_event_reorg_candidate_entitlement_fields":             "d91fbd1dde41b84587cffa9908b2c544b736ba49551411bed9037cbab9cc4575",
	"CHECK:chk_chain_event_reorg_candidate_ledger_fields":                  "efc2a6051a3605e58a1ddf20a5a28942a6dd99fcf1dcddb0158501968b110716",
	"CHECK:chk_chain_event_reorg_candidate_project_fields":                 "c8192417ff20f361fa6c03c61b361cf1d79a5746bcc08d73156399feff7e8a2b",
	"CHECK:chk_reorg_candidate_project_uint256_raw":                        "2e47f07892db440c12eafb48788b40108fa3a2d9dea8d1a232636a6df6e4b747",
	"CHECK:chk_chain_event_reorg_candidate_v1_fields":                      "ae56a0d60b900e4549660a03f2f7d8a4b0f1ddc847dd5096f233e813a1266865",
	"CHECK:chk_chain_event_reorg_candidate_wallet_fields":                  "5100adbd8afa65411f1b9ec685b9db92f27c1de8a0703663282f5bac99fcdcc3",
	"CHECK:chk_reorg_candidate_wallet_uint256_raw":                         "460b6ac957446003e1ee5740d83d5003b756f2743a9bdfecc21f9105cffdb885",
	"CHECK:chk_chain_event_reorg_generation_checkpoint_fields":             "ddf45cc906f62ebf3a107fb5e0967fe37aaf138183e183c3fa0a2d9929bb7940",
	"CHECK:chk_chain_event_reorg_generation_event_fields":                  "a9a9780c11d0c2bc0cd4b0ad0f4955453a545edb4de088b7067023904e4caa46",
	"CHECK:chk_chain_event_reorg_generation_job_checkpoint":                "9559b3ce79dc0a42d7f51820f28dd322b294015b040317bd00867bc380ab40b0",
	"CHECK:chk_chain_event_reorg_generation_job_state":                     "c3539ff6531e37cf9d63b47094e782c098eb909405995fbb03474a8f8093b147",
	"CHECK:chk_chain_event_reorg_generation_manifest_fields":               "6e2b46e4b59bd3fb7d317d4879f834f066d60a42569b81ddd52bbeab2e97973f",
	"CHECK:chk_chain_event_reorg_generation_nonce_fields":                  "1872c4e44cc92eb2f53cd0fe49a696e9f8bd27062aea86caf778744f42e6213c",
	"CHECK:chk_chain_event_reorg_generation_requirement_fields":            "e778af52ed1560b2a0ebab45125023301870ca6074b38fede16b1f844bad9b5a",
	"CHECK:chk_chain_event_reorg_generation_sender_fields":                 "03747d2c0010dd3f53364dd4a75bfead59907a6e34dbc7c93165876163bd6274",
	"CHECK:chk_chain_event_reorg_generation_submission_fields":             "9280fb810c8242ff6904ae70154cffa2f62f5db7f06b093315b84d912895561d",
	"CHECK:chk_reorg_submission_uint256_raw":                               "119f5c6432a4e9fd0e4d445e13986498e901329227e5669f45d68e0679ace619",
	"TRIGGER:trg_chain_event_reorg_candidate_delta_no_delete":              "64ac639eee725884f28a286a8c3ca3e38a734e682dfc54deba84c29e4d33e688",
	"TRIGGER:trg_chain_event_reorg_candidate_delta_no_update":              "917b10369d31d7db92034220bc8f9726e955cb65da1ed69fc1df73d7ec6cc3f7",
	"TRIGGER:trg_chain_event_reorg_candidate_delta_validate_insert":        "78fbc771d7346c7a7a51ff098cc8cc6734754fbec764ad1ffadc524737292eda",
	"TRIGGER:trg_chain_event_reorg_candidate_entitlement_no_delete":        "2066594818327b07ee05f8b178030a8846bc7ae398bfc20fb5ee8650cd22ccff",
	"TRIGGER:trg_chain_event_reorg_candidate_entitlement_no_update":        "95e640a70967ab52f1363d618bba2e2c63eae73a4fde5bb5d6d8e604a1d7187e",
	"TRIGGER:trg_chain_event_reorg_candidate_entitlement_validate_insert":  "441731ea1dd2b33b41e5f07190338d8d0846faa16534cb43df79561e14ce69fb",
	"TRIGGER:trg_chain_event_reorg_candidate_ledger_no_delete":             "26964c0ec1c603a01613e9d1bad02705dd6fb2d828d54d94d7cb1a9f99336b08",
	"TRIGGER:trg_chain_event_reorg_candidate_ledger_no_update":             "ab82ae2bf7d269f2b4403770ca6a01cd8efaedfde51fc762a7eb06abf99ff463",
	"TRIGGER:trg_chain_event_reorg_candidate_ledger_validate_insert":       "e51639bd1123689faeea297bbaf84ff43a3962ab42acc075b9dccee424ffd60a",
	"TRIGGER:trg_chain_event_reorg_candidate_project_no_delete":            "52d1c3e5cb717d47c0a025ae923dacf21a6ede91b9f1e3a3c6f8f3c56a340b16",
	"TRIGGER:trg_chain_event_reorg_candidate_project_no_update":            "fddfee8645c25c263ce01b6c72f88de4b48b78e92e9d27c2500e111876bce906",
	"TRIGGER:trg_chain_event_reorg_candidate_project_validate_insert":      "52c7a1eca312dae77d44358b163e6ef6d75efaced9b239ca995561ca253337b9",
	"TRIGGER:trg_chain_event_reorg_candidate_v1_no_delete":                 "e5acca7faf476f359bd5277d2bf2b81e5ca9e2e7ff8fba2c1d062a9acb223794",
	"TRIGGER:trg_chain_event_reorg_candidate_v1_no_update":                 "ab3f51700243d2c95df55c32180b6a06aaab45f2d98f0d0efff87a149c3959a6",
	"TRIGGER:trg_chain_event_reorg_candidate_v1_validate_insert":           "3e577dfea788b34a307c9c22e6e40f99899c863b8955b2bd2510160da5f26453",
	"TRIGGER:trg_chain_event_reorg_candidate_wallet_no_delete":             "347f3e8e8eec4846a50c59192a8a832ac5cd041a0a60e238682188786cf529ec",
	"TRIGGER:trg_chain_event_reorg_candidate_wallet_no_update":             "e29f669d3d04486ace47df6dbed2a3a067ea82e6098dac1cdc84fbee14f4dc30",
	"TRIGGER:trg_chain_event_reorg_candidate_wallet_validate_insert":       "30fc69196d260f429181737f1e1fd30224e7547b750d512fcac76be2df03c845",
	"TRIGGER:trg_chain_event_reorg_generation_checkpoint_no_delete":        "60992f70717508f0e14364cc04323b6996a22636cfe32e6f639faeb06cd4da16",
	"TRIGGER:trg_chain_event_reorg_generation_checkpoint_no_update":        "c7b7f88d00f690be558c707032f9aac58585a41fa6ad26c8e9213857f6f49252",
	"TRIGGER:trg_chain_event_reorg_generation_checkpoint_validate_insert":  "01c105e59241ea35099583ab15577a20308aaa0dc5f6ba935f439a4aefbab134",
	"TRIGGER:trg_chain_event_reorg_generation_event_no_delete":             "2080d3da22330247b859fa5d32f21780cb1087b918166fa9ee665466d5953aa5",
	"TRIGGER:trg_chain_event_reorg_generation_event_no_update":             "e7f68dd01463ecfedad65b3f0b5b7822db350cb7cb479ae1f1e699abbce1c4e3",
	"TRIGGER:trg_chain_event_reorg_generation_event_validate_insert":       "d4344662f353978bd0fa85c049df126162067b0693305f53da6aa313846c098a",
	"TRIGGER:trg_chain_event_reorg_generation_job_no_delete":               "806bf1301ca20bd40aeb8411426f4bbbd4070155a06d182f23f7140736831c11",
	"TRIGGER:trg_chain_event_reorg_generation_job_validate_insert":         "cb5f9dc0598e158ee99e65513c32c21bc4579549318d7fb55e394beda9fbcf94",
	"TRIGGER:trg_chain_event_reorg_generation_job_validate_update":         "c466c8dfd8d6922dc102453bdf68eee07554a912e391cf01d8957383875f9b41",
	"TRIGGER:trg_chain_event_reorg_generation_manifest_no_delete":          "a929f13e2dd545a65f44756ab8f0aed20b8405168b730abe86388ca0fdf77144",
	"TRIGGER:trg_chain_event_reorg_generation_manifest_no_update":          "2ad1967763177359ca016c85f954490fcbbe5876585f2687bbab5e5162837739",
	"TRIGGER:trg_chain_event_reorg_generation_manifest_validate_insert":    "b691bb01a38ea9b3a94bce52c54b213c37356a7ef5adfba60bebd4ba447a0297",
	"TRIGGER:trg_chain_event_reorg_generation_no_delete":                   "c98759b7a191adda553390a1c3a63da2e73d804b391831957cb683d1a3b1cf48",
	"TRIGGER:trg_chain_event_reorg_generation_no_update":                   "7297cee8a53cc24bf6ca676e8c7537122a14d57a6b7cb83207058ac6b7611bed",
	"TRIGGER:trg_chain_event_reorg_generation_nonce_no_delete":             "1fd6cf8e7f88491392b33553e484688918247d8a213d0b42207dda179fc9efe0",
	"TRIGGER:trg_chain_event_reorg_generation_nonce_no_update":             "a5c1b9e86e2e8fbc0b1c5122f26c3cba9083befa1577d147a33ddbc430103741",
	"TRIGGER:trg_chain_event_reorg_generation_nonce_validate_insert":       "8f6ad808e3f9970703af6cf6edcb528844ddfca1abeb1ba7671dd73e67e4442d",
	"TRIGGER:trg_chain_event_reorg_generation_requirement_no_delete":       "088896fa1c11c5f47dcf5e28de69588c6bda90f265ca1e82f43a69d51d29db57",
	"TRIGGER:trg_chain_event_reorg_generation_requirement_no_update":       "0f37710ea6cca30b43cfd4339d54b8a5623551ae92210dd6ed01dc37d146fc96",
	"TRIGGER:trg_chain_event_reorg_generation_requirement_validate_insert": "f235bda6c439c947f3428057cc4c8263d5f22f4e1ed918a8d5fd6bc12c28a4c0",
	"TRIGGER:trg_chain_event_reorg_generation_sender_no_delete":            "15c1e4b835f46b9b14b3d096a6986464430409c9154ae1501742516b4e0761d2",
	"TRIGGER:trg_chain_event_reorg_generation_sender_no_update":            "96c49ca5bc73f0126e1ef181cc3e1f9a02165dbd8d6c5a2766d399e8f70fcc90",
	"TRIGGER:trg_chain_event_reorg_generation_sender_validate_insert":      "0dc74e8010927d9cd467657c395913db6050435b25ee7b4d769eef6b9c704b21",
	"TRIGGER:trg_chain_event_reorg_generation_submission_no_delete":        "2291efccc5ee27c3f75d69bbd8bd638a9caaa1a23ad304626e1543d512e704f8",
	"TRIGGER:trg_chain_event_reorg_generation_submission_no_update":        "1dc09da2f3b7aa8ba3bbf791e367622c0d875ddec18540b04464e3ec6ac909fc",
	"TRIGGER:trg_chain_event_reorg_generation_submission_validate_insert":  "dd292be762037df652bb13af2a17e39344c9b6817ea2c460b00a875786998711",
}

func requireApprovedChainEventReorgGenerationDefinitions(database *gorm.DB) error {
	database = primarySchemaDatabase(database)
	var definitions []chainEventReorgRecoveryDefinition
	if err := database.Raw(chainEventReorgGenerationDefinitionSQL).Scan(&definitions).Error; err != nil {
		return fmt.Errorf("read chain event reorg generation definitions: %w", err)
	}
	if len(definitions) != len(approvedChainEventReorgGenerationDefinitions) {
		return fmt.Errorf("chain event reorg generation definition count is not approved: got=%d want=%d", len(definitions), len(approvedChainEventReorgGenerationDefinitions))
	}
	seen := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		key := strings.ToUpper(definition.ObjectType) + ":" + definition.ObjectName
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("duplicate chain event reorg generation definition: %s", key)
		}
		seen[key] = struct{}{}
		digest, err := chainEventReorgRecoveryDefinitionDigest(definition)
		if err != nil {
			return fmt.Errorf("invalid chain event reorg generation definition %s: %w", key, err)
		}
		if approved, ok := approvedChainEventReorgGenerationDefinitions[key]; !ok || approved != digest {
			return fmt.Errorf("chain event reorg generation definition is not approved: %s", key)
		}
	}
	return nil
}
