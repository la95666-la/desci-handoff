package chaintx

import (
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"
	basechain "joincare_server/pkg/common/chain"
	"math/big"
	"testing"
	"time"
)

func TestEnd72SignatureAndIdentityGuards(t *testing.T) {
	row, b, _, _ := signedActivation45Fixture(t)
	b.ProjectID = End72Project
	row.ID = 72
	row.ProjectID = End72Project
	row.Action = "ido_v3_end"
	row.AttemptNo = 1
	nonce := uint64(56)
	gas := uint64(61838)
	row.Nonce = &nonce
	row.GasLimit = &gas
	now := time.Now()
	row.V3SignedAt = &now
	data, e := basechain.PackV3Lifecycle("endProject", new(big.Int).SetUint64(End72Project))
	if e != nil {
		t.Fatal(e)
	}
	key, _ := crypto.HexToECDSA("1111111111111111111111111111111111111111111111111111111111111111")
	signed, e := types.SignTx(types.NewTx(&types.LegacyTx{Nonce: 56, Gas: 61838, GasPrice: big.NewInt(50000000), To: &b.ProxyAddress, Value: big.NewInt(0), Data: data}), types.NewEIP155Signer(big.NewInt(56)), key)
	if e != nil {
		t.Fatal(e)
	}
	row.RawTx, _ = signed.MarshalBinary()
	h := signed.Hash()
	hash := h.Hex()
	row.TxHash = &hash
	row.CallData = data
	sender := common.HexToAddress(row.SenderAddress)
	if verifyEnd72SignedRow(row, b, data, h, sender, new(types.Transaction)) != nil {
		t.Fatal("valid synthetic fixture rejected")
	}
	for name, change := range map[string]func(){"broadcast": func() { row.BroadcastCount = 1 }, "id": func() { row.ID = 73 }, "state": func() { row.State = "submitted" }, "nonce": func() { n := uint64(57); row.Nonce = &n }, "gas": func() { g := uint64(70000); row.GasLimit = &g }, "signed_at": func() { row.V3SignedAt = nil }, "receipt": func() { n := uint64(1); row.ReceiptStatus = &n }, "bytes": func() { row.RawTx = append([]byte(nil), row.RawTx...); row.RawTx[5] ^= 1 }} {
		before := row
		change()
		if verifyEnd72SignedRow(row, b, data, h, sender, new(types.Transaction)) == nil {
			t.Fatal(name)
		}
		row = before
	}
	if verifyEnd72SignedRow(row, b, data, common.HexToHash(End72Hash), sender, new(types.Transaction)) == nil {
		t.Fatal("foreign production hash accepted")
	}
}
func TestEnd72CommitFenceAndSingleSend(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		execute                     bool
		markErr, commitErr, sendErr error
		calls                       int
		ok                          bool
	}{
		{"dry", false, errEnd72DryRun, nil, nil, 0, true}, {"marker rejected", true, errors.New("rejected"), nil, nil, 0, false}, {"commit unknown", true, nil, errors.New("unknown"), nil, 0, false}, {"one send", true, nil, nil, nil, 1, true}, {"ambiguous send", true, nil, nil, errors.New("unknown"), 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, m := recoveryFlowDB(t)
			m.ExpectBegin()
			if tc.markErr != nil {
				m.ExpectRollback()
			} else if tc.commitErr != nil {
				m.ExpectCommit().WillReturnError(tc.commitErr)
			} else {
				m.ExpectCommit()
			}
			calls := 0
			e := finishEnd72(context.Background(), db, tc.execute, func(*gorm.DB) error { return tc.markErr }, func() error {
				if e := m.ExpectationsWereMet(); e != nil {
					t.Fatal("send before commit", e)
				}
				calls++
				return tc.sendErr
			})
			if (e == nil) != tc.ok || calls != tc.calls {
				t.Fatal(e, calls)
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestEnd72MarkerCAS(t *testing.T) {
	row, b, _, _ := signedActivation45Fixture(t)
	row.ID = 72
	row.ProjectID = End72Project
	b.ProjectID = End72Project
	row.State = "signed"
	h := End72Hash
	row.TxHash = &h
	n := uint64(56)
	row.Nonce = &n
	for _, count := range []int64{0, 1} {
		db, m := recoveryFlowDB(t)
		m.ExpectBegin()
		m.ExpectExec("UPDATE .*chain_tx_submission.*SET").WillReturnResult(sqlmock.NewResult(0, count))
		if count == 1 {
			m.ExpectCommit()
		} else {
			m.ExpectRollback()
		}
		e := db.Transaction(func(tx *gorm.DB) error { return markEnd72Tx(tx, row, b) })
		if (e == nil) != (count == 1) {
			t.Fatal(e)
		}
		if e := m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}
