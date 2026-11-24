package chaintx

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"
	"testing"
)

func TestConsumedHistoricalProjectsDoNotFenceSender(t *testing.T) {
	for _, tc := range []struct {
		name  string
		nonce any
		want  bool
	}{{"old-halted", uint64(84), true}, {"same-nonce", uint64(85), false}, {"future-nonce", uint64(86), false}, {"unknown-nonce", nil, false}} {
		t.Run(tc.name, func(t *testing.T) {
			d, m := recoveryFlowDB(t)
			rows := sqlmock.NewRows([]string{"id", "nonce"})
			for i := 0; i < 46; i++ {
				rows.AddRow(i+1, tc.nonce)
			}
			m.ExpectQuery("SELECT id,nonce FROM chain_tx_submission").WillReturnRows(rows)
			e := requireConsumedSuspendedTx(d, 56, common.HexToAddress("0x123"), 85)
			if (e == nil) != tc.want {
				t.Fatal(e)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

type isolationRPC struct {
	firstRPC
	confirmed     uint64
	fork, pending bool
}

func (r *isolationRPC) NonceAt(_ context.Context, _ common.Address, n *big.Int) (uint64, error) {
	if n != nil {
		return r.confirmed, nil
	}
	return r.nonce, nil
}
func (r *isolationRPC) PendingNonceAt(context.Context, common.Address) (uint64, error) {
	if r.pending {
		return r.nonce + 1, nil
	}
	return r.nonce, nil
}
func (r *isolationRPC) HeaderByNumber(_ context.Context, n *big.Int) (*types.Header, error) {
	if n == nil {
		n = big.NewInt(200)
	}
	h := &types.Header{Number: new(big.Int).Set(n)}
	if r.fork {
		h.Extra = []byte{1}
	}
	return h, nil
}
func TestIndependentConfirmedSenderNonce(t *testing.T) {
	for _, kind := range []string{"valid", "fork", "disagree", "pending", "drift"} {
		a, b := &isolationRPC{firstRPC: firstRPC{nonce: 85}, confirmed: 85}, &isolationRPC{firstRPC: firstRPC{nonce: 85}, confirmed: 85}
		switch kind {
		case "fork":
			b.fork = true
		case "disagree":
			b.confirmed = 84
		case "pending":
			b.pending = true
		case "drift":
			b.nonce = 86
		}
		n, e := ConfirmV3SenderNonce(context.Background(), 56, common.HexToAddress("0x123"), 85, a, b)
		if (e == nil) != (kind == "valid") || (e == nil && n != 85) {
			t.Fatal(kind, n, e)
		}
	}
}
