package schema

import "testing"

func TestApprovedChainEventReorgAssessmentDefinitionsAreFrozen(t *testing.T) {
	if len(approvedChainEventReorgAssessmentDefinitions) != 28 {
		t.Fatalf("unexpected approved assessment definition count: %d", len(approvedChainEventReorgAssessmentDefinitions))
	}
	checks, triggers := 0, 0
	for key, digest := range approvedChainEventReorgAssessmentDefinitions {
		if len(digest) != 64 {
			t.Fatalf("invalid approved assessment digest %q=%q", key, digest)
		}
		if len(key) > 6 && key[:6] == "CHECK:" {
			checks++
		} else if len(key) > 8 && key[:8] == "TRIGGER:" {
			triggers++
		}
	}
	if checks != 10 || triggers != 18 {
		t.Fatalf("unexpected assessment manifest composition: checks=%d triggers=%d", checks, triggers)
	}
}
