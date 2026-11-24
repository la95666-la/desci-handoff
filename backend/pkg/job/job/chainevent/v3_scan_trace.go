package chainevent

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	drivermysql "github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"

	"joincare_server/pkg/common/log"
)

type v3ScanTraceKey struct{}
type v3ScanTrace struct {
	mu                                       sync.Mutex
	project                                  uint64
	phase                                    string
	started                                  time.Time
	cursor, revision, epoch, from, to, block uint64
	proxyLogs, selectedLogs, purchases       int
	rpcMethod, sourceKind                    string
	httpStatus                               int
	dbCode                                   uint16
	consumerFrom, consumerTo, producerTip    uint64
	consumerObserved                         bool
}

func v3TraceConsumerProgress(ctx context.Context, from, to, tip uint64) {
	if t, ok := ctx.Value(v3ScanTraceKey{}).(*v3ScanTrace); ok {
		t.mu.Lock()
		t.consumerFrom, t.consumerTo, t.producerTip, t.consumerObserved = from, to, tip, true
		t.mu.Unlock()
	}
}

func logV3ConsumerProgress(ctx context.Context, project uint64) {
	t, ok := ctx.Value(v3ScanTraceKey{}).(*v3ScanTrace)
	if !ok {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.consumerObserved {
		return
	}
	if logger := log.GetLogger(); logger != nil {
		logger.WithFields(logrus.Fields{"project_id": strconv.FormatUint(project, 10), "cursor_before": t.consumerFrom,
			"cursor_after": t.consumerTo, "producer_tip": t.producerTip, "advanced_blocks": t.consumerTo - t.consumerFrom,
			"consumer_lag": t.producerTip - t.consumerTo, "caught_up_to_snapshot": t.consumerTo == t.producerTip,
			"elapsed_ms": time.Since(t.started).Milliseconds()}).Info("V3 project consumer progress")
	}
}

func newV3ScanTrace(ctx context.Context, project uint64) context.Context {
	return context.WithValue(ctx, v3ScanTraceKey{}, &v3ScanTrace{project: project, phase: "dispatch", started: time.Now()})
}

// Fixed call-site literals only. Never pass provider text, SQL or a wallet.
func v3TracePhase(ctx context.Context, phase string) {
	if t, ok := ctx.Value(v3ScanTraceKey{}).(*v3ScanTrace); ok {
		t.mu.Lock()
		t.phase = phase
		t.mu.Unlock()
	}
}
func v3TraceWindow(ctx context.Context, cursor, revision, epoch, from, to uint64) {
	if t, ok := ctx.Value(v3ScanTraceKey{}).(*v3ScanTrace); ok {
		t.mu.Lock()
		t.cursor = cursor
		t.revision = revision
		t.epoch = epoch
		t.from = from
		t.to = to
		t.mu.Unlock()
	}
}
func v3TraceBlock(ctx context.Context, block uint64, proxyLogs, selectedLogs, purchases int) {
	if t, ok := ctx.Value(v3ScanTraceKey{}).(*v3ScanTrace); ok {
		t.mu.Lock()
		t.block = block
		t.proxyLogs = proxyLogs
		t.selectedLogs = selectedLogs
		t.purchases = purchases
		t.mu.Unlock()
	}
}
func v3TraceSource(ctx context.Context, method string, err error) {
	if err == nil {
		return
	}
	if t, ok := ctx.Value(v3ScanTraceKey{}).(*v3ScanTrace); ok {
		t.mu.Lock()
		t.rpcMethod = method
		t.sourceKind = v3LogErrorKind(err)
		var status rpc.HTTPError
		if errors.As(err, &status) {
			t.httpStatus = status.StatusCode
		}
		t.mu.Unlock()
	}
}
func v3TraceDB(ctx context.Context, err error) {
	if err == nil {
		return
	}
	if t, ok := ctx.Value(v3ScanTraceKey{}).(*v3ScanTrace); ok {
		t.mu.Lock()
		var mysql *drivermysql.MySQLError
		if errors.As(err, &mysql) {
			t.dbCode = mysql.Number
		}
		t.mu.Unlock()
	}
}
func v3LogErrorKind(err error) string {
	if err == nil {
		return "none"
	}
	if errors.Is(err, errV3StreamConflict) {
		return "evidence_conflict"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	var mysql *drivermysql.MySQLError
	if errors.As(err, &mysql) {
		return "database"
	}
	var http rpc.HTTPError
	if errors.As(err, &http) {
		return "rpc_http"
	}
	var node rpc.Error
	if errors.As(err, &node) {
		return "rpc_response"
	}
	var network net.Error
	if errors.As(err, &network) {
		return "network"
	}
	if v3Deferred(err) {
		return "deferred"
	}
	return "unclassified"
}

func v3ScanLogFields(ctx context.Context, project uint64, err error, decision string) logrus.Fields {
	f := logrus.Fields{"project_id": strconv.FormatUint(project, 10), "failure_class": v3FailureClass(err), "error_kind": v3LogErrorKind(err), "decision": decision}
	if t, ok := ctx.Value(v3ScanTraceKey{}).(*v3ScanTrace); ok {
		t.mu.Lock()
		defer t.mu.Unlock()
		f["phase"] = t.phase
		f["source_http_status"] = t.httpStatus
		f["cursor_block"] = t.cursor
		f["cursor_revision"] = t.revision
		f["epoch"] = t.epoch
		f["window_from"] = t.from
		f["window_to"] = t.to
		f["block_number"] = t.block
		f["proxy_log_count"] = t.proxyLogs
		f["selected_log_count"] = t.selectedLogs
		f["purchase_count"] = t.purchases
		f["rpc_method"] = t.rpcMethod
		f["source_error_kind"] = t.sourceKind
		f["db_error_number"] = t.dbCode
		f["elapsed_ms"] = time.Since(t.started).Milliseconds()
	}
	if ctx.Err() != nil {
		f["context_status"] = v3LogErrorKind(ctx.Err())
	} else {
		f["context_status"] = "active"
	}
	return f
}
func logV3ScanFailure(ctx context.Context, project uint64, err error, decision string) {
	if logger := log.GetLogger(); logger != nil {
		logger.WithFields(v3ScanLogFields(ctx, project, err, decision)).Warn("V3 scan failure decision")
	}
}
