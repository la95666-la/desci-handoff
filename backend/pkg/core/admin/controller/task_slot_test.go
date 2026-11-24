package controller

import (
	"testing"

	"joincare_server/pkg/common/entity"
)

func TestValidateTaskTargetSlot(t *testing.T) {
	tests := []struct {
		platform string
		key      string
		slot     uint8
		wantErr  bool
	}{
		{"twitter", "like", 1, false},
		{"twitter", "like", 2, false},
		{"twitter", "reply", 2, false},
		{"twitter", "retweet", 3, true},
		{"twitter", "bookmark", 2, true},
		{"twitter", "follow", 1, false},
		{"twitter", "follow", 2, true},
		{"telegram", "join", 2, true},
		{"twitter", "like", 0, false},
	}
	for _, test := range tests {
		err := validateTaskTargetSlot(test.platform, test.key, test.slot)
		if (err != nil) != test.wantErr {
			t.Fatalf("validateTaskTargetSlot(%q,%q,%d) err=%v wantErr=%v", test.platform, test.key, test.slot, err, test.wantErr)
		}
	}
}

func TestCrossSlotTwitterTargetReuseIsOperatorDecision(t *testing.T) {
	// No database uniqueness lookup: an old slot-1 post can be assigned to
	// slot 2. This does not bypass the per-task round reuse check on updates.
	task := entity.Task{Platform: "twitter", TaskKey: "like", URL: "https://x.com/joincare/status/12345", TargetSlot: 2}
	if err := rejectDuplicateTwitterTarget(nil, task, 0); err != nil {
		t.Fatalf("cross-slot target reuse rejected: %v", err)
	}
	task.URL = "https://x.com/joincare"
	if err := rejectDuplicateTwitterTarget(nil, task, 0); err == nil {
		t.Fatal("invalid X post URL must still be rejected")
	}
}
