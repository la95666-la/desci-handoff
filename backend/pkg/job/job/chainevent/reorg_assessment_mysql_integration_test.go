package chainevent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	commonSchema "joincare_server/pkg/common/database/schema"
	"joincare_server/pkg/common/entity"
	jobDB "joincare_server/pkg/job/db"
)

func TestReorgAssessmentWorkerAgainstMySQL(t *testing.T) {
	database := openReorgWorkerVerificationMySQL(t)
	for name, gate := range map[string]func(*gorm.DB) error{
		"chain event collation compatibility": commonSchema.RequireChainEventCollationCompatibility,
		"purchase entitlement":                commonSchema.RequireIDOPurchaseEntitlement,
		"reconciliation evidence":             commonSchema.RequireIDOReconciliationEvidence,
		"reorg evidence":                      commonSchema.RequireChainEventReorgEvidence,
		"reorg recovery scan":                 commonSchema.RequireChainEventReorgRecoveryScan,
		"reorg materialization":               commonSchema.RequireChainEventReorgMaterialization,
		"reorg assessment":                    commonSchema.RequireChainEventReorgAssessment,
	} {
		if err := gate(database); err != nil {
			t.Fatalf("%s schema gate: %v", name, err)
		}
	}
	previous := jobDB.Client.Mysql
	jobDB.Client.Mysql = database
	t.Cleanup(func() { jobDB.Client.Mysql = previous })

	fixture := createAssessmentWorkerFixture(t, database, 101)
	run := fixture.AssessmentRun

	firstToken := strings.Repeat("1", 64)
	first, err := claimAssessmentLease(run.ID, firstToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := claimAssessmentLease(run.ID, strings.Repeat("2", 64)); err == nil {
		t.Fatal("concurrent assessment lease was accepted")
	}
	if err := processAssessmentPage(context.Background(), &run, first, firstToken); err != nil {
		t.Fatal(err)
	}
	assertAssessmentCheckpoint(t, database, run.ID, 0, 7, 1, reorgAssessmentStatePreparing)

	secondToken := strings.Repeat("3", 64)
	second, err := claimAssessmentLease(run.ID, secondToken)
	if err != nil {
		t.Fatal(err)
	}
	assertInvalidAssessmentPlansRejected(t, database, run, second)
	assertAssessmentPageRollback(t, database, run, second, secondToken)
	if err := processAssessmentPage(context.Background(), &run, second, secondToken); err != nil {
		t.Fatal(err)
	}
	assertAssessmentCheckpoint(t, database, run.ID, 100, 8, 101, reorgAssessmentStatePreparing)

	if err := processAssessmentPage(context.Background(), &run, first, firstToken); err == nil {
		t.Fatal("stale first assessment lease wrote after a later fence epoch")
	}
	assertAssessmentCheckpoint(t, database, run.ID, 100, 8, 101, reorgAssessmentStatePreparing)

	thirdToken := strings.Repeat("4", 64)
	third, err := claimAssessmentLease(run.ID, thirdToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&entity.ChainEventReorgAssessmentJob{}).
		Where("run_id = ? AND lease_token = ? AND fence_epoch = ?", run.ID, thirdToken, third.FenceEpoch).
		Update("lease_expires_at", gorm.Expr("DATE_SUB(CURRENT_TIMESTAMP(6), INTERVAL 1 SECOND)")).Error; err != nil {
		t.Fatal(err)
	}
	fourthToken := strings.Repeat("5", 64)
	fourth, err := claimAssessmentLease(run.ID, fourthToken)
	if err != nil {
		t.Fatal(err)
	}
	if fourth.FenceEpoch != third.FenceEpoch+1 {
		t.Fatalf("lease takeover fence epoch=%d want=%d", fourth.FenceEpoch, third.FenceEpoch+1)
	}
	if err := processAssessmentPage(context.Background(), &run, third, thirdToken); err == nil {
		t.Fatal("expired assessment lease wrote after DB-clock takeover")
	}
	if err := processAssessmentPage(context.Background(), &run, fourth, fourthToken); err != nil {
		t.Fatal(err)
	}
	assertAssessmentCheckpoint(t, database, run.ID, 101, 8, 102, reorgAssessmentStatePreparing)

	finalToken := strings.Repeat("6", 64)
	finalLease, err := claimAssessmentLease(run.ID, finalToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := processAssessmentPage(context.Background(), &run, finalLease, finalToken); err != nil {
		t.Fatal(err)
	}
	assertAssessmentCheckpoint(t, database, run.ID, 101, 8, 102, reorgAssessmentStateCompleted)

	if _, err := claimAssessmentLease(run.ID, strings.Repeat("7", 64)); err == nil {
		t.Fatal("completed assessment accepted another lease")
	}
	beforeReplay := assessmentEvidenceCardinality(t, database, run.ID)
	if err := processAssessmentPage(context.Background(), &run, finalLease, finalToken); err == nil {
		t.Fatal("completed assessment accepted authentic final-lease replay")
	}
	if afterReplay := assessmentEvidenceCardinality(t, database, run.ID); afterReplay != beforeReplay {
		t.Fatalf("completed assessment replay changed evidence: before=%+v after=%+v", beforeReplay, afterReplay)
	}
	assertAssessmentEvidence(t, database, run, fixture.MaterializationManifest)
	exercisePrerequisiteInventory(t, database, fixture.Incident)
}

func TestReorgPrerequisiteSourceDriftAgainstMySQL(t *testing.T) {
	database := openReorgWorkerVerificationMySQL(t)
	previous := jobDB.Client.Mysql
	jobDB.Client.Mysql = database
	t.Cleanup(func() { jobDB.Client.Mysql = previous })
	fixture := createAssessmentWorkerFixture(t, database, 1)
	completeAssessmentFixture(t, database, fixture.AssessmentRun)
	run, _, err := ensurePrerequisiteRunForIncident(&fixture.Incident)
	if err != nil {
		t.Fatal(err)
	}
	firstToken := strings.Repeat("a", 64)
	first, err := claimPrerequisiteLease(run.ID, firstToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := processPrerequisiteStage(run, first, firstToken); err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&entity.ChainEventSyncState{}).Where("id = 1").Update("cursor_block_hash", strings.ToLower(common.HexToHash("0xdead").Hex())).Error; err != nil {
		t.Fatal(err)
	}
	finalToken := strings.Repeat("b", 64)
	finalLease, err := claimPrerequisiteLease(run.ID, finalToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := processPrerequisiteStage(run, finalLease, finalToken); err != nil {
		t.Fatal(err)
	}
	job := entity.ChainEventReorgPrerequisiteJob{}
	if err := database.Where("run_id = ?", run.ID).Take(&job).Error; err != nil {
		t.Fatal(err)
	}
	if job.State != reorgPrerequisiteStateInvalidated || job.LastErrorCode != "SOURCE_DRIFT" || job.ActiveSlot != nil {
		t.Fatalf("prerequisite source drift did not invalidate run: %+v", job)
	}
	var manifests int64
	if err := database.Model(&entity.ChainEventReorgPrerequisiteManifest{}).Where("run_id = ?", run.ID).Count(&manifests).Error; err != nil {
		t.Fatal(err)
	}
	if manifests != 0 {
		t.Fatalf("source-drifted prerequisite run produced %d manifests", manifests)
	}
	events := make([]entity.ChainEventReorgPrerequisiteEvent, 0, 2)
	if err := database.Where("run_id = ?", run.ID).Order("sequence_no ASC").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].EventType != "CREATED" || events[1].EventType != "INVALIDATED" || events[1].ErrorCode != "SOURCE_DRIFT" {
		t.Fatalf("source drift terminal evidence invalid: %+v", events)
	}
}

func TestReorgGenerationFoundationAgainstMySQL(t *testing.T) {
	database := openReorgWorkerVerificationMySQL(t)
	previous := jobDB.Client.Mysql
	jobDB.Client.Mysql = database
	t.Cleanup(func() { jobDB.Client.Mysql = previous })
	fixture := createAssessmentWorkerFixture(t, database, 1)
	completeAssessmentFixture(t, database, fixture.AssessmentRun)
	prerequisite := exercisePrerequisiteInventory(t, database, fixture.Incident)
	_ = prerequisite

	sender := "0x4000000000000000000000000000000000000004"
	unrelatedSender := "0x5000000000000000000000000000000000000005"
	otherContract := "0x6000000000000000000000000000000000000006"
	now := time.Now().Truncate(time.Microsecond)
	active := uint8(1)
	direct := entity.ChainTxSubmission{ProjectID: 1, Action: "v2_create", AttemptNo: 1, ChainID: fixture.Incident.ChainID,
		SenderAddress: sender, ContractAddress: fixture.Incident.ContractAddress, CallData: []byte{1, 2}, ValueRaw: "0",
		State: constant.ChainTxStateIntent, GasPriceRaw: "0", ActiveSlot: &active, LastError: "", ReceiptBlockHash: "",
		CreatedAt: now, UpdatedAt: now}
	if err := database.Create(&direct).Error; err != nil {
		t.Fatal(err)
	}
	other := entity.ChainTxSubmission{ProjectID: 2, Action: "v2_activate", AttemptNo: 1, ChainID: fixture.Incident.ChainID,
		SenderAddress: sender, ContractAddress: otherContract, CallData: []byte{3, 4}, ValueRaw: "0",
		State: constant.ChainTxStateReady, GasPriceRaw: "0", ActiveSlot: &active, LastError: "", ReceiptBlockHash: "",
		CreatedAt: now, UpdatedAt: now}
	if err := database.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	unrelated := entity.ChainTxSubmission{ProjectID: 3, Action: "v2_create", AttemptNo: 1, ChainID: fixture.Incident.ChainID,
		SenderAddress: unrelatedSender, ContractAddress: otherContract, CallData: []byte{5, 6}, ValueRaw: "0",
		State: constant.ChainTxStateIntent, GasPriceRaw: "0", ActiveSlot: &active, LastError: "", ReceiptBlockHash: "",
		CreatedAt: now, UpdatedAt: now}
	if err := database.Create(&unrelated).Error; err != nil {
		t.Fatal(err)
	}
	nonce := entity.ChainRelayerNonce{ChainID: fixture.Incident.ChainID, SenderAddress: sender, NextNonce: 9,
		ActiveSubmissionID: &direct.ID, CreatedAt: now, UpdatedAt: now}
	if err := database.Create(&nonce).Error; err != nil {
		t.Fatal(err)
	}
	before := []entity.ChainTxSubmission{}
	if err := database.Order("id ASC").Find(&before).Error; err != nil {
		t.Fatal(err)
	}

	var descriptorClockBefore time.Time
	if err := database.Raw("SELECT CURRENT_TIMESTAMP(6)").Scan(&descriptorClockBefore).Error; err != nil {
		t.Fatal(err)
	}
	run, initial, err := ensureGenerationRunForIncident(&fixture.Incident)
	if err != nil {
		t.Fatal(err)
	}
	var descriptorClockAfter time.Time
	if err := database.Raw("SELECT CURRENT_TIMESTAMP(6)").Scan(&descriptorClockAfter).Error; err != nil {
		t.Fatal(err)
	}
	if run.CreatedAt.Before(descriptorClockBefore) || run.CreatedAt.After(descriptorClockAfter) || run.DescriptorDigest != generationDescriptorDigest(*run) {
		t.Fatalf("generation descriptor is not bound to database clock: before=%s created=%s after=%s", descriptorClockBefore, run.CreatedAt, descriptorClockAfter)
	}
	if initial.Stage != reorgGenerationStageSnapshot || run.AttemptNo != 1 {
		t.Fatalf("generation initial state invalid: run=%+v job=%+v", run, initial)
	}
	firstToken := strings.Repeat("7", 64)
	first, err := claimGenerationLease(run.ID, firstToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := processGenerationStage(run, first, firstToken); err != nil {
		t.Fatal(err)
	}
	snapshotJob := entity.ChainEventReorgGenerationJob{}
	if err := database.Where("run_id=?", run.ID).Take(&snapshotJob).Error; err != nil {
		t.Fatal(err)
	}
	if snapshotJob.Stage != reorgGenerationStageRequirements || snapshotJob.SenderCount != 1 || snapshotJob.SubmissionCount != 2 || snapshotJob.NonceCount != 1 {
		t.Fatalf("generation snapshot checkpoint invalid: %+v", snapshotJob)
	}
	var sourceIDs []uint64
	if err := database.Model(&entity.ChainEventReorgGenerationSubmission{}).Where("run_id=?", run.ID).Order("ordinal ASC").Pluck("source_submission_id", &sourceIDs).Error; err != nil {
		t.Fatal(err)
	}
	if len(sourceIDs) != 2 || sourceIDs[0] != direct.ID || sourceIDs[1] != other.ID {
		t.Fatalf("generation affected sender scope invalid: %v", sourceIDs)
	}
	assertGenerationFutureRowsRejected(t, database, run.ID)
	if err := processGenerationStage(run, first, firstToken); err == nil {
		t.Fatal("released generation snapshot lease wrote requirements")
	}

	secondToken := strings.Repeat("8", 64)
	second, err := claimGenerationLease(run.ID, secondToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := processGenerationStage(run, second, secondToken); err != nil {
		t.Fatal(err)
	}
	thirdToken := strings.Repeat("9", 64)
	third, err := claimGenerationLease(run.ID, thirdToken)
	if err != nil {
		t.Fatal(err)
	}
	assertGenerationTerminalBypassRejected(t, database, run.ID, thirdToken, third.FenceEpoch)
	if err := processGenerationStage(run, third, thirdToken); err != nil {
		t.Fatal(err)
	}
	completed := entity.ChainEventReorgGenerationJob{}
	if err := database.Where("run_id=?", run.ID).Take(&completed).Error; err != nil {
		t.Fatal(err)
	}
	if completed.State != reorgGenerationStateCompleted || completed.Stage != reorgGenerationStageDone || completed.RequirementCount != 5 || completed.UnmetCount != 4 || completed.VerifiedCount != 1 {
		t.Fatalf("generation terminal checkpoint invalid: %+v", completed)
	}
	manifest := entity.ChainEventReorgGenerationManifest{}
	if err := database.Where("run_id=?", run.ID).Take(&manifest).Error; err != nil {
		t.Fatal(err)
	}
	if manifest.Outcome != reorgGenerationOutcomeBlocked || manifest.CandidateReady || manifest.SubmissionCount != 2 || manifest.NonceCount != 1 || manifest.ManifestDigest != generationManifestDigest(manifest) {
		t.Fatalf("generation manifest invalid: %+v", manifest)
	}
	after := []entity.ChainTxSubmission{}
	if err := database.Order("id ASC").Find(&after).Error; err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Fatalf("generation worker changed current submission cardinality: %d -> %d", len(before), len(after))
	}
	for index := range before {
		if generationSubmissionSourceDigest(before[index]) != generationSubmissionSourceDigest(after[index]) {
			t.Fatalf("generation worker mutated current submission id=%d", before[index].ID)
		}
	}
	if _, _, err := ensureGenerationRunForIncident(&fixture.Incident); !errors.Is(err, errGenerationAlreadyComplete) {
		t.Fatalf("completed generation foundation did not remain idempotent: %v", err)
	}
}

func TestReorgGenerationResourceGateAgainstMySQL(t *testing.T) {
	database := openReorgWorkerVerificationMySQL(t)
	previous := jobDB.Client.Mysql
	jobDB.Client.Mysql = database
	t.Cleanup(func() { jobDB.Client.Mysql = previous })
	fixture := createAssessmentWorkerFixture(t, database, 1)
	completeAssessmentFixture(t, database, fixture.AssessmentRun)
	exercisePrerequisiteInventory(t, database, fixture.Incident)

	now := time.Now().Truncate(time.Microsecond)
	active := uint8(1)
	source := entity.ChainTxSubmission{ProjectID: 21, Action: "v2_create", AttemptNo: 1, ChainID: fixture.Incident.ChainID,
		SenderAddress: "0x9000000000000000000000000000000000000009", ContractAddress: fixture.Incident.ContractAddress,
		CallData: make([]byte, reorgGenerationMaxRowPayload+1), ValueRaw: "0", State: constant.ChainTxStateIntent,
		GasPriceRaw: "0", ActiveSlot: &active, LastError: "", ReceiptBlockHash: "", CreatedAt: now, UpdatedAt: now}
	if err := database.Create(&source).Error; err != nil {
		t.Fatal(err)
	}
	run, _, err := ensureGenerationRunForIncident(&fixture.Incident)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("e", 64)
	lease, err := claimGenerationLease(run.ID, token)
	if err != nil {
		t.Fatal(err)
	}
	err = processGenerationStage(run, lease, token)
	if err == nil || !strings.Contains(err.Error(), "row payload limit exceeded") {
		t.Fatalf("oversized generation source did not fail closed: %v", err)
	}
	for _, table := range []string{"chain_event_reorg_generation_sender", "chain_event_reorg_generation_submission", "chain_event_reorg_generation_nonce", "chain_event_reorg_generation_manifest"} {
		var count int64
		if err := database.Table(table).Where("run_id=?", run.ID).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("resource gate left partial %s rows=%d err=%v", table, count, err)
		}
	}
}

func TestReorgGenerationSourceDriftAgainstMySQL(t *testing.T) {
	database := openReorgWorkerVerificationMySQL(t)
	previous := jobDB.Client.Mysql
	jobDB.Client.Mysql = database
	t.Cleanup(func() { jobDB.Client.Mysql = previous })
	fixture := createAssessmentWorkerFixture(t, database, 1)
	completeAssessmentFixture(t, database, fixture.AssessmentRun)
	exercisePrerequisiteInventory(t, database, fixture.Incident)

	now := time.Now().Truncate(time.Microsecond)
	active := uint8(1)
	sender := "0x7000000000000000000000000000000000000007"
	source := entity.ChainTxSubmission{ProjectID: 11, Action: "v2_create", AttemptNo: 1, ChainID: fixture.Incident.ChainID,
		SenderAddress: sender, ContractAddress: fixture.Incident.ContractAddress, CallData: []byte{1}, ValueRaw: "0",
		State: constant.ChainTxStateIntent, GasPriceRaw: "0", ActiveSlot: &active, LastError: "", ReceiptBlockHash: "",
		CreatedAt: now, UpdatedAt: now}
	if err := database.Create(&source).Error; err != nil {
		t.Fatal(err)
	}
	run, _, err := ensureGenerationRunForIncident(&fixture.Incident)
	if err != nil {
		t.Fatal(err)
	}
	firstToken := strings.Repeat("a", 64)
	first, err := claimGenerationLease(run.ID, firstToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := processGenerationStage(run, first, firstToken); err != nil {
		t.Fatal(err)
	}
	drift := entity.ChainTxSubmission{ProjectID: 12, Action: "v2_activate", AttemptNo: 1, ChainID: fixture.Incident.ChainID,
		SenderAddress: sender, ContractAddress: "0x8000000000000000000000000000000000000008", CallData: []byte{2}, ValueRaw: "0",
		State: constant.ChainTxStateIntent, GasPriceRaw: "0", ActiveSlot: &active, LastError: "", ReceiptBlockHash: "",
		CreatedAt: now, UpdatedAt: now}
	if err := database.Create(&drift).Error; err != nil {
		t.Fatal(err)
	}
	secondToken := strings.Repeat("b", 64)
	second, err := claimGenerationLease(run.ID, secondToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := processGenerationStage(run, second, secondToken); err != nil {
		t.Fatal(err)
	}
	expiredToken := strings.Repeat("c", 64)
	previousExpiry := generationLeaseExpiryExpr
	generationLeaseExpiryExpr = func() clause.Expr {
		return gorm.Expr("DATE_ADD(CURRENT_TIMESTAMP(6),INTERVAL 100000 MICROSECOND)")
	}
	t.Cleanup(func() { generationLeaseExpiryExpr = previousExpiry })
	expired, err := claimGenerationLease(run.ID, expiredToken)
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(2 * time.Second); ; {
		var expiredByDB bool
		if err := database.Raw("SELECT lease_expires_at<=CURRENT_TIMESTAMP(6) FROM chain_event_reorg_generation_job WHERE run_id=?", run.ID).Scan(&expiredByDB).Error; err != nil {
			t.Fatal(err)
		}
		if expiredByDB {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("generation lease did not expire according to database clock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	takeoverToken := strings.Repeat("d", 64)
	takeover, err := claimGenerationLease(run.ID, takeoverToken)
	if err != nil {
		t.Fatal(err)
	}
	if takeover.FenceEpoch != expired.FenceEpoch+1 {
		t.Fatalf("generation takeover fence=%d want=%d", takeover.FenceEpoch, expired.FenceEpoch+1)
	}
	if err := processGenerationStage(run, expired, expiredToken); err == nil {
		t.Fatal("expired generation lease wrote after DB-clock takeover")
	}
	if err := processGenerationStage(run, takeover, takeoverToken); err != nil {
		t.Fatal(err)
	}
	invalidated := entity.ChainEventReorgGenerationJob{}
	if err := database.Where("run_id=?", run.ID).Take(&invalidated).Error; err != nil {
		t.Fatal(err)
	}
	if invalidated.State != reorgGenerationStateInvalidated || invalidated.LastErrorCode != "SOURCE_DRIFT" || invalidated.ActiveSlot != nil {
		t.Fatalf("source drift did not invalidate generation foundation: %+v", invalidated)
	}
	var manifests int64
	if err := database.Model(&entity.ChainEventReorgGenerationManifest{}).Where("run_id=?", run.ID).Count(&manifests).Error; err != nil || manifests != 0 {
		t.Fatalf("invalidated generation produced manifest: count=%d err=%v", manifests, err)
	}
	terminal := entity.ChainEventReorgGenerationEvent{}
	if err := database.Where("run_id=? AND sequence_no=2", run.ID).Take(&terminal).Error; err != nil {
		t.Fatal(err)
	}
	if terminal.EventType != "INVALIDATED" || terminal.Stage != "invalidate" || terminal.ErrorCode != "SOURCE_DRIFT" {
		t.Fatalf("invalidated generation terminal event invalid: %+v", terminal)
	}
	retry, retryJob, err := ensureGenerationRunForIncident(&fixture.Incident)
	if err != nil {
		t.Fatal(err)
	}
	if retry.AttemptNo != 2 || retry.ID == run.ID || retryJob.State != reorgGenerationStatePreparing || retryJob.Stage != reorgGenerationStageSnapshot {
		t.Fatalf("invalidated generation did not create contiguous retry: run=%+v job=%+v", retry, retryJob)
	}
}

func assertGenerationTerminalBypassRejected(t *testing.T, database *gorm.DB, runID uint64, token string, fence uint64) {
	t.Helper()
	tx := database.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	err := tx.Model(&entity.ChainEventReorgGenerationJob{}).
		Where(activeGenerationLeaseWhere, runID, token, fence).
		Updates(map[string]interface{}{"state": reorgGenerationStateCompleted, "stage": reorgGenerationStageDone,
			"active_slot": nil, "lease_token": "", "lease_expires_at": nil,
			"completed_at": gorm.Expr("CURRENT_TIMESTAMP(6)"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)")}).Error
	if err == nil || !strings.Contains(err.Error(), "terminal blocked evidence") {
		t.Fatalf("generation completion without terminal event and manifest was not rejected: %v", err)
	}
}

func assertGenerationFutureRowsRejected(t *testing.T, database *gorm.DB, runID uint64) {
	t.Helper()
	for table, values := range map[string]map[string]interface{}{
		"chain_event_reorg_candidate_project":          {"run_id": runID},
		"chain_event_reorg_candidate_ledger":           {"run_id": runID},
		"chain_event_reorg_candidate_v2_entitlement":   {"run_id": runID},
		"chain_event_reorg_candidate_v1_fact":          {"run_id": runID},
		"chain_event_reorg_candidate_wallet_aggregate": {"run_id": runID},
		"chain_event_reorg_candidate_delta":            {"run_id": runID},
		"chain_event_reorg_generation_checkpoint":      {"run_id": runID},
	} {
		t.Run("forbid_"+table, func(t *testing.T) {
			tx := database.Begin()
			if tx.Error != nil {
				t.Fatal(tx.Error)
			}
			defer tx.Rollback()
			err := tx.Table(table).Create(values).Error
			if err == nil || !strings.Contains(err.Error(), "B1-B0 forbids") {
				t.Fatalf("future generation table %s accepted insert: %v", table, err)
			}
		})
	}
}

func completeAssessmentFixture(t *testing.T, database *gorm.DB, run entity.ChainEventReorgAssessmentRun) {
	t.Helper()
	for step := 0; step < 10; step++ {
		current := entity.ChainEventReorgAssessmentJob{}
		if err := database.Where("run_id = ?", run.ID).Take(&current).Error; err != nil {
			t.Fatal(err)
		}
		if current.State == reorgAssessmentStateCompleted {
			return
		}
		token := fmt.Sprintf("%064x", step+100)
		lease, err := claimAssessmentLease(run.ID, token)
		if err != nil {
			t.Fatal(err)
		}
		if err := processAssessmentPage(context.Background(), &run, lease, token); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("assessment fixture did not complete within 10 stages")
}

func exercisePrerequisiteInventory(t *testing.T, database *gorm.DB, incident entity.ChainEventReorgIncident) *entity.ChainEventReorgPrerequisiteRun {
	t.Helper()
	run, job, err := ensurePrerequisiteRunForIncident(&incident)
	if err != nil {
		t.Fatal(err)
	}
	resumedRun, resumedJob, err := ensurePrerequisiteRunForIncident(&incident)
	if err != nil || resumedRun.ID != run.ID || resumedJob.RunID != job.RunID {
		t.Fatalf("production prerequisite creation was not idempotent: run=%+v job=%+v err=%v", resumedRun, resumedJob, err)
	}

	firstToken := strings.Repeat("c", 64)
	first, err := claimPrerequisiteLease(run.ID, firstToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := processPrerequisiteStage(run, first, firstToken); err != nil {
		t.Fatal(err)
	}
	assertPrerequisiteCheckpoint(t, database, run.ID, reorgPrerequisiteStatePreparing, 7, 5, 2, 8)
	if err := processPrerequisiteStage(run, first, firstToken); err == nil {
		t.Fatal("released prerequisite lease wrote a later stage")
	}

	secondToken := strings.Repeat("d", 64)
	second, err := claimPrerequisiteLease(run.ID, secondToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&entity.ChainEventReorgPrerequisiteArtifact{RunID: run.ID, Ordinal: 1,
		RequirementCode: prerequisiteRequirements[0].Code, ArtifactKind: "boundary_snapshot", SchemaVersion: "v1",
		SourceKind: "database_snapshot", SourcePayload: []byte("untrusted current projection"),
		SourceDigest: sha256Hex([]byte("untrusted current projection")), SafeSummary: "must never be accepted in B1-A v1",
		PreviousRowDigest: reorgZeroDigest, RowDigest: strings.Repeat("e", 64), CreatedAt: time.Now()}).Error; err == nil || !strings.Contains(err.Error(), "B1-A v1 forbids prerequisite artifacts") {
		t.Fatalf("prerequisite artifact was not physically rejected: %v", err)
	}
	if err := recordPrerequisiteError(run.ID, secondToken, second.FenceEpoch, "PREREQUISITE_INVENTORY_FAILED", errors.New("transient prerequisite worker failure")); err != nil {
		t.Fatal(err)
	}
	errorCheckpoint := entity.ChainEventReorgPrerequisiteJob{}
	if err := database.Where("run_id = ?", run.ID).Take(&errorCheckpoint).Error; err != nil {
		t.Fatal(err)
	}
	if errorCheckpoint.LastErrorCode != "PREREQUISITE_INVENTORY_FAILED" || errorCheckpoint.LeaseToken != "" || errorCheckpoint.LeaseExpiresAt != nil {
		t.Fatalf("prerequisite transient error checkpoint invalid: %+v", errorCheckpoint)
	}
	recoveryToken := strings.Repeat("f", 64)
	recoveryLease, err := claimPrerequisiteLease(run.ID, recoveryToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&entity.ChainEventReorgPrerequisiteJob{}).
		Where("run_id = ? AND lease_token = ? AND fence_epoch = ?", run.ID, recoveryToken, recoveryLease.FenceEpoch).
		Update("lease_expires_at", gorm.Expr("DATE_SUB(CURRENT_TIMESTAMP(6), INTERVAL 1 SECOND)")).Error; err != nil {
		t.Fatal(err)
	}
	thirdToken := strings.Repeat("e", 64)
	third, err := claimPrerequisiteLease(run.ID, thirdToken)
	if err != nil {
		t.Fatal(err)
	}
	if third.FenceEpoch != recoveryLease.FenceEpoch+1 {
		t.Fatalf("prerequisite takeover fence=%d want=%d", third.FenceEpoch, recoveryLease.FenceEpoch+1)
	}
	if err := processPrerequisiteStage(run, recoveryLease, recoveryToken); err == nil {
		t.Fatal("expired prerequisite lease wrote after DB-clock takeover")
	}
	assertPrerequisiteTerminalBypassesRejected(t, database, *run, third, thirdToken)
	if err := processPrerequisiteStage(run, third, thirdToken); err != nil {
		t.Fatal(err)
	}
	assertPrerequisiteCheckpoint(t, database, run.ID, reorgPrerequisiteStateCompleted, 7, 5, 2, 8)
	before := prerequisiteEvidenceCardinality(t, database, run.ID)
	if err := processPrerequisiteStage(run, third, thirdToken); err == nil {
		t.Fatal("completed prerequisite inventory accepted authentic final-lease replay")
	}
	if after := prerequisiteEvidenceCardinality(t, database, run.ID); after != before {
		t.Fatalf("prerequisite replay changed evidence: before=%+v after=%+v", before, after)
	}
	assertPrerequisiteEvidence(t, database, *run)
	return run
}

func assertPrerequisiteTerminalBypassesRejected(t *testing.T, database *gorm.DB, run entity.ChainEventReorgPrerequisiteRun, lease *entity.ChainEventReorgPrerequisiteJob, token string) {
	t.Helper()
	before := prerequisiteEvidenceCardinality(t, database, run.ID)
	tx := database.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	err := tx.Model(&entity.ChainEventReorgPrerequisiteJob{}).Where(activePrerequisiteLeaseWhere, run.ID, token, lease.FenceEpoch).Updates(map[string]interface{}{
		"state": reorgPrerequisiteStateCompleted, "active_slot": nil, "lease_token": "", "lease_expires_at": nil,
		"completed_at": gorm.Expr("CURRENT_TIMESTAMP(6)"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)"),
	}).Error
	if err == nil || !strings.Contains(err.Error(), "terminal evidence and blocked manifest") {
		t.Fatalf("job completion without terminal evidence was not rejected: %v", err)
	}
	err = tx.Model(&entity.ChainEventReorgPrerequisiteJob{}).Where(activePrerequisiteLeaseWhere, run.ID, token, lease.FenceEpoch).Updates(map[string]interface{}{
		"state": reorgPrerequisiteStateInvalidated, "active_slot": nil, "lease_token": "", "lease_expires_at": nil,
		"last_error_code": "FORGED_INVALIDATION", "last_error_message": "forged invalidation without terminal evidence",
		"completed_at": gorm.Expr("CURRENT_TIMESTAMP(6)"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)"),
	}).Error
	if err == nil || !strings.Contains(err.Error(), "matching terminal evidence") {
		t.Fatalf("job invalidation without terminal evidence was not rejected: %v", err)
	}
	created := entity.ChainEventReorgPrerequisiteEvent{}
	if err := tx.Where("run_id = ? AND sequence_no = 1", run.ID).Take(&created).Error; err != nil {
		t.Fatal(err)
	}
	duplicateCreated := entity.ChainEventReorgPrerequisiteEvent{RunID: run.ID, SequenceNo: 2, EventType: "CREATED",
		Message: "forged duplicate prerequisite creation", PreviousEventHash: created.EventHash, CreatedAt: time.Now()}
	duplicateCreated.EventHash = prerequisiteEventDigest(duplicateCreated)
	if err := tx.Create(&duplicateCreated).Error; err == nil || !strings.Contains(err.Error(), "event chain is not contiguous") {
		t.Fatalf("duplicate CREATED event was not rejected: %v", err)
	}
	forged := entity.ChainEventReorgPrerequisiteManifest{RunID: run.ID, IncidentID: run.IncidentID,
		AssessmentManifestDigest: run.AssessmentManifestDigest, Outcome: reorgPrerequisiteOutcomeBlocked, CandidateReady: false,
		RequirementCount: 7, SatisfiedCount: 0, UnmetCount: 5, UntrustedCount: 0, DeferredCount: 2, ArtifactCount: 0,
		RequirementDigest: lease.RequirementDigest, ArtifactDigest: reorgZeroDigest, EventDigest: created.EventHash, CompletedAt: time.Now()}
	forged.ManifestDigest = prerequisiteManifestDigest(forged)
	if err := tx.Create(&forged).Error; err == nil || !strings.Contains(err.Error(), "evidence sets are incomplete") {
		t.Fatalf("manifest with non-terminal event root was not rejected: %v", err)
	}
	if err := tx.Rollback().Error; err != nil {
		t.Fatal(err)
	}
	if after := prerequisiteEvidenceCardinality(t, database, run.ID); after != before {
		t.Fatalf("terminal bypass attempts changed evidence: before=%+v after=%+v", before, after)
	}
}

func assertPrerequisiteCheckpoint(t *testing.T, database *gorm.DB, runID uint64, state string, requirements, unmet, deferred, next uint64) {
	t.Helper()
	job := entity.ChainEventReorgPrerequisiteJob{}
	if err := database.Where("run_id = ?", runID).Take(&job).Error; err != nil {
		t.Fatal(err)
	}
	if job.State != state || job.RequirementCount != requirements || job.UnmetCount != unmet || job.UntrustedCount != 0 ||
		job.DeferredCount != deferred || job.ArtifactCount != 0 || job.ArtifactDigest != reorgZeroDigest || job.NextRequirementOrdinal != next ||
		job.LastErrorCode != "" || job.LastErrorMessage != "" {
		t.Fatalf("prerequisite checkpoint drifted: %+v", job)
	}
}

type prerequisiteEvidenceCounts struct {
	Requirements int64
	Artifacts    int64
	Events       int64
	Manifests    int64
}

func prerequisiteEvidenceCardinality(t *testing.T, database *gorm.DB, runID uint64) prerequisiteEvidenceCounts {
	t.Helper()
	counts := prerequisiteEvidenceCounts{}
	for table, destination := range map[string]*int64{
		"chain_event_reorg_prerequisite_requirement": &counts.Requirements,
		"chain_event_reorg_prerequisite_artifact":    &counts.Artifacts,
		"chain_event_reorg_prerequisite_event":       &counts.Events,
		"chain_event_reorg_prerequisite_manifest":    &counts.Manifests,
	} {
		if err := database.Table(table).Where("run_id = ?", runID).Count(destination).Error; err != nil {
			t.Fatal(err)
		}
	}
	return counts
}

func assertPrerequisiteEvidence(t *testing.T, database *gorm.DB, run entity.ChainEventReorgPrerequisiteRun) {
	t.Helper()
	requirements := make([]entity.ChainEventReorgPrerequisiteRequirement, 0, 7)
	if err := database.Where("run_id = ?", run.ID).Order("ordinal ASC").Find(&requirements).Error; err != nil {
		t.Fatal(err)
	}
	if len(requirements) != 7 {
		t.Fatalf("prerequisite requirement count=%d want=7", len(requirements))
	}
	previous := reorgZeroDigest
	for index, row := range requirements {
		want := prerequisiteRequirements[index]
		if row.Ordinal != uint64(index+1) || row.RequirementCode != want.Code || row.EvidenceStatus != want.Status ||
			row.ArtifactID != nil || row.PreviousRowDigest != previous || row.RowDigest != prerequisiteRequirementDigest(row) {
			t.Fatalf("prerequisite requirement chain invalid at %d: %+v", index, row)
		}
		previous = row.RowDigest
	}
	events := make([]entity.ChainEventReorgPrerequisiteEvent, 0, 2)
	if err := database.Where("run_id = ?", run.ID).Order("sequence_no ASC").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	eventPrevious := reorgZeroDigest
	for index, row := range events {
		if row.SequenceNo != uint(index+1) || row.PreviousEventHash != eventPrevious || row.EventHash != prerequisiteEventDigest(row) {
			t.Fatalf("prerequisite event chain invalid at %d: %+v", index, row)
		}
		eventPrevious = row.EventHash
	}
	if len(events) != 2 || events[0].EventType != "CREATED" || events[1].EventType != "COMPLETED" {
		t.Fatalf("prerequisite terminal event set invalid: %+v", events)
	}
	manifest := entity.ChainEventReorgPrerequisiteManifest{}
	if err := database.Where("run_id = ?", run.ID).Take(&manifest).Error; err != nil {
		t.Fatal(err)
	}
	if manifest.Outcome != reorgPrerequisiteOutcomeBlocked || manifest.CandidateReady || manifest.RequirementCount != 7 ||
		manifest.SatisfiedCount != 0 || manifest.UnmetCount != 5 || manifest.UntrustedCount != 0 || manifest.DeferredCount != 2 ||
		manifest.ArtifactCount != 0 || manifest.ArtifactDigest != reorgZeroDigest || manifest.RequirementDigest != previous ||
		manifest.AssessmentManifestDigest != run.AssessmentManifestDigest || manifest.EventDigest != events[1].EventHash ||
		manifest.ManifestDigest != prerequisiteManifestDigest(manifest) {
		t.Fatalf("prerequisite manifest invalid: %+v", manifest)
	}
}

func assertAssessmentPageRollback(t *testing.T, database *gorm.DB, run entity.ChainEventReorgAssessmentRun, lease *entity.ChainEventReorgAssessmentJob, token string) {
	t.Helper()
	effects := make([]entity.ChainEventReorgMaterializationEffect, 0, 2)
	if err := database.Where("run_id = ? AND ordinal >= ?", run.MaterializationRunID, lease.NextEffectOrdinal).Order("ordinal ASC").Limit(2).Find(&effects).Error; err != nil {
		t.Fatal(err)
	}
	if len(effects) != 2 {
		t.Fatalf("partial-page rollback fixture effects=%d want=2", len(effects))
	}
	prepared, err := prepareAssessmentDispositions(effects)
	if err != nil {
		t.Fatal(err)
	}
	prepared[1].Payload = []byte(`{"schema_version":"reorg-assessment-plan-v1"}`)
	if err := commitAssessmentDispositions(&run, lease, token, prepared); err == nil || !strings.Contains(err.Error(), "assessment disposition plan payload identity mismatch") {
		t.Fatalf("partial assessment page was not rejected: %v", err)
	}
	assertAssessmentCheckpoint(t, database, run.ID, 0, reorgAssessmentGlobalBlockerCount, 1, reorgAssessmentStatePreparing)
	cardinality := assessmentEvidenceCardinality(t, database, run.ID)
	if cardinality.Dispositions != 0 || cardinality.Blockers != reorgAssessmentGlobalBlockerCount || cardinality.Events != 1 || cardinality.Manifests != 0 {
		t.Fatalf("failed assessment page left partial evidence: %+v", cardinality)
	}
}

type assessmentEvidenceCounts struct {
	Dispositions int64
	Blockers     int64
	Events       int64
	Manifests    int64
}

func assessmentEvidenceCardinality(t *testing.T, database *gorm.DB, runID uint64) assessmentEvidenceCounts {
	t.Helper()
	counts := assessmentEvidenceCounts{}
	for table, destination := range map[string]*int64{
		"chain_event_reorg_assessment_disposition": &counts.Dispositions,
		"chain_event_reorg_assessment_blocker":     &counts.Blockers,
		"chain_event_reorg_assessment_event":       &counts.Events,
		"chain_event_reorg_assessment_manifest":    &counts.Manifests,
	} {
		if err := database.Table(table).Where("run_id = ?", runID).Count(destination).Error; err != nil {
			t.Fatal(err)
		}
	}
	return counts
}

func assertInvalidAssessmentPlansRejected(t *testing.T, database *gorm.DB, run entity.ChainEventReorgAssessmentRun, lease *entity.ChainEventReorgAssessmentJob) {
	t.Helper()
	effect := entity.ChainEventReorgMaterializationEffect{}
	if err := database.Where("run_id = ? AND ordinal = ?", run.MaterializationRunID, lease.NextEffectOrdinal).Take(&effect).Error; err != nil {
		t.Fatal(err)
	}
	for name, payload := range map[string][]byte{
		"missing required keys": []byte(`{"schema_version":"reorg-assessment-plan-v1"}`),
		"wrong project type":    []byte(`{"schema_version":"reorg-assessment-plan-v1","source_effect_digest":"x","branch":"replacement","contract_version":"v2_stablecoin","event_type":"ignored","project_id":"0","replayability":"noop","action_kind":"apply_approved_abi_noop"}`),
		"invalid utf8":          {0x80, 0x80},
	} {
		t.Run(name, func(t *testing.T) {
			row := entity.ChainEventReorgAssessmentDisposition{RunID: run.ID, Ordinal: lease.NextEffectOrdinal, MaterializationEffectID: effect.ID, SourceEffectDigest: effect.EffectDigest, Branch: effect.Branch, ContractVersion: effect.ContractVersion, EventType: effect.EventType, ProjectID: effect.ProjectID, Replayability: reorgAssessmentReplayNoop, ActionKind: "apply_approved_abi_noop", PlanPayload: payload, PlanPayloadDigest: sha256Hex(payload), PreviousRowDigest: lease.DispositionDigest, CreatedAt: time.Now()}
			row.RowDigest = assessmentDispositionDigest(row)
			err := database.Create(&row).Error
			if err == nil || !strings.Contains(err.Error(), "assessment disposition plan payload identity mismatch") {
				t.Fatalf("invalid plan was not rejected by JSON envelope: %v", err)
			}
			var count int64
			if err := database.Model(&entity.ChainEventReorgAssessmentDisposition{}).Where("run_id = ?", run.ID).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("invalid plan left %d dispositions", count)
			}
		})
	}
}

type assessmentWorkerFixture struct {
	Incident                entity.ChainEventReorgIncident
	AssessmentRun           entity.ChainEventReorgAssessmentRun
	MaterializationManifest entity.ChainEventReorgMaterializationManifest
}

func openReorgWorkerVerificationMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("JOINCARE_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("JOINCARE_MYSQL_TEST_DSN is not set")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := database.Raw("SELECT DATABASE()").Scan(&name).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(name, "joincare_reorg_verify_assessment_worker_") {
		t.Fatalf("refuse non-disposable assessment worker database %q", name)
	}
	return database
}

func createAssessmentWorkerFixture(t *testing.T, database *gorm.DB, logCount int) assessmentWorkerFixture {
	t.Helper()
	for _, table := range []string{"chain_event_reorg_incident", "chain_event_reorg_recovery_run", "chain_event_reorg_materialization_run", "chain_event_reorg_assessment_run"} {
		var count int64
		if err := database.Table(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("assessment worker fixture requires empty %s", table)
		}
	}

	chainID := uint64(31337)
	v1Address := strings.ToLower(common.HexToAddress("0x1000000000000000000000000000000000000001").Hex())
	v2Address := strings.ToLower(common.HexToAddress("0x2000000000000000000000000000000000000002").Hex())
	boundaryBlock, cursorBlock, targetBlock := uint64(199), uint64(200), uint64(201)
	boundaryHash := common.HexToHash("0x199").Hex()
	cursorHash := common.HexToHash("0x200").Hex()
	observedHash := common.HexToHash("0x201").Hex()
	blockHeader := &types.Header{Number: new(big.Int).SetUint64(cursorBlock), ParentHash: common.HexToHash(boundaryHash), Time: cursorBlock}
	targetHeader := &types.Header{Number: new(big.Int).SetUint64(targetBlock), ParentHash: blockHeader.Hash(), Time: targetBlock}
	targetHash := strings.ToLower(targetHeader.Hash().Hex())
	now := time.Now()

	if err := database.Model(&entity.ChainEventSyncState{}).Where("id = 1").Updates(map[string]interface{}{
		"initialized": true, "cursor_block": cursorBlock, "cursor_block_hash": strings.ToLower(cursorHash),
		"v2_registered": true, "v2_contract_address": v2Address, "v2_start_block": uint64(1),
		"lease_token": "", "updated_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	boundaryCheckpoint := entity.ChainEventBlockCheckpoint{ChainID: chainID, BlockNumber: boundaryBlock, BlockHash: strings.ToLower(boundaryHash), PreviousCheckpointHash: reorgZeroDigest, CreatedAt: now}
	boundaryCheckpoint.CheckpointHash = checkpointDigest(chainID, boundaryBlock, boundaryCheckpoint.BlockHash, reorgZeroDigest)
	if err := database.Create(&boundaryCheckpoint).Error; err != nil {
		t.Fatal(err)
	}
	cursorCheckpoint := entity.ChainEventBlockCheckpoint{ChainID: chainID, BlockNumber: cursorBlock, BlockHash: strings.ToLower(cursorHash), PreviousCheckpointHash: boundaryCheckpoint.CheckpointHash, CreatedAt: now}
	cursorCheckpoint.CheckpointHash = checkpointDigest(chainID, cursorBlock, cursorCheckpoint.BlockHash, cursorCheckpoint.PreviousCheckpointHash)
	if err := database.Create(&cursorCheckpoint).Error; err != nil {
		t.Fatal(err)
	}
	active := uint8(1)
	incident := entity.ChainEventReorgIncident{IncidentKey: reorgIncidentDigest(chainID, v2Address, cursorBlock, cursorHash, observedHash), ChainID: chainID, ContractAddress: v2Address, ContractVersion: constant.ContractVersionV2Stablecoin, CursorBlock: cursorBlock, ExpectedBlockHash: strings.ToLower(cursorHash), ObservedBlockHash: strings.ToLower(observedHash), CursorCheckpointID: cursorCheckpoint.ID, V2StartBlock: 1, Status: reorgStatusDetected, ActiveSlot: &active, DetectionCode: reorgDetectionCode, DetectedAt: now, CreatedAt: now, UpdatedAt: now}
	if err := database.Create(&incident).Error; err != nil {
		t.Fatal(err)
	}
	incidentMessage := "created disposable assessment worker reorg fixture"
	incidentEvent := entity.ChainEventReorgEvent{IncidentID: incident.ID, SequenceNo: 1, EventType: reorgEventDetected, Message: incidentMessage, PreviousEventHash: reorgZeroDigest, CreatedAt: now}
	incidentEvent.EventHash = reorgEventDigest(incident.ID, 1, reorgEventDetected, incidentMessage, reorgZeroDigest)
	if err := database.Create(&incidentEvent).Error; err != nil {
		t.Fatal(err)
	}

	boundaryID := boundaryCheckpoint.ID
	recoveryRun := entity.ChainEventReorgRecoveryRun{RunUUID: strings.ToLower(uuid.NewString()), IncidentID: incident.ID, AttemptNo: 1, ChainID: chainID, ContractAddress: v2Address, LegacyContractAddress: v1Address, LegacySourceRegistered: true, ContractVersion: constant.ContractVersionV2Stablecoin, AlgorithmVersion: reorgRecoveryAlgorithm, BoundaryKind: reorgRecoveryBoundaryLast, BoundaryCheckpointID: &boundaryID, BoundaryBlock: boundaryBlock, BoundaryBlockHash: strings.ToLower(boundaryHash), TargetSafeBlock: targetBlock, TargetSafeBlockHash: targetHash, ExpectedBatchCount: 1, ExpectedScannedBlocks: 2, RequestedAt: now}
	if err := database.Create(&recoveryRun).Error; err != nil {
		t.Fatal(err)
	}
	recoveryJob := entity.ChainEventReorgRecoveryJob{RunID: recoveryRun.ID, IncidentID: incident.ID, State: reorgRecoveryStateScan, ActiveSlot: &active, NextBlock: cursorBlock, NextBatchOrdinal: 1, NextLogOrdinal: 1, BatchEvidenceDigest: reorgZeroDigest, LogEvidenceDigest: reorgZeroDigest, UpdatedAt: now}
	if err := database.Create(&recoveryJob).Error; err != nil {
		t.Fatal(err)
	}
	if err := appendRecoveryEventTx(database, recoveryRun.ID, "CREATED", "boundary", "", "created disposable fixed-range recovery fixture"); err != nil {
		t.Fatal(err)
	}
	recoveryToken := strings.Repeat("a", 64)
	recoveryLease, err := claimRecoveryLease(recoveryRun.ID, recoveryToken)
	if err != nil {
		t.Fatal(err)
	}
	blockHash := blockHeader.Hash()
	logs := make([]types.Log, 0, logCount)
	for index := 0; index < logCount; index++ {
		topics := []common.Hash{crypto.Keccak256Hash([]byte("OwnershipTransferred(address,address)"))}
		data := []byte(nil)
		if index == reorgAssessmentPageSize-1 {
			token := common.HexToAddress("0x3000000000000000000000000000000000000003")
			topics = []common.Hash{crypto.Keccak256Hash([]byte("PaymentTokenConfigured(address,bool,uint8)")), common.BytesToHash(common.LeftPadBytes(token.Bytes(), 32))}
			data = append(common.LeftPadBytes([]byte{1}, 32), common.LeftPadBytes([]byte{6}, 32)...)
		}
		logs = append(logs, types.Log{Address: common.HexToAddress(v2Address), Topics: topics, Data: data, BlockNumber: cursorBlock, BlockHash: blockHash, TxHash: common.BigToHash(common.Big1), TxIndex: 0, Index: uint(index)})
	}
	if err := commitRecoveryBatch(&incident, recoveryRun, recoveryLease, recoveryToken, map[uint64]*types.Header{cursorBlock: blockHeader, targetBlock: targetHeader}, logs, cursorBlock, targetBlock); err != nil {
		t.Fatal(err)
	}
	recoveryFinalToken := strings.Repeat("b", 64)
	recoveryFinalLease, err := claimRecoveryLease(recoveryRun.ID, recoveryFinalToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := finalizeRecoveryFixture(database, incident, recoveryRun, recoveryFinalLease, recoveryFinalToken); err != nil {
		t.Fatal(err)
	}
	recoveryManifest := entity.ChainEventReorgRecoveryManifest{}
	if err := database.Where("run_id = ?", recoveryRun.ID).Take(&recoveryManifest).Error; err != nil {
		t.Fatal(err)
	}

	materializationRun := entity.ChainEventReorgMaterializationRun{RunUUID: strings.ToLower(uuid.NewString()), RecoveryRunID: recoveryRun.ID, IncidentID: incident.ID, AlgorithmVersion: reorgMaterializationAlgorithm, RecoveryManifestDigest: recoveryManifest.ManifestDigest, ChainID: chainID, ContractAddress: v2Address, LegacyContractAddress: v1Address, LegacySourceRegistered: true, BoundaryBlock: boundaryBlock, BoundaryBlockHash: strings.ToLower(boundaryHash), OldCursorBlock: cursorBlock, OldCursorBlockHash: strings.ToLower(cursorHash), TargetSafeBlock: targetBlock, TargetSafeBlockHash: targetHash, ExpectedOldRecordCount: 0, OldRecordSourceDigest: reorgZeroDigest, ExpectedReplacementLogCount: uint64(logCount), ReplacementLogSourceDigest: recoveryManifest.LogEvidenceDigest, CreatedAt: now}
	if err := database.Create(&materializationRun).Error; err != nil {
		t.Fatal(err)
	}
	materializationJob := entity.ChainEventReorgMaterializationJob{RunID: materializationRun.ID, IncidentID: incident.ID, State: reorgMaterializationStatePreparing, ActiveSlot: &active, NextBranch: reorgMaterializationBranchOld, NextOldBlock: cursorBlock, NextReplacementOrdinal: 1, NextEffectOrdinal: 1, NextBranchOrdinal: 1, OldEffectDigest: reorgZeroDigest, ReplacementEffectDigest: reorgZeroDigest, UpdatedAt: now}
	if err := database.Create(&materializationJob).Error; err != nil {
		t.Fatal(err)
	}
	if err := appendMaterializationEventTx(database, materializationRun.ID, "CREATED", "old_branch", "", "created disposable materialization worker fixture"); err != nil {
		t.Fatal(err)
	}
	materializationCompleted := false
	for step := 0; step < 10; step++ {
		token := fmt.Sprintf("%064x", step+12)
		lease, err := claimMaterializationLease(materializationRun.ID, token)
		if err != nil {
			t.Fatal(err)
		}
		if err := processMaterializationPage(context.Background(), &materializationRun, lease, token); err != nil {
			t.Fatal(err)
		}
		current := entity.ChainEventReorgMaterializationJob{}
		if err := database.Where("run_id = ?", materializationRun.ID).Take(&current).Error; err != nil {
			t.Fatal(err)
		}
		if current.State == reorgMaterializationStateCompleted {
			materializationCompleted = true
			break
		}
	}
	if !materializationCompleted {
		t.Fatal("materialization fixture did not complete within 10 stages")
	}
	materializationManifest := entity.ChainEventReorgMaterializationManifest{}
	if err := database.Where("run_id = ?", materializationRun.ID).Take(&materializationManifest).Error; err != nil {
		t.Fatal(err)
	}

	assessmentRun, assessmentJob, err := ensureAssessmentRunForIncident(&incident)
	if err != nil {
		t.Fatal(err)
	}
	resumedRun, resumedJob, err := ensureAssessmentRunForIncident(&incident)
	if err != nil {
		t.Fatal(err)
	}
	if resumedRun.ID != assessmentRun.ID || resumedJob.RunID != assessmentJob.RunID {
		t.Fatal("production assessment creation was not idempotent")
	}
	return assessmentWorkerFixture{Incident: incident, AssessmentRun: *assessmentRun, MaterializationManifest: materializationManifest}
}

func finalizeRecoveryFixture(database *gorm.DB, incident entity.ChainEventReorgIncident, run entity.ChainEventReorgRecoveryRun, job *entity.ChainEventReorgRecoveryJob, token string) error {
	return database.Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		locked, err := lockRecoveryJobTx(tx, run.ID, token, job.FenceEpoch)
		if err != nil {
			return err
		}
		manifest := entity.ChainEventReorgRecoveryManifest{RunID: run.ID, IncidentID: incident.ID, BatchCount: run.ExpectedBatchCount, LogCount: locked.ObservedLogCount, BatchEvidenceDigest: locked.BatchEvidenceDigest, LogEvidenceDigest: locked.LogEvidenceDigest, CompletedAt: time.Now()}
		manifest.ManifestDigest = recoveryManifestDigest(run.ID, incident.ID, uint64(run.ExpectedBatchCount), manifest.LogCount, manifest.BatchEvidenceDigest, manifest.LogEvidenceDigest)
		if err := tx.Create(&manifest).Error; err != nil {
			return err
		}
		if err := appendRecoveryEventTx(tx, run.ID, "COMPLETED", "finalize", "", "completed disposable recovery fixture"); err != nil {
			return err
		}
		now := time.Now()
		result := tx.Model(&entity.ChainEventReorgRecoveryJob{}).Where(activeRecoveryLeaseWhere, run.ID, token, job.FenceEpoch).Updates(map[string]interface{}{"state": reorgRecoveryStateDone, "active_slot": nil, "lease_token": "", "lease_expires_at": nil, "completed_at": now, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("fixture recovery lease lost")
		}
		return nil
	})
}

func assertAssessmentCheckpoint(t *testing.T, database *gorm.DB, runID, dispositions, blockers, next uint64, state string) {
	t.Helper()
	job := entity.ChainEventReorgAssessmentJob{}
	if err := database.Where("run_id = ?", runID).Take(&job).Error; err != nil {
		t.Fatal(err)
	}
	wantConditional := uint64(0)
	wantNoop := dispositions
	if dispositions != 0 {
		wantConditional, wantNoop = 1, dispositions-1
	}
	if job.State != state || job.DispositionCount != dispositions || job.NoopCount != wantNoop || job.BlockerCount != blockers || job.NextEffectOrdinal != next || job.ExactCount != 0 || job.ConditionalCount != wantConditional || job.BlockedCount != 0 {
		t.Fatalf("assessment checkpoint drifted: %+v", job)
	}
}

func assertAssessmentEvidence(t *testing.T, database *gorm.DB, run entity.ChainEventReorgAssessmentRun, source entity.ChainEventReorgMaterializationManifest) {
	t.Helper()
	dispositions := make([]entity.ChainEventReorgAssessmentDisposition, 0, run.ExpectedEffectCount)
	if err := database.Where("run_id = ?", run.ID).Order("ordinal ASC").Find(&dispositions).Error; err != nil {
		t.Fatal(err)
	}
	if uint64(len(dispositions)) != run.ExpectedEffectCount {
		t.Fatalf("disposition count=%d want=%d", len(dispositions), run.ExpectedEffectCount)
	}
	previous := reorgZeroDigest
	for index, row := range dispositions {
		wantReplayability := reorgAssessmentReplayNoop
		if index == reorgAssessmentPageSize-1 {
			wantReplayability = reorgAssessmentReplayConditional
		}
		plan := map[string]interface{}{}
		if err := json.Unmarshal(row.PlanPayload, &plan); err != nil {
			t.Fatalf("disposition plan JSON invalid at %d: %v", index, err)
		}
		wantKeys := 8
		if wantReplayability == reorgAssessmentReplayConditional {
			wantKeys = 9
		}
		if row.Ordinal != uint64(index+1) || row.PreviousRowDigest != previous || row.RowDigest != assessmentDispositionDigest(row) || row.Replayability != wantReplayability || len(plan) != wantKeys {
			t.Fatalf("disposition chain invalid at %d: %+v", index, row)
		}
		previous = row.RowDigest
	}
	blockers := make([]entity.ChainEventReorgAssessmentBlocker, 0, reorgAssessmentGlobalBlockerCount)
	if err := database.Where("run_id = ?", run.ID).Order("ordinal ASC").Find(&blockers).Error; err != nil {
		t.Fatal(err)
	}
	if len(blockers) != reorgAssessmentGlobalBlockerCount+1 {
		t.Fatalf("blocker count=%d want=%d", len(blockers), reorgAssessmentGlobalBlockerCount+1)
	}
	blockerPrevious := reorgZeroDigest
	for index, row := range blockers {
		validIdentity := row.Scope == "effect" && index == reorgAssessmentGlobalBlockerCount && row.BlockerCode == "GLOBAL_CONFIG_BASELINE_REQUIRED" && row.DispositionID != nil
		if index < reorgAssessmentGlobalBlockerCount {
			validIdentity = row.Scope == "run" && row.BlockerCode == assessmentRequiredBlockers[index].Code && row.DispositionID == nil
		}
		if row.Ordinal != uint64(index+1) || row.PreviousRowDigest != blockerPrevious || row.RowDigest != assessmentBlockerDigest(row) || !validIdentity {
			t.Fatalf("blocker chain invalid at %d: %+v", index, row)
		}
		blockerPrevious = row.RowDigest
	}
	events := make([]entity.ChainEventReorgAssessmentEvent, 0, 2)
	if err := database.Where("run_id = ?", run.ID).Order("sequence_no ASC").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	eventPrevious := reorgZeroDigest
	for index, row := range events {
		if row.SequenceNo != uint(index+1) || row.PreviousEventHash != eventPrevious || row.EventHash != assessmentEventDigest(row) {
			t.Fatalf("event chain invalid at %d: %+v", index, row)
		}
		eventPrevious = row.EventHash
	}
	if len(events) != 2 || events[0].EventType != "CREATED" || events[1].EventType != "COMPLETED" {
		t.Fatalf("assessment terminal event set invalid: %+v", events)
	}
	manifest := entity.ChainEventReorgAssessmentManifest{}
	if err := database.Where("run_id = ?", run.ID).Take(&manifest).Error; err != nil {
		t.Fatal(err)
	}
	if manifest.Outcome != reorgAssessmentOutcomeBlocked || manifest.DispositionCount != run.ExpectedEffectCount || manifest.ConditionalCount != 1 || manifest.NoopCount != run.ExpectedEffectCount-1 || manifest.BlockerCount != reorgAssessmentGlobalBlockerCount+1 || manifest.DispositionDigest != previous || manifest.BlockerDigest != blockerPrevious || manifest.MaterializationManifestDigest != source.ManifestDigest || manifest.ManifestDigest != assessmentManifestDigest(manifest) {
		t.Fatalf("assessment manifest invalid: %+v", manifest)
	}
	encoded, err := json.Marshal(manifest)
	if err != nil || strings.Contains(string(encoded), "switch_authorized") {
		t.Fatalf("assessment manifest projection unexpectedly authorizes switch: %s err=%v", encoded, err)
	}
}
