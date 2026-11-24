package chaintx

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
)

func TestV3CreationFinalizerRequiresPersistedSignedBeforeSet(t *testing.T) {
	for _, tc := range []struct {
		name string
		at   *time.Time
	}{
		{"missing signing time", nil},
		{"signed at expiry", func() *time.Time { v := time.Unix(150, 0).UTC(); return &v }()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := shutdownMockDB(t)
			row, binding, operator, audit := v3ReconcileFixture(t)
			row.Action = "ido_v3_create"
			row.State = constant.ChainTxStateBroadcastUnknown
			mock.ExpectBegin()
			current := sqlmock.NewRows([]string{"id", "project_id", "action", "chain_id", "sender_address", "contract_address",
				"call_data", "raw_tx", "tx_hash", "nonce", "active_slot", "state", "v3_signed_at"}).
				AddRow(row.ID, row.ProjectID, row.Action, row.ChainID, row.SenderAddress, row.ContractAddress,
					row.CallData, row.RawTx, *row.TxHash, *row.Nonce, 1, row.State, tc.at)
			mock.ExpectQuery("SELECT .* FROM `chain_tx_submission`.*FOR UPDATE").WithArgs(row.ID, 1).WillReturnRows(current)
			mock.ExpectRollback()
			if err := db.Transaction(func(tx *gorm.DB) error {
				_, err := finalizeV3CreationReceiptLockedTx(tx, row, binding, operator, audit, 150)
				return err
			}); err == nil {
				t.Fatal("expired or missing signing time advanced receipt")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
