package main

import (
	"strings"
	"testing"
)

func TestRecoveryCommandRequiresExplicitExactOriginalHash(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"--execute"},
		{"--config-dir", "/nonexistent", "--execute"},
		{"--config-dir", "/nonexistent", "--confirm-hash", originalHash},
		{"--config-dir", "/nonexistent", "--execute", "--confirm-hash", "0x" + strings.Repeat("0", 64)},
		{"--config-dir", "/nonexistent", "--execute", "--confirm-hash", originalHash, "unexpected"},
	} {
		if err := run(args); err == nil {
			t.Fatalf("unexpectedly accepted invalid flags: %v", args)
		}
	}
}
