package chaintx

import (
	"context"
	"errors"
	"math/big"
	"net"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

// Finite, secret-free diagnostics. Never return an endpoint, provider payload,
// raw transaction, address parameters or error.Error() to logs/HTTP.
type V3MaintenanceReadDiagnostic struct {
	Provider string `json:"provider"`
	Method   string `json:"rpc_method"`
	Kind     string `json:"rpc_error_kind"`
	Millis   int64  `json:"rpc_elapsed_ms"`
	Attempts int    `json:"rpc_attempts"`
}

// Reader is deliberately serial/request-local. Only safe read methods exist;
// it cannot sign, send, replace or release nonce fences. The underlying scanner
// HTTP transport already retries429; do not multiply those retries here.
type v3MaintenanceReader struct {
	RetiredNonceRPC
	provider   string
	diagnostic V3MaintenanceReadDiagnostic
}

func maintenanceReadKind(err error) (string, bool) {
	if err == nil {
		return "verification", false
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled", false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout", false
	}
	var n net.Error
	if errors.As(err, &n) {
		if n.Timeout() {
			return "timeout", true
		}
		return "network", true
	}
	var h rpc.HTTPError
	if errors.As(err, &h) {
		if h.StatusCode == 429 {
			return "rate_limited", false
		}
		if h.StatusCode >= 500 && h.StatusCode <= 599 {
			return "http_server", true
		}
		return "http_rejected", false
	}
	var r rpc.Error
	if errors.As(err, &r) {
		if r.ErrorCode() == -32005 {
			return "rate_limited", false
		}
		return "rpc_rejected", false
	}
	return "read_unavailable", false
}

func (r *v3MaintenanceReader) read(ctx context.Context, method string, fn func() error) error {
	started := time.Now()
	attempts := 0
	var err error
	for attempts < 2 {
		attempts++
		if err = ctx.Err(); err != nil {
			break
		}
		err = fn()
		_, retry := maintenanceReadKind(err)
		if !retry || attempts == 2 {
			break
		}
		// One bounded retry for transient transport/5xx failures, no replay of
		// mismatched observations, missing transactions or consensus conflicts.
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < 2*time.Second {
			break
		}
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			err = ctx.Err()
		case <-timer.C:
		}
	}
	kind, _ := maintenanceReadKind(err)
	r.diagnostic = V3MaintenanceReadDiagnostic{Provider: r.provider, Method: method, Kind: kind, Millis: time.Since(started).Milliseconds(), Attempts: attempts}
	return err
}

func (r *v3MaintenanceReader) ChainID(c context.Context) (v *big.Int, e error) {
	e = r.read(c, "eth_chainId", func() error { var err error; v, err = r.RetiredNonceRPC.ChainID(c); return err })
	return
}
func (r *v3MaintenanceReader) BlockNumber(c context.Context) (v uint64, e error) {
	e = r.read(c, "eth_blockNumber", func() error { var err error; v, err = r.RetiredNonceRPC.BlockNumber(c); return err })
	return
}
func (r *v3MaintenanceReader) HeaderByNumber(c context.Context, n *big.Int) (v *types.Header, e error) {
	e = r.read(c, "eth_getBlockByNumber", func() error { var err error; v, err = r.RetiredNonceRPC.HeaderByNumber(c, n); return err })
	return
}
func (r *v3MaintenanceReader) TransactionReceipt(c context.Context, h common.Hash) (v *types.Receipt, e error) {
	e = r.read(c, "eth_getTransactionReceipt", func() error { var err error; v, err = r.RetiredNonceRPC.TransactionReceipt(c, h); return err })
	return
}
func (r *v3MaintenanceReader) TransactionByHash(c context.Context, h common.Hash) (v *types.Transaction, p bool, e error) {
	e = r.read(c, "eth_getTransactionByHash", func() error { var err error; v, p, err = r.RetiredNonceRPC.TransactionByHash(c, h); return err })
	return
}
func (r *v3MaintenanceReader) NonceAt(c context.Context, a common.Address, n *big.Int) (v uint64, e error) {
	method := "eth_getTransactionCount_latest"
	if n != nil {
		method = "eth_getTransactionCount_confirmed"
	}
	e = r.read(c, method, func() error { var err error; v, err = r.RetiredNonceRPC.NonceAt(c, a, n); return err })
	return
}
func (r *v3MaintenanceReader) PendingNonceAt(c context.Context, a common.Address) (v uint64, e error) {
	e = r.read(c, "eth_getTransactionCount_pending", func() error { var err error; v, err = r.RetiredNonceRPC.PendingNonceAt(c, a); return err })
	return
}
