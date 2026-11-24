package chain

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestDecodeV3PurchaseLogRequiresExactProxyAndABIBeforeFinancialUse(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	buyer := common.HexToAddress("0x2222222222222222222222222222222222222222")
	payment := common.HexToAddress("0x3333333333333333333333333333333333333333")
	token := common.HexToAddress("0x4444444444444444444444444444444444444444")
	log := types.Log{
		Address: proxy, BlockNumber: 42, BlockHash: common.HexToHash("0x42"), TxHash: common.HexToHash("0x43"), Index: 5,
		Topics: []common.Hash{v3PurchasedTopic, common.BigToHash(big.NewInt(41376991395446785)), common.BytesToHash(buyer.Bytes()), common.BytesToHash(payment.Bytes())},
		Data:   make([]byte, 96),
	}
	copy(log.Data[:32], common.LeftPadBytes(new(big.Int).Lsh(big.NewInt(1), 200).Bytes(), 32))
	copy(log.Data[44:64], token.Bytes())
	copy(log.Data[64:96], common.LeftPadBytes(big.NewInt(9000000000000000000).Bytes(), 32))
	got, err := DecodeV3PurchaseLog(log, 31337, proxy)
	if err != nil || got.ProjectID != 41376991395446785 || got.PaymentRaw != new(big.Int).Lsh(big.NewInt(1), 200).String() || got.ProjectTokenRaw != "9000000000000000000" || got.Buyer != buyer || got.PaymentToken != payment || got.ProjectToken != token || got.LogIndex != 5 {
		t.Fatalf("valid V3 purchase lost raw financial facts: %+v %v", got, err)
	}
	// V2 has the same event signature. DecodeV2Log alone cannot route it:
	// only independently admitted chain/proxy/project identity may do so.
	if v2, err := DecodeV2Log(log); err != nil || v2 == nil || v2.Event != V2Purchased {
		t.Fatalf("expected shared V2 topic to require source routing: %+v %v", v2, err)
	}
	if _, err := DecodeV3PurchaseLog(log, 0, proxy); err == nil {
		t.Fatal("missing chain accepted")
	}
	if _, err := DecodeV3PurchaseLog(log, 31337, common.HexToAddress("0x5555555555555555555555555555555555555555")); err == nil {
		t.Fatal("V2 or unrelated proxy admitted as V3 purchase")
	}
	for name, mutate := range map[string]func(*types.Log){
		"removed":             func(l *types.Log) { l.Removed = true },
		"missing block":       func(l *types.Log) { l.BlockHash = common.Hash{} },
		"missing tx":          func(l *types.Log) { l.TxHash = common.Hash{} },
		"wrong topic":         func(l *types.Log) { l.Topics[0] = common.Hash{} },
		"extra topic":         func(l *types.Log) { l.Topics = append(l.Topics, common.Hash{}) },
		"short data":          func(l *types.Log) { l.Data = l.Data[:64] },
		"oversized project":   func(l *types.Log) { l.Topics[1][0] = 1 },
		"zero project":        func(l *types.Log) { l.Topics[1] = common.Hash{} },
		"dirty buyer padding": func(l *types.Log) { l.Topics[2][0] = 1 },
		"dirty pay padding":   func(l *types.Log) { l.Topics[3][0] = 1 },
		"dirty token padding": func(l *types.Log) { l.Data[32] = 1 },
		"zero buyer":          func(l *types.Log) { l.Topics[2] = common.Hash{} },
		"zero payment token":  func(l *types.Log) { l.Topics[3] = common.Hash{} },
		"zero project token":  func(l *types.Log) { clear(l.Data[32:64]) },
		"zero payment amount": func(l *types.Log) { clear(l.Data[:32]) },
		"zero token amount":   func(l *types.Log) { clear(l.Data[64:96]) },
	} {
		t.Run(name, func(t *testing.T) {
			bad := log
			bad.Topics = append([]common.Hash(nil), log.Topics...)
			bad.Data = append([]byte(nil), log.Data...)
			mutate(&bad)
			if _, err := DecodeV3PurchaseLog(bad, 31337, proxy); err == nil {
				t.Fatal("malformed purchase admitted")
			}
		})
	}
}

func TestMatchV3PurchaseReceiptFailsClosedOnBindingAndPlacement(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	buyer := common.HexToAddress("0x2222222222222222222222222222222222222222")
	payment := common.HexToAddress("0x3333333333333333333333333333333333333333")
	token := common.HexToAddress("0x4444444444444444444444444444444444444444")
	log := &types.Log{
		Address: proxy, BlockNumber: 42, BlockHash: common.HexToHash("0x42"), TxHash: common.HexToHash("0x43"), Index: 5,
		Topics: []common.Hash{v3PurchasedTopic, common.BigToHash(big.NewInt(99)), common.BytesToHash(buyer.Bytes()), common.BytesToHash(payment.Bytes())}, Data: make([]byte, 96),
	}
	log.Data[31] = 100
	copy(log.Data[44:64], token.Bytes())
	log.Data[95] = 10
	event, err := DecodeV3PurchaseLog(*log, 31337, proxy)
	if err != nil {
		t.Fatal(err)
	}
	binding := V3PurchaseBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 99, PaymentToken: payment, ProjectToken: token}
	receipt := &types.Receipt{Status: types.ReceiptStatusSuccessful, BlockNumber: big.NewInt(42), BlockHash: log.BlockHash, TxHash: log.TxHash, Logs: []*types.Log{log}}
	check := func() error { return MatchV3PurchaseReceipt(event, binding, receipt) }
	if err := check(); err != nil {
		t.Fatal(err)
	}
	other := *log
	other.Index = 4
	receipt.Logs = []*types.Log{&other, log}
	if err := check(); err != nil {
		t.Fatalf("independent purchase at different index blocked: %v", err)
	}
	receipt.Logs = []*types.Log{log}
	for name, mutate := range map[string]func(*V3PurchaseBinding){
		"wrong chain":         func(b *V3PurchaseBinding) { b.ChainID++ },
		"wrong proxy":         func(b *V3PurchaseBinding) { b.ProxyAddress = buyer },
		"wrong project":       func(b *V3PurchaseBinding) { b.ProjectID++ },
		"wrong payment token": func(b *V3PurchaseBinding) { b.PaymentToken = token },
		"wrong sale token":    func(b *V3PurchaseBinding) { b.ProjectToken = payment },
	} {
		t.Run(name, func(t *testing.T) {
			bad := binding
			mutate(&bad)
			if err := MatchV3PurchaseReceipt(event, bad, receipt); err == nil {
				t.Fatal("wrong binding accepted")
			}
		})
	}
	receipt.Status = types.ReceiptStatusFailed
	if err := check(); err == nil {
		t.Fatal("revert accepted")
	}
	receipt.Status = types.ReceiptStatusSuccessful
	receipt.BlockHash = common.HexToHash("0x99")
	if err := check(); err == nil {
		t.Fatal("orphaned receipt accepted")
	}
	receipt.BlockHash = log.BlockHash
	receipt.Logs = nil
	if err := check(); err == nil {
		t.Fatal("missing log accepted")
	}
	receipt.Logs = []*types.Log{log, log}
	if err := check(); err == nil {
		t.Fatal("duplicate log accepted")
	}
	receipt.Logs = []*types.Log{log}
	altered := *log
	altered.Data = append([]byte(nil), log.Data...)
	altered.Data[31] = 99
	receipt.Logs = []*types.Log{&altered}
	if err := check(); err == nil {
		t.Fatal("altered amount accepted")
	}
	receipt.Logs = []*types.Log{log}
	alteredEvent := *event
	alteredEvent.LogIndex++
	if err := MatchV3PurchaseReceipt(&alteredEvent, binding, receipt); err == nil {
		t.Fatal("different index accepted")
	}
}
