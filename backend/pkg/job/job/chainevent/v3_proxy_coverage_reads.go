package chainevent

import (
	"context"
	"encoding/json"
	"github.com/ethereum/go-ethereum/core/types"
	"sync"

	"gorm.io/gorm"
)

type v3CoverageReadKey struct{}
type v3CoverageReadID struct {
	s                 v3StreamScope
	epoch, start, end uint64
}
type v3CoverageReads struct {
	mu      sync.Mutex
	db      *gorm.DB
	entries map[v3CoverageReadID]map[uint64]v3StreamBlock
	bytes   int
}

// Exact tick / DB / epoch scoped immutable evidence only. No RPC authority,
// cursor, policy decision or mutable project result is retained here.
func newV3CoverageReads(ctx context.Context, db *gorm.DB) context.Context {
	return context.WithValue(ctx, v3CoverageReadKey{}, &v3CoverageReads{db: db, entries: make(map[v3CoverageReadID]map[uint64]v3StreamBlock)})
}
func readV3ConsumerCoverage(ctx context.Context, db *gorm.DB, s v3StreamScope, epoch, start, end uint64) (map[uint64]v3StreamBlock, error) {
	cache, ok := ctx.Value(v3CoverageReadKey{}).(*v3CoverageReads)
	// Never memoize reads from an uncommitted test/bootstrap/recovery writer.
	_, nested := db.Statement.ConnPool.(gorm.TxCommitter)
	if !ok || cache.db.Statement.ConnPool != db.Statement.ConnPool || nested {
		return readV3Coverage(db.WithContext(ctx), s, start, end)
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	key := v3CoverageReadID{s: s, epoch: epoch, start: start, end: end}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if blocks, ok := cache.entries[key]; ok {
		return cloneV3Coverage(blocks), nil
	}
	before, e := v3StreamCursorRead(db.WithContext(ctx), s, false)
	if e != nil || before.Epoch != epoch || before.State != "running" || before.Mode != "stream" {
		return nil, errV3Stream
	}
	blocks, e := readV3Coverage(db.WithContext(ctx), s, start, end)
	if e != nil {
		return nil, e
	}
	// The joined epoch must still equal the requested namespace before store.
	current, e := v3StreamCursorRead(db.WithContext(ctx), s, false)
	if e != nil || current.Epoch != epoch || current.State != "running" || current.Mode != "stream" {
		return nil, errV3Stream
	}
	raw, e := json.Marshal(blocks)
	if e != nil {
		return nil, errV3Stream
	}
	if len(cache.entries) < 128 && len(raw) <= 8<<20 && cache.bytes+len(raw) <= 32<<20 {
		cache.entries[key] = cloneV3Coverage(blocks)
		cache.bytes += len(raw)
	}
	return blocks, nil
}
func cloneV3Coverage(source map[uint64]v3StreamBlock) map[uint64]v3StreamBlock {
	out := make(map[uint64]v3StreamBlock, len(source))
	for n, b := range source {
		copy := b
		if b.Header != nil {
			copy.Header = (*v3StreamHeader)(types.CopyHeader(b.header()))
		}
		copy.Logs = v3CopyLogs(b.Logs)
		copy.RetiredTests = append([]uint64(nil), b.RetiredTests...)
		copy.Receipts = make(map[string]*v3StreamReceipt, len(b.Receipts))
		for hash, r := range b.Receipts {
			copy.Receipts[hash] = (*v3StreamReceipt)(v3CloneReceipt((*types.Receipt)(r)))
		}
		out[n] = copy
	}
	return out
}
