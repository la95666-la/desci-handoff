package websocket

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"

	commonlog "joincare_server/pkg/common/log"
	"joincare_server/pkg/notice/service/action"
	"sync/atomic"
	"testing"
)

func TestCapacityLimiterConcurrentBounds(t *testing.T) {
	limiter := newCapacityLimiter(50, 7)
	var accepted atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if limiter.reserve(fmt.Sprintf("192.0.2.%d", i%5)) {
				accepted.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if got := accepted.Load(); got != 35 {
		t.Fatalf("accepted=%d, want 35 (5 IPs * 7)", got)
	}
	if limiter.total != 35 {
		t.Fatalf("total=%d, want 35", limiter.total)
	}
	for ip, count := range limiter.byIP {
		if count > 7 {
			t.Fatalf("ip %s count=%d exceeds per-IP limit", ip, count)
		}
		for range count {
			limiter.release(ip)
		}
		limiter.release(ip)
	}
	if limiter.total != 0 || len(limiter.byIP) != 0 {
		t.Fatalf("release did not return limiter to zero: total=%d byIP=%v", limiter.total, limiter.byIP)
	}
}

func TestCapacityLimiterGlobalBound(t *testing.T) {
	limiter := newCapacityLimiter(3, 10)
	for i := 0; i < 3; i++ {
		if !limiter.reserve(fmt.Sprintf("198.51.100.%d", i)) {
			t.Fatalf("reservation %d unexpectedly rejected", i)
		}
	}
	if limiter.reserve("203.0.113.1") {
		t.Fatal("reservation exceeded global limit")
	}
}

func TestClientIPTrustsForwardedHeaderOnlyFromConfiguredProxy(t *testing.T) {
	request := httptest.NewRequest("GET", "http://example.test/", nil)
	request.RemoteAddr = "198.51.100.10:4321"
	request.Header.Set("X-Forwarded-For", "203.0.113.9")

	if got := clientIP(request, nil); got != "198.51.100.10" {
		t.Fatalf("untrusted peer spoofed client IP: %q", got)
	}
	trusted := parseTrustedProxyCIDRs([]string{"198.51.100.0/24", "not-a-cidr"})
	if got := clientIP(request, trusted); got != "203.0.113.9" {
		t.Fatalf("trusted proxy client IP=%q, want 203.0.113.9", got)
	}
	request.Header.Set("X-Forwarded-For", "192.0.2.66, 203.0.113.9, 198.51.100.20")
	if got := clientIP(request, trusted); got != "203.0.113.9" {
		t.Fatalf("spoofed left-most XFF bypassed trusted-chain parsing: %q", got)
	}
	request.Header.Set("X-Forwarded-For", "invalid")
	request.Header.Set("X-Real-IP", "192.0.2.8")
	if got := clientIP(request, trusted); got != "198.51.100.10" {
		t.Fatalf("malformed XFF did not fail closed to peer IP: %q", got)
	}
	request.Header.Del("X-Forwarded-For")
	if got := clientIP(request, trusted); got != "192.0.2.8" {
		t.Fatalf("trusted proxy X-Real-IP=%q, want 192.0.2.8", got)
	}
}

func TestHandlerRejectsCapacityBeforeUpgrade(t *testing.T) {
	previousCapacity, previousTrusted := capacity, trustedProxies
	t.Cleanup(func() { capacity, trustedProxies = previousCapacity, previousTrusted })
	capacity = newCapacityLimiter(1, 1)
	trustedProxies = nil
	if !capacity.reserve("198.51.100.1") {
		t.Fatal("failed to fill capacity")
	}
	request := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	request.RemoteAddr = "198.51.100.2:1234"
	response := httptest.NewRecorder()
	WsHandler(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", response.Code)
	}
	if got := response.Header().Get("Retry-After"); got != "5" {
		t.Fatalf("Retry-After=%q, want 5", got)
	}
}

func TestHandlerCapacityIsReleasedAfterConnectionClose(t *testing.T) {
	commonlog.Init(t.TempDir(), "ERROR", 1)
	commonlog.GetLogger().SetOutput(io.Discard)
	viper.Set("protocol.websocket.maxConnections", 1)
	viper.Set("protocol.websocket.maxConnectionsPerIP", 1)
	viper.Set("protocol.websocket.trustedProxyCIDRs", []string{})
	t.Cleanup(func() {
		viper.Reset()
	})
	InitUpGrader()
	stop := make(chan struct{})
	action.OnInit(stop)
	t.Cleanup(func() { close(stop) })
	server := httptest.NewServer(http.HandlerFunc(WsHandler))
	defer server.Close()
	url := "ws" + strings.TrimPrefix(server.URL, "http")

	first, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("first websocket dial: %v", err)
	}
	_, response, err := websocket.DefaultDialer.Dial(url, nil)
	if err == nil {
		t.Fatal("second websocket unexpectedly exceeded per-IP limit")
	}
	if response == nil || response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("second websocket status=%v, want 503", response)
	}
	_ = first.Close()

	deadline := time.Now().Add(time.Second)
	for {
		next, _, dialErr := websocket.DefaultDialer.Dial(url, nil)
		if dialErr == nil {
			_ = next.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("capacity was not released after close: %v", dialErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCapacityDefaultsCannotBeDisabledByMissingValues(t *testing.T) {
	limiter := newCapacityLimiter(0, -1)
	if limiter.maxTotal != defaultMaxConnections || limiter.maxPerIP != defaultMaxConnectionsPerIP {
		t.Fatalf("defaults not applied: total=%d perIP=%d", limiter.maxTotal, limiter.maxPerIP)
	}
}
