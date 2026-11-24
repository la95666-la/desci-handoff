package chainevent

import "sort"

// fairLaunchLocalE2ERequiredPrivileges mirrors the accepted Deploy local_job
// policy at commit 8eef5accc07dcd2540eb7f2d3a469b4e5a7b8431. It is a static
// comparison contract only; it does not connect, grant, or mint capability.
var fairLaunchLocalE2ERequiredPrivileges = map[string][]string{
	"chain_event_application_attempt_v2":          {"SELECT", "INSERT", "UPDATE"},
	"chain_event_block_checkpoint":                {"SELECT", "INSERT", "UPDATE"},
	"chain_event_failure":                         {"SELECT", "INSERT", "UPDATE"},
	"chain_event_identity_quarantine_v2":          {"SELECT", "INSERT", "UPDATE"},
	"chain_event_payload_conflict_v2":             {"SELECT", "INSERT", "UPDATE"},
	"chain_event_placement_observation_v2":        {"SELECT", "INSERT", "UPDATE"},
	"chain_event_placement_v2":                    {"SELECT", "INSERT", "UPDATE"},
	"chain_event_record":                          {"SELECT", "INSERT", "UPDATE"},
	"chain_event_sync_state":                      {"SELECT", "INSERT", "UPDATE"},
	"chain_contract_source":                       {"SELECT"},
	"chain_contract_source_revision":              {"SELECT"},
	"fair_launch_application_lineage_v3":          {"SELECT", "INSERT"},
	"fair_launch_cursor_checkpoint_v3":            {"SELECT", "INSERT"},
	"fair_launch_effect_attestation_v3":           {"SELECT", "INSERT"},
	"fair_launch_effect_dependency_v3":            {"SELECT", "INSERT"},
	"fair_launch_effect_guard_v3":                 {"SELECT", "INSERT", "UPDATE"},
	"fair_launch_effect_journal_v3":               {"SELECT", "INSERT"},
	"fair_launch_feature_watermark":               {"SELECT"},
	"fair_launch_governance_current_v3":           {"SELECT", "INSERT", "UPDATE"},
	"fair_launch_governance_history_v3":           {"SELECT", "INSERT"},
	"fair_launch_governance_ledger_entry_v3":      {"SELECT", "INSERT"},
	"fair_launch_journal_reversal_v3":             {"SELECT", "INSERT"},
	"fair_launch_ledger_entry_v3":                 {"SELECT", "INSERT"},
	"fair_launch_lineage_reversal_v3":             {"SELECT", "INSERT"},
	"fair_launch_local_e2e_observation":           {"SELECT"},
	"fair_launch_local_e2e_run":                   {"SELECT"},
	"fair_launch_policy_current_v3":               {"SELECT", "INSERT", "UPDATE"},
	"fair_launch_policy_history_v3":               {"SELECT", "INSERT"},
	"fair_launch_position_v3":                     {"SELECT", "INSERT", "UPDATE"},
	"fair_launch_project_binding_v3":              {"SELECT"},
	"fair_launch_project_state_v3":                {"SELECT", "INSERT", "UPDATE"},
	"fair_launch_reversal_closure_v3":             {"SELECT", "INSERT"},
	"fair_launch_reversal_evidence_item_v3":       {"SELECT", "INSERT"},
	"fair_launch_reversal_evidence_manifest_v3":   {"SELECT", "INSERT"},
	"fair_launch_reversal_serialization_guard_v3": {"SELECT"},
	"fair_launch_source_registry_snapshot":        {"SELECT"},
	"fair_launch_source_revision_binding_v3":      {"SELECT"},
	"fair_launch_source_token_binding_v3":         {"SELECT"},
	"project_tge":                                 {"SELECT"},
	"projects":                                    {"SELECT"},
	"settings":                                    {"SELECT"},
	"source_materialization_cursor":               {"SELECT", "INSERT", "UPDATE"},
}

func fairLaunchLocalE2EPrivilegeTables() []string {
	tables := make([]string, 0, len(fairLaunchLocalE2ERequiredPrivileges))
	for table := range fairLaunchLocalE2ERequiredPrivileges {
		tables = append(tables, table)
	}
	sort.Strings(tables)
	return tables
}
