package main

import "testing"

func TestRetiredNonceToolRequiresExactOriginalAttempt(t *testing.T) {
	for _, args := range [][]string{{}, {"--execute"}, {"--config-dir", "missing", "--submission-id", "41"}, {"--config-dir", "missing", "--submission-id", "41", "--confirm-hash", "0xwrong", "--execute"}} {
		if run(args) == nil {
			t.Fatal("missing exact identity admitted")
		}
	}
}
