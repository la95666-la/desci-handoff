package chaintx

import (
	"math/big"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

func v3ReconcileFixture(t *testing.T) (entity.ChainTxSubmission, idov3whitelist.ExpectedProjectBinding, common.Address, V3ReceiptAudit) {
	t.Helper()
	proxy := common.HexToAddress("0x2222222222222222222222222222222222222222")
	operator := common.HexToAddress("0x3333333333333333333333333333333333333333")
	data, err := basechain.PackV3Lifecycle("activateProject", big.NewInt(42))
	if err != nil {
		t.Fatal(err)
	}
	slot, nonce := uint8(1), uint64(7)
	hash := common.HexToHash("0x1234").Hex()
	row := entity.ChainTxSubmission{ID: 19, ProjectID: 42, ChainID: 31337,
		Action: "ido_v3_activate", ContractAddress: proxy.Hex(), SenderAddress: operator.Hex(),
		CallData: data, RawTx: []byte{1, 2, 3}, TxHash: &hash, Nonce: &nonce,
		ActiveSlot: &slot, State: constant.ChainTxStateSubmitted}
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 42, ProxyAddress: proxy}
	audit := V3ReceiptAudit{Hash: common.HexToHash(hash), Nonce: nonce, Block: 91,
		BlockHash: common.HexToHash("0xabcd"), GasUsed: 120000}
	return row, binding, operator, audit
}

func v3SubmissionSQLRow(row entity.ChainTxSubmission, state string, audit V3ReceiptAudit) *sqlmock.Rows {
	columns := []string{"id", "project_id", "action", "chain_id", "sender_address", "contract_address", "call_data", "raw_tx", "tx_hash", "nonce", "active_slot", "state", "receipt_status", "receipt_block_number", "receipt_block_hash"}
	var status, block interface{}
	blockHash := ""
	if state == constant.ChainTxStateFinalSuccess || state == constant.ChainTxStateFinalRevert {
		status, block, blockHash = uint64(1), audit.Block, audit.BlockHash.Hex()
		if state == constant.ChainTxStateFinalRevert {
			status = uint64(0)
		}
	}
	var slot interface{} = uint8(1)
	if state == constant.ChainTxStateFinalRevert {
		slot = nil
	}
	return sqlmock.NewRows(columns).AddRow(row.ID, row.ProjectID, row.Action, row.ChainID, row.SenderAddress, row.ContractAddress,
		row.CallData, row.RawTx, *row.TxHash, *row.Nonce, slot, state, status, block, blockHash)
}

func expectV3FinalizationReads(mock sqlmock.Sqlmock, row entity.ChainTxSubmission, audit V3ReceiptAudit, activeID, nextNonce uint64) {
	mock.ExpectQuery("SELECT .* FROM `chain_event_sync_state`.*FOR UPDATE").WithArgs(1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `chain_event_reorg_incident`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT .* FROM `chain_tx_submission`.*FOR UPDATE").WithArgs(row.ID, 1).WillReturnRows(v3SubmissionSQLRow(row, row.State, audit))
	mock.ExpectQuery("SELECT .* FROM `chain_relayer_nonce`.*FOR UPDATE").WithArgs(row.ChainID, row.SenderAddress, 1).
		WillReturnRows(sqlmock.NewRows([]string{"chain_id", "sender_address", "next_nonce", "active_submission_id"}).
			AddRow(row.ChainID, row.SenderAddress, nextNonce, activeID))
}

func TestV3FinalizationAtomicReceiptAndNonceRelease(t *testing.T) {
	for _, reverted := range []bool{false, true} {
		t.Run(map[bool]string{false: "success-held", true: "revert-released"}[reverted], func(t *testing.T) {
			db, mock := shutdownMockDB(t)
			row, binding, operator, audit := v3ReconcileFixture(t)
			audit.Reverted = reverted
			mock.ExpectBegin()
			expectV3FinalizationReads(mock, row, audit, row.ID, 8)
			mock.ExpectExec(regexp.QuoteMeta("UPDATE `chain_tx_submission` SET")).WillReturnResult(sqlmock.NewResult(0, 1))
			state := constant.ChainTxStateFinalSuccess
			if reverted {
				state = constant.ChainTxStateFinalRevert
				mock.ExpectExec(regexp.QuoteMeta("UPDATE `chain_relayer_nonce` SET")).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectQuery("SELECT .* FROM `chain_tx_submission` WHERE id = ").WithArgs(row.ID, 1).
				WillReturnRows(v3SubmissionSQLRow(row, state, audit))
			mock.ExpectCommit()
			var got entity.ChainTxSubmission
			err := db.Transaction(func(tx *gorm.DB) error {
				var e error
				got, e = finalizeV3LifecycleReceiptTx(tx, row, binding, operator, audit)
				return e
			})
			if err != nil || got.State != state {
				t.Fatalf("got=%+v err=%v", got, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestV3FinalizationFenceDriftAndFailedWriteRollback(t *testing.T) {
	for _, tc := range []string{"wrong-active-sender", "nonce-cursor-drift", "receipt-write-failed", "release-failed"} {
		t.Run(tc, func(t *testing.T) {
			db, mock := shutdownMockDB(t)
			row, binding, operator, audit := v3ReconcileFixture(t)
			audit.Reverted = true
			mock.ExpectBegin()
			activeID := row.ID
			if tc == "wrong-active-sender" {
				activeID++
			}
			nextNonce := uint64(8)
			if tc == "nonce-cursor-drift" {
				nextNonce++
			}
			expectV3FinalizationReads(mock, row, audit, activeID, nextNonce)
			if tc != "wrong-active-sender" && tc != "nonce-cursor-drift" {
				result := sqlmock.NewResult(0, 1)
				if tc == "receipt-write-failed" {
					result = sqlmock.NewResult(0, 0)
				}
				mock.ExpectExec(regexp.QuoteMeta("UPDATE `chain_tx_submission` SET")).WillReturnResult(result)
			}
			if tc == "release-failed" {
				mock.ExpectExec(regexp.QuoteMeta("UPDATE `chain_relayer_nonce` SET")).WillReturnResult(sqlmock.NewResult(0, 0))
			}
			mock.ExpectRollback()
			err := db.Transaction(func(tx *gorm.DB) error {
				_, e := finalizeV3LifecycleReceiptTx(tx, row, binding, operator, audit)
				return e
			})
			if err == nil {
				t.Fatal("failure not fenced")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
