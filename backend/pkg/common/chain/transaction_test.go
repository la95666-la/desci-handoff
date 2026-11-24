package chain

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type fakeTransactionSender struct{ err error }

func (sender fakeTransactionSender) SendTransaction(context.Context, *types.Transaction) error {
	return sender.err
}

func TestBroadcastSignedTransactionAlwaysReturnsDeterministicHash(t *testing.T) {
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    7,
		To:       ptrAddress(common.HexToAddress("0x1111111111111111111111111111111111111111")),
		Gas:      21_000,
		GasPrice: big.NewInt(1),
	})
	wantHash := tx.Hash().Hex()

	gotHash, err := broadcastSignedTransaction(context.Background(), fakeTransactionSender{}, tx)
	if err != nil || gotHash != wantHash {
		t.Fatalf("successful broadcast hash=%s err=%v want=%s", gotHash, err, wantHash)
	}

	broadcastErr := errors.New("rpc timeout after accept")
	gotHash, err = broadcastSignedTransaction(context.Background(), fakeTransactionSender{err: broadcastErr}, tx)
	if gotHash != wantHash {
		t.Fatalf("ambiguous broadcast lost deterministic hash: got=%s want=%s", gotHash, wantHash)
	}
	if !errors.Is(err, broadcastErr) {
		t.Fatalf("ambiguous broadcast error=%v want wrapped %v", err, broadcastErr)
	}
}

func ptrAddress(address common.Address) *common.Address { return &address }

func TestBufferedGasLimit(t *testing.T) {
	for _, test := range []struct{ estimate, want uint64 }{
		{1, 2}, {10, 13}, {77_548, 100_813}, {94_920, 123_396},
		{math.MaxUint64 / 13 * 10, math.MaxUint64 / 13 * 13},
	} {
		got, err := bufferedGasLimit(test.estimate)
		if err != nil || got != test.want {
			t.Errorf("estimate %d: gas limit %d, error %v; want %d", test.estimate, got, err, test.want)
		}
	}
	for _, estimate := range []uint64{0, math.MaxUint64} {
		if got, err := bufferedGasLimit(estimate); err == nil || got != 0 {
			t.Errorf("estimate %d must fail closed, got %d %v", estimate, got, err)
		}
	}
}
