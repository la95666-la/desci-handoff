package controller

import (
	"encoding/json/v2"
	"strings"
	"testing"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

func TestChainEventAdminReplayable(t *testing.T) {
	tests := []struct {
		name string
		row  entity.ChainEventFailure
		want bool
	}{
		{name: "ordinary dead letter", row: entity.ChainEventFailure{Event: "ProjectCreated", Status: constant.ChainEventFailureDeadLetter}, want: true},
		{name: "reconciliation dead letter", row: entity.ChainEventFailure{Event: "reconciliation", Status: constant.ChainEventFailureDeadLetter}, want: false},
		{name: "retrying", row: entity.ChainEventFailure{Event: "ProjectCreated", Status: constant.ChainEventFailureRetrying}, want: false},
		{name: "resolved", row: entity.ChainEventFailure{Event: "ProjectCreated", Status: constant.ChainEventFailureResolved}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := chainEventAdminReplayable(test.row); got != test.want {
				t.Fatalf("chainEventAdminReplayable=%t want=%t", got, test.want)
			}
		})
	}
}

func TestNextChainEventReplayNo(t *testing.T) {
	for _, test := range []struct {
		current uint
		want    uint
	}{{current: 0, want: 1}, {current: 1, want: 2}, {current: 7, want: 8}} {
		failure := entity.ChainEventFailure{ReplayCount: test.current}
		if got := nextChainEventReplayNo(failure); got != test.want {
			t.Fatalf("nextChainEventReplayNo(%d)=%d want=%d", test.current, got, test.want)
		}
	}
}

func TestChainEventFailureJSONNeverLeaksRawLog(t *testing.T) {
	record := entity.ChainEventFailure{RawLog: "secret-raw-chain-log", AdminReplayable: true, ReplayAudits: []entity.ChainEventReplayAudit{{FailureID: 7, ReplayNo: 1, AdminID: 42, Reason: "canonical log verified"}}}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"raw_log", "secret-raw-chain-log"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("serialized chain event failure leaked %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, `"admin_replayable":true`) {
		t.Fatalf("serialized failure omitted the safe replay flag: %s", text)
	}
	for _, evidence := range []string{`"replay_audits":[`, `"replay_no":1`, `"admin_id":42`, `"reason":"canonical log verified"`} {
		if !strings.Contains(text, evidence) {
			t.Fatalf("serialized failure omitted replay evidence %s: %s", evidence, text)
		}
	}
}

func TestChainEventHealthSnapshotsNeverLeakOperationalErrorsOrPayloads(t *testing.T) {
	sync := safeChainEventSyncState(entity.ChainEventSyncState{LastError: "rpc https://secret.example failed with database payload"})
	recovery := entity.ChainEventReorgRecoveryJob{
		LeaseToken: "secret-recovery-lease", LastErrorCode: "RECOVERY_SCAN_FAILED", LastErrorMessage: "secret worker/database error",
	}
	recoveryLog := entity.ChainEventReorgRecoveryLog{RawLog: []byte("secret raw recovery log")}
	materialization := entity.ChainEventReorgMaterializationJob{LeaseToken: "secret-materialization-lease", LastErrorCode: "MATERIALIZATION_PREPARE_FAILED", LastErrorMessage: "secret materialization worker error"}
	materializationEffect := entity.ChainEventReorgMaterializationEffect{SemanticPayload: []byte("secret semantic payload")}
	materializationSource := entity.ChainEventReorgMaterializationOldSource{EventData: []byte("secret old source payload")}
	for name, value := range map[string]any{"sync": sync, "recovery": recovery, "recovery_log": recoveryLog, "materialization": materialization, "materialization_effect": materializationEffect, "materialization_source": materializationSource} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		text := string(encoded)
		for _, forbidden := range []string{"secret.example", "database payload", "secret-recovery-lease", "secret worker/database error", "secret raw recovery log", "secret-materialization-lease", "secret materialization worker error", "secret semantic payload", "secret old source payload", `"lease_token":`, `"raw_log":`, `"semantic_payload":`, `"event_data":`} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s serialization leaked %q: %s", name, forbidden, text)
			}
		}
	}
	encoded, _ := json.Marshal(sync)
	if !strings.Contains(string(encoded), `"last_error_code":"CHAIN_EVENT_SYNC_FAILED"`) {
		t.Fatalf("safe sync snapshot omitted stable error code: %s", encoded)
	}
}

func TestMaterializationHealthSummaryExposesOnlySafeEvidence(t *testing.T) {
	summary := chainEventReorgMaterializationSummary{ID: 1, RunUUID: "safe-run", RecoveryManifestDigest: strings.Repeat("a", 64), OldEffectDigest: strings.Repeat("b", 64), ReplacementEffectDigest: strings.Repeat("c", 64), LastErrorCode: "MATERIALIZATION_PREPARE_FAILED"}
	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, required := range []string{"recovery_manifest_digest", "old_effect_digest", "replacement_effect_digest", "last_error_code"} {
		if !strings.Contains(text, required) {
			t.Fatalf("safe materialization summary omitted %q: %s", required, text)
		}
	}
	for _, forbidden := range []string{"semantic_payload", "event_data", "old_record_source_digest", "replacement_log_source_digest", "next_old_record_id", "lease_token", "lease_expires_at", "fence_epoch", "last_error_message"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("safe materialization summary leaked %q: %s", forbidden, text)
		}
	}
}

func TestAssessmentHealthSummaryIsExplicitlyNonAuthorizingAndSafe(t *testing.T) {
	outcome, manifest := "BLOCKED", strings.Repeat("a", 64)
	summary := chainEventReorgAssessmentSummary{ID: 1, RunUUID: "safe-assessment", State: "completed", Outcome: &outcome, ManifestDigest: &manifest, BlockerCount: 7, SwitchAuthorized: false, Blockers: []entity.ChainEventReorgAssessmentBlocker{{ID: 1, RunID: 1, Ordinal: 1, Scope: "run", BlockerCode: "BOUNDARY_STATE_SNAPSHOT_REQUIRED", SafeDetail: "an attested boundary snapshot is not installed", RowDigest: strings.Repeat("b", 64)}}}
	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, required := range []string{`"outcome":"BLOCKED"`, `"switch_authorized":false`, `"blocker_code":"BOUNDARY_STATE_SNAPSHOT_REQUIRED"`} {
		if !strings.Contains(text, required) {
			t.Fatalf("assessment summary omitted %s: %s", required, text)
		}
	}
	for _, forbidden := range []string{"plan_payload", "source_effect_digest", "materialization_effect_id", "lease_token", "lease_expires_at", "fence_epoch", "last_error_message", "semantic_payload"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("assessment summary leaked %q: %s", forbidden, text)
		}
	}
}

func TestAssessmentDispositionEntityDoesNotLeakSourceOrPlanPayload(t *testing.T) {
	row := entity.ChainEventReorgAssessmentDisposition{MaterializationEffectID: 9, SourceEffectDigest: "secret-source-effect", PlanPayload: []byte("secret-plan-payload"), PlanPayloadDigest: strings.Repeat("a", 64)}
	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{`"materialization_effect_id":`, `"source_effect_digest":`, "secret-source-effect", `"plan_payload":`, "secret-plan-payload"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("assessment disposition leaked %q: %s", forbidden, text)
		}
	}
}

func TestPrerequisiteHealthSummaryIsBlockedAndSafe(t *testing.T) {
	outcome, manifest := "BLOCKED", strings.Repeat("a", 64)
	summary := chainEventReorgPrerequisiteSummary{ID: 1, RunUUID: "safe-prerequisite", State: "completed",
		Outcome: &outcome, CandidateReady: false, SatisfiedCount: 0, ArtifactCount: 0, ManifestDigest: &manifest,
		Requirements: []entity.ChainEventReorgPrerequisiteRequirement{{ID: 1, RunID: 1, Ordinal: 1,
			RequirementCode: "BOUNDARY_STATE_SNAPSHOT_REQUIRED", EvidenceStatus: "unmet",
			SafeDetail: "no trusted boundary state snapshot is installed", RowDigest: strings.Repeat("b", 64)}}}
	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, required := range []string{`"outcome":"BLOCKED"`, `"candidate_ready":false`, `"satisfied_count":0`, `"evidence_status":"unmet"`} {
		if !strings.Contains(text, required) {
			t.Fatalf("prerequisite summary omitted %s: %s", required, text)
		}
	}
	for _, forbidden := range []string{"source_payload", "secret payload", "lease_token", "lease_expires_at", "fence_epoch", "last_error_message"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("prerequisite summary leaked %q: %s", forbidden, text)
		}
	}
}

func TestPrerequisiteArtifactEntityDoesNotLeakPayload(t *testing.T) {
	row := entity.ChainEventReorgPrerequisiteArtifact{SourcePayload: []byte("secret payload"), SourceDigest: strings.Repeat("a", 64)}
	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "source_payload") || strings.Contains(string(encoded), "secret payload") {
		t.Fatalf("prerequisite artifact leaked payload: %s", encoded)
	}
}

func TestGenerationHealthSummaryIsBlockedAndSafe(t *testing.T) {
	outcome, manifest := "BLOCKED", strings.Repeat("a", 64)
	summary := chainEventReorgGenerationSummary{ID: 1, GenerationUUID: "safe-generation", State: "completed", Stage: "done",
		Outcome: &outcome, ManifestDigest: &manifest, CandidateReady: false, RequirementCount: 5, UnmetCount: 4,
		VerifiedCount: 1, SubmissionCount: 2, NonceCount: 1, SubmissionRoot: strings.Repeat("b", 64),
		NonceRoot: strings.Repeat("c", 64), Requirements: []entity.ChainEventReorgGenerationRequirement{{ID: 1,
			RunID: 1, Ordinal: 1, RequirementCode: "BOUNDARY_STATE_SNAPSHOT_REQUIRED", EvidenceStatus: "unmet",
			SafeDetail: "trusted boundary snapshot is unavailable", RowDigest: strings.Repeat("d", 64)}}}
	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, required := range []string{`"candidate_ready":false`, `"submission_count":2`, `"requirement_code":"BOUNDARY_STATE_SNAPSHOT_REQUIRED"`} {
		if !strings.Contains(text, required) {
			t.Fatalf("generation summary omitted %s: %s", required, text)
		}
	}
	for _, forbidden := range []string{"call_data", "raw_tx", "sender_address", "next_nonce", "lease_token", "fence_epoch", "last_error_message", "writer_lease_token_digest"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("generation summary leaked %q: %s", forbidden, text)
		}
	}
}
