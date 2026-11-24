package chaintx

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
)

func shutdownMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db, mock
}

func expectSubmissionIdentity(mock sqlmock.Sqlmock, id uint64, action, state string) {
	rows := sqlmock.NewRows([]string{"id", "action", "state", "chain_id", "sender_address", "contract_address"}).AddRow(id, action, state, 31337, "0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `chain_tx_submission` WHERE id = ? LIMIT ?")).WithArgs(id, 1).WillReturnRows(rows)
}

func TestFairLaunchTransactionClassifierIncludesCurrentAndFutureActions(t *testing.T) {
	for _, action := range []string{constant.TransactionTypeFairLaunchCreate, constant.TransactionTypeFairLaunchEnd, "fair_launch_future_governance", " FAIR_LAUNCH_FUTURE "} {
		if !IsFairLaunchTransactionAction(action) {
			t.Fatalf("not classified: %q", action)
		}
	}
	for _, action := range []string{constant.TransactionTypeV2Create, "project_end", "fair-launch-end", ""} {
		if IsFairLaunchTransactionAction(action) {
			t.Fatalf("misclassified: %q", action)
		}
	}
}

func TestFairLaunchProgressionRejectsAfterIdentityReadWithoutMutation(t *testing.T) {
	for _, test := range []struct {
		name string
		call func(*gorm.DB) error
	}{
		{"prepare", func(db *gorm.DB) error { _, err := Prepare(context.Background(), db, nil, 7); return err }},
		{"dispatch", func(db *gorm.DB) error { _, err := Dispatch(context.Background(), db, nil, 7); return err }},
		{"reconcile", func(db *gorm.DB) error { _, err := ReconcileReceipt(context.Background(), db, nil, 7, 1); return err }},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock := shutdownMockDB(t)
			expectSubmissionIdentity(mock, 7, "fair_launch_future_action", constant.ChainTxStateReady)
			if err := test.call(db); !errors.Is(err, ErrFairLaunchMaterializationUnavailable) {
				t.Fatalf("err=%v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

type countingGateway struct{ commit, journal int }

func (g *countingGateway) Commit(context.Context, GatewayCommitRequest) (GatewayJournalEntry, error) {
	g.commit++
	return GatewayJournalEntry{}, nil
}
func (g *countingGateway) Journal(context.Context, GatewayJournalKey) (GatewayJournalEntry, error) {
	g.journal++
	return GatewayJournalEntry{}, nil
}

func TestGatewayProgressionRejectsFairLaunchBeforeCallsOrMutation(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(*gorm.DB, *countingGateway) error
	}{
		{"run", func(db *gorm.DB, gateway *countingGateway) error {
			_, err := RunGatewaySigning(context.Background(), db, gateway, SigningReservation{SubmissionID: 7}, []byte{1}, 1, time.Now())
			return err
		}},
		{"recover", func(db *gorm.DB, gateway *countingGateway) error {
			_, err := RecoverGatewaySigning(context.Background(), db, gateway, SigningReservation{SubmissionID: 7}, 1, time.Now())
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := shutdownMockDB(t)
			rows := sqlmock.NewRows([]string{"action"}).AddRow("fair_launch_future_action")
			mock.ExpectQuery(regexp.QuoteMeta("SELECT `action` FROM `chain_tx_submission` WHERE id = ? LIMIT ?")).WithArgs(7, 1).WillReturnRows(rows)
			gateway := &countingGateway{}
			if err := tc.call(db, gateway); !errors.Is(err, ErrFairLaunchMaterializationUnavailable) {
				t.Fatalf("err=%v", err)
			}
			if gateway.commit != 0 || gateway.journal != 0 {
				t.Fatalf("gateway called: %+v", gateway)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDirectProgressionHelpersRejectFairLaunchBeforeMutation(t *testing.T) {
	for _, test := range []struct {
		name  string
		state string
		call  func(*gorm.DB) error
	}{
		{"cancel intent", constant.ChainTxStateIntent, func(db *gorm.DB) error { return CancelIntentTx(db, 7) }},
		{"cancel unsigned", constant.ChainTxStateReady, func(db *gorm.DB) error { return CancelUnsignedTx(db, 7, 9, "operator cancellation") }},
		{"mark ready", constant.ChainTxStateIntent, func(db *gorm.DB) error { return MarkReadyTx(db, 7) }},
		{"mark applied", constant.ChainTxStateFinalSuccess, func(db *gorm.DB) error { return MarkAppliedTx(db, 7, 8) }},
		{"mark signing recovery", ChainTxStateSigningReserved, func(db *gorm.DB) error { return MarkSigningRecoveryTx(db, 7, "gateway uncertain", time.Now()) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock := shutdownMockDB(t)
			if test.name == "cancel unsigned" || test.name == "mark ready" || test.name == "mark applied" {
				rows := sqlmock.NewRows([]string{"id", "action", "state"}).AddRow(7, "fair_launch_future_action", test.state)
				mock.ExpectQuery("SELECT \\* FROM `chain_tx_submission` WHERE id = \\? LIMIT \\? FOR UPDATE").WithArgs(7, 1).WillReturnRows(rows)
			} else {
				rows := sqlmock.NewRows([]string{"id", "action", "state"}).AddRow(7, "fair_launch_future_action", test.state)
				mock.ExpectQuery(regexp.QuoteMeta("SELECT id,action FROM `chain_tx_submission` WHERE id = ? LIMIT ?")).WithArgs(7, 1).WillReturnRows(rows)
			}
			if err := test.call(db); !errors.Is(err, ErrFairLaunchMaterializationUnavailable) {
				t.Fatalf("err=%v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReserveSigningRejectsFairLaunchBeforeNonceFenceMutation(t *testing.T) {
	db, mock := shutdownMockDB(t)
	mock.ExpectBegin()
	rows := sqlmock.NewRows([]string{"id", "action", "state", "chain_id", "sender_address"}).AddRow(7, "fair_launch_future_action", constant.ChainTxStateReady, 31337, "0x1111111111111111111111111111111111111111")
	mock.ExpectQuery("SELECT \\* FROM `chain_tx_submission` WHERE id = \\? LIMIT \\? FOR UPDATE").WithArgs(7, 1).WillReturnRows(rows)
	mock.ExpectRollback()
	err := db.Transaction(func(tx *gorm.DB) error {
		_, err := ReserveSigningTx(tx, 7, common.HexToAddress("0x1111111111111111111111111111111111111111"), "0x"+strings.Repeat("1", 64), "0x"+strings.Repeat("2", 64), time.Now())
		return err
	})
	if !errors.Is(err, ErrFairLaunchMaterializationUnavailable) {
		t.Fatalf("err=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
