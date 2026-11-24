package idov3whitelist

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestPolicyEvidenceFailureStageIsClosedFiniteEnumeration(t *testing.T) {
	for _, check := range []policyEvidenceStageError{
		"request", "scope", "receipt_lookup", "confirmations", "receipt_header",
		"receipt_policy_read", "receipt_policy_match", "head_header", "head_policy_read",
		"head_policy_match", "receipt_header_recheck", "head_header_recheck",
	} {
		err := fmt.Errorf("wrapped: %w", check)
		if got := PolicyEvidenceFailureStage(err); got != string(check) {
			t.Fatalf("check %s mapped to %s", check, got)
		}
		if err.Error() == string(check) || errors.Unwrap(err) == nil {
			t.Fatal("stage error contract changed")
		}
	}
	for _, err := range []error{errors.New("private rpc token"), policyEvidenceStageError("injected_secret"), nil} {
		if got := PolicyEvidenceFailureStage(err); got != "policy_evidence_unavailable" {
			t.Fatalf("unrecognized check leaked: %s", got)
		}
	}
}

func TestPolicyReceiptLookupSeparatesDeadlineRPCMissingAndMalformed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		ctx     context.Context
		receipt *types.Receipt
		err     error
		want    string
	}{
		{ctx, nil, errors.New("sensitive provider URL"), "receipt_lookup_deadline"},
		{context.Background(), nil, errors.New("sensitive provider URL"), "receipt_lookup_rpc"},
		{context.Background(), nil, nil, "receipt_lookup_missing"},
		{context.Background(), &types.Receipt{BlockNumber: big.NewInt(0)}, nil, "receipt_lookup_malformed"},
		{context.Background(), &types.Receipt{BlockNumber: big.NewInt(2), BlockHash: common.HexToHash("0x01")}, nil, ""},
	} {
		if got := policyReceiptLookupCheck(tc.ctx, tc.receipt, tc.err); got != tc.want {
			t.Fatalf("receipt check=%q want=%q", got, tc.want)
		}
	}
	for _, check := range []string{"receipt_lookup_deadline", "receipt_lookup_rpc", "receipt_lookup_missing", "receipt_lookup_malformed"} {
		if got := PolicyEvidenceFailureStage(policyEvidenceStageError(check)); got != check {
			t.Fatalf("closed receipt check=%q", got)
		}
	}
}

type evidenceStub struct {
	scope        Scope
	runtime      RuntimeIdentity
	receipt      *types.Receipt
	receiptHead  *types.Header
	headHeader   *types.Header
	policyAtHead []byte
	chain        *big.Int
	proxyCode    []byte
	implCode     []byte
	head         uint64
	reorg        bool
	receiptReads int
	headReorg    bool
	headReads    int
}

func (s *evidenceStub) ChainID(context.Context) (*big.Int, error)   { return s.chain, nil }
func (s *evidenceStub) BlockNumber(context.Context) (uint64, error) { return s.head, nil }
func (s *evidenceStub) TransactionReceipt(_ context.Context, _ common.Hash) (*types.Receipt, error) {
	return s.receipt, nil
}
func (s *evidenceStub) HeaderByNumber(_ context.Context, block *big.Int) (*types.Header, error) {
	if block.Uint64() == s.receipt.BlockNumber.Uint64() {
		s.receiptReads++
		if s.reorg && s.receiptReads >= 4 {
			return &types.Header{Number: block, Extra: []byte("replacement")}, nil
		}
		return s.receiptHead, nil
	}
	s.headReads++
	if s.headReorg && s.headReads >= 4 {
		return &types.Header{Number: block, Extra: []byte("replacement head")}, nil
	}
	return s.headHeader, nil
}
func (s *evidenceStub) CodeAt(_ context.Context, address common.Address, _ *big.Int) ([]byte, error) {
	if address == s.scope.ProxyAddress {
		return s.proxyCode, nil
	}
	if address == s.runtime.Implementation {
		return s.implCode, nil
	}
	return nil, nil
}
func (s *evidenceStub) StorageAt(_ context.Context, _ common.Address, _ common.Hash, _ *big.Int) ([]byte, error) {
	slot := make([]byte, 32)
	copy(slot[12:], s.runtime.Implementation[:])
	return slot, nil
}
func (s *evidenceStub) CallContract(_ context.Context, call ethereum.CallMsg, block *big.Int) ([]byte, error) {
	if len(call.Data) == 4 {
		return v3ContractVersion[:], nil
	}
	if block.Uint64() == s.head {
		return s.policyAtHead, nil
	}
	return s.policyAtHead, nil
}

func TestReconcilePolicyEvidenceChecksReceiptFinalityAndCurrentState(t *testing.T) {
	item := loadVectors(t).Cases[0]
	scope := Scope{ChainID: parseDecimal(t, item.Scope.ChainID), ProxyAddress: common.HexToAddress(item.Scope.ProxyAddress), ProjectID: parseDecimal(t, item.Scope.ProjectID)}
	tree, err := Build(scope, item.Wallets)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := tree.CommitSnapshot(scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	proxyCode, implCode := []byte{0x60, 0x80}, []byte{0x60, 0x01}
	runtime := RuntimeIdentity{
		Implementation: common.HexToAddress("0x2234567890123456789012345678901234567890"),
		ProxyCodeHash:  crypto.Keccak256Hash(proxyCode), CodeHash: crypto.Keccak256Hash(implCode),
	}
	receiptHead := &types.Header{Number: big.NewInt(90)}
	headHeader := &types.Header{Number: big.NewInt(100)}
	hash := common.HexToHash("0x1122")
	projectTopic, versionTopic := common.Hash{}, common.Hash{}
	scope.ProjectID.FillBytes(projectTopic[:])
	versionTopic[31] = 1
	data := make([]byte, 64)
	data[31] = 1
	copy(data[32:], candidate.Root[:])
	log := &types.Log{Address: scope.ProxyAddress, Topics: []common.Hash{policyUpdatedTopic, projectTopic, versionTopic}, Data: data, TxHash: hash, BlockNumber: 90, BlockHash: receiptHead.Hash()}
	receipt := &types.Receipt{Status: types.ReceiptStatusSuccessful, TxHash: hash, BlockHash: receiptHead.Hash(), BlockNumber: big.NewInt(90), Logs: []*types.Log{log}}
	policy := make([]byte, 96)
	policy[31], policy[95] = 1, 1
	copy(policy[32:64], candidate.Root[:])
	stub := &evidenceStub{scope: scope, runtime: runtime, receipt: receipt, receiptHead: receiptHead, headHeader: headHeader, policyAtHead: policy, chain: scope.ChainID, proxyCode: proxyCode, implCode: implCode, head: 100}
	check := func(confirmations uint64) (PolicyEvidence, error) {
		stub.receiptReads = 0
		stub.headReads = 0
		return ReconcilePolicyEvidence(context.Background(), stub, scope, runtime, candidate, hash, confirmations)
	}
	evidence, err := check(11)
	if err != nil || evidence.Head.BlockNumber != 100 || evidence.ReceiptBlock.BlockNumber != 90 {
		t.Fatalf("confirmed same-policy evidence rejected: %+v err=%v", evidence, err)
	}
	if _, err := check(12); err == nil {
		t.Fatal("insufficient confirmations accepted")
	}
	if _, err := check(0); err == nil {
		t.Fatal("zero-confirmation request accepted")
	}
	stub.head = 89
	if _, err := check(1); err == nil {
		t.Fatal("future receipt block accepted")
	}
	stub.head = 100
	stub.receipt.BlockHash = common.HexToHash("0x99")
	if _, err := check(1); err == nil {
		t.Fatal("orphaned receipt accepted")
	}
	stub.receipt.BlockHash = receiptHead.Hash()
	stub.policyAtHead = append([]byte(nil), policy...)
	stub.policyAtHead[95] = 2
	if _, err := check(1); err == nil {
		t.Fatal("changed policy accepted")
	}
	stub.policyAtHead = policy
	stub.reorg = true
	if _, err := check(1); err == nil {
		t.Fatal("receipt block reorg during reconciliation accepted")
	}
	stub.reorg = false
	stub.headReorg = true
	if _, err := check(1); err == nil {
		t.Fatal("confirmation head reorg during reconciliation accepted")
	}
}
