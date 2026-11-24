package schema

import "testing"

func TestApprovedChainEventReorgMaterializationDefinitionsAreFrozen(t *testing.T) {
	if len(approvedChainEventReorgMaterializationDefinitions) != 28 {
		t.Fatalf("unexpected approved materialization definition count: %d", len(approvedChainEventReorgMaterializationDefinitions))
	}
	checks, triggers := 0, 0
	for key, digest := range approvedChainEventReorgMaterializationDefinitions {
		if len(digest) != 64 {
			t.Fatalf("invalid approved materialization digest %q=%q", key, digest)
		}
		if len(key) > 6 && key[:6] == "CHECK:" {
			checks++
		} else if len(key) > 8 && key[:8] == "TRIGGER:" {
			triggers++
		}
	}
	if checks != 8 || triggers != 20 {
		t.Fatalf("unexpected materialization manifest composition: checks=%d triggers=%d", checks, triggers)
	}
}
