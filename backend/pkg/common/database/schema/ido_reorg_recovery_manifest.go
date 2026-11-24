package schema

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"unicode"

	"gorm.io/gorm"
)

const chainEventReorgRecoveryDefinitionSQL = `
SELECT 'CHECK' object_type,tc.CONSTRAINT_NAME object_name,tc.TABLE_NAME object_table,
tc.ENFORCED enforced,'' action_timing,'' event_manipulation,'' action_orientation,cc.CHECK_CLAUSE definition
FROM information_schema.TABLE_CONSTRAINTS tc
JOIN information_schema.CHECK_CONSTRAINTS cc
  ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK'
AND LOWER(tc.TABLE_NAME) IN ('chain_event_reorg_recovery_run','chain_event_reorg_recovery_job',
'chain_event_reorg_recovery_event','chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch',
'chain_event_reorg_recovery_manifest','chain_event_reorg_recovery_definition')
AND tc.TABLE_NAME<>'chain_event_reorg_recovery_run'
UNION ALL
SELECT 'TRIGGER',TRIGGER_NAME,EVENT_OBJECT_TABLE,'',ACTION_TIMING,EVENT_MANIPULATION,ACTION_ORIENTATION,ACTION_STATEMENT
FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND LOWER(EVENT_OBJECT_TABLE) IN (
'chain_event_reorg_recovery_run','chain_event_reorg_recovery_job','chain_event_reorg_recovery_event',
'chain_event_reorg_recovery_log','chain_event_reorg_recovery_batch','chain_event_reorg_recovery_manifest',
'chain_event_reorg_recovery_definition')`

type chainEventReorgRecoveryDefinition struct {
	ObjectType        string `gorm:"column:object_type"`
	ObjectName        string `gorm:"column:object_name"`
	ObjectTable       string `gorm:"column:object_table"`
	Enforced          string `gorm:"column:enforced"`
	ActionTiming      string `gorm:"column:action_timing"`
	EventManipulation string `gorm:"column:event_manipulation"`
	ActionOrientation string `gorm:"column:action_orientation"`
	Definition        string `gorm:"column:definition"`
}

// This manifest is an external trust anchor compiled into core and job. The
// database cannot authorize changed safety definitions by rewriting its own
// attestation rows. Values are derived from the reviewed 08_23 migration with
// SQL token normalization; changing any semantic token requires a code review.
var approvedChainEventReorgRecoveryDefinitions = map[string]string{
	"CHECK:chk_chain_event_reorg_recovery_batch_digests":                "cabb9d1470b613402b1658e1c234b3e64f58075b3b403d0507fbbc4860a7a2be",
	"CHECK:chk_chain_event_reorg_recovery_batch_identity":               "af79212193f4eb3d4d9ae37b87a3002c4938db10fb80dae31dc7e4af8ffb1901",
	"CHECK:chk_chain_event_reorg_recovery_definition_digests":           "72c5b77979963ed29a7ce3b09b0067ca48f2bdca316f23cdfa2a973c905c279d",
	"CHECK:chk_chain_event_reorg_recovery_definition_identity":          "5d1be2fa3f894e2ac91fdc041b9669cab2db962f9ecb793240f000ffe40c816a",
	"CHECK:chk_chain_event_reorg_recovery_event_fields":                 "a38a5dd2f837d661bc3f3174d1f8409816f9e9d4a1cd7d678ecb5e85040115af",
	"CHECK:chk_chain_event_reorg_recovery_job_digests":                  "6cd361e49d380092f73a69ff0b303b7e6ee39871a74456b9f87b4d48364a144c",
	"CHECK:chk_chain_event_reorg_recovery_job_state":                    "6de17c32987009c74e9c788b77ac4402fec7da33c75bf95250be1a641acd13d5",
	"CHECK:chk_chain_event_reorg_recovery_log_digests":                  "34713caa387ddce296c003fd03903888cafac8f2375ffab4f80950efdb65f280",
	"CHECK:chk_chain_event_reorg_recovery_log_identity":                 "3774504583c35dfc839bc0a0de579f24b0917c519733f8643117f1522de1909a",
	"CHECK:chk_chain_event_reorg_recovery_manifest_fields":              "7f6d8119de1e84a1e14a4122b66d2e3efa7c34f8c326ff4df40012c28752cb9e",
	"TRIGGER:trg_chain_event_reorg_recovery_batch_no_delete":            "fdb4441fc45f31ee60e915aa0df10aebd0649a54a8de3a6ea342c309c000b017",
	"TRIGGER:trg_chain_event_reorg_recovery_batch_no_update":            "33993409eaef5dcc1d9cd948b955789d30e8521d825010df412afcab50793a78",
	"TRIGGER:trg_chain_event_reorg_recovery_batch_validate_insert":      "fea4d5a0668dd8aae821f7bca4c7cca90cd952045e14e3cabb0cfe07ae55f471",
	"TRIGGER:trg_chain_event_reorg_recovery_definition_no_delete":       "47b909757dce4cfc911d50d090de707b3077dc189edd294429e27728845d3c68",
	"TRIGGER:trg_chain_event_reorg_recovery_definition_no_update":       "14f50596ed0de4c5be0a54b92ee137d1d234cc6af8733f8c611b600162dd5ae4",
	"TRIGGER:trg_chain_event_reorg_recovery_definition_validate_insert": "08fc54a085b6913dce1682a09fac66b8d1bb85c2bde07b68a1eb7293d5ec165b",
	"TRIGGER:trg_chain_event_reorg_recovery_event_no_delete":            "0d7a64cb52e13366fbddc10052959f7ed3ff4829eee19f04cb8b8e391e8bce88",
	"TRIGGER:trg_chain_event_reorg_recovery_event_no_update":            "eab35276dca1c48520100d953ad6778f0cbb44ccd5724f1c0977a53a40ea9e44",
	"TRIGGER:trg_chain_event_reorg_recovery_event_validate_insert":      "bd1ae294be75ad53cf6003a77631004bfa3cd9d44a9a5b2600bab02d6c5840eb",
	"TRIGGER:trg_chain_event_reorg_recovery_job_no_delete":              "b8de125cb7975b7b052e70f6d257fd0910b313b5ca3c39cb4843ef3a8cd9a9a1",
	"TRIGGER:trg_chain_event_reorg_recovery_job_validate_insert":        "1a34ee73ac74c2f4ef44572e69216b6ae58db430f3a3c08556aaa22af62cd005",
	"TRIGGER:trg_chain_event_reorg_recovery_job_validate_update":        "45121658626ed023eac3481c9dd7fb0237b42e6df791efbe7d075ad75bbb72da",
	"TRIGGER:trg_chain_event_reorg_recovery_log_no_delete":              "5744f3b4e890e137376b63fa5cdf6128b4e9c6b9886c695b75d90c4acff21aad",
	"TRIGGER:trg_chain_event_reorg_recovery_log_no_update":              "d04d5418e9dee1dce9d6e6a0fc7279dd8df8b61487897880acd0758a27544d33",
	"TRIGGER:trg_chain_event_reorg_recovery_log_validate_insert":        "5679900f6a6ea3fb1cd451706318a7503fc75a3fa514040c1ebaa8c8a827b4f4",
	"TRIGGER:trg_chain_event_reorg_recovery_manifest_no_delete":         "8bb717aca564346525d839170fa2fe3ae72ebc6b1ab9205e5a5af74cdf987883",
	"TRIGGER:trg_chain_event_reorg_recovery_manifest_no_update":         "3a717a55fda91dcd80eb349c7fe1d8e8b930c3ecfef6f768e17da4e40629786a",
	"TRIGGER:trg_chain_event_reorg_recovery_manifest_validate_insert":   "2d4d5a4170d9fae93f700b98107862db19c9669aa756b3eea68de4f7b14fac4c",
	"TRIGGER:trg_chain_event_reorg_recovery_run_no_delete":              "faf743a59a053188648c6c5fde66fc033371e7ca9a89aa840b6de2cf92dbc49e",
	"TRIGGER:trg_chain_event_reorg_recovery_run_no_update":              "a91a1246ffa899462dd898499296bc135e7dad98fa4adb3c3f0fdeb6e8ac1022",
	"TRIGGER:trg_chain_event_reorg_recovery_run_validate_insert":        "744a3c856ed3bdd97f0338579cdbea9602f171c33142f58e4c2f5e659fe8ba3a",
}

func requireApprovedChainEventReorgRecoveryDefinitions(database *gorm.DB) error {
	database = primarySchemaDatabase(database)
	var definitions []chainEventReorgRecoveryDefinition
	if err := database.Raw(chainEventReorgRecoveryDefinitionSQL).Scan(&definitions).Error; err != nil {
		return fmt.Errorf("read chain event reorg recovery definitions: %w", err)
	}
	if len(definitions) != len(approvedChainEventReorgRecoveryDefinitions) {
		return fmt.Errorf("chain event reorg recovery definition count is not approved: got=%d want=%d", len(definitions), len(approvedChainEventReorgRecoveryDefinitions))
	}
	seen := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		key := strings.ToUpper(definition.ObjectType) + ":" + definition.ObjectName
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("duplicate chain event reorg recovery definition: %s", key)
		}
		seen[key] = struct{}{}
		digest, err := chainEventReorgRecoveryDefinitionDigest(definition)
		if err != nil {
			return fmt.Errorf("invalid chain event reorg recovery definition %s: %w", key, err)
		}
		if approved, ok := approvedChainEventReorgRecoveryDefinitions[key]; !ok || approved != digest {
			return fmt.Errorf("chain event reorg recovery definition is not approved: %s", key)
		}
	}
	return nil
}

func chainEventReorgRecoveryDefinitionDigest(definition chainEventReorgRecoveryDefinition) (string, error) {
	canonicalDefinition, err := canonicalChainEventReorgRecoverySQL(definition.Definition)
	if err != nil {
		return "", err
	}
	var parts []string
	switch strings.ToUpper(definition.ObjectType) {
	case "CHECK":
		if !strings.EqualFold(definition.Enforced, "YES") {
			return "", fmt.Errorf("CHECK is not enforced")
		}
		parts = []string{"joincare.mysql-check-definition.v2", definition.ObjectTable, "YES", canonicalDefinition}
	case "TRIGGER":
		if !strings.EqualFold(definition.ActionOrientation, "ROW") {
			return "", fmt.Errorf("trigger is not row-oriented")
		}
		parts = []string{"joincare.mysql-trigger-definition.v2", definition.ObjectTable, strings.ToUpper(definition.ActionTiming), strings.ToUpper(definition.EventManipulation), "ROW", canonicalDefinition}
	default:
		return "", fmt.Errorf("unsupported object type %q", definition.ObjectType)
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return fmt.Sprintf("%x", sum), nil
}

func canonicalChainEventReorgRecoverySQL(sql string) (string, error) {
	for _, identifier := range sqlIdentifiers(sql) {
		lower := strings.ToLower(identifier)
		if strings.HasPrefix(lower, "chain_event_") && identifier != lower {
			return "", fmt.Errorf("case-sensitive chain-event identifier is not canonical: %q", identifier)
		}
	}
	return canonicalSQLTokens(sql), nil
}

func sqlIdentifiers(sql string) []string {
	var identifiers []string
	for i := 0; i < len(sql); {
		if unicode.IsSpace(rune(sql[i])) {
			i++
			continue
		}
		if sql[i] == '\'' || sql[i] == '"' {
			quote := sql[i]
			i++
			for i < len(sql) {
				if sql[i] == quote {
					if i+1 < len(sql) && sql[i+1] == quote {
						i += 2
						continue
					}
					i++
					break
				}
				if sql[i] == '\\' && i+1 < len(sql) {
					i += 2
					continue
				}
				i++
			}
			continue
		}
		if sql[i] == '`' {
			var identifier strings.Builder
			for i++; i < len(sql); {
				if sql[i] == '`' {
					if i+1 < len(sql) && sql[i+1] == '`' {
						identifier.WriteByte('`')
						i += 2
						continue
					}
					i++
					break
				}
				identifier.WriteByte(sql[i])
				i++
			}
			identifiers = append(identifiers, identifier.String())
			continue
		}
		if isSQLWordByte(sql[i]) {
			start := i
			for i < len(sql) && isSQLWordByte(sql[i]) {
				i++
			}
			identifiers = append(identifiers, sql[start:i])
			continue
		}
		i++
	}
	return identifiers
}

func canonicalSQLTokens(sql string) string {
	var tokens []string
	for i := 0; i < len(sql); {
		if unicode.IsSpace(rune(sql[i])) {
			i++
			continue
		}
		if sql[i] == '`' {
			var identifier strings.Builder
			for i++; i < len(sql); {
				if sql[i] == '`' {
					if i+1 < len(sql) && sql[i+1] == '`' {
						identifier.WriteByte('`')
						i += 2
						continue
					}
					i++
					break
				}
				identifier.WriteByte(sql[i])
				i++
			}
			tokens = append(tokens, strings.ToLower(identifier.String()))
			continue
		}
		if sql[i] == '\'' || sql[i] == '"' {
			quote := sql[i]
			start := i
			i++
			for i < len(sql) {
				if sql[i] == quote {
					if i+1 < len(sql) && sql[i+1] == quote {
						i += 2
						continue
					}
					i++
					break
				}
				if sql[i] == '\\' && i+1 < len(sql) {
					i += 2
					continue
				}
				i++
			}
			tokens = append(tokens, sql[start:i])
			continue
		}
		if isSQLWordByte(sql[i]) {
			start := i
			for i < len(sql) && isSQLWordByte(sql[i]) {
				i++
			}
			tokens = append(tokens, strings.ToLower(sql[start:i]))
			continue
		}
		if i+1 < len(sql) && isTwoByteSQLOperator(sql[i:i+2]) {
			tokens = append(tokens, sql[i:i+2])
			i += 2
			continue
		}
		tokens = append(tokens, sql[i:i+1])
		i++
	}
	return strings.Join(tokens, "\n")
}

func isSQLWordByte(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == '_' || value == '$'
}

func isTwoByteSQLOperator(value string) bool {
	switch value {
	case "<>", ">=", "<=", "!=", ":=", "||", "&&", "<<", ">>":
		return true
	default:
		return false
	}
}
