package chainevent

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"joincare_server/pkg/common/entity"
)

func TestArchiveFairLaunchMainnetBatchTransactionCommit(t *testing.T) {
	fixture := newFairLaunchMainnetArchiveSQLFixture(t)
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	expectFairLaunchMainnetArchiveWrites(t, mock, fixture)
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_archive_batch_event_v1`").WillReturnResult(sqlmock.NewResult(31, 1))
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_archive_event_payload_v1`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `fair_launch_mainnet_archive_cursor_v1`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var result FairLaunchMainnetArchiveBatchResult
	err := database.Transaction(func(tx *gorm.DB) error {
		var inner error
		result, inner = archiveFairLaunchMainnetBatchTx(fixture.capability, tx, fixture.batch, fixture.now)
		return inner
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.BatchReceiptID != 30 || result.FromBlock != 100 || result.ThroughBlock != 100 || result.EventCount != 1 || result.Replayed || len(result.Events) != 1 || result.Events[0].seal == nil {
		t.Fatalf("unexpected archive result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveFairLaunchMainnetCollectedBatchTransactionCommit(t *testing.T) {
	fixture := newFairLaunchMainnetArchiveSQLFixture(t)
	collected := FairLaunchMainnetCollectedBatch{seal: &fairLaunchMainnetCollectedBatchSeal{batch: fixture.batch, authorityRowID: fixture.admission.AuthorityRowID, chainID: fixture.admission.ChainID, sourceID: fixture.admission.SourceID, sourceRevisionID: fixture.admission.SourceRevisionID, leaseGeneration: fixture.admission.LeaseGeneration, fenceEpoch: fixture.admission.FenceEpoch, rpcIdentityDigest: fixture.admission.RPCIdentityDigest, batchDigest: fairLaunchMainnetCollectedBatchDigest(fixture.batch)}}
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	expectFairLaunchMainnetArchiveWrites(t, mock, fixture)
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_archive_batch_event_v1`").WillReturnResult(sqlmock.NewResult(31, 1))
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_archive_event_payload_v1`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `fair_launch_mainnet_archive_cursor_v1`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var result FairLaunchMainnetArchiveBatchResult
	err := database.Transaction(func(tx *gorm.DB) error {
		var inner error
		result, inner = ArchiveFairLaunchMainnetCollectedBatchTx(fixture.capability, tx, collected, fixture.now)
		return inner
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.BatchReceiptID != 30 || len(result.Events) != 1 || result.Events[0].seal == nil {
		t.Fatalf("unexpected collected archive result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveFairLaunchMainnetBatchTransactionRollback(t *testing.T) {
	fixture := newFairLaunchMainnetArchiveSQLFixture(t)
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	expectFairLaunchMainnetArchiveWrites(t, mock, fixture)
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_archive_batch_event_v1`").WillReturnError(errors.New("injected archive event failure"))
	mock.ExpectRollback()

	err := database.Transaction(func(tx *gorm.DB) error {
		_, inner := archiveFairLaunchMainnetBatchTx(fixture.capability, tx, fixture.batch, fixture.now)
		return inner
	})
	if err == nil || !strings.Contains(err.Error(), "injected archive event failure") {
		t.Fatalf("expected injected rollback error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveFairLaunchMainnetPayloadTransactionRollback(t *testing.T) {
	fixture := newFairLaunchMainnetArchiveSQLFixture(t)
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	expectFairLaunchMainnetArchiveWrites(t, mock, fixture)
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_archive_batch_event_v1`").WillReturnResult(sqlmock.NewResult(31, 1))
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_archive_event_payload_v1`").WillReturnError(errors.New("injected archive payload failure"))
	mock.ExpectRollback()

	err := database.Transaction(func(tx *gorm.DB) error {
		_, inner := archiveFairLaunchMainnetBatchTx(fixture.capability, tx, fixture.batch, fixture.now)
		return inner
	})
	if err == nil || !strings.Contains(err.Error(), "injected archive payload failure") {
		t.Fatalf("expected injected payload rollback error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInitializeFairLaunchMainnetArchiveCursorTransactionCommit(t *testing.T) {
	fixture := newFairLaunchMainnetArchiveSQLFixture(t)
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	expectFairLaunchMainnetArchiveLease(t, mock, fixture)
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_cursor_v1`").WillReturnRows(sqlmock.NewRows([]string{"activation_row_id"}))
	source := entity.ChainContractSource{ID: fixture.admission.SourceID, ChainID: 56, ContractAddress: fixture.admission.ProxyAddress, ContractVersion: fairLaunchLocalContractVersion, FundingMode: fairLaunchLocalFundingMode, StartBlock: 100, Enabled: true}
	mock.ExpectQuery("SELECT .* FROM `chain_contract_source`").WillReturnRows(sqlRows(t, source))
	revision := entity.ChainContractSourceRevision{ID: fixture.admission.SourceRevisionID, SourceID: fixture.admission.SourceID, FromBlockNumber: 100, ActivationBlockHash: fixture.admission.SourceStartBlockHash, ProxyKind: "uups", ImplementationAddress: fixture.admission.ImplementationAddress, ImplementationCodeHash: fixture.admission.ImplementationCodeHash, ABIDigest: fixture.admission.ABIDigest, ConfirmationPolicy: fixture.admission.ConfirmationPolicy}
	mock.ExpectQuery("SELECT .* FROM `chain_contract_source_revision`").WillReturnRows(sqlRows(t, revision))
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_archive_cursor_v1`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := database.Transaction(func(tx *gorm.DB) error {
		return InitializeFairLaunchMainnetArchiveCursorTx(fixture.capability, tx, fixture.admission.SourceStartBlockHash, fixture.cursor.ArchivedThroughBlockHash, fixture.now)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInitializeFairLaunchMainnetArchiveCursorExactReplay(t *testing.T) {
	fixture := newFairLaunchMainnetArchiveSQLFixture(t)
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	expectFairLaunchMainnetArchiveLease(t, mock, fixture)
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_cursor_v1`").WillReturnRows(sqlRows(t, fixture.cursor))
	mock.ExpectCommit()
	err := database.Transaction(func(tx *gorm.DB) error {
		return InitializeFairLaunchMainnetArchiveCursorTx(fixture.capability, tx, fixture.admission.SourceStartBlockHash, fixture.cursor.ArchivedThroughBlockHash, fixture.now)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBlockFairLaunchMainnetArchiveCursorTransactionCommit(t *testing.T) {
	fixture := newFairLaunchMainnetArchiveSQLFixture(t)
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	expectFairLaunchMainnetArchiveLease(t, mock, fixture)
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_cursor_v1`").WillReturnRows(sqlRows(t, fixture.cursor))
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_archive_dead_letter_v1`").WillReturnResult(sqlmock.NewResult(40, 1))
	mock.ExpectExec("UPDATE `fair_launch_mainnet_archive_cursor_v1`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var result FairLaunchMainnetArchiveFailureResult
	err := database.Transaction(func(tx *gorm.DB) error {
		var inner error
		result, inner = BlockFairLaunchMainnetArchiveCursorTx(fixture.capability, tx, fairLaunchMainnetArchiveFailureFixture(), fixture.now)
		return inner
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.DeadLetterID != 40 || result.CursorStatus != "reorg_detected" || result.EvidenceDigest == "" || result.Replayed {
		t.Fatalf("unexpected failure result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBlockFairLaunchMainnetArchiveCursorTransactionRollback(t *testing.T) {
	fixture := newFairLaunchMainnetArchiveSQLFixture(t)
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	expectFairLaunchMainnetArchiveLease(t, mock, fixture)
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_cursor_v1`").WillReturnRows(sqlRows(t, fixture.cursor))
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_archive_dead_letter_v1`").WillReturnResult(sqlmock.NewResult(40, 1))
	mock.ExpectExec("UPDATE `fair_launch_mainnet_archive_cursor_v1`").WillReturnError(errors.New("injected cursor blocker failure"))
	mock.ExpectRollback()

	err := database.Transaction(func(tx *gorm.DB) error {
		_, inner := BlockFairLaunchMainnetArchiveCursorTx(fixture.capability, tx, fairLaunchMainnetArchiveFailureFixture(), fixture.now)
		return inner
	})
	if err == nil || !strings.Contains(err.Error(), "injected cursor blocker failure") {
		t.Fatalf("expected injected blocker rollback, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRecoverFairLaunchMainnetArchiveCursorTransactionCommit(t *testing.T) {
	fixture, recovery, cursor, deadLetter, previousLease, replacementLease := fairLaunchMainnetArchiveRecoverySQLFixture(t)
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	expectFairLaunchMainnetArchiveRecoveryReads(t, mock, fixture, recovery, cursor, deadLetter, previousLease, replacementLease)
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_archive_recovery_receipt_v1`").WillReturnResult(sqlmock.NewResult(50, 1))
	mock.ExpectExec("UPDATE `fair_launch_mainnet_archive_cursor_v1`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	var result FairLaunchMainnetArchiveRecoveryResult
	err := database.Transaction(func(tx *gorm.DB) error {
		var inner error
		result, inner = recoverFairLaunchMainnetArchiveCursorTx(fixture.capability, tx, recovery, fixture.now)
		return inner
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RecoveryReceiptID != 50 || result.CursorStatus != "active" || result.CursorGeneration != cursor.CursorGeneration+1 || result.LeaseGeneration != replacementLease.LeaseGeneration || result.FenceEpoch != replacementLease.FenceEpoch || result.ReceiptDigest == "" || result.Replayed {
		t.Fatalf("unexpected recovery result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRecoverFairLaunchMainnetArchiveCursorTransactionRollback(t *testing.T) {
	fixture, recovery, cursor, deadLetter, previousLease, replacementLease := fairLaunchMainnetArchiveRecoverySQLFixture(t)
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	expectFairLaunchMainnetArchiveRecoveryReads(t, mock, fixture, recovery, cursor, deadLetter, previousLease, replacementLease)
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_archive_recovery_receipt_v1`").WillReturnResult(sqlmock.NewResult(50, 1))
	mock.ExpectExec("UPDATE `fair_launch_mainnet_archive_cursor_v1`").WillReturnError(errors.New("injected recovery failure"))
	mock.ExpectRollback()
	err := database.Transaction(func(tx *gorm.DB) error {
		_, inner := recoverFairLaunchMainnetArchiveCursorTx(fixture.capability, tx, recovery, fixture.now)
		return inner
	})
	if err == nil || !strings.Contains(err.Error(), "injected recovery failure") {
		t.Fatalf("expected recovery rollback, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRecoverFairLaunchMainnetArchiveCursorExactReplay(t *testing.T) {
	fixture, recovery, cursor, deadLetter, previousLease, replacementLease := fairLaunchMainnetArchiveRecoverySQLFixture(t)
	receipt := fairLaunchMainnetArchiveRecoveryReceiptFixture(fixture, recovery, cursor, deadLetter, previousLease, replacementLease)
	receipt.ID = 50
	receipt.ReceiptDigest = fairLaunchMainnetArchiveRecoveryReceiptDigest(receipt)
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_activation_lease_v1`").WillReturnRows(sqlRows(t, replacementLease))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_recovery_receipt_v1`").WillReturnRows(sqlRows(t, receipt))
	activeCursor := cursor
	activeCursor.Status = "active"
	activeCursor.LastErrorCode = nil
	activeCursor.LastErrorDigest = nil
	activeCursor.LastRecoveryReceiptID = uint64Pointer(receipt.ID)
	activeCursor.LeaseGeneration = replacementLease.LeaseGeneration
	activeCursor.FenceEpoch = replacementLease.FenceEpoch
	activeCursor.LeaseTokenHash = replacementLease.LeaseTokenHash
	activeCursor.CursorGeneration++
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_cursor_v1`").WillReturnRows(sqlRows(t, activeCursor))
	mock.ExpectCommit()
	var result FairLaunchMainnetArchiveRecoveryResult
	err := database.Transaction(func(tx *gorm.DB) error {
		var inner error
		result, inner = recoverFairLaunchMainnetArchiveCursorTx(fixture.capability, tx, recovery, fixture.now)
		return inner
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Replayed || result.RecoveryReceiptID != receipt.ID || result.ReceiptDigest != receipt.ReceiptDigest {
		t.Fatalf("unexpected recovery replay: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRecoverFairLaunchMainnetArchiveCursorReplayConflictRejected(t *testing.T) {
	fixture, recovery, cursor, deadLetter, previousLease, replacementLease := fairLaunchMainnetArchiveRecoverySQLFixture(t)
	receipt := fairLaunchMainnetArchiveRecoveryReceiptFixture(fixture, recovery, cursor, deadLetter, previousLease, replacementLease)
	receipt.ID = 50
	receipt.ReceiptDigest = fairLaunchMainnetArchiveRecoveryReceiptDigest(receipt)
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_activation_lease_v1`").WillReturnRows(sqlRows(t, replacementLease))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_recovery_receipt_v1`").WillReturnRows(sqlRows(t, receipt))
	activeCursor := cursor
	activeCursor.Status = "active"
	activeCursor.LastErrorCode = nil
	activeCursor.LastErrorDigest = nil
	activeCursor.LastRecoveryReceiptID = uint64Pointer(receipt.ID)
	activeCursor.LeaseGeneration = replacementLease.LeaseGeneration
	activeCursor.FenceEpoch = replacementLease.FenceEpoch
	activeCursor.LeaseTokenHash = replacementLease.LeaseTokenHash
	activeCursor.CursorGeneration++
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_cursor_v1`").WillReturnRows(sqlRows(t, activeCursor))
	mock.ExpectRollback()
	recovery.CanonicalChainDigest = strings.Repeat("a", 64)
	err := database.Transaction(func(tx *gorm.DB) error {
		_, inner := recoverFairLaunchMainnetArchiveCursorTx(fixture.capability, tx, recovery, fixture.now)
		return inner
	})
	if err == nil || err.Error() != FairLaunchMainnetArchiveUnavailable {
		t.Fatalf("expected recovery replay conflict rejection, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type fairLaunchMainnetArchiveSQLFixture struct {
	capability FairLaunchMainnetCapability
	admission  FairLaunchMainnetAdmission
	batch      FairLaunchMainnetArchiveBatch
	cursor     entity.FairLaunchMainnetArchiveCursorV1
	now        time.Time
}

func newFairLaunchMainnetArchiveSQLFixture(t *testing.T) fairLaunchMainnetArchiveSQLFixture {
	t.Helper()
	capability, _, _, _, _ := fairLaunchProjectAdmissionFixture(t)
	now := fairLaunchMainnetArchiveTestTime()
	_, cursor, batch := fairLaunchMainnetArchiveValidationFixture(t)
	capability.seal.admission.Component = "job"
	rpcIdentityDigest, err := FairLaunchMainnetRPCIdentityDigest("provider-a")
	if err != nil {
		t.Fatal(err)
	}
	capability.seal.admission.RPCIdentityDigest = rpcIdentityDigest
	capability.seal.admission.ConfirmationPolicy = "20_blocks"
	capability.seal.admission.SourceID = 11
	capability.seal.admission.SourceRevisionID = 12
	capability.seal.admission.SourceStartBlock = 100
	capability.seal.admission.SourceStartBlockHash = hashMainnetArchive('b')
	capability.seal.admission.ProxyAddress = addressMainnet('a')
	capability.seal.admission.ImplementationAddress = addressMainnet('2')
	capability.seal.admission.ImplementationCodeHash = strings.Repeat("5", 64)
	capability.seal.admission.ABIDigest = strings.Repeat("6", 64)
	capability.seal.admission.LeaseGeneration = 3
	capability.seal.admission.FenceEpoch = 4
	capability.seal.admission.ValidUntil = now.Add(time.Hour)
	capability.seal.leaseHash[0] = 1
	admission := capability.seal.admission
	cursor.ActivationRowID = admission.AuthorityRowID
	cursor.ChainID = admission.ChainID
	cursor.SourceID = admission.SourceID
	cursor.SourceRevisionID = admission.SourceRevisionID
	cursor.LeaseGeneration = admission.LeaseGeneration
	cursor.FenceEpoch = admission.FenceEpoch
	cursor.LeaseTokenHash = "01" + strings.Repeat("0", 62)
	cursor.Status = "active"
	cursor.CursorGeneration = 1
	cursor.ArchivedThroughBlock = admission.SourceStartBlock - 1
	cursor.ArchivedThroughBlockHash = hashMainnetArchive('a')
	cursor.CreatedAt = now
	cursor.UpdatedAt = now
	return fairLaunchMainnetArchiveSQLFixture{capability: capability, admission: admission, batch: batch, cursor: cursor, now: now}
}

func openFairLaunchMainnetArchiveSQLMock(t *testing.T) (sqlmock.Sqlmock, *gorm.DB) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	return mock, database
}

func expectFairLaunchMainnetArchiveWrites(t *testing.T, mock sqlmock.Sqlmock, fixture fairLaunchMainnetArchiveSQLFixture) {
	t.Helper()
	expectFairLaunchMainnetArchiveLease(t, mock, fixture)
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_cursor_v1`").WillReturnRows(sqlRows(t, fixture.cursor))
	source := entity.ChainContractSource{ID: fixture.admission.SourceID, ChainID: 56, ContractAddress: fixture.admission.ProxyAddress, ContractVersion: fairLaunchLocalContractVersion, FundingMode: fairLaunchLocalFundingMode, StartBlock: 100, Enabled: true}
	mock.ExpectQuery("SELECT .* FROM `chain_contract_source`").WillReturnRows(sqlRows(t, source))
	revision := entity.ChainContractSourceRevision{ID: fixture.admission.SourceRevisionID, SourceID: fixture.admission.SourceID, FromBlockNumber: 100, ActivationBlockHash: fixture.admission.SourceStartBlockHash, ProxyKind: "uups", ImplementationAddress: fixture.admission.ImplementationAddress, ImplementationCodeHash: fixture.admission.ImplementationCodeHash, ABIDigest: fixture.admission.ABIDigest, ConfirmationPolicy: fixture.admission.ConfirmationPolicy}
	mock.ExpectQuery("SELECT .* FROM `chain_contract_source_revision`").WillReturnRows(sqlRows(t, revision))
	mock.ExpectQuery("SELECT .* FROM `chain_event_identity_v2`").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec("INSERT INTO `chain_event_identity_v2`").WillReturnResult(sqlmock.NewResult(20, 1))
	mock.ExpectQuery("SELECT .* FROM `chain_event_placement_v2`").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT COALESCE.*FROM `chain_event_placement_v2`").WillReturnRows(sqlmock.NewRows([]string{"COALESCE(MAX(generation),0)"}).AddRow(0))
	mock.ExpectExec("INSERT INTO `chain_event_placement_v2`").WillReturnResult(sqlmock.NewResult(21, 1))
	mock.ExpectQuery("SELECT count.*FROM `chain_event_payload_conflict_v2`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("INSERT INTO `chain_event_placement_observation_v2`").WillReturnResult(sqlmock.NewResult(22, 1))
	mock.ExpectQuery("SELECT count.*FROM `chain_event_placement_observation_v2`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_archive_batch_receipt_v1`").WillReturnResult(sqlmock.NewResult(30, 1))
}

func expectFairLaunchMainnetArchiveLease(t *testing.T, mock sqlmock.Sqlmock, fixture fairLaunchMainnetArchiveSQLFixture) {
	t.Helper()
	active := uint8(1)
	lease := entity.FairLaunchMainnetActivationLeaseV1{ID: 2, ActivationRowID: fixture.admission.AuthorityRowID, ChainID: fixture.admission.ChainID, LeaseGeneration: fixture.admission.LeaseGeneration, FenceEpoch: fixture.admission.FenceEpoch, LeaseTokenHash: fixture.cursor.LeaseTokenHash, Status: "active", ActiveSlot: &active, ValidFrom: fixture.now.Add(-time.Hour), ValidUntil: fixture.now.Add(time.Hour)}
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_activation_lease_v1`").WillReturnRows(sqlRows(t, lease))
}

func fairLaunchMainnetArchiveFailureFixture() FairLaunchMainnetArchiveFailure {
	block := uint64(100)
	txIndex := uint32(2)
	txLogOrdinal := uint32(0)
	rpcLogIndex := uint32(3)
	blockHash := hashMainnetArchive('b')
	txHash := hashMainnetArchive('d')
	return FairLaunchMainnetArchiveFailure{ReasonCode: "canonical_conflict", BlockNumber: &block, BlockHash: &blockHash, TransactionIndex: &txIndex, TxLogOrdinal: &txLogOrdinal, RPCLogIndex: &rpcLogIndex, TxHash: &txHash}
}

func fairLaunchMainnetArchiveRecoverySQLFixture(t *testing.T) (fairLaunchMainnetArchiveSQLFixture, FairLaunchMainnetArchiveRecovery, entity.FairLaunchMainnetArchiveCursorV1, entity.FairLaunchMainnetArchiveDeadLetterV1, entity.FairLaunchMainnetActivationLeaseV1, entity.FairLaunchMainnetActivationLeaseV1) {
	t.Helper()
	fixture := newFairLaunchMainnetArchiveSQLFixture(t)
	fixture.capability.seal.admission.LeaseGeneration = 4
	fixture.capability.seal.admission.FenceEpoch = 5
	fixture.capability.seal.leaseHash[0] = 2
	fixture.admission = fixture.capability.seal.admission
	reason := "canonical_conflict"
	evidenceDigest := strings.Repeat("d", 64)
	cursor := fixture.cursor
	cursor.Status = "reorg_detected"
	cursor.LastErrorCode = &reason
	cursor.LastErrorDigest = &evidenceDigest
	cursor.CursorGeneration = 3
	deadLetter := entity.FairLaunchMainnetArchiveDeadLetterV1{ID: 40, ActivationRowID: fixture.admission.AuthorityRowID, ChainID: fixture.admission.ChainID, SourceID: fixture.admission.SourceID, SourceRevisionID: fixture.admission.SourceRevisionID, ReasonCode: reason, EvidenceDigest: evidenceDigest, CreatedAt: fixture.now}
	previousLease := entity.FairLaunchMainnetActivationLeaseV1{ID: 2, ActivationRowID: fixture.admission.AuthorityRowID, ChainID: fixture.admission.ChainID, LeaseGeneration: cursor.LeaseGeneration, FenceEpoch: cursor.FenceEpoch, LeaseTokenHash: cursor.LeaseTokenHash, Status: "revoked", ValidFrom: fixture.now.Add(-2 * time.Hour), ValidUntil: fixture.now.Add(-time.Hour), RevokedAt: localTimePointer(fixture.now.Add(-time.Minute))}
	active := uint8(1)
	replacementLease := entity.FairLaunchMainnetActivationLeaseV1{ID: 3, ActivationRowID: fixture.admission.AuthorityRowID, ChainID: fixture.admission.ChainID, LeaseGeneration: fixture.admission.LeaseGeneration, FenceEpoch: fixture.admission.FenceEpoch, LeaseTokenHash: "02" + strings.Repeat("0", 62), Status: "active", ActiveSlot: &active, ValidFrom: fixture.now.Add(-time.Minute), ValidUntil: fixture.now.Add(time.Hour)}
	recovery := FairLaunchMainnetArchiveRecovery{DeadLetterID: deadLetter.ID, PreviousCursorGeneration: cursor.CursorGeneration, RecoveryAction: "canonical_reverified", VerificationProviderSetDigest: strings.Repeat("e", 64), VerificationCount: 2, CanonicalChainDigest: strings.Repeat("f", 64), ApprovalEvidenceDigest: strings.Repeat("1", 64), RecoveryManifestDigest: strings.Repeat("2", 64)}
	return fixture, recovery, cursor, deadLetter, previousLease, replacementLease
}

func expectFairLaunchMainnetArchiveRecoveryReads(t *testing.T, mock sqlmock.Sqlmock, fixture fairLaunchMainnetArchiveSQLFixture, recovery FairLaunchMainnetArchiveRecovery, cursor entity.FairLaunchMainnetArchiveCursorV1, deadLetter entity.FairLaunchMainnetArchiveDeadLetterV1, previousLease, replacementLease entity.FairLaunchMainnetActivationLeaseV1) {
	t.Helper()
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_activation_lease_v1`").WillReturnRows(sqlRows(t, replacementLease))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_recovery_receipt_v1`").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_cursor_v1`").WillReturnRows(sqlRows(t, cursor))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_dead_letter_v1`").WillReturnRows(sqlRows(t, deadLetter))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_activation_lease_v1`").WillReturnRows(sqlRows(t, previousLease))
}

func fairLaunchMainnetArchiveRecoveryReceiptFixture(fixture fairLaunchMainnetArchiveSQLFixture, recovery FairLaunchMainnetArchiveRecovery, cursor entity.FairLaunchMainnetArchiveCursorV1, deadLetter entity.FairLaunchMainnetArchiveDeadLetterV1, previousLease, replacementLease entity.FairLaunchMainnetActivationLeaseV1) entity.FairLaunchMainnetArchiveRecoveryReceiptV1 {
	return entity.FairLaunchMainnetArchiveRecoveryReceiptV1{ActivationRowID: fixture.admission.AuthorityRowID, ChainID: fixture.admission.ChainID, SourceID: fixture.admission.SourceID, SourceRevisionID: fixture.admission.SourceRevisionID, DeadLetterID: deadLetter.ID, PreviousStatus: cursor.Status, ReasonCode: deadLetter.ReasonCode, PreviousErrorDigest: deadLetter.EvidenceDigest, ArchivedThroughBlock: cursor.ArchivedThroughBlock, ArchivedThroughBlockHash: cursor.ArchivedThroughBlockHash, LastBatchReceiptID: cursor.LastBatchReceiptID, PreviousCursorGeneration: cursor.CursorGeneration, PreviousLeaseID: previousLease.ID, ReplacementLeaseID: replacementLease.ID, RecoveryAction: recovery.RecoveryAction, ConfirmationPolicy: fixture.admission.ConfirmationPolicy, VerificationProviderSetDigest: recovery.VerificationProviderSetDigest, VerificationCount: recovery.VerificationCount, CanonicalChainDigest: recovery.CanonicalChainDigest, ApprovalEvidenceDigest: recovery.ApprovalEvidenceDigest, RecoveryManifestDigest: recovery.RecoveryManifestDigest, CreatedAt: fixture.now}
}
