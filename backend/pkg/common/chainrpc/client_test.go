package chainrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func request(t *testing.T, ctx context.Context, body string) *http.Request {
	t.Helper()
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://example.invalid/private", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestOnlyExplicitReadsAreReplayable(t *testing.T) {
	for _, method := range []string{"eth_call", "eth_getLogs", "eth_getCode", "eth_getStorageAt", "eth_chainId", "eth_blockNumber", "eth_getBlockByNumber", "eth_getTransactionReceipt", "eth_getTransactionCount", "eth_estimateGas", "eth_gasPrice"} {
		if !retryableRead(request(t, context.Background(), `{"method":"`+method+`"}`)) {
			t.Fatal(method)
		}
	}
	for _, body := range []string{`{"method":"eth_sendRawTransaction"}`, `{"method":"eth_sendTransaction"}`, `{"method":"eth_sign"}`, `{"method":"personal_sign"}`, `{"method":"unknown"}`, `[{"method":"eth_call"}]`, `{}`, `not json`,
		`{"method":"eth_sendRawTransaction","method":"eth_call"}`, `{"method":"eth_call"} {"method":"eth_sendRawTransaction"}`} {
		calls := 0
		transport := NewTransport(t.Name() + body)
		transport.Base = roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 429, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(""))}, nil
		})
		r, err := transport.RoundTrip(request(t, context.Background(), body))
		if err != nil || calls != 1 {
			t.Fatalf("unsafe replay: calls=%d err=%v", calls, err)
		}
		r.Body.Close()
	}
}

func TestReadRetryPreservesBodyAndOnlyRetries429(t *testing.T) {
	for _, status := range []int{429, 500, 401} {
		calls := 0
		original := []byte(`{"method":"eth_call","params":[{"to":"0x01"},"0x123"]}`)
		transport := NewTransport(t.Name() + string(rune(status)))
		transport.Base = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(r.Body)
			r.Body.Close()
			if !bytes.Equal(body, original) {
				t.Error("changed pinned call")
			}
			calls++
			code := status
			if calls > 1 {
				code = 200
			}
			return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString("{}"))}, nil
		})
		r, err := transport.RoundTrip(request(t, context.Background(), string(original)))
		want := 1
		if status == 429 {
			want = 2
		}
		if err != nil || calls != want {
			t.Fatalf("status=%d calls=%d err=%v", status, calls, err)
		}
		r.Body.Close()
	}
}

func TestReadRateBudgetSharedAndQueueCancellation(t *testing.T) {
	endpoint := "https://" + t.Name() + ".quiknode.pro/key"
	a, b := NewTransport(endpoint), NewTransport(endpoint)
	if a.state != b.state {
		t.Fatal("new client reset lane")
	}
	entered, release := make(chan struct{}), make(chan struct{})
	a.Base = roundTripFunc(func(*http.Request) (*http.Response, error) {
		close(entered)
		<-release
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString("{}"))}, nil
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		r, e := a.RoundTrip(request(t, context.Background(), `{"method":"eth_call"}`))
		if e == nil {
			r.Body.Close()
		}
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	var calls atomic.Int32
	b.Base = roundTripFunc(func(*http.Request) (*http.Response, error) { calls.Add(1); return nil, errors.New("unexpected") })
	_, err := b.RoundTrip(request(t, ctx, `{"method":"eth_call"}`))
	close(release)
	<-done
	if !errors.Is(err, context.DeadlineExceeded) || calls.Load() != 0 {
		t.Fatalf("queued cancellation: %v", err)
	}
	previous := a.state.last
	var at time.Time
	b.Base = roundTripFunc(func(*http.Request) (*http.Response, error) {
		at = time.Now()
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString("{}"))}, nil
	})
	r, err := b.RoundTrip(request(t, context.Background(), `{"method":"eth_call"}`))
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if at.IsZero() {
		t.Fatal("second client did not issue read")
	}
	if at.Sub(previous) < 300*time.Millisecond {
		t.Fatal("second client bypassed shared pacing")
	}
	if a.interval != 350*time.Millisecond {
		t.Fatal("commercial spacing missing")
	}
}

func TestPersistent429BoundedByContextAndRetryAfter(t *testing.T) {
	for _, after := range []string{"", "30"} {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		calls := 0
		transport := NewTransport(t.Name() + after)
		transport.Base = roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{after}}, Body: io.NopCloser(bytes.NewBufferString("{}"))}, nil
		})
		r, err := transport.RoundTrip(request(t, ctx, `{"method":"eth_getTransactionReceipt"}`))
		cancel()
		if after == "" && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("deadline: %v", err)
		}
		if after != "" && (err != nil || r == nil || r.StatusCode != 429) {
			t.Fatal("long delay not failed closed")
		}
		if r != nil {
			r.Body.Close()
		}
		if calls != 1 {
			t.Fatal("deadline bypassed")
		}
	}
}

func TestEthereumClientNeverReplaysRawTransaction(t *testing.T) {
	var reads, writes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var call struct {
			Method string          `json:"method"`
			ID     json.RawMessage `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&call)
		if call.Method == "eth_sendRawTransaction" {
			writes.Add(1)
			w.WriteHeader(429)
			return
		}
		if reads.Add(1) == 1 {
			w.WriteHeader(429)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": call.ID, "result": "0x38"})
	}))
	defer server.Close()
	client, err := DialRPC(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var result string
	if err := client.CallContext(ctx, &result, "eth_chainId"); err != nil || result != "0x38" || reads.Load() != 2 {
		t.Fatalf("read retry: %v", err)
	}
	if err := client.CallContext(ctx, &result, "eth_sendRawTransaction", "0x1234"); err == nil || writes.Load() != 1 {
		t.Fatal("raw transaction replayed")
	}
}

func TestReplayBodyAndRedirectFailuresRemainClosed(t *testing.T) {
	r := request(t, context.Background(), `{"method":"eth_call"}`)
	r.GetBody = nil
	if retryableRead(r) {
		t.Fatal("non-replayable body accepted")
	}
	if retryableRead(request(t, context.Background(), strings.Repeat("x", 65537))) {
		t.Fatal("oversize body accepted")
	}
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1); w.WriteHeader(200) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", target.URL)
		w.WriteHeader(307)
	}))
	defer source.Close()
	c, err := DialRPC(context.Background(), source.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var result string
	if err := c.CallContext(context.Background(), &result, "eth_chainId"); err == nil || redirected.Load() != 0 {
		t.Fatal("RPC request followed redirect")
	}
}

func TestRead429RetriesAreBoundedAndTransportErrorsNeverRetried(t *testing.T) {
	for _, transportFailure := range []bool{false, true} {
		calls := 0
		transport := NewTransport(t.Name() + strconv.FormatBool(transportFailure))
		transport.Base = roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			if transportFailure {
				return nil, errors.New("connection outcome unavailable")
			}
			return &http.Response{StatusCode: 429, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}"))}, nil
		})
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		r, err := transport.RoundTrip(request(t, ctx, `{"method":"eth_getLogs"}`))
		cancel()
		if transportFailure {
			if err == nil || calls != 1 {
				t.Fatal("connection error retried")
			}
		} else {
			if err != nil || r == nil || r.StatusCode != 429 || calls != 4 {
				t.Fatalf("unbounded retry calls=%d err=%v", calls, err)
			}
			r.Body.Close()
		}
	}
}

func TestCancelledWriteAndJSONRPCErrorAreNeverRetried(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	transport := NewTransport(t.Name())
	transport.Base = roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"jsonrpc":"2.0","id":1,"error":{"code":-32005,"message":"rate limited"}}`))}, nil
	})
	if _, err := transport.RoundTrip(request(t, ctx, `{"method":"eth_sendRawTransaction"}`)); !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatal("cancelled write reached transport")
	}
	r, err := transport.RoundTrip(request(t, context.Background(), `{"method":"eth_call"}`))
	if err != nil || calls != 1 {
		t.Fatal("JSON-RPC failure replayed")
	}
	r.Body.Close()
}

func TestScannerHasIndependentBoundedLaneWithoutCorePacing(t *testing.T) {
	endpoint := "https://" + t.Name() + ".quiknode.pro/private"
	core := NewTransport(endpoint)
	a, b := NewScannerTransport(endpoint), NewScannerTransport(endpoint)
	if a.state == core.state || a.state != b.state || a.interval != 0 || core.interval != 350*time.Millisecond {
		t.Fatal("scanner inherits Core budget or resets lane")
	}
	entered := make(chan struct{}, 8)
	release := make(chan struct{})
	base := roundTripFunc(func(*http.Request) (*http.Response, error) {
		entered <- struct{}{}
		<-release
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	})
	a.Base = base
	b.Base = base
	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func(i int) {
			transport := a
			if i%2 == 0 {
				transport = b
			}
			r, err := transport.RoundTrip(request(t, context.Background(), `{"method":"eth_getBlockByNumber"}`))
			if r != nil {
				r.Body.Close()
			}
			done <- err
		}(i)
	}
	for i := 0; i < 4; i++ {
		select {
		case <-entered:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("scanner requests serialized")
		}
	}
	select {
	case <-entered:
		close(release)
		t.Fatal("scanner concurrency unbounded")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	for i := 0; i < 5; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}

func TestScannerRetriesPinnedReadsButNeverWrites(t *testing.T) {
	for _, method := range []string{"eth_getBlockByNumber", "eth_sendRawTransaction"} {
		calls := 0
		transport := NewScannerTransport(t.Name() + method)
		payload := `{"method":"` + method + `","params":["0x123",false]}`
		transport.Base = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(r.Body)
			r.Body.Close()
			if string(body) != payload {
				t.Fatal("scanner changed pinned payload")
			}
			calls++
			return &http.Response{StatusCode: 429, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}"))}, nil
		})
		r, err := transport.RoundTrip(request(t, context.Background(), payload))
		want := 1
		if method == "eth_getBlockByNumber" {
			want = 4
		}
		if err != nil || calls != want {
			t.Fatalf("scanner method=%s calls=%d err=%v", method, calls, err)
		}
		r.Body.Close()
	}
}
