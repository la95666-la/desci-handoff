// Package chainrpc applies bounded read-only HTTP rate-limit handling to
// Ethereum JSON-RPC. Writes are NEVER replayed, even after HTTP 429.
package chainrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type lane struct {
	once     sync.Once
	gate     chan struct{}
	last     time.Time
	capacity int
}

// Shared only within a process. Core and Job sharing a provider credential
// still need independent provider quotas; this is not a distributed limiter.
var lanes sync.Map
var scannerLanes sync.Map

type Transport struct {
	Base     http.RoundTripper
	state    *lane
	interval time.Duration
}

// NewTransport shares a lane for the exact endpoint, including its credential.
// The endpoint is never logged. Public/local nodes retain their normal pace;
// reviewed commercial providers are spaced like the existing Core reader.
func NewTransport(endpoint string) *Transport {
	state, _ := lanes.LoadOrStore(endpoint, &lane{})
	interval := time.Duration(0)
	if u, err := url.Parse(endpoint); err == nil && strings.HasSuffix(strings.ToLower(u.Hostname()), ".quiknode.pro") {
		interval = 350 * time.Millisecond
	}
	return &Transport{Base: http.DefaultTransport, state: state.(*lane), interval: interval}
}

// NewScannerTransport is for a read-only scanner, not a signing client.
// Core's serial 350ms interactive policy cannot sustain sequential header
// audits for multiple projects within a 50s Job tick. Scanner reads have no
// artificial delay, but share a maximum of four in-flight requests for this
// endpoint and retain the exact same bounded 429/backoff/write-no-retry rules.
func NewScannerTransport(endpoint string) *Transport {
	state, _ := scannerLanes.LoadOrStore(endpoint, &lane{capacity: 4})
	return &Transport{Base: http.DefaultTransport, state: state.(*lane)}
}

func retryableRead(req *http.Request) bool {
	if req.Method != http.MethodPost || req.GetBody == nil {
		return false
	}
	body, err := req.GetBody()
	if err != nil {
		return false
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, 65537))
	if err != nil || len(data) > 65536 {
		return false
	}
	// Reject duplicate JSON keys: a proxy must not interpret a different
	// method from the one classified here. Batches/trailing JSON fail closed.
	dec := json.NewDecoder(bytes.NewReader(data))
	start, err := dec.Token()
	if err != nil || start != json.Delim('{') {
		return false
	}
	seen := map[string]bool{}
	method := ""
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return false
		}
		name, ok := key.(string)
		if !ok || seen[name] {
			return false
		}
		seen[name] = true
		var value json.RawMessage
		if dec.Decode(&value) != nil {
			return false
		}
		if name == "method" && json.Unmarshal(value, &method) != nil {
			return false
		}
	}
	if end, err := dec.Token(); err != nil || end != json.Delim('}') {
		return false
	}
	if dec.Decode(new(any)) != io.EOF {
		return false
	}
	switch method {
	case "eth_chainId", "net_version", "eth_blockNumber", "eth_getBlockByNumber", "eth_getBlockByHash",
		"eth_getTransactionReceipt", "eth_getTransactionByHash", "eth_getTransactionCount", "eth_getBalance",
		"eth_getCode", "eth_getStorageAt", "eth_call", "eth_getLogs", "eth_estimateGas",
		"eth_gasPrice", "eth_maxPriorityFeePerGas", "eth_feeHistory":
		return true
	}
	return false
}

func wait(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	if !retryableRead(req) {
		return base.RoundTrip(req)
	}
	state := t.state
	if state == nil {
		return nil, errors.New("RPC read lane unavailable")
	}
	state.once.Do(func() {
		n := state.capacity
		if n == 0 {
			n = 1
		}
		state.gate = make(chan struct{}, n)
		for i := 0; i < n; i++ {
			state.gate <- struct{}{}
		}
	})
	select {
	case <-req.Context().Done():
		return nil, req.Context().Err()
	case <-state.gate:
	}
	defer func() { state.gate <- struct{}{} }()
	backoff := time.Duration(0)
	for attempt := 0; attempt < 4; attempt++ {
		if err := req.Context().Err(); err != nil {
			return nil, err
		}
		delay := time.Duration(0)
		if t.interval > 0 {
			delay = time.Until(state.last.Add(t.interval))
		}
		if backoff > delay {
			delay = backoff
		}
		if err := wait(req.Context(), delay); err != nil {
			return nil, err
		}
		request := req.Clone(req.Context())
		if attempt > 0 {
			body, err := req.GetBody()
			if err != nil {
				return nil, errors.New("RPC read replay body unavailable")
			}
			request.Body = body
		}
		if t.interval > 0 {
			state.last = time.Now()
		}
		response, err := base.RoundTrip(request)
		if err != nil || response == nil || response.StatusCode != http.StatusTooManyRequests || attempt == 3 {
			return response, err
		}
		backoff = time.Duration(1<<uint(attempt)) * time.Second
		if raw := strings.TrimSpace(response.Header.Get("Retry-After")); raw != "" {
			seconds, e := strconv.Atoi(raw)
			if e != nil || seconds < 0 || seconds > 4 {
				return response, nil
			}
			if time.Duration(seconds)*time.Second > backoff {
				backoff = time.Duration(seconds) * time.Second
			}
		}
		response.Body.Close()
	}
	return nil, errors.New("RPC read rate limit unavailable")
}

func DialRPC(ctx context.Context, endpoint string) (*rpc.Client, error) {
	return dialRPC(ctx, endpoint, NewTransport(endpoint))
}

func dialRPC(ctx context.Context, endpoint string, transport *Transport) (*rpc.Client, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, errors.New("RPC context unavailable")
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, errors.New("RPC endpoint unavailable")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return rpc.DialContext(ctx, endpoint)
	}
	client, err := rpc.DialHTTPWithClient(endpoint, &http.Client{Transport: transport, Timeout: 12 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }})
	if err != nil {
		return nil, errors.New("RPC client unavailable")
	}
	return client, nil
}

func DialScanner(ctx context.Context, endpoint string) (*ethclient.Client, error) {
	client, err := dialRPC(ctx, endpoint, NewScannerTransport(endpoint))
	if err != nil {
		return nil, err
	}
	return ethclient.NewClient(client), nil
}

func Dial(ctx context.Context, endpoint string) (*ethclient.Client, error) {
	client, err := DialRPC(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	return ethclient.NewClient(client), nil
}
