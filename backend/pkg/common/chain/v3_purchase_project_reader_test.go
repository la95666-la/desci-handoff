package chain

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"joincare_server/pkg/common/idov3whitelist"
)

type projectPurchaseRPC struct {
	chainID                       *big.Int
	receipt                       *types.Receipt
	block, head                   *types.Header
	proxy, implementation         common.Address
	proxyCode, implementationCode []byte
	projectResult                 []byte
	blockReads, headReads         int
	replaceAfterProject           bool
	failProject                   bool
	receiptReads                  int
	changeReceipt                 bool
	failReceipt                   bool
}

func (s *projectPurchaseRPC) ChainID(context.Context) (*big.Int, error) { return s.chainID, nil }
func (s *projectPurchaseRPC) BlockNumber(context.Context) (uint64, error) {
	return s.head.Number.Uint64(), nil
}
func (s *projectPurchaseRPC) TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error) {
	s.receiptReads++
	if s.failReceipt && s.receiptReads == 2 {
		return nil, errors.New("sensitive receipt RPC response")
	}
	if s.changeReceipt && s.receiptReads == 2 {
		copyOfReceipt := *s.receipt
		copyOfReceipt.Status = types.ReceiptStatusFailed
		return &copyOfReceipt, nil
	}
	return s.receipt, nil
}
func (s *projectPurchaseRPC) HeaderByNumber(_ context.Context, n *big.Int) (*types.Header, error) {
	if n.Cmp(s.block.Number) == 0 {
		s.blockReads++
		if s.replaceAfterProject && s.blockReads >= 5 {
			return &types.Header{Number: n, Extra: []byte("reorg")}, nil
		}
		return s.block, nil
	}
	s.headReads++
	return s.head, nil
}
func (s *projectPurchaseRPC) CodeAt(_ context.Context, a common.Address, _ *big.Int) ([]byte, error) {
	if a == s.proxy {
		return s.proxyCode, nil
	}
	if a == s.implementation {
		return s.implementationCode, nil
	}
	return nil, nil
}
func (s *projectPurchaseRPC) StorageAt(_ context.Context, _ common.Address, _ common.Hash, _ *big.Int) ([]byte, error) {
	return common.LeftPadBytes(s.implementation.Bytes(), 32), nil
}
func (s *projectPurchaseRPC) CallContract(_ context.Context, call ethereum.CallMsg, _ *big.Int) ([]byte, error) {
	if s.failProject {
		return nil, errors.New("sensitive read-only RPC error")
	}
	if call.To == nil || *call.To != s.proxy {
		return nil, errors.New("unexpected contract")
	}
	if len(call.Data) == 4 && string(call.Data) == string(crypto.Keccak256([]byte("contractVersion()"))[:4]) {
		version := crypto.Keccak256Hash([]byte("joincare.ido_v3_stablecoin.v1"))
		return version[:], nil
	}
	if len(call.Data) == 36 && string(call.Data[:4]) == string(crypto.Keccak256([]byte("getWhitelistPolicy(uint256)"))[:4]) {
		return make([]byte, 96), nil
	}
	if len(call.Data) == 36 && string(call.Data[:4]) == string(crypto.Keccak256([]byte("getProject(uint256)"))[:4]) {
		return s.projectResult, nil
	}
	return nil, errors.New("unexpected selector")
}

func TestReconcileV3PurchaseProjectEvidenceRemainsTransientAndPinned(t *testing.T) {
	binding := V3PurchaseBinding{ChainID: 31337, ProxyAddress: common.HexToAddress("0x1111111111111111111111111111111111111111"), ProjectID: 77,
		PaymentToken: common.HexToAddress("0x2222222222222222222222222222222222222222"), ProjectToken: common.HexToAddress("0x3333333333333333333333333333333333333333")}
	block, head := &types.Header{Number: big.NewInt(90)}, &types.Header{Number: big.NewInt(100)}
	txHash := common.HexToHash("0x44")
	log := &types.Log{Address: binding.ProxyAddress, BlockNumber: 90, BlockHash: block.Hash(), TxHash: txHash, Index: 5,
		Topics: []common.Hash{v3PurchasedTopic, common.BigToHash(big.NewInt(77)), common.BytesToHash(common.HexToAddress("0x4444444444444444444444444444444444444444").Bytes()), common.BytesToHash(binding.PaymentToken.Bytes())}, Data: make([]byte, 96)}
	log.Data[31], log.Data[95] = 100, 10
	copy(log.Data[44:64], binding.ProjectToken.Bytes())
	proxyCode, implCode := []byte{0x60, 0x80}, []byte{0x60, 0x01}
	impl := common.HexToAddress("0x5555555555555555555555555555555555555555")
	runtime := idov3whitelist.RuntimeIdentity{Implementation: impl, ProxyCodeHash: crypto.Keccak256Hash(proxyCode), CodeHash: crypto.Keccak256Hash(implCode)}
	expected := idov3whitelist.ExpectedProjectBinding{ChainID: binding.ChainID, ProxyAddress: binding.ProxyAddress, ProjectID: binding.ProjectID,
		PaymentToken: binding.PaymentToken, ProjectToken: binding.ProjectToken}
	word := func(v uint64) []byte { return common.LeftPadBytes(new(big.Int).SetUint64(v).Bytes(), 32) }
	tuple := make([]byte, 32+23*32+2*64)
	copy(tuple[:32], word(32))
	field := func(i int) []byte { return tuple[32+i*32 : 32+(i+1)*32] }
	copy(field(0), word(77))
	for i, a := range map[int]common.Address{1: binding.ProjectToken, 2: binding.PaymentToken, 3: binding.ProjectToken, 4: binding.PaymentToken} {
		copy(field(i), common.LeftPadBytes(a.Bytes(), 32))
	}
	for i, value := range map[int]uint64{5: 18, 6: 6, 7: 200, 8: 1, 9: 2, 10: 1000, 11: 500, 12: 600, 13: 100, 14: 200, 15: 100, 16: 10, 17: 1000, 21: 23 * 32, 22: 23*32 + 64} {
		copy(field(i), word(value))
	}
	copy(tuple[32+23*32:], word(1))
	copy(tuple[32+23*32+32:], word(300))
	copy(tuple[32+23*32+64:], word(1))
	copy(tuple[32+23*32+96:], word(10000))
	rpc := &projectPurchaseRPC{chainID: big.NewInt(31337), receipt: &types.Receipt{Status: types.ReceiptStatusSuccessful, BlockNumber: big.NewInt(90), BlockHash: block.Hash(), TxHash: txHash, Logs: []*types.Log{log}},
		block: block, head: head, proxy: binding.ProxyAddress, implementation: impl, proxyCode: proxyCode, implementationCode: implCode, projectResult: tuple}
	check := func(want idov3whitelist.ExpectedProjectBinding) (V3PurchaseProjectEvidence, error) {
		rpc.blockReads, rpc.headReads, rpc.receiptReads = 0, 0, 0
		return ReconcileV3PurchaseProjectEvidence(context.Background(), rpc, binding, want, runtime, txHash, 5, 11)
	}
	got, err := check(expected)
	if err != nil || got.Purchase.Purchase.PaymentRaw != "100" || got.Project.BlockHash != block.Hash() {
		t.Fatalf("valid composite evidence rejected: %+v %v", got, err)
	}
	copy(rpc.projectResult[32+15*32:32+16*32], word(99))
	if _, err := check(expected); err == nil {
		t.Fatal("purchase larger than project block raised total accepted")
	}
	copy(rpc.projectResult[32+15*32:32+16*32], word(100))
	copy(rpc.projectResult[32+16*32:32+17*32], word(9))
	if _, err := check(expected); err == nil {
		t.Fatal("purchase larger than project block sold total accepted")
	}
	copy(rpc.projectResult[32+16*32:32+17*32], word(10))
	bad := expected
	bad.ProjectToken = common.HexToAddress("0x6666666666666666666666666666666666666666")
	if _, err := check(bad); err == nil {
		t.Fatal("different project identity accepted")
	}
	rpc.failProject = true
	if _, err := check(expected); err == nil || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("raw RPC error leaked or accepted: %v", err)
	}
	rpc.failProject = false
	rpc.changeReceipt = true
	if _, err := check(expected); err == nil {
		t.Fatal("changed receipt during project read accepted")
	}
	rpc.changeReceipt = false
	rpc.failReceipt = true
	if _, err := check(expected); err == nil || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("raw receipt RPC error leaked or accepted: %v", err)
	}
	rpc.failReceipt = false
	rpc.replaceAfterProject = true
	if _, err := check(expected); err == nil {
		t.Fatal("mid-read reorg accepted")
	}
}
