package chain

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestV3PolicyEventDecoderIsSourceAndABIIsolated(t *testing.T) {
	proxy := common.HexToAddress("0x1234567890123456789012345678901234567890")
	project := common.BigToHash(big.NewInt(41376991395446785))
	root := common.HexToHash("0x1234")
	log := types.Log{Address: proxy, BlockNumber: 12, BlockHash: common.HexToHash("0xab"), TxHash: common.HexToHash("0xcd"), Topics: []common.Hash{v3PolicyUpdatedTopic, project, common.BigToHash(big.NewInt(7))}, Data: make([]byte, 64)}
	log.Data[31] = 1
	copy(log.Data[32:], root[:])
	got, err := DecodeV3PolicyLog(log, 31337, proxy)
	if err != nil || got.ProjectID != 41376991395446785 || got.Version != 7 || !got.Enabled || got.Root != root || got.ProxyAddress != proxy {
		t.Fatalf("V3 policy event rejected: %+v %v", got, err)
	}
	if previous, err := DecodeV2Log(log); err != nil || previous != nil {
		t.Fatal("V3 topic leaked into V2 decoder")
	}
	for name, mutate := range map[string]func(*types.Log){
		"different proxy": func(l *types.Log) { l.Address = common.HexToAddress("0x2234567890123456789012345678901234567890") },
		"wrong topic":     func(l *types.Log) { l.Topics[0] = common.HexToHash("0x01") },
		"bad version":     func(l *types.Log) { l.Topics[2] = common.Hash{} },
		"bad project":     func(l *types.Log) { l.Topics[1] = common.Hash{} },
		"dirty bool":      func(l *types.Log) { l.Data[0] = 1 },
		"wrong root": func(l *types.Log) {
			for i := 32; i < 64; i++ {
				l.Data[i] = 0
			}
		},
		"removed":    func(l *types.Log) { l.Removed = true },
		"no tx":      func(l *types.Log) { l.TxHash = common.Hash{} },
		"short data": func(l *types.Log) { l.Data = l.Data[:63] },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := log
			candidate.Topics = append([]common.Hash(nil), log.Topics...)
			candidate.Data = append([]byte(nil), log.Data...)
			mutate(&candidate)
			if _, err := DecodeV3PolicyLog(candidate, 31337, proxy); err == nil {
				t.Fatal("malformed V3 event accepted")
			}
		})
	}
}

func TestMatchV3PolicyEventRejectsAmbiguousOrUnboundReceipt(t *testing.T) {
	proxy := common.HexToAddress("0x1234567890123456789012345678901234567890")
	project := common.BigToHash(big.NewInt(41376991395446785))
	root := common.HexToHash("0x1234")
	log := &types.Log{Address: proxy, BlockNumber: 12, BlockHash: common.HexToHash("0xab"), TxHash: common.HexToHash("0xcd"), Index: 3, Topics: []common.Hash{v3PolicyUpdatedTopic, project, common.BigToHash(big.NewInt(7))}, Data: make([]byte, 64)}
	log.Data[31] = 1
	copy(log.Data[32:], root[:])
	event, err := DecodeV3PolicyLog(*log, 31337, proxy)
	if err != nil {
		t.Fatal(err)
	}
	receipt := &types.Receipt{Status: types.ReceiptStatusSuccessful, TxHash: log.TxHash, BlockHash: log.BlockHash, BlockNumber: big.NewInt(12), Logs: []*types.Log{log}}
	check := func() error {
		return MatchV3PolicyEvent(event, 31337, proxy, 41376991395446785, 7, true, root, receipt)
	}
	if err := check(); err != nil {
		t.Fatalf("valid receipt rejected: %v", err)
	}
	if err := MatchV3PolicyEvent(event, 31338, proxy, event.ProjectID, 7, true, root, receipt); err == nil {
		t.Fatal("wrong chain accepted")
	}
	if err := MatchV3PolicyEvent(event, 31337, proxy, event.ProjectID, 8, true, root, receipt); err == nil {
		t.Fatal("wrong version accepted")
	}
	receipt.Logs = append(receipt.Logs, log)
	if err := check(); err == nil {
		t.Fatal("duplicate project event accepted")
	}
	receipt.Logs = receipt.Logs[:1]
	receipt.Status = types.ReceiptStatusFailed
	if err := check(); err == nil {
		t.Fatal("failed receipt accepted")
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
	receipt.Logs = []*types.Log{log}
	altered := *log
	altered.Index = 4
	receipt.Logs = []*types.Log{&altered}
	if err := check(); err == nil {
		t.Fatal("log index mismatch accepted")
	}
	altered = *log
	altered.Topics = []common.Hash{v3PolicyUpdatedTopic}
	receipt.Logs = []*types.Log{&altered}
	if err := check(); err == nil {
		t.Fatal("malformed log accepted")
	}
}
