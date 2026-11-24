package chainevent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"
	"joincare_server/pkg/common/entity"
)

func TestVerifyFairLaunchMainnetArchiveRecoveryConsensus(t *testing.T) {
	capability, providers, cursor, deadLetter, previousLease, replacementLease, providerRows, request := fairLaunchMainnetRecoveryVerifierFixture(t)
	mock, database := fairLaunchMainnetWorkerMockDB(t)
	bindings := bindFairLaunchMainnetRecoveryProvidersForTest(t, mock, database, capability, providers, providerRows)
	expectFairLaunchMainnetRecoveryVerifierRows(t, mock, capability, cursor, deadLetter, previousLease, replacementLease)
	verified, err := VerifyFairLaunchMainnetArchiveRecovery(context.Background(), capability, database, bindings, request, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if verified.seal == nil || verified.seal.recovery.RecoveryAction != "canonical_reverified" || verified.seal.recovery.VerificationCount != 2 || verified.seal.recovery.VerificationProviderSetDigest == "" || verified.seal.recovery.CanonicalChainDigest == "" || verified.seal.verificationHash == "" {
		t.Fatalf("unexpected verified recovery: %+v", verified.seal)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyFairLaunchMainnetArchiveRecoveryProviderDisagreementRejected(t *testing.T) {
	capability, providers, cursor, deadLetter, previousLease, replacementLease, providerRows, request := fairLaunchMainnetRecoveryVerifierFixture(t)
	second := providers[1].(*testFairLaunchMainnetRPC)
	second.mutateOnRead[100] = fairLaunchMainnetRPCBlock{Hash: hashMainnetArchive('f'), Number: "0x64", ParentHash: hashMainnetArchive('a')}
	second.mutateAfterRead = 1
	mock, database := fairLaunchMainnetWorkerMockDB(t)
	bindings := bindFairLaunchMainnetRecoveryProvidersForTest(t, mock, database, capability, providers, providerRows)
	expectFairLaunchMainnetRecoveryVerifierRows(t, mock, capability, cursor, deadLetter, previousLease, replacementLease)
	if _, err := VerifyFairLaunchMainnetArchiveRecovery(context.Background(), capability, database, bindings, request, time.Now().UTC()); err == nil || err.Error() != FairLaunchMainnetRecoveryRejected {
		t.Fatalf("provider view drift accepted: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyFairLaunchMainnetArchiveRecoveryRetryableRPC(t *testing.T) {
	capability, providers, cursor, deadLetter, previousLease, replacementLease, providerRows, request := fairLaunchMainnetRecoveryVerifierFixture(t)
	providers[1].(*testFairLaunchMainnetRPC).callErrors["eth_chainId"] = errors.New("temporary timeout")
	mock, database := fairLaunchMainnetWorkerMockDB(t)
	bindings := bindFairLaunchMainnetRecoveryProvidersForTest(t, mock, database, capability, providers, providerRows)
	expectFairLaunchMainnetRecoveryVerifierRows(t, mock, capability, cursor, deadLetter, previousLease, replacementLease)
	if _, err := VerifyFairLaunchMainnetArchiveRecovery(context.Background(), capability, database, bindings, request, time.Now().UTC()); err == nil || err.Error() != FairLaunchMainnetRecoveryRetryable {
		t.Fatalf("temporary provider failure misclassified: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyFairLaunchMainnetArchiveRecoveryRequiresAdmittedPrimary(t *testing.T) {
	capability, providers, _, _, _, _, providerRows, _ := fairLaunchMainnetRecoveryVerifierFixture(t)
	providerRows[0].ProviderRole = "independent"
	mock, database := fairLaunchMainnetWorkerMockDB(t)
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_recovery_provider_v1`").WillReturnRows(sqlRows(t, providerRows[0]))
	if _, err := BindFairLaunchMainnetRecoveryProvider(capability, database, providers[0], time.Now().UTC()); err == nil || err.Error() != FairLaunchMainnetRecoveryUnavailable {
		t.Fatalf("provider set without admitted primary accepted: %v", err)
	}
}

func TestRecoverFairLaunchMainnetArchiveCursorRejectsTamperedSeal(t *testing.T) {
	capability, _, _, _, _, _, _, request := fairLaunchMainnetRecoveryVerifierFixture(t)
	recovery := FairLaunchMainnetArchiveRecovery{DeadLetterID: request.DeadLetterID, PreviousCursorGeneration: request.PreviousCursorGeneration, RecoveryAction: "canonical_reverified", VerificationProviderSetDigest: strings.Repeat("e", 64), VerificationCount: 2, CanonicalChainDigest: strings.Repeat("f", 64), ApprovalEvidenceDigest: strings.Repeat("5", 64), RecoveryManifestDigest: strings.Repeat("6", 64)}
	admission := capability.seal.admission
	verified := FairLaunchMainnetVerifiedRecovery{seal: &fairLaunchMainnetVerifiedRecoverySeal{recovery: recovery, authorityRowID: admission.AuthorityRowID, chainID: admission.ChainID, sourceID: admission.SourceID, sourceRevisionID: admission.SourceRevisionID, leaseGeneration: admission.LeaseGeneration, fenceEpoch: admission.FenceEpoch}}
	verified.seal.verifiedAt = time.Now().UTC()
	verified.seal.expiresAt = verified.seal.verifiedAt.Add(fairLaunchMainnetRecoverySealTTL)
	verified.seal.verificationHash = fairLaunchMainnetVerifiedRecoveryDigest(admission, recovery, verified.seal.verifiedAt, verified.seal.expiresAt)
	verified.seal.recovery.CanonicalChainDigest = strings.Repeat("a", 64)
	_, database := fairLaunchMainnetWorkerMockDB(t)
	if _, err := RecoverFairLaunchMainnetArchiveCursorTx(capability, database, verified, time.Now().UTC()); err == nil || err.Error() != FairLaunchMainnetRecoveryUnavailable {
		t.Fatalf("tampered recovery seal accepted: %v", err)
	}
}

func TestRecoverFairLaunchMainnetArchiveCursorOpaqueHandoff(t *testing.T) {
	capability, _, cursor, deadLetter, previousLease, replacementLease, _, request := fairLaunchMainnetRecoveryVerifierFixture(t)
	recovery := FairLaunchMainnetArchiveRecovery{DeadLetterID: request.DeadLetterID, PreviousCursorGeneration: request.PreviousCursorGeneration, RecoveryAction: "canonical_reverified", VerificationProviderSetDigest: strings.Repeat("e", 64), VerificationCount: 2, CanonicalChainDigest: strings.Repeat("f", 64), ApprovalEvidenceDigest: strings.Repeat("5", 64), RecoveryManifestDigest: strings.Repeat("6", 64)}
	admission := capability.seal.admission
	verified := FairLaunchMainnetVerifiedRecovery{seal: &fairLaunchMainnetVerifiedRecoverySeal{recovery: recovery, authorityRowID: admission.AuthorityRowID, chainID: admission.ChainID, sourceID: admission.SourceID, sourceRevisionID: admission.SourceRevisionID, leaseGeneration: admission.LeaseGeneration, fenceEpoch: admission.FenceEpoch}}
	verified.seal.verifiedAt = time.Now().UTC()
	verified.seal.expiresAt = verified.seal.verifiedAt.Add(fairLaunchMainnetRecoverySealTTL)
	verified.seal.verificationHash = fairLaunchMainnetVerifiedRecoveryDigest(admission, recovery, verified.seal.verifiedAt, verified.seal.expiresAt)
	fixture := newFairLaunchMainnetArchiveSQLFixture(t)
	fixture.capability = capability
	fixture.admission = admission
	fixture.now = time.Now().UTC()
	mock, database := openFairLaunchMainnetArchiveSQLMock(t)
	expectFairLaunchMainnetArchiveRecoveryReads(t, mock, fixture, recovery, cursor, deadLetter, previousLease, replacementLease)
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_archive_recovery_receipt_v1`").WillReturnResult(sqlmock.NewResult(50, 1))
	mock.ExpectExec("UPDATE `fair_launch_mainnet_archive_cursor_v1`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	var result FairLaunchMainnetArchiveRecoveryResult
	err := database.Transaction(func(tx *gorm.DB) error {
		var inner error
		result, inner = RecoverFairLaunchMainnetArchiveCursorTx(capability, tx, verified, fixture.now)
		return inner
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RecoveryReceiptID != 50 || result.CursorStatus != "active" || result.Replayed {
		t.Fatalf("unexpected opaque recovery result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRecoverFairLaunchMainnetArchiveCursorRejectsExpiredSeal(t *testing.T) {
	capability, _, _, _, _, _, _, request := fairLaunchMainnetRecoveryVerifierFixture(t)
	recovery := FairLaunchMainnetArchiveRecovery{DeadLetterID: request.DeadLetterID, PreviousCursorGeneration: request.PreviousCursorGeneration, RecoveryAction: "canonical_reverified", VerificationProviderSetDigest: strings.Repeat("e", 64), VerificationCount: 2, CanonicalChainDigest: strings.Repeat("f", 64), ApprovalEvidenceDigest: strings.Repeat("5", 64), RecoveryManifestDigest: strings.Repeat("6", 64)}
	admission := capability.seal.admission
	verifiedAt := time.Now().UTC().Add(-fairLaunchMainnetRecoverySealTTL)
	expiresAt := verifiedAt.Add(fairLaunchMainnetRecoverySealTTL)
	verified := FairLaunchMainnetVerifiedRecovery{seal: &fairLaunchMainnetVerifiedRecoverySeal{recovery: recovery, authorityRowID: admission.AuthorityRowID, chainID: admission.ChainID, sourceID: admission.SourceID, sourceRevisionID: admission.SourceRevisionID, leaseGeneration: admission.LeaseGeneration, fenceEpoch: admission.FenceEpoch, verifiedAt: verifiedAt, expiresAt: expiresAt}}
	verified.seal.verificationHash = fairLaunchMainnetVerifiedRecoveryDigest(admission, recovery, verifiedAt, expiresAt)
	_, database := fairLaunchMainnetWorkerMockDB(t)
	if _, err := RecoverFairLaunchMainnetArchiveCursorTx(capability, database, verified, expiresAt); err == nil || err.Error() != FairLaunchMainnetRecoveryUnavailable {
		t.Fatalf("expired recovery seal accepted: %v", err)
	}
}

func fairLaunchMainnetRecoveryVerifierFixture(t *testing.T) (FairLaunchMainnetCapability, []FairLaunchMainnetRPC, entity.FairLaunchMainnetArchiveCursorV1, entity.FairLaunchMainnetArchiveDeadLetterV1, entity.FairLaunchMainnetActivationLeaseV1, entity.FairLaunchMainnetActivationLeaseV1, []entity.FairLaunchMainnetRecoveryProviderV1, FairLaunchMainnetRecoveryVerificationRequest) {
	t.Helper()
	capability, primary := fairLaunchMainnetCollectorFixture(t)
	now := time.Now().UTC()
	capability.seal.admission.SourceID = 11
	capability.seal.admission.SourceRevisionID = 12
	capability.seal.admission.ImplementationAddress = addressMainnet('2')
	capability.seal.admission.ImplementationCodeHash = strings.Repeat("5", 64)
	capability.seal.admission.ABIDigest = strings.Repeat("6", 64)
	capability.seal.admission.LeaseGeneration = 4
	capability.seal.admission.FenceEpoch = 5
	capability.seal.leaseHash[0] = 2
	independent := *primary
	independent.providerIdentity = "bsc-mainnet-provider-b"
	independent.blocks = cloneFairLaunchMainnetRecoveryBlocks(primary.blocks)
	independent.blockReads = map[uint64]uint32{}
	independent.mutateOnRead = map[uint64]fairLaunchMainnetRPCBlock{}
	independent.callErrors = map[string]error{}
	reason := "canonical_conflict"
	evidenceDigest := strings.Repeat("d", 64)
	cursor := entity.FairLaunchMainnetArchiveCursorV1{ActivationRowID: capability.seal.admission.AuthorityRowID, ChainID: 56, SourceID: 11, SourceRevisionID: 12, ArchivedThroughBlock: 100, ArchivedThroughBlockHash: primary.blocks[100].Hash, LeaseGeneration: 3, FenceEpoch: 4, LeaseTokenHash: "01" + strings.Repeat("0", 62), Status: "reorg_detected", LastErrorCode: &reason, LastErrorDigest: &evidenceDigest, CursorGeneration: 3}
	deadLetter := entity.FairLaunchMainnetArchiveDeadLetterV1{ID: 40, ActivationRowID: cursor.ActivationRowID, ChainID: cursor.ChainID, SourceID: cursor.SourceID, SourceRevisionID: cursor.SourceRevisionID, ReasonCode: reason, EvidenceDigest: evidenceDigest, CreatedAt: now}
	previousLease := entity.FairLaunchMainnetActivationLeaseV1{ID: 2, ActivationRowID: cursor.ActivationRowID, ChainID: cursor.ChainID, LeaseGeneration: cursor.LeaseGeneration, FenceEpoch: cursor.FenceEpoch, LeaseTokenHash: cursor.LeaseTokenHash, Status: "revoked", RevokedAt: localTimePointer(now.Add(-time.Minute))}
	active := uint8(1)
	replacementLease := entity.FairLaunchMainnetActivationLeaseV1{ID: 3, ActivationRowID: cursor.ActivationRowID, ChainID: cursor.ChainID, LeaseGeneration: 4, FenceEpoch: 5, LeaseTokenHash: "02" + strings.Repeat("0", 62), Status: "active", ActiveSlot: &active, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), ApprovalEvidenceDigest: strings.Repeat("5", 64)}
	primaryDigest, _ := FairLaunchMainnetRPCIdentityDigest(primary.ProviderIdentity())
	independentDigest, _ := FairLaunchMainnetRPCIdentityDigest(independent.ProviderIdentity())
	providerRows := []entity.FairLaunchMainnetRecoveryProviderV1{
		{ID: 60, ActivationRowID: cursor.ActivationRowID, ChainID: cursor.ChainID, SourceID: cursor.SourceID, SourceRevisionID: cursor.SourceRevisionID, ProviderIdentity: primary.ProviderIdentity(), RPCIdentityDigest: primaryDigest, ProviderRole: "primary", AdmissionManifestDigest: strings.Repeat("1", 64), ApprovalEvidenceDigest: strings.Repeat("2", 64), ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour)},
		{ID: 61, ActivationRowID: cursor.ActivationRowID, ChainID: cursor.ChainID, SourceID: cursor.SourceID, SourceRevisionID: cursor.SourceRevisionID, ProviderIdentity: independent.ProviderIdentity(), RPCIdentityDigest: independentDigest, ProviderRole: "independent", AdmissionManifestDigest: strings.Repeat("3", 64), ApprovalEvidenceDigest: strings.Repeat("4", 64), ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour)},
	}
	request := FairLaunchMainnetRecoveryVerificationRequest{DeadLetterID: deadLetter.ID, PreviousCursorGeneration: cursor.CursorGeneration}
	return capability, []FairLaunchMainnetRPC{primary, &independent}, cursor, deadLetter, previousLease, replacementLease, providerRows, request
}

func expectFairLaunchMainnetRecoveryVerifierRows(t *testing.T, mock sqlmock.Sqlmock, capability FairLaunchMainnetCapability, cursor entity.FairLaunchMainnetArchiveCursorV1, deadLetter entity.FairLaunchMainnetArchiveDeadLetterV1, previousLease, replacementLease entity.FairLaunchMainnetActivationLeaseV1) {
	t.Helper()
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_activation_lease_v1`").WillReturnRows(sqlRows(t, replacementLease))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_cursor_v1`").WillReturnRows(sqlRows(t, cursor))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_archive_dead_letter_v1`").WillReturnRows(sqlRows(t, deadLetter))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_activation_lease_v1`").WillReturnRows(sqlRows(t, previousLease))
	admission := capability.seal.admission
	source := entity.ChainContractSource{ID: admission.SourceID, ChainID: admission.ChainID, ContractAddress: admission.ProxyAddress, ContractVersion: fairLaunchLocalContractVersion, FundingMode: fairLaunchLocalFundingMode, StartBlock: admission.SourceStartBlock, Enabled: true}
	mock.ExpectQuery("SELECT .* FROM `chain_contract_source`").WillReturnRows(sqlRows(t, source))
	revision := entity.ChainContractSourceRevision{ID: admission.SourceRevisionID, SourceID: admission.SourceID, FromBlockNumber: admission.SourceStartBlock, ActivationBlockHash: admission.SourceStartBlockHash, ProxyKind: "uups", ImplementationAddress: admission.ImplementationAddress, ImplementationCodeHash: admission.ImplementationCodeHash, ABIDigest: admission.ABIDigest, ConfirmationPolicy: admission.ConfirmationPolicy}
	mock.ExpectQuery("SELECT .* FROM `chain_contract_source_revision`").WillReturnRows(sqlRows(t, revision))
}

func bindFairLaunchMainnetRecoveryProvidersForTest(t *testing.T, mock sqlmock.Sqlmock, database *gorm.DB, capability FairLaunchMainnetCapability, providers []FairLaunchMainnetRPC, rows []entity.FairLaunchMainnetRecoveryProviderV1) []FairLaunchMainnetRecoveryProvider {
	t.Helper()
	bindings := make([]FairLaunchMainnetRecoveryProvider, len(providers))
	for index, provider := range providers {
		mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_recovery_provider_v1`").WillReturnRows(sqlRows(t, rows[index]))
		binding, err := BindFairLaunchMainnetRecoveryProvider(capability, database, provider, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		bindings[index] = binding
	}
	return bindings
}

func cloneFairLaunchMainnetRecoveryBlocks(source map[uint64]fairLaunchMainnetRPCBlock) map[uint64]fairLaunchMainnetRPCBlock {
	result := make(map[uint64]fairLaunchMainnetRPCBlock, len(source))
	for number, block := range source {
		block.Transactions = append([]string(nil), block.Transactions...)
		result[number] = block
	}
	return result
}
