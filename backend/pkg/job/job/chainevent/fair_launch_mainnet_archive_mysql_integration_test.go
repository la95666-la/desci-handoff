package chainevent

import (
	"context"
	"math/big"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/entity"
)

func TestFairLaunchMainnetArchiveMySQLAtomicity(t *testing.T) {
	database := openFairLaunchMainnetArchiveMySQL(t)
	fixture := loadFairLaunchMainnetArchiveMySQLFixture(t, database)
	initializeFairLaunchMainnetArchiveCursorMySQL(t, database, fixture)
	var recoveredCapability FairLaunchMainnetCapability

	t.Run("archive rollback leaves cursor and evidence unchanged", func(t *testing.T) {
		collected := fairLaunchMainnetCollectedBatchFixture(fixture)
		if err := database.Exec(`CREATE TRIGGER test_fl_archive_fail BEFORE INSERT ON fair_launch_mainnet_archive_batch_event_v1 FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected archive event failure'`).Error; err != nil {
			t.Fatal(err)
		}
		err := database.Transaction(func(tx *gorm.DB) error {
			_, inner := ArchiveFairLaunchMainnetCollectedBatchTx(fixture.capability, tx, collected, fixture.now)
			return inner
		})
		if err == nil || !strings.Contains(err.Error(), "injected archive event failure") {
			t.Fatalf("expected injected archive rollback, got %v", err)
		}
		if err := database.Exec("DROP TRIGGER test_fl_archive_fail").Error; err != nil {
			t.Fatal(err)
		}
		assertFairLaunchMainnetArchiveMySQLState(t, database, fixture.admission.AuthorityRowID, "99/1/0/0/0/active")
	})

	t.Run("commit exact replay and conflict rejection", func(t *testing.T) {
		rpc := fairLaunchMainnetArchiveMySQLRPCFixture(t, &fixture)
		worker, err := runFairLaunchMainnetArchiveOnce(context.Background(), mustFairLaunchMainnetSourceRegistry(t, fixture.capability, rpc), database, fixture.now)
		if err != nil {
			t.Fatal(err)
		}
		first := worker.Batch
		if worker.Outcome != "archived" || first.BatchReceiptID == 0 || worker.ArchivedThroughBlock != fixture.admission.SourceStartBlock {
			t.Fatalf("unexpected worker archive result: %+v", worker)
		}
		collected, err := CollectFairLaunchMainnetFinalizedBatch(context.Background(), fixture.capability, rpc, fixture.admission.SourceStartBlock, fixture.admission.SourceStartBlock, fixture.now)
		if err != nil {
			t.Fatal(err)
		}
		var replay FairLaunchMainnetArchiveBatchResult
		if err := database.Transaction(func(tx *gorm.DB) error {
			var inner error
			replay, inner = ArchiveFairLaunchMainnetCollectedBatchTx(fixture.capability, tx, collected, fixture.now)
			return inner
		}); err != nil {
			t.Fatal(err)
		}
		if !replay.Replayed || replay.BatchReceiptID != first.BatchReceiptID || replay.ReceiptDigest != first.ReceiptDigest || len(replay.Events) != 1 {
			t.Fatalf("unexpected archive replay first=%+v replay=%+v", first, replay)
		}
		conflict := fairLaunchMainnetCollectedBatchFixture(fixture)
		conflict.seal.batch.Blocks[0].ProviderIdentity = "provider-b"
		conflict.seal.batch.Blocks[0].Events[0].ProviderIdentity = "provider-b"
		err = database.Transaction(func(tx *gorm.DB) error {
			_, inner := ArchiveFairLaunchMainnetCollectedBatchTx(fixture.capability, tx, conflict, fixture.now)
			return inner
		})
		if err == nil || !strings.Contains(err.Error(), FairLaunchMainnetArchiveUnavailable) {
			t.Fatalf("expected replay conflict rejection, got %v", err)
		}
		assertFairLaunchMainnetArchiveMySQLState(t, database, fixture.admission.AuthorityRowID, "100/2/1/1/0/active")
	})

	t.Run("dead letter rollback then commit and exact replay", func(t *testing.T) {
		if err := database.Exec(`CREATE TRIGGER test_fl_dead_letter_fail BEFORE UPDATE ON fair_launch_mainnet_archive_cursor_v1 FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected archive blocker failure'`).Error; err != nil {
			t.Fatal(err)
		}
		err := database.Transaction(func(tx *gorm.DB) error {
			_, inner := BlockFairLaunchMainnetArchiveCursorTx(fixture.capability, tx, fairLaunchMainnetArchiveFailureFixture(), fixture.now)
			return inner
		})
		if err == nil || !strings.Contains(err.Error(), "injected archive blocker failure") {
			t.Fatalf("expected blocker rollback, got %v", err)
		}
		if err := database.Exec("DROP TRIGGER test_fl_dead_letter_fail").Error; err != nil {
			t.Fatal(err)
		}
		assertFairLaunchMainnetArchiveMySQLState(t, database, fixture.admission.AuthorityRowID, "100/2/1/1/0/active")
		var first FairLaunchMainnetArchiveFailureResult
		if err := database.Transaction(func(tx *gorm.DB) error {
			var inner error
			first, inner = BlockFairLaunchMainnetArchiveCursorTx(fixture.capability, tx, fairLaunchMainnetArchiveFailureFixture(), fixture.now)
			return inner
		}); err != nil {
			t.Fatal(err)
		}
		var replay FairLaunchMainnetArchiveFailureResult
		if err := database.Transaction(func(tx *gorm.DB) error {
			var inner error
			replay, inner = BlockFairLaunchMainnetArchiveCursorTx(fixture.capability, tx, fairLaunchMainnetArchiveFailureFixture(), fixture.now)
			return inner
		}); err != nil {
			t.Fatal(err)
		}
		if !replay.Replayed || replay.DeadLetterID != first.DeadLetterID || replay.EvidenceDigest != first.EvidenceDigest || replay.CursorStatus != "reorg_detected" {
			t.Fatalf("unexpected dead-letter replay first=%+v replay=%+v", first, replay)
		}
		assertFairLaunchMainnetArchiveMySQLState(t, database, fixture.admission.AuthorityRowID, "100/3/1/1/1/reorg_detected")
	})

	t.Run("recovery rollback then commit and exact replay", func(t *testing.T) {
		var cursor entity.FairLaunchMainnetArchiveCursorV1
		if err := database.Where("activation_row_id=?", fixture.admission.AuthorityRowID).Take(&cursor).Error; err != nil {
			t.Fatal(err)
		}
		if cursor.LastErrorDigest == nil || cursor.LastErrorCode == nil {
			t.Fatal("blocked cursor evidence missing")
		}
		var deadLetter entity.FairLaunchMainnetArchiveDeadLetterV1
		if err := database.Where("activation_row_id=? AND evidence_digest=?", fixture.admission.AuthorityRowID, *cursor.LastErrorDigest).Take(&deadLetter).Error; err != nil {
			t.Fatal(err)
		}
		var previousLease entity.FairLaunchMainnetActivationLeaseV1
		if err := database.Where("activation_row_id=? AND lease_generation=? AND fence_epoch=?", fixture.admission.AuthorityRowID, cursor.LeaseGeneration, cursor.FenceEpoch).Take(&previousLease).Error; err != nil {
			t.Fatal(err)
		}
		if err := database.Model(&entity.FairLaunchMainnetActivationLeaseV1{}).Where("id=? AND status='active'", previousLease.ID).UpdateColumns(map[string]any{"status": "revoked", "active_slot": nil, "revoked_at": fixture.now}).Error; err != nil {
			t.Fatal(err)
		}
		active := uint8(1)
		replacementLease := entity.FairLaunchMainnetActivationLeaseV1{ActivationRowID: fixture.admission.AuthorityRowID, ChainID: fixture.admission.ChainID, LeaseGeneration: cursor.LeaseGeneration + 1, FenceEpoch: cursor.FenceEpoch + 1, LeaseTokenHash: "02" + strings.Repeat("0", 62), ComponentSetDigest: previousLease.ComponentSetDigest, ApprovalEvidenceDigest: strings.Repeat("4", 64), ValidFrom: fixture.now.Add(-time.Minute), ValidUntil: fixture.now.Add(time.Hour), Status: "active", ActiveSlot: &active, CreatedAt: fixture.now}
		if err := database.Create(&replacementLease).Error; err != nil {
			t.Fatal(err)
		}
		recoveryCapability := fixture.capability
		recoveryCapability.seal = &fairLaunchMainnetSeal{admission: fixture.capability.seal.admission, leaseHash: fixture.capability.seal.leaseHash}
		recoveryCapability.seal.admission.LeaseGeneration = replacementLease.LeaseGeneration
		recoveryCapability.seal.admission.FenceEpoch = replacementLease.FenceEpoch
		recoveryCapability.seal.leaseHash[0] = 2
		primaryRPC := fairLaunchMainnetArchiveMySQLRPCFixture(t, &fixture)
		independentRPC := *primaryRPC
		independentRPC.providerIdentity = "provider-b"
		independentRPC.blocks = cloneFairLaunchMainnetRecoveryBlocks(primaryRPC.blocks)
		independentRPC.blockReads = map[uint64]uint32{}
		independentRPC.mutateOnRead = map[uint64]fairLaunchMainnetRPCBlock{}
		independentRPC.callErrors = map[string]error{}
		primaryBinding, err := BindFairLaunchMainnetRecoveryProvider(recoveryCapability, database, primaryRPC, fixture.now)
		if err != nil {
			t.Fatal(err)
		}
		independentBinding, err := BindFairLaunchMainnetRecoveryProvider(recoveryCapability, database, &independentRPC, fixture.now)
		if err != nil {
			t.Fatal(err)
		}
		verificationRequest := FairLaunchMainnetRecoveryVerificationRequest{DeadLetterID: deadLetter.ID, PreviousCursorGeneration: cursor.CursorGeneration}
		verified, err := VerifyFairLaunchMainnetArchiveRecovery(context.Background(), recoveryCapability, database, []FairLaunchMainnetRecoveryProvider{primaryBinding, independentBinding}, verificationRequest, fixture.now)
		if err != nil {
			t.Fatal(err)
		}
		if err := database.Exec(`CREATE TRIGGER test_fl_recovery_fail BEFORE UPDATE ON fair_launch_mainnet_archive_cursor_v1 FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected archive recovery failure'`).Error; err != nil {
			t.Fatal(err)
		}
		err = database.Transaction(func(tx *gorm.DB) error {
			_, inner := RecoverFairLaunchMainnetArchiveCursorTx(recoveryCapability, tx, verified, fixture.now)
			return inner
		})
		if err == nil || !strings.Contains(err.Error(), "injected archive recovery failure") {
			t.Fatalf("expected recovery rollback, got %v", err)
		}
		if err := database.Exec("DROP TRIGGER test_fl_recovery_fail").Error; err != nil {
			t.Fatal(err)
		}
		assertFairLaunchMainnetArchiveMySQLRecoveryState(t, database, fixture.admission.AuthorityRowID, "reorg_detected/3/3/4/0")
		var first FairLaunchMainnetArchiveRecoveryResult
		if err := database.Transaction(func(tx *gorm.DB) error {
			var inner error
			first, inner = RecoverFairLaunchMainnetArchiveCursorTx(recoveryCapability, tx, verified, fixture.now)
			return inner
		}); err != nil {
			t.Fatal(err)
		}
		var replay FairLaunchMainnetArchiveRecoveryResult
		if err := database.Transaction(func(tx *gorm.DB) error {
			var inner error
			replay, inner = RecoverFairLaunchMainnetArchiveCursorTx(recoveryCapability, tx, verified, fixture.now)
			return inner
		}); err != nil {
			t.Fatal(err)
		}
		if !replay.Replayed || replay.RecoveryReceiptID != first.RecoveryReceiptID || replay.ReceiptDigest != first.ReceiptDigest || replay.CursorGeneration != first.CursorGeneration {
			t.Fatalf("unexpected recovery replay first=%+v replay=%+v", first, replay)
		}
		assertFairLaunchMainnetArchiveMySQLRecoveryState(t, database, fixture.admission.AuthorityRowID, "active/4/4/5/1")
		recoveredCapability = recoveryCapability
	})

	t.Run("materialization rollback then receipt-backed commit", func(t *testing.T) {
		if recoveredCapability.seal == nil {
			t.Fatal("recovered capability unavailable")
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		recoveredCapability.seal.admission.ValidUntil = now.Add(time.Hour)
		var sourceBinding entity.FairLaunchSourceRevisionBindingV3
		if err := database.Where("run_row_id=? AND chain_id=56 AND source_id=? AND source_revision_id=?", fixture.admission.AuthorityRowID, fixture.admission.SourceID, fixture.admission.SourceRevisionID).Take(&sourceBinding).Error; err != nil {
			t.Fatal(err)
		}
		recoveredCapability.seal.admission.SourceBindingID = sourceBinding.ID
		seedFairLaunchMainnetMaterializationGovernanceMySQL(t, database, recoveredCapability, now)
		projectEvent := fairLaunchProjectCreatedArchiveEvent(t, recoveredCapability.seal.admission.ProxyAddress, 900051, 101, fairLaunchLifecycleHash(101), fairLaunchLifecycleHash(1001), 0, 0)
		projectArchive := archiveFairLaunchMainnetMaterializationEventMySQL(t, database, recoveredCapability, projectEvent, hashMainnetArchive('b'), 0, now)
		bootstrap := bootstrapFairLaunchMainnetMaterializationProjectMySQL(t, database, recoveredCapability, projectArchive, now)
		createFairLaunchMainnetMaterializationProfileMySQL(t, database, recoveredCapability, bootstrap, projectArchive, now)
		var initialized FairLaunchMainnetMaterializationInitialization
		if err := database.Transaction(func(tx *gorm.DB) error {
			var inner error
			initialized, inner = InitializeFairLaunchMainnetMaterializationTx(recoveredCapability, tx, now)
			return inner
		}); err != nil {
			t.Fatal(err)
		}
		if initialized.ArchiveEventID != projectArchive.archiveEvent.ID || initialized.EventIdentityID != projectArchive.identity.ID || initialized.Replayed {
			t.Fatalf("unexpected initialized materialization: %+v", initialized)
		}
		depositEvent := fairLaunchFinancialArchiveEvent(t, "FairLaunchTokensDeposited", recoveredCapability.seal.admission.ProxyAddress, 900051, 102, fairLaunchLifecycleHash(102), fairLaunchLifecycleHash(1002), common.HexToAddress(addressMainnet('1')), common.HexToAddress(addressMainnet('3')), big.NewInt(1000), big.NewInt(1000))
		archiveFairLaunchMainnetMaterializationEventMySQL(t, database, recoveredCapability, depositEvent, fairLaunchLifecycleHash(101), 0, now)
		if err := database.Exec(`CREATE TRIGGER test_fl_materialization_fail BEFORE UPDATE ON fair_launch_mainnet_materialization_cursor_v1 FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected mainnet materialization cursor failure'`).Error; err != nil {
			t.Fatal(err)
		}
		err := database.Transaction(func(tx *gorm.DB) error {
			_, inner := materializeFairLaunchMainnetNextTx(recoveredCapability, tx, now)
			return inner
		})
		if err == nil || !strings.Contains(err.Error(), "injected mainnet materialization cursor failure") {
			t.Fatalf("expected materialization rollback, got %v", err)
		}
		if err := database.Exec("DROP TRIGGER test_fl_materialization_fail").Error; err != nil {
			t.Fatal(err)
		}
		assertFairLaunchMainnetMaterializationMySQLState(t, database, recoveredCapability.seal.admission.AuthorityRowID, fixture.admission.SourceID, "Pending/101/1/101/1/0/0/0/0/0/0")
		lastReceipt := ""
		materialize := func(eventName string, generation uint64) FairLaunchMainnetMaterializationResult {
			t.Helper()
			var result FairLaunchMainnetMaterializationResult
			if err := database.Transaction(func(tx *gorm.DB) error {
				var inner error
				result, inner = materializeFairLaunchMainnetNextTx(recoveredCapability, tx, now)
				return inner
			}); err != nil {
				t.Fatal(err)
			}
			if result.EventName != eventName || result.ApplicationID == 0 || result.LineageID == 0 || result.AttestationID == 0 || result.ReceiptID == 0 || result.ReceiptDigest == "" || result.CursorGeneration != generation || result.NoWork {
				t.Fatalf("unexpected %s materialization: %+v", eventName, result)
			}
			var receipt entity.FairLaunchMainnetMaterializationReceiptV1
			if err := database.Where("id=?", result.ReceiptID).Take(&receipt).Error; err != nil {
				t.Fatal(err)
			}
			if generation == 2 && receipt.PreviousReceiptDigest != nil || generation > 2 && (receipt.PreviousReceiptDigest == nil || *receipt.PreviousReceiptDigest != lastReceipt) {
				t.Fatalf("%s receipt predecessor mismatch: %+v", eventName, receipt)
			}
			lastReceipt = result.ReceiptDigest
			return result
		}
		deposit := materialize("FairLaunchTokensDeposited", 2)
		activatedEvent := fairLaunchFinancialArchiveEvent(t, "FairLaunchActivated", recoveredCapability.seal.admission.ProxyAddress, 900051, 103, fairLaunchLifecycleHash(103), fairLaunchLifecycleHash(1003))
		archiveFairLaunchMainnetMaterializationEventMySQL(t, database, recoveredCapability, activatedEvent, fairLaunchLifecycleHash(102), 0, now)
		materialize("FairLaunchActivated", 3)
		contributionEvent := fairLaunchFinancialArchiveEvent(t, "FairLaunchContribution", recoveredCapability.seal.admission.ProxyAddress, 900051, 104, fairLaunchLifecycleHash(104), fairLaunchLifecycleHash(1004), common.HexToAddress(addressMainnet('7')), common.HexToAddress(addressMainnet('2')), big.NewInt(400), big.NewInt(400), big.NewInt(400))
		archiveFairLaunchMainnetMaterializationEventMySQL(t, database, recoveredCapability, contributionEvent, fairLaunchLifecycleHash(103), 0, now)
		materialize("FairLaunchContribution", 4)
		anchorHash := common.HexToHash(fairLaunchLifecycleHash(104))
		endedEvent := fairLaunchFinancialArchiveEvent(t, "FairLaunchEnded", recoveredCapability.seal.admission.ProxyAddress, 900051, 105, fairLaunchLifecycleHash(105), fairLaunchLifecycleHash(1005), uint8(2), uint8(0), common.HexToAddress(addressMainnet('2')), big.NewInt(400), big.NewInt(400), big.NewInt(1000), big.NewInt(105), big.NewInt(104), [32]byte(anchorHash))
		archiveFairLaunchMainnetMaterializationEventMySQL(t, database, recoveredCapability, endedEvent, fairLaunchLifecycleHash(104), 0, now)
		materialize("FairLaunchEnded", 5)
		releaseEvent := fairLaunchFinancialArchiveEvent(t, "FairLaunchSuccessfulReleased", recoveredCapability.seal.admission.ProxyAddress, 900051, 106, fairLaunchLifecycleHash(106), fairLaunchLifecycleHash(1006), big.NewInt(105), common.HexToAddress(addressMainnet('6')), big.NewInt(104), [32]byte(anchorHash), bytes32Mainnet(0x61), uint32(0), bytes32Mainnet(0x62), bytes32Mainnet(0x55), uint32(6))
		archiveFairLaunchMainnetMaterializationEventMySQL(t, database, recoveredCapability, releaseEvent, fairLaunchLifecycleHash(105), 0, now)
		materialize("FairLaunchSuccessfulReleased", 6)
		claimEvent := fairLaunchFinancialArchiveEvent(t, "FairLaunchClaimed", recoveredCapability.seal.admission.ProxyAddress, 900051, 107, fairLaunchLifecycleHash(107), fairLaunchLifecycleHash(1007), common.HexToAddress(addressMainnet('7')), common.HexToAddress(addressMainnet('1')), big.NewInt(1000), big.NewInt(1000), common.HexToAddress(addressMainnet('7')))
		archiveFairLaunchMainnetMaterializationEventMySQL(t, database, recoveredCapability, claimEvent, fairLaunchLifecycleHash(106), 0, now)
		materialize("FairLaunchClaimed", 7)
		feeEvent := fairLaunchFinancialArchiveEvent(t, "FairLaunchPlatformFeeWithdrawn", recoveredCapability.seal.admission.ProxyAddress, 900051, 108, fairLaunchLifecycleHash(108), fairLaunchLifecycleHash(1008), common.HexToAddress(addressMainnet('2')), common.HexToAddress(addressMainnet('5')), big.NewInt(10))
		archiveFairLaunchMainnetMaterializationEventMySQL(t, database, recoveredCapability, feeEvent, fairLaunchLifecycleHash(107), 0, now)
		materialize("FairLaunchPlatformFeeWithdrawn", 8)
		proceedsEvent := fairLaunchFinancialArchiveEvent(t, "FairLaunchProjectProceedsWithdrawn", recoveredCapability.seal.admission.ProxyAddress, 900051, 109, fairLaunchLifecycleHash(109), fairLaunchLifecycleHash(1009), common.HexToAddress(addressMainnet('2')), common.HexToAddress(addressMainnet('4')), big.NewInt(390), big.NewInt(10))
		archiveFairLaunchMainnetMaterializationEventMySQL(t, database, recoveredCapability, proceedsEvent, fairLaunchLifecycleHash(108), 0, now)
		materialize("FairLaunchProjectProceedsWithdrawn", 9)
		if deposit.EventName != "FairLaunchTokensDeposited" {
			t.Fatal("deposit materialization unavailable")
		}
		governanceEvent := fairLaunchGovernanceArchiveEvent(t, recoveredCapability.seal.admission.ProxyAddress, 110, fairLaunchLifecycleHash(110), fairLaunchLifecycleHash(1010), 0, 0)
		archiveFairLaunchMainnetMaterializationEventMySQL(t, database, recoveredCapability, governanceEvent, fairLaunchLifecycleHash(109), 0, now)
		var noWork FairLaunchMainnetMaterializationResult
		if err := database.Transaction(func(tx *gorm.DB) error {
			var inner error
			noWork, inner = materializeFairLaunchMainnetNextTx(recoveredCapability, tx, now)
			return inner
		}); err != nil || !noWork.NoWork || noWork.CursorGeneration != 9 {
			t.Fatalf("unexpected exact no-work replay: %+v err=%v", noWork, err)
		}
		secondProjectEvent := fairLaunchProjectCreatedArchiveEvent(t, recoveredCapability.seal.admission.ProxyAddress, 900052, 111, fairLaunchLifecycleHash(111), fairLaunchLifecycleHash(1011), 0, 0)
		secondArchive := archiveFairLaunchMainnetMaterializationEventMySQL(t, database, recoveredCapability, secondProjectEvent, fairLaunchLifecycleHash(110), 0, now)
		secondBootstrap := bootstrapFairLaunchMainnetMaterializationProjectMySQL(t, database, recoveredCapability, secondArchive, now)
		if secondBootstrap.WatermarkID != 0 || secondBootstrap.Replayed {
			t.Fatalf("subsequent project minted first watermark: %+v", secondBootstrap)
		}
		var secondReplay FairLaunchMainnetProjectBootstrapResult
		if err := database.Transaction(func(tx *gorm.DB) error {
			authority, inner := recoveredCapability.v3Authority(now)
			if inner != nil {
				return inner
			}
			input := FairLaunchCEWrite{Event: secondArchive.event, Context: FairLaunchMaterializationContext{ChainID: 56, TxLogOrdinal: secondArchive.identity.TxLogOrdinal, SourceRevisionID: recoveredCapability.seal.admission.SourceRevisionID, EffectVersion: 1, Canonical: true, Finalized: true}, ProviderIdentity: secondArchive.observation.ProviderIdentity, TopicsDataDigest: secondArchive.observation.TopicsDataDigest, RawLogDigest: secondArchive.observation.RawLogDigest, TransactionIndex: secondArchive.placement.TransactionIndex, SourceID: recoveredCapability.seal.admission.SourceID, LeaseToken: authority.LeaseToken, FenceEpoch: authority.FenceEpoch}
			evidence := fairLaunchV3ArchiveEvidence{IdentityID: secondArchive.identity.ID, PlacementID: secondArchive.placement.ID, ObservationID: secondArchive.observation.ID, PlacementGeneration: secondArchive.placement.Generation, SourceRevisionID: secondArchive.placement.SourceRevisionID, BlockNumber: secondArchive.placement.BlockNumber, BlockHash: secondArchive.placement.BlockHash, TransactionIndex: secondArchive.placement.TransactionIndex, TxLogOrdinal: secondArchive.identity.TxLogOrdinal, RPCLogIndex: secondArchive.placement.RPCLogIndex}
			secondReplay, inner = BootstrapFairLaunchMainnetProjectCreatedTx(recoveredCapability, tx, input, evidence, now)
			return inner
		}); err != nil || !secondReplay.Replayed || secondReplay.BootstrapReceiptID != secondBootstrap.BootstrapReceiptID || secondReplay.WatermarkID != 0 {
			t.Fatalf("subsequent project replay mismatch: %+v err=%v", secondReplay, err)
		}
		secondDeposit := fairLaunchFinancialArchiveEvent(t, "FairLaunchTokensDeposited", recoveredCapability.seal.admission.ProxyAddress, 900052, 112, fairLaunchLifecycleHash(112), fairLaunchLifecycleHash(1012), common.HexToAddress(addressMainnet('1')), common.HexToAddress(addressMainnet('3')), big.NewInt(1000), big.NewInt(1000))
		archiveFairLaunchMainnetMaterializationEventMySQL(t, database, recoveredCapability, secondDeposit, fairLaunchLifecycleHash(111), 0, now)
		materialize("FairLaunchTokensDeposited", 10)
		secondActivated := fairLaunchFinancialArchiveEvent(t, "FairLaunchActivated", recoveredCapability.seal.admission.ProxyAddress, 900052, 113, fairLaunchLifecycleHash(113), fairLaunchLifecycleHash(1013))
		archiveFairLaunchMainnetMaterializationEventMySQL(t, database, recoveredCapability, secondActivated, fairLaunchLifecycleHash(112), 0, now)
		materialize("FairLaunchActivated", 11)
		secondContribution := fairLaunchFinancialArchiveEvent(t, "FairLaunchContribution", recoveredCapability.seal.admission.ProxyAddress, 900052, 114, fairLaunchLifecycleHash(114), fairLaunchLifecycleHash(1014), common.HexToAddress(addressMainnet('8')), common.HexToAddress(addressMainnet('2')), big.NewInt(100), big.NewInt(100), big.NewInt(100))
		archiveFairLaunchMainnetMaterializationEventMySQL(t, database, recoveredCapability, secondContribution, fairLaunchLifecycleHash(113), 0, now)
		materialize("FairLaunchContribution", 12)
		cancelEvent := fairLaunchFinancialArchiveEvent(t, "FairLaunchCancelled", recoveredCapability.seal.admission.ProxyAddress, 900052, 115, fairLaunchLifecycleHash(115), fairLaunchLifecycleHash(1015), uint8(1), bytes32Mainnet(0x71), bytes32Mainnet(0x72), common.HexToAddress(addressMainnet('6')))
		archiveFairLaunchMainnetMaterializationEventMySQL(t, database, recoveredCapability, cancelEvent, fairLaunchLifecycleHash(114), 0, now)
		materialize("FairLaunchCancelled", 13)
		refundEvent := fairLaunchFinancialArchiveEvent(t, "FairLaunchRefunded", recoveredCapability.seal.admission.ProxyAddress, 900052, 116, fairLaunchLifecycleHash(116), fairLaunchLifecycleHash(1016), common.HexToAddress(addressMainnet('8')), common.HexToAddress(addressMainnet('2')), big.NewInt(100), common.HexToAddress(addressMainnet('8')))
		archiveFairLaunchMainnetMaterializationEventMySQL(t, database, recoveredCapability, refundEvent, fairLaunchLifecycleHash(115), 0, now)
		materialize("FairLaunchRefunded", 14)
		failedTokensEvent := fairLaunchFinancialArchiveEvent(t, "FairLaunchFailedProjectTokensWithdrawn", recoveredCapability.seal.admission.ProxyAddress, 900052, 117, fairLaunchLifecycleHash(117), fairLaunchLifecycleHash(1017), common.HexToAddress(addressMainnet('1')), common.HexToAddress(addressMainnet('3')), big.NewInt(1000))
		archiveFairLaunchMainnetMaterializationEventMySQL(t, database, recoveredCapability, failedTokensEvent, fairLaunchLifecycleHash(116), 0, now)
		materialize("FairLaunchFailedProjectTokensWithdrawn", 15)
		assertFairLaunchMainnetLifecycleMySQLState(t, database, recoveredCapability.seal.admission.AuthorityRowID, fixture.admission.SourceID)
	})
}

type fairLaunchMainnetMaterializationArchivedEvent struct {
	archiveEvent entity.FairLaunchMainnetArchiveBatchEventV1
	identity     entity.ChainEventIdentityV2
	placement    entity.ChainEventPlacementV2
	observation  entity.ChainEventPlacementObservationV2
	event        *commonchain.FairLaunchTypedEvent
}

func archiveFairLaunchMainnetMaterializationEventMySQL(t *testing.T, database *gorm.DB, capability FairLaunchMainnetCapability, event *commonchain.FairLaunchTypedEvent, parentHash string, txLogOrdinal uint32, now time.Time) fairLaunchMainnetMaterializationArchivedEvent {
	t.Helper()
	topicsDigest, rawDigest := fairLaunchLocalLogDigests(event.Log)
	provider := "provider-a"
	batch := FairLaunchMainnetArchiveBatch{Blocks: []FairLaunchMainnetFinalizedBlock{{Number: event.Log.BlockNumber, Hash: strings.ToLower(event.Log.BlockHash.Hex()), ParentHash: parentHash, ProviderIdentity: provider, Events: []FairLaunchMainnetArchiveLog{{Event: event, TransactionIndex: uint32(event.Log.TxIndex), TxLogOrdinal: txLogOrdinal, ProviderIdentity: provider, TopicsDataDigest: topicsDigest, RawLogDigest: rawDigest}}}}}
	admission := capability.seal.admission
	collected := FairLaunchMainnetCollectedBatch{seal: &fairLaunchMainnetCollectedBatchSeal{batch: batch, authorityRowID: admission.AuthorityRowID, chainID: admission.ChainID, sourceID: admission.SourceID, sourceRevisionID: admission.SourceRevisionID, leaseGeneration: admission.LeaseGeneration, fenceEpoch: admission.FenceEpoch, rpcIdentityDigest: admission.RPCIdentityDigest, batchDigest: fairLaunchMainnetCollectedBatchDigest(batch)}}
	var result FairLaunchMainnetArchiveBatchResult
	if err := database.Transaction(func(tx *gorm.DB) error {
		var inner error
		result, inner = ArchiveFairLaunchMainnetCollectedBatchTx(capability, tx, collected, now)
		return inner
	}); err != nil {
		t.Fatal(err)
	}
	if result.EventCount != 1 || len(result.Events) != 1 || result.Events[0].seal == nil {
		t.Fatalf("unexpected materialization archive result: %+v", result)
	}
	seal := result.Events[0].seal
	var archiveEvent entity.FairLaunchMainnetArchiveBatchEventV1
	if err := database.Where("batch_receipt_id=? AND event_identity_id=?", result.BatchReceiptID, seal.evidence.IdentityID).Take(&archiveEvent).Error; err != nil {
		t.Fatal(err)
	}
	var identity entity.ChainEventIdentityV2
	var placement entity.ChainEventPlacementV2
	var observation entity.ChainEventPlacementObservationV2
	if database.Where("id=?", seal.evidence.IdentityID).Take(&identity).Error != nil || database.Where("id=?", seal.evidence.PlacementID).Take(&placement).Error != nil || database.Where("id=?", seal.evidence.ObservationID).Take(&observation).Error != nil {
		t.Fatal("materialization archive evidence unavailable")
	}
	return fairLaunchMainnetMaterializationArchivedEvent{archiveEvent: archiveEvent, identity: identity, placement: placement, observation: observation, event: event}
}

func bootstrapFairLaunchMainnetMaterializationProjectMySQL(t *testing.T, database *gorm.DB, capability FairLaunchMainnetCapability, archived fairLaunchMainnetMaterializationArchivedEvent, now time.Time) FairLaunchMainnetProjectBootstrapResult {
	t.Helper()
	facts, err := decodeFairLaunchProjectCreatedFacts(archived.event)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, baseProject, _ := fairLaunchProjectAdmissionFixture(t)
	baseProject.ID = 0
	baseProject.ProjectID = int(facts.ProjectID)
	baseProject.Slug = "mainnet-materialization-" + strconv.FormatUint(facts.ProjectID, 10)
	baseProject.TokenAddress = facts.ProjectToken
	baseProject.PaymentTokenAddress = facts.PaymentToken
	baseProject.OwnerAddress = facts.ProjectOwner
	baseProject.FeeReceiver = facts.PlatformFeeReceiver
	baseProject.SuccessFeeRate = facts.PlatformFeeBPS
	baseProject.SaleTokenAmountRaw = facts.SaleTokenAmountRaw
	baseProject.SoftCapRaw = facts.SoftCapRaw
	baseProject.MinContributionRaw = facts.MinContributionRaw
	baseProject.MaxContributionRaw = facts.MaxContributionRaw
	baseProject.StartTime = time.Unix(int64(facts.StartTime), 0).UTC()
	baseProject.EndTime = time.Unix(int64(facts.EndTime), 0).UTC()
	baseProject.TgeTime = baseProject.EndTime
	baseProject.TeamJSON, baseProject.TokenomicsJSON, baseProject.RoadmapJSON, baseProject.DocumentsJSON, baseProject.FAQJSON = "[]", "[]", "[]", "[]", "[]"
	baseProject.CreatedAt, baseProject.UpdatedAt = now, now
	baseProject.OnchainJSON = `{"fairLaunch":{"saleTokenAmountRaw":"` + facts.SaleTokenAmountRaw + `","softCapRaw":"` + facts.SoftCapRaw + `","minContributionRaw":"` + facts.MinContributionRaw + `","maxContributionRaw":"` + facts.MaxContributionRaw + `","projectProceedsReceiver":"` + facts.ProjectProceedsReceiver + `","projectProceedsReceiverAttestationDigest":"` + facts.ProjectProceedsReceiverAttestationDigest + `","projectProceedsReceiverAttestationVersion":2,"platformFeeReceiverAttestationDigest":"` + facts.PlatformFeeReceiverAttestationDigest + `","platformFeeReceiverAttestationVersion":3,"projectTokenPolicyDigest":"` + facts.ProjectTokenPolicyDigest + `","projectTokenPolicyVersion":4,"paymentTokenPolicyDigest":"` + facts.PaymentTokenPolicyDigest + `","paymentTokenPolicyVersion":5,"releasePolicyDigest":"` + facts.ReleasePolicyDigest + `","releasePolicyVersion":6}}`
	if err := database.Omit("FinalTotalRaisedRaw", "FinalSaleTokenAmountRaw").Create(&baseProject).Error; err != nil {
		t.Fatal(err)
	}
	var payment entity.FairLaunchMainnetPaymentTokenV1
	if err := database.Where("activation_row_id=? AND token_address=?", capability.seal.admission.AuthorityRowID, facts.PaymentToken).Take(&payment).Error; err != nil {
		t.Fatal(err)
	}
	admission := entity.FairLaunchMainnetProjectAdmissionV1{ActivationRowID: capability.seal.admission.AuthorityRowID, ChainID: 56, ProjectRowID: baseProject.ID, ProjectID: facts.ProjectID, PaymentTokenAddress: facts.PaymentToken, PaymentTokenDecimals: payment.TokenDecimals, ProjectTokenAddress: facts.ProjectToken, ProjectTokenDecimals: 18, ProjectTokenRuntimeCodeHash: strings.Repeat("6", 64), MaxTotalContributionRaw: facts.MaxContributionRaw, MaxWalletContributionRaw: facts.MaxContributionRaw, AdmissionDigest: strings.Repeat("8", 64), CreatedAt: now}
	if err := database.Create(&admission).Error; err != nil {
		t.Fatal(err)
	}
	tokenBinding := entity.FairLaunchSourceTokenBindingV3{RunRowID: capability.seal.admission.AuthorityRowID, ChainID: 56, SourceID: capability.seal.admission.SourceID, ContractAddress: capability.seal.admission.ProxyAddress, TokenAddress: facts.ProjectToken, TokenDecimals: 18, MetadataVersion: 1, AdmissionGeneration: 1, RuntimeCodeHash: admission.ProjectTokenRuntimeCodeHash, MetadataDigest: strings.Repeat("f", 64), AdmittedAt: now}
	if err := database.Where("run_row_id=? AND chain_id=? AND source_id=? AND contract_address=? AND token_address=?", tokenBinding.RunRowID, tokenBinding.ChainID, tokenBinding.SourceID, tokenBinding.ContractAddress, tokenBinding.TokenAddress).FirstOrCreate(&tokenBinding).Error; err != nil {
		t.Fatal(err)
	}
	authority, err := capability.v3Authority(now)
	if err != nil {
		t.Fatal(err)
	}
	input := FairLaunchCEWrite{Event: archived.event, Context: FairLaunchMaterializationContext{ChainID: 56, TxLogOrdinal: archived.identity.TxLogOrdinal, SourceRevisionID: capability.seal.admission.SourceRevisionID, EffectVersion: 1, Canonical: true, Finalized: true}, ProviderIdentity: archived.observation.ProviderIdentity, TopicsDataDigest: archived.observation.TopicsDataDigest, RawLogDigest: archived.observation.RawLogDigest, TransactionIndex: archived.placement.TransactionIndex, SourceID: capability.seal.admission.SourceID, LeaseToken: authority.LeaseToken, FenceEpoch: authority.FenceEpoch}
	evidence := fairLaunchV3ArchiveEvidence{IdentityID: archived.identity.ID, PlacementID: archived.placement.ID, ObservationID: archived.observation.ID, PlacementGeneration: archived.placement.Generation, SourceRevisionID: archived.placement.SourceRevisionID, BlockNumber: archived.placement.BlockNumber, BlockHash: archived.placement.BlockHash, TransactionIndex: archived.placement.TransactionIndex, TxLogOrdinal: archived.identity.TxLogOrdinal, RPCLogIndex: archived.placement.RPCLogIndex}
	var result FairLaunchMainnetProjectBootstrapResult
	if err := database.Transaction(func(tx *gorm.DB) error {
		var inner error
		result, inner = BootstrapFairLaunchMainnetProjectCreatedTx(capability, tx, input, evidence, now)
		return inner
	}); err != nil {
		t.Fatal(err)
	}
	return result
}

func seedFairLaunchMainnetMaterializationGovernanceMySQL(t *testing.T, database *gorm.DB, capability FairLaunchMainnetCapability, now time.Time) {
	t.Helper()
	admission := capability.seal.admission
	var historyCount int64
	if err := database.Model(&entity.FairLaunchMainnetActivationHistoryV1{}).Where("activation_row_id=?", admission.AuthorityRowID).Count(&historyCount).Error; err != nil {
		t.Fatal(err)
	}
	if historyCount == 0 {
		rows := []entity.FairLaunchMainnetActivationHistoryV1{{ActivationRowID: admission.AuthorityRowID, ChainID: 56, SequenceNo: 1, State: "prepared", EntryDigest: strings.Repeat("1", 64), EvidenceDigest: strings.Repeat("2", 64), CreatedAt: now}, {ActivationRowID: admission.AuthorityRowID, ChainID: 56, SequenceNo: 2, PreviousState: stringPointer("prepared"), State: "admitted", PreviousEntryDigest: stringPointer(strings.Repeat("1", 64)), EntryDigest: strings.Repeat("3", 64), EvidenceDigest: strings.Repeat("4", 64), CreatedAt: now}, {ActivationRowID: admission.AuthorityRowID, ChainID: 56, SequenceNo: 3, PreviousState: stringPointer("admitted"), State: "source_activated", PreviousEntryDigest: stringPointer(strings.Repeat("3", 64)), EntryDigest: strings.Repeat("5", 64), EvidenceDigest: strings.Repeat("6", 64), CreatedAt: now}}
		if err := database.Create(&rows).Error; err != nil {
			t.Fatal(err)
		}
	}
	component := entity.FairLaunchMainnetComponentBindingV1{ActivationRowID: admission.AuthorityRowID, ChainID: 56, Component: "job", GitCommit: strings.Repeat("1", 40), ArtifactSHA256: strings.Repeat("2", 64), ArtifactSizeBytes: 1, BuildManifestDigest: strings.Repeat("3", 64), SBOMDigest: strings.Repeat("4", 64), CreatedAt: now}
	if err := database.Where("activation_row_id=? AND component='job'", admission.AuthorityRowID).FirstOrCreate(&component).Error; err != nil {
		t.Fatal(err)
	}
}

func createFairLaunchMainnetMaterializationProfileMySQL(t *testing.T, database *gorm.DB, capability FairLaunchMainnetCapability, bootstrap FairLaunchMainnetProjectBootstrapResult, archived fairLaunchMainnetMaterializationArchivedEvent, now time.Time) {
	t.Helper()
	admission := capability.seal.admission
	profile := entity.FairLaunchMainnetMaterializationProfileV1{ActivationRowID: admission.AuthorityRowID, ChainID: 56, SourceID: admission.SourceID, SourceRevisionID: admission.SourceRevisionID, SourceBindingID: admission.SourceBindingID, BootstrapReceiptID: bootstrap.BootstrapReceiptID, BootstrapArchiveEventID: archived.archiveEvent.ID, InitialEventIdentityID: archived.identity.ID, EffectManifestVersion: 1, EffectManifestDigest: FairLaunchMainnetFinancialEffectManifestDigest(), InitialBlockNumber: archived.placement.BlockNumber, InitialBlockHash: archived.placement.BlockHash, InitialTransactionIndex: archived.placement.TransactionIndex, InitialTxLogOrdinal: archived.identity.TxLogOrdinal, ApprovalEvidenceDigest: strings.Repeat("9", 64), ValidFrom: now, ValidUntil: now.Add(time.Hour), CreatedAt: now}
	profile.ProfileDigest = fairLaunchMainnetMaterializationProfileDigest(profile)
	if err := database.Create(&profile).Error; err != nil {
		t.Fatal(err)
	}
}

func fairLaunchProjectCreatedArchiveEvent(t *testing.T, contract string, projectID, blockNumber uint64, blockHash, txHash string, txIndex, logIndex uint32) *commonchain.FairLaunchTypedEvent {
	t.Helper()
	log := fairLaunchProjectCreatedArchiveLog(t, contract)
	log.BlockNumber, log.BlockHash, log.TxHash, log.TxIndex, log.Index = blockNumber, common.HexToHash(blockHash), common.HexToHash(txHash), uint(txIndex), uint(logIndex)
	log.Topics[1] = common.BigToHash(new(big.Int).SetUint64(projectID))
	event, err := commonchain.DecodeFairLaunchTypedLog(log)
	if err != nil || event == nil {
		t.Fatalf("decode project-created archive event: event=%v err=%v", event, err)
	}
	return event
}

func fairLaunchFinancialArchiveEvent(t *testing.T, name, contract string, projectID, blockNumber uint64, blockHash, txHash string, values ...any) *commonchain.FairLaunchTypedEvent {
	t.Helper()
	type eventShape struct {
		types   []string
		indexed map[int]bool
	}
	shapes := map[string]eventShape{
		"FairLaunchTokensDeposited":              {[]string{"uint256", "address", "address", "uint256", "uint256"}, map[int]bool{0: true, 1: true, 2: true}},
		"FairLaunchActivated":                    {[]string{"uint256"}, map[int]bool{0: true}},
		"FairLaunchContribution":                 {[]string{"uint256", "address", "address", "uint256", "uint256", "uint256"}, map[int]bool{0: true, 1: true, 2: true}},
		"FairLaunchEnded":                        {[]string{"uint256", "uint8", "uint8", "address", "uint256", "uint256", "uint256", "uint256", "uint256", "bytes32"}, map[int]bool{0: true, 3: true}},
		"FairLaunchSuccessfulReleased":           {[]string{"uint256", "uint256", "address", "uint256", "bytes32", "bytes32", "uint32", "bytes32", "bytes32", "uint32"}, map[int]bool{0: true, 1: true, 2: true}},
		"FairLaunchCancelled":                    {[]string{"uint256", "uint8", "bytes32", "bytes32", "address"}, map[int]bool{0: true, 4: true}},
		"FairLaunchRefunded":                     {[]string{"uint256", "address", "address", "uint256", "address"}, map[int]bool{0: true, 1: true, 2: true}},
		"FairLaunchClaimed":                      {[]string{"uint256", "address", "address", "uint256", "uint256", "address"}, map[int]bool{0: true, 1: true, 2: true}},
		"FairLaunchProjectProceedsWithdrawn":     {[]string{"uint256", "address", "address", "uint256", "uint256"}, map[int]bool{0: true, 1: true, 2: true}},
		"FairLaunchPlatformFeeWithdrawn":         {[]string{"uint256", "address", "address", "uint256"}, map[int]bool{0: true, 1: true, 2: true}},
		"FairLaunchFailedProjectTokensWithdrawn": {[]string{"uint256", "address", "address", "uint256"}, map[int]bool{0: true, 1: true, 2: true}},
	}
	shape, ok := shapes[name]
	if !ok || len(values)+1 != len(shape.types) {
		t.Fatalf("invalid %s fixture argument count %d", name, len(values))
	}
	allValues := append([]any{new(big.Int).SetUint64(projectID)}, values...)
	dataArguments := abi.Arguments{}
	dataValues := []any{}
	topics := []common.Hash{crypto.Keccak256Hash([]byte(commonchain.FairLaunchEventSignatures[name]))}
	for index, typeName := range shape.types {
		typeValue, err := abi.NewType(typeName, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if shape.indexed[index] {
			switch value := allValues[index].(type) {
			case common.Address:
				topics = append(topics, common.BytesToHash(value.Bytes()))
			case *big.Int:
				topics = append(topics, common.BigToHash(value))
			default:
				t.Fatalf("unsupported indexed %s arg%d type %T", name, index, value)
			}
			continue
		}
		dataArguments = append(dataArguments, abi.Argument{Type: typeValue})
		dataValues = append(dataValues, allValues[index])
	}
	data, err := dataArguments.Pack(dataValues...)
	if err != nil {
		t.Fatalf("pack %s: %v", name, err)
	}
	log := types.Log{Address: common.HexToAddress(contract), Topics: topics, Data: data, BlockNumber: blockNumber, BlockHash: common.HexToHash(blockHash), TxHash: common.HexToHash(txHash)}
	event, err := commonchain.DecodeFairLaunchTypedLog(log)
	if err != nil || event == nil || event.Name != name {
		t.Fatalf("decode %s archive event: event=%v err=%v", name, event, err)
	}
	return event
}

func fairLaunchLifecycleHash(value uint64) string {
	return "0x" + strings.Repeat("0", 64-len(strconv.FormatUint(value, 16))) + strconv.FormatUint(value, 16)
}

func fairLaunchActivatedArchiveEvent(t *testing.T, contract string, projectID, blockNumber uint64, blockHash, txHash string, txIndex, logIndex uint32) *commonchain.FairLaunchTypedEvent {
	t.Helper()
	signature := commonchain.FairLaunchEventSignatures["FairLaunchActivated"]
	log := types.Log{Address: common.HexToAddress(contract), Topics: []common.Hash{crypto.Keccak256Hash([]byte(signature)), common.BigToHash(new(big.Int).SetUint64(projectID))}, BlockNumber: blockNumber, BlockHash: common.HexToHash(blockHash), TxHash: common.HexToHash(txHash), TxIndex: uint(txIndex), Index: uint(logIndex)}
	event, err := commonchain.DecodeFairLaunchTypedLog(log)
	if err != nil || event == nil {
		t.Fatalf("decode activated archive event: event=%v err=%v", event, err)
	}
	return event
}

func fairLaunchContributionArchiveEvent(t *testing.T, contract string, projectID, blockNumber uint64, blockHash, txHash string, txIndex, logIndex uint32) *commonchain.FairLaunchTypedEvent {
	t.Helper()
	uint256Type, err := abi.NewType("uint256", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := (abi.Arguments{{Type: uint256Type}, {Type: uint256Type}, {Type: uint256Type}}).Pack(big.NewInt(10), big.NewInt(10), big.NewInt(10))
	if err != nil {
		t.Fatal(err)
	}
	signature := commonchain.FairLaunchEventSignatures["FairLaunchContribution"]
	log := types.Log{Address: common.HexToAddress(contract), Topics: []common.Hash{crypto.Keccak256Hash([]byte(signature)), common.BigToHash(new(big.Int).SetUint64(projectID)), common.BytesToHash(common.HexToAddress(addressMainnet('7')).Bytes()), common.BytesToHash(common.HexToAddress(addressMainnet('2')).Bytes())}, Data: data, BlockNumber: blockNumber, BlockHash: common.HexToHash(blockHash), TxHash: common.HexToHash(txHash), TxIndex: uint(txIndex), Index: uint(logIndex)}
	event, err := commonchain.DecodeFairLaunchTypedLog(log)
	if err != nil || event == nil {
		t.Fatalf("decode contribution archive event: event=%v err=%v", event, err)
	}
	return event
}

func fairLaunchGovernanceArchiveEvent(t *testing.T, contract string, blockNumber uint64, blockHash, txHash string, txIndex, logIndex uint32) *commonchain.FairLaunchTypedEvent {
	t.Helper()
	bytes32Type, err := abi.NewType("bytes32", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := (abi.Arguments{{Type: bytes32Type}, {Type: bytes32Type}}).Pack(bytes32Mainnet(0x66), bytes32Mainnet(0x77))
	if err != nil {
		t.Fatal(err)
	}
	signature := commonchain.FairLaunchEventSignatures["FairLaunchContributionsPaused"]
	log := types.Log{Address: common.HexToAddress(contract), Topics: []common.Hash{crypto.Keccak256Hash([]byte(signature)), common.BytesToHash(common.HexToAddress(addressMainnet('1')).Bytes())}, Data: data, BlockNumber: blockNumber, BlockHash: common.HexToHash(blockHash), TxHash: common.HexToHash(txHash), TxIndex: uint(txIndex), Index: uint(logIndex)}
	event, err := commonchain.DecodeFairLaunchTypedLog(log)
	if err != nil || event == nil {
		t.Fatalf("decode governance archive event: event=%v err=%v", event, err)
	}
	return event
}

func assertFairLaunchMainnetMaterializationMySQLState(t *testing.T, database *gorm.DB, activationID, sourceID uint64, expected string) {
	t.Helper()
	var state string
	query := `SELECT CONCAT(p.status,'/',m.materialized_through_block,'/',m.cursor_generation,'/',s.materialized_through_block,'/',s.cursor_generation,'/',(SELECT COUNT(*) FROM fair_launch_mainnet_materialization_receipt_v1 WHERE activation_row_id=?),'/',(SELECT COUNT(*) FROM fair_launch_application_lineage_v3 WHERE run_row_id=? AND disposition='forward'),'/',(SELECT COUNT(*) FROM fair_launch_effect_attestation_v3 WHERE run_row_id=?),'/',(SELECT COUNT(*) FROM chain_event_application_attempt_v2 a JOIN fair_launch_application_lineage_v3 l ON l.ce_application_attempt_id=a.id WHERE l.run_row_id=? AND l.disposition='forward'),'/',(SELECT COUNT(*) FROM fair_launch_effect_guard_v3 WHERE run_row_id=?),'/',(SELECT COUNT(*) FROM fair_launch_effect_journal_v3 WHERE run_row_id=?)) FROM fair_launch_project_state_v3 p JOIN fair_launch_mainnet_materialization_cursor_v1 m ON m.activation_row_id=p.run_row_id JOIN source_materialization_cursor s ON s.source_id=? WHERE p.run_row_id=?`
	if err := database.Raw(query, activationID, activationID, activationID, activationID, activationID, activationID, sourceID, activationID).Scan(&state).Error; err != nil {
		t.Fatal(err)
	}
	if state != expected {
		t.Fatalf("materialization state got=%s want=%s", state, expected)
	}
}

func assertFairLaunchMainnetLifecycleMySQLState(t *testing.T, database *gorm.DB, activationID, sourceID uint64) {
	t.Helper()
	var counts string
	query := `SELECT CONCAT((SELECT COUNT(*) FROM fair_launch_mainnet_project_bootstrap_receipt_v1 WHERE activation_row_id=?),'/',(SELECT COUNT(*) FROM fair_launch_mainnet_watermark_v1 WHERE activation_row_id=? AND watermark_kind='first_project_created'),'/',(SELECT COUNT(*) FROM fair_launch_mainnet_materialization_receipt_v1 WHERE activation_row_id=?),'/',(SELECT COUNT(*) FROM chain_event_application_attempt_v2 a JOIN fair_launch_application_lineage_v3 l ON l.ce_application_attempt_id=a.id WHERE l.run_row_id=? AND l.disposition='forward'),'/',(SELECT COUNT(*) FROM fair_launch_application_lineage_v3 WHERE run_row_id=? AND disposition='forward'),'/',(SELECT COUNT(*) FROM fair_launch_effect_attestation_v3 WHERE run_row_id=?),'/',(SELECT COUNT(*) FROM fair_launch_effect_guard_v3 WHERE run_row_id=?),'/',(SELECT COUNT(*) FROM fair_launch_effect_journal_v3 WHERE run_row_id=?),'/',(SELECT COUNT(*) FROM fair_launch_ledger_entry_v3 WHERE run_row_id=?),'/',(SELECT COUNT(*) FROM fair_launch_position_v3 WHERE run_row_id=?),'/',(SELECT cursor_generation FROM source_materialization_cursor WHERE source_id=?),'/',(SELECT cursor_generation FROM fair_launch_mainnet_materialization_cursor_v1 WHERE activation_row_id=?))`
	if err := database.Raw(query, activationID, activationID, activationID, activationID, activationID, activationID, activationID, activationID, activationID, activationID, sourceID, activationID).Scan(&counts).Error; err != nil {
		t.Fatal(err)
	}
	if counts != "2/1/14/14/14/14/14/18/9/2/15/15" {
		t.Fatalf("lifecycle evidence counts got=%s", counts)
	}
	var eventSet string
	eventQuery := `SELECT GROUP_CONCAT(DISTINCT e.event_name ORDER BY e.event_name SEPARATOR ',') FROM fair_launch_mainnet_materialization_receipt_v1 r JOIN fair_launch_mainnet_archive_batch_event_v1 e ON e.id=r.archive_event_id WHERE r.activation_row_id=?`
	if err := database.Raw(eventQuery, activationID).Scan(&eventSet).Error; err != nil {
		t.Fatal(err)
	}
	wantEvents := append([]string(nil), fairLaunchMainnetFinancialEvents...)
	sort.Strings(wantEvents)
	if eventSet != strings.Join(wantEvents, ",") {
		t.Fatalf("materialized event set got=%s", eventSet)
	}
	var states string
	stateQuery := `SELECT GROUP_CONCAT(CONCAT(project_id,':',status,':',terminal_reason,':',total_raised_raw,':',total_tokens_deposited_raw,':',total_tokens_claimed_raw,':',project_proceeds_withdrawn,':',platform_fee_withdrawn,':',failed_project_tokens_withdrawn) ORDER BY project_id SEPARATOR '/') FROM fair_launch_project_state_v3 WHERE run_row_id=?`
	if err := database.Raw(stateQuery, activationID).Scan(&states).Error; err != nil {
		t.Fatal(err)
	}
	if states != "900051:Successful:soft_cap_met:400:1000:1000:1:1:0/900052:Cancelled:cancelled:100:1000:0:0:0:1" {
		t.Fatalf("lifecycle project states got=%s", states)
	}
	var positions string
	positionQuery := `SELECT GROUP_CONCAT(CONCAT(project_id,':',contribution_amount_raw,':',claimed_tokens_raw,':',refunded) ORDER BY project_id SEPARATOR '/') FROM fair_launch_position_v3 WHERE run_row_id=?`
	if err := database.Raw(positionQuery, activationID).Scan(&positions).Error; err != nil {
		t.Fatal(err)
	}
	if positions != "900051:400:1000:0/900052:100:0:1" {
		t.Fatalf("lifecycle positions got=%s", positions)
	}
	var balances string
	balanceQuery := `SELECT GROUP_CONCAT(CONCAT(project_id,':',asset_role,':',inflow,':',outflow,':',CAST(inflow AS DECIMAL(65,0))-CAST(outflow AS DECIMAL(65,0))) ORDER BY project_id,asset_role SEPARATOR '/') FROM (SELECT project_id,asset_role,SUM(CASE WHEN flow_direction='inflow' THEN CAST(amount_raw AS DECIMAL(65,0)) ELSE 0 END) inflow,SUM(CASE WHEN flow_direction='outflow' THEN CAST(amount_raw AS DECIMAL(65,0)) ELSE 0 END) outflow FROM fair_launch_ledger_entry_v3 WHERE run_row_id=? GROUP BY project_id,asset_role) balanced`
	if err := database.Raw(balanceQuery, activationID).Scan(&balances).Error; err != nil {
		t.Fatal(err)
	}
	if balances != "900051:payment:400:400:0/900051:project:1000:1000:0/900052:payment:100:100:0/900052:project:1000:1000:0" {
		t.Fatalf("lifecycle ledger conservation got=%s", balances)
	}
}

func fairLaunchMainnetCollectedBatchFixture(fixture fairLaunchMainnetArchiveSQLFixture) FairLaunchMainnetCollectedBatch {
	blocks := append([]FairLaunchMainnetFinalizedBlock(nil), fixture.batch.Blocks...)
	for index := range blocks {
		blocks[index].Events = append([]FairLaunchMainnetArchiveLog(nil), blocks[index].Events...)
	}
	batch := FairLaunchMainnetArchiveBatch{Blocks: blocks}
	return FairLaunchMainnetCollectedBatch{seal: &fairLaunchMainnetCollectedBatchSeal{batch: batch, authorityRowID: fixture.admission.AuthorityRowID, chainID: fixture.admission.ChainID, sourceID: fixture.admission.SourceID, sourceRevisionID: fixture.admission.SourceRevisionID, leaseGeneration: fixture.admission.LeaseGeneration, fenceEpoch: fixture.admission.FenceEpoch, rpcIdentityDigest: fixture.admission.RPCIdentityDigest, batchDigest: fairLaunchMainnetCollectedBatchDigest(batch)}}
}

func fairLaunchMainnetArchiveMySQLRPCFixture(t *testing.T, fixture *fairLaunchMainnetArchiveSQLFixture) *testFairLaunchMainnetRPC {
	t.Helper()
	_, rpc := fairLaunchMainnetCollectorFixture(t)
	providerIdentity := "provider-a"
	rpc.providerIdentity = providerIdentity
	rpcIdentityDigest, err := FairLaunchMainnetRPCIdentityDigest(providerIdentity)
	if err != nil {
		t.Fatal(err)
	}
	fixture.capability.seal.admission.RPCGenesisHash = rpc.blocks[0].Hash
	fixture.capability.seal.admission.RPCIdentityDigest = rpcIdentityDigest
	fixture.admission = fixture.capability.seal.admission
	rpc.blocks[fixture.admission.SourceStartBlock-1] = fairLaunchMainnetRPCBlock{Hash: fixture.batch.Blocks[0].ParentHash, Number: "0x63", ParentHash: hashMainnetArchive('9')}
	return rpc
}

func openFairLaunchMainnetArchiveMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsnFile := os.Getenv("JOINCARE_FAIR_LAUNCH_ARCHIVE_MYSQL_TEST_DSN_FILE")
	if dsnFile == "" {
		t.Skip("JOINCARE_FAIR_LAUNCH_ARCHIVE_MYSQL_TEST_DSN_FILE is not set")
	}
	info, err := os.Stat(dsnFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("refuse group/world-readable archive DSN file mode %o", info.Mode().Perm())
	}
	encoded, err := os.ReadFile(dsnFile)
	if err != nil {
		t.Fatal(err)
	}
	dsn := strings.TrimSpace(string(encoded))
	if dsn == "" {
		t.Fatal("archive DSN file is empty")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := database.Raw("SELECT DATABASE()").Scan(&name).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(name, "joincare_fl_archive_") {
		t.Fatalf("refuse non-disposable archive database %q", name)
	}
	return database
}

func loadFairLaunchMainnetArchiveMySQLFixture(t *testing.T, database *gorm.DB) fairLaunchMainnetArchiveSQLFixture {
	t.Helper()
	fixture := newFairLaunchMainnetArchiveSQLFixture(t)
	fixture.now = time.Now().UTC()
	var activation entity.FairLaunchMainnetActivationV1
	if err := database.Where("chain_id=56").Take(&activation).Error; err != nil {
		t.Fatal(err)
	}
	var lease entity.FairLaunchMainnetActivationLeaseV1
	if err := database.Where("activation_row_id=? AND chain_id=56 AND active_slot=1", activation.AuthorityRowID).Take(&lease).Error; err != nil {
		t.Fatal(err)
	}
	fixture.capability.seal.admission.AuthorityRowID = activation.AuthorityRowID
	fixture.capability.seal.admission.ChainID = activation.ChainID
	fixture.capability.seal.admission.Component = "job"
	providerIdentity := "provider-a"
	rpcIdentityDigest, err := FairLaunchMainnetRPCIdentityDigest(providerIdentity)
	if err != nil {
		t.Fatal(err)
	}
	fixture.capability.seal.admission.RPCIdentityDigest = rpcIdentityDigest
	fixture.capability.seal.admission.ConfirmationPolicy = "20_blocks"
	fixture.capability.seal.admission.SourceID = activation.SourceID
	fixture.capability.seal.admission.SourceRevisionID = activation.SourceRevisionID
	fixture.capability.seal.admission.SourceStartBlock = activation.SourceStartBlock
	fixture.capability.seal.admission.SourceStartBlockHash = activation.SourceStartBlockHash
	fixture.capability.seal.admission.SourceRegistryDigest = activation.SourceRegistryDigest
	fixture.capability.seal.admission.ProxyAddress = activation.ProxyAddress
	fixture.capability.seal.admission.ImplementationAddress = activation.ImplementationAddress
	fixture.capability.seal.admission.ImplementationCodeHash = activation.ImplementationCodeHash
	fixture.capability.seal.admission.ABIDigest = activation.ABIDigest
	fixture.capability.seal.admission.LeaseGeneration = lease.LeaseGeneration
	fixture.capability.seal.admission.FenceEpoch = lease.FenceEpoch
	fixture.capability.seal.admission.ValidUntil = fixture.now.Add(time.Hour)
	fixture.capability.seal.leaseHash[0] = 1
	fixture.admission = fixture.capability.seal.admission
	fixture.batch.Blocks[0].ProviderIdentity = providerIdentity
	fixture.batch.Blocks[0].Events[0].ProviderIdentity = providerIdentity
	fixture.batch.Blocks[0].Hash = activation.SourceStartBlockHash
	fixture.batch.Blocks[0].Events[0].Event.Log.BlockHash = common.HexToHash(activation.SourceStartBlockHash)
	fixture.batch.Blocks[0].Events[0].Event.Log.Address = common.HexToAddress(activation.ProxyAddress)
	fixture.batch.Blocks[0].Events[0].Event.Log.BlockNumber = activation.SourceStartBlock
	fixture.batch.Blocks[0].Number = activation.SourceStartBlock
	fixture.batch.Blocks[0].ParentHash = hashMainnetArchive('a')
	fixture.batch.Blocks[0].Events[0].Event.Log.TxHash = common.HexToHash(hashMainnetArchive('d'))
	topicsDataDigest, rawLogDigest := fairLaunchLocalLogDigests(fixture.batch.Blocks[0].Events[0].Event.Log)
	fixture.batch.Blocks[0].Events[0].TopicsDataDigest = topicsDataDigest
	fixture.batch.Blocks[0].Events[0].RawLogDigest = rawLogDigest
	return fixture
}

func initializeFairLaunchMainnetArchiveCursorMySQL(t *testing.T, database *gorm.DB, fixture fairLaunchMainnetArchiveSQLFixture) {
	t.Helper()
	if err := database.Transaction(func(tx *gorm.DB) error {
		return InitializeFairLaunchMainnetArchiveCursorTx(fixture.capability, tx, fixture.admission.SourceStartBlockHash, fixture.batch.Blocks[0].ParentHash, fixture.now)
	}); err != nil {
		t.Fatal(err)
	}
}

func assertFairLaunchMainnetArchiveMySQLState(t *testing.T, database *gorm.DB, activationID uint64, expected string) {
	t.Helper()
	var value string
	if err := database.Raw(`SELECT CONCAT(archived_through_block,'/',cursor_generation,'/',(SELECT COUNT(*) FROM fair_launch_mainnet_archive_batch_receipt_v1 WHERE activation_row_id=?),'/',(SELECT COUNT(*) FROM fair_launch_mainnet_archive_batch_event_v1 WHERE activation_row_id=?),'/',(SELECT COUNT(*) FROM fair_launch_mainnet_archive_dead_letter_v1 WHERE activation_row_id=?),'/',status) FROM fair_launch_mainnet_archive_cursor_v1 WHERE activation_row_id=?`, activationID, activationID, activationID, activationID).Scan(&value).Error; err != nil {
		t.Fatal(err)
	}
	if value != expected {
		t.Fatalf("archive state got=%s want=%s", value, expected)
	}
}

func assertFairLaunchMainnetArchiveMySQLRecoveryState(t *testing.T, database *gorm.DB, activationID uint64, expected string) {
	t.Helper()
	var value string
	if err := database.Raw(`SELECT CONCAT(status,'/',cursor_generation,'/',lease_generation,'/',fence_epoch,'/',(SELECT COUNT(*) FROM fair_launch_mainnet_archive_recovery_receipt_v1 WHERE activation_row_id=?)) FROM fair_launch_mainnet_archive_cursor_v1 WHERE activation_row_id=?`, activationID, activationID).Scan(&value).Error; err != nil {
		t.Fatal(err)
	}
	if value != expected {
		t.Fatalf("archive recovery state got=%s want=%s", value, expected)
	}
}
