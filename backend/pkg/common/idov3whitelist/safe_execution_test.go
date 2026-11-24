package idov3whitelist

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type safePolicyStub struct {
	*evidenceStub
	tx        *types.Transaction
	pending   bool
	threshold uint64
}

func (s *safePolicyStub) TransactionByHash(_ context.Context, hash common.Hash) (*types.Transaction, bool, error) {
	return s.tx, s.pending, nil
}

func (s *safePolicyStub) CallContract(ctx context.Context, call ethereum.CallMsg, block *big.Int) ([]byte, error) {
	if call.To != nil && *call.To != s.scope.ProxyAddress {
		return new(big.Int).SetUint64(s.threshold).FillBytes(make([]byte, 32)), nil
	}
	return s.evidenceStub.CallContract(ctx, call, block)
}

func TestV3SafeExecutionRequiresOriginalOuterTxAndTwoOwnerSuccess(t *testing.T) {
	scope := Scope{ChainID: big.NewInt(31337), ProjectID: big.NewInt(93), ProxyAddress: common.HexToAddress("0x9393939393939393939393939393939393939393")}
	governor := common.HexToAddress("0x2222222222222222222222222222222222222222")
	tree, err := Build(scope, []string{"0x1111111111111111111111111111111111111111"})
	if err != nil {
		t.Fatal(err)
	}
	commitment, err := tree.CommitSnapshot(scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	change := PolicyChange{ExpectedVersion: 0, TargetVersion: 1, Enabled: true, Root: commitment.Root, Candidate: &commitment}
	inner, err := PackPolicyChangeCall(scope, change)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := abi.JSON(strings.NewReader(safeExecutionABI))
	if err != nil {
		t.Fatal(err)
	}
	outer, err := parsed.Pack("execTransaction", scope.ProxyAddress, big.NewInt(0), inner, uint8(0),
		big.NewInt(0), big.NewInt(0), big.NewInt(0), common.Address{}, common.Address{}, make([]byte, 130))
	if err != nil {
		t.Fatal(err)
	}
	private, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	makeTx := func(to common.Address, data []byte) *types.Transaction {
		t.Helper()
		tx, e := types.SignTx(types.NewTx(&types.LegacyTx{Nonce: 1, To: &to, Gas: 400000, GasPrice: big.NewInt(1), Data: data}),
			types.LatestSignerForChainID(scope.ChainID), private)
		if e != nil {
			t.Fatal(e)
		}
		return tx
	}
	tx := makeTx(governor, outer)
	safeHash := common.HexToHash("0xabc")
	blockHash := common.HexToHash("0xdef")
	data := make([]byte, 64)
	data[31] = 1
	copy(data[32:], commitment.Root[:])
	successData := make([]byte, 64)
	copy(successData, safeHash[:])
	success := &types.Log{Address: governor, Topics: []common.Hash{safeExecutionSuccess}, Data: successData,
		TxHash: tx.Hash(), BlockHash: blockHash, BlockNumber: 90}
	policy := &types.Log{Address: scope.ProxyAddress, Topics: []common.Hash{policyUpdatedTopic, common.BigToHash(scope.ProjectID), common.BigToHash(big.NewInt(1))}, Data: data,
		TxHash: tx.Hash(), BlockHash: blockHash, BlockNumber: 90}
	receipt := &types.Receipt{TxHash: tx.Hash(), Status: types.ReceiptStatusSuccessful,
		BlockNumber: big.NewInt(90), BlockHash: blockHash, Logs: []*types.Log{success, policy}}
	stub := &safePolicyStub{evidenceStub: &evidenceStub{scope: scope}, tx: tx, threshold: 2}
	check := func() error {
		return CheckSafePolicyExecution(context.Background(), stub, scope, governor, change, tx.Hash(), safeHash, receipt)
	}
	if err := check(); err != nil {
		t.Fatalf("valid signed outer Safe execution rejected: %v", err)
	}
	// The deployed Policy Safe indexes the Safe transaction hash. A wrong
	// topic or nonzero payment must never authenticate the outer transaction.
	indexed := *success
	indexed.Topics = []common.Hash{safeExecutionSuccess, safeHash}
	indexed.Data = make([]byte, 32)
	indexedReceipt := *receipt
	indexedReceipt.Logs = []*types.Log{&indexed, policy}
	if err := CheckSafePolicyExecution(context.Background(), stub, scope, governor, change, tx.Hash(), safeHash, &indexedReceipt); err != nil {
		t.Fatalf("indexed Safe success rejected: %v", err)
	}
	for _, malformed := range []struct {
		name   string
		topics []common.Hash
		data   []byte
	}{
		{"wrong hash", []common.Hash{safeExecutionSuccess, common.HexToHash("0xdef")}, make([]byte, 32)},
		{"nonzero payment", indexed.Topics, append(make([]byte, 31), 1)},
		{"extra topic", append(append([]common.Hash{}, indexed.Topics...), common.Hash{}), make([]byte, 32)},
	} {
		t.Run(malformed.name, func(t *testing.T) {
			badLog := indexed
			badLog.Topics, badLog.Data = malformed.topics, malformed.data
			badReceipt := indexedReceipt
			badReceipt.Logs = []*types.Log{&badLog, policy}
			if CheckSafePolicyExecution(context.Background(), stub, scope, governor, change, tx.Hash(), safeHash, &badReceipt) == nil {
				t.Fatal("malformed Safe event accepted")
			}
		})
	}
	stub.threshold = 1
	if check() == nil {
		t.Fatal("one-owner Safe accepted")
	}
	stub.threshold = 2
	stub.pending = true
	if check() == nil {
		t.Fatal("pending Safe transaction accepted")
	}
	stub.pending = false
	stub.tx = makeTx(scope.ProxyAddress, outer)
	if check() == nil {
		t.Fatal("wrong outer Safe target accepted")
	}
	stub.tx = tx
	success.Data = make([]byte, 64)
	if check() == nil {
		t.Fatal("wrong Safe execution hash accepted")
	}
	copy(success.Data, safeHash[:])
	receipt.Logs = append(receipt.Logs, policy)
	if check() == nil {
		t.Fatal("ambiguous policy event accepted")
	}
}
