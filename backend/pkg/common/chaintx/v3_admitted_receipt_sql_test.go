package chaintx

import (
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

func TestV3AdmittedReceiptStopsAtCanonicalOrProjectGate(t *testing.T) {
	for _, tc := range []struct {
		name, cursor, status string
		checkpoint, project  bool
	}{
		{"halted cursor", "reorg_halted", "", false, false},
		{"missing checkpoint", "running", "", false, false},
		{"wrong project status", "running", "Active", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := shutdownMockDB(t)
			row, binding, operator, audit := v3ReconcileFixture(t)
			binding.ProjectToken = operator
			binding.PaymentToken = binding.ProxyAddress
			hash := "0x" + strings.Repeat("a", 64)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT .* FROM `chain_event_sync_state`.*FOR UPDATE").WithArgs(1).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			mock.ExpectQuery("SELECT count\\(\\*\\) FROM `chain_event_reorg_incident`").
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor")).
				WithArgs(binding.ChainID, strings.ToLower(binding.ProxyAddress.Hex()), binding.ProjectID).
				WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "revision", "state"}).AddRow(90, hash, 1, tc.cursor))
			if tc.cursor == "running" {
				checkpoints := sqlmock.NewRows([]string{"block_hash"})
				if tc.checkpoint {
					checkpoints.AddRow(hash)
				}
				mock.ExpectQuery(regexp.QuoteMeta("SELECT block_hash FROM ido_v3_block_checkpoint")).
					WithArgs(binding.ChainID, strings.ToLower(binding.ProxyAddress.Hex()), binding.ProjectID, uint64(90), hash).
					WillReturnRows(checkpoints)
			}
			if tc.project {
				mock.ExpectQuery(regexp.QuoteMeta("SELECT p.status FROM projects p JOIN ido_v3_project_binding b")).
					WithArgs(binding.ProjectID, binding.ChainID, strings.ToLower(binding.ProxyAddress.Hex()),
						strings.ToLower(binding.ProjectToken.Hex()), strings.ToLower(binding.PaymentToken.Hex()), strings.ToLower(operator.Hex())).
					WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(tc.status))
			}
			mock.ExpectRollback()
			if err := db.Transaction(func(tx *gorm.DB) error {
				_, err := finalizeV3AdmittedLifecycleReceiptTx(tx, row, binding, operator, audit)
				return err
			}); err == nil {
				t.Fatal("receipt passed a halted or mismatched gate")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestV3AdmittedReceiptKeepsLockOrderAndReleasesOnlyConfirmedRevert(t *testing.T) {
	for _, reverted := range []bool{false, true} {
		t.Run(map[bool]string{false: "success-held", true: "revert-released"}[reverted], func(t *testing.T) {
			db, mock := shutdownMockDB(t)
			row, binding, operator, audit := v3ReconcileFixture(t)
			binding.ProjectToken = operator
			binding.PaymentToken = binding.ProxyAddress
			audit.Reverted = reverted
			hash := "0x" + strings.Repeat("a", 64)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT .* FROM `chain_event_sync_state`.*FOR UPDATE").WithArgs(1).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			mock.ExpectQuery("SELECT count\\(\\*\\) FROM `chain_event_reorg_incident`").
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor")).
				WithArgs(binding.ChainID, strings.ToLower(binding.ProxyAddress.Hex()), binding.ProjectID).
				WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "revision", "state"}).AddRow(90, hash, 1, "running"))
			mock.ExpectQuery(regexp.QuoteMeta("SELECT block_hash FROM ido_v3_block_checkpoint")).
				WithArgs(binding.ChainID, strings.ToLower(binding.ProxyAddress.Hex()), binding.ProjectID, uint64(90), hash).
				WillReturnRows(sqlmock.NewRows([]string{"block_hash"}).AddRow(hash))
			mock.ExpectQuery(regexp.QuoteMeta("SELECT p.status FROM projects p JOIN ido_v3_project_binding b")).
				WithArgs(binding.ProjectID, binding.ChainID, strings.ToLower(binding.ProxyAddress.Hex()),
					strings.ToLower(binding.ProjectToken.Hex()), strings.ToLower(binding.PaymentToken.Hex()), strings.ToLower(operator.Hex())).
				WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("Pending"))
			mock.ExpectQuery("SELECT .* FROM `chain_tx_submission`.*FOR UPDATE").WithArgs(row.ID, 1).
				WillReturnRows(v3SubmissionSQLRow(row, row.State, audit))
			mock.ExpectQuery("SELECT .* FROM `chain_relayer_nonce`.*FOR UPDATE").WithArgs(row.ChainID, row.SenderAddress, 1).
				WillReturnRows(sqlmock.NewRows([]string{"chain_id", "sender_address", "next_nonce", "active_submission_id"}).
					AddRow(row.ChainID, row.SenderAddress, 8, row.ID))
			mock.ExpectExec(regexp.QuoteMeta("UPDATE `chain_tx_submission` SET")).WillReturnResult(sqlmock.NewResult(0, 1))
			state := constant.ChainTxStateFinalSuccess
			if reverted {
				state = constant.ChainTxStateFinalRevert
				mock.ExpectExec(regexp.QuoteMeta("UPDATE `chain_relayer_nonce` SET")).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectQuery("SELECT .* FROM `chain_tx_submission` WHERE id = ").WithArgs(row.ID, 1).
				WillReturnRows(v3SubmissionSQLRow(row, state, audit))
			mock.ExpectCommit()
			var updated entity.ChainTxSubmission
			err := db.Transaction(func(tx *gorm.DB) error {
				var err error
				updated, err = finalizeV3AdmittedLifecycleReceiptTx(tx, row, binding, operator, audit)
				return err
			})
			if err != nil || updated.State != state {
				t.Fatalf("state=%s err=%v", updated.State, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
