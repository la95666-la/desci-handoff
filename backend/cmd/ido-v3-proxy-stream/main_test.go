package main

import "testing"

func TestStreamOperatorRejectsUnconfirmedMutationBeforeConfig(t *testing.T) {
	for _, args := range [][]string{
		{"--config-dir", "missing", "--action", "switch-stream"},
		{"--config-dir", "missing", "--action", "switch-stream", "--execute"},
		{"--config-dir", "missing", "--action", "recover", "--execute", "--expected-revision", "1"},
		{"--config-dir", "missing", "--execute"},
		{"--config-dir", "missing", "--action", "send", "--execute", "--expected-revision", "1"},
		{"--config-dir", "missing", "--action", "recover-project", "--execute", "--expected-revision", "1"},
		{"--config-dir", "missing", "--action", "check-project", "--execute", "--expected-revision", "1", "--project-id", "7"},
		{"--config-dir", "missing", "--action", "audit-project", "--execute", "--project-id", "7"},
		{"--config-dir", "missing", "--action", "backfill", "--expected-revision", "1"},
		{"--config-dir", "missing", "--action", "retire-tests-rebaseline", "--execute", "--expected-revision", "1"},
	} {
		if err := run(args); err == nil {
			t.Fatal("operator mutation admitted without exact intent", args)
		}
	}
}
