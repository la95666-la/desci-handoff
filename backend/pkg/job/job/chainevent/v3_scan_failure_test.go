package chainevent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	drivermysql "github.com/go-sql-driver/mysql"
)

func TestV3FailureClassificationDoesNotInferConflictFromTransient(t *testing.T) {
	for _, e := range []error{context.Canceled, context.DeadlineExceeded, io.EOF, io.ErrUnexpectedEOF, rpc.HTTPError{StatusCode: 401}, rpc.HTTPError{StatusCode: 403}, rpc.HTTPError{StatusCode: 429}, rpc.HTTPError{StatusCode: 503}, &drivermysql.MySQLError{Number: 1213}, &drivermysql.MySQLError{Number: 1205}} {
		if !v3Deferred(fmt.Errorf("wrapped: %w", e)) {
			t.Fatalf("temporary error not deferred %T", e)
		}
	}
	for _, e := range []error{errV3Stream, errV3StreamConflict, errors.New("context deadline exceeded text is not type evidence"), rpc.HTTPError{StatusCode: 200}, &drivermysql.MySQLError{Number: 1062}} {
		if v3Deferred(e) {
			t.Fatalf("integrity/unclassified failure deferred %T", e)
		}
	}
}

func TestV3StreamRevocationLocksStreamBeforeProject(t *testing.T) {
	for _, file := range []string{"v3_proxy_runtime.go", "v3_proxy_prepared_consumer.go"} {
		raw, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		s := string(raw)
		start := strings.LastIndex(s, "v3StreamWriterTx(cleanup")
		if start < 0 {
			t.Fatal("cleanup missing", file)
		}
		block := s[start:]
		lock := strings.Index(block, "v3StreamCursorRead(tx, s, true)")
		halt := strings.Index(block, "haltRangeUnknown(")
		if halt < 0 {
			halt = strings.Index(block, "haltApprovedV3Projection(")
		}
		if lock < 0 || halt < lock {
			t.Fatal("inverse stream/project cleanup lock order", file)
		}
	}
}

func TestV3SnapshotCASDoesNotMaskChangedCanonicalHash(t *testing.T) {
	old := v3StreamCursor{State: "running", Mode: "stream", Epoch: 2, CursorBlock: 100, CursorHash: "a", Revision: 3}
	changed := old
	changed.CursorHash = "b"
	if v3Deferred(v3StreamSnapshotChanged(old, changed)) {
		t.Fatal("same-height hash conflict deferred")
	}
	changed = old
	changed.CursorBlock--
	if v3Deferred(v3StreamSnapshotChanged(old, changed)) {
		t.Fatal("uncontrolled rewind deferred")
	}
	changed = old
	changed.Epoch++
	if !v3Deferred(v3StreamSnapshotChanged(old, changed)) {
		t.Fatal("controlled epoch switch mistaken for corruption")
	}
}

func TestV3TrackedFailurePreservesRedactedSourceAndConflictPrecedence(t *testing.T) {
	r := &v3TrackedRPC{}
	r.track(context.DeadlineExceeded)
	e := r.finish(context.Background(), "admission", errors.New("redacted underlying error"))
	if !v3Deferred(e) || e.Error() != "V3 scan deferred: admission" {
		t.Fatal(e)
	}
	if v3Deferred(r.finish(context.Background(), "anchor", errV3StreamConflict)) {
		t.Fatal("explicit conflict masked by earlier transient")
	}
	if r.finish(context.Background(), "complete", nil) != nil {
		t.Fatal("successful checks turned into failure")
	}
}

func TestProducerHTTPReadFailureIsUnavailableNotReorgAndKeepsConflictPrecedence(t *testing.T) {
	for _, code := range []int{400, 401, 403, 408, 429, 500, 502, 503, 504} {
		ctx := newV3ScanTrace(context.Background(), 0)
		r := &v3TrackedRPC{}
		err := rpc.HTTPError{StatusCode: code, Body: []byte("must not log source secret")}
		r.track(err)
		v3TraceSource(ctx, "eth_chainId", err)
		got := r.finish(ctx, "producer_unit", errV3Stream)
		if !v3Deferred(got) {
			t.Fatalf("HTTP %d halted an unavailable source", code)
		}
		if v3Deferred(r.finish(ctx, "producer_unit", errV3StreamConflict)) {
			t.Fatal("HTTP failure masked canonical conflict")
		}
		fields := v3ScanLogFields(ctx, 0, got, "defer")
		if fields["source_http_status"] != code || strings.Contains(fmt.Sprint(fields), "source secret") {
			t.Fatal(fields)
		}
	}
}

func TestV3TemporaryBackoffIsProjectLocalAndNeverSuccess(t *testing.T) {
	s := &v3RetryState{}
	calls := 0
	e := s.attempt(context.Background(), 1, func() error { calls++; return &v3ScanDeferred{"rpc", context.DeadlineExceeded} })
	if !v3Deferred(e) || calls != 1 {
		t.Fatal(e, calls)
	}
	if e := s.attempt(context.Background(), 1, func() error { t.Fatal("backoff bypassed"); return nil }); !v3Deferred(e) {
		t.Fatal("backoff reported success")
	}
	if e := s.attempt(context.Background(), 2, func() error { return nil }); e != nil {
		t.Fatal("other project starved", e)
	}
	s.mu.Lock()
	entry := s.projects[1]
	entry.after = time.Now().Add(-time.Second)
	s.projects[1] = entry
	s.mu.Unlock()
	if e := s.attempt(context.Background(), 1, func() error { return nil }); e != nil {
		t.Fatal(e)
	}
	if len(s.projects) != 0 {
		t.Fatal("success retained retry state")
	}
}

func TestV3FairScanTemporaryFailureDoesNotRevokeOrPublishComplete(t *testing.T) {
	for _, failure := range []error{context.DeadlineExceeded, rpc.HTTPError{StatusCode: 429}, &drivermysql.MySQLError{Number: 1213}, errV3StreamConflict, errV3Stream} {
		after := uint64(0)
		revoked := 0
		got, e := fairApprovedV3ScanWorkers(context.Background(), []uint64{1, 2}, &after, time.Second, 1, func(_ context.Context, id uint64) error {
			if id == 1 {
				return failure
			}
			return nil
		}, func(uint64) error { revoked++; return nil })
		if e == nil || got.Failed != 1 || got.Completed != 1 {
			t.Fatal("partial tick published full success", got, e)
		}
		want := 1
		if v3Deferred(failure) {
			want = 0
		}
		if revoked != want {
			t.Fatal("wrong revocation scope", failure, revoked)
		}
	}
}
