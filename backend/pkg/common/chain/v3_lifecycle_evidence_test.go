package chain

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"joincare_server/pkg/common/idov3whitelist"
)

type lifecycleEvidenceStub struct {
	chain                                    *big.Int
	block, head                              *types.Header
	receipt                                  *types.Receipt
	reads, receiptReads                      int
	changeAnchor, changeReceipt, failReceipt bool
}

func (s *lifecycleEvidenceStub) ChainID(context.Context) (*big.Int, error) { return s.chain, nil }
func (s *lifecycleEvidenceStub) BlockNumber(context.Context) (uint64, error) {
	return s.head.Number.Uint64(), nil
}
func (s *lifecycleEvidenceStub) HeaderByNumber(_ context.Context, n *big.Int) (*types.Header, error) {
	if n.Cmp(s.block.Number) == 0 {
		s.reads++
		if s.changeAnchor && s.reads == 2 {
			return &types.Header{Number: n, Extra: []byte("replacement")}, nil
		}
		return s.block, nil
	}
	return s.head, nil
}
func (s *lifecycleEvidenceStub) TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error) {
	s.receiptReads++
	if s.failReceipt && s.receiptReads == 2 {
		return nil, errors.New("sensitive RPC failure")
	}
	if s.changeReceipt && s.receiptReads == 2 {
		copyOfReceipt := *s.receipt
		copyOfReceipt.Status = types.ReceiptStatusFailed
		return &copyOfReceipt, nil
	}
	return s.receipt, nil
}

func TestReconcileV3LifecycleObservationRemainsTransient(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	payment := common.HexToAddress("0x2222222222222222222222222222222222222222")
	projectToken := common.HexToAddress("0x3333333333333333333333333333333333333333")
	user := common.HexToAddress("0x4444444444444444444444444444444444444444")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7, PaymentToken: payment, ProjectToken: projectToken}
	block, head := &types.Header{Number: big.NewInt(90)}, &types.Header{Number: big.NewInt(100)}
	txHash := common.HexToHash("0x66")
	log := &types.Log{Address: proxy, TxHash: txHash, BlockNumber: 90, BlockHash: block.Hash(), Index: 5,
		Topics: []common.Hash{crypto.Keccak256Hash([]byte("Refunded(uint256,address,address,uint256)")), common.BigToHash(big.NewInt(7)), common.BytesToHash(user.Bytes()), common.BytesToHash(payment.Bytes())},
		Data:   common.LeftPadBytes(big.NewInt(100).Bytes(), 32)}
	s := &lifecycleEvidenceStub{chain: big.NewInt(31337), block: block, head: head,
		receipt: &types.Receipt{Status: types.ReceiptStatusSuccessful, BlockNumber: big.NewInt(90), BlockHash: block.Hash(), TxHash: txHash, Logs: []*types.Log{log}}}
	check := func(expected idov3whitelist.ExpectedProjectBinding, depth uint64) (V3LifecycleObservation, error) {
		s.reads, s.receiptReads = 0, 0
		return ReconcileV3LifecycleObservation(context.Background(), s, expected, txHash, 5, depth)
	}
	got, err := check(binding, 11)
	if err != nil || got.Event.Kind != "Refunded" || got.Event.AmountRaw != "100" || got.ConfirmedHead != 100 || got.HeadHash != head.Hash() || s.receiptReads != 2 {
		t.Fatalf("valid read-only lifecycle observation rejected: %+v %v", got, err)
	}
	if _, err := check(binding, 12); err == nil {
		t.Fatal("insufficient confirmations accepted")
	}
	if _, err := check(binding, 0); err == nil {
		t.Fatal("zero depth accepted")
	}
	bad := binding
	bad.PaymentToken = projectToken
	if _, err := check(bad, 11); err == nil {
		t.Fatal("wrong token binding accepted")
	}
	s.chain = big.NewInt(56)
	if _, err := check(binding, 11); err == nil {
		t.Fatal("wrong chain accepted")
	}
	s.chain = big.NewInt(31337)
	s.changeAnchor = true
	if _, err := check(binding, 11); err == nil {
		t.Fatal("in-flight reorg accepted")
	}
	s.changeAnchor = false
	s.changeReceipt = true
	if _, err := check(binding, 11); err == nil {
		t.Fatal("second receipt status change accepted")
	}
	s.changeReceipt = false
	s.failReceipt = true
	if _, err := check(binding, 11); err == nil || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("raw RPC error leaked or accepted: %v", err)
	}
	s.failReceipt = false
	s.receipt.Status = types.ReceiptStatusFailed
	if _, err := check(binding, 11); err == nil {
		t.Fatal("failed receipt accepted")
	}
	s.receipt.Status = types.ReceiptStatusSuccessful
	if _, err := ReconcileV3LifecycleObservation(context.Background(), s, binding, txHash, 6, 11); err == nil {
		t.Fatal("wrong log index accepted")
	}
}
