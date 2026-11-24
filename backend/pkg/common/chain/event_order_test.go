package chain

import (
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
)

func TestSortEventLogsUsesCanonicalExecutionOrder(t *testing.T) {
	logs := []types.Log{
		{BlockNumber: 101, TxIndex: 0, Index: 0},
		{BlockNumber: 100, TxIndex: 2, Index: 5},
		{BlockNumber: 100, TxIndex: 1, Index: 9},
		{BlockNumber: 100, TxIndex: 2, Index: 4},
	}
	sortEventLogs(logs)
	want := []struct {
		block uint64
		tx    uint
		index uint
	}{{100, 1, 9}, {100, 2, 4}, {100, 2, 5}, {101, 0, 0}}
	for i, expected := range want {
		if logs[i].BlockNumber != expected.block || logs[i].TxIndex != expected.tx || logs[i].Index != expected.index {
			t.Fatalf("log %d order=%d/%d/%d want=%d/%d/%d", i, logs[i].BlockNumber, logs[i].TxIndex, logs[i].Index, expected.block, expected.tx, expected.index)
		}
	}
}
