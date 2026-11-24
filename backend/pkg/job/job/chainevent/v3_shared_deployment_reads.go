package chainevent

import (
	"context"
	"encoding/json"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// Tick-local immutable public deployment observations, pinned to headers read
// LIVE by the surrounding verifier. Never caches headers/head, project calls,
// policy, receipts, errors or unpinned/latest state. Source is never reused
// across ticks/epochs. Changed header immediately selects a new namespace.
type v3DeploymentReads struct {
	v3ScannerRPC
	mu             sync.Mutex
	headers        map[uint64]common.Hash
	values         map[string][]byte
	bytes          int
	hits, upstream uint64
}

func newV3DeploymentReads(r v3ScannerRPC) *v3DeploymentReads {
	return &v3DeploymentReads{v3ScannerRPC: r, headers: make(map[uint64]common.Hash), values: make(map[string][]byte)}
}
func (r *v3DeploymentReads) HeaderByNumber(ctx context.Context, n *big.Int) (*types.Header, error) {
	h, err := r.v3ScannerRPC.HeaderByNumber(ctx, n)
	if n != nil && n.IsUint64() {
		r.mu.Lock()
		delete(r.headers, n.Uint64())
		if err == nil && h != nil && h.Number != nil && h.Number.Cmp(n) == 0 && len(r.headers) < 4096 {
			r.headers[n.Uint64()] = h.Hash()
		}
		r.mu.Unlock()
	}
	return h, err
}
func (r *v3DeploymentReads) read(ctx context.Context, n *big.Int, key string, load func() ([]byte, error)) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if n == nil || !n.IsUint64() {
		return load()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.headers[n.Uint64()]
	if !ok {
		return load()
	}
	key = n.String() + ":" + h.Hex() + ":" + key
	if b, ok := r.values[key]; ok {
		r.hits++
		return append([]byte(nil), b...), nil
	}
	r.upstream++
	b, err := load()
	if err != nil {
		return nil, err
	}
	if len(r.values) < 4096 && len(b) <= 256<<10 && r.bytes+len(b) <= 8<<20 {
		r.values[key] = append([]byte(nil), b...)
		r.bytes += len(b)
	}
	return append([]byte(nil), b...), nil
}

func (r *v3DeploymentReads) stats() (uint64, uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hits, r.upstream
}
func (r *v3DeploymentReads) CodeAt(ctx context.Context, a common.Address, n *big.Int) ([]byte, error) {
	return r.read(ctx, n, "code:"+a.Hex(), func() ([]byte, error) { return r.v3ScannerRPC.CodeAt(ctx, a, n) })
}
func (r *v3DeploymentReads) StorageAt(ctx context.Context, a common.Address, k common.Hash, n *big.Int) ([]byte, error) {
	return r.read(ctx, n, "storage:"+a.Hex()+":"+k.Hex(), func() ([]byte, error) { return r.v3ScannerRPC.StorageAt(ctx, a, k, n) })
}
func (r *v3DeploymentReads) CallContract(ctx context.Context, m ethereum.CallMsg, n *big.Int) ([]byte, error) {
	shared := false
	for _, sig := range []string{"contractVersion()", "owner()", "whitelistGovernor()", "operator()"} {
		if string(m.Data) == string(crypto.Keccak256([]byte(sig))[:4]) {
			shared = true
			break
		}
	}
	if !shared || m.To == nil || m.From != (common.Address{}) || m.Value != nil {
		return r.v3ScannerRPC.CallContract(ctx, m, n)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return r.v3ScannerRPC.CallContract(ctx, m, n)
	}
	return r.read(ctx, n, "call:"+string(raw), func() ([]byte, error) { return r.v3ScannerRPC.CallContract(ctx, m, n) })
}
