package controller

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestResolveRestoreTarget(t *testing.T) {
	// A recorded pre-archive status is restored verbatim and never rewritten to
	// a default such as pre_launch (no unconditional pre_launch).
	for _, status := range []string{"Setup", "Set", "pre_launch", "warmup"} {
		t.Run(status, func(t *testing.T) {
			target, err := resolveRestoreTarget(status)
			if err != nil || target != status {
				t.Fatalf("restore target must round-trip %q: target=%q err=%v", status, target, err)
			}
		})
	}
	// Leading/trailing whitespace is normalized.
	if target, err := resolveRestoreTarget("  pre_launch "); err != nil || target != "pre_launch" {
		t.Fatalf("whitespace must be trimmed: target=%q err=%v", target, err)
	}
}

func TestResolveRestoreTargetRejectsUnknown(t *testing.T) {
	for _, recorded := range []string{
		"",                       // legacy archive without a recorded status
		"archived",               // cannot restore to archived
		"Pending",                // on-chain lifecycle states are never pre-archive
		"Active",                 // on-chain lifecycle states are never pre-archive
		"Successful",             // on-chain lifecycle states are never pre-archive
		"pre_launch  extra",      // not in the allowlist
		"late_launch",            // invented status
		"ARCHIVED",               // case-sensitive allowlist
		"COVID",                  // arbitrary value from a tampered row
	} {
		t.Run(recorded, func(t *testing.T) {
			target, err := resolveRestoreTarget(recorded)
			if err == nil {
				t.Fatalf("recorded %q must be rejected, got target=%q", recorded, target)
			}
			if !errors.Is(err, errProjectRestoreUnknownState) {
				t.Fatalf("expected errProjectRestoreUnknownState, got %v", err)
			}
			if !strings.Contains(err.Error(), "refusing to guess") {
				t.Fatalf("error must explain the no-guessing policy: %v", err)
			}
		})
	}
}

func TestRestoreErrorContract(t *testing.T) {
	// Stable sentinel errors are the contract behind the HTTP error codes.
	if errProjectRestoreLifecycleLocked == nil || errProjectRestoreNotArchived == nil ||
		errProjectRestoreUnknownState == nil || errProjectRestoreImmutableFacts == nil || errProjectRestoreConflict == nil {
		t.Fatal("restore sentinel errors must be initialized")
	}
	var notArchived error = fmt.Errorf("%w: got %q", errProjectRestoreNotArchived, "Active")
	if !errors.Is(notArchived, errProjectRestoreNotArchived) {
		t.Fatal("wrapped not-archived error must be discoverable via errors.Is")
	}
	var facts error = fmt.Errorf("%w: fund_ledger has 3 rows", errProjectRestoreImmutableFacts)
	if !errors.Is(facts, errProjectRestoreImmutableFacts) {
		t.Fatal("wrapped immutable-facts error must be discoverable via errors.Is")
	}
}