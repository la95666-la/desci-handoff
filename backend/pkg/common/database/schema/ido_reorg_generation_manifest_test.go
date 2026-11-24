package schema

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

func TestApprovedChainEventReorgGenerationDefinitionsAreFrozen(t *testing.T) {
	if len(approvedChainEventReorgGenerationDefinitions) != 62 {
		t.Fatalf("unexpected approved generation definition count: %d", len(approvedChainEventReorgGenerationDefinitions))
	}
	for key, digest := range approvedChainEventReorgGenerationDefinitions {
		if !strings.HasPrefix(key, "CHECK:") && !strings.HasPrefix(key, "TRIGGER:") {
			t.Fatalf("invalid generation definition key %q", key)
		}
		if len(digest) != 64 {
			t.Fatalf("invalid generation definition digest %s=%q", key, digest)
		}
	}
	for _, key := range []string{
		"CHECK:chk_reorg_submission_uint256_raw",
		"CHECK:chk_reorg_candidate_project_uint256_raw",
		"CHECK:chk_reorg_candidate_wallet_uint256_raw",
	} {
		if _, ok := approvedChainEventReorgGenerationDefinitions[key]; !ok {
			t.Fatalf("missing Fair Launch generation definition %q", key)
		}
	}
}

func TestExportChainEventReorgGenerationDefinitionsFromMySQL(t *testing.T) {
	database := openIDOVerificationMySQL(t)
	var definitions []chainEventReorgRecoveryDefinition
	if err := database.Raw(chainEventReorgGenerationDefinitionSQL).Scan(&definitions).Error; err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 62 {
		t.Fatalf("unexpected generation definition count: %d", len(definitions))
	}
	lines := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		digest, err := chainEventReorgRecoveryDefinitionDigest(definition)
		if err != nil {
			t.Fatalf("digest %s: %v", definition.ObjectName, err)
		}
		lines = append(lines, fmt.Sprintf("\t%q: %q,", strings.ToUpper(definition.ObjectType)+":"+definition.ObjectName, digest))
	}
	sort.Strings(lines)
	for _, line := range lines {
		t.Log(line)
	}
}
