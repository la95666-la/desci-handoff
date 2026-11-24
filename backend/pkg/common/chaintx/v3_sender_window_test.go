package chaintx

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"testing"
)

func TestSenderWindowIsSchedulingOnlyAndUsesSQLUTC(t *testing.T) {
	for _, kind := range []string{"fresh", "expired", "missing", "revoked", "active"} {
		t.Run(kind, func(t *testing.T) {
			d, m := recoveryFlowDB(t)
			m.ExpectBegin()
			var slot any
			if kind == "active" {
				slot = uint64(96)
			}
			m.ExpectQuery("SELECT active_submission_id FROM chain_relayer_nonce").WillReturnRows(sqlmock.NewRows([]string{"active_submission_id"}).AddRow(slot))
			state := any("valid")
			remaining := any(int64(120))
			if kind == "expired" {
				remaining = int64(-10)
			}
			if kind == "missing" {
				state = nil
				remaining = nil
			}
			if kind == "revoked" {
				state = "revoked"
			}
			m.ExpectQuery("SELECT e.state,TIMESTAMPDIFF\\(SECOND,UTC_TIMESTAMP").WillReturnRows(sqlmock.NewRows([]string{"state", "remaining"}).AddRow(state, remaining))
			m.ExpectCommit()
			out, e := ReadV3SenderMaintenanceWindow(context.Background(), d, 56, common.HexToAddress("0x123"))
			if e != nil {
				t.Fatal(e)
			}
			if (kind == "missing" || kind == "revoked") != (out.RemainingSeconds == nil) {
				t.Fatal(out)
			}
			if (kind == "active") != (out.ActiveSubmission != nil) {
				t.Fatal(out)
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
