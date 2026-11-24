package chaintx

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	"joincare_server/pkg/common/v3diagnostic"
)

type firstRPC struct {
	nonce                                          uint64
	present, receipt, unknown, simulation, balance bool
	calls                                          int
}

func (f *firstRPC) ChainID(context.Context) (*big.Int, error) { return big.NewInt(56), nil }
func (f *firstRPC) HeaderByNumber(_ context.Context, n *big.Int) (*types.Header, error) {
	if n == nil {
		n = big.NewInt(200)
	}
	return &types.Header{Number: new(big.Int).Set(n)}, nil
}
func (f *firstRPC) NonceAt(context.Context, common.Address, *big.Int) (uint64, error) {
	return f.nonce, nil
}
func (f *firstRPC) PendingNonceAt(context.Context, common.Address) (uint64, error) {
	return f.nonce, nil
}
func (f *firstRPC) BalanceAt(context.Context, common.Address, *big.Int) (*big.Int, error) {
	if f.balance {
		return big.NewInt(0), nil
	}
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(20), nil), nil
}
func (f *firstRPC) TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error) {
	if f.unknown {
		return nil, errors.New("timeout secret")
	}
	if f.receipt {
		return &types.Receipt{}, nil
	}
	return nil, ethereum.NotFound
}
func (f *firstRPC) TransactionByHash(context.Context, common.Hash) (*types.Transaction, bool, error) {
	if f.present {
		return new(types.Transaction), true, nil
	}
	return nil, false, ethereum.NotFound
}
func (f *firstRPC) CallContract(_ context.Context, m ethereum.CallMsg, _ *big.Int) ([]byte, error) {
	f.calls++
	if m.Gas != 74315 || m.GasPrice.String() != "50000000" || m.Value.Sign() != 0 {
		return nil, errors.New("not exact gas")
	}
	if f.simulation {
		return nil, errors.New("revert")
	}
	return nil, nil
}
func TestFirstBroadcastUsesExactOriginalAndIndependentSimulation(t *testing.T) {
	row, b, _, _ := signedActivation45Fixture(t)
	now := time.Now()
	row.V3SignedAt = &now
	for _, tc := range []struct {
		name   string
		change func(*firstRPC)
		want   bool
	}{
		{"valid", func(*firstRPC) {}, true}, {"nonce used", func(f *firstRPC) { f.nonce++ }, false}, {"already pending", func(f *firstRPC) { f.present = true }, false}, {"receipt exists", func(f *firstRPC) { f.receipt = true }, false}, {"unknown not absent", func(f *firstRPC) { f.unknown = true }, false}, {"revert", func(f *firstRPC) { f.simulation = true }, false}, {"balance", func(f *firstRPC) { f.balance = true }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			one, two := &firstRPC{nonce: 39}, &firstRPC{nonce: 39}
			tc.change(two)
			e := CheckV3OriginalFirstBroadcast(context.Background(), row, b, one, two)
			if (e == nil) != tc.want {
				t.Fatal(e)
			}
			if e != nil && (v3diagnostic.Stage(e) == "unavailable" || strings.Contains(e.Error(), "secret")) {
				t.Fatal("missing finite safe detail", e)
			}
		})
	}
	row.BroadcastCount = 1
	if CheckV3OriginalFirstBroadcast(context.Background(), row, b, &firstRPC{nonce: 39}, &firstRPC{nonce: 39}) == nil {
		t.Fatal("second send admitted")
	}
	row.BroadcastCount = 0
	row.GasPriceRaw = "1"
	if CheckV3OriginalFirstBroadcast(context.Background(), row, b, &firstRPC{nonce: 39}, &firstRPC{nonce: 39}) == nil {
		t.Fatal("gas metadata mismatch")
	}
}

type countSender struct {
	calls int
	err   error
	m     sqlmock.Sqlmock
	t     *testing.T
}

func (s *countSender) SendTransaction(context.Context, *types.Transaction) error {
	if e := s.m.ExpectationsWereMet(); e != nil {
		s.t.Fatal("send before COMMIT", e)
	}
	s.calls++
	return s.err
}
func TestPersistedFirstBroadcastTransactionFence(t *testing.T) {
	for _, tc := range []struct {
		name               string
		rows               int64
		commitErr, sendErr error
		calls              int
		ok                 bool
		dry                bool
		revoked            bool
	}{
		{"commit then send", 1, nil, nil, 1, true, false, false}, {"CAS lost", 0, nil, nil, 0, false, false, false}, {"commit ambiguous", 1, errors.New("unknown"), nil, 0, false, false, false}, {"send ambiguous", 1, nil, errors.New("unknown"), 1, false, false, false}, {"dry rollback", 1, nil, nil, 0, true, true, false}, {"permission revoked", 1, nil, nil, 0, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row, b, _, _ := signedActivation45Fixture(t)
			db, m := recoveryFlowDB(t)
			m.ExpectBegin()
			m.ExpectQuery("SELECT .*chain_event_sync_state").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			m.ExpectQuery("SELECT count.*chain_event_reorg_incident").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			m.ExpectQuery("SELECT action FROM chain_tx_submission").WillReturnRows(sqlmock.NewRows([]string{"action"}).AddRow("ido_v3_activate"))
			// Shared running cursor primitive has its own checkpoint fence.
			m.ExpectQuery("SELECT .*ido_v3_scan_cursor").WillReturnRows(sqlmock.NewRows([]string{"state", "cursor_block", "cursor_hash", "revision"}).AddRow("running", 100, common.HexToHash("0x1").Hex(), 7))
			m.ExpectQuery("SELECT .*ido_v3_block_checkpoint").WillReturnRows(sqlmock.NewRows([]string{"block_hash"}).AddRow(common.HexToHash("0x1").Hex()))
			m.ExpectQuery("SELECT p.status,p.onchain_locked").WillReturnRows(sqlmock.NewRows([]string{"status", "onchain_locked"}).AddRow("Pending", 1))
			if tc.revoked {
				m.ExpectRollback()
				sender := &countSender{m: m, t: t}
				_, e := BroadcastV3PersistedOnce(context.Background(), db, sender, b, common.HexToAddress(row.SenderAddress), row.ID, func(*gorm.DB) error { return errors.New("permission revoked") })
				if e == nil || sender.calls != 0 {
					t.Fatal("revoked permission sent")
				}
				if e := m.ExpectationsWereMet(); e != nil {
					t.Fatal(e)
				}
				return
			}
			m.ExpectQuery("SELECT .*chain_tx_submission").WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "chain_id", "action", "state", "contract_address", "sender_address", "active_slot", "nonce", "gas_limit", "gas_price_raw", "value_raw", "raw_tx", "call_data", "tx_hash"}).AddRow(row.ID, row.ProjectID, row.ChainID, row.Action, row.State, row.ContractAddress, row.SenderAddress, 1, *row.Nonce, *row.GasLimit, row.GasPriceRaw, "0", row.RawTx, row.CallData, *row.TxHash))
			m.ExpectQuery("SELECT .*chain_relayer_nonce").WillReturnRows(sqlmock.NewRows([]string{"chain_id", "sender_address", "next_nonce", "active_submission_id"}).AddRow(56, row.SenderAddress, 40, row.ID))
			m.ExpectQuery("SELECT COUNT.*information_schema.TRIGGERS").WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(2))
			m.ExpectQuery("SELECT id,nonce FROM chain_tx_submission").WillReturnRows(sqlmock.NewRows([]string{"id", "nonce"}))
			if !tc.dry {
				m.ExpectExec("UPDATE .*chain_tx_submission.*SET").WillReturnResult(sqlmock.NewResult(0, tc.rows))
			}
			if tc.rows == 0 || tc.dry {
				m.ExpectRollback()
			} else if tc.commitErr != nil {
				m.ExpectCommit().WillReturnError(tc.commitErr)
			} else {
				m.ExpectCommit()
			}
			sender := &countSender{err: tc.sendErr, m: m, t: t}
			var e error
			if tc.dry {
				_, e = PreviewV3PersistedFirstBroadcast(context.Background(), db, b, common.HexToAddress(row.SenderAddress), row.ID, func(*gorm.DB) error { return nil })
			} else {
				_, e = BroadcastV3PersistedOnce(context.Background(), db, sender, b, common.HexToAddress(row.SenderAddress), row.ID, func(*gorm.DB) error { return nil })
			}
			if (e == nil) != tc.ok || sender.calls != tc.calls {
				t.Fatal(e, sender.calls)
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
