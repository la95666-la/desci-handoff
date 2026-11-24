package schema

import "testing"

func TestApprovedChainEventReorgPrerequisiteDefinitionsAreFrozen(t *testing.T) {
	if len(approvedChainEventReorgPrerequisiteDefinitions) != 26 {
		t.Fatalf("unexpected approved prerequisite definition count: %d", len(approvedChainEventReorgPrerequisiteDefinitions))
	}
	checks, triggers := 0, 0
	for key, digest := range approvedChainEventReorgPrerequisiteDefinitions {
		if len(digest) != 64 {
			t.Fatalf("invalid approved prerequisite digest %q=%q", key, digest)
		}
		if len(key) > 6 && key[:6] == "CHECK:" {
			checks++
		} else if len(key) > 8 && key[:8] == "TRIGGER:" {
			triggers++
		}
	}
	if checks != 8 || triggers != 18 {
		t.Fatalf("unexpected prerequisite manifest composition: checks=%d triggers=%d", checks, triggers)
	}
}
