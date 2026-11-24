package chainevent

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
)

func recoveryMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return database, mock
}

func TestRecoveryReadySelectionSkipsFairLaunchVariantsWithoutBlockingV2(t *testing.T) {
	database, mock := recoveryMockDB(t)
	query := "SELECT * FROM `chain_tx_submission` WHERE chain_id = ? AND sender_address = ? AND active_slot = 1 AND state = ? AND LOWER(TRIM(action)) NOT LIKE ? AND LOWER(TRIM(action)) NOT LIKE ? AND LOWER(TRIM(action)) NOT LIKE ? ORDER BY id ASC LIMIT ?"
	rows := sqlmock.NewRows([]string{"id", "action", "state", "chain_id", "sender_address"}).AddRow(12, constant.TransactionTypeV2Create, constant.ChainTxStateReady, 56, "0x1111111111111111111111111111111111111111")
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(uint64(56), "0x1111111111111111111111111111111111111111", constant.ChainTxStateReady, "fair\\_launch\\_%", "v3\\_%", "ido\\_v3\\_%", 1).WillReturnRows(rows)
	got, err := nextReadyRecoverySubmission(database, 56, "0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 12 || got.Action != constant.TransactionTypeV2Create {
		t.Fatalf("unexpected selection: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestActiveFairLaunchFenceStopsBeforeReadySelection(t *testing.T) {
	database, mock := recoveryMockDB(t)
	id := uint64(7)
	rows := sqlmock.NewRows([]string{"id", "action"}).AddRow(id, "  FAIR_LAUNCH_FUTURE  ")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,action FROM `chain_tx_submission` WHERE id = ? LIMIT ?")).WithArgs(id, 1).WillReturnRows(rows)
	active, found, err := activeRecoverySubmission(database, &id)
	if err != nil || !found {
		t.Fatalf("active lookup=(%+v,%t,%v)", active, found, err)
	}
	if !chaintx.IsFairLaunchTransactionAction(active.Action) {
		t.Fatalf("active action not classified: %q", active.Action)
	}
	// No ready-selection expectation is installed: any query after the active
	// fence classification fails the test, proving the caller can return first.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNoActiveFencePerformsNoQuery(t *testing.T) {
	database, mock := recoveryMockDB(t)
	if _, found, err := activeRecoverySubmission(database, nil); err != nil || found {
		t.Fatalf("found=%t err=%v", found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestActiveV3FenceStopsBeforeRecovery(t *testing.T) {
	for _, action := range []string{" v3_create ", " IDO_V3_future_action "} {
		t.Run(action, func(t *testing.T) {
			database, mock := recoveryMockDB(t)
			id := uint64(8)
			mock.ExpectQuery(regexp.QuoteMeta("SELECT id,action FROM `chain_tx_submission` WHERE id = ? LIMIT ?")).WithArgs(id, 1).
				WillReturnRows(sqlmock.NewRows([]string{"id", "action"}).AddRow(id, action))
			active, found, err := activeRecoverySubmission(database, &id)
			if err != nil || !found || !chaintx.IsIDOV3TransactionAction(active.Action) {
				t.Fatalf("active lookup=(%+v,%t,%v)", active, found, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
