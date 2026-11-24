package entity

import (
	"reflect"
	"strings"
	"testing"
)

func TestFairLaunchV3EntityColumnsMatchAcceptedDeploySchema(t *testing.T) {
	cases := []struct {
		model interface{ TableName() string }
		want  string
	}{
		{&FairLaunchLocalE2ERun{}, "id run_id environment exact_rpc_url redis_endpoint chain_id database_name mysql_server_uuid mysql_version container_id owner_label_digest schema_generation baseline_digest migration_manifest_digest schema_digest grant_digest source_registry_digest hardhat_genesis_hash chain_manifest_digest deployment_manifest_digest runtime_admission_digest created_at expires_at closed_at state"},
		{&FairLaunchLocalE2EObservation{}, "id run_row_id observation_kind observed_value value_digest collector_version observed_at"},
		{&FairLaunchSourceRevisionBindingV3{}, "id run_row_id chain_id source_id source_revision_id from_block_number from_transaction_index from_tx_log_ordinal_inclusive to_block_number to_transaction_index to_tx_log_ordinal_exclusive source_binding_digest created_at"},
		{&FairLaunchProjectBindingV3{}, "id run_row_id project_row_id chain_id contract_address contract_version project_id project_id_raw source_binding_id source_id source_revision_id project_token_address project_token_decimals payment_token_address payment_token_decimals binding_digest created_at"},
		{&FairLaunchProjectStateV3{}, "id project_binding_id run_row_id chain_id source_id contract_address contract_version project_id project_token_address payment_token_address status terminal_reason sale_token_amount_raw total_raised_raw total_tokens_deposited_raw total_tokens_claimed_raw participant_count_raw final_total_raised_raw final_price_numerator_raw final_price_denominator_raw platform_fee_reserved_raw project_proceeds_paid_raw platform_fee_paid_raw settlement_block_number settlement_block_hash settlement_anchor_block_number settlement_anchor_block_hash settlement_tx_hash settlement_tx_log_ordinal release_manifest_digest release_policy_digest release_policy_version project_proceeds_withdrawn platform_fee_withdrawn failed_project_tokens_withdrawn tombstoned state_generation last_event_identity_id last_placement_id last_source_revision_id last_effect_manifest_version last_application_attempt_id source_application_lineage_id bootstrap_receipt_id state_digest created_at updated_at"},
		{&FairLaunchPositionV3{}, "id project_binding_id run_row_id chain_id source_id contract_address contract_version project_id wallet_address payment_token_address payment_token_decimals contribution_amount_raw contribution_amount_display project_token_address project_token_decimals claimed_tokens_raw claimed_tokens_display refunded first_contribution_event_id last_contribution_event_id last_claim_event_id last_refund_event_id last_event_identity_id last_placement_id last_source_revision_id last_effect_manifest_version last_application_attempt_id source_application_lineage_id state_generation state_digest tombstoned status created_at updated_at"},
		{&FairLaunchGovernanceCurrentV3{}, "source_id run_row_id latest_source_revision_id chain_id contract_address operator_address pause_guardian_address unpause_timelock_address treasury_address platform_fee_receiver platform_fee_bps contributions_paused payouts_paused claims_pause_until state_generation last_event_identity_id last_placement_id last_source_revision_id last_effect_manifest_version last_application_attempt_id source_application_lineage_id state_digest updated_at"},
		{&FairLaunchGovernanceHistoryV3{}, "id run_row_id chain_id source_id source_revision_id state_generation before_payload_jcs before_digest after_payload_jcs after_digest source_event_identity_id source_placement_id source_application_attempt_id effect_manifest_version source_application_lineage_id created_at"},
		{&FairLaunchPolicyCurrentV3{}, "id run_row_id chain_id source_id latest_source_revision_id policy_kind subject_address policy_version enabled emergency_blocked token_decimals runtime_code_hash policy_digest attestation_digest state_generation last_event_identity_id last_placement_id last_source_revision_id last_effect_manifest_version last_application_attempt_id source_application_lineage_id state_digest updated_at"},
		{&FairLaunchPolicyHistoryV3{}, "id policy_current_id run_row_id chain_id source_id source_revision_id state_generation before_payload_jcs before_digest after_payload_jcs after_digest source_event_identity_id source_placement_id source_application_attempt_id effect_manifest_version source_application_lineage_id created_at"},
		{&FairLaunchApplicationLineageV3{}, "id ce_application_attempt_id run_row_id chain_id source_binding_id source_id event_identity_id placement_id source_revision_id effect_manifest_version placement_generation predecessor_lineage_id reverses_lineage_id disposition expected_effect payload_digest effect_digest ledger_digest mutation_digest state applied_at created_at"},
		{&FairLaunchReversalSerializationGuardV3{}, "reversed_lineage_id reversal_lineage_id run_row_id chain_id source_binding_id source_id source_revision_id state created_at bound_at closed_at"},
		{&FairLaunchEffectGuardV3{}, "run_row_id chain_id source_id event_identity_id effect_manifest_version placement_generation source_application_lineage_id state active_slot payload_digest effect_digest created_at"},
		{&FairLaunchEffectJournalV3{}, "id run_row_id chain_id source_id event_identity_id placement_id source_revision_id source_application_lineage_id effect_manifest_version effect_kind aggregate_kind aggregate_key mutation_ordinal before_payload_jcs before_digest after_payload_jcs after_digest created_at"},
		{&FairLaunchLineageReversalV3{}, "reversed_lineage_id reversal_lineage_id reversed_at created_at"},
		{&FairLaunchJournalReversalV3{}, "source_journal_id reversed_lineage_id reversal_lineage_id created_at"},
		{&FairLaunchReversalEvidenceItemV3{}, "reversed_lineage_id reversal_lineage_id evidence_ordinal evidence_kind evidence_id evidence_digest previous_chain_digest item_chain_digest created_at"},
		{&FairLaunchReversalEvidenceManifestV3{}, "reversed_lineage_id reversal_lineage_id source_journal_count dependency_count attestation_count cursor_checkpoint_count governance_history_count policy_history_count source_ledger_count source_governance_ledger_count evidence_item_count final_item_chain_digest manifest_digest created_at"},
		{&FairLaunchReversalClosureV3{}, "reversed_lineage_id reversal_lineage_id evidence_manifest_digest source_journal_count linked_journal_count dependency_count attestation_count cursor_checkpoint_count governance_history_count policy_history_count source_ledger_count compensation_ledger_count source_governance_ledger_count compensation_governance_ledger_count closure_digest created_at"},
		{&FairLaunchEffectDependencyV3{}, "id predecessor_lineage_id dependent_lineage_id run_row_id chain_id source_binding_id source_id source_revision_id effect_manifest_version aggregate_kind aggregate_key dependency_digest created_at"},
		{&FairLaunchEffectAttestationV3{}, "id run_row_id chain_id source_binding_id source_id event_identity_id placement_id source_revision_id source_application_lineage_id effect_manifest_version payload_digest effect_digest ledger_digest mutation_digest ledger_row_count mutation_row_count project_mutation_count position_mutation_count governance_mutation_count policy_mutation_count attestation_digest created_at"},
		{&FairLaunchCursorCheckpointV3{}, "id run_row_id chain_id source_binding_id source_id source_revision_id predecessor_block_number predecessor_block_hash predecessor_tx_index predecessor_tx_log_ordinal predecessor_event_identity_id reason reversal_application_lineage_id checkpoint_digest created_at"},
		{&FairLaunchLedgerEntryV3{}, "id run_row_id chain_id source_id project_binding_id project_id contract_address contract_version wallet_address counterparty_address entry_type asset_role flow_direction token_address token_decimals amount_raw amount_display source_event_identity_id source_placement_id source_application_lineage_id source_revision_id entry_ordinal effect_manifest_version reversal_of_ledger_id compensation_generation row_digest created_at"},
		{&FairLaunchSourceTokenBindingV3{}, "id run_row_id chain_id source_id contract_address token_address token_decimals metadata_version admission_generation runtime_code_hash metadata_digest admitted_at"},
		{&FairLaunchGovernanceLedgerEntryV3{}, "id run_row_id chain_id source_id contract_address token_binding_id entry_type asset_role flow_direction token_address receiver_address token_decimals amount_raw amount_display source_application_lineage_id source_event_identity_id source_placement_id source_revision_id entry_ordinal effect_manifest_version reversal_of_ledger_id compensation_generation row_digest created_at"},
		{&FairLaunchMainnetArchiveCursorV1{}, "activation_row_id chain_id source_id source_revision_id archived_through_block archived_through_block_hash lease_generation fence_epoch lease_token_hash status last_error_code last_error_digest last_batch_receipt_id last_recovery_receipt_id cursor_generation created_at updated_at"},
		{&FairLaunchMainnetArchiveBatchReceiptV1{}, "id activation_row_id chain_id source_id source_revision_id lease_generation fence_epoch from_block through_block from_parent_block_hash through_block_hash provider_identity_digest block_count event_count coverage_digest previous_receipt_digest receipt_digest created_at"},
		{&FairLaunchMainnetArchiveBatchEventV1{}, "id batch_receipt_id activation_row_id event_ordinal event_identity_id placement_id observation_id event_name payload_digest row_digest created_at"},
		{&FairLaunchMainnetArchiveEventPayloadV1{}, "archive_event_id activation_row_id event_identity_id placement_id observation_id topic_count topic0 topic1 topic2 topic3 data_hex payload_row_digest created_at"},
		{&FairLaunchMainnetMaterializationProfileV1{}, "activation_row_id chain_id source_id source_revision_id source_binding_id bootstrap_receipt_id bootstrap_archive_event_id initial_event_identity_id effect_manifest_version effect_manifest_digest initial_block_number initial_block_hash initial_transaction_index initial_tx_log_ordinal profile_digest approval_evidence_digest valid_from valid_until created_at"},
		{&FairLaunchMainnetMaterializationCursorV1{}, "activation_row_id chain_id source_id source_revision_id source_binding_id materialized_through_block materialized_through_block_hash materialized_through_tx_index materialized_through_tx_log_ordinal last_event_identity_id last_archive_event_id last_materialization_receipt_id lease_generation fence_epoch lease_token_hash status cursor_generation created_at updated_at"},
		{&FairLaunchMainnetMaterializationReceiptV1{}, "id activation_row_id chain_id source_id source_revision_id source_binding_id archive_batch_receipt_id archive_event_id event_identity_id placement_id observation_id application_attempt_id source_application_lineage_id attestation_id effect_manifest_version payload_digest effect_digest ledger_digest mutation_digest attestation_digest lease_generation fence_epoch previous_cursor_generation previous_receipt_digest receipt_digest created_at"},
		{&FairLaunchMainnetArchiveDeadLetterV1{}, "id activation_row_id chain_id source_id source_revision_id block_number block_hash transaction_index tx_log_ordinal rpc_log_index tx_hash reason_code evidence_digest created_at"},
		{&FairLaunchMainnetArchiveRecoveryReceiptV1{}, "id activation_row_id chain_id source_id source_revision_id dead_letter_id previous_status reason_code previous_error_digest archived_through_block archived_through_block_hash last_batch_receipt_id previous_cursor_generation previous_lease_id replacement_lease_id recovery_action confirmation_policy verification_provider_set_digest verification_count canonical_chain_digest approval_evidence_digest recovery_manifest_digest receipt_digest created_at"},
		{&FairLaunchMainnetRecoveryProviderV1{}, "id activation_row_id chain_id source_id source_revision_id provider_identity rpc_identity_digest provider_role admission_manifest_digest approval_evidence_digest valid_from valid_until created_at"},
	}

	for _, tc := range cases {
		t.Run(tc.model.TableName(), func(t *testing.T) {
			got := entityColumns(tc.model)
			want := strings.Fields(tc.want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("columns mismatch\n got: %v\nwant: %v", got, want)
			}
		})
	}
}

func entityColumns(model interface{ TableName() string }) []string {
	typ := reflect.TypeOf(model).Elem()
	columns := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		for _, option := range strings.Split(typ.Field(i).Tag.Get("gorm"), ";") {
			if strings.HasPrefix(option, "column:") {
				columns = append(columns, strings.TrimPrefix(option, "column:"))
			}
		}
	}
	return columns
}
