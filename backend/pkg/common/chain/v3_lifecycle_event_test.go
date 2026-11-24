package chain

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestDecodeV3LifecycleLogExactShape(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	token := common.HexToAddress("0x2222222222222222222222222222222222222222")
	user := common.HexToAddress("0x3333333333333333333333333333333333333333")
	word := func(v uint64) []byte { return common.LeftPadBytes(new(big.Int).SetUint64(v).Bytes(), 32) }
	for _, tc := range []struct {
		name, sig         string
		addresses         []common.Address
		values            []uint64
		amount, secondary string
		status            uint8
	}{
		{"ProjectTokensDeposited", "ProjectTokensDeposited(uint256,address,uint256)", []common.Address{token}, []uint64{10}, "10", "", 0},
		{"ProjectActivated", "ProjectActivated(uint256)", nil, nil, "", "", 0},
		{"ProjectEnded", "ProjectEnded(uint256,uint8,address,uint256)", []common.Address{token}, []uint64{2, 10}, "10", "", 2},
		{"ProjectCancelled", "ProjectCancelled(uint256,address,uint256)", []common.Address{token}, []uint64{0}, "0", "", 0},
		{"Refunded", "Refunded(uint256,address,address,uint256)", []common.Address{user, token}, []uint64{10}, "10", "", 0},
		{"Claimed", "Claimed(uint256,address,address,uint256)", []common.Address{user, token}, []uint64{10}, "10", "", 0},
		{"ProjectProceedsWithdrawn", "ProjectProceedsWithdrawn(uint256,address,address,uint256,uint256)", []common.Address{token, user}, []uint64{9, 1}, "9", "1", 0},
		{"PlatformFeeWithdrawn", "PlatformFeeWithdrawn(uint256,address,address,uint256)", []common.Address{token, user}, []uint64{0}, "0", "", 0},
		{"ProjectTokensWithdrawn", "ProjectTokensWithdrawn(uint256,address,address,uint256)", []common.Address{token, user}, []uint64{0}, "0", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := types.Log{Address: proxy, BlockNumber: 12, BlockHash: common.HexToHash("0x12"), TxHash: common.HexToHash("0x34"), Index: 5,
				Topics: []common.Hash{crypto.Keccak256Hash([]byte(tc.sig)), common.BigToHash(big.NewInt(7))}}
			for _, a := range tc.addresses {
				log.Topics = append(log.Topics, common.BytesToHash(a.Bytes()))
			}
			for _, v := range tc.values {
				log.Data = append(log.Data, word(v)...)
			}
			got, err := DecodeV3LifecycleLog(log, 31337, proxy)
			if err != nil || got.Kind != tc.name || got.ProjectID != 7 || got.AmountRaw != tc.amount || got.SecondaryRaw != tc.secondary || got.Status != tc.status {
				t.Fatalf("valid V3 event rejected: %+v %v", got, err)
			}
			binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7,
				ProjectToken: token, PaymentToken: common.HexToAddress("0x4444444444444444444444444444444444444444")}
			if tc.name != "ProjectTokensDeposited" && tc.name != "Claimed" && tc.name != "ProjectTokensWithdrawn" {
				binding.PaymentToken = token
				binding.ProjectToken = common.HexToAddress("0x4444444444444444444444444444444444444444")
			}
			if err := MatchV3LifecycleBinding(got, binding); err != nil {
				t.Fatalf("bound lifecycle log rejected: %v", err)
			}
			wrongToken := binding
			if tc.name == "ProjectTokensDeposited" || tc.name == "Claimed" || tc.name == "ProjectTokensWithdrawn" {
				wrongToken.ProjectToken = user
			} else if tc.name != "ProjectActivated" {
				wrongToken.PaymentToken = user
			}
			if tc.name != "ProjectActivated" && MatchV3LifecycleBinding(got, wrongToken) == nil {
				t.Fatal("wrong bound lifecycle token accepted")
			}
			binding.ProxyAddress = user
			if err := MatchV3LifecycleBinding(got, binding); err == nil {
				t.Fatal("wrong bound proxy accepted")
			}
			if _, err := DecodeV3LifecycleLog(log, 31337, token); err == nil {
				t.Fatal("wrong proxy accepted")
			}
			bad := log
			bad.Topics = append([]common.Hash(nil), log.Topics...)
			bad.Data = append([]byte(nil), log.Data...)
			bad.Topics[1] = common.Hash{}
			if _, err := DecodeV3LifecycleLog(bad, 31337, proxy); err == nil {
				t.Fatal("zero project accepted")
			}
			bad = log
			bad.Data = append(append([]byte(nil), log.Data...), 0)
			if _, err := DecodeV3LifecycleLog(bad, 31337, proxy); err == nil {
				t.Fatal("dirty data length accepted")
			}
			if len(tc.addresses) > 0 {
				bad = log
				bad.Topics = append([]common.Hash(nil), log.Topics...)
				bad.Topics[2][0] = 1
				if _, err := DecodeV3LifecycleLog(bad, 31337, proxy); err == nil {
					t.Fatal("dirty address padding accepted")
				}
			}
			bad = log
			bad.Removed = true
			if _, err := DecodeV3LifecycleLog(bad, 31337, proxy); err == nil {
				t.Fatal("removed log accepted")
			}
		})
	}
}

func TestMatchV3LifecycleReceiptRequiresUniqueSuccessfulLog(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	token := common.HexToAddress("0x2222222222222222222222222222222222222222")
	user := common.HexToAddress("0x3333333333333333333333333333333333333333")
	blockHash, txHash := common.HexToHash("0xab"), common.HexToHash("0xcd")
	log := &types.Log{Address: proxy, BlockNumber: 90, BlockHash: blockHash, TxHash: txHash, Index: 4,
		Topics: []common.Hash{crypto.Keccak256Hash([]byte("Refunded(uint256,address,address,uint256)")), common.BigToHash(big.NewInt(7)), common.BytesToHash(user.Bytes()), common.BytesToHash(token.Bytes())},
		Data:   common.LeftPadBytes(big.NewInt(100).Bytes(), 32)}
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7,
		ProjectToken: common.HexToAddress("0x4444444444444444444444444444444444444444"), PaymentToken: token}
	event, err := DecodeV3LifecycleLog(*log, binding.ChainID, proxy)
	if err != nil {
		t.Fatal(err)
	}
	receipt := &types.Receipt{Status: types.ReceiptStatusSuccessful, BlockNumber: big.NewInt(90), BlockHash: blockHash, TxHash: txHash, Logs: []*types.Log{log}}
	check := func() error { return MatchV3LifecycleReceipt(event, binding, receipt) }
	if err := check(); err != nil {
		t.Fatalf("valid lifecycle receipt rejected: %v", err)
	}
	// A second distinct log in a multi-event transaction is allowed; a
	// second log at the same index is not, regardless of its topic or address.
	other := *log
	other.Index = 5
	receipt.Logs = append(receipt.Logs, &other)
	if err := check(); err != nil {
		t.Fatalf("unrelated same-transaction log rejected: %v", err)
	}
	other.Index = 4
	if err := check(); err == nil {
		t.Fatal("duplicate log index accepted")
	}
	receipt.Logs = receipt.Logs[:1]
	changed := *log
	changed.Data = common.LeftPadBytes(big.NewInt(99).Bytes(), 32)
	receipt.Logs = []*types.Log{&changed}
	if err := check(); err == nil {
		t.Fatal("changed refund amount accepted")
	}
	receipt.Logs = []*types.Log{log}
	changed = *log
	changed.Address = binding.ProjectToken
	receipt.Logs = []*types.Log{&changed}
	if err := check(); err == nil {
		t.Fatal("different proxy accepted")
	}
	receipt.Logs = []*types.Log{log}
	for name, change := range map[string]func(*types.Receipt){
		"failed receipt":    func(r *types.Receipt) { r.Status = types.ReceiptStatusFailed },
		"wrong tx":          func(r *types.Receipt) { r.TxHash = common.HexToHash("0xee") },
		"wrong block":       func(r *types.Receipt) { r.BlockHash = common.HexToHash("0xee") },
		"missing placement": func(r *types.Receipt) { r.BlockNumber = nil },
		"missing log":       func(r *types.Receipt) { r.Logs = nil },
	} {
		t.Run(name, func(t *testing.T) {
			copyOfReceipt := *receipt
			change(&copyOfReceipt)
			if MatchV3LifecycleReceipt(event, binding, &copyOfReceipt) == nil {
				t.Fatal("invalid receipt accepted")
			}
		})
	}
	wrong := binding
	wrong.PaymentToken = binding.ProjectToken
	if MatchV3LifecycleReceipt(event, wrong, receipt) == nil {
		t.Fatal("wrong payment token accepted")
	}
	if MatchV3LifecycleReceipt(nil, binding, receipt) == nil {
		t.Fatal("nil lifecycle event accepted")
	}
}
