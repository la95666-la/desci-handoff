package chainevent

import (
	"reflect"
	"testing"
)

func TestFairLaunchLocalPrivilegeSpecIncludesAcceptedReversalAndTokenEvidence(t *testing.T) {
	want := map[string][]string{
		"fair_launch_application_lineage_v3":          {"SELECT", "INSERT"},
		"fair_launch_reversal_serialization_guard_v3": {"SELECT"},
		"fair_launch_effect_journal_v3":               {"SELECT", "INSERT"},
		"fair_launch_lineage_reversal_v3":             {"SELECT", "INSERT"},
		"fair_launch_journal_reversal_v3":             {"SELECT", "INSERT"},
		"fair_launch_reversal_evidence_item_v3":       {"SELECT", "INSERT"},
		"fair_launch_reversal_evidence_manifest_v3":   {"SELECT", "INSERT"},
		"fair_launch_reversal_closure_v3":             {"SELECT", "INSERT"},
		"fair_launch_ledger_entry_v3":                 {"SELECT", "INSERT"},
		"fair_launch_governance_ledger_entry_v3":      {"SELECT", "INSERT"},
		"fair_launch_source_token_binding_v3":         {"SELECT"},
	}
	for table, privileges := range want {
		if got := fairLaunchLocalE2ERequiredPrivileges[table]; !reflect.DeepEqual(got, privileges) {
			t.Fatalf("%s privileges=%v, want %v", table, got, privileges)
		}
	}
}

func TestFairLaunchLocalPrivilegeSpecNeverMutatesImmutableEvidence(t *testing.T) {
	immutable := []string{
		"fair_launch_application_lineage_v3",
		"fair_launch_effect_journal_v3",
		"fair_launch_lineage_reversal_v3",
		"fair_launch_journal_reversal_v3",
		"fair_launch_reversal_evidence_item_v3",
		"fair_launch_reversal_evidence_manifest_v3",
		"fair_launch_reversal_closure_v3",
		"fair_launch_ledger_entry_v3",
		"fair_launch_governance_ledger_entry_v3",
		"fair_launch_effect_attestation_v3",
		"fair_launch_governance_history_v3",
		"fair_launch_policy_history_v3",
	}
	for _, table := range immutable {
		for _, privilege := range fairLaunchLocalE2ERequiredPrivileges[table] {
			if privilege == "UPDATE" || privilege == "DELETE" {
				t.Fatalf("immutable %s grants %s", table, privilege)
			}
		}
	}
}

func TestFairLaunchLocalPrivilegeSpecIncludesAdmissionReads(t *testing.T) {
	for _, table := range []string{
		"fair_launch_local_e2e_run",
		"fair_launch_local_e2e_observation",
		"fair_launch_source_revision_binding_v3",
		"fair_launch_project_binding_v3",
		"fair_launch_source_token_binding_v3",
		"chain_contract_source",
		"chain_contract_source_revision",
	} {
		privileges := fairLaunchLocalE2ERequiredPrivileges[table]
		if len(privileges) != 1 || privileges[0] != "SELECT" {
			t.Fatalf("admission evidence %s privileges=%v, want SELECT only", table, privileges)
		}
	}
}

func TestFairLaunchLocalPrivilegeTablesAreStableAndSorted(t *testing.T) {
	tables := fairLaunchLocalE2EPrivilegeTables()
	if len(tables) != len(fairLaunchLocalE2ERequiredPrivileges) {
		t.Fatalf("tables=%d spec=%d", len(tables), len(fairLaunchLocalE2ERequiredPrivileges))
	}
	for i := 1; i < len(tables); i++ {
		if tables[i-1] >= tables[i] {
			t.Fatalf("privilege tables not strictly sorted: %v", tables)
		}
	}
}
