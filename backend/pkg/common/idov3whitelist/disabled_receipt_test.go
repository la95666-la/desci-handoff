package idov3whitelist

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestV3DisabledReceiptRequiresExactCanonicalPolicyEvent(t *testing.T) {
	scope := Scope{ChainID: big.NewInt(31337), ProjectID: big.NewInt(93), ProxyAddress: common.HexToAddress("0x9393939393939393939393939393939393939393")}
	hash, blockHash := common.HexToHash("0xabc"), common.HexToHash("0x123")
	log := &types.Log{Address: scope.ProxyAddress, TxHash: hash, BlockNumber: 90, BlockHash: blockHash,
		Topics: []common.Hash{policyUpdatedTopic, common.BigToHash(scope.ProjectID), common.BigToHash(big.NewInt(2))}, Data: make([]byte, 64)}
	receipt := &types.Receipt{TxHash: hash, Status: types.ReceiptStatusSuccessful, BlockNumber: big.NewInt(90), BlockHash: blockHash, Logs: []*types.Log{log}}
	observed := PolicyObservation{BlockNumber: 90, BlockHash: blockHash, Policy: Policy{Version: 2}}
	if err := MatchDisabledPolicyReceipt(scope, 2, hash, receipt, observed); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(){
		func() { log.Data[31] = 1 },
		func() { log.Data[31] = 0; log.Topics[2] = common.BigToHash(big.NewInt(3)) },
		func() { log.Topics[2] = common.BigToHash(big.NewInt(2)); receipt.Logs = append(receipt.Logs, log) },
	} {
		mutate()
		if err := MatchDisabledPolicyReceipt(scope, 2, hash, receipt, observed); err == nil {
			t.Fatal("altered disabled event accepted")
		}
	}
}
