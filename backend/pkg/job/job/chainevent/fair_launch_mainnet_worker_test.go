package chainevent

import (
	"context"
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

func TestPlanFairLaunchMainnetArchiveRange(t *testing.T) {
	tests := []struct {
		archived, finalized uint64
		from, through       uint64
		hasWork             bool
	}{
		{99, 99, 0, 0, false},
		{99, 100, 100, 100, true},
		{99, 200, 100, 163, true},
	}
	for _, test := range tests {
		from, through, hasWork := planFairLaunchMainnetArchiveRange(test.archived, test.finalized)
		if from != test.from || through != test.through || hasWork != test.hasWork {
			t.Fatalf("plan %d/%d got %d/%d/%v", test.archived, test.finalized, from, through, hasWork)
		}
	}
}

func TestClassifyFairLaunchMainnetCollectorError(t *testing.T) {
	if _, retryable, block := classifyFairLaunchMainnetCollectorError(fairLaunchMainnetCollectorRetryable("timeout")); !retryable || block {
		t.Fatal("retryable collector error misclassified")
	}
	failure, retryable, block := classifyFairLaunchMainnetCollectorError(fairLaunchMainnetCollectorBlocked("canonical_conflict", "fork"))
	if retryable || !block || failure.ReasonCode != "canonical_conflict" {
		t.Fatal("canonical conflict misclassified")
	}
	if _, retryable, block := classifyFairLaunchMainnetCollectorError(errors.New("other")); retryable || block {
		t.Fatal("unknown error classified")
	}
}

func TestBindFairLaunchMainnetSourceRegistryFailClosed(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	if _, err := BindFairLaunchMainnetSourceRegistry(capability, rpc, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	missingDigest := capability
	missingDigest.seal = &fairLaunchMainnetSeal{admission: capability.seal.admission, leaseHash: capability.seal.leaseHash}
	missingDigest.seal.admission.SourceRegistryDigest = ""
	if _, err := BindFairLaunchMainnetSourceRegistry(missingDigest, rpc, time.Now().UTC()); err == nil || err.Error() != FairLaunchMainnetWorkerUnavailable {
		t.Fatalf("missing source registry digest accepted: %v", err)
	}
	wrongProvider := *rpc
	wrongProvider.providerIdentity = "provider-b"
	if _, err := BindFairLaunchMainnetSourceRegistry(capability, &wrongProvider, time.Now().UTC()); err == nil || err.Error() != FairLaunchMainnetWorkerUnavailable {
		t.Fatalf("wrong provider accepted: %v", err)
	}
	if _, _, _, err := (FairLaunchMainnetSourceRegistry{}).route(time.Now().UTC()); err == nil || err.Error() != FairLaunchMainnetWorkerUnavailable {
		t.Fatalf("zero registry routed: %v", err)
	}
}

func TestRunFairLaunchMainnetArchiveOnceNoWork(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	capability.seal.leaseHash[0] = 1
	cursor := fairLaunchMainnetWorkerCursor(capability, 100, rpc.blocks[100].Hash, "active")
	mock, database := fairLaunchMainnetWorkerMockDB(t)
	expectFairLaunchMainnetWorkerCursor(t, mock, capability, cursor)
	result, err := runFairLaunchMainnetArchiveOnce(context.Background(), mustFairLaunchMainnetSourceRegistry(t, capability, rpc), database, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "no_work" || result.ArchivedThroughBlock != 100 || result.ObservedHeadBlock != 120 || result.FinalizedThroughBlock != 100 || result.LagBlocks != 0 {
		t.Fatalf("unexpected no-work result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRunFairLaunchMainnetArchiveOnceStopsBlockedCursorWithoutRPC(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	capability.seal.leaseHash[0] = 1
	reason := "unknown_event"
	cursor := fairLaunchMainnetWorkerCursor(capability, 100, rpc.blocks[100].Hash, "blocked")
	cursor.LastErrorCode = &reason
	mock, database := fairLaunchMainnetWorkerMockDB(t)
	expectFairLaunchMainnetWorkerCursor(t, mock, capability, cursor)
	rpc.callErrors["eth_chainId"] = errors.New("must not be called")
	result, err := runFairLaunchMainnetArchiveOnce(context.Background(), mustFairLaunchMainnetSourceRegistry(t, capability, rpc), database, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "blocked" || result.ReasonCode != reason || result.CursorGeneration != cursor.CursorGeneration {
		t.Fatalf("unexpected stopped result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRunFairLaunchMainnetArchiveOnceArchivesBoundedBatch(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	capability.seal.leaseHash[0] = 1
	rpc.blocks[99] = fairLaunchMainnetRPCBlock{Hash: hashMainnetArchive('a'), Number: "0x63", ParentHash: hashMainnetArchive('9')}
	cursor := fairLaunchMainnetWorkerCursor(capability, 99, hashMainnetArchive('a'), "active")
	mock, database := fairLaunchMainnetWorkerMockDB(t)
	expectFairLaunchMainnetWorkerCursor(t, mock, capability, cursor)
	mock.ExpectBegin()
	fixture := fairLaunchMainnetWorkerSQLFixture(capability, cursor)
	fixture.batch = fairLaunchMainnetArchiveValidationFixtureBatch(t, capability.seal.admission, rpc)
	expectFairLaunchMainnetArchiveWrites(t, mock, fixture)
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_archive_batch_event_v1`").WillReturnResult(sqlmock.NewResult(31, 1))
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_archive_event_payload_v1`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `fair_launch_mainnet_archive_cursor_v1`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	result, err := runFairLaunchMainnetArchiveOnce(context.Background(), mustFairLaunchMainnetSourceRegistry(t, capability, rpc), database, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "archived" || result.ArchivedThroughBlock != 100 || result.ObservedHeadBlock != 120 || result.FinalizedThroughBlock != 100 || result.LagBlocks != 0 || result.Batch.BatchReceiptID != 30 {
		t.Fatalf("unexpected archive result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRunFairLaunchMainnetArchiveOnceRetryableRPCDoesNotBlockCursor(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	capability.seal.leaseHash[0] = 1
	cursor := fairLaunchMainnetWorkerCursor(capability, 100, rpc.blocks[100].Hash, "active")
	mock, database := fairLaunchMainnetWorkerMockDB(t)
	expectFairLaunchMainnetWorkerCursor(t, mock, capability, cursor)
	rpc.callErrors["eth_chainId"] = errors.New("temporary timeout")
	if _, err := runFairLaunchMainnetArchiveOnce(context.Background(), mustFairLaunchMainnetSourceRegistry(t, capability, rpc), database, time.Now().UTC()); err == nil || !strings.Contains(err.Error(), FairLaunchMainnetWorkerRetryable) {
		t.Fatalf("temporary RPC failure did not remain retryable: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRunFairLaunchMainnetArchiveOnceCanonicalConflictBlocksAfterRead(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	capability.seal.leaseHash[0] = 1
	cursor := fairLaunchMainnetWorkerCursor(capability, 100, hashMainnetArchive('f'), "active")
	mock, database := fairLaunchMainnetWorkerMockDB(t)
	expectFairLaunchMainnetWorkerCursor(t, mock, capability, cursor)
	mock.ExpectBegin()
	expectFairLaunchMainnetArchiveLease(t, mock, fairLaunchMainnetWorkerSQLFixture(capability, cursor))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_cursor_v1`").WillReturnRows(sqlRows(t, cursor))
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_archive_dead_letter_v1`").WillReturnResult(sqlmock.NewResult(40, 1))
	mock.ExpectExec("UPDATE `fair_launch_mainnet_archive_cursor_v1`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	result, err := runFairLaunchMainnetArchiveOnce(context.Background(), mustFairLaunchMainnetSourceRegistry(t, capability, rpc), database, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "reorg_detected" || result.ReasonCode != "canonical_conflict" || result.Failure.DeadLetterID != 40 {
		t.Fatalf("unexpected blocker result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRunFairLaunchMainnetArchiveOnceMalformedLatestBlocksCursor(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	capability.seal.leaseHash[0] = 1
	cursor := fairLaunchMainnetWorkerCursor(capability, 100, rpc.blocks[100].Hash, "active")
	mock, database := fairLaunchMainnetWorkerMockDB(t)
	expectFairLaunchMainnetWorkerCursor(t, mock, capability, cursor)
	rpc.blocks[120] = fairLaunchMainnetRPCBlock{Hash: "bad", Number: "0x78", ParentHash: hashMainnetArchive('1')}
	mock.ExpectBegin()
	expectFairLaunchMainnetArchiveLease(t, mock, fairLaunchMainnetWorkerSQLFixture(capability, cursor))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_cursor_v1`").WillReturnRows(sqlRows(t, cursor))
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_archive_dead_letter_v1`").WillReturnResult(sqlmock.NewResult(41, 1))
	mock.ExpectExec("UPDATE `fair_launch_mainnet_archive_cursor_v1`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	result, err := runFairLaunchMainnetArchiveOnce(context.Background(), mustFairLaunchMainnetSourceRegistry(t, capability, rpc), database, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "reorg_detected" || result.ReasonCode != "rpc_inconsistent" || result.Failure.DeadLetterID != 41 {
		t.Fatalf("unexpected malformed latest result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFairLaunchMainnetArchiveStatusReadOnly(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	capability.seal.leaseHash[0] = 1
	rpc.blocks[99] = fairLaunchMainnetRPCBlock{Hash: hashMainnetArchive('a'), Number: "0x63", ParentHash: hashMainnetArchive('9')}
	cursor := fairLaunchMainnetWorkerCursor(capability, 99, hashMainnetArchive('a'), "active")
	mock, database := fairLaunchMainnetWorkerMockDB(t)
	expectFairLaunchMainnetWorkerCursor(t, mock, capability, cursor)
	mock.ExpectQuery("SELECT count.*FROM `fair_launch_mainnet_archive_dead_letter_v1`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	status, err := LoadFairLaunchMainnetArchiveStatus(context.Background(), mustFairLaunchMainnetSourceRegistry(t, capability, rpc), database, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if !status.CursorCanonical || status.ObservedHeadBlock != 120 || status.FinalizedThroughBlock != 100 || status.LagBlocks != 1 || status.DeadLetterCount != 2 || status.CursorGeneration != cursor.CursorGeneration {
		t.Fatalf("unexpected status: %+v", status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFairLaunchMainnetArchiveStatusReportsNoncanonicalCursor(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	capability.seal.leaseHash[0] = 1
	cursor := fairLaunchMainnetWorkerCursor(capability, 100, hashMainnetArchive('f'), "reorg_detected")
	reason := "canonical_conflict"
	cursor.LastErrorCode = &reason
	mock, database := fairLaunchMainnetWorkerMockDB(t)
	expectFairLaunchMainnetWorkerCursor(t, mock, capability, cursor)
	mock.ExpectQuery("SELECT count.*FROM `fair_launch_mainnet_archive_dead_letter_v1`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	status, err := LoadFairLaunchMainnetArchiveStatus(context.Background(), mustFairLaunchMainnetSourceRegistry(t, capability, rpc), database, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if status.CursorCanonical || status.CursorStatus != "reorg_detected" || status.LastErrorCode == nil || *status.LastErrorCode != reason || status.DeadLetterCount != 1 {
		t.Fatalf("unexpected noncanonical status: %+v", status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFairLaunchMainnetArchiveStatusRejectsWrongNetworkIdentity(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	capability.seal.leaseHash[0] = 1
	cursor := fairLaunchMainnetWorkerCursor(capability, 100, rpc.blocks[100].Hash, "active")
	mock, database := fairLaunchMainnetWorkerMockDB(t)
	expectFairLaunchMainnetWorkerCursor(t, mock, capability, cursor)
	rpc.chainID = "0x1"
	if _, err := LoadFairLaunchMainnetArchiveStatus(context.Background(), mustFairLaunchMainnetSourceRegistry(t, capability, rpc), database, time.Now().UTC()); err == nil || err.Error() != FairLaunchMainnetWorkerUnavailable {
		t.Fatalf("wrong network identity accepted by status: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func fairLaunchMainnetWorkerMockDB(t *testing.T) (sqlmock.Sqlmock, *gorm.DB) {
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
	return mock, database
}

func mustFairLaunchMainnetSourceRegistry(t *testing.T, capability FairLaunchMainnetCapability, rpc FairLaunchMainnetRPC) FairLaunchMainnetSourceRegistry {
	t.Helper()
	registry, err := BindFairLaunchMainnetSourceRegistry(capability, rpc, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func fairLaunchMainnetWorkerCursor(capability FairLaunchMainnetCapability, block uint64, hash, status string) entity.FairLaunchMainnetArchiveCursorV1 {
	admission := capability.seal.admission
	return entity.FairLaunchMainnetArchiveCursorV1{ActivationRowID: admission.AuthorityRowID, ChainID: admission.ChainID, SourceID: admission.SourceID, SourceRevisionID: admission.SourceRevisionID, ArchivedThroughBlock: block, ArchivedThroughBlockHash: hash, LeaseGeneration: admission.LeaseGeneration, FenceEpoch: admission.FenceEpoch, LeaseTokenHash: "01" + strings.Repeat("0", 62), Status: status, CursorGeneration: 2}
}

func expectFairLaunchMainnetWorkerCursor(t *testing.T, mock sqlmock.Sqlmock, capability FairLaunchMainnetCapability, cursor entity.FairLaunchMainnetArchiveCursorV1) {
	t.Helper()
	now := time.Now().UTC()
	active := uint8(1)
	admission := capability.seal.admission
	lease := entity.FairLaunchMainnetActivationLeaseV1{ID: 2, ActivationRowID: admission.AuthorityRowID, ChainID: admission.ChainID, LeaseGeneration: admission.LeaseGeneration, FenceEpoch: admission.FenceEpoch, LeaseTokenHash: cursor.LeaseTokenHash, Status: "active", ActiveSlot: &active, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour)}
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_activation_lease_v1`").WillReturnRows(sqlRows(t, lease))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_cursor_v1`").WillReturnRows(sqlRows(t, cursor))
	source := entity.ChainContractSource{ID: admission.SourceID, ChainID: admission.ChainID, ContractAddress: admission.ProxyAddress, ContractVersion: fairLaunchLocalContractVersion, FundingMode: fairLaunchLocalFundingMode, StartBlock: admission.SourceStartBlock, Enabled: true}
	mock.ExpectQuery("SELECT .* FROM `chain_contract_source`").WillReturnRows(sqlRows(t, source))
	revision := entity.ChainContractSourceRevision{ID: admission.SourceRevisionID, SourceID: admission.SourceID, FromBlockNumber: admission.SourceStartBlock, ActivationBlockHash: admission.SourceStartBlockHash, ProxyKind: "uups", ImplementationAddress: admission.ImplementationAddress, ImplementationCodeHash: admission.ImplementationCodeHash, ABIDigest: admission.ABIDigest, ConfirmationPolicy: admission.ConfirmationPolicy}
	mock.ExpectQuery("SELECT .* FROM `chain_contract_source_revision`").WillReturnRows(sqlRows(t, revision))
}

func fairLaunchMainnetWorkerSQLFixture(capability FairLaunchMainnetCapability, cursor entity.FairLaunchMainnetArchiveCursorV1) fairLaunchMainnetArchiveSQLFixture {
	return fairLaunchMainnetArchiveSQLFixture{capability: capability, admission: capability.seal.admission, cursor: cursor, now: time.Now().UTC()}
}

func fairLaunchMainnetArchiveValidationFixtureBatch(t *testing.T, admission FairLaunchMainnetAdmission, rpc *testFairLaunchMainnetRPC) FairLaunchMainnetArchiveBatch {
	t.Helper()
	collected, err := CollectFairLaunchMainnetFinalizedBatch(context.Background(), FairLaunchMainnetCapability{seal: &fairLaunchMainnetSeal{admission: admission}}, rpc, 100, 100, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	return collected.seal.batch
}
