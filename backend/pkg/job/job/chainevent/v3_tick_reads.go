package chainevent

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Sharing is opt-in at first-observation call sites ONLY. All unmarked calls
// (including independent rechecks and admission) go directly to the source.
type v3FirstReadKey struct{}

func v3FirstRead(ctx context.Context) context.Context {
	return context.WithValue(ctx, v3FirstReadKey{}, true)
}

const (
	v3TickReadEntries = 4096
	v3TickReadBytes   = 32 << 20
	v3TickReadItem    = 1 << 20
)

type v3TickRead struct {
	done  chan struct{}
	value any // private deep snapshot, never a provider/consumer-owned pointer
	err   error
}

type v3TickReads struct {
	v3ScannerRPC
	ctx      context.Context // exact tick lifetime, never a project leader's deadline
	cancel   context.CancelFunc
	workers  sync.WaitGroup
	mu       sync.Mutex
	entries  map[string]*v3TickRead
	bytes    int
	shared   atomic.Uint64
	upstream atomic.Uint64
}

func newV3TickReads(ctx context.Context, rpc v3ScannerRPC) *v3TickReads {
	owner, cancel := context.WithCancel(ctx)
	return &v3TickReads{v3ScannerRPC: rpc, ctx: owner, cancel: cancel, entries: make(map[string]*v3TickRead)}
}

// Called only after all project workers exit. No detached read crosses ticks.
func (r *v3TickReads) close() { r.cancel(); r.workers.Wait() }

// Only successful bounded first observations are retained. A canceled waiter
// does not cancel another project's read; the owner tick/12s cap still does.
// Errors and missing receipts are not memoized. No write method is exposed.
func v3SharedRead[T any](ctx context.Context, r *v3TickReads, key string,
	read func(context.Context) (T, error), valid func(T) bool, clone func(T) T) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if ctx.Value(v3FirstReadKey{}) != true {
		return read(ctx)
	}
	if err := r.ctx.Err(); err != nil {
		return zero, err
	}
	r.mu.Lock()
	entry, found := r.entries[key]
	if found {
		r.shared.Add(1)
	} else if len(r.entries) >= v3TickReadEntries || r.bytes >= v3TickReadBytes {
		r.mu.Unlock()
		return read(ctx) // saturation changes cost, never authority
	} else {
		entry = &v3TickRead{done: make(chan struct{})}
		r.entries[key] = entry
		r.upstream.Add(1)
		r.workers.Add(1)
		go func() {
			defer r.workers.Done()
			requestCtx, cancel := context.WithTimeout(r.ctx, 12*time.Second)
			defer cancel()
			value, err := read(requestCtx)
			var data []byte
			if err == nil && !valid(value) {
				err = errors.New("V3 shared first observation unavailable")
			}
			if err == nil {
				value = clone(value)
				data, err = json.Marshal(value) // accounting only; no lossy receipt wire roundtrip
			}
			r.mu.Lock()
			entry.value, entry.err = value, err
			if err != nil || len(data) > v3TickReadItem || r.bytes+len(data) > v3TickReadBytes || r.ctx.Err() != nil {
				delete(r.entries, key)
			} else {
				r.bytes += len(data)
			}
			close(entry.done)
			r.mu.Unlock()
		}()
	}
	r.mu.Unlock()
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case <-r.ctx.Done():
		return zero, r.ctx.Err()
	case <-entry.done:
	}
	if entry.err != nil {
		return zero, entry.err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if err := r.ctx.Err(); err != nil {
		return zero, err
	}
	return clone(entry.value.(T)), nil
}

func (r *v3TickReads) HeaderByNumber(ctx context.Context, n *big.Int) (*types.Header, error) {
	// Latest/pending/invalid tags never share snapshots.
	if n == nil || !n.IsUint64() {
		return r.v3ScannerRPC.HeaderByNumber(ctx, n)
	}
	number := new(big.Int).Set(n)
	return v3SharedRead(ctx, r, "header:"+number.String(), func(c context.Context) (*types.Header, error) {
		return r.v3ScannerRPC.HeaderByNumber(c, number)
	}, func(h *types.Header) bool { return h != nil && h.Number != nil && h.Number.Cmp(number) == 0 }, types.CopyHeader)
}

func (r *v3TickReads) FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	if q.FromBlock == nil || q.ToBlock == nil || !q.FromBlock.IsUint64() || !q.ToBlock.IsUint64() || q.BlockHash != nil {
		return r.v3ScannerRPC.FilterLogs(ctx, q)
	}
	// The leader may outlive a canceled waiter. Own all query storage before
	// starting it, so caller mutation cannot change the request behind its key.
	q.FromBlock, q.ToBlock = new(big.Int).Set(q.FromBlock), new(big.Int).Set(q.ToBlock)
	q.Addresses = append([]common.Address(nil), q.Addresses...)
	topics := make([][]common.Hash, len(q.Topics))
	for i := range q.Topics {
		topics[i] = append([]common.Hash(nil), q.Topics[i]...)
	}
	q.Topics = topics
	// Encode the whole query: address, topics and both range bounds are identity.
	key, err := json.Marshal(q)
	if err != nil {
		return r.v3ScannerRPC.FilterLogs(ctx, q)
	}
	return v3SharedRead(ctx, r, "logs:"+string(key), func(c context.Context) ([]types.Log, error) { return r.v3ScannerRPC.FilterLogs(c, q) }, func([]types.Log) bool { return true }, v3CopyLogs)
}

func (r *v3TickReads) TransactionReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	return v3SharedRead(ctx, r, "receipt:"+hash.Hex(), func(c context.Context) (*types.Receipt, error) { return r.v3ScannerRPC.TransactionReceipt(c, hash) }, func(v *types.Receipt) bool { return v != nil && v.TxHash == hash }, v3CloneReceipt)
}

func v3CloneReceipt(source *types.Receipt) *types.Receipt {
	copy := *source
	copy.PostState = append([]byte(nil), source.PostState...)
	copy.Logs = make([]*types.Log, len(source.Logs))
	for i, log := range source.Logs {
		if log != nil {
			item := v3CopyLogs([]types.Log{*log})[0]
			copy.Logs[i] = &item
		}
	}
	for _, pair := range [][2]**big.Int{{&copy.BlockNumber, &source.BlockNumber}, {&copy.EffectiveGasPrice, &source.EffectiveGasPrice}, {&copy.BlobGasPrice, &source.BlobGasPrice}} {
		if *pair[1] != nil {
			*pair[0] = new(big.Int).Set(*pair[1])
		}
	}
	return &copy
}
