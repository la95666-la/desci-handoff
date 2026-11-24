package chainevent

import (
	"context"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"
)

// Current reviewed provider plan permits at most five block heights per log
// query. Never infer emptiness from an error/413 or project-specific topics.
// The caller repeats this complete segmented pass independently before commit.
func v3ReadBoundedLogs(ctx context.Context, rpc v3ScannerRPC, q ethereum.FilterQuery) ([]types.Log, error) {
	if q.BlockHash != nil || q.FromBlock == nil || q.ToBlock == nil || !q.FromBlock.IsUint64() || !q.ToBlock.IsUint64() || q.FromBlock.Cmp(q.ToBlock) > 0 || q.ToBlock.Uint64()-q.FromBlock.Uint64() >= 128 {
		return nil, errV3Stream
	}
	out := []types.Log{}
	end := q.ToBlock.Uint64()
	for start := q.FromBlock.Uint64(); ; {
		last := start + 4
		if last < start || last > end {
			last = end
		}
		part := q
		part.FromBlock = new(big.Int).SetUint64(start)
		part.ToBlock = new(big.Int).SetUint64(last)
		logs, e := rpc.FilterLogs(ctx, part)
		if e != nil {
			return nil, errV3Stream
		} // no credential/provider body leakage
		for _, l := range logs {
			if l.BlockNumber < start || l.BlockNumber > last {
				return nil, errV3Stream
			}
		}
		out = append(out, v3CopyLogs(logs)...)
		if len(out) > 4096 {
			return nil, errV3Stream
		}
		if last == end {
			break
		}
		start = last + 1
	}
	return out, nil
}
