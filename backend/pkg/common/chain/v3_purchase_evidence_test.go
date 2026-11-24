package chain

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type v3PurchaseRPCStub struct {
	chain       *big.Int
	head        uint64
	receipt     *types.Receipt
	blockHeader *types.Header
	headHeader  *types.Header
	blockReads  int
	headReads   int
	reorgBlock  bool
	reorgHead   bool
	fail        bool
}

func (s *v3PurchaseRPCStub) ChainID(context.Context) (*big.Int, error)   { return s.chain, nil }
func (s *v3PurchaseRPCStub) BlockNumber(context.Context) (uint64, error) { return s.head, nil }
func (s *v3PurchaseRPCStub) TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error) {
	if s.fail {
		return nil, errors.New("sensitive RPC endpoint")
	}
	return s.receipt, nil
}
func (s *v3PurchaseRPCStub) HeaderByNumber(_ context.Context, number *big.Int) (*types.Header, error) {
	if number.Uint64() == s.receipt.BlockNumber.Uint64() {
		s.blockReads++
		if s.reorgBlock && s.blockReads == 2 {
			return &types.Header{Number: number, Extra: []byte("replacement")}, nil
		}
		return s.blockHeader, nil
	}
	s.headReads++
	if s.reorgHead && s.headReads == 2 {
		return &types.Header{Number: number, Extra: []byte("replacement")}, nil
	}
	return s.headHeader, nil
}

func TestReconcileV3PurchaseObservationIsReadOnlyAndReorgSensitive(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	binding := V3PurchaseBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 99,
		PaymentToken: common.HexToAddress("0x3333333333333333333333333333333333333333"),
		ProjectToken: common.HexToAddress("0x4444444444444444444444444444444444444444")}
	blockHeader := &types.Header{Number: big.NewInt(90)}
	headHeader := &types.Header{Number: big.NewInt(100)}
	hash := common.HexToHash("0x43")
	log := &types.Log{Address: proxy, BlockNumber: 90, BlockHash: blockHeader.Hash(), TxHash: hash, Index: 5,
		Topics: []common.Hash{v3PurchasedTopic, common.BigToHash(big.NewInt(99)), common.BytesToHash(common.HexToAddress("0x2222222222222222222222222222222222222222").Bytes()), common.BytesToHash(binding.PaymentToken.Bytes())}, Data: make([]byte, 96)}
	log.Data[31] = 100
	copy(log.Data[44:64], binding.ProjectToken.Bytes())
	log.Data[95] = 10
	receipt := &types.Receipt{Status: types.ReceiptStatusSuccessful, BlockNumber: big.NewInt(90), BlockHash: log.BlockHash, TxHash: hash, Logs: []*types.Log{log}}
	stub := &v3PurchaseRPCStub{chain: big.NewInt(31337), head: 100, receipt: receipt, blockHeader: blockHeader, headHeader: headHeader}
	check := func(depth uint64) (V3PurchaseObservation, error) {
		stub.blockReads, stub.headReads = 0, 0
		return ReconcileV3PurchaseObservation(context.Background(), stub, binding, hash, 5, depth)
	}
	got, err := check(11)
	if err != nil || got.ConfirmedHead != 100 || got.HeadHash != headHeader.Hash() || got.Purchase.PaymentRaw != "100" {
		t.Fatalf("valid observation rejected: %+v %v", got, err)
	}
	if _, err := check(12); err == nil {
		t.Fatal("insufficient confirmations accepted")
	}
	if _, err := check(0); err == nil {
		t.Fatal("zero depth accepted")
	}
	stub.chain = big.NewInt(56)
	if _, err := check(1); err == nil {
		t.Fatal("wrong chain accepted")
	}
	stub.chain = big.NewInt(31337)
	stub.head = 89
	if _, err := check(1); err == nil {
		t.Fatal("receipt after head accepted")
	}
	stub.head = 100
	receipt.BlockHash = common.HexToHash("0x55")
	if _, err := check(1); err == nil {
		t.Fatal("orphaned receipt accepted")
	}
	receipt.BlockHash = log.BlockHash
	if _, err := ReconcileV3PurchaseObservation(context.Background(), stub, binding, hash, 6, 1); err == nil {
		t.Fatal("wrong index accepted")
	}
	receipt.Status = types.ReceiptStatusFailed
	if _, err := check(1); err == nil {
		t.Fatal("reverted receipt accepted")
	}
	receipt.Status = types.ReceiptStatusSuccessful
	stub.reorgBlock = true
	if _, err := check(1); err == nil {
		t.Fatal("mid-read receipt reorg accepted")
	}
	stub.reorgBlock = false
	stub.reorgHead = true
	if _, err := check(1); err == nil {
		t.Fatal("mid-read head reorg accepted")
	}
	stub.reorgHead = false
	stub.fail = true
	if _, err := check(1); err == nil || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("raw RPC failure leaked or accepted: %v", err)
	}
}
