package chain

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"joincare_server/pkg/common/idov3whitelist"
)

func TestMatchV3PurchaseProjectObservationRequiresSameBlockAndIdentity(t *testing.T) {
	binding := V3PurchaseBinding{ChainID: 31337, ProxyAddress: common.HexToAddress("0x1111111111111111111111111111111111111111"), ProjectID: 7,
		PaymentToken: common.HexToAddress("0x2222222222222222222222222222222222222222"),
		ProjectToken: common.HexToAddress("0x3333333333333333333333333333333333333333")}
	blockHash, txHash := common.HexToHash("0x11"), common.HexToHash("0x22")
	buyer := common.HexToAddress("0x4444444444444444444444444444444444444444")
	log := &types.Log{Address: binding.ProxyAddress, BlockNumber: 90, BlockHash: blockHash, TxHash: txHash, Index: 5,
		Topics: []common.Hash{v3PurchasedTopic, common.BigToHash(big.NewInt(7)), common.BytesToHash(buyer.Bytes()), common.BytesToHash(binding.PaymentToken.Bytes())}, Data: make([]byte, 96)}
	log.Data[31], log.Data[95] = 100, 10
	copy(log.Data[44:64], binding.ProjectToken.Bytes())
	receipt := &types.Receipt{Status: types.ReceiptStatusSuccessful, BlockNumber: big.NewInt(90), BlockHash: blockHash, TxHash: txHash, Logs: []*types.Log{log}}
	event, err := DecodeV3PurchaseLog(*log, 31337, binding.ProxyAddress)
	if err != nil {
		t.Fatal(err)
	}
	project := idov3whitelist.ProjectIdentityObservation{ChainID: 31337, ProxyAddress: binding.ProxyAddress, ProjectID: 7,
		PaymentToken: binding.PaymentToken, ProjectToken: binding.ProjectToken, State: idov3whitelist.ProjectActive, BlockNumber: 90, BlockHash: blockHash,
		RaisedAtBlockRaw: "100", TokensSoldAtBlockRaw: "10"}
	match := func(observed idov3whitelist.ProjectIdentityObservation) error {
		return MatchV3PurchaseProjectObservation(event, binding, receipt, observed)
	}
	if err := match(project); err != nil {
		t.Fatal(err)
	}
	project.RaisedAtBlockRaw, project.TokensSoldAtBlockRaw = "1000", "100"
	if err := match(project); err != nil {
		t.Fatalf("later same-block purchases rejected: %v", err)
	}
	project.State = idov3whitelist.ProjectSuccessful // hard cap can end during purchase
	if err := match(project); err != nil {
		t.Fatalf("hard-cap closing purchase rejected: %v", err)
	}
	project.State = idov3whitelist.ProjectActive
	for name, change := range map[string]func(*idov3whitelist.ProjectIdentityObservation){
		"wrong block":               func(p *idov3whitelist.ProjectIdentityObservation) { p.BlockNumber++ },
		"orphan block":              func(p *idov3whitelist.ProjectIdentityObservation) { p.BlockHash = common.HexToHash("0x33") },
		"wrong chain":               func(p *idov3whitelist.ProjectIdentityObservation) { p.ChainID = 56 },
		"wrong proxy":               func(p *idov3whitelist.ProjectIdentityObservation) { p.ProxyAddress = buyer },
		"wrong project":             func(p *idov3whitelist.ProjectIdentityObservation) { p.ProjectID++ },
		"wrong payment":             func(p *idov3whitelist.ProjectIdentityObservation) { p.PaymentToken = buyer },
		"wrong sale":                func(p *idov3whitelist.ProjectIdentityObservation) { p.ProjectToken = buyer },
		"pending":                   func(p *idov3whitelist.ProjectIdentityObservation) { p.State = idov3whitelist.ProjectPending },
		"failed":                    func(p *idov3whitelist.ProjectIdentityObservation) { p.State = idov3whitelist.ProjectFailed },
		"cancelled":                 func(p *idov3whitelist.ProjectIdentityObservation) { p.State = idov3whitelist.ProjectCancelled },
		"payment above block total": func(p *idov3whitelist.ProjectIdentityObservation) { p.RaisedAtBlockRaw = "99" },
		"tokens above block total":  func(p *idov3whitelist.ProjectIdentityObservation) { p.TokensSoldAtBlockRaw = "9" },
		"malformed total":           func(p *idov3whitelist.ProjectIdentityObservation) { p.RaisedAtBlockRaw = "" },
	} {
		t.Run(name, func(t *testing.T) {
			got := project
			change(&got)
			if match(got) == nil {
				t.Fatal("mismatched purchase and project evidence accepted")
			}
		})
	}
	receipt.Status = types.ReceiptStatusFailed
	if err := match(project); err == nil {
		t.Fatal("failed purchase receipt accepted")
	}
}
