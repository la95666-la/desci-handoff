package chainevent

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"joincare_server/pkg/common/idov3whitelist"
)

// An RPC wrapper can reuse its own first result's memory on the next read.
// Filter equality must compare against an immutable first response.
type v3AliasedFilterRPC struct {
	*v3PinnedScanStub
	first []types.Log
}

func (s *v3AliasedFilterRPC) FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	s.filters++
	if s.filters == 1 {
		s.first = s.logs
		return s.first, nil
	}
	if s.filters == 2 {
		// Mutate the first result in place and return the same slice. A shallow
		// copy of []types.Log, Data or Topics cannot detect this drift.
		s.first[0].Data[0] ^= 1
		s.first[0].Topics[0] = crypto.Keccak256Hash([]byte("replaced"))
		return s.first, nil
	}
	return s.v3PinnedScanStub.FilterLogs(ctx, q)
}

func TestV3ScannerRejectsAliasedFilterMutation(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	impl := common.HexToAddress("0x4444444444444444444444444444444444444444")
	payment := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sale := common.HexToAddress("0x3333333333333333333333333333333333333333")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 7, ProxyAddress: proxy, PaymentToken: payment, ProjectToken: sale}
	runtime := idov3whitelist.RuntimeIdentity{Implementation: impl, ProxyCodeHash: crypto.Keccak256Hash([]byte{0x60, 0x80}), CodeHash: crypto.Keccak256Hash([]byte{0x60, 0x01})}
	head := &types.Header{Number: big.NewInt(90), ParentHash: common.HexToHash("0xa1")}
	// A known global event is included in the complete filter, not selected
	// for financial replay. Its mutation must still invalidate the scan.
	global := types.Log{Address: proxy, BlockNumber: 90, BlockHash: head.Hash(), TxHash: common.HexToHash("0xb1"), Index: 4,
		Topics: []common.Hash{crypto.Keccak256Hash([]byte("Paused(address)"))}, Data: []byte{0, 1}}
	stub := &v3AliasedFilterRPC{v3PinnedScanStub: &v3PinnedScanStub{chain: 31337, head: head, proxy: proxy, impl: impl, payment: payment, sale: sale, project: 7, tuple: v3PendingTuple(7, sale, payment), logs: []types.Log{global}}}
	if _, _, _, err := v3ScanBlock(context.Background(), stub, binding, runtime, 90, 3); err == nil {
		t.Fatal("aliased full proxy filter changed without rejection")
	}
}

func TestV3CopyLogsOwnsNestedResponseAndFullSnapshot(t *testing.T) {
	original := []types.Log{{Topics: []common.Hash{common.HexToHash("0x01")}, Data: []byte{1}}}
	copyA := v3CopyLogs(original)
	copyB := v3CopyLogs(copyA)
	original[0].Topics[0] = common.HexToHash("0x02")
	original[0].Data[0] = 2
	copyA[0].Topics[0] = common.HexToHash("0x03")
	copyA[0].Data[0] = 3
	if copyB[0].Topics[0] != common.HexToHash("0x01") || copyB[0].Data[0] != 1 {
		t.Fatal("nested log fields shared by complete response snapshots")
	}
}
