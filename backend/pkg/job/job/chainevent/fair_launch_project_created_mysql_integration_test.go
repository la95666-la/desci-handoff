package chainevent

import (
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"joincare_server/pkg/common/entity"
)

func TestFairLaunchMainnetProjectCreatedMySQLAtomicity(t *testing.T) {
	database := openFairLaunchProjectCreatedMySQL(t)
	resetFairLaunchProjectCreatedMySQL(t, database)

	t.Run("trigger failure rolls back every bootstrap write", func(t *testing.T) {
		fixture := seedFairLaunchProjectCreatedMySQL(t, database, 900041, 'b', 'c')
		if err := database.Exec(`CREATE TRIGGER test_fl_bootstrap_fail BEFORE INSERT ON fair_launch_mainnet_watermark_v1 FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected bootstrap watermark failure'`).Error; err != nil {
			t.Fatal(err)
		}
		err := database.Transaction(func(tx *gorm.DB) error {
			_, inner := BootstrapFairLaunchMainnetProjectCreatedTx(fixture.capability, tx, fixture.input, fixture.archive, fixture.now)
			return inner
		})
		if err == nil || !strings.Contains(err.Error(), "injected bootstrap watermark failure") {
			t.Fatalf("expected injected rollback, got %v", err)
		}
		if err := database.Exec("DROP TRIGGER test_fl_bootstrap_fail").Error; err != nil {
			t.Fatal(err)
		}
		assertFairLaunchProjectCreatedMySQLCounts(t, database, "0/0/0/0/0")
		var row entity.Project
		if err := database.Where("id=?", fixture.project.ID).Take(&row).Error; err != nil {
			t.Fatal(err)
		}
		if row.OnchainLocked || row.Status != "Set" || strings.Contains(row.OnchainJSON, "projectCreatedEventIdentityId") {
			t.Fatalf("rollback leaked project finalization: %+v", row)
		}
	})

	t.Run("commit and exact replay", func(t *testing.T) {
		fixture := seedFairLaunchProjectCreatedMySQL(t, database, 900042, 'd', 'e')
		var first FairLaunchMainnetProjectBootstrapResult
		if err := database.Transaction(func(tx *gorm.DB) error {
			var inner error
			first, inner = BootstrapFairLaunchMainnetProjectCreatedTx(fixture.capability, tx, fixture.input, fixture.archive, fixture.now)
			return inner
		}); err != nil {
			t.Fatal(err)
		}
		assertFairLaunchProjectCreatedMySQLCounts(t, database, "1/1/1/1/1")
		var row entity.Project
		if err := database.Where("id=?", fixture.project.ID).Take(&row).Error; err != nil {
			t.Fatal(err)
		}
		if !row.OnchainLocked || row.Status != "Pending" || !strings.Contains(row.OnchainJSON, "projectCreatedEventIdentityId") {
			t.Fatalf("project row not finalized: %+v", row)
		}
		var replay FairLaunchMainnetProjectBootstrapResult
		if err := database.Transaction(func(tx *gorm.DB) error {
			var inner error
			replay, inner = BootstrapFairLaunchMainnetProjectCreatedTx(fixture.capability, tx, fixture.input, fixture.archive, fixture.now)
			return inner
		}); err != nil {
			t.Fatal(err)
		}
		if !replay.Replayed || replay.ProjectBindingID != first.ProjectBindingID || replay.BootstrapReceiptID != first.BootstrapReceiptID || replay.ProjectStateID != first.ProjectStateID || replay.WatermarkID != first.WatermarkID {
			t.Fatalf("unexpected replay result first=%+v replay=%+v", first, replay)
		}
		assertFairLaunchProjectCreatedMySQLCounts(t, database, "1/1/1/1/1")
	})
}

func openFairLaunchProjectCreatedMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsnFile := os.Getenv("JOINCARE_FAIR_LAUNCH_BOOTSTRAP_MYSQL_TEST_DSN_FILE")
	if dsnFile == "" {
		t.Skip("JOINCARE_FAIR_LAUNCH_BOOTSTRAP_MYSQL_TEST_DSN_FILE is not set")
	}
	info, err := os.Stat(dsnFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("refuse group/world-readable bootstrap DSN file mode %o", info.Mode().Perm())
	}
	encoded, err := os.ReadFile(dsnFile)
	if err != nil {
		t.Fatal(err)
	}
	dsn := strings.TrimSpace(string(encoded))
	if dsn == "" {
		t.Fatal("bootstrap DSN file is empty")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := database.Raw("SELECT DATABASE()").Scan(&name).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(name, "joincare_fl_bootstrap_") {
		t.Fatalf("refuse non-disposable bootstrap database %q", name)
	}
	return database
}

func resetFairLaunchProjectCreatedMySQL(t *testing.T, database *gorm.DB) {
	t.Helper()
	for _, statement := range []string{
		"DROP TRIGGER IF EXISTS test_fl_bootstrap_fail",
		"DELETE FROM fair_launch_mainnet_watermark_v1",
		"DELETE FROM fair_launch_project_state_v3",
		"DELETE FROM fair_launch_mainnet_project_bootstrap_receipt_v1",
		"DELETE FROM fair_launch_project_binding_v3",
		"DELETE FROM chain_event_payload_conflict_v2",
		"DELETE FROM chain_event_placement_observation_v2",
		"DELETE FROM chain_event_placement_v2",
		"DELETE FROM chain_event_identity_v2",
		"UPDATE projects SET status='Set',onchain_locked=0,onchain_json=JSON_REMOVE(onchain_json,'$.fairLaunch.projectCreatedEventIdentityId','$.fairLaunch.projectCreatedPlacementId','$.fairLaunch.projectCreatedPayloadDigest') WHERE funding_mode='fair_launch'",
	} {
		if err := database.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func seedFairLaunchProjectCreatedMySQL(t *testing.T, database *gorm.DB, projectID uint64, txDigit, blockDigit byte) fairLaunchProjectCreatedSQLFixture {
	t.Helper()
	fixture := newFairLaunchProjectCreatedSQLFixture(t)
	var activation entity.FairLaunchMainnetActivationV1
	if err := database.Where("chain_id=56").Take(&activation).Error; err != nil {
		t.Fatal(err)
	}
	var payment entity.FairLaunchMainnetPaymentTokenV1
	if err := database.Where("activation_row_id=? AND chain_id=56", activation.AuthorityRowID).Take(&payment).Error; err != nil {
		t.Fatal(err)
	}
	var sourceBinding entity.FairLaunchSourceRevisionBindingV3
	if err := database.Where("run_row_id=? AND chain_id=56", activation.AuthorityRowID).Take(&sourceBinding).Error; err != nil {
		t.Fatal(err)
	}
	fixture.capability.seal.admission.AuthorityRowID = activation.AuthorityRowID
	fixture.capability.seal.admission.SourceID = activation.SourceID
	fixture.capability.seal.admission.SourceRevisionID = activation.SourceRevisionID
	fixture.capability.seal.admission.SourceBindingID = sourceBinding.ID
	fixture.capability.seal.admission.SourceStartBlock = activation.SourceStartBlock
	fixture.capability.seal.admission.ProxyAddress = activation.ProxyAddress
	fixture.capability.seal.admission.ValidUntil = fixture.now.Add(time.Hour)
	fixture.input.SourceID = activation.SourceID
	fixture.input.Context.SourceRevisionID = activation.SourceRevisionID
	authority, err := fixture.capability.v3Authority(fixture.now)
	if err != nil {
		t.Fatal(err)
	}
	fixture.input.LeaseToken = authority.LeaseToken
	fixture.input.FenceEpoch = authority.FenceEpoch
	fixture.archive.SourceRevisionID = activation.SourceRevisionID
	fixture.payment = payment
	fixture.project.ID = 0
	fixture.project.ProjectID = int(projectID)
	fixture.project.PaymentToken = payment.TokenSymbol
	fixture.project.PaymentTokenAddress = payment.TokenAddress
	fixture.project.PaymentTokenDecimals = int(payment.TokenDecimals)
	fixture.project.PaymentTokenPolicyRevision = &payment.PolicyVersion
	fixture.project.TgeTime = time.Unix(200, 0).UTC()
	fixture.project.TeamJSON = "[]"
	fixture.project.TokenomicsJSON = "[]"
	fixture.project.RoadmapJSON = "[]"
	fixture.project.DocumentsJSON = "[]"
	fixture.project.FAQJSON = "[]"
	fixture.admission.ID = 0
	fixture.admission.ActivationRowID = activation.AuthorityRowID
	fixture.admission.ProjectRowID = projectID
	fixture.admission.ProjectID = projectID
	fixture.admission.PaymentTokenAddress = payment.TokenAddress
	fixture.admission.PaymentTokenDecimals = payment.TokenDecimals
	fixture.input.Event.Indexed["arg2"] = common.HexToAddress(payment.TokenAddress)
	fixture.input.Event.ProjectID.SetUint64(projectID)
	fixture.input.Event.Indexed["arg0"] = newBigUint(projectID)
	fixture.input.Event.RawDecimal["arg0"] = newBigUint(projectID).String()
	fixture.input.Event.Log.TxHash = common.HexToHash("0x" + strings.Repeat(string(txDigit), 64))
	fixture.input.Event.Log.BlockHash = common.HexToHash("0x" + strings.Repeat(string(blockDigit), 64))
	fixture.archive.BlockHash = strings.ToLower(fixture.input.Event.Log.BlockHash.Hex())
	fixture.project.OnchainLocked = false
	fixture.project.Status = "Set"
	if err := database.Omit("FinalTotalRaisedRaw", "FinalSaleTokenAmountRaw").Create(&fixture.project).Error; err != nil {
		t.Fatal(err)
	}
	fixture.admission.ProjectRowID = fixture.project.ID
	if err := database.Create(&fixture.admission).Error; err != nil {
		t.Fatal(err)
	}
	projectTokenBinding := fairLaunchTokenBindingFixture(fixture, fixture.admission.ProjectTokenAddress, fixture.admission.ProjectTokenDecimals, fixture.admission.ProjectTokenRuntimeCodeHash)
	projectTokenBinding.RunRowID = activation.AuthorityRowID
	projectTokenBinding.SourceID = activation.SourceID
	if err := database.Where("run_row_id=? AND chain_id=56 AND source_id=? AND contract_address=? AND token_address=?", projectTokenBinding.RunRowID, projectTokenBinding.SourceID, projectTokenBinding.ContractAddress, projectTokenBinding.TokenAddress).FirstOrCreate(&projectTokenBinding).Error; err != nil {
		t.Fatal(err)
	}
	identity := entity.ChainEventIdentityV2{ChainID: 56, TxHash: strings.ToLower(fixture.input.Event.Log.TxHash.Hex()), TxLogOrdinal: 2, CreatedAt: fixture.now}
	if err := database.Create(&identity).Error; err != nil {
		t.Fatal(err)
	}
	canonical := uint8(1)
	placement := entity.ChainEventPlacementV2{EventIdentityID: identity.ID, Generation: 1, BlockNumber: 100, BlockHash: fixture.archive.BlockHash, TransactionIndex: 1, RPCLogIndex: 2, SourceRevisionID: activation.SourceRevisionID, CanonicalSlot: &canonical, FinalizedAt: &fixture.now}
	if err := database.Create(&placement).Error; err != nil {
		t.Fatal(err)
	}
	observation := entity.ChainEventPlacementObservationV2{PlacementID: placement.ID, EventIdentityID: identity.ID, ProviderIdentity: "provider-a", EmittingAddress: addressMainnet('a'), TopicsDataDigest: strings.Repeat("d", 64), RawLogDigest: strings.Repeat("e", 64), ObservedAt: fixture.now}
	if err := database.Create(&observation).Error; err != nil {
		t.Fatal(err)
	}
	fixture.archive.IdentityID, fixture.archive.PlacementID, fixture.archive.ObservationID = identity.ID, placement.ID, observation.ID
	return fixture
}

func assertFairLaunchProjectCreatedMySQLCounts(t *testing.T, database *gorm.DB, expected string) {
	t.Helper()
	var value string
	if err := database.Raw(`SELECT CONCAT((SELECT COUNT(*) FROM fair_launch_project_binding_v3),'/',(SELECT COUNT(*) FROM fair_launch_mainnet_project_bootstrap_receipt_v1),'/',(SELECT COUNT(*) FROM fair_launch_project_state_v3),'/',(SELECT COUNT(*) FROM fair_launch_mainnet_watermark_v1 WHERE watermark_kind='first_project_created'),'/',(SELECT COUNT(*) FROM projects WHERE funding_mode='fair_launch' AND onchain_locked=1))`).Scan(&value).Error; err != nil {
		t.Fatal(err)
	}
	if value != expected {
		t.Fatalf("bootstrap counts got=%s want=%s", value, expected)
	}
}

func newBigUint(value uint64) *big.Int { return new(big.Int).SetUint64(value) }
