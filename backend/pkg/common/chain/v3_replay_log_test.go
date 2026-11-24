package chain

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestDecodeV3ReplayLogRequiresExplicitProxyAndProjectBinding(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	pay := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sale := common.HexToAddress("0x3333333333333333333333333333333333333333")
	buyer := common.HexToAddress("0x4444444444444444444444444444444444444444")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 7, ProxyAddress: proxy, PaymentToken: pay, ProjectToken: sale}
	addressTopic := func(address common.Address) common.Hash { return common.BytesToHash(address.Bytes()) }
	word := func(value uint64) []byte { return new(big.Int).SetUint64(value).FillBytes(make([]byte, 32)) }
	log := types.Log{Address: proxy, BlockNumber: 22, BlockHash: common.HexToHash("0xa1"), TxHash: common.HexToHash("0xb1"), Index: 2,
		Topics: []common.Hash{v3PurchasedTopic, common.BigToHash(big.NewInt(7)), addressTopic(buyer), addressTopic(pay)},
		Data:   append(append(word(100), addressTopic(sale).Bytes()...), word(50)...)}
	event, err := DecodeV3ReplayLog(log, binding)
	if err != nil || event.Purchase == nil || event.Purchase.ProjectTokenRaw != "50" {
		t.Fatalf("valid purchase rejected: %+v %v", event, err)
	}
	bad := log
	bad.Address = common.HexToAddress("0x5555555555555555555555555555555555555555")
	if _, err := DecodeV3ReplayLog(bad, binding); err == nil {
		t.Fatal("same-topic V2 address admitted")
	}
	bad = log
	bad.Topics = append([]common.Hash(nil), log.Topics...)
	bad.Topics[1] = common.BigToHash(big.NewInt(8))
	if _, err := DecodeV3ReplayLog(bad, binding); err == nil {
		t.Fatal("wrong project admitted")
	}
	bad = log
	bad.Removed = true
	if _, err := DecodeV3ReplayLog(bad, binding); err == nil {
		t.Fatal("removed log admitted")
	}
	bad = log
	bad.Topics = append([]common.Hash(nil), log.Topics...)
	bad.Topics[0] = crypto.Keccak256Hash([]byte("Unknown(uint256)"))
	if _, err := DecodeV3ReplayLog(bad, binding); err == nil {
		t.Fatal("unknown topic admitted")
	}
	bad = log
	bad.Topics = []common.Hash{crypto.Keccak256Hash([]byte("ProjectActivated(uint256)")), common.BigToHash(big.NewInt(7))}
	bad.Data = nil
	if event, err := DecodeV3ReplayLog(bad, binding); err != nil || event.Lifecycle == nil || event.Lifecycle.Kind != "ProjectActivated" {
		t.Fatalf("lifecycle rejected: %+v %v", event, err)
	}
}
