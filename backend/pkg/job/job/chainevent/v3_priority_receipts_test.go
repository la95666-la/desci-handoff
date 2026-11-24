package chainevent

import (
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/viper"
	"math/big"
	"strings"
	"testing"
)

func TestPriorityReceiptSelectorFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name      string
		count     int
		hash      string
		dbError   bool
		wantError bool
	}{
		{"idle", 0, "", false, false},
		{"one", 1, "0x" + strings.Repeat("a", 64), false, false},
		{"ambiguous", 2, "0x" + strings.Repeat("a", 64), false, true},
		{"malformed", 1, "0x" + strings.Repeat("z", 64), false, true},
		{"no silent zero hash", 1, "0x" + strings.Repeat("0", 64), false, true},
		{"read failed", 0, "", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, m := recoveryMockDB(t)
			exp := m.ExpectQuery("SELECT s.id,s.project_id,s.action,s.tx_hash AS hash")
			if tc.dbError {
				exp.WillReturnError(errors.New("SECRET DB"))
			} else {
				rows := sqlmock.NewRows([]string{"id", "project_id", "action", "hash"})
				for i := 0; i < tc.count; i++ {
					rows.AddRow(i+1, 99, "ido_v3_create", tc.hash)
				}
				exp.WillReturnRows(rows)
			}
			got, err := selectV3PriorityReceipt(context.Background(), db)
			if (err != nil) != tc.wantError {
				t.Fatal(got, err)
			}
			if !tc.wantError && tc.count == 1 && (got == nil || got.ID != 1) {
				t.Fatal(got)
			}
			if tc.count == 0 && got != nil {
				t.Fatal(got)
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

type priorityReceiptStub struct {
	receipt *types.Receipt
	err     error
}

func (s priorityReceiptStub) TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error) {
	return s.receipt, s.err
}
func TestPriorityReceiptAbsentMismatchCannotReachFinalizer(t *testing.T) {
	hash := common.HexToHash("0x1234")
	target := v3PriorityReceiptTarget{ID: 99, ProjectID: 9, Action: "ido_v3_create", Hash: hash.Hex()}
	for _, tc := range []struct {
		name             string
		receipt          *types.Receipt
		rpcErr, finalErr error
		calls            int
		out              string
	}{
		{"missing", nil, nil, nil, 0, "receipt_unavailable"},
		{"network", nil, errors.New("SECRET URL"), nil, 0, "receipt_unavailable"},
		{"different hash", &types.Receipt{TxHash: common.HexToHash("0x1")}, nil, nil, 0, "receipt_mismatch"},
		{"malformed", &types.Receipt{TxHash: hash}, nil, nil, 0, "receipt_mismatch"},
		{"unconfirmed finalizer refuses", &types.Receipt{TxHash: hash, BlockNumber: big.NewInt(90), BlockHash: common.HexToHash("0x2"), Status: 1}, nil, errors.New("not canonical"), 1, "reconcile_rejected"},
		{"formally reconciled revert", &types.Receipt{TxHash: hash, BlockNumber: big.NewInt(90), BlockHash: common.HexToHash("0x2"), Status: 0}, nil, nil, 1, "reconciled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			got := runV3PriorityReceipt(context.Background(), target, priorityReceiptStub{tc.receipt, tc.rpcErr}, func() error { calls++; return tc.finalErr })
			if calls != tc.calls || got != tc.out {
				t.Fatal(calls, got)
			}
		})
	}
}
func TestPriorityReceiptLaneDefaultOffDoesNotReadOrSend(t *testing.T) {
	old := viper.Get("idoV3OriginalReconciliation.enabled")
	defer viper.Set("idoV3OriginalReconciliation.enabled", old)
	viper.Set("idoV3OriginalReconciliation.enabled", false)
	db, m := recoveryMockDB(t)
	j := &ApprovedV3ScannerJob{Database: db}
	j.runPriorityReceipts()
	j.runPriorityReceiptsLocked(context.Background())
	if !j.receiptsNext.IsZero() {
		t.Fatal("disabled lane scheduled writes")
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestPriorityReceiptSharesSingleWriterMutex(t *testing.T) {
	// A scanner-held mutex must not be unlocked by the separate cron lane.
	j := &ApprovedV3ScannerJob{}
	j.mu.Lock()
	j.runPriorityReceipts()
	if j.mu.TryLock() {
		t.Fatal("writer lock released")
	}
	j.mu.Unlock()
}
