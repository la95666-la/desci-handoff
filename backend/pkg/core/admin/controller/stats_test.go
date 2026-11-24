package controller

import "testing"

func TestNormalizeAnonymousVisitorID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		valid bool
	}{
		{name: "rolling deployment without id", input: "", want: "", valid: true},
		{name: "uuid", input: " 9b2f239d-6f89-4ef1-8caa-15df70a482b7 ", want: "9b2f239d-6f89-4ef1-8caa-15df70a482b7", valid: true},
		{name: "fallback id", input: "anon_mdy4kny5_5p6a2l7c", want: "anon_mdy4kny5_5p6a2l7c", valid: true},
		{name: "too short", input: "visitor", want: "visitor", valid: false},
		{name: "unsafe characters", input: "visitor-id-with-spaces!", want: "visitor-id-with-spaces!", valid: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, valid := normalizeAnonymousVisitorID(test.input)
			if got != test.want || valid != test.valid {
				t.Fatalf("normalizeAnonymousVisitorID(%q) = (%q, %v), want (%q, %v)", test.input, got, valid, test.want, test.valid)
			}
		})
	}
}

func TestDailyAnonymousVisitorMember(t *testing.T) {
	visitorID := "9b2f239d-6f89-4ef1-8caa-15df70a482b7"
	today := dailyAnonymousVisitorMember("2026-07-27", visitorID)
	if len(today) != 64 {
		t.Fatalf("daily member length = %d, want 64", len(today))
	}
	if today != dailyAnonymousVisitorMember("2026-07-27", visitorID) {
		t.Fatal("same visitor and day must produce a stable member")
	}
	if today == dailyAnonymousVisitorMember("2026-07-28", visitorID) {
		t.Fatal("different days must not produce a linkable member")
	}
}
