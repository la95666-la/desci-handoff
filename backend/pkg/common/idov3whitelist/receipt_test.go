package idov3whitelist

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestMatchPolicyReceiptFailsClosedOnExecutionAndPlacement(t *testing.T) {
	item := loadVectors(t).Cases[2]
	scope := Scope{ChainID: parseDecimal(t, item.Scope.ChainID), ProxyAddress: common.HexToAddress(item.Scope.ProxyAddress), ProjectID: parseDecimal(t, item.Scope.ProjectID)}
	tree, err := Build(scope, item.Wallets)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := tree.CommitSnapshot(scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	txHash := common.HexToHash("0x1234")
	blockHash := common.HexToHash("0x5678")
	projectTopic, versionTopic := common.Hash{}, common.Hash{}
	scope.ProjectID.FillBytes(projectTopic[:])
	versionTopic[31] = 1
	data := make([]byte, 64)
	data[31] = 1
	copy(data[32:], candidate.Root[:])
	log := &types.Log{Address: scope.ProxyAddress, Topics: []common.Hash{policyUpdatedTopic, projectTopic, versionTopic}, Data: data, TxHash: txHash, BlockHash: blockHash, BlockNumber: 99}
	receipt := &types.Receipt{Status: types.ReceiptStatusSuccessful, TxHash: txHash, BlockHash: blockHash, BlockNumber: big.NewInt(99), Logs: []*types.Log{log}}
	observation := PolicyObservation{Policy: Policy{Enabled: true, Root: candidate.Root, Version: 1}, BlockNumber: 99, BlockHash: blockHash}
	check := func(r *types.Receipt, c SnapshotCommitment, o PolicyObservation) error {
		return MatchPolicyReceipt(scope, c, txHash, r, o)
	}
	if err := check(receipt, candidate, observation); err != nil {
		t.Fatalf("valid same-block policy receipt rejected: %v", err)
	}
	clone := func() *types.Receipt {
		out := *receipt
		entry := *log
		entry.Topics = append([]common.Hash(nil), log.Topics...)
		entry.Data = append([]byte(nil), log.Data...)
		out.Logs = []*types.Log{&entry}
		return &out
	}
	for name, mutate := range map[string]func(*types.Receipt){
		"revert":           func(r *types.Receipt) { r.Status = types.ReceiptStatusFailed },
		"tx hash":          func(r *types.Receipt) { r.TxHash = common.HexToHash("0x01") },
		"receipt block":    func(r *types.Receipt) { r.BlockHash = common.HexToHash("0x01") },
		"missing log":      func(r *types.Receipt) { r.Logs = nil },
		"duplicate update": func(r *types.Receipt) { r.Logs = append(r.Logs, r.Logs[0]) },
		"wrong proxy": func(r *types.Receipt) {
			r.Logs[0].Address = common.HexToAddress("0x2234567890123456789012345678901234567890")
		},
		"wrong project":    func(r *types.Receipt) { r.Logs[0].Topics[1] = common.HexToHash("0x01") },
		"wrong version":    func(r *types.Receipt) { r.Logs[0].Topics[2] = common.HexToHash("0x02") },
		"wrong root":       func(r *types.Receipt) { r.Logs[0].Data[63] ^= 1 },
		"disabled":         func(r *types.Receipt) { r.Logs[0].Data[31] = 0 },
		"dirty bool":       func(r *types.Receipt) { r.Logs[0].Data[0] = 1 },
		"removed":          func(r *types.Receipt) { r.Logs[0].Removed = true },
		"forged log block": func(r *types.Receipt) { r.Logs[0].BlockHash = common.HexToHash("0x99") },
	} {
		t.Run(name, func(t *testing.T) {
			r := clone()
			mutate(r)
			if err := check(r, candidate, observation); err == nil {
				t.Fatal("mismatched receipt accepted")
			}
		})
	}
	otherProject := clone()
	otherLog := *otherProject.Logs[0]
	otherLog.Topics = append([]common.Hash(nil), otherLog.Topics...)
	otherLog.Topics[1] = common.HexToHash("0x01")
	otherProject.Logs = append(otherProject.Logs, &otherLog)
	if err := check(otherProject, candidate, observation); err != nil {
		t.Fatalf("independent project batch incorrectly rejected: %v", err)
	}
	wrong := candidate
	wrong.Digest = common.HexToHash("0x11")
	if err := check(receipt, wrong, observation); err == nil {
		t.Fatal("wrong full-member digest accepted")
	}
	wrongObservation := observation
	wrongObservation.Policy.Version++
	if err := check(receipt, candidate, wrongObservation); err == nil {
		t.Fatal("wrong observed version accepted")
	}
	if err := check(nil, candidate, observation); err == nil {
		t.Fatal("missing receipt accepted")
	}
}
