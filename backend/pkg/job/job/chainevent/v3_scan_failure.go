package chainevent

import (
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm/logger"
	"io"
	"math/big"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	drivermysql "github.com/go-sql-driver/mysql"
)

type v3ScanDeferred struct {
	stage string
	cause error
}

func (e *v3ScanDeferred) Error() string { return "V3 scan deferred: " + e.stage }
func (e *v3ScanDeferred) Unwrap() error { return e.cause }
func v3TemporaryRead(e error) bool {
	if errors.Is(e, io.EOF) || errors.Is(e, io.ErrUnexpectedEOF) || errors.Is(e, syscall.ECONNRESET) || errors.Is(e, syscall.ECONNREFUSED) || errors.Is(e, syscall.EPIPE) {
		return true
	}
	if errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) {
		return true
	}
	var n net.Error
	if errors.As(e, &n) {
		return n.Timeout() || n.Temporary()
	}
	var h rpc.HTTPError
	if errors.As(e, &h) {
		// HTTP rejection is source unavailability (including auth/quota/WAF),
		// never evidence that a confirmed block or contract identity changed.
		// Keep freshness closed and retry; only real integrity conflicts halt.
		return h.StatusCode >= 400 && h.StatusCode <= 599
	}
	return false
}
func v3Deferred(e error) bool {
	if e == nil || errors.Is(e, errV3StreamConflict) {
		return false
	}
	var d *v3ScanDeferred
	if errors.As(e, &d) {
		return true
	}
	var m *drivermysql.MySQLError
	if errors.As(e, &m) {
		return m.Number == 1213 || m.Number == 1205
	}
	return v3TemporaryRead(e)
}

// Legacy evidence helpers intentionally redact/wrap provider failures. Track
// only recognized transient source errors per operation, never raw text or a
// cross-project/global latch. Unclassified failures retain persistent closure.
type v3TrackedRPC struct {
	v3ScannerRPC
	mu        sync.Mutex
	temporary error
	reads     uint64
}

func (r *v3TrackedRPC) track(e error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reads++
	if v3TemporaryRead(e) {
		if r.temporary == nil {
			r.temporary = e
		}
	}
}
func (r *v3TrackedRPC) finish(ctx context.Context, stage string, e error) error {
	if e == nil || errors.Is(e, errV3StreamConflict) {
		return e
	}
	r.mu.Lock()
	source := r.temporary
	r.mu.Unlock()
	if source != nil {
		return &v3ScanDeferred{stage, source}
	}
	if v3Deferred(e) {
		return &v3ScanDeferred{stage, e}
	}
	return e
}
func (r *v3TrackedRPC) ChainID(c context.Context) (*big.Int, error) {
	v, e := r.v3ScannerRPC.ChainID(c)
	v3TraceSource(c, "eth_chainId", e)
	r.track(e)
	return v, e
}
func (r *v3TrackedRPC) BlockNumber(c context.Context) (uint64, error) {
	v, e := r.v3ScannerRPC.BlockNumber(c)
	v3TraceSource(c, "eth_blockNumber", e)
	r.track(e)
	return v, e
}
func (r *v3TrackedRPC) HeaderByNumber(c context.Context, n *big.Int) (*types.Header, error) {
	v, e := r.v3ScannerRPC.HeaderByNumber(c, n)
	v3TraceSource(c, "eth_getBlockByNumber", e)
	r.track(e)
	return v, e
}
func (r *v3TrackedRPC) CodeAt(c context.Context, a common.Address, n *big.Int) ([]byte, error) {
	v, e := r.v3ScannerRPC.CodeAt(c, a, n)
	v3TraceSource(c, "eth_getCode", e)
	r.track(e)
	return v, e
}
func (r *v3TrackedRPC) StorageAt(c context.Context, a common.Address, h common.Hash, n *big.Int) ([]byte, error) {
	v, e := r.v3ScannerRPC.StorageAt(c, a, h, n)
	v3TraceSource(c, "eth_getStorageAt", e)
	r.track(e)
	return v, e
}
func (r *v3TrackedRPC) CallContract(c context.Context, m ethereum.CallMsg, n *big.Int) ([]byte, error) {
	v, e := r.v3ScannerRPC.CallContract(c, m, n)
	v3TraceSource(c, "eth_call", e)
	r.track(e)
	return v, e
}
func (r *v3TrackedRPC) FilterLogs(c context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	v, e := r.v3ScannerRPC.FilterLogs(c, q)
	v3TraceSource(c, "eth_getLogs", e)
	r.track(e)
	return v, e
}
func (r *v3TrackedRPC) TransactionReceipt(c context.Context, h common.Hash) (*types.Receipt, error) {
	v, e := r.v3ScannerRPC.TransactionReceipt(c, h)
	v3TraceSource(c, "eth_getTransactionReceipt", e)
	r.track(e)
	return v, e
}
func v3FailureClass(e error) string {
	if v3Deferred(e) {
		return "deferred"
	}
	return "integrity_or_unclassified"
}
func v3Conflict(stage string) error { return fmt.Errorf("%w: %s", errV3StreamConflict, stage) }

func v3StreamSnapshotChanged(old, current v3StreamCursor) error {
	if current.Epoch != old.Epoch || current.State != old.State || current.Mode != old.Mode || current.AnchorBlock != old.AnchorBlock {
		return &v3ScanDeferred{"stream_control_changed", errV3Stream}
	}
	if current.CursorBlock < old.CursorBlock || current.Revision < old.Revision || (current.CursorBlock == old.CursorBlock && current.CursorHash != old.CursorHash) {
		return v3Conflict("stream_cursor_rewrite")
	}
	return &v3ScanDeferred{"stream_cas_changed", errV3Stream}
}

// Journal helpers redact errors. Preserve typed DB contention without parsing
// SQL/error text, and without letting a prior error mask explicit conflicts.
type v3DBFailures struct {
	logger.Interface
	mu        sync.Mutex
	temporary error
}

func (r *v3DBFailures) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), e error) {
	v3TraceDB(ctx, e)
	if v3Deferred(e) {
		r.mu.Lock()
		r.temporary = e
		r.mu.Unlock()
	}
	r.Interface.Trace(ctx, begin, fc, e)
}
func (r *v3DBFailures) finish(e error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e != nil && r.temporary != nil && !errors.Is(e, errV3StreamConflict) {
		return &v3ScanDeferred{"db_transaction", r.temporary}
	}
	return e
}
