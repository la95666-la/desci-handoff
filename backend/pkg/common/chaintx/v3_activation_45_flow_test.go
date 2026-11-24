package chaintx

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func recoveryFlowDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return database, mock
}

func TestActivation45TransactionGatesOneOriginalSend(t *testing.T) {
	hash := common.HexToHash("0xbd41c9a1171b7a360d27194313f976ff5c0ac2560e675b03681ed3adb891f54f")
	for _, tc := range []struct {
		name         string
		execute      bool
		markError    error
		commitError  error
		sendError    error
		wantSend, ok bool
	}{
		{"dry-run rolls back", false, errActivation45DryRun, nil, nil, false, true},
		{"marker rejected", true, errors.New("marker rejected"), nil, nil, false, false},
		{"commit unknown", true, nil, errors.New("commit unknown"), nil, false, false},
		{"committed then one send", true, nil, nil, nil, true, true},
		{"committed then provider unknown", true, nil, nil, errors.New("provider result unknown"), true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database, mock := recoveryFlowDB(t)
			mock.ExpectBegin()
			if tc.markError != nil {
				mock.ExpectRollback()
			} else if tc.commitError != nil {
				mock.ExpectCommit().WillReturnError(tc.commitError)
			} else {
				mock.ExpectCommit()
			}
			calls := 0
			err := finishActivation45(context.Background(), database, tc.execute, hash,
				func(*gorm.DB) error { return tc.markError },
				func() error { calls++; return tc.sendError })
			if (err == nil) != tc.ok || (calls == 1) != tc.wantSend || calls > 1 {
				t.Fatalf("result=%v sends=%d", err, calls)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestActivation45DryRunRollsBackAfterSignedRowValidation(t *testing.T) {
	row, binding, hash, data := signedActivation45Fixture(t)
	database, mock := recoveryFlowDB(t)
	mock.ExpectBegin()
	mock.ExpectRollback()
	sends := 0
	err := finishActivation45(context.Background(), database, false, hash,
		func(*gorm.DB) error {
			if err := verifyActivation45SignedRowForSender(row, binding, data, hash,
				common.HexToAddress(row.SenderAddress), new(types.Transaction)); err != nil {
				return err
			}
			return errActivation45DryRun
		}, func() error { sends++; return nil })
	if err != nil || sends != 0 {
		t.Fatalf("dry-run wrote or sent: err=%v sends=%d", err, sends)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestActivation45MarkerIsConditionalAndOneShot(t *testing.T) {
	row, binding, _, _ := signedActivation45Fixture(t)
	for _, tc := range []struct {
		name string
		rows int64
		want bool
	}{
		{"one original row", 1, true},
		{"already marked or competing write", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database, mock := recoveryFlowDB(t)
			mock.ExpectBegin()
			mock.ExpectExec("UPDATE .*chain_tx_submission.*SET").
				WillReturnResult(sqlmock.NewResult(0, tc.rows))
			if tc.want {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			err := database.Transaction(func(tx *gorm.DB) error {
				return markActivation45OriginalTx(tx, row, binding)
			})
			if (err == nil) != tc.want {
				t.Fatalf("marker result=%v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
	row.State = "broadcast_unknown"
	database, mock := recoveryFlowDB(t)
	if err := markActivation45OriginalTx(database, row, binding); err == nil {
		t.Fatal("already sent state accepted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestActivation45RealMarkerCommitsBeforeSendAndCannotBeReused(t *testing.T) {
	row, binding, _, _ := signedActivation45Fixture(t)
	// The hash guard belongs to the send boundary, not to the database marker.
	// A synthetic signature keeps this test off all production infrastructure.
	hash := common.HexToHash("0xbd41c9a1171b7a360d27194313f976ff5c0ac2560e675b03681ed3adb891f54f")
	database, mock := recoveryFlowDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .*chain_tx_submission.*SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	committed, sends := false, 0
	err := finishActivation45(context.Background(), database, true, hash,
		func(tx *gorm.DB) error { return markActivation45OriginalTx(tx, row, binding) },
		func() error {
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("provider called before marker commit: %v", err)
			}
			sends++
			committed = true
			return errors.New("ambiguous network response")
		})
	if err == nil || !committed || sends != 1 {
		t.Fatalf("marker/send ordering or single-send violated: err=%v sends=%d", err, sends)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	// The next invocation sees a lost compare-and-swap and cannot send.
	database, mock = recoveryFlowDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .*chain_tx_submission.*SET").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	if err := finishActivation45(context.Background(), database, true, hash,
		func(tx *gorm.DB) error { return markActivation45OriginalTx(tx, row, binding) },
		func() error { sends++; return nil }); err == nil || sends != 1 {
		t.Fatal("competing invocation sent a second transaction")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
