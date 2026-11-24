package chainevent

import (
	"math/big"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	commonchain "joincare_server/pkg/common/chain"
)

func TestFairLaunchEndedProjectEffectUsesFinalRaisedNotReason(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	event := &commonchain.FairLaunchTypedEvent{
		Name: "FairLaunchEnded", ProjectID: big.NewInt(9),
		RawDecimal: map[string]string{"arg2": "2", "arg4": "123456"},
	}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `projects` SET .*`final_total_raised_raw`.*").WithArgs("123456", sqlmock.AnyArg(), uint64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := database.Transaction(func(tx *gorm.DB) error { return applyFairLaunchProjectEffectTx(tx, event, time.Now()) }); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAcquireSourceCursorRejectsInvalidFenceWithoutDatabase(t *testing.T) {
	_, err := AcquireSourceMaterializationCursorTx(nil, 1, 1, FairLaunchCursorFence{}, time.Now())
	if err == nil {
		t.Fatal("expected invalid fence rejection")
	}
}
func TestApplyFairLaunchCERejectsInvalidWriteWithoutDatabase(t *testing.T) {
	_, err := ApplyFairLaunchCEV2Tx(nil, FairLaunchCEWrite{}, time.Now())
	if err == nil {
		t.Fatal("expected invalid write rejection")
	}
}

func TestApplyFairLaunchCEValidWriteIsStaticallyUnavailableBeforeDatabase(t *testing.T) {
	write := validUnavailableFairLaunchCEWrite("FairLaunchContribution")
	_, err := ApplyFairLaunchCEV2Tx(nil, write, time.Now().UTC())
	if err == nil || err.Error() != "fair_launch_materialization_unavailable" {
		t.Fatalf("unexpected disabled materialization result: %v", err)
	}
}

func TestApplyFairLaunchCEAllFrozenEventClassesRemainUnavailableLocally(t *testing.T) {
	// These names intentionally resemble possible local/test admission switches.
	// ApplyFairLaunchCEV2Tx must not consult ordinary environment configuration;
	// enabling persistence requires a separately reviewed implementation change.
	for _, name := range []string{
		"JC_FAIR_LAUNCH_LOCAL_ONLY",
		"JC_FAIR_LAUNCH_CE_V2_PERSISTENCE_ENABLED",
		"FAIR_LAUNCH_CE_V2_PERSISTENCE_ENABLED",
		"JC_FAIR_LAUNCH_TEST_CE_V2_ENABLED",
	} {
		t.Setenv(name, "true")
	}

	for eventName := range commonchain.FairLaunchEventSignatures {
		t.Run(eventName, func(t *testing.T) {
			_, err := ApplyFairLaunchCEV2Tx(nil, validUnavailableFairLaunchCEWrite(eventName), time.Now().UTC())
			if err == nil || err.Error() != "fair_launch_materialization_unavailable" {
				t.Fatalf("event %s escaped static materialization shutdown: %v", eventName, err)
			}
		})
	}
}

func validUnavailableFairLaunchCEWrite(eventName string) FairLaunchCEWrite {
	return FairLaunchCEWrite{
		Event:      &commonchain.FairLaunchTypedEvent{Name: eventName},
		Context:    FairLaunchMaterializationContext{ChainID: 31337, SourceRevisionID: 1, EffectVersion: 1, Canonical: true, Finalized: true},
		SourceID:   1,
		LeaseToken: "local-test-lease",
		FenceEpoch: 1,
	}
}
