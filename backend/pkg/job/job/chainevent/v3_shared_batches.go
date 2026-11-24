package chainevent

import (
	"context"
	"encoding/json"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"
	"joincare_server/pkg/common/idov3whitelist"
)

const v3SharedConsumerBlocks uint64 = 512
const v3CatchupConsumerBlocks uint64 = 1536

type v3CatchupBatchKey struct{}

func v3ConsumerBlockLimit(ctx context.Context) uint64 {
	if ctx == nil {
		return 128
	}
	if ctx.Value(v3SharedBatchKey{}) != true {
		return 128
	}
	if ctx.Value(v3CatchupBatchKey{}) == true {
		return v3CatchupConsumerBlocks
	}
	return v3SharedConsumerBlocks
}

type v3SharedBatchKey struct{}
type v3SharedConfirmedKey struct{}
type v3SharedConfirmed struct {
	height uint64
	hash   common.Hash
}

// Coverage is fetched in fixed 128-block pages, independent of a project's
// cursor offset. All consumers of overlapping pages reuse the same validated
// immutable evidence, yet receive separate deep copies.
func readV3SharedCoverage(ctx context.Context, db *gorm.DB, s v3StreamScope, stream v3StreamCursor, start, end uint64) (map[uint64]v3StreamBlock, error) {
	if end < start || end-start > v3ConsumerBlockLimit(ctx) || start < stream.AnchorBlock || end > stream.CursorBlock {
		return nil, errV3Stream
	}
	result := make(map[uint64]v3StreamBlock)
	bytes := 0
	for n := start; n <= end; {
		page := n / 128 * 128
		if page < stream.AnchorBlock {
			page = stream.AnchorBlock
		}
		last := n/128*128 + 127
		if last < n || last > stream.CursorBlock {
			last = stream.CursorBlock
		}
		blocks, err := readV3ConsumerCoverage(ctx, db, s, stream.Epoch, page, last)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(blocks)
		if err != nil {
			return nil, errV3Stream
		}
		bytes += len(raw)
		if bytes > 32<<20 {
			return nil, &v3ScanDeferred{stage: "shared_batch_bytes"}
		}
		stop := last
		if stop > end {
			stop = end
		}
		for h := n; h <= stop; h++ {
			b, ok := blocks[h]
			if !ok {
				return nil, errV3Stream
			}
			if h > start && b.header().ParentHash != result[h-1].header().Hash() {
				return nil, errV3StreamConflict
			}
			result[h] = b
		}
		if stop == math.MaxUint64 {
			break
		}
		n = stop + 1
	}
	return result, nil
}

// Routing only. A known project topic plus its indexed ID allows another
// project's event to bypass THIS project's heavy replay, not to authorize it.
// Global/governance and unknown/malformed topics must take the checked path.
func v3BlockAffectsProject(block v3StreamBlock, project uint64) (bool, error) {
	for _, l := range block.Logs {
		if len(l.Topics) == 0 {
			return false, errV3StreamConflict
		}
		_, financial := v3ProjectEventTopics[l.Topics[0]]
		if financial || l.Topics[0] == v3PolicyUpdateTopic {
			if len(l.Topics) < 2 || !new(big.Int).SetBytes(l.Topics[1][:]).IsUint64() || new(big.Int).SetBytes(l.Topics[1][:]).Sign() == 0 {
				return false, errV3StreamConflict
			}
			if new(big.Int).SetBytes(l.Topics[1][:]).Uint64() == project {
				return true, nil
			}
			continue
		}
		if _, global := v3NonProjectTopics[l.Topics[0]]; !global {
			return false, errV3StreamConflict
		}
		return true, nil
	}
	return false, nil
}

func prepareV3ProjectQuiet(ctx context.Context, rpc v3CoverageRPC, s v3StreamScope, b idov3whitelist.ExpectedProjectBinding, rt idov3whitelist.RuntimeIdentity, start, end uint64) (v3ConsumerUnit, error) {
	var zero v3ConsumerUnit
	if start == 0 || end < start || end-start >= v3CatchupConsumerBlocks {
		return zero, errV3Stream
	}
	u := v3ConsumerUnit{empty: true}
	for n := start; n <= end; n++ {
		block, ok := rpc.blocks[n]
		if !ok {
			return zero, errV3Stream
		}
		affects, err := v3BlockAffectsProject(block, b.ProjectID)
		if err != nil || affects {
			return zero, errV3Stream
		}
		for _, l := range block.Logs {
			receipt, ok := block.Receipts[l.TxHash.Hex()]
			if !ok || receipt == nil || v3StreamMatchReceipt((*types.Receipt)(receipt), l, s.Proxy) != nil {
				return zero, errV3StreamConflict
			}
		}
		u.headers = append(u.headers, types.CopyHeader(block.header()))
	}
	// No project events in this complete interval. One pinned end observation
	// is enough to compare immutable terms and counters to the existing replay;
	// duplicated start/end policy reads add no information about this interval.
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(b.ChainID), ProjectID: new(big.Int).SetUint64(b.ProjectID), ProxyAddress: b.ProxyAddress}
	observed, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, rt, end, rpc.blocks[end].header().Hash())
	if err != nil || idov3whitelist.MatchProjectIdentityObservation(b, observed) != nil {
		return zero, errV3Stream
	}
	u.observed = observed
	return u, nil
}
