package chainevent

import (
	jsonv2 "encoding/json/v2"
	"strings"
	"testing"
	"time"

	"joincare_server/pkg/common/entity"
)

func TestGenerationSnapshotJSONHidesSensitiveEvidence(t *testing.T) {
	active := uint8(1)
	row := entity.ChainEventReorgGenerationSubmission{
		CallData: []byte("private-call-data"), RawTx: []byte("signed-raw-transaction"), SourceActiveSlot: &active,
		WriterFenceEpoch: 7, WriterLeaseTokenDigest: strings.Repeat("a", 64), LastErrorDigest: strings.Repeat("b", 64),
	}
	payload, err := jsonv2.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	for _, forbidden := range []string{"private-call-data", "signed-raw-transaction", "source_active_slot", "writer_fence_epoch", "writer_lease_token_digest"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("generation snapshot JSON leaked %q: %s", forbidden, text)
		}
	}
}

func TestGenerationSourceDigestsBindNullableAndTimestampFields(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 34, 56, 123456000, time.UTC)
	hash := "0x" + strings.Repeat("1", 64)
	nonce := uint64(9)
	row := entity.ChainTxSubmission{
		ID: 1, ProjectID: 2, Action: "v2_create", AttemptNo: 1, ChainID: 31337,
		SenderAddress: "0x" + strings.Repeat("2", 40), ContractAddress: "0x" + strings.Repeat("3", 40),
		CallData: []byte{1, 2}, ValueRaw: "0", State: "signed", Nonce: &nonce, GasLimit: &nonce,
		GasPriceRaw: "1", RawTx: []byte{3, 4}, TxHash: &hash, BroadcastCount: 1,
		LastBroadcastAt: &now, LastError: "temporary", ReceiptBlockHash: "", CancelReason: "", CreatedAt: now, UpdatedAt: now,
	}
	first := generationSubmissionSourceDigest(row)
	row.LastError = "changed"
	if first == generationSubmissionSourceDigest(row) {
		t.Fatal("submission source digest ignored operational source changes")
	}
	nonceRow := entity.ChainRelayerNonce{ChainID: row.ChainID, SenderAddress: row.SenderAddress, NextNonce: 10, CreatedAt: now, UpdatedAt: now}
	nonceDigest := generationNonceSourceDigest(nonceRow)
	nonceRow.NextNonce++
	if nonceDigest == generationNonceSourceDigest(nonceRow) {
		t.Fatal("nonce source digest ignored next nonce")
	}
}

func TestGenerationDescriptorDigestBindsDatabaseTimestamp(t *testing.T) {
	base := entity.ChainEventReorgGeneration{
		GenerationUUID: "11111111-1111-4111-8111-111111111111", IncidentID: 1, AttemptNo: 1,
		GenerationKind: reorgGenerationKindCandidate, PrerequisiteRunID: 2, PrerequisiteManifestDigest: strings.Repeat("1", 64),
		AssessmentRunID: 3, AssessmentManifestDigest: strings.Repeat("2", 64), MaterializationRunID: 4,
		MaterializationManifestDigest: strings.Repeat("3", 64), ChainID: 31337, BoundaryBlock: 10,
		BoundaryBlockHash: "0x" + strings.Repeat("4", 64), TargetSafeBlock: 11, TargetSafeBlockHash: "0x" + strings.Repeat("5", 64),
		LegacyContractAddress: "0x" + strings.Repeat("6", 40), V2ContractAddress: "0x" + strings.Repeat("7", 40),
		LegacySourceRegistered: true,
		OldEffectRoot: strings.Repeat("8", 64), ReplacementEffectRoot: strings.Repeat("9", 64),
		AlgorithmVersion: reorgGenerationAlgorithm, SchemaVersion: reorgGenerationSchema,
		CreatedAt: time.Date(2026, 8, 15, 12, 0, 0, 123456000, time.UTC),
	}
	digest := generationDescriptorDigest(base)
	base.CreatedAt = base.CreatedAt.Add(time.Microsecond)
	if digest == generationDescriptorDigest(base) {
		t.Fatal("generation descriptor digest ignored database timestamp")
	}
}

func TestGenerationManifestDigestBindsTerminalFenceAndRoots(t *testing.T) {
	base := entity.ChainEventReorgGenerationManifest{
		RunID: 1, IncidentID: 2, DescriptorDigest: strings.Repeat("a", 64), PrerequisiteManifestDigest: strings.Repeat("b", 64),
		AssessmentManifestDigest: strings.Repeat("c", 64), MaterializationManifestDigest: strings.Repeat("d", 64),
		Outcome: reorgGenerationOutcomeBlocked, RequirementCount: 5, UnmetCount: 4, VerifiedCount: 1,
		SenderRoot: strings.Repeat("e", 64), SubmissionRoot: strings.Repeat("f", 64), NonceRoot: strings.Repeat("1", 64),
		RequirementRoot: strings.Repeat("2", 64), EventCount: 2, EventRoot: strings.Repeat("3", 64),
		WriterFenceEpoch: 4, WriterLeaseTokenDigest: strings.Repeat("4", 64),
	}
	digest := generationManifestDigest(base)
	base.EventRoot = strings.Repeat("5", 64)
	if digest == generationManifestDigest(base) {
		t.Fatal("generation manifest digest ignored terminal event root")
	}
	base.EventRoot = strings.Repeat("3", 64)
	base.CandidateProjectCount = 1
	if digest == generationManifestDigest(base) {
		t.Fatal("generation manifest digest ignored candidate counts")
	}
	base.CandidateProjectCount = 0
	base.CandidateRoot = strings.Repeat("6", 64)
	if digest == generationManifestDigest(base) {
		t.Fatal("generation manifest digest ignored candidate roots")
	}
}
