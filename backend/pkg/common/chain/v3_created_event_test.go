package chain

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestV3ProjectCreatedStrictlyDecodesWithoutV2Routing(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	sale := common.HexToAddress("0x2222222222222222222222222222222222222222")
	payment := common.HexToAddress("0x3333333333333333333333333333333333333333")
	owner := common.HexToAddress("0x4444444444444444444444444444444444444444")
	receiver := common.HexToAddress("0x5555555555555555555555555555555555555555")
	word := func(v uint64) []byte { return common.LeftPadBytes(new(big.Int).SetUint64(v).Bytes(), 32) }
	log := types.Log{Address: proxy, BlockNumber: 90, BlockHash: common.HexToHash("0x90"), TxHash: common.HexToHash("0xaa"), Index: 4,
		Topics: []common.Hash{v3CreatedTopic, common.BigToHash(big.NewInt(41376991395446785)), common.BytesToHash(sale.Bytes()), common.BytesToHash(payment.Bytes())}}
	for _, a := range []common.Address{owner, receiver} {
		log.Data = append(log.Data, common.LeftPadBytes(a.Bytes(), 32)...)
	}
	for _, v := range []uint64{18, 6, 200, 2, 1000, 500, 600, 100, 200} {
		log.Data = append(log.Data, word(v)...)
	}
	check := func(candidate types.Log) error { _, err := DecodeV3CreatedLog(candidate, 31337, proxy); return err }
	got, err := DecodeV3CreatedLog(log, 31337, proxy)
	if err != nil || got.ProjectID != 41376991395446785 || got.ProjectToken != sale || got.PaymentToken != payment || got.ProjectOwner != owner || got.FeeReceiver != receiver || got.FeeRate != 200 || got.MaxPerUserRaw != "600" {
		t.Fatalf("V3 creation rejected: %+v %v", got, err)
	}
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: got.ProjectID, ProjectToken: sale, PaymentToken: payment}
	if MatchV3CreatedBinding(got, binding) != nil {
		t.Fatal("valid binding rejected")
	}
	binding.PaymentToken = sale
	if MatchV3CreatedBinding(got, binding) == nil {
		t.Fatal("wrong token binding accepted")
	}
	if _, err := DecodeV3CreatedLog(log, 31337, payment); err == nil {
		t.Fatal("wrong proxy accepted")
	}
	if _, err := DecodeV2Log(log); err != nil {
		t.Fatal("shared V2 topic unexpectedly different")
	}
	for name, change := range map[string]func(*types.Log){
		"wrong topic":    func(l *types.Log) { l.Topics[0] = common.Hash{} },
		"dirty project":  func(l *types.Log) { l.Topics[1][0] = 1 },
		"dirty token":    func(l *types.Log) { l.Topics[2][0] = 1 },
		"zero owner":     func(l *types.Log) { clear(l.Data[:32]) },
		"dirty receiver": func(l *types.Log) { l.Data[32] = 1 },
		"bad decimals":   func(l *types.Log) { l.Data[2*32+31] = 19 },
		"fee too high":   func(l *types.Log) { copy(l.Data[4*32:5*32], word(1001)) },
		"soft over cap":  func(l *types.Log) { copy(l.Data[7*32:8*32], word(1001)) },
		"zero price":     func(l *types.Log) { clear(l.Data[5*32 : 6*32]) },
		"short ABI":      func(l *types.Log) { l.Data = l.Data[:len(l.Data)-32] },
		"removed":        func(l *types.Log) { l.Removed = true },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := log
			candidate.Topics = append([]common.Hash(nil), log.Topics...)
			candidate.Data = append([]byte(nil), log.Data...)
			change(&candidate)
			if check(candidate) == nil {
				t.Fatal("malformed creation accepted")
			}
		})
	}
}

func TestV3CreatedReceiptRequiresUniqueSuccessfulExactLog(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	sale := common.HexToAddress("0x2222222222222222222222222222222222222222")
	pay := common.HexToAddress("0x3333333333333333333333333333333333333333")
	owner := common.HexToAddress("0x4444444444444444444444444444444444444444")
	receiver := common.HexToAddress("0x5555555555555555555555555555555555555555")
	word := func(v uint64) []byte { return common.LeftPadBytes(new(big.Int).SetUint64(v).Bytes(), 32) }
	log := &types.Log{Address: proxy, BlockNumber: 90, BlockHash: common.HexToHash("0x90"), TxHash: common.HexToHash("0xaa"), Index: 4,
		Topics: []common.Hash{v3CreatedTopic, common.BigToHash(big.NewInt(7)), common.BytesToHash(sale.Bytes()), common.BytesToHash(pay.Bytes())}}
	for _, a := range []common.Address{owner, receiver} {
		log.Data = append(log.Data, common.LeftPadBytes(a.Bytes(), 32)...)
	}
	for _, v := range []uint64{18, 6, 200, 2, 1000, 500, 600, 100, 200} {
		log.Data = append(log.Data, word(v)...)
	}
	event, err := DecodeV3CreatedLog(*log, 31337, proxy)
	if err != nil {
		t.Fatal(err)
	}
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7, ProjectToken: sale, PaymentToken: pay}
	receipt := &types.Receipt{Status: types.ReceiptStatusSuccessful, BlockNumber: big.NewInt(90), BlockHash: log.BlockHash, TxHash: log.TxHash, Logs: []*types.Log{log}}
	check := func() error { return MatchV3CreatedReceipt(event, binding, receipt) }
	if err := check(); err != nil {
		t.Fatalf("valid V3 creation receipt rejected: %v", err)
	}
	other := *log
	other.Index = 5
	receipt.Logs = append(receipt.Logs, &other)
	if err := check(); err != nil {
		t.Fatalf("other log rejected: %v", err)
	}
	other.Index = 4
	if err := check(); err == nil {
		t.Fatal("duplicate index accepted")
	}
	receipt.Logs = receipt.Logs[:1]
	changed := *log
	changed.Data = append([]byte(nil), log.Data...)
	changed.Data[5*32+31]++
	receipt.Logs = []*types.Log{&changed}
	if err := check(); err == nil {
		t.Fatal("changed price accepted")
	}
	receipt.Logs = []*types.Log{log}
	for name, change := range map[string]func(*types.Receipt){
		"failed":       func(r *types.Receipt) { r.Status = types.ReceiptStatusFailed },
		"wrong tx":     func(r *types.Receipt) { r.TxHash = common.HexToHash("0xbb") },
		"wrong block":  func(r *types.Receipt) { r.BlockHash = common.HexToHash("0xbb") },
		"wrong height": func(r *types.Receipt) { r.BlockNumber = big.NewInt(91) },
		"missing":      func(r *types.Receipt) { r.Logs = nil },
	} {
		t.Run(name, func(t *testing.T) {
			r := *receipt
			change(&r)
			if MatchV3CreatedReceipt(event, binding, &r) == nil {
				t.Fatal("invalid receipt accepted")
			}
		})
	}
	bad := binding
	bad.ProxyAddress = pay
	if MatchV3CreatedReceipt(event, bad, receipt) == nil {
		t.Fatal("wrong proxy binding accepted")
	}
}
