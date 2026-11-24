package chainevent

import (
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/entity"
)

func TestInitializeFairLaunchMainnetMaterializationTransactionCommit(t *testing.T) {
	fixture := newFairLaunchMainnetMaterializationFixture(t)
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	expectFairLaunchMainnetMaterializationAuthority(t, mock, fixture)
	mock.ExpectQuery("SELECT .* FROM `source_materialization_cursor`").WillReturnRows(sqlmock.NewRows([]string{"source_id"}))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_materialization_cursor_v1`").WillReturnRows(sqlmock.NewRows([]string{"activation_row_id"}))
	mock.ExpectExec("INSERT INTO `source_materialization_cursor`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_materialization_cursor_v1`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var result FairLaunchMainnetMaterializationInitialization
	err := database.Transaction(func(tx *gorm.DB) error {
		var inner error
		result, inner = InitializeFairLaunchMainnetMaterializationTx(fixture.capability, tx, fixture.now)
		return inner
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ActivationRowID != fixture.admission.AuthorityRowID || result.ArchiveEventID != fixture.archiveEvent.ID || result.EventIdentityID != fixture.identity.ID || result.CursorGeneration != 1 || result.Replayed {
		t.Fatalf("unexpected initialization result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInitializeFairLaunchMainnetMaterializationLateFailureRollsBack(t *testing.T) {
	fixture := newFairLaunchMainnetMaterializationFixture(t)
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	expectFairLaunchMainnetMaterializationAuthority(t, mock, fixture)
	mock.ExpectQuery("SELECT .* FROM `source_materialization_cursor`").WillReturnRows(sqlmock.NewRows([]string{"source_id"}))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_materialization_cursor_v1`").WillReturnRows(sqlmock.NewRows([]string{"activation_row_id"}))
	mock.ExpectExec("INSERT INTO `source_materialization_cursor`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_materialization_cursor_v1`").WillReturnError(errors.New("injected mainnet cursor failure"))
	mock.ExpectRollback()

	err := database.Transaction(func(tx *gorm.DB) error {
		_, inner := InitializeFairLaunchMainnetMaterializationTx(fixture.capability, tx, fixture.now)
		return inner
	})
	if err == nil || !strings.Contains(err.Error(), "injected mainnet cursor failure") {
		t.Fatalf("expected late failure rollback, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInitializeFairLaunchMainnetMaterializationExactReplay(t *testing.T) {
	fixture := newFairLaunchMainnetMaterializationFixture(t)
	generic, mainnet := fixture.cursors()
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	expectFairLaunchMainnetMaterializationAuthority(t, mock, fixture)
	mock.ExpectQuery("SELECT .* FROM `source_materialization_cursor`").WillReturnRows(sqlRows(t, generic))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_materialization_cursor_v1`").WillReturnRows(sqlRows(t, mainnet))
	mock.ExpectCommit()

	var result FairLaunchMainnetMaterializationInitialization
	err := database.Transaction(func(tx *gorm.DB) error {
		var inner error
		result, inner = InitializeFairLaunchMainnetMaterializationTx(fixture.capability, tx, fixture.now)
		return inner
	})
	if err != nil || !result.Replayed {
		t.Fatalf("exact replay rejected result=%+v err=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInitializeFairLaunchMainnetMaterializationRejectsPayloadTamper(t *testing.T) {
	fixture := newFairLaunchMainnetMaterializationFixture(t)
	fixture.payload.DataHex = "0x00"
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	expectFairLaunchMainnetMaterializationAuthority(t, mock, fixture)
	mock.ExpectRollback()
	err := database.Transaction(func(tx *gorm.DB) error {
		_, inner := InitializeFairLaunchMainnetMaterializationTx(fixture.capability, tx, fixture.now)
		return inner
	})
	if err == nil || !strings.Contains(err.Error(), FairLaunchMainnetMaterializationUnavailable) {
		t.Fatalf("tampered payload accepted: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMaterializeFairLaunchMainnetNextNoWork(t *testing.T) {
	fixture := newFairLaunchMainnetMaterializationFixture(t)
	generic, mainnet := fixture.cursors()
	archiveCursor := fixture.archiveCursor()
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	expectFairLaunchMainnetMaterializationLeaseAndProfile(t, mock, fixture)
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_materialization_cursor_v1`").WillReturnRows(sqlRows(t, mainnet))
	mock.ExpectQuery("SELECT .* FROM `source_materialization_cursor`").WillReturnRows(sqlRows(t, generic))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_cursor_v1`").WillReturnRows(sqlRows(t, archiveCursor))
	mock.ExpectQuery("SELECT e.id AS archive_event_id FROM fair_launch_mainnet_archive_batch_event_v1 AS e").WillReturnRows(sqlmock.NewRows([]string{"archive_event_id"}))
	mock.ExpectCommit()

	var result FairLaunchMainnetMaterializationResult
	err := database.Transaction(func(tx *gorm.DB) error {
		var inner error
		result, inner = materializeFairLaunchMainnetNextTx(fixture.capability, tx, fixture.now)
		return inner
	})
	if err != nil || !result.NoWork || result.CursorGeneration != 1 {
		t.Fatalf("unexpected no-work result=%+v err=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMaterializeFairLaunchMainnetNextRejectsDoubleCursorDrift(t *testing.T) {
	fixture := newFairLaunchMainnetMaterializationFixture(t)
	generic, mainnet := fixture.cursors()
	generic.MaterializedThroughBlock++
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	expectFairLaunchMainnetMaterializationLeaseAndProfile(t, mock, fixture)
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_materialization_cursor_v1`").WillReturnRows(sqlRows(t, mainnet))
	mock.ExpectQuery("SELECT .* FROM `source_materialization_cursor`").WillReturnRows(sqlRows(t, generic))
	mock.ExpectRollback()

	err := database.Transaction(func(tx *gorm.DB) error {
		_, inner := materializeFairLaunchMainnetNextTx(fixture.capability, tx, fixture.now)
		return inner
	})
	if err == nil || err.Error() != FairLaunchMainnetMaterializationUnavailable {
		t.Fatalf("double cursor drift accepted: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMaterializeFairLaunchMainnetNextRejectsExpiredProfile(t *testing.T) {
	fixture := newFairLaunchMainnetMaterializationFixture(t)
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	active := uint8(1)
	lease := entity.FairLaunchMainnetActivationLeaseV1{ID: 8, ActivationRowID: fixture.admission.AuthorityRowID, ChainID: 56, LeaseGeneration: fixture.admission.LeaseGeneration, FenceEpoch: fixture.admission.FenceEpoch, LeaseTokenHash: "01" + strings.Repeat("0", 62), Status: "active", ActiveSlot: &active, ValidFrom: fixture.now.Add(-time.Minute), ValidUntil: fixture.now.Add(time.Hour)}
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_activation_lease_v1`").WillReturnRows(sqlRows(t, lease))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_materialization_profile_v1`").WillReturnRows(sqlmock.NewRows([]string{"activation_row_id"}))
	mock.ExpectRollback()

	err := database.Transaction(func(tx *gorm.DB) error {
		_, inner := materializeFairLaunchMainnetNextTx(fixture.capability, tx, fixture.now)
		return inner
	})
	if err == nil || err.Error() != FairLaunchMainnetMaterializationUnavailable {
		t.Fatalf("expired profile accepted: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type fairLaunchMainnetMaterializationFixture struct {
	capability   FairLaunchMainnetCapability
	admission    FairLaunchMainnetAdmission
	now          time.Time
	profile      entity.FairLaunchMainnetMaterializationProfileV1
	bootstrap    entity.FairLaunchMainnetProjectBootstrapReceiptV1
	archiveEvent entity.FairLaunchMainnetArchiveBatchEventV1
	identity     entity.ChainEventIdentityV2
	placement    entity.ChainEventPlacementV2
	observation  entity.ChainEventPlacementObservationV2
	payload      entity.FairLaunchMainnetArchiveEventPayloadV1
}

func newFairLaunchMainnetMaterializationFixture(t *testing.T) fairLaunchMainnetMaterializationFixture {
	t.Helper()
	now := fairLaunchMainnetArchiveTestTime()
	capability, _, _, _, _ := fairLaunchProjectAdmissionFixture(t)
	capability.seal.admission.Component = "job"
	capability.seal.admission.SourceID = 11
	capability.seal.admission.SourceRevisionID = 12
	capability.seal.admission.SourceBindingID = 9
	capability.seal.admission.LeaseGeneration = 3
	capability.seal.admission.FenceEpoch = 4
	capability.seal.admission.ProxyAddress = addressMainnet('a')
	capability.seal.admission.ValidUntil = now.Add(time.Hour)
	capability.seal.leaseHash[0] = 1
	admission := capability.seal.admission
	log := fairLaunchProjectCreatedArchiveLog(t, admission.ProxyAddress)
	event, err := commonchain.DecodeFairLaunchTypedLog(log)
	if err != nil || event == nil {
		t.Fatalf("decode project-created fixture: event=%v err=%v", event, err)
	}
	payloadDigest, err := commonchain.FairLaunchTypedEventDigest(event)
	if err != nil {
		t.Fatal(err)
	}
	identity := entity.ChainEventIdentityV2{ID: 20, ChainID: 56, TxHash: strings.ToLower(log.TxHash.Hex()), TxLogOrdinal: 2, CreatedAt: now}
	finalized := now
	canonical := uint8(1)
	placement := entity.ChainEventPlacementV2{ID: 21, EventIdentityID: identity.ID, Generation: 1, BlockNumber: log.BlockNumber, BlockHash: strings.ToLower(log.BlockHash.Hex()), TransactionIndex: 3, RPCLogIndex: uint32(log.Index), SourceRevisionID: admission.SourceRevisionID, CanonicalSlot: &canonical, FinalizedAt: &finalized}
	topicsDigest, rawDigest := fairLaunchLocalLogDigests(log)
	observation := entity.ChainEventPlacementObservationV2{ID: 22, PlacementID: placement.ID, EventIdentityID: identity.ID, ProviderIdentity: "provider-a", EmittingAddress: admission.ProxyAddress, TopicsDataDigest: topicsDigest, RawLogDigest: rawDigest, ObservedAt: now}
	archiveEvent := entity.FairLaunchMainnetArchiveBatchEventV1{ID: 31, BatchReceiptID: 30, ActivationRowID: admission.AuthorityRowID, EventOrdinal: 0, EventIdentityID: identity.ID, PlacementID: placement.ID, ObservationID: observation.ID, EventName: event.Name, PayloadDigest: payloadDigest, CreatedAt: now}
	archiveEvent.RowDigest = fairLaunchMainnetArchiveEventRowDigest(admission, 0, identity.ID, placement.ID, observation.ID, event.Name, payloadDigest)
	payload, err := fairLaunchMainnetArchivePayloadRow(admission.AuthorityRowID, archiveEvent, log, now)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap := entity.FairLaunchMainnetProjectBootstrapReceiptV1{ID: 40, ActivationRowID: admission.AuthorityRowID, ChainID: 56, ProjectBindingID: 41, ProjectID: 41, EventIdentityID: identity.ID, PlacementID: placement.ID, ObservationID: observation.ID, SourceID: admission.SourceID, SourceRevisionID: admission.SourceRevisionID, PlacementGeneration: placement.Generation, EffectManifestVersion: 1, PayloadDigest: payloadDigest, BindingDigest: strings.Repeat("b", 64), InitialStateDigest: strings.Repeat("c", 64), CreatedAt: now}
	bootstrap.ReceiptDigest = fairLaunchV3DomainDigest("JC_FL_MAINNET_PROJECT_BOOTSTRAP_RECEIPT_V1", map[string]any{"activationRowId": bootstrap.ActivationRowID, "chainId": bootstrap.ChainID, "projectBindingId": bootstrap.ProjectBindingID, "projectId": bootstrap.ProjectID, "eventIdentityId": bootstrap.EventIdentityID, "placementId": bootstrap.PlacementID, "observationId": bootstrap.ObservationID, "sourceId": bootstrap.SourceID, "sourceRevisionId": bootstrap.SourceRevisionID, "placementGeneration": bootstrap.PlacementGeneration, "effectManifestVersion": bootstrap.EffectManifestVersion, "payloadDigest": bootstrap.PayloadDigest, "bindingDigest": bootstrap.BindingDigest, "initialStateDigest": bootstrap.InitialStateDigest})
	profile := entity.FairLaunchMainnetMaterializationProfileV1{ActivationRowID: admission.AuthorityRowID, ChainID: 56, SourceID: admission.SourceID, SourceRevisionID: admission.SourceRevisionID, SourceBindingID: admission.SourceBindingID, BootstrapReceiptID: bootstrap.ID, BootstrapArchiveEventID: archiveEvent.ID, InitialEventIdentityID: identity.ID, EffectManifestVersion: 1, EffectManifestDigest: FairLaunchMainnetFinancialEffectManifestDigest(), InitialBlockNumber: placement.BlockNumber, InitialBlockHash: placement.BlockHash, InitialTransactionIndex: placement.TransactionIndex, InitialTxLogOrdinal: identity.TxLogOrdinal, ApprovalEvidenceDigest: strings.Repeat("1", 64), ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour), CreatedAt: now.Add(-time.Minute)}
	profile.ProfileDigest = fairLaunchMainnetMaterializationProfileDigest(profile)
	return fairLaunchMainnetMaterializationFixture{capability: capability, admission: admission, now: now, profile: profile, bootstrap: bootstrap, archiveEvent: archiveEvent, identity: identity, placement: placement, observation: observation, payload: payload}
}

func (fixture fairLaunchMainnetMaterializationFixture) cursors() (entity.SourceMaterializationCursor, entity.FairLaunchMainnetMaterializationCursorV1) {
	leaseHash := "01" + strings.Repeat("0", 62)
	generic := entity.SourceMaterializationCursor{SourceID: fixture.admission.SourceID, SourceRevisionID: fixture.admission.SourceRevisionID, MaterializedThroughBlock: fixture.placement.BlockNumber, MaterializedThroughBlockHash: fixture.placement.BlockHash, MaterializedThroughTxIndex: fixture.placement.TransactionIndex, MaterializedThroughTxLogOrdinal: fixture.identity.TxLogOrdinal, LastEventIdentityID: uint64Pointer(fixture.identity.ID), LeaseToken: leaseHash, FenceEpoch: fixture.admission.FenceEpoch, Status: "active", UpdatedAt: fixture.now, CursorGeneration: 1}
	mainnet := entity.FairLaunchMainnetMaterializationCursorV1{ActivationRowID: fixture.admission.AuthorityRowID, ChainID: fixture.admission.ChainID, SourceID: fixture.admission.SourceID, SourceRevisionID: fixture.admission.SourceRevisionID, SourceBindingID: fixture.admission.SourceBindingID, MaterializedThroughBlock: fixture.placement.BlockNumber, MaterializedThroughBlockHash: fixture.placement.BlockHash, MaterializedThroughTxIndex: fixture.placement.TransactionIndex, MaterializedThroughTxLogOrdinal: fixture.identity.TxLogOrdinal, LastEventIdentityID: uint64Pointer(fixture.identity.ID), LastArchiveEventID: uint64Pointer(fixture.archiveEvent.ID), LeaseGeneration: fixture.admission.LeaseGeneration, FenceEpoch: fixture.admission.FenceEpoch, LeaseTokenHash: leaseHash, Status: "active", CursorGeneration: 1, CreatedAt: fixture.now, UpdatedAt: fixture.now}
	return generic, mainnet
}

func (fixture fairLaunchMainnetMaterializationFixture) archiveCursor() entity.FairLaunchMainnetArchiveCursorV1 {
	return entity.FairLaunchMainnetArchiveCursorV1{ActivationRowID: fixture.admission.AuthorityRowID, ChainID: fixture.admission.ChainID, SourceID: fixture.admission.SourceID, SourceRevisionID: fixture.admission.SourceRevisionID, ArchivedThroughBlock: fixture.placement.BlockNumber, ArchivedThroughBlockHash: fixture.placement.BlockHash, LeaseGeneration: fixture.admission.LeaseGeneration, FenceEpoch: fixture.admission.FenceEpoch, LeaseTokenHash: "01" + strings.Repeat("0", 62), Status: "active", CursorGeneration: 1, CreatedAt: fixture.now, UpdatedAt: fixture.now}
}

func expectFairLaunchMainnetMaterializationLeaseAndProfile(t *testing.T, mock sqlmock.Sqlmock, fixture fairLaunchMainnetMaterializationFixture) {
	t.Helper()
	active := uint8(1)
	lease := entity.FairLaunchMainnetActivationLeaseV1{ID: 8, ActivationRowID: fixture.admission.AuthorityRowID, ChainID: 56, LeaseGeneration: fixture.admission.LeaseGeneration, FenceEpoch: fixture.admission.FenceEpoch, LeaseTokenHash: "01" + strings.Repeat("0", 62), Status: "active", ActiveSlot: &active, ValidFrom: fixture.now.Add(-time.Minute), ValidUntil: fixture.now.Add(time.Hour)}
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_activation_lease_v1`").WillReturnRows(sqlRows(t, lease))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_materialization_profile_v1`").WillReturnRows(sqlRows(t, fixture.profile))
}

func expectFairLaunchMainnetMaterializationAuthority(t *testing.T, mock sqlmock.Sqlmock, fixture fairLaunchMainnetMaterializationFixture) {
	t.Helper()
	active := uint8(1)
	lease := entity.FairLaunchMainnetActivationLeaseV1{ID: 8, ActivationRowID: fixture.admission.AuthorityRowID, ChainID: 56, LeaseGeneration: fixture.admission.LeaseGeneration, FenceEpoch: fixture.admission.FenceEpoch, LeaseTokenHash: "01" + strings.Repeat("0", 62), Status: "active", ActiveSlot: &active, ValidFrom: fixture.now.Add(-time.Minute), ValidUntil: fixture.now.Add(time.Hour)}
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_activation_lease_v1`").WillReturnRows(sqlRows(t, lease))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_materialization_profile_v1`").WillReturnRows(sqlRows(t, fixture.profile))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_project_bootstrap_receipt_v1`").WillReturnRows(sqlRows(t, fixture.bootstrap))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_batch_event_v1`").WillReturnRows(sqlRows(t, fixture.archiveEvent))
	mock.ExpectQuery("SELECT .* FROM `chain_event_identity_v2`").WillReturnRows(sqlRows(t, fixture.identity))
	mock.ExpectQuery("SELECT .* FROM `chain_event_placement_v2`").WillReturnRows(sqlRows(t, fixture.placement))
	mock.ExpectQuery("SELECT .* FROM `chain_event_placement_observation_v2`").WillReturnRows(sqlRows(t, fixture.observation))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_event_payload_v1`").WillReturnRows(sqlRows(t, fixture.payload))
}

func fairLaunchProjectCreatedArchiveLog(t *testing.T, contract string) types.Log {
	t.Helper()
	typesList := []string{"address", "address", "bytes32", "uint32", "address", "bytes32", "uint32", "uint16", "uint256", "uint256", "uint256", "uint256", "uint64", "uint64", "bytes32", "uint32", "bytes32", "uint32", "bytes32", "uint32"}
	arguments := make(abi.Arguments, len(typesList))
	for index, typeName := range typesList {
		typeValue, err := abi.NewType(typeName, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		arguments[index] = abi.Argument{Type: typeValue}
	}
	data, err := arguments.Pack(
		common.HexToAddress(addressMainnet('3')), common.HexToAddress(addressMainnet('4')), bytes32Mainnet(0x11), uint32(2),
		common.HexToAddress(addressMainnet('5')), bytes32Mainnet(0x22), uint32(3), uint16(250),
		big.NewInt(1000), big.NewInt(20), big.NewInt(5), big.NewInt(500), uint64(100), uint64(200),
		bytes32Mainnet(0x33), uint32(4), bytes32Mainnet(0x44), uint32(5), bytes32Mainnet(0x55), uint32(6),
	)
	if err != nil {
		t.Fatal(err)
	}
	signature := commonchain.FairLaunchEventSignatures["FairLaunchProjectCreated"]
	return types.Log{Address: common.HexToAddress(contract), Topics: []common.Hash{crypto.Keccak256Hash([]byte(signature)), common.BigToHash(big.NewInt(41)), common.BytesToHash(common.HexToAddress(addressMainnet('1')).Bytes()), common.BytesToHash(common.HexToAddress(addressMainnet('2')).Bytes())}, Data: data, BlockNumber: 100, TxHash: common.HexToHash(hashMainnetArchive('d')), BlockHash: common.HexToHash(hashMainnetArchive('b')), TxIndex: 3, Index: 4}
}
