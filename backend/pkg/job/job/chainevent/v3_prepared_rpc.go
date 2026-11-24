package chainevent

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"
)

// Request-local ordered evidence transcript, not a canonicality cache. The
// preparation executes EVERY original independent RPC; publication replays
// exactly that sequence with zero network under the public row lock. A new
// query/order/argument fails closed. Live postcommit checks remain mandatory.
type v3PreparedAnswer struct {
	key   string
	value any
}
type v3PreparedRPC struct {
	v3ScannerRPC
	replay       bool
	position     int
	answers      []v3PreparedAnswer
	deferNetwork bool
	pending      func() error
}

var errV3PreparedRead = errors.New("V3 live evidence required outside transaction")

type v3PreparedBudgetKey struct{}

func newV3PreparedRPC(r v3ScannerRPC) *v3PreparedRPC { return &v3PreparedRPC{v3ScannerRPC: r} }
func preparedRead[T any](ctx context.Context, r *v3PreparedRPC, method string, args any, read func() (T, error), copy func(T) T) (T, error) {
	var zero T
	if ctx.Err() != nil {
		return zero, ctx.Err()
	}
	raw, e := json.Marshal(args)
	if e != nil {
		return zero, errV3Stream
	}
	key := method + string(raw)
	if r.deferNetwork {
		if r.pending != nil {
			return zero, errV3PreparedRead
		}
		if r.position < len(r.answers) {
			if r.answers[r.position].key != key {
				return zero, errV3Stream
			}
			v, ok := r.answers[r.position].value.(T)
			if !ok {
				return zero, errV3Stream
			}
			r.position++
			return copy(v), nil
		}
		r.pending = func() error {
			v, e := read()
			if e != nil {
				return e
			}
			r.answers = append(r.answers, v3PreparedAnswer{key, copy(v)})
			return nil
		}
		return zero, errV3PreparedRead
	}
	if r.replay {
		if r.position >= len(r.answers) || r.answers[r.position].key != key {
			return zero, errV3Stream
		}
		v, ok := r.answers[r.position].value.(T)
		r.position++
		if !ok {
			return zero, errV3Stream
		}
		return copy(v), nil
	}
	v, e := read()
	if e != nil {
		return zero, e
	}
	v = copy(v)
	r.answers = append(r.answers, v3PreparedAnswer{key, v})
	return copy(v), nil
}
func copyBig(v *big.Int) *big.Int {
	if v == nil {
		return nil
	}
	return new(big.Int).Set(v)
}
func copyBytes(v []byte) []byte { return append([]byte(nil), v...) }
func (r *v3PreparedRPC) ChainID(c context.Context) (*big.Int, error) {
	return preparedRead(c, r, "chain", nil, func() (*big.Int, error) { return r.v3ScannerRPC.ChainID(c) }, copyBig)
}
func (r *v3PreparedRPC) BlockNumber(c context.Context) (uint64, error) {
	return preparedRead(c, r, "head", nil, func() (uint64, error) { return r.v3ScannerRPC.BlockNumber(c) }, func(v uint64) uint64 { return v })
}
func (r *v3PreparedRPC) HeaderByNumber(c context.Context, n *big.Int) (*types.Header, error) {
	return preparedRead(c, r, "header", n, func() (*types.Header, error) { return r.v3ScannerRPC.HeaderByNumber(c, n) }, func(h *types.Header) *types.Header {
		if h == nil {
			return nil
		}
		return types.CopyHeader(h)
	})
}
func (r *v3PreparedRPC) CodeAt(c context.Context, a common.Address, n *big.Int) ([]byte, error) {
	return preparedRead(c, r, "code", []any{a, n}, func() ([]byte, error) { return r.v3ScannerRPC.CodeAt(c, a, n) }, copyBytes)
}
func (r *v3PreparedRPC) StorageAt(c context.Context, a common.Address, h common.Hash, n *big.Int) ([]byte, error) {
	return preparedRead(c, r, "storage", []any{a, h, n}, func() ([]byte, error) { return r.v3ScannerRPC.StorageAt(c, a, h, n) }, copyBytes)
}
func (r *v3PreparedRPC) CallContract(c context.Context, m ethereum.CallMsg, n *big.Int) ([]byte, error) {
	return preparedRead(c, r, "call", []any{m, n}, func() ([]byte, error) { return r.v3ScannerRPC.CallContract(c, m, n) }, copyBytes)
}
func (r *v3PreparedRPC) FilterLogs(c context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	return preparedRead(c, r, "logs", q, func() ([]types.Log, error) { return r.v3ScannerRPC.FilterLogs(c, q) }, v3CopyLogs)
}
func (r *v3PreparedRPC) TransactionReceipt(c context.Context, h common.Hash) (*types.Receipt, error) {
	return preparedRead(c, r, "receipt", h, func() (*types.Receipt, error) { return r.v3ScannerRPC.TransactionReceipt(c, h) }, func(v *types.Receipt) *types.Receipt {
		if v == nil {
			return nil
		}
		return v3CloneReceipt(v)
	})
}
