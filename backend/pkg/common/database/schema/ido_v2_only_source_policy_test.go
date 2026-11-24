package schema

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

func TestApprovedV2OnlySourcePolicyDefinitionsAreFrozen(t *testing.T) {
	if len(approvedV2OnlySourcePolicyDefinitions) != 10 {
		t.Fatalf("unexpected V2-only source policy definition count: %d", len(approvedV2OnlySourcePolicyDefinitions))
	}
	for key, digest := range approvedV2OnlySourcePolicyDefinitions {
		if key == "" || (digest != "" && len(digest) != 64) {
			t.Fatalf("invalid V2-only source policy definition %q=%q", key, digest)
		}
	}
}

func TestExportV2OnlySourcePolicyDefinitionsFromMySQL(t *testing.T) {
	database := openIDOVerificationMySQL(t)
	var definitions []chainEventReorgRecoveryDefinition
	if err := database.Raw(v2OnlySourcePolicyDefinitionSQL).Scan(&definitions).Error; err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 10 {
		t.Fatalf("unexpected V2-only source policy definition count: %d", len(definitions))
	}
	lines := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		digest, err := chainEventReorgRecoveryDefinitionDigest(definition)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, fmt.Sprintf("%s=%s", strings.ToUpper(definition.ObjectType)+":"+definition.ObjectName, digest))
	}
	sort.Strings(lines)
	for _, line := range lines {
		t.Log(line)
	}
}
